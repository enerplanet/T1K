package convert

import (
	"fmt"
	"math"

	"github.com/enerplanet/T1K/internal/jsondoc"
)

// argReader reads a converter's typed arguments and, through done, rejects
// the ones the converter does not know, so a misspelt argument fails when
// the configuration is loaded rather than being silently ignored.
type argReader struct {
	args map[string]any
	seen map[string]bool
}

func newArgReader(args map[string]any) *argReader {
	return &argReader{args: args, seen: map[string]bool{}}
}

func (r *argReader) get(key string) (any, bool) {
	r.seen[key] = true
	v, ok := r.args[key]
	return v, ok
}

func (r *argReader) float(key string, def float64) (float64, error) {
	v, ok := r.get(key)
	if !ok {
		return def, nil
	}
	f, ok := jsondoc.Number(v)
	if !ok {
		return 0, fmt.Errorf("argument %q must be a number", key)
	}
	return f, nil
}

// intOpt reads an optional integer; the second result reports its presence.
func (r *argReader) intOpt(key string) (int, bool, error) {
	v, ok := r.get(key)
	if !ok {
		return 0, false, nil
	}
	f, ok := jsondoc.Number(v)
	if !ok || f != math.Trunc(f) {
		return 0, false, fmt.Errorf("argument %q must be an integer", key)
	}
	return int(f), true, nil
}

func (r *argReader) str(key, def string) (string, error) {
	v, ok := r.get(key)
	if !ok {
		return def, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("argument %q must be a string", key)
	}
	return s, nil
}

func (r *argReader) boolean(key string, def bool) (bool, error) {
	v, ok := r.get(key)
	if !ok {
		return def, nil
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("argument %q must be a boolean", key)
	}
	return b, nil
}

// done fails on any argument no read has consumed.
func (r *argReader) done() error {
	for k := range r.args {
		if !r.seen[k] {
			return fmt.Errorf("unknown argument %q", k)
		}
	}
	return nil
}
