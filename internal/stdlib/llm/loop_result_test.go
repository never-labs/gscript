package llm

import "testing"

func TestBudgetExceededError(t *testing.T) {
	err := BudgetExceededError("calls", 3, 4)
	if err.Kind != ErrorKindBudget || err.Dimension != "calls" {
		t.Fatalf("budget identity = %+v", err)
	}
	if err.Message != "llm budget exceeded: calls" {
		t.Fatalf("budget message = %q", err.Message)
	}
	if err.Limit != 3 || err.Used != 4 {
		t.Fatalf("budget counters = %+v", err)
	}
}

func TestCancelledError(t *testing.T) {
	err := CancelledError("")
	if err.Kind != ErrorKindCancelled || err.Message != "cancelled" {
		t.Fatalf("default cancel = %+v", err)
	}
	err = CancelledError("context deadline exceeded")
	if err.Kind != ErrorKindDeadline || err.Message != "context deadline exceeded" {
		t.Fatalf("deadline cancel = %+v", err)
	}
}

func TestReactResult(t *testing.T) {
	result := ReactResult(ReactStatusStopped, "text", "max_steps")
	if result.Status != ReactStatusStopped || result.Text != "text" || result.Reason != "max_steps" {
		t.Fatalf("react result = %+v", result)
	}
}

func TestReactResultHelpers(t *testing.T) {
	done := ReactDoneResult("ok")
	if done.Status != ReactStatusDone || done.Text != "ok" || done.Reason != "" {
		t.Fatalf("done result = %+v", done)
	}
	stopped := ReactStoppedResult(StopReasonMaxSteps)
	if stopped.Status != ReactStatusStopped || stopped.Text != "" || stopped.Reason != StopReasonMaxSteps {
		t.Fatalf("stopped result = %+v", stopped)
	}
	pending := ReactPendingResult()
	if pending.Status != ReactStatusPending || pending.Text != "" || pending.Reason != "" {
		t.Fatalf("pending result = %+v", pending)
	}
}

func TestIsFinalAnswerStatus(t *testing.T) {
	if !IsFinalAnswerStatus("") || !IsFinalAnswerStatus(TurnStatusFinalAnswer) {
		t.Fatalf("final answer statuses were not recognized")
	}
	if IsFinalAnswerStatus(TurnStatusToolCalls) {
		t.Fatalf("tool calls should not be a final answer")
	}
}
