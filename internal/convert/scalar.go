package convert

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/enerplanet/T1K/internal/jsondoc"
)

// identity passes the value through in both directions.
type identity struct{}

func newIdentity(args map[string]any) (Converter, error) {
	return identity{}, newArgReader(args).done()
}

func (identity) Forward(v any, present bool) (any, bool, error) { return v, present, nil }
func (identity) Reverse(v any, present bool) (any, bool, error) { return v, present, nil }

// number accepts a JSON number or a numeric string and yields a JSON number,
// in both directions. A number keeps its exact digits; a string is normalised.
type number struct{}

func newNumber(args map[string]any) (Converter, error) {
	return number{}, newArgReader(args).done()
}

func (number) Forward(v any, present bool) (any, bool, error) { return asNumber(v, present) }
func (number) Reverse(v any, present bool) (any, bool, error) { return asNumber(v, present) }

func asNumber(v any, present bool) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	if n, ok := v.(json.Number); ok {
		return n, true, nil
	}
	f, ok := jsondoc.Float(v)
	if !ok {
		return nil, false, fmt.Errorf("number: %s %s is not numeric", jsondoc.TypeName(v), jsondoc.Describe(v))
	}
	n, err := jsondoc.FormatNumber(f)
	return n, true, err
}

// stringConv renders a scalar as a string. The reverse turns a string that
// reads as a number or boolean back into one and leaves other strings alone;
// a non-string passes through.
type stringConv struct{}

func newString(args map[string]any) (Converter, error) {
	return stringConv{}, newArgReader(args).done()
}

func (stringConv) Forward(v any, present bool) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	s, ok := jsondoc.ScalarString(v)
	if !ok {
		return nil, false, fmt.Errorf("string: cannot render %s as a string", jsondoc.TypeName(v))
	}
	return s, true, nil
}

func (stringConv) Reverse(v any, present bool) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	s, ok := v.(string)
	if !ok {
		return v, true, nil
	}
	return parseScalar(s), true, nil
}

// parseScalar reads a boolean or a finite number out of a string, falling
// back to the string itself.
func parseScalar(s string) any {
	switch s {
	case "true":
		return true
	case "false":
		return false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return s
	}
	if n, err := jsondoc.FormatNumber(f); err == nil {
		return n
	}
	return s
}
