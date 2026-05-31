#!/usr/bin/env python3
"""Reliable selected-benchmark timing comparison.

This helper compares the current worktree binary against a clean HEAD snapshot
and LuaJIT. It is meant for performance investigation, not as an optimization
itself: it builds binaries in temporary directories, runs selected benchmarks
with calibrated repeated invocations, and reports median, repeat, CV, and Tier 2
exit counts without trusting low-resolution 0.000/0.001s script timings.
"""

from __future__ import annotations

import argparse
import json
import math
import os
import platform
import re
import shutil
import statistics
import subprocess
import sys
import tarfile
import tempfile
import time
from dataclasses import asdict, dataclass, field
from pathlib import Path


DEFAULT_ORDER = [
    "fib",
    "fib_recursive",
    "sieve",
    "mandelbrot",
    "ackermann",
    "matmul",
    "spectral_norm",
    "nbody",
    "fannkuch",
    "sort",
    "sum_primes",
    "mutual_recursion",
    "method_dispatch",
    "closure_bench",
    "string_bench",
    "binary_trees",
    "table_field_access",
    "table_array_access",
    "coroutine_bench",
    "fibonacci_iterative",
    "math_intensive",
    "object_creation",
]

GROUPS = ["numeric", "recursion", "table", "calls", "string", "concurrency", "data", "app", "control"]
LEGACY_GROUP_ALIASES = {
    "suite": ["numeric", "recursion", "table", "calls", "string", "control"],
    "extended": ["app", "table", "string", "concurrency"],
    "variants": ["recursion", "calls", "numeric", "table"],
    "official": ["calls", "control", "table", "string", "app"],
    "data_oriented": ["data"],
}
LEGACY_BENCH_ALIASES = {
    "recursion/ackermann": "recursion/ackermann",
    "recursion/binary_trees": "recursion/binary_trees",
    "calls/closure_bench": "calls/closure_bench",
    "calls/coroutine_bench": "calls/coroutine_bench",
    "numeric/fannkuch": "numeric/fannkuch",
    "recursion/fib": "recursion/fib",
    "recursion/fib_recursive": "recursion/fib_recursive",
    "recursion/fibonacci_iterative": "recursion/fibonacci_iterative",
    "numeric/mandelbrot": "numeric/mandelbrot",
    "numeric/math_intensive": "numeric/math_intensive",
    "numeric/matmul": "numeric/matmul",
    "numeric/matmul_dense": "numeric/matmul_dense",
    "numeric/matmul_dense_split2": "numeric/matmul_dense_split2",
    "numeric/matmul_dense_tb": "numeric/matmul_dense_tb",
    "numeric/matmul_dense_unroll2": "numeric/matmul_dense_unroll2",
    "calls/method_dispatch": "calls/method_dispatch",
    "recursion/mutual_recursion": "recursion/mutual_recursion",
    "numeric/nbody": "numeric/nbody",
    "numeric/nbody_dense": "numeric/nbody_dense",
    "calls/object_creation": "calls/object_creation",
    "control/sieve": "control/sieve",
    "table/sort": "table/sort",
    "numeric/spectral_norm": "numeric/spectral_norm",
    "numeric/spectral_norm_dense": "numeric/spectral_norm_dense",
    "string/string_bench": "string/string_bench",
    "numeric/sum_primes": "numeric/sum_primes",
    "table/table_array_access": "table/table_array_access",
    "table/table_field_access": "table/table_field_access",
    "app/actors_dispatch_mutation": "app/actors_dispatch_mutation",
    "table/groupby_nested_agg": "table/groupby_nested_agg",
    "table/json_table_walk": "table/json_table_walk",
    "string/log_tokenize_format": "string/log_tokenize_format",
    "app/mixed_inventory_sim": "app/mixed_inventory_sim",
    "concurrency/producer_consumer_pipeline": "concurrency/producer_consumer_pipeline",
    "recursion/ack_nested_shifted": "recursion/ack_nested_shifted",
    "calls/closure_accumulator": "calls/closure_accumulator",
    "numeric/matmul_row": "numeric/matmul_row",
    "table/sort_mixed_numeric": "table/sort_mixed_numeric",
    "calls/call_len_pairs_metamethod": "calls/call_len_pairs_metamethod",
    "calls/calls_vararg_coroutine": "calls/calls_vararg_coroutine",
    "control/defer_protected": "control/defer_protected",
    "table/events_metamethod": "table/events_metamethod",
    "string/math_bit_utf8": "string/math_bit_utf8",
    "table/nextvar_table": "table/nextvar_table",
    "string/regexp_random": "string/regexp_random",
    "app/stdlib_host": "app/stdlib_host",
    "string/strings_patterns": "string/strings_patterns",
    "table/table_sort_proxy": "table/table_sort_proxy",
}
LEGACY_BENCH_ALIASES.update({
    "suite/ackermann": "recursion/ackermann",
    "suite/binary_trees": "recursion/binary_trees",
    "suite/closure_bench": "calls/closure_bench",
    "suite/coroutine_bench": "calls/coroutine_bench",
    "suite/fannkuch": "numeric/fannkuch",
    "suite/fib": "recursion/fib",
    "suite/fib_recursive": "recursion/fib_recursive",
    "suite/fibonacci_iterative": "recursion/fibonacci_iterative",
    "suite/mandelbrot": "numeric/mandelbrot",
    "suite/math_intensive": "numeric/math_intensive",
    "suite/matmul": "numeric/matmul",
    "suite/matmul_dense": "numeric/matmul_dense",
    "suite/matmul_dense_split2": "numeric/matmul_dense_split2",
    "suite/matmul_dense_tb": "numeric/matmul_dense_tb",
    "suite/matmul_dense_unroll2": "numeric/matmul_dense_unroll2",
    "suite/method_dispatch": "calls/method_dispatch",
    "suite/mutual_recursion": "recursion/mutual_recursion",
    "suite/nbody": "numeric/nbody",
    "suite/nbody_dense": "numeric/nbody_dense",
    "suite/object_creation": "calls/object_creation",
    "suite/sieve": "control/sieve",
    "suite/sort": "table/sort",
    "suite/spectral_norm": "numeric/spectral_norm",
    "suite/spectral_norm_dense": "numeric/spectral_norm_dense",
    "suite/string_bench": "string/string_bench",
    "suite/sum_primes": "numeric/sum_primes",
    "suite/table_array_access": "table/table_array_access",
    "suite/table_field_access": "table/table_field_access",
    "extended/actors_dispatch_mutation": "app/actors_dispatch_mutation",
    "extended/groupby_nested_agg": "table/groupby_nested_agg",
    "extended/json_table_walk": "table/json_table_walk",
    "extended/log_tokenize_format": "string/log_tokenize_format",
    "extended/mixed_inventory_sim": "app/mixed_inventory_sim",
    "extended/producer_consumer_pipeline": "concurrency/producer_consumer_pipeline",
    "variants/ack_nested_shifted": "recursion/ack_nested_shifted",
    "variants/closure_accumulator_variant": "calls/closure_accumulator",
    "variants/matmul_row_variant": "numeric/matmul_row",
    "variants/sort_mixed_numeric": "table/sort_mixed_numeric",
    "official/call_len_pairs_metamethod_hot": "calls/call_len_pairs_metamethod",
    "official/calls_vararg_coroutine_hot": "calls/calls_vararg_coroutine",
    "official/defer_protected_hot": "control/defer_protected",
    "official/events_metamethod_hot": "table/events_metamethod",
    "official/math_bit_utf8_hot": "string/math_bit_utf8",
    "official/nextvar_table_hot": "table/nextvar_table",
    "official/regexp_random_hot": "string/regexp_random",
    "official/stdlib_host_hot": "app/stdlib_host",
    "official/strings_patterns_hot": "string/strings_patterns",
    "official/table_sort_proxy_hot": "table/table_sort_proxy",
    "data_oriented/soa_affine_many_hot": "data/soa_affine_many",
    "data_oriented/soa_masked_aggregate_hot": "data/soa_masked_aggregate",
})
MODES = ["default", "vm", "no_filter"]
TIME_SOURCES = ["auto", "script", "wall"]

