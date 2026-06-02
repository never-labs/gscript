package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMAgentScenarioAgentAsToolStructuredHandoff(t *testing.T) {
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
						ID:   "call_delegate_1",
						Tool: "delegate_research",
						Args: map[string]any{"topic": "agent handoff"},
					}},
				},
				{
					Status: "final_answer",
					Text:   `{"summary":"Agents can be delegated as tools","confidence":0.91}`,
				},
				{Status: "final_answer", Text: "Use delegated research: Agents can be delegated as tools."},
			}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
agent extract_research(topic) {
    model: "mock-extractor"
    system: "Extract a structured research handoff."
    user: "Research " .. topic
    output: {
        summary: "short finding"
        confidence: 1
    }
}

//leia:requires none
tool delegate_research(topic) {
    result, err := extract_research(topic)
    if err != nil {
        return nil, err
    }
    return {
        topic: topic
        summary: result.value.summary
        confidence: result.value.confidence
    }, nil
}

agent supervisor(question) {
    model: "mock-supervisor"
    system: "Use delegated specialist agents as tools before answering."
    user: question
    tools: [delegate_research]
}

result, err := supervisor("Should this workflow delegate research?")
final_text := result.text
outer_history_len := #result.history
tool_summary := result.history[4].value.summary
tool_topic := result.history[4].value.topic
tool_confidence := result.history[4].value.confidence
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			if len(provider.requests) != 3 {
				t.Fatalf("requests = %d, want 3", len(provider.requests))
			}
			first := provider.requests[0]
			if first.Model != "mock-supervisor" || len(first.Tools) != 1 || first.Tools[0].Name != "delegate_research" {
				t.Fatalf("first request = %#v", first)
			}
			if len(first.Messages) != 2 ||
				first.Messages[0].Role != "system" || first.Messages[0].Text != "Use delegated specialist agents as tools before answering." ||
				first.Messages[1].Role != "user" || first.Messages[1].Text != "Should this workflow delegate research?" {
				t.Fatalf("first messages = %#v", first.Messages)
			}

			nested := provider.requests[1]
			if nested.Model != "mock-extractor" || len(nested.Tools) != 0 {
				t.Fatalf("nested request = %#v", nested)
			}
			if len(nested.Messages) != 2 ||
				nested.Messages[0].Role != "system" || nested.Messages[0].Text != "Extract a structured research handoff." ||
				nested.Messages[1].Role != "user" || nested.Messages[1].Text != "Research agent handoff" {
				t.Fatalf("nested messages = %#v", nested.Messages)
			}
			format, ok := nested.ResponseFormat.(map[string]any)
			if !ok || format["type"] != "json_object" {
				t.Fatalf("nested response_format = %#v", nested.ResponseFormat)
			}

			final := provider.requests[2]
			if final.Model != "mock-supervisor" || len(final.Tools) != 1 || final.Tools[0].Name != "delegate_research" {
				t.Fatalf("final request = %#v", final)
			}
			if len(final.Messages) != 4 ||
				final.Messages[2].Role != "assistant" || final.Messages[2].ToolCall == nil ||
				final.Messages[2].ToolCall.ID != "call_delegate_1" ||
				final.Messages[3].Role != "tool" || final.Messages[3].ToolUseID != "call_delegate_1" {
				t.Fatalf("final messages = %#v", final.Messages)
			}
			toolValue, ok := final.Messages[3].Value.(map[string]any)
			if !ok {
				t.Fatalf("tool result value = %#v", final.Messages[3].Value)
			}
			if toolValue["topic"] != "agent handoff" ||
				toolValue["summary"] != "Agents can be delegated as tools" ||
				toolValue["confidence"] != 0.91 {
				t.Fatalf("tool result value = %#v", toolValue)
			}

			for name, want := range map[string]any{
				"final_text":        "Use delegated research: Agents can be delegated as tools.",
				"outer_history_len": int64(4),
				"tool_summary":      "Agents can be delegated as tools",
				"tool_topic":        "agent handoff",
				"tool_confidence":   0.91,
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

func TestLLMAgentScenarioToolofRuntimeAgentAsTool(t *testing.T) {
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
						ID:   "call_delegate_toolof_1",
						Tool: "delegate_research",
						Args: map[string]any{"topic": "toolof"},
					}},
				},
				{
					Status: "final_answer",
					Text:   `{"summary":"toolof invokes the original agent","confidence":0.82}`,
				},
				{Status: "final_answer", Text: "Delegation complete."},
			}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
agent extract_research(topic) {
    model: "mock-extractor"
    system: "Return structured research."
    user: "Research " .. topic
    output: {
        summary: "short finding"
        confidence: 1
    }
}

