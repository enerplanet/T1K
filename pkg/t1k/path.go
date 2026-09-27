package t1k

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Path expressions
//
// A path addresses values inside a JSON document:
//
//	model.time.start              object keys, separated by dots
//	geometry.coordinates[1]       a fixed array index
//	topology[$i].length           an array element bound to the variable $i
//	model.nodes{$k}.coords        an object entry whose key is bound to $k
//	model.transmission{line_$i}   a key template: literal text around variables
//	items[*].name                 an anonymous variable (named by position)
//	/model.metadata.name          absolute (from the document root) inside a scope
//	.                             the current element itself
//	"my.key".value                a quoted key may contain any character
//
// Variables are what make a rule bidirectional: matched on one side, they are
// substituted on the other. A key template with several variables separates
// them with literal text; when a key is parsed against the template, earlier
// variables take the shortest possible match.

type stepKind int

const (
	stepKey      stepKind = iota // literal object key
	stepIndex                    // literal array index
	stepIndexVar                 // array index bound to a variable
	stepMap                      // object key matched against a template
)

type step struct {
	kind  stepKind
	key   string
	index int
	v     string
	tmpl  *keyTemplate
}

type path struct {
	raw      string
	absolute bool
	steps    []step
	vars     []string // in order of first appearance, without the "$"
}

// bindings maps variable names (without "$") to the string they are bound to.
type bindings map[string]string

func (b bindings) clone() bindings {
	c := make(bindings, len(b)+1)
	for k, v := range b {
		c[k] = v
	}
	return c
}

var varNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// parsePath parses a path expression. anon counts anonymous variables so that
// "*" gets a stable name per rule side.
func parsePath(raw string, anon *int) (*path, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("empty path")
	}
	p := &path{raw: raw}
	if strings.HasPrefix(s, "/") {
		p.absolute = true
		s = s[1:]
	}
	if s == "" || s == "." {
		return p, nil
	}
	sc := &pathScanner{s: s, anon: anon}
	if err := sc.parse(p); err != nil {
		return nil, fmt.Errorf("path %q: %w", raw, err)
	}
	return p, nil
}

type pathScanner struct {
	s    string
	i    int
	anon *int
}

func (sc *pathScanner) parse(p *path) error {
	first := true
	for {
		key, quoted, err := sc.readKey()
		if err != nil {
			return err
		}
		hasSuffix := sc.i < len(sc.s) && (sc.s[sc.i] == '[' || sc.s[sc.i] == '{')
		switch {
		case key == "" && !quoted && !hasSuffix:
			return fmt.Errorf("empty key at offset %d", sc.i)
		case key == "" && !quoted && !first:
			return fmt.Errorf("a segment without a key is only allowed at the start of a path")
		case key != "" || quoted:
			p.addStep(step{kind: stepKey, key: key})
		}
		for sc.i < len(sc.s) && (sc.s[sc.i] == '[' || sc.s[sc.i] == '{') {
			if err := sc.readSuffix(p); err != nil {
				return err
			}
		}
		first = false
		if sc.i >= len(sc.s) {
			return nil
		}
		if sc.s[sc.i] != '.' {
			return fmt.Errorf("unexpected %q at offset %d", sc.s[sc.i], sc.i)
		}
		sc.i++
		if sc.i >= len(sc.s) {
			return fmt.Errorf("path must not end with a dot")
		}
	}
}

// readKey reads a bare or quoted object key; the key may be empty.
func (sc *pathScanner) readKey() (key string, quoted bool, err error) {
	if sc.i < len(sc.s) && sc.s[sc.i] == '"' {
		var b strings.Builder
		for j := sc.i + 1; j < len(sc.s); j++ {
			switch sc.s[j] {
			case '\\':
				if j+1 >= len(sc.s) {
					return "", false, fmt.Errorf("unterminated escape in quoted key")
				}
				j++
				b.WriteByte(sc.s[j])
			case '"':
				sc.i = j + 1
				return b.String(), true, nil
			default:
				b.WriteByte(sc.s[j])
			}
		}
		return "", false, fmt.Errorf("unterminated quoted key")
	}
	start := sc.i
	for sc.i < len(sc.s) && !strings.ContainsRune(".[{]}\"", rune(sc.s[sc.i])) {
		sc.i++
	}
	key = sc.s[start:sc.i]
	if strings.Contains(key, "$") {
		return "", false, fmt.Errorf("variables are only allowed inside [ ] or { } (key %q)", key)
	}
	return key, false, nil
}

