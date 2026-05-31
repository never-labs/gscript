import re
import unittest
from pathlib import Path

import benchmark_discovery as discovery


ROOT = Path(__file__).resolve().parents[1]


class ScriptEntrypointConsistencyTest(unittest.TestCase):
    def test_diag_shell_domains_match_shared_discovery(self):
        diag = (ROOT / "scripts" / "diag.sh").read_text()
        match = re.search(r"for d in ([^;]+); do\s+if \[ -d \"benchmarks/\$d\" \]", diag)
        self.assertIsNotNone(match)

        shell_domains = match.group(1).split()
        self.assertEqual(shell_domains, discovery.GROUPS)

    def test_diag_shell_legacy_group_aliases_match_shared_discovery(self):
        diag = (ROOT / "scripts" / "diag.sh").read_text()
        cases = dict(
            re.findall(
                r"^\s+([a-z_]+)\) printf '%s\\n' ([^;]+) ;;$",
                diag,
                re.MULTILINE,
            )
        )
        shell_aliases = {
            name: groups.split()
            for name, groups in cases.items()
            if name in discovery.LEGACY_GROUP_ALIASES
        }

        self.assertEqual(shell_aliases, discovery.LEGACY_GROUP_ALIASES)

    def test_benchmark_shell_wrappers_exec_matching_python(self):
        for stem in ("regression_guard", "strict_guard"):
            with self.subTest(stem=stem):
                wrapper = (ROOT / "benchmarks" / f"{stem}.sh").read_text()
                self.assertIn(f'exec python3 benchmarks/{stem}.py "$@"', wrapper)

    def test_scripts_performance_gate_wraps_benchmark_python_tools(self):
        gate = (ROOT / "scripts" / "performance_gate.sh").read_text()
        self.assertIn("python3 benchmarks/timing_compare.py", gate)
        self.assertIn("python3 benchmarks/strict_guard.py", gate)


if __name__ == "__main__":
    unittest.main()
