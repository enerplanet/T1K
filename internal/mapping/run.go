package mapping

import (
	"fmt"

	"github.com/enerplanet/T1K/internal/jsondoc"
	"github.com/enerplanet/T1K/internal/jsonpath"
)

// run is the state of one transformation: the direction, the input document
// and the output being built. Every Run call creates its own, which is why a
// Program can be used from several goroutines at once.
type run struct {
	dir Direction
	in  any
	out any
}

// frame is where the rules currently apply: the bound variables, the input
// element relative paths are read from, and the output location relative
// paths are written to. The root frame has no bindings, the whole input and
// the root location.
type frame struct {
	bindings jsonpath.Bindings
	input    any
	output   jsonpath.Location
}

// Run applies the program to a document and returns the document it builds.
// Errors wrap ErrRule and name the rule and the direction.
func (p *Program) Run(dir Direction, in any) (any, error) {
	r := &run{dir: dir, in: in, out: map[string]any{}}
	if err := r.apply(p.rules, &frame{bindings: jsonpath.Bindings{}, input: in}); err != nil {
		return nil, err
	}
	return jsonpath.Compact(r.out), nil
}

func (r *run) apply(rules []*rule, f *frame) error {
	for _, rl := range rules {
		if err := r.applyRule(rl, f); err != nil {
			return err
		}
	}
	return nil
}

func (r *run) applyRule(rl *rule, f *frame) error {
	switch rl.kind {
	case ruleCopy:
		return r.applyCopy(rl, f)
	case ruleConstant:
		return r.applyConstant(rl, f)
	default:
		return r.applyScope(rl, f)
	}
}

func (r *run) fail(rl *rule, err error) error {
	return fmt.Errorf("%w: %s (%s): %w", ErrRule, rl.loc, r.dir, err)
}

// sides returns the path read from and the path written to in the current
// direction.
func (r *run) sides(from, to *jsonpath.Path) (src, dst *jsonpath.Path) {
	if r.dir == Forward {
		return from, to
	}
	return to, from
}

// applyCopy copies every value the source path addresses to the target path.
func (r *run) applyCopy(rl *rule, f *frame) error {
	src, dst := r.sides(rl.from, rl.to)
	matches, err := src.Expand(r.in, f.input, f.bindings)
	if err != nil {
		return r.fail(rl, err)
	}
	for _, m := range matches {
		if err := r.copyMatch(rl, f, src, dst, m); err != nil {
			return err
		}
	}
	return nil
}

// copyMatch converts one matched value and writes it, or the direction's
// default when the value is absent, to the target.
func (r *run) copyMatch(rl *rule, f *frame, src, dst *jsonpath.Path, m jsonpath.Match) error {
	ok, err := r.check(rl.when, f.input, m.Bindings)
	if err != nil {
		return r.fail(rl, err)
	}
	if !ok {
		return nil
	}
	v, present, err := r.convert(rl, m.Value, m.Present)
	if err != nil {
		return r.fail(rl, fmt.Errorf("%s: %w", src, err))
	}
	if !present {
		if v, present = r.defaultFor(rl); !present {
			return nil
		}
	}
	return r.write(rl, dst, f.output, m.Bindings, v)
}

func (r *run) convert(rl *rule, v any, present bool) (any, bool, error) {
	if r.dir == Forward {
		return rl.conv.Forward(v, present)
	}
	return rl.conv.Reverse(v, present)
}

func (r *run) defaultFor(rl *rule) (any, bool) {
	if r.dir == Forward {
		return rl.def.value, rl.def.set
	}
	return rl.revDef.value, rl.revDef.set
}

// write stores a copy of v at the location dst resolves to below base.
func (r *run) write(rl *rule, dst *jsonpath.Path, base jsonpath.Location, b jsonpath.Bindings, v any) error {
	loc, err := dst.Resolve(base, b)
	if err != nil {
		return r.fail(rl, err)
	}
	jsonpath.Set(&r.out, loc, jsondoc.DeepCopy(v))
	return nil
}

// applyConstant writes a fixed value on the side the current direction
// produces; a constant declared for the other side is skipped.
func (r *run) applyConstant(rl *rule, f *frame) error {
	_, dst := r.sides(rl.from, rl.to)
	if dst == nil {
		return nil
	}
	ok, err := r.check(rl.when, f.input, f.bindings)
	if err != nil {
		return r.fail(rl, err)
	}
	if !ok {
		return nil
	}
	v, err := rl.constant(f.bindings)
	if err != nil {
		return r.fail(rl, err)
	}
	return r.write(rl, dst, f.output, f.bindings, v)
}

// constant returns the rule's value, rendering a template with the bindings.
func (rl *rule) constant(b jsonpath.Bindings) (any, error) {
	if rl.template != nil {
		return rl.template.Render(b)
	}
	return rl.value, nil
}

// check evaluates a rule's conditions for the current direction against the
// input element; a rule without conditions always applies.
func (r *run) check(w *whenSpec, elem any, b jsonpath.Bindings) (bool, error) {
	if w == nil {
		return true, nil
	}
	for _, c := range w.forDirection(r.dir) {
		holds, err := r.evaluate(c, elem, b)
		if err != nil || !holds {
			return false, err
		}
	}
	return true, nil
}

func (r *run) evaluate(c condition, elem any, b jsonpath.Bindings) (bool, error) {
	matches, err := c.path.Expand(r.in, elem, b)
	if err != nil {
		return false, err
	}
	if len(matches) != 1 {
		return false, fmt.Errorf("condition path %q must address a single value", c.path)
	}
	return c.holds(matches[0].Value, matches[0].Present), nil
}
