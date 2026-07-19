#!/usr/bin/env bash
set -euo pipefail

goos="$(go env GOOS)"
goarch="$(go env GOARCH)"

go list ./... | while IFS= read -r package; do
  if [ "$package" = "github.com/never-labs/leia/internal/methodjit" ] &&
     { [ "$goos" != "darwin" ] || [ "$goarch" != "arm64" ]; }; then
    continue
  fi
  printf '%s\n' "$package"
done
