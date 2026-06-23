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

func TestTestCommandManifestCheckDispatchesTestsManifest(t *testing.T) {
	oldCheckExecCommand := checkExecCommand
	t.Cleanup(func() { checkExecCommand = oldCheckExecCommand })
	var gotName string
	var gotArgs []string
	checkExecCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "manifest")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--manifest-check"}, cliRunOptions{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q", code, stderr.String())
	}
	if gotName != "go" {
		t.Fatalf("manifest command = %q, want go", gotName)
	}
	if len(gotArgs) != 6 || gotArgs[0] != "run" || gotArgs[1] != "./cmd/leia" || gotArgs[2] != "run" || !strings.HasSuffix(gotArgs[3], filepath.Join("scripts", "manifest.leia")) || gotArgs[4] != "check" || gotArgs[5] != "tests" {
		t.Fatalf("args = %#v, want go run ./cmd/leia run scripts/manifest.leia check tests", gotArgs)
	}
	if !strings.Contains(stdout.String(), "manifest helper ok") {
		t.Fatalf("stdout = %q, want helper output", stdout.String())
	}
}

func TestRunTestCommandDefaultsToCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
	if err := os.WriteFile(filepath.Join(dir, "ok.leia"), []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=json"}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q", code, stderr.String())
	}
	var result testRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not JSON test result: %v; stdout = %q", err, stdout.String())
	}
	if result.SchemaVersion != 1 || !result.OK || result.Status != "pass" || result.Total != 1 || result.Passed != 1 {
		t.Fatalf("result = %+v, want one passing default-directory test", result)
	}
}

