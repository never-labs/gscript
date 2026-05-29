# Security and Isolation Roadmap

This document defines the production security model GScript should expose to
embedders. It is a roadmap, not a statement that every control is implemented
today. Current code already has an important base: `LibFlags`,
`CapabilityFlags`, `LibSafe`, `WithSandbox`, stdlib module restriction,
filesystem root confinement, bytecode VM call-frame limits, and host-controlled
process exit errors. Production isolation needs those controls to be unified
under one policy object and enforced consistently in the tree interpreter,
bytecode VM, JIT, stdlib, host callbacks, coroutines, goroutines, and channels.

## Goals

- Make untrusted script execution bounded by default.
- Keep all host effects behind explicit capabilities.
- Make resource exhaustion fail as ordinary script errors, not host panics or
  process crashes.
- Give embedders one auditable policy surface for CPU, memory, recursion,
  concurrency, modules, filesystem, network, process, and host callbacks.
- Preserve fast paths by making security checks cheap, predictable, and shared
  across interpreter and JIT paths.

## Threat Model

The security model is designed for running code supplied by tenants, plugins,
users, generated agents, or config-driven automation. It must defend the host
against:

- Infinite loops and pathological recursion.
- Memory blowups from table/string growth, deep coroutine trees, and large
  stdlib results.
- Goroutine, coroutine, and channel exhaustion.
- Filesystem, network, process, environment, and OS escape through stdlib or
  registered Go callbacks.
- Module loading outside an allowed root or from unapproved module names.
- Panic propagation across the VM boundary.
- JIT paths bypassing interpreter checks.

This is not a kernel sandbox. The host process remains trusted. OS-level
isolation such as containers, seccomp, pledge/unveil, namespaces, cgroups, or
firecracker-style microVMs is still recommended for hostile multi-tenant code.

## Current Baseline

The embedding API already exposes `WithLibs`, library presets,
`WithCapabilities`, `WithModuleLoading`, `WithFilesystem`,
`WithFilesystemRead`, `WithFilesystemWrite`, `WithFilesystemRoot`, and
`WithSandbox`.

Current controls are split deliberately:

- `LibFlags` select which standard-library tables are present as globals and
  available through built-in `require(name)`. `LibSafe` removes I/O, network,
  process, script, debug, HTTP server, GL, test diagnostics, and other unsafe
  or host-heavy modules from that table set.
- `CapabilityFlags` gate host-backed effects that can remain dangerous even
  when a library table exists. The current public flags are
  `CapModuleLoading` for filesystem-backed `.gs` module loading and
  `CapFilesystemRead` / `CapFilesystemWrite` for script-side filesystem APIs.
  `CapFilesystem` is a compatibility alias for
  `CapFilesystemRead | CapFilesystemWrite`.
- `WithLibs` does not grant host capabilities. `WithCapabilities` does not
  make hidden stdlib tables visible. A filesystem-backed stdlib table such as
  `fs` needs both its `Lib*` bit and at least the relevant filesystem
  capability. Read APIs such as `fs.readfile`, `fs.stat`, `fs.readdir`,
  `dofile`, and `loadfile` require `CapFilesystemRead`; write APIs such as
  `fs.writefile`, `fs.remove`, `fs.rename`, `fs.mkdir`, `fs.chdir`, and
  `fs.tempfile` require `CapFilesystemWrite`.

The public VM applies stdlib restriction to both tree-walk and bytecode VMs.
Capability restrictions are also bridged into bytecode execution: disabled
filesystem globals are removed, and the bytecode `require` path uses the
interpreter-backed policy when module loading is disabled or a filesystem root
is configured.

The bytecode VM has internal constants such as `maxStack`, `maxCallDepth`, and
`maxMetaDepth`. These are useful safety rails, but they are not yet an
embedder-facing policy. They should become configured limits with consistent
errors and audit events.

The stdlib surface includes host-effect modules such as `io`, `fs`, `net`,
`http`, `os`, `process`, `script`, `debug`, and `testkit`. In production these
modules must be considered capabilities, not ordinary libraries. The current
`CapabilityFlags` layer is intentionally smaller than the future
`SecurityPolicy`: it covers module loading, filesystem-backed script APIs,
environment allowlists, process execution gates, and the `process.shell` gate,
but not network, executable allowlists, debug introspection, memory, or host
callback effects yet.

## Default Security Configuration

