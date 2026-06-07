import unittest
import json
from pathlib import Path
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parent))
import manifest


class ManifestTest(unittest.TestCase):
    def required_core_q_features(self, test_path, example_path, benchmark_path):
        return [
            {
                "id": feature_id,
                "status": "supported",
                "coverage": {
                    "tests": [test_path],
                    "examples": [example_path],
                    "benchmarks": [benchmark_path],
                },
            }
            for feature_id in sorted(manifest.Q_CORE_REQUIRED_FEATURES)
        ]

    def test_tests_manifest_covers_all_leia_cases(self):
        self.assertEqual(manifest.validate_manifest("tests"), [])

    def test_benchmarks_manifest_covers_all_leia_cases(self):
        self.assertEqual(manifest.validate_manifest("benchmarks"), [])

    def test_benchmark_case_helpers_generate_expected_metadata(self):
        root = manifest.ROOT
        path = root / "benchmarks" / "numeric" / "matmul.leia"

        self.assertEqual(manifest.case_id("benchmarks", path), "numeric/matmul")
        self.assertEqual(manifest.domain_for("benchmarks", path), "numeric")
        self.assertEqual(manifest.tags_for("benchmarks", "numeric", "benchmarks/lua_ref/numeric/matmul.lua"), ["numeric", "benchmark"])
        self.assertEqual(manifest.status_for("benchmarks"), "active")

    def test_iter_leia_cases_includes_only_benchmark_domains(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            for rel in (
                "benchmarks/numeric/keep.leia",
                "benchmarks/lua_ref/numeric/skip.leia",
                "benchmarks/not_a_domain/skip.leia",
                "benchmarks/__pycache__/skip.leia",
            ):
                path = root / rel
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("-- test\n")

            manifest.ROOT = root
            try:
                self.assertEqual(
                    [path.relative_to(root).as_posix() for path in manifest.iter_leia_cases("benchmarks")],
                    ["benchmarks/numeric/keep.leia"],
                )
            finally:
                manifest.ROOT = original_root

    def test_tests_manifest_discovery_includes_restructured_test_dirs(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            for rel in (
                "tests/llm/agent_case.leia",
                "tests/integration/llm/provider_case.leia",
                "tests/sdk/api_case.leia",
                "tests/__pycache__/skip.leia",
            ):
                path = root / rel
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("-- test\n")

            manifest.ROOT = root
            try:
                self.assertEqual(
                    [path.relative_to(root).as_posix() for path in manifest.iter_leia_cases("tests")],
                    [
                        "tests/integration/llm/provider_case.leia",
                        "tests/llm/agent_case.leia",
                        "tests/sdk/api_case.leia",
                    ],
                )
                self.assertEqual(
                    [(case["path"], case["domain"]) for case in manifest.discover_cases("tests")],
                    [
                        ("tests/integration/llm/provider_case.leia", "integration"),
                        ("tests/llm/agent_case.leia", "llm"),
                        ("tests/sdk/api_case.leia", "sdk"),
                    ],
                )
                discovered = manifest.discover_cases("tests")[0]
                self.assertIn("reference", discovered)
                self.assertNotIn("lua_ref", discovered)
            finally:
                manifest.ROOT = original_root

    def test_lua_ref_for_uses_peer_refs_for_tests_and_lua_ref_tree_for_benchmarks(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            for rel in (
                "tests/language/case.leia",
                "tests/language/case.lua",
                "benchmarks/string/case.leia",
                "benchmarks/lua_ref/string/case.lua",
            ):
                path = root / rel
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("-- test\n")

            manifest.ROOT = root
            try:
                self.assertEqual(
                    manifest.lua_ref_for("tests", root / "tests" / "language" / "case.leia"),
                    "tests/language/case.lua",
                )
                self.assertEqual(
                    manifest.lua_ref_for("benchmarks", root / "benchmarks" / "string" / "case.leia"),
                    "benchmarks/lua_ref/string/case.lua",
                )
            finally:
                manifest.ROOT = original_root

    def test_generated_benchmark_manifest_uses_domain_workloads_without_old_schema_entries(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            bench = root / "benchmarks" / "numeric" / "case.leia"
            bench.parent.mkdir(parents=True)
            bench.write_text("-- test\n")
            current_manifest = {
                "workloads": [
                    {
                        "id": "numeric/case",
                        "domain": "numeric",
                        "name": "case",
                        "script": "benchmarks/numeric/case.leia",
                        "comparison_reference": {"kind": "lua", "path": "benchmarks/lua_ref/numeric/case.lua"},
                        "params": {},
                        "recommended_scale": {"hot": {}},
                        "time_source_hint": "script_time_line",
                        "tags": ["numeric"],
                    }
                ],
                "time_source_hints": {"numeric/case": "script_time_line"},
            }
            (root / "benchmarks" / "manifest.json").write_text(json.dumps(current_manifest))

            manifest.ROOT = root
            try:
                generated = manifest.generated_manifest("benchmarks")
                self.assertEqual(generated["schema_version"], manifest.BENCHMARK_SCHEMA_VERSION)
                self.assertEqual(generated["domains"], list(manifest.BENCHMARK_DOMAINS))
                self.assertNotIn("groups", generated)
                self.assertNotIn("benchmarks", generated)
                self.assertNotIn("compatibility", generated)
                self.assertEqual(generated["workloads"][0]["domain"], "numeric")
                self.assertEqual(
                    generated["workloads"][0]["comparison_reference"],
                    {"kind": "lua", "path": "benchmarks/lua_ref/numeric/case.lua"},
                )
            finally:
                manifest.ROOT = original_root

    def test_generated_benchmark_manifest_does_not_convert_old_benchmarks_array(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            bench = root / "benchmarks" / "numeric" / "case.leia"
            bench.parent.mkdir(parents=True)
            bench.write_text("-- test\n")
            old_schema_manifest = {
                "benchmarks": [
                    {
                        "id": "numeric/case",
                        "group": "numeric",
                        "name": "case",
                        "leia_path": "benchmarks/numeric/case.leia",
                    }
                ]
            }
            (root / "benchmarks" / "manifest.json").write_text(json.dumps(old_schema_manifest))

            manifest.ROOT = root
            try:
                generated = manifest.generated_manifest("benchmarks")
                self.assertEqual(len(generated["workloads"]), 1)
                self.assertEqual(generated["workloads"][0]["id"], "numeric/case")
                self.assertEqual(generated["workloads"][0]["script"], "benchmarks/numeric/case.leia")
                self.assertNotIn("benchmarks", generated)
            finally:
                manifest.ROOT = original_root

    def test_benchmark_manifest_rejects_legacy_top_level_aliases(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            bench = root / "benchmarks" / "numeric" / "case.leia"
            bench.parent.mkdir(parents=True)
            bench.write_text("-- test\n")
            (root / "benchmarks" / "manifest.json").write_text(
                json.dumps(
                    {
                        "schema_version": manifest.BENCHMARK_SCHEMA_VERSION,
                        "case_count": 1,
                        "domains": ["numeric"],
                        "cases": [
                            {
                                "id": "numeric/case",
                                "path": "benchmarks/numeric/case.leia",
                                "domain": "numeric",
                                "kind": "benchmark",
                                "reference": None,
                                "status": "active",
                                "tags": ["numeric", "benchmark"],
                            }
                        ],
                        "workloads": [
                            {
                                "id": "numeric/case",
                                "domain": "numeric",
                                "name": "case",
                                "script": "benchmarks/numeric/case.leia",
                            }
                        ],
                        "benchmarks": [],
                        "groups": [],
                        "compatibility": {},
                    }
                )
            )

            manifest.ROOT = root
            try:
                errors = manifest.validate_manifest("benchmarks")
                self.assertTrue(any("legacy top-level key is not allowed: benchmarks" in error for error in errors))
                self.assertTrue(any("legacy top-level key is not allowed: groups" in error for error in errors))
                self.assertTrue(any("legacy top-level key is not allowed: compatibility" in error for error in errors))
            finally:
                manifest.ROOT = original_root

    def test_q_sidecar_validation_requires_complete_examples(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            example = root / "examples" / "data" / "q_new_project" / "main.leia"
            example.parent.mkdir(parents=True)
            example.write_text("-- test\n")
            sidecar = root / "examples" / "data" / "q-cases.json"
            sidecar.write_text(json.dumps({"schema_version": 1, "cases": []}))

            manifest.ROOT = root
            try:
                self.assertEqual(
                    [path.relative_to(root).as_posix() for path in manifest.discover_q_examples()],
                    ["examples/data/q_new_project/main.leia"],
                )
                errors = manifest.validate_q_sidecar(sidecar, manifest.discover_q_examples())
                self.assertIn("examples/data/q_new_project/main.leia", errors[0])
            finally:
                manifest.ROOT = original_root

    def test_q_discovery_includes_qsql_tests_and_frame_qsql_benchmarks(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            for rel in (
                "tests/language/q_case.leia",
                "tests/language/qsql_query_tdd.leia",
                "tests/language/not_q.leia",
                "benchmarks/data/q_case.leia",
                "benchmarks/data/qsql_join_variants.leia",
                "benchmarks/data/frame_qsql_rollup.leia",
                "benchmarks/data/not_q.leia",
            ):
                path = root / rel
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("-- test\n")

            manifest.ROOT = root
            try:
                self.assertEqual(
                    [path.relative_to(root).as_posix() for path in manifest.discover_q_tests()],
                    ["tests/language/q_case.leia", "tests/language/qsql_query_tdd.leia"],
                )
                self.assertEqual(
                    [path.relative_to(root).as_posix() for path in manifest.discover_q_benchmarks()],
                    [
                        "benchmarks/data/frame_qsql_rollup.leia",
                        "benchmarks/data/q_case.leia",
                        "benchmarks/data/qsql_join_variants.leia",
                    ],
                )
                self.assertEqual(
                    manifest.q_paths("benchmarks"),
                    [
                        "benchmarks/data/frame_qsql_rollup.leia",
                        "benchmarks/data/q_case.leia",
                        "benchmarks/data/qsql_join_variants.leia",
                    ],
                )
            finally:
                manifest.ROOT = original_root

    def test_q_paths_scope_separates_core_from_extended_surface_cases(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            for rel in (
                "tests/language/q_core_case.leia",
                "tests/language/q_ipc_codec_boundaries_project.leia",
                "tests/language/q_session_workspace_project.leia",
                "tests/language/q_columnar_partitioned_store_project.leia",
                "examples/data/q_core_case.leia",
                "examples/data/q_ipc_codec_boundaries_project/main.leia",
                "benchmarks/data/q_core_case.leia",
            ):
                path = root / rel
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("-- test\n")
            (root / "examples" / "data" / "q-cases.json").write_text(
                json.dumps(
                    {
                        "schema_version": 1,
                        "cases": [
                            {
                                "id": "q_core_case",
                                "path": "examples/data/q_core_case.leia",
                                "types": ["smoke"],
                                "features": ["qsql"],
                            },
                            {
                                "id": "q_ipc_codec_boundaries_project",
                                "path": "examples/data/q_ipc_codec_boundaries_project/main.leia",
                                "gate_scope": "extended",
                                "types": ["smoke", "conformance"],
                                "features": ["ipc"],
                            },
                        ],
                    }
                )
            )
            (root / "benchmarks" / "data" / "q-cases.json").write_text(
                json.dumps(
                    {
                        "schema_version": 1,
                        "cases": [
                            {
                                "id": "data/q_core_case",
                                "path": "benchmarks/data/q_core_case.leia",
                                "types": ["perf"],
                                "features": ["qsql"],
                            }
                        ],
                    }
                )
            )

            manifest.ROOT = root
            try:
                self.assertEqual(manifest.q_paths("tests", "core"), ["tests/language/q_core_case.leia"])
                self.assertEqual(
                    manifest.q_paths("tests", "extended"),
                    [
                        "tests/language/q_columnar_partitioned_store_project.leia",
                        "tests/language/q_ipc_codec_boundaries_project.leia",
                        "tests/language/q_session_workspace_project.leia",
                    ],
                )
                self.assertEqual(
                    manifest.q_paths("examples", "core"),
                    ["examples/data/q_core_case.leia"],
                )
                self.assertEqual(
                    manifest.q_paths("examples", "extended"),
                    ["examples/data/q_ipc_codec_boundaries_project/main.leia"],
                )
                self.assertEqual(
                    manifest.q_paths("examples", "all"),
                    [
                        "examples/data/q_core_case.leia",
                        "examples/data/q_ipc_codec_boundaries_project/main.leia",
                    ],
                )
                self.assertEqual(manifest.q_paths("benchmarks", "core"), ["benchmarks/data/q_core_case.leia"])
            finally:
                manifest.ROOT = original_root

    def test_q_coverage_requires_benchmark_workloads_and_sidecars(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            q_test = root / "tests" / "language" / "q_case.leia"
            q_test.parent.mkdir(parents=True)
            q_test.write_text("-- test\n")
            q_bench = root / "benchmarks" / "data" / "q_case.leia"
            q_bench.parent.mkdir(parents=True)
            q_bench.write_text("-- test\n")
            (root / "benchmarks" / "data" / "q-cases.json").write_text(
                json.dumps(
                    {
                        "schema_version": 1,
                        "cases": [
                            {
                                "id": "data/q_case",
                                "path": "benchmarks/data/q_case.leia",
                                "types": ["perf"],
                                "features": ["qsql"],
                            }
                        ],
                    }
                )
            )
            q_example = root / "examples" / "data" / "q_case.leia"
            q_example.parent.mkdir(parents=True)
            q_example.write_text("-- test\n")
            (root / "examples" / "data" / "q-cases.json").write_text(
                json.dumps(
                    {
                        "schema_version": 1,
                        "feature_matrix": [
                            {
                                "id": "dictionary",
                                "status": "supported",
                                "coverage": {
                                    "tests": ["tests/language/q_case.leia"],
                                    "examples": ["examples/data/q_case.leia"],
                                    "benchmarks": [],
                                },
                            }
                        ]
                        + self.required_core_q_features(
                            "tests/language/q_case.leia",
                            "examples/data/q_case.leia",
                            "benchmarks/data/q_case.leia",
                        ),
                        "cases": [
                            {
                                "id": "q_case",
                                "path": "examples/data/q_case.leia",
                                "types": ["smoke"],
                                "features": ["dictionary"],
                            }
                        ],
                    }
                )
            )
            (root / "tests" / "manifest.json").write_text(
                json.dumps(
                    {
                        "cases": [
                            {
                                "id": "language/q_case",
                                "path": "tests/language/q_case.leia",
                                "domain": "language",
                                "kind": "test",
                                "reference": None,
                                "status": "passing",
                                "tags": ["language", "conformance"],
                            }
                        ]
                    }
                )
            )
            (root / "benchmarks" / "manifest.json").write_text(
                json.dumps(
                    {
                        "cases": [
                            {
                                "id": "data/q_case",
                                "path": "benchmarks/data/q_case.leia",
                                "domain": "data",
                                "kind": "benchmark",
                                "reference": None,
                                "status": "active",
                                "tags": ["data", "benchmark"],
                            }
                        ],
                        "workloads": [],
                    }
                )
            )

            manifest.ROOT = root
            try:
                self.assertEqual(manifest.discover_q_benchmarks(), [q_bench])
                self.assertEqual(
                    manifest.validate_q_coverage(),
                    ["benchmarks/manifest.json: missing q benchmark workload benchmarks/data/q_case.leia"],
                )
            finally:
                manifest.ROOT = original_root

    def test_q_coverage_requires_qsql_language_cases_in_manifest(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            q_test = root / "tests" / "language" / "qsql_query_tdd.leia"
            q_test.parent.mkdir(parents=True)
            q_test.write_text("-- test\n")
            (root / "tests" / "manifest.json").write_text(json.dumps({"cases": []}))
            (root / "benchmarks" / "manifest.json").parent.mkdir(parents=True)
            (root / "benchmarks" / "manifest.json").write_text(json.dumps({"cases": [], "workloads": []}))
            (root / "benchmarks" / "data").mkdir(parents=True)
            (root / "benchmarks" / "data" / "q-cases.json").write_text(json.dumps({"schema_version": 1, "cases": []}))

            manifest.ROOT = root
            try:
                self.assertEqual(
                    manifest.validate_q_coverage({"tests"}),
                    ["tests/manifest.json: missing q language case tests/language/qsql_query_tdd.leia"],
                )
            finally:
                manifest.ROOT = original_root

    def test_q_feature_matrix_validates_status_paths_and_case_links(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            q_test = root / "tests" / "language" / "q_case.leia"
            q_test.parent.mkdir(parents=True)
            q_test.write_text("-- test\n")
            q_example = root / "examples" / "data" / "q_case.leia"
            q_example.parent.mkdir(parents=True)
            q_example.write_text("-- test\n")
            q_bench = root / "benchmarks" / "data" / "q_case.leia"
            q_bench.parent.mkdir(parents=True)
            q_bench.write_text("-- test\n")
            (root / "benchmarks" / "data" / "q-cases.json").write_text(
                json.dumps(
                    {
                        "schema_version": 1,
                        "cases": [
                            {
                                "id": "data/q_case",
                                "path": "benchmarks/data/q_case.leia",
                                "types": ["perf"],
                                "features": ["qsql"],
                            }
                        ],
                    }
                )
            )
            sidecar = root / "examples" / "data" / "q-cases.json"
            sidecar.write_text(
                json.dumps(
                    {
                        "schema_version": 1,
                        "feature_matrix": [
                            {
                                "id": "future-feature",
                                "status": "planned",
                                "coverage": {"tests": [], "examples": [], "benchmarks": []},
                            },
                        ]
                        + self.required_core_q_features(
                            "tests/language/q_case.leia",
                            "examples/data/q_case.leia",
                            "benchmarks/data/q_case.leia",
                        ),
                        "cases": [
                            {
                                "id": "q_case",
                                "path": "examples/data/q_case.leia",
                                "types": ["smoke"],
                                "features": ["qsql"],
                            }
                        ],
                    }
                )
            )

            manifest.ROOT = root
            try:
                self.assertEqual(manifest.validate_q_feature_matrix(sidecar), [])
            finally:
                manifest.ROOT = original_root

    def test_q_feature_matrix_rejects_unknown_feature_and_missing_tests(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            q_example = root / "examples" / "data" / "q_case.leia"
            q_example.parent.mkdir(parents=True)
            q_example.write_text("-- test\n")
            (root / "benchmarks" / "data").mkdir(parents=True)
            (root / "benchmarks" / "data" / "q-cases.json").write_text(json.dumps({"schema_version": 1, "cases": []}))
            sidecar = root / "examples" / "data" / "q-cases.json"
            sidecar.write_text(
                json.dumps(
                    {
                        "schema_version": 1,
                        "feature_matrix": [
                            {
                                "id": "qsql",
                                "status": "supported",
                                "gate_scope": "invalid",
                                "coverage": {"tests": [], "examples": [], "benchmarks": []},
                            }
                        ],
                        "cases": [
                            {
                                "id": "q_case",
                                "path": "examples/data/q_case.leia",
                                "types": ["smoke"],
                                "features": ["unknown-feature"],
                            }
                        ],
                    }
                )
            )

            manifest.ROOT = root
            try:
                errors = manifest.validate_q_feature_matrix(sidecar)
                self.assertTrue(any("qsql status supported requires at least one q language test" in error for error in errors))
                self.assertTrue(any("qsql has invalid gate_scope 'invalid'" in error for error in errors))
                self.assertTrue(any("unknown-feature" in error for error in errors))
            finally:
                manifest.ROOT = original_root

    def test_q_feature_matrix_enforces_core_denominator_and_classification(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            q_test = root / "tests" / "language" / "q_case.leia"
            q_test.parent.mkdir(parents=True)
            q_test.write_text("-- test\n")
            q_example = root / "examples" / "data" / "q_case.leia"
            q_example.parent.mkdir(parents=True)
            q_example.write_text("-- test\n")
            q_unclassified = root / "examples" / "data" / "q_unclassified.leia"
            q_unclassified.write_text("-- test\n")
            q_bench = root / "benchmarks" / "data" / "q_case.leia"
            q_bench.parent.mkdir(parents=True)
            q_bench.write_text("-- test\n")
            (root / "benchmarks" / "data" / "q-cases.json").write_text(
                json.dumps(
                    {
                        "schema_version": 1,
                        "cases": [
                            {
                                "id": "data/q_case",
                                "path": "benchmarks/data/q_case.leia",
                                "types": ["perf"],
                                "features": ["qsql", "ipc"],
                            }
                        ],
                    }
                )
            )
            sidecar = root / "examples" / "data" / "q-cases.json"
            sidecar.write_text(
                json.dumps(
                    {
                        "schema_version": 1,
                        "feature_matrix": [
                            {
                                "id": "qsql",
                                "status": "supported",
                                "coverage": {
                                    "tests": ["tests/language/q_case.leia"],
                                    "examples": [],
                                    "benchmarks": ["benchmarks/data/q_case.leia"],
                                },
                            },
                            {
                                "id": "ipc",
                                "status": "supported",
                                "coverage": {
                                    "tests": ["tests/language/q_case.leia"],
                                    "examples": ["examples/data/q_case.leia"],
                                    "benchmarks": [],
                                },
                            },
                        ],
                        "cases": [
                            {
                                "id": "q_case",
                                "path": "examples/data/q_case.leia",
                                "types": ["smoke"],
                                "features": ["qsql", "ipc"],
                            }
                        ],
                    }
                )
            )

            manifest.ROOT = root
            try:
                errors = manifest.validate_q_feature_matrix(sidecar)
                self.assertTrue(any("missing required core q feature runtime-kernel" in error for error in errors))
                self.assertTrue(any("ipc must be extended, not core" in error for error in errors))
                self.assertTrue(any("core q case examples/data/q_case.leia must not include extended feature ipc" in error for error in errors))
                self.assertTrue(any("core q case benchmarks/data/q_case.leia must not include extended feature ipc" in error for error in errors))
                self.assertTrue(any("q examples case is not classified in feature_matrix coverage" in error for error in errors))
            finally:
                manifest.ROOT = original_root


if __name__ == "__main__":
    unittest.main()
