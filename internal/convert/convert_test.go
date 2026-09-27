package convert

import (
	"encoding/json"
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

func mustConv(t *testing.T, spec string) Converter {
	t.Helper()
	c, err := Parse(json.RawMessage(spec))
	if err != nil {
		t.Fatalf("Parse(%s): %v", spec, err)
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
			out, outPresent, err := c.Forward(in, present)
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
			back, backPresent, err := c.Reverse(out, outPresent)
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
		v, present, err := c.Reverse(mustDecode(t, in), true)
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
			_, err := Parse(json.RawMessage(tc.spec))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	if _, err := Parse(json.RawMessage(`{`)); err == nil {
		t.Fatal("invalid JSON should fail")
	}
}

func TestConverterNames(t *testing.T) {
	names := Names()
	want := "absent, datetime, identity, linear, lookup, number, string"
	if got := strings.Join(names, ", "); got != want {
		t.Fatalf("converterNames = %q, want %q", got, want)
	}
	_, err := Parse(json.RawMessage(`"missing"`))
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("unknown converter error should list the names: %v", err)
	}
}

func TestArgumentTypeErrors(t *testing.T) {
	tests := []struct{ spec, want string }{
		{`{"datetime": {"from": 5, "to": "RFC3339"}}`, `argument "from" must be a string`},
		{`{"datetime": {"from": "DateOnly", "to": 5}}`, `argument "to" must be a string`},
		{`{"datetime": {"from": "DateOnly", "to": "RFC3339", "zone": 1}}`, `argument "zone" must be a string`},
		{`{"datetime": {"from": "DateOnly", "to": "RFC3339", "tz": "UTC"}}`, `unknown argument "tz"`},
		{`{"linear": {"divisor": "x"}}`, `argument "divisor" must be a number`},
		{`{"linear": {"offset": true}}`, `argument "offset" must be a number`},
		{`{"linear": {"round_reverse": -2}}`, "must not be negative"},
		{`{"lookup": {"pairs": [[1, 2]], "nope": 1}}`, `unknown argument "nope"`},
		{`{"absent": {"values": [1], "ignore_case": "yes"}}`, `argument "ignore_case" must be a boolean`},
		{`{"absent": {"values": [1], "extra": 1}}`, `unknown argument "extra"`},
		{`{"number": {"x": 1}}`, `unknown argument "x"`},
		{`{"string": {"x": 1}}`, `unknown argument "x"`},
		{`[{"number": {}}, {"linear": {"factor": "2"}}]`, "convert step 1: converter \"linear\": argument \"factor\" must be a number"},
	}
	for _, tc := range tests {
		t.Run(tc.spec, func(t *testing.T) {
			_, err := Parse(json.RawMessage(tc.spec))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	// null arguments mean "no arguments".
	if _, err := Parse(json.RawMessage(`{"identity": null}`)); err != nil {
		t.Errorf("{\"identity\": null} should parse: %v", err)
	}
}

func TestAbsentInputPassesThroughEveryConverter(t *testing.T) {
	for _, spec := range []string{`"identity"`, `"number"`, `"string"`, `{"linear": {"factor": 2}}`, `{"lookup": {"pairs": [[1, 2]]}}`, `{"datetime": {"from": "DateOnly", "to": "RFC3339"}}`, `{"absent": {"values": [1]}}`, `["number", {"linear": {"factor": 2}}]`} {
		c := mustConv(t, spec)
		for name, dir := range map[string]func(any, bool) (any, bool, error){"forward": c.Forward, "reverse": c.Reverse} {
			v, present, err := dir(nil, false)
			if err != nil || present || v != nil {
				t.Errorf("%s %s of absent = %v, %v, %v", spec, name, v, present, err)
			}
		}
	}
}

func TestChainPropagatesErrors(t *testing.T) {
	c := mustConv(t, `["number", {"datetime": {"from": "DateOnly", "to": "RFC3339"}}]`)
	if _, _, err := c.Forward(mustDecode(t, `5`), true); err == nil || !strings.Contains(err.Error(), "datetime: expected a string") {
		t.Errorf("forward chain error = %v", err)
	}
	c = mustConv(t, `[{"linear": {"divisor": 1000}}, "string"]`)
	if _, _, err := c.Reverse("abc", true); err == nil || !strings.Contains(err.Error(), "linear:") {
		t.Errorf("reverse chain error = %v", err)
	}
	// Three steps run in order forward and backwards in reverse.
	c = mustConv(t, `[{"absent": {"values": ["inf"]}}, {"linear": {"factor": 10}}, "string"]`)
	v, present, err := c.Forward(mustDecode(t, `"2.5"`), true)
	if err != nil || !present || v != "25" {
		t.Errorf("three-step forward = %v, %v, %v", v, present, err)
	}
	back, present, err := c.Reverse(v, present)
	if err != nil || !present || string(back.(json.Number)) != "2.5" {
		t.Errorf("three-step reverse = %v, %v, %v", back, present, err)
	}
}

func TestStringReverseEdgeCases(t *testing.T) {
	c := mustConv(t, `"string"`)
	for in, want := range map[string]string{`"false"`: `false`, `"Inf"`: `"Inf"`, `"NaN"`: `"NaN"`, `"1e3"`: `1000`, `"007"`: `7`, `""`: `""`, `"1e400"`: `"1e400"`} {
		v, present, err := c.Reverse(mustDecode(t, in), true)
		if err != nil {
			t.Fatal(err)
		}
		checkValue(t, "reverse "+in, v, present, want)
	}
	if _, _, err := c.Forward([]any{}, true); err == nil {
		t.Error("an array has no string form")
	}
}

func TestNumberKeepsExponentText(t *testing.T) {
	// A JSON number is kept as written, exponent notation included; only
	// strings are normalised.
	c := mustConv(t, `"number"`)
	v, _, err := c.Forward(mustDecode(t, `1e3`), true)
	if err != nil || string(v.(json.Number)) != "1e3" {
		t.Errorf("Forward(1e3) = %v, %v", v, err)
	}
	v, _, err = c.Forward("1e3", true)
	if err != nil || string(v.(json.Number)) != "1000" {
		t.Errorf("Forward(\"1e3\") = %v, %v", v, err)
	}
}

func TestLinearEdgeCases(t *testing.T) {
	runConvCases(t, []convCase{
		{name: "offset and rounding both ways", spec: `{"linear": {"factor": 3, "offset": 1, "round_forward": 2, "round_reverse": 3}}`, in: `0.3333`, want: `2`, back: `0.333`},
		{name: "negative factor", spec: `{"linear": {"factor": -1}}`, in: `5`, want: `-5`},
		{name: "spaces in numeric strings", spec: `{"linear": {"factor": 2}}`, in: `" 4.5 "`, want: `9`, back: `4.5`},
		{name: "huge values render as digits", spec: `{"linear": {"factor": 1000}}`, in: `1e300`, want: "1" + strings.Repeat("0", 303), back: "1" + strings.Repeat("0", 300)},
	})
	c := mustConv(t, `{"linear": {"factor": 1e300}}`)
	if _, _, err := c.Forward(mustDecode(t, `1e300`), true); err == nil {
		t.Error("a result that overflows to infinity must fail")
	}
}

func TestLookupEdgeCases(t *testing.T) {
	runConvCases(t, []convCase{
		{name: "null pairs with a value", spec: `{"lookup": {"pairs": [[null, "none"], [true, "yes"]]}}`, in: `null`, want: `"none"`},
		{name: "nested pairs", spec: `{"lookup": {"pairs": [[[1, 2], {"a": 1}]]}}`, in: `[1, 2]`, want: `{"a":1}`, back: `[1,2]`},
		{name: "lenient reverse passes through", spec: `{"lookup": {"pairs": [[60, "1H"]], "strict": false}}`, in: `"unknown"`, want: `"unknown"`},
	})
	c := mustConv(t, `{"lookup": {"pairs": [[60, "1H"]]}}`)
	if _, _, err := c.Reverse("2H", true); err == nil || !strings.Contains(err.Error(), `no pair for string "2H"`) {
		t.Errorf("strict reverse miss = %v", err)
	}
	// The table's values are copied out, never aliased.
	v, _, _ := c.Reverse("1H", true)
	if _, ok := v.(json.Number); !ok {
		t.Errorf("reverse should yield the left value as decoded, got %T", v)
	}
}

func TestDatetimeEdgeCases(t *testing.T) {
	runConvCases(t, []convCase{
		{name: "rendered in the zone", spec: `{"datetime": {"from": "RFC3339", "to": "DateTime", "zone": "Europe/Berlin"}}`, in: `"2018-06-01T10:00:00Z"`, want: `"2018-06-01 12:00:00"`, back: `"2018-06-01T12:00:00+02:00"`},
		{name: "fractional seconds are lost", spec: `{"datetime": {"from": "RFC3339Nano", "to": "RFC3339"}}`, in: `"2018-06-01T10:00:00.123456789Z"`, want: `"2018-06-01T10:00:00Z"`, back: `"2018-06-01T10:00:00Z"`},
		{name: "time only", spec: `{"datetime": {"from": "TimeOnly", "to": "15h04"}}`, in: `"10:30:00"`, want: `"10h30"`, back: `"10:30:00"`},
		{name: "custom layout", spec: `{"datetime": {"from": "02.01.2006", "to": "DateOnly"}}`, in: `"31.12.2018"`, want: `"2018-12-31"`},
	})
	c := mustConv(t, `{"datetime": {"from": "DateOnly", "to": "RFC3339"}}`)
	if _, _, err := c.Reverse("2018-13-45T00:00:00Z", true); err == nil {
		t.Error("an impossible date must fail")
	}
}

func TestAbsentEdgeCases(t *testing.T) {
	runConvCases(t, []convCase{
		{name: "ignore_case only affects strings", spec: `{"absent": {"values": [1, "a"], "ignore_case": true}}`, in: `"A"`, want: "", back: absentMark},
		{name: "objects compare structurally", spec: `{"absent": {"values": [{"x": 1}]}}`, in: `{"x": 1.0}`, want: "", back: absentMark},
		{name: "false is not absent unless listed", spec: `{"absent": {"values": [null]}}`, in: `false`, want: `false`},
	})
}

// FuzzParse checks that no spec can panic the converter factories and that
// a parsed converter handles a set of representative inputs without
// panicking in either direction.
func FuzzParse(f *testing.F) {
	for _, seed := range []string{`"number"`, `{"linear": {"divisor": 1000}}`, `[{"absent": {"values": ["inf"]}}, {"linear": {"factor": 2}}]`, `{"lookup": {"pairs": [[1, "a"]]}}`, `{"datetime": {"from": "DateOnly", "to": "RFC3339"}}`, `{"string": null}`, `5`, `[]`, `{"nope": {}}`} {
		f.Add(seed)
	}
	inputs := []any{nil, json.Number("1"), json.Number("-2.5"), "1e3", "inf", "2018-01-01", true, []any{}, map[string]any{}}
	f.Fuzz(func(t *testing.T, spec string) {
		c, err := Parse(json.RawMessage(spec))
		if err != nil {
			return
		}
		for _, in := range inputs {
			v, present, err := c.Forward(in, true)
			if err == nil && present {
				_, _, _ = c.Reverse(v, true)
			}
			_, _, _ = c.Reverse(in, true)
			_, _, _ = c.Forward(nil, false)
		}
	})
}
