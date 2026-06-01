#!/usr/bin/env python3
"""Generate and validate lightweight Leia case manifests."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[1]
SCHEMA_VERSION = 2
BENCHMARK_SCHEMA_VERSION = 3
EXCLUDED_DIRS = {
    "__pycache__",
    "lua_ref",
}
BENCHMARK_DOMAINS = (
    "numeric",
    "recursion",
    "table",
    "calls",
    "string",
    "concurrency",
    "data",
    "app",
    "control",
    "precision",
)


def repo_rel(path: Path) -> str:
    return path.relative_to(ROOT).as_posix()


def case_id(root_name: str, path: Path) -> str:
    rel = path.relative_to(ROOT / root_name).with_suffix("")
    return rel.as_posix()


def domain_for(root_name: str, path: Path) -> str:
    rel = path.relative_to(ROOT / root_name)
    if len(rel.parts) == 1:
        return "integration" if root_name == "tests" else "default"
    return rel.parts[0]


def tags_for(root_name: str, domain: str, lua_ref: str | None) -> list[str]:
    tags = [domain]
    if root_name == "tests":
        tags.append("conformance" if domain == "language" else "integration")
    else:
        tags.append("benchmark")
    return tags


def status_for(root_name: str) -> str:
    return "passing" if root_name == "tests" else "active"


def lua_ref_for(root_name: str, path: Path) -> str | None:
    rel = path.relative_to(ROOT / root_name).with_suffix(".lua")
    if root_name == "tests":
        lua_path = ROOT / root_name / rel
    else:
        lua_path = ROOT / root_name / "lua_ref" / rel
    return repo_rel(lua_path) if lua_path.exists() else None


def iter_leia_cases(root_name: str) -> list[Path]:
    base = ROOT / root_name
    paths: list[Path] = []
    for path in sorted(base.rglob("*.leia")):
        rel_parts = path.relative_to(base).parts
        if any(part in EXCLUDED_DIRS for part in rel_parts):
            continue
        if root_name == "benchmarks" and rel_parts[0] not in BENCHMARK_DOMAINS:
            continue
        paths.append(path)
    return paths


def discover_cases(root_name: str) -> list[dict[str, Any]]:
    cases: list[dict[str, Any]] = []
    for path in iter_leia_cases(root_name):
        domain = domain_for(root_name, path)
        lua_ref = lua_ref_for(root_name, path)
        cases.append(
            {
                "id": case_id(root_name, path),
                "path": repo_rel(path),
                "domain": domain,
                "kind": "test" if root_name == "tests" else "benchmark",
                "reference": {"kind": "lua", "path": lua_ref} if lua_ref else None,
                "status": status_for(root_name),
                "tags": tags_for(root_name, domain, lua_ref),
            }
        )
    return cases


def reference_lua_path(case: dict[str, Any]) -> str | None:
    reference = case.get("reference")
    if isinstance(reference, dict) and reference.get("kind") == "lua":
        path = reference.get("path")
        return path if isinstance(path, str) else None
    return None


def load_manifest(root_name: str) -> dict[str, Any]:
    path = ROOT / root_name / "manifest.json"
    with path.open() as f:
        return json.load(f)


def generated_manifest(root_name: str) -> dict[str, Any]:
    cases = discover_cases(root_name)
    manifest: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "case_count": len(cases),
        "cases": cases,
    }
    if root_name == "benchmarks":
        manifest["schema_version"] = BENCHMARK_SCHEMA_VERSION
        manifest["domains"] = list(BENCHMARK_DOMAINS)
        existing_path = ROOT / "benchmarks" / "manifest.json"
        if existing_path.exists():
            existing = json.loads(existing_path.read_text())
            for key in ("time_source_hints", "scale_profiles"):
                if key in existing:
                    manifest[key] = existing[key]
            existing_workloads = existing.get("workloads")
            manifest["workloads"] = existing_workloads if isinstance(existing_workloads, list) else []
        else:
            manifest["workloads"] = []
    return manifest


def validate_manifest(root_name: str) -> list[str]:
    errors: list[str] = []
    manifest_path = ROOT / root_name / "manifest.json"
    if not manifest_path.exists():
        return [f"{repo_rel(manifest_path)} is missing"]

    manifest = load_manifest(root_name)
    cases = manifest.get("cases")
    if not isinstance(cases, list):
        return [f"{repo_rel(manifest_path)}: missing cases array"]

    discovered_by_path = {case["path"]: case for case in discover_cases(root_name)}
    manifest_by_path: dict[str, dict[str, Any]] = {}
    required = {"id", "path", "domain", "kind", "reference", "status", "tags"}

    for index, case in enumerate(cases):
        if not isinstance(case, dict):
            errors.append(f"{repo_rel(manifest_path)}: cases[{index}] is not an object")
            continue
        missing = sorted(required - case.keys())
        if missing:
            errors.append(f"{repo_rel(manifest_path)}: cases[{index}] missing {', '.join(missing)}")
        path = case.get("path")
        if not isinstance(path, str):
            errors.append(f"{repo_rel(manifest_path)}: cases[{index}] has non-string path")
            continue
        if path in manifest_by_path:
            errors.append(f"{repo_rel(manifest_path)}: duplicate case path {path}")
        manifest_by_path[path] = case
        if path not in discovered_by_path:
            errors.append(f"{repo_rel(manifest_path)}: listed path does not exist or is excluded: {path}")

    missing_paths = sorted(set(discovered_by_path) - set(manifest_by_path))
    extra_paths = sorted(set(manifest_by_path) - set(discovered_by_path))
    if missing_paths:
        errors.append(f"{repo_rel(manifest_path)}: missing .leia cases: {', '.join(missing_paths)}")
    if extra_paths:
        errors.append(f"{repo_rel(manifest_path)}: extra .leia cases: {', '.join(extra_paths)}")

    if manifest.get("case_count") != len(cases):
        errors.append(f"{repo_rel(manifest_path)}: case_count does not match cases length")

    for expected in discovered_by_path.values():
        actual = manifest_by_path.get(expected["path"])
        if actual is None:
            continue
        for field in required:
            if actual.get(field) != expected.get(field):
                errors.append(
                    f"{repo_rel(manifest_path)}: {expected['path']} field {field} "
                    f"is {actual.get(field)!r}, expected {expected.get(field)!r}"
                )

    return errors


def write_manifest(root_name: str) -> None:
    path = ROOT / root_name / "manifest.json"
    path.write_text(json.dumps(generated_manifest(root_name), indent=2, sort_keys=False) + "\n")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("generate", "check", "list"))
    parser.add_argument("roots", nargs="*", choices=("tests", "benchmarks"), default=("tests", "benchmarks"))
    args = parser.parse_args(argv)

    if args.command == "generate":
        for root_name in args.roots:
            write_manifest(root_name)
        return 0

    if args.command == "list":
        for root_name in args.roots:
            for case in discover_cases(root_name):
                print(json.dumps(case, sort_keys=True))
        return 0

    errors: list[str] = []
    for root_name in args.roots:
        errors.extend(validate_manifest(root_name))
    if errors:
        for error in errors:
            print(error, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
