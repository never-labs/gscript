#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/install.sh [--version VERSION] [--bin-dir DIR] [--repo OWNER/REPO] [--base-url URL] [--dry-run] [--json]

Install the Leia CLI and LSP from GitHub release artifacts.

Options:
      --version VERSION  Release tag to install, for example v0.1.0.
                         Defaults to the latest GitHub release.
      --bin-dir DIR      Install directory. Default: /usr/local/bin
      --repo OWNER/REPO  GitHub repository. Default: never-labs/leia
      --base-url URL     Release asset directory URL. Defaults to the GitHub
                         release download URL for --repo and --version.
      --os GOOS          Override detected OS for validation.
      --arch GOARCH      Override detected arch for validation.
      --dry-run          Print the planned download and install paths only.
      --no-verify        Skip SHA256SUMS verification.
      --json             Print a machine-readable install plan/report.
  -h, --help             Show this help.

Environment:
  LEIA_INSTALL_VERSION   Default version when --version is omitted.
  LEIA_INSTALL_DIR       Default install directory when --bin-dir is omitted.
  LEIA_INSTALL_REPO      Default repository when --repo is omitted.
  LEIA_INSTALL_BASE_URL  Default release asset directory URL.
USAGE
}

repo="${LEIA_INSTALL_REPO:-never-labs/leia}"
version="${LEIA_INSTALL_VERSION:-}"
bin_dir="${LEIA_INSTALL_DIR:-/usr/local/bin}"
base_url_override="${LEIA_INSTALL_BASE_URL:-}"
goos=""
goarch=""
dry_run="false"
verify="true"
json_out="false"

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
    --base-url)
      [[ $# -ge 2 && -n "$2" ]] || { echo "error: --base-url requires a URL" >&2; exit 2; }
      base_url_override="${2%/}"
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
    --json)
      json_out="true"
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
  printf '{\n'
  printf '  "schema_version": 1,\n'
  printf '  "status": "pass",\n'
  printf '  "dry_run": %s,\n' "$dry_run"
  printf '  "verify": %s,\n' "$verify"
  printf '  "repo": "%s",\n' "$(json_escape "$repo")"
  printf '  "version": "%s",\n' "$(json_escape "$version")"
  printf '  "goos": "%s",\n' "$(json_escape "$goos")"
  printf '  "goarch": "%s",\n' "$(json_escape "$goarch")"
  printf '  "archive_ext": "%s",\n' "$(json_escape "$archive_ext")"
  printf '  "asset": "%s",\n' "$(json_escape "$asset")"
  printf '  "url": "%s",\n' "$(json_escape "$asset_url")"
  printf '  "checksums": "%s",\n' "$(json_escape "$checksums_url")"
  printf '  "bin_dir": "%s",\n' "$(json_escape "$bin_dir")"
  printf '  "binary": "%s",\n' "$(json_escape "$binary_name")"
  printf '  "lsp_binary": "%s",\n' "$(json_escape "$lsp_binary_name")"
  printf '  "install_path": "%s",\n' "$(json_escape "$install_path")"
  printf '  "lsp_install_path": "%s"\n' "$(json_escape "$lsp_install_path")"
  printf '}\n'
}

latest_version() {
  require_cmd curl
  curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" |
    sed -n 's/^[[:space:]]*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' |
    head -n 1
}

fetch_url() {
  local url="$1"
  local out="$2"
  case "$url" in
    file://*)
      local path="${url#file://}"
      if [[ ! -f "$path" ]]; then
        echo "error: local release asset not found: $path" >&2
        exit 1
      fi
      cp "$path" "$out"
      ;;
    *)
      require_cmd curl
      curl -fL "$url" -o "$out"
      ;;
  esac
}

archive_entries() {
  local archive="$1"
  if [[ "$archive_ext" == "tar.gz" ]]; then
    tar -tzf "$archive"
  else
    unzip -Z1 "$archive"
  fi
}

validate_archive_entry() {
  local entry="$1"
  while [[ "$entry" == ./* ]]; do
    entry="${entry#./}"
  done
  if [[ -z "$entry" || "$entry" == */ || "$entry" == /* || "$entry" == *".."* || "$entry" == *"/"* ]]; then
    echo "error: unsafe archive entry: $1" >&2
    exit 1
  fi
  case "$entry" in
    "$binary_name"|"$lsp_binary_name"|README.md|SECURITY.md)
      printf '%s\n' "$entry"
      ;;
    *)
      echo "error: unexpected archive entry: $1" >&2
      exit 1
      ;;
  esac
}

validate_archive_entries() {
  local archive="$1"
  local found_cli=0
  local found_lsp=0
  local entry
  while IFS= read -r entry; do
    entry="$(validate_archive_entry "$entry")"
    case "$entry" in
      "$binary_name") found_cli=$((found_cli + 1)) ;;
      "$lsp_binary_name") found_lsp=$((found_lsp + 1)) ;;
    esac
  done < <(archive_entries "$archive")
  if [[ "$found_cli" -ne 1 || "$found_lsp" -ne 1 ]]; then
    echo "error: archive must contain exactly one $binary_name and one $lsp_binary_name" >&2
    exit 1
  fi
}

goos="${goos:-$(detect_os)}"
goarch="${goarch:-$(detect_arch)}"

if [[ ! "$repo" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
  echo "error: --repo must be OWNER/REPO: $repo" >&2
  exit 2
fi

if [[ -n "$version" && ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "error: --version must match vMAJOR.MINOR.PATCH or prerelease: $version" >&2
  exit 2
fi

case "$goos" in
  darwin|linux|windows) ;;
  *) echo "error: unsupported GOOS: $goos" >&2; exit 1 ;;
esac

case "$goarch" in
  amd64|arm64) ;;
  *) echo "error: unsupported GOARCH: $goarch" >&2; exit 1 ;;
esac

if [[ -z "$version" && -n "$base_url_override" ]]; then
  echo "error: --version is required when --base-url is used" >&2
  exit 2
fi

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
if [[ -n "$base_url_override" ]]; then
  base_url="$base_url_override"
else
  base_url="https://github.com/${repo}/releases/download/${version}"
fi
asset_url="${base_url}/${asset}"
checksums_url="${base_url}/SHA256SUMS"
install_path="${bin_dir}/${binary_name}"
lsp_install_path="${bin_dir}/${lsp_binary_name}"

if [[ "$dry_run" == "true" ]]; then
  if [[ "$json_out" == "true" ]]; then
    print_json_report
    exit 0
  fi
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

fetch_url "$asset_url" "$archive_path"

if [[ "$verify" == "true" ]]; then
  fetch_url "$checksums_url" "$checksums_path"
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
validate_archive_entries "$archive_path"
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
if [[ "$json_out" == "true" ]]; then
  print_json_report
else
  echo "installed $install_path"
  echo "installed $lsp_install_path"
fi