HOT_SCALE_PROFILE = [
    "recursion/mutual_recursion:REPS=1000000",
    "numeric/matmul_dense_unroll2:N=600",
    "calls/method_dispatch:N=50000000",
    "numeric/spectral_norm:N=2000",
    "table/table_array_access:REPS=4000",
    "calls/coroutine_bench:N1=1000000",
    "calls/coroutine_bench:N2=500000",
    "calls/coroutine_bench:N3=1000000",
    "app/actors_dispatch_mutation:N=15000",
    "app/actors_dispatch_mutation:TICKS=3000",
    "string/log_tokenize_format:PASSES=24",
    "calls/closure_accumulator:INT_REPS=8000000",
    "calls/closure_accumulator:FLOAT_REPS=8000000",
    "calls/call_len_pairs_metamethod:GROUPS=1920",
    "calls/call_len_pairs_metamethod:REPS=1920",
    "calls/calls_vararg_coroutine:N_CALLS=880000",
    "calls/calls_vararg_coroutine:N_CORO=360000",
    "control/defer_protected:PROTECTED_N=720000",
    "control/defer_protected:COROUTINE_N=180000",
    "table/events_metamethod:N=2400000",
    "string/math_bit_utf8:N=720000",
    "table/nextvar_table:SIZE=5200",
    "table/nextvar_table:REPS=180",
    "table/nextvar_table:ALLOC_N=1400",
    "table/nextvar_table:ALLOC_REPS=360",
    "string/regexp_random:N=144000",
    "app/stdlib_host:N=28000",
    "table/table_sort_proxy:N=840",
    "table/table_sort_proxy:PASSES=3000",
    "data/soa_affine_many:STEPS=600",
    "data/soa_masked_aggregate:REPS=2400",
    "data/soa_filter_gather:FILTER_REPS=360",
    "data/soa_filter_gather:GATHER_REPS=360",
]

TIME_RE = re.compile(r"^Time:\s*([0-9]+(?:\.[0-9]+)?)s\b", re.MULTILINE)
T2_ATTEMPTED_RE = re.compile(r"^\s*Tier 2 attempted:\s*([0-9]+)\b", re.MULTILINE)
T2_ENTERED_RE = re.compile(r"^\s*Tier 2 entered:\s*([0-9]+)\s+functions\b", re.MULTILINE)
T2_FAILED_RE = re.compile(r"^\s*Tier 2 failed:\s*([0-9]+)\s+functions\b", re.MULTILINE)
EXIT_TOTAL_RE = re.compile(r"^\s*total exits:\s*([0-9]+)\b", re.MULTILINE)
GS_ASSIGN_RE_TEMPLATE = r"(?m)^(\s*{name}\s*:=\s*)([^\n]+?)(\s*(?://.*)?)$"
SCRIPT_SAMPLE_MARGIN = 1.2
LUA_ASSIGN_RE_TEMPLATE = r"(?m)^(\s*(?:local\s+)?{name}\s*=\s*)([^\n]+?)(\s*(?:--.*)?)$"


@dataclass(frozen=True)
class BenchmarkSpec:
    group: str
    name: str
    gscript_rel: str
    luajit_rel: str | None = None

    @property
    def benchmark_id(self) -> str:
        return f"{self.group}/{self.name}"


@dataclass(frozen=True)
class ScaleOverride:
    selector: str | None
    name: str
    value: str


@dataclass
class CommandRun:
    status: str
    seconds: float | None = None
    wall_seconds: float | None = None
    exit_code: int | None = None
    t2_attempted: int = 0
    t2_entered: int = 0
    t2_failed: int = 0
    exit_total: int = 0
    output_tail: str = ""


@dataclass
class Sample:
    status: str
    seconds: float | None = None
    repeat: int = 1
    source: str = ""
    script_total_seconds: float | None = None
    wall_total_seconds: float | None = None
    note: str = ""
    last_run: CommandRun | None = None


@dataclass
class Stats:
    n: int = 0
    median: float | None = None
    mean: float | None = None
    min: float | None = None
    max: float | None = None
    stdev: float | None = None
    cv_pct: float | None = None
    ci95_low: float | None = None
    ci95_high: float | None = None
    ci95_half_width: float | None = None
    ci95_half_width_pct: float | None = None


@dataclass
class SubjectResult:
    subject: str
    mode: str
    status: str
    repeat: int = 1
    source: str = ""
    stats: Stats = field(default_factory=Stats)
    samples: list[Sample] = field(default_factory=list)
    t2_attempted: int = 0
    t2_entered: int = 0
    t2_failed: int = 0
    exit_total: int = 0
    note: str = ""
    diagnostic: dict[str, object] = field(default_factory=dict)


@dataclass
class BenchmarkResult:
    benchmark: str
    group: str
    scale: dict[str, str] = field(default_factory=dict)
    modes: dict[str, dict[str, SubjectResult]] = field(default_factory=dict)


def parse_time(output: str) -> float | None:
    match = TIME_RE.search(output)
    return float(match.group(1)) if match else None


def parse_counter(pattern: re.Pattern[str], output: str) -> int:
    match = pattern.search(output)
    return int(match.group(1)) if match else 0


def tail(output: str, n: int = 8) -> str:
    lines = [line for line in output.strip().splitlines() if line.strip()]
    return "\n".join(lines[-n:])


def parse_run(output: str, status: str, exit_code: int | None, wall_seconds: float) -> CommandRun:
    seconds = parse_time(output) if status == "ok" else None
    if status == "ok" and seconds is None:
        status = "no_time"
    return CommandRun(
        status=status,
        seconds=seconds,
        wall_seconds=wall_seconds,
        exit_code=exit_code,
        t2_attempted=parse_counter(T2_ATTEMPTED_RE, output),
        t2_entered=parse_counter(T2_ENTERED_RE, output),
        t2_failed=parse_counter(T2_FAILED_RE, output),
        exit_total=parse_counter(EXIT_TOTAL_RE, output),
        output_tail=tail(output),
    )


