# Go Embedding API Audit

This document audits the current public Go embedding API and the internal
concepts that are indirectly exposed through it. Scope: `gscript/`,
`cmd/gscript`, and the `internal` runtime/VM concepts that shape the public
embedding contract.

## Current Available API

Public import path:

```go
import "github.com/gscript/gscript/gscript"
```

Current public package surface:

- VM lifecycle: `New(opts ...Option) *VM`.
- Compilation: `Compile`, `CompileContext`, `CompileFile`,
  `CompileFileContext`, `WithSourceName`, and `Program.SourceName`.
- Execution: `(*VM).Exec(src string) error`, `(*VM).ExecContext`,
  `(*VM).ExecFile`, `(*VM).ExecFileContext`, `(*VM).Run`, and
  `(*VM).RunContext`.
- Calls: `(*VM).Call(name string, args ...interface{}) ([]interface{}, error)`,
  `(*VM).CallContext`, `(*VM).CallValue(fn interface{}, args ...interface{})
  ([]interface{}, error)`, and `(*VM).CallValueContext`.
- Globals: `(*VM).Set`, `(*VM).Get`.
- Raw-value globals and calls: `(*VM).GetValue`, `(*VM).SetValue`,
  `(*VM).CallFunction`.
- Host binding: `RegisterFunc`, `RegisterTable`, `BindStruct`,
  `BindStructWithConstructor`, `BindMethod`.
- Conversion: `ToValue`, `MustToValue`, `FromValue`.
- Options: `WithLibs`, `WithRequirePath`, `WithPrint`, `WithVM`, `WithJIT`,
  `WithTracing`.
- Standard-library flags and presets: `LibFlags`, individual `Lib*` bits,
  `LibAll`, `LibSafe`, `LibApp`, `LibGame`, and `LibRL`.
- Concurrency helper: `Pool`, `NewPool`, `Get`, `Put`, `Do`, `Size`.
- Error model: `ErrorKind`, `Error`, `ErrLex`, `ErrParse`, `ErrRuntime`,
  `ErrScript`.
- Advanced escape hatch: `(*VM).Interpreter() *runtime.Interpreter`.

Execution currently has two public engine modes. `New()` defaults to the
tree-walking interpreter. `WithVM()` switches execution to the bytecode VM.
`WithJIT()` implies `WithVM()` and enables the platform JIT where available.
`cmd/gscript` defaults differently: the CLI default is VM plus JIT unless
flags select VM-only or interpreter execution.

`cmd/gscript` exposes operational capabilities through flags, not through the
embedding API: `-e`, `-vm`, `-jit`, CPU/memory profiling, JIT tier stats,
timeline dumps, warm dumps, exit stats, Tier 2 diagnostics, MethodJIT op
audits, coroutine stats, and runtime path stats. These flags are useful design
signals for future Go diagnostics APIs, but they are not currently reusable by
embedders except by shelling out.

## Productionization Gaps

- Context-aware entry points exist, but cancellation is currently checked only
  before starting and after completion. Interpreter, bytecode VM, and JIT loops
  still need preemptive cancellation and instruction-budget polling.
- A public `Program` artifact exists for parsed source and lazily compiled
  bytecode, but it is not yet a full production compiled-artifact contract:
  sharing/concurrency, metadata, cache invalidation, and JIT-state ownership
  still need to be specified.
- Public API leaks unusable internal types. `ToValue`, `MustToValue`,
  `FromValue`, `GetValue`, `SetValue`, `CallFunction`, and `Interpreter`
  mention `internal/runtime` types. External modules cannot import those
  packages, so these APIs are not a stable third-party embedding contract.
- Error propagation is shallow. `gscript.Error` has `Line`, `Col`, `File`, and
  `Value`, but the wrappers mostly populate kind/message/file only. Internal
  `runtime.SourceError`, `runtime.LuaError`, stack/debug frames, and bytecode
  diagnostics are not normalized into a public error shape.
- `LibSafe` is a stdlib preset, not a complete sandbox. `WithMaxSteps` now
  provides a first CPU/work-unit budget for interpreter statements and bytecode
  instructions, but there is still no public filesystem/network/process policy,
  environment policy, module loader policy, host-call policy, wall-clock
  timeout, or allocation/table/string limit.
- `WithMaxSteps` disables JIT execution while the limit is active so native
  code cannot bypass budget checkpoints. JIT budget polling remains future
  work.
- Module loading is path-based and VM-owned. `WithRequirePath` and
  `ExecFile` set a script directory, but there is no public loader interface
  for canonical names, cache scoping, deny/allow decisions, virtual filesystems,
  content hashing, or source mapping.
- Reflection binding is convenient but underspecified for production. There
  are no allowlists, struct tags, nil policies, pointer/value ownership rules,
  panic recovery policy, context injection, receiver policy, or lifecycle
  hooks.
- Struct userdata is implemented as table/metatable plus a package-level Go
  value registry. That creates unclear lifetime, cleanup, isolation, and pool
  reset semantics.