The current public safe entry point is:

```go
vm := gscript.New(
    gscript.SecuritySandbox(),
    gscript.WithMaxSteps(1_000_000),
)
```

For applications that prefer one auditable configuration object, the public API
also exposes `WithSecurity(SecurityPolicy)`. The first implementation groups
the controls that exist today: stdlib preset, host capabilities, module
loading, JIT policy, step budget, native-call budget, call-depth budget,
script goroutine limit, channel-capacity limit, host-result byte limit,
module-byte limit, module-depth limit, filesystem read-byte limit, and
filesystem write-byte limit, and dynamic-eval disablement.
Environment reads can also be narrowed to a named allowlist.

`SecuritySandbox()` currently means:

- `LibSafe` is selected as the stdlib preset.
- `CapSafe` is selected, which disables `CapModuleLoading` and
  both `CapFilesystemRead` and `CapFilesystemWrite`.
- JIT is disabled by default so step budgets and context cancellation are not
  bypassed by native code.
- Filesystem-backed globals `fs`, `dofile`, and `loadfile` are absent.
- `require("json")` and other enabled built-in stdlib modules still work,
  because stdlib module identity is controlled by `LibFlags`, not
  `CapModuleLoading`.
- `require("tenant.module")` cannot load a `.gs` file from the host
  filesystem.

This is an in-process sandbox boundary, not a complete isolation boundary. It
does not currently add memory budgets, wall-clock preemption, network/process
policy, debug redaction, or host-callback capability
wrapping. For untrusted scripts, embedders should combine `SecuritySandbox()`
with `WithMaxSteps`, context deadlines, explicit host bindings, and OS-level
isolation where needed.

The future full security policy should add:

- CPU budget enabled.
- Wall-clock timeout enabled.
- Heap and object budgets enabled.
- Call depth and metamethod depth limits enabled.
- Coroutine, goroutine, and channel limits enabled.
- `LibSafe` module set only.
- No filesystem, network, process, environment, debug, HTTP server, dynamic
  script loading, or host reflection capabilities.
- JIT allowed only if it consumes the same budget counters and guard checks as
  the bytecode VM. Otherwise JIT is disabled for sandboxed VMs.
- Panic recovery enabled at every host boundary.
- Audit logging enabled for denied capability attempts and budget exits.

`LibAll` should remain a development convenience, not the default for
untrusted code. Documentation and examples that run user-supplied scripts
should prefer `SecuritySandbox` or `LibSafe`.

## CPU Budget

CPU budget should be measured in VM work units rather than only wall time. Wall
time catches blocking host calls, but instruction budgets make pure compute
deterministic and testable.

Required controls:

- `WithMaxSteps` / future `MaxInstructions`: checked at bytecode dispatch and
  tree-walker statement boundaries. This exists today for interpreter and
  bytecode VM execution.
- `WithMaxNativeCalls`: limits calls from script into Go stdlib or registered
  host callbacks. This exists today for interpreter and bytecode VM execution,
  including bytecode stdlib fast paths. Setting this option disables JIT until
  compiled code debits the same counter.
- `WithMaxCallDepth`: limits active function call depth in the interpreter and
  bytecode VM. Setting this option disables JIT until compiled calls use the
  same frame-depth checks.
- `WithMaxGoroutines`: limits active goroutines started by script `go`
  statements. Setting this option disables JIT until compiled task creation
  uses the same counter.
- `WithMaxChannelCapacity`: limits the buffer capacity accepted by
  `make(chan, n)` in the interpreter and bytecode VM. Setting this option
  disables JIT until compiled channel creation uses the same check.
- `WithMaxHostResultBytes`: limits string bytes returned from one native Go
  call, including stdlib functions and registered host callbacks. Setting this
  option disables JIT until compiled native calls use the same result check.
- `WithMaxModuleBytes`: limits bytes read by script-side module/file loading
  APIs such as `require`, `dofile`, `loadfile`, and `script.loadFile`.
  Host-side `CompileFile`/`ExecFile` calls are not counted.
- `WithMaxModuleDepth`: limits nested filesystem-backed `require` chains.
  Built-in and preloaded modules do not consume this budget.
- `WithMaxFilesystemReadBytes`: limits bytes read into memory by `fs.readfile`
  and `fs.copy`.
