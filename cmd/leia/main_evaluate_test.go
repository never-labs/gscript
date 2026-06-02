package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	if report.Summary.CasesSelected != 1 || report.Summary.CasesPassed != 1 || report.Summary.Assertions != 1 || report.Summary.PassRate != 1 {
		t.Fatalf("summary execution = %#v", report.Summary)
	}
	if len(report.Cases) != 1 || report.Cases[0].Name != "answer baseline" || report.Cases[0].Status != "passed" {
		t.Fatalf("cases = %#v", report.Cases)
	}
	if report.Cases[0].DurationMS < 0 || len(report.Cases[0].Assertions) != 1 || report.Cases[0].Assertions[0].Status != "passed" {
		t.Fatalf("case metadata = %#v", report.Cases[0])
	}
	if _, err := time.Parse(time.RFC3339, report.Cases[0].StartedAt); err != nil {
		t.Fatalf("case started_at = %q: %v", report.Cases[0].StartedAt, err)
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
	if !strings.Contains(stdout.String(), "evaluate: ok") || !strings.Contains(stdout.String(), "0 selected/0 discovered cases") || !strings.Contains(stdout.String(), "1 todos") || !strings.Contains(stdout.String(), "findings:") {
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
	for _, want := range []string{"evaluate: ok", "1 selected/1 discovered cases", "1.00 pass rate", "PASS text report case", "1 assertions"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
}

func TestEvaluateCommandFiltersCases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checks.leia")
	src := `evaluate "skip me" {
    assert(false, "should not run")
}

evaluate "run me" {
    assert(true)
}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--format=json", "--filter", "run me", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	var report evaluate.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.Summary.EvaluateBlocks != 2 || len(report.Cases) != 1 || report.Cases[0].Name != "run me" {
		t.Fatalf("report = %#v", report)
	}
}

func TestEvaluateCommandListsCasesWithoutExecuting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checks.leia")
	src := `evaluate "list only" {
    assert(false, "should not execute")
}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--format=json", "--list", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	var report evaluate.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.Status != "ok" || len(report.Cases) != 1 || report.Cases[0].Status != "listed" {
		t.Fatalf("report = %#v", report)
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
	if report.LLM == nil || report.LLM.Mode != "replay" || report.LLM.ReplayedTurns != 1 {
		t.Fatalf("llm report = %#v", report.LLM)
	}
}

func TestEvaluateCommandLLMReplayMismatchFailsReport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "llm_eval.leia")
	src := `evaluate "cli replay mismatch" {
    result, err := llm.turn({
        model: "mock-fast",
        messages: {llm.user("actual")},
    })
    assert(result == nil)
    assert(err.message == "llm replay request mismatch at turn 0")
}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(dir, "records.json")
	if err := llm.SaveRecords(recordPath, []llm.Record{{
		Request: llm.TurnRequest{
			Model:    "mock-fast",
			Messages: []llm.Message{{Role: "user", Text: "expected"}},
		},
		Result: llm.TurnResult{Status: "final_answer", Text: "unused"},
	}}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--format=json", "--llm-replay", recordPath, path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	var report evaluate.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.Status != "failed" || len(report.Cases) != 1 || report.Cases[0].Status != "passed" {
		t.Fatalf("report = %#v", report)
	}
	if len(report.Findings) != 1 || report.Findings[0].Kind != "llm_replay_mismatch" {
		t.Fatalf("findings = %#v", report.Findings)
	}
	if report.Findings[0].Details["expected"] == nil || report.Findings[0].Details["actual"] == nil {
		t.Fatalf("finding details = %#v", report.Findings[0].Details)
	}
	expected, ok := report.Findings[0].Details["expected"].(map[string]any)
	if !ok || expected["Model"] != nil || expected["model"] != "mock-fast" {
		t.Fatalf("expected details = %#v", report.Findings[0].Details["expected"])
	}
	actual, ok := report.Findings[0].Details["actual"].(map[string]any)
	if !ok || actual["Model"] != nil || actual["model"] != "mock-fast" {
		t.Fatalf("actual details = %#v", report.Findings[0].Details["actual"])
	}
}

func TestEvaluateCommandAgentReplayExample(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{
		"--format=json",
		"--llm-replay", "../../examples/evaluate/agent_replay.records.json",
		"../../examples/evaluate/agent_replay.leia",
	}, &stdout, &stderr)
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
	if report.LLM == nil || report.LLM.Mode != "replay" || report.LLM.ReplayedTurns != 1 {
		t.Fatalf("llm report = %#v", report.LLM)
	}
}

func TestEvaluateCommandMultiturnReplayExample(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{
		"--format=json",
		"--llm-replay", "../../examples/evaluate/multiturn_replay.records.json",
		"../../examples/evaluate/multiturn_replay.leia",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	var report evaluate.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.Status != "ok" || len(report.Cases) != 1 || report.Cases[0].Name != "multiturn history replay" || report.Cases[0].Status != "passed" {
		t.Fatalf("report = %#v", report)
	}
	if report.LLM == nil || report.LLM.Mode != "replay" || report.LLM.ReplayedTurns != 2 || report.LLM.RemainingTurns != 0 {
		t.Fatalf("llm report = %#v", report.LLM)
	}
}

func TestEvaluateCommandUpdateGoldenWritesFixture(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "basic.leia")
	src := `evaluate "cli update golden" {
    assert(true)
}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(dir, "golden.json")
	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--format=json", "--update-golden", recordPath, path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	var report evaluate.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.LLM == nil || report.LLM.Mode != "update_golden" || !report.LLM.GoldenUpdated || report.LLM.RecordedTurns != 0 {
		t.Fatalf("llm report = %#v", report.LLM)
	}
	records, err := llm.LoadRecords(recordPath)
	if err != nil {
		t.Fatalf("load golden: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("records = %#v, want empty fixture", records)
	}
}
