package bind

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	stdlibllm "github.com/never-labs/leia/internal/stdlib/lib/llm"
)

type llmLibBuilder struct {
	t *Table

	call                   ScriptFunctionCaller
	currentProvider        func() LLMProvider
	currentProviderFactory func() LLMProviderFactory
	hostLimit              func() int64
	currentContext         func() context.Context
	trace                  func(LLMTraceEvent)

	agentConfigMu sync.RWMutex
	agentDefaults *Table
	modelAliases  *Table

	agentContextMu sync.Mutex
	ambientAgents  []*Table

	budgetMu       sync.Mutex
	ambientBudgets []*llmBudget
}

func newLLMLibBuilder(call ScriptFunctionCaller, provider func() LLMProvider, providerFactory func() LLMProviderFactory, maxHostResult func() int64, ctx func() context.Context, traces []LLMTraceSink) *llmLibBuilder {
	b := &llmLibBuilder{
		t:             NewTable(),
		call:          call,
		agentDefaults: NewTable(),
		modelAliases:  NewTable(),
	}
	b.hostLimit = func() int64 {
		if maxHostResult == nil {
			return 0
		}
		return maxHostResult()
	}
	b.currentProvider = func() LLMProvider {
		if provider == nil {
			return nil
		}
		return provider()
	}
	b.currentProviderFactory = func() LLMProviderFactory {
		if providerFactory == nil {
			return nil
		}
		return providerFactory()
	}
	b.currentContext = func() context.Context {
		if ctx == nil || ctx() == nil {
			return context.Background()
		}
		return ctx()
	}
	b.trace = func(event LLMTraceEvent) {
		if len(traces) == 0 || traces[0] == nil {
			return
		}
		traces[0](event)
	}
	return b
}

func (b *llmLibBuilder) set(name string, fn func([]Value) ([]Value, error)) {
	setLLMFunction(b.t, "llm", name, fn)
}

func (b *llmLibBuilder) register() {
	registerLLMMessageConstructors(b.t, "llm")
	registerLLMMemoryHelpers(b.t)
	registerLLMReplayHelpers(b.t)
	b.registerMessageHelpers()
	b.registerToolHelpers()
	b.registerToolOutcomeHelpers()
	b.registerSchemaHelpers()
	b.registerModelIOEnvelopeHelpers()
	b.registerToolCheckHelper()
	b.registerAgentStateCheckpointHelpers()
	b.registerPolicyHelpers()
	b.registerApprovalHelpers()
	b.registerBudgetOutcomeHelpers()
	b.registerTraceHelpers()
	b.registerRuntimeHelpers()
	b.registerValidationHelpers()
	registerLLMWorkflowHelpers(b.t, b.call)
	b.registerAgentConfigHelpers()
}

func (b *llmLibBuilder) pushBudget(budget *llmBudget) {
	b.budgetMu.Lock()
	b.ambientBudgets = append(b.ambientBudgets, budget)
	b.budgetMu.Unlock()
}

func (b *llmLibBuilder) popBudget(budget *llmBudget) {
	b.budgetMu.Lock()
	defer b.budgetMu.Unlock()
	for i := len(b.ambientBudgets) - 1; i >= 0; i-- {
		if b.ambientBudgets[i] == budget {
			copy(b.ambientBudgets[i:], b.ambientBudgets[i+1:])
			b.ambientBudgets[len(b.ambientBudgets)-1] = nil
			b.ambientBudgets = b.ambientBudgets[:len(b.ambientBudgets)-1]
			return
		}
	}
}

func (b *llmLibBuilder) currentBudgets() llmBudgetGroup {
	b.budgetMu.Lock()
	defer b.budgetMu.Unlock()
	out := make(llmBudgetGroup, len(b.ambientBudgets))
	copy(out, b.ambientBudgets)
	return out
}

func (b *llmLibBuilder) pushAgent(config *Table) {
	if config == nil {
		return
	}
	b.agentContextMu.Lock()
	b.ambientAgents = append(b.ambientAgents, config)
	b.agentContextMu.Unlock()
}

