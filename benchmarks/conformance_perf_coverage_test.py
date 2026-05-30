import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import conformance_perf_coverage as coverage


class ConformancePerfCoverageTest(unittest.TestCase):
    def test_check_helpers_accept_current_repository_map(self):
        cases = coverage.conformance_cases(coverage.ROOT)
        known = coverage.benchmark_ids(coverage.ROOT)
        self.assertEqual(coverage.missing_coverage_families(cases), [])
        self.assertEqual(coverage.missing_benchmark_refs(cases, known), set())

    def test_unknown_non_host_family_requires_explicit_classification(self):
        cases = {"newfamily": []}
        self.assertEqual(coverage.missing_coverage_families(cases), ["newfamily"])

    def test_missing_benchmark_refs_are_reported(self):
        cases = {"math": []}
        self.assertIn("numeric/math_intensive", coverage.missing_benchmark_refs(cases, set()))


if __name__ == "__main__":
    unittest.main()
