package bind

import (
	"fmt"
	"sort"
	"strings"
)

// BuildDialect creates the "dialect" standard library table. Dialects are a
// small native dispatch layer used by tagged literals and tagged blocks:
// sh`...`, cmd`...`, shellwords`...`, glob`...`, json`...`, prompt`...`,
// quote { ... }, and small safe data transforms such as path`...`, url`...`, words`...`, nums`...`,
// mdtable`...`, kv`...`, env`...`, jsonl`...`, semver`...`, html_escape`...`, urlquery`...`, mime`...`,
// urlpath`...`, duration`...`, tap`...`, junit`...`, base64`...`, and hash`...`.
func BuildDialect(opts HostOptions, maxHostResult func() int64) *Table {
	t := markStdlibBoundModule(NewTable())

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "dialect." + name, Fn: fn}))
	}

	registry := newDialectRegistry()
	register := registry.register

	registerDialectShellFS(register, opts, maxHostResult)
	registerDialectText(register, maxHostResult)
	registerDialectWeb(register)
	registerDialectData(register, maxHostResult)
	registerDialectAI(register)

	eval := func(tag string, body Value, options *Table) ([]Value, error) {
		handler, ok := registry.handler(tag)
		if !ok || handler.eval == nil {
			return nil, fmt.Errorf("unknown dialect %q (available: %s)", tag, registry.availableText())
		}
		return handler.eval(body, options)
	}

	set("eval", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() {
			return nil, fmt.Errorf("bad arguments to 'dialect.eval' (tag and body expected)")
		}
		return eval(args[0].Str(), args[1], optionalTableArg(args, 2))
	})

	set("tags", func(args []Value) ([]Value, error) {
		out := NewAppendArrayTable(len(registry.names))
		for i, name := range registry.names {
			out.RawSetInt(int64(i+1), StringValue(name))
		}
		return []Value{TableValue(out)}, nil
	})

	set("register", func(args []Value) ([]Value, error) {
		names, handler, err := dialectUserHandler(args, opts.Call)
		if err != nil {
			return nil, err
		}
		if len(names) == 0 {
			return nil, fmt.Errorf("bad arguments to 'dialect.register' (name expected)")
		}
		if err := registry.tryRegister(names, handler); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	})

	set("eval_block", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() {
			return nil, fmt.Errorf("bad arguments to 'dialect.eval_block' (tag and config expected)")
		}
		tag := args[0].Str()
		optsTbl := optionalTableArg(args, 2)
		if handler, ok := registry.handler(tag); ok && handler.block != nil {
			return handler.block(args[1], optsTbl)
		}
		return eval(tag, args[1], optsTbl)
	})

	set("eval_raw", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() {
			return nil, fmt.Errorf("bad arguments to 'dialect.eval_raw' (tag and thunk expected)")
		}
		optsTbl := optionalTableArg(args, 2)
		return []Value{dialectQuote(args[0].Str(), args[1], optsTbl)}, nil
	})

	return t
}

type dialectHandler struct {
	eval  func(Value, *Table) ([]Value, error)
	block func(Value, *Table) ([]Value, error)
}

type dialectRegisterFunc func([]string, dialectHandler)

type dialectRegistry struct {
	handlers map[string]dialectHandler
	names    []string
}

func newDialectRegistry() *dialectRegistry {
	return &dialectRegistry{handlers: make(map[string]dialectHandler)}
}

func (r *dialectRegistry) register(names []string, handler dialectHandler) {
	if err := r.tryRegister(names, handler); err != nil {
		panic(err)
	}
}

func (r *dialectRegistry) tryRegister(names []string, handler dialectHandler) error {
	if handler.eval == nil && handler.block == nil {
		return fmt.Errorf("dialect registry: handler requires eval or block")
	}
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name == "" {
			return fmt.Errorf("dialect registry: empty dialect name")
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("dialect registry: duplicate dialect %q", name)
		}
		seen[name] = struct{}{}
		if _, exists := r.handlers[name]; exists {
			return fmt.Errorf("dialect registry: duplicate dialect %q", name)
		}
	}
	for _, name := range names {
		r.handlers[name] = handler
		r.names = append(r.names, name)
	}
	sort.Strings(r.names)
	return nil
}

