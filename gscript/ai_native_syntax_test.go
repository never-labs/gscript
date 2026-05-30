package gscript_test

import (
	"strings"
	"testing"

	gs "github.com/gscript/gscript/gscript"
)

func TestAINativeSyntaxExecutesThroughLLMStdlib(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []gs.LLMTurnResult{
				{Status: "final_answer", Text: "agent-ok"},
				{Status: "final_answer", Text: "turn-ok"},
			}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
tool lookup(query) {
    return "found:" .. query, nil
}

models {
    default: "mock-alias"
    "mock-alias": {provider_model: "mock-fast"}
}

agent defaults {
    model: "mock-alias"
    system: "Be concise."
    tools: [lookup]
}

answer := agent(q) {
    user: q
}

result, err := answer("search gscript")
direct, direct_err := turn {
    model: "direct-model"
    messages: messages { user: "single turn" }
}
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
			if len(agentReq.Messages) != 2 || agentReq.Messages[0].Role != "system" || agentReq.Messages[1].Text != "search gscript" {
				t.Fatalf("agent messages = %#v", agentReq.Messages)
			}
			if len(agentReq.Tools) != 1 || agentReq.Tools[0].Name != "lookup" {
				t.Fatalf("agent tools = %#v", agentReq.Tools)
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

func TestAINativeSyntaxValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "duplicate defaults",
			src: `
agent defaults { model: "a" }
agent defaults { model: "b" }
`,
			want: "duplicate agent defaults",
		},
		{
			name: "nested defaults",
			src: `
func f() {
    agent defaults { model: "a" }
}
`,
			want: "module scope",
		},
		{
			name: "literal api key",
			src: `
models {
    default: "m"
    m: {provider_model: "m", api_key: "secret"}
}
`,
			want: "api_key",
		},
		{
			name: "model alias cycle",
			src: `
models {
    a: "b"
    b: "a"
}
`,
			want: "alias cycle",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []struct {
				name string
				opts []gs.Option
			}{
				{name: "interpreter"},
				{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
			} {
				t.Run(mode.name, func(t *testing.T) {
					opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
					vm := gs.New(opts...)
					err := vm.Exec(tc.src)
					if err == nil || !strings.Contains(err.Error(), tc.want) {
						t.Fatalf("Exec error = %v, want substring %q", err, tc.want)
					}
				})
			}
		})
	}
}

func TestAINativeNamedAgentFlowAndDirectTurnSugar(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []gs.LLMTurnResult{
				{Status: "final_answer", Text: "flow-ok"},
				{Status: "final_answer", Text: "turn-sugar-ok"},
			}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
models {
    default: "alias"
    alias: "resolved-model"
}

tool echo_tool(query) {
    return query, nil
}

agent support(q) {
    model: "alias"
    tools: [echo_tool]
    system: "Use tools when useful."
    user: q
} flow {
    r, err := turn {
        model: model
        messages: messages {
            system: system
            user: q
        }
        tools: tools
    }
    return r, err
}

flow_result, flow_err := support("hello")
turn_result, turn_err := turn {
    user: "direct user shorthand"
}
flow_text := flow_result.text
turn_text := turn_result.text
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 2 {
				t.Fatalf("requests = %d, want 2", len(provider.requests))
			}
			if provider.requests[0].Model != "resolved-model" {
				t.Fatalf("flow model = %q, want resolved-model", provider.requests[0].Model)
			}
			if len(provider.requests[0].Messages) != 2 || provider.requests[0].Messages[0].Text != "Use tools when useful." || provider.requests[0].Messages[1].Text != "hello" {
				t.Fatalf("flow messages = %#v", provider.requests[0].Messages)
			}
			if len(provider.requests[0].Tools) != 1 || provider.requests[0].Tools[0].Name != "echo_tool" {
				t.Fatalf("flow tools = %#v", provider.requests[0].Tools)
			}
			if provider.requests[1].Model != "resolved-model" {
				t.Fatalf("turn sugar model = %q, want default resolved-model", provider.requests[1].Model)
			}
			if len(provider.requests[1].Messages) != 1 || provider.requests[1].Messages[0].Role != "user" || provider.requests[1].Messages[0].Text != "direct user shorthand" {
				t.Fatalf("turn sugar messages = %#v", provider.requests[1].Messages)
			}
			flowText, err := vm.Get("flow_text")
			if err != nil {
				t.Fatalf("Get flow_text: %v", err)
			}
			if flowText != "flow-ok" {
				t.Fatalf("flow_text = %#v", flowText)
			}
			turnText, err := vm.Get("turn_text")
			if err != nil {
				t.Fatalf("Get turn_text: %v", err)
			}
			if turnText != "turn-sugar-ok" {
				t.Fatalf("turn_text = %#v", turnText)
			}
		})
	}
}
