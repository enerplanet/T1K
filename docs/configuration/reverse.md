# The reverse direction

`Reverse` is not a second mapping: it is the same rules read from `to` to
`from`. This page collects what that means and how to write a mapping whose
reverse does what you expect.

## What Reverse does with each construct

| Construct | Forward | Reverse |
|---|---|---|
| copy rule | read `from`, convert, write `to` | read `to`, convert backwards, write `from` |
| `default` | written when `from` is absent | ignored |
| `reverse_default` | ignored | written when `to` is absent |
| constant with `to` | written | ignored |
| constant with `from` | ignored | written |
| `each` | enumerate `from` elements, write below `to` | enumerate `to` entries, write below `from` (or join, see below) |
| `bind` | read the variable from the element | write the variable back (iterate mode) or read it from the created element (join mode) |
| `when.forward` | gates the rule | ignored |
| `when.reverse` | ignored | gates the rule |
| converter | forward | reverse, chains backwards |

## Lossless and lossy

A copy rule with an invertible converter is lossless: the value comes back
exactly (numbers to their digits, unless a converter computed them). Two
things are lossy by nature:

- A source field with no counterpart on the target side is not carried.
  Declare what Reverse should put there with a `from` constant or a
  `reverse_default`, choosing a value that is honest about being a
  placeholder, and document it.
- A converter that rounds, or a `datetime` layout that drops parts, loses
  the digits or parts it does not keep.

Everything else round-trips, and the shipped mapping's tests check exactly
that: converting the reverse output forward again reproduces the job, and
reversing that reproduces the reverse output.

## Patterns

**Placeholder values.** EnerPlanET's `"inf"` means "no bound" and has no
MEME counterpart. Drop it forward with `absent`, restore it backwards with
`reverse_default`:

```json
{"from": "cont_energy_cap_max", "to": "capacity.max",
 "convert": [{"absent": {"values": ["inf"], "ignore_case": true}}, {"linear": {"divisor": 1000}}],
 "reverse_default": "inf"}
```

**Defaults before values.** A `from` constant followed by a copy rule for
the same path gives "this value, unless the target has a real one":

```json
{"from": "properties.demand_energy", "value": 0},
{"each": {"from": "properties", "to": "/model.technologies{demand-$k}"},
 "rules": [{"from": "demand_energy", "to": "demand_profile", "convert": {"linear": {"divisor": 8760000}}}]}
```

**Discriminators.** A source field that selects a shape (`feature_type` being
`BasePOI` or `TopologyNode`) can often be reconstructed from what the forward
direction produced for each shape. Two `from` constants with complementary
`when.reverse` conditions do it:

```json
{"from": "properties.feature_type", "value": "BasePOI",      "when": {"reverse": {"path": "/model.trade{grid-$k}", "exists": false}}},
{"from": "properties.feature_type", "value": "TopologyNode", "when": {"reverse": {"path": "/model.trade{grid-$k}", "exists": true}}}
```

**Collections keyed by value.** An `each` with `bind` whose key does not
carry the array position runs in join mode backwards: it fills the elements
that earlier rules created. Order the rules so the creating rule (the one
that writes `topology[$i]` with `from.id`) comes first. Elements nothing
creates are not restored; in the shipped mapping, that is a building without
a connection, which has a node but no transmission arc.

**Keys that carry positions.** A template such as `{line_$i}` paired with
`topology[$i]` places entries by the number in their key. Entries whose key
does not fit the template are skipped; a key that fits but holds something
other than a non-negative integer is an error, because it cannot be an index.
Gaps in the numbers are closed when arrays are compacted at the end.

## Checking a mapping

For any mapping, the useful checks are: `Reverse(Transform(x))` has the shape
of `x` with only the declared placeholders differing;
`Transform(Reverse(Transform(x)))` equals `Transform(x)`; and both outputs
are accepted by the systems that will read them. See
[Testing](../development/testing.md) for how the repository does this for the
shipped mapping.
