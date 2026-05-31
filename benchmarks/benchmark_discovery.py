"""Shared benchmark discovery and selector helpers."""

from __future__ import annotations

import argparse
from dataclasses import dataclass
from pathlib import Path
from typing import Protocol, TypeVar


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

DOMAIN_GROUPS = ["numeric", "recursion", "table", "calls", "string", "concurrency", "data", "app", "control", "precision"]
GROUPS = DOMAIN_GROUPS

RELATED_BENCHMARK_BASES = {
    "ack_nested_shifted": "ackermann",
    "sort_mixed_numeric": "sort",
    "matmul_row": "matmul",
    "closure_accumulator": "closure_bench",
}


@dataclass(frozen=True)
class DiscoveredBenchmark:
    group: str
    name: str
    gscript: Path
    luajit: Path | None = None
    base: str | None = None

    @property
    def benchmark_id(self) -> str:
        return f"{self.group}/{self.name}"

    @property
    def gscript_rel(self) -> str:
        return f"benchmarks/{self.group}/{self.name}.gs"

    @property
    def luajit_rel(self) -> str | None:
        if self.luajit is None:
            return None
        return f"benchmarks/lua_ref/{self.group}/{self.name}.lua"


class SelectableSpec(Protocol):
    name: str

    @property
    def benchmark_id(self) -> str: ...


SpecT = TypeVar("SpecT", bound=SelectableSpec)


def canonical_group(group: str) -> list[str]:
    return [group]


def group_choices(allowed_groups: list[str] | tuple[str, ...] = GROUPS) -> list[str]:
    return list(allowed_groups)


def canonical_selector(selector: str) -> str:
    return selector


def selector_candidates(selector: str) -> list[str]:
    return [canonical_selector(selector)]


def selector_matches(selector: str, allowed: set[str]) -> bool:
    return any(candidate in allowed for candidate in selector_candidates(selector))


def spec_selectors(specs: Iterable[SelectableSpec]) -> set[str]:
    selectors: set[str] = set()
    for spec in specs:
        selectors.add(spec.name)
        selectors.add(spec.benchmark_id)
    return selectors


def spec_selector_set(spec: SelectableSpec) -> set[str]:
    return spec_selectors([spec])


def selector_matches_spec(selector: str, spec: SelectableSpec) -> bool:
    return selector_matches(selector, spec_selector_set(spec))


def parse_selector_count_overrides(
    values: list[str] | None,
    modes: list[str] | tuple[str, ...],
    option_name: str,
) -> dict[tuple[str | None, str], int]:
    overrides: dict[tuple[str | None, str], int] = {}
    modes_set = set(modes)
    for value in values or []:
        if "=" not in value:
            raise argparse.ArgumentTypeError(
                f"{option_name} entries must be BENCH=N, GROUP/BENCH=N, or MODE/BENCH=N"
            )
        key, raw_count = value.split("=", 1)
        try:
            count = int(raw_count)
        except ValueError as exc:
            raise argparse.ArgumentTypeError(f"invalid count in {value!r}") from exc
        if count <= 0:
            raise argparse.ArgumentTypeError("count must be > 0")
        if "/" in key:
            head, tail = key.split("/", 1)
            if head in modes_set:
                for selector in selector_candidates(tail):
                    overrides[(head, selector)] = count
            else:
                for selector in selector_candidates(key):
                    overrides[(None, selector)] = count
        else:
            overrides[(None, key)] = count
    return overrides


def selector_count_override(
    overrides: dict[tuple[str | None, str], int],
    mode: str,
    name: str,
    benchmark_id: str | None = None,
) -> int | None:
    selectors = [name]
    if benchmark_id:
        selectors.insert(0, benchmark_id)
    for selector in selectors:
        if value := overrides.get((mode, selector)):
            return value
    for selector in selectors:
        if value := overrides.get((None, selector)):
            return value
    return None


