import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import triage


class TriageSelectorTest(unittest.TestCase):
    def test_bench_script_path_accepts_timing_compare_legacy_aliases(self):
        root = Path(__file__).resolve().parents[1]

        self.assertEqual(
            triage.bench_id_to_path(root, "data_oriented/soa_dot"),
            ("data", "soa_dot", root / "benchmarks" / "data" / "soa_dot.gs"),
        )
        self.assertEqual(
            triage.bench_id_to_path(root, "extended/goroutine_sleep"),
            ("concurrency", "goroutine_sleep", root / "benchmarks" / "concurrency" / "goroutine_sleep.gs"),
        )
        self.assertEqual(
            triage.bench_id_to_path(root, "official/events_metamethod_hot"),
            ("table", "events_metamethod", root / "benchmarks" / "table" / "events_metamethod.gs"),
        )


if __name__ == "__main__":
    unittest.main()
