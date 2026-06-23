package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBenchCommandManifestCheckDispatchesBenchmarksManifest(t *testing.T) {
	oldCheckExecCommand := checkExecCommand
	t.Cleanup(func() { checkExecCommand = oldCheckExecCommand })
	var gotArgs []string
	var gotName string
	checkExecCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "manifest")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"--manifest-check"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if gotName != "go" {
		t.Fatalf("manifest command = %q, want go", gotName)
	}
	if len(gotArgs) != 6 || gotArgs[0] != "run" || gotArgs[1] != "./cmd/leia" || gotArgs[2] != "run" || !strings.HasSuffix(gotArgs[3], filepath.Join("scripts", "manifest.leia")) || gotArgs[4] != "check" || gotArgs[5] != "benchmarks" {
		t.Fatalf("args = %#v, want go run ./cmd/leia run scripts/manifest.leia check benchmarks", gotArgs)
	}
	if !strings.Contains(stdout.String(), "manifest helper ok") {
		t.Fatalf("stdout = %q, want helper output", stdout.String())
	}
}

func TestBenchCommandDispatchesCompareHarness(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var gotName string
	var gotArgs []string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"compare", "--bench", "numeric/mandelbrot"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if gotName != "python3" {
		t.Fatalf("python command = %q, want python3", gotName)
	}
	if len(gotArgs) != 3 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "timing_compare.py")) || gotArgs[1] != "--bench" || gotArgs[2] != "numeric/mandelbrot" {
		t.Fatalf("args = %#v, want timing_compare.py --bench numeric/mandelbrot", gotArgs)
	}
	if !strings.Contains(stdout.String(), "bench helper ok") {
		t.Fatalf("stdout = %q, want helper output", stdout.String())
	}
}

func TestBenchCommandDefaultsToQuickCompare(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var gotArgs []string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runBenchCommand(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) != 9 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "timing_compare.py")) || gotArgs[1] != "--bench" || gotArgs[2] != "control/sieve" || gotArgs[3] != "--runs" || gotArgs[4] != "1" || gotArgs[5] != "--warmup" || gotArgs[6] != "0" || gotArgs[7] != "--timeout" || gotArgs[8] != "60" {
		t.Fatalf("args = %#v, want timing_compare.py quick control/sieve profile", gotArgs)
	}
}

func TestBenchCommandDispatchesBenchmarkSelector(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var gotArgs []string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"table/table_field_access", "--runs", "2"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) != 5 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "timing_compare.py")) || gotArgs[1] != "--bench" || gotArgs[2] != "table/table_field_access" || gotArgs[3] != "--runs" || gotArgs[4] != "2" {
		t.Fatalf("args = %#v, want timing_compare.py --bench table/table_field_access --runs 2", gotArgs)
	}
}

func TestBenchCommandDispatchesProfiles(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var calls [][]string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string{name}, args...))
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	if code := runBenchCommand([]string{"--quick"}, &stdout, &stderr); code != 0 {
		t.Fatalf("quick code = %d, stderr = %q", code, stderr.String())
	}
	if code := runBenchCommand([]string{"--full"}, &stdout, &stderr); code != 0 {
		t.Fatalf("full code = %d, stderr = %q", code, stderr.String())
	}
	if code := runBenchCommand([]string{"--guard"}, &stdout, &stderr); code != 0 {
		t.Fatalf("guard code = %d, stderr = %q", code, stderr.String())
	}
	if len(calls) != 3 {
		t.Fatalf("calls = %#v, want three profile dispatches", calls)
	}
	if !strings.HasSuffix(calls[0][1], filepath.Join("benchmarks", "timing_compare.py")) || !containsString(calls[0], "table/table_array_access") {
		t.Fatalf("quick call = %#v, want timing quick profile", calls[0])
	}
	if !strings.HasSuffix(calls[1][1], filepath.Join("benchmarks", "timing_compare.py")) || !containsString(calls[1], "--all-groups") {
		t.Fatalf("full call = %#v, want timing full profile", calls[1])
	}
	if !strings.HasSuffix(calls[2][1], filepath.Join("benchmarks", "strict_guard.py")) || !containsString(calls[2], "control/sieve") {
		t.Fatalf("guard call = %#v, want strict guard profile", calls[2])
	}
}

