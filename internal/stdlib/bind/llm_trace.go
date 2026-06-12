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

	traceSummary := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.trace_summary' (trace envelope or events table expected)")
		}
		return []Value{TableValue(llmTraceSummaryValue(args[0].Table()))}, nil
	}
	b.set("trace_summary", traceSummary)
	b.set("traceSummary", traceSummary)

	traceAssert := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.trace_assert' (trace envelope or events table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.trace_assert' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmTraceAssertValue(args[0].Table(), opts))}, nil
	}
	b.set("trace_assert", traceAssert)
	b.set("traceAssert", traceAssert)

	replayTraceEvent := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.replay_trace_event' (replay match table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.replay_trace_event' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmReplayTraceEventValue(args[0].Table(), opts))}, nil
	}
	b.set("replay_trace_event", replayTraceEvent)
	b.set("replayTraceEvent", replayTraceEvent)

	approvalTraceEvent := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.approval_trace_event' (approval trace table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.approval_trace_event' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmApprovalTraceEventValue(args[0].Table(), opts))}, nil
	}
	b.set("approval_trace_event", approvalTraceEvent)
	b.set("approvalTraceEvent", approvalTraceEvent)

	policyOutcomeEvent := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.policy_outcome_event' (policy outcome table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.policy_outcome_event' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmPolicyOutcomeTraceEventValue(args[0].Table(), opts))}, nil
	}
	b.set("policy_outcome_event", policyOutcomeEvent)
	b.set("policyOutcomeEvent", policyOutcomeEvent)
	b.set("policy_outcome_trace_event", policyOutcomeEvent)
	b.set("policyOutcomeTraceEvent", policyOutcomeEvent)
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

func llmReplayTraceEventValue(match, opts *Table) *Table {
	status := llmTraceString(match, "status", "mismatch")
	eventType := llmReplayTraceEventType(status)
	src := NewTable()
	for _, key := range opts.PairsKeysSnapshot() {
		src.RawSet(key, llmCloneValue(opts.RawGet(key)))
	}
	src.RawSetString("event_type", StringValue(llmTraceString(opts, "event_type", eventType)))
	src.RawSetString("status", StringValue(status))
	src.RawSetString("provider_free", BoolValue(llmTraceBool(opts, "provider_free", true)))
	src.RawSetString("live_network", BoolValue(llmTraceBool(opts, "live_network", false)))
	src.RawSetString("live_model", BoolValue(llmTraceBool(opts, "live_model", false)))

	replay := NewTable()
	replay.RawSetString("mode", StringValue(llmTraceString(opts, "replay_mode", llmReplayModeFixture)))
	replay.RawSetString("provider_free", BoolValue(true))
	replay.RawSetString("deterministic", BoolValue(true))
	replay.RawSetString("created_from_provider", BoolValue(false))
	payload := NewTable()
	payload.RawSetString("ok", llmCloneValue(match.RawGetString("ok")))
	payload.RawSetString("status", StringValue(status))
	payload.RawSetString("next_index", llmCloneValue(match.RawGetString("next_index")))
	if finding := match.RawGetString("finding_kind"); !finding.IsNil() {
		payload.RawSetString("finding_kind", llmCloneValue(finding))
	}
	if message := match.RawGetString("message"); !message.IsNil() {
		payload.RawSetString("message", StringValue(llmReplayTraceSafeMessage(message.Str())))
	}
	llmReplayTraceCopySummary(payload, match.RawGetString("summary"))
	llmReplayTraceCopyIdentity(match, replay, payload)
	llmReplayTraceCopyCorrelation(src, replay, payload)
	src.RawSetString("replay", TableValue(replay))
	src.RawSetString("payload", TableValue(payload))
	return llmTraceEventValue(src)
}

func llmReplayTraceEventType(status string) string {
	switch status {
	case "matched":
		return "replay_record_matched"
	case "exhausted":
		return "replay_record_exhausted"
	default:
		return "replay_record_mismatch"
	}
}

func llmReplayTraceSafeMessage(message string) string {
	const prefix = "replay identity mismatch on "
	if len(message) > len(prefix) && message[:len(prefix)] == prefix {
		field := message[len(prefix):]
		for i, r := range field {
			if r == ':' {
				field = field[:i]
				break
			}
		}
		if field != "" {
			return prefix + field
		}
	}
	if message == "" {
		return "replay trace event"
	}
	return "replay trace event"
}

