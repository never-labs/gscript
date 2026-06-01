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
    leia: Path
    luajit: Path | None = None
    base: str | None = None

    @property
    def benchmark_id(self) -> str:
        return f"{self.group}/{self.name}"

    @property
    def leia_rel(self) -> str:
        return f"benchmarks/{self.group}/{self.name}.leia"

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


def benchmark_id_from_selector(selector: str, allowed_groups: list[str] | tuple[str, ...] = GROUPS) -> str | None:
    text = selector.removeprefix("benchmarks/")
    if text.endswith(".leia"):
        text = text[:-3]
    parts = text.split("/")
    if len(parts) != 2:
        return None
    group, name = parts
    if group not in allowed_groups or not name:
        return None
    return f"{group}/{name}"


def canonical_group(group: str) -> list[str]:
    return [group]


def group_choices(allowed_groups: list[str] | tuple[str, ...] = GROUPS) -> list[str]:
    return list(allowed_groups)


def selector_matches(selector: str, allowed: set[str]) -> bool:
    benchmark_id = benchmark_id_from_selector(selector)
    return benchmark_id in allowed if benchmark_id is not None else selector in allowed


def spec_selectors(specs: Iterable[SelectableSpec]) -> set[str]:
    return {spec.benchmark_id for spec in specs}


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
                f"{option_name} entries must be DOMAIN/BENCH=N or MODE/DOMAIN/BENCH=N"
            )
        key, raw_count = value.split("=", 1)
        try:
            count = int(raw_count)
        except ValueError as exc:
            raise argparse.ArgumentTypeError(f"invalid count in {value!r}") from exc
        if count <= 0:
            raise argparse.ArgumentTypeError("count must be > 0")
        mode: str | None = None
        selector = key
        if "/" in key:
            head, tail = key.split("/", 1)
            if head in modes_set:
                mode = head
                selector = tail
        benchmark_id = benchmark_id_from_selector(selector)
        if benchmark_id is None:
            raise argparse.ArgumentTypeError(
                f"{option_name} selector must be a domain benchmark id or path: {selector!r}"
            )
        overrides[(mode, benchmark_id)] = count
    return overrides


def selector_count_override(
    overrides: dict[tuple[str | None, str], int],
    mode: str,
    name: str,
    benchmark_id: str | None = None,
) -> int | None:
    if benchmark_id is None:
        return None
    if value := overrides.get((mode, benchmark_id)):
        return value
    if value := overrides.get((None, benchmark_id)):
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
    ordered = [name for name in DEFAULT_ORDER if (bench_dir / f"{name}.leia").exists()]
    ordered_set = set(ordered)
    extras = sorted(path.stem for path in bench_dir.glob("*.leia") if path.stem not in ordered_set)
    specs: list[DiscoveredBenchmark] = []
    for name in [*ordered, *extras]:
        luajit = root / "benchmarks" / "lua_ref" / group / f"{name}.lua"
        specs.append(
            DiscoveredBenchmark(
                group,
                name,
                bench_dir / f"{name}.leia",
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
        selector = benchmark_id_from_selector(raw_selector)
        if selector is None:
            raise SystemExit(f"unknown benchmark selector: {raw_selector}")
        matches = [spec for spec in specs if spec.benchmark_id == selector]
        if not matches:
            raise SystemExit(f"unknown benchmark selector: {raw_selector}")
        for match in matches:
            if match not in selected:
                selected.append(match)
    return selected


def resolve_script_path(root: Path, bench: str, groups: list[str] | tuple[str, ...] = GROUPS) -> Path | None:
    benchmark_id = benchmark_id_from_selector(bench, groups)
    if benchmark_id is None:
        return None
    group, name = benchmark_id.split("/", 1)
    path = root / "benchmarks" / group / f"{name}.leia"
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