func TestRunTestsReportsFailingFile(t *testing.T) {
	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.leia")
	badPath := filepath.Join(dir, "bad.leia")
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
	path := filepath.Join(dir, "ok.leia")
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
	path := filepath.Join(dir, "bad.leia")
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
	okPath := filepath.Join(dir, "ok.leia")
	if err := os.WriteFile(okPath, []byte("print(\"ok\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(okPath, ".leia")+".out", []byte("ok\n"), 0644); err != nil {
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
	if result.SchemaVersion != 1 || !result.OK || result.Status != "pass" || result.Total != 1 || result.Passed != 1 || result.Failed != 0 || result.GoldenMode != "auto" {
		t.Fatalf("result = %+v, want one passing test", result)
	}
	if len(result.Files) != 1 || result.Files[0].File != okPath || !result.Files[0].OK {
		t.Fatalf("files = %+v, want passing %s", result.Files, okPath)
	}
}

func TestRunTestCommandWritesJSONReportToOutputFile(t *testing.T) {
	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.leia")
	if err := os.WriteFile(okPath, []byte("print(\"ok\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(okPath, ".leia")+".out", []byte("ok\n"), 0644); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(dir, "test-report.json")

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--json", "--output", reportPath, dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var result testRunResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("report is not JSON: %v; data = %q", err, string(data))
	}
	if result.SchemaVersion != 1 || !result.OK || result.Status != "pass" || result.Total != 1 || result.Passed != 1 || result.GoldenMode != "auto" {
		t.Fatalf("result = %+v, want one passing test", result)
	}
}

func TestRunTestCommandGoldenRequireReportsMissingGolden(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.leia")
	golden := strings.TrimSuffix(path, ".leia") + ".out"
	if err := os.WriteFile(path, []byte("print(\"ok\")\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=json", "--golden=require", dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
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
	if result.OK || result.GoldenMode != "require" || len(result.Files) != 1 || result.Files[0].Golden != golden || !strings.Contains(result.Files[0].Error, "missing golden") {
		t.Fatalf("result = %+v, want missing golden failure for %s", result, golden)
	}
}

func TestRunTestCommandWritesFailingJSONReportToOutputFile(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.leia")
	if err := os.WriteFile(badPath, []byte("print(\"actual\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(badPath, ".leia")+".out", []byte("expected\n"), 0644); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(dir, "test-report.json")

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=json", "--output", reportPath, dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runTestCommand code = %d, want 1; stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var result testRunResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("report is not JSON: %v; data = %q", err, string(data))
	}
	if result.SchemaVersion != 1 || result.OK || result.Status != "issues" || result.Total != 1 || result.Failed != 1 || len(result.Files) != 1 {
		t.Fatalf("result = %+v, want one failing test", result)
	}
}

func TestRunTestCommandGoldenIgnoreSkipsComparison(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.leia")
	if err := os.WriteFile(path, []byte("print(\"actual\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(path, ".leia")+".out", []byte("expected\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=json", "--golden=ignore", dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	var result testRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not JSON test result: %v; stdout = %q", err, stdout.String())
	}
	if !result.OK || result.GoldenMode != "ignore" || len(result.Files) != 1 || result.Files[0].Expected != "" || result.Files[0].Actual != "" {
		t.Fatalf("result = %+v, want ignored golden mismatch", result)
	}
}

func TestRunTestCommandSuppressesPassingScriptStdout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noisy.leia")
	if err := os.WriteFile(path, []byte("print(\"script-noise\")\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "script-noise") || strings.Contains(stderr.String(), "script-noise") {
		t.Fatalf("script stdout leaked: stdout = %q stderr = %q", stdout.String(), stderr.String())
	}
}

func TestRunTestCommandGoldenUpdateWritesGolden(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.leia")
	golden := strings.TrimSuffix(path, ".leia") + ".out"
	if err := os.WriteFile(path, []byte("print(\"new\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(golden, []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=json", "--golden=update", dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	got, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new\n" {
		t.Fatalf("golden = %q, want updated stdout", got)
	}
	var result testRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not JSON test result: %v; stdout = %q", err, stdout.String())
	}
	if !result.OK || result.GoldenMode != "update" || len(result.Files) != 1 || result.Files[0].Golden != golden {
		t.Fatalf("result = %+v, want update golden result", result)
	}
}

func TestRunTestCommandListsFiles(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.leia")
	b := filepath.Join(dir, "b.leia")
	if err := os.WriteFile(a, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("x := 2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--list", dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, a) || !strings.Contains(got, b) {
		t.Fatalf("stdout = %q, want listed files %q and %q", got, a, b)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunTestCommandListJSONReportsSchemaAndCounts(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a.leia")
	second := filepath.Join(dir, "b.leia")
	if err := os.WriteFile(first, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("x := 2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--list", "--json", "--golden=require", dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var report testListReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON test list report: %v; stdout = %q", err, stdout.String())
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || !report.ListOnly || report.GoldenMode != "require" || report.FileCount != 2 || len(report.Files) != 2 {
		t.Fatalf("report = %+v, want two-file schema v1 list report", report)
	}
	if report.Files[0] != first || report.Files[1] != second {
		t.Fatalf("files = %#v, want sorted files %#v", report.Files, []string{first, second})
	}
}

func TestRunTestCommandSeedIsVisibleToScripts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seed.leia")
	if err := os.WriteFile(path, []byte(`print(os.getenv("LEIA_TEST_SEED"))
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(path, ".leia")+".out", []byte("odin\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=json", "--seed", "odin", dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	var result testRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not JSON test result: %v; stdout = %q", err, stdout.String())
	}
	if result.Seed != "odin" || !result.OK {
		t.Fatalf("result = %+v, want seed and passing test", result)
	}
	if got := os.Getenv("LEIA_TEST_SEED"); got == "odin" {
		t.Fatal("LEIA_TEST_SEED leaked after test run")
	}
}

func TestRunTestCommandUsesConfiguredFormat(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "leia.toml"), []byte("[tool.test]\nformat = \"json\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ok.leia")
	if err := os.WriteFile(path, []byte("print(\"ok\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(path, ".leia")+".out", []byte("ok\n"), 0644); err != nil {
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
	badPath := filepath.Join(dir, "bad.leia")
	if err := os.WriteFile(badPath, []byte("print(\"actual\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(badPath, ".leia")+".out", []byte("expected\n"), 0644); err != nil {
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

func TestRunTestCommandRejectsUnsupportedFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=xml", "x.leia"}, cliRunOptions{}, &stdout, &stderr)
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