func llmReplayTraceCopySummary(payload *Table, summary Value) {
	if !summary.IsTable() {
		return
	}
	src := summary.Table()
	out := NewTable()
	for _, field := range []string{"fixture_id", "strategy", "loaded_records", "requests", "matched", "mismatches", "exhausted", "unconsumed", "next_index"} {
		if value := src.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		}
	}
	if value := src.RawGetString("finding_kinds"); !value.IsNil() {
		out.RawSetString("finding_kinds", llmCloneValue(value))
	}
	if value := src.RawGetString("matched_record_ids"); !value.IsNil() {
		out.RawSetString("matched_record_ids", llmCloneValue(value))
	}
	payload.RawSetString("summary", TableValue(out))
}

func llmReplayTraceCopyIdentity(match, replay, payload *Table) {
	record := match.RawGetString("record")
	request := match.RawGetString("request")
	if record.IsTable() {
		recordTable := record.Table()
		for _, field := range []string{"record_id", "replay_key", "request_hash", "response_hash", "fixture_key"} {
			llmReplayTraceCopyField(recordTable, replay, field)
		}
		if replayValue := recordTable.RawGetString("replay"); replayValue.IsTable() {
			for _, field := range []string{"replay_key", "request_hash", "response_hash"} {
				llmReplayTraceCopyField(replayValue.Table(), replay, field)
			}
		}
		if recordID := recordTable.RawGetString("record_id"); !recordID.IsNil() {
			payload.RawSetString("record_id", llmCloneValue(recordID))
		}
	}
	if request.IsTable() {
		requestTable := request.Table()
		for _, field := range []string{"replay_key", "request_hash"} {
			if replay.RawGetString(field).IsNil() {
				llmReplayTraceCopyField(requestTable, replay, field)
			}
		}
	}
}

func llmReplayTraceCopyCorrelation(src, replay, payload *Table) {
	if src.RawGetString("replay_session_id").IsNil() {
		if summary := payload.RawGetString("summary"); summary.IsTable() {
			if fixtureID := summary.Table().RawGetString("fixture_id"); !fixtureID.IsNil() {
				src.RawSetString("replay_session_id", llmCloneValue(fixtureID))
			}
		}
	}
	if replayKey := replay.RawGetString("replay_key"); !replayKey.IsNil() {
		if src.RawGetString("turn_id").IsNil() {
			src.RawSetString("turn_id", llmCloneValue(replayKey))
		}
		if src.RawGetString("correlation_id").IsNil() {
			src.RawSetString("correlation_id", llmCloneValue(replayKey))
		}
	}
}

func llmReplayTraceCopyField(src, dst *Table, field string) {
	if value := src.RawGetString(field); !value.IsNil() {
		dst.RawSetString(field, llmCloneValue(value))
	}
}

