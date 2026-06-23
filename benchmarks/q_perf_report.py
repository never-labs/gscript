#!/usr/bin/env python3
"""Build a q performance completeness report from Go benchmark output."""

from __future__ import annotations

import argparse
import json
import math
import re
import subprocess
import sys
from dataclasses import asdict, dataclass, field
from datetime import date
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]

BENCH_RE = re.compile(r"^(Benchmark[^\s]+)\s+(\d+)\s+([0-9.]+)\s+ns/op(?:\s+(.*))?$")
BENCH_NO_NS_RE = re.compile(r"^(Benchmark[^\s]+)\s+(\d+)\s+(.*)$")
BENCH_CPU_SUFFIX_RE = re.compile(r"-\d+$")
Q_PIPELINE_FALLBACK_RE = re.compile(r"q_pipeline_fallback_report\s+(.*)$")
MIN_TRUSTED_GO_BASELINE_NS = 100.0

# Ratchet baseline for "Leia beats Go" progress: per-case trusted ratios are
# captured with --update-ratio-baseline and gated as no-regression under --check.
DEFAULT_RATIO_BASELINE_PATH = Path("benchmarks/data/qeval_go_ratio_baseline.json")
RATIO_BASELINE_REGRESSION_TOLERANCE = 1.15
RATIO_BASELINE_FAMILIES = (
    ("warm_go_ratio", "BenchmarkQSessionEvalVectorWarmExecution"),
    ("jit_go_ratio", "BenchmarkQEvalJITScriptWarm"),
)
MIN_FAMILY_GEOMEAN_TRUSTED_COVERAGE = 0.5

# Real-data annex: env-injected dense columns (benchmarks/
# q_eval_realdata_bench_test.go). The synthetic 483-case suite builds data
# in-q, so lazy carriers and closed forms can absorb the work; the annex pins
# real dense-kernel cost. Its ratio family (realdata_go_ratio) is reported and
# ratcheted SEPARATELY from the synthetic families so the two stories stay
# side by side instead of averaging each other out.
REALDATA_WARM_PREFIX = "BenchmarkQEvalRealDataWarm"
REALDATA_GO_PREFIX = "BenchmarkQEvalRealDataGoBaseline"

# Hard-cap thresholds that may be sourced from `milestone_caps` in the ratio
# baseline JSON. When the matching CLI flag is left at its argparse default,
# the milestone value overrides the default; an explicitly provided CLI flag
# always wins. Caps are tightened over time via PR-reviewed edits to the JSON;
# the per-case no-regression ratchet remains the primary guard.
MILESTONE_CAP_KEYS = (
    "max_leia_go_ratio",
    "max_leia_jit_go_ratio",
    "max_leia_realdata_go_ratio",
    "min_typed_hit_pct",
    "max_typed_fallbacks_op",
    "max_pipeline_fallback_shapes",
    "max_allocs_op",
    "min_runtime_jit_backend_benchmarks",
    "min_runtime_array_bridge_benchmarks",
    "min_runtime_backend_route_benchmarks",
    "min_runtime_backend_route_hits_op",
    "max_runtime_backend_route_errors_op",
    "min_q_eval_family_cases",
    "min_q_session_planned_op_exit_op",
)
MILESTONE_CAP_HELP = (
    "When this flag is omitted, milestone_caps.%s in the ratio baseline JSON "
    "overrides the default; an explicit flag always wins."
)

QSQL_BENCH = (
    "BenchmarkQSQL("
    "BindRunSQLWarmCacheSelectWhereProject|"
    "BindRunSQLColdCacheSelectWhereProject|"
    "BindFastArg2WarmCacheSelectWhereProject|"
    "BindRunSQLWarmCacheGroupByAggregate|"
    "BindRunSQLWarmCacheJoin|"
    "BindRunSQLColdCacheJoin|"
    "BindRunSQLWarmCacheLeftJoin|"
    "BindRunSQLWarmCacheChainedJoin|"
    "BindRunSQLWarmCacheAsofJoin|"
    "BindRunSQLWarmCacheWindowJoin|"
    "BindRunSQLWarmCacheUnionJoin|"
    "BindRunSQLWarmCachePlusJoin|"
    "BindRunSQLWarmCacheDistinctOrderTake|"
    "BindRunSQLWarmCacheGroupByXbarAggregate|"
    "BindRunSQLWarmCacheUpdateWhere|"
    "BindRunSQLWarmCacheDeleteWhere|"
    "NativeGoSelectWhereProject|"
    "NativeGoGroupByAggregate|"
    "NativeGoJoin|"
    "NativeGoJoinTopK|"
    "NativeGoJoinTopKMaterialized|"
    "DataRuntimeJoinTopK|"
    "BindMatrixWarm|"
    "BindMatrixCold|"
    "NativeGoMatrix"
    ")"
)

QEVAL_BENCH = (
    "Benchmark("
    "QEvalVector(ResultCacheWarm|Cold|GoBaseline)|"
    "QSessionEvalVectorWarmExecution|"
    "QEvalJITScriptWarm|"
    "QEvalRealData(Warm|GoBaseline)"
    ")"
)

QJIT_BENCHES = (
    ("qjit-typed-runtime-callpath", "BenchmarkQEvalPipelineNativeExitCallpath/CodegenNativeExit"),
    ("qjit-array-runtime-bridge", "BenchmarkQEvalPipelineArrayRuntimeBridge/Bulk"),
    ("qjit-backend-route", "BenchmarkQFrameVectorMethodJITRoute"),
)

Q_EVAL_FAMILY_DEFS = (
    (
        "ordinary_list_adverb",
        "session rows carrying q_pipeline_category_ordinary_list_adverb",
    ),
    (
        "type_matrix",
        "TypeMatrix* benchmark cases across typed/null/promotion matrix rows",
    ),
    (
        "complex_combo",
        "Combo* benchmark cases covering depth, mixed-type, nested-adverb, dict/table, and apply/index combinations",
    ),
)


@dataclass
class CommandResult:
    label: str
    cmd: list[str]
    exit_code: int
    output: str
    parsed_benchmark_count: int = 0


@dataclass
class BenchRow:
    name: str
    iterations: int
    ns_op: float
    metrics: dict[str, float] = field(default_factory=dict)


@dataclass
class RatioRow:
    scenario: str
    numerator: str
    denominator: str
    ratio: float | None
    note: str = ""


@dataclass
class CurrentVsOldRow:
    benchmark: str
    mode: str
    current_seconds: float | None
    old_seconds: float | None
    ratio: float | None
    source: str = ""


@dataclass
class RuntimeMetricRow:
    benchmark: str
    ns_op: float
    bytes_op: float | None
    allocs_op: float | None
    kernel_hit_pct: float | None
    fallbacks_op: float | None
    typed_kernel_hit_pct: float | None
    typed_kernel_attempts_op: float | None
    typed_kernel_hits_op: float | None
    typed_kernel_fallbacks_op: float | None
    typed_kernel_errors_op: float | None
    typed_pipeline_shapes: float | None
    typed_pipeline_fallback_shapes: float | None
    data_runtime_hit_pct: float | None
    data_runtime_attempts_op: float | None
    data_runtime_hits_op: float | None
    data_runtime_fallbacks_op: float | None
    data_runtime_errors_op: float | None
    data_runtime_pipeline_shapes: float | None
    linalg_vector_attempts_op: float | None
    linalg_vector_hits_op: float | None
    linalg_vector_fallbacks_op: float | None
    linalg_vector_errors_op: float | None
    linalg_matrix_attempts_op: float | None
    linalg_matrix_hits_op: float | None
    linalg_matrix_fallbacks_op: float | None
    linalg_matrix_errors_op: float | None
    jit_typed_direct_return_op: float | None
    jit_typed_native_exit_op: float | None
    jit_typed_op_exit_op: float | None
    jit_typed_kernel_success_op: float | None
    jit_typed_kernel_errors_op: float | None
    jit_typed_pipeline_shapes: float | None
    q_session_planned_op_exit_op: float | None
    q_session_shell_fallback_op: float | None
    q_session_eval_errors_op: float | None
    q_session_backend_shapes: float | None
    q_array_bridge_bulk_hits_op: float | None
    q_array_bridge_fallbacks_op: float | None
    q_array_bridge_errors_op: float | None
    q_array_bridge_rows_op: float | None
    runtime_primitive_hits_op: float | None
    runtime_primitive_errors_op: float | None
    frame_runtime_primitive_hits_op: float | None
    frame_runtime_primitive_errors_op: float | None
    vector_runtime_primitive_hits_op: float | None
    vector_runtime_primitive_errors_op: float | None
    methodjit_frame_runtime_success_op: float | None
    methodjit_frame_runtime_errors_op: float | None
    methodjit_frame_runtime_direct_helper_op: float | None
    methodjit_frame_runtime_native_exit_op: float | None
    methodjit_frame_runtime_op_exit_op: float | None
    methodjit_vector_runtime_success_op: float | None
    methodjit_vector_runtime_errors_op: float | None
    methodjit_vector_runtime_direct_helper_op: float | None
    methodjit_vector_runtime_native_exit_op: float | None
    methodjit_vector_runtime_op_exit_op: float | None


@dataclass
class JITRouteSummaryRow:
    route: str
    calls_per_op: float
    share_pct: float
    benchmark_count: int


@dataclass
class RuntimeObservabilityRow:
    layer: str
    benchmark_count: int
    attempts_op: float | None = None
    hits_op: float | None = None
    fallbacks_op: float | None = None
    errors_op: float | None = None
    hit_pct: float | None = None
    shapes: float | None = None
    fallback_shapes: float | None = None
    direct_return_op: float | None = None
    native_exit_op: float | None = None
    op_exit_op: float | None = None
    slow_route_pct: float | None = None
    note: str = ""


@dataclass
class RuntimeHealthRow:
    scope: str
    benchmark_count: int
    avg_allocs_op: float | None
    max_allocs_op: float | None
    typed_fallbacks_op: float
    typed_errors_op: float
    pipeline_fallback_shapes: float
    jit_direct_return_op: float
    jit_native_exit_op: float
    jit_op_exit_op: float
    jit_slow_route_pct: float | None
    note: str = ""


@dataclass
class RuntimeBridgeEfficiencyRow:
    scope: str
    benchmark_count: int
    direct_calls_op: float
    slow_bridge_calls_op: float
    direct_call_share_pct: float | None
    avg_allocs_op: float | None
    allocs_per_direct_call: float | None
    note: str = ""


@dataclass
class RuntimeArrayBridgeRow:
    scope: str
    benchmark_count: int
    attempts_op: float
    bulk_hits_op: float
    fallbacks_op: float
    errors_op: float
    bulk_hit_pct: float | None
    rows_op: float
    avg_allocs_op: float | None
    max_allocs_op: float | None
    note: str = ""


@dataclass
class RuntimeBackendRouteRow:
    scope: str
    benchmark_count: int
    registry_benchmark_count: int
    methodjit_frame_vector_benchmark_count: int
    hits_op: float
    errors_op: float
    direct_helper_op: float
    native_exit_op: float
    op_exit_op: float
    hit_pct: float | None
    note: str = ""


@dataclass
class PipelineFallbackTopRow:
    category: str
    pipeline_shape: str
    kernel: str
    reason: str
    outcome: str
    count: int


@dataclass
class PipelineCategoryMetricRow:
    category: str
    benchmark_count: int
    avg_ns_op: float | None
    avg_bytes_op: float | None
    avg_allocs_op: float | None
    avg_typed_hit_pct: float | None
    total_fallbacks_op: float
    total_fallback_shapes: float


@dataclass
class QEvalCaseDiagnosticRow:
    case: str
    go_baseline_ns_op: float | None
    go_baseline_allocs_op: float | None
    trusted_go_baseline: bool
    session_ns_op: float | None
    session_allocs_op: float | None
    session_go_ratio: float | None
    result_cache_warm_ns_op: float | None
    result_cache_allocs_op: float | None
    result_cache_warm_session_ratio: float | None
    cold_ns_op: float | None
    cold_allocs_op: float | None
    cold_session_ratio: float | None
    jit_warm_ns_op: float | None
    jit_warm_allocs_op: float | None
    jit_go_ratio: float | None
    vm_warm_ns_op: float | None
    vm_warm_allocs_op: float | None
    vm_go_ratio: float | None
    typed_hit_pct: float | None
    typed_attempts_op: float | None
    typed_hits_op: float | None
    typed_fallbacks_op: float | None
    typed_errors_op: float | None
    typed_pipeline_shapes: float | None
    typed_pipeline_fallback_shapes: float | None
    jit_direct_return_op: float | None
    jit_native_exit_op: float | None
    jit_op_exit_op: float | None
    jit_backend_slow_route_pct: float | None
    jit_kernel_errors_op: float | None
    q_session_planned_op_exit_op: float | None
    q_session_shell_fallback_op: float | None
    q_session_eval_errors_op: float | None
    q_session_planned_route_pct: float | None
    q_session_backend_shapes: float | None
    primary_pressure: str
    note: str = ""


@dataclass
class GatePolicy:
    max_leia_go_ratio: float
    min_typed_hit_pct: float
    max_typed_fallbacks_op: float
    max_pipeline_fallback_shapes: float
    max_allocs_op: float
    max_leia_jit_go_ratio: float = 5.0
    max_leia_realdata_go_ratio: float = 60.0
    max_jit_typed_errors_op: float = 0.0
    max_jit_backend_slow_route_pct: float = 0.0
    min_runtime_direct_bridge_share_pct: float = 95.0
    max_runtime_allocs_per_direct_call: float = 32.0
    min_q_array_bridge_bulk_hit_pct: float = 95.0
    max_q_array_bridge_fallbacks_op: float = 0.0
    min_runtime_typed_primitive_benchmarks: int = 1
    min_runtime_jit_backend_benchmarks: int = 1
    min_runtime_array_bridge_benchmarks: int = 1
    min_runtime_bridge_benchmark_count: int = 3
    min_q_array_bridge_rows_op: float = 1.0
    max_q_array_bridge_avg_allocs_op: float = 64.0
    max_q_array_bridge_max_allocs_op: float = 64.0
    min_runtime_backend_route_benchmarks: int = 1
    min_runtime_backend_route_hits_op: float = 1.0
    max_runtime_backend_route_errors_op: float = 0.0
    min_q_eval_family_cases: int = 1
    min_q_session_planned_op_exit_op: float = 0.9


@dataclass
class GateCheck:
    signal: str
    benchmark: str
    value: float | None
    threshold: str
    status: str
    note: str = ""


@dataclass
class QEvalComputeCoverage:
    session_case_count: int
    go_baseline_case_count: int
    trusted_go_baseline_count: int
    untrusted_go_baseline_count: int
    result_cache_warm_case_count: int
    cold_case_count: int
    matched_go_baseline_count: int
    matched_result_cache_warm_count: int
    matched_cold_count: int
    missing_go_baseline: list[str]
    missing_result_cache_warm: list[str]
    missing_cold: list[str]
    orphan_go_baseline: list[str]
    untrusted_go_baselines: list[str]


@dataclass
class QEvalFamilyCoverageRow:
    family: str
    session_case_count: int
    go_baseline_case_count: int
    jit_case_count: int
    matched_go_baseline_count: int
    matched_jit_case_count: int
    missing_go_baseline: list[str]
    missing_jit_case: list[str]
    note: str = ""


@dataclass
class QSQLBenchmarkCoverage:
    leia_case_count: int
    native_go_case_count: int
    data_runtime_case_count: int
    expected_case_count: int
    matched_expected_count: int
    missing_expected: list[str]


QSQL_EXPECTED_BENCHMARKS = {
    "BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject",
    "BenchmarkQSQLBindRunSQLColdCacheSelectWhereProject",
    "BenchmarkQSQLBindFastArg2WarmCacheSelectWhereProject",
    "BenchmarkQSQLBindRunSQLWarmCacheGroupByAggregate",
    "BenchmarkQSQLBindRunSQLWarmCacheJoin",
    "BenchmarkQSQLBindRunSQLColdCacheJoin",
    "BenchmarkQSQLBindRunSQLWarmCacheLeftJoin",
    "BenchmarkQSQLBindRunSQLWarmCacheChainedJoin",
    "BenchmarkQSQLBindRunSQLWarmCacheAsofJoin",
    "BenchmarkQSQLBindRunSQLWarmCacheWindowJoin",
    "BenchmarkQSQLBindRunSQLWarmCacheUnionJoin",
    "BenchmarkQSQLBindRunSQLWarmCachePlusJoin",
    "BenchmarkQSQLBindRunSQLWarmCacheDistinctOrderTake",
    "BenchmarkQSQLBindRunSQLWarmCacheGroupByXbarAggregate",
    "BenchmarkQSQLBindRunSQLWarmCacheUpdateWhere",
    "BenchmarkQSQLBindRunSQLWarmCacheDeleteWhere",
    "BenchmarkQSQLNativeGoSelectWhereProject",
    "BenchmarkQSQLNativeGoGroupByAggregate",
    "BenchmarkQSQLDataRuntimeGroupByAggregate",
    "BenchmarkQSQLNativeGoJoin",
    "BenchmarkQSQLNativeGoJoinTopK",
    "BenchmarkQSQLNativeGoJoinTopKMaterialized",
    "BenchmarkQSQLDataRuntimeJoinTopK",
}


def run_command(label: str, cmd: list[str]) -> CommandResult:
    proc = subprocess.run(
        cmd,
        cwd=ROOT,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        check=False,
    )
    return CommandResult(label=label, cmd=cmd, exit_code=proc.returncode, output=proc.stdout)


def parse_go_benchmarks(output: str) -> dict[str, BenchRow]:
    rows: dict[str, BenchRow] = {}
    for line in output.splitlines():
        stripped = line.strip()
        match = BENCH_RE.match(stripped)
        if match:
            raw_name, iterations, ns_op, rest = match.groups()
            name = normalize_benchmark_name(raw_name)
            rows[name] = BenchRow(
                name=name,
                iterations=int(iterations),
                ns_op=float(ns_op),
                metrics=parse_metric_pairs(rest or ""),
            )
            continue
        match = BENCH_NO_NS_RE.match(stripped)
        if not match:
            continue
        raw_name, iterations, rest = match.groups()
        if not raw_name.startswith("Benchmark"):
            continue
        name = normalize_benchmark_name(raw_name)
        rows[name] = BenchRow(
            name=name,
            iterations=int(iterations),
            ns_op=0.0,
            metrics=parse_metric_pairs(rest or ""),
        )
    return rows


def normalize_benchmark_name(raw_name: str) -> str:
    return BENCH_CPU_SUFFIX_RE.sub("", raw_name)


