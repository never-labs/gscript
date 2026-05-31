# GScript

GScript is a Go-syntax scripting language and embeddable Go runtime with
Lua-like dynamic semantics, a bytecode VM, and an ARM64 JIT path.

```bash
go run ./cmd/gscript help
go run ./cmd/gscript version --json
go run ./cmd/gscript eval 'print("hello from gscript")'
go run ./cmd/gscript run path/to/script.gs
```

## Tooling

```bash
go run ./cmd/gscript check --json tests/01_basic.gs
go run ./cmd/gscript bench compare --bench numeric/mandelbrot --runs 3 --warmup 1
go run ./cmd/gscript diag bundle --output /tmp/gscript-diag --skip-benchmarks
go run ./cmd/gscript ci smoke --list
```

## Embedding

```go
package main

import gs "github.com/never-labs/gscript"

func main() {
	vm := gs.New(gs.WithLibs(gs.LibSafe))
	if err := vm.Exec(`print("hello from embedded gscript")`); err != nil {
		panic(err)
	}
}
```

See [docs/embedding.md](docs/embedding.md), [docs/tooling.md](docs/tooling.md),
and [docs/language-spec.md](docs/language-spec.md).
