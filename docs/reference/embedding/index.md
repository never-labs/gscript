# Leia Go Embedding API

The public Go package is `github.com/never-labs/leia`. It is the stable
embedding surface for applications that run Leia as an in-process scripting
language.

Use internal packages only when developing Leia itself. Embedders should not
import `internal/runtime`, `internal/vm`, `internal/stdlib`, or compiler
packages.

## Core Types

| API | Purpose |
|---|---|
| `leia.New(opts ...Option) *VM` | Create a VM with standard library, module, sandbox, and execution options. |
| `type VM` | Mutable execution environment. Not goroutine-safe. |
| `leia.Compile(src, opts...)` | Parse source into a reusable `Program`. |
| `leia.CompileFile(path, opts...)` | Read and compile a `.leia` file. |
| `type Program` | Compiled source unit. Do not run the same `Program` concurrently. |
| `type Value` | Public Leia value wrapper that hides runtime representation. |
| `type Module` | Go-backed namespace for `require(name)`. |
| `type Pool` | Simple VM pool for concurrent host workloads. |
| `type HotLoader` | Reload-oriented loader that preserves runtime state where possible. |

See [Hot reload](../hot-reload/index.md) for generation, state-preservation,
and rollback behavior.

## Running Code

```go
vm := leia.New(leia.WithVM())
if err := vm.Exec(`print("hello")`); err != nil {
    return err
}
```

For reusable code, compile once and run on a VM:

```go
prog, err := leia.Compile(`func add(a, b) { return a + b }`, leia.WithSourceName("math.leia"))
if err != nil {
    return err
}
vm := leia.New(leia.WithVM(), leia.WithJIT())
if err := vm.Run(prog); err != nil {
    return err
}
out, err := vm.Call("add", 2, 3)
```

Context-aware methods exist for compile, run, exec, and call paths:
`CompileContext`, `CompileFileContext`, `ExecContext`, `ExecFileContext`,
`RunContext`, `CallContext`, and `CallValueContext`. Context cancellation is
checked before starting and after completion. Runtime preemption for
long-running scripts is controlled separately through sandbox and resource
budgets.

Use `WithArgs(script, args...)` at construction time, or `VM.SetArgs(script,
args)` on an existing VM, to set the `arg` table for script entrypoints.
`arg[0]` is the script name and `arg[1..n]` are user arguments.

`WithPrint(fn)` overrides script `print()` output for tests, games, servers,
and other hosts that should not write directly to stdout.

## Host Bindings

Use explicit registration. Leia does not dynamically reflect arbitrary Go
packages by import path.

| API | Purpose |
|---|---|
| `RegisterFunc(name, fn)` | Expose one Go function as a global. |
| `RegisterTable(name, map[string]any)` | Expose a global namespace table. |
| `RegisterModule(name, Module)` | Expose a `require(name)` module. |
| `RegisterModuleFrom(name, source, opts...)` | Build a module from exported Go fields and methods. |
| `WithGoImports(map[string]any)` | Allow `require("go:...")` for explicit host-provided bindings. |
| `BindStruct(name, proto)` | Expose a Go struct as a Leia class-like value. |
| `BindStructWithConstructor(name, proto, ctor)` | Expose a struct with a custom `.new()` constructor. |
| `BindMethod(className, methodName, fn)` | Add a method to a bound struct class. |

Host callbacks use reflection conversion rules. Prefer small, auditable
modules over broad service objects.

`ModuleFrom` and `RegisterModuleFrom` lower-case the first exported Go name rune
by default. Use `WithModuleExactNames()` to preserve Go names exactly, or
`WithModuleNameMapper(fn)` to define a host-specific naming policy.

## Values

Construct public values with:

```go
leia.Nil()
leia.Bool(true)
leia.Int(42)
leia.Float(3.14)
leia.String("ok")
leia.Decode(goValue)
```

