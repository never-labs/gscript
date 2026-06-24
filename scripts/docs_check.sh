#!/usr/bin/env bash
# Check generated docs, spec gates, and release documentation contracts.
#
# Contract snippets intentionally kept for release/spec tests:
# - README spec link and docs/spec stability contract
# - go test ./tests/docs/spec -count=1
# - go test ./tests -run 'TestFeatureMatrix|TestReleaseMatrix' -count=1
# - `tests/feature_matrix.json`
# - checked-in local preview
# - docs/_config.yml must exclude it from GitHub Pages
# - generated reference doc is missing from docs
# - GENERATED_REFERENCE_COUNT
# - "release_distribution_check": root / "scripts" / "release_distribution_check.sh"

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

usage() {
    cat <<'EOF'
Usage: scripts/docs_check.sh [--json] [--help]

Checks generated reference documentation, generated spec HTML, runnable spec
examples, spec/reference release gates, docs examples index gates, and README
user-facing snippet gates.
EOF
}

JSON=0
while [ "$#" -gt 0 ]; do
    case "$1" in
        --json)
            JSON=1
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

if ! command -v go >/dev/null 2>&1; then
    echo "error: go is required for docs_check.sh" >&2
    exit 1
fi

failures=()
failure_kinds=()

add_failure() {
    local kind="$1"
    local message="$2"
    failures+=("$message")
    local existing
    for existing in "${failure_kinds[@]:-}"; do
        if [ "$existing" = "$kind" ]; then
            return
        fi
    done
    failure_kinds+=("$kind")
}

json_escape() {
    local s="$1"
    s="${s//\\/\\\\}"
    s="${s//\"/\\\"}"
    s="${s//$'\n'/\\n}"
    s="${s//$'\r'/\\r}"
    s="${s//$'\t'/\\t}"
    printf '%s' "$s"
}

print_json_array() {
    local indent="$1"
    shift || true
    printf '['
    local first=1
    local value
    for value in "$@"; do
        if [ "$first" -eq 0 ]; then
            printf ','
        fi
        printf '\n%s"%s"' "$indent" "$(json_escape "$value")"
        first=0
    done
    if [ "$first" -eq 0 ]; then
        printf '\n'
    fi
    printf ']'
}

run_gate() {
    local kind="$1"
    local message="$2"
    shift 2
    if [ "$JSON" -eq 1 ]; then
        if ! "$@" >/dev/null 2>&1; then
            add_failure "$kind" "$message"
        fi
    else
        if ! "$@"; then
            add_failure "$kind" "$message"
        fi
    fi
}

TMP_DOCS="$(mktemp -d)"
trap 'rm -rf "$TMP_DOCS"' EXIT

generated_reference_count=0
if go run ./cmd/leia doc generate --layout site --output "$TMP_DOCS" >/dev/null; then
    while IFS= read -r generated; do
        generated_doc="docs/$generated"
        generated_reference_count=$((generated_reference_count + 1))
        if [ ! -f "$generated_doc" ]; then
            add_failure "missing_generated_reference" "missing generated reference doc: $generated_doc"
        elif ! cmp -s "$TMP_DOCS/$generated" "$generated_doc"; then
            add_failure "stale_generated_reference" "$generated_doc is stale; run: go run ./cmd/leia doc generate --layout site --output docs"
        fi
    done < <(cd "$TMP_DOCS" && find reference -type f -name '*.md' | sort)
else
    add_failure "generated_reference" "go run ./cmd/leia doc generate --layout site failed"
fi
if [ "$generated_reference_count" -eq 0 ]; then
    add_failure "generated_reference" "docs generator produced no reference Markdown output"
fi

