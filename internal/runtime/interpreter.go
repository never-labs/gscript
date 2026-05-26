package runtime

// Core tree-walking interpreter: the Interpreter type, its constructor New /
// NewInterpreterGlobals, and the global-environment / args / package-cache
// accessors. Builtin registration, metamethod helpers, statement and
// expression evaluation, and numeric parsing live in the sibling
// interpreter_*.go files.
// Interpreter is the tree-walking evaluator for GScript programs.
type Interpreter struct {
	globals           *Environment
	output            []string         // captured print output (for testing)
	currentCo         *Coroutine       // non-nil when running inside a coroutine
	modules           map[string]Value // require() cache
	stringMeta        *Table           // metatable for string values (__index → string lib)
	scriptDir         string           // directory of the main script (for require path resolution)
	currentSourceName string           // source name for diagnostics while executing parsed source
	args              []string         // current script entrypoint args: [0]=script, [1:]=user args
	callStack         []DebugFrame     // active runtime calls, oldest to newest
	deferStack        [][]deferredCall // active function-scope deferred calls
	debugHook         Value            // optional GScript diagnostic hook
	debugOpts         DebugHookOptions // filters for debugHook
	debugSink         Value            // optional explicit diagnostic sink
	debugBusy         bool             // prevents debug hooks from recursively firing
	gcMode            string           // host-facing collectgarbage mode label
	gcRunning         bool             // host-facing collectgarbage running flag
}

// New creates a new Interpreter with built-in globals.
func New() *Interpreter {
	interp := &Interpreter{
		globals:   NewEnvironment(nil),
		modules:   make(map[string]Value),
		gcMode:    "incremental",
		gcRunning: true,
	}
	interp.registerBuiltins()
	interp.registerStdlib()
	return interp
}

// Globals returns the global environment.
func (interp *Interpreter) Globals() *Environment {
	return interp.globals
}

// StringMeta returns the string metatable.
func (interp *Interpreter) StringMeta() *Table {
	return interp.stringMeta
}

// ExportGlobals returns a flat map of all global variables.
// Used by the VM to share stdlib/builtins from the tree-walker.
func (interp *Interpreter) ExportGlobals() map[string]Value {
	m := make(map[string]Value)
	for name, uv := range interp.globals.vars {
		m[name] = uv.Get()
	}
	return m
}

// RestrictStdlib removes standard-library globals not present in allowed.
func (interp *Interpreter) RestrictStdlib(allowed map[string]bool) {
	for _, name := range stdlibModuleNames {
		if allowed[name] {
			continue
		}
		interp.globals.Delete(name)
		delete(interp.modules, name)
		interp.markPackageLoaded(name, NilValue())
		if name == "string" {
			interp.stringMeta = nil
		}
	}
}

// NewInterpreterGlobals creates a fresh globals map with all builtins and stdlib registered.
// This is used by the bytecode VM to get the same standard library as the tree-walker.
func NewInterpreterGlobals() map[string]Value {
	interp := New()
	return interp.ExportGlobals()
}

// SetGlobal defines or overwrites a global variable.
func (interp *Interpreter) SetGlobal(name string, val Value) {
	interp.globals.Define(name, val)
}

// GetGlobal retrieves a global variable.
func (interp *Interpreter) GetGlobal(name string) Value {
	v, _ := interp.globals.Get(name)
	return v
}

// SetScriptDir sets the directory of the main script, used for require path resolution.
func (interp *Interpreter) SetScriptDir(dir string) {
	interp.scriptDir = dir
}

// ScriptDir returns the directory used for relative script/module loading.
func (interp *Interpreter) ScriptDir() string {
	return interp.scriptDir
}

func (interp *Interpreter) builtinModule(name string) (Value, bool) {
	for _, moduleName := range stdlibModuleNames {
		if name != moduleName {
			continue
		}
		v, ok := interp.globals.Get(name)
		return v, ok && v.IsTable()
	}
	return NilValue(), false
}

func (interp *Interpreter) markPackageLoaded(name string, module Value) {
	pkg, ok := interp.globals.Get("package")
	if !ok || !pkg.IsTable() {
		return
	}
	loaded := pkg.Table().RawGetString("loaded")
	if !loaded.IsTable() {
		return
	}
	loaded.Table().RawSetString(name, module)
}

func (interp *Interpreter) packageLoaded(name string) Value {
	pkg, ok := interp.globals.Get("package")
	if !ok || !pkg.IsTable() {
		return NilValue()
	}
	loaded := pkg.Table().RawGetString("loaded")
	if !loaded.IsTable() {
		return NilValue()
	}
	return loaded.Table().RawGetString(name)
}

// SetArgs sets the script entrypoint arguments and updates the global arg table.
// The resulting table follows GScript's Lua-compatible convention:
// arg[0] is the script name, and arg[1..n] are user arguments.
func (interp *Interpreter) SetArgs(script string, args []string) {
	argv := make([]string, 0, len(args)+1)
	argv = append(argv, script)
	argv = append(argv, args...)
	interp.args = argv

	tbl := NewTable()
	tbl.RawSet(IntValue(0), StringValue(script))
	for i, arg := range args {
		tbl.RawSet(IntValue(int64(i+1)), StringValue(arg))
	}
	interp.SetGlobal("arg", TableValue(tbl))
}

// Args returns a copy of the current script entrypoint arguments.
func (interp *Interpreter) Args() []string {
	out := make([]string, len(interp.args))
	copy(out, interp.args)
	return out
}

// Output returns captured print output (for testing).
func (interp *Interpreter) Output() []string {
	return interp.output
}
