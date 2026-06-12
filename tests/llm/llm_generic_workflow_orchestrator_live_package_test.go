package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type genericWorkflowOrchestratorManifest struct {
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
	Entrypoints        map[string]string                  `json:"entrypoints"`
	DialectEntrypoints map[string]string                  `json:"dialect_entrypoints"`
	Schemas            map[string]string                  `json:"schemas"`
	Fixtures           map[string]string                  `json:"fixtures"`
	WorkflowGraph      genericWorkflowGraphManifest       `json:"workflow_graph"`
	RetryPolicy        genericWorkflowRetryPolicy         `json:"retry_policy"`
	CachePolicy        genericWorkflowCachePolicy         `json:"cache_policy"`
	TraceEmissionHooks []genericWorkflowTraceEmissionHook `json:"trace_emission_hooks"`
	Capabilities       []string                           `json:"capabilities"`
	DialectSurface     []string                           `json:"dialect_surface"`
	TestGates          []string                           `json:"test_gates"`
	NoBuiltIn          map[string]json.RawMessage         `json:"no_built_in_guarantee"`
}

type genericWorkflowGraphManifest struct {
	ID         string                         `json:"id"`
	Entrypoint string                         `json:"entrypoint"`
	Capability string                         `json:"capability"`
	Stages     []genericWorkflowManifestStage `json:"stages"`
	Edges      [][]string                     `json:"edges"`
}

type genericWorkflowManifestStage struct {
	ID           string   `json:"id"`
	Capability   string   `json:"capability"`
	DependsOn    []string `json:"depends_on"`
	InputSchema  string   `json:"input_schema"`
	OutputSchema string   `json:"output_schema"`
	FixtureKey   string   `json:"fixture_key"`
	ProviderFree bool     `json:"provider_free"`
	LiveNetwork  bool     `json:"live_network"`
}

type genericWorkflowRetryPolicy struct {
	Enabled           bool     `json:"enabled"`
	ProviderFree      bool     `json:"provider_free"`
	LiveNetwork       bool     `json:"live_network"`
	Mode              string   `json:"mode"`
	MaxAttempts       int      `json:"max_attempts"`
	Backoff           string   `json:"backoff"`
	RetryableStatuses []string `json:"retryable_statuses"`
}

type genericWorkflowCachePolicy struct {
	Enabled      bool     `json:"enabled"`
	ProviderFree bool     `json:"provider_free"`
	LiveNetwork  bool     `json:"live_network"`
	Mode         string   `json:"mode"`
	KeyFields    []string `json:"key_fields"`
	CacheStates  []string `json:"cache_states"`
}

type genericWorkflowTraceEmissionHook struct {
	ID           string `json:"id"`
	Phase        string `json:"phase"`
	Capability   string `json:"capability"`
	FixtureKey   string `json:"fixture_key"`
	ProviderFree bool   `json:"provider_free"`
	LiveNetwork  bool   `json:"live_network"`
}

