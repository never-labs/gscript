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
BENCH_CPU_SUFFIX_RE = re.compile(r"-\d+$")

QSQL_BENCH = (
    "BenchmarkQSQL("
    "BindRunSQLWarmCacheSelectWhereProject|"
    "BindRunSQLColdCacheSelectWhereProject|"
    "BindFastArg2WarmCacheSelectWhereProject|"
    "BindRunSQLWarmCacheGroupByAggregate|"
    "BindRunSQLWarmCacheJoin|"
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
class QEvalComputeCoverage:
    session_case_count: int
    go_baseline_case_count: int
    result_cache_warm_case_count: int
    cold_case_count: int
    matched_go_baseline_count: int
    matched_result_cache_warm_count: int
    matched_cold_count: int
    missing_go_baseline: list[str]
    missing_result_cache_warm: list[str]
    missing_cold: list[str]
    orphan_go_baseline: list[str]


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
        match = BENCH_RE.match(line.strip())
        if not match:
            continue
        raw_name, iterations, ns_op, rest = match.groups()
        name = normalize_benchmark_name(raw_name)
        rows[name] = BenchRow(
            name=name,
            iterations=int(iterations),
            ns_op=float(ns_op),
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


def qeval_cases(rows: dict[str, BenchRow], prefix: str) -> set[str]:
    marker = prefix + "/"
    return {name.removeprefix(marker) for name in rows if name.startswith(marker)}


def build_qeval_compute_coverage(rows: dict[str, BenchRow]) -> QEvalComputeCoverage:
    session = qeval_cases(rows, "BenchmarkQSessionEvalVectorWarmExecution")
    go = qeval_cases(rows, "BenchmarkQEvalVectorGoBaseline")
    warm = qeval_cases(rows, "BenchmarkQEvalVectorResultCacheWarm")
    cold = qeval_cases(rows, "BenchmarkQEvalVectorCold")
    return QEvalComputeCoverage(
        session_case_count=len(session),
        go_baseline_case_count=len(go),
        result_cache_warm_case_count=len(warm),
        cold_case_count=len(cold),
        matched_go_baseline_count=len(session & go),
        matched_result_cache_warm_count=len(session & warm),
        matched_cold_count=len(session & cold),
        missing_go_baseline=sorted(session - go),
        missing_result_cache_warm=sorted(session - warm),
        missing_cold=sorted(session - cold),
        orphan_go_baseline=sorted(go - session),
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
        ratios.extend(
            [
                RatioRow(
                    f"q.eval {case} session execution vs Go",
                    session,
                    go,
                    ratio(rows, session, go),
                    "session eval bypasses q.eval result cache and measures repeated execution",
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


def metric_present(rows: dict[str, BenchRow], names: list[str], metric: str) -> bool:
    return any(metric in rows.get(name, BenchRow(name, 0, 0)).metrics for name in names)


def build_coverage(rows: dict[str, BenchRow]) -> list[dict[str, str]]:
    qsql_names = [name for name in rows if name.startswith("BenchmarkQSQL")]
    qeval_names = [name for name in rows if name.startswith("BenchmarkQEval") or name.startswith("BenchmarkQSessionEval")]
    return [
        {
            "signal": "current Leia vs old Leia",
            "qSQL": "covered by q_columnar_suite current-vs-HEAD, not this Go bench report",
            "q.eval": "covered by q_columnar_suite for data/q_columnar_eval_primitives, not this Go bench report",
            "gap": "run q_columnar_suite alongside this report for old-Leia ratios",
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
            "q.eval": "missing",
            "gap": "q.eval exposes eval-cache stats, but no per-shape typed-kernel hit/fallback metric is benchmark-readable from benchmarks/",
        },
        {
            "signal": "fallback rate",
            "qSQL": "covered" if metric_present(rows, qsql_names, "fallbacks/op") else "missing",
            "q.eval": "missing",
            "gap": "q.eval fallback stats need a public/cache_stats shape tied to vector/list kernels",
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


def markdown_report(rows: dict[str, BenchRow], commands: list[CommandResult]) -> str:
    coverage = build_coverage(rows)
    qeval_compute = build_qeval_compute_coverage(rows)
    ratios = build_ratios(rows)
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
            "## Ordinary q Compute Coverage",
            "",
            "| Signal | Count |",
            "|---|---:|",
            f"| session execution cases | {qeval_compute.session_case_count} |",
            f"| Go baseline cases | {qeval_compute.go_baseline_case_count} |",
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
            "## Raw Benchmarks",
            "",
            "| Benchmark | ns/op | B/op | allocs/op | kernel_hit_pct | fallbacks/op |",
            "|---|---:|---:|---:|---:|---:|",
        ]
    )
    for name in sorted(rows):
        row = rows[name]
        lines.append(
            f"| {name} | {row.ns_op:.0f} | "
            f"{row.metrics.get('B/op', 0):.0f} | "
            f"{row.metrics.get('allocs/op', 0):.0f} | "
            f"{row.metrics.get('kernel_hit_pct', 0):.1f} | "
            f"{row.metrics.get('fallbacks/op', 0):.3f} |"
        )
    lines.extend(
        [
            "",
            "## Required Follow-up Gaps",
            "",
            "- Add q.eval/vector-runtime typed kernel hit and fallback counters that are visible through `q.cache_stats()` or a benchmark-safe public API.",
            "- Add stable q.eval math-map coverage once unary/vector math expressions such as exp/log/sqrt have complete parser/eval support.",
            "- Add qSQL cold-cache counterparts for group and join if those paths are used to judge schema-stable cache value.",
            "- Add current-vs-old Go benchmark comparison for q.eval/qSQL, or always pair this report with `bash benchmarks/q_columnar_suite.sh` for current-vs-HEAD timing.",
        ]
    )
    return "\n".join(lines) + "\n"


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--benchtime", default="100x")
    parser.add_argument("--json", type=Path, default=Path("benchmarks/data/q_perf_report_latest.json"))
    parser.add_argument("--markdown", type=Path, default=Path("benchmarks/data/q_perf_report_latest.md"))
    parser.add_argument("--from-output", type=Path, action="append", default=[], help="Parse existing go test output instead of running commands.")
    args = parser.parse_args(argv)

    commands: list[CommandResult] = []
    rows: dict[str, BenchRow] = {}

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

    payload = {
        "commands": [asdict(command) for command in commands],
        "benchmarks": {name: asdict(row) for name, row in sorted(rows.items())},
        "coverage": build_coverage(rows),
        "q_eval_compute_coverage": asdict(build_qeval_compute_coverage(rows)),
        "ratios": [asdict(row) for row in build_ratios(rows)],
    }

    args.json.parent.mkdir(parents=True, exist_ok=True)
    args.markdown.parent.mkdir(parents=True, exist_ok=True)
    args.json.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n")
    args.markdown.write_text(markdown_report(rows, commands))

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
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
