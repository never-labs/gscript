#!/usr/bin/env python3
"""Strict benchmark harness for statistically meaningful Leia comparisons.

Runs domain benchmarks in VM, default JIT, no-filter JIT, and optional LuaJIT
modes. Unlike regression_guard.py, this harness keeps timing quality explicit:
zero/too-small script times are either calibrated with repeated invocations or
reported as low_resolution instead of being treated as wins.
"""

from __future__ import annotations

import argparse
import concurrent.futures
import hashlib
import json
import math
import platform
import re
import shutil
import statistics
import subprocess
import sys
import tempfile
import time
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Iterable

import benchmark_discovery as discovery
import benchmark_output

DEFAULT_MODES = ["vm", "default", "no_filter", "luajit"]
DEFAULT_ORDER = discovery.DEFAULT_ORDER
ALL_GROUPS = discovery.GROUPS
STRICT_DEFAULT_GROUPS = tuple(group for group in discovery.GROUPS if group != "concurrency")
DEFAULT_GROUPS = STRICT_DEFAULT_GROUPS
SERIAL_GROUPS = {"concurrency"}

LOGICAL_TIME_BENCHMARKS = {
    "control/defer_protected",
}

CHECKSUM_RE = re.compile(r"^\s*checksum\s*[:=]\s*(.+?)\s*$", re.IGNORECASE | re.MULTILINE)
EMBEDDED_TIME_RE = re.compile(r"(:\s*)[0-9]+(?:\.[0-9]+)?s(\s+)")
T2_ATTEMPTED_RE = re.compile(r"^\s*Tier 2 attempted:\s*([0-9]+)\b", re.MULTILINE)
T2_ENTERED_RE = re.compile(r"^\s*Tier 2 entered:\s*([0-9]+)\s+functions\b", re.MULTILINE)
T2_FAILED_RE = re.compile(r"^\s*Tier 2 failed:\s*([0-9]+)\s+functions\b", re.MULTILINE)
EXIT_TOTAL_RE = re.compile(r"^\s*total exits:\s*([0-9]+)\b", re.MULTILINE)


@dataclass(frozen=True)
class BenchmarkSpec:
    group: str
    name: str
    leia: Path
    luajit: Path | None = None
    base: str | None = None

    @property
    def benchmark_id(self) -> str:
        return f"{self.group}/{self.name}"


@dataclass
class Stats:
    n: int = 0
    median: float | None = None
    min: float | None = None
    max: float | None = None
    mean: float | None = None
    stdev: float | None = None
    mad: float | None = None
    cv_pct: float | None = None


@dataclass
class CommandRun:
    status: str
    seconds: float | None = None
    wall_seconds: float | None = None
    exit_code: int | None = None
    output_tail: str = ""
    output_hash: str = ""
    checksum_text: str = ""
    t2_attempted: int = 0
    t2_entered: int = 0
    t2_failed: int = 0
    exit_total: int = 0


@dataclass
class Sample:
    status: str
    seconds: float | None = None
    repeat: int = 1
    time_source: str = ""
    script_total_seconds: float | None = None
    wall_total_seconds: float | None = None
    note: str = ""
    runs: list[CommandRun] = field(default_factory=list)


@dataclass
class ModeResult:
    status: str
    repeat: int = 1
    stats: Stats = field(default_factory=Stats)
    samples: list[Sample] = field(default_factory=list)
    warmups: list[Sample] = field(default_factory=list)
    output_hash: str = ""
    checksum_text: str = ""
    checksum_status: str = "missing"
    t2_attempted: int = 0
    t2_entered: int = 0
    t2_failed: int = 0
    exit_total: int = 0
    note: str = ""


@dataclass
class BenchmarkResult:
    benchmark: str
    group: str = "unknown"
    base: str | None = None
    modes: dict[str, ModeResult] = field(default_factory=dict)


parse_time = benchmark_output.parse_time


