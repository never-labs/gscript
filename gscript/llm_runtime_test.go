package gscript_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

type mockLLMProvider struct {
	last     gs.LLMTurnRequest
	requests []gs.LLMTurnRequest
	res      gs.LLMTurnResult
	results  []gs.LLMTurnResult
	err      error
}

func (p *mockLLMProvider) Turn(_ context.Context, req gs.LLMTurnRequest) (gs.LLMTurnResult, error) {
	p.last = req
	p.requests = append(p.requests, req)
	if p.err != nil {
		return gs.LLMTurnResult{}, p.err
	}
	if len(p.results) > 0 {
		res := p.results[0]
		p.results = p.results[1:]
		return res, nil
	}
	if p.res.Status != "" || p.res.Text != "" || len(p.res.Calls) > 0 {
		return p.res, nil
	}
	return gs.LLMTurnResult{Status: "final_answer", Text: "ok"}, nil
}

func TestLLMTurnWithMockProvider(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: gs.LLMTurnResult{
				Status: "tool_calls",
				Calls: []gs.LLMToolCall{{
					ID:   "call_1",
					Tool: "lookup",
					Args: map[string]any{"query": "gscript"},
				}},
				Usage: gs.LLMTurnUsage{InputTokens: 3, OutputTokens: 4},
			}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
lookup := llm.tool("lookup", func(query) {
    return query, nil
}, {description: "lookup docs", params: {"query"}})
tools := {lookup}
value := nil
dispatch_err := nil
result, err := llm.turn({
    model: "mock-fast",
    messages: {llm.system("Be concise."), llm.user("search gscript")},
    tools: tools,
    max_tokens: 64,
})
value, dispatch_err = llm.dispatch(result.calls[1], tools)
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if provider.last.Model != "mock-fast" {
				t.Fatalf("model = %q", provider.last.Model)
			}
			if len(provider.last.Messages) != 2 || provider.last.Messages[0].Role != "system" || provider.last.Messages[1].Text != "search gscript" {
				t.Fatalf("messages = %#v", provider.last.Messages)
			}
			if len(provider.last.Tools) != 1 || provider.last.Tools[0].Name != "lookup" || provider.last.Tools[0].Description != "lookup docs" {
				t.Fatalf("tools = %#v", provider.last.Tools)
			}
			got, err := vm.Get("value")
			if err != nil {
				t.Fatalf("Get value: %v", err)
			}
			if got != "gscript" {
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
		})
	}
}

func TestLLMTurnWithoutProviderReturnsError(t *testing.T) {
	vm := gs.New(gs.WithLibs(gs.LibString | gs.LibLLM))
	if err := vm.Exec(`
result, err := llm.turn({messages: {llm.user("hi")}})
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
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: gs.LLMTurnResult{
				Status: "final_answer",
				Text:   "ok",
			}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
call := {id: "call_1", tool: "lookup", args: {name: "gscript"}}
messages := {
    msg.system("system text"),
    msg.user("user text"),
    msg.assistant("assistant text"),
    msg.assistant_call(call),
    msg.tool_result("call_1", "docs"),
    msg.tool_error("call_2", "missing"),
}
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
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := gs.New(append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, tc.opts...)...)
			if err := vm.Exec(`
history := {
    msg.system("You are concise."),
    msg.user("one two three four"),
    msg.assistant("five six"),
}
more := {msg.user("seven eight")}
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

func TestLoopHelpers(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []gs.LLMTurnResult{
				{Status: "final_answer", Text: "simple"},
				{
					Status: "tool_calls",
					Calls: []gs.LLMToolCall{{
						ID:   "call_1",
						Tool: "lookup",
						Args: map[string]any{"name": "gscript"},
					}},
				},
				{Status: "final_answer", Text: "react"},
			}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
simple, simple_err := loop.simple({system: "short", user: "hello"})
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: {"name"}})
react, react_err := loop.react({
    user: "find docs",
    tools: {lookup},
    max_steps: 3,
})
simple_text := simple.text
react_text := react.text
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			simpleText, _ := vm.Get("simple_text")
			reactText, _ := vm.Get("react_text")
			if simpleText != "simple" || reactText != "react" {
				t.Fatalf("simple=%#v react=%#v", simpleText, reactText)
			}
			if len(provider.requests) != 3 || len(provider.requests[0].Messages) != 2 || provider.requests[0].Messages[0].Role != "system" {
				t.Fatalf("requests = %#v", provider.requests)
			}
			if len(provider.requests[2].Messages) != 3 || provider.requests[2].Messages[2].Value != "docs:gscript" {
				t.Fatalf("react request = %#v", provider.requests[2])
			}
		})
	}
}

