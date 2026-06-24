#!/bin/bash
# scripts/diag.sh — Production-parity Tier 2 diagnostic dump.
#
# For each benchmark under benchmarks/<domain>/, runs the
# full production Tier 2 compile pipeline on every Tier-2-promotable proto
# and writes (nested under the originating subdirectory):
#
#   diag/<domain>/<bench>/<proto>.bin        — raw ARM64 code bytes
#   diag/<domain>/<bench>/<proto>.ir.txt     — post-pipeline IR + regalloc + intrinsics
#   diag/<domain>/<bench>/<proto>.asm.txt    — golang.org/x/arch ARM64 disasm
#   diag/<domain>/<bench>/stats.json         — per-proto insn count + histogram
#
# Plus an aggregate diag/summary.md with top drifters vs reference.json.
#
# The Go side (internal/methodjit/diag_dump_test.go) calls
# TieringManager.CompileForDiagnostics, which shares compileTier2Pipeline
# with the production path — enforced by TestDiag_ProductionParity_*.
# This means every byte shown here is byte-for-byte what production Tier 2
# would install at runtime. Rule 5 of CLAUDE.md is load-bearing on this.
#
# Usage:
#   bash scripts/diag.sh all                  — dump every domain benchmark
#   bash scripts/diag.sh numeric              — dump numeric/ only
#   bash scripts/diag.sh table                — dump table/ only
#   bash scripts/diag.sh <benchmark>          — dump a single benchmark.
#                                                Forms accepted:
#                                                  sieve, sieve.leia
#                                                  control/sieve, control/sieve.leia
#                                                  table/json_table_walk
#                                                  recursion/ack_nested_shifted
#                                                Bare basenames are searched in
#                                                the domain order below.
#
# Runtime: ~3 seconds per benchmark, ~2 minutes for the full all-domain dump.

set -uo pipefail
cd "$(dirname "$0")/.."

BENCHMARK=""
for arg in "$@"; do
    case "$arg" in
        all|numeric|recursion|table|calls|string|concurrency|data|app|control|precision) BENCHMARK="$arg" ;;
        *) BENCHMARK="$arg" ;;
    esac
done

if [ -z "$BENCHMARK" ]; then
    echo "Usage: $0 <benchmark|all|domain>" >&2
    exit 2
fi

DIAG_ROOT="diag"
mkdir -p "$DIAG_ROOT"

# Resolve benchmark list. Each entry is "<domain>/<file>.leia", relative to
# benchmarks/.
BENCHES=()
DOMAIN_GROUPS=(numeric recursion table calls string concurrency data app control precision)
DEFAULT_ORDER=(
    fib fib_recursive sieve mandelbrot ackermann matmul spectral_norm nbody
    fannkuch sort sum_primes mutual_recursion method_dispatch closure_bench
    string_bench binary_trees table_field_access table_array_access
    coroutine_bench fibonacci_iterative math_intensive object_creation
)

is_domain_group() {
    local value="$1"
    local group
    for group in "${DOMAIN_GROUPS[@]}"; do
        [ "$value" = "$group" ] && return 0
    done
    return 1
}

add_group_benches() {
    local group="$1"
    local bench_dir="benchmarks/$group"
    local name path
    [ -d "$bench_dir" ] || return 0

    declare -A seen=()
    for name in "${DEFAULT_ORDER[@]}"; do
        path="$bench_dir/$name.leia"
        if [ -f "$path" ]; then
            BENCHES+=("$group/$name.leia")
            seen["$name"]=1
        fi
    done
    while IFS= read -r path; do
        [ -n "$path" ] || continue
        name="$(basename "$path" .leia)"
        [ -n "${seen[$name]+x}" ] && continue
        BENCHES+=("$group/$name.leia")
    done < <(find "$bench_dir" -maxdepth 1 -type f -name '*.leia' | sort)
}

resolve_selector() {
    local selector="${1#benchmarks/}"
    selector="${selector%.leia}"
    local group="${selector%%/*}"
    local name="${selector#*/}"
    if [ "$group" != "$selector" ] && is_domain_group "$group" && [ -n "$name" ] && [ -f "benchmarks/$group/$name.leia" ]; then
        echo "$group/$name.leia"
        return 0
    fi
    return 1
}

if [ "$BENCHMARK" = "all" ]; then
    for group in "${DOMAIN_GROUPS[@]}"; do
        add_group_benches "$group"
    done
elif is_domain_group "$BENCHMARK"; then
    add_group_benches "$BENCHMARK"
else
    resolved="$(resolve_selector "$BENCHMARK" || true)"
    if [ -z "$resolved" ]; then
        echo "No such domain benchmark: $BENCHMARK" >&2
        exit 2
    fi
    BENCHES+=("$resolved")
fi

if [ ${#BENCHES[@]} -eq 0 ]; then
    echo "No benchmarks selected: $BENCHMARK" >&2
    exit 2
fi

if [ "$BENCHMARK" = "all" ]; then
    # Wipe everything inside diag/ — both the new nested layout and any
    # previous-schema flat dirs — so a renamed/removed source can't leave a
    # stale stats.json behind. summary.md is regenerated at the end.
    find "$DIAG_ROOT" -mindepth 1 -maxdepth 1 ! -name summary.md -exec rm -rf {} +
fi

echo "=== scripts/diag.sh ==="
echo "Benchmarks: ${#BENCHES[@]}"
echo

failed=()
for bench in "${BENCHES[@]}"; do
    sub="${bench%%/*}"               # benchmark domain
    file="${bench#*/}"               # foo.leia
    name="${file%.leia}"               # foo
    out_dir="$DIAG_ROOT/$sub/$name"
    rm -rf "$out_dir"
    mkdir -p "$out_dir"

    echo "--- $bench ---"
    if ! DIAG_BENCH="$bench" DIAG_OUT="$(pwd)/$out_dir" \
         go test ./internal/methodjit/ -run '^TestDiagDump$' -count=1 >"$out_dir/gotest.log" 2>&1; then
        echo "  FAIL — see $out_dir/gotest.log"
        failed+=("$bench")
        continue
    fi
    # .asm.txt files are written by the Go harness via
    # golang.org/x/arch/arm64/arm64asm — self-contained, no external
    # disassembler required.

    # Summary line.
    total_insns=$(awk '
        /"insn_count"[[:space:]]*:/ {
            value=$0
            sub(/^.*"insn_count"[[:space:]]*:[[:space:]]*/, "", value)
            sub(/[^0-9].*$/, "", value)
            total += value + 0
        }
        /"insn_count"[[:space:]]*:/ { protos += 1 }
        /"skip_reason"[[:space:]]*:/ {
            value=$0
            sub(/^.*"skip_reason"[[:space:]]*:[[:space:]]*/, "", value)
            if (value ~ /^""/ || value ~ /^null/) promoted += 1
        }
        END {
            if (protos > 0 && promoted == 0) promoted = protos
            printf "  %d/%d protos promoted, %d total insns\n", promoted, protos, total
        }
    ' "$out_dir/stats.json")
    echo "$total_insns"
done

if [ ${#failed[@]} -gt 0 ]; then
    echo
    echo "FAILED: ${failed[*]}"
    exit 1
fi

echo
echo "=== Writing diag/summary.md ==="
go run ./cmd/leia diag summary "$DIAG_ROOT" >"$DIAG_ROOT/summary.md" || {
    echo "diag summary failed (non-fatal)"
}
echo "Done. See $DIAG_ROOT/"
