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

type equityAnalysisPipelineLiveManifest struct {
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
	Entrypoints  map[string]string                   `json:"entrypoints"`
	Schemas      map[string]string                   `json:"schemas"`
	Fixtures     map[string]string                   `json:"fixtures"`
	Stages       []equityAnalysisPipelineStage       `json:"stages"`
	FailureHooks []equityAnalysisPipelineFailureHook `json:"failure_hooks"`
	TestGates    []string                            `json:"test_gates"`
	NoBuiltIn    map[string]json.RawMessage          `json:"no_built_in_guarantee"`
}

type equityAnalysisPipelineStage struct {
	ID           string   `json:"id"`
	Capability   string   `json:"capability"`
	DependsOn    []string `json:"depends_on"`
	InputSchema  string   `json:"input_schema"`
	OutputSchema string   `json:"output_schema"`
	FixtureKey   string   `json:"fixture_key"`
	ProviderFree bool     `json:"provider_free"`
	LiveNetwork  bool     `json:"live_network"`
}

type equityAnalysisPipelineFailureHook struct {
	ID          string `json:"id"`
	StageID     string `json:"stage_id"`
	Capability  string `json:"capability"`
	FixtureKey  string `json:"fixture_key"`
	CleanSkip   bool   `json:"clean_skip"`
	LiveNetwork bool   `json:"live_network"`
}

