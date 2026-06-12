package bind

import "fmt"

func registerLLMEvidenceOutcomeHelpers(t *Table) {
	evidenceOutcome := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.evidence_outcome' (evidence table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.evidence_outcome' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmEvidenceOutcomeValue(args[0].Table(), opts))}, nil
	}
	setLLMFunction(t, "llm", "evidence_outcome", evidenceOutcome)
	setLLMFunction(t, "llm", "evidenceOutcome", evidenceOutcome)
	setLLMFunction(t, "llm", "citation_outcome", evidenceOutcome)
	setLLMFunction(t, "llm", "citationOutcome", evidenceOutcome)
}

func llmEvidenceOutcomeValue(src, opts *Table) *Table {
	refs := llmMemoryOutcomeMatches(src)
	out := NewTable()
	out.RawSetString("kind", StringValue("evidence_outcome"))
	out.RawSetString("schema_version", IntValue(1))
	out.RawSetString("version", StringValue("evidence_outcome.v1"))
	out.RawSetString("source_kind", StringValue(llmEvidenceOutcomeSourceKind(src)))
	out.RawSetString("provider_free", BoolValue(true))
	out.RawSetString("ok", BoolValue(true))
	out.RawSetString("empty", BoolValue(refs.Length() == 0))
	out.RawSetString("evidence_count", IntValue(int64(refs.Length())))
	if refs.Length() == 0 {
		out.RawSetString("status", StringValue("empty"))
		out.RawSetString("result_status", StringValue("empty"))
	} else {
		out.RawSetString("status", StringValue("cited"))
		out.RawSetString("result_status", StringValue("ok"))
	}
	for _, field := range []string{"query", "label", "operation", "component", "report_id", "section", "artifact_id"} {
		if value := opts.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		} else if value := src.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		}
	}
	for _, field := range []string{"workflow_run_id", "workflow_step_id", "agent_run_id", "turn_id", "tool_call_id", "correlation_id", "replay_session_id"} {
		if value := opts.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		} else if value := src.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		}
	}
	out.RawSetString("evidence_refs", TableValue(refs))
	out.RawSetString("citation_count", IntValue(int64(llmEvidenceOutcomeCitationCount(refs))))
	out.RawSetString("source_count", IntValue(int64(llmEvidenceOutcomeDistinctCount(refs, "source"))))
	out.RawSetString("artifact_count", IntValue(int64(llmEvidenceOutcomeDistinctCount(refs, "artifact_id"))))
	if top := refs.RawGet(IntValue(1)); top.IsTable() {
		out.RawSetString("top_evidence", llmCloneValue(top))
		if id := top.Table().RawGetString("id"); !id.IsNil() {
			out.RawSetString("top_id", llmCloneValue(id))
		}
		if source := top.Table().RawGetString("source"); !source.IsNil() {
			out.RawSetString("top_source", llmCloneValue(source))
		}
		if artifact := top.Table().RawGetString("artifact_id"); !artifact.IsNil() {
			out.RawSetString("top_artifact_id", llmCloneValue(artifact))
		}
	}
	out.RawSetString("redaction", TableValue(llmEvidenceOutcomeRedaction(opts)))
	return out
}

func llmEvidenceOutcomeSourceKind(src *Table) string {
	switch {
	case src.RawGetString(llmMemoryContextMarker).Truthy():
		return "evidence"
	case src.RawGetString("matches").IsTable():
		return "retrieval"
	case src.RawGetString(llmMemoryCollectionMarker).Truthy():
		return "collection"
	default:
		return llmTraceString(src, "kind", "evidence")
	}
}

func llmEvidenceOutcomeCitationCount(refs *Table) int {
	if refs == nil {
		return 0
	}
	count := 0
	for i := 1; i <= refs.Length(); i++ {
		item := refs.RawGet(IntValue(int64(i)))
		if item.IsTable() && item.Table().RawGetString("citation").IsTable() {
			count++
		}
	}
	return count
}

func llmEvidenceOutcomeDistinctCount(refs *Table, field string) int {
	if refs == nil {
		return 0
	}
	seen := map[string]bool{}
	for i := 1; i <= refs.Length(); i++ {
		item := refs.RawGet(IntValue(int64(i)))
		if !item.IsTable() {
			continue
		}
		value := item.Table().RawGetString(field)
		if value.IsNil() || value.Str() == "" {
			continue
		}
		seen[value.Str()] = true
	}
	return len(seen)
}

func llmEvidenceOutcomeRedaction(opts *Table) *Table {
	redaction := NewTable()
	redaction.RawSetString("enabled", BoolValue(true))
	redaction.RawSetString("policy", StringValue("evidence_outcome_ref_only"))
	redaction.RawSetString("raw_text_stored", BoolValue(false))
	redaction.RawSetString("raw_snippet_stored", BoolValue(false))
	redaction.RawSetString("raw_prompt_stored", BoolValue(false))
	redaction.RawSetString("raw_completion_stored", BoolValue(false))
	redaction.RawSetString("secret_values_present", BoolValue(false))
	if opts != nil {
		if override := opts.RawGetString("redaction"); override.IsTable() {
			llmCopyTable(redaction, override.Table(), true)
		}
	}
	return redaction
}
