package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/evaluate"
)

func TestEvaluateCommandWritesJSONSkeleton(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.leia")
	src := `// TODO: add golden agent transcript
agent answer(question) {
    model: "mock"
    user: question
}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	var report evaluate.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != 1 || report.Phase != "syntax-static" {
		t.Fatalf("report = %#v", report)
	}
	if report.Summary.Agents != 1 || report.Summary.TODOs != 1 {
		t.Fatalf("summary = %#v", report.Summary)
	}
}

func TestEvaluateCommandTextFormat(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plain.md"), []byte("TODO: evaluate notes\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--format=text", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "evaluate: ok") || !strings.Contains(stdout.String(), "1 todos") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
