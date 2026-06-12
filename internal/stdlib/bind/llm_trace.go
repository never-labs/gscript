package bind

import (
	"fmt"
	"time"
)

func (b *llmLibBuilder) registerTraceHelpers() {
	traceEvent := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.trace_event' (table expected)")
		}
		return []Value{TableValue(llmTraceEventValue(args[0].Table()))}, nil
	}
	b.set("trace_event", traceEvent)
	b.set("traceEvent", traceEvent)

	traceEnvelope := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.trace_envelope' (events table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.trace_envelope' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmTraceEnvelopeValue(args[0].Table(), opts))}, nil
	}
	b.set("trace_envelope", traceEnvelope)
	b.set("traceEnvelope", traceEnvelope)
}

func llmTraceEventValue(src *Table) *Table {
	out := NewTable()
	out.RawSetString("schema_version", IntValue(int64(llmTraceInt(src, "schema_version", 1))))
	traceID := llmTraceString(src, "trace_id", "trace")
	sequence := llmTraceInt(src, "sequence", 1)
	eventType := llmTraceString(src, "event_type", llmTraceString(src, "type", "event"))
	status := llmTraceString(src, "status", "ok")
	eventID := llmTraceString(src, "event_id", fmt.Sprintf("%s:event:%d", traceID, sequence))

	out.RawSetString("trace_id", StringValue(traceID))
	out.RawSetString("event_id", StringValue(eventID))
	out.RawSetString("event_type", StringValue(eventType))
	out.RawSetString("type", StringValue(eventType))
	out.RawSetString("sequence", IntValue(int64(sequence)))
	out.RawSetString("timestamp_ms", IntValue(int64(llmTraceInt64(src, "timestamp_ms", time.Now().UTC().UnixMilli()))))
	out.RawSetString("status", StringValue(status))
	out.RawSetString("provider_free", BoolValue(llmTraceBool(src, "provider_free", true)))
	out.RawSetString("live_network", BoolValue(llmTraceBool(src, "live_network", false)))
	out.RawSetString("live_model", BoolValue(llmTraceBool(src, "live_model", false)))
	out.RawSetString("credentials_required", BoolValue(llmTraceBool(src, "credentials_required", false)))
	out.RawSetString("real_dependency_imports", BoolValue(llmTraceBool(src, "real_dependency_imports", false)))

	out.RawSetString("correlation", llmTraceCorrelationValue(src, traceID, eventID))
	out.RawSetString("redaction", llmTraceRedactionValue(src))
	out.RawSetString("replay", llmTraceReplayValue(src))
	out.RawSetString("payload", llmTracePayloadValue(src))
	return out
}

func llmTraceEnvelopeValue(events, opts *Table) *Table {
	out := NewTable()
	out.RawSetString("schema_version", IntValue(int64(llmTraceInt(opts, "schema_version", 1))))
	out.RawSetString("kind", StringValue(llmTraceString(opts, "kind", "generic_ai_trace_envelope")))
	out.RawSetString("trace_id", StringValue(llmTraceString(opts, "trace_id", "trace")))
	out.RawSetString("provider_free", BoolValue(llmTraceBool(opts, "provider_free", true)))
	out.RawSetString("live_network", BoolValue(llmTraceBool(opts, "live_network", false)))
	out.RawSetString("live_model", BoolValue(llmTraceBool(opts, "live_model", false)))
	out.RawSetString("credentials_required", BoolValue(llmTraceBool(opts, "credentials_required", false)))
	out.RawSetString("real_dependency_imports", BoolValue(llmTraceBool(opts, "real_dependency_imports", false)))
	out.RawSetString("trace_envelope_schema", llmTraceEnvelopeSchemaValue())
	out.RawSetString("events", llmTraceEventsValue(events, opts))
	out.RawSetString("redaction", llmTraceRedactionValue(opts))
	return out
}

func llmTraceEnvelopeSchemaValue() Value {
	schema := NewTable()
	schema.RawSetString("name", StringValue("generic_ai_trace_event"))
	schema.RawSetString("version", IntValue(1))

	required := NewSequentialArrayTable(0)
	for i, field := range []string{"schema_version", "trace_id", "event_id", "event_type", "timestamp_ms", "sequence", "status", "correlation", "payload"} {
		required.RawSet(IntValue(int64(i+1)), StringValue(field))
	}
	schema.RawSetString("required_fields", TableValue(required))

	correlation := NewSequentialArrayTable(0)
	for i, field := range []string{"trace_id", "event_id", "parent_event_id", "turn_id", "tool_call_id", "workflow_run_id", "workflow_step_id", "agent_run_id", "approval_id", "replay_session_id", "correlation_id"} {
		correlation.RawSet(IntValue(int64(i+1)), StringValue(field))
	}
	schema.RawSetString("correlation_id_fields", TableValue(correlation))
	return TableValue(schema)
}

