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
scripts/run.sh perf --syntax-smoke --no-luajit

# Current worktree vs clean HEAD vs LuaJIT.
scripts/run.sh perf --full

# Investigation-only current worktree vs clean HEAD vs LuaJIT.
go run ./cmd/leia bench compare --all-groups --runs=5 --warmup=1 \
  --time-source=auto --min-sample-seconds=0.100 --max-repeat=128 \
  --sort=luajit-gap \
  --json /tmp/leia_timing_compare.json \
  --markdown /tmp/leia_timing_compare.md

# Strict truth pass across hot, script-timed groups.
go run ./cmd/leia bench strict --runs=3 --warmup=1 --timeout=90 \
  --json benchmarks/data/strict_guard_latest.json \
  --markdown benchmarks/data/strict_guard_latest.md

# Representative subset while iterating.
go run ./cmd/leia bench strict --runs=3 --warmup=0 --max-repeat=8 \
  --bench=numeric/matmul --bench=numeric/matmul_row \
  --bench=table/json_table_walk

# q in-memory columnar analytics suite.
go run ./cmd/leia bench q-columnar --runs=3 --warmup=1 \
  --time-source=auto --min-sample-seconds=0.100 --max-repeat=128 \
  --json /tmp/leia_q_columnar_timing.json \
  --markdown /tmp/leia_q_columnar_timing.md

# Full q performance suite: q columnar scripts plus Go-level qSQL/q.eval
# comparisons against hand-written Go with allocation metrics.
LEIA_GO_BENCHTIME=100x go run ./cmd/leia bench q-suite --runs=3 --warmup=1 \
  --time-source=auto --min-sample-seconds=0.100 --max-repeat=128

# q performance completeness report: qSQL plus ordinary q.eval/list/vector
# workloads against hand-written Go, with warm/cold, cache/fallback, and allocs.
go run ./cmd/leia bench q-report --benchtime=100x \
  --json benchmarks/data/q_perf_report_latest.json \
  --markdown benchmarks/data/q_perf_report_latest.md

# q stateful eval ordinary compute vs hand-written Go with reusable buffers.
BENCHTIME=100x go run ./cmd/leia bench q-general

# q.eval vector/list compute microbenchmarks with Go baselines and allocs/op.
go test ./benchmarks -run '^TestQEvalVectorBenchmarkExpressions$' \
  -bench 'Benchmark(QEvalVector(ResultCacheWarm|Cold|GoBaseline)|QSessionEvalVectorWarmExecution|QEvalJITScriptWarm)' \
  -benchmem

# Hot-loop scaling profile for low-resolution workloads.
go run ./cmd/leia bench compare --runs=5 --warmup=1 \
  --scale-profile=hot --sort=luajit-gap \
  --json /tmp/leia_hot_timing.json \
  --markdown /tmp/leia_hot_timing.md

# Semantic-family performance coverage audit.
go run ./cmd/leia bench coverage
```

`--sort=luajit-gap` orders compare reports by the worst current/LuaJIT median
ratio so the largest runtime gaps appear first.

Use `--syntax-smoke` after lexer/parser/grammar-only work when you need a
quick current-vs-HEAD check over control, calls, table, string, and data hot
paths plus the Leia-only dialect truth pass. Add `--no-strict` only when the
change does not need VM/default/no-filter output stability evidence.

## Diagnostics

```bash
go run ./cmd/leia bench profile-exits --bench=numeric/spectral_norm --top=30

go run ./cmd/leia bench triage --bench=numeric/spectral_norm \
  --scale=numeric/spectral_norm:N=2000 --time-source=script \
  --diag --pprof --memprofile --warm-dump --out-dir=/tmp/leia-triage

go run ./cmd/leia bench diagnose \
  --bench=calls/method_dispatch \
  --bench=table/groupby_nested_agg \
  --out-dir=/tmp/leia-diagnose

