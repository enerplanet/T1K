package t1k

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Converters
//
// A converter changes a value on its way from one side of a rule to the
// other. Every converter is invertible: forward is used by Transform, reverse
// by Reverse, and reverse(forward(v)) yields v again (up to the documented
// loss of representation, such as the number of decimals a rounding keeps).
// A converter may also turn a value into "absent", which means the target
// key is not written, and absent back into a value.
//
// A rule's "convert" is a single step or a list of steps applied in order; a
// step is a converter name ("number") or a one-key object carrying its
// arguments ({"linear": {"divisor": 1000}}). The reverse direction runs the
// steps backwards.

type converter interface {
	forward(v any, present bool) (any, bool, error)
	reverse(v any, present bool) (any, bool, error)
}

type converterFactory func(args map[string]any) (converter, error)

var converterFactories = map[string]converterFactory{
	"identity": newIdentity,
	"number":   newNumber,
	"string":   newStringConv,
	"linear":   newLinear,
	"lookup":   newLookup,
	"datetime": newDatetime,
	"absent":   newAbsent,
}

// converterNames lists the registered converters (for documentation and the
// CLI).
func converterNames() []string {
	names := make([]string, 0, len(converterFactories))
	for n := range converterFactories {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// parseConvert builds the converter chain described by a rule's "convert".
func parseConvert(raw json.RawMessage) (converter, error) {
	spec, err := decodeJSON(raw)
	if err != nil {
		return nil, err
	}
	steps, isList := spec.([]any)
	if !isList {
		steps = []any{spec}
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("convert: empty list")
	}
	c := make(chain, 0, len(steps))
	for i, s := range steps {
		conv, err := parseConvertStep(s)
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

func parseConvertStep(spec any) (converter, error) {
	var name string
	args := map[string]any{}
	switch s := spec.(type) {
	case string:
		name = s
	case map[string]any:
		if len(s) != 1 {
			return nil, fmt.Errorf("a converter step is a name or a one-key object {\"name\": {args}}")
		}
		for k, v := range s {
			name = k
			a, ok := v.(map[string]any)
			if !ok && v != nil {
				return nil, fmt.Errorf("converter %q: arguments must be an object", k)
			}
			if ok {
				args = a
			}
		}
	default:
		return nil, fmt.Errorf("a converter step is a name or a one-key object, got %s", typeName(spec))
	}
	factory, ok := converterFactories[name]
	if !ok {
		return nil, fmt.Errorf("unknown converter %q (available: %s)", name, strings.Join(converterNames(), ", "))
	}
	conv, err := factory(args)
	if err != nil {
		return nil, fmt.Errorf("converter %q: %w", name, err)
	}
	return conv, nil
}

// chain applies converters in sequence; the reverse direction walks it
// backwards.
type chain []converter

func (c chain) forward(v any, present bool) (any, bool, error) {
	var err error
	for _, conv := range c {
		if v, present, err = conv.forward(v, present); err != nil {
			return nil, false, err
		}
	}
	return v, present, nil
}

func (c chain) reverse(v any, present bool) (any, bool, error) {
	var err error
	for i := len(c) - 1; i >= 0; i-- {
		if v, present, err = c[i].reverse(v, present); err != nil {
			return nil, false, err
		}
	}
	return v, present, nil
}

// --- argument helpers -------------------------------------------------------

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
	f, ok := numericValue(v)
	if !ok {
		return 0, fmt.Errorf("argument %q must be a number", key)
	}
	return f, nil
}

func (r *argReader) intOpt(key string) (int, bool, error) {
	v, ok := r.get(key)
	if !ok {
		return 0, false, nil
	}
	f, ok := numericValue(v)
	if !ok || f != math.Trunc(f) {
		return 0, false, fmt.Errorf("argument %q must be an integer", key)
	}
	return int(f), true, nil
}

func (r *argReader) str(key string, def string) (string, error) {
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

// done rejects arguments the converter does not know, which catches typos.
func (r *argReader) done() error {
	for k := range r.args {
		if !r.seen[k] {
			return fmt.Errorf("unknown argument %q", k)
		}
	}
	return nil
}

// --- identity ---------------------------------------------------------------

type identity struct{}

func newIdentity(args map[string]any) (converter, error) {
	return identity{}, newArgReader(args).done()
}

func (identity) forward(v any, present bool) (any, bool, error) { return v, present, nil }
func (identity) reverse(v any, present bool) (any, bool, error) { return v, present, nil }

// --- number -----------------------------------------------------------------

// number accepts a JSON number or a numeric string and yields a JSON number,
// in both directions. A number keeps its exact digits; a string is normalised.
type number struct{}

func newNumber(args map[string]any) (converter, error) {
	return number{}, newArgReader(args).done()
}

func (number) forward(v any, present bool) (any, bool, error) { return asNumber(v, present) }
func (number) reverse(v any, present bool) (any, bool, error) { return asNumber(v, present) }

func asNumber(v any, present bool) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	if n, ok := v.(json.Number); ok {
		return n, true, nil
	}
	f, ok := toFloat(v)
	if !ok {
		return nil, false, fmt.Errorf("number: %s %s is not numeric", typeName(v), describe(v))
	}
	n, err := numberValue(f)
	return n, true, err
}

// --- string -----------------------------------------------------------------

// stringConv renders a scalar as a string. The reverse turns a string back
// into a number or boolean when it reads as one, and leaves other strings
// alone.
type stringConv struct{}

func newStringConv(args map[string]any) (converter, error) {
	return stringConv{}, newArgReader(args).done()
}

func (stringConv) forward(v any, present bool) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	s, ok := scalarString(v)
	if !ok {
		return nil, false, fmt.Errorf("string: cannot render %s as a string", typeName(v))
	}
	return s, true, nil
}

func (stringConv) reverse(v any, present bool) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	s, ok := v.(string)
	if !ok {
		return v, true, nil
	}
	switch s {
	case "true":
		return true, true, nil
	case "false":
		return false, true, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) {
		if n, err := numberValue(f); err == nil {
			return n, true, nil
		}
	}
	return s, true, nil
}

// --- linear -----------------------------------------------------------------

// linear applies y = x * factor / divisor + offset; the reverse solves for x.
// round_forward and round_reverse round the result of the respective
// direction to that many decimals (unit conversions such as kWh/a to MW are
// the typical use).
type linear struct {
	factor, divisor, offset  float64
	roundFwd, roundRev       int
	hasRoundFwd, hasRoundRev bool
}

func newLinear(args map[string]any) (converter, error) {
	r := newArgReader(args)
	c := &linear{}
	var err error
	if c.factor, err = r.float("factor", 1); err != nil {
		return nil, err
	}
	if c.divisor, err = r.float("divisor", 1); err != nil {
		return nil, err
	}
	if c.offset, err = r.float("offset", 0); err != nil {
		return nil, err
	}
	if c.roundFwd, c.hasRoundFwd, err = r.intOpt("round_forward"); err != nil {
		return nil, err
	}
	if c.roundRev, c.hasRoundRev, err = r.intOpt("round_reverse"); err != nil {
		return nil, err
	}
	if err := r.done(); err != nil {
		return nil, err
	}
	if c.factor == 0 || c.divisor == 0 {
		return nil, fmt.Errorf("factor and divisor must not be zero")
	}
	if c.hasRoundFwd && c.roundFwd < 0 || c.hasRoundRev && c.roundRev < 0 {
		return nil, fmt.Errorf("rounding decimals must not be negative")
	}
	return c, nil
}

func (c *linear) forward(v any, present bool) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	x, ok := toFloat(v)
	if !ok {
		return nil, false, fmt.Errorf("linear: %s %s is not numeric", typeName(v), describe(v))
	}
	y := x*c.factor/c.divisor + c.offset
	if c.hasRoundFwd {
		y = roundTo(y, c.roundFwd)
	}
	n, err := numberValue(y)
	return n, true, err
}

