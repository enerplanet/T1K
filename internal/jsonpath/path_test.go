package jsonpath

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/enerplanet/T1K/internal/jsondoc"
)

// stepsString renders parsed steps in a compact form for table tests.
func stepsString(p *Path) string {
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
			parts = append(parts, "var("+s.variable+")")
		case stepTemplate:
			parts = append(parts, "map("+s.template.text+")")
		}
	}
	return strings.Join(parts, " ")
}

func TestParse(t *testing.T) {
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
			p, err := Parse(tc.in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.in, err)
			}
			if got := stepsString(p); got != tc.steps {
				t.Errorf("steps = %q, want %q", got, tc.steps)
			}
			if got := strings.Join(p.Vars(), " "); got != tc.vars {
				t.Errorf("vars = %q, want %q", got, tc.vars)
			}
			if p.String() != tc.in {
				t.Errorf("String() = %q, want the expression as written", p.String())
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
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
			_, err := Parse(tc.in)
			if err == nil {
				t.Fatalf("Parse(%q): expected an error", tc.in)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestTemplate(t *testing.T) {
	tmpl, err := ParseTemplate("${p}_$i")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tmpl.Vars(), " "); got != "p i" {
		t.Errorf("Vars = %q", got)
	}
	b, ok := tmpl.match("lv_0", Bindings{})
	if !ok || b["p"] != "lv" || b["i"] != "0" {
		t.Fatalf("match lv_0 = %v, %v", b, ok)
	}
	// Earlier variables take the shortest match.
	b, ok = tmpl.match("a_b_c", Bindings{})
	if !ok || b["p"] != "a" || b["i"] != "b_c" {
		t.Fatalf("match a_b_c = %v, %v", b, ok)
	}
	// A bound variable must agree with the key.
	if _, ok := tmpl.match("lv_0", Bindings{"p": "mv"}); ok {
		t.Fatal("expected mismatch for bound p=mv")
	}
	if _, ok := tmpl.match("nounderscore", Bindings{}); ok {
		t.Fatal("expected no match without the literal separator")
	}
	s, err := tmpl.Render(Bindings{"p": "mv", "i": "7"})
	if err != nil || s != "mv_7" {
		t.Fatalf("Render = %q, %v", s, err)
	}
	if _, err := tmpl.Render(Bindings{"p": "mv"}); err == nil {
		t.Fatal("Render with an unbound variable should fail")
	}

	lit, err := ParseTemplate("cost$$center")
	if err != nil {
		t.Fatal(err)
	}
	if len(lit.Vars()) != 0 || lit.parts[0].literal != "cost$center" {
		t.Fatalf("literal template = %+v", lit.parts)
	}
	if _, ok := lit.match("cost$center", Bindings{}); !ok {
		t.Fatal("a literal template should match its text")
	}
	if _, err := ParseTemplate("$"); err == nil {
		t.Fatal("a bare $ should be rejected")
	}
	if anon, err := ParseTemplate("*"); err != nil || strings.Join(anon.Vars(), "") != "_1" {
		t.Fatalf("anonymous template = %v, %v", anon, err)
	}
}

func mustPath(t *testing.T, s string) *Path {
	t.Helper()
	p, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return p
}

func mustDecode(t *testing.T, s string) any {
	t.Helper()
	v, err := jsondoc.Decode([]byte(s))
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
		b    Bindings
		want []want
	}{
		{"name", nil, []want{{"", `"root"`, true}}},
		{"missing.key", nil, []want{{"", "", false}}},
		{"items[1].id", nil, []want{{"", `"b"`, true}}},
		{"items[5].id", nil, []want{{"", "", false}}},
		{"items[$i].id", nil, []want{{"i=0", `"a"`, true}, {"i=1", `"b"`, true}}},
		{"items[$i].id", Bindings{"i": "1"}, []want{{"i=1", `"b"`, true}}},
		{"items[$i].id", Bindings{"i": "9"}, []want{{"i=9", "", false}}},
		{"byKey{line_$n}.v", nil, []want{{"n=0", "10", true}, {"n=1", "11", true}}},
		{"byKey{$k}.v", nil, []want{{"k=line_0", "10", true}, {"k=line_1", "11", true}, {"k=other", "12", true}}},
		{"byKey{line_$n}.v", Bindings{"n": "1"}, []want{{"n=1", "11", true}}},
		{"byKey{line_$n}.v", Bindings{"n": "7"}, []want{{"n=7", "", false}}},
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
				b = Bindings{}
			}
			matches, err := mustPath(t, tc.path).Expand(doc, doc, b)
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) != len(tc.want) {
				t.Fatalf("got %d matches, want %d: %+v", len(matches), len(tc.want), matches)
			}
			for i, m := range matches {
				if got := bindingsString(m.Bindings); got != tc.want[i].bindings {
					t.Errorf("match %d bindings = %q, want %q", i, got, tc.want[i].bindings)
				}
				if m.Present != tc.want[i].present {
					t.Errorf("match %d present = %v, want %v", i, m.Present, tc.want[i].present)
				}
				if m.Present && !jsondoc.Equal(m.Value, mustDecode(t, tc.want[i].value)) {
					t.Errorf("match %d value = %v, want %s", i, m.Value, tc.want[i].value)
				}
			}
		})
	}
}