scripts/run.sh diag table/table_array_access
```

`leia bench compare` is the primary local optimization harness. It records timing
source, repeat count, CI, parameter scaling, current-vs-HEAD deltas, and LuaJIT
gaps. JSON and Markdown reports also carry runtime/JIT observability counters:
Tier 2 attempted/entered/failed and total exits for each current sample and
summary row.

`leia bench strict` is the release/regression truth pass for hot, script-timed
workloads. It runs VM, default JIT, no-filter JIT, and LuaJIT where a reference
exists, then checks output stability and timing quality. Concurrency benchmarks
remain available via explicit `--group=concurrency` or `--bench=...`, but they
are not part of the default strict pass because many are semantic or blocking
wall-time workloads rather than comparable hot loops.

Benchmark selectors are domain IDs such as `numeric/matmul` or
`table/json_table_walk`. Historical selector families are intentionally not
accepted by the harness.

## q Columnar Analytics

`leia bench q-suite` is the broad q performance entrypoint. It
runs the focused q analytics script suite and then runs Go benchmarks for qSQL
and ordinary q session expressions against hand-written Go baselines with
`ns/op`, `B/op`, and `allocs/op`.

See [q_benchmark_coverage.md](q_benchmark_coverage.md) for the q language
benchmark coverage audit. That document maps semantic q coverage from
`eval_test.go`, `parser_test.go`, and `bind/q_test.go` to the benchmark
dimensions that still need performance rows.

`leia bench q-columnar` runs the focused q analytics baseline for
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
| `BenchmarkQEvalJITScriptWarm/...` | the same ordinary q shapes inside a JIT-compiled Leia hot loop |

Use the full wrapper after q runtime, frame/vector, schema-cache, or JIT path
changes to keep measurements comparable:

```bash
LEIA_GO_BENCHTIME=100x go run ./cmd/leia bench q-suite --runs=5 --warmup=1 \
  --time-source=auto --min-sample-seconds=0.100 --max-repeat=128
```

To run only the Go-level qSQL and q.eval comparison benchmarks:

```bash
go test ./internal/stdlib/bind -run '^$' \
  -bench 'BenchmarkQSQL(Bind|DataRuntime|NativeGo)' \
  -benchmem -benchtime=100x

go test ./benchmarks -run '^TestQEvalVectorBenchmarkExpressions$' \
  -bench 'Benchmark(QEvalVector(ResultCacheWarm|Cold|GoBaseline)|QSessionEvalVectorWarmExecution|QEvalJITScriptWarm)' \
  -benchmem -benchtime=100x
```

For the fuller q performance report, prefer:

```bash
go run ./cmd/leia bench q-report --benchtime=100x \
  --json benchmarks/data/q_perf_report_latest.json \
  --markdown benchmarks/data/q_perf_report_latest.md
```

That report runs:

```bash
go test ./internal/stdlib/bind -run '^$' \
  -bench 'BenchmarkQSQL(...)' -benchmem -benchtime=100x

go test ./benchmarks -run '^$' \
  -bench 'Benchmark(QEvalVector|QSessionEvalVector|QEvalJITScriptWarm)(...)' -benchmem -benchtime=100x