- `WithMaxFilesystemWriteBytes`: limits bytes written by `fs.writefile`,
  `fs.appendfile`, and `fs.copy`.
- `WithDynamicEval(false)`: disables script-side string compilation APIs such
  as `load`, `loadstring`, `script.compile`, and `script.eval`.
- `MaxJITTicks`: JIT code must periodically debit the same execution budget at
  loop backedges, call exits, and side exits.
- `Deadline` or `Timeout`: wall-clock cancellation via `context.Context`;
  public run/call entry points poll it in interpreter and bytecode VM
  checkpoints.
- `CheckInterval`: amortizes budget checks for hot loops without making
  cancellation unresponsive.

Failure mode: return `ErrBudgetExceeded` with kind `cpu`, current source
location when available, and budget counters. Do not panic. Do not call
`os.Exit`.

Implementation notes:

- The first implementation stores per-VM step and native-call counters in the
  runtime interpreter and bytecode VM, and the same checkpoints poll active
  public execution contexts. A later shared `ExecutionBudget` should add
  structured counters, deadline metadata, JIT polling, and host-call duration
  accounting.
- Compile/JIT loop backedges should include budget polls. If a compiled path
  cannot poll correctly, it must side-exit to the interpreter before continuing.
- Blocking stdlib operations must use the VM context so a timed-out script
  cannot remain stuck in HTTP, process, sleep, or file operations.

## Memory Limits

Memory limits must cover VM-owned objects, host result materialization, and
large string/table operations. Go heap usage can only be approximate, so the
VM should combine explicit accounting with optional runtime heap sampling.

Required controls:

- `MaxHeapBytes`: approximate bytes allocated by VM tables, strings, closures,
  coroutines, channels, and buffers.
- `MaxObjects`: total VM object count.
- `MaxStringBytes`: maximum size of one string.
- `MaxTableEntries`: maximum entries in one table.
- `MaxArrayLength`: maximum sequence length for array-like tables.
- `WithMaxHostResultBytes`: maximum string bytes returned by one native call,
  including `fs.readfile`, `io.read`, `net.*`, `process.run`, JSON
  encode/decode, compression, and registered host callbacks. This exists today
  for direct string results and strings nested in returned tables.
- `MaxRegexWorkBytes` or equivalent output limits for regexp split/find-all.

Failure mode: return `ErrBudgetExceeded` with kind `memory`. Partially created
objects must not be published to globals or tables after the error.

Implementation notes:

- Put accounting in allocation constructors and growth paths, not at every
  read. Table resize, string creation, byte buffers, channel creation, and
  coroutine creation are the main enforcement points.
- Charge host callbacks for values converted into GScript values.
- Keep `collectgarbage("stats")` diagnostic-only. It should not be the security
  enforcement API.

## Recursion and Call Depth

Call depth limits need to cover all call routes:

- Tree-walker function calls.
- Bytecode `OP_CALL`, protected calls, metamethod calls, and `tostring` hooks.
- JIT direct calls and native recursive fast paths.
- Host callbacks that call back into the VM.
- Coroutine resume chains.

Required controls:

- `WithMaxCallDepth`: total active script/native frames. This exists today for
  the interpreter and bytecode VM.
- `MaxNativeCallDepth`: script to Go to script nesting.
- `MaxMetaDepth`: `__index`, `__newindex`, `__call`, comparison, arithmetic,
  and `__tostring` recursion.
- `MaxResumeDepth`: coroutine resume nesting.
- `MaxDeferDepth`: deferred callback nesting, if defer is active in a frame.

Failure mode: return `ErrBudgetExceeded` with kind `call_depth` or
`meta_depth`. A depth failure must unwind like an ordinary runtime error and
must remain catchable by `pcall`/`xpcall`.

The existing bytecode `maxCallDepth` and `maxMetaDepth` should become policy
defaults. JIT recursion limits must remain tied to Go goroutine stack safety;
compiled recursion cannot rely on OS guard pages because JIT code runs on a Go
goroutine stack.

## Goroutine, Coroutine, and Channel Limits

GScript exposes coroutines today and has tests around Go channel host
integration. Any production concurrency feature must be bounded because it can
otherwise bypass CPU and memory assumptions.

Required controls:

- `MaxCoroutines`: total live VM coroutines.
- `MaxCoroutineStackBytes`: approximate retained register/frame memory per
  coroutine.
