# q.eval Breadth Performance Audit

Date: 2026-06-10

Baseline: `fd2c676c benchmarks: merge expanded q coverage`

Command:

```bash
go test ./benchmarks -run '^$' -bench 'BenchmarkQ(SessionEvalVectorWarmExecution|EvalVectorCold|EvalVectorGoBaseline)/Breadth' -benchmem -benchtime=200ms -count=1
```

Machine:

```text
goos: darwin
goarch: arm64
cpu: Apple M4 Max
```

## Summary

The new breadth suite is useful enough to expose the next runtime priorities, but some Go baselines are not yet reliable. Cases with sub-10 ns/op Go baselines are likely folded to constants or reduced to a scalar formula by the compiler, so their warm/go ratios should not drive runtime decisions until the Go side is rewritten to perform equivalent work over input data.

Important signals:

- Most breadth cases report 100% typed-kernel hit rate, so the main gap is often shell overhead, intermediate materialization, or an overly narrow fused shape rather than total absence of typed runtime coverage.
- `BreadthSymbolDistinctGroupSort*` is the only breadth family with visible fallback pressure: 80% hit rate, 1 fallback/op.
- The highest allocation families are symbol distinct/group/sort, div/mod envelope, and aggregate avg/med/range.
- Warm execution is not consistently faster than cold for the largest allocation cases, which means schema-stable cache is not yet removing the dominant work there.

## Worst Valid Warm vs Go Ratios

Only rows with Go baseline >= 10 ns/op are included here.

| Case | Warm ns/op | Go ns/op | Warm/Go | B/op | allocs/op | typed hit | fallback/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `BreadthApplyBracketGatherWide` | 2,860 | 67.1 | 42.6x slower | 157 | 10 | 100% | 0 |
| `BreadthArithmeticDivModEnvelopeMod5Bias2` | 221,160 | 7,775 | 28.4x slower | 383,571 | 14,934 | 100% | 0 |
| `BreadthArithmeticDivModEnvelopeMod3Bias1` | 225,104 | 8,076 | 27.9x slower | 387,664 | 15,444 | 100% | 0 |
| `BreadthArithmeticDivModEnvelopeMod11Bias6` | 248,737 | 9,449 | 26.3x slower | 371,338 | 13,406 | 100% | 0 |
| `BreadthArithmeticDivModEnvelopeMod7Bias4` | 206,124 | 8,103 | 25.4x slower | 379,729 | 14,426 | 100% | 0 |
| `BreadthApplyBracketGatherMedium` | 2,517 | 127.4 | 19.8x slower | 161 | 10 | 100% | 0 |
| `BreadthListDropTakeSublistMid` | 2,442 | 149.2 | 16.4x slower | 224 | 12 | 100% | 0 |
| `BreadthApplyBracketGatherSmall` | 2,498 | 247.6 | 10.1x slower | 157 | 10 | 100% | 0 |
| `BreadthListCutRazeChecksumLong` | 17,635 | 1,772 | 10.0x slower | 2,231 | 67 | 100% | 0 |
| `BreadthFloatFloorCeilingReciprocalMod5Bias2` | 108,020 | 10,925 | 9.9x slower | 2,544 | 110 | 100% | 0 |

## Suspect Go Baselines

These Go baselines are below 10 ns/op and should be fixed before comparing ratios:

| Case | Warm ns/op | Go ns/op | B/op | allocs/op | Note |
| --- | ---: | ---: | ---: | ---: | --- |
| `BreadthAggregateAvgMedRange` | 90,343 | 0.744 | 130,828 | 8,017 | Go is a closed-form formula |
| `BreadthAggregateProductOnesAndWavg` | 33,480 | 0.741 | 5,545 | 133 | Go is constant/closed-form |
| `BreadthRunningMinMaxAvgEnvelope` | 14,552 | 0.744 | 1,648 | 73 | Go is closed-form |
| `BreadthCallableDotApply*` | 10,418-13,387 | 0.742-0.775 | ~7,270 | 48 | Go is closed-form |
| `BreadthApplyAtGather*` | 2,421-2,527 | 0.742-0.912 | 296 | 15 | Go is closed-form |
| `BreadthMatrixReshapeCellProbe*` | 406-471 | 0.744-0.907 | 167-168 | 9 | Go is closed-form |
| `BreadthListDropTakeSublistShort` | 2,011 | 8.276 | 207 | 10 | Too small to be stable |

