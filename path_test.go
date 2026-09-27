package t1k

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// stepsString renders parsed steps in a compact form for table tests.
func stepsString(p *path) string {
	var parts []string
	if p.absolute {
		parts = append(parts, "abs")
	}
	for _, s := range p.steps {
		switch s.kind {
		case stepKey:
			parts = append(parts, "key("+s.key+")")
		case stepIndex:
			parts = append(parts, fmt.Sprintf("idx(%d)", s.index))
		case stepIndexVar:
			parts = append(parts, "var("+s.v+")")
		case stepMap:
			parts = append(parts, "map("+s.tmpl.raw+")")
		}
	}
	return strings.Join(parts, " ")
}

func TestParsePath(t *testing.T) {
	tests := []struct {
		in    string
		steps string
		vars  string
	}{
		{"a", "key(a)", ""},
		{"a.b.c", "key(a) key(b) key(c)", ""},
		{"a[0]", "key(a) idx(0)", ""},
		{"a[0][1].b", "key(a) idx(0) idx(1) key(b)", ""},
		{"a[$i].b", "key(a) var(i) key(b)", "i"},
		{"a[${i}]", "key(a) var(i)", "i"},
		{"a{$k}", "key(a) map($k)", "k"},
		{"a{line_$i}.d", "key(a) map(line_$i) key(d)", "i"},
		{"a{${p}_$i}", "key(a) map(${p}_$i)", "p i"},
		{"a{$t-$k}", "key(a) map($t-$k)", "t k"},
		{"a{fixed}", "key(a) key(fixed)", ""},
		{"a[*].b[*]", "key(a) var(_1) key(b) var(_2)", "_1 _2"},
		{"a{*}", "key(a) map(*)", "_1"},
		{"/a.b", "abs key(a) key(b)", ""},
		{"/", "abs", ""},
		{".", "", ""},
		{"[0].x", "idx(0) key(x)", ""},
		{"{$k}.x", "map($k) key(x)", "k"},
		{`"my.key".x`, "key(my.key) key(x)", ""},
		{`"quoted \"name\"".x`, `key(quoted "name") key(x)`, ""},
		{"a[$i].b[$i]", "key(a) var(i) key(b) var(i)", "i"},
		{" a.b ", "key(a) key(b)", ""},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			anon := 0
			p, err := parsePath(tc.in, &anon)
			if err != nil {
				t.Fatalf("parsePath(%q): %v", tc.in, err)
			}
			if got := stepsString(p); got != tc.steps {
				t.Errorf("steps = %q, want %q", got, tc.steps)
			}
			if got := strings.Join(p.vars, " "); got != tc.vars {
				t.Errorf("vars = %q, want %q", got, tc.vars)
			}
		})
	}
}

