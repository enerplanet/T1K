package t1k

import (
	"errors"
	"fmt"
)

// ErrInput marks a document that is not valid JSON.
var ErrInput = errors.New("t1k: invalid input")

// ErrRule marks a rule that could not be applied to a document, for example a
// converter that received a value it cannot handle or a "bind" path that is
// missing from an element.
var ErrRule = errors.New("t1k: rule failed")

type direction int

const (
	forward direction = iota
	reverse
)

func (d direction) String() string {
	if d == forward {
		return "forward"
	}
	return "reverse"
}

// run is the state of one transformation. Every call to Transform or Reverse
// creates its own run, so a TransformTask can be used from several goroutines.
type run struct {
	dir direction
	in  any
	out any
}

// frame is the position the rules currently apply to: the bound variables,
// the input element relative paths are read from, and the output location
// relative paths are written to.
type frame struct {
	b      bindings
	inVal  any
	outLoc location
}

func execute(cfg *Config, dir direction, in any) (any, error) {
	r := &run{dir: dir, in: in, out: map[string]any{}}
	if err := r.apply(cfg.rules, &frame{b: bindings{}, inVal: in}); err != nil {
		return nil, err
	}
	return compact(r.out), nil
}

func (r *run) apply(rules []*rule, f *frame) error {
	for _, rl := range rules {
		var err error
		switch rl.kind {
		case ruleCopy:
			err = r.applyCopy(rl, f)
		case ruleConstant:
			err = r.applyConstant(rl, f)
		case ruleScope:
			err = r.applyScope(rl, f)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *run) fail(rl *rule, err error) error {
	return fmt.Errorf("%w: %s (%s): %w", ErrRule, rl.loc, r.dir, err)
}

// sides returns the path read from and the path written to in the current
// direction.
func (r *run) sides(from, to *path) (src, dst *path) {
	if r.dir == forward {
		return from, to
	}
	return to, from
}

func (r *run) applyCopy(rl *rule, f *frame) error {
	src, dst := r.sides(rl.from, rl.to)
	matches, err := src.expand(r.in, f.inVal, f.b)
	if err != nil {
		return r.fail(rl, err)
	}
	for _, m := range matches {
		ok, err := r.check(rl.when, f.inVal, m.bindings)
		if err != nil {
			return r.fail(rl, err)
		}
		if !ok {
			continue
		}
		v, present, err := r.convert(rl, m.value, m.present)
		if err != nil {
			return r.fail(rl, fmt.Errorf("%s: %w", src.raw, err))
		}
		if !present {
			def, has := r.defaultFor(rl)
			if !has {
				continue
			}
			v = def
		}
		loc, err := dst.resolve(f.outLoc, m.bindings)
		if err != nil {
			return r.fail(rl, err)
		}
		setAt(&r.out, loc, deepCopy(v))
	}
	return nil
}

func (r *run) convert(rl *rule, v any, present bool) (any, bool, error) {
	if r.dir == forward {
		return rl.conv.forward(v, present)
	}
	return rl.conv.reverse(v, present)
}

func (r *run) defaultFor(rl *rule) (any, bool) {
	if r.dir == forward {
		return rl.def, rl.hasDef
	}
	return rl.revDef, rl.hasRevDef
}

// applyConstant writes a fixed value on the side the current direction
// produces; a constant declared for the other side is skipped.
func (r *run) applyConstant(rl *rule, f *frame) error {
	_, dst := r.sides(rl.from, rl.to)
	if dst == nil {
		return nil
	}
	ok, err := r.check(rl.when, f.inVal, f.b)
	if err != nil {
		return r.fail(rl, err)
	}
	if !ok {
		return nil
	}
	v := rl.value
	if rl.template != nil {
		s, err := rl.template.tmpl.render(f.b)
		if err != nil {
			return r.fail(rl, err)
		}
		v = s
	}
	loc, err := dst.resolve(f.outLoc, f.b)
	if err != nil {
		return r.fail(rl, err)
	}
	setAt(&r.out, loc, deepCopy(v))
	return nil
}

// applyScope iterates the elements an "each" addresses and applies the nested
// rules to each of them.
//
// Forward: the elements of each.from are enumerated in the input, "bind"
// variables are read from every element, and the nested rules write below
// each.to. Reverse: the entries of each.to are enumerated in the input and
// the nested rules write below each.from; bound variables are written back
// into their bind paths. When each.from has variables that each.to cannot
// supply (typically an array index, while each.to is keyed by a value), the
// reverse instead joins onto the elements of each.from that earlier rules
// already created in the output, reading the bind values from them.
func (r *run) applyScope(rl *rule, f *frame) error {
	if r.dir == reverse && rl.each.join {
		return r.applyJoin(rl, f)
	}
	src, dst := r.sides(rl.each.from, rl.each.to)
	matches, err := src.expand(r.in, f.inVal, f.b)
	if err != nil {
		return r.fail(rl, err)
	}
	for _, m := range matches {
		if !m.present {
			continue
		}
		ok, err := r.check(rl.when, m.value, m.bindings)
		if err != nil {
			return r.fail(rl, err)
		}
		if !ok {
			continue
		}
		b := m.bindings
		if r.dir == forward {
			if b, err = r.bindFrom(rl, r.in, m.value, b); err != nil {
				return err
			}
		}
		loc, err := dst.resolve(f.outLoc, b)
		if err != nil {
			return r.fail(rl, err)
		}
		if r.dir == reverse {
			for _, bd := range rl.each.bind {
				bloc, err := bd.path.resolve(loc, b)
				if err != nil {
					return r.fail(rl, err)
				}
				setAt(&r.out, bloc, b[bd.v])
			}
		}
		if err := r.apply(rl.rules, &frame{b: b, inVal: m.value, outLoc: loc}); err != nil {
			return err
		}
	}
	return nil
}

// applyJoin is the reverse of a keyed scope: for every element of each.from
// that exists in the output so far, the bind paths identify the each.to entry
// of the input to read from.
func (r *run) applyJoin(rl *rule, f *frame) error {
	base, ok := getAt(r.out, f.outLoc)
	if !ok {
		return nil
	}
	matches, err := rl.each.from.expand(r.out, base, f.b)
	if err != nil {
		return r.fail(rl, err)
	}
	for _, m := range matches {
		if !m.present {
			continue
		}
		b, err := r.bindFrom(rl, r.out, m.value, m.bindings)
		if err != nil {
			return err
		}
		srcMatches, err := rl.each.to.expand(r.in, f.inVal, b)
		if err != nil {
			return r.fail(rl, err)
		}
		if len(srcMatches) != 1 || !srcMatches[0].present {
			continue
		}
		src := srcMatches[0].value
		ok, err := r.check(rl.when, src, b)
		if err != nil {
			return r.fail(rl, err)
		}
		if !ok {
			continue
		}
		loc, err := rl.each.from.resolve(f.outLoc, b)
		if err != nil {
			return r.fail(rl, err)
		}
		if err := r.apply(rl.rules, &frame{b: b, inVal: src, outLoc: loc}); err != nil {
			return err
		}
	}
	return nil
}

// bindFrom reads the scope's bind paths from an element and adds the values
// to the bindings.
func (r *run) bindFrom(rl *rule, root, elem any, b bindings) (bindings, error) {
	if len(rl.each.bind) == 0 {
		return b, nil
	}
	out := b.clone()
	for _, bd := range rl.each.bind {
		matches, err := bd.path.expand(root, elem, out)
		if err != nil {
			return nil, r.fail(rl, err)
		}
		if len(matches) != 1 || !matches[0].present {
			return nil, r.fail(rl, fmt.Errorf("bind $%s: %q not found in the element", bd.v, bd.path.raw))
		}
		s, ok := scalarString(matches[0].value)
		if !ok {
			return nil, r.fail(rl, fmt.Errorf("bind $%s: %q is %s, but a key must be a string, number or boolean", bd.v, bd.path.raw, typeName(matches[0].value)))
		}
		out[bd.v] = s
	}
	return out, nil
}

// check evaluates a rule's conditions for the current direction against the
// input element; a rule without conditions always applies.
func (r *run) check(w *whenSpec, elem any, b bindings) (bool, error) {
	if w == nil {
		return true, nil
	}
	conds := w.forward
	if r.dir == reverse {
		conds = w.reverse
	}
	for _, c := range conds {
		matches, err := c.path.expand(r.in, elem, b)
		if err != nil {
			return false, err
		}
		if len(matches) != 1 {
			return false, fmt.Errorf("condition path %q must address a single value", c.path.raw)
		}
		if !evalCondition(c, matches[0].value, matches[0].present) {
			return false, nil
		}
	}
	return true, nil
}

func evalCondition(c condition, v any, present bool) bool {
	for _, op := range c.ops {
		if !evalOp(op, v, present) {
			return false
		}
	}
	return true
}

func evalOp(op condOp, v any, present bool) bool {
	switch op.op {
	case "exists":
		return present == op.value.(bool)
	case "equals":
		return present && equalJSON(v, op.value)
	case "not_equals":
		return !present || !equalJSON(v, op.value)
	case "in":
		if !present {
			return false
		}
		for _, candidate := range op.value.([]any) {
			if equalJSON(v, candidate) {
				return true
			}
		}
		return false
	}
	if !present {
		return false
	}
	x, ok := numericValue(v)
	if !ok {
		return false
	}
	y, _ := numericValue(op.value)
	switch op.op {
	case "gt":
		return x > y
	case "gte":
		return x >= y
	case "lt":
		return x < y
	case "lte":
		return x <= y
	}
	return false
}
