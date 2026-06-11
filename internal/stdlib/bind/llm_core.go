package bind

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	stdlibllm "github.com/never-labs/leia/internal/stdlib/lib/llm"
)

// BuildLLMLib creates the "llm" standard library table. It is the first-stage
// runtime substrate for the agent layer: future syntax can compile to these
// functions without changing provider or tool-dispatch semantics.
func BuildLLMLib(call ScriptFunctionCaller, provider func() LLMProvider, providerFactory func() LLMProviderFactory, maxHostResult func() int64, ctx func() context.Context, traces ...LLMTraceSink) *Table {
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
	currentProviderFactory := func() LLMProviderFactory {
		if providerFactory == nil {
			return nil
		}
		return providerFactory()
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
	agentDefaults := NewTable()
	modelAliases := NewTable()
	var agentConfigMu sync.RWMutex
	var agentContextMu sync.Mutex
	var ambientAgents []*Table
	var budgetMu sync.Mutex
	var ambientBudgets []*llmBudget

	pushBudget := func(b *llmBudget) {
		budgetMu.Lock()
		ambientBudgets = append(ambientBudgets, b)
		budgetMu.Unlock()
	}
	popBudget := func(b *llmBudget) {
		budgetMu.Lock()
		defer budgetMu.Unlock()
		for i := len(ambientBudgets) - 1; i >= 0; i-- {
			if ambientBudgets[i] == b {
				copy(ambientBudgets[i:], ambientBudgets[i+1:])
				ambientBudgets[len(ambientBudgets)-1] = nil
				ambientBudgets = ambientBudgets[:len(ambientBudgets)-1]
				return
			}
		}
	}
	currentBudgets := func() llmBudgetGroup {
		budgetMu.Lock()
		defer budgetMu.Unlock()
		out := make(llmBudgetGroup, len(ambientBudgets))
		copy(out, ambientBudgets)
		return out
	}
	pushAgent := func(config *Table) {
		if config == nil {
			return
		}
		agentContextMu.Lock()
		ambientAgents = append(ambientAgents, config)
		agentContextMu.Unlock()
	}
	popAgent := func(config *Table) {
		if config == nil {
			return
		}
		agentContextMu.Lock()
		defer agentContextMu.Unlock()
		for i := len(ambientAgents) - 1; i >= 0; i-- {
			if ambientAgents[i] == config {
				copy(ambientAgents[i:], ambientAgents[i+1:])
				ambientAgents[len(ambientAgents)-1] = nil
				ambientAgents = ambientAgents[:len(ambientAgents)-1]
				return
			}
		}
	}
	currentAgentConfig := func() *Table {
		agentContextMu.Lock()
		defer agentContextMu.Unlock()
		if len(ambientAgents) == 0 {
			return nil
		}
		out := NewTable()
		for _, config := range ambientAgents {
			llmCopyTable(out, config, true)
		}
		return out
	}

	set := func(name string, fn func([]Value) ([]Value, error)) { setLLMFunction(t, "llm", name, fn) }

	newToolValue := func(name string, fn Value, opts Value) Value {
		desc := ""
		var params Value
		var requires Value
		var schema Value
		var output Value
		if opts.IsTable() {
			optTable := opts.Table()
			desc = optTable.RawGetString("description").Str()
			params = optTable.RawGetString("params")
			requires = optTable.RawGetString("requires")
			schema = optTable.RawGetString("schema")
			output = optTable.RawGetString("output")
		}
		tool := NewTable()
		tool.RawSetString("__llm_tool", BoolValue(true))
		tool.RawSetString("name", StringValue(name))
		tool.RawSetString("fn", fn)
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
		if !output.IsNil() {
			tool.RawSetString("output", output)
		}
		return TableValue(tool)
	}

	registerLLMMessageConstructors(t, "llm")
	registerLLMMemoryHelpers(t)
	set("assistantCall", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.assistantCall' (table expected)")
		}
		return []Value{TableValue(llmAssistantCallMessageTable(args[0]))}, nil
	})
	set("assistant_call", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.assistant_call' (table expected)")
		}
		return []Value{TableValue(llmAssistantCallMessageTable(args[0]))}, nil
	})
	set("toolResult", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'llm.toolResult'")
		}
		return []Value{TableValue(llmToolResultMessageTable(args[0], args[1]))}, nil
	})
	set("tool_result", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'llm.tool_result'")
		}
		return []Value{TableValue(llmToolResultMessageTable(args[0], args[1]))}, nil
	})
	set("toolError", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'llm.toolError'")
		}
		return []Value{TableValue(llmToolErrorMessageTable(args[0], args[1].Str()))}, nil
	})
	set("tool_error", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'llm.tool_error'")
		}
		return []Value{TableValue(llmToolErrorMessageTable(args[0], args[1].Str()))}, nil
	})

	set("tool", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsFunction() {
			return nil, fmt.Errorf("bad argument to 'llm.tool' (name, function expected)")
		}
		opts := NilValue()
		if len(args) >= 3 {
			opts = args[2]
		}
		return []Value{newToolValue(args[0].Str(), args[1], opts)}, nil
	})

	agentAsTool := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsFunction() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.toolof' (agent function expected)")
		}
		if len(args) >= 2 && !args[1].IsNil() && !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument #2 to 'llm.toolof' (options table expected)")
		}
		if call == nil {
			return nil, fmt.Errorf("llm.toolof requires a function caller")
		}
		agent := args[0]
		opts := NilValue()
		if len(args) >= 2 {
			opts = args[1]
		}
		meta, _ := llmAgentMetadataForValue(agent)
		name := meta.Name
		if opts.IsTable() && opts.Table().RawGetString("name").IsString() {
			name = opts.Table().RawGetString("name").Str()
		}
		if name == "" {
			name = "agent"
		}
		wrapper := FunctionValue(&GoFunction{Name: "llm.agent_as_tool." + name, Fn: func(callArgs []Value) ([]Value, error) {
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
			return []Value{llmAgentToolResultValue(results[0]), NilValue()}, nil
		}})
		tool := newToolValue(name, wrapper, opts)
		if tt := tool.Table(); tt != nil {
			tt.RawSetString("__llm_agent_tool", BoolValue(true))
			if !tt.RawGetString("params").IsTable() && len(meta.Params) > 0 {
				tt.RawSetString("params", llmStringArrayValue(meta.Params))
			}
			if tt.RawGetString("output").IsNil() && !meta.Output.IsNil() {
				tt.RawSetString("output", meta.Output)
			}
		}
		return []Value{tool}, nil
	}
	set("toolof", agentAsTool)
	set("agent_as_tool", agentAsTool)

	set("tool_caps", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.tool_caps' (tools table expected)")
		}
		return []Value{llmToolCapsValue(args[0].Table())}, nil
	})

	toolSchema := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.tool_schema' (tool or tools table expected)")
		}
		return []Value{llmToolSchemaValue(args[0])}, nil
	}
	set("tool_schema", toolSchema)
	set("toolSchema", toolSchema)

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
		agentConfigMu.RLock()
		var opts *Table
		if ambient := currentAgentConfig(); ambient != nil {
			opts = llmMergeTables(ambient, args[0].Table())
		} else {
			opts = llmCloneTable(args[0].Table())
		}
		if tv := opts.RawGetString("tools"); llmToolsListHasAgents(tv) {
			opts.RawSetString("tools", llmNormalizeToolsValue(call, tv))
		}
		onStream := opts.RawGetString("on_stream")
		if onStream.IsNil() {
			onStream = opts.RawGetString("onStream")
		}
		p, providerErr := llmResolveProviderForModel(opts, modelAliases, currentProvider(), currentProviderFactory())
		if !providerErr.IsNil() {
			agentConfigMu.RUnlock()
			trace(LLMTraceEvent{Type: "turn_error", ErrorKind: "provider", Message: providerErr.Table().RawGetString("message").Str()})
			return []Value{NilValue(), providerErr}, nil
		}
		llmResolveModelAlias(opts, modelAliases)
		if p == nil {
			agentConfigMu.RUnlock()
			trace(LLMTraceEvent{Type: "turn_error", ErrorKind: "provider", Message: "llm provider not configured"})
			return []Value{NilValue(), llmErrorValue("provider", "llm provider not configured")}, nil
		}
		if opts.RawGetString("messages").IsNil() {
			memoryContext := opts.RawGetString("context")
			memoryEvidence := opts.RawGetString("evidence")
			normalized, err := llmLoopOptions(opts, stdlibllm.DefaultSimpleMaxSteps)
			if err != nil {
				agentConfigMu.RUnlock()
				trace(LLMTraceEvent{Type: "turn_error", ErrorKind: "validation", Message: err.Error()})
				return []Value{NilValue(), llmErrorValue("validation", err.Error())}, nil
			}
			opts = normalized
			if !memoryContext.IsNil() {
				opts.RawSetString("context", memoryContext)
			}
			if !memoryEvidence.IsNil() {
				opts.RawSetString("evidence", memoryEvidence)
			}
		} else if opts.RawGetString("response_format").IsNil() && opts.RawGetString("output").IsTable() {
			// When an ambient agent (typically a flow agent) declares output:
			// forward a json_object response_format to the provider so the
			// model knows the requested shape. Auto-validation is left to the
			// flow body via llm.validate_output.
			opts.RawSetString("response_format", TableValue(llmJSONResponseFormatTable()))
		}
		llmApplyMemoryContext(opts)
		if !onStream.IsNil() {
			if !onStream.IsFunction() {
				agentConfigMu.RUnlock()
				trace(LLMTraceEvent{Type: "turn_error", ErrorKind: "validation", Message: "llm.turn on_stream must be a function"})
				return []Value{NilValue(), llmErrorValue("validation", "llm.turn on_stream must be a function")}, nil
			}
			opts.RawSetString("stream", BoolValue(true))
		}
		agentConfigMu.RUnlock()
		req, err := llmRequestFromTable(opts)
		if err != nil {
			trace(LLMTraceEvent{Type: "turn_error", ErrorKind: "validation", Message: err.Error()})
			return []Value{NilValue(), llmErrorValue("validation", err.Error())}, nil
		}
		budgets := currentBudgets().with(llmBudgetFromOptions(opts))
		if err := budgets.beforeTurn(); !err.IsNil() {
			trace(LLMTraceEvent{Type: "turn_error", Model: req.Model, ErrorKind: llmErrorKind(err), Message: err.Table().RawGetString("message").Str()})
			return []Value{NilValue(), err}, nil
		}
		trace(LLMTraceEvent{Type: "turn_start", Model: req.Model, MessageCount: len(req.Messages), ToolCount: len(req.Tools)})
		res, err := llmTurnWithOptionalStream(currentContext(), p, req, trace, LLMTraceEvent{}, call, onStream)
		if err != nil {
			trace(LLMTraceEvent{Type: "turn_error", Model: req.Model, ErrorKind: ClassifyLLMProviderError(err), Message: err.Error()})
			return []Value{NilValue(), llmProviderErrorValue(err)}, nil
		}
		trace(LLMTraceEvent{Type: "turn_end", Model: req.Model, Status: llmResultStatus(res), MessageCount: len(req.Messages), ToolCount: len(req.Tools), Usage: res.Usage})
		out := llmResultValue(res)
		if err := CheckHostResultBytes(hostLimit(), out); err != nil {
			return nil, err
		}
		budgets.chargeTurn(res.Usage)
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
		result, err := llmReact(args[0].Table(), p, call, currentContext(), hostLimit(), trace, currentBudgets())
		if err != nil {
			return nil, err
		}
		return result, nil
	})

	set("with_budget", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
			return nil, fmt.Errorf("bad argument to 'llm.with_budget' (budget table, function expected)")
		}
		if call == nil {
			return nil, fmt.Errorf("llm.with_budget requires a function caller")
		}
		budget := llmBudgetFromConfig(args[0].Table())
		pushBudget(budget)
		defer popBudget(budget)
		return call(args[1], nil)
	})

	validateOutput := func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'llm.validate_output' (value, schema expected)")
		}
		value := args[0]
		schema := args[1]
		// Accept either a decoded table value or a JSON string for the value.
		if value.IsString() {
			dec := json.NewDecoder(strings.NewReader(value.Str()))
			dec.UseNumber()
			var raw any
			if err := dec.Decode(&raw); err != nil {
				return []Value{BoolValue(false), StringValue("value is not valid JSON: " + err.Error())}, nil
			}
			value = JSONGoToValue(raw)
		}
		if !schema.IsTable() {
			return []Value{BoolValue(false), StringValue("schema must be a table example")}, nil
		}
		if !value.IsTable() {
			return []Value{BoolValue(false), StringValue("value must decode to a table")}, nil
		}
		if msg := llmValidateStructuredOutputShape(schema.Table(), value.Table()); msg != "" {
			return []Value{BoolValue(false), StringValue(msg)}, nil
		}
		return []Value{BoolValue(true), StringValue("")}, nil
	}
	set("validate_output", validateOutput)
	set("validateOutput", validateOutput)

	set("agent_defaults", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.agent_defaults' (table expected)")
		}
		agentConfigMu.Lock()
		agentDefaults = llmCloneTable(args[0].Table())
		agentConfigMu.Unlock()
		return []Value{BoolValue(true), NilValue()}, nil
	})

	registerModels := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.register_models' (table expected)")
		}
		if err := llmValidateModelAliases(args[0].Table()); err != nil {
			return nil, err
		}
		agentConfigMu.Lock()
		modelAliases = llmCloneTable(args[0].Table())
		agentConfigMu.Unlock()
		return []Value{BoolValue(true), NilValue()}, nil
	}
	set("models", registerModels)
	set("register_models", registerModels)

	runAgentConfig := func(src *Table) ([]Value, error) {
		if call == nil {
			return nil, fmt.Errorf("llm.run_agent requires a function caller")
		}
		agentConfigMu.RLock()
		merged := llmMergeTables(agentDefaults, src)
		p, providerErr := llmResolveProviderForModel(merged, modelAliases, currentProvider(), currentProviderFactory())
		if !providerErr.IsNil() {
			agentConfigMu.RUnlock()
			trace(LLMTraceEvent{Type: "react_error", ErrorKind: "provider", Message: providerErr.Table().RawGetString("message").Str()})
			return []Value{NilValue(), providerErr}, nil
		}
		llmResolveModelAlias(merged, modelAliases)
		if p == nil {
			agentConfigMu.RUnlock()
			trace(LLMTraceEvent{Type: "react_error", ErrorKind: "provider", Message: "llm provider not configured"})
			return []Value{NilValue(), llmErrorValue("provider", "llm provider not configured")}, nil
		}
		memoryContext := merged.RawGetString("context")
		memoryEvidence := merged.RawGetString("evidence")
		opts, err := llmLoopOptions(merged, 0)
		agentConfigMu.RUnlock()
		if err == nil {
			if !memoryContext.IsNil() {
				opts.RawSetString("context", memoryContext)
			}
			if !memoryEvidence.IsNil() {
				opts.RawSetString("evidence", memoryEvidence)
			}
			if tv := opts.RawGetString("tools"); llmToolsListHasAgents(tv) {
				opts.RawSetString("tools", llmNormalizeToolsValue(call, tv))
			}
			llmApplyMemoryContext(opts)
		}
		if err != nil {
			trace(LLMTraceEvent{Type: "react_error", ErrorKind: "validation", Message: err.Error()})
			return []Value{NilValue(), llmErrorValue("validation", err.Error())}, nil
		}
		result, err := llmReact(opts, p, call, currentContext(), hostLimit(), trace, currentBudgets())
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	set("run_agent", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.run_agent' (table expected)")
		}
		return runAgentConfig(args[0].Table())
	})
	registerLLMSectionHelpers(t, runAgentConfig)

	set("agent", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsFunction() {
			return nil, fmt.Errorf("bad argument to 'llm.agent' (name, config function expected)")
		}
		if len(args) >= 3 && !args[2].IsNil() && !args[2].IsFunction() {
			return nil, fmt.Errorf("bad argument #3 to 'llm.agent' (flow function expected)")
		}
		if call == nil {
			return nil, fmt.Errorf("llm.agent requires a function caller")
		}
		name := args[0].Str()
		configFn := args[1]
		flowFn := NilValue()
		if len(args) >= 3 {
			flowFn = args[2]
		}
		metadata := llmAgentMetadata{Name: name, Params: llmFunctionParamNames(configFn)}
		if len(args) >= 4 && args[3].IsTable() {
			metaTable := args[3].Table()
			if params := metaTable.RawGetString("params"); params.IsTable() {
				metadata.Params = llmStringSliceFromValue(params)
			}
			if output := metaTable.RawGetString("output"); !output.IsNil() {
				metadata.Output = output
			}
			if desc := metaTable.RawGetString("description"); desc.IsString() {
				metadata.Description = desc.Str()
			}
		}
		agentFn := &GoFunction{Name: "llm.agent." + name, Fn: func(callArgs []Value) ([]Value, error) {
			configVals, err := call(configFn, callArgs)
			if err != nil {
				return nil, err
			}
			if len(configVals) >= 2 && !configVals[1].IsNil() {
				return []Value{NilValue(), configVals[1]}, nil
			}
			if len(configVals) == 0 || !configVals[0].IsTable() {
				return []Value{NilValue(), llmErrorValue("validation", "agent config function must return a table")}, nil
			}
			if !flowFn.IsFunction() {
				return runAgentConfig(configVals[0].Table())
			}
			agentConfigMu.RLock()
			merged := llmMergeTables(agentDefaults, configVals[0].Table())
			llmResolveModelAlias(merged, modelAliases)
			agentConfigMu.RUnlock()
			budget := llmBudgetFromOptions(merged)
			pushAgent(merged)
			pushBudget(budget)
			defer popBudget(budget)
			defer popAgent(merged)
			return call(flowFn, callArgs)
		}}
		llmAgentMetadataByFunction.Store(agentFn, metadata)
		return []Value{FunctionValue(agentFn)}, nil
	})

	return t
}
func llmJSONResponseFormatTable() *Table {
	format := NewTable()
	format.RawSetString("type", StringValue("json_object"))
	return format
}