```

It intentionally separates qSQL from ordinary `q.eval` list/vector/math/adverb
workloads. `q.eval` now has hand-written Go baselines and warm/cold cache
measurements for vector arithmetic/reduce, compare/where, list slice/reduce,
adverb/running/moving aggregate, symbol/string/temporal, typed/null/cast, and
composition shapes. qSQL bind benchmarks expose `kernel_hit_pct` and
`fallbacks/op`; ordinary `q.eval` session benchmarks also report
`typed_kernel_*` counters where runtime kernel instrumentation is available.
The ordinary q benchmark rows also include the JIT script layer through
`BenchmarkQEvalJITScriptWarm`, so report output can join the same case across
session-warm, Go baseline, warm/cold result-cache, and JIT-script measurements.

Read the q performance baseline through these ratios:

| Signal | How to read it |
|---|---|
| Current Leia vs old Leia | `leia bench q-columnar` reports current worktree vs clean `HEAD`; compare `Current`, `HEAD`, and `HEAD delta` |
| Current Leia vs hand-written Go | compare `BenchmarkQSQLBind...` rows with `BenchmarkQSQLNativeGo...` rows for qSQL; compare `BenchmarkQSessionEvalVectorWarmExecution/...` with `BenchmarkQEvalVectorGoBaseline/...` for ordinary q compute |
| Warm vs cold | compare `BenchmarkQSQLBindRunSQLWarmCache...` with `BenchmarkQSQLBindRunSQLColdCache...` |
| Typed kernel hit/fallback rate | use `kernel_hit_pct`, `template_hit_pct`, `aligned_hit_pct`, and `fallbacks/op` in bind benchmark output; use `typed_kernel_hit_pct` and `typed_kernel_fallbacks/op` in ordinary q session rows |
| Shared data runtime coverage | use `data_runtime_*` and `linalg_vector_*`/`linalg_matrix_*` report fields when benchmark rows exercise shared data-runtime kernels; these are report-only by default |
| JIT q.session route health | use `q_session_planned_op_exit/op`, `q_session_shell_fallback/op`, `q_session_eval_errors/op`, and `q_session_backend_shapes` on `BenchmarkQEvalJITScriptWarm` rows |
| Allocation pressure | use `B/op` and `allocs/op`; q columnar hot paths should trend toward low per-row allocation |

Generate a machine-readable q performance report from the focused qSQL and
ordinary q benchmark rows:

```bash
go run ./cmd/leia bench q-report --benchtime=100x
```

The report writes `benchmarks/data/q_perf_report_latest.md` and
`benchmarks/data/q_perf_report_latest.json`, including Leia-vs-Go ratios,
warm/cold ratios, typed kernel hit/fallback counters, allocation metrics, and
fallback shape summary rows. Shared data-runtime counters such as
`data_runtime_attempts/op`, `data_runtime_hits/op`, `linalg_vector_hits/op`, and
`linalg_matrix_hits/op` are surfaced as report-only observability signals. For
the JIT-script layer it records the
`q_session_planned_op_exit/op`, `q_session_shell_fallback/op`,
`q_session_eval_errors/op`, and `q_session_backend_shapes` counters emitted by
`BenchmarkQEvalJITScriptWarm`.

The `q.eval Case Diagnostics` section is the first stop for runtime/JIT backend
triage. It joins each ordinary q case across `BenchmarkQSessionEvalVectorWarmExecution`,
`BenchmarkQEvalVectorGoBaseline`, result-cache warm, cold, `BenchmarkQEvalJITScriptWarm`,
and optional VM script rows. The `Pressure` column classifies the likely issue
as missing or untrusted Go baseline, cold-start pressure, typed fallback/error,
JIT backend slow route/error, allocation pressure, or ratio-only pressure. Gate
failures for Leia-vs-Go, typed hit/fallback, pipeline fallback, JIT errors, and
`allocs/op` include the same diagnostic note in both Markdown and JSON output.

Use `--check` to turn the same report into a performance gate:

```bash
go run ./cmd/leia bench q-report \
  --benchtime=100x \
  --check \
  --max-leia-go-ratio=5 \
  --min-typed-hit-pct=95 \
  --max-typed-fallbacks-op=0 \
  --max-pipeline-fallback-shapes=0 \
  --max-allocs-op=64 \
  --max-jit-typed-errors-op=0 \
  --max-jit-backend-slow-route-pct=0 \
  --min-q-session-planned-op-exit-op=0.9 \
  --min-runtime-direct-bridge-share-pct=95 \
  --max-runtime-allocs-per-direct-call=32 \
  --min-runtime-typed-primitive-benchmarks=1 \
  --min-runtime-jit-backend-benchmarks=1 \
  --min-runtime-array-bridge-benchmarks=1 \
  --min-runtime-bridge-benchmark-count=3 \
  --min-q-array-bridge-rows-op=1 \
  --max-q-array-bridge-avg-allocs-op=64 \
  --max-q-array-bridge-max-allocs-op=64 \
  --min-runtime-backend-route-benchmarks=1 \
  --min-runtime-backend-route-hits-op=1 \
  --max-runtime-backend-route-errors-op=0 \
  --min-q-eval-family-cases=1
```

For quick iteration on saved `go test -bench` output, skip rerunning benchmarks:

```bash
go run ./cmd/leia bench q-report \
  --from-output /tmp/qbench.txt \
  --check
