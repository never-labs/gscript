package runtime

import (
	"context"
	"fmt"
)

// LLMProvider is the host boundary behind llm.turn. Implementations may call a
// remote API, a local model, or a test double; the runtime only sees this small
// protocol shape.
type LLMProvider interface {
	Turn(context.Context, LLMTurnRequest) (LLMTurnResult, error)
}

type LLMMessage struct {
	Role      string
	Text      string
	ToolCall  *LLMToolCall
	ToolUseID string
	Value     any
	Error     string
}

type LLMTool struct {
	Name        string
	Description string
	Params      []string
}

type LLMToolCall struct {
	ID   string
	Tool string
	Args map[string]any
}

type LLMTurnRequest struct {
	Model     string
	Messages  []LLMMessage
	Tools     []LLMTool
	MaxTokens int64
	Stream    bool
}

type LLMTurnUsage struct {
	InputTokens  int64
	OutputTokens int64
	Cost         float64
	LatencyMS    int64
}

type LLMTurnResult struct {
	Status string
	Text   string
	Calls  []LLMToolCall
	Reason string
	Usage  LLMTurnUsage
}

// BuildLLMLib creates the "llm" standard library table. It is the first-stage
// runtime substrate for the agent layer: future syntax can compile to these
// functions without changing provider or tool-dispatch semantics.
func BuildLLMLib(call FunctionCaller, provider func() LLMProvider, maxHostResult func() int64, ctx func() context.Context) *Table {
	t := NewTable()

	hostLimit := func() int64 {
		if maxHostResult == nil {
			return 0
		}
		return maxHostResult()
	}
	currentProvider := func() LLMProvider {
		if provider == nil {
			return nil
		}
		return provider()
	}
	currentContext := func() context.Context {
		if ctx == nil || ctx() == nil {
			return context.Background()
		}
		return ctx()
	}

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{Name: "llm." + name, Fn: fn}))
	}

	set("system", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'llm.system'")
		}
		return []Value{TableValue(llmMessageTable("system", args[0].Str()))}, nil
	})
	set("user", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'llm.user'")
		}
		return []Value{TableValue(llmMessageTable("user", args[0].Str()))}, nil
	})
	set("assistant", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'llm.assistant'")
		}
		return []Value{TableValue(llmMessageTable("assistant", args[0].Str()))}, nil
	})
	set("assistantCall", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.assistantCall' (table expected)")
		}
		msg := NewTable()
		msg.RawSetString("role", StringValue("assistant"))
		msg.RawSetString("tool_call", args[0])
		return []Value{TableValue(msg)}, nil
	})
	set("toolResult", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'llm.toolResult'")
		}
		msg := NewTable()
		msg.RawSetString("role", StringValue("tool"))
		msg.RawSetString("tool_use_id", args[0])
		msg.RawSetString("value", args[1])
		return []Value{TableValue(msg)}, nil
	})
	set("toolError", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'llm.toolError'")
		}
		msg := NewTable()
		msg.RawSetString("role", StringValue("tool"))
		msg.RawSetString("tool_use_id", args[0])
		msg.RawSetString("error", StringValue(args[1].Str()))
		return []Value{TableValue(msg)}, nil
	})

	set("tool", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsFunction() {
			return nil, fmt.Errorf("bad argument to 'llm.tool' (name, function expected)")
		}
		desc := ""
		var params Value
		if len(args) >= 3 && args[2].IsTable() {
			opts := args[2].Table()
			desc = opts.RawGetString("description").Str()
			params = opts.RawGetString("params")
		}
		tool := NewTable()
		tool.RawSetString("__llm_tool", BoolValue(true))
		tool.RawSetString("name", args[0])
		tool.RawSetString("fn", args[1])
		tool.RawSetString("description", StringValue(desc))
		if params.IsTable() {
			tool.RawSetString("params", params)
		}
		return []Value{TableValue(tool)}, nil
	})

	set("turn", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.turn' (table expected)")
		}
		p := currentProvider()
		if p == nil {
			return []Value{NilValue(), llmErrorValue("provider", "llm provider not configured")}, nil
		}
		req, err := llmRequestFromTable(args[0].Table())
		if err != nil {
			return []Value{NilValue(), llmErrorValue("validation", err.Error())}, nil
		}
		res, err := p.Turn(currentContext(), req)
		if err != nil {
			return []Value{NilValue(), llmErrorValue("provider", err.Error())}, nil
		}
		out := llmResultValue(res)
		if err := CheckHostResultBytes(hostLimit(), out); err != nil {
			return nil, err
		}
		return []Value{out, NilValue()}, nil
	})

	set("dispatch", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() || !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument to 'llm.dispatch' (call, tools expected)")
		}
		if call == nil {
			return nil, fmt.Errorf("llm.dispatch requires a function caller")
		}
		return llmDispatch(call, args[0].Table(), args[1].Table())
	})

	set("react", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.react' (table expected)")
		}
		if call == nil {
			return nil, fmt.Errorf("llm.react requires a function caller")
		}
		p := currentProvider()
		if p == nil {
			return []Value{NilValue(), llmErrorValue("provider", "llm provider not configured")}, nil
		}
		result, err := llmReact(args[0].Table(), p, call, currentContext(), hostLimit())
		if err != nil {
			return nil, err
		}
		return result, nil
	})

	return t
}

