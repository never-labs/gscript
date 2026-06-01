package sync

import (
	"errors"
	stdsync "sync"
	"testing"
)

func TestAddWaitGroupReportsInvalidCounterState(t *testing.T) {
	var wg stdsync.WaitGroup
	if err := AddWaitGroup(&wg, -1); err == nil {
		t.Fatal("expected invalid counter state error")
	}
}

func TestTaskErrorsRecordsFirstErrorAndCount(t *testing.T) {
	var state TaskErrors
	if first := state.Record(nil); first {
		t.Fatal("nil error should not be recorded")
	}

	err1 := errors.New("first")
	err2 := errors.New("second")
	if first := state.Record(err1); !first {
		t.Fatal("first non-nil error should be reported")
	}
	if first := state.Record(err2); first {
		t.Fatal("second non-nil error should not be reported as first")
	}
	if got := state.Error(); got != err1 {
		t.Fatalf("Error() = %v, want %v", got, err1)
	}
	if got := state.Count(); got != 2 {
		t.Fatalf("Count() = %d, want 2", got)
	}
}