func TestBenchCommandDispatchesCompareQuickProfile(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var gotArgs []string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"compare", "--quick"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "timing_compare.py")) ||
		!containsString(gotArgs, "control/sieve") ||
		!containsString(gotArgs, "table/table_array_access") ||
		!containsString(gotArgs, "--timeout") {
		t.Fatalf("args = %#v, want timing_compare.py quick compare profile", gotArgs)
	}
}

func TestBenchCommandDispatchesStrictHarness(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var gotArgs []string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"strict", "--group", "table"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) != 3 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "strict_guard.py")) || gotArgs[1] != "--group" || gotArgs[2] != "table" {
		t.Fatalf("args = %#v, want strict_guard.py --group table", gotArgs)
	}
}

func TestBenchCommandDispatchesDiagnoseHarness(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var gotArgs []string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"diagnose", "--bench", "control/sieve", "--no-timing"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) != 4 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "diagnose.py")) || gotArgs[1] != "--bench" || gotArgs[2] != "control/sieve" || gotArgs[3] != "--no-timing" {
		t.Fatalf("args = %#v, want diagnose.py --bench control/sieve --no-timing", gotArgs)
	}
}

func TestBenchAuditCommandReportsSections(t *testing.T) {
	payload := map[string]any{
		"results": []map[string]any{
			{
				"benchmark": "fast",
				"default":   map[string]any{"seconds": 0.002, "exit_total": 0},
				"luajit":    map[string]any{"status": "ok", "seconds": 0.004},
			},
			{
				"benchmark": "missing_ref",
				"default":   map[string]any{"seconds": 0.020, "exit_total": 25},
				"luajit":    map[string]any{"status": "missing", "seconds": nil},
			},
			{
				"benchmark": "tiny",
				"default":   map[string]any{"seconds": 0.0, "exit_total": 0},
				"luajit":    map[string]any{"status": "ok", "seconds": 0.010},
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "guard.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"audit", path, "--low-resolution-cutoff", "0.001", "--exit-cutoff", "20"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand audit code = %d, stderr = %q", code, stderr.String())
	}
	report := stdout.String()
	for _, want := range []string{
		"## Confirmed LuaJIT Comparisons",
		"| fast | 0.002s | 0.004s | 0.50x |",
		"| missing_ref | missing | 0.020s |",
		"| tiny | 0.000s | Needs calibrated repeats or ns/op bench |",
		"| missing_ref | 25 |",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("audit report missing %q:\n%s", want, report)
		}
	}
}

