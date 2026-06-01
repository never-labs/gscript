package leia_test

import (
	"errors"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestWithMaxStepsLimitsInterpreterExecution(t *testing.T) {
	vm := leia.New(leia.WithMaxSteps(8))
	err := vm.Exec(`
		i := 0
		for {
			i += 1
		}
	`)
	if err == nil {
		t.Fatal("expected max step error")
	}
	if !strings.Contains(err.Error(), "execution step limit exceeded") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithMaxStepsLimitsEmptyInterpreterLoop(t *testing.T) {
	vm := leia.New(leia.WithMaxSteps(8))
	err := vm.Exec(`for {}`)
	if err == nil {
		t.Fatal("expected step limit error")
	}
	var budgetErr *leia.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "steps" || budgetErr.Limit != 8 {
		t.Fatalf("budget = %s %d, want steps 8", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxStepsAllowsInterpreterExecutionWithinBudget(t *testing.T) {
	vm := leia.New(leia.WithMaxSteps(64))
	if err := vm.Exec(`
		sum := 0
		for i := 0; i < 5; i++ {
			sum += i
		}
	`); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("sum")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(10) {
		t.Fatalf("sum = %v (%T), want int64(10)", got, got)
	}
}

func TestWithMaxStepsLimitsBytecodeExecution(t *testing.T) {
	vm := leia.New(leia.WithVM(), leia.WithMaxSteps(8))
	err := vm.Exec(`
		i := 0
		for {
			i += 1
		}
	`)
	if err == nil {
		t.Fatal("expected max step error")
	}
	if !strings.Contains(err.Error(), "execution step limit exceeded") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithMaxStepsLimitsEmptyBytecodeLoop(t *testing.T) {
	vm := leia.New(leia.WithVM(), leia.WithMaxSteps(8))
	err := vm.Exec(`for {}`)
	if err == nil {
		t.Fatal("expected step limit error")
	}
	var budgetErr *leia.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "steps" || budgetErr.Limit != 8 {
		t.Fatalf("budget = %s %d, want steps 8", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxStepsAllowsBytecodeExecutionWithinBudget(t *testing.T) {
	vm := leia.New(leia.WithVM(), leia.WithMaxSteps(256))
	if err := vm.Exec(`
		sum := 0
		for i := 0; i < 5; i++ {
			sum += i
		}
	`); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("sum")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(10) {
		t.Fatalf("sum = %v (%T), want int64(10)", got, got)
	}
}
