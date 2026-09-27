package t1k

import (
	_ "embed"
	"fmt"
)

// defaultConfigJSON is the mapping shipped with the package: the conversion
// of an EnerPlanET calculation payload into a MEME job. It is parsed once,
// when the package initialises, and shared by every task created without an
// explicit configuration.
//
//go:embed config/enerplanet-to-meme.json
var defaultConfigJSON []byte

var defaultConfig = mustLoadConfig(defaultConfigJSON)

func mustLoadConfig(data []byte) *Config {
	cfg, err := LoadConfig(data)
	if err != nil {
		panic(fmt.Sprintf("t1k: embedded default configuration is invalid: %v", err))
	}
	return cfg
}

// DefaultConfig returns the mapping configuration embedded in the package
// (config/enerplanet-to-meme.json in the repository).
func DefaultConfig() *Config { return defaultConfig }

// DefaultConfigJSON returns a copy of the embedded default configuration's
// JSON text, for example to write it out as a starting point for a custom
// mapping.
func DefaultConfigJSON() []byte { return append([]byte(nil), defaultConfigJSON...) }

// TransformTask converts JSON documents according to a mapping configuration.
// Transform applies the mapping as written; Reverse applies it backwards.
//
// A task holds no per-call state, so one task may be used concurrently; the
// intended pattern is nevertheless one task per conversion job, which is
// cheap (a task is a pointer to a shared, immutable configuration and two
// formatting options).
type TransformTask struct {
	cfg    *Config
	prefix string
	indent string
}

// Option configures a TransformTask.
type Option func(*TransformTask)

// WithConfig selects the mapping configuration; nil keeps the default.
func WithConfig(cfg *Config) Option {
	return func(t *TransformTask) {
		if cfg != nil {
			t.cfg = cfg
		}
	}
}

// WithIndent makes Transform and Reverse produce indented JSON, in the manner
// of json.MarshalIndent. The default is compact output.
func WithIndent(prefix, indent string) Option {
	return func(t *TransformTask) {
		t.prefix = prefix
		t.indent = indent
	}
}

// NewTransformTask creates a task bound to the default configuration unless
// WithConfig is given.
func NewTransformTask(opts ...Option) *TransformTask {
	t := &TransformTask{cfg: defaultConfig}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Transform converts input, a JSON document in the source structure, into the
// target structure described by the configuration.
func (t *TransformTask) Transform(input []byte) ([]byte, error) {
	return t.run(forward, input)
}

// Reverse converts input, a JSON document in the target structure, back into
// the source structure. It undoes Transform as far as the mapping allows:
// values without a counterpart on the other side come from the rules'
// reverse defaults and constants, or are left out.
func (t *TransformTask) Reverse(input []byte) ([]byte, error) {
	return t.run(reverse, input)
}

func (t *TransformTask) run(dir direction, input []byte) ([]byte, error) {
	in, err := decodeJSON(input)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInput, err)
	}
	out, err := execute(t.cfg, dir, in)
	if err != nil {
		return nil, err
	}
	data, err := encodeJSON(out, t.prefix, t.indent)
	if err != nil {
		return nil, fmt.Errorf("t1k: encode result: %w", err)
	}
	return data, nil
}
