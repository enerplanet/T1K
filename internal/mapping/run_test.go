package mapping

import (
	"errors"
	"strings"
	"testing"

	"github.com/enerplanet/T1K/internal/jsondoc"
)

func mustDecode(t *testing.T, s string) any {
	t.Helper()
	v, err := jsondoc.Decode([]byte(s))
	if err != nil {
		t.Fatalf("decode %s: %v", s, err)
	}
	return v
}

// runRules applies a configuration (given only its rules JSON) to a document
// and returns the compact JSON result.
func runRules(t *testing.T, rules, input string, dir Direction) (string, error) {
	t.Helper()
	cfg, err := Compile([]byte(`{"name": "test", "rules": ` + rules + `}`))
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	out, err := cfg.Run(dir, mustDecode(t, input))
	if err != nil {
		return "", err
	}
	data, err := jsondoc.Encode(out, "", "")
	if err != nil {
		t.Fatal(err)
	}
	return string(data), nil
}

func canon(t *testing.T, s string) string {
	t.Helper()
	data, err := jsondoc.Encode(mustDecode(t, s), "", "")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type engineCase struct {
	name    string
	rules   string
	input   string
	forward string // expected Transform output ("" to skip)
	reverse string // expected Reverse output of the forward result ("" to skip); reverseInput overrides the input
	revIn   string
}

func runEngineCases(t *testing.T, cases []engineCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fwd, err := runRules(t, tc.rules, tc.input, Forward)
			if err != nil {
				t.Fatalf("forward: %v", err)
			}
			if tc.forward != "" && fwd != canon(t, tc.forward) {
				t.Errorf("forward\n got %s\nwant %s", fwd, canon(t, tc.forward))
			}
			if tc.reverse == "" {
				return
			}
			revIn := fwd
			if tc.revIn != "" {
				revIn = tc.revIn
			}
			rev, err := runRules(t, tc.rules, revIn, Reverse)
			if err != nil {
				t.Fatalf("reverse: %v", err)
			}
			if rev != canon(t, tc.reverse) {
				t.Errorf("reverse\n got %s\nwant %s", rev, canon(t, tc.reverse))
			}
		})
	}
}

func TestEngineCopyAndConstants(t *testing.T) {
	runEngineCases(t, []engineCase{
		{
			name:    "rename and nest",
			rules:   `[{"from": "a", "to": "x.y.z"}, {"from": "b.c", "to": "w"}]`,
			input:   `{"a": 1, "b": {"c": [1, 2]}, "ignored": true}`,
			forward: `{"x": {"y": {"z": 1}}, "w": [1, 2]}`,
			reverse: `{"a": 1, "b": {"c": [1, 2]}}`,
		},
		{
			name:    "empty rules give an empty object",
			rules:   `[]`,
			input:   `{"a": 1}`,
			forward: `{}`,
			reverse: `{}`,
		},
		{
			name:    "absent source writes nothing",
			rules:   `[{"from": "missing", "to": "x"}]`,
			input:   `{"a": 1}`,
			forward: `{}`,
		},
		{
			name:    "null is a value",
			rules:   `[{"from": "a", "to": "x"}]`,
			input:   `{"a": null}`,
			forward: `{"x": null}`,
			reverse: `{"a": null}`,
		},
		{
			name:    "constants per direction and templates",
			rules:   `[{"to": "kind", "value": "target"}, {"from": "kind", "value": "source"}, {"to": "obj", "value": {"k": [1, {"n": null}]}}, {"from": "a", "to": "b"}]`,
			input:   `{"a": 1, "kind": "source"}`,
			forward: `{"kind": "target", "obj": {"k": [1, {"n": null}]}, "b": 1}`,
			reverse: `{"kind": "source", "a": 1}`,
		},
		{
			name:    "later rules win",
			rules:   `[{"from": "a", "to": "x"}, {"from": "b", "to": "x"}, {"to": "x", "value": "const"}]`,
			input:   `{"a": 1, "b": 2}`,
			forward: `{"x": "const"}`,
		},
		{
			name:    "writing through a scalar or null",
			rules:   `[{"to": "x", "value": null}, {"from": "a", "to": "x.y"}, {"to": "z", "value": 1}, {"from": "a", "to": "z[1]"}]`,
			input:   `{"a": 1}`,
			forward: `{"x": {"y": 1}, "z": [1]}`,
		},
		{
			name:    "defaults",
			rules:   `[{"from": "a", "to": "x", "default": "dx", "reverse_default": "da"}, {"from": "b", "to": "y", "default": null}]`,
			input:   `{}`,
			forward: `{"x": "dx", "y": null}`,
			reverse: `{"a": "da", "b": null}`,
			revIn:   `{"y": null}`,
		},
		{
			name:    "default does not fire for iterated paths",
			rules:   `[{"from": "list[$i].a", "to": "out[$i]", "default": 0}]`,
			input:   `{"list": [{"a": 1}, {"b": 2}]}`,
			forward: `{"out": [1]}`,
		},
		{
			name:    "convert with absent and reverse default",
			rules:   `[{"from": "cap", "to": "max", "convert": [{"absent": {"values": ["inf"]}}, {"linear": {"divisor": 1000}}], "reverse_default": "inf"}]`,
			input:   `{"cap": "inf"}`,
			forward: `{}`,
			reverse: `{"cap": "inf"}`,
		},
		{
			name:    "convert numbers",
			rules:   `[{"from": "cap", "to": "max", "convert": {"linear": {"divisor": 1000}}}]`,
			input:   `{"cap": 8715}`,
			forward: `{"max": 8.715}`,
			reverse: `{"cap": 8715}`,
		},
		{
			name:    "self and root paths",
			rules:   `[{"from": ".", "to": "wrapped"}]`,
			input:   `{"a": 1}`,
			forward: `{"wrapped": {"a": 1}}`,
			reverse: `{"a": 1}`,
		},
		{
			name:    "root array input",
			rules:   `[{"from": "[$i].n", "to": "names[$i]"}]`,
			input:   `[{"n": "a"}, {"n": "b"}]`,
			forward: `{"names": ["a", "b"]}`,
			reverse: `[{"n": "a"}, {"n": "b"}]`,
		},
	})
}