func TestFinRobotEquityAnalysisPipelineLivePackageManifest(t *testing.T) {
	base := equityAnalysisPipelineLivePackageDir(t)
	manifest := loadEquityAnalysisPipelineLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-equity-analysis-pipeline-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-equity-analysis-pipeline" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	for _, want := range []string{"market data", "filings", "model provider", "forecast engine", "artifact storage"} {
		if !strings.Contains(manifest.Credentials.Policy, want) {
			t.Fatalf("credential policy should name %q boundary: %q", want, manifest.Credentials.Policy)
		}
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_equity_analysis_pipeline_live_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}

	wantSources := []string{"finrobot_equity/core/src/generate_financial_analysis.py"}
	if !reflect.DeepEqual(manifest.SourceModules, wantSources) {
		t.Fatalf("source modules = %#v, want %#v", manifest.SourceModules, wantSources)
	}
	for _, key := range []string{"smoke", "stage_dag_contract", "failure_hooks_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"pipeline_input", "normalized_financials", "forecast_result", "section_agent_handoff", "artifact_manifest", "failure_hook", "provider_free_trace"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertEquityAnalysisPipelineJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "input", "normalized_financials", "forecast", "section_agent_handoff", "artifact_manifest", "failure_hooks", "provider_free_trace"} {
		path := manifest.Fixtures[key]
		if path == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertEquityAnalysisPipelineJSONFile(t, filepath.Join(base, path))
	}
	if len(manifest.NoBuiltIn) == 0 {
		t.Fatal("missing no_built_in_guarantee")
	}
	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"generate_financial_analysis.py", "fetch", "normalize", "forecast", "section_agents", "artifact", "failure hooks", "provider"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotEquityAnalysisPipelineStageDAGContract(t *testing.T) {
	base := equityAnalysisPipelineLivePackageDir(t)
	manifest := loadEquityAnalysisPipelineLiveManifest(t, base)

	var ids []string
	stageByID := map[string]equityAnalysisPipelineStage{}
	for _, stage := range manifest.Stages {
		ids = append(ids, stage.ID)
		stageByID[stage.ID] = stage
		if stage.ID == "" || stage.Capability == "" || stage.InputSchema == "" || stage.OutputSchema == "" || stage.FixtureKey == "" {
			t.Fatalf("stage metadata incomplete: %#v", stage)
		}
		if !stage.ProviderFree || stage.LiveNetwork {
			t.Fatalf("stage must be provider-free and offline: %#v", stage)
		}
		if !strings.HasPrefix(stage.Capability, "finance.equity_analysis_pipeline.") {
			t.Fatalf("%s capability = %q", stage.ID, stage.Capability)
		}
	}
	wantOrder := []string{"fetch", "normalize", "forecast", "section_agents", "artifact_plan"}
	if !reflect.DeepEqual(ids, wantOrder) {
		t.Fatalf("stage order = %#v, want %#v", ids, wantOrder)
	}
	if len(stageByID["fetch"].DependsOn) != 0 ||
		!reflect.DeepEqual(stageByID["normalize"].DependsOn, []string{"fetch"}) ||
		!reflect.DeepEqual(stageByID["forecast"].DependsOn, []string{"normalize"}) ||
		!reflect.DeepEqual(stageByID["section_agents"].DependsOn, []string{"forecast"}) ||
		!reflect.DeepEqual(stageByID["artifact_plan"].DependsOn, []string{"section_agents"}) {
		t.Fatalf("unexpected stage dependencies: %#v", stageByID)
	}

	var contract struct {
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		SourceModule          string `json:"source_module"`
		Stages                []struct {
			ID             string   `json:"id"`
			DependsOn      []string `json:"depends_on"`
			RequiredFields []string `json:"required_fields"`
			OutputFixture  string   `json:"output_fixture"`
		} `json:"stages"`
		DAGEdges      [][]string `json:"dag_edges"`
		TraceContract struct {
			Schema         string   `json:"schema"`
			Fixture        string   `json:"fixture"`
			RequiredFields []string `json:"required_fields"`
		} `json:"trace_contract"`
		AcceptanceGates []string `json:"acceptance_gates"`
	}
	decodeEquityAnalysisPipelineJSONFile(t, filepath.Join(base, "contracts", "stage_dag_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || contract.SourceModule != "finrobot_equity/core/src/generate_financial_analysis.py" || len(contract.Stages) != 5 || len(contract.DAGEdges) != 4 {
		t.Fatalf("stage DAG contract header/count = %#v", contract)
	}
	for _, stage := range contract.Stages {
		if stage.ID == "" || len(stage.RequiredFields) < 5 || stage.OutputFixture == "" {
			t.Fatalf("stage contract incomplete: %#v", stage)
		}
		assertEquityAnalysisPipelineJSONFile(t, filepath.Join(base, stage.OutputFixture))
	}
	if contract.TraceContract.Schema == "" || contract.TraceContract.Fixture == "" || len(contract.TraceContract.RequiredFields) < 6 {
		t.Fatalf("trace contract incomplete: %#v", contract.TraceContract)
	}
	assertEquityAnalysisPipelineJSONFile(t, filepath.Join(base, contract.TraceContract.Schema))
	assertEquityAnalysisPipelineJSONFile(t, filepath.Join(base, contract.TraceContract.Fixture))
}

func TestFinRobotEquityAnalysisPipelineFixturesAndTrace(t *testing.T) {
	base := equityAnalysisPipelineLivePackageDir(t)

	var index struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Fixtures              []struct {
			FixtureKey string         `json:"fixture_key"`
			Capability string         `json:"capability"`
			Path       string         `json:"path"`
			Schema     string         `json:"schema"`
			Metadata   map[string]any `json:"metadata"`
		} `json:"fixtures"`
	}
	decodeEquityAnalysisPipelineJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 7 {
		t.Fatalf("fixture index header/count = %#v", index)
	}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Capability == "" || fixture.Path == "" || fixture.Schema == "" {
			t.Fatalf("fixture metadata incomplete: %#v", fixture)
		}
		if fixture.Metadata["replay_ready"] != true || fixture.Metadata["provider_free"] != true {
			t.Fatalf("%s replay/provider metadata = %#v", fixture.FixtureKey, fixture.Metadata)
		}
		assertEquityAnalysisPipelineJSONFile(t, filepath.Join(base, fixture.Path))
		assertEquityAnalysisPipelineJSONFile(t, filepath.Join(base, fixture.Schema))
	}

	var forecast struct {
		ProviderFree     bool               `json:"provider_free"`
		LiveNetwork      bool               `json:"live_network"`
		Ticker           string             `json:"ticker"`
		ForecastPeriods  []string           `json:"forecast_periods"`
		Assumptions      map[string]float64 `json:"assumptions"`
		ForecastRows     []map[string]any   `json:"forecast_rows"`
		ValuationSummary map[string]any     `json:"valuation_summary"`
	}
	decodeEquityAnalysisPipelineJSONFile(t, filepath.Join(base, "fixtures", "forecast_ACME_fixture.json"), &forecast)
	if !forecast.ProviderFree || forecast.LiveNetwork || forecast.Ticker != "ACME" || len(forecast.ForecastPeriods) != 3 || len(forecast.ForecastRows) != 3 {
		t.Fatalf("forecast fixture incomplete: %#v", forecast)
	}
	for _, key := range []string{"revenue_cagr", "terminal_growth", "discount_rate", "target_ebitda_margin"} {
		if forecast.Assumptions[key] == 0 {
			t.Fatalf("forecast assumptions missing %q: %#v", key, forecast.Assumptions)
		}
	}
	for _, key := range []string{"dcf_price", "peer_multiple_price", "target_price", "rating", "market_price"} {
		if forecast.ValuationSummary[key] == nil {
			t.Fatalf("valuation summary missing %q: %#v", key, forecast.ValuationSummary)
		}
	}

	var handoff struct {
		ProviderFree        bool             `json:"provider_free"`
		LiveNetwork         bool             `json:"live_network"`
		AgentRoles          []string         `json:"agent_roles"`
		OrderedSections     []string         `json:"ordered_sections"`
		HandoffPayloads     []map[string]any `json:"handoff_payloads"`
		EvidenceRefs        []string         `json:"evidence_refs"`
		TerminateConvention struct {
			Required bool   `json:"required"`
			Token    string `json:"token"`
		} `json:"terminate_convention"`
	}
	decodeEquityAnalysisPipelineJSONFile(t, filepath.Join(base, "fixtures", "section_agent_handoff_ACME_fixture.json"), &handoff)
	if !handoff.ProviderFree || handoff.LiveNetwork || len(handoff.AgentRoles) < 4 || len(handoff.OrderedSections) < 4 || len(handoff.HandoffPayloads) < 2 || len(handoff.EvidenceRefs) < 4 || !handoff.TerminateConvention.Required || handoff.TerminateConvention.Token != "TERMINATE" {
		t.Fatalf("handoff fixture incomplete: %#v", handoff)
	}

	var artifact struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		ReportID     string `json:"report_id"`
		Artifacts    []struct {
			ArtifactID string   `json:"artifact_id"`
			Kind       string   `json:"kind"`
			Status     string   `json:"status"`
			SourceRefs []string `json:"source_refs"`
		} `json:"artifacts"`
		TraceRefs   []string `json:"trace_refs"`
		Disclosures []struct {
			ID         string `json:"id"`
			MustRender bool   `json:"must_render"`
		} `json:"disclosures"`
	}
	decodeEquityAnalysisPipelineJSONFile(t, filepath.Join(base, "fixtures", "artifact_manifest_ACME_fixture.json"), &artifact)
	if !artifact.ProviderFree || artifact.LiveNetwork || artifact.ReportID == "" || len(artifact.Artifacts) != 5 || len(artifact.TraceRefs) == 0 || len(artifact.Disclosures) < 2 {
		t.Fatalf("artifact fixture incomplete: %#v", artifact)
	}
	kinds := map[string]bool{}
	for _, item := range artifact.Artifacts {
		if item.ArtifactID == "" || item.Kind == "" || item.Status == "" || len(item.SourceRefs) == 0 {
			t.Fatalf("artifact entry incomplete: %#v", item)
		}
		kinds[item.Kind] = true
	}
	for _, want := range []string{"report", "table", "chart", "trace", "disclosure"} {
		if !kinds[want] {
			t.Fatalf("artifact manifest missing kind %q: %#v", want, kinds)
		}
	}

	var trace struct {
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		RunID                 string `json:"run_id"`
		StageEvents           []struct {
			EventID    string `json:"event_id"`
			StageID    string `json:"stage_id"`
			Status     string `json:"status"`
			FixtureKey string `json:"fixture_key"`
		} `json:"stage_events"`
		ArtifactRefs []string `json:"artifact_refs"`
	}
	decodeEquityAnalysisPipelineJSONFile(t, filepath.Join(base, "fixtures", "provider_free_trace_ACME_fixture.json"), &trace)
	if !trace.ProviderFree || trace.LiveNetwork || trace.RealDependencyImports || trace.RunID == "" || len(trace.StageEvents) != 5 || len(trace.ArtifactRefs) != 5 {
		t.Fatalf("provider-free trace incomplete: %#v", trace)
	}
	var eventStages []string
	for _, event := range trace.StageEvents {
		eventStages = append(eventStages, event.StageID)
		if event.EventID == "" || event.Status == "" || event.FixtureKey == "" {
			t.Fatalf("trace event incomplete: %#v", event)
		}
	}
	if !reflect.DeepEqual(eventStages, []string{"fetch", "normalize", "forecast", "section_agents", "artifact_plan"}) {
		t.Fatalf("trace stages = %#v", eventStages)
	}
}