def stable_output_lines(output: str) -> list[str]:
    lines: list[str] = []
    skip_prefixes = (
        "Time:",
        "JIT Statistics:",
        "Tier 2 Exit Profile:",
        "[DEBUG]",
    )
    skip_contains = (
        "Tier 2 attempted:",
        "Tier 2 compiled:",
        "Tier 2 entered:",
        "Tier 2 failed:",
        "total exits:",
    )
    for raw_line in output.splitlines():
        line = raw_line.strip()
        if not line:
            continue
        if line.startswith("Time:"):
            break
        if any(line.startswith(prefix) for prefix in skip_prefixes):
            continue
        if any(fragment in line for fragment in skip_contains):
            continue
        lines.append(stable_output_line(line))
    return lines


def stable_output_line(line: str) -> str:
    return EMBEDDED_TIME_RE.sub(r"\1<time>\2", line)


def output_hash(output: str) -> str:
    payload = "\n".join(stable_output_lines(output))
    if not payload:
        return ""
    return hashlib.sha256(payload.encode()).hexdigest()[:16]


def checksum_text(output: str) -> str:
    match = CHECKSUM_RE.search(output)
    if match:
        return match.group(1)
    lines = stable_output_lines(output)
    return lines[-1] if lines else ""


parse_counter = benchmark_output.parse_counter
output_tail = benchmark_output.output_tail


def parse_command_run(output: str, status: str, exit_code: int | None = None) -> CommandRun:
    seconds = parse_time(output) if status == "ok" else None
    if status == "ok" and seconds is None:
        status = "no_time"
    return CommandRun(
        status=status,
        seconds=seconds,
        exit_code=exit_code,
        output_tail=output_tail(output),
        output_hash=output_hash(output),
        checksum_text=checksum_text(output),
        t2_attempted=parse_counter(T2_ATTEMPTED_RE, output),
        t2_entered=parse_counter(T2_ENTERED_RE, output),
        t2_failed=parse_counter(T2_FAILED_RE, output),
        exit_total=parse_counter(EXIT_TOTAL_RE, output),
    )


def median_absolute_deviation(values: list[float]) -> float | None:
    if not values:
        return None
    med = statistics.median(values)
    return statistics.median([abs(v - med) for v in values])


def compute_stats(values: list[float]) -> Stats:
    if not values:
        return Stats()
    mean = statistics.fmean(values)
    stdev = statistics.stdev(values) if len(values) > 1 else 0.0
    return Stats(
        n=len(values),
        median=statistics.median(values),
        min=min(values),
        max=max(values),
        mean=mean,
        stdev=stdev,
        mad=median_absolute_deviation(values),
        cv_pct=(stdev / mean * 100.0) if mean > 0 else None,
    )


def run_command(cmd: list[str], timeout: int, env: dict[str, str] | None = None) -> CommandRun:
    result = benchmark_output.run_text_command(cmd, timeout, env)
    run = parse_command_run(result.output, result.status, result.exit_code)
    run.wall_seconds = result.wall_seconds
    return run


