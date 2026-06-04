package leia_test

import (
	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
	"testing"
)

func TestLLMStandaloneBudgetLimitsDirectTurns(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: "one"},
				{Status: "final_answer", Text: "two"},
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)
			if err := vm.Exec(`
err_kind := nil
err_dimension := nil

llm.with_budget({turns: 1}, func() {
    first, first_err := llm.turn({messages: {llm.user("first")}})
    second, second_err := llm.turn({messages: {llm.user("second")}})
    err_kind = second_err.kind
    err_dimension = second_err.dimension
})
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			kind, _ := vm.Get("err_kind")
			dimension, _ := vm.Get("err_dimension")
			if kind != "budget" || dimension != "turns" {
				t.Fatalf("err kind=%#v dimension=%#v", kind, dimension)
			}
		})
	}
}

func TestLLMStandaloneBudgetNestedIntersectionUsesOuterTokens(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: "one", Usage: llm.TurnUsage{InputTokens: 2, OutputTokens: 3}},
				{Status: "final_answer", Text: "two"},
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)
			if err := vm.Exec(`
err_kind := nil
err_dimension := nil

llm.with_budget({tokens: 5}, func() {
    llm.with_budget({tokens: 100}, func() {
        first, first_err := llm.turn({messages: {llm.user("first")}})
        second, second_err := llm.turn({messages: {llm.user("second")}})
        err_kind = second_err.kind
        err_dimension = second_err.dimension
    })
})
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			kind, _ := vm.Get("err_kind")
			dimension, _ := vm.Get("err_dimension")
			if kind != "budget" || dimension != "tokens" {
				t.Fatalf("err kind=%#v dimension=%#v", kind, dimension)
			}
		})
	}
}

func TestLLMStandaloneBudgetLimitsToolCallsAndTime(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{
					Status: "tool_calls",
					Calls: []llm.ToolCall{{
						ID:   "call_1",
						Tool: "lookup",
						Args: map[string]any{"query": "leia"},
					}},
				},
				{Status: "final_answer", Text: "unused"},
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)
			if err := vm.Exec(`
lookup := llm.tool("lookup", func(query) {
    return "found:" .. query, nil
}, {params: {"query"}, requires: {"none"}})

call_kind := nil
call_dimension := nil
time_kind := nil
time_message := nil

llm.with_budget({calls: 0}, func() {
    result, err := llm.run_agent({
        model: "mock"
        tools: {lookup}
        user: "find leia"
    })
    call_kind = err.kind
    call_dimension = err.dimension
})

llm.with_budget({time: 0}, func() {
    result, err := llm.turn({messages: {llm.user("deadline")}})
    time_kind = err.kind
    time_message = err.message
})
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			callKind, _ := vm.Get("call_kind")
			callDimension, _ := vm.Get("call_dimension")
			if callKind != "budget" || callDimension != "calls" {
				t.Fatalf("call err kind=%#v dimension=%#v", callKind, callDimension)
			}
			timeKind, _ := vm.Get("time_kind")
			timeMessage, _ := vm.Get("time_message")
			if timeKind != "deadline" || timeMessage != "deadline exceeded" {
				t.Fatalf("time err kind=%#v message=%#v", timeKind, timeMessage)
			}
		})
	}
}
