package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

type llmAgentMetadata struct {
	Name        string
	Params      []string
	Output      Value
	Description string
}

var llmAgentMetadataByFunction sync.Map // map[*GoFunction]llmAgentMetadata

// LLMProvider is the host boundary behind llm.turn. Implementations may call a
// remote API, a local model, or a test double; the runtime only sees this small
// protocol shape.
type LLMProvider interface {
	Turn(context.Context, LLMTurnRequest) (LLMTurnResult, error)
}

// LLMProviderConfig is the runtime shape of one models {} provider entry.
// The runtime records this data, but construction is delegated to the host
// package to avoid coupling the interpreter to concrete HTTP providers.
type LLMProviderConfig struct {
	Name          string
	Protocol      string
	BaseURL       string
	APIKey        string
	ProviderModel string
	Provider      string
}

type LLMProviderFactory func(LLMProviderConfig) (LLMProvider, error)

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
	Model          string
	Messages       []LLMMessage
	Tools          []LLMTool
	ForceTool      string
	MaxTokens      int64
	Temperature    *float64
	TopP           *float64
	ResponseFormat any
	Stream         bool
	Stop           []string
	Metadata       map[string]string
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
	Token        string
	ErrorKind    string
	Message      string
	Step         int64
	Attempt      int64
	MessageCount int
	ToolCount    int
	Store        bool
	Usage        LLMTurnUsage
}

type LLMTraceSink func(LLMTraceEvent)

const (
	LLMProviderErrorNetwork   = "network"
	LLMProviderErrorAuth      = "auth"
	LLMProviderErrorRateLimit = "rate_limit"
	LLMProviderErrorRequest   = "request"
	LLMProviderErrorProvider  = "provider"
)

type llmProviderErrorKind interface {
	LLMProviderErrorKind() string
}

// ClassifyLLMProviderError maps provider failures into a stable diagnostic
// category without inspecting prompts, messages, or token values.
func ClassifyLLMProviderError(err error) string {
	if err == nil {
		return ""
	}
	var typed llmProviderErrorKind
	if errors.As(err, &typed) {
		if kind := typed.LLMProviderErrorKind(); kind != "" {
			return kind
		}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return LLMProviderErrorNetwork
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return LLMProviderErrorNetwork
	}
	return LLMProviderErrorProvider
}