func (c *linear) reverse(v any, present bool) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	y, ok := toFloat(v)
	if !ok {
		return nil, false, fmt.Errorf("linear: %s %s is not numeric", typeName(v), describe(v))
	}
	x := (y - c.offset) * c.divisor / c.factor
	if c.hasRoundRev {
		x = roundTo(x, c.roundRev)
	}
	n, err := numberValue(x)
	return n, true, err
}

func roundTo(x float64, decimals int) float64 {
	p := math.Pow10(decimals)
	return math.Round(x*p) / p
}

// --- lookup -----------------------------------------------------------------

// lookup translates between two vocabularies through a list of pairs
// [[left, right], ...]. Forward maps left to right, reverse maps right to
// left; both columns must be free of duplicates so the table is invertible.
// A value without a pair is an error unless strict is false, in which case it
// passes through unchanged.
type lookup struct {
	pairs  [][2]any
	strict bool
}

func newLookup(args map[string]any) (converter, error) {
	r := newArgReader(args)
	raw, ok := r.get("pairs")
	if !ok {
		return nil, fmt.Errorf("argument \"pairs\" is required")
	}
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		return nil, fmt.Errorf("\"pairs\" must be a non-empty array of [left, right] pairs")
	}
	c := &lookup{}
	var err error
	if c.strict, err = r.boolean("strict", true); err != nil {
		return nil, err
	}
	if err := r.done(); err != nil {
		return nil, err
	}
	for i, p := range list {
		pair, ok := p.([]any)
		if !ok || len(pair) != 2 {
			return nil, fmt.Errorf("pairs[%d] must be a [left, right] pair", i)
		}
		for _, prev := range c.pairs {
			if equalJSON(prev[0], pair[0]) {
				return nil, fmt.Errorf("pairs[%d]: left value %s appears twice", i, describe(pair[0]))
			}
			if equalJSON(prev[1], pair[1]) {
				return nil, fmt.Errorf("pairs[%d]: right value %s appears twice", i, describe(pair[1]))
			}
		}
		c.pairs = append(c.pairs, [2]any{pair[0], pair[1]})
	}
	return c, nil
}

