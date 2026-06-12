package leia_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

const genericPlanningGraphPackageDir = "examples/ai/finrobot_translation/live_packages/generic_planning_graph"

type genericPlanningGraphContract struct {
	SchemaVersion         int    `json:"schema_version"`
	ID                    string `json:"id"`
	PackageManifestID     string `json:"package_manifest_id"`
	PackageName           string `json:"package_name"`
	ProviderFree          bool   `json:"provider_free"`
	DomainSpecific        bool   `json:"domain_specific"`
	LiveNetwork           bool   `json:"live_network"`
	LiveModel             bool   `json:"live_model"`
	RealDependencyImports bool   `json:"real_dependency_imports"`
	SchemaRefs            struct {
		PlanningGraph string `json:"planning_graph"`
		PlanningTrace string `json:"planning_trace"`
	} `json:"schema_refs"`
	FixtureIndexRef string   `json:"fixture_index_ref"`
	CapabilityRefs  []string `json:"capability_refs"`
	Semantics       struct {
		PlanNode struct {
			RequiredFields  []string `json:"required_fields"`
			AllowedNodeType []string `json:"allowed_node_types"`
		} `json:"plan_node"`
		Dependency struct {
			RequiredFields  []string `json:"required_fields"`
			AllowedEdgeType []string `json:"allowed_edge_types"`
			Acyclic         bool     `json:"acyclic"`
		} `json:"dependency"`
		Retry struct {
			RequiredFields []string `json:"required_fields"`
			ProviderFree   bool     `json:"provider_free"`
			AllowedBackoff []string `json:"allowed_backoff"`
		} `json:"retry"`
		TraceEvidence struct {
			RequiredFields []string `json:"required_fields"`
			EvidenceKinds  []string `json:"evidence_kinds"`
		} `json:"trace_evidence"`
	} `json:"semantics"`
	PlanNodes []struct {
		ID          string   `json:"id"`
		NodeType    string   `json:"node_type"`
		DependsOn   []string `json:"depends_on"`
		Branches    []string `json:"branches"`
		MergePolicy struct {
			Mode           string   `json:"mode"`
			RequiredInputs []string `json:"required_inputs"`
		} `json:"merge_policy"`
		RetryPolicy struct {
			MaxAttempts int    `json:"max_attempts"`
			Retryable   bool   `json:"retryable"`
			Backoff     string `json:"backoff"`
		} `json:"retry_policy"`
		TraceEvidence []string `json:"trace_evidence"`
	} `json:"plan_nodes"`
	Edges []struct {
		From     string `json:"from"`
		To       string `json:"to"`
		EdgeType string `json:"edge_type"`
		BranchID string `json:"branch_id"`
	} `json:"edges"`
	AcceptanceGates []string `json:"acceptance_gates"`
}

type genericPlanningGraphManifest struct {
	SchemaVersion               int               `json:"schema_version"`
	ID                          string            `json:"id"`
	PackageName                 string            `json:"package_name"`
	ProviderFree                bool              `json:"provider_free"`
	DomainSpecific              bool              `json:"domain_specific"`
	LiveNetworkDefault          bool              `json:"live_network_default"`
	LiveModelDefault            bool              `json:"live_model_default"`
	RealDependencyImportDefault bool              `json:"real_dependency_import_default"`
	Entrypoints                 map[string]string `json:"entrypoints"`
	Schemas                     map[string]string `json:"schemas"`
	Fixtures                    map[string]string `json:"fixtures"`
	Capabilities                []string          `json:"capabilities"`
	DefaultPolicy               struct {
		Mode                        string `json:"mode"`
		ProviderFree                bool   `json:"provider_free"`
		LiveNetwork                 bool   `json:"live_network"`
		LiveModel                   bool   `json:"live_model"`
		ProviderCredentialsRequired bool   `json:"provider_credentials_required"`
		RealDependencyImports       bool   `json:"real_dependency_imports"`
		CleanSkipWithoutDependency  bool   `json:"clean_skip_without_dependency"`
		FixtureHook                 string `json:"fixture_hook"`
	} `json:"default_policy"`
}

