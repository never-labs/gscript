package evaluate

import (
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/llm"
)

func TestRunReportsSyntaxSkeletonAndTODOs(t *testing.T) {
	dir := t.TempDir()
	src := `// TODO: wire real eval fixtures later
models { default: "mock" }

// Echoes input.
//leia:requires none
//leia:param text input text
tool echo(text) {
    return text, nil
}

agent answer(question) {
    model: "mock"
    user: question
    tools: [echo]
}

evaluate "answer echoes through tool" {
    agent case_answer(question) {
        model: "mock"
        user: question
        tools: [echo]
    }
}
`
	path := filepath.Join(dir, "agent.leia")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != SchemaVersion || report.Phase != "runtime-minimal" || report.Status != "ok" {
		t.Fatalf("report header = %#v", report)
	}
	if _, err := time.Parse(time.RFC3339, report.StartedAt); err != nil {
		t.Fatalf("started_at = %q: %v", report.StartedAt, err)
	}
	if report.Runtime.LeiaVersion == "" || report.Runtime.GoVersion == "" || report.Runtime.GOOS != goruntime.GOOS || report.Runtime.GOARCH != goruntime.GOARCH {
		t.Fatalf("runtime = %#v", report.Runtime)
	}
	if report.Summary.Files != 1 || report.Summary.ParsedFiles != 1 {
		t.Fatalf("summary files = %#v", report.Summary)
	}
	if report.Summary.Agents != 2 || report.Summary.Tools != 1 || report.Summary.Models != 1 {
		t.Fatalf("summary LLM counts = %#v", report.Summary)
	}
	if report.Summary.EvaluateBlocks != 1 {
		t.Fatalf("summary evaluate blocks = %#v", report.Summary)
	}
	if len(report.Cases) != 1 || report.Cases[0].Name != "answer echoes through tool" || report.Cases[0].Status != "passed" {
		t.Fatalf("cases = %#v", report.Cases)
	}
	if _, err := time.Parse(time.RFC3339, report.Cases[0].StartedAt); err != nil {
		t.Fatalf("case started_at = %q: %v", report.Cases[0].StartedAt, err)
	}
	if report.Summary.CasesSelected != 1 || report.Summary.CasesPassed != 1 || report.Summary.CasesFailed != 0 || report.Summary.CasesListed != 0 || report.Summary.Assertions != 0 || report.Summary.PassRate != 1 {
		t.Fatalf("summary execution = %#v", report.Summary)
	}
	if report.Cases[0].SourcePath != path || report.Cases[0].Range.StartLine == 0 || report.Cases[0].CaseID == "" {
		t.Fatalf("case source metadata = %#v", report.Cases[0])
	}
	if report.Summary.TODOs != 1 {
		t.Fatalf("summary TODOs = %#v", report.Summary)
	}
	if len(report.Findings) != 1 || report.Findings[0].Kind != "todo" {
		t.Fatalf("findings = %#v, want one TODO finding", report.Findings)
	}
}

