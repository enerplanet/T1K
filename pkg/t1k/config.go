package t1k

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ErrConfig marks a configuration that failed to parse or validate.
var ErrConfig = errors.New("t1k: invalid configuration")

// Config is a parsed and validated mapping configuration. It is immutable
// after loading and safe to share between any number of TransformTasks.
type Config struct {
	// Name identifies the mapping, for example "enerplanet-to-meme".
	Name string
	// Version is the mapping's own version string, as written by its author.
	Version string
	// Description explains what the forward transformation produces.
	Description string

	rules []*rule
}

// LoadConfig parses a mapping configuration from its JSON text.
func LoadConfig(data []byte) (*Config, error) {
	cfg, err := compileConfig(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	return cfg, nil
}

// LoadConfigFile reads and parses a mapping configuration file.
func LoadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	cfg, err := LoadConfig(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Rules returns the number of top-level rules, for diagnostics.
func (c *Config) Rules() int { return len(c.rules) }

// ---------------------------------------------------------------------------
// Compiled rule model

type ruleKind int

const (
	ruleCopy     ruleKind = iota // from <-> to, optionally converted
	ruleConstant                 // a fixed value written on one side only
	ruleScope                    // "each": iterate and apply nested rules
)

type rule struct {
	kind ruleKind
	loc  string // position in the configuration, for error messages
	desc string

	from, to *path

	// ruleCopy
	conv      converter
	def       any
	hasDef    bool
	revDef    any
	hasRevDef bool

	// ruleConstant
	value    any
	template *valueTemplate

	// ruleScope
	each  *eachSpec
	rules []*rule

	when *whenSpec
}

type eachSpec struct {
	from, to *path
	bind     []binding // in configuration order
	// join is set when each.from has variables each.to cannot supply, so the
	// reverse direction must join onto elements earlier rules created.
	join bool
}

type binding struct {
	v    string
	path *path
}

type whenSpec struct {
	forward, reverse []condition
}

type condition struct {
	path *path
	ops  []condOp
}

type condOp struct {
	op    string // exists, equals, not_equals, in, gt, gte, lt, lte
	value any
}

// valueTemplate is a constant string with variable references.
type valueTemplate struct {
	raw  string
	tmpl *keyTemplate
}

// ---------------------------------------------------------------------------
// Parsing

type rawConfig struct {
	Name        string                       `json:"name"`
	Version     string                       `json:"version"`
	Description string                       `json:"description"`
	Definitions map[string][]json.RawMessage `json:"definitions"`
	Rules       []json.RawMessage            `json:"rules"`
}

var configKeys = keySet("$schema", "name", "version", "description", "definitions", "rules")

func compileConfig(data []byte) (*Config, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("configuration is not a JSON object: %w", err)
	}
	if err := checkKeys("configuration", top, configKeys); err != nil {
		return nil, err
	}
	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if strings.TrimSpace(raw.Name) == "" {
		return nil, fmt.Errorf("configuration needs a non-empty \"name\"")
	}
	if raw.Rules == nil {
		return nil, fmt.Errorf("configuration needs a \"rules\" array")
	}
	c := &compiler{defs: raw.Definitions}
	rules, err := c.compileRules(raw.Rules, "rules", newScope())
	if err != nil {
		return nil, err
	}
	return &Config{Name: raw.Name, Version: raw.Version, Description: raw.Description, rules: rules}, nil
}

type compiler struct {
	defs     map[string][]json.RawMessage
	useStack []string
}

// scope tracks which variables the enclosing "each" rules have bound.
type scope struct {
	bound map[string]bool
}

func newScope() *scope { return &scope{bound: map[string]bool{}} }

func (s *scope) with(vars ...string) *scope {
	n := newScope()
	for v := range s.bound {
		n.bound[v] = true
	}
	for _, v := range vars {
		n.bound[v] = true
	}
	return n
}

func (c *compiler) compileRules(raws []json.RawMessage, loc string, sc *scope) ([]*rule, error) {
	var out []*rule
	for i, raw := range raws {
		rloc := fmt.Sprintf("%s[%d]", loc, i)
		rules, err := c.compileRule(raw, rloc, sc)
		if err != nil {
			return nil, err
		}
		out = append(out, rules...)
	}
	return out, nil
}

var (
	useKeys      = keySet("use", "description")
	scopeKeys    = keySet("each", "rules", "when", "description")
	constantKeys = keySet("from", "to", "value", "template", "when", "description")
	copyKeys     = keySet("from", "to", "convert", "default", "reverse_default", "when", "description")
	eachKeys     = keySet("from", "to", "bind")
	whenKeys     = keySet("forward", "reverse")
	condKeys     = keySet("path", "exists", "equals", "not_equals", "in", "gt", "gte", "lt", "lte")
)

// compileRule returns one rule, or several when the entry is a "use" of a
// definition.
func (c *compiler) compileRule(raw json.RawMessage, loc string, sc *scope) ([]*rule, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("%s: a rule must be a JSON object: %w", loc, err)
	}
	_, isUse := fields["use"]
	_, isScope := fields["each"]
	_, hasValue := fields["value"]
	_, hasTemplate := fields["template"]
	switch {
	case isUse:
		return c.compileUse(fields, loc, sc)
	case isScope:
		r, err := c.compileScope(fields, loc, sc)
		return []*rule{r}, err
	case hasValue || hasTemplate:
		r, err := c.compileConstant(fields, loc, sc)
		return []*rule{r}, err
	default:
		r, err := c.compileCopy(fields, loc, sc)
		return []*rule{r}, err
	}
}

