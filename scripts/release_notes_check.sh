#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd "$script_dir/.." && pwd -P)"
version=""
require_ready="false"

usage() {
  cat <<'USAGE'
Usage: scripts/release_notes_check.sh [--version VERSION] [--require-ready] [--help]

Checks release notes evidence.

Default mode validates the reusable release notes template. With --version,
the script checks docs/release/notes/VERSION.md. With --require-ready, missing
or placeholder release notes fail the command.

Options:
  --version VERSION   Release tag such as v0.1.0
  --require-ready     Fail when VERSION notes are missing or still templated
  -h, --help          Show this help
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      if [[ $# -lt 2 || -z "$2" ]]; then
        echo "error: --version requires a value" >&2
        usage >&2
        exit 2
      fi
      version="$2"
      shift 2
      ;;
    --require-ready)
      require_ready="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

cd "$repo_root"

failures=()

add_failure() {
  failures+=("$1")
}

require_contains() {
  local file="$1"
  local text="$2"
  if ! grep -Fq -- "$text" "$file"; then
    add_failure "$file missing required text: $text"
  fi
}

check_template() {
  local template="docs/release/notes-template.md"
  if [[ ! -f "$template" ]]; then
    add_failure "missing $template"
    return
  fi
  require_contains "$template" "bash scripts/release_artifacts_check.sh --build --require-clean --require-tag --version vX.Y.Z"
  require_contains "$template" "List known issues, or write \`None known\` after release validation."
  require_contains "$template" "## Checksums And Artifacts"
  require_contains "$template" "## Release Decisions"
}

check_version() {
  if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
    add_failure "release notes version must match vMAJOR.MINOR.PATCH or prerelease: $version"
    return
  fi

  local notes="docs/release/notes/$version.md"
  if [[ ! -f "$notes" ]]; then
    add_failure "missing release notes for $version: $notes"
    return
  fi

  for heading in "## Validation" "## Known Issues" "## Checksums And Artifacts" "## Release Decisions"; do
    require_contains "$notes" "$heading"
  done
  require_contains "$notes" "$version"
  require_contains "$notes" "bash scripts/release_artifacts_check.sh --build --require-clean --require-tag --version $version"

  for placeholder in \
    "vX.Y.Z" \
    "| | |" \
    "List known issues, or write" \
    "- License:" \
    "- Security reporting:" \
    "TODO" \
    "TBD"; do
    if grep -Fq -- "$placeholder" "$notes"; then
      add_failure "$notes still contains template placeholder: $placeholder"
    fi
  done
}

check_template
if [[ -n "$version" ]]; then
  check_version
fi

if [[ ${#failures[@]} -eq 0 ]]; then
  if [[ -n "$version" ]]; then
    echo "release_notes_check.sh: pass ($version)"
  else
    echo "release_notes_check.sh: pass"
  fi
  exit 0
fi

echo "release_notes_check.sh: release notes issues:"
for failure in "${failures[@]}"; do
  echo "  - $failure"
done

if [[ "$require_ready" == "true" ]]; then
  exit 1
fi

echo "release_notes_check.sh: audit mode; use --require-ready to fail"
