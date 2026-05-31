"""Shared parsing helpers for benchmark command output."""

from __future__ import annotations

import re

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