func (b *llmLibBuilder) popAgent(config *Table) {
	if config == nil {
		return
	}
	b.agentContextMu.Lock()
	defer b.agentContextMu.Unlock()
	for i := len(b.ambientAgents) - 1; i >= 0; i-- {
		if b.ambientAgents[i] == config {
			copy(b.ambientAgents[i:], b.ambientAgents[i+1:])
			b.ambientAgents[len(b.ambientAgents)-1] = nil
			b.ambientAgents = b.ambientAgents[:len(b.ambientAgents)-1]
			return
		}
	}
}

func (b *llmLibBuilder) currentAgentConfig() *Table {
	b.agentContextMu.Lock()
	defer b.agentContextMu.Unlock()
	if len(b.ambientAgents) == 0 {
		return nil
	}
	out := NewTable()
	for _, config := range b.ambientAgents {
		llmCopyTable(out, config, true)
	}
	return out
}

func (b *llmLibBuilder) registerMessageHelpers() {
	b.set("assistantCall", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.assistantCall' (table expected)")
		}
		return []Value{TableValue(llmAssistantCallMessageTable(args[0]))}, nil
	})
	b.set("assistant_call", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.assistant_call' (table expected)")
		}
		return []Value{TableValue(llmAssistantCallMessageTable(args[0]))}, nil
	})
	b.set("toolResult", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'llm.toolResult'")
		}
		return []Value{TableValue(llmToolResultMessageTable(args[0], args[1]))}, nil
	})
	b.set("tool_result", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'llm.tool_result'")
		}
		return []Value{TableValue(llmToolResultMessageTable(args[0], args[1]))}, nil
	})
	b.set("toolError", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'llm.toolError'")
		}
		return []Value{TableValue(llmToolErrorMessageTable(args[0], args[1].Str()))}, nil
	})
	b.set("tool_error", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'llm.tool_error'")
		}
		return []Value{TableValue(llmToolErrorMessageTable(args[0], args[1].Str()))}, nil
	})
}

func (b *llmLibBuilder) registerToolHelpers() {
	b.set("tool", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsFunction() {
			return nil, fmt.Errorf("bad argument to 'llm.tool' (name, function expected)")
		}
		opts := NilValue()
		if len(args) >= 3 {
			opts = args[2]
		}
		return []Value{llmNewToolValue(args[0].Str(), args[1], opts)}, nil
	})

	agentAsTool := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsFunction() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.toolof' (agent function expected)")
		}
		if len(args) >= 2 && !args[1].IsNil() && !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument #2 to 'llm.toolof' (options table expected)")
		}
		if b.call == nil {
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
		name = llmAgentToolName(agent, llmAgentMetadata{Name: name}, "agent")
		wrapper := llmAgentToolWrapper(b.call, agent, meta, name)
		tool := llmNewToolValue(name, wrapper, opts)
		if tt := tool.Table(); tt != nil {
			tt.RawSetString("__llm_agent_tool", BoolValue(true))
			tt.RawSetString("trace_contract", StringValue("agent_tool.v1"))
			if !tt.RawGetString("params").IsTable() && len(meta.Params) > 0 {
				tt.RawSetString("params", llmStringArrayValue(meta.Params))
			}
			if tt.RawGetString("output").IsNil() && !meta.Output.IsNil() {
				tt.RawSetString("output", meta.Output)
			}
		}
		return []Value{tool}, nil
	}
	b.set("toolof", agentAsTool)
	b.set("agent_as_tool", agentAsTool)
	b.set("handoff", agentAsTool)
	b.set("delegate", agentAsTool)

	b.set("tool_caps", func(args []Value) ([]Value, error) {
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
	b.set("tool_schema", toolSchema)
	b.set("toolSchema", toolSchema)

	toolInfo := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.tool_info' (tool or tools table expected)")
		}
		return []Value{llmToolInfoValue(args[0])}, nil
	}
	b.set("tool_info", toolInfo)
	b.set("toolInfo", toolInfo)
	b.set("tool_descriptor", toolInfo)
	b.set("toolDescriptor", toolInfo)
}

func (b *llmLibBuilder) registerToolCheckHelper() {
	b.set("check_tools", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() || !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument to 'llm.check_tools' (tools, caps expected)")
		}
		if err := llmCheckToolCaps(args[0].Table(), args[1].Table()); !err.IsNil() {
			return []Value{NilValue(), err}, nil
		}
		return []Value{BoolValue(true), NilValue()}, nil
	})
	validateTools := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.validate_tools' (tools table expected)")
		}
		if err := llmValidateToolContracts(args[0].Table()); !err.IsNil() {
			return []Value{NilValue(), err}, nil
		}
		return []Value{BoolValue(true), NilValue()}, nil
	}
	b.set("validate_tools", validateTools)
	b.set("validateTools", validateTools)
}

