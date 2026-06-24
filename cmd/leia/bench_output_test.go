package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestBenchParseTimeReadsFlatScriptTime(t *testing.T) {
	if got := benchParseTime("result\nTime: 0.123s\n"); got == nil || *got != 0.123 {
		t.Fatalf("benchParseTime = %v, want 0.123", got)
	}
	if got := benchParseTime("inner: 0.123s\n"); got != nil {
		t.Fatalf("benchParseTime inner = %v, want nil", *got)
	}
}

func TestBenchParseCounterDefaultsToZero(t *testing.T) {
	pattern := regexp.MustCompile(`(?m)^count:\s*([0-9]+)$`)
	if got := benchParseCounter(pattern, "count: 42\n"); got != 42 {
		t.Fatalf("benchParseCounter = %d, want 42", got)
	}
	if got := benchParseCounter(pattern, "missing\n"); got != 0 {
		t.Fatalf("benchParseCounter missing = %d, want 0", got)
	}
}

func TestBenchOutputTailKeepsNonemptyLines(t *testing.T) {
	if got := benchOutputTail("\na\n\nb\nc\n", 2); got != "b\nc" {
		t.Fatalf("benchOutputTail = %q, want b\\nc", got)
	}
}

func TestBenchTextOutputNormalizesSubprocessPayloads(t *testing.T) {
	if got := benchTextOutput(nil); got != "" {
		t.Fatalf("benchTextOutput nil = %q, want empty", got)
	}
	if got := benchTextOutput([]byte("ok")); got != "ok" {
		t.Fatalf("benchTextOutput text = %q, want ok", got)
	}
	if got := benchTextOutput([]byte{'b', 'a', 'd', ':', 0xff}); got != "bad:\uFFFD" {
		t.Fatalf("benchTextOutput invalid = %q, want replacement char", got)
	}
}

func TestBenchMarkdownRowFormatsCellsWithoutChangingPayloads(t *testing.T) {
	if got := benchMarkdownRow("a/b", 3, "x | y"); got != "| a/b | 3 | x | y |" {
		t.Fatalf("benchMarkdownRow = %q", got)
	}
}

func TestBenchMarkdownSectionFormatsPlainSection(t *testing.T) {
	got := benchMarkdownSection("Diagnostics", "", "")
	want := []string{"", "## Diagnostics", ""}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("benchMarkdownSection = %#v, want %#v", got, want)
	}
}

func TestBenchMarkdownSectionFormatsTableHeader(t *testing.T) {
	got := benchMarkdownSection("Measurements", "| A | B |", "|---|---:|")
	want := []string{"", "## Measurements", "", "| A | B |", "|---|---:|"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("benchMarkdownSection = %#v, want %#v", got, want)
	}
}

func TestBenchRunTextCommandReportsSuccess(t *testing.T) {
	result := benchRunTextCommand([]string{os.Args[0], "-test.run=TestBenchRunTextCommandHelper", "--", "ok"}, 5*time.Second, nil)
	if result.Status != "ok" || result.ExitCode == nil || *result.ExitCode != 0 || result.Output != "ok\n" || result.WallSeconds < 0 {
		t.Fatalf("result = %#v, want ok output", result)
	}
}

func TestBenchRunTextCommandReportsError(t *testing.T) {
	result := benchRunTextCommand([]string{os.Args[0], "-test.run=TestBenchRunTextCommandHelper", "--", "error"}, 5*time.Second, nil)
	if result.Status != "error" || result.ExitCode == nil || *result.ExitCode != 7 || result.Output != "bad\n" {
		t.Fatalf("result = %#v, want error exit 7", result)
	}
}

func TestBenchRunTextCommandReportsTimeout(t *testing.T) {
	result := benchRunTextCommand([]string{os.Args[0], "-test.run=TestBenchRunTextCommandHelper", "--", "timeout"}, 50*time.Millisecond, nil)
	if result.Status != "timeout" || result.ExitCode != nil || !strings.Contains(result.Output, "TIMEOUT after 0.05s") || result.WallSeconds < 0 {
		t.Fatalf("result = %#v, want timeout", result)
	}
}

func TestBenchRunTextCommandHelper(t *testing.T) {
	if len(os.Args) < 2 || os.Args[len(os.Args)-2] != "--" {
		return
	}
	switch os.Args[len(os.Args)-1] {
	case "ok":
		_, _ = os.Stdout.WriteString("ok\n")
		os.Exit(0)
	case "error":
		_, _ = os.Stdout.WriteString("bad\n")
		os.Exit(7)
	case "timeout":
		_, _ = os.Stdout.WriteString("start\n")
		time.Sleep(time.Second)
		os.Exit(0)
	}
}

func TestBenchBuildLeiaUsesStandardGoCommand(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	root := t.TempDir()
	var gotName string
	var gotArgs []string
	var gotCmd *exec.Cmd
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "bench")
		cmd := exec.Command(helper, helperArgs...)
		gotCmd = cmd
		return cmd
	}

	if err := benchBuildLeia(root, "/tmp/leia", "", nil); err != nil {
		t.Fatalf("benchBuildLeia error = %v", err)
	}
	if gotName != "go" || strings.Join(gotArgs, "\x00") != strings.Join([]string{"build", "-o", "/tmp/leia", "./cmd/leia"}, "\x00") {
		t.Fatalf("command = %s %#v, want go build -o /tmp/leia ./cmd/leia", gotName, gotArgs)
	}
	if gotCmd == nil {
		t.Fatal("benchExecCommand was not called")
	}
	if gotCmd.Dir != root {
		t.Fatalf("cmd.Dir = %q, want %q", gotCmd.Dir, root)
	}
}