type genericPlanningGraphFixtureIndex struct {
	SchemaVersion         int      `json:"schema_version"`
	ID                    string   `json:"id"`
	ProviderFree          bool     `json:"provider_free"`
	DomainSpecific        bool     `json:"domain_specific"`
	LiveNetwork           bool     `json:"live_network"`
	LiveModel             bool     `json:"live_model"`
	RealDependencyImports bool     `json:"real_dependency_imports"`
	PackageManifestID     string   `json:"package_manifest_id"`
	ContractID            string   `json:"contract_id"`
	Capabilities          []string `json:"capabilities"`
	Fixtures              []struct {
		ID                    string          `json:"id"`
		Path                  string          `json:"path"`
		SchemaRef             string          `json:"schema_ref"`
		ContractID            string          `json:"contract_id"`
		Capabilities          []string        `json:"capabilities"`
		ProviderFree          bool            `json:"provider_free"`
		LiveNetwork           bool            `json:"live_network"`
		RealDependencyImports bool            `json:"real_dependency_imports"`
		Metadata              map[string]bool `json:"metadata"`
		Contract              string          `json:"contract"`
		Covers                []string        `json:"covers"`
	} `json:"fixtures"`
}

type genericPlanningGraphTraceFixture struct {
	SchemaVersion         int    `json:"schema_version"`
	FixtureKey            string `json:"fixture_key"`
	ContractID            string `json:"contract_id"`
	ProviderFree          bool   `json:"provider_free"`
	DomainSpecific        bool   `json:"domain_specific"`
	LiveNetwork           bool   `json:"live_network"`
	LiveModel             bool   `json:"live_model"`
	RealDependencyImports bool   `json:"real_dependency_imports"`
	RunID                 string `json:"run_id"`
	Events                []struct {
		TraceID               string   `json:"trace_id"`
		EventID               string   `json:"event_id"`
		EvidenceKind          string   `json:"evidence_kind"`
		PlanNodeID            string   `json:"plan_node_id"`
		Sequence              int      `json:"sequence"`
		Status                string   `json:"status"`
		ProviderFree          bool     `json:"provider_free"`
		LiveModel             bool     `json:"live_model"`
		Attempt               int      `json:"attempt"`
		Branches              []string `json:"branches"`
		MergedInputs          []string `json:"merged_inputs"`
		DependenciesSatisfied []string `json:"dependencies_satisfied"`
		RetryMetadata         struct {
			MaxAttempts int    `json:"max_attempts"`
			Retryable   bool   `json:"retryable"`
			Backoff     string `json:"backoff"`
			BackoffMS   int    `json:"backoff_ms"`
			Reason      string `json:"reason"`
		} `json:"retry_metadata"`
	} `json:"events"`
	Summary struct {
		PlanNodeCount     int    `json:"plan_node_count"`
		EdgeCount         int    `json:"edge_count"`
		RetryAttemptCount int    `json:"retry_attempt_count"`
		BranchCount       int    `json:"branch_count"`
		MergeCount        int    `json:"merge_count"`
		FinalStatus       string `json:"final_status"`
	} `json:"summary"`
}

