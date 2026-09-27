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