func TestGenericWorkflowOrchestratorLivePackageManifest(t *testing.T) {
	base := genericWorkflowOrchestratorLivePackageDir(t)
	manifest := loadGenericWorkflowOrchestratorManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "generic-ai-workflow-orchestrator-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-generic-ai-workflow-orchestrator" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	for _, want := range []string{"provider adapters", "model endpoints", "storage backends", "queue brokers", "tracing sinks"} {
		if !strings.Contains(manifest.Credentials.Policy, want) {
			t.Fatalf("credential policy should name %q boundary: %q", want, manifest.Credentials.Policy)
		}
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_generic_workflow_orchestrator_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}

	wantSources := []string{"main.leia"}
	if !reflect.DeepEqual(manifest.SourceModules, wantSources) {
		t.Fatalf("source modules = %#v, want %#v", manifest.SourceModules, wantSources)
	}
	for _, want := range []string{"generic.ai.workflow.orchestration", "ai.workflow.orchestrate"} {
		if !contains(manifest.DialectSurface, want) {
			t.Fatalf("dialect surface missing %q: %#v", want, manifest.DialectSurface)
		}
	}
	for _, key := range []string{"smoke", "workflow_graph_contract", "trace_hooks_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
		assertGenericWorkflowOrchestratorJSONOrLeiaFile(t, filepath.Join(base, manifest.Entrypoints[key]))
	}
	if manifest.DialectEntrypoints["orchestrate"] != "ai.workflow.orchestrate" {
		t.Fatalf("orchestrate dialect entrypoint = %q", manifest.DialectEntrypoints["orchestrate"])
	}
	for _, key := range []string{"workflow_graph", "stage_io", "planning_graph_stage_projection", "handoff_trace", "retry_cache_policy", "workflow_result", "trace_emission_hooks"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "workflow_graph", "stage_io", "planning_graph_stage_projection", "handoff_trace", "workflow_result", "trace_emission_hooks"} {
		path := manifest.Fixtures[key]
		if path == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, path))
	}
	if !manifest.RetryPolicy.Enabled || !manifest.RetryPolicy.ProviderFree || manifest.RetryPolicy.LiveNetwork || manifest.RetryPolicy.Mode != "deterministic_fixture_replay" || manifest.RetryPolicy.MaxAttempts != 1 || manifest.RetryPolicy.Backoff != "none" {
		t.Fatalf("retry policy must be deterministic and offline: %#v", manifest.RetryPolicy)
	}
	if !manifest.CachePolicy.Enabled || !manifest.CachePolicy.ProviderFree || manifest.CachePolicy.LiveNetwork || manifest.CachePolicy.Mode != "fixture_metadata_only" || len(manifest.CachePolicy.KeyFields) < 5 || len(manifest.CachePolicy.CacheStates) != 3 {
		t.Fatalf("cache policy must be fixture-only and explicit: %#v", manifest.CachePolicy)
	}
	if len(manifest.TraceEmissionHooks) != 3 {
		t.Fatalf("trace emission hooks = %#v", manifest.TraceEmissionHooks)
	}
	for _, hook := range manifest.TraceEmissionHooks {
		if hook.ID == "" || hook.Phase == "" || hook.Capability == "" || hook.FixtureKey == "" || !hook.ProviderFree || hook.LiveNetwork {
			t.Fatalf("trace hook metadata incomplete: %#v", hook)
		}
	}
	for _, want := range []string{
		"generic.ai.workflow.orchestration.graph",
		"generic.ai.workflow.orchestration.stage.plan",
		"generic.ai.workflow.orchestration.stage.execute",
		"generic.ai.workflow.orchestration.stage.handoff",
		"generic.ai.workflow.orchestration.stage.finalize",
		"generic.ai.workflow.orchestration.planning_graph_stage_projection",
		"generic.ai.workflow.orchestration.workflow_result",
		"ai.workflow.orchestrate.provider_free",
	} {
		if !contains(manifest.Capabilities, want) {
			t.Fatalf("capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}
	if len(manifest.NoBuiltIn) == 0 {
		t.Fatal("missing no_built_in_guarantee")
	}
	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"generic.ai.workflow.orchestration", "ai.workflow.orchestrate", "workflow graph", "stage inputs", "planning graph projection", "handoff trace", "retry/cache", "workflow result", "trace emission hooks"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestGenericWorkflowOrchestratorGraphStageIOAndContracts(t *testing.T) {
	base := genericWorkflowOrchestratorLivePackageDir(t)
	manifest := loadGenericWorkflowOrchestratorManifest(t, base)

	if manifest.WorkflowGraph.Entrypoint != "ai.workflow.orchestrate" || manifest.WorkflowGraph.Capability != "generic.ai.workflow.orchestration.graph" {
		t.Fatalf("workflow graph header = %#v", manifest.WorkflowGraph)
	}
	var ids []string
	stageByID := map[string]genericWorkflowManifestStage{}
	for _, stage := range manifest.WorkflowGraph.Stages {
		ids = append(ids, stage.ID)
		stageByID[stage.ID] = stage
		if stage.ID == "" || stage.Capability == "" || stage.InputSchema == "" || stage.OutputSchema == "" || stage.FixtureKey == "" {
			t.Fatalf("stage metadata incomplete: %#v", stage)
		}
		if !stage.ProviderFree || stage.LiveNetwork {
			t.Fatalf("stage must be provider-free and offline: %#v", stage)
		}
		if !strings.HasPrefix(stage.Capability, "generic.ai.workflow.orchestration.stage.") {
			t.Fatalf("%s capability = %q", stage.ID, stage.Capability)
		}
	}
	wantOrder := []string{"plan", "execute", "handoff", "finalize"}
	if !reflect.DeepEqual(ids, wantOrder) {
		t.Fatalf("stage order = %#v, want %#v", ids, wantOrder)
	}
	if len(stageByID["plan"].DependsOn) != 0 ||
		!reflect.DeepEqual(stageByID["execute"].DependsOn, []string{"plan"}) ||
		!reflect.DeepEqual(stageByID["handoff"].DependsOn, []string{"execute"}) ||
		!reflect.DeepEqual(stageByID["finalize"].DependsOn, []string{"handoff"}) {
		t.Fatalf("unexpected stage dependencies: %#v", stageByID)
	}

	var contract struct {
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		PackageName           string `json:"package_name"`
		Entrypoint            string `json:"entrypoint"`
		WorkflowGraphFixture  string `json:"workflow_graph_fixture"`
		StageIOFixture        string `json:"stage_io_fixture"`
		HandoffTraceFixture   string `json:"handoff_trace_fixture"`
		WorkflowResultFixture string `json:"workflow_result_fixture"`
		Stages                []struct {
			ID             string   `json:"id"`
			DependsOn      []string `json:"depends_on"`
			InputSchema    string   `json:"input_schema"`
			OutputSchema   string   `json:"output_schema"`
			RequiredFields []string `json:"required_fields"`
		} `json:"stages"`
		DAGEdges           [][]string `json:"dag_edges"`
		RetryCacheContract struct {
			ProviderFree        bool     `json:"provider_free"`
			LiveNetwork         bool     `json:"live_network"`
			Schema              string   `json:"schema"`
			MaxAttempts         int      `json:"max_attempts"`
			CacheStates         []string `json:"cache_states"`
			RequiredEventFields []string `json:"required_event_fields"`
		} `json:"retry_cache_contract"`
		AcceptanceGates []string `json:"acceptance_gates"`
	}
	decodeGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, "contracts", "workflow_graph_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || contract.PackageName != "generic.ai.workflow.orchestration" || contract.Entrypoint != "ai.workflow.orchestrate" || len(contract.Stages) != 4 || len(contract.DAGEdges) != 3 {
		t.Fatalf("workflow graph contract header/count = %#v", contract)
	}
	for _, path := range []string{contract.WorkflowGraphFixture, contract.StageIOFixture, contract.HandoffTraceFixture, contract.WorkflowResultFixture, contract.RetryCacheContract.Schema} {
		assertGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, path))
	}
	for _, stage := range contract.Stages {
		if stage.ID == "" || stage.InputSchema == "" || stage.OutputSchema == "" || len(stage.RequiredFields) < 5 {
			t.Fatalf("stage contract incomplete: %#v", stage)
		}
		assertGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, stage.InputSchema))
		assertGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, stage.OutputSchema))
	}
	if !contract.RetryCacheContract.ProviderFree || contract.RetryCacheContract.LiveNetwork || contract.RetryCacheContract.MaxAttempts != 1 || len(contract.RetryCacheContract.CacheStates) != 3 || len(contract.RetryCacheContract.RequiredEventFields) < 5 {
		t.Fatalf("retry/cache contract incomplete: %#v", contract.RetryCacheContract)
	}
}

