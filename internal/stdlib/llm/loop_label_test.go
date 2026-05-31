package llm

import "testing"

func TestLoopFunctionLabel(t *testing.T) {
	if got := LoopFunctionLabel("loop", LoopNamePlanExecute); got != "loop.plan_execute" {
		t.Fatalf("loop label = %q", got)
	}
}
