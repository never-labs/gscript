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
Q_CASE_TYPES = {
    "conformance",
    "perf",
    "smoke",
}
Q_FEATURE_STATUSES = {
    "supported",
    "rejected",
    "planned",
}
Q_COVERAGE_KINDS = (
    "tests",
    "examples",
    "benchmarks",
)
Q_LIST_KINDS = Q_COVERAGE_KINDS
Q_GATE_SCOPES = {
    "all",
    "core",
    "extended",
}
Q_CORE_REQUIRED_FEATURES = {
    "adverbs",
    "asof",
    "join",
    "keyed",
    "mutation",
    "qsql",
    "runtime-kernel",
    "temporal",
    "typed-null",
    "window",
}
Q_CORE_EXCLUDED_FEATURES = {
    "ipc",
    "persistence",
    "serialization",
    "session",
    "session-workspace",
    "storage-path",
    "workspace",
}
Q_EXTENDED_TEST_MARKERS = (
    "q_boundaries_and_gap_closures_project",
    "q_columnar_partitioned_store_project",
    "q_ipc_codec_boundaries_project",
    "q_session_workspace_project",
    "q_workspace_session_project_example",
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


def is_q_benchmark_path(path: Path) -> bool:
    rel = path.relative_to(ROOT).as_posix()
    name = path.name
    return rel.startswith("benchmarks/data/") and name.startswith(("q_", "qsql_", "frame_qsql_"))


def is_q_example_path(path: Path) -> bool:
    rel = path.relative_to(ROOT).parts
    if len(rel) < 3 or rel[0] != "examples" or rel[1] != "data":
        return False
    if path.name.startswith(("q_", "qsql_")):
        return True
    return any(part.startswith(("q_", "qsql_", "db_q_")) for part in rel[2:-1])


def discover_q_benchmarks() -> list[Path]:
    base = ROOT / "benchmarks" / "data"
    if not base.exists():
        return []
    return sorted(path for path in base.glob("*.leia") if is_q_benchmark_path(path))


def discover_q_examples() -> list[Path]:
    base = ROOT / "examples" / "data"
    if not base.exists():
        return []
    return sorted(path for path in base.rglob("*.leia") if is_q_example_path(path))


def discover_q_tests() -> list[Path]:
    base = ROOT / "tests" / "language"
    if not base.exists():
        return []
    paths = [*base.glob("q_*.leia"), *base.glob("qsql_*.leia")]
    return sorted(set(paths))


def load_q_sidecar(path: Path) -> tuple[list[dict[str, Any]], list[str]]:
    errors: list[str] = []
    if not path.exists():
        return [], [f"{repo_rel(path)} is missing"]
    try:
        data = json.loads(path.read_text())
    except json.JSONDecodeError as exc:
        return [], [f"{repo_rel(path)}: invalid JSON: {exc}"]
    cases = data.get("cases")
    if not isinstance(cases, list):
        return [], [f"{repo_rel(path)}: missing cases array"]
    out: list[dict[str, Any]] = []
    for index, case in enumerate(cases):
        if not isinstance(case, dict):
            errors.append(f"{repo_rel(path)}: cases[{index}] is not an object")
            continue
        out.append(case)
    return out, errors


def q_feature_matrix(sidecar_path: Path) -> tuple[list[dict[str, Any]], list[str]]:
    errors: list[str] = []
    if not sidecar_path.exists():
        return [], [f"{repo_rel(sidecar_path)} is missing"]
    try:
        data = json.loads(sidecar_path.read_text())
    except json.JSONDecodeError as exc:
        return [], [f"{repo_rel(sidecar_path)}: invalid JSON: {exc}"]
    matrix = data.get("feature_matrix")
    if not isinstance(matrix, list):
        return [], [f"{repo_rel(sidecar_path)}: missing feature_matrix array"]
    out: list[dict[str, Any]] = []
    for index, feature in enumerate(matrix):
        if not isinstance(feature, dict):
            errors.append(f"{repo_rel(sidecar_path)}: feature_matrix[{index}] is not an object")
            continue
        out.append(feature)
    return out, errors


def valid_q_coverage_paths() -> dict[str, set[str]]:
    return {
        "tests": {repo_rel(path) for path in discover_q_tests()},
        "examples": {repo_rel(path) for path in discover_q_examples()},
        "benchmarks": {repo_rel(path) for path in discover_q_benchmarks()},
    }


def validate_q_sidecar(sidecar_path: Path, expected_paths: list[Path]) -> list[str]:
    errors: list[str] = []
    cases, load_errors = load_q_sidecar(sidecar_path)
    errors.extend(load_errors)
    expected_rel = {repo_rel(path) for path in expected_paths}
    by_path: dict[str, dict[str, Any]] = {}
    ids: set[str] = set()

    for index, case in enumerate(cases):
        case_id_value = case.get("id")
        path_value = case.get("path")
        if not isinstance(case_id_value, str) or not case_id_value:
            errors.append(f"{repo_rel(sidecar_path)}: cases[{index}] has missing or non-string id")
        elif case_id_value in ids:
            errors.append(f"{repo_rel(sidecar_path)}: duplicate case id {case_id_value}")
        else:
            ids.add(case_id_value)

        if not isinstance(path_value, str):
            errors.append(f"{repo_rel(sidecar_path)}: cases[{index}] has missing or non-string path")
            continue
        if path_value in by_path:
            errors.append(f"{repo_rel(sidecar_path)}: duplicate case path {path_value}")
        by_path[path_value] = case
        if not (ROOT / path_value).exists():
            errors.append(f"{repo_rel(sidecar_path)}: listed q case does not exist: {path_value}")

        types = case.get("types")
        if not isinstance(types, list) or not types or not all(isinstance(value, str) for value in types):
            errors.append(f"{repo_rel(sidecar_path)}: {path_value} has missing or invalid types")
        elif unknown := sorted(set(types) - Q_CASE_TYPES):
            errors.append(f"{repo_rel(sidecar_path)}: {path_value} has unknown types: {', '.join(unknown)}")

        gate_scope = case.get("gate_scope", "core")
        if gate_scope not in Q_GATE_SCOPES - {"all"}:
            errors.append(f"{repo_rel(sidecar_path)}: {path_value} has invalid gate_scope {gate_scope!r}")

        features = case.get("features")
        if not isinstance(features, list) or not features or not all(isinstance(value, str) for value in features):
            errors.append(f"{repo_rel(sidecar_path)}: {path_value} has missing or invalid features")

    actual_rel = set(by_path)
    missing_paths = sorted(expected_rel - actual_rel)
    extra_paths = sorted(actual_rel - expected_rel)
    if missing_paths:
        errors.append(f"{repo_rel(sidecar_path)}: missing q cases: {', '.join(missing_paths)}")
    if extra_paths:
        errors.append(f"{repo_rel(sidecar_path)}: extra q cases: {', '.join(extra_paths)}")
    return errors


def validate_q_feature_matrix(sidecar_path: Path) -> list[str]:
    errors: list[str] = []
    matrix, matrix_errors = q_feature_matrix(sidecar_path)
    errors.extend(matrix_errors)
    valid_paths = valid_q_coverage_paths()
    known_features: set[str] = set()
    covered_paths = {kind: set() for kind in Q_COVERAGE_KINDS}
    rows_by_feature: dict[str, dict[str, Any]] = {}

    for index, feature in enumerate(matrix):
        feature_id = feature.get("id")
        if not isinstance(feature_id, str) or not feature_id:
            errors.append(f"{repo_rel(sidecar_path)}: feature_matrix[{index}] has missing or non-string id")
            continue
        if feature_id in known_features:
            errors.append(f"{repo_rel(sidecar_path)}: duplicate q feature id {feature_id}")
        known_features.add(feature_id)
        rows_by_feature[feature_id] = feature

        status = feature.get("status")
        if status not in Q_FEATURE_STATUSES:
            errors.append(f"{repo_rel(sidecar_path)}: {feature_id} has invalid status {status!r}")

        gate_scope = feature.get("gate_scope", "core")
        if gate_scope not in Q_GATE_SCOPES - {"all"}:
            errors.append(f"{repo_rel(sidecar_path)}: {feature_id} has invalid gate_scope {gate_scope!r}")
        if gate_scope == "core" and feature_id in Q_CORE_EXCLUDED_FEATURES:
            errors.append(f"{repo_rel(sidecar_path)}: {feature_id} must be extended, not core")

        coverage = feature.get("coverage")
        if not isinstance(coverage, dict):
            errors.append(f"{repo_rel(sidecar_path)}: {feature_id} has missing coverage object")
            coverage = {}

        covered_any = False
        for kind in Q_COVERAGE_KINDS:
            paths = coverage.get(kind, [])
            if not isinstance(paths, list) or not all(isinstance(value, str) for value in paths):
                errors.append(f"{repo_rel(sidecar_path)}: {feature_id} coverage.{kind} must be a string array")
                continue
            if len(paths) != len(set(paths)):
                errors.append(f"{repo_rel(sidecar_path)}: {feature_id} coverage.{kind} has duplicate paths")
            for path in paths:
                covered_any = True
                covered_paths[kind].add(path)
                if path not in valid_paths[kind]:
                    errors.append(f"{repo_rel(sidecar_path)}: {feature_id} coverage.{kind} references unknown q case {path}")

        if status in {"supported", "rejected"}:
            tests = coverage.get("tests", [])
            if not isinstance(tests, list) or not tests:
                errors.append(f"{repo_rel(sidecar_path)}: {feature_id} status {status} requires at least one q language test")
        if status == "supported" and not covered_any:
            errors.append(f"{repo_rel(sidecar_path)}: {feature_id} status supported requires coverage")
        if status == "supported" and gate_scope == "core":
            tests = coverage.get("tests", []) if isinstance(coverage, dict) else []
            if not isinstance(tests, list) or not any(
                isinstance(path, str) and q_path_gate_scope("tests", path) == "core" for path in tests
            ):
                errors.append(f"{repo_rel(sidecar_path)}: {feature_id} core supported feature requires at least one core q language test")

    for feature_id in sorted(Q_CORE_REQUIRED_FEATURES):
        feature = rows_by_feature.get(feature_id)
        if feature is None:
            errors.append(f"{repo_rel(sidecar_path)}: missing required core q feature {feature_id}")
            continue
        if feature.get("status") != "supported":
            errors.append(f"{repo_rel(sidecar_path)}: required core q feature {feature_id} must be supported")
        if feature.get("gate_scope", "core") != "core":
            errors.append(f"{repo_rel(sidecar_path)}: required core q feature {feature_id} must have core gate_scope")

    for kind in Q_COVERAGE_KINDS:
        for path in sorted(valid_paths[kind] - covered_paths[kind]):
            errors.append(f"{repo_rel(sidecar_path)}: q {kind} case is not classified in feature_matrix coverage: {path}")

    for sidecar in (sidecar_path, ROOT / "benchmarks" / "data" / "q-cases.json"):
        cases, load_errors = load_q_sidecar(sidecar)
        errors.extend(load_errors)
        for case in cases:
            path = case.get("path", "<unknown>")
            features = case.get("features", [])
            if not isinstance(features, list):
                continue
            gate_scope = case.get("gate_scope", "core")
            if gate_scope == "core":
                for feature_id in sorted(set(features) & Q_CORE_EXCLUDED_FEATURES):
                    errors.append(f"{repo_rel(sidecar)}: core q case {path} must not include extended feature {feature_id}")
            kind = "examples" if sidecar.parts[-3:-1] == ("examples", "data") else "benchmarks"
            for feature_id in features:
                if not isinstance(feature_id, str):
                    continue
                if feature_id not in known_features:
                    errors.append(f"{repo_rel(sidecar)}: {path} references unknown q feature {feature_id}")
                    continue
                for row in matrix:
                    if row.get("id") != feature_id:
                        continue
                    coverage = row.get("coverage", {})
                    paths = coverage.get(kind, []) if isinstance(coverage, dict) else []
                    if path not in paths:
                        errors.append(f"{repo_rel(sidecar_path)}: {feature_id} coverage.{kind} is missing {path}")
                    break

    return errors


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
            if isinstance(existing_workloads, list):
                workloads = [
                    row
                    for row in existing_workloads
                    if isinstance(row, dict)
                    and isinstance(row.get("script"), str)
                    and (ROOT / row["script"]).exists()
                ]
            else:
                workloads = []
            workload_ids = {row.get("id") for row in workloads if isinstance(row.get("id"), str)}
            cases_by_script = {
                case["path"]: case
                for case in cases
                if isinstance(case, dict) and isinstance(case.get("path"), str)
            }
            for case in cases:
                case_id_value = case["id"]
                if case_id_value in workload_ids:
                    continue
                workload = {
                    "id": case_id_value,
                    "domain": case["domain"],
                    "name": Path(case["path"]).stem,
                    "script": case["path"],
                    "comparison_reference": case["reference"],
                    "params": {},
                    "recommended_scale": {"hot": {}},
                    "time_source_hint": "script_time_line",
                    "tags": [case["domain"]],
                }
                workloads.append(workload)
                workload_ids.add(case_id_value)
            for row in workloads:
                script = row.get("script")
                if not isinstance(script, str):
                    continue
                case = cases_by_script.get(script)
                if case is None:
                    continue
                if row.get("comparison_reference") is None and case.get("reference") is not None:
                    row["comparison_reference"] = case["reference"]
            manifest["workloads"] = workloads
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

    if root_name == "benchmarks":
        for legacy_key in ("benchmarks", "groups", "compatibility"):
            if legacy_key in manifest:
                errors.append(f"{repo_rel(manifest_path)}: legacy top-level key is not allowed: {legacy_key}")
        workloads = manifest.get("workloads")
        if not isinstance(workloads, list):
            errors.append(f"{repo_rel(manifest_path)}: missing workloads array")
        else:
            workload_ids: set[str] = set()
            for index, row in enumerate(workloads):
                if not isinstance(row, dict):
                    errors.append(f"{repo_rel(manifest_path)}: workloads[{index}] is not an object")
                    continue
                row_id = row.get("id")
                if not isinstance(row_id, str):
                    errors.append(f"{repo_rel(manifest_path)}: workloads[{index}] has non-string id")
                elif row_id in workload_ids:
                    errors.append(f"{repo_rel(manifest_path)}: duplicate workload id {row_id}")
                else:
                    workload_ids.add(row_id)
                script = row.get("script")
                if not isinstance(script, str):
                    errors.append(f"{repo_rel(manifest_path)}: workloads[{index}] has non-string script")
                    continue
                if not (ROOT / script).exists():
                    errors.append(f"{repo_rel(manifest_path)}: workload script does not exist: {script}")

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


def validate_q_coverage(roots: set[str] | None = None) -> list[str]:
    roots = {"tests", "benchmarks"} if roots is None else roots
    errors: list[str] = []
    if "tests" in roots:
        tests_manifest = load_manifest("tests")
        tests_by_path = {
            case.get("path"): case
            for case in tests_manifest.get("cases", [])
            if isinstance(case, dict) and isinstance(case.get("path"), str)
        }
        for path in discover_q_tests():
            rel = repo_rel(path)
            case = tests_by_path.get(rel)
            if case is None:
                errors.append(f"tests/manifest.json: missing q language case {rel}")
                continue
            if case.get("domain") != "language" or "conformance" not in case.get("tags", []):
                errors.append(f"tests/manifest.json: q language case {rel} is not tagged language conformance")

    if "benchmarks" in roots:
        benchmarks_manifest = load_manifest("benchmarks")
        benchmark_cases = {
            case.get("path"): case
            for case in benchmarks_manifest.get("cases", [])
            if isinstance(case, dict) and isinstance(case.get("path"), str)
        }
        benchmark_workloads = {
            row.get("script"): row
            for row in benchmarks_manifest.get("workloads", [])
            if isinstance(row, dict) and isinstance(row.get("script"), str)
        }
        q_benchmarks = discover_q_benchmarks()
        for path in q_benchmarks:
            rel = repo_rel(path)
            if rel not in benchmark_cases:
                errors.append(f"benchmarks/manifest.json: missing q benchmark case {rel}")
            if rel not in benchmark_workloads:
                errors.append(f"benchmarks/manifest.json: missing q benchmark workload {rel}")
        errors.extend(validate_q_sidecar(ROOT / "benchmarks" / "data" / "q-cases.json", q_benchmarks))

    if {"tests", "benchmarks"}.issubset(roots):
        errors.extend(validate_q_sidecar(ROOT / "examples" / "data" / "q-cases.json", discover_q_examples()))
        errors.extend(validate_q_feature_matrix(ROOT / "examples" / "data" / "q-cases.json"))
    return errors


def write_manifest(root_name: str) -> None:
    path = ROOT / root_name / "manifest.json"
    path.write_text(json.dumps(generated_manifest(root_name), indent=2, sort_keys=False) + "\n")


def q_sidecar_path(kind: str) -> Path | None:
    if kind == "examples":
        return ROOT / "examples" / "data" / "q-cases.json"
    if kind == "benchmarks":
        return ROOT / "benchmarks" / "data" / "q-cases.json"
    return None


def q_path_gate_scope(kind: str, path: str) -> str:
    if kind == "tests":
        name = Path(path).stem
        return "extended" if any(marker == name for marker in Q_EXTENDED_TEST_MARKERS) else "core"

    sidecar = q_sidecar_path(kind)
    if sidecar is None or not sidecar.exists():
        return "core"
    cases, _ = load_q_sidecar(sidecar)
    for case in cases:
        if case.get("path") == path:
            scope = case.get("gate_scope", "core")
            return scope if scope in {"core", "extended"} else "core"
    return "core"


def q_paths(kind: str, scope: str = "all") -> list[str]:
    if kind == "tests":
        paths = [repo_rel(path) for path in discover_q_tests()]
    elif kind == "examples":
        paths = [repo_rel(path) for path in discover_q_examples()]
    elif kind == "benchmarks":
        paths = [repo_rel(path) for path in discover_q_benchmarks()]
    else:
        raise ValueError(f"unknown q list kind: {kind}")
    if scope == "all":
        return paths
    if scope not in Q_GATE_SCOPES:
        raise ValueError(f"unknown q gate scope: {scope}")
    return [path for path in paths if q_path_gate_scope(kind, path) == scope]


def main(argv: list[str] | None = None) -> int:
    raw_args = list(sys.argv[1:] if argv is None else argv)
    scope = "all"
    cleaned_args: list[str] = []
    index = 0
    while index < len(raw_args):
        arg = raw_args[index]
        if arg == "--scope":
            index += 1
            if index >= len(raw_args):
                print("--scope requires a value", file=sys.stderr)
                return 2
            scope = raw_args[index]
        elif arg.startswith("--scope="):
            scope = arg.split("=", 1)[1]
        else:
            cleaned_args.append(arg)
        index += 1
    if scope not in Q_GATE_SCOPES:
        print(f"--scope must be one of: {', '.join(sorted(Q_GATE_SCOPES))}", file=sys.stderr)
        return 2

    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("generate", "check", "list", "list-q"))
    parser.add_argument(
        "roots",
        nargs="*",
        default=None,
        metavar="root",
    )
    args = parser.parse_args(cleaned_args)

    if args.command == "list-q":
        kinds = list(args.roots or Q_LIST_KINDS)
        for kind in kinds:
            if kind not in Q_LIST_KINDS:
                parser.error("list-q accepts only: tests, examples, benchmarks")
            for path in q_paths(kind, scope):
                print(path)
        return 0

    if args.command == "generate":
        for root_name in args.roots or ("tests", "benchmarks"):
            if root_name not in {"tests", "benchmarks"}:
                parser.error("generate accepts only: tests, benchmarks")
            write_manifest(root_name)
        return 0

    if args.command == "list":
        for root_name in args.roots or ("tests", "benchmarks"):
            if root_name not in {"tests", "benchmarks"}:
                parser.error("list accepts only: tests, benchmarks")
            for case in discover_cases(root_name):
                print(json.dumps(case, sort_keys=True))
        return 0

    errors: list[str] = []
    roots = args.roots or ("tests", "benchmarks")
    for root_name in roots:
        if root_name not in {"tests", "benchmarks"}:
            parser.error("check accepts only: tests, benchmarks")
        errors.extend(validate_manifest(root_name))
    errors.extend(validate_q_coverage(set(roots)))
    if errors:
        for error in errors:
            print(error, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
