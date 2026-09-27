package mapping

import (
	"errors"
	"strings"
	"testing"
)

func TestCompileValid(t *testing.T) {
	cfg, err := Compile([]byte(`{
	  "$schema": "https://example.invalid/t1k.schema.json",
	  "name": "demo", "version": "1.2.3", "description": "d",
	  "definitions": {"pair": [{"from": "a", "to": "b", "description": "copy a"}]},
	  "rules": [
	    {"use": "pair", "description": "reuse"},
	    {"to": "const", "value": 1},
	    {"each": {"from": "list[$i]", "to": "map{item_$i}"}, "rules": [{"from": "x", "to": "y"}]},
	    {"each": {"from": "list[$i].sub", "to": "keyed{$k}", "bind": {"$k": "id"}}, "rules": [{"to": "node", "template": "$k"}]}
	  ]}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "demo" || cfg.Version != "1.2.3" || cfg.Description != "d" || cfg.RuleCount() != 4 {
		t.Errorf("metadata = %q %q %q, %d rules", cfg.Name, cfg.Version, cfg.Description, cfg.RuleCount())
	}
	if cfg.rules[0].desc != "copy a" || cfg.rules[0].kind != ruleCopy {
		t.Errorf("use did not splice the definition: %+v", cfg.rules[0])
	}
	if cfg.rules[2].each.join {
		t.Error("an each without bind must not be a join")
	}
	if !cfg.rules[3].each.join {
		t.Error("an each whose from has a variable to cannot supply must be a join")
	}
}

func TestCompileErrors(t *testing.T) {
	tests := []struct{ name, cfg, want string }{
		{"not an object", `[]`, "not a JSON object"},
		{"unknown top key", `{"name": "x", "rules": [], "extra": 1}`, `unknown key "extra"`},
		{"missing name", `{"rules": []}`, `non-empty "name"`},
		{"missing rules", `{"name": "x"}`, `"rules" array`},
		{"rule not object", `{"name": "x", "rules": [5]}`, "must be a JSON object"},
		{"unknown rule key", `{"name": "x", "rules": [{"from": "a", "to": "b", "too": "c"}]}`, `unknown key "too"`},
		{"copy missing to", `{"name": "x", "rules": [{"from": "a"}]}`, `"to" is required`},
		{"copy bad path", `{"name": "x", "rules": [{"from": "a..b", "to": "c"}]}`, "empty key"},
		{"copy path not string", `{"name": "x", "rules": [{"from": 1, "to": "c"}]}`, "must be a path string"},
		{"copy var mismatch", `{"name": "x", "rules": [{"from": "a[$i]", "to": "b[$j]"}]}`, "same variables"},
		{"copy var missing", `{"name": "x", "rules": [{"from": "a[$i]", "to": "b"}]}`, "binds $i but \"to\" binds no variables"},
		{"copy bad convert", `{"name": "x", "rules": [{"from": "a", "to": "b", "convert": "nope"}]}`, "unknown converter"},
		{"copy bad default", `{"name": "x", "rules": [{"from": "a", "to": "b", "default": }]}`, "invalid character"},
		{"constant both sides", `{"name": "x", "rules": [{"from": "a", "to": "b", "value": 1}]}`, "exactly one of"},
		{"constant no side", `{"name": "x", "rules": [{"value": 1}]}`, "exactly one of"},
		{"constant value and template", `{"name": "x", "rules": [{"to": "a", "value": 1, "template": "x"}]}`, "not both"},
		{"constant unbound var", `{"name": "x", "rules": [{"to": "a{$k}", "value": 1}]}`, "no enclosing \"each\" binds"},
		{"template unbound var", `{"name": "x", "rules": [{"to": "a", "template": "$k"}]}`, "no enclosing \"each\" binds"},
		{"template not string", `{"name": "x", "rules": [{"to": "a", "template": 5}]}`, "must be a string"},
		{"template invalid", `{"name": "x", "rules": [{"to": "a", "template": "$"}]}`, "invalid variable name"},
		{"scope each not object", `{"name": "x", "rules": [{"each": "a"}]}`, "must be an object"},
		{"scope unknown each key", `{"name": "x", "rules": [{"each": {"from": "a", "to": "b", "key": "id"}}]}`, `unknown key "key"`},
		{"scope to unbound", `{"name": "x", "rules": [{"each": {"from": "a[$i]", "to": "b{$k}"}}]}`, "$k in \"to\" is bound neither"},
		{"scope from unused", `{"name": "x", "rules": [{"each": {"from": "a[$i]", "to": "b"}}]}`, "does not appear in \"to\""},
		{"scope rules not array", `{"name": "x", "rules": [{"each": {"from": "a", "to": "b"}, "rules": 1}]}`, "must be an array"},
		{"bind not object", `{"name": "x", "rules": [{"each": {"from": "a[$i]", "to": "b{$k}", "bind": "id"}}]}`, "must map variables to paths"},
		{"bind bad name", `{"name": "x", "rules": [{"each": {"from": "a[$i]", "to": "b{$k}", "bind": {"$1": "id"}}}]}`, "invalid variable name"},
		{"bind unused", `{"name": "x", "rules": [{"each": {"from": "a[$i]", "to": "b{$i}", "bind": {"$k": "id"}}}]}`, "$k is not used in \"to\""},
		{"bind clashes from", `{"name": "x", "rules": [{"each": {"from": "a[$i]", "to": "b{$i}", "bind": {"$i": "id"}}}]}`, "already bound by \"from\""},
		{"bind clashes outer", `{"name": "x", "rules": [{"each": {"from": "a[$k]", "to": "b[$k]"}, "rules": [{"each": {"from": "c[$i]", "to": "d{$k}", "bind": {"$k": "id"}}}]}]}`, "already bound by an enclosing"},
		{"bind bad path", `{"name": "x", "rules": [{"each": {"from": "a[$i]", "to": "b{$k}", "bind": {"$k": "id."}}}]}`, "must not end with a dot"},
		{"bind unbound var in path", `{"name": "x", "rules": [{"each": {"from": "a[$i]", "to": "b{$k}", "bind": {"$k": "ids[$j]"}}}]}`, "path uses unbound $j"},
		{"use missing", `{"name": "x", "rules": [{"use": "nope"}]}`, `definition "nope" does not exist`},
		{"use not string", `{"name": "x", "rules": [{"use": 1}]}`, "must be the name of a definition"},
		{"use cycle", `{"name": "x", "definitions": {"a": [{"use": "b"}], "b": [{"use": "a"}]}, "rules": [{"use": "a"}]}`, "uses itself"},
		{"use inherits scope errors", `{"name": "x", "definitions": {"d": [{"to": "n", "template": "$k"}]}, "rules": [{"use": "d"}]}`, "definitions.d"},
		{"when not object", `{"name": "x", "rules": [{"from": "a", "to": "b", "when": 1}]}`, "must be an object"},
		{"when empty", `{"name": "x", "rules": [{"from": "a", "to": "b", "when": {}}]}`, `needs "forward" and/or "reverse"`},
		{"when unknown key", `{"name": "x", "rules": [{"from": "a", "to": "b", "when": {"always": {}}}]}`, `unknown key "always"`},
		{"when cond not object", `{"name": "x", "rules": [{"from": "a", "to": "b", "when": {"forward": 5}}]}`, "must be an object"},
		{"when no op", `{"name": "x", "rules": [{"from": "a", "to": "b", "when": {"forward": {"path": "c"}}}]}`, "at least one of"},
		{"when unknown op", `{"name": "x", "rules": [{"from": "a", "to": "b", "when": {"forward": {"path": "c", "like": 1}}}]}`, `unknown key "like"`},
		{"when exists not bool", `{"name": "x", "rules": [{"from": "a", "to": "b", "when": {"forward": {"path": "c", "exists": 1}}}]}`, "must be true or false"},
		{"when in not array", `{"name": "x", "rules": [{"from": "a", "to": "b", "when": {"forward": {"path": "c", "in": 1}}}]}`, "must be an array"},
		{"when gt not number", `{"name": "x", "rules": [{"from": "a", "to": "b", "when": {"forward": {"path": "c", "gt": "1"}}}]}`, "must be a number"},
		{"when unbound var", `{"name": "x", "rules": [{"from": "a", "to": "b", "when": {"forward": {"path": "c[$z]", "exists": true}}}]}`, "unbound $z"},
		{"when forward may not use bind", `{"name": "x", "rules": [{"each": {"from": "a[$i]", "to": "b{$k}", "bind": {"$k": "id"}}, "when": {"forward": {"path": "/c{$k}", "exists": true}}}]}`, "unbound $k"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile([]byte(tc.cfg))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !errors.Is(err, ErrConfig) {
				t.Errorf("error is not ErrConfig: %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestCompileErrorPositions(t *testing.T) {
	// Every error names the position of the rule, including rules spliced in
	// from a definition and nested ones.
	tests := []struct{ name, cfg, want string }{
		{"top-level field of the wrong type", `{"name": "x", "rules": [], "definitions": 5}`, "cannot unmarshal"},
		{"use with unknown key", `{"name": "x", "definitions": {"d": []}, "rules": [{"use": "d", "extra": 1}]}`, `rules[0]: unknown key "extra"`},
		{"constant with unknown key", `{"name": "x", "rules": [{"to": "a", "value": 1, "bogus": 2}]}`, `rules[0]: unknown key "bogus"`},
		{"constant with a bad path", `{"name": "x", "rules": [{"from": "a..b", "value": 1}]}`, "rules[0]: from: path"},
		{"constant path not a string", `{"name": "x", "rules": [{"to": 5, "value": 1}]}`, `rules[0]: "to" must be a path string`},
		{"constant with a bad condition", `{"name": "x", "rules": [{"to": "a", "value": 1, "when": {"forward": {"path": "x"}}}]}`, "rules[0].when.forward[0]: a condition needs"},
		{"scope with unknown key", `{"name": "x", "rules": [{"each": {"from": "a", "to": "b"}, "bogus": 1}]}`, `rules[0]: unknown key "bogus"`},
		{"scope with bad from", `{"name": "x", "rules": [{"each": {"from": "a..", "to": "b"}}]}`, "rules[0].each: from: path"},
		{"scope with bad to", `{"name": "x", "rules": [{"each": {"from": "a", "to": "b["}}]}`, "rules[0].each: to: path"},
		{"reverse condition error", `{"name": "x", "rules": [{"from": "a", "to": "b", "when": {"reverse": {"path": "c"}}}]}`, "rules[0].when.reverse[0]: a condition needs"},
		{"condition with a bad path", `{"name": "x", "rules": [{"from": "a", "to": "b", "when": {"forward": {"path": "c..d", "exists": true}}}]}`, "rules[0].when.forward[0]: path: path"},
		{"nested rule error", `{"name": "x", "rules": [{"each": {"from": "l[$i]", "to": "m[$i]"}, "rules": [{"from": "a"}]}]}`, `rules[0].rules[0]: "to" is required`},
		{"definition rule error", `{"name": "x", "definitions": {"d": [{"from": "a", "to": "b[$z]"}]}, "rules": [{"use": "d"}]}`, "rules[0](definitions.d)[0]"},
		{"nested use cycle", `{"name": "x", "definitions": {"a": [{"use": "b"}], "b": [{"use": "c"}], "c": [{"use": "a"}]}, "rules": [{"use": "a"}]}`, "uses itself (via a > b > c)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile([]byte(tc.cfg))
			if err == nil || !errors.Is(err, ErrConfig) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want ErrConfig mentioning %q", err, tc.want)
			}
		})
	}
}

func TestCompileAcceptsUnusualButValidShapes(t *testing.T) {
	cfg := `{
	  "$schema": "ignored",
	  "name": "shapes",
	  "definitions": {"d": [{"from": "x", "to": "y"}], "e": [{"use": "d"}, {"to": "n", "template": "$k"}]},
	  "rules": [
	    {"description": 5, "from": "a", "to": "b"},
	    {"each": {"from": "m{$k}", "to": "o{$k}"}, "rules": [{"use": "e"}]},
	    {"each": {"from": "l[$i]", "to": "p{$i}"}, "rules": [{"use": "d"}]},
	    {"each": {"from": "q", "to": "r"}},
	    {"each": {"from": "s", "to": "t"}, "rules": []},
	    {"to": "u", "value": null},
	    {"from": "v", "to": "w", "default": null, "reverse_default": [1, {"x": null}]},
	    {"from": "aa[$i][$j]", "to": "bb{$j}[$i]"},
	    {"from": "cc", "to": "dd", "when": {"forward": [], "reverse": []}}
	  ]}`
	p, err := Compile([]byte(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if p.RuleCount() != 9 {
		t.Errorf("RuleCount = %d", p.RuleCount())
	}
	if p.rules[0].desc != "" {
		t.Error("a non-string description is ignored")
	}
	// The same definition compiles per use site: once under $k, once under $i.
	if len(p.rules[1].rules) != 2 || len(p.rules[2].rules) != 1 {
		t.Errorf("definitions were not spliced as expected: %d, %d", len(p.rules[1].rules), len(p.rules[2].rules))
	}
	if !p.rules[6].def.set || p.rules[6].def.value != nil || !p.rules[6].revDef.set {
		t.Error("null defaults must count as set")
	}
	if p.rules[8].when == nil || len(p.rules[8].when.forward) != 0 {
		t.Error("empty condition lists are allowed and mean no conditions")
	}
}

func TestCompileBindShapes(t *testing.T) {
	p, err := Compile([]byte(`{"name": "b", "rules": [
	  {"each": {"from": "l[$i]", "to": "m{${a}-${b}}", "bind": {"$b": "kind", "$a": "group.name"}}},
	  {"each": {"from": "l[$i].sub", "to": "n{$k}", "bind": {"$k": "/root.id"}}}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	// Bind entries are compiled in name order, so the model is deterministic.
	if b := p.rules[0].each.bind; len(b) != 2 || b[0].variable != "a" || b[1].variable != "b" {
		t.Errorf("bind order = %+v", b)
	}
	// $i is not in "to" and bind variables exist: both scopes join in reverse.
	if !p.rules[0].each.join || !p.rules[1].each.join {
		t.Error("a from variable missing from to selects join mode")
	}
}