func (c *compiler) compileUse(fields map[string]json.RawMessage, loc string, sc *scope) ([]*rule, error) {
	if err := checkKeys(loc, fields, useKeys); err != nil {
		return nil, err
	}
	var name string
	if err := json.Unmarshal(fields["use"], &name); err != nil || name == "" {
		return nil, fmt.Errorf("%s: \"use\" must be the name of a definition", loc)
	}
	def, ok := c.defs[name]
	if !ok {
		return nil, fmt.Errorf("%s: definition %q does not exist", loc, name)
	}
	for _, open := range c.useStack {
		if open == name {
			return nil, fmt.Errorf("%s: definition %q uses itself (via %s)", loc, name, strings.Join(c.useStack, " > "))
		}
	}
	c.useStack = append(c.useStack, name)
	defer func() { c.useStack = c.useStack[:len(c.useStack)-1] }()
	return c.compileRules(def, fmt.Sprintf("%s(definitions.%s)", loc, name), sc)
}

func (c *compiler) compileCopy(fields map[string]json.RawMessage, loc string, sc *scope) (*rule, error) {
	if err := checkKeys(loc, fields, copyKeys); err != nil {
		return nil, err
	}
	r := &rule{kind: ruleCopy, loc: loc}
	var err error
	if r.from, err = pathField(fields, "from", loc, true); err != nil {
		return nil, err
	}
	if r.to, err = pathField(fields, "to", loc, true); err != nil {
		return nil, err
	}
	fromVars, toVars := unboundVars(r.from, sc), unboundVars(r.to, sc)
	if !sameSet(fromVars, toVars) {
		return nil, fmt.Errorf("%s: \"from\" binds %s but \"to\" binds %s; both sides of a rule must use the same variables", loc, varList(fromVars), varList(toVars))
	}
	if raw, ok := fields["convert"]; ok {
		if r.conv, err = parseConvert(raw); err != nil {
			return nil, fmt.Errorf("%s: %w", loc, err)
		}
	} else {
		r.conv = identity{}
	}
	if raw, ok := fields["default"]; ok {
		if r.def, err = decodeJSON(raw); err != nil {
			return nil, fmt.Errorf("%s: default: %w", loc, err)
		}
		r.hasDef = true
	}
	if raw, ok := fields["reverse_default"]; ok {
		if r.revDef, err = decodeJSON(raw); err != nil {
			return nil, fmt.Errorf("%s: reverse_default: %w", loc, err)
		}
		r.hasRevDef = true
	}
	ruleScope := sc.with(fromVars...)
	if r.when, err = whenField(fields, loc, ruleScope, ruleScope); err != nil {
		return nil, err
	}
	r.desc = descField(fields)
	return r, nil
}

