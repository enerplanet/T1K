package mapping

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/enerplanet/T1K/internal/jsonpath"
)

// compileScope compiles an "each" rule: the iteration, the conditions and
// the nested rules.
func (c *compiler) compileScope(f fields, loc string, bound boundVars) (*rule, error) {
	if err := f.check(loc, scopeKeys); err != nil {
		return nil, err
	}
	r := &rule{kind: ruleScope, loc: loc, desc: f.description()}
	each, vars, err := compileEach(f, loc, bound)
	if err != nil {
		return nil, err
	}
	r.each = each
	// Forward conditions run before the bind paths are read, so they may only
	// use the variables the iteration itself binds; reverse conditions and
	// the nested rules run with everything bound.
	inner := bound.with(vars.all()...)
	if r.when, err = parseWhen(f, loc, bound.with(vars.from...), inner); err != nil {
		return nil, err
	}
	r.rules, err = c.compileNested(f, loc, inner)
	return r, err
}

// eachVars are the variables an "each" binds beyond the enclosing scopes:
// by iterating "from", by iterating "to", and through "bind".
type eachVars struct {
	from, to, bind []string
}

func (v eachVars) all() []string {
	return append(append(append([]string(nil), v.from...), v.to...), v.bind...)
}

// compileEach reads the "each" block: the element paths, the bind entries
// and the variable discipline between them.
func compileEach(f fields, loc string, bound boundVars) (*eachSpec, eachVars, error) {
	eloc := loc + ".each"
	ef, err := f.object("each", loc, "\"each\" must be an object with \"from\" and \"to\"")
	if err != nil {
		return nil, eachVars{}, err
	}
	if err := ef.check(eloc, eachKeys); err != nil {
		return nil, eachVars{}, err
	}
	e := &eachSpec{}
	var vars eachVars
	if vars.from, vars.to, err = e.paths(ef, eloc, bound); err != nil {
		return nil, eachVars{}, err
	}
	if vars.bind, err = e.compileBind(ef, eloc, bound, vars.from, vars.to); err != nil {
		return nil, eachVars{}, err
	}
	if err := e.checkVariables(eloc, vars.from, vars.to, vars.bind); err != nil {
		return nil, eachVars{}, err
	}
	return e, vars, nil
}

// compileNested compiles the rules an "each" applies to every element.
func (c *compiler) compileNested(f fields, loc string, bound boundVars) ([]*rule, error) {
	nested, err := f.list("rules", loc)
	if err != nil {
		return nil, err
	}
	return c.compileRules(nested, loc+".rules", bound)
}

// paths reads the two element paths and returns the variables each of them
// binds beyond the enclosing scopes.
func (e *eachSpec) paths(ef fields, loc string, bound boundVars) (fromVars, toVars []string, err error) {
	if e.from, err = ef.path("from", loc, true); err != nil {
		return nil, nil, err
	}
	if e.to, err = ef.path("to", loc, true); err != nil {
		return nil, nil, err
	}
	return bound.unbound(e.from.Vars()), bound.unbound(e.to.Vars()), nil
}

// checkVariables verifies that "to" can be resolved in the forward direction
// and decides how the reverse direction enumerates: by iterating "to" when it
// supplies every variable of "from", or by joining onto the elements earlier
// rules created when it does not.
func (e *eachSpec) checkVariables(loc string, fromVars, toVars, bindVars []string) error {
	for _, v := range toVars {
		if !contains(fromVars, v) && !contains(bindVars, v) {
			return fmt.Errorf("%s: $%s in \"to\" is bound neither by \"from\" nor by \"bind\"", loc, v)
		}
	}
	for _, v := range fromVars {
		if contains(toVars, v) {
			continue
		}
		if len(bindVars) == 0 {
			return fmt.Errorf("%s: $%s in \"from\" does not appear in \"to\"; use it there or bind the key with \"bind\"", loc, v)
		}
		e.join = true
	}
	return nil
}

// compileBind reads the "bind" map and returns the variables it binds.
func (e *eachSpec) compileBind(ef fields, loc string, bound boundVars, fromVars, toVars []string) ([]string, error) {
	if !ef.has("bind") {
		return nil, nil
	}
	var entries map[string]string
	if err := json.Unmarshal(ef["bind"], &entries); err != nil {
		return nil, fmt.Errorf("%s: \"bind\" must map variables to paths, like {\"$k\": \"id\"}", loc)
	}
	var vars []string
	for _, name := range sortedNames(entries) {
		b, err := compileBinding(name, entries[name], loc, bound, fromVars, toVars)
		if err != nil {
			return nil, err
		}
		e.bind = append(e.bind, b)
		vars = append(vars, b.variable)
	}
	return vars, nil
}

func sortedNames(m map[string]string) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// compileBinding validates one "$variable": "path" entry: the variable must
// be new, must be used in "to", and its path may only use variables the
// iteration binds.
func compileBinding(name, expr, loc string, bound boundVars, fromVars, toVars []string) (binding, error) {
	v := strings.TrimPrefix(name, "$")
	switch {
	case !jsonpath.ValidVarName(v):
		return binding{}, fmt.Errorf("%s: bind: invalid variable name %q", loc, name)
	case bound[v]:
		return binding{}, fmt.Errorf("%s: bind: $%s is already bound by an enclosing \"each\"", loc, v)
	case contains(fromVars, v):
		return binding{}, fmt.Errorf("%s: bind: $%s is already bound by \"from\"", loc, v)
	case !contains(toVars, v):
		return binding{}, fmt.Errorf("%s: bind: $%s is not used in \"to\"", loc, v)
	}
	p, err := jsonpath.Parse(expr)
	if err != nil {
		return binding{}, fmt.Errorf("%s: bind $%s: %w", loc, v, err)
	}
	if unbound := bound.with(fromVars...).unbound(p.Vars()); len(unbound) > 0 {
		return binding{}, fmt.Errorf("%s: bind $%s: path uses unbound %s", loc, v, varList(unbound))
	}
	return binding{variable: v, path: p}, nil
}
