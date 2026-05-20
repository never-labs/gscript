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
    "api": FamilyCoverage("partial", ("suite/table_field_access", "suite/table_array_access"), "raw table helpers are covered indirectly; API host-call paths need more hot coverage"),
    "big": FamilyCoverage("covered", ("suite/table_array_access", "suite/object_creation"), "large table/object construction has benchmark coverage"),
    "bwcoercion": FamilyCoverage("partial", ("suite/math_intensive",), "numeric coercion is covered; string-to-bit32 coercion needs focused hot coverage"),
    "bitwise": FamilyCoverage("partial", ("suite/math_intensive",), "numeric hot paths covered; bit32 extraction/rotation helpers need a focused benchmark"),
    "calls": FamilyCoverage("covered", ("suite/method_dispatch", "suite/mutual_recursion", "suite/fib_recursive"), "call, method dispatch, and recursion are covered"),
    "closure": FamilyCoverage("covered", ("suite/closure_bench", "variants/closure_accumulator_variant"), "closure allocation/capture and accumulator hot paths are covered"),
    "code": FamilyCoverage("covered", ("suite/math_intensive", "suite/string_bench", "suite/sum_primes"), "constant/immediate compiler paths are exercised by numeric, string, and branch-heavy benchmarks"),
    "constructs": FamilyCoverage("covered", ("suite/sum_primes", "suite/fannkuch"), "loop/control-flow hot paths are covered"),
    "control": FamilyCoverage("semantic_only", (), "defer/coroutine interaction is primarily semantic; extract a hot workload only if it appears in real programs"),
    "coroutine": FamilyCoverage("covered", ("suite/coroutine_bench", "extended/producer_consumer_pipeline"), "coroutine resume/yield and pipeline paths are covered"),
    "cstack": FamilyCoverage("partial", ("extended/log_tokenize_format",), "pattern stack behavior is covered only indirectly; complex pattern recursion needs a focused benchmark"),
    "errors": FamilyCoverage("semantic_only", (), "error paths are normally cold; track correctness and targeted latency separately"),
    "events": FamilyCoverage("partial", ("suite/method_dispatch", "extended/actors_dispatch_mutation"), "method/index dispatch covered; arithmetic/compare metamethod hot loops need a benchmark"),
    "files": FamilyCoverage("semantic_only", (), "IO depends on host filesystem and is not comparable to LuaJIT as a core VM hot path"),
    "gengc": FamilyCoverage("semantic_only", (), "GC mode controls are semantic/diagnostic checks; allocation pressure is tracked separately"),
    "gc": FamilyCoverage("partial", ("suite/object_creation", "suite/binary_trees"), "allocation pressure covered; collectgarbage/finalization APIs are semantic/host behavior"),
    "heavy": FamilyCoverage("partial", ("suite/string_bench", "extended/log_tokenize_format"), "string pressure covered indirectly; generated concat/string growth deserves a focused hot benchmark"),
    "literals": FamilyCoverage("semantic_only", (), "literal parsing is front-end correctness; not a steady-state runtime hot path"),
    "locals": FamilyCoverage("covered", ("suite/fibonacci_iterative", "suite/sum_primes"), "local-slot integer loops are covered"),
    "math": FamilyCoverage("covered", ("suite/math_intensive", "suite/spectral_norm", "suite/mandelbrot"), "integer, float, transcendental, and loop-heavy math are covered"),
    "nextvar": FamilyCoverage("partial", ("suite/table_array_access", "extended/json_table_walk"), "array/table traversal covered; pairs/next mutation-order variants need hot coverage"),
    "pm": FamilyCoverage("partial", ("suite/string_bench", "extended/log_tokenize_format"), "string search/format covered; Lua pattern capture/gsub/gmatch need focused hot benchmarks"),
    "sort": FamilyCoverage("covered", ("suite/sort", "variants/sort_mixed_numeric"), "numeric and mixed sort hot paths are covered"),
    "strings": FamilyCoverage("partial", ("suite/string_bench", "extended/log_tokenize_format"), "common string ops covered; format/pattern/table.concat edge families need separate hot coverage"),
    "table": FamilyCoverage("covered", ("suite/table_field_access", "suite/table_array_access", "extended/json_table_walk"), "field, array, and nested walk paths are covered"),
    "utf8": FamilyCoverage("partial", ("suite/string_bench",), "byte string work covered; utf8 iterator/validation helpers need hot coverage"),
    "vararg": FamilyCoverage("partial", ("suite/closure_bench",), "call/closure coverage exists; select/pack/unpack adjustment needs a focused benchmark"),
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
    return ids


def hot_hints(paths: list[Path]) -> list[str]:
    hinted: list[str] = []
    for path in paths:
        text = path.read_text(errors="replace")
        if HOT_LOOP_RE.search(text) or len(text.splitlines()) >= 80:
            hinted.append(path.stem)
    return hinted


def coverage_for(prefix: str) -> FamilyCoverage:
    if prefix in COVERAGE:
        return COVERAGE[prefix]
    if prefix in HOST_OR_SEMANTIC_PREFIXES:
        return FamilyCoverage("semantic_only", (), "host integration or short semantic checks; compare correctness unless a hot workload is extracted")
    return FamilyCoverage("missing", (), "no explicit performance coverage classification yet")


def render_markdown(cases: dict[str, list[Path]], known_benchmarks: set[str]) -> str:
    rows: list[tuple[str, int, str, str, str, str]] = []
    summary: dict[str, int] = {}
    missing_refs: set[str] = set()
    for prefix, paths in sorted(cases.items()):
        cov = coverage_for(prefix)
        summary[cov.status] = summary.get(cov.status, 0) + len(paths)
        for bench in cov.benchmarks:
            if bench not in known_benchmarks:
                missing_refs.add(bench)
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
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
