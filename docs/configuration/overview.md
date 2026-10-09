# Mapping configuration

A mapping configuration is a JSON document that describes how one JSON
structure maps onto another. T1K reads it in both directions: the forward
direction builds the target document from the source document, the reverse
direction builds the source document from the target document.

```json
{
  "name": "enerplanet-to-meme",
  "version": "1.1.0",
  "description": "What the forward direction produces, in one or two sentences.",
  "definitions": {
    "costs": [ {"from": "cost_energy_cap", "to": "costs.monetary.investment_per_capacity"} ]
  },
  "rules": [
    {"from": "model_id", "to": "model.metadata.name"},
    {"each": {"from": "techs.pv", "to": "/model.technologies{pv_$k}"}, "rules": [ {"use": "costs"} ]}
  ]
}
```

| Key | Required | Meaning |
|---|---|---|
| `name` | yes | identifies the mapping; must not be empty |
| `version` | no | the mapping's own version, free text |
| `description` | no | what the forward direction produces |
| `definitions` | no | named rule lists, spliced in wherever a rule says `{"use": "name"}` |
| `rules` | yes | the rules, applied in order; may be empty |
| `$schema` | no | accepted and ignored, for editors |

Configurations are validated strictly when loaded. Any other key, at any
level, is an error, as are unbalanced variables, unknown converters and
malformed conditions, and every message names the position of the offending
rule: `rules[3].rules[1]`, or `rules[2](definitions.costs)[0]` for a rule
spliced in from a definition.

The three kinds of rule are described on [Rules](rules.md), the path syntax
on [Paths and variables](paths.md), the converters on
[Converters](converters.md), and what all of this means when run backwards
on [The reverse direction](reverse.md).

## Evaluation model

Both directions follow the same procedure. The *source* is the input document
and the *target* the document being built. In the forward direction a rule's
`from` addresses the source and `to` the target; in the reverse direction the
roles swap.

1. The target starts as an empty object.
2. Rules are applied in the order written. A copy rule reads the values its
   source path addresses, converts them, and writes them to the target path;
   a constant rule writes its value; an `each` rule applies its nested rules
   once per matched element.
3. Writing creates the objects and arrays on the way. A value already present
   at the target path is replaced, so **later rules win**; a container of the
   wrong kind in the way is replaced as well.
4. A copy rule whose source path addresses nothing writes nothing, unless it
   declares a default for that direction.
5. After the last rule, arrays are *compacted*: positions no rule wrote
   (which arise when an index comes from a key such as `item_5`) are removed,
   so index variables need not be contiguous. Explicit `null` values are kept.

### Output conventions

- Numbers are carried as their original text. A value that is only moved
  keeps its digits (`8.0` stays `8.0`, `1e3` stays `1e3`); a converter that
  computes a number emits it in plain decimal notation, rounded to 15
  significant digits, so `8889.3 / 1000` is `8.8893`.
- Object keys are emitted in sorted order. JSON objects are unordered, and
  sorted output keeps results reproducible and diffs small.
- The output never shares memory with the input: values are copied, never
  referenced.

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

Forward: the reference and the total are copied (the total divided by 100),
the currency is set, and every line with a positive quantity becomes an item
keyed by its position. Reverse: the reference and the total come back (the
total multiplied by 100), the status is set to `converted`, and every
`line_N` item becomes the line at position N. `currency` and `position` were
written for the invoice side only and are simply not read.
