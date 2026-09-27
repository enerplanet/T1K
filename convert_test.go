package t1k

import (
	"encoding/json"
	"strings"
	"testing"
)

func mustConv(t *testing.T, spec string) converter {
	t.Helper()
	c, err := parseConvert(json.RawMessage(spec))
	if err != nil {
		t.Fatalf("parseConvert(%s): %v", spec, err)
	}
	return c
}

// convCase drives a converter forward and back and compares JSON renderings.
type convCase struct {
	name    string
	spec    string
	in      string // JSON literal, "" for absent
	want    string // JSON literal, "" for absent
	back    string // expected result of reverse(want); "" means "same as in", absentMark means absent
	wantErr string
}

const absentMark = "<absent>"

func runConvCases(t *testing.T, cases []convCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := mustConv(t, tc.spec)
			var in any
			present := tc.in != ""
			if present {
				in = mustDecode(t, tc.in)
			}
			out, outPresent, err := c.forward(in, present)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("forward error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("forward: %v", err)
			}
			checkValue(t, "forward", out, outPresent, tc.want)
			back, backPresent, err := c.reverse(out, outPresent)
			if err != nil {
				t.Fatalf("reverse: %v", err)
			}
			wantBack := tc.back
			switch wantBack {
			case "":
				wantBack = tc.in
			case absentMark:
				wantBack = ""
			}
			checkValue(t, "reverse", back, backPresent, wantBack)
		})
	}
}

func checkValue(t *testing.T, dir string, v any, present bool, want string) {
	t.Helper()
	if want == "" {
		if present {
			t.Fatalf("%s: got %v, want absent", dir, v)
		}
		return
	}
	if !present {
		t.Fatalf("%s: got absent, want %s", dir, want)
	}
	got, _ := json.Marshal(v)
	if string(got) != want {
		t.Fatalf("%s: got %s, want %s", dir, got, want)
	}
}

