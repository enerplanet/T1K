package jsondoc

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// ScalarString renders a scalar the way it is used as an object key or a
// variable binding. Containers and null have no string form.
func ScalarString(v any) (string, bool) {
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

// TypeName names a value's JSON type for error messages: object, array,
// string, number, boolean or null.
func TypeName(v any) string {
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
		if _, ok := Number(v); ok {
			return "number"
		}
		return fmt.Sprintf("%T", v)
	}
}

// Describe renders a value for error messages: strings quoted, other scalars
// as they are, containers by their type name.
func Describe(v any) string {
	if s, ok := v.(string); ok {
		return strconv.Quote(s)
	}
	if s, ok := ScalarString(v); ok {
		return s
	}
	return TypeName(v)
}
