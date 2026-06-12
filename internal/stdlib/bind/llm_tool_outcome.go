package bind

import "fmt"

func (b *llmLibBuilder) registerToolOutcomeHelpers() {
	toolOutcome := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.tool_outcome' (tool call or tool message table expected)")
		}
		result := NilValue()
		if len(args) >= 2 {
			result = args[1]
		}
		opts := NewTable()
		if len(args) >= 3 {
			if !args[2].IsTable() {
				return nil, fmt.Errorf("bad argument #3 to 'llm.tool_outcome' (options table expected)")
			}
			opts = args[2].Table()
		}
		return []Value{TableValue(llmToolOutcomeValue(args[0].Table(), result, opts))}, nil
	}
	b.set("tool_outcome", toolOutcome)
	b.set("toolOutcome", toolOutcome)
}

func llmToolOutcomeValue(source *Table, result Value, opts *Table) *Table {
	call := llmToolOutcomeCallTable(source)
	out := NewTable()
	out.RawSetString("kind", StringValue("tool_outcome"))
	out.RawSetString("schema_version", IntValue(1))
	out.RawSetString("version", StringValue("tool_outcome.v1"))
	out.RawSetString("ok", BoolValue(true))
	out.RawSetString("status", StringValue("ok"))
	out.RawSetString("result_status", StringValue("ok"))
	redaction := NewTable()
	redaction.RawSetString("args_redacted", BoolValue(true))
	redaction.RawSetString("result_redacted", BoolValue(true))
	redaction.RawSetString("raw_args_stored", BoolValue(false))
	redaction.RawSetString("raw_result_stored", BoolValue(false))
	redaction.RawSetString("policy", StringValue("tool_outcome_ref_only"))
	out.RawSetString("redaction", TableValue(redaction))

	if tool := llmToolOutcomeString(opts, source, call, "tool"); tool != "" {
		out.RawSetString("tool", StringValue(tool))
		out.RawSetString("tool_name", StringValue(tool))
	}
	if callID := llmToolOutcomeString(opts, source, call, "tool_call_id", "id", "tool_use_id"); callID != "" {
		out.RawSetString("tool_call_id", StringValue(callID))
	}
	for _, field := range []string{"operation", "component", "workflow_run_id", "workflow_step_id", "agent_run_id", "turn_id", "correlation_id", "replay_key", "result_ref"} {
		if value := opts.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		} else if value := source.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		}
	}
	if argNames := llmToolOutcomeArgNames(call); argNames.Length() > 0 {
		out.RawSetString("arg_names", TableValue(argNames))
	}

	if source.RawGetString("role").Str() == "tool" {
		if err := source.RawGetString("error"); !err.IsNil() {
			result = llmErrorValue("tool", err.Str())
		} else if value := source.RawGetString("value"); !value.IsNil() && result.IsNil() {
			result = value
		}
	}
	if llmToolOutcomeIsError(result) {
		errTable := result.Table()
		out.RawSetString("ok", BoolValue(false))
		out.RawSetString("status", StringValue("error"))
		out.RawSetString("result_status", StringValue("error"))
		if kind := errTable.RawGetString("kind"); !kind.IsNil() {
			out.RawSetString("error_kind", llmCloneValue(kind))
		}
		if message := errTable.RawGetString("message"); !message.IsNil() {
			out.RawSetString("message", StringValue("tool outcome error"))
			out.RawSetString("error_message", llmCloneValue(message))
		}
	} else {
		out.RawSetString("result_present", BoolValue(!result.IsNil()))
		out.RawSetString("result_type", StringValue(llmToolOutcomeValueType(result)))
		if out.RawGetString("result_ref").IsNil() {
			if callID := out.RawGetString("tool_call_id").Str(); callID != "" {
				out.RawSetString("result_ref", StringValue(callID+":result"))
			}
		}
	}
	return out
}

func llmToolOutcomeCallTable(source *Table) *Table {
	if source == nil {
		return nil
	}
	if call := source.RawGetString("tool_call"); call.IsTable() {
		return call.Table()
	}
	return source
}

func llmToolOutcomeString(opts, source, call *Table, fields ...string) string {
	for _, field := range fields {
		if opts != nil {
			if value := opts.RawGetString(field); value.IsString() && value.Str() != "" {
				return value.Str()
			}
		}
		if source != nil {
			if value := source.RawGetString(field); value.IsString() && value.Str() != "" {
				return value.Str()
			}
		}
		if call != nil {
			if value := call.RawGetString(field); value.IsString() && value.Str() != "" {
				return value.Str()
			}
		}
	}
	return ""
}

func llmToolOutcomeIsError(value Value) bool {
	if !value.IsTable() {
		return false
	}
	table := value.Table()
	return !table.RawGetString("kind").IsNil() && !table.RawGetString("message").IsNil()
}

func llmToolOutcomeArgNames(call *Table) *Table {
	out := NewSequentialArrayTable(0)
	if call == nil {
		return out
	}
	args := call.RawGetString("args")
	if !args.IsTable() {
		return out
	}
	for _, key := range args.Table().PairsKeysSnapshot() {
		if key.IsString() {
			out.RawSet(IntValue(int64(out.Length()+1)), StringValue(key.Str()))
		}
	}
	return out
}

func llmToolOutcomeValueType(value Value) string {
	switch {
	case value.IsNil():
		return "nil"
	case value.IsString():
		return "string"
	case value.IsBool():
		return "bool"
	case value.IsInt():
		return "int"
	case value.IsNumber():
		return "number"
	case value.IsTable():
		return "table"
	case value.IsFunction():
		return "function"
	default:
		return "value"
	}
}
