package gscript_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gs "github.com/never-labs/gscript/gscript"
	"github.com/never-labs/gscript/internal/runtime"
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

func TestLoopBudgets(t *testing.T) {
	provider := &mockLLMProvider{results: []gs.LLMTurnResult{
		{
			Status: "tool_calls",
			Calls: []gs.LLMToolCall{{
				ID:   "call_1",
				Tool: "lookup",
				Args: map[string]any{"name": "gscript"},
			}},
			Usage: gs.LLMTurnUsage{InputTokens: 2, OutputTokens: 3},
		},
		{Status: "final_answer", Text: "done"},
	}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(provider))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: {"name"}})
result, err := loop.react({
    user: "find docs",
    tools: {lookup},
    max_steps: 3,
    budget: {tokens: 5, turns: 3, calls: 2},
})
err_kind := err.kind
err_dimension := err.dimension

`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	kind, _ := vm.Get("err_kind")
	dimension, _ := vm.Get("err_dimension")
	if kind != "budget" || dimension != "tokens" {
		t.Fatalf("err kind=%#v dimension=%#v", kind, dimension)
	}
}

func TestLoopToolCallBudget(t *testing.T) {
	provider := &mockLLMProvider{res: gs.LLMTurnResult{
		Status: "tool_calls",
		Calls: []gs.LLMToolCall{
			{ID: "call_1", Tool: "lookup", Args: map[string]any{"name": "a"}},
			{ID: "call_2", Tool: "lookup", Args: map[string]any{"name": "b"}},
		},
	}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(provider))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: {"name"}})
result, err := loop.react({
    user: "find docs",
    tools: {lookup},
    max_steps: 2,
    budget: {calls: 0},
})
err_kind := err.kind
err_dimension := err.dimension
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	kind, _ := vm.Get("err_kind")
	dimension, _ := vm.Get("err_dimension")
	if kind != "budget" || dimension != "calls" {
		t.Fatalf("err kind=%#v dimension=%#v", kind, dimension)
	}
}

func TestLoopTimeBudget(t *testing.T) {
	provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: "done"}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(provider))
	if err := vm.Exec(`
result, err := loop.react({
    user: "find docs",
    budget: {time: 0},
})
err_kind := err.kind
err_message := err.message
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	kind, _ := vm.Get("err_kind")
	message, _ := vm.Get("err_message")
	if kind != "deadline" || message != "deadline exceeded" {
		t.Fatalf("err kind=%#v message=%#v", kind, message)
	}
	if len(provider.requests) != 0 {
		t.Fatalf("provider requests = %d, want 0", len(provider.requests))
	}
}

func TestLoopScriptContextCancellation(t *testing.T) {
	provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: "done"}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(provider))
	if err := vm.Exec(`
ctx, cancel := context.withCancel()
cancel()
result, err := loop.react({
    user: "find docs",
    ctx: ctx,
})
err_kind := err.kind
err_message := err.message
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	kind, _ := vm.Get("err_kind")
	message, _ := vm.Get("err_message")
	if kind != "cancelled" || message != "cancelled" {
		t.Fatalf("err kind=%#v message=%#v", kind, message)
	}
	if len(provider.requests) != 0 {
		t.Fatalf("provider requests = %d, want 0", len(provider.requests))
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

func TestLLMTraceSinkReceivesTurnAndReactEvents(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []gs.LLMTurnResult{
				{Status: "final_answer", Text: "turn", Usage: gs.LLMTurnUsage{InputTokens: 1, OutputTokens: 2}},
				{
					Status: "tool_calls",
					Calls: []gs.LLMToolCall{{
						ID:   "call_1",
						Tool: "lookup",
						Args: map[string]any{"name": "gscript"},
					}},
				},
				{Status: "final_answer", Text: "done", Usage: gs.LLMTurnUsage{InputTokens: 3, OutputTokens: 4}},
			}}
			var events []gs.LLMTraceEvent
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
				gs.WithLLMTrace(func(event gs.LLMTraceEvent) {
					events = append(events, event)
				}),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
turn_result, turn_err := llm.turn({
    model: "mock-fast",
    messages: {llm.user("hello")},
})
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: {"name"}})
react_result, react_err := llm.react({
    model: "mock-fast",
    messages: {llm.user("find docs")},
    tools: {lookup},
    max_steps: 3,
})
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			got := make([]string, 0, len(events))
			for _, event := range events {
				got = append(got, event.Type)
			}
			want := []string{
				"turn_start", "turn_end",
				"turn_start", "turn_end", "tool_call", "tool_result",
				"turn_start", "turn_end", "react_done",
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("events = %#v, want %#v", got, want)
			}
			if events[0].MessageCount != 1 || events[0].ToolCount != 0 || events[1].Usage.OutputTokens != 2 {
				t.Fatalf("turn metadata = %#v %#v", events[0], events[1])
			}
			if events[4].Tool != "lookup" || events[4].CallID != "call_1" || events[6].Step != 1 || events[7].Usage.OutputTokens != 4 {
				t.Fatalf("react metadata = %#v", events)
			}
		})
	}
}

func TestLLMTraceRecorderHelper(t *testing.T) {
	provider := &mockLLMProvider{res: gs.LLMTurnResult{
		Status: "final_answer",
		Text:   "done",
		Usage:  gs.LLMTurnUsage{InputTokens: 1, OutputTokens: 2},
	}}
	recorder := gs.NewLLMTraceRecorder(gs.LLMTraceEvent{Type: "seed"})
	vm := gs.New(
		gs.WithLibs(gs.LibString|gs.LibLLM),
		gs.WithLLMProvider(provider),
		gs.WithLLMTrace(recorder.Record),
	)
	if err := vm.Exec(`
result, err := llm.turn({
    model: "mock-fast",
    messages: {llm.user("hello")},
})
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	events := recorder.Events()
	if len(events) != 3 || events[0].Type != "seed" || events[1].Type != "turn_start" || events[2].Usage.OutputTokens != 2 {
		t.Fatalf("events = %#v", events)
	}
	events[0].Type = "mutated"
	if recorder.Events()[0].Type != "seed" {
		t.Fatalf("Events returned mutable internal state")
	}
	recorder.Reset()
	if got := recorder.Events(); len(got) != 0 {
		t.Fatalf("after Reset events = %#v", got)
	}
}

func TestLLMRecorderAndReplay(t *testing.T) {
	provider := &mockLLMProvider{res: gs.LLMTurnResult{
		Status: "final_answer",
		Text:   "recorded",
		Usage:  gs.LLMTurnUsage{InputTokens: 5, OutputTokens: 6},
	}}
	var records []gs.LLMRecord
	vm := gs.New(
		gs.WithLibs(gs.LibString|gs.LibLLM),
		gs.WithLLMProvider(provider),
		gs.WithLLMRecorder(func(record gs.LLMRecord) {
			records = append(records, record)
		}),
	)
	if err := vm.Exec(`
result, err := llm.turn({
    model: "mock-fast",
    messages: {llm.system("short"), llm.user("hello")},
    max_tokens: 16,
})
text := result.text
`); err != nil {
		t.Fatalf("record Exec: %v", err)
	}
	if len(records) != 1 || records[0].Request.Model != "mock-fast" || records[0].Result.Text != "recorded" {
		t.Fatalf("records = %#v", records)
	}
	replay := gs.New(
		gs.WithLibs(gs.LibString|gs.LibLLM),
		gs.WithLLMReplay(records),
	)
	if err := replay.Exec(`
result, err := llm.turn({
    model: "mock-fast",
    messages: {llm.system("short"), llm.user("hello")},
    max_tokens: 16,
})
text := result.text
usage := result.usage.output_tokens
`); err != nil {
		t.Fatalf("replay Exec: %v", err)
	}
	text, _ := replay.Get("text")
	usage, _ := replay.Get("usage")
	if text != "recorded" || usage != int64(6) {
		t.Fatalf("replay text=%#v usage=%#v", text, usage)
	}
}

