import re
import sys
import unittest
from pathlib import Path
from unittest import mock

import benchmark_output as output


class BenchmarkOutputTest(unittest.TestCase):
    def test_parse_time_reads_flat_script_time(self):
        self.assertEqual(output.parse_time("result\nTime: 0.123s\n"), 0.123)
        self.assertIsNone(output.parse_time("inner: 0.123s\n"))

    def test_parse_counter_defaults_to_zero(self):
        pattern = re.compile(r"^count:\s*([0-9]+)$", re.MULTILINE)
        self.assertEqual(output.parse_counter(pattern, "count: 42\n"), 42)
        self.assertEqual(output.parse_counter(pattern, "missing\n"), 0)

    def test_output_tail_keeps_nonempty_lines(self):
        self.assertEqual(output.output_tail("\na\n\nb\nc\n", 2), "b\nc")

    def test_text_output_normalizes_subprocess_payloads(self):
        self.assertEqual(output.text_output(None), "")
        self.assertEqual(output.text_output("ok"), "ok")
        self.assertEqual(output.text_output(b"bad:\xff"), "bad:\ufffd")

    def test_markdown_row_formats_cells_without_changing_payloads(self):
        self.assertEqual(output.markdown_row(["a/b", 3, "x | y"]), "| a/b | 3 | x | y |")

    def test_markdown_section_formats_plain_section(self):
        self.assertEqual(output.markdown_section("Diagnostics"), ["", "## Diagnostics", ""])

    def test_markdown_section_formats_table_header(self):
        self.assertEqual(
            output.markdown_section("Measurements", "| A | B |", "|---|---:|"),
            ["", "## Measurements", "", "| A | B |", "|---|---:|"],
        )

    def test_run_text_command_reports_success(self):
        result = output.run_text_command(
            [sys.executable, "-c", "print('ok')"],
            timeout=5,
        )

        self.assertEqual(result.status, "ok")
        self.assertEqual(result.exit_code, 0)
        self.assertEqual(result.output, "ok\n")
        self.assertGreaterEqual(result.wall_seconds, 0)

    def test_run_text_command_reports_error(self):
        result = output.run_text_command(
            [sys.executable, "-c", "import sys; print('bad'); sys.exit(7)"],
            timeout=5,
        )

        self.assertEqual(result.status, "error")
        self.assertEqual(result.exit_code, 7)
        self.assertEqual(result.output, "bad\n")

    def test_run_text_command_reports_timeout(self):
        result = output.run_text_command(
            [sys.executable, "-c", "import time; print('start', flush=True); time.sleep(1)"],
            timeout=0.05,
        )

        self.assertEqual(result.status, "timeout")
        self.assertIsNone(result.exit_code)
        self.assertIn("TIMEOUT after 0.05s", result.output)
        self.assertGreaterEqual(result.wall_seconds, 0)

    def test_build_leia_uses_standard_go_command(self):
        with mock.patch("benchmark_output.subprocess.run") as run:
            run.return_value = mock.Mock(returncode=0, stdout="")

            output.build_leia(Path("/repo"), Path("/tmp/leia"))

        run.assert_called_once_with(
            ["go", "build", "-o", "/tmp/leia", "./cmd/leia"],
            cwd=Path("/repo"),
            stdout=output.subprocess.PIPE,
            stderr=output.subprocess.STDOUT,
            text=True,
            check=False,
        )

    def test_build_leia_keeps_custom_failure_message(self):
        with mock.patch("benchmark_output.subprocess.run") as run, mock.patch("benchmark_output.print") as print_:
            run.return_value = mock.Mock(returncode=12, stdout="compiler output")

            with self.assertRaisesRegex(SystemExit, "build failed in /repo with exit 12"):
                output.build_leia(
                    Path("/repo"),
                    Path("/tmp/leia"),
                    failure_message="build failed in {root} with exit {exit_code}",
                )

        print_.assert_called_once_with("compiler output", file=output.sys.stderr)

    def test_leia_mode_command_formats_vm_mode(self):
        cmd, env = output.leia_mode_command("vm", Path("/bin/leia"), Path("/bench/main.leia"))

        self.assertEqual(cmd, ["/bin/leia", "-vm", "/bench/main.leia"])
        self.assertIsInstance(env, dict)
        self.assertNotIn("LEIA_TIER2_NO_FILTER", env)

    def test_leia_mode_command_formats_jit_mode(self):
        cmd, env = output.leia_mode_command("default", Path("/bin/leia"), Path("/bench/main.leia"))

        self.assertEqual(cmd, ["/bin/leia", "-jit", "-jit-stats", "-exit-stats", "/bench/main.leia"])
        self.assertIsInstance(env, dict)
        self.assertNotIn("LEIA_TIER2_NO_FILTER", env)

    def test_leia_mode_command_formats_no_filter_mode(self):
        cmd, env = output.leia_mode_command("no_filter", Path("/bin/leia"), Path("/bench/main.leia"))

        self.assertEqual(cmd, ["/bin/leia", "-jit", "-jit-stats", "-exit-stats", "/bench/main.leia"])
        self.assertEqual(env["LEIA_TIER2_NO_FILTER"], "1")

    def test_leia_mode_command_rejects_unknown_mode(self):
        with self.assertRaisesRegex(ValueError, "unknown mode: bad"):
            output.leia_mode_command("bad", Path("/bin/leia"), Path("/bench/main.leia"))

    def test_benchmark_mode_command_formats_luajit_mode(self):
        with mock.patch("pathlib.Path.exists", return_value=True):
            cmd, env, unavailable = output.benchmark_mode_command(
                "luajit",
                Path("/bin/leia"),
                Path("/bench/main.leia"),
                luajit_bin="/bin/luajit",
                luajit_script=Path("/bench/main.lua"),
            )

        self.assertEqual(cmd, ["/bin/luajit", "/bench/main.lua"])
        self.assertIsNone(env)
        self.assertIsNone(unavailable)

    def test_benchmark_mode_command_reports_missing_luajit_input(self):
        cmd, env, unavailable = output.benchmark_mode_command(
            "luajit",
            Path("/bin/leia"),
            Path("/bench/main.leia"),
            luajit_bin="/bin/luajit",
            luajit_script=Path("/missing/main.lua"),
        )

        self.assertIsNone(cmd)
        self.assertIsNone(env)
        self.assertEqual(unavailable, "missing")

    def test_benchmark_mode_command_reports_skipped_luajit_mode(self):
        cmd, env, unavailable = output.benchmark_mode_command(
            "luajit",
            Path("/bin/leia"),
            Path("/bench/main.leia"),
            luajit_bin=None,
            luajit_script=Path("/bench/main.lua"),
        )

        self.assertIsNone(cmd)
        self.assertIsNone(env)
        self.assertEqual(unavailable, "skipped")

    def test_benchmark_mode_command_formats_leia_mode(self):
        with mock.patch("pathlib.Path.exists", return_value=True):
            cmd, env, unavailable = output.benchmark_mode_command(
                "default",
                Path("/bin/leia"),
                Path("/bench/main.leia"),
            )

        self.assertEqual(cmd, ["/bin/leia", "-jit", "-jit-stats", "-exit-stats", "/bench/main.leia"])
        self.assertIsInstance(env, dict)
        self.assertIsNone(unavailable)

    def test_benchmark_mode_command_reports_missing_leia_input(self):
        cmd, env, unavailable = output.benchmark_mode_command(
            "default",
            Path("/bin/leia"),
            Path("/missing/main.leia"),
        )

        self.assertIsNone(cmd)
        self.assertIsNone(env)
        self.assertEqual(unavailable, "missing")


if __name__ == "__main__":
    unittest.main()
