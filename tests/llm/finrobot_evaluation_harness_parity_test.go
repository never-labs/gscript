package leia_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type finrobotEvaluationHarnessParityManifest struct {
	HarnessID      string `json:"harness_id"`
	FixtureVersion string `json:"fixture_version"`
	CIReport       struct {
		ProviderFree bool   `json:"provider_free"`
		Command      string `json:"command_template"`
	} `json:"ci_report"`
	AIEvaluationCapability struct {
		ID                   string   `json:"id"`
		Scope                string   `json:"scope"`
		DomainSpecific       bool     `json:"domain_specific"`
		NetworkPolicy        string   `json:"network_policy"`
		ModelPolicy          string   `json:"model_policy"`
		TestHardcodingPolicy string   `json:"test_hardcoding_policy"`
		DialectSurface       []string `json:"dialect_surface"`
		CapabilitySpecimen   struct {
			ID                    string   `json:"id"`
			SourcePath            string   `json:"source_path"`
			DatasetPath           string   `json:"dataset_path"`
			ReplayRecordsPath     string   `json:"replay_records_path"`
			SourceSHA256          string   `json:"source_sha256"`
			DatasetSHA256         string   `json:"dataset_sha256"`
			ReplaySHA256          string   `json:"replay_records_sha256"`
			TurnCount             int      `json:"turn_count"`
			Models                []string `json:"models"`
			DatasetRows           int      `json:"dataset_rows"`
			SubcaseIDs            []string `json:"subcase_ids"`
			SourceMaterialization struct {
				Required              bool   `json:"required"`
				InputExtension        string `json:"input_extension"`
				MaterializedExtension string `json:"materialized_extension"`
				MaterializedFilename  string `json:"materialized_filename"`
				DatasetCopyRequired   bool   `json:"dataset_copy_required"`
				Reason                string `json:"reason"`
			} `json:"source_materialization"`
			GoldenReport struct {
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
	ScoringTrace          finrobotEvaluationScoringTraceManifest `json:"scoring_trace"`
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

type finrobotEvaluationScoringTraceManifest struct {
	ReportFields  []string `json:"report_fields"`
	CaseFields    []string `json:"case_fields"`
	LLMCaseFields []string `json:"llm_case_fields"`
	SubcaseFields []string `json:"subcase_fields"`
}

type finrobotEvaluationHarnessParityReport struct {
	raw     map[string]json.RawMessage
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
		ReplayPath    string  `json:"replay_path"`
		LoadedTurns   int     `json:"loaded_turns"`
		ReplayedTurns int     `json:"replayed_turns"`
		Turns         int     `json:"turns"`
		InputTokens   int64   `json:"input_tokens"`
		OutputTokens  int64   `json:"output_tokens"`
		Cost          float64 `json:"cost"`
		Errors        int     `json:"errors"`
	} `json:"llm"`
	Inputs []struct {
		Path   string `json:"path"`
		Status string `json:"status"`
	} `json:"inputs"`
	Cases []struct {
		CaseID     string `json:"case_id"`
		Name       string `json:"name"`
		SourcePath string `json:"source_path"`
		Status     string `json:"status"`
		Assertions []any  `json:"assertions"`
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
	assertGoldenThresholdGateMetrics(t, manifest)
	if manifest.ProviderFreeModelStub.LiveProviderCalls || manifest.ProviderFreeModelStub.NetworkCalls || manifest.ProviderFreeModelStub.AllowedModelPrefix != "mock-" {
		t.Fatalf("provider-free model stub = %#v", manifest.ProviderFreeModelStub)
	}
	if !manifest.CIReport.ProviderFree || manifest.HarnessID == "" || manifest.FixtureVersion == "" ||
		!strings.Contains(manifest.CIReport.Command, "--replay") ||
		!strings.Contains(manifest.CIReport.Command, "${MATERIALIZED_SOURCE}") ||
		strings.Contains(manifest.CIReport.Command, " ${SOURCE}") {
		t.Fatalf("offline CI report/fixture marker = harness %q fixture %q ci %#v", manifest.HarnessID, manifest.FixtureVersion, manifest.CIReport)
	}
	materialization := capability.CapabilitySpecimen.SourceMaterialization
	if !materialization.Required ||
		materialization.InputExtension != ".source.txt" ||
		materialization.MaterializedExtension != ".leia" ||
		materialization.MaterializedFilename == "" ||
		!materialization.DatasetCopyRequired ||
		!strings.HasSuffix(capability.CapabilitySpecimen.SourcePath, materialization.InputExtension) ||
		!strings.HasSuffix(materialization.MaterializedFilename, materialization.MaterializedExtension) ||
		!strings.Contains(strings.ToLower(materialization.Reason), "materialized") {
		t.Fatalf("source materialization contract incomplete: %#v", materialization)
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
	assertFinRobotEvaluationScoringTraceCoverage(t, report, manifest.ScoringTrace)
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
	assertGoldenMetricsHaveSubcaseEvidence(t, report, golden.Metrics)
	assertReportMetricEvidenceTraceableToManifest(t, report, manifest)
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
	assertReplayMismatchEnvelopeEvidence(t, report, manifest, *mismatchFinding, sourcePath, mismatchPath)
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
	if err := json.Unmarshal(data, &report.raw); err != nil {
		t.Fatalf("decode raw report: %v\n%s", err, string(data))
	}
	return report
}

func assertFinRobotEvaluationScoringTraceCoverage(t *testing.T, report finrobotEvaluationHarnessParityReport, scoring finrobotEvaluationScoringTraceManifest) {
	t.Helper()
	requireJSONFields(t, "report", report.raw, scoring.ReportFields)
	var rawCases []map[string]json.RawMessage
	if err := json.Unmarshal(report.raw["cases"], &rawCases); err != nil {
		t.Fatalf("decode raw cases: %v", err)
	}
	if len(rawCases) != len(report.Cases) {
		t.Fatalf("raw cases = %d, typed cases = %d", len(rawCases), len(report.Cases))
	}
	for i, rawCase := range rawCases {
		casePath := "cases[" + strconv.Itoa(i) + "]"
		requireJSONFields(t, casePath, rawCase, scoring.CaseFields)
		if report.Cases[i].LLM == nil {
			t.Fatalf("%s missing typed llm trace", casePath)
		}
		var rawLLM map[string]json.RawMessage
		if err := json.Unmarshal(rawCase["llm"], &rawLLM); err != nil {
			t.Fatalf("decode %s.llm: %v", casePath, err)
		}
		requireJSONFields(t, casePath+".llm", rawLLM, scoring.LLMCaseFields)
		var rawSubcases []map[string]json.RawMessage
		if err := json.Unmarshal(rawCase["subcases"], &rawSubcases); err != nil {
			t.Fatalf("decode %s.subcases: %v", casePath, err)
		}
		if len(rawSubcases) != len(report.Cases[i].Subcases) {
			t.Fatalf("%s raw subcases = %d, typed subcases = %d", casePath, len(rawSubcases), len(report.Cases[i].Subcases))
		}
		for j, rawSubcase := range rawSubcases {
			requireJSONFields(t, casePath+".subcases["+strconv.Itoa(j)+"]", rawSubcase, scoring.SubcaseFields)
		}
	}
}

func requireJSONFields(t *testing.T, path string, object map[string]json.RawMessage, fields []string) {
	t.Helper()
	for _, field := range fields {
		value, ok := object[field]
		if !ok {
			t.Fatalf("%s missing manifest-declared field %q in %#v", path, field, sortedJSONFieldNames(object))
		}
		if bytes.Equal(value, []byte("null")) {
			t.Fatalf("%s field %q is null", path, field)
		}
	}
}

func sortedJSONFieldNames(object map[string]json.RawMessage) []string {
	fields := make([]string, 0, len(object))
	for field := range object {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
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

func assertGoldenMetricsHaveSubcaseEvidence(t *testing.T, report finrobotEvaluationHarnessParityReport, want []struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Count    int     `json:"count"`
	PassRate float64 `json:"pass_rate"`
	Mean     float64 `json:"mean"`
}) {
	t.Helper()
	type metricEvidence struct {
		Type  string
		Count int
	}
	evidence := map[string]metricEvidence{}
	for _, c := range report.Cases {
		for _, subcase := range c.Subcases {
			for _, metric := range subcase.Metrics {
				item := evidence[metric.Name]
				if item.Type != "" && item.Type != metric.Type {
					t.Fatalf("subcase metric %q has mixed types %q and %q", metric.Name, item.Type, metric.Type)
				}
				item.Type = metric.Type
				item.Count++
				evidence[metric.Name] = item
			}
		}
	}
	for _, metric := range want {
		actual, ok := evidence[metric.Name]
		if !ok {
			t.Fatalf("golden metric %q has no subcase evidence in %#v", metric.Name, evidence)
		}
		if actual.Type != metric.Type || actual.Count != metric.Count {
			t.Fatalf("golden metric %q subcase evidence = %#v, want type %q count %d", metric.Name, actual, metric.Type, metric.Count)
		}
	}
}

func assertReportMetricEvidenceTraceableToManifest(t *testing.T, report finrobotEvaluationHarnessParityReport, manifest finrobotEvaluationHarnessParityManifest) {
	t.Helper()
	registry := map[string]struct {
		Type        string
		Aggregation string
		GoldenMin   float64
		GoldenMax   float64
	}{}
	for _, metric := range manifest.MetricRegistry {
		registry[metric.Name] = struct {
			Type        string
			Aggregation string
			GoldenMin   float64
			GoldenMax   float64
		}{Type: metric.Type, Aggregation: metric.Aggregation, GoldenMin: metric.GoldenMin, GoldenMax: metric.GoldenMax}
	}
	wantCaseNames := finrobotEvaluationHarnessParityStringSet(manifest.AIEvaluationCapability.CapabilitySpecimen.GoldenReport.CaseNames)
	wantSubcases := finrobotEvaluationHarnessParityStringSet(manifest.AIEvaluationCapability.CapabilitySpecimen.SubcaseIDs)
	evidence := map[string]struct {
		Type     string
		Count    int
		CaseIDs  map[string]bool
		Subcases map[string]bool
	}{}
	for _, c := range report.Cases {
		if !wantCaseNames[c.Name] || c.CaseID == "" {
			t.Fatalf("report case %q/%q is not manifest-declared; want names %#v", c.CaseID, c.Name, manifest.AIEvaluationCapability.CapabilitySpecimen.GoldenReport.CaseNames)
		}
		for _, subcase := range c.Subcases {
			if !wantSubcases[subcase.CaseID] {
				t.Fatalf("report subcase %q is not manifest-declared; want %#v", subcase.CaseID, manifest.AIEvaluationCapability.CapabilitySpecimen.SubcaseIDs)
			}
			for _, metric := range subcase.Metrics {
				registered, ok := registry[metric.Name]
				if !ok {
					t.Fatalf("subcase %q metric %q is not in manifest metric_registry", subcase.CaseID, metric.Name)
				}
				if registered.Type != metric.Type {
					t.Fatalf("subcase %q metric %q type = %q, registry = %q", subcase.CaseID, metric.Name, metric.Type, registered.Type)
				}
				item := evidence[metric.Name]
				if item.CaseIDs == nil {
					item.CaseIDs = map[string]bool{}
					item.Subcases = map[string]bool{}
				}
				item.Type = metric.Type
				item.Count++
				item.CaseIDs[c.CaseID] = true
				item.Subcases[subcase.CaseID] = true
				evidence[metric.Name] = item
			}
		}
	}
	for _, metric := range report.Metrics {
		registered, ok := registry[metric.Name]
		if !ok {
			t.Fatalf("report metric %q is not in manifest metric_registry", metric.Name)
		}
		actual, ok := evidence[metric.Name]
		if !ok {
			t.Fatalf("report metric %q has no case/subcase evidence", metric.Name)
		}
		if metric.Type != registered.Type || metric.Type != actual.Type || metric.Count != actual.Count {
			t.Fatalf("report metric %q = type %q count %d, registry type %q, evidence %#v", metric.Name, metric.Type, metric.Count, registered.Type, actual)
		}
		if len(actual.CaseIDs) == 0 || len(actual.Subcases) == 0 {
			t.Fatalf("report metric %q evidence lacks case/subcase ids: %#v", metric.Name, actual)
		}
	}
	reportMetrics := reportMetricsByName(report)
	for _, gate := range manifest.GoldenThresholdGates.MetricThresholds {
		metric, ok := reportMetrics[gate.Name]
		if !ok {
			t.Fatalf("gate metric %q has no report metric evidence", gate.Name)
		}
		if _, ok := evidence[gate.Name]; !ok {
			t.Fatalf("gate metric %q has no subcase evidence", gate.Name)
		}
		if gate.PassRateMin != 0 && metric.PassRate < gate.PassRateMin {
			t.Fatalf("gate metric %q pass_rate = %v, min %v", gate.Name, metric.PassRate, gate.PassRateMin)
		}
		if gate.MeanMax != 0 && metric.Mean > gate.MeanMax {
			t.Fatalf("gate metric %q mean = %v, max %v", gate.Name, metric.Mean, gate.MeanMax)
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

func assertGoldenThresholdGateMetrics(t *testing.T, manifest finrobotEvaluationHarnessParityManifest) {
	t.Helper()
	registryTypes := map[string]string{}
	for _, metric := range manifest.MetricRegistry {
		if metric.Name == "" || metric.Type == "" {
			t.Fatalf("metric registry entry is incomplete: %#v", metric)
		}
		registryTypes[metric.Name] = metric.Type
	}
	goldenTypes := map[string]string{}
	for _, metric := range manifest.AIEvaluationCapability.CapabilitySpecimen.GoldenReport.Metrics {
		if metric.Name == "" || metric.Type == "" {
			t.Fatalf("golden metric entry is incomplete: %#v", metric)
		}
		goldenTypes[metric.Name] = metric.Type
	}
	for _, gate := range manifest.GoldenThresholdGates.MetricThresholds {
		registryType, ok := registryTypes[gate.Name]
		if !ok {
			t.Fatalf("golden threshold metric %q is not registered", gate.Name)
		}
		goldenType, ok := goldenTypes[gate.Name]
		if !ok {
			t.Fatalf("golden threshold metric %q is missing from golden report metrics", gate.Name)
		}
		if gate.Type == "" || gate.Type != registryType || gate.Type != goldenType {
			t.Fatalf("golden threshold metric %q type = %q, registry = %q, golden = %q", gate.Name, gate.Type, registryType, goldenType)
		}
		if gate.PassRateMin == 0 && gate.MeanMax == 0 {
			t.Fatalf("golden threshold metric %q has no threshold: %#v", gate.Name, gate)
		}
	}
}

func assertReplayMismatchEnvelopeEvidence(t *testing.T, report finrobotEvaluationHarnessParityReport, manifest finrobotEvaluationHarnessParityManifest, finding struct {
	Kind     string         `json:"kind"`
	Severity string         `json:"severity"`
	Message  string         `json:"message"`
	Path     string         `json:"path"`
	Details  map[string]any `json:"details"`
}, sourcePath, replayPath string) {
	t.Helper()
	if report.LLM == nil || report.LLM.Mode != "replay" || report.LLM.ReplayPath != replayPath {
		t.Fatalf("mismatch envelope replay association = %#v, want replay path %q", report.LLM, replayPath)
	}
	if !manifest.CIReport.ProviderFree ||
		manifest.AIEvaluationCapability.NetworkPolicy != "disabled_by_default" ||
		manifest.AIEvaluationCapability.ModelPolicy != "replay_or_stub_only" ||
		manifest.ProviderFreeModelStub.LiveProviderCalls || manifest.ProviderFreeModelStub.NetworkCalls {
		t.Fatalf("mismatch envelope is not anchored to provider-free offline manifest policy: ci %#v capability %#v stub %#v", manifest.CIReport, manifest.AIEvaluationCapability, manifest.ProviderFreeModelStub)
	}
	if len(report.Inputs) != 1 || report.Inputs[0].Path != sourcePath || report.Inputs[0].Status != "error" {
		t.Fatalf("mismatch envelope source fixture association = %#v, want %q error", report.Inputs, sourcePath)
	}
	caseID, _ := finding.Details["case_id"].(string)
	if caseID == "" {
		t.Fatalf("mismatch envelope missing case_id detail: %#v", finding.Details)
	}
	if _, ok := finding.Details["turn"]; !ok {
		t.Fatalf("mismatch envelope missing replay turn detail: %#v", finding.Details)
	}
	if _, ok := finding.Details["expected"]; !ok {
		t.Fatalf("mismatch envelope missing replay expected detail: %#v", finding.Details)
	}
	if _, ok := finding.Details["actual"]; !ok {
		t.Fatalf("mismatch envelope missing replay actual detail: %#v", finding.Details)
	}
	wantSubcases := finrobotEvaluationHarnessParityStringSet(manifest.AIEvaluationCapability.CapabilitySpecimen.SubcaseIDs)
	for _, c := range report.Cases {
		if c.CaseID != caseID {
			continue
		}
		if c.SourcePath != sourcePath || finding.Path != sourcePath {
			t.Fatalf("mismatch case/source path association = case %q finding %q, want %q", c.SourcePath, finding.Path, sourcePath)
		}
		for _, subcase := range c.Subcases {
			if subcase.Status == "failed" {
				if !wantSubcases[subcase.CaseID] {
					t.Fatalf("failed subcase %q is not manifest-declared", subcase.CaseID)
				}
				return
			}
		}
		t.Fatalf("mismatch case %q has no failed manifest subcase: %#v", caseID, c.Subcases)
	}
	t.Fatalf("mismatch finding case_id %q has no report case", caseID)
}

func reportMetricsByName(report finrobotEvaluationHarnessParityReport) map[string]struct {
	Name     string
	Type     string
	Count    int
	PassRate float64
	Mean     float64
	Values   map[string]int
} {
	metrics := map[string]struct {
		Name     string
		Type     string
		Count    int
		PassRate float64
		Mean     float64
		Values   map[string]int
	}{}
	for _, metric := range report.Metrics {
		metrics[metric.Name] = struct {
			Name     string
			Type     string
			Count    int
			PassRate float64
			Mean     float64
			Values   map[string]int
		}{
			Name:     metric.Name,
			Type:     metric.Type,
			Count:    metric.Count,
			PassRate: metric.PassRate,
			Mean:     metric.Mean,
			Values:   metric.Values,
		}
	}
	return metrics
}

func finrobotEvaluationHarnessParityStringSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return set
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