def parse_metric_pairs(text: str) -> dict[str, float]:
    tokens = text.split()
    metrics: dict[str, float] = {}
    i = 0
    while i + 1 < len(tokens):
        try:
            value = float(tokens[i])
        except ValueError:
            i += 1
            continue
        unit = tokens[i + 1]
        metrics[unit] = value
        i += 2
    return metrics


def parse_key_value_tokens(text: str) -> dict[str, str]:
    out: dict[str, str] = {}
    for token in text.split():
        if "=" not in token:
            continue
        key, value = token.split("=", 1)
        out[key] = value
    return out


def parse_q_pipeline_fallback_reports(output: str) -> list[PipelineFallbackTopRow]:
    rows: dict[tuple[str, str, str, str, str], int] = {}
    for line in output.splitlines():
        match = Q_PIPELINE_FALLBACK_RE.search(line)
        if not match:
            continue
        fields = parse_key_value_tokens(match.group(1))
        if fields.get("rank") in ("none", None):
            continue
        try:
            count = int(fields.get("count", "0"))
        except ValueError:
            continue
        key = (
            fields.get("category") or "unknown",
            fields.get("pipeline_shape") or "unknown",
            fields.get("kernel") or "unknown",
            fields.get("reason") or "unknown",
            fields.get("outcome") or "unknown",
        )
        rows[key] = rows.get(key, 0) + count
    out = [
        PipelineFallbackTopRow(
            category=category,
            pipeline_shape=pipeline_shape,
            kernel=kernel,
            reason=reason,
            outcome=outcome,
            count=count,
        )
        for (category, pipeline_shape, kernel, reason, outcome), count in rows.items()
    ]
    out.sort(key=lambda row: (-row.count, row.category, row.pipeline_shape, row.kernel, row.reason, row.outcome))
    return out


def ratio(rows: dict[str, BenchRow], numerator: str, denominator: str) -> float | None:
    left = rows.get(numerator)
    right = rows.get(denominator)
    if left is None or right is None or right.ns_op == 0:
        return None
    return left.ns_op / right.ns_op


def trusted_qeval_go_ratio(rows: dict[str, BenchRow], session: str, go: str) -> tuple[float | None, str]:
    go_row = rows.get(go)
    if go_row is None:
        return None, "missing Go baseline"
    if go_row.ns_op < MIN_TRUSTED_GO_BASELINE_NS:
        return None, (
            f"Go baseline is {go_row.ns_op:g} ns/op, below {MIN_TRUSTED_GO_BASELINE_NS:g} ns/op; "
            "treat as correctness oracle or constant-folded baseline, not a performance denominator"
        )
    return ratio(rows, session, go), "session eval bypasses q.eval result cache and measures repeated execution"


def subject_seconds(subject: dict | None) -> float | None:
    if not isinstance(subject, dict):
        return None
    stats = subject.get("stats")
    if isinstance(stats, dict) and stats.get("median") is not None:
        return float(stats["median"])
    if subject.get("seconds") is not None:
        return float(subject["seconds"])
    return None


def subject_source(subject: dict | None) -> str:
    if not isinstance(subject, dict):
        return ""
    return str(subject.get("source") or "")


def parse_timing_compare_payload(payload: dict) -> list[CurrentVsOldRow]:
    rows: list[CurrentVsOldRow] = []
    for result in payload.get("results", []):
        if not isinstance(result, dict):
            continue
        benchmark = str(result.get("benchmark") or "")
        group = str(result.get("group") or "")
        name = f"{group}/{benchmark}" if group and benchmark else benchmark or group
        modes = result.get("modes") or {}
        if not isinstance(modes, dict):
            continue
        for mode, subjects in modes.items():
            if not isinstance(subjects, dict):
                continue
            current = subjects.get("current")
            old = subjects.get("head")
            cur_s = subject_seconds(current)
            old_s = subject_seconds(old)
            rows.append(
                CurrentVsOldRow(
                    benchmark=name,
                    mode=str(mode),
                    current_seconds=cur_s,
                    old_seconds=old_s,
                    ratio=(cur_s / old_s) if cur_s is not None and old_s not in (None, 0) else None,
                    source="/".join(part for part in (subject_source(current), subject_source(old)) if part),
                )
            )
    return rows


def parse_timing_compare_json(path: Path) -> list[CurrentVsOldRow]:
    return parse_timing_compare_payload(json.loads(path.read_text()))


def build_runtime_metric_rows(rows: dict[str, BenchRow]) -> list[RuntimeMetricRow]:
    out: list[RuntimeMetricRow] = []
    for name, row in sorted(rows.items()):
        metrics = row.metrics
        out.append(
            RuntimeMetricRow(
                benchmark=name,
                ns_op=row.ns_op,
                bytes_op=metrics.get("B/op"),
                allocs_op=metrics.get("allocs/op"),
                kernel_hit_pct=metrics.get("kernel_hit_pct"),
                fallbacks_op=metrics.get("fallbacks/op"),
                typed_kernel_hit_pct=metrics.get("typed_kernel_hit_pct"),
                typed_kernel_attempts_op=metrics.get("typed_kernel_attempts/op"),
                typed_kernel_hits_op=metrics.get("typed_kernel_hits/op"),
                typed_kernel_fallbacks_op=metrics.get("typed_kernel_fallbacks/op"),
                typed_kernel_errors_op=metrics.get("typed_kernel_errors/op"),
                typed_pipeline_shapes=metrics.get("typed_pipeline_shapes"),
                typed_pipeline_fallback_shapes=metrics.get("typed_pipeline_fallback_shapes"),
                data_runtime_hit_pct=metrics.get("data_runtime_hit_pct"),
                data_runtime_attempts_op=metrics.get("data_runtime_attempts/op"),
                data_runtime_hits_op=metrics.get("data_runtime_hits/op"),
                data_runtime_fallbacks_op=metrics.get("data_runtime_fallbacks/op"),
                data_runtime_errors_op=metrics.get("data_runtime_errors/op"),
                data_runtime_pipeline_shapes=metrics.get("data_runtime_pipeline_shapes"),
                linalg_vector_attempts_op=metrics.get("linalg_vector_attempts/op"),
                linalg_vector_hits_op=metrics.get("linalg_vector_hits/op"),
                linalg_vector_fallbacks_op=metrics.get("linalg_vector_fallbacks/op"),
                linalg_vector_errors_op=metrics.get("linalg_vector_errors/op"),
                linalg_matrix_attempts_op=metrics.get("linalg_matrix_attempts/op"),
                linalg_matrix_hits_op=metrics.get("linalg_matrix_hits/op"),
                linalg_matrix_fallbacks_op=metrics.get("linalg_matrix_fallbacks/op"),
                linalg_matrix_errors_op=metrics.get("linalg_matrix_errors/op"),
                jit_typed_direct_return_op=metrics.get("jit_typed_direct_return/op"),
                jit_typed_native_exit_op=metrics.get("jit_typed_native_exit/op"),
                jit_typed_op_exit_op=metrics.get("jit_typed_op_exit/op"),
                jit_typed_kernel_success_op=metrics.get("jit_typed_kernel_success/op"),
                jit_typed_kernel_errors_op=metrics.get("jit_typed_kernel_errors/op"),
                jit_typed_pipeline_shapes=metrics.get("jit_typed_pipeline_shapes"),
                q_session_planned_op_exit_op=metrics.get("q_session_planned_op_exit/op"),
                q_session_shell_fallback_op=metrics.get("q_session_shell_fallback/op"),
                q_session_eval_errors_op=metrics.get("q_session_eval_errors/op"),
                q_session_backend_shapes=metrics.get("q_session_backend_shapes"),
                q_array_bridge_bulk_hits_op=metrics.get("q_array_bridge_bulk_hits/op"),
                q_array_bridge_fallbacks_op=metrics.get("q_array_bridge_fallbacks/op"),
                q_array_bridge_errors_op=metrics.get("q_array_bridge_errors/op"),
                q_array_bridge_rows_op=metrics.get("q_array_bridge_rows/op"),
                runtime_primitive_hits_op=metrics.get("runtime_primitive_hits/op"),
                runtime_primitive_errors_op=metrics.get("runtime_primitive_errors/op"),
                frame_runtime_primitive_hits_op=metrics.get("frame_runtime_primitive_hits/op"),
                frame_runtime_primitive_errors_op=metrics.get("frame_runtime_primitive_errors/op"),
                vector_runtime_primitive_hits_op=metrics.get("vector_runtime_primitive_hits/op"),
                vector_runtime_primitive_errors_op=metrics.get("vector_runtime_primitive_errors/op"),
                methodjit_frame_runtime_success_op=metrics.get("methodjit_frame_runtime_success/op"),
                methodjit_frame_runtime_errors_op=metrics.get("methodjit_frame_runtime_errors/op"),
                methodjit_frame_runtime_direct_helper_op=metrics.get("methodjit_frame_runtime_direct_helper/op"),
                methodjit_frame_runtime_native_exit_op=metrics.get("methodjit_frame_runtime_native_exit/op"),
                methodjit_frame_runtime_op_exit_op=metrics.get("methodjit_frame_runtime_op_exit/op"),
                methodjit_vector_runtime_success_op=metrics.get("methodjit_vector_runtime_success/op"),
                methodjit_vector_runtime_errors_op=metrics.get("methodjit_vector_runtime_errors/op"),
                methodjit_vector_runtime_direct_helper_op=metrics.get("methodjit_vector_runtime_direct_helper/op"),
                methodjit_vector_runtime_native_exit_op=metrics.get("methodjit_vector_runtime_native_exit/op"),
                methodjit_vector_runtime_op_exit_op=metrics.get("methodjit_vector_runtime_op_exit/op"),
            )
        )
    return out


def build_fallback_shape_rows(rows: dict[str, BenchRow]) -> list[RuntimeMetricRow]:
    return [
        row
        for row in build_runtime_metric_rows(rows)
        if (row.typed_pipeline_fallback_shapes or 0) > 0
        or (row.typed_kernel_fallbacks_op or 0) > 0
        or (row.fallbacks_op or 0) > 0
    ]


def build_jit_route_summary(rows: dict[str, BenchRow]) -> list[JITRouteSummaryRow]:
    routes = {
        "direct_return": "jit_typed_direct_return_op",
        "native_exit": "jit_typed_native_exit_op",
        "op_exit": "jit_typed_op_exit_op",
        "success": "jit_typed_kernel_success_op",
        "error": "jit_typed_kernel_errors_op",
    }
    totals = {route: 0.0 for route in routes}
    counts = {route: 0 for route in routes}
    for row in build_runtime_metric_rows(rows):
        for route, attr in routes.items():
            value = getattr(row, attr)
            if value is None:
                continue
            totals[route] += value
            counts[route] += 1
    route_total = totals["direct_return"] + totals["native_exit"] + totals["op_exit"]
    out: list[JITRouteSummaryRow] = []
    for route in ("direct_return", "native_exit", "op_exit", "success", "error"):
        share = 0.0
        if route in {"direct_return", "native_exit", "op_exit"} and route_total > 0:
            share = 100 * totals[route] / route_total
        out.append(
            JITRouteSummaryRow(
                route=route,
                calls_per_op=totals[route],
                share_pct=share,
                benchmark_count=counts[route],
            )
        )
    return out


def build_runtime_observability_summary(rows: dict[str, BenchRow]) -> list[RuntimeObservabilityRow]:
    runtime_rows = build_runtime_metric_rows(rows)
    out: list[RuntimeObservabilityRow] = []

    qsql_kernel_rows = [row for row in runtime_rows if row.kernel_hit_pct is not None or row.fallbacks_op is not None]
    if qsql_kernel_rows:
        hit_values = [row.kernel_hit_pct for row in qsql_kernel_rows if row.kernel_hit_pct is not None]
        out.append(
            RuntimeObservabilityRow(
                layer="qsql_kernel",
                benchmark_count=len(qsql_kernel_rows),
                fallbacks_op=sum(row.fallbacks_op or 0.0 for row in qsql_kernel_rows),
                hit_pct=average(hit_values),
                note="qSQL bind/runtime kernel metrics emitted directly by qSQL benchmarks",
            )
        )

    typed_rows = [row for row in runtime_rows if row.typed_kernel_attempts_op is not None]
    if typed_rows:
        attempts = sum(row.typed_kernel_attempts_op or 0.0 for row in typed_rows)
        hits = sum(row.typed_kernel_hits_op or 0.0 for row in typed_rows)
        fallbacks = sum(row.typed_kernel_fallbacks_op or 0.0 for row in typed_rows)
        errors = sum(row.typed_kernel_errors_op or 0.0 for row in typed_rows)
        out.append(
            RuntimeObservabilityRow(
                layer="typed_primitive",
                benchmark_count=len(typed_rows),
                attempts_op=attempts,
                hits_op=hits,
                fallbacks_op=fallbacks,
                errors_op=errors,
                hit_pct=(100 * hits / attempts) if attempts > 0 else None,
                note="ordinary q typed primitive dispatch across session-execution benchmarks",
            )
        )

    pipeline_rows = [row for row in runtime_rows if row.typed_pipeline_shapes is not None]
    if pipeline_rows:
        shapes = sum(row.typed_pipeline_shapes or 0.0 for row in pipeline_rows)
        fallback_shapes = sum(row.typed_pipeline_fallback_shapes or 0.0 for row in pipeline_rows)
        out.append(
            RuntimeObservabilityRow(
                layer="unified_pipeline",
                benchmark_count=len(pipeline_rows),
                hit_pct=(100 * (shapes - fallback_shapes) / shapes) if shapes > 0 else None,
                shapes=shapes,
                fallback_shapes=fallback_shapes,
                note="recognized q expression pipeline shapes and shapes that still fell back",
            )
        )

    data_runtime_rows = [row for row in runtime_rows if row.data_runtime_attempts_op is not None]
    if data_runtime_rows:
        attempts = sum(row.data_runtime_attempts_op or 0.0 for row in data_runtime_rows)
        hits = sum(row.data_runtime_hits_op or 0.0 for row in data_runtime_rows)
        fallbacks = sum(row.data_runtime_fallbacks_op or 0.0 for row in data_runtime_rows)
        errors = sum(row.data_runtime_errors_op or 0.0 for row in data_runtime_rows)
        out.append(
            RuntimeObservabilityRow(
                layer="data_runtime",
                benchmark_count=len(data_runtime_rows),
                attempts_op=attempts,
                hits_op=hits,
                fallbacks_op=fallbacks,
                errors_op=errors,
                hit_pct=(100 * hits / attempts) if attempts > 0 else None,
                shapes=sum(row.data_runtime_pipeline_shapes or 0.0 for row in data_runtime_rows),
                note="shared data-runtime typed kernels, reported separately from q typed primitive gates",
            )
        )

    for layer, prefix in (("linalg_vector", "linalg_vector"), ("linalg_matrix", "linalg_matrix")):
        layer_rows = [row for row in runtime_rows if getattr(row, f"{prefix}_attempts_op") is not None]
        if not layer_rows:
            continue
        attempts = sum(getattr(row, f"{prefix}_attempts_op") or 0.0 for row in layer_rows)
        hits = sum(getattr(row, f"{prefix}_hits_op") or 0.0 for row in layer_rows)
        fallbacks = sum(getattr(row, f"{prefix}_fallbacks_op") or 0.0 for row in layer_rows)
        errors = sum(getattr(row, f"{prefix}_errors_op") or 0.0 for row in layer_rows)
        out.append(
            RuntimeObservabilityRow(
                layer=layer,
                benchmark_count=len(layer_rows),
                attempts_op=attempts,
                hits_op=hits,
                fallbacks_op=fallbacks,
                errors_op=errors,
                hit_pct=(100 * hits / attempts) if attempts > 0 else None,
                note="linalg data-runtime family; report-only until dedicated scientific gates are enabled",
            )
        )

    jit_rows = [
        row
        for row in runtime_rows
        if row.jit_typed_direct_return_op is not None
        or row.jit_typed_native_exit_op is not None
        or row.jit_typed_op_exit_op is not None
        or row.jit_typed_kernel_success_op is not None
        or row.jit_typed_kernel_errors_op is not None
        or row.q_session_planned_op_exit_op is not None
        or row.q_session_shell_fallback_op is not None
        or row.q_session_eval_errors_op is not None
    ]
    if jit_rows:
        direct = sum(row.jit_typed_direct_return_op or 0.0 for row in jit_rows)
        native = sum(row.jit_typed_native_exit_op or 0.0 for row in jit_rows)
        op_exit = sum(row.jit_typed_op_exit_op or 0.0 for row in jit_rows)
        success = sum(row.jit_typed_kernel_success_op or 0.0 for row in jit_rows)
        errors = sum(row.jit_typed_kernel_errors_op or 0.0 for row in jit_rows)
        session_planned = sum(row.q_session_planned_op_exit_op or 0.0 for row in jit_rows)
        session_shell = sum(row.q_session_shell_fallback_op or 0.0 for row in jit_rows)
        session_errors = sum(row.q_session_eval_errors_op or 0.0 for row in jit_rows)
        route_total = direct + native + op_exit
        session_route_total = session_planned + session_shell + session_errors
        kernel_total = success + errors
        if kernel_total == 0 and (session_planned + session_shell + session_errors) > 0:
            kernel_total = session_planned + session_shell + session_errors
            success = session_planned
            errors = session_errors
        out.append(
            RuntimeObservabilityRow(
                layer="jit_backend",
                benchmark_count=len(jit_rows),
                attempts_op=kernel_total if kernel_total > 0 else None,
                hits_op=success,
                errors_op=errors,
                hit_pct=(100 * success / kernel_total) if kernel_total > 0 else None,
                shapes=sum((row.jit_typed_pipeline_shapes or 0.0) + (row.q_session_backend_shapes or 0.0) for row in jit_rows),
                direct_return_op=direct + session_planned,
                native_exit_op=native,
                op_exit_op=op_exit + session_shell,
                slow_route_pct=(
                    (100 * (session_shell + session_errors) / session_route_total)
                    if session_route_total > 0
                    else ((100 * (native + op_exit) / route_total) if route_total > 0 else None)
                ),
                note="JIT typed backend route split; session planned exits are the steady q session hot route",
            )
        )

    array_bridge_rows = [
        row
        for row in runtime_rows
        if row.q_array_bridge_bulk_hits_op is not None
        or row.q_array_bridge_fallbacks_op is not None
        or row.q_array_bridge_errors_op is not None
    ]
    if array_bridge_rows:
        bulk_hits = sum(row.q_array_bridge_bulk_hits_op or 0.0 for row in array_bridge_rows)
        fallbacks = sum(row.q_array_bridge_fallbacks_op or 0.0 for row in array_bridge_rows)
        errors = sum(row.q_array_bridge_errors_op or 0.0 for row in array_bridge_rows)
        attempts = bulk_hits + fallbacks + errors
        out.append(
            RuntimeObservabilityRow(
                layer="methodjit_array_bridge",
                benchmark_count=len(array_bridge_rows),
                attempts_op=attempts,
                hits_op=bulk_hits,
                fallbacks_op=fallbacks,
                errors_op=errors,
                hit_pct=(100 * bulk_hits / attempts) if attempts > 0 else None,
                shapes=sum(row.q_array_bridge_rows_op or 0.0 for row in array_bridge_rows),
                note="MethodJIT q pipeline data.Array to runtime.Value bridge; hits use bulk typed export",
            )
        )

    return out


