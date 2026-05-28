# GScript

GScript is a Go-syntax scripting language and embeddable Go runtime with
Lua-like dynamic semantics, a bytecode VM, and an ARM64 JIT path.

## Run

```bash
go run ./cmd/gscript -e 'print("hello from gscript")'
go run ./cmd/gscript -vm path/to/script.gs
```

Use `-vm` for bytecode VM execution without native JIT. On Apple Silicon, the
default CLI path enables the VM plus JIT where supported.

## Embed In Go

```go
package main

import (
	"log"

	gs "github.com/gscript/gscript/gscript"
)

func main() {
	vm := gs.New(gs.WithLibs(gs.LibSafe))
	if err := vm.Exec(`print("hello from embedded gscript")`); err != nil {
		log.Fatal(err)
	}
}
```

For host functions, structs, pools, and API caveats, see
[docs/embedding.md](docs/embedding.md).

## Contributor Entrypoints

- Tests: [docs/testing-matrix.md](docs/testing-matrix.md)
  ```bash
  go test ./... -count=1 -p 1 -timeout=600s
  ```
- Performance: [docs/performance.md](docs/performance.md)
  ```bash
  python3 benchmarks/timing_compare.py --all-groups --runs=5 --warmup=1
  ```
- Diagnostics: [docs/tooling.md](docs/tooling.md)
  ```bash
  python3 benchmarks/diagnose.py --bench suite/fib --out-dir /tmp/gscript_diag
  ```
- Release gates: [docs/release.md](docs/release.md)
