package jsondoc

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Number reads a JSON number: a json.Number, or a Go number supplied by a
// caller building trees programmatically. Strings are not numbers here, even
// when they hold digits.
func Number(v any) (float64, bool) {
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

// Float is the lenient reading numeric converters use: a JSON number, or a
// string holding a finite decimal number.
func Float(v any) (float64, bool) {
	if f, ok := Number(v); ok {
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

// FormatNumber renders a float as a json.Number in plain decimal notation, so
// 1e-7 becomes 0.0000001 and never an exponent. The value is first rounded
// to 15 significant digits, which removes the binary floating-point noise of
// arithmetic (8889.3 / 1000 is 8.889299999999999 in float64) while keeping
// every digit a number in this domain carries.
func FormatNumber(f float64) (json.Number, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "", fmt.Errorf("%v is not a finite number", f)
	}
	rounded, err := strconv.ParseFloat(strconv.FormatFloat(f, 'g', 15, 64), 64)
	if err != nil {
		return "", err
	}
	return json.Number(strconv.FormatFloat(rounded, 'f', -1, 64)), nil
}
