package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
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