def run_command(cmd: list[str], timeout: int, env: dict[str, str] | None = None) -> CommandRun:
    started = time.perf_counter()
    try:
        proc = subprocess.run(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            timeout=timeout,
            env=env,
            check=False,
        )
        wall = time.perf_counter() - started
    except subprocess.TimeoutExpired as exc:
        wall = time.perf_counter() - started
        output = exc.stdout or ""
        if isinstance(output, bytes):
            output = output.decode(errors="replace")
        return parse_run(output + f"\nTIMEOUT after {timeout}s", "timeout", None, wall)

    status = "ok" if proc.returncode == 0 else "error"
    return parse_run(proc.stdout, status, proc.returncode, wall)


def summarize_runs(
    runs: list[CommandRun],
    repeat: int,
    timer_resolution: float,
    min_sample_seconds: float,
    wall_fallback: bool,
    time_source: str,
) -> Sample:
    last = runs[-1] if runs else None
    bad = [run for run in runs if run.status not in {"ok", "no_time"}]
    wall_total = sum(run.wall_seconds or 0.0 for run in runs)
    if bad:
        return Sample(
            status=bad[-1].status,
            repeat=repeat,
            wall_total_seconds=wall_total,
            note=f"{len(bad)} of {repeat} command runs failed",
            last_run=last,
        )
    if time_source == "wall":
        if wall_total > 0:
            note = "used high-resolution command wall time; includes process startup overhead"
            if wall_total < min_sample_seconds:
                note += "; sample below --min-sample-seconds"
            return Sample(
                status="ok",
                seconds=wall_total / repeat,
                repeat=repeat,
                source="wall_hr",
                wall_total_seconds=wall_total,
                note=note,
                last_run=last,
            )
        return Sample(status="low_resolution", repeat=repeat, last_run=last)
    if any(run.status == "no_time" for run in runs):
        return Sample(
            status="no_time",
            repeat=repeat,
            wall_total_seconds=wall_total,
            note="no Time: line in command output",
            last_run=last,
        )

    script_total = sum(run.seconds or 0.0 for run in runs)
    if script_total > timer_resolution and script_total >= min_sample_seconds:
        return Sample(
            status="ok",
            seconds=script_total / repeat,
            repeat=repeat,
            source="script_repeat",
            script_total_seconds=script_total,
            wall_total_seconds=wall_total,
            last_run=last,
        )
    if time_source == "script":
        return Sample(
            status="low_resolution",
            repeat=repeat,
            script_total_seconds=script_total,
            wall_total_seconds=wall_total,
            note="script Time: below resolution; increase workload with --scale/--param",
            last_run=last,
        )
    if wall_fallback and wall_total > 0:
        note = "script Time: below resolution; used command wall time"
        note += "; includes process startup overhead"
        if wall_total < min_sample_seconds:
            note += "; sample below --min-sample-seconds"
        return Sample(
            status="ok",
            seconds=wall_total / repeat,
            repeat=repeat,
            source="wall_repeat",
            script_total_seconds=script_total,
            wall_total_seconds=wall_total,
            note=note,
            last_run=last,
        )
    return Sample(
        status="low_resolution",
        repeat=repeat,
        script_total_seconds=script_total,
        wall_total_seconds=wall_total,
        note="increase --max-repeat or keep wall fallback enabled",
        last_run=last,
    )


def run_sample(
    cmd: list[str],
    env: dict[str, str] | None,
    repeat: int,
    timeout: int,
    timer_resolution: float,
    min_sample_seconds: float,
    wall_fallback: bool,
    time_source: str,
) -> Sample:
    runs = [run_command(cmd, timeout, env) for _ in range(repeat)]
    return summarize_runs(runs, repeat, timer_resolution, min_sample_seconds, wall_fallback, time_source)


def sample_big_enough(sample: Sample, min_sample_seconds: float) -> bool:
    if sample.status != "ok":
        return False
    if sample.source == "script_repeat":
        return (sample.script_total_seconds or 0.0) >= min_sample_seconds * SCRIPT_SAMPLE_MARGIN
    if sample.source == "wall_repeat":
        if sample.repeat <= 1:
            return False
        return (sample.wall_total_seconds or 0.0) >= min_sample_seconds
    if sample.source == "wall_hr":
        return (sample.wall_total_seconds or 0.0) >= min_sample_seconds
    return False


def calibrate_repeat(
    cmd: list[str],
    env: dict[str, str] | None,
    timeout: int,
    timer_resolution: float,
    min_sample_seconds: float,
    max_repeat: int,
    wall_fallback: bool,
    min_wall_repeat: int,
    time_source: str,
) -> tuple[int, Sample]:
    repeat = 1
    last: Sample | None = None
    while repeat <= max_repeat:
        last = run_sample(cmd, env, repeat, timeout, timer_resolution, min_sample_seconds, wall_fallback, time_source)
        if (
            time_source == "auto"
            and last.source == "wall_repeat"
            and last.script_total_seconds is not None
            and last.script_total_seconds > 0
            and last.script_total_seconds <= min_sample_seconds
            and repeat < max_repeat
        ):
            repeat *= 2
            continue
        if sample_big_enough(last, min_sample_seconds) and (
            last.source not in {"wall_repeat", "wall_hr"} or repeat >= min_wall_repeat
        ):
            return repeat, last
        if last.status in {"error", "timeout", "no_time"}:
            return repeat, last
        repeat *= 2
    assert last is not None
    return max_repeat, last


def compute_stats(values: list[float]) -> Stats:
    if not values:
        return Stats()
    mean = statistics.fmean(values)
    stdev = statistics.stdev(values) if len(values) > 1 else 0.0
    ci_half = None
    ci_low = None
    ci_high = None
    ci_pct = None
    if len(values) > 1:
        tcrit = t_critical_95(len(values) - 1)
        ci_half = tcrit * stdev / math.sqrt(len(values))
        ci_low = mean - ci_half
        ci_high = mean + ci_half
        ci_pct = (ci_half / mean * 100.0) if mean > 0 else None
    return Stats(
        n=len(values),
        median=statistics.median(values),
        mean=mean,
        min=min(values),
        max=max(values),
        stdev=stdev,
        cv_pct=(stdev / mean * 100.0) if mean > 0 else None,
        ci95_low=ci_low,
        ci95_high=ci_high,
        ci95_half_width=ci_half,
        ci95_half_width_pct=ci_pct,
    )


def t_critical_95(df: int) -> float:
    # Two-sided 95% Student t critical values. Values above 30 are close to z.
    table = {
        1: 12.706,
        2: 4.303,
        3: 3.182,
        4: 2.776,
        5: 2.571,
        6: 2.447,
        7: 2.365,
        8: 2.306,
        9: 2.262,
        10: 2.228,
        11: 2.201,
        12: 2.179,
        13: 2.160,
        14: 2.145,
        15: 2.131,
        16: 2.120,
        17: 2.110,
        18: 2.101,
        19: 2.093,
        20: 2.086,
        21: 2.080,
        22: 2.074,
        23: 2.069,
        24: 2.064,
        25: 2.060,
        26: 2.056,
        27: 2.052,
        28: 2.048,
        29: 2.045,
        30: 2.042,
    }
    if df <= 0:
        return 0.0
    if df <= 30:
        return table[df]
    if df <= 60:
        return 2.000
    if df <= 120:
        return 1.980
    return 1.960


