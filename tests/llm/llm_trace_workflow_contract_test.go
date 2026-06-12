package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
)

func TestLLMWorkflowTraceContractIncludesStepErrorMetadata(t *testing.T) {
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
    llm.step("collect", func(ctx) {
        return {value: "draft", text: "draft"}, nil
    }),
    llm.step("spend", func(ctx) {
        return nil, {
            kind: "budget"
            dimension: "tokens"
            limit: 5
            used: 6
            message: "budget exceeded"
        }
    }),
    llm.step("unused", func(ctx) {
        return "unused", nil
    }),
})
result, err := flow.run("topic")
status := result.status
err_kind := err.kind
workflow_trace_type := result.trace.type
workflow_trace_status := result.trace.status
child_count := #result.trace.children
first_step_name := result.trace.children[1].name
first_step_status := result.trace.children[1].status
first_step_parent := result.trace.children[1].parent.type
second_step_name := result.steps[2].trace.name
second_step_status := result.steps[2].trace.status
second_step_error_kind := result.steps[2].trace.error.kind
second_step_budget_dimension := result.steps[2].trace.budget.dimension
second_step_metadata_index := result.steps[2].trace.metadata.index
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"status":                       "error",
				"err_kind":                     "budget",
				"workflow_trace_type":          "workflow",
				"workflow_trace_status":        "error",
				"child_count":                  int64(2),
				"first_step_name":              "collect",
				"first_step_status":            "ok",
				"first_step_parent":            "workflow",
				"second_step_name":             "spend",
				"second_step_status":           "error",
				"second_step_error_kind":       "budget",
				"second_step_budget_dimension": "tokens",
				"second_step_metadata_index":   int64(2),
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

func TestLLMAgentAsToolTraceContractExposesHandoffParentChild(t *testing.T) {
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
func specialist_config(topic) {
    return {model: "mock", user: topic}, nil
}

func specialist_flow(topic) {
    return {
        status: "done"
        value: {
            summary: "checked " .. topic
        }
        text: "checked " .. topic
    }, nil
}

specialist := llm.agent("specialist", specialist_config, specialist_flow, {params: {"topic"}})
handoff := llm.handoff(specialist, {name: "handoff_specialist"})
value, err := llm.dispatch({
    id: "call_handoff"
    tool: "handoff_specialist"
    args: {topic: "trace"}
}, {handoff})
summary := value.summary
tool_contract := handoff.trace_contract
trace_type := value.trace.type
trace_name := value.trace.name
trace_status := value.trace.status
child_type := value.trace.children[1].type
child_name := value.trace.children[1].name
child_parent_name := value.trace.children[1].parent.name
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"summary":           "checked trace",
				"tool_contract":     "agent_tool.v1",
				"trace_type":        "agent_tool",
				"trace_name":        "handoff_specialist",
				"trace_status":      "done",
				"child_type":        "agent",
				"child_name":        "specialist",
				"child_parent_name": "handoff_specialist",
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

func TestLLMTraceEventEnvelopeHelpers(t *testing.T) {
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
event := llm.trace_event({
    trace_id: "trace-1"
    event_id: "event-2"
    event_type: "tool_call"
    sequence: 2
    timestamp_ms: 12345
    status: "ok"
    turn_id: "turn-1"
    tool_call_id: "call-1"
    workflow_run_id: "wf-1"
    replay_session_id: "session-1"
    replay_key: "turn:1"
    request_hash: "sha256:req"
    response_hash: "sha256:res"
    capability: "generic.tool.invoke"
    payload: {
        tool_name: "lookup"
        args_schema: "redacted"
    }
})
envelope := llm.trace_envelope({event}, {
    trace_id: "trace-1"
    kind: "generic_ai_trace_envelope"
})

event_schema := event.schema_version
event_type := event.event_type
event_type_alias := event.type
event_provider_free := event.provider_free
event_live_network := event.live_network
event_correlation_trace := event.correlation.trace_id
event_correlation_tool := event.correlation.tool_call_id
event_replay_mode := event.replay.mode
event_replay_key := event.replay.replay_key
event_payload_tool := event.payload.tool_name
event_redaction_secret := event.redaction.secret_values_present
envelope_kind := envelope.kind
envelope_trace := envelope.trace_id
envelope_provider_free := envelope.provider_free
envelope_events := #envelope.events
envelope_event_id := envelope.events[1].event_id
envelope_event_sequence := envelope.events[1].sequence
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"event_schema":            int64(1),
				"event_type":              "tool_call",
				"event_type_alias":        "tool_call",
				"event_provider_free":     true,
				"event_live_network":      false,
				"event_correlation_trace": "trace-1",
				"event_correlation_tool":  "call-1",
				"event_replay_mode":       "fixture_replay",
				"event_replay_key":        "turn:1",
				"event_payload_tool":      "lookup",
				"event_redaction_secret":  false,
				"envelope_kind":           "generic_ai_trace_envelope",
				"envelope_trace":          "trace-1",
				"envelope_provider_free":  true,
				"envelope_events":         int64(1),
				"envelope_event_id":       "event-2",
				"envelope_event_sequence": int64(2),
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

func TestLLMTraceSummaryAndAssertHelpers(t *testing.T) {
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
start := llm.trace_event({
    trace_id: "trace-summary"
    event_id: "event-1"
    event_type: "turn_start"
    sequence: 1
    timestamp_ms: 100
    status: "ok"
    turn_id: "turn-1"
    replay_session_id: "session-1"
    replay_key: "turn:1"
})
done := llm.trace_event({
    trace_id: "trace-summary"
    event_id: "event-2"
    event_type: "turn_end"
    sequence: 2
    timestamp_ms: 110
    status: "error"
    turn_id: "turn-1"
    replay_session_id: "session-1"
    replay_key: "turn:1"
})
envelope := llm.trace_envelope({start, done}, {
    trace_id: "trace-summary"
    provider_free: true
    live_network: false
    live_model: false
})
summary := llm.trace_summary(envelope)
check := llm.trace_assert(envelope, {
    require_provider_free: true
    deny_live_network: true
    deny_live_model: true
    required_event_types: {"turn_start", "turn_end"}
    require_correlation_fields: {"turn_id", "replay_session_id"}
})
bad := llm.trace_assert(envelope, {
    required_event_types: {"tool_call"}
    require_correlation_fields: {"tool_call_id"}
})

event_count := summary.events
first_type := summary.event_types[1]
second_type := summary.event_types[2]
ok_count := summary.status_counts.ok
error_count := summary.status_counts.error
replay_key := summary.replay_keys[1]
missing_correlation := summary.missing_correlation
sequence_gaps := summary.sequence_gaps
non_monotonic := summary.non_monotonic_timestamps
check_ok := check.ok
check_status := check.status
bad_ok := bad.ok
bad_status := bad.status
bad_findings := #bad.findings
bad_first_kind := bad.findings[1].kind
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"event_count":         int64(2),
				"first_type":          "turn_start",
				"second_type":         "turn_end",
				"ok_count":            int64(1),
				"error_count":         int64(1),
				"replay_key":          "turn:1",
				"missing_correlation": int64(0),
				"sequence_gaps":       int64(0),
				"non_monotonic":       int64(0),
				"check_ok":            true,
				"check_status":        "ok",
				"bad_ok":              false,
				"bad_status":          "failed",
				"bad_findings":        int64(3),
				"bad_first_kind":      "generic.ai.trace.missing_event_type",
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
