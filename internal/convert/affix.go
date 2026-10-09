package convert

import (
	"errors"
	"fmt"
	"strings"

	"github.com/enerplanet/T1K/internal/jsondoc"
)

// affix wraps a value in literal text: forward renders prefix + value +
// suffix as a string, reverse strips both again. It is the value-side
// counterpart of a key template such as {n$k}, for the places where an
// identifier travels as a value rather than as a key: a node id copied into
// a reference, for example, becomes "n1" and comes back as "1".
//
// Forward accepts any scalar and always yields a string, so a number comes
// back from the reverse as the string of its digits. Reverse requires the
// value to carry the prefix and the suffix; with strict false a value
// without them passes through unchanged instead of failing.
type affix struct {
	prefix, suffix string
	strict         bool
}

func newAffix(args map[string]any) (Converter, error) {
	r := newArgReader(args)
	c := &affix{}
	var err error
	if c.prefix, err = r.str("prefix", ""); err != nil {
		return nil, err
	}
	if c.suffix, err = r.str("suffix", ""); err != nil {
		return nil, err
	}
	if c.strict, err = r.boolean("strict", true); err != nil {
		return nil, err
	}
	if err := r.done(); err != nil {
		return nil, err
	}
	if c.prefix == "" && c.suffix == "" {
		return nil, errors.New("at least one of \"prefix\" and \"suffix\" is required")
	}
	return c, nil
}

func (c *affix) Forward(v any, present bool) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	s, ok := jsondoc.ScalarString(v)
	if !ok {
		return nil, false, fmt.Errorf("affix: cannot wrap %s %s in text", jsondoc.TypeName(v), jsondoc.Describe(v))
	}
	return c.prefix + s + c.suffix, true, nil
}

func (c *affix) Reverse(v any, present bool) (any, bool, error) {
	if !present {
		return nil, false, nil
	}
	s, ok := v.(string)
	if !ok {
		return nil, false, fmt.Errorf("affix: expected a string, got %s", jsondoc.TypeName(v))
	}
	inner, ok := c.strip(s)
	if ok {
		return inner, true, nil
	}
	if !c.strict {
		return v, true, nil
	}
	return nil, false, fmt.Errorf("affix: %s does not carry %s", jsondoc.Describe(s), c.describe())
}

// strip removes the prefix and the suffix; ok is false when either is
// missing or the two would overlap.
func (c *affix) strip(s string) (string, bool) {
	if len(s) < len(c.prefix)+len(c.suffix) || !strings.HasPrefix(s, c.prefix) || !strings.HasSuffix(s, c.suffix) {
		return "", false
	}
	return s[len(c.prefix) : len(s)-len(c.suffix)], true
}

func (c *affix) describe() string {
	switch {
	case c.suffix == "":
		return fmt.Sprintf("the prefix %q", c.prefix)
	case c.prefix == "":
		return fmt.Sprintf("the suffix %q", c.suffix)
	}
	return fmt.Sprintf("the prefix %q and the suffix %q", c.prefix, c.suffix)
}
