#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd "$script_dir/.." && pwd -P)"
require_goreleaser="false"
require_workflows="false"
json_out="false"
workflow_files=()
install_targets=()
goreleaser_available="false"
local_install_fixture="pending"

usage() {
  cat <<'USAGE'
Usage: scripts/release_distribution_check.sh [--require-goreleaser] [--require-workflows] [--json] [--help]

Checks release distribution configuration and install-script planning.

Options:
  --require-goreleaser  Fail if the goreleaser CLI is not available locally
  --require-workflows   Fail if hosted release/distribution workflows are absent
  --json                Print a machine-readable distribution report
  -h, --help            Show this help
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --require-goreleaser)
      require_goreleaser="true"
      shift
      ;;
    --require-workflows)
      require_workflows="true"
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

print_json_string_array() {
  local indent="$1"
  shift
  local values=("$@")
  printf '[\n'
  local i=0
  while [[ "$i" -lt "${#values[@]}" ]]; do
    local value="${values[$i]}"
    printf '%s  "%s"' "$indent" "$(json_escape "$value")"
    if [[ "$i" -lt $((${#values[@]} - 1)) ]]; then
      printf ','
    fi
    printf '\n'
    i=$((i + 1))
  done
  printf '%s]' "$indent"
}

print_json_report() {
  printf '{\n'
  printf '  "schema_version": 1,\n'
  printf '  "status": "pass",\n'
  printf '  "require_goreleaser": %s,\n' "$require_goreleaser"
  printf '  "require_workflows": %s,\n' "$require_workflows"
  printf '  "goreleaser_available": %s,\n' "$goreleaser_available"
  printf '  "local_install_fixture": "%s",\n' "$local_install_fixture"
  printf '  "workflow_count": %d,\n' "${#workflow_files[@]}"
  printf '  "workflow_files": '
  print_json_string_array "  " "${workflow_files[@]}"
  printf ',\n'
  printf '  "install_target_count": %d,\n' "${#install_targets[@]}"
  printf '  "install_targets": '
  print_json_string_array "  " "${install_targets[@]}"
  printf '\n'
  printf '}\n'
}

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

check_local_install_fixture() {
  local version="v0.0.0-local"
  local tmp_dir
  tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/leia-install-fixture.XXXXXX")"
  local release_dir="$tmp_dir/release"
  local archive_dir="$tmp_dir/archive"
  local bin_dir="$tmp_dir/bin"
  mkdir -p "$release_dir" "$archive_dir" "$bin_dir"

  cleanup_local_fixture() {
    rm -rf "$tmp_dir"
  }
  trap cleanup_local_fixture RETURN

  printf '#!/usr/bin/env sh\nprintf "fixture leia\\n"\n' >"$archive_dir/leia"
  printf '#!/usr/bin/env sh\nprintf "fixture leia-lsp\\n"\n' >"$archive_dir/leia-lsp"
  chmod 0755 "$archive_dir/leia" "$archive_dir/leia-lsp"

  local asset="leia_${version}_linux_amd64.tar.gz"
  tar -C "$archive_dir" -czf "$release_dir/$asset" leia leia-lsp
  (
    cd "$release_dir"
    printf '%s  %s\n' "$(sha256_file "$asset")" "$asset" >SHA256SUMS
  )

  bash scripts/install.sh \
    --version "$version" \
    --os linux \
    --arch amd64 \
    --bin-dir "$bin_dir" \
    --base-url "file://$release_dir" >/dev/null

  if [[ ! -x "$bin_dir/leia" || ! -x "$bin_dir/leia-lsp" ]]; then
    echo "error: local install fixture did not install both executables" >&2
    exit 1
  fi
  if [[ "$("$bin_dir/leia")" != "fixture leia" ]]; then
    echo "error: installed leia fixture did not execute as expected" >&2
    exit 1
  fi
  if [[ "$("$bin_dir/leia-lsp")" != "fixture leia-lsp" ]]; then
    echo "error: installed leia-lsp fixture did not execute as expected" >&2
    exit 1
  fi

  local bad_release_dir="$tmp_dir/bad-release"
  local bad_bin_dir="$tmp_dir/bad-bin"
  mkdir -p "$bad_release_dir" "$bad_bin_dir"
  printf 'unexpected\n' >"$archive_dir/unexpected.txt"
  tar -C "$archive_dir" -czf "$bad_release_dir/$asset" leia leia-lsp unexpected.txt
  (
    cd "$bad_release_dir"
    printf '%s  %s\n' "$(sha256_file "$asset")" "$asset" >SHA256SUMS
  )
  if bash scripts/install.sh \
    --version "$version" \
    --os linux \
    --arch amd64 \
    --bin-dir "$bad_bin_dir" \
    --base-url "file://$bad_release_dir" >/dev/null 2>&1; then
    echo "error: install accepted archive with unexpected entry" >&2
    exit 1
  fi

  if command -v zip >/dev/null 2>&1 && command -v unzip >/dev/null 2>&1; then
    rm -rf "$release_dir" "$bin_dir"
    mkdir -p "$release_dir" "$bin_dir" "$archive_dir/windows"
    cp "$archive_dir/leia" "$archive_dir/windows/leia.exe"
    cp "$archive_dir/leia-lsp" "$archive_dir/windows/leia-lsp.exe"
    (
      cd "$archive_dir/windows"
      zip -q "$release_dir/leia_${version}_windows_amd64.zip" leia.exe leia-lsp.exe
    )
    (
      cd "$release_dir"
      printf '%s  %s\n' "$(sha256_file "leia_${version}_windows_amd64.zip")" "leia_${version}_windows_amd64.zip" >SHA256SUMS
    )
    bash scripts/install.sh \
      --version "$version" \
      --os windows \
      --arch amd64 \
      --bin-dir "$bin_dir" \
      --base-url "file://$release_dir" >/dev/null
    if [[ ! -x "$bin_dir/leia.exe" || ! -x "$bin_dir/leia-lsp.exe" ]]; then
      echo "error: local zip install fixture did not install both Windows executables" >&2
      exit 1
    fi
    rm -rf "$bad_release_dir" "$bad_bin_dir"
    mkdir -p "$bad_release_dir" "$bad_bin_dir" "$archive_dir/windows-bad"
    cp "$archive_dir/leia" "$archive_dir/windows-bad/leia.exe"
    cp "$archive_dir/leia-lsp" "$archive_dir/windows-bad/leia-lsp.exe"
    printf 'unexpected\n' >"$archive_dir/windows-bad/unexpected.txt"
    (
      cd "$archive_dir/windows-bad"
      zip -q "$bad_release_dir/leia_${version}_windows_amd64.zip" leia.exe leia-lsp.exe unexpected.txt
    )
    (
      cd "$bad_release_dir"
      printf '%s  %s\n' "$(sha256_file "leia_${version}_windows_amd64.zip")" "leia_${version}_windows_amd64.zip" >SHA256SUMS
    )
    if bash scripts/install.sh \
      --version "$version" \
      --os windows \
      --arch amd64 \
      --bin-dir "$bad_bin_dir" \
      --base-url "file://$bad_release_dir" >/dev/null 2>&1; then
      echo "error: install accepted zip archive with unexpected entry" >&2
      exit 1
    fi
  else
    log_info "release_distribution_check.sh: zip or unzip not installed; skipping local zip install fixture"
  fi

  trap - RETURN
  rm -rf "$tmp_dir"
  local_install_fixture="verified"
}

require_file .goreleaser.yaml
require_file scripts/install.sh

optional_workflow() {
  local file="$1"
  if [[ -f "$file" ]]; then
    workflow_files+=("$file")
    log_info "release_distribution_check.sh: found $file"
  elif [[ "$require_workflows" == "true" ]]; then
    echo "error: required hosted workflow not found: $file" >&2
    exit 1
  else
    echo "release_distribution_check.sh: $file not present; skipping hosted workflow check"
  fi
}

optional_workflow .github/workflows/release.yml
optional_workflow .github/workflows/distribution-check.yml
optional_workflow .github/workflows/pages.yml

if [[ -f .github/workflows/pages.yml ]]; then
  require_file docs/_config.yml
  require_contains docs/_config.yml "exclude:"
  require_contains docs/_config.yml "spec/index.html"
  require_contains .github/workflows/pages.yml "source: ./docs"
  require_contains .github/workflows/pages.yml "destination: ./_site"
fi

require_contains .goreleaser.yaml "version: 2"
require_contains .goreleaser.yaml "id: leia"
require_contains .goreleaser.yaml "main: ./cmd/leia"
require_contains .goreleaser.yaml "id: leia-lsp"
require_contains .goreleaser.yaml "main: ./cmd/leia-lsp"
require_contains .goreleaser.yaml "binary: leia-lsp"
require_contains .goreleaser.yaml "- darwin"
require_contains .goreleaser.yaml "- linux"
require_contains .goreleaser.yaml "- windows"
require_contains .goreleaser.yaml "- amd64"
require_contains .goreleaser.yaml "- arm64"
require_contains .goreleaser.yaml "name_template: \"{{ .ProjectName }}_{{ .Tag }}_{{ .Os }}_{{ .Arch }}\""
require_contains .goreleaser.yaml "- leia-lsp"
if [[ -f .github/workflows/release.yml ]]; then
  require_contains .github/workflows/release.yml "go install github.com/goreleaser/goreleaser/v2@v2.16.0"
  require_contains .github/workflows/release.yml "release tags must match vMAJOR.MINOR.PATCH"
  require_contains .github/workflows/release.yml '"$(go env GOPATH)/bin/goreleaser" --version'
  require_contains .github/workflows/release.yml 'bash scripts/release_notes_check.sh --require-ready --version "${GITHUB_REF_NAME}"'
  require_contains .github/workflows/release.yml '"$(go env GOPATH)/bin/goreleaser" release --clean'
  require_contains .github/workflows/release.yml '--release-notes "docs/release/notes/${GITHUB_REF_NAME}.md"'
fi
if [[ -f .github/workflows/distribution-check.yml ]]; then
  require_contains .github/workflows/distribution-check.yml "go install github.com/goreleaser/goreleaser/v2@v2.16.0"
  require_contains .github/workflows/distribution-check.yml "scripts/release_notes_check.sh"
  require_contains .github/workflows/distribution-check.yml "bash scripts/release_notes_check.sh"
  require_contains .github/workflows/distribution-check.yml '"$(go env GOPATH)/bin/goreleaser" --version'
  require_contains .github/workflows/distribution-check.yml '"$(go env GOPATH)/bin/goreleaser" release --snapshot --clean --skip=publish'
fi

bash -n scripts/install.sh

for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  goos="${target%/*}"
  goarch="${target#*/}"
  install_targets+=("$target")
  output="$(bash scripts/install.sh --dry-run --version v0.0.0 --os "$goos" --arch "$goarch" --bin-dir /tmp/leia-bin)"
  case "$goos" in
    windows)
      expected_asset="asset=leia_v0.0.0_${goos}_${goarch}.zip"
      expected_path="install_path=/tmp/leia-bin/leia.exe"
      expected_lsp_path="lsp_install_path=/tmp/leia-bin/leia-lsp.exe"
      ;;
    *)
      expected_asset="asset=leia_v0.0.0_${goos}_${goarch}.tar.gz"
      expected_path="install_path=/tmp/leia-bin/leia"
      expected_lsp_path="lsp_install_path=/tmp/leia-bin/leia-lsp"
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
  if ! grep -Fq -- "$expected_lsp_path" <<<"$output"; then
    echo "error: install dry-run for $target did not plan $expected_lsp_path" >&2
    echo "$output" >&2
    exit 1
  fi
done

log_info "release_distribution_check.sh: install script dry-run matrix verified"

check_local_install_fixture
log_info "release_distribution_check.sh: local install fixture verified"

if command -v goreleaser >/dev/null 2>&1; then
  goreleaser_available="true"
  if [[ "$json_out" == "true" ]]; then
    goreleaser check >/dev/null
  else
    goreleaser check
  fi
elif [[ "$require_goreleaser" == "true" ]]; then
  echo "error: goreleaser CLI is required for release distribution profile" >&2
  exit 1
else
  log_info "release_distribution_check.sh: goreleaser not installed; skipping local goreleaser check"
fi

if [[ "$json_out" == "true" ]]; then
  print_json_report
else
  echo "release_distribution_check.sh: pass"
fi