func TestLoopSnapshotResume(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := gs.New(append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, tc.opts...)...)
			if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: {"name"}})
pending := {id: "call_1", tool: "lookup", args: {name: "old"}}
token := loop.snapshot({msg.user("find docs")}, pending)
approved, approved_err := loop.resume(token, {ok: true, args: {name: "gscript"}}, {lookup})
approved_status := approved.status
approved_history_len := #approved.history
approved_value := approved.value

denied_token := loop.snapshot({msg.user("refund")}, {id: "call_2", tool: "lookup", args: {name: "x"}})
denied, denied_err := loop.resume(denied_token, {ok: false, reason: "needs approval"})
denied_status := denied.status
denied_history_len := #denied.history
missing, missing_err := loop.resume(denied_token, {ok: true})
missing_kind := missing_err.kind
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			approvedStatus, _ := vm.Get("approved_status")
			approvedHistoryLen, _ := vm.Get("approved_history_len")
			approvedValue, _ := vm.Get("approved_value")
			deniedStatus, _ := vm.Get("denied_status")
			deniedHistoryLen, _ := vm.Get("denied_history_len")
			missingKind, _ := vm.Get("missing_kind")
			if approvedStatus != "dispatched" || approvedHistoryLen != int64(3) || approvedValue != "docs:gscript" {
				t.Fatalf("approved status=%#v history=%#v value=%#v", approvedStatus, approvedHistoryLen, approvedValue)
			}
			if deniedStatus != "denied" || deniedHistoryLen != int64(3) || missingKind != "validation" {
				t.Fatalf("denied status=%#v history=%#v missing=%#v", deniedStatus, deniedHistoryLen, missingKind)
			}
		})
	}
}