func (sc *pathScanner) readSuffix(p *path) error {
	open := sc.s[sc.i]
	closer := byte(']')
	if open == '{' {
		closer = '}'
	}
	end := sc.findCloser(closer)
	if end < 0 {
		return fmt.Errorf("missing %q after offset %d", closer, sc.i)
	}
	inner := sc.s[sc.i+1 : end]
	sc.i = end + 1
	if open == '[' {
		return sc.readIndex(p, inner)
	}
	tmpl, err := parseKeyTemplate(inner, sc.anon)
	if err != nil {
		return err
	}
	if len(tmpl.vars) == 0 {
		p.addStep(step{kind: stepKey, key: tmpl.parts[0].lit})
		return nil
	}
	p.addStep(step{kind: stepMap, tmpl: tmpl})
	return nil
}

// findCloser returns the offset of the bracket closing the suffix that starts
// at sc.i, skipping the braces of "${name}" variable references.
func (sc *pathScanner) findCloser(closer byte) int {
	for j := sc.i + 1; j < len(sc.s); j++ {
		switch {
		case sc.s[j] == '$' && j+1 < len(sc.s) && sc.s[j+1] == '{':
			end := strings.IndexByte(sc.s[j+2:], '}')
			if end < 0 {
				return -1
			}
			j += 2 + end
		case sc.s[j] == closer:
			return j
		}
	}
	return -1
}

func (sc *pathScanner) readIndex(p *path, inner string) error {
	switch {
	case inner == "*":
		p.addStep(step{kind: stepIndexVar, v: sc.nextAnon()})
	case strings.HasPrefix(inner, "$"):
		name := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(inner, "$"), "{"), "}")
		if !varNameRE.MatchString(name) {
			return fmt.Errorf("invalid variable name %q", inner)
		}
		p.addStep(step{kind: stepIndexVar, v: name})
	default:
		n, err := strconv.Atoi(inner)
		if err != nil || n < 0 {
			return fmt.Errorf("invalid array index %q", inner)
		}
		p.addStep(step{kind: stepIndex, index: n})
	}
	return nil
}

func (sc *pathScanner) nextAnon() string {
	*sc.anon++
	return "_" + strconv.Itoa(*sc.anon)
}

func (p *path) addStep(s step) {
	p.steps = append(p.steps, s)
	switch s.kind {
	case stepIndexVar:
		p.addVar(s.v)
	case stepMap:
		for _, v := range s.tmpl.vars {
			p.addVar(v)
		}
	}
}

func (p *path) addVar(name string) {
	for _, v := range p.vars {
		if v == name {
			return
		}
	}
	p.vars = append(p.vars, name)
}

// unbound lists the path's variables that b does not bind.
func (p *path) unbound(b bindings) []string {
	var out []string
	for _, v := range p.vars {
		if _, ok := b[v]; !ok {
			out = append(out, v)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Key templates

type tmplPart struct {
	lit string
	v   string // variable name; empty for a literal part
}

type keyTemplate struct {
	raw   string
	parts []tmplPart
	vars  []string
	re    *regexp.Regexp
}

// parseKeyTemplate parses the text between braces: literal characters, "$name"
// or "${name}" variables, "$$" for a literal dollar sign, or "*" alone for an
// anonymous variable.
func parseKeyTemplate(raw string, anon *int) (*keyTemplate, error) {
	t := &keyTemplate{raw: raw}
	if raw == "*" {
		*anon++
		t.parts = []tmplPart{{v: "_" + strconv.Itoa(*anon)}}
		t.vars = []string{t.parts[0].v}
		t.compile()
		return t, nil
	}
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			t.parts = append(t.parts, tmplPart{lit: lit.String()})
			lit.Reset()
		}
	}
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c != '$' {
			lit.WriteByte(c)
			continue
		}
		if i+1 < len(raw) && raw[i+1] == '$' {
			lit.WriteByte('$')
			i++
			continue
		}
		name, next, err := readVarName(raw, i+1)
		if err != nil {
			return nil, fmt.Errorf("key template {%s}: %w", raw, err)
		}
		flush()
		if n := len(t.parts); n > 0 && t.parts[n-1].v != "" {
			return nil, fmt.Errorf("key template {%s}: variables $%s and $%s need literal text between them", raw, t.parts[n-1].v, name)
		}
		t.parts = append(t.parts, tmplPart{v: name})
		t.vars = append(t.vars, name)
		i = next - 1
	}
	flush()
	if len(t.parts) == 0 {
		return nil, fmt.Errorf("key template {%s}: empty", raw)
	}
	for i, v := range t.vars {
		for _, w := range t.vars[:i] {
			if v == w {
				return nil, fmt.Errorf("key template {%s}: variable $%s appears twice", raw, v)
			}
		}
	}
	t.compile()
	return t, nil
}

