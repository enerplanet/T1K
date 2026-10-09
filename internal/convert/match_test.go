package convert

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMatch(t *testing.T) {
	runConvCases(t, []convCase{
		{name: "identifier passes", spec: `{"match": {"pattern": "[^_\\d]\\w*"}}`, in: `"n1"`, want: `"n1"`},
		{name: "number is matched by its text", spec: `{"match": {"pattern": "\\d+"}}`, in: `42`, want: `42`},
		{name: "number keeps its digits", spec: `{"match": {"pattern": "[0-9.]+"}}`, in: `1.50`, want: `1.50`},
		{name: "boolean is matched by its text", spec: `{"match": {"pattern": "true|false"}}`, in: `false`, want: `false`},
		{name: "anchored pattern means the same", spec: `{"match": {"pattern": "^[a-z]+$"}}`, in: `"abc"`, want: `"abc"`},
		{name: "absent", spec: `{"match": {"pattern": ".*"}}`, in: "", want: ""},
		{name: "leading digit", spec: `{"match": {"pattern": "[^_\\d]\\w*"}}`, in: `"1"`, wantErr: `match: "1" does not match "[^_\\d]\\w*"`},
		{name: "partial match is not enough", spec: `{"match": {"pattern": "[a-z]+"}}`, in: `"abc-def"`, wantErr: "does not match"},
		{name: "empty string against a non-empty pattern", spec: `{"match": {"pattern": "\\w+"}}`, in: `""`, wantErr: "does not match"},
		{name: "null has no text", spec: `{"match": {"pattern": ".*"}}`, in: `null`, wantErr: "match: null has no text"},
		{name: "arrays have no text", spec: `{"match": {"pattern": ".*"}}`, in: `[1]`, wantErr: "match: array has no text"},
		{name: "unicode text", spec: `{"match": {"pattern": "\\p{L}+"}}`, in: `"Straße"`, want: `"Straße"`},
		{name: "newlines need an explicit flag", spec: `{"match": {"pattern": "(?s).+"}}`, in: `"a\nb"`, want: `"a\nb"`},
	})
	c := mustConv(t, `{"match": {"pattern": "[a-z]+"}}`)
	if _, _, err := c.Reverse("ABC", true); err == nil {
		t.Error("the guard applies in reverse too")
	}
	if _, _, err := c.Forward("a\nb", true); err == nil {
		t.Error("without (?s) a dot does not cross a newline, so the whole value does not match")
	}
}

func TestMatchArguments(t *testing.T) {
	tests := []struct{ spec, want string }{
		{`{"match": {}}`, `argument "pattern" is required`},
		{`{"match": {"pattern": ""}}`, `argument "pattern" is required`},
		{`{"match": {"pattern": 5}}`, `argument "pattern" must be a string`},
		{`{"match": {"pattern": "("}}`, "pattern: error parsing regexp"},
		{`{"match": {"pattern": "a", "flags": "i"}}`, `unknown argument "flags"`},
	}
	for _, tc := range tests {
		t.Run(tc.spec, func(t *testing.T) {
			_, err := Parse(json.RawMessage(tc.spec))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
