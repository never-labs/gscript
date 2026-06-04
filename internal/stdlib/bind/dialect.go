package bind

import (
	"fmt"
)

// BuildDialect creates the "dialect" standard library table. Dialects are a
// small native dispatch layer used by tagged literals and tagged blocks:
// sh`...`, cmd`...`, glob`...`, json`...`, prompt`...`, quote { ... }, and
// small safe data transforms such as path`...`, url`...`, words`...`, nums`...`,
// kv`...`, env`...`, jsonl`...`, html_escape`...`, urlquery`...`, mime`...`,
// base64`...`, and hash`...`.
func BuildDialect(opts HostOptions, maxHostResult func() int64) *Table {
	t := markStdlibBoundModule(NewTable())

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "dialect." + name, Fn: fn}))
	}

	handlers := make(map[string]dialectHandler)
	register := func(names []string, handler dialectHandler) {
		for _, name := range names {
			handlers[name] = handler
		}
	}

	registerDialectShellFS(register, opts, maxHostResult)
	registerDialectText(register, maxHostResult)
	registerDialectWeb(register)
	registerDialectData(register, maxHostResult)
	registerDialectAI(register)

	eval := func(tag string, body Value, options *Table) ([]Value, error) {
		handler, ok := handlers[tag]
		if !ok || handler.eval == nil {
			return nil, fmt.Errorf("unknown dialect %q", tag)
		}
		return handler.eval(body, options)
	}

	set("eval", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() {
			return nil, fmt.Errorf("bad arguments to 'dialect.eval' (tag and body expected)")
		}
		return eval(args[0].Str(), args[1], optionalTableArg(args, 2))
	})

	set("eval_block", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() {
			return nil, fmt.Errorf("bad arguments to 'dialect.eval_block' (tag and config expected)")
		}
		tag := args[0].Str()
		optsTbl := optionalTableArg(args, 2)
		if handler, ok := handlers[tag]; ok && handler.block != nil {
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

func dialectFailFast(opts *Table) bool {
	return opts != nil && opts.RawGetString("fail_fast").Truthy()
}

func optionalTableArg(args []Value, idx int) *Table {
	if len(args) > idx && args[idx].IsTable() {
		return args[idx].Table()
	}
	return nil
}
