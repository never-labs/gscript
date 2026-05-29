package main

import (
	"bytes"
	"encoding/json"
	"os"
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
	if !containsString(caps.Commands, "capabilities") || !containsString(caps.Commands, "fmt") || !containsString(caps.Commands, "lint") {
		t.Fatalf("commands = %#v, want capabilities/fmt/lint", caps.Commands)
	}
	if !containsString(caps.Tooling.Linter.Formats, "json") || !containsString(caps.Tooling.Linter.Formats, "sarif") || !containsString(caps.Tooling.Linter.Codes, "GS1001") {
		t.Fatalf("linter capabilities = %+v, want json and GS1001", caps.Tooling.Linter)
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