func (c *lookup) forward(v any, present bool) (any, bool, error) {
	return c.translate(v, present, 0, 1)
}
func (c *lookup) reverse(v any, present bool) (any, bool, error) {
	return c.translate(v, present, 1, 0)
}

func (c *lookup) translate(v any, present bool, from, to int) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	for _, p := range c.pairs {
		if equalJSON(p[from], v) {
			return deepCopy(p[to]), true, nil
		}
	}
	if c.strict {
		return nil, false, fmt.Errorf("lookup: no pair for %s %s", typeName(v), describe(v))
	}
	return v, true, nil
}

// --- datetime ---------------------------------------------------------------

// datetime re-formats a timestamp string between two layouts (Go reference
// layouts or one of the names below). Forward parses "from" and renders "to";
// reverse does the opposite. Timestamps without zone information are read
// in "zone" (default UTC).
type datetime struct {
	from, to string
	loc      *time.Location
}

var namedLayouts = map[string]string{
	"RFC3339":     time.RFC3339,
	"RFC3339Nano": time.RFC3339Nano,
	"RFC1123":     time.RFC1123,
	"RFC1123Z":    time.RFC1123Z,
	"DateOnly":    time.DateOnly,
	"DateTime":    time.DateTime,
	"TimeOnly":    time.TimeOnly,
}

func newDatetime(args map[string]any) (converter, error) {
	r := newArgReader(args)
	c := &datetime{}
	var err error
	if c.from, err = r.str("from", ""); err != nil {
		return nil, err
	}
	if c.to, err = r.str("to", ""); err != nil {
		return nil, err
	}
	zone, err := r.str("zone", "UTC")
	if err != nil {
		return nil, err
	}
	if err := r.done(); err != nil {
		return nil, err
	}
	if c.from == "" || c.to == "" {
		return nil, fmt.Errorf("arguments \"from\" and \"to\" (layouts) are required")
	}
	c.from = resolveLayout(c.from)
	c.to = resolveLayout(c.to)
	if c.loc, err = time.LoadLocation(zone); err != nil {
		return nil, fmt.Errorf("zone %q: %w", zone, err)
	}
	return c, nil
}

func resolveLayout(name string) string {
	if l, ok := namedLayouts[name]; ok {
		return l
	}
	return name
}

func (c *datetime) forward(v any, present bool) (any, bool, error) {
	return c.reformat(v, present, c.from, c.to)
}

func (c *datetime) reverse(v any, present bool) (any, bool, error) {
	return c.reformat(v, present, c.to, c.from)
}

func (c *datetime) reformat(v any, present bool, parse, format string) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	s, ok := v.(string)
	if !ok {
		return nil, false, fmt.Errorf("datetime: expected a string, got %s", typeName(v))
	}
	t, err := time.ParseInLocation(parse, s, c.loc)
	if err != nil {
		return nil, false, fmt.Errorf("datetime: %w", err)
	}
	return t.In(c.loc).Format(format), true, nil
}

// --- absent -----------------------------------------------------------------

// absent treats the listed values as "not present" in both directions, so a
// placeholder such as "inf" or null is dropped instead of copied. Combine it
// with a rule's default or reverse_default to restore the placeholder.
type absent struct {
	values     []any
	ignoreCase bool
}

func newAbsent(args map[string]any) (converter, error) {
	r := newArgReader(args)
	raw, ok := r.get("values")
	if !ok {
		return nil, fmt.Errorf("argument \"values\" is required")
	}
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		return nil, fmt.Errorf("\"values\" must be a non-empty array")
	}
	c := &absent{values: list}
	var err error
	if c.ignoreCase, err = r.boolean("ignore_case", false); err != nil {
		return nil, err
	}
	return c, r.done()
}

func (c *absent) forward(v any, present bool) (any, bool, error) { return c.filter(v, present) }
func (c *absent) reverse(v any, present bool) (any, bool, error) { return c.filter(v, present) }

func (c *absent) filter(v any, present bool) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	for _, a := range c.values {
		if equalJSON(a, v) {
			return nil, false, nil
		}
		if c.ignoreCase {
			as, aok := a.(string)
			vs, vok := v.(string)
			if aok && vok && strings.EqualFold(as, vs) {
				return nil, false, nil
			}
		}
	}
	return v, true, nil
}

// describe renders a scalar for error messages.
func describe(v any) string {
	if s, ok := v.(string); ok {
		return strconv.Quote(s)
	}
	if s, ok := scalarString(v); ok {
		return s
	}
	return typeName(v)
}