func TestGenericWorkflowOrchestratorSchemaContractFixtureConsistency(t *testing.T) {
	base := genericWorkflowOrchestratorLivePackageDir(t)
	manifest := loadGenericWorkflowOrchestratorManifest(t, base)

	for schemaKey, wants := range map[string][]string{
		"workflow_graph":                  {"workflow_id", "entrypoint", "provider_free", "live_network", "real_dependency_imports", "package_name", "stages", "edges"},
		"stage_io":                        {"provider_free", "live_network", "workflow_id", "run_id", "stage_io"},
		"planning_graph_stage_projection": {"provider_free", "live_network", "real_dependency_imports", "source_package", "target_package", "source_refs", "workflow_id", "run_id", "stage_mappings", "edge_mappings", "evidence_mappings", "projection_assertions"},
		"handoff_trace":                   {"provider_free", "live_network", "workflow_id", "run_id", "handoffs"},
		"retry_cache_policy":              {"retry_policy", "cache_policy"},
		"workflow_result":                 {"provider_free", "live_network", "real_dependency_imports", "workflow_id", "run_id", "entrypoint", "status", "stage_results", "artifacts", "trace_refs"},
		"trace_emission_hooks":            {"provider_free", "live_network", "real_dependency_imports", "workflow_id", "run_id", "hooks"},
	} {
		assertGenericWorkflowOrchestratorSchemaRequired(t, filepath.Join(base, manifest.Schemas[schemaKey]), nil, wants)
	}
	assertGenericWorkflowOrchestratorSchemaRequired(t, filepath.Join(base, manifest.Schemas["workflow_graph"]), []string{"properties", "stages", "items"}, []string{"id", "depends_on", "input_ref", "output_ref", "input_schema", "output_schema"})
	assertGenericWorkflowOrchestratorSchemaRequired(t, filepath.Join(base, manifest.Schemas["stage_io"]), []string{"properties", "stage_io", "items"}, []string{"stage_id", "input_ref", "input_shape", "output_ref", "output_shape", "status", "fixture_key", "attempt", "max_attempts", "cache_key", "cache_state", "trace_event_id"})
	assertGenericWorkflowOrchestratorSchemaRequired(t, filepath.Join(base, manifest.Schemas["handoff_trace"]), []string{"properties", "handoffs", "items"}, []string{"from_stage", "to_stage", "payload_ref", "payload_schema", "status", "trace_event_id"})
	assertGenericWorkflowOrchestratorSchemaRequired(t, filepath.Join(base, manifest.Schemas["workflow_result"]), []string{"properties", "stage_results", "items"}, []string{"stage_id", "status", "output_ref", "trace_event_id"})
	assertGenericWorkflowOrchestratorSchemaRequired(t, filepath.Join(base, manifest.Schemas["workflow_result"]), []string{"properties", "artifacts", "items"}, []string{"artifact_id", "kind", "status", "source_refs"})
	assertGenericWorkflowOrchestratorSchemaRequired(t, filepath.Join(base, manifest.Schemas["trace_emission_hooks"]), []string{"properties", "hooks", "items"}, []string{"id", "phase", "event_id", "emission_mode", "payload_ref", "provider_sink"})
	assertGenericWorkflowOrchestratorSchemaRequired(t, filepath.Join(base, manifest.Schemas["planning_graph_stage_projection"]), []string{"properties", "stage_mappings", "items"}, []string{"stage_id", "source_node_ids", "source_node_types", "input_ref", "output_ref", "trace_event_refs"})
	assertGenericWorkflowOrchestratorSchemaRequired(t, filepath.Join(base, manifest.Schemas["planning_graph_stage_projection"]), []string{"properties", "edge_mappings", "items"}, []string{"source_edge_type", "source_edges", "workflow_edge", "handoff_ref"})
	assertGenericWorkflowOrchestratorSchemaRequired(t, filepath.Join(base, manifest.Schemas["planning_graph_stage_projection"]), []string{"properties", "evidence_mappings", "items"}, []string{"evidence_kind", "source_event_refs", "workflow_trace_refs"})

	type fixtureIndexRow struct {
		FixtureKey        string   `json:"fixture_key"`
		Capability        string   `json:"capability"`
		CapabilityScope   string   `json:"capability_scope"`
		CapabilityAliases []string `json:"capability_aliases"`
		Path              string   `json:"path"`
		Schema            string   `json:"schema"`
		ProviderFree      bool     `json:"provider_free"`
		LiveNetwork       bool     `json:"live_network"`
		RealImports       bool     `json:"real_dependency_imports"`
	}
	var index struct {
		Fixtures []fixtureIndexRow `json:"fixtures"`
	}
	decodeGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, manifest.Fixtures["index"]), &index)
	fixturesByPath := map[string]fixtureIndexRow{}
	for _, fixture := range index.Fixtures {
		fixturesByPath[fixture.Path] = fixture
	}
	for key, fixturePath := range manifest.Fixtures {
		if key == "index" {
			continue
		}
		fixture, ok := fixturesByPath[fixturePath]
		if !ok {
			t.Fatalf("manifest fixture %q=%q missing from fixture index", key, fixturePath)
		}
		if fixture.Schema != manifest.Schemas[key] {
			t.Fatalf("fixture %q schema = %q, want manifest schema %q", key, fixture.Schema, manifest.Schemas[key])
		}
		if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealImports {
			t.Fatalf("fixture %q provider boundary mismatch: %#v", key, fixture)
		}
	}

	traceHookCapabilities := map[string]bool{}
	for _, hook := range manifest.TraceEmissionHooks {
		traceHookCapabilities[hook.Capability] = true
		if !contains(manifest.Capabilities, hook.Capability) {
			t.Fatalf("manifest capabilities missing trace hook capability %q", hook.Capability)
		}
	}
	traceHookFixture := fixturesByPath[manifest.Fixtures["trace_emission_hooks"]]
	if traceHookFixture.Capability != "generic.ai.workflow.orchestration.trace_hooks" || traceHookFixture.CapabilityScope != "aggregate" {
		t.Fatalf("trace hook fixture must be an aggregate capability: %#v", traceHookFixture)
	}
	if len(traceHookFixture.CapabilityAliases) != len(traceHookCapabilities) {
		t.Fatalf("trace hook aliases = %#v, want capabilities %#v", traceHookFixture.CapabilityAliases, traceHookCapabilities)
	}
	for _, alias := range traceHookFixture.CapabilityAliases {
		if !traceHookCapabilities[alias] {
			t.Fatalf("trace hook alias %q is not declared by manifest trace hooks %#v", alias, manifest.TraceEmissionHooks)
		}
	}

	var contract struct {
		Stages []struct {
			ID             string   `json:"id"`
			OutputSchema   string   `json:"output_schema"`
			RequiredFields []string `json:"required_fields"`
		} `json:"stages"`
	}
	decodeGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, manifest.Entrypoints["workflow_graph_contract"]), &contract)
	for _, stage := range contract.Stages {
		switch stage.OutputSchema {
		case manifest.Schemas["stage_io"]:
			assertGenericWorkflowOrchestratorRequiredSubset(t, "stage_io item required fields", stage.RequiredFields, genericWorkflowOrchestratorSchemaRequired(t, filepath.Join(base, manifest.Schemas["stage_io"]), []string{"properties", "stage_io", "items"}))
		case manifest.Schemas["handoff_trace"]:
			assertGenericWorkflowOrchestratorRequiredSubset(t, "handoff item required fields", stage.RequiredFields, genericWorkflowOrchestratorSchemaRequired(t, filepath.Join(base, manifest.Schemas["handoff_trace"]), []string{"properties", "handoffs", "items"}))
		case manifest.Schemas["workflow_result"]:
			assertGenericWorkflowOrchestratorRequiredSubset(t, "workflow result required fields", stage.RequiredFields, genericWorkflowOrchestratorSchemaRequired(t, filepath.Join(base, manifest.Schemas["workflow_result"]), nil))
		default:
			t.Fatalf("stage %q output schema %q not declared in manifest schemas", stage.ID, stage.OutputSchema)
		}
	}
}