func TestLLMRecorderHelper(t *testing.T) {
	provider := &mockLLMProvider{res: gs.LLMTurnResult{
		Status: "final_answer",
		Text:   "recorded",
	}}
	recorder := gs.NewLLMRecorder()
	vm := gs.New(
		gs.WithLibs(gs.LibString|gs.LibLLM),
		gs.WithLLMProvider(provider),
		gs.WithLLMRecorder(recorder.Record),
	)
	if err := vm.Exec(`
result, err := llm.turn({
    model: "mock-fast",
    messages: {llm.user("hello")},
})
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	records := recorder.Records()
	if len(records) != 1 || records[0].Result.Text != "recorded" {
		t.Fatalf("records = %#v", records)
	}
	records[0].Result.Text = "mutated"
	if recorder.Records()[0].Result.Text != "recorded" {
		t.Fatalf("Records returned mutable internal state")
	}
	path := filepath.Join(t.TempDir(), "records.json")
	if err := recorder.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := gs.LoadLLMRecorder(path)
	if err != nil {
		t.Fatalf("LoadLLMRecorder: %v", err)
	}
	replay := gs.NewLLMReplayProvider(loaded.Records())
	res, err := replay.Turn(context.Background(), recorder.Records()[0].Request)
	if err != nil || res.Text != "recorded" {
		t.Fatalf("Turn res=%#v err=%v", res, err)
	}
	recorder.Reset()
	if got := recorder.Records(); len(got) != 0 {
		t.Fatalf("after Reset records = %#v", got)
	}
}

func TestLLMReplayRejectsMismatchedRequest(t *testing.T) {
	records := []gs.LLMRecord{{
		Request: gs.LLMTurnRequest{
			Model:    "mock-fast",
			Messages: []gs.LLMMessage{{Role: "user", Text: "expected"}},
		},
		Result: gs.LLMTurnResult{Status: "final_answer", Text: "ok"},
	}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMReplay(records))
	if err := vm.Exec(`
result, err := llm.turn({
    model: "mock-fast",
    messages: {llm.user("actual")},
})
kind := err.kind
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	kind, _ := vm.Get("kind")
	if kind != "provider" {
		t.Fatalf("kind = %#v", kind)
	}
}

func TestLLMReplayTypedErrors(t *testing.T) {
	replay := gs.NewLLMReplayProvider([]gs.LLMRecord{{
		Request: gs.LLMTurnRequest{
			Model:    "mock-fast",
			Messages: []gs.LLMMessage{{Role: "user", Text: "expected"}},
		},
		Result: gs.LLMTurnResult{Status: "final_answer", Text: "ok"},
	}})
	_, err := replay.Turn(context.Background(), gs.LLMTurnRequest{
		Model:    "mock-fast",
		Messages: []gs.LLMMessage{{Role: "user", Text: "actual"}},
	})
	var mismatch *gs.LLMReplayMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("err = %T %v, want LLMReplayMismatchError", err, err)
	}
	if mismatch.Turn != 0 || mismatch.Expected.Messages[0].Text != "expected" || mismatch.Actual.Messages[0].Text != "actual" {
		t.Fatalf("mismatch = %#v", mismatch)
	}
	mismatch.Expected.Messages[0].Text = "mutated"
	if replay.Remaining() != 0 {
		t.Fatalf("remaining = %d, want 0", replay.Remaining())
	}

	empty := gs.NewLLMReplayProvider(nil)
	_, err = empty.Turn(context.Background(), gs.LLMTurnRequest{})
	var exhausted *gs.LLMReplayExhaustedError
	if !errors.As(err, &exhausted) || exhausted.Turn != 0 {
		t.Fatalf("err = %T %v, exhausted=%#v", err, err, exhausted)
	}
}

func TestLLMReplayProviderStateHelpers(t *testing.T) {
	record := gs.LLMRecord{
		Request: gs.LLMTurnRequest{
			Model:    "mock-fast",
			Messages: []gs.LLMMessage{{Role: "user", Text: "hello"}},
		},
		Result: gs.LLMTurnResult{Status: "final_answer", Text: "ok"},
	}
	replay := gs.NewLLMReplayProvider([]gs.LLMRecord{record})
	if replay.Consumed() != 0 || replay.Remaining() != 1 {
		t.Fatalf("initial consumed=%d remaining=%d", replay.Consumed(), replay.Remaining())
	}
	records := replay.Records()
	records[0].Request.Messages[0].Text = "mutated"
	if replay.Records()[0].Request.Messages[0].Text != "hello" {
		t.Fatalf("Records returned mutable internal state")
	}
	res, err := replay.Turn(context.Background(), record.Request)
	if err != nil || res.Text != "ok" {
		t.Fatalf("Turn res=%#v err=%v", res, err)
	}
	if replay.Consumed() != 1 || replay.Remaining() != 0 {
		t.Fatalf("after turn consumed=%d remaining=%d", replay.Consumed(), replay.Remaining())
	}
	replay.Reset()
	if replay.Consumed() != 0 || replay.Remaining() != 1 {
		t.Fatalf("after reset consumed=%d remaining=%d", replay.Consumed(), replay.Remaining())
	}
	res, err = replay.Turn(context.Background(), record.Request)
	if err != nil || res.Text != "ok" {
		t.Fatalf("Turn after reset res=%#v err=%v", res, err)
	}
}

func TestLLMRecordJSONRoundTrip(t *testing.T) {
	records := []gs.LLMRecord{{
		Request: gs.LLMTurnRequest{
			Model: "mock-fast",
			Messages: []gs.LLMMessage{{
				Role: "user",
				Text: "hello",
				Value: map[string]any{
					"count": int64(3),
					"tags":  []any{"a", int64(2)},
				},
			}},
			Tools: []gs.LLMTool{{
				Name:     "lookup",
				Params:   []string{"name"},
				Requires: []string{"docs.read"},
				Schema: map[string]any{
					"type":  "object",
					"limit": int64(3),
				},
			}},
			Metadata: map[string]string{"trace_id": "abc"},
		},
		Result: gs.LLMTurnResult{
			Status: "tool_calls",
			Calls: []gs.LLMToolCall{{
				ID:   "call_1",
				Tool: "lookup",
				Args: map[string]any{"name": "gscript", "limit": int64(3)},
			}},
		},
	}}
	data, err := gs.MarshalLLMRecords(records)
	if err != nil {
		t.Fatalf("MarshalLLMRecords: %v", err)
	}
	decoded, err := gs.UnmarshalLLMRecords(data)
	if err != nil {
		t.Fatalf("UnmarshalLLMRecords: %v", err)
	}
	replay := gs.NewLLMReplayProvider(decoded)
	res, err := replay.Turn(context.Background(), records[0].Request)
	if err != nil {
		t.Fatalf("Turn: %v", err)
	}
	if got := res.Calls[0].Args["limit"]; got != int64(3) {
		t.Fatalf("limit = %#v (%T), want int64(3)", got, got)
	}
}

func TestLLMRecordFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.json")
	records := []gs.LLMRecord{{
		Request: gs.LLMTurnRequest{
			Model:    "mock-fast",
			Messages: []gs.LLMMessage{{Role: "user", Text: "hello"}},
		},
		Result: gs.LLMTurnResult{Status: "final_answer", Text: "ok"},
	}}
	if err := gs.SaveLLMRecords(path, records); err != nil {
		t.Fatalf("SaveLLMRecords: %v", err)
	}
	decoded, err := gs.LoadLLMRecords(path)
	if err != nil {
		t.Fatalf("LoadLLMRecords: %v", err)
	}
	replay := gs.NewLLMReplayProvider(decoded)
	res, err := replay.Turn(context.Background(), records[0].Request)
	if err != nil || res.Text != "ok" {
		t.Fatalf("Turn res=%#v err=%v", res, err)
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

func TestLLMToolMetadata(t *testing.T) {
	provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: "done"}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(provider))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {
    description: "lookup docs",
    params: {"name"},
    requires: {"docs.read", "net.client"},
    schema: {
        type: "object",
        properties: {name: {type: "string"}},
        required: {"name"},
    },
})
result, err := llm.turn({
    messages: {llm.user("hello")},
    tools: {lookup},
})
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(provider.last.Tools) != 1 {
		t.Fatalf("tools = %#v", provider.last.Tools)
	}
	tool := provider.last.Tools[0]
	if tool.Name != "lookup" || tool.Description != "lookup docs" {
		t.Fatalf("tool = %#v", tool)
	}
	if len(tool.Requires) != 2 || tool.Requires[0] != "docs.read" || tool.Requires[1] != "net.client" {
		t.Fatalf("requires = %#v", tool.Requires)
	}
	schema, ok := tool.Schema.(map[string]any)
	if !ok || schema["type"] != "object" {
		t.Fatalf("schema = %#v", tool.Schema)
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v", schema["properties"])
	}
	name, ok := props["name"].(map[string]any)
	if !ok || name["type"] != "string" {
		t.Fatalf("name schema = %#v", props["name"])
	}
}

