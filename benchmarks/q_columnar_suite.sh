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
  --bench=data/q_vector_adverb_dict \
  --bench=data/q_operator_pipeline \
  --bench=data/q_projection_limit_cache \
  --bench=data/q_query_rollup \
  --bench=data/q_xbar_asof_rollup \
  --bench=data/q_native_join_update_delete \
  --bench=data/qsql_join_variants \
  --bench=data/qsql_join_mutation \
  --bench=data/frame_qsql_distinct_take \
  --bench=data/frame_qsql_rollup \
  --bench=data/soa_affine_many \
  --bench=data/soa_clamp \
  --bench=data/soa_dot \
  --bench=data/soa_filter_gather \
  --bench=data/soa_mask_compare \
  --bench=data/soa_masked_aggregate \
  --bench=data/soa_reducers \
  --bench=data/soa_scan \
  --bench=data/soa_scatter \
  --bench=data/soa_select \
  --bench=data/q_integrated_market_replay_project \
  --bench=data/q_kdb_market_tape_complex_project \
  --bench=data/q_market_microstructure_project \
  --bench=data/q_temporal_window_analytics_project \
  --bench=data/q_ticks_bars_asof_project \
  "$@"
