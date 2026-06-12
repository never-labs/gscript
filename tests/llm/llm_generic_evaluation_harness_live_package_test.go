package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type genericEvaluationHarnessManifest struct {
	SchemaVersion               int      `json:"schema_version"`
	ID                          string   `json:"id"`
	PackageName                 string   `json:"package_name"`
	ProviderFree                bool     `json:"provider_free"`
	LiveNetworkDefault          bool     `json:"live_network_default"`
	RealDependencyImportDefault bool     `json:"real_dependency_import_default"`
	SourceModules               []string `json:"source_modules"`
	Credentials                 struct {
		Required          []string `json:"required"`
		Optional          []string `json:"optional"`
		SecretEnvPatterns []string `json:"secret_env_patterns"`
		Policy            string   `json:"policy"`
	} `json:"credentials"`
	DefaultPolicy struct {
		Mode                        string `json:"mode"`
		LiveNetwork                 bool   `json:"live_network"`
		ProviderCredentialsRequired bool   `json:"provider_credentials_required"`
		RealDependencyImports       bool   `json:"real_dependency_imports"`
		CleanSkipWithoutDependency  bool   `json:"clean_skip_without_dependency"`
		FixtureHook                 string `json:"fixture_hook"`
	} `json:"default_policy"`
	Entrypoints        map[string]string              `json:"entrypoints"`
	Schemas            map[string]string              `json:"schemas"`
	Fixtures           map[string]string              `json:"fixtures"`
	Capabilities       []string                       `json:"capabilities"`
	DialectSurface     []string                       `json:"dialect_surface"`
	ArtifactBoundaries []genericEvaluationBoundary    `json:"artifact_boundaries"`
	GoldenGate         genericEvaluationGoldenGateRef `json:"golden_gate"`
	TestGates          []string                       `json:"test_gates"`
	NoBuiltIn          struct {
		Required  bool   `json:"required"`
		Statement string `json:"statement"`
	} `json:"no_built_in_guarantee"`
}

type genericEvaluationBoundary struct {
	ID           string `json:"id"`
	Capability   string `json:"capability"`
	Schema       string `json:"schema"`
	Fixture      string `json:"fixture"`
	ProviderFree bool   `json:"provider_free"`
	LiveNetwork  bool   `json:"live_network"`
}

type genericEvaluationGoldenGateRef struct {
	Fixture              string  `json:"fixture"`
	Gate                 bool    `json:"gate"`
	SummaryPassRateMin   float64 `json:"summary_pass_rate_min"`
	CasesFailedMax       int     `json:"cases_failed_max"`
	FindingsMax          int     `json:"findings_max"`
	JudgeReplayErrorsMax int     `json:"judge_replay_errors_max"`
}

