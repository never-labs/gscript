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
			return testRuntimeLLMProvider{res: runtime.LLMTurnResult{Status: "final_answer", Text: "recorded"}}, nil
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
		Result: llm.TurnResult{Status: "final_answer", Text: "from replay"},
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
