package bind

import "fmt"

func registerDialectAI(register dialectRegisterFunc, opts HostOptions) {
	ai := dialectAI{opts: opts}
	register([]string{"prompt"}, dialectHandler{
		eval:  func(body Value, options *Table) ([]Value, error) { return []Value{dialectPrompt(body, options)}, nil },
		block: func(body Value, options *Table) ([]Value, error) { return []Value{dialectPrompt(body, options)}, nil },
	})
	register([]string{"quote"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return []Value{dialectQuote("quote", body, options)}, nil
		},
		block: func(body Value, options *Table) ([]Value, error) {
			return []Value{dialectQuote("quote", body, options)}, nil
		},
	})
	register([]string{"model"}, dialectHandler{
		block: ai.models,
		meta:  dialectMetadata{Category: "llm", Capabilities: []string{"llm.turn"}, Builtin: true},
	})
	register([]string{"turn"}, dialectHandler{
		block: ai.turn,
		meta:  dialectMetadata{Category: "llm", Capabilities: []string{"llm.turn"}, Builtin: true},
	})
	register([]string{"tool"}, dialectHandler{
		block: ai.tool,
		meta:  dialectMetadata{Category: "llm", Capabilities: []string{"llm.turn"}, Builtin: true},
	})
	register([]string{"agent"}, dialectHandler{
		block: ai.agent,
		meta:  dialectMetadata{Category: "llm", Capabilities: []string{"llm.turn"}, Builtin: true},
	})
}

func dialectPrompt(body Value, opts *Table) Value {
	out := NewTable()
	out.RawSetString("body", body)
	text := body.String()
	role := NilValue()
	if body.IsTable() {
		t := body.Table()
		if v := t.RawGetString("text"); v.IsString() {
			text = v.Str()
		}
		role = t.RawGetString("role")
	}
	out.RawSetString("text", StringValue(text))
	if opts != nil {
		out.RawSetString("options", TableValue(opts))
		if optRole := opts.RawGetString("role"); optRole.IsString() {
			role = optRole
		}
	}
	if role.IsString() && role.Str() != "" {
		out.RawSetString("role", role)
	}
	return TableValue(out)
}

type dialectAI struct {
	opts HostOptions
}

func (d dialectAI) models(body Value, _ *Table) ([]Value, error) {
	if !body.IsTable() {
		return nil, fmt.Errorf("model dialect requires a field block")
	}
	return d.callLLM("register_models", body)
}

func (d dialectAI) turn(body Value, _ *Table) ([]Value, error) {
	if !body.IsTable() {
		return nil, fmt.Errorf("turn dialect requires a field block")
	}
	return d.callLLM("turn", body)
}

func (d dialectAI) tool(body Value, _ *Table) ([]Value, error) {
	if !body.IsTable() {
		return nil, fmt.Errorf("tool dialect requires a field block")
	}
	t := body.Table()
	name := t.RawGetString("name")
	fn := t.RawGetString("fn")
	if !name.IsString() || name.Str() == "" {
		return nil, fmt.Errorf("tool dialect requires string field name")
	}
	if !fn.IsFunction() {
		return nil, fmt.Errorf("tool dialect requires function field fn")
	}
	return d.callLLM("tool", name, fn, body)
}

