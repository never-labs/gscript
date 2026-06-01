import subprocess
import unittest
from pathlib import Path

import benchmark_discovery as discovery
import conformance_perf_coverage as coverage
import profile_exits
import regression_guard
import validate_lua_refs


ROOT = Path(__file__).resolve().parents[1]


def module_path() -> str:
    for line in (ROOT / "go.mod").read_text().splitlines():
        if line.startswith("module "):
            return line.split()[1]
    raise AssertionError("go.mod has no module declaration")


class ScriptEntrypointConsistencyTest(unittest.TestCase):
    def test_diag_shell_uses_shared_discovery(self):
        diag = (ROOT / "scripts" / "diag.sh").read_text()
        self.assertIn("import benchmark_discovery as discovery", diag)
        self.assertIn("discovery.GROUPS", diag)
        self.assertIn("selector in discovery.GROUPS", diag)
        self.assertIn("discovery.resolve_script_path(root, selector)", diag)
        self.assertNotIn("domain_list_for()", diag)

    def test_benchmark_shell_wrappers_exec_matching_python(self):
        for stem in ("regression_guard", "strict_guard"):
            with self.subTest(stem=stem):
                wrapper = (ROOT / "benchmarks" / f"{stem}.sh").read_text()
                self.assertIn(f'exec python3 benchmarks/{stem}.py "$@"', wrapper)

    def test_scripts_performance_gate_wraps_benchmark_python_tools(self):
        gate = (ROOT / "scripts" / "performance_gate.sh").read_text()
        self.assertIn("python3 benchmarks/timing_compare.py", gate)
        self.assertIn("python3 benchmarks/strict_guard.py", gate)
        self.assertIn("--quick-phase-smoke", gate)
        self.assertIn('PROFILE="quick_phase_smoke"', gate)

    def test_python_benchmark_entrypoints_share_discovery_groups(self):
        expected = tuple(discovery.GROUPS)
        self.assertEqual(coverage.BENCHMARK_GROUPS, expected)
        self.assertEqual(profile_exits.BENCHMARK_GROUPS, expected)
        self.assertEqual(regression_guard.BENCHMARK_GROUPS, expected)
        self.assertEqual(validate_lua_refs.BENCHMARK_GROUPS, expected)

    def test_release_scripts_gate_current_module_path(self):
        expected = module_path()
        for script in ("production_check.sh", "release_artifacts_check.sh"):
            with self.subTest(script=script):
                text = (ROOT / "scripts" / script).read_text()
                self.assertIn(expected, text)
                self.assertNotIn("github.com/leia/leia", text)
                self.assertNotIn("github.com/Never-Labs/leia", text)

    def test_production_check_full_plan_avoids_go_test_duplicates(self):
        text = (ROOT / "scripts" / "production_check.sh").read_text()
        full_plan = text.split("build_full_plan() {", 1)[1].split("\n}", 1)[0]

        self.assertIn('add_go_test "Correctness"', full_plan)
        self.assertIn('add_skip "Feature Matrix" "covered by Correctness', full_plan)
        self.assertIn('add_skip "Language Conformance Surface" "covered by Correctness', full_plan)
        self.assertIn('add_skip "Release Matrix Metadata" "covered by Correctness', full_plan)
        self.assertNotIn('add_go_test "Feature Matrix"', full_plan)
        self.assertNotIn('add_go_test "Language Conformance Surface"', full_plan)
        self.assertNotIn('add_go_test "Release Matrix Metadata"', full_plan)

    def test_scripts_have_valid_type_specific_syntax(self):
        scripts = sorted(path for path in (ROOT / "scripts").iterdir() if path.is_file())
        self.assertGreater(len(scripts), 0)
        for script in scripts:
            with self.subTest(script=script.name):
                if script.suffix == ".sh":
                    cmd = ["bash", "-n", str(script)]
                elif script.suffix == ".py":
                    cmd = ["python3", "-m", "py_compile", str(script)]
                else:
                    self.fail(f"unsupported script entrypoint type: {script.name}")
                subprocess.run(
                    cmd,
                    cwd=ROOT,
                    check=True,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    text=True,
                )


if __name__ == "__main__":
    unittest.main()
