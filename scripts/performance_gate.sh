#!/bin/bash
# Repeatable performance gate for core GScript hot paths.
#
# This is a thin wrapper around benchmarks/timing_compare.py plus an optional
# strict_guard.py truth pass. It does not tune workloads in production code; it
# only chooses representative benchmark inputs, records artifacts, sorts
# current-vs-HEAD deltas, and fails on clear regressions or unreliable samples.

set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

RUNS=5
WARMUP=1
TIMEOUT=120
THRESHOLD=0.12
WALL_THRESHOLD=0.30
OUT_DIR="${TMPDIR:-/tmp}/gscript_performance_gate"
HEAD_REF="HEAD"
PROFILE="core"
STRICT=1
NO_LUAJIT=0
VALIDATE_ONLY=""

BENCHES=()

CORE_BENCHES=(
    "suite/mutual_recursion"
    "suite/method_dispatch"
    "suite/table_array_access"
    "suite/spectral_norm"
    "extended/actors_dispatch_mutation"
    "official/calls_vararg_coroutine_hot"
    "official/nextvar_table_hot"
)

SMOKE_BENCHES=(
    "suite/sieve"
    "suite/table_array_access"
)

FEATURE_SMOKE_BENCHES=(
    "extended/producer_consumer_pipeline"
    "official/calls_vararg_coroutine_hot"
    "official/stdlib_host_hot"
    "data_oriented/soa_affine_many_hot"
    "data_oriented/soa_masked_aggregate_hot"
)

STRICT_CORE_BENCHES=(
    "suite/sieve"
    "suite/string_bench"
    "suite/table_array_access"
    "extended/json_table_walk"
)

STRICT_FEATURE_BENCHES=(
    "extended/producer_consumer_pipeline"
    "official/calls_vararg_coroutine_hot"
    "official/stdlib_host_hot"
    "data_oriented/soa_affine_many_hot"
    "data_oriented/soa_masked_aggregate_hot"
)

usage() {
    cat <<'USAGE'
Usage: bash scripts/performance_gate.sh [options]

Options:
  --smoke                 Run a short two-benchmark gate.
  --feature-smoke         Run hot-path smoke coverage for newer language features.
  --full                  Run all benchmark groups through timing_compare.py.
  --bench ID              Add one benchmark selector, e.g. suite/spectral_norm.
  --runs N                Measured timing samples after calibration. Default: 5.
  --warmup N              Warmup samples after calibration. Default: 1.
  --timeout N             Timeout per command invocation. Default: 120.
  --threshold F           Script-timed current/HEAD regression limit. Default: 0.12.
  --wall-threshold F      Wall-timed current/HEAD regression limit. Default: 0.30.
  --out-dir DIR           Artifact directory. Default: ${TMPDIR:-/tmp}/gscript_performance_gate.
  --head-ref REF          Clean baseline ref for timing_compare.py. Default: HEAD.
  --no-luajit             Skip LuaJIT timing.
  --strict / --no-strict  Enable or skip strict_guard.py truth pass. Default: --strict.
  --validate-only JSON    Only validate an existing timing_compare JSON artifact.
  -h, --help              Show this help.

The gate writes timing_gate.json, timing_gate.md, strict_gate.json, and
strict_gate.md under --out-dir when the corresponding pass runs.
USAGE
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --smoke)
            PROFILE="smoke"
            RUNS=1
            WARMUP=0
            TIMEOUT=60
            ;;
        --feature-smoke)
            PROFILE="feature_smoke"
            RUNS=2
            WARMUP=1
            TIMEOUT=90
            ;;
        --full)
            PROFILE="full"
            ;;
        --bench)
            shift
            if [ "$#" -eq 0 ]; then
                echo "--bench requires a selector" >&2
                exit 2
            fi
            BENCHES+=("$1")
            PROFILE="custom"
            ;;
        --runs)
            shift
            RUNS="$1"
            ;;
        --warmup)
            shift
            WARMUP="$1"
            ;;
        --timeout)
            shift
            TIMEOUT="$1"
            ;;
        --threshold)
            shift
            THRESHOLD="$1"
            ;;
        --wall-threshold)
            shift
            WALL_THRESHOLD="$1"
            ;;
        --out-dir)
            shift
            OUT_DIR="$1"
            ;;
        --head-ref)
            shift
            HEAD_REF="$1"
            ;;
        --no-luajit)
            NO_LUAJIT=1
            ;;
        --strict)
            STRICT=1
            ;;
        --no-strict)
            STRICT=0
            ;;
        --validate-only)
            shift
            if [ "$#" -eq 0 ]; then
                echo "--validate-only requires a JSON path" >&2
                exit 2
            fi
            VALIDATE_ONLY="$1"
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
    shift
