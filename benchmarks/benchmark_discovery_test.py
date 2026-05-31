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

    def test_select_specs_accepts_legacy_hot_alias(self):
        specs = [FakeSpec("table", "events_metamethod")]

        self.assertEqual(discovery.select_specs(specs, ["official/events_metamethod_hot"]), specs)

    def test_resolve_script_path_accepts_variant_and_hot_suffixes(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            (root / "benchmarks" / "calls").mkdir(parents=True)
            (root / "benchmarks" / "calls" / "closure_accumulator.gs").write_text("-- test\n")

            self.assertEqual(
                discovery.resolve_script_path(root, "calls/closure_accumulator_variant"),
                root / "benchmarks" / "calls" / "closure_accumulator.gs",
            )
            self.assertEqual(
                discovery.resolve_script_path(root, "calls/closure_accumulator_hot"),
                root / "benchmarks" / "calls" / "closure_accumulator.gs",
            )

    def test_groups_for_selectors_includes_legacy_selector_domains(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            for group, name in (("concurrency", "goroutine_sleep"), ("table", "events_metamethod")):
                (root / "benchmarks" / group).mkdir(parents=True, exist_ok=True)
                (root / "benchmarks" / group / f"{name}.gs").write_text("-- test\n")

            self.assertEqual(
                discovery.groups_for_selectors(
                    root,
                    ["data_oriented"],
                    ["extended/goroutine_sleep", "official/events_metamethod_hot"],
                ),
                ["data", "concurrency", "table"],
            )


if __name__ == "__main__":
    unittest.main()
