package leia_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
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
		CapabilityID     string `json:"capability_id"`
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

type genericToolRegistryFixtureIndex struct {
	SchemaVersion         int    `json:"schema_version"`
	ID                    string `json:"id"`
	ProviderFree          bool   `json:"provider_free"`
	LiveNetwork           bool   `json:"live_network"`
	RealDependencyImports bool   `json:"real_dependency_imports"`
	LocalExecution        bool   `json:"local_execution"`
	Fixtures              []struct {
		FixtureKey            string         `json:"fixture_key"`
		Capability            string         `json:"capability"`
		SchemaRef             string         `json:"schema_ref"`
		Path                  string         `json:"path"`
		ProviderFree          bool           `json:"provider_free"`
		LiveNetwork           bool           `json:"live_network"`
		RealDependencyImports bool           `json:"real_dependency_imports"`
		Metadata              map[string]any `json:"metadata"`
		Status                string         `json:"status"`
	} `json:"fixtures"`
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

func TestGenericToolRegistryLivePackageFixtureIndex(t *testing.T) {
	base := genericToolRegistryDir(t)
	var index genericToolRegistryFixtureIndex
	decodeGenericToolRegistryJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if index.SchemaVersion != 1 ||
		index.ID != "generic-tool-registry-provider-free-fixture-index" ||
		!index.ProviderFree || index.LiveNetwork || index.RealDependencyImports ||
		index.LocalExecution || len(index.Fixtures) != 3 {
		t.Fatalf("fixture index boundary drifted: %#v", index)
	}

	want := map[string]struct {
		capability string
		schemaRef  string
		path       string
		status     string
	}{
		"generic_tool:registry:descriptor:v1": {
			capability: "generic.tool.registry.declare",
			schemaRef:  "schemas/tool_descriptor.schema.json",
			path:       "fixtures/tool_registry_replay_trace_fixture.json",
			status:     "ok",
		},
		"generic_tool:invocation:trace:v1": {
			capability: "generic.tool.invocation.trace",
			schemaRef:  "schemas/tool_invocation_trace.schema.json",
			path:       "fixtures/tool_registry_replay_trace_fixture.json",
			status:     "ok",
		},
		"generic_tool:approval:denied:v1": {
			capability: "generic.tool.approval.edge",
			schemaRef:  "schemas/tool_invocation_trace.schema.json",
			path:       "fixtures/approval_edge_fixture.json",
			status:     "denied",
		},
	}
	for _, fixture := range index.Fixtures {
		expected, ok := want[fixture.FixtureKey]
		if !ok {
			t.Fatalf("unexpected fixture index key %q", fixture.FixtureKey)
		}
		if fixture.Capability != expected.capability ||
			fixture.SchemaRef != expected.schemaRef ||
			fixture.Path != expected.path ||
			fixture.Status != expected.status ||
			!fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports ||
			fixture.Metadata["provider_free"] != true ||
			fixture.Metadata["live_network"] != false ||
			fixture.Metadata["real_dependency_imports"] != false {
			t.Fatalf("fixture index entry drifted: %#v", fixture)
		}
		assertGenericToolRegistryFileExists(t, base, fixture.SchemaRef)
		assertGenericToolRegistryFileExists(t, base, fixture.Path)
		delete(want, fixture.FixtureKey)
	}
	if len(want) != 0 {
		t.Fatalf("fixture index missing keys: %#v", want)
	}
}

func TestGenericToolRegistryManifestContractFixtureTraceRefsBidirectional(t *testing.T) {
	base := genericToolRegistryDir(t)
	manifest := loadGenericToolRegistryManifest(t, base)
	contract := genericToolRegistryContract{}
	decodeGenericToolRegistryJSONFile(t, filepath.Join(base, "contracts", "tool_registry_contract.json"), &contract)
	index := loadGenericToolRegistryFixtureIndex(t, base)

	manifestSchemas := map[string]bool{}
	for _, rel := range manifest.Schemas {
		manifestSchemas[rel] = true
	}
	manifestFixtures := map[string]bool{}
	for _, rel := range manifest.Fixtures {
		manifestFixtures[rel] = true
	}
	manifestCapabilitySchema := map[string]string{}
	manifestCapabilityApproval := map[string]bool{}
	for _, capability := range manifest.Capabilities {
		if capability.ID == "" || capability.CapabilityID != capability.ID {
			t.Fatalf("manifest capability id/capability_id mismatch: %#v", capability)
		}
		if manifestCapabilitySchema[capability.ID] != "" {
			t.Fatalf("duplicate manifest capability %q", capability.ID)
		}
		if !manifestSchemas[capability.Schema] {
			t.Fatalf("manifest capability %q schema %q is not declared in manifest schemas %#v", capability.ID, capability.Schema, manifest.Schemas)
		}
		manifestCapabilitySchema[capability.ID] = capability.Schema
		manifestCapabilityApproval[capability.ID] = capability.ApprovalRequired
	}
	if len(manifestCapabilitySchema) != len(contract.RequiredCapabilities) {
		t.Fatalf("manifest capabilities and contract required capabilities differ: manifest=%#v contract=%#v", manifestCapabilitySchema, contract.RequiredCapabilities)
	}
	for _, capability := range contract.RequiredCapabilities {
		if manifestCapabilitySchema[capability] == "" {
			t.Fatalf("contract required capability %q missing from manifest capabilities", capability)
		}
	}
	for _, schemaRef := range []string{contract.DescriptorContract.SchemaRef, contract.InvocationTraceContract.SchemaRef} {
		if !manifestSchemas[schemaRef] {
			t.Fatalf("contract schema ref %q missing from manifest schemas %#v", schemaRef, manifest.Schemas)
		}
	}
	if !manifestFixtures[contract.InvocationTraceContract.FixtureRef] {
		t.Fatalf("contract trace fixture ref %q missing from manifest fixtures %#v", contract.InvocationTraceContract.FixtureRef, manifest.Fixtures)
	}
	for _, edge := range contract.ApprovalEdges {
		if edge.FixtureRef != "" && !manifestFixtures[edge.FixtureRef] {
			t.Fatalf("approval edge %q fixture ref %q missing from manifest fixtures %#v", edge.ID, edge.FixtureRef, manifest.Fixtures)
		}
	}

	indexByKey := map[string]struct {
		Capability string
		SchemaRef  string
		Path       string
		Status     string
	}{}
	for _, fixture := range index.Fixtures {
		if manifestCapabilitySchema[fixture.Capability] == "" {
			t.Fatalf("fixture index capability %q missing from manifest capabilities", fixture.Capability)
		}
		if fixture.SchemaRef != manifestCapabilitySchema[fixture.Capability] {
			t.Fatalf("fixture index schema %q does not match manifest capability %q schema %q", fixture.SchemaRef, fixture.Capability, manifestCapabilitySchema[fixture.Capability])
		}
		if !manifestFixtures[fixture.Path] {
			t.Fatalf("fixture index path %q missing from manifest fixtures %#v", fixture.Path, manifest.Fixtures)
		}
		if manifestCapabilityApproval[fixture.Capability] != (fixture.Status == "denied") && fixture.Capability == "generic.tool.approval.edge" {
			t.Fatalf("approval fixture status does not mirror manifest approval requirement: %#v", fixture)
		}
		if indexByKey[fixture.FixtureKey].Capability != "" {
			t.Fatalf("duplicate fixture key %q", fixture.FixtureKey)
		}
		indexByKey[fixture.FixtureKey] = struct {
			Capability string
			SchemaRef  string
			Path       string
			Status     string
		}{
			Capability: fixture.Capability,
			SchemaRef:  fixture.SchemaRef,
			Path:       fixture.Path,
			Status:     fixture.Status,
		}
	}

	seenTraceKeys := map[string]bool{}
	for _, rel := range manifest.Fixtures {
		if rel == "fixtures/provider_free_fixture_index.json" {
			continue
		}
		fixture := loadGenericToolRegistryFixture(t, filepath.Join(base, filepath.FromSlash(rel)))
		traceKey := fixture.Trace.Provenance.FixtureKey
		indexEntry, ok := indexByKey[traceKey]
		if !ok {
			t.Fatalf("trace provenance fixture key %q missing from fixture index", traceKey)
		}
		if indexEntry.Path != rel {
			t.Fatalf("trace provenance fixture key %q points to %q in index, want manifest fixture path %q", traceKey, indexEntry.Path, rel)
		}
		if indexEntry.SchemaRef != contract.InvocationTraceContract.SchemaRef {
			t.Fatalf("trace fixture key %q schema ref = %q, want contract trace schema %q", traceKey, indexEntry.SchemaRef, contract.InvocationTraceContract.SchemaRef)
		}
		if !stringSliceContains(fixture.Trace.CapabilityIDs, indexEntry.Capability) {
			t.Fatalf("trace %q missing indexed capability %q in trace capability ids %#v", traceKey, indexEntry.Capability, fixture.Trace.CapabilityIDs)
		}
		for _, capability := range append(fixture.Descriptor.CapabilityIDs, fixture.Trace.CapabilityIDs...) {
			if manifestCapabilitySchema[capability] == "" {
				t.Fatalf("fixture %q capability %q missing from manifest capabilities", traceKey, capability)
			}
			if !stringSliceContains(contract.RequiredCapabilities, capability) {
				t.Fatalf("fixture %q capability %q missing from contract required capabilities %#v", traceKey, capability, contract.RequiredCapabilities)
			}
		}
		seenTraceKeys[traceKey] = true
	}
	for key, entry := range indexByKey {
		if entry.SchemaRef == contract.InvocationTraceContract.SchemaRef && !seenTraceKeys[key] {
			t.Fatalf("trace schema fixture index key %q did not round-trip from fixture provenance", key)
		}
	}
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

func TestGenericToolRegistryLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericToolRegistryDir(t)
	descriptorSchema := filepath.Join(base, "schemas", "tool_descriptor.schema.json")
	assertDocumentPipelineSchemaRequired(t, descriptorSchema, []string{"tool_name", "description", "capability_ids", "input_schema", "output_schema", "caller_role", "executor_role", "effect", "approval_policy", "provider_wire_format", "live_network", "secret_parameters_allowed"})

	traceSchema := filepath.Join(base, "schemas", "tool_invocation_trace.schema.json")
	assertDocumentPipelineSchemaRequired(t, traceSchema, []string{"trace_id", "tool_name", "caller_id", "executor_id", "capability_ids", "events", "schema_validation", "approval", "result", "provenance"})
	assertDocumentPipelineNestedSchemaRequired(t, traceSchema, []string{"properties", "events", "items"}, []string{"event", "sequence"})
	assertDocumentPipelineNestedSchemaRequired(t, traceSchema, []string{"properties", "schema_validation"}, []string{"input_valid", "output_valid", "additional_properties_rejected"})
	assertDocumentPipelineNestedSchemaRequired(t, traceSchema, []string{"properties", "approval"}, []string{"required", "decision", "approval_id", "reason"})
	assertDocumentPipelineNestedSchemaRequired(t, traceSchema, []string{"properties", "result"}, []string{"ok", "status", "content", "metadata"})
	assertDocumentPipelineNestedSchemaRequired(t, traceSchema, []string{"properties", "provenance"}, []string{"provider_free", "live_network", "real_dependency_imports", "local_execution", "fixture_key"})
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

func TestGenericToolRegistryMainSmokeOutput(t *testing.T) {
	base := genericToolRegistryDir(t)
	manifest := loadGenericToolRegistryManifest(t, base)
	contract := genericToolRegistryContract{}
	decodeGenericToolRegistryJSONFile(t, filepath.Join(base, "contracts", "tool_registry_contract.json"), &contract)

	for _, result := range runFinRobotLivePackageSummarySmoke(t, filepath.Join(base, manifest.Entrypoints["main"]), "generic_tool_registry_live_package_summary", "generic_tool_registry_live_package", leia.LibString) {
		summary := result.Fields
		requireFinRobotSummaryFields(t, summary, "descriptors", "traces", "approval_edges", "capabilities", "provider_free", "live_network", "imports", "local_execution")
		if summary["approval_edges"] != fmt.Sprint(len(contract.ApprovalEdges)) ||
			(summary["provider_free"] == "true") != manifest.ProviderFree ||
			(summary["live_network"] == "true") != manifest.LiveNetworkDefault ||
			(summary["imports"] == "true") != manifest.RealDependencyImportDefault ||
			(summary["local_execution"] == "true") != manifest.LocalExecutionDefault ||
			(summary["provider_free"] == "true") != contract.ProviderFree ||
			(summary["live_network"] == "true") != contract.LiveNetwork ||
			(summary["imports"] == "true") != contract.RealDependencyImports ||
			(summary["local_execution"] == "true") != contract.LocalExecution {
			t.Fatalf("summary does not align with package provider-free boundary: summary=%#v manifest=%#v contract=%#v", summary, manifest, contract)
		}
	}
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

func loadGenericToolRegistryFixtureIndex(t *testing.T, base string) genericToolRegistryFixtureIndex {
	t.Helper()
	var index genericToolRegistryFixtureIndex
	decodeGenericToolRegistryJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	return index
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