func TestFinRobotGenericEvaluationHarnessManifest(t *testing.T) {
	base := genericEvaluationHarnessLivePackageDir(t)
	manifest := loadGenericEvaluationHarnessManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "generic-ai-evaluation-harness-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-generic-ai-evaluation-harness" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("generic evaluation harness must not declare credentials: %#v", manifest.Credentials)
	}
	if !strings.Contains(manifest.Credentials.Policy, "replay records") || !strings.Contains(manifest.Credentials.Policy, "mock stubs") {
		t.Fatalf("credential policy should name provider-free judge boundaries: %q", manifest.Credentials.Policy)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_generic_evaluation_harness_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}
	for _, want := range []string{"generic.ai.evaluation.harness", "ai.eval.run"} {
		if !contains(manifest.DialectSurface, want) {
			t.Fatalf("manifest missing dialect surface/source %q: sources=%#v surface=%#v", want, manifest.SourceModules, manifest.DialectSurface)
		}
	}
	if !reflect.DeepEqual(manifest.SourceModules, []string{"main.leia"}) {
		t.Fatalf("source modules must be file paths only: %#v", manifest.SourceModules)
	}
	for _, want := range []string{
		"generic.ai.evaluation.harness.dataset_manifest",
		"generic.ai.evaluation.harness.case_inputs",
		"generic.ai.evaluation.harness.metric_specs",
		"generic.ai.evaluation.harness.judge_replay_records",
		"generic.ai.evaluation.harness.findings",
		"generic.ai.evaluation.harness.golden_gates",
		"generic.ai.evaluation.harness.case_result_summary",
		"generic.ai.evaluation.harness.agent_run_projection",
		"ai.eval.run.provider_free",
		"ai.eval.run.fixture_replay",
		"ai.eval.run.golden_gate",
	} {
		if !contains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}
	for _, key := range []string{"smoke", "evaluation_harness_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
		assertGenericEvaluationJSONOrLeiaFile(t, filepath.Join(base, manifest.Entrypoints[key]))
	}
	for _, key := range []string{"dataset_manifest", "case_inputs", "metric_specs", "judge_replay_records", "findings", "golden_gates", "case_result_summary", "agent_run_evaluation_projection"} {
		if manifest.Schemas[key] == "" || manifest.Fixtures[key] == "" {
			t.Fatalf("missing schema/fixture for %q: schemas=%#v fixtures=%#v", key, manifest.Schemas, manifest.Fixtures)
		}
		assertGenericEvaluationJSONFile(t, filepath.Join(base, manifest.Schemas[key]))
		assertGenericEvaluationJSONFile(t, filepath.Join(base, manifest.Fixtures[key]))
	}
	if len(manifest.ArtifactBoundaries) != len(manifest.Schemas) {
		t.Fatalf("artifact boundaries = %#v", manifest.ArtifactBoundaries)
	}
	boundaryIDs := map[string]bool{}
	for _, boundary := range manifest.ArtifactBoundaries {
		if boundary.ID == "" || boundary.Capability == "" || boundary.Schema == "" || boundary.Fixture == "" {
			t.Fatalf("artifact boundary incomplete: %#v", boundary)
		}
		if manifest.Schemas[boundary.Schema] == "" || manifest.Fixtures[boundary.Fixture] == "" {
			t.Fatalf("artifact boundary does not resolve through manifest schema/fixture maps: %#v", boundary)
		}
		if !boundary.ProviderFree || boundary.LiveNetwork {
			t.Fatalf("artifact boundary must be provider-free/offline: %#v", boundary)
		}
		boundaryIDs[boundary.ID] = true
	}
	for _, want := range []string{"dataset_manifest", "case_inputs", "metric_specs", "judge_replay_records", "findings", "golden_gates", "case_result_summary", "agent_run_evaluation_projection"} {
		if !boundaryIDs[want] {
			t.Fatalf("artifact boundaries missing %q: %#v", want, manifest.ArtifactBoundaries)
		}
	}
	if !manifest.GoldenGate.Gate || manifest.GoldenGate.SummaryPassRateMin != 1 ||
		manifest.GoldenGate.CasesFailedMax != 0 || manifest.GoldenGate.FindingsMax != 0 ||
		manifest.GoldenGate.JudgeReplayErrorsMax != 0 {
		t.Fatalf("golden gate = %#v", manifest.GoldenGate)
	}
	if !manifest.NoBuiltIn.Required || !strings.Contains(manifest.NoBuiltIn.Statement, manifest.PackageName) {
		t.Fatalf("no-built-in guarantee missing package name: %#v", manifest.NoBuiltIn)
	}
	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"generic.ai.evaluation.harness", "ai.eval.run", "dataset manifest", "case inputs", "metric specs", "judge replay records", "findings", "golden gates", "case result summary", "agent-run projection", "provider-free"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotGenericEvaluationHarnessContractAndFixtures(t *testing.T) {
	base := genericEvaluationHarnessLivePackageDir(t)
	manifest := loadGenericEvaluationHarnessManifest(t, base)

	var contract struct {
		ProviderFree          bool     `json:"provider_free"`
		LiveNetwork           bool     `json:"live_network"`
		RealDependencyImports bool     `json:"real_dependency_imports"`
		RequiresCredentials   bool     `json:"requires_credentials"`
		DialectSurface        []string `json:"dialect_surface"`
		TypedFixtures         []struct {
			ID             string   `json:"id"`
			Schema         string   `json:"schema"`
			Fixture        string   `json:"fixture"`
			RequiredFields []string `json:"required_fields"`
		} `json:"typed_fixtures"`
		DatasetManifestContract struct {
			Format             string   `json:"format"`
			IDField            string   `json:"id_field"`
			MinimumCases       int      `json:"minimum_cases"`
			RequiredCaseFields []string `json:"required_case_fields"`
		} `json:"dataset_manifest_contract"`
		JudgeReplayContract struct {
			Mode                        string   `json:"mode"`
			AllowedModelPrefix          string   `json:"allowed_model_prefix"`
			LiveProviderCalls           bool     `json:"live_provider_calls"`
			NetworkCalls                bool     `json:"network_calls"`
			MatchedRequestFields        []string `json:"matched_request_fields"`
			UnmatchedRequestFindingKind string   `json:"unmatched_request_finding_kind"`
			ExhaustedReplayFindingKind  string   `json:"exhausted_replay_finding_kind"`
			UnconsumedReplayFindingKind string   `json:"unconsumed_replay_finding_kind"`
		} `json:"judge_replay_contract"`
		GoldenGateContract genericEvaluationGoldenGateRef `json:"golden_gate_contract"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(base, "contracts", "evaluation_harness_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || contract.RequiresCredentials {
		t.Fatalf("contract header = %#v", contract)
	}
	if !contains(contract.DialectSurface, "generic.ai.evaluation.harness") || !contains(contract.DialectSurface, "ai.eval.run") {
		t.Fatalf("contract dialect surface = %#v", contract.DialectSurface)
	}
	if len(contract.TypedFixtures) != len(manifest.Schemas) {
		t.Fatalf("typed fixtures = %d, want %d", len(contract.TypedFixtures), len(manifest.Schemas))
	}
	for _, fixture := range contract.TypedFixtures {
		assertGenericEvaluationJSONFile(t, filepath.Join(base, fixture.Schema))
		assertGenericEvaluationJSONFile(t, filepath.Join(base, fixture.Fixture))
		if len(fixture.RequiredFields) == 0 {
			t.Fatalf("typed fixture %q has no required fields", fixture.ID)
		}
	}
	if contract.DatasetManifestContract.Format != "json" ||
		contract.DatasetManifestContract.IDField != "case_id" ||
		contract.DatasetManifestContract.MinimumCases != 2 ||
		!contains(contract.DatasetManifestContract.RequiredCaseFields, "rubric") {
		t.Fatalf("dataset manifest contract = %#v", contract.DatasetManifestContract)
	}
	if contract.JudgeReplayContract.Mode != "replay" ||
		contract.JudgeReplayContract.AllowedModelPrefix != "mock-" ||
		contract.JudgeReplayContract.LiveProviderCalls ||
		contract.JudgeReplayContract.NetworkCalls ||
		!contains(contract.JudgeReplayContract.MatchedRequestFields, "model") ||
		!contains(contract.JudgeReplayContract.MatchedRequestFields, "messages") ||
		contract.JudgeReplayContract.UnmatchedRequestFindingKind == "" ||
		contract.JudgeReplayContract.ExhaustedReplayFindingKind == "" ||
		contract.JudgeReplayContract.UnconsumedReplayFindingKind == "" {
		t.Fatalf("judge replay contract = %#v", contract.JudgeReplayContract)
	}
	if !contract.GoldenGateContract.Gate || contract.GoldenGateContract.SummaryPassRateMin != 1 ||
		contract.GoldenGateContract.CasesFailedMax != 0 || contract.GoldenGateContract.FindingsMax != 0 ||
		contract.GoldenGateContract.JudgeReplayErrorsMax != 0 {
		t.Fatalf("golden gate contract = %#v", contract.GoldenGateContract)
	}
}

func TestFinRobotGenericEvaluationHarnessDatasetReplayAndSummary(t *testing.T) {
	base := genericEvaluationHarnessLivePackageDir(t)

	var dataset struct {
		ProviderFree      bool     `json:"provider_free"`
		LiveNetwork       bool     `json:"live_network"`
		DatasetID         string   `json:"dataset_id"`
		Format            string   `json:"format"`
		IDField           string   `json:"id_field"`
		CaseCount         int      `json:"case_count"`
		RequiredFields    []string `json:"required_fields"`
		CaseInputFixture  string   `json:"case_input_fixture"`
		MetricSpecFixture string   `json:"metric_spec_fixture"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(base, "fixtures", "dataset_manifest_fixture.json"), &dataset)
	if !dataset.ProviderFree || dataset.LiveNetwork || dataset.Format != "json" || dataset.IDField != "case_id" || dataset.CaseCount != 2 {
		t.Fatalf("dataset manifest = %#v", dataset)
	}
	if dataset.DatasetID == "" ||
		dataset.CaseInputFixture != "fixtures/case_inputs_fixture.json" ||
		dataset.MetricSpecFixture != "fixtures/metric_specs_fixture.json" {
		t.Fatalf("dataset manifest must link checked-in case inputs and metric specs: %#v", dataset)
	}
	for _, field := range []string{"case_id", "input", "expected", "rubric"} {
		if !contains(dataset.RequiredFields, field) {
			t.Fatalf("dataset required fields missing %q: %#v", field, dataset.RequiredFields)
		}
	}

	var cases struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		DatasetID    string `json:"dataset_id"`
		Cases        []struct {
			CaseID   string `json:"case_id"`
			Name     string `json:"name"`
			Input    any    `json:"input"`
			Expected any    `json:"expected"`
			Rubric   struct {
				JudgeModel string   `json:"judge_model"`
				PassLabel  string   `json:"pass_label"`
				Criteria   []string `json:"criteria"`
			} `json:"rubric"`
		} `json:"cases"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(base, "fixtures", "case_inputs_fixture.json"), &cases)
	if !cases.ProviderFree || cases.LiveNetwork || cases.DatasetID != dataset.DatasetID || len(cases.Cases) != dataset.CaseCount {
		t.Fatalf("case inputs = %#v", cases)
	}
	caseIDs := map[string]bool{}
	caseNames := map[string]string{}
	caseJudgeModels := map[string]string{}
	for _, c := range cases.Cases {
		if c.CaseID == "" || c.Name == "" || c.Input == nil || c.Expected == nil || c.Rubric.JudgeModel != "mock-ai-eval-judge" || c.Rubric.PassLabel != "pass" || len(c.Rubric.Criteria) == 0 {
			t.Fatalf("case input incomplete: %#v", c)
		}
		if caseIDs[c.CaseID] {
			t.Fatalf("duplicate case_id %q", c.CaseID)
		}
		caseIDs[c.CaseID] = true
		caseNames[c.CaseID] = c.Name
		caseJudgeModels[c.CaseID] = c.Rubric.JudgeModel
	}

	var metrics struct {
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
		Metrics      []struct {
			MetricID    string         `json:"metric_id"`
			Type        string         `json:"type"`
			Aggregation string         `json:"aggregation"`
			Gate        map[string]any `json:"gate"`
		} `json:"metrics"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(base, "fixtures", "metric_specs_fixture.json"), &metrics)
	if !metrics.ProviderFree || metrics.LiveNetwork {
		t.Fatalf("metric specs must be provider-free/offline: %#v", metrics)
	}
	gotMetricTypes := map[string]string{}
	metricGates := map[string]map[string]any{}
	for _, metric := range metrics.Metrics {
		if metric.MetricID == "" || metric.Type == "" || metric.Aggregation == "" || len(metric.Gate) == 0 {
			t.Fatalf("metric spec incomplete: %#v", metric)
		}
		if gotMetricTypes[metric.MetricID] != "" {
			t.Fatalf("duplicate metric_id %q", metric.MetricID)
		}
		gotMetricTypes[metric.MetricID] = metric.Type
		metricGates[metric.MetricID] = metric.Gate
	}
	for _, want := range []string{"dataset_item_valid", "judge_passed", "answer_chars", "rubric_label", "judge_tokens"} {
		if gotMetricTypes[want] == "" {
			t.Fatalf("metric spec missing %q: %#v", want, metrics.Metrics)
		}
	}

	var replay struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		Mode         string `json:"mode"`
		Records      []struct {
			TraceID string `json:"trace_id"`
			CaseID  string `json:"case_id"`
			Request struct {
				Model    string `json:"model"`
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages"`
			} `json:"request"`
			Response struct {
				Label string  `json:"label"`
				Score float64 `json:"score"`
			} `json:"response"`
			Usage struct {
				JudgeTokens int     `json:"judge_tokens"`
				JudgeCost   float64 `json:"judge_cost"`
			} `json:"usage"`
		} `json:"records"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(base, "fixtures", "judge_replay_records_fixture.json"), &replay)
	if !replay.ProviderFree || replay.LiveNetwork || replay.Mode != "replay" || len(replay.Records) != dataset.CaseCount {
		t.Fatalf("judge replay records = %#v", replay)
	}
	traceIDs := map[string]bool{}
	for _, record := range replay.Records {
		if !caseIDs[record.CaseID] || record.TraceID == "" || !strings.HasPrefix(record.Request.Model, "mock-") ||
			len(record.Request.Messages) != 2 || record.Response.Label != "pass" || record.Response.Score != 1 ||
			record.Usage.JudgeTokens <= 0 || record.Usage.JudgeCost != 0 {
			t.Fatalf("judge replay record incomplete/offline violation: %#v", record)
		}
		if record.Request.Model != caseJudgeModels[record.CaseID] {
			t.Fatalf("judge replay model for case %s = %q, want rubric model %q", record.CaseID, record.Request.Model, caseJudgeModels[record.CaseID])
		}
		if !strings.Contains(record.Request.Messages[1].Content, record.CaseID) {
			t.Fatalf("judge replay request for case %s does not include case_id: %#v", record.CaseID, record.Request.Messages)
		}
		if traceIDs[record.TraceID] {
			t.Fatalf("duplicate judge replay trace_id %q", record.TraceID)
		}
		traceIDs[record.TraceID] = true
	}

	var findings struct {
		ProviderFree bool     `json:"provider_free"`
		LiveNetwork  bool     `json:"live_network"`
		FindingKinds []string `json:"finding_kinds"`
		Findings     []struct {
			Kind    string `json:"kind"`
			CaseID  string `json:"case_id"`
			Metric  string `json:"metric_id"`
			TraceID string `json:"trace_id"`
		} `json:"findings"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(base, "fixtures", "findings_fixture.json"), &findings)
	if !findings.ProviderFree || findings.LiveNetwork {
		t.Fatalf("findings must be provider-free/offline: %#v", findings)
	}
	for _, want := range []string{"dataset_manifest_invalid", "case_input_invalid", "metric_gate_failed", "judge_replay_request_mismatch", "judge_replay_exhausted", "judge_replay_unconsumed"} {
		if !contains(findings.FindingKinds, want) {
			t.Fatalf("findings kinds missing %q: %#v", want, findings.FindingKinds)
		}
	}
	for _, finding := range findings.Findings {
		if !contains(findings.FindingKinds, finding.Kind) ||
			(finding.CaseID != "" && !caseIDs[finding.CaseID]) ||
			(finding.Metric != "" && gotMetricTypes[finding.Metric] == "") ||
			(finding.TraceID != "" && !traceIDs[finding.TraceID]) {
			t.Fatalf("finding does not resolve through case/metric/replay records: %#v", finding)
		}
	}

	var golden struct {
		ProviderFree         bool    `json:"provider_free"`
		LiveNetwork          bool    `json:"live_network"`
		Gate                 bool    `json:"gate"`
		SummaryPassRateMin   float64 `json:"summary_pass_rate_min"`
		CasesFailedMax       int     `json:"cases_failed_max"`
		FindingsMax          int     `json:"findings_max"`
		JudgeReplayErrorsMax int     `json:"judge_replay_errors_max"`
		MetricThresholds     []struct {
			MetricID string `json:"metric_id"`
			Type     string `json:"type"`
		} `json:"metric_thresholds"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(base, "fixtures", "golden_gates_fixture.json"), &golden)
	if !golden.ProviderFree || golden.LiveNetwork || !golden.Gate ||
		golden.SummaryPassRateMin != 1 || golden.CasesFailedMax != 0 ||
		golden.FindingsMax != 0 || golden.JudgeReplayErrorsMax != 0 {
		t.Fatalf("golden gates = %#v", golden)
	}
	goldenMetricIDs := map[string]bool{}
	for _, threshold := range golden.MetricThresholds {
		if gotMetricTypes[threshold.MetricID] != threshold.Type || len(metricGates[threshold.MetricID]) == 0 {
			t.Fatalf("golden gate threshold does not resolve through metric specs: %#v", threshold)
		}
		goldenMetricIDs[threshold.MetricID] = true
	}

	var summary struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		Status       string `json:"status"`
		Summary      struct {
			CasesTotal        int     `json:"cases_total"`
			CasesPassed       int     `json:"cases_passed"`
			CasesFailed       int     `json:"cases_failed"`
			Assertions        int     `json:"assertions"`
			PassRate          float64 `json:"pass_rate"`
			JudgeReplayErrors int     `json:"judge_replay_errors"`
		} `json:"summary"`
		Cases []struct {
			CaseID        string `json:"case_id"`
			Name          string `json:"name"`
			Status        string `json:"status"`
			JudgeTraceRef string `json:"judge_trace_ref"`
			Assertions    int    `json:"assertions"`
			Metrics       []struct {
				MetricID string `json:"metric_id"`
				Type     string `json:"type"`
				Value    any    `json:"value"`
			} `json:"metrics"`
		} `json:"cases"`
		Metrics []struct {
			MetricID string `json:"metric_id"`
			Type     string `json:"type"`
			Count    int    `json:"count"`
		} `json:"metrics"`
		Findings []any `json:"findings"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(base, "fixtures", "case_result_summary_fixture.json"), &summary)
	if !summary.ProviderFree || summary.LiveNetwork || summary.Status != "passed" ||
		summary.Summary.CasesTotal != 2 || summary.Summary.CasesPassed != 2 ||
		summary.Summary.CasesFailed != 0 || summary.Summary.Assertions != 8 ||
		summary.Summary.PassRate != 1 || summary.Summary.JudgeReplayErrors != 0 ||
		len(summary.Findings) != 0 {
		t.Fatalf("case result summary = %#v", summary)
	}
	summaryMetricCounts := map[string]int{}
	for _, metric := range summary.Metrics {
		if gotMetricTypes[metric.MetricID] != metric.Type || metric.Count != dataset.CaseCount {
			t.Fatalf("summary aggregate metric does not resolve through metric specs/cases: %#v", metric)
		}
		summaryMetricCounts[metric.MetricID] = metric.Count
	}
	for _, c := range summary.Cases {
		if !caseIDs[c.CaseID] || c.Name != caseNames[c.CaseID] || c.Status != "passed" ||
			!traceIDs[c.JudgeTraceRef] || c.Assertions <= 0 || len(c.Metrics) != len(metrics.Metrics) {
			t.Fatalf("case result incomplete: %#v", c)
		}
		for _, metric := range c.Metrics {
			if gotMetricTypes[metric.MetricID] != metric.Type {
				t.Fatalf("case %s metric %s type = %q, want %q", c.CaseID, metric.MetricID, metric.Type, gotMetricTypes[metric.MetricID])
			}
			if summaryMetricCounts[metric.MetricID] != dataset.CaseCount {
				t.Fatalf("case %s metric %s missing from summary aggregates: %#v", c.CaseID, metric.MetricID, summary.Metrics)
			}
			if metric.Value == nil {
				t.Fatalf("case %s metric %s has nil value", c.CaseID, metric.MetricID)
			}
		}
	}
	for _, want := range []string{"dataset_item_valid", "judge_passed", "answer_chars", "judge_tokens"} {
		if !goldenMetricIDs[want] {
			t.Fatalf("golden gates missing threshold for required metric %q: %#v", want, golden.MetricThresholds)
		}
	}
}

