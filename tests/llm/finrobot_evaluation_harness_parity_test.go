package leia_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type finrobotEvaluationHarnessParityManifest struct {
	AIEvaluationCapability struct {
		ID                   string   `json:"id"`
		Scope                string   `json:"scope"`
		DomainSpecific       bool     `json:"domain_specific"`
		NetworkPolicy        string   `json:"network_policy"`
		ModelPolicy          string   `json:"model_policy"`
		TestHardcodingPolicy string   `json:"test_hardcoding_policy"`
		DialectSurface       []string `json:"dialect_surface"`
		CapabilitySpecimen   struct {
			ID                string   `json:"id"`
			SourcePath        string   `json:"source_path"`
			DatasetPath       string   `json:"dataset_path"`
			ReplayRecordsPath string   `json:"replay_records_path"`
			SourceSHA256      string   `json:"source_sha256"`
			DatasetSHA256     string   `json:"dataset_sha256"`
			ReplaySHA256      string   `json:"replay_records_sha256"`
			TurnCount         int      `json:"turn_count"`
			Models            []string `json:"models"`
			DatasetRows       int      `json:"dataset_rows"`
			SubcaseIDs        []string `json:"subcase_ids"`
			GoldenReport      struct {
				Status           string   `json:"status"`
				EvaluateBlocks   int      `json:"evaluate_blocks"`
				CasesPassed      int      `json:"cases_passed"`
				CasesFailed      int      `json:"cases_failed"`
				Assertions       int      `json:"assertions"`
				LLMLoadedTurns   int      `json:"llm_loaded_turns"`
				LLMReplayedTurns int      `json:"llm_replayed_turns"`
				LLMTurns         int      `json:"llm_turns"`
				CaseNames        []string `json:"case_names"`
				Metrics          []struct {
					Name     string  `json:"name"`
					Type     string  `json:"type"`
					Count    int     `json:"count"`
					PassRate float64 `json:"pass_rate"`
					Mean     float64 `json:"mean"`
				} `json:"metrics"`
			} `json:"golden_report"`
		} `json:"capability_specimen"`
	} `json:"ai_evaluation_capability"`
	JudgeSpec struct {
		ID              string `json:"id"`
		RequestContract struct {
			RequiredFields   []string `json:"required_fields"`
			DefaultMaxTokens int      `json:"default_max_tokens"`
			MessageRoles     []string `json:"message_roles"`
		} `json:"request_contract"`
		ResponseContract struct {
			UsageMetrics       []string `json:"usage_metrics"`
			FailureFindingKind string   `json:"failure_finding_kind"`
		} `json:"response_contract"`
	} `json:"judge_spec"`
	MetricRegistry []struct {
		Name        string  `json:"name"`
		Type        string  `json:"type"`
		Aggregation string  `json:"aggregation"`
		GoldenMin   float64 `json:"golden_min"`
		GoldenMax   float64 `json:"golden_max"`
	} `json:"metric_registry"`
	DatasetManifest struct {
		Format         string   `json:"format"`
		IDField        string   `json:"id_field"`
		RequiredFields []string `json:"required_fields"`
		MinimumRows    int      `json:"minimum_rows"`
		RowPolicy      string   `json:"row_policy"`
	} `json:"dataset_manifest"`
	RecordReplayMatching struct {
		Mode                        string   `json:"mode"`
		MatchedRequestFields        []string `json:"matched_request_fields"`
		UnmatchedRequestFindingKind string   `json:"unmatched_request_finding_kind"`
		ExhaustedReplayFindingKind  string   `json:"exhausted_replay_finding_kind"`
		UnconsumedReplayFindingKind string   `json:"unconsumed_replay_finding_kind"`
		FailureDetails              []string `json:"failure_details"`
	} `json:"record_replay_matching"`
	ScoringTrace struct {
		ReportFields  []string `json:"report_fields"`
		CaseFields    []string `json:"case_fields"`
		LLMCaseFields []string `json:"llm_case_fields"`
		SubcaseFields []string `json:"subcase_fields"`
	} `json:"scoring_trace"`
	ProviderFreeModelStub struct {
		AllowedModelPrefix string   `json:"allowed_model_prefix"`
		LiveProviderCalls  bool     `json:"live_provider_calls"`
		NetworkCalls       bool     `json:"network_calls"`
		StubModes          []string `json:"stub_modes"`
	} `json:"provider_free_model_stub"`
	FailureEnvelope struct {
		StatusOnFailure    string   `json:"status_on_failure"`
		FindingKinds       []string `json:"finding_kinds"`
		RequiredFields     []string `json:"required_fields"`
		DetailsRequiredFor []string `json:"details_required_for"`
	} `json:"failure_envelope"`
	GoldenThresholdGates struct {
		Gate                bool    `json:"gate"`
		SummaryPassRateMin  float64 `json:"summary_pass_rate_min"`
		CasesFailedMax      int     `json:"cases_failed_max"`
		FindingsMax         int     `json:"findings_max"`
		RegressionThreshold float64 `json:"regression_threshold"`
		MetricThresholds    []struct {
			Name        string  `json:"name"`
			Type        string  `json:"type"`
			PassRateMin float64 `json:"pass_rate_min"`
			MeanMax     float64 `json:"mean_max"`
		} `json:"metric_thresholds"`
	} `json:"golden_threshold_gates"`
}

