package gscript_test

import (
	"testing"

	gs "github.com/never-labs/gscript"
	"github.com/never-labs/gscript/llm"
)

func TestLLMReactDispatchLoop(t *testing.T) {
	provider := &mockLLMProvider{results: []llm.TurnResult{
		{
			Status: "tool_calls",
			Calls: []llm.ToolCall{{
				ID:   "call_1",
				Tool: "lookup",
				Args: map[string]any{"name": "gscript"},
			}},
		},
		{Status: "final_answer", Text: "done"},
	}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(provider))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: {"name"}})
result, err := llm.react({
    messages: {llm.user("find docs")},
    tools: {lookup},
    max_steps: 3,
})
status := result.status
text := result.text
history_len := #result.history
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(provider.requests))
	}
	if len(provider.requests[1].Messages) != 3 || provider.requests[1].Messages[1].ToolCall == nil || provider.requests[1].Messages[2].Value != "docs:gscript" {
		t.Fatalf("second request messages = %#v", provider.requests[1].Messages)
	}
	status, _ := vm.Get("status")
	text, _ := vm.Get("text")
	historyLen, _ := vm.Get("history_len")
	if status != "done" || text != "done" || historyLen != int64(3) {
		t.Fatalf("status=%#v text=%#v history_len=%#v", status, text, historyLen)
	}
}

func TestLLMReactCanWindowHistory(t *testing.T) {
	provider := &mockLLMProvider{results: []llm.TurnResult{
		{
			Status: "tool_calls",
			Calls: []llm.ToolCall{{
				ID:   "call_1",
				Tool: "lookup",
				Args: map[string]any{"name": "gscript"},
			}},
		},
		{Status: "final_answer", Text: "done"},
	}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(provider))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: {"name"}})
result, err := llm.react({
    messages: {msg.system("drop this long system prompt"), msg.user("drop this long user prompt")},
    tools: {lookup},
    max_steps: 3,
    max_history_tokens: 10,
})
status := result.status
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(provider.requests))
	}
	second := provider.requests[1].Messages
	if len(second) != 2 || second[0].Role != "assistant" || second[0].ToolCall == nil || second[1].Role != "tool" || second[1].Value != "docs:gscript" {
		t.Fatalf("second request messages = %#v", second)
	}
	status, _ := vm.Get("status")
	if status != "done" {
		t.Fatalf("status = %#v", status)
	}
}

func TestLLMReactRetriesTransientToolErrors(t *testing.T) {
	provider := &mockLLMProvider{results: []llm.TurnResult{
		{
			Status: "tool_calls",
			Calls: []llm.ToolCall{{
				ID:   "call_1",
				Tool: "lookup",
				Args: map[string]any{"name": "gscript"},
			}},
		},
		{Status: "final_answer", Text: "done"},
	}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(provider))
	if err := vm.Exec(`
attempts := 0
lookup := llm.tool("lookup", func(name) {
    attempts = attempts + 1
    if attempts == 1 {
        return nil, {kind: "network", message: "retry me"}
    }
    return "docs:" .. name, nil
}, {params: {"name"}})
result, err := llm.react({
    messages: {llm.user("find docs")},
    tools: {lookup},
    max_steps: 3,
    max_tool_retries: 1,
})
status := result.status
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	attempts, _ := vm.Get("attempts")
	status, _ := vm.Get("status")
	if attempts != int64(2) || status != "done" {
		t.Fatalf("attempts=%#v status=%#v", attempts, status)
	}
	if len(provider.requests) != 2 || len(provider.requests[1].Messages) != 3 || provider.requests[1].Messages[2].Value != "docs:gscript" {
		t.Fatalf("requests = %#v", provider.requests)
	}
}

func TestLLMReactFatalToolErrorPropagates(t *testing.T) {
	provider := &mockLLMProvider{res: llm.TurnResult{
		Status: "tool_calls",
		Calls: []llm.ToolCall{{
			ID:   "call_1",
			Tool: "lookup",
			Args: map[string]any{"name": "gscript"},
		}},
	}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(provider))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return nil, {kind: "fatal", message: "stop"}
}, {params: {"name"}})
result, err := llm.react({
    messages: {llm.user("find docs")},
    tools: {lookup},
    max_steps: 1,
})
err_kind := err.kind
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	kind, _ := vm.Get("err_kind")
	if kind != "fatal" {
		t.Fatalf("err_kind = %#v", kind)
	}
}
