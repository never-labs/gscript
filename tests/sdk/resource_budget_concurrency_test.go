package gscript_test

import (
	"errors"
	"testing"

	gs "github.com/never-labs/gscript"
)

func TestWithMaxGoroutinesLimitsInterpreterGoStatements(t *testing.T) {
	vm := gs.New(gs.WithMaxGoroutines(0))
	if err := vm.Exec(`func done() {}; go done()`); err != nil {
		t.Fatal(err)
	}

	limited := gs.New(gs.WithMaxGoroutines(1))
	err := limited.Exec(`
		block := make(chan)
		func worker() { <-block }
		go worker()
		go worker()
	`)
	if err == nil {
		t.Fatal("expected goroutine budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "goroutines" || budgetErr.Limit != 1 {
		t.Fatalf("budget = %s %d, want goroutines 1", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxChannelCapacityLimitsInterpreterMakeChan(t *testing.T) {
	vm := gs.New(gs.WithMaxChannelCapacity(2))
	err := vm.Exec(`ch := make(chan, 3)`)
	if err == nil {
		t.Fatal("expected channel capacity budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "channel_capacity" || budgetErr.Limit != 2 {
		t.Fatalf("budget = %s %d, want channel_capacity 2", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxGoroutinesLimitsBytecodeGoStatements(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxGoroutines(1))
	err := vm.Exec(`
		block := make(chan)
		func worker() { <-block }
		go worker()
		go worker()
	`)
	if err == nil {
		t.Fatal("expected goroutine budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "goroutines" || budgetErr.Limit != 1 {
		t.Fatalf("budget = %s %d, want goroutines 1", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxChannelCapacityLimitsBytecodeMakeChan(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxChannelCapacity(2))
	err := vm.Exec(`ch := make(chan, 3)`)
	if err == nil {
		t.Fatal("expected channel capacity budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "channel_capacity" || budgetErr.Limit != 2 {
		t.Fatalf("budget = %s %d, want channel_capacity 2", budgetErr.Resource, budgetErr.Limit)
	}
}