func TestParsePathErrors(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", "empty path"},
		{"a..b", "empty key"},
		{"a.", "must not end with a dot"},
		{"a[", "missing"},
		{"a{", "missing"},
		{"a{${p", "missing"},
		{"a[-1]", "invalid array index"},
		{"a[1x]", "invalid array index"},
		{"a[$1x]", "invalid variable name"},
		{"a{$k$j}", "need literal text between them"},
		{"a{$k-$k}", "appears twice"},
		{"a{}", "empty"},
		{"a{$}", "invalid variable name"},
		{"$x", "only allowed inside"},
		{"a.[0]", "only allowed at the start"},
		{"a]b", "unexpected"},
		{`"open`, "unterminated quoted key"},
		{`"esc\`, "unterminated escape"},
		{"a{x}}", "unexpected"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			anon := 0
			_, err := parsePath(tc.in, &anon)
			if err == nil {
				t.Fatalf("parsePath(%q): expected an error", tc.in)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestKeyTemplate(t *testing.T) {
	anon := 0
	tmpl, err := parseKeyTemplate("${p}_$i", &anon)
	if err != nil {
		t.Fatal(err)
	}
	b, ok := tmpl.match("lv_0", bindings{})
	if !ok || b["p"] != "lv" || b["i"] != "0" {
		t.Fatalf("match lv_0 = %v, %v", b, ok)
	}
	// Earlier variables take the shortest match.
	b, ok = tmpl.match("a_b_c", bindings{})
	if !ok || b["p"] != "a" || b["i"] != "b_c" {
		t.Fatalf("match a_b_c = %v, %v", b, ok)
	}
	// A bound variable must agree with the key.
	if _, ok := tmpl.match("lv_0", bindings{"p": "mv"}); ok {
		t.Fatal("expected mismatch for bound p=mv")
	}
	if _, ok := tmpl.match("nounderscore", bindings{}); ok {
		t.Fatal("expected no match without the literal separator")
	}
	s, err := tmpl.render(bindings{"p": "mv", "i": "7"})
	if err != nil || s != "mv_7" {
		t.Fatalf("render = %q, %v", s, err)
	}
	if _, err := tmpl.render(bindings{"p": "mv"}); err == nil {
		t.Fatal("render with unbound variable should fail")
	}

	lit, err := parseKeyTemplate("cost$$center", &anon)
	if err != nil {
		t.Fatal(err)
	}
	if len(lit.vars) != 0 || lit.parts[0].lit != "cost$center" {
		t.Fatalf("literal template = %+v", lit.parts)
	}
	if _, ok := lit.match("cost$center", bindings{}); !ok {
		t.Fatal("literal template should match its text")
	}
	if _, err := parseKeyTemplate("$", &anon); err == nil {
		t.Fatal("bare $ should be rejected")
	}
}

func mustPath(t *testing.T, s string) *path {
	t.Helper()
	anon := 0
	p, err := parsePath(s, &anon)
	if err != nil {
		t.Fatalf("parsePath(%q): %v", s, err)
	}
	return p
}

func mustDecode(t *testing.T, s string) any {
	t.Helper()
	v, err := decodeJSON([]byte(s))
	if err != nil {
		t.Fatalf("decode %s: %v", s, err)
	}
	return v
}

const fixture = `{
  "name": "root",
  "items": [
    {"id": "a", "n": 1},
    {"id": "b", "n": 2}
  ],
  "byKey": {"line_0": {"v": 10}, "line_1": {"v": 11}, "other": {"v": 12}},
  "nested": {"list": [[1, 2], [3]]},
  "my.key": {"x": true}
}`

func TestExpand(t *testing.T) {
	doc := mustDecode(t, fixture)
	type want struct {
		bindings string
		value    string
		present  bool
	}
	tests := []struct {
		path string
		b    bindings
		want []want
	}{
		{"name", nil, []want{{"", `"root"`, true}}},
		{"missing.key", nil, []want{{"", "", false}}},
		{"items[1].id", nil, []want{{"", `"b"`, true}}},
		{"items[5].id", nil, []want{{"", "", false}}},
		{"items[$i].id", nil, []want{{"i=0", `"a"`, true}, {"i=1", `"b"`, true}}},
		{"items[$i].id", bindings{"i": "1"}, []want{{"i=1", `"b"`, true}}},
		{"items[$i].id", bindings{"i": "9"}, []want{{"i=9", "", false}}},
		{"byKey{line_$n}.v", nil, []want{{"n=0", "10", true}, {"n=1", "11", true}}},
		{"byKey{$k}.v", nil, []want{{"k=line_0", "10", true}, {"k=line_1", "11", true}, {"k=other", "12", true}}},
		{"byKey{line_$n}.v", bindings{"n": "1"}, []want{{"n=1", "11", true}}},
		{"byKey{line_$n}.v", bindings{"n": "7"}, []want{{"n=7", "", false}}},
		{"nested.list[$i][$j]", nil, []want{{"i=0 j=0", "1", true}, {"i=0 j=1", "2", true}, {"i=1 j=0", "3", true}}},
		{"name.deeper", nil, []want{{"", "", false}}},
		{"items{$k}", nil, nil},
		{`"my.key".x`, nil, []want{{"", "true", true}}},
		{".", nil, []want{{"", fixture, true}}},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			b := tc.b
			if b == nil {
				b = bindings{}
			}
			matches, err := mustPath(t, tc.path).expand(doc, doc, b)
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) != len(tc.want) {
				t.Fatalf("got %d matches, want %d: %+v", len(matches), len(tc.want), matches)
			}
			for i, m := range matches {
				if got := bindingsString(m.bindings); got != tc.want[i].bindings {
					t.Errorf("match %d bindings = %q, want %q", i, got, tc.want[i].bindings)
				}
				if m.present != tc.want[i].present {
					t.Errorf("match %d present = %v, want %v", i, m.present, tc.want[i].present)
				}
				if m.present && !equalJSON(m.value, mustDecode(t, tc.want[i].value)) {
					t.Errorf("match %d value = %v, want %s", i, m.value, tc.want[i].value)
				}
			}
		})
	}
}

func TestExpandAbsoluteAndErrors(t *testing.T) {
	doc := mustDecode(t, fixture)
	base := mustDecode(t, `{"name": "inner"}`)
	m, err := mustPath(t, "name").expand(doc, base, bindings{})
	if err != nil || !equalJSON(m[0].value, "inner") {
		t.Fatalf("relative expand = %v, %v", m, err)
	}
	m, err = mustPath(t, "/name").expand(doc, base, bindings{})
	if err != nil || !equalJSON(m[0].value, "root") {
		t.Fatalf("absolute expand = %v, %v", m, err)
	}
	if _, err := mustPath(t, "items[$i].id").expand(doc, doc, bindings{"i": "x"}); err == nil {
		t.Fatal("non-integer index binding should fail")
	}
	// Holes are invisible to readers.
	withHole := []any{hole{}, map[string]any{"id": "b"}}
	m, err = mustPath(t, "[$i].id").expand(withHole, withHole, bindings{})
	if err != nil || len(m) != 1 || m[0].bindings["i"] != "1" {
		t.Fatalf("expand over hole = %+v, %v", m, err)
	}
	m, _ = mustPath(t, "[0]").expand(withHole, withHole, bindings{})
	if m[0].present {
		t.Fatal("a hole must read as absent")
	}
}

func bindingsString(b bindings) string {
	keys := make([]string, 0, len(b))
	for k := range b {
		keys = append(keys, k)
	}
	sortStrings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + b[k]
	}
	return strings.Join(parts, " ")
}

func TestResolveSetGet(t *testing.T) {
	p := mustPath(t, "model.nodes{$k}.techs[$i].name")
	loc, err := p.resolve(nil, bindings{"k": "n1", "i": "2"})
	if err != nil {
		t.Fatal(err)
	}
	if got := loc.String(); got != "model.nodes.n1.techs[2].name" {
		t.Errorf("location = %q", got)
	}
	if _, err := p.resolve(nil, bindings{"k": "n1"}); err == nil {
		t.Error("unbound $i should fail")
	}
	if _, err := p.resolve(nil, bindings{"k": "n1", "i": "two"}); err == nil {
		t.Error("non-integer $i should fail")
	}
	rel, err := mustPath(t, "x.y").resolve(location{{key: "base"}}, bindings{})
	if err != nil || rel.String() != "base.x.y" {
		t.Errorf("relative location = %q, %v", rel, err)
	}
	abs, err := mustPath(t, "/x.y").resolve(location{{key: "base"}}, bindings{})
	if err != nil || abs.String() != "x.y" {
		t.Errorf("absolute location = %q, %v", abs, err)
	}
	if got := (location{}).String(); got != "/" {
		t.Errorf("root location = %q", got)
	}

	var out any
	setAt(&out, loc, "pv")
	v, ok := getAt(out, loc)
	if !ok || v != "pv" {
		t.Errorf("getAt = %v, %v", v, ok)
	}
	if _, ok := getAt(out, location{{key: "model"}, {key: "nodes"}, {key: "n1"}, {key: "techs"}, {index: 0, isIndex: true}}); ok {
		t.Error("a hole must not be readable")
	}
	got, _ := json.Marshal(compact(out))
	if string(got) != `{"model":{"nodes":{"n1":{"techs":[{"name":"pv"}]}}}}` {
		t.Errorf("setAt built %s", got)
	}
	// Writing through a scalar or null replaces it with a container.
	var scalar any = "text"
	setAt(&scalar, location{{key: "a"}, {index: 1, isIndex: true}}, 1)
	got, _ = json.Marshal(compact(scalar))
	if string(got) != `{"a":[1]}` {
		t.Errorf("setAt through scalar built %s", got)
	}
	var whole any
	setAt(&whole, nil, []any{1})
	if !equalJSON(whole, []any{json.Number("1")}) {
		t.Errorf("setAt at root = %v", whole)
	}
}

func TestCompact(t *testing.T) {
	v := map[string]any{
		"a": []any{hole{}, json.Number("1"), hole{}, []any{hole{}, nil}},
		"b": map[string]any{"c": []any{hole{}}},
	}
	got, _ := json.Marshal(compact(v))
	if string(got) != `{"a":[1,[null]],"b":{"c":[]}}` {
		t.Errorf("compact = %s", got)
	}
}