func (r *dialectRegistry) handler(name string) (dialectHandler, bool) {
	handler, ok := r.handlers[name]
	return handler, ok
}

func (r *dialectRegistry) availableText() string {
	return strings.Join(r.names, ", ")
}

func dialectFailFast(opts *Table) bool {
	return opts != nil && opts.RawGetString("fail_fast").Truthy()
}

func optionalTableArg(args []Value, idx int) *Table {
	if len(args) > idx && args[idx].IsTable() {
		return args[idx].Table()
	}
	return nil
}

func dialectUserHandler(args []Value, call ScriptFunctionCaller) ([]string, dialectHandler, error) {
	if len(args) == 0 {
		return nil, dialectHandler{}, fmt.Errorf("bad arguments to 'dialect.register' (name and function expected)")
	}
	if call == nil {
		return nil, dialectHandler{}, fmt.Errorf("dialect.register requires script callback support")
	}
	if args[0].IsTable() {
		return dialectUserHandlerFromSpec(args[0].Table(), call)
	}
	if !args[0].IsString() || len(args) < 2 || !args[1].IsFunction() {
		return nil, dialectHandler{}, fmt.Errorf("bad arguments to 'dialect.register' (name and function expected)")
	}
	names, err := dialectRegisterNames(args[0], optionalTableArg(args, 2))
	if err != nil {
		return nil, dialectHandler{}, err
	}
	evalFn := args[1]
	blockFn := evalFn
	if opts := optionalTableArg(args, 2); opts != nil {
		if block := opts.RawGetString("block"); block.IsFunction() {
			blockFn = block
		}
		if block := opts.RawGetString("block_fn"); block.IsFunction() {
			blockFn = block
		}
	}
	return names, dialectScriptHandler(call, evalFn, blockFn), nil
}

func dialectUserHandlerFromSpec(spec *Table, call ScriptFunctionCaller) ([]string, dialectHandler, error) {
	name := spec.RawGetString("name")
	if name.IsNil() {
		name = spec.RawGetString("tag")
	}
	evalFn := spec.RawGetString("eval")
	if evalFn.IsNil() {
		evalFn = spec.RawGetString("fn")
	}
	if !evalFn.IsFunction() {
		return nil, dialectHandler{}, fmt.Errorf("dialect.register spec requires eval or fn function")
	}
	blockFn := spec.RawGetString("block")
	if blockFn.IsNil() {
		blockFn = evalFn
	}
	if !blockFn.IsFunction() {
		return nil, dialectHandler{}, fmt.Errorf("dialect.register spec block must be a function")
	}
	names, err := dialectRegisterNames(name, spec)
	if err != nil {
		return nil, dialectHandler{}, err
	}
	return names, dialectScriptHandler(call, evalFn, blockFn), nil
}

func dialectRegisterNames(name Value, opts *Table) ([]string, error) {
	names := make([]string, 0, 4)
	if name.IsString() {
		names = append(names, name.Str())
	}
	if opts != nil {
		if aliases := opts.RawGetString("aliases"); aliases.IsTable() {
			tbl := aliases.Table()
			for i := 1; i <= tbl.Length(); i++ {
				alias := tbl.RawGetInt(int64(i))
				if !alias.IsString() {
					return nil, fmt.Errorf("dialect.register alias %d must be a string", i)
				}
				names = append(names, alias.Str())
			}
		}
	}
	for _, name := range names {
		if !validDialectName(name) {
			return nil, fmt.Errorf("dialect.register invalid dialect name %q", name)
		}
	}
	return names, nil
}

func dialectScriptHandler(call ScriptFunctionCaller, evalFn, blockFn Value) dialectHandler {
	callScript := func(fn Value, body Value, opts *Table) ([]Value, error) {
		optValue := NilValue()
		if opts != nil {
			optValue = TableValue(opts)
		}
		return call(fn, []Value{body, optValue})
	}
	return dialectHandler{
		eval: func(body Value, opts *Table) ([]Value, error) {
			return callScript(evalFn, body, opts)
		},
		block: func(body Value, opts *Table) ([]Value, error) {
			return callScript(blockFn, body, opts)
		},
	}
}

func validDialectName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if r == '_' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || (i > 0 && '0' <= r && r <= '9') {
			continue
		}
		return false
	}
	return true
}
