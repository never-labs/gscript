import json
import tempfile
import unittest
from pathlib import Path

import q_perf_report as report


SAMPLE = """
BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject-16    100  1000 ns/op  8192 input_rows/s  200 B/op  3 allocs/op  99.0 kernel_hit_pct  0 fallbacks/op
BenchmarkQSQLBindRunSQLColdCacheSelectWhereProject-16    100  4000 ns/op  8192 input_rows/s  300 B/op  5 allocs/op  0 kernel_hit_pct  0 fallbacks/op
BenchmarkQSQLNativeGoSelectWhereProject-16               100  2000 ns/op  8192 input_rows/s  100 B/op  1 allocs/op
BenchmarkQEvalVectorResultCacheWarm/MaskWhere-16         100  500 ns/op  64 B/op  2 allocs/op
BenchmarkQEvalVectorCold/MaskWhere-16                    100  2500 ns/op  512 B/op  12 allocs/op
BenchmarkQSessionEvalVectorWarmExecution/MaskWhere-16    100  2000 ns/op  256 B/op  8 allocs/op  100.0 typed_kernel_hit_pct  1 typed_kernel_attempts/op  1 typed_kernel_hits/op  0 typed_kernel_fallbacks/op  0 typed_kernel_errors/op
BenchmarkQEvalVectorGoBaseline/MaskWhere-16              100  1000 ns/op  0 B/op  0 allocs/op
"""

TIMING_PAYLOAD = {
    "results": [
        {
            "group": "data",
            "benchmark": "q_columnar_eval_primitives",
            "modes": {
                "default": {
                    "current": {"stats": {"median": 0.002}, "source": "script_repeat"},
                    "head": {"stats": {"median": 0.004}, "source": "script_repeat"},
                }
            },
        }
    ]
}


class QPerfReportTest(unittest.TestCase):
    def test_parse_go_benchmarks_reads_standard_and_custom_metrics(self):
        rows = report.parse_go_benchmarks(SAMPLE)

        qsql = rows["BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject"]
        self.assertEqual(qsql.iterations, 100)
        self.assertEqual(qsql.ns_op, 1000)
        self.assertEqual(qsql.metrics["B/op"], 200)
        self.assertEqual(qsql.metrics["allocs/op"], 3)
        self.assertEqual(qsql.metrics["kernel_hit_pct"], 99)
        self.assertEqual(qsql.metrics["fallbacks/op"], 0)

    def test_build_ratios_includes_qsql_and_qeval(self):
        rows = report.parse_go_benchmarks(SAMPLE)
        ratios = {row.scenario: row.ratio for row in report.build_ratios(rows)}

        self.assertEqual(ratios["qSQL select/filter/project"], 0.5)
        self.assertEqual(ratios["qSQL select/filter/project warm-vs-cold"], 0.25)
        self.assertEqual(ratios["q.eval MaskWhere session execution vs Go"], 2.0)
        self.assertEqual(ratios["q.eval MaskWhere result-cache warm vs session execution"], 0.25)
        self.assertEqual(ratios["q.eval MaskWhere cold vs session execution"], 1.25)

    def test_coverage_marks_qeval_kernel_stats_covered(self):
        rows = report.parse_go_benchmarks(SAMPLE)
        coverage = {row["signal"]: row for row in report.build_coverage(rows)}

        self.assertEqual(coverage["current Leia vs old Leia"]["qSQL"], "missing")
        self.assertEqual(coverage["typed kernel hit rate"]["qSQL"], "covered")
        self.assertEqual(coverage["typed kernel hit rate"]["q.eval"], "covered")
        self.assertEqual(coverage["fallback rate"]["q.eval"], "covered")
        self.assertEqual(coverage["allocs/op"]["q.eval"], "covered")

    def test_timing_compare_json_covers_current_vs_old(self):
        rows = report.parse_go_benchmarks(SAMPLE)
        current_vs_old = report.parse_timing_compare_payload(TIMING_PAYLOAD)

        self.assertEqual(len(current_vs_old), 1)
        self.assertEqual(current_vs_old[0].benchmark, "data/q_columnar_eval_primitives")
        self.assertEqual(current_vs_old[0].ratio, 0.5)

        coverage = {row["signal"]: row for row in report.build_coverage(rows, current_vs_old)}
        self.assertEqual(coverage["current Leia vs old Leia"]["qSQL"], "covered")

    def test_runtime_metrics_structures_allocs_kernel_and_fallback_values(self):
        rows = report.parse_go_benchmarks(SAMPLE)
        metrics = {row.benchmark: row for row in report.build_runtime_metric_rows(rows)}

        qsql = metrics["BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject"]
        self.assertEqual(qsql.bytes_op, 200)
        self.assertEqual(qsql.allocs_op, 3)
        self.assertEqual(qsql.kernel_hit_pct, 99)
        self.assertEqual(qsql.fallbacks_op, 0)
        qeval = metrics["BenchmarkQSessionEvalVectorWarmExecution/MaskWhere"]
        self.assertEqual(qeval.typed_kernel_hit_pct, 100)
        self.assertEqual(qeval.typed_kernel_attempts_op, 1)
        self.assertEqual(qeval.typed_kernel_fallbacks_op, 0)

    def test_qsql_benchmark_coverage_reports_missing_expected_rows(self):
        rows = report.parse_go_benchmarks(SAMPLE)
        coverage = report.build_qsql_benchmark_coverage(rows)

        self.assertEqual(coverage.leia_case_count, 2)
        self.assertEqual(coverage.native_go_case_count, 1)
        self.assertEqual(coverage.data_runtime_case_count, 0)
        self.assertIn("BenchmarkQSQLBindRunSQLWarmCacheGroupByAggregate", coverage.missing_expected)

    def test_main_can_write_report_from_existing_output(self):
        with tempfile.TemporaryDirectory() as td:
            td_path = Path(td)
            out = td_path / "bench.txt"
            out.write_text(SAMPLE)
            timing = td_path / "timing.json"
            timing.write_text(json.dumps(TIMING_PAYLOAD))
            json_path = td_path / "report.json"
            md_path = td_path / "report.md"

            code = report.main([
                "--from-output", str(out),
                "--timing-json", str(timing),
                "--json", str(json_path),
                "--markdown", str(md_path),
            ])

            self.assertEqual(code, 0)
            payload = json.loads(json_path.read_text())
            self.assertIn("BenchmarkQEvalVectorResultCacheWarm/MaskWhere", payload["benchmarks"])
            self.assertIn("qsql_benchmark_coverage", payload)
            self.assertEqual(payload["current_vs_old"][0]["ratio"], 0.5)
            self.assertIn("runtime_metrics", payload)
            markdown = md_path.read_text()
            self.assertIn("q Performance Completeness Report", markdown)
            self.assertIn("Current vs Old Leia", markdown)


if __name__ == "__main__":
    unittest.main()
