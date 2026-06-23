#!/bin/bash
# Language conformance semantic-family performance coverage guard.
#
# Usage:
#   bash benchmarks/coverage_guard.sh
#   bash benchmarks/coverage_guard.sh --markdown /tmp/conformance_perf_coverage.md --json /tmp/conformance_perf_coverage.json

set -u
cd "$(dirname "$0")/.."
exec go run ./cmd/leia bench coverage --check "$@"
