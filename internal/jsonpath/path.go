// Package jsonpath implements the path expressions of T1K's mapping
// language: how a rule addresses values in a JSON document, how the variables
// in a path match on one side of a rule and are substituted on the other, and
// how a resolved location is read and written.
//
// A path is a dot-separated sequence of segments:
//
//	model.time.start              object keys
//	geometry.coordinates[1]       a fixed array index
//	topology[$i].length           every array element, binding its index to $i
//	model.nodes{$k}.coords        every object entry, binding its key to $k
//	model.transmission{line_$i}   a key template: literal text around variables
//	items[*].name                 an anonymous variable, named by position
//	/model.metadata.name          absolute: from the document root
//	.                             the current element itself
//	"my.key".value                a quoted key may contain any character
//
// Variables make rules bidirectional. Unbound on the side that is read, they
// iterate over the elements and bind to each; on the side that is written,
// their bound values are substituted. Bindings carries those values, Expand
// reads a document through a path, and Resolve turns a path into a concrete
// Location that Get and Set operate on.
package jsonpath

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type stepKind int

const (
	stepKey      stepKind = iota // a literal object key
	stepIndex                    // a literal array index
	stepIndexVar                 // an array index bound to a variable
	stepTemplate                 // an object key matched against a template
)

type step struct {
	kind     stepKind
	key      string
	index    int
	variable string
	template *Template
}

// Path is a parsed path expression.
type Path struct {
	expr     string
	absolute bool
	steps    []step
	vars     []string // in order of first appearance, without the "$"
}

// Parse parses a path expression. Anonymous variables ("*") are named $_1,
// $_2, ... in their order of appearance within the path.
func Parse(expr string) (*Path, error) {
	text := strings.TrimSpace(expr)
	if text == "" {
		return nil, errors.New("empty path")
	}
	p := &Path{expr: expr}
	if strings.HasPrefix(text, "/") {
		p.absolute = true
		text = text[1:]
	}
	if text == "" || text == "." {
		return p, nil
	}
	counter := 0
	sc := &scanner{src: text, anonymous: &counter}
	if err := sc.parseSegments(p); err != nil {
		return nil, fmt.Errorf("path %q: %w", expr, err)
	}
	return p, nil
}

// String returns the expression as it was written.
func (p *Path) String() string { return p.expr }

// Absolute reports whether the path starts at the document root rather than
// at the current element.
func (p *Path) Absolute() bool { return p.absolute }

// Vars returns the path's variables in order of first appearance, without
// the "$".
func (p *Path) Vars() []string { return append([]string(nil), p.vars...) }

// Unbound lists the path's variables that b does not bind.
func (p *Path) Unbound(b Bindings) []string { return unbound(p.vars, b) }

func (p *Path) addStep(s step) {
	p.steps = append(p.steps, s)
	switch s.kind {
	case stepIndexVar:
		p.addVar(s.variable)
	case stepTemplate:
		for _, v := range s.template.vars {
			p.addVar(v)
		}
	}
}

func (p *Path) addVar(name string) {
	for _, v := range p.vars {
		if v == name {
			return
		}
	}
	p.vars = append(p.vars, name)
}

// scanner reads a path expression segment by segment.
type scanner struct {
	src       string
	pos       int
	anonymous *int // counts anonymous variables so "*" gets a stable name
}

func (sc *scanner) parseSegments(p *Path) error {
	for first := true; ; first = false {
		if err := sc.parseSegment(p, first); err != nil {
			return err
		}
		if sc.done() {
			return nil
		}
		if err := sc.expectDot(); err != nil {
			return err
		}
	}
}

// parseSegment reads one segment: a key, then any number of [ ] and { }
// suffixes. Only the first segment may consist of suffixes alone, so that a
// path can start at an array element of the current document.
func (sc *scanner) parseSegment(p *Path, first bool) error {
	key, quoted, err := sc.readKey()
	if err != nil {
		return err
	}
	switch {
	case key == "" && !quoted && !sc.atSuffix():
		return fmt.Errorf("empty key at offset %d", sc.pos)
	case key == "" && !quoted && !first:
		return errors.New("a segment without a key is only allowed at the start of a path")
	case key != "" || quoted:
		p.addStep(step{kind: stepKey, key: key})
	}
	for sc.atSuffix() {
		if err := sc.readSuffix(p); err != nil {
			return err
		}
	}
	return nil
}

func (sc *scanner) done() bool { return sc.pos >= len(sc.src) }

