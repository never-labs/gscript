#!/usr/bin/env python3
"""Audit performance coverage against translated official Lua cases.

The translated official cases are correctness tests.  Most are tiny assertions,
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


@dataclass(frozen=True)
class FamilyCoverage:
    status: str
    benchmarks: tuple[str, ...]
    note: str


COVERAGE: dict[str, FamilyCoverage] = {
    "api": FamilyCoverage("covered", ("suite/table_field_access", "suite/table_array_access", "official/nextvar_table_hot"), "raw table helpers and table traversal are covered by hot benchmarks"),
    "big": FamilyCoverage("covered", ("suite/table_array_access", "suite/object_creation"), "large table/object construction has benchmark coverage"),
    "bwcoercion": FamilyCoverage("covered", ("suite/math_intensive", "official/math_bit_utf8_hot"), "numeric and string-to-bit32 coercion hot paths are covered"),
    "bitwise": FamilyCoverage("covered", ("suite/math_intensive", "official/math_bit_utf8_hot"), "numeric and bit32 helper hot paths are covered"),
    "calls": FamilyCoverage("covered", ("suite/method_dispatch", "suite/mutual_recursion", "suite/fib_recursive", "official/calls_vararg_coroutine_hot", "official/call_len_pairs_metamethod_hot"), "call, method dispatch, recursion, call adjustment, and __call are covered"),
    "closure": FamilyCoverage("covered", ("suite/closure_bench", "variants/closure_accumulator_variant", "official/calls_vararg_coroutine_hot"), "closure allocation/capture and accumulator hot paths are covered"),
    "code": FamilyCoverage("covered", ("suite/math_intensive", "suite/string_bench", "suite/sum_primes"), "constant/immediate compiler paths are exercised by numeric, string, and branch-heavy benchmarks"),
    "constructs": FamilyCoverage("covered", ("suite/sum_primes", "suite/fannkuch"), "loop/control-flow hot paths are covered"),
    "control": FamilyCoverage("covered", ("official/calls_vararg_coroutine_hot", "official/defer_protected_hot"), "coroutine, protected-call, and defer-style cleanup control paths are covered"),
    "coroutine": FamilyCoverage("covered", ("suite/coroutine_bench", "extended/producer_consumer_pipeline", "official/calls_vararg_coroutine_hot"), "coroutine resume/yield and pipeline paths are covered"),
    "cstack": FamilyCoverage("covered", ("extended/log_tokenize_format", "official/strings_patterns_hot"), "pattern stack behavior is covered by string/pattern hot benchmarks"),
    "errors": FamilyCoverage("semantic_only", (), "error paths are normally cold; track correctness and targeted latency separately"),
    "defer": FamilyCoverage("covered", ("official/defer_protected_hot",), "defer-style cleanup and protected-call unwinding are covered by a hot benchmark"),
    "events": FamilyCoverage("covered", ("suite/method_dispatch", "extended/actors_dispatch_mutation", "official/events_metamethod_hot", "official/call_len_pairs_metamethod_hot", "official/table_sort_proxy_hot"), "method/index dispatch and metamethod hot loops are covered"),
    "files": FamilyCoverage("semantic_only", (), "IO depends on host filesystem and is not comparable to LuaJIT as a core VM hot path"),
    "gengc": FamilyCoverage("semantic_only", (), "GC mode controls are semantic/diagnostic checks; allocation pressure is tracked separately"),
    "gc": FamilyCoverage("covered", ("suite/object_creation", "suite/binary_trees", "official/nextvar_table_hot"), "allocation pressure and table churn are covered; collectgarbage/finalization APIs remain semantic/host behavior"),
    "heavy": FamilyCoverage("covered", ("suite/string_bench", "extended/log_tokenize_format", "official/strings_patterns_hot"), "string pressure, generated concat, and string growth are covered"),
    "literals": FamilyCoverage("semantic_only", (), "literal parsing is front-end correctness; not a steady-state runtime hot path"),
    "locals": FamilyCoverage("covered", ("suite/fibonacci_iterative", "suite/sum_primes"), "local-slot integer loops are covered"),
    "math": FamilyCoverage("covered", ("suite/math_intensive", "suite/spectral_norm", "suite/mandelbrot", "official/math_bit_utf8_hot"), "integer, float, transcendental, conversion, and loop-heavy math are covered"),
    "nextvar": FamilyCoverage("covered", ("suite/table_array_access", "extended/json_table_walk", "official/nextvar_table_hot", "official/call_len_pairs_metamethod_hot"), "array/table traversal and pairs/next mutation-order variants are covered"),
    "pm": FamilyCoverage("covered", ("suite/string_bench", "extended/log_tokenize_format", "official/strings_patterns_hot"), "string search/format and Lua pattern capture/gsub/gmatch are covered"),
    "regexp": FamilyCoverage("covered", ("official/regexp_random_hot",), "Go regexp compile/match/split hot paths are covered separately from Lua pattern matching"),
    "sort": FamilyCoverage("covered", ("suite/sort", "variants/sort_mixed_numeric", "official/table_sort_proxy_hot"), "numeric, mixed, and proxy table sort hot paths are covered"),
    "strings": FamilyCoverage("covered", ("suite/string_bench", "extended/log_tokenize_format", "official/strings_patterns_hot"), "common string ops plus format/pattern/table.concat edge families are covered"),
    "table": FamilyCoverage("covered", ("suite/table_field_access", "suite/table_array_access", "extended/json_table_walk", "official/nextvar_table_hot", "official/table_sort_proxy_hot"), "field, array, nested walk, mutation traversal, table.move, and proxy table paths are covered"),
    "utf8": FamilyCoverage("covered", ("suite/string_bench", "official/math_bit_utf8_hot"), "byte string work and utf8 iterator/validation helpers are covered"),
    "vararg": FamilyCoverage("covered", ("suite/closure_bench", "official/calls_vararg_coroutine_hot"), "call/closure and select/pack/unpack adjustment hot paths are covered"),
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


def official_cases(root: Path) -> dict[str, list[Path]]:
    cases: dict[str, list[Path]] = {}
    for path in sorted((root / "tests" / "official_lua_cases").glob("*.lua")):
        prefix = path.stem.split("_", 1)[0]
        cases.setdefault(prefix, []).append(path)
    return cases


def benchmark_ids(root: Path) -> set[str]:
    ids: set[str] = set()
    for group in ("suite", "extended", "variants"):
        for path in (root / "benchmarks" / group).glob("*.gs"):
            ids.add(f"{group}/{path.stem}")
    for path in (root / "benchmarks" / "official_hot").glob("*.gs"):
        ids.add(f"official/{path.stem}")
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
        "# Official Lua Case Performance Coverage",
        "",
        "This report maps translated official Lua correctness cases to hot-loop performance coverage.",
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

    cases = official_cases(ROOT)
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
