// Package jsondoc defines the document model the rest of T1K operates on: the
// generic tree that encoding/json produces for a value of type any, with
// numbers kept as json.Number.
//
// Objects are map[string]any, arrays are []any, and scalars are string, bool,
// nil and json.Number. Because numbers stay in their textual form, a value
// that a transformation only moves is written out with exactly the digits it
// came in with. Programs that assemble documents in Go may also use float64
// and integer values; every function here accepts them as numbers.
package jsondoc

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// Decode parses a JSON document into the generic tree. Trailing content after
// the document is an error, so a concatenation of two documents is rejected
// rather than silently truncated.
func Decode(data []byte) (any, error) {
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

// Encode serialises a tree without a trailing newline. An empty prefix and
// indent produce compact output. HTML escaping is disabled so that URLs and
// comparison operators such as "<=" survive unchanged.
func Encode(v any, prefix, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent(prefix, indent)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// DeepCopy clones objects and arrays recursively, so that a document built
// from another never shares memory with it. Scalars are immutable and are
// returned as they are.
func DeepCopy(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, e := range x {
			m[k] = DeepCopy(e)
		}
		return m
	case []any:
		s := make([]any, len(x))
		for i, e := range x {
			s[i] = DeepCopy(e)
		}
		return s
	default:
		return v
	}
}

// Equal compares two trees structurally. Numbers compare by value, so 60 and
// 60.0 are equal whatever their textual form; a numeric string never equals a
// number.
func Equal(a, b any) bool {
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		return ok && equalObjects(x, y)
	case []any:
		y, ok := b.([]any)
		return ok && equalArrays(x, y)
	case nil:
		return b == nil
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	default:
		fa, oka := Number(a)
		fb, okb := Number(b)
		return oka && okb && fa == fb
	}
}

func equalObjects(x, y map[string]any) bool {
	if len(x) != len(y) {
		return false
	}
	for k, xv := range x {
		yv, ok := y[k]
		if !ok || !Equal(xv, yv) {
			return false
		}
	}
	return true
}

func equalArrays(x, y []any) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if !Equal(x[i], y[i]) {
			return false
		}
	}
	return true
}