func TestBenchAuditCommandWritesMarkdown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guard.json")
	if err := os.WriteFile(path, []byte(`{"results":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(t.TempDir(), "audit.md")
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"audit", path, "--markdown", outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand audit code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty when --markdown is used", stdout.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# Benchmark Audit") {
		t.Fatalf("audit markdown = %q, want report", string(data))
	}
}

func TestBenchAuditRejectsPreviousSchemaResultsMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "previous_schema.json")
	if err := os.WriteFile(path, []byte(`{"results":{"fib":{}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"audit", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runBenchCommand audit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "guard JSON must contain a list-valued 'results'") {
		t.Fatalf("stderr = %q, want schema error", stderr.String())
	}
}

func TestBenchRankLuaJITGapsReportsMarkdownAndCSV(t *testing.T) {
	path := writeBenchRankFixture(t)
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"rank-luajit-gaps", path, "--top", "1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand rank-luajit-gaps code = %d, stderr = %q", code, stderr.String())
	}
	report := stdout.String()
	if !strings.Contains(report, "| 1 | slow | 0.120s | 0.060s | 0.090s | 0.030s | 2.00x | 3.00x | 2.00x | 4/3/1 | 7 |") {
		t.Fatalf("markdown report = %s", report)
	}
	if strings.Contains(report, "fast") {
		t.Fatalf("top=1 report included lower-ranked row: %s", report)
	}

	stdout.Reset()
	stderr.Reset()
	code = runBenchCommand([]string{"rank-luajit-gaps", path, "--format", "csv"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand rank-luajit-gaps csv code = %d, stderr = %q", code, stderr.String())
	}
	csv := stdout.String()
	if !strings.Contains(csv, "rank,benchmark,vm_seconds,default_seconds,no_filter_seconds,luajit_seconds,default_luajit_ratio") ||
		!strings.Contains(csv, "1,slow,0.12,0.06,0.09,0.03,2") ||
		!strings.Contains(csv, "2,fast,0.03,0.02,,0.04,0.5") {
		t.Fatalf("csv report = %s", csv)
	}
}

func TestBenchRankLuaJITGapsRejectsUnknownFormat(t *testing.T) {
	path := writeBenchRankFixture(t)
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"rank-luajit-gaps", path, "--format", "json"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runBenchCommand rank-luajit-gaps code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown format") {
		t.Fatalf("stderr = %q, want format error", stderr.String())
	}
}

