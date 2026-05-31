#!/usr/bin/env bash
# Smoke-check release artifact planning, and optionally build into a temp dir.

set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/release_artifacts_check.sh [--build] [--output-dir DIR] [--version VERSION] [--keep-output]

Checks scripts/release_artifacts.sh without changing its implementation.

Default mode runs a dry-run and verifies the planned artifact names and metadata.
With --build, the script builds into a temporary output directory unless
--output-dir is provided, then verifies SHA256SUMS and runs the built binary
against tests/smoke/01_basic.gs.

Options:
      --build           Run a real local build after the dry-run check
  -o, --output-dir DIR  Output directory for --build mode; defaults to mktemp
      --version VERSION Version to pass to release_artifacts.sh
      --keep-output     Do not remove an auto-created temp output directory
  -h, --help            Show this help
USAGE
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd "$script_dir/.." && pwd -P)"
release_script="$repo_root/scripts/release_artifacts.sh"
smoke_script="tests/smoke/01_basic.gs"
expected_module_path="github.com/never-labs/gscript"

build="false"
out_dir=""
version="smoke-check"
keep_output="false"

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

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

require_file() {
  if [[ ! -f "$1" ]]; then
    echo "error: missing required file: $1" >&2
    exit 1
  fi
}

require_contains() {
  local file="$1"
  local pattern="$2"
  if ! grep -Fq "$pattern" "$file"; then
    echo "error: $file does not contain expected text: $pattern" >&2
    exit 1
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
    echo "error: need sha256sum, shasum, or openssl for checksum verification" >&2
    exit 1
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
      echo "error: malformed checksum line in $sums: $expected $name $extra" >&2
      exit 1
    fi
    if [[ ! "$expected" =~ ^[0-9a-fA-F]{64}$ ]]; then
      echo "error: malformed sha256 digest in $sums: $expected" >&2
      exit 1
    fi
    if [[ "$name" == */* || "$name" == "." || "$name" == ".." ]]; then
      echo "error: unsafe checksum filename in $sums: $name" >&2
      exit 1
    fi
    require_file "$dir/$name"
    actual="$(sha256_file "$dir/$name")"
    if [[ "$actual" != "$expected" ]]; then
      echo "error: checksum mismatch for $name" >&2
      echo "  expected: $expected" >&2
      echo "  actual:   $actual" >&2
      exit 1
    fi
    checked=$((checked + 1))
  done < "$sums"

  if [[ "$checked" -ne 2 ]]; then
    echo "error: expected 2 checksum entries, found $checked" >&2
    exit 1
  fi
}

require_cmd git
require_cmd go
require_cmd grep
require_cmd awk
require_file "$release_script"
require_file "$repo_root/$smoke_script"

goos="$(go env GOOS)"
goarch="$(go env GOARCH)"
module_path="$(go list -m)"
if [[ "$module_path" != "$expected_module_path" ]]; then
  echo "error: module path = $module_path, want $expected_module_path" >&2
  exit 1
fi
exe_ext=""
if [[ "$goos" == "windows" ]]; then
  exe_ext=".exe"
fi

artifact_base="gscript_${version}_${goos}_${goarch}"
binary_name="${artifact_base}${exe_ext}"
metadata_name="${artifact_base}_metadata.txt"

dry_run_dir="$(mktemp -d "${TMPDIR:-/tmp}/gscript-release-artifacts-check-dry-run.XXXXXX")"
dry_run_log="$(mktemp "${TMPDIR:-/tmp}/gscript-release-artifacts-dry-run.XXXXXX")"
trap 'rm -f "$dry_run_log"' EXIT

rmdir "$dry_run_dir"
bash "$release_script" --version "$version" --output-dir "$dry_run_dir" --dry-run > "$dry_run_log"

if [[ -e "$dry_run_dir" ]]; then
  echo "error: dry-run created output path: $dry_run_dir" >&2
  exit 1
fi

require_contains "$dry_run_log" "dry-run: would write $dry_run_dir/$binary_name"
require_contains "$dry_run_log" "dry-run: would write $dry_run_dir/$metadata_name"
require_contains "$dry_run_log" "dry-run: would write $dry_run_dir/SHA256SUMS"
require_contains "$dry_run_log" "metadata:"
require_contains "$dry_run_log" "artifact=$binary_name"
require_contains "$dry_run_log" "module=$expected_module_path"
require_contains "$dry_run_log" "version=$version"
require_contains "$dry_run_log" "goos=$goos"
require_contains "$dry_run_log" "goarch=$goarch"

echo "release_artifacts_check.sh: dry-run plan verified"

if [[ "$build" != "true" ]]; then
  echo "release_artifacts_check.sh: pass"
  exit 0
fi

created_tmp="false"
if [[ -z "$out_dir" ]]; then
  out_dir="$(mktemp -d "${TMPDIR:-/tmp}/gscript-release-artifacts-build.XXXXXX")"
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

bash "$release_script" --version "$version" --output-dir "$out_dir"

binary_path="$out_dir/$binary_name"
metadata_path="$out_dir/$metadata_name"
checksums_path="$out_dir/SHA256SUMS"

require_file "$binary_path"
require_file "$metadata_path"
require_file "$checksums_path"

if [[ ! -x "$binary_path" ]]; then
  echo "error: built binary is not executable: $binary_path" >&2
  exit 1
fi

require_contains "$metadata_path" "artifact=$binary_name"
require_contains "$metadata_path" "module=$expected_module_path"
require_contains "$metadata_path" "version=$version"
require_contains "$metadata_path" "goos=$goos"
require_contains "$metadata_path" "goarch=$goarch"

verify_checksums "$out_dir"
"$binary_path" "$smoke_script" >/dev/null

echo "release_artifacts_check.sh: build artifacts verified in $out_dir"
echo "release_artifacts_check.sh: pass"
