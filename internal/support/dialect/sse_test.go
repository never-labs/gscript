package dialect

import "testing"

func TestParseSSEParsesEventsAndMultilineData(t *testing.T) {
	events, err := ParseSSE(": heartbeat\nid: 1\nevent: token\ndata: hello\ndata: world\nretry: 2500\n\n")
	if err != nil {
		t.Fatalf("ParseSSE: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if got := events[0]; got.ID != "1" || got.Event != "token" || got.Data != "hello\nworld" || got.Retry != 2500 {
		t.Fatalf("event = %#v", got)
	}
}

func TestParseSSERejectsBadRetry(t *testing.T) {
	if _, err := ParseSSE("retry: soon\n\n"); err == nil {
		t.Fatalf("ParseSSE bad retry returned nil error")
	}
}

func TestEncodeSSEWritesDeterministicFrames(t *testing.T) {
	got := EncodeSSE([]SSEEvent{{ID: "1", Event: "token", Data: "hello\nworld", Retry: 2500}})
	want := "event: token\nid: 1\nretry: 2500\ndata: hello\ndata: world\n\n"
	if got != want {
		t.Fatalf("EncodeSSE = %q, want %q", got, want)
	}
}
