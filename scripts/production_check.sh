#!/usr/bin/env bash
# Release-gate entrypoint for the production readiness checklist.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

MODE="full"
LIST_ONLY=0
OUT_DIR=""
RELEASE_PROFILE=0
ARTIFACT_PLAN=""
ARTIFACT_COMMAND_LOG=""
SMOKE_SCRIPT="tests/smoke/01_basic.leia"
EXPECTED_MODULE_PATH="github.com/never-labs/leia"

usage() {
    cat <<'EOF'
Usage: scripts/production_check.sh [--quick] [--full] [--release-profile] [--list] [--out-dir DIR] [--help]

Runs the release-gate commands from docs/release/index.md.

Options:
  --quick   Run short correctness gates plus core performance smoke: core
            packages, feature matrix, integration, release matrix metadata,
            stdlib contract, and representative stable hot paths.
  --full    Run the available Required Commands subset. This is the default.
  --release-profile
            Treat release-only tool checks as required: editor tree-sitter
            corpus, distribution configuration, and artifact dry-run evidence.
            This avoids silent skips for local release validation while keeping
            ordinary contributor machines able to run --full.
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
        --release-profile)
            MODE="full"
            RELEASE_PROFILE=1
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
        echo "release_profile: $RELEASE_PROFILE"
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
    add_run "Performance Gate" "$cmd"
}

add_performance_smoke() {
    local cmd="bash scripts/performance_gate.sh --smoke --runs 2 --warmup 1"

    if ! have_cmd python3; then
        add_skip "Performance Smoke" "missing python3"
        return
    fi
    if [ ! -f scripts/performance_gate.sh ]; then
        add_skip "Performance Smoke" "missing scripts/performance_gate.sh"
        return
    fi
    if [ ! -f benchmarks/timing_compare.py ]; then
        add_skip "Performance Smoke" "missing benchmarks/timing_compare.py"
        return
    fi
    if [ ! -f benchmarks/strict_guard.py ]; then
        add_skip "Performance Smoke" "missing benchmarks/strict_guard.py"
        return
    fi
    add_run "Performance Smoke" "$cmd"
}

add_q_performance_gate() {
    if ! have_cmd go; then
        add_skip "Q Performance Gate" "missing go"
        return
    fi
    if ! have_cmd python3; then
        add_skip "Q Performance Gate" "missing python3"
        return
    fi
    if [ ! -f benchmarks/q_performance_suite.sh ]; then
        add_skip "Q Performance Gate" "missing benchmarks/q_performance_suite.sh"
        return
    fi
    if [ ! -f benchmarks/q_perf_report.py ]; then
        add_skip "Q Performance Gate" "missing benchmarks/q_perf_report.py"
        return
    fi
    # Go-bench q suite (timing_compare leg skipped; the generic Performance
    # Gate already covers current-vs-HEAD timing) teed into the q ratio /
    # runtime-counter report check. Full mode only, like the other long
    # performance stages; --quick keeps it out of the plan.
    add_run "Q Performance Gate" \
        "q_perf_out=\$(mktemp \"\${TMPDIR:-/tmp}/leia-q-perf-gate.XXXXXX\") && trap 'rm -f \"\$q_perf_out\"' EXIT && set -o pipefail && LEIA_SKIP_TIMING_COMPARE=1 bash benchmarks/q_performance_suite.sh | tee \"\$q_perf_out\" && python3 benchmarks/q_perf_report.py --from-output \"\$q_perf_out\" --check"
}

add_documentation_references() {
    if [ ! -f scripts/docs_check.sh ]; then
        add_skip "Documentation References" "missing scripts/docs_check.sh"
        return
    fi
    if ! have_cmd python3; then
        add_skip "Documentation References" "missing python3"
        return
    fi
    add_run "Documentation References" "bash scripts/docs_check.sh"
}

add_editor_assets() {
    if [ ! -f scripts/editor_check.sh ]; then
        add_skip "Editor Assets" "missing scripts/editor_check.sh"
        return
    fi
    if [ "$RELEASE_PROFILE" -eq 1 ]; then
        add_run "Editor Assets" "bash scripts/editor_check.sh --require-tree-sitter"
        return
    fi
    if ! have_cmd python3; then
        add_skip "Editor Assets" "missing python3"
        return
    fi
    if ! have_cmd node; then
        add_skip "Editor Assets" "missing node"
        return
    fi
    add_run "Editor Assets" "bash scripts/editor_check.sh"
}