func llmApprovalTraceEventValue(trace, opts *Table) *Table {
	src := NewTable()
	for _, key := range opts.PairsKeysSnapshot() {
		src.RawSet(key, llmCloneValue(opts.RawGet(key)))
	}
	decisionStatus := llmApprovalTraceDecisionStatus(trace)
	src.RawSetString("event_type", StringValue(llmTraceString(opts, "event_type", "approval_replay_trace")))
	src.RawSetString("status", StringValue(llmTraceString(opts, "status", decisionStatus)))
	src.RawSetString("provider_free", BoolValue(llmTraceBool(opts, "provider_free", llmTraceBool(trace, "provider_free", true))))
	src.RawSetString("live_network", BoolValue(llmTraceBool(opts, "live_network", llmTraceBool(trace, "live_network", false))))
	src.RawSetString("live_model", BoolValue(llmTraceBool(opts, "live_model", llmTraceBool(trace, "live_model", false))))
	src.RawSetString("credentials_required", BoolValue(llmTraceBool(opts, "credentials_required", llmTraceBool(trace, "credentials_required", false))))
	src.RawSetString("real_dependency_imports", BoolValue(llmTraceBool(opts, "real_dependency_imports", llmTraceBool(trace, "real_dependency_imports", false))))
	llmApprovalTraceCopyCorrelation(trace, src)

	replay := NewTable()
	replay.RawSetString("mode", StringValue(llmTraceString(opts, "replay_mode", llmReplayModeFixture)))
	replay.RawSetString("provider_free", BoolValue(true))
	replay.RawSetString("deterministic", BoolValue(true))
	replay.RawSetString("created_from_provider", BoolValue(false))
	for _, field := range []string{"fixture_key", "replay_key", "request_hash", "response_hash", "record_id"} {
		if value := opts.RawGetString(field); !value.IsNil() {
			replay.RawSetString(field, llmCloneValue(value))
		} else if value := trace.RawGetString(field); !value.IsNil() {
			replay.RawSetString(field, llmCloneValue(value))
		}
	}
	if replayInfo := trace.RawGetString("replay"); replayInfo.IsTable() {
		for _, field := range []string{"fixture_key", "replay_key", "request_hash", "response_hash", "record_id", "deterministic"} {
			if replay.RawGetString(field).IsNil() {
				llmReplayTraceCopyField(replayInfo.Table(), replay, field)
			}
		}
	}

	payload := NewTable()
	payload.RawSetString("kind", StringValue(llmTraceString(trace, "kind", "approval_replay_trace")))
	payload.RawSetString("version", StringValue(llmTraceString(trace, "version", "approval_replay.v1")))
	payload.RawSetString("decision", StringValue(decisionStatus))
	payload.RawSetString("decision_status", StringValue(decisionStatus))
	if resultStatus := llmApprovalTraceResultStatus(trace); resultStatus != "" {
		payload.RawSetString("result_status", StringValue(resultStatus))
	}
	if reason := llmApprovalTraceDecisionReason(trace); reason != "" {
		payload.RawSetString("reason", StringValue(reason))
	}
	llmApprovalTraceCopyRequestSummary(trace, payload)
	llmApprovalTraceCopyPolicySummary(trace, payload)
	if value := replay.RawGetString("fixture_key"); !value.IsNil() {
		payload.RawSetString("source_fixture_key", llmCloneValue(value))
	}
	src.RawSetString("replay", TableValue(replay))
	src.RawSetString("payload", TableValue(payload))
	return llmTraceEventValue(src)
}

func llmApprovalTraceDecisionStatus(trace *Table) string {
	if decision := trace.RawGetString("decision"); decision.IsTable() {
		return llmTraceString(decision.Table(), "status", "denied")
	}
	if approval := trace.RawGetString("approval"); approval.IsTable() && approval.Table().RawGetString("ok").Truthy() {
		return "approved"
	}
	return "denied"
}

func llmApprovalTraceDecisionReason(trace *Table) string {
	if decision := trace.RawGetString("decision"); decision.IsTable() {
		if reason := llmTraceString(decision.Table(), "reason", ""); reason != "" {
			return reason
		}
	}
	if approval := trace.RawGetString("approval"); approval.IsTable() {
		return llmTraceString(approval.Table(), "reason", "")
	}
	return ""
}

func llmApprovalTraceResultStatus(trace *Table) string {
	if result := trace.RawGetString("result"); result.IsTable() {
		return llmTraceString(result.Table(), "status", "")
	}
	return ""
}

func llmApprovalTraceCopyCorrelation(trace, src *Table) {
	request := llmApprovalTraceRequestTable(trace)
	if request != nil {
		for _, pair := range []struct {
			source string
			target string
		}{
			{"request_id", "tool_call_id"},
			{"id", "tool_call_id"},
			{"turn_id", "turn_id"},
			{"workflow_run_id", "workflow_run_id"},
			{"workflow_step_id", "workflow_step_id"},
			{"agent_run_id", "agent_run_id"},
		} {
			if src.RawGetString(pair.target).IsNil() {
				if value := request.RawGetString(pair.source); !value.IsNil() {
					src.RawSetString(pair.target, llmCloneValue(value))
				}
			}
		}
	}
	if approval := trace.RawGetString("approval"); approval.IsTable() && src.RawGetString("approval_id").IsNil() {
		if value := approval.Table().RawGetString("approval_id"); !value.IsNil() {
			src.RawSetString("approval_id", llmCloneValue(value))
		}
	}
	if decision := trace.RawGetString("decision"); decision.IsTable() && src.RawGetString("approval_id").IsNil() {
		if value := decision.Table().RawGetString("approval_id"); !value.IsNil() {
			src.RawSetString("approval_id", llmCloneValue(value))
		}
	}
	if fixture := trace.RawGetString("fixture_key"); !fixture.IsNil() && src.RawGetString("replay_session_id").IsNil() {
		src.RawSetString("replay_session_id", llmCloneValue(fixture))
	}
	if src.RawGetString("correlation_id").IsNil() {
		if value := src.RawGetString("tool_call_id"); !value.IsNil() {
			src.RawSetString("correlation_id", llmCloneValue(value))
		} else if value := src.RawGetString("approval_id"); !value.IsNil() {
			src.RawSetString("correlation_id", llmCloneValue(value))
		}
	}
}

