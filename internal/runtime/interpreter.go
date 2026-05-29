package runtime

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
)

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
	moduleLoading     bool             // require() may load .gs files from the filesystem
	filesystemEnabled bool             // script-side file APIs may access the filesystem
	filesystemRead    bool             // fs read operations are enabled
	filesystemWrite   bool             // fs write operations are enabled
	filesystemRoot    string           // optional root for script-side filesystem access
	dynamicEval       bool             // script-side string compile/eval is enabled
	environmentRead   bool             // script-side environment reads are enabled
	environmentWrite  bool             // script-side environment writes are enabled
	allowedEnv        map[string]bool  // nil means all environment variables are allowed
	networkAccess     bool             // net/http host network APIs are enabled
	processExecution  bool             // process.run/exec/which are enabled
	processShell      bool             // process.shell is enabled
	debugAccess       bool             // script-side debug APIs are enabled
	testkitAccess     bool             // script-side testkit APIs are enabled
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
	maxSteps          int64            // <=0 means unlimited
	steps             int64
	maxNativeCalls    int64 // <=0 means unlimited
	nativeCalls       int64
	maxCallDepth      int64 // <=0 means unlimited
	maxGoroutines     int64 // <=0 means unlimited
	activeGoroutines  *atomic.Int64
	maxChannelCap     int64 // <=0 means unlimited
	maxHostResult     int64 // <=0 means unlimited
	maxModuleBytes    int64 // <=0 means unlimited
	maxModuleDepth    int64 // <=0 means unlimited
	maxFSReadBytes    int64 // <=0 means unlimited
	maxFSWriteBytes   int64 // <=0 means unlimited
	moduleDepth       int64
	ctx               context.Context
}

