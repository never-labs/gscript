package bind

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func (b *llmLibBuilder) registerAgentStateCheckpointHelpers() {
	checkpoint := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.agent_state_checkpoint' (state table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.agent_state_checkpoint' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmAgentStateCheckpointValue(args[0].Table(), opts))}, nil
	}
	b.set("agent_state_checkpoint", checkpoint)
	b.set("agentStateCheckpoint", checkpoint)
	b.set("state_checkpoint", checkpoint)
	b.set("stateCheckpoint", checkpoint)
}

func llmAgentStateCheckpointValue(state, opts *Table) *Table {
	out := NewTable()
	out.RawSetString("kind", StringValue("agent_state_checkpoint"))
	out.RawSetString("schema_version", IntValue(1))
	out.RawSetString("version", StringValue("agent_state_checkpoint.v1"))
	out.RawSetString("provider_free", BoolValue(true))
	out.RawSetString("status", StringValue(llmTraceString(state, "status", "checkpointed")))
	out.RawSetString("result_status", StringValue(llmTraceString(state, "result_status", "ok")))

	for _, field := range []string{
		"agent_run_id",
		"session_id",
		"state_version",
		"turn_id",
		"trace_id",
		"event_id",
		"parent_event_id",
		"workflow_run_id",
		"workflow_step_id",
		"correlation_id",
		"replay_session_id",
		"operation",
		"component",
	} {
		if value := opts.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		} else if value := state.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		}
	}
	for _, field := range []string{"input_refs", "output_refs", "memory_refs"} {
		if value := state.RawGetString(field); value.IsTable() {
			out.RawSetString(field, llmCloneValue(value))
		}
	}
	checkpointKey := llmTraceString(opts, "checkpoint_key", llmTraceString(state, "checkpoint_key", ""))
	if checkpointKey == "" {
		checkpointKey = llmAgentStateCheckpointHash("checkpoint", out)
	}
	cacheKey := llmTraceString(opts, "cache_key", llmTraceString(state, "cache_key", ""))
	if cacheKey == "" {
		cacheKey = llmAgentStateCheckpointHash("cache", out)
	}
	out.RawSetString("checkpoint_key", StringValue(checkpointKey))
	out.RawSetString("cache_key", StringValue(cacheKey))
	out.RawSetString("resume_token", StringValue(llmTraceString(opts, "resume_token", llmTraceString(state, "resume_token", "checkpoint:"+checkpointKey))))
	out.RawSetString("checkpoint", TableValue(llmAgentStateCheckpointMeta(checkpointKey, cacheKey)))
	out.RawSetString("redaction", TableValue(llmAgentStateCheckpointRedaction(state, opts)))
	out.RawSetString("trace_correlation", TableValue(llmAgentStateCheckpointTraceCorrelation(out)))
	return out
}

func llmAgentStateCheckpointMeta(checkpointKey, cacheKey string) *Table {
	meta := NewTable()
	meta.RawSetString("checkpoint_key", StringValue(checkpointKey))
	meta.RawSetString("cache_key", StringValue(cacheKey))
	meta.RawSetString("key_algorithm", StringValue("sha256"))
	meta.RawSetString("cache_key_algorithm", StringValue("sha256"))
	meta.RawSetString("stable_across_replay", BoolValue(true))
	fields := NewSequentialArrayTable(0)
	for _, field := range []string{"agent_run_id", "session_id", "state_version", "turn_id", "input_ref_ids", "output_ref_ids", "memory_ref_ids"} {
		fields.RawSet(IntValue(int64(fields.Length()+1)), StringValue(field))
	}
	meta.RawSetString("key_material_fields", TableValue(fields))
	excluded := NewSequentialArrayTable(0)
	for _, field := range []string{"raw_input", "raw_output", "raw_prompt", "raw_completion", "secret", "credentials", "token", "authorization", "cookie", "api_key", "access_token", "refresh_token", "password"} {
		excluded.RawSet(IntValue(int64(excluded.Length()+1)), StringValue(field))
	}
	meta.RawSetString("excluded_fields", TableValue(excluded))
	return meta
}

func llmAgentStateCheckpointTraceCorrelation(checkpoint *Table) *Table {
	correlation := NewTable()
	for _, field := range []string{"trace_id", "event_id", "parent_event_id", "agent_run_id", "session_id", "state_version", "turn_id", "workflow_run_id", "workflow_step_id", "replay_session_id", "checkpoint_key", "cache_key"} {
		if value := checkpoint.RawGetString(field); !value.IsNil() {
			correlation.RawSetString(field, llmCloneValue(value))
		}
	}
	return correlation
}

func llmAgentStateCheckpointRedaction(state, opts *Table) *Table {
	redaction := NewTable()
	if existing := state.RawGetString("redaction"); existing.IsTable() {
		redaction = llmCloneValue(existing).Table()
	}
	if existing := opts.RawGetString("redaction"); existing.IsTable() {
		llmCopyTable(redaction, existing.Table(), true)
	}
	redaction.RawSetString("enabled", BoolValue(true))
	redaction.RawSetString("secret_values_present", BoolValue(false))
	redaction.RawSetString("raw_prompt_stored", BoolValue(false))
	redaction.RawSetString("raw_completion_stored", BoolValue(false))
	redaction.RawSetString("raw_inputs_stored", BoolValue(false))
	redaction.RawSetString("raw_outputs_stored", BoolValue(false))
	redaction.RawSetString("policy", StringValue("agent_state_checkpoint_ref_only"))
	return redaction
}

func llmAgentStateCheckpointHash(prefix string, state *Table) string {
	var b strings.Builder
	b.WriteString(prefix)
	for _, field := range []string{"agent_run_id", "session_id", "state_version", "turn_id", "trace_id", "workflow_run_id", "workflow_step_id"} {
		b.WriteString("|")
		b.WriteString(field)
		b.WriteString("=")
		b.WriteString(state.RawGetString(field).Str())
	}
	for _, field := range []string{"input_refs", "output_refs", "memory_refs"} {
		b.WriteString("|")
		b.WriteString(field)
		b.WriteString("=")
		b.WriteString(llmAgentStateCheckpointRefMaterial(state.RawGetString(field)))
	}
	sum := sha256.Sum256([]byte(b.String()))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func llmAgentStateCheckpointRefMaterial(value Value) string {
	if !value.IsTable() {
		return ""
	}
	var b strings.Builder
	table := value.Table()
	for i := 1; i <= table.Length(); i++ {
		item := table.RawGet(IntValue(int64(i)))
		if !item.IsTable() {
			continue
		}
		ref := item.Table()
		for _, field := range []string{"ref_id", "id", "kind", "digest", "summary", "raw_value_stored"} {
			b.WriteString(field)
			b.WriteString(":")
			b.WriteString(ref.RawGetString(field).Str())
			b.WriteString(";")
		}
	}
	return b.String()
}
