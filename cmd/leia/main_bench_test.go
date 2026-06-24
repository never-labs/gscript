package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	jsonPath := filepath.Join(t.TempDir(), "compare.json")
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"compare", "--bench", "numeric/mandelbrot", "--runs", "1", "--warmup", "0", "--no-luajit", "--json", jsonPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if gotName == "" {
		t.Fatalf("compare did not invoke build command")
	}
	if !strings.Contains(stdout.String(), "wrote "+jsonPath) {
		t.Fatalf("stdout = %q, want JSON write", stdout.String())
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
	jsonPath := filepath.Join(t.TempDir(), "default.json")
	code := runBenchCommand([]string{"--quick", "--no-luajit", "--json", jsonPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) == 0 {
		t.Fatalf("default quick compare did not invoke build command")
	}
	if !strings.Contains(stdout.String(), "wrote "+jsonPath) {
		t.Fatalf("stdout = %q, want JSON write", stdout.String())
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
	jsonPath := filepath.Join(t.TempDir(), "selector.json")
	code := runBenchCommand([]string{"table/table_field_access", "--runs", "1", "--warmup", "0", "--no-luajit", "--json", jsonPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) == 0 {
		t.Fatalf("selector compare did not invoke build command")
	}
	if !strings.Contains(stdout.String(), "wrote "+jsonPath) {
		t.Fatalf("stdout = %q, want JSON write", stdout.String())
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
	if code := runBenchCommand([]string{"--quick", "--dry-run", "--no-luajit", "--json", filepath.Join(t.TempDir(), "quick.json")}, &stdout, &stderr); code != 0 {
		t.Fatalf("quick code = %d, stderr = %q", code, stderr.String())
	}
	if code := runBenchCommand([]string{"--full", "--dry-run", "--no-luajit", "--json", filepath.Join(t.TempDir(), "full.json")}, &stdout, &stderr); code != 0 {
		t.Fatalf("full code = %d, stderr = %q", code, stderr.String())
	}
	if code := runBenchCommand([]string{"--guard", "--dry-run", "--no-luajit", "--json", filepath.Join(t.TempDir(), "guard.json")}, &stdout, &stderr); code != 0 {
		t.Fatalf("guard code = %d, stderr = %q", code, stderr.String())
	}
	if len(calls) != 0 {
		t.Fatalf("dry-run invoked external commands: %#v", calls)
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
	jsonPath := filepath.Join(t.TempDir(), "quick-compare.json")
	code := runBenchCommand([]string{"compare", "--quick", "--no-luajit", "--json", jsonPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) == 0 {
		t.Fatalf("quick compare did not invoke build command")
	}
	if !strings.Contains(stdout.String(), "wrote "+jsonPath) {
		t.Fatalf("stdout = %q, want JSON write", stdout.String())
	}
}

func TestBenchCompareJobsKeepDeterministicResultOrder(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	jsonPath := filepath.Join(t.TempDir(), "jobs.json")
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{
		"compare",
		"--dry-run",
		"--jobs", "2",
		"--bench", "control/sieve",
		"--bench", "table/table_array_access",
		"--no-luajit",
		"--head-ref", "HEAD",
		"--json", jsonPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	var report struct {
		Results []map[string]any `json:"results"`
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Results) != 2 || report.Results[0]["group"] != "control" || report.Results[0]["benchmark"] != "sieve" || report.Results[1]["group"] != "table" || report.Results[1]["benchmark"] != "table_array_access" {
		t.Fatalf("results = %#v", report.Results)
	}
}

func TestBenchCompareRejectsInvalidJobs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"compare", "--jobs", "0", "--bench", "control/sieve"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runBenchCommand code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--jobs must be >= 1") {
		t.Fatalf("stderr = %q", stderr.String())
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
	jsonPath := filepath.Join(t.TempDir(), "strict.json")
	code := runBenchCommand([]string{"strict", "--group", "table", "--runs", "1", "--warmup", "0", "--no-luajit", "--json", jsonPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) == 0 {
		t.Fatalf("strict did not invoke build command")
	}
	if !strings.Contains(stdout.String(), "wrote "+jsonPath) {
		t.Fatalf("stdout = %q, want JSON write", stdout.String())
	}
}

func TestBenchCommandDispatchesDiagnoseHarness(t *testing.T) {
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"diagnose", "--bench", "control/sieve", "--no-timing", "--pprof", "--warm-dump", "--out-dir", outDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Wrote diagnostics:") {
		t.Fatalf("stdout = %q, want diagnostics path", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(outDir, "diagnostics.json")); err != nil {
		t.Fatalf("diagnostics.json missing: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(outDir, "diagnostics.json"))
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Benchmarks []map[string]any `json:"benchmarks"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Benchmarks) != 1 {
		t.Fatalf("benchmarks = %#v", report.Benchmarks)
	}
	runtimeSummary, _ := report.Benchmarks[0]["runtime_summary"].(map[string]any)
	tier2Summary, _ := report.Benchmarks[0]["tier2_call_summary"].(map[string]any)
	if runtimeSummary["status"] != "skipped" || runtimeSummary["readiness"] != "needs-attention" || runtimeSummary["has_exits"] != false {
		t.Fatalf("runtime_summary = %#v", runtimeSummary)
	}
	if tier2Summary["attempted"] != float64(0) || tier2Summary["entered_pct"] != float64(0) {
		t.Fatalf("tier2_call_summary = %#v", tier2Summary)
	}
	pprofSummary, _ := report.Benchmarks[0]["pprof_summary"].(map[string]any)
	warmDumpSummary, _ := report.Benchmarks[0]["warm_dump_summary"].(map[string]any)
	if report.Benchmarks[0]["pprof_requested"] != true || report.Benchmarks[0]["pprof_effective"] != false || pprofSummary["status"] != "not_collected" {
		t.Fatalf("pprof fields = row %#v summary %#v", report.Benchmarks[0], pprofSummary)
	}
	if report.Benchmarks[0]["warm_dump_requested"] != true || report.Benchmarks[0]["warm_dump_effective"] != false || warmDumpSummary["status"] != "not_collected" {
		t.Fatalf("warm dump fields = row %#v summary %#v", report.Benchmarks[0], warmDumpSummary)
	}
	md, err := os.ReadFile(filepath.Join(outDir, "diagnostics.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "pprof not_collected: pprof collection is not yet wired for bench diagnose") ||
		!strings.Contains(string(md), "warm-dump not_collected: warm dump collection is not yet wired for bench diagnose") {
		t.Fatalf("diagnostics markdown missing optional evidence state:\n%s", string(md))
	}
}

func TestBenchCommandDispatchesTriageHarness(t *testing.T) {
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"triage", "--bench", "numeric/spectral_norm", "--runs", "1", "--out-dir", outDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# Performance Triage") {
		t.Fatalf("stdout = %q, want triage markdown", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(outDir, "triage.json")); err != nil {
		t.Fatalf("triage.json missing: %v", err)
	}
}

func TestBenchTriageRowsKeepRuntimeCounters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compare.json")
	writeTestFile(t, path, map[string]any{
		"results": []map[string]any{
			{
				"group":     "control",
				"benchmark": "sieve",
				"modes": map[string]any{
					"default": map[string]any{
						"current": map[string]any{
							"status":       "ok",
							"source":       "script_repeat",
							"repeat":       8,
							"stats":        map[string]any{"median": 0.02},
							"t2_attempted": 5,
							"t2_entered":   4,
							"t2_failed":    1,
							"exit_total":   3,
						},
						"head":   map[string]any{"status": "ok", "stats": map[string]any{"median": 0.03}},
						"luajit": map[string]any{"status": "ok", "stats": map[string]any{"median": 0.01}},
					},
				},
			},
		},
	})
	rows, err := benchTriageRowsFromCompare(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["t2_attempted"] != float64(5) || rows[0]["t2_entered"] != float64(4) || rows[0]["t2_failed"] != float64(1) || rows[0]["exits"] != float64(3) {
		t.Fatalf("rows = %#v", rows)
	}
	bottlenecks := benchTriageBottlenecks(rows)
	if len(bottlenecks) == 0 {
		t.Fatalf("bottlenecks = %#v", bottlenecks)
	}
	artifacts := map[string]any{
		"timing_json": map[string]any{"path": "/tmp/timing.json", "status": "ok", "note": "compare JSON report"},
	}
	markdown := benchTriageMarkdown(rows, bottlenecks, benchTriageRecommendations(rows), artifacts)
	if !strings.Contains(markdown, "| control/sieve | default | 0.020000s | 0.030000s | 0.010000s | 5/4/1 | 3 | calibrated timing captured |") {
		t.Fatalf("markdown = %s", markdown)
	}
	if !strings.Contains(markdown, "| P1 | control/sieve | default | tier2-failed | high | t2_failed=1 | run bench diagnose and inspect Tier 2 failure reasons |") {
		t.Fatalf("markdown missing tier2 bottleneck: %s", markdown)
	}
	if !strings.Contains(markdown, "| timing_json | ok | /tmp/timing.json | compare JSON report |") {
		t.Fatalf("markdown missing artifact status: %s", markdown)
	}
}

func TestBenchTriageTopLevelSummariesAreStructured(t *testing.T) {
	rows := []map[string]any{
		{
			"benchmark": "control/sieve",
			"mode":      "default",
			"exits":     float64(3),
			"status":    map[string]any{"current": "ok", "head": "ok", "luajit": "missing"},
		},
	}
	exitSummary := benchTriageExitSummary(rows)
	statuses, _ := exitSummary["statuses"].(map[string]int)
	if exitSummary["total"] != 3 || statuses["current:ok"] != 1 || statuses["luajit:missing"] != 1 {
		t.Fatalf("exit summary = %#v", exitSummary)
	}
	pprof := benchTriageNotCollectedSummary("unit")
	if pprof["status"] != "not_collected" || pprof["reason"] != "unit" {
		t.Fatalf("not collected summary = %#v", pprof)
	}
}

func TestBenchCommandRunsGoQReportFromOutput(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		t.Fatalf("q-report --from-output invoked external command %s %#v", name, args)
		return exec.Command("false")
	}

	dir := t.TempDir()
	input := filepath.Join(dir, "qbench.txt")
	if err := os.WriteFile(input, []byte(strings.TrimSpace(`
BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject-16    100  1000 ns/op  8192 input_rows/s  200 B/op  3 allocs/op  99.0 kernel_hit_pct  0 fallbacks/op
BenchmarkQSQLNativeGoSelectWhereProject-16               100  2000 ns/op  8192 input_rows/s  100 B/op  1 allocs/op
BenchmarkQSessionEvalVectorWarmExecution/MaskWhere-16    100  2000 ns/op  256 B/op  8 allocs/op  100.0 typed_kernel_hit_pct  1 typed_kernel_attempts/op  1 typed_kernel_hits/op  0 typed_kernel_fallbacks/op  0 typed_kernel_errors/op  2 typed_pipeline_shapes  0 typed_pipeline_fallback_shapes
BenchmarkQEvalVectorGoBaseline/MaskWhere-16              100  1000 ns/op  0 B/op  0 allocs/op
BenchmarkQEvalJITScriptWarm/MaskWhere-16                 100  1500 ns/op  128 B/op  4 allocs/op  1 q_session_planned_op_exit/op  0 q_session_shell_fallback/op  0 q_session_eval_errors/op  1 q_session_backend_shapes
`)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(dir, "q-report.json")
	mdPath := filepath.Join(dir, "q-report.md")

	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"q-report", "--from-output", input, "--json", jsonPath, "--markdown", mdPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wrote "+mdPath) || !strings.Contains(stdout.String(), "wrote "+jsonPath) {
		t.Fatalf("stdout = %q, want written report paths", stdout.String())
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("q-report JSON failed to decode: %v\n%s", err, string(data))
	}
	benchmarks := payload["benchmarks"].(map[string]any)
	if _, ok := benchmarks["BenchmarkQSessionEvalVectorWarmExecution/MaskWhere"]; !ok {
		t.Fatalf("benchmarks keys = %#v, want parsed q.eval row", benchmarks)
	}
	runtimeMetrics := payload["runtime_metrics"].([]any)
	if len(runtimeMetrics) == 0 || runtimeMetrics[0].(map[string]any)["data_runtime_hit_pct"] != nil {
		t.Fatalf("runtime metrics = %#v, want custom metric schema with nil missing values", runtimeMetrics)
	}
	markdown, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdown), "q Performance Completeness Report") {
		t.Fatalf("markdown = %q, want q report title", string(markdown))
	}
}

func TestBenchQReportCheckFailsObviousRuntimeRegression(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "qbench.txt")
	if err := os.WriteFile(input, []byte(strings.TrimSpace(`
BenchmarkQSessionEvalVectorWarmExecution/FallbackShape-16    100  9000 ns/op  2048 B/op  90 allocs/op  80.0 typed_kernel_hit_pct  1 typed_kernel_attempts/op  0.8 typed_kernel_hits/op  0.2 typed_kernel_fallbacks/op  0 typed_kernel_errors/op  2 typed_pipeline_shapes  1 typed_pipeline_fallback_shapes
BenchmarkQEvalVectorGoBaseline/FallbackShape-16              100  1000 ns/op  0 B/op  0 allocs/op
`)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(dir, "q-report.json")
	mdPath := filepath.Join(dir, "q-report.md")
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{
		"q-report",
		"--from-output", input,
		"--check",
		"--ratio-baseline", filepath.Join(dir, "missing-baseline.json"),
		"--json", jsonPath,
		"--markdown", mdPath,
		"--min-runtime-jit-backend-benchmarks=0",
		"--min-runtime-array-bridge-benchmarks=0",
		"--min-runtime-bridge-benchmark-count=0",
		"--min-runtime-backend-route-benchmarks=0",
		"--min-runtime-backend-route-hits-op=0",
		"--min-q-eval-family-cases=0",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runBenchCommand code = %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "q performance gate failed") {
		t.Fatalf("stderr = %q, want gate failure", stderr.String())
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("q-report JSON failed to decode: %v\n%s", err, string(data))
	}
	var failed []string
	for _, item := range payload["gate"].([]any) {
		check := item.(map[string]any)
		if check["status"] == "fail" {
			failed = append(failed, check["signal"].(string))
		}
	}
	if !containsString(failed, "typed_hit_pct") || !containsString(failed, "allocs_op") {
		t.Fatalf("failed signals = %#v, want typed_hit_pct and allocs_op", failed)
	}
}

func TestBenchCommandDispatchesQSuiteHarness(t *testing.T) {
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
	code := runBenchCommand([]string{"q-suite", "--runs=1", "--warmup=0"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if gotName != "bash" {
		t.Fatalf("command = %q, want bash", gotName)
	}
	if len(gotArgs) != 3 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "q_performance_suite.sh")) || gotArgs[1] != "--runs=1" || gotArgs[2] != "--warmup=0" {
		t.Fatalf("args = %#v, want q_performance_suite.sh --runs=1 --warmup=0", gotArgs)
	}
}

func TestBenchCommandDispatchesQColumnarHarness(t *testing.T) {
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
	code := runBenchCommand([]string{"q-columnar", "--runs=1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if gotName != "bash" {
		t.Fatalf("command = %q, want bash", gotName)
	}
	if len(gotArgs) != 2 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "q_columnar_suite.sh")) || gotArgs[1] != "--runs=1" {
		t.Fatalf("args = %#v, want q_columnar_suite.sh --runs=1", gotArgs)
	}
}

func TestBenchCommandDispatchesQGeneralHarness(t *testing.T) {
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
	code := runBenchCommand([]string{"q-general"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if gotName != "bash" {
		t.Fatalf("command = %q, want bash", gotName)
	}
	if len(gotArgs) != 1 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "q_general_compute_suite.sh")) {
		t.Fatalf("args = %#v, want q_general_compute_suite.sh", gotArgs)
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

func TestBenchCoverageCommandAcceptsCurrentRepositoryMap(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	t.Chdir(root)
	cases := benchCoverageConformanceCases(root)
	known := benchCoverageBenchmarkIDs(root)
	if missing := benchCoverageMissingFamilies(cases); len(missing) != 0 {
		t.Fatalf("missing coverage families = %v", missing)
	}
	if missing := benchCoverageMissingRefs(cases, known); len(missing) != 0 {
		t.Fatalf("missing benchmark refs = %v", missing)
	}
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"coverage", "--check"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand coverage code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "# Language Conformance Performance Coverage") ||
		!strings.Contains(stdout.String(), "| Family | Cases | Status | Existing Hot Benchmarks |") {
		t.Fatalf("coverage markdown missing expected sections:\n%s", stdout.String())
	}
}

func TestBenchCoverageCommandWritesJSONAndMarkdown(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	t.Chdir(root)
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "coverage.json")
	mdPath := filepath.Join(dir, "coverage.md")
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"coverage", "--check", "--json", jsonPath, "--markdown", mdPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand coverage code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty when --markdown is used", stdout.String())
	}
	md, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "# Language Conformance Performance Coverage") {
		t.Fatalf("markdown = %q, want report", string(md))
	}
	var payload []map[string]any
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("coverage JSON failed to decode: %v\n%s", err, string(data))
	}
	if len(payload) == 0 {
		t.Fatal("coverage JSON is empty")
	}
}

func TestBenchCoverageReportsMissingFamilyAndRefs(t *testing.T) {
	cases := map[string][]string{"newfamily": nil, "math": nil}
	if got := benchCoverageMissingFamilies(cases); len(got) != 1 || got[0] != "newfamily" {
		t.Fatalf("missing families = %v, want newfamily", got)
	}
	refs := benchCoverageMissingRefs(map[string][]string{"math": nil}, map[string]bool{})
	if !containsString(refs, "numeric/math_intensive") {
		t.Fatalf("missing refs = %v, want numeric/math_intensive", refs)
	}
}

func TestBenchProfileExitsParsesTimeAndExitJSON(t *testing.T) {
	output := "warmup\nTime: 0.125s\n{\n  \"total\": 2,\n  \"by_exit_code\": {\"ExitDeopt\": 2},\n  \"sites\": []\n}\n"
	stats, err := benchProfileExtractExitJSON(output)
	if err != nil {
		t.Fatal(err)
	}
	if got := benchDebugToInt(stats["total"]); got != 2 {
		t.Fatalf("total = %d, want 2", got)
	}
	seconds := benchProfileParseTime(output)
	if seconds == nil || *seconds != 0.125 {
		t.Fatalf("seconds = %v, want 0.125", seconds)
	}
}

func TestBenchProfileExitsMarkdownAggregates(t *testing.T) {
	seconds := 0.125
	report := benchProfileMarkdown([]benchProfileExitResult{
		{
			Benchmark: "numeric/spectral_norm",
			Status:    "ok",
			Seconds:   &seconds,
			Stats: map[string]any{
				"total":        3,
				"by_exit_code": map[string]any{"ExitDeopt": 3},
				"sites": []any{map[string]any{
					"count":     3,
					"proto":     "main",
					"exit_name": "ExitDeopt",
					"pc":        7,
					"op_id":     11,
					"reason":    "guard:type",
				}},
			},
		},
		{Benchmark: "missing/bench", Status: "missing"},
	}, 10)
	for _, want := range []string{
		"| numeric/spectral_norm | 0.125s | 3 | ExitDeopt=3 |",
		"| missing/bench | missing | - | - |",
		"| ExitDeopt | 3 |",
		"| guard:type | 3 |",
		"| 3 | numeric/spectral_norm | main | ExitDeopt | 7 | 11 | guard:type |",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("profile report missing %q:\n%s", want, report)
		}
	}
}

func TestBenchProfileExitsCommandRunsSelectedBenchmark(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	td := t.TempDir()
	benchDir := filepath.Join(td, "benchmarks", "numeric")
	if err := os.MkdirAll(benchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(benchDir, "unit.leia"), []byte("print(1)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "go.mod"), filepath.Join(td, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "cmd"), filepath.Join(td, "cmd")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(td)

	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var calls [][]string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string{name}, args...))
		switch name {
		case "go":
			return exec.Command("true")
		default:
			helper, helperArgs := testHelperCommand(t, "bench-exit-profile")
			return exec.Command(helper, helperArgs...)
		}
	}

	jsonPath := filepath.Join(td, "out", "profile.json")
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"profile-exits", "--bench", "numeric/unit", "--json", jsonPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand profile-exits code = %d, stderr = %q", code, stderr.String())
	}
	if len(calls) != 2 || calls[0][0] != "go" || calls[1][0] == "" || !strings.HasSuffix(calls[1][len(calls[1])-1], filepath.Join("benchmarks", "numeric", "unit.leia")) {
		t.Fatalf("calls = %#v, want build then leia run on numeric/unit.leia", calls)
	}
	if !strings.Contains(stdout.String(), "| numeric/unit | 0.125s | 3 | ExitDeopt=3 |") {
		t.Fatalf("stdout = %s", stdout.String())
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"mode": "default"`) || !strings.Contains(string(data), `"benchmark": "numeric/unit"`) {
		t.Fatalf("json = %s", string(data))
	}
}

func TestBenchValidateLuaRefsCommandRunsReferences(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	td := t.TempDir()
	luaDir := filepath.Join(td, "benchmarks", "lua_ref", "numeric")
	if err := os.MkdirAll(luaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(luaDir, "unit.lua"), []byte("print('Time: 0.010s')\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "go.mod"), filepath.Join(td, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "cmd"), filepath.Join(td, "cmd")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(td)

	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var gotName string
	var gotArgs []string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "lua-ref")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"validate-lua-refs", "--lua-bin", "luajit-test"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand validate-lua-refs code = %d, stderr = %q", code, stderr.String())
	}
	if gotName != "luajit-test" || len(gotArgs) != 1 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "lua_ref", "numeric", "unit.lua")) {
		t.Fatalf("lua command = %q %#v, want luajit-test numeric/unit.lua", gotName, gotArgs)
	}
	if !strings.Contains(stdout.String(), "numeric/unit: ok") || !strings.Contains(stdout.String(), "Validated 1 Lua references.") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestBenchValidateLuaRefsReportsNoTime(t *testing.T) {
	td := t.TempDir()
	luaDir := filepath.Join(td, "benchmarks", "lua_ref", "numeric")
	if err := os.MkdirAll(luaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(luaDir, "unit.lua"), []byte("print('no timing')\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("printf", "no timing\n")
	}
	status, message := benchValidateLuaRef(td, "lua", "numeric/unit", time.Second)
	if status != "no_time" || !strings.Contains(message, "no parseable Time") {
		t.Fatalf("status/message = %s %q, want no_time", status, message)
	}
}

func TestBenchSubmitGuardRejectsLuaJITRatio(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	t.Chdir(root)
	path := writeBenchTimingPayload(t, []benchTimingFixture{{Name: "numeric/matmul_dense", Current: 0.81, LuaJIT: 1.0}})
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"submit-guard", path, "--ratio-threshold", "0.8"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("submit-guard code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "luajit") || !strings.Contains(stdout.String(), "numeric/matmul_dense") {
		t.Fatalf("stdout = %q, want luajit violation", stdout.String())
	}
}

func TestBenchSubmitGuardRejectsBaselineRegression(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	t.Chdir(root)
	candidate := writeBenchTimingPayload(t, []benchTimingFixture{{Name: "numeric/matmul_dense", Current: 0.75, LuaJIT: 1.0}})
	baseline := writeBenchTimingPayload(t, []benchTimingFixture{{Name: "numeric/matmul_dense", Current: 0.70, LuaJIT: 1.0}})
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"submit-guard", candidate, "--baseline", baseline, "--ratio-threshold", "0.8", "--regression-tolerance", "0.03"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("submit-guard code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "regression") || !strings.Contains(stdout.String(), "+7.14%") {
		t.Fatalf("stdout = %q, want regression violation", stdout.String())
	}
}

func TestBenchSubmitGuardSkipsMixedSourcesAndLeiaOnlyLuaJIT(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	t.Chdir(root)
	path := writeBenchTimingPayload(t, []benchTimingFixture{
		{Name: "numeric/matmul_dense", Current: 0.02, LuaJIT: 0.01, CurrentSource: "wall_repeat", LuaJITSource: "script_repeat"},
		{Name: "data/q_columnar_qsql_filter_project", Current: 0.42, LuaJITStatus: "missing", CurrentSource: "script_repeat"},
	})
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"submit-guard", path, "--ratio-threshold", "0.8"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("submit-guard code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "Guard passed.") || strings.Contains(stdout.String(), "numeric/matmul_dense") {
		t.Fatalf("stdout = %q, want mixed source omitted from ratio table and guard pass", stdout.String())
	}
}

func TestBenchJITAddrMapResolvesExplicitPC(t *testing.T) {
	warm := filepath.Join(t.TempDir(), "warm")
	if err := os.MkdirAll(warm, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(warm, "pcmap.json"), map[string]any{
		"functions": []map[string]any{{
			"name": "main",
			"ranges": []map[string]any{{
				"pc_start":    "0x1000",
				"pc_end":      "0x1010",
				"ir_instr":    7,
				"ir_op":       "AddInt",
				"block":       2,
				"bytecode_pc": 4,
				"bytecode_op": "ADD",
				"source_line": 12,
				"pass":        "lower",
				"proto":       "main",
			}},
		}},
	})
	outPath := filepath.Join(t.TempDir(), "pcmap.json")
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"jit-addr-map", "--warm-dir", warm, "--pc", "0x1008", "--json", outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("jit-addr-map code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "| - | - | main | 7 | AddInt | 2 | 4 | lower | 0x1008 |") {
		t.Fatalf("stdout = %s", stdout.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"status": "ok"`) || !strings.Contains(string(data), `"mapped_rows": 1`) {
		t.Fatalf("json = %s", string(data))
	}
}

func TestBenchJITAddrMapAggregatesPprofRaw(t *testing.T) {
	warm := filepath.Join(t.TempDir(), "warm")
	if err := os.MkdirAll(warm, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(warm, "pcmap.json"), map[string]any{
		"functions": []map[string]any{{
			"name": "hot",
			"ranges": []map[string]any{{
				"pc_start":    0x2000,
				"pc_end":      0x2010,
				"ir_instr":    3,
				"ir_op":       "Call",
				"block":       1,
				"bytecode_pc": 9,
				"pass":        "emit",
			}},
		}},
	})
	raw := filepath.Join(t.TempDir(), "raw.txt")
	if err := os.WriteFile(raw, []byte("Samples:\n  5 1000: 1\nLocations\n  1: 0x2004 M=1 runtime._ExternalCode /tmp/x.go:1:0 s=1\nMappings\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(t.TempDir(), "mapped.json")
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"jit-addr-map", "--warm-dir", warm, "--pprof-raw", raw, "--json", outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("jit-addr-map code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "| 5 | 0.000001s | hot | 3 | Call | 1 | 9 | emit | 0x2004 |") {
		t.Fatalf("stdout = %s", stdout.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"profile_samples": 5`) || !strings.Contains(string(data), `"external_code_samples": 5`) {
		t.Fatalf("json = %s", string(data))
	}
}

func TestBenchRegressionGuardParsesSampleAndBaseline(t *testing.T) {
	sample := benchRegressionParseSample("Time: 0.010s\n  Tier 2 attempted: 3\n  Tier 2 entered:  1 functions\n  Tier 2 failed: 1 functions\n  total exits: 7\n", "ok", intPtr(0))
	if sample.Seconds == nil || *sample.Seconds != 0.01 || sample.T2Attempted != 3 || sample.T2Entered != 1 || sample.T2Failed != 1 || sample.ExitTotal != 7 {
		t.Fatalf("sample = %+v", sample)
	}
	if sec, ok := benchRegressionParseSeconds("2.5ms"); !ok || sec != 0.0025 {
		t.Fatalf("parse seconds = %v %v, want 0.0025 true", sec, ok)
	}
	path := filepath.Join(t.TempDir(), "baseline.json")
	writeTestFile(t, path, map[string]any{"results": map[string]any{"fib": map[string]any{"jit": "Time: 1.500s"}}})
	if got := benchRegressionLoadBaseline(path); got["fib"] != 1.5 {
		t.Fatalf("baseline = %v, want fib=1.5", got)
	}
}

func TestBenchRegressionGuardSummarizesPartialSuccess(t *testing.T) {
	a, b := 0.3, 0.1
	result := benchRegressionSummarizeSamples([]benchRegressionSample{
		{Status: "timeout"},
		{Status: "ok", Seconds: &a, T2Attempted: 2, T2Entered: 1},
		{Status: "ok", Seconds: &b, T2Attempted: 4, T2Entered: 3},
	})
	if result.Status != "partial" || result.Seconds == nil || *result.Seconds != 0.2 || result.T2Attempted != 4 || result.T2Entered != 3 {
		t.Fatalf("summary = %+v", result)
	}
}

func TestBenchGoHarnessParsesAndReportsRuntimeCounters(t *testing.T) {
	output := "Time: 0.010s\n  Tier 2 attempted: 3\n  Tier 2 entered:  1 functions\n  Tier 2 failed: 1 functions\n  total exits: 7\n"
	if got := benchParseCounter(benchT2AttemptedRE, output); got != 3 {
		t.Fatalf("t2 attempted = %d, want 3", got)
	}
	if got := benchParseCounter(benchT2EnteredRE, output); got != 1 {
		t.Fatalf("t2 entered = %d, want 1", got)
	}
	if got := benchParseCounter(benchT2FailedRE, output); got != 1 {
		t.Fatalf("t2 failed = %d, want 1", got)
	}
	if got := benchParseCounter(benchExitTotalRE, output); got != 7 {
		t.Fatalf("exit total = %d, want 7", got)
	}

	a, b := 0.3, 0.1
	result := summarizeBenchGoSubject("current", "default", []benchGoSample{
		{Status: "timeout", T2Attempted: 99},
		{Status: "ok", Seconds: &a, Source: "script_repeat", T2Attempted: 2, T2Entered: 1, ExitTotal: 4},
		{Status: "ok", Seconds: &b, Source: "script_repeat", T2Attempted: 4, T2Entered: 3, T2Failed: 1, ExitTotal: 7},
	}, 1)
	if result.Status != "partial" || result.Stats.Median == nil || *result.Stats.Median != 0.2 || result.T2Attempted != 2 || result.T2Entered != 1 || result.T2Failed != 0 || result.ExitTotal != 4 {
		t.Fatalf("summary = %+v", result)
	}

	row := benchGoBenchmarkResult{
		Group:     "control",
		Benchmark: "sieve",
		Modes: map[string]map[string]benchGoSubjectResult{
			"default": {
				"current": result,
				"head":    {Status: "ok", Stats: benchGoComputeStats([]float64{0.4})},
				"luajit":  {Status: "ok", Stats: benchGoComputeStats([]float64{0.05})},
			},
		},
	}
	markdown := benchGoMarkdown(benchGoHarnessConfig{Mode: "compare", Modes: []string{"default"}}, []benchGoBenchmarkResult{row})
	if !strings.Contains(markdown, "| control/sieve | default | 0.200000s | 0.400000s | 0.050000s | script_repeat | 2/1/0 | 4 |") {
		t.Fatalf("markdown = %s", markdown)
	}
}

func TestBenchGoHarnessSortsLuaJITGapDescending(t *testing.T) {
	row := func(group, name string, current, luajit float64) benchGoBenchmarkResult {
		return benchGoBenchmarkResult{
			Group:     group,
			Benchmark: name,
			Modes: map[string]map[string]benchGoSubjectResult{
				"default": {
					"current": {Status: "ok", Stats: benchGoComputeStats([]float64{current})},
					"luajit":  {Status: "ok", Stats: benchGoComputeStats([]float64{luajit})},
				},
			},
		}
	}
	rows := []benchGoBenchmarkResult{
		row("table", "fast_gap", 0.20, 0.10),
		row("control", "slow_gap", 0.90, 0.10),
		row("app", "missing_luajit", 0.10, 0),
	}
	rows[2].Modes["default"]["luajit"] = benchGoSubjectResult{Status: "skip"}

	sorted := sortBenchGoResults(rows, benchGoHarnessConfig{Mode: "compare", Modes: []string{"default"}, Sort: "luajit-gap"})
	if got := []string{benchGoRowID(sorted[0]), benchGoRowID(sorted[1]), benchGoRowID(sorted[2])}; !reflect.DeepEqual(got, []string{"control/slow_gap", "table/fast_gap", "app/missing_luajit"}) {
		t.Fatalf("sorted rows = %v", got)
	}
}

func TestBenchGoHarnessRejectsUnknownSort(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseBenchGoHarnessConfig("compare", []string{"--sort", "latency"}, &stderr)
	if err == nil || !strings.Contains(err.Error(), `unknown --sort "latency"`) {
		t.Fatalf("err = %v, stderr = %q", err, stderr.String())
	}
}

func TestBenchRegressionGuardWritesCSVAndMarkdown(t *testing.T) {
	vm, def, lua, base, pct := 1.0, 0.5, 0.25, 0.4, 25.0
	row := benchRegressionResult{
		Benchmark:       "fib",
		VM:              &benchRegressionMode{Status: "ok", Seconds: &vm},
		Default:         &benchRegressionMode{Status: "ok", Seconds: &def, T2Attempted: 2, T2Entered: 1},
		LuaJIT:          &benchRegressionMode{Status: "ok", Seconds: &lua},
		BaselineSeconds: &base,
		RegressionPct:   &pct,
		Regression:      true,
	}
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "guard.csv")
	if err := benchRegressionWriteCSV(csvPath, []benchRegressionResult{row}); err != nil {
		t.Fatal(err)
	}
	csvData, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(csvData), "benchmark,vm_seconds") || !strings.Contains(string(csvData), "fib,1,0.5") {
		t.Fatalf("csv = %s", string(csvData))
	}
	markdown := benchRegressionMarkdown([]benchRegressionResult{row}, 10.0)
	if !strings.Contains(markdown, "| fib | 1.000s | 0.500s") || !strings.Contains(markdown, "REG +25.0%") {
		t.Fatalf("markdown = %s", markdown)
	}
}

func TestBenchRegressionGuardCommandRunsSelectedBenchmark(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	td := t.TempDir()
	benchDir := filepath.Join(td, "benchmarks", "numeric")
	luaDir := filepath.Join(td, "benchmarks", "lua_ref", "numeric")
	if err := os.MkdirAll(benchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(luaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(benchDir, "unit.leia"), []byte("print(1)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(luaDir, "unit.lua"), []byte("print('Time: 0.1s')\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "go.mod"), filepath.Join(td, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "cmd"), filepath.Join(td, "cmd")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(td)

	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var runCalls int
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		if name == "go" {
			return exec.Command("true")
		}
		runCalls++
		helper, helperArgs := testHelperCommand(t, "bench-regression")
		return exec.Command(helper, helperArgs...)
	}

	jsonPath := filepath.Join(td, "out", "guard.json")
	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"regression-guard", "--bench", "numeric/unit", "--runs", "1", "--no-luajit", "--json", jsonPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("regression-guard code = %d, stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if runCalls != 3 {
		t.Fatalf("run calls = %d, want vm/default/no_filter", runCalls)
	}
	if !strings.Contains(stdout.String(), "numeric/unit") || !strings.Contains(stdout.String(), "Wrote JSON:") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"benchmark": "numeric/unit"`) || !strings.Contains(string(data), `"t2_attempted": 3`) {
		t.Fatalf("json = %s", string(data))
	}
}

type benchTimingFixture struct {
	Name          string
	Current       float64
	LuaJIT        float64
	CurrentSource string
	LuaJITSource  string
	LuaJITStatus  string
}

func writeBenchTimingPayload(t *testing.T, rows []benchTimingFixture) string {
	t.Helper()
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		group, bench, _ := strings.Cut(row.Name, "/")
		current := map[string]any{"status": "ok", "stats": map[string]any{"median": row.Current}}
		if row.CurrentSource != "" {
			current["source"] = row.CurrentSource
		}
		luaStatus := row.LuaJITStatus
		if luaStatus == "" {
			luaStatus = "ok"
		}
		luajit := map[string]any{"status": luaStatus}
		if row.LuaJIT > 0 {
			luajit["stats"] = map[string]any{"median": row.LuaJIT}
		}
		if row.LuaJITSource != "" {
			luajit["source"] = row.LuaJITSource
		}
		results = append(results, map[string]any{
			"group":     group,
			"benchmark": bench,
			"modes": map[string]any{
				"default": map[string]any{
					"current": current,
					"luajit":  luajit,
				},
			},
		})
	}
	path := filepath.Join(t.TempDir(), "timing.json")
	writeTestFile(t, path, map[string]any{"results": results})
	return path
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
