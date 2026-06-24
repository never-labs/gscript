#!/usr/bin/env bash
# Smoke-check release artifact planning, and optionally build into a temp dir.

set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/release_artifacts_check.sh [--build] [--output-dir DIR] [--version VERSION] [--keep-output] [--require-clean] [--require-tag] [--json]

Checks scripts/release_artifacts.sh without changing its implementation.

Default mode runs a dry-run and verifies the planned artifact names and metadata.
With --build, the script builds into a temporary output directory unless
--output-dir is provided, then verifies SHA256SUMS, runs the built CLI against
tests/smoke/01_basic.leia, checks that the LSP binary starts in help mode, and
verifies scripts/install.sh from a local tar.gz/zip release fixture.

Options:
      --build           Run a real local build after the dry-run check
  -o, --output-dir DIR  Output directory for --build mode; defaults to mktemp
      --version VERSION Version to pass to release_artifacts.sh
      --keep-output     Do not remove an auto-created temp output directory
      --require-clean   Fail unless the git worktree has no tracked or untracked changes
      --require-tag     Fail unless HEAD is exactly tagged with VERSION
      --json            Print a machine-readable artifact check report
  -h, --help            Show this help
USAGE
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd "$script_dir/.." && pwd -P)"
release_script="$repo_root/scripts/release_artifacts.sh"
install_script="$repo_root/scripts/install.sh"
smoke_script="tests/smoke/01_basic.leia"
expected_module_path="github.com/never-labs/leia"

build="false"
out_dir=""
version="v0.0.0-local"
keep_output="false"
require_clean="false"
require_tag="false"
json_out="false"
dry_run_verified="false"
build_verified="false"
install_archive_verified="false"
checksum_entry_count=0
install_archive_checksum_count=0
goos=""
goarch=""
binary_name=""
lsp_binary_name=""
metadata_name=""
install_archive_name=""
artifact_files=()
failure_kinds=()
failure_messages=()
failure_printed="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --build)
      build="true"
      shift
      ;;
    -o|--output-dir)
      if [[ $# -lt 2 || -z "$2" ]]; then
        echo "error: $1 requires a directory" >&2
        exit 2
      fi
      out_dir="$2"
      shift 2
      ;;
    --version)
      if [[ $# -lt 2 || -z "$2" ]]; then
        echo "error: $1 requires a non-empty version" >&2
        exit 2
      fi
      version="$2"
      shift 2
      ;;
    --keep-output)
      keep_output="true"
      shift
      ;;
    --require-clean)
      require_clean="true"
      shift
      ;;
    --require-tag)
      require_tag="true"
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

