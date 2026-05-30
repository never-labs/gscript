#!/usr/bin/env python3
"""Audit performance coverage against language conformance cases.

The conformance cases are correctness tests.  Most are tiny assertions,
so comparing their process wall time against LuaJIT mostly measures startup and
compile overhead.  This audit answers a narrower question: which semantic
families have hot benchmark coverage, and which families still need dedicated
hot-loop benchmarks before we can make broad performance claims.
"""

from __future__ import annotations

import argparse
import json
import re
from dataclasses import dataclass
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
BENCHMARK_GROUPS = ("numeric", "recursion", "table", "calls", "string", "concurrency", "data", "app", "control")


@dataclass(frozen=True)
class FamilyCoverage:
    status: str
    benchmarks: tuple[str, ...]
    note: str


COVERAGE: dict[str, FamilyCoverage] = {
    "api": FamilyCoverage("covered", ("table/table_field_access", "table/table_array_access", "table/nextvar_table"), "raw table helpers and table traversal are covered by hot benchmarks"),
    "big": FamilyCoverage("covered", ("table/table_array_access", "calls/object_creation"), "large table/object construction has benchmark coverage"),
    "bwcoercion": FamilyCoverage("covered", ("numeric/math_intensive", "string/math_bit_utf8"), "numeric and string-to-bit32 coercion hot paths are covered"),
    "bitwise": FamilyCoverage("covered", ("numeric/math_intensive", "string/math_bit_utf8"), "numeric and bit32 helper hot paths are covered"),
    "calls": FamilyCoverage("covered", ("calls/method_dispatch", "recursion/mutual_recursion", "recursion/fib_recursive", "calls/calls_vararg_coroutine", "calls/call_len_pairs_metamethod"), "call, method dispatch, recursion, call adjustment, and __call are covered"),
    "closure": FamilyCoverage("covered", ("calls/closure_bench", "calls/closure_accumulator", "calls/calls_vararg_coroutine"), "closure allocation/capture and accumulator hot paths are covered"),
    "code": FamilyCoverage("covered", ("numeric/math_intensive", "string/string_bench", "numeric/sum_primes"), "constant/immediate compiler paths are exercised by numeric, string, and branch-heavy benchmarks"),
    "constructs": FamilyCoverage("covered", ("numeric/sum_primes", "numeric/fannkuch"), "loop/control-flow hot paths are covered"),
    "control": FamilyCoverage("covered", ("calls/calls_vararg_coroutine", "control/defer_protected"), "coroutine, protected-call, and defer-style cleanup control paths are covered"),
    "coroutine": FamilyCoverage("covered", ("calls/coroutine_bench", "concurrency/producer_consumer_pipeline", "calls/calls_vararg_coroutine"), "coroutine resume/yield and pipeline paths are covered"),
    "cstack": FamilyCoverage("covered", ("string/log_tokenize_format", "string/strings_patterns"), "pattern stack behavior is covered by string/pattern hot benchmarks"),
    "errors": FamilyCoverage("semantic_only", (), "error paths are normally cold; track correctness and targeted latency separately"),
    "defer": FamilyCoverage("covered", ("control/defer_protected",), "defer-style cleanup and protected-call unwinding are covered by a hot benchmark"),
    "events": FamilyCoverage("covered", ("calls/method_dispatch", "app/actors_dispatch_mutation", "table/events_metamethod", "calls/call_len_pairs_metamethod", "table/table_sort_proxy"), "method/index dispatch and metamethod hot loops are covered"),
    "files": FamilyCoverage("semantic_only", (), "IO depends on host filesystem and is not comparable to LuaJIT as a core VM hot path"),
    "gengc": FamilyCoverage("semantic_only", (), "GC mode controls are semantic/diagnostic checks; allocation pressure is tracked separately"),
    "gc": FamilyCoverage("covered", ("calls/object_creation", "recursion/binary_trees", "table/nextvar_table"), "allocation pressure and table churn are covered; collectgarbage/finalization APIs remain semantic/host behavior"),
    "heavy": FamilyCoverage("covered", ("string/string_bench", "string/log_tokenize_format", "string/strings_patterns"), "string pressure, generated concat, and string growth are covered"),
    "literals": FamilyCoverage("semantic_only", (), "literal parsing is front-end correctness; not a steady-state runtime hot path"),
    "locals": FamilyCoverage("covered", ("recursion/fibonacci_iterative", "numeric/sum_primes"), "local-slot integer loops are covered"),
    "math": FamilyCoverage("covered", ("numeric/math_intensive", "numeric/spectral_norm", "numeric/mandelbrot", "string/math_bit_utf8"), "integer, float, transcendental, conversion, and loop-heavy math are covered"),
    "nextvar": FamilyCoverage("covered", ("table/table_array_access", "table/json_table_walk", "table/nextvar_table", "calls/call_len_pairs_metamethod"), "array/table traversal and pairs/next mutation-order variants are covered"),
    "pm": FamilyCoverage("covered", ("string/string_bench", "string/log_tokenize_format", "string/strings_patterns"), "string search/format and Lua pattern capture/gsub/gmatch are covered"),
    "regexp": FamilyCoverage("covered", ("string/regexp_random",), "Go regexp compile/match/split hot paths are covered separately from Lua pattern matching"),
    "sort": FamilyCoverage("covered", ("table/sort", "table/sort_mixed_numeric", "table/table_sort_proxy"), "numeric, mixed, and proxy table sort hot paths are covered"),
    "strings": FamilyCoverage("covered", ("string/string_bench", "string/log_tokenize_format", "string/strings_patterns"), "common string ops plus format/pattern/table.concat edge families are covered"),
    "table": FamilyCoverage("covered", ("table/table_field_access", "table/table_array_access", "table/json_table_walk", "table/nextvar_table", "table/table_sort_proxy"), "field, array, nested walk, mutation traversal, table.move, and proxy table paths are covered"),
    "utf8": FamilyCoverage("covered", ("string/string_bench", "string/math_bit_utf8"), "byte string work and utf8 iterator/validation helpers are covered"),
    "vararg": FamilyCoverage("covered", ("calls/closure_bench", "calls/calls_vararg_coroutine"), "call/closure and select/pack/unpack adjustment hot paths are covered"),
}