def build_runtime_health_summary(rows: dict[str, BenchRow]) -> list[RuntimeHealthRow]:
    runtime_rows = build_runtime_metric_rows(rows)
    health_rows = [
        row
        for row in runtime_rows
        if row.typed_kernel_attempts_op is not None
        or row.typed_pipeline_shapes is not None
        or row.jit_typed_direct_return_op is not None
        or row.jit_typed_native_exit_op is not None
        or row.jit_typed_op_exit_op is not None
        or row.jit_typed_kernel_success_op is not None
        or row.jit_typed_kernel_errors_op is not None
        or row.q_session_planned_op_exit_op is not None
        or row.q_session_shell_fallback_op is not None
        or row.q_session_eval_errors_op is not None
        or row.q_array_bridge_bulk_hits_op is not None
        or row.q_array_bridge_fallbacks_op is not None
        or row.q_array_bridge_errors_op is not None
    ]
    if not health_rows:
        return []
    direct = sum(row.jit_typed_direct_return_op or 0.0 for row in health_rows)
    native = sum(row.jit_typed_native_exit_op or 0.0 for row in health_rows)
    op_exit = sum(row.jit_typed_op_exit_op or 0.0 for row in health_rows)
    session_planned = sum(row.q_session_planned_op_exit_op or 0.0 for row in health_rows)
    session_shell = sum(row.q_session_shell_fallback_op or 0.0 for row in health_rows)
    session_errors = sum(row.q_session_eval_errors_op or 0.0 for row in health_rows)
    route_total = direct + native + op_exit
    session_route_total = session_planned + session_shell + session_errors
    alloc_values = [row.allocs_op for row in health_rows if row.allocs_op is not None]
    return [
        RuntimeHealthRow(
            scope="q_runtime_hotpath",
            benchmark_count=len(health_rows),
            avg_allocs_op=average(alloc_values),
            max_allocs_op=max(alloc_values) if alloc_values else None,
            typed_fallbacks_op=sum(row.typed_kernel_fallbacks_op or 0.0 for row in health_rows),
            typed_errors_op=sum(row.typed_kernel_errors_op or 0.0 for row in health_rows)
            + sum(row.jit_typed_kernel_errors_op or 0.0 for row in health_rows)
            + sum(row.q_session_eval_errors_op or 0.0 for row in health_rows),
            pipeline_fallback_shapes=sum(row.typed_pipeline_fallback_shapes or 0.0 for row in health_rows),
            jit_direct_return_op=direct + session_planned,
            jit_native_exit_op=native,
            jit_op_exit_op=op_exit + session_shell,
            jit_slow_route_pct=(
                (100 * (session_shell + session_errors) / session_route_total)
                if session_route_total > 0
                else ((100 * (native + op_exit) / route_total) if route_total > 0 else None)
            ),
            note=(
                "combined health of typed primitive fallback, pipeline fallback, "
                "JIT route split, and allocation pressure"
            ),
        )
    ]


def build_runtime_bridge_efficiency_summary(rows: dict[str, BenchRow]) -> list[RuntimeBridgeEfficiencyRow]:
    runtime_rows = build_runtime_metric_rows(rows)
    bridge_rows = [
        row
        for row in runtime_rows
        if row.typed_kernel_attempts_op is not None
        or row.jit_typed_direct_return_op is not None
        or row.jit_typed_native_exit_op is not None
        or row.jit_typed_op_exit_op is not None
        or row.jit_typed_kernel_success_op is not None
        or row.jit_typed_kernel_errors_op is not None
        or row.q_session_planned_op_exit_op is not None
        or row.q_session_shell_fallback_op is not None
        or row.q_session_eval_errors_op is not None
        or row.q_array_bridge_bulk_hits_op is not None
        or row.q_array_bridge_fallbacks_op is not None
        or row.q_array_bridge_errors_op is not None
    ]
    if not bridge_rows:
        return []
    typed_direct = sum(row.typed_kernel_hits_op or 0.0 for row in bridge_rows)
    typed_slow = sum(row.typed_kernel_fallbacks_op or 0.0 for row in bridge_rows)
    typed_slow += sum(row.typed_kernel_errors_op or 0.0 for row in bridge_rows)
    jit_direct = sum(row.jit_typed_direct_return_op or 0.0 for row in bridge_rows)
    jit_slow = sum(row.jit_typed_native_exit_op or 0.0 for row in bridge_rows)
    jit_slow += sum(row.jit_typed_op_exit_op or 0.0 for row in bridge_rows)
    jit_slow += sum(row.jit_typed_kernel_errors_op or 0.0 for row in bridge_rows)
    jit_direct += sum(row.q_session_planned_op_exit_op or 0.0 for row in bridge_rows)
    jit_slow += sum(row.q_session_shell_fallback_op or 0.0 for row in bridge_rows)
    jit_slow += sum(row.q_session_eval_errors_op or 0.0 for row in bridge_rows)
    array_direct = sum(row.q_array_bridge_bulk_hits_op or 0.0 for row in bridge_rows)
    array_slow = sum(row.q_array_bridge_fallbacks_op or 0.0 for row in bridge_rows)
    array_slow += sum(row.q_array_bridge_errors_op or 0.0 for row in bridge_rows)
    direct = typed_direct + jit_direct + array_direct
    slow = typed_slow + jit_slow + array_slow
    total = direct + slow
    alloc_values = [row.allocs_op for row in bridge_rows if row.allocs_op is not None]
    avg_allocs = average(alloc_values)
    return [
        RuntimeBridgeEfficiencyRow(
            scope="typed_runtime_and_jit_backend",
            benchmark_count=len(bridge_rows),
            direct_calls_op=direct,
            slow_bridge_calls_op=slow,
            direct_call_share_pct=(100 * direct / total) if total > 0 else None,
            avg_allocs_op=avg_allocs,
            allocs_per_direct_call=(avg_allocs / direct) if avg_allocs is not None and direct > 0 else None,
            note=(
                "direct calls combine typed primitive hits, JIT direct returns, and q session planned exits; "
                "slow bridge calls combine typed fallback/error, JIT native/op exits, session shell fallback, and errors"
            ),
        )
    ]


def build_runtime_array_bridge_summary(rows: dict[str, BenchRow]) -> list[RuntimeArrayBridgeRow]:
    runtime_rows = build_runtime_metric_rows(rows)
    array_rows = [
        row
        for row in runtime_rows
        if row.q_array_bridge_bulk_hits_op is not None
        or row.q_array_bridge_fallbacks_op is not None
        or row.q_array_bridge_errors_op is not None
    ]
    if not array_rows:
        return []
    bulk_hits = sum(row.q_array_bridge_bulk_hits_op or 0.0 for row in array_rows)
    fallbacks = sum(row.q_array_bridge_fallbacks_op or 0.0 for row in array_rows)
    errors = sum(row.q_array_bridge_errors_op or 0.0 for row in array_rows)
    attempts = bulk_hits + fallbacks + errors
    alloc_values = [row.allocs_op for row in array_rows if row.allocs_op is not None]
    return [
        RuntimeArrayBridgeRow(
            scope="methodjit_array_bridge",
            benchmark_count=len(array_rows),
            attempts_op=attempts,
            bulk_hits_op=bulk_hits,
            fallbacks_op=fallbacks,
            errors_op=errors,
            bulk_hit_pct=(100 * bulk_hits / attempts) if attempts > 0 else None,
            rows_op=sum(row.q_array_bridge_rows_op or 0.0 for row in array_rows),
            avg_allocs_op=average(alloc_values),
            max_allocs_op=max(alloc_values) if alloc_values else None,
            note="MethodJIT q array bridge route split; bulk hits avoid row-wise Array.At fallback",
        )
    ]


def build_runtime_backend_route_summary(rows: dict[str, BenchRow]) -> list[RuntimeBackendRouteRow]:
    runtime_rows = build_runtime_metric_rows(rows)
    route_rows = [
        row
        for row in runtime_rows
        if row.runtime_primitive_hits_op is not None
        or row.runtime_primitive_errors_op is not None
        or row.frame_runtime_primitive_hits_op is not None
        or row.frame_runtime_primitive_errors_op is not None
        or row.vector_runtime_primitive_hits_op is not None
        or row.vector_runtime_primitive_errors_op is not None
        or row.methodjit_frame_runtime_success_op is not None
        or row.methodjit_frame_runtime_errors_op is not None
        or row.methodjit_frame_runtime_direct_helper_op is not None
        or row.methodjit_frame_runtime_native_exit_op is not None
        or row.methodjit_frame_runtime_op_exit_op is not None
        or row.methodjit_vector_runtime_success_op is not None
        or row.methodjit_vector_runtime_errors_op is not None
        or row.methodjit_vector_runtime_direct_helper_op is not None
        or row.methodjit_vector_runtime_native_exit_op is not None
        or row.methodjit_vector_runtime_op_exit_op is not None
    ]
    if not route_rows:
        return []
    registry_rows = [
        row
        for row in route_rows
        if row.runtime_primitive_hits_op is not None
        or row.runtime_primitive_errors_op is not None
        or row.frame_runtime_primitive_hits_op is not None
        or row.frame_runtime_primitive_errors_op is not None
        or row.vector_runtime_primitive_hits_op is not None
        or row.vector_runtime_primitive_errors_op is not None
    ]
    frame_vector_rows = [
        row
        for row in route_rows
        if row.methodjit_frame_runtime_success_op is not None
        or row.methodjit_frame_runtime_errors_op is not None
        or row.methodjit_frame_runtime_direct_helper_op is not None
        or row.methodjit_frame_runtime_native_exit_op is not None
        or row.methodjit_frame_runtime_op_exit_op is not None
        or row.methodjit_vector_runtime_success_op is not None
        or row.methodjit_vector_runtime_errors_op is not None
        or row.methodjit_vector_runtime_direct_helper_op is not None
        or row.methodjit_vector_runtime_native_exit_op is not None
        or row.methodjit_vector_runtime_op_exit_op is not None
    ]
    hits = sum(row.runtime_primitive_hits_op or 0.0 for row in route_rows)
    hits += sum(row.frame_runtime_primitive_hits_op or 0.0 for row in route_rows)
    hits += sum(row.vector_runtime_primitive_hits_op or 0.0 for row in route_rows)
    hits += sum(row.methodjit_frame_runtime_success_op or 0.0 for row in route_rows)
    hits += sum(row.methodjit_vector_runtime_success_op or 0.0 for row in route_rows)
    errors = sum(row.runtime_primitive_errors_op or 0.0 for row in route_rows)
    errors += sum(row.frame_runtime_primitive_errors_op or 0.0 for row in route_rows)
    errors += sum(row.vector_runtime_primitive_errors_op or 0.0 for row in route_rows)
    errors += sum(row.methodjit_frame_runtime_errors_op or 0.0 for row in route_rows)
    errors += sum(row.methodjit_vector_runtime_errors_op or 0.0 for row in route_rows)
    direct_helper = sum(row.methodjit_frame_runtime_direct_helper_op or 0.0 for row in route_rows)
    direct_helper += sum(row.methodjit_vector_runtime_direct_helper_op or 0.0 for row in route_rows)
    native_exit = sum(row.methodjit_frame_runtime_native_exit_op or 0.0 for row in route_rows)
    native_exit += sum(row.methodjit_vector_runtime_native_exit_op or 0.0 for row in route_rows)
    op_exit = sum(row.methodjit_frame_runtime_op_exit_op or 0.0 for row in route_rows)
    op_exit += sum(row.methodjit_vector_runtime_op_exit_op or 0.0 for row in route_rows)
    attempts = hits + errors
    return [
        RuntimeBackendRouteRow(
            scope="runtime_primitive_registry_and_frame_vector_routes",
            benchmark_count=len(route_rows),
            registry_benchmark_count=len(registry_rows),
            methodjit_frame_vector_benchmark_count=len(frame_vector_rows),
            hits_op=hits,
            errors_op=errors,
            direct_helper_op=direct_helper,
            native_exit_op=native_exit,
            op_exit_op=op_exit,
            hit_pct=(100 * hits / attempts) if attempts > 0 else None,
            note=(
                "VM primitive registry plus MethodJIT frame/vector typed-runtime route counters; "
                "presence is a contract that backend route stats did not silently disappear"
            ),
        )
    ]


def average(values: list[float]) -> float | None:
    if not values:
        return None
    return sum(values) / len(values)


def build_pipeline_category_metric_rows(rows: dict[str, BenchRow]) -> list[PipelineCategoryMetricRow]:
    grouped: dict[str, list[BenchRow]] = {}
    for row in rows.values():
        if not row.name.startswith("BenchmarkQSessionEvalVectorWarmExecution/"):
            continue
        for metric in row.metrics:
            if metric.startswith("q_pipeline_category_") and row.metrics.get(metric, 0) > 0:
                category = metric.removeprefix("q_pipeline_category_")
                grouped.setdefault(category, []).append(row)
    out: list[PipelineCategoryMetricRow] = []
    for category, items in sorted(grouped.items()):
        out.append(
            PipelineCategoryMetricRow(
                category=category,
                benchmark_count=len(items),
                avg_ns_op=average([item.ns_op for item in items]),
                avg_bytes_op=average([item.metrics["B/op"] for item in items if "B/op" in item.metrics]),
                avg_allocs_op=average([item.metrics["allocs/op"] for item in items if "allocs/op" in item.metrics]),
                avg_typed_hit_pct=average([item.metrics["typed_kernel_hit_pct"] for item in items if "typed_kernel_hit_pct" in item.metrics]),
                total_fallbacks_op=sum(item.metrics.get("typed_kernel_fallbacks/op", item.metrics.get("fallbacks/op", 0.0)) for item in items),
                total_fallback_shapes=sum(item.metrics.get("typed_pipeline_fallback_shapes", 0.0) for item in items),
            )
        )
    return out


def row_metric(rows: dict[str, BenchRow], name: str, metric: str) -> float | None:
    row = rows.get(name)
    if row is None:
        return None
    return row.metrics.get(metric)


def row_ns(rows: dict[str, BenchRow], name: str) -> float | None:
    row = rows.get(name)
    if row is None:
        return None
    return row.ns_op


def safe_ratio_value(numerator: float | None, denominator: float | None) -> float | None:
    if numerator is None or denominator in (None, 0):
        return None
    return numerator / denominator


def classify_qeval_pressure(
    *,
    trusted_go: bool,
    go_ns: float | None,
    session_ns: float | None,
    cold_session_ratio: float | None,
    typed_fallbacks: float | None,
    typed_errors: float | None,
    typed_fallback_shapes: float | None,
    session_allocs: float | None,
    jit_ratio: float | None,
    jit_slow_route_pct: float | None,
    jit_errors: float | None,
) -> str:
    if go_ns is None:
        return "missing_go_baseline"
    if not trusted_go:
        return "untrusted_go_baseline"
    if session_ns is None:
        return "missing_session_warm"
    if (typed_errors or 0.0) > 0:
        return "typed_errors"
    if (typed_fallbacks or 0.0) > 0 or (typed_fallback_shapes or 0.0) > 0:
        return "typed_fallback"
    if (jit_errors or 0.0) > 0:
        return "jit_backend_errors"
    if (jit_slow_route_pct or 0.0) > 0:
        return "jit_slow_route"
    if session_allocs is not None and session_allocs > 64:
        return "alloc_pressure"
    if cold_session_ratio is None:
        return "missing_cold"
    if cold_session_ratio > 2.0:
        return "cold_start_pressure"
    if jit_ratio is None:
        return "missing_jit_warm"
    return "healthy_or_ratio_only"


def qeval_diagnostic_note(row: QEvalCaseDiagnosticRow) -> str:
    parts = [row.primary_pressure]
    if row.session_go_ratio is not None:
        parts.append(f"session/go={row.session_go_ratio:.3f}x")
    if row.jit_go_ratio is not None:
        parts.append(f"jit/go={row.jit_go_ratio:.3f}x")
    if row.cold_session_ratio is not None:
        parts.append(f"cold/session={row.cold_session_ratio:.3f}x")
    if row.typed_hit_pct is not None:
        parts.append(f"typed_hit={row.typed_hit_pct:.1f}%")
    if row.typed_fallbacks_op is not None:
        parts.append(f"typed_fallbacks/op={row.typed_fallbacks_op:.3f}")
    if row.jit_backend_slow_route_pct is not None:
        parts.append(f"jit_slow_route={row.jit_backend_slow_route_pct:.1f}%")
    if row.q_session_planned_op_exit_op is not None:
        parts.append(f"session_planned/op={row.q_session_planned_op_exit_op:.3f}")
    if row.q_session_shell_fallback_op is not None:
        parts.append(f"session_shell/op={row.q_session_shell_fallback_op:.3f}")
    if row.q_session_eval_errors_op is not None:
        parts.append(f"session_errors/op={row.q_session_eval_errors_op:.3f}")
    if row.q_session_planned_route_pct is not None:
        parts.append(f"session_planned_route={row.q_session_planned_route_pct:.1f}%")
    if row.session_allocs_op is not None:
        parts.append(f"session_allocs/op={row.session_allocs_op:.0f}")
    return "; ".join(parts)


