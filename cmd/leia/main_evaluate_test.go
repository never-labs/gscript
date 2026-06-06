package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvaluateCorpusMetricsExampleCoversHarnessHelpers(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "examples", "evaluate", "corpus_metrics.leia")

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--json", sourcePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("evaluate corpus metrics code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}

	var report struct {
		Status  string `json:"status"`
		Summary struct {
			CasesPassed int `json:"cases_passed"`
			CasesFailed int `json:"cases_failed"`
			Assertions  int `json:"assertions"`
			Budgets     int `json:"budgets"`
		} `json:"summary"`
		Metrics []struct {
			Name     string  `json:"name"`
			Type     string  `json:"type"`
			Count    int     `json:"count"`
			PassRate float64 `json:"pass_rate"`
			Mean     float64 `json:"mean"`
		} `json:"metrics"`
		Cases []struct {
			Name     string `json:"name"`
			Status   string `json:"status"`
			Subcases []struct {
				CaseID  string `json:"case_id"`
				Status  string `json:"status"`
				Metrics []struct {
					Name string `json:"name"`
				} `json:"metrics"`
			} `json:"subcases"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON evaluate report: %v; stdout = %q", err, stdout.String())
	}
	if report.Status != "ok" || report.Summary.CasesPassed != 1 || report.Summary.CasesFailed != 0 || report.Summary.Assertions != 1 || report.Summary.Budgets != 0 {
		t.Fatalf("summary = %+v, want one passed evaluate with one top-level assertion", report.Summary)
	}
	if len(report.Cases) != 1 || report.Cases[0].Status != "passed" || len(report.Cases[0].Subcases) != 3 {
		t.Fatalf("cases = %+v, want one passed case with three subcases", report.Cases)
	}
	if report.Cases[0].Subcases[0].CaseID != "refund-damaged" || report.Cases[0].Subcases[0].Status != "passed" {
		t.Fatalf("first subcase = %+v, want refund-damaged passed", report.Cases[0].Subcases[0])
	}
	if report.Cases[0].Subcases[2].CaseID != "wip-edge" || report.Cases[0].Subcases[2].Status != "skipped" {
		t.Fatalf("third subcase = %+v, want wip-edge skipped", report.Cases[0].Subcases[2])
	}
	assertMetricSummary(t, report.Metrics, "correct", "bool", 2, 1, 0)
	assertMetricSummary(t, report.Metrics, "input_chars", "number", 2, 0, 0)
	assertMetricSummary(t, report.Metrics, "label", "string", 2, 0, 0)
}

func TestEvaluateCLIListFilterDoesNotExecuteCases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "filtered.leia")
	source := `
evaluate "refund flow" {
    assert(false)
}

evaluate "shipping flow" {
    assert(false)
}
`
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--json", "--list", "--filter", "refund", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("evaluate --list --filter code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var report struct {
		Status  string `json:"status"`
		Summary struct {
			EvaluateBlocks int `json:"evaluate_blocks"`
			CasesSelected  int `json:"cases_selected"`
			CasesListed    int `json:"cases_listed"`
			CasesSkipped   int `json:"cases_skipped"`
			CasesFailed    int `json:"cases_failed"`
		} `json:"summary"`
		Cases []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"cases"`
		Notes []string `json:"notes"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON evaluate report: %v; stdout = %q", err, stdout.String())
	}
	if report.Status != "ok" || report.Summary.EvaluateBlocks != 2 || report.Summary.CasesSelected != 1 || report.Summary.CasesListed != 1 || report.Summary.CasesSkipped != 1 || report.Summary.CasesFailed != 0 {
		t.Fatalf("summary = %+v, want one listed case and one skipped case with no execution failures", report.Summary)
	}
	if len(report.Cases) != 1 || report.Cases[0].Name != "refund flow" || report.Cases[0].Status != "listed" {
		t.Fatalf("cases = %+v, want only refund flow listed", report.Cases)
	}
	if !containsEvaluateString(report.Notes, "filter: refund") || !containsEvaluateString(report.Notes, "list mode: evaluate cases are discovered but not executed") {
		t.Fatalf("notes = %#v, want filter and list mode notes", report.Notes)
	}
}

func TestEvaluateCLIFormatAndReportRouting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "format.leia")
	source := `
evaluate "html <case>" {
    eval.metric("ready", true)
    assert(true)
}
`
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(dir, "report.html")

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--format=html", "--report", reportPath, path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("evaluate --format=html --report code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty when --report writes rendered output", stdout.String())
	}
	htmlBytes, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read html report: %v", err)
	}
	htmlReport := string(htmlBytes)
	if !strings.Contains(htmlReport, "<!doctype html>") || !strings.Contains(htmlReport, "html &lt;case&gt;") || !strings.Contains(htmlReport, "<h2>Metrics</h2>") {
		t.Fatalf("html report missing expected escaped case and metrics content:\n%s", htmlReport)
	}

	stdout.Reset()
	stderr.Reset()
	code = runEvaluateCommand([]string{"--format=text", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("evaluate --format=text code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "evaluate: ok") || !strings.Contains(stdout.String(), "PASS html <case>") || !strings.Contains(stdout.String(), "ready bool pass_rate=1.00") {
		t.Fatalf("text report missing expected summary, case, and metric:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = runEvaluateCommand([]string{"--format=xml", path}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("evaluate --format=xml code = %d, want 2", code)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), `unknown format "xml"`) {
		t.Fatalf("stdout = %q stderr = %q, want unknown format diagnostic on stderr only", stdout.String(), stderr.String())
	}
}

func TestEvaluateCLIBaselineCompareAndGateRegressions(t *testing.T) {
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "baseline.json")
	currentPath := filepath.Join(dir, "current.json")
	baselineJSON := `{
  "schema_version": 1,
  "phase": "runtime-minimal",
  "status": "ok",
  "summary": {"pass_rate": 1},
  "inputs": [],
  "cases": [],
  "metrics": [{"name": "quality", "type": "bool", "count": 2, "true": 2, "pass_rate": 1}],
  "findings": []
}`
	currentJSON := `{
  "schema_version": 1,
  "phase": "runtime-minimal",
  "status": "ok",
  "summary": {"pass_rate": 0.8},
  "inputs": [],
  "cases": [],
  "metrics": [{"name": "quality", "type": "bool", "count": 2, "true": 1, "false": 1, "pass_rate": 0.5}],
  "findings": []
}`
	if err := os.WriteFile(baselinePath, []byte(baselineJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(currentPath, []byte(currentJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	sourcePath := filepath.Join(dir, "current.leia")
	source := `
evaluate "quality gate" {
    eval.metric("quality", true)
    assert(true)
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--json", "--baseline", baselinePath, "--regression-threshold", "0.05", sourcePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("evaluate --baseline code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var baselineReport struct {
		Status     string `json:"status"`
		Comparison *struct {
			BaselinePath string `json:"baseline_path"`
			Summary      *struct {
				BaselinePassRate float64 `json:"baseline_pass_rate"`
				CurrentPassRate  float64 `json:"current_pass_rate"`
				Regressed        bool    `json:"regressed"`
			} `json:"summary"`
		} `json:"comparison"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &baselineReport); err != nil {
		t.Fatalf("baseline stdout is not JSON evaluate report: %v; stdout = %q", err, stdout.String())
	}
	if baselineReport.Status != "ok" || baselineReport.Comparison == nil || baselineReport.Comparison.BaselinePath != baselinePath || baselineReport.Comparison.Summary == nil || baselineReport.Comparison.Summary.BaselinePassRate != 1 || baselineReport.Comparison.Summary.CurrentPassRate != 1 || baselineReport.Comparison.Summary.Regressed {
		t.Fatalf("baseline comparison = %+v, want attached non-regressing comparison", baselineReport.Comparison)
	}

	stdout.Reset()
	stderr.Reset()
	code = runEvaluateCommand([]string{"--compare", baselinePath, currentPath, "--format=text", "--gate", "--regression-threshold", "0.05"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("evaluate --compare regression code = %d, want 1; stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty for rendered regression report", stderr.String())
	}
	for _, want := range []string{
		"evaluate: failed",
		"comparison: baseline=" + baselinePath + " threshold=0.05",
		"summary pass_rate 1 -> 0.8 (delta -0.2, regressed)",
		"metric quality bool 1 -> 0.5 (delta -0.5, regressed)",
		"evaluate_regression",
		"evaluate_metric_regression",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("compare text report missing %q:\n%s", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = runEvaluateCommand([]string{"--compare", baselinePath, currentPath, "--filter", "quality"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("evaluate --compare --filter code = %d, want 2", code)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "--compare cannot be combined") {
		t.Fatalf("stdout = %q stderr = %q, want compare combination diagnostic", stdout.String(), stderr.String())
	}
}

func assertMetricSummary(t *testing.T, metrics []struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Count    int     `json:"count"`
	PassRate float64 `json:"pass_rate"`
	Mean     float64 `json:"mean"`
}, name, typ string, count int, passRate, mean float64) {
	t.Helper()
	for _, metric := range metrics {
		if metric.Name != name {
			continue
		}
		if metric.Type != typ || metric.Count != count {
			t.Fatalf("metric %s = %+v, want type %s count %d", name, metric, typ, count)
		}
		if passRate != 0 && metric.PassRate != passRate {
			t.Fatalf("metric %s pass_rate = %v, want %v", name, metric.PassRate, passRate)
		}
		if mean != 0 && metric.Mean != mean {
			t.Fatalf("metric %s mean = %v, want %v", name, metric.Mean, mean)
		}
		return
	}
	t.Fatalf("missing metric summary %s in %+v", name, metrics)
}

func containsEvaluateString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
