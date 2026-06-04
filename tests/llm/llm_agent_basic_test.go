package leia_test

import (
	goruntime "runtime"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMDirectTurnMessagesReachProviderInVMJIT(t *testing.T) {
	cases := []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	}
	if goruntime.GOOS == "darwin" && goruntime.GOARCH == "arm64" {
		cases = append(cases, struct {
			name string
			opts []leia.Option
		}{name: "jit", opts: []leia.Option{leia.WithJIT()}})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: "ok"}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
history := {
    llm.system("Keep this system prompt."),
    llm.user("Keep this user prompt."),
}
result, err := llm.turn({
    model: "mock-chat"
    messages: history
})
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
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: "Paris"}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
llm.register_models({
    default: "chat"
    chat: {provider_model: "mock-chat"}
})

llm.agent_defaults({
    model: "chat"
    system: "Answer with only the requested fact."
})

answer := llm.agent("answer", func(question) {
    return {user: question}, nil
}, nil, {params: {"question"}})

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
						ID:   "call_lookup_1",
						Tool: "lookup",
						Args: map[string]any{"topic": "leia"},
					}},
				},
				{Status: "final_answer", Text: "Leia docs found."},
			}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
lookup := llm.tool("lookup", func(topic) {
    return "doc:" .. topic, nil
}, {
    params: {"topic"}
    requires: {"none"}
})

func researcher_config(topic) {
    return {
        model: "mock-react"
        system: "Use tools before answering."
        tools: {lookup}
        user: "Find docs for " .. topic
    }
}

researcher := llm.agent("researcher", researcher_config, nil, {params: {"topic"}})

result, err := researcher("leia")
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
			if len(first.Messages) != 2 || first.Messages[0].Role != "system" || first.Messages[1].Text != "Find docs for leia" {
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
				second.Messages[3].Value != "doc:leia" {
				t.Fatalf("second messages = %#v", second.Messages)
			}
			status, _ := vm.Get("status")
			text, _ := vm.Get("text")
			historyLen, _ := vm.Get("history_len")
			if status != "done" || text != "Leia docs found." || historyLen != int64(4) {
				t.Fatalf("status=%#v text=%#v history_len=%#v", status, text, historyLen)
			}
		})
	}
}

func TestLLMAgentScenarioComplexFlowCustomTurns(t *testing.T) {
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
						ID:   "call_lookup_2",
						Tool: "lookup",
						Args: map[string]any{"topic": "agents"},
					}},
				},
				{Status: "final_answer", Text: "Agents need explicit history."},
			}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
lookup := llm.tool("lookup", func(topic) {
    return "note:" .. topic, nil
}, {
    params: {"topic"}
    requires: {"none"}
})

func analyst_config(topic) {
    return {
        model: "mock-flow"
        system: "Build the answer from tool evidence."
        tools: {lookup}
    }
}

analyst := llm.agent("analyst", analyst_config, func(topic) {
    cfg := analyst_config(topic)
    history := {
        llm.system(cfg.system),
        llm.user("Investigate " .. topic),
    }

    first, first_err := llm.turn({
        model: cfg.model
        messages: history
        tools: cfg.tools
    })
    if first_err != nil {
        return nil, first_err
    }

    call := first.calls[1]
    value, dispatch_err := llm.dispatch(call, cfg.tools)
    if dispatch_err != nil {
        return nil, dispatch_err
    }
    history[#history + 1] = msg.assistant_call(call)
    history[#history + 1] = msg.tool_result(call.id, value)

    final, final_err := llm.turn({
        model: cfg.model
        messages: history
        tools: cfg.tools
        max_tokens: 48
    })
    return {
        first_status: first.status
        final_text: final.text
        tool_value: value
        history_len: #history
    }, final_err
}, {
    params: {"topic"}
    description: "Build the answer from tool evidence."
})

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