func (c *compiler) compileConstant(fields map[string]json.RawMessage, loc string, sc *scope) (*rule, error) {
	if err := checkKeys(loc, fields, constantKeys); err != nil {
		return nil, err
	}
	r := &rule{kind: ruleConstant, loc: loc}
	var err error
	if r.from, err = pathField(fields, "from", loc, false); err != nil {
		return nil, err
	}
	if r.to, err = pathField(fields, "to", loc, false); err != nil {
		return nil, err
	}
	if (r.from == nil) == (r.to == nil) {
		return nil, fmt.Errorf("%s: a constant rule has exactly one of \"from\" (written by Reverse) or \"to\" (written by Transform)", loc)
	}
	p := r.from
	if p == nil {
		p = r.to
	}
	if vars := unboundVars(p, sc); len(vars) > 0 {
		return nil, fmt.Errorf("%s: constant path %q uses %s, which no enclosing \"each\" binds", loc, p.raw, varList(vars))
	}
	rawValue, hasValue := fields["value"]
	rawTemplate, hasTemplate := fields["template"]
	if hasValue && hasTemplate {
		return nil, fmt.Errorf("%s: use either \"value\" or \"template\", not both", loc)
	}
	if hasValue {
		if r.value, err = decodeJSON(rawValue); err != nil {
			return nil, fmt.Errorf("%s: value: %w", loc, err)
		}
	} else {
		var s string
		if err := json.Unmarshal(rawTemplate, &s); err != nil {
			return nil, fmt.Errorf("%s: \"template\" must be a string", loc)
		}
		anon := 0
		t, err := parseKeyTemplate(s, &anon)
		if err != nil {
			return nil, fmt.Errorf("%s: template: %w", loc, err)
		}
		if vars := unboundVarNames(t.vars, sc); len(vars) > 0 {
			return nil, fmt.Errorf("%s: template %q uses %s, which no enclosing \"each\" binds", loc, s, varList(vars))
		}
		r.template = &valueTemplate{raw: s, tmpl: t}
	}
	if r.when, err = whenField(fields, loc, sc, sc); err != nil {
		return nil, err
	}
	r.desc = descField(fields)
	return r, nil
}

func (c *compiler) compileScope(fields map[string]json.RawMessage, loc string, sc *scope) (*rule, error) {
	if err := checkKeys(loc, fields, scopeKeys); err != nil {
		return nil, err
	}
	var eachFields map[string]json.RawMessage
	if err := json.Unmarshal(fields["each"], &eachFields); err != nil {
		return nil, fmt.Errorf("%s: \"each\" must be an object with \"from\" and \"to\"", loc)
	}
	eloc := loc + ".each"
	if err := checkKeys(eloc, eachFields, eachKeys); err != nil {
		return nil, err
	}
	r := &rule{kind: ruleScope, loc: loc, each: &eachSpec{}}
	var err error
	if r.each.from, err = pathField(eachFields, "from", eloc, true); err != nil {
		return nil, err
	}
	if r.each.to, err = pathField(eachFields, "to", eloc, true); err != nil {
		return nil, err
	}
	fromVars, toVars := unboundVars(r.each.from, sc), unboundVars(r.each.to, sc)
	bindVars, err := c.compileBind(eachFields, eloc, sc, fromVars, toVars, r.each)
	if err != nil {
		return nil, err
	}
	for _, v := range toVars {
		if !contains(fromVars, v) && !contains(bindVars, v) {
			return nil, fmt.Errorf("%s: $%s in \"to\" is bound neither by \"from\" nor by \"bind\"", eloc, v)
		}
	}
	for _, v := range fromVars {
		if contains(toVars, v) {
			continue
		}
		if len(bindVars) == 0 {
			return nil, fmt.Errorf("%s: $%s in \"from\" does not appear in \"to\"; use it there or bind the key with \"bind\"", eloc, v)
		}
		r.each.join = true
	}
	inner := sc.with(fromVars...).with(toVars...).with(bindVars...)
	// Forward conditions run before the bind paths are read, so they may only
	// use the variables the iteration itself binds; reverse conditions run
	// with everything bound.
	if r.when, err = whenField(fields, loc, sc.with(fromVars...), inner); err != nil {
		return nil, err
	}
	var nested []json.RawMessage
	if raw, ok := fields["rules"]; ok {
		if err := json.Unmarshal(raw, &nested); err != nil {
			return nil, fmt.Errorf("%s: \"rules\" must be an array", loc)
		}
	}
	if r.rules, err = c.compileRules(nested, loc+".rules", inner); err != nil {
		return nil, err
	}
	r.desc = descField(fields)
	return r, nil
}