func TestGenericWorkflowOrchestratorFixturesResultAndTraceHooks(t *testing.T) {
	base := genericWorkflowOrchestratorLivePackageDir(t)
	manifest := loadGenericWorkflowOrchestratorManifest(t, base)
	cacheStateAllowed := map[string]bool{}
	for _, state := range manifest.CachePolicy.CacheStates {
		cacheStateAllowed[state] = true
	}

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
	decodeGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 6 {
		t.Fatalf("fixture index header/count = %#v", index)
	}
	cacheStates := map[string]bool{}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Capability == "" || fixture.Path == "" || fixture.Schema == "" {
			t.Fatalf("fixture metadata incomplete: %#v", fixture)
		}
		if fixture.Metadata["replay_ready"] != true || fixture.Metadata["provider_free"] != true || fixture.Metadata["cache_key"] == "" || fixture.Metadata["max_attempts"] != float64(1) {
			t.Fatalf("%s metadata incomplete: %#v", fixture.FixtureKey, fixture.Metadata)
		}
		cacheState, ok := fixture.Metadata["cache_state"].(string)
		if !ok || cacheState == "" {
			t.Fatalf("%s cache_state missing: %#v", fixture.FixtureKey, fixture.Metadata)
		}
		cacheStates[cacheState] = true
		assertGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, fixture.Path))
		assertGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, fixture.Schema))
	}
	for _, want := range []string{"hit", "miss", "bypass"} {
		if !cacheStates[want] {
			t.Fatalf("fixture cache states missing %q: %#v", want, cacheStates)
		}
	}

	var graph struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		WorkflowID   string `json:"workflow_id"`
		Entrypoint   string `json:"entrypoint"`
		Stages       []struct {
			ID           string   `json:"id"`
			DependsOn    []string `json:"depends_on"`
			InputRef     string   `json:"input_ref"`
			OutputRef    string   `json:"output_ref"`
			InputSchema  string   `json:"input_schema"`
			OutputSchema string   `json:"output_schema"`
		} `json:"stages"`
		Edges [][]string `json:"edges"`
	}
	decodeGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, "fixtures", "workflow_graph_fixture.json"), &graph)
	if !graph.ProviderFree || graph.LiveNetwork || graph.WorkflowID == "" || graph.Entrypoint != "ai.workflow.orchestrate" || len(graph.Stages) != 4 || len(graph.Edges) != 3 {
		t.Fatalf("workflow graph fixture incomplete: %#v", graph)
	}
	graphStageOrder := make([]string, 0, len(graph.Stages))
	graphStageByID := map[string]struct {
		ID           string   `json:"id"`
		DependsOn    []string `json:"depends_on"`
		InputRef     string   `json:"input_ref"`
		OutputRef    string   `json:"output_ref"`
		InputSchema  string   `json:"input_schema"`
		OutputSchema string   `json:"output_schema"`
	}{}
	graphOutputRefByStage := map[string]string{}
	for _, stage := range graph.Stages {
		if graphStageByID[stage.ID].ID != "" {
			t.Fatalf("duplicate workflow graph stage id %q", stage.ID)
		}
		graphStageOrder = append(graphStageOrder, stage.ID)
		graphStageByID[stage.ID] = stage
		graphOutputRefByStage[stage.ID] = stage.OutputRef
	}
	if !reflect.DeepEqual(graphStageOrder, []string{"plan", "execute", "handoff", "finalize"}) {
		t.Fatalf("workflow graph fixture stage order = %#v", graphStageOrder)
	}
	graphEdges := map[string]string{}
	for _, edge := range graph.Edges {
		if len(edge) != 2 || graphStageByID[edge[0]].ID == "" || graphStageByID[edge[1]].ID == "" {
			t.Fatalf("workflow graph edge does not reference known stages: %#v", edge)
		}
		graphEdges[edge[0]] = edge[1]
	}

	var stageIO struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		WorkflowID   string `json:"workflow_id"`
		RunID        string `json:"run_id"`
		StageIO      []struct {
			StageID      string         `json:"stage_id"`
			InputRef     string         `json:"input_ref"`
			InputShape   map[string]any `json:"input_shape"`
			OutputRef    string         `json:"output_ref"`
			OutputShape  map[string]any `json:"output_shape"`
			Status       string         `json:"status"`
			FixtureKey   string         `json:"fixture_key"`
			Attempt      int            `json:"attempt"`
			MaxAttempts  int            `json:"max_attempts"`
			CacheKey     string         `json:"cache_key"`
			CacheState   string         `json:"cache_state"`
			TraceEventID string         `json:"trace_event_id"`
		} `json:"stage_io"`
	}
	decodeGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, "fixtures", "stage_io_fixture.json"), &stageIO)
	if !stageIO.ProviderFree || stageIO.LiveNetwork || stageIO.WorkflowID != graph.WorkflowID || stageIO.RunID == "" || len(stageIO.StageIO) != 2 {
		t.Fatalf("stage io fixture incomplete: %#v", stageIO)
	}
	stageIOByStage := map[string]struct {
		StageID      string         `json:"stage_id"`
		InputRef     string         `json:"input_ref"`
		InputShape   map[string]any `json:"input_shape"`
		OutputRef    string         `json:"output_ref"`
		OutputShape  map[string]any `json:"output_shape"`
		Status       string         `json:"status"`
		FixtureKey   string         `json:"fixture_key"`
		Attempt      int            `json:"attempt"`
		MaxAttempts  int            `json:"max_attempts"`
		CacheKey     string         `json:"cache_key"`
		CacheState   string         `json:"cache_state"`
		TraceEventID string         `json:"trace_event_id"`
	}{}
	traceEventIDs := map[string]bool{}
	for _, stage := range stageIO.StageIO {
		if stage.StageID == "" || stage.InputRef == "" || len(stage.InputShape) == 0 || stage.OutputRef == "" || len(stage.OutputShape) == 0 || stage.Status != "completed" || stage.FixtureKey == "" || stage.Attempt != 1 || stage.MaxAttempts != 1 || stage.CacheKey == "" || stage.CacheState == "" || stage.TraceEventID == "" {
			t.Fatalf("stage io row incomplete: %#v", stage)
		}
		graphStage := graphStageByID[stage.StageID]
		if graphStage.ID == "" || stage.InputRef != graphStage.InputRef || stage.OutputRef != graphStage.OutputRef {
			t.Fatalf("stage io row does not correlate with graph stage: row=%#v graph=%#v", stage, graphStage)
		}
		if !strings.Contains(stage.CacheKey, stageIO.RunID) || !strings.Contains(stage.CacheKey, stage.StageID) || !cacheStateAllowed[stage.CacheState] {
			t.Fatalf("stage io retry/cache metadata not tied to run/stage/cache policy: %#v", stage)
		}
		stageIOByStage[stage.StageID] = stage
		traceEventIDs[stage.TraceEventID] = true
	}

	var handoff struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		WorkflowID   string `json:"workflow_id"`
		RunID        string `json:"run_id"`
		Handoffs     []struct {
			FromStage     string `json:"from_stage"`
			ToStage       string `json:"to_stage"`
			PayloadRef    string `json:"payload_ref"`
			PayloadSchema string `json:"payload_schema"`
			Status        string `json:"status"`
			TraceEventID  string `json:"trace_event_id"`
		} `json:"handoffs"`
	}
	decodeGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, "fixtures", "handoff_trace_fixture.json"), &handoff)
	if !handoff.ProviderFree || handoff.LiveNetwork || handoff.WorkflowID != graph.WorkflowID || handoff.RunID != stageIO.RunID || len(handoff.Handoffs) != 3 {
		t.Fatalf("handoff trace fixture incomplete: %#v", handoff)
	}
	for _, row := range handoff.Handoffs {
		if row.FromStage == "" || row.ToStage == "" || row.PayloadRef == "" || row.PayloadSchema == "" || row.Status != "accepted" || row.TraceEventID == "" {
			t.Fatalf("handoff row incomplete: %#v", row)
		}
		if graphEdges[row.FromStage] != row.ToStage || graphOutputRefByStage[row.FromStage] != row.PayloadRef {
			t.Fatalf("handoff row does not correlate with graph edge/output ref: %#v", row)
		}
		traceEventIDs[row.TraceEventID] = true
	}

	var result struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		WorkflowID   string `json:"workflow_id"`
		RunID        string `json:"run_id"`
		Entrypoint   string `json:"entrypoint"`
		Status       string `json:"status"`
		StageResults []struct {
			StageID      string `json:"stage_id"`
			Status       string `json:"status"`
			OutputRef    string `json:"output_ref"`
			TraceEventID string `json:"trace_event_id"`
		} `json:"stage_results"`
		Artifacts []struct {
			ArtifactID string   `json:"artifact_id"`
			Kind       string   `json:"kind"`
			Status     string   `json:"status"`
			SourceRefs []string `json:"source_refs"`
		} `json:"artifacts"`
		TraceRefs []string `json:"trace_refs"`
	}
	decodeGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, "fixtures", "workflow_result_fixture.json"), &result)
	if !result.ProviderFree || result.LiveNetwork || result.WorkflowID != graph.WorkflowID || result.RunID != stageIO.RunID || result.Entrypoint != "ai.workflow.orchestrate" || result.Status != "completed" || len(result.StageResults) != 4 || len(result.Artifacts) != 2 || len(result.TraceRefs) < 9 {
		t.Fatalf("workflow result fixture incomplete: %#v", result)
	}
	resultTraceRefs := map[string]bool{}
	for _, ref := range result.TraceRefs {
		resultTraceRefs[ref] = true
	}
	for _, ref := range []string{"evt_workflow_started", "evt_workflow_completed"} {
		if !resultTraceRefs[ref] {
			t.Fatalf("workflow result trace refs missing lifecycle event %q: %#v", ref, result.TraceRefs)
		}
	}
	if len(result.StageResults) != len(graphStageOrder) {
		t.Fatalf("workflow result stage result count = %d, want graph stage count %d", len(result.StageResults), len(graphStageOrder))
	}
	for i, stage := range result.StageResults {
		if stage.StageID != graphStageOrder[i] || stage.Status != "completed" || stage.OutputRef != graphOutputRefByStage[stage.StageID] || stage.TraceEventID == "" {
			t.Fatalf("workflow result stage row does not correlate with graph order/output: row=%#v graph_order=%#v", stage, graphStageOrder)
		}
		traceEventIDs[stage.TraceEventID] = true
		if !resultTraceRefs[stage.TraceEventID] {
			t.Fatalf("workflow result trace refs missing stage event %q", stage.TraceEventID)
		}
	}
	for eventID := range traceEventIDs {
		if !resultTraceRefs[eventID] {
			t.Fatalf("workflow result trace refs missing correlated event %q", eventID)
		}
	}

	var hooksContract struct {
		ProviderFree              bool   `json:"provider_free"`
		LiveNetwork               bool   `json:"live_network"`
		RealDependencyImports     bool   `json:"real_dependency_imports"`
		TraceEmissionHooksFixture string `json:"trace_emission_hooks_fixture"`
		TraceEmissionHooksSchema  string `json:"trace_emission_hooks_schema"`
		Hooks                     []struct {
			ID             string   `json:"id"`
			Phase          string   `json:"phase"`
			RequiredFields []string `json:"required_fields"`
		} `json:"hooks"`
		AcceptanceGates []string `json:"acceptance_gates"`
	}
	decodeGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, "contracts", "trace_emission_hooks_contract.json"), &hooksContract)
	if !hooksContract.ProviderFree || hooksContract.LiveNetwork || hooksContract.RealDependencyImports || hooksContract.TraceEmissionHooksFixture == "" || hooksContract.TraceEmissionHooksSchema == "" || len(hooksContract.Hooks) != 3 {
		t.Fatalf("trace hooks contract incomplete: %#v", hooksContract)
	}
	assertGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, hooksContract.TraceEmissionHooksFixture))
	assertGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, hooksContract.TraceEmissionHooksSchema))
	joined := strings.ToLower(strings.Join(hooksContract.AcceptanceGates, " "))
	for _, want := range []string{"offline fixture hooks", "no hook imports", "retry/cache", "workflow result"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("trace hooks acceptance gates missing %q: %s", want, joined)
		}
	}

	var hooks struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		WorkflowID   string `json:"workflow_id"`
		RunID        string `json:"run_id"`
		Hooks        []struct {
			ID            string         `json:"id"`
			Phase         string         `json:"phase"`
			EventID       string         `json:"event_id"`
			StageID       string         `json:"stage_id"`
			Status        string         `json:"status"`
			EmissionMode  string         `json:"emission_mode"`
			PayloadRef    string         `json:"payload_ref"`
			ProviderSink  any            `json:"provider_sink"`
			RetryCacheRef map[string]any `json:"retry_cache_ref"`
			TraceRefs     []string       `json:"trace_refs"`
		} `json:"hooks"`
	}
	decodeGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, "fixtures", "trace_emission_hooks_fixture.json"), &hooks)
	if !hooks.ProviderFree || hooks.LiveNetwork || hooks.WorkflowID != graph.WorkflowID || hooks.RunID != result.RunID || len(hooks.Hooks) != 3 {
		t.Fatalf("trace hooks fixture incomplete: %#v", hooks)
	}
	for _, hook := range hooks.Hooks {
		if hook.ID == "" || hook.Phase == "" || hook.EventID == "" || hook.EmissionMode != "fixture_append_only" || hook.PayloadRef == "" || hook.ProviderSink != nil {
			t.Fatalf("trace hook row incomplete: %#v", hook)
		}
		if !resultTraceRefs[hook.EventID] {
			t.Fatalf("trace hook event %q is not present in workflow result trace refs", hook.EventID)
		}
		if hook.Phase == "stage_completed" {
			stage := stageIOByStage[hook.StageID]
			if stage.StageID == "" || hook.Status != stage.Status || hook.PayloadRef != stage.OutputRef || hook.EventID != stage.TraceEventID {
				t.Fatalf("stage trace hook does not correlate with stage IO: hook=%#v stage=%#v", hook, stage)
			}
			if hook.RetryCacheRef["attempt"] != float64(stage.Attempt) ||
				hook.RetryCacheRef["max_attempts"] != float64(stage.MaxAttempts) ||
				hook.RetryCacheRef["cache_key"] != stage.CacheKey ||
				hook.RetryCacheRef["cache_state"] != stage.CacheState {
				t.Fatalf("stage trace hook retry/cache ref does not match stage IO: hook=%#v stage=%#v", hook.RetryCacheRef, stage)
			}
		}
		if hook.Phase == "workflow_completed" {
			if hook.Status != result.Status || hook.PayloadRef != "workflow_result" || len(hook.TraceRefs) == 0 {
				t.Fatalf("workflow completed hook does not correlate with workflow result: %#v", hook)
			}
			for _, ref := range hook.TraceRefs {
				if !resultTraceRefs[ref] {
					t.Fatalf("workflow completed hook trace ref %q is not present in workflow result refs", ref)
				}
			}
		}
	}
	assertGenericWorkflowOrchestratorNoSecretMarkers(t, base)
}