func TestGenericPlanningGraphLivePackageManifestContractAndFixtureIndexStayConsistent(t *testing.T) {
	root := repoRoot(t)
	manifest := loadGenericPlanningGraphManifest(t, root)
	contract := loadGenericPlanningGraphContract(t, root)
	index := loadGenericPlanningGraphFixtureIndex(t, root)

	assertGenericPlanningGraphProviderFree(t, root, "package.manifest.json", manifest.ProviderFree, manifest.DomainSpecific, manifest.LiveNetworkDefault, manifest.LiveModelDefault, manifest.RealDependencyImportDefault)
	assertGenericPlanningGraphProviderFree(t, root, "fixtures/provider_free_fixture_index.json", index.ProviderFree, index.DomainSpecific, index.LiveNetwork, index.LiveModel, index.RealDependencyImports)

	if manifest.SchemaVersion != 1 || manifest.ID != "generic-planning-graph-live-package" || manifest.PackageName != "leia-generic-ai-planning-graph" {
		t.Fatalf("manifest header mismatch: %#v", manifest)
	}
	if manifest.Entrypoints["smoke"] != "main.leia" || manifest.Entrypoints["main"] != "main.leia" {
		t.Fatalf("manifest smoke/main entrypoints must both target main.leia: %#v", manifest.Entrypoints)
	}
	for _, rel := range []string{
		manifest.Entrypoints["smoke"],
		manifest.Entrypoints["planning_graph_contract"],
		manifest.Entrypoints["trace_fixture"],
		manifest.Entrypoints["fixture_index"],
		manifest.Schemas["planning_graph"],
		manifest.Schemas["planning_trace"],
		manifest.Fixtures["index"],
		manifest.Fixtures["trace"],
	} {
		assertGenericPlanningGraphPackageFileExists(t, root, rel)
	}
	if manifest.Entrypoints["planning_graph_contract"] != "contracts/planning_graph_contract.json" ||
		manifest.Entrypoints["fixture_index"] != manifest.Fixtures["index"] ||
		manifest.Entrypoints["trace_fixture"] != manifest.Fixtures["trace"] {
		t.Fatalf("manifest entrypoints and fixture aliases diverged: entrypoints=%#v fixtures=%#v", manifest.Entrypoints, manifest.Fixtures)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		!manifest.DefaultPolicy.ProviderFree ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.LiveModel ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_generic_planning_graph_fixture" {
		t.Fatalf("manifest default policy must stay provider-free fixture replay: %#v", manifest.DefaultPolicy)
	}

	if contract.PackageManifestID != manifest.ID || contract.PackageName != manifest.PackageName {
		t.Fatalf("contract does not point back to manifest/package: contract=%#v manifest=%#v", contract, manifest)
	}
	if contract.SchemaRefs.PlanningGraph != manifest.Schemas["planning_graph"] ||
		contract.SchemaRefs.PlanningTrace != manifest.Schemas["planning_trace"] ||
		contract.FixtureIndexRef != manifest.Fixtures["index"] {
		t.Fatalf("contract refs diverge from manifest: schema_refs=%#v fixture_index=%q manifest=%#v", contract.SchemaRefs, contract.FixtureIndexRef, manifest)
	}
	assertGenericPlanningGraphStringSet(t, "manifest/contract capabilities", contract.CapabilityRefs, manifest.Capabilities)

	if index.PackageManifestID != manifest.ID || index.ContractID != contract.ID {
		t.Fatalf("fixture index header does not link manifest and contract: %#v", index)
	}
	assertGenericPlanningGraphStringSet(t, "fixture index capabilities", index.Capabilities, manifest.Capabilities)
	if len(index.Fixtures) != 1 {
		t.Fatalf("fixture index fixtures = %d, want 1", len(index.Fixtures))
	}
	fixture := index.Fixtures[0]
	if fixture.ID != "planning_graph_trace" ||
		fixture.Path != manifest.Fixtures["trace"] ||
		fixture.SchemaRef != manifest.Schemas["planning_trace"] ||
		fixture.Contract != manifest.Entrypoints["planning_graph_contract"] ||
		fixture.ContractID != contract.ID {
		t.Fatalf("fixture index trace entry diverges from manifest/contract: %#v", fixture)
	}
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports ||
		!fixture.Metadata["provider_free"] || fixture.Metadata["live_network"] || fixture.Metadata["real_dependency_imports"] {
		t.Fatalf("fixture index trace entry is not provider-free: %#v", fixture)
	}
	assertGenericPlanningGraphStringSet(t, "fixture index fixture capabilities", fixture.Capabilities, manifest.Capabilities)
	assertGenericPlanningGraphStringSet(t, "fixture index semantic coverage", fixture.Covers, []string{"branch", "dependency", "merge", "plan_node", "retry", "trace_evidence"})
}

