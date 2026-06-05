import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "performance_gate.sh"


def shell_array_values(text, name):
    start = text.index(f"{name}=(")
    end = text.index("\n)", start)
    values = []
    for line in text[start:end].splitlines()[1:]:
        stripped = line.strip()
        if stripped.startswith('"') and stripped.endswith('"'):
            values.append(stripped.strip('"'))
    return values


def subject(median, status="ok", source="script_repeat", cv=2.0):
    return {
        "status": status,
        "source": source,
        "stats": {
            "median": median,
            "cv_pct": cv,
        },
    }


def timing_payload(
    current,
    head,
    *,
    luajit=2.0,
    current_status="ok",
    head_status="ok",
    luajit_status="ok",
    source="script_repeat",
):
    return {
        "modes": ["default"],
        "results": [
            {
                "group": "numeric",
                "benchmark": "hot_loop",
                "modes": {
                    "default": {
                        "current": subject(current, current_status, source),
                        "head": subject(head, head_status, source),
                        "luajit": subject(luajit, luajit_status, source),
                    }
                },
            }
        ],
    }


def run_validate(payload, *args):
    with tempfile.TemporaryDirectory() as td:
        path = Path(td) / "timing.json"
        path.write_text(json.dumps(payload))
        return subprocess.run(
            ["bash", str(SCRIPT), "--validate-only", str(path), *args],
            cwd=ROOT,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            check=False,
        )


class PerformanceGateValidationTest(unittest.TestCase):
    def test_validate_only_accepts_flat_script_timed_row(self):
        proc = run_validate(timing_payload(1.05, 1.00))
        self.assertEqual(proc.returncode, 0, proc.stdout)
        self.assertIn("Performance gate current/HEAD ranking", proc.stdout)
        self.assertIn("Performance gate passed.", proc.stdout)

    def test_quick_phase_smoke_is_explicit_parseable_profile(self):
        proc = run_validate(timing_payload(1.05, 1.00), "--quick-phase-smoke")
        self.assertEqual(proc.returncode, 0, proc.stdout)
        self.assertIn("Performance gate passed.", proc.stdout)

    def test_syntax_smoke_is_explicit_parseable_profile(self):
        proc = run_validate(timing_payload(1.05, 1.00), "--syntax-smoke")
        self.assertEqual(proc.returncode, 0, proc.stdout)
        self.assertIn("Performance gate passed.", proc.stdout)

    def test_syntax_smoke_includes_dialect_guard(self):
        gate = SCRIPT.read_text()
        self.assertIn('"app/dialect_syntax_smoke"', gate)

    def test_syntax_smoke_keeps_dialect_out_of_timing_comparisons(self):
        gate = SCRIPT.read_text()
        self.assertNotIn("app/dialect_syntax_smoke", shell_array_values(gate, "SYNTAX_SMOKE_BENCHES"))
        self.assertEqual(
            shell_array_values(gate, "SYNTAX_DIALECT_SMOKE_BENCHES"),
            ["app/dialect_syntax_smoke"],
        )

    def test_syntax_smoke_strict_pass_uses_leia_only_modes_for_dialect_guard(self):
        gate = SCRIPT.read_text()
        self.assertIn(
            'if [ "$PROFILE" = "syntax_smoke" ]; then\n'
            "        STRICT_CMD+=(--mode vm --mode default --mode no_filter)\n"
            "    fi",
            gate,
        )

    def test_builtin_gate_selectors_are_registered_manifest_workloads(self):
        gate = SCRIPT.read_text()
        manifest = json.loads((ROOT / "benchmarks" / "manifest.json").read_text())
        case_ids = {case["id"] for case in manifest["cases"]}
        workload_ids = {workload["id"] for workload in manifest["workloads"]}
        array_names = (
            "CORE_BENCHES",
            "SMOKE_BENCHES",
            "SYNTAX_SMOKE_BENCHES",
            "SYNTAX_DIALECT_SMOKE_BENCHES",
            "STRICT_SMOKE_BENCHES",
            "PHASE_SMOKE_BENCHES",
            "FEATURE_SMOKE_BENCHES",
            "STRICT_CORE_BENCHES",
            "STRICT_FEATURE_BENCHES",
        )

        selectors = {
            selector
            for name in array_names
            for selector in shell_array_values(gate, name)
        }

        self.assertEqual(sorted(selectors - case_ids), [])
        self.assertEqual(sorted(selectors - workload_ids), [])

    def test_feature_smoke_keeps_data_oriented_dense_and_soa_gate(self):
        gate = SCRIPT.read_text()
        for array_name in ("PHASE_SMOKE_BENCHES", "FEATURE_SMOKE_BENCHES", "STRICT_FEATURE_BENCHES"):
            values = shell_array_values(gate, array_name)
            self.assertIn("numeric/matmul_dense", values)
            self.assertIn("data/soa_affine_many", values)
            self.assertIn("data/soa_masked_aggregate", values)

    def test_help_documents_syntax_smoke_profile(self):
        proc = subprocess.run(
            ["bash", str(SCRIPT), "--help"],
            cwd=ROOT,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            check=False,
        )
        self.assertEqual(proc.returncode, 0, proc.stdout)
        self.assertIn("--syntax-smoke", proc.stdout)
        self.assertIn("grammar-change hot-path gate", proc.stdout)

    def test_validate_only_rejects_obvious_regression(self):
        proc = run_validate(timing_payload(1.20, 1.00), "--threshold", "0.10")
        self.assertEqual(proc.returncode, 1, proc.stdout)
        self.assertIn("Performance gate violations", proc.stdout)
        self.assertIn("numeric/hot_loop", proc.stdout)

    def test_validate_only_rejects_luajit_ratio_above_threshold(self):
        proc = run_validate(timing_payload(0.81, 1.00, luajit=1.00))
        self.assertEqual(proc.returncode, 1, proc.stdout)
        self.assertIn("Guard violations", proc.stdout)
        self.assertIn("numeric/hot_loop", proc.stdout)
        self.assertIn("luajit", proc.stdout)

    def test_validate_only_rejects_low_resolution_rows(self):
        proc = run_validate(timing_payload(None, 1.00, current_status="low_resolution"))
        self.assertEqual(proc.returncode, 1, proc.stdout)
        self.assertIn("Unreliable timing rows", proc.stdout)
        self.assertIn("low_resolution/ok", proc.stdout)

    def test_wall_timed_rows_need_larger_regression_to_fail(self):
        proc = run_validate(timing_payload(1.20, 1.00, source="wall_repeat"), "--threshold", "0.10")
        self.assertEqual(proc.returncode, 0, proc.stdout)
        self.assertIn("wall_timed_startup_noise", proc.stdout)

        proc = run_validate(
            timing_payload(1.40, 1.00, source="wall_repeat"),
            "--threshold",
            "0.10",
            "--wall-threshold",
            "0.30",
        )
        self.assertEqual(proc.returncode, 1, proc.stdout)
        self.assertIn("wall_regression", proc.stdout)


if __name__ == "__main__":
    unittest.main()
