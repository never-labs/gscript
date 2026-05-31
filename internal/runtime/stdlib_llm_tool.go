package runtime

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
