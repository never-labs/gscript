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

func TestCICommandListsProfiles(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCICommand([]string{"smoke", "--list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runCICommand code = %d, stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"go test", "./cmd/leia-lsp", "./internal/tooling/lsp", "tests/manifest.py", "github.com/never-labs/leia", "leia check", "--no-docs", "--no-editor", "tests/smoke/01_basic.leia", "worktree_audit.sh"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout = %q, want %q", out, want)
		}
	}
}

func TestCICommandPRProfileIncludesExampleCheck(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCICommand([]string{"pr", "--list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runCICommand code = %d, stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"leia examples check", "--jobs=6", "performance_gate.sh"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout = %q, want %q", out, want)
		}
	}
}

func TestCICommandRunsProfileCommands(t *testing.T) {
	oldCIExecCommand := ciExecCommand
	t.Cleanup(func() { ciExecCommand = oldCIExecCommand })
	var commands []string
	ciExecCommand = func(name string, args ...string) *exec.Cmd {
		commands = append(commands, shellJoin(append([]string{name}, args...)))
		helper, helperArgs := testHelperCommand(t, "ci")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runCICommand([]string{"perf", "--no-luajit"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runCICommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(commands) != 1 || !strings.Contains(commands[0], "performance_gate.sh") || !strings.Contains(commands[0], "--no-luajit") {
		t.Fatalf("commands = %#v, want performance gate with --no-luajit", commands)
	}
	if !strings.Contains(stdout.String(), "ci helper ok") {
		t.Fatalf("stdout = %q, want helper output", stdout.String())
	}
}

func TestCICommandRejectsUnknownProfile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCICommand([]string{"nightly"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runCICommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unknown ci profile") {
		t.Fatalf("stderr = %q, want unknown profile", stderr.String())
	}
}

func TestCheckCommandJSONRunsEnabledSteps(t *testing.T) {
	oldCheckExecCommand := checkExecCommand
	t.Cleanup(func() { checkExecCommand = oldCheckExecCommand })
	checkExecCommand = func(name string, args ...string) *exec.Cmd {
		helper, helperArgs := testHelperCommand(t, "docs")
		return exec.Command(helper, helperArgs...)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "ok.leia")
	if err := os.WriteFile(path, []byte("print(\"ok\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(path, ".leia")+".out", []byte("ok\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runCheckCommand([]string{"--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runCheckCommand code = %d, stderr = %q", code, stderr.String())
	}
	var report checkReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON check report: %v; stdout = %q", err, stdout.String())
	}
	if !report.OK || len(report.Steps) != 7 {
		t.Fatalf("report = %+v, want seven passing steps", report)
	}
	for _, step := range report.Steps {
		if !step.OK || step.Skipped || step.ExitCode != 0 {
			t.Fatalf("step = %+v, want passing non-skipped step", step)
		}
	}
}

func TestCheckCommandReportsFailureAndSkips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.leia")
	if err := os.WriteFile(path, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runCheckCommand([]string{"--json", "--no-test", "--no-manifest", "--no-docs", "--no-editor", "--no-examples", dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runCheckCommand code = %d, want 1", code)
	}
	var report checkReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON check report: %v; stdout = %q", err, stdout.String())
	}
	if report.OK || len(report.Steps) != 7 {
		t.Fatalf("report = %+v, want failed report with seven steps", report)
	}
	if !report.Steps[2].Skipped || !report.Steps[2].OK {
		t.Fatalf("test step = %+v, want skipped ok", report.Steps[2])
	}
	if !report.Steps[3].Skipped || !report.Steps[3].OK {
		t.Fatalf("manifest step = %+v, want skipped ok", report.Steps[3])
	}
	if !report.Steps[4].Skipped || !report.Steps[4].OK {
		t.Fatalf("docs step = %+v, want skipped ok", report.Steps[4])
	}
	if !report.Steps[5].Skipped || !report.Steps[5].OK {
		t.Fatalf("editor step = %+v, want skipped ok", report.Steps[5])
	}
	if !report.Steps[6].Skipped || !report.Steps[6].OK {
		t.Fatalf("examples step = %+v, want skipped ok", report.Steps[6])
	}
}

func TestCheckCommandUsesSmokeSourceForRepositoryRootTooling(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	got := checkToolingPath(root)
	want := filepath.Join(root, "tests", "smoke", "01_basic.leia")
	if got != want {
		t.Fatalf("checkToolingPath(repo root) = %q, want %q", got, want)
	}

	dir := t.TempDir()
	if got := checkToolingPath(dir); got != dir {
		t.Fatalf("checkToolingPath(user dir) = %q, want %q", got, dir)
	}
}
