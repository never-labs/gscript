# Getting Started

Run the CLI from a checkout:

```bash
go run ./cmd/leia help
go run ./cmd/leia eval 'print("hello from leia")'
go run ./cmd/leia run tests/smoke/01_basic.leia
```

Run the local quality gate:

```bash
go run ./cmd/leia check --no-docs .
go test ./...
```

Embed Leia in Go:

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

Next steps:

- Read the [language specification](../spec/language.md).
- Browse the [standard-library index](../reference/stdlib/index.md).
- Use [embedding](../guides/embedding.md) for host integration.
- Use [AI-native](../guides/ai-native.md) for model providers and agents.

