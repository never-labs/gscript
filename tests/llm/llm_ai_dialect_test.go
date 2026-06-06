package leia_test

import (
	"context"
	"path/filepath"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

type aiDialectStreamingProvider struct {
	requests       []llm.TurnRequest
	streamRequests []llm.TurnRequest
}

func (p *aiDialectStreamingProvider) Turn(_ context.Context, req llm.TurnRequest) (llm.TurnResult, error) {
	p.requests = append(p.requests, req)
	return llm.TurnResult{Status: "final_answer", Text: "agent-ok"}, nil
}

func (p *aiDialectStreamingProvider) StreamTurn(_ context.Context, req llm.TurnRequest, sink llm.StreamSink) (llm.TurnResult, error) {
	p.streamRequests = append(p.streamRequests, req)
	for _, token := range []string{"stream", "-", "ok"} {
		if sink != nil {
			if err := sink(llm.StreamEvent{Type: "token", Token: token, Text: token}); err != nil {
				return llm.TurnResult{}, err
			}
		}
	}
	return llm.TurnResult{Status: "final_answer", Text: "stream-ok"}, nil
}

func TestAIDialectUsesLLMStdlibRuntime(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: "agent-ok"},
				{Status: "final_answer", Text: "turn-ok"},
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM | leia.LibDialect),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)
			err := vm.Exec(`
model {
    default: "mock-fast"
}

read_file := tool {
    name: "read_file"
    fn: func(path) { return "file:" .. path, nil }
    params: {"path"}
    description: "Read a workspace file."
}

search_text := tool {
    name: "search_text"
    fn: func(query) { return "matches:" .. query, nil }
    params: {"query"}
    description: "Search workspace text."
}

apply_patch := tool {
    name: "apply_patch"
    fn: func(patch) { return "patched", nil }
    params: {"patch"}
    description: "Apply a patch."
}

run_shell := tool {
    name: "run_shell"
    fn: func(command) { return "ran:" .. command, nil }
    params: {"command"}
    description: "Run a shell command."
}

coding_agent := agent {
    name: "coding_agent"
    config: func(task) {
        return {
            model: "mock-fast"
            system: "Use the repository tools."
            user: task
            tools: {read_file, search_text, apply_patch, run_shell}
        }, nil
    }
    params: {"task"}
    description: "Repository coding agent."
}

agent_result, agent_err := coding_agent("inspect the README")
turn_result, turn_err := turn {
    model: "mock-fast"
    messages: {llm.user("single turn")}
}
agent_err_kind := nil
agent_err_message := nil
if agent_err != nil {
    agent_err_kind = agent_err.kind
    agent_err_message = agent_err.message
}
turn_err_kind := nil
turn_err_message := nil
if turn_err != nil {
    turn_err_kind = turn_err.kind
    turn_err_message = turn_err.message
}
agent_text := nil
turn_text := nil
if agent_result != nil {
    agent_text = agent_result.text
}
if turn_result != nil {
    turn_text = turn_result.text
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			agentText, _ := vm.Get("agent_text")
			turnText, _ := vm.Get("turn_text")
			if len(provider.requests) != 2 {
				agentKind, _ := vm.Get("agent_err_kind")
				agentMessage, _ := vm.Get("agent_err_message")
				turnKind, _ := vm.Get("turn_err_kind")
				turnMessage, _ := vm.Get("turn_err_message")
				t.Fatalf("requests = %d, want 2; got=%#v agent_text=%#v turn_text=%#v agent_err=%#v/%#v turn_err=%#v/%#v", len(provider.requests), provider.requests, agentText, turnText, agentKind, agentMessage, turnKind, turnMessage)
			}
			agentReq := provider.requests[0]
			if agentReq.Model != "mock-fast" {
				t.Fatalf("agent model = %q, want mock-fast", agentReq.Model)
			}
			wantTools := []string{"read_file", "search_text", "apply_patch", "run_shell"}
			if len(agentReq.Tools) != len(wantTools) {
				t.Fatalf("agent tools = %#v", agentReq.Tools)
			}
			for i, want := range wantTools {
				if agentReq.Tools[i].Name != want {
					t.Fatalf("tool[%d].Name = %q, want %q", i, agentReq.Tools[i].Name, want)
				}
			}
			if provider.requests[1].Messages[0].Text != "single turn" {
				t.Fatalf("turn request = %#v", provider.requests[1])
			}
			if agentText != "agent-ok" || turnText != "turn-ok" {
				agentKind, _ := vm.Get("agent_err_kind")
				agentMessage, _ := vm.Get("agent_err_message")
				turnKind, _ := vm.Get("turn_err_kind")
				turnMessage, _ := vm.Get("turn_err_message")
				t.Fatalf("agent_text=%#v turn_text=%#v agent_err=%#v/%#v turn_err=%#v/%#v", agentText, turnText, agentKind, agentMessage, turnKind, turnMessage)
			}
		})
	}
}

func TestAIDialectTurnWorksOnBytecodeVM(t *testing.T) {
	provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: "turn-ok"}}
	vm := leia.New(
		leia.WithVM(),
		leia.WithLibs(leia.LibString|leia.LibLLM|leia.LibDialect),
		leia.WithLLMProvider(provider),
	)
	if err := vm.Exec(`
model {
    default: "mock-fast"
}

read_file := tool {
    name: "read_file"
    fn: func(path) { return "file:" .. path, nil }
    params: {"path"}
}

search_text := tool {
    name: "search_text"
    fn: func(query) { return "matches:" .. query, nil }
    params: {"query"}
}

result, err := turn {
    model: "mock-fast"
    messages: {llm.user("single turn")}
    tools: {read_file, search_text}
}
text := result.text
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(provider.requests))
	}
	if got := []string{provider.requests[0].Tools[0].Name, provider.requests[0].Tools[1].Name}; got[0] != "read_file" || got[1] != "search_text" {
		t.Fatalf("tool names = %#v", got)
	}
	text, _ := vm.Get("text")
	if text != "turn-ok" {
		t.Fatalf("text = %#v", text)
	}
}