- `WithMaxGoroutines`: script-created goroutines or background tasks. This
  exists today for `go` statements in the interpreter and bytecode VM.
- `MaxChannels`: total live channels.
- `WithMaxChannelCapacity`: per-channel buffer capacity. This exists today for
  script-created channels.
- `MaxBlockedTasks`: tasks blocked on channel send/receive, sleep, network, or
  process waits.
- `AllowBackgroundTasks`: default false in sandbox mode.

Failure mode: creating a task, coroutine, or channel past the limit returns an
ordinary runtime error. When VM context is cancelled, all child tasks should be
signalled and joined before `Exec` returns, or reported as leaked with an audit
event.

Implementation notes:

- Store concurrency counters in a shared `SecurityRuntime` so child coroutine
  VMs debit the parent policy.
- Script goroutines must not share mutable VM state without a VM-owned
  scheduler or explicit synchronization. If they use separate child VMs, they
  must share the same budget and capability object.
- Channel send/receive should poll context cancellation.

## IO, Network, and Process Permissions

Stdlib module flags are necessary but not sufficient. Once a module is enabled,
operations need path, address, method, size, and timeout checks.

Filesystem policy:

- Default deny.
- Allow explicit read roots and write roots.
- Resolve symlinks and clean paths before checking roots.
- Separate permissions for read, write, append, remove, rename, mkdir, glob,
  temp files, and current working directory access.
- Limit file size read into memory and bytes written per operation.
- Deny device files, FIFOs, sockets, and special files unless explicitly
  allowed.

Current public root confinement:

- `WithFilesystemRoot(root)` confines script-side filesystem paths to `root`
  and enables `CapFilesystem`, the compatibility alias for both filesystem
  read and write access.
- `WithFilesystemRead(false)` disables read APIs such as `fs.readfile`,
  `fs.stat`, `fs.readdir`, `dofile`, and `loadfile`. If write access remains
  enabled, `fs` can still expose write operations.
- `WithFilesystemWrite(false)` disables mutating APIs such as `fs.writefile`,
  `fs.remove`, `fs.rename`, `fs.mkdir`, `fs.chdir`, and `fs.tempfile`. If read
  access remains enabled, `fs` can still expose read operations and
  `dofile`/`loadfile`.
- `WithFilesystem(false)` disables both filesystem read and write access by
  clearing `CapFilesystem`.
- Options are applied in order. To create a confined read-only filesystem, pass
  `WithFilesystemRoot(root)` before `WithFilesystemWrite(false)`; to create a
  confined write-only filesystem, pass it before `WithFilesystemRead(false)`.
- Relative paths are resolved under `root`; absolute paths are allowed only
  when their cleaned absolute form is equal to `root` or below it.
- Escapes through `..` are rejected with a filesystem access error. This
  applies to `fs` operations and script/module file loading paths that route
  through the interpreter-backed policy.
- `WithMaxFilesystemReadBytes` limits `fs.readfile` and source-side `fs.copy`
  reads; `WithMaxFilesystemWriteBytes` limits `fs.writefile`, `fs.appendfile`,
  and destination-side `fs.copy` writes.
- The current implementation performs lexical/absolute path cleaning. Full
  symlink-resolution policy, special-file denial, separate read/write roots,
  finer-grained operation permissions, and per-directory byte policy remain
  production roadmap items.
- When read and write access are both disabled, `fs`, `dofile`, and
  `loadfile` are removed from script
  globals. It does not by itself define the logical stdlib allowlist; use
  `WithLibs` for library visibility and `WithModuleLoading(false)` for
  filesystem-backed `require`.

Network policy:

- Default deny.
- Allow explicit schemes, hostnames, ports, CIDR ranges, and HTTP methods.
- Deny link-local, loopback, private, metadata-service, and Unix socket targets
  unless explicitly allowed.
- Enforce connect, request, response-header, and body-size limits.
- Control redirects by re-checking every redirect target.
- Provide optional DNS pinning or resolver injection for hosts that need it.

Process policy:

- Default deny.
- Allow exact executables or command specs, not arbitrary shell strings.
- `process.run`, `process.exec`, and `process.which` should be disableable as a
  group. Current public API exposes this as `WithProcessExecution(enabled)` and
  `SecurityPolicy.DisableProcessExecution`.
- `process.shell` should be disabled for production policies unless shell
  execution is explicitly needed. Current public API exposes this as
  `WithProcessShell(enabled)` and `SecurityPolicy.DisableProcessShell`.
