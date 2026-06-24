#!/bin/bash
# Repeatable performance gate for core Leia hot paths.
#
# This is a thin wrapper around go run ./cmd/leia bench compare plus an optional
# go run ./cmd/leia bench strict truth pass. It does not tune workloads in production code; it
# only chooses representative benchmark inputs, records artifacts, sorts
# current-vs-HEAD deltas, and fails on clear regressions or unreliable samples.

set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

RUNS=5
WARMUP=1
TIMEOUT=120
MIN_SAMPLE_SECONDS=0.100
MAX_REPEAT=128
MIN_WALL_REPEAT=8
THRESHOLD=0.12
WALL_THRESHOLD=0.30
LUAJIT_THRESHOLD=0.80
OUT_DIR="${TMPDIR:-/tmp}/leia_performance_gate"
HEAD_REF="HEAD"
PROFILE="core"
STRICT=1
NO_LUAJIT=0
VALIDATE_ONLY=""
JSON_OUT=0
JOBS="${LEIA_PERF_JOBS:-}"
MAX_JOBS="${LEIA_PERF_MAX_JOBS:-8}"

BENCHES=()

CORE_BENCHES=(
    "recursion/mutual_recursion"
    "calls/method_dispatch"
    "table/table_array_access"
    "numeric/spectral_norm"
    "app/actors_dispatch_mutation"
    "calls/calls_vararg_coroutine"
    "table/nextvar_table"
)

SMOKE_BENCHES=(
    "control/sieve"
    "table/table_array_access"
)

SYNTAX_SMOKE_BENCHES=(
    "control/sieve"
    "calls/calls_vararg_coroutine"
    "table/table_array_access"
    "string/string_bench"
    "data/soa_affine_many"
)

SYNTAX_DIALECT_SMOKE_BENCHES=(
    "app/dialect_syntax_smoke"
)

STRICT_SMOKE_BENCHES=(
    "control/sieve"
    "string/string_bench"
)

PHASE_SMOKE_BENCHES=(
    "control/sieve"
    "table/table_array_access"
    "concurrency/producer_consumer_pipeline"
    "calls/calls_vararg_coroutine"
    "app/stdlib_host"
    "app/ai_runtime_smoke"
    "app/serve_loopback_smoke"
    "app/sqlite_memory_smoke"
    "numeric/matmul_dense"
    "data/soa_affine_many"
    "data/soa_masked_aggregate"
)

FEATURE_SMOKE_BENCHES=(
    "concurrency/sync_group"
    "calls/calls_vararg_coroutine"
    "app/stdlib_host"
    "app/ai_runtime_smoke"
    "app/serve_loopback_smoke"
    "app/sqlite_memory_smoke"
    "numeric/matmul_dense"
    "data/q_query_rollup"
    "data/soa_affine_many"
    "data/soa_masked_aggregate"
    "data/soa_filter_gather"
)

STRICT_CORE_BENCHES=(
    "control/sieve"
    "string/string_bench"
    "table/table_field_access"
    "table/json_table_walk"
)

STRICT_FEATURE_BENCHES=(
    "concurrency/sync_group"
    "calls/calls_vararg_coroutine"
    "app/stdlib_host"
    "app/ai_runtime_smoke"
    "app/serve_loopback_smoke"
    "app/sqlite_memory_smoke"
    "numeric/matmul_dense"
    "data/q_query_rollup"
    "data/soa_affine_many"
    "data/soa_masked_aggregate"
    "data/soa_filter_gather"
)

