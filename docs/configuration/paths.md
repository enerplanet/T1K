# Paths and variables

A path addresses values in a JSON document.

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
| `"my.key".x` | a quoted key may contain any character (`\"` and `\\` escape) |

Segments may combine: `grid{$r}[$c]` reads row `$r`, column `$c`; `[0].x`
starts at an array element when the document (or the current element) is an
array.

## Variables

A variable is `$` followed by a name (`[A-Za-z_][A-Za-z0-9_]*`), or `${name}`
where the name would otherwise run into following text (`{${p}_$i}`). Its
value is always a string: an array index becomes `"0"`, `"1"`, ...; an object
key is taken as it is.

Variables make rules bidirectional:

- On the side being *read*, an unbound variable iterates: the rule applies
  once for every element or entry that matches, and each match binds the
  variable.
- On the side being *written*, every variable must be bound; its value is
  substituted into the path. A variable used as an array index must then
  hold a non-negative integer, otherwise the rule fails.
- A variable that is already bound (by an enclosing `each`, or by an earlier
  step of the same path) selects exactly one element or entry.

Hence a copy rule must use the same set of variables on both sides, apart
from those bound by an enclosing `each`:

```json
{"from": "items[$i].v", "to": "byIndex{item_$i}.value"}
```

Forward, `$i` iterates the array and names the keys; reverse, `$i` is parsed
out of every `item_N` key and places the value at index N. Keys that do not
fit the template (`other`) are ignored in the reverse direction.

## Key templates

The text between braces is a template of literal characters and variables.
`$$` stands for a literal dollar sign. Two variables must be separated by
literal text (`{$a$b}` is rejected, `{$a-$b}` is fine), because a key is
parsed by matching that text: earlier variables take the shortest possible
match, the last one the rest. For `{${p}_$i}`, the key `lv_0` binds `p=lv`
and `i=0`, and `a_b_c` binds `p=a` and `i=b_c`. A template without variables
is a literal key: `a{fixed}` is the same as `a.fixed`.

## Matching and absence

A path with no unbound variables addresses exactly one value, which is either
present or absent. Absence is not an error: a copy rule with an absent source
writes nothing (or its default), and a condition can test it with `exists`.
Reading through a value of the wrong kind (a key on an array, an index on an
object, anything on a scalar) is likewise absence.

A path with unbound variables yields one match per element that exists, and
none when nothing does.

## Anonymous variables

`[*]` and `{*}` are named `$_1`, `$_2`, ... in order of appearance on each
side of a rule, so `items[*].name` paired with `things[*].label` pairs the
first `*` of one side with the first of the other. They are convenient for
simple copies; use named variables as soon as a path has more than one or a
nested rule refers to it.
