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

type LLMTraceEvent struct {
	Type         string
	Model        string
	Status       string
	Tool         string
	CallID       string
	ErrorKind    string
	Message      string
	Step         int64
	Attempt      int64
	MessageCount int
	ToolCount    int
	Usage        LLMTurnUsage
}

type LLMTraceSink func(LLMTraceEvent)

// BuildLLMLib creates the "llm" standard library table. It is the first-stage
// runtime substrate for the agent layer: future syntax can compile to these
// functions without changing provider or tool-dispatch semantics.
func BuildLLMLib(call FunctionCaller, provider func() LLMProvider, maxHostResult func() int64, ctx func() context.Context, traces ...LLMTraceSink) *Table {
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
	trace := func(event LLMTraceEvent) {
		if len(traces) == 0 || traces[0] == nil {
			return
		}
		traces[0](event)
	}

	set := func(name string, fn func([]Value) ([]Value, error)) { setLLMFunction(t, "llm", name, fn) }

	registerLLMMessageConstructors(t, "llm")
	set("assistantCall", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.assistantCall' (table expected)")
		}
		msg := NewTable()
		msg.RawSetString("role", StringValue("assistant"))
		msg.RawSetString("tool_call", args[0])
		return []Value{TableValue(msg)}, nil
	})
	set("assistant_call", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.assistant_call' (table expected)")
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
	set("tool_result", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'llm.tool_result'")
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
	set("tool_error", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'llm.tool_error'")
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
			trace(LLMTraceEvent{Type: "turn_error", ErrorKind: "provider", Message: "llm provider not configured"})
			return []Value{NilValue(), llmErrorValue("provider", "llm provider not configured")}, nil
		}
		req, err := llmRequestFromTable(args[0].Table())
		if err != nil {
			trace(LLMTraceEvent{Type: "turn_error", ErrorKind: "validation", Message: err.Error()})
			return []Value{NilValue(), llmErrorValue("validation", err.Error())}, nil
		}
		trace(LLMTraceEvent{Type: "turn_start", Model: req.Model, MessageCount: len(req.Messages), ToolCount: len(req.Tools)})
		res, err := p.Turn(currentContext(), req)
		if err != nil {
			trace(LLMTraceEvent{Type: "turn_error", Model: req.Model, ErrorKind: "provider", Message: err.Error()})
			return []Value{NilValue(), llmErrorValue("provider", err.Error())}, nil
		}
		trace(LLMTraceEvent{Type: "turn_end", Model: req.Model, Status: llmResultStatus(res), MessageCount: len(req.Messages), ToolCount: len(req.Tools), Usage: res.Usage})
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
			trace(LLMTraceEvent{Type: "react_error", ErrorKind: "provider", Message: "llm provider not configured"})
			return []Value{NilValue(), llmErrorValue("provider", "llm provider not configured")}, nil
		}
		result, err := llmReact(args[0].Table(), p, call, currentContext(), hostLimit(), trace)
		if err != nil {
			return nil, err
		}
		return result, nil
	})

	return t
}

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
		msg := NewTable()
		msg.RawSetString("role", StringValue("assistant"))
		msg.RawSetString("tool_call", args[0])
		return []Value{TableValue(msg)}, nil
	})
	setLLMFunction(t, "msg", "tool_result", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'msg.tool_result'")
		}
		msg := NewTable()
		msg.RawSetString("role", StringValue("tool"))
		msg.RawSetString("tool_use_id", args[0])
		msg.RawSetString("value", args[1])
		return []Value{TableValue(msg)}, nil
	})
	setLLMFunction(t, "msg", "tool_error", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'msg.tool_error'")
		}
		msg := NewTable()
		msg.RawSetString("role", StringValue("tool"))
		msg.RawSetString("tool_use_id", args[0])
		msg.RawSetString("error", StringValue(args[1].Str()))
		return []Value{TableValue(msg)}, nil
	})
	return t
}

func registerLLMMessageConstructors(t *Table, module string) {
	setLLMFunction(t, module, "system", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to '%s.system'", module)
		}
		return []Value{TableValue(llmMessageTable("system", args[0].Str()))}, nil
	})
	setLLMFunction(t, module, "user", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to '%s.user'", module)
		}
		return []Value{TableValue(llmMessageTable("user", args[0].Str()))}, nil
	})
	setLLMFunction(t, module, "assistant", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to '%s.assistant'", module)
		}
		return []Value{TableValue(llmMessageTable("assistant", args[0].Str()))}, nil
	})
}

