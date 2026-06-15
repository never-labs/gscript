package bind

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func jsonInterp(t *testing.T, src string) *Interpreter {
	t.Helper()
	return runWithLib(t, src, "json", BuildJSON())
}

func TestJSONEncodeScalars(t *testing.T) {
	interp := jsonInterp(t, `
		n := json.encode(nil)
		t := json.encode(true)
		f := json.encode(false)
		i := json.encode(42)
		x := json.encode(3.14)
		s := json.encode("hello world")
	`)
	cases := map[string]string{
		"n": "null",
		"t": "true",
		"f": "false",
		"i": "42",
		"x": "3.14",
		"s": `"hello world"`,
	}
	for name, want := range cases {
		if got := interp.GetGlobal(name); !got.IsString() || got.Str() != want {
			t.Fatalf("%s = %v, want %q", name, got, want)
		}
	}
}

func TestJSONEncodeArrayObjectAndNestedValues(t *testing.T) {
	interp := jsonInterp(t, `
		array := json.encode([1, 2, 3])
		object := json.encode({name: "test", age: 30})
		nested := json.encode({items: [1, 2, 3], meta: {count: 3}})
		mixed := json.encode({10, 20, name: "test"})
	`)
	if got := interp.GetGlobal("array").Str(); got != "[1,2,3]" {
		t.Fatalf("array JSON = %q, want [1,2,3]", got)
	}
	object := interp.GetGlobal("object").Str()
	if !strings.Contains(object, `"name":"test"`) || !strings.Contains(object, `"age":30`) {
		t.Fatalf("object JSON missing fields: %s", object)
	}
	nested := interp.GetGlobal("nested").Str()
	if !strings.Contains(nested, `"items":[1,2,3]`) || !strings.Contains(nested, `"count":3`) {
		t.Fatalf("nested JSON missing fields: %s", nested)
	}
	mixed := interp.GetGlobal("mixed").Str()
	if !strings.Contains(mixed, `"1":10`) || !strings.Contains(mixed, `"2":20`) || !strings.Contains(mixed, `"name":"test"`) {
		t.Fatalf("mixed table should encode as object with numeric keys stringified: %s", mixed)
	}
}

func TestJSONDecodeScalars(t *testing.T) {
	interp := jsonInterp(t, `
		n := json.decode("null")
		t := json.decode("true")
		f := json.decode("false")
		i := json.decode("42")
		x := json.decode("3.14")
		s := json.decode("\"hello\"")
	`)
	if !interp.GetGlobal("n").IsNil() {
		t.Fatalf("null decode = %v, want nil", interp.GetGlobal("n"))
	}
	if got := interp.GetGlobal("t"); !got.IsBool() || !got.Bool() {
		t.Fatalf("true decode = %v", got)
	}
	if got := interp.GetGlobal("f"); !got.IsBool() || got.Bool() {
		t.Fatalf("false decode = %v", got)
	}
	if got := interp.GetGlobal("i"); !got.IsInt() || got.Int() != 42 {
		t.Fatalf("int decode = %v (%s), want int 42", got, got.TypeName())
	}
	if got := interp.GetGlobal("x"); !got.IsFloat() || got.Float() != 3.14 {
		t.Fatalf("float decode = %v, want 3.14", got)
	}
	if got := interp.GetGlobal("s"); !got.IsString() || got.Str() != "hello" {
		t.Fatalf("string decode = %v, want hello", got)
	}
}

func TestJSONDecodeArrayObjectAndNestedValues(t *testing.T) {
	interp := jsonInterp(t, `
		array := json.decode("[1, 2, 3]")
		object := json.decode("{\"name\":\"test\",\"age\":30}")
		nested := json.decode("{\"inner\":{\"value\":42}}")
	`)
	array := interp.GetGlobal("array")
	if !array.IsTable() {
		t.Fatalf("array decode type = %s, want table", array.TypeName())
	}
	if got := array.Table().Length(); got != 3 {
		t.Fatalf("array length = %d, want 3", got)
	}
	for i := int64(1); i <= 3; i++ {
		if got := array.Table().RawGet(IntValue(i)); !got.IsInt() || got.Int() != i {
			t.Fatalf("array[%d] = %v, want %d", i, got, i)
		}
	}

	object := interp.GetGlobal("object").Table()
	if got := object.RawGet(StringValue("name")); !got.IsString() || got.Str() != "test" {
		t.Fatalf("object.name = %v, want test", got)
	}
	if got := object.RawGet(StringValue("age")); !got.IsInt() || got.Int() != 30 {
		t.Fatalf("object.age = %v, want 30", got)
	}

	inner := interp.GetGlobal("nested").Table().RawGet(StringValue("inner"))
	if !inner.IsTable() {
		t.Fatalf("nested.inner type = %s, want table", inner.TypeName())
	}
	if got := inner.Table().RawGet(StringValue("value")); !got.IsInt() || got.Int() != 42 {
		t.Fatalf("nested.inner.value = %v, want 42", got)
	}
}

