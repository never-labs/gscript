package leia_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestWithMaxModuleBytesLimitsInterpreterRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "big.leia"), []byte(`return "12345"`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := leia.New(leia.WithRequirePath(dir), leia.WithMaxModuleBytes(4))
	err := vm.Exec(`require("big")`)
	var budgetErr *leia.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "module_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("expected module_bytes budget 4, got %T %v", err, err)
	}
}

func TestWithMaxModuleDepthLimitsInterpreterNestedRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.leia"), []byte(`return require("b")`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.leia"), []byte(`return { ok: true }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := leia.New(leia.WithRequirePath(dir), leia.WithMaxModuleDepth(1))
	err := vm.Exec(`require("a")`)
	var budgetErr *leia.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "module_depth" || budgetErr.Limit != 1 {
		t.Fatalf("expected module_depth budget 1, got %T %v", err, err)
	}
}

func TestWithMaxModuleBytesLimitsBytecodeRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "big.leia"), []byte(`return "12345"`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := leia.New(leia.WithVM(), leia.WithRequirePath(dir), leia.WithMaxModuleBytes(4))
	err := vm.Exec(`require("big")`)
	var budgetErr *leia.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "module_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("expected module_bytes budget 4, got %T %v", err, err)
	}
}

func TestWithMaxModuleDepthLimitsBytecodeNestedRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.leia"), []byte(`return require("b")`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.leia"), []byte(`return { ok: true }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := leia.New(leia.WithVM(), leia.WithRequirePath(dir), leia.WithMaxModuleDepth(1))
	err := vm.Exec(`require("a")`)
	var budgetErr *leia.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "module_depth" || budgetErr.Limit != 1 {
		t.Fatalf("expected module_depth budget 1, got %T %v", err, err)
	}
}