func llmNewToolValue(name string, fn Value, opts Value) Value {
	desc := ""
	var params Value
	var requires Value
	var schema Value
	var output Value
	var result Value
	var errorSpec Value
	var replayKey Value
	var callerRole Value
	var executorRole Value
	var effect Value
	var approvalPolicy Value
	var providerWireFormat Value
	var liveNetwork Value
	var secretParametersAllowed Value
	if opts.IsTable() {
		optTable := opts.Table()
		desc = optTable.RawGetString("description").Str()
		params = optTable.RawGetString("params")
		requires = optTable.RawGetString("requires")
		if requires.IsNil() {
			requires = optTable.RawGetString("capabilities")
		}
		schema = optTable.RawGetString("schema")
		output = optTable.RawGetString("output")
		result = optTable.RawGetString("result")
		errorSpec = optTable.RawGetString("error")
		replayKey = optTable.RawGetString("replay_key")
		callerRole = optTable.RawGetString("caller_role")
		executorRole = optTable.RawGetString("executor_role")
		effect = optTable.RawGetString("effect")
		approvalPolicy = optTable.RawGetString("approval_policy")
		providerWireFormat = optTable.RawGetString("provider_wire_format")
		liveNetwork = optTable.RawGetString("live_network")
		secretParametersAllowed = optTable.RawGetString("secret_parameters_allowed")
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
		tool.RawSetString("capabilities", requires)
	}
	if !schema.IsNil() {
		tool.RawSetString("schema", schema)
	}
	if !output.IsNil() {
		tool.RawSetString("output", output)
	}
	if !result.IsNil() {
		tool.RawSetString("result", result)
		if output.IsNil() {
			tool.RawSetString("output", result)
		}
	}
	if !errorSpec.IsNil() {
		tool.RawSetString("error", errorSpec)
	}
	if !replayKey.IsNil() {
		tool.RawSetString("replay_key", replayKey)
	}
	if !callerRole.IsNil() {
		tool.RawSetString("caller_role", callerRole)
	}
	if !executorRole.IsNil() {
		tool.RawSetString("executor_role", executorRole)
	}
	if !effect.IsNil() {
		tool.RawSetString("effect", effect)
	}
	if !approvalPolicy.IsNil() {
		tool.RawSetString("approval_policy", approvalPolicy)
	}
	if !providerWireFormat.IsNil() {
		tool.RawSetString("provider_wire_format", providerWireFormat)
	}
	if !liveNetwork.IsNil() {
		tool.RawSetString("live_network", liveNetwork)
	}
	if !secretParametersAllowed.IsNil() {
		tool.RawSetString("secret_parameters_allowed", secretParametersAllowed)
	}
	return TableValue(tool)
}

func (b *llmLibBuilder) registerSchemaHelpers() {
	schemaValue := func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'llm.schema' (schema spec expected)")
		}
		return []Value{llmSchemaValue(args[0])}, nil
	}
	b.set("schema", schemaValue)
	b.set("schema_info", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'llm.schema_info' (schema spec expected)")
		}
		return []Value{llmSchemaInfoValue(args[0])}, nil
	})
	b.set("schemaInfo", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'llm.schemaInfo' (schema spec expected)")
		}
		return []Value{llmSchemaInfoValue(args[0])}, nil
	})
	outputSchema := func(args []Value) ([]Value, error) {
		v, err := llmOutputSchemaValue(args)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}
	b.set("output_schema", outputSchema)
	b.set("outputSchema", outputSchema)
	reportArtifactContract := func(args []Value) ([]Value, error) {
		if len(args) >= 1 && !args[0].IsNil() && !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.report_artifact_contract' (options table expected)")
		}
		var opts *Table
		if len(args) >= 1 && args[0].IsTable() {
			opts = args[0].Table()
		}
		return []Value{llmReportArtifactContractValue(opts)}, nil
	}
	b.set("report_artifact_contract", reportArtifactContract)
	b.set("reportArtifactContract", reportArtifactContract)
	packageContract := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.provider_free_package_contract' (package contract table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.provider_free_package_contract' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmProviderFreePackageContractValue(args[0].Table(), opts))}, nil
	}
	b.set("provider_free_package_contract", packageContract)
	b.set("providerFreePackageContract", packageContract)
	b.set("package_contract", packageContract)
	b.set("packageContract", packageContract)
}

