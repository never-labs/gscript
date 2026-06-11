package bind

import (
	"strings"
	"sync"
)

type llmAgentMetadata struct {
	Name        string
	Params      []string
	Output      Value
	Description string
}

var llmAgentMetadataByFunction sync.Map // map[*GoFunction]llmAgentMetadata

func llmAgentMetadataForValue(v Value) (llmAgentMetadata, bool) {
	if gf := v.GoFunction(); gf != nil {
		if meta, ok := llmAgentMetadataByFunction.Load(gf); ok {
			if typed, ok := meta.(llmAgentMetadata); ok {
				return typed, true
			}
		}
		if strings.HasPrefix(gf.Name, "llm.agent.") {
			return llmAgentMetadata{Name: strings.TrimPrefix(gf.Name, "llm.agent.")}, true
		}
	}
	return llmAgentMetadata{}, false
}

func llmFunctionParamNames(v Value) []string {
	if cl := v.Closure(); cl != nil && cl.Proto != nil {
		return append([]string(nil), cl.Proto.Params...)
	}
	return nil
}

func llmStringArrayValue(items []string) Value {
	t := NewSequentialArrayTable(len(items))
	for i, item := range items {
		t.RawSet(IntValue(int64(i+1)), StringValue(item))
	}
	return TableValue(t)
}

func llmAgentCallArgs(meta llmAgentMetadata, args []Value) []Value {
	if len(args) != 1 || !args[0].IsTable() || len(meta.Params) == 0 {
		return args
	}
	t := args[0].Table()
	out := make([]Value, 0, len(meta.Params))
	for _, name := range meta.Params {
		out = append(out, t.RawGetString(name))
	}
	return out
}

func llmAgentToolResultValue(v Value) Value {
	if !v.IsTable() {
		return v
	}
	t := v.Table()
	if value := t.RawGetString("value"); !value.IsNil() {
		return value
	}
	if text := t.RawGetString("text"); !text.IsNil() {
		return text
	}
	if result := t.RawGetString("result"); result.IsTable() {
		if value := result.Table().RawGetString("value"); !value.IsNil() {
			return value
		}
		if text := result.Table().RawGetString("text"); !text.IsNil() {
			return text
		}
	}
	return v
}

func llmAgentToolName(agent Value, meta llmAgentMetadata, fallback string) string {
	name := meta.Name
	if name == "" {
		if gf := agent.GoFunction(); gf != nil {
			name = strings.TrimPrefix(gf.Name, "llm.agent.")
		}
	}
	if name == "" {
		name = fallback
	}
	return name
}

func llmAgentToolWrapper(call ScriptFunctionCaller, agent Value, meta llmAgentMetadata, name string) Value {
	return FunctionValue(&GoFunction{Name: "llm.agent_as_tool." + name, Fn: func(callArgs []Value) ([]Value, error) {
		if call == nil {
			return []Value{NilValue(), llmErrorValue("internal", "agent-as-tool requires a function caller")}, nil
		}
		results, err := call(agent, llmAgentCallArgs(meta, callArgs))
		if err != nil {
			return nil, err
		}
		if len(results) >= 2 && !results[1].IsNil() {
			return []Value{NilValue(), results[1]}, nil
		}
		if len(results) == 0 {
			return []Value{NilValue(), NilValue()}, nil
		}
		result := results[0]
		if result.IsTable() {
			status := result.Table().RawGetString("status").Str()
			switch status {
			case "pending":
				pendErr := llmErrorValue("pending", "delegated agent paused for approval")
				pendErr.Table().RawSetString("pending", result)
				return []Value{NilValue(), pendErr}, nil
			case "stopped":
				reason := result.Table().RawGetString("reason").Str()
				if reason == "" {
					reason = "delegated agent stopped"
				}
				return []Value{NilValue(), llmErrorValue("tool", reason)}, nil
			}
		}
		return []Value{llmAgentToolResultValue(result), NilValue()}, nil
	}})
}

// llmIsAgentValue returns true if v is an agent function value produced by
// llm.agent (it carries an llm.agent.* GoFunction name and has registered
// metadata).
func llmIsAgentValue(v Value) bool {
	if !v.IsFunction() {
		return false
	}
	gf := v.GoFunction()
	if gf == nil {
		return false
	}
	if _, ok := llmAgentMetadataByFunction.Load(gf); ok {
		return true
	}
	return strings.HasPrefix(gf.Name, "llm.agent.")
}

// llmAgentFunctionToToolTable builds a tool table from an agent function value
// so it can be passed directly inside a `tools: [...]` list. The wrapper calls
// the agent through `call`, unwraps result.value, and propagates pending/stopped
// statuses as tool-level signals.
func llmAgentFunctionToToolTable(call ScriptFunctionCaller, agent Value) *Table {
	meta, _ := llmAgentMetadataForValue(agent)
	name := llmAgentToolName(agent, meta, "agent")
	wrapper := llmAgentToolWrapper(call, agent, meta, name)
	tool := NewTable()
	tool.RawSetString("__llm_tool", BoolValue(true))
	tool.RawSetString("__llm_agent_tool", BoolValue(true))
	tool.RawSetString("name", StringValue(name))
	tool.RawSetString("fn", wrapper)
	tool.RawSetString("description", StringValue(meta.Description))
	if len(meta.Params) > 0 {
		tool.RawSetString("params", llmStringArrayValue(meta.Params))
	}
	if !meta.Output.IsNil() {
		tool.RawSetString("output", meta.Output)
	}
	return tool
}

// llmToolsListHasAgents reports whether any entry in the tools list is an
// agent function value (rather than an already-constructed tool table).
func llmToolsListHasAgents(v Value) bool {
	if !v.IsTable() {
		return false
	}
	t := v.Table()
	for i := 1; i <= t.Length(); i++ {
		if llmIsAgentValue(t.RawGet(IntValue(int64(i)))) {
			return true
		}
	}
	return false
}

// llmNormalizeToolsValue scans a tools list value and, for any entry that is an
// agent function value, returns a new list with that entry replaced by a
// synthesized tool table. The original list is not mutated. If no agent values
// are present, the input value is returned unchanged.
func llmNormalizeToolsValue(call ScriptFunctionCaller, v Value) Value {
	if !llmToolsListHasAgents(v) {
		return v
	}
	t := v.Table()
	n := t.Length()
	out := NewSequentialArrayTable(n)
	for i := 1; i <= n; i++ {
		entry := t.RawGet(IntValue(int64(i)))
		if llmIsAgentValue(entry) {
			entry = TableValue(llmAgentFunctionToToolTable(call, entry))
		}
		out.RawSet(IntValue(int64(i)), entry)
	}
	return TableValue(out)
}