def summarize_subject(subject: str, mode: str, samples: list[Sample], repeat: int) -> SubjectResult:
    ok = [sample for sample in samples if sample.status == "ok" and sample.seconds is not None]
    if not samples:
        status = "missing"
    elif len(ok) == len(samples):
        status = "ok"
    elif ok:
        status = "partial"
    else:
        status = samples[-1].status
    source = ",".join(sorted({sample.source for sample in ok if sample.source}))
    last_run = next((sample.last_run for sample in reversed(samples) if sample.last_run), None)
    return SubjectResult(
        subject=subject,
        mode=mode,
        status=status,
        repeat=repeat,
        source=source,
        stats=compute_stats([sample.seconds for sample in ok if sample.seconds is not None]),
        samples=samples,
        t2_attempted=last_run.t2_attempted if last_run else 0,
        t2_entered=last_run.t2_entered if last_run else 0,
        t2_failed=last_run.t2_failed if last_run else 0,
        exit_total=last_run.exit_total if last_run else 0,
        note="; ".join(sorted({sample.note for sample in samples if sample.note})),
    )


def max_sample_total(samples: list[Sample], attr: str) -> float | None:
    values = [getattr(sample, attr) for sample in samples if getattr(sample, attr) is not None]
    return max(values) if values else None


def low_resolution_samples(samples: list[Sample]) -> list[Sample]:
    return [sample for sample in samples if sample.status == "low_resolution"]


def scale_arg_values(overrides: list[ScaleOverride]) -> list[str]:
    return [f"{override.selector + ':' if override.selector else ''}{override.name}={override.value}" for override in overrides]


def recommended_scale_args(spec: BenchmarkSpec, applied_overrides: list[ScaleOverride]) -> list[str]:
    if applied_overrides:
        return scale_arg_values(applied_overrides)
    profile = scale_overrides_for(spec, parse_scale_overrides(HOT_SCALE_PROFILE))
    return scale_arg_values(profile)


def subject_diagnostic(
    spec: BenchmarkSpec,
    subject: SubjectResult,
    samples: list[Sample],
    applied_overrides: list[ScaleOverride],
    args: argparse.Namespace,
) -> dict[str, object]:
    low_samples = low_resolution_samples(samples)
    wall_timed = source_is_wall_timed(subject)
    diagnostic: dict[str, object] = {
        "repeat": subject.repeat,
        "max_repeat": args.max_repeat,
        "min_sample_seconds": args.min_sample_seconds,
        "timer_resolution": args.timer_resolution,
        "min_wall_repeat": args.min_wall_repeat,
        "wall_fallback": not args.no_wall_fallback,
        "time_source": args.time_source,
        "scale_profile": args.scale_profile,
        "scale": scale_arg_values(applied_overrides),
    }
    script_total = max_sample_total(samples, "script_total_seconds")
    wall_total = max_sample_total(samples, "wall_total_seconds")
    if script_total is not None:
        diagnostic["max_script_total_seconds"] = script_total
    if wall_total is not None:
        diagnostic["max_wall_total_seconds"] = wall_total
    if low_samples:
        scale_args = recommended_scale_args(spec, applied_overrides)
        recommendation: dict[str, object] = {
            "reason": "script Time total is below timer resolution or --min-sample-seconds",
            "min_sample_seconds": max(args.min_sample_seconds, args.timer_resolution * 50.0),
            "max_repeat": max(args.max_repeat, subject.repeat * 2),
            "min_wall_repeat": max(args.min_wall_repeat, 8),
            "time_source": "script",
            "scale": scale_args,
        }
        rerun_args = ["--time-source=script"]
        if scale_args:
            rerun_args.extend(f"--scale {value}" for value in scale_args)
        else:
            recommendation["scale_note"] = "no built-in top-level --scale parameter is available for this benchmark"
        rerun_args.extend(
            [
                f"--min-sample-seconds {recommendation['min_sample_seconds']:.3f}",
                f"--max-repeat {recommendation['max_repeat']}",
                f"--min-wall-repeat {recommendation['min_wall_repeat']}",
            ]
        )
        recommendation["rerun_args"] = " ".join(rerun_args)
        diagnostic["low_resolution"] = recommendation
    elif wall_timed:
        diagnostic["wall_repeat"] = {
            "reason": "sample used repeated command wall time; measurements include process startup overhead",
            "recommendation": "scale workload enough for script_repeat or rerun with --time-source=script",
            "min_wall_repeat": args.min_wall_repeat,
        }
    return diagnostic


def canonical_group(group: str) -> list[str]:
    return LEGACY_GROUP_ALIASES.get(group, [group])


def canonical_selector(selector: str) -> str:
    if selector in LEGACY_BENCH_ALIASES:
        return LEGACY_BENCH_ALIASES[selector]
    if "/" not in selector:
        return selector
    group, name = selector.split("/", 1)
    groups = LEGACY_GROUP_ALIASES.get(group)
    if groups and len(groups) == 1:
        return f"{groups[0]}/{name}"
    return selector


def selector_candidates(selector: str) -> list[str]:
    canonical = canonical_selector(selector)
    out = [canonical]
    if canonical != selector or "/" not in selector:
        return out
    group, name = selector.split("/", 1)
    for canonical_group_name in LEGACY_GROUP_ALIASES.get(group, []):
        candidate = f"{canonical_group_name}/{name}"
        if candidate not in out:
            out.append(candidate)
        hot_candidate = f"{canonical_group_name}/{name.removesuffix('_hot')}"
        if hot_candidate not in out:
            out.append(hot_candidate)
    return out


def selector_matches(selector: str, allowed: set[str]) -> bool:
    return any(candidate in allowed for candidate in selector_candidates(selector))


def canonical_groups(groups: list[str]) -> list[str]:
    out: list[str] = []
    for group in groups:
        for canonical in canonical_group(group):
            if canonical not in GROUPS:
                raise SystemExit(f"unknown benchmark group: {group}")
            if canonical not in out:
                out.append(canonical)
    return out


def discover_specs(root: Path, groups: list[str]) -> list[BenchmarkSpec]:
    specs: list[BenchmarkSpec] = []
    for group in canonical_groups(groups):
        bench_dir = root / "benchmarks" / group
        ordered = [name for name in DEFAULT_ORDER if (bench_dir / f"{name}.gs").exists()]
        extras = sorted(path.stem for path in bench_dir.glob("*.gs") if path.stem not in set(ordered))
        for name in [*ordered, *extras]:
            luajit = root / "benchmarks" / "lua_ref" / group / f"{name}.lua"
            specs.append(
                BenchmarkSpec(
                    group,
                    name,
                    f"benchmarks/{group}/{name}.gs",
                    f"benchmarks/lua_ref/{group}/{name}.lua" if luajit.exists() else None,
                )
            )
    return specs


