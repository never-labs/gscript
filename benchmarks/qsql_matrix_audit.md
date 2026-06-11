# qSQL Matrix First Audit

First measurement of the expanded qSQL benchmark matrix
(`internal/stdlib/bind/q_bench_matrix_test.go`). This is the qSQL analogue of
`q_breadth_perf_audit.md`: a worklist for the next qSQL optimization round,
not a regression gate.

- Command: `go test ./internal/stdlib/bind -run '^$' -bench 'BenchmarkQSQL' -benchmem -benchtime=100x`
- Captured: 2026-06-11, single 100x run on Apple Silicon (M-series, -16).
- 111 qSQL benchmark rows total; 87 are new matrix rows
  (29 cases x {BindMatrixWarm, BindMatrixCold, NativeGoMatrix}).
- Every case checksum-verified against its Go baseline
  (`TestQSQLMatrixBenchmarkCasesMatchGoBaseline`) and row-scaled
  (`TestQSQLMatrixGoBaselinesDoRealWork`, empty allowlist).
- Caveat: three agents were concurrently optimizing
  `internal/stdlib/lib/{q,data}` kernels while this snapshot was taken; treat
  absolute ratios as a single-run baseline and re-measure before/after any
  optimization lands.

## Headline

| Metric | Value |
|---|---:|
| Matrix cases | 29 |
| Geomean warm Leia/Go | **32.0x** |
| Cases at/below 5x | 6 |
| Cases above 100x | 9 |
| Worst case | InsertKeyedNewKey at **458x** |

The matrix exposes exactly the families the legacy 23-row suite did not
measure: keyed mutations, the asof/window join family, multi-key grouping, and
plain-row insert/upsert all run 20-460x behind hand-written Go, while the
typed-kernel select/filter and inner-join-topK paths are at or below Go parity.

## Warm-vs-Go ratios (worst first)

| Case | Warm us | Go us | Warm/Go | Cold/Warm | Warm B/op | Warm allocs/op |
|---|---:|---:|---:|---:|---:|---:|
| InsertKeyedNewKey | 2434 | 5.3 | 458.3 | 0.99 | 8,273,082 | 140,137 |
| UpsertKeyedExistingKey | 2377 | 6.4 | 371.1 | 0.99 | 7,355,304 | 140,112 |
| UpdateKeyedWhere | 2389 | 11.7 | 204.2 | 1.01 | 6,140,613 | 114,423 |
| JoinAsofFill | 2382 | 11.7 | 203.4 | 0.99 | 3,452,086 | 153,324 |
| JoinAsofFill0 | 2494 | 13.3 | 187.4 | 0.96 | 3,711,588 | 161,509 |
| DeleteKeyedWhere | 1656 | 9.9 | 166.7 | 0.97 | 3,652,181 | 57,641 |
| JoinAsofTemporal | 2291 | 15.0 | 152.9 | 0.98 | 3,449,492 | 153,324 |
| JoinAsof0PreserveRightTime | 2389 | 16.4 | 145.7 | 1.03 | 3,710,608 | 161,509 |
| GroupMultiKeySumCountAvg | 1241 | 10.4 | 118.8 | 1.04 | 1,006,940 | 42,077 |
| UpdateComputedWhere | 1104 | 16.9 | 65.5 | 0.87 | 2,832,244 | 66,980 |
| InsertPlainRow | 670 | 10.2 | 65.4 | 0.97 | 4,362,338 | 45,121 |
| UpdateGroupedBroadcast | 1484 | 22.7 | 65.3 | 0.95 | 3,264,884 | 85,274 |
| UpsertPlainAppend | 689 | 11.3 | 60.9 | 1.02 | 4,600,398 | 53,311 |
| UpdateEmptyMatchBoundary | 525 | 10.0 | 52.6 | 0.90 | 2,273,367 | 45,103 |
| JoinUnionAppend | 1928 | 59.0 | 32.7 | 0.95 | 4,726,362 | 105,553 |
| GroupVarDev | 221 | 7.5 | 29.6 | 1.17 | 86,323 | 142 |
| JoinPlusAddShared | 1549 | 59.6 | 26.0 | 0.76 | 2,620,145 | 44,168 |
| ExecVectorWhere | 138 | 5.8 | 24.0 | 0.99 | 372,664 | 6,197 |
| JoinWindow1Last | 3661 | 176.0 | 20.8 | 0.92 | 4,426,941 | 251,659 |
| JoinWindowAggSum | 4007 | 194.9 | 20.6 | 0.98 | 5,462,788 | 283,891 |
| GroupFirstLastCount | 130 | 6.4 | 20.2 | 1.49 | 155,856 | 8,345 |
| GroupXbarBucketSum | 3471 | 178.6 | 19.4 | 1.02 | 3,086,434 | 137,583 |
| DeleteColumns | 38 | 6.6 | 5.8 | 7.44 | 607,176 | 94 |
| DeleteEmptyMatchBoundary | 52 | 10.2 | 5.1 | 1.09 | 588,723 | 113 |
| GroupMedMinMax | 583 | 144.9 | 4.0 | 1.12 | 489,641 | 16,634 |
| DeleteWhereInactive | 61 | 16.8 | 3.7 | 0.97 | 502,774 | 113 |
| JoinLeftNullFill | 144 | 57.1 | 2.5 | 0.95 | 330,092 | 6,270 |
| JoinChainedInnerLeft | 218 | 102.3 | 2.1 | 0.95 | 803,289 | 193 |
| JoinInnerAliasedKeyTopK | 277 | 737.0 | 0.4 | 1.06 | 31,164 | 127 |

