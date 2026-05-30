package gscript_test

import (
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	gs "github.com/Never-Labs/gscript/gscript"
)

func TestAINativeDirectTurnMessagesReachProviderInVMJIT(t *testing.T) {
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
			provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: "ok"}}
			vm := gs.New(aiNativeScenarioOptions(provider, tc.opts...)...)

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

func TestAINativeAgentScenarioSimpleDefaultsQuestionAnswer(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: "Paris"}}
			vm := gs.New(aiNativeScenarioOptions(provider, tc.opts...)...)

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

func TestAINativeAgentScenarioReactToolAutoDispatch(t *testing.T) {
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
						ID:   "call_lookup_1",
						Tool: "lookup",
						Args: map[string]any{"topic": "gscript"},
					}},
				},
				{Status: "final_answer", Text: "GScript docs found."},
			}}
			vm := gs.New(aiNativeScenarioOptions(provider, tc.opts...)...)

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

func TestAINativeAgentScenarioComplexFlowCustomTurns(t *testing.T) {
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
						ID:   "call_lookup_2",
						Tool: "lookup",
						Args: map[string]any{"topic": "agents"},
					}},
				},
				{Status: "final_answer", Text: "Agents need explicit history."},
			}}
			vm := gs.New(aiNativeScenarioOptions(provider, tc.opts...)...)

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

