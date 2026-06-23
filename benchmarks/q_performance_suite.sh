#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

: "${LEIA_GO_BENCHTIME:=100x}"
: "${LEIA_GO_BENCHCOUNT:=1}"

SMOKE=0
ARGS=()
for arg in "$@"; do
  case "$arg" in
    --smoke)
      SMOKE=1
      ;;
    *)
      ARGS+=("$arg")
      ;;
  esac
done

if [[ "$SMOKE" == "1" ]]; then
  SMOKE_BENCHTIME="${LEIA_Q_SUITE_SMOKE_BENCHTIME:-1x}"
  SMOKE_BENCHCOUNT="${LEIA_Q_SUITE_SMOKE_BENCHCOUNT:-1}"
  go test ./internal/stdlib/bind -run '^$' \
    -bench 'BenchmarkQSQLBindMatrixWarm/ExecVectorWhere$' \
    -benchmem \
    -benchtime="${SMOKE_BENCHTIME}" \
    -count="${SMOKE_BENCHCOUNT}"
  go test ./benchmarks -run '^TestQEvalVectorBenchmarkExpressions$' \
    -bench 'BenchmarkQEvalVectorResultCacheWarm/VectorAffineSumSmall$' \
    -benchmem \
    -benchtime="${SMOKE_BENCHTIME}" \
    -count="${SMOKE_BENCHCOUNT}"
  exit 0
fi

if [[ "${LEIA_SKIP_TIMING_COMPARE:-0}" != "1" ]]; then
  bash benchmarks/q_columnar_suite.sh "${ARGS[@]}"
fi

go test ./internal/stdlib/bind -run '^$' \
  -bench 'BenchmarkQSQL(Bind|DataRuntime|NativeGo)' \
  -benchmem \
  -benchtime="${LEIA_GO_BENCHTIME}" \
  -count="${LEIA_GO_BENCHCOUNT}"

go test ./benchmarks -run '^TestQEvalVectorBenchmarkExpressions$' \
  -bench 'Benchmark(QEvalVector(ResultCacheWarm|Cold|GoBaseline)|QSessionEvalVectorWarmExecution)' \
  -benchmem \
  -benchtime="${LEIA_GO_BENCHTIME}" \
  -count="${LEIA_GO_BENCHCOUNT}"

# Real-data annex: env-injected dense columns where lazy carriers and
# closed-form kernels cannot fire. Feeds the realdata_go_ratio family in
# q_perf_report.py (reported and ratcheted separately from the synthetic
# suite). The Test* run checksum-verifies q results against the Go baselines.
go test ./benchmarks -run '^TestQEvalRealDataMatchesGoBaseline$' \
  -bench 'BenchmarkQEvalRealData(Warm|GoBaseline)' \
  -benchmem \
  -benchtime="${LEIA_GO_BENCHTIME}" \
  -count="${LEIA_GO_BENCHCOUNT}"

# JIT/VM script-binding warm benches feeding the jit_script/vm_script Go-ratio
# families in q_perf_report.py. Tolerant while these benches land: `-bench`
# with zero matches still exits 0 and simply contributes no rows.
go test ./benchmarks -run '^$' \
  -bench 'BenchmarkQEval(JIT|VM)ScriptWarm' \
  -benchmem \
  -benchtime="${LEIA_GO_BENCHTIME}" \
  -count="${LEIA_GO_BENCHCOUNT}"

go test ./internal/methodjit -run '^$' \
  -bench 'BenchmarkQEvalPipelineNativeExitCallpath/CodegenNativeExit' \
  -benchmem \
  -benchtime="${LEIA_GO_BENCHTIME}" \
  -count="${LEIA_GO_BENCHCOUNT}"

go test ./internal/methodjit -run '^$' \
  -bench 'BenchmarkQEvalPipelineArrayRuntimeBridge/Bulk' \
  -benchmem \
  -benchtime="${LEIA_GO_BENCHTIME}" \
  -count="${LEIA_GO_BENCHCOUNT}"

go test ./internal/methodjit -run '^$' \
  -bench 'BenchmarkQFrameVectorMethodJITRoute' \
  -benchmem \
  -benchtime="${LEIA_GO_BENCHTIME}" \
  -count="${LEIA_GO_BENCHCOUNT}"
