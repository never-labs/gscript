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

func llmToolInfoValue(v Value) Value {
	if !v.IsTable() {
		return NilValue()
	}
	t := v.Table()
	if llmLooksLikeToolTable(t) {
		return TableValue(llmSingleToolContractTable(t))
	}
	out := NewTable()
	for i := 1; i <= t.Length(); i++ {
		tv := t.RawGet(IntValue(int64(i)))
		if !tv.IsTable() || !llmLooksLikeToolTable(tv.Table()) {
			continue
		}
		out.RawSet(IntValue(int64(out.Length()+1)), TableValue(llmSingleToolContractTable(tv.Table())))
	}
	out.RawSetString("kind", StringValue("tool_inventory"))
	out.RawSetString("capabilities", llmToolCapsValue(t))
	return TableValue(out)
}

func llmLooksLikeToolTable(t *Table) bool {
	if t == nil {
		return false
	}
	return t.RawGetString("__llm_tool").Bool() || !t.RawGetString("name").IsNil() || !t.RawGetString("fn").IsNil()
}

func llmSingleToolSchemaTable(tool *Table) *Table {
	out := llmSingleToolContractTable(tool)
	out.RawSetString("kind", StringValue("tool_schema"))
	return out
}

func llmSingleToolContractTable(tool *Table) *Table {
	out := NewTable()
	out.RawSetString("kind", StringValue("tool_contract"))
	name := tool.RawGetString("name").Str()
	out.RawSetString("name", StringValue(name))
	out.RawSetString("tool_name", StringValue(name))
	out.RawSetString("description", StringValue(tool.RawGetString("description").Str()))
	if llmToolIsAgentTool(tool) {
		out.RawSetString("type", StringValue("agent"))
	} else {
		out.RawSetString("type", StringValue("function"))
	}
	if params := tool.RawGetString("params"); params.IsTable() {
		out.RawSetString("params", llmCloneValue(params))
	}
	requires := llmToolCapabilitiesValue(tool)
	if requires.IsTable() {
		out.RawSetString("requires", llmCloneValue(requires))
		out.RawSetString("capabilities", llmCloneValue(requires))
		out.RawSetString("capability_ids", llmCloneValue(requires))
	}
	var inputSchema Value
	if schema := tool.RawGetString("schema"); !schema.IsNil() {
		inputSchema = schema
	} else if params := llmStringSliceFromValue(tool.RawGetString("params")); len(params) > 0 {
		inputSchema = llmToolParamsSchemaValue(params)
	} else if output := tool.RawGetString("output"); !output.IsNil() {
		inputSchema = output
	}
	if !inputSchema.IsNil() {
		out.RawSetString("schema", llmCloneValue(inputSchema))
		out.RawSetString("input_schema", llmCloneValue(inputSchema))
	}
	if output := tool.RawGetString("output"); !output.IsNil() {
		out.RawSetString("output", llmCloneValue(output))
		out.RawSetString("output_schema", llmCloneValue(output))
	}
	if result := llmToolResultValue(tool); !result.IsNil() {
		out.RawSetString("result", llmCloneValue(result))
		out.RawSetString("output_schema", llmCloneValue(result))
	}
	if err := tool.RawGetString("error"); !err.IsNil() {
		out.RawSetString("error", llmCloneValue(err))
	}
	if replayKey := tool.RawGetString("replay_key"); !replayKey.IsNil() {
		out.RawSetString("replay_key", llmCloneValue(replayKey))
	}
	llmSetToolDescriptorString(out, tool, "caller_role", "caller")
	llmSetToolDescriptorString(out, tool, "executor_role", "executor")
	llmSetToolDescriptorString(out, tool, "effect", "read_only")
	llmSetToolDescriptorString(out, tool, "approval_policy", "not_required_for_fixture")
	llmSetToolDescriptorString(out, tool, "provider_wire_format", "none")
	llmSetToolDescriptorBool(out, tool, "live_network", false)
	llmSetToolDescriptorBool(out, tool, "secret_parameters_allowed", false)
	return out
}

func llmSetToolDescriptorString(out, tool *Table, field, fallback string) {
	if value := tool.RawGetString(field); value.IsString() {
		out.RawSetString(field, llmCloneValue(value))
		return
	}
	out.RawSetString(field, StringValue(fallback))
}

func llmSetToolDescriptorBool(out, tool *Table, field string, fallback bool) {
	if value := tool.RawGetString(field); value.IsBool() {
		out.RawSetString(field, llmCloneValue(value))
		return
	}
	out.RawSetString(field, BoolValue(fallback))
}

