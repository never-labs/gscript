#!/usr/bin/env bash
# Verify a GoReleaser snapshot archive through the public installer path.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
dist_dir="$repo_root/dist"
bin_dir=""
goos=""
goarch=""
json_out="false"

usage() {
  cat <<'USAGE'
Usage: scripts/release_snapshot_install_check.sh [--dist-dir DIR] [--bin-dir DIR] [--os GOOS] [--arch GOARCH] [--json] [--help]

Finds a GoReleaser snapshot archive in DIR, stages it under the public installer
asset naming contract, and installs it with scripts/install.sh using file://.

Options:
  --dist-dir DIR   GoReleaser dist directory. Default: ./dist.
  --bin-dir DIR    Install directory. Default: a temporary directory.
  --os GOOS        Target GOOS. Default: current host GOOS.
  --arch GOARCH    Target GOARCH. Default: current host GOARCH.
  --json           Print a machine-readable verification report.
  -h, --help       Show this help.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dist-dir)
      [[ $# -ge 2 && -n "$2" ]] || { echo "error: --dist-dir requires a value" >&2; exit 2; }
      dist_dir="$2"
      shift 2
      ;;
    --bin-dir)
      [[ $# -ge 2 && -n "$2" ]] || { echo "error: --bin-dir requires a value" >&2; exit 2; }
      bin_dir="$2"
      shift 2
      ;;
    --os)
      [[ $# -ge 2 && -n "$2" ]] || { echo "error: --os requires a GOOS value" >&2; exit 2; }
      goos="$2"
      shift 2
      ;;
    --arch)
      [[ $# -ge 2 && -n "$2" ]] || { echo "error: --arch requires a GOARCH value" >&2; exit 2; }
      goarch="$2"
      shift 2
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

detect_os() {
  case "$(uname -s)" in
    Darwin) echo darwin ;;
    Linux) echo linux ;;
    MINGW*|MSYS*|CYGWIN*) echo windows ;;
    *) echo "error: unsupported OS: $(uname -s)" >&2; exit 1 ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    arm64|aarch64) echo arm64 ;;
    *) echo "error: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
  esac
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
  local status="$1"
  local failure_kind="${2:-}"
  local failure_message="${3:-}"
  printf '{\n'
  printf '  "schema_version": 1,\n'
  printf '  "status": "%s",\n' "$(json_escape "$status")"
  printf '  "dist_dir": "%s",\n' "$(json_escape "$dist_dir")"
  printf '  "goos": "%s",\n' "$(json_escape "$goos")"
  printf '  "goarch": "%s",\n' "$(json_escape "$goarch")"
  printf '  "archive": "%s",\n' "$(json_escape "${archive:-}")"
  printf '  "archive_name": "%s",\n' "$(json_escape "${archive_name:-}")"
  printf '  "snapshot_version": "%s",\n' "$(json_escape "${snapshot_version:-}")"
  printf '  "installer_version": "%s",\n' "$(json_escape "${installer_version:-}")"
  printf '  "staged_asset": "%s",\n' "$(json_escape "${staged_asset:-}")"
  printf '  "staged_release_dir": "%s",\n' "$(json_escape "${staged_release_dir:-}")"
  printf '  "bin_dir": "%s",\n' "$(json_escape "${bin_dir:-}")"
  printf '  "install_count": %d,\n' "${install_count:-0}"
  printf '  "installed_paths": [\n'
  if [[ "${install_count:-0}" -gt 0 ]]; then
    printf '    "%s",\n' "$(json_escape "${installed_cli:-}")"
    printf '    "%s"\n' "$(json_escape "${installed_lsp:-}")"
  fi
  printf '  ],\n'
  if [[ -n "$failure_kind" ]]; then
    printf '  "failure_kind_count": 1,\n'
    printf '  "failure_count": 1,\n'
    printf '  "failure_kinds": ["%s"],\n' "$(json_escape "$failure_kind")"
    printf '  "failure_details": [{"kind": "%s", "message": "%s"}]\n' "$(json_escape "$failure_kind")" "$(json_escape "$failure_message")"
  else
    printf '  "failure_kind_count": 0,\n'
    printf '  "failure_count": 0,\n'
    printf '  "failure_kinds": [],\n'
    printf '  "failure_details": []\n'
  fi
  printf '}\n'
}

