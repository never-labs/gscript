package ai

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
