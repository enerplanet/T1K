package jsonpath

import (
	"fmt"
	"sort"
	"strconv"
)

// Match is one value a path addressed, together with the variables it bound
// on the way. Present is false for a concrete path whose value is missing.
type Match struct {
	Bindings Bindings
	Value    any
	Present  bool
}

// Expand evaluates the path against a document. Relative paths start at
// base, the current element; absolute paths start at root. Variables already
// bound select a single element; unbound ones iterate over every element and
// bind to it. A path without unbound variables always yields exactly one
// match, present or not, so callers can tell absence from "nothing matched".
func (p *Path) Expand(root, base any, b Bindings) ([]Match, error) {
	start := base
	if p.absolute {
		start = root
	}
	w := &walker{path: p}
	if err := w.walk(start, 0, b); err != nil {
		return nil, err
	}
	if len(w.matches) == 0 && len(p.Unbound(b)) == 0 {
		return []Match{{Bindings: b}}, nil
	}
	return w.matches, nil
}

// walker descends a document along a path, collecting matches.
type walker struct {
	path    *Path
	matches []Match
}

func (w *walker) walk(cur any, i int, b Bindings) error {
	if isHole(cur) {
		return nil
	}
	if i == len(w.path.steps) {
		w.matches = append(w.matches, Match{Bindings: b, Value: cur, Present: true})
		return nil
	}
	s := w.path.steps[i]
	switch s.kind {
	case stepKey:
		return w.walkKey(cur, s, i, b)
	case stepIndex:
		return w.walkIndex(cur, s, i, b)
	case stepIndexVar:
		return w.walkIndexVar(cur, s, i, b)
	case stepTemplate:
		return w.walkTemplate(cur, s, i, b)
	}
	return fmt.Errorf("path %q: unknown step kind", w.path.expr)
}

func (w *walker) walkKey(cur any, s step, i int, b Bindings) error {
	obj, ok := cur.(map[string]any)
	if !ok {
		return nil
	}
	child, ok := obj[s.key]
	if !ok {
		return nil
	}
	return w.walk(child, i+1, b)
}

func (w *walker) walkIndex(cur any, s step, i int, b Bindings) error {
	arr, ok := cur.([]any)
	if !ok || s.index >= len(arr) {
		return nil
	}
	return w.walk(arr[s.index], i+1, b)
}

// walkIndexVar selects one element when the variable is bound and iterates
// over all of them otherwise.
func (w *walker) walkIndexVar(cur any, s step, i int, b Bindings) error {
	arr, ok := cur.([]any)
	if !ok {
		return nil
	}
	if bound, ok := b[s.variable]; ok {
		return w.selectElement(arr, s, bound, i, b)
	}
	return w.iterateElements(arr, s, i, b)
}

func (w *walker) selectElement(arr []any, s step, bound string, i int, b Bindings) error {
	n, err := w.path.arrayIndex(s.variable, bound)
	if err != nil {
		return err
	}
	if n >= len(arr) {
		return nil
	}
	return w.walk(arr[n], i+1, b)
}

func (w *walker) iterateElements(arr []any, s step, i int, b Bindings) error {
	for n, el := range arr {
		nb := b.Clone()
		nb[s.variable] = strconv.Itoa(n)
		if err := w.walk(el, i+1, nb); err != nil {
			return err
		}
	}
	return nil
}

// walkTemplate selects one entry when every template variable is bound and
// otherwise iterates over the keys that fit the template, in sorted order so
// results are deterministic.
func (w *walker) walkTemplate(cur any, s step, i int, b Bindings) error {
	obj, ok := cur.(map[string]any)
	if !ok {
		return nil
	}
	if len(s.template.unbound(b)) == 0 {
		return w.selectEntry(obj, s, i, b)
	}
	return w.iterateEntries(obj, s, i, b)
}

func (w *walker) selectEntry(obj map[string]any, s step, i int, b Bindings) error {
	key, err := s.template.Render(b)
	if err != nil {
		return err
	}
	child, ok := obj[key]
	if !ok {
		return nil
	}
	return w.walk(child, i+1, b)
}

func (w *walker) iterateEntries(obj map[string]any, s step, i int, b Bindings) error {
	for _, key := range sortedKeys(obj) {
		nb, ok := s.template.match(key, b)
		if !ok {
			continue
		}
		if err := w.walk(obj[key], i+1, nb); err != nil {
			return err
		}
	}
	return nil
}

// arrayIndex interprets a variable's binding as an array index.
func (p *Path) arrayIndex(variable, bound string) (int, error) {
	n, err := strconv.Atoi(bound)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("path %q: variable $%s is bound to %q, which is not an array index", p.expr, variable, bound)
	}
	return n, nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
