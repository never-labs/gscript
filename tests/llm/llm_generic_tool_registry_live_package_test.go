package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type genericToolRegistryManifest struct {
	SchemaVersion               int    `json:"schema_version"`
	ID                          string `json:"id"`
	PackageName                 string `json:"package_name"`
	ProviderFree                bool   `json:"provider_free"`
	DomainSpecific              bool   `json:"domain_specific"`
	FinRobotSpecific            bool   `json:"finrobot_specific"`
	LiveNetworkDefault          bool   `json:"live_network_default"`
	RealDependencyImportDefault bool   `json:"real_dependency_import_default"`
	LocalExecutionDefault       bool   `json:"local_execution_default"`
	Credentials                 struct {
		Required          []string `json:"required"`
		Optional          []string `json:"optional"`
		SecretEnvPatterns []string `json:"secret_env_patterns"`
	} `json:"credentials"`
	DefaultPolicy struct {
		Mode                              string `json:"mode"`
		LiveNetwork                       bool   `json:"live_network"`
		ProviderCredentialsRequired       bool   `json:"provider_credentials_required"`
		RealDependencyImports             bool   `json:"real_dependency_imports"`
		LocalExecution                    bool   `json:"local_execution"`
		ApprovalRequiredForEffectfulTools bool   `json:"approval_required_for_effectful_tools"`
		CleanSkipWithoutApproval          bool   `json:"clean_skip_without_approval"`
		FixtureHook                       string `json:"fixture_hook"`
	} `json:"default_policy"`
	Entrypoints  map[string]string `json:"entrypoints"`
	Schemas      map[string]string `json:"schemas"`
	Fixtures     map[string]string `json:"fixtures"`
	Capabilities []struct {
		ID               string `json:"id"`
		Schema           string `json:"schema"`
		Default          string `json:"default"`
		ApprovalRequired bool   `json:"approval_required"`
	} `json:"capabilities"`
	TestGates []string `json:"test_gates"`
}

type genericToolRegistryContract struct {
	SchemaVersion         int      `json:"schema_version"`
	ID                    string   `json:"id"`
	ProviderFree          bool     `json:"provider_free"`
	DomainSpecific        bool     `json:"domain_specific"`
	FinRobotSpecific      bool     `json:"finrobot_specific"`
	LiveNetwork           bool     `json:"live_network"`
	RealDependencyImports bool     `json:"real_dependency_imports"`
	LocalExecution        bool     `json:"local_execution"`
	DefaultMode           string   `json:"default_mode"`
	CapabilityPrefix      string   `json:"capability_prefix"`
	RequiredCapabilities  []string `json:"required_capabilities"`
	DescriptorContract    struct {
		SchemaRef                  string   `json:"schema_ref"`
		RequiredFields             []string `json:"required_fields"`
		ProviderWireFormat         string   `json:"provider_wire_format"`
		AdditionalPropertiesPolicy string   `json:"additional_properties_policy"`
		SecretPolicy               string   `json:"secret_policy"`
	} `json:"descriptor_contract"`
	InvocationTraceContract struct {
		SchemaRef              string   `json:"schema_ref"`
		FixtureRef             string   `json:"fixture_ref"`
		RequiredEvents         []string `json:"required_events"`
		RequiredFields         []string `json:"required_fields"`
		ResultEnvelopeRequired []string `json:"result_envelope_required"`
	} `json:"invocation_trace_contract"`
	ApprovalEdges []struct {
		ID               string `json:"id"`
		Effect           string `json:"effect"`
		ApprovalRequired bool   `json:"approval_required"`
		DefaultDecision  string `json:"default_decision"`
		FixtureRef       string `json:"fixture_ref"`
		CleanSkip        bool   `json:"clean_skip"`
	} `json:"approval_edges"`
	NegativeGates struct {
		ForbiddenImports          []string `json:"forbidden_imports"`
		ForbiddenLiveBehaviors    []string `json:"forbidden_live_behaviors"`
		ForbiddenDescriptorFields []string `json:"forbidden_descriptor_fields"`
	} `json:"negative_gates"`
}

