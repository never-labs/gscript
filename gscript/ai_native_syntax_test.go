package gscript_test

import (
	"strings"
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

func TestAINativeSyntaxExecutesThroughLLMStdlib(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []gs.LLMTurnResult{
				{Status: "final_answer", Text: "agent-ok"},
				{Status: "final_answer", Text: "turn-ok"},
			}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
// Lookup docs.
//gscript:requires docs.read, net.client
//gscript:param query search query text
tool lookup(query) {
    return "found:" .. query, nil
}

models {
    default: "mock-alias"
    "mock-alias": {provider_model: "mock-fast"}
}

agent defaults {
    model: "mock-alias"
    system: "Be concise."
    tools: [lookup]
}

answer := agent(q) {
    user: q
}

result, err := answer("search gscript")
direct, direct_err := turn {
    model: "direct-model"
    messages: messages { user: "single turn" }
}
agent_text := result.text
turn_text := direct.text
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 2 {
				t.Fatalf("requests = %d, want 2", len(provider.requests))
			}
			agentReq := provider.requests[0]
			if agentReq.Model != "mock-fast" {
				t.Fatalf("agent model = %q, want mock-fast", agentReq.Model)
			}
			if len(agentReq.Messages) != 2 || agentReq.Messages[0].Role != "system" || agentReq.Messages[1].Text != "search gscript" {
				t.Fatalf("agent messages = %#v", agentReq.Messages)
			}
			if len(agentReq.Tools) != 1 || agentReq.Tools[0].Name != "lookup" {
				t.Fatalf("agent tools = %#v", agentReq.Tools)
			}
			if agentReq.Tools[0].Description != "Lookup docs." {
				t.Fatalf("tool description = %q", agentReq.Tools[0].Description)
			}
			if len(agentReq.Tools[0].Requires) != 2 || agentReq.Tools[0].Requires[0] != "docs.read" || agentReq.Tools[0].Requires[1] != "net.client" {
				t.Fatalf("tool requires = %#v", agentReq.Tools[0].Requires)
			}
			if provider.requests[1].Model != "direct-model" || len(provider.requests[1].Messages) != 1 || provider.requests[1].Messages[0].Text != "single turn" {
				t.Fatalf("turn request = %#v", provider.requests[1])
			}
			gotAgent, err := vm.Get("agent_text")
			if err != nil {
				t.Fatalf("Get agent_text: %v", err)
			}
			if gotAgent != "agent-ok" {
				t.Fatalf("agent_text = %#v", gotAgent)
			}
			gotTurn, err := vm.Get("turn_text")
			if err != nil {
				t.Fatalf("Get turn_text: %v", err)
			}
			if gotTurn != "turn-ok" {
				t.Fatalf("turn_text = %#v", gotTurn)
			}
		})
	}
}

func TestAINativeSyntaxValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "duplicate defaults",
			src: `
agent defaults { model: "a" }
agent defaults { model: "b" }
`,
			want: "duplicate agent defaults",
		},
		{
			name: "nested defaults",
			src: `
func f() {
    agent defaults { model: "a" }
}
`,
			want: "module scope",
		},
		{
			name: "literal api key",
			src: `
models {
    default: "m"
    m: {provider_model: "m", api_key: "secret"}
}
`,
			want: "api_key",
		},
		{
			name: "provider protocol must be string",
			src: `
models {
    default: "m"
    m: {protocol: ("openai" .. "_compatible"), provider_model: "m"}
}
`,
			want: "protocol must be a string literal",
		},
		{
			name: "provider protocol whitelist",
			src: `
models {
    default: "m"
    m: {protocol: "unsupported-test-protocol", provider_model: "m"}
}
`,
			want: "unsupported model protocol",
		},
		{
			name: "provider config requires model",
			src: `
models {
    default: "m"
    m: {protocol: "openai_compatible", base_url: "http://127.0.0.1:1"}
}
`,
			want: "provider_model or model",
		},
		{
			name: "model alias cycle",
			src: `
models {
    a: "b"
    b: "a"
}
`,
			want: "alias cycle",
		},
		{
			name: "tool missing requires",
			src: `
tool lookup(query) {
    return query, nil
}
`,
			want: "missing gscript:requires",
		},
		{
			name: "tool invalid requires",
			src: `
//gscript:requires docs..read
tool lookup(query) {
    return query, nil
}
`,
			want: "invalid gscript:requires",
		},
		{
			name: "tool unknown param doc",
			src: `
//gscript:requires none
//gscript:param missing not a parameter
tool lookup(query) {
    return query, nil
}
`,
			want: "unknown parameter",
		},
		{
			name: "tool duplicate param doc",
			src: `
//gscript:requires none
//gscript:param query first
//gscript:param query second
tool lookup(query) {
    return query, nil
}
`,
			want: "duplicate gscript:param",
		},
		{
			name: "agent duplicate tools",
			src: `
//gscript:requires none
tool lookup(query) {
    return query, nil
}

agent answer(q) {
    tools: [lookup, lookup]
    user: q
}
`,
			want: "duplicate tool",
		},
		{
			name: "agent unknown static tool",
			src: `
agent answer(q) {
    tools: [missing]
    user: q
}
`,
			want: "undeclared tool",
		},
		{
			name: "defaults unknown static tool",
			src: `
agent defaults {
    tools: [missing]
}
`,
			want: "undeclared tool",
		},
		{
			name: "turn unknown static tool",
			src: `
func f() {
    _ = turn {
        tools: [missing]
        user: "hello"
    }
}
`,
			want: "undeclared tool",
		},
		{
			name: "agent capabilities missing tool requirement",
			src: `
//gscript:requires docs.read, net.client
tool lookup(query) {
    return query, nil
}

agent answer(q) {
    tools: [lookup]
    capabilities: ["docs.read"]
    user: q
}
`,
			want: "capabilities missing required capability \"net.client\"",
		},
		{
			name: "agent defaults merged capabilities missing inherited tool requirement",
			src: `
//gscript:requires net.client
tool lookup(query) {
    return query, nil
}

agent defaults {
    tools: [lookup]
}

agent answer(q) {
    capabilities: []
    user: q
}
`,
			want: "capabilities missing required capability \"net.client\"",
		},
		{
			name: "turn caps missing tool requirement",
			src: `
//gscript:requires payments.refund
tool refund(id) {
    return id, nil
}

func f() {
    _ = turn {
        tools: [refund]
        caps: []
        user: "refund"
    }
}
`,
			want: "capabilities missing required capability \"payments.refund\"",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []struct {
				name string
				opts []gs.Option
			}{
				{name: "interpreter"},
				{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
			} {
				t.Run(mode.name, func(t *testing.T) {
					opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
					vm := gs.New(opts...)
					err := vm.Exec(tc.src)
					if err == nil || !strings.Contains(err.Error(), tc.want) {
						t.Fatalf("Exec error = %v, want substring %q", err, tc.want)
					}
				})
			}
		})
	}
}

