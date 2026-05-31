package runtime

import "github.com/never-labs/gscript/internal/stdlib/ai"

func llmSnapshotToken() (string, error) {
	return ai.SnapshotToken()
}

func llmIsSnapshotStore(v Value) bool {
	if !v.IsTable() {
		return false
	}
	t := v.Table()
	return t.RawGetString("save").IsFunction() &&
		t.RawGetString("load").IsFunction() &&
		t.RawGetString("delete").IsFunction()
}

func llmStoreSave(call ScriptFunctionCaller, store *Table, token string, snapshot Value) (Value, error) {
	return llmStoreCall(call, store.RawGetString("save"), []Value{StringValue(token), snapshot})
}

func llmStoreLoad(call ScriptFunctionCaller, store *Table, token string) (Value, Value, error) {
	result, errVal, err := llmStoreCall2(call, store.RawGetString("load"), []Value{StringValue(token)})
	if err != nil || !errVal.IsNil() {
		return NilValue(), errVal, err
	}
	return result, NilValue(), nil
}

func llmStoreDelete(call ScriptFunctionCaller, store *Table, token string) (Value, error) {
	return llmStoreCall(call, store.RawGetString("delete"), []Value{StringValue(token)})
}

func llmStoreCall(call ScriptFunctionCaller, fn Value, args []Value) (Value, error) {
	_, errVal, err := llmStoreCall2(call, fn, args)
	return errVal, err
}

func llmStoreCall2(call ScriptFunctionCaller, fn Value, args []Value) (Value, Value, error) {
	if call == nil {
		return NilValue(), llmErrorValue("internal", "snapshot store requires a function caller"), nil
	}
	results, err := call(fn, args)
	if err != nil {
		return NilValue(), NilValue(), err
	}
	if len(results) >= 2 && !results[1].IsNil() {
		return NilValue(), results[1], nil
	}
	if len(results) == 0 {
		return NilValue(), NilValue(), nil
	}
	return results[0], NilValue(), nil
}

func llmResumeSnapshot(snapshot, approval, tools *Table, call ScriptFunctionCaller) ([]Value, error) {
	historyValue := snapshot.RawGetString("history")
	pendingValue := snapshot.RawGetString("pending")
	if !historyValue.IsTable() || !pendingValue.IsTable() {
		return []Value{NilValue(), llmErrorValue("validation", "malformed snapshot")}, nil
	}
	history := llmMessageValuesFromTable(historyValue.Table())
	pending := pendingValue
	if replacementArgs := approval.RawGetString("args"); replacementArgs.IsTable() {
		pending = llmToolCallValue(llmToolCallFromTable(pendingValue.Table()))
		pending.Table().RawSetString("args", replacementArgs)
	}
	history = append(history, llmAssistantCallMessage(pending))

	approved := approval.RawGetString("ok").Truthy()
	reason := approval.RawGetString("reason").Str()
	if !approved {
		if reason == "" {
			reason = "denied"
		}
		history = append(history, llmToolErrorMessage(pending.Table().RawGetString("id").Str(), reason))
		return []Value{llmResumeResultValue("denied", pending, llmTableFromValues(history), NilValue()), NilValue()}, nil
	}
	if tools == nil || call == nil {
		return []Value{llmResumeResultValue("approved", pending, llmTableFromValues(history), NilValue()), NilValue()}, nil
	}
	result, err := llmDispatch(call, pending.Table(), tools)
	if err != nil {
		return nil, err
	}
	if len(result) >= 2 && !result[1].IsNil() {
		message := result[1].Table().RawGetString("message").Str()
		history = append(history, llmToolErrorMessage(pending.Table().RawGetString("id").Str(), message))
		return []Value{llmResumeResultValue("tool_error", pending, llmTableFromValues(history), result[1]), NilValue()}, nil
	}
	value := NilValue()
	if len(result) > 0 {
		value = result[0]
	}
	history = append(history, llmToolResultMessage(pending.Table().RawGetString("id").Str(), value))
	return []Value{llmResumeResultValue("dispatched", pending, llmTableFromValues(history), value), NilValue()}, nil
}

func llmResumeResultValue(status string, pending, history, value Value) Value {
	t := NewTable()
	t.RawSetString("status", StringValue(status))
	t.RawSetString("pending", pending)
	t.RawSetString("history", history)
	t.RawSetString("value", value)
	return TableValue(t)
}
