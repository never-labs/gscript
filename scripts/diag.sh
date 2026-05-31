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
#   bash scripts/diag.sh suite                — legacy alias for numeric/recursion/table/calls/string/control
#   bash scripts/diag.sh <benchmark>          — dump a single benchmark.
#                                                Forms accepted:
#                                                  sieve, sieve.gs
#                                                  control/sieve, control/sieve.gs
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
        all|numeric|recursion|table|calls|string|concurrency|data|app|control|suite|extended|variants|official|data_oriented) BENCHMARK="$arg" ;;
        *) BENCHMARK="$arg" ;;
    esac
done

if [ -z "$BENCHMARK" ]; then
    echo "Usage: $0 <benchmark|all|domain>" >&2
    exit 2
fi

DIAG_ROOT="diag"
mkdir -p "$DIAG_ROOT"

# Discover which top-level benchmark domain dirs actually exist.
DOMAIN_DIRS=()
for d in numeric recursion table calls string concurrency data app control; do
    if [ -d "benchmarks/$d" ]; then
        DOMAIN_DIRS+=("$d")
    fi
done

domain_list_for() {
    case "$1" in
        all) printf '%s\n' "${DOMAIN_DIRS[@]}" ;;
        suite) printf '%s\n' numeric recursion table calls string control ;;
        extended) printf '%s\n' app table string concurrency ;;
        variants) printf '%s\n' recursion calls numeric table ;;
        official) printf '%s\n' calls control table string app ;;
        data_oriented) printf '%s\n' data ;;
        *) printf '%s\n' "$1" ;;
    esac
}

# Resolve benchmark list. Each entry is "<domain>/<file>.gs", relative to
# benchmarks/. macOS bash 3.2 compatible (no mapfile, no associative arrays).
BENCHES=()
collect_dir() {
    local sub="$1"
    if [ ! -d "benchmarks/$sub" ]; then
        return
    fi
    local f
    for f in "benchmarks/$sub"/*.gs; do
        [ -f "$f" ] || continue
        BENCHES+=("$sub/$(basename "$f")")
    done
}

case "$BENCHMARK" in
    all)
        for d in "${DOMAIN_DIRS[@]}"; do
            collect_dir "$d"
        done
        # Wipe everything inside diag/ — both the new nested layout and any legacy
        # flat dirs from prior runs — so a renamed/removed source can't leave a
        # stale stats.json behind. summary.md is regenerated at the end.
        find "$DIAG_ROOT" -mindepth 1 -maxdepth 1 ! -name summary.md -exec rm -rf {} +
        ;;
    numeric|recursion|table|calls|string|concurrency|data|app|control|suite|extended|variants|official|data_oriented)
        while IFS= read -r d; do
            [ -n "$d" ] || continue
            if [ ! -d "benchmarks/$d" ]; then
                echo "No such benchmark domain: benchmarks/$d" >&2
                exit 2
            fi
            collect_dir "$d"
            rm -rf "$DIAG_ROOT/$d"
        done <<EOF
$(domain_list_for "$BENCHMARK")
EOF
        ;;
    *)
        # Single benchmark. Accept: name | name.gs | domain/name | domain/name.gs.
        rel=""
        if [[ "$BENCHMARK" == */* ]]; then
            rel="$BENCHMARK"
            case "$rel" in
                suite/*)
                    name="${rel#suite/}"
                    for d in numeric recursion table calls string control; do
                        if [ -f "benchmarks/$d/${name%.gs}.gs" ]; then rel="$d/${name%.gs}.gs"; break; fi
                    done
                    ;;
                extended/*)
                    name="${rel#extended/}"
                    for d in app table string concurrency; do
                        if [ -f "benchmarks/$d/${name%.gs}.gs" ]; then rel="$d/${name%.gs}.gs"; break; fi
                    done
                    ;;
                variants/*)
                    name="${rel#variants/}"
                    [ "$name" = "closure_accumulator_variant" ] && name="closure_accumulator"
                    [ "$name" = "matmul_row_variant" ] && name="matmul_row"
                    for d in recursion calls numeric table; do
                        if [ -f "benchmarks/$d/${name%.gs}.gs" ]; then rel="$d/${name%.gs}.gs"; break; fi
                    done
                    ;;
                official/*)
                    name="${rel#official/}"
                    name="${name%_hot}"
                    for d in calls control table string app; do
                        if [ -f "benchmarks/$d/${name%.gs}.gs" ]; then rel="$d/${name%.gs}.gs"; break; fi
                    done
                    ;;
                data_oriented/*)
                    name="${rel#data_oriented/}"
                    name="${name%_hot}"
                    rel="data/${name%.gs}.gs"
                    ;;
            esac
            [[ "$rel" != *.gs ]] && rel="${rel}.gs"
            if [ ! -f "benchmarks/$rel" ]; then
                echo "No such benchmark: benchmarks/$rel" >&2
                exit 2
            fi
        else
            base="${BENCHMARK%.gs}"
            for d in "${DOMAIN_DIRS[@]}"; do
                if [ -f "benchmarks/$d/$base.gs" ]; then
                    rel="$d/$base.gs"
                    break
                fi
            done
            if [ -z "$rel" ]; then
                echo "No such domain benchmark: $BENCHMARK" >&2
                exit 2
            fi
        fi
        BENCHES=("$rel")
        ;;
esac

echo "=== scripts/diag.sh ==="
echo "Benchmarks: ${#BENCHES[@]}"
echo

failed=()
for bench in "${BENCHES[@]}"; do
    sub="${bench%%/*}"               # benchmark domain
    file="${bench#*/}"               # foo.gs
    name="${file%.gs}"               # foo
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
    total_insns=$(python3 -c "
import json, sys
d = json.load(open('$out_dir/stats.json'))
total = sum(p.get('insn_count', 0) for p in d['protos'])
protos = len(d['protos'])
promoted = sum(1 for p in d['protos'] if not p.get('skip_reason'))
print(f'  {promoted}/{protos} protos promoted, {total} total insns')
")
    echo "$total_insns"
done

if [ ${#failed[@]} -gt 0 ]; then
    echo
    echo "FAILED: ${failed[*]}"
    exit 1
fi

echo
echo "=== Writing diag/summary.md ==="
python3 scripts/diag_summary.py "$DIAG_ROOT" >"$DIAG_ROOT/summary.md" || {
    echo "diag_summary.py failed (non-fatal)"
}
echo "Done. See $DIAG_ROOT/"
