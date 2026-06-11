package leia_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericRecordReplayReportIsEvaluationCaseEvidence(t *testing.T) {
	root := repoRoot(t)
	recordReplay := genericEvidenceCapabilityByID(t, root, "generic.ai.record.replay")
	evaluationHarness := genericEvidenceCapabilityByID(t, root, "generic.ai.evaluation.harness")
	if !recordReplay.ProviderFree || !evaluationHarness.ProviderFree ||
		recordReplay.DomainSpecific || evaluationHarness.DomainSpecific ||
		recordReplay.FinRobotSpecificSyntaxAssumption || evaluationHarness.FinRobotSpecificSyntaxAssumption {
		t.Fatalf("composition capabilities must stay generic/provider-free: record=%#v eval=%#v", recordReplay, evaluationHarness)
	}

	specimen := writeGenericRecordReplayEvidenceSpecimen(t)
	successReport := runGenericRecordReplayEvidenceReport(t, root, specimen.sourcePath, specimen.successRecordsPath, true)
	successEvidence := genericEvidenceFromReplayReport(t, successReport, recordReplay, evaluationHarness, specimen.successRecordsPath)
	if successEvidence.Status != "passed" ||
		successEvidence.Summary.Status != "ok" ||
		successEvidence.Summary.LLMMode != "replay" ||
		successEvidence.Summary.LoadedTurns != 1 ||
		successEvidence.Summary.ReplayedTurns != 1 ||
		successEvidence.Summary.LLMTurns != 1 ||
		successEvidence.Summary.LLMErrors != 0 ||
		len(successEvidence.Findings) != 0 {
		t.Fatalf("success replay report is not usable evaluation evidence: %#v", successEvidence)
	}

	mismatchReport := runGenericRecordReplayEvidenceReport(t, root, specimen.sourcePath, specimen.mismatchRecordsPath, false)
	mismatchEvidence := genericEvidenceFromReplayReport(t, mismatchReport, recordReplay, evaluationHarness, specimen.mismatchRecordsPath)
	if mismatchEvidence.Status != "failed" ||
		mismatchEvidence.Summary.Status != "failed" ||
		mismatchEvidence.Summary.LLMMode != "replay" ||
		mismatchEvidence.Summary.LLMErrors == 0 ||
		!genericEvidenceHasFinding(mismatchEvidence, "llm_replay_mismatch") {
		t.Fatalf("mismatch replay report is not usable failure evidence: %#v", mismatchEvidence)
	}
	for _, finding := range mismatchEvidence.Findings {
		if finding.Kind == "" || finding.Severity == "" || finding.Message == "" || finding.Path == "" {
			t.Fatalf("finding is not structured evidence: %#v", finding)
		}
		if finding.Kind == "llm_replay_mismatch" {
			for _, key := range []string{"expected", "actual", "case_id", "case_name"} {
				if _, ok := finding.Details[key]; !ok {
					t.Fatalf("mismatch finding missing detail %q: %#v", key, finding)
				}
			}
		}
	}
}

type genericRecordReplayEvidenceSpecimen struct {
	sourcePath          string
	successRecordsPath  string
	mismatchRecordsPath string
}

