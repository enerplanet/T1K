package mapping

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/enerplanet/T1K/internal/jsonpath"
)

// rawConfig is the top level of a configuration file.
type rawConfig struct {
	Name        string                       `json:"name"`
	Version     string                       `json:"version"`
	Description string                       `json:"description"`
	Definitions map[string][]json.RawMessage `json:"definitions"`
	Rules       []json.RawMessage            `json:"rules"`
}

var (
	configKeys   = keySet("$schema", "name", "version", "description", "definitions", "rules")
	useKeys      = keySet("use", "description")
	scopeKeys    = keySet("each", "rules", "when", "description")
	constantKeys = keySet("from", "to", "value", "template", "when", "description")
	copyKeys     = keySet("from", "to", "convert", "default", "reverse_default", "when", "description")
	eachKeys     = keySet("from", "to", "bind")
)

func compile(data []byte) (*Program, error) {
	raw, err := parseDocument(data)
	if err != nil {
		return nil, err
	}
	c := &compiler{definitions: raw.Definitions}
	rules, err := c.compileRules(raw.Rules, "rules", boundVars{})
	if err != nil {
		return nil, err
	}
	return &Program{Name: raw.Name, Version: raw.Version, Description: raw.Description, rules: rules}, nil
}

// parseDocument decodes the top level of a configuration and checks the keys
// every configuration needs.
func parseDocument(data []byte) (*rawConfig, error) {
	var top fields
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("configuration is not a JSON object: %w", err)
	}
	if err := top.check("configuration", configKeys); err != nil {
		return nil, err
	}
	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if strings.TrimSpace(raw.Name) == "" {
		return nil, errors.New("configuration needs a non-empty \"name\"")
	}
	if raw.Rules == nil {
		return nil, errors.New("configuration needs a \"rules\" array")
	}
	return &raw, nil
}

// compiler turns raw rules into the compiled model, splicing definitions in
// where they are used and tracking the ones being expanded to detect cycles.
type compiler struct {
	definitions map[string][]json.RawMessage
	useStack    []string
}

func (c *compiler) compileRules(raws []json.RawMessage, loc string, bound boundVars) ([]*rule, error) {
	var out []*rule
	for i, raw := range raws {
		rules, err := c.compileRule(raw, fmt.Sprintf("%s[%d]", loc, i), bound)
		if err != nil {
			return nil, err
		}
		out = append(out, rules...)
	}
	return out, nil
}

// compileRule compiles one entry, which is a "use" of a definition (several
// rules), an "each" rule, a constant (it has "value" or "template") or a
// copy rule.
func (c *compiler) compileRule(raw json.RawMessage, loc string, bound boundVars) ([]*rule, error) {
	f, err := parseFields(raw, "a rule must be a JSON object", loc)
	if err != nil {
		return nil, err
	}
	switch {
	case f.has("use"):
		return c.compileUse(f, loc, bound)
	case f.has("each"):
		return one(c.compileScope(f, loc, bound))
	case f.has("value") || f.has("template"):
		return one(c.compileConstant(f, loc, bound))
	default:
		return one(c.compileCopy(f, loc, bound))
	}
}

func one(r *rule, err error) ([]*rule, error) {
	if err != nil {
		return nil, err
	}
	return []*rule{r}, nil
}

// compileUse splices the named definition in, compiled at the place of use
// so it may rely on the variables bound there.
func (c *compiler) compileUse(f fields, loc string, bound boundVars) ([]*rule, error) {
	if err := f.check(loc, useKeys); err != nil {
		return nil, err
	}
	name, err := definitionName(f, loc)
	if err != nil {
		return nil, err
	}
	def, err := c.definition(name, loc)
	if err != nil {
		return nil, err
	}
	c.useStack = append(c.useStack, name)
	defer func() { c.useStack = c.useStack[:len(c.useStack)-1] }()
	return c.compileRules(def, fmt.Sprintf("%s(definitions.%s)", loc, name), bound)
}

func definitionName(f fields, loc string) (string, error) {
	var name string
	if err := json.Unmarshal(f["use"], &name); err != nil || name == "" {
		return "", fmt.Errorf("%s: \"use\" must be the name of a definition", loc)
	}
	return name, nil
}

