//go:build darwin && arm64

package methodjit

import (
	"errors"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestExecuteQTypedRuntimeValueIntoSlotPreservesMessagesAndStoresValue(t *testing.T) {
	regs := []runtime.Value{runtime.NilValue()}
	err := executeQTypedRuntimeValueIntoSlot(
		regs,
		1,
		7,
		func() (runtime.Value, bool, error) {
			t.Fatal("execute should not run for out-of-range slot")
			return runtime.NilValue(), false, nil
		},
		"QSQLKernelPlan op-exit out of register range",
		"QSQLKernelPlan op-exit plan %d was not handled",
	)
	if err == nil || err.Error() != "QSQLKernelPlan op-exit out of register range" {
		t.Fatalf("out-of-range error = %v, want original qSQL message", err)
	}

	err = executeQTypedRuntimeValueIntoSlot(
		regs,
		0,
		7,
		func() (runtime.Value, bool, error) {
			return runtime.NilValue(), false, nil
		},
		"QEvalPipelinePlan exit out of register range",
		"QEvalPipelinePlan exit plan %d was not handled",
	)
	if err == nil || err.Error() != "QEvalPipelinePlan exit plan 7 was not handled" {
		t.Fatalf("unhandled error = %v, want original q.eval message", err)
	}

	wantErr := errors.New("boom")
	err = executeQTypedRuntimeValueIntoSlot(
		regs,
		0,
		7,
		func() (runtime.Value, bool, error) {
			return runtime.NilValue(), true, wantErr
		},
		"range",
		"unhandled %d",
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("execute error = %v, want boom", err)
	}

	err = executeQTypedRuntimeValueIntoSlot(
		regs,
		0,
		7,
		func() (runtime.Value, bool, error) {
			return runtime.IntValue(42), true, nil
		},
		"range",
		"unhandled %d",
	)
	if err != nil {
		t.Fatalf("success error = %v", err)
	}
	if !regs[0].IsInt() || regs[0].Int() != 42 {
		t.Fatalf("slot value = %v, want int 42", regs[0])
	}
}
