#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT_DIR="${LEIA_Q_BASELINE_OUT:-${TMPDIR:-/tmp}/leia-q-baseline}"
RUNS="${LEIA_Q_BASELINE_RUNS:-3}"
WARMUP="${LEIA_Q_BASELINE_WARMUP:-1}"
GO_BENCHTIME="${LEIA_Q_BASELINE_GO_BENCHTIME:-100x}"
SMOKE=0
JIT_FULL=0

for arg in "$@"; do
  case "$arg" in
    --smoke)
      SMOKE=1
      ;;
    --jit-full)
      JIT_FULL=1
      ;;
    -h|--help)
      cat <<'USAGE'
Usage: benchmarks/q_baseline_suite.sh [--smoke] [--jit-full]

Writes a q performance baseline directory with:
  q_luajit_compare.json/.md  Leia current vs HEAD vs LuaJIT
  q_go_bench.txt             q/qSQL Go benchmark output
  q_perf_report.json/.md     parsed Go ratios, allocs, cache and fallback stats

Environment:
  LEIA_Q_BASELINE_OUT=/tmp/leia-q-baseline
  LEIA_Q_BASELINE_RUNS=3
  LEIA_Q_BASELINE_WARMUP=1
  LEIA_Q_BASELINE_GO_BENCHTIME=100x
  LEIA_Q_BASELINE_JOBS=4
USAGE
      exit 0
      ;;
    *)
      echo "q_baseline_suite.sh: unknown argument $arg" >&2
      exit 2
      ;;
  esac
done

if [[ "$SMOKE" == "1" ]]; then
  RUNS=1
  WARMUP=0
  GO_BENCHTIME="${LEIA_Q_BASELINE_SMOKE_GO_BENCHTIME:-1x}"
fi
mkdir -p "$OUT_DIR"

TIMING_JSON="$OUT_DIR/q_luajit_compare.json"
TIMING_MD="$OUT_DIR/q_luajit_compare.md"
GO_OUTPUT="$OUT_DIR/q_go_bench.txt"
REPORT_JSON="$OUT_DIR/q_perf_report.json"
REPORT_MD="$OUT_DIR/q_perf_report.md"
Q_SUITE_ARGS=()
if [[ "$JIT_FULL" == "1" ]]; then
  Q_SUITE_ARGS+=(--jit-full)
fi

go run ./cmd/leia bench compare \
  --runs="$RUNS" \
  --warmup="$WARMUP" \
  --timeout=90 \
  --time-source=auto \
  --min-sample-seconds=0.100 \
  --timer-resolution=0.001 \
  --max-repeat=128 \
  --min-wall-repeat=4 \
  --scale-profile=hot \
  --sort=luajit-gap \
  --jobs="${LEIA_Q_BASELINE_JOBS:-4}" \
  --head-ref=HEAD \
  --json "$TIMING_JSON" \
  --markdown "$TIMING_MD" \
  --bench data/q_operator_pipeline \
  --bench data/q_query_rollup \
  $(if [[ "$SMOKE" != "1" ]]; then printf '%s\n' \
    --bench data/soa_mask_compare \
    --bench data/soa_filter_gather \
    --bench data/soa_masked_aggregate \
    --bench data/soa_reducers \
    --bench data/soa_scan \
    --bench data/soa_select; fi) \
  --bench numeric/math_intensive \
  --bench numeric/spectral_norm \
  $(if [[ "$SMOKE" != "1" ]]; then printf '%s\n' \
    --bench table/table_array_access \
    --bench string/string_bench \
    --bench control/sieve \
    --bench calls/closure_accumulator; fi)

if [[ "${#Q_SUITE_ARGS[@]}" -gt 0 ]]; then
  LEIA_SKIP_TIMING_COMPARE=1 \
  LEIA_GO_BENCHTIME="$GO_BENCHTIME" \
    go run ./cmd/leia bench q-suite "${Q_SUITE_ARGS[@]}" | tee "$GO_OUTPUT"
else
  LEIA_SKIP_TIMING_COMPARE=1 \
  LEIA_GO_BENCHTIME="$GO_BENCHTIME" \
    go run ./cmd/leia bench q-suite | tee "$GO_OUTPUT"
fi

go run ./cmd/leia bench q-report \
  --from-output "$GO_OUTPUT" \
  --timing-json "$TIMING_JSON" \
  --json "$REPORT_JSON" \
  --markdown "$REPORT_MD"

echo "q baseline timing: $TIMING_JSON $TIMING_MD"
echo "q baseline report: $REPORT_JSON $REPORT_MD"