func llmCloneTable(src *Table) *Table {
	out := NewTable()
	llmCopyTable(out, src, true)
	return out
}

func llmMergeTables(defaults, src *Table) *Table {
	out := NewTable()
	llmCopyTable(out, defaults, true)
	llmCopyTable(out, src, true)
	return out
}

func llmCopyTable(dst, src *Table, overwrite bool) {
	if dst == nil || src == nil {
		return
	}
	for _, key := range src.PairsKeysSnapshot() {
		val := src.RawGet(key)
		if val.IsNil() {
			continue
		}
		if !overwrite && !dst.RawGet(key).IsNil() {
			continue
		}
		dst.RawSet(key, val)
	}
}

func llmValidateModelAliases(aliases *Table) error {
	if aliases == nil {
		return nil
	}
	for _, key := range aliases.PairsKeysSnapshot() {
		if !key.IsString() {
			continue
		}
		name := key.Str()
		alias := aliases.RawGetString(name)
		if !alias.IsString() || alias.Str() == "" {
			continue
		}
		seen := map[string]int{name: 0}
		path := []string{name}
		for next := alias.Str(); next != ""; {
			if idx, ok := seen[next]; ok {
				cycle := append(append([]string{}, path[idx:]...), next)
				return fmt.Errorf("llm model alias cycle: %s", strings.Join(cycle, " -> "))
			}
			seen[next] = len(path)
			path = append(path, next)
			v := aliases.RawGetString(next)
			if !v.IsString() || v.Str() == "" {
				break
			}
			next = v.Str()
		}
	}
	return nil
}

