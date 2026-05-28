#!/usr/bin/env bash
# Release-gate entrypoint for the production readiness checklist.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

MODE="full"
LIST_ONLY=0

usage() {
    cat <<'EOF'
Usage: scripts/production_check.sh [--quick] [--full] [--list] [--help]

Runs the release-gate commands from docs/production-readiness-checklist.md.

Options:
  --quick   Run short correctness gates only: core packages, feature matrix,
            integration, and stdlib contract. Skips long performance passes.
  --full    Run the available Required Commands subset. This is the default.
  --list    Print the commands that would run without executing them.
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

echo "=== production_check.sh ($MODE) ==="
if ! have_cmd luajit; then
    echo "LuaJIT not found; benchmark commands that support it will use --no-luajit."
fi
echo

if [ ${#RUN_NAMES[@]} -eq 0 ]; then
    echo "No runnable production checks were found."
    for reason in "${SKIP_REASONS[@]}"; do
        echo "SKIP $reason"
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
    exit 0
fi

echo
failed=()
i=0
while [ "$i" -lt ${#RUN_NAMES[@]} ]; do
    name="${RUN_NAMES[$i]}"
    cmd="${RUN_CMDS[$i]}"
    echo "=== RUN $name ==="
    if bash -lc "$cmd"; then
        echo "=== PASS $name ==="
    else
        status=$?
        echo "=== FAIL $name (exit $status) ==="
        failed+=("$name")
    fi
    echo
    i=$((i + 1))
done

if [ ${#failed[@]} -gt 0 ]; then
    echo "Failed checks:"
    for name in "${failed[@]}"; do
        echo "  - $name"
    done
    exit 1
fi

echo "All runnable production checks passed."