func TestLoopSnapshotStore(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := gs.New(append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, tc.opts...)...)
			if err := vm.Exec(`
saved := nil
saved_token := ""
loaded_token := ""
deleted_token := ""
store := {
    save: func(token, snapshot) {
        saved_token = token
        saved = snapshot
        return true, nil
    },
    load: func(token) {
        loaded_token = token
        return saved, nil
    },
    delete: func(token) {
        deleted_token = token
        return true, nil
    },
}
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: {"name"}})
token, snap_err := loop.snapshot({msg.user("find docs")}, {id: "call_1", tool: "lookup", args: {name: "gscript"}}, store)
stored_name := saved.pending.args.name
loaded, loaded_err := loop.resume("external-token", {ok: true}, {lookup}, store)
loaded_status := loaded.status
loaded_value := loaded.value
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]interface{}{
				"stored_name":   "gscript",
				"loaded_status": "dispatched",
				"loaded_value":  "docs:gscript",
				"loaded_token":  "external-token",
				"deleted_token": "external-token",
			} {
				got, _ := vm.Get(name)
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
			token, _ := vm.Get("token")
			savedToken, _ := vm.Get("saved_token")
			if token == "" || token != savedToken {
				t.Fatalf("token=%#v saved_token=%#v", token, savedToken)
			}
		})
	}
}

func TestLoopSnapshotTraceEvents(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []gs.LLMTraceEvent
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMTrace(func(event gs.LLMTraceEvent) {
					events = append(events, event)
				}),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
saved := nil
store := {
    save: func(token, snapshot) {
        saved = snapshot
        return true, nil
    },
    load: func(token) {
        return saved, nil
    },
    delete: func(token) {
        return true, nil
    },
}
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: {"name"}})
token, snap_err := loop.snapshot({msg.user("find docs")}, {id: "call_1", tool: "lookup", args: {name: "gscript"}}, store)
loaded, loaded_err := loop.resume(token, {ok: true}, {lookup}, store)
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			got := make([]string, 0, len(events))
			for _, event := range events {
				got = append(got, event.Type)
			}
			want := []string{
				"snapshot_saved",
				"snapshot_store_saved",
				"resume_start",
				"resume_loaded",
				"resume_store_deleted",
				"resume_done",
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("events = %#v, want %#v", got, want)
			}
			if events[0].Token == "" || events[1].Token != events[0].Token || !events[1].Store {
				t.Fatalf("snapshot events = %#v", events[:2])
			}
			if events[5].Status != "dispatched" || events[5].Token != events[0].Token || !events[5].Store {
				t.Fatalf("resume done = %#v", events[5])
			}
		})
	}
}

func TestLoopReactApproveWhenPauses(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: gs.LLMTurnResult{
				Status: "tool_calls",
				Calls: []gs.LLMToolCall{{
					ID:   "call_1",
					Tool: "refund",
					Args: map[string]any{"amount": int64(150)},
				}},
			}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
refund := llm.tool("refund", func(amount) {
    return "refund:" .. amount, nil
}, {params: {"amount"}})
result, err := loop.react({
    user: "refund order",
    tools: {refund},
    approve_when: func(call) {
        return call.tool == "refund" && call.args.amount > 100
    },
})
pending_status := result.status
pending_tool := result.payload.tool
pending_amount := result.payload.args.amount
resume, resume_err := loop.resume(result.token, {ok: true}, {refund})
resume_status := resume.status
resume_value := resume.value
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			pendingStatus, _ := vm.Get("pending_status")
			pendingTool, _ := vm.Get("pending_tool")
			pendingAmount, _ := vm.Get("pending_amount")
			resumeStatus, _ := vm.Get("resume_status")
			resumeValue, _ := vm.Get("resume_value")
			if pendingStatus != "pending" || pendingTool != "refund" || pendingAmount != int64(150) {
				t.Fatalf("pending status=%#v tool=%#v amount=%#v", pendingStatus, pendingTool, pendingAmount)
			}
			if resumeStatus != "dispatched" || resumeValue != "refund:150" {
				t.Fatalf("resume status=%#v value=%#v", resumeStatus, resumeValue)
			}
		})
	}
}

func TestLoopPlanExecute(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []gs.LLMTurnResult{
				{Status: "final_answer", Text: "1. lookup docs\n2. answer"},
				{
					Status: "tool_calls",
					Calls: []gs.LLMToolCall{{
						ID:   "call_1",
						Tool: "lookup",
						Args: map[string]any{"name": "gscript"},
					}},
				},
				{Status: "final_answer", Text: "done"},
			}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: {"name"}})
result, err := loop.plan_execute({
    user: "find docs",
    tools: {lookup},
    plan_model: "planner",
    exec_model: "executor",
    max_steps: 3,
})
status := result.status
text := result.text
plan := result.plan
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			status, _ := vm.Get("status")
			text, _ := vm.Get("text")
			plan, _ := vm.Get("plan")
			if status != "done" || text != "done" || plan != "1. lookup docs\n2. answer" {
				t.Fatalf("status=%#v text=%#v plan=%#v", status, text, plan)
			}
			if len(provider.requests) != 3 {
				t.Fatalf("requests = %#v", provider.requests)
			}
			if provider.requests[0].Model != "planner" || provider.requests[1].Model != "executor" {
				t.Fatalf("models = %#v / %#v", provider.requests[0].Model, provider.requests[1].Model)
			}
			if len(provider.requests[1].Messages) < 2 || !strings.Contains(provider.requests[1].Messages[0].Text, "lookup docs") {
				t.Fatalf("execute messages = %#v", provider.requests[1].Messages)
			}
			if len(provider.requests[2].Messages) < 4 || provider.requests[2].Messages[3].Value != "docs:gscript" {
				t.Fatalf("final messages = %#v", provider.requests[2].Messages)
			}
		})
	}
}

func TestLoopReflect(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []gs.LLMTurnResult{
				{Status: "final_answer", Text: "draft"},
				{Status: "final_answer", Text: "refined"},
			}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
result, err := loop.reflect({
    user: "answer",
    model: "writer",
    reflect_model: "critic",
    max_iters: 1,
})
status := result.status
text := result.text
reflection_text := result.reflection[1].text
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			status, _ := vm.Get("status")
			text, _ := vm.Get("text")
			reflectionText, _ := vm.Get("reflection_text")
			if status != "done" || text != "refined" || reflectionText != "refined" {
				t.Fatalf("status=%#v text=%#v reflection=%#v", status, text, reflectionText)
			}
			if len(provider.requests) != 2 {
				t.Fatalf("requests = %#v", provider.requests)
			}
			if provider.requests[0].Model != "writer" || provider.requests[1].Model != "critic" {
				t.Fatalf("models = %#v / %#v", provider.requests[0].Model, provider.requests[1].Model)
			}
			if len(provider.requests[1].Messages) != 2 || provider.requests[1].Messages[1].Text != "draft" {
				t.Fatalf("reflection messages = %#v", provider.requests[1].Messages)
			}
		})
	}
}

func TestLLMReactDispatchLoop(t *testing.T) {
	provider := &mockLLMProvider{results: []gs.LLMTurnResult{
		{
			Status: "tool_calls",
			Calls: []gs.LLMToolCall{{
				ID:   "call_1",
				Tool: "lookup",
				Args: map[string]any{"name": "gscript"},
			}},
		},
		{Status: "final_answer", Text: "done"},
	}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(provider))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: {"name"}})
result, err := llm.react({
    messages: {llm.user("find docs")},
    tools: {lookup},
    max_steps: 3,
})
status := result.status
text := result.text
history_len := #result.history
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(provider.requests))
	}
	if len(provider.requests[1].Messages) != 3 || provider.requests[1].Messages[1].ToolCall == nil || provider.requests[1].Messages[2].Value != "docs:gscript" {
		t.Fatalf("second request messages = %#v", provider.requests[1].Messages)
	}
	status, _ := vm.Get("status")
	text, _ := vm.Get("text")
	historyLen, _ := vm.Get("history_len")
	if status != "done" || text != "done" || historyLen != int64(3) {
		t.Fatalf("status=%#v text=%#v history_len=%#v", status, text, historyLen)
	}
}

func TestLLMReactCanWindowHistory(t *testing.T) {
	provider := &mockLLMProvider{results: []gs.LLMTurnResult{
		{
			Status: "tool_calls",
			Calls: []gs.LLMToolCall{{
				ID:   "call_1",
				Tool: "lookup",
				Args: map[string]any{"name": "gscript"},
			}},
		},
		{Status: "final_answer", Text: "done"},
	}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(provider))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: {"name"}})
result, err := llm.react({
    messages: {msg.system("drop this long system prompt"), msg.user("drop this long user prompt")},
    tools: {lookup},
    max_steps: 3,
    max_history_tokens: 10,
})
status := result.status
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(provider.requests))
	}
	second := provider.requests[1].Messages
	if len(second) != 2 || second[0].Role != "assistant" || second[0].ToolCall == nil || second[1].Role != "tool" || second[1].Value != "docs:gscript" {
		t.Fatalf("second request messages = %#v", second)
	}
	status, _ := vm.Get("status")
	if status != "done" {
		t.Fatalf("status = %#v", status)
	}
}

func TestLLMReactRetriesTransientToolErrors(t *testing.T) {
	provider := &mockLLMProvider{results: []gs.LLMTurnResult{
		{
			Status: "tool_calls",
			Calls: []gs.LLMToolCall{{
				ID:   "call_1",
				Tool: "lookup",
				Args: map[string]any{"name": "gscript"},
			}},
		},
		{Status: "final_answer", Text: "done"},
	}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(provider))
	if err := vm.Exec(`
attempts := 0
lookup := llm.tool("lookup", func(name) {
    attempts = attempts + 1
    if attempts == 1 {
        return nil, {kind: "network", message: "retry me"}
    }
    return "docs:" .. name, nil
}, {params: {"name"}})
result, err := llm.react({
    messages: {llm.user("find docs")},
    tools: {lookup},
    max_steps: 3,
    max_tool_retries: 1,
})
status := result.status
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	attempts, _ := vm.Get("attempts")
	status, _ := vm.Get("status")
	if attempts != int64(2) || status != "done" {
		t.Fatalf("attempts=%#v status=%#v", attempts, status)
	}
	if len(provider.requests) != 2 || len(provider.requests[1].Messages) != 3 || provider.requests[1].Messages[2].Value != "docs:gscript" {
		t.Fatalf("requests = %#v", provider.requests)
	}
}

