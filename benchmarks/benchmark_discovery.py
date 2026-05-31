"""Shared benchmark discovery and selector helpers."""

from __future__ import annotations

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

VARIANT_BASES = {
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
                VARIANT_BASES.get(name),
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
    out = canonical_groups(list(groups or allowed_groups), allowed_groups)
    for selector in selectors or []:
        path = resolve_script_path(root, selector, allowed_groups)
        if path is None:
            continue
        group = path.parent.name
        if group in allowed_groups and group not in out:
            out.append(group)
    return out


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
        variant_name = name.removesuffix("_variant")
        if variant_name != name:
            for group in search_groups:
                path = root / "benchmarks" / group / f"{variant_name}.gs"
                if path.exists():
                    return path
        hot_name = name.removesuffix("_hot")
        if hot_name != name:
            for group in search_groups:
                path = root / "benchmarks" / group / f"{hot_name}.gs"
                if path.exists():
                    return path
    return None
