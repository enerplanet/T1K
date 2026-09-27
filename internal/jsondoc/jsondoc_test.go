package jsondoc

import (
	"encoding/json"
	"testing"
)

func decode(t *testing.T, s string) any {
	t.Helper()
	v, err := Decode([]byte(s))
	if err != nil {
		t.Fatalf("Decode(%s): %v", s, err)
	}
	return v
}

func TestDecode(t *testing.T) {
	if _, err := Decode([]byte(`{"a": 1} trailing`)); err == nil {
		t.Error("trailing data should fail")
	}
	if _, err := Decode([]byte(`{`)); err == nil {
		t.Error("truncated JSON should fail")
	}
	v := decode(t, `{"n": 12345678901234567890, "f": 1.0}`)
	obj := v.(map[string]any)
	if obj["n"].(json.Number) != "12345678901234567890" || obj["f"].(json.Number) != "1.0" {
		t.Errorf("numbers lost their digits: %v", obj)
	}
}

func TestEncode(t *testing.T) {
	out, err := Encode(map[string]any{"u": "a<b>&c", "n": json.Number("1.0")}, "", "")
	if err != nil || string(out) != `{"n":1.0,"u":"a<b>&c"}` {
		t.Errorf("Encode = %s, %v", out, err)
	}
	out, err = Encode([]any{json.Number("1")}, "", "  ")
	if err != nil || string(out) != "[\n  1\n]" {
		t.Errorf("indented Encode = %q, %v", out, err)
	}
}

func TestDeepCopy(t *testing.T) {
	in := decode(t, `{"a": {"list": [1, {"b": null}]}}`)
	out := DeepCopy(in)
	if !Equal(in, out) {
		t.Fatal("the copy must equal the original")
	}
	out.(map[string]any)["a"].(map[string]any)["list"].([]any)[0] = "changed"
	if Equal(in, out) {
		t.Error("the copy shares memory with the original")
	}
}

func TestEqual(t *testing.T) {
	a := decode(t, `{"x": [1, 2.0, "s", true, null, {"y": 60}]}`)
	b := decode(t, `{"x": [1.0, 2, "s", true, null, {"y": 60.0}]}`)
	if !Equal(a, b) {
		t.Error("numerically equal documents should compare equal")
	}
	for _, pair := range [][2]string{{`"60"`, `60`}, {`[1]`, `[1, 2]`}, {`{"a":1}`, `{"b":1}`}, {`{"a":1}`, `{"a":1,"b":2}`}, {`null`, `false`}, {`true`, `false`}, {`"a"`, `"b"`}} {
		if Equal(decode(t, pair[0]), decode(t, pair[1])) {
			t.Errorf("%s and %s compared equal", pair[0], pair[1])
		}
	}
	if !Equal(60, json.Number("60")) || !Equal(int64(2), float32(2)) {
		t.Error("Go numbers must compare with JSON numbers")
	}
}

func TestNumbers(t *testing.T) {
	if f, ok := Number(json.Number("2.5")); !ok || f != 2.5 {
		t.Errorf("Number = %v, %v", f, ok)
	}
	if _, ok := Number("2.5"); ok {
		t.Error("a string is not a Number")
	}
	if f, ok := Float(" 2.5 "); !ok || f != 2.5 {
		t.Errorf("Float = %v, %v", f, ok)
	}
	for _, bad := range []any{"inf", "abc", true, nil, []any{}} {
		if _, ok := Float(bad); ok {
			t.Errorf("Float(%v) should fail", bad)
		}
	}
	for in, want := range map[float64]string{1e-7: "0.0000001", 8889.3 / 1000: "8.8893", 1007.93 * 1000: "1007930", 3: "3", -2.5: "-2.5"} {
		got, err := FormatNumber(in)
		if err != nil || string(got) != want {
			t.Errorf("FormatNumber(%v) = %s, %v; want %s", in, got, err, want)
		}
	}
	if _, err := FormatNumber(1 / zero()); err == nil {
		t.Error("an infinite value should fail")
	}
}

func zero() float64 { return 0 }

func TestDescribe(t *testing.T) {
	if s, ok := ScalarString(json.Number("7")); !ok || s != "7" {
		t.Errorf("ScalarString = %q, %v", s, ok)
	}
	if s, ok := ScalarString(true); !ok || s != "true" {
		t.Errorf("ScalarString = %q, %v", s, ok)
	}
	if _, ok := ScalarString(nil); ok {
		t.Error("null has no string form")
	}
	for v, want := range map[any]string{json.Number("1"): "number", nil: "null", "s": "string", true: "boolean"} {
		if got := TypeName(v); got != want {
			t.Errorf("TypeName(%v) = %q, want %q", v, got, want)
		}
	}
	if TypeName([]any{}) != "array" || TypeName(map[string]any{}) != "object" {
		t.Error("container type names")
	}
	if Describe("x") != `"x"` || Describe(json.Number("5")) != "5" || Describe([]any{}) != "array" {
		t.Error("Describe renderings")
	}
}