func TestLLMToolCapabilities(t *testing.T) {
	vm := gs.New(gs.WithLibs(gs.LibString | gs.LibLLM))
	if err := vm.Exec(`
read_docs := llm.tool("read_docs", func(name) {
    return "docs:" .. name, nil
}, {requires: {"docs.read", "net.client"}})
refund := llm.tool("refund", func(id) {
    return id, nil
}, {requires: {"payments.refund"}})
tools := {read_docs, refund}
caps := llm.tool_caps(tools)
ok, ok_err := llm.check_tools(tools, {"docs.read", "net.client", "payments.refund"})
missing, missing_err := llm.check_tools(tools, {"docs.read", "net.client"})
all_ok, all_err := llm.check_tools(tools, {"cap.all"})
cap1 := caps[1]
cap2 := caps[2]
cap3 := caps[3]
missing_kind := missing_err.kind
missing_cap := missing_err.capability
missing_tool := missing_err.tool
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	for name, want := range map[string]interface{}{
		"cap1":         "docs.read",
		"cap2":         "net.client",
		"cap3":         "payments.refund",
		"ok":           true,
		"ok_err":       nil,
		"missing":      nil,
		"missing_kind": "capability",
		"missing_cap":  "payments.refund",
		"missing_tool": "refund",
		"all_ok":       true,
		"all_err":      nil,
	} {
		got, _ := vm.Get(name)
		if got != want {
			t.Fatalf("%s = %#v, want %#v", name, got, want)
		}
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

// --- Basic VM tests ---

func TestExec(t *testing.T) {
	var output []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))
	err := vm.Exec(`print("hello", "world")`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 1 || output[0] != "hello\tworld" {
		t.Fatalf("expected 'hello\\tworld', got %v", output)
	}
}

func TestCompileRunProgram(t *testing.T) {
	prog, err := gs.Compile(`result := 40 + 2`, gs.WithSourceName("calc.gs"))
	if err != nil {
		t.Fatal(err)
	}
	if prog.SourceName() != "calc.gs" {
		t.Fatalf("SourceName = %q, want calc.gs", prog.SourceName())
	}
	vm := gs.New()
	if err := vm.Run(prog); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(42) {
		t.Fatalf("result = %v (%T), want int64(42)", got, got)
	}
}

func TestCompileRunProgramWithVM(t *testing.T) {
	prog, err := gs.Compile(`func add(a, b) { return a + b }`)
	if err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithVM())
	if err := vm.Run(prog); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Call("add", 20, 22)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != int64(42) {
		t.Fatalf("add result = %v, want [42]", got)
	}
}

func TestCompileFileSetsSourceAndRequireDir(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.gs")
	helperPath := filepath.Join(dir, "helper.gs")
	if err := os.WriteFile(helperPath, []byte(`return { value: 42 }`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mainPath, []byte(`helper := require("helper"); result := helper.value`), 0644); err != nil {
		t.Fatal(err)
	}
	prog, err := gs.CompileFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	if prog.SourceName() != mainPath {
		t.Fatalf("SourceName = %q, want %q", prog.SourceName(), mainPath)
	}
	vm := gs.New()
	if err := vm.Run(prog); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(42) {
		t.Fatalf("result = %v (%T), want int64(42)", got, got)
	}
}

func TestContextEntrypointsRespectCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	vm := gs.New()
	if err := vm.ExecContext(ctx, `x := 1`); !errors.Is(err, context.Canceled) {
		t.Fatalf("ExecContext err = %v, want context.Canceled", err)
	}
	if _, err := gs.CompileContext(ctx, `x := 1`); !errors.Is(err, context.Canceled) {
		t.Fatalf("CompileContext err = %v, want context.Canceled", err)
	}
	if _, err := vm.CallContext(ctx, "missing"); !errors.Is(err, context.Canceled) {
		t.Fatalf("CallContext err = %v, want context.Canceled", err)
	}
}

func TestExecContextCancelsInterpreterLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	vm := gs.New()
	err := vm.ExecContext(ctx, `for {}`)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ExecContext err = %v, want context deadline", err)
	}
}

func TestExecContextCancelsBytecodeLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	vm := gs.New(gs.WithVM())
	err := vm.ExecContext(ctx, `for {}`)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ExecContext err = %v, want context deadline", err)
	}
}

func TestCallContextCancelsRunningFunction(t *testing.T) {
	vm := gs.New()
	if err := vm.Exec(`func spin() { for {} }`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := vm.CallContext(ctx, "spin")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("CallContext err = %v, want context deadline", err)
	}
}

func TestWithMaxStepsLimitsInterpreterExecution(t *testing.T) {
	vm := gs.New(gs.WithMaxSteps(8))
	err := vm.Exec(`
		i := 0
		for {
			i += 1
		}
	`)
	if err == nil {
		t.Fatal("expected max step error")
	}
	if !strings.Contains(err.Error(), "execution step limit exceeded") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithMaxStepsLimitsEmptyInterpreterLoop(t *testing.T) {
	vm := gs.New(gs.WithMaxSteps(8))
	err := vm.Exec(`for {}`)
	if err == nil {
		t.Fatal("expected step limit error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "steps" || budgetErr.Limit != 8 {
		t.Fatalf("budget = %s %d, want steps 8", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxStepsAllowsInterpreterExecutionWithinBudget(t *testing.T) {
	vm := gs.New(gs.WithMaxSteps(64))
	if err := vm.Exec(`
		sum := 0
		for i := 0; i < 5; i++ {
			sum += i
		}
	`); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("sum")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(10) {
		t.Fatalf("sum = %v (%T), want int64(10)", got, got)
	}
}

func TestWithMaxNativeCallsLimitsInterpreterHostCalls(t *testing.T) {
	vm := gs.New(gs.WithMaxNativeCalls(3))
	var calls int64
	if err := vm.RegisterFunc("tick", func() int64 {
		calls++
		return calls
	}); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`
		for i := 1; i <= 5; i++ {
			tick()
		}
	`)
	if err == nil {
		t.Fatal("expected native call budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "native_calls" || budgetErr.Limit != 3 {
		t.Fatalf("budget = %s %d, want native_calls 3", budgetErr.Resource, budgetErr.Limit)
	}
	if calls != 3 {
		t.Fatalf("host calls = %d, want 3", calls)
	}
}

func TestWithMaxCallDepthLimitsInterpreterRecursion(t *testing.T) {
	vm := gs.New(gs.WithMaxCallDepth(8))
	err := vm.Exec(`
		func recurse(n) {
			return recurse(n + 1)
		}
		recurse(0)
	`)
	if err == nil {
		t.Fatal("expected call depth budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "call_depth" || budgetErr.Limit != 8 {
		t.Fatalf("budget = %s %d, want call_depth 8", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxGoroutinesLimitsInterpreterGoStatements(t *testing.T) {
	vm := gs.New(gs.WithMaxGoroutines(0))
	if err := vm.Exec(`func done() {}; go done()`); err != nil {
		t.Fatal(err)
	}

	limited := gs.New(gs.WithMaxGoroutines(1))
	err := limited.Exec(`
		block := make(chan)
		func worker() { <-block }
		go worker()
		go worker()
	`)
	if err == nil {
		t.Fatal("expected goroutine budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "goroutines" || budgetErr.Limit != 1 {
		t.Fatalf("budget = %s %d, want goroutines 1", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxChannelCapacityLimitsInterpreterMakeChan(t *testing.T) {
	vm := gs.New(gs.WithMaxChannelCapacity(2))
	err := vm.Exec(`ch := make(chan, 3)`)
	if err == nil {
		t.Fatal("expected channel capacity budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "channel_capacity" || budgetErr.Limit != 2 {
		t.Fatalf("budget = %s %d, want channel_capacity 2", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxHostResultBytesLimitsInterpreterHostCallback(t *testing.T) {
	vm := gs.New(gs.WithMaxHostResultBytes(4))
	if err := vm.RegisterFunc("large", func() string { return "12345" }); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`value := large()`)
	if err == nil {
		t.Fatal("expected host result budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("budget = %s %d, want host_result_bytes 4", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxHostResultBytesLimitsInterpreterProcessOutput(t *testing.T) {
	for _, src := range []string{
		`result := process.run("echo hello")`,
		`result := process.exec("echo", "hello")`,
		`result := process.shell("echo hello")`,
	} {
		vm := gs.New(gs.WithLibs(gs.LibString|gs.LibProcess), gs.WithMaxHostResultBytes(4))
		err := vm.Exec(src)
		var budgetErr *gs.BudgetError
		if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
			t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
		}
	}
}

func TestWithMaxHostResultBytesLimitsInterpreterNetworkResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "12345")
	}))
	defer server.Close()

	for _, src := range []string{
		fmt.Sprintf(`result := net.get(%q)`, server.URL),
		fmt.Sprintf(`result := http.get(%q)`, server.URL),
	} {
		vm := gs.New(gs.WithLibs(gs.LibString|gs.LibNet|gs.LibHTTP), gs.WithMaxHostResultBytes(4))
		err := vm.Exec(src)
		var budgetErr *gs.BudgetError
		if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
			t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
		}
	}
}

func TestWithMaxModuleBytesLimitsInterpreterRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "big.gs"), []byte(`return "12345"`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithRequirePath(dir), gs.WithMaxModuleBytes(4))
	err := vm.Exec(`require("big")`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "module_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("expected module_bytes budget 4, got %T %v", err, err)
	}
}

func TestWithMaxModuleDepthLimitsInterpreterNestedRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.gs"), []byte(`return require("b")`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.gs"), []byte(`return { ok: true }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithRequirePath(dir), gs.WithMaxModuleDepth(1))
	err := vm.Exec(`require("a")`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "module_depth" || budgetErr.Limit != 1 {
		t.Fatalf("expected module_depth budget 1, got %T %v", err, err)
	}
}

func TestWithMaxStepsLimitsBytecodeExecution(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxSteps(8))
	err := vm.Exec(`
		i := 0
		for {
			i += 1
		}
	`)
	if err == nil {
		t.Fatal("expected max step error")
	}
	if !strings.Contains(err.Error(), "execution step limit exceeded") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithMaxStepsLimitsEmptyBytecodeLoop(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxSteps(8))
	err := vm.Exec(`for {}`)
	if err == nil {
		t.Fatal("expected step limit error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "steps" || budgetErr.Limit != 8 {
		t.Fatalf("budget = %s %d, want steps 8", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxNativeCallsLimitsBytecodeHostCalls(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxNativeCalls(3))
	var calls int64
	if err := vm.RegisterFunc("tick", func() int64 {
		calls++
		return calls
	}); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`
		for i := 1; i <= 5; i++ {
			tick()
		}
	`)
	if err == nil {
		t.Fatal("expected native call budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "native_calls" || budgetErr.Limit != 3 {
		t.Fatalf("budget = %s %d, want native_calls 3", budgetErr.Resource, budgetErr.Limit)
	}
	if calls != 3 {
		t.Fatalf("host calls = %d, want 3", calls)
	}
}

func TestWithMaxNativeCallsLimitsBytecodeFastStdlibCalls(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxNativeCalls(2))
	err := vm.Exec(`
		s := "abcdef"
		for i := 1; i <= 4; i++ {
			string.len(s)
		}
	`)
	if err == nil {
		t.Fatal("expected native call budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "native_calls" || budgetErr.Limit != 2 {
		t.Fatalf("budget = %s %d, want native_calls 2", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxCallDepthLimitsBytecodeRecursion(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxCallDepth(8))
	err := vm.Exec(`
		func recurse(n) {
			return recurse(n + 1)
		}
		recurse(0)
	`)
	if err == nil {
		t.Fatal("expected call depth budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "call_depth" || budgetErr.Limit != 8 {
		t.Fatalf("budget = %s %d, want call_depth 8", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxGoroutinesLimitsBytecodeGoStatements(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxGoroutines(1))
	err := vm.Exec(`
		block := make(chan)
		func worker() { <-block }
		go worker()
		go worker()
	`)
	if err == nil {
		t.Fatal("expected goroutine budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "goroutines" || budgetErr.Limit != 1 {
		t.Fatalf("budget = %s %d, want goroutines 1", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxChannelCapacityLimitsBytecodeMakeChan(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxChannelCapacity(2))
	err := vm.Exec(`ch := make(chan, 3)`)
	if err == nil {
		t.Fatal("expected channel capacity budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "channel_capacity" || budgetErr.Limit != 2 {
		t.Fatalf("budget = %s %d, want channel_capacity 2", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxHostResultBytesLimitsBytecodeHostCallback(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxHostResultBytes(4))
	if err := vm.RegisterFunc("large", func() string { return "12345" }); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`value := large()`)
	if err == nil {
		t.Fatal("expected host result budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("budget = %s %d, want host_result_bytes 4", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxHostResultBytesLimitsBytecodeFastStdlibResult(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxHostResultBytes(4))
	err := vm.Exec(`value := base64.encode("1234")`)
	if err == nil {
		t.Fatal("expected host result budget error")
	}
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T %v", err, err)
	}
	if budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("budget = %s %d, want host_result_bytes 4", budgetErr.Resource, budgetErr.Limit)
	}
}

func TestWithMaxHostResultBytesPreflightsEncodingStdlibResults(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`value := base64.encode("1234")`,
				`value := base64.decode("MTIzNDU=")`,
				`value := encoding.hexEncode("123")`,
				`value := encoding.base32Encode("1234")`,
			} {
				opts := append([]gs.Option{
					gs.WithLibs(gs.LibString | gs.LibBase64 | gs.LibEncoding),
					gs.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := gs.New(opts...)
				err := vm.Exec(src)
				var budgetErr *gs.BudgetError
				if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
					t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
				}
			}
		})
	}
}

func TestWithMaxHostResultBytesLimitsCSVEncoding(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`value := csv.encode({{"12345"}})`,
				`value := csv.encodeWithHeaders({{name: "12345"}}, {"name"})`,
			} {
				opts := append([]gs.Option{
					gs.WithLibs(gs.LibString | gs.LibCSV),
					gs.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := gs.New(opts...)
				err := vm.Exec(src)
				var budgetErr *gs.BudgetError
				if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
					t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
				}
			}
		})
	}
}

func TestWithMaxHostResultBytesLimitsBytesAndBinaryOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`buf := bytes.fromString("12345"); value := buf.toString()`,
				`buf := bytes.fromString("123"); value := buf.toHex()`,
				`value := bytes.toHex("123")`,
				`value := bytes.repeat("12", 3)`,
				`value := bytes.concat("12", "345")`,
				`value := binary.pack("bytes:5", "12345")`,
			} {
				opts := append([]gs.Option{
					gs.WithLibs(gs.LibString | gs.LibBytes | gs.LibBinary),
					gs.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := gs.New(opts...)
				err := vm.Exec(src)
				var budgetErr *gs.BudgetError
				if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
					t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
				}
			}
		})
	}
}

func TestWithMaxHostResultBytesLimitsCryptoOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`value := crypto.randomBytes(5)`,
				`value := crypto.randomHex(3)`,
				`value := crypto.generateKey(16)`,
				`key := "1234567890123456"; value := crypto.aesGcmEncrypt(key, "x")`,
			} {
				opts := append([]gs.Option{
					gs.WithLibs(gs.LibString | gs.LibCrypto),
					gs.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := gs.New(opts...)
				err := vm.Exec(src)
				var budgetErr *gs.BudgetError
				if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
					t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
				}
			}
		})
	}
}

