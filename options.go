package gscript

import (
	"github.com/never-labs/gscript/internal/support/modresolve"
	"github.com/never-labs/gscript/llm"
)

// LibFlags controls which standard libraries are loaded.
type LibFlags uint64

const (
	LibString    LibFlags = 1 << iota // string.*
	LibTable                          // table.*
	LibMath                           // math.*
	LibIO                             // io.*
	LibOS                             // os.*
	LibCoroutine                      // coroutine (built-in, always available)
	LibHTTP                           // http.* (server)
	LibRL                             // rl.* (raylib)
	LibJSON                           // json.*
	LibBase64                         // base64.*
	LibHash                           // hash.*
	LibFS                             // fs.*
	LibPath                           // path.*
	LibTime                           // time.*
	LibNet                            // net.* (HTTP client)
	LibVec                            // vec.* (2D/3D vectors)
	LibColor                          // color.*
	LibRegexp                         // regexp.*
	LibUTF8                           // utf8.*
	LibBit32                          // bit32.*
	LibBinary                         // binary.*
	LibBits                           // bits.*
	LibBytes                          // bytes.*
	LibCSV                            // csv.*
	LibURL                            // url.*
	LibUUID                           // uuid.*
	LibProcess                        // process.*
	LibScript                         // script.*
	LibDebug                          // debug.*
	LibTestkit                        // testkit.*
	LibMatrix                         // matrix.*
	LibRand                           // rand.*
	LibSort                           // sort.*
	LibEncoding                       // encoding.*
	LibCompress                       // compress.*
	LibCrypto                         // crypto.*
	LibContainer                      // container.*
	LibLog                            // log.*
	LibArray                          // array.* dense arrays
	LibSoA                            // soa.* structure-of-arrays
	LibLLM                            // llm.* native model/tool integration

	// LibAll includes every library (default).
	LibAll = LibString | LibTable | LibMath | LibIO | LibOS | LibCoroutine |
		LibHTTP | LibRL | LibJSON | LibBase64 | LibHash |
		LibFS | LibPath | LibTime | LibNet |
		LibVec | LibColor | LibRegexp | LibUTF8 | LibBit32 |
		LibBinary | LibBits | LibBytes | LibCSV | LibURL | LibUUID |
		LibProcess | LibScript | LibDebug | LibTestkit | LibMatrix |
		LibRand | LibSort | LibEncoding | LibCompress | LibCrypto |
		LibContainer | LibLog | LibArray | LibSoA | LibLLM

	// LibSafe is a sandboxed subset with no I/O, network, or system access.
	LibSafe = LibString | LibTable | LibMath | LibCoroutine |
		LibJSON | LibBase64 | LibHash | LibVec | LibColor |
		LibRegexp | LibUTF8 | LibBit32 | LibBinary | LibBits |
		LibBytes | LibCSV | LibURL | LibUUID | LibMatrix |
		LibPath | LibTime | LibRand | LibSort | LibEncoding | LibCompress | LibCrypto |
		LibContainer | LibArray | LibSoA

	// LibApp is a convenient preset for application development (no GL).
	LibApp = LibString | LibTable | LibMath | LibIO | LibOS | LibCoroutine |
		LibJSON | LibBase64 | LibHash | LibFS | LibPath | LibTime | LibNet |
		LibRegexp | LibUTF8 | LibBit32 | LibBinary | LibBits |
		LibBytes | LibCSV | LibURL | LibUUID | LibProcess | LibScript |
		LibDebug | LibMatrix | LibRand | LibSort | LibEncoding |
		LibCompress | LibCrypto | LibContainer | LibLog | LibArray | LibSoA |
		LibLLM

	// LibGame is a preset for game development (no I/O, includes rl/vec/color).
	LibGame = LibString | LibTable | LibMath | LibCoroutine |
		LibRL | LibVec | LibColor | LibJSON | LibBit32 | LibBits |
		LibTime | LibRand | LibArray | LibSoA
)

// CapabilityFlags controls host capabilities that are separate from selecting
// which standard-library tables exist.
type CapabilityFlags uint64

const (
	CapModuleLoading    CapabilityFlags = 1 << iota // require() may load .gs files from the host filesystem
	CapFilesystemRead                               // script file APIs may read the host filesystem
	CapFilesystemWrite                              // script file APIs may mutate the host filesystem
	CapEnvironmentRead                              // script OS APIs may read host environment variables
	CapEnvironmentWrite                             // script OS APIs may mutate host environment variables

	// CapFilesystem enables both filesystem read and write access.
	CapFilesystem = CapFilesystemRead | CapFilesystemWrite

	// CapEnvironment enables both environment read and write access.
	CapEnvironment = CapEnvironmentRead | CapEnvironmentWrite

	// CapAll enables every host capability (default, for compatibility).
	CapAll = CapModuleLoading | CapFilesystem | CapEnvironment

	// CapSafe disables host-backed capabilities.
	CapSafe CapabilityFlags = 0
)