```

### Runtime Bridge Efficiency Gate

The `Runtime Bridge Efficiency` report section is the regression gate for bulk
bridge and direct bridge work. It rolls up existing q benchmark metrics without
depending on a specific optimizer implementation:

| Field | Meaning | Regression signal |
|---|---|---|
| `direct calls/op` | typed primitive hits plus JIT direct runtime returns | should rise as typed runtime/JIT backend coverage improves |
| `slow bridge calls/op` | typed fallback/error plus JIT native/op exits/errors | should trend to zero on supported hot paths |
| `direct call share` | direct calls divided by direct plus slow calls | gate with `--min-runtime-direct-bridge-share-pct` |
| `allocs/direct call` | average `allocs/op` divided by direct calls/op | gate with `--max-runtime-allocs-per-direct-call` |

The default `--check` policy also requires benchmark rows for all three backend
layers: ordinary q typed primitive counters, JIT backend route counters, and
MethodJIT array bridge counters. This prevents a partial benchmark output from
looking healthy merely because one layer was not run. For intentionally partial
local checks, set the corresponding `--min-runtime-*-benchmarks=0` flag.
`BenchmarkQEvalJITScriptWarm` contributes JIT backend route evidence through
the q session counters: planned op-exit calls are the direct route, shell
fallbacks are the slow route, eval errors must remain zero, and backend shapes
show how many distinct q session lowerings were observed. The default
`leia bench q-suite` runs a stable JIT/VM script subset for release evidence;
pass `--jit-full` when auditing the full q.eval script coverage table.

The `Runtime Primitive Registry Routes` section is the lower-level backend
contract. It accepts either VM runtime primitive registry counters such as
`runtime_primitive_hits/op` and `runtime_primitive_errors/op`, or MethodJIT
Frame/Vector route counters such as `methodjit_frame_runtime_success/op` and
`methodjit_vector_runtime_success/op`, plus route split counters such as
`methodjit_frame_runtime_op_exit/op`, `methodjit_vector_runtime_native_exit/op`,
and `methodjit_vector_runtime_direct_helper/op`. The default `--check` policy
requires at least one backend-route benchmark row, at least one hit/op, and zero
errors/op. This catches partial benchmark output where typed-kernel or JIT
summary rows are present but the underlying runtime primitive registry or
Frame/Vector route statistics stopped being emitted.

The `Ordinary q Family Coverage` section is the breadth contract for non-qSQL
q work. The default `--check` policy requires actual benchmark output for
ordinary list/adverb rows, `TypeMatrix*` rows, and `Combo*` rows, with matching
`BenchmarkQSessionEvalVectorWarmExecution`, `BenchmarkQEvalVectorGoBaseline`,
and `BenchmarkQEvalJITScriptWarm` cases. This catches perf runs that have good
qSQL or single-shape ratios but accidentally omit the ordinary q breadth layer.

The `Runtime Array Bridge Summary` section is the focused MethodJIT q array
bridge gate. It consumes the `q_array_bridge_*` benchmark counters directly, so
bulk export regressions are visible without inferring them from the broader
runtime bridge aggregate:

| Field | Meaning | Regression signal |
|---|---|---|
| `bulk hits/op` | arrays converted through typed bulk export | should cover supported primitive and symbol carriers |
| `fallbacks/op` | arrays converted through row-wise fallback | gate with `--max-q-array-bridge-fallbacks-op` |
| `bulk hit pct` | bulk hits divided by bulk hits plus fallback/error routes | gate with `--min-q-array-bridge-bulk-hit-pct` |
| `rows/op` | array rows observed by the bridge benchmarks | sanity check that route counters cover meaningful data volume |
| `avg/max allocs/op` | allocation pressure of the bridge rows | gate with `--max-q-array-bridge-avg-allocs-op` and `--max-q-array-bridge-max-allocs-op` |

Use this stricter gate after bulk export, direct return, executable backend, or
typed kernel changes:

```bash
go run ./cmd/leia bench q-report \
  --from-output /tmp/qbench.txt \
  --check \
  --min-runtime-direct-bridge-share-pct=95 \
  --max-runtime-allocs-per-direct-call=32 \
  --min-q-array-bridge-bulk-hit-pct=95 \
  --max-q-array-bridge-fallbacks-op=0 \
  --min-q-array-bridge-rows-op=1 \
  --max-q-array-bridge-avg-allocs-op=64 \
  --max-q-array-bridge-max-allocs-op=64 \
  --max-jit-backend-slow-route-pct=0 \
  --max-jit-typed-errors-op=0