func TestWithMaxHostResultBytesLimitsURLOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`value := url.encode("12345")`,
				`value := url.decode("12345")`,
				`value := url.build({scheme: "https", host: "example.com"})`,
				`value := url.queryEncode({name: "12345"})`,
				`value := url.join("https://example.com/", "12345")`,
				`value := url.getHost("https://example.com/path")`,
				`value := url.getPath("https://example.com/12345")`,
			} {
				opts := append([]gs.Option{
					gs.WithLibs(gs.LibString | gs.LibURL),
					gs.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := gs.New(opts...)
				err := vm.Exec(src)
				var budgetErr *gs.BudgetError
				if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
					t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
				}
			}
		})
	}
}

func TestWithMaxHostResultBytesLimitsUTF8Output(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`value := utf8.char(49, 50, 51, 52, 53)`,
				`value := utf8.sanitize("12345")`,
				`value := utf8.reverse("12345")`,
				`value := utf8.sub("12345", 1, 5)`,
				`value := utf8.upper("abcde")`,
				`value := utf8.lower("ABCDE")`,
			} {
				opts := append([]gs.Option{
					gs.WithLibs(gs.LibString | gs.LibUTF8),
					gs.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := gs.New(opts...)
				err := vm.Exec(src)
				var budgetErr *gs.BudgetError
				if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
					t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
				}
			}
		})
	}
}

func TestWithMaxHostResultBytesPreflightsStringOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`value := string.char(49, 50, 51, 52, 53)`,
				`value := string.rep("12", 3)`,
				`value := string.rep("1", 3, "-")`,
				`value := string.repeat("12", 3)`,
				`value := string.join({"12", "345"}, "")`,
				`value := string.padLeft("1", 5, "0")`,
				`value := string.padRight("1", 5, "0")`,
				`value := string.pack("bytes:5", "12345")`,
			} {
				opts := append([]gs.Option{
					gs.WithLibs(gs.LibString),
					gs.WithMaxHostResultBytes(4),
				}, tc.opts...)
				vm := gs.New(opts...)
				err := vm.Exec(src)
				var budgetErr *gs.BudgetError
				if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
					t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
				}
			}
		})
	}
}

func TestWithMaxHostResultBytesLimitsCompressDecodeExpansion(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte("12345")); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	blob := buf.String()

	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibCompress),
				gs.WithMaxHostResultBytes(4),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Set("blob", blob); err != nil {
				t.Fatal(err)
			}
			err := vm.Exec(`value := compress.gzipDecode(blob)`)
			var budgetErr *gs.BudgetError
			if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
				t.Fatalf("expected host_result_bytes budget 4, got %T %v", err, err)
			}
		})
	}
}

func TestWithMaxHostResultBytesLimitsBytecodeProcessOutput(t *testing.T) {
	for _, src := range []string{
		`result := process.run("echo hello")`,
		`result := process.exec("echo", "hello")`,
		`result := process.shell("echo hello")`,
	} {
		vm := gs.New(gs.WithVM(), gs.WithLibs(gs.LibString|gs.LibProcess), gs.WithMaxHostResultBytes(4))
		err := vm.Exec(src)
		var budgetErr *gs.BudgetError
		if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
			t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
		}
	}
}

func TestWithMaxHostResultBytesLimitsBytecodeNetworkResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "12345")
	}))
	defer server.Close()

	for _, src := range []string{
		fmt.Sprintf(`result := net.get(%q)`, server.URL),
		fmt.Sprintf(`result := http.get(%q)`, server.URL),
	} {
		vm := gs.New(gs.WithVM(), gs.WithLibs(gs.LibString|gs.LibNet|gs.LibHTTP), gs.WithMaxHostResultBytes(4))
		err := vm.Exec(src)
		var budgetErr *gs.BudgetError
		if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
			t.Fatalf("%s expected host_result_bytes budget 4, got %T %v", src, err, err)
		}
	}
}

func TestWithMaxModuleBytesLimitsBytecodeRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "big.gs"), []byte(`return "12345"`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithVM(), gs.WithRequirePath(dir), gs.WithMaxModuleBytes(4))
	err := vm.Exec(`require("big")`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "module_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("expected module_bytes budget 4, got %T %v", err, err)
	}
}