func TestAINativeSyntaxValidationAllowsAliasOnlyModels(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
models {
    default: "fast"
    fast: "host-fast"
    host: {provider_model: "host-only"}
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestAINativeSyntaxValidationAllowsStaticToolCapsCoverage(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
//gscript:requires docs.read, net.client
tool lookup(query) {
    return query, nil
}

//gscript:requires none
tool local_only(query) {
    return query, nil
}

agent defaults {
    tools: [lookup]
}

agent inherited(q) {
    capabilities: ["docs.read", "net.client"]
    user: q
}

agent override(q) {
    tools: [local_only]
    capabilities: []
    user: q
}

agent answer(q) {
    tools: [lookup]
    capabilities: ["docs.read", "net.client"]
    user: q
}

func f(caps) {
    _ = turn {
        tools: [lookup]
        caps: caps
        user: "hello"
    }
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestAINativeSyntaxValidationAllowsDynamicDefaultsToolCapsRefs(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
//gscript:requires net.client
tool lookup(query) {
    return query, nil
}

default_tools := [lookup]
default_caps := []

agent defaults {
    tools: default_tools
    capabilities: default_caps
}

agent answer(q) {
    user: q
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestAINativeSyntaxValidationAllowsDynamicToolRefs(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
func f(tools) {
    _ = turn {
        tools: [tools[0]]
        user: "hello"
    }
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestAINativeSyntaxValidationAllowsScopedToolRefs(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
func make_agent(prefix) {
    //gscript:requires none
    tool local_lookup(query) {
        return prefix .. query, nil
    }
    return agent(q) {
        tools: [local_lookup]
        user: q
    }
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestAINativeStandaloneBudgetLimitsDirectTurns(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []gs.LLMTurnResult{
				{Status: "final_answer", Text: "one"},
				{Status: "final_answer", Text: "two"},
			}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
err_kind := nil
err_dimension := nil

budget { turns: 1 } {
    first, first_err := turn { user: "first" }
    second, second_err := turn { user: "second" }
    err_kind = second_err.kind
    err_dimension = second_err.dimension
}
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			kind, _ := vm.Get("err_kind")
			dimension, _ := vm.Get("err_dimension")
			if kind != "budget" || dimension != "turns" {
				t.Fatalf("err kind=%#v dimension=%#v", kind, dimension)
			}
		})
	}
}

func TestAINativeStandaloneBudgetNestedIntersectionUsesOuterTokens(t *testing.T) {
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
			if err := vm.Exec(`
err_kind := nil
err_dimension := nil

budget { tokens: 5 } {
    budget { tokens: 100 } {
        first, first_err := turn { user: "first" }
        second, second_err := turn { user: "second" }
        err_kind = second_err.kind
        err_dimension = second_err.dimension
    }
}
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			kind, _ := vm.Get("err_kind")
			dimension, _ := vm.Get("err_dimension")
			if kind != "budget" || dimension != "tokens" {
				t.Fatalf("err kind=%#v dimension=%#v", kind, dimension)
			}
		})
	}
}

func TestAINativeStandaloneBudgetLimitsToolCallsAndTime(t *testing.T) {
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
						ID:   "call_1",
						Tool: "lookup",
						Args: map[string]any{"query": "gscript"},
					}},
				},
				{Status: "final_answer", Text: "unused"},
			}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
//gscript:requires none
tool lookup(query) {
    return "found:" .. query, nil
}

agent researcher() {
    model: "mock"
    tools: [lookup]
    user: "find gscript"
}

call_kind := nil
call_dimension := nil
time_kind := nil
time_message := nil

budget { calls: 0 } {
    result, err := researcher()
    call_kind = err.kind
    call_dimension = err.dimension
}

budget { time: 0 } {
    result, err := turn { user: "deadline" }
    time_kind = err.kind
    time_message = err.message
}
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			callKind, _ := vm.Get("call_kind")
			callDimension, _ := vm.Get("call_dimension")
			if callKind != "budget" || callDimension != "calls" {
				t.Fatalf("call err kind=%#v dimension=%#v", callKind, callDimension)
			}
			timeKind, _ := vm.Get("time_kind")
			timeMessage, _ := vm.Get("time_message")
			if timeKind != "deadline" || timeMessage != "deadline exceeded" {
				t.Fatalf("time err kind=%#v message=%#v", timeKind, timeMessage)
			}
		})
	}
}

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

func TestAINativeAgentFlowImplicitConfigLocalsAreWhitelistedAndShadowable(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := gs.New(append([]gs.Option{gs.WithLibs(gs.LibLLM)}, tc.opts...)...)
			err := vm.Exec(`
agent probe(q) {
    model: "cfg-model"
    system: "cfg-system"
    capabilities: ["cfg.cap"]
    user: q
    response_format: {type: "json_object"}
} flow {
    observed := model .. "|" .. system .. "|" .. capabilities[1]
    model := "local-model"
    system := "local-system"
    capabilities := ["local.cap"]
    return {
        observed: observed,
        shadowed: model .. "|" .. system .. "|" .. capabilities[1]
    }, nil
}

out, err := probe("hello")
observed := out.observed
shadowed := out.shadowed
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			observed, _ := vm.Get("observed")
			shadowed, _ := vm.Get("shadowed")
			if observed != "cfg-model|cfg-system|cfg.cap" {
				t.Fatalf("observed = %#v", observed)
			}
			if shadowed != "local-model|local-system|local.cap" {
				t.Fatalf("shadowed = %#v", shadowed)
			}
		})
	}
}

func TestAINativeAgentFlowDoesNotInjectArbitraryMetaFields(t *testing.T) {
	for _, field := range []struct {
		name   string
		config string
	}{
		{name: "user", config: `user: q`},
		{name: "response_format", config: `response_format: {type: "json_object"}`},
		{name: "metadata", config: `metadata: {trace_id: "abc"}`},
	} {
		t.Run(field.name, func(t *testing.T) {
			for _, tc := range []struct {
				name string
				opts []gs.Option
			}{
				{name: "interpreter"},
				{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					vm := gs.New(append([]gs.Option{gs.WithLibs(gs.LibLLM)}, tc.opts...)...)
					err := vm.Exec(`
` + field.name + ` := "outer-` + field.name + `"

agent probe(q) {
    model: "cfg-model"
    ` + field.config + `
} flow {
    return ` + field.name + `, nil
}

out, err := probe("hello")
got := out
`)
					if err != nil {
						t.Fatalf("Exec: %v", err)
					}
					got, err := vm.Get("got")
					if err != nil {
						t.Fatalf("Get got: %v", err)
					}
					if want := "outer-" + field.name; got != want {
						t.Fatalf("got = %#v, want %q", got, want)
					}
				})
			}
		})
	}
}
