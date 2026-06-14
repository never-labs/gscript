#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd "$script_dir/.." && pwd -P)"
require_resolved="false"

usage() {
  cat <<'USAGE'
Usage: scripts/public_release_blockers_check.sh [--require-resolved] [--help]

Audits repository-level public release blockers that require maintainer
decisions. Default mode reports unresolved blockers and exits successfully.
With --require-resolved, unresolved blockers fail the command.

Options:
  --require-resolved  Fail when public release decisions are still unresolved
  -h, --help          Show this help
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --require-resolved)
      require_resolved="true"
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

blockers=()

add_blocker() {
  blockers+=("$1")
}

require_file() {
  local file="$1"
  if [[ ! -f "$file" ]]; then
    add_blocker "missing required public release file: $file"
    return 1
  fi
}

if [[ ! -f LICENSE ]]; then
  add_blocker "missing root LICENSE file"
fi

if [[ -f README.md ]] && grep -Fq "No license has been selected in this repository yet." README.md; then
  add_blocker "README.md still declares that no license has been selected"
fi

require_file SECURITY.md || true
if [[ -f SECURITY.md ]]; then
  if grep -Fq "when available" SECURITY.md; then
    add_blocker "SECURITY.md still uses an unconfirmed reporting route"
  fi
  if ! grep -Fq "Do not file public issues for vulnerabilities" SECURITY.md; then
    add_blocker "SECURITY.md must keep private-reporting guidance"
  fi
fi

require_file docs/release/decisions.md || true
if [[ -f docs/release/decisions.md ]]; then
  while IFS= read -r line; do
    if [[ "$line" == \|* ]] && grep -Eq '\|[[:space:]]*Open\.[[:space:]]*\|' <<<"$line"; then
      area="$(awk -F'|' '{ gsub(/^[ \t]+|[ \t]+$/, "", $2); print $2 }' <<<"$line")"
      [[ -n "$area" ]] || area="release decision"
      add_blocker "open release decision: $area"
    fi
  done < docs/release/decisions.md
fi

require_file docs/release/index.md || true
if [[ -f docs/release/index.md ]]; then
  for snippet in \
    "choose a license and add a root \`LICENSE\` file" \
    "confirm the vulnerability reporting route in \`SECURITY.md\`" \
    "complete the release decisions recorded in \`docs/release/decisions.md\`" \
    "state tested platforms and execution modes" \
    "SHA256 checksums"; do
    if ! grep -Fq "$snippet" docs/release/index.md; then
      add_blocker "docs/release/index.md missing public blocker snippet: $snippet"
    fi
  done
fi

require_file docs/reference/platforms/index.md || true
if [[ -f docs/reference/platforms/index.md ]]; then
  for level in Tested Supported Available Unknown; do
    if ! grep -Fq "| $level |" docs/reference/platforms/index.md; then
      add_blocker "docs/reference/platforms/index.md missing support level: $level"
    fi
  done
fi

if [[ ${#blockers[@]} -eq 0 ]]; then
  echo "public_release_blockers_check.sh: pass"
  exit 0
fi

echo "public_release_blockers_check.sh: unresolved public release blockers:"
for blocker in "${blockers[@]}"; do
  echo "  - $blocker"
done

if [[ "$require_resolved" == "true" ]]; then
  exit 1
fi

echo "public_release_blockers_check.sh: audit mode; use --require-resolved to fail"
