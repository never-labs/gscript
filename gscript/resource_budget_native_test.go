package gscript_test

import (
	"errors"
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

func TestWithMaxNativeCallsLimitsInterpreterHostCalls(t *testing.T) {
	vm := gs.New(gs.WithMaxNativeCalls(3))
	var calls int64
	if err := vm.RegisterFunc("tick", func() int64 {
		calls++
		return calls
	}); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`
		for i := 1; i <= 5; i++ {
			tick()
		}
	`)
	if err == nil {
		t.Fatal("expected native call budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "native_calls" || budgetErr.Limit != 3 {
		t.Fatalf("budget = %s %d, want native_calls 3", budgetErr.Resource, budgetErr.Limit)
	}
	if calls != 3 {
		t.Fatalf("host calls = %d, want 3", calls)
	}
}

func TestWithMaxCallDepthLimitsInterpreterRecursion(t *testing.T) {
	vm := gs.New(gs.WithMaxCallDepth(8))
	err := vm.Exec(`
		func recurse(n) {
			return recurse(n + 1)
		}
		recurse(0)
	`)
	if err == nil {
		t.Fatal("expected call depth budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "call_depth" || budgetErr.Limit != 8 {
		t.Fatalf("budget = %s %d, want call_depth 8", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxNativeCallsLimitsBytecodeHostCalls(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxNativeCalls(3))
	var calls int64
	if err := vm.RegisterFunc("tick", func() int64 {
		calls++
		return calls
	}); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`
		for i := 1; i <= 5; i++ {
			tick()
		}
	`)
	if err == nil {
		t.Fatal("expected native call budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "native_calls" || budgetErr.Limit != 3 {
		t.Fatalf("budget = %s %d, want native_calls 3", budgetErr.Resource, budgetErr.Limit)
	}
	if calls != 3 {
		t.Fatalf("host calls = %d, want 3", calls)
	}
}

func TestWithMaxNativeCallsLimitsBytecodeFastStdlibCalls(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxNativeCalls(2))
	err := vm.Exec(`
		s := "abcdef"
		for i := 1; i <= 4; i++ {
			string.len(s)
		}
	`)
	if err == nil {
		t.Fatal("expected native call budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "native_calls" || budgetErr.Limit != 2 {
		t.Fatalf("budget = %s %d, want native_calls 2", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxCallDepthLimitsBytecodeRecursion(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxCallDepth(8))
	err := vm.Exec(`
		func recurse(n) {
			return recurse(n + 1)
		}
		recurse(0)
	`)
	if err == nil {
		t.Fatal("expected call depth budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "call_depth" || budgetErr.Limit != 8 {
		t.Fatalf("budget = %s %d, want call_depth 8", budgetErr.Resource, budgetErr.Limit)
	}
}
