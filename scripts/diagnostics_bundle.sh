#!/usr/bin/env bash
# Collect a self-contained local diagnostics bundle.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
OUT_ARG="diagnostics/$timestamp"
RUN_GO_TESTS=1
RUN_BENCHMARKS=1

usage() {
    cat <<'EOF'
Usage: scripts/diagnostics_bundle.sh [--output DIR] [DIR] [--skip-go-tests] [--skip-benchmarks] [--help]

Collects git revision/status, Go environment summary, quick Go test results,
and quick benchmark guard summaries when the local tools are available.

The default output directory is diagnostics/<timestamp>. Output inside the
repository must be git-ignored; use /tmp/... for an untracked external bundle.
EOF
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        -o|--output)
            if [ "$#" -lt 2 ]; then
                echo "missing value for $1" >&2
                exit 2
            fi
            OUT_ARG="$2"
            shift 2
            ;;
        --skip-go-tests)
            RUN_GO_TESTS=0
            shift
            ;;
        --skip-benchmarks)
            RUN_BENCHMARKS=0
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        -*)
            echo "unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
        *)
            OUT_ARG="$1"
            shift
            ;;
    esac
done

case "$OUT_ARG" in
    /*) OUT_DIR="$OUT_ARG" ;;
    *) OUT_DIR="$ROOT/$OUT_ARG" ;;
esac

inside_repo=0
case "$OUT_DIR" in
    "$ROOT"|"$ROOT"/*) inside_repo=1 ;;
esac

if [ "$inside_repo" -eq 1 ]; then
    rel_out="${OUT_DIR#$ROOT/}"
    if [ "$rel_out" = "$OUT_DIR" ]; then
        rel_out="."
    fi
    if ! git check-ignore -q "$rel_out"; then
        echo "refusing to write unignored repository path: $rel_out" >&2
        echo "choose diagnostics/<name>, another ignored path, or an external /tmp path" >&2
        exit 2
    fi
fi

mkdir -p "$OUT_DIR"
SUMMARY="$OUT_DIR/summary.md"
MANIFEST="$OUT_DIR/manifest.txt"
: >"$SUMMARY"
: >"$MANIFEST"

record_manifest() {
    printf '%s\n' "$1" >>"$MANIFEST"
}

have_cmd() {
    command -v "$1" >/dev/null 2>&1
}

run_logged() {
    local name="$1"
    local logfile="$2"
    local cmd="$3"
    local status_file="$4"

    {
        printf '# %s\n\n' "$name"
        printf 'Command: `%s`\n\n' "$cmd"
    } >>"$SUMMARY"

    printf '=== RUN %s ===\n' "$name"
    if bash -lc "$cmd" >"$logfile" 2>&1; then
        printf '0\n' >"$status_file"
        printf 'Result: PASS\n\n' >>"$SUMMARY"
        printf '=== PASS %s ===\n' "$name"
        return 0
    else
        status=$?
        printf '%s\n' "$status" >"$status_file"
        printf 'Result: FAIL (exit %s)\n\n' "$status" >>"$SUMMARY"
        printf '=== FAIL %s (exit %s) ===\n' "$name" "$status"
        return "$status"
    fi
}

write_skip() {
    local name="$1"
    local reason="$2"
    local file="$3"

    printf 'SKIPPED: %s\n' "$reason" >"$file"
    {
        printf '# %s\n\n' "$name"
        printf 'Result: SKIPPED - %s\n\n' "$reason"
    } >>"$SUMMARY"
}

failures=0

{
    printf '# GScript Diagnostics Bundle\n\n'
    printf '%s\n' "- Created: \`$(date -u +%Y-%m-%dT%H:%M:%SZ)\`"
    printf '%s\n' "- Output: \`$OUT_DIR\`"
    printf '%s\n\n' "- Root: \`$ROOT\`"
} >>"$SUMMARY"

record_manifest "summary.md"
record_manifest "manifest.txt"

if have_cmd git; then
    {
        printf 'HEAD=%s\n' "$(git rev-parse HEAD)"
        printf 'BRANCH=%s\n' "$(git branch --show-current 2>/dev/null || true)"
        printf 'DESCRIBE=%s\n' "$(git describe --always --dirty --tags 2>/dev/null || git rev-parse --short HEAD)"
    } >"$OUT_DIR/git_revision.txt"
    git status --short --branch >"$OUT_DIR/git_status.txt" 2>&1
    git diff --stat >"$OUT_DIR/git_diff_stat.txt" 2>&1
    record_manifest "git_revision.txt"
    record_manifest "git_status.txt"
    record_manifest "git_diff_stat.txt"
else
    write_skip "Git Revision" "missing git" "$OUT_DIR/git_revision.txt"
    record_manifest "git_revision.txt"
fi

if have_cmd go; then
    go version >"$OUT_DIR/go_version.txt" 2>&1
    go env GOVERSION GOOS GOARCH GOHOSTOS GOHOSTARCH GOMOD GOWORK GOPATH GOCACHE CGO_ENABLED CC >"$OUT_DIR/go_env_summary.txt" 2>&1
    record_manifest "go_version.txt"
    record_manifest "go_env_summary.txt"
else
    write_skip "Go Environment" "missing go" "$OUT_DIR/go_env_summary.txt"
    record_manifest "go_env_summary.txt"
fi

if [ "$RUN_GO_TESTS" -eq 1 ]; then
    if have_cmd go; then
        if ! run_logged "Quick Go Packages" "$OUT_DIR/go_test_quick_packages.log" \
            "go test . ./cmd/gscript ./internal/lexer ./internal/parser ./internal/runtime ./internal/vm -count=1" \
            "$OUT_DIR/go_test_quick_packages.status"; then
            failures=$((failures + 1))
        fi
        if ! run_logged "Quick Go Matrix" "$OUT_DIR/go_test_quick_matrix.log" \
            "go test ./tests -run 'TestFeatureMatrix|TestIntegration|TestStdlibContract' -count=1" \
            "$OUT_DIR/go_test_quick_matrix.status"; then
            failures=$((failures + 1))
        fi
        record_manifest "go_test_quick_packages.log"
        record_manifest "go_test_quick_packages.status"
        record_manifest "go_test_quick_matrix.log"
        record_manifest "go_test_quick_matrix.status"
    else
        write_skip "Quick Go Tests" "missing go" "$OUT_DIR/go_test_quick.skipped"
        record_manifest "go_test_quick.skipped"
    fi
else
    write_skip "Quick Go Tests" "disabled by --skip-go-tests" "$OUT_DIR/go_test_quick.skipped"
    record_manifest "go_test_quick.skipped"
fi

if [ "$RUN_BENCHMARKS" -eq 1 ]; then
    if have_cmd python3 && [ -f benchmarks/timing_compare.py ]; then
        if ! run_logged "Benchmark Quick Summary" "$OUT_DIR/benchmark_quick.log" \
            "python3 benchmarks/timing_compare.py --runs=1 --warmup=0 --timeout=30 --min-sample-seconds=0.001 --max-repeat=1 --no-luajit --bench=control/sieve --json '$OUT_DIR/benchmark_quick.json' --markdown '$OUT_DIR/benchmark_quick.md'" \
            "$OUT_DIR/benchmark_quick.status"; then
            failures=$((failures + 1))
        fi
        record_manifest "benchmark_quick.log"
        record_manifest "benchmark_quick.status"
        record_manifest "benchmark_quick.json"
        record_manifest "benchmark_quick.md"
    else
        write_skip "Benchmark Quick Summary" "missing python3 or benchmarks/timing_compare.py" "$OUT_DIR/benchmark_quick.skipped"
        record_manifest "benchmark_quick.skipped"
    fi

    if have_cmd python3 && [ -f benchmarks/strict_guard.py ]; then
        if ! run_logged "Strict Guard Summary" "$OUT_DIR/strict_guard.log" \
            "python3 benchmarks/strict_guard.py --runs=1 --warmup=0 --timeout=30 --min-sample-seconds=0.001 --max-repeat=1 --no-luajit --mode=vm --mode=default --mode=no_filter --bench=control/sieve --json '$OUT_DIR/strict_guard.json' --markdown '$OUT_DIR/strict_guard.md'" \
            "$OUT_DIR/strict_guard.status"; then
            failures=$((failures + 1))
        fi
        record_manifest "strict_guard.log"
        record_manifest "strict_guard.status"
        record_manifest "strict_guard.json"
        record_manifest "strict_guard.md"
    else
        write_skip "Strict Guard Summary" "missing python3 or benchmarks/strict_guard.py" "$OUT_DIR/strict_guard.skipped"
        record_manifest "strict_guard.skipped"
    fi
else
    write_skip "Benchmark Summaries" "disabled by --skip-benchmarks" "$OUT_DIR/benchmarks.skipped"
    record_manifest "benchmarks.skipped"
fi

{
    printf '# Files\n\n'
    sed 's/^/- `/' "$MANIFEST" | sed 's/$/`/'
    printf '\n'
} >>"$SUMMARY"

printf 'Diagnostics bundle written to %s\n' "$OUT_DIR"
if [ "$failures" -gt 0 ]; then
    printf '%s check(s) failed; see %s\n' "$failures" "$SUMMARY" >&2
    exit 1
fi