// BuildLLMLib creates the "llm" standard library table. It is the first-stage
// runtime substrate for the agent layer: future syntax can compile to these
// functions without changing provider or tool-dispatch semantics.
func BuildLLMLib(call FunctionCaller, provider func() LLMProvider, providerFactory func() LLMProviderFactory, maxHostResult func() int64, ctx func() context.Context, traces ...LLMTraceSink) *Table {
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
		if opts.IsTable() {
			optTable := opts.Table()
			desc = optTable.RawGetString("description").Str()
			params = optTable.RawGetString("params")
			requires = optTable.RawGetString("requires")
			schema = optTable.RawGetString("schema")
			if schema.IsNil() {
				schema = optTable.RawGetString("output")
			}
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
		return TableValue(tool)
	}

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
			results, err := call(agent, callArgs)
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
			if !tt.RawGetString("params").IsTable() && len(meta.Params) > 0 {
				tt.RawSetString("params", llmStringArrayValue(meta.Params))
			}
			if tt.RawGetString("schema").IsNil() && !meta.Output.IsNil() {
				tt.RawSetString("schema", meta.Output)
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
			normalized, err := llmLoopOptions(opts, 1)
			if err != nil {
				agentConfigMu.RUnlock()
				trace(LLMTraceEvent{Type: "turn_error", ErrorKind: "validation", Message: err.Error()})
				return []Value{NilValue(), llmErrorValue("validation", err.Error())}, nil
			}
			opts = normalized
		} else if opts.RawGetString("response_format").IsNil() && opts.RawGetString("output").IsTable() {
			// When an ambient agent (typically a flow agent) declares output:
			// forward a json_object response_format to the provider so the
			// model knows the requested shape. Auto-validation is left to the
			// flow body via llm.validate_output.
			opts.RawSetString("response_format", TableValue(llmJSONResponseFormatTable()))
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
		res, err := p.Turn(currentContext(), req)
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
			value = jsonGoToGScript(raw)
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
		opts, err := llmLoopOptions(merged, 0)
		agentConfigMu.RUnlock()
		if err == nil {
			if tv := opts.RawGetString("tools"); llmToolsListHasAgents(tv) {
				opts.RawSetString("tools", llmNormalizeToolsValue(call, tv))
			}
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

// BuildLLMHistoryLib creates the "history" stdlib table with helpers for
// searching and mutating conversation history values produced by agents and
// llm.react. The block-form `messages { }` constructor remains the preferred
// way to assemble *initial* history; these helpers cover the dynamic case where
// the user reads/appends to a history list at runtime.
func BuildLLMHistoryLib() *Table {
	t := NewTable()
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{Name: "history." + name, Fn: fn}))
	}

	matches := func(msg *Table, opts *Table) bool {
		if msg == nil {
			return false
		}
		for _, key := range opts.PairsKeysSnapshot() {
			if !key.IsString() {
				continue
			}
			k := key.Str()
			want := opts.RawGet(key)
			switch k {
			case "role":
				if msg.RawGetString("role").Str() != want.Str() {
					return false
				}
			case "tool":
				name := ""
				if call := msg.RawGetString("tool_call"); call.IsTable() {
					name = call.Table().RawGetString("tool").Str()
				}
				if name == "" && msg.RawGetString("role").Str() == "tool" {
					// tool result messages don't carry the tool name directly;
					// match via tool_use_id prefix or skip.
					return false
				}
				if name != want.Str() {
					return false
				}
			case "tool_use_id", "id":
				if msg.RawGetString("tool_use_id").Str() != want.Str() {
					return false
				}
			case "has_error":
				if want.Truthy() != (msg.RawGetString("error").Str() != "") {
					return false
				}
			default:
				if msg.RawGetString(k).Str() != want.Str() {
					return false
				}
			}
		}
		return true
	}

	iter := func(historyVal, optsVal Value, yield func(idx int, msg Value) bool) {
		if !historyVal.IsTable() {
			return
		}
		ht := historyVal.Table()
		var opts *Table
		if optsVal.IsTable() {
			opts = optsVal.Table()
		} else {
			opts = NewTable()
		}
		for i := 1; i <= ht.Length(); i++ {
			m := ht.RawGet(IntValue(int64(i)))
			if !m.IsTable() {
				continue
			}
			if !matches(m.Table(), opts) {
				continue
			}
			if !yield(i, m) {
				return
			}
		}
	}

	set("find", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'history.find' (history expected)")
		}
		optsVal := NilValue()
		if len(args) >= 2 {
			optsVal = args[1]
		}
		var foundMsg Value = NilValue()
		foundIdx := int64(-1)
		iter(args[0], optsVal, func(idx int, msg Value) bool {
			foundMsg = msg
			foundIdx = int64(idx)
			return false
		})
		return []Value{foundMsg, IntValue(foundIdx)}, nil
	})

	set("find_all", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'history.find_all' (history expected)")
		}
		optsVal := NilValue()
		if len(args) >= 2 {
			optsVal = args[1]
		}
		out := NewTable()
		iter(args[0], optsVal, func(idx int, msg Value) bool {
			out.RawSet(IntValue(int64(out.Length()+1)), msg)
			return true
		})
		return []Value{TableValue(out)}, nil
	})

	set("last", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'history.last' (history expected)")
		}
		optsVal := NilValue()
		if len(args) >= 2 {
			optsVal = args[1]
		}
		var foundMsg Value = NilValue()
		foundIdx := int64(-1)
		iter(args[0], optsVal, func(idx int, msg Value) bool {
			foundMsg = msg
			foundIdx = int64(idx)
			return true
		})
		return []Value{foundMsg, IntValue(foundIdx)}, nil
	})

	set("append", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument to 'history.append' (history, message expected)")
		}
		h := args[0].Table()
		h.RawSet(IntValue(int64(h.Length()+1)), args[1])
		return []Value{args[0]}, nil
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
		return llmReact(opts, p, call, currentContext(), hostLimit(), trace, nil, &llmHITL{
			approveWhen: args[0].Table().RawGetString("approve_when"),
			store:       args[0].Table().RawGetString("store"),
			trace:       trace,
			snapshots:   snapshots,
			snapshotsMu: &snapshotsMu,
		})
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
		return llmReact(opts, p, call, currentContext(), hostLimit(), trace, nil)
	})

	set("plan_execute", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'loop.plan_execute' (table expected)")
		}
		if call == nil {
			return nil, fmt.Errorf("loop.plan_execute requires a function caller")
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
		planResult, planErr := llmPlanTurn(args[0].Table(), opts, p, currentContext(), hostLimit(), trace)
		if !planErr.IsNil() {
			return []Value{NilValue(), planErr}, nil
		}
		llmInjectPlan(opts, planResult.Text)
		if execModel := args[0].Table().RawGetString("exec_model"); !execModel.IsNil() {
			opts.RawSetString("model", execModel)
		}
		result, err := llmReact(opts, p, call, currentContext(), hostLimit(), trace, nil, &llmHITL{
			approveWhen: args[0].Table().RawGetString("approve_when"),
			store:       args[0].Table().RawGetString("store"),
			trace:       trace,
			snapshots:   snapshots,
			snapshotsMu: &snapshotsMu,
		})
		if err != nil || len(result) == 0 || !result[1].IsNil() || !result[0].IsTable() {
			return result, err
		}
		result[0].Table().RawSetString("plan", StringValue(planResult.Text))
		return result, nil
	})

	set("reflect", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'loop.reflect' (table expected)")
		}
		if call == nil {
			return nil, fmt.Errorf("loop.reflect requires a function caller")
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
		result, err := llmReact(opts, p, call, currentContext(), hostLimit(), trace, nil, &llmHITL{
			approveWhen: args[0].Table().RawGetString("approve_when"),
			store:       args[0].Table().RawGetString("store"),
			trace:       trace,
			snapshots:   snapshots,
			snapshotsMu: &snapshotsMu,
		})
		if err != nil || len(result) == 0 || !result[1].IsNil() || !result[0].IsTable() {
			return result, err
		}
		if reflectErr := llmReflectResult(args[0].Table(), result[0].Table(), p, currentContext(), hostLimit(), trace); !reflectErr.IsNil() {
			return []Value{NilValue(), reflectErr}, nil
		}
		return result, nil
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
		trace(LLMTraceEvent{Type: "snapshot_saved", Token: token})
		if len(args) >= 3 && llmIsSnapshotStore(args[2]) {
			if errVal, err := llmStoreSave(call, args[2].Table(), token, TableValue(snapshot)); err != nil {
				return nil, err
			} else if !errVal.IsNil() {
				return []Value{NilValue(), errVal}, nil
			}
			trace(LLMTraceEvent{Type: "snapshot_store_saved", Token: token, Store: true})
		}
		return []Value{StringValue(token), NilValue()}, nil
	})

	set("resume", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument to 'loop.resume' (token, approval expected)")
		}
		token := args[0].Str()
		var tools *Table
		var store Value
		if len(args) >= 3 && args[2].IsTable() {
			if llmIsSnapshotStore(args[2]) {
				store = args[2]
			} else {
				tools = args[2].Table()
			}
		}
		if len(args) >= 4 && llmIsSnapshotStore(args[3]) {
			store = args[3]
		}
		trace(LLMTraceEvent{Type: "resume_start", Token: token, Store: llmIsSnapshotStore(store)})
		snapshotsMu.Lock()
		snapshot, ok := snapshots[token]
		if !ok || !snapshot.IsTable() {
			snapshotsMu.Unlock()
			if llmIsSnapshotStore(store) {
				loaded, errVal, err := llmStoreLoad(call, store.Table(), token)
				if err != nil {
					return nil, err
				}
				if !errVal.IsNil() {
					return []Value{NilValue(), errVal}, nil
				}
				snapshot = loaded
				trace(LLMTraceEvent{Type: "resume_loaded", Token: token, Store: true})
			}
			if !snapshot.IsTable() {
				return []Value{NilValue(), llmErrorValue("validation", "snapshot not found")}, nil
			}
		} else {
			delete(snapshots, token)
			snapshotsMu.Unlock()
			trace(LLMTraceEvent{Type: "resume_loaded", Token: token})
		}
		result, err := llmResumeSnapshot(snapshot.Table(), args[1].Table(), tools, call)
		if err != nil {
			return nil, err
		}
		if llmIsSnapshotStore(store) {
			if errVal, err := llmStoreDelete(call, store.Table(), token); err != nil {
				return nil, err
			} else if !errVal.IsNil() {
				return []Value{NilValue(), errVal}, nil
			}
			trace(LLMTraceEvent{Type: "resume_store_deleted", Token: token, Store: true})
		}
		if len(result) > 0 && result[0].IsTable() {
			trace(LLMTraceEvent{Type: "resume_done", Token: token, Status: result[0].Table().RawGetString("status").Str(), Store: llmIsSnapshotStore(store)})
		}
		return result, nil
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

