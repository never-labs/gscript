# Go Embedding API Audit and Roadmap

This document audits the current Go-facing embedding surface and defines the
API shape needed before GScript can be treated as a production embeddable
runtime. It is intentionally scoped to the public `gscript` package and the
runtime behavior visible through `gscript/`, `internal/runtime`,
`internal/vm`, and `cmd/gscript`.

## Current public API

The public import path is:

```go
import gs "github.com/gscript/gscript/gscript"
```

The current API is useful for tests, demos, and controlled in-process
extensions:

- VM creation: `gscript.New(opts ...Option) *VM`.
- Execution: `(*VM).Exec(src string) error` and `(*VM).ExecFile(path string) error`.
- Compiled programs: `Compile`, `CompileFile`, `(*VM).Run`, `Program`, and
  `Program.SourceName`.
- Script function calls: `(*VM).Call(name string, args ...interface{}) ([]interface{}, error)`.
- Function-value calls: `(*VM).CallValue(fn interface{}, args ...interface{}) ([]interface{}, error)` and the lower-level `(*VM).CallFunction(runtime.Value, []runtime.Value)`.
- Globals: `Set`, `Get`, `SetValue`, and `GetValue`.
- Go binding: `RegisterFunc`, `RegisterTable`, `RegisterModule`, `BindStruct`,
  `BindStructWithConstructor`, and `BindMethod`.
- Hot loading: `HotLoader`, `ModuleHandle`, and `HotInstance`.
- Value conversion: `ToValue`, `MustToValue`, and `FromValue`.
- Options: `WithLibs`, `WithCapabilities`, `WithSandbox`, `SecuritySandbox`,
  `WithModuleLoading`, `WithFilesystem`, `WithFilesystemRead`,
  `WithFilesystemWrite`, `WithFilesystemRoot`, `WithRequirePath`,
  `WithMaxSteps`, `WithMaxNativeCalls`, `WithMaxCallDepth`,
  `WithMaxGoroutines`, `WithMaxChannelCapacity`, `WithMaxHostResultBytes`,
  `WithPrint`, `WithVM`, `WithJIT`, and `WithTracing`.
- Standard-library presets: `LibAll`, `LibSafe`, `LibApp`, and `LibGame`.
- Concurrency helper: `Pool`, with the explicit contract that a `VM` is not goroutine-safe.
- Advanced escape hatch: `Interpreter() *runtime.Interpreter`.

The API currently exposes `internal/runtime.Value` through several advanced
methods. That is legal for code inside this module but not usable as a stable
external contract because Go `internal` packages cannot be imported by
embedders outside the parent tree. Any production embedding API must avoid
requiring external users to name `internal/runtime` types.

## Tested public examples

The public surface has executable Go examples in
`examples/embedding/embedding_test.go` and package-level examples in
`gscript/example_test.go`. They are normal Go example tests, so
`go test ./examples/embedding ./gscript` verifies that snippets for
`Compile`/`Run`, public `Value`, host function binding, `WithSandbox`,
`WithMaxSteps`, hot loading, explicit Go-backed host modules, and structured
errors continue to compile and match their documented output. See the
[embedding examples index](../examples/embedding/README.md) for the concise
example-by-example coverage list.

For example, a host can compile once, run on a VM, then inspect globals:

```go
prog, err := gs.Compile(`result := 40 + 2`, gs.WithSourceName("calc.gs"))
if err != nil {
    panic(err)
}
vm := gs.New(gs.WithVM())
if err := vm.Run(prog); err != nil {
    panic(err)
}
result, err := vm.Get("result")
if err != nil {
    panic(err)
}
fmt.Println(prog.SourceName(), result)
```

Structured errors use standard Go error APIs:

```go
var gsErr *gs.Error
var hostErr *gs.HostCallbackError
fmt.Println(errors.As(err, &gsErr), errors.As(err, &hostErr))
```

