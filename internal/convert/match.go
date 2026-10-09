package convert

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/enerplanet/T1K/internal/jsondoc"
)

// match is a guard: it passes a value through unchanged in both directions
// and fails when the value's text does not match the pattern as a whole. It
// turns a constraint of the target system, such as an identifier syntax,
// into an error that names the rule, instead of a rejection downstream.
//
// Numbers and booleans are matched by their text; null and containers have
// no text and are errors. The pattern is Go regular expression syntax and is
// anchored to the whole value, so "[a-z]+" and "^[a-z]+$" mean the same.
type match struct {
	pattern *regexp.Regexp
	text    string
}

func newMatch(args map[string]any) (Converter, error) {
	r := newArgReader(args)
	pattern, err := r.str("pattern", "")
	if err != nil {
		return nil, err
	}
	if err := r.done(); err != nil {
		return nil, err
	}
	if pattern == "" {
		return nil, errors.New("argument \"pattern\" is required")
	}
	re, err := regexp.Compile("^(?:" + pattern + ")$")
	if err != nil {
		return nil, fmt.Errorf("pattern: %w", err)
	}
	return &match{pattern: re, text: pattern}, nil
}

func (c *match) Forward(v any, present bool) (any, bool, error) { return c.check(v, present) }
func (c *match) Reverse(v any, present bool) (any, bool, error) { return c.check(v, present) }

func (c *match) check(v any, present bool) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	s, ok := jsondoc.ScalarString(v)
	if !ok {
		return nil, false, fmt.Errorf("match: %s has no text to match against %q", jsondoc.TypeName(v), c.text)
	}
	if !c.pattern.MatchString(s) {
		return nil, false, fmt.Errorf("match: %s does not match %q", jsondoc.Describe(v), c.text)
	}
	return v, true, nil
}