func (b *llmLibBuilder) registerValidationHelpers() {
	validateOutput := func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'llm.validate_output' (value, schema expected)")
		}
		value := args[0]
		schema := args[1]
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
			return []Value{BoolValue(false), StringValue("schema must be a table example or JSON schema")}, nil
		}
		if !value.IsTable() {
			return []Value{BoolValue(false), StringValue("value must decode to a table")}, nil
		}
		if llmLooksLikeJSONSchema(schema) {
			if msg := llmValidateStructuredOutputSchema(llmSchemaValue(schema), value, ""); msg != "" {
				return []Value{BoolValue(false), StringValue(msg)}, nil
			}
		} else if msg := llmValidateStructuredOutputShape(schema.Table(), value.Table()); msg != "" {
			return []Value{BoolValue(false), StringValue(msg)}, nil
		}
		return []Value{BoolValue(true), StringValue("")}, nil
	}
	b.set("validate_output", validateOutput)
	b.set("validateOutput", validateOutput)
	validatePackageContract := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.validate_package_contract' (package contract table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.validate_package_contract' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmValidatePackageContractValue(args[0].Table(), opts))}, nil
	}
	b.set("validate_package_contract", validatePackageContract)
	b.set("validatePackageContract", validatePackageContract)
}

func (b *llmLibBuilder) registerRuntimeHelpers() {
	b.set("turn", b.llmTurn)

	b.set("dispatch", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() || !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument to 'llm.dispatch' (call, tools expected)")
		}
		if b.call == nil {
			return nil, fmt.Errorf("llm.dispatch requires a function caller")
		}
		return llmDispatch(b.call, args[0].Table(), args[1].Table())
	})

	b.set("react", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.react' (table expected)")
		}
		if b.call == nil {
			return nil, fmt.Errorf("llm.react requires a function caller")
		}
		p := b.currentProvider()
		if p == nil {
			b.trace(LLMTraceEvent{Type: "react_error", ErrorKind: "provider", Message: "llm provider not configured"})
			return []Value{NilValue(), llmErrorValue("provider", "llm provider not configured")}, nil
		}
		result, err := llmReact(args[0].Table(), p, b.call, b.currentContext(), b.hostLimit(), b.trace, b.currentBudgets())
		if err != nil {
			return nil, err
		}
		return result, nil
	})

	b.set("with_budget", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
			return nil, fmt.Errorf("bad argument to 'llm.with_budget' (budget table, function expected)")
		}
		if b.call == nil {
			return nil, fmt.Errorf("llm.with_budget requires a function caller")
		}
		budget := llmBudgetFromConfig(args[0].Table())
		b.pushBudget(budget)
		defer b.popBudget(budget)
		return b.call(args[1], nil)
	})
}

