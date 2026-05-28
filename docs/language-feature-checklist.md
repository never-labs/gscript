# Language Feature Checklist

This checklist is the P5 language feature coverage matrix. It is intentionally
small and auditable: the source of truth is `tests/feature_matrix.json`, while
this document explains the fields and summarizes the initial state.

The matrix does not change JIT behavior. It records where each feature is
covered today across parsing, bytecode generation, interpreter execution,
MethodJIT Tier1, MethodJIT Tier2, semantic gates, translated official cases,
and hot performance cases.

## Matrix Fields

| Field | Meaning |
|---|---|
| `parser` | Front-end syntax and AST coverage. |
| `bytecode` | Compiler/opcode coverage for the feature. |
| `interpreter` | Baseline VM/runtime semantic coverage. |
| `tier1` | MethodJIT Tier1 coverage, or an explicit `semantic_only`/`not_applicable` status. |
| `tier2` | MethodJIT Tier2 optimization/codegen coverage, or an explicit exclusion. |
| `semantic_gate` | CI or official-case gate that should block semantic regressions. |
| `official_case` | Translated official Lua or comparable semantic cases. |
| `perf_hot_case` | Hot-loop benchmark coverage, when the feature matters for steady-state performance. |
| `spec_sections` | Level-2 sections in `docs/language-spec.md` that define this feature's language contract. |

Allowed status values are:

| Status | Meaning |
|---|---|
| `covered` | Existing coverage is direct enough for this matrix row. |
| `partial` | Coverage exists, but important edges or tier-specific behavior remain to be filled. |
| `missing` | No meaningful coverage recorded yet. |
| `not_applicable` | The field does not apply to this feature. |
| `semantic_only` | Keep correctness coverage, but do not expect a JIT/perf hot case unless a real workload appears. |

## Initial Feature Rows

| Feature | Parser | Bytecode | Interpreter | Tier1 | Tier2 | Semantic gate | Official case | Perf hot case |
|---|---|---|---|---|---|---|---|---|
| Literals and constants | covered | covered | covered | semantic_only | semantic_only | covered | covered | semantic_only |
| Numeric arithmetic and coercion | covered | covered | covered | covered | covered | covered | covered | covered |
| Comparison, boolean, and control flow | covered | covered | covered | covered | covered | covered | covered | covered |
| Loops and numeric for | covered | covered | covered | covered | covered | covered | covered | covered |
| Generic for, pairs, ipairs, and next | covered | covered | covered | partial | partial | covered | covered | covered |
| Functions, calls, returns, and tail calls | covered | covered | covered | covered | covered | covered | covered | covered |
| Varargs and multi-return adjustment | covered | covered | covered | partial | partial | covered | covered | covered |
| Closures and upvalues | covered | covered | covered | partial | partial | covered | covered | covered |
| Tables, arrays, fields, and constructors | covered | covered | covered | covered | covered | covered | covered | covered |
| Metatables and metamethods | not_applicable | covered | covered | partial | partial | covered | covered | covered |
| Strings, patterns, formatting, and concat | covered | covered | covered | partial | partial | covered | covered | covered |
| Errors, pcall, xpcall, and defer | covered | covered | covered | partial | partial | covered | covered | covered |
| Coroutines and resume/yield | not_applicable | covered | covered | partial | partial | covered | covered | covered |
| Bitwise operators and bit32 | covered | covered | covered | partial | partial | covered | covered | covered |
| Table library, sort, pack, unpack, and move | not_applicable | not_applicable | covered | semantic_only | semantic_only | covered | covered | covered |
| UTF-8 library | not_applicable | not_applicable | covered | semantic_only | semantic_only | covered | covered | covered |
| Host stdlibs | not_applicable | not_applicable | covered | semantic_only | semantic_only | covered | covered | partial |
| Dense arrays and matrix helpers | not_applicable | partial | covered | covered | covered | covered | covered | covered |
| Class-style and method syntax examples | covered | partial | covered | partial | partial | partial | not_applicable | covered |

## Maintenance

Update `tests/feature_matrix.json` first. The lightweight Go test
`TestFeatureMatrixSchema` checks that every feature row has all required fields,
uses an allowed status, keeps references as relative paths, points file
references at existing repository files, and maps `spec_sections` to existing
level-2 headings in `docs/language-spec.md`. It also checks the reverse
direction: every level-2 language-spec section must be referenced by at least
one feature row unless it is an explicitly ignored process/planning section.

Release metadata is checked by `TestReleaseMatrix*` in
`tests/release_matrix_test.go`. That gate requires every stable language-spec
section to have a `semantic_gate` or `official_case` reference, every paired
official translated case to be classified in
`tests/official_lua_cases/MANIFEST.md` or recorded in
`tests/official_lua_cases/KNOWN_FAILURES.md`, every known-gap ledger to be
named from `docs/test-matrix.md`, and every `docs/stdlib-contract.md` module to
have an official-case or capability-ledger coverage entry.

When adding a new language feature, add one row even if several columns are
`partial` or `missing`. The point of this matrix is to make gaps explicit
before they turn into assumptions about semantic or tier coverage. If no
language-spec section can be cited yet, update the specification first; parser
behavior alone is not a user-facing contract.
