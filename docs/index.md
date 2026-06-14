# Leia

Leia is a Go-native scripting language built for DSLs, dialects, and embedded
automation. It keeps the core language small, puts domain syntax behind tagged
dialects, and backs q-style analytics and numeric hot paths with measured
runtime and JIT gates.

```bash
go run ./cmd/leia eval 'print("hello from leia")'
go run ./cmd/leia playground --help
```

## Language

- [Language specification](spec/index.md)
- [Language overview](spec/language.md)
- [Grammar appendix](spec/grammar.ebnf)
- [Getting started](tutorial/getting-started.md)
- [Style guide](guides/style.md)
- [Editors and LSP](guides/editors.md)

## Core Runtime

- [Standard library](reference/stdlib/index.md)
- [CLI reference](reference/cli/index.md)
- [Go embedding API](reference/embedding/index.md)
- [Embedding guide](guides/embedding.md)
- [Modules](reference/modules/index.md)
- [Packages and modules](guides/packages.md)
- [File directives](reference/directives/index.md)
- [Security and sandboxing](reference/security/index.md)
- [Hot reload](reference/hot-reload/index.md)
- [Platforms and execution modes](reference/platforms/index.md)
- [Errors and diagnostics](reference/diagnostics/index.md)

## DSLs And Data

- [Tagged dialects](reference/dialects/index.md)
- [Data-oriented programming](reference/data-oriented/index.md)
- [q conformance matrix](design/q-conformance.md)
- [Performance and benchmarks](reference/performance/index.md)
- [Concurrency](reference/concurrency/index.md)

## AI Dialect

- [AI dialect reference](reference/ai/index.md)
- [AI dialect guide](guides/ai-dialect.md)
- [Evaluate reference](reference/evaluate/index.md)

## Examples And Release

- [Examples](examples/index.md)
- [Example tree README](https://github.com/never-labs/leia/blob/main/examples/README.md)
- [Cookbook](cookbook/index.md)
- [Tooling guide](guides/tooling.md)
- [Testing and release gates](testing.md)
- [Release process](release/index.md)
- [Release decisions](release/decisions.md)
- [Governance](governance.md)
- [Contributing](https://github.com/never-labs/leia/blob/main/CONTRIBUTING.md)
- [Security policy](https://github.com/never-labs/leia/blob/main/SECURITY.md)
- [Code of conduct](https://github.com/never-labs/leia/blob/main/CODE_OF_CONDUCT.md)
- [Performance contribution guide](contributing/performance.md)