func TestAINativeAgentScenarioAgentAsToolStructuredHandoff(t *testing.T) {
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
			vm := gs.New(aiNativeScenarioOptions(provider, tc.opts...)...)

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

//gscript:requires none
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

func TestAINativeAgentScenarioToolofRuntimeAgentAsTool(t *testing.T) {
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
			vm := gs.New(aiNativeScenarioOptions(provider, tc.opts...)...)

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
delegate_schema_summary := delegate_research.schema.summary
delegate_schema_confidence := delegate_research.schema.confidence
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
			schema, ok := tool.Schema.(map[string]any)
			if !ok || schema["summary"] != "short finding" || schema["confidence"] != int64(1) {
				t.Fatalf("tool schema = %#v", tool.Schema)
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
				"delegate_schema_summary":    "short finding",
				"delegate_schema_confidence": int64(1),
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

func TestAINativeAgentScenarioDirectAgentInToolsList(t *testing.T) {
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
			vm := gs.New(aiNativeScenarioOptions(provider, tc.opts...)...)

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
			schema, ok := tool.Schema.(map[string]any)
			if !ok || schema["summary"] != "short finding" || schema["confidence"] != int64(1) {
				t.Fatalf("direct agent tool schema = %#v", tool.Schema)
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

func TestAINativeAgentTurnScenarioRecordReplay(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `
models {
    default: "chat"
    chat: {provider_model: "mock-chat"}
}

agent reviewer(topic) {
    model: "chat"
    system: "Review with two passes."
} flow {
    draft, draft_err := turn {
        messages: messages {
            system: system
            user: "Draft " .. topic
        }
        max_tokens: 32
    }
    if draft_err != nil {
        return nil, draft_err
    }

    final, final_err := turn {
        messages: messages {
            user: draft.text .. " / final"
        }
    }
    return {draft: draft.text, final: final.text}, final_err
}

out, err := reviewer("recording")
draft_text := out.draft
final_text := out.final
`
			provider := &mockLLMProvider{results: []gs.LLMTurnResult{
				{Status: "final_answer", Text: "draft pass", Usage: gs.LLMTurnUsage{InputTokens: 2, OutputTokens: 3}},
				{Status: "final_answer", Text: "final pass", Usage: gs.LLMTurnUsage{InputTokens: 4, OutputTokens: 5}},
			}}
			recorder := gs.NewLLMRecorder()
			recordOpts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
				gs.WithLLMRecorder(recorder.Record),
			}, tc.opts...)
			recordVM := gs.New(recordOpts...)

			if err := recordVM.Exec(source); err != nil {
				t.Fatalf("record Exec: %v", err)
			}
			records := recorder.Records()
			if len(records) != 2 {
				t.Fatalf("records = %#v, want 2", records)
			}
			if records[0].Request.Model != "mock-chat" || records[0].Request.MaxTokens != 32 {
				t.Fatalf("first recorded request = %#v", records[0].Request)
			}
			if len(records[0].Request.Messages) != 2 ||
				records[0].Request.Messages[0].Role != "system" ||
				records[0].Request.Messages[0].Text != "Review with two passes." ||
				records[0].Request.Messages[1].Text != "Draft recording" {
				t.Fatalf("first recorded messages = %#v", records[0].Request.Messages)
			}
			if records[1].Request.Model != "mock-chat" || len(records[1].Request.Messages) != 1 ||
				records[1].Request.Messages[0].Text != "draft pass / final" {
				t.Fatalf("second recorded request = %#v", records[1].Request)
			}

			replayOpts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMReplay(records),
			}, tc.opts...)
			replayVM := gs.New(replayOpts...)
			if err := replayVM.Exec(source); err != nil {
				t.Fatalf("replay Exec: %v", err)
			}
			for name, want := range map[string]any{
				"draft_text": "draft pass",
				"final_text": "final pass",
			} {
				got, err := replayVM.Get(name)
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

func TestAINativeAgentScenarioIncidentResponseExampleSmoke(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []gs.LLMTurnResult{
				{Status: "final_answer", Text: "Checkout incidents are owned by payments on-call."},
				{
					Status: "tool_calls",
					Calls: []gs.LLMToolCall{{
						ID:   "call_metrics_1",
						Tool: "get_metrics",
						Args: map[string]any{"service": "checkout"},
					}},
				},
				{Status: "final_answer", Text: "Checkout latency is elevated; page payments on-call."},
				{
					Status: "tool_calls",
					Calls: []gs.LLMToolCall{{
						ID:   "call_runbook_1",
						Tool: "search_runbook",
						Args: map[string]any{
							"service": "checkout",
							"symptom": "p95 latency spike",
						},
					}},
				},
				{Status: "final_answer", Text: "Incident brief: checkout latency spike, follow runbook."},
			}}
			vm := gs.New(aiNativeScenarioOptions(provider, tc.opts...)...)

			if err := vm.ExecFile(filepath.Join("..", "examples", "ai_native_incident_response.gs")); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}

			if len(provider.requests) != 5 {
				t.Fatalf("requests = %d, want 5", len(provider.requests))
			}
			faq := provider.requests[0]
			if faq.Model != "mock-ops" || len(faq.Tools) != 0 {
				t.Fatalf("faq request = %#v", faq)
			}
			if len(faq.Messages) != 2 || faq.Messages[0].Role != "system" || faq.Messages[1].Text != "Who owns checkout incidents?" {
				t.Fatalf("faq messages = %#v", faq.Messages)
			}

			researchFirst := provider.requests[1]
			if researchFirst.Model != "mock-ops" || len(researchFirst.Tools) != 2 {
				t.Fatalf("research first request = %#v", researchFirst)
			}
			if researchFirst.Tools[0].Name != "search_runbook" || researchFirst.Tools[1].Name != "get_metrics" {
				t.Fatalf("research tools = %#v", researchFirst.Tools)
			}
			researchFinal := provider.requests[2]
			if len(researchFinal.Messages) != 4 ||
				researchFinal.Messages[2].ToolCall == nil ||
				researchFinal.Messages[2].ToolCall.Tool != "get_metrics" ||
				researchFinal.Messages[3].Value != "metrics:checkout:latency=high,error_rate=2%" {
				t.Fatalf("research final messages = %#v", researchFinal.Messages)
			}

			briefFirst := provider.requests[3]
			if briefFirst.Model != "mock-planner" || len(briefFirst.Tools) != 2 {
				t.Fatalf("brief first request = %#v", briefFirst)
			}
			briefFinal := provider.requests[4]
			if briefFinal.Model != "mock-planner" || briefFinal.MaxTokens != 320 {
				t.Fatalf("brief final request = %#v", briefFinal)
			}
			if len(briefFinal.Messages) != 4 ||
				briefFinal.Messages[2].ToolCall == nil ||
				briefFinal.Messages[2].ToolCall.ID != "call_runbook_1" ||
				briefFinal.Messages[3].ToolUseID != "call_runbook_1" ||
				briefFinal.Messages[3].Value != "runbook:checkout:p95 latency spike" {
				t.Fatalf("brief final messages = %#v", briefFinal.Messages)
			}

			for name, want := range map[string]any{
				"faq_text":          "Checkout incidents are owned by payments on-call.",
				"research_text":     "Checkout latency is elevated; page payments on-call.",
				"brief_text":        "Incident brief: checkout latency spike, follow runbook.",
				"brief_evidence":    "runbook:checkout:p95 latency spike",
				"brief_history_len": int64(4),
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

func TestAINativeDirectTurnResponseFormatProviderRequest(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: `{"name":"Ada","email":"ada@example.com"}`}}
			vm := gs.New(aiNativeScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
result, err := turn {
    model: "mock-json"
    messages: messages {
        system: "Return only valid JSON."
        user: "Extract the contact."
    }
    response_format: {
        type: "json_schema"
        json_schema: {
            name: "contact"
            schema: {
                type: "object"
                properties: {
                    name: {type: "string"}
                    email: {type: "string"}
                }
                required: ["name", "email"]
                additionalProperties: false
            }
        }
    }
}
text := result.text
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			req := provider.requests[0]
			if req.Model != "mock-json" {
				t.Fatalf("model = %q, want mock-json", req.Model)
			}
			if len(req.Messages) != 2 ||
				req.Messages[0].Role != "system" || req.Messages[0].Text != "Return only valid JSON." ||
				req.Messages[1].Role != "user" || req.Messages[1].Text != "Extract the contact." {
				t.Fatalf("messages = %#v", req.Messages)
			}
			format, ok := req.ResponseFormat.(map[string]any)
			if !ok || format["type"] != "json_schema" {
				t.Fatalf("response_format = %#v", req.ResponseFormat)
			}
			jsonSchema, ok := format["json_schema"].(map[string]any)
			if !ok || jsonSchema["name"] != "contact" {
				t.Fatalf("json_schema = %#v", format["json_schema"])
			}
			schema, ok := jsonSchema["schema"].(map[string]any)
			if !ok || schema["type"] != "object" || schema["additionalProperties"] != false {
				t.Fatalf("schema = %#v", jsonSchema["schema"])
			}
			properties, ok := schema["properties"].(map[string]any)
			if !ok {
				t.Fatalf("properties = %#v", schema["properties"])
			}
			name, ok := properties["name"].(map[string]any)
			if !ok || name["type"] != "string" {
				t.Fatalf("name property = %#v", properties["name"])
			}
			required, ok := schema["required"].([]any)
			if !ok || len(required) != 2 || required[0] != "name" || required[1] != "email" {
				t.Fatalf("required = %#v", schema["required"])
			}
			text, err := vm.Get("text")
			if err != nil {
				t.Fatalf("Get text: %v", err)
			}
			if text != `{"name":"Ada","email":"ada@example.com"}` {
				t.Fatalf("text = %#v", text)
			}
		})
	}
}

