package gscript_test

import (
	goruntime "runtime"
	"strings"
	"testing"

	gs "github.com/never-labs/gscript"
	"github.com/never-labs/gscript/llm"
)

func TestLLMDirectTurnMessagesReachProviderInVMJIT(t *testing.T) {
	cases := []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	}
	if goruntime.GOOS == "darwin" && goruntime.GOARCH == "arm64" {
		cases = append(cases, struct {
			name string
			opts []gs.Option
		}{name: "jit", opts: []gs.Option{gs.WithJIT()}})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: "ok"}}
			vm := gs.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
history := messages {
    system: "Keep this system prompt."
    user: "Keep this user prompt."
}
result, err := turn {
    model: "mock-chat"
    messages: history
}
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			req := provider.requests[0]
			if len(req.Messages) != 2 {
				t.Fatalf("messages = %#v, want 2 messages", req.Messages)
			}
			if req.Messages[0].Role != "system" || strings.TrimSpace(req.Messages[0].Text) == "" {
				t.Fatalf("system message = %#v", req.Messages[0])
			}
			if req.Messages[1].Role != "user" || strings.TrimSpace(req.Messages[1].Text) == "" {
				t.Fatalf("user message = %#v", req.Messages[1])
			}
		})
	}
}

func TestLLMAgentScenarioSimpleDefaultsQuestionAnswer(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: "Paris"}}
			vm := gs.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
models {
    default: "chat"
    chat: {provider_model: "mock-chat"}
}

agent defaults {
    model: "chat"
    system: "Answer with only the requested fact."
}

answer := agent(question) {
    user: question
}

result, err := answer("What is the capital of France?")
answer_text := result.text
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			req := provider.requests[0]
			if req.Model != "mock-chat" {
				t.Fatalf("model = %q, want mock-chat", req.Model)
			}
			if len(req.Messages) != 2 ||
				req.Messages[0].Role != "system" || req.Messages[0].Text != "Answer with only the requested fact." ||
				req.Messages[1].Role != "user" || req.Messages[1].Text != "What is the capital of France?" {
				t.Fatalf("messages = %#v", req.Messages)
			}
			if len(req.Tools) != 0 {
				t.Fatalf("tools = %#v, want none", req.Tools)
			}
			answerText, err := vm.Get("answer_text")
			if err != nil {
				t.Fatalf("Get answer_text: %v", err)
			}
			if answerText != "Paris" {
				t.Fatalf("answer_text = %#v, want Paris", answerText)
			}
		})
	}
}

func TestLLMAgentScenarioReactToolAutoDispatch(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{
					Status: "tool_calls",
					Calls: []llm.ToolCall{{
						ID:   "call_lookup_1",
						Tool: "lookup",
						Args: map[string]any{"topic": "gscript"},
					}},
				},
				{Status: "final_answer", Text: "GScript docs found."},
			}}
			vm := gs.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
//gscript:requires none
tool lookup(topic) {
    return "doc:" .. topic, nil
}

agent researcher(topic) {
    model: "mock-react"
    system: "Use tools before answering."
    tools: [lookup]
    user: "Find docs for " .. topic
}

result, err := researcher("gscript")
status := result.status
text := result.text
history_len := #result.history
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			if len(provider.requests) != 2 {
				t.Fatalf("requests = %d, want 2", len(provider.requests))
			}
			first := provider.requests[0]
			if first.Model != "mock-react" || len(first.Tools) != 1 || first.Tools[0].Name != "lookup" {
				t.Fatalf("first request = %#v", first)
			}
			if len(first.Messages) != 2 || first.Messages[0].Role != "system" || first.Messages[1].Text != "Find docs for gscript" {
				t.Fatalf("first messages = %#v", first.Messages)
			}
			second := provider.requests[1]
			if second.Model != "mock-react" || len(second.Tools) != 1 || second.Tools[0].Name != "lookup" {
				t.Fatalf("second request = %#v", second)
			}
			if len(second.Messages) != 4 ||
				second.Messages[2].Role != "assistant" || second.Messages[2].ToolCall == nil ||
				second.Messages[2].ToolCall.Tool != "lookup" ||
				second.Messages[3].Role != "tool" || second.Messages[3].ToolUseID != "call_lookup_1" ||
				second.Messages[3].Value != "doc:gscript" {
				t.Fatalf("second messages = %#v", second.Messages)
			}
			status, _ := vm.Get("status")
			text, _ := vm.Get("text")
			historyLen, _ := vm.Get("history_len")
			if status != "done" || text != "GScript docs found." || historyLen != int64(4) {
				t.Fatalf("status=%#v text=%#v history_len=%#v", status, text, historyLen)
			}
		})
	}
}

