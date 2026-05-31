import argparse
import json
import sys
import unittest
from dataclasses import asdict
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import timing_compare as timing


def args(**overrides):
    values = {
        "runs": 3,
        "warmup": 1,
        "min_sample_seconds": 0.05,
        "timer_resolution": 0.001,
        "min_wall_repeat": 4,
        "max_repeat": 128,
        "no_wall_fallback": False,
        "time_source": "auto",
        "scale_profile": "none",
        "scale_values": [],
    }
    values.update(overrides)
    return argparse.Namespace(**values)


class TimingCompareDiagnosticTest(unittest.TestCase):
    def test_discovery_matches_domain_manifest_benchmark_ids(self):
        root = Path(__file__).resolve().parents[1]
        manifest = json.loads((root / "benchmarks" / "manifest.json").read_text())
        expected = {row["id"] for row in manifest["benchmarks"]}
        specs = timing.discover_specs(root, timing.GROUPS)
        self.assertEqual({spec.benchmark_id for spec in specs}, expected)

    def test_select_specs_accepts_legacy_group_selector_aliases(self):
        root = Path(__file__).resolve().parents[1]
        specs = timing.discover_specs(root, timing.GROUPS)

        self.assertEqual(
            [spec.benchmark_id for spec in timing.select_specs(specs, ["data_oriented/soa_dot"])],
            ["data/soa_dot"],
        )
        self.assertEqual(
            [spec.benchmark_id for spec in timing.select_specs(specs, ["extended/goroutine_sleep"])],
            ["concurrency/goroutine_sleep"],
        )
        self.assertEqual(
            [spec.benchmark_id for spec in timing.select_specs(specs, ["official/events_metamethod_hot"])],
            ["table/events_metamethod"],
        )

    def test_scale_overrides_accept_legacy_group_selector_aliases(self):
        root = Path(__file__).resolve().parents[1]
        specs = timing.select_specs(timing.discover_specs(root, timing.GROUPS), ["extended/goroutine_sleep"])
        overrides = timing.parse_scale_overrides(["extended/goroutine_sleep:N=10"])

        timing.validate_scale_selectors(specs, overrides)
        self.assertEqual(timing.scale_overrides_for(specs[0], overrides), overrides)

    def test_conformance_low_resolution_gets_concrete_rerun_advice(self):
        spec = timing.BenchmarkSpec(
            "calls",
            "calls_vararg_coroutine",
            "benchmarks/calls/calls_vararg_coroutine.gs",
            "benchmarks/lua_ref/calls/calls_vararg_coroutine.lua",
        )
        samples = [
            timing.Sample(
                status="low_resolution",
                repeat=128,
                script_total_seconds=0.0,
                wall_total_seconds=0.012,
                note="script Time: below resolution",
            )
        ]
        subject = timing.summarize_subject("current", "default", samples, repeat=128)
        subject.diagnostic = timing.subject_diagnostic(spec, subject, samples, [], args())

        payload = asdict(subject)
        advice = payload["diagnostic"]["low_resolution"]
        self.assertIn("calls/calls_vararg_coroutine:N_CALLS=880000", advice["scale"])
        self.assertIn("--scale calls/calls_vararg_coroutine:N_CORO=360000", advice["rerun_args"])
        self.assertEqual(advice["min_sample_seconds"], 0.05)
        self.assertEqual(advice["min_wall_repeat"], 8)
        self.assertEqual(advice["max_repeat"], 256)

    def test_markdown_reports_low_resolution_and_wall_repeat_diagnostics(self):
        bench = timing.BenchmarkResult("calls_vararg_coroutine", "calls")
        spec = timing.BenchmarkSpec(
            "calls",
            "calls_vararg_coroutine",
            "benchmarks/calls/calls_vararg_coroutine.gs",
            "benchmarks/lua_ref/calls/calls_vararg_coroutine.lua",
        )
        low_samples = [
            timing.Sample(status="low_resolution", repeat=128, script_total_seconds=0.0, wall_total_seconds=0.010)
        ]
        current = timing.summarize_subject("current", "default", low_samples, repeat=128)
        current.diagnostic = timing.subject_diagnostic(spec, current, low_samples, [], args())

        wall_samples = [
            timing.Sample(status="ok", seconds=0.006, repeat=8, source="wall_repeat", wall_total_seconds=0.048)
        ]
        luajit = timing.summarize_subject("luajit", "default", wall_samples, repeat=8)
        luajit.diagnostic = timing.subject_diagnostic(spec, luajit, wall_samples, [], args())

        bench.modes["default"] = {
            "current": current,
            "head": timing.SubjectResult("head", "default", "skipped"),
            "luajit": luajit,
        }

        report = timing.markdown([bench], ["default"], args())
        self.assertIn("## Low-Resolution Diagnostics", report)
        self.assertIn("calls/calls_vararg_coroutine:N_CALLS=880000", report)
        self.assertIn("--min-sample-seconds 0.050", report)
        self.assertIn("## Wall-Repeat Diagnostics", report)
        self.assertIn("scale workload enough for script_repeat", report)


if __name__ == "__main__":
    unittest.main()
