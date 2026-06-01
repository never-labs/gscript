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


def assert_path(path: str) -> None:
    if not (ROOT / path).is_file():
        fail(f"missing path {path}")


def check_textmate() -> None:
    leia = load_json(ROOT / "tools/syntax/textmate/leia.tmLanguage.json")
    leia_mod = load_json(ROOT / "tools/syntax/textmate/leia-mod.tmLanguage.json")
    source = (ROOT / "tools/editor/smoke/fixtures/ai_native.leia").read_text(encoding="utf-8")
    manifest = (ROOT / "tools/editor/smoke/fixtures/leia.mod").read_text(encoding="utf-8")

    if leia.get("scopeName") != "source.leia":
        fail("Leia TextMate grammar has the wrong scope")
    if leia_mod.get("scopeName") != "source.leia.mod":
        fail("leia.mod TextMate grammar has the wrong scope")

    assert_match(leia, "keyword.control.directive.leia", source)
    assert_match(leia, "storage.type.function.leia", source)
    assert_match(leia, "keyword.control.ai.leia", source)
    assert_match(leia, "support.type.primitive.leia", source)
    assert_match(leia, "constant.numeric.duration.leia", source)
    assert_match(leia, "keyword.operator.leia", source)
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
    ):
        if command not in commands:
            fail(f"VS Code package is missing command {command}")

    snippets = load_json(ROOT / "editors/vscode/snippets/leia.json")
    for key in ("function", "agent", "tool", "turn", "test", "go routine"):
        if key not in snippets:
            fail(f"VS Code snippets missing {key}")


def check_tree_sitter_assets() -> None:
    load_json(ROOT / "tools/tree-sitter-leia/src/grammar.json")
    node_types = load_json(ROOT / "tools/tree-sitter-leia/src/node-types.json")
    named_types = {item["type"] for item in node_types if item.get("named")}
    for node_type in (
        "agent_declaration",
        "models_declaration",
        "tool_declaration",
        "turn_expression",
        "messages_expression",
        "dense_literal",
    ):
        if node_type not in named_types:
            fail(f"tree-sitter node-types missing {node_type}")


def main() -> int:
    check_textmate()
    check_vscode()
    check_tree_sitter_assets()
    print("editor_smoke.py: ok")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
