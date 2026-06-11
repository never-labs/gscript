package leia_test

import (
	"encoding/json"
	"fmt"
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
	for _, key := range []string{"workflow_graph", "stage_io", "handoff_trace", "retry_cache_policy", "workflow_result", "trace_emission_hooks"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertGenericWorkflowOrchestratorJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "workflow_graph", "stage_io", "handoff_trace", "workflow_result", "trace_emission_hooks"} {
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
	for _, want := range []string{"generic.ai.workflow.orchestration", "ai.workflow.orchestrate", "workflow graph", "stage inputs", "handoff trace", "retry/cache", "workflow result", "trace emission hooks"} {
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
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 5 {
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
			got, err := vm.Get("generic_workflow_orchestrator_live_package_summary")
			if err != nil {
				t.Fatalf("Get generic_workflow_orchestrator_live_package_summary: %v", err)
			}
			want := "generic_workflow_orchestrator_live_package package=generic.ai.workflow.orchestration entrypoint=ai.workflow.orchestrate stages=4 fixtures=5 schemas=6 provider_free=true live_network=false imports=false graph=true stage_io=true handoff_trace=true retry_cache=true workflow_result=true trace_hooks=true"
			if got != want {
				t.Fatalf("generic_workflow_orchestrator_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
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