func llmIsSnapshotStore(v Value) bool {
	if !v.IsTable() {
		return false
	}
	t := v.Table()
	return t.RawGetString("save").IsFunction() &&
		t.RawGetString("load").IsFunction() &&
		t.RawGetString("delete").IsFunction()
}

func llmStoreSave(call FunctionCaller, store *Table, token string, snapshot Value) (Value, error) {
	return llmStoreCall(call, store.RawGetString("save"), []Value{StringValue(token), snapshot})
}

func llmStoreLoad(call FunctionCaller, store *Table, token string) (Value, Value, error) {
	result, errVal, err := llmStoreCall2(call, store.RawGetString("load"), []Value{StringValue(token)})
	if err != nil || !errVal.IsNil() {
		return NilValue(), errVal, err
	}
	return result, NilValue(), nil
}

func llmStoreDelete(call FunctionCaller, store *Table, token string) (Value, error) {
	return llmStoreCall(call, store.RawGetString("delete"), []Value{StringValue(token)})
}

func llmStoreCall(call FunctionCaller, fn Value, args []Value) (Value, error) {
	_, errVal, err := llmStoreCall2(call, fn, args)
	return errVal, err
}

func llmStoreCall2(call FunctionCaller, fn Value, args []Value) (Value, Value, error) {
	if call == nil {
		return NilValue(), llmErrorValue("internal", "snapshot store requires a function caller"), nil
	}
	results, err := call(fn, args)
	if err != nil {
		return NilValue(), NilValue(), err
	}
	if len(results) >= 2 && !results[1].IsNil() {
		return NilValue(), results[1], nil
	}
	if len(results) == 0 {
		return NilValue(), NilValue(), nil
	}
	return results[0], NilValue(), nil
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
		"temperature",
		"top_p",
		"response_format",
		"stream",
		"max_steps",
		"max_tool_retries",
		"max_history_tokens",
		"force_tool",
		"stop",
		"metadata",
		"output",
		"output_repair",
		"output_retries",
		"budget",
		"budget_tokens",
		"budget_turns",
		"budget_calls",
		"budget_money",
		"budget_time",
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
	if opts.RawGetString("response_format").IsNil() && opts.RawGetString("output").IsTable() {
		opts.RawSetString("response_format", TableValue(llmJSONResponseFormatTable()))
	}
	return opts, nil
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
	model := src.RawGetString("plan_model").Str()
	if model == "" {
		model = opts.RawGetString("model").Str()
	}
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
	res, err := provider.Turn(ctx, req)
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
	t.RawSet(IntValue(1), TableValue(llmMessageTable("system", "Create a concise execution plan. Do not call tools.")))
	if messages.IsTable() {
		for _, msg := range llmMessageValuesFromTable(messages.Table()) {
			t.RawSet(IntValue(int64(t.Length()+1)), msg)
		}
	}
	return TableValue(t)
}

