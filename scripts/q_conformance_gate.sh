#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

RUN_BENCH="${RUN_BENCH:-smoke}"
Q_GATE_SCOPE="${Q_GATE_SCOPE:-core}"
JOBS="${JOBS:-6}"
TIMEOUT="${TIMEOUT:-120}"
TMPDIR="${TMPDIR:-$ROOT/.tmp/q-gate}"
JSON_OUT="false"

usage() {
  cat <<'USAGE'
Usage: scripts/q_conformance_gate.sh [--scope SCOPE] [--bench MODE] [--jobs N] [--timeout SECONDS] [--tmpdir DIR] [--json] [--help]

Checks q/qSQL manifest coverage, runtime packages, executable q cases, q
examples, and optional q benchmark smoke coverage.

Options:
  --scope SCOPE       q manifest scope passed to tests/manifest.py (default: core)
  --bench MODE        Benchmark mode: none, smoke, or all (default: smoke)
  --jobs N            Benchmark worker count (default: 6)
  --timeout SECONDS   Per-benchmark timeout (default: 120)
  --tmpdir DIR        Temporary output directory (default: .tmp/q-gate)
  --json              Print a machine-readable q gate report
  -h, --help          Show this help
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --scope)
      if [ "$#" -lt 2 ] || [ -z "$2" ]; then
        echo "error: --scope requires a value" >&2
        usage >&2
        exit 2
      fi
      Q_GATE_SCOPE="$2"
      shift 2
      ;;
    --bench)
      if [ "$#" -lt 2 ] || [ -z "$2" ]; then
        echo "error: --bench requires a value" >&2
        usage >&2
        exit 2
      fi
      RUN_BENCH="$2"
      shift 2
      ;;
    --jobs)
      if [ "$#" -lt 2 ] || [ -z "$2" ]; then
        echo "error: --jobs requires a value" >&2
        usage >&2
        exit 2
      fi
      JOBS="$2"
      shift 2
      ;;
    --timeout)
      if [ "$#" -lt 2 ] || [ -z "$2" ]; then
        echo "error: --timeout requires a value" >&2
        usage >&2
        exit 2
      fi
      TIMEOUT="$2"
      shift 2
      ;;
    --tmpdir)
      if [ "$#" -lt 2 ] || [ -z "$2" ]; then
        echo "error: --tmpdir requires a value" >&2
        usage >&2
        exit 2
      fi
      TMPDIR="$2"
      shift 2
      ;;
    --json)
      JSON_OUT="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

mkdir -p "$TMPDIR"
export GOCACHE="${GOCACHE:-$TMPDIR/go-cache}"
mkdir -p "$GOCACHE"

if ! [[ "$JOBS" =~ ^[1-9][0-9]*$ ]]; then
  echo "error: --jobs must be a positive integer: $JOBS" >&2
  exit 2
fi
if ! [[ "$TIMEOUT" =~ ^[1-9][0-9]*$ ]]; then
  echo "error: --timeout must be a positive integer: $TIMEOUT" >&2
  exit 2
fi

log_info() {
  if [ "$JSON_OUT" != "true" ]; then
    echo "$1"
  fi
}

json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '%s' "$value"
}