`os.exit` and `process.exit` return a catchable `*gs.ExitError` through the
public API instead of terminating the embedding process. The CLI maps that
error back to an operating-system exit code.

Go-backed modules are explicit host capabilities:

```go
vm := gs.New(gs.WithSandbox(), gs.WithModuleLoading(false))
vm.RegisterModule("go/strings", gs.Module{"upper": strings.ToUpper})
vm.Exec(`strings := require("go/strings"); result := strings.upper("hello")`)
```

For service structs or third-party clients, `RegisterModuleFrom` can export
public fields and methods without a hand-written module map:

```go
type Jobs struct{ Prefix string }
func (j *Jobs) Label(id int64) string { return fmt.Sprintf("%s-%03d", j.Prefix, id) }

vm.RegisterModuleFrom("go/jobs", &Jobs{Prefix: "job"})
vm.Exec(`jobs := require("go/jobs"); result := jobs.label(7)`)
```

Exported Go names are lower-camel-cased by default (`Label` becomes `label`).
Use `WithModuleExactNames` or `WithModuleNameMapper` when a host wants an exact
or custom script-facing API.

Registered host modules are available through `require(name)` even when
filesystem-backed module loading is disabled. A sandboxed embedding should
register only the `go/...` modules it intends scripts to access.

## Current execution model

`New` creates a tree-walking interpreter by default. `WithVM` switches `Exec`
and later calls to the bytecode VM path. `WithJIT` implies `WithVM` and enables
the platform JIT where available. The CLI defaults to bytecode VM plus JIT,
but the public `gscript.New()` default remains the interpreter path.

`Exec` and `ExecFile` parse source every time. The bytecode path compiles to an
internal `*vm.FuncProto` and persists the bytecode VM inside `gscript.VM` so
subsequent `Call` operations can route bytecode closures correctly. There is
also a reusable `Program` from `Compile`/`CompileFile` for embedding code that
wants an explicit compile step.

`HotLoader` is the public hot-reload helper. It compiles files with
`CompileFileContext`, stores the latest successful `Program` in an atomic
`ModuleHandle`, increments a generation counter on successful reload, and keeps
the previous generation active when recompilation fails. It does not watch the
filesystem and does not register files into `require`; embedders call `Reload`
from their own watcher, admin endpoint, or deployment hook.

Watchers should normally call `ReloadIfChanged` instead of `Reload`. It hashes
the source bytes and returns the current generation without recompiling,
publishing, or rerunning top-level code when the file has not changed. Use
`Reload` only when the host intentionally wants to force a generation bump.

`HotInstance` adds online reload semantics on top of `HotLoader`: it owns a
persistent VM, runs the initial program once, and on reload skips top-level
initializers for existing non-function globals while replacing function
definitions. Existing scalar/table/object state therefore survives ordinary
code reloads automatically, new globals receive their defaults, and removed
functions are not deleted by default. Compile failures and top-level runtime
failures leave the previous generation installed. Running goroutines and
externally saved old closures are not migrated.

`ExecFile` sets the interpreter script directory from the file path. `WithRequirePath`
sets the initial base directory used by `require` and script loading. Internal
runtime and VM code also support script `compile`, `eval`, `loadFile`,
`runFile`, sandbox/env options, and source names, but those capabilities are
not first-class public Go APIs.

## Binding and conversion audit

`RegisterFunc` wraps Go functions by reflection. It supports fixed and
variadic parameters, multiple return values, and a final `error` return. If the
final `error` is non-nil, the script call fails with that error.

`ToValue` currently supports:

- `nil`, booleans, signed and unsigned integers, floats, strings.
- slices and arrays as 1-based GScript arrays.
- maps as tables.
- structs and pointers to structs as reflected table/userdata-like values.
- functions as reflected native functions.
- raw `runtime.Value` pass-through.