func (b *llmLibBuilder) llmTurn(args []Value) ([]Value, error) {
	if len(args) < 1 || !args[0].IsTable() {
		return nil, fmt.Errorf("bad argument #1 to 'llm.turn' (table expected)")
	}
	b.agentConfigMu.RLock()
	var opts *Table
	if ambient := b.currentAgentConfig(); ambient != nil {
		opts = llmMergeTables(ambient, args[0].Table())
	} else {
		opts = llmCloneTable(args[0].Table())
	}
	if tv := opts.RawGetString("tools"); llmToolsListHasAgents(tv) {
		opts.RawSetString("tools", llmNormalizeToolsValue(b.call, tv))
	}
	onStream := opts.RawGetString("on_stream")
	if onStream.IsNil() {
		onStream = opts.RawGetString("onStream")
	}
	replayPlan := llmTurnReplayPlanFromOptions(opts)
	var p LLMProvider
	if !replayPlan.enabled {
		var providerErr Value
		p, providerErr = llmResolveProviderForModel(opts, b.modelAliases, b.currentProvider(), b.currentProviderFactory())
		if !providerErr.IsNil() {
			b.agentConfigMu.RUnlock()
			b.trace(LLMTraceEvent{Type: "turn_error", ErrorKind: "provider", Message: providerErr.Table().RawGetString("message").Str()})
			return []Value{NilValue(), providerErr}, nil
		}
	}
	llmResolveModelAlias(opts, b.modelAliases)
	if opts.RawGetString("messages").IsNil() {
		memoryContext := opts.RawGetString("context")
		memoryEvidence := opts.RawGetString("evidence")
		normalized, err := llmLoopOptions(opts, stdlibllm.DefaultSimpleMaxSteps)
		if err != nil {
			b.agentConfigMu.RUnlock()
			b.trace(LLMTraceEvent{Type: "turn_error", ErrorKind: "validation", Message: err.Error()})
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
			b.agentConfigMu.RUnlock()
			b.trace(LLMTraceEvent{Type: "turn_error", ErrorKind: "validation", Message: "llm.turn on_stream must be a function"})
			return []Value{NilValue(), llmErrorValue("validation", "llm.turn on_stream must be a function")}, nil
		}
		opts.RawSetString("stream", BoolValue(true))
	}
	b.agentConfigMu.RUnlock()
	req, err := llmRequestFromTable(opts)
	if err != nil {
		b.trace(LLMTraceEvent{Type: "turn_error", ErrorKind: "validation", Message: err.Error()})
		return []Value{NilValue(), llmErrorValue("validation", err.Error())}, nil
	}
	if replayPlan.enabled {
		if errVal := llmTurnReplayValidate(replayPlan, req); !errVal.IsNil() {
			b.trace(LLMTraceEvent{Type: "turn_replay_error", Model: req.Model, ErrorKind: llmErrorKind(errVal), Message: errVal.Table().RawGetString("message").Str(), ReplayKey: replayPlan.replayKey, RequestHash: llmTurnRequestHash(req), ReplayMode: replayPlan.mode, ProviderFree: true})
			return []Value{NilValue(), errVal}, nil
		}
		out := llmTurnReplayResult(replayPlan, req)
		b.trace(LLMTraceEvent{Type: "turn_replay", Model: req.Model, Status: out.Table().RawGetString("status").Str(), MessageCount: len(req.Messages), ToolCount: len(req.Tools), ReplayKey: replayPlan.replayKey, RequestHash: llmTurnRequestHash(req), ResponseHash: out.Table().RawGetString("replay").Table().RawGetString("response_hash").Str(), ReplayMode: replayPlan.mode, ProviderFree: true})
		return []Value{out, NilValue()}, nil
	}
	if p == nil {
		b.trace(LLMTraceEvent{Type: "turn_error", ErrorKind: "provider", Message: "llm provider not configured"})
		return []Value{NilValue(), llmErrorValue("provider", "llm provider not configured")}, nil
	}
	budgets := b.currentBudgets().with(llmBudgetFromOptions(opts))
	if err := budgets.beforeTurn(); !err.IsNil() {
		b.trace(LLMTraceEvent{Type: "turn_error", Model: req.Model, ErrorKind: llmErrorKind(err), Message: err.Table().RawGetString("message").Str()})
		return []Value{NilValue(), err}, nil
	}
	b.trace(LLMTraceEvent{Type: "turn_start", Model: req.Model, MessageCount: len(req.Messages), ToolCount: len(req.Tools)})
	res, err := llmTurnWithOptionalStream(b.currentContext(), p, req, b.trace, LLMTraceEvent{}, b.call, onStream)
	if err != nil {
		b.trace(LLMTraceEvent{Type: "turn_error", Model: req.Model, ErrorKind: ClassifyLLMProviderError(err), Message: err.Error()})
		return []Value{NilValue(), llmProviderErrorValue(err)}, nil
	}
	b.trace(LLMTraceEvent{Type: "turn_end", Model: req.Model, Status: llmResultStatus(res), MessageCount: len(req.Messages), ToolCount: len(req.Tools), Usage: res.Usage})
	out := llmResultValue(res)
	if err := CheckHostResultBytes(b.hostLimit(), out); err != nil {
		return nil, err
	}
	budgets.chargeTurn(res.Usage)
	return []Value{out, NilValue()}, nil
}