func (c *compiler) compileBind(fields map[string]json.RawMessage, loc string, sc *scope, fromVars, toVars []string, spec *eachSpec) ([]string, error) {
	raw, ok := fields["bind"]
	if !ok {
		return nil, nil
	}
	var bindMap map[string]string
	if err := json.Unmarshal(raw, &bindMap); err != nil {
		return nil, fmt.Errorf("%s: \"bind\" must map variables to paths, like {\"$k\": \"id\"}", loc)
	}
	names := make([]string, 0, len(bindMap))
	for name := range bindMap {
		names = append(names, name)
	}
	sort.Strings(names)
	var vars []string
	for _, name := range names {
		v := strings.TrimPrefix(name, "$")
		if !varNameRE.MatchString(v) {
			return nil, fmt.Errorf("%s: bind: invalid variable name %q", loc, name)
		}
		if sc.bound[v] {
			return nil, fmt.Errorf("%s: bind: $%s is already bound by an enclosing \"each\"", loc, v)
		}
		if contains(fromVars, v) {
			return nil, fmt.Errorf("%s: bind: $%s is already bound by \"from\"", loc, v)
		}
		if !contains(toVars, v) {
			return nil, fmt.Errorf("%s: bind: $%s is not used in \"to\"", loc, v)
		}
		anon := 0
		p, err := parsePath(bindMap[name], &anon)
		if err != nil {
			return nil, fmt.Errorf("%s: bind $%s: %w", loc, v, err)
		}
		if unb := unboundVarNames(p.vars, sc.with(fromVars...)); len(unb) > 0 {
			return nil, fmt.Errorf("%s: bind $%s: path uses unbound %s", loc, v, varList(unb))
		}
		spec.bind = append(spec.bind, binding{v: v, path: p})
		vars = append(vars, v)
	}
	return vars, nil
}

// --- field helpers ----------------------------------------------------------

func pathField(fields map[string]json.RawMessage, key, loc string, required bool) (*path, error) {
	raw, ok := fields[key]
	if !ok {
		if required {
			return nil, fmt.Errorf("%s: %q is required", loc, key)
		}
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("%s: %q must be a path string", loc, key)
	}
	anon := 0
	p, err := parsePath(s, &anon)
	if err != nil {
		return nil, fmt.Errorf("%s: %s: %w", loc, key, err)
	}
	return p, nil
}

func descField(fields map[string]json.RawMessage) string {
	var s string
	if raw, ok := fields["description"]; ok {
		_ = json.Unmarshal(raw, &s)
	}
	return s
}