func TestBenchBuildLeiaKeepsCustomFailureMessage(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	root := t.TempDir()
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=TestBenchRunTextCommandHelper", "--", "error")
		return cmd
	}
	var stderr bytes.Buffer
	err := benchBuildLeia(root, "/tmp/leia", "build failed in {root} with exit {exit_code}", &stderr)
	if err == nil || err.Error() != "build failed in "+root+" with exit 7" {
		t.Fatalf("benchBuildLeia error = %v", err)
	}
	if stderr.String() != "bad\n" {
		t.Fatalf("stderr = %q, want compiler output", stderr.String())
	}
}

func TestBenchLeiaModeCommandFormatsModes(t *testing.T) {
	oldEnv, hadEnv := os.LookupEnv("LEIA_TIER2_NO_FILTER")
	if err := os.Unsetenv("LEIA_TIER2_NO_FILTER"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadEnv {
			_ = os.Setenv("LEIA_TIER2_NO_FILTER", oldEnv)
		} else {
			_ = os.Unsetenv("LEIA_TIER2_NO_FILTER")
		}
	})
	tests := []struct {
		mode string
		want []string
		env  string
	}{
		{"vm", []string{"/bin/leia", "-vm", "/bench/main.leia"}, ""},
		{"default", []string{"/bin/leia", "-jit", "-jit-stats", "-exit-stats", "/bench/main.leia"}, ""},
		{"no_filter", []string{"/bin/leia", "-jit", "-jit-stats", "-exit-stats", "/bench/main.leia"}, "LEIA_TIER2_NO_FILTER=1"},
	}
	for _, tt := range tests {
		cmd, env, err := benchLeiaModeCommand(tt.mode, "/bin/leia", "/bench/main.leia")
		if err != nil {
			t.Fatalf("benchLeiaModeCommand(%q) error = %v", tt.mode, err)
		}
		if strings.Join(cmd, "\x00") != strings.Join(tt.want, "\x00") {
			t.Fatalf("cmd = %#v, want %#v", cmd, tt.want)
		}
		if tt.env != "" && !containsString(env, tt.env) {
			t.Fatalf("env missing %q: %#v", tt.env, env)
		}
		if tt.env == "" && containsString(env, "LEIA_TIER2_NO_FILTER=1") {
			t.Fatalf("env unexpectedly contains LEIA_TIER2_NO_FILTER=1")
		}
	}
}

func TestBenchLeiaModeCommandRejectsUnknownMode(t *testing.T) {
	_, _, err := benchLeiaModeCommand("bad", "/bin/leia", "/bench/main.leia")
	if err == nil || err.Error() != "unknown mode: bad" {
		t.Fatalf("err = %v, want unknown mode", err)
	}
}

func TestBenchBenchmarkModeCommandFormatsLuaJITMode(t *testing.T) {
	lua := filepath.Join(t.TempDir(), "main.lua")
	if err := os.WriteFile(lua, []byte("print('ok')\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := benchBenchmarkModeCommand("luajit", "/bin/leia", "/bench/main.leia", "/bin/luajit", lua)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Args, "\x00") != strings.Join([]string{"/bin/luajit", lua}, "\x00") || got.Env != nil || got.Unavailable != "" {
		t.Fatalf("command = %#v", got)
	}
}

func TestBenchBenchmarkModeCommandReportsUnavailableModes(t *testing.T) {
	got, err := benchBenchmarkModeCommand("luajit", "/bin/leia", "/bench/main.leia", "/bin/luajit", filepath.Join(t.TempDir(), "missing.lua"))
	if err != nil || got.Unavailable != "missing" || got.Args != nil || got.Env != nil {
		t.Fatalf("missing luajit = %#v, err = %v", got, err)
	}
	got, err = benchBenchmarkModeCommand("luajit", "/bin/leia", "/bench/main.leia", "", "/bench/main.lua")
	if err != nil || got.Unavailable != "skipped" || got.Args != nil || got.Env != nil {
		t.Fatalf("skipped luajit = %#v, err = %v", got, err)
	}
	got, err = benchBenchmarkModeCommand("default", "/bin/leia", filepath.Join(t.TempDir(), "missing.leia"), "", "")
	if err != nil || got.Unavailable != "missing" || got.Args != nil || got.Env != nil {
		t.Fatalf("missing leia = %#v, err = %v", got, err)
	}
}

func TestBenchBenchmarkModeCommandFormatsLeiaMode(t *testing.T) {
	script := filepath.Join(t.TempDir(), "main.leia")
	if err := os.WriteFile(script, []byte("print('ok')\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := benchBenchmarkModeCommand("default", "/bin/leia", script, "", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/bin/leia", "-jit", "-jit-stats", "-exit-stats", script}
	if strings.Join(got.Args, "\x00") != strings.Join(want, "\x00") || got.Env == nil || got.Unavailable != "" {
		t.Fatalf("command = %#v, want leia mode", got)
	}
}

func TestBenchBuildLeiaUsesCustomFailureForStartError(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command(filepath.Join(t.TempDir(), "missing-go"))
	}
	err := benchBuildLeia("/repo", "/tmp/leia", "build failed in {root} with exit {exit_code}", nil)
	if err == nil || !errors.Is(err, exec.ErrNotFound) && err.Error() != "build failed in /repo with exit 1" {
		t.Fatalf("err = %v, want formatted build failure", err)
	}
}
