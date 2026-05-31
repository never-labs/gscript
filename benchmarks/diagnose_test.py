import argparse
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import diagnose


class DiagnoseSelectorTest(unittest.TestCase):
    def test_groups_for_args_accepts_legacy_group_and_selector_aliases(self):
        args = argparse.Namespace(
            all_groups=False,
            group=["data_oriented"],
            bench=["extended/goroutine_sleep", "official/events_metamethod_hot"],
        )

        self.assertEqual(diagnose.groups_for_args(args), ["data", "concurrency", "table"])


if __name__ == "__main__":
    unittest.main()