func (b *llmLibBuilder) registerAgentConfigHelpers() {
	b.set("agent_defaults", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.agent_defaults' (table expected)")
		}
		b.agentConfigMu.Lock()
		b.agentDefaults = llmCloneTable(args[0].Table())
		b.agentConfigMu.Unlock()
		return []Value{BoolValue(true), NilValue()}, nil
	})

	registerModels := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.register_models' (table expected)")
		}
		if err := llmValidateModelAliases(args[0].Table()); err != nil {
			return nil, err
		}
		b.agentConfigMu.Lock()
		b.modelAliases = llmCloneTable(args[0].Table())
		b.agentConfigMu.Unlock()
		return []Value{BoolValue(true), NilValue()}, nil
	}
	b.set("models", registerModels)
	b.set("register_models", registerModels)

	runAgentConfig := b.runAgentConfig
	b.set("run_agent", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.run_agent' (table expected)")
		}
		return runAgentConfig(args[0].Table())
	})
	registerLLMSectionHelpers(b.t, runAgentConfig)
	b.set("agent", b.newAgent)
}

func (b *llmLibBuilder) runAgentConfig(src *Table) ([]Value, error) {
	if b.call == nil {
		return nil, fmt.Errorf("llm.run_agent requires a function caller")
	}
	b.agentConfigMu.RLock()
	merged := llmMergeTables(b.agentDefaults, src)
	p, providerErr := llmResolveProviderForModel(merged, b.modelAliases, b.currentProvider(), b.currentProviderFactory())
	if !providerErr.IsNil() {
		b.agentConfigMu.RUnlock()
		b.trace(LLMTraceEvent{Type: "react_error", ErrorKind: "provider", Message: providerErr.Table().RawGetString("message").Str()})
		return []Value{NilValue(), providerErr}, nil
	}
	llmResolveModelAlias(merged, b.modelAliases)
	if p == nil {
		b.agentConfigMu.RUnlock()
		b.trace(LLMTraceEvent{Type: "react_error", ErrorKind: "provider", Message: "llm provider not configured"})
		return []Value{NilValue(), llmErrorValue("provider", "llm provider not configured")}, nil
	}
	memoryContext := merged.RawGetString("context")
	memoryEvidence := merged.RawGetString("evidence")
	opts, err := llmLoopOptions(merged, 0)
	b.agentConfigMu.RUnlock()
	if err == nil {
		if !memoryContext.IsNil() {
			opts.RawSetString("context", memoryContext)
		}
		if !memoryEvidence.IsNil() {
			opts.RawSetString("evidence", memoryEvidence)
		}
		if tv := opts.RawGetString("tools"); llmToolsListHasAgents(tv) {
			opts.RawSetString("tools", llmNormalizeToolsValue(b.call, tv))
		}
		llmApplyMemoryContext(opts)
	}
	if err != nil {
		b.trace(LLMTraceEvent{Type: "react_error", ErrorKind: "validation", Message: err.Error()})
		return []Value{NilValue(), llmErrorValue("validation", err.Error())}, nil
	}
	result, err := llmReact(opts, p, b.call, b.currentContext(), b.hostLimit(), b.trace, b.currentBudgets())
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (b *llmLibBuilder) newAgent(args []Value) ([]Value, error) {
	if len(args) < 2 || !args[0].IsString() || !args[1].IsFunction() {
		return nil, fmt.Errorf("bad argument to 'llm.agent' (name, config function expected)")
	}
	if len(args) >= 3 && !args[2].IsNil() && !args[2].IsFunction() {
		return nil, fmt.Errorf("bad argument #3 to 'llm.agent' (flow function expected)")
	}
	if b.call == nil {
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
		configVals, err := b.call(configFn, callArgs)
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
			return b.runAgentConfig(configVals[0].Table())
		}
		b.agentConfigMu.RLock()
		merged := llmMergeTables(b.agentDefaults, configVals[0].Table())
		llmResolveModelAlias(merged, b.modelAliases)
		b.agentConfigMu.RUnlock()
		budget := llmBudgetFromOptions(merged)
		b.pushAgent(merged)
		b.pushBudget(budget)
		defer b.popBudget(budget)
		defer b.popAgent(merged)
		return b.call(flowFn, callArgs)
	}}
	llmAgentMetadataByFunction.Store(agentFn, metadata)
	return []Value{FunctionValue(agentFn)}, nil
}
