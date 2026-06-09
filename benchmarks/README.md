# Leia Benchmarks

Benchmarks are grouped by capability domain:

| Domain | Focus |
|---|---|
| `numeric` | math, matrices, nbody, spectral norm |
| `recursion` | recursive calls and recursive data structures |
| `table` | arrays, maps, sort, traversal, metamethod tables |
| `calls` | closures, varargs, coroutine and method dispatch |
| `string` | strings, patterns, regexp-style workloads |
| `concurrency` | goroutine-like runtime features |
| `data` | data-oriented and SoA kernels |
| `app` | larger application-shaped workloads |
| `control` | control-flow and protected execution |
| `precision` | numeric precision checks and stability probes |

LuaJIT references live under `benchmarks/lua_ref/<domain>/`.

## Main Commands

```bash
# Fast grammar-change hot-path gate; writes reports under /tmp by default.
bash scripts/performance_gate.sh --syntax-smoke --no-luajit

# Current worktree vs clean HEAD vs LuaJIT.
bash scripts/performance_gate.sh --full

# Investigation-only current worktree vs clean HEAD vs LuaJIT.
python3 benchmarks/timing_compare.py --all-groups --runs=5 --warmup=1 \
  --time-source=auto --min-sample-seconds=0.100 --max-repeat=128 \
  --sort=luajit-gap \
  --json /tmp/leia_timing_compare.json \
  --markdown /tmp/leia_timing_compare.md

# Strict truth pass across hot, script-timed groups.
python3 benchmarks/strict_guard.py --runs=3 --warmup=1 --timeout=90 \
  --json benchmarks/data/strict_guard_latest.json \
  --markdown benchmarks/data/strict_guard_latest.md

# Representative subset while iterating.
python3 benchmarks/strict_guard.py --runs=3 --warmup=0 --max-repeat=8 \
  --bench=numeric/matmul --bench=numeric/matmul_row \
  --bench=table/json_table_walk

# q in-memory columnar analytics suite.
bash benchmarks/q_columnar_suite.sh --runs=3 --warmup=1 \
  --time-source=auto --min-sample-seconds=0.100 --max-repeat=128 \
  --json /tmp/leia_q_columnar_timing.json \
  --markdown /tmp/leia_q_columnar_timing.md

# Full q performance suite: q columnar scripts plus Go-level qSQL/q.eval
# comparisons against hand-written Go with allocation metrics.
LEIA_GO_BENCHTIME=100x bash benchmarks/q_performance_suite.sh --runs=3 --warmup=1 \
  --time-source=auto --min-sample-seconds=0.100 --max-repeat=128

# q performance completeness report: qSQL plus ordinary q.eval/list/vector
# workloads against hand-written Go, with warm/cold, cache/fallback, and allocs.
python3 benchmarks/q_perf_report.py --benchtime=100x \
  --json benchmarks/data/q_perf_report_latest.json \
  --markdown benchmarks/data/q_perf_report_latest.md

# q stateful eval ordinary compute vs hand-written Go with reusable buffers.
BENCHTIME=100x bash benchmarks/q_general_compute_suite.sh

# q.eval vector/list compute microbenchmarks with Go baselines and allocs/op.
go test ./benchmarks -run '^TestQEvalVectorBenchmarkExpressions$' \
  -bench 'Benchmark(QEvalVector(ResultCacheWarm|Cold|GoBaseline)|QSessionEvalVectorWarmExecution)' \
  -benchmem

# Hot-loop scaling profile for low-resolution workloads.
python3 benchmarks/timing_compare.py --runs=5 --warmup=1 \
  --scale-profile=hot --sort=luajit-gap \
  --json /tmp/leia_hot_timing.json \
  --markdown /tmp/leia_hot_timing.md

# Semantic-family performance coverage audit.
bash benchmarks/coverage_guard.sh
```

Use `--syntax-smoke` after lexer/parser/grammar-only work when you need a
quick current-vs-HEAD check over control, calls, table, string, and data hot
paths plus the Leia-only dialect truth pass. Add `--no-strict` only when the
change does not need VM/default/no-filter output stability evidence.

## Diagnostics

```bash
python3 benchmarks/profile_exits.py --bench=numeric/spectral_norm --top=30

python3 benchmarks/triage.py --bench=numeric/spectral_norm \
  --scale=numeric/spectral_norm:N=2000 --time-source=script \
  --diag --pprof --memprofile --warm-dump --out-dir=/tmp/leia-triage

python3 benchmarks/diagnose.py \
  --bench=calls/method_dispatch \
  --bench=table/groupby_nested_agg \
  --out-dir=/tmp/leia-diagnose

bash scripts/diag.sh table/table_array_access
```