done

validate_artifact() {
    local json_path="$1"
    python3 - "$json_path" "$THRESHOLD" "$WALL_THRESHOLD" <<'PY'
import json
import math
import sys
from pathlib import Path

path = Path(sys.argv[1])
threshold = float(sys.argv[2])
wall_threshold = float(sys.argv[3])

payload = json.loads(path.read_text())
rows = payload.get("results")
if not isinstance(rows, list):
    print("performance gate: JSON artifact has no list-valued results", file=sys.stderr)
    sys.exit(2)


def subject(row, mode, name):
    modes = row.get("modes")
    if not isinstance(modes, dict):
        return {}
    mode_row = modes.get(mode)
    if not isinstance(mode_row, dict):
        return {}
    value = mode_row.get(name)
    return value if isinstance(value, dict) else {}


def seconds(item):
    stats = item.get("stats")
    if isinstance(stats, dict) and isinstance(stats.get("median"), (int, float)):
        return float(stats["median"])
    if isinstance(item.get("seconds"), (int, float)):
        return float(item["seconds"])
    return None


def source(item):
    return str(item.get("source") or "")


def status(item):
    return str(item.get("status") or "missing")


def is_wall(src):
    parts = {part.strip() for part in src.split(",") if part.strip()}
    return bool(parts & {"wall_repeat", "wall_hr"})


def cv(item):
    stats = item.get("stats")
    if isinstance(stats, dict) and isinstance(stats.get("cv_pct"), (int, float)):
        return float(stats["cv_pct"])
    return None


ranked = []
violations = []
unreliable = []

for raw in rows:
    if not isinstance(raw, dict):
        continue
    group = str(raw.get("group") or "")
    bench = str(raw.get("benchmark") or "")
    name = f"{group}/{bench}" if group else bench
    modes = payload.get("modes") if isinstance(payload.get("modes"), list) else ["default"]
    for mode in modes:
        cur = subject(raw, mode, "current")
        head = subject(raw, mode, "head")
        cur_s = seconds(cur)
        head_s = seconds(head)
        cur_status = status(cur)
        head_status = status(head)
        cur_source = source(cur)
        head_source = source(head)
        ratio = None
        change = None
        comparable = cur_s is not None and head_s is not None and head_s > 0
        if comparable:
            ratio = cur_s / head_s
            change = ratio - 1.0

        note = []
        if cur_status == "low_resolution" or head_status == "low_resolution":
            note.append("low_resolution")
            unreliable.append((name, mode, cur_status, head_status, cur_source, head_source))
        elif cur_status not in {"ok", "partial"} or head_status not in {"ok", "partial"}:
            note.append(f"status={cur_status}/{head_status}")
            unreliable.append((name, mode, cur_status, head_status, cur_source, head_source))

        wall = is_wall(cur_source) or is_wall(head_source)
        if wall:
            note.append("wall_timed_startup_noise")

        if comparable and not note:
            if change is not None and change > threshold:
                violations.append(("regression", name, mode, change, threshold, cur_s, head_s, cur_source, head_source))
        elif comparable and wall and cur_status in {"ok", "partial"} and head_status in {"ok", "partial"}:
            if change is not None and change > wall_threshold:
                violations.append(("wall_regression", name, mode, change, wall_threshold, cur_s, head_s, cur_source, head_source))

        ranked.append(
            {
                "name": name,
                "mode": mode,
                "current": cur_s,
                "head": head_s,
                "change": change,
                "current_status": cur_status,
                "head_status": head_status,
                "current_source": cur_source,
                "head_source": head_source,
                "current_cv": cv(cur),
                "note": ",".join(note) if note else "-",
            }
        )


def fmt_seconds(value):
    return "-" if value is None else f"{value:.6f}s"


def fmt_change(value):
    return "-" if value is None or math.isnan(value) else f"{value * 100:+.2f}%"


def fmt_pct(value):
    return "-" if value is None else f"{value:.2f}%"


ranked.sort(key=lambda row: row["change"] if row["change"] is not None else -999.0, reverse=True)
print("Performance gate current/HEAD ranking:")
print("Benchmark                          Mode      Current      HEAD       Change    CV cur   Status        Source                 Note")
print("-------------------------------------------------------------------------------------------------------------------------------")
for row in ranked:
    print(
        f"{row['name']:<34} {row['mode']:<9} "
        f"{fmt_seconds(row['current']):>10} {fmt_seconds(row['head']):>10} "
        f"{fmt_change(row['change']):>9} {fmt_pct(row['current_cv']):>8} "
        f"{row['current_status'] + '/' + row['head_status']:<13} "
        f"{row['current_source'] + '/' + row['head_source']:<22} "
        f"{row['note']}"
    )

if violations:
    print("\nPerformance gate violations:")
    print("Kind             Benchmark                          Mode      Change     Limit")
    print("----------------------------------------------------------------------------")
    for kind, name, mode, change, limit, cur_s, head_s, cur_source, head_source in violations:
        print(f"{kind:<16} {name:<34} {mode:<9} {change * 100:+8.2f}% {limit * 100:>7.2f}%")

if unreliable:
    print("\nUnreliable timing rows:")
    print("Benchmark                          Mode      Current/HEAD status     Current/HEAD source")
    print("----------------------------------------------------------------------------------------")
    for name, mode, cur_status, head_status, cur_source, head_source in unreliable:
        print(f"{name:<34} {mode:<9} {cur_status + '/' + head_status:<23} {cur_source + '/' + head_source}")

if violations or unreliable:
    sys.exit(1)
print("\nPerformance gate passed.")
PY
}