func TestWithMaxModuleDepthLimitsBytecodeNestedRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.gs"), []byte(`return require("b")`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.gs"), []byte(`return { ok: true }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithVM(), gs.WithRequirePath(dir), gs.WithMaxModuleDepth(1))
	err := vm.Exec(`require("a")`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "module_depth" || budgetErr.Limit != 1 {
		t.Fatalf("expected module_depth budget 1, got %T %v", err, err)
	}
}

func TestWithMaxStepsAllowsBytecodeExecutionWithinBudget(t *testing.T) {
	vm := gs.New(gs.WithVM(), gs.WithMaxSteps(256))
	if err := vm.Exec(`
		sum := 0
		for i := 0; i < 5; i++ {
			sum += i
		}
	`); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("sum")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(10) {
		t.Fatalf("sum = %v (%T), want int64(10)", got, got)
	}
}

func TestExecGoStyleNumberLiteralsWithVM(t *testing.T) {
	vm := gs.New(gs.WithVM())
	if err := vm.Exec(`result := 0xFF + 0b1010 + 0o20 + 1_000`); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(1281) {
		t.Fatalf("expected 1281, got %v (%T)", got, got)
	}
}

func TestExecError(t *testing.T) {
	vm := gs.New()
	err := vm.Exec(`x :=`)
	if err == nil {
		t.Fatal("expected parse error")
	}
	gsErr, ok := err.(*gs.Error)
	if !ok {
		t.Fatalf("expected *gscript.Error, got %T", err)
	}
	if gsErr.Kind != gs.ErrParse {
		t.Fatalf("expected ErrParse, got %s", gsErr.Kind)
	}
}

func TestCall(t *testing.T) {
	vm := gs.New()
	err := vm.Exec(`
		func add(a, b) {
			return a + b
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	results, err := vm.Call("add", 3, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// GScript int + int returns int
	if results[0] != int64(7) {
		t.Fatalf("expected 7, got %v (%T)", results[0], results[0])
	}
}

func TestCallFunctionRoutesBytecodeClosures(t *testing.T) {
	vm := gs.New(gs.WithVM())
	if err := vm.Exec(`
		func add(a, b) {
			return a + b
		}
	`); err != nil {
		t.Fatal(err)
	}
	fn := vm.GetValue("add")
	if !fn.IsFunction() {
		t.Fatalf("add = %s, want function", fn.TypeName())
	}
	results, err := vm.CallFunction(fn, []runtime.Value{runtime.IntValue(3), runtime.IntValue(4)})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Int() != 7 {
		t.Fatalf("CallFunction results = %v, want 7", results)
	}
}

func TestCallNotFound(t *testing.T) {
	vm := gs.New()
	_, err := vm.Call("nonexistent")
	if err == nil {
		t.Fatal("expected error calling nonexistent function")
	}
}

func TestSetGet(t *testing.T) {
	vm := gs.New()
	if err := vm.Set("x", 42); err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("x")
	if err != nil {
		t.Fatal(err)
	}
	if val != int64(42) {
		t.Fatalf("expected 42, got %v (%T)", val, val)
	}
}

func TestSetGet_string(t *testing.T) {
	vm := gs.New()
	if err := vm.Set("name", "gscript"); err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("name")
	if err != nil {
		t.Fatal(err)
	}
	if val != "gscript" {
		t.Fatalf("expected 'gscript', got %v", val)
	}
}

func TestSetGet_float(t *testing.T) {
	vm := gs.New()
	if err := vm.Set("pi", 3.14); err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("pi")
	if err != nil {
		t.Fatal(err)
	}
	if val != 3.14 {
		t.Fatalf("expected 3.14, got %v", val)
	}
}

func TestSetGet_bool(t *testing.T) {
	vm := gs.New()
	if err := vm.Set("flag", true); err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("flag")
	if err != nil {
		t.Fatal(err)
	}
	if val != true {
		t.Fatalf("expected true, got %v", val)
	}
}

func TestSetGet_nil(t *testing.T) {
	vm := gs.New()
	if err := vm.Set("nothing", nil); err != nil {
		t.Fatal(err)
	}
	val, err := vm.Get("nothing")
	if err != nil {
		t.Fatal(err)
	}
	if val != nil {
		t.Fatalf("expected nil, got %v", val)
	}
}

func TestOSEexitReturnsCatchableExitError(t *testing.T) {
	for _, tc := range []struct {
		name string
		vm   *gs.VM
	}{
		{name: "interpreter", vm: gs.New()},
		{name: "bytecode", vm: gs.New(gs.WithVM())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.vm.Exec(`os.exit(7)`)
			if err == nil {
				t.Fatal("expected exit error")
			}
			var exitErr *gs.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("expected ExitError, got %T %v", err, err)
			}
			if exitErr.Code != 7 {
				t.Fatalf("exit code = %d, want 7", exitErr.Code)
			}
		})
	}
}

func TestOSEexitBooleanStatus(t *testing.T) {
	vm := gs.New()
	err := vm.Exec(`os.exit(false)`)
	var exitErr *gs.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Fatalf("exit code = %d, want 1", exitErr.Code)
	}
}

// --- Error handling tests ---

func TestError_parseError(t *testing.T) {
	vm := gs.New()
	err := vm.Exec(`func {`)
	if err == nil {
		t.Fatal("expected error")
	}
	gsErr, ok := err.(*gs.Error)
	if !ok {
		t.Fatalf("expected *gscript.Error, got %T", err)
	}
	if gsErr.Kind != gs.ErrParse {
		t.Fatalf("expected ErrParse, got %s", gsErr.Kind)
	}
}

func TestError_runtimeError(t *testing.T) {
	vm := gs.New()
	err := vm.Exec(`x := 1 + "abc"`)
	if err == nil {
		t.Fatal("expected runtime error")
	}
	gsErr, ok := err.(*gs.Error)
	if !ok {
		t.Fatalf("expected *gscript.Error, got %T", err)
	}
	if gsErr.Kind != gs.ErrRuntime {
		t.Fatalf("expected ErrRuntime, got %s", gsErr.Kind)
	}
}

// --- Options tests ---

func TestWithPrint(t *testing.T) {
	var captured []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		captured = append(captured, strings.Join(parts, " "))
	}))
	vm.Exec(`print("test", 123)`)
	if len(captured) != 1 {
		t.Fatalf("expected 1 captured, got %d", len(captured))
	}
	if captured[0] != "test 123" {
		t.Fatalf("expected 'test 123', got %q", captured[0])
	}
}

func TestWithLibs(t *testing.T) {
	// LibSafe should still work for basic math
	vm := gs.New(gs.WithLibs(gs.LibSafe))
	err := vm.Exec(`x := 1 + 2`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWithLibsRestrictsUnsafeGlobals(t *testing.T) {
	vm := gs.New(gs.WithLibs(gs.LibSafe))
	err := vm.Exec(`
		hasMath := type(math)
		hasJSON := type(json)
		hasBytes := type(bytes)
		hasURL := type(url)
	`)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hasMath", "hasJSON", "hasBytes", "hasURL"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != "table" {
			t.Fatalf("%s = %v, want table", name, got)
		}
	}
	for _, name := range []string{"io", "os", "fs", "net", "http", "process", "script", "debug", "testkit"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
}

func TestWithLibsRestrictsBytecodeVM(t *testing.T) {
	vm := gs.New(gs.WithLibs(gs.LibSafe), gs.WithVM())
	err := vm.Exec(`
		hasString := type(string)
		hasBytes := type(bytes)
	`)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hasString", "hasBytes"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != "table" {
			t.Fatalf("%s = %v, want table", name, got)
		}
	}
	for _, name := range []string{"http", "debug", "testkit"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
}