Use `MustDecode(goValue)` only for examples and initialization paths where a
panic is acceptable. Convert back with `leia.Encode(v)` or `v.Encode()`.
`Value` is useful for storing script functions or data without exposing
internal runtime types.

VM globals and function values can stay on the public value boundary:

| API | Purpose |
|---|---|
| `Set(name, value)` / `Get(name)` | Convert between Go values and script globals. |
| `SetPublicValue(name, value)` / `GetPublicValue(name)` | Store or read globals as public `Value`. |
| `CallValue(fn, args...)` | Call a script function value using Go values. |
| `CallPublicValue(fn, args...)` | Call a script function value using public `Value` arguments and results. |

## Standard Library And Capabilities

`WithLibs` selects which standard-library modules exist. `WithCapabilities`
controls host-backed behavior behind those modules.

Recommended presets:

| Preset | Use |
|---|---|
| `LibAll` + `CapAll` | Compatibility and trusted local scripts. This is the default. |
| `LibSafe` + `CapSafe` | Pure in-process sandbox baseline. |
| `LibApp` | Application scripts with common host-facing libraries. |
| `LibGame` | Game scripting with math, vectors, color, arrays, and time. |

For untrusted scripts, start with `SecuritySandbox()`. It selects `LibSafe`,
`CapSafe`, disables dynamic eval and host-backed process/network/debug/testkit
surfaces, and disables JIT by default. Grant back only the library,
capability, filesystem, module, and budget knobs that the embedding needs.

For long-lived embedded policies or game/server scripts, put those controls on
the hot-loader VM options so every reload keeps the same security envelope:

```go
loader := leia.NewHotLoader(leia.WithHotLoaderVMOptions(
    leia.SecuritySandbox(),
    leia.WithLibs(leia.LibSafe),
    leia.WithGoImports(map[string]any{
        "go:host/safe": leia.Module{"label": safeLabel},
    }),
    leia.WithMaxSteps(100_000),
    leia.WithMaxNativeCalls(1_000),
    leia.WithMaxHostResultBytes(1<<20),
))
inst, err := loader.LoadInstance("policy.leia")
```

Source syntax such as `import "go:host/safe" as host` succeeds only for
bindings in `WithGoImports` or `RegisterModule`; `import "go:os" as os` remains
rejected unless the host explicitly provided that binding. `HotInstance`
reloads changed code while preserving compatible script state under the same
sandbox and budget limits.

Important sandbox and budget options:

```go
vm := leia.New(
    leia.SecuritySandbox(),
    leia.WithMaxSteps(100_000),
    leia.WithMaxNativeCalls(1_000),
    leia.WithMaxCallDepth(128),
    leia.WithMaxGoroutines(16),
    leia.WithMaxChannelCapacity(64),
    leia.WithMaxHostResultBytes(1<<20),
    leia.WithMaxModuleBytes(1<<20),
    leia.WithMaxModuleDepth(8),
    leia.WithFilesystemRoot("/srv/app/scripts"),
)
```

| Option | Budget |
|---|---|
| `WithMaxSteps(n)` | Script execution steps per `Exec`/`Run`. |
| `WithMaxNativeCalls(n)` | Calls from script into Go callbacks and Go-backed standard-library functions. |
| `WithMaxCallDepth(n)` | Active script/native call depth. |
| `WithMaxGoroutines(n)` | Active goroutines started by script `go` statements. |
| `WithMaxChannelCapacity(n)` | Maximum `make(chan, n)` buffer capacity. |
| `WithMaxHostResultBytes(n)` | Bytes returned by one Go-backed host or standard-library call. |
| `WithMaxModuleBytes(n)` | Bytes read by script-side `require`, `dofile`, `loadfile`, and `script.loadFile`. |
| `WithMaxModuleDepth(n)` | Nested filesystem-backed `require()` depth. |
| `WithMaxFilesystemReadBytes(n)` | Bytes read by script filesystem APIs. |
| `WithMaxFilesystemWriteBytes(n)` | Bytes written by script filesystem APIs. |

