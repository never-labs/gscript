package gscript_test

import (
	gs "github.com/never-labs/gscript/gscript"
	"testing"
)

func TestAINativeStandaloneBudgetLimitsDirectTurns(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []gs.LLMTurnResult{
				{Status: "final_answer", Text: "one"},
				{Status: "final_answer", Text: "two"},
			}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
err_kind := nil
err_dimension := nil

budget { turns: 1 } {
    first, first_err := turn { user: "first" }
    second, second_err := turn { user: "second" }
    err_kind = second_err.kind
    err_dimension = second_err.dimension
}
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

func TestAINativeStandaloneBudgetNestedIntersectionUsesOuterTokens(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []gs.LLMTurnResult{
				{Status: "final_answer", Text: "one", Usage: gs.LLMTurnUsage{InputTokens: 2, OutputTokens: 3}},
				{Status: "final_answer", Text: "two"},
			}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
err_kind := nil
err_dimension := nil

budget { tokens: 5 } {
    budget { tokens: 100 } {
        first, first_err := turn { user: "first" }
        second, second_err := turn { user: "second" }
        err_kind = second_err.kind
        err_dimension = second_err.dimension
    }
}
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

func TestAINativeStandaloneBudgetLimitsToolCallsAndTime(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []gs.LLMTurnResult{
				{
					Status: "tool_calls",
					Calls: []gs.LLMToolCall{{
						ID:   "call_1",
						Tool: "lookup",
						Args: map[string]any{"query": "gscript"},
					}},
				},
				{Status: "final_answer", Text: "unused"},
			}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
//gscript:requires none
tool lookup(query) {
    return "found:" .. query, nil
}

agent researcher() {
    model: "mock"
    tools: [lookup]
    user: "find gscript"
}

call_kind := nil
call_dimension := nil
time_kind := nil
time_message := nil

budget { calls: 0 } {
    result, err := researcher()
    call_kind = err.kind
    call_dimension = err.dimension
}

budget { time: 0 } {
    result, err := turn { user: "deadline" }
    time_kind = err.kind
    time_message = err.message
}
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