`FromValue` supports typed conversion to booleans, integers, unsigned integers,
floats, strings, slices, maps, structs, and empty interface. Untyped default
conversion maps GScript ints to `int64`, floats to `float64`, strings to
`string`, arrays to `[]interface{}`, string-keyed tables to
`map[string]interface{}`, and functions to the raw value.

Struct binding is implemented as table plus metatable behavior backed by a
global Go value registry. Exported fields and methods are visible, fields are
settable when the reflected value allows it, and constructors are available via
`Name.new(...)`. This is adequate for experiments but needs a production
userdata ownership model, lifecycle rules, finalization/cleanup policy, and
clear pointer versus value semantics.

## Error and diagnostics audit

The public `gscript.Error` has `Kind`, `Message`, `Line`, `Col`, `File`, and
`Value` fields. In practice, most public wrappers currently set `Kind`,
`Message`, and sometimes `File`; line/column and original script error values
are not consistently propagated through the public API.

Internal runtime code has `runtime.SourceError` for source coordinates and
`runtime.LuaError` for script-raised values. The bytecode VM has a debug
library with stack snapshots and traceback formatting. These are not surfaced
as stable Go diagnostics. Production embedding needs structured stack frames,
source names, script error values, and error wrapping that works for both
interpreter and bytecode VM execution.

## Sandbox and module-loading audit

`WithLibs` restricts which registered standard-library globals remain
available. `LibSafe` removes obvious I/O, network, process, debug, script,
HTTP server, GL, testkit, and other unsafe or host-heavy modules from the
public preset. This is a useful compatibility control, but it is not a complete
security sandbox by itself.

The current public API separates library visibility from host effects:

- `LibFlags` answer "is this stdlib table visible and require-able as a
  built-in module?"
- `CapabilityFlags` answer "may visible script APIs perform host-backed
  effects?" The current flags are `CapModuleLoading`,
  `CapFilesystemRead`, and `CapFilesystemWrite`. `CapFilesystem` is a
  compatibility alias for `CapFilesystemRead | CapFilesystemWrite`.
- `WithSandbox()` selects `LibSafe` and `CapSafe`, so filesystem-backed module
  loading and script-side filesystem APIs are disabled by default.
- `WithModuleLoading(false)` blocks `.gs` files loaded through `require`, but
  still allows enabled built-in stdlib modules such as `json`.
- `WithFilesystem(false)` clears both filesystem read and write capabilities.
  When both are disabled, `fs`, `dofile`, and `loadfile` are removed; this does
  not change which safe built-in tables are present.
- `WithFilesystemRead(false)` disables read APIs such as `fs.readfile`,
  `fs.stat`, `fs.readdir`, `dofile`, and `loadfile` while leaving write access
  unchanged.
- `WithFilesystemWrite(false)` disables mutating APIs such as `fs.writefile`,
  `fs.remove`, `fs.rename`, `fs.mkdir`, `fs.chdir`, and `fs.tempfile` while
  leaving read access unchanged.
- `WithFilesystemRoot(root)` confines script-side paths to `root` and enables
  `CapFilesystem`, so it grants both read and write access unless followed by
  `WithFilesystemRead(false)` or `WithFilesystemWrite(false)`.
- Options are applied in order, so combine root confinement with read-only or
  write-only access by passing `WithFilesystemRoot(root)` before the narrower
  `WithFilesystemWrite(false)` or `WithFilesystemRead(false)` option.

Current sandbox gaps:

- `WithMaxSteps` exposes a first CPU/work-unit budget for interpreter
  statements and bytecode instructions. It does not yet cover memory, table
  growth, string allocation, recursion depth, goroutine/thread usage, host-call
  duration, or wall-clock time.
- `WithMaxNativeCalls` limits script calls into Go-backed functions, including
  standard-library calls and registered host callbacks. It counts bytecode
  stdlib fast paths too, but it is a call-count budget, not a duration or
  memory budget for work performed inside the host callback. Setting it
  disables JIT until compiled code consumes the same native-call counter.
