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
blocker_kinds=()
blocker_areas=()
blocker_actions=()
blocker_statuses=()
blocker_paths=()

add_blocker() {
  blockers+=("$1")
  blocker_kinds+=("${2:-general}")
  blocker_areas+=("${3:-}")
  blocker_actions+=("${4:-}")
  blocker_statuses+=("${5:-}")
  blocker_paths+=("${6:-}")
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

count_blocker_kind() {
  local kind="$1"
  local count=0
  local i=0
  while [[ "$i" -lt ${#blocker_kinds[@]} ]]; do
    if [[ "${blocker_kinds[$i]}" == "$kind" ]]; then
      count=$((count + 1))
    fi
    i=$((i + 1))
  done
  printf '%d' "$count"
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
  printf '  "missing_file_count": %d,\n' "$(count_blocker_kind "missing_file")"
  printf '  "release_decision_count": %d,\n' "$(count_blocker_kind "release_decision")"
  printf '  "stale_text_count": %d,\n' "$(count_blocker_kind "stale_text")"
  printf '  "unconfirmed_policy_count": %d,\n' "$(count_blocker_kind "unconfirmed_policy")"
  printf '  "missing_guidance_count": %d,\n' "$(count_blocker_kind "missing_guidance")"
  printf '  "missing_doc_snippet_count": %d,\n' "$(count_blocker_kind "missing_doc_snippet")"
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
  printf '  ],\n'
  printf '  "blocker_details": [\n'
  i=0
  while [[ "$i" -lt ${#blockers[@]} ]]; do
    printf '    {\n'
    printf '      "message": "%s",\n' "$(json_escape "${blockers[$i]}")"
    printf '      "kind": "%s"' "$(json_escape "${blocker_kinds[$i]}")"
    if [[ -n "${blocker_areas[$i]}" ]]; then
      printf ',\n      "area": "%s"' "$(json_escape "${blocker_areas[$i]}")"
    fi
    if [[ -n "${blocker_actions[$i]}" ]]; then
      printf ',\n      "action": "%s"' "$(json_escape "${blocker_actions[$i]}")"
    fi
    if [[ -n "${blocker_statuses[$i]}" ]]; then
      printf ',\n      "decision_status": "%s"' "$(json_escape "${blocker_statuses[$i]}")"
    fi
    if [[ -n "${blocker_paths[$i]}" ]]; then
      printf ',\n      "path": "%s"' "$(json_escape "${blocker_paths[$i]}")"
    fi
    printf '\n    }'
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
    add_blocker "missing required public release file: $file" "missing_file" "Release files" "Add required public release file" "Open" "$file"
    return 1
  fi
}

if [[ ! -f LICENSE ]]; then
  add_blocker "missing root LICENSE file" "missing_file" "License" "Choose the repository license and add a root LICENSE file" "Open" "LICENSE"
fi

if [[ -f README.md ]] && grep -Fq "No license has been selected in this repository yet." README.md; then
  add_blocker "README.md still declares that no license has been selected" "stale_text" "License" "Remove no-license placeholder after selecting a license" "Open" "README.md"
fi

require_file SECURITY.md || true
if [[ -f SECURITY.md ]]; then
  if grep -Fq "when available" SECURITY.md; then
    add_blocker "SECURITY.md still uses an unconfirmed reporting route" "unconfirmed_policy" "Security reporting" "Confirm the private vulnerability reporting route" "Open" "SECURITY.md"
  fi
  if ! grep -Fq "Do not file public issues for vulnerabilities" SECURITY.md; then
    add_blocker "SECURITY.md must keep private-reporting guidance" "missing_guidance" "Security reporting" "Keep private-reporting guidance in SECURITY.md" "Open" "SECURITY.md"
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
        add_blocker "unresolved release decision: $area: $decision_needed ($status)" "release_decision" "$area" "$decision_needed" "$status" "docs/release/decisions.md"
      else
        add_blocker "unresolved release decision: $area ($status)" "release_decision" "$area" "" "$status" "docs/release/decisions.md"
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
      add_blocker "docs/release/index.md missing public blocker snippet: $snippet" "missing_doc_snippet" "Release documentation" "Restore required public blocker snippet" "Open" "docs/release/index.md"
    fi
  done
fi

require_file docs/reference/platforms/index.md || true
if [[ -f docs/reference/platforms/index.md ]]; then
  for level in Tested Supported Available Unknown; do
    if ! grep -Fq "| $level |" docs/reference/platforms/index.md; then
      add_blocker "docs/reference/platforms/index.md missing support level: $level" "missing_doc_snippet" "Platform support" "Document support level $level" "Open" "docs/reference/platforms/index.md"
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
