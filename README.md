# Leia

Leia is a Go-native embedded scripting language with JIT execution, q-style in-memory analytics, and first-class extensible dialects.

- Go-native: small host API, Go-shaped syntax, and direct embedding in Go
  services.
- Performance-oriented: bytecode execution plus ARM64 JIT hot paths, with
  reproducible checks tracking LuaJIT-class workloads and typed runtime paths.
- Analytics-native: q-style vector syntax, qSQL, typed runtime kernels, and
  high-throughput in-memory columnar computation.
- Dialect-native: domain syntax lives in reusable tagged dialects, so embedders
  can add specialized languages without expanding the core.

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

The same language embeds directly in Go:

```go
package main

import leia "github.com/never-labs/leia"

func main() {
	vm := leia.New(leia.WithLibs(leia.LibSafe))
	script := "trades := dialect.eval(\"q\", \"flip `sym`px`qty!(`AAPL`MSFT`AAPL;100 101.5 100.75;10 12 8)\")\n" +
		"leader := q.sql(trades, \"select qty:sum qty, avg_px:avg px by sym from trades order by qty desc\")\n" +
		"print(leader[1].sym, leader[1].qty, leader[1].avg_px)"
	if err := vm.Exec(script); err != nil {
		panic(err)
	}
}
```

Use `leia.SecuritySandbox()` and explicit budgets for untrusted scripts.

## References

- [Documentation](docs/index.md)
- [Language specification](docs/spec/index.md)
- [Playground](docs/playground.md)
- [CLI reference](docs/reference/cli/index.md)
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