func TestAIDialectTaggedLiteralRawBlockAgentToolTurnAndStreaming(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &aiDialectStreamingProvider{}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM | leia.LibDialect),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)
			if err := vm.Exec(`
subject := "leia"

model {
    default: "mock-fast"
}

lookup := tool {
    name: "lookup"
    fn: func(query) { return "found:" .. query, nil }
    params: {"query"}
    description: "Lookup docs."
}

assistant := agent {
    name: "assistant"
    config: func(q) {
        return {
            model: "mock-fast"
            messages: {llm.user(prompt` + "`" + `research ${q}` + "`" + `.text)}
            tools: {lookup}
        }, nil
    }
    params: {"q"}
    description: "Prompt-backed research agent."
}

raw_quote := quote {
    step := "collect"
    step = step .. "-evidence"
}

agent_result, agent_err := assistant(subject)
streamed := ""
turn_result, turn_err := turn {
    model: "mock-fast"
    messages: {llm.user(prompt` + "`" + `stream ${subject}` + "`" + `.text)}
    tools: {lookup}
    force_tool: lookup
    on_stream: func(event) {
        streamed = streamed .. event.token
    }
}

agent_text := agent_result.text
turn_text := turn_result.text
quote_kind := raw_quote.kind
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 1 {
				t.Fatalf("agent requests = %#v, want one non-streaming request", provider.requests)
			}
			if got := provider.requests[0].Messages[0].Text; got != "research leia" {
				t.Fatalf("agent message = %q, want research leia", got)
			}
			if len(provider.requests[0].Tools) != 1 || provider.requests[0].Tools[0].Name != "lookup" {
				t.Fatalf("agent tools = %#v", provider.requests[0].Tools)
			}
			if len(provider.streamRequests) != 1 {
				t.Fatalf("stream requests = %#v, want one streaming turn", provider.streamRequests)
			}
			streamReq := provider.streamRequests[0]
			if !streamReq.Stream || streamReq.ForceTool != "lookup" || streamReq.Messages[0].Text != "stream leia" {
				t.Fatalf("stream request = %#v", streamReq)
			}
			if len(streamReq.Tools) != 1 || streamReq.Tools[0].Name != "lookup" || streamReq.Tools[0].Description != "Lookup docs." {
				t.Fatalf("stream tools = %#v", streamReq.Tools)
			}
			for name, want := range map[string]any{
				"agent_text": "agent-ok",
				"turn_text":  "stream-ok",
				"streamed":   "stream-ok",
				"quote_kind": "function",
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

func TestEvaluateStatementIsNoopOutsideEvaluateCLI(t *testing.T) {
	source := `
before := "kept"
evaluate "noop outside evaluate CLI" {
    before = "changed"
    missing_call()
}
after := before
`
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(tc.opts...)
			if err := vm.Exec(source); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			got, err := vm.Get("after")
			if err != nil {
				t.Fatalf("Get after: %v", err)
			}
			if got != "kept" {
				t.Fatalf("after = %#v, want evaluate block to be skipped", got)
			}
		})
	}
}

func TestAIDialectReplayExampleIsDeterministic(t *testing.T) {
	records, err := llm.LoadRecords(filepath.Join("..", "..", "examples", "ai", "coding_agent_replay.records.json"))
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
	vm := leia.New(
		leia.WithLibs(leia.LibString|leia.LibLLM|leia.LibDialect),
		leia.WithLLMReplay(records),
	)
	if err := vm.ExecFile(filepath.Join("..", "..", "examples", "ai", "coding_agent_replay.leia")); err != nil {
		t.Fatalf("ExecFile: %v", err)
	}
	text, _ := vm.Get("summary")
	if text != "Use read_file, search_text, apply_patch, and run_shell." {
		t.Fatalf("summary = %#v", text)
	}
}
