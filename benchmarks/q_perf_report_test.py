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

SAMPLE_QSQL_MATRIX = """
BenchmarkQSQLBindMatrixWarm/JoinInnerAliasedKeyTopK-16    100  3000 ns/op  8192 input_rows/s  400 B/op  6 allocs/op  99.0 kernel_hit_pct  0 fallbacks/op
BenchmarkQSQLBindMatrixCold/JoinInnerAliasedKeyTopK-16    100  9000 ns/op  8192 input_rows/s  900 B/op  20 allocs/op  0 kernel_hit_pct  0 fallbacks/op
BenchmarkQSQLNativeGoMatrix/JoinInnerAliasedKeyTopK-16    100  1500 ns/op  8192 input_rows/s  100 B/op  1 allocs/op
"""

SAMPLE_WITH_FALLBACK = """
BenchmarkQSessionEvalVectorWarmExecution/FallbackShape-16    100  9000 ns/op  2048 B/op  90 allocs/op  80.0 typed_kernel_hit_pct  1 typed_kernel_attempts/op  0.8 typed_kernel_hits/op  0.2 typed_kernel_fallbacks/op  0 typed_kernel_errors/op  2 typed_pipeline_shapes  1 typed_pipeline_fallback_shapes  1 q_pipeline_category_xbar_within
BenchmarkQEvalVectorGoBaseline/FallbackShape-16              100  1000 ns/op  0 B/op  0 allocs/op
"""

SAMPLE_JIT_SLOW_ROUTE = """
BenchmarkQEvalPipelineNativeExitCallpath/SlowRoute-16  100  6000 ns/op  256 B/op  6 allocs/op  1 jit_typed_direct_return/op  2 jit_typed_native_exit/op  1 jit_typed_op_exit/op  3 jit_typed_kernel_success/op  1 jit_typed_kernel_errors/op  2 jit_typed_pipeline_shapes
"""

SAMPLE_BRIDGE_HEALTHY = """
BenchmarkQSessionEvalVectorWarmExecution/BridgeHealthy-16  100  900 ns/op  64 B/op  4 allocs/op  100.0 typed_kernel_hit_pct  3 typed_kernel_attempts/op  3 typed_kernel_hits/op  0 typed_kernel_fallbacks/op  0 typed_kernel_errors/op  3 typed_pipeline_shapes  0 typed_pipeline_fallback_shapes  1 q_pipeline_category_where_project_reduce  1 runtime_primitive_hits/op  0 runtime_primitive_errors/op
BenchmarkQEvalPipelineNativeExitCallpath/BridgeHealthy-16  100  700 ns/op  64 B/op  4 allocs/op  1 jit_typed_direct_return/op  0 jit_typed_native_exit/op  0 jit_typed_op_exit/op  1 jit_typed_kernel_success/op  0 jit_typed_kernel_errors/op  1 jit_typed_pipeline_shapes  2 methodjit_frame_runtime_success/op  0 methodjit_frame_runtime_errors/op  3 methodjit_vector_runtime_success/op  0 methodjit_vector_runtime_errors/op
"""

SAMPLE_ARRAY_BRIDGE = """
BenchmarkQEvalPipelineArrayRuntimeBridge/BulkI64Range-16       100  1200 ns/op  65648 B/op  2 allocs/op  1 q_array_bridge_bulk_hits/op  0 q_array_bridge_fallbacks/op  0 q_array_bridge_errors/op  8192 q_array_bridge_rows/op
BenchmarkQEvalPipelineArrayRuntimeBridge/BulkEncodedSymbol-16  100  1400 ns/op  131184 B/op  2 allocs/op  1 q_array_bridge_bulk_hits/op  0 q_array_bridge_fallbacks/op  0 q_array_bridge_errors/op  8192 q_array_bridge_rows/op
BenchmarkQEvalPipelineArrayRuntimeBridge/FallbackMixedAny-16   100  9000 ns/op  360648 B/op  7 allocs/op  0 q_array_bridge_bulk_hits/op  1 q_array_bridge_fallbacks/op  0 q_array_bridge_errors/op  8192 q_array_bridge_rows/op
"""

SAMPLE_ARRAY_BRIDGE_HEALTHY = """
BenchmarkQEvalPipelineArrayRuntimeBridge/BulkI64Range-16       100  1200 ns/op  65648 B/op  2 allocs/op  1 q_array_bridge_bulk_hits/op  0 q_array_bridge_fallbacks/op  0 q_array_bridge_errors/op  8192 q_array_bridge_rows/op
BenchmarkQEvalPipelineArrayRuntimeBridge/BulkEncodedSymbol-16  100  1400 ns/op  131184 B/op  2 allocs/op  1 q_array_bridge_bulk_hits/op  0 q_array_bridge_fallbacks/op  0 q_array_bridge_errors/op  8192 q_array_bridge_rows/op
"""

SAMPLE_BACKEND_ROUTE = """
BenchmarkRuntimePrimitiveRegistry/DenseArrayGather-16  100  800 ns/op  64 B/op  2 allocs/op  1 runtime_primitive_hits/op  0 runtime_primitive_errors/op
BenchmarkQFrameVectorMethodJITRoute/FrameVector-16    100  900 ns/op  96 B/op  3 allocs/op  2 methodjit_frame_runtime_success/op  0 methodjit_frame_runtime_errors/op  3 methodjit_vector_runtime_success/op  1 methodjit_vector_runtime_errors/op
"""

SAMPLE_DATA_RUNTIME = """
BenchmarkQSessionEvalVectorWarmExecution/LinalgFacade-16  100  1100 ns/op  96 B/op  3 allocs/op  100.0 data_runtime_hit_pct  4 data_runtime_attempts/op  4 data_runtime_hits/op  0 data_runtime_fallbacks/op  0 data_runtime_errors/op  2 data_runtime_pipeline_shapes  2 linalg_vector_attempts/op  2 linalg_vector_hits/op  0 linalg_vector_fallbacks/op  0 linalg_vector_errors/op  2 linalg_matrix_attempts/op  2 linalg_matrix_hits/op  0 linalg_matrix_fallbacks/op  0 linalg_matrix_errors/op
"""

SAMPLE_QEVAL_FAMILY_COVERAGE = """
BenchmarkQSessionEvalVectorWarmExecution/ListAdverbScan-16       100  2100 ns/op  128 B/op  4 allocs/op  100.0 typed_kernel_hit_pct  1 typed_kernel_attempts/op  1 typed_kernel_hits/op  0 typed_kernel_fallbacks/op  0 typed_kernel_errors/op  1 typed_pipeline_shapes  0 typed_pipeline_fallback_shapes  1 q_pipeline_category_ordinary_list_adverb
BenchmarkQEvalVectorGoBaseline/ListAdverbScan-16                 100  1900 ns/op  0 B/op  0 allocs/op
BenchmarkQEvalJITScriptWarm/ListAdverbScan-16                    100  1800 ns/op  96 B/op  3 allocs/op  1 q_session_planned_op_exit/op  0 q_session_shell_fallback/op  0 q_session_eval_errors/op  1 q_session_backend_shapes
BenchmarkQSessionEvalVectorWarmExecution/TypeMatrixShortNull-16  100  2200 ns/op  128 B/op  4 allocs/op  100.0 typed_kernel_hit_pct  1 typed_kernel_attempts/op  1 typed_kernel_hits/op  0 typed_kernel_fallbacks/op  0 typed_kernel_errors/op  1 typed_pipeline_shapes  0 typed_pipeline_fallback_shapes
BenchmarkQEvalVectorGoBaseline/TypeMatrixShortNull-16            100  2000 ns/op  0 B/op  0 allocs/op
BenchmarkQEvalJITScriptWarm/TypeMatrixShortNull-16               100  1700 ns/op  96 B/op  3 allocs/op  1 q_session_planned_op_exit/op  0 q_session_shell_fallback/op  0 q_session_eval_errors/op  1 q_session_backend_shapes
BenchmarkQSessionEvalVectorWarmExecution/ComboNestedAdverb-16    100  2300 ns/op  128 B/op  4 allocs/op  100.0 typed_kernel_hit_pct  1 typed_kernel_attempts/op  1 typed_kernel_hits/op  0 typed_kernel_fallbacks/op  0 typed_kernel_errors/op  1 typed_pipeline_shapes  0 typed_pipeline_fallback_shapes
BenchmarkQEvalVectorGoBaseline/ComboNestedAdverb-16              100  2100 ns/op  0 B/op  0 allocs/op
BenchmarkQEvalJITScriptWarm/ComboNestedAdverb-16                 100  1600 ns/op  96 B/op  3 allocs/op  1 q_session_planned_op_exit/op  0 q_session_shell_fallback/op  0 q_session_eval_errors/op  1 q_session_backend_shapes
"""

