import re
import sys
import unittest

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


if __name__ == "__main__":
    unittest.main()
