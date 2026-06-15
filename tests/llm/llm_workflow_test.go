package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMWorkflowRunsStepsWithPriorTextAsInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: "draft about leia"},
				{Status: "final_answer", Text: "final: draft about leia"},
			}}
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)...)
			if err := vm.Exec(`
flow := llm.workflow({
    llm.step("draft", func(ctx) {
        return llm.turn({model: "mock-fast", messages: [llm.user(ctx.input)]})
    }),
    llm.step("revise", func(ctx) {
        return llm.turn({model: "mock-fast", messages: [llm.user(ctx.input)]})
    }),
})
result, err := flow.run("leia")
text := result.text
first_input := result.steps[1].input
second_input := result.steps[2].input
context_text := result.context.draft.text
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 2 {
				t.Fatalf("provider requests = %d, want 2", len(provider.requests))
			}
			if provider.requests[0].Messages[0].Text != "leia" || provider.requests[1].Messages[0].Text != "draft about leia" {
				t.Fatalf("request messages = %#v", provider.requests)
			}
			for name, want := range map[string]any{
				"text":         "final: draft about leia",
				"first_input":  "leia",
				"second_input": "draft about leia",
				"context_text": "draft about leia",
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

func TestLLMWorkflowSupportsProviderReplay(t *testing.T) {
	records := []llm.Record{
		{
			Request: llm.TurnRequest{
				Model:    "mock-fast",
				Messages: []llm.Message{{Role: "user", Text: "topic"}},
			},
			Result: llm.TurnResult{Status: "final_answer", Text: "draft"},
		},
		{
			Request: llm.TurnRequest{
				Model:    "mock-fast",
				Messages: []llm.Message{{Role: "user", Text: "draft"}},
			},
			Result: llm.TurnResult{Status: "final_answer", Text: "final"},
		},
	}
	vm := leia.New(leia.WithLibs(leia.LibString|leia.LibLLM), leia.WithLLMReplay(records))
	if err := vm.Exec(`
flow := llm.workflow({
    llm.step("draft", func(ctx) {
        return llm.turn({model: "mock-fast", messages: [llm.user(ctx.input)]})
    }),
    llm.step("final", func(ctx) {
        return llm.turn({model: "mock-fast", messages: [llm.user(ctx.input)]})
    }),
})
result, err := flow.run("topic")
text := result.text
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	text, _ := vm.Get("text")
	if text != "final" {
		t.Fatalf("text = %#v, want final", text)
	}
}

func TestLLMWorkflowMockSkipsProviderAndFeedsNextStep(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{}
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)...)
			if err := vm.Exec(`
flow := llm.workflow({
    llm.step("draft", func(ctx) {
        return llm.turn({messages: [llm.user("should not run")]})
    }),
    llm.step("review", func(ctx) {
        return {value: "reviewed " .. ctx.input, text: "reviewed " .. ctx.input}, nil
    }),
})
mocked := flow.mock({draft: {text: "mock draft"}})
result, err := mocked.run("topic")
text := result.text
draft_mocked := result.steps[1].mocked
review_input := result.steps[2].input
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 0 {
				t.Fatalf("provider requests = %d, want 0", len(provider.requests))
			}
			for name, want := range map[string]any{
				"text":         "reviewed mock draft",
				"draft_mocked": true,
				"review_input": "mock draft",
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

func TestLLMWorkflowCanCallAgentSteps(t *testing.T) {
	provider := &mockLLMProvider{results: []llm.TurnResult{
		{Status: "final_answer", Text: "agent draft"},
		{Status: "final_answer", Text: "agent final"},
	}}
	vm := leia.New(
		leia.WithLibs(leia.LibString|leia.LibLLM),
		leia.WithLLMProvider(provider),
	)
	if err := vm.Exec(`
writer := llm.agent("writer", func(topic) {
    return {model: "mock-fast", messages: [llm.user(topic)]}
})
flow := llm.workflow({
    llm.step("draft", func(ctx) {
        return writer(ctx.input)
    }),
    llm.step("final", func(ctx) {
        return writer(ctx.input)
    }),
})
result, err := flow.run("topic")
text := result.text
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	text, _ := vm.Get("text")
	if text != "agent final" {
		t.Fatalf("text = %#v, want agent final", text)
	}
}

