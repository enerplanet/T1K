package t1k

import (
	"fmt"

	"github.com/enerplanet/T1K/internal/jsondoc"
	"github.com/enerplanet/T1K/internal/mapping"
)

// TransformTask converts JSON documents according to a mapping configuration.
// Transform applies the mapping as written; Reverse applies it backwards.
//
// A task holds no per-call state, so one task may be used concurrently; the
// intended pattern is nevertheless one task per conversion job, which is
// cheap: a task is a pointer to a shared, immutable configuration and two
// formatting options.
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
// target structure the configuration describes.
func (t *TransformTask) Transform(input []byte) ([]byte, error) {
	return t.run(mapping.Forward, input)
}

// Reverse converts input, a JSON document in the target structure, back into
// the source structure. It undoes Transform as far as the mapping allows:
// values without a counterpart on the other side come from the rules'
// reverse defaults and constants, or are left out.
func (t *TransformTask) Reverse(input []byte) ([]byte, error) {
	return t.run(mapping.Reverse, input)
}

// run decodes the input, applies the program in the given direction and
// encodes the result with the task's formatting.
func (t *TransformTask) run(dir mapping.Direction, input []byte) ([]byte, error) {
	in, err := jsondoc.Decode(input)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInput, err)
	}
	out, err := t.cfg.program.Run(dir, in)
	if err != nil {
		return nil, err
	}
	data, err := jsondoc.Encode(out, t.prefix, t.indent)
	if err != nil {
		return nil, fmt.Errorf("t1k: encode result: %w", err)
	}
	return data, nil
}
