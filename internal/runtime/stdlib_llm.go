package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
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
	Requires    []string
	Schema      any
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
	ForceTool string
	MaxTokens int64
	Stream    bool
	Stop      []string
	Metadata  map[string]string
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
		var requires Value
		var schema Value
		if len(args) >= 3 && args[2].IsTable() {
			opts := args[2].Table()
			desc = opts.RawGetString("description").Str()
			params = opts.RawGetString("params")
			requires = opts.RawGetString("requires")
			schema = opts.RawGetString("schema")
		}
		tool := NewTable()
		tool.RawSetString("__llm_tool", BoolValue(true))
		tool.RawSetString("name", args[0])
		tool.RawSetString("fn", args[1])
		tool.RawSetString("description", StringValue(desc))
		if params.IsTable() {
			tool.RawSetString("params", params)
		}
		if requires.IsTable() {
			tool.RawSetString("requires", requires)
		}
		if !schema.IsNil() {
			tool.RawSetString("schema", schema)
		}
		return []Value{TableValue(tool)}, nil
	})

	set("tool_caps", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.tool_caps' (tools table expected)")
		}
		return []Value{llmToolCapsValue(args[0].Table())}, nil
	})

	set("check_tools", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() || !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument to 'llm.check_tools' (tools, caps expected)")
		}
		if err := llmCheckToolCaps(args[0].Table(), args[1].Table()); !err.IsNil() {
			return []Value{NilValue(), err}, nil
		}
		return []Value{BoolValue(true), NilValue()}, nil
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

func BuildLLMLoopLib(call FunctionCaller, provider func() LLMProvider, maxHostResult func() int64, ctx func() context.Context, traces ...LLMTraceSink) *Table {
	t := NewTable()
	snapshots := map[string]Value{}
	var snapshotsMu sync.Mutex

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
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{Name: "loop." + name, Fn: fn}))
	}

	set("react", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'loop.react' (table expected)")
		}
		if call == nil {
			return nil, fmt.Errorf("loop.react requires a function caller")
		}
		p := currentProvider()
		if p == nil {
			trace(LLMTraceEvent{Type: "react_error", ErrorKind: "provider", Message: "llm provider not configured"})
			return []Value{NilValue(), llmErrorValue("provider", "llm provider not configured")}, nil
		}
		opts, err := llmLoopOptions(args[0].Table(), 0)
		if err != nil {
			trace(LLMTraceEvent{Type: "react_error", ErrorKind: "validation", Message: err.Error()})
			return []Value{NilValue(), llmErrorValue("validation", err.Error())}, nil
		}
		return llmReact(opts, p, call, currentContext(), hostLimit(), trace)
	})

	set("simple", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'loop.simple' (table expected)")
		}
		if call == nil {
			return nil, fmt.Errorf("loop.simple requires a function caller")
		}
		p := currentProvider()
		if p == nil {
			trace(LLMTraceEvent{Type: "react_error", ErrorKind: "provider", Message: "llm provider not configured"})
			return []Value{NilValue(), llmErrorValue("provider", "llm provider not configured")}, nil
		}
		opts, err := llmLoopOptions(args[0].Table(), 1)
		if err != nil {
			trace(LLMTraceEvent{Type: "react_error", ErrorKind: "validation", Message: err.Error()})
			return []Value{NilValue(), llmErrorValue("validation", err.Error())}, nil
		}
		return llmReact(opts, p, call, currentContext(), hostLimit(), trace)
	})

	set("snapshot", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() || !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument to 'loop.snapshot' (history, pending_call expected)")
		}
		token, err := llmSnapshotToken()
		if err != nil {
			return nil, err
		}
		snapshot := NewTable()
		snapshot.RawSetString("history", args[0])
		snapshot.RawSetString("pending", args[1])
		snapshotsMu.Lock()
		snapshots[token] = TableValue(snapshot)
		snapshotsMu.Unlock()
		return []Value{StringValue(token)}, nil
	})

	set("resume", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument to 'loop.resume' (token, approval expected)")
		}
		snapshotsMu.Lock()
		snapshot, ok := snapshots[args[0].Str()]
		if !ok || !snapshot.IsTable() {
			snapshotsMu.Unlock()
			return []Value{NilValue(), llmErrorValue("validation", "snapshot not found")}, nil
		}
		delete(snapshots, args[0].Str())
		snapshotsMu.Unlock()
		var tools *Table
		if len(args) >= 3 && args[2].IsTable() {
			tools = args[2].Table()
		}
		return llmResumeSnapshot(snapshot.Table(), args[1].Table(), tools, call)
	})

	return t
}

func llmSnapshotToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("loop.snapshot: failed to generate token: %v", err)
	}
	return hex.EncodeToString(b[:]), nil
}

func llmResumeSnapshot(snapshot, approval, tools *Table, call FunctionCaller) ([]Value, error) {
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

func llmLoopOptions(src *Table, defaultMaxSteps int64) (*Table, error) {
	opts := NewTable()
	if messages := src.RawGetString("messages"); messages.IsTable() {
		opts.RawSetString("messages", messages)
	} else {
		user := src.RawGetString("user")
		if user.IsNil() {
			return nil, fmt.Errorf("loop requires messages or user")
		}
		messages := NewAppendArrayTable(2)
		if system := src.RawGetString("system"); !system.IsNil() {
			messages.RawSet(IntValue(int64(messages.Length()+1)), TableValue(llmMessageTable("system", system.Str())))
		}
		messages.RawSet(IntValue(int64(messages.Length()+1)), TableValue(llmMessageTable("user", user.Str())))
		opts.RawSetString("messages", TableValue(messages))
	}
	for _, key := range []string{
		"model",
		"tools",
		"max_tokens",
		"stream",
		"max_steps",
		"max_tool_retries",
		"max_history_tokens",
		"force_tool",
		"stop",
		"metadata",
		"budget",
		"budget_tokens",
		"budget_turns",
		"budget_calls",
		"budget_money",
		"ctx",
		"context",
		"cancel",
	} {
		if v := src.RawGetString(key); !v.IsNil() {
			opts.RawSetString(key, v)
		}
	}
	if defaultMaxSteps > 0 && opts.RawGetString("max_steps").IsNil() {
		opts.RawSetString("max_steps", IntValue(defaultMaxSteps))
	}
	return opts, nil
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
		ForceTool: llmForceToolFromValue(t.RawGetString("force_tool")),
		MaxTokens: toInt(t.RawGetString("max_tokens")),
		Stream:    t.RawGetString("stream").Truthy(),
		Stop:      llmStringSliceFromValue(t.RawGetString("stop")),
		Metadata:  llmStringMapFromValue(t.RawGetString("metadata")),
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
			Requires:    llmStringSliceFromValue(tt.RawGetString("requires")),
			Schema:      llmAnyFromValue(tt.RawGetString("schema")),
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

func llmForceToolFromValue(v Value) string {
	if v.IsString() {
		return v.Str()
	}
	if v.IsTable() {
		return v.Table().RawGetString("name").Str()
	}
	return ""
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

func llmToolCapsValue(tools *Table) Value {
	caps := NewTable()
	seen := map[string]bool{}
	if tools == nil {
		return TableValue(caps)
	}
	for i := 1; i <= tools.Length(); i++ {
		tv := tools.RawGet(IntValue(int64(i)))
		if !tv.IsTable() {
			continue
		}
		for _, cap := range llmStringSliceFromValue(tv.Table().RawGetString("requires")) {
			if cap == "" || seen[cap] {
				continue
			}
			seen[cap] = true
			caps.RawSet(IntValue(int64(caps.Length()+1)), StringValue(cap))
		}
	}
	return TableValue(caps)
}

func llmCheckToolCaps(tools, caps *Table) Value {
	allowed := map[string]bool{}
	for _, cap := range llmStringSliceFromValue(TableValue(caps)) {
		allowed[cap] = true
	}
	if allowed["all"] || allowed["cap.all"] || allowed["*"] {
		return NilValue()
	}
	if allowed["none"] || allowed["cap.none"] {
		allowed = map[string]bool{}
	}
	if tools == nil {
		return NilValue()
	}
	for i := 1; i <= tools.Length(); i++ {
		tv := tools.RawGet(IntValue(int64(i)))
		if !tv.IsTable() {
			continue
		}
		tool := tv.Table()
		toolName := tool.RawGetString("name").Str()
		for _, cap := range llmStringSliceFromValue(tool.RawGetString("requires")) {
			if cap == "" || cap == "none" || cap == "cap.none" || allowed[cap] {
				continue
			}
			err := llmErrorValue("capability", "missing capability: "+cap)
			et := err.Table()
			et.RawSetString("capability", StringValue(cap))
			et.RawSetString("tool", StringValue(toolName))
			return err
		}
	}
	return NilValue()
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
	maxHistoryTokens := toInt(opts.RawGetString("max_history_tokens"))
	if maxHistoryTokens < 0 {
		maxHistoryTokens = 0
	}
	budget := llmBudgetFromOptions(opts)
	cancel := llmCancelFromOptions(opts, ctx)
	for step := 0; step < maxSteps; step++ {
		if err := cancel.check(); !err.IsNil() {
			llmTrace(trace, LLMTraceEvent{Type: "react_error", ErrorKind: llmErrorKind(err), Message: err.Table().RawGetString("message").Str()})
			return []Value{NilValue(), err}, nil
		}
		if err := budget.beforeTurn(); !err.IsNil() {
			llmTrace(trace, LLMTraceEvent{Type: "react_error", ErrorKind: llmErrorKind(err), Message: err.Table().RawGetString("message").Str()})
			return []Value{NilValue(), err}, nil
		}
		requestHistory := history
		if maxHistoryTokens > 0 {
			requestHistory = chatWindow(llmTableFromValues(history).Table(), maxHistoryTokens)
		}
		req := LLMTurnRequest{
			Model:     model,
			Messages:  llmMessagesFromValue(llmTableFromValues(requestHistory)),
			Tools:     llmToolsFromValue(toolsValue),
			ForceTool: llmForceToolFromValue(opts.RawGetString("force_tool")),
			MaxTokens: toInt(opts.RawGetString("max_tokens")),
			Stream:    opts.RawGetString("stream").Truthy(),
			Stop:      llmStringSliceFromValue(opts.RawGetString("stop")),
			Metadata:  llmStringMapFromValue(opts.RawGetString("metadata")),
		}
		llmTrace(trace, LLMTraceEvent{Type: "turn_start", Model: req.Model, Step: int64(step), MessageCount: len(req.Messages), ToolCount: len(req.Tools)})
		res, err := provider.Turn(ctx, req)
		if err != nil {
			llmTrace(trace, LLMTraceEvent{Type: "turn_error", Model: req.Model, Step: int64(step), ErrorKind: "provider", Message: err.Error()})
			return []Value{NilValue(), llmErrorValue("provider", err.Error())}, nil
		}
		if err := cancel.check(); !err.IsNil() {
			llmTrace(trace, LLMTraceEvent{Type: "react_error", ErrorKind: llmErrorKind(err), Message: err.Table().RawGetString("message").Str()})
			return []Value{NilValue(), err}, nil
		}
		llmTrace(trace, LLMTraceEvent{Type: "turn_end", Model: req.Model, Step: int64(step), Status: llmResultStatus(res), MessageCount: len(req.Messages), ToolCount: len(req.Tools), Usage: res.Usage})
		turnValue := llmResultValue(res)
		if err := CheckHostResultBytes(maxHostResult, turnValue); err != nil {
			return nil, err
		}
		budget.chargeTurn(res.Usage)
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
				if err := cancel.check(); !err.IsNil() {
					llmTrace(trace, LLMTraceEvent{Type: "tool_fatal", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID, ErrorKind: llmErrorKind(err)})
					return []Value{NilValue(), err}, nil
				}
				if err := budget.beforeToolCall(); !err.IsNil() {
					llmTrace(trace, LLMTraceEvent{Type: "tool_fatal", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID, ErrorKind: llmErrorKind(err)})
					return []Value{NilValue(), err}, nil
				}
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

type llmBudget struct {
	maxTokens int64
	maxTurns  int64
	maxCalls  int64
	maxMoney  float64

	usedTokens int64
	usedTurns  int64
	usedCalls  int64
	usedMoney  float64
}

func llmBudgetFromOptions(opts *Table) *llmBudget {
	b := &llmBudget{
		maxTokens: -1,
		maxTurns:  -1,
		maxCalls:  -1,
		maxMoney:  -1,
	}
	if v := opts.RawGetString("budget_tokens"); !v.IsNil() {
		b.maxTokens = toInt(v)
	}
	if v := opts.RawGetString("budget_turns"); !v.IsNil() {
		b.maxTurns = toInt(v)
	}
	if v := opts.RawGetString("budget_calls"); !v.IsNil() {
		b.maxCalls = toInt(v)
	}
	if v := opts.RawGetString("budget_money"); !v.IsNil() {
		b.maxMoney = toFloat(v)
	}
	if t := opts.RawGetString("budget"); t.IsTable() {
		bt := t.Table()
		if v := bt.RawGetString("tokens"); !v.IsNil() {
			b.maxTokens = toInt(v)
		}
		if v := bt.RawGetString("turns"); !v.IsNil() {
			b.maxTurns = toInt(v)
		}
		if v := bt.RawGetString("calls"); !v.IsNil() {
			b.maxCalls = toInt(v)
		}
		if v := bt.RawGetString("money"); !v.IsNil() {
			b.maxMoney = toFloat(v)
		}
	}
	if b.maxTokens < 0 {
		b.maxTokens = -1
	}
	if b.maxTurns < 0 {
		b.maxTurns = -1
	}
	if b.maxCalls < 0 {
		b.maxCalls = -1
	}
	if b.maxMoney < 0 {
		b.maxMoney = -1
	}
	return b
}

func (b *llmBudget) beforeTurn() Value {
	if b == nil {
		return NilValue()
	}
	if b.maxTurns >= 0 && b.usedTurns >= b.maxTurns {
		return llmBudgetError("turns", b.maxTurns, b.usedTurns)
	}
	if b.maxTokens >= 0 && b.usedTokens >= b.maxTokens {
		return llmBudgetError("tokens", b.maxTokens, b.usedTokens)
	}
	if b.maxMoney >= 0 && b.usedMoney >= b.maxMoney {
		return llmBudgetError("money", 0, 0)
	}
	b.usedTurns++
	return NilValue()
}

func (b *llmBudget) chargeTurn(usage LLMTurnUsage) {
	if b == nil {
		return
	}
	b.usedTokens += usage.InputTokens + usage.OutputTokens
	b.usedMoney += usage.Cost
}

func (b *llmBudget) beforeToolCall() Value {
	if b == nil {
		return NilValue()
	}
	if b.maxCalls >= 0 && b.usedCalls >= b.maxCalls {
		return llmBudgetError("calls", b.maxCalls, b.usedCalls)
	}
	b.usedCalls++
	return NilValue()
}

func llmBudgetError(dimension string, limit, used int64) Value {
	t := NewTable()
	t.RawSetString("kind", StringValue("budget"))
	t.RawSetString("dimension", StringValue(dimension))
	t.RawSetString("message", StringValue("llm budget exceeded: "+dimension))
	if limit > 0 {
		t.RawSetString("limit", IntValue(limit))
	}
	if used > 0 {
		t.RawSetString("used", IntValue(used))
	}
	return TableValue(t)
}

type llmCancel struct {
	host context.Context
	done *Channel
	err  Value
}

func llmCancelFromOptions(opts *Table, host context.Context) llmCancel {
	c := llmCancel{host: host}
	for _, key := range []string{"ctx", "context", "cancel"} {
		if done, errFn, ok := contextDoneAndErr(opts.RawGetString(key)); ok {
			c.done = done
			c.err = errFn
			break
		}
	}
	return c
}

func (c llmCancel) check() Value {
	if c.host != nil {
		if err := c.host.Err(); err != nil {
			return llmCancelError(err.Error())
		}
	}
	if c.done != nil {
		if reason, cancelled := contextCancelledValue(c.done, c.err); cancelled {
			if reason.IsNil() {
				reason = StringValue("cancelled")
			}
			return llmCancelError(reason.String())
		}
	}
	return NilValue()
}

func llmCancelError(reason string) Value {
	if reason == "" {
		reason = "cancelled"
	}
	kind := "cancelled"
	if reason == "deadline exceeded" || reason == "context deadline exceeded" {
		kind = "deadline"
	}
	t := NewTable()
	t.RawSetString("kind", StringValue(kind))
	t.RawSetString("message", StringValue(reason))
	return TableValue(t)
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
