# Leia

Leia is a Go-native scripting language built for DSLs, dialects, and embedded automation.

- Go-native: small host API, Go-shaped syntax, and direct embedding in Go
  services.
- Performance-oriented: bytecode execution plus ARM64 JIT hot paths, with
  reproducible checks tracking LuaJIT-class workloads and VM equivalence.
- Analytics-native: q-style vector syntax, qSQL, typed runtime kernels, and
  high-throughput in-memory columnar computation.
- Dialect-native: `q`, `sql`, `json`, `yaml`, shell/data tags, and
  host-defined dialects extend the language without expanding the core.

The goal is a small language core with specialized runtime backends. Supported
columnar and numeric hot paths are benchmarked against handwritten Go baselines
and external runtimes. Performance claims are benchmark-bound. JIT paths must preserve VM/runtime semantics.

## Example

````leia
trades := q```flip `sym`px`qty!(`AAPL`MSFT`AAPL;100 101.5 100.75;10 12 8)```
leader := q.sql(trades, "select qty:sum qty, avg_px:avg px by sym from trades order by qty desc")

card := json`{"symbol": "${leader[1].sym}", "qty": ${leader[1].qty}, "avg_px": ${leader[1].avg_px}}`
note := prompt`Review ${card.symbol}: ${card.qty} shares at avg ${card.avg_px}.`
print(note.text)
````

Tagged forms such as `q`, `json`, `sql`, `prompt`, and `quote` are ordinary
extension points. Optional LLM support lives in dialects and libraries, not in
the core language runtime.

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
go run ./cmd/leia eval 'print(1 + 2 + 3)'
go run ./cmd/leia examples check examples/hello/dialects.leia
go run ./cmd/leia bench compare --bench data/q_operator_pipeline --runs 3
go run ./cmd/leia doc check
```

## References

- [Documentation](docs/index.md)
- [Language specification](docs/spec/index.md)
- [CLI and playground](docs/reference/cli/index.md)
- [Embedding](docs/guides/embedding.md)
- [Modules](docs/reference/modules/index.md)
- [Packages and modules](docs/guides/packages.md)
- [Tagged dialects](docs/reference/dialects/index.md)
- [Data-oriented programming](docs/reference/data-oriented/index.md)
- [q analytics](docs/design/q-conformance.md)
- [Performance](docs/reference/performance/index.md)
- [Optional LLM dialect](docs/reference/ai/index.md)
- [Security](SECURITY.md)
- [Contributing](CONTRIBUTING.md)
- [Code of conduct](CODE_OF_CONDUCT.md)
