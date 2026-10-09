package convert

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAffix(t *testing.T) {
	runConvCases(t, []convCase{
		{name: "prefix", spec: `{"affix": {"prefix": "n"}}`, in: `"1"`, want: `"n1"`},
		{name: "suffix", spec: `{"affix": {"suffix": "_node"}}`, in: `"1"`, want: `"1_node"`},
		{name: "both", spec: `{"affix": {"prefix": "[", "suffix": "]"}}`, in: `"x"`, want: `"[x]"`},
		{name: "number becomes text", spec: `{"affix": {"prefix": "n"}}`, in: `7`, want: `"n7"`, back: `"7"`},
		{name: "number keeps its digits", spec: `{"affix": {"prefix": "n"}}`, in: `7.50`, want: `"n7.50"`, back: `"7.50"`},
		{name: "boolean becomes text", spec: `{"affix": {"suffix": "!"}}`, in: `true`, want: `"true!"`, back: `"true"`},
		{name: "empty value is wrapped and unwrapped", spec: `{"affix": {"prefix": "n"}}`, in: `""`, want: `"n"`},
		{name: "value that already carries the prefix", spec: `{"affix": {"prefix": "n"}}`, in: `"n1"`, want: `"nn1"`},
		{name: "unicode affixes", spec: `{"affix": {"prefix": "→", "suffix": "←"}}`, in: `"é"`, want: `"→é←"`},
		{name: "overlapping affixes need the full length", spec: `{"affix": {"prefix": "aa", "suffix": "a"}}`, in: `"a"`, want: `"aaaa"`},
		{name: "lenient reverse passes a bare value through", spec: `{"affix": {"prefix": "n", "strict": false}}`, in: `"1"`, want: `"n1"`},
		{name: "absent", spec: `{"affix": {"prefix": "n"}}`, in: "", want: ""},
		{name: "null cannot be wrapped", spec: `{"affix": {"prefix": "n"}}`, in: `null`, wantErr: "affix: cannot wrap null"},
		{name: "objects cannot be wrapped", spec: `{"affix": {"prefix": "n"}}`, in: `{"a": 1}`, wantErr: "affix: cannot wrap object"},
	})
}

func TestAffixReverse(t *testing.T) {
	strict := mustConv(t, `{"affix": {"prefix": "n", "suffix": "_x"}}`)
	lenient := mustConv(t, `{"affix": {"prefix": "n", "suffix": "_x", "strict": false}}`)
	tests := []struct {
		in      string // JSON
		want    string // strict result, "" for an error
		lenient string // lenient result
		err     string
	}{
		{`"n1_x"`, `"1"`, `"1"`, ""},
		{`"n_x"`, `""`, `""`, ""},
		{`"1_x"`, "", `"1_x"`, `"1_x" does not carry the prefix "n" and the suffix "_x"`},
		{`"n1"`, "", `"n1"`, "does not carry"},
		{`"n"`, "", `"n"`, "does not carry"},     // shorter than prefix + suffix
		{`"_xn"`, "", `"_xn"`, "does not carry"}, // right pieces, wrong order
		{`5`, "", "", "expected a string, got number"},
		{`null`, "", "", "expected a string, got null"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			v, present, err := strict.Reverse(mustDecode(t, tc.in), true)
			if tc.want == "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("strict reverse error = %v, want %q", err, tc.err)
				}
			} else {
				if err != nil {
					t.Fatalf("strict reverse: %v", err)
				}
				checkValue(t, "strict", v, present, tc.want)
			}
			v, present, err = lenient.Reverse(mustDecode(t, tc.in), true)
			if tc.lenient == "" {
				if err == nil {
					t.Fatalf("lenient reverse of %s should still reject non-strings", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("lenient reverse: %v", err)
			}
			checkValue(t, "lenient", v, present, tc.lenient)
		})
	}
}

func TestAffixArguments(t *testing.T) {
	tests := []struct{ spec, want string }{
		{`{"affix": {}}`, `at least one of "prefix" and "suffix"`},
		{`{"affix": {"prefix": "", "suffix": ""}}`, `at least one of "prefix" and "suffix"`},
		{`{"affix": {"prefix": 1}}`, `argument "prefix" must be a string`},
		{`{"affix": {"suffix": true}}`, `argument "suffix" must be a string`},
		{`{"affix": {"prefix": "n", "strict": "no"}}`, `argument "strict" must be a boolean`},
		{`{"affix": {"prefix": "n", "infix": "x"}}`, `unknown argument "infix"`},
	}
	for _, tc := range tests {
		t.Run(tc.spec, func(t *testing.T) {
			_, err := Parse(json.RawMessage(tc.spec))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	if _, err := Parse(json.RawMessage(`"affix"`)); err == nil {
		t.Error("affix needs arguments")
	}
}

func TestAffixInChains(t *testing.T) {
	runConvCases(t, []convCase{
		{name: "lookup then affix", spec: `[{"lookup": {"pairs": [[60, "h"]]}}, {"affix": {"prefix": "every_"}}]`, in: `60`, want: `"every_h"`},
		{name: "absent then affix leaves absence alone", spec: `[{"absent": {"values": ["inf"]}}, {"affix": {"prefix": "n"}}]`, in: `"inf"`, want: "", back: absentMark},
	})
}

// FuzzAffix checks the round-trip property for arbitrary strings and
// affixes: reverse(forward(v)) is v, and reverse never panics on anything.
func FuzzAffix(f *testing.F) {
	f.Add("n", "", "1")
	f.Add("", "_x", "")
	f.Add("aa", "a", "a")
	f.Add("→", "←", "é")
	f.Fuzz(func(t *testing.T, prefix, suffix, value string) {
		c, err := newAffix(map[string]any{"prefix": prefix, "suffix": suffix})
		if err != nil {
			if prefix == "" && suffix == "" {
				return
			}
			t.Fatalf("newAffix(%q, %q): %v", prefix, suffix, err)
		}
		wrapped, present, err := c.Forward(value, true)
		if err != nil || !present {
			t.Fatalf("Forward(%q) = %v, %v", value, present, err)
		}
		back, present, err := c.Reverse(wrapped, true)
		if err != nil || !present || back != value {
			t.Fatalf("Reverse(Forward(%q)) = %v, %v, %v with prefix %q suffix %q", value, back, present, err, prefix, suffix)
		}
		_, _, _ = c.Reverse(value, true)
	})
}