func TestEngineCollections(t *testing.T) {
	runEngineCases(t, []engineCase{
		{
			name:    "array to array with anonymous variables",
			rules:   `[{"from": "items[*].name", "to": "things[*].label"}, {"from": "items[*].n", "to": "things[*].count"}]`,
			input:   `{"items": [{"name": "a", "n": 1}, {"name": "b"}]}`,
			forward: `{"things": [{"label": "a", "count": 1}, {"label": "b"}]}`,
			reverse: `{"items": [{"name": "a", "n": 1}, {"name": "b"}]}`,
		},
		{
			name:    "array to keyed object with an index template",
			rules:   `[{"from": "items[$i].v", "to": "byIndex{item_$i}.value"}]`,
			input:   `{"items": [{"v": 10}, {"v": 11}]}`,
			forward: `{"byIndex": {"item_0": {"value": 10}, "item_1": {"value": 11}}}`,
			reverse: `{"items": [{"v": 10}, {"v": 11}]}`,
		},
		{
			name:    "reverse compacts gaps and skips foreign keys",
			rules:   `[{"from": "items[$i].v", "to": "byIndex{item_$i}"}]`,
			input:   `{"items": []}`,
			reverse: `{"items": [{"v": 10}, {"v": 12}]}`,
			revIn:   `{"byIndex": {"item_0": 10, "item_5": 12, "other": 99}}`,
		},
		{
			name:    "object keys iterate",
			rules:   `[{"from": "map{$k}.v", "to": "copy{$k}"}]`,
			input:   `{"map": {"a": {"v": 1}, "b": {"v": 2}}}`,
			forward: `{"copy": {"a": 1, "b": 2}}`,
			reverse: `{"map": {"a": {"v": 1}, "b": {"v": 2}}}`,
		},
		{
			name:    "two-variable key template",
			rules:   `[{"from": "grid{$r}[$c]", "to": "cells{${r}-$c}"}]`,
			input:   `{"grid": {"x": [1, 2], "y": [3]}}`,
			forward: `{"cells": {"x-0": 1, "x-1": 2, "y-0": 3}}`,
			reverse: `{"grid": {"x": [1, 2], "y": [3]}}`,
		},
		{
			name: "scope with nested rules and constants",
			rules: `[{"each": {"from": "list[$i]", "to": "out{n_$i}"}, "rules": [
			          {"from": "a", "value": 0}, {"from": "a", "to": "x"}, {"to": "kind", "value": "k"}, {"to": "index", "template": "$i"}]}]`,
			input:   `{"list": [{"a": 1}, {"a": 2}]}`,
			forward: `{"out": {"n_0": {"x": 1, "kind": "k", "index": "0"}, "n_1": {"x": 2, "kind": "k", "index": "1"}}}`,
			reverse: `{"list": [{"a": 1}, {"a": 2}]}`,
		},
		{
			name:    "scope over a concrete path applies once",
			rules:   `[{"each": {"from": "cfg", "to": "settings"}, "rules": [{"from": "a", "to": "b"}, {"to": "c", "value": 1}]}]`,
			input:   `{"cfg": {"a": 1}}`,
			forward: `{"settings": {"b": 1, "c": 1}}`,
			reverse: `{"cfg": {"a": 1}}`,
		},
		{
			name:    "scope over an absent path applies nothing",
			rules:   `[{"each": {"from": "cfg", "to": "settings"}, "rules": [{"to": "c", "value": 1}]}]`,
			input:   `{}`,
			forward: `{}`,
		},
		{
			name: "absolute paths inside scopes",
			rules: `[{"each": {"from": "list[$i]", "to": "out[$i]"}, "rules": [
			          {"from": "a", "to": "/flat{a_$i}"}, {"from": "/meta.unit", "to": "unit"}]}]`,
			input:   `{"meta": {"unit": "kW"}, "list": [{"a": 1}, {"a": 2}]}`,
			forward: `{"flat": {"a_0": 1, "a_1": 2}, "out": [{"unit": "kW"}, {"unit": "kW"}]}`,
			reverse: `{"meta": {"unit": "kW"}, "list": [{"a": 1}, {"a": 2}]}`,
		},
		{
			name: "nested scopes",
			rules: `[{"each": {"from": "groups[$g]", "to": "byGroup{$g}"}, "rules": [
			          {"from": "name", "to": "label"},
			          {"each": {"from": "members[$m]", "to": "people{p_$m}"}, "rules": [{"from": "n", "to": "name"}, {"to": "group", "template": "$g"}]}]}]`,
			input:   `{"groups": [{"name": "A", "members": [{"n": "x"}, {"n": "y"}]}]}`,
			forward: `{"byGroup": {"0": {"label": "A", "people": {"p_0": {"name": "x", "group": "0"}, "p_1": {"name": "y", "group": "0"}}}}}`,
			reverse: `{"groups": [{"name": "A", "members": [{"n": "x"}, {"n": "y"}]}]}`,
		},
		{
			name: "bind in iterate mode writes the key back",
			rules: `[{"each": {"from": "conns[$i]", "to": "arcs{${p}_$i}", "bind": {"$p": "pipe"}}, "rules": [
			          {"from": "len", "to": "distance"}]}]`,
			input:   `{"conns": [{"pipe": "lv", "len": 1}, {"pipe": "mv", "len": 2}]}`,
			forward: `{"arcs": {"lv_0": {"distance": 1}, "mv_1": {"distance": 2}}}`,
			reverse: `{"conns": [{"pipe": "lv", "len": 1}, {"pipe": "mv", "len": 2}]}`,
		},
		{
			name: "bind in join mode",
			rules: `[
			  {"each": {"from": "conns[$i]", "to": "arcs{arc_$i}"}, "rules": [{"from": "from.id", "to": "from"}, {"from": "to.id", "to": "to"}]},
			  {"each": {"from": "conns[$i].from", "to": "nodes{$k}", "bind": {"$k": "id"}}, "rules": [{"from": "name", "to": "label"}]},
			  {"each": {"from": "conns[$i].to", "to": "nodes{$k}", "bind": {"$k": "id"}}, "rules": [{"from": "name", "to": "label"}]}
			]`,
			input: `{"conns": [
			  {"from": {"id": "a", "name": "A"}, "to": {"id": "t", "name": "T"}},
			  {"from": {"id": "b", "name": "B"}, "to": {"id": "t", "name": "T"}}]}`,
			forward: `{"arcs": {"arc_0": {"from": "a", "to": "t"}, "arc_1": {"from": "b", "to": "t"}},
			           "nodes": {"a": {"label": "A"}, "b": {"label": "B"}, "t": {"label": "T"}}}`,
			reverse: `{"conns": [
			  {"from": {"id": "a", "name": "A"}, "to": {"id": "t", "name": "T"}},
			  {"from": {"id": "b", "name": "B"}, "to": {"id": "t", "name": "T"}}]}`,
		},
		{
			name: "join with a missing entry leaves the element as created",
			rules: `[
			  {"each": {"from": "conns[$i]", "to": "arcs{arc_$i}"}, "rules": [{"from": "from.id", "to": "from"}]},
			  {"each": {"from": "conns[$i].from", "to": "nodes{$k}", "bind": {"$k": "id"}}, "rules": [{"from": "name", "to": "label"}]}
			]`,
			input:   `{"conns": []}`,
			reverse: `{"conns": [{"from": {"id": "a", "name": "A"}}, {"from": {"id": "zz"}}]}`,
			revIn:   `{"arcs": {"arc_0": {"from": "a"}, "arc_1": {"from": "zz"}}, "nodes": {"a": {"label": "A"}}}`,
		},
		{
			name: "join needs its elements first",
			rules: `[
			  {"each": {"from": "conns[$i].from", "to": "nodes{$k}", "bind": {"$k": "id"}}, "rules": [{"from": "name", "to": "label"}]},
			  {"each": {"from": "conns[$i]", "to": "arcs{arc_$i}"}, "rules": [{"from": "from.id", "to": "from"}]}
			]`,
			input:   `{"conns": []}`,
			reverse: `{"conns": [{"from": {"id": "a"}}]}`,
			revIn:   `{"arcs": {"arc_0": {"from": "a"}}, "nodes": {"a": {"label": "A"}}}`,
		},
		{
			name:    "bind from a numeric value",
			rules:   `[{"each": {"from": "list[$i]", "to": "byNum{$n}", "bind": {"$n": "num"}}, "rules": [{"from": "v", "to": "value"}]}]`,
			input:   `{"list": [{"num": 7, "v": "x"}]}`,
			forward: `{"byNum": {"7": {"value": "x"}}}`,
		},
	})
}