delegate_research := llm.toolof(extract_research, {
    name: "delegate_research"
    description: "Delegate research to a specialist agent."
    requires: ["none"]
})
top_level_delegate := toolof(extract_research, {name: "top_level_delegate"})
alias_delegate := llm.agent_as_tool(extract_research, {name: "alias_delegate"})
runtime_tools := [delegate_research]

agent supervisor(question) {
    model: "mock-supervisor"
    system: "Use the specialist."
    user: question
    tools: runtime_tools
}

result, err := supervisor("Check delegation.")
final_text := result.text
tool_summary := result.history[4].value.summary
tool_confidence := result.history[4].value.confidence
delegate_param := delegate_research.params[1]
delegate_output_summary := delegate_research.output.summary
delegate_output_confidence := delegate_research.output.confidence
top_level_name := top_level_delegate.name
alias_name := alias_delegate.name
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			if len(provider.requests) != 3 {
				t.Fatalf("requests = %d, want 3", len(provider.requests))
			}
			first := provider.requests[0]
			if first.Model != "mock-supervisor" || len(first.Tools) != 1 {
				t.Fatalf("first request = %#v", first)
			}
			tool := first.Tools[0]
			if tool.Name != "delegate_research" ||
				tool.Description != "Delegate research to a specialist agent." ||
				len(tool.Params) != 1 || tool.Params[0] != "topic" ||
				len(tool.Requires) != 1 || tool.Requires[0] != "none" {
				t.Fatalf("tool metadata = %#v", tool)
			}
			if tool.Schema != nil {
				t.Fatalf("agent output shape must not be sent as provider tool input schema: %#v", tool.Schema)
			}
			nested := provider.requests[1]
			if nested.Model != "mock-extractor" || len(nested.Messages) != 2 || nested.Messages[1].Text != "Research toolof" {
				t.Fatalf("nested request = %#v", nested)
			}
			final := provider.requests[2]
			if final.Model != "mock-supervisor" || len(final.Messages) != 4 {
				t.Fatalf("final request = %#v", final)
			}
			toolValue, ok := final.Messages[3].Value.(map[string]any)
			if !ok || toolValue["summary"] != "toolof invokes the original agent" || toolValue["confidence"] != 0.82 {
				t.Fatalf("tool result value = %#v", final.Messages[3].Value)
			}
			for name, want := range map[string]any{
				"final_text":                 "Delegation complete.",
				"tool_summary":               "toolof invokes the original agent",
				"tool_confidence":            0.82,
				"delegate_param":             "topic",
				"delegate_output_summary":    "short finding",
				"delegate_output_confidence": int64(1),
				"top_level_name":             "top_level_delegate",
				"alias_name":                 "alias_delegate",
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

func TestLLMAgentScenarioDirectAgentInToolsList(t *testing.T) {
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
						ID:   "call_direct_agent_1",
						Tool: "extract_research",
						Args: map[string]any{"topic": "direct tools"},
					}},
				},
				{
					Status: "final_answer",
					Text:   `{"summary":"direct agent tools work","confidence":0.77}`,
				},
				{Status: "final_answer", Text: "Direct delegation complete."},
			}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
agent extract_research(topic) {
    model: "mock-extractor"
    system: "Return structured research."
    user: "Research " .. topic
    output: {
        summary: "short finding"
        confidence: 1
    }
}

agent supervisor(question) {
    model: "mock-supervisor"
    system: "Use the specialist."
    user: question
    tools: [extract_research]
}

result, err := supervisor("Check direct delegation.")
final_text := result.text
tool_summary := result.history[4].value.summary
tool_confidence := result.history[4].value.confidence
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			if len(provider.requests) != 3 {
				t.Fatalf("requests = %d, want 3", len(provider.requests))
			}
			first := provider.requests[0]
			if first.Model != "mock-supervisor" || len(first.Tools) != 1 {
				t.Fatalf("first request = %#v", first)
			}
			tool := first.Tools[0]
			if tool.Name != "extract_research" || len(tool.Params) != 1 || tool.Params[0] != "topic" {
				t.Fatalf("direct agent tool metadata = %#v", tool)
			}
			if tool.Schema != nil {
				t.Fatalf("direct agent output shape must not be sent as provider tool input schema: %#v", tool.Schema)
			}
			nested := provider.requests[1]
			if nested.Model != "mock-extractor" || len(nested.Messages) != 2 || nested.Messages[1].Text != "Research direct tools" {
				t.Fatalf("nested request = %#v", nested)
			}
			for name, want := range map[string]any{
				"final_text":      "Direct delegation complete.",
				"tool_summary":    "direct agent tools work",
				"tool_confidence": 0.77,
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
