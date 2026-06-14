#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/release_artifacts.sh [--output-dir DIR] [--version VERSION] [--dry-run]

Build the current-platform Leia CLI and LSP release artifacts locally.

Options:
  -o, --output-dir DIR  Write artifacts under DIR instead of dist/
      --version VERSION Use VERSION in artifact names and metadata instead of
                        the exact tag or dev-<commit> default
      --dry-run         Print planned output files and metadata; do not build
                        or write any files
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

if [[ "$dry_run" == "true" ]]; then
  echo "dry-run: would write $binary_path"
  echo "dry-run: would write $lsp_binary_path"
  echo "dry-run: would write $metadata_path"
  echo "dry-run: would write $checksums_path"
  echo
  echo "metadata:"
  write_metadata
  exit 0
fi

echo "building $binary_name"
go build -trimpath -ldflags="$ldflags" -o "$binary_path" ./cmd/leia
echo "building $lsp_binary_name"
go build -trimpath -ldflags="$ldflags" -o "$lsp_binary_path" ./cmd/leia-lsp

write_metadata >"$metadata_path"

(
  cd "$out_dir"
  sha256_file "$binary_name"
  sha256_file "$lsp_binary_name"
  sha256_file "$metadata_name"
) >"$checksums_path"

echo "wrote $binary_path"
echo "wrote $lsp_binary_path"
echo "wrote $metadata_path"
echo "wrote $checksums_path"
