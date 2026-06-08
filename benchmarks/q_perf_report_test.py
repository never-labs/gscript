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
BenchmarkQSessionEvalVectorWarmExecution/MaskWhere-16    100  2000 ns/op  256 B/op  8 allocs/op
BenchmarkQEvalVectorGoBaseline/MaskWhere-16              100  1000 ns/op  0 B/op  0 allocs/op
"""


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

    def test_coverage_marks_qeval_kernel_stats_gap(self):
        rows = report.parse_go_benchmarks(SAMPLE)
        coverage = {row["signal"]: row for row in report.build_coverage(rows)}

        self.assertEqual(coverage["typed kernel hit rate"]["qSQL"], "covered")
        self.assertEqual(coverage["typed kernel hit rate"]["q.eval"], "missing")
        self.assertIn("q.eval", coverage["fallback rate"]["gap"])
        self.assertEqual(coverage["allocs/op"]["q.eval"], "covered")

    def test_main_can_write_report_from_existing_output(self):
        with tempfile.TemporaryDirectory() as td:
            td_path = Path(td)
            out = td_path / "bench.txt"
            out.write_text(SAMPLE)
            json_path = td_path / "report.json"
            md_path = td_path / "report.md"

            code = report.main(["--from-output", str(out), "--json", str(json_path), "--markdown", str(md_path)])

            self.assertEqual(code, 0)
            payload = json.loads(json_path.read_text())
            self.assertIn("BenchmarkQEvalVectorResultCacheWarm/MaskWhere", payload["benchmarks"])
            self.assertIn("q Performance Completeness Report", md_path.read_text())


if __name__ == "__main__":
    unittest.main()
