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


if __name__ == "__main__":
    unittest.main()
