# Leia Documentation

Leia is a Go-native, AI-native, hot-reloadable scripting language with dynamic
semantics, an embeddable Go API, a bytecode VM, and an ARM64 JIT.

This directory is the current documentation surface. Older design notes,
compiler journals, audits, and pre-rename documents are archived under
[`archive/2026-06-pre-rewrite/`](archive/2026-06-pre-rewrite/) for reference
only.

## Start Here

- [Language specification](spec/language.md): the normative syntax and behavior
  contract for users, tooling, VM, JIT, and embedding APIs.
- [Getting started](tutorial/getting-started.md): install, run, test, and embed
  Leia in a small Go program.
- [Standard library](reference/stdlib/index.md): generated-style module index
  organized from the runtime catalog.
- [CLI reference](reference/cli/index.md): stable command surface.
- [File directives](reference/directives/index.md): `//leia:` metadata consumed
  by tooling.
- [Modules](reference/modules/index.md): `leia.mod`, `leia.sum`, vendoring, and
  Go-native binding metadata.
- [Concurrency](reference/concurrency/index.md): goroutines, channels, select,
  sync primitives, contexts, and concurrency budgets.
- [Data-oriented programming](reference/data-oriented/index.md): dense arrays,
  SoA layouts, masks, column kernels, and numeric performance model.
- [Go embedding API](reference/embedding/index.md): public Go package surface,
  host bindings, sandbox options, and VM lifetime rules.
- [Security and sandboxing](reference/security/index.md): library selection,
  host capabilities, resource budgets, and Go binding rules.
- [Hot reload](reference/hot-reload/index.md): loader handles, persistent
  instances, automatic state preservation, and rollback behavior.
- [Errors and diagnostics](reference/diagnostics/index.md): Go error types,
  CLI JSON/SARIF outputs, and diagnostic bundle entrypoints.
- [Performance and benchmarks](reference/performance/index.md): benchmark
  selectors, timing modes, strict guards, and release artifacts.
- [AI-native reference](reference/ai/index.md): models, tools, messages, turns,
  agents, budgets, providers, and replay.
- [Embedding guide](guides/embedding.md): host integration, sandboxing, and
  reload-oriented runtime usage.
- [AI-native guide](guides/ai-native.md): agents, tools, models, message
  history, and provider setup.
- [Testing and release gates](testing.md): correctness, docs, and release
  evidence.

## Documentation Policy

The docs are split into user-facing reference, tutorials, guides, release
process, and internals. Generated reference should come from code-owned metadata
where practical. Hand-written prose should explain concepts and tradeoffs, not
duplicate long API tables that the runtime can emit.