func TestRunExecutesEvaluateBodyAndReportsFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checks.leia")
	if err := os.WriteFile(path, []byte(`evaluate "math still works" {
    value := 2 + 2
    assert(value == 5, "expected five")
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if len(report.Cases) != 1 || report.Cases[0].Status != "failed" {
		t.Fatalf("cases = %#v", report.Cases)
	}
	if report.Summary.CasesSelected != 1 || report.Summary.CasesPassed != 0 || report.Summary.CasesFailed != 1 || report.Summary.Assertions != 1 || report.Summary.PassRate != 0 {
		t.Fatalf("summary execution = %#v", report.Summary)
	}
	if len(report.Findings) != 1 || report.Findings[0].Kind != "case_runtime_error" {
		t.Fatalf("findings = %#v, want case_runtime_error", report.Findings)
	}
	if report.Findings[0].Line != 1 || report.Findings[0].Column != 1 {
		t.Fatalf("finding source = %#v", report.Findings[0])
	}
}

func TestRunCollectsEvalMetricsAndSubcases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "eval_cases.leia")
	if err := os.WriteFile(path, []byte(`evaluate "dataset metrics" {
    eval.metric("outer_count", 2)
    ok, err := eval.case("case_001", func() {
        eval.metric("correct", true)
        eval.metric("label", "refund")
        assert(true)
    })
    assert(ok)
    ok2, err2 := eval.case("case_002", func() {
        eval.metric("correct", false)
        assert(false, "intentional failure")
    })
    assert(!ok2)
    assert(err2 != nil)
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "failed" || report.Summary.CasesFailed != 1 {
		t.Fatalf("report = %#v", report)
	}
	if len(report.Cases) != 1 {
		t.Fatalf("cases = %#v", report.Cases)
	}
	c := report.Cases[0]
	if c.Status != "failed" || len(c.Metrics) != 1 || c.Metrics[0].Name != "outer_count" || c.Metrics[0].Value != int64(2) {
		t.Fatalf("case metrics/status = %#v", c)
	}
	if len(c.Subcases) != 2 {
		t.Fatalf("subcases = %#v", c.Subcases)
	}
	if c.Subcases[0].CaseID != "case_001" || c.Subcases[0].Status != "passed" || len(c.Subcases[0].Metrics) != 2 {
		t.Fatalf("subcase 1 = %#v", c.Subcases[0])
	}
	if c.Subcases[1].CaseID != "case_002" || c.Subcases[1].Status != "failed" || len(c.Subcases[1].Diagnostics) != 1 {
		t.Fatalf("subcase 2 = %#v", c.Subcases[1])
	}
	if len(report.Metrics) != 3 {
		t.Fatalf("metric summaries = %#v", report.Metrics)
	}
	summaries := metricSummariesByName(report.Metrics)
	if summaries["correct"].Type != "bool" || summaries["correct"].Count != 2 || summaries["correct"].True != 1 || summaries["correct"].False != 1 || summaries["correct"].PassRate != 0.5 {
		t.Fatalf("correct summary = %#v", summaries["correct"])
	}
	if summaries["label"].Type != "string" || summaries["label"].Values["refund"] != 1 {
		t.Fatalf("label summary = %#v", summaries["label"])
	}
	if summaries["outer_count"].Type != "number" || summaries["outer_count"].Mean != 2 {
		t.Fatalf("outer_count summary = %#v", summaries["outer_count"])
	}
	text := FormatText(report)
	for _, want := range []string{
		"metrics:",
		"correct bool pass_rate=0.50 true=1 false=1 count=2",
		"1 metrics, 2 subcases",
		"metric outer_count=2 (number)",
		"PASS case case_001",
		"FAIL case case_002",
		"metric correct=false (bool)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text report missing %q:\n%s", want, text)
		}
	}
}

func TestRunEvalLoadJSONLAndSkipIf(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cases.jsonl"), []byte(
		"{\"id\":\"keep\",\"expected\":\"refund\"}\n"+
			"{\"id\":\"skip\",\"expected\":\"exchange\",\"skip\":true}\n",
	), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "eval_jsonl.leia")
	if err := os.WriteFile(path, []byte(`evaluate "jsonl corpus" {
    cases := eval.load_jsonl("cases.jsonl")
    for _, case := range cases {
        eval.case(case.id, func() {
            if eval.skip_if(case.skip, "fixture disabled") {
                return
            }
            eval.metric("expected", case.expected)
            eval.fail_if(case.expected != "refund", "unexpected label")
        })
    }
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "ok" || len(report.Cases) != 1 {
		t.Fatalf("report = %#v", report)
	}
	subcases := report.Cases[0].Subcases
	if len(subcases) != 2 {
		t.Fatalf("subcases = %#v", subcases)
	}
	if subcases[0].CaseID != "keep" || subcases[0].Status != "passed" || len(subcases[0].Metrics) != 1 || subcases[0].Metrics[0].Value != "refund" {
		t.Fatalf("first subcase = %#v", subcases[0])
	}
	if subcases[1].CaseID != "skip" || subcases[1].Status != "skipped" || len(subcases[1].Diagnostics) != 1 {
		t.Fatalf("second subcase = %#v", subcases[1])
	}
	summaries := metricSummariesByName(report.Metrics)
	if summaries["expected"].Count != 1 || summaries["expected"].Values["refund"] != 1 || summaries["expected"].Values["exchange"] != 0 {
		t.Fatalf("expected summary includes skipped row or is missing value: %#v", summaries["expected"])
	}
}

func TestRunParallelPreservesReportSemantics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parallel.leia")
	if err := os.WriteFile(path, []byte(`evaluate "case one" {
    eval.metric("correct", true)
    assert(true)
}

evaluate "case two" {
    eval.metric("correct", true)
    assert(true)
}

evaluate "case three" {
    eval.metric("label", "stable")
    assert(true)
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{path}, Parallel: 3})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "ok" || report.Summary.EvaluateBlocks != 3 || report.Summary.CasesSelected != 3 || report.Summary.CasesPassed != 3 {
		t.Fatalf("summary/report = status %q summary %#v", report.Status, report.Summary)
	}
	if len(report.Cases) != 3 {
		t.Fatalf("cases = %#v", report.Cases)
	}
	for i, want := range []string{"case one", "case two", "case three"} {
		if report.Cases[i].Name != want || report.Cases[i].Status != "passed" {
			t.Fatalf("case %d = %#v, want %q passed", i, report.Cases[i], want)
		}
	}
	summaries := metricSummariesByName(report.Metrics)
	if summaries["correct"].Type != "bool" || summaries["correct"].Count != 2 || summaries["correct"].PassRate != 1 {
		t.Fatalf("correct summary = %#v", summaries["correct"])
	}
	if summaries["label"].Type != "string" || summaries["label"].Values["stable"] != 1 {
		t.Fatalf("label summary = %#v", summaries["label"])
	}
	if !hasNoteContaining(report.Notes, "parallel evaluate execution: 3 workers") {
		t.Fatalf("notes = %#v", report.Notes)
	}
}

