# Getting Started

Leia can be used as a standalone script runner or embedded in a Go process.
From a checkout, run the CLI with `go run`:

```bash
go run ./cmd/leia help
go run ./cmd/leia eval 'print("hello from leia")'
go run ./cmd/leia run tests/smoke/01_basic.leia
```

Install from the module path when you want local development binaries:

```bash
go install github.com/never-labs/leia/cmd/leia@latest
go install github.com/never-labs/leia/cmd/leia-lsp@latest
leia version
```

After public binary releases exist, `scripts/install.sh` can install a
checksummed release artifact containing both `leia` and `leia-lsp`:

```bash
bash scripts/install.sh --version v0.1.0 --bin-dir "$HOME/bin" --dry-run
```

Use `--base-url` when installing from a release mirror or local artifact
fixture that contains the archive and `SHA256SUMS`:

```bash
bash scripts/install.sh --version v0.1.0 --base-url file:///tmp/leia-release --bin-dir "$HOME/bin"
```

## Run Examples

Start with the repository example entrypoints:

```bash
go run ./cmd/leia examples list
go run ./cmd/leia examples check examples/hello/fib.leia examples/hello/types_demo.leia
go run ./cmd/leia examples run repo-hello-fib
```

Then try examples for concurrency, data-oriented code, and q-style analytics:

```bash
go run ./cmd/leia examples run examples/concurrency/goroutines_channels.leia
go run ./cmd/leia examples run examples/data_processing/data_oriented/soa_kernels.leia
go run ./cmd/leia examples run repo-data-q_trade_analytics_project-main
```

Leia's larger examples emphasize DSLs: q-style analytics, shell/data/web
dialects, spreadsheets, and optional AI workflows. AI examples require a host
provider or replay fixture; keep API keys in environment variables, not source
files.

## Check Code

Use the local quality gate while editing:

```bash
go run ./cmd/leia check --quick .
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
  [data-oriented programming](../reference/data-oriented/index.md) plus
  [scientific numeric programming](../reference/scientific/index.md) for the
  main language extensions.
- Use the [AI dialect guide](../guides/ai-dialect.md) when a project needs model
  providers, tools, agents, or replay.