// definition looks a definition up and rejects one that is already being
// expanded, which would recurse forever.
func (c *compiler) definition(name, loc string) ([]json.RawMessage, error) {
	def, ok := c.definitions[name]
	if !ok {
		return nil, fmt.Errorf("%s: definition %q does not exist", loc, name)
	}
	if contains(c.useStack, name) {
		return nil, fmt.Errorf("%s: definition %q uses itself (via %s)", loc, name, strings.Join(c.useStack, " > "))
	}
	return def, nil
}

// compileCopy compiles a rule that copies a value between its two sides.
func (c *compiler) compileCopy(f fields, loc string, bound boundVars) (*rule, error) {
	if err := f.check(loc, copyKeys); err != nil {
		return nil, err
	}
	r := &rule{kind: ruleCopy, loc: loc, desc: f.description()}
	vars, err := r.copyPaths(f, loc, bound)
	if err != nil {
		return nil, err
	}
	if r.conv, err = f.converter(loc); err != nil {
		return nil, err
	}
	if r.def, err = f.optional("default", loc); err != nil {
		return nil, err
	}
	if r.revDef, err = f.optional("reverse_default", loc); err != nil {
		return nil, err
	}
	ruleBound := bound.with(vars...)
	if r.when, err = parseWhen(f, loc, ruleBound, ruleBound); err != nil {
		return nil, err
	}
	return r, nil
}

// copyPaths reads both sides and checks that they use the same unbound
// variables, which is what makes the rule work in both directions. It
// returns those variables.
func (r *rule) copyPaths(f fields, loc string, bound boundVars) ([]string, error) {
	var err error
	if r.from, err = f.path("from", loc, true); err != nil {
		return nil, err
	}
	if r.to, err = f.path("to", loc, true); err != nil {
		return nil, err
	}
	fromVars, toVars := bound.unbound(r.from.Vars()), bound.unbound(r.to.Vars())
	if !sameSet(fromVars, toVars) {
		return nil, fmt.Errorf("%s: \"from\" binds %s but \"to\" binds %s; both sides of a rule must use the same variables", loc, varList(fromVars), varList(toVars))
	}
	return fromVars, nil
}

// compileConstant compiles a rule that writes a fixed value on one side.
func (c *compiler) compileConstant(f fields, loc string, bound boundVars) (*rule, error) {
	if err := f.check(loc, constantKeys); err != nil {
		return nil, err
	}
	r := &rule{kind: ruleConstant, loc: loc, desc: f.description()}
	if err := r.constantSide(f, loc, bound); err != nil {
		return nil, err
	}
	if err := r.constantValue(f, loc, bound); err != nil {
		return nil, err
	}
	var err error
	if r.when, err = parseWhen(f, loc, bound, bound); err != nil {
		return nil, err
	}
	return r, nil
}

// constantSide reads the one side a constant has and checks that its path
// is concrete where the rule sits.
func (r *rule) constantSide(f fields, loc string, bound boundVars) error {
	var err error
	if r.from, err = f.path("from", loc, false); err != nil {
		return err
	}
	if r.to, err = f.path("to", loc, false); err != nil {
		return err
	}
	if (r.from == nil) == (r.to == nil) {
		return fmt.Errorf("%s: a constant rule has exactly one of \"from\" (written by Reverse) or \"to\" (written by Transform)", loc)
	}
	p := r.from
	if p == nil {
		p = r.to
	}
	if vars := bound.unbound(p.Vars()); len(vars) > 0 {
		return fmt.Errorf("%s: constant path %q uses %s, which no enclosing \"each\" binds", loc, p, varList(vars))
	}
	return nil
}

// constantValue reads "value" or "template", exactly one of which a constant
// carries.
func (r *rule) constantValue(f fields, loc string, bound boundVars) error {
	if f.has("value") && f.has("template") {
		return fmt.Errorf("%s: use either \"value\" or \"template\", not both", loc)
	}
	if f.has("value") {
		var err error
		r.value, err = f.value("value", loc)
		return err
	}
	return r.constantTemplate(f, loc, bound)
}

func (r *rule) constantTemplate(f fields, loc string, bound boundVars) error {
	var text string
	if err := json.Unmarshal(f["template"], &text); err != nil {
		return fmt.Errorf("%s: \"template\" must be a string", loc)
	}
	t, err := jsonpath.ParseTemplate(text)
	if err != nil {
		return fmt.Errorf("%s: template: %w", loc, err)
	}
	if vars := bound.unbound(t.Vars()); len(vars) > 0 {
		return fmt.Errorf("%s: template %q uses %s, which no enclosing \"each\" binds", loc, text, varList(vars))
	}
	r.template = t
	return nil
}
