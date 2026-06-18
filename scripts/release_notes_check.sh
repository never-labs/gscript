#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd "$script_dir/.." && pwd -P)"
version=""
require_ready="false"
json_out="false"

usage() {
  cat <<'USAGE'
Usage: scripts/release_notes_check.sh [--version VERSION] [--require-ready] [--json] [--help]

Checks release notes evidence.

Default mode validates the reusable release notes template. With --version,
the script checks docs/release/notes/VERSION.md. With --require-ready, missing
or placeholder release notes fail the command.

Options:
  --version VERSION   Release tag such as v0.1.0
  --require-ready     Fail when VERSION notes are missing or still templated
  --json              Print a machine-readable release notes report
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

if [[ -n "$version" && ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "error: release notes version must match vMAJOR.MINOR.PATCH or prerelease: $version" >&2
  exit 2
fi

failures=()
failure_kinds=()
failure_paths=()
failure_values=()
failure_kind_values=()
checked_files=()
checked_file_roles=()
required_artifact_count=0
artifact_checksum_count=0
required_artifact_paths=()
required_artifact_names=()
required_artifact_checksum_present=()

add_failure() {
  failures+=("$1")
  failure_kinds+=("${2:-general}")
  failure_paths+=("${3:-}")
  failure_values+=("${4:-}")
  record_failure_kind "${2:-general}"
}

add_checked_file() {
  checked_files+=("$1")
  checked_file_roles+=("${2:-checked}")
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

record_failure_kind() {
  local kind="$1"
  local existing
  if [[ "${#failure_kind_values[@]}" -gt 0 ]]; then
    for existing in "${failure_kind_values[@]}"; do
      if [[ "$existing" == "$kind" ]]; then
        return
      fi
    done
  fi
  failure_kind_values+=("$kind")
}

print_json_string_array() {
  local indent="$1"
  shift
  local values=("$@")
  printf '[\n'
  local i=0
  while [[ "$i" -lt "${#values[@]}" ]]; do
    printf '%s  "%s"' "$indent" "$(json_escape "${values[$i]}")"
    if [[ "$i" -lt $((${#values[@]} - 1)) ]]; then
      printf ','
    fi
    printf '\n'
    i=$((i + 1))
  done
  printf '%s]' "$indent"
}

print_json_checked_file_details() {
  local indent="$1"
  printf '[\n'
  local i=0
  while [[ "$i" -lt "${#checked_files[@]}" ]]; do
    local path="${checked_files[$i]}"
    local role="${checked_file_roles[$i]}"
    local exists="false"
    if [[ -f "$path" ]]; then
      exists="true"
    fi
    printf '%s  {"path": "%s", "role": "%s", "required": true, "exists": %s}' "$indent" "$(json_escape "$path")" "$(json_escape "$role")" "$exists"
    if [[ "$i" -lt $((${#checked_files[@]} - 1)) ]]; then
      printf ','
    fi
    printf '\n'
    i=$((i + 1))
  done
  printf '%s]' "$indent"
}

print_json_required_artifact_details() {
  local indent="$1"
  printf '[\n'
  local i=0
  while [[ "$i" -lt "${#required_artifact_names[@]}" ]]; do
    printf '%s  {"path": "%s", "artifact": "%s", "checksum_present": %s}' "$indent" "$(json_escape "${required_artifact_paths[$i]}")" "$(json_escape "${required_artifact_names[$i]}")" "${required_artifact_checksum_present[$i]}"
    if [[ "$i" -lt $((${#required_artifact_names[@]} - 1)) ]]; then
      printf ','
    fi
    printf '\n'
    i=$((i + 1))
  done
  printf '%s]' "$indent"
}

print_json_report() {
  local status="pass"
  if [[ ${#failures[@]} -gt 0 ]]; then
    status="issues"
  fi
  printf '{\n'
  printf '  "schema_version": 1,\n'
  printf '  "status": "%s",\n' "$status"
  printf '  "require_ready": %s,\n' "$require_ready"
  printf '  "version": "%s",\n' "$(json_escape "$version")"
  printf '  "checked_file_count": %d,\n' "${#checked_files[@]}"
  printf '  "required_artifact_count": %d,\n' "$required_artifact_count"
  printf '  "artifact_checksum_count": %d,\n' "$artifact_checksum_count"
  printf '  "failure_kind_count": %d,\n' "${#failure_kind_values[@]}"
  printf '  "checked_files": '
  print_json_string_array "  " "${checked_files[@]}"
  printf ',\n'
  printf '  "checked_file_details": '
  print_json_checked_file_details "  "
  printf ',\n'
  printf '  "required_artifact_details": '
  print_json_required_artifact_details "  "
  printf ',\n'
  printf '  "failure_count": %d,\n' "${#failures[@]}"
  printf '  "failure_kinds": '
  print_json_string_array "  " ${failure_kind_values[@]+"${failure_kind_values[@]}"}
  printf ',\n'
  printf '  "failures": [\n'
  local i=0
  while [[ "$i" -lt ${#failures[@]} ]]; do
    printf '    "%s"' "$(json_escape "${failures[$i]}")"
    if [[ "$i" -lt $((${#failures[@]} - 1)) ]]; then
      printf ','
    fi
    printf '\n'
    i=$((i + 1))
  done
  printf '  ],\n'
  printf '  "failure_details": [\n'
  i=0
  while [[ "$i" -lt ${#failures[@]} ]]; do
    printf '    {\n'
    printf '      "message": "%s",\n' "$(json_escape "${failures[$i]}")"
    printf '      "kind": "%s"' "$(json_escape "${failure_kinds[$i]}")"
    if [[ -n "${failure_paths[$i]}" ]]; then
      printf ',\n      "path": "%s"' "$(json_escape "${failure_paths[$i]}")"
    fi
    if [[ -n "${failure_values[$i]}" ]]; then
      printf ',\n      "value": "%s"' "$(json_escape "${failure_values[$i]}")"
    fi
    printf '\n    }'
    if [[ "$i" -lt $((${#failures[@]} - 1)) ]]; then
      printf ','
    fi
    printf '\n'
    i=$((i + 1))
  done
  printf '  ]\n'
  printf '}\n'
}

require_contains() {
  local file="$1"
  local text="$2"
  if ! grep -Fq -- "$text" "$file"; then
    add_failure "$file missing required text: $text" "missing_text" "$file" "$text"
  fi
}

require_not_matching() {
  local file="$1"
  local pattern="$2"
  local description="$3"
  if grep -Eq -- "$pattern" "$file"; then
    add_failure "$file still contains template placeholder: $description" "placeholder" "$file" "$description"
  fi
}

release_archive_names() {
  local archive_version="$1"
  printf 'leia_%s_darwin_amd64.tar.gz\n' "$archive_version"
  printf 'leia_%s_darwin_arm64.tar.gz\n' "$archive_version"
  printf 'leia_%s_linux_amd64.tar.gz\n' "$archive_version"
  printf 'leia_%s_linux_arm64.tar.gz\n' "$archive_version"
  printf 'leia_%s_windows_amd64.zip\n' "$archive_version"
  printf 'leia_%s_windows_arm64.zip\n' "$archive_version"
}

require_release_archive_names() {
  local file="$1"
  local archive_version="$2"
  local archive
  while IFS= read -r archive; do
    require_contains "$file" "$archive"
  done < <(release_archive_names "$archive_version")
}

require_archive_checksum() {
  local file="$1"
  local artifact="$2"
  required_artifact_count=$((required_artifact_count + 1))
  required_artifact_paths+=("$file")
  required_artifact_names+=("$artifact")
  if grep -E "\\|[[:space:]]*\`?${artifact}\`?[[:space:]]*\\|[[:space:]]*[[:xdigit:]]{64}[[:space:]]*\\|" "$file" >/dev/null; then
    artifact_checksum_count=$((artifact_checksum_count + 1))
    required_artifact_checksum_present+=("true")
  else
    required_artifact_checksum_present+=("false")
    add_failure "$file must include a 64-hex SHA256 checksum for $artifact" "missing_checksum" "$file" "$artifact"
  fi
}

require_release_archive_checksums() {
  local file="$1"
  local archive_version="$2"
  local archive
  while IFS= read -r archive; do
    require_archive_checksum "$file" "$archive"
  done < <(release_archive_names "$archive_version")
  require_archive_checksum "$file" "SHA256SUMS"
}

check_template() {
  local template="docs/release/notes-template.md"
  add_checked_file "$template" "template"
  if [[ ! -f "$template" ]]; then
    add_failure "missing $template" "missing_file" "$template"
    return
  fi
  require_contains "$template" "bash scripts/release_artifacts_check.sh --build --require-clean --require-tag --version vX.Y.Z"
  require_contains "$template" "List known issues, or write \`None known\` after release validation."
  require_contains "$template" "## Checksums And Artifacts"
  require_contains "$template" "## Release Decisions"
  require_release_archive_names "$template" "vX.Y.Z"
  require_contains "$template" "Each archive includes \`leia\` and \`leia-lsp\`."
}

check_version() {
  local notes="docs/release/notes/$version.md"
  add_checked_file "$notes" "version_notes"
  if [[ ! -f "$notes" ]]; then
    add_failure "missing release notes for $version: $notes" "missing_file" "$notes" "$version"
    return
  fi

  for heading in "## Validation" "## Known Issues" "## Checksums And Artifacts" "## Release Decisions"; do
    require_contains "$notes" "$heading"
  done
  require_contains "$notes" "$version"
  require_contains "$notes" "bash scripts/release_artifacts_check.sh --build --require-clean --require-tag --version $version"
  require_contains "$notes" "leia-lsp"
  require_release_archive_names "$notes" "$version"

  if ! grep -Eq '[[:xdigit:]]{64}' "$notes"; then
    add_failure "$notes must include at least one 64-hex SHA256 checksum" "missing_checksum" "$notes"
  fi
  require_release_archive_checksums "$notes" "$version"

  for placeholder in \
    "vX.Y.Z" \
    "| | |" \
    "List known issues, or write" \
    "TODO" \
    "TBD"; do
    if grep -Fq -- "$placeholder" "$notes"; then
      add_failure "$notes still contains template placeholder: $placeholder" "placeholder" "$notes" "$placeholder"
    fi
  done
  require_not_matching "$notes" '^[[:space:]]*-[[:space:]]*License:[[:space:]]*$' "- License:"
  require_not_matching "$notes" '^[[:space:]]*-[[:space:]]*Security reporting:[[:space:]]*$' "- Security reporting:"
  require_not_matching "$notes" '^[[:space:]]*-[[:space:]]*Platform support:[[:space:]]*$' "- Platform support:"
  require_not_matching "$notes" '^[[:space:]]*-[[:space:]]*Release channels:[[:space:]]*$' "- Release channels:"
  require_not_matching "$notes" '^[[:space:]]*-[[:space:]]*Artifact signing:[[:space:]]*$' "- Artifact signing:"
  require_not_matching "$notes" '^[[:space:]]*-[[:space:]]*Compatibility policy:[[:space:]]*$' "- Compatibility policy:"
}

check_template
if [[ -n "$version" ]]; then
  check_version
fi

if [[ ${#failures[@]} -eq 0 ]]; then
  if [[ "$json_out" == "true" ]]; then
    print_json_report
    exit 0
  fi
  if [[ -n "$version" ]]; then
    echo "release_notes_check.sh: pass ($version)"
  else
    echo "release_notes_check.sh: pass"
  fi
  exit 0
fi

if [[ "$json_out" == "true" ]]; then
  print_json_report
  if [[ "$require_ready" == "true" ]]; then
    exit 1
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
