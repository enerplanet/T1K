# Go package

```go
import "github.com/enerplanet/T1K/pkg/t1k"
```

API reference: [pkg.go.dev/github.com/enerplanet/T1K/pkg/t1k](https://pkg.go.dev/github.com/enerplanet/T1K/pkg/t1k).

## Transforming

```go
task := t1k.NewTransformTask()
job, err := task.Transform(payload)   // []byte in, []byte out
back, err := task.Reverse(job)
```

`Transform` applies the configuration in the forward direction, `Reverse` in
the reverse direction. Both take a JSON document as a byte slice and return
one; the output is compact unless `WithIndent` is set.

A `TransformTask` binds a configuration and the output format. It holds no
per-call state: every call decodes its input, runs on a fresh output
document and encodes the result, so one task may be used from several
goroutines at once. Creating one task per conversion job, the intended
pattern, is cheap: a task is a pointer to a shared, immutable configuration
and two formatting options.

## Configurations

| Function | Purpose |
|---|---|
| `DefaultConfig()` | the embedded mapping (`config/enerplanet-to-meme.json`, shipped through the `config` package), parsed once when the package initialises |
| `DefaultConfigJSON()` | a copy of its JSON text, for example to write out as the starting point of a custom mapping |
| `LoadConfig(data []byte)` | parse and validate a configuration from its JSON text |
| `LoadConfigFile(path string)` | the same from a file; the error names the file |

A `Config` is immutable after loading and safe to share between tasks. Its
exported fields `Name`, `Version` and `Description` carry the metadata from
the file, and `Rules()` returns the number of top-level rules, for
diagnostics.

```go
cfg, err := t1k.LoadConfigFile("orders.json")
if err != nil {
	return err // wraps t1k.ErrConfig; the message names the offending rule
}
task := t1k.NewTransformTask(t1k.WithConfig(cfg), t1k.WithIndent("", "  "))
```

## Options

| Option | Effect |
|---|---|
| `WithConfig(cfg)` | use `cfg` instead of the default; `nil` keeps the default |
| `WithIndent(prefix, indent)` | indented output, as `json.MarshalIndent` would produce; empty strings restore compact output |

## Errors

Every error the package returns wraps one of three sentinels, so callers can
classify failures with `errors.Is`:

| Sentinel | When |
|---|---|
| `ErrConfig` | the configuration failed to parse or validate, or its file could not be read; the message names the rule and the field |
| `ErrInput` | the document is not valid JSON, or has content after the first document |
| `ErrRule` | a rule could not be applied: a converter received a value it cannot handle, a `match` guard failed, a `bind` path is missing from an element, or a variable bound to text is used as an array index; the message names the rule and the direction |

```go
out, err := task.Transform(in)
switch {
case errors.Is(err, t1k.ErrInput):
	// reject the request: the body is not JSON
case errors.Is(err, t1k.ErrRule):
	// the document does not fit the mapping; the message names the rule
}
```

## Working with decoded values

The package works on JSON text. If a program already holds a decoded value,
marshal it first; if it needs the result as a value, unmarshal the output.
This keeps the contract small and the number handling explicit: T1K preserves
the digits of every number it only moves (`1.0` stays `1.0`), which a round
trip through `float64` would not.
