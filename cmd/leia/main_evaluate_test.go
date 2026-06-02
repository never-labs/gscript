package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/tooling/evaluate"
	"github.com/never-labs/leia/llm"
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
	if report.Cases[0].DurationMS < 0 || len(report.Cases[0].Assertions) != 1 || report.Cases[0].Assertions[0].Status != "passed" {
		t.Fatalf("case metadata = %#v", report.Cases[0])
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
	if len(report.Cases[0].Assertions) != 1 || report.Cases[0].Assertions[0].Status != "failed" || len(report.Cases[0].Diagnostics) != 1 {
		t.Fatalf("case failure metadata = %#v", report.Cases[0])
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
	if !strings.Contains(stdout.String(), "evaluate: ok") || !strings.Contains(stdout.String(), "0 cases") || !strings.Contains(stdout.String(), "1 todos") || !strings.Contains(stdout.String(), "findings:") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestEvaluateCommandTextFormatListsCases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checks.leia")
	src := `evaluate "text report case" {
    assert(true)
}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--format=text", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	for _, want := range []string{"evaluate: ok", "PASS text report case", "1 assertions"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
}

func TestEvaluateCommandLLMReplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "llm_eval.leia")
	src := `evaluate "cli replay" {
    result, err := llm.turn({
        model: "mock-fast",
        messages: {llm.user("hello")},
    })
    assert(err == nil)
    assert(result.text == "from cli replay")
}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(dir, "records.json")
	if err := llm.SaveRecords(recordPath, []llm.Record{{
		Request: llm.TurnRequest{
			Model:    "mock-fast",
			Messages: []llm.Message{{Role: "user", Text: "hello"}},
		},
		Result: llm.TurnResult{Status: "final_answer", Text: "from cli replay"},
	}}); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--format=json", "--llm-replay", recordPath, path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	var report evaluate.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.Status != "ok" || len(report.Cases) != 1 || report.Cases[0].Status != "passed" {
		t.Fatalf("report = %#v", report)
	}
}
