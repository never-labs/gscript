#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd "$script_dir/.." && pwd -P)"
require_resolved="false"
json_out="false"

usage() {
  cat <<'USAGE'
Usage: scripts/public_release_blockers_check.sh [--require-resolved] [--json] [--help]

Audits repository-level public release blockers that require maintainer
decisions. Default mode reports unresolved blockers and exits successfully.
With --require-resolved, unresolved blockers fail the command.

Options:
  --require-resolved  Fail when public release decisions are still unresolved
  --json              Print a machine-readable blocker report
  -h, --help          Show this help
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --require-resolved)
      require_resolved="true"
      shift
      ;;
    --json)
      json_out="true"
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

json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '%s' "$value"
}

print_json_report() {
  local status="pass"
  if [[ ${#blockers[@]} -gt 0 ]]; then
    status="blocked"
  fi
  printf '{\n'
  printf '  "schema_version": 1,\n'
  printf '  "status": "%s",\n' "$status"
  printf '  "require_resolved": %s,\n' "$require_resolved"
  printf '  "blocker_count": %d,\n' "${#blockers[@]}"
  printf '  "blockers": [\n'
  local i=0
  while [[ "$i" -lt ${#blockers[@]} ]]; do
    printf '    "%s"' "$(json_escape "${blockers[$i]}")"
    if [[ "$i" -lt $((${#blockers[@]} - 1)) ]]; then
      printf ','
    fi
    printf '\n'
    i=$((i + 1))
  done
  printf '  ]\n'
  printf '}\n'
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
    [[ "$line" == \|* ]] || continue
    area="$(awk -F'|' '{ gsub(/^[ \t]+|[ \t]+$/, "", $2); print $2 }' <<<"$line")"
    decision_needed="$(awk -F'|' '{ gsub(/^[ \t]+|[ \t]+$/, "", $3); print $3 }' <<<"$line")"
    status="$(awk -F'|' '{ gsub(/^[ \t]+|[ \t]+$/, "", $4); print $4 }' <<<"$line")"
    [[ -n "$area" && -n "$status" ]] || continue
    [[ "$area" != "Area" && "$area" != "---" ]] || continue
    if ! grep -Eq '^(Resolved|Accepted|N/A)([:.]|$)' <<<"$status"; then
      if [[ -n "$decision_needed" && "$decision_needed" != "---" ]]; then
        add_blocker "unresolved release decision: $area: $decision_needed ($status)"
      else
        add_blocker "unresolved release decision: $area ($status)"
      fi
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

if [[ "$json_out" == "true" ]]; then
  print_json_report
  if [[ ${#blockers[@]} -gt 0 && "$require_resolved" == "true" ]]; then
    exit 1
  fi
  exit 0
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
