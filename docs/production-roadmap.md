# Leia Production Roadmap

This roadmap tracks the work needed to make Leia viable both as an embedded
Go scripting language and as a standalone programming language. It is organized
around release readiness rather than benchmark milestones.

## Release Goal

Leia should become a small, predictable, embeddable language with:

- stable language semantics and documented intentional differences from Lua;
- a public Go API that can be used without depending on `internal/*`;
- a standard library shaped around Go host capabilities;
- sandboxing and resource controls suitable for untrusted or semi-trusted code;
- repeatable tooling for formatting, testing, benchmarking, diagnostics, and
  release packaging;
- JIT performance treated as a guarded optimization layer, never as a semantic
  dependency;
- a release process with compatibility policy, CI gates, and user-facing docs.

## Workstreams

| Area | Primary doc | Release question |
|---|---|---|
| Language semantics | `docs/language-spec.md` | What does a valid Leia program mean, and where does it intentionally differ from Lua? |
| Go embedding API | `docs/embedding.md` | Can a Go host safely compile, run, call, bind, cancel, and sandbox scripts using stable public APIs? |
| Standard library | `docs/stdlib.md` | Which host capabilities are available, what are their contracts, and how are errors represented? |
| Security and isolation | `docs/security.md` | Can hosts bound CPU, memory, recursion, goroutines, IO, network, process execution, and module access? |
| Tooling | `docs/tooling.md` | Can users format, lint, test, benchmark, diagnose, and package Leia projects? |
| Performance and JIT | `docs/performance.md` | Are performance wins validated by correctness oracles and stable benchmark guardrails? |
| Release engineering | `docs/release.md` | Can the project ship versioned artifacts with documented compatibility and support policy? |

Supporting audits:

- `docs/api-audit.md` records the current public Go API surface and the
  internal concepts that should or should not become stable.
- `docs/test-matrix.md` maps correctness, official translated tests, stdlib
  tests, and benchmark oracles to release gates.
- `docs/benchmark-timing-audit.md` records the current timing harness risks and
  the path to removing unexplained `low_resolution` cells.
- `docs/cli-audit.md` records the current command-line surface and the command
  set expected from a standalone language.
- `docs/data-oriented-design.md` records the Odin-derived design slice for
  typed dense arrays, vectors, matrices, SoA layout, runtime specialization, and
  the kill features that keep the work out of parser/runtime scope until the
  semantics are ready.

## Phase Plan

### Phase 0: Freeze Language Semantics

Language specification is a hard dependency for every production-facing layer.
The embedding API cannot be stable if the behavior underneath it is implicit;
formatter/linter cannot be correct without grammar; sandboxing cannot be
defined without a capability and behavior surface.

- Maintain `docs/language-spec.md` as the source of truth for Leia grammar,
  operator precedence, statements, value behavior, errors, modules, tables,
  coroutines, channels, stdlib, and VM/JIT semantic gates.
- Treat parser behavior that is not written in the spec as implementation
  detail, not user contract.
- Map feature-matrix rows and official translated cases back to spec sections.
- Require spec-first changes for language-visible behavior.

Exit criteria:

- the language spec includes BNF/EBNF and behavior rules, not only roadmap text;
- each stable language feature has at least one test or explicit non-goal;
- formatter, linter, embedding, sandbox, and JIT work can cite spec sections.

### Phase 1: Specify APIs and Stabilize Tests

- Keep the language specification current as behavior moves from experimental
  to stable, and split detailed appendices only when the main spec becomes too
  large to review.
- Define the public embedding surface before adding more host-facing features.
- Convert the missing-capabilities ledger into Leia-native backlog items.
- Make official translated tests and Leia-specific tests part of the same
  correctness matrix.

Exit criteria:

- language/API/security/stdlib/tooling/performance/release docs exist and name
  owners, gates, and next tasks;
- public API gaps are listed separately from internal implementation debt;
- any known non-goals are explicit.

### Phase 2: Host API and Safety

- Introduce stable `leia` package APIs for engine creation, compilation,
  function calls, value conversion, module loading, and host function binding.
- Add context cancellation and resource budget plumbing that VM and JIT both
  honor.
- Define a capability-based standard-library loader so hosts opt into IO,
  network, process, and filesystem access.
- Add panic recovery and structured script error types suitable for production
  embedding.

Exit criteria:

- examples can embed Leia without importing internal packages;
- sandbox defaults deny filesystem/network/process unless explicitly granted;
- timeout/cancel tests cover interpreter, VM, and JIT paths.

### Phase 3: Toolchain and Testability

- Add a first-class test runner for `.leia` files with golden output and expected
  error support.
- Add formatter/linter decisions, even if initial implementation is minimal.
- Normalize benchmark harness timing sources and low-resolution handling.
- Make diagnostics bundle generation a supported CLI workflow.

Exit criteria:

- CI can run unit tests, official translated tests, stdlib tests, and benchmark
  smoke checks with one documented command set;
- benchmark reports clearly separate hot-loop script timing from process wall
  timing;
- all generated diagnostic artifacts are ignored or written to explicit output
  directories.

### Phase 4: Performance Guardrails

- Keep LuaJIT comparison as a benchmark reference, not a semantic target.
- Require every JIT optimization to have a correctness oracle, fallback
  contract, and regression benchmark when it touches observable behavior.
- Continue moving specialization toward runtime-discovered shapes rather than
  benchmark-specific kernels.
- Maintain full benchmark history and sorted gap reports for release candidates.

Exit criteria:

- full benchmark suite has a comparable result for every release-gate case or a
  documented reason why it is not comparable;
- performance CI blocks clear regressions in core and official hot cases;
- JIT can be disabled without changing program output.

### Phase 5: Release Readiness

- Define semantic versioning and compatibility guarantees.
- Publish installation/build instructions, embedding examples, CLI examples,
  stdlib reference, and migration notes.
- Produce cross-platform artifacts or clearly document supported targets.
- Attach correctness and performance reports to each release.

Exit criteria:

- a new user can install, run, embed, test, and debug Leia from docs alone;
- release notes identify compatibility changes and known limitations;
- supported platforms and unsupported JIT platforms are explicit.

## Immediate Next Tasks

1. Land the seven detailed roadmap docs named above. Done in this roadmap
   bundle.
2. Use `docs/production-readiness-checklist.md` as the release-gate index that
   maps docs to tests, benchmark commands, and CI candidates.
3. Use `docs/api-audit.md` to decide which internal concepts become public.
4. Use `docs/benchmark-timing-audit.md` to normalize official hot benchmark
   timing so no release table contains unexplained `low_resolution` cells.
5. Choose the first production feature implementation after the audit: public
   embedding API, sandbox capabilities, or test runner.