SAMPLE_JIT_SCRIPT = """
BenchmarkQEvalJITScriptWarm/MaskWhere-16    100  1500 ns/op  128 B/op  4 allocs/op  1 q_session_planned_op_exit/op  0 q_session_shell_fallback/op  0 q_session_eval_errors/op  1 q_session_backend_shapes
BenchmarkQEvalVMScriptWarm/MaskWhere-16     100  4000 ns/op  256 B/op  9 allocs/op
"""

SAMPLE_JIT_SESSION_SLOW_ROUTE = """
BenchmarkQEvalJITScriptWarm/MaskWhere-16    100  1800 ns/op  96 B/op  3 allocs/op  0.5 q_session_planned_op_exit/op  1 q_session_shell_fallback/op  1 q_session_eval_errors/op  1 q_session_backend_shapes
"""

SAMPLE_UNTRUSTED_GO_BASELINE = """
BenchmarkQSessionEvalVectorWarmExecution/TinyConst-16    100  50 ns/op  0 B/op  0 allocs/op
BenchmarkQEvalVectorGoBaseline/TinyConst-16              100  5 ns/op  0 B/op  0 allocs/op
"""

SAMPLE_MILESTONE_PRESSURE = """
BenchmarkQSessionEvalVectorWarmExecution/FallbackShape-16    100  9000 ns/op  2048 B/op  90 allocs/op  80.0 typed_kernel_hit_pct  1 typed_kernel_attempts/op  0.8 typed_kernel_hits/op  0.2 typed_kernel_fallbacks/op  0 typed_kernel_errors/op  2 typed_pipeline_shapes  1 typed_pipeline_fallback_shapes
BenchmarkQEvalVectorGoBaseline/FallbackShape-16              100  1000 ns/op  0 B/op  0 allocs/op
BenchmarkQSessionEvalVectorWarmExecution/HealthyA-16         100  2000 ns/op  256 B/op  8 allocs/op  100.0 typed_kernel_hit_pct  20 typed_kernel_attempts/op  20 typed_kernel_hits/op  0 typed_kernel_fallbacks/op  0 typed_kernel_errors/op  2 typed_pipeline_shapes  0 typed_pipeline_fallback_shapes
BenchmarkQEvalVectorGoBaseline/HealthyA-16                   100  1000 ns/op  0 B/op  0 allocs/op
BenchmarkQSessionEvalVectorWarmExecution/HealthyB-16         100  2000 ns/op  256 B/op  8 allocs/op  100.0 typed_kernel_hit_pct  20 typed_kernel_attempts/op  20 typed_kernel_hits/op  0 typed_kernel_fallbacks/op  0 typed_kernel_errors/op  2 typed_pipeline_shapes  0 typed_pipeline_fallback_shapes
BenchmarkQEvalVectorGoBaseline/HealthyB-16                   100  1000 ns/op  0 B/op  0 allocs/op
"""

MILESTONE_CAPS = {
    "max_leia_go_ratio": 10.0,
    "max_leia_jit_go_ratio": 12.0,
    "min_typed_hit_pct": 75.0,
    "max_typed_fallbacks_op": 1.0,
    "max_pipeline_fallback_shapes": 2.0,
    "max_allocs_op": 128.0,
    "min_runtime_jit_backend_benchmarks": 0,
    "min_runtime_array_bridge_benchmarks": 0,
    "min_runtime_backend_route_benchmarks": 0,
    "min_runtime_backend_route_hits_op": 0,
    "max_runtime_backend_route_errors_op": 0,
    "min_q_eval_family_cases": 0,
    "min_q_session_planned_op_exit_op": 0.9,
}


SAMPLE_REALDATA = """
BenchmarkQEvalRealDataWarm/GatherSum1M-16          100  9800 ns/op  2048 B/op  14 allocs/op
BenchmarkQEvalRealDataGoBaseline/GatherSum1M-16    100  10000 ns/op  0 B/op  0 allocs/op
BenchmarkQEvalRealDataWarm/WhereChainAnd4Mid-16        100  40000 ns/op  4096 B/op  59 allocs/op
BenchmarkQEvalRealDataGoBaseline/WhereChainAnd4Mid-16  100  2000 ns/op  0 B/op  0 allocs/op
BenchmarkQEvalRealDataWarm/TinyVector8Rows-16          100  1800 ns/op  512 B/op  19 allocs/op
BenchmarkQEvalRealDataGoBaseline/TinyVector8Rows-16    100  7 ns/op  0 B/op  0 allocs/op
"""


