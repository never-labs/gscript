#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/run.sh <task> [args...]
       scripts/run.sh help <task>

Single repository script launcher.

Tasks:
  arch                 Run architecture health scan
  diag                 Run JIT diagnostic dump
  diagnostics          Collect diagnostics bundle
  docs                 Run documentation checks
  editor               Run editor asset checks
  language-conformance Run translated language conformance cases
  manifest-check       Check test and benchmark manifests
  manifest-list-q      List q manifest paths by scope
  perf                 Run performance gate
  production           Run production readiness gate
  module-path          Check repository module path
  public-blockers      Check public release blocker decisions
  q                    Run q conformance gate
  q-perf               Run q performance report gate
  release-artifacts    Build local release artifacts
  release-artifacts-gate
                       Run release artifact gate with release-profile defaults
  release-check        Check local release artifacts
  release-dist         Check release distribution config
  release-notes        Check release notes evidence
  release-notes-gate   Run release notes gate with release-profile defaults
  release-smoke        Run release-profile smoke checks
  release-snapshot     Verify a snapshot archive through the installer
  site                 Check rendered static site output
  shell-syntax         Parse all tracked shell scripts
  test                 Run named repository test profiles
  cli-experience       Run CLI experience checks
  worktree             Audit git worktrees

Bootstrap-only entrypoints stay outside this launcher:
  scripts/install.sh

Future tasks should move implementation into Leia or Go CLI commands and keep
this file as the only shell launcher.
USAGE
}

if [ "$#" -eq 0 ]; then
  usage >&2
  exit 2
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd "$script_dir/.." && pwd -P)"
cd "$repo_root"

task="$1"
shift
if [ "$task" = "help" ] && [ "$#" -gt 0 ]; then
  task="$1"
  shift
  set -- --help "$@"
fi

run_shell_task() {
  local script="$1"
  shift
  exec bash "$repo_root/$script" "$@"
}

run_leia_task() {
  local script="$1"
  shift
  if [ -n "${LEIA_BIN:-}" ]; then
    exec "$LEIA_BIN" run "$repo_root/$script" "$@"
  fi
  if command -v leia >/dev/null 2>&1; then
    exec leia run "$repo_root/$script" "$@"
  fi
  exec go run ./cmd/leia run "$repo_root/$script" "$@"
}

run_q_perf_task() {
  local output_dir=""
  local suite_args=()
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --output)
        if [ "$#" -lt 2 ] || [ -z "$2" ]; then
          echo "scripts/run.sh q-perf: --output requires a directory" >&2
          exit 2
        fi
        output_dir="$2"
        shift 2
        ;;
      --output=*)
        output_dir="${1#--output=}"
        shift
        ;;
      -h|--help)
        cat <<'USAGE'
Usage: scripts/run.sh q-perf [--output DIR] [q-suite args...]

Runs the q performance suite, captures its output, and checks the q performance
report. The output directory receives output.txt, q_perf_report.json, and
q_perf_report.md.
USAGE
        return
        ;;
      *)
        suite_args+=("$1")
        shift
        ;;
    esac
  done
  if [ -z "$output_dir" ]; then
    output_dir="$(mktemp -d "${TMPDIR:-/tmp}/leia-q-perf-gate.XXXXXX")"
  fi
  mkdir -p "$output_dir"
  set -o pipefail
  LEIA_SKIP_TIMING_COMPARE="${LEIA_SKIP_TIMING_COMPARE:-1}" \
    go run ./cmd/leia bench q-suite "${suite_args[@]}" | tee "$output_dir/output.txt"
  go run ./cmd/leia bench q-report \
    --from-output "$output_dir/output.txt" \
    --check \
    --json "$output_dir/q_perf_report.json" \
    --markdown "$output_dir/q_perf_report.md"
  echo "q performance evidence: $output_dir/q_perf_report.json $output_dir/q_perf_report.md"
}

run_release_artifacts_gate_task() {
  local version=""
  local require_tag=0
  local args=(--build --require-clean)
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --version)
        if [ "$#" -lt 2 ] || [ -z "$2" ]; then
          echo "scripts/run.sh release-artifacts-gate: --version requires a value" >&2
          exit 2
        fi
        version="$2"
        require_tag=1
        shift 2
        ;;
      --version=*)
        version="${1#--version=}"
        require_tag=1
        shift
        ;;
      --require-tag)
        require_tag=1
        shift
        ;;
      -h|--help)
        cat <<'USAGE'
