package mapping

import (
	"encoding/json"
	"fmt"

	"github.com/enerplanet/T1K/internal/jsondoc"
)

var (
	whenKeys      = keySet("forward", "reverse")
	conditionKeys = keySet("path", "exists", "equals", "not_equals", "in", "gt", "gte", "lt", "lte")
	operatorNames = []string{"exists", "equals", "not_equals", "in", "gt", "gte", "lt", "lte"}
)

// parseWhen reads a rule's "when": conditions per direction, each compiled
// against the variables bound when that direction evaluates them.
func parseWhen(f fields, loc string, forwardBound, reverseBound boundVars) (*whenSpec, error) {
	if !f.has("when") {
		return nil, nil
	}
	wloc := loc + ".when"
	wf, err := f.object("when", loc, "\"when\" must be an object with \"forward\" and/or \"reverse\"")
	if err != nil {
		return nil, err
	}
	if err := wf.check(wloc, whenKeys); err != nil {
		return nil, err
	}
	if len(wf) == 0 {
		return nil, fmt.Errorf("%s: needs \"forward\" and/or \"reverse\"", wloc)
	}
	spec := &whenSpec{}
	if spec.forward, err = parseConditions(wf["forward"], wloc+".forward", forwardBound); err != nil {
		return nil, err
	}
	if spec.reverse, err = parseConditions(wf["reverse"], wloc+".reverse", reverseBound); err != nil {
		return nil, err
	}
	return spec, nil
}

// parseConditions reads one condition or a list of them; nil raw means the
// direction has none.
func parseConditions(raw json.RawMessage, loc string, bound boundVars) ([]condition, error) {
	if raw == nil {
		return nil, nil
	}
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		list = []json.RawMessage{raw}
	}
	conds := make([]condition, 0, len(list))
	for i, item := range list {
		cond, err := parseCondition(item, fmt.Sprintf("%s[%d]", loc, i), bound)
		if err != nil {
			return nil, err
		}
		conds = append(conds, cond)
	}
	return conds, nil
}

func parseCondition(raw json.RawMessage, loc string, bound boundVars) (condition, error) {
	f, err := parseFields(raw, "a condition must be an object", loc)
	if err != nil {
		return condition{}, err
	}
	if err := f.check(loc, conditionKeys); err != nil {
		return condition{}, err
	}
	p, err := f.path("path", loc, true)
	if err != nil {
		return condition{}, err
	}
	if vars := bound.unbound(p.Vars()); len(vars) > 0 {
		return condition{}, fmt.Errorf("%s: path %q uses unbound %s", loc, p, varList(vars))
	}
	ops, err := parseOperators(f, loc)
	if err != nil {
		return condition{}, err
	}
	return condition{path: p, ops: ops}, nil
}

// parseOperators reads every operator key a condition carries; it needs at
// least one.
func parseOperators(f fields, loc string) ([]operator, error) {
	var ops []operator
	for _, name := range operatorNames {
		if !f.has(name) {
			continue
		}
		v, err := f.value(name, loc)
		if err != nil {
			return nil, err
		}
		if err := checkOperand(name, v); err != nil {
			return nil, fmt.Errorf("%s: %w", loc, err)
		}
		ops = append(ops, operator{name: name, value: v})
	}
	if len(ops) == 0 {
		return nil, fmt.Errorf("%s: a condition needs at least one of exists, equals, not_equals, in, gt, gte, lt, lte", loc)
	}
	return ops, nil
}

// checkOperand rejects operands of the wrong type for their operator.
func checkOperand(name string, v any) error {
	switch name {
	case "exists":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("\"exists\" must be true or false")
		}
	case "in":
		if _, ok := v.([]any); !ok {
			return fmt.Errorf("\"in\" must be an array of values")
		}
	case "gt", "gte", "lt", "lte":
		if _, ok := jsondoc.Number(v); !ok {
			return fmt.Errorf("%q must be a number", name)
		}
	}
	return nil
}

// holds reports whether every operator of the condition holds for the value
// at its path.
func (c condition) holds(v any, present bool) bool {
	for _, op := range c.ops {
		if !op.holds(v, present) {
			return false
		}
	}
	return true
}

func (o operator) holds(v any, present bool) bool {
	switch o.name {
	case "exists":
		return present == o.value.(bool)
	case "equals":
		return present && jsondoc.Equal(v, o.value)
	case "not_equals":
		return !present || !jsondoc.Equal(v, o.value)
	case "in":
		return present && o.member(v)
	default:
		return present && o.compare(v)
	}
}

func (o operator) member(v any) bool {
	for _, candidate := range o.value.([]any) {
		if jsondoc.Equal(v, candidate) {
			return true
		}
	}
	return false
}

// compare evaluates a numeric relation; a non-numeric value satisfies none.
func (o operator) compare(v any) bool {
	x, ok := jsondoc.Number(v)
	if !ok {
		return false
	}
	y, _ := jsondoc.Number(o.value)
	switch o.name {
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
