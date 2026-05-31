import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import triage


class TriageSelectorTest(unittest.TestCase):
    def test_bench_script_path_accepts_domain_selectors(self):
        root = Path(__file__).resolve().parents[1]

        self.assertEqual(
            triage.bench_id_to_path(root, "data/soa_dot"),
            ("data", "soa_dot", root / "benchmarks" / "data" / "soa_dot.gs"),
        )
        self.assertEqual(
            triage.bench_id_to_path(root, "concurrency/goroutine_sleep"),
            ("concurrency", "goroutine_sleep", root / "benchmarks" / "concurrency" / "goroutine_sleep.gs"),
        )
        self.assertEqual(
            triage.bench_id_to_path(root, "table/events_metamethod"),
            ("table", "events_metamethod", root / "benchmarks" / "table" / "events_metamethod.gs"),
        )

    def test_groups_for_benches_uses_shared_domain_selector_resolution(self):
        root = Path(__file__).resolve().parents[1]

        self.assertEqual(
            triage.groups_for_benches(
                root,
                [
                    "table/events_metamethod",
                    "concurrency/goroutine_sleep",
                    "data/soa_dot",
                ],
            ),
            ["table", "concurrency", "data"],
        )

    def test_write_report_uses_shared_markdown_row_shape(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp) / "triage.md"
            summary = Path(tmp) / "triage.json"
            timing = Path(tmp) / "timing.md"
            artifacts = {
                "timing": triage.ArtifactStatus(str(timing), "ok", "ready"),
                "diag": triage.ArtifactStatus(None, "not-requested", ""),
            }

            triage.write_report(
                out,
                [
                    {
                        "benchmark": "math/sum",
                        "scale": {"n": 10},
                        "mode": "default",
                        "current": 0.1,
                        "head": 0.2,
                        "luajit": 0.3,
                        "cur_head": 0.5,
                        "cur_luajit": 0.333,
                        "source": "script",
                        "repeat": 4,
                        "exits": 0,
                        "ci95": 1.25,
                        "note": "",
                    }
                ],
                timing,
                None,
                None,
                None,
                None,
                None,
                None,
                summary,
                [triage.Bottleneck("runtime-call-heavy", "P2", "medium", ["runtime call"], "inline hot path")],
                artifacts,
            )

            text = out.read_text()

        self.assertIn("| P2 | runtime-call-heavy | medium | runtime call | inline hot path |", text)
        self.assertIn("| math/sum | n:10 | default | 0.100000s | 0.200000s | 0.300000s |", text)
        self.assertIn("| diag | not-requested | `-` | - |", text)


if __name__ == "__main__":
    unittest.main()
