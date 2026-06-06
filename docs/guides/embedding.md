# Embedding Leia

The public Go import path is:

```go
import leia "github.com/never-labs/leia"
```

The root package is the embedding API. It should stay small: VM construction,
execution, module options, sandbox/resource controls, public value conversion,
hot reload handles, and stable error types belong here. Compiler internals,
runtime tables, and stdlib binding code remain under `internal/`.

Minimal host:

```go
vm := leia.New(leia.WithLibs(leia.LibSafe))
err := vm.Exec(`print("hello")`)
```

## VM Lifetime

`VM` is mutable and not goroutine-safe. Use one VM per request, session, or
script instance. For concurrent host workloads, create multiple VMs or use
`Pool`.

Compile reusable code once:

```go
prog, err := leia.Compile(`func add(a, b) { return a + b }`)
if err != nil {
	return err
}

vm := leia.New(leia.WithVM())
if err := vm.Run(prog); err != nil {
	return err
}
out, err := vm.Call("add", 2, 3)
```

Do not run the same `Program` concurrently.

## Sandbox First

Sandboxed hosts should start from `SecuritySandbox()`, then grant only the
modules, capabilities, and budgets the script needs.

```go
vm := leia.New(
	leia.SecuritySandbox(),
	leia.WithLibs(leia.LibSafe|leia.LibFS),
	leia.WithFilesystemRoot("/srv/app/scripts"),
	leia.WithFilesystemRead(true),
	leia.WithFilesystemWrite(false),
	leia.WithMaxSteps(100_000),
	leia.WithMaxNativeCalls(1_000),
)
```

File, environment, module loading, native call, step, goroutine, channel, and
host-result-size budgets are separate controls. See the
[security reference](../reference/security/index.md).

A production embedding usually combines the sandbox, budgets, explicit host
bindings, and hot reload in one place:

```go
loader := leia.NewHotLoader(leia.WithHotLoaderVMOptions(
	leia.SecuritySandbox(),
	leia.WithLibs(leia.LibSafe),
	leia.WithGoImports(map[string]any{
		"go:host/safe": leia.Module{
			"label": safeLabel,
		},
	}),
	leia.WithMaxSteps(100_000),
	leia.WithMaxNativeCalls(1_000),
	leia.WithMaxHostResultBytes(1<<20),
))

inst, err := loader.LoadInstance("policy.leia")
```

The script can import only the allowlisted `go:host/safe` module. Reloading the
`HotInstance` swaps changed function bodies while preserving compatible script
state.

## Host Modules

Expose Go capabilities explicitly:

```go
type SafeMath struct{}

func (SafeMath) Double(n int) int { return n * 2 }

mod, err := leia.ModuleFrom(SafeMath{})
if err != nil {
	return err
}

vm := leia.New()
if err := vm.RegisterModule("safe/math", mod); err != nil {
	return err
}
```

`WithGoImports` allows `require("go:...")` only for host-provided entries:

```go
vm := leia.New(leia.WithGoImports(map[string]any{
	"go:example.com/app/safe": SafeBinding{},
}))
```

Leia does not reflect arbitrary Go packages by import path at runtime.

## Module-Aware Hosts

Project hosts can load module options from `leia.mod`:

```go
opts := leia.ModuleOptionsForScript("scripts/main.leia")
vm := leia.New(opts...)
```

Use readonly or vendor module mode for reproducible deployment:

```go
vm := leia.New(
	leia.WithModuleMode(leia.ModuleModeVendor),
	leia.WithModuleCache("./vendor"),
)
```

## Hot Reload

Hot reload should preserve state by default where compatible. Source changes
that alter incompatible state layout or active concurrency may require explicit
host policy.

Use `HotInstance` for a persistent script instance:

```go
loader := leia.NewHotLoader(leia.WithHotLoaderVMOptions(leia.WithVM()))
inst, err := loader.LoadInstance("logic.leia")
if err != nil {
	return err
}

_, err = inst.Call("tick")
if result, err := inst.ReloadIfChanged(); err == nil && result.Changed {
	_, err = inst.Call("tick")
}
```

Use `HotLoader` plus `ModuleHandle` when each request owns its own VM. See the
[hot reload reference](../reference/hot-reload/index.md).

## Cancellation

Prefer context-aware host entrypoints in services:

```go
ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
defer cancel()

err := vm.ExecContext(ctx, source)
```

Script-level `context` tables are separate values for script APIs such as
`time.sleep(ctx, seconds)` and `process.run(ctx, args)`.
