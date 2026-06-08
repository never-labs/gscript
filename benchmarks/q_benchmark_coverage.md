# q Benchmark Coverage Audit

This audit maps Leia q language coverage from semantic tests to benchmark
coverage. It is intentionally benchmark-focused: semantic tests prove behavior,
while benchmark cases must prove hot-path cost, allocation pressure, cache value,
typed-kernel hit rate, and fallback pressure.

Sources reviewed:

- `internal/stdlib/lib/q/eval_test.go`
- `internal/stdlib/lib/q/parser_test.go`
- `internal/stdlib/bind/q_test.go`
- `benchmarks/q_eval_vector_bench_test.go`
- `benchmarks/q_columnar_suite.sh`
- `benchmarks/q_perf_report.py`

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

The suite needs broad q language coverage, not only qSQL. q's performance story
depends on vector/list primitives, adverbs, dictionaries, symbols, temporal
values, keyed tables, joins, mutations, and table metadata operations.

## Existing Semantic Surface

The q semantic tests already cover a much wider surface than the historical
benchmark set. These categories should drive benchmark case growth.

| Category | Evidence in tests | Benchmark status |
|---|---|---|
| Numeric atoms/vectors | numeric literals, typed numeric suffixes, casts, dyadic ops, promotion, null propagation, percent divide | Covered by q.eval coverage tags; still needs more per-type performance depth |
| Boolean masks and `where` | bool vectors, compare masks, scalar comparisons, comma where as and, logical `&`/`|` | Covered by q.eval coverage tags and 0/1/50/99/100% value-gather-reduce selectivity cases; compound qSQL predicate depth remains open |
| Adverbs | each, each-prior, each-left/right, over, scan, verb adverbs, function projections | Covered by q.eval coverage tags; more complex list-of-lists cases should be added |
| Reductions and scans | `sum`, `sums`, `+/`, `+\`, `min`, `max`, `count`, `avg`, `var`, `dev`, `med`, `wavg` | Covered by q.eval coverage tags except deeper `wavg`/null/type matrix |
| Set/list verbs | `distinct`, `group`, `where`, `reverse`, `prev`, `next`, `deltas`, `fills`, `enlist`, `raze`, `cut`, `drop`, sort indexes | Covered at category level; needs exhaustive verb-by-type matrix |
| Dicts | symbol dictionaries, nested dictionaries, lookup, `keys`, `value`, amend/upsert | Covered at category level; keyed/table nested hot paths need more rows |
| Table literals | `flip`, native table literal, qSQL-style table literal, keyed table literals | Covered at category level; large table materialization cases need more rows |
| Keyed tables | `xkey`, lookup, keyed amend/upsert, key/value/cols/meta APIs | Covered at category level; lookup/amend/upsert hot paths need direct Go baselines |
| Symbols and enums | symbol vectors, enum metadata, grouped/unique/sorted attributes | Covered at category level; grouped/unique/sorted attribute matrices need more rows |
| Temporal values | date, time, timestamp, timespan, temporal typed nulls, `.z.*` values | Covered at category level; all temporal kind combinations need expansion |
| Table verbs | `xcols`, `xasc`, `xdesc`, `xgroup`, `ungroup`, `meta`, `cols`, `key` | Covered at category level; table-size scaling cases remain open |
| qSQL select/exec | projection, computed projection, filter, order, limit/take, distinct, dict exec | Partially covered by qSQL Go benchmarks and columnar scripts |
| qSQL grouped analytics | `by`, computed keys, `xbar`, aggregate aliases, extended aggregates | Partially covered; needs more aggregate/key/type combinations |
| qSQL joins | inner, left, asof variants, union, plus, window joins, chained joins, aliased keys | Asof/join subsets covered; window/union/plus/chained joins need benchmark rows |
| qSQL mutation | update, delete, insert, upsert, grouped mutation, keyed mutation | Semantics covered; benchmark coverage is limited |
| Cache/fallback/runtime stats | plan cache, query kernel cache, schema-stable keys, explain/fallback stats | qSQL benchmark metrics exist; q.eval typed kernel/fallback stats need benchmark-readable rows |
| IPC/system/session | loopback IPC, safe system commands, session state | Not core to in-memory analytics performance; benchmark only if it becomes a product target |

## Current Benchmark Case Dimensions

The ordinary `q.eval` vector/list suite should be treated as the non-qSQL
performance front door. It now needs to scale from a handful of checks to a
matrix of q-language operations:

| Dimension | Case shapes to keep or add |
|---|---|
| Numeric vector arithmetic | affine sums, mixed arithmetic, square/product expressions, int and float variants |
| Typed numeric values | short/int/long/real/float suffixes, casts, typed nulls, promotion boundaries |
| Masks and `where` | 0/1/50/99/100% selectivity, value gather after `where`, reduce after projection, compound logical predicates |
| Slicing and reordering | `take`, `drop`, `reverse`, positive/negative `rotate`, first/last checksums |
| Reductions/scans | `sum`, `sums`, `+/`, `+\`, `min`, `max`, `avg`, `count`, null-aware variants |
| Adverbs | each, each-prior, each-left/right, over, scan, projected functions |
| List/table-like transforms | `distinct`, `group`, `raze`, `enlist`, `cut`, `fills`, `prev`, `next`, `deltas` |
| Dictionaries | symbol-key dict lookup, dictionary each, `keys`, `value`, nested dict traversal |
| Symbols | symbol vector compare, distinct/group/sort, attribute-marked vectors |
| Temporal lists | date/time/timestamp/timespan compare, xbar bucket, typed null propagation |

The qSQL benchmark suite should become a matrix rather than a small set of
representative queries:

| Dimension | Case shapes to keep or add |
|---|---|
| Select/filter/project | scalar threshold, bound scalar, computed projection, projection width, order/limit/take |
| Predicate shape | numeric compare, symbol compare, temporal compare, compound and comma-where predicates |
| Grouped aggregate | one key, multi-key, symbol key, temporal/xbar key, `sum/count/avg/min/max/var/dev/med/wavg` |
| Join shape | inner, left, asof, asof variants, window, union, plus, chained joins, aliased keys |
| Mutation | update/delete/insert/upsert, keyed mutation, grouped update, empty-match boundaries |
| Cache behavior | cold parse/lower, warm plan cache, schema-stable cache, literal/bound-scalar split |
| Runtime stats | typed kernel hit/fallback rows for each qSQL shape |

## Immediate Expansion Targets

To make performance results convincing, grow benchmark cases by at least 10x
from the original five ordinary q cases and four columnar script cases:

| Target | Minimum breadth |
|---|---|
| `q.eval` ordinary compute | 88 cases with a required coverage-tag gate across vector arithmetic, typed/null/cast/promotion, where/selectivity, slice/reorder, adverb, dict, symbol, temporal, table-verb, IPC, and safe-system shapes |
| qSQL Go benchmarks | 25+ rows across select, group, join, mutation, cache, and direct-runtime baselines |
| q columnar script suite | 40+ script-level cases drawn from market data, rollup, join, keyed state, and vector/adverb projects |
| Metrics per case | `ns/op`, `B/op`, `allocs/op`, warm/cold ratio, Go ratio where practical, typed-kernel/fallback stats where available |

Case growth should avoid benchmark-only shortcuts. Each case should correspond
to a real q operation shape that exists in semantic tests or in the data
analytics examples under `benchmarks/data`.

## Known Gaps

| Gap | Why it matters |
|---|---|
| q.eval typed kernel hit/fallback metrics are not yet benchmark-readable per case | We can time q.eval operations, but cannot fully explain whether runtime kernels or interpreter fallback dominated |
| q.eval coverage is category-complete but not yet exhaustive by operation x type x null x shape matrix | The new tag gate prevents missing whole expression families; the next round must enumerate complex combinations and per-type variants |
| qSQL cold/warm coverage is select-heavy | Group, join, mutation, and temporal shapes also need cold/warm cache rows |
| Join benchmarks need more variants | Asof is important, but inner/left/window/union/plus/chained joins exercise different runtime costs |
| Mutation benchmarks are thin | Update/delete/upsert are core table analytics operations and interact with keyed frames and schema-stable cache |
| Script-level q suite is too narrow | Current-vs-HEAD evidence should include more ordinary q and market-data project shapes, not only four columnar scripts |

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
- allocation elimination through reusable buffers and immutable column views
- schema-stable caches with explainable fallback and hit-rate statistics

If an optimization cannot be explained as one of these reusable mechanisms, it
does not belong in the runtime/JIT path even if it improves one benchmark row.