func TestRunParallelPreservesFailureAccounting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parallel_fail.leia")
	if err := os.WriteFile(path, []byte(`evaluate "passes" {
    assert(true)
}

evaluate "assert fails" {
    assert(false, "intentional")
}

evaluate "subcase fails" {
    eval.case("row", func() {
        assert(false, "row failed")
    })
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{path}, Parallel: 2})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "failed" || report.Summary.CasesPassed != 1 || report.Summary.CasesFailed != 2 {
		t.Fatalf("report = status %q summary %#v", report.Status, report.Summary)
	}
	if len(report.Cases) != 3 || report.Cases[0].Status != "passed" || report.Cases[1].Status != "failed" || report.Cases[2].Status != "failed" {
		t.Fatalf("cases = %#v", report.Cases)
	}
	kinds := map[string]int{}
	for _, finding := range report.Findings {
		kinds[finding.Kind]++
	}
	if kinds["case_runtime_error"] != 1 || kinds["eval_subcase_failure"] != 1 {
		t.Fatalf("findings = %#v", report.Findings)
	}
	if len(report.Inputs) != 1 || report.Inputs[0].Status != "error" {
		t.Fatalf("inputs = %#v", report.Inputs)
	}
}

func TestRunParallelListOnlyDoesNotExecute(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parallel_list.leia")
	if err := os.WriteFile(path, []byte(`evaluate "listed only" {
    assert(false, "must not execute")
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{path}, ListOnly: true, Parallel: 4})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "ok" || len(report.Cases) != 1 || report.Cases[0].Status != "listed" {
		t.Fatalf("report = %#v", report)
	}
	if report.Summary.CasesListed != 1 || report.Summary.CasesFailed != 0 {
		t.Fatalf("summary = %#v", report.Summary)
	}
}

func TestRunParallelRejectsNegativeWorkers(t *testing.T) {
	_, err := Run(Options{Paths: []string{"."}, Parallel: -1})
	if err == nil || !strings.Contains(err.Error(), "parallel must be non-negative") {
		t.Fatalf("err = %v", err)
	}
}

func hasNoteContaining(notes []string, want string) bool {
	for _, note := range notes {
		if strings.Contains(note, want) {
			return true
		}
	}
	return false
}

func metricSummariesByName(metrics []MetricSummary) map[string]MetricSummary {
	out := map[string]MetricSummary{}
	for _, metric := range metrics {
		out[metric.Name] = metric
	}
	return out
}