type genericToolRegistryFixture struct {
	SchemaVersion int `json:"schema_version"`
	Descriptor    struct {
		ToolName                string         `json:"tool_name"`
		Description             string         `json:"description"`
		CapabilityIDs           []string       `json:"capability_ids"`
		InputSchema             map[string]any `json:"input_schema"`
		OutputSchema            map[string]any `json:"output_schema"`
		CallerRole              string         `json:"caller_role"`
		ExecutorRole            string         `json:"executor_role"`
		Effect                  string         `json:"effect"`
		ApprovalPolicy          string         `json:"approval_policy"`
		ProviderWireFormat      string         `json:"provider_wire_format"`
		LiveNetwork             bool           `json:"live_network"`
		SecretParametersAllowed bool           `json:"secret_parameters_allowed"`
	} `json:"descriptor"`
	Trace struct {
		TraceID       string   `json:"trace_id"`
		ToolName      string   `json:"tool_name"`
		CallerID      string   `json:"caller_id"`
		ExecutorID    string   `json:"executor_id"`
		CapabilityIDs []string `json:"capability_ids"`
		Events        []struct {
			Event    string `json:"event"`
			Sequence int    `json:"sequence"`
		} `json:"events"`
		SchemaValidation struct {
			InputValid                   bool `json:"input_valid"`
			OutputValid                  bool `json:"output_valid"`
			AdditionalPropertiesRejected bool `json:"additional_properties_rejected"`
		} `json:"schema_validation"`
		Approval struct {
			Required   bool    `json:"required"`
			Decision   string  `json:"decision"`
			ApprovalID *string `json:"approval_id"`
			Reason     string  `json:"reason"`
		} `json:"approval"`
		Result struct {
			OK       bool           `json:"ok"`
			Status   string         `json:"status"`
			Content  string         `json:"content"`
			Metadata map[string]any `json:"metadata"`
		} `json:"result"`
		Provenance struct {
			ProviderFree          bool   `json:"provider_free"`
			LiveNetwork           bool   `json:"live_network"`
			RealDependencyImports bool   `json:"real_dependency_imports"`
			LocalExecution        bool   `json:"local_execution"`
			FixtureKey            string `json:"fixture_key"`
		} `json:"provenance"`
	} `json:"trace"`
}