def summarize_repeated_runs(
    runs: list[CommandRun],
    repeat: int,
    timer_resolution: float,
    min_sample_seconds: float,
    allow_wall_time: bool,
    prefer_wall_time: bool = False,
) -> Sample:
    wall_total = sum(r.wall_seconds or 0.0 for r in runs)
    bad = [r for r in runs if r.status not in {"ok", "no_time"}]
    if bad:
        status = bad[-1].status
        return Sample(
            status=status,
            repeat=repeat,
            wall_total_seconds=wall_total,
            note=f"{len(bad)} of {repeat} command runs ended with {status}",
            runs=runs,
        )

    if any(r.status == "no_time" for r in runs):
        return Sample(
            status="no_time",
            repeat=repeat,
            wall_total_seconds=wall_total,
            note="benchmark completed but did not print a Time: line",
            runs=runs,
        )

    script_total = sum(r.seconds or 0.0 for r in runs)
    if prefer_wall_time:
        if allow_wall_time and wall_total >= min_sample_seconds:
            return Sample(
                status="ok",
                seconds=wall_total / repeat,
                repeat=repeat,
                time_source="wall_repeat",
                script_total_seconds=script_total,
                wall_total_seconds=wall_total,
                note="benchmark script Time is logical; per-command wall time used",
                runs=runs,
            )
        return Sample(
            status="low_resolution",
            repeat=repeat,
            script_total_seconds=script_total,
            wall_total_seconds=wall_total,
            note="benchmark script Time is logical; enable --allow-wall-time for performance comparison",
            runs=runs,
        )

    if script_total <= timer_resolution:
        if allow_wall_time and wall_total >= min_sample_seconds:
            return Sample(
                status="ok",
                seconds=wall_total / repeat,
                repeat=repeat,
                time_source="wall_repeat",
                script_total_seconds=script_total,
                wall_total_seconds=wall_total,
                note="script timer was below resolution; per-command wall time used",
                runs=runs,
            )
        return Sample(
            status="low_resolution",
            repeat=repeat,
            script_total_seconds=script_total,
            wall_total_seconds=wall_total,
            note="script timer was below resolution",
            runs=runs,
        )

    return Sample(
        status="ok",
        seconds=script_total / repeat,
        repeat=repeat,
        time_source="script",
        script_total_seconds=script_total,
        wall_total_seconds=wall_total,
        runs=runs,
    )


def run_sample(
    cmd: list[str],
    env: dict[str, str] | None,
    repeat: int,
    timeout: int,
    timer_resolution: float,
    min_sample_seconds: float,
    allow_wall_time: bool,
    prefer_wall_time: bool = False,
) -> Sample:
    runs = [run_command(cmd, timeout, env) for _ in range(repeat)]
    return summarize_repeated_runs(
        runs, repeat, timer_resolution, min_sample_seconds, allow_wall_time, prefer_wall_time
    )


def sample_is_big_enough(sample: Sample, min_sample_seconds: float) -> bool:
    if sample.status != "ok":
        return False
    if sample.time_source == "script":
        return (sample.script_total_seconds or 0.0) >= min_sample_seconds
    if sample.time_source == "wall_repeat":
        return (sample.wall_total_seconds or 0.0) >= min_sample_seconds
    return False


def calibrate_repeat(
    cmd: list[str],
    env: dict[str, str] | None,
    timeout: int,
    timer_resolution: float,
    min_sample_seconds: float,
    max_repeat: int,
    allow_wall_time: bool,
    prefer_wall_time: bool = False,
) -> tuple[int, Sample]:
    repeat = 1
    last: Sample | None = None
    while repeat <= max_repeat:
        last = run_sample(
            cmd, env, repeat, timeout, timer_resolution, min_sample_seconds, allow_wall_time, prefer_wall_time
        )
        if sample_is_big_enough(last, min_sample_seconds):
            return repeat, last
        if last.status in {"error", "timeout", "no_time"}:
            return repeat, last
        repeat *= 2
    assert last is not None
    return max_repeat, last


