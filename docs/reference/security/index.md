# Leia Security And Sandboxing

Leia is an embeddable scripting language. Security is therefore host-owned:
the host decides which libraries exist, which host capabilities are enabled,
which Go bindings are registered, and which resource budgets apply.

## Default Trust Model

`leia.New()` defaults to `LibAll` and `CapAll` for local-script
compatibility. That is appropriate for trusted tools and development scripts,
not for untrusted user input.

Production embeddings should start from `SecuritySandbox()` and opt back into
specific capabilities.

```go
vm := leia.New(
    leia.SecuritySandbox(),
    leia.WithMaxSteps(100_000),
    leia.WithMaxNativeCalls(1_000),
    leia.WithMaxCallDepth(128),
)
```

## Libraries And Capabilities

Library flags decide which standard-library tables are installed.
Capability flags decide which installed APIs may touch host resources.

| Control | Meaning |
|---|---|
| `WithLibs(flags)` | Select available standard-library modules. |
| `LibAll` | Every standard library. Default. |
| `LibSafe` | Pure or sandbox-friendly modules with no filesystem, network, process, debug, script, or AI host access. |
| `LibApp` | Application preset with common host-facing libraries. |
| `LibGame` | Game preset with math, vectors, color, arrays, SoA, time, and random. |
| `WithCapabilities(flags)` | Select host-backed capability bits. |
| `CapAll` | Module loading, filesystem read/write, and environment read/write. Default. |
| `CapSafe` | No host-backed capability bits. |

Capability bits are separate from library selection so embedders can expose a
module table while denying its host effects.

Network, debug, testkit, process execution, shell execution, and dynamic eval
are controlled by separate switches. They default to enabled for trusted local
scripts and are disabled by `WithSandbox()` and `SecuritySandbox()`.

| Capability | Gates |
|---|---|
| `CapModuleLoading` | Script-side `require()` loading `.leia` files from the host filesystem. |
| `CapFilesystemRead` | `fs.readfile`, `fs.stat`, `fs.readdir`, `dofile`, `loadfile`, and related reads. |
| `CapFilesystemWrite` | `fs.writefile`, `fs.remove`, `fs.rename`, `fs.mkdir`, `fs.chdir`, and related writes. |
| `CapEnvironmentRead` | `os.getenv`, `os.environ`, `os.expand`. |
| `CapEnvironmentWrite` | `os.setenv`, `os.unsetenv`. |

## Sandbox Presets

| API | Behavior |
|---|---|
| `WithSandbox()` | Selects `LibSafe`, `CapSafe`, disables dynamic eval, network, debug, testkit, process execution, and shell execution. |
| `SecuritySandbox()` | Same baseline and also disables JIT by default. Use this for untrusted scripts. |
| `WithSecurity(policy)` | Applies one auditable grouped `SecurityPolicy`. |

JIT is disabled whenever runtime resource budgets require checkpoints that
native code could otherwise bypass.

## Filesystem And Environment

Constrain path access with a filesystem root:

```go
vm := leia.New(
    leia.SecuritySandbox(),
    leia.WithLibs(leia.LibSafe|leia.LibFS),
    leia.WithFilesystemRoot("/srv/app/scripts"),
    leia.WithFilesystemRead(true),
    leia.WithFilesystemWrite(false),
)
```

`WithFilesystemRoot` confines script-side file loading and `fs` module paths to
that root. It does not constrain host-side `CompileFile` or `ExecFile` calls,
because those are explicit host operations.

Restrict environment variables with an allowlist:

```go
vm := leia.New(
    leia.SecuritySandbox(),
    leia.WithEnvironmentRead(true),
    leia.WithEnvironmentAllowlist("APP_REGION", "FEATURE_FLAG"),
)
```

Passing an empty allowlist allows no variables. Omitting the allowlist keeps
environment access unrestricted when environment capability is enabled.

## Dynamic Code And Processes

These switches gate higher-risk host behavior:

| API | Gates |
|---|---|
| `WithDynamicEval(false)` | `load`, `loadstring`, `script.compile`, `script.eval`. |
| `WithNetworkAccess(false)` | Host-backed `net` and `http` APIs. |
| `WithDebugAccess(false)` | Script-visible debug APIs. |
| `WithTestkitAccess(false)` | Script-visible test diagnostics. |
| `WithProcessExecution(false)` | `process.run`, `process.exec`, `process.which`. |
| `WithProcessShell(false)` | `process.shell`. |

## Resource Budgets

| API | Limit |
|---|---|
| `WithMaxSteps(n)` | Interpreter statements or bytecode instructions per run. |
| `WithMaxNativeCalls(n)` | Script calls into Go functions, including stdlib and host callbacks. |
| `WithMaxCallDepth(n)` | Active script/native call depth. |
| `WithMaxGoroutines(n)` | Script goroutines started by `go`. |
| `WithMaxChannelCapacity(n)` | Maximum accepted `make(chan, n)` buffer capacity. |
| `WithMaxHostResultBytes(n)` | String bytes returned from one Go call. |
| `WithMaxModuleBytes(n)` | Bytes read by script-side source/module loading APIs. |
| `WithMaxModuleDepth(n)` | Nested filesystem-backed `require()` depth. |
| `WithMaxFilesystemReadBytes(n)` | Bytes read into memory by `fs.readfile` and `fs.copy`. |
| `WithMaxFilesystemWriteBytes(n)` | Bytes written by `fs.writefile`, `fs.appendfile`, and `fs.copy`. |

Use context deadlines around `Exec`, `Run`, `Call`, and hot reload methods for
wall-clock cancellation.

## Go Bindings

Leia never imports arbitrary Go packages by path at runtime. `require("go:...")`
works only for bindings registered by the host:

```go
vm := leia.New(leia.WithGoImports(map[string]any{
    "go:example.com/app/safe": SafeBinding{},
}))
```

Prefer narrow modules over exposing broad service objects. Host callbacks are
native code from Leia's point of view, so pair them with native-call and
host-result budgets when scripts are not fully trusted.

## Module Capability Summaries

`leia.mod` can declare capability summaries:

```text
capability fs.read
capability llm.turn
```

These declarations make module boundaries auditable. Use:

```bash
cd path/to/module
leia mod capability --json
```

to inspect capability summaries across the module graph.
