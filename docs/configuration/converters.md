# Converters

A converter changes a value on its way from one side of a copy rule to the
other. Every converter is invertible: Transform applies it forwards, Reverse
backwards. A converter may also turn a value into *absent*, in which case the
target key is not written and the rule's default, if any, applies.

`convert` takes one step or a list of steps applied in order; Reverse runs
the list backwards. A step is a converter name or a one-key object carrying
its arguments:

```json
"convert": "number"
"convert": {"linear": {"divisor": 1000}}
"convert": [{"absent": {"values": ["inf"], "ignore_case": true}}, {"linear": {"divisor": 1000}}]
```

Unknown converters and unknown arguments are rejected when the configuration
is loaded.

## identity

Passes the value through. It is the default when `convert` is omitted.

## number

Accepts a JSON number or a string holding one and yields a JSON number, in
both directions. A number keeps its exact digits (`8.0` stays `8.0`); a string
is parsed and normalised (`" 8 "` becomes `8`). Booleans, `null` and
containers are errors.

## string

Forward renders a number or boolean as a string (`8.5` to `"8.5"`, `true` to
`"true"`); strings pass through. Reverse turns a string that reads as a
number or boolean back into one and leaves other strings alone; a non-string
passes through.

## linear

`y = x * factor / divisor + offset`; the reverse solves for `x`.

| Argument | Default | Meaning |
|---|---|---|
| `factor` | 1 | multiplier |
| `divisor` | 1 | divisor |
| `offset` | 0 | added after scaling |
| `round_forward` | none | round the forward result to this many decimals |
| `round_reverse` | none | round the reverse result to this many decimals |

`factor` and `divisor` must not be zero. The input may be a number or a
numeric string; anything else is an error. Results are emitted in plain
decimal notation rounded to 15 significant digits, so `8889.3 / 1000` is
`8.8893`, not `8.889299999999999`.

Rounding is where a linear conversion may lose information; the shipped
mapping uses it to turn an annual energy in kWh into an average power in MW
with nine decimals and back into whole kWh.

## lookup

Translates between two vocabularies through a list of pairs:

```json
{"lookup": {"pairs": [[15, "15min"], [30, "30min"], [60, "1H"]], "strict": true}}
```

Forward maps the left value to the right, reverse the right to the left;
values compare structurally (numbers by value, so `60.0` matches `60`) and
may be of any JSON type. Both columns must be free of duplicates so the table
is invertible. A value without a pair is an error, or passes through
unchanged when `strict` is `false`.

## datetime

Re-formats a timestamp string between two layouts:

```json
{"datetime": {"from": "2006-01-02T15:04:05.000Z", "to": "RFC3339", "zone": "UTC"}}
```

Forward parses with `from` and renders with `to`; reverse does the opposite.
Layouts are Go reference layouts, or one of the names `RFC3339`,
`RFC3339Nano`, `RFC1123`, `RFC1123Z`, `DateOnly` (`2006-01-02`), `DateTime`
(`2006-01-02 15:04:05`) and `TimeOnly`. Timestamps without zone information
are read in `zone` (an IANA name, default `UTC`), and output is rendered in
that zone. Parts a layout does not carry (fractions of a second, the time of
day with `DateOnly`) are lost on the way, which is the one place this
converter is not exact.

## absent

Treats the listed values as *not present*, in both directions:

```json
{"absent": {"values": ["inf", null], "ignore_case": true}}
```

With it, a placeholder such as `"inf"` or `null` is dropped instead of
copied. Pair it with the rule's `default` or `reverse_default` to restore
the placeholder in the other direction:

```json
{"from": "cont_energy_cap_max", "to": "capacity.max",
 "convert": [{"absent": {"values": ["inf"]}}, {"linear": {"divisor": 1000}}],
 "reverse_default": "inf"}
```

Values compare structurally; `ignore_case` extends the match to strings that
differ only in case.
