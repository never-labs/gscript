#!/usr/bin/env python3
"""Validate LuaJIT reference coverage for domain benchmarks."""

from __future__ import annotations

import argparse
import shutil
import subprocess
import sys
from pathlib import Path

import benchmark_discovery as discovery
import strict_guard as guard


BENCHMARK_GROUPS = tuple(discovery.GROUPS)


def lua_refs(root: Path) -> list[str]:
    refs: list[str] = []
    for group in BENCHMARK_GROUPS:
        for path in sorted((root / "benchmarks" / "lua_ref" / group).glob("*.lua")):
            refs.append(f"{group}/{path.stem}")
    return refs


def validate_ref(root: Path, lua_bin: str, name: str, timeout: int) -> tuple[str, str]:
    if "/" not in name:
        return "missing", f"{name}: expected domain/name"
    group, bench = name.split("/", 1)
    lua_file = root / "benchmarks" / "lua_ref" / group / f"{bench}.lua"
    if not lua_file.exists():
        return "missing", f"{name}: missing {lua_file.relative_to(root)}"

    proc = subprocess.run(
        [lua_bin, str(lua_file)],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        timeout=timeout,
        check=False,
    )
    tail = "\n".join(line for line in proc.stdout.strip().splitlines()[-6:] if line)
    if proc.returncode != 0:
        return "error", f"{name}: exit {proc.returncode}\n{tail}"
    if guard.parse_time(proc.stdout) is None:
        return "no_time", f"{name}: no parseable Time: ...s line\n{tail}"
    return "ok", f"{name}: ok"


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--lua-bin", default=shutil.which("luajit") or "luajit")
    parser.add_argument("--timeout", type=int, default=60)
    args = parser.parse_args(argv)

    root = Path(__file__).resolve().parents[1]
    failures: list[tuple[str, str]] = []

    refs = lua_refs(root)
    for name in refs:
        try:
            status, message = validate_ref(root, args.lua_bin, name, args.timeout)
        except subprocess.TimeoutExpired:
            status = "timeout"
            message = f"{name}: timeout after {args.timeout}s"
        except FileNotFoundError:
            print(f"Lua binary not found: {args.lua_bin}", file=sys.stderr)
            return 2

        print(message)
        if status != "ok":
            failures.append((status, name))

    if failures:
        summary = ", ".join(f"{name}={status}" for status, name in failures)
        print(f"Lua reference validation failed: {summary}", file=sys.stderr)
        return 1

    print(f"Validated {len(refs)} Lua references.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
