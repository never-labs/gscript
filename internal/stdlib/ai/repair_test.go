package ai

import (
	"strings"
	"testing"
)

func TestStructuredOutputRepairPrompt(t *testing.T) {
	got := StructuredOutputRepairPrompt("  Fix JSON.  ", `{"bad":true}`, "missing summary", `{"summary":""}`)
	if !strings.HasPrefix(got, "Fix JSON.\n") {
		t.Fatalf("custom prompt was not trimmed/applied: %q", got)
	}
	for _, want := range []string{
		"Validation error: missing summary",
		`Output shape example: {"summary":""}`,
		"Previous response:\n{\"bad\":true}",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt missing %q in %q", want, got)
		}
	}
}

func TestStructuredOutputRepairPromptDefault(t *testing.T) {
	got := StructuredOutputRepairPrompt("", "not json", "", "")
	if !strings.HasPrefix(got, defaultStructuredOutputRepairPrompt+"\n") {
		t.Fatalf("default prompt not applied: %q", got)
	}
	if strings.Contains(got, "Validation error:") || strings.Contains(got, "Output shape example:") {
		t.Fatalf("empty optional fields should be omitted: %q", got)
	}
}
