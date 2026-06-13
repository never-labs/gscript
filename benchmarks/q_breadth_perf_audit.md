# q.eval Breadth Performance Audit

Date: 2026-06-11

Status: **historical snapshot**. This audit records the post-wave-2 state from
2026-06-11 and is kept as a worklist reference, not as the current coverage
contract. Current ordinary `q.eval` performance reporting treats the synthetic
suite as **483 cases** and includes the JIT script layer through
`BenchmarkQEvalJITScriptWarm` rows in the q.eval benchmark/report join.

Baseline: `0844e0ef benchmarks: refresh q go-ratio baseline after wave-2, ratchet caps 320/56 -> 64/8`

Data sources:

- `/tmp/q_suite_run3.txt` — canonical post-wave-2 run of
  `benchmarks/q_performance_suite.sh` (qSQL bind/native rows, the four
  483-case q.eval families, and, at this snapshot date, the 44-case JIT/VM
  script layer)
- `benchmarks/data/qeval_go_ratio_baseline.json` — per-case Go ratios captured
  from the same run (483 cases; 459 trusted warm ratios, 44 jit ratios in this
  historical capture)

Machine:

```text
goos: darwin
goarch: arm64
cpu: Apple M4 Max
```

## Summary

Two build-out phases (case growth to 483 with honest row-scaled Go baselines;
source-derived verb/form/combo coverage gates) and two optimization waves are
in. The wave-1+2 fixes that landed:

- **fills O(n^2) eliminated** (`806e9922`): forward-fill and nullable scalar
  fill materialize as typed kernels in one O(n) pass; the worst fills+msum
  combos went from ~400ms/op to ~126us/op.
- **Compound-predicate bulk masks + `not` precedence fix** (`c047ff6d`):
  3-4 clause where predicates bulk-materialize lazy mask trees instead of
  per-row dispatch (was 100-500x slower than Go); q `not` now correctly
  negates everything to its right on the script-binding parse route.
- **`OpQEvalSessionEval` JIT route** (`c901ad9d`): constant-source
  `q.session().eval` lowers to a per-iteration Tier 2 op-exit, making the
  JIT-script layer a real per-iteration measurement.
- **Typed adverb kernels** (`bb1ca2df`): each-prior/over/scan route through
  typed columnar kernels.
- **Float/mixed numeric bulk kernels** (`ddc63fd8`): bulk typed kernels
  extended beyond int to float and mixed numeric shapes.
- **Set-op kernels + plan persistence fix** (`f36f1f76`): typed set-op
  kernels (`inter`/`except`/`union`/`distinct`), constant-statement memoization,
  plan-cache probe reorder, and lazy-carrier materialization; statements now
  execute through pointers so per-statement fast plans actually persist.

Where that left the suite in this historical snapshot (459 trusted warm cases,
44 jit cases):

| Layer | Geomean Leia/Go | Beat Go | >= 10x slower |
|---|---:|---:|---:|
| Session-warm | **0.99** | 253 / 459 | 68 |
| JIT script | **0.47** | 33 / 44 | 0 |

The geomean is at Go parity warm and well under it on the JIT route, but the
warm tail is long: 36 trusted cases are still >= 20x. Hard caps were
tightened from 320/56 to **64 (warm) / 8 (jit)** in `milestone_caps`, with the
x1.15 per-case no-regression ratchet as the primary guard.

Fallback pressure is nearly gone: 8 of 483 warm rows report any typed-kernel
fallbacks. The worst two are `TypeMatrixLongNullNotEqualEqualNullCount`
(9% hit, 31 fallbacks/op) and `ComboNullMaskArithWithinEnvelope` (42% hit,
7 fallbacks/op); everything else is at or near 100% hit with the remaining
gap coming from shell overhead and intermediate materialization, not missing
kernels.

## Worst Warm vs Go Ratios

Trusted Go baselines only (row-scaled, allowlist-checked). All numbers from
`/tmp/q_suite_run3.txt`; ratios match `qeval_go_ratio_baseline.json`.

