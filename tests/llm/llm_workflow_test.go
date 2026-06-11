package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMWorkflowRunsStepsWithPriorTextAsInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: "draft about leia"},
				{Status: "final_answer", Text: "final: draft about leia"},
			}}
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)...)
			if err := vm.Exec(`
flow := llm.workflow({
    llm.step("draft", func(ctx) {
        return llm.turn({model: "mock-fast", messages: {llm.user(ctx.input)}})
    }),
    llm.step("revise", func(ctx) {
        return llm.turn({model: "mock-fast", messages: {llm.user(ctx.input)}})
    }),
})
result, err := flow.run("leia")
text := result.text
first_input := result.steps[1].input
second_input := result.steps[2].input
context_text := result.context.draft.text
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 2 {
				t.Fatalf("provider requests = %d, want 2", len(provider.requests))
			}
			if provider.requests[0].Messages[0].Text != "leia" || provider.requests[1].Messages[0].Text != "draft about leia" {
				t.Fatalf("request messages = %#v", provider.requests)
			}
			for name, want := range map[string]any{
				"text":         "final: draft about leia",
				"first_input":  "leia",
				"second_input": "draft about leia",
				"context_text": "draft about leia",
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
		})
	}
}

func TestLLMWorkflowSupportsProviderReplay(t *testing.T) {
	records := []llm.Record{
		{
			Request: llm.TurnRequest{
				Model:    "mock-fast",
				Messages: []llm.Message{{Role: "user", Text: "topic"}},
			},
			Result: llm.TurnResult{Status: "final_answer", Text: "draft"},
		},
		{
			Request: llm.TurnRequest{
				Model:    "mock-fast",
				Messages: []llm.Message{{Role: "user", Text: "draft"}},
			},
			Result: llm.TurnResult{Status: "final_answer", Text: "final"},
		},
	}
	vm := leia.New(leia.WithLibs(leia.LibString|leia.LibLLM), leia.WithLLMReplay(records))
	if err := vm.Exec(`
flow := llm.workflow({
    llm.step("draft", func(ctx) {
        return llm.turn({model: "mock-fast", messages: {llm.user(ctx.input)}})
    }),
    llm.step("final", func(ctx) {
        return llm.turn({model: "mock-fast", messages: {llm.user(ctx.input)}})
    }),
})
result, err := flow.run("topic")
text := result.text
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	text, _ := vm.Get("text")
	if text != "final" {
		t.Fatalf("text = %#v, want final", text)
	}
}

func TestLLMWorkflowMockSkipsProviderAndFeedsNextStep(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{}
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)...)
			if err := vm.Exec(`
flow := llm.workflow({
    llm.step("draft", func(ctx) {
        return llm.turn({messages: {llm.user("should not run")}})
    }),
    llm.step("review", func(ctx) {
        return {value: "reviewed " .. ctx.input, text: "reviewed " .. ctx.input}, nil
    }),
})
mocked := flow.mock({draft: {text: "mock draft"}})
result, err := mocked.run("topic")
text := result.text
draft_mocked := result.steps[1].mocked
review_input := result.steps[2].input
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 0 {
				t.Fatalf("provider requests = %d, want 0", len(provider.requests))
			}
			for name, want := range map[string]any{
				"text":         "reviewed mock draft",
				"draft_mocked": true,
				"review_input": "mock draft",
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
		})
	}
}

func TestLLMWorkflowCanCallAgentSteps(t *testing.T) {
	provider := &mockLLMProvider{results: []llm.TurnResult{
		{Status: "final_answer", Text: "agent draft"},
		{Status: "final_answer", Text: "agent final"},
	}}
	vm := leia.New(
		leia.WithLibs(leia.LibString|leia.LibLLM),
		leia.WithLLMProvider(provider),
	)
	if err := vm.Exec(`
writer := llm.agent("writer", func(topic) {
    return {model: "mock-fast", messages: {llm.user(topic)}}
})
flow := llm.workflow({
    llm.step("draft", func(ctx) {
        return writer(ctx.input)
    }),
    llm.step("final", func(ctx) {
        return writer(ctx.input)
    }),
})
result, err := flow.run("topic")
text := result.text
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	text, _ := vm.Get("text")
	if text != "agent final" {
		t.Fatalf("text = %#v, want agent final", text)
	}
}