func TestAINativeAgentOutputStructuredValue(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: `{"name":"Ada","email":"ada@example.com"}`}}
			vm := gs.New(aiNativeScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
agent extract_contact(text) {
    model: "mock-json"
    system: "Extract contact information."
    user: text
    output: {
        name: "Ada Lovelace"
        email: "ada@example.com"
    }
}

result, err := extract_contact("Ada <ada@example.com>")
name := result.value.name
email := result.value.email
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			format, ok := provider.requests[0].ResponseFormat.(map[string]any)
			if !ok || format["type"] != "json_object" {
				t.Fatalf("response_format = %#v", provider.requests[0].ResponseFormat)
			}
			name, err := vm.Get("name")
			if err != nil {
				t.Fatalf("Get name: %v", err)
			}
			if name != "Ada" {
				t.Fatalf("name = %#v, want Ada", name)
			}
			email, err := vm.Get("email")
			if err != nil {
				t.Fatalf("Get email: %v", err)
			}
			if email != "ada@example.com" {
				t.Fatalf("email = %#v, want ada@example.com", email)
			}
		})
	}
}

func TestAINativeAgentOutputKeepsExplicitResponseFormat(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: `{"ok":true}`}}
			vm := gs.New(aiNativeScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
agent extract(text) {
    model: "mock-json"
    user: text
    output: {ok: true}
    response_format: {type: "json_schema", name: "explicit"}
}

result, err := extract("ok")
ok := result.value.ok
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			format, ok := provider.requests[0].ResponseFormat.(map[string]any)
			if !ok || format["type"] != "json_schema" || format["name"] != "explicit" {
				t.Fatalf("response_format = %#v", provider.requests[0].ResponseFormat)
			}
			value, err := vm.Get("ok")
			if err != nil {
				t.Fatalf("Get ok: %v", err)
			}
			if value != true {
				t.Fatalf("ok = %#v, want true", value)
			}
		})
	}
}

