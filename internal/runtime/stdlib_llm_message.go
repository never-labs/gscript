package runtime

import (
	"fmt"

	"github.com/never-labs/gscript/internal/stdlib/ai"
)

// BuildLLMMessageLib creates the spec-facing "msg" helper table. It is kept
// separate from provider access so future agent syntax can depend on message
// construction without gaining model-call authority.
func BuildLLMMessageLib() *Table {
	t := NewTable()
	registerLLMMessageConstructors(t, "msg")
	setLLMFunction(t, "msg", "assistant_call", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'msg.assistant_call' (table expected)")
		}
		return []Value{TableValue(llmAssistantCallMessageTable(args[0]))}, nil
	})
	setLLMFunction(t, "msg", "tool_result", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'msg.tool_result'")
		}
		return []Value{TableValue(llmToolResultMessageTable(args[0], args[1]))}, nil
	})
	setLLMFunction(t, "msg", "tool_error", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'msg.tool_error'")
		}
		return []Value{TableValue(llmToolErrorMessageTable(args[0], args[1].Str()))}, nil
	})
	return t
}
func registerLLMMessageConstructors(t *Table, module string) {
	setLLMFunction(t, module, "system", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to '%s.system'", module)
		}
		return []Value{TableValue(llmMessageTable(ai.RoleSystem, args[0].Str()))}, nil
	})
	setLLMFunction(t, module, "user", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to '%s.user'", module)
		}
		return []Value{TableValue(llmMessageTable(ai.RoleUser, args[0].Str()))}, nil
	})
	setLLMFunction(t, module, "assistant", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to '%s.assistant'", module)
		}
		return []Value{TableValue(llmMessageTable(ai.RoleAssistant, args[0].Str()))}, nil
	})
}

func setLLMFunction(t *Table, module, name string, fn func([]Value) ([]Value, error)) {
	t.RawSet(StringValue(name), FunctionValue(&GoFunction{Name: module + "." + name, Fn: fn}))
}

func llmMessageTable(role, text string) *Table {
	spec := ai.NewTextMessage(role, text)
	msg := NewTable()
	msg.RawSetString("role", StringValue(spec.Role))
	msg.RawSetString("text", StringValue(spec.Text))
	return msg
}

func llmAssistantCallMessageTable(toolCall Value) *Table {
	spec := ai.NewAssistantCallMessage()
	msg := NewTable()
	msg.RawSetString("role", StringValue(spec.Role))
	msg.RawSetString(spec.ToolCallKey, toolCall)
	return msg
}

func llmToolResultMessageTable(toolUseID, value Value) *Table {
	spec := ai.NewToolResultMessage()
	msg := NewTable()
	msg.RawSetString("role", StringValue(spec.Role))
	msg.RawSetString(spec.ToolUseIDKey, toolUseID)
	msg.RawSetString(spec.ValueKey, value)
	return msg
}

func llmToolErrorMessageTable(toolUseID Value, message string) *Table {
	spec := ai.NewToolResultMessage()
	msg := NewTable()
	msg.RawSetString("role", StringValue(spec.Role))
	msg.RawSetString(spec.ToolUseIDKey, toolUseID)
	msg.RawSetString(spec.ErrorKey, StringValue(message))
	return msg
}

func llmRequestFromTable(t *Table) (LLMTurnRequest, error) {
	req := LLMTurnRequest{
		Model:          t.RawGetString("model").Str(),
		ForceTool:      llmForceToolFromValue(t.RawGetString("force_tool")),
		MaxTokens:      toInt(t.RawGetString("max_tokens")),
		Temperature:    llmOptionalFloatFromValue(t.RawGetString("temperature")),
		TopP:           llmOptionalFloatFromValue(t.RawGetString("top_p")),
		ResponseFormat: llmAnyFromValue(t.RawGetString("response_format")),
		Stream:         t.RawGetString("stream").Truthy(),
		Stop:           llmStringSliceFromValue(t.RawGetString("stop")),
		Metadata:       llmStringMapFromValue(t.RawGetString("metadata")),
	}
	messages := t.RawGetString("messages")
	if !messages.IsTable() {
		return req, fmt.Errorf("llm.turn requires messages")
	}
	req.Messages = llmMessagesFromValue(messages)
	req.Tools = llmToolsFromValue(t.RawGetString("tools"))
	return req, nil
}

func llmMessageValuesFromTable(t *Table) []Value {
	if t == nil {
		return nil
	}
	out := make([]Value, 0, t.Length())
	for i := 1; i <= t.Length(); i++ {
		v := t.RawGet(IntValue(int64(i)))
		if v.IsTable() {
			out = append(out, v)
		}
	}
	return out
}

func llmTableFromValues(values []Value) Value {
	t := NewSequentialArrayTable(len(values))
	for i, v := range values {
		t.array[i+1] = v
	}
	return TableValue(t)
}

func llmMessagesFromValue(v Value) []LLMMessage {
	t := v.Table()
	if t == nil {
		return nil
	}
	n := t.Length()
	out := make([]LLMMessage, 0, n)
	for i := 1; i <= n; i++ {
		mv := t.RawGet(IntValue(int64(i)))
		if !mv.IsTable() {
			continue
		}
		mt := mv.Table()
		msg := LLMMessage{
			Role:      mt.RawGetString("role").Str(),
			Text:      mt.RawGetString("text").Str(),
			ToolUseID: mt.RawGetString("tool_use_id").Str(),
			Error:     mt.RawGetString("error").Str(),
		}
		if call := mt.RawGetString("tool_call"); call.IsTable() {
			c := llmToolCallFromTable(call.Table())
			msg.ToolCall = &c
		}
		if value := mt.RawGetString("value"); !value.IsNil() {
			msg.Value = llmAnyFromValue(value)
		}
		out = append(out, msg)
	}
	return out
}