func TestGenericPlanningGraphContractCoversPlanDependencyRetryBranchMergeSemantics(t *testing.T) {
	root := repoRoot(t)
	contract := loadGenericPlanningGraphContract(t, root)
	assertGenericPlanningGraphProviderFree(t, root, "contracts/planning_graph_contract.json", contract.ProviderFree, contract.DomainSpecific, contract.LiveNetwork, contract.LiveModel, contract.RealDependencyImports)

	if contract.SchemaVersion != 1 || contract.ID != "generic-planning-graph-contract-v1" {
		t.Fatalf("unexpected contract header: %#v", contract)
	}
	assertGenericPlanningGraphStringSet(t, "plan node fields", contract.Semantics.PlanNode.RequiredFields, []string{"depends_on", "id", "node_type", "retry_policy", "trace_evidence"})
	assertGenericPlanningGraphStringSet(t, "node types", contract.Semantics.PlanNode.AllowedNodeType, []string{"artifact", "branch", "decision", "input", "merge", "transform"})
	assertGenericPlanningGraphStringSet(t, "edge types", contract.Semantics.Dependency.AllowedEdgeType, []string{"branch", "control", "data", "merge"})
	if !contract.Semantics.Dependency.Acyclic || !contract.Semantics.Retry.ProviderFree {
		t.Fatalf("generic graph must require acyclic provider-free retry semantics: %#v", contract.Semantics)
	}
	assertGenericPlanningGraphStringSet(t, "trace evidence kinds", contract.Semantics.TraceEvidence.EvidenceKinds, []string{
		"plan.branch.selected",
		"plan.merge.completed",
		"plan.node.completed",
		"plan.node.started",
		"plan.retry.attempt",
	})

	nodes := map[string]struct {
		nodeType string
		depends  []string
	}{}
	traceRefs := map[string]string{}
	for _, node := range contract.PlanNodes {
		if node.ID == "" || node.NodeType == "" || node.RetryPolicy.MaxAttempts == 0 || len(node.TraceEvidence) == 0 {
			t.Fatalf("incomplete plan node: %#v", node)
		}
		nodes[node.ID] = struct {
			nodeType string
			depends  []string
		}{nodeType: node.NodeType, depends: node.DependsOn}
		for _, eventID := range node.TraceEvidence {
			traceRefs[eventID] = node.ID
		}
	}
	if len(nodes) != 7 || len(contract.Edges) != 7 {
		t.Fatalf("graph size nodes=%d edges=%d", len(nodes), len(contract.Edges))
	}
	assertGenericPlanningGraphAcyclic(t, nodes, contract.Edges)
	assertGenericPlanningGraphBranchMerge(t, contract)
	if traceRefs["evt-expand-plan-attempt-1"] != "expand_plan" || traceRefs["evt-merge-checks-completed"] != "merge_checks" {
		t.Fatalf("retry/merge trace evidence refs missing: %#v", traceRefs)
	}
}

func TestGenericPlanningGraphSchemasRequireContractAndTraceEnvelope(t *testing.T) {
	root := repoRoot(t)
	graphSchema := loadGenericPlanningGraphSchema(t, root, "schemas/planning_graph.schema.json")
	traceSchema := loadGenericPlanningGraphSchema(t, root, "schemas/planning_trace.schema.json")

	assertGenericPlanningGraphRequiredContains(t, "planning graph root", graphSchema, []string{
		"schema_version",
		"id",
		"package_manifest_id",
		"package_name",
		"provider_free",
		"domain_specific",
		"live_network",
		"live_model",
		"real_dependency_imports",
		"schema_refs",
		"fixture_index_ref",
		"capability_refs",
		"semantics",
		"plan_nodes",
		"edges",
		"acceptance_gates",
	})
	graphProperties := genericPlanningGraphObject(t, graphSchema, "properties")
	planNodes := genericPlanningGraphObject(t, graphProperties, "plan_nodes")
	planNodeItems := genericPlanningGraphObject(t, planNodes, "items")
	assertGenericPlanningGraphRequiredContains(t, "planning graph plan node items", planNodeItems, []string{"id", "node_type", "depends_on", "retry_policy", "trace_evidence"})
	retryPolicy := genericPlanningGraphObject(t, genericPlanningGraphObject(t, planNodeItems, "properties"), "retry_policy")
	assertGenericPlanningGraphRequiredContains(t, "planning graph retry policy", retryPolicy, []string{"max_attempts", "retryable", "backoff"})
	edgeItems := genericPlanningGraphObject(t, genericPlanningGraphObject(t, graphProperties, "edges"), "items")
	assertGenericPlanningGraphRequiredContains(t, "planning graph edge items", edgeItems, []string{"from", "to", "edge_type"})
	semantics := genericPlanningGraphObject(t, graphProperties, "semantics")
	assertGenericPlanningGraphRequiredContains(t, "planning graph semantics", semantics, []string{"plan_node", "dependency", "retry", "trace_evidence"})

	assertGenericPlanningGraphRequiredContains(t, "planning trace root", traceSchema, []string{
		"schema_version",
		"fixture_key",
		"contract_id",
		"provider_free",
		"domain_specific",
		"live_network",
		"live_model",
		"real_dependency_imports",
		"run_id",
		"events",
		"summary",
	})
	traceProperties := genericPlanningGraphObject(t, traceSchema, "properties")
	eventItems := genericPlanningGraphObject(t, genericPlanningGraphObject(t, traceProperties, "events"), "items")
	assertGenericPlanningGraphRequiredContains(t, "planning trace events", eventItems, []string{"trace_id", "event_id", "evidence_kind", "plan_node_id", "sequence", "status", "provider_free", "live_model"})
	assertGenericPlanningGraphRequiredContains(t, "planning trace summary", genericPlanningGraphObject(t, traceProperties, "summary"), []string{"plan_node_count", "edge_count", "retry_attempt_count", "branch_count", "merge_count", "final_status"})
}

