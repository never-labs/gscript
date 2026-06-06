package evaluate

import (
	"math"
	"testing"
)

func TestAttachBaselineComparisonMarksSummaryAndBoolMetricRegressions(t *testing.T) {
	current := Report{
		Status:  "ok",
		Summary: Summary{PassRate: 0.89},
		Metrics: []MetricSummary{
			{Name: "accuracy", Type: "bool", Count: 10, PassRate: 0.7},
			{Name: "latency_ms", Type: "number", Count: 2, Mean: 120},
			{Name: "label", Type: "string", Count: 1, Values: map[string]int{"refund": 1}},
		},
	}
	baseline := Report{
		Status:  "ok",
		Summary: Summary{PassRate: 1},
		Metrics: []MetricSummary{
			{Name: "accuracy", Type: "bool", Count: 10, PassRate: 1},
			{Name: "latency_ms", Type: "number", Count: 2, Mean: 100},
			{Name: "label", Type: "string", Count: 1, Values: map[string]int{"shipping": 1}},
		},
	}

	AttachBaselineComparison(&current, baseline, "baseline.json", 0.05)

	if current.Status != "failed" {
		t.Fatalf("status = %q, want failed after regression", current.Status)
	}
	if current.Comparison == nil || current.Comparison.Summary == nil {
		t.Fatalf("comparison = %+v, want summary comparison", current.Comparison)
	}
	if !current.Comparison.Summary.Regressed || math.Abs(current.Comparison.Summary.DeltaPassRate-(-0.11)) > 0.0000001 {
		t.Fatalf("summary comparison = %+v, want pass-rate regression", current.Comparison.Summary)
	}
	if len(current.Comparison.Metrics) != 3 {
		t.Fatalf("metric comparisons = %+v, want bool, number, and string metrics", current.Comparison.Metrics)
	}
	var sawBoolRegression, sawNumberComparison, sawStringComparison bool
	for _, metric := range current.Comparison.Metrics {
		switch metric.Name + ":" + metric.Type {
		case "accuracy:bool":
			sawBoolRegression = metric.Regressed && metric.Baseline == 1 && metric.Current == 0.7
		case "latency_ms:number":
			sawNumberComparison = !metric.Regressed && metric.Baseline == 100 && metric.Current == 120 && metric.Delta == 20
		case "label:string":
			sawStringComparison = !metric.Regressed && metric.BaselineCount == 1 && metric.CurrentCount == 1
		}
	}
	if !sawBoolRegression || !sawNumberComparison || !sawStringComparison {
		t.Fatalf("metric comparisons = %+v, want bool regression plus non-regressing number/string comparisons", current.Comparison.Metrics)
	}
	if len(current.Findings) != 2 {
		t.Fatalf("findings = %+v, want summary and bool metric regression findings", current.Findings)
	}
	if current.Findings[0].Kind != "evaluate_regression" || current.Findings[1].Kind != "evaluate_metric_regression" {
		t.Fatalf("findings = %+v, want evaluate regression finding kinds", current.Findings)
	}
}