// New creates a new Interpreter with built-in globals.
func New() *Interpreter {
	interp := &Interpreter{
		globals:           NewEnvironment(nil),
		modules:           make(map[string]Value),
		gcMode:            "incremental",
		gcRunning:         true,
		moduleLoading:     true,
		filesystemEnabled: true,
		filesystemRead:    true,
		filesystemWrite:   true,
		dynamicEval:       true,
		environmentRead:   true,
		environmentWrite:  true,
		networkAccess:     true,
		processExecution:  true,
		processShell:      true,
		debugAccess:       true,
		testkitAccess:     true,
		activeGoroutines:  &atomic.Int64{},
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

// ReplaceGlobals replaces the global binding set with values from globals.
// It is intended for host-managed rollback/snapshot operations.
func (interp *Interpreter) ReplaceGlobals(globals map[string]Value) {
	env := NewEnvironment(nil)
	for name, val := range globals {
		env.Define(name, val)
	}
	interp.globals = env
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

// AssignGlobal updates an existing global binding in place when possible, or
// defines it when absent. This preserves upvalue identity for closures that
// already captured the global binding.
func (interp *Interpreter) AssignGlobal(name string, val Value) {
	if interp.globals.Set(name, val) {
		return
	}
	interp.globals.Define(name, val)
}

// SetModule registers a prebuilt module value for require(name). Registered
// modules are available even when filesystem-backed module loading is disabled.
func (interp *Interpreter) SetModule(name string, val Value) {
	interp.modules[name] = val
	interp.markPackageLoaded(name, val)
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

// SetModuleLoading controls whether require() may load .gs files from the
// filesystem. Requiring already-registered stdlib modules remains controlled by
// RestrictStdlib.
func (interp *Interpreter) SetModuleLoading(enabled bool) {
	interp.moduleLoading = enabled
}

// SetDynamicEval controls script-side string compilation APIs such as load,
// loadstring, script.compile, and script.eval. Host-side Compile/Exec calls are
// unaffected.
func (interp *Interpreter) SetDynamicEval(enabled bool) {
	interp.dynamicEval = enabled
}

// SetNetworkAccess controls host-backed network APIs in net and http.
func (interp *Interpreter) SetNetworkAccess(enabled bool) {
	interp.networkAccess = enabled
}

// SetDebugAccess controls script-side debug APIs. Internal debug frame
// accounting remains active for host diagnostics.
func (interp *Interpreter) SetDebugAccess(enabled bool) {
	interp.debugAccess = enabled
}

// SetTestkitAccess controls script-side testkit diagnostics.
func (interp *Interpreter) SetTestkitAccess(enabled bool) {
	interp.testkitAccess = enabled
}

// SetProcessExecution controls process.run, process.exec, and process.which.
// process.shell has a separate switch because shell strings carry extra risk.
func (interp *Interpreter) SetProcessExecution(enabled bool) {
	interp.processExecution = enabled
}

// SetProcessShell controls process.shell.
func (interp *Interpreter) SetProcessShell(enabled bool) {
	interp.processShell = enabled
}

// SetFilesystemEnabled controls script-side filesystem APIs such as fs,
// dofile, and loadfile.
func (interp *Interpreter) SetFilesystemEnabled(enabled bool) {
	interp.filesystemEnabled = enabled
	interp.filesystemRead = enabled
	interp.filesystemWrite = enabled
	if enabled {
		interp.refreshFSLib()
		interp.refreshOSLib()
		return
	}
	interp.globals.Delete("fs")
	delete(interp.modules, "fs")
	interp.markPackageLoaded("fs", NilValue())
	interp.globals.Delete("dofile")
	interp.globals.Delete("loadfile")
	interp.refreshOSLib()
}

// SetFilesystemRoot confines script-side file paths to root when root is not
// empty. It refreshes fs in place so existing package.loaded.fs references see
// the same policy.
func (interp *Interpreter) SetFilesystemRoot(root string) {
	interp.filesystemRoot = root
	interp.refreshFSLib()
	interp.refreshOSLib()
}

func (interp *Interpreter) refreshFSLib() {
	if !interp.filesystemEnabled {
		return
	}
	if v, ok := interp.globals.Get("fs"); ok && v.IsTable() {
		fs := TableValue(buildFSLibWithCapabilities(interp.filesystemRoot, interp.filesystemRead, interp.filesystemWrite, interp.maxFSReadBytes, interp.maxFSWriteBytes))
		interp.globals.Define("fs", fs)
		interp.modules["fs"] = fs
		interp.markPackageLoaded("fs", fs)
	}
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

// SetMaxSteps sets the maximum number of interpreter statement checkpoints.
// A non-positive value disables the limit. The counter resets for each Exec.
func (interp *Interpreter) SetMaxSteps(max int64) {
	interp.maxSteps = max
	interp.steps = 0
}

// SetMaxNativeCalls sets the maximum number of native Go calls made by one
// Exec/Run or host Call. A non-positive value disables the limit.
func (interp *Interpreter) SetMaxNativeCalls(max int64) {
	interp.maxNativeCalls = max
	interp.nativeCalls = 0
}

// SetMaxCallDepth sets the maximum number of active function calls. A
// non-positive value disables the limit.
func (interp *Interpreter) SetMaxCallDepth(max int64) {
	interp.maxCallDepth = max
}

// SetMaxGoroutines sets the maximum number of active script-created
// goroutines. A non-positive value disables the limit.
func (interp *Interpreter) SetMaxGoroutines(max int64) {
	interp.maxGoroutines = max
}

// SetMaxChannelCapacity sets the maximum buffer capacity for script-created
// channels. A non-positive value disables the limit.
func (interp *Interpreter) SetMaxChannelCapacity(max int64) {
	interp.maxChannelCap = max
}

// SetMaxHostResultBytes sets the maximum byte size of strings returned from a
// single native Go call. A non-positive value disables the limit.
func (interp *Interpreter) SetMaxHostResultBytes(max int64) {
	interp.maxHostResult = max
}

// SetMaxModuleBytes limits bytes read by script-side file loading APIs such as
// require, dofile, and loadfile. A non-positive value disables the limit.
func (interp *Interpreter) SetMaxModuleBytes(max int64) {
	interp.maxModuleBytes = max
}

// SetMaxModuleDepth limits nested filesystem-backed require calls. Built-in
// and pre-registered modules do not consume this budget.
func (interp *Interpreter) SetMaxModuleDepth(max int64) {
	interp.maxModuleDepth = max
}

// SetMaxFilesystemReadBytes limits bytes read into memory by fs.readfile and
// fs.copy. A non-positive value disables the limit.
func (interp *Interpreter) SetMaxFilesystemReadBytes(max int64) {
	interp.maxFSReadBytes = max
	interp.refreshFSLib()
}

// SetMaxFilesystemWriteBytes limits bytes written by fs.writefile,
// fs.appendfile, and fs.copy. A non-positive value disables the limit.
func (interp *Interpreter) SetMaxFilesystemWriteBytes(max int64) {
	interp.maxFSWriteBytes = max
	interp.refreshFSLib()
}

// SetContext installs a host cancellation context checked at interpreter
// statement/loop checkpoints. A nil context disables cancellation polling.
func (interp *Interpreter) SetContext(ctx context.Context) {
	interp.ctx = ctx
}

func (interp *Interpreter) resetExecutionBudgets() {
	interp.steps = 0
	interp.nativeCalls = 0
}

func (interp *Interpreter) checkStepBudget() error {
	if interp.ctx != nil {
		select {
		case <-interp.ctx.Done():
			return interp.ctx.Err()
		default:
		}
	}
	if interp.maxSteps > 0 {
		interp.steps++
		if interp.steps > interp.maxSteps {
			return fmt.Errorf("execution step limit exceeded (%d)", interp.maxSteps)
		}
	}
	return nil
}

func (interp *Interpreter) checkNativeCallBudget() error {
	if interp.maxNativeCalls <= 0 {
		return nil
	}
	interp.nativeCalls++
	if interp.nativeCalls > interp.maxNativeCalls {
		return fmt.Errorf("native call limit exceeded (%d)", interp.maxNativeCalls)
	}
	return nil
}

func (interp *Interpreter) checkCallDepthBudget() error {
	if interp.maxCallDepth <= 0 {
		return nil
	}
	if int64(len(interp.callStack)) >= interp.maxCallDepth {
		return fmt.Errorf("call depth limit exceeded (%d)", interp.maxCallDepth)
	}
	return nil
}

func (interp *Interpreter) reserveGoroutineBudget() error {
	if interp.maxGoroutines <= 0 {
		return nil
	}
	if interp.activeGoroutines == nil {
		interp.activeGoroutines = &atomic.Int64{}
	}
	for {
		current := interp.activeGoroutines.Load()
		if current >= interp.maxGoroutines {
			return fmt.Errorf("goroutine limit exceeded (%d)", interp.maxGoroutines)
		}
		if interp.activeGoroutines.CompareAndSwap(current, current+1) {
			return nil
		}
	}
}

func (interp *Interpreter) releaseGoroutineBudget() {
	if interp.maxGoroutines <= 0 || interp.activeGoroutines == nil {
		return
	}
	interp.activeGoroutines.Add(-1)
}

func (interp *Interpreter) checkChannelCapacityBudget(capacity int) error {
	if interp.maxChannelCap <= 0 || int64(capacity) <= interp.maxChannelCap {
		return nil
	}
	return fmt.Errorf("channel capacity limit exceeded (%d)", interp.maxChannelCap)
}

func (interp *Interpreter) checkHostResultBudget(values []Value) error {
	return CheckHostResultBytes(interp.maxHostResult, values...)
}

func (interp *Interpreter) checkModuleFileBudget(filename string) error {
	if interp.maxModuleBytes <= 0 {
		return nil
	}
	info, err := os.Stat(filename)
	if err != nil {
		return err
	}
	if info.Size() > interp.maxModuleBytes {
		return fmt.Errorf("module byte limit exceeded (%d)", interp.maxModuleBytes)
	}
	return nil
}

func (interp *Interpreter) enterModuleLoad() error {
	if interp.maxModuleDepth <= 0 {
		interp.moduleDepth++
		return nil
	}
	if interp.moduleDepth >= interp.maxModuleDepth {
		return fmt.Errorf("module depth limit exceeded (%d)", interp.maxModuleDepth)
	}
	interp.moduleDepth++
	return nil
}

func (interp *Interpreter) leaveModuleLoad() {
	if interp.moduleDepth > 0 {
		interp.moduleDepth--
	}
}