func TestExpandAbsoluteAndErrors(t *testing.T) {
	doc := mustDecode(t, fixture)
	base := mustDecode(t, `{"name": "inner"}`)
	m, err := mustPath(t, "name").Expand(doc, base, Bindings{})
	if err != nil || !jsondoc.Equal(m[0].Value, "inner") {
		t.Fatalf("relative Expand = %v, %v", m, err)
	}
	m, err = mustPath(t, "/name").Expand(doc, base, Bindings{})
	if err != nil || !jsondoc.Equal(m[0].Value, "root") {
		t.Fatalf("absolute Expand = %v, %v", m, err)
	}
	if !mustPath(t, "/name").Absolute() || mustPath(t, "name").Absolute() {
		t.Error("Absolute()")
	}
	if _, err := mustPath(t, "items[$i].id").Expand(doc, doc, Bindings{"i": "x"}); err == nil {
		t.Fatal("a non-integer index binding should fail")
	}
	// Holes are invisible to readers.
	withHole := []any{hole{}, map[string]any{"id": "b"}}
	m, err = mustPath(t, "[$i].id").Expand(withHole, withHole, Bindings{})
	if err != nil || len(m) != 1 || m[0].Bindings["i"] != "1" {
		t.Fatalf("Expand over a hole = %+v, %v", m, err)
	}
	m, _ = mustPath(t, "[0]").Expand(withHole, withHole, Bindings{})
	if m[0].Present {
		t.Fatal("a hole must read as absent")
	}
}

func bindingsString(b Bindings) string {
	keys := make([]string, 0, len(b))
	for k := range b {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + b[k]
	}
	return strings.Join(parts, " ")
}

func TestResolveSetGet(t *testing.T) {
	p := mustPath(t, "model.nodes{$k}.techs[$i].name")
	loc, err := p.Resolve(nil, Bindings{"k": "n1", "i": "2"})
	if err != nil {
		t.Fatal(err)
	}
	if got := loc.String(); got != "model.nodes.n1.techs[2].name" {
		t.Errorf("location = %q", got)
	}
	if _, err := p.Resolve(nil, Bindings{"k": "n1"}); err == nil {
		t.Error("an unbound $i should fail")
	}
	if _, err := p.Resolve(nil, Bindings{"k": "n1", "i": "two"}); err == nil {
		t.Error("a non-integer $i should fail")
	}
	rel, err := mustPath(t, "x.y").Resolve(Location{Key("base")}, Bindings{})
	if err != nil || rel.String() != "base.x.y" {
		t.Errorf("relative location = %q, %v", rel, err)
	}
	abs, err := mustPath(t, "/x.y").Resolve(Location{Key("base")}, Bindings{})
	if err != nil || abs.String() != "x.y" {
		t.Errorf("absolute location = %q, %v", abs, err)
	}
	if got := (Location{}).String(); got != "/" {
		t.Errorf("root location = %q", got)
	}

	var out any
	Set(&out, loc, "pv")
	v, ok := Get(out, loc)
	if !ok || v != "pv" {
		t.Errorf("Get = %v, %v", v, ok)
	}
	if _, ok := Get(out, Location{Key("model"), Key("nodes"), Key("n1"), Key("techs"), Index(0)}); ok {
		t.Error("a hole must not be readable")
	}
	if _, ok := Get(out, Location{Key("model"), Index(0)}); ok {
		t.Error("an index into an object must be absent")
	}
	got, _ := json.Marshal(Compact(out))
	if string(got) != `{"model":{"nodes":{"n1":{"techs":[{"name":"pv"}]}}}}` {
		t.Errorf("Set built %s", got)
	}
	// Writing through a scalar or null replaces it with a container.
	var scalar any = "text"
	Set(&scalar, Location{Key("a"), Index(1)}, 1)
	got, _ = json.Marshal(Compact(scalar))
	if string(got) != `{"a":[1]}` {
		t.Errorf("Set through a scalar built %s", got)
	}
	var whole any
	Set(&whole, nil, []any{1})
	if !jsondoc.Equal(whole, []any{json.Number("1")}) {
		t.Errorf("Set at the root = %v", whole)
	}
}