func llmTraceEventsValue(events, opts *Table) Value {
	out := NewSequentialArrayTable(0)
	traceID := llmTraceString(opts, "trace_id", "trace")
	for i := 1; i <= events.Length(); i++ {
		event := events.RawGet(IntValue(int64(i)))
		if !event.IsTable() {
			continue
		}
		eventTable := llmCloneValue(event).Table()
		if eventTable.RawGetString("trace_id").IsNil() {
			eventTable.RawSetString("trace_id", StringValue(traceID))
		}
		if eventTable.RawGetString("sequence").IsNil() {
			eventTable.RawSetString("sequence", IntValue(int64(i)))
		}
		out.RawSet(IntValue(int64(out.Length()+1)), TableValue(llmTraceEventValue(eventTable)))
	}
	return TableValue(out)
}

func llmTraceCorrelationValue(src *Table, traceID, eventID string) Value {
	correlation := NewTable()
	if existing := src.RawGetString("correlation"); existing.IsTable() {
		correlation = llmCloneValue(existing).Table()
	}
	correlation.RawSetString("trace_id", StringValue(llmTraceString(correlation, "trace_id", traceID)))
	correlation.RawSetString("event_id", StringValue(llmTraceString(correlation, "event_id", eventID)))
	for _, field := range []string{
		"parent_event_id",
		"turn_id",
		"tool_call_id",
		"workflow_run_id",
		"workflow_step_id",
		"agent_run_id",
		"approval_id",
		"replay_session_id",
		"correlation_id",
	} {
		if value := src.RawGetString(field); !value.IsNil() && correlation.RawGetString(field).IsNil() {
			correlation.RawSetString(field, llmCloneValue(value))
		}
	}
	return TableValue(correlation)
}

func llmTraceRedactionValue(src *Table) Value {
	if redaction := src.RawGetString("redaction"); redaction.IsTable() {
		return llmCloneValue(redaction)
	}
	redaction := NewTable()
	redaction.RawSetString("policy", StringValue(llmTraceString(src, "redaction_policy", "no_raw_payload")))
	redaction.RawSetString("secret_values_present", BoolValue(false))
	redaction.RawSetString("raw_prompt_stored", BoolValue(false))
	redaction.RawSetString("raw_completion_stored", BoolValue(false))
	return TableValue(redaction)
}

func llmTraceReplayValue(src *Table) Value {
	if replay := src.RawGetString("replay"); replay.IsTable() {
		return llmCloneValue(replay)
	}
	replay := NewTable()
	replay.RawSetString("mode", StringValue(llmTraceString(src, "replay_mode", llmReplayModeFixture)))
	replay.RawSetString("provider_free", BoolValue(llmTraceBool(src, "provider_free", true)))
	replay.RawSetString("deterministic", BoolValue(llmTraceBool(src, "deterministic", true)))
	replay.RawSetString("created_from_provider", BoolValue(llmTraceBool(src, "created_from_provider", false)))
	for _, field := range []string{"replay_key", "request_hash", "response_hash", "record_id", "fixture_key"} {
		if value := src.RawGetString(field); !value.IsNil() {
			replay.RawSetString(field, llmCloneValue(value))
		}
	}
	return TableValue(replay)
}

func llmTracePayloadValue(src *Table) Value {
	if payload := src.RawGetString("payload"); payload.IsTable() {
		return llmCloneValue(payload)
	}
	return TableValue(NewTable())
}

func llmTraceString(t *Table, key, fallback string) string {
	if t == nil {
		return fallback
	}
	if value := t.RawGetString(key); value.IsString() && value.Str() != "" {
		return value.Str()
	}
	return fallback
}

func llmTraceBool(t *Table, key string, fallback bool) bool {
	if t == nil {
		return fallback
	}
	value := t.RawGetString(key)
	if value.IsNil() {
		return fallback
	}
	return value.Truthy()
}

func llmTraceInt(t *Table, key string, fallback int) int {
	return int(llmTraceInt64(t, key, int64(fallback)))
}

func llmTraceInt64(t *Table, key string, fallback int64) int64 {
	if t == nil {
		return fallback
	}
	if value := t.RawGetString(key); value.IsInt() {
		return value.Int()
	}
	return fallback
}
