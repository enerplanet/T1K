# Go package

```go
import t1k "github.com/enerplanet/T1K"
```

Package documentation: [pkg.go.dev/github.com/enerplanet/T1K](https://pkg.go.dev/github.com/enerplanet/T1K).

## Transforming

```go
task := t1k.NewTransformTask()
job, err := task.Transform(payload)   // []byte in, []byte out
back, err := task.Reverse(job)
```

`Transform` applies the configuration as written, `Reverse` applies it
backwards. Both take and return JSON documents as byte slices; the output is
compact unless `WithIndent` is set.

A `TransformTask` is the unit of work: it binds a configuration and the
output format. It holds no per-call state, so one task may be used from
several goroutines, and creating one per conversion job (the intended
pattern) is cheap.

## Configurations

| Function | Purpose |
|---|---|
| `DefaultConfig()` | the embedded mapping (`config/enerplanet-to-meme.json`), parsed when the package initialises |
| `DefaultConfigJSON()` | a copy of its JSON text |
| `LoadConfig(data []byte)` | parse and validate a configuration |
| `LoadConfigFile(path string)` | the same from a file |

A `Config` is immutable after loading and safe to share. Its exported fields
`Name`, `Version` and `Description` carry the metadata from the file.

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
| `WithIndent(prefix, indent)` | indented output, as `json.MarshalIndent` would produce |

## Errors

Every error wraps one of three sentinels, so callers can classify them with
`errors.Is`:

| Sentinel | When |
|---|---|
| `ErrConfig` | the configuration failed to parse or validate |
| `ErrInput` | the document is not valid JSON |
| `ErrRule` | a rule could not be applied: a converter received a value it cannot handle, a `bind` path is missing from an element, a key does not fit an array index |

```go
out, err := task.Transform(in)
switch {
case errors.Is(err, t1k.ErrInput):
	// reject the request
case errors.Is(err, t1k.ErrRule):
	// the document does not fit the mapping; the message names the rule
}
```

## Working with decoded values

The package works on JSON text. If your program already holds a decoded
value, marshal it first; if you need the result as a value, unmarshal the
output. This keeps the package's contract small and makes the number
handling explicit: T1K preserves the digits of every number it merely moves
(`1.0` stays `1.0`), which a round trip through `float64` would not.
