# Leia

Leia is an embeddable scripting language for Go systems.

Go-like syntax. Go-style syntax where it matters: blocks, functions, control
flow, and host-facing code. A small host API. Runtime work focuses on
typed hot-path optimization. Leia has native q-style columnar analytics for
high-performance in-memory data. Its tagged dialects are the language
extension mechanism.

The runtime includes interpreter and bytecode execution, with an ARM64 JIT path
for supported hot code.

Performance claims are benchmark-bound. Release gates compare supported hot
workloads against the configured LuaJIT baseline where a useful reference
exists, and JIT paths must preserve VM/runtime semantics.

## Surface

````leia
name := "Leia"

interpolated := "hello ${name}"
single_quoted := 'hello ${name}'
go_raw := `hello ${name}`
vector := q`1 2 3`

trades := q```
flip `sym`price`size!(
  `AAPL`MSFT`AAPL`NVDA;
  100 101.5 100.75 102.25;
  100 120 80 150
)
```

rollup := q.sql(
    "select notional:sum price*size, fills:count i by sym from trades order by notional desc",
    {trades: trades}
)

model {
    default: "claude"
}

summarize := agent {
    name: "market_summary"
    model: "claude"
    instructions: prompt { role: "system", text: "Write concise market notes." }
    params: {"table"}
    output: {summary: "short"}
}

result, err := summarize(rollup)
if err != nil {
    print(err.message)
} else {
    print(result.value.summary)
}
````

String rules are deliberately simple:

- `"..."` processes escapes and `${expr}` interpolation.
- `'...'` processes escapes and never interpolates.
- `` `...` `` is Go-style raw text and never interpolates.
- <code>q`...`</code> and <code>q```...```</code> are tagged dialect forms; `q`
  receives the embedded source and returns ordinary Leia values.

AI is a dialect/stdlib layer, not an AI-native runtime or the language core.

```leia
answer, err := turn {
    model: "claude"
    messages: {prompt { role: "user", text: "Summarize this table." }}
    max_tokens: 64
}
```

Host dialects are opt-in capabilities; a shell form such as
<code>cmd := $`git status --short`</code> is syntax, not ambient permission.

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

Use `leia.SecuritySandbox()` and explicit budgets for untrusted scripts.

## Tooling

```bash
go run ./cmd/leia fmt --check tests/smoke/01_basic.leia
go run ./cmd/leia lint tests/smoke/01_basic.leia
go run ./cmd/leia test tests/smoke/01_basic.leia
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
