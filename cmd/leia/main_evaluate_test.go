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

func TestEvaluateCommandHelpExplainsReplayModes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runEvaluateCommand code = %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{
		"usage: leia evaluate [options] [path-or-dir...]",
		"Run source-level evaluate blocks",
		"Examples:",
		"--report eval-report.json",
		"--format=html --report eval-report.html",
		"--replay examples/evaluate/agent_replay.records.json",
		"LLM fixture modes are mutually exclusive:",
		"--record",
		"--llm-record",
		"--update-golden",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
}

func TestEvaluateCommandWritesReportToOutputFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checks.leia")
	src := `evaluate "file report" {
    assert(true)
}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(dir, "eval-report.json")
	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--format=json", "--output", reportPath, path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var report evaluate.Report
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, string(data))
	}
	if report.Status != "ok" || len(report.Cases) != 1 || report.Cases[0].Status != "passed" {
		t.Fatalf("report = %#v", report)
	}
}

func TestEvaluateCommandWritesHTMLReportAlias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checks.leia")
	src := `evaluate "html report" {
    eval.metric("correct", true)
    eval.case("row_1", func() {
        eval.metric("score", 0.75)
        assert(true)
    })
}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(dir, "eval-report.html")
	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--format=html", "--report", reportPath, path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	for _, want := range []string{"<!doctype html>", "Leia Evaluate Report", "html report", "row_1", "correct", "score", "mean 0.75"} {
		if !strings.Contains(html, want) {
			t.Fatalf("html report missing %q:\n%s", want, html)
		}
	}
}

func TestEvaluateCommandRejectsConflictingOutputAliases(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--output", "a.json", "--report", "b.json"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--output and --report specify different files") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestEvaluateCommandWritesFailedReportToOutputFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checks.leia")
	src := `evaluate "file failure report" {
    assert(false, "expected failure")
}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(dir, "eval-report.json")
	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--json", "--output", reportPath, path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var report evaluate.Report
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, string(data))
	}
	if report.Status != "failed" || len(report.Cases) != 1 || report.Cases[0].Status != "failed" {
		t.Fatalf("report = %#v", report)
	}
}

func TestEvaluateCommandGateFlagKeepsFailureExit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fail.leia")
	src := `evaluate "gate failure" {
    assert(false, "boom")
}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--gate", "--format=text", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "evaluate: failed") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestEvaluateCommandBaselineComparisonPassesWithinThreshold(t *testing.T) {
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "baseline.json")
	baseline := evaluate.Report{
		SchemaVersion: 1,
		Status:        "ok",
		Summary:       evaluate.Summary{PassRate: 0.9},
		Metrics: []evaluate.MetricSummary{{
			Name: "correct", Type: "bool", Count: 10, True: 9, False: 1, PassRate: 0.9,
		}},
	}
	writeEvaluateJSONReport(t, baselinePath, baseline)
	path := filepath.Join(dir, "checks.leia")
	src := `evaluate "baseline ok" {
    eval.case("row_1", func() { eval.metric("correct", true) })
    eval.case("row_2", func() { eval.metric("correct", true) })
}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--format=json", "--baseline", baselinePath, "--regression-threshold", "0.05", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	var report evaluate.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.Comparison == nil || report.Comparison.Summary == nil || report.Comparison.Summary.Regressed {
		t.Fatalf("comparison = %#v", report.Comparison)
	}
	if report.Comparison.Metrics[0].Name != "correct" || report.Comparison.Metrics[0].Regressed {
		t.Fatalf("metric comparison = %#v", report.Comparison.Metrics)
	}
}

func TestEvaluateCommandBaselineComparisonFailsGate(t *testing.T) {
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "baseline.json")
	baseline := evaluate.Report{
		SchemaVersion: 1,
		Status:        "ok",
		Summary:       evaluate.Summary{PassRate: 1},
		Metrics: []evaluate.MetricSummary{{
			Name: "correct", Type: "bool", Count: 2, True: 2, PassRate: 1,
		}},
	}
	writeEvaluateJSONReport(t, baselinePath, baseline)
	path := filepath.Join(dir, "checks.leia")
	src := `evaluate "baseline regression" {
    eval.case("row_1", func() { eval.metric("correct", true) })
    eval.case("row_2", func() { eval.metric("correct", false) })
}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--format=text", "--baseline", baselinePath, "--regression-threshold", "0.1", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q stdout=%q", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{"evaluate: failed", "comparison:", "summary pass_rate 1 -> 1", "metric correct bool 1 -> 0.5", "regressed"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestEvaluateCommandComparesExistingReports(t *testing.T) {
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "baseline.json")
	currentPath := filepath.Join(dir, "current.json")
	baseline := evaluate.Report{
		SchemaVersion: 1,
		Status:        "ok",
		Summary:       evaluate.Summary{PassRate: 1},
		Metrics: []evaluate.MetricSummary{{
			Name: "correct", Type: "bool", Count: 4, True: 4, PassRate: 1,
		}},
	}
	current := evaluate.Report{
		SchemaVersion: 1,
		Status:        "ok",
		Summary:       evaluate.Summary{PassRate: 1},
		Metrics: []evaluate.MetricSummary{{
			Name: "correct", Type: "bool", Count: 4, True: 2, False: 2, PassRate: 0.5,
		}},
	}
	writeEvaluateJSONReport(t, baselinePath, baseline)
	writeEvaluateJSONReport(t, currentPath, current)

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--compare", "--format=json", "--regression-threshold", "0.1", baselinePath, currentPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q stdout=%q", code, stderr.String(), stdout.String())
	}
	var report evaluate.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.Status != "failed" || report.Comparison == nil || report.Comparison.BaselinePath != baselinePath {
		t.Fatalf("report comparison = status %q comparison %#v", report.Status, report.Comparison)
	}
	if len(report.Comparison.Metrics) != 1 || report.Comparison.Metrics[0].Name != "correct" || !report.Comparison.Metrics[0].Regressed {
		t.Fatalf("metric comparison = %#v", report.Comparison.Metrics)
	}
	if len(report.Findings) != 1 || report.Findings[0].Kind != "evaluate_metric_regression" {
		t.Fatalf("findings = %#v", report.Findings)
	}
}

func TestEvaluateCommandCompareRejectsWrongArity(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--compare", "one.json"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--compare requires BASELINE and CURRENT report paths") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func writeEvaluateJSONReport(t *testing.T, path string, report evaluate.Report) {
	t.Helper()
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
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
	code := runEvaluateCommand([]string{"--format=json", "--replay", recordPath, path}, &stdout, &stderr)
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

func TestEvaluateCommandRejectsConflictingReplayAliases(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--replay", "short.json", "--llm-replay", "long.json"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runEvaluateCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--replay and --llm-replay specify different files") {
		t.Fatalf("stderr = %q", stderr.String())
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
