import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "performance_gate.sh"


def shell_array_values(text, name):
    start = text.index(f"{name}=(")
    end = text.index("\n)", start)
    values = []
    for line in text[start:end].splitlines()[1:]:
        stripped = line.strip()
        if stripped.startswith('"') and stripped.endswith('"'):
            values.append(stripped.strip('"'))
    return values


def feature_cell_refs(feature_id, cell_name):
    matrix = json.loads((ROOT / "tests" / "feature_matrix.json").read_text())
    for feature in matrix["features"]:
        if feature["id"] == feature_id:
            return feature[cell_name]["refs"]
    raise AssertionError(f"feature_matrix.json missing {feature_id}")


def benchmark_ids_from_feature_refs(feature_id, cell_name):
    ids = []
    for ref in feature_cell_refs(feature_id, cell_name):
        if ref.startswith("benchmarks/") and ref.endswith(".leia"):
            ids.append(ref.removeprefix("benchmarks/")[: -len(".leia")])
    return ids


def subject(median, status="ok", source="script_repeat", cv=2.0):
    return {
        "status": status,
        "source": source,
        "stats": {
            "median": median,
            "cv_pct": cv,
        },
    }


def timing_payload(
    current,
    head,
    *,
    benchmark_id="numeric/hot_loop",
    luajit=2.0,
    current_status="ok",
    head_status="ok",
    luajit_status="ok",
    source="script_repeat",
):
    return {
        "modes": ["default"],
        "results": [
            {
                "group": benchmark_id.split("/", 1)[0],
                "benchmark": benchmark_id.split("/", 1)[1],
                "modes": {
                    "default": {
                        "current": subject(current, current_status, source),
                        "head": subject(head, head_status, source),
                        "luajit": subject(luajit, luajit_status, source),
                    }
                },
            }
        ],
    }


def run_validate(payload, *args):
    with tempfile.TemporaryDirectory() as td:
        path = Path(td) / "timing.json"
        path.write_text(json.dumps(payload))
        return subprocess.run(
            ["bash", str(SCRIPT), "--validate-only", str(path), *args],
            cwd=ROOT,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            check=False,
        )