func TestLLMAgentScenarioComplexFlowCustomTurns(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{
					Status: "tool_calls",
					Calls: []llm.ToolCall{{
						ID:   "call_lookup_2",
						Tool: "lookup",
						Args: map[string]any{"topic": "agents"},
					}},
				},
				{Status: "final_answer", Text: "Agents need explicit history."},
			}}
			vm := gs.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
//gscript:requires none
tool lookup(topic) {
    return "note:" .. topic, nil
}

agent analyst(topic) {
    model: "mock-flow"
    system: "Build the answer from tool evidence."
    tools: [lookup]
} flow {
    history := messages {
        system: system
        user: "Investigate " .. topic
    }

    first, first_err := turn {
        model: model
        messages: history
        tools: tools
    }
    if first_err != nil {
        return nil, first_err
    }

    call := first.calls[1]
    value, dispatch_err := llm.dispatch(call, tools)
    if dispatch_err != nil {
        return nil, dispatch_err
    }
    history[#history + 1] = msg.assistant_call(call)
    history[#history + 1] = msg.tool_result(call.id, value)

    final, final_err := turn {
        model: model
        messages: history
        tools: tools
        max_tokens: 48
    }
    return {
        first_status: first.status
        final_text: final.text
        tool_value: value
        history_len: #history
    }, final_err
}

out, err := analyst("agents")
first_status := out.first_status
final_text := out.final_text
tool_value := out.tool_value
history_len := out.history_len
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			if len(provider.requests) != 2 {
				t.Fatalf("requests = %d, want 2", len(provider.requests))
			}
			first := provider.requests[0]
			if first.Model != "mock-flow" || len(first.Tools) != 1 || first.Tools[0].Name != "lookup" {
				t.Fatalf("first request = %#v", first)
			}
			if len(first.Messages) != 2 || first.Messages[0].Text != "Build the answer from tool evidence." || first.Messages[1].Text != "Investigate agents" {
				t.Fatalf("first messages = %#v", first.Messages)
			}
			second := provider.requests[1]
			if second.Model != "mock-flow" || second.MaxTokens != 48 || len(second.Tools) != 1 || second.Tools[0].Name != "lookup" {
				t.Fatalf("second request = %#v", second)
			}
			if len(second.Messages) != 4 ||
				second.Messages[2].Role != "assistant" || second.Messages[2].ToolCall == nil ||
				second.Messages[2].ToolCall.ID != "call_lookup_2" ||
				second.Messages[3].Role != "tool" || second.Messages[3].ToolUseID != "call_lookup_2" ||
				second.Messages[3].Value != "note:agents" {
				t.Fatalf("second messages = %#v", second.Messages)
			}
			firstStatus, _ := vm.Get("first_status")
			finalText, _ := vm.Get("final_text")
			toolValue, _ := vm.Get("tool_value")
			historyLen, _ := vm.Get("history_len")
			if firstStatus != "tool_calls" || finalText != "Agents need explicit history." || toolValue != "note:agents" || historyLen != int64(4) {
				t.Fatalf("first_status=%#v final_text=%#v tool_value=%#v history_len=%#v", firstStatus, finalText, toolValue, historyLen)
			}
		})
	}
}