func TestJSONDecodeErrorsAndTrailingData(t *testing.T) {
	interp := jsonInterp(t, `
		badResult, badErr := json.decode("invalid json{{{")
		trailResult, trailErr := json.decode("{} []")
	`)
	if got := interp.GetGlobal("badResult"); !got.IsNil() {
		t.Fatalf("bad result = %v, want nil", got)
	}
	if got := interp.GetGlobal("badErr"); !got.IsString() || got.Str() == "" {
		t.Fatalf("bad err = %v, want message", got)
	}
	if got := interp.GetGlobal("trailResult"); !got.IsNil() {
		t.Fatalf("trailing result = %v, want nil", got)
	}
	if got := interp.GetGlobal("trailErr"); !got.IsString() || !strings.Contains(got.Str(), "trailing data") {
		t.Fatalf("trailing err = %v, want trailing data message", got)
	}
}

func TestJSONValid(t *testing.T) {
	interp := jsonInterp(t, `
		object := json.valid("{\"name\":\"test\",\"age\":30}")
		array := json.valid("[1, true, null]")
		stringValue := json.valid("\"hello\"")
		malformed := json.valid("{")
		trailing := json.valid("{} []")
		empty := json.valid("")
	`)
	for _, name := range []string{"object", "array", "stringValue"} {
		if got := interp.GetGlobal(name); !got.IsBool() || !got.Bool() {
			t.Fatalf("%s valid = %v, want true", name, got)
		}
	}
	for _, name := range []string{"malformed", "trailing", "empty"} {
		if got := interp.GetGlobal(name); !got.IsBool() || got.Bool() {
			t.Fatalf("%s valid = %v, want false", name, got)
		}
	}
}

func TestJSONPrettyAndIndent(t *testing.T) {
	interp := jsonInterp(t, `
		prettyDefault := json.pretty({a: 1})
		prettyCustom := json.pretty({a: 1}, "    ")
		indentDefault := json.indent({a: 1})
		indentCustom := json.indent({a: 1}, "\t")
	`)
	for _, name := range []string{"prettyDefault", "indentDefault"} {
		got := interp.GetGlobal(name).Str()
		if !strings.Contains(got, "\n") || !strings.Contains(got, "  ") {
			t.Fatalf("%s = %q, want newline and two-space indent", name, got)
		}
	}
	if got := interp.GetGlobal("prettyCustom").Str(); !strings.Contains(got, "    ") {
		t.Fatalf("prettyCustom = %q, want four-space indent", got)
	}
	if got := interp.GetGlobal("indentCustom").Str(); !strings.Contains(got, "\t") {
		t.Fatalf("indentCustom = %q, want tab indent", got)
	}
}

func TestJSONRoundtrip(t *testing.T) {
	interp := jsonInterp(t, `
		original := {name: "test", scores: {90, 85, 95}, active: true}
		encoded := json.encode(original)
		decoded := json.decode(encoded)
		name := decoded.name
		active := decoded.active
		score1 := decoded.scores[1]
	`)
	if got := interp.GetGlobal("name"); got.Str() != "test" {
		t.Fatalf("name = %v, want test", got)
	}
	if got := interp.GetGlobal("active"); !got.Bool() {
		t.Fatalf("active = %v, want true", got)
	}
	if got := interp.GetGlobal("score1"); got.Int() != 90 {
		t.Fatalf("score1 = %v, want 90", got)
	}
}

func TestJSONMappingBoundaries(t *testing.T) {
	t.Run("empty table encodes as object", func(t *testing.T) {
		if got := jsonInterp(t, `result := json.encode({})`).GetGlobal("result").Str(); got != "{}" {
			t.Fatalf("empty table JSON = %q, want {}", got)
		}
	})
	t.Run("sparse integer table encodes as object", func(t *testing.T) {
		interp := jsonInterp(t, `
			t := {}
			t[2] = "two"
			result := json.encode(t)
		`)
		if got := interp.GetGlobal("result").Str(); !strings.Contains(got, `"2":"two"`) {
			t.Fatalf("sparse table JSON = %q, want object with key 2", got)
		}
	})
	t.Run("non-finite floats encode as null", func(t *testing.T) {
		for name, value := range map[string]Value{
			"pos": FloatValue(math.Inf(1)),
			"neg": FloatValue(math.Inf(-1)),
			"nan": FloatValue(math.NaN()),
		} {
			if got := runtime.JSONValueToGo(value); got != nil {
				t.Fatalf("%s maps to %#v, want nil", name, got)
			}
		}
	})
	t.Run("safe boxed JSON integer remains int", func(t *testing.T) {
		got := runtime.JSONGoToValue(json.Number("140737488355327"))
		if !got.IsInt() || got.Int() != 140737488355327 {
			t.Fatalf("decoded max boxed int = %v (%s), want int", got, got.TypeName())
		}
	})
	t.Run("integer outside boxed range falls back to float", func(t *testing.T) {
		got := runtime.JSONGoToValue(json.Number("140737488355328"))
		if !got.IsFloat() {
			t.Fatalf("decoded out-of-range integer = %v (%s), want float", got, got.TypeName())
		}
	})
}