func TestEngineConditions(t *testing.T) {
	runEngineCases(t, []engineCase{
		{
			name:    "exists and equals",
			rules:   `[{"from": "a", "to": "x", "when": {"forward": {"path": "flag", "exists": true}}}, {"from": "b", "to": "y", "when": {"forward": [{"path": "kind", "equals": "k"}, {"path": "n", "gt": 1}]}}]`,
			input:   `{"a": 1, "b": 2, "kind": "k", "n": 2}`,
			forward: `{"y": 2}`,
		},
		{
			name:    "not_equals, in and comparisons",
			rules:   `[{"from": "a", "to": "x", "when": {"forward": {"path": "kind", "not_equals": "k"}}}, {"from": "a", "to": "y", "when": {"forward": {"path": "n", "in": [1, 2]}}}, {"from": "a", "to": "z", "when": {"forward": {"path": "n", "gte": 2, "lte": 2, "lt": 3}}}, {"from": "a", "to": "w", "when": {"forward": {"path": "kind", "gt": 0}}}]`,
			input:   `{"a": 1, "kind": "k", "n": 2}`,
			forward: `{"y": 1, "z": 1}`,
		},
		{
			name:    "conditions per direction",
			rules:   `[{"from": "a", "to": "x", "when": {"forward": {"path": "a", "gt": 0}, "reverse": {"path": "x", "gt": 5}}}]`,
			input:   `{"a": 1}`,
			forward: `{"x": 1}`,
			reverse: `{}`,
		},
		{
			name:    "scope condition per element and absolute condition with bound variable",
			rules:   `[{"each": {"from": "list[$i]", "to": "out{$i}"}, "when": {"forward": {"path": "on", "equals": true}, "reverse": {"path": "/flags{$i}", "exists": true}}, "rules": [{"from": "v", "to": "value"}]}]`,
			input:   `{"list": [{"on": true, "v": 1}, {"on": false, "v": 2}, {"v": 3}]}`,
			forward: `{"out": {"0": {"value": 1}}}`,
			reverse: `{"list": [{"v": 1}]}`,
			revIn:   `{"out": {"0": {"value": 1}, "1": {"value": 2}}, "flags": {"0": 1}}`,
		},
		{
			name:    "constant with condition",
			rules:   `[{"to": "kind", "value": "big", "when": {"forward": {"path": "n", "gt": 10}}}, {"to": "kind", "value": "small", "when": {"forward": {"path": "n", "lte": 10}}}]`,
			input:   `{"n": 3}`,
			forward: `{"kind": "small"}`,
		},
		{
			name:    "copy rule condition uses the iteration variable",
			rules:   `[{"from": "list[$i].v", "to": "out[$i]", "when": {"forward": {"path": "list[$i].on", "equals": true}}}]`,
			input:   `{"list": [{"on": true, "v": 1}, {"on": false, "v": 2}]}`,
			forward: `{"out": [1]}`,
		},
	})
}

