package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLoopBudgets(t *testing.T) {
	provider := &mockLLMProvider{results: []llm.TurnResult{
		{
			Status: "tool_calls",
			Calls: []llm.ToolCall{{
				ID:   "call_1",
				Tool: "lookup",
				Args: map[string]any{"name": "leia"},
			}},
			Usage: llm.TurnUsage{InputTokens: 2, OutputTokens: 3},
		},
		{Status: "final_answer", Text: "done"},
	}}
	vm := leia.New(leia.WithLibs(leia.LibString|leia.LibLLM), leia.WithLLMProvider(provider))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: ["name"]})
result, err := loop.react({
    user: "find docs",
    tools: [lookup],
    max_steps: 3,
    budget: {tokens: 5, turns: 3, calls: 2},
})
err_kind := err.kind
err_dimension := err.dimension
err_message := err.message
err_limit := err.limit
err_used := err.used

`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	kind, _ := vm.Get("err_kind")
	dimension, _ := vm.Get("err_dimension")
	if kind != "budget" || dimension != "tokens" {
		t.Fatalf("err kind=%#v dimension=%#v", kind, dimension)
	}
	message, _ := vm.Get("err_message")
	limit, _ := vm.Get("err_limit")
	used, _ := vm.Get("err_used")
	if message != "llm budget exceeded: tokens" || limit != int64(5) || used != int64(5) {
		t.Fatalf("err message=%#v limit=%#v used=%#v", message, limit, used)
	}
}

func TestLoopToolCallBudget(t *testing.T) {
	provider := &mockLLMProvider{res: llm.TurnResult{
		Status: "tool_calls",
		Calls: []llm.ToolCall{
			{ID: "call_1", Tool: "lookup", Args: map[string]any{"name": "a"}},
			{ID: "call_2", Tool: "lookup", Args: map[string]any{"name": "b"}},
		},
	}}
	vm := leia.New(leia.WithLibs(leia.LibString|leia.LibLLM), leia.WithLLMProvider(provider))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: ["name"]})
result, err := loop.react({
    user: "find docs",
    tools: [lookup],
    max_steps: 2,
    budget: {calls: 0},
})
err_kind := err.kind
err_dimension := err.dimension
err_message := err.message
err_limit_missing := err.limit == nil
err_used_missing := err.used == nil
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	kind, _ := vm.Get("err_kind")
	dimension, _ := vm.Get("err_dimension")
	if kind != "budget" || dimension != "calls" {
		t.Fatalf("err kind=%#v dimension=%#v", kind, dimension)
	}
	message, _ := vm.Get("err_message")
	limitMissing, _ := vm.Get("err_limit_missing")
	usedMissing, _ := vm.Get("err_used_missing")
	if message != "llm budget exceeded: calls" || limitMissing != true || usedMissing != true {
		t.Fatalf("err message=%#v limit_missing=%#v used_missing=%#v", message, limitMissing, usedMissing)
	}
}

func TestLoopTimeBudget(t *testing.T) {
	provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: "done"}}
	vm := leia.New(leia.WithLibs(leia.LibString|leia.LibLLM), leia.WithLLMProvider(provider))
	if err := vm.Exec(`
result, err := loop.react({
    user: "find docs",
    budget: {time: 0},
})
err_kind := err.kind
err_message := err.message
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	kind, _ := vm.Get("err_kind")
	message, _ := vm.Get("err_message")
	if kind != "deadline" || message != "deadline exceeded" {
		t.Fatalf("err kind=%#v message=%#v", kind, message)
	}
	if len(provider.requests) != 0 {
		t.Fatalf("provider requests = %d, want 0", len(provider.requests))
	}
}

func TestLoopScriptContextCancellation(t *testing.T) {
	provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: "done"}}
	vm := leia.New(leia.WithLibs(leia.LibString|leia.LibLLM), leia.WithLLMProvider(provider))
	if err := vm.Exec(`
ctx, cancel := context.withCancel()
cancel()
result, err := loop.react({
    user: "find docs",
    ctx: ctx,
})
err_kind := err.kind
err_message := err.message
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	kind, _ := vm.Get("err_kind")
	message, _ := vm.Get("err_message")
	if kind != "cancelled" || message != "cancelled" {
		t.Fatalf("err kind=%#v message=%#v", kind, message)
	}
	if len(provider.requests) != 0 {
		t.Fatalf("provider requests = %d, want 0", len(provider.requests))
	}
}