HOST_OR_SEMANTIC_PREFIXES = {
    "all",
    "attrib",
    "base64",
    "binary",
    "bits",
    "bytes",
    "compress",
    "container",
    "crypto",
    "csv",
    "db",
    "debug",
    "defer",
    "encoding",
    "fs",
    "go",
    "goto",
    "http",
    "io",
    "json",
    "log",
    "main",
    "matrix",
    "net",
    "os",
    "process",
    "rand",
    "regexp",
    "time",
    "tracegc",
    "tpack",
    "url",
    "uuid",
    "vec",
    "verybig",
    "xpcall",
}


HOT_LOOP_RE = re.compile(r"\bfor\b[^\n]*(?:1000|10000|100000|1e4)\b")


def conformance_cases(root: Path) -> dict[str, list[Path]]:
    cases: dict[str, list[Path]] = {}
    for path in sorted((root / "tests" / "language").glob("*.lua")):
        prefix = path.stem.split("_", 1)[0]
        cases.setdefault(prefix, []).append(path)
    return cases


def benchmark_ids(root: Path) -> set[str]:
    ids: set[str] = set()
    for group in BENCHMARK_GROUPS:
        for path in (root / "benchmarks" / group).glob("*.gs"):
            ids.add(f"{group}/{path.stem}")
    return ids


def hot_hints(paths: list[Path]) -> list[str]:
    hinted: list[str] = []
    for path in paths:
        text = path.read_text(errors="replace")
        if HOT_LOOP_RE.search(text) or len(text.splitlines()) >= 80:
            hinted.append(path.stem)
    return hinted


