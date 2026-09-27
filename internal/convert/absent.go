package convert

import (
	"errors"
	"strings"

	"github.com/enerplanet/T1K/internal/jsondoc"
)

// absent treats the listed values as "not present" in both directions, so a
// placeholder such as "inf" or null is dropped instead of copied. A rule's
// default or reverse_default restores the placeholder in the other direction.
type absent struct {
	values     []any
	ignoreCase bool
}

func newAbsent(args map[string]any) (Converter, error) {
	r := newArgReader(args)
	raw, ok := r.get("values")
	if !ok {
		return nil, errors.New("argument \"values\" is required")
	}
	values, ok := raw.([]any)
	if !ok || len(values) == 0 {
		return nil, errors.New("\"values\" must be a non-empty array")
	}
	ignoreCase, err := r.boolean("ignore_case", false)
	if err != nil {
		return nil, err
	}
	return &absent{values: values, ignoreCase: ignoreCase}, r.done()
}

func (c *absent) Forward(v any, present bool) (any, bool, error) { return c.filter(v, present) }
func (c *absent) Reverse(v any, present bool) (any, bool, error) { return c.filter(v, present) }

// filter makes the value absent when it is one of the listed values.
func (c *absent) filter(v any, present bool) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	for _, listed := range c.values {
		if c.matches(listed, v) {
			return nil, false, nil
		}
	}
	return v, true, nil
}

func (c *absent) matches(listed, v any) bool {
	if jsondoc.Equal(listed, v) {
		return true
	}
	if !c.ignoreCase {
		return false
	}
	ls, lok := listed.(string)
	vs, vok := v.(string)
	return lok && vok && strings.EqualFold(ls, vs)
}
