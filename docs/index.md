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
- [Go embedding API](reference/embedding/index.md): public Go package surface,
  host bindings, sandbox options, and VM lifetime rules.
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
