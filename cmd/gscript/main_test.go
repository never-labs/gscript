package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
)

func TestTestFilesSingleFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(path, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := testFiles(path)
	if err != nil {
		t.Fatalf("testFiles err = %v", err)
	}
	if len(files) != 1 || files[0] != path {
		t.Fatalf("testFiles = %#v, want [%q]", files, path)
	}
}

func TestCapabilitiesJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCapabilitiesCommand([]string{"--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runCapabilitiesCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var caps cliCapabilities
	if err := json.Unmarshal(stdout.Bytes(), &caps); err != nil {
		t.Fatalf("stdout is not JSON capabilities: %v; stdout = %q", err, stdout.String())
	}
	if caps.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", caps.SchemaVersion)
	}
	if caps.Platform.GOOS != goruntime.GOOS || caps.Platform.GOARCH != goruntime.GOARCH {
		t.Fatalf("platform = %s/%s, want %s/%s", caps.Platform.GOOS, caps.Platform.GOARCH, goruntime.GOOS, goruntime.GOARCH)
	}
	if !caps.Execution.Interpreter || !caps.Execution.BytecodeVM {
		t.Fatalf("execution capabilities = %+v, want interpreter and bytecode VM", caps.Execution)
	}
	if len(caps.StdlibModules) == 0 {
		t.Fatal("stdlib_modules is empty")
	}
	if !containsString(caps.Commands, "bench") || !containsString(caps.Commands, "capabilities") || !containsString(caps.Commands, "check") || !containsString(caps.Commands, "config") || !containsString(caps.Commands, "fmt") || !containsString(caps.Commands, "lint") {
		t.Fatalf("commands = %#v, want bench/capabilities/check/config/fmt/lint", caps.Commands)
	}
	if !containsString(caps.Tooling.Linter.Formats, "json") || !containsString(caps.Tooling.Linter.Formats, "sarif") || !containsString(caps.Tooling.Linter.Codes, "GS1001") {
		t.Fatalf("linter capabilities = %+v, want json and GS1001", caps.Tooling.Linter)
	}
	if caps.Tooling.Config.FileName != "gscript.toml" || !containsString(caps.Tooling.Config.Formats, "json") {
		t.Fatalf("config capabilities = %+v, want gscript.toml/json", caps.Tooling.Config)
	}
}

func TestCapabilitiesRejectsExtraArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCapabilitiesCommand([]string{"extra"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runCapabilitiesCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: gscript capabilities") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GSCRIPT_TEST_HELPER") == "" {
		return
	}
	switch os.Getenv("GSCRIPT_TEST_HELPER") {
	case "bench":
		_, _ = os.Stdout.WriteString("bench helper ok\n")
		os.Exit(0)
	case "docs":
		_, _ = os.Stdout.WriteString("docs helper ok\n")
		os.Exit(0)
	default:
		os.Exit(2)
	}
}

func testHelperCommand(t *testing.T, helper string) (string, []string) {
	t.Helper()
	args := []string{"-test.run=TestHelperProcess", "--"}
	t.Setenv("GSCRIPT_TEST_HELPER", helper)
	return os.Args[0], args
}

