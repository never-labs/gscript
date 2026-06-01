package leia_test

import (
	"fmt"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestRegisterFunc(t *testing.T) {
	vm := leia.New()
	err := vm.RegisterFunc("square", func(x float64) float64 {
		return x * x
	})
	if err != nil {
		t.Fatal(err)
	}
	results, err := vm.Call("square", 5.0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0] != float64(25) {
		t.Fatalf("expected 25.0, got %v (%T)", results[0], results[0])
	}
}

func TestRegisterFunc_multiReturn(t *testing.T) {
	vm := leia.New()
	err := vm.RegisterFunc("divmod", func(a, b int64) (int64, int64) {
		return a / b, a % b
	})
	if err != nil {
		t.Fatal(err)
	}
	results, err := vm.Call("divmod", 17, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0] != int64(3) || results[1] != int64(2) {
		t.Fatalf("expected [3 2], got %v", results)
	}
}

func TestRegisterFunc_error(t *testing.T) {
	vm := leia.New()
	err := vm.RegisterFunc("fail", func() error {
		return fmt.Errorf("something went wrong")
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = vm.Call("fail")
	if err == nil {
		t.Fatal("expected error from fail()")
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegisterFunc_panicReturnsError(t *testing.T) {
	vm := leia.New()
	err := vm.RegisterFunc("explode", func() int64 {
		panic("boom")
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = vm.Call("explode")
	if err == nil {
		t.Fatal("expected error from panic")
	}
	msg := err.Error()
	if !strings.Contains(msg, "explode") || !strings.Contains(msg, "boom") {
		t.Fatalf("panic error = %q, want function name and panic value", msg)
	}
}

func TestRegisterFunc_panicFromScriptReturnsError(t *testing.T) {
	vm := leia.New()
	err := vm.RegisterFunc("explodeFromScript", func() {
		panic("script boom")
	})
	if err != nil {
		t.Fatal(err)
	}

	err = vm.Exec(`explodeFromScript()`)
	if err == nil {
		t.Fatal("expected error from script-called panic")
	}
	msg := err.Error()
	if !strings.Contains(msg, "explodeFromScript") || !strings.Contains(msg, "script boom") {
		t.Fatalf("panic error = %q, want function name and panic value", msg)
	}
}

func TestRegisterFunc_fromScript(t *testing.T) {
	var output []string
	vm := leia.New(leia.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))
	vm.RegisterFunc("double", func(x int64) int64 { return x * 2 })
	err := vm.Exec(`print(double(21))`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 1 || output[0] != "42" {
		t.Fatalf("expected '42', got %v", output)
	}
}

func TestRegisterTable(t *testing.T) {
	vm := leia.New()
	err := vm.RegisterTable("mymath", map[string]interface{}{
		"add": func(a, b float64) float64 { return a + b },
		"mul": func(a, b float64) float64 { return a * b },
		"pi":  3.14159,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = vm.Exec(`result := mymath.add(mymath.pi, 1.0)`)
	if err != nil {
		t.Fatal(err)
	}

	val, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	expected := 3.14159 + 1.0
	if val != expected {
		t.Fatalf("expected %v, got %v", expected, val)
	}
}
