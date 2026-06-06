#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/install.sh [--version VERSION] [--bin-dir DIR] [--repo OWNER/REPO] [--dry-run]

Install the Leia CLI and LSP from GitHub release artifacts.

Options:
      --version VERSION  Release tag to install, for example v0.1.0.
                         Defaults to the latest GitHub release.
      --bin-dir DIR      Install directory. Default: /usr/local/bin
      --repo OWNER/REPO  GitHub repository. Default: never-labs/leia
      --os GOOS          Override detected OS for validation.
      --arch GOARCH      Override detected arch for validation.
      --dry-run          Print the planned download and install paths only.
      --no-verify        Skip SHA256SUMS verification.
  -h, --help             Show this help.

Environment:
  LEIA_INSTALL_VERSION   Default version when --version is omitted.
  LEIA_INSTALL_DIR       Default install directory when --bin-dir is omitted.
  LEIA_INSTALL_REPO      Default repository when --repo is omitted.
USAGE
}

repo="${LEIA_INSTALL_REPO:-never-labs/leia}"
version="${LEIA_INSTALL_VERSION:-}"
bin_dir="${LEIA_INSTALL_DIR:-/usr/local/bin}"
goos=""
goarch=""
dry_run="false"
verify="true"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      [[ $# -ge 2 && -n "$2" ]] || { echo "error: --version requires a value" >&2; exit 2; }
      version="$2"
      shift 2
      ;;
    --bin-dir)
      [[ $# -ge 2 && -n "$2" ]] || { echo "error: --bin-dir requires a directory" >&2; exit 2; }
      bin_dir="$2"
      shift 2
      ;;
    --repo)
      [[ $# -ge 2 && -n "$2" ]] || { echo "error: --repo requires OWNER/REPO" >&2; exit 2; }
      repo="$2"
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
    --dry-run)
      dry_run="true"
      shift
      ;;
    --no-verify)
      verify="false"
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

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

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

latest_version() {
  require_cmd curl
  curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" |
    sed -n 's/^[[:space:]]*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' |
    head -n 1
}

goos="${goos:-$(detect_os)}"
goarch="${goarch:-$(detect_arch)}"

case "$goos" in
  darwin|linux|windows) ;;
  *) echo "error: unsupported GOOS: $goos" >&2; exit 1 ;;
esac

case "$goarch" in
  amd64|arm64) ;;
  *) echo "error: unsupported GOARCH: $goarch" >&2; exit 1 ;;
esac

if [[ -z "$version" ]]; then
  version="$(latest_version)"
fi
if [[ -z "$version" ]]; then
  echo "error: could not determine latest release version" >&2
  exit 1
fi

archive_ext="tar.gz"
binary_name="leia"
lsp_binary_name="leia-lsp"
if [[ "$goos" == "windows" ]]; then
  archive_ext="zip"
  binary_name="leia.exe"
  lsp_binary_name="leia-lsp.exe"
fi

asset="leia_${version}_${goos}_${goarch}.${archive_ext}"
base_url="https://github.com/${repo}/releases/download/${version}"
asset_url="${base_url}/${asset}"
checksums_url="${base_url}/SHA256SUMS"
install_path="${bin_dir}/${binary_name}"
lsp_install_path="${bin_dir}/${lsp_binary_name}"

if [[ "$dry_run" == "true" ]]; then
  echo "version=$version"
  echo "goos=$goos"
  echo "goarch=$goarch"
  echo "asset=$asset"
  echo "url=$asset_url"
  echo "checksums=$checksums_url"
  echo "install_path=$install_path"
  echo "lsp_install_path=$lsp_install_path"
  exit 0
fi

require_cmd curl
require_cmd awk
if [[ "$archive_ext" == "tar.gz" ]]; then
  require_cmd tar
else
  require_cmd unzip
fi

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/leia-install.XXXXXX")"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

archive_path="$tmp_dir/$asset"
checksums_path="$tmp_dir/SHA256SUMS"

curl -fL "$asset_url" -o "$archive_path"

if [[ "$verify" == "true" ]]; then
  curl -fsSL "$checksums_url" -o "$checksums_path"
  expected="$(awk -v name="$asset" '$2 == name { print $1 }' "$checksums_path")"
  if [[ -z "$expected" ]]; then
    echo "error: checksum for $asset not found in SHA256SUMS" >&2
    exit 1
  fi
  actual="$(sha256_file "$archive_path")"
  if [[ "$actual" != "$expected" ]]; then
    echo "error: checksum mismatch for $asset" >&2
    echo "  expected: $expected" >&2
    echo "  actual:   $actual" >&2
    exit 1
  fi
fi

extract_dir="$tmp_dir/extract"
mkdir -p "$extract_dir"
if [[ "$archive_ext" == "tar.gz" ]]; then
  tar -xzf "$archive_path" -C "$extract_dir"
else
  unzip -q "$archive_path" -d "$extract_dir"
fi

if [[ ! -f "$extract_dir/$binary_name" ]]; then
  echo "error: archive did not contain $binary_name" >&2
  exit 1
fi
if [[ ! -f "$extract_dir/$lsp_binary_name" ]]; then
  echo "error: archive did not contain $lsp_binary_name" >&2
  exit 1
fi

mkdir -p "$bin_dir"
install -m 0755 "$extract_dir/$binary_name" "$install_path"
install -m 0755 "$extract_dir/$lsp_binary_name" "$lsp_install_path"
echo "installed $install_path"
echo "installed $lsp_install_path"