type finrobotEvaluationHarnessParityReport struct {
	Status  string `json:"status"`
	Summary struct {
		EvaluateBlocks int     `json:"evaluate_blocks"`
		CasesPassed    int     `json:"cases_passed"`
		CasesFailed    int     `json:"cases_failed"`
		Assertions     int     `json:"assertions"`
		PassRate       float64 `json:"pass_rate"`
	} `json:"summary"`
	LLM *struct {
		Mode          string  `json:"mode"`
		LoadedTurns   int     `json:"loaded_turns"`
		ReplayedTurns int     `json:"replayed_turns"`
		Turns         int     `json:"turns"`
		InputTokens   int64   `json:"input_tokens"`
		OutputTokens  int64   `json:"output_tokens"`
		Cost          float64 `json:"cost"`
		Errors        int     `json:"errors"`
	} `json:"llm"`
	Cases []struct {
		CaseID     string `json:"case_id"`
		Name       string `json:"name"`
		SourcePath string `json:"source_path"`
		Status     string `json:"status"`
		LLM        *struct {
			TraceRef     string  `json:"trace_ref"`
			Turns        int     `json:"turns"`
			InputTokens  int64   `json:"input_tokens"`
			OutputTokens int64   `json:"output_tokens"`
			Cost         float64 `json:"cost"`
			Errors       int     `json:"errors"`
		} `json:"llm"`
		Subcases []struct {
			CaseID  string `json:"case_id"`
			Status  string `json:"status"`
			Metrics []struct {
				Name  string `json:"name"`
				Type  string `json:"type"`
				Value any    `json:"value"`
			} `json:"metrics"`
		} `json:"subcases"`
	} `json:"cases"`
	Metrics []struct {
		Name     string         `json:"name"`
		Type     string         `json:"type"`
		Count    int            `json:"count"`
		PassRate float64        `json:"pass_rate"`
		Mean     float64        `json:"mean"`
		Values   map[string]int `json:"values"`
	} `json:"metrics"`
	Findings []struct {
		Kind     string         `json:"kind"`
		Severity string         `json:"severity"`
		Message  string         `json:"message"`
		Path     string         `json:"path"`
		Details  map[string]any `json:"details"`
	} `json:"findings"`
}

