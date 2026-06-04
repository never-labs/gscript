import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "performance_gate.sh"


def subject(median, status="ok", source="script_repeat", cv=2.0):
    return {
        "status": status,
        "source": source,
        "stats": {
            "median": median,
            "cv_pct": cv,
        },
    }


def timing_payload(current, head, *, current_status="ok", head_status="ok", source="script_repeat"):
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