func (sc *scanner) atSuffix() bool {
	return !sc.done() && (sc.src[sc.pos] == '[' || sc.src[sc.pos] == '{')
}

func (sc *scanner) expectDot() error {
	if sc.src[sc.pos] != '.' {
		return fmt.Errorf("unexpected %q at offset %d", sc.src[sc.pos], sc.pos)
	}
	sc.pos++
	if sc.done() {
		return errors.New("path must not end with a dot")
	}
	return nil
}

// readKey reads a bare or quoted object key; a bare key may be empty.
func (sc *scanner) readKey() (key string, quoted bool, err error) {
	if !sc.done() && sc.src[sc.pos] == '"' {
		key, err = sc.readQuotedKey()
		return key, true, err
	}
	key, err = sc.readBareKey()
	return key, false, err
}

// readQuotedKey reads a key between double quotes, where a backslash escapes
// the next character.
func (sc *scanner) readQuotedKey() (string, error) {
	var b strings.Builder
	for i := sc.pos + 1; i < len(sc.src); i++ {
		switch sc.src[i] {
		case '\\':
			if i+1 >= len(sc.src) {
				return "", errors.New("unterminated escape in quoted key")
			}
			i++
			b.WriteByte(sc.src[i])
		case '"':
			sc.pos = i + 1
			return b.String(), nil
		default:
			b.WriteByte(sc.src[i])
		}
	}
	return "", errors.New("unterminated quoted key")
}

// readBareKey reads a key up to the next separator or bracket. A "$" in a
// bare key is almost always a variable that belongs inside [ ] or { }.
func (sc *scanner) readBareKey() (string, error) {
	start := sc.pos
	for !sc.done() && !strings.ContainsRune(".[{]}\"", rune(sc.src[sc.pos])) {
		sc.pos++
	}
	key := sc.src[start:sc.pos]
	if strings.Contains(key, "$") {
		return "", fmt.Errorf("variables are only allowed inside [ ] or { } (key %q)", key)
	}
	return key, nil
}

// readSuffix reads one bracketed suffix, [ ] for an array index or { } for a
// key template, and appends its step.
func (sc *scanner) readSuffix(p *Path) error {
	open := sc.src[sc.pos]
	closer := byte(']')
	if open == '{' {
		closer = '}'
	}
	end := sc.findCloser(closer)
	if end < 0 {
		return fmt.Errorf("missing %q after offset %d", closer, sc.pos)
	}
	inner := sc.src[sc.pos+1 : end]
	sc.pos = end + 1
	if open == '[' {
		return sc.readIndex(p, inner)
	}
	return sc.readTemplate(p, inner)
}

// findCloser returns the offset of the bracket closing the suffix that starts
// at the current position, skipping the braces of "${name}" references.
func (sc *scanner) findCloser(closer byte) int {
	for i := sc.pos + 1; i < len(sc.src); i++ {
		switch {
		case sc.src[i] == '$' && i+1 < len(sc.src) && sc.src[i+1] == '{':
			end := strings.IndexByte(sc.src[i+2:], '}')
			if end < 0 {
				return -1
			}
			i += 2 + end
		case sc.src[i] == closer:
			return i
		}
	}
	return -1
}

// readIndex interprets the text between [ ]: "*", a "$variable" or a
// non-negative integer.
func (sc *scanner) readIndex(p *Path, inner string) error {
	switch {
	case inner == "*":
		p.addStep(step{kind: stepIndexVar, variable: sc.nextAnonymous()})
	case strings.HasPrefix(inner, "$"):
		name := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(inner, "$"), "{"), "}")
		if !ValidVarName(name) {
			return fmt.Errorf("invalid variable name %q", inner)
		}
		p.addStep(step{kind: stepIndexVar, variable: name})
	default:
		n, err := strconv.Atoi(inner)
		if err != nil || n < 0 {
			return fmt.Errorf("invalid array index %q", inner)
		}
		p.addStep(step{kind: stepIndex, index: n})
	}
	return nil
}

// readTemplate interprets the text between { }. A template without variables
// is a literal key.
func (sc *scanner) readTemplate(p *Path, inner string) error {
	t, err := parseTemplate(inner, sc.anonymous)
	if err != nil {
		return err
	}
	if len(t.vars) == 0 {
		p.addStep(step{kind: stepKey, key: t.parts[0].literal})
		return nil
	}
	p.addStep(step{kind: stepTemplate, template: t})
	return nil
}

func (sc *scanner) nextAnonymous() string {
	*sc.anonymous++
	return "_" + strconv.Itoa(*sc.anonymous)
}
