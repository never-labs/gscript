package ai

import "testing"

func TestSelectChatModels(t *testing.T) {
	if got := SelectPlanModel("planner", "default"); got != "planner" {
		t.Fatalf("plan model = %q", got)
	}
	if got := SelectPlanModel("", "default"); got != "default" {
		t.Fatalf("plan fallback model = %q", got)
	}
	if got := SelectReflectModel("reviewer", "default"); got != "reviewer" {
		t.Fatalf("reflect model = %q", got)
	}
	if got := SelectReflectModel("", "default"); got != "default" {
		t.Fatalf("reflect fallback model = %q", got)
	}
}

func TestChatPrompts(t *testing.T) {
	if PlanPrompt() == "" {
		t.Fatalf("plan prompt should not be empty")
	}
	if got := ReflectPrompt("custom"); got != "custom" {
		t.Fatalf("reflect prompt = %q", got)
	}
	if ReflectPrompt("") == "" {
		t.Fatalf("default reflect prompt should not be empty")
	}
}

func TestExecutionPlanMessage(t *testing.T) {
	if _, ok := ExecutionPlanMessage(""); ok {
		t.Fatalf("empty plan should not produce a message")
	}
	got, ok := ExecutionPlanMessage("step 1")
	if !ok || got != "Execution plan:\nstep 1" {
		t.Fatalf("plan message = (%q, %v)", got, ok)
	}
}

func TestReflectIterations(t *testing.T) {
	if got := ReflectIterations(0); got != 1 {
		t.Fatalf("default reflect iterations = %d", got)
	}
	if got := ReflectIterations(3); got != 3 {
		t.Fatalf("explicit reflect iterations = %d", got)
	}
}
