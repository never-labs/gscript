#!/usr/bin/env bash
# Check internal Markdown doc links and documented script entrypoints.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

usage() {
    cat <<'EOF'
Usage: scripts/docs_check.sh [--help]

Checks README/docs Markdown for:
  - relative .md/.html links whose target file exists;
  - fenced code blocks that mention repository gate scripts whose files exist and are executable.
  - non-archive docs do not reintroduce retired project names.
  - release-readiness docs keep machine-checkable language and AI dialect gates.
  - README spec link and docs/spec stability contract stay synchronized.
  - docs/spec runnable Leia examples use stable all-mode fence tags and execute.
  - docs examples index lists each registered top-level example directory and
    keeps documented examples CLI selectors registered.
  - README concise examples stay tied to detailed docs, manifests, and focused gates.
  - README Surface, References, and Embedding snippets keep focused execution gates.
  - reference entrypoints stay linked from the docs home.
  - generated reference docs and the checked-in language spec HTML are fresh.

The repository-script mention check covers:
  scripts/production_check.sh
  scripts/performance_gate.sh
  scripts/diagnostics_bundle.sh
  scripts/docs_check.sh
  scripts/editor_check.sh
  scripts/public_release_blockers_check.sh
  scripts/release_notes_check.sh
  scripts/release_artifacts.sh
  scripts/release_artifacts_check.sh
  scripts/release_distribution_check.sh
  scripts/worktree_audit.sh
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
if ! command -v go >/dev/null 2>&1; then
    echo "error: go is required for docs_check.sh" >&2
    exit 1
fi

TMP_DOCS="$(mktemp -d)"
trap 'rm -rf "$TMP_DOCS"' EXIT
go run ./cmd/leia doc generate --layout site --output "$TMP_DOCS" >/dev/null
generated_reference_count=0
while IFS= read -r generated; do
    generated_doc="docs/$generated"
    generated_reference_count=$((generated_reference_count + 1))
    if [ ! -f "$generated_doc" ]; then
        echo "error: generated reference doc is missing from docs: $generated_doc; run: go run ./cmd/leia doc generate --layout site --output docs" >&2
        exit 1
    fi
    if ! cmp -s "$TMP_DOCS/$generated" "$generated_doc"; then
        echo "error: $generated_doc is stale; run: go run ./cmd/leia doc generate --layout site --output docs" >&2
        exit 1
    fi
done < <(cd "$TMP_DOCS" && find reference -type f -name '*.md' | sort)
if [ "$generated_reference_count" -eq 0 ]; then
    echo "error: docs generator produced no reference Markdown output" >&2
    exit 1
fi
export GENERATED_REFERENCE_COUNT="$generated_reference_count"
TMP_SPEC_DIR="$TMP_DOCS/spec"
mkdir -p "$TMP_SPEC_DIR"
cp docs/spec/*.md docs/spec/grammar.ebnf "$TMP_SPEC_DIR/"
python3 scripts/spec_preview.py --spec-dir "$TMP_SPEC_DIR" --write-index --output "$TMP_DOCS/spec-preview.html" >/dev/null
if ! cmp -s "$TMP_SPEC_DIR/index.md" "docs/spec/index.md"; then
    echo "error: docs/spec/index.md is stale; run: python3 scripts/spec_preview.py --write-index --output docs/spec/index.html" >&2
    exit 1
fi
if [ ! -f "docs/_layouts/spec.html" ]; then
    echo "error: docs/spec/index.md uses the spec layout, but docs/_layouts/spec.html is missing" >&2
    exit 1
fi
if ! grep -Fq "layout: spec" docs/spec/index.md; then
    echo "error: docs/spec/index.md must use layout: spec so GitHub Pages renders the long-form spec page" >&2
    exit 1
fi
if [ ! -f "docs/_layouts/home.html" ]; then
    echo "error: docs/index.md uses the home layout, but docs/_layouts/home.html is missing" >&2
    exit 1
fi
if [ ! -f "docs/_layouts/default.html" ] || [ ! -f "docs/_layouts/page.html" ]; then
    echo "error: docs must override Jekyll default/page layouts so reference pages do not use the Minima auto-navigation header" >&2
    exit 1
fi
if ! grep -Fq "layout: home" docs/index.md; then
    echo "error: docs/index.md must use layout: home so GitHub Pages does not render the default Minima page header" >&2
    exit 1
fi
if grep -R "guides/embedding/" docs/_layouts >/dev/null; then
    echo "error: docs top navigation must link guides/embedding.html; guides/embedding/ is not a generated Pages path" >&2
    exit 1
fi
if [ ! -s "$TMP_DOCS/spec-preview.html" ]; then
    echo "error: spec preview generator produced no output" >&2
    exit 1
fi
if ! cmp -s "$TMP_DOCS/spec-preview.html" "docs/spec/index.html"; then
    echo "error: docs/spec/index.html is stale; run: python3 scripts/spec_preview.py --write-index --output docs/spec/index.html" >&2
    exit 1
fi
if ! grep -Fq "spec/index.html" docs/_config.yml; then
    echo "error: docs/spec/index.html is a checked-in local preview; docs/_config.yml must exclude it from GitHub Pages" >&2
    exit 1
fi
python3 - <<'PY'
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import urldefrag, urlparse

root = Path.cwd()
html_path = root / "docs" / "spec" / "index.html"

class LinkParser(HTMLParser):
    def __init__(self):
        super().__init__()
        self.ids = set()
        self.hrefs = []

    def handle_starttag(self, tag, attrs):
        values = dict(attrs)
        if "id" in values:
            self.ids.add(values["id"])
        if tag == "a" and "href" in values:
            self.hrefs.append(values["href"])

parser = LinkParser()
parser.feed(html_path.read_text(encoding="utf-8"))

errors = []
for href in parser.hrefs:
    parsed = urlparse(href)
    if parsed.scheme or parsed.netloc:
        continue
    path, fragment = urldefrag(href)
    if fragment and not path and fragment not in parser.ids:
        errors.append(f"{html_path.relative_to(root)}: missing local anchor #{fragment}")
    if not path:
        continue
    target = (html_path.parent / path).resolve()
    try:
        target.relative_to(root.resolve())
    except ValueError:
        errors.append(f"{html_path.relative_to(root)}: link escapes repository: {href}")
        continue
    if not target.exists():
        errors.append(f"{html_path.relative_to(root)}: missing linked file: {href}")

if errors:
    for error in errors:
        print(f"error: {error}")
    raise SystemExit(1)
PY

if ! go test ./tests -run 'TestSpecRunnableExamples|TestSpecLeiaCodeFencesAreExecutableOrExplicitlyNonExecutable' -count=1; then
    echo "error: docs/spec runnable Leia example gate failed" >&2
    exit 1
fi
if ! go test ./tests/docs/spec -count=1; then
    echo "error: docs/spec contract gate failed" >&2
    exit 1
fi
if ! go test ./tests -run 'TestReleaseMatrixFeatureDocsStayCoveredBySpecAndReference|TestReleaseMatrixDocsIndexCoversReferenceEntrypoints|TestReleaseMatrixReadmeReferencesEntrypointsStayGated' -count=1; then
    echo "error: docs release/spec reference gate failed" >&2
    exit 1
fi
if ! go test ./cmd/leia -run 'TestExamplesDocsIndexCoversTopLevelExampleDirectories|TestExamplesDocsIndexCommandsReferenceRegisteredExamples' -count=1; then
    echo "error: docs examples index gate failed" >&2
    exit 1
fi

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
fence_re = re.compile(r"^\s*(`{3,}|~{3,})")
script_names = {
    "production_check": root / "scripts" / "production_check.sh",
    "performance_gate": root / "scripts" / "performance_gate.sh",
    "diagnostics_bundle": root / "scripts" / "diagnostics_bundle.sh",
    "docs_check": root / "scripts" / "docs_check.sh",
    "editor_check": root / "scripts" / "editor_check.sh",
    "public_release_blockers_check": root / "scripts" / "public_release_blockers_check.sh",
    "release_notes_check": root / "scripts" / "release_notes_check.sh",
    "release_artifacts": root / "scripts" / "release_artifacts.sh",
    "release_artifacts_check": root / "scripts" / "release_artifacts_check.sh",
    "release_distribution_check": root / "scripts" / "release_distribution_check.sh",
    "worktree_audit": root / "scripts" / "worktree_audit.sh",
}

errors = []
checked_links = 0
checked_script_mentions = 0
checked_release_gate_docs = 0
checked_retired_paths = 0
checked_retired_names = 0
checked_spec_runnable_examples = 0
checked_spec_contract_docs = 0
checked_examples_index_dirs = 0
checked_examples_capability_drift_gates = 0
checked_readme_user_facing_gates = 0
checked_readme_reference_entrypoints = 0
checked_reference_entrypoints = 0
spec_runnable_report = ""

retired_paths = {
    "docs/language-spec.md": "docs/spec/index.md",
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
            if not (target_no_query.endswith(".md") or target_no_query.endswith(".html")):
                continue

            checked_links += 1
            resolved = (rel_dir / unquote(target_no_query)).resolve()
            try:
                resolved.relative_to(root)
            except ValueError:
                errors.append(
                    f"{path.relative_to(root)}:{line_no}: documentation link escapes repo: {target}"
                )
                continue
            if not resolved.is_file():
                errors.append(
                    f"{path.relative_to(root)}:{line_no}: missing documentation link target: {target}"
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
        errors.append(f"{rel}:{line_no}: references retired project naming; use Leia naming")


def check_script_mentions(path: Path) -> None:
    global checked_script_mentions
    in_fence = False
    fence_start = 0
    fence_char = ""
    fence_len = 0
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
        match = fence_re.match(line)
        if match:
            marker = match.group(1)
            if in_fence:
                if marker[0] == fence_char and len(marker) >= fence_len:
                    flush(line_no)
                    block = []
                    in_fence = False
            else:
                in_fence = True
                fence_start = line_no
                fence_char = marker[0]
                fence_len = len(marker)
                block = []
            continue
        if in_fence:
            block.append(line)

    if in_fence:
        errors.append(f"{path.relative_to(root)}:{fence_start}: unclosed fenced code block")


def read_readme_reference_entrypoints() -> list[str]:
    readme = root / "README.md"
    text = readme.read_text(encoding="utf-8")
    start = text.find("## References")
    if start == -1:
        errors.append("README.md: missing References section")
        return []
    end = text.find("\n## ", start + len("## References"))
    section = text[start:] if end == -1 else text[start:end]

    refs = []
    for line in section.splitlines():
        if not line.startswith("- "):
            continue
        match = link_re.search(line)
        if not match:
            continue
        target = strip_link_destination(match.group(1))
        if not target or is_external(target) or target.startswith("#"):
            continue
        target = target.split("#", 1)[0].split("?", 1)[0]
        if target:
            refs.append(target)
    return refs


def check_readme_reference_entrypoints() -> None:
    global checked_readme_reference_entrypoints
    refs = read_readme_reference_entrypoints()
    if not refs:
        errors.append("README.md References section must list documentation entrypoints")
        return
    for ref in refs:
        checked_readme_reference_entrypoints += 1
        resolved = (root / unquote(ref)).resolve()
        try:
            resolved.relative_to(root)
        except ValueError:
            errors.append(f"README.md References entrypoint escapes repo: {ref}")
            continue
        if not resolved.is_file():
            errors.append(f"README.md References entrypoint target is missing: {ref}")


def check_reference_entrypoints() -> None:
    global checked_reference_entrypoints
    docs_index = root / "docs" / "index.md"
    text = docs_index.read_text(encoding="utf-8")
    for path in sorted((root / "docs" / "reference").glob("*/index.md")):
        checked_reference_entrypoints += 1
        ref = path.relative_to(root / "docs").as_posix()
        if f"({ref})" not in text:
            errors.append(f"docs/index.md: missing reference entrypoint link: {ref}")


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
    release_matrix_cmd = "go test ./tests -run 'TestReleaseMatrix' -count=1"
    release_profile_cmd = "bash scripts/production_check.sh --full --release-profile --release-version vX.Y.Z"
    release_distribution_cmd = "bash scripts/release_distribution_check.sh --require-goreleaser --require-workflows"
    spec_examples_cmd = "go test ./tests -run 'TestSpecRunnableExamples|TestSpecLeiaCodeFencesAreExecutableOrExplicitlyNonExecutable' -count=1"
    require_snippets(
        root / "docs" / "release" / "index.md",
        [
            "## Machine-Checkable Release Evidence",
            "go run ./cmd/leia ci release --list",
            release_profile_cmd,
            "profile is the release validation source of truth",
            "q conformance",
            "local artifact installation evidence",
            "feature coverage records under `tests/`",
            "docs/spec/index.md",
            "tests/language/MISSING_CAPABILITIES.md",
            "docs/reference/stdlib/index.md",
            "docs/reference/security/index.md",
            "docs/reference/performance/index.md",
            "docs/reference/platforms/index.md",
            "choose a license and add a root `LICENSE` file",
            "docs/release/notes-template.md",
            "docs/release/decisions.md",
        ],
    )
    require_snippets(
        root / "docs" / "release" / "index.md",
        [
            "## Machine-Checkable Release Evidence",
            "go run ./cmd/leia ci release --list",
            release_profile_cmd,
            "scripts/docs_check.sh",
            release_distribution_cmd,
            "tests/language/MANIFEST.md",
            "tests/language/KNOWN_FAILURES.md",
            "docs/reference/hot-reload/index.md",
            "docs/reference/ai/index.md",
            "docs/reference/data-oriented/index.md",
            "examples/README.md",
            "scripts/public_release_blockers_check.sh",
            "state tested platforms and execution modes",
            "complete the release decisions recorded in `docs/release/decisions.md`",
        ],
    )
    require_snippets(
        root / "docs" / "release" / "decisions.md",
        [
            "Public releases require explicit maintainer decisions",
            "## Required Before Public Release",
            "| Area | Decision Needed | Current Status |",
            "| License | Choose the repository license",
            "| Security reporting | Confirm the private reporting route",
            "| Platform support | Define tested and supported OS/architecture combinations",
            "| Release channels | Decide which channels are public",
            "| Artifact signing | Decide whether SHA256 checksums are sufficient",
            "| Compatibility policy | Define the pre-1.0 compatibility promise",
            "The repository has no selected license until a root `LICENSE` file exists.",
            "whether GitHub private security advisories are enabled",
            "tested OS/architecture combinations",
            "whether `scripts/install.sh` is a supported install path",
            "whether SHA256 checksums are sufficient",
            "checksum and signing requirements",
            "Optimizations, JIT availability, typed kernels, and provider integrations are not compatibility guarantees",
        ],
    )
    require_snippets(
        root / "docs" / "testing.md",
        [
            "Runnable examples embedded in `docs/spec/*.md`",
            release_matrix_cmd,
            spec_examples_cmd,
            "bash scripts/docs_check.sh",
            "feature coverage records under `tests/`",
            "docs/spec/index.md",
        ],
    )


def check_spec_runnable_coverage() -> None:
    global checked_spec_runnable_examples, spec_runnable_report
    spec_dir = root / "docs" / "spec"
    by_file = {}
    files_without_examples = []

    for path in sorted(spec_dir.glob("*.md")):
        if path.name == "index.md":
            files_without_examples.append(path.name)
            continue
        run = 0
        fail = 0
        for line in path.read_text(encoding="utf-8").splitlines():
            if not line.startswith("```"):
                continue
            info = line[3:].strip()
            if info == "leia run all":
                run += 1
            elif info == "leia fail all":
                fail += 1
        if run or fail:
            by_file[path.name] = {"run": run, "fail": fail}
            checked_spec_runnable_examples += run + fail
        else:
            files_without_examples.append(path.name)

    if checked_spec_runnable_examples == 0:
        errors.append("docs/spec must contain at least one runnable Leia example fence")
        return

    total_run = sum(item["run"] for item in by_file.values())
    total_fail = sum(item["fail"] for item in by_file.values())
    lines = [
        (
            "docs/spec runnable Leia coverage: "
            f"{checked_spec_runnable_examples} examples "
            f"({total_run} run, {total_fail} fail) across {len(by_file)} files"
        )
    ]
    lines.extend(
        f"  {name}: {counts['run']} run, {counts['fail']} fail"
        for name, counts in by_file.items()
    )
    if files_without_examples:
        lines.append(
            "  no runnable examples: " + ", ".join(files_without_examples)
        )
    spec_runnable_report = "\n".join(lines)


def check_spec_contract_docs() -> None:
    global checked_spec_contract_docs
    docs_check = (root / "scripts" / "docs_check.sh").read_text(encoding="utf-8")
    readme = (root / "README.md").read_text(encoding="utf-8")
    spec_index = (root / "docs" / "spec" / "index.md").read_text(encoding="utf-8")
    for path, text, snippets in [
        (
            "scripts/docs_check.sh",
            docs_check,
            [
                "go test ./tests/docs/spec -count=1",
                "README spec link and docs/spec stability contract",
            ],
        ),
        (
            "README.md",
            readme,
            [
                "[Language specification](docs/spec/index.md)",
                "Leia is an efficient, embeddable scripting language for Go, combining a LuaJIT-class execution model, q-style high-throughput in-memory columnar analytics, and first-class extensible domain dialects.",
                "## References",
            ],
        ),
        (
            "docs/spec/index.md",
            spec_index,
            [
                "## Stability Contract",
                "`tests/feature_matrix.json`",
                "at least one semantic or conformance gate",
                "release notes or migration notes",
                "must not be advertised as stable",
            ],
        ),
    ]:
        checked_spec_contract_docs += 1
        for snippet in snippets:
            if snippet not in text:
                errors.append(f"{path}: missing spec/stable-contract snippet: {snippet}")


def check_examples_index() -> None:
    global checked_examples_index_dirs
    examples_dir = root / "examples"
    examples_index = root / "docs" / "examples" / "index.md"
    text = examples_index.read_text(encoding="utf-8")
    indexed_dirs = set(re.findall(r"`examples/([^`/]+)/`", text))
    registered_dirs = set()
    for path in sorted(examples_dir.iterdir()):
        if not path.is_dir():
            continue
        has_registered_source = any(
            child.suffix in {".leia", ".go"} for child in path.rglob("*") if child.is_file()
        )
        if not has_registered_source:
            continue
        registered_dirs.add(path.name)
        checked_examples_index_dirs += 1
        snippet = f"`examples/{path.name}/`"
        if snippet not in text:
            errors.append(
                f"{examples_index.relative_to(root)}: missing example directory index row: {snippet}"
            )
    for name in sorted(indexed_dirs - registered_dirs):
        errors.append(
            f"{examples_index.relative_to(root)}: indexes missing or unregistered example directory: `examples/{name}/`"
        )


def check_examples_capability_drift_gates() -> None:
    global checked_examples_capability_drift_gates
    for path, snippets in [
        (
            root / "tests" / "release_matrix_test.go",
            [
                "TestReleaseMatrixReadmeCapabilitiesStayCoveredByExamples",
                "TestReleaseMatrixReadmeCapabilitiesStayCoveredByExamples",
                "leia examples list --json",
                "TestExamplesCommandManifestMatchesPlaygroundRepositoryExamples",
                "TestPlaygroundRepositoryAIDialectExamplesHaveExplicitGates",
            ],
        ),
        (
            root / "cmd" / "leia" / "main_examples_command_test.go",
            [
                "TestExamplesCommandManifestMatchesPlaygroundRepositoryExamples",
                "CLI examples list is missing playground repository example",
                "manual/check-only in the CLI manifest but has no requires reason",
            ],
        ),
        (
            root / "cmd" / "leia" / "main_playground_test.go",
            [
                "TestPlaygroundRepositoryCoreExampleCoverage",
                "TestPlaygroundRepositoryAIDialectExamplesHaveExplicitGates",
                "TestPlaygroundRepositoryGameEngineExampleClassification",
            ],
        ),
    ]:
        checked_examples_capability_drift_gates += 1
        text = path.read_text(encoding="utf-8")
        for snippet in snippets:
            if snippet not in text:
                errors.append(
                    f"{path.relative_to(root)}: missing examples capability drift gate snippet: {snippet}"
                )


def check_readme_user_facing_gates() -> None:
    global checked_readme_user_facing_gates
    for path, snippets in [
        (
            root / "cmd" / "leia" / "main_readme_tooling_test.go",
            [
                "TestReadmeIntroStaysFocused",
                "Leia is an efficient, embeddable scripting language for Go, combining a LuaJIT-class execution model, q-style high-throughput in-memory columnar analytics, and first-class extensible domain dialects.",
                "Performance-oriented:",
                "LuaJIT-class workloads",
                "Analytics-native:",
                "q-style vector syntax",
                "## References",
                "TestReadmeMainLeiaExampleStaysRunnableToProviderBoundary",
                "readmeFirstLeiaSnippet",
                "README Leia example failed",
                "want 42 fallback without host LLM provider",
                'exec.Command("go", "run", "./cmd/leia", "run", file)',
                "TestReadmeEmbeddingSnippetStaysRunnable",
                "readmeEmbeddingGoSnippet",
                "README embedding snippet failed",
                "README embedding snippet stdout",
                'exec.Command("go", "run", "-mod=mod", ".")',
                "replace github.com/never-labs/leia => ",
            ],
        ),
        (
            root / "tests" / "release_matrix_test.go",
            [
                "TestReleaseMatrixReadmeUserFacingSnippetsHaveFocusedGate",
                "readReleaseReadmeSurfaceLeiaSnippet",
                "releaseToolingAuditCommands",
                "readReleaseReadmeEmbeddingGoSnippet",
                "README.md Example snippet changed or lost product surface",
                "tooling audit command must use `go run ./cmd/leia ...`",
                "cmd/leia/main_readme_tooling_test.go must keep README focused positioning gate",
                "cmd/leia/main_readme_tooling_test.go must keep README Surface focused gate",
                "cmd/leia/main_readme_tooling_test.go must keep README Embedding focused gate",
            ],
        ),
    ]:
        checked_readme_user_facing_gates += 1
        text = path.read_text(encoding="utf-8")
        for snippet in snippets:
            if snippet not in text:
                errors.append(
                    f"{path.relative_to(root)}: missing README user-facing gate snippet: {snippet}"
                )

for doc_file in doc_files:
    if doc_file.is_file():
        check_markdown_links(doc_file)
        check_script_mentions(doc_file)
        check_retired_paths(doc_file)
        check_retired_names(doc_file)

check_release_gate_docs()
check_readme_reference_entrypoints()
check_reference_entrypoints()
check_spec_runnable_coverage()
check_spec_contract_docs()
check_examples_index()
check_examples_capability_drift_gates()
check_readme_user_facing_gates()

if errors:
    print("docs_check.sh found problems:", file=sys.stderr)
    for error in errors:
        print(f"  - {error}", file=sys.stderr)
    sys.exit(1)

print(
    f"docs_check.sh: checked {len(doc_files)} Markdown files, "
    f"{checked_links} relative documentation links, "
    f"{checked_script_mentions} repository-script code-block mentions, "
    f"{checked_release_gate_docs} release-gate docs, "
    f"{checked_readme_reference_entrypoints} README References entrypoints, "
    f"{checked_reference_entrypoints} reference entrypoints, "
    f"{checked_spec_contract_docs} spec/stable-contract docs, "
    f"{checked_examples_index_dirs} examples index directories, "
    f"{checked_examples_capability_drift_gates} examples capability drift gates, "
    f"{checked_readme_user_facing_gates} README user-facing gates, "
    f"{checked_retired_paths} retired-path mentions, "
    f"{checked_retired_names} retired-name mentions, "
    f"{os.environ['GENERATED_REFERENCE_COUNT']} generated reference docs, "
    "1 generated spec HTML, "
    f"{checked_spec_runnable_examples} runnable spec examples."
)
print(spec_runnable_report)
PY
