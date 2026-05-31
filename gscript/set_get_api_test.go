package gscript_test

import (
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

func TestSetGet(t *testing.T) {
	vm := gs.New()
	if err := vm.Set("x", 42); err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("x")
	if err != nil {
		t.Fatal(err)
	}
	if val != int64(42) {
		t.Fatalf("expected 42, got %v (%T)", val, val)
	}
}

func TestSetGet_string(t *testing.T) {
	vm := gs.New()
	if err := vm.Set("name", "gscript"); err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("name")
	if err != nil {
		t.Fatal(err)
	}
	if val != "gscript" {
		t.Fatalf("expected 'gscript', got %v", val)
	}
}

func TestSetGet_float(t *testing.T) {
	vm := gs.New()
	if err := vm.Set("pi", 3.14); err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("pi")
	if err != nil {
		t.Fatal(err)
	}
	if val != 3.14 {
		t.Fatalf("expected 3.14, got %v", val)
	}
}

func TestSetGet_bool(t *testing.T) {
	vm := gs.New()
	if err := vm.Set("flag", true); err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("flag")
	if err != nil {
		t.Fatal(err)
	}
	if val != true {
		t.Fatalf("expected true, got %v", val)
	}
}

func TestSetGet_nil(t *testing.T) {
	vm := gs.New()
	if err := vm.Set("nothing", nil); err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("nothing")
	if err != nil {
		t.Fatal(err)
	}
	if val != nil {
		t.Fatalf("expected nil, got %v", val)
	}
}
