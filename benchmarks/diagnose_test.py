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


class DiagnoseSummaryTest(unittest.TestCase):
    def test_diagnostic_summary_helpers_format_missing_values(self):
        row = diagnose.DiagnosticRow(
            benchmark="sum",
            group="math",
            script="benchmarks/math/sum.gs",
            status="ok",
        )

        self.assertEqual(diagnose.diagnostic_time_text(row), "-")
        self.assertEqual(diagnose.diagnostic_tier2_text(row), "0/0/0")
        self.assertEqual(diagnose.diagnostic_runtime_text(row), "-")
        self.assertEqual(diagnose.diagnostic_pprof_text(row), "-")

    def test_diagnostic_summary_helpers_format_present_values(self):
        row = diagnose.DiagnosticRow(
            benchmark="events_metamethod",
            group="table",
            script="benchmarks/table/events_metamethod.gs",
            status="ok",
            time_seconds=0.1254,
            t2_attempted=5,
            t2_compiled=4,
            t2_entered=3,
            runtime_summary={"native_fallback": 11},
            tier2_call_summary={"turn": 2},
            pprof_runs=4,
            pprof_script_repeat=8,
            pprof_samples_seconds=0.321,
            pprof_effective=True,
        )

        self.assertEqual(diagnose.diagnostic_time_text(row), "0.125s")
        self.assertEqual(diagnose.diagnostic_tier2_text(row), "5/4/3")
        self.assertEqual(diagnose.diagnostic_runtime_text(row), "native_fallback=11, tier2_turn=2")
        self.assertEqual(diagnose.diagnostic_pprof_text(row), "ok 0.321s/4 runs/repeat 8")

    def test_diagnostic_markdown_row_formats_runtime_and_profile_bits(self):
        row = diagnose.DiagnosticRow(
            benchmark="events_metamethod",
            group="table",
            script="benchmarks/table/events_metamethod.gs",
            status="ok",
            time_seconds=0.125,
            t2_attempted=5,
            t2_compiled=4,
            t2_entered=3,
            exit_total=2,
            top_exit={"exit_name": "shape", "reason": "guard", "pc": 17, "count": 9},
            work_action="compile",
            work_target="loop",
            work_proto="<main>",
            work_priority=7,
            readiness="ready",
            runtime_summary={"native_fallback": 11, "string_format_fast": 3},
            tier2_call_summary={"turn": 2},
            pprof_runs=4,
            pprof_script_repeat=8,
            pprof_samples_seconds=0.321,
            pprof_effective=True,
            artifact_dir="out/table",
        )

        self.assertEqual(
            diagnose.diagnostic_markdown_row(row),
            (
                "| table/events_metamethod | 0.125s | 5/4/3 | 2 | "
                "shape guard pc=17 count=9 | compile/loop <main> p=7 ready | "
                "native_fallback=11, string_format_fast=3, tier2_turn=2 | "
                "ok 0.321s/4 runs/repeat 8 | `out/table` |"
            ),
        )

    def test_diagnostic_markdown_row_formats_empty_optional_fields(self):
        row = diagnose.DiagnosticRow(
            benchmark="sum",
            group="math",
            script="benchmarks/math/sum.gs",
            status="ok",
            artifact_dir="out/math",
        )

        self.assertEqual(
            diagnose.diagnostic_markdown_row(row),
            "| math/sum | - | 0/0/0 | 0 | - | - | - | - | `out/math` |",
        )


if __name__ == "__main__":
    unittest.main()