def summarize_mode(samples: list[Sample], warmups: list[Sample], repeat: int) -> ModeResult:
    ok = [s for s in samples if s.status == "ok" and s.seconds is not None]
    stats = compute_stats([s.seconds for s in ok if s.seconds is not None])
    if not samples:
        status = "missing"
    elif ok and len(ok) == len(samples):
        status = "ok"
    elif ok:
        status = "partial"
    else:
        status = samples[-1].status

    source_runs = ok[len(ok) // 2].runs if ok else (samples[-1].runs if samples else [])
    counters = source_runs[-1] if source_runs else CommandRun("missing")
    hashes = sorted({run.output_hash for sample in samples for run in sample.runs if run.output_hash})
    checksum_status = "missing"
    output_digest = ""
    checksum = ""
    if len(hashes) == 1:
        checksum_status = "ok"
        output_digest = hashes[0]
        checksum = next(
            (
                run.checksum_text
                for sample in samples
                for run in sample.runs
                if run.output_hash == output_digest and run.checksum_text
            ),
            "",
        )
    elif len(hashes) > 1:
        checksum_status = "mismatch"
        output_digest = ",".join(hashes)
    return ModeResult(
        status=status,
        repeat=repeat,
        stats=stats,
        samples=samples,
        warmups=warmups,
        output_hash=output_digest,
        checksum_text=checksum,
        checksum_status=checksum_status,
        t2_attempted=counters.t2_attempted,
        t2_entered=counters.t2_entered,
        t2_failed=counters.t2_failed,
        exit_total=counters.exit_total,
    )


def mode_command(
    mode: str,
    spec: BenchmarkSpec,
    leia_bin: Path,
    luajit_bin: str | None,
) -> tuple[list[str] | None, dict[str, str] | None, str | None]:
    return benchmark_output.benchmark_mode_command(
        mode,
        leia_bin,
        spec.leia,
        luajit_bin=luajit_bin,
        luajit_script=spec.luajit,
    )


def run_mode(
    mode: str,
    spec: BenchmarkSpec,
    leia_bin: Path,
    luajit_bin: str | None,
    warmup_runs: int,
    measured_runs: int,
    timeout: int,
    timer_resolution: float,
    min_sample_seconds: float,
    max_repeat: int,
    allow_wall_time: bool,
    repeat_override: int | None = None,
) -> ModeResult:
    cmd, env, unavailable = mode_command(mode, spec, leia_bin, luajit_bin)
    if unavailable:
        return ModeResult(status=unavailable, note=f"{mode} input unavailable")
    assert cmd is not None

    repeat = repeat_override or 1
    prefer_wall_time = spec.benchmark_id in LOGICAL_TIME_BENCHMARKS
    warmups: list[Sample] = []
    if repeat_override is None:
        repeat, calibration = calibrate_repeat(
            cmd, env, timeout, timer_resolution, min_sample_seconds, max_repeat, allow_wall_time, prefer_wall_time
        )
        warmups.append(calibration)

    for _ in range(warmup_runs):
        warmups.append(
            run_sample(cmd, env, repeat, timeout, timer_resolution, min_sample_seconds, allow_wall_time, prefer_wall_time)
        )

    samples = [
        run_sample(cmd, env, repeat, timeout, timer_resolution, min_sample_seconds, allow_wall_time, prefer_wall_time)
        for _ in range(measured_runs)
    ]
    result = summarize_mode(samples, warmups, repeat)
    if result.status == "low_resolution":
        result.note = f"increase --max-repeat above {max_repeat} or enable --allow-wall-time"
    return result


def build_leia(root: Path, out: Path) -> None:
    benchmark_output.build_leia(root, out)


def parse_repeat_overrides(values: list[str] | None) -> dict[tuple[str | None, str], int]:
    return discovery.parse_selector_count_overrides(values, DEFAULT_MODES, "--repeat")


def repeat_for(
    overrides: dict[tuple[str | None, str], int],
    mode: str,
    bench: str,
    benchmark_id: str | None = None,
) -> int | None:
    return discovery.selector_count_override(overrides, mode, bench, benchmark_id)


def fmt_seconds(value: float | None) -> str:
    if value is None:
        return "-"
    return f"{value:.6f}s"


def fmt_pct(value: float | None) -> str:
    if value is None:
        return "-"
    return f"{value:.2f}%"


def comparable_seconds(result: ModeResult | None) -> float | None:
    if result is None or result.status not in {"ok", "partial"}:
        return None
    return result.stats.median


def ratio(numer: float | None, denom: float | None) -> float | None:
    if numer is None or denom is None or denom == 0:
        return None
    return numer / denom


canonical_group = discovery.canonical_group
def domain_specs(root: Path, group: str) -> list[BenchmarkSpec]:
    return [
        BenchmarkSpec(spec.group, spec.name, spec.leia, spec.luajit, spec.base)
        for spec in discovery.domain_specs(root, group)
    ]


def discover_specs(root: Path, groups: list[str]) -> list[BenchmarkSpec]:
    return [
        BenchmarkSpec(spec.group, spec.name, spec.leia, spec.luajit, spec.base)
        for spec in discovery.discover_benchmarks(root, groups)
    ]


def select_specs(specs: list[BenchmarkSpec], selectors: list[str] | None) -> list[BenchmarkSpec]:
    return discovery.select_specs(specs, selectors)


def checksum_mismatch_modes(row: BenchmarkResult, modes: list[str]) -> list[str]:
    hashes = {
        mode: result.output_hash
        for mode in modes
        if (result := row.modes.get(mode)) and result.status in {"ok", "partial"} and result.output_hash
    }
    if len(set(hashes.values())) <= 1:
        return []
    return [f"{mode}:{digest}" for mode, digest in hashes.items()]


def suspicious_kernel_wins(
    results: Iterable[BenchmarkResult],
    args: argparse.Namespace,
) -> list[str]:
    rows = list(results)
    lines: list[str] = []
    for row in rows:
        if row.base:
            continue
        vm = comparable_seconds(row.modes.get("vm"))
        default = comparable_seconds(row.modes.get("default"))
        luajit = comparable_seconds(row.modes.get("luajit"))
        vm_speedup = ratio(vm, default)
        lj_ratio = ratio(default, luajit)
        if vm_speedup is None or lj_ratio is None:
            continue
        if vm_speedup < args.suspicious_vm_speedup or lj_ratio > args.suspicious_luajit_ratio:
            continue

        related_workloads = [r for r in rows if r.base == row.benchmark]
        if not related_workloads:
            lines.append(
                f"- `{row.benchmark}`: Default is {fmt_ratio(vm_speedup)} faster than VM and "
                f"{fmt_ratio(1.0 / lj_ratio)} faster than LuaJIT, with no related workload in this run."
            )
            continue

        for related in related_workloads:
            related_default = comparable_seconds(related.modes.get("default"))
            related_luajit = comparable_seconds(related.modes.get("luajit"))
            related_ratio = ratio(related_default, related_luajit)
            if related_ratio is None:
                lines.append(
                    f"- `{row.benchmark}`: baseline win lacks a comparable `{related.benchmark}` "
                    "LuaJIT related workload result."
                )
            elif related_ratio >= args.related_confirm_ratio:
                lines.append(
                    f"- `{row.benchmark}`: baseline beats LuaJIT by {fmt_ratio(1.0 / lj_ratio)}, "
                    f"but `{related.benchmark}` is {fmt_ratio(related_ratio)} of LuaJIT "
                    "(related workload does not confirm the win)."
                )
    return lines


def markdown_summary(results: Iterable[BenchmarkResult], modes: list[str], args: argparse.Namespace) -> str:
    rows = list(results)
    groups = getattr(args, "group", DEFAULT_GROUPS)
    lines = [
        "# Strict Benchmark Summary",
        "",
        f"- Benchmark groups: {', '.join(groups)}",
        f"- Warmups: {args.warmup}",
        f"- Measured runs: {args.runs}",
        f"- Min sample seconds: {args.min_sample_seconds:.3f}",
        f"- Timer resolution floor: {args.timer_resolution:.6f}",
        f"- Wall-time fallback: {'enabled' if args.allow_wall_time else 'disabled'}",
        "",
        "## Comparisons",
        "",
        "| Benchmark | VM/Default | Default/LuaJIT | Notes |",
        "|---|---:|---:|---|",
    ]
    for row in rows:
        vm = comparable_seconds(row.modes.get("vm"))
        default = comparable_seconds(row.modes.get("default"))
        luajit = comparable_seconds(row.modes.get("luajit"))
        notes = []
        mismatches = checksum_mismatch_modes(row, modes)
        if mismatches:
            notes.append("checksum mismatch: " + ", ".join(mismatches))
        for mode in modes:
            result = row.modes.get(mode)
            if result and result.status not in {"ok", "partial", "skipped"}:
                notes.append(f"{mode}:{result.status}")
        lines.append(
            benchmark_output.markdown_row(
                [
                    f"{row.group}/{row.benchmark}",
                    fmt_ratio(ratio(vm, default)),
                    fmt_ratio(ratio(default, luajit)),
                    ", ".join(notes) or "-",
                ]
            )
        )

    suspicious = suspicious_kernel_wins(rows, args)
    lines.extend(benchmark_output.markdown_section("Suspicious Kernel Wins"))
    if suspicious:
        lines.extend(suspicious)
    else:
        lines.append("_No baseline-only win crossed the suspicion thresholds._")

    lines.extend(
        benchmark_output.markdown_section(
            "Measurements",
            "| Benchmark | Mode | Status | Source | Repeat | N | Median | Min | Max | Stdev | MAD | CV | Checksum | T2 a/e/f | Exits | Note |",
            "|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---|",
        )
    )
    for row in rows:
        for mode in modes:
            result = row.modes.get(mode) or ModeResult("missing")
            source = "-"
            notes = [result.note] if result.note else []
            sources = sorted({s.time_source for s in result.samples if s.time_source})
            if sources:
                source = ",".join(sources)
            sample_notes = sorted({s.note for s in result.samples if s.note})
            notes.extend(sample_notes)
            t2 = f"{result.t2_attempted}/{result.t2_entered}/{result.t2_failed}"
            checksum = result.checksum_text or result.output_hash or "-"
            if result.checksum_status == "mismatch":
                checksum = "MISMATCH"
            lines.append(
                benchmark_output.markdown_row(
                    [
                        f"{row.group}/{row.benchmark}",
                        mode,
                        result.status,
                        source,
                        str(result.repeat),
                        str(result.stats.n),
                        fmt_seconds(result.stats.median),
                        fmt_seconds(result.stats.min),
                        fmt_seconds(result.stats.max),
                        fmt_seconds(result.stats.stdev),
                        fmt_seconds(result.stats.mad),
                        fmt_pct(result.stats.cv_pct),
                        checksum,
                        t2,
                        str(result.exit_total),
                        "; ".join(notes) or "-",
                    ]
                )
            )
    return "\n".join(lines) + "\n"


def fmt_ratio(value: float | None) -> str:
    if value is None or math.isinf(value) or math.isnan(value):
        return "-"
    return f"{value:.2f}x"


def print_brief(results: Iterable[BenchmarkResult], modes: list[str]) -> None:
    print(f"{'Benchmark':<34} {'Mode':<10} {'Status':<15} {'Median':>12} {'CV':>9} {'Repeat':>6} {'Source':<12} {'Checksum':<16}")
    print("-" * 126)
    for row in results:
        for mode in modes:
            result = row.modes.get(mode) or ModeResult("missing")
            sources = sorted({s.time_source for s in result.samples if s.time_source})
            checksum = result.output_hash or "-"
            if result.checksum_status == "mismatch":
                checksum = "MISMATCH"
            print(
                f"{row.group + '/' + row.benchmark:<34} {mode:<10} {result.status:<15} "
                f"{fmt_seconds(result.stats.median):>12} {fmt_pct(result.stats.cv_pct):>9} "
                f"{result.repeat:>6} {(','.join(sources) or '-'):<12} {checksum:<16}"
            )


def dry_run_results(specs: list[BenchmarkSpec], modes: list[str], luajit_bin: str | None) -> list[BenchmarkResult]:
    rows: list[BenchmarkResult] = []
    fake_bin = Path("leia")
    for spec in specs:
        row = BenchmarkResult(spec.name, spec.group, spec.base)
        for mode in modes:
            _cmd, _env, unavailable = mode_command(mode, spec, fake_bin, luajit_bin)
            row.modes[mode] = ModeResult(status=unavailable or "planned")
        rows.append(row)
    return rows


def write_json(path: Path, payload: dict[str, object]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2) + "\n")


