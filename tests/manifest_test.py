import unittest
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parent))
import manifest


class ManifestTest(unittest.TestCase):
    def test_tests_manifest_covers_all_gscript_cases(self):
        self.assertEqual(manifest.validate_manifest("tests"), [])

    def test_benchmarks_manifest_covers_all_gscript_cases(self):
        self.assertEqual(manifest.validate_manifest("benchmarks"), [])


if __name__ == "__main__":
    unittest.main()
