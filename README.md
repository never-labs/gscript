# Leia

Leia is a Go-native scripting language built for DSLs, dialects, and embedded automation.

- Go-native: small host API, Go-shaped syntax, and direct embedding in Go
  services.
- Performance-oriented: bytecode execution plus ARM64 JIT hot paths, with
  release gates tracking LuaJIT-class workloads and VM equivalence.
- Analytics-native: q-style vector syntax, qSQL, typed runtime kernels, and
  high-throughput in-memory columnar computation.
- Dialect-native: `q`, `sql`, `json`, `yaml`, `prompt`, `quote`, `llm`, and
  host-defined dialects extend the language without expanding the core.

The goal is a small language core with specialized runtime backends. Supported
columnar and numeric hot paths are benchmarked against handwritten Go baselines
and external runtimes; performance claims stay tied to reproducible gates.

## Example

````leia
prices := q`100 101.5 100.75 102.25`
sizes := q`100 120 80 150`
notional := q`100 101.5 100.75 102.25 * 100 120 80 150`
total := q`+/10000 12180 8060 15337.5`

trades := q.eval("flip `sym`price`size`notional!(`AAPL`MSFT`AAPL`NVDA;100 101.5 100.75 102.25;100 120 80 150;10000 12180 8060 15337.5)")
leaders := q.sql(trades, "select notional:sum notional, fills:count i by sym from trades order by notional desc")

note := prompt`Top symbol ${leaders[1].sym}; notional ${leaders[1].notional}; total ${total}.`
print(note.text)
````

Tagged forms such as `q`, `json`, `sql`, `prompt`, and `quote` are ordinary
extension points. AI support lives in dialects and libraries, not in the core
language runtime.

## Embedding

```go
package main

import leia "github.com/never-labs/leia"

func main() {
	vm := leia.New(leia.WithLibs(leia.LibSafe))
	if err := vm.Exec(`print(q` + "`+/1 2 3`" + `)`); err != nil {
		panic(err)
	}
}
```

Use `leia.SecuritySandbox()` and explicit budgets for untrusted scripts.

## Tooling

```bash
go run ./cmd/leia eval 'print(q`+/1 2 3`)'
go run ./cmd/leia examples check examples/hello/dialects.leia
go run ./cmd/leia bench compare --bench data/q_operator_pipeline --runs 3
go run ./cmd/leia doc check
```

## References

- [Language specification](docs/spec/index.md)
- [Embedding](docs/guides/embedding.md)
- [Modules](docs/reference/modules/index.md)
- [Packages and modules](docs/guides/packages.md)
- [Tagged dialects](docs/reference/dialects/index.md)
- [Data-oriented programming](docs/reference/data-oriented/index.md)
- [q analytics](docs/design/q-conformance.md)
- [Performance](docs/reference/performance/index.md)
- [AI dialect](docs/reference/ai/index.md)
- [Security](SECURITY.md)
- [Contributing](CONTRIBUTING.md)
- [Code of conduct](CODE_OF_CONDUCT.md)

No license has been selected in this repository yet.