- `WithMaxCallDepth` limits active function call depth in the interpreter and
  bytecode VM. Setting it disables JIT until compiled calls consume the same
  frame-depth budget.
- `WithMaxGoroutines` limits active goroutines started by script `go`
  statements, and `WithMaxChannelCapacity` limits `make(chan, n)` buffer
  sizes. Both disable JIT until compiled task/channel creation consumes the
  same policy.
- `WithMaxHostResultBytes` limits string bytes returned from one native Go
  call, including standard-library functions and registered host callbacks. It
  checks direct strings and strings nested in returned tables.
- Context-aware public entry points now poll cancellation at interpreter
  statement/loop checkpoints and bytecode instruction checkpoints. Native JIT
  loops and some blocking host operations still need broader policy-driven
  cancellation coverage.
- Filesystem policy currently has read/write capability bits plus one root
  confinement directory. It does not yet have separate read roots and write
  roots, byte limits, symlink policy, special-file policy, or per-operation
  audit events.
- No public host policy for network, process, environment, debug
  introspection, or host callbacks beyond coarse stdlib removal.
- No public loader interface for resolving, validating, caching, or auditing
  modules.
- No explicit isolation contract for host functions registered into a sandbox.

The internal `script` library can create isolated environments and child VM
executions, but the production Go API should not force embedders to call those
facilities from script code.

## Concurrency audit

The current public contract is clear: `VM` is not goroutine-safe. `Pool`
provides a simple synchronized idle list so callers can use one VM per request
or goroutine.

Production embedding still needs stronger guidance and API support:

- A compiled program should be immutable and safe to share across VMs.
- VM instances should remain single-owner while executing.
- Pools should define reset semantics, including globals, modules, stack,
  debug hooks, JIT state, and host-bound values.
- Host functions called from scripts must document whether they can be invoked
  concurrently by different VMs.
- Shared global maps in the bytecode VM need an explicit external contract,
  not just internal mutex choices.

## Production API requirements

### VM creation

Add an explicit configuration object while keeping option helpers for small
programs:

```go
type Config struct {
    Engine Engine
    Libs LibFlags
    ModuleLoader ModuleLoader
    Stdout func(context.Context, []Value) error
    Sandbox SandboxPolicy
    Limits Limits
}

func NewVM(cfg Config) (*VM, error)
```

`New` can remain a convenience wrapper, but production callers need validation
errors, default visibility, and reproducible configuration.

### Loading and compiling

Add public compiled artifacts:

```go
type Program struct { /* immutable */ }
type CompileOptions struct {
    SourceName string
    ModuleName string
}

func Compile(src []byte, opts CompileOptions) (*Program, error)
func CompileFile(path string, opts CompileOptions) (*Program, error)
func (vm *VM) Load(program *Program) error
func (vm *VM) Run(ctx context.Context, program *Program, args ...Value) ([]Value, error)
```

Compiled programs should hide `internal/vm.FuncProto` and AST details, but be
safe to cache, hash, and share across VM instances.

### Calling script functions

Add context-aware, typed call APIs:

```go
func (vm *VM) Call(ctx context.Context, name string, args ...any) ([]any, error)
func (vm *VM) CallInto(ctx context.Context, name string, out any, args ...any) error
func (vm *VM) GetFunc(name string) (Function, bool)
func (vm *VM) CallFunc(ctx context.Context, fn Function, args ...Value) ([]Value, error)
```

The existing `Call` shape is convenient but cannot express cancellation,
typed decode targets, or stable function handles without leaking
`internal/runtime.Value`.

### Go function and struct binding

Separate reflection convenience from production binding:

```go
type Binder interface {
    Bind(vm *VM) error
}

func (vm *VM) Register(name string, fn HostFunc) error
func (vm *VM) RegisterReflect(name string, fn any, opts ReflectOptions) error
func (vm *VM) RegisterType(name string, typ TypeBinding) error
```

