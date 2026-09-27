package t1k

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

// The engine works on the generic JSON tree produced by encoding/json:
// map[string]any for objects, []any for arrays, string, bool, nil and
// json.Number for scalars. Numbers are kept as json.Number so that a value
// which is only moved (never converted) is emitted with exactly the digits it
// came in with.

// decodeJSON parses a JSON document into the generic tree.
func decodeJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after the JSON document")
	}
	return v, nil
}

// encodeJSON serialises a generic tree. An empty prefix and indent produce
// compact output. HTML escaping is disabled so URLs and comparison operators
// survive unchanged.
func encodeJSON(v any, prefix, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent(prefix, indent)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// deepCopy clones containers so that the output never aliases the input.
// Scalars are immutable and returned as they are.
func deepCopy(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, e := range x {
			m[k] = deepCopy(e)
		}
		return m
	case []any:
		s := make([]any, len(x))
		for i, e := range x {
			s[i] = deepCopy(e)
		}
		return s
	default:
		return v
	}
}

// numericValue reads a JSON number (json.Number, or a Go number supplied by a
// caller building trees programmatically). Strings are not numbers here.
func numericValue(v any) (float64, bool) {
	switch x := v.(type) {
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case int32:
		return float64(x), true
	}
	return 0, false
}

// toFloat is the lenient reading used by numeric converters: a JSON number or
// a string holding a finite decimal number.
func toFloat(v any) (float64, bool) {
	if f, ok := numericValue(v); ok {
		return f, true
	}
	s, ok := v.(string)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// numberValue formats a float as a json.Number using plain decimal notation,
// so 1e-7 is emitted as 0.0000001 and never as an exponent. The value is first
// rounded to 15 significant digits, which removes the binary floating-point
// noise of arithmetic (8889.3 / 1000 is 8.889299999999999 in float64) while
// keeping every digit a JSON number in this domain carries.
func numberValue(f float64) (json.Number, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "", fmt.Errorf("%v is not a finite number", f)
	}
	rounded, err := strconv.ParseFloat(strconv.FormatFloat(f, 'g', 15, 64), 64)
	if err != nil {
		return "", err
	}
	return json.Number(strconv.FormatFloat(rounded, 'f', -1, 64)), nil
}

// scalarString renders a scalar the way it is used as a map key or variable
// binding. Containers and null have no string form.
func scalarString(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case json.Number:
		return x.String(), true
	case bool:
		return strconv.FormatBool(x), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case int:
		return strconv.Itoa(x), true
	case int64:
		return strconv.FormatInt(x, 10), true
	}
	return "", false
}

// equalJSON compares two trees structurally. Numbers compare by value, so 60
// and 60.0 are equal whatever their textual form; a numeric string never
// equals a number.
func equalJSON(a, b any) bool {
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, av := range x {
			bv, ok := y[k]
			if !ok || !equalJSON(av, bv) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !equalJSON(x[i], y[i]) {
				return false
			}
		}
		return true
	case nil:
		return b == nil
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	default:
		fa, oka := numericValue(a)
		fb, okb := numericValue(b)
		return oka && okb && fa == fb
	}
}

// typeName names a JSON value's type for error messages.
func typeName(v any) string {
	switch v.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		if _, ok := numericValue(v); ok {
			return "number"
		}
		return fmt.Sprintf("%T", v)
	}
}

func sortStrings(s []string) { sort.Strings(s) }
