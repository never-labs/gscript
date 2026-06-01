package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBenchCommandManifestCheckDispatchesBenchmarksManifest(t *testing.T) {
	oldCheckExecCommand := checkExecCommand
	t.Cleanup(func() { checkExecCommand = oldCheckExecCommand })
	var gotArgs []string
	checkExecCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "manifest")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"--manifest-check"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) != 3 || !strings.HasSuffix(gotArgs[0], filepath.Join("tests", "manifest.py")) || gotArgs[1] != "check" || gotArgs[2] != "benchmarks" {
		t.Fatalf("args = %#v, want tests/manifest.py check benchmarks", gotArgs)
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
