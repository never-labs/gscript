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
// mdtable`...`, markdown`...`, yaml`...`, kv`...`, logfmt`...`, env`...`, jsonl`...`, semver`...`, html_escape`...`, urlquery`...`, mime`...`,
// urlpath`...`, duration`...`, tap`...`, junit`...`, ipaddr`...`, cidr`...`, base64`...`, and hash`...`.
func BuildDialect(opts HostOptions, maxHostResult func() int64, specs ...DialectSpec) *Table {
	t := markStdlibBoundModule(NewTable())

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "dialect." + name, Fn: fn}))
	}

	registry := newDialectRegistry()
	register := registry.register

	registerDialectShellFS(register, opts, maxHostResult)
	registerDialectText(register, maxHostResult)
	registerDialectProtocol(register, maxHostResult)
	registerDialectProtocolNetwork(register)
	registerDialectData(register, maxHostResult)
	registerDialectDatabase(register)
	registerDialectAI(register)
	for _, spec := range specs {
		names, handler, err := dialectSpecHandler(spec)
		if err != nil {
			panic(err)
		}
		register(names, handler)
	}

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

	set("info", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad arguments to 'dialect.info' (tag expected)")
		}
		info, ok := registry.info(args[0].Str())
		if !ok {
			return []Value{NilValue()}, nil
		}
		return []Value{TableValue(info)}, nil
	})

	set("list", func(args []Value) ([]Value, error) {
		out := NewAppendArrayTable(len(registry.names))
		for i, name := range registry.names {
			info, _ := registry.info(name)
			out.RawSetInt(int64(i+1), TableValue(info))
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

func dialectMode(opts *Table) string {
	if opts != nil && opts.RawGetString("mode").IsString() {
		return opts.RawGetString("mode").Str()
	}
	return ""
}

func dialectModeAllowed(mode string, allowed ...string) bool {
	for _, item := range allowed {
		if mode == item {
			return true
		}
	}
	return false
}

func dialectUnknownMode(name, mode string) ([]Value, error) {
	return []Value{NilValue(), StringValue(fmt.Sprintf("%s dialect: unknown mode %q", name, mode))}, nil
}

type dialectHandler struct {
	eval func(Value, *Table) ([]Value, error)

	block func(Value, *Table) ([]Value, error)
	meta  dialectMetadata
}

type dialectMetadata struct {
	Category     string
	Capabilities []string
	Builtin      bool
}

// DialectSpec describes a host-provided dialect registration.
type DialectSpec struct {
	Name         string
	Aliases      []string
	Category     string
	Capabilities []string
	Eval         func(Value, *Table) ([]Value, error)
	Block        func(Value, *Table) ([]Value, error)
}

type dialectRegisterFunc func([]string, dialectHandler)

type dialectRegistry struct {
	handlers map[string]dialectHandler
	aliases  map[string][]string
	names    []string
}

func newDialectRegistry() *dialectRegistry {
	return &dialectRegistry{
		handlers: make(map[string]dialectHandler),
		aliases:  make(map[string][]string),
	}
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
		handler.meta = normalizeDialectMetadata(name, handler.meta)
		r.handlers[name] = handler
		r.aliases[name] = dialectAliasesForName(name, names)
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

func (r *dialectRegistry) info(name string) (*Table, bool) {
	handler, ok := r.handlers[name]
	if !ok {
		return nil, false
	}
	info := NewTable()
	info.RawSetString("name", StringValue(name))
	info.RawSetString("category", StringValue(handler.meta.Category))
	info.RawSetString("builtin", BoolValue(handler.meta.Builtin))
	info.RawSetString("eval", BoolValue(handler.eval != nil))
	info.RawSetString("block", BoolValue(handler.block != nil))
	info.RawSetString("aliases", stringArrayValue(r.aliases[name]))
	info.RawSetString("capabilities", stringArrayValue(handler.meta.Capabilities))
	return info, true
}

func normalizeDialectMetadata(name string, meta dialectMetadata) dialectMetadata {
	if meta.Category == "" {
		meta.Builtin = true
	}
	if meta.Category == "" {
		meta.Category = builtinDialectCategory(name)
	}
	if meta.Capabilities == nil {
		meta.Capabilities = builtinDialectCapabilities(name)
	}
	return meta
}

func dialectAliasesForName(name string, names []string) []string {
	aliases := make([]string, 0, len(names)-1)
	for _, candidate := range names {
		if candidate != name {
			aliases = append(aliases, candidate)
		}
	}
	sort.Strings(aliases)
	return aliases
}

func stringArrayValue(values []string) Value {
	out := NewAppendArrayTable(len(values))
	for i, value := range values {
		out.RawSetInt(int64(i+1), StringValue(value))
	}
	return TableValue(out)
}

func builtinDialectCategory(name string) string {
	switch name {
	case "sh", "cmd", "glob":
		return "host"
	case "shellwords", "path", "json", "jsonptr", "jsonl", "csv", "tsv", "mdtable", "markdown", "md", "lines", "split", "words", "nums", "numbers", "kv", "logfmt", "env", "ini", "yaml", "yml", "semver", "duration", "tap", "xml", "template", "re", "regexp":
		return "text"
	case "url", "html_escape", "urlquery", "urlpath", "mime", "headers", "http_headers", "httpmsg", "cookie", "cookies", "sse", "multipart", "jwt":
		return "protocol"
	case "ipaddr", "cidr", "hostport":
		return "protocol"
	case "base64", "hash", "hex", "base32", "uuid", "gzip", "zlib", "deflate", "binary":
		return "data"
	case "sql":
		return "database"
	case "prompt", "quote":
		return "llm"
	case "junit":
		return "compat"
	default:
		return "user"
	}
}

func builtinDialectCapabilities(name string) []string {
	switch name {
	case "sh":
		return []string{"process.shell"}
	case "cmd":
		return []string{"process.exec"}
	case "glob":
		return []string{"fs.read"}
	default:
		return []string{}
	}
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
	handler := dialectScriptHandler(call, evalFn, blockFn)
	handler.meta = dialectUserMetadata(optionalTableArg(args, 2))
	return names, handler, nil
}

func dialectSpecHandler(spec DialectSpec) ([]string, dialectHandler, error) {
	names := make([]string, 0, 1+len(spec.Aliases))
	names = append(names, spec.Name)
	names = append(names, spec.Aliases...)
	for _, name := range names {
		if !validDialectName(name) {
			return nil, dialectHandler{}, fmt.Errorf("dialect registry: invalid dialect name %q", name)
		}
	}
	handler := dialectHandler{
		eval:  spec.Eval,
		block: spec.Block,
		meta: dialectMetadata{
			Category:     spec.Category,
			Capabilities: append([]string(nil), spec.Capabilities...),
		},
	}
	if handler.block == nil {
		handler.block = handler.eval
	}
	return names, handler, nil
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
	handler := dialectScriptHandler(call, evalFn, blockFn)
	handler.meta = dialectUserMetadata(spec)
	return names, handler, nil
}

func dialectUserMetadata(opts *Table) dialectMetadata {
	meta := dialectMetadata{Category: "user"}
	if opts == nil {
		return meta
	}
	if category := opts.RawGetString("category"); category.IsString() && category.Str() != "" {
		meta.Category = category.Str()
	}
	if capabilities := opts.RawGetString("capabilities"); capabilities.IsTable() {
		meta.Capabilities = stringSliceFromArrayValue(capabilities.Table())
	}
	return meta
}

func stringSliceFromArrayValue(tbl *Table) []string {
	values := make([]string, 0, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		value := tbl.RawGetInt(int64(i))
		if value.IsString() {
			values = append(values, value.Str())
		}
	}
	return values
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
