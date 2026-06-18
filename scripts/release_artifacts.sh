#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/release_artifacts.sh [--output-dir DIR] [--version VERSION] [--dry-run] [--json]

Build the current-platform Leia CLI and LSP release artifacts locally.

Options:
  -o, --output-dir DIR  Write artifacts under DIR instead of dist/
      --version VERSION Use VERSION in artifact names and metadata instead of
                        the exact tag or dev-<commit> default
      --dry-run         Print planned output files and metadata; do not build
                        or write any files
      --json            Print a machine-readable artifact plan/report
  -h, --help            Show this help

Outputs:
  DIR/leia_<version>_<goos>_<goarch>[.exe]
  DIR/leia-lsp_<version>_<goos>_<goarch>[.exe]
  DIR/leia_<version>_<goos>_<goarch>_metadata.txt
  DIR/SHA256SUMS

This script only writes local files. It does not tag, publish, or upload.
USAGE
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd "$script_dir/.." && pwd -P)"
out_dir="$repo_root/dist"
requested_version=""
dry_run="false"
json_out="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    -o|--output-dir)
      if [[ $# -lt 2 ]]; then
        echo "error: $1 requires a directory" >&2
        exit 2
      fi
      out_dir="$2"
      shift 2
      ;;
    --version)
      if [[ $# -lt 2 ]]; then
        echo "error: $1 requires a version" >&2
        exit 2
      fi
      if [[ -z "$2" ]]; then
        echo "error: $1 requires a non-empty version" >&2
        exit 2
      fi
      requested_version="$2"
      shift 2
      ;;
    --dry-run)
      dry_run="true"
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

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

require_cmd git
require_cmd go

if [[ -n "$requested_version" && ! "$requested_version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ && "$requested_version" != "smoke-check" ]]; then
  echo "error: release artifact version must match vMAJOR.MINOR.PATCH or prerelease: $requested_version" >&2
  exit 2
fi

if [[ "$out_dir" != /* ]]; then
  out_dir="$repo_root/$out_dir"
fi

if [[ "$dry_run" == "false" ]]; then
  mkdir -p "$out_dir"
  out_dir="$(cd "$out_dir" && pwd -P)"
fi

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1"
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1"
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 -r "$1"
  else
    echo "error: need sha256sum, shasum, or openssl to write SHA256SUMS" >&2
    exit 1
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

sanitize_component() {
  printf '%s' "$1" | tr -c '[:alnum:]._-' '-'
}

goos="$(go env GOOS)"
goarch="$(go env GOARCH)"
gohostos="$(go env GOHOSTOS)"
gohostarch="$(go env GOHOSTARCH)"
cgo_enabled="$(go env CGO_ENABLED)"
go_version="$(go version)"
module_path="$(go list -m)"

commit="$(git rev-parse HEAD)"
short_commit="$(git rev-parse --short=12 HEAD)"
branch="$(git rev-parse --abbrev-ref HEAD)"
exact_tag="$(git describe --tags --exact-match 2>/dev/null || true)"
describe="$(git describe --tags --always --dirty 2>/dev/null || git rev-parse --short HEAD)"

if [[ -n "$requested_version" ]]; then
  version="$requested_version"
elif [[ -n "$exact_tag" ]]; then
  version="$exact_tag"
else
  version="dev-$short_commit"
fi
version="$(sanitize_component "$version")"

dirty="false"
if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
  dirty="true"
fi

exe_ext=""
if [[ "$goos" == "windows" ]]; then
  exe_ext=".exe"
fi

artifact_base="leia_${version}_${goos}_${goarch}"
binary_name="${artifact_base}${exe_ext}"
lsp_artifact_base="leia-lsp_${version}_${goos}_${goarch}"
lsp_binary_name="${lsp_artifact_base}${exe_ext}"
metadata_name="${artifact_base}_metadata.txt"
binary_path="$out_dir/$binary_name"
lsp_binary_path="$out_dir/$lsp_binary_name"
metadata_path="$out_dir/$metadata_name"
checksums_path="$out_dir/SHA256SUMS"
build_time_utc="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
ldflags="-s -w -X main.cliVersion=$version"

write_metadata() {
  echo "artifact=$binary_name"
  echo "lsp_artifact=$lsp_binary_name"
  echo "module=$module_path"
  echo "version=$version"
  echo "git_commit=$commit"
  echo "git_short_commit=$short_commit"
  echo "git_branch=$branch"
  echo "git_exact_tag=$exact_tag"
  echo "git_describe=$describe"
  echo "git_dirty=$dirty"
  echo "go_version=$go_version"
  echo "goos=$goos"
  echo "goarch=$goarch"
  echo "gohostos=$gohostos"
  echo "gohostarch=$gohostarch"
  echo "cgo_enabled=$cgo_enabled"
  echo "platform_uname=$(uname -srm 2>/dev/null || true)"
  echo "build_time_utc=$build_time_utc"
}

print_json_report() {
  printf '{\n'
  printf '  "schema_version": 1,\n'
  printf '  "status": "pass",\n'
  printf '  "dry_run": %s,\n' "$dry_run"
  printf '  "output_dir": "%s",\n' "$(json_escape "$out_dir")"
  printf '  "version": "%s",\n' "$(json_escape "$version")"
  printf '  "module": "%s",\n' "$(json_escape "$module_path")"
  printf '  "goos": "%s",\n' "$(json_escape "$goos")"
  printf '  "goarch": "%s",\n' "$(json_escape "$goarch")"
  printf '  "artifact": "%s",\n' "$(json_escape "$binary_name")"
  printf '  "lsp_artifact": "%s",\n' "$(json_escape "$lsp_binary_name")"
  printf '  "metadata": "%s",\n' "$(json_escape "$metadata_name")"
  printf '  "checksums": "SHA256SUMS",\n'
  printf '  "artifact_count": 4,\n'
  printf '  "checksum_entry_count": 3,\n'
  printf '  "artifact_files": [\n'
  printf '    "%s",\n' "$(json_escape "$binary_name")"
  printf '    "%s",\n' "$(json_escape "$lsp_binary_name")"
  printf '    "%s",\n' "$(json_escape "$metadata_name")"
  printf '    "SHA256SUMS"\n'
  printf '  ],\n'
  printf '  "artifact_entries": [\n'
  printf '    {"role": "cli", "name": "%s", "path": "%s"},\n' "$(json_escape "$binary_name")" "$(json_escape "$binary_path")"
  printf '    {"role": "lsp", "name": "%s", "path": "%s"},\n' "$(json_escape "$lsp_binary_name")" "$(json_escape "$lsp_binary_path")"
  printf '    {"role": "metadata", "name": "%s", "path": "%s"},\n' "$(json_escape "$metadata_name")" "$(json_escape "$metadata_path")"
  printf '    {"role": "checksums", "name": "SHA256SUMS", "path": "%s"}\n' "$(json_escape "$checksums_path")"
  printf '  ],\n'
  printf '  "artifact_path": "%s",\n' "$(json_escape "$binary_path")"
  printf '  "lsp_artifact_path": "%s",\n' "$(json_escape "$lsp_binary_path")"
  printf '  "metadata_path": "%s",\n' "$(json_escape "$metadata_path")"
  printf '  "checksums_path": "%s",\n' "$(json_escape "$checksums_path")"
  printf '  "git_commit": "%s",\n' "$(json_escape "$commit")"
  printf '  "git_short_commit": "%s",\n' "$(json_escape "$short_commit")"
  printf '  "git_branch": "%s",\n' "$(json_escape "$branch")"
  printf '  "git_exact_tag": "%s",\n' "$(json_escape "$exact_tag")"
  printf '  "git_describe": "%s",\n' "$(json_escape "$describe")"
  printf '  "git_dirty": %s,\n' "$dirty"
  printf '  "go_version": "%s",\n' "$(json_escape "$go_version")"
  printf '  "build_time_utc": "%s"\n' "$(json_escape "$build_time_utc")"
  printf '}\n'
}

if [[ "$dry_run" == "true" ]]; then
  if [[ "$json_out" == "true" ]]; then
    print_json_report
    exit 0
  fi
  echo "dry-run: would write $binary_path"
  echo "dry-run: would write $lsp_binary_path"
  echo "dry-run: would write $metadata_path"
  echo "dry-run: would write $checksums_path"
  echo
  echo "metadata:"
  write_metadata
  exit 0
fi

if [[ "$json_out" == "true" ]]; then
  echo "building $binary_name" >&2
else
  echo "building $binary_name"
fi
go build -trimpath -ldflags="$ldflags" -o "$binary_path" ./cmd/leia
if [[ "$json_out" == "true" ]]; then
  echo "building $lsp_binary_name" >&2
else
  echo "building $lsp_binary_name"
fi
go build -trimpath -ldflags="$ldflags" -o "$lsp_binary_path" ./cmd/leia-lsp

write_metadata >"$metadata_path"

(
  cd "$out_dir"
  sha256_file "$binary_name"
  sha256_file "$lsp_binary_name"
  sha256_file "$metadata_name"
) >"$checksums_path"

if [[ "$json_out" == "true" ]]; then
  print_json_report
else
  echo "wrote $binary_path"
  echo "wrote $lsp_binary_path"
  echo "wrote $metadata_path"
  echo "wrote $checksums_path"
fi
