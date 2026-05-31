package llm

import "testing"

func TestNewTextMessage(t *testing.T) {
	msg := NewTextMessage(RoleUser, "hello")
	if msg.Role != RoleUser || msg.Text != "hello" {
		t.Fatalf("message = %#v", msg)
	}
}

func TestDefaultTurnStatus(t *testing.T) {
	if got := DefaultTurnStatus("custom", 3); got != "custom" {
		t.Fatalf("explicit status = %q", got)
	}
	if got := DefaultTurnStatus("", 1); got != StatusToolCalls {
		t.Fatalf("tool status = %q", got)
	}
	if got := DefaultTurnStatus("", 0); got != StatusFinalAnswer {
		t.Fatalf("final status = %q", got)
	}
}

func TestToolMessageShapes(t *testing.T) {
	call := NewAssistantCallMessage()
	if call.Role != RoleAssistant || call.ToolCallKey != "tool_call" {
		t.Fatalf("assistant call shape = %+v", call)
	}

	result := NewToolResultMessage()
	if result.Role != RoleTool || result.ToolUseIDKey != "tool_use_id" || result.ValueKey != "value" || result.ErrorKey != "error" {
		t.Fatalf("tool result shape = %+v", result)
	}
}
