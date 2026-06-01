package leia_test

import (
	"errors"
	"fmt"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestErrorAPI_parseAndRuntimeKinds(t *testing.T) {
	vm := leia.New()

	parseErr := vm.Exec(`x :=`)
	if !errors.Is(parseErr, &leia.Error{Kind: leia.ErrParse}) {
		t.Fatalf("parse error is not identifiable as ErrParse: %T %v", parseErr, parseErr)
	}

	runtimeErr := vm.Exec(`x := 1 + "abc"`)
	if !errors.Is(runtimeErr, &leia.Error{Kind: leia.ErrRuntime}) {
		t.Fatalf("runtime error is not identifiable as ErrRuntime: %T %v", runtimeErr, runtimeErr)
	}
}

func TestErrorAPI_scriptErrorValue(t *testing.T) {
	vm := leia.New()
	err := vm.Exec(`error("boom")`)
	if !errors.Is(err, &leia.Error{Kind: leia.ErrScript}) {
		t.Fatalf("script error is not identifiable as ErrScript: %T %v", err, err)
	}
	var leiaErr *leia.Error
	if !errors.As(err, &leiaErr) {
		t.Fatalf("expected *leia.Error, got %T", err)
	}
	if leiaErr.Value != "boom" {
		t.Fatalf("script error value = %#v, want %q", leiaErr.Value, "boom")
	}
}

func TestErrorAPI_hostCallbackReturnedError(t *testing.T) {
	sentinel := errors.New("host failed")
	vm := leia.New()
	if err := vm.RegisterFunc("fail", func() error {
		return sentinel
	}); err != nil {
		t.Fatal(err)
	}

	_, err := vm.Call("fail")
	var hostErr *leia.HostCallbackError
	if !errors.As(err, &hostErr) {
		t.Fatalf("expected HostCallbackError, got %T %v", err, err)
	}
	if hostErr.Name != "fail" {
		t.Fatalf("host callback name = %q, want fail", hostErr.Name)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("host callback error does not unwrap to sentinel: %v", err)
	}
}

func TestErrorAPI_hostCallbackPanic(t *testing.T) {
	vm := leia.New()
	if err := vm.RegisterFunc("explode", func() {
		panic("boom")
	}); err != nil {
		t.Fatal(err)
	}

	err := vm.Exec(`explode()`)
	var panicErr *leia.HostCallbackPanicError
	if !errors.As(err, &panicErr) {
		t.Fatalf("expected HostCallbackPanicError, got %T %v", err, err)
	}
	if panicErr.Name != "explode" || panicErr.Value != "boom" {
		t.Fatalf("panic error = name:%q value:%#v", panicErr.Name, panicErr.Value)
	}
}

func TestErrorAPI_budgetError(t *testing.T) {
	for _, tc := range []struct {
		name string
		vm   *leia.VM
	}{
		{name: "interpreter", vm: leia.New(leia.WithMaxSteps(8))},
		{name: "bytecode", vm: leia.New(leia.WithVM(), leia.WithMaxSteps(8))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.vm.Exec(`
				i := 0
				for {
					i += 1
				}
			`)
			var budgetErr *leia.BudgetError
			if !errors.As(err, &budgetErr) {
				t.Fatalf("expected BudgetError, got %T %v", err, err)
			}
			if budgetErr.Resource != "steps" || budgetErr.Limit != 8 {
				t.Fatalf("budget = %s %d, want steps 8", budgetErr.Resource, budgetErr.Limit)
			}
			if !errors.Is(err, &leia.Error{Kind: leia.ErrRuntime}) {
				t.Fatalf("budget error should also be runtime error: %v", err)
			}
		})
	}
}

func TestErrorAPI_hostCallbackErrorFromScript(t *testing.T) {
	sentinel := fmt.Errorf("script host failed")
	vm := leia.New()
	if err := vm.RegisterFunc("failFromScript", func() error {
		return sentinel
	}); err != nil {
		t.Fatal(err)
	}

	err := vm.Exec(`failFromScript()`)
	var hostErr *leia.HostCallbackError
	if !errors.As(err, &hostErr) {
		t.Fatalf("expected HostCallbackError, got %T %v", err, err)
	}
	if hostErr.Name != "failFromScript" {
		t.Fatalf("host callback name = %q, want failFromScript", hostErr.Name)
	}
}