usage() {
    cat <<'USAGE'
Usage: bash scripts/performance_gate.sh [options]

Options:
  --smoke                 Run a short two-benchmark gate.
  --syntax-smoke          Run a fast grammar-change hot-path gate plus Leia-only dialect truth smoke.
  --phase-smoke           Run stage-end correctness + performance smoke.
  --quick-phase-smoke     Run an explicit fast phase smoke for local iteration.
  --feature-smoke         Run hot-path smoke coverage for newer language features.
  --full                  Run all benchmark groups through leia bench compare.
  --bench ID              Add one benchmark selector, e.g. numeric/spectral_norm.
  --runs N                Measured timing samples after calibration. Default: 5.
  --warmup N              Warmup samples after calibration. Default: 1.
  --timeout N             Timeout per command invocation. Default: 120.
  --threshold F           Script-timed current/HEAD regression limit. Default: 0.12.
  --wall-threshold F      Wall-timed current/HEAD regression limit. Default: 0.30.
  --luajit-threshold F    Script-timed current/LuaJIT limit. Default: 0.80.
  --out-dir DIR           Artifact directory. Default: ${TMPDIR:-/tmp}/leia_performance_gate.
  --head-ref REF          Clean baseline ref for leia bench compare. Default: HEAD.
  --no-luajit             Skip LuaJIT timing.
  --jobs N                Run up to N benchmarks concurrently. Default: auto, capped by LEIA_PERF_MAX_JOBS or 8.
  --strict / --no-strict  Enable or skip leia bench strict truth pass. Default: --strict.
  --validate-only JSON    Only validate an existing leia bench compare JSON artifact.
  --json                  Print a validate-only machine-readable report. Valid with --validate-only.
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
            # Smoke is a short local signal. Keep LuaJIT evidence enabled, but
            # leave a small margin for 1-2 sample timing jitter. Full/release
            # gates keep the default 0.80 threshold.
            LUAJIT_THRESHOLD=0.85
            # Strict smoke includes short script-timed rows; keep samples away
            # from the timer-resolution boundary so one 0.099s sample cannot
            # make an otherwise healthy truth pass look partial.
            MIN_SAMPLE_SECONDS=0.300
            MAX_REPEAT=256
            ;;
        --syntax-smoke)
            PROFILE="syntax_smoke"
            RUNS=1
            WARMUP=0
            TIMEOUT=45
            MIN_SAMPLE_SECONDS=0.020
            MAX_REPEAT=16
            MIN_WALL_REPEAT=4
            STRICT=1
            ;;
        --phase-smoke)
            PROFILE="phase_smoke"
            RUNS=2
            WARMUP=1
            TIMEOUT=90
            ;;
        --quick-phase-smoke)
            PROFILE="quick_phase_smoke"
            RUNS=1
            WARMUP=0
            TIMEOUT=60
            ;;
        --feature-smoke)
            PROFILE="feature_smoke"
            RUNS=2
            WARMUP=1
            TIMEOUT=90
            # Feature smoke includes loopback/http/sqlite/data workloads whose
            # individual script timings are short enough that 0.1s samples can
            # make current-vs-HEAD comparisons fail on measurement noise alone.
            MIN_SAMPLE_SECONDS=0.300
            MAX_REPEAT=256
            # Keep the mixed feature smoke serial by default. These workloads
            # compare current, clean HEAD, and LuaJIT binaries; running several
            # calibrated samples at once measures local CPU contention more
            # than language/runtime performance. A caller can still pass
            # --jobs=N explicitly when using the profile for exploratory runs.
            MAX_JOBS=1
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
        --luajit-threshold)
            shift
            LUAJIT_THRESHOLD="$1"
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
        --jobs)
            shift
            JOBS="$1"
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
        --json)
            JSON_OUT=1
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

if [ "$JSON_OUT" -eq 1 ] && [ -z "$VALIDATE_ONLY" ]; then
    echo "--json is only supported with --validate-only" >&2
    exit 2
fi

json_escape() {
    local value="$1"
    value="${value//\\/\\\\}"
    value="${value//\"/\\\"}"
    value="${value//$'\n'/\\n}"
    value="${value//$'\r'/\\r}"
    value="${value//$'\t'/\\t}"
    printf '%s' "$value"
}

print_json_string_array_from_file() {
    local file="$1"
    printf '['
    local first=1
    while IFS= read -r line; do
        if [ "$first" -eq 0 ]; then
            printf ','
        fi
        printf '\n    "%s"' "$(json_escape "$line")"
        first=0
    done < "$file"
    if [ "$first" -eq 0 ]; then
        printf '\n  '
    fi
    printf ']'
}