`HostFunc` should accept `context.Context` and public `[]Value` values. Reflect
bindings should support explicit names, tags, pointer/value receiver policy,
method allowlists, field allowlists, error mapping, and panic recovery.

### Value conversion

Introduce a public `Value` type or interface in the `gscript` package. The
current raw-value escape hatches should not expose `internal/runtime`.

Required API:

- constructors such as `Nil()`, `Bool(bool)`, `Int(int64)`, `Float(float64)`,
  `String(string)`, `Array([]Value)`, and `Table(map[Value]Value)`;
- inspection methods such as `Kind`, `IsNil`, `Bool`, `Int`, `Float`,
  `String`, `Len`, `Index`, and `Field`;
- `Encode(any) (Value, error)` and `Decode(Value, any) error`;
- deterministic numeric overflow behavior;
- explicit table conversion policy for sparse arrays and non-string keys;
- userdata/host-object handles with lifecycle rules.

### Context and cancellation

Every execution entry point should accept `context.Context`. Cancellation must
be checked by both interpreter and bytecode VM loops, not only at call
boundaries. Timeouts should return a distinguishable error, for example
`errors.Is(err, context.Canceled)` or `context.DeadlineExceeded`.

`WithMaxSteps`, `WithMaxNativeCalls`, `WithMaxCallDepth`,
`WithMaxGoroutines`, `WithMaxChannelCapacity`, and
`WithMaxHostResultBytes` are the first production-limits APIs. A full
production limits object should still cover wall time, allocation/table sizes,
module count, and host-call duration policy.

### Errors and stack traces

Add a stable error model:

```go
type Error struct {
    Kind ErrorKind
    Message string
    SourceName string
    Line int
    Column int
    Stack []Frame
    Value Value
    Cause error
}

type Frame struct {
    Function string
    SourceName string
    Line int
    Column int
    Native bool
}
```

All public entry points should preserve `errors.As`/`errors.Is` behavior.
Interpreter and bytecode VM errors should report the same source coordinate and
stack-frame shape. Script `error(value)` should preserve the original value.
Host panics should be recovered at the boundary and returned as host errors
unless explicitly configured otherwise.

### Sandbox

`LibSafe` should be documented as a preset, not a security boundary. A
production sandbox should be policy-driven:

```go
type SandboxPolicy struct {
    AllowStdlib LibFlags
    FS FileSystemPolicy
    Network NetworkPolicy
    Process ProcessPolicy
    Environment EnvironmentPolicy
    Clock ClockPolicy
    Random RandomPolicy
    Debug DebugPolicy
}
```

All unsafe standard-library operations should route through policy interfaces.
The default production sandbox should deny filesystem, network, process,
environment, debug introspection, dynamic script loading, and native host
object access unless explicitly enabled.

`SecuritySandbox()` is the production-oriented baseline option: it is
`LibSafe` plus `CapSafe`, and it disables JIT by default so configured step
budgets and context cancellation are not bypassed by native code. It removes
filesystem-backed script APIs and disables file-module loading, while keeping
safe built-in modules require-able.

`WithSandbox()` remains the compatibility shorthand for `LibSafe` plus
`CapSafe`. It does not change execution mode, so production examples should
prefer `SecuritySandbox()` and then opt into explicit budgets such as
`WithMaxSteps`, `WithMaxNativeCalls`, `WithMaxCallDepth`,
`WithMaxGoroutines`, `WithMaxChannelCapacity`, and
`WithMaxHostResultBytes`.

Neither sandbox option wraps registered Go functions or provides fine-grained
network/process/debug policies if an embedder explicitly re-enables the
corresponding libraries. Those remain policy-layer work.

### Module loader

Define a public loader:

```go
type ModuleLoader interface {
    LoadModule(ctx context.Context, name string, from SourceRef) (*Program, error)
}
```