| Case | Warm ns/op | Go ns/op | Warm/Go | B/op | allocs/op | typed hit | fallback/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `VerbFormPipeLogicalMaskCount` | 274,201 | 4,535 | 60.5x | 148,176 | 25 | 100% | 0 |
| `VerbFormAmpLogicalMaskCount` | 223,516 | 4,334 | 51.6x | 148,176 | 25 | 100% | 0 |
| `TypeMatrixWhereNullGuardCompoundGatherSum` | 567,780 | 12,410 | 45.8x | 399,433 | 172 | 100% | 0 |
| `DeepReverseWhereWindowCountSmall` | 109,519 | 2,727 | 40.2x | 2,048 | 64 | 100% | 0 |
| `DeepReverseWhereWindowCountMedium` | 111,095 | 2,912 | 38.2x | 2,056 | 65 | 100% | 0 |
| `TypeMatrixCastBoolMaskGatherSum` | 155,835 | 4,106 | 38.0x | 940,288 | 67 | 100% | 0 |
| `DeepReverseWhereWindowCountWide` | 108,313 | 2,858 | 37.9x | 2,048 | 64 | 100% | 0 |
| `DeepReverseWhereWindowCountCycle` | 109,218 | 2,925 | 37.3x | 2,056 | 65 | 100% | 0 |
| `FunctionalAmendWhereVector` | 77,156 | 2,086 | 37.0x | 69,920 | 86 | 100% | 0 |
| `TypeMatrixCastRealLongRoundTripSum` | 331,815 | 9,322 | 35.6x | 423,288 | 23,892 | 100% | 0 |
| `ComboNullMaskArithWithinEnvelope` | 361,084 | 10,375 | 34.8x | 15,536 | 178 | 42% | 7 |
| `DeepDeltasFillsChecksumInts` | 161,815 | 4,659 | 34.7x | 264,976 | 43 | 100% | 0 |
| `TypeMatrixCastRealLetterQuarterSum` | 340,918 | 11,030 | 30.9x | 295,994 | 16,189 | 100% | 0 |
| `TypeMatrixLongNullVectorAddVectorSum` | 243,558 | 8,180 | 29.8x | 424,712 | 11,923 | 100% | 0 |
| `TypeMatrixLongNullNotEqualEqualNullCount` | 278,247 | 9,408 | 29.6x | 161,672 | 236 | 9% | 31 |

Families, not cases, are the units here:

- **VerbForm logical masks (50-60x)**: `a & b` / `a | b` over int vectors
  followed by `count where` — the bulk mask kernels cover compare leaves but
  the bare int-vector `&`/`|` form still materializes boxed intermediates.
- **Null-guard compound where (46x)** and the long-null arithmetic/compare
  rows (~30x): null carriers still pay boxed per-row paths inside otherwise
  typed pipelines.
- **DeepReverseWhereWindow\* (37-40x)**: tiny allocation (2KB/op), 100% typed
  hit, yet ~109us warm vs ~2.8us Go — almost pure q.eval statement shell, the
  cleanest exhibit for the wave-3 shell work.
- **FunctionalAmendWhere (37x)**: functional amend over a where mask lacks a
  fused typed kernel.
- **Cast real/long round trips (31-36x)**: cast chains allocate 12k-24k
  times per op; cast carriers need bulk materialization like the wave-2
  compare/arith ones.

## Worst JIT-Script vs Go Ratios

All 44 rows in this snapshot were under the 8x hard cap (was 56). VM column
included to show the JIT loop itself was not the gap — the residual cost was in
the q runtime work per op-exit. Current `BenchmarkQEvalJITScriptWarm` rows also
emit q session route metrics:

| Metric | Meaning |
|---|---|
| `q_session_planned_op_exit/op` | per-iteration planned `OpQEvalSessionEval` JIT op-exit route |
| `q_session_shell_fallback/op` | fallback to shell eval instead of the planned route |
| `q_session_eval_errors/op` | eval failures seen through the q session route |
| `q_session_backend_shapes` | distinct backend-lowered q session shapes observed by the benchmark row |

