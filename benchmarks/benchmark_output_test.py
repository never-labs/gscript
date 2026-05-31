import re
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


if __name__ == "__main__":
    unittest.main()
