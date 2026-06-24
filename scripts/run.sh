#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/run.sh <task> [args...]

Single repository script launcher.

Tasks:
  arch                 Run architecture health scan
  diag                 Run JIT diagnostic dump
  diagnostics          Collect diagnostics bundle
  docs                 Run documentation checks
  editor               Run editor asset checks
  perf                 Run performance gate
  production           Run production readiness gate
  public-blockers      Check public release blocker decisions
  q                    Run q conformance gate
  release-artifacts    Build local release artifacts
  release-check        Check local release artifacts
  release-dist         Check release distribution config
  release-notes        Check release notes evidence
  release-snapshot     Verify a snapshot archive through the installer
  site                 Check rendered static site output
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
  perf|performance|performance-gate)
    run_shell_task scripts/performance_gate.sh "$@"
    ;;
  production|production-check)
    run_shell_task scripts/production_check.sh "$@"
    ;;
  public-blockers|public-release-blockers|public-release-blockers-check)
    run_shell_task scripts/public_release_blockers_check.sh "$@"
    ;;
  q|q-conformance)
    run_shell_task scripts/q_conformance_gate.sh "$@"
    ;;
  release-artifacts)
    run_shell_task scripts/release_artifacts.sh "$@"
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
  release-snapshot|release-snapshot-install|release-snapshot-install-check)
    run_shell_task scripts/release_snapshot_install_check.sh "$@"
    ;;
  site|site-check)
    run_shell_task scripts/site_check.sh "$@"
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
