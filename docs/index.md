---
layout: home
title: Leia
---

# Leia

Leia is a general-purpose scripting language designed to run standalone or
inside Go applications. It combines a compact, Go-shaped syntax with dynamic
values, modules, concurrency, an embeddable VM, and opt-in domain dialects.

````leia
rows := [
    {sym: "AAPL", px: 100.0, qty: 10},
    {sym: "MSFT", px: 101.5, qty: 12},
    {sym: "AAPL", px: 100.75, qty: 8},
]

total := 0
for _, r := range ipairs(rows) {
    if r.sym == "AAPL" {
        total += r.qty
    }
}
print(total)
````

## What It Is

- A Go-embedded scripting runtime with Go-shaped syntax and a small host API.
- Native DSL extension through tagged dialects, so domain syntax can live
  beside Leia code without expanding the core grammar.
- High-throughput in-memory data primitives for vectors, frames, matrices, and
  typed runtime/JIT work.
- A practical automation language for services, tools, data pipelines, and
  embedded application logic.

## Start Here

- [Language specification](spec/index.md)
- [Playground](playground.md)
- [Getting started](tutorial/getting-started.md)
- [CLI reference](reference/cli/index.md)
- [Go embedding API](reference/embedding/index.md)
- [Embedding guide](guides/embedding.md)
- [Examples](examples/index.md)

## Language

- [Language overview](spec/language.md)
- [Grammar appendix](spec/grammar.ebnf)
- [Standard library](reference/stdlib/index.md)
- [Modules](reference/modules/index.md)
- [Packages and modules](guides/packages.md)
- [Concurrency](reference/concurrency/index.md)
- [Errors and diagnostics](reference/diagnostics/index.md)

## Dialects And Data

- [Tagged dialects](reference/dialects/index.md)
- [Data-oriented programming](reference/data-oriented/index.md)
- [Scientific numeric programming](reference/scientific/index.md)
- [Performance and benchmarks](reference/performance/index.md)
- [Evaluate reference](reference/evaluate/index.md)
- [Optional LLM dialect reference](reference/ai/index.md)
- [Optional LLM dialect guide](guides/ai-dialect.md)

## Runtime And Tooling

- [File directives](reference/directives/index.md)
- [Security and sandboxing](reference/security/index.md)
- [Hot reload](reference/hot-reload/index.md)
- [Platforms and execution modes](reference/platforms/index.md)
- [Tooling guide](guides/tooling.md)
- [Script entrypoints](design/script-entrypoints.md)
- [Editors and LSP](guides/editors.md)
- [Testing and release validation](testing.md)

## Project

- [Cookbook](cookbook/index.md)
- [Example tree README](https://github.com/never-labs/leia/blob/main/examples/README.md)
- [Release process](release/index.md)
- [Release decisions](release/decisions.md)
- [Governance](governance.md)
- [Contributing](https://github.com/never-labs/leia/blob/main/CONTRIBUTING.md)
- [Security policy](https://github.com/never-labs/leia/blob/main/SECURITY.md)
- [Code of conduct](https://github.com/never-labs/leia/blob/main/CODE_OF_CONDUCT.md)
- [Performance contribution guide](contributing/performance.md)