log_info() {
  if [[ "$json_out" != "true" ]]; then
    echo "$1"
  fi
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

record_failure() {
  local kind="$1"
  local message="$2"
  failure_kinds+=("$kind")
  failure_messages+=("$message")
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

print_json_failure_details() {
  local indent="$1"
  printf '[\n'
  local i=0
  while [[ "$i" -lt "${#failure_messages[@]}" ]]; do
    printf '%s  {"kind": "%s", "message": "%s"}' "$indent" "$(json_escape "${failure_kinds[$i]}")" "$(json_escape "${failure_messages[$i]}")"
    if [[ "$i" -lt $((${#failure_messages[@]} - 1)) ]]; then
      printf ','
    fi
    printf '\n'
    i=$((i + 1))
  done
  printf '%s]' "$indent"
}

print_json_artifact_entries() {
  local indent="$1"
  local report_out_dir="$2"
  local roles=("cli" "lsp" "metadata" "checksums")
  local names=("$binary_name" "$lsp_binary_name" "$metadata_name" "SHA256SUMS")
  printf '[\n'
  local i=0
  while [[ "$i" -lt "${#names[@]}" ]]; do
    printf '%s  {"role": "%s", "name": "%s"' "$indent" "$(json_escape "${roles[$i]}")" "$(json_escape "${names[$i]}")"
    if [[ -n "$report_out_dir" ]]; then
      printf ', "path": "%s/%s"' "$(json_escape "$report_out_dir")" "$(json_escape "${names[$i]}")"
    fi
    printf '}'
    if [[ "$i" -lt $((${#names[@]} - 1)) ]]; then
      printf ','
    fi
    printf '\n'
    i=$((i + 1))
  done
  printf '%s]' "$indent"
}

print_json_report() {
  local status="${1:-pass}"
  local report_out_dir="$out_dir"
  local artifact_file_count="${#artifact_files[@]}"
  local failure_kind_count="${#failure_kinds[@]}"
  if [[ "$build" != "true" ]]; then
    report_out_dir=""
  fi
  printf '{\n'
  printf '  "schema_version": 1,\n'
  printf '  "status": "%s",\n' "$(json_escape "$status")"
  printf '  "version": "%s",\n' "$(json_escape "$version")"
  printf '  "build": %s,\n' "$build"
  printf '  "require_clean": %s,\n' "$require_clean"
  printf '  "require_tag": %s,\n' "$require_tag"
  printf '  "goos": "%s",\n' "$(json_escape "$goos")"
  printf '  "goarch": "%s",\n' "$(json_escape "$goarch")"
  printf '  "artifact": "%s",\n' "$(json_escape "$binary_name")"
  printf '  "lsp_artifact": "%s",\n' "$(json_escape "$lsp_binary_name")"
  printf '  "metadata": "%s",\n' "$(json_escape "$metadata_name")"
  printf '  "install_archive": "%s",\n' "$(json_escape "$install_archive_name")"
  printf '  "artifact_count": %d,\n' "$artifact_file_count"
  printf '  "checksum_entry_count": %d,\n' "$checksum_entry_count"
  printf '  "install_archive_checksum_count": %d,\n' "$install_archive_checksum_count"
  printf '  "failure_kind_count": %d,\n' "$failure_kind_count"
  printf '  "failure_kinds": '
  if [[ "$failure_kind_count" -eq 0 ]]; then
    print_json_string_array "  "
  else
    print_json_string_array "  " "${failure_kinds[@]}"
  fi
  printf ',\n'
  printf '  "failure_count": %d,\n' "${#failure_messages[@]}"
  printf '  "failure_details": '
  print_json_failure_details "  "
  printf ',\n'
  printf '  "artifact_files": '
  if [[ "$artifact_file_count" -eq 0 ]]; then
    print_json_string_array "  "
  else
    print_json_string_array "  " "${artifact_files[@]}"
  fi
  printf ',\n'
  printf '  "artifact_entries": '
  print_json_artifact_entries "  " "$report_out_dir"
  printf ',\n'
  printf '  "dry_run_verified": %s,\n' "$dry_run_verified"
  printf '  "build_verified": %s,\n' "$build_verified"
  printf '  "install_archive_verified": %s,\n' "$install_archive_verified"
  printf '  "output_dir": "%s"\n' "$(json_escape "$report_out_dir")"
  printf '}\n'
}

fail() {
  local kind="$1"
  local message="$2"
  local code="${3:-1}"
  record_failure "$kind" "$message"
  if [[ "$json_out" == "true" ]]; then
    failure_printed="true"
    print_json_report "fail"
  else
    echo "error: $message" >&2
  fi
  exit "$code"
}

on_error() {
  local code="$1"
  if [[ "$json_out" == "true" && "$failure_printed" != "true" ]]; then
    record_failure "command_failed" "command failed: ${BASH_COMMAND}"
    failure_printed="true"
    print_json_report "fail"
  fi
  exit "$code"
}

trap 'on_error "$?"' ERR

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "missing_command" "required command not found: $1"
  fi
}

require_file() {
  if [[ ! -f "$1" ]]; then
    fail "missing_file" "missing required file: $1"
  fi
}

require_contains() {
  local file="$1"
  local pattern="$2"
  if ! grep -Fq "$pattern" "$file"; then
    fail "missing_text" "$file does not contain expected text: $pattern"
  fi
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 -r "$1" | awk '{print $1}'
  else
    fail "missing_command" "need sha256sum, shasum, or openssl for checksum verification"
  fi
}

verify_checksums() {
  local dir="$1"
  local sums="$dir/SHA256SUMS"
  local checked=0

  require_file "$sums"

  while read -r expected name extra; do
    if [[ -z "${expected:-}" ]]; then
      continue
    fi
    if [[ -n "${extra:-}" ]]; then
      fail "checksum_format" "malformed checksum line in $sums: $expected $name $extra"
    fi
    if [[ ! "$expected" =~ ^[0-9a-fA-F]{64}$ ]]; then
      fail "checksum_format" "malformed sha256 digest in $sums: $expected"
    fi
    if [[ "$name" == */* || "$name" == "." || "$name" == ".." ]]; then
      fail "checksum_format" "unsafe checksum filename in $sums: $name"
    fi
    require_file "$dir/$name"
    actual="$(sha256_file "$dir/$name")"
    if [[ "$actual" != "$expected" ]]; then
      fail "checksum_mismatch" "checksum mismatch for $name"
    fi
    checked=$((checked + 1))
  done < "$sums"

  if [[ "$checked" -ne 3 ]]; then
    fail "checksum_count" "expected 3 checksum entries, found $checked"
  fi
  checksum_entry_count="$checked"
}

write_checksum_line() {
  local dir="$1"
  local name="$2"
  (
    cd "$dir"
    printf '%s  %s\n' "$(sha256_file "$name")" "$name"
  )
}

package_install_archive() {
  local release_dir="$1"
  local archive_name="$2"
  local source_binary="$3"
  local source_lsp_binary="$4"
  local archive_ext="$5"
  local package_dir="$release_dir/package"

  rm -rf "$package_dir"
  mkdir -p "$package_dir"
  cp "$source_binary" "$package_dir/$install_binary_name"
  cp "$source_lsp_binary" "$package_dir/$lsp_install_binary_name"
  chmod 0755 "$package_dir/$install_binary_name" "$package_dir/$lsp_install_binary_name"

  if [[ "$archive_ext" == "tar.gz" ]]; then
    tar -C "$package_dir" -czf "$release_dir/$archive_name" "$install_binary_name" "$lsp_install_binary_name"
  else
    require_cmd zip
    (
      cd "$package_dir"
      zip -q "$release_dir/$archive_name" "$install_binary_name" "$lsp_install_binary_name"
    )
  fi
}

require_cmd git
require_cmd go
require_cmd grep
require_cmd awk
require_file "$release_script"
require_file "$install_script"
require_file "$repo_root/$smoke_script"

if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ && "$version" != "smoke-check" ]]; then
  fail "invalid_version" "release artifact check version must match vMAJOR.MINOR.PATCH, prerelease, or smoke-check legacy smoke version: $version" 2
fi

if [[ "$require_clean" == "true" && -n "$(git status --porcelain)" ]]; then
  fail "dirty_worktree" "release artifact check requires a clean git worktree"
fi
if [[ "$require_tag" == "true" ]]; then
  exact_tag="$(git describe --tags --exact-match 2>/dev/null || true)"
  if [[ -z "$exact_tag" ]]; then
    fail "missing_tag" "release artifact check requires HEAD to be exactly tagged"
  fi
  if [[ "$version" != "$exact_tag" ]]; then
    fail "tag_mismatch" "release artifact version $version does not match exact git tag $exact_tag"
  fi
fi

goos="$(go env GOOS)"
goarch="$(go env GOARCH)"
module_path="$(go list -m)"
if [[ "$module_path" != "$expected_module_path" ]]; then
  fail "module_path" "module path = $module_path, want $expected_module_path"
fi
exe_ext=""
archive_ext="tar.gz"
if [[ "$goos" == "windows" ]]; then
  exe_ext=".exe"
  archive_ext="zip"
fi

artifact_base="leia_${version}_${goos}_${goarch}"
binary_name="${artifact_base}${exe_ext}"
lsp_artifact_base="leia-lsp_${version}_${goos}_${goarch}"
lsp_binary_name="${lsp_artifact_base}${exe_ext}"
metadata_name="${artifact_base}_metadata.txt"
install_archive_name="${artifact_base}.${archive_ext}"
install_binary_name="leia${exe_ext}"
lsp_install_binary_name="leia-lsp${exe_ext}"
artifact_files=("$binary_name" "$lsp_binary_name" "$metadata_name" "SHA256SUMS")

dry_run_dir="$(mktemp -d "${TMPDIR:-/tmp}/leia-release-artifacts-check-dry-run.XXXXXX")"
dry_run_log="$(mktemp "${TMPDIR:-/tmp}/leia-release-artifacts-dry-run.XXXXXX")"
trap 'rm -f "$dry_run_log"' EXIT

rmdir "$dry_run_dir"
bash "$release_script" --version "$version" --output-dir "$dry_run_dir" --dry-run > "$dry_run_log"

if [[ -e "$dry_run_dir" ]]; then
  fail "dry_run_side_effect" "dry-run created output path: $dry_run_dir"
fi

require_contains "$dry_run_log" "dry-run: would write $dry_run_dir/$binary_name"
require_contains "$dry_run_log" "dry-run: would write $dry_run_dir/$lsp_binary_name"
require_contains "$dry_run_log" "dry-run: would write $dry_run_dir/$metadata_name"
require_contains "$dry_run_log" "dry-run: would write $dry_run_dir/SHA256SUMS"
require_contains "$dry_run_log" "metadata:"
require_contains "$dry_run_log" "artifact=$binary_name"
require_contains "$dry_run_log" "lsp_artifact=$lsp_binary_name"
require_contains "$dry_run_log" "module=$expected_module_path"
require_contains "$dry_run_log" "version=$version"
require_contains "$dry_run_log" "goos=$goos"
require_contains "$dry_run_log" "goarch=$goarch"

dry_run_verified="true"
log_info "release_artifacts_check.sh: dry-run plan verified"

if [[ "$build" != "true" ]]; then
  if [[ "$json_out" == "true" ]]; then
    print_json_report
  else
    echo "release_artifacts_check.sh: pass"
  fi
  exit 0
fi

created_tmp="false"
if [[ -z "$out_dir" ]]; then
  out_dir="$(mktemp -d "${TMPDIR:-/tmp}/leia-release-artifacts-build.XXXXXX")"
  created_tmp="true"
elif [[ "$out_dir" != /* ]]; then
  out_dir="$repo_root/$out_dir"
fi

cleanup() {
  rm -f "$dry_run_log"
  if [[ "$created_tmp" == "true" && "$keep_output" != "true" ]]; then
    rm -rf "$out_dir"
  fi
}
trap cleanup EXIT

if [[ "$json_out" == "true" ]]; then
  bash "$release_script" --version "$version" --output-dir "$out_dir" >/dev/null
else
  bash "$release_script" --version "$version" --output-dir "$out_dir"
fi

binary_path="$out_dir/$binary_name"
lsp_binary_path="$out_dir/$lsp_binary_name"
metadata_path="$out_dir/$metadata_name"
checksums_path="$out_dir/SHA256SUMS"

require_file "$binary_path"
require_file "$lsp_binary_path"
require_file "$metadata_path"
require_file "$checksums_path"

if [[ ! -x "$binary_path" ]]; then
  fail "artifact_mode" "built binary is not executable: $binary_path"
fi
if [[ ! -x "$lsp_binary_path" ]]; then
  fail "artifact_mode" "built LSP binary is not executable: $lsp_binary_path"
fi

require_contains "$metadata_path" "artifact=$binary_name"
require_contains "$metadata_path" "lsp_artifact=$lsp_binary_name"
require_contains "$metadata_path" "module=$expected_module_path"
require_contains "$metadata_path" "version=$version"
require_contains "$metadata_path" "goos=$goos"
require_contains "$metadata_path" "goarch=$goarch"

verify_checksums "$out_dir"
version_json="$("$binary_path" version --json)"
if ! grep -Fq "\"version\": \"$version\"" <<<"$version_json"; then
  fail "version_metadata" "built CLI version metadata did not include version $version"
fi
if grep -Fq '"version": "dev"' <<<"$version_json"; then
  fail "version_metadata" "built CLI still reports dev version"
fi
"$binary_path" "$smoke_script" >/dev/null
"$lsp_binary_path" --help >/dev/null
build_verified="true"

release_dir="$(mktemp -d "${TMPDIR:-/tmp}/leia-release-install.XXXXXX")"
install_bin_dir="$(mktemp -d "${TMPDIR:-/tmp}/leia-release-install-bin.XXXXXX")"
cleanup_install_fixture() {
  rm -rf "$release_dir" "$install_bin_dir"
}
trap 'cleanup; cleanup_install_fixture' EXIT

package_install_archive "$release_dir" "$install_archive_name" "$binary_path" "$lsp_binary_path" "$archive_ext"
write_checksum_line "$release_dir" "$install_archive_name" >"$release_dir/SHA256SUMS"
install_archive_checksum_count=1

bash "$install_script" \
  --version "$version" \
  --os "$goos" \
  --arch "$goarch" \
  --bin-dir "$install_bin_dir" \
  --base-url "file://$release_dir" >/dev/null

installed_binary="$install_bin_dir/$install_binary_name"
installed_lsp_binary="$install_bin_dir/$lsp_install_binary_name"
require_file "$installed_binary"
require_file "$installed_lsp_binary"
if [[ ! -x "$installed_binary" ]]; then
  fail "install_archive" "installed CLI is not executable: $installed_binary"
fi
if [[ ! -x "$installed_lsp_binary" ]]; then
  fail "install_archive" "installed LSP is not executable: $installed_lsp_binary"
fi
installed_version_json="$("$installed_binary" version --json)"
if ! grep -Fq "\"version\": \"$version\"" <<<"$installed_version_json"; then
  fail "install_archive" "installed CLI version metadata did not include version $version"
fi
"$installed_lsp_binary" --help >/dev/null
install_archive_verified="true"

if [[ "$json_out" == "true" ]]; then
  print_json_report
else
  echo "release_artifacts_check.sh: build artifacts verified in $out_dir"
  echo "release_artifacts_check.sh: local install archive verified from $release_dir"
  echo "release_artifacts_check.sh: pass"
fi