// readVarName reads a variable name starting at s[i] (just after the "$").
func readVarName(s string, i int) (name string, next int, err error) {
	if i < len(s) && s[i] == '{' {
		end := strings.IndexByte(s[i:], '}')
		if end < 0 {
			return "", 0, fmt.Errorf("unterminated ${ variable")
		}
		name = s[i+1 : i+end]
		next = i + end + 1
	} else {
		j := i
		for j < len(s) && (s[j] == '_' || s[j] >= '0' && s[j] <= '9' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z') {
			j++
		}
		name = s[i:j]
		next = j
	}
	if !varNameRE.MatchString(name) {
		return "", 0, fmt.Errorf("invalid variable name %q", "$"+name)
	}
	return name, next, nil
}

func (t *keyTemplate) compile() {
	if len(t.vars) == 0 {
		return
	}
	var b strings.Builder
	b.WriteString("^")
	for _, part := range t.parts {
		if part.v != "" {
			b.WriteString("(.+?)")
		} else {
			b.WriteString(regexp.QuoteMeta(part.lit))
		}
	}
	b.WriteString("$")
	t.re = regexp.MustCompile(b.String())
}

// render substitutes bound variables into the template.
func (t *keyTemplate) render(b bindings) (string, error) {
	var out strings.Builder
	for _, part := range t.parts {
		if part.v == "" {
			out.WriteString(part.lit)
			continue
		}
		val, ok := b[part.v]
		if !ok {
			return "", fmt.Errorf("variable $%s is not bound", part.v)
		}
		out.WriteString(val)
	}
	return out.String(), nil
}

// match parses key against the template. Variables already bound in b must
// agree with the parsed value; the returned bindings add the new ones.
func (t *keyTemplate) match(key string, b bindings) (bindings, bool) {
	if t.re == nil {
		return b, key == t.parts[0].lit
	}
	m := t.re.FindStringSubmatch(key)
	if m == nil {
		return nil, false
	}
	out := b.clone()
	for i, v := range t.vars {
		if cur, ok := out[v]; ok && cur != m[i+1] {
			return nil, false
		}
		out[v] = m[i+1]
	}
	return out, true
}

// ---------------------------------------------------------------------------
// Matching

// match is one value addressed by a path, with the variables it bound.
type match struct {
	bindings bindings
	value    any
	present  bool
}

