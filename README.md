# Leia

Go-native scripting runtime with dynamic values, host capabilities, hot reload,
bytecode VM, ARM64 JIT, tagged DSLs, and q-style columnar analytics.

## Quick Start

```bash
go run ./cmd/leia help
go run ./cmd/leia eval 'print("hello from leia")'
go run ./cmd/leia run tests/smoke/01_basic.leia
go run ./cmd/leia run examples/hello/fib.leia
```

## Install

```bash
go install ./cmd/leia ./cmd/leia-lsp
leia version
leia run tests/smoke/01_basic.leia
```

## Small Scripts

```leia
func fib(n) {
    if n < 2 {
        return n
    }
    return fib(n - 1) + fib(n - 2)
}

print(fib(10))
```

## q-Style Analytics

```leia
q := require("q")

result := q.eval(`
    px:100 101 103 99 104f;
    sz:10 20 15 30 25;
    idx:where px>100;
    +/sz[idx]
`)

print(result)
```

```leia
q := require("q")
trades := q.sql("([] sym:`AAPL`MSFT`AAPL; px:190.5 410.0 191.2; sz:100 50 125)")

out := q.sql(trades, "select notional:sum px*sz by sym from trades")
print(out)
```

## Tagged Dialects

```leia
text := markdown`
# Release note
- q analytics uses typed runtime kernels
- host applications choose enabled capabilities
`

cmd := $`git status --short`
```

```leia
answer, err := turn {
    model: "fast"
    messages: {
        prompt { role: "user", text: "Summarize this in one sentence." }
    }
}
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

For untrusted scripts, start with `leia.SecuritySandbox()` and explicit
budgets.

## Tooling

```bash
go run ./cmd/leia fmt --check tests/smoke/01_basic.leia
go run ./cmd/leia lint tests/smoke/01_basic.leia
go run ./cmd/leia test tests/smoke/01_basic.leia
go run ./cmd/leia check --quick .
go run ./cmd/leia bench compare --bench data/q_operator_pipeline --runs 3
```

## Documentation

- [Documentation home](docs/index.md)
- [Language specification](docs/spec/index.md)
- [Language specification HTML](docs/spec/index.html)
- [Getting started](docs/tutorial/getting-started.md)
- [Embedding guide](docs/guides/embedding.md)
- [Modules](docs/reference/modules/index.md)
- [Packages and modules](docs/guides/packages.md)
- [CLI reference](docs/reference/cli/index.md)
- [Standard library](docs/reference/stdlib/index.md)
- [Tagged dialects](docs/reference/dialects/index.md)
- [Data-oriented programming](docs/reference/data-oriented/index.md)
- [q conformance matrix](docs/design/q-conformance.md)
- [Performance reference](docs/reference/performance/index.md)
- [AI dialect reference](docs/reference/ai/index.md)
- [Security policy](SECURITY.md)
- [Contributing](CONTRIBUTING.md)
- [Code of conduct](CODE_OF_CONDUCT.md)

## Project Status

Active development. Stable behavior is defined by spec, matrices, tests, and
release gates.

No license has been selected in this repository yet.