func TestFinRobotEquityAnalysisPipelineFailureHooks(t *testing.T) {
	base := equityAnalysisPipelineLivePackageDir(t)
	manifest := loadEquityAnalysisPipelineLiveManifest(t, base)

	var hookIDs []string
	for _, hook := range manifest.FailureHooks {
		hookIDs = append(hookIDs, hook.ID)
		if hook.ID == "" || hook.StageID == "" || hook.Capability == "" || hook.FixtureKey == "" || !hook.CleanSkip || hook.LiveNetwork {
			t.Fatalf("failure hook metadata incomplete: %#v", hook)
		}
	}
	sort.Strings(hookIDs)
	wantHooks := []string{"artifact_renderer_missing", "fetch_provider_unavailable", "forecast_assumption_gap", "normalization_missing_metric"}
	if !reflect.DeepEqual(hookIDs, wantHooks) {
		t.Fatalf("failure hook ids = %#v, want %#v", hookIDs, wantHooks)
	}

	var contract struct {
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		FailureHooksFixture   string `json:"failure_hooks_fixture"`
		FailureHooks          []struct {
			ID             string `json:"id"`
			StageID        string `json:"stage_id"`
			FallbackAction string `json:"fallback_action"`
			ExpectedStatus string `json:"expected_status"`
		} `json:"failure_hooks"`
		AcceptanceGates []string `json:"acceptance_gates"`
	}
	decodeEquityAnalysisPipelineJSONFile(t, filepath.Join(base, "contracts", "failure_hooks_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || contract.FailureHooksFixture == "" || len(contract.FailureHooks) != 4 {
		t.Fatalf("failure hook contract incomplete: %#v", contract)
	}
	assertEquityAnalysisPipelineJSONFile(t, filepath.Join(base, contract.FailureHooksFixture))
	joined := strings.ToLower(strings.Join(contract.AcceptanceGates, " "))
	for _, want := range []string{"offline fixture", "no hook imports", "clean-skip", "trace"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("failure hook acceptance gates missing %q: %s", want, joined)
		}
	}
}