- Environment default empty or allowlisted.
- Working directory must pass filesystem policy.
- Enforce timeout, stdout/stderr byte limits, stdin byte limits, and process
  tree cleanup on cancellation. `WithMaxHostResultBytes` currently bounds
  captured `process.run`, `process.exec`, and `process.shell` output.
- `os.exit` and `process.exit` return host-visible script exit errors and do
  not terminate the embedding process unless the CLI explicitly translates the
  error to an OS exit code.

OS/environment policy:

- `os.getenv`, `process.env`, and related APIs should use an environment
  allowlist. Current public API exposes this as
  `WithEnvironmentAllowlist(names...)` and
  `SecurityPolicy.EnvironmentAllowlist`.
- Time and randomness are safe by default, but deterministic sandboxes should
  allow injected clocks and random sources.

## Module Whitelist

Module loading should have two independent gates:

- Standard library modules allowed by name.
- Script modules allowed by module name and resolved file path.

Current public module-loading policy:

- Built-in stdlib `require(name)` is allowed only for modules present under the
  active `LibFlags` preset.
- `WithModuleLoading(false)` disables filesystem-backed `.gs` module loading
  while still allowing enabled built-in stdlib modules to be required.
- `WithSandbox()` sets `CapModuleLoading` off, so file modules cannot be loaded
  by `require`.
- `WithRequirePath(path)` chooses the base directory for relative file-module
  lookup. If `WithFilesystemRoot(root)` is also set, resolved file-module paths
  must remain inside `root`.
- `ExecFile` and `CompileFile` are host-side entry points. They read the file
  the embedder explicitly passed and are not treated as script-side filesystem
  permission grants.

Required controls:

- `AllowedStdlib`: exact set of stdlib names.
- `AllowedModules`: exact or pattern-based logical module names.
- `RequireRoots`: approved directories for `require` and `script.load`.
- `AllowRelativeRequire`: default true only within approved roots.
- `AllowDynamicEval`: controls string-based compile/eval. Current public API
  exposes this as `WithDynamicEval(enabled)` and
  `SecurityPolicy.DisableDynamicEval`.
- `MaxModuleBytes`: source size limit per module.
- `MaxModuleDepth`: nested require depth.

The current `WithLibs` API should stay as a compatibility layer. The security
policy should derive stdlib names from `LibFlags` when explicit module policy
is not supplied.

## Host Capability Model

Registered Go functions, struct bindings, and host tables are the largest
escape hatch. Production embedding should treat every host callback as a named
capability.

Core rules:

- A script can call only registered capabilities that are present in its
  policy.
- Capabilities receive a `CallContext` with deadline, tenant/script identity,
  audit sink, and resource budget.
- Capabilities declare effects: `pure`, `read_fs`, `write_fs`, `network`,
  `process`, `env`, `clock`, `random`, `debug`, `unsafe`.
- Arguments and return values are charged to memory budgets during conversion.
- Panics in capabilities are recovered and converted to `ErrHostPanic` unless
  the embedder explicitly opts into panic propagation.
- Capabilities can be revoked per VM. A function value captured before
  revocation must check the capability at call time.

Host callbacks should be registered through a secure wrapper:

```go
vm.RegisterCapability("tenant.lookupUser", gscript.Capability{
    Effects: []gscript.Effect{gscript.EffectNetwork},
    Fn: func(ctx gscript.CallContext, userID string) (User, error) {
        if err := ctx.Check(); err != nil {
            return User{}, err
        }
        return lookupUser(ctx.Context(), userID)
    },
})
```

Legacy `RegisterFunc`, `RegisterTable`, and struct binding APIs can continue to
exist, but sandboxed VMs should either reject them or wrap them as
`EffectUnsafeHost` unless the embedder supplies metadata.

## Panic and Error Isolation

No script input should be able to crash the host process through an unhandled
panic. Recovery is required at these boundaries:

- `Exec`, `ExecFile`, `Call`, `CallValue`, and `CallFunction`.
- Tree-walker dispatch.
- Bytecode VM execution.
- JIT entry, side exit, call exit, and continuation resume.
- Stdlib Go functions.
- Registered host capabilities.
- Goroutine/task entrypoints.

