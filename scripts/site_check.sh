#!/usr/bin/env bash
# Validate rendered static site output.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SITE_DIR="$ROOT/_site"
JSON=0

usage() {
  cat <<'USAGE'
Usage: scripts/site_check.sh [--site-dir DIR] [--json] [--help]

Checks rendered static site HTML for local link targets, local fragment anchors,
and local asset references. External URLs are not fetched.

Options:
  --site-dir DIR   Rendered site directory to inspect. Defaults to ./_site.
  --json           Print a machine-readable site report.
  -h, --help       Show this help.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --site-dir)
      if [[ $# -lt 2 || -z "$2" ]]; then
        echo "error: --site-dir requires a value" >&2
        usage >&2
        exit 2
      fi
      SITE_DIR="$2"
      shift 2
      ;;
    --json)
      JSON=1
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

args=(doc site-check --site-dir "$SITE_DIR")
if [ "$JSON" -eq 1 ]; then
  args+=(--json)
fi
exec go run ./cmd/leia "${args[@]}"
