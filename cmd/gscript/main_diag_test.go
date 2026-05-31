package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagCommandDispatchesDumpScript(t *testing.T) {
	oldDiagExecCommand := diagExecCommand
	t.Cleanup(func() { diagExecCommand = oldDiagExecCommand })
	var gotName string
	var gotArgs []string
	diagExecCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "diag")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runDiagCommand([]string{"dump", "control/sieve"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDiagCommand code = %d, stderr = %q", code, stderr.String())
	}
	if gotName != "bash" {
		t.Fatalf("command = %q, want bash", gotName)
	}
	if len(gotArgs) != 2 || !strings.HasSuffix(gotArgs[0], filepath.Join("scripts", "diag.sh")) || gotArgs[1] != "control/sieve" {
		t.Fatalf("args = %#v, want scripts/diag.sh control/sieve", gotArgs)
	}
	if !strings.Contains(stdout.String(), "diag helper ok") {
		t.Fatalf("stdout = %q, want helper output", stdout.String())
	}
}

func TestDiagCommandDispatchesBundleScript(t *testing.T) {
	oldDiagExecCommand := diagExecCommand
	t.Cleanup(func() { diagExecCommand = oldDiagExecCommand })
	var gotArgs []string
	diagExecCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "diag")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runDiagCommand([]string{"bundle", "--skip-benchmarks"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDiagCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) != 2 || !strings.HasSuffix(gotArgs[0], filepath.Join("scripts", "diagnostics_bundle.sh")) || gotArgs[1] != "--skip-benchmarks" {
		t.Fatalf("args = %#v, want diagnostics_bundle.sh --skip-benchmarks", gotArgs)
	}
}

func TestDiagCommandRejectsUnknownMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runDiagCommand([]string{"unknown"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runDiagCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unknown diag mode") {
		t.Fatalf("stderr = %q, want unknown mode", stderr.String())
	}
}

func TestDiagnoseCommandDispatchesBenchmarkSelector(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var gotArgs []string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runDiagnoseCommand([]string{"table/table_field_access", "--no-timing"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDiagnoseCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) != 4 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "diagnose.py")) || gotArgs[1] != "--bench" || gotArgs[2] != "table/table_field_access" || gotArgs[3] != "--no-timing" {
		t.Fatalf("args = %#v, want diagnose.py --bench table/table_field_access --no-timing", gotArgs)
	}
}
