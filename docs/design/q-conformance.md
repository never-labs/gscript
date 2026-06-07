# q/kdb+ Conformance Matrix

This repository tracks q/kdb+-style feature coverage with a machine-readable matrix, not with handwritten manifest entries.

## Source of truth

`examples/data/q-cases.json` owns the `feature_matrix` array. Each row has:

- `id`: stable feature id used by q examples and benchmarks.
- `status`: one of `supported`, `rejected`, or `planned`.
- `coverage.tests`: `tests/language/q_*.leia` files that assert the behavior or rejection.
- `coverage.examples`: q example programs that exercise the feature.
- `coverage.benchmarks`: q data benchmarks that exercise the feature.

`benchmarks/data/q-cases.json` stays as the benchmark sidecar. Its case `features` must reference feature ids declared in `examples/data/q-cases.json`.

## Status meanings

- `supported`: implemented behavior with at least one q language conformance test.
- `rejected`: intentionally rejected q construct or error boundary with at least one q language test.
- `planned`: known gap with no executable coverage requirement yet.

The matrix is intentionally a coverage ledger. It does not imply full kdb+ compatibility for a feature, only that the listed fixtures cover the repository's current behavior.

## Validation

`tests/manifest.py check tests benchmarks` validates:

- every `tests/language/q_*.leia` case is present in `tests/manifest.json` as language conformance;
- every q data benchmark is present in `benchmarks/manifest.json` cases and workloads;
- q example and benchmark sidecars list all discovered q cases;
- every sidecar case feature is declared in the matrix;
- every matrix path points at an existing q test, example, or benchmark;
- `supported` and `rejected` rows have at least one q language test.

The Python unit coverage for these rules lives in `tests/manifest_test.py`.

## Runtime boundary

This matrix only connects existing tests, examples, and benchmarks. Updating it must not add Go q runtime behavior. Runtime implementation work belongs in a separate change with focused parser/evaluator tests.