def build_qeval_case_diagnostics(rows: dict[str, BenchRow]) -> list[QEvalCaseDiagnosticRow]:
    cases = sorted(
        qeval_cases(rows, "BenchmarkQSessionEvalVectorWarmExecution")
        | qeval_cases(rows, "BenchmarkQEvalVectorGoBaseline")
        | qeval_cases(rows, "BenchmarkQEvalVectorResultCacheWarm")
        | qeval_cases(rows, "BenchmarkQEvalVectorCold")
        | qeval_cases(rows, "BenchmarkQEvalJITScriptWarm")
        | qeval_cases(rows, "BenchmarkQEvalVMScriptWarm")
    )
    out: list[QEvalCaseDiagnosticRow] = []
    for case in cases:
        session = f"BenchmarkQSessionEvalVectorWarmExecution/{case}"
        go = f"BenchmarkQEvalVectorGoBaseline/{case}"
        warm = f"BenchmarkQEvalVectorResultCacheWarm/{case}"
        cold = f"BenchmarkQEvalVectorCold/{case}"
        jit = f"BenchmarkQEvalJITScriptWarm/{case}"
        vm = f"BenchmarkQEvalVMScriptWarm/{case}"
        go_ns = row_ns(rows, go)
        session_ns = row_ns(rows, session)
        cold_ratio = safe_ratio_value(row_ns(rows, cold), session_ns)
        jit_direct = row_metric(rows, jit, "jit_typed_direct_return/op")
        jit_native = row_metric(rows, jit, "jit_typed_native_exit/op")
        jit_op_exit = row_metric(rows, jit, "jit_typed_op_exit/op")
        q_session_planned = row_metric(rows, jit, "q_session_planned_op_exit/op")
        q_session_shell = row_metric(rows, jit, "q_session_shell_fallback/op")
        q_session_errors = row_metric(rows, jit, "q_session_eval_errors/op")
        jit_route_total = (jit_direct or 0.0) + (jit_native or 0.0) + (jit_op_exit or 0.0)
        q_session_route_total = (q_session_planned or 0.0) + (q_session_shell or 0.0) + (q_session_errors or 0.0)
        jit_slow_route_pct = None
        q_session_planned_route_pct = None
        if q_session_route_total > 0:
            jit_slow_route_pct = 100 * ((q_session_shell or 0.0) + (q_session_errors or 0.0)) / q_session_route_total
            q_session_planned_route_pct = 100 * (q_session_planned or 0.0) / q_session_route_total
        elif jit_route_total > 0:
            jit_slow_route_pct = 100 * ((jit_native or 0.0) + (jit_op_exit or 0.0)) / jit_route_total
        typed_fallbacks = row_metric(rows, session, "typed_kernel_fallbacks/op")
        typed_errors = row_metric(rows, session, "typed_kernel_errors/op")
        typed_fallback_shapes = row_metric(rows, session, "typed_pipeline_fallback_shapes")
        trusted_go = go_ns is not None and go_ns >= MIN_TRUSTED_GO_BASELINE_NS
        jit_ratio = safe_ratio_value(row_ns(rows, jit), go_ns) if trusted_go else None
        row = QEvalCaseDiagnosticRow(
            case=case,
            go_baseline_ns_op=go_ns,
            go_baseline_allocs_op=row_metric(rows, go, "allocs/op"),
            trusted_go_baseline=trusted_go,
            session_ns_op=session_ns,
            session_allocs_op=row_metric(rows, session, "allocs/op"),
            session_go_ratio=safe_ratio_value(session_ns, go_ns) if trusted_go else None,
            result_cache_warm_ns_op=row_ns(rows, warm),
            result_cache_allocs_op=row_metric(rows, warm, "allocs/op"),
            result_cache_warm_session_ratio=safe_ratio_value(row_ns(rows, warm), session_ns),
            cold_ns_op=row_ns(rows, cold),
            cold_allocs_op=row_metric(rows, cold, "allocs/op"),
            cold_session_ratio=cold_ratio,
            jit_warm_ns_op=row_ns(rows, jit),
            jit_warm_allocs_op=row_metric(rows, jit, "allocs/op"),
            jit_go_ratio=jit_ratio,
            vm_warm_ns_op=row_ns(rows, vm),
            vm_warm_allocs_op=row_metric(rows, vm, "allocs/op"),
            vm_go_ratio=safe_ratio_value(row_ns(rows, vm), go_ns) if trusted_go else None,
            typed_hit_pct=row_metric(rows, session, "typed_kernel_hit_pct"),
            typed_attempts_op=row_metric(rows, session, "typed_kernel_attempts/op"),
            typed_hits_op=row_metric(rows, session, "typed_kernel_hits/op"),
            typed_fallbacks_op=typed_fallbacks,
            typed_errors_op=typed_errors,
            typed_pipeline_shapes=row_metric(rows, session, "typed_pipeline_shapes"),
            typed_pipeline_fallback_shapes=typed_fallback_shapes,
            jit_direct_return_op=jit_direct,
            jit_native_exit_op=jit_native,
            jit_op_exit_op=jit_op_exit,
            jit_backend_slow_route_pct=jit_slow_route_pct,
            jit_kernel_errors_op=row_metric(rows, jit, "jit_typed_kernel_errors/op"),
            q_session_planned_op_exit_op=q_session_planned,
            q_session_shell_fallback_op=q_session_shell,
            q_session_eval_errors_op=q_session_errors,
            q_session_planned_route_pct=q_session_planned_route_pct,
            q_session_backend_shapes=row_metric(rows, jit, "q_session_backend_shapes"),
            primary_pressure="",
        )
        row.primary_pressure = classify_qeval_pressure(
            trusted_go=trusted_go,
            go_ns=go_ns,
            session_ns=session_ns,
            cold_session_ratio=cold_ratio,
            typed_fallbacks=typed_fallbacks,
            typed_errors=typed_errors,
            typed_fallback_shapes=typed_fallback_shapes,
            session_allocs=row.session_allocs_op,
            jit_ratio=jit_ratio,
            jit_slow_route_pct=jit_slow_route_pct,
            jit_errors=(row.jit_kernel_errors_op or 0.0) + (q_session_errors or 0.0),
        )
        row.note = qeval_diagnostic_note(row)
        out.append(row)
    return out


def qeval_cases(rows: dict[str, BenchRow], prefix: str) -> set[str]:
    marker = prefix + "/"
    return {name.removeprefix(marker) for name in rows if name.startswith(marker)}


def build_qeval_compute_coverage(rows: dict[str, BenchRow]) -> QEvalComputeCoverage:
    session = qeval_cases(rows, "BenchmarkQSessionEvalVectorWarmExecution")
    go = qeval_cases(rows, "BenchmarkQEvalVectorGoBaseline")
    warm = qeval_cases(rows, "BenchmarkQEvalVectorResultCacheWarm")
    cold = qeval_cases(rows, "BenchmarkQEvalVectorCold")
    trusted_go = {
        case
        for case in go
        if rows.get(f"BenchmarkQEvalVectorGoBaseline/{case}", BenchRow("", 0, 0)).ns_op >= MIN_TRUSTED_GO_BASELINE_NS
    }
    untrusted_go = go - trusted_go
    return QEvalComputeCoverage(
        session_case_count=len(session),
        go_baseline_case_count=len(go),
        trusted_go_baseline_count=len(trusted_go),
        untrusted_go_baseline_count=len(untrusted_go),
        result_cache_warm_case_count=len(warm),
        cold_case_count=len(cold),
        matched_go_baseline_count=len(session & go),
        matched_result_cache_warm_count=len(session & warm),
        matched_cold_count=len(session & cold),
        missing_go_baseline=sorted(session - go),
        missing_result_cache_warm=sorted(session - warm),
        missing_cold=sorted(session - cold),
        orphan_go_baseline=sorted(go - session),
        untrusted_go_baselines=sorted(untrusted_go),
    )


def qeval_family_session_cases(rows: dict[str, BenchRow], family: str) -> set[str]:
    if family == "ordinary_list_adverb":
        prefix = "BenchmarkQSessionEvalVectorWarmExecution/"
        metric = "q_pipeline_category_ordinary_list_adverb"
        return {
            name.removeprefix(prefix)
            for name, row in rows.items()
            if name.startswith(prefix) and row.metrics.get(metric, 0.0) > 0
        }
    if family == "type_matrix":
        return {
            case
            for case in qeval_cases(rows, "BenchmarkQSessionEvalVectorWarmExecution")
            if case.startswith("TypeMatrix")
        }
    if family == "complex_combo":
        return {
            case
            for case in qeval_cases(rows, "BenchmarkQSessionEvalVectorWarmExecution")
            if case.startswith("Combo")
        }
    return set()


def qeval_family_named_cases(rows: dict[str, BenchRow], prefix: str, family: str) -> set[str]:
    cases = qeval_cases(rows, prefix)
    if family == "ordinary_list_adverb":
        session_cases = qeval_family_session_cases(rows, family)
        return cases & session_cases
    if family == "type_matrix":
        return {case for case in cases if case.startswith("TypeMatrix")}
    if family == "complex_combo":
        return {case for case in cases if case.startswith("Combo")}
    return set()


def build_qeval_family_coverage(rows: dict[str, BenchRow]) -> list[QEvalFamilyCoverageRow]:
    out: list[QEvalFamilyCoverageRow] = []
    for family, note in Q_EVAL_FAMILY_DEFS:
        session = qeval_family_session_cases(rows, family)
        go = qeval_family_named_cases(rows, "BenchmarkQEvalVectorGoBaseline", family)
        jit = qeval_family_named_cases(rows, "BenchmarkQEvalJITScriptWarm", family)
        out.append(
            QEvalFamilyCoverageRow(
                family=family,
                session_case_count=len(session),
                go_baseline_case_count=len(go),
                jit_case_count=len(jit),
                matched_go_baseline_count=len(session & go),
                matched_jit_case_count=len(session & jit),
                missing_go_baseline=sorted(session - go),
                missing_jit_case=sorted(session - jit),
                note=note,
            )
        )
    return out


def build_qsql_benchmark_coverage(rows: dict[str, BenchRow]) -> QSQLBenchmarkCoverage:
    present = set(rows)
    return QSQLBenchmarkCoverage(
        leia_case_count=sum(
            1
            for name in rows
            if name.startswith("BenchmarkQSQLBind")
        ),
        native_go_case_count=sum(
            1
            for name in rows
            if name.startswith("BenchmarkQSQLNativeGo")
        ),
        data_runtime_case_count=sum(
            1
            for name in rows
            if name.startswith("BenchmarkQSQLDataRuntime")
        ),
        expected_case_count=len(QSQL_EXPECTED_BENCHMARKS),
        matched_expected_count=len(QSQL_EXPECTED_BENCHMARKS & present),
        missing_expected=sorted(QSQL_EXPECTED_BENCHMARKS - present),
    )


def build_ratios(rows: dict[str, BenchRow]) -> list[RatioRow]:
    ratios = [
        RatioRow(
            "qSQL select/filter/project",
            "BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject",
            "BenchmarkQSQLNativeGoSelectWhereProject",
            ratio(rows, "BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject", "BenchmarkQSQLNativeGoSelectWhereProject"),
        ),
        RatioRow(
            "qSQL select/filter/project warm-vs-cold",
            "BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject",
            "BenchmarkQSQLBindRunSQLColdCacheSelectWhereProject",
            ratio(rows, "BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject", "BenchmarkQSQLBindRunSQLColdCacheSelectWhereProject"),
        ),
        RatioRow(
            "qSQL group aggregate",
            "BenchmarkQSQLBindRunSQLWarmCacheGroupByAggregate",
            "BenchmarkQSQLNativeGoGroupByAggregate",
            ratio(rows, "BenchmarkQSQLBindRunSQLWarmCacheGroupByAggregate", "BenchmarkQSQLNativeGoGroupByAggregate"),
        ),
        RatioRow(
            "qSQL join/order/take vs Go full materialization",
            "BenchmarkQSQLBindRunSQLWarmCacheJoin",
            "BenchmarkQSQLNativeGoJoin",
            ratio(rows, "BenchmarkQSQLBindRunSQLWarmCacheJoin", "BenchmarkQSQLNativeGoJoin"),
        ),
        RatioRow(
            "qSQL join/order/take vs Go topK materialized",
            "BenchmarkQSQLBindRunSQLWarmCacheJoin",
            "BenchmarkQSQLNativeGoJoinTopKMaterialized",
            ratio(rows, "BenchmarkQSQLBindRunSQLWarmCacheJoin", "BenchmarkQSQLNativeGoJoinTopKMaterialized"),
            "fairer hand-written Go baseline for fused topK shape",
        ),
    ]
    qsql_matrix_cases = sorted(
        name.removeprefix("BenchmarkQSQLBindMatrixWarm/")
        for name in rows
        if name.startswith("BenchmarkQSQLBindMatrixWarm/")
    )
    for case in qsql_matrix_cases:
        warm = f"BenchmarkQSQLBindMatrixWarm/{case}"
        cold = f"BenchmarkQSQLBindMatrixCold/{case}"
        go = f"BenchmarkQSQLNativeGoMatrix/{case}"
        ratios.extend(
            [
                RatioRow(
                    f"qSQL matrix {case} warm vs Go",
                    warm,
                    go,
                    ratio(rows, warm, go),
                ),
                RatioRow(
                    f"qSQL matrix {case} cold vs warm",
                    cold,
                    warm,
                    ratio(rows, cold, warm),
                ),
            ]
        )
    qeval_cases = sorted(
        name.removeprefix("BenchmarkQSessionEvalVectorWarmExecution/")
        for name in rows
        if name.startswith("BenchmarkQSessionEvalVectorWarmExecution/")
    )
    for case in qeval_cases:
        session = f"BenchmarkQSessionEvalVectorWarmExecution/{case}"
        go = f"BenchmarkQEvalVectorGoBaseline/{case}"
        warm = f"BenchmarkQEvalVectorResultCacheWarm/{case}"
        cold = f"BenchmarkQEvalVectorCold/{case}"
        go_ratio, go_note = trusted_qeval_go_ratio(rows, session, go)
        ratios.extend(
            [
                RatioRow(
                    f"q.eval {case} session execution vs Go",
                    session,
                    go,
                    go_ratio,
                    go_note,
                ),
                RatioRow(
                    f"q.eval {case} result-cache warm vs session execution",
                    warm,
                    session,
                    ratio(rows, warm, session),
                    "warm result-cache hits are not recomputation",
                ),
                RatioRow(
                    f"q.eval {case} cold vs session execution",
                    cold,
                    session,
                    ratio(rows, cold, session),
                ),
            ]
        )
    jit_script_cases = sorted(
        name.removeprefix("BenchmarkQEvalJITScriptWarm/")
        for name in rows
        if name.startswith("BenchmarkQEvalJITScriptWarm/")
    )
    for case in jit_script_cases:
        numerator = f"BenchmarkQEvalJITScriptWarm/{case}"
        go = f"BenchmarkQEvalVectorGoBaseline/{case}"
        jit_ratio, jit_note = trusted_qeval_go_ratio(rows, numerator, go)
        ratios.append(
            RatioRow(
                f"q.eval {case} JIT script warm vs Go",
                numerator,
                go,
                jit_ratio,
                jit_note if jit_ratio is None else "JIT-compiled q script warm execution vs hand-written Go",
            )
        )
    vm_script_cases = sorted(
        name.removeprefix("BenchmarkQEvalVMScriptWarm/")
        for name in rows
        if name.startswith("BenchmarkQEvalVMScriptWarm/")
    )
    for case in vm_script_cases:
        numerator = f"BenchmarkQEvalVMScriptWarm/{case}"
        go = f"BenchmarkQEvalVectorGoBaseline/{case}"
        vm_ratio, vm_note = trusted_qeval_go_ratio(rows, numerator, go)
        ratios.append(
            RatioRow(
                f"q.eval {case} VM script warm vs Go",
                numerator,
                go,
                vm_ratio,
                vm_note if vm_ratio is None else "VM script warm execution vs hand-written Go; attribution only, not gated",
            )
        )
    return ratios


def qeval_case_from_benchmark(name: str) -> str | None:
    for prefix in (
        "BenchmarkQSessionEvalVectorWarmExecution/",
        "BenchmarkQEvalJITScriptWarm/",
        "BenchmarkQEvalVectorResultCacheWarm/",
        "BenchmarkQEvalVectorCold/",
        "BenchmarkQSQLBindMatrixWarm/",
    ):
        if name.startswith(prefix):
            return name.removeprefix(prefix)
    return None


def qeval_diagnostic_notes(rows: dict[str, BenchRow]) -> dict[str, str]:
    return {row.case: row.note for row in build_qeval_case_diagnostics(rows)}


def append_note(*parts: str) -> str:
    return "; ".join(part for part in parts if part)


def ratio_gate_checks(rows: dict[str, BenchRow], policy: GatePolicy, ratio_baseline: dict | None = None) -> list[GateCheck]:
    checks: list[GateCheck] = []
    exceptions = (ratio_baseline or {}).get("exceptions") or {}
    diagnostics = qeval_diagnostic_notes(rows)
    for item in build_ratios(rows):
        if "Go" not in item.denominator:
            continue
        if item.numerator.startswith("BenchmarkQEvalVMScriptWarm/"):
            # VM script ratios are reported for attribution only; never hard-cap gated.
            continue
        if item.numerator.startswith("BenchmarkQEvalJITScriptWarm/"):
            signal = "leia_jit_go_ratio"
            cap = policy.max_leia_jit_go_ratio
        else:
            signal = "leia_go_ratio"
            cap = policy.max_leia_go_ratio
        case = qeval_case_from_benchmark(item.numerator)
        if case is not None and case in exceptions:
            exception = exceptions.get(case) or {}
            reason = exception.get("reason") or "listed in baseline exceptions"
            checks.append(
                GateCheck(
                    signal=signal,
                    benchmark=item.numerator,
                    value=item.ratio,
                    threshold=f"<= {cap:g}",
                    status="skip",
                    note=f"exempt from hard cap via baseline exception: {reason} (no-regression gate still applies)",
                )
            )
            continue
        if item.ratio is None:
            checks.append(
                GateCheck(
                    signal=signal,
                    benchmark=item.numerator,
                    value=None,
                    threshold=f"<= {cap:g}",
                    status="skip",
                    note=append_note(item.note or "missing or untrusted denominator", diagnostics.get(case or "")),
                )
            )
            continue
        checks.append(
            GateCheck(
                signal=signal,
                benchmark=item.numerator,
                value=item.ratio,
                threshold=f"<= {cap:g}",
                status="pass" if item.ratio <= cap else "fail",
                note=append_note(item.note, diagnostics.get(case or "")),
            )
        )
    return checks


@dataclass
class QEvalRealDataRow:
    case: str
    warm_ns_op: float | None
    warm_allocs_op: float | None
    go_ns_op: float | None
    trusted_go_baseline: bool
    realdata_go_ratio: float | None
    note: str = ""