func writeGenericRecordReplayEvidenceSpecimen(t *testing.T) genericRecordReplayEvidenceSpecimen {
	t.Helper()
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "generic_record_replay_evidence.leia")
	datasetPath := filepath.Join(dir, "generic_record_replay_evidence_dataset.jsonl")
	successRecordsPath := filepath.Join(dir, "generic_record_replay_evidence.records.json")
	mismatchRecordsPath := filepath.Join(dir, "generic_record_replay_evidence_mismatch.records.json")

	source := `evaluate "generic record replay evidence bridge" {
    rows := eval.load_jsonl("` + filepath.ToSlash(datasetPath) + `")
    eval.fail_if(#rows != 1, "dataset must contain one generic evidence row")

    for _, row := range ipairs(rows) {
        eval.case(row.id, func() {
            result, err := eval.judge({
                model: "mock-generic-eval-judge"
                messages: {
                    llm.system("Return pass when the answer satisfies the rubric."),
                    llm.user("Rubric: " .. row.rubric .. " Answer: " .. row.answer),
                }
            })
            assert(err == nil)
            assert(result.text == row.expected_judgment)
            eval.metric("judge_passed", result.text == row.expected_judgment)
        })
    }

    usage := eval.usage()
    assert(usage.turns == 1)
    eval.budget({turns: 1, tokens: 16, cost: 0.02})
}`
	dataset := `{"id":"generic-evidence-row","answer":"record replay reports include summary and findings as evidence","rubric":"mentions summary and findings as evidence.","expected_judgment":"pass"}` + "\n"
	records := `[
  {
    "Request": {
      "Model": "mock-generic-eval-judge",
      "Messages": [
        {
          "Role": "system",
          "Text": "Return pass when the answer satisfies the rubric."
        },
        {
          "Role": "user",
          "Text": "Rubric: mentions summary and findings as evidence. Answer: record replay reports include summary and findings as evidence"
        }
      ],
      "MaxTokens": 200
    },
    "Result": {
      "Status": "final_answer",
      "Text": "pass",
      "Usage": {
        "InputTokens": 4,
        "OutputTokens": 3,
        "Cost": 0.004
      }
    }
  }
]`
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(datasetPath, []byte(dataset), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(successRecordsPath, []byte(records), 0o600); err != nil {
		t.Fatal(err)
	}
	mismatch := strings.Replace(records, "mock-generic-eval-judge", "mock-generic-eval-drift", 1)
	if err := os.WriteFile(mismatchRecordsPath, []byte(mismatch), 0o600); err != nil {
		t.Fatal(err)
	}
	return genericRecordReplayEvidenceSpecimen{
		sourcePath:          sourcePath,
		successRecordsPath:  successRecordsPath,
		mismatchRecordsPath: mismatchRecordsPath,
	}
}

type genericEvidenceReport struct {
	Status  string `json:"status"`
	Summary struct {
		EvaluateBlocks int `json:"evaluate_blocks"`
		CasesPassed    int `json:"cases_passed"`
		CasesFailed    int `json:"cases_failed"`
		Assertions     int `json:"assertions"`
	} `json:"summary"`
	LLM *struct {
		Mode          string `json:"mode"`
		LoadedTurns   int    `json:"loaded_turns"`
		ReplayedTurns int    `json:"replayed_turns"`
		Turns         int    `json:"turns"`
		Errors        int    `json:"errors"`
	} `json:"llm"`
	Cases []struct {
		CaseID     string `json:"case_id"`
		Name       string `json:"name"`
		SourcePath string `json:"source_path"`
		Status     string `json:"status"`
		LLM        *struct {
			TraceRef string `json:"trace_ref"`
		} `json:"llm"`
	} `json:"cases"`
	Findings []struct {
		Kind     string         `json:"kind"`
		Severity string         `json:"severity"`
		Message  string         `json:"message"`
		Path     string         `json:"path"`
		Details  map[string]any `json:"details"`
	} `json:"findings"`
}