```

For CI or release gating, generate `/tmp/qbench.txt` from the q benchmark
families first, then feed it to the report:

```bash
{
  go test ./internal/stdlib/bind -run '^$' \
    -bench 'BenchmarkQSQL(Bind|DataRuntime|NativeGo)' \
    -benchmem -benchtime=100x
  go test ./benchmarks -run '^$' \
    -bench 'Benchmark(QEvalVector|QSessionEvalVector|QEvalJITScriptWarm)' \
    -benchmem -benchtime=100x
  go test ./internal/methodjit -run '^$' \
    -bench 'BenchmarkQEvalPipelineNativeExitCallpath/CodegenNativeExit' \
    -benchmem -benchtime=100x
  go test ./internal/methodjit -run '^$' \
    -bench 'BenchmarkQEvalPipelineArrayRuntimeBridge/Bulk' \
    -benchmem -benchtime=100x
  go test ./internal/methodjit -run '^$' \
    -bench 'BenchmarkQFrameVectorMethodJITRoute' \
    -benchmem -benchtime=100x
} | tee /tmp/qbench.txt

go run ./cmd/leia bench q-report \
  --from-output /tmp/qbench.txt \
  --check \
  --min-runtime-direct-bridge-share-pct=95 \
  --max-runtime-allocs-per-direct-call=32 \
  --min-q-array-bridge-bulk-hit-pct=95 \
  --max-q-array-bridge-fallbacks-op=0 \
  --min-q-session-planned-op-exit-op=0.9 \
  --markdown /tmp/q_perf_report.md \
  --json /tmp/q_perf_report.json
```

## q.eval Vector/List Compute

`benchmarks/q_eval_vector_bench_test.go` covers ordinary q expressions outside
qSQL. This suite is the main non-qSQL q performance matrix and now has **483
cases** plus required coverage-tag, matrix, and semantic-shape gates. It spans numeric
vector arithmetic, typed suffix/null/cast/promotion behavior, compare masks to
`where`, selectivity changes, `take`/`drop`/`cut`/`reverse`/`rotate`,
reductions, adverbs, running and moving aggregates, list/set/search verbs,
dictionaries and amend/upsert, symbols/enums, string transforms, temporal
values, table/keyed-table transforms, safe system commands, and loopback IPC.
The expanded ordinary-expression matrix adds parameterized non-SQL combinations
for `take`/`drop`/`reverse`/`rotate`, arithmetic map/reduce/scan, modulo
where/filter/project, symbol filters, temporal filters, and null/fill/cast
shapes. Every case is checked against a same-semantics Go checksum before it is
benchmarked.
Each shape has five synthetic benchmark rows:

| Row | Signal |
|---|---|
| `BenchmarkQEvalVectorResultCacheWarm/...` | repeated `q.eval` of one cacheable expression after the current result cache is warm |
| `BenchmarkQEvalVectorCold/...` | equivalent `q.eval` expressions with distinct cache keys, exercising parse/lower and execution cost |
| `BenchmarkQSessionEvalVectorWarmExecution/...` | repeated execution through `q.session.eval`, bypassing the global q.eval result cache |
| `BenchmarkQEvalVectorGoBaseline/...` | hand-written Go loop for the same checksum |
| `BenchmarkQEvalJITScriptWarm/...` | the same ordinary q case inside a JIT-compiled Leia loop, including q session route counters |

Use `-benchmem` to compare `B/op` and `allocs/op`; use result-cache warm vs
cold ratios to judge q.eval cache value; use `QSessionEvalVectorWarmExecution`
vs Go baseline to see how much ordinary vector/list execution still pays in
bridge, allocation, and dynamic dispatch overhead; use `QEvalJITScriptWarm`
rows to confirm the JIT script layer is still taking the planned q session
op-exit route instead of the shell fallback route.

`leia bench q-general` is the short wrapper for this ordinary
q compute suite. It keeps the benchmarks under `benchmarks/` while still
exercising q result-cache warm, session warm execution, cold parse/lower, and
hand-written Go baseline rows in one command.

The q coverage target is now gate-based, not just count-based. Ordinary
`q.eval` coverage must pass `TestQEvalVectorBenchmarkCoverageTags`. qSQL still
needs an equivalent tag gate across select/group/join/mutation/cache/runtime
stats before the full q performance suite can be called exhaustive. Keep new
benchmark cases tied to real q language shapes from the semantic tests rather
than to individual optimizer implementation details.
