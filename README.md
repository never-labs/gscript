# Leia

Leia is a Go-native, AI-native, hot-reloadable scripting language. It uses
Go-flavored syntax with dynamic values, Lua-compatible table and multi-return
semantics where useful, an embeddable Go API, a bytecode VM, and an ARM64 JIT.

Leia is designed for host applications that need scriptability without giving up
Go operational habits: explicit capabilities, small deployment surface,
repeatable tests, package metadata, and source-level hot reload.

## Quick Start

From a checkout:

```bash
go run ./cmd/leia help
go run ./cmd/leia eval 'print("hello from leia")'
go run ./cmd/leia run tests/smoke/01_basic.leia
go run ./cmd/leia run examples/hello/fib.leia
```

Install the CLI from the module path when publishing from a tag or commit:

```bash
go install github.com/never-labs/leia/cmd/leia@latest
leia version
leia run path/to/script.leia
```

## What It Includes

- Go embedding API with sandbox, resource budgets, host bindings, and hot reload.
- AI-native syntax and stdlib support for models, tools, messages, turns,
  agents, replay, and provider adapters.
- Go-style concurrency primitives: `go`, channels, `select`, sync helpers, and
  cancellation-oriented host integration.
- Data-oriented helpers for dense arrays, matrices, vectors, and SoA layouts.
- CLI tooling for format, lint, test, docs, diagnostics, modules, benchmarks,
  and release evidence.

## Tooling

```bash
go run ./cmd/leia fmt --check tests/smoke/01_basic.leia
go run ./cmd/leia lint tests/smoke/01_basic.leia
go run ./cmd/leia test tests/smoke
go run ./cmd/leia check --no-docs .
go run ./cmd/leia bench compare --bench numeric/mandelbrot --runs 3 --warmup 1
go run ./cmd/leia diag bundle --output /tmp/leia-diag --skip-benchmarks
```

Use `leia check .` before submitting changes. It runs formatting, linting,
manifest checks, tests, and documentation checks unless a skip flag is supplied.

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
- [Language specification](docs/spec/language.md)
- [Getting started](docs/tutorial/getting-started.md)
- [Standard library](docs/reference/stdlib/index.md)
- [CLI reference](docs/reference/cli/index.md)
- [Embedding guide](docs/guides/embedding.md)
- [AI-native guide](docs/guides/ai-native.md)
- [Security reference](docs/reference/security/index.md)
- [Performance reference](docs/reference/performance/index.md)
- [Examples](docs/examples/index.md)
- [Security policy](SECURITY.md)
- [Contributing](CONTRIBUTING.md)

## Project Status

Leia is under active development. The stable contract is the language spec plus
feature matrix and release gates. Experimental behavior should be documented as
such before users depend on it.

No license has been selected in this repository yet. Do not assume redistribution
or production adoption rights until a root `LICENSE` file is added.
