package bind

import (
	"context"
	"fmt"
	"strings"

	stdlibllm "github.com/never-labs/leia/internal/stdlib/lib/llm"
)

// BuildLLMLib creates the "llm" standard library table. It is the first-stage
// runtime substrate for the agent layer: future syntax can compile to these
// functions without changing provider or tool-dispatch semantics.
func BuildLLMLib(call ScriptFunctionCaller, provider func() LLMProvider, providerFactory func() LLMProviderFactory, maxHostResult func() int64, ctx func() context.Context, traces ...LLMTraceSink) *Table {
	b := newLLMLibBuilder(call, provider, providerFactory, maxHostResult, ctx, traces)
	b.register()
	return b.t
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