Usage: scripts/run.sh release-artifacts-gate [--version VERSION] [--require-tag]

Runs the release artifact gate with release-profile defaults. The gate builds
artifacts and requires a clean worktree. A version argument, --require-tag, or
LEIA_RELEASE_REQUIRE_TAG=1 adds exact-tag validation.
USAGE
        return
        ;;
      *)
        args+=("$1")
        shift
        ;;
    esac
  done
  if [ -z "$version" ] && [ -n "${LEIA_RELEASE_ARTIFACT_VERSION:-}" ]; then
    version="$LEIA_RELEASE_ARTIFACT_VERSION"
  fi
  if [ "$require_tag" -eq 0 ] && [ -n "${LEIA_RELEASE_REQUIRE_TAG:-}" ]; then
    require_tag=1
  fi
  if [ "$require_tag" -eq 1 ]; then
    if [ -z "$version" ]; then
      version="$(git describe --tags --exact-match)"
    fi
    args+=(--require-tag --version "$version")
  elif [ -n "$version" ]; then
    args+=(--version "$version")
  fi
  run_shell_task scripts/release_artifacts_check.sh "${args[@]}"
}

run_release_notes_gate_task() {
  local version=""
  local require_ready=1
  local args=()
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --version)
        if [ "$#" -lt 2 ] || [ -z "$2" ]; then
          echo "scripts/run.sh release-notes-gate: --version requires a value" >&2
          exit 2
        fi
        version="$2"
        shift 2
        ;;
      --version=*)
        version="${1#--version=}"
        shift
        ;;
      --audit)
        require_ready=0
        shift
        ;;
      -h|--help)
        cat <<'USAGE'
Usage: scripts/run.sh release-notes-gate [--version VERSION] [--audit]

Runs release notes validation with release-profile defaults. By default the
gate requires ready release notes. VERSION, LEIA_RELEASE_ARTIFACT_VERSION, or
LEIA_RELEASE_REQUIRE_TAG=1 selects the release notes file to validate.
USAGE
        return
        ;;
      *)
        args+=("$1")
        shift
        ;;
    esac
  done
  if [ -z "$version" ] && [ -n "${LEIA_RELEASE_ARTIFACT_VERSION:-}" ]; then
    version="$LEIA_RELEASE_ARTIFACT_VERSION"
  fi
  if [ -z "$version" ] && [ -n "${LEIA_RELEASE_REQUIRE_TAG:-}" ]; then
    version="$(git describe --tags --exact-match)"
  fi
  if [ "$require_ready" -eq 1 ]; then
    args+=(--require-ready)
  fi
  if [ -n "$version" ]; then
    args+=(--version "$version")
  fi
  run_shell_task scripts/release_notes_check.sh "${args[@]}"
}

run_release_smoke_task() {
  if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
    cat <<'USAGE'
Usage: scripts/run.sh release-smoke [SMOKE_SCRIPT]

Runs release-profile smoke checks: direct script execution, JIT execution, and
bytecode inspection.
USAGE
    return
  fi
  local smoke_script="${1:-tests/smoke/01_basic.leia}"
  if [ ! -f "$smoke_script" ]; then
    echo "scripts/run.sh release-smoke: missing $smoke_script" >&2
    exit 1
  fi
  if [ ! -f benchmarks/table/table_field_access.leia ]; then
    echo "scripts/run.sh release-smoke: missing benchmarks/table/table_field_access.leia" >&2
    exit 1
  fi
  go run ./cmd/leia "$smoke_script"
  go run ./cmd/leia -jit benchmarks/table/table_field_access.leia
  go run ./cmd/leia inspect bytecode "$smoke_script"
}

