#!/usr/bin/env bash
# Check internal Markdown doc links and documented script entrypoints.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

usage() {
    cat <<'EOF'
Usage: scripts/docs_check.sh [--help]

Checks README/docs Markdown for:
  - relative .md links whose target file exists;
  - fenced code blocks that mention release scripts whose files exist and are executable.
  - non-archive docs do not reintroduce retired GScript-era names.
  - release-readiness docs keep machine-checkable language and AI-native gates.

The release-script check covers:
  scripts/production_check.sh
  scripts/performance_gate.sh
  scripts/diagnostics_bundle.sh
  scripts/release_artifacts.sh
EOF
}

while [ "$#" -gt 0 ]; do
    case "$1" in
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

if ! command -v python3 >/dev/null 2>&1; then
    echo "error: python3 is required for docs_check.sh" >&2
    exit 1
fi

TMP_DOCS="$(mktemp -d)"
trap 'rm -rf "$TMP_DOCS"' EXIT
go run ./cmd/leia doc generate --layout site --output "$TMP_DOCS" >/dev/null
for generated in reference/cli/index.md reference/stdlib/index.md; do
    if ! cmp -s "$TMP_DOCS/$generated" "docs/$generated"; then
        echo "error: docs/$generated is stale; run: go run ./cmd/leia doc generate --layout site --output docs" >&2
        exit 1
    fi
done

python3 - <<'PY'
import os
import re
import sys
from pathlib import Path
from urllib.parse import unquote

root = Path.cwd()
doc_files = [root / "README.md"]
doc_files.extend(
    path
    for path in sorted((root / "docs").rglob("*.md"))
    if "archive" not in path.relative_to(root / "docs").parts
)

link_re = re.compile(r"(?<!!)\[[^\]\n]*\]\(([^)\n]+)\)")
fence_re = re.compile(r"^\s*(```+|~~~+)")
script_names = {
    "production_check": root / "scripts" / "production_check.sh",
    "performance_gate": root / "scripts" / "performance_gate.sh",
    "diagnostics_bundle": root / "scripts" / "diagnostics_bundle.sh",
    "release_artifacts": root / "scripts" / "release_artifacts.sh",
}

errors = []
checked_links = 0
checked_script_mentions = 0
checked_release_gate_docs = 0
checked_retired_paths = 0
checked_retired_names = 0

retired_paths = {
    "docs/language-spec.md": "docs/spec/language.md",
    "docs/stdlib-contract.md": "docs/reference/stdlib/index.md",
    "docs/test-matrix.md": "docs/testing.md",
    "docs/testing-matrix.md": "docs/testing.md",
    "docs/embedding.md": "docs/guides/embedding.md",
    "docs/tooling.md": "docs/guides/tooling.md",
    "docs/release.md": "docs/release/index.md",
    "docs/production-readiness-checklist.md": "docs/release/index.md",
    "docs/stdlib.md": "docs/reference/stdlib/index.md",
}

retired_name_re = re.compile(r"\bGScript\b|\bgscript\b|\.gs\b|//gs:")


def strip_link_destination(raw: str) -> str:
    raw = raw.strip()
    if not raw:
        return raw
    if raw.startswith("<"):
        end = raw.find(">")
        if end != -1:
            return raw[1:end]
    return raw.split()[0]


def is_external(target: str) -> bool:
    return bool(re.match(r"^[a-zA-Z][a-zA-Z0-9+.-]*:", target)) or target.startswith("//")


def check_markdown_links(path: Path) -> None:
    global checked_links
    text = path.read_text(encoding="utf-8")
    rel_dir = path.parent

    for line_no, line in enumerate(text.splitlines(), start=1):
        for match in link_re.finditer(line):
            target = strip_link_destination(match.group(1))
            if not target or is_external(target) or target.startswith("#"):
                continue

            target_no_anchor = target.split("#", 1)[0]
            target_no_query = target_no_anchor.split("?", 1)[0]
            if not target_no_query.endswith(".md"):
                continue

            checked_links += 1
            resolved = (rel_dir / unquote(target_no_query)).resolve()
            try:
                resolved.relative_to(root)
            except ValueError:
                errors.append(
                    f"{path.relative_to(root)}:{line_no}: markdown link escapes repo: {target}"
                )
                continue
            if not resolved.is_file():
                errors.append(
                    f"{path.relative_to(root)}:{line_no}: missing markdown link target: {target}"
                )


