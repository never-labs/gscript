"""Bridge default unittest discovery to this directory's *_test.py files."""

from __future__ import annotations

from pathlib import Path
import unittest


def load_tests(
    loader: unittest.TestLoader,
    standard_tests: unittest.TestSuite,
    pattern: str | None,
) -> unittest.TestSuite:
    del standard_tests, pattern
    here = Path(__file__).resolve().parent
    return loader.discover(str(here), pattern="*_test.py")