func llmApprovalTraceCopyRequestSummary(trace, payload *Table) {
	request := llmApprovalTraceRequestTable(trace)
	if request == nil {
		return
	}
	for _, field := range []string{"id", "request_id", "tool", "operation", "capability", "risk_level", "approval_required"} {
		if value := request.RawGetString(field); !value.IsNil() {
			payload.RawSetString(field, llmCloneValue(value))
		}
	}
	if payload.RawGetString("operation").IsNil() {
		if tool := payload.RawGetString("tool"); !tool.IsNil() {
			payload.RawSetString("operation", llmCloneValue(tool))
		}
	}
}

func llmApprovalTraceCopyPolicySummary(trace, payload *Table) {
	if policy := trace.RawGetString("policy"); policy.IsTable() {
		for _, pair := range []struct {
			source string
			target string
		}{
			{"kind", "policy_kind"},
			{"version", "policy_version"},
			{"default", "policy_default"},
			{"package", "policy_id"},
		} {
			if value := policy.Table().RawGetString(pair.source); !value.IsNil() {
				payload.RawSetString(pair.target, llmCloneValue(value))
			}
		}
	}
}

func llmApprovalTraceRequestTable(trace *Table) *Table {
	if pending := trace.RawGetString("pending"); pending.IsTable() {
		return pending.Table()
	}
	if request := trace.RawGetString("request"); request.IsTable() {
		return request.Table()
	}
	return nil
}

func llmPolicyOutcomeTraceEventValue(outcome, opts *Table) *Table {
	src := NewTable()
	for _, key := range opts.PairsKeysSnapshot() {
		src.RawSet(key, llmCloneValue(opts.RawGet(key)))
	}
	status := llmTraceString(outcome, "status", "allowed")
	src.RawSetString("event_type", StringValue(llmTraceString(opts, "event_type", "policy_outcome")))
	src.RawSetString("status", StringValue(llmTraceString(opts, "status", status)))
	src.RawSetString("provider_free", BoolValue(llmTraceBool(opts, "provider_free", true)))
	src.RawSetString("live_network", BoolValue(llmTraceBool(opts, "live_network", false)))
	src.RawSetString("live_model", BoolValue(llmTraceBool(opts, "live_model", false)))
	src.RawSetString("credentials_required", BoolValue(llmTraceBool(opts, "credentials_required", false)))
	src.RawSetString("real_dependency_imports", BoolValue(llmTraceBool(opts, "real_dependency_imports", false)))
	llmPolicyOutcomeCopyCorrelation(outcome, src)

	payload := NewTable()
	for _, field := range []string{
		"kind",
		"version",
		"status",
		"result_status",
		"capability",
		"class",
		"policy",
		"policy_kind",
		"policy_version",
		"policy_default",
		"allowed",
		"denied",
		"clean_skip",
		"approval_required",
		"side_effect_allowed",
		"reason",
		"dependency",
	} {
		if value := outcome.RawGetString(field); !value.IsNil() {
			payload.RawSetString(field, llmCloneValue(value))
		}
	}
	if capabilities := outcome.RawGetString("capabilities"); capabilities.IsTable() {
		payload.RawSetString("capabilities", llmCloneValue(capabilities))
		if payload.RawGetString("capability").IsNil() {
			first := capabilities.Table().RawGet(IntValue(1))
			if !first.IsNil() {
				payload.RawSetString("capability", llmCloneValue(first))
			}
		}
	}
	if message := outcome.RawGetString("message"); message.IsString() && message.Str() != "" {
		payload.RawSetString("message", StringValue("policy outcome"))
	}
	src.RawSetString("payload", TableValue(payload))
	return llmTraceEventValue(src)
}