func llmResolveModelAlias(opts, aliases *Table) {
	if opts == nil || aliases == nil {
		return
	}
	model := opts.RawGetString("model")
	if model.IsNil() {
		model = aliases.RawGetString("default")
	}
	if !model.IsString() || model.Str() == "" {
		return
	}
	alias := aliases.RawGetString(model.Str())
	switch {
	case alias.IsString() && alias.Str() != "":
		opts.RawSetString("model", alias)
	case alias.IsTable():
		providerModel := alias.Table().RawGetString("provider_model")
		if providerModel.IsNil() {
			providerModel = alias.Table().RawGetString("model")
		}
		if providerModel.IsString() && providerModel.Str() != "" {
			opts.RawSetString("model", providerModel)
		} else {
			opts.RawSetString("model", model)
		}
	default:
		opts.RawSetString("model", model)
	}
}

func llmResolveProviderForModel(opts, aliases *Table, defaultProvider LLMProvider, factory LLMProviderFactory) (LLMProvider, Value) {
	if defaultProvider != nil {
		return defaultProvider, NilValue()
	}
	if factory == nil {
		return nil, NilValue()
	}
	name, config := llmModelConfigTable(opts, aliases)
	if config == nil {
		return nil, NilValue()
	}
	cfg := LLMProviderConfig{
		Name:          name,
		Protocol:      llmTableString(config, "protocol"),
		BaseURL:       llmTableString(config, "base_url"),
		APIKey:        llmTableString(config, "api_key"),
		ProviderModel: llmTableString(config, "provider_model"),
		Provider:      llmTableString(config, "provider"),
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = llmTableString(config, "endpoint")
	}
	if cfg.ProviderModel == "" {
		cfg.ProviderModel = llmTableString(config, "model")
	}
	if cfg.Protocol == "" {
		return nil, NilValue()
	}
	p, err := factory(cfg)
	if err != nil {
		return nil, llmProviderErrorValue(err)
	}
	if p == nil {
		return nil, llmErrorValue("provider", "llm provider factory returned nil")
	}
	return p, NilValue()
}

