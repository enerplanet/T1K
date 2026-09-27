# Mapping configuration

A mapping configuration is a JSON document:

```json
{
  "name": "enerplanet-to-meme",
  "version": "1.0.0",
  "description": "What Transform produces, in one or two sentences.",
  "definitions": {
    "costs": [ {"from": "cost_energy_cap", "to": "costs.monetary.investment_per_capacity"} ]
  },
  "rules": [
    {"from": "model_id", "to": "model.metadata.name"},
    {"each": {"from": "techs.pv", "to": "/model.technologies{pv-$k}"}, "rules": [ {"use": "costs"} ]}
  ]
}
```

| Key | Required | Meaning |
|---|---|---|
| `name` | yes | identifies the mapping |
| `version` | no | the mapping's own version, free text |
| `description` | no | what the forward direction produces |
| `definitions` | no | named rule lists, spliced in wherever a rule says `{"use": "name"}` |
| `rules` | yes | the rules, applied in order |
| `$schema` | no | accepted and ignored, for editors |

Any other key, at any level, is an error: configurations are validated
strictly when loaded, and every message names the position of the offending
rule (`rules[3].rules[1]`, or `rules[2](definitions.costs)[0]` for a rule
spliced in from a definition).

The three kinds of rule are described on [Rules](rules.md); the path syntax on
[Paths and variables](paths.md); the converters on [Converters](converters.md);
and what all of this means when run backwards on
[The reverse direction](reverse.md).

## Evaluation model

Both directions follow the same procedure. The *source* is the input document
and the *target* the document being built. In the forward direction a rule's
`from` addresses the source and `to` the target; in the reverse direction the
roles swap.

1. The target starts as an empty object.
2. Rules are applied in the order written. A rule reads the values its source
   path addresses, converts them, and writes them to the target path. Writing
   creates the objects and arrays on the way; a value already present at the
   target path is replaced, so **later rules win**.
3. A rule whose source path addresses nothing writes nothing, unless it has a
   default for that direction.
4. `each` rules apply their nested rules once per matched element, with paths
   relative to the pair of elements.
5. After the last rule, arrays are *compacted*: positions no rule wrote (which
   arise when an index variable comes from a key such as `item_5`) are
   removed, so index variables need not be contiguous. Explicit `null`
   values written by rules are kept.

Numbers are carried as their original text, so a value that is only moved
keeps its digits (`8.0` stays `8.0`); converters that compute a number emit it
in plain decimal notation, rounded to 15 significant digits. Object keys are
emitted in sorted order; JSON objects are unordered, and sorted output keeps
results reproducible and diffs small.

## A complete example

```json
{
  "name": "orders-to-invoices",
  "rules": [
    {"from": "order.number", "to": "invoice.reference"},
    {"from": "order.total_cents", "to": "invoice.total", "convert": {"linear": {"divisor": 100}}},
    {"to": "invoice.currency", "value": "EUR"},
    {"from": "order.status", "value": "converted"},
    {
      "each": {"from": "order.lines[$i]", "to": "invoice.items{line_$i}"},
      "when": {"forward": {"path": "qty", "gt": 0}},
      "rules": [
        {"from": "sku", "to": "article"},
        {"from": "qty", "to": "quantity", "convert": "number"},
        {"to": "position", "template": "$i"}
      ]
    }
  ]
}
```

Forward: the reference and total are copied (the total divided by 100), the
currency is set, and every line with a positive quantity becomes an item keyed
by its position. Reverse: the reference and total come back (the total
multiplied by 100), the status is set to `converted`, and every `line_N`
item becomes the line at position N; `currency` and `position` were written
for the invoice side only and are simply not read.
