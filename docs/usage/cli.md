# Command line

```
t1k [-config FILE] [-reverse] [-in FILE] [-out FILE] [-compact]
```

| Flag | Meaning |
|---|---|
| `-config FILE` | the mapping configuration; without it the embedded EnerPlanET to MEME mapping is used |
| `-in FILE` | the input document; default: standard input |
| `-out FILE` | the output document; default: standard output |
| `-reverse` | apply the reverse transformation (target structure to source structure) |
| `-compact` | write compact JSON; the default is two-space indentation |
| `-print-config` | print the embedded mapping and exit, as a starting point for a custom one |
| `-version` | print the version and exit |
| `-h`, `-help` | print the usage |

The output always ends with a newline. Object keys are written in sorted
order (see [the evaluation model](../configuration/overview.md#evaluation-model)).

## Exit status

| Status | Meaning |
|---|---|
| 0 | success |
| 1 | the configuration is invalid, the input is not JSON, a rule failed, or a file could not be read or written; the message on standard error says which |
| 2 | usage error: an unknown flag or a stray argument |

A failing rule is reported with its position in the configuration and the
direction, for example:

```
t1k: t1k: rule failed: rules[16].rules[5] (forward): length: number: string "far" is not numeric
```

`rules[16].rules[5]` is the sixth nested rule of the seventeenth top-level
rule; rules spliced in from a definition carry the definition's name, as in
`rules[17].rules[0](definitions.feature)[3]`.

## Examples

```bash
# forward and back with the embedded mapping
t1k -in calculation.json -out meme-job.json
t1k -reverse -in meme-job.json -out calculation.json

# a custom mapping in a pipeline
curl -s https://example.invalid/order/17 | t1k -config orders.json -compact | jq .

# start a new mapping from the embedded one
t1k -print-config > my-mapping.json
```

## Building

`make build` produces `bin/t1k` and stamps the version from `git describe`.
A plain `go build ./cmd/t1k` reports the version as `dev`; to set it
yourself, pass `-ldflags "-X main.version=1.2.3"`.