type vmOptions struct {
	libs               LibFlags
	capabilities       CapabilityFlags
	requirePath        string
	moduleCollections  map[string]string
	moduleReplaces     map[string]string
	moduleCacheDir     string
	moduleCacheModules []modresolve.CacheModule
	filesystemRoot     string
	dynamicEval        bool
	environmentVars    []string
	networkAccess      bool
	debugAccess        bool
	testkitAccess      bool
	processExec        bool
	processShell       bool
	maxSteps           int64
	maxNativeCalls     int64
	maxCallDepth       int64
	maxGoroutines      int64
	maxChannelCap      int64
	maxHostResult      int64
	maxModuleBytes     int64
	maxModuleDepth     int64
	maxFSReadBytes     int64
	maxFSWriteBytes    int64
	printFunc          func(args ...interface{})
	llmProvider        llm.Provider
	llmProviderFactory llm.ProviderFactory
	llmTraceSink       llm.TraceSink
	llmRecordSink      llm.RecordSink
	goImports          map[string]any
	argScript          string
	args               []string
	argsSet            bool
	useVM              bool // use bytecode VM instead of tree-walker
	useJIT             bool // enable JIT compilation (implies useVM)
}

// SecurityPolicy groups production sandbox controls behind one auditable
// embedding option. Zero-valued fields keep existing defaults.
type SecurityPolicy struct {
	Libs                    LibFlags
	Capabilities            CapabilityFlags
	MaxSteps                int64
	MaxNativeCalls          int64
	MaxCallDepth            int64
	MaxGoroutines           int64
	MaxChannelCapacity      int64
	MaxHostResultBytes      int64
	MaxModuleBytes          int64
	MaxModuleDepth          int64
	MaxFilesystemReadBytes  int64
	MaxFilesystemWriteBytes int64
	EnvironmentAllowlist    []string
	DisableDynamicEval      bool
	DisableNetworkAccess    bool
	DisableDebugAccess      bool
	DisableTestkitAccess    bool
	DisableProcessExecution bool
	DisableProcessShell     bool
	DisableJIT              bool
	DisableModuleLoading    bool
}

// Option configures a VM instance.
type Option func(*vmOptions)

// WithArgs sets the script entrypoint arguments. The global arg table follows
// GScript's Lua-compatible convention: arg[0] is script and arg[1..n] are args.
func WithArgs(script string, args ...string) Option {
	return func(o *vmOptions) {
		o.argScript = script
		o.args = append([]string(nil), args...)
		o.argsSet = true
	}
}

// WithLibs sets which standard libraries are available.
// Default: LibAll
func WithLibs(libs LibFlags) Option {
	return func(o *vmOptions) { o.libs = libs }
}

// WithCapabilities sets which host capabilities are available to scripts.
// Default: CapAll. Use CapSafe with LibSafe for an in-process sandbox that has
// no script module loading and no filesystem-backed script APIs.
func WithCapabilities(caps CapabilityFlags) Option {
	return func(o *vmOptions) { o.capabilities = caps }
}

// WithModuleLoading controls whether require() may load .gs files from the
// host filesystem. Requiring enabled built-in standard libraries is still
// allowed, because that is controlled by WithLibs.
func WithModuleLoading(enabled bool) Option {
	return func(o *vmOptions) {
		if enabled {
			o.capabilities |= CapModuleLoading
		} else {
			o.capabilities &^= CapModuleLoading
		}
	}
}

// WithFilesystem controls filesystem-backed script APIs such as fs, dofile,
// and loadfile. It enables or disables both filesystem read and write access.
// It does not affect host-side ExecFile/CompileFile calls.
func WithFilesystem(enabled bool) Option {
	return func(o *vmOptions) {
		if enabled {
			o.capabilities |= CapFilesystem
		} else {
			o.capabilities &^= CapFilesystem
		}
	}
}

// WithFilesystemRead controls script-side filesystem read access. This gates
// APIs such as fs.readfile, fs.stat, fs.readdir, dofile, and loadfile.
func WithFilesystemRead(enabled bool) Option {
	return func(o *vmOptions) {
		if enabled {
			o.capabilities |= CapFilesystemRead
		} else {
			o.capabilities &^= CapFilesystemRead
		}
	}
}