func TestEngineErrors(t *testing.T) {
	tests := []struct {
		name  string
		rules string
		input string
		dir   Direction
		want  string
	}{
		{"converter failure names the rule", `[{"from": "a", "to": "b", "convert": "number"}]`, `{"a": true}`, Forward, "rules[0] (forward): a: number:"},
		{"converter failure in reverse", `[{"from": "a", "to": "b", "convert": "number"}]`, `{"b": "x"}`, Reverse, "rules[0] (reverse): b: number:"},
		{"bind path missing", `[{"each": {"from": "l[$i]", "to": "m{$k}", "bind": {"$k": "id"}}}]`, `{"l": [{"x": 1}]}`, Forward, `bind $k: "id" not found`},
		{"bind value not scalar", `[{"each": {"from": "l[$i]", "to": "m{$k}", "bind": {"$k": "id"}}}]`, `{"l": [{"id": {}}]}`, Forward, "bind $k"},
		{"non-integer key for an index", `[{"from": "l[$i]", "to": "m{$i}"}]`, `{"m": {"x": 1}}`, Reverse, "not an array index"},
		{"nested rule error keeps its location", `[{"each": {"from": "l[$i]", "to": "m[$i]"}, "rules": [{"from": "a", "to": "b", "convert": "number"}]}]`, `{"l": [{"a": "z"}]}`, Forward, "rules[0].rules[0] (forward)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runRules(t, tc.rules, tc.input, tc.dir)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !errors.Is(err, ErrRule) {
				t.Errorf("error is not ErrRule: %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestEngineOutputDoesNotAliasInput(t *testing.T) {
	cfg, err := Compile([]byte(`{"name": "t", "rules": [{"from": "a", "to": "b"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	in := mustDecode(t, `{"a": {"list": [1]}}`)
	out, err := cfg.Run(Forward, in)
	if err != nil {
		t.Fatal(err)
	}
	out.(map[string]any)["b"].(map[string]any)["list"].([]any)[0] = "changed"
	if in.(map[string]any)["a"].(map[string]any)["list"].([]any)[0] != mustDecode(t, "1") {
		t.Error("output shares memory with the input")
	}
}

func TestDirectionString(t *testing.T) {
	if Forward.String() != "forward" || Reverse.String() != "reverse" {
		t.Error("direction names")
	}
}

func TestEngineRuntimeErrorsInsideScopes(t *testing.T) {
	// Every place that resolves a variable reports a binding that is not an
	// array index, with the rule's position and the direction.
	tests := []struct {
		name  string
		rules string
		input string
		dir   Direction
		want  string
	}{
		{"copy source inside a scope", `[{"each": {"from": "m{$k}", "to": "n{$k}"}, "rules": [{"from": "/arr[$k]", "to": "v"}]}]`, `{"m": {"x": {}}, "arr": [1]}`, Forward, "rules[0].rules[0] (forward): path \"/arr[$k]\""},
		{"copy target inside a scope", `[{"each": {"from": "m{$k}", "to": "n{$k}"}, "rules": [{"from": "v", "to": "/arr[$k]"}]}]`, `{"m": {"x": {"v": 1}}}`, Forward, "rules[0].rules[0] (forward): path \"/arr[$k]\""},
		{"copy condition", `[{"each": {"from": "m{$k}", "to": "n{$k}"}, "rules": [{"from": "v", "to": "w", "when": {"forward": {"path": "/arr[$k]", "exists": true}}}]}]`, `{"m": {"x": {"v": 1}}, "arr": []}`, Forward, "rules[0].rules[0] (forward)"},
		{"constant condition", `[{"each": {"from": "m{$k}", "to": "n{$k}"}, "rules": [{"to": "w", "value": 1, "when": {"forward": {"path": "/arr[$k]", "exists": true}}}]}]`, `{"m": {"x": {}}, "arr": []}`, Forward, "rules[0].rules[0] (forward)"},
		{"scope source", `[{"each": {"from": "m{$k}", "to": "n{$k}"}, "rules": [{"each": {"from": "/arr[$k]", "to": "e"}}]}]`, `{"m": {"x": {}}, "arr": [1]}`, Forward, "rules[0].rules[0] (forward)"},
		{"scope condition", `[{"each": {"from": "m{$k}", "to": "n{$k}"}, "rules": [{"each": {"from": "s", "to": "e"}, "when": {"forward": {"path": "/arr[$k]", "exists": true}}}]}]`, `{"m": {"x": {"s": {}}}, "arr": []}`, Forward, "rules[0].rules[0] (forward)"},
		{"scope target", `[{"each": {"from": "m{$k}", "to": "arr[$k]"}}]`, `{"m": {"x": {}}}`, Forward, "rules[0] (forward): path \"arr[$k]\""},
		{"bind path", `[{"each": {"from": "m{$k}", "to": "n{$k}"}, "rules": [{"each": {"from": "s[$i]", "to": "/o{$p}", "bind": {"$p": "/ids[$k]"}}}]}]`, `{"m": {"x": {"s": [{}]}}, "ids": []}`, Forward, "rules[0].rules[0] (forward): path \"/ids[$k]\""},
		{"bind write in reverse", `[{"each": {"from": "m{$k}", "to": "n{$k}"}, "rules": [{"each": {"from": "s[$i]", "to": "o{${p}_$i}", "bind": {"$p": "/kinds[$k]"}}}]}]`, `{"n": {"x": {"o": {"a_0": {}}}}}`, Reverse, "rules[0].rules[0] (reverse): path \"/kinds[$k]\""},
		{"join lookup", `[{"from": "l[$i].ref.id", "to": "refs[$i]"}, {"each": {"from": "l[$i].ref", "to": "/arr[$p]", "bind": {"$p": "id"}}, "rules": [{"from": "x", "to": "y"}]}]`, `{"refs": ["abc"], "arr": []}`, Reverse, "rules[1] (reverse): path \"/arr[$p]\""},
		{"join condition", `[{"from": "l[$i].ref.id", "to": "refs[$i]"}, {"each": {"from": "l[$i].ref", "to": "/m{$p}", "bind": {"$p": "id"}}, "when": {"reverse": {"path": "/arr[$p]", "exists": true}}, "rules": [{"from": "x", "to": "y"}]}]`, `{"refs": ["abc"], "m": {"abc": {"y": 1}}, "arr": []}`, Reverse, "rules[1] (reverse): path \"/arr[$p]\""},
		{"join bind missing", `[{"from": "l[$i].ref.name", "to": "names[$i]"}, {"each": {"from": "l[$i].ref", "to": "/m{$p}", "bind": {"$p": "id"}}, "rules": [{"from": "x", "to": "y"}]}]`, `{"names": ["n"], "m": {}}`, Reverse, `bind $p: "id" not found`},
		{"join nested rule", `[{"from": "l[$i].ref.id", "to": "refs[$i]"}, {"each": {"from": "l[$i].ref", "to": "/m{$p}", "bind": {"$p": "id"}}, "rules": [{"from": "x", "to": "/arr[$p]"}]}]`, `{"refs": ["abc"], "m": {"abc": {}}, "arr": []}`, Reverse, "rules[1].rules[0] (reverse): path \"/arr[$p]\""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runRules(t, tc.rules, tc.input, tc.dir)
			if err == nil || !errors.Is(err, ErrRule) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want ErrRule mentioning %q", err, tc.want)
			}
		})
	}
}

func TestEngineJoinEdgeCases(t *testing.T) {
	runEngineCases(t, []engineCase{
		{
			// A nested join whose parent element was never created in the output has nothing to join onto.
			name: "join without a created base",
			rules: `[{"each": {"from": "groups{$g}", "to": "byGroup{$g}"}, "rules": [
			          {"each": {"from": "members[$i].ref", "to": "/people{$p}", "bind": {"$p": "id"}}, "rules": [{"from": "name", "to": "label"}]}]}]`,
			input:   `{"groups": {}}`,
			reverse: `{}`,
			revIn:   `{"byGroup": {"a": {}}, "people": {"x": {"label": "X"}}}`,
		},
		{
			name: "join skips elements whose reverse condition fails",
			rules: `[{"from": "l[$i].ref.id", "to": "ids[$i]"},
			         {"each": {"from": "l[$i].ref", "to": "m{$p}", "bind": {"$p": "id"}}, "when": {"reverse": {"path": "on", "equals": true}}, "rules": [{"from": "x", "to": "y"}]}]`,
			input:   `{"l": []}`,
			reverse: `{"l": [{"ref": {"id": "a", "x": 1}}, {"ref": {"id": "b"}}]}`,
			revIn:   `{"ids": ["a", "b"], "m": {"a": {"on": true, "y": 1}, "b": {"on": false, "y": 2}}}`,
		},
		{
			name: "join reads bind values from nested paths and numbers",
			rules: `[{"from": "l[$i].ref.meta.num", "to": "nums[$i]"},
			         {"each": {"from": "l[$i].ref", "to": "m{n_$p}", "bind": {"$p": "meta.num"}}, "rules": [{"from": "x", "to": "y"}]}]`,
			input:   `{"l": [{"ref": {"meta": {"num": 7}, "x": "seven"}}]}`,
			forward: `{"nums": [7], "m": {"n_7": {"y": "seven"}}}`,
			reverse: `{"l": [{"ref": {"meta": {"num": 7}, "x": "seven"}}]}`,
		},
		{
			name:    "forward dedupes repeated keyed elements",
			rules:   `[{"each": {"from": "l[$i]", "to": "m{$k}", "bind": {"$k": "id"}}, "rules": [{"from": "v", "to": "w"}, {"to": "seen", "value": true}]}]`,
			input:   `{"l": [{"id": "a", "v": 1}, {"id": "a", "v": 2}, {"id": "b"}]}`,
			forward: `{"m": {"a": {"w": 2, "seen": true}, "b": {"seen": true}}}`,
		},
	})
}

func TestEngineReverseIterateWritesKeysAsStrings(t *testing.T) {
	// In iterate mode the reverse writes bound variables back as strings,
	// even when the forward direction read them from numbers; the key of an
	// object cannot carry a type.
	runEngineCases(t, []engineCase{{
		name:    "numeric bind value comes back as a string",
		rules:   `[{"each": {"from": "l[$i]", "to": "m{${n}_$i}", "bind": {"$n": "num"}}, "rules": [{"from": "v", "to": "w"}]}]`,
		input:   `{"l": [{"num": 7, "v": 1}]}`,
		forward: `{"m": {"7_0": {"w": 1}}}`,
		reverse: `{"l": [{"num": "7", "v": 1}]}`,
	}})
}

func TestEngineRootsAndNulls(t *testing.T) {
	runEngineCases(t, []engineCase{
		{
			name:    "null input yields only constants",
			rules:   `[{"from": "a", "to": "b"}, {"to": "c", "value": 1}, {"from": "d", "value": 2}]`,
			input:   `null`,
			forward: `{"c": 1}`,
			reverse: `{"d": 2}`,
			revIn:   `null`,
		},
		{
			name:    "array input",
			rules:   `[{"from": "[0]", "to": "first"}, {"from": "[$i].n", "to": "names[$i]"}]`,
			input:   `[{"n": "a"}, {"n": "b"}]`,
			forward: `{"first": {"n": "a"}, "names": ["a", "b"]}`,
		},
		{
			name:    "writing the root replaces the output",
			rules:   `[{"from": "wrapped", "to": "/"}]`,
			input:   `{"wrapped": [1, 2]}`,
			forward: `[1, 2]`,
			reverse: `{"wrapped": [1, 2]}`,
		},
		{
			name:    "a later root write wins over earlier keys",
			rules:   `[{"to": "a", "value": 1}, {"to": "/", "value": "flat"}]`,
			input:   `{}`,
			forward: `"flat"`,
		},
		{
			name:    "explicit nulls survive compaction next to holes",
			rules:   `[{"from": "m{item_$i}", "to": "l[$i]"}]`,
			input:   `{"m": {"item_0": null, "item_2": 2}}`,
			forward: `{"l": [null, 2]}`,
		},
	})
}

func TestEngineConditionOperators(t *testing.T) {
	runEngineCases(t, []engineCase{
		{
			name: "in without a member and exists false on a present value",
			rules: `[{"from": "a", "to": "x", "when": {"forward": {"path": "n", "in": [1, 2]}}},
			         {"from": "a", "to": "y", "when": {"forward": {"path": "n", "exists": false}}},
			         {"from": "a", "to": "z", "when": {"forward": {"path": "missing", "not_equals": 1}}},
			         {"from": "a", "to": "w", "when": {"forward": {"path": "s", "gte": 1}}},
			         {"from": "a", "to": "v", "when": {"forward": {"path": "n", "in": [3, "x", null]}}}]`,
			input:   `{"a": 1, "n": 3, "s": "text"}`,
			forward: `{"z": 1, "v": 1}`,
		},
		{
			name:    "comparisons are numeric across representations",
			rules:   `[{"from": "a", "to": "x", "when": {"forward": [{"path": "n", "gt": 2.5}, {"path": "n", "lt": 3.5}, {"path": "n", "equals": 3.0}]}}]`,
			input:   `{"a": 1, "n": 3}`,
			forward: `{"x": 1}`,
		},
		{
			name:    "objects and arrays compare structurally in equals and in",
			rules:   `[{"from": "a", "to": "x", "when": {"forward": {"path": "o", "equals": {"k": [1, {"z": null}]}}}}, {"from": "a", "to": "y", "when": {"forward": {"path": "o", "in": [1, {"k": [1, {"z": null}]}]}}}]`,
			input:   `{"a": 1, "o": {"k": [1.0, {"z": null}]}}`,
			forward: `{"x": 1, "y": 1}`,
		},
	})
}

func TestEngineTemplatesAndConstants(t *testing.T) {
	runEngineCases(t, []engineCase{
		{
			name:    "templates render literal dollars and several variables",
			rules:   `[{"each": {"from": "g{$a}[$i]", "to": "out{$a}[$i]"}, "rules": [{"to": "id", "template": "$$-${a}-$i"}]}]`,
			input:   `{"g": {"x": [{}, {}]}}`,
			forward: `{"out": {"x": [{"id": "$-x-0"}, {"id": "$-x-1"}]}}`,
		},
		{
			name:    "constants copy their value so repeated writes do not alias",
			rules:   `[{"each": {"from": "l[$i]", "to": "o[$i]"}, "rules": [{"to": "cfg", "value": {"list": [1]}}]}, {"to": "o[0].cfg.list[1]", "value": 2}]`,
			input:   `{"l": [{}, {}]}`,
			forward: `{"o": [{"cfg": {"list": [1, 2]}}, {"cfg": {"list": [1]}}]}`,
		},
		{
			name:    "a reverse-only constant is ignored forward and vice versa",
			rules:   `[{"from": "kind", "value": "src"}, {"to": "kind", "value": "dst"}]`,
			input:   `{"kind": "whatever"}`,
			forward: `{"kind": "dst"}`,
			reverse: `{"kind": "src"}`,
		},
	})
}

// FuzzCompile checks that no configuration text can panic the compiler.
func FuzzCompile(f *testing.F) {
	for _, seed := range []string{
		`{"name": "x", "rules": []}`,
		`{"name": "x", "rules": [{"from": "a[$i]", "to": "b{item_$i}", "convert": {"linear": {"divisor": 1000}}}]}`,
		`{"name": "x", "definitions": {"d": [{"use": "d"}]}, "rules": [{"use": "d"}]}`,
		`{"name": "x", "rules": [{"each": {"from": "l[$i].f", "to": "m{$k}", "bind": {"$k": "id"}}, "when": {"forward": {"path": "x", "gt": 0}}, "rules": [{"to": "n", "template": "$k"}]}]}`,
		`[]`, `{`, `{"name": 1}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := Compile(data)
		if err != nil {
			if !errors.Is(err, ErrConfig) {
				t.Fatalf("compile error does not wrap ErrConfig: %v", err)
			}
			return
		}
		for _, in := range []any{nil, map[string]any{"a": []any{map[string]any{"id": "x"}}}, []any{"s"}} {
			for _, dir := range []Direction{Forward, Reverse} {
				if _, err := p.Run(dir, in); err != nil && !errors.Is(err, ErrRule) {
					t.Fatalf("run error does not wrap ErrRule: %v", err)
				}
			}
		}
	})
}

// FuzzRun checks that a fixed, feature-rich program never panics on
// arbitrary documents and only ever fails with ErrRule.
func FuzzRun(f *testing.F) {
	program, err := Compile([]byte(`{"name": "fuzz", "rules": [
	  {"from": "a.b", "to": "x.y", "convert": [{"absent": {"values": ["inf"]}}, {"linear": {"divisor": 10}}], "reverse_default": "inf"},
	  {"to": "k", "value": 1},
	  {"each": {"from": "l[$i]", "to": "m{${p}_$i}", "bind": {"$p": "kind"}}, "when": {"forward": {"path": "on", "not_equals": false}}, "rules": [{"from": "v", "to": "w", "convert": "number"}]},
	  {"each": {"from": "l[$i].ref", "to": "n{$q}", "bind": {"$q": "id"}}, "rules": [{"from": "name", "to": "label"}, {"to": "node", "template": "$q"}]}
	]}`))
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range []string{`{"a": {"b": "5"}, "l": [{"kind": "x", "v": "1", "ref": {"id": "r", "name": "R"}}]}`, `{"m": {"x_0": {"w": 2}}, "n": {"r": {"label": "L"}}}`, `null`, `[]`, `{"l": [{"kind": 1}]}`, `{"m": {"x_a": {}}}`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		in, err := jsondoc.Decode(data)
		if err != nil {
			return
		}
		for _, dir := range []Direction{Forward, Reverse} {
			out, err := program.Run(dir, in)
			if err != nil {
				if !errors.Is(err, ErrRule) {
					t.Fatalf("run error does not wrap ErrRule: %v", err)
				}
				continue
			}
			if _, err := jsondoc.Encode(out, "", ""); err != nil {
				t.Fatalf("output does not encode: %v", err)
			}
		}
	})
}