print_json_string_array() {
    local indent="$1"
    shift
    local values=("$@")
    printf '['
    local i=0
    while [ "$i" -lt "${#values[@]}" ]; do
        if [ "$i" -eq 0 ]; then
            printf '\n'
        fi
        printf '%s  "%s"' "$indent" "$(json_escape "${values[$i]}")"
        if [ "$i" -lt $((${#values[@]} - 1)) ]; then
            printf ','
        fi
        printf '\n'
        i=$((i + 1))
    done
    if [ "${#values[@]}" -gt 0 ]; then
        printf '%s' "$indent"
    fi
    printf ']'
}

print_validate_json_report() {
    local status="$1"
    local output_file="$2"
    local output_line_count
    local validate_exists=false
    local validate_is_file=false
    local validate_size_bytes=0
    output_line_count="$(awk 'END { print NR + 0 }' "$output_file")"
    if [ -e "$VALIDATE_ONLY" ]; then
        validate_exists=true
    fi
    if [ -f "$VALIDATE_ONLY" ]; then
        validate_is_file=true
        validate_size_bytes="$(wc -c < "$VALIDATE_ONLY" | tr -d '[:space:]')"
    fi
    printf '{\n'
    printf '  "schema_version": 1,\n'
    printf '  "status": "%s",\n' "$status"
    printf '  "validate_only": true,\n'
    printf '  "timing_json": "%s",\n' "$(json_escape "$VALIDATE_ONLY")"
    printf '  "validate_target": {"path": "%s", "exists": %s, "is_file": %s, "size_bytes": %d},\n' "$(json_escape "$VALIDATE_ONLY")" "$validate_exists" "$validate_is_file" "$validate_size_bytes"
    printf '  "no_luajit": %s,\n' "$([ "$NO_LUAJIT" -eq 1 ] && printf true || printf false)"
    printf '  "threshold": %s,\n' "$THRESHOLD"
    printf '  "wall_threshold": %s,\n' "$WALL_THRESHOLD"
    printf '  "luajit_threshold": %s,\n' "$LUAJIT_THRESHOLD"
    printf '  "failure_count": %d,\n' "${#failures[@]}"
    printf '  "failure_kind_count": %d,\n' "${#failure_kinds[@]}"
    printf '  "output_line_count": %d,\n' "$output_line_count"
    printf '  "failure_kinds": '
    print_json_string_array "  " ${failure_kinds[@]+"${failure_kinds[@]}"}
    printf ',\n'
    printf '  "failures": [\n'
    local i=0
    while [ "$i" -lt "${#failures[@]}" ]; do
        printf '    "%s"' "$(json_escape "${failures[$i]}")"
        if [ "$i" -lt $((${#failures[@]} - 1)) ]; then
            printf ','
        fi
        printf '\n'
        i=$((i + 1))
    done
    printf '  ],\n'
    printf '  "failure_details": [\n'
    i=0
    while [ "$i" -lt "${#failures[@]}" ]; do
        printf '    {\n'
        printf '      "message": "%s",\n' "$(json_escape "${failures[$i]}")"
        printf '      "kind": "%s",\n' "$(json_escape "${failure_kinds[$i]}")"
        printf '      "value": "%s"\n' "$(json_escape "${failure_values[$i]}")"
        printf '    }'
        if [ "$i" -lt $((${#failures[@]} - 1)) ]; then
            printf ','
        fi
        printf '\n'
        i=$((i + 1))
    done
    printf '  ],\n'
    printf '  "output_lines": '
    print_json_string_array_from_file "$output_file"
    printf '\n}\n'
}

validate_artifact() {
    local json_path="$1"
    go run ./cmd/leia bench gate-validate --kind compare --threshold "$THRESHOLD" --wall-threshold "$WALL_THRESHOLD" "$json_path"
}

validate_strict_artifact() {
    local json_path="$1"
    go run ./cmd/leia bench gate-validate --kind strict "$json_path"
}

validate_luajit_artifact() {
    local json_path="$1"
    if [ "$NO_LUAJIT" -eq 1 ]; then
        echo "LuaJIT performance submit guard skipped (--no-luajit)."
        return 0
    fi
    go run ./cmd/leia bench submit-guard "$json_path" --ratio-threshold "$LUAJIT_THRESHOLD"
}

if [ -n "$VALIDATE_ONLY" ]; then
    if [ "$JSON_OUT" -eq 1 ]; then
        validate_log="$(mktemp "${TMPDIR:-/tmp}/leia-performance-validate.XXXXXX")"
        failures=()
        failure_kinds=()
        failure_values=()
        if ! validate_artifact "$VALIDATE_ONLY" >"$validate_log" 2>&1; then
            failures+=("timing validation failed")
            failure_kinds+=("timing_validation")
            failure_values+=("$VALIDATE_ONLY")
        fi
        if ! validate_luajit_artifact "$VALIDATE_ONLY" >>"$validate_log" 2>&1; then
            failures+=("luajit submit guard failed")
            failure_kinds+=("luajit_submit_guard")
            failure_values+=("$VALIDATE_ONLY")
        fi
        status="pass"
        if [ "${#failures[@]}" -gt 0 ]; then
            status="issues"
        fi
        print_validate_json_report "$status" "$validate_log"
        rm -f "$validate_log"
        if [ "$status" = "issues" ]; then
            exit 1
        fi
        exit 0
    fi
    validate_artifact "$VALIDATE_ONLY"
    validate_luajit_artifact "$VALIDATE_ONLY"
    exit $?
fi

mkdir -p "$OUT_DIR"

if [ -z "$JOBS" ]; then
    cpu_count="$(getconf _NPROCESSORS_ONLN 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || printf '1')"
    case "$cpu_count" in
        ''|*[!0-9]*) cpu_count=1 ;;
    esac
    case "$MAX_JOBS" in
        ''|*[!0-9]*) MAX_JOBS=8 ;;
    esac
    if [ "$MAX_JOBS" -lt 1 ]; then
        MAX_JOBS=1
    fi
    if [ "$cpu_count" -lt 1 ]; then
        cpu_count=1
    fi
    JOBS="$MAX_JOBS"
    if [ "$cpu_count" -lt "$JOBS" ]; then
        JOBS="$cpu_count"
    fi
fi

TIMING_JSON="$OUT_DIR/timing_gate.json"
TIMING_MD="$OUT_DIR/timing_gate.md"
STRICT_JSON="$OUT_DIR/strict_gate.json"
STRICT_MD="$OUT_DIR/strict_gate.md"
ALL_BENCHMARK_GROUPS=(
    numeric
    recursion
    table
    calls
    string
    concurrency
    data
    app
    control
    precision
)

TIMING_CMD=(
    go run ./cmd/leia bench compare
    --runs="$RUNS"
    --warmup="$WARMUP"
    --timeout="$TIMEOUT"
    --time-source=auto
    --min-sample-seconds="$MIN_SAMPLE_SECONDS"
    --timer-resolution=0.001
    --max-repeat="$MAX_REPEAT"
    --min-wall-repeat="$MIN_WALL_REPEAT"
    --scale-profile=hot
    --sort=luajit-gap
    --progress
    --jobs="$JOBS"
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
elif [ "$PROFILE" = "syntax_smoke" ]; then
    TIMING_CMD+=(--all-groups)
    for bench in "${SYNTAX_SMOKE_BENCHES[@]}"; do
        TIMING_CMD+=(--bench "$bench")
    done
elif [ "$PROFILE" = "phase_smoke" ] || [ "$PROFILE" = "quick_phase_smoke" ]; then
    TIMING_CMD+=(--all-groups)
    for bench in "${PHASE_SMOKE_BENCHES[@]}"; do
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

echo "=== leia bench compare performance gate ==="
printf 'Command:'
printf ' %q' "${TIMING_CMD[@]}"
printf '\n'
"${TIMING_CMD[@]}"

echo
validate_artifact "$TIMING_JSON"
validate_luajit_artifact "$TIMING_JSON"

if [ "$STRICT" -eq 1 ]; then
    STRICT_CMD=(
        go run ./cmd/leia bench strict
        --runs="$RUNS"
        --warmup="$WARMUP"
        --timeout="$TIMEOUT"
        --min-sample-seconds="$MIN_SAMPLE_SECONDS"
        --max-repeat="$MAX_REPEAT"
        --jobs="$JOBS"
        --progress
        --json "$STRICT_JSON"
        --markdown "$STRICT_MD"
    )
    for group in "${ALL_BENCHMARK_GROUPS[@]}"; do
        STRICT_CMD+=(--group "$group")
    done
    if [ "$NO_LUAJIT" -eq 1 ]; then
        STRICT_CMD+=(--no-luajit)
    fi
    if [ "$PROFILE" = "syntax_smoke" ]; then
        STRICT_CMD+=(--mode vm --mode default --mode no_filter)
    fi
    if [ "$PROFILE" = "full" ]; then
        for bench in "${STRICT_CORE_BENCHES[@]}"; do
            STRICT_CMD+=(--bench "$bench")
        done
    else
        if [ "$PROFILE" = "smoke" ]; then
            for bench in "${STRICT_SMOKE_BENCHES[@]}"; do
                STRICT_CMD+=(--bench "$bench")
            done
        elif [ "$PROFILE" = "syntax_smoke" ]; then
            for bench in "${SYNTAX_DIALECT_SMOKE_BENCHES[@]}"; do
                STRICT_CMD+=(--bench "$bench")
            done
        elif [ "$PROFILE" = "phase_smoke" ] || [ "$PROFILE" = "quick_phase_smoke" ]; then
            for bench in "${PHASE_SMOKE_BENCHES[@]}"; do
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
    echo "=== leia bench strict truth pass ==="
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
