#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

: "${LEIA_GO_BENCHTIME:=100x}"
: "${LEIA_GO_BENCHCOUNT:=1}"

if [[ "${LEIA_SKIP_TIMING_COMPARE:-0}" != "1" ]]; then
  bash benchmarks/q_columnar_suite.sh "$@"
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