def canonical_groups(groups: list[str], allowed_groups: list[str] | tuple[str, ...] = GROUPS) -> list[str]:
    allowed = set(allowed_groups)
    out: list[str] = []
    for group in groups:
        for canonical in canonical_group(group):
            if canonical not in allowed:
                raise SystemExit(f"unknown benchmark group: {group}")
            if canonical not in out:
                out.append(canonical)
    return out


def domain_specs(root: Path, group: str) -> list[DiscoveredBenchmark]:
    bench_dir = root / "benchmarks" / group
    ordered = [name for name in DEFAULT_ORDER if (bench_dir / f"{name}.gs").exists()]
    ordered_set = set(ordered)
    extras = sorted(path.stem for path in bench_dir.glob("*.gs") if path.stem not in ordered_set)
    specs: list[DiscoveredBenchmark] = []
    for name in [*ordered, *extras]:
        luajit = root / "benchmarks" / "lua_ref" / group / f"{name}.lua"
        specs.append(
            DiscoveredBenchmark(
                group,
                name,
                bench_dir / f"{name}.gs",
                luajit if luajit.exists() else None,
                RELATED_BENCHMARK_BASES.get(name),
            )
        )
    return specs


def discover_benchmarks(root: Path, groups: list[str]) -> list[DiscoveredBenchmark]:
    specs: list[DiscoveredBenchmark] = []
    for group in canonical_groups(groups):
        specs.extend(domain_specs(root, group))
    return specs


def groups_for_selectors(
    root: Path,
    groups: list[str] | tuple[str, ...] | None,
    selectors: list[str] | tuple[str, ...] | None,
    allowed_groups: list[str] | tuple[str, ...] = GROUPS,
) -> list[str]:
    out = canonical_groups(list(allowed_groups if groups is None else groups), allowed_groups)
    for selector in selectors or []:
        identity = resolve_script_identity(root, selector, allowed_groups)
        if identity is None:
            continue
        group = identity[0]
        if group not in out:
            out.append(group)
    return out


def groups_for_selection(
    root: Path,
    groups: list[str] | tuple[str, ...] | None,
    selectors: list[str] | tuple[str, ...] | None,
    all_groups: bool,
    allowed_groups: list[str] | tuple[str, ...] = GROUPS,
) -> list[str]:
    if all_groups:
        return list(allowed_groups)
    return groups_for_selectors(root, groups, selectors, allowed_groups)


def select_specs(specs: list[SpecT], selectors: list[str] | None) -> list[SpecT]:
    if not selectors:
        return specs
    selected: list[SpecT] = []
    for raw_selector in selectors:
        candidates = selector_candidates(raw_selector)
        matches = [spec for spec in specs if spec.benchmark_id in candidates or spec.name in candidates]
        if not matches:
            raise SystemExit(f"unknown benchmark selector: {raw_selector}")
        if len(matches) > 1 and "/" not in candidates[0]:
            ids = ", ".join(spec.benchmark_id for spec in matches)
            raise SystemExit(f"ambiguous benchmark selector {candidates[0]!r}; use one of: {ids}")
        for match in matches:
            if match not in selected:
                selected.append(match)
    return selected


def resolve_script_path(root: Path, bench: str, groups: list[str] | tuple[str, ...] = GROUPS) -> Path | None:
    candidates = selector_candidates(bench) if "/" in bench else [bench]
    for candidate in candidates:
        if "/" in candidate:
            group, name = candidate.split("/", 1)
            search_groups = [group]
        else:
            name = candidate
            search_groups = list(groups)
        for group in search_groups:
            path = root / "benchmarks" / group / f"{name}.gs"
            if path.exists():
                return path
    return None


def resolve_script_identity(root: Path, bench: str, groups: list[str] | tuple[str, ...] = GROUPS) -> tuple[str, str, Path] | None:
    path = resolve_script_path(root, bench, groups)
    if path is None:
        return None
    group = path.parent.name
    if group not in groups:
        return None
    return group, path.stem, path