func TestGenericWorkflowOrchestratorPlanningGraphStageProjection(t *testing.T) {
	root := repoRoot(t)
	base := genericWorkflowOrchestratorLivePackageDir(t)
	planningContract := loadGenericPlanningGraphContract(t, root)
	planningTrace := loadGenericPlanningGraphTraceFixture(t, root)

	var projection struct {
		FixtureKey            string `json:"fixture_key"`
		Capability            string `json:"capability"`
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		SourcePackage         string `json:"source_package"`
		TargetPackage         string `json:"target_package"`
		SourceRefs            struct {
			PlanningGraphContract string `json:"planning_graph_contract"`
			PlanningTraceFixture  string `json:"planning_trace_fixture"`
			WorkflowGraphFixture  string `json:"workflow_graph_fixture"`
			StageIOFixture        string `json:"stage_io_fixture"`
			HandoffTraceFixture   string `json:"handoff_trace_fixture"`
			WorkflowResultFixture string `json:"workflow_result_fixture"`
		} `json:"source_refs"`
		WorkflowID    string `json:"workflow_id"`
		RunID         string `json:"run_id"`
		StageMappings []struct {
			StageID         string   `json:"stage_id"`
			SourceNodeIDs   []string `json:"source_node_ids"`
			SourceNodeTypes []string `json:"source_node_types"`
			InputRef        string   `json:"input_ref"`
			OutputRef       string   `json:"output_ref"`
			StageIORef      string   `json:"stage_io_ref"`
			TraceEventRefs  []string `json:"trace_event_refs"`
			RetryProjection struct {
				Retryable          bool     `json:"retryable"`
				MaxAttempts        int      `json:"max_attempts"`
				AttemptEventRefs   []string `json:"attempt_event_refs"`
				WorkflowCacheState string   `json:"workflow_cache_state"`
			} `json:"retry_projection"`
			BranchProjection struct {
				BranchNodeID  string   `json:"branch_node_id"`
				BranchTargets []string `json:"branch_targets"`
			} `json:"branch_projection"`
			MergeProjection struct {
				MergeNodeID    string   `json:"merge_node_id"`
				RequiredInputs []string `json:"required_inputs"`
				Mode           string   `json:"mode"`
			} `json:"merge_projection"`
		} `json:"stage_mappings"`
		EdgeMappings []struct {
			SourceEdgeType string   `json:"source_edge_type"`
			SourceEdges    []string `json:"source_edges"`
			WorkflowEdge   string   `json:"workflow_edge"`
			HandoffRef     string   `json:"handoff_ref"`
		} `json:"edge_mappings"`
		EvidenceMappings []struct {
			EvidenceKind      string   `json:"evidence_kind"`
			SourceEventRefs   []string `json:"source_event_refs"`
			WorkflowTraceRefs []string `json:"workflow_trace_refs"`
		} `json:"evidence_mappings"`
		ProjectionAssertions map[string]bool `json:"projection_assertions"`
	}
	decodeGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, "fixtures", "planning_graph_stage_projection_fixture.json"), &projection)
	if projection.FixtureKey == "" || projection.Capability != "generic.ai.workflow.orchestration.planning_graph_stage_projection" ||
		!projection.ProviderFree || projection.LiveNetwork || projection.RealDependencyImports ||
		projection.SourcePackage != "generic_planning_graph" || projection.TargetPackage != "generic_workflow_orchestrator" {
		t.Fatalf("projection header/provider boundary mismatch: %#v", projection)
	}
	for _, ref := range []string{
		projection.SourceRefs.PlanningGraphContract,
		projection.SourceRefs.PlanningTraceFixture,
		projection.SourceRefs.WorkflowGraphFixture,
		projection.SourceRefs.StageIOFixture,
		projection.SourceRefs.HandoffTraceFixture,
		projection.SourceRefs.WorkflowResultFixture,
	} {
		if ref == "" {
			t.Fatalf("projection source refs incomplete: %#v", projection.SourceRefs)
		}
	}
	if projection.WorkflowID != "generic_orchestration_graph_v1" || projection.RunID != "run_generic_orchestration_fixture_001" || len(projection.StageMappings) != 4 {
		t.Fatalf("projection workflow identity/stage count mismatch: %#v", projection)
	}

	contractNodeIDs := map[string]string{}
	contractTraceRefs := map[string]string{}
	for _, node := range planningContract.PlanNodes {
		contractNodeIDs[node.ID] = node.NodeType
		for _, eventID := range node.TraceEvidence {
			contractTraceRefs[eventID] = node.ID
		}
	}
	traceEventKinds := map[string]string{}
	for _, event := range planningTrace.Events {
		traceEventKinds[event.EventID] = event.EvidenceKind
	}
	mappedNodes := map[string]bool{}
	mappedTraceEvents := map[string]bool{}
	stageIDs := map[string]bool{}
	for _, mapping := range projection.StageMappings {
		if mapping.StageID == "" || mapping.InputRef == "" || mapping.OutputRef == "" || len(mapping.SourceNodeIDs) == 0 || len(mapping.TraceEventRefs) == 0 {
			t.Fatalf("projection stage mapping incomplete: %#v", mapping)
		}
		stageIDs[mapping.StageID] = true
		if len(mapping.SourceNodeIDs) != len(mapping.SourceNodeTypes) {
			t.Fatalf("%s source node id/type count mismatch: %#v", mapping.StageID, mapping)
		}
		for i, nodeID := range mapping.SourceNodeIDs {
			nodeType := contractNodeIDs[nodeID]
			if nodeType == "" || mapping.SourceNodeTypes[i] != nodeType {
				t.Fatalf("%s maps unknown or mismatched node %q/%q, contract=%#v", mapping.StageID, nodeID, mapping.SourceNodeTypes[i], contractNodeIDs)
			}
			mappedNodes[nodeID] = true
		}
		for _, eventID := range mapping.TraceEventRefs {
			if contractTraceRefs[eventID] == "" && !strings.HasPrefix(eventID, "evt_") {
				t.Fatalf("%s maps unknown trace event %q", mapping.StageID, eventID)
			}
			mappedTraceEvents[eventID] = true
		}
	}
	if !stageIDs["plan"] || !stageIDs["execute"] || !stageIDs["handoff"] || !stageIDs["finalize"] {
		t.Fatalf("projection stage ids incomplete: %#v", stageIDs)
	}
	for nodeID := range contractNodeIDs {
		if !mappedNodes[nodeID] {
			t.Fatalf("planning node %q missing from workflow projection", nodeID)
		}
	}
	for eventID := range contractTraceRefs {
		if !mappedTraceEvents[eventID] {
			t.Fatalf("planning trace event %q missing from workflow projection", eventID)
		}
	}
	if len(projection.EdgeMappings) != 4 {
		t.Fatalf("projection edge mapping count = %d, want 4", len(projection.EdgeMappings))
	}
	contractEdges := map[string]string{}
	for _, edge := range planningContract.Edges {
		contractEdges[edge.From+"->"+edge.To] = edge.EdgeType
	}
	mappedEdgeCount := 0
	for _, mapping := range projection.EdgeMappings {
		if mapping.SourceEdgeType == "" || mapping.WorkflowEdge == "" || mapping.HandoffRef == "" || len(mapping.SourceEdges) == 0 {
			t.Fatalf("projection edge mapping incomplete: %#v", mapping)
		}
		for _, edge := range mapping.SourceEdges {
			if contractEdges[edge] != mapping.SourceEdgeType {
				t.Fatalf("edge %q maps as %q, contract type is %q", edge, mapping.SourceEdgeType, contractEdges[edge])
			}
			mappedEdgeCount++
		}
	}
	if mappedEdgeCount != len(planningContract.Edges) {
		t.Fatalf("mapped edge count = %d, want %d", mappedEdgeCount, len(planningContract.Edges))
	}
	evidenceKinds := map[string]bool{}
	for _, mapping := range projection.EvidenceMappings {
		if mapping.EvidenceKind == "" || len(mapping.SourceEventRefs) == 0 || len(mapping.WorkflowTraceRefs) == 0 {
			t.Fatalf("evidence mapping incomplete: %#v", mapping)
		}
		evidenceKinds[mapping.EvidenceKind] = true
		for _, eventID := range mapping.SourceEventRefs {
			if traceEventKinds[eventID] != mapping.EvidenceKind {
				t.Fatalf("evidence event %q maps as %q, trace fixture kind is %q", eventID, mapping.EvidenceKind, traceEventKinds[eventID])
			}
		}
	}
	for _, want := range planningContract.Semantics.TraceEvidence.EvidenceKinds {
		if !evidenceKinds[want] {
			t.Fatalf("projection evidence mappings missing %q", want)
		}
	}
	for _, want := range []string{
		"all_source_plan_nodes_mapped",
		"all_source_dependency_edges_mapped",
		"all_retry_attempts_visible_in_stage_metadata",
		"branch_targets_preserved",
		"merge_required_inputs_preserved",
		"workflow_stage_ids_are_not_assumed_equal_to_plan_node_ids",
		"projection_is_provider_free",
	} {
		if !projection.ProjectionAssertions[want] {
			t.Fatalf("projection assertion missing %q: %#v", want, projection.ProjectionAssertions)
		}
	}
}