func llmModelConfigTable(opts, aliases *Table) (string, *Table) {
	if aliases == nil {
		return "", nil
	}
	model := NilValue()
	if opts != nil {
		model = opts.RawGetString("model")
	}
	if model.IsNil() {
		model = aliases.RawGetString("default")
		if model.IsTable() {
			return "default", model.Table()
		}
	}
	if !model.IsString() || model.Str() == "" {
		return "", nil
	}
	name := model.Str()
	seen := map[string]bool{}
	for name != "" {
		if seen[name] {
			return "", nil
		}
		seen[name] = true
		alias := aliases.RawGetString(name)
		switch {
		case alias.IsTable():
			return name, alias.Table()
		case alias.IsString() && alias.Str() != "":
			name = alias.Str()
		default:
			return "", nil
		}
	}
	return "", nil
}

func llmTableString(tbl *Table, key string) string {
	if tbl == nil {
		return ""
	}
	v := tbl.RawGetString(key)
	if !v.IsString() {
		return ""
	}
	return v.Str()
}

func llmPlanTurn(src, opts *Table, provider LLMProvider, ctx context.Context, maxHostResult int64, trace func(LLMTraceEvent)) (LLMTurnResult, Value) {
	model := stdlibllm.SelectPlanModel(src.RawGetString("plan_model").Str(), opts.RawGetString("model").Str())
	messages := llmPlanMessages(opts.RawGetString("messages"))
	req := LLMTurnRequest{
		Model:          model,
		Messages:       llmMessagesFromValue(messages),
		MaxTokens:      toInt(src.RawGetString("plan_max_tokens")),
		Temperature:    llmOptionalFloatFromValue(src.RawGetString("plan_temperature")),
		TopP:           llmOptionalFloatFromValue(src.RawGetString("plan_top_p")),
		ResponseFormat: llmAnyFromValue(src.RawGetString("plan_response_format")),
		Stop:           llmStringSliceFromValue(src.RawGetString("plan_stop")),
		Metadata:       llmStringMapFromValue(src.RawGetString("metadata")),
	}
	trace(LLMTraceEvent{Type: "turn_start", Model: req.Model, MessageCount: len(req.Messages)})
	res, err := llmTurnWithOptionalStream(ctx, provider, req, trace, LLMTraceEvent{}, nil, NilValue())
	if err != nil {
		trace(LLMTraceEvent{Type: "turn_error", Model: req.Model, ErrorKind: ClassifyLLMProviderError(err), Message: err.Error()})
		return LLMTurnResult{}, llmProviderErrorValue(err)
	}
	trace(LLMTraceEvent{Type: "turn_end", Model: req.Model, Status: llmResultStatus(res), MessageCount: len(req.Messages), Usage: res.Usage})
	if err := CheckHostResultBytes(maxHostResult, llmResultValue(res)); err != nil {
		return LLMTurnResult{}, llmErrorValue("internal", err.Error())
	}
	return res, NilValue()
}