`timing_compare.py` is the primary local optimization harness. It records timing
source, repeat count, CI, parameter scaling, current-vs-HEAD deltas, and LuaJIT
gaps.

`strict_guard.py` is the release/regression truth pass for hot, script-timed
workloads. It runs VM, default JIT, no-filter JIT, and LuaJIT where a reference
exists, then checks output stability and timing quality. Concurrency benchmarks
remain available via explicit `--group=concurrency` or `--bench=...`, but they
are not part of the default strict pass because many are semantic or blocking
wall-time workloads rather than comparable hot loops.

Benchmark selectors are domain IDs such as `numeric/matmul` or
`table/json_table_walk`. Historical selector families are intentionally not
accepted by the harness.

## q Columnar Analytics

`benchmarks/q_performance_suite.sh` is the broad q performance entrypoint. It
runs the focused q analytics script suite and then runs Go benchmarks for qSQL
and ordinary q session expressions against hand-written Go baselines with
`ns/op`, `B/op`, and `allocs/op`.

See [q_benchmark_coverage.md](q_benchmark_coverage.md) for the q language
benchmark coverage audit. That document maps semantic q coverage from
`eval_test.go`, `parser_test.go`, and `bind/q_test.go` to the benchmark
dimensions that still need performance rows.

`benchmarks/q_columnar_suite.sh` runs the focused q analytics baseline for
runtime and JIT work. The script suite is Leia-only and compares the current
worktree against a clean HEAD build over these stable shapes:

| Benchmark | Focus |
|---|---|
| `data/q_columnar_eval_primitives` | q vector arithmetic, compare masks, `where`, `xbar`, and adverb reducers |
| `data/q_columnar_qsql_filter_project` | qSQL typed filter, computed projection, order, and take |
| `data/q_columnar_qsql_group_xbar` | qSQL grouped aggregation over symbol and temporal bucket keys |
| `data/q_columnar_qsql_asof_join` | qSQL partitioned temporal asof join with projection and filter |

The Go-level q benchmarks add these direct comparison families:

| Benchmark family | Focus |
|---|---|
| `BenchmarkQSQLBind...` | user-facing qSQL warm/cold/cache paths |
| `BenchmarkQSQLDataRuntime...` | direct Frame/runtime primitive paths |
| `BenchmarkQSQLNativeGo...` | simple and optimized hand-written Go qSQL baselines |
| `BenchmarkQSessionEvalVectorWarmExecution/...` | ordinary q vector/list/math/adverb session expressions without global result-cache hits |
| `BenchmarkQEvalVectorGoBaseline/...` | hand-written Go baselines for the same ordinary q shapes |

Use the full wrapper after q runtime, frame/vector, schema-cache, or JIT path
changes to keep measurements comparable:

```bash
LEIA_GO_BENCHTIME=100x bash benchmarks/q_performance_suite.sh --runs=5 --warmup=1 \
  --time-source=auto --min-sample-seconds=0.100 --max-repeat=128
```

To run only the Go-level qSQL and q.eval comparison benchmarks:

```bash
go test ./internal/stdlib/bind -run '^$' \
  -bench 'BenchmarkQSQL(Bind|DataRuntime|NativeGo)' \
  -benchmem -benchtime=100x

go test ./benchmarks -run '^TestQEvalVectorBenchmarkExpressions$' \
  -bench 'Benchmark(QEvalVector(ResultCacheWarm|Cold|GoBaseline)|QSessionEvalVectorWarmExecution)' \
  -benchmem -benchtime=100x
```

For the fuller q performance report, prefer:

```bash
python3 benchmarks/q_perf_report.py --benchtime=100x \
  --json benchmarks/data/q_perf_report_latest.json \
  --markdown benchmarks/data/q_perf_report_latest.md
```

That report runs:

```bash
go test ./internal/stdlib/bind -run '^$' \
  -bench 'BenchmarkQSQL(...)' -benchmem -benchtime=100x

go test ./benchmarks -run '^$' \
  -bench 'Benchmark(QEvalVector|QSessionEvalVector)(...)' -benchmem -benchtime=100x
```

It intentionally separates qSQL from ordinary `q.eval` list/vector/math/adverb
workloads. `q.eval` now has hand-written Go baselines and warm/cold cache
measurements for vector arithmetic/reduce, compare/where, list slice/reduce,
adverb/running/moving aggregate, symbol/string/temporal, typed/null/cast, and
composition shapes. qSQL bind benchmarks expose `kernel_hit_pct` and
`fallbacks/op`; ordinary `q.eval` session benchmarks also report
`typed_kernel_*` counters where runtime kernel instrumentation is available.