add_release_distribution_gate() {
    if [ ! -f scripts/release_distribution_check.sh ]; then
        add_skip "Release Distribution" "missing scripts/release_distribution_check.sh"
        return
    fi
    if [ "$RELEASE_PROFILE" -eq 1 ]; then
        add_run "Release Distribution" "bash scripts/release_distribution_check.sh --require-goreleaser --require-workflows"
    else
        add_run "Release Distribution" "bash scripts/release_distribution_check.sh"
    fi
}

add_release_artifacts_gate() {
    if [ ! -f scripts/release_artifacts_check.sh ]; then
        add_skip "Release Artifacts" "missing scripts/release_artifacts_check.sh"
        return
    fi
    if [ "$RELEASE_PROFILE" -eq 1 ]; then
        add_run "Release Artifacts" "bash scripts/release_artifacts_check.sh --build"
        return
    fi
    if ! have_cmd go; then
        add_skip "Release Artifacts" "missing go"
        return
    fi
    add_run "Release Artifacts" "bash scripts/release_artifacts_check.sh"
}

add_manifest_coverage() {
    if [ ! -f tests/manifest.py ]; then
        add_skip "Manifest Coverage" "missing tests/manifest.py"
        return
    fi
    if [ ! -f tests/manifest.json ]; then
        add_skip "Manifest Coverage" "missing tests/manifest.json"
        return
    fi
    if [ ! -f benchmarks/manifest.json ]; then
        add_skip "Manifest Coverage" "missing benchmarks/manifest.json"
        return
    fi
    if ! have_cmd python3; then
        add_skip "Manifest Coverage" "missing python3"
        return
    fi
    add_run "Manifest Coverage" "python3 tests/manifest.py check tests benchmarks"
}

add_module_path_gate() {
    if ! have_cmd go; then
        add_skip "Module Path Gate" "missing go"
        return
    fi
    add_run "Module Path Gate" "test \"\$(go list -m)\" = \"$EXPECTED_MODULE_PATH\""
}

add_release_smoke() {
    if ! have_cmd go; then
        add_skip "Release Smoke" "missing go"
        return
    fi
    if [ ! -f "$SMOKE_SCRIPT" ]; then
        add_skip "Release Smoke" "missing $SMOKE_SCRIPT"
        return
    fi
    if [ ! -f benchmarks/table/table_field_access.leia ]; then
        add_skip "Release Smoke" "missing benchmarks/table/table_field_access.leia"
        return
    fi
    add_run "Release Smoke" \
        "go run ./cmd/leia $SMOKE_SCRIPT && go run ./cmd/leia -jit benchmarks/table/table_field_access.leia && go run ./cmd/leia inspect bytecode $SMOKE_SCRIPT"
}

add_cli_experience_gate() {
    if ! have_cmd go; then
        add_skip "CLI Experience" "missing go"
        return
    fi
    if [ ! -f "$SMOKE_SCRIPT" ]; then
        add_skip "CLI Experience" "missing $SMOKE_SCRIPT"
        return
    fi
    if [ ! -f examples/hello/fib.leia ] || [ ! -f examples/hello/types_demo.leia ] || [ ! -f examples/hello/dialects.leia ]; then
        add_skip "CLI Experience" "missing hello examples"
        return
    fi
    if [ ! -f examples/evaluate/basic_assert.leia ]; then
        add_skip "CLI Experience" "missing examples/evaluate/basic_assert.leia"
        return
    fi
    add_run "CLI Experience" \
        "cli_tmp=\$(mktemp -d \"\${TMPDIR:-/tmp}/leia-cli-experience.XXXXXX\") && trap 'rm -rf \"\$cli_tmp\"' EXIT && go run ./cmd/leia run $SMOKE_SCRIPT && go run ./cmd/leia test $SMOKE_SCRIPT && go run ./cmd/leia check --quick $SMOKE_SCRIPT && go run ./cmd/leia examples check --jobs=6 examples/hello/fib.leia examples/hello/types_demo.leia examples/hello/dialects.leia && go run ./cmd/leia evaluate --json examples/evaluate/basic_assert.leia && go run ./cmd/leia mod init --module example.com/cli-experience --dir \"\$cli_tmp\" && go run ./cmd/leia mod check --json \"\$cli_tmp\" && go run ./cmd/leia playground --help"
}

