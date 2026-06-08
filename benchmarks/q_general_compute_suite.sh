#!/usr/bin/env bash
set -euo pipefail

BENCHTIME="${BENCHTIME:-100x}"
COUNT="${COUNT:-1}"

go test ./benchmarks -run '^TestQEvalVectorBenchmarkExpressions$' \
  -bench 'Benchmark(QEvalVector(ResultCacheWarm|Cold|GoBaseline)|QSessionEvalVectorWarmExecution)' \
  -benchmem \
  -benchtime="${BENCHTIME}" \
  -count="${COUNT}"