func TestGenericPlanningGraphTraceFixtureMatchesContractEvidence(t *testing.T) {
	root := repoRoot(t)
	contract := loadGenericPlanningGraphContract(t, root)
	fixture := loadGenericPlanningGraphTraceFixture(t, root)
	assertGenericPlanningGraphProviderFree(t, root, "fixtures/planning_graph_trace_fixture.json", fixture.ProviderFree, fixture.DomainSpecific, fixture.LiveNetwork, fixture.LiveModel, fixture.RealDependencyImports)

	if fixture.ContractID != contract.ID || fixture.RunID == "" || fixture.Summary.FinalStatus != "completed" {
		t.Fatalf("trace fixture header mismatch: %#v", fixture)
	}
	if fixture.Summary.PlanNodeCount != len(contract.PlanNodes) || fixture.Summary.EdgeCount != len(contract.Edges) {
		t.Fatalf("trace summary does not match contract: %#v", fixture.Summary)
	}

	nodeTraceRefs := map[string]string{}
	for _, node := range contract.PlanNodes {
		for _, eventID := range node.TraceEvidence {
			nodeTraceRefs[eventID] = node.ID
		}
	}
	seenKinds := map[string]bool{}
	seenEvents := map[string]bool{}
	retryAttempts := map[int]bool{}
	var lastSequence int
	for _, event := range fixture.Events {
		if event.TraceID == "" || event.EventID == "" || event.PlanNodeID == "" || event.EvidenceKind == "" || event.Status == "" {
			t.Fatalf("trace event missing required evidence envelope fields: %#v", event)
		}
		if !event.ProviderFree || event.LiveModel {
			t.Fatalf("trace event is not provider-free: %#v", event)
		}
		if event.Sequence <= lastSequence {
			t.Fatalf("trace event sequence is unstable: %#v after %d", event, lastSequence)
		}
		lastSequence = event.Sequence
		wantNode := nodeTraceRefs[event.EventID]
		if wantNode == "" || wantNode != event.PlanNodeID {
			t.Fatalf("trace event %q maps to node %q, want %q", event.EventID, event.PlanNodeID, wantNode)
		}
		seenEvents[event.EventID] = true
		seenKinds[event.EvidenceKind] = true
		if event.EvidenceKind == "plan.retry.attempt" {
			if event.Attempt == 0 || event.RetryMetadata.MaxAttempts != 2 || event.RetryMetadata.Backoff != "fixed_fixture_ms" {
				t.Fatalf("retry evidence is incomplete: %#v", event)
			}
			retryAttempts[event.Attempt] = true
		}
		if event.EvidenceKind == "plan.branch.selected" {
			assertGenericPlanningGraphStringSet(t, "branch evidence", event.Branches, []string{"collect_evidence", "validate_dependencies"})
		}
		if event.EvidenceKind == "plan.merge.completed" {
			assertGenericPlanningGraphStringSet(t, "merge evidence", event.MergedInputs, []string{"collect_evidence", "validate_dependencies"})
		}
	}
	for eventID := range nodeTraceRefs {
		if !seenEvents[eventID] {
			t.Fatalf("contract trace evidence %q missing from fixture", eventID)
		}
	}
	if !retryAttempts[1] || !retryAttempts[2] || fixture.Summary.RetryAttemptCount != len(retryAttempts) {
		t.Fatalf("retry attempts = %#v summary=%#v", retryAttempts, fixture.Summary)
	}
	for _, want := range contract.Semantics.TraceEvidence.EvidenceKinds {
		if !seenKinds[want] {
			t.Fatalf("trace fixture missing evidence kind %q", want)
		}
	}
}