func TestConfigCommandJSONDiscoversAndParsesProjectConfig(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "cmd", "tool")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	config := `[project]
name = "demo"
version = "0.1.0"

[tool.fmt]
indent_width = 4
line_width = 100

[tool.lint]
format = "sarif"

[tool.test]
format = "json"
`
	if err := os.WriteFile(filepath.Join(root, "gscript.toml"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runConfigCommand([]string{"--json", nested}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runConfigCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var report cliConfigReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON config report: %v; stdout = %q", err, stdout.String())
	}
	if !report.Found || report.Root != root || report.Path != filepath.Join(root, "gscript.toml") {
		t.Fatalf("report location = %+v, want discovered root %s", report, root)
	}
	if report.Config == nil {
		t.Fatal("config is nil, want parsed config")
	}
	if report.Config.Project.Name != "demo" || report.Config.Project.Version != "0.1.0" {
		t.Fatalf("project = %+v, want demo 0.1.0", report.Config.Project)
	}
	if report.Config.Tool.Format.IndentWidth != 4 || report.Config.Tool.Format.LineWidth != 100 {
		t.Fatalf("fmt config = %+v, want 4/100", report.Config.Tool.Format)
	}
	if report.Config.Tool.Lint.Format != "sarif" || report.Config.Tool.Test.Format != "json" {
		t.Fatalf("tool config = %+v, want sarif/json", report.Config.Tool)
	}
}

func TestConfigCommandReportsMissingConfig(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runConfigCommand([]string{"--json", dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runConfigCommand code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty for JSON mode", stderr.String())
	}
	var report cliConfigReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON config report: %v; stdout = %q", err, stdout.String())
	}
	if report.Found || len(report.Diagnostics) != 1 || report.Diagnostics[0].Code != "GS9001" {
		t.Fatalf("report = %+v, want not found diagnostic", report)
	}
}

func TestConfigCommandReportsParseErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gscript.toml"), []byte("[tool.test]\nformat = \"xml\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runConfigCommand([]string{dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runConfigCommand code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "GS9002") || !strings.Contains(stderr.String(), "tool.test.format") {
		t.Fatalf("stderr = %q, want config parse diagnostic", stderr.String())
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
	path := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(path, []byte("print(\"ok\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(path, ".gs")+".out", []byte("ok\n"), 0644); err != nil {
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
	if !report.OK || len(report.Steps) != 4 {
		t.Fatalf("report = %+v, want four passing steps", report)
	}
	for _, step := range report.Steps {
		if !step.OK || step.Skipped || step.ExitCode != 0 {
			t.Fatalf("step = %+v, want passing non-skipped step", step)
		}
	}
}

func TestCheckCommandReportsFailureAndSkips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(path, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runCheckCommand([]string{"--json", "--no-test", "--no-docs", dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runCheckCommand code = %d, want 1", code)
	}
	var report checkReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON check report: %v; stdout = %q", err, stdout.String())
	}
	if report.OK || len(report.Steps) != 4 {
		t.Fatalf("report = %+v, want failed report with four steps", report)
	}
	if !report.Steps[2].Skipped || !report.Steps[2].OK {
		t.Fatalf("test step = %+v, want skipped ok", report.Steps[2])
	}
	if !report.Steps[3].Skipped || !report.Steps[3].OK {
		t.Fatalf("docs step = %+v, want skipped ok", report.Steps[3])
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
	code := runBenchCommand([]string{"compare", "--bench", "suite/mandelbrot"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if gotName != "python3" {
		t.Fatalf("python command = %q, want python3", gotName)
	}
	if len(gotArgs) != 3 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "timing_compare.py")) || gotArgs[1] != "--bench" || gotArgs[2] != "suite/mandelbrot" {
		t.Fatalf("args = %#v, want timing_compare.py --bench suite/mandelbrot", gotArgs)
	}
	if !strings.Contains(stdout.String(), "bench helper ok") {
		t.Fatalf("stdout = %q, want helper output", stdout.String())
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
	code := runBenchCommand([]string{"strict", "--group", "suite"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) != 3 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "strict_guard.py")) || gotArgs[1] != "--group" || gotArgs[2] != "suite" {
		t.Fatalf("args = %#v, want strict_guard.py --group suite", gotArgs)
	}
}

func TestTestFilesDirectoryCollectsGSFilesSorted(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		filepath.Join(dir, "b.gs"),
		filepath.Join(dir, "a.gs"),
		filepath.Join(dir, "nested", "c.gs"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x := 1\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := testFiles(dir)
	if err != nil {
		t.Fatalf("testFiles err = %v", err)
	}
	want := []string{paths[1], paths[0], paths[2]}
	if len(files) != len(want) {
		t.Fatalf("testFiles len = %d, want %d: %#v", len(files), len(want), files)
	}
	for i := range want {
		if files[i] != want[i] {
			t.Fatalf("testFiles = %#v, want %#v", files, want)
		}
	}
}

func TestRunTestsReportsFailingFile(t *testing.T) {
	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.gs")
	badPath := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(okPath, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badPath, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	if runTests(dir, cliRunOptions{UseVM: false}, &stderr) {
		t.Fatal("runTests succeeded, want failure")
	}
	out := stderr.String()
	if !strings.Contains(out, badPath) {
		t.Fatalf("stderr = %q, want failing filename %q", out, badPath)
	}
	if !strings.Contains(out, "parse error") {
		t.Fatalf("stderr = %q, want parse error", out)
	}
}

func TestRunTestsComparesGoldenStdout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(path, []byte("print(\"hello\", \"world\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ok.out"), []byte("hello\tworld\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	if !runTests(path, cliRunOptions{UseVM: false}, &stderr) {
		t.Fatalf("runTests failed, stderr = %q", stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunTestsReportsGoldenStdoutMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.gs")
	golden := filepath.Join(dir, "bad.out")
	if err := os.WriteFile(path, []byte("print(\"actual\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(golden, []byte("expected\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	if runTests(path, cliRunOptions{UseVM: false}, &stderr) {
		t.Fatal("runTests succeeded, want failure")
	}
	out := stderr.String()
	for _, want := range []string{path, golden, "stdout mismatch", "expected:\nexpected\n", "got:\nactual\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stderr = %q, want %q", out, want)
		}
	}
}

func TestRunTestCommandJSONReportsResults(t *testing.T) {
	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(okPath, []byte("print(\"ok\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(okPath, ".gs")+".out", []byte("ok\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=json", dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var result testRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not JSON test result: %v; stdout = %q", err, stdout.String())
	}
	if !result.OK || result.Total != 1 || result.Passed != 1 || result.Failed != 0 {
		t.Fatalf("result = %+v, want one passing test", result)
	}
	if len(result.Files) != 1 || result.Files[0].File != okPath || !result.Files[0].OK {
		t.Fatalf("files = %+v, want passing %s", result.Files, okPath)
	}
}

func TestRunTestCommandUsesConfiguredFormat(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gscript.toml"), []byte("[tool.test]\nformat = \"json\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(path, []byte("print(\"ok\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(path, ".gs")+".out", []byte("ok\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q", code, stderr.String())
	}
	var result testRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not configured JSON test result: %v; stdout = %q", err, stdout.String())
	}
	if !result.OK || result.Total != 1 {
		t.Fatalf("result = %+v, want one passing configured JSON test", result)
	}
}

func TestRunTestCommandJSONReportsFailures(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(badPath, []byte("print(\"actual\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(badPath, ".gs")+".out", []byte("expected\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=json", dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runTestCommand code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var result testRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not JSON test result: %v; stdout = %q", err, stdout.String())
	}
	if result.OK || result.Total != 1 || result.Passed != 0 || result.Failed != 1 {
		t.Fatalf("result = %+v, want one failing test", result)
	}
	if len(result.Files) != 1 || result.Files[0].Error != "" || result.Files[0].Expected != "expected\n" || result.Files[0].Actual != "actual\n" {
		t.Fatalf("file result = %+v, want stdout mismatch payload", result.Files)
	}
}

func TestLintCommandUsesConfiguredFormat(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gscript.toml"), []byte("[tool.lint]\nformat = \"json\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(path, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runLintCommand code = %d, want diagnostics failure", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty in configured JSON mode", stderr.String())
	}
	var diagnostics []lintDiagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostics); err != nil {
		t.Fatalf("stdout is not configured JSON diagnostics: %v; stdout = %q", err, stdout.String())
	}
	if len(diagnostics) != 1 || diagnostics[0].File != path || diagnostics[0].Code != "GS1001" {
		t.Fatalf("diagnostics = %+v, want configured JSON parse diagnostic for %s", diagnostics, path)
	}
}

func TestRunTestCommandRejectsUnsupportedFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=xml", "x.gs"}, cliRunOptions{}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runTestCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unsupported --format") {
		t.Fatalf("stderr = %q, want unsupported format", stderr.String())
	}
}

func TestFmtCheckReportsUnformattedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "needs_fmt.gs")
	original := []byte("x := 1 \t\r\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--check", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runFmtCommand code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), path+": not formatted") {
		t.Fatalf("stderr = %q, want not formatted diagnostic", stderr.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("file changed during --check: %q", string(got))
	}
}

