package gscript_test

import (
	"reflect"
	"strings"
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

func TestPublicSDKRecommendedAPISignaturesHideInternalRuntime(t *testing.T) {
	vmType := reflect.TypeOf((*gs.VM)(nil))
	for _, name := range []string{
		"Exec",
		"ExecContext",
		"ExecFile",
		"ExecFileContext",
		"Run",
		"RunContext",
		"Call",
		"CallContext",
		"CallValue",
		"CallValueContext",
		"Set",
		"Get",
		"RegisterFunc",
		"RegisterTable",
		"RegisterModule",
		"RegisterModuleFrom",
		"Reset",
		"SetArgs",
	} {
		method, ok := vmType.MethodByName(name)
		if !ok {
			t.Fatalf("VM.%s missing", name)
		}
		assertNoInternalRuntimeType(t, "VM."+name, method.Type)
	}

	for name, fn := range map[string]interface{}{
		"New":         gs.New,
		"Compile":     gs.Compile,
		"CompileFile": gs.CompileFile,
		"Decode":      gs.Decode,
		"Encode":      gs.Encode,
		"Nil":         gs.Nil,
		"Bool":        gs.Bool,
		"Int":         gs.Int,
		"Float":       gs.Float,
		"String":      gs.String,
	} {
		assertNoInternalRuntimeType(t, name, reflect.TypeOf(fn))
	}
}

func TestPublicValueBoundaryWorksWithoutRawRuntimeTypes(t *testing.T) {
	vm := gs.New(gs.WithVM())

	limit := gs.Int(40)
	if limit.Kind() != gs.KindInt || limit.Int() != 40 {
		t.Fatalf("limit = %s/%d, want int/40", limit.Kind(), limit.Int())
	}
	if err := vm.Set("limit", limit); err != nil {
		t.Fatal(err)
	}
	if err := vm.Exec(`
		config := {label: "answer", values: {limit, 2}}
		func add(a, b) { return a + b }
		result := add(config.values[1], config.values[2])
	`); err != nil {
		t.Fatal(err)
	}

	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(42) {
		t.Fatalf("result = %v (%T), want int64(42)", got, got)
	}

	encoded, err := gs.String("answer").Encode()
	if err != nil {
		t.Fatal(err)
	}
	if encoded != "answer" {
		t.Fatalf("encoded string = %v (%T), want answer", encoded, encoded)
	}

	decoded, err := gs.Decode(map[string]interface{}{
		"label": "answer",
		"count": 42,
	})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Kind() != gs.KindTable {
		t.Fatalf("decoded kind = %s, want table", decoded.Kind())
	}
	roundTrip, err := gs.Encode(decoded)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := roundTrip.(map[string]interface{})
	if !ok {
		t.Fatalf("roundTrip = %T, want map[string]interface{}", roundTrip)
	}
	if m["label"] != "answer" || m["count"] != int64(42) {
		t.Fatalf("roundTrip = %#v, want label/count fields", m)
	}
}

func TestPublicValueCanCallHostAndScriptBoundaries(t *testing.T) {
	vm := gs.New(gs.WithVM())
	if err := vm.RegisterFunc("scale", func(v gs.Value) int64 {
		if v.Kind() != gs.KindInt {
			t.Fatalf("scale arg kind = %s, want int", v.Kind())
		}
		return v.Int() * 2
	}); err != nil {
		t.Fatal(err)
	}
	if err := vm.Exec(`
		func apply(fn, value) {
			return fn(value)
		}
		result := apply(scale, 21)
	`); err != nil {
		t.Fatal(err)
	}
	results, err := vm.Call("apply", func(v gs.Value) int64 {
		if v.Kind() != gs.KindInt {
			t.Fatalf("apply arg kind = %s, want int", v.Kind())
		}
		return v.Int() + 1
	}, gs.Int(41))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0] != int64(42) {
		t.Fatalf("apply result = %#v, want [42]", results)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(42) {
		t.Fatalf("result = %v (%T), want int64(42)", got, got)
	}
}

func assertNoInternalRuntimeType(t *testing.T, name string, typ reflect.Type) {
	t.Helper()
	signature := typ.String()
	if strings.Contains(signature, "/internal/runtime.") || strings.Contains(signature, "runtime.Value") || strings.Contains(signature, "runtime.Interpreter") {
		t.Fatalf("%s exposes internal runtime in recommended API signature: %s", name, signature)
	}
}