func llmInjectPlan(opts *Table, plan string) {
	if plan == "" {
		return
	}
	messages := opts.RawGetString("messages")
	if !messages.IsTable() {
		return
	}
	merged := NewAppendArrayTable(messages.Table().Length() + 1)
	merged.RawSet(IntValue(1), TableValue(llmMessageTable("system", "Execution plan:\n"+plan)))
	for _, msg := range llmMessageValuesFromTable(messages.Table()) {
		merged.RawSet(IntValue(int64(merged.Length()+1)), msg)
	}
	opts.RawSetString("messages", TableValue(merged))
}

func llmReflectResult(src, result *Table, provider LLMProvider, ctx context.Context, maxHostResult int64, trace func(LLMTraceEvent)) Value {
	if result.RawGetString("status").Str() != "done" {
		return NilValue()
	}
	maxIters := toInt(src.RawGetString("max_iters"))
	if maxIters <= 0 {
		maxIters = 1
	}
	model := src.RawGetString("reflect_model").Str()
	if model == "" {
		model = src.RawGetString("model").Str()
	}
	reflections := NewAppendArrayTable(int(maxIters))
	text := result.RawGetString("text").Str()
	for i := int64(0); i < maxIters; i++ {
		messages := NewAppendArrayTable(2)
		prompt := src.RawGetString("reflect_prompt").Str()
		if prompt == "" {
			prompt = "Review the answer. If it can be improved, return the improved final answer only."
		}
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
		res, err := provider.Turn(ctx, req)
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

func llmProviderErrorValue(err error) Value {
	if err == nil {
		return llmErrorValue(LLMProviderErrorProvider, "")
	}
	return llmErrorValue(ClassifyLLMProviderError(err), err.Error())
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
		t.array[i+1] = StringValue(item)
	}
	return TableValue(t)
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
func llmAgentFunctionToToolTable(call FunctionCaller, agent Value) *Table {
	meta, _ := llmAgentMetadataForValue(agent)
	name := meta.Name
	if name == "" {
		if gf := agent.GoFunction(); gf != nil {
			name = strings.TrimPrefix(gf.Name, "llm.agent.")
		}
	}
	if name == "" {
		name = "agent"
	}
	wrapper := FunctionValue(&GoFunction{Name: "llm.agent_as_tool." + name, Fn: func(callArgs []Value) ([]Value, error) {
		if call == nil {
			return []Value{NilValue(), llmErrorValue("internal", "agent-as-tool requires a function caller")}, nil
		}
		results, err := call(agent, callArgs)
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
	tool := NewTable()
	tool.RawSetString("__llm_tool", BoolValue(true))
	tool.RawSetString("name", StringValue(name))
	tool.RawSetString("fn", wrapper)
	tool.RawSetString("description", StringValue(meta.Description))
	if len(meta.Params) > 0 {
		tool.RawSetString("params", llmStringArrayValue(meta.Params))
	}
	if !meta.Output.IsNil() {
		tool.RawSetString("schema", meta.Output)
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
func llmNormalizeToolsValue(call FunctionCaller, v Value) Value {
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
		out.array[i] = entry
	}
	return TableValue(out)
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

type llmHITL struct {
	approveWhen Value
	store       Value
	trace       func(LLMTraceEvent)
	snapshots   map[string]Value
	snapshotsMu *sync.Mutex
}

func llmReact(opts *Table, provider LLMProvider, call FunctionCaller, ctx context.Context, maxHostResult int64, trace func(LLMTraceEvent), ambient llmBudgetGroup, hitls ...*llmHITL) ([]Value, error) {
	var hitl *llmHITL
	if len(hitls) > 0 {
		hitl = hitls[0]
	}
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
	budgets := ambient.with(llmBudgetFromOptions(opts))
	cancel := llmCancelFromOptions(opts, ctx)
	for step := 0; step < maxSteps; step++ {
		if err := cancel.check(); !err.IsNil() {
			llmTrace(trace, LLMTraceEvent{Type: "react_error", ErrorKind: llmErrorKind(err), Message: err.Table().RawGetString("message").Str()})
			return []Value{NilValue(), err}, nil
		}
		if err := budgets.beforeTurn(); !err.IsNil() {
			llmTrace(trace, LLMTraceEvent{Type: "react_error", ErrorKind: llmErrorKind(err), Message: err.Table().RawGetString("message").Str()})
			return []Value{NilValue(), err}, nil
		}
		requestHistory := history
		if maxHistoryTokens > 0 {
			requestHistory = chatWindow(llmTableFromValues(history).Table(), maxHistoryTokens)
		}
		req := LLMTurnRequest{
			Model:          model,
			Messages:       llmMessagesFromValue(llmTableFromValues(requestHistory)),
			Tools:          llmToolsFromValue(toolsValue),
			ForceTool:      llmForceToolFromValue(opts.RawGetString("force_tool")),
			MaxTokens:      toInt(opts.RawGetString("max_tokens")),
			Temperature:    llmOptionalFloatFromValue(opts.RawGetString("temperature")),
			TopP:           llmOptionalFloatFromValue(opts.RawGetString("top_p")),
			ResponseFormat: llmAnyFromValue(opts.RawGetString("response_format")),
			Stream:         opts.RawGetString("stream").Truthy(),
			Stop:           llmStringSliceFromValue(opts.RawGetString("stop")),
			Metadata:       llmStringMapFromValue(opts.RawGetString("metadata")),
		}
		llmTrace(trace, LLMTraceEvent{Type: "turn_start", Model: req.Model, Step: int64(step), MessageCount: len(req.Messages), ToolCount: len(req.Tools)})
		res, err := provider.Turn(ctx, req)
		if err != nil {
			llmTrace(trace, LLMTraceEvent{Type: "turn_error", Model: req.Model, Step: int64(step), ErrorKind: ClassifyLLMProviderError(err), Message: err.Error()})
			return []Value{NilValue(), llmProviderErrorValue(err)}, nil
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
		budgets.chargeTurn(res.Usage)
		switch res.Status {
		case "", "final_answer":
			result := llmReactResultValue("done", res.Text, "", turnValue, history)
			if value, errValue := llmStructuredOutputValue(opts, res.Text); !errValue.IsNil() {
				repairedValue, repairedTurn, repairedText, repairErr, repaired := llmRepairStructuredOutput(opts, provider, ctx, maxHostResult, trace, budgets, cancel, model, toolsValue, history, res.Text, errValue, int64(step))
				if !repairErr.IsNil() {
					llmTrace(trace, LLMTraceEvent{Type: "react_error", ErrorKind: llmErrorKind(repairErr), Message: repairErr.Table().RawGetString("message").Str()})
					return []Value{NilValue(), repairErr}, nil
				}
				if repaired {
					result = llmReactResultValue("done", repairedText, "", repairedTurn, history)
					result.Table().RawSetString("value", repairedValue)
					llmTrace(trace, LLMTraceEvent{Type: "react_done", Model: model, Step: int64(step), Status: "done"})
					return []Value{result, NilValue()}, nil
				}
				llmTrace(trace, LLMTraceEvent{Type: "react_error", ErrorKind: llmErrorKind(errValue), Message: errValue.Table().RawGetString("message").Str()})
				return []Value{NilValue(), errValue}, nil
			} else if !value.IsNil() {
				result.Table().RawSetString("value", value)
			}
			llmTrace(trace, LLMTraceEvent{Type: "react_done", Model: model, Step: int64(step), Status: "done"})
			return []Value{result, NilValue()}, nil
		case "stop":
			llmTrace(trace, LLMTraceEvent{Type: "react_stopped", Model: model, Step: int64(step), Status: "stopped", Message: res.Reason})
			return []Value{llmReactResultValue("stopped", "", res.Reason, turnValue, history), NilValue()}, nil
		case "tool_calls":
			for i := range res.Calls {
				callValue := llmToolCallValue(res.Calls[i])
				llmTrace(trace, LLMTraceEvent{Type: "tool_call", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID})
				if pending, err := llmMaybePauseForApproval(hitl, call, llmTableFromValues(history), callValue); !pending.IsNil() || err != nil {
					if err != nil {
						llmTrace(trace, LLMTraceEvent{Type: "tool_fatal", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID, ErrorKind: "internal", Message: err.Error()})
						return []Value{NilValue(), llmErrorValue("internal", err.Error())}, nil
					}
					llmTrace(trace, LLMTraceEvent{Type: "react_stopped", Model: model, Step: int64(step), Status: "pending", Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID})
					return []Value{pending, NilValue()}, nil
				}
				history = append(history, llmAssistantCallMessage(callValue))
				if err := cancel.check(); !err.IsNil() {
					llmTrace(trace, LLMTraceEvent{Type: "tool_fatal", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID, ErrorKind: llmErrorKind(err)})
					return []Value{NilValue(), err}, nil
				}
				if err := budgets.beforeToolCall(); !err.IsNil() {
					llmTrace(trace, LLMTraceEvent{Type: "tool_fatal", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID, ErrorKind: llmErrorKind(err)})
					return []Value{NilValue(), err}, nil
				}
				dispatchResult, err := llmDispatchWithRetry(call, callValue.Table(), tools, maxToolRetries, trace, int64(step), res.Calls[i])
				if !err.IsNil() {
					if llmErrorKind(err) == "pending" {
						pendingPayload := err.Table().RawGetString("pending")
						llmTrace(trace, LLMTraceEvent{Type: "react_stopped", Model: model, Step: int64(step), Status: "pending", Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID})
						if pendingPayload.IsTable() {
							pendingPayload.Table().RawSetString("history", llmTableFromValues(history))
							return []Value{pendingPayload, NilValue()}, nil
						}
						pending := NewTable()
						pending.RawSetString("status", StringValue("pending"))
						pending.RawSetString("history", llmTableFromValues(history))
						return []Value{TableValue(pending), NilValue()}, nil
					}
					llmTrace(trace, LLMTraceEvent{Type: "tool_fatal", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID, ErrorKind: llmErrorKind(err)})
					return []Value{NilValue(), err}, nil
				}
				if len(dispatchResult) >= 2 && !dispatchResult[1].IsNil() {
					if llmErrorKind(dispatchResult[1]) == "pending" {
						pendingPayload := dispatchResult[1].Table().RawGetString("pending")
						llmTrace(trace, LLMTraceEvent{Type: "react_stopped", Model: model, Step: int64(step), Status: "pending", Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID})
						if pendingPayload.IsTable() {
							pendingPayload.Table().RawSetString("history", llmTableFromValues(history))
							return []Value{pendingPayload, NilValue()}, nil
						}
						pending := NewTable()
						pending.RawSetString("status", StringValue("pending"))
						pending.RawSetString("history", llmTableFromValues(history))
						return []Value{TableValue(pending), NilValue()}, nil
					}
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

func llmMaybePauseForApproval(hitl *llmHITL, call FunctionCaller, history, pending Value) (Value, error) {
	if hitl == nil || hitl.approveWhen.IsNil() || !hitl.approveWhen.IsFunction() {
		return NilValue(), nil
	}
	results, err := call(hitl.approveWhen, []Value{pending})
	if err != nil {
		return NilValue(), err
	}
	if len(results) == 0 || !results[0].Truthy() {
		return NilValue(), nil
	}
	token, err := llmSnapshotToken()
	if err != nil {
		return NilValue(), err
	}
	snapshot := NewTable()
	snapshot.RawSetString("history", history)
	snapshot.RawSetString("pending", pending)
	hitl.snapshotsMu.Lock()
	hitl.snapshots[token] = TableValue(snapshot)
	hitl.snapshotsMu.Unlock()
	llmTrace(hitl.trace, LLMTraceEvent{Type: "snapshot_saved", Token: token, Tool: pending.Table().RawGetString("tool").Str(), CallID: pending.Table().RawGetString("id").Str()})
	if llmIsSnapshotStore(hitl.store) {
		if errVal, err := llmStoreSave(call, hitl.store.Table(), token, TableValue(snapshot)); err != nil {
			return NilValue(), err
		} else if !errVal.IsNil() {
			return NilValue(), fmt.Errorf("%s", errVal.String())
		}
		llmTrace(hitl.trace, LLMTraceEvent{Type: "snapshot_store_saved", Token: token, Store: true, Tool: pending.Table().RawGetString("tool").Str(), CallID: pending.Table().RawGetString("id").Str()})
	}

	result := NewTable()
	result.RawSetString("status", StringValue("pending"))
	result.RawSetString("token", StringValue(token))
	result.RawSetString("payload", pending)
	result.RawSetString("pending", pending)
	result.RawSetString("history", history)
	return TableValue(result), nil
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

func llmRepairStructuredOutput(opts *Table, provider LLMProvider, ctx context.Context, maxHostResult int64, trace func(LLMTraceEvent), budgets llmBudgetGroup, cancel llmCancel, model string, toolsValue Value, history []Value, previousText string, validationErr Value, step int64) (Value, Value, string, Value, bool) {
	maxRetries := llmOutputRepairRetries(opts)
	if maxRetries <= 0 {
		return NilValue(), NilValue(), "", NilValue(), false
	}
	lastErr := validationErr
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := cancel.check(); !err.IsNil() {
			return NilValue(), NilValue(), "", err, true
		}
		if err := budgets.beforeTurn(); !err.IsNil() {
			return NilValue(), NilValue(), "", err, true
		}
		repairHistory := llmStructuredOutputRepairHistory(opts, history, previousText, lastErr)
		req := LLMTurnRequest{
			Model:          model,
			Messages:       llmMessagesFromValue(llmTableFromValues(repairHistory)),
			Tools:          llmToolsFromValue(toolsValue),
			ForceTool:      llmForceToolFromValue(opts.RawGetString("force_tool")),
			MaxTokens:      toInt(opts.RawGetString("max_tokens")),
			Temperature:    llmOptionalFloatFromValue(opts.RawGetString("temperature")),
			TopP:           llmOptionalFloatFromValue(opts.RawGetString("top_p")),
			ResponseFormat: llmAnyFromValue(opts.RawGetString("response_format")),
			Stream:         opts.RawGetString("stream").Truthy(),
			Stop:           llmStringSliceFromValue(opts.RawGetString("stop")),
			Metadata:       llmStringMapFromValue(opts.RawGetString("metadata")),
		}
		llmTrace(trace, LLMTraceEvent{Type: "turn_start", Model: req.Model, Step: step, Attempt: int64(attempt), MessageCount: len(req.Messages), ToolCount: len(req.Tools)})
		res, err := provider.Turn(ctx, req)
		if err != nil {
			llmTrace(trace, LLMTraceEvent{Type: "turn_error", Model: req.Model, Step: step, Attempt: int64(attempt), ErrorKind: ClassifyLLMProviderError(err), Message: err.Error()})
			return NilValue(), NilValue(), "", llmProviderErrorValue(err), true
		}
		if err := cancel.check(); !err.IsNil() {
			return NilValue(), NilValue(), "", err, true
		}
		llmTrace(trace, LLMTraceEvent{Type: "turn_end", Model: req.Model, Step: step, Attempt: int64(attempt), Status: llmResultStatus(res), MessageCount: len(req.Messages), ToolCount: len(req.Tools), Usage: res.Usage})
		turnValue := llmResultValue(res)
		if err := CheckHostResultBytes(maxHostResult, turnValue); err != nil {
			return NilValue(), NilValue(), "", llmErrorValue("internal", err.Error()), true
		}
		budgets.chargeTurn(res.Usage)
		if llmResultStatus(res) != "final_answer" {
			lastErr = llmErrorValue("validation", "structured output repair did not return a final answer")
			previousText = res.Text
			continue
		}
		value, errValue := llmStructuredOutputValue(opts, res.Text)
		if errValue.IsNil() {
			return value, turnValue, res.Text, NilValue(), true
		}
		lastErr = errValue
		previousText = res.Text
	}
	return NilValue(), NilValue(), "", lastErr, true
}

func llmOutputRepairRetries(opts *Table) int {
	if opts == nil {
		return 0
	}
	retries := int(toInt(opts.RawGetString("output_retries")))
	if retries < 0 {
		retries = 0
	}
	if retries == 0 && opts.RawGetString("output_repair").Truthy() {
		retries = 1
	}
	return retries
}

func llmStructuredOutputRepairHistory(opts *Table, history []Value, previousText string, validationErr Value) []Value {
	repairHistory := make([]Value, 0, len(history)+1)
	repairHistory = append(repairHistory, history...)
	repairHistory = append(repairHistory, TableValue(llmMessageTable("user", llmStructuredOutputRepairPrompt(opts, previousText, validationErr))))
	return repairHistory
}

func llmStructuredOutputRepairPrompt(opts *Table, previousText string, validationErr Value) string {
	prompt := ""
	if repair := opts.RawGetString("output_repair"); repair.IsString() {
		prompt = strings.TrimSpace(repair.Str())
	}
	if prompt == "" {
		prompt = "Return only a JSON object that matches the requested output shape."
	}
	message := ""
	if validationErr.IsTable() {
		message = validationErr.Table().RawGetString("message").Str()
	}
	shape := llmStructuredOutputShapeJSON(opts)
	var b strings.Builder
	b.WriteString(prompt)
	if message != "" {
		b.WriteString("\nValidation error: ")
		b.WriteString(message)
	}
	if shape != "" {
		b.WriteString("\nOutput shape example: ")
		b.WriteString(shape)
	}
	b.WriteString("\nPrevious response:\n")
	b.WriteString(previousText)
	return b.String()
}

func llmStructuredOutputShapeJSON(opts *Table) string {
	if opts == nil || !opts.RawGetString("output").IsTable() {
		return ""
	}
	data := llmAnyFromValue(opts.RawGetString("output"))
	encoded, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(encoded)
}

type llmBudget struct {
	maxTokens int64
	maxTurns  int64
	maxCalls  int64
	maxMoney  float64
	maxTime   time.Duration

	usedTokens int64
	usedTurns  int64
	usedCalls  int64
	usedMoney  float64
	started    time.Time
}

type llmBudgetGroup []*llmBudget

func (g llmBudgetGroup) with(b *llmBudget) llmBudgetGroup {
	if b == nil {
		return g
	}
	out := make(llmBudgetGroup, 0, len(g)+1)
	out = append(out, g...)
	out = append(out, b)
	return out
}

func (g llmBudgetGroup) beforeTurn() Value {
	for _, b := range g {
		if err := b.checkTurn(); !err.IsNil() {
			return err
		}
	}
	for _, b := range g {
		b.startTurn()
	}
	return NilValue()
}

func (g llmBudgetGroup) chargeTurn(usage LLMTurnUsage) {
	for _, b := range g {
		b.chargeTurn(usage)
	}
}

func (g llmBudgetGroup) beforeToolCall() Value {
	for _, b := range g {
		if err := b.checkToolCall(); !err.IsNil() {
			return err
		}
	}
	for _, b := range g {
		b.startToolCall()
	}
	return NilValue()
}

func newLLMBudget() *llmBudget {
	return &llmBudget{
		maxTokens: -1,
		maxTurns:  -1,
		maxCalls:  -1,
		maxMoney:  -1,
		maxTime:   -1,
	}
}

func llmBudgetFromOptions(opts *Table) *llmBudget {
	b := newLLMBudget()
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
	if v := opts.RawGetString("budget_time"); !v.IsNil() {
		b.maxTime = llmBudgetDuration(v)
	}
	if t := opts.RawGetString("budget"); t.IsTable() {
		llmApplyBudgetConfig(b, t.Table())
	}
	llmNormalizeBudget(b)
	return b
}

func llmBudgetFromConfig(config *Table) *llmBudget {
	b := newLLMBudget()
	llmApplyBudgetConfig(b, config)
	llmNormalizeBudget(b)
	return b
}

func llmApplyBudgetConfig(b *llmBudget, config *Table) {
	if b == nil || config == nil {
		return
	}
	if v := config.RawGetString("tokens"); !v.IsNil() {
		b.maxTokens = toInt(v)
	}
	if v := config.RawGetString("turns"); !v.IsNil() {
		b.maxTurns = toInt(v)
	}
	if v := config.RawGetString("calls"); !v.IsNil() {
		b.maxCalls = toInt(v)
	}
	if v := config.RawGetString("money"); !v.IsNil() {
		b.maxMoney = toFloat(v)
	}
	if v := config.RawGetString("time"); !v.IsNil() {
		b.maxTime = llmBudgetDuration(v)
	}
}

func llmNormalizeBudget(b *llmBudget) {
	if b == nil {
		return
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
	if b.maxTime < 0 {
		b.maxTime = -1
	}
	if b.maxTime >= 0 {
		b.started = time.Now()
	}
}

func (b *llmBudget) beforeTurn() Value {
	if b == nil {
		return NilValue()
	}
	if err := b.checkTurn(); !err.IsNil() {
		return err
	}
	b.startTurn()
	return NilValue()
}

func (b *llmBudget) checkTurn() Value {
	if b == nil {
		return NilValue()
	}
	if err := b.beforeWork(); !err.IsNil() {
		return err
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
	return NilValue()
}

func (b *llmBudget) startTurn() {
	if b != nil {
		b.usedTurns++
	}
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
	if err := b.checkToolCall(); !err.IsNil() {
		return err
	}
	b.startToolCall()
	return NilValue()
}

func (b *llmBudget) checkToolCall() Value {
	if b == nil {
		return NilValue()
	}
	if err := b.beforeWork(); !err.IsNil() {
		return err
	}
	if b.maxCalls >= 0 && b.usedCalls >= b.maxCalls {
		return llmBudgetError("calls", b.maxCalls, b.usedCalls)
	}
	return NilValue()
}

func (b *llmBudget) startToolCall() {
	if b != nil {
		b.usedCalls++
	}
}

func (b *llmBudget) beforeWork() Value {
	if b.maxTime >= 0 && !b.started.IsZero() && time.Since(b.started) >= b.maxTime {
		return llmCancelError("deadline exceeded")
	}
	return NilValue()
}

func llmBudgetDuration(v Value) time.Duration {
	secs := toFloat(v)
	if secs < 0 {
		return -1
	}
	return time.Duration(secs * float64(time.Second))
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

func llmStructuredOutputValue(opts *Table, text string) (Value, Value) {
	if opts == nil || !opts.RawGetString("output").IsTable() {
		return NilValue(), NilValue()
	}
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var data any
	if err := dec.Decode(&data); err != nil {
		return NilValue(), llmErrorValue("validation", "structured output is not valid JSON: "+err.Error())
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return NilValue(), llmErrorValue("validation", "structured output contains trailing JSON")
		}
		return NilValue(), llmErrorValue("validation", "structured output is not valid JSON: "+err.Error())
	}
	value := jsonGoToGScript(data)
	if !value.IsTable() {
		return NilValue(), llmErrorValue("validation", "structured output JSON must decode to a table")
	}
	if message := llmValidateStructuredOutputShape(opts.RawGetString("output").Table(), value.Table()); message != "" {
		return NilValue(), llmErrorValue("validation", message)
	}
	return value, NilValue()
}

func llmValidateStructuredOutputShape(expected, actual *Table) string {
	return llmValidateStructuredOutputValue(TableValue(expected), TableValue(actual), "")
}

func llmValidateStructuredOutputValue(expected Value, actual Value, path string) string {
	expectedType := llmStructuredOutputExampleType(expected)
	if expectedType != "" && !llmStructuredOutputTypeMatches(expectedType, actual) {
		return fmt.Sprintf("structured output field %q has type %s, want %s", path, llmStructuredOutputActualType(actual), expectedType)
	}
	if !expected.IsTable() || !actual.IsTable() {
		return ""
	}
	expectedTable := expected.Table()
	actualTable := actual.Table()
	if expectedTable.Length() > 0 {
		return llmValidateStructuredOutputArray(expectedTable, actualTable, path)
	}
	return llmValidateStructuredOutputObject(expectedTable, actualTable, path)
}

func llmValidateStructuredOutputObject(expected, actual *Table, path string) string {
	if expected == nil || actual == nil {
		return ""
	}
	for _, key := range expected.PairsKeysSnapshot() {
		if !key.IsString() {
			continue
		}
		name := key.Str()
		expectedValue := expected.RawGetString(name)
		actualValue := actual.RawGetString(name)
		if actualValue.IsNil() {
			return fmt.Sprintf("structured output missing field %q", llmStructuredOutputPath(path, name))
		}
		if message := llmValidateStructuredOutputValue(expectedValue, actualValue, llmStructuredOutputPath(path, name)); message != "" {
			return message
		}
	}
	return ""
}

func llmValidateStructuredOutputArray(expected, actual *Table, path string) string {
	if expected == nil || actual == nil {
		return ""
	}
	if actual.Length() == 0 {
		return fmt.Sprintf("structured output field %q must contain at least one item", path)
	}
	exemplar := expected.RawGet(IntValue(1))
	if exemplar.IsNil() {
		return ""
	}
	for i := 1; i <= actual.Length(); i++ {
		item := actual.RawGet(IntValue(int64(i)))
		if item.IsNil() {
			return fmt.Sprintf("structured output missing field %q", llmStructuredOutputIndexPath(path, i))
		}
		if message := llmValidateStructuredOutputValue(exemplar, item, llmStructuredOutputIndexPath(path, i)); message != "" {
			return message
		}
	}
	return ""
}

func llmStructuredOutputPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func llmStructuredOutputIndexPath(prefix string, index int) string {
	if prefix == "" {
		return fmt.Sprintf("[%d]", index)
	}
	return fmt.Sprintf("%s[%d]", prefix, index)
}

func llmStructuredOutputExampleType(v Value) string {
	switch {
	case v.IsString():
		return "string"
	case v.IsNumber():
		return "number"
	case v.IsBool():
		return "bool"
	case v.IsTable():
		return "table"
	default:
		return ""
	}
}

func llmStructuredOutputTypeMatches(expected string, v Value) bool {
	switch expected {
	case "string":
		return v.IsString()
	case "number":
		return v.IsNumber()
	case "bool":
		return v.IsBool()
	case "table":
		return v.IsTable()
	default:
		return true
	}
}

func llmStructuredOutputActualType(v Value) string {
	if v.IsNumber() {
		return "number"
	}
	if v.IsBool() {
		return "bool"
	}
	if v.IsString() {
		return "string"
	}
	if v.IsTable() {
		return "table"
	}
	return v.TypeName()
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
