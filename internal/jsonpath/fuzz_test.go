package jsonpath

import (
	"strings"
	"testing"
)

// FuzzParse checks that no expression can make the parser panic and that
// every accepted expression round-trips through String().
func FuzzParse(f *testing.F) {
	for _, seed := range []string{"a.b[0]", "topology[$i].from", "model.nodes{$k}", "m{${p}_$i}", "items[*].n{*}", "/x", ".", `"q.k".z`, "a{$$}", "a[", "a{$k$j}", ""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, expr string) {
		p, err := Parse(expr)
		if err != nil {
			return
		}
		again, err := Parse(p.String())
		if err != nil {
			t.Fatalf("Parse(%q) accepted but Parse(String()=%q) failed: %v", expr, p.String(), err)
		}
		if stepsString(p) != stepsString(again) {
			t.Fatalf("%q: steps differ after round trip", expr)
		}
		if len(p.Vars()) != len(p.Unbound(Bindings{})) {
			t.Fatalf("%q: all variables must be unbound with empty bindings", expr)
		}
		_, _ = p.Resolve(nil, Bindings{})
		_, _ = p.Expand(map[string]any{"a": []any{1}}, map[string]any{}, Bindings{})
	})
}

// FuzzParseTemplate checks that no template text panics and that a
// rendered key parses back to the bindings it was rendered from.
func FuzzParseTemplate(f *testing.F) {
	for _, seed := range []string{"$k", "line_$i", "${p}_$i", "a-$b-c", "$$", "*", "", "$", "$a$b"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		tmpl, err := ParseTemplate(text)
		if err != nil {
			return
		}
		b := Bindings{}
		for i, v := range tmpl.Vars() {
			b[v] = strings.Repeat("v", i+1)
		}
		key, err := tmpl.Render(b)
		if err != nil {
			t.Fatalf("%q: Render with every variable bound failed: %v", text, err)
		}
		if _, ok := tmpl.match(key, Bindings{}); !ok {
			t.Fatalf("%q: rendered key %q does not match its own template", text, key)
		}
	})
}