func TestGenericPlanningGraphMainLeiaSmokeExecutes(t *testing.T) {
	root := repoRoot(t)
	mainPath := filepath.Join(root, filepath.FromSlash(genericPlanningGraphPackageDir), "main.leia")
	data, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ToLower(string(data))
	for _, forbidden := range []string{"import q", "q" + "/runtime", "$`", "$!`", "openai", "anthropic", "provider_sdk", "autogen", "finrobot"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("main.leia must stay provider-free and generic; found %q", forbidden)
		}
	}

	vm := leia.New(leia.WithLibs(leia.LibAll), leia.WithVM())
	if err := vm.ExecFile(mainPath); err != nil {
		t.Fatalf("ExecFile main.leia: %v", err)
	}
	value, err := vm.Get("generic_planning_graph_live_package_summary")
	if err != nil {
		t.Fatalf("get smoke summary: %v", err)
	}
	summary := fmt.Sprint(value)
	for _, want := range []string{
		"generic_planning_graph_live_package",
		"nodes=7",
		"edges=7",
		"trace_events=10",
		"capabilities=6",
		"provider_free=true",
		"live_network=false",
		"imports=false",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("smoke summary missing %q: %s", want, summary)
		}
	}
}

func loadGenericPlanningGraphManifest(t *testing.T, root string) genericPlanningGraphManifest {
	t.Helper()
	var manifest genericPlanningGraphManifest
	readGenericPlanningGraphJSON(t, root, "package.manifest.json", &manifest)
	return manifest
}

func loadGenericPlanningGraphContract(t *testing.T, root string) genericPlanningGraphContract {
	t.Helper()
	var contract genericPlanningGraphContract
	readGenericPlanningGraphJSON(t, root, "contracts/planning_graph_contract.json", &contract)
	return contract
}

func loadGenericPlanningGraphFixtureIndex(t *testing.T, root string) genericPlanningGraphFixtureIndex {
	t.Helper()
	var index genericPlanningGraphFixtureIndex
	readGenericPlanningGraphJSON(t, root, "fixtures/provider_free_fixture_index.json", &index)
	return index
}

func loadGenericPlanningGraphTraceFixture(t *testing.T, root string) genericPlanningGraphTraceFixture {
	t.Helper()
	var fixture genericPlanningGraphTraceFixture
	readGenericPlanningGraphJSON(t, root, "fixtures/planning_graph_trace_fixture.json", &fixture)
	return fixture
}

func loadGenericPlanningGraphSchema(t *testing.T, root, rel string) map[string]any {
	t.Helper()
	var schema map[string]any
	readGenericPlanningGraphJSON(t, root, rel, &schema)
	return schema
}

func readGenericPlanningGraphJSON(t *testing.T, root, rel string, target any) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(genericPlanningGraphPackageDir), filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func assertGenericPlanningGraphPackageFileExists(t *testing.T, root, rel string) {
	t.Helper()
	if rel == "" {
		t.Fatal("empty package-relative path")
	}
	if filepath.IsAbs(rel) || strings.Contains(rel, "..") {
		t.Fatalf("package path must be relative and scoped: %q", rel)
	}
	path := filepath.Join(root, filepath.FromSlash(genericPlanningGraphPackageDir), filepath.FromSlash(rel))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("missing package file %s: %v", rel, err)
	}
}

