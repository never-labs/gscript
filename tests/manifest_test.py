import unittest
import json
from pathlib import Path
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parent))
import manifest


class ManifestTest(unittest.TestCase):
    def test_tests_manifest_covers_all_gscript_cases(self):
        self.assertEqual(manifest.validate_manifest("tests"), [])

    def test_benchmarks_manifest_covers_all_gscript_cases(self):
        self.assertEqual(manifest.validate_manifest("benchmarks"), [])

    def test_benchmark_case_helpers_generate_expected_metadata(self):
        root = manifest.ROOT
        path = root / "benchmarks" / "numeric" / "matmul.gs"

        self.assertEqual(manifest.case_id("benchmarks", path), "numeric/matmul")
        self.assertEqual(manifest.domain_for("benchmarks", path), "numeric")
        self.assertEqual(manifest.tags_for("benchmarks", "numeric", "benchmarks/lua_ref/numeric/matmul.lua"), ["numeric", "benchmark"])
        self.assertEqual(manifest.status_for("benchmarks"), "active")

    def test_iter_gscript_cases_includes_only_benchmark_domains(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            for rel in (
                "benchmarks/numeric/keep.gs",
                "benchmarks/lua_ref/numeric/skip.gs",
                "benchmarks/not_a_domain/skip.gs",
                "benchmarks/__pycache__/skip.gs",
            ):
                path = root / rel
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("-- test\n")

            manifest.ROOT = root
            try:
                self.assertEqual(
                    [path.relative_to(root).as_posix() for path in manifest.iter_gscript_cases("benchmarks")],
                    ["benchmarks/numeric/keep.gs"],
                )
            finally:
                manifest.ROOT = original_root

    def test_tests_manifest_discovery_includes_restructured_test_dirs(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            for rel in (
                "tests/llm/agent_case.gs",
                "tests/integration/llm/provider_case.gs",
                "tests/sdk/api_case.gs",
                "tests/__pycache__/skip.gs",
            ):
                path = root / rel
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("-- test\n")

            manifest.ROOT = root
            try:
                self.assertEqual(
                    [path.relative_to(root).as_posix() for path in manifest.iter_gscript_cases("tests")],
                    [
                        "tests/integration/llm/provider_case.gs",
                        "tests/llm/agent_case.gs",
                        "tests/sdk/api_case.gs",
                    ],
                )
                self.assertEqual(
                    [(case["path"], case["domain"]) for case in manifest.discover_cases("tests")],
                    [
                        ("tests/integration/llm/provider_case.gs", "integration"),
                        ("tests/llm/agent_case.gs", "llm"),
                        ("tests/sdk/api_case.gs", "sdk"),
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
                "tests/language/case.gs",
                "tests/language/case.lua",
                "benchmarks/string/case.gs",
                "benchmarks/lua_ref/string/case.lua",
            ):
                path = root / rel
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("-- test\n")

            manifest.ROOT = root
            try:
                self.assertEqual(
                    manifest.lua_ref_for("tests", root / "tests" / "language" / "case.gs"),
                    "tests/language/case.lua",
                )
                self.assertEqual(
                    manifest.lua_ref_for("benchmarks", root / "benchmarks" / "string" / "case.gs"),
                    "benchmarks/lua_ref/string/case.lua",
                )
            finally:
                manifest.ROOT = original_root

    def test_generated_benchmark_manifest_uses_domain_workloads_and_historical_compatibility(self):
        original_root = manifest.ROOT
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            bench = root / "benchmarks" / "numeric" / "case.gs"
            bench.parent.mkdir(parents=True)
            bench.write_text("-- test\n")
            old_manifest = {
                "benchmarks": [
                    {
                        "id": "numeric/case",
                        "group": "numeric",
                        "name": "case",
                        "gscript_path": "benchmarks/numeric/case.gs",
                        "lua_path": "benchmarks/lua_ref/numeric/case.lua",
                        "params": {},
                        "recommended_scale": {"hot": {}},
                        "time_source_hint": "script_time_line",
                        "tags": ["numeric"],
                    }
                ],
                "time_source_hints": {"numeric/case": "script_time_line"},
            }
            (root / "benchmarks" / "manifest.json").write_text(json.dumps(old_manifest))

            manifest.ROOT = root
            try:
                generated = manifest.generated_manifest("benchmarks")
                self.assertEqual(generated["schema_version"], manifest.BENCHMARK_SCHEMA_VERSION)
                self.assertEqual(generated["domains"], list(manifest.BENCHMARK_DOMAINS))
                self.assertNotIn("groups", generated)
                self.assertNotIn("benchmarks", generated)
                self.assertEqual(generated["workloads"][0]["domain"], "numeric")
                self.assertEqual(
                    generated["workloads"][0]["comparison_reference"],
                    {"kind": "lua", "path": "benchmarks/lua_ref/numeric/case.lua"},
                )
                self.assertEqual(
                    generated["compatibility"]["historical"]["benchmarks"][0]["group"],
                    "numeric",
                )
            finally:
                manifest.ROOT = original_root


if __name__ == "__main__":
    unittest.main()