class PerformanceGateValidationTest(unittest.TestCase):
    def test_validate_only_accepts_flat_script_timed_row(self):
        proc = run_validate(timing_payload(1.05, 1.00))
        self.assertEqual(proc.returncode, 0, proc.stdout)
        self.assertIn("Performance gate current/HEAD ranking", proc.stdout)
        self.assertIn("Performance gate passed.", proc.stdout)

    def test_validate_only_json_reports_machine_readable_pass(self):
        proc = run_validate(timing_payload(1.05, 1.00), "--json")
        self.assertEqual(proc.returncode, 0, proc.stdout)
        report = json.loads(proc.stdout)
        self.assertEqual(report["schema_version"], 1)
        self.assertEqual(report["status"], "pass")
        self.assertTrue(report["validate_only"])
        self.assertEqual(report["failure_count"], 0)
        self.assertEqual(report["failures"], [])
        self.assertIn("timing.json", report["timing_json"])
        self.assertIn("Performance gate passed.", "\n".join(report["output_lines"]))

    def test_validate_only_json_reports_machine_readable_failure(self):
        proc = run_validate(timing_payload(1.50, 1.00), "--json")
        self.assertEqual(proc.returncode, 1, proc.stdout)
        report = json.loads(proc.stdout)
        self.assertEqual(report["schema_version"], 1)
        self.assertEqual(report["status"], "issues")
        self.assertEqual(report["failure_count"], 1)
        self.assertEqual(report["failures"], ["timing validation failed"])
        self.assertIn("Performance gate violations", "\n".join(report["output_lines"]))

    def test_json_requires_validate_only(self):
        proc = subprocess.run(
            ["bash", str(SCRIPT), "--json", "--no-strict", "--no-luajit"],
            cwd=ROOT,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            check=False,
        )
        self.assertEqual(proc.returncode, 2, proc.stdout)
        self.assertIn("--json is only supported with --validate-only", proc.stdout)

    def test_validate_only_accepts_current_only_new_benchmark(self):
        proc = run_validate(timing_payload(1.05, None, benchmark_id="data/q_query_rollup", head_status="missing"))
        self.assertEqual(proc.returncode, 0, proc.stdout)
        self.assertIn("current_only_new_benchmark", proc.stdout)
        self.assertIn("Performance gate passed.", proc.stdout)

    def test_quick_phase_smoke_is_explicit_parseable_profile(self):
        proc = run_validate(timing_payload(1.05, 1.00), "--quick-phase-smoke")
        self.assertEqual(proc.returncode, 0, proc.stdout)
        self.assertIn("Performance gate passed.", proc.stdout)

    def test_syntax_smoke_is_explicit_parseable_profile(self):
        proc = run_validate(timing_payload(1.05, 1.00), "--syntax-smoke")
        self.assertEqual(proc.returncode, 0, proc.stdout)
        self.assertIn("Performance gate passed.", proc.stdout)

    def test_syntax_smoke_includes_dialect_guard(self):
        gate = SCRIPT.read_text()
        self.assertIn('"app/dialect_syntax_smoke"', gate)

    def test_syntax_smoke_keeps_dialect_out_of_timing_comparisons(self):
        gate = SCRIPT.read_text()
        self.assertNotIn("app/dialect_syntax_smoke", shell_array_values(gate, "SYNTAX_SMOKE_BENCHES"))
        self.assertEqual(
            shell_array_values(gate, "SYNTAX_DIALECT_SMOKE_BENCHES"),
            ["app/dialect_syntax_smoke"],
        )

    def test_syntax_smoke_strict_pass_uses_leia_only_modes_for_dialect_guard(self):
        gate = SCRIPT.read_text()
        self.assertIn(
            'if [ "$PROFILE" = "syntax_smoke" ]; then\n'
            "        STRICT_CMD+=(--mode vm --mode default --mode no_filter)\n"
            "    fi",
            gate,
        )

    def test_builtin_gate_selectors_are_registered_manifest_workloads(self):
        gate = SCRIPT.read_text()
        manifest = json.loads((ROOT / "benchmarks" / "manifest.json").read_text())
        case_ids = {case["id"] for case in manifest["cases"]}
        workload_ids = {workload["id"] for workload in manifest["workloads"]}
        array_names = (
            "CORE_BENCHES",
            "SMOKE_BENCHES",
            "SYNTAX_SMOKE_BENCHES",
            "SYNTAX_DIALECT_SMOKE_BENCHES",
            "STRICT_SMOKE_BENCHES",
            "PHASE_SMOKE_BENCHES",
            "FEATURE_SMOKE_BENCHES",
            "STRICT_CORE_BENCHES",
            "STRICT_FEATURE_BENCHES",
        )

        selectors = {
            selector
            for name in array_names
            for selector in shell_array_values(gate, name)
        }

        self.assertEqual(sorted(selectors - case_ids), [])
        self.assertEqual(sorted(selectors - workload_ids), [])

    def test_feature_smoke_keeps_data_oriented_dense_and_soa_gate(self):
        gate = SCRIPT.read_text()
        for array_name in ("PHASE_SMOKE_BENCHES", "FEATURE_SMOKE_BENCHES", "STRICT_FEATURE_BENCHES"):
            values = shell_array_values(gate, array_name)
            self.assertIn("numeric/matmul_dense", values)
            self.assertIn("data/soa_affine_many", values)
            self.assertIn("data/soa_masked_aggregate", values)
        for array_name in ("FEATURE_SMOKE_BENCHES", "STRICT_FEATURE_BENCHES"):
            values = shell_array_values(gate, array_name)
            self.assertIn("data/q_query_rollup", values)

    def test_feature_smoke_covers_q_analytics_data_hot_refs(self):
        gate = SCRIPT.read_text()
        q_hot_refs = set(benchmark_ids_from_feature_refs("q_analytics_dialect", "perf_hot_case"))

        for array_name in ("FEATURE_SMOKE_BENCHES", "STRICT_FEATURE_BENCHES"):
            values = set(shell_array_values(gate, array_name))
            self.assertEqual(sorted(q_hot_refs - values), [])

    def test_feature_smoke_uses_stable_sampling_for_short_app_workloads(self):
        gate = SCRIPT.read_text()
        self.assertIn(
            '--feature-smoke)\n'
            '            PROFILE="feature_smoke"\n'
            "            RUNS=2\n"
            "            WARMUP=1\n"
            "            TIMEOUT=90\n"
            "            # Feature smoke includes loopback/http/sqlite/data workloads whose\n"
            "            # individual script timings are short enough that 0.1s samples can\n"
            "            # make current-vs-HEAD comparisons fail on measurement noise alone.\n"
            "            MIN_SAMPLE_SECONDS=0.300\n"
            "            MAX_REPEAT=256\n"
            "            # Keep the mixed feature smoke serial by default. These workloads\n"
            "            # compare current, clean HEAD, and LuaJIT binaries; running several\n"
            "            # calibrated samples at once measures local CPU contention more\n"
            "            # than language/runtime performance. A caller can still pass\n"
            "            # --jobs=N explicitly when using the profile for exploratory runs.\n"
            "            MAX_JOBS=1",
            gate,
        )

    def test_q_analytics_feature_matrix_hot_refs_include_runnable_q_query_smoke(self):
        manifest = json.loads((ROOT / "benchmarks" / "manifest.json").read_text())
        case_ids = {case["id"] for case in manifest["cases"]}
        workloads = {workload["id"]: workload for workload in manifest["workloads"]}
        hot_refs = benchmark_ids_from_feature_refs("q_analytics_dialect", "perf_hot_case")

        self.assertIn("data/q_query_rollup", hot_refs)
        self.assertEqual(sorted(set(hot_refs) - case_ids), [])
        self.assertEqual(sorted(set(hot_refs) - set(workloads)), [])
        for benchmark_id in ("data/q_query_rollup", "data/q_operator_pipeline"):
            with self.subTest(benchmark_id=benchmark_id):
                workload = workloads[benchmark_id]
                self.assertEqual(workload["time_source_hint"], "script_time_line")
                lua_ref = workload["comparison_reference"]
                self.assertIsNotNone(lua_ref)
                self.assertEqual(lua_ref["kind"], "lua")
                self.assertTrue((ROOT / lua_ref["path"]).exists(), lua_ref["path"])

    def test_data_oriented_feature_matrix_hot_refs_are_manifested_with_luajit_refs(self):
        manifest = json.loads((ROOT / "benchmarks" / "manifest.json").read_text())
        case_ids = {case["id"] for case in manifest["cases"]}
        workloads = {workload["id"]: workload for workload in manifest["workloads"]}
        expected = [
            "numeric/matmul_dense",
            "numeric/spectral_norm_dense",
            "data/soa_affine_many",
            "data/soa_masked_aggregate",
            "data/soa_filter_gather",
        ]
        hot_refs = benchmark_ids_from_feature_refs("matrix_dense_arrays", "perf_hot_case")

        self.assertEqual(hot_refs, expected)
        self.assertEqual(sorted(set(hot_refs) - case_ids), [])
        self.assertEqual(sorted(set(hot_refs) - set(workloads)), [])
        for benchmark_id in hot_refs:
            with self.subTest(benchmark_id=benchmark_id):
                workload = workloads[benchmark_id]
                self.assertEqual(workload["time_source_hint"], "script_time_line")
                lua_ref = workload["comparison_reference"]
                self.assertIsNotNone(lua_ref)
                self.assertEqual(lua_ref["kind"], "lua")
                self.assertTrue((ROOT / lua_ref["path"]).exists(), lua_ref["path"])

    def test_full_gate_selectors_cover_data_oriented_feature_refs_without_expanding_quick_gate(self):
        gate = SCRIPT.read_text()
        data_hot_refs = set(benchmark_ids_from_feature_refs("matrix_dense_arrays", "perf_hot_case"))
        full_block = gate.split('if [ "$PROFILE" = "full" ]; then', 1)[1].split("elif", 1)[0]
        all_groups_block = gate.split("ALL_BENCHMARK_GROUPS=(", 1)[1].split("\n)", 1)[0]

        self.assertIn("TIMING_CMD+=(--all-groups)", full_block)
        self.assertIn("numeric", all_groups_block)
        self.assertIn("data", all_groups_block)

        phase_values = set(shell_array_values(gate, "PHASE_SMOKE_BENCHES"))
        self.assertLess(len(phase_values & data_hot_refs), len(data_hot_refs))
        self.assertIn("numeric/matmul_dense", phase_values)
        self.assertIn("data/soa_affine_many", phase_values)
        self.assertIn("data/soa_masked_aggregate", phase_values)

        for array_name in ("FEATURE_SMOKE_BENCHES", "STRICT_FEATURE_BENCHES"):
            values = set(shell_array_values(gate, array_name))
            self.assertLess(len(values & data_hot_refs), len(data_hot_refs))
            self.assertIn("numeric/matmul_dense", values)
            self.assertIn("data/soa_affine_many", values)
            self.assertIn("data/soa_masked_aggregate", values)
            self.assertIn("data/soa_filter_gather", values)

    def test_jit_fallback_luajit_contract_keeps_gate_refs(self):
        semantic_refs = feature_cell_refs("arm64_jit_runtime_fallback", "semantic_gate")
        perf_refs = feature_cell_refs("arm64_jit_runtime_fallback", "perf_hot_case")
        gate = SCRIPT.read_text()

        for ref in (
            "internal/methodjit/semantic_gate_test.go",
            "internal/methodjit/diagnose_test.go",
            "internal/methodjit/exit_resume_check_test.go",
            "scripts/performance_gate.sh",
            "benchmarks/performance_gate_test.py",
            "benchmarks/perf_submit_guard_test.py",
            "benchmarks/manifest.json",
            "docs/reference/performance/index.md",
        ):
            self.assertIn(ref, semantic_refs)
        for ref in (
            "benchmarks/numeric/matmul_dense.leia",
            "benchmarks/table/table_field_access.leia",
            "benchmarks/app/mixed_inventory_sim.leia",
        ):
            self.assertIn(ref, perf_refs)
        self.assertIn("validate_luajit_artifact", gate)
        self.assertIn("--luajit-threshold", gate)
        self.assertIn("validate_strict_artifact", gate)

    def test_help_documents_syntax_smoke_profile(self):
        proc = subprocess.run(
            ["bash", str(SCRIPT), "--help"],
            cwd=ROOT,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            check=False,
        )
        self.assertEqual(proc.returncode, 0, proc.stdout)
        self.assertIn("--syntax-smoke", proc.stdout)
        self.assertIn("grammar-change hot-path gate", proc.stdout)

    def test_validate_only_rejects_obvious_regression(self):
        proc = run_validate(timing_payload(1.20, 1.00), "--threshold", "0.10")
        self.assertEqual(proc.returncode, 1, proc.stdout)
        self.assertIn("Performance gate violations", proc.stdout)
        self.assertIn("numeric/hot_loop", proc.stdout)

    def test_validate_only_rejects_luajit_ratio_above_threshold(self):
        proc = run_validate(timing_payload(0.81, 1.00, benchmark_id="numeric/matmul_dense", luajit=1.00))
        self.assertEqual(proc.returncode, 1, proc.stdout)
        self.assertIn("Guard violations", proc.stdout)
        self.assertIn("numeric/matmul_dense", proc.stdout)
        self.assertIn("luajit", proc.stdout)

    def test_validate_only_rejects_low_resolution_rows(self):
        proc = run_validate(timing_payload(None, 1.00, current_status="low_resolution"))
        self.assertEqual(proc.returncode, 1, proc.stdout)
        self.assertIn("Unreliable timing rows", proc.stdout)
        self.assertIn("low_resolution/ok", proc.stdout)

    def test_wall_timed_rows_need_larger_regression_to_fail(self):
        proc = run_validate(timing_payload(1.20, 1.00, source="wall_repeat"), "--threshold", "0.10")
        self.assertEqual(proc.returncode, 0, proc.stdout)
        self.assertIn("wall_timed_startup_noise", proc.stdout)

        proc = run_validate(
            timing_payload(1.40, 1.00, source="wall_repeat"),
            "--threshold",
            "0.10",
            "--wall-threshold",
            "0.30",
        )
        self.assertEqual(proc.returncode, 1, proc.stdout)
        self.assertIn("wall_regression", proc.stdout)


if __name__ == "__main__":
    unittest.main()