- Concurrency semantics stop at "VM is not goroutine-safe." `Pool` has no
  reset/preload contract, no guarantee about module/global cleanup, and no
  guidance for shared host values or shared compiled code.
- Diagnostics and observability are CLI-first. The CLI can print JIT,
  coroutine, path, and warm-dump information, but Go embedders do not have a
  stable metrics/tracing hook.
- Engine selection and JIT fallback are not fully explicit. `WithJIT` documents
  Apple Silicon availability, but production callers need a validated engine
  config and a clear fallback/error policy.

## Internal Concepts: Publish or Keep Internal

Should become public, behind stable `gscript` package types:

- Value model: expose a `gscript.Value` abstraction with constructors,
  inspection, table access, function handles, and encode/decode helpers. Do
  not expose `runtime.Value` directly.
- Compiled programs: expose immutable `Program`/`Function` handles. Hide AST,
  bytecode, `vm.FuncProto`, JIT fields, caches, and feedback vectors.
- Source references and stack frames: expose source name, line, column,
  function name, native/script frame marker, and cause/value fields.
- Module loading: expose a `ModuleLoader` interface and source metadata
  types. Keep parser/runtime filesystem details internal.
- Sandbox and limits: expose policy interfaces and limit structs. Keep stdlib
  implementation tables internal.
- Host functions: expose a non-reflection `HostFunc` and a reflection binder
  with options. Keep `runtime.GoFunction` internal.
- Engine choice: expose a small enum/config for interpreter, bytecode VM, and
  JIT preference. Keep tiering managers, native code pointers, op audits,
  feedback vectors, and specialization machinery internal.
- Observability: expose structured events/counters for compile/run/call/module
  load/host call/cancel/error. Keep CLI dump formats as optional adapters, not
  core embedding types.

Should remain internal:

- Lexer/parser/AST concrete nodes and token streams, unless a separate parser
  API is intentionally designed.
- `runtime.Interpreter`, `runtime.Environment`, `runtime.Table`,
  `runtime.Value`, `runtime.GoFunction`, upvalues, coroutine internals, and
  package-cache internals.
- `vm.VM`, `vm.FuncProto`, opcodes, register files, call frames, global-index
  arrays, inline caches, type feedback, runtime specialization caches, and JIT
  ABI details.
- `methodjit` and `jit` types, native executable memory management, direct
  entry pointers, warm dump internals, and Tier 2 speculation state.
- Nanbox representation and memory layout details.
- The package-level struct registry currently used to back reflected struct
  values; replace it with VM-owned userdata internally rather than publishing
  it.

## Recommended Go API Shape

Keep the existing simple API for small programs, and add a production layer
that does not leak internal packages.

Configuration and engines:

```go
type EngineMode int

const (
    EngineInterpreter EngineMode = iota
    EngineBytecode
    EngineJIT
)

type Config struct {
    Engine EngineMode
    Libs LibFlags
    Loader ModuleLoader
    Sandbox SandboxPolicy
    Limits Limits
    Stdout func(context.Context, []Value) error
    Stderr func(context.Context, []Value) error
    Observer Observer
}

func NewVM(Config) (*VM, error)
```

Compilation and execution:

```go
type Program struct{}

type CompileOptions struct {
    SourceName string
    ModuleName string
}

func Compile(ctx context.Context, src []byte, opts CompileOptions) (*Program, error)
func CompileFile(ctx context.Context, path string, opts CompileOptions) (*Program, error)

func (vm *VM) Run(ctx context.Context, p *Program, args ...Value) ([]Value, error)
func (vm *VM) ExecContext(ctx context.Context, src string, opts CompileOptions) error
```

Calls and globals:

```go
type Function struct{}

func (vm *VM) GetFunc(name string) (Function, bool)
func (vm *VM) CallContext(ctx context.Context, name string, args ...any) ([]any, error)
func (vm *VM) CallInto(ctx context.Context, name string, out any, args ...any) error
func (vm *VM) CallFunc(ctx context.Context, fn Function, args ...Value) ([]Value, error)

func (vm *VM) SetValue(name string, v Value) error
func (vm *VM) Value(name string) (Value, bool)
```

Values and conversion:

```go
type Kind int
type Value struct{}

func Nil() Value
func Bool(bool) Value
func Int(int64) Value
func Float(float64) Value
func String(string) Value
func Array([]Value) Value
func Table(map[Value]Value) Value

func Encode(any) (Value, error)
func Decode(Value, any) error
```

The value API should define numeric overflow behavior, sparse-array handling,
map key support, table iteration order expectations, userdata lifetime, and
function identity.

Host binding:

```go
type HostFunc func(context.Context, []Value) ([]Value, error)

type ReflectOptions struct {
    Name string
    Tags []string
    Fields []string
    Methods []string
    PointerReceiver bool
    RecoverPanics bool
    InjectContext bool
}

func (vm *VM) Register(name string, fn HostFunc) error
func (vm *VM) RegisterReflect(name string, fn any, opts ReflectOptions) error
func (vm *VM) RegisterType(name string, binding TypeBinding) error
```

