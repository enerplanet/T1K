![T1K banner](assets/logos/t1k-banner-dark.png#only-dark)
![T1K banner](assets/logos/t1k-banner-light.png#only-light)

# T1K

T1K converts one JSON structure into another from a declarative mapping
configuration, and converts the result back again. It is a Go package with a
command-line tool, depends only on the Go standard library, and ships with the
mapping that turns an [EnerPlanET](https://github.com/enerplanet/enerplanet)
calculation payload into a [MEME](https://github.com/enerplanet/meme) job.

## The core idea

A mapping configuration is a list of rules. Each rule pairs a path in the
source document with a path in the target document, optionally passing the
value through an invertible converter:

```json
{"from": "order.total_cents", "to": "invoice.total", "convert": {"linear": {"divisor": 100}}}
```

The forward direction (`Transform`) reads the rule from `from` to `to`; the
reverse direction (`Reverse`) reads it from `to` to `from` and runs the
converter backwards. Every rule is written once and read in both directions,
so one configuration yields both transformations and they cannot drift apart.

Paths may contain variables. A variable iterates on the side that is read and
is substituted on the side that is written, which is how one rule moves a
whole collection and restructures it:

```json
{
  "each": {"from": "order.lines[$i]", "to": "invoice.items{line_$i}"},
  "rules": [
    {"from": "sku", "to": "article"},
    {"from": "qty", "to": "quantity", "convert": "number"}
  ]
}
```

This rule turns an array of lines into an object keyed `line_0`, `line_1`,
... and, in reverse, the object back into the array.

## Where to go next

- [Installation](usage/installation.md) and the [Quickstart](usage/quickstart.md)
- Using the [command line](usage/cli.md) or the [Go package](usage/library.md)
- The mapping language: [overview](configuration/overview.md),
  [paths and variables](configuration/paths.md), [rules](configuration/rules.md),
  [converters](configuration/converters.md) and
  [the reverse direction](configuration/reverse.md)
- The shipped mapping: [EnerPlanET to MEME](mappings/enerplanet-to-meme.md)
- For contributors: [architecture](development/architecture.md) and
  [testing](development/testing.md)

## Design goals

- **Reversible by construction.** A rule is a relation between two documents,
  not a procedure. Converters are invertible, and what cannot be inverted (a
  field the target has no place for) is restored from a declared default.
- **Strict configurations.** Unknown keys, unbalanced variables, unknown
  converters and malformed conditions are rejected when a configuration is
  loaded, and every message names the position of the offending rule.
- **Faithful documents.** A number keeps its digits unless a converter
  computes it; the input document is never modified; every conversion runs on
  its own state, so tasks can run in parallel.
- **No dependencies.** The standard library only, so the package is easy to
  audit and to embed.

!!! note "Status"
    T1K is pre-1.0. Before 1.0, a minor release may change the configuration
    format or the keys the shipped mapping generates; such changes are listed
    in the [changelog](https://github.com/enerplanet/T1K/blob/main/CHANGELOG.md)
    with the keys concerned.
