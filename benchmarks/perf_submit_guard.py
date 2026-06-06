#!/usr/bin/env python3
"""Submission gate for timing_compare performance artifacts.

The gate is intentionally separate from the measurement runner.  It consumes a
JSON file produced by benchmarks/timing_compare.py and fails when any benchmark
misses the configured current/LuaJIT ratio.  With --baseline it also rejects
candidate results that regress relative to a previously accepted full run.
"""

from __future__ import annotations

import argparse
import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]


@dataclass(frozen=True)
class PerfRow:
    name: str
    current: float | None
    luajit: float | None
    current_status: str
    luajit_status: str
    current_source: str
    luajit_source: str

    @property
    def ratio(self) -> float | None:
        if (
            self.current is None
            or self.luajit is None
            or self.luajit <= 0
            or not self.has_comparable_luajit_timing
        ):
            return None
        return self.current / self.luajit

    @property
    def has_timed_luajit_pair(self) -> bool:
        return self.current is not None and self.luajit is not None and self.luajit > 0

    @property
    def has_comparable_luajit_timing(self) -> bool:
        if not self.has_timed_luajit_pair:
            return False
        if not self.current_source or not self.luajit_source:
            return True
        return self.current_source == self.luajit_source

    def has_comparable_current_timing(self, other: "PerfRow") -> bool:
        if self.current is None or other.current is None:
            return False
        if not self.current_source or not other.current_source:
            return True
        return self.current_source == other.current_source


@dataclass(frozen=True)
class Violation:
    kind: str
    name: str
    value: str
    limit: str


def _subject_seconds(subject: dict[str, Any]) -> float | None:
    stats = subject.get("stats")
    if isinstance(stats, dict):
        median = stats.get("median")
        if isinstance(median, (int, float)):
            return float(median)
    seconds = subject.get("seconds")
    if isinstance(seconds, (int, float)):
        return float(seconds)
    return None


def _subject_status(subject: dict[str, Any]) -> str:
    status = subject.get("status")
    return str(status) if status is not None else "missing"


def _subject_source(subject: dict[str, Any]) -> str:
    source = subject.get("source")
    return str(source) if source is not None else ""


def _timing_compare_row(row: dict[str, Any], mode: str) -> PerfRow | None:
    modes = row.get("modes")
    if not isinstance(modes, dict):
        return None
    mode_row = modes.get(mode)
    if not isinstance(mode_row, dict):
        return None
    current = mode_row.get("current")
    luajit = mode_row.get("luajit")
    if not isinstance(current, dict):
        current = {}
    if not isinstance(luajit, dict):
        luajit = {}
    group = str(row.get("group") or "")
    bench = str(row.get("benchmark") or "")
    name = f"{group}/{bench}" if group and not bench.startswith(f"{group}/") else bench
    return PerfRow(
        name=name,
        current=_subject_seconds(current),
        luajit=_subject_seconds(luajit),
        current_status=_subject_status(current),
        luajit_status=_subject_status(luajit),
        current_source=_subject_source(current),
        luajit_source=_subject_source(luajit),
    )


def _flat_guard_row(row: dict[str, Any]) -> PerfRow | None:
    bench = row.get("benchmark")
    if not isinstance(bench, str):
        return None
    default = row.get("default")
    luajit = row.get("luajit")
    if not isinstance(default, dict):
        default = {}
    if not isinstance(luajit, dict):
        luajit = {}
    return PerfRow(
        name=bench,
        current=_subject_seconds(default),
        luajit=_subject_seconds(luajit),
        current_status=_subject_status(default),
        luajit_status=_subject_status(luajit),
        current_source=_subject_source(default),
        luajit_source=_subject_source(luajit),
    )


def load_rows(path: Path, *, mode: str = "default") -> dict[str, PerfRow]:
    with path.open() as f:
        payload = json.load(f)
    raw_rows = payload.get("results")
    if not isinstance(raw_rows, list):
        raise ValueError("performance JSON must contain a list-valued 'results'")

    rows: dict[str, PerfRow] = {}
    for raw in raw_rows:
        if not isinstance(raw, dict):
            continue
        row = _timing_compare_row(raw, mode) or _flat_guard_row(raw)
        if row is not None and row.name:
            rows[row.name] = row
    return rows