func TestFinRobotEquityAnalysisPipelineLivePackageNoLiveImports(t *testing.T) {
	base := equityAnalysisPipelineLivePackageDir(t)
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".leia") {
			files = append(files, filepath.Join(base, entry.Name()))
		}
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		for _, pattern := range []string{
			`(?m)^\s*import\s+`,
			`(?m)^\s*use\s+`,
			`(?m)^\s*load\s*\(`,
			`(?m)^\s*require\s*\(`,
			`(?m)^\s*(yfinance|finnhub|openbb|requests|http|pandas|numpy|autogen|matplotlib)\s*[.(]`,
		} {
			if regexp.MustCompile(pattern).FindString(source) != "" {
				t.Fatalf("%s contains live dependency loader matching %q", path, pattern)
			}
		}
	}
}

func TestFinRobotEquityAnalysisPipelineLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(equityAnalysisPipelineLivePackageDir(t), "main.leia")

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
			got, err := vm.Get("equity_analysis_pipeline_live_package_summary")
			if err != nil {
				t.Fatalf("Get equity_analysis_pipeline_live_package_summary: %v", err)
			}
			want := "equity_analysis_pipeline_live_package stages=5 fixtures=7 schemas=7 failure_hooks=4 provider_free=true live_network=false imports=false trace=provider_free"
			if got != want {
				t.Fatalf("equity_analysis_pipeline_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func equityAnalysisPipelineLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "equity_analysis_pipeline")
}

func loadEquityAnalysisPipelineLiveManifest(t *testing.T, base string) equityAnalysisPipelineLiveManifest {
	t.Helper()
	var manifest equityAnalysisPipelineLiveManifest
	decodeEquityAnalysisPipelineJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertEquityAnalysisPipelineJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeEquityAnalysisPipelineJSONFile(t, path, &value)
}

func decodeEquityAnalysisPipelineJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
