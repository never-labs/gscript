"""Shared parsing helpers for benchmark command output."""

from __future__ import annotations

from dataclasses import dataclass
import os
import re
import subprocess
import sys
import time
from pathlib import Path


@dataclass
class TextCommandResult:
    output: str
    status: str
    exit_code: int | None
    wall_seconds: float

TIME_RE = re.compile(r"^Time:\s*([0-9]+(?:\.[0-9]+)?)s\b", re.MULTILINE)


def parse_time(output: str) -> float | None:
    match = TIME_RE.search(output)
    return float(match.group(1)) if match else None


def parse_counter(pattern: re.Pattern[str], output: str) -> int:
    match = pattern.search(output)
    return int(match.group(1)) if match else 0


def output_tail(output: str, limit: int = 8) -> str:
    lines = [line for line in output.strip().splitlines() if line.strip()]
    return "\n".join(lines[-limit:])


def text_output(output: str | bytes | None) -> str:
    if output is None:
        return ""
    if isinstance(output, bytes):
        return output.decode(errors="replace")
    return output


def run_text_command(cmd: list[str], timeout: float, env: dict[str, str] | None = None) -> TextCommandResult:
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
    except subprocess.TimeoutExpired as exc:
        wall = time.perf_counter() - started
        output = text_output(exc.stdout)
        return TextCommandResult(output + f"\nTIMEOUT after {timeout}s", "timeout", None, wall)

    wall = time.perf_counter() - started
    status = "ok" if proc.returncode == 0 else "error"
    return TextCommandResult(proc.stdout, status, proc.returncode, wall)


def build_gscript(root: Path, out: Path, failure_message: str | None = None) -> None:
    proc = subprocess.run(
        ["go", "build", "-o", str(out), "./cmd/gscript/"],
        cwd=root,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        check=False,
    )
    if proc.returncode != 0:
        print(proc.stdout, file=sys.stderr)
        message = failure_message or f"build failed with exit {proc.returncode}"
        raise SystemExit(message.format(root=root, exit_code=proc.returncode))


def markdown_row(cells: list[object]) -> str:
    return "| " + " | ".join(str(cell) for cell in cells) + " |"


def gscript_mode_command(mode: str, gscript_bin: Path, script: Path) -> tuple[list[str], dict[str, str]]:
    env = os.environ.copy()
    if mode == "vm":
        return [str(gscript_bin), "-vm", str(script)], env
    if mode == "default":
        return [str(gscript_bin), "-jit", "-jit-stats", "-exit-stats", str(script)], env
    if mode == "no_filter":
        env["GSCRIPT_TIER2_NO_FILTER"] = "1"
        return [str(gscript_bin), "-jit", "-jit-stats", "-exit-stats", str(script)], env
    raise ValueError(f"unknown mode: {mode}")


def benchmark_mode_command(
    mode: str,
    gscript_bin: Path | None,
    gscript_script: Path | None,
    luajit_bin: str | None = None,
    luajit_script: Path | None = None,
) -> tuple[list[str] | None, dict[str, str] | None, str | None]:
    if mode == "luajit":
        if luajit_bin is None:
            return None, None, "skipped"
        if luajit_script is None or not luajit_script.exists():
            return None, None, "missing"
        return [luajit_bin, str(luajit_script)], None, None

    if gscript_bin is None or gscript_script is None or not gscript_script.exists():
        return None, None, "missing"
    cmd, env = gscript_mode_command(mode, gscript_bin, gscript_script)
    return cmd, env, None