func whenField(fields map[string]json.RawMessage, loc string, fwd, rev *scope) (*whenSpec, error) {
	raw, ok := fields["when"]
	if !ok {
		return nil, nil
	}
	var w map[string]json.RawMessage
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, fmt.Errorf("%s: \"when\" must be an object with \"forward\" and/or \"reverse\"", loc)
	}
	wloc := loc + ".when"
	if err := checkKeys(wloc, w, whenKeys); err != nil {
		return nil, err
	}
	if len(w) == 0 {
		return nil, fmt.Errorf("%s: needs \"forward\" and/or \"reverse\"", wloc)
	}
	spec := &whenSpec{}
	var err error
	if raw, ok := w["forward"]; ok {
		if spec.forward, err = parseConditions(raw, wloc+".forward", fwd); err != nil {
			return nil, err
		}
	}
	if raw, ok := w["reverse"]; ok {
		if spec.reverse, err = parseConditions(raw, wloc+".reverse", rev); err != nil {
			return nil, err
		}
	}
	return spec, nil
}

func parseConditions(raw json.RawMessage, loc string, sc *scope) ([]condition, error) {
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		list = []json.RawMessage{raw}
	}
	var conds []condition
	for i, item := range list {
		cloc := fmt.Sprintf("%s[%d]", loc, i)
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(item, &fields); err != nil {
			return nil, fmt.Errorf("%s: a condition must be an object", cloc)
		}
		if err := checkKeys(cloc, fields, condKeys); err != nil {
			return nil, err
		}
		p, err := pathField(fields, "path", cloc, true)
		if err != nil {
			return nil, err
		}
		if vars := unboundVars(p, sc); len(vars) > 0 {
			return nil, fmt.Errorf("%s: path %q uses unbound %s", cloc, p.raw, varList(vars))
		}
		cond := condition{path: p}
		for _, op := range []string{"exists", "equals", "not_equals", "in", "gt", "gte", "lt", "lte"} {
			rawOp, ok := fields[op]
			if !ok {
				continue
			}
			v, err := decodeJSON(rawOp)
			if err != nil {
				return nil, fmt.Errorf("%s: %s: %w", cloc, op, err)
			}
			if err := checkCondValue(op, v); err != nil {
				return nil, fmt.Errorf("%s: %w", cloc, err)
			}
			cond.ops = append(cond.ops, condOp{op: op, value: v})
		}
		if len(cond.ops) == 0 {
			return nil, fmt.Errorf("%s: a condition needs at least one of exists, equals, not_equals, in, gt, gte, lt, lte", cloc)
		}
		conds = append(conds, cond)
	}
	return conds, nil
}

func checkCondValue(op string, v any) error {
	switch op {
	case "exists":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("\"exists\" must be true or false")
		}
	case "in":
		if _, ok := v.([]any); !ok {
			return fmt.Errorf("\"in\" must be an array of values")
		}
	case "gt", "gte", "lt", "lte":
		if _, ok := numericValue(v); !ok {
			return fmt.Errorf("%q must be a number", op)
		}
	}
	return nil
}

// --- small utilities --------------------------------------------------------

func keySet(keys ...string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

func checkKeys(loc string, fields map[string]json.RawMessage, allowed map[string]bool) error {
	var unknown []string
	for k := range fields {
		if !allowed[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	known := make([]string, 0, len(allowed))
	for k := range allowed {
		known = append(known, k)
	}
	sort.Strings(known)
	return fmt.Errorf("%s: unknown key %q (allowed: %s)", loc, unknown[0], strings.Join(known, ", "))
}

func unboundVars(p *path, sc *scope) []string {
	return unboundVarNames(p.vars, sc)
}

func unboundVarNames(vars []string, sc *scope) []string {
	var out []string
	for _, v := range vars {
		if !sc.bound[v] {
			out = append(out, v)
		}
	}
	return out
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, v := range a {
		if !contains(b, v) {
			return false
		}
	}
	return true
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func varList(vars []string) string {
	if len(vars) == 0 {
		return "no variables"
	}
	names := make([]string, len(vars))
	for i, v := range vars {
		names[i] = "$" + v
	}
	return strings.Join(names, ", ")
}