def make_ratio_baseline(**overrides):
    baseline = {
        "schema_version": 1,
        "captured": "2026-06-10",
        "max_untrusted_go_baselines": 64,
        "cases": {},
        "family_targets": {},
        "exceptions": {},
    }
    baseline.update(overrides)
    return baseline


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

    def test_build_ratios_includes_qsql_matrix_family(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_QSQL_MATRIX)
        ratios = {row.scenario: row.ratio for row in report.build_ratios(rows)}

        self.assertEqual(ratios["qSQL matrix JoinInnerAliasedKeyTopK warm vs Go"], 2.0)
        self.assertEqual(ratios["qSQL matrix JoinInnerAliasedKeyTopK cold vs warm"], 3.0)

    def test_qsql_matrix_rows_counted_in_qsql_coverage(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_QSQL_MATRIX)
        coverage = report.build_qsql_benchmark_coverage(rows)

        self.assertEqual(coverage.leia_case_count, 4)
        self.assertEqual(coverage.native_go_case_count, 2)

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
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_JIT_SCRIPT + SAMPLE_ARRAY_BRIDGE + SAMPLE_DATA_RUNTIME)
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
        qsession = metrics["BenchmarkQEvalJITScriptWarm/MaskWhere"]
        self.assertEqual(qsession.q_session_planned_op_exit_op, 1)
        self.assertEqual(qsession.q_session_shell_fallback_op, 0)
        self.assertEqual(qsession.q_session_eval_errors_op, 0)
        self.assertEqual(qsession.q_session_backend_shapes, 1)
        bridge = metrics["BenchmarkQEvalPipelineArrayRuntimeBridge/BulkI64Range"]
        self.assertEqual(bridge.q_array_bridge_bulk_hits_op, 1)
        self.assertEqual(bridge.q_array_bridge_fallbacks_op, 0)
        data_runtime = metrics["BenchmarkQSessionEvalVectorWarmExecution/LinalgFacade"]
        self.assertEqual(data_runtime.data_runtime_hit_pct, 100)
        self.assertEqual(data_runtime.data_runtime_attempts_op, 4)
        self.assertEqual(data_runtime.data_runtime_fallbacks_op, 0)
        self.assertEqual(data_runtime.data_runtime_pipeline_shapes, 2)
        self.assertEqual(data_runtime.linalg_vector_hits_op, 2)
        self.assertEqual(data_runtime.linalg_matrix_hits_op, 2)
        self.assertEqual(bridge.q_array_bridge_errors_op, 0)
        self.assertEqual(bridge.q_array_bridge_rows_op, 8192)

    def test_runtime_backend_route_summary_exposes_registry_and_frame_vector_routes(self):
        rows = report.parse_go_benchmarks(SAMPLE_BACKEND_ROUTE)
        summary = report.build_runtime_backend_route_summary(rows)

        self.assertEqual(len(summary), 1)
        routes = summary[0]
        self.assertEqual(routes.scope, "runtime_primitive_registry_and_frame_vector_routes")
        self.assertEqual(routes.benchmark_count, 2)
        self.assertEqual(routes.registry_benchmark_count, 1)
        self.assertEqual(routes.methodjit_frame_vector_benchmark_count, 1)
        self.assertEqual(routes.hits_op, 6)
        self.assertEqual(routes.errors_op, 1)
        self.assertAlmostEqual(routes.hit_pct, 100 * 6 / 7)

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

    def test_runtime_observability_summary_rolls_up_pipeline_primitive_and_jit(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_WITH_FALLBACK + SAMPLE_JIT_SLOW_ROUTE + SAMPLE_ARRAY_BRIDGE + SAMPLE_DATA_RUNTIME)
        summary = {row.layer: row for row in report.build_runtime_observability_summary(rows)}

        self.assertEqual(summary["qsql_kernel"].benchmark_count, 2)
        self.assertEqual(summary["qsql_kernel"].fallbacks_op, 0)
        self.assertEqual(summary["typed_primitive"].attempts_op, 2)
        self.assertEqual(summary["typed_primitive"].hits_op, 1.8)
        self.assertEqual(summary["typed_primitive"].fallbacks_op, 0.2)
        self.assertEqual(summary["typed_primitive"].hit_pct, 90)
        self.assertEqual(summary["unified_pipeline"].shapes, 4)
        self.assertEqual(summary["unified_pipeline"].fallback_shapes, 1)
        self.assertEqual(summary["unified_pipeline"].hit_pct, 75)
        self.assertEqual(summary["jit_backend"].direct_return_op, 2)
        self.assertEqual(summary["jit_backend"].native_exit_op, 2)
        self.assertEqual(summary["jit_backend"].op_exit_op, 1)
        self.assertEqual(summary["jit_backend"].errors_op, 1)
        self.assertEqual(summary["jit_backend"].slow_route_pct, 60)
        self.assertEqual(summary["methodjit_array_bridge"].attempts_op, 3)
        self.assertEqual(summary["methodjit_array_bridge"].hits_op, 2)
        self.assertEqual(summary["methodjit_array_bridge"].fallbacks_op, 1)
        self.assertAlmostEqual(summary["methodjit_array_bridge"].hit_pct, 100 * 2 / 3)
        self.assertEqual(summary["data_runtime"].attempts_op, 4)
        self.assertEqual(summary["data_runtime"].hits_op, 4)
        self.assertEqual(summary["data_runtime"].hit_pct, 100)
        self.assertEqual(summary["data_runtime"].shapes, 2)
        self.assertEqual(summary["linalg_vector"].hits_op, 2)
        self.assertEqual(summary["linalg_matrix"].hits_op, 2)

    def test_data_runtime_metrics_are_report_only_not_gate_signals(self):
        rows = report.parse_go_benchmarks(SAMPLE_DATA_RUNTIME)
        checks = report.runtime_gate_checks(
            rows,
            report.GatePolicy(
                max_leia_go_ratio=5,
                min_typed_hit_pct=95,
                max_typed_fallbacks_op=0,
                max_pipeline_fallback_shapes=0,
                max_allocs_op=64,
            ),
        )

        self.assertFalse(any(check.signal.startswith(("data_runtime", "linalg_")) for check in checks))

    def test_runtime_health_summary_combines_jit_fallback_and_alloc_pressure(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_WITH_FALLBACK + SAMPLE_JIT_SLOW_ROUTE)
        summary = report.build_runtime_health_summary(rows)

        self.assertEqual(len(summary), 1)
        health = summary[0]
        self.assertEqual(health.scope, "q_runtime_hotpath")
        self.assertEqual(health.benchmark_count, 4)
        self.assertEqual(health.avg_allocs_op, 108 / 4)
        self.assertEqual(health.max_allocs_op, 90)
        self.assertEqual(health.typed_fallbacks_op, 0.2)
        self.assertEqual(health.typed_errors_op, 1)
        self.assertEqual(health.pipeline_fallback_shapes, 1)
        self.assertEqual(health.jit_direct_return_op, 2)
        self.assertEqual(health.jit_native_exit_op, 2)
        self.assertEqual(health.jit_op_exit_op, 1)
        self.assertEqual(health.jit_slow_route_pct, 60)

    def test_runtime_bridge_efficiency_summary_quantifies_direct_backend_value(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_WITH_FALLBACK + SAMPLE_JIT_SLOW_ROUTE + SAMPLE_ARRAY_BRIDGE)
        summary = report.build_runtime_bridge_efficiency_summary(rows)

        self.assertEqual(len(summary), 1)
        efficiency = summary[0]
        self.assertEqual(efficiency.scope, "typed_runtime_and_jit_backend")
        self.assertEqual(efficiency.benchmark_count, 7)
        self.assertEqual(efficiency.direct_calls_op, 5.8)
        self.assertEqual(efficiency.slow_bridge_calls_op, 5.2)
        self.assertAlmostEqual(efficiency.direct_call_share_pct, 100 * 5.8 / 11)
        self.assertEqual(efficiency.avg_allocs_op, 119 / 7)
        self.assertAlmostEqual(efficiency.allocs_per_direct_call, (119 / 7) / 5.8)

    def test_runtime_array_bridge_summary_exposes_bulk_hit_and_fallback_counts(self):
        rows = report.parse_go_benchmarks(SAMPLE_ARRAY_BRIDGE)
        summary = report.build_runtime_array_bridge_summary(rows)

        self.assertEqual(len(summary), 1)
        bridge = summary[0]
        self.assertEqual(bridge.scope, "methodjit_array_bridge")
        self.assertEqual(bridge.benchmark_count, 3)
        self.assertEqual(bridge.attempts_op, 3)
        self.assertEqual(bridge.bulk_hits_op, 2)
        self.assertEqual(bridge.fallbacks_op, 1)
        self.assertEqual(bridge.errors_op, 0)
        self.assertAlmostEqual(bridge.bulk_hit_pct, 100 * 2 / 3)
        self.assertEqual(bridge.rows_op, 24576)
        self.assertEqual(bridge.avg_allocs_op, 11 / 3)
        self.assertEqual(bridge.max_allocs_op, 7)

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
        notes = {check.benchmark: check.note for check in checks if check.status == "fail"}
        self.assertIn("typed_fallback", notes["BenchmarkQSessionEvalVectorWarmExecution/FallbackShape"])
        self.assertIn("session/go=9.000x", notes["BenchmarkQSessionEvalVectorWarmExecution/FallbackShape"])

    def test_gate_checks_cover_jit_backend_errors_and_slow_routes(self):
        rows = report.parse_go_benchmarks(SAMPLE_JIT_SLOW_ROUTE)
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
            max_jit_typed_errors_op=0,
            max_jit_backend_slow_route_pct=50,
        )
        checks = report.build_gate_checks(rows, policy)
        failed = {(check.signal, check.benchmark) for check in checks if check.status == "fail"}

        self.assertIn(("jit_typed_errors_op", "BenchmarkQEvalPipelineNativeExitCallpath/SlowRoute"), failed)
        self.assertIn(("jit_backend_errors_op", "jit_backend"), failed)
        self.assertIn(("jit_backend_slow_route_pct", "jit_backend"), failed)
        self.assertIn(("runtime_health_typed_errors_op", "q_runtime_hotpath"), failed)
        self.assertIn(("runtime_health_jit_slow_route_pct", "q_runtime_hotpath"), failed)

    def test_gate_checks_cover_jit_session_shell_fallback_and_errors(self):
        rows = report.parse_go_benchmarks(SAMPLE_JIT_SESSION_SLOW_ROUTE)
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
            max_jit_typed_errors_op=0,
            max_jit_backend_slow_route_pct=50,
        )
        checks = report.build_gate_checks(rows, policy)
        failed = {(check.signal, check.benchmark) for check in checks if check.status == "fail"}

        self.assertIn(("q_session_shell_fallback_op", "BenchmarkQEvalJITScriptWarm/MaskWhere"), failed)
        self.assertIn(("q_session_eval_errors_op", "BenchmarkQEvalJITScriptWarm/MaskWhere"), failed)
        self.assertIn(("q_session_planned_op_exit_op", "BenchmarkQEvalJITScriptWarm/MaskWhere"), failed)
        self.assertIn(("jit_backend_errors_op", "jit_backend"), failed)
        self.assertIn(("jit_backend_slow_route_pct", "jit_backend"), failed)
        self.assertIn(("runtime_health_typed_errors_op", "q_runtime_hotpath"), failed)
        self.assertIn(("runtime_health_jit_slow_route_pct", "q_runtime_hotpath"), failed)

    def test_gate_checks_cover_runtime_bridge_efficiency(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_WITH_FALLBACK + SAMPLE_JIT_SLOW_ROUTE)
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
            min_runtime_direct_bridge_share_pct=90,
            max_runtime_allocs_per_direct_call=4,
        )
        checks = report.build_gate_checks(rows, policy)
        failed = {(check.signal, check.benchmark) for check in checks if check.status == "fail"}

        self.assertIn(("runtime_bridge_direct_call_share_pct", "typed_runtime_and_jit_backend"), failed)
        self.assertIn(("runtime_bridge_allocs_per_direct_call", "typed_runtime_and_jit_backend"), failed)

    def test_gate_checks_cover_runtime_array_bridge_direct_metrics(self):
        rows = report.parse_go_benchmarks(SAMPLE_ARRAY_BRIDGE)
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=10000,
            min_q_array_bridge_bulk_hit_pct=90,
            max_q_array_bridge_fallbacks_op=0,
            min_runtime_typed_primitive_benchmarks=0,
            min_runtime_jit_backend_benchmarks=0,
            min_runtime_array_bridge_benchmarks=1,
            min_runtime_bridge_benchmark_count=1,
            max_q_array_bridge_avg_allocs_op=3,
            max_q_array_bridge_max_allocs_op=4,
        )
        checks = report.build_gate_checks(rows, policy)
        failed = {(check.signal, check.benchmark) for check in checks if check.status == "fail"}

        self.assertIn(("q_array_bridge_bulk_hit_pct", "methodjit_array_bridge"), failed)
        self.assertIn(("q_array_bridge_fallbacks_op", "methodjit_array_bridge"), failed)
        self.assertIn(("q_array_bridge_avg_allocs_op", "methodjit_array_bridge"), failed)
        self.assertIn(("q_array_bridge_max_allocs_op", "methodjit_array_bridge"), failed)

    def test_gate_checks_fail_when_required_runtime_metric_layers_are_missing(self):
        rows = report.parse_go_benchmarks(SAMPLE)
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
            min_runtime_typed_primitive_benchmarks=1,
            min_runtime_jit_backend_benchmarks=1,
            min_runtime_array_bridge_benchmarks=1,
            min_runtime_bridge_benchmark_count=3,
        )
        checks = report.build_gate_checks(rows, policy)
        failed = {(check.signal, check.benchmark) for check in checks if check.status == "fail"}

        self.assertNotIn(("runtime_contract_typed_primitive_benchmarks", "typed_primitive"), failed)
        self.assertNotIn(("runtime_contract_jit_backend_benchmarks", "jit_backend"), failed)
        self.assertIn(("runtime_contract_array_bridge_benchmarks", "methodjit_array_bridge"), failed)
        self.assertIn(("runtime_contract_bridge_benchmark_count", "typed_runtime_and_jit_backend"), failed)
        self.assertIn(("runtime_backend_route_benchmarks", "runtime_primitive_registry_and_frame_vector_routes"), failed)
        self.assertIn(("runtime_backend_route_hits_op", "runtime_primitive_registry_and_frame_vector_routes"), failed)

    def test_gate_checks_cover_runtime_backend_route_contract(self):
        rows = report.parse_go_benchmarks(SAMPLE_BACKEND_ROUTE)
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
            min_runtime_typed_primitive_benchmarks=0,
            min_runtime_jit_backend_benchmarks=0,
            min_runtime_array_bridge_benchmarks=0,
            min_runtime_bridge_benchmark_count=0,
            min_runtime_backend_route_benchmarks=2,
            min_runtime_backend_route_hits_op=6,
            max_runtime_backend_route_errors_op=0,
        )
        checks = report.build_gate_checks(rows, policy)
        failed = {(check.signal, check.benchmark) for check in checks if check.status == "fail"}

        self.assertIn(("runtime_backend_route_errors_op", "runtime_primitive_registry_and_frame_vector_routes"), failed)
        self.assertNotIn(("runtime_backend_route_benchmarks", "runtime_primitive_registry_and_frame_vector_routes"), failed)
        self.assertNotIn(("runtime_backend_route_hits_op", "runtime_primitive_registry_and_frame_vector_routes"), failed)

    def test_gate_checks_pass_runtime_contract_when_all_backend_layers_are_present(self):
        rows = report.parse_go_benchmarks(SAMPLE_BRIDGE_HEALTHY + SAMPLE_ARRAY_BRIDGE_HEALTHY)
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=99,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=8,
            max_jit_typed_errors_op=0,
            max_jit_backend_slow_route_pct=0,
            min_runtime_direct_bridge_share_pct=100,
            max_runtime_allocs_per_direct_call=1,
            min_q_array_bridge_bulk_hit_pct=100,
            max_q_array_bridge_fallbacks_op=0,
            min_runtime_typed_primitive_benchmarks=1,
            min_runtime_jit_backend_benchmarks=1,
            min_runtime_array_bridge_benchmarks=2,
            min_runtime_bridge_benchmark_count=4,
            min_q_array_bridge_rows_op=1,
            max_q_array_bridge_avg_allocs_op=2,
            max_q_array_bridge_max_allocs_op=2,
            min_q_eval_family_cases=0,
        )
        checks = report.build_gate_checks(rows, policy)

        self.assertFalse(report.gate_failed(checks))
        by_signal = {check.signal: check for check in checks}
        self.assertEqual(by_signal["runtime_contract_typed_primitive_benchmarks"].value, 1)
        self.assertEqual(by_signal["runtime_contract_jit_backend_benchmarks"].value, 1)
        self.assertEqual(by_signal["runtime_contract_array_bridge_benchmarks"].value, 2)
        self.assertEqual(by_signal["q_array_bridge_rows_op"].value, 16384)
        self.assertEqual(by_signal["runtime_backend_route_benchmarks"].value, 2)
        self.assertEqual(by_signal["runtime_backend_route_hits_op"].value, 6)

    def test_runtime_bridge_efficiency_sample_passes_strict_gate(self):
        rows = report.parse_go_benchmarks(SAMPLE_BRIDGE_HEALTHY + SAMPLE_ARRAY_BRIDGE_HEALTHY)
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=99,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=8,
            max_jit_typed_errors_op=0,
            max_jit_backend_slow_route_pct=0,
            min_runtime_direct_bridge_share_pct=100,
            max_runtime_allocs_per_direct_call=1,
            min_q_array_bridge_bulk_hit_pct=100,
            max_q_array_bridge_fallbacks_op=0,
            min_runtime_array_bridge_benchmarks=2,
            min_runtime_bridge_benchmark_count=4,
            max_q_array_bridge_avg_allocs_op=2,
            max_q_array_bridge_max_allocs_op=2,
            min_q_eval_family_cases=0,
        )
        checks = report.build_gate_checks(rows, policy)

        self.assertTrue(checks)
        self.assertFalse(report.gate_failed(checks))
        by_signal = {check.signal: check for check in checks}
        self.assertEqual(by_signal["runtime_bridge_direct_call_share_pct"].value, 100)
        self.assertAlmostEqual(by_signal["runtime_bridge_allocs_per_direct_call"].value, (12 / 4) / 6)

    def test_gate_checks_cover_runtime_health_fallback_and_alloc_pressure(self):
        rows = report.parse_go_benchmarks(SAMPLE_WITH_FALLBACK)
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
        )
        checks = report.build_gate_checks(rows, policy)
        failed = {(check.signal, check.benchmark) for check in checks if check.status == "fail"}

        self.assertIn(("runtime_health_typed_fallbacks_op", "q_runtime_hotpath"), failed)
        self.assertIn(("runtime_health_pipeline_fallback_shapes", "q_runtime_hotpath"), failed)
        self.assertIn(("runtime_health_max_allocs_op", "q_runtime_hotpath"), failed)

    def test_build_ratios_includes_jit_and_vm_script_families(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_JIT_SCRIPT)
        ratios = {row.scenario: row for row in report.build_ratios(rows)}

        jit = ratios["q.eval MaskWhere JIT script warm vs Go"]
        self.assertEqual(jit.ratio, 1.5)
        self.assertEqual(jit.numerator, "BenchmarkQEvalJITScriptWarm/MaskWhere")
        self.assertEqual(jit.denominator, "BenchmarkQEvalVectorGoBaseline/MaskWhere")
        vm = ratios["q.eval MaskWhere VM script warm vs Go"]
        self.assertEqual(vm.ratio, 4.0)
        self.assertIn("attribution only", vm.note)

    def test_build_ratios_tolerates_absent_script_warm_families(self):
        rows = report.parse_go_benchmarks(SAMPLE)
        scenarios = [row.scenario for row in report.build_ratios(rows)]

        self.assertFalse(any("script warm vs Go" in scenario for scenario in scenarios))

    def test_qeval_case_diagnostics_join_warm_cold_go_jit_route_and_allocs(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_JIT_SCRIPT)
        diagnostics = {row.case: row for row in report.build_qeval_case_diagnostics(rows)}

        mask = diagnostics["MaskWhere"]
        self.assertEqual(mask.go_baseline_ns_op, 1000)
        self.assertEqual(mask.session_ns_op, 2000)
        self.assertEqual(mask.session_go_ratio, 2.0)
        self.assertEqual(mask.result_cache_warm_session_ratio, 0.25)
        self.assertEqual(mask.cold_session_ratio, 1.25)
        self.assertEqual(mask.jit_warm_ns_op, 1500)
        self.assertEqual(mask.jit_go_ratio, 1.5)
        self.assertEqual(mask.vm_go_ratio, 4.0)
        self.assertEqual(mask.typed_hit_pct, 100)
        self.assertEqual(mask.typed_fallbacks_op, 0)
        self.assertEqual(mask.session_allocs_op, 8)
        self.assertEqual(mask.jit_warm_allocs_op, 4)
        self.assertEqual(mask.q_session_planned_op_exit_op, 1)
        self.assertEqual(mask.q_session_shell_fallback_op, 0)
        self.assertEqual(mask.q_session_eval_errors_op, 0)
        self.assertEqual(mask.q_session_backend_shapes, 1)
        self.assertEqual(mask.q_session_planned_route_pct, 100)
        self.assertEqual(mask.primary_pressure, "healthy_or_ratio_only")
        self.assertIn("session/go=2.000x", mask.note)
        self.assertIn("session_planned/op=1.000", mask.note)
        self.assertIn("session_planned_route=100.0%", mask.note)

    def test_qeval_case_diagnostics_classify_jit_slow_route(self):
        sample = SAMPLE + """
BenchmarkQEvalJITScriptWarm/MaskWhere-16  100  1800 ns/op  96 B/op  3 allocs/op  1 jit_typed_direct_return/op  2 jit_typed_native_exit/op  1 jit_typed_op_exit/op  0 jit_typed_kernel_errors/op
"""
        rows = report.parse_go_benchmarks(sample)
        diagnostics = {row.case: row for row in report.build_qeval_case_diagnostics(rows)}

        self.assertEqual(diagnostics["MaskWhere"].primary_pressure, "jit_slow_route")
        self.assertEqual(diagnostics["MaskWhere"].jit_backend_slow_route_pct, 75)

    def test_qeval_case_diagnostics_classify_session_shell_and_error_routes(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_JIT_SESSION_SLOW_ROUTE)
        diagnostics = {row.case: row for row in report.build_qeval_case_diagnostics(rows)}

        self.assertEqual(diagnostics["MaskWhere"].primary_pressure, "jit_backend_errors")
        self.assertEqual(diagnostics["MaskWhere"].jit_backend_slow_route_pct, 80)
        self.assertEqual(diagnostics["MaskWhere"].q_session_planned_route_pct, 20)
        self.assertIn("session_shell/op=1.000", diagnostics["MaskWhere"].note)
        self.assertIn("session_errors/op=1.000", diagnostics["MaskWhere"].note)

    def test_gate_checks_gate_jit_script_family_but_never_vm(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_JIT_SCRIPT)
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
            max_leia_jit_go_ratio=1.2,
        )
        checks = report.build_gate_checks(rows, policy)
        failed = {(check.signal, check.benchmark) for check in checks if check.status == "fail"}

        self.assertIn(("leia_jit_go_ratio", "BenchmarkQEvalJITScriptWarm/MaskWhere"), failed)
        ratio_signals = {"leia_go_ratio", "leia_jit_go_ratio", "leia_go_ratio_regression"}
        self.assertFalse(
            any(
                check.benchmark.startswith("BenchmarkQEvalVMScriptWarm/")
                for check in checks
                if check.signal in ratio_signals
            )
        )

    def test_ratio_baseline_no_regression_pass_and_fail(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_JIT_SCRIPT)
        baseline = make_ratio_baseline(cases={"MaskWhere": {"warm_go_ratio": 2.0, "jit_go_ratio": 1.0}})
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
        )
        checks = report.build_gate_checks(rows, policy, baseline)
        regression = {check.benchmark: check for check in checks if check.signal == "leia_go_ratio_regression"}

        warm = regression["BenchmarkQSessionEvalVectorWarmExecution/MaskWhere"]
        self.assertEqual(warm.status, "pass")  # 2.0 <= 2.0 * 1.15
        jit = regression["BenchmarkQEvalJITScriptWarm/MaskWhere"]
        self.assertEqual(jit.status, "fail")  # 1.5 > 1.0 * 1.15
        self.assertEqual(jit.value, 1.5)

    def test_ratio_baseline_missing_case_passes_with_capture_note(self):
        rows = report.parse_go_benchmarks(SAMPLE)
        baseline = make_ratio_baseline()
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
        )
        checks = report.build_gate_checks(rows, policy, baseline)
        regression = {check.benchmark: check for check in checks if check.signal == "leia_go_ratio_regression"}

        warm = regression["BenchmarkQSessionEvalVectorWarmExecution/MaskWhere"]
        self.assertEqual(warm.status, "pass")
        self.assertIn("--update-ratio-baseline", warm.note)

    def test_ratio_baseline_exceptions_exempt_hard_cap_but_not_regression(self):
        rows = report.parse_go_benchmarks(SAMPLE_WITH_FALLBACK)
        baseline = make_ratio_baseline(
            cases={"FallbackShape": {"warm_go_ratio": 5.0, "jit_go_ratio": None}},
            exceptions={"FallbackShape": {"reason": "known kernel gap", "ref": "ISSUE-1"}},
        )
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
        )
        checks = report.build_gate_checks(rows, policy, baseline)

        hard_cap = [
            check
            for check in checks
            if check.signal == "leia_go_ratio"
            and check.benchmark == "BenchmarkQSessionEvalVectorWarmExecution/FallbackShape"
        ]
        self.assertEqual(len(hard_cap), 1)
        self.assertEqual(hard_cap[0].status, "skip")
        self.assertIn("known kernel gap", hard_cap[0].note)
        regression = [
            check
            for check in checks
            if check.signal == "leia_go_ratio_regression"
            and check.benchmark == "BenchmarkQSessionEvalVectorWarmExecution/FallbackShape"
        ]
        self.assertEqual(len(regression), 1)
        self.assertEqual(regression[0].status, "fail")  # 9.0 > 5.0 * 1.15
        counts = [check for check in checks if check.signal == "ratio_baseline_exception_count"]
        self.assertEqual(len(counts), 1)
        self.assertEqual(counts[0].value, 1)
        self.assertEqual(counts[0].status, "pass")

    def test_ratio_baseline_stale_exception_fails(self):
        rows = report.parse_go_benchmarks(SAMPLE)
        baseline = make_ratio_baseline(exceptions={"RemovedCase": {"reason": "gone", "ref": ""}})
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
        )
        checks = report.build_gate_checks(rows, policy, baseline)
        failed = {(check.signal, check.benchmark) for check in checks if check.status == "fail"}

        self.assertIn(("ratio_baseline_stale_exception", "RemovedCase"), failed)

    def test_untrusted_go_baseline_count_gate(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_UNTRUSTED_GO_BASELINE)
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
        )

        strict = report.build_gate_checks(rows, policy, make_ratio_baseline(max_untrusted_go_baselines=0))
        by_signal = {check.signal: check for check in strict if check.signal == "untrusted_go_baseline_count"}
        self.assertEqual(by_signal["untrusted_go_baseline_count"].status, "fail")
        self.assertEqual(by_signal["untrusted_go_baseline_count"].value, 1)

        relaxed = report.build_gate_checks(rows, policy, make_ratio_baseline(max_untrusted_go_baselines=1))
        by_signal = {check.signal: check for check in relaxed if check.signal == "untrusted_go_baseline_count"}
        self.assertEqual(by_signal["untrusted_go_baseline_count"].status, "pass")

    def test_family_geomean_gating_skip_and_fail(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_JIT_SCRIPT)
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
        )

        empty = report.build_gate_checks(rows, policy, make_ratio_baseline(family_targets={}))
        empty_checks = [check for check in empty if check.signal == "family_geomean_jit_go_ratio"]
        self.assertEqual(len(empty_checks), 1)
        self.assertEqual(empty_checks[0].status, "skip")
        self.assertIn("later phase", empty_checks[0].note)

        failing = report.build_gate_checks(
            rows,
            policy,
            make_ratio_baseline(
                family_targets={"core": {"cases": ["MaskWhere"], "max_geomean_jit_go_ratio": 1.2}}
            ),
        )
        failing_checks = {check.benchmark: check for check in failing if check.signal == "family_geomean_jit_go_ratio"}
        self.assertEqual(failing_checks["core"].status, "fail")
        self.assertAlmostEqual(failing_checks["core"].value, 1.5)

        sparse = report.build_gate_checks(
            rows,
            policy,
            make_ratio_baseline(
                family_targets={
                    "sparse": {
                        "cases": ["MaskWhere", "MissingA", "MissingB"],
                        "max_geomean_jit_go_ratio": 1.2,
                    }
                }
            ),
        )
        sparse_checks = {check.benchmark: check for check in sparse if check.signal == "family_geomean_jit_go_ratio"}
        self.assertEqual(sparse_checks["sparse"].status, "skip")
        self.assertIn("1/3", sparse_checks["sparse"].note)

    def test_ratio_baseline_gate_skips_when_baseline_file_missing(self):
        rows = report.parse_go_benchmarks(SAMPLE)
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
        )
        checks = report.build_gate_checks(rows, policy, None)
        baseline_checks = [check for check in checks if check.signal == "ratio_baseline"]

        self.assertEqual(len(baseline_checks), 1)
        self.assertEqual(baseline_checks[0].status, "skip")
        self.assertFalse(any(check.signal == "leia_go_ratio_regression" for check in checks))

    def test_update_ratio_baseline_round_trip(self):
        with tempfile.TemporaryDirectory() as td:
            td_path = Path(td)
            out = td_path / "bench.txt"
            out.write_text(SAMPLE + SAMPLE_JIT_SCRIPT + SAMPLE_UNTRUSTED_GO_BASELINE)
            baseline_path = td_path / "qeval_go_ratio_baseline.json"
            baseline_path.write_text(
                json.dumps(
                    make_ratio_baseline(
                        exceptions={"MaskWhere": {"reason": "kept", "ref": "ISSUE-2"}},
                        family_targets={"core": {"cases": ["MaskWhere"], "max_geomean_jit_go_ratio": 2.0}},
                    )
                )
            )
            argv = [
                "--from-output", str(out),
                "--json", str(td_path / "report.json"),
                "--markdown", str(td_path / "report.md"),
                "--ratio-baseline", str(baseline_path),
                "--update-ratio-baseline",
            ]

            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                code = report.main(argv)

            self.assertEqual(code, 0)
            first_text = baseline_path.read_text()
            payload = json.loads(first_text)
            self.assertEqual(payload["schema_version"], 1)
            self.assertEqual(payload["max_untrusted_go_baselines"], 1)
            self.assertEqual(payload["cases"]["MaskWhere"], {"warm_go_ratio": 2.0, "jit_go_ratio": 1.5})
            self.assertEqual(payload["cases"]["TinyConst"], {"warm_go_ratio": None, "jit_go_ratio": None})
            self.assertEqual(payload["exceptions"], {"MaskWhere": {"reason": "kept", "ref": "ISSUE-2"}})
            self.assertEqual(payload["family_targets"], {"core": {"cases": ["MaskWhere"], "max_geomean_jit_go_ratio": 2.0}})

            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                code = report.main(argv)

            self.assertEqual(code, 0)
            self.assertEqual(baseline_path.read_text(), first_text)

    def test_realdata_rows_form_separate_family_with_geomean(self):
        rows = report.parse_go_benchmarks(SAMPLE + SAMPLE_REALDATA)
        items = report.build_qeval_realdata_rows(rows)
        by_case = {item.case: item for item in items}
        self.assertEqual(set(by_case), {"GatherSum1M", "WhereChainAnd4Mid", "TinyVector8Rows"})
        self.assertAlmostEqual(by_case["GatherSum1M"].realdata_go_ratio, 0.98)
        self.assertAlmostEqual(by_case["WhereChainAnd4Mid"].realdata_go_ratio, 20.0)
        # 7 ns/op Go baseline is below the trust threshold: correctness-only.
        self.assertIsNone(by_case["TinyVector8Rows"].realdata_go_ratio)
        self.assertFalse(by_case["TinyVector8Rows"].trusted_go_baseline)
        geo, trusted, total = report.realdata_trusted_geomean(rows)
        self.assertEqual((trusted, total), (2, 3))
        self.assertAlmostEqual(geo, (0.98 * 20.0) ** 0.5, places=6)
        # The realdata family never leaks into the synthetic baseline cases.
        self.assertNotIn("GatherSum1M", report.collect_ratio_baseline_cases(rows))

    def test_realdata_gate_checks_hard_cap_and_ratchet(self):
        rows = report.parse_go_benchmarks(SAMPLE_REALDATA)
        baseline = make_ratio_baseline(
            realdata_cases={
                "GatherSum1M": {"realdata_go_ratio": 1.0},
                "WhereChainAnd4Mid": {"realdata_go_ratio": 10.0},
            }
        )
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
            max_leia_realdata_go_ratio=60.0,
        )
        checks = report.realdata_gate_checks(rows, policy, baseline)
        regression = {check.benchmark: check for check in checks if check.signal == "realdata_go_ratio_regression"}
        hard_cap = {check.benchmark: check for check in checks if check.signal == "leia_realdata_go_ratio"}

        gather = regression["BenchmarkQEvalRealDataWarm/GatherSum1M"]
        self.assertEqual(gather.status, "pass")  # 0.98 <= 1.0 * 1.15
        chain = regression["BenchmarkQEvalRealDataWarm/WhereChainAnd4Mid"]
        self.assertEqual(chain.status, "fail")  # 20.0 > 10.0 * 1.15
        self.assertEqual(chain.value, 20.0)
        # Untrusted Go baseline never produces a ratchet row, only a skip.
        self.assertNotIn("BenchmarkQEvalRealDataWarm/TinyVector8Rows", regression)
        tiny = hard_cap["BenchmarkQEvalRealDataWarm/TinyVector8Rows"]
        self.assertEqual(tiny.status, "skip")
        self.assertEqual(hard_cap["BenchmarkQEvalRealDataWarm/WhereChainAnd4Mid"].status, "pass")

    def test_realdata_gate_checks_hard_cap_fail_and_missing_baseline_entry(self):
        rows = report.parse_go_benchmarks(SAMPLE_REALDATA)
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
            max_leia_realdata_go_ratio=10.0,
        )
        checks = report.realdata_gate_checks(rows, policy, make_ratio_baseline())
        hard_cap = {check.benchmark: check for check in checks if check.signal == "leia_realdata_go_ratio"}
        self.assertEqual(hard_cap["BenchmarkQEvalRealDataWarm/WhereChainAnd4Mid"].status, "fail")
        self.assertEqual(hard_cap["BenchmarkQEvalRealDataWarm/GatherSum1M"].status, "pass")
        regression = {check.benchmark: check for check in checks if check.signal == "realdata_go_ratio_regression"}
        gather = regression["BenchmarkQEvalRealDataWarm/GatherSum1M"]
        self.assertEqual(gather.status, "pass")
        self.assertIn("--update-ratio-baseline", gather.note)

    def test_update_ratio_baseline_captures_and_preserves_realdata_cases(self):
        with tempfile.TemporaryDirectory() as td:
            td_path = Path(td)
            out = td_path / "bench.txt"
            out.write_text(SAMPLE + SAMPLE_REALDATA)
            baseline_path = td_path / "qeval_go_ratio_baseline.json"
            baseline_path.write_text(json.dumps(make_ratio_baseline()))
            argv = [
                "--from-output", str(out),
                "--json", str(td_path / "report.json"),
                "--markdown", str(td_path / "report.md"),
                "--ratio-baseline", str(baseline_path),
                "--update-ratio-baseline",
            ]

            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                code = report.main(argv)

            self.assertEqual(code, 0)
            payload = json.loads(baseline_path.read_text())
            self.assertEqual(
                payload["realdata_cases"]["GatherSum1M"], {"realdata_go_ratio": 0.98}
            )
            self.assertEqual(
                payload["realdata_cases"]["WhereChainAnd4Mid"], {"realdata_go_ratio": 20.0}
            )
            self.assertEqual(
                payload["realdata_cases"]["TinyVector8Rows"], {"realdata_go_ratio": None}
            )
            # realdata cases never bleed into the synthetic per-case map
            self.assertNotIn("GatherSum1M", payload["cases"])

            # an update from synthetic-only output must preserve the captured
            # realdata ratchet untouched
            out.write_text(SAMPLE)
            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                code = report.main(argv)
            self.assertEqual(code, 0)
            payload = json.loads(baseline_path.read_text())
            self.assertEqual(
                payload["realdata_cases"]["GatherSum1M"], {"realdata_go_ratio": 0.98}
            )

    def test_milestone_cap_overrides_realdata_hard_cap(self):
        with tempfile.TemporaryDirectory() as td:
            td_path = Path(td)
            out = td_path / "bench.txt"
            out.write_text(SAMPLE_REALDATA)
            baseline_path = td_path / "baseline.json"
            baseline_path.write_text(
                json.dumps(make_ratio_baseline(milestone_caps={"max_leia_realdata_go_ratio": 15.0}))
            )
            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                code = report.main([
                    "--from-output", str(out),
                    "--json", str(td_path / "report.json"),
                    "--markdown", str(td_path / "report.md"),
                    "--ratio-baseline", str(baseline_path),
                    "--check",
                ])
            self.assertEqual(code, 2)
            payload = json.loads((td_path / "report.json").read_text())
            self.assertEqual(payload["gate_policy"]["max_leia_realdata_go_ratio"], 15.0)
            fails = [
                row["benchmark"]
                for row in payload["gate"]
                if row["signal"] == "leia_realdata_go_ratio" and row["status"] == "fail"
            ]
            self.assertEqual(fails, ["BenchmarkQEvalRealDataWarm/WhereChainAnd4Mid"])
            realdata_section = payload["q_eval_realdata"]
            self.assertEqual(len(realdata_section), 3)

    def test_update_ratio_baseline_requires_qeval_bench_input(self):
        with tempfile.TemporaryDirectory() as td:
            td_path = Path(td)
            out = td_path / "bench.txt"
            out.write_text("BenchmarkQSQLNativeGoSelectWhereProject-16  100  2000 ns/op  100 B/op  1 allocs/op\n")
            baseline_path = td_path / "qeval_go_ratio_baseline.json"

            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                code = report.main([
                    "--from-output", str(out),
                    "--json", str(td_path / "report.json"),
                    "--markdown", str(td_path / "report.md"),
                    "--ratio-baseline", str(baseline_path),
                    "--update-ratio-baseline",
                ])

            self.assertEqual(code, 1)
            self.assertFalse(baseline_path.exists())

    def test_main_check_uses_ratio_baseline_file(self):
        with tempfile.TemporaryDirectory() as td:
            td_path = Path(td)
            out = td_path / "bench.txt"
            out.write_text(SAMPLE + SAMPLE_JIT_SCRIPT)
            baseline_path = td_path / "qeval_go_ratio_baseline.json"
            baseline_path.write_text(
                json.dumps(make_ratio_baseline(cases={"MaskWhere": {"warm_go_ratio": 2.0, "jit_go_ratio": 1.0}}))
            )
            json_path = td_path / "report.json"

            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                code = report.main([
                    "--from-output", str(out),
                    "--json", str(json_path),
                    "--markdown", str(td_path / "report.md"),
                    "--ratio-baseline", str(baseline_path),
                    "--check",
                ])

            self.assertEqual(code, 2)
            payload = json.loads(json_path.read_text())
            regression_rows = [
                row
                for row in payload["gate"]
                if row["signal"] == "leia_go_ratio_regression"
                and row["benchmark"] == "BenchmarkQEvalJITScriptWarm/MaskWhere"
            ]
            self.assertEqual(len(regression_rows), 1)
            self.assertEqual(regression_rows[0]["status"], "fail")

    def run_milestone_check(self, td_path, baseline, extra_args=()):
        out = td_path / "bench.txt"
        out.write_text(SAMPLE_MILESTONE_PRESSURE)
        baseline_path = td_path / "qeval_go_ratio_baseline.json"
        baseline_path.write_text(json.dumps(baseline))
        json_path = td_path / "report.json"

        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            code = report.main([
                "--from-output", str(out),
                "--json", str(json_path),
                "--markdown", str(td_path / "report.md"),
                "--ratio-baseline", str(baseline_path),
                "--check",
                *extra_args,
            ])
        return code, json.loads(json_path.read_text())

    def test_milestone_caps_relax_default_thresholds_when_flags_omitted(self):
        with tempfile.TemporaryDirectory() as td:
            code, payload = self.run_milestone_check(
                Path(td), make_ratio_baseline(milestone_caps=dict(MILESTONE_CAPS))
            )

            self.assertEqual(code, 0)
            self.assertEqual(payload["gate_policy"]["max_leia_go_ratio"], 10.0)
            self.assertEqual(payload["gate_policy"]["max_leia_jit_go_ratio"], 12.0)
            self.assertEqual(payload["gate_policy"]["min_typed_hit_pct"], 75.0)
            self.assertEqual(payload["gate_policy"]["max_typed_fallbacks_op"], 1.0)
            self.assertEqual(payload["gate_policy"]["max_pipeline_fallback_shapes"], 2.0)
            self.assertEqual(payload["gate_policy"]["max_allocs_op"], 128.0)
            self.assertEqual(payload["gate_policy"]["min_runtime_jit_backend_benchmarks"], 0)
            self.assertEqual(payload["gate_policy"]["min_runtime_array_bridge_benchmarks"], 0)
            self.assertEqual(payload["gate_policy"]["min_runtime_backend_route_benchmarks"], 0)
            self.assertEqual(payload["gate_policy"]["min_q_session_planned_op_exit_op"], 0.9)
            self.assertFalse(any(row["status"] == "fail" for row in payload["gate"]))

    def test_explicit_cli_flag_wins_over_milestone_caps(self):
        with tempfile.TemporaryDirectory() as td:
            # --max-allocs-op 64 equals the argparse default, but an explicit
            # flag must still beat milestone_caps.max_allocs_op = 128.
            code, payload = self.run_milestone_check(
                Path(td),
                make_ratio_baseline(milestone_caps=dict(MILESTONE_CAPS)),
                extra_args=("--max-allocs-op", "64"),
            )

            self.assertEqual(code, 2)
            self.assertEqual(payload["gate_policy"]["max_allocs_op"], 64.0)
            # other milestone caps still apply
            self.assertEqual(payload["gate_policy"]["min_typed_hit_pct"], 75.0)
            alloc_fails = [
                row
                for row in payload["gate"]
                if row["signal"] == "allocs_op" and row["status"] == "fail"
            ]
            self.assertEqual(
                [row["benchmark"] for row in alloc_fails],
                ["BenchmarkQSessionEvalVectorWarmExecution/FallbackShape"],
            )

    def test_missing_milestone_caps_keeps_default_thresholds(self):
        with tempfile.TemporaryDirectory() as td:
            code, payload = self.run_milestone_check(Path(td), make_ratio_baseline())

            self.assertEqual(code, 2)
            self.assertEqual(payload["gate_policy"]["max_leia_go_ratio"], 5.0)
            self.assertEqual(payload["gate_policy"]["max_allocs_op"], 64.0)
            self.assertEqual(payload["gate_policy"]["min_typed_hit_pct"], 95.0)
            failed_signals = {row["signal"] for row in payload["gate"] if row["status"] == "fail"}
            self.assertIn("leia_go_ratio", failed_signals)
            self.assertIn("allocs_op", failed_signals)
            self.assertIn("typed_hit_pct", failed_signals)

    def test_update_ratio_baseline_preserves_milestone_caps(self):
        with tempfile.TemporaryDirectory() as td:
            td_path = Path(td)
            out = td_path / "bench.txt"
            out.write_text(SAMPLE + SAMPLE_JIT_SCRIPT)
            baseline_path = td_path / "qeval_go_ratio_baseline.json"
            baseline_path.write_text(
                json.dumps(make_ratio_baseline(milestone_caps=dict(MILESTONE_CAPS)))
            )

            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                code = report.main([
                    "--from-output", str(out),
                    "--json", str(td_path / "report.json"),
                    "--markdown", str(td_path / "report.md"),
                    "--ratio-baseline", str(baseline_path),
                    "--update-ratio-baseline",
                ])

            self.assertEqual(code, 0)
            payload = json.loads(baseline_path.read_text())
            self.assertEqual(payload["milestone_caps"], MILESTONE_CAPS)
            self.assertEqual(payload["cases"]["MaskWhere"], {"warm_go_ratio": 2.0, "jit_go_ratio": 1.5})

    def test_update_ratio_baseline_writes_empty_milestone_caps_when_absent(self):
        with tempfile.TemporaryDirectory() as td:
            td_path = Path(td)
            out = td_path / "bench.txt"
            out.write_text(SAMPLE)
            baseline_path = td_path / "qeval_go_ratio_baseline.json"

            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                code = report.main([
                    "--from-output", str(out),
                    "--json", str(td_path / "report.json"),
                    "--markdown", str(td_path / "report.md"),
                    "--ratio-baseline", str(baseline_path),
                    "--update-ratio-baseline",
                ])

            self.assertEqual(code, 0)
            payload = json.loads(baseline_path.read_text())
            self.assertEqual(payload["milestone_caps"], {})

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

    def test_qeval_family_coverage_requires_go_and_jit_rows_for_breadth_families(self):
        rows = report.parse_go_benchmarks(SAMPLE_QEVAL_FAMILY_COVERAGE)
        coverage = {row.family: row for row in report.build_qeval_family_coverage(rows)}

        self.assertEqual(coverage["ordinary_list_adverb"].session_case_count, 1)
        self.assertEqual(coverage["ordinary_list_adverb"].matched_go_baseline_count, 1)
        self.assertEqual(coverage["ordinary_list_adverb"].matched_jit_case_count, 1)
        self.assertEqual(coverage["type_matrix"].matched_jit_case_count, 1)
        self.assertEqual(coverage["complex_combo"].matched_go_baseline_count, 1)

        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
            min_runtime_typed_primitive_benchmarks=0,
            min_runtime_jit_backend_benchmarks=0,
            min_runtime_array_bridge_benchmarks=0,
            min_runtime_bridge_benchmark_count=0,
            min_runtime_backend_route_benchmarks=0,
            min_runtime_backend_route_hits_op=0,
            min_q_eval_family_cases=1,
        )
        checks = report.qeval_family_coverage_gate_checks(rows, policy)

        self.assertFalse(report.gate_failed(checks))

    def test_qeval_family_coverage_gate_fails_when_perf_output_omits_jit_or_go_baseline(self):
        rows = report.parse_go_benchmarks("""
BenchmarkQSessionEvalVectorWarmExecution/ListAdverbScan-16       100  2100 ns/op  128 B/op  4 allocs/op  1 q_pipeline_category_ordinary_list_adverb
BenchmarkQEvalVectorGoBaseline/ListAdverbScan-16                 100  1900 ns/op  0 B/op  0 allocs/op
BenchmarkQSessionEvalVectorWarmExecution/TypeMatrixShortNull-16  100  2200 ns/op  128 B/op  4 allocs/op
BenchmarkQEvalJITScriptWarm/TypeMatrixShortNull-16               100  1700 ns/op  96 B/op  3 allocs/op
""")
        policy = report.GatePolicy(
            max_leia_go_ratio=5,
            min_typed_hit_pct=95,
            max_typed_fallbacks_op=0,
            max_pipeline_fallback_shapes=0,
            max_allocs_op=64,
            min_q_eval_family_cases=1,
        )
        checks = report.qeval_family_coverage_gate_checks(rows, policy)
        failed = {(check.signal, check.benchmark) for check in checks if check.status == "fail"}

        self.assertIn(("q_eval_family_jit_cases", "ordinary_list_adverb"), failed)
        self.assertIn(("q_eval_family_go_baseline_cases", "type_matrix"), failed)
        self.assertIn(("q_eval_family_session_cases", "complex_combo"), failed)

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
            self.assertIn("q_eval_case_diagnostics", payload)
            self.assertEqual(payload["q_eval_case_diagnostics"][0]["case"], "MaskWhere")
            self.assertIn("runtime_metrics", payload)
            self.assertIn("data_runtime_hit_pct", payload["runtime_metrics"][0])
            self.assertIn("jit_route_summary", payload)
            self.assertEqual(payload["jit_route_summary"][0]["route"], "direct_return")
            self.assertIn("runtime_observability_summary", payload)
            self.assertEqual(payload["runtime_observability_summary"][0]["layer"], "qsql_kernel")
            self.assertIn("runtime_health_summary", payload)
            self.assertEqual(payload["runtime_health_summary"][0]["scope"], "q_runtime_hotpath")
            self.assertIn("runtime_bridge_efficiency_summary", payload)
            self.assertEqual(payload["runtime_bridge_efficiency_summary"][0]["scope"], "typed_runtime_and_jit_backend")
            self.assertIn("runtime_array_bridge_summary", payload)
            self.assertIn("runtime_backend_route_summary", payload)
            self.assertIn("pipeline_category_metrics", payload)
            self.assertIn("q_eval_family_coverage", payload)
            self.assertIn("pipeline_fallback_top", payload)
            self.assertIn("fallback_shape_summary", payload)
            self.assertIn("gate_policy", payload)
            markdown = md_path.read_text()
            self.assertIn("q Performance Completeness Report", markdown)
            self.assertIn("Current vs Old Leia", markdown)
            self.assertIn("Gate Summary", markdown)
            self.assertIn("q.eval Case Diagnostics", markdown)
            self.assertIn("JIT Typed Runtime Routes", markdown)
            self.assertIn("data_runtime_attempts/op", markdown)
            self.assertIn("Runtime Observability Summary", markdown)
            self.assertIn("Runtime Health Summary", markdown)
            self.assertIn("Runtime Bridge Efficiency", markdown)
            self.assertIn("Runtime Array Bridge Summary", markdown)
            self.assertIn("Runtime Primitive Registry Routes", markdown)
            self.assertIn("Ordinary q Family Coverage", markdown)
            self.assertIn("Pipeline Category Metrics", markdown)
            self.assertIn("Pipeline Fallback Top-N", markdown)

    def test_readme_documents_runtime_bridge_regression_gate(self):
        readme = (Path(__file__).resolve().parent / "README.md").read_text()

        self.assertIn("Runtime Bridge Efficiency", readme)
        self.assertIn("Runtime Array Bridge Summary", readme)
        self.assertIn("--min-runtime-direct-bridge-share-pct", readme)
        self.assertIn("--max-runtime-allocs-per-direct-call", readme)
        self.assertIn("--min-q-array-bridge-bulk-hit-pct", readme)
        self.assertIn("--max-q-array-bridge-fallbacks-op", readme)
        self.assertIn("--min-runtime-backend-route-benchmarks", readme)
        self.assertIn("--max-runtime-backend-route-errors-op", readme)
        self.assertIn("--min-q-eval-family-cases", readme)
        self.assertIn("q.eval Case Diagnostics", readme)
        self.assertIn("Pressure", readme)
        self.assertIn("Ordinary q Family Coverage", readme)
        self.assertIn("direct bridge", readme)

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
