# Leia

Leia is a Go-native, AI-native, hot-reloadable scripting language. It uses
Go-flavored syntax with dynamic values, Lua-compatible table and multi-return
semantics where useful, an embeddable Go API, a bytecode VM, and an ARM64 JIT.

Leia is designed for host applications that need scriptability without giving up
Go operational habits: explicit capabilities, small deployment surface,
repeatable tests, package metadata, and source-level hot reload.

The JIT accelerates supported hot paths and falls back to the VM/runtime for
unsupported operations. The language contract is the shared interpreter/VM
semantics enforced by the spec, feature matrix, and release gates.

## Quick Start

From a checkout:

```bash
go run ./cmd/leia help
go run ./cmd/leia eval 'print("hello from leia")'
go run ./cmd/leia run tests/smoke/01_basic.leia
go run ./cmd/leia run examples/hello/fib.leia
```

Install the CLI and language server from a checkout:

```bash
go install ./cmd/leia ./cmd/leia-lsp
leia version
leia run tests/smoke/01_basic.leia
```

## What It Includes

- Go embedding API with sandbox, resource budgets, host bindings, and hot reload.
- AI-native syntax and stdlib support for models, tools, messages, turns,
  agents, replay, and provider adapters.
- DSL-native tagged dialects for shell commands, data formats, web routes,
  q-style analytics, spreadsheets, and AI workflows.
- Go-style concurrency primitives: `go`, channels, `select`, sync helpers, and
  cancellation-oriented host integration.
- Data-oriented helpers for dense arrays, matrices, vectors, and SoA layouts.
- CLI tooling for format, lint, test, docs, diagnostics, modules, benchmarks,
  and release evidence.

## Tooling

```bash
go run ./cmd/leia fmt --check tests/smoke/01_basic.leia
go run ./cmd/leia lint tests/smoke/01_basic.leia
go run ./cmd/leia test tests/smoke/01_basic.leia
go run ./cmd/leia check --no-docs --no-editor --no-examples .
go run ./cmd/leia examples check examples/hello/fib.leia examples/hello/types_demo.leia examples/hello/dialects.leia
go run ./cmd/leia doc check
go run ./cmd/leia mod verify --json examples/ui/package_managed
go run ./cmd/leia bench compare --bench numeric/mandelbrot --runs 3 --warmup 1
go run ./cmd/leia diag bundle --output /tmp/leia-diag --skip-benchmarks
go run ./cmd/leia ci release --list
```

Use `leia check .` before submitting changes. It runs formatting, linting,
manifest checks, tests, documentation checks, editor asset checks, and runnable
example checks unless a skip flag is supplied. Use `leia check --quick .` for a
fast local loop that skips the slower release-evidence steps.

## Embedding

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

For untrusted scripts, start with `leia.SecuritySandbox()` and explicit budgets.
Host APIs and Go bindings should be added deliberately through the embedding
surface.

## Documentation

Start with:

- [Documentation home](docs/index.md)
- [Language specification](docs/spec/index.md)
- [Language specification HTML](docs/spec/index.html)
- [Getting started](docs/tutorial/getting-started.md)
- [Standard library](docs/reference/stdlib/index.md)
- [Tagged dialects](docs/reference/dialects/index.md)
- [Data-oriented programming](docs/reference/data-oriented/index.md)
- [CLI reference](docs/reference/cli/index.md)
- [Embedding guide](docs/guides/embedding.md)
- [Package guide](docs/guides/packages.md)
- [Modules reference](docs/reference/modules/index.md)
- [AI-native guide](docs/guides/ai-native.md)
- [Security reference](docs/reference/security/index.md)
- [Performance reference](docs/reference/performance/index.md)
- [Platforms and execution modes](docs/reference/platforms/index.md)
- [Examples](docs/examples/index.md)
- [Security policy](SECURITY.md)
- [Contributing](CONTRIBUTING.md)
- [Code of conduct](CODE_OF_CONDUCT.md)

## Project Status

Leia is under active development. The stable contract is the language spec plus
feature matrix and release gates. Experimental behavior should be documented as
such before users depend on it.

No license has been selected in this repository yet. Do not assume redistribution
or production adoption rights until a root `LICENSE` file is added.
