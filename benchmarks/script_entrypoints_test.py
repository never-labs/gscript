import os
import subprocess
import sys
import unittest
from pathlib import Path

sys.dont_write_bytecode = True

import benchmark_discovery as discovery
import timing_compare


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
        regression = (ROOT / "benchmarks" / "regression_guard.sh").read_text()
        self.assertIn('exec go run ./cmd/leia bench regression-guard "$@"', regression)
        strict = (ROOT / "benchmarks" / "strict_guard.sh").read_text()
        self.assertIn('exec go run ./cmd/leia bench strict "$@"', strict)

    def test_q_columnar_suite_wraps_timing_compare(self):
        wrapper = (ROOT / "benchmarks" / "q_columnar_suite.sh").read_text()
        self.assertIn("go run ./cmd/leia bench compare", wrapper)
        self.assertIn("--no-luajit", wrapper)
        for bench in (
            "data/q_columnar_eval_primitives",
            "data/q_columnar_qsql_filter_project",
            "data/q_columnar_qsql_group_xbar",
            "data/q_columnar_qsql_asof_join",
        ):
            with self.subTest(bench=bench):
                self.assertIn(f"--bench={bench}", wrapper)

    def test_scripts_performance_gate_wraps_benchmark_python_tools(self):
        gate = (ROOT / "scripts" / "performance_gate.sh").read_text()
        self.assertIn("go run ./cmd/leia bench compare", gate)
        self.assertIn("go run ./cmd/leia bench strict", gate)
        self.assertIn("--progress", gate)
        self.assertIn('--jobs="$JOBS"', gate)
        self.assertIn("table/table_field_access", gate)
        self.assertIn("--syntax-smoke", gate)
        self.assertIn("SYNTAX_SMOKE_BENCHES=(", gate)
        self.assertIn('PROFILE="syntax_smoke"', gate)
        self.assertIn('STRICT=0', gate)
        self.assertIn("--quick-phase-smoke", gate)
        self.assertIn('PROFILE="quick_phase_smoke"', gate)
        self.assertIn("STRICT_SMOKE_BENCHES=(", gate)
        self.assertIn('for bench in "${STRICT_SMOKE_BENCHES[@]}"; do', gate)
        self.assertIn('if [ "$PROFILE" = "full" ]; then', gate)
        self.assertIn('for bench in "${STRICT_CORE_BENCHES[@]}"; do', gate)

    def test_python_benchmark_entrypoints_share_discovery_groups(self):
        expected = tuple(discovery.GROUPS)
        self.assertEqual(tuple(timing_compare.GROUPS), expected)

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
        self.assertIn('add_skip "Release Matrix Metadata" "covered by Correctness', full_plan)
        self.assertNotIn('add_go_test "Feature Matrix"', full_plan)
        self.assertNotIn('add_go_test "Release Matrix Metadata"', full_plan)

    def test_release_profile_requires_release_only_tool_smokes(self):
        production = (ROOT / "scripts" / "production_check.sh").read_text()
        distribution = (ROOT / "scripts" / "release_distribution_check.sh").read_text()
        ci = (ROOT / "cmd" / "leia" / "ci.go").read_text()

        self.assertIn("--release-profile", production)
        self.assertIn("scripts/run.sh editor --require-tree-sitter", production)
        self.assertIn("scripts/run.sh release-dist --require-goreleaser", production)
        self.assertIn("scripts/run.sh release-check", production)
        self.assertIn("--require-goreleaser", distribution)
        self.assertIn("goreleaser CLI is required for release distribution profile", distribution)
        self.assertIn('"--full", "--release-profile"', ci)

    def test_production_release_profile_list_includes_required_tool_flags(self):
        proc = subprocess.run(
            ["bash", "scripts/production_check.sh", "--full", "--release-profile", "--list"],
            cwd=ROOT,
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
        )

        self.assertIn("Release profile: critical release tool skips are treated as failures.", proc.stdout)
        self.assertIn("scripts/run.sh editor --require-tree-sitter", proc.stdout)
        self.assertIn("scripts/run.sh release-dist --require-goreleaser", proc.stdout)
        self.assertIn("scripts/run.sh release-check", proc.stdout)

    def test_release_distribution_require_goreleaser_fails_without_cli(self):
        env = os.environ.copy()
        env["PATH"] = "/usr/bin:/bin"
        if subprocess.run(
            ["bash", "-lc", "command -v goreleaser >/dev/null 2>&1"],
            env=env,
            check=False,
        ).returncode == 0:
            self.skipTest("goreleaser is available on the restricted PATH")

        proc = subprocess.run(
            ["bash", "scripts/release_distribution_check.sh", "--require-goreleaser"],
            cwd=ROOT,
            env=env,
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
        )

        self.assertEqual(proc.returncode, 1, proc.stdout)
        self.assertIn("goreleaser CLI is required for release distribution profile", proc.stdout)

    def test_scripts_have_valid_type_specific_syntax(self):
        scripts = sorted(path for path in (ROOT / "scripts").iterdir() if path.is_file())
        self.assertGreater(len(scripts), 0)
        for script in scripts:
            with self.subTest(script=script.name):
                if script.suffix == ".sh":
                    cmd = ["bash", "-n", str(script)]
                elif script.suffix == ".py":
                    cmd = [
                        "python3",
                        "-c",
                        (
                            "import ast, pathlib, sys; "
                            "path = pathlib.Path(sys.argv[1]); "
                            "ast.parse(path.read_text(encoding='utf-8'), filename=str(path))"
                        ),
                        str(script),
                    ]
                elif script.suffix == ".leia":
                    cmd = ["go", "run", "./cmd/leia", "lint", str(script)]
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