// WithFilesystemWrite controls script-side filesystem write access. This gates
// APIs such as fs.writefile, fs.remove, fs.rename, fs.mkdir, fs.chdir, and
// fs.tempfile.
func WithFilesystemWrite(enabled bool) Option {
	return func(o *vmOptions) {
		if enabled {
			o.capabilities |= CapFilesystemWrite
		} else {
			o.capabilities &^= CapFilesystemWrite
		}
	}
}

// WithEnvironment controls script-side host environment variable APIs. It
// enables or disables both read and write access.
func WithEnvironment(enabled bool) Option {
	return func(o *vmOptions) {
		if enabled {
			o.capabilities |= CapEnvironment
		} else {
			o.capabilities &^= CapEnvironment
		}
	}
}

// WithEnvironmentRead controls script-side environment variable reads. This
// gates APIs such as os.getenv, os.environ, and os.expand.
func WithEnvironmentRead(enabled bool) Option {
	return func(o *vmOptions) {
		if enabled {
			o.capabilities |= CapEnvironmentRead
		} else {
			o.capabilities &^= CapEnvironmentRead
		}
	}
}

// WithEnvironmentWrite controls script-side environment variable writes. This
// gates APIs such as os.setenv and os.unsetenv.
func WithEnvironmentWrite(enabled bool) Option {
	return func(o *vmOptions) {
		if enabled {
			o.capabilities |= CapEnvironmentWrite
		} else {
			o.capabilities &^= CapEnvironmentWrite
		}
	}
}

// WithEnvironmentAllowlist restricts script-side environment APIs to the named
// variables. Passing no names allows no variables; omit this option to keep the
// default unrestricted behavior when environment access is otherwise enabled.
func WithEnvironmentAllowlist(names ...string) Option {
	return func(o *vmOptions) { o.environmentVars = append([]string(nil), names...) }
}

// WithProcessExecution controls process.run(), process.exec(), and
// process.which(). process.shell() has a separate switch.
func WithProcessExecution(enabled bool) Option {
	return func(o *vmOptions) { o.processExec = enabled }
}

// WithProcessShell controls process.shell().
func WithProcessShell(enabled bool) Option {
	return func(o *vmOptions) { o.processShell = enabled }
}

// WithFilesystemRoot confines fs module paths and script-side file loading to
// root. Relative script paths are resolved inside root. An empty root leaves
// filesystem paths unrestricted.
func WithFilesystemRoot(root string) Option {
	return func(o *vmOptions) {
		o.filesystemRoot = root
		o.capabilities |= CapFilesystem
	}
}

// WithDynamicEval controls script-side string compilation APIs such as load(),
// loadstring(), script.compile(), and script.eval(). It does not affect
// host-side Compile/Exec calls.
func WithDynamicEval(enabled bool) Option {
	return func(o *vmOptions) { o.dynamicEval = enabled }
}

// WithNetworkAccess controls host-backed network APIs in net and http.
func WithNetworkAccess(enabled bool) Option {
	return func(o *vmOptions) { o.networkAccess = enabled }
}

// WithDebugAccess controls script-side debug APIs. Internal debug frame
// accounting remains active for host diagnostics.
func WithDebugAccess(enabled bool) Option {
	return func(o *vmOptions) { o.debugAccess = enabled }
}

// WithTestkitAccess controls script-side testkit diagnostics.
func WithTestkitAccess(enabled bool) Option {
	return func(o *vmOptions) { o.testkitAccess = enabled }
}

// WithSandbox selects the safe standard library set and disables host
// filesystem-backed capabilities.
func WithSandbox() Option {
	return func(o *vmOptions) {
		o.libs = LibSafe
		o.capabilities = CapSafe
		o.dynamicEval = false
		o.networkAccess = false
		o.debugAccess = false
		o.testkitAccess = false
		o.processExec = false
		o.processShell = false
	}
}

// SecuritySandbox selects the production-oriented in-process sandbox baseline:
// safe standard libraries, no host-backed capabilities, and no JIT by default.
// Pair it with context deadlines and WithMaxSteps for concrete resource
// budgets.
func SecuritySandbox() Option {
	return func(o *vmOptions) {
		o.libs = LibSafe
		o.capabilities = CapSafe
		o.dynamicEval = false
		o.networkAccess = false
		o.debugAccess = false
		o.testkitAccess = false
		o.processExec = false
		o.processShell = false
		o.useJIT = false
	}
}