func TestFinRobotEvaluationHarnessParityDeclaresGenericAICapability(t *testing.T) {
	root := repoRoot(t)
	manifest := loadFinRobotEvaluationHarnessParityManifest(t, root)
	capability := manifest.AIEvaluationCapability
	if capability.ID == "" || capability.Scope != "generic-ai-evaluation" || capability.DomainSpecific {
		t.Fatalf("AI evaluation capability is not generic: %#v", capability)
	}
	if capability.NetworkPolicy != "disabled_by_default" || capability.ModelPolicy != "replay_or_stub_only" ||
		capability.TestHardcodingPolicy != "manifest_driven" {
		t.Fatalf("provider-free policies = network %q model %q tests %q", capability.NetworkPolicy, capability.ModelPolicy, capability.TestHardcodingPolicy)
	}
	requireStringSet(t, capability.DialectSurface, []string{
		"evaluate_block", "eval.load_jsonl", "eval.case", "eval.metric", "eval.judge", "eval.usage", "eval.budget", "llm.replay",
	})
	requireStringSet(t, manifest.JudgeSpec.RequestContract.RequiredFields, []string{"model", "messages"})
	requireStringSet(t, manifest.JudgeSpec.ResponseContract.UsageMetrics, []string{"judge_cost", "judge_input_tokens", "judge_output_tokens", "judge_tokens"})
	if manifest.JudgeSpec.RequestContract.DefaultMaxTokens != 200 || manifest.JudgeSpec.ResponseContract.FailureFindingKind != manifest.RecordReplayMatching.UnmatchedRequestFindingKind {
		t.Fatalf("judge spec is not aligned with replay mismatch contract: %#v %#v", manifest.JudgeSpec, manifest.RecordReplayMatching)
	}
	requireStringSet(t, metricRegistryNames(manifest), []string{"dataset_item_valid", "judge_passed", "answer_chars", "rubric_label", "judge_tokens", "judge_cost"})
	if !manifest.GoldenThresholdGates.Gate || manifest.GoldenThresholdGates.SummaryPassRateMin != 1 || manifest.GoldenThresholdGates.CasesFailedMax != 0 {
		t.Fatalf("golden threshold gate = %#v", manifest.GoldenThresholdGates)
	}
	if manifest.ProviderFreeModelStub.LiveProviderCalls || manifest.ProviderFreeModelStub.NetworkCalls || manifest.ProviderFreeModelStub.AllowedModelPrefix != "mock-" {
		t.Fatalf("provider-free model stub = %#v", manifest.ProviderFreeModelStub)
	}
	assertFileSHA256(t, filepath.Join(root, capability.CapabilitySpecimen.SourcePath), capability.CapabilitySpecimen.SourceSHA256)
	assertFileSHA256(t, filepath.Join(root, capability.CapabilitySpecimen.DatasetPath), capability.CapabilitySpecimen.DatasetSHA256)
	assertFileSHA256(t, filepath.Join(root, capability.CapabilitySpecimen.ReplayRecordsPath), capability.CapabilitySpecimen.ReplaySHA256)
	assertRecordFixtureShape(t, filepath.Join(root, capability.CapabilitySpecimen.ReplayRecordsPath), capability.CapabilitySpecimen.TurnCount, capability.CapabilitySpecimen.Models)
	assertDatasetManifestShape(t, filepath.Join(root, capability.CapabilitySpecimen.DatasetPath), manifest.DatasetManifest, capability.CapabilitySpecimen.DatasetRows)
}

func TestFinRobotEvaluationHarnessParityRunsGenericAISpecimenProviderFree(t *testing.T) {
	root := repoRoot(t)
	manifest := loadFinRobotEvaluationHarnessParityManifest(t, root)
	specimen := manifest.AIEvaluationCapability.CapabilitySpecimen
	sourcePath := materializeFinRobotEvaluationHarnessSpecimen(t, root, specimen.SourcePath, specimen.DatasetPath)
	reportPath := filepath.Join(t.TempDir(), "generic-ai-evaluation.report.json")
	report := runFinRobotEvaluationHarnessParityReport(t, root, sourcePath, specimen.ReplayRecordsPath, reportPath, true)
	golden := specimen.GoldenReport
	if report.Status != golden.Status || report.Summary.EvaluateBlocks != golden.EvaluateBlocks ||
		report.Summary.CasesPassed != golden.CasesPassed || report.Summary.CasesFailed != golden.CasesFailed ||
		report.Summary.Assertions != golden.Assertions || report.Summary.PassRate < manifest.GoldenThresholdGates.SummaryPassRateMin {
		t.Fatalf("summary = %#v, golden = %#v", report.Summary, golden)
	}
	if report.LLM == nil || report.LLM.Mode != "replay" || report.LLM.LoadedTurns != golden.LLMLoadedTurns ||
		report.LLM.ReplayedTurns != golden.LLMReplayedTurns || report.LLM.Turns != golden.LLMTurns {
		t.Fatalf("llm summary = %#v, golden = %#v", report.LLM, golden)
	}
	if len(report.Findings) > manifest.GoldenThresholdGates.FindingsMax {
		t.Fatalf("findings = %#v, gate max = %d", report.Findings, manifest.GoldenThresholdGates.FindingsMax)
	}
	var caseNames []string
	var subcaseIDs []string
	for _, c := range report.Cases {
		caseNames = append(caseNames, c.Name)
		if c.LLM == nil || !strings.HasPrefix(c.LLM.TraceRef, "case:") || c.LLM.Turns != 1 {
			t.Fatalf("case %q llm trace = %#v", c.Name, c.LLM)
		}
		for _, subcase := range c.Subcases {
			if subcase.Status != "passed" {
				t.Fatalf("subcase %q status = %q", subcase.CaseID, subcase.Status)
			}
			subcaseIDs = append(subcaseIDs, subcase.CaseID)
		}
	}
	sort.Strings(subcaseIDs)
	wantSubcases := append([]string(nil), specimen.SubcaseIDs...)
	sort.Strings(wantSubcases)
	if strings.Join(caseNames, "\x00") != strings.Join(golden.CaseNames, "\x00") || strings.Join(subcaseIDs, "\x00") != strings.Join(wantSubcases, "\x00") {
		t.Fatalf("case names/subcases = %#v/%#v, want %#v/%#v", caseNames, subcaseIDs, golden.CaseNames, wantSubcases)
	}
	assertGoldenMetrics(t, report, golden.Metrics)
}

