package gscript_test

import (
	"strings"
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

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
