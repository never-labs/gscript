#!/usr/bin/env bash
set -euo pipefail

BENCHTIME="${BENCHTIME:-100x}"
COUNT="${COUNT:-1}"
RUN_REGEX="${RUN_REGEX:-^$}"
BENCH_REGEX="${BENCH_REGEX:-Benchmark(QEvalVector(ResultCacheWarm|Cold|GoBaseline)|QSessionEvalVectorWarmExecution)}"

if [[ "${VERIFY_EXPRESSIONS:-0}" == "1" ]]; then
  RUN_REGEX="^TestQEvalVectorBenchmarkExpressions$"
fi

go test ./benchmarks -run "${RUN_REGEX}" \
  -bench "${BENCH_REGEX}" \
  -benchmem \
  -benchtime="${BENCHTIME}" \
  -count="${COUNT}"