// WithSecurity applies a grouped security policy. It is equivalent to applying
// the corresponding fine-grained options, but gives embedders one place to
// construct and audit production limits.
func WithSecurity(policy SecurityPolicy) Option {
	return func(o *vmOptions) {
		if policy.Libs != 0 {
			o.libs = policy.Libs
		}
		if policy.Capabilities != 0 || policy.DisableModuleLoading {
			o.capabilities = policy.Capabilities
		}
		if policy.DisableModuleLoading {
			o.capabilities &^= CapModuleLoading
		}
		if policy.MaxSteps > 0 {
			o.maxSteps = policy.MaxSteps
		}
		if policy.MaxNativeCalls > 0 {
			o.maxNativeCalls = policy.MaxNativeCalls
		}
		if policy.MaxCallDepth > 0 {
			o.maxCallDepth = policy.MaxCallDepth
		}
		if policy.MaxGoroutines > 0 {
			o.maxGoroutines = policy.MaxGoroutines
		}
		if policy.MaxChannelCapacity > 0 {
			o.maxChannelCap = policy.MaxChannelCapacity
		}
		if policy.MaxHostResultBytes > 0 {
			o.maxHostResult = policy.MaxHostResultBytes
		}
		if policy.MaxModuleBytes > 0 {
			o.maxModuleBytes = policy.MaxModuleBytes
		}
		if policy.MaxModuleDepth > 0 {
			o.maxModuleDepth = policy.MaxModuleDepth
		}
		if policy.MaxFilesystemReadBytes > 0 {
			o.maxFSReadBytes = policy.MaxFilesystemReadBytes
		}
		if policy.MaxFilesystemWriteBytes > 0 {
			o.maxFSWriteBytes = policy.MaxFilesystemWriteBytes
		}
		if policy.EnvironmentAllowlist != nil {
			o.environmentVars = append([]string(nil), policy.EnvironmentAllowlist...)
		}
		if policy.DisableDynamicEval {
			o.dynamicEval = false
		}
		if policy.DisableNetworkAccess {
			o.networkAccess = false
		}
		if policy.DisableDebugAccess {
			o.debugAccess = false
		}
		if policy.DisableTestkitAccess {
			o.testkitAccess = false
		}
		if policy.DisableProcessExecution {
			o.processExec = false
		}
		if policy.DisableProcessShell {
			o.processShell = false
		}
		if policy.DisableJIT {
			o.useJIT = false
		}
	}
}

// WithRequirePath sets the base directory for require() module loading.
func WithRequirePath(path string) Option {
	return func(o *vmOptions) { o.requirePath = path }
}

// WithModuleCollection maps a require collection prefix to a filesystem root.
// require("vendor:pkg.util") resolves as ROOT/pkg/util.gs. This mirrors Odin's
// collection-style package roots while keeping GScript module loading explicit.
func WithModuleCollection(name, root string) Option {
	return func(o *vmOptions) {
		if name == "" {
			return
		}
		if o.moduleCollections == nil {
			o.moduleCollections = make(map[string]string)
		}
		o.moduleCollections[name] = root
	}
}

// WithModuleReplace maps a module path prefix to a local filesystem root.
// require("example.com/lib/foo") resolves as ROOT/foo.gs when the prefix is
// example.com/lib. This is the runtime side of gscript.mod replace directives.
func WithModuleReplace(path, root string) Option {
	return func(o *vmOptions) {
		if path == "" || root == "" {
			return
		}
		if o.moduleReplaces == nil {
			o.moduleReplaces = make(map[string]string)
		}
		o.moduleReplaces[path] = root
	}
}

// WithModuleCache enables read-only module resolution from a module cache
// populated by gscript mod download. It never downloads modules at runtime.
func WithModuleCache(root string) Option {
	return func(o *vmOptions) {
		o.moduleCacheDir = root
	}
}

func withModuleCacheModules(modules []modresolve.CacheModule) Option {
	return func(o *vmOptions) {
		o.moduleCacheModules = append([]modresolve.CacheModule(nil), modules...)
	}
}

// WithGoImports registers an explicit allowlist of Go-backed modules that
// scripts may load with require("go:..."). The values are converted with the
// same rules as ModuleFrom, so pass concrete modules or host values; this does
// not dynamically load Go packages by import path.
func WithGoImports(imports map[string]any) Option {
	return func(o *vmOptions) {
		if imports == nil {
			o.goImports = nil
			return
		}
		o.goImports = make(map[string]any, len(imports))
		for name, source := range imports {
			o.goImports[name] = source
		}
	}
}

// WithPrint overrides the print() function (useful to capture output in tests/games).
func WithPrint(fn func(args ...interface{})) Option {
	return func(o *vmOptions) { o.printFunc = fn }
}

