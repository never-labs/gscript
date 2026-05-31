package gscript_test

import (
	gs "github.com/never-labs/gscript"
	"strings"
	"testing"
)

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

//gscript:requires none
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

func TestAINativeMessagesBlockAllowsMixedMessageItems(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: "ok"}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
call := {id: "call_1", tool: "lookup", args: {query: "gscript"}}
history := messages {
    system: "System text."
    user: "Find docs."
    msg.assistant_call(call)
    msg.tool_result("call_1", "docs")
    user: "Summarize."
}
result, err := turn {
    model: "mock-chat"
    messages: history
}
history_len := #history
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			msgs := provider.requests[0].Messages
			if len(msgs) != 5 {
				t.Fatalf("messages = %#v, want 5", msgs)
			}
			if msgs[0].Role != "system" || msgs[1].Role != "user" ||
				msgs[2].Role != "assistant" || msgs[2].ToolCall == nil || msgs[2].ToolCall.Tool != "lookup" ||
				msgs[3].Role != "tool" || msgs[3].ToolUseID != "call_1" || msgs[3].Value != "docs" ||
				msgs[4].Role != "user" || msgs[4].Text != "Summarize." {
				t.Fatalf("messages order/content = %#v", msgs)
			}
			historyLen, err := vm.Get("history_len")
			if err != nil {
				t.Fatalf("Get history_len: %v", err)
			}
			if historyLen != int64(5) {
				t.Fatalf("history_len = %#v, want 5", historyLen)
			}
		})
	}
}

func TestAINativeHistoryHelpersAndValidateOutput(t *testing.T) {
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
call := {id: "call_1", tool: "lookup", args: {query: "gscript"}}
h := messages {
    user: "Find docs."
    msg.assistant_call(call)
    msg.tool_result("call_1", {summary: "docs"})
}
tool_msg, tool_idx := history.find(h, {role: "tool"})
assistant_msg, assistant_idx := history.last(h, {role: "assistant"})
all_users := history.find_all(h, {role: "user"})
history.append(h, msg.user("Summarize."))
ok, ok_msg := llm.validate_output({summary: "docs"}, {summary: "example"})
bad, bad_msg := llm.validate_output({summary: 1}, {summary: "example"})
json_ok, _ := llm.validate_output("{\"summary\":\"docs\"}", {summary: "example"})
history_len := #h
tool_summary := tool_msg.value.summary
assistant_tool := assistant_msg.tool_call.tool
user_count := #all_users
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"tool_idx":       int64(3),
				"assistant_idx":  int64(2),
				"history_len":    int64(4),
				"tool_summary":   "docs",
				"assistant_tool": "lookup",
				"user_count":     int64(1),
				"ok":             true,
				"ok_msg":         "",
				"bad":            false,
				"json_ok":        true,
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
			badMsg, err := vm.Get("bad_msg")
			if err != nil {
				t.Fatalf("Get bad_msg: %v", err)
			}
			if msg, ok := badMsg.(string); !ok || !strings.Contains(msg, `field "summary" has type number, want string`) {
				t.Fatalf("bad_msg = %#v", badMsg)
			}
		})
	}
}

func TestAINativeFlowAgentUsesStdlibAgentAmbientConfig(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []gs.LLMTurnResult{
				{Status: "final_answer", Text: "one", Usage: gs.LLMTurnUsage{InputTokens: 2, OutputTokens: 3}},
				{Status: "final_answer", Text: "two"},
			}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
models {
    default: "default-alias"
    "default-alias": "resolved-default"
    flow_alias: "resolved-flow"
}

// Echoes a query.
//gscript:requires none
tool echo_tool(query) {
    return query, nil
}

agent defaults {
    model: "default-alias"
    system: "Default system."
}

agent flow_support(q) {
    model: "flow_alias"
    tools: [echo_tool]
    user: q
    budget: { tokens: 5 }
} flow {
    first, first_err := turn {}
    second, second_err := turn { user: "second " .. q }
    return {
        first_text: first.text
        err_kind: second_err.kind
        err_dimension: second_err.dimension
    }, nil
}

out, err := flow_support("hello")
first_text := out.first_text
err_kind := out.err_kind
err_dimension := out.err_dimension
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			req := provider.requests[0]
			if req.Model != "resolved-flow" {
				t.Fatalf("flow model = %q, want resolved-flow", req.Model)
			}
			if len(req.Messages) != 2 ||
				req.Messages[0].Role != "system" || req.Messages[0].Text != "Default system." ||
				req.Messages[1].Role != "user" || req.Messages[1].Text != "hello" {
				t.Fatalf("flow messages = %#v", req.Messages)
			}
			if len(req.Tools) != 1 || req.Tools[0].Name != "echo_tool" {
				t.Fatalf("flow tools = %#v", req.Tools)
			}
			firstText, _ := vm.Get("first_text")
			errKind, _ := vm.Get("err_kind")
			errDimension, _ := vm.Get("err_dimension")
			if firstText != "one" || errKind != "budget" || errDimension != "tokens" {
				t.Fatalf("first_text=%#v err_kind=%#v err_dimension=%#v", firstText, errKind, errDimension)
			}
		})
	}
}
