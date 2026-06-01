package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/tooling/evaluate"
)

func TestEvaluateCommandWritesJSONSkeleton(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.leia")
	src := `// TODO: add golden agent transcript
func answer(question) {
    return "ok:" .. question
}

evaluate "answer baseline" {
    result := answer("hello")
    assert(result == "ok:hello")
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
	if report.SchemaVersion != 1 || report.Phase != "runtime-minimal" {
		t.Fatalf("report = %#v", report)
	}
	if report.Summary.Agents != 0 || report.Summary.EvaluateBlocks != 1 || report.Summary.TODOs != 1 {
		t.Fatalf("summary = %#v", report.Summary)
	}
	if len(report.Cases) != 1 || report.Cases[0].Name != "answer baseline" || report.Cases[0].Status != "passed" {
		t.Fatalf("cases = %#v", report.Cases)
	}
}

func TestEvaluateCommandJSONFailureStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fail.leia")
	src := `evaluate "failing assertion" {
    assert(false, "boom")
}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--json", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	var report evaluate.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.Status != "failed" || len(report.Cases) != 1 || report.Cases[0].Status != "failed" {
		t.Fatalf("report = %#v", report)
	}
	if len(report.Findings) != 1 || report.Findings[0].Kind != "case_runtime_error" {
		t.Fatalf("findings = %#v", report.Findings)
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
	if !strings.Contains(stdout.String(), "evaluate: ok") || !strings.Contains(stdout.String(), "0 cases") || !strings.Contains(stdout.String(), "1 todos") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
