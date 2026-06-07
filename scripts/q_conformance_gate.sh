#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

RUN_BENCH="${RUN_BENCH:-smoke}"
Q_GATE_SCOPE="${Q_GATE_SCOPE:-core}"
JOBS="${JOBS:-6}"
TIMEOUT="${TIMEOUT:-120}"
TMPDIR="${TMPDIR:-$ROOT/.tmp/q-gate}"
mkdir -p "$TMPDIR"
export GOCACHE="${GOCACHE:-$TMPDIR/go-cache}"
mkdir -p "$GOCACHE"

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
  echo "[q-gate] $label cases ok: $total"
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

echo "[q-gate] manifest check"
python3 tests/manifest.py check tests benchmarks
echo "[q-gate] q gate scope: $Q_GATE_SCOPE"

echo "[q-gate] go test q/data/bind"
go test ./internal/stdlib/lib/q ./internal/stdlib/lib/data ./internal/stdlib/bind -count=1

bin="$(mktemp "$TMPDIR/leia-q-gate-bin-XXXXXX")"
cleanup() {
  rm -f "$bin" "$TMPDIR"/leia-q-gate-*.out
}
trap cleanup EXIT

echo "[q-gate] build leia"
go build -o "$bin" ./cmd/leia

q_tests=()
while IFS= read -r path; do q_tests+=("$path"); done < <(read_q_paths tests)
q_examples=()
while IFS= read -r path; do q_examples+=("$path"); done < <(read_q_paths examples)
q_benchmarks=()
while IFS= read -r path; do q_benchmarks+=("$path"); done < <(read_q_paths benchmarks)

echo "[q-gate] language q/qsql cases"
run_leia_paths language "${q_tests[@]}"

echo "[q-gate] q examples"
run_leia_paths examples "${q_examples[@]}"

bench_args=()
if [ "${#q_benchmarks[@]}" -gt 0 ]; then
  while IFS= read -r arg; do bench_args+=("$arg"); done < <(q_benchmark_args "${q_benchmarks[@]}")
fi

case "$RUN_BENCH" in
  none)
    echo "[q-gate] benchmark smoke skipped"
    ;;
  smoke)
    if [ "${#bench_args[@]}" -eq 0 ]; then
      echo "[q-gate] benchmark smoke skipped (no q benchmarks in scope)"
    else
      echo "[q-gate] q benchmark smoke cases: ${#q_benchmarks[@]}"
      python3 benchmarks/timing_compare.py \
        --runs=1 --warmup=0 --timeout="$TIMEOUT" \
        --min-sample-seconds=0.005 --max-repeat=1 \
        --no-luajit --jobs="$JOBS" \
        "${bench_args[@]}" \
        --json "$TMPDIR/leia_q_gate_smoke.json" \
        --markdown "$TMPDIR/leia_q_gate_smoke.md" \
        --progress
    fi
    ;;
  all)
    if [ "${#bench_args[@]}" -eq 0 ]; then
      echo "[q-gate] benchmark suite skipped (no q benchmarks in scope)"
    else
      echo "[q-gate] all q benchmark cases: ${#q_benchmarks[@]}"
      python3 benchmarks/timing_compare.py \
        --runs=3 --warmup=1 --timeout="$TIMEOUT" \
        --min-sample-seconds=0.01 --max-repeat=3 \
        --no-luajit --jobs="$JOBS" \
        "${bench_args[@]}" \
        --json "$TMPDIR/leia_q_gate_all_smoke.json" \
        --markdown "$TMPDIR/leia_q_gate_all_smoke.md" \
        --progress
    fi
    ;;
  *)
    echo "RUN_BENCH must be one of: none, smoke, all" >&2
    exit 2
    ;;
esac

echo "[q-gate] ok"