func TestConverters(t *testing.T) {
	runConvCases(t, []convCase{
		{name: "identity", spec: `"identity"`, in: `{"a":[1]}`, want: `{"a":[1]}`},
		{name: "identity absent", spec: `"identity"`, in: "", want: ""},
		{name: "number keeps digits", spec: `"number"`, in: `1.0`, want: `1.0`},
		{name: "number parses string", spec: `"number"`, in: `" 8 "`, want: `8`, back: `8`},
		{name: "number rejects bool", spec: `"number"`, in: `true`, wantErr: "not numeric"},
		{name: "number absent", spec: `"number"`, in: "", want: ""},
		{name: "string from number", spec: `"string"`, in: `8.5`, want: `"8.5"`},
		{name: "string from bool", spec: `"string"`, in: `true`, want: `"true"`},
		{name: "string keeps text", spec: `"string"`, in: `"abc"`, want: `"abc"`},
		{name: "string rejects null", spec: `"string"`, in: `null`, wantErr: "cannot render"},
		{name: "linear divisor", spec: `{"linear": {"divisor": 1000}}`, in: `8715`, want: `8.715`},
		{name: "linear factor", spec: `{"linear": {"factor": 1000}}`, in: `0.2359`, want: `235.9`},
		{name: "linear noise", spec: `{"linear": {"divisor": 1000}}`, in: `8889.3`, want: `8.8893`},
		{name: "linear offset", spec: `{"linear": {"offset": -180}}`, in: `180`, want: `0`},
		{name: "linear string input", spec: `{"linear": {"factor": 2}}`, in: `"4"`, want: `8`, back: `4`},
		{name: "linear rounding", spec: `{"linear": {"divisor": 8760000, "round_forward": 9, "round_reverse": 0}}`, in: `3745`, want: `0.000427511`},
		{name: "linear rejects text", spec: `{"linear": {"factor": 2}}`, in: `"inf"`, wantErr: "not numeric"},
		{name: "linear absent", spec: `{"linear": {"factor": 2}}`, in: "", want: ""},
		{name: "lookup forward", spec: `{"lookup": {"pairs": [[15, "15min"], [60, "1H"]]}}`, in: `60`, want: `"1H"`},
		{name: "lookup numeric equality", spec: `{"lookup": {"pairs": [[60, "1H"]]}}`, in: `60.0`, want: `"1H"`, back: `60`},
		{name: "lookup strict miss", spec: `{"lookup": {"pairs": [[60, "1H"]]}}`, in: `45`, wantErr: "no pair for number 45"},
		{name: "lookup lenient miss", spec: `{"lookup": {"pairs": [[60, "1H"]], "strict": false}}`, in: `45`, want: `45`},
		{name: "lookup objects", spec: `{"lookup": {"pairs": [[{"a":1}, ["x"]]]}}`, in: `{"a":1}`, want: `["x"]`},
		{name: "datetime", spec: `{"datetime": {"from": "2006-01-02T15:04:05.000Z", "to": "RFC3339"}}`, in: `"2018-01-01T00:00:00.000Z"`, want: `"2018-01-01T00:00:00Z"`},
		{name: "datetime zone", spec: `{"datetime": {"from": "DateOnly", "to": "RFC3339", "zone": "Europe/Berlin"}}`, in: `"2018-06-01"`, want: `"2018-06-01T00:00:00+02:00"`},
		{name: "datetime bad input", spec: `{"datetime": {"from": "DateOnly", "to": "RFC3339"}}`, in: `"yesterday"`, wantErr: "datetime:"},
		{name: "datetime non-string", spec: `{"datetime": {"from": "DateOnly", "to": "RFC3339"}}`, in: `5`, wantErr: "expected a string"},
		{name: "absent string", spec: `{"absent": {"values": ["inf"]}}`, in: `"inf"`, want: "", back: absentMark},
		{name: "absent ignore case", spec: `{"absent": {"values": ["inf"], "ignore_case": true}}`, in: `"INF"`, want: "", back: absentMark},
		{name: "absent case sensitive", spec: `{"absent": {"values": ["inf"]}}`, in: `"INF"`, want: `"INF"`},
		{name: "absent number", spec: `{"absent": {"values": [0]}}`, in: `0.0`, want: "", back: absentMark},
		{name: "absent null", spec: `{"absent": {"values": [null]}}`, in: `null`, want: "", back: absentMark},
		{name: "absent keeps others", spec: `{"absent": {"values": [0]}}`, in: `5`, want: `5`},
		{name: "chain", spec: `[{"absent": {"values": ["inf"]}}, {"linear": {"divisor": 1000}}]`, in: `"8"`, want: `0.008`, back: `8`},
		{name: "chain absent", spec: `[{"absent": {"values": ["inf"]}}, {"linear": {"divisor": 1000}}]`, in: `"inf"`, want: "", back: absentMark},
		{name: "chain single", spec: `["number"]`, in: `"3"`, want: `3`, back: `3`},
	})
}

func TestStringReverse(t *testing.T) {
	c := mustConv(t, `"string"`)
	for in, want := range map[string]string{`"8"`: `8`, `"true"`: `true`, `"x"`: `"x"`, `7`: `7`} {
		v, present, err := c.reverse(mustDecode(t, in), true)
		if err != nil {
			t.Fatal(err)
		}
		checkValue(t, "reverse "+in, v, present, want)
	}
}