Errors and diagnostics:

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

`Error` should support `Unwrap`, `errors.Is`, and `errors.As`. Script
`error(value)` should preserve `Value`. Host panics should be recovered at the
boundary by default and reported as host errors unless explicitly configured.

Sandboxing and modules:

```go
type ModuleLoader interface {
    LoadModule(context.Context, ModuleRequest) (*Program, error)
}

type ModuleRequest struct {
    Name string
    From SourceRef
}

type Limits struct {
    Instructions int64
    WallTime time.Duration
    CallDepth int
    MemoryBytes int64
    TableEntries int64
    StringBytes int64
}
```

All filesystem, network, process, environment, clock, random, dynamic-load,
and debug capabilities should route through explicit policy objects.

## Backward Compatibility Strategy

- Keep `New(opts ...Option) *VM` as the convenience constructor. Implement it
  on top of `NewVM(Config)` once the production config exists.
- Keep `Exec`, `ExecFile`, `Call`, `CallValue`, `Set`, `Get`,
  `RegisterFunc`, `RegisterTable`, `BindStruct`, `BindStructWithConstructor`,
  `BindMethod`, and `Pool` source-compatible through the next major cycle.
- Add context-aware names instead of changing existing method signatures:
  `ExecContext`, `CallContext`, `Run`, `CallInto`.
- Keep `WithTracing()` as an alias for `WithJIT()` until a major version can
  remove or deprecate it cleanly.
- Mark APIs that mention `internal/runtime` as advanced/legacy in docs, then
  provide public replacements before deprecation. Do not remove them until
  embedders have `gscript.Value`, `Function`, and `Program`.
- Preserve `LibFlags` bit values and preset names. Add new library bits only
  at higher unused positions.
- Keep current reflection behavior as default for compatibility. Add stricter
  `ReflectOptions` for production behavior instead of changing existing
  binding defaults.
- Keep CLI flags stable. If Go diagnostics APIs are added, make the CLI call
  those APIs rather than changing flag output first.
- Define a versioned stability policy: convenience API is best-effort stable;
  production API is semver-stable; internals remain unsupported.

## Landing Plan

### P0

- Create public documentation that clearly labels the current embedding API,
  unstable raw-value escape hatches, and the fact that `LibSafe` is not a full
  security sandbox.
- Fill `gscript.Error` consistently from `runtime.SourceError` and
  `runtime.LuaError` in interpreter and bytecode paths.
- Add examples/tests for `WithLibs(LibSafe)`, `WithRequirePath`, `WithVM`,
  `WithJIT`, `RegisterFunc`, `RegisterTable`, struct binding, and `Pool`.
- Extend the existing `ExecContext`/`CallContext`/`RunContext` entry points
  with deep cancellation polling in interpreter, bytecode VM, and JIT paths.
- Decide engine default policy for embedding versus CLI and document the
  mismatch if it remains intentional.

### P1

- Add `gscript.Value`, `Kind`, constructors, inspection methods,
  `Encode`, and `Decode`.
- Harden the existing `Program`, `Compile`, `CompileFile`, and `Run` APIs into
  a documented compiled-artifact contract with explicit concurrency and
  JIT-state ownership rules.
- Add `Function` handles and public call APIs that avoid
  `internal/runtime.Value`.
- Implement context cancellation checks in interpreter and bytecode VM loops.
- Expand the initial `WithMaxSteps` control into `Limits` for instruction
  count, call depth, wall-clock timeout, memory, and host-call duration.
- Add structured `Frame` stack traces with parity across interpreter and
  bytecode VM.
- Add `HostFunc` and panic-safe host-call boundary.

### P2

- Add `ModuleLoader`, source references, module cache policy, and virtual
  filesystem hooks.
- Add policy-based sandboxing for filesystem, network, process, environment,
  clock, random, debug, and dynamic script loading.
- Replace the package-level reflected struct registry with VM-owned userdata
  and documented ownership/finalization semantics.
- Add `VM.Reset` and pool preload/reset options.
- Expose observability hooks for compile/run/call/module/host-call/JIT events.
- Move CLI diagnostics onto reusable Go diagnostics APIs.
- Add race/stress tests for pooled VMs, shared compiled programs, concurrent
  host callbacks, cancellation, and sandbox denial paths.

## Summary

The current API is good enough for demos, tests, trusted scripts, and
controlled in-process extension points. It is not yet a production embedding
contract because core pieces either leak `internal` types or exist only as
CLI/internal mechanisms. The recommended path is additive: keep the current
convenience layer, add public `Value`/`Program`/`Function` abstractions,
introduce context/limits/sandbox/module-loader policies, and keep VM, bytecode,
JIT, specialization, and runtime representation details internal.
