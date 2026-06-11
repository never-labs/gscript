package leia_test

import (
	"encoding/json"
	"fmt"
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
	for _, key := range []string{"dataset_manifest", "case_inputs", "metric_specs", "judge_replay_records", "findings", "golden_gates", "case_result_summary"} {
		if manifest.Schemas[key] == "" || manifest.Fixtures[key] == "" {
			t.Fatalf("missing schema/fixture for %q: schemas=%#v fixtures=%#v", key, manifest.Schemas, manifest.Fixtures)
		}
		assertGenericEvaluationJSONFile(t, filepath.Join(base, manifest.Schemas[key]))
		assertGenericEvaluationJSONFile(t, filepath.Join(base, manifest.Fixtures[key]))
	}
	if len(manifest.ArtifactBoundaries) != 5 {
		t.Fatalf("artifact boundaries = %#v", manifest.ArtifactBoundaries)
	}
	for _, boundary := range manifest.ArtifactBoundaries {
		if boundary.ID == "" || boundary.Capability == "" || boundary.Schema == "" || boundary.Fixture == "" {
			t.Fatalf("artifact boundary incomplete: %#v", boundary)
		}
		if !boundary.ProviderFree || boundary.LiveNetwork {
			t.Fatalf("artifact boundary must be provider-free/offline: %#v", boundary)
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
	for _, want := range []string{"generic.ai.evaluation.harness", "ai.eval.run", "dataset manifest", "case inputs", "metric specs", "judge replay records", "findings", "golden gates", "case result summary", "provider-free"} {
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
		ProviderFree   bool     `json:"provider_free"`
		LiveNetwork    bool     `json:"live_network"`
		DatasetID      string   `json:"dataset_id"`
		Format         string   `json:"format"`
		IDField        string   `json:"id_field"`
		CaseCount      int      `json:"case_count"`
		RequiredFields []string `json:"required_fields"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(base, "fixtures", "dataset_manifest_fixture.json"), &dataset)
	if !dataset.ProviderFree || dataset.LiveNetwork || dataset.Format != "json" || dataset.IDField != "case_id" || dataset.CaseCount != 2 {
		t.Fatalf("dataset manifest = %#v", dataset)
	}
	for _, field := range []string{"case_id", "input", "expected", "rubric"} {
		if !contains(dataset.RequiredFields, field) {
			t.Fatalf("dataset required fields missing %q: %#v", field, dataset.RequiredFields)
		}
	}

	var cases struct {
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
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
	if !cases.ProviderFree || cases.LiveNetwork || len(cases.Cases) != dataset.CaseCount {
		t.Fatalf("case inputs = %#v", cases)
	}
	caseIDs := map[string]bool{}
	for _, c := range cases.Cases {
		if c.CaseID == "" || c.Name == "" || c.Input == nil || c.Expected == nil || c.Rubric.JudgeModel != "mock-ai-eval-judge" || c.Rubric.PassLabel != "pass" || len(c.Rubric.Criteria) == 0 {
			t.Fatalf("case input incomplete: %#v", c)
		}
		caseIDs[c.CaseID] = true
	}

	var metrics struct {
		Metrics []struct {
			MetricID    string         `json:"metric_id"`
			Type        string         `json:"type"`
			Aggregation string         `json:"aggregation"`
			Gate        map[string]any `json:"gate"`
		} `json:"metrics"`
	}
	decodeGenericEvaluationJSONFile(t, filepath.Join(base, "fixtures", "metric_specs_fixture.json"), &metrics)
	gotMetricTypes := map[string]string{}
	for _, metric := range metrics.Metrics {
		if metric.MetricID == "" || metric.Type == "" || metric.Aggregation == "" || len(metric.Gate) == 0 {
			t.Fatalf("metric spec incomplete: %#v", metric)
		}
		gotMetricTypes[metric.MetricID] = metric.Type
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
		traceIDs[record.TraceID] = true
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
			Status        string `json:"status"`
			JudgeTraceRef string `json:"judge_trace_ref"`
			Metrics       []struct {
				MetricID string `json:"metric_id"`
				Type     string `json:"type"`
			} `json:"metrics"`
		} `json:"cases"`
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
	for _, c := range summary.Cases {
		if !caseIDs[c.CaseID] || c.Status != "passed" || !traceIDs[c.JudgeTraceRef] || len(c.Metrics) != len(metrics.Metrics) {
			t.Fatalf("case result incomplete: %#v", c)
		}
		for _, metric := range c.Metrics {
			if gotMetricTypes[metric.MetricID] != metric.Type {
				t.Fatalf("case %s metric %s type = %q, want %q", c.CaseID, metric.MetricID, metric.Type, gotMetricTypes[metric.MetricID])
			}
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
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || index.RequiresCredentials || len(index.Fixtures) != 5 {
		t.Fatalf("fixture index header = %#v", index)
	}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Capability == "" || fixture.Path == "" || len(fixture.Schemas) == 0 {
			t.Fatalf("fixture index entry incomplete: %#v", fixture)
		}
		assertGenericEvaluationJSONFile(t, filepath.Join(base, fixture.Path))
		for _, schema := range fixture.Schemas {
			assertGenericEvaluationJSONFile(t, filepath.Join(base, schema))
		}
		if fixture.Metadata["network_required"] == true || fixture.Metadata["live_provider_calls"] == true {
			t.Fatalf("fixture index entry requires live dependency: %#v", fixture)
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

func TestFinRobotGenericEvaluationHarnessExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericEvaluationHarnessLivePackageDir(t), "main.leia")
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var prints []string
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString),
				leia.WithPrint(func(args ...any) {
					var parts []string
					for _, arg := range args {
						parts = append(parts, fmt.Sprint(arg))
					}
					prints = append(prints, strings.Join(parts, " "))
				}),
			}, tc.opts...)...)
			if err := vm.ExecFile(path); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}
			got, err := vm.Get("generic_evaluation_harness_live_package_summary")
			if err != nil {
				t.Fatalf("Get generic_evaluation_harness_live_package_summary: %v", err)
			}
			want := "generic_evaluation_harness_live_package surfaces=2 datasets=1 cases=2 metrics=5 replay_records=2 finding_kinds=6 golden_gates=4 summaries=2 provider_free=true live_network=false imports=false"
			if got != want {
				t.Fatalf("generic_evaluation_harness_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func TestFinRobotGenericEvaluationHarnessDeterministicOrdering(t *testing.T) {
	manifest := loadGenericEvaluationHarnessManifest(t, genericEvaluationHarnessLivePackageDir(t))
	var schemaKeys []string
	for key := range manifest.Schemas {
		schemaKeys = append(schemaKeys, key)
	}
	sort.Strings(schemaKeys)
	wantSchemas := []string{"case_inputs", "case_result_summary", "dataset_manifest", "findings", "golden_gates", "judge_replay_records", "metric_specs"}
	if !reflect.DeepEqual(schemaKeys, wantSchemas) {
		t.Fatalf("schema keys = %#v, want %#v", schemaKeys, wantSchemas)
	}
	var fixtureKeys []string
	for key := range manifest.Fixtures {
		fixtureKeys = append(fixtureKeys, key)
	}
	sort.Strings(fixtureKeys)
	wantFixtures := []string{"case_inputs", "case_result_summary", "dataset_manifest", "findings", "golden_gates", "index", "judge_replay_records", "metric_specs"}
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
