package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMSyntaxExecutesThroughLLMStdlib(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: "agent-ok"},
				{Status: "final_answer", Text: "turn-ok"},
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)
			err := vm.Exec(`
lookup := llm.tool("lookup", func(query) {
    return "found:" .. query, nil
}, {
    description: "Lookup docs."
    requires: {"docs.read", "net.client"}
    params: {"query"}
})

llm.register_models({
    default: "mock-alias",
    ["mock-alias"]: {provider_model: "mock-fast"}
})

llm.agent_defaults({
    model: "mock-alias"
    system: "Be concise."
    tools: {lookup}
})

answer := llm.agent("answer", func(q) {
    return {user: q}, nil
})

result, err := answer("search leia")
direct, direct_err := llm.turn({
    model: "direct-model"
    messages: {llm.user("single turn")}
})
agent_text := result.text
turn_text := direct.text
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 2 {
				t.Fatalf("requests = %d, want 2", len(provider.requests))
			}
			agentReq := provider.requests[0]
			if agentReq.Model != "mock-fast" {
				t.Fatalf("agent model = %q, want mock-fast", agentReq.Model)
			}
			if len(agentReq.Messages) != 2 || agentReq.Messages[0].Role != "system" || agentReq.Messages[1].Text != "search leia" {
				t.Fatalf("agent messages = %#v", agentReq.Messages)
			}
			if len(agentReq.Tools) != 1 || agentReq.Tools[0].Name != "lookup" {
				t.Fatalf("agent tools = %#v", agentReq.Tools)
			}
			if agentReq.Tools[0].Description != "Lookup docs." {
				t.Fatalf("tool description = %q", agentReq.Tools[0].Description)
			}
			if len(agentReq.Tools[0].Requires) != 2 || agentReq.Tools[0].Requires[0] != "docs.read" || agentReq.Tools[0].Requires[1] != "net.client" {
				t.Fatalf("tool requires = %#v", agentReq.Tools[0].Requires)
			}
			if provider.requests[1].Model != "direct-model" || len(provider.requests[1].Messages) != 1 || provider.requests[1].Messages[0].Text != "single turn" {
				t.Fatalf("turn request = %#v", provider.requests[1])
			}
			gotAgent, err := vm.Get("agent_text")
			if err != nil {
				t.Fatalf("Get agent_text: %v", err)
			}
			if gotAgent != "agent-ok" {
				t.Fatalf("agent_text = %#v", gotAgent)
			}
			gotTurn, err := vm.Get("turn_text")
			if err != nil {
				t.Fatalf("Get turn_text: %v", err)
			}
			if gotTurn != "turn-ok" {
				t.Fatalf("turn_text = %#v", gotTurn)
			}
		})
	}
}
