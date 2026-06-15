package leia_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

type mockLLMProvider struct {
	last     llm.TurnRequest
	requests []llm.TurnRequest
	res      llm.TurnResult
	results  []llm.TurnResult
	err      error
}

func (p *mockLLMProvider) Turn(_ context.Context, req llm.TurnRequest) (llm.TurnResult, error) {
	p.last = req
	p.requests = append(p.requests, req)
	if p.err != nil {
		return llm.TurnResult{}, p.err
	}
	if len(p.results) > 0 {
		res := p.results[0]
		p.results = p.results[1:]
		return res, nil
	}
	if p.res.Status != "" || p.res.Text != "" || len(p.res.Calls) > 0 {
		return p.res, nil
	}
	return llm.TurnResult{Status: "final_answer", Text: "ok"}, nil
}

func TestLLMTurnWithMockProvider(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{
				Status: "tool_calls",
				Calls: []llm.ToolCall{{
					ID:   "call_1",
					Tool: "lookup",
					Args: map[string]any{"query": "leia"},
				}},
				Usage: llm.TurnUsage{InputTokens: 3, OutputTokens: 4},
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)
			err := vm.Exec(`
lookup := llm.tool("lookup", func(query) {
    return query, nil
}, {description: "lookup docs", params: ["query"]})
fail := llm.tool("fail", func(query) {
    return nil, {kind: "validation", message: "raw failure for " .. query}
}, {description: "fail docs", params: ["query"]})
tools := [lookup]
fail_tools := [fail]
value := nil
dispatch_err := nil
result, err := llm.turn({
    model: "mock-fast",
    messages: [llm.system("Be concise."), llm.user("search leia")],
    tools: tools,
    max_tokens: 64,
})
value, dispatch_err = llm.dispatch(result.calls[1], tools)
ok_outcome := llm.tool_outcome(result.calls[1], value, {
    workflow_run_id: "wf-tool"
    workflow_step_id: "dispatch-ok"
    component: "runtime-test"
})
ok_event := llm.tool_outcome_event(ok_outcome, {
    trace_id: "trace-tool"
    sequence: 1
})
fail_call := {id: "call_fail", tool: "fail", args: {query: "secret-query"}}
fail_value, fail_err := llm.dispatch(fail_call, fail_tools)
fail_outcome := llm.tool_outcome(fail_call, fail_err, {
    workflow_run_id: "wf-tool"
    workflow_step_id: "dispatch-fail"
})
fail_event := llm.tool_outcome_trace_event(fail_outcome, {
    trace_id: "trace-tool"
    sequence: 2
})
tool_trace := llm.trace_envelope({ok_event, fail_event}, {trace_id: "trace-tool"})
tool_gate := llm.trace_assert(tool_trace, {
    required_event_types: {"tool_outcome"}
    require_event_payload_fields: {tool_outcome: {"tool_call_id", "tool_name", "status", "result_status"}}
    require_correlation_fields: {"workflow_run_id", "workflow_step_id", "tool_call_id", "correlation_id"}
    max_status_counts: {ok: 1, error: 1}
    deny_raw_prompt_stored: true
    deny_raw_completion_stored: true
})
ok_outcome_kind := ok_outcome.kind
ok_outcome_schema := ok_outcome.schema_version
ok_outcome_status := ok_outcome.status
ok_outcome_result_status := ok_outcome.result_status
ok_outcome_tool := ok_outcome.tool_name
ok_outcome_call_id := ok_outcome.tool_call_id
ok_outcome_result_type := ok_outcome.result_type
ok_outcome_result_ref := ok_outcome.result_ref
ok_outcome_arg_name := ok_outcome.arg_names[1]
ok_outcome_args_redacted := ok_outcome.redaction.args_redacted
ok_outcome_raw_args_stored := ok_outcome.redaction.raw_args_stored
ok_event_type := ok_event.event_type
ok_event_payload_value_missing := ok_event.payload.value == nil
ok_event_payload_args_missing := ok_event.payload.args == nil
ok_event_redaction_policy := ok_event.redaction.policy
fail_outcome_status := fail_outcome.status
fail_outcome_result_status := fail_outcome.result_status
fail_outcome_ok := fail_outcome.ok
fail_outcome_error_kind := fail_outcome.error_kind
fail_outcome_message := fail_outcome.message
fail_event_status := fail_event.status
fail_event_payload_error_message_missing := fail_event.payload.error_message == nil
tool_gate_ok := tool_gate.ok
tool_gate_status := tool_gate.status
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if provider.last.Model != "mock-fast" {
				t.Fatalf("model = %q", provider.last.Model)
			}
			if len(provider.last.Messages) != 2 || provider.last.Messages[0].Role != "system" || provider.last.Messages[1].Text != "search leia" {
				t.Fatalf("messages = %#v", provider.last.Messages)
			}
			if len(provider.last.Tools) != 1 || provider.last.Tools[0].Name != "lookup" || provider.last.Tools[0].Description != "lookup docs" {
				t.Fatalf("tools = %#v", provider.last.Tools)
			}
			got, err := vm.Get("value")
			if err != nil {
				t.Fatalf("Get value: %v", err)
			}
			if got != "leia" {
				rawDispatchErr, _ := vm.Get("dispatch_err")
				t.Fatalf("value = %#v dispatch_err=%#v", got, rawDispatchErr)
			}
			dispatchErr, err := vm.Get("dispatch_err")
			if err != nil {
				t.Fatalf("Get dispatch_err: %v", err)
			}
			if dispatchErr != nil {
				t.Fatalf("dispatch_err = %#v", dispatchErr)
			}
			for name, want := range map[string]any{
				"ok_outcome_kind":                          "tool_outcome",
				"ok_outcome_schema":                        int64(1),
				"ok_outcome_status":                        "ok",
				"ok_outcome_result_status":                 "ok",
				"ok_outcome_tool":                          "lookup",
				"ok_outcome_call_id":                       "call_1",
				"ok_outcome_result_type":                   "string",
				"ok_outcome_result_ref":                    "call_1:result",
				"ok_outcome_arg_name":                      "query",
				"ok_outcome_args_redacted":                 true,
				"ok_outcome_raw_args_stored":               false,
				"ok_event_type":                            "tool_outcome",
				"ok_event_payload_value_missing":           true,
				"ok_event_payload_args_missing":            true,
				"ok_event_redaction_policy":                "tool_outcome_ref_only",
				"fail_outcome_status":                      "error",
				"fail_outcome_result_status":               "error",
				"fail_outcome_ok":                          false,
				"fail_outcome_error_kind":                  "validation",
				"fail_outcome_message":                     "tool outcome error",
				"fail_event_status":                        "error",
				"fail_event_payload_error_message_missing": true,
				"tool_gate_ok":                             true,
				"tool_gate_status":                         "ok",
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

func TestLLMTurnWithoutProviderReturnsError(t *testing.T) {
	vm := leia.New(leia.WithLibs(leia.LibString | leia.LibLLM))
	if err := vm.Exec(`
result, err := llm.turn({messages: [llm.user("hi")]})
kind := err.kind
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	got, err := vm.Get("kind")
	if err != nil {
		t.Fatalf("Get kind: %v", err)
	}
	if got != "provider" {
		t.Fatalf("kind = %#v", got)
	}
}

func TestLLMMessageHelpers(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{
				Status: "final_answer",
				Text:   "ok",
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)
			if err := vm.Exec(`
call := {id: "call_1", tool: "lookup", args: {name: "leia"}}
messages := [
    msg.system("system text"),
    msg.user("user text"),
    msg.assistant("assistant text"),
    msg.assistant_call(call),
    msg.tool_result("call_1", "docs"),
    msg.tool_error("call_2", "missing"),
]
result, err := llm.turn({messages: messages})
roles := messages[1].role .. "," .. messages[4].role .. "," .. messages[6].role
tool_id := messages[5].tool_use_id
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.last.Messages) != 6 {
				t.Fatalf("provider messages = %#v", provider.last.Messages)
			}
			if provider.last.Messages[3].ToolCall == nil || provider.last.Messages[3].ToolCall.Tool != "lookup" {
				t.Fatalf("assistant call = %#v", provider.last.Messages[3])
			}
			if provider.last.Messages[4].Value != "docs" || provider.last.Messages[5].Error != "missing" {
				t.Fatalf("tool messages = %#v", provider.last.Messages)
			}
			roles, _ := vm.Get("roles")
			toolID, _ := vm.Get("tool_id")
			if roles != "system,assistant,tool" || toolID != "call_1" {
				t.Fatalf("roles=%#v tool_id=%#v", roles, toolID)
			}
		})
	}
}

func TestChatHelpers(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, tc.opts...)...)
			if err := vm.Exec(`
history := [
    msg.system("You are concise."),
    msg.user("one two three four"),
    msg.assistant("five six"),
]
more := [msg.user("seven eight")]
merged := chat.merge(history, more)
windowed := chat.window(merged, 4)
tokens := chat.token_count(merged)
summary := chat.summarize(merged, {max_chars: 32})
merged_len := #merged
window_len := #windowed
summary_role := summary.role
summary_text := summary.text
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			mergedLen, _ := vm.Get("merged_len")
			windowLen, _ := vm.Get("window_len")
			tokens, _ := vm.Get("tokens")
			summaryRole, _ := vm.Get("summary_role")
			summaryText, _ := vm.Get("summary_text")
			if mergedLen != int64(4) || windowLen != int64(1) {
				t.Fatalf("merged_len=%#v window_len=%#v", mergedLen, windowLen)
			}
			if tokens.(int64) <= 0 {
				t.Fatalf("tokens = %#v", tokens)
			}
			if summaryRole != "system" || !strings.Contains(fmt.Sprint(summaryText), "...") {
				t.Fatalf("summary_role=%#v summary_text=%#v", summaryRole, summaryText)
			}
		})
	}
}