def check_retired_paths(path: Path) -> None:
    global checked_retired_paths
    text = path.read_text(encoding="utf-8")
    rel = path.relative_to(root)
    for old, new in retired_paths.items():
        if old not in text:
            continue
        checked_retired_paths += 1
        errors.append(f"{rel}: references retired docs path {old}; use {new}")


def check_retired_names(path: Path) -> None:
    global checked_retired_names
    text = path.read_text(encoding="utf-8")
    rel = path.relative_to(root)
    for line_no, line in enumerate(text.splitlines(), start=1):
        if not retired_name_re.search(line):
            continue
        checked_retired_names += 1
        errors.append(
            f"{rel}:{line_no}: references retired GScript-era naming; use Leia naming"
        )


def check_script_mentions(path: Path) -> None:
    global checked_script_mentions
    in_fence = False
    fence_start = 0
    block = []

    def flush(end_line: int) -> None:
        global checked_script_mentions
        content = "\n".join(block)
        for name, script_path in script_names.items():
            if name not in content and script_path.name not in content:
                continue
            checked_script_mentions += 1
            rel_script = script_path.relative_to(root)
            if not script_path.is_file():
                errors.append(
                    f"{path.relative_to(root)}:{fence_start}-{end_line}: missing script referenced in code block: {rel_script}"
                )
            elif not os.access(script_path, os.X_OK):
                errors.append(
                    f"{path.relative_to(root)}:{fence_start}-{end_line}: script referenced in code block is not executable: {rel_script}"
                )

    for line_no, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        if fence_re.match(line):
            if in_fence:
                flush(line_no)
                block = []
                in_fence = False
            else:
                in_fence = True
                fence_start = line_no
                block = []
            continue
        if in_fence:
            block.append(line)

    if in_fence:
        errors.append(f"{path.relative_to(root)}:{fence_start}: unclosed fenced code block")


def require_snippets(path: Path, snippets: list[str]) -> None:
    global checked_release_gate_docs
    text = path.read_text(encoding="utf-8")
    checked_release_gate_docs += 1
    for snippet in snippets:
        if snippet not in text:
            errors.append(
                f"{path.relative_to(root)}: missing release-readiness snippet: {snippet}"
            )


def check_release_gate_docs() -> None:
    release_matrix_cmd = "go test ./tests -run 'TestFeatureMatrixSchema|TestReleaseMatrix' -count=1"
    require_snippets(
        root / "docs" / "release" / "index.md",
        [
            "## Machine-Checkable Release Evidence",
            release_matrix_cmd,
            "bash scripts/performance_gate.sh --feature-smoke",
            "tests/feature_matrix.json",
            "docs/spec/language.md",
            "tests/language/MISSING_CAPABILITIES.md",
            "docs/reference/stdlib/index.md",
        ],
    )
    require_snippets(
        root / "docs" / "release" / "index.md",
        [
            "## Machine-Checkable Release Evidence",
            "bash scripts/production_check.sh --quick",
            release_matrix_cmd,
            "scripts/docs_check.sh",
            "bash scripts/performance_gate.sh --feature-smoke",
            "tests/language/MANIFEST.md",
            "tests/language/KNOWN_FAILURES.md",
        ],
    )


for doc_file in doc_files:
    if doc_file.is_file():
        check_markdown_links(doc_file)
        check_script_mentions(doc_file)
        check_retired_paths(doc_file)
        check_retired_names(doc_file)

check_release_gate_docs()

if errors:
    print("docs_check.sh found problems:", file=sys.stderr)
    for error in errors:
        print(f"  - {error}", file=sys.stderr)
    sys.exit(1)

print(
    f"docs_check.sh: checked {len(doc_files)} Markdown files, "
    f"{checked_links} relative .md links, "
    f"{checked_script_mentions} release-script code-block mentions, "
    f"{checked_release_gate_docs} release-gate docs, "
    f"{checked_retired_paths} retired-path mentions, "
    f"{checked_retired_names} retired-name mentions, "
    "2 generated reference docs."
)
PY