func TestAINativeAgentOutputValidationError(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: `not json`}}
			vm := gs.New(aiNativeScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
agent extract_contact(text) {
    model: "mock-json"
    user: text
    output: {name: "Ada"}
}

result, err := extract_contact("Ada")
err_kind := err.kind
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			format, ok := provider.requests[0].ResponseFormat.(map[string]any)
			if !ok || format["type"] != "json_object" {
				t.Fatalf("response_format = %#v", provider.requests[0].ResponseFormat)
			}
			kind, err := vm.Get("err_kind")
			if err != nil {
				t.Fatalf("Get err_kind: %v", err)
			}
			if kind != "validation" {
				t.Fatalf("err_kind = %#v, want validation", kind)
			}
		})
	}
}

func TestAINativeAgentOutputValidationMissingField(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: `{"name":"Ada"}`}}
			vm := gs.New(aiNativeScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
agent extract_contact(text) {
    model: "mock-json"
    user: text
    output: {
        name: "Ada"
        email: "ada@example.com"
    }
}

result, err := extract_contact("Ada")
err_kind := err.kind
err_message := err.message
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			kind, err := vm.Get("err_kind")
			if err != nil {
				t.Fatalf("Get err_kind: %v", err)
			}
			if kind != "validation" {
				t.Fatalf("err_kind = %#v, want validation", kind)
			}
			message, err := vm.Get("err_message")
			if err != nil {
				t.Fatalf("Get err_message: %v", err)
			}
			if !strings.Contains(message.(string), `missing field "email"`) {
				t.Fatalf("err_message = %#v, want missing email field", message)
			}
		})
	}
}

func TestAINativeAgentOutputValidationTypeMismatch(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: `{"name":"Ada","score":"high","ok":true,"meta":{}}`}}
			vm := gs.New(aiNativeScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
agent classify(text) {
    model: "mock-json"
    user: text
    output: {
        name: "Ada"
        score: 1
        ok: true
        meta: {source: "email"}
    }
}

result, err := classify("Ada")
err_kind := err.kind
err_message := err.message
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			kind, err := vm.Get("err_kind")
			if err != nil {
				t.Fatalf("Get err_kind: %v", err)
			}
			if kind != "validation" {
				t.Fatalf("err_kind = %#v, want validation", kind)
			}
			message, err := vm.Get("err_message")
			if err != nil {
				t.Fatalf("Get err_message: %v", err)
			}
			if !strings.Contains(message.(string), `field "score" has type string, want number`) {
				t.Fatalf("err_message = %#v, want score type mismatch", message)
			}
		})
	}
}

func TestAINativeCustomFlowDoesNotAutoValidateOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: `not json`}}
			vm := gs.New(aiNativeScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
agent extract(text) {
    model: "mock-json"
    user: text
    output: {name: "Ada"}
} flow {
    result, err := turn {}
    return result, err
}

result, err := extract("Ada")
text := result.text
err_is_nil := err == nil
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			format, ok := provider.requests[0].ResponseFormat.(map[string]any)
			if !ok || format["type"] != "json_object" {
				t.Fatalf("response_format = %#v", provider.requests[0].ResponseFormat)
			}
			text, err := vm.Get("text")
			if err != nil {
				t.Fatalf("Get text: %v", err)
			}
			errIsNil, err := vm.Get("err_is_nil")
			if err != nil {
				t.Fatalf("Get err_is_nil: %v", err)
			}
			if text != "not json" || errIsNil != true {
				t.Fatalf("text=%#v err_is_nil=%#v, want unvalidated flow result", text, errIsNil)
			}
		})
	}
}

func aiNativeScenarioOptions(provider gs.LLMProvider, opts ...gs.Option) []gs.Option {
	base := []gs.Option{
		gs.WithLibs(gs.LibString | gs.LibLLM),
		gs.WithLLMProvider(provider),
	}
	return append(base, opts...)
}