func TestConverterSpecErrors(t *testing.T) {
	tests := []struct{ spec, want string }{
		{`"nope"`, "unknown converter \"nope\""},
		{`5`, "a converter step is a name or a one-key object"},
		{`[]`, "empty list"},
		{`{"linear": {"divisor": 1000}, "number": {}}`, "one-key object"},
		{`{"linear": 5}`, "arguments must be an object"},
		{`{"linear": {"divisor": 0}}`, "must not be zero"},
		{`{"linear": {"factor": 0}}`, "must not be zero"},
		{`{"linear": {"factor": "2"}}`, "must be a number"},
		{`{"linear": {"round_forward": -1}}`, "must not be negative"},
		{`{"linear": {"round_forward": 1.5}}`, "must be an integer"},
		{`{"linear": {"faktor": 2}}`, "unknown argument \"faktor\""},
		{`{"identity": {"x": 1}}`, "unknown argument"},
		{`{"lookup": {}}`, "\"pairs\" is required"},
		{`{"lookup": {"pairs": []}}`, "non-empty array"},
		{`{"lookup": {"pairs": [[1]]}}`, "must be a [left, right] pair"},
		{`{"lookup": {"pairs": [[1, "a"], [1, "b"]]}}`, "left value 1 appears twice"},
		{`{"lookup": {"pairs": [[1, "a"], [2, "a"]]}}`, "right value \"a\" appears twice"},
		{`{"lookup": {"pairs": [[1, "a"]], "strict": "yes"}}`, "must be a boolean"},
		{`{"datetime": {"from": "DateOnly"}}`, "\"from\" and \"to\""},
		{`{"datetime": {"from": "DateOnly", "to": "RFC3339", "zone": "Mars/Olympus"}}`, "zone"},
		{`{"absent": {}}`, "\"values\" is required"},
		{`{"absent": {"values": "inf"}}`, "non-empty array"},
		{`["number", 5]`, "convert step 1"},
	}
	for _, tc := range tests {
		t.Run(tc.spec, func(t *testing.T) {
			_, err := parseConvert(json.RawMessage(tc.spec))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	if _, err := parseConvert(json.RawMessage(`{`)); err == nil {
		t.Fatal("invalid JSON should fail")
	}
}

func TestConverterNames(t *testing.T) {
	names := converterNames()
	want := "absent, datetime, identity, linear, lookup, number, string"
	if got := strings.Join(names, ", "); got != want {
		t.Fatalf("converterNames = %q, want %q", got, want)
	}
	_, err := parseConvert(json.RawMessage(`"missing"`))
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("unknown converter error should list the names: %v", err)
	}
}

func TestNumberValue(t *testing.T) {
	for in, want := range map[float64]string{1e-7: "0.0000001", 8889.3 / 1000: "8.8893", 1007.93 * 1000: "1007930", 3: "3", -2.5: "-2.5"} {
		got, err := numberValue(in)
		if err != nil || string(got) != want {
			t.Errorf("numberValue(%v) = %s, %v; want %s", in, got, err, want)
		}
	}
	if _, err := numberValue(1 / zero()); err == nil {
		t.Error("infinite value should fail")
	}
}

func zero() float64 { return 0 }

func TestDecodeJSON(t *testing.T) {
	if _, err := decodeJSON([]byte(`{"a": 1} trailing`)); err == nil {
		t.Error("trailing data should fail")
	}
	if _, err := decodeJSON([]byte(`{`)); err == nil {
		t.Error("truncated JSON should fail")
	}
	v, err := decodeJSON([]byte(`{"n": 12345678901234567890}`))
	if err != nil {
		t.Fatal(err)
	}
	if n := v.(map[string]any)["n"].(json.Number); n != "12345678901234567890" {
		t.Errorf("number lost its digits: %s", n)
	}
	out, err := encodeJSON(map[string]any{"u": "a<b>&c"}, "", "")
	if err != nil || string(out) != `{"u":"a<b>&c"}` {
		t.Errorf("encodeJSON = %s, %v", out, err)
	}
}

func TestEqualJSON(t *testing.T) {
	a := mustDecode(t, `{"x": [1, 2.0, "s", true, null, {"y": 60}]}`)
	b := mustDecode(t, `{"x": [1.0, 2, "s", true, null, {"y": 60.0}]}`)
	if !equalJSON(a, b) {
		t.Error("numerically equal documents should compare equal")
	}
	if equalJSON(mustDecode(t, `"60"`), mustDecode(t, `60`)) {
		t.Error("a numeric string must not equal a number")
	}
	if equalJSON(mustDecode(t, `[1]`), mustDecode(t, `[1, 2]`)) || equalJSON(mustDecode(t, `{"a":1}`), mustDecode(t, `{"b":1}`)) {
		t.Error("different containers compared equal")
	}
	if typeName(json.Number("1")) != "number" || typeName(nil) != "null" || typeName([]any{}) != "array" {
		t.Error("typeName mismatch")
	}
}