def load_luajit_required_benchmarks(manifest_path: Path | None = None) -> set[str]:
    manifest_path = manifest_path or ROOT / "benchmarks" / "manifest.json"
    manifest = json.loads(manifest_path.read_text())
    workloads = manifest.get("workloads")
    if not isinstance(workloads, list):
        return set()
    required: set[str] = set()
    for workload in workloads:
        if not isinstance(workload, dict):
            continue
        benchmark_id = workload.get("id")
        if isinstance(benchmark_id, str) and workload.get("comparison_reference") is not None:
            required.add(benchmark_id)
    return required


def check_rows(
    candidate: dict[str, PerfRow],
    *,
    baseline: dict[str, PerfRow] | None = None,
    luajit_required: set[str] | None = None,
    ratio_threshold: float = 0.8,
    regression_tolerance: float = 0.03,
) -> list[Violation]:
    violations: list[Violation] = []
    for name, row in sorted(candidate.items()):
        require_luajit = luajit_required is None or name in luajit_required
        ratio = row.ratio
        if require_luajit and not row.has_timed_luajit_pair:
            violations.append(
                Violation("missing", name, f"current={row.current_status} luajit={row.luajit_status}", "timed current+luajit")
            )
        elif not require_luajit:
            pass
        elif not row.has_comparable_luajit_timing:
            pass
        elif ratio > ratio_threshold:
            violations.append(Violation("luajit", name, f"{ratio:.3f}x", f"<={ratio_threshold:.3f}x"))

        if baseline is None:
            continue
        base = baseline.get(name)
        if base is None or base.current is None or row.current is None or base.current <= 0:
            continue
        if not row.has_comparable_current_timing(base):
            continue
        change = row.current / base.current - 1.0
        if change > regression_tolerance:
            violations.append(Violation("regression", name, f"+{change * 100:.2f}%", f"<={regression_tolerance * 100:.2f}%"))
    return violations


def format_summary(rows: dict[str, PerfRow], violations: list[Violation]) -> str:
    worst = sorted(
        [row for row in rows.values() if row.has_comparable_luajit_timing and row.ratio is not None],
        key=lambda row: row.ratio or -1.0,
        reverse=True,
    )[:12]
    lines = ["Worst current/LuaJIT ratios:", "Benchmark                          Current     LuaJIT    Cur/LJ"]
    lines.append("----------------------------------------------------------------")
    for row in worst:
        lines.append(f"{row.name:<34} {row.current:>8.6f}s {row.luajit:>8.6f}s {row.ratio:>7.3f}x")
    if violations:
        lines.extend(["", "Guard violations:", "Kind        Benchmark                          Value       Limit"])
        lines.append("----------------------------------------------------------------")
        for item in violations:
            lines.append(f"{item.kind:<11} {item.name:<34} {item.value:>10} {item.limit:>10}")
    else:
        lines.append("")
        lines.append("Guard passed.")
    return "\n".join(lines) + "\n"


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("candidate", type=Path, help="timing_compare JSON to validate")
    parser.add_argument("--baseline", type=Path, help="previous accepted timing_compare JSON")
    parser.add_argument("--mode", default="default")
    parser.add_argument("--ratio-threshold", type=float, default=0.8)
    parser.add_argument("--regression-tolerance", type=float, default=0.03)
    args = parser.parse_args(argv)

    candidate = load_rows(args.candidate, mode=args.mode)
    baseline = load_rows(args.baseline, mode=args.mode) if args.baseline else None
    luajit_required = load_luajit_required_benchmarks()
    violations = check_rows(
        candidate,
        baseline=baseline,
        luajit_required=luajit_required,
        ratio_threshold=args.ratio_threshold,
        regression_tolerance=args.regression_tolerance,
    )
    print(format_summary(candidate, violations), end="")
    return 1 if violations else 0


if __name__ == "__main__":
    raise SystemExit(main())