(JoinInnerAliasedKeyTopK's Go baseline uses a full stable sort; the legacy
heap-based `BenchmarkQSQLNativeGoJoinTopK` baselines remain the fair parity
reference for that fused shape. Leia's pruned topK join path beating the
full-sort baseline is the expected outcome.)

## Findings

### 1. Keyed-frame mutations re-key the whole table per operation

`InsertKeyedNewKey` (458x), `UpsertKeyedExistingKey` (371x),
`UpdateKeyedWhere` (204x), `DeleteKeyedWhere` (167x) all allocate 3.6-8.3 MB
and 57k-140k objects to mutate one row (or one column slice) of an 8192-row
keyed frame. The non-insert keyed paths run the plain mutation then call
`data.KeyBy` from scratch; the keyed insert/upsert path
(`KeyedFrame.mutate`) rebuilds every column through boxed `[]any` values. A
single-row delta should be O(delta + touched columns), not O(table).

### 2. Asof joins gather row-at-a-time through boxed values

The whole asof family sits at 146-203x with ~150k allocs/op — roughly 19
allocations per left row. `AsofMatchIndexes` plus `joinGatherOptional`
materialize through `any` per cell. The partition-sorted two-pointer match
itself is cheap (the Go baseline does it in 12-16us); the cost is the boxed
gather and output materialization. A typed gather kernel for
Symbol/Timestamp/F64 columns with an `[]int` index vector would close most of
the gap. The same applies to window joins (20x, 250k-280k allocs/op), where
per-row list materialization dominates.

### 3. Multi-key group-by misses the typed query kernel

`GroupMultiKeySumCountAvg` (sym,venue keys) runs 119x behind Go with 42k
allocs/op while the legacy single-key `GroupByAggregate` row is near parity —
multi-key grouping falls back to string `rowKey` building per row. Extending
the compiled query-kernel group path to multi-column keys (composite int key
or two-level dictionary) is the highest-leverage grouped-analytics item.
`GroupXbarBucketSum` (19x, 137k allocs) additionally pays per-row boxed xbar
evaluation for the computed temporal key.

### 4. Plain mutations rebuild every column through []any

`update`/`insert`/`upsert` on a plain 6-column frame cost 0.5-1.1ms and
45k-85k allocs even when the where-clause matches nothing
(`UpdateEmptyMatchBoundary`, 53x): `UpdateWhere`/`InsertRow` call
`Column.Values()` (boxing all rows) per column. Column-level copy-on-write
(share unmodified columns, typed append for the delta) would collapse the
whole mutation family. `DeleteWhere` rows (3.7-5.8x) already use typed
gather and show what the floor looks like.

### 5. Exec and first/last grouping pay per-row boxing in cheap shapes

`ExecVectorWhere` (24x, 6.2k allocs) and `GroupFirstLastCount` (20x, 8.3k
allocs) are small absolute costs but show per-row allocation in paths that
should be allocation-free given the typed filter kernels already exist for
select-where shapes.

### 6. Cold-vs-warm is flat for execution-bound shapes

Cold/warm is ~1.0 for 27 of 29 cases: parse/lower/plan-bind cost is invisible
behind execution for these heavy shapes. Only `DeleteColumns` (7.4x
cold/warm) — the one case whose execution is trivially cheap — shows the plan
cache earning its keep. Interpretation: for the gap families above, kernel
work, not caching, is the optimization target.

## Recommended optimization priorities (next round)

1. **Keyed-frame single-row mutation fast path** — O(delta) keyed
   insert/upsert/update/delete: reuse the existing key index, copy-on-write
   columns. Targets the 167-458x rows.
2. **Typed asof/window join gather kernels** — typed index gather for
   Symbol/Timestamp/F64 plus list-free windowed aggregates. Targets the
   146-203x asof family and both 20x window rows.
3. **Multi-key and computed-key (xbar) group-by in the query kernel** —
   composite typed keys instead of per-row string `rowKey`. Targets 119x
   multi-key and 19x xbar rows.
4. **Copy-on-write plain mutations** — share untouched columns in
   UpdateWhere/InsertRow/UpsertRow; typed assignment evaluation. Targets the
   53-65x update/insert/upsert rows.
5. **Union/plus join typed materialization** — boxed per-cell `At`/`ApplyBinary`
   loops in `UnionJoinOn`/`PlusJoinOn` (33x/26x, 44k-105k allocs).

Per the coverage doc's optimization rule, all of these are reusable
runtime/kernel mechanisms — none may special-case benchmark names, literals,
or row counts.

## Coverage state after this round

- qSQL Go-benchmark rows: 23 legacy + 87 matrix = **110** (was ~23).
- Source-derived gate (`TestQSQLBenchmarkCoverageMatchesSourceSurface`):
  all 6 query kinds, all 10 parser join kinds, 10/11 lowerable aggregates
  covered; `wavg` backlogged (qSQL lowering emits no weight expression —
  execution fails), backlog frozen at max 1.
- Matrix case floor pinned at 29 (`qSQLMatrixCaseFloor`); row-invariant
  baseline allowlist frozen at 0.