func llmPlanMessages(messages Value) Value {
	t := NewAppendArrayTable(2)
	t.RawSet(IntValue(1), TableValue(llmMessageTable("system", stdlibllm.PlanPrompt())))
	if messages.IsTable() {
		for _, msg := range llmMessageValuesFromTable(messages.Table()) {
			t.RawSet(IntValue(int64(t.Length()+1)), msg)
		}
	}
	return TableValue(t)
}

func llmInjectPlan(opts *Table, plan string) {
	text, ok := stdlibllm.ExecutionPlanMessage(plan)
	if !ok {
		return
	}
	messages := opts.RawGetString("messages")
	if !messages.IsTable() {
		return
	}
	merged := NewAppendArrayTable(messages.Table().Length() + 1)
	merged.RawSet(IntValue(1), TableValue(llmMessageTable("system", text)))
	for _, msg := range llmMessageValuesFromTable(messages.Table()) {
		merged.RawSet(IntValue(int64(merged.Length()+1)), msg)
	}
	opts.RawSetString("messages", TableValue(merged))
}

func llmReflectResult(src, result *Table, provider LLMProvider, ctx context.Context, maxHostResult int64, trace func(LLMTraceEvent)) Value {
	if result.RawGetString("status").Str() != stdlibllm.ReactStatusDone {
		return NilValue()
	}
	maxIters := stdlibllm.ReflectIterations(toInt(src.RawGetString("max_iters")))
	model := stdlibllm.SelectReflectModel(src.RawGetString("reflect_model").Str(), src.RawGetString("model").Str())
	reflections := NewAppendArrayTable(int(maxIters))
	text := result.RawGetString("text").Str()
	for i := int64(0); i < maxIters; i++ {
		messages := NewAppendArrayTable(2)
		prompt := stdlibllm.ReflectPrompt(src.RawGetString("reflect_prompt").Str())
		messages.RawSet(IntValue(1), TableValue(llmMessageTable("system", prompt)))
		messages.RawSet(IntValue(2), TableValue(llmMessageTable("user", text)))
		req := LLMTurnRequest{
			Model:          model,
			Messages:       llmMessagesFromValue(TableValue(messages)),
			MaxTokens:      toInt(src.RawGetString("reflect_max_tokens")),
			Temperature:    llmOptionalFloatFromValue(src.RawGetString("reflect_temperature")),
			TopP:           llmOptionalFloatFromValue(src.RawGetString("reflect_top_p")),
			ResponseFormat: llmAnyFromValue(src.RawGetString("reflect_response_format")),
			Stop:           llmStringSliceFromValue(src.RawGetString("reflect_stop")),
			Metadata:       llmStringMapFromValue(src.RawGetString("metadata")),
		}
		trace(LLMTraceEvent{Type: "turn_start", Model: req.Model, MessageCount: len(req.Messages)})
		res, err := llmTurnWithOptionalStream(ctx, provider, req, trace, LLMTraceEvent{}, nil, NilValue())
		if err != nil {
			trace(LLMTraceEvent{Type: "turn_error", Model: req.Model, ErrorKind: ClassifyLLMProviderError(err), Message: err.Error()})
			return llmProviderErrorValue(err)
		}
		trace(LLMTraceEvent{Type: "turn_end", Model: req.Model, Status: llmResultStatus(res), MessageCount: len(req.Messages), Usage: res.Usage})
		turn := llmResultValue(res)
		if err := CheckHostResultBytes(maxHostResult, turn); err != nil {
			return llmErrorValue("internal", err.Error())
		}
		reflections.RawSet(IntValue(int64(reflections.Length()+1)), turn)
		if res.Text != "" {
			text = res.Text
			result.RawSetString("text", StringValue(text))
		}
	}
	result.RawSetString("reflection", TableValue(reflections))
	return NilValue()
}
