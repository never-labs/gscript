package gscript_test

import (
	"errors"
	"fmt"
	"testing"

	gs "github.com/Never-Labs/gscript/gscript"
)

func TestErrorAPI_parseAndRuntimeKinds(t *testing.T) {
	vm := gs.New()

	parseErr := vm.Exec(`x :=`)
	if !errors.Is(parseErr, &gs.Error{Kind: gs.ErrParse}) {
		t.Fatalf("parse error is not identifiable as ErrParse: %T %v", parseErr, parseErr)
	}

	runtimeErr := vm.Exec(`x := 1 + "abc"`)
	if !errors.Is(runtimeErr, &gs.Error{Kind: gs.ErrRuntime}) {
		t.Fatalf("runtime error is not identifiable as ErrRuntime: %T %v", runtimeErr, runtimeErr)
	}
}

func TestErrorAPI_scriptErrorValue(t *testing.T) {
	vm := gs.New()
	err := vm.Exec(`error("boom")`)
	if !errors.Is(err, &gs.Error{Kind: gs.ErrScript}) {
		t.Fatalf("script error is not identifiable as ErrScript: %T %v", err, err)
	}
	var gsErr *gs.Error
	if !errors.As(err, &gsErr) {
		t.Fatalf("expected *gscript.Error, got %T", err)
	}
	if gsErr.Value != "boom" {
		t.Fatalf("script error value = %#v, want %q", gsErr.Value, "boom")
	}
}

func TestErrorAPI_hostCallbackReturnedError(t *testing.T) {
	sentinel := errors.New("host failed")
	vm := gs.New()
	if err := vm.RegisterFunc("fail", func() error {
		return sentinel
	}); err != nil {
		t.Fatal(err)
	}

	_, err := vm.Call("fail")
	var hostErr *gs.HostCallbackError
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
	vm := gs.New()
	if err := vm.RegisterFunc("explode", func() {
		panic("boom")
	}); err != nil {
		t.Fatal(err)
	}

	err := vm.Exec(`explode()`)
	var panicErr *gs.HostCallbackPanicError
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
		vm   *gs.VM
	}{
		{name: "interpreter", vm: gs.New(gs.WithMaxSteps(8))},
		{name: "bytecode", vm: gs.New(gs.WithVM(), gs.WithMaxSteps(8))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.vm.Exec(`
				i := 0
				for {
					i += 1
				}
			`)
			var budgetErr *gs.BudgetError
			if !errors.As(err, &budgetErr) {
				t.Fatalf("expected BudgetError, got %T %v", err, err)
			}
			if budgetErr.Resource != "steps" || budgetErr.Limit != 8 {
				t.Fatalf("budget = %s %d, want steps 8", budgetErr.Resource, budgetErr.Limit)
			}
			if !errors.Is(err, &gs.Error{Kind: gs.ErrRuntime}) {
				t.Fatalf("budget error should also be runtime error: %v", err)
			}
		})
	}
}

func TestErrorAPI_hostCallbackErrorFromScript(t *testing.T) {
	sentinel := fmt.Errorf("script host failed")
	vm := gs.New()
	if err := vm.RegisterFunc("failFromScript", func() error {
		return sentinel
	}); err != nil {
		t.Fatal(err)
	}

	err := vm.Exec(`failFromScript()`)
	var hostErr *gs.HostCallbackError
	if !errors.As(err, &hostErr) {
		t.Fatalf("expected HostCallbackError, got %T %v", err, err)
	}
	if hostErr.Name != "failFromScript" {
		t.Fatalf("host callback name = %q, want failFromScript", hostErr.Name)
	}
}
