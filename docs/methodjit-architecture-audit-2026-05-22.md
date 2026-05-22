# MethodJIT Architecture Audit - 2026-05-22

This audit focuses on future iteration speed: adding language features safely,
and adding JIT optimizations without repeatedly touching unrelated compiler
layers.

## Executive Summary

The MethodJIT architecture has a much stronger optimizer skeleton than before:
Tier2 modules are registered through phases, modules declare analysis-fact
contracts, diagnostics record module contracts and reasons, and compilation
dependencies are now recorded by selected production passes and checked before
cached Tier2 code is reused.

The remaining bottleneck is not the phase pipeline. The main bottleneck is that
the compiler still has several central "god surfaces":

- `internal/methodjit/ir_ops.go`: every IR op is centralized in one enum and one
  name table.
- `internal/methodjit/analysis_result.go`: every analysis fact payload lives in
  one large struct.
- `internal/methodjit/emit_dispatch.go`: codegen dispatch is one large switch
  that every new op must touch.
- `internal/methodjit/ir.go`: `Function` owns many domain-specific payload
  slices and caches.
- `internal/methodjit/emit_call_native.go`, `pass_fixed_shape_table.go`,
  `emit_compile.go`, `specialized_abi.go`, `regalloc.go`: very large files that
  concentrate unrelated responsibilities.

This means the optimizer pipeline is modular, but the IR/codegen/analysis data
model is still mostly monolithic. Future language and JIT work will remain slow
unless the next refactor targets these surfaces.

## Current Strengths

### Optimizer Pipeline

The Tier2 pipeline is now a real extension point:

- `Tier2OptimizerPlan.PhaseGroups` is the canonical execution shape.
- `ModuleRegistry` supports explicit construction and injection.
- `Tier2ValidatedOptimizerPlan` supports validated plan reuse.
- `Tier2OptimizerFeatureFlags` supports phase/module disabling.
- `Tier2OptimizerModule` has `Requires`, `Provides`, and `Updates`.
- `ValidateDependencyOrder` catches ordering and contract violations.

This is the right direction. New optimizer passes can be inserted without
editing the primary pipeline function.

### Diagnostics

Diagnostics are now much closer to what a compiler needs:

- `PhaseScope` records module timing, snapshots, errors, and module runs.
- `Tier2ModuleRun` carries contracts and hit/skip/error reasons.
- `CompileForDiagnostics`, warm dump, and diag dump expose contracts/reasons.
- `OptimizationRemarks` gives passes a structured sink for hit/miss reasons.

This substantially improves the "why did this optimization happen" workflow.

### Compilation Dependencies

The dependency framework has become a real safety mechanism:

- `CompilationDependencyRegistry` records global, shape, field, metatable, and
  call-ABI assumptions.
- Cached Tier2 code validates dependencies before reuse.
- Selected production passes now record dependencies:
  `GlobalConstSpecialization`, `CallABI`, `FieldSvalsLower`,
  `ShapeFieldTypeGuard`.

This is important because runtime specialization only scales if invalidation is
explicit and observable.

## Main Architectural Risks

### 1. IR Operation Definition Is Not Extensible Enough

Adding an IR op currently requires coordinated edits across:

- `ir_ops.go` enum and `opNames`
- graph builder or lowering pass
- validator contracts
- printer/diagnostics
- emitter dispatch
- register allocation behavior
- interpreter/oracle support, when applicable
- tests and benchmarks

This is acceptable for a small compiler, but MethodJIT already has many
domain-specific ops: matrix, string, table arrays, field caches, record-array
kernels, complex escape kernels, and more. The lack of per-op metadata makes
new op work easy to get wrong.

Recommended next step:

- Introduce an `OpSpec` table that declares at least:
  name, result type policy, side-effect kind, memory effect, terminator shape,
  deopt behavior, arg count policy, regalloc class hints, and emitter family.
- Keep the `Op` enum for speed, but drive validation, printing, and basic
  diagnostics from `OpSpec`.
- Add a test that every `Op` has an `OpSpec`, name, validator policy, and
  codegen/interpreter handling classification.

Expected impact:

- New language features and optimizations have a checklist enforced by tests.
- Fewer silent omissions in printer, validator, diagnostics, or codegen.

### 2. AnalysisResult Is a Shared Mutable Data Dump

`AnalysisResult` contains integer ranges, table facts, call ABI facts, fixed
shape facts, string facts, speculation data, global facts, and codegen hints.
It is convenient, but every pass can theoretically mutate any field.

The recently added `AnalysisFact` contract describes module-level dataflow, but
it does not yet enforce payload ownership inside `AnalysisResult`.

Recommended next step:

- Split analysis payloads into typed stores by domain:
  `NumericFacts`, `TableFacts`, `CallFacts`, `StringFacts`, `SpecFacts`.
- Make pass helpers expose narrow methods such as `SetCallABI`, `GetIntRange`,
  `MarkFieldTypeElided` instead of direct map mutation everywhere.
- Add lightweight fact diffing to `Tier2ModuleRun`: declared facts plus actual
  changed fact domains.

Expected impact:

- Future passes have clear ownership boundaries.
- Diagnostics can report "declared Updates=IntRanges, actual changed IntRanges
  and Int48Safe" and catch incorrect contracts.

### 3. Codegen Dispatch Is Too Centralized

`emit_dispatch.go` is the mandatory edit point for every new lowered op. It also
owns cache invalidation decisions around field caches, bounded-key tracking, and
shape verification. This couples operation semantics with backend housekeeping.

Recommended next step:

- Introduce emitter families:
  numeric, table, field, call, string, matrix, control, coroutine/channel.