| Case | JIT ns/op | VM ns/op | Go ns/op | JIT/Go | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `WhereGatherProjectionSum` | 6,665 | 6,602 | 1,085 | 6.1x | 3,411 | 55 |
| `BreadthListCutRazeChecksumLong` | 7,135 | 7,038 | 1,843 | 3.9x | 2,368 | 68 |
| `AdverbInitialOverScanProducts` | 7,716 | 7,762 | 2,168 | 3.6x | 2,497 | 75 |
| `BreadthListCutRazeChecksumShort` | 5,902 | 5,738 | 1,961 | 3.0x | 2,240 | 52 |
| `TaskDMathSqrtLogVectorSum` | 55,950 | 55,524 | 24,338 | 2.3x | 456 | 13 |
| `FbyGroupedAggregateRowScaled` | 8,938 | 8,675 | 4,342 | 2.1x | 2,551 | 25 |
| `NumericMonadExpReciprocalSignumNot` | 55,859 | 56,780 | 30,677 | 1.8x | 2,222 | 58 |
| `BreadthFloatFloorCeilingReciprocalMod3Bias1` | 17,870 | 18,051 | 10,832 | 1.6x | 889 | 25 |

## Allocation Hotspots (warm layer)

| Case | Warm ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| `ComboDictTableXgroupUngroupSum` | 1,387,264 | 3,240,179 | 33,187 |
| `TypeMatrixCastBoolMaskGatherSum` | 155,835 | 940,288 | 67 |
| `VerbFormWordAliasLeftRightGather` | 366,998 | 781,486 | 31,809 |
| `VerbFormXdescColumnProbe` | 553,484 | 528,372 | 116 |
| `TypeMatrixLongNullVectorAddVectorSum` | 243,558 | 424,712 | 11,923 |
| `TypeMatrixCastRealLongRoundTripSum` | 331,815 | 423,288 | 23,892 |
| `ComboDictTableFbyNotionalSum` | 132,153 | 402,919 | 281 |
| `ComboDictTableXascHeadGatherSum` | 1,166,158 | 399,640 | 200 |
| `TypeMatrixWhereNullGuardCompoundGatherSum` | 567,780 | 399,433 | 172 |
| `ComboDepth3SumsModXbarReduce` | 222,662 | 392,880 | 16,308 |

Two distinct profiles: dict/table chains (`xgroup`/`ungroup`, `xasc`/`xdesc`)
allocate megabytes through frame materialization, while the cast/null
TypeMatrix rows allocate tens of thousands of small boxes — per-row boxing
that bulk carrier kernels should remove.

## Recommended Priority

Wave-3 agents are already working items 1 and 2.

1. **q.eval per-statement shell (~23us/statement)** — the identified next
   bottleneck. Shell-bound rows like `DeepReverseWhereWindowCount*` spend
   ~109us warm against ~2.8us of Go data work with only 2KB/op allocated and
   100% typed hits: the cost is statement splitting, top-level operator
   scanning, and plan lookup around each statement
   (`EvalState.evalCachedOrString` and the `findTopLevel` /
   `splitTopLevelOperator` / `TrimSpace` helpers dominate the shell profile),
   not kernel work. *In progress (wave 3).*
2. **Binding-plan coverage extension** — widen the set of expressions the
   script-binding/JIT route can lower so more of the 483-case table gets
   JIT-layer evidence (currently 44 representative rows) and more statements
   skip the string-scanning shell entirely. *In progress (wave 3).*
3. **Bare int-vector logical mask kernels** — route `a & b` / `a | b` mask
   composition (no compare leaf) through the wave-2 bulk mask machinery; this
   alone covers the two worst rows (50-60x).
4. **Null-carrier and cast bulk kernels** — extend bulk materialization to
   long-null arithmetic/compare and cast chains (real/long/letter round
   trips); kills the 30-46x TypeMatrix tail, the two remaining fallback-heavy
   rows, and the 12k-24k allocs/op boxing.
5. **Fused functional-amend-over-where and reverse-where-window shapes** —
   `FunctionalAmendWhereVector` (37x) and the window-count family need fused
   typed kernels rather than mask + gather + amend stages.
6. **Dict/table chain materialization** — `xgroup`/`ungroup` and sort-head
   chains allocate 0.4-3.2MB/op; column views and reusable buffers apply.