run_cli_experience_task() {
  if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
    cat <<'USAGE'
Usage: scripts/run.sh cli-experience [SMOKE_SCRIPT]

Runs user-facing CLI checks covering run, test, check, examples, evaluate,
module setup, and playground help.
USAGE
    return
  fi
  local smoke_script="${1:-tests/smoke/01_basic.leia}"
  if [ ! -f "$smoke_script" ]; then
    echo "scripts/run.sh cli-experience: missing $smoke_script" >&2
    exit 1
  fi
  local required=(
    examples/hello/fib.leia
    examples/hello/types_demo.leia
    examples/hello/dialects.leia
    examples/evaluate/basic_assert.leia
  )
  local path
  for path in "${required[@]}"; do
    if [ ! -f "$path" ]; then
      echo "scripts/run.sh cli-experience: missing $path" >&2
      exit 1
    fi
  done
  local cli_tmp
  cli_tmp="$(mktemp -d "${TMPDIR:-/tmp}/leia-cli-experience.XXXXXX")"
  trap 'rm -rf "$cli_tmp"' RETURN
  go run ./cmd/leia run "$smoke_script"
  go run ./cmd/leia test "$smoke_script"
  go run ./cmd/leia check --quick "$smoke_script"
  go run ./cmd/leia examples check --jobs=6 examples/hello/fib.leia examples/hello/types_demo.leia examples/hello/dialects.leia
  go run ./cmd/leia evaluate --json examples/evaluate/basic_assert.leia
  go run ./cmd/leia mod init --module example.com/cli-experience --dir "$cli_tmp"
  go run ./cmd/leia mod check --json "$cli_tmp"
  go run ./cmd/leia playground --help
}

run_language_conformance_task() {
  if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
    cat <<'USAGE'
Usage: scripts/run.sh language-conformance

Runs translated language conformance cases. LUA_BIN selects the Lua reference
runtime and defaults to lua.
USAGE
    return
  fi
  LUA_BIN="${LUA_BIN:-lua}" go test ./tests -run TestLanguageConformanceTranslatedCases -count=1
}

run_manifest_check_task() {
  if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
    cat <<'USAGE'
Usage: scripts/run.sh manifest-check [ROOT...]

Checks repository manifests with scripts/manifest.leia. Defaults to tests and
benchmarks.
USAGE
    return
  fi
  if [ "$#" -eq 0 ]; then
    set -- tests benchmarks
  fi
  run_leia_task scripts/manifest.leia check "$@"
}

run_manifest_list_q_task() {
  if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
    cat <<'USAGE'
Usage: scripts/run.sh manifest-list-q [--scope SCOPE] KIND

Lists q manifest paths using scripts/manifest.leia. KIND is tests, examples,
or benchmarks. SCOPE defaults to core.
USAGE
    return
  fi
  run_leia_task scripts/manifest.leia list-q "$@"
}

run_module_path_task() {
  local expected="${1:-github.com/never-labs/leia}"
  local actual
  actual="$(go list -m)"
  if [ "$actual" != "$expected" ]; then
    echo "scripts/run.sh module-path: module path $actual, want $expected" >&2
    exit 1
  fi
  echo "module path: $actual"
}

run_shell_syntax_task() {
  local script
  while IFS= read -r script; do
    bash -n "$script"
  done < <(git ls-files '*.sh')
  echo "shell syntax: ok"
}

run_test_task() {
  local profile="${1:-}"
  if [ -z "$profile" ] || [ "$profile" = "-h" ] || [ "$profile" = "--help" ]; then
    cat <<'USAGE'
Usage: scripts/run.sh test <profile>

Profiles:
  core                    Core package smoke tests
  correctness             Full Go test suite
  feature-integration     Feature matrix and integration tests
  release-matrix          Feature and release matrix metadata
  spec-examples           Runnable spec examples
  stdlib                  Standard library contract
  methodjit               MethodJIT regression tests
  race-smoke              Race detector smoke tests
  concurrency-contract    Go-style concurrency contract
USAGE
    return
  fi
  shift
  case "$profile" in
    core)
      go test . ./cmd/leia ./internal/lexer ./internal/parser ./internal/runtime ./internal/vm -count=1 "$@"
      ;;
    correctness)
      go test ./... -count=1 "$@"
      ;;
    feature-integration)
      go test ./tests -run 'TestFeatureMatrix|TestIntegration' -count=1 "$@"
      ;;
    release-matrix)
      go test ./tests -run 'TestFeatureMatrix|TestReleaseMatrix' -count=1 "$@"
      ;;
    spec-examples)
      go test ./tests -run 'TestSpecRunnableExamples|TestSpecLeiaCodeFencesAreExecutableOrExplicitlyNonExecutable' -count=1 "$@"
      ;;
    stdlib)
      go test ./tests -run TestStdlibContract -count=1 "$@"
      ;;
    methodjit)
      go test ./internal/vm -run TestCompilerReturn -count=1 "$@"
      go test ./internal/methodjit -run 'TestRawIntSelfABI_NonEligibleStaysBoxed|TestRawIntSelfABI_EligibleExecutionMatrix|TestRawIntSelfABI_ExitResumeFallbackKeepsCallerLiveValues|TestExitResumeCheck_RawIntSelfCallFallbackFrame|TestTier2_StringFormatLookupPreservesPositiveDivisorModuloSemantics|TestTier2_StringFormatIntLoweringCoversGenericSingleIntPatterns|TestTier2_StringFormatIntMinInt64FallsBackPrecisely|TestTier2_StringFormatIntReboundCalleeFallsBackPrecisely|TestTier2_StringFormatIntFeedbackDynamicPatternGuardsPattern|TestTier2_StringFormatConstMultiArgUsesPreciseOpExit' -count=1 "$@"
      ;;
    race-smoke)
      go test -race ./internal/runtime ./internal/nanbox ./internal/vm ./llm ./tests/sdk ./tests/llm ./cmd/leia -count=1 "$@"
      ;;
    concurrency-contract)
      go test -race ./tests -run TestGoStyleConcurrencyContract -count=1 "$@"
      ;;
    *)
      echo "scripts/run.sh test: unknown profile: $profile" >&2
      exit 2
      ;;
  esac
}

