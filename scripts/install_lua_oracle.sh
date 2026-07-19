#!/usr/bin/env bash
set -euo pipefail

readonly LUA_VERSION="5.5.0"
readonly LUA_SHA256="57ccc32bbbd005cab75bcc52444052535af691789dba2b9016d5c50640d68b3d"
readonly LUA_URL="https://www.lua.org/ftp/lua-${LUA_VERSION}.tar.gz"

usage() {
  cat <<'USAGE'
Usage: scripts/install_lua_oracle.sh [--bin-dir DIR]

Build and install the pinned Lua reference interpreter used by conformance tests.

Options:
  --bin-dir DIR  Install directory. Default: $HOME/.local/bin
  -h, --help     Show this help.
USAGE
}

bin_dir="${HOME}/.local/bin"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --bin-dir)
      [[ $# -ge 2 && -n "$2" ]] || { echo "error: --bin-dir requires a directory" >&2; exit 2; }
      bin_dir="$2"
      shift 2
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

for command in curl make tar; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "error: required command not found: $command" >&2
    exit 1
  fi
done

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    echo "error: need sha256sum or shasum for checksum verification" >&2
    exit 1
  fi
}

case "$(uname -s)" in
  Linux) make_target="linux" ;;
  Darwin) make_target="macosx" ;;
  *) echo "error: unsupported host OS: $(uname -s)" >&2; exit 1 ;;
esac

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
archive="$tmp_dir/lua-${LUA_VERSION}.tar.gz"

curl --fail --location --retry 3 --silent --show-error "$LUA_URL" --output "$archive"
actual_sha256="$(sha256_file "$archive")"
if [[ "$actual_sha256" != "$LUA_SHA256" ]]; then
  echo "error: Lua ${LUA_VERSION} checksum mismatch: got ${actual_sha256}" >&2
  exit 1
fi

tar -xzf "$archive" -C "$tmp_dir"
make -C "$tmp_dir/lua-${LUA_VERSION}" "$make_target"
mkdir -p "$bin_dir"
install -m 0755 "$tmp_dir/lua-${LUA_VERSION}/src/lua" "$bin_dir/lua"
install -m 0755 "$tmp_dir/lua-${LUA_VERSION}/src/luac" "$bin_dir/luac"

"$bin_dir/lua" -v
