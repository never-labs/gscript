import json
import contextlib
import io
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
BenchmarkQSessionEvalVectorWarmExecution/MaskWhere-16    100  2000 ns/op  256 B/op  8 allocs/op  100.0 typed_kernel_hit_pct  1 typed_kernel_attempts/op  1 typed_kernel_hits/op  0 typed_kernel_fallbacks/op  0 typed_kernel_errors/op  2 typed_pipeline_shapes  0 typed_pipeline_fallback_shapes  1 q_pipeline_category_where_project_reduce
BenchmarkQEvalVectorGoBaseline/MaskWhere-16              100  1000 ns/op  0 B/op  0 allocs/op
BenchmarkQEvalPipelineNativeExitCallpath/CodegenNativeExit/ModuloWhereCount-16  100  3000 ns/op  128 B/op  4 allocs/op  1 jit_typed_direct_return/op  0 jit_typed_native_exit/op  0 jit_typed_op_exit/op  1 jit_typed_kernel_success/op  0 jit_typed_kernel_errors/op  1 jit_typed_pipeline_shapes
"""

SAMPLE_WITH_FALLBACK = """
BenchmarkQSessionEvalVectorWarmExecution/FallbackShape-16    100  9000 ns/op  2048 B/op  90 allocs/op  80.0 typed_kernel_hit_pct  1 typed_kernel_attempts/op  0.8 typed_kernel_hits/op  0.2 typed_kernel_fallbacks/op  0 typed_kernel_errors/op  2 typed_pipeline_shapes  1 typed_pipeline_fallback_shapes  1 q_pipeline_category_xbar_within
BenchmarkQEvalVectorGoBaseline/FallbackShape-16              100  1000 ns/op  0 B/op  0 allocs/op
"""

FALLBACK_REPORT_LOG = """
    q_eval_vector_bench_test.go:2844: q_pipeline_fallback_report cases=2 categories=2 rows=1
    q_eval_vector_bench_test.go:2844: q_pipeline_fallback_report rank=1 category=xbar_within pipeline_shape=bin kernel=ArrayBin reason=unsupported_type outcome=fallback count=3
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
        self.assertEqual(qeval.typed_pipeline_shapes, 2)
        self.assertEqual(qeval.typed_pipeline_fallback_shapes, 0)
        qjit = metrics["BenchmarkQEvalPipelineNativeExitCallpath/CodegenNativeExit/ModuloWhereCount"]
        self.assertEqual(qjit.jit_typed_direct_return_op, 1)
        self.assertEqual(qjit.jit_typed_native_exit_op, 0)
        self.assertEqual(qjit.jit_typed_op_exit_op, 0)
        self.assertEqual(qjit.jit_typed_kernel_success_op, 1)
        self.assertEqual(qjit.jit_typed_kernel_errors_op, 0)
        self.assertEqual(qjit.jit_typed_pipeline_shapes, 1)

    def test_jit_route_summary_aggregates_route_metrics(self):
        rows = report.parse_go_benchmarks(SAMPLE)
        summary = {row.route: row for row in report.build_jit_route_summary(rows)}

        self.assertEqual(summary["direct_return"].calls_per_op, 1)
        self.assertEqual(summary["direct_return"].share_pct, 100)
        self.assertEqual(summary["direct_return"].benchmark_count, 1)
        self.assertEqual(summary["native_exit"].calls_per_op, 0)
        self.assertEqual(summary["op_exit"].calls_per_op, 0)
        self.assertEqual(summary["success"].calls_per_op, 1)
        self.assertEqual(summary["error"].calls_per_op, 0)

    def test_gate_checks_cover_ratio_hit_rate_fallback_and_allocs(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_WITH_FALLBACK)
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
        )
        checks = report.build_gate_checks(rows, policy)
        failed = {(check.signal, check.benchmark) for check in checks if check.status == "fail"}

        self.assertIn(("leia_go_ratio", "BenchmarkQSessionEvalVectorWarmExecution/FallbackShape"), failed)
        self.assertIn(("typed_hit_pct", "BenchmarkQSessionEvalVectorWarmExecution/FallbackShape"), failed)
        self.assertIn(("fallbacks_op", "BenchmarkQSessionEvalVectorWarmExecution/FallbackShape"), failed)
        self.assertIn(("pipeline_fallback_shapes", "BenchmarkQSessionEvalVectorWarmExecution/FallbackShape"), failed)
        self.assertIn(("allocs_op", "BenchmarkQSessionEvalVectorWarmExecution/FallbackShape"), failed)
        self.assertTrue(report.gate_failed(checks))

    def test_fallback_shape_summary_filters_only_rows_with_fallback_pressure(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_WITH_FALLBACK)
        fallback_rows = report.build_fallback_shape_rows(rows)

        self.assertEqual([row.benchmark for row in fallback_rows], ["BenchmarkQSessionEvalVectorWarmExecution/FallbackShape"])

    def test_pipeline_category_metrics_group_benchmark_rows(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_WITH_FALLBACK)
        categories = {row.category: row for row in report.build_pipeline_category_metric_rows(rows)}

        self.assertEqual(categories["where_project_reduce"].benchmark_count, 1)
        self.assertEqual(categories["where_project_reduce"].avg_allocs_op, 8)
        self.assertEqual(categories["xbar_within"].total_fallback_shapes, 1)

    def test_parse_pipeline_fallback_report_logs_top_rows(self):
        rows = report.parse_q_pipeline_fallback_reports(FALLBACK_REPORT_LOG + FALLBACK_REPORT_LOG)

        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0].category, "xbar_within")
        self.assertEqual(rows[0].pipeline_shape, "bin")
        self.assertEqual(rows[0].kernel, "ArrayBin")
        self.assertEqual(rows[0].reason, "unsupported_type")
        self.assertEqual(rows[0].count, 6)

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

            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
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
            self.assertIn("jit_route_summary", payload)
            self.assertEqual(payload["jit_route_summary"][0]["route"], "direct_return")
            self.assertIn("pipeline_category_metrics", payload)
            self.assertIn("pipeline_fallback_top", payload)
            self.assertIn("fallback_shape_summary", payload)
            self.assertIn("gate_policy", payload)
            markdown = md_path.read_text()
            self.assertIn("q Performance Completeness Report", markdown)
            self.assertIn("Current vs Old Leia", markdown)
            self.assertIn("Gate Summary", markdown)
            self.assertIn("JIT Typed Runtime Routes", markdown)
            self.assertIn("Pipeline Category Metrics", markdown)
            self.assertIn("Pipeline Fallback Top-N", markdown)

    def test_main_check_returns_nonzero_for_gate_failures(self):
        with tempfile.TemporaryDirectory() as td:
            td_path = Path(td)
            out = td_path / "bench.txt"
            out.write_text(SAMPLE_WITH_FALLBACK)
            json_path = td_path / "report.json"
            md_path = td_path / "report.md"

            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                code = report.main([
                    "--from-output", str(out),
                    "--json", str(json_path),
                    "--markdown", str(md_path),
                    "--check",
                ])

            self.assertEqual(code, 2)
            payload = json.loads(json_path.read_text())
            self.assertTrue(any(row["status"] == "fail" for row in payload["gate"]))
            self.assertIn("Fallback Shape Summary", md_path.read_text())


if __name__ == "__main__":
    unittest.main()