Setting resource budgets disables JIT execution for that VM so native code
cannot bypass budget checkpoints. Budget failures are reported as
`*leia.BudgetError`, which exposes `Resource` and `Limit`.

`WithSecurity(SecurityPolicy{...})` groups the same sandbox controls behind one
auditable option. The fine-grained switches remain available when a host wants
to compose its own policy:

| Option | Controls |
|---|---|
| `WithSandbox()` | Legacy safe-library and no-filesystem baseline. |
| `WithModuleLoading(false)` | Filesystem-backed `require()` loading. |
| `WithFilesystem(false)` | Filesystem read and write together. |
| `WithFilesystemRead(false)` / `WithFilesystemWrite(false)` | Filesystem reads or writes separately. |
| `WithEnvironment(false)` | Environment reads and writes together. |
| `WithEnvironmentRead(false)` / `WithEnvironmentWrite(false)` | Environment reads or writes separately. |
| `WithEnvironmentAllowlist(names...)` | Environment variables visible to scripts. |
| `WithDynamicEval(false)` | Script-side string compilation APIs. |
| `WithNetworkAccess(false)` | Host-backed network APIs in `net` and `http`. |
| `WithProcessExecution(false)` / `WithProcessShell(false)` | Process execution and shell helpers. |
| `WithDebugAccess(false)` / `WithTestkitAccess(false)` | Script-visible diagnostics surfaces. |

See [Security and sandboxing](../security/index.md) for the full capability,
filesystem, process, dynamic-eval, and resource-budget model.

## Modules

Module-aware embeddings can use:

| API | Purpose |
|---|---|
| `WithRequirePath(path)` | Base directory for `require()`. |
| `WithModuleCollection(name, root)` | Map `name:pkg.util` collection imports to a root. |
| `WithModuleReplace(path, root)` | Map a module path prefix to a local root. |
| `WithModuleCache(root)` | Use a cache populated by `leia mod download`. |
| `WithModuleMode(ModuleModeReadonly)` | Record readonly module mode. |
| `WithModuleMode(ModuleModeVendor)` | Restrict cache resolution to vendor entries. |
| `ModuleOptionsForScript(path)` | Build options from the nearest `leia.mod`. |
| `ModuleOptionsForScriptMode(path, mode)` | Build module options with an explicit mode. |
| `ValidModuleMode(mode)` | Validate user-provided module mode strings. |

See [Modules](../modules/index.md) for the `leia.mod` file format.

## Dialects

`WithDialect(name, handler, opts...)` registers a host-provided tagged dialect.
`DialectHandler` receives the tagged body and options as public `Value` objects
and returns public `Value` results. `DialectOptions` can define aliases,
category metadata, capability labels, and a separate block handler.

## AI Providers

The root package exposes host options for AI-native scripts, while provider
implementations live under `github.com/never-labs/leia/llm/...`.

| API | Purpose |
|---|---|
| `WithLLMProvider(provider)` | Install the provider used by `llm.turn`. |
| `WithLLMProviderFactory(factory)` | Construct providers for script-declared `model {}` aliases. |
| `WithLLMTrace(sink)` | Receive metadata trace events. |
| `WithLLMRecorder(sink)` | Record provider turns for tests or offline review. |
| `WithLLMReplay(records)` | Replay recorded turns deterministically. |

## VM Lifetime

`VM` is mutable and not goroutine-safe. Create one VM per request/goroutine, or
use `Pool`.

`Reset()` clears script-created state and reinitializes the VM with the same
options. Pooling preserves state unless the pool was created with an explicit
reset hook.

## Execution Engines

| Option | Behavior |
|---|---|
| `WithVM()` | Use the bytecode VM instead of the tree-walking interpreter. |
| `WithJIT()` | Enable the ARM64 JIT and imply `WithVM()`. |
| `WithTracing()` | Compatibility alias for `WithJIT()`. |

The interpreter is the semantic baseline. VM and JIT modes are accelerators and
must preserve script behavior.