func TestFmtWritesWhitespaceNormalization(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "needs_fmt.gs")
	if err := os.WriteFile(path, []byte("x := 1  \n\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runFmtCommand code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), path) {
		t.Fatalf("stdout = %q, want formatted filename", stdout.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "x := 1\n" {
		t.Fatalf("formatted file = %q, want %q", string(got), "x := 1\n")
	}
}

func TestFmtStdinWritesFormattedSourceToStdout(t *testing.T) {
	oldStdin := cliStdin
	cliStdin = strings.NewReader("x := 1  \r\n\r\n")
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--stdin-file-name", "scratch.gs"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runFmtCommand code = %d, stderr = %q", code, stderr.String())
	}
	if got, want := stdout.String(), "x := 1\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestFmtStdinCheckReportsUnformattedSource(t *testing.T) {
	oldStdin := cliStdin
	cliStdin = strings.NewReader("x := 1 \t\n")
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--check", "--stdin-file-name", "scratch.gs"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runFmtCommand code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "scratch.gs: not formatted") {
		t.Fatalf("stderr = %q, want not formatted diagnostic", stderr.String())
	}
}

func TestFmtStdinCheckAcceptsFormattedSource(t *testing.T) {
	oldStdin := cliStdin
	cliStdin = strings.NewReader("x := 1\n")
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--check", "--stdin-file-name", "scratch.gs"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runFmtCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestFmtStdinRejectsPathArguments(t *testing.T) {
	oldStdin := cliStdin
	cliStdin = strings.NewReader("x := 1\n")
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--stdin-file-name", "scratch.gs", "file.gs"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runFmtCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--stdin-file-name cannot be used with path arguments") {
		t.Fatalf("stderr = %q, want stdin/path diagnostic", stderr.String())
	}
}

func TestFmtRefusesSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.gs")
	original := []byte("func {\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runFmtCommand code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "parse error") {
		t.Fatalf("stderr = %q, want parse error", stderr.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("file changed after parse failure: %q", string(got))
	}
}

func TestLintReportsSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.gs")
	badPath := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(okPath, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badPath, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runLintCommand code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	out := stderr.String()
	for _, want := range []string{badPath, "GS1001", "parse error"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stderr = %q, want %q", out, want)
		}
	}
	if strings.Contains(out, okPath) {
		t.Fatalf("stderr = %q, did not want clean file", out)
	}
}

func TestLintJSONReportsSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.gs")
	badPath := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(okPath, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badPath, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format=json", dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runLintCommand code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var diagnostics []lintDiagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostics); err != nil {
		t.Fatalf("stdout is not JSON diagnostics: %v; stdout = %q", err, stdout.String())
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics len = %d, want 1: %#v", len(diagnostics), diagnostics)
	}
	got := diagnostics[0]
	if got.File != badPath {
		t.Fatalf("diagnostic file = %q, want %q", got.File, badPath)
	}
	if got.Code != "GS1001" {
		t.Fatalf("diagnostic code = %q, want GS1001", got.Code)
	}
	if got.Severity != "error" {
		t.Fatalf("diagnostic severity = %q, want error", got.Severity)
	}
	if !strings.Contains(got.Message, "parse error") {
		t.Fatalf("diagnostic message = %q, want parse error", got.Message)
	}
	if got.Line != 1 {
		t.Fatalf("diagnostic line = %d, want 1", got.Line)
	}
	if got.Column != 6 {
		t.Fatalf("diagnostic column = %d, want 6", got.Column)
	}
}

func TestLintJSONReportsLexerErrorPosition(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(badPath, []byte("\"unterminated\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format=json", badPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runLintCommand code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var diagnostics []lintDiagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostics); err != nil {
		t.Fatalf("stdout is not JSON diagnostics: %v; stdout = %q", err, stdout.String())
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics len = %d, want 1: %#v", len(diagnostics), diagnostics)
	}
	got := diagnostics[0]
	if got.File != badPath {
		t.Fatalf("diagnostic file = %q, want %q", got.File, badPath)
	}
	if got.Code != "GS1001" {
		t.Fatalf("diagnostic code = %q, want GS1001", got.Code)
	}
	if !strings.Contains(got.Message, "lexer error") {
		t.Fatalf("diagnostic message = %q, want lexer error", got.Message)
	}
	if got.Line != 1 {
		t.Fatalf("diagnostic line = %d, want 1", got.Line)
	}
	if got.Column != 1 {
		t.Fatalf("diagnostic column = %d, want 1", got.Column)
	}
}

func TestLintJSONReportsEmptyDiagnosticsOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(path, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format", "json", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runLintCommand code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var diagnostics []lintDiagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostics); err != nil {
		t.Fatalf("stdout is not JSON diagnostics: %v; stdout = %q", err, stdout.String())
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want empty", diagnostics)
	}
}

func TestLintSARIFReportsSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(badPath, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format=sarif", badPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runLintCommand code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var log sarifLog
	if err := json.Unmarshal(stdout.Bytes(), &log); err != nil {
		t.Fatalf("stdout is not SARIF JSON: %v; stdout = %q", err, stdout.String())
	}
	if log.Version != "2.1.0" || len(log.Runs) != 1 {
		t.Fatalf("SARIF version/runs = %q/%d, want 2.1.0/1", log.Version, len(log.Runs))
	}
	if log.Runs[0].Tool.Driver.Name != "gscript lint" {
		t.Fatalf("tool name = %q, want gscript lint", log.Runs[0].Tool.Driver.Name)
	}
	if len(log.Runs[0].Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(log.Runs[0].Results))
	}
	result := log.Runs[0].Results[0]
	if result.RuleID != "GS1001" || result.Level != "error" {
		t.Fatalf("result = %+v, want GS1001 error", result)
	}
	if !strings.Contains(result.Message.Text, "parse error") {
		t.Fatalf("message = %q, want parse error", result.Message.Text)
	}
	if len(result.Locations) != 1 || result.Locations[0].PhysicalLocation.ArtifactLocation.URI != filepath.ToSlash(badPath) {
		t.Fatalf("locations = %+v, want %s", result.Locations, filepath.ToSlash(badPath))
	}
}

func TestLintRejectsUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(path, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format=xml", path}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runLintCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unsupported --format") {
		t.Fatalf("stderr = %q, want unsupported format diagnostic", stderr.String())
	}
}
