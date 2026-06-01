# Getting Started

Leia can be used as a standalone script runner or embedded in a Go process.
From a checkout, run the CLI with `go run`:

```bash
go run ./cmd/leia help
go run ./cmd/leia eval 'print("hello from leia")'
go run ./cmd/leia run tests/smoke/01_basic.leia
```

## Run Examples

Start with small scripts:

```bash
go run ./cmd/leia run examples/hello/fib.leia
go run ./cmd/leia run examples/hello/types_demo.leia
```

Then try the product-direction examples:

```bash
go run ./cmd/leia run examples/concurrency/goroutines_channels.leia
go run ./cmd/leia run examples/data_processing/data_oriented/soa_kernels.leia
```

AI examples require a host provider or model configuration. Keep API keys in
environment variables, not source files.

## Check Code

Use the local quality gate while editing:

```bash
go run ./cmd/leia check --no-docs .
go test ./...
```

Useful focused commands:

```bash
go run ./cmd/leia fmt --check tests/smoke/01_basic.leia
go run ./cmd/leia lint tests/smoke/01_basic.leia
go run ./cmd/leia test tests/smoke/01_basic.leia
go run ./cmd/leia bench --quick
```

## Embed In Go

```go
package main

import leia "github.com/never-labs/leia"

func main() {
	vm := leia.New(leia.WithLibs(leia.LibSafe))
	if err := vm.Exec(`print("hello from embedded leia")`); err != nil {
		panic(err)
	}
}
```

For untrusted scripts, start from the security preset:

```go
vm := leia.New(
	leia.SecuritySandbox(),
	leia.WithMaxSteps(100_000),
	leia.WithMaxNativeCalls(1_000),
)
```

## Hot Reload

Hosts can reload script code without restarting the Go process:

```go
loader := leia.NewHotLoader(leia.WithHotLoaderVMOptions(leia.WithVM()))
inst, err := loader.LoadInstance("logic.leia")
if err != nil {
	return err
}
_, err = inst.Call("tick")
_, _ = inst.ReloadIfChanged()
```

`HotInstance` preserves ordinary script state where compatible and rolls back
failed reloads.

## Project Modules

Create module metadata when a project has dependencies, vendored packages, or
capability summaries:

```bash
go run ./cmd/leia mod init --module github.com/example/tool
go run ./cmd/leia mod list --json
go run ./cmd/leia mod verify --json
```

Next steps:

- Read the [language specification](../spec/index.md).
- Browse the [standard-library index](../reference/stdlib/index.md).
- Use the [embedding reference](../reference/embedding/index.md) and
  [security reference](../reference/security/index.md) for host integration.
- Use [hot reload](../reference/hot-reload/index.md) for long-running hosts.
- Use [concurrency](../reference/concurrency/index.md) and
  [data-oriented programming](../reference/data-oriented/index.md) for the
  main language extensions.
- Use [AI-native Leia](../guides/ai-native.md) for model providers and agents.