def write_markdown(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)


def positive_int(value: str) -> int:
    parsed = int(value)
    if parsed <= 0:
        raise argparse.ArgumentTypeError("must be > 0")
    return parsed


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bench", action="append", help="benchmark name or group/name to run; repeatable")
    parser.add_argument(
        "--group",
        action="append",
        choices=discovery.group_choices(ALL_GROUPS),
        help="benchmark group to run; repeatable; default is all domain groups",
    )
    parser.add_argument("--mode", action="append", choices=DEFAULT_MODES, help="mode to run; repeatable")
    parser.add_argument("--warmup", type=int, default=1, help="warmup samples after calibration")
    parser.add_argument("--runs", "--measured", dest="runs", type=positive_int, default=7)
    parser.add_argument("--timeout", type=positive_int, default=60, help="timeout per command invocation")
    parser.add_argument("--min-sample-seconds", type=float, default=0.020)
    parser.add_argument("--timer-resolution", type=float, default=0.001)
    parser.add_argument("--max-repeat", type=positive_int, default=32)
    parser.add_argument(
        "--repeat",
        action="append",
        help="force repeat count for BENCH=N or MODE/BENCH=N; disables calibration for that cell",
    )
    parser.add_argument(
        "--allow-wall-time",
        action="store_true",
        help="use repeated command wall time when script Time output is below resolution",
    )
    parser.add_argument("--no-luajit", action="store_true", help="skip LuaJIT even when installed")
    parser.add_argument("--dry-run", action="store_true", help="build no binary and run no benchmarks")
    parser.add_argument("--suspicious-vm-speedup", type=float, default=2.0)
    parser.add_argument("--suspicious-luajit-ratio", type=float, default=0.75)
    parser.add_argument("--related-confirm-ratio", type=float, default=0.95)
    parser.add_argument("--json", type=Path, default=Path("benchmarks/data/strict_guard_latest.json"))
    parser.add_argument("--markdown", type=Path, default=Path("benchmarks/data/strict_guard_latest.md"))
    parser.add_argument("--keep-bin", action="store_true", help="keep temporary leia binary")
    parser.add_argument(
        "--jobs",
        type=positive_int,
        default=1,
        help="number of benchmarks to run concurrently; modes within a benchmark remain sequential",
    )
    parser.add_argument(
        "--progress",
        action="store_true",
        help="print per-benchmark progress to stderr for long strict runs",
    )
    args = parser.parse_args(argv)

    if args.warmup < 0:
        parser.error("--warmup must be >= 0")
    if args.min_sample_seconds <= 0:
        parser.error("--min-sample-seconds must be > 0")
    if args.timer_resolution < 0:
        parser.error("--timer-resolution must be >= 0")

    root = Path(__file__).resolve().parents[1]
    args.group = args.group or DEFAULT_GROUPS
    specs = select_specs(discover_specs(root, args.group), args.bench)
    modes = args.mode or DEFAULT_MODES
    luajit_bin = None if args.no_luajit else shutil.which("luajit")
    try:
        repeat_overrides = parse_repeat_overrides(args.repeat)
    except argparse.ArgumentTypeError as exc:
        parser.error(str(exc))

    started = time.time()
    tempdir = Path(tempfile.mkdtemp(prefix="leia_strict_guard_"))
    leia_bin = tempdir / "leia"
    results: list[BenchmarkResult]
    try:
        if args.dry_run:
            results = dry_run_results(specs, modes, luajit_bin)
        else:
            build_leia(root, leia_bin)

            total_specs = len(specs)

            def run_spec(index: int, spec: BenchmarkSpec) -> tuple[int, BenchmarkResult]:
                if args.progress:
                    print(
                        f"[strict_guard] {index}/{total_specs} start {spec.benchmark_id}",
                        file=sys.stderr,
                        flush=True,
                    )
                row = BenchmarkResult(spec.name, spec.group, spec.base)
                for mode in modes:
                    row.modes[mode] = run_mode(
                        mode,
                        spec,
                        leia_bin,
                        luajit_bin,
                        args.warmup,
                        args.runs,
                        args.timeout,
                        args.timer_resolution,
                        args.min_sample_seconds,
                        args.max_repeat,
                        args.allow_wall_time,
                        repeat_for(repeat_overrides, mode, spec.name, spec.benchmark_id),
                    )
                if args.progress:
                    print(
                        f"[strict_guard] {index}/{total_specs} done  {spec.benchmark_id}",
                        file=sys.stderr,
                        flush=True,
                    )
                return index, row

            results_by_index: list[BenchmarkResult | None] = [None] * len(specs)
            if args.jobs == 1 or len(specs) <= 1:
                for index, spec in enumerate(specs, start=1):
                    completed_index, row = run_spec(index, spec)
                    results_by_index[completed_index - 1] = row
            else:
                parallel_specs = [
                    (index, spec)
                    for index, spec in enumerate(specs, start=1)
                    if spec.group not in SERIAL_GROUPS
                ]
                serial_specs = [
                    (index, spec)
                    for index, spec in enumerate(specs, start=1)
                    if spec.group in SERIAL_GROUPS
                ]
                with concurrent.futures.ThreadPoolExecutor(max_workers=args.jobs) as executor:
                    future_map = {
                        executor.submit(run_spec, index, spec): index
                        for index, spec in parallel_specs
                    }
                    for future in concurrent.futures.as_completed(future_map):
                        completed_index, row = future.result()
                        results_by_index[completed_index - 1] = row
                for index, spec in serial_specs:
                    completed_index, row = run_spec(index, spec)
                    results_by_index[completed_index - 1] = row
            results = [row for row in results_by_index if row is not None]

        print_brief(results, modes)

        payload = {
            "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "duration_seconds": round(time.time() - started, 3),
            "commit": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip(),
            "platform": {
                "machine": platform.machine(),
                "system": platform.system(),
                "go": subprocess.check_output(["go", "version"], cwd=root, text=True).strip(),
                "luajit": luajit_bin or "",
            },
            "benchmarks": [spec.benchmark_id for spec in specs],
            "benchmark_specs": [
                {
                    "id": spec.benchmark_id,
                    "group": spec.group,
                    "name": spec.name,
                    "leia": str(spec.leia.relative_to(root)),
                    "luajit": str(spec.luajit.relative_to(root)) if spec.luajit else "",
                    "base": spec.base or "",
                }
                for spec in specs
            ],
            "modes": modes,
            "warmup_runs": args.warmup,
            "measured_runs": args.runs,
            "timeout_seconds": args.timeout,
            "timer_resolution_seconds": args.timer_resolution,
            "min_sample_seconds": args.min_sample_seconds,
            "max_repeat": args.max_repeat,
            "allow_wall_time": args.allow_wall_time,
            "dry_run": args.dry_run,
            "results": [asdict(r) for r in results],
        }

        json_path = root / args.json if not args.json.is_absolute() else args.json
        md_path = root / args.markdown if not args.markdown.is_absolute() else args.markdown
        write_json(json_path, payload)
        write_markdown(md_path, markdown_summary(results, modes, args))
        print(f"Wrote JSON: {json_path}")
        print(f"Wrote Markdown: {md_path}")

        bad_statuses = {"error", "timeout"}
        return 1 if any(m.status in bad_statuses for r in results for m in r.modes.values()) else 0
    finally:
        if args.keep_bin:
            print(f"Kept leia binary: {leia_bin}")
        else:
            shutil.rmtree(tempdir, ignore_errors=True)


if __name__ == "__main__":
    raise SystemExit(main())
