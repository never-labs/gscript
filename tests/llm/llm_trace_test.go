package leia_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMTraceSinkReceivesTurnAndReactEvents(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: "turn", Usage: llm.TurnUsage{InputTokens: 1, OutputTokens: 2}},
				{
					Status: "tool_calls",
					Calls: []llm.ToolCall{{
						ID:   "call_1",
						Tool: "lookup",
						Args: map[string]any{"name": "leia"},
					}},
				},
				{Status: "final_answer", Text: "done", Usage: llm.TurnUsage{InputTokens: 3, OutputTokens: 4}},
			}}
			var events []llm.TraceEvent
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
				leia.WithLLMTrace(func(event llm.TraceEvent) {
					events = append(events, event)
				}),
			}, tc.opts...)
			vm := leia.New(opts...)
			if err := vm.Exec(`
turn_result, turn_err := llm.turn({
    model: "mock-fast",
    messages: [llm.user("hello")],
})
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: ["name"]})
react_result, react_err := llm.react({
    model: "mock-fast",
    messages: [llm.user("find docs")],
    tools: [lookup],
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

type streamingTraceProvider struct {
	usedStream bool
}

func (p *streamingTraceProvider) Turn(context.Context, llm.TurnRequest) (llm.TurnResult, error) {
	return llm.TurnResult{Status: "final_answer", Text: "fallback"}, nil
}

func (p *streamingTraceProvider) StreamTurn(_ context.Context, req llm.TurnRequest, sink llm.StreamSink) (llm.TurnResult, error) {
	p.usedStream = req.Stream
	for _, token := range []string{"hello", " ", "stream"} {
		if sink != nil {
			if err := sink(llm.StreamEvent{Type: "token", Token: token, Text: token}); err != nil {
				return llm.TurnResult{}, err
			}
		}
	}
	return llm.TurnResult{
		Status: "final_answer",
		Text:   "hello stream",
		Usage:  llm.TurnUsage{InputTokens: 1, OutputTokens: 3},
	}, nil
}

func TestLLMTraceSinkReceivesStreamingTokens(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &streamingTraceProvider{}
			var events []llm.TraceEvent
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
				leia.WithLLMTrace(func(event llm.TraceEvent) {
					events = append(events, event)
				}),
			}, tc.opts...)
			vm := leia.New(opts...)
			if err := vm.Exec(`
result, err := llm.turn({
    model: "mock-fast",
    messages: [llm.user("hello")],
    stream: true,
})
text := result.text
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			text, _ := vm.Get("text")
			if text != "hello stream" || !provider.usedStream {
				t.Fatalf("text=%#v usedStream=%v", text, provider.usedStream)
			}
			got := make([]string, 0, len(events))
			tokens := make([]string, 0, 3)
			for _, event := range events {
				got = append(got, event.Type)
				if event.Type == "turn_stream" {
					tokens = append(tokens, event.Token)
				}
			}
			want := []string{"turn_start", "turn_stream", "turn_stream", "turn_stream", "turn_end"}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("events = %#v, want %#v", got, want)
			}
			if strings.Join(tokens, "") != "hello stream" {
				t.Fatalf("tokens = %#v", tokens)
			}
		})
	}
}

func TestLLMTurnStreamsToScriptCallback(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &streamingTraceProvider{}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)
			if err := vm.Exec(`
streamed := ""
last_event := ""
result, err := llm.turn({
    model: "mock-fast",
    messages: [llm.user("hello")],
    on_stream: func(event) {
        streamed = streamed .. event.token
        last_event = event.type
    },
})
text := result.text
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			text, _ := vm.Get("text")
			streamed, _ := vm.Get("streamed")
			lastEvent, _ := vm.Get("last_event")
			if text != "hello stream" || streamed != "hello stream" || lastEvent != "token" || !provider.usedStream {
				t.Fatalf("text=%#v streamed=%#v last_event=%#v usedStream=%v", text, streamed, lastEvent, provider.usedStream)
			}
		})
	}
}

func TestLLMTurnRejectsNonFunctionStreamCallback(t *testing.T) {
	provider := &streamingTraceProvider{}
	vm := leia.New(
		leia.WithLibs(leia.LibString|leia.LibLLM),
		leia.WithLLMProvider(provider),
	)
	if err := vm.Exec(`
result, err := llm.turn({
    messages: [llm.user("hello")],
    on_stream: "not a function",
})
kind := err.kind
message := err.message
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	kind, _ := vm.Get("kind")
	message, _ := vm.Get("message")
	if kind != "validation" || !strings.Contains(fmt.Sprint(message), "on_stream") {
		t.Fatalf("kind=%#v message=%#v", kind, message)
	}
}

func TestLLMTraceRecorderHelper(t *testing.T) {
	provider := &mockLLMProvider{res: llm.TurnResult{
		Status: "final_answer",
		Text:   "done",
		Usage:  llm.TurnUsage{InputTokens: 1, OutputTokens: 2},
	}}
	recorder := llm.NewTraceRecorder(llm.TraceEvent{Type: "seed"})
	vm := leia.New(
		leia.WithLibs(leia.LibString|leia.LibLLM),
		leia.WithLLMProvider(provider),
		leia.WithLLMTrace(recorder.Record),
	)
	if err := vm.Exec(`
result, err := llm.turn({
    model: "mock-fast",
    messages: [llm.user("hello")],
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