func llmStringSliceFromValue(v Value) []string {
	if !v.IsTable() {
		return nil
	}
	t := v.Table()
	out := make([]string, 0, t.Length())
	for i := 1; i <= t.Length(); i++ {
		out = append(out, t.RawGet(IntValue(int64(i))).Str())
	}
	return out
}

func llmStringMapFromValue(v Value) map[string]string {
	if !v.IsTable() {
		return nil
	}
	t := v.Table()
	out := make(map[string]string)
	for _, key := range t.PairsKeysSnapshot() {
		if !key.IsString() {
			continue
		}
		out[key.Str()] = t.RawGet(key).Str()
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func llmOptionalFloatFromValue(v Value) *float64 {
	if v.IsNil() {
		return nil
	}
	f := toFloat(v)
	return &f
}

func llmForceToolFromValue(v Value) string {
	if v.IsString() {
		return v.Str()
	}
	if v.IsTable() {
		return v.Table().RawGetString("name").Str()
	}
	return ""
}

func llmResultValue(res LLMTurnResult) Value {
	res.Status = ai.DefaultTurnStatus(res.Status, len(res.Calls))
	t := NewTable()
	t.RawSetString("status", StringValue(res.Status))
	t.RawSetString("text", StringValue(res.Text))
	t.RawSetString("reason", StringValue(res.Reason))
	calls := NewSequentialArrayTable(len(res.Calls))
	for i, call := range res.Calls {
		calls.array[i+1] = llmToolCallValue(call)
	}
	t.RawSetString("calls", TableValue(calls))
	usage := NewTable()
	usage.RawSetString("input_tokens", IntValue(res.Usage.InputTokens))
	usage.RawSetString("output_tokens", IntValue(res.Usage.OutputTokens))
	usage.RawSetString("cost", FloatValue(res.Usage.Cost))
	usage.RawSetString("latency_ms", IntValue(res.Usage.LatencyMS))
	t.RawSetString("usage", TableValue(usage))
	return TableValue(t)
}

func llmResultStatus(res LLMTurnResult) string {
	return ai.DefaultTurnStatus(res.Status, len(res.Calls))
}

func llmToolCallValue(call LLMToolCall) Value {
	t := NewTable()
	t.RawSetString("id", StringValue(call.ID))
	t.RawSetString("tool", StringValue(call.Tool))
	args := NewTable()
	for k, v := range call.Args {
		args.RawSetString(k, llmValueFromAny(v))
	}
	t.RawSetString("args", TableValue(args))
	return TableValue(t)
}

func llmErrorValue(kind, message string) Value {
	t := NewTable()
	t.RawSetString("kind", StringValue(kind))
	t.RawSetString("message", StringValue(message))
	return TableValue(t)
}

func llmProviderErrorValue(err error) Value {
	if err == nil {
		return llmErrorValue(LLMProviderErrorProvider, "")
	}
	return llmErrorValue(ClassifyLLMProviderError(err), err.Error())
}

func llmAssistantCallMessage(callValue Value) Value {
	t := NewTable()
	t.RawSetString("role", StringValue(ai.RoleAssistant))
	t.RawSetString("tool_call", callValue)
	return TableValue(t)
}

func llmToolResultMessage(id string, value Value) Value {
	t := NewTable()
	t.RawSetString("role", StringValue(ai.RoleTool))
	t.RawSetString("tool_use_id", StringValue(id))
	t.RawSetString("value", value)
	return TableValue(t)
}

func llmToolErrorMessage(id, message string) Value {
	t := NewTable()
	t.RawSetString("role", StringValue(ai.RoleTool))
	t.RawSetString("tool_use_id", StringValue(id))
	t.RawSetString("error", StringValue(message))
	return TableValue(t)
}

func llmAnyFromValue(v Value) any {
	switch {
	case v.IsNil():
		return nil
	case v.IsBool():
		return v.Bool()
	case v.IsInt():
		return v.Int()
	case v.IsFloat():
		return v.Float()
	case v.IsString():
		return v.Str()
	case v.IsTable():
		t := v.Table()
		if n := t.Length(); n > 0 {
			out := make([]any, n)
			for i := 1; i <= n; i++ {
				out[i-1] = llmAnyFromValue(t.RawGet(IntValue(int64(i))))
			}
			return out
		}
		out := map[string]any{}
		for _, key := range t.PairsKeysSnapshot() {
			out[key.Str()] = llmAnyFromValue(t.RawGet(key))
		}
		return out
	default:
		return v.String()
	}
}

func llmValueFromAny(v any) Value {
	switch x := v.(type) {
	case nil:
		return NilValue()
	case Value:
		return x
	case bool:
		return BoolValue(x)
	case int:
		return IntValue(int64(x))
	case int64:
		return IntValue(x)
	case float64:
		return FloatValue(x)
	case string:
		return StringValue(x)
	case map[string]any:
		t := NewTable()
		for k, v := range x {
			t.RawSetString(k, llmValueFromAny(v))
		}
		return TableValue(t)
	case []any:
		t := NewSequentialArrayTable(len(x))
		for i, v := range x {
			t.array[i+1] = llmValueFromAny(v)
		}
		return TableValue(t)
	default:
		return StringValue(fmt.Sprint(x))
	}
}