func TestWithSandboxDisablesFilesystemCapabilities(t *testing.T) {
	vm := gs.New(gs.WithSandbox())
	if err := vm.Exec(`hasJSON := type(json)`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fs", "dofile", "loadfile"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
	got, err := vm.Get("hasJSON")
	if err != nil {
		t.Fatal(err)
	}
	if got != "table" {
		t.Fatalf("hasJSON = %v, want table", got)
	}
}

func TestSecuritySandboxDisablesHostCapabilitiesAndJIT(t *testing.T) {
	vm := gs.New(gs.WithJIT(), gs.SecuritySandbox(), gs.WithMaxSteps(16))
	if err := vm.Exec(`hasJSON := type(require("json"))`); err != nil {
		t.Fatalf("safe stdlib should remain available: %v", err)
	}
	for _, src := range []string{
		`fs.readfile("x")`,
		`os.getenv("PATH")`,
		`process.pid()`,
		`require("helper")`,
	} {
		if err := vm.Exec(src); err == nil {
			t.Fatalf("SecuritySandbox allowed %s", src)
		}
	}
	err := vm.Exec(`for {}`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected step budget in sandboxed loop, got %T %v", err, err)
	}
	if err := vm.Exec(`fn, loadErr := load("x := 1")`); err != nil {
		t.Fatal(err)
	}
	loadErr, err := vm.Get("loadErr")
	if err != nil {
		t.Fatal(err)
	}
	if msg, ok := loadErr.(string); !ok || !strings.Contains(msg, "dynamic eval disabled") {
		t.Fatalf("loadErr = %v, want dynamic eval disabled", loadErr)
	}
}

func TestWithDynamicEvalFalseBlocksScriptStringCompilation(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithDynamicEval(false)}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`fn, loadErr := load("x := 1")`); err != nil {
				t.Fatal(err)
			}
			fn, err := vm.Get("fn")
			if err != nil {
				t.Fatal(err)
			}
			if fn != nil {
				t.Fatalf("fn = %v, want nil", fn)
			}
			loadErr, err := vm.Get("loadErr")
			if err != nil {
				t.Fatal(err)
			}
			if msg, ok := loadErr.(string); !ok || !strings.Contains(msg, "dynamic eval disabled") {
				t.Fatalf("loadErr = %v, want dynamic eval disabled", loadErr)
			}
			err = vm.Exec(`script.eval("x := 1")`)
			if err == nil || !strings.Contains(err.Error(), "dynamic eval disabled") {
				t.Fatalf("script.eval err = %v, want dynamic eval disabled", err)
			}
		})
	}
}

func TestWithSecurityAppliesSandboxAndBudgets(t *testing.T) {
	vm := gs.New(gs.WithJIT(), gs.WithSecurity(gs.SecurityPolicy{
		Libs:                    gs.LibSafe,
		Capabilities:            gs.CapSafe,
		DisableModuleLoading:    true,
		DisableJIT:              true,
		MaxSteps:                32,
		MaxNativeCalls:          4,
		MaxCallDepth:            8,
		MaxGoroutines:           1,
		MaxChannelCapacity:      2,
		MaxHostResultBytes:      4,
		MaxModuleBytes:          128,
		MaxModuleDepth:          1,
		MaxFilesystemReadBytes:  128,
		MaxFilesystemWriteBytes: 128,
		EnvironmentAllowlist:    []string{"GSCRIPT_PUBLIC_ENV_CAP_TEST"},
		DisableDynamicEval:      true,
		DisableNetworkAccess:    true,
		DisableDebugAccess:      true,
		DisableTestkitAccess:    true,
		DisableProcessExecution: true,
		DisableProcessShell:     true,
	}))
	if err := vm.RegisterFunc("large", func() string { return "12345" }); err != nil {
		t.Fatal(err)
	}
	if got, err := vm.Get("json"); err != nil || got == nil {
		t.Fatalf("safe stdlib should remain available: got=%v err=%v", got, err)
	}
	if err := vm.Exec(`fs.readfile("x")`); err == nil {
		t.Fatal("WithSecurity allowed filesystem API")
	}
	err := vm.Exec(`value := large()`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("expected host_result_bytes budget 4, got %T %v", err, err)
	}
	err = vm.Exec(`for {}`)
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "steps" || budgetErr.Limit != 32 {
		t.Fatalf("expected steps budget 32, got %T %v", err, err)
	}
}

func TestEnvironmentCapabilities(t *testing.T) {
	t.Setenv("GSCRIPT_PUBLIC_ENV_CAP_TEST", "visible")

	tests := []struct {
		name    string
		opts    []gs.Option
		src     string
		wantErr string
	}{
		{
			name:    "environment disabled blocks getenv",
			opts:    []gs.Option{gs.WithEnvironment(false)},
			src:     `value := os.getenv("GSCRIPT_PUBLIC_ENV_CAP_TEST")`,
			wantErr: "environment read access disabled",
		},
		{
			name:    "read disabled blocks expand",
			opts:    []gs.Option{gs.WithEnvironmentRead(false)},
			src:     `value := os.expand("$GSCRIPT_PUBLIC_ENV_CAP_TEST")`,
			wantErr: "environment read access disabled",
		},
		{
			name:    "write disabled blocks setenv",
			opts:    []gs.Option{gs.WithEnvironmentWrite(false)},
			src:     `ok := os.setenv("GSCRIPT_PUBLIC_ENV_WRITE_TEST", "blocked")`,
			wantErr: "environment write access disabled",
		},
		{
			name: "read only still reads",
			opts: []gs.Option{gs.WithEnvironmentWrite(false)},
			src:  `value := os.getenv("GSCRIPT_PUBLIC_ENV_CAP_TEST")`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm := gs.New(tc.opts...)
			err := vm.Exec(tc.src)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Exec error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := vm.Get("value")
			if err != nil {
				t.Fatal(err)
			}
			if got != "visible" {
				t.Fatalf("value = %v, want visible", got)
			}
		})
	}
}

func TestWithProcessShellFalseBlocksShell(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibProcess),
				gs.WithProcessShell(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`result := process.shell("echo blocked")`)
			if err == nil || !strings.Contains(err.Error(), "process shell access disabled") {
				t.Fatalf("process.shell err = %v, want process shell access disabled", err)
			}
		})
	}
}

func TestWithProcessExecutionFalseBlocksRunExecAndWhich(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibProcess),
				gs.WithProcessExecution(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			for _, src := range []string{
				`result := process.run("echo blocked")`,
				`result := process.exec("echo", "blocked")`,
				`result := process.which("echo")`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "process execution access disabled") {
					t.Fatalf("%s err = %v, want process execution access disabled", src, err)
				}
			}
		})
	}
}

func TestWithFilesystemRootConfinesProcessRunDir(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibProcess),
				gs.WithFilesystemRoot(root),
			}, tc.opts...)
			vm := gs.New(opts...)
			src := fmt.Sprintf(`result := process.run({"pwd"}, {dir: %q})`, outside)
			err := vm.Exec(src)
			if err == nil || !strings.Contains(err.Error(), "filesystem access denied") {
				t.Fatalf("process.run dir escape err = %v, want filesystem access denied", err)
			}
		})
	}
}

