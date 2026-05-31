import argparse
import tempfile
import unittest
from dataclasses import dataclass
from pathlib import Path

import benchmark_discovery as discovery


@dataclass(frozen=True)
class FakeSpec:
    group: str
    name: str

    @property
    def benchmark_id(self):
        return f"{self.group}/{self.name}"


class BenchmarkDiscoveryTest(unittest.TestCase):
    def test_group_choices_includes_only_domain_group_names(self):
        self.assertEqual(
            discovery.group_choices(discovery.DOMAIN_GROUPS),
            [
                "numeric",
                "recursion",
                "table",
                "calls",
                "string",
                "concurrency",
                "data",
                "app",
                "control",
                "precision",
            ],
        )

    def test_group_choices_uses_allowed_groups_verbatim(self):
        self.assertEqual(
            discovery.group_choices(["numeric", "data"]),
            ["numeric", "data"],
        )

    def test_domain_specs_prefers_default_order_then_sorted_extras_and_luajit_refs(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            bench_dir = root / "benchmarks" / "numeric"
            lua_dir = root / "benchmarks" / "lua_ref" / "numeric"
            bench_dir.mkdir(parents=True)
            lua_dir.mkdir(parents=True)
            for name in ("z_extra", "matmul", "a_extra", "matmul_row"):
                (bench_dir / f"{name}.gs").write_text("-- test\n")
            (lua_dir / "matmul.lua").write_text("-- ref\n")

            specs = discovery.domain_specs(root, "numeric")

        self.assertEqual([spec.name for spec in specs], ["matmul", "a_extra", "matmul_row", "z_extra"])
        self.assertEqual(specs[0].luajit_rel, "benchmarks/lua_ref/numeric/matmul.lua")
        self.assertIsNone(specs[1].luajit_rel)
        self.assertEqual(specs[2].base, "matmul")

    def test_select_specs_rejects_ambiguous_short_names(self):
        specs = [FakeSpec("numeric", "sort"), FakeSpec("table", "sort")]

        with self.assertRaisesRegex(SystemExit, "ambiguous benchmark selector 'sort'"):
            discovery.select_specs(specs, ["sort"])

    def test_select_specs_rejects_unknown_domain_selector(self):
        specs = [FakeSpec("table", "events_metamethod")]

        with self.assertRaisesRegex(SystemExit, "unknown benchmark selector"):
            discovery.select_specs(specs, ["missing_domain/events_metamethod"])

    def test_spec_selectors_includes_short_and_canonical_names(self):
        self.assertEqual(
            discovery.spec_selectors([FakeSpec("numeric", "matmul"), FakeSpec("calls", "closure_accumulator")]),
            {"matmul", "numeric/matmul", "closure_accumulator", "calls/closure_accumulator"},
        )

    def test_selector_candidates_map_historical_prefixes_to_domain_selectors_internally(self):
        self.assertEqual(discovery.selector_candidates("suite/fib"), ["suite/fib", "fib"])
        self.assertEqual(discovery.selector_candidates("extended/log_tokenize_format"), ["extended/log_tokenize_format", "log_tokenize_format"])
        self.assertEqual(discovery.selector_candidates("variants/matmul_row"), ["variants/matmul_row", "matmul_row"])
        self.assertEqual(discovery.selector_candidates("numeric/matmul"), ["numeric/matmul"])

    def test_selector_matches_spec_accepts_only_domain_selectors(self):
        self.assertTrue(discovery.selector_matches_spec("numeric/matmul", FakeSpec("numeric", "matmul")))
        self.assertFalse(discovery.selector_matches_spec("missing_domain/matmul", FakeSpec("numeric", "matmul")))
        self.assertFalse(
            discovery.selector_matches_spec("missing_domain/events_metamethod", FakeSpec("table", "events_metamethod"))
        )

    def test_parse_selector_count_overrides_accepts_modes_and_domain_selectors(self):
        overrides = discovery.parse_selector_count_overrides(
            ["fib=4", "numeric/matmul=6", "vm/table/events_metamethod=8"],
            ["vm", "default"],
            "--repeat",
        )

        self.assertEqual(discovery.selector_count_override(overrides, "default", "fib"), 4)
        self.assertEqual(discovery.selector_count_override(overrides, "default", "matmul", "numeric/matmul"), 6)
        self.assertEqual(
            discovery.selector_count_override(overrides, "vm", "events_metamethod", "table/events_metamethod"),
            8,
        )
        self.assertIsNone(discovery.selector_count_override(overrides, "default", "events_metamethod"))
        self.assertIsNone(discovery.selector_count_override(overrides, "default", "matmul", "missing_domain/matmul"))

    def test_parse_selector_count_overrides_rejects_bad_counts(self):
        with self.assertRaises(argparse.ArgumentTypeError):
            discovery.parse_selector_count_overrides(["fib=0"], ["vm"], "--repeat")
        with self.assertRaises(argparse.ArgumentTypeError):
            discovery.parse_selector_count_overrides(["fib=nope"], ["vm"], "--repeat")

    def test_resolve_script_path_rejects_unknown_suffix_selectors(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            (root / "benchmarks" / "calls").mkdir(parents=True)
            (root / "benchmarks" / "calls" / "closure_accumulator.gs").write_text("-- test\n")

            self.assertEqual(
                discovery.resolve_script_path(root, "calls/closure_accumulator"),
                root / "benchmarks" / "calls" / "closure_accumulator.gs",
            )
            self.assertEqual(
                discovery.resolve_script_path(root, "variants/closure_accumulator"),
                root / "benchmarks" / "calls" / "closure_accumulator.gs",
            )
            self.assertIsNone(discovery.resolve_script_path(root, "calls/closure_accumulator_unknown"))

    def test_resolve_script_identity_returns_domain_group_name_and_path(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            (root / "benchmarks" / "table").mkdir(parents=True)
            (root / "benchmarks" / "table" / "events_metamethod.gs").write_text("-- test\n")

            self.assertEqual(
                discovery.resolve_script_identity(root, "table/events_metamethod"),
                ("table", "events_metamethod", root / "benchmarks" / "table" / "events_metamethod.gs"),
            )
            self.assertIsNone(discovery.resolve_script_identity(root, "missing_domain/events_metamethod"))

    def test_groups_for_selectors_includes_domain_selector_groups(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            for group, name in (("concurrency", "goroutine_sleep"), ("table", "events_metamethod")):
                (root / "benchmarks" / group).mkdir(parents=True, exist_ok=True)
                (root / "benchmarks" / group / f"{name}.gs").write_text("-- test\n")

            self.assertEqual(
                discovery.groups_for_selectors(
                    root,
                    ["data"],
                    ["concurrency/goroutine_sleep", "table/events_metamethod"],
                ),
                ["data", "concurrency", "table"],
            )

    def test_groups_for_selectors_ignores_unknown_selectors(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            for group, name in (("concurrency", "goroutine_sleep"), ("table", "events_metamethod")):
                (root / "benchmarks" / group).mkdir(parents=True, exist_ok=True)
                (root / "benchmarks" / group / f"{name}.gs").write_text("-- test\n")

            self.assertEqual(
                discovery.groups_for_selectors(
                    root,
                    ["data"],
                    ["missing_domain/goroutine_sleep", "unknown/events_metamethod"],
                ),
                ["data"],
            )

    def test_groups_for_selectors_can_start_from_only_selectors(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            for group, name in (("table", "events_metamethod"), ("data", "soa_dot")):
                (root / "benchmarks" / group).mkdir(parents=True, exist_ok=True)
                (root / "benchmarks" / group / f"{name}.gs").write_text("-- test\n")

            self.assertEqual(
                discovery.groups_for_selectors(
                    root,
                    [],
                    ["table/events_metamethod", "data/soa_dot"],
                ),
                ["table", "data"],
            )

    def test_groups_for_selectors_ignores_selectors_outside_allowed_groups(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            for group, name in (("data", "soa_dot"), ("concurrency", "goroutine_sleep")):
                (root / "benchmarks" / group).mkdir(parents=True, exist_ok=True)
                (root / "benchmarks" / group / f"{name}.gs").write_text("-- test\n")

            self.assertEqual(
                discovery.groups_for_selectors(
                    root,
                    ["data"],
                    ["missing_domain/goroutine_sleep"],
                    allowed_groups=["data"],
                ),
                ["data"],
            )

    def test_groups_for_selection_handles_all_groups_flag(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            self.assertEqual(
                discovery.groups_for_selection(
                    root,
                    ["data"],
                    ["table/events_metamethod"],
                    True,
                    allowed_groups=["data", "table"],
                ),
                ["data", "table"],
            )


if __name__ == "__main__":
    unittest.main()
