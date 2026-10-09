# Converters

A converter changes a value on its way from one side of a copy rule to the
other. Every converter is invertible: the forward direction applies it
forwards, the reverse direction backwards, and `reverse(forward(v))` yields
`v` again up to the documented loss of representation. A converter may also
turn a value into *absent*, in which case the target key is not written and
the rule's default for that direction, if any, applies.

`convert` takes one step or a list of steps applied in order; the reverse
direction runs the list backwards. A step is a converter name, or an object
with one key, the name, whose value holds the arguments:

```json
"convert": "number"
"convert": {"linear": {"divisor": 1000}}
"convert": [{"absent": {"values": ["inf"], "ignore_case": true}}, {"linear": {"divisor": 1000}}]
```

Unknown converters and unknown or mistyped arguments are rejected when the
configuration is loaded. An absent value passes through every converter
unchanged.

| Converter | Forward | Reverse |
|---|---|---|
| [`identity`](#identity) | the value as it is | the same |
| [`number`](#number) | a number or numeric string as a number | the same |
| [`string`](#string) | a scalar as a string | a string that reads as a number or boolean back to one |
| [`linear`](#linear) | `x * factor / divisor + offset` | solved for `x` |
| [`lookup`](#lookup) | the right value of the pair whose left value matches | the left value of the matching right value |
| [`datetime`](#datetime) | a timestamp rendered in the `to` layout | rendered in the `from` layout |
| [`absent`](#absent) | listed values become absent | the same |
| [`affix`](#affix) | `prefix + value + suffix` | the prefix and suffix stripped |
| [`match`](#match) | the value, or an error when it does not match the pattern | the same |

## identity

Passes the value through unchanged. It is the converter of a rule without
`convert`; naming it explicitly is allowed.

## number

Accepts a JSON number or a string holding one and yields a JSON number, in
both directions. A number keeps its exact text (`8.0` stays `8.0`, `1e3` stays
`1e3`); a string is parsed and normalised (`" 8 "` becomes `8`, `"1e3"`
becomes `1000`). Booleans, `null`, containers and strings that do not hold a
finite decimal number are errors.

## string

Forward renders a number or boolean as a string (`8.5` to `"8.5"`, `true` to
`"true"`); strings pass through; `null` and containers are errors. Reverse
turns a string that reads as a finite number or as `true`/`false` back into
that value and leaves every other string alone; a non-string passes through.

## linear

`y = x * factor / divisor + offset`; the reverse solves for `x`.

| Argument | Default | Meaning |
|---|---|---|
| `factor` | 1 | multiplier |
| `divisor` | 1 | divisor |
| `offset` | 0 | added after scaling |
| `round_forward` | none | round the forward result to this many decimals |
| `round_reverse` | none | round the reverse result to this many decimals |

`factor` and `divisor` must not be zero, and rounding decimals must not be
negative. The input may be a number or a numeric string; anything else is an
error. Results are emitted in plain decimal notation, rounded to 15
significant digits, so `8889.3 / 1000` is `8.8893` and never
`8.889299999999999`; a result that is not finite is an error.

Rounding is the one place a linear conversion loses information. The shipped
mapping uses it to turn an annual energy in kWh into an average power in MW
with nine decimals and back into whole kWh.

## lookup

Translates between two vocabularies through a list of pairs:

```json
{"lookup": {"pairs": [[15, "15min"], [30, "30min"], [60, "1H"]], "strict": true}}
```

Forward maps the left value of a pair to its right value, reverse the right
value to the left. Values are compared structurally (numbers by value, so
`60.0` matches `60`) and may be of any JSON type, objects and arrays
included; the matched value is copied out of the table, never shared. Both
columns must be free of duplicates so the table is invertible. A value
without a pair is an error, or passes through unchanged when `strict` is
`false`.

## datetime

Re-formats a timestamp string between two layouts:

```json
{"datetime": {"from": "2006-01-02T15:04:05.000Z", "to": "RFC3339", "zone": "UTC"}}
```

Forward parses the value with `from` and renders it with `to`; reverse does
the opposite. Both layouts are required. A layout is a Go reference layout,
or one of the names `RFC3339`, `RFC3339Nano`, `RFC1123`, `RFC1123Z`,
`DateOnly` (`2006-01-02`), `DateTime` (`2006-01-02 15:04:05`) and `TimeOnly`
(`15:04:05`). Timestamps without zone information are read in `zone`, an IANA
zone name that defaults to `UTC`, and output is rendered in that zone. A
non-string value or a timestamp the layout cannot parse is an error.

Parts a layout does not carry (fractions of a second, the time of day with
`DateOnly`) are lost on the way, which is the one place this converter is not
exact.

## absent

Treats the listed values as *not present*, in both directions:

```json
{"absent": {"values": ["inf", null], "ignore_case": true}}
```

With it, a placeholder such as `"inf"` or `null` is dropped instead of
copied. Pair it with the rule's `default` or `reverse_default` to restore the
placeholder in the other direction:

```json
{"from": "cont_energy_cap_max", "to": "capacity.max",
 "convert": [{"absent": {"values": ["inf"]}}, {"linear": {"divisor": 1000}}],
 "reverse_default": "inf"}
```

`values` must be a non-empty list. Values compare structurally;
`ignore_case` extends the match to strings that differ only in case and has
no effect on other types.

## affix

Wraps a value in literal text:

```json
{"affix": {"prefix": "n", "suffix": "", "strict": true}}
```

Forward renders `prefix + value + suffix` as a string; reverse strips both
again. At least one of `prefix` and `suffix` is required. `affix` is the
value-side counterpart of a key template such as `{n$k}`: where an
identifier travels as a value rather than as a key, for example a node id
copied into a reference, `affix` keeps it consistent with the wrapped key.

Forward accepts any scalar and always yields a string, so a number comes
back from the reverse as the text of its digits; `null` and containers are
errors. Reverse requires a string that starts with the prefix and ends with
the suffix, and fails otherwise; with `strict` false such a value passes
through unchanged. An empty value is wrapped and unwrapped like any other.

## match

A guard that changes nothing and fails when the value does not match a
pattern:

```json
{"match": {"pattern": "[^_\\d]\\w*"}}
```

The pattern uses Go regular expression syntax and is anchored to the whole
value, so `[a-z]+` and `^[a-z]+$` mean the same. The guard applies in both
directions. Numbers and booleans are matched by their text; `null` and
containers have no text and fail. An invalid pattern is rejected when the
configuration is loaded.

Use `match` to turn a constraint of the target system, such as an identifier
syntax, into an error that names the rule, instead of a rejection downstream.
Chained after `affix`, it checks the wrapped value:

```json
{"from": "from.id", "to": "from", "convert": [{"affix": {"prefix": "n"}}, {"match": {"pattern": "[^_\\d]\\w*"}}]}
```
