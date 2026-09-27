package mapping

import (
	"fmt"

	"github.com/enerplanet/T1K/internal/jsondoc"
	"github.com/enerplanet/T1K/internal/jsonpath"
)

// applyScope applies an "each" rule: it enumerates the source elements and
// applies the nested rules to each of them in a frame of its own.
//
// Forward, the elements of each.from are enumerated in the input, the bind
// variables are read from every element, and the nested rules write below
// each.to. Reverse, the entries of each.to are enumerated in the input and
// the nested rules write below each.from, with every bound variable written
// back to its bind path. When each.from has variables each.to cannot supply,
// the reverse joins instead: see applyJoin.
func (r *run) applyScope(rl *rule, f *frame) error {
	if r.dir == Reverse && rl.each.join {
		return r.applyJoin(rl, f)
	}
	src, dst := r.sides(rl.each.from, rl.each.to)
	matches, err := src.Expand(r.in, f.input, f.bindings)
	if err != nil {
		return r.fail(rl, err)
	}
	for _, m := range matches {
		if !m.Present {
			continue
		}
		if err := r.applyElement(rl, f, dst, m); err != nil {
			return err
		}
	}
	return nil
}

// applyElement applies the nested rules to one matched source element.
func (r *run) applyElement(rl *rule, f *frame, dst *jsonpath.Path, m jsonpath.Match) error {
	ok, err := r.check(rl.when, m.Value, m.Bindings)
	if err != nil {
		return r.fail(rl, err)
	}
	if !ok {
		return nil
	}
	b := m.Bindings
	if r.dir == Forward {
		if b, err = r.bindFrom(rl, r.in, m.Value, b); err != nil {
			return err
		}
	}
	loc, err := dst.Resolve(f.output, b)
	if err != nil {
		return r.fail(rl, err)
	}
	if r.dir == Reverse {
		if err := r.writeBindings(rl, loc, b); err != nil {
			return err
		}
	}
	return r.apply(rl.rules, &frame{bindings: b, input: m.Value, output: loc})
}

// writeBindings stores each bound variable at its bind path below the
// element being built: the reverse of reading it from a source element.
func (r *run) writeBindings(rl *rule, loc jsonpath.Location, b jsonpath.Bindings) error {
	for _, bd := range rl.each.bind {
		bloc, err := bd.path.Resolve(loc, b)
		if err != nil {
			return r.fail(rl, err)
		}
		jsonpath.Set(&r.out, bloc, b[bd.variable])
	}
	return nil
}

// applyJoin is the reverse of a scope keyed by a value: for every element of
// each.from that earlier rules created in the output, the bind paths read
// from that element identify the each.to entry of the input to fill it from.
func (r *run) applyJoin(rl *rule, f *frame) error {
	base, ok := jsonpath.Get(r.out, f.output)
	if !ok {
		return nil
	}
	matches, err := rl.each.from.Expand(r.out, base, f.bindings)
	if err != nil {
		return r.fail(rl, err)
	}
	for _, m := range matches {
		if !m.Present {
			continue
		}
		if err := r.joinElement(rl, f, m); err != nil {
			return err
		}
	}
	return nil
}

// joinElement fills one output element from the input entry its bind values
// identify; an element whose entry does not exist is left as it is.
func (r *run) joinElement(rl *rule, f *frame, m jsonpath.Match) error {
	b, err := r.bindFrom(rl, r.out, m.Value, m.Bindings)
	if err != nil {
		return err
	}
	src, found, err := r.lookupSource(rl, f, b)
	if err != nil || !found {
		return err
	}
	ok, err := r.check(rl.when, src, b)
	if err != nil {
		return r.fail(rl, err)
	}
	if !ok {
		return nil
	}
	loc, err := rl.each.from.Resolve(f.output, b)
	if err != nil {
		return r.fail(rl, err)
	}
	return r.apply(rl.rules, &frame{bindings: b, input: src, output: loc})
}

// lookupSource reads the input entry each.to addresses with the bindings.
func (r *run) lookupSource(rl *rule, f *frame, b jsonpath.Bindings) (src any, found bool, err error) {
	matches, err := rl.each.to.Expand(r.in, f.input, b)
	if err != nil {
		return nil, false, r.fail(rl, err)
	}
	if len(matches) != 1 || !matches[0].Present {
		return nil, false, nil
	}
	return matches[0].Value, true, nil
}

// bindFrom reads the scope's bind paths from an element and returns the
// bindings extended by their values.
func (r *run) bindFrom(rl *rule, root, elem any, b jsonpath.Bindings) (jsonpath.Bindings, error) {
	if len(rl.each.bind) == 0 {
		return b, nil
	}
	out := b.Clone()
	for _, bd := range rl.each.bind {
		s, err := r.bindValue(rl, bd, root, elem, out)
		if err != nil {
			return nil, err
		}
		out[bd.variable] = s
	}
	return out, nil
}

// bindValue reads one bind path from an element as the string its variable
// is bound to. A missing or non-scalar value is an error, so the mapping
// never drops elements silently.
func (r *run) bindValue(rl *rule, bd binding, root, elem any, b jsonpath.Bindings) (string, error) {
	matches, err := bd.path.Expand(root, elem, b)
	if err != nil {
		return "", r.fail(rl, err)
	}
	if len(matches) != 1 || !matches[0].Present {
		return "", r.fail(rl, fmt.Errorf("bind $%s: %q not found in the element", bd.variable, bd.path))
	}
	s, ok := jsondoc.ScalarString(matches[0].Value)
	if !ok {
		return "", r.fail(rl, fmt.Errorf("bind $%s: %q is %s, but a key must be a string, number or boolean", bd.variable, bd.path, jsondoc.TypeName(matches[0].Value)))
	}
	return s, nil
}
