#!/bin/bash
# Official semantic-family performance coverage guard.
#
# Usage:
#   bash benchmarks/coverage_guard.sh
#   bash benchmarks/coverage_guard.sh --markdown /tmp/official_perf_coverage.md --json /tmp/official_perf_coverage.json

set -u
cd "$(dirname "$0")/.."
exec python3 benchmarks/official_perf_coverage.py --check "$@"