validate_strict_artifact() {
    local json_path="$1"
    python3 - "$json_path" <<'PY'
import json
import sys
from pathlib import Path

path = Path(sys.argv[1])
payload = json.loads(path.read_text())
rows = payload.get("results")
if not isinstance(rows, list):
    print("strict gate: JSON artifact has no list-valued results", file=sys.stderr)
    sys.exit(2)

violations = []
for raw in rows:
    if not isinstance(raw, dict):
        continue
    group = str(raw.get("group") or "suite")
    bench = str(raw.get("benchmark") or "")
    name = f"{group}/{bench}" if group and not bench.startswith(f"{group}/") else bench
    modes = raw.get("modes")
    if not isinstance(modes, dict):
        violations.append((name, "-", "missing_modes", "-"))
        continue
    for mode in ("vm", "default", "no_filter"):
        result = modes.get(mode)
        if not isinstance(result, dict):
            violations.append((name, mode, "missing", "-"))
            continue
        status = str(result.get("status") or "missing")
        checksum = str(result.get("checksum_status") or "")
        if status != "ok":
            violations.append((name, mode, status, checksum or "-"))
        elif checksum and checksum not in {"ok", "single"}:
            violations.append((name, mode, status, checksum))

if violations:
    print("Strict gate violations:")
    print("Benchmark                          Mode       Status           Checksum")
    print("-----------------------------------------------------------------------")
    for name, mode, status, checksum in violations:
        print(f"{name:<34} {mode:<10} {status:<16} {checksum}")
    sys.exit(1)

print("Strict gate passed.")
PY
}

if [ -n "$VALIDATE_ONLY" ]; then
    validate_artifact "$VALIDATE_ONLY"
    exit $?
fi

mkdir -p "$OUT_DIR"

