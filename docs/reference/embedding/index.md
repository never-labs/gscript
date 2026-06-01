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
prog, err := leia.Compile(`func add(a, b) { return a + b }`)
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
checked at entry/exit and runtime checkpoints.

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

Convert back with `leia.Encode(v)` or `v.Encode()`. `Value` is useful for
storing script functions or data without exposing internal runtime types.

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

Important sandbox options:

```go
vm := leia.New(
    leia.SecuritySandbox(),
    leia.WithMaxSteps(100_000),
    leia.WithMaxNativeCalls(1_000),
    leia.WithMaxCallDepth(128),
    leia.WithMaxModuleBytes(1<<20),
    leia.WithFilesystemRoot("/srv/app/scripts"),
)
```

Setting resource budgets disables JIT execution for that VM so native code
cannot bypass budget checkpoints.

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

See [Modules](../modules/index.md) for the `leia.mod` file format.

## AI Providers

The root package exposes host options for AI-native scripts, while provider
implementations live under `github.com/never-labs/leia/llm/...`.

| API | Purpose |
|---|---|
| `WithLLMProvider(provider)` | Install the provider used by `llm.turn`. |
| `WithLLMProviderFactory(factory)` | Construct providers for script-declared `models {}` configs. |
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
