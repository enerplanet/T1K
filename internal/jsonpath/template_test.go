package jsonpath

import (
	"strings"
	"testing"
)

// TestTemplateRejectsInvalidUTF8 pins a defect the fuzzer found: a template
// with invalid UTF-8 reached the regular expression compiler and panicked.
// It is an error now, in a path and in a template on its own.
func TestTemplateRejectsInvalidUTF8(t *testing.T) {
	for _, text := range []string{"$A\xcf", "\xa4$k", "x\xff"} {
		if _, err := ParseTemplate(text); err == nil || !strings.Contains(err.Error(), "invalid UTF-8") {
			t.Errorf("ParseTemplate(%q) = %v, want an invalid UTF-8 error", text, err)
		}
		if _, err := Parse("a{" + text + "}"); err == nil {
			t.Errorf("Parse with template %q must fail, not panic", text)
		}
	}
}

// TestTemplateMatchesKeysWithNewlines pins that a variable matches any
// character of a key, including a newline, which a JSON key may contain.
func TestTemplateMatchesKeysWithNewlines(t *testing.T) {
	doc := map[string]any{"m": map[string]any{"a\nb": 1, "line_x\ny": 2}}
	m, err := mustPath(t, "m{$k}").Expand(doc, doc, Bindings{})
	if err != nil || len(m) != 2 {
		t.Fatalf("Expand over keys with newlines = %+v, %v", m, err)
	}
	m, err = mustPath(t, "m{line_$k}").Expand(doc, doc, Bindings{})
	if err != nil || len(m) != 1 || m[0].Bindings["k"] != "x\ny" {
		t.Fatalf("template with a literal prefix = %+v, %v", m, err)
	}
	tmpl, err := ParseTemplate("$a-$b")
	if err != nil {
		t.Fatal(err)
	}
	if b, ok := tmpl.match("1\n2-3\n4", Bindings{}); !ok || b["a"] != "1\n2" || b["b"] != "3\n4" {
		t.Errorf("match with newlines = %v, %v", b, ok)
	}
}
