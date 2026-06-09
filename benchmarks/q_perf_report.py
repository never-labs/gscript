#!/usr/bin/env python3
"""Build a q performance completeness report from Go benchmark output."""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from dataclasses import asdict, dataclass, field
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]

BENCH_RE = re.compile(r"^(Benchmark[^\s]+)\s+(\d+)\s+([0-9.]+)\s+ns/op(?:\s+(.*))?$")
BENCH_NO_NS_RE = re.compile(r"^(Benchmark[^\s]+)\s+(\d+)\s+(.*)$")
BENCH_CPU_SUFFIX_RE = re.compile(r"-\d+$")
MIN_TRUSTED_GO_BASELINE_NS = 100.0

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
    "DataRuntimeJoinTopK"
    ")"
)

QEVAL_BENCH = (
    "Benchmark("
    "QEvalVector(ResultCacheWarm|Cold|GoBaseline)|"
    "QSessionEvalVectorWarmExecution"
    ")"
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


@dataclass
class GatePolicy:
    max_leia_go_ratio: float
    min_typed_hit_pct: float
    max_typed_fallbacks_op: float
    max_pipeline_fallback_shapes: float
    max_allocs_op: float


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
    return ratios


def ratio_gate_checks(rows: dict[str, BenchRow], policy: GatePolicy) -> list[GateCheck]:
    checks: list[GateCheck] = []
    for item in build_ratios(rows):
        if "Go" not in item.denominator:
            continue
        if item.ratio is None:
            checks.append(
                GateCheck(
                    signal="leia_go_ratio",
                    benchmark=item.numerator,
                    value=None,
                    threshold=f"<= {policy.max_leia_go_ratio:g}",
                    status="skip",
                    note=item.note or "missing or untrusted denominator",
                )
            )
            continue
        checks.append(
            GateCheck(
                signal="leia_go_ratio",
                benchmark=item.numerator,
                value=item.ratio,
                threshold=f"<= {policy.max_leia_go_ratio:g}",
                status="pass" if item.ratio <= policy.max_leia_go_ratio else "fail",
                note=item.note,
            )
        )
    return checks


def runtime_gate_checks(rows: dict[str, BenchRow], policy: GatePolicy) -> list[GateCheck]:
    checks: list[GateCheck] = []
    for item in build_runtime_metric_rows(rows):
        if item.typed_kernel_hit_pct is not None:
            checks.append(
                GateCheck(
                    signal="typed_hit_pct",
                    benchmark=item.benchmark,
                    value=item.typed_kernel_hit_pct,
                    threshold=f">= {policy.min_typed_hit_pct:g}",
                    status="pass" if item.typed_kernel_hit_pct >= policy.min_typed_hit_pct else "fail",
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
                )
            )
    return checks


def build_gate_checks(rows: dict[str, BenchRow], policy: GatePolicy) -> list[GateCheck]:
    return ratio_gate_checks(rows, policy) + runtime_gate_checks(rows, policy)


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
            "gap": "" if current_vs_old else "provide --timing-json from benchmarks/timing_compare.py or q_columnar_suite JSON output",
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
) -> str:
    current_vs_old = current_vs_old or []
    gate_checks = gate_checks or []
    coverage = build_coverage(rows, current_vs_old)
    qsql_coverage = build_qsql_benchmark_coverage(rows)
    qeval_compute = build_qeval_compute_coverage(rows)
    ratios = build_ratios(rows)
    runtime_metrics = build_runtime_metric_rows(rows)
    fallback_shapes = build_fallback_shape_rows(rows)
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
    lines.extend(
        [
            "",
            "## Runtime Metrics",
            "",
            "| Benchmark | ns/op | B/op | allocs/op | kernel_hit_pct | fallbacks/op | typed_kernel_hit_pct | typed_kernel_attempts/op | typed_kernel_fallbacks/op | typed_kernel_errors/op | typed_pipeline_shapes | typed_pipeline_fallback_shapes |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
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
            f"{format_metric(item.typed_pipeline_fallback_shapes, 0)} |"
        )
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
            "- Pass `--timing-json` from `benchmarks/timing_compare.py` / `q_columnar_suite.sh --json ...` when this report is used for current-vs-old decisions.",
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
    parser.add_argument("--max-leia-go-ratio", type=float, default=5.0)
    parser.add_argument("--min-typed-hit-pct", type=float, default=95.0)
    parser.add_argument("--max-typed-fallbacks-op", type=float, default=0.0)
    parser.add_argument("--max-pipeline-fallback-shapes", type=float, default=0.0)
    parser.add_argument("--max-allocs-op", type=float, default=64.0)
    args = parser.parse_args(argv)

    commands: list[CommandResult] = []
    rows: dict[str, BenchRow] = {}
    current_vs_old: list[CurrentVsOldRow] = []

    if args.from_output:
        for path in args.from_output:
            output = path.read_text()
            parsed = parse_go_benchmarks(output)
            rows.update(parsed)
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

        qeval = run_command(
            "qeval-native",
            ["go", "test", "./benchmarks", "-run", "^$", "-bench", QEVAL_BENCH, "-benchmem", f"-benchtime={args.benchtime}"],
        )
        qeval_rows = parse_go_benchmarks(qeval.output)
        qeval.parsed_benchmark_count = len(qeval_rows)
        commands.append(qeval)
        rows.update(qeval_rows)

    for path in args.timing_json:
        current_vs_old.extend(parse_timing_compare_json(path))

    policy = GatePolicy(
        max_leia_go_ratio=args.max_leia_go_ratio,
        min_typed_hit_pct=args.min_typed_hit_pct,
        max_typed_fallbacks_op=args.max_typed_fallbacks_op,
        max_pipeline_fallback_shapes=args.max_pipeline_fallback_shapes,
        max_allocs_op=args.max_allocs_op,
    )
    gate_checks = build_gate_checks(rows, policy) if args.check else []
    payload = {
        "commands": [asdict(command) for command in commands],
        "benchmarks": {name: asdict(row) for name, row in sorted(rows.items())},
        "coverage": build_coverage(rows, current_vs_old),
        "current_vs_old": [asdict(row) for row in current_vs_old],
        "runtime_metrics": [asdict(row) for row in build_runtime_metric_rows(rows)],
        "qsql_benchmark_coverage": asdict(build_qsql_benchmark_coverage(rows)),
        "q_eval_compute_coverage": asdict(build_qeval_compute_coverage(rows)),
        "ratios": [asdict(row) for row in build_ratios(rows)],
        "fallback_shape_summary": [asdict(row) for row in build_fallback_shape_rows(rows)],
        "gate_policy": asdict(policy),
        "gate": [asdict(row) for row in gate_checks],
    }

    args.json.parent.mkdir(parents=True, exist_ok=True)
    args.markdown.parent.mkdir(parents=True, exist_ok=True)
    args.json.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n")
    args.markdown.write_text(markdown_report(rows, commands, current_vs_old, gate_checks))

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
