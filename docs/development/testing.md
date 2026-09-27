# Testing

```bash
make test        # go test ./...
make test-race   # go test -race -shuffle=on ./...
make lint        # go vet + golangci-lint (v2, configuration in .golangci.yml)
make cover       # coverage summary
```

CI runs lint, the race tests, a build, a conversion of the example in both
directions and `govulncheck` on every push and pull request.

## Layers

| Tests | What they pin |
|---|---|
| `path_test.go` | parsing of every path form and its error messages, key template matching and rendering, matching with bound and unbound variables, concrete locations, reads and writes, compaction |
| `convert_test.go` | every converter forward and back (each case is run both ways), argument validation, chains, number formatting, JSON decoding and equality |
| `config_test.go` | a valid configuration with every construct, and one case per validation error with the message it must produce |
| `engine_test.go` | table cases for copy rules, constants and templates, arrays and keyed objects, `each` in iterate and join mode, nested scopes, conditions, defaults, error reporting and the no-aliasing guarantee; each case checks forward and reverse |
| `t1k_test.go` | the public API, golden files for the default mapping, the round-trip property, the concurrency guarantee |
| `cmd/t1k/main_test.go` | flags, files, standard streams, exit codes and error messages |

## Golden files

`testdata/meme-job.golden.json` and `testdata/enerplanet-reverse.golden.json`
are the outputs of the default mapping for `examples/enerplanet-calculation.json`
in both directions. After a deliberate change to the mapping:

```bash
make golden-update
git diff testdata
```

Review every changed line; the diff is the change's effect on the contract.

## The round-trip property

`TestDefaultMappingRoundTrip` converts the example forward, back and forward
again and requires the second job to equal the first apart from the building
without a connection, which the reverse cannot place (see
[the mapping page](../mappings/enerplanet-to-meme.md#the-reverse-direction)).
It then reverses the second job and requires that output to equal the first
reverse output, so the transformation is stable from the first round trip
on. A new lossy rule shows up here immediately.

## Checking against the consumers

MEME's decoder rejects unknown fields and its validators check the model, so
the surest test of a mapping change is MEME itself. In a clone of the MEME
repository, a small program in `cmd/` can decode a job with
`json.Decoder.DisallowUnknownFields`, call `job.Model.Validate()` and
`job.Experiment.Validate()`, and run each registered target's `ValidateJob`
to see the per-target gates and warnings; the compatibility table on the
mapping page was produced that way.

The reverse output can be checked against the simulation engine's
`servicehub/schemas/calliope.json` with any JSON Schema validator. It passes
once the placeholders the mapping documents (`country`, ...) are filled in.
