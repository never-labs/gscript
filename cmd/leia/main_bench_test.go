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
