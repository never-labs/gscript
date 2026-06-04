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
// kv`...`, env`...`, jsonl`...`, html_escape`...`, urlquery`...`, mime`...`,
// urlpath`...`, base64`...`, and hash`...`.
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
	if handler.eval == nil && handler.block == nil {
		panic("dialect registry: handler requires eval or block")
	}
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name == "" {
			panic("dialect registry: empty dialect name")
		}
		if _, exists := seen[name]; exists {
			panic(fmt.Sprintf("dialect registry: duplicate dialect %q", name))
		}
		seen[name] = struct{}{}
		if _, exists := r.handlers[name]; exists {
			panic(fmt.Sprintf("dialect registry: duplicate dialect %q", name))
		}
	}
	for _, name := range names {
		r.handlers[name] = handler
		r.names = append(r.names, name)
	}
	sort.Strings(r.names)
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
