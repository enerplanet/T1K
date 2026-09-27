// Package convert implements the value converters of T1K's mapping
// language. A converter changes a value on its way from one side of a rule to
// the other, and every converter is invertible: Forward is used by the
// forward transformation, Reverse by the reverse one, and Reverse(Forward(v))
// yields v again up to the documented loss of representation, such as the
// decimals a rounding keeps.
//
// A converter may also turn a value into "absent", meaning the target key is
// not written, and absent back into a value; the (value, present) pair every
// method takes and returns carries that.
//
// A rule's "convert" is one step or a list of steps applied in order. A step
// is a converter name ("number") or a one-key object carrying its arguments
// ({"linear": {"divisor": 1000}}). The reverse direction runs the steps
// backwards.
package convert

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/enerplanet/T1K/internal/jsondoc"
)

// Converter transforms a value in both directions. present is false for an
// absent value; a converter that returns present=false makes the value
// absent.
type Converter interface {
	Forward(v any, present bool) (any, bool, error)
	Reverse(v any, present bool) (any, bool, error)
}

// factory builds a converter from the arguments of its step object.
type factory func(args map[string]any) (Converter, error)

var factories = map[string]factory{
	"identity": newIdentity,
	"number":   newNumber,
	"string":   newString,
	"linear":   newLinear,
	"lookup":   newLookup,
	"datetime": newDatetime,
	"absent":   newAbsent,
}

// Identity passes values through unchanged; it is the converter of a rule
// without "convert".
var Identity Converter = identity{}

// Names lists the registered converters in alphabetical order.
func Names() []string {
	names := make([]string, 0, len(factories))
	for n := range factories {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Parse builds the converter a rule's "convert" describes: a single step, or
// a list of steps that becomes a chain.
func Parse(spec json.RawMessage) (Converter, error) {
	decoded, err := jsondoc.Decode(spec)
	if err != nil {
		return nil, err
	}
	steps, isList := decoded.([]any)
	if !isList {
		return parseStep(decoded)
	}
	return parseChain(steps)
}

func parseChain(steps []any) (Converter, error) {
	if len(steps) == 0 {
		return nil, errors.New("convert: empty list")
	}
	c := make(chain, 0, len(steps))
	for i, s := range steps {
		conv, err := parseStep(s)
		if err != nil {
			return nil, fmt.Errorf("convert step %d: %w", i, err)
		}
		c = append(c, conv)
	}
	if len(c) == 1 {
		return c[0], nil
	}
	return c, nil
}

// parseStep interprets one step: a converter name, or a one-key object whose
// key is the name and whose value holds the arguments.
func parseStep(spec any) (Converter, error) {
	switch s := spec.(type) {
	case string:
		return build(s, nil)
	case map[string]any:
		return parseStepObject(s)
	default:
		return nil, fmt.Errorf("a converter step is a name or a one-key object, got %s", jsondoc.TypeName(spec))
	}
}

func parseStepObject(obj map[string]any) (Converter, error) {
	if len(obj) != 1 {
		return nil, errors.New("a converter step is a name or a one-key object {\"name\": {args}}")
	}
	for name, raw := range obj {
		args, ok := raw.(map[string]any)
		if !ok && raw != nil {
			return nil, fmt.Errorf("converter %q: arguments must be an object", name)
		}
		return build(name, args)
	}
	return nil, errors.New("unreachable")
}

func build(name string, args map[string]any) (Converter, error) {
	factory, ok := factories[name]
	if !ok {
		return nil, fmt.Errorf("unknown converter %q (available: %s)", name, strings.Join(Names(), ", "))
	}
	if args == nil {
		args = map[string]any{}
	}
	conv, err := factory(args)
	if err != nil {
		return nil, fmt.Errorf("converter %q: %w", name, err)
	}
	return conv, nil
}

// chain applies converters in sequence; the reverse direction walks it
// backwards.
type chain []Converter

func (c chain) Forward(v any, present bool) (any, bool, error) {
	var err error
	for _, conv := range c {
		if v, present, err = conv.Forward(v, present); err != nil {
			return nil, false, err
		}
	}
	return v, present, nil
}

func (c chain) Reverse(v any, present bool) (any, bool, error) {
	var err error
	for i := len(c) - 1; i >= 0; i-- {
		if v, present, err = c[i].Reverse(v, present); err != nil {
			return nil, false, err
		}
	}
	return v, present, nil
}