func runGenericRecordReplayEvidenceReport(t *testing.T, root, sourcePath, replayPath string, wantSuccess bool) genericEvidenceReport {
	t.Helper()
	reportPath := filepath.Join(t.TempDir(), "generic-record-replay-evidence.report.json")
	cmd := exec.Command("go", "run", "./cmd/leia", "evaluate", "--gate", "--report", reportPath, "--replay", replayPath, sourcePath)
	cmd.Dir = root
	cmd.Env = withoutLLMProviderEnv(os.Environ())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if wantSuccess && err != nil {
		t.Fatalf("evaluate failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if !wantSuccess && err == nil {
		t.Fatalf("evaluate unexpectedly passed\nstdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}
	data, readErr := os.ReadFile(reportPath)
	if readErr != nil {
		t.Fatalf("read report: %v\nstdout:\n%s\nstderr:\n%s", readErr, stdout.String(), stderr.String())
	}
	var report genericEvidenceReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, string(data))
	}
	return report
}

type genericRecordReplayEvaluationEvidence struct {
	SchemaVersion        int    `json:"schema_version"`
	EvidenceKind         string `json:"evidence_kind"`
	ProducerCapabilityID string `json:"producer_capability_id"`
	ConsumerCapabilityID string `json:"consumer_capability_id"`
	ProviderFree         bool   `json:"provider_free"`
	DomainSpecific       bool   `json:"domain_specific"`
	ReplayRecordsPath    string `json:"replay_records_path"`
	CaseID               string `json:"case_id"`
	CaseName             string `json:"case_name"`
	Status               string `json:"status"`
	SourcePath           string `json:"source_path"`
	TraceRef             string `json:"trace_ref"`
	Summary              struct {
		Status         string `json:"status"`
		EvaluateBlocks int    `json:"evaluate_blocks"`
		CasesPassed    int    `json:"cases_passed"`
		CasesFailed    int    `json:"cases_failed"`
		Assertions     int    `json:"assertions"`
		LLMMode        string `json:"llm_mode"`
		LoadedTurns    int    `json:"loaded_turns"`
		ReplayedTurns  int    `json:"replayed_turns"`
		LLMTurns       int    `json:"llm_turns"`
		LLMErrors      int    `json:"llm_errors"`
	} `json:"summary"`
	Findings []struct {
		Kind     string         `json:"kind"`
		Severity string         `json:"severity"`
		Message  string         `json:"message"`
		Path     string         `json:"path"`
		Details  map[string]any `json:"details"`
	} `json:"findings"`
}

func genericEvidenceFromReplayReport(
	t *testing.T,
	report genericEvidenceReport,
	recordReplay genericAIDialectIndexItem,
	evaluationHarness genericAIDialectIndexItem,
	replayRecordsPath string,
) genericRecordReplayEvaluationEvidence {
	t.Helper()
	if len(report.Cases) != 1 {
		t.Fatalf("report cases = %d, want one evidence-producing case", len(report.Cases))
	}
	c := report.Cases[0]
	if c.LLM == nil {
		t.Fatalf("case %q missing llm trace: %#v", c.Name, c)
	}
	evidence := genericRecordReplayEvaluationEvidence{
		SchemaVersion:        1,
		EvidenceKind:         "generic_record_replay.case_evidence",
		ProducerCapabilityID: recordReplay.CapabilityID,
		ConsumerCapabilityID: evaluationHarness.CapabilityID,
		ProviderFree:         recordReplay.ProviderFree && evaluationHarness.ProviderFree,
		DomainSpecific:       recordReplay.DomainSpecific || evaluationHarness.DomainSpecific,
		ReplayRecordsPath:    replayRecordsPath,
		CaseID:               c.CaseID,
		CaseName:             c.Name,
		Status:               c.Status,
		SourcePath:           c.SourcePath,
		TraceRef:             c.LLM.TraceRef,
		Findings: append([]struct {
			Kind     string         `json:"kind"`
			Severity string         `json:"severity"`
			Message  string         `json:"message"`
			Path     string         `json:"path"`
			Details  map[string]any `json:"details"`
		}(nil), report.Findings...),
	}
	evidence.Summary.Status = report.Status
	evidence.Summary.EvaluateBlocks = report.Summary.EvaluateBlocks
	evidence.Summary.CasesPassed = report.Summary.CasesPassed
	evidence.Summary.CasesFailed = report.Summary.CasesFailed
	evidence.Summary.Assertions = report.Summary.Assertions
	if report.LLM != nil {
		evidence.Summary.LLMMode = report.LLM.Mode
		evidence.Summary.LoadedTurns = report.LLM.LoadedTurns
		evidence.Summary.ReplayedTurns = report.LLM.ReplayedTurns
		evidence.Summary.LLMTurns = report.LLM.Turns
		evidence.Summary.LLMErrors = report.LLM.Errors
	}
	if evidence.EvidenceKind == "" || evidence.ProducerCapabilityID != "generic.ai.record.replay" ||
		evidence.ConsumerCapabilityID != "generic.ai.evaluation.harness" ||
		!evidence.ProviderFree || evidence.DomainSpecific ||
		evidence.CaseID == "" || evidence.CaseName == "" ||
		evidence.SourcePath == "" || evidence.TraceRef == "" ||
		evidence.ReplayRecordsPath == "" {
		t.Fatalf("case evidence envelope incomplete: %#v", evidence)
	}
	return evidence
}

func genericEvidenceCapabilityByID(t *testing.T, root, capabilityID string) genericAIDialectIndexItem {
	t.Helper()
	index := loadGenericAIDialectIndex(t, root)
	for _, entry := range index.Entries {
		if entry.CapabilityID == capabilityID {
			return entry
		}
	}
	t.Fatalf("generic AI dialect capability %q missing", capabilityID)
	return genericAIDialectIndexItem{}
}

func genericEvidenceHasFinding(evidence genericRecordReplayEvaluationEvidence, kind string) bool {
	for _, finding := range evidence.Findings {
		if finding.Kind == kind {
			return true
		}
	}
	return false
}
