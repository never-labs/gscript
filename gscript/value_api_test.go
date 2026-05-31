package gscript_test

import (
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

func TestToValue_slice(t *testing.T) {
	vm := gs.New()
	err := vm.Set("arr", []int{10, 20, 30})
	if err != nil {
		t.Fatal(err)
	}
	// GScript: arr is a 1-based table
	err = vm.Exec(`result := arr[1] + arr[2] + arr[3]`)
	if err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if val != int64(60) {
		t.Fatalf("expected 60, got %v", val)
	}
}

func TestToValue_map(t *testing.T) {
	vm := gs.New()
	err := vm.Set("data", map[string]interface{}{
		"name": "test",
		"val":  42,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = vm.Exec(`name := data.name`)
	if err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("name")
	if err != nil {
		t.Fatal(err)
	}
	if val != "test" {
		t.Fatalf("expected 'test', got %v", val)
	}
}

func TestToValue_func(t *testing.T) {
	vm := gs.New()
	err := vm.Set("greet", func(name string) string {
		return "Hello, " + name + "!"
	})
	if err != nil {
		t.Fatal(err)
	}
	results, err := vm.Call("greet", "world")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0] != "Hello, world!" {
		t.Fatalf("expected 'Hello, world!', got %v", results)
	}
}