func llmPolicyOutcomeCopyCorrelation(outcome, src *Table) {
	for _, field := range []string{
		"workflow_run_id",
		"workflow_step_id",
		"agent_run_id",
		"turn_id",
		"tool_call_id",
		"approval_id",
		"replay_session_id",
		"correlation_id",
	} {
		if src.RawGetString(field).IsNil() {
			if value := outcome.RawGetString(field); !value.IsNil() {
				src.RawSetString(field, llmCloneValue(value))
			}
		}
	}
	if src.RawGetString("correlation_id").IsNil() {
		if value := outcome.RawGetString("capability"); !value.IsNil() {
			src.RawSetString("correlation_id", llmCloneValue(value))
		} else if capabilities := outcome.RawGetString("capabilities"); capabilities.IsTable() {
			first := capabilities.Table().RawGet(IntValue(1))
			if !first.IsNil() {
				src.RawSetString("correlation_id", llmCloneValue(first))
			}
		}
	}
}

func llmTraceSummaryValue(input *Table) *Table {
	events := llmTraceInputEvents(input)
	out := NewTable()
	out.RawSetString("trace_id", StringValue(llmTraceString(input, "trace_id", "")))
	out.RawSetString("events", IntValue(int64(events.Length())))
	out.RawSetString("event_types", TableValue(NewSequentialArrayTable(0)))
	out.RawSetString("replay_keys", TableValue(NewSequentialArrayTable(0)))
	out.RawSetString("status_counts", TableValue(NewTable()))
	out.RawSetString("missing_correlation", IntValue(0))
	out.RawSetString("sequence_gaps", IntValue(0))
	out.RawSetString("non_monotonic_timestamps", IntValue(0))
	out.RawSetString("provider_free", BoolValue(llmTraceBool(input, "provider_free", true)))
	out.RawSetString("live_network", BoolValue(llmTraceBool(input, "live_network", false)))
	out.RawSetString("live_model", BoolValue(llmTraceBool(input, "live_model", false)))
	out.RawSetString("credentials_required", BoolValue(llmTraceBool(input, "credentials_required", false)))
	out.RawSetString("real_dependency_imports", BoolValue(llmTraceBool(input, "real_dependency_imports", false)))

	eventTypes := out.RawGetString("event_types").Table()
	replayKeys := out.RawGetString("replay_keys").Table()
	statusCounts := out.RawGetString("status_counts").Table()
	seenTypes := map[string]bool{}
	seenReplayKeys := map[string]bool{}
	lastSequence := int64(0)
	lastTimestamp := int64(0)
	for i := 1; i <= events.Length(); i++ {
		event := events.RawGet(IntValue(int64(i)))
		if !event.IsTable() {
			continue
		}
		eventTable := event.Table()
		eventType := llmTraceString(eventTable, "event_type", llmTraceString(eventTable, "type", ""))
		if eventType != "" && !seenTypes[eventType] {
			seenTypes[eventType] = true
			eventTypes.RawSet(IntValue(int64(eventTypes.Length()+1)), StringValue(eventType))
		}
		status := llmTraceString(eventTable, "status", "ok")
		statusCounts.RawSetString(status, IntValue(llmTraceInt64(statusCounts, status, 0)+1))
		if i == 1 {
			out.RawSetString("first_event_id", llmCloneValue(eventTable.RawGetString("event_id")))
		}
		out.RawSetString("last_event_id", llmCloneValue(eventTable.RawGetString("event_id")))
		if correlation := eventTable.RawGetString("correlation"); !correlation.IsTable() {
			out.RawSetString("missing_correlation", IntValue(out.RawGetString("missing_correlation").Int()+1))
		}
		sequence := llmTraceInt64(eventTable, "sequence", int64(i))
		if lastSequence != 0 && sequence != lastSequence+1 {
			out.RawSetString("sequence_gaps", IntValue(out.RawGetString("sequence_gaps").Int()+1))
		}
		lastSequence = sequence
		timestamp := llmTraceInt64(eventTable, "timestamp_ms", 0)
		if timestamp != 0 && lastTimestamp != 0 && timestamp < lastTimestamp {
			out.RawSetString("non_monotonic_timestamps", IntValue(out.RawGetString("non_monotonic_timestamps").Int()+1))
		}
		if timestamp != 0 {
			lastTimestamp = timestamp
		}
		if replay := eventTable.RawGetString("replay"); replay.IsTable() {
			replayKey := llmTraceString(replay.Table(), "replay_key", "")
			if replayKey != "" && !seenReplayKeys[replayKey] {
				seenReplayKeys[replayKey] = true
				replayKeys.RawSet(IntValue(int64(replayKeys.Length()+1)), StringValue(replayKey))
			}
		}
	}
	return out
}