Recovered panics should become `ErrHostPanic` with a redacted message by
default. Debug builds may include stack traces in audit logs, but script-visible
errors should avoid leaking host paths, environment values, or secrets.

Protected calls must preserve language semantics: `pcall` and `xpcall` catch
script runtime errors and security errors. Internal sentinels such as coroutine
yield and process exit must not be mistaken for host panics.

## Audit Logging

Every security decision that denies, truncates, cancels, or forcefully cleans
up an operation should emit a structured audit event.

Minimum event fields:

- `time`
- `vm_id`
- `tenant_id`
- `script`
- `source`
- `operation`
- `capability`
- `decision`: `allow`, `deny`, `limit`, `cancel`, `panic`, `cleanup`
- `reason`
- `limit`
- `usage`
- `duration`
- `error_kind`

Events should be available through an embedder-supplied sink:

```go
type AuditSink interface {
    Emit(SecurityEvent)
}
```

Audit logging must not call back into the script VM. The sink should be
best-effort and non-blocking by default. A blocking audit backend can otherwise
turn denied operations into a denial-of-service path.

## API Draft

The API should make the safe path concise and the unsafe path explicit.

```go
package gscript

type SecurityPolicy struct {
    Identity IdentityPolicy
    Budget   BudgetPolicy
    Memory   MemoryPolicy
    Calls    CallPolicy
    Concur   ConcurrencyPolicy
    Modules  ModulePolicy
    FS       FilesystemPolicy
    Network  NetworkPolicy
    Process  ProcessPolicy
    Host     HostPolicy
    Audit    AuditSink

    RecoverPanics bool
    RedactErrors  bool
    AllowJIT      bool
}

type IdentityPolicy struct {
    VMID     string
    TenantID string
    ScriptID string
}

type BudgetPolicy struct {
    MaxInstructions int64
    MaxNativeCalls  int64
    Timeout         time.Duration
    CheckInterval   int64
}

type MemoryPolicy struct {
    MaxHeapBytes      int64
    MaxObjects        int64
    MaxStringBytes    int64
    MaxTableEntries   int64
    MaxArrayLength    int64
    MaxHostResultBytes int64
}

type CallPolicy struct {
    MaxCallDepth       int
    MaxNativeCallDepth int
    MaxMetaDepth       int
    MaxResumeDepth     int
    MaxDeferDepth      int
}

type ConcurrencyPolicy struct {
    MaxCoroutines       int
    MaxCoroutineBytes   int64
    MaxGoroutines       int
    MaxChannels         int
    MaxChannelCapacity  int
    MaxBlockedTasks     int
    AllowBackgroundTasks bool
}

type ModulePolicy struct {
    AllowedStdlib      []string
    AllowedModules     []string
    RequireRoots       []string
    AllowRelativeRequire bool
    AllowDynamicEval   bool
    MaxModuleBytes     int64
    MaxModuleDepth     int
}

type FilesystemPolicy struct {
    ReadRoots  []string
    WriteRoots []string
    TempRoots  []string
    MaxReadBytes  int64
    MaxWriteBytes int64
    AllowRemove   bool
    AllowRename   bool
    AllowSpecialFiles bool
}

type NetworkPolicy struct {
    AllowedSchemes []string
    AllowedHosts   []string
    AllowedCIDRs   []string
    AllowedPorts   []int
    AllowedMethods []string
    DenyPrivateNetworks bool
    MaxRequestBytes  int64
    MaxResponseBytes int64
    Timeout          time.Duration
}

type ProcessPolicy struct {
    AllowedCommands []CommandSpec
    AllowedEnv      []string
    AllowShell      bool
    MaxStdinBytes   int64
    MaxStdoutBytes  int64
    MaxStderrBytes  int64
    Timeout         time.Duration
}

type HostPolicy struct {
    AllowedCapabilities []string
    DenyUnsafeHost      bool
}

func WithSecurity(policy SecurityPolicy) Option
```

Execution should expose structured errors:

```go
type SecurityError struct {
    Kind       SecurityErrorKind
    Operation  string
    Capability string
    Limit      int64
    Usage      int64
    Source     SourceLocation
    Cause      error
}

const (
    ErrCPUExceeded SecurityErrorKind = "cpu_exceeded"
    ErrMemoryExceeded SecurityErrorKind = "memory_exceeded"
    ErrCallDepthExceeded SecurityErrorKind = "call_depth_exceeded"
    ErrCapabilityDenied SecurityErrorKind = "capability_denied"
    ErrModuleDenied SecurityErrorKind = "module_denied"
    ErrHostPanic SecurityErrorKind = "host_panic"
)
```

