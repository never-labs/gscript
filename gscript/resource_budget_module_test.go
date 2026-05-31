package gscript_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

func TestWithMaxModuleBytesLimitsInterpreterRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "big.gs"), []byte(`return "12345"`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithRequirePath(dir), gs.WithMaxModuleBytes(4))
	err := vm.Exec(`require("big")`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "module_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("expected module_bytes budget 4, got %T %v", err, err)
	}
}

func TestWithMaxModuleDepthLimitsInterpreterNestedRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.gs"), []byte(`return require("b")`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.gs"), []byte(`return { ok: true }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithRequirePath(dir), gs.WithMaxModuleDepth(1))
	err := vm.Exec(`require("a")`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "module_depth" || budgetErr.Limit != 1 {
		t.Fatalf("expected module_depth budget 1, got %T %v", err, err)
	}
}

func TestWithMaxModuleBytesLimitsBytecodeRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "big.gs"), []byte(`return "12345"`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithVM(), gs.WithRequirePath(dir), gs.WithMaxModuleBytes(4))
	err := vm.Exec(`require("big")`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "module_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("expected module_bytes budget 4, got %T %v", err, err)
	}
}

func TestWithMaxModuleDepthLimitsBytecodeNestedRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.gs"), []byte(`return require("b")`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.gs"), []byte(`return { ok: true }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithVM(), gs.WithRequirePath(dir), gs.WithMaxModuleDepth(1))
	err := vm.Exec(`require("a")`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "module_depth" || budgetErr.Limit != 1 {
		t.Fatalf("expected module_depth budget 1, got %T %v", err, err)
	}
}
