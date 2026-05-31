import unittest
from pathlib import Path

import benchmark_discovery as discovery
import conformance_perf_coverage as coverage
import profile_exits
import regression_guard


ROOT = Path(__file__).resolve().parents[1]


class ScriptEntrypointConsistencyTest(unittest.TestCase):
    def test_diag_shell_uses_shared_discovery(self):
        diag = (ROOT / "scripts" / "diag.sh").read_text()
        self.assertIn("import benchmark_discovery as discovery", diag)
        self.assertIn("discovery.GROUPS", diag)
        self.assertIn("discovery.canonical_group(selector)", diag)
        self.assertIn("discovery.resolve_script_path(root, selector)", diag)
        self.assertNotIn("domain_list_for()", diag)
        for alias in discovery.LEGACY_GROUP_ALIASES:
            self.assertNotIn(f"{alias}) printf", diag)

    def test_benchmark_shell_wrappers_exec_matching_python(self):
        for stem in ("regression_guard", "strict_guard"):
            with self.subTest(stem=stem):
                wrapper = (ROOT / "benchmarks" / f"{stem}.sh").read_text()
                self.assertIn(f'exec python3 benchmarks/{stem}.py "$@"', wrapper)

    def test_scripts_performance_gate_wraps_benchmark_python_tools(self):
        gate = (ROOT / "scripts" / "performance_gate.sh").read_text()
        self.assertIn("python3 benchmarks/timing_compare.py", gate)
        self.assertIn("python3 benchmarks/strict_guard.py", gate)

    def test_python_benchmark_entrypoints_share_discovery_groups(self):
        expected = tuple(discovery.GROUPS)
        self.assertEqual(coverage.BENCHMARK_GROUPS, expected)
        self.assertEqual(profile_exits.BENCHMARK_GROUPS, expected)
        self.assertEqual(regression_guard.BENCHMARK_GROUPS, expected)


if __name__ == "__main__":
    unittest.main()
