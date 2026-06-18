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
failure_kinds=()
failure_messages=()
failure_values=()
failure_printed="false"
q_tests=()
q_examples=()
q_benchmarks=()
benchmark_json=""
benchmark_markdown=""

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

record_failure() {
  local kind="$1"
  local message="$2"
  local value="${3:-}"
  failure_kinds+=("$kind")
  failure_messages+=("$message")
  failure_values+=("$value")
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

print_json_failure_details() {
  local indent="$1"
  printf '[\n'
  local i=0
  while [ "$i" -lt "${#failure_messages[@]}" ]; do
    printf '%s  {"kind": "%s", "message": "%s", "value": "%s"}' "$indent" "$(json_escape "${failure_kinds[$i]}")" "$(json_escape "${failure_messages[$i]}")" "$(json_escape "${failure_values[$i]}")"
    if [ "$i" -lt $((${#failure_messages[@]} - 1)) ]; then
      printf ','
    fi
    printf '\n'
    i=$((i + 1))
  done
  printf '%s]' "$indent"
}

print_json_array_var() {
  local indent="$1"
  shift
  if [ "$#" -eq 0 ]; then
    print_json_string_array "$indent"
  else
    print_json_string_array "$indent" "$@"
  fi
}

fail() {
  local kind="$1"
  local message="$2"
  local code="${3:-1}"
  local value="${4:-}"
  record_failure "$kind" "$message" "$value"
  if [ "$JSON_OUT" == "true" ]; then
    failure_printed="true"
    print_json_report "fail"
  else
    echo "error: $message" >&2
  fi
  exit "$code"
}

on_error() {
  local code="$1"
  if [ "$JSON_OUT" == "true" ] && [ "$failure_printed" != "true" ]; then
    record_failure "command_failed" "command failed: ${BASH_COMMAND}" "${BASH_COMMAND}"
    failure_printed="true"
    print_json_report "fail"
  fi
  exit "$code"
}

run_logged() {
  local log_path="$1"
  shift
  if [ "$JSON_OUT" == "true" ]; then
    if ! "$@" >"$log_path" 2>&1; then
      record_failure "command_failed" "command failed: $*" "$log_path"
      failure_printed="true"
      print_json_report "fail"
      exit 1
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
      if [ "$JSON_OUT" == "true" ]; then
        record_failure "q_case_failed" "failed $label case: $path" "$path"
        failure_printed="true"
        print_json_report "fail"
      else
        echo "[q-gate] failed $label case: $path" >&2
        cat "$out" >&2
      fi
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
  local status="${1:-pass}"
  local failure_kind_count="${#failure_kinds[@]}"
  local language_case_count="${#q_tests[@]}"
  local example_case_count="${#q_examples[@]}"
  local benchmark_case_count="${#q_benchmarks[@]}"
  printf '{\n'
  printf '  "schema_version": 1,\n'
  printf '  "status": "%s",\n' "$(json_escape "$status")"
  printf '  "scope": "%s",\n' "$(json_escape "$Q_GATE_SCOPE")"
  printf '  "bench_mode": "%s",\n' "$(json_escape "$RUN_BENCH")"
  printf '  "jobs": %s,\n' "$JOBS"
  printf '  "timeout_seconds": %s,\n' "$TIMEOUT"
  printf '  "failure_kind_count": %d,\n' "$failure_kind_count"
  printf '  "failure_count": %d,\n' "${#failure_messages[@]}"
  printf '  "language_case_count": %d,\n' "$language_case_count"
  printf '  "example_case_count": %d,\n' "$example_case_count"
  printf '  "benchmark_case_count": %d,\n' "$benchmark_case_count"
  printf '  "benchmark_json": "%s",\n' "$(json_escape "$benchmark_json")"
  printf '  "benchmark_markdown": "%s",\n' "$(json_escape "$benchmark_markdown")"
  printf '  "failure_kinds": '
  if [ "$failure_kind_count" -eq 0 ]; then
    print_json_array_var "  "
  else
    print_json_array_var "  " "${failure_kinds[@]}"
  fi
  printf ',\n'
  printf '  "failure_details": '
  print_json_failure_details "  "
  printf ',\n'
  printf '  "language_cases": '
  if [ "$language_case_count" -eq 0 ]; then
    print_json_array_var "  "
  else
    print_json_array_var "  " "${q_tests[@]}"
  fi
  printf ',\n'
  printf '  "example_cases": '
  if [ "$example_case_count" -eq 0 ]; then
    print_json_array_var "  "
  else
    print_json_array_var "  " "${q_examples[@]}"
  fi
  printf ',\n'
  printf '  "benchmark_cases": '
  if [ "$benchmark_case_count" -eq 0 ]; then
    print_json_array_var "  "
  else
    print_json_array_var "  " "${q_benchmarks[@]}"
  fi
  printf '\n'
  printf '}\n'
}

trap 'on_error "$?"' ERR

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

while IFS= read -r path; do q_tests+=("$path"); done < <(read_q_paths tests)
while IFS= read -r path; do q_examples+=("$path"); done < <(read_q_paths examples)
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
    fail "invalid_bench_mode" "RUN_BENCH must be one of: none, smoke, all" 2 "$RUN_BENCH"
    ;;
esac

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
  print_json_report "pass"
else
  echo "[q-gate] ok"
fi