def build_qeval_realdata_rows(rows: dict[str, BenchRow]) -> list[QEvalRealDataRow]:
    cases = sorted(qeval_cases(rows, REALDATA_WARM_PREFIX) | qeval_cases(rows, REALDATA_GO_PREFIX))
    out: list[QEvalRealDataRow] = []
    for case in cases:
        warm = f"{REALDATA_WARM_PREFIX}/{case}"
        go = f"{REALDATA_GO_PREFIX}/{case}"
        go_ns = row_ns(rows, go)
        trusted = go_ns is not None and go_ns >= MIN_TRUSTED_GO_BASELINE_NS
        ratio_value, note = trusted_qeval_go_ratio(rows, warm, go)
        out.append(
            QEvalRealDataRow(
                case=case,
                warm_ns_op=row_ns(rows, warm),
                warm_allocs_op=row_metric(rows, warm, "allocs/op"),
                go_ns_op=go_ns,
                trusted_go_baseline=trusted,
                realdata_go_ratio=ratio_value,
                note=note if ratio_value is None else "env-injected dense columns; closed forms cannot fire",
            )
        )
    return out


def realdata_trusted_geomean(rows: dict[str, BenchRow]) -> tuple[float | None, int, int]:
    """Geomean over trusted realdata_go_ratio cases: (value, trusted, total).

    Reported in its own section and NEVER folded into the synthetic family
    geomeans: the point of the annex is to keep the synthetic warm-dispatch
    story and the real-data kernel story side by side.
    """
    items = build_qeval_realdata_rows(rows)
    trusted = [item.realdata_go_ratio for item in items if item.realdata_go_ratio is not None and item.realdata_go_ratio > 0]
    if not trusted:
        return None, 0, len(items)
    return geomean(trusted), len(trusted), len(items)


def collect_realdata_baseline_cases(rows: dict[str, BenchRow], existing: dict | None = None) -> dict[str, dict[str, float | None]]:
    existing_cases = (existing or {}).get("realdata_cases") or {}
    warm = qeval_cases(rows, REALDATA_WARM_PREFIX)
    go = qeval_cases(rows, REALDATA_GO_PREFIX)
    if not warm or not go:
        # Bench input without annex rows must not wipe the captured realdata
        # ratchet; preserve the existing entries unchanged.
        return existing_cases
    cases: dict[str, dict[str, float | None]] = {}
    for case in sorted(warm & go):
        value, _ = trusted_qeval_go_ratio(rows, f"{REALDATA_WARM_PREFIX}/{case}", f"{REALDATA_GO_PREFIX}/{case}")
        cases[case] = {"realdata_go_ratio": round(value, 4) if value is not None else None}
    return cases


def realdata_gate_checks(rows: dict[str, BenchRow], policy: GatePolicy, ratio_baseline: dict | None) -> list[GateCheck]:
    checks: list[GateCheck] = []
    warm_cases = sorted(qeval_cases(rows, REALDATA_WARM_PREFIX))
    if not warm_cases:
        return checks
    baseline_cases = (ratio_baseline or {}).get("realdata_cases") or {}
    tolerance = RATIO_BASELINE_REGRESSION_TOLERANCE
    for case in warm_cases:
        numerator = f"{REALDATA_WARM_PREFIX}/{case}"
        denominator = f"{REALDATA_GO_PREFIX}/{case}"
        current, note = trusted_qeval_go_ratio(rows, numerator, denominator)
        cap = policy.max_leia_realdata_go_ratio
        if current is None:
            checks.append(
                GateCheck(
                    signal="leia_realdata_go_ratio",
                    benchmark=numerator,
                    value=None,
                    threshold=f"<= {cap:g}",
                    status="skip",
                    note=note or "missing or untrusted Go baseline",
                )
            )
            continue
        checks.append(
            GateCheck(
                signal="leia_realdata_go_ratio",
                benchmark=numerator,
                value=current,
                threshold=f"<= {cap:g}",
                status="pass" if current <= cap else "fail",
                note="real-data annex hard cap",
            )
        )
        if ratio_baseline is None:
            continue
        entry = baseline_cases.get(case) or {}
        baseline_value = entry.get("realdata_go_ratio")
        if baseline_value is None:
            checks.append(
                GateCheck(
                    signal="realdata_go_ratio_regression",
                    benchmark=numerator,
                    value=current,
                    threshold="no baseline entry",
                    status="pass",
                    note="case not in realdata baseline yet; it will be captured at the next --update-ratio-baseline",
                )
            )
            continue
        limit = baseline_value * tolerance
        checks.append(
            GateCheck(
                signal="realdata_go_ratio_regression",
                benchmark=numerator,
                value=current,
                threshold=f"<= {limit:.4f}",
                status="pass" if current <= limit else "fail",
                note=f"baseline realdata_go_ratio {baseline_value:g} * tolerance {tolerance:g}",
            )
        )
    return checks


def load_ratio_baseline(path: Path) -> dict | None:
    if not path.exists():
        return None
    return json.loads(path.read_text())


def count_untrusted_go_baselines(rows: dict[str, BenchRow]) -> int:
    return sum(
        1
        for name, row in rows.items()
        if name.startswith("BenchmarkQEvalVectorGoBaseline/") and row.ns_op < MIN_TRUSTED_GO_BASELINE_NS
    )


def collect_ratio_baseline_cases(rows: dict[str, BenchRow]) -> dict[str, dict[str, float | None]]:
    go = qeval_cases(rows, "BenchmarkQEvalVectorGoBaseline")
    session = qeval_cases(rows, "BenchmarkQSessionEvalVectorWarmExecution")
    jit = qeval_cases(rows, "BenchmarkQEvalJITScriptWarm")
    cases: dict[str, dict[str, float | None]] = {}
    for case in sorted(go & (session | jit)):
        denominator = f"BenchmarkQEvalVectorGoBaseline/{case}"
        warm_ratio, _ = trusted_qeval_go_ratio(rows, f"BenchmarkQSessionEvalVectorWarmExecution/{case}", denominator)
        jit_ratio, _ = trusted_qeval_go_ratio(rows, f"BenchmarkQEvalJITScriptWarm/{case}", denominator)
        cases[case] = {
            "warm_go_ratio": round(warm_ratio, 4) if warm_ratio is not None else None,
            "jit_go_ratio": round(jit_ratio, 4) if jit_ratio is not None else None,
        }
    return cases


def build_ratio_baseline_payload(rows: dict[str, BenchRow], existing: dict | None = None) -> dict:
    existing = existing or {}
    return {
        "schema_version": 1,
        "captured": date.today().isoformat(),
        "max_untrusted_go_baselines": count_untrusted_go_baselines(rows),
        "cases": collect_ratio_baseline_cases(rows),
        "realdata_cases": collect_realdata_baseline_cases(rows, existing),
        "family_targets": existing.get("family_targets") or {},
        "exceptions": existing.get("exceptions") or {},
        "milestone_caps": existing.get("milestone_caps") or {},
    }


def flag_in_argv(argv: list[str], key: str) -> bool:
    flag = "--" + key.replace("_", "-")
    return any(token == flag or token.startswith(flag + "=") for token in argv)


def apply_milestone_caps(
    args: argparse.Namespace,
    parser: argparse.ArgumentParser,
    ratio_baseline: dict | None,
    argv: list[str] | None = None,
) -> list[str]:
    """Override default-valued threshold flags with milestone_caps from the ratio baseline.

    A flag that is explicitly provided on the command line (or whose parsed
    value differs from the argparse default) always wins over milestone_caps.
    A missing or empty milestone_caps object leaves every flag untouched.
    """
    caps = (ratio_baseline or {}).get("milestone_caps") or {}
    applied: list[str] = []
    for key in MILESTONE_CAP_KEYS:
        cap_value = caps.get(key)
        if cap_value is None:
            continue
        if argv is not None and flag_in_argv(argv, key):
            continue
        default = parser.get_default(key)
        if getattr(args, key) != default:
            continue
        cap_type = type(default)
        setattr(args, key, cap_type(cap_value))
        applied.append(key)
    return applied