func TestProcessRunEnvFollowsEnvironmentPolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name+"/write-disabled", func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibProcess),
				gs.WithEnvironmentWrite(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`result := process.run({"pwd"}, {env: {GSCRIPT_PROCESS_ENV_POLICY_TEST: "blocked"}})`)
			if err == nil || !strings.Contains(err.Error(), "environment write access disabled") {
				t.Fatalf("process.run env err = %v, want environment write access disabled", err)
			}
		})

		t.Run(tc.name+"/allowlist", func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibProcess),
				gs.WithEnvironmentAllowlist("GSCRIPT_PROCESS_ENV_ALLOWED"),
			}, tc.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`result := process.run({"pwd"}, {env: {GSCRIPT_PROCESS_ENV_BLOCKED: "blocked"}})`)
			if err == nil || !strings.Contains(err.Error(), "environment variable not allowed: GSCRIPT_PROCESS_ENV_BLOCKED") {
				t.Fatalf("process.run env allowlist err = %v, want environment variable not allowed", err)
			}
		})
	}
}

func TestWithNetworkAccessFalseBlocksNetAndHTTP(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibNet | gs.LibHTTP),
				gs.WithNetworkAccess(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			for _, src := range []string{
				`resp := net.get("http://127.0.0.1:1")`,
				`resp := net.request({url: "http://127.0.0.1:1"})`,
				`resp := http.get("http://127.0.0.1:1")`,
				`server := http.listen("127.0.0.1:0", func(req, res) {}, {background: true})`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "network access disabled") {
					t.Fatalf("%s err = %v, want network access disabled", src, err)
				}
			}
		})
	}
}

func TestWithDebugAccessFalseBlocksDebugAPIs(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibDebug),
				gs.WithDebugAccess(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			for _, src := range []string{
				`stack := debug.stack()`,
				`globals := debug.globals()`,
				`raw := debug.goStack()`,
				`debug.setHook(func(event) {})`,
				`debug.emit("blocked")`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "debug access disabled") {
					t.Fatalf("%s err = %v, want debug access disabled", src, err)
				}
			}
		})
	}
}

func TestWithTestkitAccessFalseBlocksTestkitAPIs(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibTestkit),
				gs.WithTestkitAccess(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			for _, src := range []string{
				`stats := testkit.memory()`,
				`info := testkit.value(42)`,
				`kind := testkit.typeOf(42)`,
				`result := testkit.protect(func() { return 1 })`,
				`same := testkit.sameFunction(print, print)`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "testkit access disabled") {
					t.Fatalf("%s err = %v, want testkit access disabled", src, err)
				}
			}
		})
	}
}

func TestEnvironmentAllowlist(t *testing.T) {
	t.Setenv("GSCRIPT_PUBLIC_ENV_ALLOWED", "visible")
	t.Setenv("GSCRIPT_PUBLIC_ENV_BLOCKED", "secret")

	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithEnvironmentAllowlist("GSCRIPT_PUBLIC_ENV_ALLOWED")}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
				allowed := os.getenv("GSCRIPT_PUBLIC_ENV_ALLOWED")
				blocked := os.getenv("GSCRIPT_PUBLIC_ENV_BLOCKED")
				expanded := os.expand("$GSCRIPT_PUBLIC_ENV_ALLOWED:$GSCRIPT_PUBLIC_ENV_BLOCKED")
				all := os.environ()
				procEnv := process.env()
			`); err != nil {
				t.Fatal(err)
			}
			for name, want := range map[string]interface{}{
				"allowed":  "visible",
				"blocked":  nil,
				"expanded": "visible:",
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Fatalf("%s = %v, want %v", name, got, want)
				}
			}
			for _, tableName := range []string{"all", "procEnv"} {
				got, err := vm.Get(tableName)
				if err != nil {
					t.Fatal(err)
				}
				env, ok := got.(map[string]interface{})
				if !ok {
					t.Fatalf("%s = %T, want map", tableName, got)
				}
				if env["GSCRIPT_PUBLIC_ENV_ALLOWED"] != "visible" {
					t.Fatalf("%s allowed = %v, want visible", tableName, env["GSCRIPT_PUBLIC_ENV_ALLOWED"])
				}
				if _, ok := env["GSCRIPT_PUBLIC_ENV_BLOCKED"]; ok {
					t.Fatalf("%s exposed blocked environment variable", tableName)
				}
			}
			err := vm.Exec(`os.setenv("GSCRIPT_PUBLIC_ENV_BLOCKED", "changed")`)
			if err == nil || !strings.Contains(err.Error(), "environment variable not allowed") {
				t.Fatalf("setenv blocked err = %v, want environment variable not allowed", err)
			}
		})
	}
}

func TestWithModuleLoadingFalseAllowsStdlibRequireButBlocksFileModules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "helper.gs"), []byte(`return { value: 42 }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithRequirePath(dir), gs.WithModuleLoading(false))
	if err := vm.Exec(`result := type(require("json"))`); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != "table" {
		t.Fatalf("stdlib require result = %v, want table", got)
	}
	err = vm.Exec(`require("helper")`)
	if err == nil || !strings.Contains(err.Error(), "module loading disabled") {
		t.Fatalf("require helper error = %v, want module loading disabled", err)
	}
}

func TestWithModuleLoadingFalseRestrictsBytecodeVM(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "helper.gs"), []byte(`return { value: 42 }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithRequirePath(dir), gs.WithModuleLoading(false), gs.WithVM())
	if err := vm.Exec(`stdlibResult := type(require("json"))`); err != nil {
		t.Fatalf("stdlib require should still work with module loading disabled: %v", err)
	}
	got, err := vm.Get("stdlibResult")
	if err != nil {
		t.Fatal(err)
	}
	if got != "table" {
		t.Fatalf("stdlibResult = %v, want table", got)
	}
	err = vm.Exec(`require("helper")`)
	if err == nil {
		t.Fatal("expected require to fail when module loading is disabled")
	}
}

func TestEachPublicLibFlagExposesNamedGlobal(t *testing.T) {
	tests := []struct {
		name   string
		flag   gs.LibFlags
		global string
	}{
		{"bytes", gs.LibBytes, "bytes"},
		{"url", gs.LibURL, "url"},
		{"bits", gs.LibBits, "bits"},
		{"csv", gs.LibCSV, "csv"},
		{"uuid", gs.LibUUID, "uuid"},
		{"matrix", gs.LibMatrix, "matrix"},
		{"compress", gs.LibCompress, "compress"},
		{"container", gs.LibContainer, "container"},
		{"rl", gs.LibRL, "rl"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm := gs.New(gs.WithLibs(tc.flag))
			if err := vm.Exec(`result := type(` + tc.global + `)`); err != nil {
				t.Fatal(err)
			}
			got, err := vm.Get("result")
			if err != nil {
				t.Fatal(err)
			}
			if got != "table" {
				t.Fatalf("type(%s) = %v, want table", tc.global, got)
			}
		})
	}
}

// --- Integration: Go functions called from GScript ---

func TestIntegration_goFuncWithScriptCallback(t *testing.T) {
	var output []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))

	vm.RegisterFunc("applyTwice", func(x int64) int64 {
		return x * 2 * 2
	})

	err := vm.Exec(`
		result := applyTwice(5)
		print(result)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 1 || output[0] != "20" {
		t.Fatalf("expected '20', got %v", output)
	}
}
