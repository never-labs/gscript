#!/usr/bin/env bash
# Release-gate entrypoint for the production readiness checklist.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

MODE="full"
LIST_ONLY=0
OUT_DIR=""
ARTIFACT_PLAN=""
ARTIFACT_COMMAND_LOG=""

usage() {
    cat <<'EOF'
Usage: scripts/production_check.sh [--quick] [--full] [--list] [--out-dir DIR] [--help]

Runs the release-gate commands from docs/production-readiness-checklist.md.

Options:
  --quick   Run short correctness gates only: core packages, feature matrix,
            integration, and stdlib contract. Skips long performance passes.
  --full    Run the available Required Commands subset. This is the default.
  --list    Print the commands that would run without executing them.
  --out-dir DIR
            Write the resolved plan and command logs to DIR. Defaults to no
            artifact output.
  --help    Show this help.
EOF
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --quick)
            MODE="quick"
            ;;
        --full)
            MODE="full"
            ;;
        --list)
            LIST_ONLY=1
            ;;
        --out-dir)
            if [ "$#" -lt 2 ] || [ -z "$2" ]; then
                echo "--out-dir requires a directory argument" >&2
                usage >&2
                exit 2
            fi
            OUT_DIR="$2"
            shift
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

RUN_NAMES=()
RUN_CMDS=()
SKIP_REASONS=()

add_run() {
    RUN_NAMES+=("$1")
    RUN_CMDS+=("$2")
}

add_skip() {
    SKIP_REASONS+=("$1: $2")
}

have_cmd() {
    command -v "$1" >/dev/null 2>&1
}

quote_cmd() {
    printf '%s' "$1"
}

artifact_log() {
    if [ -n "$ARTIFACT_COMMAND_LOG" ]; then
        printf '%s\n' "$*" >> "$ARTIFACT_COMMAND_LOG"
    fi
}

safe_artifact_name() {
    printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -c '[:alnum:]' '_'
}

init_artifacts() {
    if [ -z "$OUT_DIR" ]; then
        return
    fi

    mkdir -p "$OUT_DIR"
    ARTIFACT_PLAN="$OUT_DIR/plan.txt"
    ARTIFACT_COMMAND_LOG="$OUT_DIR/commands.log"
    : > "$ARTIFACT_COMMAND_LOG"
}

