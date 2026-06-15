package leia_test

import (
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLoopHelpers(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: "simple"},
				{
					Status: "tool_calls",
					Calls: []llm.ToolCall{{
						ID:   "call_1",
						Tool: "lookup",
						Args: map[string]any{"name": "leia"},
					}},
				},
				{Status: "final_answer", Text: "react"},
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)
			if err := vm.Exec(`
simple, simple_err := loop.simple({system: "short", user: "hello"})
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: ["name"]})
react, react_err := loop.react({
    user: "find docs",
    tools: [lookup],
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
			if len(provider.requests[2].Messages) != 3 || provider.requests[2].Messages[2].Value != "docs:leia" {
				t.Fatalf("react request = %#v", provider.requests[2])
			}
		})
	}
}

func TestLoopSnapshotResume(t *testing.T) {
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
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: ["name"]})
pending := {id: "call_1", tool: "lookup", args: {name: "old"}}
token := loop.snapshot({msg.user("find docs")}, pending)
approved, approved_err := loop.resume(token, {ok: true, args: {name: "leia"}}, {lookup})
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
			if approvedStatus != "dispatched" || approvedHistoryLen != int64(3) || approvedValue != "docs:leia" {
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
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, tc.opts...)...)
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
}, {params: ["name"]})
token, snap_err := loop.snapshot({msg.user("find docs")}, {id: "call_1", tool: "lookup", args: {name: "leia"}}, store)
stored_name := saved.pending.args.name
loaded, loaded_err := loop.resume("external-token", {ok: true}, {lookup}, store)
loaded_status := loaded.status
loaded_value := loaded.value
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]interface{}{
				"stored_name":   "leia",
				"loaded_status": "dispatched",
				"loaded_value":  "docs:leia",
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
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []llm.TraceEvent
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMTrace(func(event llm.TraceEvent) {
					events = append(events, event)
				}),
			}, tc.opts...)
			vm := leia.New(opts...)
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
}, {params: ["name"]})
token, snap_err := loop.snapshot({msg.user("find docs")}, {id: "call_1", tool: "lookup", args: {name: "leia"}}, store)
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
					Tool: "refund",
					Args: map[string]any{"amount": int64(150)},
				}},
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)
			if err := vm.Exec(`
refund := llm.tool("refund", func(amount) {
    return "refund:" .. amount, nil
}, {params: ["amount"]})
result, err := loop.react({
    user: "refund order",
    tools: [refund],
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
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: "1. lookup docs\n2. answer"},
				{
					Status: "tool_calls",
					Calls: []llm.ToolCall{{
						ID:   "call_1",
						Tool: "lookup",
						Args: map[string]any{"name": "leia"},
					}},
				},
				{Status: "final_answer", Text: "done"},
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)
			if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: ["name"]})
result, err := loop.plan_execute({
    user: "find docs",
    tools: [lookup],
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
			if len(provider.requests[2].Messages) < 4 || provider.requests[2].Messages[3].Value != "docs:leia" {
				t.Fatalf("final messages = %#v", provider.requests[2].Messages)
			}
		})
	}
}

func TestLoopReflect(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: "draft"},
				{Status: "final_answer", Text: "refined"},
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)
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