Loader behavior should include canonical module names, base directories,
search paths, package cache scoping, cycle handling, source names for
diagnostics, optional bytecode/program cache integration, and sandbox policy
checks before filesystem access.

Current module loading is path-based and VM-owned. Built-in stdlib modules are
resolved by name from the active `LibFlags` set and can still be required when
file-module loading is disabled. File modules are resolved relative to the
script directory or `WithRequirePath`, then checked against
`WithFilesystemRoot` when a root is configured. A production `ModuleLoader`
should preserve that separation: stdlib allowlists are not filesystem
permissions, and filesystem permissions are not stdlib visibility.

### Concurrent use

Keep `VM` single-owner and make that explicit. Add:

- immutable, shareable `Program`;
- `VM.Reset(ResetOptions)` for pool reuse;
- `Pool` support that can preload compiled programs and bindings;
- race-test coverage for pool use, shared compiled programs, and concurrent
  host callbacks;
- documentation that host-bound Go values are shared by reference only when the
  binder says so.

## Phased rollout

### Phase 0: Document and stabilize the current surface

- Keep `gscript.New`, `Exec`, `ExecFile`, `Call`, `Set`, `Get`,
  `RegisterFunc`, `RegisterTable`, `BindStruct`, and `Pool` working.
- Document that methods exposing `internal/runtime.Value` are advanced and not
  a stable external embedding contract.
- Fill line/column/file propagation in `gscript.Error` where internal
  `SourceError` already exists.
- Add examples for `WithLibs(LibSafe)`, `WithRequirePath`, `WithVM`, and
  pooling.

### Phase 1: Public values and compiled programs

- Harden the initial `Program`, `Compile`, `CompileFile`, and `Run` APIs into a
  stable production contract.
- Introduce `gscript.Value`.
- Hide internal `runtime.Value` from new public APIs.
- Decide whether compiled programs are immutable and safe for concurrent
  sharing, or explicitly single-owner because they may hold JIT state.
- Add conversion tests for overflow, sparse tables, maps, structs, function
  values, and script error values.

### Phase 2: Context, limits, and diagnostics

- Extend existing context-aware entry points for run and call with preemptive
  cancellation.
- Implement VM/interpreter cancellation checks and instruction budgets.
- Surface structured stack traces consistently across interpreter and bytecode
  VM paths.
- Recover panics from host functions and report them as structured host errors.

### Phase 3: Binding API hardening

- Add non-reflection `HostFunc` registration.
- Add reflection options for tags, allowlists, receiver policy, nil handling,
  panic policy, and context injection.
- Replace the global struct registry with a VM-owned userdata store and
  documented lifetime semantics.
- Add typed `CallInto`/`Decode` helpers for service code.

### Phase 4: Sandbox and module loader

- Add `SandboxPolicy` and route stdlib side effects through it.
- Add `ModuleLoader` and remove direct filesystem assumptions from production
  module loading.
- Define module cache scoping per VM, per pool, and per loader.
- Add deny-by-default examples for running untrusted scripts.

### Phase 5: Production concurrency and observability

- Add `VM.Reset` and pool preload hooks.
- Add execution metrics hooks: compile time, run time, instruction count,
  allocations where available, module loads, host calls, and cancellations.
- Add race tests and stress tests for shared programs and pooled VMs.
- Document JIT availability and fallback behavior separately from the embedding
  API so embedders can choose predictable engine modes.

## Bottom line

The current Go API is a practical convenience wrapper around the interpreter
and bytecode VM, but it is not yet a production embedding contract. The main
gaps are stable public value types, a fully specified compiled-program
contract, preemptive context cancellation, structured diagnostics,
policy-based sandboxing, a host-controlled module loader, and precise
concurrency/reset semantics. The recommended path is to preserve the current
convenience API while adding a stricter production layer that does not leak
internal packages.