func TestLLMReactFatalToolErrorPropagates(t *testing.T) {
	provider := &mockLLMProvider{res: gs.LLMTurnResult{
		Status: "tool_calls",
		Calls: []gs.LLMToolCall{{
			ID:   "call_1",
			Tool: "lookup",
			Args: map[string]any{"name": "gscript"},
		}},
	}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(provider))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return nil, {kind: "fatal", message: "stop"}
}, {params: {"name"}})
result, err := llm.react({
    messages: {llm.user("find docs")},
    tools: {lookup},
    max_steps: 1,
})
err_kind := err.kind
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	kind, _ := vm.Get("err_kind")
	if kind != "fatal" {
		t.Fatalf("err_kind = %#v", kind)
	}
}

func TestLLMTurnRequestProviderOptions(t *testing.T) {
	provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: "done"}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(provider))
	if err := vm.Exec(`
result, err := llm.turn({
    model: "mock-fast",
    messages: {llm.user("hello")},
    force_tool: "lookup",
    max_tokens: 16,
    temperature: 0.25,
    top_p: 0.9,
    response_format: {type: "json_object"},
    stream: true,
    stop: {"END", "\n\n"},
    metadata: {trace_id: "abc", route: "test"},
})
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if provider.last.Model != "mock-fast" || provider.last.MaxTokens != 16 || !provider.last.Stream {
		t.Fatalf("request = %#v", provider.last)
	}
	if provider.last.ForceTool != "lookup" {
		t.Fatalf("force_tool = %#v", provider.last.ForceTool)
	}
	if provider.last.Temperature == nil || *provider.last.Temperature != 0.25 {
		t.Fatalf("temperature = %#v", provider.last.Temperature)
	}
	if provider.last.TopP == nil || *provider.last.TopP != 0.9 {
		t.Fatalf("top_p = %#v", provider.last.TopP)
	}
	format, _ := provider.last.ResponseFormat.(map[string]any)
	if format["type"] != "json_object" {
		t.Fatalf("response_format = %#v", provider.last.ResponseFormat)
	}
	if len(provider.last.Stop) != 2 || provider.last.Stop[0] != "END" || provider.last.Stop[1] != "\n\n" {
		t.Fatalf("stop = %#v", provider.last.Stop)
	}
	if provider.last.Metadata["trace_id"] != "abc" || provider.last.Metadata["route"] != "test" {
		t.Fatalf("metadata = %#v", provider.last.Metadata)
	}
}

func TestLoopRequestProviderOptions(t *testing.T) {
	provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: "done"}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(provider))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: {"name"}})
result, err := loop.react({
    user: "hello",
    model: "mock-fast",
    tools: {lookup},
    force_tool: lookup,
    max_tokens: 32,
    stream: true,
    stop: {"DONE"},
    metadata: {trace_id: "loop-1"},
})
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("requests = %d", len(provider.requests))
	}
	req := provider.requests[0]
	if req.Model != "mock-fast" || req.MaxTokens != 32 || !req.Stream {
		t.Fatalf("request = %#v", req)
	}
	if req.ForceTool != "lookup" {
		t.Fatalf("force_tool = %#v", req.ForceTool)
	}
	if len(req.Stop) != 1 || req.Stop[0] != "DONE" {
		t.Fatalf("stop = %#v", req.Stop)
	}
	if req.Metadata["trace_id"] != "loop-1" {
		t.Fatalf("metadata = %#v", req.Metadata)
	}
}
