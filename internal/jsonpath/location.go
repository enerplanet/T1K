package jsonpath

import (
	"fmt"
	"strings"
)

// Step is one step of a Location: an object key or an array index.
type Step struct {
	Key     string
	Index   int
	IsIndex bool
}

// Key returns the step that selects an object entry.
func Key(name string) Step { return Step{Key: name} }

// Index returns the step that selects an array element.
func Index(i int) Step { return Step{Index: i, IsIndex: true} }

// Location is a fully resolved position in a document: the concrete keys and
// indices from the root to a value, with every variable substituted.
type Location []Step

// String renders the location like a path expression; the root is "/".
func (l Location) String() string {
	if len(l) == 0 {
		return "/"
	}
	var b strings.Builder
	for i, s := range l {
		switch {
		case s.IsIndex:
			fmt.Fprintf(&b, "[%d]", s.Index)
		case i > 0:
			b.WriteString("." + s.Key)
		default:
			b.WriteString(s.Key)
		}
	}
	return b.String()
}

// Resolve turns the path into a concrete location. Every variable must be
// bound, and a variable used as an array index must hold a non-negative
// integer. A relative path is appended to base; an absolute one starts at
// the root.
func (p *Path) Resolve(base Location, b Bindings) (Location, error) {
	var loc Location
	if !p.absolute {
		loc = append(loc, base...)
	}
	for _, s := range p.steps {
		resolved, err := p.resolveStep(s, b)
		if err != nil {
			return nil, err
		}
		loc = append(loc, resolved)
	}
	return loc, nil
}

func (p *Path) resolveStep(s step, b Bindings) (Step, error) {
	switch s.kind {
	case stepIndex:
		return Index(s.index), nil
	case stepIndexVar:
		bound, ok := b[s.variable]
		if !ok {
			return Step{}, fmt.Errorf("path %q: variable $%s is not bound", p.expr, s.variable)
		}
		n, err := p.arrayIndex(s.variable, bound)
		return Index(n), err
	case stepTemplate:
		key, err := s.template.Render(b)
		if err != nil {
			return Step{}, fmt.Errorf("path %q: %w", p.expr, err)
		}
		return Key(key), nil
	default:
		return Key(s.key), nil
	}
}

// Get reads the value at a location. Positions no rule has written read as
// absent.
func Get(root any, loc Location) (any, bool) {
	cur := root
	for _, s := range loc {
		child, ok := child(cur, s)
		if !ok {
			return nil, false
		}
		cur = child
	}
	if isHole(cur) {
		return nil, false
	}
	return cur, true
}

func child(cur any, s Step) (any, bool) {
	if isHole(cur) {
		return nil, false
	}
	if s.IsIndex {
		arr, ok := cur.([]any)
		if !ok || s.Index >= len(arr) {
			return nil, false
		}
		return arr[s.Index], true
	}
	obj, ok := cur.(map[string]any)
	if !ok {
		return nil, false
	}
	v, ok := obj[s.Key]
	return v, ok
}

// Set stores v at a location, creating the objects and arrays on the way. A
// container of the wrong kind, or a scalar, in the way is replaced. Arrays
// grow as far as the index written, with holes in the positions skipped; see
// Compact.
func Set(root *any, loc Location, v any) {
	if len(loc) == 0 {
		*root = v
		return
	}
	if loc[0].IsIndex {
		setIndex(root, loc, v)
		return
	}
	setKey(root, loc, v)
}

func setIndex(root *any, loc Location, v any) {
	arr, _ := (*root).([]any)
	arr = growTo(arr, loc[0].Index)
	Set(&arr[loc[0].Index], loc[1:], v) //nolint:gosec // growTo covers the index
	*root = arr
}

func setKey(root *any, loc Location, v any) {
	obj, ok := (*root).(map[string]any)
	if !ok {
		obj = map[string]any{}
	}
	entry := obj[loc[0].Key]
	Set(&entry, loc[1:], v)
	obj[loc[0].Key] = entry
	*root = obj
}

// growTo extends arr so that index is valid, filling new positions with
// holes.
func growTo(arr []any, index int) []any {
	for len(arr) <= index {
		arr = append(arr, hole{})
	}
	return arr
}

// hole marks an array position no rule has written yet. Arrays grow with
// holes when an index variable is written out of order; Compact removes them
// once a run is complete, so index variables need not be contiguous.
type hole struct{}

func isHole(v any) bool {
	_, ok := v.(hole)
	return ok
}

// Compact removes the holes left in arrays by out-of-order writes, in place,
// and returns the document. Explicit null values are kept.
func Compact(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = Compact(e)
		}
		return x
	case []any:
		out := x[:0]
		for _, e := range x {
			if !isHole(e) {
				out = append(out, Compact(e))
			}
		}
		return out
	}
	return v
}