Read the q performance baseline through these ratios:

| Signal | How to read it |
|---|---|
| Current Leia vs old Leia | `benchmarks/q_columnar_suite.sh` reports current worktree vs clean `HEAD`; compare `Current`, `HEAD`, and `HEAD delta` |
| Current Leia vs hand-written Go | compare `BenchmarkQSQLBind...` rows with `BenchmarkQSQLNativeGo...` rows for qSQL; compare `BenchmarkQSessionEvalVectorWarmExecution/...` with `BenchmarkQEvalVectorGoBaseline/...` for ordinary q compute |
| Warm vs cold | compare `BenchmarkQSQLBindRunSQLWarmCache...` with `BenchmarkQSQLBindRunSQLColdCache...` |
| Typed kernel hit/fallback rate | use `kernel_hit_pct`, `template_hit_pct`, `aligned_hit_pct`, and `fallbacks/op` in bind benchmark output; use `typed_kernel_hit_pct` and `typed_kernel_fallbacks/op` in ordinary q session rows |
| Allocation pressure | use `B/op` and `allocs/op`; q columnar hot paths should trend toward low per-row allocation |

Generate a machine-readable q performance report from the focused qSQL and
ordinary q benchmark rows:

```bash
python3 benchmarks/q_perf_report.py --benchtime=100x
```

The report writes `benchmarks/data/q_perf_report_latest.md` and
`benchmarks/data/q_perf_report_latest.json`, including Leia-vs-Go ratios,
warm/cold ratios, typed kernel hit/fallback counters, allocation metrics, and
fallback shape summary rows.

Use `--check` to turn the same report into a performance gate:

```bash
python3 benchmarks/q_perf_report.py \
  --benchtime=100x \
  --check \
  --max-leia-go-ratio=5 \
  --min-typed-hit-pct=95 \
  --max-typed-fallbacks-op=0 \
  --max-pipeline-fallback-shapes=0 \
  --max-allocs-op=64
```

For quick iteration on saved `go test -bench` output, skip rerunning benchmarks:

```bash
python3 benchmarks/q_perf_report.py \
  --from-output /tmp/qbench.txt \
  --check
```

## q.eval Vector/List Compute

`benchmarks/q_eval_vector_bench_test.go` covers ordinary q expressions outside
qSQL. This suite is the main non-qSQL q performance matrix and now has 151 cases
plus required coverage-tag, matrix, and semantic-shape gates. It spans numeric
vector arithmetic, typed suffix/null/cast/promotion behavior, compare masks to
`where`, selectivity changes, `take`/`drop`/`cut`/`reverse`/`rotate`,
reductions, adverbs, running and moving aggregates, list/set/search verbs,
dictionaries and amend/upsert, symbols/enums, string transforms, temporal
values, table/keyed-table transforms, safe system commands, and loopback IPC.
Each shape has four benchmark rows:

| Row | Signal |
|---|---|
| `BenchmarkQEvalVectorResultCacheWarm/...` | repeated `q.eval` of one cacheable expression after the current result cache is warm |
| `BenchmarkQEvalVectorCold/...` | equivalent `q.eval` expressions with distinct cache keys, exercising parse/lower and execution cost |
| `BenchmarkQSessionEvalVectorWarmExecution/...` | repeated execution through `q.session.eval`, bypassing the global q.eval result cache |
| `BenchmarkQEvalVectorGoBaseline/...` | hand-written Go loop for the same checksum |

Use `-benchmem` to compare `B/op` and `allocs/op`; use result-cache warm vs
cold ratios to judge q.eval cache value; use `QSessionEvalVectorWarmExecution`
vs Go baseline to see how much ordinary vector/list execution still pays in
bridge, allocation, and dynamic dispatch overhead.

`benchmarks/q_general_compute_suite.sh` is the short wrapper for this ordinary
q compute suite. It keeps the benchmarks under `benchmarks/` while still
exercising q result-cache warm, session warm execution, cold parse/lower, and
hand-written Go baseline rows in one command.

The q coverage target is now gate-based, not just count-based. Ordinary
`q.eval` coverage must pass `TestQEvalVectorBenchmarkCoverageTags`. qSQL still
needs an equivalent tag gate across select/group/join/mutation/cache/runtime
stats before the full q performance suite can be called exhaustive. Keep new
benchmark cases tied to real q language shapes from the semantic tests rather
than to individual optimizer implementation details.