func TestFinRobotEvaluationHarnessParityFailureEnvelopeForReplayMismatch(t *testing.T) {
	root := repoRoot(t)
	manifest := loadFinRobotEvaluationHarnessParityManifest(t, root)
	specimen := manifest.AIEvaluationCapability.CapabilitySpecimen
	sourcePath := materializeFinRobotEvaluationHarnessSpecimen(t, root, specimen.SourcePath, specimen.DatasetPath)
	data, err := os.ReadFile(filepath.Join(root, specimen.ReplayRecordsPath))
	if err != nil {
		t.Fatal(err)
	}
	mismatch := bytes.ReplaceAll(data, []byte("mock-ai-eval-judge"), []byte("mock-ai-eval-drift"))
	mismatchPath := filepath.Join(t.TempDir(), "mismatch.records.json")
	if err := os.WriteFile(mismatchPath, mismatch, 0o600); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(t.TempDir(), "mismatch.report.json")
	report := runFinRobotEvaluationHarnessParityReport(t, root, sourcePath, mismatchPath, reportPath, false)
	if report.Status != manifest.FailureEnvelope.StatusOnFailure || report.LLM == nil || report.LLM.Errors == 0 {
		t.Fatalf("failure report status/llm = %q/%#v", report.Status, report.LLM)
	}
	var mismatchFinding *struct {
		Kind     string         `json:"kind"`
		Severity string         `json:"severity"`
		Message  string         `json:"message"`
		Path     string         `json:"path"`
		Details  map[string]any `json:"details"`
	}
	for i := range report.Findings {
		if report.Findings[i].Kind == manifest.RecordReplayMatching.UnmatchedRequestFindingKind {
			mismatchFinding = &report.Findings[i]
			break
		}
	}
	if mismatchFinding == nil {
		t.Fatalf("findings = %#v, want %q", report.Findings, manifest.RecordReplayMatching.UnmatchedRequestFindingKind)
	}
	for _, field := range manifest.FailureEnvelope.RequiredFields {
		if !findingHasField(*mismatchFinding, field) {
			t.Fatalf("mismatch finding missing required field %q: %#v", field, mismatchFinding)
		}
	}
	for _, detail := range manifest.RecordReplayMatching.FailureDetails {
		if _, ok := mismatchFinding.Details[detail]; !ok {
			t.Fatalf("mismatch details missing %q: %#v", detail, mismatchFinding.Details)
		}
	}
}

func loadFinRobotEvaluationHarnessParityManifest(t *testing.T, root string) finrobotEvaluationHarnessParityManifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "examples", "ai", "finrobot_translation", "evaluation_harness", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest finrobotEvaluationHarnessParityManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode parity manifest: %v", err)
	}
	return manifest
}

func runFinRobotEvaluationHarnessParityReport(t *testing.T, root, sourcePath, replayPath, reportPath string, wantSuccess bool) finrobotEvaluationHarnessParityReport {
	t.Helper()
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
	var report finrobotEvaluationHarnessParityReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, string(data))
	}
	return report
}

