# Implementation Requirements

This chapter defines implementation constraints for stable language behavior.
It is normative for interpreter, VM, JIT, standard-library, documentation, and
release-gate work.

## Interpreter Baseline

The interpreter is the semantic baseline for all stable language features. A
stable feature is not implemented until the interpreter behavior is defined,
tested, and listed in `tests/feature_matrix.json` with the relevant coverage
references.

Bytecode VM and JIT execution must preserve interpreter-visible behavior. The
following observable results must be equivalent across execution modes unless a
feature entry explicitly records an unsupported mode:

- returned values and side effects;
- control-flow behavior, including errors, protected-call results, defers,
  coroutine state, channel operations, and synchronization behavior;
- source locations and diagnostics that are specified by the language or CLI
  contract;
- standard-library module identity and visible module state.

Implementation details are not observable language behavior and must not be
used by user code as compatibility guarantees:

- table layout;
- dense numeric representation;
- raw integer JIT representation;
- inline caches;
- native call ABI;
- garbage collection timing;
- scheduling details except for specified channel and synchronization behavior.

## VM And JIT Equivalence

VM and JIT support for a stable feature must be justified by equivalence tests,
not by benchmark success alone. A change that alters parsing, bytecode
generation, interpreter execution, VM execution, or JIT execution for a stable
feature must update the corresponding `tests/feature_matrix.json` row and must
include conformance, semantic-gate, or mode-equivalence coverage appropriate to
the changed behavior.

Runnable examples in the spec are part of this contract. Stable spec examples
must use `leia run all` for accepted programs or `leia fail all` for rejected
programs, and are validated by `tests/spec_examples_test.go` across the
interpreter, VM, and default execution modes. Any normative example added to
`docs/spec/*.md` that is intended to demonstrate accepted or rejected behavior
must use one of those all-mode runnable fence tags unless the example is
explicitly non-executable syntax notation.

## Runtime Specialization

Runtime specialization is permitted only behind guards that check the facts
required by the specialized path. A specialized path must either:

- prove that the specialized facts still hold before executing behavior that
  depends on them; or
- deoptimize, side-exit, or recover to an unspecialized path before user-visible
  behavior can diverge.

Guard failure, side exit, deoptimization, fallback, or recovery must preserve
the same visible result as interpreter execution for the same program state.
Recovery must not replay visible side effects, skip required side effects, alter
error values, or change coroutine/channel/synchronization state.

Specialization must be derived from runtime-observed program facts, static
language facts, or explicit implementation configuration. It must not depend on
benchmark names, source file names, directory names, fixed benchmark input
sizes, or other source-name based selectors. Benchmark fixtures may exercise
specialized behavior, but they must not be the condition that enables that
behavior.

## Release Gates

Stable syntax and runtime behavior must be represented in
`tests/feature_matrix.json`. The matrix is a release gate: a stable feature row
must carry `parser`, `bytecode`, `interpreter`, `tier1`, `tier2`,
`semantic_gate`, `conformance_case`, and `perf_hot_case` status according to
the matrix schema. The schema and referenced files are checked by
`go test ./tests -run 'TestFeatureMatrix|TestReleaseMatrix' -count=1`.

Changes to stable syntax require:

- grammar and spec updates in `docs/spec/grammar.ebnf` and the relevant
  `docs/spec/*.md` chapter;
- parser or lexer tests covering the syntax;
- conformance or semantic tests proving accepted and rejected behavior;
- a matching `tests/feature_matrix.json` update.

Changes to stable runtime behavior require:

- interpreter coverage for the baseline behavior;
- VM coverage when bytecode execution is affected;
- JIT semantic-gate or equivalence coverage when tier1 or tier2 execution is
  affected;
- release-matrix coverage for any changed stable spec section.

Documentation release checks are enforced by `scripts/docs_check.sh`. That gate
checks Markdown links, documented release-script references, release-readiness
snippets, generated reference documentation, and generated spec HTML freshness.

## Standard Library And Catalog Synchronization

The public standard-library surface must stay synchronized across implementation
registration, catalog metadata, generated reference documentation, and tests.
Adding, removing, renaming, or changing the public behavior of a standard
library module requires updating `internal/stdlib/catalog/catalog.go`,
`docs/reference/stdlib/index.md`, and the corresponding stdlib tests. Generated
stdlib documentation must remain reproducible by the docs generation command
checked by `scripts/docs_check.sh`.

Catalog metadata is part of the release surface. Module names, layers,
capabilities, and safe-default classification must match the implementation and
the documented embedding/security behavior. Host-capability modules must not be
documented or cataloged as safe defaults unless their implementation is safe
without host authority.

## Performance-Sensitive Changes

A performance-sensitive change is any implementation change that affects hot
loops, numeric operations, table access, calls, strings, concurrency, data
oriented operations, standard-library dispatch, tiering, guards, deopt, or JIT
code generation. Such a change requires correctness evidence and performance
evidence. Benchmark output is not a substitute for semantic coverage.

Performance-sensitive changes must use the repository performance gate relevant
to the touched path. The repository provides `scripts/performance_gate.sh`,
which wraps `benchmarks/timing_compare.py` and `benchmarks/strict_guard.py`.
Release documentation also references `bash scripts/performance_gate.sh
--feature-smoke` for feature-smoke performance evidence.

## Architecture And Package Boundaries

Implementation changes must preserve package ownership boundaries. Runtime,
VM, JIT, method-JIT, stdlib catalog, stdlib binding, stdlib installation, and
stdlib module packages must not gain imports that collapse their separation of
responsibilities. Internal package boundaries are checked by
`tests/architecture/package_boundary_test.go`.

The standard-library catalog must remain metadata-only and must not depend on
runtime values, module constructors, VM, JIT, method-JIT, stdlib binding, or
stdlib installation packages. Standard-library module implementations must not
import VM, JIT, method-JIT, binding, installation, or runtime internals unless a
specific architecture test and spec update authorize the dependency.
