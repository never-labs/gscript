import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import perf_submit_guard as guard


def timing_payload(rows):
    results = []
    for row in rows:
        name, current, luajit = row[:3]
        current_source = row[3] if len(row) > 3 else None
        luajit_source = row[4] if len(row) > 4 else None
        current_subject = {"status": "ok", "stats": {"median": current}}
        luajit_subject = {"status": "ok", "stats": {"median": luajit}}
        if current_source is not None:
            current_subject["source"] = current_source
        if luajit_source is not None:
            luajit_subject["source"] = luajit_source
        results.append(
            {
                "benchmark": name.split("/", 1)[1],
                "group": name.split("/", 1)[0],
                "modes": {
                    "default": {
                        "current": current_subject,
                        "luajit": luajit_subject,
                    }
                },
            }
        )
    return {"results": results}


def timing_payload_with_luajit_status(name, current, luajit_status):
    group, bench = name.split("/", 1)
    return {
        "results": [
            {
                "benchmark": bench,
                "group": group,
                "modes": {
                    "default": {
                        "current": {"status": "ok", "stats": {"median": current}, "source": "script_repeat"},
                        "luajit": {"status": luajit_status, "source": ""},
                    }
                },
            }
        ]
    }


class PerfSubmitGuardTest(unittest.TestCase):
    def test_rejects_luajit_ratio_above_threshold(self):
        rows = guard.load_rows(write_json(timing_payload([("numeric/a", 0.81, 1.0)])))
        violations = guard.check_rows(rows, ratio_threshold=0.8)
        self.assertEqual([(v.kind, v.name) for v in violations], [("luajit", "numeric/a")])

    def test_rejects_regression_against_baseline(self):
        candidate = guard.load_rows(write_json(timing_payload([("numeric/a", 0.75, 1.0)])))
        baseline = guard.load_rows(write_json(timing_payload([("numeric/a", 0.70, 1.0)])))
        violations = guard.check_rows(candidate, baseline=baseline, ratio_threshold=0.8, regression_tolerance=0.03)
        self.assertEqual([(v.kind, v.name) for v in violations], [("regression", "numeric/a")])

    def test_accepts_under_threshold_without_regression(self):
        candidate = guard.load_rows(write_json(timing_payload([("numeric/a", 0.72, 1.0)])))
        baseline = guard.load_rows(write_json(timing_payload([("numeric/a", 0.71, 1.0)])))
        self.assertEqual(guard.check_rows(candidate, baseline=baseline, ratio_threshold=0.8), [])

    def test_skips_luajit_ratio_for_mixed_timing_sources(self):
        rows = guard.load_rows(write_json(timing_payload([("numeric/a", 0.02, 0.01, "wall_repeat", "script_repeat")])))
        self.assertEqual(guard.check_rows(rows, ratio_threshold=0.8), [])
        self.assertNotIn("numeric/a", guard.format_summary(rows, []))

    def test_skips_baseline_regression_for_mixed_current_sources(self):
        candidate = guard.load_rows(write_json(timing_payload([("numeric/a", 0.90, 1.50, "wall_repeat", "script_repeat")])))
        baseline = guard.load_rows(write_json(timing_payload([("numeric/a", 0.70, 1.50, "script_repeat", "script_repeat")])))
        self.assertEqual(
            guard.check_rows(candidate, baseline=baseline, ratio_threshold=0.8, regression_tolerance=0.03),
            [],
        )

    def test_skips_luajit_requirement_for_manifested_leia_only_benchmarks(self):
        rows = guard.load_rows(
            write_json(timing_payload_with_luajit_status("data/q_columnar_qsql_filter_project", 0.42, "missing"))
        )
        self.assertEqual(
            guard.check_rows(rows, luajit_required=guard.load_luajit_required_benchmarks(), ratio_threshold=0.8),
            [],
        )

    def test_keeps_luajit_requirement_for_manifested_comparison_benchmarks(self):
        rows = guard.load_rows(write_json(timing_payload_with_luajit_status("numeric/matmul_dense", 0.01, "missing")))
        violations = guard.check_rows(rows, luajit_required=guard.load_luajit_required_benchmarks(), ratio_threshold=0.8)
        self.assertEqual([(v.kind, v.name) for v in violations], [("missing", "numeric/matmul_dense")])


def write_json(payload):
    td = tempfile.TemporaryDirectory()
    path = Path(td.name) / "timing.json"
    path.write_text(json.dumps(payload))
    write_json.keepalive.append(td)
    return path


write_json.keepalive = []


if __name__ == "__main__":
    unittest.main()
