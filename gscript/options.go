package gscript

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
	LibGL                             // gl.* (OpenGL)
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

	LibRL = LibGL // compatibility alias for the registered rl.* module

	// LibAll includes every library (default).
	LibAll = LibString | LibTable | LibMath | LibIO | LibOS | LibCoroutine |
		LibHTTP | LibGL | LibJSON | LibBase64 | LibHash |
		LibFS | LibPath | LibTime | LibNet |
		LibVec | LibColor | LibRegexp | LibUTF8 | LibBit32 |
		LibBinary | LibBits | LibBytes | LibCSV | LibURL | LibUUID |
		LibProcess | LibScript | LibDebug | LibTestkit | LibMatrix |
		LibRand | LibSort | LibEncoding | LibCompress | LibCrypto |
		LibContainer | LibLog

	// LibSafe is a sandboxed subset with no I/O, network, or system access.
	LibSafe = LibString | LibTable | LibMath | LibCoroutine |
		LibJSON | LibBase64 | LibHash | LibVec | LibColor |
		LibRegexp | LibUTF8 | LibBit32 | LibBinary | LibBits |
		LibBytes | LibCSV | LibURL | LibUUID | LibMatrix |
		LibRand | LibSort | LibEncoding | LibCompress | LibCrypto |
		LibContainer

	// LibApp is a convenient preset for application development (no GL).
	LibApp = LibString | LibTable | LibMath | LibIO | LibOS | LibCoroutine |
		LibJSON | LibBase64 | LibHash | LibFS | LibPath | LibTime | LibNet |
		LibRegexp | LibUTF8 | LibBit32 | LibBinary | LibBits |
		LibBytes | LibCSV | LibURL | LibUUID | LibProcess | LibScript |
		LibDebug | LibMatrix | LibRand | LibSort | LibEncoding |
		LibCompress | LibCrypto | LibContainer | LibLog

	// LibGame is a preset for game development (no I/O, includes GL/vec/color).
	LibGame = LibString | LibTable | LibMath | LibCoroutine |
		LibGL | LibVec | LibColor | LibJSON | LibBit32 | LibBits |
		LibTime | LibRand
)

// CapabilityFlags controls host capabilities that are separate from selecting
// which standard-library tables exist.
type CapabilityFlags uint64

const (
	CapModuleLoading   CapabilityFlags = 1 << iota // require() may load .gs files from the host filesystem
	CapFilesystemRead                              // script file APIs may read the host filesystem
	CapFilesystemWrite                             // script file APIs may mutate the host filesystem

	// CapFilesystem enables both filesystem read and write access.
	CapFilesystem = CapFilesystemRead | CapFilesystemWrite

	// CapAll enables every host capability (default, for compatibility).
	CapAll = CapModuleLoading | CapFilesystem

	// CapSafe disables host filesystem-backed capabilities.
	CapSafe CapabilityFlags = 0
)

type vmOptions struct {
	libs           LibFlags
	capabilities   CapabilityFlags
	requirePath    string
	filesystemRoot string
	maxSteps       int64
	printFunc      func(args ...interface{})
	useVM          bool // use bytecode VM instead of tree-walker
	useJIT         bool // enable JIT compilation (implies useVM)
}

// Option configures a VM instance.
type Option func(*vmOptions)

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

// WithFilesystemRoot confines fs module paths and script-side file loading to
// root. Relative script paths are resolved inside root. An empty root leaves
// filesystem paths unrestricted.
func WithFilesystemRoot(root string) Option {
	return func(o *vmOptions) {
		o.filesystemRoot = root
		o.capabilities |= CapFilesystem
	}
}

// WithSandbox selects the safe standard library set and disables host
// filesystem-backed capabilities.
func WithSandbox() Option {
	return func(o *vmOptions) {
		o.libs = LibSafe
		o.capabilities = CapSafe
	}
}

// WithRequirePath sets the base directory for require() module loading.
func WithRequirePath(path string) Option {
	return func(o *vmOptions) { o.requirePath = path }
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