func materializeFinRobotEvaluationHarnessSpecimen(t *testing.T, root, sourceRel, datasetRel string) string {
	t.Helper()
	dir := t.TempDir()
	sourceData, err := os.ReadFile(filepath.Join(root, sourceRel))
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(dir, "generic_ai_evaluation.leia")
	if err := os.WriteFile(sourcePath, sourceData, 0o600); err != nil {
		t.Fatal(err)
	}
	datasetData, err := os.ReadFile(filepath.Join(root, datasetRel))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.Base(datasetRel)), datasetData, 0o600); err != nil {
		t.Fatal(err)
	}
	return sourcePath
}

func assertDatasetManifestShape(t *testing.T, path string, manifest struct {
	Format         string   `json:"format"`
	IDField        string   `json:"id_field"`
	RequiredFields []string `json:"required_fields"`
	MinimumRows    int      `json:"minimum_rows"`
	RowPolicy      string   `json:"row_policy"`
}, wantRows int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if manifest.Format != "jsonl" || manifest.IDField == "" || manifest.MinimumRows <= 0 {
		t.Fatalf("dataset manifest = %#v", manifest)
	}
	seenIDs := map[string]bool{}
	scanner := bufio.NewScanner(f)
	rows := 0
	for scanner.Scan() {
		rows++
		var row map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatalf("%s row %d: %v", path, rows, err)
		}
		for _, field := range manifest.RequiredFields {
			if _, ok := row[field]; !ok {
				t.Fatalf("%s row %d missing field %q", path, rows, field)
			}
		}
		id, _ := row[manifest.IDField].(string)
		if id == "" || seenIDs[id] {
			t.Fatalf("%s row %d duplicate/empty id %q", path, rows, id)
		}
		seenIDs[id] = true
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if rows != wantRows || rows < manifest.MinimumRows {
		t.Fatalf("%s rows = %d, want %d and >= %d", path, rows, wantRows, manifest.MinimumRows)
	}
}

func assertGoldenMetrics(t *testing.T, report finrobotEvaluationHarnessParityReport, want []struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Count    int     `json:"count"`
	PassRate float64 `json:"pass_rate"`
	Mean     float64 `json:"mean"`
}) {
	t.Helper()
	got := map[string]struct {
		Type     string
		Count    int
		PassRate float64
		Mean     float64
	}{}
	for _, metric := range report.Metrics {
		got[metric.Name] = struct {
			Type     string
			Count    int
			PassRate float64
			Mean     float64
		}{Type: metric.Type, Count: metric.Count, PassRate: metric.PassRate, Mean: metric.Mean}
	}
	for _, metric := range want {
		actual, ok := got[metric.Name]
		if !ok {
			t.Fatalf("metric %q missing from report metrics %#v", metric.Name, got)
		}
		if actual.Type != metric.Type || actual.Count != metric.Count {
			t.Fatalf("metric %q = %#v, want type %q count %d", metric.Name, actual, metric.Type, metric.Count)
		}
		if metric.PassRate != 0 && actual.PassRate != metric.PassRate {
			t.Fatalf("metric %q pass_rate = %v, want %v", metric.Name, actual.PassRate, metric.PassRate)
		}
		if metric.Mean != 0 && actual.Mean != metric.Mean {
			t.Fatalf("metric %q mean = %v, want %v", metric.Name, actual.Mean, metric.Mean)
		}
	}
}

func metricRegistryNames(manifest finrobotEvaluationHarnessParityManifest) []string {
	var names []string
	for _, metric := range manifest.MetricRegistry {
		names = append(names, metric.Name)
	}
	return names
}

func requireStringSet(t *testing.T, got, want []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, item := range got {
		seen[item] = true
	}
	for _, item := range want {
		if !seen[item] {
			t.Fatalf("missing %q from %#v", item, got)
		}
	}
}

func findingHasField(finding struct {
	Kind     string         `json:"kind"`
	Severity string         `json:"severity"`
	Message  string         `json:"message"`
	Path     string         `json:"path"`
	Details  map[string]any `json:"details"`
}, field string) bool {
	switch field {
	case "kind":
		return finding.Kind != ""
	case "severity":
		return finding.Severity != ""
	case "message":
		return finding.Message != ""
	case "path":
		return finding.Path != ""
	case "details":
		return len(finding.Details) > 0
	default:
		return false
	}
}