func assertGenericPlanningGraphProviderFree(t *testing.T, root, rel string, providerFree, domainSpecific, liveNetwork, liveModel, realDependencyImports bool) {
	t.Helper()
	if !providerFree || domainSpecific || liveNetwork || liveModel || realDependencyImports {
		t.Fatalf("%s provider boundary mismatch: provider_free=%v domain_specific=%v live_network=%v live_model=%v real_dependency_imports=%v", rel, providerFree, domainSpecific, liveNetwork, liveModel, realDependencyImports)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(genericPlanningGraphPackageDir), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(data))
	for _, forbidden := range []string{
		"anthropic",
		"autogen",
		"finrobot",
		"finance",
		"fingpt",
		"openai",
		"openbb",
		"provider_model",
		"trading",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("%s contains provider/domain marker %q", rel, forbidden)
		}
	}
}

func assertGenericPlanningGraphRequiredContains(t *testing.T, label string, object map[string]any, want []string) {
	t.Helper()
	rawRequired, ok := object["required"].([]any)
	if !ok {
		t.Fatalf("%s missing required array: %#v", label, object["required"])
	}
	got := make([]string, 0, len(rawRequired))
	for _, value := range rawRequired {
		field, ok := value.(string)
		if !ok {
			t.Fatalf("%s required contains non-string value: %#v", label, value)
		}
		got = append(got, field)
	}
	for _, field := range want {
		if !genericPlanningGraphContains(got, field) {
			t.Fatalf("%s required missing %q: %#v", label, field, got)
		}
	}
}

func genericPlanningGraphObject(t *testing.T, object map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := object[key].(map[string]any)
	if !ok {
		t.Fatalf("object key %q is not an object: %#v", key, object[key])
	}
	return value
}

func genericPlanningGraphContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func assertGenericPlanningGraphAcyclic(t *testing.T, nodes map[string]struct {
	nodeType string
	depends  []string
}, edges []struct {
	From     string `json:"from"`
	To       string `json:"to"`
	EdgeType string `json:"edge_type"`
	BranchID string `json:"branch_id"`
}) {
	t.Helper()
	indegree := map[string]int{}
	outgoing := map[string][]string{}
	for nodeID := range nodes {
		indegree[nodeID] = 0
	}
	for _, edge := range edges {
		if _, ok := nodes[edge.From]; !ok {
			t.Fatalf("edge source %q is not a plan node", edge.From)
		}
		if _, ok := nodes[edge.To]; !ok {
			t.Fatalf("edge target %q is not a plan node", edge.To)
		}
		outgoing[edge.From] = append(outgoing[edge.From], edge.To)
		indegree[edge.To]++
	}
	queue := []string{}
	for nodeID, degree := range indegree {
		if degree == 0 {
			queue = append(queue, nodeID)
		}
	}
	visited := 0
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		visited++
		for _, next := range outgoing[nodeID] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if visited != len(nodes) {
		t.Fatalf("planning graph has a cycle or disconnected dependency count: visited=%d nodes=%d", visited, len(nodes))
	}
}

func assertGenericPlanningGraphBranchMerge(t *testing.T, contract genericPlanningGraphContract) {
	t.Helper()
	branchTargets := map[string][]string{}
	mergeSources := map[string][]string{}
	for _, edge := range contract.Edges {
		switch edge.EdgeType {
		case "branch":
			branchTargets[edge.From] = append(branchTargets[edge.From], edge.To)
		case "merge":
			mergeSources[edge.To] = append(mergeSources[edge.To], edge.From)
		}
	}
	for _, node := range contract.PlanNodes {
		switch node.NodeType {
		case "branch":
			assertGenericPlanningGraphStringSet(t, node.ID+" branch targets", branchTargets[node.ID], node.Branches)
			if len(branchTargets[node.ID]) < 2 {
				t.Fatalf("branch node %s must have at least two branch edges", node.ID)
			}
		case "merge":
			assertGenericPlanningGraphStringSet(t, node.ID+" merge inputs", mergeSources[node.ID], node.DependsOn)
			assertGenericPlanningGraphStringSet(t, node.ID+" required merge inputs", node.MergePolicy.RequiredInputs, node.DependsOn)
			if node.MergePolicy.Mode != "all_success" {
				t.Fatalf("merge node %s mode = %q", node.ID, node.MergePolicy.Mode)
			}
		}
	}
}

func assertGenericPlanningGraphStringSet(t *testing.T, label string, got, want []string) {
	t.Helper()
	got = append([]string(nil), got...)
	want = append([]string(nil), want...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("%s = %#v, want %#v", label, got, want)
	}
}