add_methodjit_regression_gate() {
    if ! have_cmd go; then
        add_skip "MethodJIT Regression" "missing go"
        return
    fi
    local goos
    local goarch
    goos="$(go env GOOS 2>/dev/null || true)"
    goarch="$(go env GOARCH 2>/dev/null || true)"
    if [ "$goos" != "darwin" ] || [ "$goarch" != "arm64" ]; then
        add_skip "MethodJIT Regression" "methodjit native tests require darwin/arm64, got ${goos}/${goarch}"
        return
    fi
    add_run "MethodJIT Regression" \
        "go test ./internal/vm -run TestCompilerReturn -count=1 && go test ./internal/methodjit -run 'TestRawIntSelfABI_NonEligibleStaysBoxed|TestRawIntSelfABI_EligibleExecutionMatrix|TestRawIntSelfABI_ExitResumeFallbackKeepsCallerLiveValues|TestExitResumeCheck_RawIntSelfCallFallbackFrame|TestTier2_StringFormatLookupPreservesPositiveDivisorModuloSemantics|TestTier2_StringFormatIntLoweringCoversGenericSingleIntPatterns|TestTier2_StringFormatIntMinInt64FallsBackPrecisely|TestTier2_StringFormatIntReboundCalleeFallsBackPrecisely|TestTier2_StringFormatIntFeedbackDynamicPatternGuardsPattern|TestTier2_StringFormatConstMultiArgUsesPreciseOpExit' -count=1"
}

add_race_smoke_gate() {
    if ! have_cmd go; then
        add_skip "Concurrency Race Smoke" "missing go"
        return
    fi
    local goos
    local goarch
    goos="$(go env GOOS 2>/dev/null || true)"
    goarch="$(go env GOARCH 2>/dev/null || true)"
    case "$goos/$goarch" in
        darwin/arm64|darwin/amd64|linux/amd64|linux/arm64)
            ;;
        *)
            add_skip "Concurrency Race Smoke" "race detector not enabled for ${goos}/${goarch}"
            return
            ;;
    esac
    add_run "Concurrency Race Smoke" \
        "go test -race ./internal/runtime ./internal/nanbox ./internal/vm ./llm ./tests/sdk ./tests/llm ./cmd/leia -count=1"
    add_run "Go-style Concurrency Contract" \
        "go test -race ./tests -run TestGoStyleConcurrencyContract -count=1"
}

build_quick_plan() {
    add_go_test "Core Go packages" \
        "go test . ./cmd/leia ./internal/lexer ./internal/parser ./internal/runtime ./internal/vm -count=1"
    add_race_smoke_gate
    add_methodjit_regression_gate
    add_go_test "Feature Matrix and Integration" \
        "go test ./tests -run 'TestFeatureMatrix|TestIntegration' -count=1"
    add_go_test "Release Matrix Metadata" \
        "go test ./tests -run 'TestFeatureMatrix|TestReleaseMatrix' -count=1"
    add_go_test "Spec Runnable Examples" \
        "go test ./tests -run 'TestSpecRunnableExamples|TestSpecLeiaCodeFencesAreExecutableOrExplicitlyNonExecutable' -count=1"
    add_go_test "Stdlib Contract" \
        "go test ./tests -run TestStdlibContract -count=1"
    add_manifest_coverage
    add_module_path_gate
    add_performance_smoke
    add_documentation_references
    add_editor_assets
}

build_full_plan() {
    add_go_test "Correctness" \
        "go test ./... -count=1"
    add_race_smoke_gate
    if have_cmd go; then
        add_skip "Feature Matrix" "covered by Correctness (go test ./... -count=1)"
        add_skip "Language Conformance Surface" "covered by Correctness (go test ./... -count=1)"
        add_skip "Release Matrix Metadata" "covered by Correctness (go test ./... -count=1)"
    else
        add_skip "Feature Matrix" "missing go"
        add_skip "Language Conformance Surface" "missing go"
        add_skip "Release Matrix Metadata" "missing go"
    fi
    add_manifest_coverage
    add_module_path_gate
    add_documentation_references
    add_editor_assets
    add_performance_gate
    add_q_performance_gate
    add_release_smoke
    add_cli_experience_gate
    if [ "$RELEASE_PROFILE" -eq 1 ]; then
        add_release_distribution_gate
        add_release_artifacts_gate
    fi
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
if [ "$RELEASE_PROFILE" -eq 1 ]; then
    echo "Release profile: critical release tool skips are treated as failures."
fi
if [ -n "$OUT_DIR" ]; then
    echo "Artifact output: $OUT_DIR"
fi
if ! have_cmd luajit; then
    echo "LuaJIT not found; performance gates that compare against LuaJIT are expected to fail."
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
