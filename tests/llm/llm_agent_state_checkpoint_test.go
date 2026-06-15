package leia_test

import (
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestLLMAgentStateCheckpointProjectsRefOnlyTraceEvent(t *testing.T) {
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
state := {
    agent_run_id: "agent-run-1"
    session_id: "session-1"
    state_version: 2
    status: "paused"
    trace_id: "trace-state"
    turn_id: "turn-2"
    parent_event_id: "event-parent"
    raw_input: "secret prompt text"
    raw_output: "secret completion text"
    input_refs: {
        {ref_id: "input-1", kind: "prompt_ref", digest: "sha256:input", summary: "prompt ref", raw_value_stored: false}
    }
    output_refs: {
        {ref_id: "output-1", kind: "answer_ref", digest: "sha256:output", summary: "answer ref", raw_value_stored: false}
    }
    memory_refs: {
        {ref_id: "memory-1", kind: "context_ref", digest: "sha256:memory", summary: "context ref", raw_value_stored: false}
    }
}
checkpoint := llm.agent_state_checkpoint(state, {
    workflow_run_id: "wf-state"
    workflow_step_id: "step-state"
    component: "state-test"
})
event := llm.agent_state_checkpoint_event(checkpoint, {
    trace_id: "trace-state"
    sequence: 1
})
envelope := llm.trace_envelope([event], {trace_id: "trace-state"})
gate := llm.trace_assert(envelope, {
    required_event_types: ["agent_state_checkpoint"]
    require_event_payload_fields: {agent_state_checkpoint: ["agent_run_id", "session_id", "state_version", "checkpoint_key", "cache_key", "resume_token"]}
    require_correlation_fields: ["agent_run_id", "turn_id", "workflow_run_id", "workflow_step_id", "correlation_id"]
    max_status_counts: {paused: 1}
    deny_secret_values_present: true
    deny_raw_prompt_stored: true
    deny_raw_completion_stored: true
})
checkpoint_kind := checkpoint.kind
checkpoint_schema := checkpoint.schema_version
checkpoint_status := checkpoint.status
checkpoint_result_status := checkpoint.result_status
checkpoint_agent := checkpoint.agent_run_id
checkpoint_session := checkpoint.session_id
checkpoint_version := checkpoint.state_version
checkpoint_key := checkpoint.checkpoint_key
checkpoint_cache_key := checkpoint.cache_key
checkpoint_resume_token := checkpoint.resume_token
checkpoint_algorithm := checkpoint.checkpoint.key_algorithm
checkpoint_stable := checkpoint.checkpoint.stable_across_replay
checkpoint_input_ref := checkpoint.input_refs[1].ref_id
checkpoint_output_ref := checkpoint.output_refs[1].ref_id
checkpoint_memory_ref := checkpoint.memory_refs[1].ref_id
checkpoint_raw_input_missing := checkpoint.raw_input == nil
checkpoint_raw_output_missing := checkpoint.raw_output == nil
checkpoint_redaction_policy := checkpoint.redaction.policy
checkpoint_raw_inputs_stored := checkpoint.redaction.raw_inputs_stored
checkpoint_raw_outputs_stored := checkpoint.redaction.raw_outputs_stored
checkpoint_trace_session := checkpoint.trace_correlation.session_id
checkpoint_trace_checkpoint := checkpoint.trace_correlation.checkpoint_key
event_type := event.event_type
event_status := event.status
event_payload_checkpoint := event.payload.checkpoint_key
event_payload_cache := event.payload.cache_key
event_payload_trace_session := event.payload.trace_correlation.session_id
event_payload_raw_input_missing := event.payload.raw_input == nil
event_payload_raw_output_missing := event.payload.raw_output == nil
event_redaction_policy := event.redaction.policy
event_correlation_agent := event.correlation.agent_run_id
event_correlation_session := event.correlation.session_id
event_correlation_checkpoint := event.correlation.checkpoint_key
event_correlation_turn := event.correlation.turn_id
event_correlation_id := event.correlation.correlation_id
event_replay_mode := event.replay.mode
event_replay_provider_free := event.replay.provider_free
gate_ok := gate.ok
gate_status := gate.status
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"checkpoint_kind":                  "agent_state_checkpoint",
				"checkpoint_schema":                int64(1),
				"checkpoint_status":                "paused",
				"checkpoint_result_status":         "ok",
				"checkpoint_agent":                 "agent-run-1",
				"checkpoint_session":               "session-1",
				"checkpoint_version":               int64(2),
				"checkpoint_algorithm":             "sha256",
				"checkpoint_stable":                true,
				"checkpoint_input_ref":             "input-1",
				"checkpoint_output_ref":            "output-1",
				"checkpoint_memory_ref":            "memory-1",
				"checkpoint_raw_input_missing":     true,
				"checkpoint_raw_output_missing":    true,
				"checkpoint_redaction_policy":      "agent_state_checkpoint_ref_only",
				"checkpoint_raw_inputs_stored":     false,
				"checkpoint_raw_outputs_stored":    false,
				"checkpoint_trace_session":         "session-1",
				"event_type":                       "agent_state_checkpoint",
				"event_status":                     "paused",
				"event_payload_trace_session":      "session-1",
				"event_payload_raw_input_missing":  true,
				"event_payload_raw_output_missing": true,
				"event_redaction_policy":           "agent_state_checkpoint_ref_only",
				"event_correlation_agent":          "agent-run-1",
				"event_correlation_session":        "session-1",
				"event_correlation_turn":           "turn-2",
				"event_replay_mode":                "fixture_replay",
				"event_replay_provider_free":       true,
				"gate_ok":                          true,
				"gate_status":                      "ok",
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
			for _, name := range []string{"checkpoint_key", "checkpoint_cache_key", "checkpoint_resume_token", "checkpoint_trace_checkpoint", "event_payload_checkpoint", "event_payload_cache", "event_correlation_checkpoint", "event_correlation_id"} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				s, ok := got.(string)
				if !ok || !strings.HasPrefix(s, "sha256:") && name != "checkpoint_resume_token" {
					t.Fatalf("%s = %#v, want sha256 string", name, got)
				}
				if name == "checkpoint_resume_token" && !strings.HasPrefix(s, "checkpoint:sha256:") {
					t.Fatalf("%s = %#v, want checkpoint:sha256 string", name, got)
				}
			}
		})
	}
}
