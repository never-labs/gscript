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
while IFS= read -r bench; do
    [ -n "$bench" ] || continue
    BENCHES+=("$bench")
done < <(python3 - "$BENCHMARK" <<'PY'
import sys
from pathlib import Path

root = Path.cwd()
sys.path.insert(0, str(root / "benchmarks"))
import benchmark_discovery as discovery

selector = sys.argv[1]
try:
    if selector == "all":
        specs = discovery.discover_benchmarks(root, discovery.GROUPS)
        for spec in specs:
            print(spec.leia.relative_to(root / "benchmarks"))
        raise SystemExit(0)

    if selector in discovery.GROUPS:
        specs = discovery.discover_benchmarks(root, [selector])
        for spec in specs:
            print(spec.leia.relative_to(root / "benchmarks"))
        raise SystemExit(0)

    path = discovery.resolve_script_path(root, selector)
    if path is None:
        raise SystemExit(f"No such domain benchmark: {selector}")
    print(path.relative_to(root / "benchmarks"))
except SystemExit:
    raise
except Exception as exc:
    raise SystemExit(str(exc))
PY
)

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
go run ./cmd/leia diag summary "$DIAG_ROOT" >"$DIAG_ROOT/summary.md" || {
    echo "diag summary failed (non-fatal)"
}
echo "Done. See $DIAG_ROOT/"
