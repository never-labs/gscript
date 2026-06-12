package bind

import "fmt"

func registerLLMMemoryOutcomeHelpers(t *Table) {
	memoryOutcome := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.memory_outcome' (memory table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.memory_outcome' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmMemoryOutcomeValue(args[0].Table(), opts))}, nil
	}
	setLLMFunction(t, "llm", "memory_outcome", memoryOutcome)
	setLLMFunction(t, "llm", "memoryOutcome", memoryOutcome)
	setLLMFunction(t, "llm", "retrieval_outcome", memoryOutcome)
	setLLMFunction(t, "llm", "retrievalOutcome", memoryOutcome)
}

func llmMemoryOutcomeValue(src, opts *Table) *Table {
	matches := llmMemoryOutcomeMatches(src)
	out := NewTable()
	out.RawSetString("kind", StringValue("memory_outcome"))
	out.RawSetString("version", StringValue("memory_outcome.v1"))
	out.RawSetString("source_kind", StringValue(llmMemoryOutcomeSourceKind(src)))
	out.RawSetString("ok", BoolValue(true))
	out.RawSetString("empty", BoolValue(matches.Length() == 0))
	out.RawSetString("match_count", IntValue(int64(matches.Length())))
	if matches.Length() == 0 {
		out.RawSetString("status", StringValue("empty"))
		out.RawSetString("result_status", StringValue("empty"))
	} else {
		out.RawSetString("status", StringValue("matched"))
		out.RawSetString("result_status", StringValue("ok"))
	}
	for _, field := range []string{"query", "label", "operation", "component"} {
		if value := opts.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		} else if value := src.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		}
	}
	for _, field := range []string{"workflow_run_id", "workflow_step_id", "agent_run_id", "turn_id", "tool_call_id", "correlation_id"} {
		if value := opts.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		} else if value := src.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		}
	}
	out.RawSetString("match_refs", TableValue(matches))
	if top := matches.RawGet(IntValue(1)); top.IsTable() {
		out.RawSetString("top_match", llmCloneValue(top))
		if score := top.Table().RawGetString("score"); !score.IsNil() {
			out.RawSetString("top_score", llmCloneValue(score))
		}
		if id := top.Table().RawGetString("id"); !id.IsNil() {
			out.RawSetString("top_id", llmCloneValue(id))
		}
	}
	return out
}

func llmMemoryOutcomeSourceKind(src *Table) string {
	switch {
	case src.RawGetString("matches").IsTable():
		return "retrieval"
	case src.RawGetString(llmMemoryContextMarker).Truthy():
		return "context"
	case src.RawGetString(llmMemoryCollectionMarker).Truthy():
		return "collection"
	default:
		return llmTraceString(src, "kind", "memory")
	}
}

func llmMemoryOutcomeMatches(src *Table) *Table {
	if matches := src.RawGetString("matches"); matches.IsTable() {
		return llmMemoryOutcomeMatchRefs(matches.Table())
	}
	if docs := src.RawGetString("docs"); docs.IsTable() {
		return llmMemoryOutcomeMatchRefs(docs.Table())
	}
	return llmMemoryOutcomeMatchRefs(src)
}

func llmMemoryOutcomeMatchRefs(src *Table) *Table {
	out := NewSequentialArrayTable(0)
	for i := 1; i <= src.Length(); i++ {
		item := src.RawGet(IntValue(int64(i)))
		if !item.IsTable() {
			continue
		}
		ref := llmMemoryOutcomeMatchRef(item.Table(), i)
		out.RawSet(IntValue(int64(out.Length()+1)), TableValue(ref))
	}
	return out
}

func llmMemoryOutcomeMatchRef(match *Table, rank int) *Table {
	ref := NewTable()
	ref.RawSetString("rank", IntValue(int64(rank)))
	for _, field := range []string{"id", "doc_id", "chunk_id", "title", "source", "artifact_id", "section", "score"} {
		if value := match.RawGetString(field); !value.IsNil() {
			ref.RawSetString(field, llmCloneValue(value))
		}
	}
	if citation := match.RawGetString("citation"); citation.IsTable() {
		ref.RawSetString("citation", TableValue(llmMemoryOutcomeCitationRef(citation.Table())))
	} else {
		ref.RawSetString("citation", TableValue(llmMemoryOutcomeCitationRef(llmMemoryCitation(TableValue(match)))))
	}
	return ref
}

func llmMemoryOutcomeCitationRef(citation *Table) *Table {
	ref := NewTable()
	for _, field := range []string{"id", "doc_id", "chunk_id", "title", "source", "artifact_id", "section", "score"} {
		if value := citation.RawGetString(field); !value.IsNil() {
			ref.RawSetString(field, llmCloneValue(value))
		}
	}
	return ref
}