def missing_benchmark_refs(cases: dict[str, list[Path]], known_benchmarks: set[str]) -> set[str]:
    missing_refs: set[str] = set()
    for prefix in cases:
        cov = coverage_for(prefix)
        for bench in cov.benchmarks:
            if bench not in known_benchmarks:
                missing_refs.add(bench)
    return missing_refs


def missing_coverage_families(cases: dict[str, list[Path]]) -> list[str]:
    return sorted(prefix for prefix in cases if coverage_for(prefix).status == "missing")


def coverage_for(prefix: str) -> FamilyCoverage:
    if prefix in COVERAGE:
        return COVERAGE[prefix]
    if prefix in HOST_OR_SEMANTIC_PREFIXES:
        return FamilyCoverage("semantic_only", (), "host integration or short semantic checks; compare correctness unless a hot workload is extracted")
    return FamilyCoverage("missing", (), "no explicit performance coverage classification yet")


def render_markdown(cases: dict[str, list[Path]], known_benchmarks: set[str]) -> str:
    rows: list[tuple[str, int, str, str, str, str]] = []
    summary: dict[str, int] = {}
    missing_refs = missing_benchmark_refs(cases, known_benchmarks)
    for prefix, paths in sorted(cases.items()):
        cov = coverage_for(prefix)
        summary[cov.status] = summary.get(cov.status, 0) + len(paths)
        hot = hot_hints(paths)
        rows.append((
            prefix,
            len(paths),
            cov.status,
            ", ".join(cov.benchmarks) or "-",
            ", ".join(hot[:6]) + (" ..." if len(hot) > 6 else "") if hot else "-",
            cov.note,
        ))

    lines = [
        "# Language Conformance Performance Coverage",
        "",
        "This report maps language conformance correctness cases to hot-loop performance coverage.",
        "Short semantic cases are not treated as LuaJIT comparisons because process wall time would be dominated by startup noise.",
        "",
        "## Summary",
        "",
    ]
    for status in sorted(summary):
        lines.append(f"- `{status}`: {summary[status]} cases")
    if missing_refs:
        lines.append(f"- missing benchmark references in map: {', '.join(sorted(missing_refs))}")
    lines.extend([
        "",
        "## Families",
        "",
        "| Family | Cases | Status | Existing Hot Benchmarks | Hot Candidates | Note |",
        "|---|---:|---|---|---|---|",
    ])
    for prefix, count, status, benches, hot, note in rows:
        lines.append(f"| `{prefix}` | {count} | `{status}` | {benches} | {hot} | {note} |")
    lines.append("")
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--json", type=Path)
    parser.add_argument("--markdown", type=Path)
    parser.add_argument("--check", action="store_true", help="fail when a case family lacks classification or a mapped benchmark is missing")
    args = parser.parse_args()

    cases = conformance_cases(ROOT)
    known = benchmark_ids(ROOT)
    report = render_markdown(cases, known)

    if args.markdown:
        args.markdown.parent.mkdir(parents=True, exist_ok=True)
        args.markdown.write_text(report)
    else:
        print(report)

    if args.json:
        payload = []
        for prefix, paths in sorted(cases.items()):
            cov = coverage_for(prefix)
            payload.append({
                "family": prefix,
                "case_count": len(paths),
                "status": cov.status,
                "benchmarks": list(cov.benchmarks),
                "hot_candidates": hot_hints(paths),
                "note": cov.note,
            })
        args.json.parent.mkdir(parents=True, exist_ok=True)
        args.json.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n")

    if args.check:
        missing_families = missing_coverage_families(cases)
        missing_refs = missing_benchmark_refs(cases, known)
        if missing_families or missing_refs:
            if missing_families:
                print("missing coverage families: " + ", ".join(missing_families))
            if missing_refs:
                print("missing benchmark references: " + ", ".join(sorted(missing_refs)))
            return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