func setLLMFunction(t *Table, module, name string, fn func([]Value) ([]Value, error)) {
	t.RawSet(StringValue(name), FunctionValue(&GoFunction{Name: module + "." + name, Fn: fn}))
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

func llmResultStatus(res LLMTurnResult) string {
	if res.Status != "" {
		return res.Status
	}
	if len(res.Calls) > 0 {
		return "tool_calls"
	}
	return "final_answer"
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

func llmReact(opts *Table, provider LLMProvider, call FunctionCaller, ctx context.Context, maxHostResult int64, trace func(LLMTraceEvent)) ([]Value, error) {
	messagesValue := opts.RawGetString("messages")
	if !messagesValue.IsTable() {
		llmTrace(trace, LLMTraceEvent{Type: "react_error", ErrorKind: "validation", Message: "llm.react requires messages"})
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
	maxToolRetries := int(toInt(opts.RawGetString("max_tool_retries")))
	if maxToolRetries < 0 {
		maxToolRetries = 0
	}
	for step := 0; step < maxSteps; step++ {
		req := LLMTurnRequest{
			Model:     model,
			Messages:  llmMessagesFromValue(llmTableFromValues(history)),
			Tools:     llmToolsFromValue(toolsValue),
			MaxTokens: toInt(opts.RawGetString("max_tokens")),
			Stream:    opts.RawGetString("stream").Truthy(),
		}
		llmTrace(trace, LLMTraceEvent{Type: "turn_start", Model: req.Model, Step: int64(step), MessageCount: len(req.Messages), ToolCount: len(req.Tools)})
		res, err := provider.Turn(ctx, req)
		if err != nil {
			llmTrace(trace, LLMTraceEvent{Type: "turn_error", Model: req.Model, Step: int64(step), ErrorKind: "provider", Message: err.Error()})
			return []Value{NilValue(), llmErrorValue("provider", err.Error())}, nil
		}
		llmTrace(trace, LLMTraceEvent{Type: "turn_end", Model: req.Model, Step: int64(step), Status: llmResultStatus(res), MessageCount: len(req.Messages), ToolCount: len(req.Tools), Usage: res.Usage})
		turnValue := llmResultValue(res)
		if err := CheckHostResultBytes(maxHostResult, turnValue); err != nil {
			return nil, err
		}
		switch res.Status {
		case "", "final_answer":
			llmTrace(trace, LLMTraceEvent{Type: "react_done", Model: model, Step: int64(step), Status: "done"})
			return []Value{llmReactResultValue("done", res.Text, "", turnValue, history), NilValue()}, nil
		case "stop":
			llmTrace(trace, LLMTraceEvent{Type: "react_stopped", Model: model, Step: int64(step), Status: "stopped", Message: res.Reason})
			return []Value{llmReactResultValue("stopped", "", res.Reason, turnValue, history), NilValue()}, nil
		case "tool_calls":
			for i := range res.Calls {
				callValue := llmToolCallValue(res.Calls[i])
				history = append(history, llmAssistantCallMessage(callValue))
				llmTrace(trace, LLMTraceEvent{Type: "tool_call", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID})
				dispatchResult, err := llmDispatchWithRetry(call, callValue.Table(), tools, maxToolRetries, trace, int64(step), res.Calls[i])
				if !err.IsNil() {
					llmTrace(trace, LLMTraceEvent{Type: "tool_fatal", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID, ErrorKind: llmErrorKind(err)})
					return []Value{NilValue(), err}, nil
				}
				if len(dispatchResult) >= 2 && !dispatchResult[1].IsNil() {
					message := dispatchResult[1].Table().RawGetString("message").Str()
					llmTrace(trace, LLMTraceEvent{Type: "tool_error", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID, ErrorKind: llmErrorKind(dispatchResult[1]), Message: message})
					history = append(history, llmToolErrorMessage(res.Calls[i].ID, message))
				} else {
					value := NilValue()
					if len(dispatchResult) > 0 {
						value = dispatchResult[0]
					}
					llmTrace(trace, LLMTraceEvent{Type: "tool_result", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID})
					history = append(history, llmToolResultMessage(res.Calls[i].ID, value))
				}
			}
		default:
			llmTrace(trace, LLMTraceEvent{Type: "react_stopped", Model: model, Step: int64(step), Status: "stopped", Message: res.Status})
			return []Value{llmReactResultValue("stopped", "", res.Status, turnValue, history), NilValue()}, nil
		}
	}
	llmTrace(trace, LLMTraceEvent{Type: "react_stopped", Model: model, Step: int64(maxSteps), Status: "stopped", Message: "max_steps"})
	return []Value{llmReactResultValue("stopped", "", "max_steps", NilValue(), history), NilValue()}, nil
}

func llmTrace(trace func(LLMTraceEvent), event LLMTraceEvent) {
	if trace != nil {
		trace(event)
	}
}

func llmDispatchWithRetry(call FunctionCaller, callTable, tools *Table, maxRetries int, trace func(LLMTraceEvent), step int64, callInfo LLMToolCall) ([]Value, Value) {
	var lastErr Value
	for attempt := 0; attempt <= maxRetries; attempt++ {
		result, err := llmDispatch(call, callTable, tools)
		if err != nil {
			return nil, llmErrorValue("internal", err.Error())
		}
		if len(result) < 2 || result[1].IsNil() {
			return result, NilValue()
		}
		lastErr = result[1]
		kind := llmErrorKind(lastErr)
		if llmRecoverableToolError(kind) {
			return result, NilValue()
		}
		if !llmTransientToolError(kind) {
			return nil, lastErr
		}
		if attempt < maxRetries {
			llmTrace(trace, LLMTraceEvent{Type: "tool_retry", Step: step, Attempt: int64(attempt + 1), Tool: callInfo.Tool, CallID: callInfo.ID, ErrorKind: kind, Message: lastErr.Table().RawGetString("message").Str()})
		}
	}
	return []Value{NilValue(), lastErr}, NilValue()
}

func llmErrorKind(v Value) string {
	if !v.IsTable() {
		return ""
	}
	return v.Table().RawGetString("kind").Str()
}

func llmRecoverableToolError(kind string) bool {
	switch kind {
	case "validation", "policy", "user", "capability":
		return true
	default:
		return false
	}
}

func llmTransientToolError(kind string) bool {
	switch kind {
	case "network", "provider", "internal":
		return true
	default:
		return false
	}
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
