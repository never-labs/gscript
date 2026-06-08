#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

exec python3 benchmarks/timing_compare.py \
  --no-luajit \
  --bench=data/q_columnar_eval_primitives \
  --bench=data/q_columnar_qsql_filter_project \
  --bench=data/q_columnar_qsql_group_xbar \
  --bench=data/q_columnar_qsql_asof_join \
  "$@"
