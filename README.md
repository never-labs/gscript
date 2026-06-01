# Leia

Leia is a Go-syntax scripting language and embeddable Go runtime with
Lua-like dynamic semantics, a bytecode VM, and an ARM64 JIT path.

```bash
go run ./cmd/leia help
go run ./cmd/leia version --json
go run ./cmd/leia eval 'print("hello from leia")'
go run ./cmd/leia run path/to/script.leia
```

## Tooling

```bash
go run ./cmd/leia check --json tests/01_basic.leia
go run ./cmd/leia bench compare --bench numeric/mandelbrot --runs 3 --warmup 1
go run ./cmd/leia diag bundle --output /tmp/leia-diag --skip-benchmarks
go run ./cmd/leia ci smoke --list
```

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

See [docs/embedding.md](docs/embedding.md), [docs/tooling.md](docs/tooling.md),
and [docs/language-spec.md](docs/language-spec.md).
