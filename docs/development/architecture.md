# Architecture

T1K is the `t1k` package under `pkg/t1k`, the `config` package that ships
the mapping files, and the `cmd/t1k` command. It depends only on the
standard library. `pkg/` follows the layout of the organisation's shared Go
libraries; the Go team's own layout guide would also accept the package at
the module root.

## Files

| File | Responsibility |
|---|---|
| `pkg/t1k/t1k.go` | the public API: `TransformTask`, options, `DefaultConfig`, the default mapping parsed at package initialisation |
| `pkg/t1k/config.go` | parsing and strict validation of a mapping into compiled rules; `definitions`/`use` splicing; variable balance checks |
| `pkg/t1k/path.go` | the path language: parsing, key templates, matching with bindings, concrete locations, reads and writes on the JSON tree, array compaction |
| `pkg/t1k/convert.go` | the converter registry and the invertible converters |
| `pkg/t1k/engine.go` | applying compiled rules to a document in either direction |
| `pkg/t1k/value.go` | the generic JSON tree: decoding with `json.Number`, encoding, deep copy, structural equality, number formatting |
| `cmd/t1k/main.go` | the command line: flags, files, exit codes |
| `config/config.go` | the `config` package: embeds every mapping in `config/` with `go:embed` and names the default one |
| `config/enerplanet-to-meme.json` | the default mapping |

## Pipeline

```
input bytes ──decodeJSON──▶ tree ──execute(cfg, direction)──▶ tree ──encodeJSON──▶ output bytes
```

`decodeJSON` keeps numbers as `json.Number`, so a value that is only moved is
emitted with its original digits. `execute` creates a `run` per call with the
direction, the input tree and an output tree that starts as an empty object;
nothing is shared between calls, which is why a task can be used
concurrently.

## Compiled rules

`LoadConfig` turns the JSON into a tree of `rule` values of three kinds:
copy, constant and scope (`each`). Compilation resolves every path, key
template, converter chain and condition once and checks the variable
discipline that makes rules reversible:

- a copy rule uses the same unbound variables on both sides;
- a constant or template uses only variables an enclosing `each` binds;
- an `each` binds every variable of `to` through `from` or `bind`, and
  records whether the reverse must *join* (some variable of `from` is not in
  `to`) or can iterate.

`use` splices a definition's rules at the use site and compiles them there,
so a definition sees the variables bound where it is used; cycles are
detected with a stack of open definitions.

## The engine

A `frame` is where the rules currently apply: the bindings, the input element
relative paths read from, and the output *location* (a resolved list of keys
and indices) relative paths write to. The root frame has no bindings, the
whole input and the root location.

Reading uses `path.expand`, which walks the tree with the current bindings,
iterating unbound variables and selecting with bound ones, and returns one
`match` per addressed value (a concrete path returns one match, present or
not). Writing uses `path.resolve`, which substitutes every variable into a
concrete location, and `setAt`, which creates containers on the way and grows
arrays with `hole` markers that `compact` removes at the end.

The direction only decides which side is read and which is written, which
converter direction runs, which default and which conditions apply, and
which of a scope's two modes runs backwards. `applyScope` enumerates the
source elements and applies the nested rules in a new frame per element;
`applyJoin` is the reverse of a value-keyed scope and enumerates the elements
earlier rules created in the output instead.

## Converters

A converter implements `forward` and `reverse` over `(value, present)`, so it
can turn a value into absence and back. A chain runs its steps in order
forward and backwards in reverse. Factories parse arguments strictly and
reject unknown ones, so a misspelt argument fails at load time.

## Principles

- **Bidirectional by construction:** there is one rule set and one engine;
  nothing is written twice.
- **Strict at load, tolerant at run:** a configuration error is reported
  before any document is touched; an absent value at run time is not an
  error, a value of the wrong type for a converter is.
- **Faithful documents:** numbers keep their text, the input is never
  modified, the output never aliases the input.
- **Deterministic output:** sorted keys, no exponent notation, stable
  compaction.