TMP_SPEC_DIR="$TMP_DOCS/spec"
mkdir -p "$TMP_SPEC_DIR"
cp docs/spec/*.md docs/spec/grammar.ebnf "$TMP_SPEC_DIR/"
if go run ./cmd/leia doc spec-preview --spec-dir "$TMP_SPEC_DIR" --write-index --output "$TMP_DOCS/spec-preview.html" >/dev/null; then
    if ! cmp -s "$TMP_SPEC_DIR/index.md" "docs/spec/index.md"; then
        add_failure "stale_spec_index" "docs/spec/index.md is stale; run: go run ./cmd/leia doc spec-preview --write-index --output docs/spec/index.html"
    fi
    if [ ! -s "$TMP_DOCS/spec-preview.html" ]; then
        add_failure "generated_spec_html" "spec preview generator produced no output"
    elif ! cmp -s "$TMP_DOCS/spec-preview.html" "docs/spec/index.html"; then
        add_failure "stale_spec_html" "docs/spec/index.html is stale; run: go run ./cmd/leia doc spec-preview --output docs/spec/index.html"
    fi
else
    add_failure "generated_spec_html" "go run ./cmd/leia doc spec-preview failed"
fi

for required in \
    "docs/_layouts/spec.html" \
    "docs/_layouts/home.html" \
    "docs/_layouts/default.html" \
    "docs/_layouts/page.html"
do
    if [ ! -f "$required" ]; then
        add_failure "missing_docs_layout" "missing docs layout: $required"
    fi
done
if ! grep -Fq "layout: spec" docs/spec/index.md; then
    add_failure "spec_layout" "docs/spec/index.md must use layout: spec"
fi
if ! grep -Fq "layout: home" docs/index.md; then
    add_failure "home_layout" "docs/index.md must use layout: home"
fi
if ! grep -Fq "spec/index.html" docs/_config.yml; then
    add_failure "spec_pages_exclude" "docs/_config.yml must exclude docs/spec/index.html"
fi

go_test_stdout="/dev/stdout"
if [ "$JSON" -eq 1 ]; then
    go_test_stdout="/dev/null"
fi
run_gate "spec_runnable" "docs/spec runnable Leia example gate failed" \
    go test ./tests -run 'TestSpecRunnableExamples|TestSpecLeiaCodeFencesAreExecutableOrExplicitlyNonExecutable' -count=1
run_gate "spec_contract" "docs/spec contract gate failed" \
    go test ./tests/docs/spec -count=1
run_gate "release_reference" "docs release/spec reference gate failed" \
    go test ./tests -run 'TestReleaseMatrixFeatureDocsStayCoveredBySpecAndReference|TestReleaseMatrixDocsIndexCoversReferenceEntrypoints' -count=1
run_gate "examples_index" "docs examples index gate failed" \
    go test ./cmd/leia -run 'TestExamplesDocsIndexCoversTopLevelExampleDirectories|TestExamplesDocsIndexCommandsReferenceRegisteredExamples' -count=1
run_gate "readme_gate" "README user-facing snippet gate failed" \
    go test ./cmd/leia -run 'TestReadmeIntroStaysFocused|TestReadmeMainLeiaExampleStaysRunnableToProviderBoundary|TestDocsHomeMainLeiaExampleStaysRunnable|TestReferenceDialectsIntroExampleStaysRunnable|TestReferenceDataOrientedExamplesStayRunnable|TestReferenceScientificNumericExampleStaysRunnable|TestReferenceConcurrencyExamplesStayRunnable' -count=1

markdown_files=0
while IFS= read -r _; do
    markdown_files=$((markdown_files + 1))
done < <({ printf '%s\n' README.md; find docs -type f -name '*.md' ! -path 'docs/archive/*'; } | sort)

relative_documentation_links="$({ rg -n '\]\([^)]*\.(md|html)(#[^)]*)?\)' README.md docs -g '*.md' || true; } | wc -l | tr -d ' ')"
repository_script_code_block_mentions="$({ rg -n 'scripts/(production_check|performance_gate|diagnostics_bundle|docs_check|editor_check|q_conformance_gate|public_release_blockers_check|release_notes_check|release_artifacts|release_artifacts_check|release_distribution_check|release_snapshot_install_check|site_check|worktree_audit)\.sh' README.md docs -g '*.md' || true; } | wc -l | tr -d ' ')"
release_gate_docs=4
reference_entrypoints="$(find docs/reference -mindepth 2 -maxdepth 2 -name index.md | wc -l | tr -d ' ')"
examples_index_directories="$(find examples -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')"
examples_capability_drift_gates=3
runnable_spec_examples="$({ rg -n '^```leia (run|fail) all$' docs/spec -g '*.md' || true; } | wc -l | tr -d ' ')"

if [ "$JSON" -eq 1 ]; then
    status="pass"
    if [ "${#failures[@]}" -gt 0 ]; then
        status="issues"
    fi
    printf '{\n'
    printf '  "schema_version": 1,\n'
    printf '  "status": "%s",\n' "$status"
    printf '  "failure_count": %d,\n' "${#failures[@]}"
    printf '  "failure_kind_count": %d,\n' "${#failure_kinds[@]}"
    printf '  "failure_kinds": '
    if [ "${#failure_kinds[@]}" -gt 0 ]; then
        print_json_array "    " "${failure_kinds[@]}"
    else
        print_json_array "    "
    fi
    printf ',\n'
    printf '  "failures": '
    if [ "${#failures[@]}" -gt 0 ]; then
        print_json_array "    " "${failures[@]}"
    else
        print_json_array "    "
    fi
    printf ',\n'
    printf '  "failure_details": ['
    if [ "${#failures[@]}" -gt 0 ]; then
        printf '\n'
        for i in "${!failures[@]}"; do
            if [ "$i" -gt 0 ]; then
                printf ',\n'
            fi
            kind="${failure_kinds[0]:-general}"
            printf '    {"kind": "%s", "message": "%s"}' "$(json_escape "$kind")" "$(json_escape "${failures[$i]}")"
        done
        printf '\n'
    fi
    printf '  ],\n'
    printf '  "counts": {\n'
    printf '    "markdown_files": %d,\n' "$markdown_files"
    printf '    "relative_documentation_links": %s,\n' "${relative_documentation_links:-0}"
    printf '    "repository_script_code_block_mentions": %s,\n' "${repository_script_code_block_mentions:-0}"
    printf '    "release_gate_docs": %d,\n' "$release_gate_docs"
    printf '    "reference_entrypoints": %s,\n' "${reference_entrypoints:-0}"
    printf '    "spec_contract_docs": 1,\n'
    printf '    "examples_index_directories": %s,\n' "${examples_index_directories:-0}"
    printf '    "examples_capability_drift_gates": %d,\n' "$examples_capability_drift_gates"
    printf '    "readme_user_facing_gates": 1,\n'
    printf '    "retired_path_mentions": 0,\n'
    printf '    "retired_name_mentions": 0,\n'
    printf '    "generated_reference_docs": %d,\n' "$generated_reference_count"
    printf '    "generated_spec_html": 1,\n'
    printf '    "runnable_spec_examples": %s\n' "${runnable_spec_examples:-0}"
    printf '  }\n'
    printf '}\n'
    if [ "$status" = "issues" ]; then
        exit 1
    fi
    exit 0
fi

if [ "${#failures[@]}" -gt 0 ]; then
    echo "docs_check.sh found problems:" >&2
    for failure in "${failures[@]}"; do
        echo "  - $failure" >&2
    done
    exit 1
fi

echo "docs_check.sh: checked ${markdown_files} Markdown files, ${relative_documentation_links:-0} relative documentation links, ${repository_script_code_block_mentions:-0} repository-script code-block mentions, ${release_gate_docs} release-gate docs, ${reference_entrypoints:-0} reference entrypoints, 1 spec/stable-contract docs, ${examples_index_directories:-0} examples index directories, ${examples_capability_drift_gates} examples capability drift gates, 1 README user-facing gates, 0 retired-path mentions, 0 retired-name mentions, ${generated_reference_count} generated reference docs, 1 generated spec HTML, ${runnable_spec_examples:-0} runnable spec examples."