func llmMessageTable(role, text string) *Table {
	msg := NewTable()
	msg.RawSetString("role", StringValue(role))
	msg.RawSetString("text", StringValue(text))
	return msg
}

func llmRequestFromTable(t *Table) (LLMTurnRequest, error) {
	req := LLMTurnRequest{
		Model:     t.RawGetString("model").Str(),
		MaxTokens: toInt(t.RawGetString("max_tokens")),
		Stream:    t.RawGetString("stream").Truthy(),
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

func llmToolsFromValue(v Value) []LLMTool {
	if !v.IsTable() {
		return nil
	}
	t := v.Table()
	out := make([]LLMTool, 0, t.Length())
	for i := 1; i <= t.Length(); i++ {
		tv := t.RawGet(IntValue(int64(i)))
		if !tv.IsTable() {
			continue
		}
		tt := tv.Table()
		out = append(out, LLMTool{
			Name:        tt.RawGetString("name").Str(),
			Description: tt.RawGetString("description").Str(),
			Params:      llmStringSliceFromValue(tt.RawGetString("params")),
		})
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

func llmToolCallFromTable(t *Table) LLMToolCall {
	c := LLMToolCall{
		ID:   t.RawGetString("id").Str(),
		Tool: t.RawGetString("tool").Str(),
		Args: map[string]any{},
	}
	if args := t.RawGetString("args"); args.IsTable() {
		for _, key := range args.Table().PairsKeysSnapshot() {
			c.Args[key.Str()] = llmAnyFromValue(args.Table().RawGet(key))
		}
	}
	return c
}

func llmResultValue(res LLMTurnResult) Value {
	if res.Status == "" {
		if len(res.Calls) > 0 {
			res.Status = "tool_calls"
		} else {
			res.Status = "final_answer"
		}
	}
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

func llmDispatch(call FunctionCaller, callTable, tools *Table) ([]Value, error) {
	toolName := callTable.RawGetString("tool").Str()
	if toolName == "" {
		return []Value{NilValue(), llmErrorValue("validation", "tool call missing tool name")}, nil
	}
	tool := llmFindTool(tools, toolName)
	if tool == nil {
		return []Value{NilValue(), llmErrorValue("capability", "tool not in scope: "+toolName)}, nil
	}
	fn := tool.RawGetString("fn")
	if !fn.IsFunction() {
		return []Value{NilValue(), llmErrorValue("internal", "tool has no function: "+toolName)}, nil
	}
	args := llmDispatchArgs(callTable.RawGetString("args"), tool.RawGetString("params"))
	results, err := call(fn, args)
	if err != nil {
		return []Value{NilValue(), llmErrorValue("internal", err.Error())}, nil
	}
	if len(results) == 0 {
		return []Value{NilValue(), NilValue()}, nil
	}
	if len(results) >= 2 && !results[1].IsNil() {
		return []Value{results[0], results[1]}, nil
	}
	return []Value{results[0], NilValue()}, nil
}

func llmReact(opts *Table, provider LLMProvider, call FunctionCaller, ctx context.Context, maxHostResult int64) ([]Value, error) {
	messagesValue := opts.RawGetString("messages")
	if !messagesValue.IsTable() {
		return []Value{NilValue(), llmErrorValue("validation", "llm.react requires messages")}, nil
	}
	toolsValue := opts.RawGetString("tools")
	tools := toolsValue.Table()
	if tools == nil {
		tools = NewTable()
	}
	history := llmMessageValuesFromTable(messagesValue.Table())
	maxSteps := int(toInt(opts.RawGetString("max_steps")))
	if maxSteps <= 0 {
		maxSteps = 8
	}
	model := opts.RawGetString("model").Str()
	for step := 0; step < maxSteps; step++ {
		req := LLMTurnRequest{
			Model:     model,
			Messages:  llmMessagesFromValue(llmTableFromValues(history)),
			Tools:     llmToolsFromValue(toolsValue),
			MaxTokens: toInt(opts.RawGetString("max_tokens")),
			Stream:    opts.RawGetString("stream").Truthy(),
		}
		res, err := provider.Turn(ctx, req)
		if err != nil {
			return []Value{NilValue(), llmErrorValue("provider", err.Error())}, nil
		}
		turnValue := llmResultValue(res)
		if err := CheckHostResultBytes(maxHostResult, turnValue); err != nil {
			return nil, err
		}
		switch res.Status {
		case "", "final_answer":
			return []Value{llmReactResultValue("done", res.Text, "", turnValue, history), NilValue()}, nil
		case "stop":
			return []Value{llmReactResultValue("stopped", "", res.Reason, turnValue, history), NilValue()}, nil
		case "tool_calls":
			for i := range res.Calls {
				callValue := llmToolCallValue(res.Calls[i])
				history = append(history, llmAssistantCallMessage(callValue))
				dispatchResult, err := llmDispatch(call, callValue.Table(), tools)
				if err != nil {
					return nil, err
				}
				if len(dispatchResult) >= 2 && !dispatchResult[1].IsNil() {
					message := dispatchResult[1].Table().RawGetString("message").Str()
					history = append(history, llmToolErrorMessage(res.Calls[i].ID, message))
				} else {
					value := NilValue()
					if len(dispatchResult) > 0 {
						value = dispatchResult[0]
					}
					history = append(history, llmToolResultMessage(res.Calls[i].ID, value))
				}
			}
		default:
			return []Value{llmReactResultValue("stopped", "", res.Status, turnValue, history), NilValue()}, nil
		}
	}
	return []Value{llmReactResultValue("stopped", "", "max_steps", NilValue(), history), NilValue()}, nil
}

func llmReactResultValue(status, text, reason string, turn Value, history []Value) Value {
	t := NewTable()
	t.RawSetString("status", StringValue(status))
	t.RawSetString("text", StringValue(text))
	t.RawSetString("reason", StringValue(reason))
	t.RawSetString("result", turn)
	t.RawSetString("history", llmTableFromValues(history))
	return TableValue(t)
}

func llmAssistantCallMessage(callValue Value) Value {
	t := NewTable()
	t.RawSetString("role", StringValue("assistant"))
	t.RawSetString("tool_call", callValue)
	return TableValue(t)
}

func llmToolResultMessage(id string, value Value) Value {
	t := NewTable()
	t.RawSetString("role", StringValue("tool"))
	t.RawSetString("tool_use_id", StringValue(id))
	t.RawSetString("value", value)
	return TableValue(t)
}

func llmToolErrorMessage(id, message string) Value {
	t := NewTable()
	t.RawSetString("role", StringValue("tool"))
	t.RawSetString("tool_use_id", StringValue(id))
	t.RawSetString("error", StringValue(message))
	return TableValue(t)
}

func llmFindTool(tools *Table, name string) *Table {
	for i := 1; i <= tools.Length(); i++ {
		v := tools.RawGet(IntValue(int64(i)))
		if v.IsTable() && v.Table().RawGetString("name").Str() == name {
			return v.Table()
		}
	}
	return nil
}

func llmDispatchArgs(argValue, paramsValue Value) []Value {
	if !argValue.IsTable() {
		if argValue.IsNil() {
			return nil
		}
		return []Value{argValue}
	}
	args := argValue.Table()
	params := llmStringSliceFromValue(paramsValue)
	if len(params) > 0 {
		out := make([]Value, len(params))
		for i, name := range params {
			out[i] = args.RawGetString(name)
		}
		return out
	}
	if n := args.Length(); n > 0 {
		out := make([]Value, n)
		for i := 1; i <= n; i++ {
			out[i-1] = args.RawGet(IntValue(int64(i)))
		}
		return out
	}
	return []Value{argValue}
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
