package mapping

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/enerplanet/T1K/internal/convert"
	"github.com/enerplanet/T1K/internal/jsondoc"
	"github.com/enerplanet/T1K/internal/jsonpath"
)

// fields is the raw JSON object of a rule, an "each" block or a condition,
// read one typed field at a time so every mistake is reported with the
// field's name and the rule's position.
type fields map[string]json.RawMessage

// parseFields decodes a JSON object; what describes the expected object in
// the error, for example "a rule must be a JSON object".
func parseFields(raw json.RawMessage, what, loc string) (fields, error) {
	var f fields
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("%s: %s: %w", loc, what, err)
	}
	return f, nil
}

func (f fields) has(key string) bool {
	_, ok := f[key]
	return ok
}

// check rejects keys outside the allowed set, which catches typos.
func (f fields) check(loc string, allowed map[string]bool) error {
	var unknown []string
	for k := range f {
		if !allowed[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return fmt.Errorf("%s: unknown key %q (allowed: %s)", loc, unknown[0], strings.Join(sortedKeys(allowed), ", "))
}

// path reads a path expression; a missing optional path is nil.
func (f fields) path(key, loc string, required bool) (*jsonpath.Path, error) {
	raw, ok := f[key]
	if !ok {
		if required {
			return nil, fmt.Errorf("%s: %q is required", loc, key)
		}
		return nil, nil
	}
	var expr string
	if err := json.Unmarshal(raw, &expr); err != nil {
		return nil, fmt.Errorf("%s: %q must be a path string", loc, key)
	}
	p, err := jsonpath.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("%s: %s: %w", loc, key, err)
	}
	return p, nil
}

// object reads a nested object such as "each" or "when".
func (f fields) object(key, loc, what string) (fields, error) {
	return parseFields(f[key], what, loc)
}

// list reads an array of raw entries, such as nested "rules".
func (f fields) list(key, loc string) ([]json.RawMessage, error) {
	raw, ok := f[key]
	if !ok {
		return nil, nil
	}
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("%s: %q must be an array", loc, key)
	}
	return list, nil
}

// value reads any JSON value, such as a constant or an operand.
func (f fields) value(key, loc string) (any, error) {
	v, err := jsondoc.Decode(f[key])
	if err != nil {
		return nil, fmt.Errorf("%s: %s: %w", loc, key, err)
	}
	return v, nil
}

// optional reads a JSON value that may be absent.
func (f fields) optional(key, loc string) (optional, error) {
	if !f.has(key) {
		return optional{}, nil
	}
	v, err := f.value(key, loc)
	if err != nil {
		return optional{}, err
	}
	return optional{value: v, set: true}, nil
}

// converter reads "convert"; a rule without it copies values unchanged.
func (f fields) converter(loc string) (convert.Converter, error) {
	raw, ok := f["convert"]
	if !ok {
		return convert.Identity, nil
	}
	conv, err := convert.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", loc, err)
	}
	return conv, nil
}

// description reads the free-text "description"; it is never an error.
func (f fields) description() string {
	var s string
	if raw, ok := f["description"]; ok {
		_ = json.Unmarshal(raw, &s)
	}
	return s
}

func keySet(keys ...string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// boundVars is the set of variables the enclosing "each" rules have bound at
// a point of the configuration.
type boundVars map[string]bool

// with returns the set extended by vars.
func (b boundVars) with(vars ...string) boundVars {
	n := make(boundVars, len(b)+len(vars))
	for v := range b {
		n[v] = true
	}
	for _, v := range vars {
		n[v] = true
	}
	return n
}

// unbound lists the variables in vars that are not in the set, in order.
func (b boundVars) unbound(vars []string) []string {
	var out []string
	for _, v := range vars {
		if !b[v] {
			out = append(out, v)
		}
	}
	return out
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, v := range a {
		if !contains(b, v) {
			return false
		}
	}
	return true
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// varList renders variable names for messages: "$i, $k", or "no variables".
func varList(vars []string) string {
	if len(vars) == 0 {
		return "no variables"
	}
	names := make([]string, len(vars))
	for i, v := range vars {
		names[i] = "$" + v
	}
	return strings.Join(names, ", ")
}