func TestGenericWorkflowOrchestratorLivePackageNoLiveImports(t *testing.T) {
	base := genericWorkflowOrchestratorLivePackageDir(t)
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
			`(?m)^\s*(openai|anthropic|autogen|langchain|requests|http|grpc|redis|kafka|sentry|otel|opentelemetry)\s*[.(]`,
		} {
			if regexp.MustCompile(pattern).FindString(source) != "" {
				t.Fatalf("%s contains live dependency loader matching %q", path, pattern)
			}
		}
	}
}

func TestGenericWorkflowOrchestratorLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericWorkflowOrchestratorLivePackageDir(t), "main.leia")
	want := "generic_workflow_orchestrator_live_package package=generic.ai.workflow.orchestration entrypoint=ai.workflow.orchestrate stages=4 fixtures=6 schemas=7 provider_free=true live_network=false imports=false graph=true stage_io=true planning_projection=true handoff_trace=true retry_cache=true workflow_result=true trace_hooks=true"
	for _, result := range runFinRobotLivePackageSummarySmoke(t, path, "generic_workflow_orchestrator_live_package_summary", "generic_workflow_orchestrator_live_package", leia.LibString) {
		if result.Summary != want {
			t.Fatalf("generic_workflow_orchestrator_live_package_summary = %#v, want %#v", result.Summary, want)
		}
	}
}

func genericWorkflowOrchestratorLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_workflow_orchestrator")
}

func loadGenericWorkflowOrchestratorManifest(t *testing.T, base string) genericWorkflowOrchestratorManifest {
	t.Helper()
	var manifest genericWorkflowOrchestratorManifest
	decodeGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertGenericWorkflowOrchestratorJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeGenericWorkflowOrchestratorJSONFile(t, path, &value)
}

func assertGenericWorkflowOrchestratorJSONOrLeiaFile(t *testing.T, path string) {
	t.Helper()
	if strings.HasSuffix(path, ".leia") {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
		return
	}
	assertGenericWorkflowOrchestratorJSONFile(t, path)
}

func decodeGenericWorkflowOrchestratorJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func assertGenericWorkflowOrchestratorSchemaRequired(t *testing.T, path string, cursor []string, wants []string) {
	t.Helper()
	required := genericWorkflowOrchestratorSchemaRequired(t, path, cursor)
	for _, want := range wants {
		if !contains(required, want) {
			t.Fatalf("%s required at %v missing %q: %#v", path, cursor, want, required)
		}
	}
}

func genericWorkflowOrchestratorSchemaRequired(t *testing.T, path string, cursor []string) []string {
	t.Helper()
	var schema map[string]any
	decodeGenericWorkflowOrchestratorJSONFile(t, path, &schema)
	node := any(schema)
	for _, segment := range cursor {
		object, ok := node.(map[string]any)
		if !ok {
			t.Fatalf("%s cursor %v segment %q is not an object: %#v", path, cursor, segment, node)
		}
		node = object[segment]
		if node == nil {
			t.Fatalf("%s missing schema cursor %v at segment %q", path, cursor, segment)
		}
	}
	object, ok := node.(map[string]any)
	if !ok {
		t.Fatalf("%s schema cursor %v is not an object: %#v", path, cursor, node)
	}
	raw, ok := object["required"].([]any)
	if !ok {
		t.Fatalf("%s schema cursor %v missing required array: %#v", path, cursor, object)
	}
	required := make([]string, 0, len(raw))
	for _, value := range raw {
		item, ok := value.(string)
		if !ok || item == "" {
			t.Fatalf("%s schema required item at %v must be a non-empty string: %#v", path, cursor, value)
		}
		required = append(required, item)
	}
	return required
}

func assertGenericWorkflowOrchestratorRequiredSubset(t *testing.T, label string, subset []string, required []string) {
	t.Helper()
	for _, field := range subset {
		if !contains(required, field) {
			t.Fatalf("%s missing contract field %q in schema required %#v", label, field, required)
		}
	}
}

func assertGenericWorkflowOrchestratorNoSecretMarkers(t *testing.T, base string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?i)(api[_-]?key\s*[:=]\s*["']?[a-z0-9._-]{8,}|secret[_-]?key\s*[:=]\s*["']?[a-z0-9._-]{8,}|token\s*[:=]\s*["']?[a-z0-9._-]{12,}|bearer\s+[a-z0-9._-]+|sk-[a-z0-9]{12,})`)
	if err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range pattern.FindAllString(string(data), -1) {
			t.Fatalf("%s contains secret-like marker %q", path, match)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