// WithMaxSteps limits interpreter statements or bytecode instructions executed
// by one Exec/Run. A non-positive value disables the limit.
//
// When a step limit is set, JIT execution is disabled so native code cannot
// bypass the budget checkpoints.
func WithMaxSteps(max int64) Option {
	return func(o *vmOptions) { o.maxSteps = max }
}

// WithMaxNativeCalls limits calls from script into native Go functions,
// including standard-library functions and registered host callbacks. A
// non-positive value disables the limit.
//
// When a native-call limit is set, JIT execution is disabled so native code
// cannot bypass host-call budget checkpoints.
func WithMaxNativeCalls(max int64) Option {
	return func(o *vmOptions) { o.maxNativeCalls = max }
}

// WithMaxCallDepth limits active script/native function call depth. A
// non-positive value uses the runtime default.
//
// When a call-depth limit is set, JIT execution is disabled so native code
// cannot bypass frame-depth checkpoints.
func WithMaxCallDepth(max int64) Option {
	return func(o *vmOptions) { o.maxCallDepth = max }
}

// WithMaxGoroutines limits active goroutines started by script go statements.
// A non-positive value disables the limit.
//
// When a goroutine limit is set, JIT execution is disabled so native code
// cannot bypass task-creation checkpoints.
func WithMaxGoroutines(max int64) Option {
	return func(o *vmOptions) { o.maxGoroutines = max }
}

// WithMaxChannelCapacity limits the buffer capacity accepted by make(chan, n).
// Unbuffered channels are still allowed. A non-positive value disables the
// limit.
//
// When a channel-capacity limit is set, JIT execution is disabled so native
// code cannot bypass channel-creation checkpoints.
func WithMaxChannelCapacity(max int64) Option {
	return func(o *vmOptions) { o.maxChannelCap = max }
}

// WithMaxHostResultBytes limits string bytes returned from a single native Go
// call, including standard-library functions and registered host callbacks. A
// non-positive value disables the limit.
//
// When a host-result limit is set, JIT execution is disabled so native code
// cannot bypass result materialization checks.
func WithMaxHostResultBytes(max int64) Option {
	return func(o *vmOptions) { o.maxHostResult = max }
}

// WithMaxModuleBytes limits bytes read by script-side module/file loading APIs
// such as require(), dofile(), loadfile(), and script.loadFile(). It does not
// limit host-side CompileFile/ExecFile calls. A non-positive value disables the
// limit.
//
// When a module-byte limit is set, JIT execution is disabled so native code
// cannot bypass file-loading checks.
func WithMaxModuleBytes(max int64) Option {
	return func(o *vmOptions) { o.maxModuleBytes = max }
}

// WithMaxModuleDepth limits nested filesystem-backed require() calls.
// Built-in standard-library modules and already loaded package entries do not
// consume this budget. A non-positive value disables the limit.
//
// When a module-depth limit is set, JIT execution is disabled so native code
// cannot bypass file-loading checks.
func WithMaxModuleDepth(max int64) Option {
	return func(o *vmOptions) { o.maxModuleDepth = max }
}

// WithMaxFilesystemReadBytes limits bytes read into memory by fs.readfile()
// and fs.copy(). It does not limit host-side CompileFile/ExecFile calls or
// script source loading, which is controlled by WithMaxModuleBytes.
//
// When a filesystem-read limit is set, JIT execution is disabled so native code
// cannot bypass filesystem checks.
func WithMaxFilesystemReadBytes(max int64) Option {
	return func(o *vmOptions) { o.maxFSReadBytes = max }
}

// WithMaxFilesystemWriteBytes limits bytes written by fs.writefile(),
// fs.appendfile(), and fs.copy().
//
// When a filesystem-write limit is set, JIT execution is disabled so native
// code cannot bypass filesystem checks.
func WithMaxFilesystemWriteBytes(max int64) Option {
	return func(o *vmOptions) { o.maxFSWriteBytes = max }
}

// WithVM enables the bytecode VM instead of the default tree-walking interpreter.
func WithVM() Option {
	return func(o *vmOptions) { o.useVM = true }
}

// WithJIT enables the ARM64 JIT compiler (implies bytecode VM).
// Only available on darwin/arm64 (Apple Silicon).
func WithJIT() Option {
	return func(o *vmOptions) {
		o.useVM = true
		o.useJIT = true
	}
}

// WithTracing is an alias for WithJIT (kept for backward compatibility).
// The JIT compiler now includes both method-level and trace-level compilation.
func WithTracing() Option {
	return WithJIT()
}