func TestCompact(t *testing.T) {
	v := map[string]any{
		"a": []any{hole{}, json.Number("1"), hole{}, []any{hole{}, nil}},
		"b": map[string]any{"c": []any{hole{}}},
	}
	got, _ := json.Marshal(Compact(v))
	if string(got) != `{"a":[1,[null]],"b":{"c":[]}}` {
		t.Errorf("Compact = %s", got)
	}
}

func TestParseMoreForms(t *testing.T) {
	tests := []struct {
		in    string
		steps string
	}{
		{`"k"[0]`, "key(k) idx(0)"},                      // a quoted key may carry suffixes
		{`""`, "key()"},                                  // an empty quoted key is a key
		{`"a\\b"`, `key(a\b)`},                           // a backslash escapes itself
		{"//a", "abs key(/a)"},                           // only the first slash is special
		{"a{x $k}", "key(a) map(x $k)"},                  // literal text may contain spaces
		{"a{$k$$}", "key(a) map($k$$)"},                  // "$$" is a literal dollar sign
		{"a{${a}-${b}}", "key(a) map(${a}-${b})"},        // braced variables in a template
		{"a[0]{$k}[$i]", "key(a) idx(0) map($k) var(i)"}, // suffixes chain
		{"a.{$k}", "key(a) map($k)"},                     // hmm: "a." then "{$k}"? see below
	}
	for _, tc := range tests[:len(tests)-1] {
		t.Run(tc.in, func(t *testing.T) {
			p, err := Parse(tc.in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.in, err)
			}
			if got := stepsString(p); got != tc.steps {
				t.Errorf("steps = %q, want %q", got, tc.steps)
			}
		})
	}
	// A segment consisting of a suffix alone is only allowed at the start.
	if _, err := Parse("a.{$k}"); err == nil || !strings.Contains(err.Error(), "only allowed at the start") {
		t.Errorf("Parse(a.{$k}) = %v", err)
	}
	if _, err := Parse("a[ 0 ]"); err == nil {
		t.Error("spaces inside an index must be rejected")
	}
	if _, err := Parse("a{${a}${b}}"); err == nil || !strings.Contains(err.Error(), "need literal text") {
		t.Errorf("adjacent braced variables: %v", err)
	}
	if _, err := Parse("a{a$}"); err == nil || !strings.Contains(err.Error(), "invalid variable name") {
		t.Errorf("dangling $: %v", err)
	}
	if _, err := ParseTemplate("${a"); err == nil || !strings.Contains(err.Error(), "unterminated") {
		t.Errorf("ParseTemplate(${a) = %v", err)
	}
}

// TestParseStringRoundTrip pins that a parsed path renders as it was written
// and that parsing that text again yields the same steps.
func TestParseStringRoundTrip(t *testing.T) {
	for _, expr := range []string{"a.b[0]{$k}", "/x{line_$i}.y", ".", "/", `"q.k".z`, "items[*].n{*}"} {
		p := mustPath(t, expr)
		again := mustPath(t, p.String())
		if stepsString(p) != stepsString(again) || strings.Join(p.Vars(), ",") != strings.Join(again.Vars(), ",") {
			t.Errorf("%q does not round-trip through String()", expr)
		}
	}
}

func TestVarsAndBindingsAreIsolated(t *testing.T) {
	p := mustPath(t, "a[$i]{$k}")
	vars := p.Vars()
	vars[0] = "changed"
	if p.Vars()[0] != "i" {
		t.Error("Vars must return a copy")
	}
	b := Bindings{"i": "0"}
	c := b.Clone()
	c["i"] = "1"
	c["k"] = "x"
	if b["i"] != "0" || len(b) != 1 {
		t.Error("Clone must not share the map")
	}
	if !ValidVarName("_a1") || ValidVarName("1a") || ValidVarName("") || ValidVarName("a-b") {
		t.Error("ValidVarName")
	}
}

