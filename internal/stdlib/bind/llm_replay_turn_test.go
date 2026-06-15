package bind

import (
	"context"
	"fmt"
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
    messages: [llm.user("hello replay")]
    replay: {
        mode: "fixture_replay"
        replay_key: "turn:hello:v1"
        request_hash: "`+requestHash+`"
        response: {
            status: "final_answer"
            text: "fixture answer"
            reason: "stop"
            calls: []
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
    messages: [llm.user("hello replay")]
    replay: {
        mode: "fixture_replay"
        replay_key: "turn:hello:v1"
        request_hash: "definitely-not-the-request-hash"
        response: {status: "final_answer" text: "fixture answer" calls: [] usage: {}}
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

func TestLLMTurnAcceptsReplayRecordHelperOutput(t *testing.T) {
	provider := &testLLMProvider{res: runtime.LLMTurnResult{Status: "final_answer", Text: "live"}}
	interp := runLLMTestProgram(t, `
record, record_err := llm.replay_record({
    replay_key: "turn:helper"
    request: {
        model: "mock"
        messages: [llm.user("hello helper replay")]
    }
    response: {
        status: "final_answer"
        text: "fixture helper answer"
        calls: []
        usage: {input_tokens: 1 output_tokens: 2 cost: 0.0 latency_ms: 0}
    }
})
result, err := llm.turn({
    model: "mock"
    messages: [llm.user("hello helper replay")]
    replay: record.replay
})
record_err_nil := record_err == nil
err_nil := err == nil
text := result.text
mode := result.replay.mode
replay_key := result.replay.replay_key
`, provider)

	if len(provider.requests) != 0 {
		t.Fatalf("provider requests = %d, want 0", len(provider.requests))
	}
	if got := interp.GetGlobal("record_err_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("record_err_nil = %v, want true", got)
	}
	if got := interp.GetGlobal("err_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("err_nil = %v, want true", got)
	}
	assertGlobalString(t, interp, "text", "fixture helper answer")
	assertGlobalString(t, interp, "mode", "fixture_replay")
	assertGlobalString(t, interp, "replay_key", "turn:helper")
}

type testLLMReplayMatchProvider struct {
	*testLLMProvider
	match runtime.LLMReplayMatch
}

func (p *testLLMReplayMatchProvider) LastLLMReplayMatch() (runtime.LLMReplayMatch, bool) {
	return p.match, true
}

func TestLLMTurnEmitsReplayMatchTraceFromProvider(t *testing.T) {
	provider := &testLLMReplayMatchProvider{
		testLLMProvider: &testLLMProvider{res: runtime.LLMTurnResult{
			Status: "final_answer",
			Text:   "matched answer",
			Usage:  runtime.LLMTurnUsage{InputTokens: 2, OutputTokens: 3},
		}},
		match: runtime.LLMReplayMatch{
			Turn:            4,
			ReplayKey:       "turn:4",
			RequestHash:     "sha256:req",
			ResponseHash:    "sha256:res",
			ReplayMode:      "fixture_replay",
			ReplaySessionID: "session-4",
			ProviderFree:    true,
		},
	}
	var events []runtime.LLMTraceEvent
	runLLMTestProgramWithTrace(t, `
result, err := llm.turn({
    model: "mock"
    messages: [llm.user("hello replay provider")]
})
`, provider, func(event runtime.LLMTraceEvent) {
		events = append(events, event)
	})
	var matched *runtime.LLMTraceEvent
	for i := range events {
		if events[i].Type == "replay_record_matched" {
			matched = &events[i]
			break
		}
	}
	if matched == nil {
		t.Fatalf("trace events = %#v, want replay_record_matched", events)
	}
	if matched.Model != "mock" ||
		matched.Status != "matched" ||
		matched.TurnID != "turn:4" ||
		matched.ReplaySessionID != "session-4" ||
		matched.ReplayKey != "turn:4" ||
		matched.RequestHash != "sha256:req" ||
		matched.ResponseHash != "sha256:res" ||
		matched.ReplayMode != "fixture_replay" ||
		!matched.ProviderFree ||
		matched.MessageCount != 1 ||
		matched.Usage.OutputTokens != 3 {
		t.Fatalf("replay match trace = %#v", matched)
	}
}

type testLLMReactReplayMatchProvider struct {
	requests []runtime.LLMTurnRequest
	results  []runtime.LLMTurnResult
}

func (p *testLLMReactReplayMatchProvider) Turn(_ context.Context, req runtime.LLMTurnRequest) (runtime.LLMTurnResult, error) {
	p.requests = append(p.requests, req)
	if len(p.results) == 0 {
		return runtime.LLMTurnResult{Status: "final_answer", Text: "done"}, nil
	}
	res := p.results[0]
	p.results = p.results[1:]
	return res, nil
}

func (p *testLLMReactReplayMatchProvider) LastLLMReplayMatch() (runtime.LLMReplayMatch, bool) {
	turn := len(p.requests) - 1
	if turn < 0 {
		return runtime.LLMReplayMatch{}, false
	}
	return runtime.LLMReplayMatch{
		Turn:         turn,
		ReplayKey:    fmt.Sprintf("turn:%d", turn),
		RequestHash:  fmt.Sprintf("sha256:req:%d", turn),
		ResponseHash: fmt.Sprintf("sha256:res:%d", turn),
		ReplayMode:   "fixture_replay",
		ProviderFree: true,
	}, true
}

func TestLLMReactEmitsReplayMatchTraceForEachTurn(t *testing.T) {
	provider := &testLLMReactReplayMatchProvider{results: []runtime.LLMTurnResult{
		{
			Status: "tool_calls",
			Calls: []runtime.LLMToolCall{{
				ID:   "call_1",
				Tool: "lookup",
				Args: map[string]any{"name": "leia"},
			}},
		},
		{Status: "final_answer", Text: "done", Usage: runtime.LLMTurnUsage{InputTokens: 3, OutputTokens: 5}},
	}}
	var events []runtime.LLMTraceEvent
	runLLMTestProgramWithTrace(t, `
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: ["name"]})
result, err := llm.react({
    model: "mock"
    messages: [llm.user("find docs")]
    tools: [lookup]
    max_steps: 3
})
`, provider, func(event runtime.LLMTraceEvent) {
		events = append(events, event)
	})
	var matches []runtime.LLMTraceEvent
	seenToolCall := false
	seenToolResult := false
	seenReactDone := false
	for _, event := range events {
		switch event.Type {
		case "replay_record_matched":
			matches = append(matches, event)
		case "tool_call":
			seenToolCall = true
		case "tool_result":
			seenToolResult = true
		case "react_done":
			seenReactDone = true
		}
	}
	if len(matches) != 2 {
		t.Fatalf("matches = %#v, all events=%#v", matches, events)
	}
	if matches[0].Step != 0 || matches[0].ToolCount != 1 || matches[0].ReplayKey != "turn:0" || !matches[0].ProviderFree {
		t.Fatalf("first replay match = %#v", matches[0])
	}
	if matches[1].Step != 1 || matches[1].ToolCount != 1 || matches[1].ReplayKey != "turn:1" || matches[1].Usage.OutputTokens != 5 {
		t.Fatalf("second replay match = %#v", matches[1])
	}
	if !seenToolCall || !seenToolResult || !seenReactDone {
		t.Fatalf("tool/react events missing: tool_call=%v tool_result=%v react_done=%v events=%#v", seenToolCall, seenToolResult, seenReactDone, events)
	}
}

func assertGlobalString(t *testing.T, interp *runtime.Interpreter, name, want string) {
	t.Helper()
	got := interp.GetGlobal(name)
	if !got.IsString() || got.Str() != want {
		t.Fatalf("%s = %v, want %q", name, got, want)
	}
}