def write_ratio_baseline(path: Path, payload: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n")


def geomean(values: list[float]) -> float:
    return math.exp(sum(math.log(value) for value in values) / len(values))


def family_geomean_gate_checks(rows: dict[str, BenchRow], ratio_baseline: dict) -> list[GateCheck]:
    checks: list[GateCheck] = []
    family_targets = ratio_baseline.get("family_targets") or {}
    if not family_targets:
        checks.append(
            GateCheck(
                signal="family_geomean_jit_go_ratio",
                benchmark="family_targets",
                value=None,
                threshold="none configured",
                status="skip",
                note="family_targets is empty in the ratio baseline; populated in a later phase",
            )
        )
        return checks
    for family, spec in sorted(family_targets.items()):
        spec = spec or {}
        cases = spec.get("cases") or []
        threshold = spec.get("max_geomean_jit_go_ratio")
        if not cases or threshold is None:
            checks.append(
                GateCheck(
                    signal="family_geomean_jit_go_ratio",
                    benchmark=family,
                    value=None,
                    threshold="invalid target",
                    status="skip",
                    note="family target needs a non-empty cases list and max_geomean_jit_go_ratio",
                )
            )
            continue
        trusted: list[float] = []
        for case in cases:
            value, _ = trusted_qeval_go_ratio(
                rows,
                f"BenchmarkQEvalJITScriptWarm/{case}",
                f"BenchmarkQEvalVectorGoBaseline/{case}",
            )
            if value is not None and value > 0:
                trusted.append(value)
        coverage = len(trusted) / len(cases)
        if coverage < MIN_FAMILY_GEOMEAN_TRUSTED_COVERAGE:
            checks.append(
                GateCheck(
                    signal="family_geomean_jit_go_ratio",
                    benchmark=family,
                    value=None,
                    threshold=f"<= {threshold:g}",
                    status="skip",
                    note=(
                        f"only {len(trusted)}/{len(cases)} cases have a trusted jit_go_ratio "
                        "(below 50% coverage); family skipped"
                    ),
                )
            )
            continue
        value = geomean(trusted)
        checks.append(
            GateCheck(
                signal="family_geomean_jit_go_ratio",
                benchmark=family,
                value=value,
                threshold=f"<= {threshold:g}",
                status="pass" if value <= threshold else "fail",
                note=f"geomean over {len(trusted)}/{len(cases)} trusted jit_go_ratio cases",
            )
        )
    return checks


def ratio_baseline_gate_checks(rows: dict[str, BenchRow], policy: GatePolicy, ratio_baseline: dict | None) -> list[GateCheck]:
    checks: list[GateCheck] = []
    if ratio_baseline is None:
        checks.append(
            GateCheck(
                signal="ratio_baseline",
                benchmark="qeval_go_ratio_baseline",
                value=None,
                threshold="baseline file present",
                status="skip",
                note="ratio baseline file missing; capture it with --update-ratio-baseline",
            )
        )
        return checks
    baseline_cases = ratio_baseline.get("cases") or {}
    tolerance = RATIO_BASELINE_REGRESSION_TOLERANCE
    for key, prefix in RATIO_BASELINE_FAMILIES:
        for case in sorted(qeval_cases(rows, prefix)):
            numerator = f"{prefix}/{case}"
            denominator = f"BenchmarkQEvalVectorGoBaseline/{case}"
            current, _ = trusted_qeval_go_ratio(rows, numerator, denominator)
            if current is None:
                continue
            entry = baseline_cases.get(case) or {}
            baseline_value = entry.get(key)
            if baseline_value is None:
                checks.append(
                    GateCheck(
                        signal="leia_go_ratio_regression",
                        benchmark=numerator,
                        value=current,
                        threshold="no baseline entry",
                        status="pass",
                        note="case not in ratio baseline yet; it will be captured at the next --update-ratio-baseline",
                    )
                )
                continue
            limit = baseline_value * tolerance
            checks.append(
                GateCheck(
                    signal="leia_go_ratio_regression",
                    benchmark=numerator,
                    value=current,
                    threshold=f"<= {limit:.4f}",
                    status="pass" if current <= limit else "fail",
                    note=f"baseline {key} {baseline_value:g} * tolerance {tolerance:g}",
                )
            )
    max_untrusted = ratio_baseline.get("max_untrusted_go_baselines")
    if max_untrusted is not None:
        untrusted = count_untrusted_go_baselines(rows)
        checks.append(
            GateCheck(
                signal="untrusted_go_baseline_count",
                benchmark="BenchmarkQEvalVectorGoBaseline",
                value=float(untrusted),
                threshold=f"<= {max_untrusted:g}",
                status="pass" if untrusted <= max_untrusted else "fail",
                note=(
                    f"Go baselines below {MIN_TRUSTED_GO_BASELINE_NS:g} ns/op are correctness-only; "
                    "ratchet the cap down with --update-ratio-baseline"
                ),
            )
        )
    exceptions = ratio_baseline.get("exceptions") or {}
    checks.append(
        GateCheck(
            signal="ratio_baseline_exception_count",
            benchmark="qeval_go_ratio_baseline",
            value=float(len(exceptions)),
            threshold="report-only",
            status="pass",
            note="hard-cap exceptions; shrink-only is enforced by PR review of the baseline JSON",
        )
    )
    known_cases = (
        qeval_cases(rows, "BenchmarkQEvalVectorGoBaseline")
        | qeval_cases(rows, "BenchmarkQSessionEvalVectorWarmExecution")
        | qeval_cases(rows, "BenchmarkQEvalJITScriptWarm")
        | qeval_cases(rows, "BenchmarkQSQLBindMatrixWarm")
    )
    if known_cases:
        for case in sorted(exceptions):
            if case in known_cases:
                continue
            checks.append(
                GateCheck(
                    signal="ratio_baseline_stale_exception",
                    benchmark=case,
                    value=None,
                    threshold="exception case must exist in bench rows",
                    status="fail",
                    note="remove the stale case from exceptions in the ratio baseline JSON",
                )
            )
    checks.extend(family_geomean_gate_checks(rows, ratio_baseline))
    return checks


def runtime_gate_checks(rows: dict[str, BenchRow], policy: GatePolicy) -> list[GateCheck]:
    checks: list[GateCheck] = []
    diagnostics = qeval_diagnostic_notes(rows)
    for item in build_runtime_metric_rows(rows):
        if item.benchmark.startswith((REALDATA_WARM_PREFIX + "/", REALDATA_GO_PREFIX + "/")):
            # The real-data annex is gated by its own family
            # (leia_realdata_go_ratio + realdata_go_ratio_regression); the
            # synthetic allocs/op cap does not apply to env-injected dense
            # workloads or their hand-written Go baselines.
            continue
        case = qeval_case_from_benchmark(item.benchmark)
        note = diagnostics.get(case or "")
        if item.typed_kernel_hit_pct is not None:
            checks.append(
                GateCheck(
                    signal="typed_hit_pct",
                    benchmark=item.benchmark,
                    value=item.typed_kernel_hit_pct,
                    threshold=f">= {policy.min_typed_hit_pct:g}",
                    status="pass" if item.typed_kernel_hit_pct >= policy.min_typed_hit_pct else "fail",
                    note=note,
                )
            )
        fallback_value = item.typed_kernel_fallbacks_op
        if fallback_value is None:
            fallback_value = item.fallbacks_op
        if fallback_value is not None:
            checks.append(
                GateCheck(
                    signal="fallbacks_op",
                    benchmark=item.benchmark,
                    value=fallback_value,
                    threshold=f"<= {policy.max_typed_fallbacks_op:g}",
                    status="pass" if fallback_value <= policy.max_typed_fallbacks_op else "fail",
                    note=note,
                )
            )
        if item.typed_pipeline_fallback_shapes is not None:
            checks.append(
                GateCheck(
                    signal="pipeline_fallback_shapes",
                    benchmark=item.benchmark,
                    value=item.typed_pipeline_fallback_shapes,
                    threshold=f"<= {policy.max_pipeline_fallback_shapes:g}",
                    status="pass" if item.typed_pipeline_fallback_shapes <= policy.max_pipeline_fallback_shapes else "fail",
                    note=note,
                )
            )
        if item.allocs_op is not None:
            checks.append(
                GateCheck(
                    signal="allocs_op",
                    benchmark=item.benchmark,
                    value=item.allocs_op,
                    threshold=f"<= {policy.max_allocs_op:g}",
                    status="pass" if item.allocs_op <= policy.max_allocs_op else "fail",
                    note=note,
                )
            )
        if item.jit_typed_kernel_errors_op is not None:
            checks.append(
                GateCheck(
                    signal="jit_typed_errors_op",
                    benchmark=item.benchmark,
                    value=item.jit_typed_kernel_errors_op,
                    threshold=f"<= {policy.max_jit_typed_errors_op:g}",
                    status="pass" if item.jit_typed_kernel_errors_op <= policy.max_jit_typed_errors_op else "fail",
                    note=note,
                )
            )
        if item.q_session_shell_fallback_op is not None:
            checks.append(
                GateCheck(
                    signal="q_session_shell_fallback_op",
                    benchmark=item.benchmark,
                    value=item.q_session_shell_fallback_op,
                    threshold=f"<= {policy.max_typed_fallbacks_op:g}",
                    status="pass" if item.q_session_shell_fallback_op <= policy.max_typed_fallbacks_op else "fail",
                    note=note,
                )
            )
        if item.benchmark.startswith("BenchmarkQEvalJITScriptWarm/"):
            route_metrics_present = (
                item.q_session_planned_op_exit_op is not None
                and item.q_session_shell_fallback_op is not None
                and item.q_session_eval_errors_op is not None
                and item.q_session_backend_shapes is not None
            )
            checks.append(
                GateCheck(
                    signal="q_session_route_metrics_present",
                    benchmark=item.benchmark,
                    value=1.0 if route_metrics_present else 0.0,
                    threshold="present",
                    status="pass" if route_metrics_present else "fail",
                    note=note,
                )
            )
            if item.q_session_planned_op_exit_op is not None:
                checks.append(
                    GateCheck(
                        signal="q_session_planned_op_exit_op",
                        benchmark=item.benchmark,
                        value=item.q_session_planned_op_exit_op,
                        threshold=f">= {policy.min_q_session_planned_op_exit_op:g}",
                        status=(
                            "pass"
                            if item.q_session_planned_op_exit_op >= policy.min_q_session_planned_op_exit_op
                            else "fail"
                        ),
                        note=note,
                    )
                )
            if item.q_session_backend_shapes is not None:
                checks.append(
                    GateCheck(
                        signal="q_session_backend_shapes",
                        benchmark=item.benchmark,
                        value=item.q_session_backend_shapes,
                        threshold=">= 1",
                        status="pass" if item.q_session_backend_shapes >= 1 else "fail",
                        note=note,
                    )
                )
        if item.q_session_eval_errors_op is not None:
            checks.append(
                GateCheck(
                    signal="q_session_eval_errors_op",
                    benchmark=item.benchmark,
                    value=item.q_session_eval_errors_op,
                    threshold=f"<= {policy.max_jit_typed_errors_op:g}",
                    status="pass" if item.q_session_eval_errors_op <= policy.max_jit_typed_errors_op else "fail",
                    note=note,
                )
            )
    return checks


def observability_gate_checks(rows: dict[str, BenchRow], policy: GatePolicy) -> list[GateCheck]:
    checks: list[GateCheck] = []
    for item in build_runtime_observability_summary(rows):
        if item.layer == "typed_primitive" and item.hit_pct is not None:
            checks.append(
                GateCheck(
                    signal="typed_primitive_hit_pct",
                    benchmark=item.layer,
                    value=item.hit_pct,
                    threshold=f">= {policy.min_typed_hit_pct:g}",
                    status="pass" if item.hit_pct >= policy.min_typed_hit_pct else "fail",
                    note=item.note,
                )
            )
        if item.layer == "typed_primitive" and item.fallbacks_op is not None:
            checks.append(
                GateCheck(
                    signal="typed_primitive_fallbacks_op",
                    benchmark=item.layer,
                    value=item.fallbacks_op,
                    threshold=f"<= {policy.max_typed_fallbacks_op:g}",
                    status="pass" if item.fallbacks_op <= policy.max_typed_fallbacks_op else "fail",
                    note=item.note,
                )
            )
        if item.layer == "unified_pipeline" and item.fallback_shapes is not None:
            checks.append(
                GateCheck(
                    signal="unified_pipeline_fallback_shapes",
                    benchmark=item.layer,
                    value=item.fallback_shapes,
                    threshold=f"<= {policy.max_pipeline_fallback_shapes:g}",
                    status="pass" if item.fallback_shapes <= policy.max_pipeline_fallback_shapes else "fail",
                    note=item.note,
                )
            )
        if item.layer == "jit_backend" and item.errors_op is not None:
            checks.append(
                GateCheck(
                    signal="jit_backend_errors_op",
                    benchmark=item.layer,
                    value=item.errors_op,
                    threshold=f"<= {policy.max_jit_typed_errors_op:g}",
                    status="pass" if item.errors_op <= policy.max_jit_typed_errors_op else "fail",
                    note=item.note,
                )
            )
        if item.layer == "jit_backend" and item.slow_route_pct is not None:
            checks.append(
                GateCheck(
                    signal="jit_backend_slow_route_pct",
                    benchmark=item.layer,
                    value=item.slow_route_pct,
                    threshold=f"<= {policy.max_jit_backend_slow_route_pct:g}",
                    status="pass" if item.slow_route_pct <= policy.max_jit_backend_slow_route_pct else "fail",
                    note=item.note,
                )
            )
    return checks


def runtime_health_gate_checks(rows: dict[str, BenchRow], policy: GatePolicy) -> list[GateCheck]:
    checks: list[GateCheck] = []
    for item in build_runtime_health_summary(rows):
        checks.append(
            GateCheck(
                signal="runtime_health_typed_fallbacks_op",
                benchmark=item.scope,
                value=item.typed_fallbacks_op,
                threshold=f"<= {policy.max_typed_fallbacks_op:g}",
                status="pass" if item.typed_fallbacks_op <= policy.max_typed_fallbacks_op else "fail",
                note=item.note,
            )
        )
        checks.append(
            GateCheck(
                signal="runtime_health_typed_errors_op",
                benchmark=item.scope,
                value=item.typed_errors_op,
                threshold=f"<= {policy.max_jit_typed_errors_op:g}",
                status="pass" if item.typed_errors_op <= policy.max_jit_typed_errors_op else "fail",
                note=item.note,
            )
        )
        checks.append(
            GateCheck(
                signal="runtime_health_pipeline_fallback_shapes",
                benchmark=item.scope,
                value=item.pipeline_fallback_shapes,
                threshold=f"<= {policy.max_pipeline_fallback_shapes:g}",
                status="pass" if item.pipeline_fallback_shapes <= policy.max_pipeline_fallback_shapes else "fail",
                note=item.note,
            )
        )
        if item.max_allocs_op is not None:
            checks.append(
                GateCheck(
                    signal="runtime_health_max_allocs_op",
                    benchmark=item.scope,
                    value=item.max_allocs_op,
                    threshold=f"<= {policy.max_allocs_op:g}",
                    status="pass" if item.max_allocs_op <= policy.max_allocs_op else "fail",
                    note=item.note,
                )
            )
        if item.jit_slow_route_pct is not None:
            checks.append(
                GateCheck(
                    signal="runtime_health_jit_slow_route_pct",
                    benchmark=item.scope,
                    value=item.jit_slow_route_pct,
                    threshold=f"<= {policy.max_jit_backend_slow_route_pct:g}",
                    status="pass" if item.jit_slow_route_pct <= policy.max_jit_backend_slow_route_pct else "fail",
                    note=item.note,
                )
            )
    return checks


def runtime_bridge_efficiency_gate_checks(rows: dict[str, BenchRow], policy: GatePolicy) -> list[GateCheck]:
    checks: list[GateCheck] = []
    for item in build_runtime_bridge_efficiency_summary(rows):
        if item.direct_call_share_pct is not None:
            checks.append(
                GateCheck(
                    signal="runtime_bridge_direct_call_share_pct",
                    benchmark=item.scope,
                    value=item.direct_call_share_pct,
                    threshold=f">= {policy.min_runtime_direct_bridge_share_pct:g}",
                    status=(
                        "pass"
                        if item.direct_call_share_pct >= policy.min_runtime_direct_bridge_share_pct
                        else "fail"
                    ),
                    note=item.note,
                )
            )
        if item.allocs_per_direct_call is not None:
            checks.append(
                GateCheck(
                    signal="runtime_bridge_allocs_per_direct_call",
                    benchmark=item.scope,
                    value=item.allocs_per_direct_call,
                    threshold=f"<= {policy.max_runtime_allocs_per_direct_call:g}",
                    status=(
                        "pass"
                        if item.allocs_per_direct_call <= policy.max_runtime_allocs_per_direct_call
                        else "fail"
                    ),
                    note=item.note,
                )
            )
    return checks


def runtime_array_bridge_gate_checks(rows: dict[str, BenchRow], policy: GatePolicy) -> list[GateCheck]:
    checks: list[GateCheck] = []
    for item in build_runtime_array_bridge_summary(rows):
        if item.bulk_hit_pct is not None:
            checks.append(
                GateCheck(
                    signal="q_array_bridge_bulk_hit_pct",
                    benchmark=item.scope,
                    value=item.bulk_hit_pct,
                    threshold=f">= {policy.min_q_array_bridge_bulk_hit_pct:g}",
                    status=(
                        "pass"
                        if item.bulk_hit_pct >= policy.min_q_array_bridge_bulk_hit_pct
                        else "fail"
                    ),
                    note=item.note,
                )
            )
        checks.append(
            GateCheck(
                signal="q_array_bridge_fallbacks_op",
                benchmark=item.scope,
                value=item.fallbacks_op,
                threshold=f"<= {policy.max_q_array_bridge_fallbacks_op:g}",
                status=(
                    "pass"
                    if item.fallbacks_op <= policy.max_q_array_bridge_fallbacks_op
                    else "fail"
                ),
                note=item.note,
            )
        )
        checks.append(
            GateCheck(
                signal="q_array_bridge_rows_op",
                benchmark=item.scope,
                value=item.rows_op,
                threshold=f">= {policy.min_q_array_bridge_rows_op:g}",
                status="pass" if item.rows_op >= policy.min_q_array_bridge_rows_op else "fail",
                note="array bridge counters must cover a non-empty data volume",
            )
        )
        if item.avg_allocs_op is not None:
            checks.append(
                GateCheck(
                    signal="q_array_bridge_avg_allocs_op",
                    benchmark=item.scope,
                    value=item.avg_allocs_op,
                    threshold=f"<= {policy.max_q_array_bridge_avg_allocs_op:g}",
                    status=(
                        "pass"
                        if item.avg_allocs_op <= policy.max_q_array_bridge_avg_allocs_op
                        else "fail"
                    ),
                    note=item.note,
                )
            )
        if item.max_allocs_op is not None:
            checks.append(
                GateCheck(
                    signal="q_array_bridge_max_allocs_op",
                    benchmark=item.scope,
                    value=item.max_allocs_op,
                    threshold=f"<= {policy.max_q_array_bridge_max_allocs_op:g}",
                    status=(
                        "pass"
                        if item.max_allocs_op <= policy.max_q_array_bridge_max_allocs_op
                        else "fail"
                    ),
                    note=item.note,
                )
            )
    return checks


def runtime_backend_route_gate_checks(rows: dict[str, BenchRow], policy: GatePolicy) -> list[GateCheck]:
    checks: list[GateCheck] = []
    summary = build_runtime_backend_route_summary(rows)
    if not summary:
        checks.extend(
            [
                GateCheck(
                    signal="runtime_backend_route_benchmarks",
                    benchmark="runtime_primitive_registry_and_frame_vector_routes",
                    value=0.0,
                    threshold=f">= {policy.min_runtime_backend_route_benchmarks:g}",
                    status="pass" if policy.min_runtime_backend_route_benchmarks <= 0 else "fail",
                    note=(
                        "runtime primitive registry or MethodJIT frame/vector route counters must be present "
                        "so backend statistics cannot silently disappear"
                    ),
                ),
                GateCheck(
                    signal="runtime_backend_route_hits_op",
                    benchmark="runtime_primitive_registry_and_frame_vector_routes",
                    value=0.0,
                    threshold=f">= {policy.min_runtime_backend_route_hits_op:g}",
                    status="pass" if policy.min_runtime_backend_route_hits_op <= 0 else "fail",
                    note="backend route counters are missing",
                ),
            ]
        )
        return checks
    for item in summary:
        checks.append(
            GateCheck(
                signal="runtime_backend_route_benchmarks",
                benchmark=item.scope,
                value=float(item.benchmark_count),
                threshold=f">= {policy.min_runtime_backend_route_benchmarks:g}",
                status="pass" if item.benchmark_count >= policy.min_runtime_backend_route_benchmarks else "fail",
                note=item.note,
            )
        )
        checks.append(
            GateCheck(
                signal="runtime_backend_route_hits_op",
                benchmark=item.scope,
                value=item.hits_op,
                threshold=f">= {policy.min_runtime_backend_route_hits_op:g}",
                status="pass" if item.hits_op >= policy.min_runtime_backend_route_hits_op else "fail",
                note=item.note,
            )
        )
        checks.append(
            GateCheck(
                signal="runtime_backend_route_errors_op",
                benchmark=item.scope,
                value=item.errors_op,
                threshold=f"<= {policy.max_runtime_backend_route_errors_op:g}",
                status="pass" if item.errors_op <= policy.max_runtime_backend_route_errors_op else "fail",
                note=item.note,
            )
        )
    return checks


def runtime_metric_contract_gate_checks(rows: dict[str, BenchRow], policy: GatePolicy) -> list[GateCheck]:
    checks: list[GateCheck] = []
    observability = {item.layer: item for item in build_runtime_observability_summary(rows)}
    bridge_summary = build_runtime_bridge_efficiency_summary(rows)
    bridge_count = bridge_summary[0].benchmark_count if bridge_summary else 0
    requirements = [
        (
            "runtime_contract_typed_primitive_benchmarks",
            "typed_primitive",
            float(observability.get("typed_primitive").benchmark_count if "typed_primitive" in observability else 0),
            float(policy.min_runtime_typed_primitive_benchmarks),
            "typed primitive counters must be present so typed kernel hit/fallback rates cannot silently disappear",
        ),
        (
            "runtime_contract_jit_backend_benchmarks",
            "jit_backend",
            float(observability.get("jit_backend").benchmark_count if "jit_backend" in observability else 0),
            float(policy.min_runtime_jit_backend_benchmarks),
            "JIT route counters must be present so direct-return versus slow exits remain observable",
        ),
        (
            "runtime_contract_array_bridge_benchmarks",
            "methodjit_array_bridge",
            float(observability.get("methodjit_array_bridge").benchmark_count if "methodjit_array_bridge" in observability else 0),
            float(policy.min_runtime_array_bridge_benchmarks),
            "array bridge counters must be present so bulk export regressions cannot be hidden",
        ),
        (
            "runtime_contract_bridge_benchmark_count",
            "typed_runtime_and_jit_backend",
            float(bridge_count),
            float(policy.min_runtime_bridge_benchmark_count),
            "runtime bridge efficiency should aggregate typed primitive, JIT backend, and array bridge rows",
        ),
    ]
    for signal, benchmark, value, threshold, note in requirements:
        checks.append(
            GateCheck(
                signal=signal,
                benchmark=benchmark,
                value=value,
                threshold=f">= {threshold:g}",
                status="pass" if value >= threshold else "fail",
                note=note,
            )
        )
    return checks


def qeval_family_coverage_gate_checks(rows: dict[str, BenchRow], policy: GatePolicy) -> list[GateCheck]:
    checks: list[GateCheck] = []
    threshold = policy.min_q_eval_family_cases
    for item in build_qeval_family_coverage(rows):
        checks.append(
            GateCheck(
                signal="q_eval_family_session_cases",
                benchmark=item.family,
                value=float(item.session_case_count),
                threshold=f">= {threshold:g}",
                status="pass" if item.session_case_count >= threshold else "fail",
                note=item.note,
            )
        )
        checks.append(
            GateCheck(
                signal="q_eval_family_go_baseline_cases",
                benchmark=item.family,
                value=float(item.matched_go_baseline_count),
                threshold=f">= {threshold:g}",
                status="pass" if item.matched_go_baseline_count >= threshold else "fail",
                note=(
                    "same-case hand-written Go baseline rows are required; "
                    f"missing={', '.join(item.missing_go_baseline[:5]) or 'none'}"
                ),
            )
        )
        checks.append(
            GateCheck(
                signal="q_eval_family_jit_cases",
                benchmark=item.family,
                value=float(item.matched_jit_case_count),
                threshold=f">= {threshold:g}",
                status="pass" if item.matched_jit_case_count >= threshold else "fail",
                note=(
                    "same-case BenchmarkQEvalJITScriptWarm rows are required; "
                    f"missing={', '.join(item.missing_jit_case[:5]) or 'none'}"
                ),
            )
        )
    return checks


def build_gate_checks(rows: dict[str, BenchRow], policy: GatePolicy, ratio_baseline: dict | None = None) -> list[GateCheck]:
    return (
        ratio_gate_checks(rows, policy, ratio_baseline)
        + ratio_baseline_gate_checks(rows, policy, ratio_baseline)
        + realdata_gate_checks(rows, policy, ratio_baseline)
        + runtime_gate_checks(rows, policy)
        + observability_gate_checks(rows, policy)
        + runtime_health_gate_checks(rows, policy)
        + runtime_bridge_efficiency_gate_checks(rows, policy)
        + runtime_array_bridge_gate_checks(rows, policy)
        + runtime_backend_route_gate_checks(rows, policy)
        + runtime_metric_contract_gate_checks(rows, policy)
        + qeval_family_coverage_gate_checks(rows, policy)
    )


def gate_failed(checks: list[GateCheck]) -> bool:
    return any(check.status == "fail" for check in checks)


def metric_present(rows: dict[str, BenchRow], names: list[str], metric: str) -> bool:
    return any(metric in rows.get(name, BenchRow(name, 0, 0)).metrics for name in names)


def build_coverage(rows: dict[str, BenchRow], current_vs_old: list[CurrentVsOldRow] | None = None) -> list[dict[str, str]]:
    current_vs_old = current_vs_old or []
    qsql_names = [name for name in rows if name.startswith("BenchmarkQSQL")]
    qeval_names = [name for name in rows if name.startswith("BenchmarkQEval") or name.startswith("BenchmarkQSessionEval")]
    qeval_has_kernel_metrics = metric_present(rows, qeval_names, "typed_kernel_hit_pct") or metric_present(rows, qeval_names, "kernel_hit_pct")
    qeval_has_fallback_metrics = metric_present(rows, qeval_names, "typed_kernel_fallbacks/op") or metric_present(rows, qeval_names, "fallbacks/op")
    return [
        {
            "signal": "current Leia vs old Leia",
            "qSQL": "covered" if current_vs_old else "missing",
            "q.eval": "covered" if current_vs_old else "missing",
            "gap": "" if current_vs_old else "provide --timing-json from leia bench compare or q_columnar_suite JSON output",
        },
        {
            "signal": "current Leia vs hand-written Go",
            "qSQL": "covered" if ratio(rows, "BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject", "BenchmarkQSQLNativeGoSelectWhereProject") is not None else "missing",
            "q.eval": "covered" if any(name.startswith("BenchmarkQEvalVectorGoBaseline/") for name in rows) else "missing",
            "gap": "",
        },
        {
            "signal": "warm run vs cold run",
            "qSQL": "covered" if ratio(rows, "BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject", "BenchmarkQSQLBindRunSQLColdCacheSelectWhereProject") is not None else "missing",
            "q.eval": "covered" if any(name.startswith("BenchmarkQEvalVectorCold/") for name in rows) else "missing",
            "gap": "qSQL cold coverage currently only exists for select/filter/project",
        },
        {
            "signal": "typed kernel hit rate",
            "qSQL": "covered" if metric_present(rows, qsql_names, "kernel_hit_pct") else "missing",
            "q.eval": "covered" if qeval_has_kernel_metrics else "missing",
            "gap": "" if qeval_has_kernel_metrics else "q.eval typed kernel execution is visible through q.cache_stats, but q.eval benchmarks do not yet emit per-op typed kernel metrics",
        },
        {
            "signal": "fallback rate",
            "qSQL": "covered" if metric_present(rows, qsql_names, "fallbacks/op") else "missing",
            "q.eval": "covered" if qeval_has_fallback_metrics else "missing",
            "gap": "" if qeval_has_fallback_metrics else "q.eval benchmarks do not yet emit per-op fallbacks/op; q.cache_stats has execution counters only",
        },
        {
            "signal": "allocs/op",
            "qSQL": "covered" if metric_present(rows, qsql_names, "allocs/op") else "missing",
            "q.eval": "covered" if metric_present(rows, qeval_names, "allocs/op") else "missing",
            "gap": "",
        },
    ]


def format_float(value: float | None) -> str:
    if value is None:
        return "missing"
    return f"{value:.3f}x"


def format_seconds(value: float | None) -> str:
    if value is None:
        return "missing"
    return f"{value:.6f}s"


def format_metric(value: float | None, digits: int) -> str:
    if value is None:
        return "missing"
    return f"{value:.{digits}f}"


def markdown_report(
    rows: dict[str, BenchRow],
    commands: list[CommandResult],
    current_vs_old: list[CurrentVsOldRow] | None = None,
    gate_checks: list[GateCheck] | None = None,
    pipeline_fallback_top: list[PipelineFallbackTopRow] | None = None,
) -> str:
    current_vs_old = current_vs_old or []
    gate_checks = gate_checks or []
    pipeline_fallback_top = pipeline_fallback_top or []
    coverage = build_coverage(rows, current_vs_old)
    qsql_coverage = build_qsql_benchmark_coverage(rows)
    qeval_compute = build_qeval_compute_coverage(rows)
    qeval_family = build_qeval_family_coverage(rows)
    ratios = build_ratios(rows)
    qeval_diagnostics = build_qeval_case_diagnostics(rows)
    runtime_metrics = build_runtime_metric_rows(rows)
    fallback_shapes = build_fallback_shape_rows(rows)
    jit_routes = build_jit_route_summary(rows)
    observability = build_runtime_observability_summary(rows)
    health = build_runtime_health_summary(rows)
    bridge_efficiency = build_runtime_bridge_efficiency_summary(rows)
    array_bridge = build_runtime_array_bridge_summary(rows)
    backend_routes = build_runtime_backend_route_summary(rows)
    category_metrics = build_pipeline_category_metric_rows(rows)
    lines = [
        "# q Performance Completeness Report",
        "",
        "## Commands",
        "",
    ]
    for result in commands:
        status = "ok" if result.exit_code == 0 else f"exit {result.exit_code}"
        lines.append(f"- `{result.label}` ({status}): `{' '.join(result.cmd)}`")
    lines.extend(
        [
            "",
            "## Coverage Matrix",
            "",
            "| Signal | qSQL | q.eval / ordinary q | Gap |",
            "|---|---|---|---|",
        ]
    )
    for item in coverage:
        lines.append(f"| {item['signal']} | {item['qSQL']} | {item['q.eval']} | {item['gap']} |")
    lines.extend(
        [
            "",
            "## Current vs Old Leia",
            "",
            "| Benchmark | Mode | Current | Old | Current/Old | Source |",
            "|---|---|---:|---:|---:|---|",
        ]
    )
    if current_vs_old:
        for item in current_vs_old:
            lines.append(
                f"| {item.benchmark} | {item.mode} | {format_seconds(item.current_seconds)} | "
                f"{format_seconds(item.old_seconds)} | {format_float(item.ratio)} | {item.source} |"
            )
    else:
        lines.append("| missing | missing | missing | missing | missing | provide `--timing-json` |")
    lines.extend(
        [
            "",
            "## Gate Summary",
            "",
            "| Status | Signal | Benchmark | Value | Threshold | Note |",
            "|---|---|---|---:|---:|---|",
        ]
    )
    if gate_checks:
        for item in gate_checks:
            value = "missing" if item.value is None else f"{item.value:.3f}"
            lines.append(
                f"| {item.status} | {item.signal} | {item.benchmark} | "
                f"{value} | {item.threshold} | {item.note} |"
            )
    else:
        lines.append("| not-run | missing | missing | missing | run with `--check` to enforce thresholds |  |")
    lines.extend(
        [
            "",
            "## qSQL Benchmark Coverage",
            "",
            "| Signal | Count |",
            "|---|---:|",
            f"| Leia bind cases | {qsql_coverage.leia_case_count} |",
            f"| native Go baseline cases | {qsql_coverage.native_go_case_count} |",
            f"| data runtime direct cases | {qsql_coverage.data_runtime_case_count} |",
            f"| expected qSQL benchmark rows | {qsql_coverage.expected_case_count} |",
            f"| expected qSQL benchmark rows present | {qsql_coverage.matched_expected_count} |",
            "",
            "### Missing qSQL benchmark rows",
            "",
        ]
    )
    if qsql_coverage.missing_expected:
        lines.extend(f"- `{item}`" for item in qsql_coverage.missing_expected)
    else:
        lines.append("- none")
    lines.extend(
        [
            "",
            "## Ordinary q Compute Coverage",
            "",
            "| Signal | Count |",
            "|---|---:|",
            f"| session execution cases | {qeval_compute.session_case_count} |",
            f"| Go baseline cases | {qeval_compute.go_baseline_case_count} |",
            f"| trusted Go performance baselines | {qeval_compute.trusted_go_baseline_count} |",
            f"| untrusted Go correctness-only baselines | {qeval_compute.untrusted_go_baseline_count} |",
            f"| result-cache warm cases | {qeval_compute.result_cache_warm_case_count} |",
            f"| cold cases | {qeval_compute.cold_case_count} |",
            f"| session cases matched with Go baseline | {qeval_compute.matched_go_baseline_count} |",
            f"| session cases matched with result-cache warm | {qeval_compute.matched_result_cache_warm_count} |",
            f"| session cases matched with cold | {qeval_compute.matched_cold_count} |",
        ]
    )
    missing_sections = [
        ("Missing Go baseline", qeval_compute.missing_go_baseline),
        ("Missing result-cache warm", qeval_compute.missing_result_cache_warm),
        ("Missing cold", qeval_compute.missing_cold),
        ("Go baselines without session execution", qeval_compute.orphan_go_baseline),
        ("Untrusted Go correctness-only baselines", qeval_compute.untrusted_go_baselines),
    ]
    for title, items in missing_sections:
        lines.extend(["", f"### {title}", ""])
        if items:
            lines.extend(f"- `{item}`" for item in items)
        else:
            lines.append("- none")
    lines.extend(
        [
            "",
            "## Ordinary q Family Coverage",
            "",
            "| Family | Session cases | Go baselines | JIT cases | matched Go | matched JIT | Note |",
            "|---|---:|---:|---:|---:|---:|---|",
        ]
    )
    for item in qeval_family:
        lines.append(
            f"| {item.family} | {item.session_case_count} | "
            f"{item.go_baseline_case_count} | {item.jit_case_count} | "
            f"{item.matched_go_baseline_count} | {item.matched_jit_case_count} | "
            f"{item.note} |"
        )
    for item in qeval_family:
        if item.missing_go_baseline:
            lines.extend(["", f"### Missing Go baselines for {item.family}", ""])
            lines.extend(f"- `{case}`" for case in item.missing_go_baseline)
        if item.missing_jit_case:
            lines.extend(["", f"### Missing JIT rows for {item.family}", ""])
            lines.extend(f"- `{case}`" for case in item.missing_jit_case)
    lines.extend(
        [
            "",
            "## Ratios",
            "",
            "`ratio < 1.0x` means Leia is faster than the hand-written Go or cold denominator.",
            "",
            "| Scenario | Leia benchmark | Denominator | Ratio | Note |",
            "|---|---|---|---:|---|",
        ]
    )
    for item in ratios:
        lines.append(
            f"| {item.scenario} | {item.numerator} | {item.denominator} | "
            f"{format_float(item.ratio)} | {item.note} |"
        )
    realdata_rows = build_qeval_realdata_rows(rows)
    if realdata_rows:
        realdata_geo, realdata_trusted, realdata_total = realdata_trusted_geomean(rows)
        lines.extend(
            [
                "",
                "## Real-Data Annex (env-injected dense columns)",
                "",
                "Data is injected via the eval environment as dense Go-built columns, so",
                "const-memo and lazy-carrier closed forms cannot fire. This family measures",
                "real-data kernel quality and is reported separately from the synthetic",
                "suite geomeans (the synthetic suite measures warm dispatch and lazy-carrier",
                "regression; mixing the two would hide both signals).",
                "",
                f"Trusted realdata geomean: {format_float(realdata_geo)} over {realdata_trusted}/{realdata_total} cases (separate family; not folded into synthetic geomeans).",
                "",
                "| Case | Warm ns/op | Go ns/op | Warm/Go | warm allocs/op | Note |",
                "|---|---:|---:|---:|---:|---|",
            ]
        )
        for item in realdata_rows:
            lines.append(
                f"| {item.case} | "
                f"{format_metric(item.warm_ns_op, 0)} | "
                f"{format_metric(item.go_ns_op, 0)} | "
                f"{format_float(item.realdata_go_ratio)} | "
                f"{format_metric(item.warm_allocs_op, 0)} | "
                f"{item.note} |"
            )
    lines.extend(
        [
            "",
            "## q.eval Case Diagnostics",
            "",
            "One row per ordinary q case, joining Leia warm/cold/JIT, Go baseline, typed runtime route, JIT route, and allocation signals.",
            "",
            "| Case | Pressure | Go ns/op | Session ns/op | Session/Go | Cold/Session | JIT ns/op | JIT/Go | Typed hit pct | typed fallbacks/op | pipeline fallback shapes | JIT direct/op | JIT slow route pct | session planned/op | session shell/op | session errors/op | session planned pct | session backend shapes | session allocs/op | JIT allocs/op | Note |",
            "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|",
        ]
    )
    if qeval_diagnostics:
        for item in qeval_diagnostics:
            lines.append(
                f"| {item.case} | {item.primary_pressure} | "
                f"{format_metric(item.go_baseline_ns_op, 0)} | "
                f"{format_metric(item.session_ns_op, 0)} | "
                f"{format_float(item.session_go_ratio)} | "
                f"{format_float(item.cold_session_ratio)} | "
                f"{format_metric(item.jit_warm_ns_op, 0)} | "
                f"{format_float(item.jit_go_ratio)} | "
                f"{format_metric(item.typed_hit_pct, 1)} | "
                f"{format_metric(item.typed_fallbacks_op, 3)} | "
                f"{format_metric(item.typed_pipeline_fallback_shapes, 0)} | "
                f"{format_metric(item.jit_direct_return_op, 3)} | "
                f"{format_metric(item.jit_backend_slow_route_pct, 1)} | "
                f"{format_metric(item.q_session_planned_op_exit_op, 3)} | "
                f"{format_metric(item.q_session_shell_fallback_op, 3)} | "
                f"{format_metric(item.q_session_eval_errors_op, 3)} | "
                f"{format_metric(item.q_session_planned_route_pct, 1)} | "
                f"{format_metric(item.q_session_backend_shapes, 0)} | "
                f"{format_metric(item.session_allocs_op, 0)} | "
                f"{format_metric(item.jit_warm_allocs_op, 0)} | "
                f"{item.note} |"
            )
    else:
        lines.append("| missing | missing | missing | missing | missing | missing | missing | missing | missing | missing | missing | missing | missing | missing | missing | missing | missing | missing | missing | missing | missing | no q.eval rows parsed |")
    lines.extend(
        [
            "",
            "## Runtime Metrics",
            "",
            "| Benchmark | ns/op | B/op | allocs/op | kernel_hit_pct | fallbacks/op | typed_kernel_hit_pct | typed_kernel_attempts/op | typed_kernel_fallbacks/op | typed_kernel_errors/op | typed_pipeline_shapes | typed_pipeline_fallback_shapes | data_runtime_hit_pct | data_runtime_attempts/op | data_runtime_fallbacks/op | data_runtime_errors/op | data_runtime_pipeline_shapes | linalg_vector_hits/op | linalg_matrix_hits/op | jit_typed_direct_return/op | jit_typed_native_exit/op | jit_typed_op_exit/op | jit_typed_kernel_success/op | jit_typed_kernel_errors/op | jit_typed_pipeline_shapes | q_session_planned_op_exit/op | q_session_shell_fallback/op | q_session_eval_errors/op | q_session_backend_shapes |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
        ]
    )
    for item in runtime_metrics:
        lines.append(
            f"| {item.benchmark} | {item.ns_op:.0f} | "
            f"{format_metric(item.bytes_op, 0)} | "
            f"{format_metric(item.allocs_op, 0)} | "
            f"{format_metric(item.kernel_hit_pct, 1)} | "
            f"{format_metric(item.fallbacks_op, 3)} | "
            f"{format_metric(item.typed_kernel_hit_pct, 1)} | "
            f"{format_metric(item.typed_kernel_attempts_op, 3)} | "
            f"{format_metric(item.typed_kernel_fallbacks_op, 3)} | "
            f"{format_metric(item.typed_kernel_errors_op, 3)} | "
            f"{format_metric(item.typed_pipeline_shapes, 0)} | "
            f"{format_metric(item.typed_pipeline_fallback_shapes, 0)} | "
            f"{format_metric(item.data_runtime_hit_pct, 1)} | "
            f"{format_metric(item.data_runtime_attempts_op, 3)} | "
            f"{format_metric(item.data_runtime_fallbacks_op, 3)} | "
            f"{format_metric(item.data_runtime_errors_op, 3)} | "
            f"{format_metric(item.data_runtime_pipeline_shapes, 0)} | "
            f"{format_metric(item.linalg_vector_hits_op, 3)} | "
            f"{format_metric(item.linalg_matrix_hits_op, 3)} | "
            f"{format_metric(item.jit_typed_direct_return_op, 3)} | "
            f"{format_metric(item.jit_typed_native_exit_op, 3)} | "
            f"{format_metric(item.jit_typed_op_exit_op, 3)} | "
            f"{format_metric(item.jit_typed_kernel_success_op, 3)} | "
            f"{format_metric(item.jit_typed_kernel_errors_op, 3)} | "
            f"{format_metric(item.jit_typed_pipeline_shapes, 0)} | "
            f"{format_metric(item.q_session_planned_op_exit_op, 3)} | "
            f"{format_metric(item.q_session_shell_fallback_op, 3)} | "
            f"{format_metric(item.q_session_eval_errors_op, 3)} | "
            f"{format_metric(item.q_session_backend_shapes, 0)} |"
        )
    if any(item.benchmark_count > 0 for item in jit_routes):
        lines.extend(
            [
                "",
                "## JIT Typed Runtime Routes",
                "",
                "| Route | calls/op | route share | benchmarks |",
                "|---|---:|---:|---:|",
            ]
        )
        for item in jit_routes:
            lines.append(
                f"| {item.route} | {item.calls_per_op:.3f} | "
                f"{item.share_pct:.1f}% | {item.benchmark_count} |"
            )
    lines.extend(
        [
            "",
            "## Runtime Observability Summary",
            "",
            "| Layer | Benchmarks | attempts/op | hits/op | fallbacks/op | errors/op | hit pct | shapes | fallback shapes | direct return/op | native exit/op | op exit/op | slow route pct | Note |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|",
        ]
    )
    if observability:
        for item in observability:
            lines.append(
                f"| {item.layer} | {item.benchmark_count} | "
                f"{format_metric(item.attempts_op, 3)} | "
                f"{format_metric(item.hits_op, 3)} | "
                f"{format_metric(item.fallbacks_op, 3)} | "
                f"{format_metric(item.errors_op, 3)} | "
                f"{format_metric(item.hit_pct, 1)} | "
                f"{format_metric(item.shapes, 0)} | "
                f"{format_metric(item.fallback_shapes, 0)} | "
                f"{format_metric(item.direct_return_op, 3)} | "
                f"{format_metric(item.native_exit_op, 3)} | "
                f"{format_metric(item.op_exit_op, 3)} | "
                f"{format_metric(item.slow_route_pct, 1)} | "
                f"{item.note} |"
            )
    else:
        lines.append("| missing | 0 | missing | missing | missing | missing | missing | missing | missing | missing | missing | missing | missing | no runtime observability metrics parsed |")
    lines.extend(
        [
            "",
            "## Runtime Health Summary",
            "",
            "| Scope | Benchmarks | avg allocs/op | max allocs/op | typed fallbacks/op | typed errors/op | pipeline fallback shapes | direct return/op | native exit/op | op exit/op | JIT slow route pct | Note |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|",
        ]
    )
    if health:
        for item in health:
            lines.append(
                f"| {item.scope} | {item.benchmark_count} | "
                f"{format_metric(item.avg_allocs_op, 1)} | "
                f"{format_metric(item.max_allocs_op, 1)} | "
                f"{item.typed_fallbacks_op:.3f} | "
                f"{item.typed_errors_op:.3f} | "
                f"{item.pipeline_fallback_shapes:.0f} | "
                f"{item.jit_direct_return_op:.3f} | "
                f"{item.jit_native_exit_op:.3f} | "
                f"{item.jit_op_exit_op:.3f} | "
                f"{format_metric(item.jit_slow_route_pct, 1)} | "
                f"{item.note} |"
            )
    else:
        lines.append("| missing | 0 | missing | missing | 0 | 0 | 0 | 0 | 0 | 0 | missing | no typed runtime or JIT route metrics parsed |")
    lines.extend(
        [
            "",
            "## Runtime Bridge Efficiency",
            "",
            "| Scope | Benchmarks | direct calls/op | slow bridge calls/op | direct call share | avg allocs/op | allocs/direct call | Note |",
            "|---|---:|---:|---:|---:|---:|---:|---|",
        ]
    )
    if bridge_efficiency:
        for item in bridge_efficiency:
            lines.append(
                f"| {item.scope} | {item.benchmark_count} | "
                f"{item.direct_calls_op:.3f} | "
                f"{item.slow_bridge_calls_op:.3f} | "
                f"{format_metric(item.direct_call_share_pct, 1)} | "
                f"{format_metric(item.avg_allocs_op, 1)} | "
                f"{format_metric(item.allocs_per_direct_call, 3)} | "
                f"{item.note} |"
            )
    else:
        lines.append("| missing | 0 | 0 | 0 | missing | missing | missing | no typed runtime or JIT bridge metrics parsed |")
    lines.extend(
        [
            "",
            "## Runtime Array Bridge Summary",
            "",
            "| Scope | Benchmarks | attempts/op | bulk hits/op | fallbacks/op | errors/op | bulk hit pct | rows/op | avg allocs/op | max allocs/op | Note |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|",
        ]
    )
    if array_bridge:
        for item in array_bridge:
            lines.append(
                f"| {item.scope} | {item.benchmark_count} | "
                f"{item.attempts_op:.3f} | "
                f"{item.bulk_hits_op:.3f} | "
                f"{item.fallbacks_op:.3f} | "
                f"{item.errors_op:.3f} | "
                f"{format_metric(item.bulk_hit_pct, 1)} | "
                f"{format_metric(item.rows_op, 0)} | "
                f"{format_metric(item.avg_allocs_op, 1)} | "
                f"{format_metric(item.max_allocs_op, 1)} | "
                f"{item.note} |"
            )
    else:
        lines.append("| missing | 0 | 0 | 0 | 0 | 0 | missing | 0 | missing | missing | no q array bridge metrics parsed |")
    lines.extend(
        [
            "",
            "## Runtime Primitive Registry Routes",
            "",
            "| Scope | Benchmarks | registry benchmarks | MethodJIT frame/vector benchmarks | hits/op | errors/op | direct helper/op | native exit/op | op-exit/op | hit pct | Note |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|",
        ]
    )
    if backend_routes:
        for item in backend_routes:
            lines.append(
                f"| {item.scope} | {item.benchmark_count} | "
                f"{item.registry_benchmark_count} | "
                f"{item.methodjit_frame_vector_benchmark_count} | "
                f"{item.hits_op:.3f} | "
                f"{item.errors_op:.3f} | "
                f"{item.direct_helper_op:.3f} | "
                f"{item.native_exit_op:.3f} | "
                f"{item.op_exit_op:.3f} | "
                f"{format_metric(item.hit_pct, 1)} | "
                f"{item.note} |"
            )
    else:
        lines.append("| missing | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | missing | no runtime primitive registry or MethodJIT frame/vector route metrics parsed |")
    lines.extend(
        [
            "",
            "## Fallback Shape Summary",
            "",
            "| Benchmark | fallbacks/op | typed_kernel_fallbacks/op | typed_pipeline_fallback_shapes |",
            "|---|---:|---:|---:|",
        ]
    )
    if fallback_shapes:
        for item in fallback_shapes:
            lines.append(
                f"| {item.benchmark} | "
                f"{format_metric(item.fallbacks_op, 3)} | "
                f"{format_metric(item.typed_kernel_fallbacks_op, 3)} | "
                f"{format_metric(item.typed_pipeline_fallback_shapes, 0)} |"
            )
    else:
        lines.append("| none | 0 | 0 | 0 |")
    lines.extend(
        [
            "",
            "## Pipeline Category Metrics",
            "",
            "| Category | Benchmarks | avg ns/op | avg B/op | avg allocs/op | avg typed hit pct | total fallbacks/op | total fallback shapes |",
            "|---|---:|---:|---:|---:|---:|---:|---:|",
        ]
    )
    if category_metrics:
        for item in category_metrics:
            lines.append(
                f"| {item.category} | {item.benchmark_count} | "
                f"{format_metric(item.avg_ns_op, 0)} | "
                f"{format_metric(item.avg_bytes_op, 0)} | "
                f"{format_metric(item.avg_allocs_op, 1)} | "
                f"{format_metric(item.avg_typed_hit_pct, 1)} | "
                f"{item.total_fallbacks_op:.3f} | "
                f"{item.total_fallback_shapes:.0f} |"
            )
    else:
        lines.append("| missing | 0 | missing | missing | missing | missing | 0 | 0 |")
    lines.extend(
        [
            "",
            "## Pipeline Fallback Top-N",
            "",
            "Rows come from `go test ./benchmarks -run TestQEvalVectorRuntimeFallbackReport -v` output.",
            "",
            "| Category | Pipeline shape | Kernel | Reason | Outcome | Count |",
            "|---|---|---|---|---|---:|",
        ]
    )
    if pipeline_fallback_top:
        for item in pipeline_fallback_top:
            lines.append(
                f"| {item.category} | {item.pipeline_shape} | {item.kernel} | "
                f"{item.reason} | {item.outcome} | {item.count} |"
            )
    else:
        lines.append("| missing | missing | missing | missing | missing | 0 |")
    lines.extend(
        [
            "",
            "## Raw Benchmarks",
            "",
            "| Benchmark | ns/op | B/op | allocs/op | kernel_hit_pct | fallbacks/op | typed_kernel_hit_pct | typed_kernel_fallbacks/op | typed_pipeline_shapes | typed_pipeline_fallback_shapes |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
        ]
    )
    for name in sorted(rows):
        row = rows[name]
        lines.append(
            f"| {name} | {row.ns_op:.0f} | "
            f"{row.metrics.get('B/op', 0):.0f} | "
            f"{row.metrics.get('allocs/op', 0):.0f} | "
            f"{row.metrics.get('kernel_hit_pct', 0):.1f} | "
            f"{row.metrics.get('fallbacks/op', 0):.3f} | "
            f"{row.metrics.get('typed_kernel_hit_pct', 0):.1f} | "
            f"{row.metrics.get('typed_kernel_fallbacks/op', 0):.3f} | "
            f"{row.metrics.get('typed_pipeline_shapes', 0):.0f} | "
            f"{row.metrics.get('typed_pipeline_fallback_shapes', 0):.0f} |"
        )
    lines.extend(
        [
            "",
            "## Required Follow-up Gaps",
            "",
            "- Add stable q.eval math-map coverage once unary/vector math expressions such as exp/log/sqrt have complete parser/eval support.",
            "- Add qSQL cold-cache counterparts for group and join if those paths are used to judge schema-stable cache value.",
            "- Pass `--timing-json` from `leia bench compare` / `q_columnar_suite.sh --json ...` when this report is used for current-vs-old decisions.",
        ]
    )
    return "\n".join(lines) + "\n"


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--benchtime", default="100x")
    parser.add_argument("--json", type=Path, default=Path("benchmarks/data/q_perf_report_latest.json"))
    parser.add_argument("--markdown", type=Path, default=Path("benchmarks/data/q_perf_report_latest.md"))
    parser.add_argument("--from-output", type=Path, action="append", default=[], help="Parse existing go test output instead of running commands.")
    parser.add_argument("--timing-json", type=Path, action="append", default=[], help="Include current-vs-old rows from timing_compare.py JSON output.")
    parser.add_argument("--check", action="store_true", help="Fail if q benchmark ratios or runtime metrics miss the configured thresholds.")
    parser.add_argument(
        "--max-leia-go-ratio",
        type=float,
        default=5.0,
        help="Hard cap for warm session-execution vs Go baseline ratios. " + MILESTONE_CAP_HELP % "max_leia_go_ratio",
    )
    parser.add_argument(
        "--max-leia-jit-go-ratio",
        type=float,
        default=5.0,
        help=(
            "Hard cap for BenchmarkQEvalJITScriptWarm vs BenchmarkQEvalVectorGoBaseline ratios. "
            + MILESTONE_CAP_HELP % "max_leia_jit_go_ratio"
        ),
    )
    parser.add_argument(
        "--max-leia-realdata-go-ratio",
        type=float,
        default=60.0,
        help=(
            "Hard cap for BenchmarkQEvalRealDataWarm vs BenchmarkQEvalRealDataGoBaseline ratios "
            "(real-data annex; gated separately from synthetic families). "
            + MILESTONE_CAP_HELP % "max_leia_realdata_go_ratio"
        ),
    )
    parser.add_argument(
        "--ratio-baseline",
        type=Path,
        default=DEFAULT_RATIO_BASELINE_PATH,
        help="Ratchet baseline JSON with per-case trusted Leia-vs-Go ratios, exceptions, and family targets.",
    )
    parser.add_argument(
        "--update-ratio-baseline",
        action="store_true",
        help="Recompute all trusted Leia-vs-Go ratios from the bench input and rewrite the ratio baseline JSON.",
    )
    parser.add_argument(
        "--min-typed-hit-pct",
        type=float,
        default=95.0,
        help="Minimum typed kernel hit percentage. " + MILESTONE_CAP_HELP % "min_typed_hit_pct",
    )
    parser.add_argument(
        "--max-typed-fallbacks-op",
        type=float,
        default=0.0,
        help="Hard cap for typed kernel fallbacks per op. " + MILESTONE_CAP_HELP % "max_typed_fallbacks_op",
    )
    parser.add_argument(
        "--max-pipeline-fallback-shapes",
        type=float,
        default=0.0,
        help="Hard cap for typed pipeline fallback shapes. " + MILESTONE_CAP_HELP % "max_pipeline_fallback_shapes",
    )
    parser.add_argument(
        "--max-allocs-op",
        type=float,
        default=64.0,
        help="Hard cap for allocs/op on q runtime benchmarks. " + MILESTONE_CAP_HELP % "max_allocs_op",
    )
    parser.add_argument("--max-jit-typed-errors-op", type=float, default=0.0)
    parser.add_argument("--max-jit-backend-slow-route-pct", type=float, default=0.0)
    parser.add_argument(
        "--min-q-session-planned-op-exit-op",
        type=float,
        default=0.9,
        help=(
            "Minimum planned q.session eval op-exit calls per JIT-script op. "
            + MILESTONE_CAP_HELP % "min_q_session_planned_op_exit_op"
        ),
    )
    parser.add_argument("--min-runtime-direct-bridge-share-pct", type=float, default=95.0)
    parser.add_argument("--max-runtime-allocs-per-direct-call", type=float, default=32.0)
    parser.add_argument("--min-q-array-bridge-bulk-hit-pct", type=float, default=95.0)
    parser.add_argument("--max-q-array-bridge-fallbacks-op", type=float, default=0.0)
    parser.add_argument("--min-runtime-typed-primitive-benchmarks", type=int, default=1)
    parser.add_argument(
        "--min-runtime-jit-backend-benchmarks",
        type=int,
        default=1,
        help=(
            "Minimum benchmarks emitting JIT backend route counters. "
            + MILESTONE_CAP_HELP % "min_runtime_jit_backend_benchmarks"
        ),
    )
    parser.add_argument(
        "--min-runtime-array-bridge-benchmarks",
        type=int,
        default=1,
        help=(
            "Minimum benchmarks emitting q array bridge counters. "
            + MILESTONE_CAP_HELP % "min_runtime_array_bridge_benchmarks"
        ),
    )
    parser.add_argument("--min-runtime-bridge-benchmark-count", type=int, default=3)
    parser.add_argument("--min-q-array-bridge-rows-op", type=float, default=1.0)
    parser.add_argument("--max-q-array-bridge-avg-allocs-op", type=float, default=64.0)
    parser.add_argument("--max-q-array-bridge-max-allocs-op", type=float, default=64.0)
    parser.add_argument(
        "--min-runtime-backend-route-benchmarks",
        type=int,
        default=1,
        help=(
            "Minimum benchmarks emitting runtime primitive registry or MethodJIT frame/vector route counters. "
            + MILESTONE_CAP_HELP % "min_runtime_backend_route_benchmarks"
        ),
    )
    parser.add_argument("--min-runtime-backend-route-hits-op", type=float, default=1.0)
    parser.add_argument("--max-runtime-backend-route-errors-op", type=float, default=0.0)
    parser.add_argument(
        "--min-q-eval-family-cases",
        type=int,
        default=1,
        help=(
            "Minimum same-family ordinary q rows required for session, Go baseline, and JIT script coverage. "
            + MILESTONE_CAP_HELP % "min_q_eval_family_cases"
        ),
    )
    parser.add_argument("--fallback-top-n", type=int, default=20)
    args = parser.parse_args(argv)

    commands: list[CommandResult] = []
    rows: dict[str, BenchRow] = {}
    current_vs_old: list[CurrentVsOldRow] = []
    pipeline_fallback_rows: list[PipelineFallbackTopRow] = []

    if args.from_output:
        for path in args.from_output:
            output = path.read_text()
            parsed = parse_go_benchmarks(output)
            rows.update(parsed)
            pipeline_fallback_rows.extend(parse_q_pipeline_fallback_reports(output))
            commands.append(
                CommandResult(
                    label=f"from-output:{path}",
                    cmd=["cat", str(path)],
                    exit_code=0,
                    output=output,
                    parsed_benchmark_count=len(parsed),
                )
            )
    else:
        qsql = run_command(
            "qsql-bind-native",
            ["go", "test", "./internal/stdlib/bind", "-run", "^$", "-bench", QSQL_BENCH, "-benchmem", f"-benchtime={args.benchtime}"],
        )
        qsql_rows = parse_go_benchmarks(qsql.output)
        qsql.parsed_benchmark_count = len(qsql_rows)
        commands.append(qsql)
        rows.update(qsql_rows)
        pipeline_fallback_rows.extend(parse_q_pipeline_fallback_reports(qsql.output))

        qeval = run_command(
            "qeval-native",
            ["go", "test", "./benchmarks", "-run", "^$", "-bench", QEVAL_BENCH, "-benchmem", f"-benchtime={args.benchtime}"],
        )
        qeval_rows = parse_go_benchmarks(qeval.output)
        qeval.parsed_benchmark_count = len(qeval_rows)
        commands.append(qeval)
        rows.update(qeval_rows)
        pipeline_fallback_rows.extend(parse_q_pipeline_fallback_reports(qeval.output))

        for label, bench in QJIT_BENCHES:
            qjit = run_command(
                label,
                ["go", "test", "./internal/methodjit", "-run", "^$", "-bench", bench, "-benchmem", f"-benchtime={args.benchtime}"],
            )
            qjit_rows = parse_go_benchmarks(qjit.output)
            qjit.parsed_benchmark_count = len(qjit_rows)
            commands.append(qjit)
            rows.update(qjit_rows)
            pipeline_fallback_rows.extend(parse_q_pipeline_fallback_reports(qjit.output))

    for path in args.timing_json:
        current_vs_old.extend(parse_timing_compare_json(path))

    ratio_baseline = load_ratio_baseline(args.ratio_baseline)
    if args.update_ratio_baseline:
        if not any(name.startswith("BenchmarkQEvalVectorGoBaseline/") for name in rows):
            print(
                "--update-ratio-baseline requires BenchmarkQEvalVectorGoBaseline rows in the bench input",
                file=sys.stderr,
            )
            return 1
        write_ratio_baseline(args.ratio_baseline, build_ratio_baseline_payload(rows, ratio_baseline))
        print(f"wrote {args.ratio_baseline}")

    apply_milestone_caps(args, parser, ratio_baseline, argv)
    policy = GatePolicy(
        max_leia_go_ratio=args.max_leia_go_ratio,
        max_leia_jit_go_ratio=args.max_leia_jit_go_ratio,
        max_leia_realdata_go_ratio=args.max_leia_realdata_go_ratio,
        min_typed_hit_pct=args.min_typed_hit_pct,
        max_typed_fallbacks_op=args.max_typed_fallbacks_op,
        max_pipeline_fallback_shapes=args.max_pipeline_fallback_shapes,
        max_allocs_op=args.max_allocs_op,
        max_jit_typed_errors_op=args.max_jit_typed_errors_op,
        max_jit_backend_slow_route_pct=args.max_jit_backend_slow_route_pct,
        min_runtime_direct_bridge_share_pct=args.min_runtime_direct_bridge_share_pct,
        max_runtime_allocs_per_direct_call=args.max_runtime_allocs_per_direct_call,
        min_q_array_bridge_bulk_hit_pct=args.min_q_array_bridge_bulk_hit_pct,
        max_q_array_bridge_fallbacks_op=args.max_q_array_bridge_fallbacks_op,
        min_runtime_typed_primitive_benchmarks=args.min_runtime_typed_primitive_benchmarks,
        min_runtime_jit_backend_benchmarks=args.min_runtime_jit_backend_benchmarks,
        min_runtime_array_bridge_benchmarks=args.min_runtime_array_bridge_benchmarks,
        min_runtime_bridge_benchmark_count=args.min_runtime_bridge_benchmark_count,
        min_q_array_bridge_rows_op=args.min_q_array_bridge_rows_op,
        max_q_array_bridge_avg_allocs_op=args.max_q_array_bridge_avg_allocs_op,
        max_q_array_bridge_max_allocs_op=args.max_q_array_bridge_max_allocs_op,
        min_runtime_backend_route_benchmarks=args.min_runtime_backend_route_benchmarks,
        min_runtime_backend_route_hits_op=args.min_runtime_backend_route_hits_op,
        max_runtime_backend_route_errors_op=args.max_runtime_backend_route_errors_op,
        min_q_eval_family_cases=args.min_q_eval_family_cases,
        min_q_session_planned_op_exit_op=args.min_q_session_planned_op_exit_op,
    )
    gate_checks = build_gate_checks(rows, policy, ratio_baseline) if args.check else []
    pipeline_fallback_rows.sort(key=lambda row: (-row.count, row.category, row.pipeline_shape, row.kernel, row.reason, row.outcome))
    if args.fallback_top_n >= 0:
        pipeline_fallback_rows = pipeline_fallback_rows[: args.fallback_top_n]
    payload = {
        "commands": [asdict(command) for command in commands],
        "benchmarks": {name: asdict(row) for name, row in sorted(rows.items())},
        "coverage": build_coverage(rows, current_vs_old),
        "current_vs_old": [asdict(row) for row in current_vs_old],
        "runtime_metrics": [asdict(row) for row in build_runtime_metric_rows(rows)],
        "jit_route_summary": [asdict(row) for row in build_jit_route_summary(rows)],
        "runtime_observability_summary": [asdict(row) for row in build_runtime_observability_summary(rows)],
        "runtime_health_summary": [asdict(row) for row in build_runtime_health_summary(rows)],
        "runtime_bridge_efficiency_summary": [asdict(row) for row in build_runtime_bridge_efficiency_summary(rows)],
        "runtime_array_bridge_summary": [asdict(row) for row in build_runtime_array_bridge_summary(rows)],
        "runtime_backend_route_summary": [asdict(row) for row in build_runtime_backend_route_summary(rows)],
        "pipeline_category_metrics": [asdict(row) for row in build_pipeline_category_metric_rows(rows)],
        "pipeline_fallback_top": [asdict(row) for row in pipeline_fallback_rows],
        "qsql_benchmark_coverage": asdict(build_qsql_benchmark_coverage(rows)),
        "q_eval_compute_coverage": asdict(build_qeval_compute_coverage(rows)),
        "q_eval_family_coverage": [asdict(row) for row in build_qeval_family_coverage(rows)],
        "q_eval_case_diagnostics": [asdict(row) for row in build_qeval_case_diagnostics(rows)],
        "q_eval_realdata": [asdict(row) for row in build_qeval_realdata_rows(rows)],
        "ratios": [asdict(row) for row in build_ratios(rows)],
        "fallback_shape_summary": [asdict(row) for row in build_fallback_shape_rows(rows)],
        "gate_policy": asdict(policy),
        "gate": [asdict(row) for row in gate_checks],
    }

    args.json.parent.mkdir(parents=True, exist_ok=True)
    args.markdown.parent.mkdir(parents=True, exist_ok=True)
    args.json.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n")
    args.markdown.write_text(markdown_report(rows, commands, current_vs_old, gate_checks, pipeline_fallback_rows))

    for command in commands:
        if command.exit_code != 0:
            print(
                f"{command.label} exited {command.exit_code}; parsed "
                f"{command.parsed_benchmark_count} benchmark rows into the report",
                file=sys.stderr,
            )
            if command.parsed_benchmark_count == 0:
                print(command.output, file=sys.stderr)
                return command.exit_code
    print(f"wrote {args.markdown}")
    print(f"wrote {args.json}")
    if args.check and gate_failed(gate_checks):
        failed = sum(1 for check in gate_checks if check.status == "fail")
        print(f"q performance gate failed: {failed} checks failed", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