func llmTraceAssertValue(input, opts *Table) *Table {
	summary := llmTraceSummaryValue(input)
	findings := NewSequentialArrayTable(0)
	events := llmTraceInputEvents(input)
	if llmTraceBool(opts, "require_provider_free", false) && !summary.RawGetString("provider_free").Truthy() {
		llmTraceAppendFinding(findings, "generic.ai.trace.provider_not_free", "trace provider_free must be true")
	}
	if llmTraceBool(opts, "deny_live_network", false) && summary.RawGetString("live_network").Truthy() {
		llmTraceAppendFinding(findings, "generic.ai.trace.live_network", "trace live_network must be false")
	}
	if llmTraceBool(opts, "deny_live_model", false) && summary.RawGetString("live_model").Truthy() {
		llmTraceAppendFinding(findings, "generic.ai.trace.live_model", "trace live_model must be false")
	}
	if llmTraceBool(opts, "deny_credentials_required", false) && summary.RawGetString("credentials_required").Truthy() {
		llmTraceAppendFinding(findings, "generic.ai.trace.credentials_required", "trace credentials_required must be false")
	}
	for _, eventType := range llmTraceStringSlice(opts.RawGetString("required_event_types")) {
		if !llmTraceSummaryHasString(summary.RawGetString("event_types"), eventType) {
			llmTraceAppendFinding(findings, "generic.ai.trace.missing_event_type", fmt.Sprintf("trace missing event_type %q", eventType))
		}
	}
	requiredCorrelation := llmTraceStringSlice(opts.RawGetString("require_correlation_fields"))
	if len(requiredCorrelation) > 0 {
		for i := 1; i <= events.Length(); i++ {
			event := events.RawGet(IntValue(int64(i)))
			if !event.IsTable() {
				continue
			}
			eventTable := event.Table()
			correlation := eventTable.RawGetString("correlation")
			for _, field := range requiredCorrelation {
				if !correlation.IsTable() || llmTraceString(correlation.Table(), field, "") == "" {
					eventID := llmTraceString(eventTable, "event_id", fmt.Sprintf("event:%d", i))
					llmTraceAppendFinding(findings, "generic.ai.trace.missing_correlation", fmt.Sprintf("%s missing correlation field %q", eventID, field))
				}
			}
		}
	}
	llmTraceAssertPayloadFields(events, findings, opts)
	llmTraceAssertRedaction(events, findings, opts)
	out := NewTable()
	out.RawSetString("ok", BoolValue(findings.Length() == 0))
	if findings.Length() == 0 {
		out.RawSetString("status", StringValue("ok"))
	} else {
		out.RawSetString("status", StringValue("failed"))
	}
	out.RawSetString("summary", TableValue(summary))
	out.RawSetString("findings", TableValue(findings))
	return out
}

func llmTraceAssertPayloadFields(events, findings, opts *Table) {
	globalFields := llmTraceStringSlice(opts.RawGetString("require_payload_fields"))
	eventFields := opts.RawGetString("require_event_payload_fields")
	if !eventFields.IsTable() {
		eventFields = opts.RawGetString("required_payload_fields_by_event_type")
	}
	if len(globalFields) == 0 && !eventFields.IsTable() {
		return
	}
	for i := 1; i <= events.Length(); i++ {
		event := events.RawGet(IntValue(int64(i)))
		if !event.IsTable() {
			continue
		}
		eventTable := event.Table()
		eventType := llmTraceString(eventTable, "event_type", llmTraceString(eventTable, "type", ""))
		eventID := llmTraceString(eventTable, "event_id", fmt.Sprintf("event:%d", i))
		payload := eventTable.RawGetString("payload")
		llmTraceAssertPayloadFieldList(findings, payload, globalFields, eventID, eventType)
		if eventFields.IsTable() {
			fields := llmTraceRequiredFieldsForEventType(eventFields.Table(), eventType)
			llmTraceAssertPayloadFieldList(findings, payload, fields, eventID, eventType)
		}
	}
}