func (d dialectAI) agent(body Value, _ *Table) ([]Value, error) {
	if !body.IsTable() {
		return nil, fmt.Errorf("agent dialect requires a field block")
	}
	t := body.Table()
	name := t.RawGetString("name")
	config := t.RawGetString("config")
	if config.IsNil() {
		config = t.RawGetString("fn")
	}
	if !name.IsString() || name.Str() == "" {
		return nil, fmt.Errorf("agent dialect requires string field name")
	}
	if config.IsNil() {
		config = FunctionValue(&GoFunction{
			Name: "llm.agent.config." + name.Str(),
			Fn: func(callArgs []Value) ([]Value, error) {
				cfg := dialectAgentConfigTable(t, callArgs)
				return []Value{TableValue(cfg), NilValue()}, nil
			},
		})
	} else if !config.IsFunction() {
		return nil, fmt.Errorf("agent dialect field config must be a function when provided")
	}
	args := []Value{name, config}
	if flow := t.RawGetString("flow"); !flow.IsNil() {
		args = append(args, flow)
	} else {
		args = append(args, NilValue())
	}
	meta := NewTable()
	for _, key := range []string{"params", "output", "description"} {
		if v := t.RawGetString(key); !v.IsNil() {
			meta.RawSetString(key, v)
		}
	}
	if len(meta.PairsKeysSnapshot()) > 0 {
		args = append(args, TableValue(meta))
	}
	return d.callLLM("agent", args...)
}

func dialectAgentConfigTable(src *Table, callArgs []Value) *Table {
	out := NewTable()
	for _, key := range src.PairsKeysSnapshot() {
		if key.IsString() && dialectAgentMetadataField(key.Str()) {
			continue
		}
		out.RawSet(key, src.RawGet(key))
	}
	if out.RawGetString("system").IsNil() {
		if instructions := src.RawGetString("instructions"); !instructions.IsNil() {
			out.RawSetString("system", dialectAITextValue(instructions))
		}
	}
	if out.RawGetString("user").IsNil() && out.RawGetString("messages").IsNil() && len(callArgs) > 0 {
		out.RawSetString("user", dialectAITextValue(callArgs[0]))
	}
	if len(callArgs) > 0 {
		out.RawSetString("args", dialectAIArgsValue(callArgs))
		if params := src.RawGetString("params"); params.IsTable() {
			out.RawSetString("input", dialectAIParamArgsValue(params.Table(), callArgs))
		}
	}
	return out
}

func dialectAgentMetadataField(key string) bool {
	switch key {
	case "name", "config", "fn", "flow", "params", "description":
		return true
	default:
		return false
	}
}

func dialectAITextValue(v Value) Value {
	if v.IsTable() {
		t := v.Table()
		if text := t.RawGetString("text"); text.IsString() {
			return text
		}
	}
	return StringValue(v.Str())
}

func dialectAIArgsValue(args []Value) Value {
	out := NewSequentialArrayTable(len(args))
	for i, arg := range args {
		out.RawSetInt(int64(i+1), arg)
	}
	return TableValue(out)
}

func dialectAIParamArgsValue(params *Table, args []Value) Value {
	out := NewTable()
	for i := 1; i <= params.Length() && i <= len(args); i++ {
		name := params.RawGetInt(int64(i))
		if name.IsString() && name.Str() != "" {
			out.RawSetString(name.Str(), args[i-1])
		}
	}
	return TableValue(out)
}

func (d dialectAI) callLLM(name string, args ...Value) ([]Value, error) {
	if d.opts.Global == nil {
		return nil, fmt.Errorf("%s dialect requires llm stdlib", name)
	}
	llm := d.opts.Global("llm")
	if !llm.IsTable() {
		return nil, fmt.Errorf("%s dialect requires llm stdlib", name)
	}
	fn := llm.Table().RawGetString(name)
	if !fn.IsFunction() {
		return nil, fmt.Errorf("%s dialect requires llm.%s", name, name)
	}
	if d.opts.Call != nil {
		return d.opts.Call(fn, args)
	}
	if gf := fn.GoFunction(); gf != nil {
		return gf.Fn(args)
	}
	return nil, fmt.Errorf("%s dialect requires host function caller", name)
}

func dialectQuote(tag string, body Value, opts *Table) Value {
	out := NewTable()
	out.RawSetString("dialect", StringValue(tag))
	out.RawSetString("body", body)
	out.RawSetString("kind", StringValue(body.TypeName()))
	if opts != nil {
		out.RawSetString("options", TableValue(opts))
	}
	return TableValue(out)
}