func TestRunMarksFailingAssertionBySourcePosition(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checks.leia")
	if err := os.WriteFile(path, []byte(`evaluate "second assertion fails" {
    assert(true, "first assertion should pass")
    value := 2 + 2
    assert(value == 5, "second assertion should fail")
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "failed" || len(report.Cases) != 1 || report.Cases[0].Status != "failed" {
		t.Fatalf("report = %#v", report)
	}
	assertions := report.Cases[0].Assertions
	if len(assertions) != 2 {
		t.Fatalf("assertions = %#v", assertions)
	}
	if assertions[0].Status != "unknown" || assertions[1].Status != "failed" {
		t.Fatalf("assertions = %#v, want only second assertion failed", assertions)
	}
}

func TestRunFiltersEvaluateCases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cases.leia")
	if err := os.WriteFile(path, []byte(`evaluate "alpha case" {
    assert(false, "alpha should be filtered out")
}

evaluate "beta case" {
    assert(true)
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{path}, Filter: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "ok" || report.Summary.EvaluateBlocks != 2 || len(report.Cases) != 1 || report.Cases[0].Name != "beta case" {
		t.Fatalf("report = %#v", report)
	}
	if report.Summary.CasesSelected != 1 || report.Summary.CasesSkipped != 1 || report.Summary.CasesPassed != 1 {
		t.Fatalf("summary filter counts = %#v", report.Summary)
	}
}

func TestRunListsEvaluateCasesWithoutExecuting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cases.leia")
	if err := os.WriteFile(path, []byte(`evaluate "listed failure" {
    assert(false, "list mode must not execute")
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{path}, ListOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "ok" || len(report.Cases) != 1 || report.Cases[0].Status != "listed" {
		t.Fatalf("report = %#v", report)
	}
	if report.Cases[0].StartedAt != "" || report.Cases[0].DurationMS != 0 {
		t.Fatalf("listed case execution metadata = %#v", report.Cases[0])
	}
	if report.Summary.CasesSelected != 1 || report.Summary.CasesListed != 1 || report.Summary.CasesPassed != 0 || report.Summary.CasesFailed != 0 || report.Summary.Assertions != 1 {
		t.Fatalf("summary list counts = %#v", report.Summary)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("findings = %#v", report.Findings)
	}
}

func TestRunEvaluateFailureExampleReportsFailedCase(t *testing.T) {
	examplePath := filepath.Join("..", "..", "..", "examples", "evaluate", "failing_assert.leia")

	report, err := Run(Options{Paths: []string{examplePath}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if report.Summary.Files != 1 || report.Summary.ParsedFiles != 1 || report.Summary.EvaluateBlocks != 1 {
		t.Fatalf("summary = %#v", report.Summary)
	}
	if len(report.Cases) != 1 || report.Cases[0].Name != "risk threshold catches stale expectation" || report.Cases[0].Status != "failed" {
		t.Fatalf("cases = %#v", report.Cases)
	}
	if len(report.Findings) != 1 || report.Findings[0].Kind != "case_runtime_error" {
		t.Fatalf("findings = %#v, want case_runtime_error", report.Findings)
	}
	if !strings.Contains(report.Findings[0].Message, "expected the report to show this failed assertion") {
		t.Fatalf("finding message = %q", report.Findings[0].Message)
	}
}

func TestRunReportsAISyntaxValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.leia")
	if err := os.WriteFile(path, []byte(`tool missing_caps() { return nil, nil }`), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if len(report.Findings) != 1 || report.Findings[0].Kind != "ai_syntax_error" {
		t.Fatalf("findings = %#v, want ai_syntax_error", report.Findings)
	}
}

func TestRunRecordsLLMTurns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "llm_eval.leia")
	if err := os.WriteFile(path, []byte(`models {
    default: {
        protocol: "openai_compatible"
        provider_model: "mock-fast"
    }
}

evaluate "records llm turn" {
    result, err := llm.turn({
        messages: {llm.user("hello")},
    })
    assert(err == nil)
    assert(result.text == "recorded")
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(dir, "records.json")

	report, err := Run(Options{
		Paths:         []string{path},
		LLMRecordPath: recordPath,
		LLMProviderFactory: func(cfg runtime.LLMProviderConfig) (runtime.LLMProvider, error) {
			if cfg.Protocol != "openai_compatible" || cfg.ProviderModel != "mock-fast" {
				t.Fatalf("cfg = %#v, want mock provider config", cfg)
			}
			return testRuntimeLLMProvider{res: runtime.LLMTurnResult{
				Status: "final_answer",
				Text:   "recorded",
				Usage:  runtime.LLMTurnUsage{InputTokens: 11, OutputTokens: 7, Cost: 0.012, LatencyMS: 123},
			}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "ok" || len(report.Cases) != 1 || report.Cases[0].Status != "passed" {
		t.Fatalf("report = %#v", report)
	}
	records, err := llm.LoadRecords(recordPath)
	if err != nil {
		t.Fatalf("load records: %v", err)
	}
	if len(records) != 1 || records[0].Request.Messages[0].Text != "hello" || records[0].Result.Text != "recorded" {
		t.Fatalf("records = %#v", records)
	}
	if report.LLM == nil || report.LLM.Turns != 1 || report.LLM.InputTokens != 11 || report.LLM.OutputTokens != 7 || report.LLM.LatencyMS != 123 || report.LLM.Cost != 0.012 {
		t.Fatalf("llm summary = %#v", report.LLM)
	}
	if report.Cases[0].LLM == nil || report.Cases[0].LLM.TraceRef == "" || report.Cases[0].LLM.Turns != 1 || report.Cases[0].LLM.InputTokens != 11 {
		t.Fatalf("case llm = %#v", report.Cases[0].LLM)
	}
}

func TestRunReplaysLLMTurns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "llm_eval.leia")
	if err := os.WriteFile(path, []byte(`evaluate "replays llm turn" {
    result, err := llm.turn({
        model: "mock-fast",
        messages: {llm.user("hello")},
    })
    assert(err == nil)
    assert(result.text == "from replay")
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(dir, "records.json")
	if err := llm.SaveRecords(recordPath, []llm.Record{{
		Request: llm.TurnRequest{
			Model:    "mock-fast",
			Messages: []llm.Message{{Role: "user", Text: "hello"}},
		},
		Result: llm.TurnResult{
			Status: "final_answer",
			Text:   "from replay",
			Usage:  llm.TurnUsage{InputTokens: 3, OutputTokens: 4, Cost: 0.005, LatencyMS: 44},
		},
	}}); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{path}, LLMReplayPath: recordPath})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "ok" || len(report.Cases) != 1 || report.Cases[0].Status != "passed" {
		t.Fatalf("report = %#v", report)
	}
	if report.LLM == nil || report.LLM.Mode != "replay" || report.LLM.LoadedTurns != 1 || report.LLM.ReplayedTurns != 1 || report.LLM.RemainingTurns != 0 {
		t.Fatalf("llm report = %#v", report.LLM)
	}
	if report.LLM.Turns != 1 || report.LLM.InputTokens != 3 || report.LLM.OutputTokens != 4 || report.LLM.LatencyMS != 44 || report.LLM.Cost != 0.005 {
		t.Fatalf("llm usage = %#v", report.LLM)
	}
	if report.Cases[0].LLM == nil || report.Cases[0].LLM.TraceRef == "" || report.Cases[0].LLM.Turns != 1 {
		t.Fatalf("case llm = %#v", report.Cases[0].LLM)
	}
}

func TestRunFailsWhenReplayHasUnconsumedTurns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "llm_eval.leia")
	if err := os.WriteFile(path, []byte(`evaluate "does not call llm" {
    assert(true)
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(dir, "records.json")
	if err := llm.SaveRecords(recordPath, []llm.Record{{
		Request: llm.TurnRequest{
			Model:    "mock-fast",
			Messages: []llm.Message{{Role: "user", Text: "unused"}},
		},
		Result: llm.TurnResult{Status: "final_answer", Text: "unused"},
	}}); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{path}, LLMReplayPath: recordPath})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "failed" || report.LLM == nil || report.LLM.RemainingTurns != 1 {
		t.Fatalf("report = %#v", report)
	}
	if len(report.Findings) != 1 || report.Findings[0].Kind != "llm_replay_unconsumed" {
		t.Fatalf("findings = %#v", report.Findings)
	}
}

func TestRunFailsWhenReplayRequestMismatches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "llm_eval.leia")
	if err := os.WriteFile(path, []byte(`evaluate "request mismatch" {
    result, err := llm.turn({
        model: "mock-fast",
        messages: {llm.user("actual")},
    })
    assert(result == nil)
    assert(err != nil)
    assert(err.message == "llm replay request mismatch at turn 0")
}
`), 0644); err != nil {
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

	report, err := Run(Options{Paths: []string{path}, LLMReplayPath: recordPath})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "failed" || len(report.Cases) != 1 || report.Cases[0].Status != "passed" {
		t.Fatalf("report = %#v", report)
	}
	if report.LLM == nil || report.LLM.ReplayedTurns != 1 || report.LLM.RemainingTurns != 0 {
		t.Fatalf("llm report = %#v", report.LLM)
	}
	if len(report.Cases[0].Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", report.Cases[0].Diagnostics)
	}
	if len(report.Findings) != 1 || report.Findings[0].Kind != "llm_replay_mismatch" || !strings.Contains(report.Findings[0].Message, "llm replay request mismatch") {
		t.Fatalf("findings = %#v", report.Findings)
	}
	if got := report.Findings[0].Details["turn"]; got != 0 {
		t.Fatalf("finding details turn = %#v", got)
	}
	if report.Findings[0].Path != path || report.Findings[0].Line == 0 {
		t.Fatalf("finding source = %#v", report.Findings[0])
	}
	if report.Findings[0].Details["case_id"] != report.Cases[0].CaseID || report.Findings[0].Details["case_name"] != "request mismatch" {
		t.Fatalf("finding case details = %#v", report.Findings[0].Details)
	}
	expected, ok := report.Findings[0].Details["expected"].(map[string]any)
	if !ok || expected["model"] != "mock-fast" {
		t.Fatalf("finding details expected = %#v", report.Findings[0].Details["expected"])
	}
	expectedMessages, ok := expected["messages"].([]map[string]any)
	if !ok || len(expectedMessages) != 1 || expectedMessages[0]["text"] != "expected" {
		t.Fatalf("finding details expected messages = %#v", expected["messages"])
	}
	actual, ok := report.Findings[0].Details["actual"].(map[string]any)
	if !ok || actual["model"] != "mock-fast" {
		t.Fatalf("finding details actual = %#v", report.Findings[0].Details["actual"])
	}
	actualMessages, ok := actual["messages"].([]map[string]any)
	if !ok || len(actualMessages) != 1 || actualMessages[0]["text"] != "actual" {
		t.Fatalf("finding details actual messages = %#v", actual["messages"])
	}
}

func TestRunFailsWhenReplayIsExhausted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "llm_eval.leia")
	if err := os.WriteFile(path, []byte(`evaluate "replay exhausted" {
    result, err := llm.turn({
        model: "mock-fast",
        messages: {llm.user("hello")},
    })
    assert(result == nil)
    assert(err != nil)
    assert(err.message == "llm replay exhausted at turn 0")
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(dir, "records.json")
	if err := llm.SaveRecords(recordPath, nil); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{path}, LLMReplayPath: recordPath})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "failed" || len(report.Cases) != 1 || report.Cases[0].Status != "passed" {
		t.Fatalf("report = %#v", report)
	}
	if report.LLM == nil || report.LLM.LoadedTurns != 0 || report.LLM.ReplayedTurns != 0 || report.LLM.RemainingTurns != 0 {
		t.Fatalf("llm report = %#v", report.LLM)
	}
	if len(report.Cases[0].Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", report.Cases[0].Diagnostics)
	}
	if len(report.Findings) != 1 || report.Findings[0].Kind != "llm_replay_exhausted" || !strings.Contains(report.Findings[0].Message, "llm replay exhausted") {
		t.Fatalf("findings = %#v", report.Findings)
	}
	if got := report.Findings[0].Details["turn"]; got != 0 {
		t.Fatalf("finding details turn = %#v", got)
	}
	if report.Findings[0].Path != path || report.Findings[0].Details["case_id"] != report.Cases[0].CaseID {
		t.Fatalf("finding attribution = %#v", report.Findings[0])
	}
}

func TestRunUpdateGoldenWritesLLMFixture(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "llm_eval.leia")
	if err := os.WriteFile(path, []byte(`models {
    default: {
        protocol: "openai_compatible"
        provider_model: "mock-fast"
    }
}

evaluate "updates golden" {
    result, err := llm.turn({
        messages: {llm.user("refresh")},
    })
    assert(err == nil)
    assert(result.text == "fresh")
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(dir, "golden.json")

	report, err := Run(Options{
		Paths:               []string{path},
		LLMUpdateGoldenPath: recordPath,
		LLMProviderFactory: func(runtime.LLMProviderConfig) (runtime.LLMProvider, error) {
			return testRuntimeLLMProvider{res: runtime.LLMTurnResult{Status: "final_answer", Text: "fresh"}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "ok" || report.LLM == nil || report.LLM.Mode != "update_golden" || !report.LLM.GoldenUpdated || report.LLM.RecordedTurns != 1 {
		t.Fatalf("report = %#v", report)
	}
	records, err := llm.LoadRecords(recordPath)
	if err != nil {
		t.Fatalf("load records: %v", err)
	}
	if len(records) != 1 || records[0].Request.Messages[0].Text != "refresh" || records[0].Result.Text != "fresh" {
		t.Fatalf("records = %#v", records)
	}
}

func TestRunRejectsRecordAndReplayTogether(t *testing.T) {
	_, err := Run(Options{LLMRecordPath: "records.json", LLMReplayPath: "records.json"})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("err = %v, want mutually exclusive", err)
	}
}

type testRuntimeLLMProvider struct {
	res runtime.LLMTurnResult
	err error
}

func (p testRuntimeLLMProvider) Turn(context.Context, runtime.LLMTurnRequest) (runtime.LLMTurnResult, error) {
	return p.res, p.err
}
