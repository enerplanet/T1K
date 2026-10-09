# Rules

`rules` is a list. Every entry is one of three kinds of rule (copy, constant
or `each`), or a `use` of a definition. Rules apply in the order written, and
a later write to a path replaces an earlier one.

| Kind | Recognised by | Purpose |
|---|---|---|
| copy | `from` and `to`, no `value` or `template` | move a value between the two sides, optionally converted |
| constant | `value` or `template` | write a fixed value on one side |
| `each` | `each` | apply nested rules to every element of a collection |
| use | `use` | splice a named rule list from `definitions` |

Every rule accepts `description`, free text that T1K ignores.

## Copy rules

```json
{"from": "cont_energy_cap_max", "to": "capacity.max",
 "convert": [{"absent": {"values": ["inf"]}}, {"linear": {"divisor": 1000}}],
 "reverse_default": "inf",
 "description": "kW to MW; 'inf' means unbounded"}
```

| Key | Meaning |
|---|---|
| `from` | the path on the source side: read by the forward direction, written by the reverse |
| `to` | the path on the target side: written by the forward direction, read by the reverse |
| `convert` | a [converter](converters.md) or a list of them, applied in order; the reverse direction runs them backwards |
| `default` | written to `to` by the forward direction when `from` is absent, or the converter made the value absent |
| `reverse_default` | written to `from` by the reverse direction when `to` is absent |
| `when` | [conditions](#conditions) per direction |

`from` and `to` must use the same variables, apart from those an enclosing
`each` binds. A default applies only when the path is concrete (no unbound
variables), because the absence of an iterated path is simply zero matches.

The value at the source path is copied as a whole: an object or array moves
with everything inside it, and nothing is taken by reference.

## Constant rules

```json
{"to": "experiment.mode", "value": "plan"}
{"from": "geometry.type", "value": "Point"}
{"to": "node", "template": "n$k"}
```

A constant has exactly one side. A constant with `to` is written by the
forward direction, one with `from` by the reverse direction; the other
direction ignores the rule. `value` may be any JSON value, including `null`,
objects and arrays; `template` is a string in which `$k`, `${k}` and `$$`
are substituted from the enclosing `each` bindings. A constant's path may use
only variables an enclosing `each` binds.

Constants are how the reverse direction restores what the forward direction
cannot carry: declare the value a source field should have when its target
counterpart does not exist. Place such a constant *before* the copy rule for
the same path, so a real value, when present, wins.

## Each rules

```json
{
  "each": {"from": "topology[$i].from", "to": "model.nodes{n$k}", "bind": {"$k": "id"}},
  "when": {"forward": {"path": "properties.feature_type", "equals": "BasePOI"}},
  "rules": [
    {"from": "geometry.coordinates[1]", "to": "coords.lat", "convert": "number"},
    {"from": "properties.osm_id", "to": "name"},
    {"from": "properties.demand_energy", "to": "/model.technologies{demand_$k}.demand_profile"}
  ]
}
```

| Key | Meaning |
|---|---|
| `each.from` | the path of the source elements |
| `each.to` | the path of the target elements |
| `each.bind` | variables read from values inside each source element (see [bind](#bind)) |
| `rules` | the rules applied to every pair of elements, with paths relative to the pair; may be omitted or empty |
| `when` | [conditions](#conditions), evaluated per element |

Inside the nested rules, relative paths start at the current element on each
side; absolute paths (`/...`) start at the document root. The variables bound
by the `each`, and by enclosing ones, are available in nested paths,
templates and conditions.

In the forward direction the elements of `each.from` are enumerated in the
source document and the nested rules write below `each.to`; in the reverse
direction the entries of `each.to` are enumerated and the nested rules write
below `each.from`. An `each` whose paths have no variables applies its rules
once if the element exists and not at all otherwise, which makes it the way
to write a group of constants only when a source object is present.

### bind

`bind` maps variables to paths read from each source element:

```json
"bind": {"$k": "id"}
```

The value at the path must be a string, number or boolean and becomes the
variable's binding, as text; a missing or non-scalar value fails the rule,
so a mapping never drops elements silently. A bind variable must be new (not
bound by an enclosing `each` nor by `each.from`), must appear in `each.to`,
and its path may use the variables the iteration binds.

With `bind`, the target side can be keyed by a value instead of a position:
`topology[$i].from` paired with `model.nodes{n$k}` and `{"$k": "id"}` turns
an array of features into an object keyed by their ids, and writes a feature
that appears several times only once.

How the reverse direction enumerates depends on whether `each.to` determines
every variable of `each.from`:

- **Iterate mode**, when it does (`conns[$i]` with `arcs{${p}_$i}` and
  `{"$p": "pipe"}`): the target entries are enumerated, each key is parsed,
  and every bound variable is written back to its bind path as a string
  (`pipe` gets `"lv"`).
- **Join mode**, when it does not (`topology[$i].from` with
  `model.nodes{n$k}`: `$i` is not in the key): the `each` looks at the source
  elements that earlier rules have already created in the output, reads the
  bind path from each of them (`id`), and fills the element from the matching
  target entry. Such a rule must therefore come *after* the rules that create
  those elements and their bind values; entries without a created element are
  not restored.

### Nesting

`each` rules nest. A nested `each` may address its own collections relative
to the current element or, with absolute paths, elsewhere in the document,
which is how per-element technologies are collected into one top-level map:

```json
{"each": {"from": "techs.pv_supply", "to": "/model.technologies{pv_supply_$k}"}, "rules": [...]}
```

## Definitions and use

`definitions` holds named rule lists; `{"use": "name"}` splices one in place.
The spliced rules are compiled where they are used, so they may rely on the
variables bound at that place (a definition used inside an `each` over
`model.nodes{n$k}` may use `$k`), and one definition can serve several `each`
rules. A definition may use other definitions; cycles are rejected.

## Conditions

```json
"when": {
  "forward": [{"path": "feature_type", "equals": "TopologyNode"}, {"path": "rated_power", "gt": 0}],
  "reverse": {"path": "/model.trade{grid_$k}", "exists": true}
}
```

`when` gates a rule per direction: `forward` applies to the forward
direction, `reverse` to the reverse direction, and a direction without
conditions always applies. Each direction takes one condition or a list of
conditions, all of which must hold. A condition names a `path`, evaluated
against the *source* document of that direction, relative to the current
element (for an `each`, the element itself) or absolute with `/`. The path
may use bound variables and must address a single value. Its remaining keys
are operators, all of which must hold:

| Operator | Holds when |
|---|---|
| `exists: true` / `false` | the path is present / absent |
| `equals: v` | the value is present and equals `v` (structurally; numbers by value) |
| `not_equals: v` | the value is absent or differs from `v` |
| `in: [v, ...]` | the value is present and equals one of the list |
| `gt`, `gte`, `lt`, `lte: n` | the value is a number in the given relation to `n`; a non-number never holds |

The forward conditions of an `each` run before its bind paths are read, so
they may use the iteration variables but not the bound ones; reverse
conditions have every variable bound.