// expand evaluates the path against a document. base is the current element
// for relative paths; root is used when the path is absolute. Variables
// already bound select a single entry; unbound ones iterate. A path without
// unbound variables always yields exactly one match, present or not.
func (p *path) expand(root, base any, b bindings) ([]match, error) {
	start := base
	if p.absolute {
		start = root
	}
	var out []match
	if err := p.walk(start, 0, b, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 && len(p.unbound(b)) == 0 {
		out = append(out, match{bindings: b, present: false})
	}
	return out, nil
}

func (p *path) walk(cur any, i int, b bindings, out *[]match) error {
	if _, isHole := cur.(hole); isHole {
		return nil
	}
	if i == len(p.steps) {
		*out = append(*out, match{bindings: b, value: cur, present: true})
		return nil
	}
	s := p.steps[i]
	switch s.kind {
	case stepKey:
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		child, ok := obj[s.key]
		if !ok {
			return nil
		}
		return p.walk(child, i+1, b, out)
	case stepIndex:
		arr, ok := cur.([]any)
		if !ok || s.index >= len(arr) {
			return nil
		}
		return p.walk(arr[s.index], i+1, b, out)
	case stepIndexVar:
		arr, ok := cur.([]any)
		if !ok {
			return nil
		}
		if bound, ok := b[s.v]; ok {
			n, err := strconv.Atoi(bound)
			if err != nil || n < 0 {
				return fmt.Errorf("path %q: variable $%s is bound to %q, which is not an array index", p.raw, s.v, bound)
			}
			if n >= len(arr) {
				return nil
			}
			return p.walk(arr[n], i+1, b, out)
		}
		for n, el := range arr {
			nb := b.clone()
			nb[s.v] = strconv.Itoa(n)
			if err := p.walk(el, i+1, nb, out); err != nil {
				return err
			}
		}
		return nil
	case stepMap:
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		if len(s.tmpl.unbound(b)) == 0 {
			key, err := s.tmpl.render(b)
			if err != nil {
				return err
			}
			child, ok := obj[key]
			if !ok {
				return nil
			}
			return p.walk(child, i+1, b, out)
		}
		for _, key := range sortedKeys(obj) {
			nb, ok := s.tmpl.match(key, b)
			if !ok {
				continue
			}
			if err := p.walk(obj[key], i+1, nb, out); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("path %q: unknown step kind", p.raw)
}

func (t *keyTemplate) unbound(b bindings) []string {
	var out []string
	for _, v := range t.vars {
		if _, ok := b[v]; !ok {
			out = append(out, v)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Concrete locations (for writing)

// cstep is one step of a fully resolved location: an object key or an array
// index.
type cstep struct {
	key     string
	index   int
	isIndex bool
}

type location []cstep

func (l location) String() string {
	if len(l) == 0 {
		return "/"
	}
	var b strings.Builder
	for i, s := range l {
		if s.isIndex {
			fmt.Fprintf(&b, "[%d]", s.index)
			continue
		}
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(s.key)
	}
	return b.String()
}

// resolve turns the path into a concrete location using b. Every variable must
// be bound; an array variable must hold a non-negative integer. Relative paths
// are appended to base; absolute ones start from the root.
func (p *path) resolve(base location, b bindings) (location, error) {
	var loc location
	if !p.absolute {
		loc = append(loc, base...)
	}
	for _, s := range p.steps {
		switch s.kind {
		case stepKey:
			loc = append(loc, cstep{key: s.key})
		case stepIndex:
			loc = append(loc, cstep{index: s.index, isIndex: true})
		case stepIndexVar:
			bound, ok := b[s.v]
			if !ok {
				return nil, fmt.Errorf("path %q: variable $%s is not bound", p.raw, s.v)
			}
			n, err := strconv.Atoi(bound)
			if err != nil || n < 0 {
				return nil, fmt.Errorf("path %q: variable $%s is bound to %q, which is not an array index", p.raw, s.v, bound)
			}
			loc = append(loc, cstep{index: n, isIndex: true})
		case stepMap:
			key, err := s.tmpl.render(b)
			if err != nil {
				return nil, fmt.Errorf("path %q: %w", p.raw, err)
			}
			loc = append(loc, cstep{key: key})
		}
	}
	return loc, nil
}

// getAt reads the value at a concrete location.
func getAt(root any, loc location) (any, bool) {
	cur := root
	for _, s := range loc {
		if _, isHole := cur.(hole); isHole {
			return nil, false
		}
		if s.isIndex {
			arr, ok := cur.([]any)
			if !ok || s.index >= len(arr) {
				return nil, false
			}
			cur = arr[s.index]
			continue
		}
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = obj[s.key]
		if !ok {
			return nil, false
		}
	}
	if _, isHole := cur.(hole); isHole {
		return nil, false
	}
	return cur, true
}

// setAt stores v at a concrete location, creating objects and arrays on the
// way. A container of the wrong kind (or a scalar) in the way is replaced;
// arrays grow with holes up to the index written (see compact).
func setAt(cur *any, loc location, v any) {
	if len(loc) == 0 {
		*cur = v
		return
	}
	s := loc[0]
	if s.isIndex {
		arr, _ := (*cur).([]any)
		if missing := s.index + 1 - len(arr); missing > 0 {
			arr = append(arr, make([]any, missing)...)
			for i := len(arr) - missing; i < len(arr); i++ {
				arr[i] = hole{}
			}
		}
		setAt(&arr[s.index], loc[1:], v) //nolint:gosec // the array was grown to cover s.index just above
		*cur = arr
		return
	}
	obj, ok := (*cur).(map[string]any)
	if !ok {
		obj = map[string]any{}
	}
	child := obj[s.key]
	setAt(&child, loc[1:], v)
	obj[s.key] = child
	*cur = obj
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

// hole marks an array position no rule has written yet. Arrays grow with
// holes when an index variable is written out of order; compact removes them
// once the run is complete, so index variables need not be contiguous.
type hole struct{}

// compact removes the holes left in arrays by out-of-order index writes.
func compact(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = compact(e)
		}
		return x
	case []any:
		out := x[:0]
		for _, e := range x {
			if _, isHole := e.(hole); isHole {
				continue
			}
			out = append(out, compact(e))
		}
		return out
	}
	return v
}
