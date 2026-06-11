# q Benchmark Coverage Audit

This audit maps Leia q language coverage from semantic tests to benchmark
coverage. It is intentionally benchmark-focused: semantic tests prove behavior,
while benchmark cases must prove hot-path cost, allocation pressure, cache value,
typed-kernel hit rate, and fallback pressure.

Sources reviewed:

- `internal/stdlib/lib/q/eval_test.go`
- `internal/stdlib/lib/q/parser_test.go`
- `internal/stdlib/lib/q/supported_forms.go`
- `internal/stdlib/bind/q_test.go`
- `benchmarks/q_eval_vector_bench_test.go` (plus the
  `q_eval_vector_cases_{typematrix,verbforms,combo}_test.go` case files)
- `benchmarks/q_eval_verb_coverage_test.go`
- `benchmarks/q_eval_jit_script_bench_test.go`
- `benchmarks/q_columnar_suite.sh`
- `benchmarks/q_perf_report.py`
- `benchmarks/data/qeval_go_ratio_baseline.json`

## Coverage Goals

The q benchmark suite should measure at least these ratios for every major
shape where a hand-written Go baseline is practical:

| Signal | Required benchmark evidence |
|---|---|
| Current Leia vs old Leia | current worktree vs clean `HEAD` for script-level suites |
| Current Leia vs Go | `q.eval`, qSQL, and direct data runtime rows against equivalent Go loops |
| Warm vs cold | schema/result cache warm rows against cold parse/lower/bind rows |
| Typed kernel hit rate | per-shape typed kernel hit and fallback counters |
| Allocation pressure | `B/op` and `allocs/op` for every Go benchmark row |

The report-level `--check` gate also requires benchmark evidence for the three
backend layers that matter to the new architecture: ordinary q typed primitive
rows, MethodJIT direct/exit route rows, and MethodJIT array bridge rows. A run
that omits one of those layers should fail the contract gate unless it
explicitly lowers the corresponding minimum benchmark count for a partial
local check.

The gate also requires at least one lower-level backend route metric from either
the VM runtime primitive registry (`runtime_primitive_hits/op` and
`runtime_primitive_errors/op`) or MethodJIT Frame/Vector runtime routes
(`methodjit_frame_runtime_success/op`, `methodjit_vector_runtime_success/op`,
and their error counters). This is separate from typed-kernel hit rate: typed
kernel rows prove q-level shape dispatch, while backend-route rows prove the
runtime registry or Frame/Vector MethodJIT counters are still wired into the
measurement surface.

The suite needs broad q language coverage, not only qSQL. q's performance story
depends on vector/list primitives, adverbs, dictionaries, symbols, temporal
values, keyed tables, joins, mutations, and table metadata operations.

## Current Coverage Status

The ordinary `q.eval` suite now stands at **483 cases** (`qEvalVectorCases`),
each with a hand-written Go baseline, up from roughly 404 after the previous
expansion round and from the original five cases. The structural floor in
`TestQEvalVectorBenchmarkExpressions` is pinned at **480**, so the case count
can only ratchet upward.

### Baseline-honesty gates

Every Go baseline must do real, row-scaled work:

- `TestQEvalVectorAllGoBaselinesDoRealWork` executes each Go baseline at two
  row counts and fails if the result is row-invariant, unless the case appears
  in the row-invariant allowlist with a written justification.
- The allowlist is shrink-only: `qEvalRowInvariantBaselineAllowlistMax = 89`
  pins its size, every entry needs a justification string, and stale entries
  (no matching case) fail the test. The 89 allowlisted cases are constant-work
  shapes on both sides (literal vectors, fixed-width probes), not broken
  baselines.
- In the Go-ratio ratchet baseline, 24 cases are marked untrusted
  (`max_untrusted_go_baselines = 24`, shrink-only) and excluded from ratio
  gating; the remaining **459 trusted cases** drive the ratchet.

### Source-derived verb coverage gate

`benchmarks/q_eval_verb_coverage_test.go` extracts every dispatch verb from
`internal/stdlib/lib/q/eval.go` via `go/ast` (the `lookupUnaryVerb` /
`lookupDyadicVerb` / `lookupDyadicVerbFunc` tables), so the gate tracks the
evaluator's real surface instead of a hand-maintained list. Non-verb special
forms come from `SupportedEvalForms()` in
`internal/stdlib/lib/q/supported_forms.go`: a curated list of **17 forms**
(control flow `if`/`do`/`while`/`cond`, the six adverbs, apply-at/apply-dot,
dict/table literals, `fby`, projection, composition), each pinned executable by
`TestSupportedEvalFormsAllEvaluate`.