def select_specs(specs: list[BenchmarkSpec], selectors: list[str] | None) -> list[BenchmarkSpec]:
    if not selectors:
        return specs
    out: list[BenchmarkSpec] = []
    for raw_selector in selectors:
        candidates = selector_candidates(raw_selector)
        matches = [spec for spec in specs if spec.name in candidates or spec.benchmark_id in candidates]
        if not matches:
            raise SystemExit(f"unknown benchmark selector: {raw_selector}")
        if len(matches) > 1 and "/" not in candidates[0]:
            ids = ", ".join(spec.benchmark_id for spec in matches)
            raise SystemExit(f"ambiguous benchmark selector {candidates[0]!r}; use one of: {ids}")
        for match in matches:
            if match not in out:
                out.append(match)
    return out


def parse_scale_overrides(values: list[str] | None) -> list[ScaleOverride]:
    overrides: list[ScaleOverride] = []
    for value in values or []:
        if "=" not in value:
            raise argparse.ArgumentTypeError("--scale entries must be VAR=VALUE or BENCH:VAR=VALUE")
        left, raw_value = value.split("=", 1)
        if not raw_value:
            raise argparse.ArgumentTypeError("--scale value must not be empty")
        selector = None
        name = left
        if ":" in left:
            selector, name = left.split(":", 1)
            if not selector:
                raise argparse.ArgumentTypeError("--scale selector must not be empty")
            selector = canonical_selector(selector)
        if not re.match(r"^[A-Za-z_][A-Za-z0-9_]*$", name):
            raise argparse.ArgumentTypeError(f"invalid scale variable name: {name!r}")
        overrides.append(ScaleOverride(selector, name, raw_value))
    return overrides


def scale_overrides_for(spec: BenchmarkSpec, overrides: list[ScaleOverride]) -> list[ScaleOverride]:
    selectors = {spec.name, spec.benchmark_id}
    return [override for override in overrides if override.selector is None or selector_matches(override.selector, selectors)]


def validate_scale_selectors(specs: list[BenchmarkSpec], overrides: list[ScaleOverride]) -> None:
    selectors = {spec.name for spec in specs} | {spec.benchmark_id for spec in specs}
    for override in overrides:
        if override.selector is not None and not selector_matches(override.selector, selectors):
            raise SystemExit(f"unknown --scale/--param selector: {override.selector}")


def filter_scale_overrides_for_specs(specs: list[BenchmarkSpec], overrides: list[ScaleOverride]) -> list[ScaleOverride]:
    selectors = {spec.name for spec in specs} | {spec.benchmark_id for spec in specs}
    return [override for override in overrides if override.selector is None or selector_matches(override.selector, selectors)]


def format_scale_overrides(overrides: list[ScaleOverride]) -> list[str]:
    out: list[str] = []
    for override in overrides:
        prefix = f"{override.selector}:" if override.selector else ""
        out.append(f"{prefix}{override.name}={override.value}")
    return out


def apply_scale_to_text(text: str, suffix: str, overrides: list[ScaleOverride]) -> tuple[str, dict[str, str]]:
    changes: dict[str, str] = {}
    out = text
    for override in overrides:
        template = LUA_ASSIGN_RE_TEMPLATE if suffix == ".lua" else GS_ASSIGN_RE_TEMPLATE
        pattern = re.compile(template.format(name=re.escape(override.name)))
        matched = pattern.search(out)
        if not matched:
            continue
        old = matched.group(2).strip()
        changes[override.name] = f"{old}->{override.value}"
        out = pattern.sub(lambda m: f"{m.group(1)}{override.value}{m.group(3)}", out, count=1)
    return out, changes


def require_scale_matches(spec: BenchmarkSpec, changes: dict[str, str], overrides: list[ScaleOverride], kind: str) -> None:
    if not overrides:
        return
    missing = [override.name for override in overrides if override.name not in changes]
    if missing:
        names = ", ".join(sorted(set(missing)))
        raise SystemExit(f"{spec.benchmark_id}: {kind} input has no top-level parameter(s): {names}")


def scaled_path(
    root: Path,
    rel: str | None,
    spec: BenchmarkSpec,
    subject: str,
    kind: str,
    tempdir: Path,
    overrides: list[ScaleOverride],
) -> tuple[Path | None, dict[str, str]]:
    if rel is None:
        return None, {}
    src = root / rel
    if not src.exists() or not overrides:
        return src, {}
    text = src.read_text()
    scaled, changes = apply_scale_to_text(text, src.suffix, overrides)
    if not changes:
        return src, {}
    out = tempdir / "scaled" / subject / kind / spec.group / src.name
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(scaled)
    return out, changes