func llmTraceRequiredFieldsForEventType(spec *Table, eventType string) []string {
	if spec == nil || eventType == "" {
		return nil
	}
	if fields := llmTraceStringSlice(spec.RawGetString(eventType)); len(fields) > 0 {
		return fields
	}
	for _, key := range spec.PairsKeysSnapshot() {
		if key.Str() == eventType {
			return llmTraceStringSlice(spec.RawGet(key))
		}
	}
	return nil
}

func llmTraceAssertPayloadFieldList(findings *Table, payload Value, fields []string, eventID, eventType string) {
	if len(fields) == 0 {
		return
	}
	var payloadTable *Table
	if payload.IsTable() {
		payloadTable = payload.Table()
	}
	for _, field := range fields {
		if payloadTable == nil || payloadTable.RawGetString(field).IsNil() {
			llmTraceAppendFinding(findings, "generic.ai.trace.missing_payload_field", fmt.Sprintf("%s event_type %q missing payload field %q", eventID, eventType, field))
		}
	}
}

func llmTraceAssertRedaction(events, findings, opts *Table) {
	checks := []struct {
		options []string
		field   string
		kind    string
	}{
		{[]string{"deny_secret_values", "deny_secret_values_present"}, "secret_values_present", "generic.ai.trace.secret_values_present"},
		{[]string{"deny_raw_prompt_stored"}, "raw_prompt_stored", "generic.ai.trace.raw_prompt_stored"},
		{[]string{"deny_raw_completion_stored"}, "raw_completion_stored", "generic.ai.trace.raw_completion_stored"},
	}
	enabled := false
	for _, check := range checks {
		if llmTraceAnyBool(opts, check.options) {
			enabled = true
			break
		}
	}
	if !enabled {
		return
	}
	for i := 1; i <= events.Length(); i++ {
		event := events.RawGet(IntValue(int64(i)))
		if !event.IsTable() {
			continue
		}
		eventTable := event.Table()
		eventID := llmTraceString(eventTable, "event_id", fmt.Sprintf("event:%d", i))
		eventType := llmTraceString(eventTable, "event_type", llmTraceString(eventTable, "type", ""))
		redaction := eventTable.RawGetString("redaction")
		if !redaction.IsTable() {
			continue
		}
		redactionTable := redaction.Table()
		for _, check := range checks {
			if llmTraceAnyBool(opts, check.options) && redactionTable.RawGetString(check.field).Truthy() {
				llmTraceAppendFinding(findings, check.kind, fmt.Sprintf("%s event_type %q redaction.%s must be false", eventID, eventType, check.field))
			}
		}
	}
}

func llmTraceAnyBool(t *Table, keys []string) bool {
	for _, key := range keys {
		if llmTraceBool(t, key, false) {
			return true
		}
	}
	return false
}

func llmTraceInputEvents(input *Table) *Table {
	if events := input.RawGetString("events"); events.IsTable() {
		return events.Table()
	}
	return input
}

func llmTraceStringSlice(value Value) []string {
	if !value.IsTable() {
		return nil
	}
	table := value.Table()
	out := make([]string, 0, table.Length())
	for i := 1; i <= table.Length(); i++ {
		item := table.RawGet(IntValue(int64(i)))
		if item.IsString() && item.Str() != "" {
			out = append(out, item.Str())
		}
	}
	return out
}

func llmTraceSummaryHasString(value Value, want string) bool {
	if !value.IsTable() {
		return false
	}
	table := value.Table()
	for i := 1; i <= table.Length(); i++ {
		if table.RawGet(IntValue(int64(i))).Str() == want {
			return true
		}
	}
	return false
}

func llmTraceAppendFinding(findings *Table, kind, message string) {
	finding := NewTable()
	finding.RawSetString("kind", StringValue(kind))
	finding.RawSetString("message", StringValue(message))
	findings.RawSet(IntValue(int64(findings.Length()+1)), TableValue(finding))
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