case "$task" in
  -h|--help|help)
    usage
    ;;
  arch|arch-check)
    run_shell_task scripts/arch_check.sh "$@"
    ;;
  diag)
    run_shell_task scripts/diag.sh "$@"
    ;;
  diagnostics|diagnostics-bundle)
    run_shell_task scripts/diagnostics_bundle.sh "$@"
    ;;
  docs|docs-check)
    run_shell_task scripts/docs_check.sh "$@"
    ;;
  editor|editor-check)
    run_shell_task scripts/editor_check.sh "$@"
    ;;
  language-conformance|language-conformance-check)
    run_language_conformance_task "$@"
    ;;
  manifest|manifest-check|manifest-coverage)
    run_manifest_check_task "$@"
    ;;
  manifest-list-q|q-manifest-list)
    run_manifest_list_q_task "$@"
    ;;
  perf|performance|performance-gate)
    run_shell_task scripts/performance_gate.sh "$@"
    ;;
  production|production-check)
    run_shell_task scripts/production_check.sh "$@"
    ;;
  module-path|module-path-check)
    run_module_path_task "$@"
    ;;
  public-blockers|public-release-blockers|public-release-blockers-check)
    run_shell_task scripts/public_release_blockers_check.sh "$@"
    ;;
  q|q-conformance)
    run_shell_task scripts/q_conformance_gate.sh "$@"
    ;;
  q-perf|q-performance|q-performance-gate)
    run_q_perf_task "$@"
    ;;
  release-artifacts)
    run_shell_task scripts/release_artifacts.sh "$@"
    ;;
  release-artifacts-gate|release-gate-artifacts)
    run_release_artifacts_gate_task "$@"
    ;;
  release-check|release-artifacts-check)
    run_shell_task scripts/release_artifacts_check.sh "$@"
    ;;
  release-dist|release-distribution|release-distribution-check)
    run_shell_task scripts/release_distribution_check.sh "$@"
    ;;
  release-notes|release-notes-check)
    run_shell_task scripts/release_notes_check.sh "$@"
    ;;
  release-notes-gate)
    run_release_notes_gate_task "$@"
    ;;
  release-smoke)
    run_release_smoke_task "$@"
    ;;
  release-snapshot|release-snapshot-install|release-snapshot-install-check)
    run_shell_task scripts/release_snapshot_install_check.sh "$@"
    ;;
  site|site-check)
    run_shell_task scripts/site_check.sh "$@"
    ;;
  shell-syntax|shell-syntax-check)
    run_shell_syntax_task "$@"
    ;;
  test|tests)
    run_test_task "$@"
    ;;
  cli-experience)
    run_cli_experience_task "$@"
    ;;
  worktree|worktree-audit)
    run_shell_task scripts/worktree_audit.sh "$@"
    ;;
  *.leia|scripts/*.leia)
    run_leia_task "$task" "$@"
    ;;
  *)
    echo "scripts/run.sh: unknown task: $task" >&2
    usage >&2
    exit 2
    ;;
esac