func TestExpandEdgeCases(t *testing.T) {
	doc := mustDecode(t, `{"items": [{"id": "a", "tags": ["x", "y"]}, {"id": "b", "tags": ["z"]}], "map": {"k": [1, 2]}, "n": 5}`)
	// A bound index selects even when written with a leading zero.
	m, err := mustPath(t, "items[$i].id").Expand(doc, doc, Bindings{"i": "01"})
	if err != nil || len(m) != 1 || !m[0].Present || m[0].Value != "b" {
		t.Errorf("leading-zero index = %+v, %v", m, err)
	}
	// A bound index on a value that is not an array is absent, not an error.
	m, err = mustPath(t, "n[$i]").Expand(doc, doc, Bindings{"i": "0"})
	if err != nil || len(m) != 1 || m[0].Present {
		t.Errorf("index into a scalar = %+v, %v", m, err)
	}
	// An out-of-range index is absent.
	m, err = mustPath(t, "items[$i].id").Expand(doc, doc, Bindings{"i": "7"})
	if err != nil || len(m) != 1 || m[0].Present {
		t.Errorf("out-of-range index = %+v, %v", m, err)
	}
	// Errors inside iterations propagate: the inner index is bound to text.
	if _, err := mustPath(t, "items[$i].tags[$j]").Expand(doc, doc, Bindings{"j": "x"}); err == nil {
		t.Error("a bad inner binding must fail while iterating an array")
	}
	if _, err := mustPath(t, "map{$k}[$j]").Expand(doc, doc, Bindings{"j": "x"}); err == nil {
		t.Error("a bad inner binding must fail while iterating an object")
	}
	if _, err := mustPath(t, "items[$i]").Expand(doc, doc, Bindings{"i": "99999999999999999999"}); err == nil {
		t.Error("an index beyond int range must fail")
	}
	if _, err := mustPath(t, "items[$i]").Expand(doc, doc, Bindings{"i": "-1"}); err == nil {
		t.Error("a negative index must fail")
	}
	// Iterating a template over a document with a hole skips the hole.
	holed := map[string]any{"a": []any{hole{}}}
	m, err = mustPath(t, "a[$i]").Expand(holed, holed, Bindings{})
	if err != nil || len(m) != 0 {
		t.Errorf("holes must not match: %+v, %v", m, err)
	}
}

func TestResolveAndLocationEdgeCases(t *testing.T) {
	loc, err := mustPath(t, "a[2].b").Resolve(nil, Bindings{})
	if err != nil || loc.String() != "a[2].b" {
		t.Errorf("literal index = %q, %v", loc, err)
	}
	if _, err := mustPath(t, "m{$k}").Resolve(nil, Bindings{}); err == nil || !strings.Contains(err.Error(), "$k is not bound") {
		t.Errorf("unbound template variable = %v", err)
	}
	if _, err := mustPath(t, "m{$k}").Resolve(nil, Bindings{"k": ""}); err != nil {
		t.Errorf("an empty key is a valid key: %v", err)
	}
	loc, _ = mustPath(t, "[0].x[1]").Resolve(nil, Bindings{})
	if loc.String() != "[0].x[1]" {
		t.Errorf("index-first location = %q", loc)
	}
	loc, _ = mustPath(t, "a[$i]").Resolve(nil, Bindings{"i": "007"})
	if loc.String() != "a[7]" {
		t.Errorf("leading zeros = %q", loc)
	}
	// Get through a hole or a missing key is absent.
	var out any
	Set(&out, Location{Key("a"), Index(2)}, "v")
	if _, ok := Get(out, Location{Key("a"), Index(0), Key("x")}); ok {
		t.Error("reading through a hole must be absent")
	}
	if _, ok := Get(out, Location{Key("missing")}); ok {
		t.Error("a missing key must be absent")
	}
	if _, ok := Get("scalar", Location{Key("a")}); ok {
		t.Error("a key on a scalar must be absent")
	}
	if v, ok := Get(out, nil); !ok || v == nil {
		t.Error("the root location reads the whole document")
	}
}

func TestSetReplacesWrongContainers(t *testing.T) {
	// An object in the way of an index becomes an array, and an array in the
	// way of a key becomes an object; explicit nulls are kept by Compact.
	var doc any = map[string]any{"a": map[string]any{"old": true}}
	Set(&doc, Location{Key("a"), Index(1)}, "v")
	got, _ := json.Marshal(Compact(doc))
	if string(got) != `{"a":["v"]}` {
		t.Errorf("index into an object built %s", got)
	}
	doc = []any{"x"}
	Set(&doc, Location{Key("k")}, nil)
	got, _ = json.Marshal(Compact(doc))
	if string(got) != `{"k":null}` {
		t.Errorf("key into an array built %s", got)
	}
	// Writing over a hole fills it.
	doc = nil
	Set(&doc, Location{Index(1)}, 1)
	Set(&doc, Location{Index(0)}, 0)
	got, _ = json.Marshal(Compact(doc))
	if string(got) != `[0,1]` {
		t.Errorf("filling a hole built %s", got)
	}
	// Compact leaves scalars and nulls alone.
	if Compact(nil) != nil || Compact("s") != "s" {
		t.Error("Compact of scalars")
	}
}