def build_gscript(root: Path, out: Path) -> None:
    proc = subprocess.run(
        ["go", "build", "-o", str(out), "./cmd/gscript/"],
        cwd=root,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    if proc.returncode != 0:
        print(proc.stdout, file=sys.stderr)
        raise SystemExit(f"build failed in {root} with exit {proc.returncode}")


def export_head_snapshot(repo: Path, dest: Path, ref: str) -> None:
    proc = subprocess.run(
        ["git", "archive", "--format=tar", ref],
        cwd=repo,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if proc.returncode != 0:
        print(proc.stderr.decode(errors="replace"), file=sys.stderr)
        raise SystemExit(f"git archive {ref} failed")
    dest.mkdir(parents=True, exist_ok=True)
    tar_path = dest.parent / f"{dest.name}.tar"
    tar_path.write_bytes(proc.stdout)
    try:
        with tarfile.open(tar_path) as tf:
            tf.extractall(dest)
    finally:
        tar_path.unlink(missing_ok=True)


def command_for(
    subject: str,
    mode: str,
    gscript_path: Path | None,
    gscript_bin: Path | None,
    luajit_path: Path | None,
    luajit_bin: str | None,
) -> tuple[list[str] | None, dict[str, str] | None, str | None]:
    if subject == "luajit":
        if luajit_bin is None:
            return None, None, "skipped"
        if luajit_path is None or not luajit_path.exists():
            return None, None, "missing"
        return [luajit_bin, str(luajit_path)], None, None

    if gscript_path is None or not gscript_path.exists():
        return None, None, "missing"
    assert gscript_bin is not None
    env = os.environ.copy()
    if mode == "default":
        return [str(gscript_bin), "-jit", "-jit-stats", "-exit-stats", str(gscript_path)], env, None
    if mode == "vm":
        return [str(gscript_bin), "-vm", str(gscript_path)], env, None
    if mode == "no_filter":
        env["GSCRIPT_TIER2_NO_FILTER"] = "1"
        return [str(gscript_bin), "-jit", "-jit-stats", "-exit-stats", str(gscript_path)], env, None
    raise ValueError(f"unknown mode: {mode}")


def run_subject(
    subject: str,
    mode: str,
    root: Path,
    gscript_bin: Path | None,
    luajit_bin: str | None,
    spec: BenchmarkSpec,
    scale_tempdir: Path,
    scale_overrides: list[ScaleOverride],
    args: argparse.Namespace,
) -> SubjectResult:
    overrides = scale_overrides_for(spec, scale_overrides)
    gscript_path, _gs_changes = scaled_path(root, spec.gscript_rel, spec, subject, "gscript", scale_tempdir, overrides)
    lua_path, _lua_changes = scaled_path(root, spec.luajit_rel, spec, subject, "luajit", scale_tempdir, overrides)
    cmd, env, unavailable = command_for(subject, mode, gscript_path, gscript_bin, lua_path, luajit_bin)
    if unavailable:
        return SubjectResult(subject=subject, mode=mode, status=unavailable, note="input unavailable")
    assert cmd is not None
    repeat, calibration = calibrate_repeat(
        cmd,
        env,
        args.timeout,
        args.timer_resolution,
        args.min_sample_seconds,
        args.max_repeat,
        not args.no_wall_fallback,
        args.min_wall_repeat,
        args.time_source,
    )
    samples = [calibration]
    for _ in range(args.warmup):
        run_sample(
            cmd,
            env,
            repeat,
            args.timeout,
            args.timer_resolution,
            args.min_sample_seconds,
            not args.no_wall_fallback,
            args.time_source,
        )
    for _ in range(args.runs):
        samples.append(
            run_sample(
                cmd,
                env,
                repeat,
                args.timeout,
                args.timer_resolution,
                args.min_sample_seconds,
                not args.no_wall_fallback,
                args.time_source,
            )
        )
    measured = samples[1:] if len(samples) > 1 else samples
    result = summarize_subject(subject, mode, measured, repeat)
    result.diagnostic = subject_diagnostic(spec, result, measured, overrides, args)
    return result


def seconds(result: SubjectResult | None) -> float | None:
    if result is None or result.status not in {"ok", "partial"}:
        return None
    return result.stats.median


def ratio(a: float | None, b: float | None) -> float | None:
    if a is None or b is None or b == 0:
        return None
    return a / b


def fmt_seconds(value: float | None) -> str:
    return "-" if value is None else f"{value:.6f}s"


def fmt_pct(value: float | None) -> str:
    return "-" if value is None else f"{value:.2f}%"


def fmt_ratio(value: float | None) -> str:
    if value is None or math.isnan(value) or math.isinf(value):
        return "-"
    return f"{value:.2f}x"


def result_note(result: SubjectResult | None) -> str:
    if result is None:
        return "-"
    return result.note or "-"


def subject_source(result: SubjectResult | None) -> str:
    if result is None:
        return "-"
    return result.source or "-"


def source_pair(current: SubjectResult | None, luajit: SubjectResult | None) -> str:
    return f"{subject_source(current)}/{subject_source(luajit)}"


def source_is_wall_timed(result: SubjectResult | None) -> bool:
    if result is None:
        return False
    sources = {source.strip() for source in (result.source or "").split(",") if source.strip()}
    return bool(sources & {"wall_repeat", "wall_hr"})


def luajit_gap_is_hot_timed(current: SubjectResult | None, luajit: SubjectResult | None) -> bool:
    # Wall-time fallback is useful as a last resort, but it includes process
    # startup and compilation noise. Keep those rows visible while preventing
    # them from driving optimization priority over script-timed hot workloads.
    return not source_is_wall_timed(current) and not source_is_wall_timed(luajit)


def luajit_hot_ratio(current: SubjectResult | None, luajit: SubjectResult | None) -> float | None:
    # A wall-timed sample includes process startup and compilation overhead;
    # comparing it with LuaJIT's in-script timer can mis-rank hot-loop work.
    # Leave the raw measurements visible, but suppress the ratio until the
    # caller scales the workload enough for comparable script-timed samples.
    if not luajit_gap_is_hot_timed(current, luajit):
        return None
    return ratio(seconds(current), seconds(luajit))


def print_table(results: list[BenchmarkResult], modes: list[str]) -> None:
    header = (
        f"{'Benchmark':<34} {'Scale':<18} {'Mode':<9} {'Current':>12} {'HEAD':>12} {'LuaJIT':>12} "
        f"{'Cur/HEAD':>9} {'Cur/LJ':>9} {'CV cur':>8} {'CI95':>8} {'Repeat':>7} {'Source':<12} {'Exits':>7}"
    )
    print(header)
    print("-" * len(header))
    for row in results:
        for mode in modes:
            current = row.modes.get(mode, {}).get("current")
            head = row.modes.get(mode, {}).get("head")
            luajit = row.modes.get(mode, {}).get("luajit")
            cur_s = seconds(current)
            head_s = seconds(head)
            lj_s = seconds(luajit)
            print(
                f"{row.group + '/' + row.benchmark:<34} {fmt_scale(row.scale):<18} {mode:<9} "
                f"{fmt_seconds(cur_s):>12} {fmt_seconds(head_s):>12} {fmt_seconds(lj_s):>12} "
                f"{fmt_ratio(ratio(cur_s, head_s)):>9} {fmt_ratio(luajit_hot_ratio(current, luajit)):>9} "
                f"{fmt_pct(current.stats.cv_pct if current else None):>8} "
                f"{fmt_pct(current.stats.ci95_half_width_pct if current else None):>8} "
                f"{(current.repeat if current else 0):>7} "
                f"{source_pair(current, luajit):<12} "
                f"{(current.exit_total if current else 0):>7}"
            )


def sorted_results_for_print(results: list[BenchmarkResult], modes: list[str], sort_by: str) -> list[BenchmarkResult]:
    if sort_by != "luajit-gap" or len(modes) != 1:
        return results
    mode = modes[0]

    def key(row: BenchmarkResult) -> float:
        current = row.modes.get(mode, {}).get("current")
        luajit = row.modes.get(mode, {}).get("luajit")
        return luajit_hot_ratio(current, luajit) or -1.0

    return sorted(results, key=key, reverse=True)


def luajit_gap_rows(results: list[BenchmarkResult], modes: list[str]) -> list[tuple[float, bool, BenchmarkResult, str]]:
    rows: list[tuple[float, bool, BenchmarkResult, str]] = []
    for row in results:
        for mode in modes:
            current = row.modes.get(mode, {}).get("current")
            luajit = row.modes.get(mode, {}).get("luajit")
            gap = luajit_hot_ratio(current, luajit)
            if gap is not None:
                rows.append((gap, luajit_gap_is_hot_timed(current, luajit), row, mode))
    return sorted(rows, key=lambda item: (item[1], item[0]), reverse=True)


def fmt_scale(scale: dict[str, str]) -> str:
    if not scale:
        return "-"
    return ", ".join(f"{name}:{value}" for name, value in sorted(scale.items()))


def diagnostic_low_resolution_rows(
    results: list[BenchmarkResult], modes: list[str]
) -> list[tuple[BenchmarkResult, str, str, SubjectResult, dict[str, object]]]:
    out: list[tuple[BenchmarkResult, str, str, SubjectResult, dict[str, object]]] = []
    for row in results:
        for mode in modes:
            for subject_name, subject in row.modes.get(mode, {}).items():
                recommendation = subject.diagnostic.get("low_resolution") if subject.diagnostic else None
                if isinstance(recommendation, dict):
                    out.append((row, mode, subject_name, subject, recommendation))
    return out


def diagnostic_wall_rows(
    results: list[BenchmarkResult], modes: list[str]
) -> list[tuple[BenchmarkResult, str, str, SubjectResult, dict[str, object]]]:
    out: list[tuple[BenchmarkResult, str, str, SubjectResult, dict[str, object]]] = []
    for row in results:
        for mode in modes:
            for subject_name, subject in row.modes.get(mode, {}).items():
                wall = subject.diagnostic.get("wall_repeat") if subject.diagnostic else None
                if isinstance(wall, dict):
                    out.append((row, mode, subject_name, subject, wall))
    return out


def fmt_list(value: object) -> str:
    if isinstance(value, list):
        return ", ".join(str(item) for item in value) if value else "-"
    return str(value) if value else "-"


def markdown(results: list[BenchmarkResult], modes: list[str], args: argparse.Namespace) -> str:
    lines = [
        "# Timing Compare",
        "",
        f"- measured runs: {args.runs}",
        f"- warmup samples: {args.warmup}",
        f"- min sample seconds: {args.min_sample_seconds:.3f}",
        f"- timer resolution floor: {args.timer_resolution:.6f}",
        f"- wall fallback: {'disabled' if args.no_wall_fallback else 'enabled'}",
        f"- time source: {args.time_source}",
        f"- min wall repeat: {args.min_wall_repeat}",
        f"- max repeat: {args.max_repeat}",
        f"- scale profile: {args.scale_profile}",
        f"- scale overrides: {', '.join(args.scale_values) if args.scale_values else 'none'}",
        "",
        "## Measurements",
        "",
        "| Benchmark | Scale | Mode | Current | HEAD | LuaJIT | Current/HEAD | Current/LuaJIT | Repeat | CV current | CI95 current | CV HEAD | Exits current | Source | Note |",
        "|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|",
    ]
    for row in results:
        for mode in modes:
            current = row.modes.get(mode, {}).get("current")
            head = row.modes.get(mode, {}).get("head")
            luajit = row.modes.get(mode, {}).get("luajit")
            cur_s = seconds(current)
            head_s = seconds(head)
            lj_s = seconds(luajit)
            lines.append(
                "| "
                + " | ".join(
                    [
                        f"{row.group}/{row.benchmark}",
                        fmt_scale(row.scale),
                        mode,
                        fmt_seconds(cur_s),
                        fmt_seconds(head_s),
                        fmt_seconds(lj_s),
                        fmt_ratio(ratio(cur_s, head_s)),
                        fmt_ratio(luajit_hot_ratio(current, luajit)),
                        str(current.repeat if current else 0),
                        fmt_pct(current.stats.cv_pct if current else None),
                        fmt_pct(current.stats.ci95_half_width_pct if current else None),
                        fmt_pct(head.stats.cv_pct if head else None),
                        str(current.exit_total if current else 0),
                        source_pair(current, luajit),
                        result_note(current),
                    ]
                )
            + " |"
        )

    low_rows = diagnostic_low_resolution_rows(results, modes)
    if low_rows:
        lines.extend(
            [
                "",
                "## Low-Resolution Diagnostics",
                "",
                "| Benchmark | Mode | Subject | Repeat | Script total | Wall total | Recommended scale | Recommended min sample | Recommended repeat | Rerun args |",
                "|---|---|---|---:|---:|---:|---|---:|---:|---|",
            ]
        )
        for row, mode, subject_name, subject, recommendation in low_rows:
            diagnostic = subject.diagnostic
            lines.append(
                "| "
                + " | ".join(
                    [
                        f"{row.group}/{row.benchmark}",
                        mode,
                        subject_name,
                        str(subject.repeat),
                        fmt_seconds(diagnostic.get("max_script_total_seconds") if isinstance(diagnostic.get("max_script_total_seconds"), float) else None),
                        fmt_seconds(diagnostic.get("max_wall_total_seconds") if isinstance(diagnostic.get("max_wall_total_seconds"), float) else None),
                        fmt_list(recommendation.get("scale")),
                        f"{float(recommendation.get('min_sample_seconds', 0.0)):.3f}s",
                        str(recommendation.get("max_repeat", "-")),
                        str(recommendation.get("rerun_args", "-")),
                    ]
                )
                + " |"
            )

    wall_rows = diagnostic_wall_rows(results, modes)
    if wall_rows:
        lines.extend(
            [
                "",
                "## Wall-Repeat Diagnostics",
                "",
                "| Benchmark | Mode | Subject | Source | Repeat | Min wall repeat | Scale | Note |",
                "|---|---|---|---|---:|---:|---|---|",
            ]
        )
        for row, mode, subject_name, subject, wall in wall_rows:
            lines.append(
                "| "
                + " | ".join(
                    [
                        f"{row.group}/{row.benchmark}",
                        mode,
                        subject_name,
                        subject.source or "-",
                        str(subject.repeat),
                        str(wall.get("min_wall_repeat", "-")),
                        fmt_list(subject.diagnostic.get("scale") if subject.diagnostic else []),
                        str(wall.get("recommendation", "-")),
                    ]
                )
                + " |"
            )

    lines.extend(
        [
            "",
            "## LuaJIT Gap Ranking",
            "",
            "| Rank | Benchmark | Scale | Mode | Current/LuaJIT | Current | LuaJIT | Current CI95 | LuaJIT CI95 | Source |",
            "|---:|---|---|---|---:|---:|---:|---:|---:|---|",
        ]
    )
    for idx, (gap, hot_timed, row, mode) in enumerate(luajit_gap_rows(results, modes), 1):
        current = row.modes.get(mode, {}).get("current")
        luajit = row.modes.get(mode, {}).get("luajit")
        lines.append(
            "| "
            + " | ".join(
                [
                    str(idx),
                    f"{row.group}/{row.benchmark}",
                    fmt_scale(row.scale),
                    mode,
                    fmt_ratio(gap),
                    fmt_seconds(seconds(current)),
                    fmt_seconds(seconds(luajit)),
                    fmt_pct(current.stats.ci95_half_width_pct if current else None),
                    fmt_pct(luajit.stats.ci95_half_width_pct if luajit else None),
                    source_pair(current, luajit) + ("" if hot_timed else " (wall-timed)"),
                ]
            )
            + " |"
        )
    return "\n".join(lines) + "\n"


def positive_int(value: str) -> int:
    parsed = int(value)
    if parsed <= 0:
        raise argparse.ArgumentTypeError("must be > 0")
    return parsed


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bench", action="append", help="benchmark name or group/name; repeatable")
    parser.add_argument("--group", action="append", choices=[*GROUPS, *LEGACY_GROUP_ALIASES.keys()], help="benchmark group; repeatable")
    parser.add_argument("--all-groups", action="store_true", help="run all benchmark groups")
    parser.add_argument("--mode", action="append", choices=MODES, help="GScript mode; repeatable")
    parser.add_argument("--runs", type=positive_int, default=5, help="measured samples after calibration")
    parser.add_argument("--warmup", type=int, default=1, help="warmup samples after calibration")
    parser.add_argument("--timeout", type=positive_int, default=60, help="timeout per command invocation")
    parser.add_argument("--min-sample-seconds", type=float, default=0.050)
    parser.add_argument("--timer-resolution", type=float, default=0.001)
    parser.add_argument(
        "--time-source",
        choices=TIME_SOURCES,
        default="auto",
        help="auto uses script Time when large enough, then wall fallback; script forbids wall fallback; wall uses high-resolution command wall time",
    )
    parser.add_argument("--max-repeat", type=positive_int, default=128)
    parser.add_argument(
        "--min-wall-repeat",
        type=positive_int,
        default=4,
        help="minimum repeat for wall-time fallback cells to reduce process startup noise",
    )
    parser.add_argument("--no-wall-fallback", action="store_true", help="do not use repeated command wall time")
    parser.add_argument("--no-luajit", action="store_true", help="skip LuaJIT")
    parser.add_argument("--head-ref", default="HEAD", help="git ref for clean baseline snapshot")
    parser.add_argument(
        "--same-workload",
        action="store_true",
        help="run the HEAD binary against current worktree benchmark files instead of HEAD snapshot files",
    )
    parser.add_argument(
        "--sort",
        choices=("input", "luajit-gap"),
        default="input",
        help="console row order; luajit-gap is supported when one mode is selected",
    )
    parser.add_argument(
        "--scale",
        action="append",
        help="temporary parameter override: VAR=VALUE or BENCH:VAR=VALUE, e.g. recursion/ackermann:REPS=5000",
    )
    parser.add_argument(
        "--param",
        action="append",
        help="alias for --scale; intended for explicit workload-size parameters",
    )
    parser.add_argument(
        "--scale-profile",
        choices=("none", "hot"),
        default="none",
        help="built-in temporary workload scaling profile for hot-loop measurements",
    )
    parser.add_argument("--json", type=Path, help="write machine-readable result JSON")
    parser.add_argument("--markdown", type=Path, help="write markdown report")
    args = parser.parse_args()

    if args.warmup < 0:
        parser.error("--warmup must be >= 0")
    if args.min_sample_seconds <= 0:
        parser.error("--min-sample-seconds must be > 0")
    if args.timer_resolution < 0:
        parser.error("--timer-resolution must be >= 0")

    root = Path(__file__).resolve().parents[1]
    groups = GROUPS if args.all_groups else (args.group or GROUPS)
    modes = args.mode or ["default"]
    specs = select_specs(discover_specs(root, groups), args.bench)
    try:
        user_scale_overrides = parse_scale_overrides([*(args.scale or []), *(args.param or [])])
        profile_scale_overrides = parse_scale_overrides(HOT_SCALE_PROFILE if args.scale_profile == "hot" else [])
    except argparse.ArgumentTypeError as exc:
        parser.error(str(exc))
    validate_scale_selectors(specs, user_scale_overrides)
    profile_scale_overrides = filter_scale_overrides_for_specs(specs, profile_scale_overrides)
    scale_overrides = [*profile_scale_overrides, *user_scale_overrides]
    args.scale_values = format_scale_overrides(scale_overrides)
    luajit_bin = None if args.no_luajit else shutil.which("luajit")

    tempdir = Path(tempfile.mkdtemp(prefix="gscript_timing_compare_"))
    current_bin = tempdir / "current-gscript"
    head_root = tempdir / "head-src"
    head_bin = tempdir / "head-gscript"
    started = time.time()
    try:
        export_head_snapshot(root, head_root, args.head_ref)
        build_gscript(root, current_bin)
        build_gscript(head_root, head_bin)

        results: list[BenchmarkResult] = []
        for spec in specs:
            spec_scale = scale_overrides_for(spec, scale_overrides)
            row_scale: dict[str, str] = {}
            if spec_scale:
                _, gs_changes = scaled_path(root, spec.gscript_rel, spec, "report", "gscript", tempdir, spec_scale)
                _, lua_changes = scaled_path(root, spec.luajit_rel, spec, "report", "luajit", tempdir, spec_scale)
                require_scale_matches(spec, gs_changes, spec_scale, "GScript")
                if spec.luajit_rel is not None:
                    require_scale_matches(spec, lua_changes, spec_scale, "LuaJIT")
                row_scale.update(gs_changes)
                for name, change in lua_changes.items():
                    row_scale.setdefault(name, change)
            row = BenchmarkResult(spec.name, spec.group, row_scale)
            for mode in modes:
                head_input_root = root if args.same_workload else head_root
                row.modes[mode] = {
                    "current": run_subject("current", mode, root, current_bin, luajit_bin, spec, tempdir, scale_overrides, args),
                    "head": run_subject("head", mode, head_input_root, head_bin, luajit_bin, spec, tempdir, scale_overrides, args),
                    "luajit": run_subject("luajit", mode, root, None, luajit_bin, spec, tempdir, scale_overrides, args),
                }
            results.append(row)

        print_table(sorted_results_for_print(results, modes, args.sort), modes)

        payload = {
            "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "duration_seconds": round(time.time() - started, 3),
            "current_commit": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip(),
            "head_ref": args.head_ref,
            "head_commit": subprocess.check_output(["git", "rev-parse", args.head_ref], cwd=root, text=True).strip(),
            "dirty": bool(subprocess.check_output(["git", "status", "--porcelain"], cwd=root, text=True).strip()),
            "platform": {
                "machine": platform.machine(),
                "system": platform.system(),
                "go": subprocess.check_output(["go", "version"], cwd=root, text=True).strip(),
                "luajit": luajit_bin or "",
            },
            "groups": groups,
            "modes": modes,
            "benchmarks": [spec.benchmark_id for spec in specs],
            "runs": args.runs,
            "warmup": args.warmup,
            "timeout": args.timeout,
            "min_sample_seconds": args.min_sample_seconds,
            "timer_resolution": args.timer_resolution,
            "max_repeat": args.max_repeat,
            "min_wall_repeat": args.min_wall_repeat,
            "wall_fallback": not args.no_wall_fallback,
            "time_source": args.time_source,
            "scale_profile": args.scale_profile,
            "scale": args.scale_values,
            "results": [asdict(row) for row in results],
        }
        if args.json:
            out = root / args.json if not args.json.is_absolute() else args.json
            out.parent.mkdir(parents=True, exist_ok=True)
            out.write_text(json.dumps(payload, indent=2) + "\n")
            print(f"Wrote JSON: {out}")
        if args.markdown:
            out = root / args.markdown if not args.markdown.is_absolute() else args.markdown
            out.parent.mkdir(parents=True, exist_ok=True)
            out.write_text(markdown(results, modes, args))
            print(f"Wrote Markdown: {out}")

        bad = {"error", "timeout"}
        return 1 if any(
            subject.status in bad
            for row in results
            for mode_results in row.modes.values()
            for subject in mode_results.values()
        ) else 0
    finally:
        shutil.rmtree(tempdir, ignore_errors=True)


if __name__ == "__main__":
    raise SystemExit(main())
