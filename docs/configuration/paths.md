# Paths and variables

A path addresses values in a JSON document. Every rule has one path per
side, and `each` rules, `bind` entries and conditions use paths as well.

| Syntax | Addresses |
|---|---|
| `a.b.c` | object keys, separated by dots |
| `a[3]` | the element at a fixed array index |
| `a[$i]` | every array element, binding its index to `$i` |
| `a{$k}` | every entry of an object, binding its key to `$k` |
| `a{line_$i}` | every entry whose key fits the template, binding the variable part |
| `a{${p}_$i}` | a template with several variables, separated by literal text |
| `a[*]`, `a{*}` | anonymous variables, named `$_1`, `$_2`, ... by position |
| `/a.b` | absolute: from the document root, even inside an `each` |
| `.` | the current element itself |
| `/` | the document root itself |
| `"my.key".x` | a quoted key may contain any character; `\"` and `\\` escape |

Suffixes chain and combine with keys: `grid{$r}[$c]` reads row `$r`, column
`$c`; `a[0]{$k}` reads the entries of the first element. A segment that
consists of suffixes alone, such as `[0].x` or `{$k}.v`, is allowed only at
the start of a path, where it addresses the current element's own elements or
entries. Whitespace around the expression is ignored; a bare key must not
contain `$`, `.`, brackets or quotes (quote it instead).

## Variables

A variable is `$` followed by a name matching `[A-Za-z_][A-Za-z0-9_]*`, or
`${name}` where the name would otherwise run into following text
(`{${p}_$i}`). A variable's value is always a string: an array index becomes
`"0"`, `"1"`, ...; an object key is taken as it is.

Variables make rules bidirectional:

- On the side being *read*, an unbound variable iterates: the rule applies
  once per element or entry that matches, and each match binds the variable.
- On the side being *written*, every variable must be bound; its value is
  substituted into the path. A variable used as an array index must then hold
  a non-negative integer (leading zeros are accepted); anything else fails the
  rule.
- A variable that is already bound, by an enclosing `each` or by an earlier
  step of the same path, selects exactly one element or entry.

A copy rule must therefore use the same variables on both sides, apart from
those bound by an enclosing `each`:

```json
{"from": "items[$i].v", "to": "byIndex{item_$i}.value"}
```

Forward, `$i` iterates the array and names the keys. Reverse, `$i` is parsed
out of every `item_N` key and places the value at index N; keys that do not
fit the template (`other`) are ignored.

### Anonymous variables

`[*]` and `{*}` are named `$_1`, `$_2`, ... in order of appearance on each
side of a rule, so `items[*].name` paired with `things[*].label` pairs the
first `*` of one side with the first of the other. They are convenient for
simple copies; use named variables as soon as a path has more than one, or a
nested rule needs to refer to it.

## Key templates

The text between braces is a template of literal characters and variables.
`$$` stands for a literal dollar sign. A template without variables is a
literal key: `a{fixed}` is the same as `a.fixed`.

A key is parsed by matching the template's literal text, so two variables
must be separated by literal text: `{$a$b}` is rejected, `{$a-$b}` is
accepted. Earlier variables take the shortest possible match and the last one
takes the rest: for `{${p}_$i}`, the key `lv_0` binds `p=lv` and `i=0`, and
`a_b_c` binds `p=a` and `i=b_c`. A variable matches any characters, including
newlines. Template text must be valid UTF-8; a template that is not is
rejected when the configuration is loaded.

## Matching and absence

A path with no unbound variables addresses exactly one value, which is either
present or absent. Absence is not an error: a copy rule with an absent source
writes nothing (or its default), and a condition can test for it with
`exists`. Reading through a value of the wrong kind (a key on an array, an
index on an object, anything on a scalar) is likewise absence, as is an index
beyond the end of an array.

A path with unbound variables yields one match per element or entry that
exists, and none when nothing does.

## Writing

Writing resolves a path to a concrete location, substituting every variable,
and creates the objects and arrays on the way. Writing an index beyond the
end of an array grows the array; the positions skipped are removed when the
run completes (see the [evaluation model](overview.md#evaluation-model)).
Writing to `.` or `/` replaces the current element or the whole output
document with the value.