- Keep one top-level dispatch, but delegate to family dispatchers by op range or
  `OpSpec.EmitFamily`.
- Move cache invalidation policy into per-op metadata where possible:
  `PreservesFieldSvalsCache`, `ClearsTableArrayBounds`, `InvalidatesShape`.

Expected impact:

- Adding a table optimization no longer risks editing call/string/numeric
  dispatch logic.
- Backend housekeeping becomes reviewable as data, not scattered switch logic.

### 4. Runtime Specialization Still Mixes Generic and Benchmark-Like Shapes

The codebase has moved away from static benchmark-specific kernels, but several
runtime-generated or guarded kernels remain very domain-shaped. That can be
valid, but the architecture needs a uniform specialization story.

Recommended next step:

- Define a `RuntimeSpecialization` abstraction:
  recognizer, guard set, dependency set, generated IR/native lowering,
  fallback semantics, diagnostics, and cache invalidation.
- Move record-array loop kernels, recursive table kernels, raw-int nested
  recurrences, and table-array kernels under that common interface.
- Add a diagnostic report grouping specializations by recognizer and guard
  reason.

Expected impact:

- A new hot loop shape is added by implementing the specialization interface,
  not by hand-threading custom fields through `Function`, `AnalysisResult`,
  VM proto caches, and emitter code.

### 5. Language Feature Additions Do Not Have a Single Contract

Adding a language feature currently crosses lexer, parser, AST, bytecode,
runtime, VM, MethodJIT graph builder, semantic gates, Tier1, Tier2, diagnostics,
and official-case tests. The tests exist, but the development contract is not
encoded.

Recommended next step:

- Add `docs/language-feature-checklist.md` covering:
  syntax, AST, bytecode, interpreter semantics, runtime library hooks,
  MethodJIT graph builder support, Tier1 support, Tier2 promotion policy,
  deopt/exit behavior, official tests, and hot benchmark coverage.
- Add a small machine-readable feature matrix, for example
  `tests/feature_matrix.json`, that records per-feature support in interpreter,
  Tier1, and Tier2.
- Make semantic gates point at matrix entries or explicit policy reasons.

Expected impact:

- New language features can land interpreter-first without accidentally entering
  unsupported JIT paths.
- Missing Tier2 support is visible as a policy decision, not tribal knowledge.

### 6. Diagnostics Are Good, But Not Yet Complete Enough for Iteration Speed

Current diagnostics answer many questions, but still miss two high-value
debugging views:

- Actual fact diff per module.
- Per-op support matrix for graph builder, regalloc, interpreter oracle, and
  emitter.

Recommended next step:

- Add `Tier2ModuleFactDiff` with lightweight before/after counters or hashes
  for fact domains.
- Add an `op audit` command/test that reports every op and whether it has:
  validator contract, printer name, regalloc handling, emitter handling,
  interpreter/oracle handling, deopt metadata.
- Include both in warm dump/diag artifacts.

Expected impact:

- When a pass starts returning wrong IR, diagnosis points to the first module
  that changed the relevant fact domain.
- When adding an op, omissions are caught before performance debugging.

## Priority Plan

### P0: Make IR Op Metadata Real

Create `op_spec.go` and use it for:

- op names
- terminator classification
- side-effect classification
- simple arg-count validation
- emitter family classification

Keep this small first. Do not try to encode all semantics in one pass.

Acceptance:

- every `Op` has an `OpSpec`
- existing `opNames` is generated from or validated against `OpSpec`
- validator uses `OpSpec` for terminator and arg-count basics
- one test fails when an op is added without metadata

### P1: Split AnalysisResult by Domain

Add domain structs and migrate pass-by-pass:

- `NumericFacts`
- `TableFacts`
- `CallFacts`
- `StringFacts`
- `SpeculationFacts`

Acceptance:

- old `AnalysisResult` remains as facade initially
- new passes use domain accessors
- `AnalysisFact` metadata points to a domain owner

### P2: Add Actual Fact Diff Diagnostics

Use domain-level counters/hashes rather than deep-copying every map.

Acceptance:

- `Tier2ModuleRun` includes declared contract and actual changed domains
- diagnostic output stays compact
- tests cover a module that declares one update and actually changes it

### P3: Modularize Codegen Dispatch

Introduce emitter families and per-op cache invalidation metadata.

Acceptance:

- top-level `emitInstr` shrinks substantially
- each family has focused tests
- cache invalidation policies are covered by op metadata tests

### P4: Define Runtime Specialization Interface

Bring current runtime-generated loop/kernel paths under one lifecycle.

Acceptance:

- recognizer/guards/dependencies/fallback are explicit
- diagnostic output explains why a specialization was selected or skipped
- dependency invalidation is tested for at least one shape and one global case

### P5: Language Feature Checklist and Matrix

Make language feature work predictable.

Acceptance:

- checklist exists
- feature matrix exists
- semantic gates reference explicit unsupported-feature reasons

## Suggested Review Gates

For every future JIT optimization:

1. Module registered through `ModuleRegistry`.
2. Declares `Requires/Provides/Updates`.
3. Emits at least one `OptimizationRemark` on hit or important miss.
4. Records `CompilationDependency` for every runtime assumption.
5. Has an op metadata entry if it introduces an op.
6. Has IR validator coverage for safety-critical shape.
7. Has one focused unit test and one warm benchmark or official-hot case.

For every future language feature:

1. Interpreter semantics pass official/translated cases.
2. Bytecode and VM behavior are covered.
3. MethodJIT graph builder either supports it or marks it unpromotable.
4. Tier1/Tier2 callable policy is explicit.
5. Diagnostics explain why JIT did or did not compile it.
6. A hot case exists if the feature is performance-sensitive.

