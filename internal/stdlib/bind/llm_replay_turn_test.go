package bind

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestLLMTurnStrictFixtureReplayBypassesProvider(t *testing.T) {
	req := runtime.LLMTurnRequest{
		Model:    "mock",
		Messages: []runtime.LLMMessage{{Role: "user", Text: "hello replay"}},
	}
	requestHash := llmTurnRequestHash(req)
	var events []runtime.LLMTraceEvent
	provider := &testLLMProvider{res: runtime.LLMTurnResult{Status: "final_answer", Text: "live"}}
	interp := runLLMTestProgramWithTrace(t, `
result, err := llm.turn({
    model: "mock"
    messages: {llm.user("hello replay")}
    replay: {
        mode: "fixture_replay"
        replay_key: "turn:hello:v1"
        request_hash: "`+requestHash+`"
        response: {
            status: "final_answer"
            text: "fixture answer"
            reason: "stop"
            calls: {}
            usage: {input_tokens: 1 output_tokens: 2 cost: 0.0 latency_ms: 0}
        }
    }
})
text := result.text
mode := result.replay.mode
provider_free := result.replay.provider_free
live_network := result.replay.live_network
replay_key := result.replay.replay_key
hash := result.replay.request_hash
`, provider, func(event runtime.LLMTraceEvent) {
		events = append(events, event)
	})

	if len(provider.requests) != 0 {
		t.Fatalf("provider requests = %d, want 0", len(provider.requests))
	}
	assertGlobalString(t, interp, "text", "fixture answer")
	assertGlobalString(t, interp, "mode", "fixture_replay")
	assertGlobalString(t, interp, "replay_key", "turn:hello:v1")
	assertGlobalString(t, interp, "hash", requestHash)
	if got := interp.GetGlobal("provider_free"); !got.IsBool() || !got.Bool() {
		t.Fatalf("provider_free = %v, want true", got)
	}
	if got := interp.GetGlobal("live_network"); !got.IsBool() || got.Bool() {
		t.Fatalf("live_network = %v, want false", got)
	}
	if len(events) != 1 {
		t.Fatalf("trace events = %#v, want one replay event", events)
	}
	event := events[0]
	if event.Type != "turn_replay" || event.ReplayKey != "turn:hello:v1" || event.RequestHash != requestHash || event.ReplayMode != "fixture_replay" || !event.ProviderFree {
		t.Fatalf("trace replay metadata = %#v", event)
	}
}

func TestLLMTurnStrictFixtureReplayRejectsRequestHashMismatch(t *testing.T) {
	provider := &testLLMProvider{res: runtime.LLMTurnResult{Status: "final_answer", Text: "live"}}
	interp := runLLMTestProgram(t, `
result, err := llm.turn({
    model: "mock"
    messages: {llm.user("hello replay")}
    replay: {
        mode: "fixture_replay"
        replay_key: "turn:hello:v1"
        request_hash: "definitely-not-the-request-hash"
        response: {status: "final_answer" text: "fixture answer" calls: {} usage: {}}
    }
})
err_kind := err.kind
err_message := err.message
`, provider)

	if len(provider.requests) != 0 {
		t.Fatalf("provider requests = %d, want 0", len(provider.requests))
	}
	assertGlobalString(t, interp, "err_kind", "validation")
	msg := interp.GetGlobal("err_message")
	if !msg.IsString() || !strings.Contains(msg.Str(), "request_hash mismatch") {
		t.Fatalf("err_message = %v, want request_hash mismatch", msg)
	}
}

func assertGlobalString(t *testing.T, interp *runtime.Interpreter, name, want string) {
	t.Helper()
	got := interp.GetGlobal(name)
	if !got.IsString() || got.Str() != want {
		t.Fatalf("%s = %v, want %q", name, got, want)
	}
}