fail() {
  local kind="$1"
  local message="$2"
  if [[ "$json_out" == "true" ]]; then
    print_json_report "fail" "$kind" "$message"
  else
    echo "release_snapshot_install_check.sh: $message" >&2
  fi
  exit 1
}

goos="${goos:-$(detect_os)}"
goarch="${goarch:-$(detect_arch)}"
archive_ext="tar.gz"
binary_name="leia"
lsp_binary_name="leia-lsp"
if [[ "$goos" == "windows" ]]; then
  archive_ext="zip"
  binary_name="leia.exe"
  lsp_binary_name="leia-lsp.exe"
fi

[[ -d "$dist_dir" ]] || fail "missing_dist_dir" "dist directory not found: $dist_dir"
archive_matches=()
while IFS= read -r candidate; do
  [[ -n "$candidate" ]] && archive_matches+=("$candidate")
done < <(find "$dist_dir" -maxdepth 1 -type f -name "leia_*_${goos}_${goarch}.${archive_ext}" | sort)
if [[ "${#archive_matches[@]}" -eq 0 ]]; then
  fail "missing_archive" "no GoReleaser archive found for ${goos}/${goarch} in $dist_dir"
fi
if [[ "${#archive_matches[@]}" -gt 1 ]]; then
  fail "ambiguous_archive" "multiple GoReleaser archives found for ${goos}/${goarch} in $dist_dir"
fi
archive="${archive_matches[0]}"
archive_name="$(basename "$archive")"

snapshot_version="${archive_name#leia_}"
snapshot_version="${snapshot_version%_${goos}_${goarch}.${archive_ext}}"
if [[ -z "$snapshot_version" || "$snapshot_version" == "$archive_name" ]]; then
  fail "invalid_archive_name" "could not derive snapshot version from $archive_name"
fi
installer_version="$snapshot_version"
if [[ "$installer_version" != v* ]]; then
  installer_version="v${installer_version}"
fi
if [[ ! "$installer_version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  fail "invalid_version" "derived installer version is not install-compatible: $installer_version"
fi

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/leia-snapshot-install.XXXXXX")"
cleanup() {
  if [[ -z "${LEIA_KEEP_SNAPSHOT_INSTALL_TMP:-}" ]]; then
    rm -rf "$tmp_dir"
  fi
}
trap cleanup EXIT

staged_release_dir="$tmp_dir/release"
mkdir -p "$staged_release_dir"
staged_asset="leia_${installer_version}_${goos}_${goarch}.${archive_ext}"
cp "$archive" "$staged_release_dir/$staged_asset"
printf '%s  %s\n' "$(sha256_file "$staged_release_dir/$staged_asset")" "$staged_asset" >"$staged_release_dir/SHA256SUMS"

if [[ -z "$bin_dir" ]]; then
  bin_dir="$tmp_dir/bin"
fi
mkdir -p "$bin_dir"

bash scripts/install.sh \
  --version "$installer_version" \
  --os "$goos" \
  --arch "$goarch" \
  --bin-dir "$bin_dir" \
  --base-url "file://$staged_release_dir" >/dev/null

installed_cli="$bin_dir/$binary_name"
installed_lsp="$bin_dir/$lsp_binary_name"
[[ -x "$installed_cli" ]] || fail "missing_installed_binary" "installer did not create executable $installed_cli"
[[ -x "$installed_lsp" ]] || fail "missing_installed_binary" "installer did not create executable $installed_lsp"
install_count=2

if [[ "$json_out" == "true" ]]; then
  print_json_report "pass"
else
  echo "release_snapshot_install_check.sh: installed $archive_name through scripts/install.sh"
  echo "  $installed_cli"
  echo "  $installed_lsp"
fi
