# T1K

T1K converts one JSON structure into another, driven by a declarative mapping
configuration, and converts the result back again. It is a Go package with a
command-line tool, depends only on the Go standard library, and ships with the
mapping that turns an [EnerPlanET](https://github.com/enerplanet/enerplanet)
calculation payload into a [MEME](https://github.com/enerplanet/meme) job.

## The core idea

A mapping is a list of rules. Each rule pairs a path in the source document
with a path in the target document:

```json
{"from": "order.total_cents", "to": "invoice.total", "convert": {"linear": {"divisor": 100}}}
```

`Transform` reads the rule left to right; `Reverse` reads it right to left and
runs the converter backwards. Because every rule is written once and read in
both directions, one configuration gives both transformations, and a mapping
cannot drift between them.

Paths can contain variables. A variable iterates on the side where it is
matched and is substituted on the other side, which is how a single rule
moves a whole collection and restructures it:

```json
{
  "each": {"from": "order.lines[$i]", "to": "invoice.items{line_$i}"},
  "rules": [
    {"from": "sku", "to": "article"},
    {"from": "qty", "to": "quantity", "convert": "number"}
  ]
}
```

turns an array of lines into an object keyed `line_0`, `line_1`, ... and back.

## Where to go next

- [Installation](usage/installation.md) and the [Quickstart](usage/quickstart.md)
- Using the [command line](usage/cli.md) or the [Go package](usage/library.md)
- The mapping language: [overview](configuration/overview.md),
  [paths](configuration/paths.md), [rules](configuration/rules.md),
  [converters](configuration/converters.md) and
  [the reverse direction](configuration/reverse.md)
- The shipped mapping: [EnerPlanET to MEME](mappings/enerplanet-to-meme.md)
- For contributors: [architecture](development/architecture.md) and
  [testing](development/testing.md)

## Design goals

- **Reversible by construction.** A rule is a relation, not a procedure.
  Converters are invertible; what cannot be inverted (a dropped field) is
  restored from a declared default.
- **Strict configurations.** Unknown keys, unbalanced variables, unknown
  converters and malformed conditions are rejected when the configuration is
  loaded, with the rule's position in the message.
- **Faithful documents.** Numbers keep their digits unless a converter
  touches them; the input is never modified; every conversion runs on its own
  state, so tasks can run in parallel.
- **No dependencies.** Standard library only, so the package is easy to vet
  and to embed.