func TestLLMStageAndWorkflowGraphRunSequentiallyWithMetadata(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
			}, tc.opts...)...)
			if err := vm.Exec(`
graph := llm.workflow_graph({
    workflow_id: "research-flow"
    entrypoint: "ai.workflow.orchestrate"
    stages: {
        llm.stage("plan", func(ctx) {
            return {value: "plan:" .. ctx.input, text: "plan:" .. ctx.input}, nil
        }, {
            capability: "generic.ai.workflow.orchestration.stage.plan"
            input_ref: "request"
            output_ref: "plan_result"
        }),
        llm.stage("finalize", func(ctx) {
            return {value: "final:" .. ctx.input, text: "final:" .. ctx.input}, nil
        }, {
            depends_on: {"plan"}
            capability: "generic.ai.workflow.orchestration.stage.finalize"
            input_ref: "plan_result"
            output_ref: "final_result"
        }),
    }
    edges: {
        {from: "plan", to: "finalize"}
    }
})
result, err := graph.run("topic")
text := result.text
workflow_id := graph.workflow_id
entrypoint := graph.entrypoint
graph_workflow_id := graph.graph.workflow_id
result_workflow_id := result.workflow_id
stage_count := #graph.graph.stages
edge_from := graph.graph.edges[1].from
edge_to := graph.graph.edges[1].to
first_stage := result.steps[1].stage
second_ctx_input := result.steps[2].input
second_dep := graph.graph.stages[2].depends_on[1]
second_capability := graph.graph.stages[2].capability
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"text":               "final:plan:topic",
				"workflow_id":        "research-flow",
				"entrypoint":         "ai.workflow.orchestrate",
				"graph_workflow_id":  "research-flow",
				"result_workflow_id": "research-flow",
				"stage_count":        int64(2),
				"edge_from":          "plan",
				"edge_to":            "finalize",
				"first_stage":        true,
				"second_ctx_input":   "plan:topic",
				"second_dep":         "plan",
				"second_capability":  "generic.ai.workflow.orchestration.stage.finalize",
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

func TestLLMWorkflowGraphReplayTraceEvidence(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{}
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)...)
			if err := vm.Exec(`
plan_record, plan_record_err := llm.replay_record({
    record_id: "rec-plan"
    replay_key: "turn:plan"
    request: {
        model: "fixture-model"
        messages: [llm.user("topic")]
    }
    response: {status: "final_answer" text: "plan text" calls: {} usage: {}}
})
final_record, final_record_err := llm.replay_record({
    record_id: "rec-final"
    replay_key: "turn:final"
    request: {
        model: "fixture-model"
        messages: [llm.user("plan text")]
    }
    response: {status: "final_answer" text: "final text" calls: {} usage: {}}
})
plan_fixture, plan_fixture_err := llm.replay_fixture({plan_record}, {
    fixture_id: "fixture:plan"
    consume_on_match: false
})
final_fixture, final_fixture_err := llm.replay_fixture({final_record}, {
    fixture_id: "fixture:final"
    consume_on_match: false
})

plan_match := plan_fixture.match({
    operation: "llm.turn"
    capability: "generic.ai.turn"
    replay_key: "turn:plan"
    request_hash: plan_record.request_hash
})
final_match := final_fixture.match({
    operation: "llm.turn"
    capability: "generic.ai.turn"
    replay_key: "turn:final"
    request_hash: final_record.request_hash
})
plan_event := llm.replay_trace_event(plan_match, {
    trace_id: "trace-workflow-replay"
    sequence: 1
    replay_session_id: "session-workflow"
    workflow_run_id: "wf-replay"
    workflow_step_id: "plan"
})
final_event := llm.replay_trace_event(final_match, {
    trace_id: "trace-workflow-replay"
    sequence: 2
    replay_session_id: "session-workflow"
    workflow_run_id: "wf-replay"
    workflow_step_id: "finalize"
})

graph := llm.workflow_graph({
    workflow_id: "wf-replay"
    entrypoint: "ai.workflow.replay"
    stages: {
        llm.stage("plan", func(ctx) {
            req := {model: "fixture-model" messages: [llm.user(ctx.input)]}
            replay, replay_err := plan_fixture.replay(req, "turn:plan")
            if replay_err != nil {
                return nil, replay_err
            }
            return llm.turn({model: "fixture-model" messages: [llm.user(ctx.input)] replay: replay})
        }, {
            capability: "generic.ai.workflow.stage.plan"
            fixture_key: "turn:plan"
            input_ref: "request.topic"
            output_ref: "plan.text"
            input_schema: "topic.v1"
            output_schema: "plan.v1"
        }),
        llm.stage("finalize", func(ctx) {
            req := {model: "fixture-model" messages: [llm.user(ctx.input)]}
            replay, replay_err := final_fixture.replay(req, "turn:final")
            if replay_err != nil {
                return nil, replay_err
            }
            return llm.turn({model: "fixture-model" messages: [llm.user(ctx.input)] replay: replay})
        }, {
            depends_on: {"plan"}
            capability: "generic.ai.workflow.stage.finalize"
            fixture_key: "turn:final"
            input_ref: "plan.text"
            output_ref: "final.text"
            input_schema: "plan.v1"
            output_schema: "final.v1"
        }),
    }
    edges: [{from: "plan", to: "finalize"}]
})
result, err := graph.run("topic")
envelope := llm.trace_envelope([plan_event, final_event], {
    trace_id: "trace-workflow-replay"
})
trace_summary := llm.trace_summary(envelope)
trace_gate := llm.trace_assert(envelope, {
    require_provider_free: true
    deny_live_network: true
    required_event_types: ["replay_record_matched"]
    require_correlation_fields: ["replay_session_id", "workflow_run_id", "workflow_step_id"]
})

setup_ok := plan_record_err == nil && final_record_err == nil && plan_fixture_err == nil && final_fixture_err == nil
run_err_nil := err == nil
text := result.text
workflow_id := result.workflow_id
first_capability := result.steps[1].trace.metadata.capability
first_fixture_key := result.steps[1].trace.metadata.fixture_key
first_input_ref := result.steps[1].trace.metadata.input_ref
first_output_ref := result.steps[1].trace.metadata.output_ref
first_input_schema := result.steps[1].trace.metadata.input_schema
first_output_schema := result.steps[1].trace.metadata.output_schema
second_dep := result.steps[2].trace.metadata.depends_on[1]
second_capability := result.steps[2].trace.metadata.capability
graph_second_fixture := result.graph.stages[2].fixture_key
plan_event_step := plan_event.correlation.workflow_step_id
final_event_step := final_event.correlation.workflow_step_id
trace_events := trace_summary.events
trace_replay_key := trace_summary.replay_keys[1]
trace_gate_ok := trace_gate.ok
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 0 {
				t.Fatalf("provider requests = %d, want replay-only workflow", len(provider.requests))
			}
			for name, want := range map[string]any{
				"setup_ok":             true,
				"run_err_nil":          true,
				"text":                 "final text",
				"workflow_id":          "wf-replay",
				"first_capability":     "generic.ai.workflow.stage.plan",
				"first_fixture_key":    "turn:plan",
				"first_input_ref":      "request.topic",
				"first_output_ref":     "plan.text",
				"first_input_schema":   "topic.v1",
				"first_output_schema":  "plan.v1",
				"second_dep":           "plan",
				"second_capability":    "generic.ai.workflow.stage.finalize",
				"graph_second_fixture": "turn:final",
				"plan_event_step":      "plan",
				"final_event_step":     "finalize",
				"trace_events":         int64(2),
				"trace_replay_key":     "turn:plan",
				"trace_gate_ok":        true,
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

func TestLLMWorkflowGraphMockPreservesGraphMetadata(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
			}, tc.opts...)...)
			if err := vm.Exec(`
graph := llm.workflow_graph({
    workflow_id: "mockable-flow"
    stages: {
        llm.stage("plan", func(ctx) {
            return {text: "should not run"}, nil
        }),
        llm.stage("finalize", func(ctx) {
            return {text: "final " .. ctx.input}, nil
        }, {depends_on: {"plan"}}),
    }
})
mocked := graph.mock({plan: {text: "mock plan"}})
result, err := mocked.run("topic")
text := result.text
mocked_graph_id := mocked.graph.workflow_id
result_graph_id := result.graph.workflow_id
plan_mocked := result.steps[1].mocked
final_input := result.steps[2].input
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"text":            "final mock plan",
				"mocked_graph_id": "mockable-flow",
				"result_graph_id": "mockable-flow",
				"plan_mocked":     true,
				"final_input":     "mock plan",
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

func TestLLMWorkflowMockCanResolveStageFixtureKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{}
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)...)
			if err := vm.Exec(`
graph := llm.workflow_graph({
    workflow_id: "fixture-key-flow"
    stages: {
        llm.stage("plan", func(ctx) {
            return llm.turn({messages: [llm.user("should not run")]})
        }, {
            fixture_key: "turn:plan"
            capability: "generic.ai.workflow.stage.plan"
        }),
        llm.stage("finalize", func(ctx) {
            return {text: "final " .. ctx.input}, nil
        }, {
            depends_on: {"plan"}
            fixture_key: "turn:final"
            capability: "generic.ai.workflow.stage.finalize"
        }),
    }
})
fixtures := {}
fixtures["turn:plan"] = {text: "fixture plan"}
mocked := graph.mock(fixtures)
result, err := mocked.run("topic")
text := result.text
plan_mocked := result.steps[1].mocked
plan_fixture_key := result.steps[1].trace.metadata.fixture_key
final_input := result.steps[2].input
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 0 {
				t.Fatalf("provider requests = %d, want 0", len(provider.requests))
			}
			for name, want := range map[string]any{
				"text":             "final fixture plan",
				"plan_mocked":      true,
				"plan_fixture_key": "turn:plan",
				"final_input":      "fixture plan",
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

func TestLLMWorkflowMockPrefersFixtureKeyThenStageNameThenIndex(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
			}, tc.opts...)...)
			if err := vm.Exec(`
flow := llm.workflow({
    llm.stage("named", func(ctx) {
        return {text: "live named"}, nil
    }, {fixture_key: "fixture:named"}),
    llm.stage("keyed", func(ctx) {
        return {text: "live keyed"}, nil
    }, {fixture_key: "fixture:keyed"}),
    llm.stage("indexed", func(ctx) {
        return {text: "live indexed"}, nil
    }),
})
fixtures := {
    named: {text: "by name"}
}
fixtures["fixture:named"] = {text: "by key should win"}
fixtures["fixture:keyed"] = {text: "by key"}
fixtures[1] = {text: "index one"}
fixtures[2] = {text: "index two"}
fixtures[3] = {text: "by index"}
mocked := flow.mock(fixtures)
result, err := mocked.run("topic")
first_text := result.steps[1].text
second_text := result.steps[2].text
third_text := result.steps[3].text
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"first_text":  "by key should win",
				"second_text": "by key",
				"third_text":  "by index",
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