print_json_string_array() {
  local indent="$1"
  shift
  local values=("$@")
  printf '[\n'
  local i=0
  while [ "$i" -lt "${#values[@]}" ]; do
    printf '%s  "%s"' "$indent" "$(json_escape "${values[$i]}")"
    if [ "$i" -lt $((${#values[@]} - 1)) ]; then
      printf ','
    fi
    printf '\n'
    i=$((i + 1))
  done
  printf '%s]' "$indent"
}

run_logged() {
  local log_path="$1"
  shift
  if [ "$JSON_OUT" == "true" ]; then
    if ! "$@" >"$log_path" 2>&1; then
      cat "$log_path" >&2
      return 1
    fi
  else
    "$@"
  fi
}

read_q_paths() {
  local kind="$1"
  local path
  while IFS= read -r path; do
    [ -n "$path" ] || continue
    printf '%s\n' "$path"
  done < <(python3 tests/manifest.py list-q --scope "$Q_GATE_SCOPE" "$kind")
}

run_leia_paths() {
  local label="$1"
  shift
  local out="$TMPDIR/leia-q-gate-${label}.out"
  local total=0
  local path
  for path in "$@"; do
    total=$((total + 1))
    if ! "$bin" run "$path" >"$out" 2>&1; then
      echo "[q-gate] failed $label case: $path" >&2
      cat "$out" >&2
      exit 1
    fi
  done
  log_info "[q-gate] $label cases ok: $total"
}

q_benchmark_args() {
  local path
  local id
  for path in "$@"; do
    id="${path#benchmarks/}"
    id="${id%.leia}"
    printf '%s\n' --bench "$id"
  done
}

print_json_report() {
  local benchmark_json="$1"
  local benchmark_markdown="$2"
  printf '{\n'
  printf '  "schema_version": 1,\n'
  printf '  "status": "pass",\n'
  printf '  "scope": "%s",\n' "$(json_escape "$Q_GATE_SCOPE")"
  printf '  "bench_mode": "%s",\n' "$(json_escape "$RUN_BENCH")"
  printf '  "jobs": %s,\n' "$JOBS"
  printf '  "timeout_seconds": %s,\n' "$TIMEOUT"
  printf '  "language_case_count": %d,\n' "${#q_tests[@]}"
  printf '  "example_case_count": %d,\n' "${#q_examples[@]}"
  printf '  "benchmark_case_count": %d,\n' "${#q_benchmarks[@]}"
  printf '  "benchmark_json": "%s",\n' "$(json_escape "$benchmark_json")"
  printf '  "benchmark_markdown": "%s",\n' "$(json_escape "$benchmark_markdown")"
  printf '  "language_cases": '
  print_json_string_array "  " "${q_tests[@]}"
  printf ',\n'
  printf '  "example_cases": '
  print_json_string_array "  " "${q_examples[@]}"
  printf ',\n'
  printf '  "benchmark_cases": '
  print_json_string_array "  " "${q_benchmarks[@]}"
  printf '\n'
  printf '}\n'
}

log_info "[q-gate] manifest check"
run_logged "$TMPDIR/leia-q-gate-manifest.out" python3 tests/manifest.py check tests benchmarks
log_info "[q-gate] q gate scope: $Q_GATE_SCOPE"

log_info "[q-gate] go test q/data/bind"
run_logged "$TMPDIR/leia-q-gate-go-test.out" go test ./internal/stdlib/lib/q ./internal/stdlib/lib/data ./internal/stdlib/bind -count=1

bin="$(mktemp "$TMPDIR/leia-q-gate-bin-XXXXXX")"
cleanup() {
  rm -f "$bin" "$TMPDIR"/leia-q-gate-*.out
}
trap cleanup EXIT

log_info "[q-gate] build leia"
run_logged "$TMPDIR/leia-q-gate-build.out" go build -o "$bin" ./cmd/leia

q_tests=()
while IFS= read -r path; do q_tests+=("$path"); done < <(read_q_paths tests)
q_examples=()
while IFS= read -r path; do q_examples+=("$path"); done < <(read_q_paths examples)
q_benchmarks=()
while IFS= read -r path; do q_benchmarks+=("$path"); done < <(read_q_paths benchmarks)

log_info "[q-gate] language q/qsql cases"
run_leia_paths language "${q_tests[@]}"

log_info "[q-gate] q examples"
run_leia_paths examples "${q_examples[@]}"

bench_args=()
if [ "${#q_benchmarks[@]}" -gt 0 ]; then
  while IFS= read -r arg; do bench_args+=("$arg"); done < <(q_benchmark_args "${q_benchmarks[@]}")
fi

case "$RUN_BENCH" in
  none)
    log_info "[q-gate] benchmark smoke skipped"
    ;;
  smoke)
    if [ "${#bench_args[@]}" -eq 0 ]; then
      log_info "[q-gate] benchmark smoke skipped (no q benchmarks in scope)"
    else
      log_info "[q-gate] q benchmark smoke cases: ${#q_benchmarks[@]}"
      if [ "$JSON_OUT" == "true" ]; then
        run_logged "$TMPDIR/leia-q-gate-benchmark.out" python3 benchmarks/timing_compare.py \
          --runs=1 --warmup=0 --timeout="$TIMEOUT" \
          --min-sample-seconds=0.005 --max-repeat=1 \
          --no-luajit --jobs="$JOBS" \
          "${bench_args[@]}" \
          --json "$TMPDIR/leia_q_gate_smoke.json" \
          --markdown "$TMPDIR/leia_q_gate_smoke.md"
      else
        progress_args=(--progress)
        run_logged "$TMPDIR/leia-q-gate-benchmark.out" python3 benchmarks/timing_compare.py \
          --runs=1 --warmup=0 --timeout="$TIMEOUT" \
          --min-sample-seconds=0.005 --max-repeat=1 \
          --no-luajit --jobs="$JOBS" \
          "${bench_args[@]}" \
          --json "$TMPDIR/leia_q_gate_smoke.json" \
          --markdown "$TMPDIR/leia_q_gate_smoke.md" \
          "${progress_args[@]}"
      fi
    fi
    ;;
  all)
    if [ "${#bench_args[@]}" -eq 0 ]; then
      log_info "[q-gate] benchmark suite skipped (no q benchmarks in scope)"
    else
      log_info "[q-gate] all q benchmark cases: ${#q_benchmarks[@]}"
      if [ "$JSON_OUT" == "true" ]; then
        run_logged "$TMPDIR/leia-q-gate-benchmark.out" python3 benchmarks/timing_compare.py \
          --runs=3 --warmup=1 --timeout="$TIMEOUT" \
          --min-sample-seconds=0.01 --max-repeat=3 \
          --no-luajit --jobs="$JOBS" \
          "${bench_args[@]}" \
          --json "$TMPDIR/leia_q_gate_all_smoke.json" \
          --markdown "$TMPDIR/leia_q_gate_all_smoke.md"
      else
        progress_args=(--progress)
        run_logged "$TMPDIR/leia-q-gate-benchmark.out" python3 benchmarks/timing_compare.py \
          --runs=3 --warmup=1 --timeout="$TIMEOUT" \
          --min-sample-seconds=0.01 --max-repeat=3 \
          --no-luajit --jobs="$JOBS" \
          "${bench_args[@]}" \
          --json "$TMPDIR/leia_q_gate_all_smoke.json" \
          --markdown "$TMPDIR/leia_q_gate_all_smoke.md" \
          "${progress_args[@]}"
      fi
    fi
    ;;
  *)
    echo "RUN_BENCH must be one of: none, smoke, all" >&2
    exit 2
    ;;
esac

benchmark_json=""
benchmark_markdown=""
case "$RUN_BENCH" in
  smoke)
    if [ "${#q_benchmarks[@]}" -gt 0 ]; then
      benchmark_json="$TMPDIR/leia_q_gate_smoke.json"
      benchmark_markdown="$TMPDIR/leia_q_gate_smoke.md"
    fi
    ;;
  all)
    if [ "${#q_benchmarks[@]}" -gt 0 ]; then
      benchmark_json="$TMPDIR/leia_q_gate_all_smoke.json"
      benchmark_markdown="$TMPDIR/leia_q_gate_all_smoke.md"
    fi
    ;;
esac

if [ "$JSON_OUT" == "true" ]; then
  print_json_report "$benchmark_json" "$benchmark_markdown"
else
  echo "[q-gate] ok"
fi