Module flags can bridge into the policy:

```go
vm := gscript.New(
    gscript.WithLibs(gscript.LibSafe),
    gscript.WithSecurity(gscript.SecurityPolicy{
        Modules: gscript.ModulePolicy{
            AllowedStdlib: gscript.StdlibNames(gscript.LibSafe),
        },
    }),
)
```

For host capabilities:

```go
type CallContext interface {
    Context() context.Context
    Check() error
    ChargeBytes(n int64) error
    ChargeNativeCall(name string) error
    Audit(event SecurityEvent)
    CapabilityAllowed(name string, effect Effect) bool
}

type Capability struct {
    Name    string
    Effects []Effect
    Fn      any
}

func (vm *VM) RegisterCapability(name string, cap Capability) error
func (vm *VM) RevokeCapability(name string)
```

## Enforcement Matrix

| Surface | Primary control | Enforcement point |
|---------|-----------------|-------------------|
| CPU loop | `MaxInstructions` | bytecode dispatch, tree statement boundary, JIT backedge |
| Wall time | `context.Context` | VM checks and blocking stdlib calls |
| Memory | `MemoryPolicy` | object allocation, table growth, string/buffer creation |
| Recursion | `CallPolicy` | call entry, metamethod entry, coroutine resume |
| Coroutine | `ConcurrencyPolicy` | `coroutine.create`, resume chains, child VM creation |
| Goroutine | `ConcurrencyPolicy` | script task spawn and host async wrappers |
| Channel | `ConcurrencyPolicy` | channel creation, send/receive, buffer allocation |
| Filesystem | `FilesystemPolicy` | `io`, `fs`, script/module file loading |
| Network | `NetworkPolicy` | `net`, `http`, host network capabilities |
| Process | `ProcessPolicy` | `process.run`, `exec`, `shell`, `exit` |
| Modules | `ModulePolicy` | stdlib registration, `require`, `script` APIs |
| Host callbacks | `HostPolicy` | `RegisterCapability`, reflected function wrappers |
| Panic isolation | `RecoverPanics` | all VM, JIT, stdlib, host callback boundaries |
| Audit | `AuditSink` | deny, limit, cancel, cleanup, panic |

## Rollout Plan

1. Introduce full `SecurityPolicy`, structured security errors, audit event
   types, and `WithSecurity` without changing existing defaults.
2. Thread a shared security runtime through tree-walker, bytecode VM, child
   coroutine VMs, stdlib registration, and host wrappers.
3. Enforce CPU, timeout, call-depth, meta-depth, coroutine, and module limits in
   interpreter paths. Disable JIT under sandbox until equivalent checks exist.
4. Add memory accounting to core allocation and host result conversion paths.
5. Move `io`, `fs`, `net`, `http`, `os`, `process`, `script`, `debug`, and
   `testkit` behind capability-aware stdlib adapters.
6. Add JIT budget polls and safe side-exit behavior, then allow JIT in sandbox
   only when tests prove parity with bytecode enforcement.
7. Add concurrency accounting for goroutines/channels and context cancellation
   cleanup.
8. Make production examples use `SecuritySandbox`; keep `LibAll` explicit for
   local development and trusted CLI workflows.

## Test Requirements

Security tests should be adversarial and shared across tree-walker, bytecode VM,
and JIT where applicable:

- Infinite loop exits by CPU budget.
- Deep recursion exits by call-depth budget without host panic.
- Metamethod recursion exits by meta-depth budget.
- Large table/string construction exits by memory budget.
- Coroutine and resume-chain limits are enforced across child VMs.
- Goroutine/channel creation limits are enforced and cancellation cleans up.
- `LibSafe` denies `io`, `fs`, `net`, `http`, `os`, `process`, `script`,
  `debug`, and `testkit`.
- Filesystem paths cannot escape allowlisted roots through `..` or symlinks.
- Network redirects are rechecked and private/metadata addresses are denied by
  default.
- `process.shell` is denied by default and command output is truncated by
  policy.
- Host callback panic becomes `ErrHostPanic`.
- Denied operations emit audit events with source location when available.
- `pcall` and `xpcall` can catch security errors as ordinary runtime errors.