func writeBenchRankFixture(t *testing.T) string {
	t.Helper()
	payload := map[string]any{
		"results": []map[string]any{
			{
				"benchmark": "fast",
				"vm":        map[string]any{"status": "ok", "seconds": 0.030},
				"default":   map[string]any{"status": "ok", "seconds": 0.020, "exit_total": 1},
				"luajit":    map[string]any{"status": "ok", "seconds": 0.040},
			},
			{
				"benchmark": "slow",
				"vm":        map[string]any{"status": "ok", "seconds": 0.120},
				"default":   map[string]any{"status": "ok", "seconds": 0.060, "t2_attempted": 4, "t2_entered": 3, "t2_failed": 1, "exit_total": 7},
				"no_filter": map[string]any{"status": "ok", "seconds": 0.090},
				"luajit":    map[string]any{"status": "ok", "seconds": 0.030},
			},
			{
				"benchmark": "missing_luajit",
				"default":   map[string]any{"status": "ok", "seconds": 0.010},
				"luajit":    map[string]any{"status": "missing"},
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "regression.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBenchDebugArtifactAggregatesExistingOutputs(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	td := t.TempDir()
	timing := filepath.Join(td, "timing.json")
	writeTestFile(t, timing, map[string]any{
		"results": []map[string]any{
			{
				"group":     "recursion",
				"benchmark": "fib",
				"modes": map[string]any{
					"default": map[string]any{
						"current": map[string]any{
							"status":     "ok",
							"source":     "script",
							"repeat":     4,
							"stats":      map[string]any{"median": 0.01},
							"t2_entered": 1,
							"exit_total": 2,
						},
					},
				},
			},
		},
	})
	exits := filepath.Join(td, "exits.json")
	writeTestFile(t, exits, map[string]any{
		"results": []map[string]any{
			{
				"benchmark": "fib",
				"status":    "ok",
				"stats": map[string]any{
					"by_exit_code": map[string]any{"ExitDeopt": 3},
					"sites":        []map[string]any{{"count": 3, "reason": "deopt:GuardType"}},
				},
			},
		},
	})
	runtimeStats := filepath.Join(td, "runtime.txt")
	if err := os.WriteFile(runtimeStats, []byte("Runtime Path Statistics:\n  native_call:\n    fast: 7\n    fallback: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	perfStats := filepath.Join(td, "perf.txt")
	if err := os.WriteFile(perfStats, []byte("Tier 2 Performance Diagnostics:\n  enabled: true\n  rows:\n    tier2_native_execution: count=2 total=100ns avg=50ns\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	spec := filepath.Join(td, "spec.json")
	writeTestFile(t, spec, []map[string]any{
		{
			"proto_name":       "fib",
			"compiled":         true,
			"version_hash":     "abc",
			"guard_count":      3,
			"suppressed_count": 2,
			"suppressed_pcs":   []int{4, 9},
			"suppressed_kinds": map[string]any{"GuardType": 1, "GuardConstString": 1},
		},
	})
	warm := filepath.Join(td, "warm")
	if err := os.MkdirAll(warm, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(warm, "manifest.json"), map[string]any{
		"protos": []map[string]any{{"name": "fib", "status": "entered", "entered": true, "compiled": true, "code_bytes": 32}},
	})
	writeTestFile(t, filepath.Join(warm, "pcmap.json"), map[string]any{
		"functions": []map[string]any{{"ranges": []map[string]any{{}, {}}}},
	})

	t.Chdir(root)
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"debug-artifact", "--benchmark-json", timing, "--exit-stats", exits, "--runtime-path-stats", runtimeStats, "--perf-stats", perfStats, "--spec-state", spec, "--warm-dump", warm, "--label", "unit"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand debug-artifact code = %d, stderr = %q", code, stderr.String())
	}
	var artifact map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &artifact); err != nil {
		t.Fatalf("debug artifact JSON failed to decode: %v\n%s", err, stdout.String())
	}
	if intFromJSONPath(t, artifact, "schema_version") != 1 {
		t.Fatalf("schema_version = %v, want 1", artifact["schema_version"])
	}
	if intFromJSONPath(t, artifact, "benchmark_summary.rows") != 1 ||
		intFromJSONPath(t, artifact, "benchmark_summary.total_exits") != 2 ||
		intFromJSONPath(t, artifact, "debug.exit_stats.total") != 3 ||
		intFromJSONPath(t, artifact, "debug.runtime_path_stats.numbers.native_call.fast") != 7 ||
		intFromJSONPath(t, artifact, "debug.tier2_perf_stats.total_nanos") != 100 ||
		intFromJSONPath(t, artifact, "debug.tier2_speculation_state.suppressed") != 2 ||
		intFromJSONPath(t, artifact, "debug.tier2_speculation_state.suppressed_kinds.GuardType") != 1 ||
		intFromJSONPath(t, artifact, "specialization.compiled") != 1 ||
		intFromJSONPath(t, artifact, "debug.warm_dump.pcmap_ranges") != 2 ||
		intFromJSONPath(t, artifact, "timing.summary.rows") != 1 ||
		intFromJSONPath(t, artifact, "tiering.t2_entered") != 1 ||
		intFromJSONPath(t, artifact, "exits.total") != 3 ||
		intFromJSONPath(t, artifact, "runtime_paths.numbers.native_call.fast") != 7 ||
		intFromJSONPath(t, artifact, "profiles.pcmap_ranges") != 2 ||
		intFromJSONPath(t, artifact, "gates.reason_counts.deopt:GuardType") != 3 {
		t.Fatalf("debug artifact missing expected rollups:\n%s", stdout.String())
	}
}

func TestBenchDebugArtifactWritesOutputFile(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	t.Chdir(root)
	outPath := filepath.Join(t.TempDir(), "artifact.json")
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"debug-artifact", "--out", outPath, "--label", "file"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand debug-artifact code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty when --out is used", stdout.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"schema_version": 1`) || !strings.Contains(string(data), `"label": "file"`) {
		t.Fatalf("artifact = %s", string(data))
	}
}

func writeTestFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func intFromJSONPath(t *testing.T, root map[string]any, path string) int {
	t.Helper()
	var value any = root
	parts := strings.Split(path, ".")
	for i, part := range parts {
		m, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("%s: %T is not object", path, value)
		}
		if joined := strings.Join(parts[i:], "."); m[joined] != nil {
			value = m[joined]
			break
		}
		value = m[part]
	}
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		t.Fatalf("%s: %T=%v is not numeric", path, value, value)
	}
	return 0
}
