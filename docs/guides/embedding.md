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

Sandboxed hosts should start from `LibSafe`, disable ambient host capabilities,
and grant only the modules and resources the script needs. File, environment,
module loading, native call, step, and output budgets are separate controls.

Hot reload should preserve state by default where compatible. Source changes
that alter incompatible state layout or active concurrency may require explicit
host policy.