## Allocation Hotspots

| Case | Warm ns/op | B/op | allocs/op | typed hit | fallback/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| `BreadthSymbolDistinctGroupSortSymbolsN` | 988,955 | 460,936 | 8,266 | 80% | 1 |
| `BreadthSymbolDistinctGroupSortSymbolsA` | 637,750 | 460,728 | 8,264 | 80% | 1 |
| `BreadthSymbolDistinctGroupSortVenuesX` | 708,316 | 460,728 | 8,264 | 80% | 1 |
| `BreadthArithmeticDivModEnvelopeMod3Bias1` | 225,104 | 387,664 | 15,444 | 100% | 0 |
| `BreadthArithmeticDivModEnvelopeMod5Bias2` | 221,160 | 383,571 | 14,934 | 100% | 0 |
| `BreadthArithmeticDivModEnvelopeMod7Bias4` | 206,124 | 379,729 | 14,426 | 100% | 0 |
| `BreadthArithmeticDivModEnvelopeMod11Bias6` | 248,737 | 371,338 | 13,406 | 100% | 0 |
| `BreadthAggregateAvgMedRange` | 90,343 | 130,828 | 8,017 | 100% | 0 |

## Warm vs Cold

Warm execution is only materially better for small shell-heavy cases. For the large allocation families, warm is flat or sometimes slower:

| Case | Warm ns/op | Cold ns/op | Warm/Cold |
| --- | ---: | ---: | ---: |
| `BreadthArithmeticDivModEnvelopeMod11Bias6` | 248,737 | 202,181 | 1.23 |
| `BreadthArithmeticDivModEnvelopeMod3Bias1` | 225,104 | 208,295 | 1.08 |
| `BreadthArithmeticDivModEnvelopeMod5Bias2` | 221,160 | 214,354 | 1.03 |
| `BreadthSymbolDistinctGroupSortSymbolsN` | 988,955 | 987,155 | 1.00 |
| `BreadthSymbolDistinctGroupSortVenuesX` | 708,316 | 709,198 | 1.00 |

This points to runtime data work dominating plan construction for those shapes. The cache is useful, but it is not sufficient until these pipelines avoid materializing intermediate vectors.

## Recommended Priority

1. Fix suspect Go baselines in the benchmark suite. Every Go baseline should perform equivalent row/list work and write into a package-level sink so the compiler cannot erase it. Do this before using ratios as release gates.
2. Add fused producer-reducer shapes for `sum(y div k)+sum(y mod k)+count y` and related integer dyadic chains. This is the largest valid gap: 25-28x slower than Go and 13k-15k allocs/op despite 100% typed hits.
3. Add direct gather-sum/count and sublist/drop/take descriptor shapes. Current absolute times are small, but ratios are 10-42x slower and allocations show expression-shell overhead instead of data-work limits.
4. Lower symbol `distinct/group/iasc` into a single typed symbol pipeline. This is the only breadth family with fallback pressure and the largest absolute runtime/allocation cost.
5. Add aggregate descriptor shapes for avg/med/min/max/count and product/wavg once Go baselines are fixed. Current allocs are high, but ratio evidence is invalid until the baseline is repaired.
6. Reduce warm-path allocation in callable dot apply and matrix cell probes. These are already fast in absolute terms, but the 7-48 alloc/op shell cost keeps them from becoming true near-zero overhead primitives.

