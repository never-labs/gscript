#!/usr/bin/env bash
# Quick mechanical scan of methodjit architecture health.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
JIT="$ROOT/internal/methodjit"
json_out="false"

usage() {
  cat <<'USAGE'
Usage: scripts/arch_check.sh [--json] [--help]

Scans methodjit source size, pass-pipeline hints, debt markers, test gaps, and
module size summary.

Options:
  --json   Print a machine-readable architecture report
  -h, --help
           Show this help
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
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

source_files=()
test_files=()
top_file_lines=()
top_file_paths=()
large_file_lines=()
large_file_paths=()
large_file_severities=()
pass_pipeline_lines=()
debt_marker_paths=()
debt_marker_lines=()
debt_marker_texts=()
missing_test_files=()

source_file_count=0
source_line_count=0
test_file_count=0
test_line_count=0
test_ratio_pct=0

json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '%s' "$value"
}

relative_path() {
  local path="$1"
  printf '%s' "${path#"$ROOT"/}"
}

scan_architecture() {
  if [[ ! -d "$JIT" ]]; then
    return
  fi

  while IFS= read -r file; do
    [[ -n "$file" ]] && source_files+=("$file")
  done < <(find "$JIT" -name "*.go" ! -name "*_test.go" | sort)
  while IFS= read -r file; do
    [[ -n "$file" ]] && test_files+=("$file")
  done < <(find "$JIT" -name "*_test.go" | sort)
  source_file_count="${#source_files[@]}"
  test_file_count="${#test_files[@]}"

  local file lines entry base severity test_file
  local file_size_entries=()
  for file in "${source_files[@]}"; do
    lines="$(wc -l < "$file" | tr -d ' ')"
    source_line_count=$((source_line_count + lines))
    printf -v entry '%08d %s' "$lines" "$file"
    file_size_entries+=("$entry")
    if [[ "$lines" -gt 800 ]]; then
      severity="split"
      if [[ "$lines" -gt 1000 ]]; then
        severity="over_limit"
      fi
      large_file_lines+=("$lines")
      large_file_paths+=("$(relative_path "$file")")
      large_file_severities+=("$severity")
    fi
    base="$(basename "$file")"
    if [[ "$base" != "doc.go" ]]; then
      test_file="${file%.go}_test.go"
      if [[ ! -f "$test_file" ]]; then
        missing_test_files+=("$(relative_path "$file")")
      fi
    fi
  done

  for file in "${test_files[@]}"; do
    lines="$(wc -l < "$file" | tr -d ' ')"
    test_line_count=$((test_line_count + lines))
  done

  if [[ "$source_line_count" -gt 0 ]]; then
    test_ratio_pct="$((test_line_count * 100 / source_line_count))"
  fi

  local sorted_entry i=0
  while IFS= read -r sorted_entry; do
    [[ -z "$sorted_entry" ]] && continue
    lines="${sorted_entry%% *}"
    file="${sorted_entry#* }"
    lines="$((10#$lines))"
    top_file_lines+=("$lines")
    top_file_paths+=("$(relative_path "$file")")
    i=$((i + 1))
    [[ "$i" -ge 15 ]] && break
  done < <(printf '%s\n' "${file_size_entries[@]}" | sort -rn)

  if [[ -f "$JIT/tiering_manager.go" ]]; then
    while IFS= read -r line; do
      [[ -n "$line" ]] && pass_pipeline_lines+=("$line")
    done < <(grep -nE "Pass|Compile|Emit|Alloc|LICM|Inline|Range|Intrinsic" "$JIT/tiering_manager.go" 2>/dev/null | grep -v "^.*://" | head -20)
  fi

  local marker
  while IFS= read -r marker; do
    [[ -z "$marker" ]] && continue
    local marker_path="${marker%%:*}"
    local rest="${marker#*:}"
    local marker_line="${rest%%:*}"
    local marker_text="${rest#*:}"
    debt_marker_paths+=("$(relative_path "$marker_path")")
    debt_marker_lines+=("$marker_line")
    debt_marker_texts+=("$marker_text")
  done < <(grep -rn "TODO\|HACK\|FIXME\|workaround\|temporary" "$JIT"/*.go 2>/dev/null || true)
}

print_text_report() {
  echo "=== File Sizes (top 15, >800 flagged) ==="
  local i=0
  while [[ "$i" -lt "${#top_file_paths[@]}" ]]; do
    local lines="${top_file_lines[$i]}"
    local base
    base="$(basename "${top_file_paths[$i]}")"
    local flag=""
    [[ "$lines" -gt 800 ]] && flag=" ⚠ SPLIT"
    [[ "$lines" -gt 1000 ]] && flag=" 🚨 OVER LIMIT"
    printf "%6d  %s%s\n" "$lines" "$base" "$flag"
    i=$((i + 1))
  done

  echo ""
  echo "=== Pass Pipeline Order (from tiering_manager.go) ==="
  printf '%s\n' "${pass_pipeline_lines[@]}"

  echo ""
  echo "=== Technical Debt Markers ==="
  echo "Total TODO/HACK/FIXME/workaround: ${#debt_marker_paths[@]}"
  i=0
  while [[ "$i" -lt "${#debt_marker_paths[@]}" && "$i" -lt 10 ]]; do
    printf '%s:%s:%s\n' "${debt_marker_paths[$i]}" "${debt_marker_lines[$i]}" "${debt_marker_texts[$i]}"
    i=$((i + 1))
  done

  echo ""
  echo "=== Test Coverage Gaps (source files without _test.go) ==="
  for file in "${missing_test_files[@]}"; do
    printf '  MISSING: %s\n' "$(basename "$file")"
  done

  echo ""
  echo "=== Module Size Summary ==="
  echo "Source: ${source_file_count} files, ${source_line_count} lines"
  echo "Tests: ${test_line_count} lines"
  if [[ "$source_line_count" -gt 0 ]]; then
    echo "Test ratio: ${test_ratio_pct}%"
  else
    echo "Test ratio: N/A"
  fi
}

print_json_string_array() {
  local indent="$1"
  shift
  local values=("$@")
  printf '[\n'
  local i=0
  while [[ "$i" -lt "${#values[@]}" ]]; do
    printf '%s  "%s"' "$indent" "$(json_escape "${values[$i]}")"
    if [[ "$i" -lt $((${#values[@]} - 1)) ]]; then
      printf ','
    fi
    printf '\n'
    i=$((i + 1))
  done
  printf '%s]' "$indent"
}

print_json_file_details() {
  local indent="$1"
  printf '[\n'
  local i=0
  while [[ "$i" -lt "${#top_file_paths[@]}" ]]; do
    printf '%s  {"path": "%s", "lines": %d}' "$indent" "$(json_escape "${top_file_paths[$i]}")" "${top_file_lines[$i]}"
    if [[ "$i" -lt $((${#top_file_paths[@]} - 1)) ]]; then
      printf ','
    fi
    printf '\n'
    i=$((i + 1))
  done
  printf '%s]' "$indent"
}

print_json_large_file_details() {
  local indent="$1"
  printf '[\n'
  local i=0
  while [[ "$i" -lt "${#large_file_paths[@]}" ]]; do
    printf '%s  {"path": "%s", "lines": %d, "severity": "%s"}' "$indent" "$(json_escape "${large_file_paths[$i]}")" "${large_file_lines[$i]}" "$(json_escape "${large_file_severities[$i]}")"
    if [[ "$i" -lt $((${#large_file_paths[@]} - 1)) ]]; then
      printf ','
    fi
    printf '\n'
    i=$((i + 1))
  done
  printf '%s]' "$indent"
}

print_json_debt_marker_details() {
  local indent="$1"
  printf '[\n'
  local i=0
  while [[ "$i" -lt "${#debt_marker_paths[@]}" ]]; do
    printf '%s  {"path": "%s", "line": %d, "text": "%s"}' "$indent" "$(json_escape "${debt_marker_paths[$i]}")" "${debt_marker_lines[$i]}" "$(json_escape "${debt_marker_texts[$i]}")"
    if [[ "$i" -lt $((${#debt_marker_paths[@]} - 1)) ]]; then
      printf ','
    fi
    printf '\n'
    i=$((i + 1))
  done
  printf '%s]' "$indent"
}

print_json_report() {
  local status="pass"
  if [[ "${#large_file_paths[@]}" -gt 0 || "${#debt_marker_paths[@]}" -gt 0 || "${#missing_test_files[@]}" -gt 0 ]]; then
    status="issues"
  fi
  printf '{\n'
  printf '  "schema_version": 1,\n'
  printf '  "status": "%s",\n' "$status"
  printf '  "module": "%s",\n' "internal/methodjit"
  printf '  "source_file_count": %d,\n' "$source_file_count"
  printf '  "source_line_count": %d,\n' "$source_line_count"
  printf '  "test_file_count": %d,\n' "$test_file_count"
  printf '  "test_line_count": %d,\n' "$test_line_count"
  printf '  "test_ratio_pct": %d,\n' "$test_ratio_pct"
  printf '  "top_file_count": %d,\n' "${#top_file_paths[@]}"
  printf '  "large_file_count": %d,\n' "${#large_file_paths[@]}"
  printf '  "pass_pipeline_line_count": %d,\n' "${#pass_pipeline_lines[@]}"
  printf '  "debt_marker_count": %d,\n' "${#debt_marker_paths[@]}"
  printf '  "missing_test_count": %d,\n' "${#missing_test_files[@]}"
  printf '  "top_file_details": '
  print_json_file_details "  "
  printf ',\n'
  printf '  "large_file_details": '
  print_json_large_file_details "  "
  printf ',\n'
  printf '  "pass_pipeline_lines": '
  print_json_string_array "  " "${pass_pipeline_lines[@]}"
  printf ',\n'
  printf '  "debt_marker_details": '
  print_json_debt_marker_details "  "
  printf ',\n'
  printf '  "missing_test_files": '
  print_json_string_array "  " "${missing_test_files[@]}"
  printf '\n'
  printf '}\n'
}

scan_architecture

if [[ "$json_out" == "true" ]]; then
  print_json_report
else
  print_text_report
fi
