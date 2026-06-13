# Leia

Go-native scripting runtime with tagged DSLs, q-style columnar analytics,
bytecode execution, ARM64 JIT, and an embeddable Go API.

```bash
go run ./cmd/leia eval 'print("hello from leia")'
go run ./cmd/leia playground --help
```

## Language

- [Language specification](spec/index.md)
- [Language specification HTML](spec/index.html)
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
- [AI dialect guide](guides/ai-native.md)
- [Evaluate reference](reference/evaluate/index.md)

## Examples And Release

- [Examples](examples/index.md)
- [Example tree README](../examples/README.md)
- [Cookbook](cookbook/index.md)
- [Tooling guide](guides/tooling.md)
- [Testing and release gates](testing.md)
- [Release process](release/index.md)
- [Governance](governance.md)
- [Contributing](../CONTRIBUTING.md)
- [Security policy](../SECURITY.md)
- [Code of conduct](../CODE_OF_CONDUCT.md)
- [Performance contribution guide](contributing/performance.md)
