package bind

import (
	"strings"

	stdlibllm "github.com/never-labs/leia/internal/stdlib/lib/llm"
)

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
			Schema:      llmToolProviderSchema(tt),
		})
	}
	return out
}

func llmToolProviderSchema(tool *Table) any {
	if tool == nil {
		return nil
	}
	if schema := tool.RawGetString("schema"); !schema.IsNil() {
		return llmAnyFromValue(schema)
	}
	if llmToolIsAgentTool(tool) {
		return nil
	}
	hasOutputMetadata := !tool.RawGetString("output").IsNil()
	if params := llmStringSliceFromValue(tool.RawGetString("params")); hasOutputMetadata && len(params) > 0 {
		return llmAnyFromValue(llmToolParamsSchemaValue(params))
	}
	if output := tool.RawGetString("output"); !output.IsNil() {
		return llmAnyFromValue(output)
	}
	return nil
}

func llmToolIsAgentTool(tool *Table) bool {
	if tool == nil {
		return false
	}
	if tool.RawGetString("__llm_agent_tool").Bool() {
		return true
	}
	if fn := tool.RawGetString("fn").GoFunction(); fn != nil {
		return strings.HasPrefix(fn.Name, "llm.agent_as_tool.")
	}
	return false
}

func llmToolParamsSchemaValue(params []string) Value {
	schema := NewTable()
	schema.RawSetString("type", StringValue("object"))
	props := NewTable()
	required := NewSequentialArrayTable(len(params))
	for i, name := range params {
		prop := NewTable()
		prop.RawSetString("type", StringValue("string"))
		props.RawSetString(name, TableValue(prop))
		required.RawSet(IntValue(int64(i+1)), StringValue(name))
	}
	schema.RawSetString("properties", TableValue(props))
	schema.RawSetString("required", TableValue(required))
	return TableValue(schema)
}

func llmToolSchemaValue(v Value) Value {
	if !v.IsTable() {
		return NilValue()
	}
	t := v.Table()
	if llmLooksLikeToolTable(t) {
		return TableValue(llmSingleToolSchemaTable(t))
	}
	out := NewTable()
	for i := 1; i <= t.Length(); i++ {
		tv := t.RawGet(IntValue(int64(i)))
		if !tv.IsTable() || !llmLooksLikeToolTable(tv.Table()) {
			continue
		}
		out.RawSet(IntValue(int64(out.Length()+1)), TableValue(llmSingleToolSchemaTable(tv.Table())))
	}
	return TableValue(out)
}

func llmLooksLikeToolTable(t *Table) bool {
	if t == nil {
		return false
	}
	return t.RawGetString("__llm_tool").Bool() || !t.RawGetString("name").IsNil() || !t.RawGetString("fn").IsNil()
}

func llmSingleToolSchemaTable(tool *Table) *Table {
	out := NewTable()
	out.RawSetString("name", StringValue(tool.RawGetString("name").Str()))
	out.RawSetString("description", StringValue(tool.RawGetString("description").Str()))
	if params := tool.RawGetString("params"); params.IsTable() {
		out.RawSetString("params", llmCloneValue(params))
	}
	if requires := tool.RawGetString("requires"); requires.IsTable() {
		out.RawSetString("requires", llmCloneValue(requires))
	}
	if schema := tool.RawGetString("schema"); !schema.IsNil() {
		out.RawSetString("schema", llmCloneValue(schema))
	} else if params := llmStringSliceFromValue(tool.RawGetString("params")); len(params) > 0 {
		out.RawSetString("schema", llmToolParamsSchemaValue(params))
	} else if output := tool.RawGetString("output"); !output.IsNil() {
		out.RawSetString("schema", llmCloneValue(output))
	}
	if output := tool.RawGetString("output"); !output.IsNil() {
		out.RawSetString("output", llmCloneValue(output))
	}
	return out
}

func llmCloneValue(v Value) Value {
	if !v.IsTable() {
		return v
	}
	src := v.Table()
	out := NewTable()
	for _, key := range src.PairsKeysSnapshot() {
		out.RawSet(key, llmCloneValue(src.RawGet(key)))
	}
	return TableValue(out)
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

func llmToolCapsValue(tools *Table) Value {
	caps := NewTable()
	for _, cap := range stdlibllm.ToolCapabilities(llmToolSummaries(tools)) {
		caps.RawSet(IntValue(int64(caps.Length()+1)), StringValue(cap))
	}
	return TableValue(caps)
}

func llmCheckToolCaps(tools, caps *Table) Value {
	missing := stdlibllm.CheckToolCapabilities(llmToolSummaries(tools), llmStringSliceFromValue(TableValue(caps)))
	if missing == nil {
		return NilValue()
	}
	err := llmErrorValue("capability", "missing capability: "+missing.Capability)
	et := err.Table()
	et.RawSetString("capability", StringValue(missing.Capability))
	et.RawSetString("tool", StringValue(missing.Tool))
	return err
}

func llmToolSummaries(tools *Table) []stdlibllm.ToolSummary {
	if tools == nil {
		return nil
	}
	out := make([]stdlibllm.ToolSummary, 0, tools.Length())
	for i := 1; i <= tools.Length(); i++ {
		tv := tools.RawGet(IntValue(int64(i)))
		if !tv.IsTable() {
			continue
		}
		tool := tv.Table()
		out = append(out, stdlibllm.ToolSummary{
			Name:     tool.RawGetString("name").Str(),
			Requires: llmStringSliceFromValue(tool.RawGetString("requires")),
		})
	}
	return out
}

func llmDispatch(call ScriptFunctionCaller, callTable, tools *Table) ([]Value, error) {
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

func llmDispatchWithRetry(call ScriptFunctionCaller, callTable, tools *Table, maxRetries int, trace func(LLMTraceEvent), step int64, callInfo LLMToolCall) ([]Value, Value) {
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