TIMING_JSON="$OUT_DIR/timing_gate.json"
TIMING_MD="$OUT_DIR/timing_gate.md"
STRICT_JSON="$OUT_DIR/strict_gate.json"
STRICT_MD="$OUT_DIR/strict_gate.md"

TIMING_CMD=(
    python3 benchmarks/timing_compare.py
    --runs="$RUNS"
    --warmup="$WARMUP"
    --timeout="$TIMEOUT"
    --time-source=auto
    --min-sample-seconds=0.100
    --timer-resolution=0.001
    --max-repeat=128
    --min-wall-repeat=8
    --scale-profile=hot
    --sort=luajit-gap
    --head-ref="$HEAD_REF"
    --json "$TIMING_JSON"
    --markdown "$TIMING_MD"
)

if [ "$NO_LUAJIT" -eq 1 ]; then
    TIMING_CMD+=(--no-luajit)
fi

if [ "$PROFILE" = "full" ]; then
    TIMING_CMD+=(--all-groups)
elif [ "$PROFILE" = "smoke" ]; then
    TIMING_CMD+=(--all-groups)
    for bench in "${SMOKE_BENCHES[@]}"; do
        TIMING_CMD+=(--bench "$bench")
    done
elif [ "$PROFILE" = "feature_smoke" ]; then
    TIMING_CMD+=(--all-groups)
    for bench in "${FEATURE_SMOKE_BENCHES[@]}"; do
        TIMING_CMD+=(--bench "$bench")
    done
else
    TIMING_CMD+=(--all-groups)
    if [ "${#BENCHES[@]}" -eq 0 ]; then
        for bench in "${CORE_BENCHES[@]}"; do
            TIMING_CMD+=(--bench "$bench")
        done
    else
        for bench in "${BENCHES[@]}"; do
            TIMING_CMD+=(--bench "$bench")
        done
    fi
fi

echo "=== timing_compare performance gate ==="
printf 'Command:'
printf ' %q' "${TIMING_CMD[@]}"
printf '\n'
"${TIMING_CMD[@]}"

echo
validate_artifact "$TIMING_JSON"

if [ "$STRICT" -eq 1 ]; then
    STRICT_CMD=(
        python3 benchmarks/strict_guard.py
        --runs="$RUNS"
        --warmup="$WARMUP"
        --timeout="$TIMEOUT"
        --min-sample-seconds=0.100
        --max-repeat=128
        --json "$STRICT_JSON"
        --markdown "$STRICT_MD"
    )
    if [ "$NO_LUAJIT" -eq 1 ]; then
        STRICT_CMD+=(--no-luajit)
    fi
    if [ "$PROFILE" != "full" ]; then
        STRICT_CMD+=(--group suite --group extended --group official)
        if [ "$PROFILE" = "smoke" ]; then
            for bench in "${SMOKE_BENCHES[@]}"; do
                STRICT_CMD+=(--bench "$bench")
            done
        elif [ "$PROFILE" = "feature_smoke" ]; then
            for bench in "${STRICT_FEATURE_BENCHES[@]}"; do
                STRICT_CMD+=(--bench "$bench")
            done
        elif [ "${#BENCHES[@]}" -eq 0 ]; then
            for bench in "${STRICT_CORE_BENCHES[@]}"; do
                STRICT_CMD+=(--bench "$bench")
            done
        else
            for bench in "${BENCHES[@]}"; do
                STRICT_CMD+=(--bench "$bench")
            done
        fi
    fi

    echo
    echo "=== strict_guard truth pass ==="
    printf 'Command:'
    printf ' %q' "${STRICT_CMD[@]}"
    printf '\n'
    "${STRICT_CMD[@]}"
    validate_strict_artifact "$STRICT_JSON"
fi

echo
echo "Artifacts:"
echo "  timing JSON:   $TIMING_JSON"
echo "  timing report: $TIMING_MD"
if [ "$STRICT" -eq 1 ]; then
    echo "  strict JSON:   $STRICT_JSON"
    echo "  strict report: $STRICT_MD"
fi