All three backlogs are now empty and frozen shrink-only at zero:

| Backlog | Entries | Max |
|---|---:|---:|
| `qEvalVerbCoverageBacklog` | 0 | 0 |
| `qEvalFormBacklog` | 0 | 0 |
| `qEvalComboBacklog` | 0 | 0 |

Any new verb or form added to the evaluator without a benchmark case fails the
gate immediately; backlogging it again is impossible without raising a frozen
constant.

Combination coverage is gated too: **7 required combo shapes**
(`qEvalRequiredComboShapes`: depth-3 producer-transform-reducer,
mixed-type arith, null-mixed pipeline, nested adverb, where compound
predicate, dict-table chain, adverb-over-apply-index) each need at least
**3 cases** (`qEvalComboMinCases`). The type system is covered by a
**44-case type x null matrix** (`q_eval_vector_cases_typematrix_test.go`):
short/int/long/real/float dense and null carriers across arithmetic, compare,
cast, promotion, and where-gather shapes. Control flow (`if`/`do`/`while`/
`$[c;t;f]`) is covered by row-scaled `VerbForm*` cases
(`q_eval_vector_cases_verbforms_test.go`).

### Two-layer Go-ratio measurement

Leia-vs-Go is measured at two layers, both pinned by routing tests:

1. **Session-warm layer** — `BenchmarkQSessionEvalVectorWarmExecution` runs
   each case through a warm `EvalSession`, which caches parse/plan artifacts
   but has no result cache, so typed q runtime kernels re-execute every
   iteration. Rows report typed-kernel attempt/hit/fallback/error counters
   per op where runtime instrumentation is available, so a case that silently
   stops doing kernel work is visible.
2. **JIT-script layer** — `BenchmarkQEvalJITScriptWarm`
   (`q_eval_jit_script_bench_test.go`) enumerates the full `qEvalVectorCases`
   table minus shrink-only harness exclusions, so the same ordinary q breadth
   that has session and Go rows also has a JIT-script measurement. It runs the
   hot loop inside a Leia script through the public embedding API so the loop
   tiers up to Tier 2 native code. The loop body uses the
   `q.session().eval(<const source>)` route, which methodjit lowers to the
   `OpQEvalSessionEval` op-exit — one session eval per loop iteration, no
   result memoization. `TestQEvalJITScriptRouting` pins per-iteration
   execution: typed-kernel attempts scale exactly with iteration count, Tier 2
   accepts the loop, and the bare-`q.eval` route stays pinned as memoized so a
   future per-iteration-capable direct route is noticed.

### Why benchmarks use session eval (result-cache semantics)

Two memoization layers make bare `q.eval(<constant source>)` measure cache
hits instead of work:

- **Bind-level result cache**: `q.eval` through `internal/stdlib/bind/q.go`
  memoizes the result of any `EvalSourceCacheable` constant source; iterations
  after the first are ~500ns map hits.
- **EvalState constant-statement memo**: closed constant expressions — no
  assignments, no free names, every bare identifier a deterministic builtin
  verb (`qEvalConstantStatementSource` in `internal/stdlib/lib/q/eval.go`) —
  memoize their value per `EvalState` (`constValueCache`), even on the session
  route.

The row-scaled benchmark expressions all use assignments
(`x:til 8192;...`), so they are non-constant and re-execute fully under
session eval. Cases must keep that property; a benchmark expression rewritten
into a closed constant form silently degrades into a memo-lookup measurement.

### Go-ratio ratchet gate

`benchmarks/data/qeval_go_ratio_baseline.json` snapshots per-case
`warm_go_ratio` and `jit_go_ratio` (483 case entries; 459 trusted warm ratios,
458 trusted jit ratios). `q_perf_report.py --check` enforces:

- **No-regression ratchet**: each case may not regress beyond its baseline
  ratio x 1.15 (`RATIO_BASELINE_REGRESSION_TOLERANCE`).