write_plan_artifact() {
    if [ -z "$ARTIFACT_PLAN" ]; then
        return
    fi

    {
        echo "production_check.sh plan"
        echo "mode: $MODE"
        echo "list_only: $LIST_ONLY"
        echo
        echo "Runnable checks:"
        local i=0
        while [ "$i" -lt ${#RUN_NAMES[@]} ]; do
            echo "  - ${RUN_NAMES[$i]}: $(quote_cmd "${RUN_CMDS[$i]}")"
            i=$((i + 1))
        done
        if [ ${#SKIP_REASONS[@]} -gt 0 ]; then
            echo
            echo "Skipped checks:"
            for reason in "${SKIP_REASONS[@]}"; do
                echo "  - $reason"
            done
        fi
    } > "$ARTIFACT_PLAN"

    artifact_log "plan: $ARTIFACT_PLAN"
}

add_go_test() {
    local name="$1"
    local cmd="$2"

    if have_cmd go; then
        add_run "$name" "$cmd"
    else
        add_skip "$name" "missing go"
    fi
}

add_performance_gate() {
    local cmd="bash scripts/performance_gate.sh --full"

    if ! have_cmd python3; then
        add_skip "Performance Gate" "missing python3"
        return
    fi
    if [ ! -f scripts/performance_gate.sh ]; then
        add_skip "Performance Gate" "missing scripts/performance_gate.sh"
        return
    fi
    if [ ! -f benchmarks/timing_compare.py ]; then
        add_skip "Performance Gate" "missing benchmarks/timing_compare.py"
        return
    fi
    if [ ! -f benchmarks/strict_guard.py ]; then
        add_skip "Performance Gate" "missing benchmarks/strict_guard.py"
        return
    fi
    if ! have_cmd luajit; then
        cmd="$cmd --no-luajit"
    fi
    add_run "Performance Gate" "$cmd"
}

add_release_smoke() {
    if ! have_cmd go; then
        add_skip "Release Smoke" "missing go"
        return
    fi
    if [ ! -f tests/01_basic.gs ]; then
        add_skip "Release Smoke" "missing tests/01_basic.gs"
        return
    fi
    if [ ! -f benchmarks/suite/table_field_access.gs ]; then
        add_skip "Release Smoke" "missing benchmarks/suite/table_field_access.gs"
        return
    fi
    if [ ! -d cmd/dump_bytecode ]; then
        add_skip "Release Smoke" "missing cmd/dump_bytecode"
        return
    fi
    add_run "Release Smoke" \
        "go run ./cmd/gscript tests/01_basic.gs && go run ./cmd/gscript -jit benchmarks/suite/table_field_access.gs && go run ./cmd/dump_bytecode tests/01_basic.gs"
}

build_quick_plan() {
    add_go_test "Core Go packages" \
        "go test ./gscript ./cmd/gscript ./internal/lexer ./internal/parser ./internal/runtime ./internal/vm -count=1"
    add_go_test "Feature Matrix and Integration" \
        "go test ./tests -run 'TestFeatureMatrix|TestIntegration' -count=1"
    add_go_test "Stdlib Contract" \
        "go test ./tests -run TestStdlibContract -count=1"
}

build_full_plan() {
    add_go_test "Correctness" \
        "go test ./... -count=1"
    add_go_test "Feature Matrix" \
        "go test ./tests -run 'TestFeatureMatrix|TestIntegration' -count=1"
    add_go_test "Official Lua Compatibility Surface" \
        "go test ./tests -run Official -count=1"
    add_performance_gate
    add_release_smoke
}

if [ "$MODE" = "quick" ]; then
    build_quick_plan
else
    build_full_plan
fi

if ! have_cmd pytest; then
    add_skip "pytest checks" "missing pytest; no pytest-based Required Command is currently configured"
fi

init_artifacts
write_plan_artifact

echo "=== production_check.sh ($MODE) ==="
if [ -n "$OUT_DIR" ]; then
    echo "Artifact output: $OUT_DIR"
fi
if ! have_cmd luajit; then
    echo "LuaJIT not found; benchmark commands that support it will use --no-luajit."
fi
echo

if [ ${#RUN_NAMES[@]} -eq 0 ]; then
    echo "No runnable production checks were found."
    artifact_log "no runnable production checks were found"
    for reason in "${SKIP_REASONS[@]}"; do
        echo "SKIP $reason"
        artifact_log "SKIP $reason"
    done
    exit 1
fi

echo "Runnable checks:"
i=0
while [ "$i" -lt ${#RUN_NAMES[@]} ]; do
    echo "  - ${RUN_NAMES[$i]}: $(quote_cmd "${RUN_CMDS[$i]}")"
    i=$((i + 1))
done

if [ ${#SKIP_REASONS[@]} -gt 0 ]; then
    echo
    echo "Skipped checks:"
    for reason in "${SKIP_REASONS[@]}"; do
        echo "  - $reason"
    done
fi

if [ "$LIST_ONLY" -eq 1 ]; then
    artifact_log "list-only mode; no commands executed"
    exit 0
fi

echo
failed=()
i=0
while [ "$i" -lt ${#RUN_NAMES[@]} ]; do
    name="${RUN_NAMES[$i]}"
    cmd="${RUN_CMDS[$i]}"
    log_file=""
    if [ -n "$OUT_DIR" ]; then
        log_file="$OUT_DIR/$(printf '%02d' $((i + 1)))-$(safe_artifact_name "$name").log"
        artifact_log "RUN $name"
        artifact_log "CMD $cmd"
        artifact_log "LOG $log_file"
    fi
    echo "=== RUN $name ==="
    if [ -n "$log_file" ]; then
        bash -lc "$cmd" 2>&1 | tee "$log_file"
        status=${PIPESTATUS[0]}
    else
        bash -lc "$cmd"
        status=$?
    fi
    if [ "$status" -eq 0 ]; then
        echo "=== PASS $name ==="
        artifact_log "PASS $name"
    else
        echo "=== FAIL $name (exit $status) ==="
        artifact_log "FAIL $name (exit $status)"
        failed+=("$name")
    fi
    echo
    artifact_log
    i=$((i + 1))
done

if [ ${#failed[@]} -gt 0 ]; then
    echo "Failed checks:"
    for name in "${failed[@]}"; do
        echo "  - $name"
    done
    exit 1
fi

artifact_log "all runnable production checks passed"
echo "All runnable production checks passed."
