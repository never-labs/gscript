#!/usr/bin/env python3
"""Smoke checks for editor sidecar assets."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]


def load_json(path: Path) -> dict:
    with path.open(encoding="utf-8") as handle:
        return json.load(handle)


def fail(message: str) -> None:
    print(f"editor_smoke.py: {message}", file=sys.stderr)
    raise SystemExit(1)


def repository_patterns(grammar: dict) -> list[dict]:
    patterns: list[dict] = []

    def walk(value: object) -> None:
        if isinstance(value, dict):
            if "name" in value and ("match" in value or "begin" in value):
                patterns.append(value)
            for child in value.values():
                walk(child)
        elif isinstance(value, list):
            for child in value:
                walk(child)

    walk(grammar.get("repository", {}))
    walk(grammar.get("patterns", []))
    return patterns


def pattern_by_name(grammar: dict, name: str) -> dict:
    for pattern in repository_patterns(grammar):
        if pattern.get("name") == name:
            return pattern
    fail(f"missing TextMate pattern {name}")


def has_pattern(grammar: dict, name: str) -> bool:
    return any(pattern.get("name") == name for pattern in repository_patterns(grammar))


def assert_match(grammar: dict, name: str, sample: str) -> None:
    pattern = pattern_by_name(grammar, name)
    regex = pattern.get("match") or pattern.get("begin")
    if not isinstance(regex, str):
        fail(f"pattern {name} has no regex")
    try:
        compiled = re.compile(regex, re.MULTILINE)
    except re.error as exc:
        fail(f"pattern {name} does not compile: {exc}")
    if not compiled.search(sample):
        fail(f"pattern {name} did not match {sample!r}")


def assert_any_match(grammar: dict, name: str, sample: str) -> None:
    for pattern in repository_patterns(grammar):
        pattern_name = pattern.get("name")
        capture_names = {
            capture.get("name")
            for captures_key in ("captures", "beginCaptures", "endCaptures")
            for capture in pattern.get(captures_key, {}).values()
            if isinstance(capture, dict)
        }
        if pattern_name != name and name not in capture_names:
            continue
        regex = pattern.get("match") or pattern.get("begin")
        if not isinstance(regex, str):
            continue
        try:
            compiled = re.compile(regex, re.MULTILINE)
        except re.error as exc:
            fail(f"pattern {name} does not compile: {exc}")
        if compiled.search(sample):
            return
    fail(f"no pattern {name} matched {sample!r}")


def assert_no_match(grammar: dict, name: str, sample: str) -> None:
    pattern = pattern_by_name(grammar, name)
    regex = pattern.get("match") or pattern.get("begin")
    if not isinstance(regex, str):
        fail(f"pattern {name} has no regex")
    try:
        compiled = re.compile(regex, re.MULTILINE)
    except re.error as exc:
        fail(f"pattern {name} does not compile: {exc}")
    if compiled.search(sample):
        fail(f"pattern {name} unexpectedly matched {sample!r}")


def assert_path(path: str) -> None:
    if not (ROOT / path).is_file():
        fail(f"missing path {path}")


def assert_js_array_literal(source: str, const_name: str, expected: list[str]) -> None:
    match = re.search(rf"const\s+{re.escape(const_name)}\s*=\s*(\[[^\]]*\])", source)
    if not match:
        fail(f"VS Code extension missing {const_name} array")
    try:
        value = json.loads(match.group(1))
    except json.JSONDecodeError as exc:
        fail(f"VS Code extension {const_name} array is not JSON-compatible: {exc}")
    if value != expected:
        fail(f"VS Code extension {const_name} = {value!r}, want {expected!r}")


def check_textmate() -> None:
    leia = load_json(ROOT / "tools/syntax/textmate/leia.tmLanguage.json")
    leia_mod = load_json(ROOT / "tools/syntax/textmate/leia-mod.tmLanguage.json")
    vscode_leia = load_json(ROOT / "editors/vscode/syntaxes/leia.tmLanguage.json")
    vscode_leia_mod = load_json(ROOT / "editors/vscode/syntaxes/leia-mod.tmLanguage.json")
    source = (ROOT / "tools/editor/smoke/fixtures/llm_surface.leia").read_text(encoding="utf-8")
    manifest = (ROOT / "tools/editor/smoke/fixtures/leia.mod").read_text(encoding="utf-8")

    if leia.get("scopeName") != "source.leia":
        fail("Leia TextMate grammar has the wrong scope")
    if leia_mod.get("scopeName") != "source.leia.mod":
        fail("leia.mod TextMate grammar has the wrong scope")
    if vscode_leia != leia:
        fail("VS Code Leia TextMate grammar drifted from tools/syntax/textmate/leia.tmLanguage.json")
    if vscode_leia_mod != leia_mod:
        fail("VS Code leia.mod TextMate grammar drifted from tools/syntax/textmate/leia-mod.tmLanguage.json")

    assert_match(leia, "keyword.control.directive.leia", source)
    assert_match(leia, "storage.type.function.leia", source)
    assert_match(leia, "meta.import.leia", 'import "go:net/http" as http')
    assert_match(leia, "meta.import.leia", 'import "json"')
    assert_match(leia, "meta.import.leia", 'import p "path"')
    assert_match(leia, "meta.import.group.leia", 'import (\n  "regexp"\n  fs "fs"\n)')
    assert_any_match(leia, "keyword.control.import.leia", 'import "go:net/http" as http')
    assert_any_match(leia, "string.quoted.double.import.path.leia", 'import "go:net/http" as http')
    assert_any_match(leia, "keyword.control.import.as.leia", 'import "go:net/http" as http')
    assert_any_match(leia, "entity.name.namespace.import.leia", 'import "go:net/http" as http')
    assert_any_match(leia, "entity.name.namespace.import.leia", 'import p "path"')
    if has_pattern(leia, "keyword.control.ai.leia"):
        fail("Leia TextMate grammar still exposes old AI-native keyword scope")
    assert_any_match(leia, "entity.name.tag.dialect.leia", "rows := csv`a,b\\n1,2\\n`")
    assert_match(leia, "meta.dialect.tagged-string.leia", "rows := csv`a,b\\n1,2\\n`")
    assert_match(leia, "meta.dialect.tagged-string.leia", "rows := csv!`a,b\\n1,2\\n`")
    assert_any_match(leia, "keyword.operator.raw.dialect.leia", "rows := csv!`a,b\\n1,2\\n`")
    assert_match(leia, "meta.dialect.shell.tagged-string.leia", "out := $`printf ok`")
    assert_any_match(leia, "entity.name.tag.shell.leia", "out := $`printf ok`")
    assert_match(leia, "meta.dialect.shell.tagged-string.leia", "out := $!`printf ok`")
    assert_any_match(leia, "keyword.operator.raw.dialect.leia", "out := $!`printf ok`")
    assert_any_match(leia, "meta.dialect.tagged-block.leia", 'prompt { role: "system" }')
    assert_match(leia, "meta.dialect.tagged-block.leia", 'prompt! { role: "system" }')
    assert_match(leia, "support.type.primitive.leia", source)
    assert_match(leia, "support.type.primitive.leia", "ids := [3]i64{1, 2, 3}")
    for unsupported in ("i8", "i16", "u8", "u16", "u32", "u64"):
        assert_no_match(leia, "support.type.primitive.leia", f"ids := [3]{unsupported}{{1, 2, 3}}")
    assert_match(leia, "constant.numeric.duration.leia", source)
    assert_match(leia, "keyword.operator.leia", source)
    operator_pattern = pattern_by_name(leia, "keyword.operator.leia")
    if "%=" in str(operator_pattern.get("match", "")):
        fail("Leia TextMate grammar still exposes unsupported %= compound assignment")
    assert_match(leia, "entity.name.function.leia", source)

    assert_match(leia_mod, "keyword.control.leia-mod", manifest)
    assert_match(leia_mod, "keyword.operator.arrow.leia-mod", manifest)
    assert_match(leia_mod, "constant.other.version.leia-mod", manifest)
    assert_match(leia_mod, "string.unquoted.path.leia-mod", manifest)


def check_vscode() -> None:
    package = load_json(ROOT / "editors/vscode/package.json")
    languages = {item["id"]: item for item in package["contributes"]["languages"]}
    grammars = {item["language"]: item for item in package["contributes"]["grammars"]}
    commands = {item["command"] for item in package["contributes"]["commands"]}
    config = package["contributes"].get("configuration", {}).get("properties", {})

    for language in ("leia", "leia-mod"):
        if language not in languages:
            fail(f"VS Code package does not contribute {language}")
        if language not in grammars:
            fail(f"VS Code package does not wire grammar for {language}")
        assert_path("editors/vscode/" + grammars[language]["path"].removeprefix("./"))

    for command in (
        "leia.runFile",
        "leia.testWorkspace",
        "leia.formatFile",
        "leia.lintWorkspace",
        "leia.checkWorkspace",
        "leia.previewSpec",
        "leia.restartLanguageServer",
        "leia.evaluate.case",
    ):
        if command not in commands:
            fail(f"VS Code package is missing command {command}")
    if "leia.agent.run" in commands:
        fail("VS Code package still exposes old agent run command")

    for setting in ("leia.languageServer.enabled", "leia.languageServer.executable"):
        if setting not in config:
            fail(f"VS Code package is missing setting {setting}")

    extension = (ROOT / "editors/vscode/extension.js").read_text(encoding="utf-8")
    assert_js_array_literal(
        extension,
        "semanticTokenTypes",
        ["keyword", "variable", "function", "method", "string", "number", "operator", "type", "parameter", "property", "namespace"],
    )
    assert_js_array_literal(
        extension,
        "semanticTokenModifiers",
        ["declaration", "readonly", "defaultLibrary", "import", "dialect"],
    )
    for marker in (
        "startLanguageServer(context)",
        "textDocument/definition",
        "textDocument/references",
        "textDocument/prepareRename",
        "textDocument/rename",
        "workspace/symbol",
        "textDocument/codeLens",
        "textDocument/inlayHint",
        "textDocument/documentLink",
        "textDocument/semanticTokens/full",
        "textDocument/publishDiagnostics",
        "restartLanguageServer",
        "failPending",
    ):
        if marker not in extension:
            fail(f"VS Code extension missing LSP marker {marker}")

    snippets = load_json(ROOT / "editors/vscode/snippets/leia.json")
    for key in ("function", "llm agent", "llm tool", "llm turn", "test", "go routine"):
        if key not in snippets:
            fail(f"VS Code snippets missing {key}")
    for key in ("agent", "tool", "turn"):
        if key in snippets:
            fail(f"VS Code snippets still expose old {key} block snippet")


def check_tree_sitter_assets() -> None:
    package = load_json(ROOT / "tools/tree-sitter-leia/package.json")
    config = load_json(ROOT / "tools/tree-sitter-leia/tree-sitter.json")
    load_json(ROOT / "tools/tree-sitter-leia/src/grammar.json")
    node_types = load_json(ROOT / "tools/tree-sitter-leia/src/node-types.json")
    grammar_js = (ROOT / "tools/tree-sitter-leia/grammar.js").read_text(encoding="utf-8")

    package_entry = package.get("tree-sitter", [{}])[0]
    if package_entry.get("scope") != "source.leia":
        fail("tree-sitter package metadata has the wrong scope")
    if package_entry.get("file-types") != ["leia"]:
        fail("tree-sitter package metadata has the wrong file types")

    grammar_entry = config.get("grammars", [{}])[0]
    if grammar_entry.get("name") != "leia":
        fail("tree-sitter.json has the wrong grammar name")
    if grammar_entry.get("scope") != "source.leia":
        fail("tree-sitter.json has the wrong scope")
    if grammar_entry.get("file-types") != ["leia"]:
        fail("tree-sitter.json has the wrong file types")

    named_types = {item["type"] for item in node_types if item.get("named")}
    for node_type in ("import_declaration", "dense_literal", "tagged_string_expression", "tagged_block_expression"):
        if node_type not in named_types:
            fail(f"tree-sitter node-types missing {node_type}")
    for old_node_type in (
        "agent_declaration",
        "agent_defaults_declaration",
        "agent_literal",
        "budget_statement",
        "evaluate_block",
        "message_field",
        "messages_expression",
        "models_declaration",
        "tool_declaration",
        "turn_expression",
    ):
        if old_node_type in named_types:
            fail(f"tree-sitter node-types still expose old AI-native node {old_node_type}")

    query = (ROOT / "tools/tree-sitter-leia/queries/highlights.scm").read_text(encoding="utf-8")
    for editor_query in (
        "editors/neovim/queries/leia/highlights.scm",
        "editors/helix/queries/leia/highlights.scm",
        "editors/zed/languages/leia/highlights.scm",
    ):
        if (ROOT / editor_query).read_text(encoding="utf-8") != query:
            fail(f"{editor_query} drifted from tools/tree-sitter-leia/queries/highlights.scm")
    for marker in (
        "@keyword.control",
        "@function.call",
        "@variable.parameter",
        "@tag.dialect",
        "@tag.shell",
        "@operator.raw.dialect",
        "@string.special.dialect",
        "@string.special.shell",
        "@keyword.control.import",
        "@keyword.control.import.as",
        "@string.special.import",
        "@namespace.import",
    ):
        if marker not in query:
            fail(f"tree-sitter highlight query missing {marker}")
    for stale_marker in ("@tag)", "@namespace)", '"import"\n] @keyword.function', '"as"\n] @keyword'):
        if stale_marker in query:
            fail(f"tree-sitter highlight query still uses stale generic marker {stale_marker}")
    for old_marker in ("(evaluate_block", "(agent_declaration", "(tool_declaration", "(message_field"):
        if old_marker in query:
            fail(f"tree-sitter highlight query still references old AI-native marker {old_marker}")

    for unsupported in ('"%="', '"i8"', '"i16"', '"u8"', '"u16"', '"u32"', '"u64"'):
        if unsupported in grammar_js or unsupported in query:
            fail(f"tree-sitter editor assets still expose unsupported token {unsupported}")

    corpus = "\n".join(
        path.read_text(encoding="utf-8")
        for path in sorted((ROOT / "tools/tree-sitter-leia/corpus").glob("*.txt"))
    )
    for marker in (
        "import \"go:net/http\" as http",
        "import \"json\"",
        "import p \"path\"",
        "fs \"fs\"",
        "csv!`a,b\\n1,2\\n`",
        "$!`printf ok`",
        "prompt! { role:",
        "(tagged_string_expression",
        "(tagged_block_expression",
        "(shell_tag)",
        "(dialect_bang)",
    ):
        if marker not in corpus:
            fail(f"tree-sitter corpus missing import/dialect smoke marker {marker}")


def check_packaged_editor_integrations() -> None:
    for path in (
        "editors/neovim/README.md",
        "editors/neovim/queries/leia/highlights.scm",
        "editors/helix/languages.toml",
        "editors/helix/queries/leia/highlights.scm",
        "editors/zed/extension.toml",
        "editors/zed/languages/leia/config.toml",
        "editors/zed/languages/leia/highlights.scm",
    ):
        assert_path(path)

    neovim_readme = (ROOT / "editors/neovim/README.md").read_text(encoding="utf-8")
    for marker in ("parser_config.leia", "tools/tree-sitter-leia", "queries/leia"):
        if marker not in neovim_readme:
            fail(f"Neovim README missing {marker}")

    helix_config = (ROOT / "editors/helix/languages.toml").read_text(encoding="utf-8")
    for marker in ('name = "leia"', 'language-servers = ["leia-lsp"]', 'source = { path = "../../tools/tree-sitter-leia" }'):
        if marker not in helix_config:
            fail(f"Helix config missing {marker}")

    zed_manifest = (ROOT / "editors/zed/extension.toml").read_text(encoding="utf-8")
    for marker in ('id = "leia"', "[grammars.leia]", "tools/tree-sitter-leia"):
        if marker not in zed_manifest:
            fail(f"Zed extension manifest missing {marker}")


def check_downstream_docs() -> None:
    tree_sitter_readme = (ROOT / "tools/tree-sitter-leia/README.md").read_text(encoding="utf-8")
    emacs_readme = (ROOT / "editors/emacs/README.md").read_text(encoding="utf-8")

    for marker in (
        "## Downstream Editor Integration",
        "grammar name: `leia`",
        "scope: `source.leia`",
        "### Neovim",
        "parser_config.leia",
        "### Helix",
        "[[grammar]]",
        "### Zed",
        "[grammars.leia]",
        "### Emacs `treesit`",
        "treesit-language-source-alist",
    ):
        if marker not in tree_sitter_readme:
            fail(f"tree-sitter README missing downstream marker {marker}")

    for marker in (
        "## Tree-sitter",
        "tools/tree-sitter-leia",
        "treesit-language-source-alist",
        "source.leia",
    ):
        if marker not in emacs_readme:
            fail(f"Emacs README missing tree-sitter marker {marker}")


def main() -> int:
    check_textmate()
    check_vscode()
    check_tree_sitter_assets()
    check_packaged_editor_integrations()
    check_downstream_docs()
    print("editor_smoke.py: ok")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
