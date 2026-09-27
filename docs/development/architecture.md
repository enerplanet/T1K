# Architecture

T1K is a small Go module with one public package and four internal ones,
each with a single responsibility. It depends only on the standard library.

```
pkg/t1k              the public API: TransformTask, Config, errors
config               the shipped mapping files, embedded
cmd/t1k              the command line
internal/mapping     the mapping language: compiling rules, evaluating them
internal/jsonpath    path expressions: parsing, matching, locations
internal/convert     the invertible value converters
internal/jsondoc     the JSON document model every layer works on
```

`pkg/` follows the layout of the organisation's shared Go libraries; the
Go team's own layout guide would also accept the package at the module root.

## Packages

| Package | Responsibility | Depends on |
|---|---|---|
| `pkg/t1k` | `TransformTask` with `Transform` and `Reverse`, `Config` loading, the default configuration parsed at initialisation, the error sentinels | `mapping`, `jsondoc`, `config` |
| `config` | the mapping files under `config/`, embedded with `go:embed` and named | nothing |
| `internal/mapping` | `Compile` turns a configuration into a `Program` of rules; `Program.Run` applies them in a `Direction` | `jsonpath`, `convert`, `jsondoc` |
| `internal/jsonpath` | `Parse` reads a path expression; `Expand` matches it against a document, binding variables; `Resolve`, `Get` and `Set` work with concrete locations; `Compact` removes the holes out-of-order writes leave | `jsondoc` (tests only) |
| `internal/convert` | `Parse` builds a `Converter` from a rule's `convert`; each converter implements `Forward` and `Reverse` | `jsondoc` |
| `internal/jsondoc` | `Decode` and `Encode` with numbers kept as `json.Number`, `DeepCopy`, structural `Equal`, number parsing and formatting, type names for messages | nothing |
| `cmd/t1k` | flags, files and exit codes around the public API | `pkg/t1k` |

Dependencies point downwards only. The public package knows nothing about
paths or converters; the mapping package knows nothing about files or
formatting.

## Pipeline

```
input bytes ──jsondoc.Decode──▶ tree ──Program.Run(direction)──▶ tree ──jsondoc.Encode──▶ output bytes
```

`Decode` keeps numbers as `json.Number`, so a value that is only moved is
emitted with its original digits. `Run` creates a `run` per call with the
direction, the input tree and an output tree that starts as an empty object;
nothing is shared between calls, which is why a task can be used
concurrently.

## Compilation

`mapping.Compile` parses the JSON into a tree of `rule` values of three
kinds: copy, constant and scope (`each`). One file compiles each kind
(`compile.go` for copy and constant rules, `compile_scope.go` for scopes and
their `bind`, `condition.go` for `when`), reading raw JSON through the
`fields` helper so every mistake is reported with the field's name and the
rule's position. Compilation resolves every path, key template, converter
chain and condition once and checks the variable discipline that makes rules
reversible:

- a copy rule uses the same unbound variables on both sides;
- a constant or template uses only variables an enclosing `each` binds;
- an `each` binds every variable of `to` through `from` or `bind`, and
  records whether the reverse must *join* (some variable of `from` is not in
  `to`) or can iterate.

`use` splices a definition's rules at the use site and compiles them there,
so a definition sees the variables bound where it is used; cycles are
detected with a stack of open definitions.

## Evaluation

A `frame` is where the rules currently apply: the bindings, the input
element relative paths read from, and the output `Location` relative paths
write to. The root frame has no bindings, the whole input and the root
location.

The direction only decides which side of a rule is read and which is
written, which converter direction runs, which default and which conditions
apply, and which of a scope's two modes runs backwards (`run.go` holds the
copy and constant rules, `run_scope.go` the scopes). Reading uses
`Path.Expand`, which walks the tree with the current bindings, iterating
unbound variables and selecting with bound ones, and returns one `Match` per
addressed value; a concrete path returns one match, present or not, so a
rule can tell absence from "nothing matched". Writing uses `Path.Resolve`,
which substitutes every variable into a concrete location, and `Set`, which
creates containers on the way and grows arrays with holes that `Compact`
removes at the end.

## Converters

A converter implements `Forward` and `Reverse` over a `(value, present)`
pair, so it can turn a value into absence and back. A chain runs its steps
in order forward and backwards in reverse. Each converter lives in its own
file; factories read their arguments through a strict reader that rejects
unknown ones, so a misspelt argument fails at load time.

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
- **Small functions:** every function does one thing and says so in its
  comment; the engine is a set of named steps, not one loop.
