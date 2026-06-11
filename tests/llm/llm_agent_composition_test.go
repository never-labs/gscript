package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMAgentCompositionHandoffAlias(t *testing.T) {
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
						ID:   "call_handoff_1",
						Tool: "handoff_research",
						Args: map[string]any{"topic": "composition"},
					}},
				},
				{
					Status: "final_answer",
					Text:   `{"summary":"handoff alias delegates through agent tooling","confidence":0.86}`,
				},
				{Status: "final_answer", Text: "Delegated composition complete."},
			}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
func research_config(topic) {
    return {
        model: "mock-specialist"
        system: "Return structured research."
        user: "Research " .. topic
        output: {
            summary: "short finding"
            confidence: 1
        }
    }, nil
}

research_agent := llm.agent("research_agent", research_config, nil, {
    params: {"topic"}
    output: {
        summary: "short finding"
        confidence: 1
    }
})

handoff_research := llm.handoff(research_agent, {
    name: "handoff_research"
    description: "Delegate research to another agent."
    requires: {"none"}
})

result, err := llm.run_agent({
    model: "mock-supervisor"
    system: "Use handoff tools before answering."
    user: "Check the composition path."
    tools: {handoff_research}
})

err_kind := nil
final_text := nil
tool_summary := nil
tool_confidence := nil
handoff_param := handoff_research.params[1]
handoff_output_summary := handoff_research.output.summary
if err != nil {
    err_kind = err.kind
} else {
    final_text = result.text
    tool_summary = result.history[4].value.summary
    tool_confidence = result.history[4].value.confidence
}
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if errKind, _ := vm.Get("err_kind"); errKind != nil {
				t.Fatalf("err_kind = %#v", errKind)
			}
			if len(provider.requests) != 3 {
				t.Fatalf("requests = %d, want 3", len(provider.requests))
			}
			first := provider.requests[0]
			if len(first.Tools) != 1 {
				t.Fatalf("first tools = %#v", first.Tools)
			}
			tool := first.Tools[0]
			if tool.Name != "handoff_research" ||
				tool.Description != "Delegate research to another agent." ||
				len(tool.Params) != 1 || tool.Params[0] != "topic" ||
				len(tool.Requires) != 1 || tool.Requires[0] != "none" {
				t.Fatalf("tool metadata = %#v", tool)
			}
			if tool.Schema != nil {
				t.Fatalf("agent output shape must not be sent as provider tool input schema: %#v", tool.Schema)
			}
			if nested := provider.requests[1]; nested.Model != "mock-specialist" || len(nested.Messages) != 2 || nested.Messages[1].Text != "Research composition" {
				t.Fatalf("nested request = %#v", nested)
			}
			for name, want := range map[string]any{
				"final_text":             "Delegated composition complete.",
				"tool_summary":           "handoff alias delegates through agent tooling",
				"tool_confidence":        0.86,
				"handoff_param":          "topic",
				"handoff_output_summary": "short finding",
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

func TestLLMAgentCompositionDelegateAliasPropagatesPending(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(llmScenarioOptions(&mockLLMProvider{}, tc.opts...)...)

			if err := vm.Exec(`
func review_config(topic) {
    return {
        model: "mock-reviewer"
        system: "Review delegated work."
        user: topic
    }, nil
}

func review_flow(topic) {
    return {
        status: "pending"
        topic: topic
        reason: "approval required"
    }, nil
}

review_agent := llm.agent("review_agent", review_config, review_flow, {params: {"topic"}})
delegate_review := llm.delegate(review_agent, {
    name: "delegate_review"
    description: "Delegate review to another agent."
})

value, dispatch_err := llm.dispatch({
    id: "call_delegate_pending_1"
    tool: "delegate_review"
    args: {topic: "approval path"}
}, {delegate_review})

value_is_nil := value == nil
err_kind := dispatch_err.kind
err_message := dispatch_err.message
pending_status := dispatch_err.pending.status
pending_topic := dispatch_err.pending.topic
delegate_param := delegate_review.params[1]
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"value_is_nil":   true,
				"err_kind":       "pending",
				"err_message":    "delegated agent paused for approval",
				"pending_status": "pending",
				"pending_topic":  "approval path",
				"delegate_param": "topic",
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
