![T1K banner](docs/assets/logos/t1k-banner-dark.png#gh-dark-mode-only)
![T1K banner](docs/assets/logos/t1k-banner-light.png#gh-light-mode-only)

# T1K

[![CI](https://github.com/enerplanet/T1K/actions/workflows/ci.yml/badge.svg)](https://github.com/enerplanet/T1K/actions/workflows/ci.yml)
[![MkDocs](https://github.com/enerplanet/T1K/actions/workflows/docs.yml/badge.svg)](https://enerplanet.github.io/T1K)
[![Go Reference](https://pkg.go.dev/badge/github.com/enerplanet/T1K/pkg/t1k.svg)](https://pkg.go.dev/github.com/enerplanet/T1K/pkg/t1k)

T1K converts one JSON structure into another, driven by a declarative mapping
configuration, and converts the result back again. It is a Go package with a
command-line tool, has no dependencies beyond the Go standard library, and
ships with the mapping that turns an [EnerPlanET](https://github.com/enerplanet/enerplanet)
calculation payload into a [MEME](https://github.com/enerplanet/meme) job.

**Documentation:** [enerplanet.github.io/T1K](https://enerplanet.github.io/T1K)

## How it works

A mapping is a JSON file of rules. Each rule pairs a path in the source
document with a path in the target document; the same rule is read forwards by
`Transform` and backwards by `Reverse`, so one configuration gives both
directions.

```json
{
  "name": "orders-to-invoices",
  "rules": [
    {"from": "order.number", "to": "invoice.reference"},
    {"from": "order.total_cents", "to": "invoice.total", "convert": {"linear": {"divisor": 100}}},
    {"to": "invoice.currency", "value": "EUR"},
    {
      "each": {"from": "order.lines[$i]", "to": "invoice.items{line_$i}"},
      "rules": [
        {"from": "sku", "to": "article"},
        {"from": "qty", "to": "quantity", "convert": "number"}
      ]
    }
  ]
}
```

Paths address object keys, array elements and object entries; variables such
as `$i` iterate on one side and are substituted on the other, which is what
lets a rule turn an array into a keyed object and back. Converters are
invertible (`linear`, `lookup`, `datetime`, `number`, `string`, `absent`), and
`each` groups nested rules relative to a pair of elements. The
[configuration reference](https://enerplanet.github.io/T1K/configuration/overview/)
describes every construct.

## Installation

```bash
go install github.com/enerplanet/T1K/cmd/t1k@latest   # the command
go get github.com/enerplanet/T1K/pkg/t1k               # the package
```

Go 1.23 or newer is required. `make build` produces `bin/t1k` from a clone.

## Command line

```bash
# EnerPlanET calculation payload -> MEME job, with the embedded mapping
t1k -in examples/enerplanet-calculation.json -out meme-job.json

# and back again
t1k -reverse -in meme-job.json -out calculation.json

# any other mapping, reading standard input and writing standard output
cat input.json | t1k -config my-mapping.json > output.json
```

| Flag | Meaning |
|---|---|
| `-config FILE` | mapping configuration; without it the embedded EnerPlanET to MEME mapping is used |
| `-in FILE`, `-out FILE` | input and output documents; default standard input and output |
| `-reverse` | apply the reverse transformation |
| `-compact` | compact instead of indented output |
| `-print-config` | print the embedded mapping, as a starting point for your own |
| `-version` | print the version |

Exit status 1 reports an invalid configuration, input or a failing rule (the
message names the rule and the direction); 2 reports a usage error.

## Go package

```go
import "github.com/enerplanet/T1K/pkg/t1k"

task := t1k.NewTransformTask()          // the embedded EnerPlanET -> MEME mapping
memeJob, err := task.Transform(payload) // payload: the calculation JSON
back, err := task.Reverse(memeJob)

cfg, err := t1k.LoadConfigFile("my-mapping.json")
custom := t1k.NewTransformTask(t1k.WithConfig(cfg), t1k.WithIndent("", "  "))
```

The default configuration is parsed when the package initialises. A
`TransformTask` carries no per-call state, so tasks can run in parallel; the
intended pattern is one task per conversion job. Errors wrap `ErrConfig`,
`ErrInput` or `ErrRule` for `errors.Is`.

## The EnerPlanET to MEME mapping

[`config/enerplanet-to-meme.json`](config/enerplanet-to-meme.json) maps the
payload EnerPlanET sends to its simulation webservice onto MEME's canonical
model: every topology feature becomes a node, every connection a transmission
arc, buildings get a demand technology from their annual consumption, attached
technologies (`pv_supply`, `battery_storage`, ...) become MEME technologies
with kW converted to MW, and transformers become the grid connection. The
[mapping page](https://enerplanet.github.io/T1K/mappings/enerplanet-to-meme/)
lists every field, the unit conversions and what the reverse restores.
[`examples/`](examples/) holds a payload and the job it produces.

## Development

```bash
make test        # go test ./...
make test-race   # with the race detector, shuffled
make lint        # go vet + golangci-lint
make example     # convert examples/enerplanet-calculation.json both ways
```

Golden files under `pkg/t1k/testdata/` pin the default mapping's output; refresh them
with `make golden-update` after a deliberate change. See
[CONTRIBUTING.md](CONTRIBUTING.md) for the workflow and the commit convention.

## License

[MIT](LICENSE). Copyright (c) 2026 BigGeoData & Spatial AI, Technische Hochschule Deggendorf.