- **Hard caps from `milestone_caps`** in the same file — currently
  `max_leia_go_ratio = 64` and `max_leia_jit_go_ratio = 8` (tightened from
  320/56 after wave 2), plus typed-hit/fallback/alloc envelope caps.
- Baselines are refreshed deliberately with `--update-ratio-baseline` after an
  optimization wave lands; the file records its capture date.

Current state (baseline captured 2026-06-11, post wave-2, origin/main `6e3d6cd3`):

| Layer | Geomean Leia/Go | Cases beating Go |
|---|---:|---:|
| Session-warm (trusted) | **0.99** | 253 / 459 |
| JIT script | **0.49** | 316 / 458 |

The report-level gate now also checks family breadth in the actual benchmark
output: ordinary list/adverb rows, `TypeMatrix*` rows, and `Combo*` rows must
have matching session, Go baseline, and JIT-script rows. This catches partial
perf runs that accidentally omit ordinary q breadth while still producing
healthy qSQL or single-shape ratios.

See `benchmarks/q_breadth_perf_audit.md` for the worst remaining rows and the
wave-3 priorities.

## Existing Semantic Surface

The q semantic tests cover a wide surface; benchmark coverage now tracks it
through the gates above.

| Category | Evidence in tests | Benchmark status |
|---|---|---|
| Numeric atoms/vectors | numeric literals, typed numeric suffixes, casts, dyadic ops, promotion, null propagation, percent divide | Covered: 44-case type x null matrix plus coverage tags and expression-combination cases |
| Boolean masks and `where` | bool vectors, compare masks, scalar comparisons, comma where as and, logical `&`/`|` | Covered: selectivity cases, compound-predicate combo shape, `VerbForm*LogicalMaskCount` rows |
| Adverbs | each, each-prior, each-left/right, over, scan, verb adverbs, function projections | Covered: all six adverb forms gated via `SupportedEvalForms()`, nested-adverb combo shape |
| Control flow | `if`, `do`, `while`, `$[c;t;f]` | Covered: row-scaled `VerbFormIfGuardedReduce`, `VerbFormDoAccumulateVectorSum`, `VerbFormWhileRowBoundSum`, `VerbFormCondBranchReduce` |
| Reductions and scans | `sum`, `sums`, `+/`, `+\`, `min`, `max`, `count`, `avg`, `var`, `dev`, `med`, `wavg` | Covered including running/moving aggregates |
| Set/list verbs | `distinct`, `group`, `where`, `reverse`, `prev`, `next`, `deltas`, `fills`, `enlist`, `raze`, `cut`, `drop`, sort indexes | Covered: verb gate derives the full list from eval.go dispatch tables; backlog empty |
| Dicts | symbol dictionaries, nested dictionaries, lookup, `keys`, `value`, amend/upsert | Covered at verb/form level plus dict-table-chain combo shape |
| Table literals | `flip`, native table literal, qSQL-style table literal, keyed table literals | Covered: table-literal form gated; large materialization cases remain thin |
| Keyed tables | `xkey`, lookup, keyed amend/upsert, key/value/cols/meta APIs | Covered at verb level; keyed amend/upsert hot paths still need more direct Go baselines |
| Symbols and enums | symbol vectors, enum metadata, grouped/unique/sorted attributes | Covered at verb level; attribute matrices need more rows |
| Temporal values | date, time, timestamp, timespan, temporal typed nulls, `.z.*` values | Covered at category level; full temporal kind matrix remains open |
| Table verbs | `xcols`, `xasc`, `xdesc`, `xgroup`, `ungroup`, `meta`, `cols`, `key` | Covered via verb gate and combo dict-table chains |
| qSQL select/exec | projection, computed projection, filter, order, limit/take, distinct, dict exec | Covered by qSQL Go benchmarks and columnar scripts |
| qSQL grouped analytics | `by`, computed keys, `xbar`, aggregate aliases, extended aggregates | Partially covered; needs more aggregate/key/type combinations |
| qSQL joins | inner, left, asof variants, union, plus, window joins, chained joins, aliased keys | Covered: warm-cache rows exist for inner/left/asof/window/union/plus/chained joins |
| qSQL mutation | update, delete, insert, upsert, grouped mutation, keyed mutation | Update/delete-where rows exist; insert/upsert/keyed mutation benchmark coverage is limited |
| Cache/fallback/runtime stats | plan cache, query kernel cache, schema-stable keys, explain/fallback stats | qSQL metric rows plus q.eval session typed-kernel counters per op |
| IPC/system/session | loopback IPC, safe system commands, session state | Not core to in-memory analytics performance; benchmark only if it becomes a product target |

## Current Benchmark Case Dimensions

The ordinary `q.eval` vector/list suite is the non-qSQL performance front
door. Its case matrix now spans:

| Dimension | Case shapes present |
|---|---|
| Numeric vector arithmetic | affine sums, mixed arithmetic, square/product expressions, int and float variants |
| Typed numeric values | short/int/long/real/float suffixes, casts, typed nulls, promotion boundaries (44-case matrix) |
| Masks and `where` | 0/1/50/99/100% selectivity, value gather after `where`, reduce after projection, 3-4 clause compound predicates, null-guard compounds |
| Slicing and reordering | `take`, `drop`, `reverse`, positive/negative `rotate`, first/last checksums |
| Reductions/scans | `sum`, `sums`, `+/`, `+\`, `min`, `max`, `avg`, `count`, null-aware variants |
| Adverbs | each, each-prior, each-left/right, over, scan, projected functions, nested adverbs |
| Control flow | `if`/`do`/`while`/`cond` bodies doing row-scaled vector work |
| List/table-like transforms | `distinct`, `group`, `raze`, `enlist`, `cut`, `fills`, `prev`, `next`, `deltas` |
| Dictionaries | symbol-key dict lookup, dictionary each, `keys`, `value`, nested dict traversal, dict-table chains |
| Symbols | symbol vector compare, distinct/group/sort, attribute-marked vectors |
| Temporal lists | date/time/timestamp/timespan compare, xbar bucket, typed null propagation |
| Word-alias verb forms | `divide`/`minus`/`equal`/`equals`/`less`/`greater`/`left`/`right` and symbol-operator equivalents |

The breadth layer (arithmetic/math envelopes, aggregation windows, list
transforms, apply/index, matrix/reshape, string/symbol) and the qSQL matrix
dimensions from the previous round remain in place; see the qSQL rows in
`benchmarks/q_performance_suite.sh` output for the join/mutation/cache
coverage that now exists.

## Known Gaps

| Gap | Why it matters |
|---|---|
| Typed-kernel counters cover instrumented families only | 8 warm cases still show fallback pressure (worst: `TypeMatrixLongNullNotEqualEqualNullCount`, 9% hit, 31 fallbacks/op); uninstrumented families need runtime-side shape labels |
| 206 of 459 trusted warm cases are still >= 1x Go, 68 >= 10x | The geomean is at parity but the tail is long; see the breadth audit for the wave-3 worklist |
| qSQL cold/warm coverage is select-heavy | Group, join, mutation, and temporal shapes also need cold/warm cache rows |
| Mutation benchmarks are thin | Insert/upsert/keyed mutation interact with keyed frames and schema-stable cache |
| Script-level q suite is narrow | Current-vs-HEAD evidence should include more ordinary q and market-data project shapes, not only the columnar scripts |
| Temporal kind matrix incomplete | All temporal kind x null combinations still need enumeration |

## Review Rule

When adding a new q semantic feature or optimizing q runtime/JIT, update this
matrix first, then add benchmark cases in the nearest existing suite. A feature
is performance-covered only when it has at least one executable benchmark row
with allocation metrics and a clear ratio target.

## Optimization Rule

Benchmark cases are evidence, not optimization targets. q performance work must
optimize reusable language/runtime shapes, never specific case names, literal
values, fixed row counts, or exact source strings. Valid optimization units are
general mechanisms such as:

- typed vector arithmetic and reduction kernels
- typed compare mask, `where`, gather/filter, and projection kernels
- table/frame primitives for group, join, sort, keyed lookup, amend, and upsert
- expression lowering that recognizes reusable q AST/data-expression shapes
- JIT calls into typed runtime kernels for stable Frame/Vector shapes
- runtime primitive registry and MethodJIT Frame/Vector route metrics that make
  backend execution observable
- allocation elimination through reusable buffers and immutable column views
- schema-stable caches with explainable fallback and hit-rate statistics

If an optimization cannot be explained as one of these reusable mechanisms, it
does not belong in the runtime/JIT path even if it improves one benchmark row.