func TestFinRobotGenericEvaluationHarnessFixtureIndexAndNoRuntimeImports(t *testing.T) {
	base := genericEvaluationHarnessLivePackageDir(t)
	var index struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		RequiresCredentials   bool `json:"requires_credentials"`
		Fixtures              []struct {
			FixtureKey string         `json:"fixture_key"`
			Capability string         `json:"capability"`
			Path       string         `json:"path"`
			Schemas    []string       `json:"schemas"`
			Metadata   map[string]any `json:"metadata"`
		} `json:"fixtures"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || index.RequiresCredentials || len(index.Fixtures) != 8 {
		t.Fatalf("fixture index header = %#v", index)
	}
	indexCapabilities := map[string]bool{}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Capability == "" || fixture.Path == "" || len(fixture.Schemas) == 0 {
			t.Fatalf("fixture index entry incomplete: %#v", fixture)
		}
		indexCapabilities[fixture.Capability] = true
		assertGenericEvaluationJSONFile(t, filepath.Join(base, fixture.Path))
		for _, schema := range fixture.Schemas {
			assertGenericEvaluationJSONFile(t, filepath.Join(base, schema))
		}
		if fixture.Metadata["network_required"] == true || fixture.Metadata["live_provider_calls"] == true {
			t.Fatalf("fixture index entry requires live dependency: %#v", fixture)
		}
		if fixture.Metadata["provider_free"] != true || fixture.Metadata["live_network"] == true || fixture.Metadata["real_dependency_imports"] == true {
			t.Fatalf("fixture index entry metadata must remain provider-free/offline/no-imports: %#v", fixture)
		}
	}
	for _, want := range []string{
		"generic.ai.evaluation.harness.dataset_manifest",
		"generic.ai.evaluation.harness.case_inputs",
		"generic.ai.evaluation.harness.metric_specs",
		"generic.ai.evaluation.harness.judge_replay_records",
		"generic.ai.evaluation.harness.findings",
		"generic.ai.evaluation.harness.golden_gates",
		"generic.ai.evaluation.harness.case_result_summary",
		"generic.ai.evaluation.harness.agent_run_projection",
	} {
		if !indexCapabilities[want] {
			t.Fatalf("fixture index missing capability %q: %#v", want, index.Fixtures)
		}
	}

	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !(strings.HasSuffix(path, ".leia") || strings.HasSuffix(path, ".json")) {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		source := string(data)
		for _, pattern := range []string{
			`(?m)^\s*import\s+`,
			`(?m)^\s*use\s+`,
			`(?m)^\s*load\s*\(`,
			`(?m)^\s*require\s*\(`,
			`(?m)^\s*(openai|anthropic|requests|http|curl)\s*[.(]`,
			`(?i)(openai|anthropic|google|azure|aws|api)[_-]?key`,
			`(?i)bearer\s+[a-z0-9._-]+`,
			`sk-[A-Za-z0-9]{20,}`,
		} {
			if regexp.MustCompile(pattern).FindString(source) != "" {
				t.Fatalf("%s contains forbidden runtime/provider/network dependency matching %q", rel, pattern)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFinRobotGenericEvaluationHarnessAgentRunProjection(t *testing.T) {
	root := repoRoot(t)
	base := genericEvaluationHarnessLivePackageDir(t)

	var projection struct {
		SchemaVersion        int    `json:"schema_version"`
		FixtureKey           string `json:"fixture_key"`
		ProjectionKind       string `json:"projection_kind"`
		ProviderFree         bool   `json:"provider_free"`
		LiveNetwork          bool   `json:"live_network"`
		RealDependencyImport bool   `json:"real_dependency_imports"`
		SourceFixtureRefs    struct {
			AgentToolContractProjection  string `json:"agent_tool_contract_projection"`
			AgentLoopTrace               string `json:"agent_loop_trace"`
			RecordReplayOrderedRecords   string `json:"record_replay_ordered_records"`
			RecordReplayMatchingRequests string `json:"record_replay_matching_requests"`
			TraceEnvelope                string `json:"trace_envelope"`
			CaseResultSummary            string `json:"case_result_summary"`
			GoldenGates                  string `json:"golden_gates"`
		} `json:"source_fixture_refs"`
		RunIdentityMappings []struct {
			SourcePackage   string `json:"source_package"`
			SourceIdentity  string `json:"source_identity"`
			TargetDatasetID string `json:"target_dataset_id"`
			TargetCaseID    string `json:"target_case_id"`
			IdentityPolicy  string `json:"identity_policy"`
			ProviderFree    bool   `json:"provider_free"`
		} `json:"run_identity_mappings"`
		ReplayRecordMappings []struct {
			RecordID           string   `json:"record_id"`
			ReplayKey          string   `json:"replay_key"`
			Operation          string   `json:"operation"`
			Capability         string   `json:"capability"`
			RequestHashRef     string   `json:"request_hash_ref"`
			TargetCaseID       string   `json:"target_case_id"`
			ProjectedMetricIDs []string `json:"projected_metric_ids"`
			ProviderFree       bool     `json:"provider_free"`
		} `json:"replay_record_mappings"`
		TraceEventMappings []struct {
			SourceTraceID       string `json:"source_trace_id"`
			SourceEventType     string `json:"source_event_type"`
			TargetCaseID        string `json:"target_case_id"`
			TargetJudgeTraceRef string `json:"target_judge_trace_ref"`
			ProjectionPolicy    string `json:"projection_policy"`
			ProviderFree        bool   `json:"provider_free"`
		} `json:"trace_event_mappings"`
		CaseResultMappings []struct {
			TargetCaseID            string `json:"target_case_id"`
			SourceToolHistoryStatus string `json:"source_tool_history_status"`
			SourceReplayKey         string `json:"source_replay_key"`
			TargetStatus            string `json:"target_status"`
			TargetJudgeTraceRef     string `json:"target_judge_trace_ref"`
			AssertionsProjected     int    `json:"assertions_projected"`
			FindingCount            int    `json:"finding_count"`
		} `json:"case_result_mappings"`
		GoldenGateProjection struct {
			SourceSummaryStatus string  `json:"source_summary_status"`
			TargetGate          bool    `json:"target_gate"`
			TargetPassRate      float64 `json:"target_pass_rate"`
			CasesFailed         int     `json:"cases_failed"`
			Findings            int     `json:"findings"`
			JudgeReplayErrors   int     `json:"judge_replay_errors"`
			ProviderFree        bool    `json:"provider_free"`
		} `json:"golden_gate_projection"`
		ProjectionAssertions map[string]bool `json:"projection_assertions"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(base, "fixtures", "agent_run_evaluation_projection_fixture.json"), &projection)
	if projection.SchemaVersion != 1 ||
		projection.FixtureKey != "generic_eval:agent_run_projection:offline:v1" ||
		projection.ProjectionKind != "agent_run_to_evaluation_harness_projection" ||
		!projection.ProviderFree || projection.LiveNetwork || projection.RealDependencyImport ||
		projection.SourceFixtureRefs.AgentToolContractProjection == "" ||
		projection.SourceFixtureRefs.RecordReplayOrderedRecords == "" ||
		projection.SourceFixtureRefs.TraceEnvelope == "" ||
		projection.SourceFixtureRefs.CaseResultSummary == "" ||
		projection.SourceFixtureRefs.GoldenGates == "" {
		t.Fatalf("agent run projection header/source refs incomplete: %#v", projection)
	}

	var agentProjection struct {
		FixtureKey          string `json:"fixture_key"`
		ProviderFree        bool   `json:"provider_free"`
		ToolHistoryMappings []struct {
			AgentStatus string `json:"agent_status"`
			ResultRef   string `json:"result_ref"`
		} `json:"tool_history_mappings"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "generic_agent_runner", "fixtures", "tool_contract_agent_projection_fixture.json"), &agentProjection)
	var agentLoopTrace struct {
		ID           string `json:"id"`
		ProviderFree bool   `json:"provider_free"`
		Events       []struct {
			Type string `json:"type"`
		} `json:"events"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "generic_agent_runner", "fixtures", "loop_trace_fixture.json"), &agentLoopTrace)
	var records struct {
		FixtureID    string `json:"fixture_id"`
		ProviderFree bool   `json:"provider_free"`
		Records      []struct {
			RecordID    string `json:"record_id"`
			ReplayKey   string `json:"replay_key"`
			Operation   string `json:"operation"`
			Capability  string `json:"capability"`
			RequestHash string `json:"request_hash"`
		} `json:"records"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "generic_record_replay", "fixtures", "ordered_records_fixture.json"), &records)
	var matching struct {
		Requests []struct {
			ReplayKey   string `json:"replay_key"`
			Operation   string `json:"operation"`
			Capability  string `json:"capability"`
			RequestHash string `json:"request_hash"`
		} `json:"requests"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "generic_record_replay", "fixtures", "matching_requests_fixture.json"), &matching)
	var trace struct {
		ProviderFree bool `json:"provider_free"`
		Events       []struct {
			TraceID   string `json:"trace_id"`
			EventType string `json:"event_type"`
		} `json:"events"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "generic_trace_events", "fixtures", "trace_envelope_fixture.json"), &trace)
	var summary struct {
		ProviderFree bool   `json:"provider_free"`
		Status       string `json:"status"`
		Summary      struct {
			CasesFailed       int     `json:"cases_failed"`
			PassRate          float64 `json:"pass_rate"`
			JudgeReplayErrors int     `json:"judge_replay_errors"`
		} `json:"summary"`
		Cases []struct {
			CaseID        string `json:"case_id"`
			Status        string `json:"status"`
			JudgeTraceRef string `json:"judge_trace_ref"`
			Assertions    int    `json:"assertions"`
		} `json:"cases"`
		Findings []any `json:"findings"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(base, "fixtures", "case_result_summary_fixture.json"), &summary)
	var golden struct {
		ProviderFree         bool    `json:"provider_free"`
		Gate                 bool    `json:"gate"`
		SummaryPassRateMin   float64 `json:"summary_pass_rate_min"`
		CasesFailedMax       int     `json:"cases_failed_max"`
		FindingsMax          int     `json:"findings_max"`
		JudgeReplayErrorsMax int     `json:"judge_replay_errors_max"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(base, "fixtures", "golden_gates_fixture.json"), &golden)

	caseIDs := map[string]struct {
		status     string
		trace      string
		assertions int
	}{}
	for _, item := range summary.Cases {
		caseIDs[item.CaseID] = struct {
			status     string
			trace      string
			assertions int
		}{status: item.Status, trace: item.JudgeTraceRef, assertions: item.Assertions}
	}
	agentStatuses := map[string]bool{}
	for _, item := range agentProjection.ToolHistoryMappings {
		agentStatuses[item.AgentStatus+":"+item.ResultRef] = true
	}
	recordByID := map[string]struct {
		replayKey   string
		operation   string
		capability  string
		requestHash string
	}{}
	for _, record := range records.Records {
		recordByID[record.RecordID] = struct {
			replayKey   string
			operation   string
			capability  string
			requestHash string
		}{replayKey: record.ReplayKey, operation: record.Operation, capability: record.Capability, requestHash: record.RequestHash}
	}
	matchingKeys := map[string]bool{}
	for _, request := range matching.Requests {
		matchingKeys[request.ReplayKey+"|"+request.RequestHash] = true
	}
	traceEvents := map[string]bool{}
	for _, event := range trace.Events {
		traceEvents[event.TraceID+"|"+event.EventType] = true
	}

	for _, mapping := range projection.RunIdentityMappings {
		if _, ok := caseIDs[mapping.TargetCaseID]; !ok ||
			mapping.TargetDatasetID != "generic-ai-eval-smoke-dataset" ||
			!strings.Contains(mapping.IdentityPolicy, "no_id_equality") ||
			!mapping.ProviderFree {
			t.Fatalf("run identity mapping does not resolve to eval case: %#v", mapping)
		}
		if mapping.SourcePackage == "generic_agent_runner" && mapping.SourceIdentity != agentLoopTrace.ID {
			t.Fatalf("agent run identity does not resolve to loop trace id: %#v", mapping)
		}
		if mapping.SourcePackage == "generic_record_replay" && mapping.SourceIdentity != records.FixtureID {
			t.Fatalf("record replay identity does not resolve to fixture id: %#v", mapping)
		}
	}
	for _, mapping := range projection.ReplayRecordMappings {
		record, ok := recordByID[mapping.RecordID]
		if !ok ||
			mapping.ReplayKey != record.replayKey ||
			mapping.Operation != record.operation ||
			mapping.Capability != record.capability ||
			mapping.RequestHashRef != record.requestHash ||
			!matchingKeys[mapping.ReplayKey+"|"+mapping.RequestHashRef] ||
			!mapping.ProviderFree ||
			len(mapping.ProjectedMetricIDs) == 0 {
			t.Fatalf("replay record mapping does not resolve to ordered/matching replay data: %#v", mapping)
		}
		if _, ok := caseIDs[mapping.TargetCaseID]; !ok {
			t.Fatalf("replay record target case missing: %#v", mapping)
		}
	}
	for _, mapping := range projection.TraceEventMappings {
		if !traceEvents[mapping.SourceTraceID+"|"+mapping.SourceEventType] ||
			caseIDs[mapping.TargetCaseID].trace != mapping.TargetJudgeTraceRef ||
			mapping.ProjectionPolicy != "trace_event_ref_only" ||
			!mapping.ProviderFree {
			t.Fatalf("trace event mapping invalid or redefines trace payload: %#v", mapping)
		}
	}
	for _, mapping := range projection.CaseResultMappings {
		item, ok := caseIDs[mapping.TargetCaseID]
		if !ok ||
			item.status != mapping.TargetStatus ||
			item.trace != mapping.TargetJudgeTraceRef ||
			item.assertions != mapping.AssertionsProjected ||
			mapping.FindingCount != 0 ||
			!agentStatuses[mapping.SourceToolHistoryStatus+":"+mapping.SourceReplayKey] {
			t.Fatalf("case result mapping does not resolve to source agent history and eval summary: %#v", mapping)
		}
	}
	if projection.GoldenGateProjection.SourceSummaryStatus != summary.Status ||
		projection.GoldenGateProjection.TargetGate != golden.Gate ||
		projection.GoldenGateProjection.TargetPassRate != summary.Summary.PassRate ||
		projection.GoldenGateProjection.CasesFailed != summary.Summary.CasesFailed ||
		projection.GoldenGateProjection.Findings != len(summary.Findings) ||
		projection.GoldenGateProjection.JudgeReplayErrors != summary.Summary.JudgeReplayErrors ||
		!projection.GoldenGateProjection.ProviderFree ||
		!golden.ProviderFree {
		t.Fatalf("golden gate projection does not resolve to eval summary/gates: %#v", projection.GoldenGateProjection)
	}
	for _, want := range []string{"agent_run_projects_to_eval_cases", "record_replay_keys_are_kept_distinct_from_trace_ids", "trace_events_are_referenced_not_redefined", "tool_history_status_projects_to_case_status", "denied_tool_history_can_project_to_passed_policy_case", "golden_gate_uses_eval_summary_not_source_trace_status", "provider_free_boundary_preserved", "live_network_absent", "real_dependency_imports_absent"} {
		if !projection.ProjectionAssertions[want] {
			t.Fatalf("agent run projection assertion missing %q: %#v", want, projection.ProjectionAssertions)
		}
	}
}

func TestFinRobotGenericEvaluationHarnessExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericEvaluationHarnessLivePackageDir(t), "main.leia")
	want := "generic_evaluation_harness_live_package surfaces=2 datasets=1 cases=2 metrics=5 replay_records=2 finding_kinds=6 golden_gates=4 summaries=2 agent_run_projections=1 provider_free=true live_network=false imports=false"
	for _, result := range runFinRobotLivePackageSummarySmoke(t, path, "generic_evaluation_harness_live_package_summary", "generic_evaluation_harness_live_package", leia.LibString) {
		if result.Summary != want {
			t.Fatalf("generic_evaluation_harness_live_package_summary = %#v, want %#v", result.Summary, want)
		}
	}
}

func TestFinRobotGenericEvaluationHarnessDeterministicOrdering(t *testing.T) {
	manifest := loadGenericEvaluationHarnessManifest(t, genericEvaluationHarnessLivePackageDir(t))
	var schemaKeys []string
	for key := range manifest.Schemas {
		schemaKeys = append(schemaKeys, key)
	}
	sort.Strings(schemaKeys)
	wantSchemas := []string{"agent_run_evaluation_projection", "case_inputs", "case_result_summary", "dataset_manifest", "findings", "golden_gates", "judge_replay_records", "metric_specs"}
	if !reflect.DeepEqual(schemaKeys, wantSchemas) {
		t.Fatalf("schema keys = %#v, want %#v", schemaKeys, wantSchemas)
	}
	var fixtureKeys []string
	for key := range manifest.Fixtures {
		fixtureKeys = append(fixtureKeys, key)
	}
	sort.Strings(fixtureKeys)
	wantFixtures := []string{"agent_run_evaluation_projection", "case_inputs", "case_result_summary", "dataset_manifest", "findings", "golden_gates", "index", "judge_replay_records", "metric_specs"}
	if !reflect.DeepEqual(fixtureKeys, wantFixtures) {
		t.Fatalf("fixture keys = %#v, want %#v", fixtureKeys, wantFixtures)
	}
}

func genericEvaluationHarnessLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_evaluation_harness")
}

func loadGenericEvaluationHarnessManifest(t *testing.T, base string) genericEvaluationHarnessManifest {
	t.Helper()
	var manifest genericEvaluationHarnessManifest
	decodeGenericEvaluationJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertGenericEvaluationJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeGenericEvaluationJSONFile(t, path, &value)
}

func assertGenericEvaluationJSONOrLeiaFile(t *testing.T, path string) {
	t.Helper()
	if strings.HasSuffix(path, ".leia") {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
		return
	}
	assertGenericEvaluationJSONFile(t, path)
}

func decodeGenericEvaluationJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