func llmToolCapabilitiesValue(tool *Table) Value {
	if tool == nil {
		return NilValue()
	}
	if caps := tool.RawGetString("capabilities"); caps.IsTable() {
		return caps
	}
	if requires := tool.RawGetString("requires"); requires.IsTable() {
		return requires
	}
	return NilValue()
}

func llmToolResultValue(tool *Table) Value {
	if tool == nil {
		return NilValue()
	}
	if result := tool.RawGetString("result"); !result.IsNil() {
		return result
	}
	return tool.RawGetString("output")
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
			Requires: llmStringSliceFromValue(llmToolCapabilitiesValue(tool)),
		})
	}
	return out
}

func llmValidateToolContracts(tools *Table) Value {
	if tools == nil {
		return llmErrorValue("validation", "tools table expected")
	}
	for i := 1; i <= tools.Length(); i++ {
		tv := tools.RawGet(IntValue(int64(i)))
		if !tv.IsTable() || !llmLooksLikeToolTable(tv.Table()) {
			err := llmErrorValue("validation", "tool inventory contains non-tool entry")
			et := err.Table()
			et.RawSetString("index", IntValue(int64(i)))
			return err
		}
		if err := llmValidateSingleToolContract(tv.Table(), i); !err.IsNil() {
			return err
		}
	}
	return NilValue()
}

func llmValidateSingleToolContract(tool *Table, index int) Value {
	missing := ""
	switch {
	case tool.RawGetString("name").Str() == "":
		missing = "name"
	case llmToolContractSchemaValue(tool).IsNil():
		missing = "schema"
	case len(llmStringSliceFromValue(llmToolCapabilitiesValue(tool))) == 0:
		missing = "capabilities"
	case llmToolResultValue(tool).IsNil():
		missing = "result"
	case tool.RawGetString("error").IsNil():
		missing = "error"
	case tool.RawGetString("replay_key").IsNil():
		missing = "replay_key"
	}
	if missing == "" {
		return llmValidateToolProviderFreeDescriptor(tool, index)
	}
	err := llmErrorValue("validation", "tool contract missing "+missing+": "+tool.RawGetString("name").Str())
	et := err.Table()
	et.RawSetString("field", StringValue(missing))
	et.RawSetString("tool", StringValue(tool.RawGetString("name").Str()))
	et.RawSetString("index", IntValue(int64(index)))
	return err
}

func llmValidateToolProviderFreeDescriptor(tool *Table, index int) Value {
	if value := tool.RawGetString("provider_wire_format"); value.IsString() && value.Str() != "none" {
		return llmToolContractFieldError(tool, index, "provider_wire_format", "tool contract provider_wire_format must be none")
	}
	if tool.RawGetString("live_network").Bool() {
		return llmToolContractFieldError(tool, index, "live_network", "tool contract live_network must be false")
	}
	if tool.RawGetString("secret_parameters_allowed").Bool() {
		return llmToolContractFieldError(tool, index, "secret_parameters_allowed", "tool contract secret_parameters_allowed must be false")
	}
	if value := tool.RawGetString("effect"); value.IsString() && value.Str() != "read_only" && value.Str() != "effectful" {
		return llmToolContractFieldError(tool, index, "effect", "tool contract effect must be read_only or effectful")
	}
	if value := tool.RawGetString("approval_policy"); value.IsString() && value.Str() != "not_required_for_fixture" && value.Str() != "deny_without_approval" {
		return llmToolContractFieldError(tool, index, "approval_policy", "tool contract approval_policy is unsupported")
	}
	return NilValue()
}

func llmToolContractFieldError(tool *Table, index int, field, message string) Value {
	err := llmErrorValue("validation", message+": "+tool.RawGetString("name").Str())
	et := err.Table()
	et.RawSetString("field", StringValue(field))
	et.RawSetString("tool", StringValue(tool.RawGetString("name").Str()))
	et.RawSetString("index", IntValue(int64(index)))
	return err
}

func llmToolContractSchemaValue(tool *Table) Value {
	if tool == nil {
		return NilValue()
	}
	if schema := tool.RawGetString("schema"); !schema.IsNil() {
		return schema
	}
	if params := llmStringSliceFromValue(tool.RawGetString("params")); len(params) > 0 {
		return llmToolParamsSchemaValue(params)
	}
	if output := tool.RawGetString("output"); !output.IsNil() {
		return output
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