func TestGenericToolRegistryLivePackageManifestContractAndFixtures(t *testing.T) {
	base := genericToolRegistryDir(t)
	manifest := loadGenericToolRegistryManifest(t, base)
	if manifest.SchemaVersion != 1 || manifest.ID != "generic-tool-registry-live-package" || manifest.PackageName != "leia-generic-ai-tool-registry" {
		t.Fatalf("manifest header invalid: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.FinRobotSpecific ||
		manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault || manifest.LocalExecutionDefault {
		t.Fatalf("generic tool registry must stay provider-free and generic: %#v", manifest)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("generic tool registry must not declare credentials: %#v", manifest.Credentials)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" || manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired || manifest.DefaultPolicy.RealDependencyImports ||
		manifest.DefaultPolicy.LocalExecution || !manifest.DefaultPolicy.ApprovalRequiredForEffectfulTools ||
		!manifest.DefaultPolicy.CleanSkipWithoutApproval || manifest.DefaultPolicy.FixtureHook == "" {
		t.Fatalf("default policy crosses provider-free boundary: %#v", manifest.DefaultPolicy)
	}

	for label, rel := range manifest.Entrypoints {
		if rel == "" {
			t.Fatalf("entrypoint %s is empty", label)
		}
		assertGenericToolRegistryFileExists(t, base, rel)
	}
	for label, rel := range manifest.Schemas {
		if rel == "" {
			t.Fatalf("schema %s is empty", label)
		}
		assertGenericToolRegistryFileExists(t, base, rel)
	}
	for label, rel := range manifest.Fixtures {
		if rel == "" {
			t.Fatalf("fixture %s is empty", label)
		}
		assertGenericToolRegistryFileExists(t, base, rel)
	}
	if len(manifest.Capabilities) < 4 {
		t.Fatalf("capabilities = %d, want registry, schema, trace, and approval edge", len(manifest.Capabilities))
	}
	for _, capability := range manifest.Capabilities {
		if !strings.HasPrefix(capability.ID, "generic.tool.") || capability.Schema == "" {
			t.Fatalf("capability must stay generic and schema-backed: %#v", capability)
		}
		assertGenericToolRegistryFileExists(t, base, capability.Schema)
	}
}

func TestGenericToolRegistryContractSchemaTraceAndApprovalEdges(t *testing.T) {
	base := genericToolRegistryDir(t)
	var contract genericToolRegistryContract
	decodeGenericToolRegistryJSONFile(t, filepath.Join(base, "contracts", "tool_registry_contract.json"), &contract)

	if contract.SchemaVersion != 1 || contract.ID != "generic-tool-registry-contract" ||
		!contract.ProviderFree || contract.DomainSpecific || contract.FinRobotSpecific ||
		contract.LiveNetwork || contract.RealDependencyImports || contract.LocalExecution ||
		contract.DefaultMode != "fixture_replay" || contract.CapabilityPrefix != "generic.tool." {
		t.Fatalf("contract boundary invalid: %#v", contract)
	}
	for _, want := range []string{
		"generic.tool.registry.declare",
		"generic.tool.registry.validate_schema",
		"generic.tool.invocation.trace",
		"generic.tool.approval.edge",
	} {
		if !stringSliceContains(contract.RequiredCapabilities, want) {
			t.Fatalf("required capability %q missing: %#v", want, contract.RequiredCapabilities)
		}
	}
	for _, want := range []string{"tool_name", "capability_ids", "input_schema", "output_schema", "approval_policy"} {
		if !stringSliceContains(contract.DescriptorContract.RequiredFields, want) {
			t.Fatalf("descriptor field %q missing: %#v", want, contract.DescriptorContract.RequiredFields)
		}
	}
	if contract.DescriptorContract.ProviderWireFormat != "none" ||
		contract.DescriptorContract.AdditionalPropertiesPolicy != "reject_unknown_tool_metadata" ||
		!strings.Contains(contract.DescriptorContract.SecretPolicy, "must not contain") {
		t.Fatalf("descriptor contract allows provider wire format or secrets: %#v", contract.DescriptorContract)
	}
	for _, want := range []string{"registered", "schema_validated", "approval_checked", "invoked", "result_recorded"} {
		if !stringSliceContains(contract.InvocationTraceContract.RequiredEvents, want) {
			t.Fatalf("trace event %q missing: %#v", want, contract.InvocationTraceContract.RequiredEvents)
		}
	}
	for _, want := range []string{"ok", "status", "content", "metadata"} {
		if !stringSliceContains(contract.InvocationTraceContract.ResultEnvelopeRequired, want) {
			t.Fatalf("result envelope field %q missing: %#v", want, contract.InvocationTraceContract.ResultEnvelopeRequired)
		}
	}

	approvalEdges := map[string]bool{}
	for _, edge := range contract.ApprovalEdges {
		approvalEdges[edge.ID] = true
		if edge.Effect == "effectful" && (!edge.ApprovalRequired || edge.DefaultDecision != "deny" || !edge.CleanSkip || edge.FixtureRef == "") {
			t.Fatalf("effectful edge must deny and clean-skip without approval: %#v", edge)
		}
	}
	for _, want := range []string{"read_only_fixture", "effectful_denied_without_token"} {
		if !approvalEdges[want] {
			t.Fatalf("approval edge %q missing", want)
		}
	}
	for _, blocked := range []string{"network", "provider_api_call", "local_code_execution"} {
		if !stringSliceContains(contract.NegativeGates.ForbiddenLiveBehaviors, blocked) {
			t.Fatalf("negative live behavior %q missing: %#v", blocked, contract.NegativeGates.ForbiddenLiveBehaviors)
		}
	}
	for _, blocked := range []string{"api_key", "access_token", "secret", "password", "credential"} {
		if !stringSliceContains(contract.NegativeGates.ForbiddenDescriptorFields, blocked) {
			t.Fatalf("negative descriptor field %q missing: %#v", blocked, contract.NegativeGates.ForbiddenDescriptorFields)
		}
	}
}

func TestGenericToolRegistryFixturesCoverSchemaTraceApprovalNoSecretNoNetwork(t *testing.T) {
	base := genericToolRegistryDir(t)
	contract := genericToolRegistryContract{}
	decodeGenericToolRegistryJSONFile(t, filepath.Join(base, "contracts", "tool_registry_contract.json"), &contract)

	success := loadGenericToolRegistryFixture(t, filepath.Join(base, "fixtures", "tool_registry_replay_trace_fixture.json"))
	assertGenericToolRegistryFixtureBoundary(t, success)
	if success.Descriptor.Effect != "read_only" || success.Trace.Approval.Required || success.Trace.Approval.Decision != "allow_fixture" ||
		!success.Trace.Result.OK || success.Trace.Result.Status != "ok" {
		t.Fatalf("success fixture does not prove read-only fixture replay: %#v", success.Trace)
	}

	denied := loadGenericToolRegistryFixture(t, filepath.Join(base, "fixtures", "approval_edge_fixture.json"))
	assertGenericToolRegistryFixtureBoundary(t, denied)
	if denied.Descriptor.Effect != "effectful" || denied.Descriptor.ApprovalPolicy != "deny_without_approval" ||
		!denied.Trace.Approval.Required || denied.Trace.Approval.Decision != "deny" ||
		denied.Trace.Result.OK || denied.Trace.Result.Status != "denied" ||
		denied.Trace.Result.Metadata["executed"] != false || denied.Trace.Result.Metadata["clean_skip"] != true {
		t.Fatalf("denied fixture does not prove approval edge clean skip: %#v", denied.Trace)
	}

	requiredEvents := append([]string(nil), contract.InvocationTraceContract.RequiredEvents...)
	sort.Strings(requiredEvents)
	for _, fixture := range []genericToolRegistryFixture{success, denied} {
		gotEvents := make([]string, 0, len(fixture.Trace.Events))
		for i, event := range fixture.Trace.Events {
			if event.Sequence != i+1 {
				t.Fatalf("%s event sequence = %d at index %d", fixture.Trace.TraceID, event.Sequence, i)
			}
			gotEvents = append(gotEvents, event.Event)
		}
		sort.Strings(gotEvents)
		if !reflect.DeepEqual(gotEvents, requiredEvents) {
			t.Fatalf("%s events = %#v, want %#v", fixture.Trace.TraceID, gotEvents, requiredEvents)
		}
	}

	assertGenericToolRegistryNoSecretNoNetwork(t, base, contract)
}

func assertGenericToolRegistryFixtureBoundary(t *testing.T, fixture genericToolRegistryFixture) {
	t.Helper()
	if fixture.SchemaVersion != 1 {
		t.Fatalf("fixture schema version = %d", fixture.SchemaVersion)
	}
	if fixture.Descriptor.ToolName == "" || fixture.Descriptor.Description == "" ||
		fixture.Descriptor.CallerRole != "caller" || fixture.Descriptor.ExecutorRole != "executor" ||
		fixture.Descriptor.ProviderWireFormat != "none" || fixture.Descriptor.LiveNetwork ||
		fixture.Descriptor.SecretParametersAllowed {
		t.Fatalf("descriptor boundary invalid: %#v", fixture.Descriptor)
	}
	if len(fixture.Descriptor.CapabilityIDs) == 0 || len(fixture.Trace.CapabilityIDs) == 0 {
		t.Fatalf("fixture missing capabilities: %#v", fixture)
	}
	for _, capability := range append(fixture.Descriptor.CapabilityIDs, fixture.Trace.CapabilityIDs...) {
		if !strings.HasPrefix(capability, "generic.tool.") {
			t.Fatalf("capability %q is not generic tool registry scoped", capability)
		}
	}
	if fixture.Trace.TraceID == "" || fixture.Trace.ToolName != fixture.Descriptor.ToolName ||
		fixture.Trace.CallerID == "" || fixture.Trace.ExecutorID == "" ||
		!fixture.Trace.SchemaValidation.InputValid || !fixture.Trace.SchemaValidation.OutputValid ||
		!fixture.Trace.SchemaValidation.AdditionalPropertiesRejected {
		t.Fatalf("trace does not prove schema validation: %#v", fixture.Trace)
	}
	if !fixture.Trace.Provenance.ProviderFree || fixture.Trace.Provenance.LiveNetwork ||
		fixture.Trace.Provenance.RealDependencyImports || fixture.Trace.Provenance.LocalExecution ||
		fixture.Trace.Provenance.FixtureKey == "" {
		t.Fatalf("trace provenance crosses provider-free boundary: %#v", fixture.Trace.Provenance)
	}
}

func assertGenericToolRegistryNoSecretNoNetwork(t *testing.T, base string, contract genericToolRegistryContract) {
	t.Helper()
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatalf("read generic tool registry dir: %v", err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, filepath.Join(base, entry.Name()))
			continue
		}
		nested, err := os.ReadDir(filepath.Join(base, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		for _, nestedEntry := range nested {
			if !nestedEntry.IsDir() {
				files = append(files, filepath.Join(base, entry.Name(), nestedEntry.Name()))
			}
		}
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)
		for _, marker := range []string{"http://", "https://"} {
			if strings.Contains(text, marker) {
				t.Fatalf("%s contains live network locator %q", path, marker)
			}
		}
		for _, blocked := range contract.NegativeGates.ForbiddenImports {
			if strings.Contains(text, "import "+blocked) ||
				strings.Contains(text, "from "+blocked+" import") ||
				strings.Contains(text, `require("`+blocked+`"`) {
				t.Fatalf("%s imports forbidden local execution dependency %q", path, blocked)
			}
		}
	}
}

func genericToolRegistryDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_tool_registry")
}

func loadGenericToolRegistryManifest(t *testing.T, base string) genericToolRegistryManifest {
	t.Helper()
	var manifest genericToolRegistryManifest
	decodeGenericToolRegistryJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func loadGenericToolRegistryFixture(t *testing.T, path string) genericToolRegistryFixture {
	t.Helper()
	var fixture genericToolRegistryFixture
	decodeGenericToolRegistryJSONFile(t, path, &fixture)
	return fixture
}

func assertGenericToolRegistryFileExists(t *testing.T, base, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(rel))); err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
}

func decodeGenericToolRegistryJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
