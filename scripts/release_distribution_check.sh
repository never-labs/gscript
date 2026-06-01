#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd "$script_dir/.." && pwd -P)"

cd "$repo_root"

require_file() {
  if [[ ! -f "$1" ]]; then
    echo "error: missing required file: $1" >&2
    exit 1
  fi
}

require_contains() {
  local file="$1"
  local text="$2"
  if ! grep -Fq -- "$text" "$file"; then
    echo "error: $file does not contain expected text: $text" >&2
    exit 1
  fi
}

require_file .goreleaser.yaml
require_file .github/workflows/release.yml
require_file .github/workflows/distribution-check.yml
require_file scripts/install.sh

require_contains .goreleaser.yaml "version: 2"
require_contains .goreleaser.yaml "main: ./cmd/leia"
require_contains .goreleaser.yaml "- darwin"
require_contains .goreleaser.yaml "- linux"
require_contains .goreleaser.yaml "- windows"
require_contains .goreleaser.yaml "- amd64"
require_contains .goreleaser.yaml "- arm64"
require_contains .goreleaser.yaml "name_template: \"{{ .ProjectName }}_{{ .Tag }}_{{ .Os }}_{{ .Arch }}\""

bash -n scripts/install.sh

for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  goos="${target%/*}"
  goarch="${target#*/}"
  output="$(bash scripts/install.sh --dry-run --version v0.0.0 --os "$goos" --arch "$goarch" --bin-dir /tmp/leia-bin)"
  case "$goos" in
    windows)
      expected_asset="asset=leia_v0.0.0_${goos}_${goarch}.zip"
      expected_path="install_path=/tmp/leia-bin/leia.exe"
      ;;
    *)
      expected_asset="asset=leia_v0.0.0_${goos}_${goarch}.tar.gz"
      expected_path="install_path=/tmp/leia-bin/leia"
      ;;
  esac
  if ! grep -Fq -- "$expected_asset" <<<"$output"; then
    echo "error: install dry-run for $target did not plan $expected_asset" >&2
    echo "$output" >&2
    exit 1
  fi
  if ! grep -Fq -- "$expected_path" <<<"$output"; then
    echo "error: install dry-run for $target did not plan $expected_path" >&2
    echo "$output" >&2
    exit 1
  fi
done

echo "release_distribution_check.sh: install script dry-run matrix verified"

if command -v goreleaser >/dev/null 2>&1; then
  goreleaser check
else
  echo "release_distribution_check.sh: goreleaser not installed; skipping local goreleaser check"
fi

echo "release_distribution_check.sh: pass"
