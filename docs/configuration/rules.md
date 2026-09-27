# Rules

`rules` is a list; every entry is one of three kinds of rule, or a `use` of a
definition. Rules apply in order and later rules overwrite earlier writes to
the same path.

## Copy rules

```json
{"from": "cont_energy_cap_max", "to": "capacity.max",
 "convert": [{"absent": {"values": ["inf"]}}, {"linear": {"divisor": 1000}}],
 "reverse_default": "inf",
 "description": "kW to MW; 'inf' means unbounded"}
```

| Key | Meaning |
|---|---|
| `from` | path on the source side (read by Transform, written by Reverse) |
| `to` | path on the target side (written by Transform, read by Reverse) |
| `convert` | a [converter](converters.md) or a list of them; Reverse runs them backwards |
| `default` | written to `to` by Transform when `from` is absent (or the converter made it absent) |
| `reverse_default` | written to `from` by Reverse when `to` is absent |
| `when` | [conditions](#conditions) |
| `description` | free text, ignored |

`from` and `to` must use the same variables, apart from those an enclosing
`each` binds. A default only applies when the path is concrete (no unbound
variables), because absence of an iterated path is simply zero matches.

The value at the source path is copied as a whole: an object or array
moves with everything inside it. Nothing is ever taken by reference; the
output never shares memory with the input.

## Constant rules

```json
{"to": "experiment.mode", "value": "plan"}
{"from": "geometry.type", "value": "Point"}
{"to": "node", "template": "$k"}
```

A constant has exactly one side. `to` constants are written by Transform,
`from` constants by Reverse; the other direction ignores the rule. `value`
may be any JSON value (`null`, an object, an array); `template` is a string
with variables, substituted from the enclosing `each` bindings (`$k`, `${k}`,
`$$` for a dollar sign).

Constants are how the reverse direction restores what the forward direction
cannot carry: declare the value a source field should have when its target
counterpart does not exist. Put such constants *before* the copy rule for the
same path, so a real value, when present, wins.

## Each rules

```json
{
  "each": {"from": "topology[$i].from", "to": "model.nodes{$k}", "bind": {"$k": "id"}},
  "when": {"forward": {"path": "properties.feature_type", "equals": "BasePOI"}},
  "rules": [
    {"from": "geometry.coordinates[1]", "to": "coords.lat", "convert": "number"},
    {"from": "properties.osm_id", "to": "name"},
    {"from": "properties.demand_energy", "to": "/model.technologies{demand-$k}.demand_profile"}
  ]
}
```

| Key | Meaning |
|---|---|
| `each.from` | path of the source elements |
| `each.to` | path of the target elements |
| `each.bind` | variables read from a value inside each source element (see below) |
| `rules` | rules applied to every pair of elements, with relative paths |
| `when` | [conditions](#conditions), evaluated per element |
| `description` | free text, ignored |

Inside the nested rules, relative paths start at the current element on each
side; absolute paths (`/...`) start at the document root. Variables bound by
the `each` (and by enclosing ones) are available in nested paths, templates
and conditions.

Forward, the elements of `each.from` are enumerated in the input and the
nested rules write below `each.to`; reverse, the entries of `each.to` are
enumerated and the nested rules write below `each.from`. An `each` whose
paths have no variables applies its rules once, if the element exists, and
not at all otherwise; that makes `each` the way to write a group of
constants only when some source object is present.

### bind

`bind` maps a variable to a path read from each source element. The value
must be a string, number or boolean and becomes the variable's binding; a
missing value is an error, so the mapping does not silently drop elements.

With `bind`, the target side can be keyed by a value rather than a position:
`topology[$i].from` paired with `model.nodes{$k}` and `{"$k": "id"}` turns an
array of features into an object keyed by their ids, and writes a feature
that appears several times only once.

The reverse direction depends on whether the target side determines every
variable of the source side:

- **Iterate mode**, when it does (`conns[$i]` with `arcs{${p}_$i}` and
  `{"$p": "pipe"}`): the target entries are enumerated, the key is parsed,
  and each bound variable is written back to its bind path (`pipe` gets
  `lv`).
- **Join mode**, when it does not (`topology[$i].from` with `model.nodes{$k}`:
  `$i` is not in the key): the `each` looks at the source elements that
  earlier rules have already created in the output, reads the bind path from
  each (`id`), and fills the element from the matching target entry. Such a
  rule must therefore come *after* the rules that create those elements and
  their bind values; entries without a created element are not restored.

### Nesting

`each` rules nest. A nested `each` may address its own collections relative
to the current element or, with absolute paths, elsewhere in the document,
which is how per-element technologies can be collected into one top-level
map:

```json
{"each": {"from": "techs.pv_supply", "to": "/model.technologies{pv_supply-$k}"}, "rules": [...]}
```

## Definitions and use

`definitions` holds named rule lists; `{"use": "name"}` splices one in
place. The spliced rules are compiled where they are used, so they may rely
on the variables bound at that place (a definition used inside an `each`
over `model.nodes{$k}` may use `$k`), and the same definition can serve
several `each` rules. A definition may use other definitions; cycles are
rejected.

## Conditions

```json
"when": {
  "forward": [{"path": "feature_type", "equals": "TopologyNode"}, {"path": "rated_power", "gt": 0}],
  "reverse": {"path": "/model.trade{grid-$k}", "exists": true}
}
```

`when` gates a rule per direction: `forward` applies to Transform, `reverse`
to Reverse; a direction without conditions always applies. Each direction
takes one condition or a list, all of which must hold. A condition names a
`path`, evaluated against the *source* document of that direction, relative
to the current element (for an `each`, the element itself), or absolute with
`/`; it may use bound variables. Its remaining keys are operators, all of
which must hold:

| Operator | Holds when |
|---|---|
| `exists: true` / `false` | the path is present / absent |
| `equals: v` | the value is present and equals `v` (numbers by value) |
| `not_equals: v` | the value is absent or differs from `v` |
| `in: [v, ...]` | the value is present and equals one of the list |
| `gt`, `gte`, `lt`, `lte: n` | the value is a number in the relation to `n` |

The forward conditions of an `each` run before its bind paths are read, so
they may use the iteration variables but not the bound ones; reverse
conditions have everything bound.
