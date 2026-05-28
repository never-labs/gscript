package main

import (
	"bytes"
	"os"
	"path/filepath"
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
