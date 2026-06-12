package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type genericToolContractsManifest struct {
	SchemaVersion               int      `json:"schema_version"`
	ID                          string   `json:"id"`
	PackageName                 string   `json:"package_name"`
	DialectSymbols              []string `json:"dialect_symbols"`
	ProviderFree                bool     `json:"provider_free"`
	LiveNetworkDefault          bool     `json:"live_network_default"`
	RealDependencyImportDefault bool     `json:"real_dependency_import_default"`
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
		ToolExecutionDefault        string `json:"tool_execution_default"`
		ApprovalDefault             string `json:"approval_default"`
		ArgumentValidationDefault   string `json:"argument_validation_default"`
		ArtifactStorageDefault      string `json:"artifact_storage_default"`
		CleanSkipWithoutDependency  bool   `json:"clean_skip_without_dependency"`
		FixtureHook                 string `json:"fixture_hook"`
	} `json:"default_policy"`
	Entrypoints          map[string]string `json:"entrypoints"`
	Schemas              map[string]string `json:"schemas"`
	Fixtures             map[string]string `json:"fixtures"`
	Capabilities         []string          `json:"capabilities"`
	ApprovalStates       []string          `json:"approval_states"`
	NormalizedErrorKinds []string          `json:"normalized_error_kinds"`
	ArtifactRefPolicy    struct {
		Mode                 string   `json:"mode"`
		InlinePayloadDefault bool     `json:"inline_payload_default"`
		RequiredFields       []string `json:"required_fields"`
	} `json:"artifact_ref_policy"`
	TestGates          []string `json:"test_gates"`
	NoBuiltInGuarantee struct {
		Required  bool   `json:"required"`
		Statement string `json:"statement"`
	} `json:"no_built_in_guarantee"`
}

func TestFinRobotGenericToolContractsLivePackageManifest(t *testing.T) {
	base := genericToolContractsLivePackageDir(t)
	manifest := loadGenericToolContractsManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "generic-ai-tool-contracts-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-generic-ai-tool-contracts" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !reflect.DeepEqual(manifest.DialectSymbols, []string{"generic.ai.tool.contract", "ai.tool.invoke"}) {
		t.Fatalf("dialect symbols = %#v", manifest.DialectSymbols)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	if !strings.Contains(manifest.Credentials.Policy, "provider-specific credentials") {
		t.Fatalf("credential policy should keep providers outside this package: %q", manifest.Credentials.Policy)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		manifest.DefaultPolicy.ToolExecutionDefault != "fixture_only" ||
		manifest.DefaultPolicy.ApprovalDefault != "deny_without_explicit_fixture" ||
		manifest.DefaultPolicy.ArgumentValidationDefault != "schema_required" ||
		manifest.DefaultPolicy.ArtifactStorageDefault != "artifact_ref_only" ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_generic_tool_contract_fixture" {
		t.Fatalf("default policy must stay fixture-only and provider-free: %#v", manifest.DefaultPolicy)
	}

	for _, key := range []string{"generic_tool_contract", "fixture_index", "invoke_success_fixture", "invoke_validation_error_fixture", "registry_descriptor_projection_fixture"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
		assertGenericToolContractsEntrypointPath(t, manifest.Entrypoints[key])
		assertGenericToolContractsJSONFile(t, filepath.Join(base, manifest.Entrypoints[key]))
	}
	if manifest.Entrypoints["smoke"] != "main.leia" {
		t.Fatalf("smoke entrypoint = %q, want main.leia", manifest.Entrypoints["smoke"])
	}
	assertGenericToolContractsEntrypointPath(t, manifest.Entrypoints["smoke"])
	if _, err := os.Stat(filepath.Join(base, manifest.Entrypoints["smoke"])); err != nil {
		t.Fatalf("smoke entrypoint missing: %v", err)
	}
	for _, key := range []string{"tool_contract", "invoke_request", "result_envelope", "normalized_error", "artifact_ref", "tool_contract_projection"} {
		if manifest.Schemas[key] == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertGenericToolContractsJSONFile(t, filepath.Join(base, manifest.Schemas[key]))
	}
	for _, key := range []string{"index", "invoke_success", "invoke_validation_error", "registry_descriptor_projection"} {
		if manifest.Fixtures[key] == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertGenericToolContractsJSONFile(t, filepath.Join(base, manifest.Fixtures[key]))
	}

	wantCapabilities := []string{
		"ai.tool.approval.state",
		"ai.tool.arguments.validate",
		"ai.tool.artifact.refs",
		"ai.tool.capability.tags",
		"ai.tool.error.normalized",
		"ai.tool.invoke",
		"ai.tool.replay.fixture",
		"ai.tool.result.envelope",
		"ai.tool.schema.declarative",
		"generic.ai.tool.contract",
		"generic.ai.tool.contract.project_registry_descriptor",
	}
	gotCapabilities := append([]string(nil), manifest.Capabilities...)
	sort.Strings(gotCapabilities)
	if !reflect.DeepEqual(gotCapabilities, wantCapabilities) {
		t.Fatalf("capabilities = %#v, want %#v", gotCapabilities, wantCapabilities)
	}
	for _, want := range []string{"fixture_only", "approved", "denied", "required"} {
		if !containsGenericToolContractsString(manifest.ApprovalStates, want) {
			t.Fatalf("approval states missing %q: %#v", want, manifest.ApprovalStates)
		}
	}
	for _, want := range []string{"validation", "approval_denied", "capability_unavailable", "artifact_unavailable"} {
		if !containsGenericToolContractsString(manifest.NormalizedErrorKinds, want) {
			t.Fatalf("normalized errors missing %q: %#v", want, manifest.NormalizedErrorKinds)
		}
	}
	if manifest.ArtifactRefPolicy.Mode != "metadata_ref_only" ||
		manifest.ArtifactRefPolicy.InlinePayloadDefault ||
		!containsGenericToolContractsString(manifest.ArtifactRefPolicy.RequiredFields, "artifact_id") ||
		!containsGenericToolContractsString(manifest.ArtifactRefPolicy.RequiredFields, "replay_key") {
		t.Fatalf("artifact ref policy incomplete: %#v", manifest.ArtifactRefPolicy)
	}
	if !manifest.NoBuiltInGuarantee.Required {
		t.Fatal("generic tool contracts package must declare no built-in guarantee")
	}
	if !strings.Contains(manifest.NoBuiltInGuarantee.Statement, manifest.PackageName) || !strings.Contains(manifest.NoBuiltInGuarantee.Statement, "provider-free package boundary") {
		t.Fatalf("no built-in guarantee should name package boundary: %q", manifest.NoBuiltInGuarantee.Statement)
	}
	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"declarative tool schema", "arguments", "capability tags", "approval state", "result envelopes", "normalized errors", "artifact refs", "registry descriptors project"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotGenericToolContractsContractAndFixtures(t *testing.T) {
	base := genericToolContractsLivePackageDir(t)
	var contract struct {
		DialectExports        []string `json:"dialect_exports"`
		ProviderFree          bool     `json:"provider_free"`
		LiveNetwork           bool     `json:"live_network"`
		RealDependencyImports bool     `json:"real_dependency_imports"`
		DeclarativeToolSchema bool     `json:"declarative_tool_schema"`
		ArgumentValidation    struct {
			Mode                 string `json:"mode"`
			RequiredBeforeInvoke bool   `json:"required_before_invoke"`
			FailureErrorKind     string `json:"failure_error_kind"`
			SchemaRef            string `json:"schema_ref"`
		} `json:"argument_validation"`
		ApprovalState struct {
			RequiredField              string   `json:"required_field"`
			DefaultState               string   `json:"default_state"`
			AllowedStates              []string `json:"allowed_states"`
			DenyWithoutExplicitFixture bool     `json:"deny_without_explicit_fixture"`
		} `json:"approval_state"`
		ResultEnvelope struct {
			SchemaRef        string   `json:"schema_ref"`
			RequiredFields   []string `json:"required_fields"`
			SuccessErrorNull bool     `json:"success_error_null"`
			FailureValueNull bool     `json:"failure_value_null"`
		} `json:"result_envelope"`
		NormalizedError struct {
			SchemaRef      string   `json:"schema_ref"`
			ProviderFree   bool     `json:"provider_free"`
			RequiredFields []string `json:"required_fields"`
			Kinds          []string `json:"kinds"`
		} `json:"normalized_error"`
		ArtifactRefs struct {
			SchemaRef            string   `json:"schema_ref"`
			MetadataRefOnly      bool     `json:"metadata_ref_only"`
			InlinePayloadDefault bool     `json:"inline_payload_default"`
			RequiredFields       []string `json:"required_fields"`
		} `json:"artifact_refs"`
		Tools []struct {
			Name              string   `json:"name"`
			CapabilityTags    []string `json:"capability_tags"`
			DeclarativeSchema struct {
				Type                 string         `json:"type"`
				AdditionalProperties bool           `json:"additionalProperties"`
				Required             []string       `json:"required"`
				Properties           map[string]any `json:"properties"`
			} `json:"declarative_schema"`
			Approval struct {
				Required bool   `json:"required"`
				State    string `json:"state"`
				Policy   string `json:"policy"`
			} `json:"approval"`
			ResultSchemaRef   string   `json:"result_schema_ref"`
			ErrorSchemaRef    string   `json:"error_schema_ref"`
			ArtifactSchemaRef string   `json:"artifact_schema_ref"`
			FixtureKeys       []string `json:"fixture_keys"`
		} `json:"tools"`
	}
	decodeGenericToolContractsJSONFile(t, filepath.Join(base, "contracts", "generic_tool_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || !contract.DeclarativeToolSchema {
		t.Fatalf("contract header must stay provider-free and declarative: %#v", contract)
	}
	if !reflect.DeepEqual(contract.DialectExports, []string{"generic.ai.tool.contract", "ai.tool.invoke"}) {
		t.Fatalf("contract dialect exports = %#v", contract.DialectExports)
	}
	if contract.ArgumentValidation.Mode != "json_schema" ||
		!contract.ArgumentValidation.RequiredBeforeInvoke ||
		contract.ArgumentValidation.FailureErrorKind != "validation" ||
		contract.ArgumentValidation.SchemaRef == "" {
		t.Fatalf("argument validation contract incomplete: %#v", contract.ArgumentValidation)
	}
	assertGenericToolContractsSchemaRef(t, contract.ArgumentValidation.SchemaRef)
	if contract.ApprovalState.RequiredField != "approval" ||
		contract.ApprovalState.DefaultState != "fixture_only" ||
		!contract.ApprovalState.DenyWithoutExplicitFixture ||
		!containsGenericToolContractsString(contract.ApprovalState.AllowedStates, "approved") ||
		!containsGenericToolContractsString(contract.ApprovalState.AllowedStates, "denied") {
		t.Fatalf("approval state contract incomplete: %#v", contract.ApprovalState)
	}
	for _, want := range []string{"ok", "value", "error", "artifact_refs", "replay", "approval"} {
		if !containsGenericToolContractsString(contract.ResultEnvelope.RequiredFields, want) {
			t.Fatalf("result envelope missing %q: %#v", want, contract.ResultEnvelope)
		}
	}
	if !contract.ResultEnvelope.SuccessErrorNull || !contract.ResultEnvelope.FailureValueNull {
		t.Fatalf("result envelope null semantics missing: %#v", contract.ResultEnvelope)
	}
	assertGenericToolContractsSchemaRef(t, contract.ResultEnvelope.SchemaRef)
	for _, want := range []string{"kind", "message", "field", "retryable", "provider_free"} {
		if !containsGenericToolContractsString(contract.NormalizedError.RequiredFields, want) {
			t.Fatalf("normalized error missing %q: %#v", want, contract.NormalizedError)
		}
	}
	if !contract.NormalizedError.ProviderFree || !containsGenericToolContractsString(contract.NormalizedError.Kinds, "validation") {
		t.Fatalf("normalized error taxonomy incomplete: %#v", contract.NormalizedError)
	}
	assertGenericToolContractsSchemaRef(t, contract.NormalizedError.SchemaRef)
	if !contract.ArtifactRefs.MetadataRefOnly || contract.ArtifactRefs.InlinePayloadDefault {
		t.Fatalf("artifact ref policy must be metadata-only: %#v", contract.ArtifactRefs)
	}
	for _, want := range []string{"artifact_id", "uri", "media_type", "sha256", "bytes", "provenance", "replay_key"} {
		if !containsGenericToolContractsString(contract.ArtifactRefs.RequiredFields, want) {
			t.Fatalf("artifact refs missing %q: %#v", want, contract.ArtifactRefs)
		}
	}
	assertGenericToolContractsSchemaRef(t, contract.ArtifactRefs.SchemaRef)
	if len(contract.Tools) != 1 {
		t.Fatalf("tools = %d, want 1", len(contract.Tools))
	}
	tool := contract.Tools[0]
	if tool.Name != "fixture.lookup" ||
		tool.DeclarativeSchema.Type != "object" ||
		tool.DeclarativeSchema.AdditionalProperties ||
		!containsGenericToolContractsString(tool.DeclarativeSchema.Required, "ticker") ||
		!containsGenericToolContractsString(tool.DeclarativeSchema.Required, "horizon") ||
		len(tool.DeclarativeSchema.Properties) != 2 {
		t.Fatalf("tool schema incomplete: %#v", tool)
	}
	for _, want := range []string{"generic.ai.tool.contract", "ai.tool.invoke", "ai.tool.arguments.validate", "ai.tool.result.envelope", "ai.tool.artifact.refs"} {
		if !containsGenericToolContractsString(tool.CapabilityTags, want) {
			t.Fatalf("tool capability tags missing %q: %#v", want, tool.CapabilityTags)
		}
	}
	assertGenericToolContractsCapabilityTags(t, tool.CapabilityTags)
	if tool.Approval.Required || tool.Approval.State != "fixture_only" || tool.ResultSchemaRef == "" || tool.ErrorSchemaRef == "" || tool.ArtifactSchemaRef == "" || len(tool.FixtureKeys) != 2 {
		t.Fatalf("tool approval/schema refs incomplete: %#v", tool)
	}
	assertGenericToolContractsSchemaRef(t, tool.ResultSchemaRef)
	assertGenericToolContractsSchemaRef(t, tool.ErrorSchemaRef)
	assertGenericToolContractsSchemaRef(t, tool.ArtifactSchemaRef)
	if !containsGenericToolContractsString(tool.FixtureKeys, "generic_tool:invoke:fixture.lookup:ACME:1d:success") ||
		!containsGenericToolContractsString(tool.FixtureKeys, "generic_tool:invoke:fixture.lookup:bad-args:validation") {
		t.Fatalf("tool fixture keys must cover success and validation error: %#v", tool.FixtureKeys)
	}

	assertGenericToolContractsInvokeFixture(t, filepath.Join(base, "fixtures", "invoke_success_fixture.json"), true, "fixture_only", "", 1)
	assertGenericToolContractsInvokeFixture(t, filepath.Join(base, "fixtures", "invoke_validation_error_fixture.json"), false, "fixture_only", "validation", 0)
}

func TestFinRobotGenericToolContractsFixtureIndexAndSmoke(t *testing.T) {
	base := genericToolContractsLivePackageDir(t)
	var index struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Fixtures              []struct {
			FixtureKey            string   `json:"fixture_key"`
			Path                  string   `json:"path"`
			Schema                string   `json:"schema"`
			DialectExport         string   `json:"dialect_export"`
			ToolName              string   `json:"tool_name"`
			CapabilityTags        []string `json:"capability_tags"`
			ApprovalState         string   `json:"approval_state"`
			ExpectedOK            bool     `json:"expected_ok"`
			ReplayReady           bool     `json:"replay_ready"`
			ProviderFree          bool     `json:"provider_free"`
			LiveNetwork           bool     `json:"live_network"`
			RealDependencyImports bool     `json:"real_dependency_imports"`
		} `json:"fixtures"`
	}
	decodeGenericToolContractsJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 3 {
		t.Fatalf("fixture index header/count = %#v", index)
	}
	keys := map[string]bool{}
	coveredOutcomes := map[bool]bool{}
	coveredProjection := false
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Path == "" || fixture.DialectExport == "" || fixture.ToolName == "" {
			t.Fatalf("fixture metadata incomplete: %#v", fixture)
		}
		switch fixture.FixtureKey {
		case "generic_tool:projection:registry_descriptor:tool_contract:v1":
			if fixture.DialectExport != "generic.ai.tool.contract.project_registry_descriptor" ||
				fixture.ToolName != "registry_descriptor_projection" ||
				fixture.Schema != "schemas/tool_contract_projection_v1.schema.json" {
				t.Fatalf("projection fixture metadata incomplete: %#v", fixture)
			}
			coveredProjection = true
		default:
			if fixture.DialectExport != "ai.tool.invoke" || fixture.ToolName != "fixture.lookup" || fixture.Schema != "" {
				t.Fatalf("invoke fixture metadata incomplete: %#v", fixture)
			}
			coveredOutcomes[fixture.ExpectedOK] = true
		}
		assertGenericToolContractsRefString(t, "fixture key", fixture.FixtureKey, "generic_tool:")
		assertGenericToolContractsCapabilityTags(t, fixture.CapabilityTags)
		if keys[fixture.FixtureKey] {
			t.Fatalf("duplicate fixture key %q", fixture.FixtureKey)
		}
		keys[fixture.FixtureKey] = true
		if fixture.ApprovalState != "fixture_only" || !fixture.ReplayReady {
			t.Fatalf("fixture must be replay-ready and fixture-only: %#v", fixture)
		}
		if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports {
			t.Fatalf("fixture metadata must stay provider-free/offline/no-import: %#v", fixture)
		}
		assertGenericToolContractsJSONFile(t, filepath.Join(base, fixture.Path))
	}
	if !coveredOutcomes[true] || !coveredOutcomes[false] || !coveredProjection {
		t.Fatalf("fixture index must cover success, validation error, and registry projection paths: %#v", keys)
	}

	mainPath := filepath.Join(base, "main.leia")
	sourceBytes, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, want := range []string{"generic.ai.tool.contract", "ai.tool.invoke", "generic.ai.tool.contract.project_registry_descriptor", "declarative_schema", "capability_tags", "approval", "artifact_refs", "validation"} {
		if !strings.Contains(source, want) {
			t.Fatalf("main.leia missing %q", want)
		}
	}
	for _, forbidden := range []string{"q/runtime", ".external/FinRobot", "import q", "$`", "$!`"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("main.leia must stay provider-free and avoid restricted surfaces; found %q", forbidden)
		}
	}

	want := "generic_tool_contracts dialect_exports=2 projection_exports=1 provider_free=true live_network=false success_artifacts=1 validation_error=validation projected_tools=2"
	for _, result := range runFinRobotLivePackageSummarySmoke(t, mainPath, "generic_tool_contracts_live_package_summary", "generic_tool_contracts", leia.LibAll) {
		if result.Summary != want {
			t.Fatalf("summary = %#v, want %#v", result.Summary, want)
		}
		fields := result.Fields
		requireFinRobotSummaryFields(t, fields, "dialect_exports", "projection_exports", "provider_free", "live_network", "success_artifacts", "validation_error", "projected_tools")
		if fields["dialect_exports"] != "2" ||
			fields["projection_exports"] != "1" ||
			fields["provider_free"] != "true" ||
			fields["live_network"] != "false" ||
			fields["success_artifacts"] != "1" ||
			fields["validation_error"] != "validation" ||
			fields["projected_tools"] != "2" {
			t.Fatalf("summary fields = %#v", fields)
		}
	}
}

func TestGenericToolContractsProjectionManifestContractFixtureRefsBidirectional(t *testing.T) {
	contractsBase := genericToolContractsLivePackageDir(t)
	registryBase := genericToolRegistryDir(t)
	manifest := loadGenericToolContractsManifest(t, contractsBase)
	registryManifest := loadGenericToolRegistryManifest(t, registryBase)
	registryIndex := loadGenericToolRegistryFixtureIndex(t, registryBase)

	manifestCapabilities := genericToolContractsCapabilitySet(manifest.Capabilities)
	manifestSchemas := genericToolContractsValueSet(manifest.Schemas)
	manifestFixtures := genericToolContractsValueSet(manifest.Fixtures)

	var contract struct {
		ArgumentValidation struct {
			SchemaRef string `json:"schema_ref"`
		} `json:"argument_validation"`
		ResultEnvelope struct {
			SchemaRef string `json:"schema_ref"`
		} `json:"result_envelope"`
		NormalizedError struct {
			SchemaRef string `json:"schema_ref"`
		} `json:"normalized_error"`
		ArtifactRefs struct {
			SchemaRef string `json:"schema_ref"`
		} `json:"artifact_refs"`
		Tools []struct {
			CapabilityTags    []string `json:"capability_tags"`
			ResultSchemaRef   string   `json:"result_schema_ref"`
			ErrorSchemaRef    string   `json:"error_schema_ref"`
			ArtifactSchemaRef string   `json:"artifact_schema_ref"`
			FixtureKeys       []string `json:"fixture_keys"`
		} `json:"tools"`
	}
	decodeGenericToolContractsJSONFile(t, filepath.Join(contractsBase, "contracts", "generic_tool_contract.json"), &contract)
	for _, schemaRef := range []string{
		contract.ArgumentValidation.SchemaRef,
		contract.ResultEnvelope.SchemaRef,
		contract.NormalizedError.SchemaRef,
		contract.ArtifactRefs.SchemaRef,
	} {
		if !manifestSchemas[schemaRef] {
			t.Fatalf("contract schema ref %q missing from generic tool contracts manifest schemas %#v", schemaRef, manifest.Schemas)
		}
	}

	var fixtureIndex struct {
		Fixtures []struct {
			FixtureKey     string   `json:"fixture_key"`
			Capability     string   `json:"capability"`
			Path           string   `json:"path"`
			Schema         string   `json:"schema"`
			DialectExport  string   `json:"dialect_export"`
			CapabilityTags []string `json:"capability_tags"`
		} `json:"fixtures"`
	}
	decodeGenericToolContractsJSONFile(t, filepath.Join(contractsBase, "fixtures", "provider_free_fixture_index.json"), &fixtureIndex)
	fixtureIndexByKey := map[string]struct {
		Capability string
		Path       string
		Schema     string
	}{}
	for _, fixture := range fixtureIndex.Fixtures {
		if !manifestCapabilities[fixture.Capability] {
			t.Fatalf("fixture index capability %q missing from manifest capabilities %#v", fixture.Capability, manifest.Capabilities)
		}
		if !manifestCapabilities[fixture.DialectExport] {
			t.Fatalf("fixture index dialect export %q missing from manifest capabilities %#v", fixture.DialectExport, manifest.Capabilities)
		}
		if !manifestFixtures[fixture.Path] {
			t.Fatalf("fixture index path %q missing from manifest fixtures %#v", fixture.Path, manifest.Fixtures)
		}
		if fixture.Schema != "" && !manifestSchemas[fixture.Schema] {
			t.Fatalf("fixture index schema %q missing from manifest schemas %#v", fixture.Schema, manifest.Schemas)
		}
		for _, tag := range fixture.CapabilityTags {
			if !manifestCapabilities[tag] {
				t.Fatalf("fixture index capability tag %q missing from manifest capabilities %#v", tag, manifest.Capabilities)
			}
		}
		fixtureIndexByKey[fixture.FixtureKey] = struct {
			Capability string
			Path       string
			Schema     string
		}{
			Capability: fixture.Capability,
			Path:       fixture.Path,
			Schema:     fixture.Schema,
		}
	}
	for _, tool := range contract.Tools {
		for _, tag := range tool.CapabilityTags {
			if !manifestCapabilities[tag] {
				t.Fatalf("contract tool capability tag %q missing from manifest capabilities %#v", tag, manifest.Capabilities)
			}
		}
		for _, schemaRef := range []string{tool.ResultSchemaRef, tool.ErrorSchemaRef, tool.ArtifactSchemaRef} {
			if !manifestSchemas[schemaRef] {
				t.Fatalf("contract tool schema ref %q missing from manifest schemas %#v", schemaRef, manifest.Schemas)
			}
		}
		for _, fixtureKey := range tool.FixtureKeys {
			if fixtureIndexByKey[fixtureKey].Path == "" {
				t.Fatalf("contract tool fixture key %q missing from fixture index", fixtureKey)
			}
		}
	}

	var projection struct {
		FixtureKey        string `json:"fixture_key"`
		ProjectionKind    string `json:"projection_kind"`
		SourceFixtureRefs struct {
			RegistryReadOnly        string `json:"registry_read_only"`
			RegistryEffectfulDenied string `json:"registry_effectful_denied"`
			ToolContract            string `json:"tool_contract"`
		} `json:"source_fixture_refs"`
		DescriptorProjections []struct {
			SourceDescriptor struct {
				ToolName         string   `json:"tool_name"`
				CapabilityIDs    []string `json:"capability_ids"`
				SourceFixtureKey string   `json:"source_fixture_key"`
			} `json:"source_descriptor"`
			ProjectedToolContract struct {
				CapabilityTags    []string `json:"capability_tags"`
				ResultSchemaRef   string   `json:"result_schema_ref"`
				ErrorSchemaRef    string   `json:"error_schema_ref"`
				ArtifactSchemaRef string   `json:"artifact_schema_ref"`
			} `json:"projected_tool_contract"`
			ProjectedInvokeRequest struct {
				CapabilityTags []string `json:"capability_tags"`
				ReplayKey      string   `json:"replay_key"`
			} `json:"projected_invoke_request"`
			ProjectedResultEnvelope struct {
				Replay struct {
					ReplayKey string `json:"replay_key"`
				} `json:"replay"`
			} `json:"projected_result_envelope"`
			CapabilityMap []struct {
				Source string `json:"source"`
				Target string `json:"target"`
			} `json:"capability_map"`
		} `json:"descriptor_projections"`
	}
	projectionRel := manifest.Fixtures["registry_descriptor_projection"]
	decodeGenericToolContractsJSONFile(t, filepath.Join(contractsBase, projectionRel), &projection)
	if fixtureIndexByKey[projection.FixtureKey].Path != projectionRel {
		t.Fatalf("projection fixture key %q path mismatch: index=%#v manifest=%q", projection.FixtureKey, fixtureIndexByKey[projection.FixtureKey], projectionRel)
	}
	if fixtureIndexByKey[projection.FixtureKey].Capability != "generic.ai.tool.contract.project_registry_descriptor" ||
		fixtureIndexByKey[projection.FixtureKey].Schema != manifest.Schemas["tool_contract_projection"] {
		t.Fatalf("projection fixture index entry does not match manifest capability/schema refs: %#v", fixtureIndexByKey[projection.FixtureKey])
	}
	if projection.ProjectionKind != "registry_descriptor_to_tool_contract" {
		t.Fatalf("projection kind = %q", projection.ProjectionKind)
	}
	if filepath.Clean(projection.SourceFixtureRefs.ToolContract) != manifest.Entrypoints["generic_tool_contract"] {
		t.Fatalf("projection tool contract ref %q does not mirror manifest entrypoint %q", projection.SourceFixtureRefs.ToolContract, manifest.Entrypoints["generic_tool_contract"])
	}

	registryCapabilities := map[string]string{}
	for _, capability := range registryManifest.Capabilities {
		registryCapabilities[capability.ID] = capability.Schema
	}
	registryFixtureByKey := map[string]struct {
		Path      string
		SchemaRef string
	}{}
	for _, fixture := range registryIndex.Fixtures {
		registryFixtureByKey[fixture.FixtureKey] = struct {
			Path      string
			SchemaRef string
		}{
			Path:      fixture.Path,
			SchemaRef: fixture.SchemaRef,
		}
	}
	sourceRefByKey := map[string]string{
		"generic_tool:invocation:trace:v1": projection.SourceFixtureRefs.RegistryReadOnly,
		"generic_tool:approval:denied:v1":  projection.SourceFixtureRefs.RegistryEffectfulDenied,
	}
	for _, descriptorProjection := range projection.DescriptorProjections {
		sourceKey := descriptorProjection.SourceDescriptor.SourceFixtureKey
		registryEntry := registryFixtureByKey[sourceKey]
		if registryEntry.Path == "" {
			t.Fatalf("projection source fixture key %q missing from registry fixture index", sourceKey)
		}
		wantSourceRef := filepath.ToSlash(filepath.Join("..", "generic_tool_registry", registryEntry.Path))
		if sourceRefByKey[sourceKey] != wantSourceRef {
			t.Fatalf("projection source ref for %q = %q, want %q", sourceKey, sourceRefByKey[sourceKey], wantSourceRef)
		}
		sourceFixture := loadGenericToolRegistryFixture(t, filepath.Join(registryBase, filepath.FromSlash(registryEntry.Path)))
		if sourceFixture.Trace.Provenance.FixtureKey != sourceKey ||
			sourceFixture.Descriptor.ToolName != descriptorProjection.SourceDescriptor.ToolName {
			t.Fatalf("projection source fixture %q does not round-trip to registry trace/descriptor: source=%#v projection=%#v", sourceKey, sourceFixture.Trace.Provenance, descriptorProjection.SourceDescriptor)
		}
		for _, capability := range descriptorProjection.SourceDescriptor.CapabilityIDs {
			if registryCapabilities[capability] == "" {
				t.Fatalf("projection source capability %q missing from registry manifest capabilities", capability)
			}
		}
		for _, mapping := range descriptorProjection.CapabilityMap {
			if registryCapabilities[mapping.Source] == "" {
				t.Fatalf("projection capability map source %q missing from registry manifest capabilities", mapping.Source)
			}
			if !manifestCapabilities[mapping.Target] {
				t.Fatalf("projection capability map target %q missing from contracts manifest capabilities", mapping.Target)
			}
		}
		for _, tag := range append(descriptorProjection.ProjectedToolContract.CapabilityTags, descriptorProjection.ProjectedInvokeRequest.CapabilityTags...) {
			if !manifestCapabilities[tag] {
				t.Fatalf("projection capability tag %q missing from contracts manifest capabilities", tag)
			}
		}
		for _, schemaRef := range []string{
			descriptorProjection.ProjectedToolContract.ResultSchemaRef,
			descriptorProjection.ProjectedToolContract.ErrorSchemaRef,
			descriptorProjection.ProjectedToolContract.ArtifactSchemaRef,
		} {
			if !manifestSchemas[schemaRef] {
				t.Fatalf("projection schema ref %q missing from contracts manifest schemas %#v", schemaRef, manifest.Schemas)
			}
		}
		if descriptorProjection.ProjectedInvokeRequest.ReplayKey != sourceKey ||
			descriptorProjection.ProjectedResultEnvelope.Replay.ReplayKey != sourceKey {
			t.Fatalf("projection replay keys do not mirror source fixture key %q: %#v", sourceKey, descriptorProjection)
		}
	}
}

func TestGenericToolRegistryProjectsDescriptorToGenericToolContracts(t *testing.T) {
	contractsBase := genericToolContractsLivePackageDir(t)
	registryBase := genericToolRegistryDir(t)
	readOnly := loadGenericToolRegistryFixture(t, filepath.Join(registryBase, "fixtures", "tool_registry_replay_trace_fixture.json"))
	denied := loadGenericToolRegistryFixture(t, filepath.Join(registryBase, "fixtures", "approval_edge_fixture.json"))

	var projection struct {
		SchemaVersion        int    `json:"schema_version"`
		FixtureKey           string `json:"fixture_key"`
		ProjectionKind       string `json:"projection_kind"`
		ProviderFree         bool   `json:"provider_free"`
		LiveNetwork          bool   `json:"live_network"`
		RealDependencyImport bool   `json:"real_dependency_imports"`
		SourceFixtureRefs    struct {
			RegistryReadOnly        string `json:"registry_read_only"`
			RegistryEffectfulDenied string `json:"registry_effectful_denied"`
			ToolContract            string `json:"tool_contract"`
		} `json:"source_fixture_refs"`
		DescriptorProjections []struct {
			SourceDescriptor struct {
				ToolName         string   `json:"tool_name"`
				Effect           string   `json:"effect"`
				ApprovalPolicy   string   `json:"approval_policy"`
				CapabilityIDs    []string `json:"capability_ids"`
				InputRequired    []string `json:"input_required"`
				OutputRequired   []string `json:"output_required"`
				SourceFixtureKey string   `json:"source_fixture_key"`
			} `json:"source_descriptor"`
			ProjectedToolContract struct {
				Name              string `json:"name"`
				Description       string `json:"description"`
				DeclarativeSchema struct {
					Type                 string   `json:"type"`
					Required             []string `json:"required"`
					AdditionalProperties bool     `json:"additionalProperties"`
				} `json:"declarative_schema"`
				CapabilityTags []string `json:"capability_tags"`
				Approval       struct {
					Required bool   `json:"required"`
					State    string `json:"state"`
					Policy   string `json:"policy"`
				} `json:"approval"`
				ResultSchemaRef   string `json:"result_schema_ref"`
				ErrorSchemaRef    string `json:"error_schema_ref"`
				ArtifactSchemaRef string `json:"artifact_schema_ref"`
			} `json:"projected_tool_contract"`
			ProjectedInvokeRequest struct {
				ToolName       string   `json:"tool_name"`
				CapabilityTags []string `json:"capability_tags"`
				Approval       struct {
					Required bool   `json:"required"`
					State    string `json:"state"`
					Policy   string `json:"policy"`
				} `json:"approval"`
				ReplayKey string `json:"replay_key"`
			} `json:"projected_invoke_request"`
			ProjectedResultEnvelope struct {
				OK       bool   `json:"ok"`
				ToolName string `json:"tool_name"`
				Approval struct {
					Required bool   `json:"required"`
					State    string `json:"state"`
					Policy   string `json:"policy"`
				} `json:"approval"`
				Error *struct {
					Kind         string `json:"kind"`
					Field        string `json:"field"`
					Retryable    bool   `json:"retryable"`
					ProviderFree bool   `json:"provider_free"`
				} `json:"error"`
				Replay struct {
					ReplayKey    string `json:"replay_key"`
					ProviderFree bool   `json:"provider_free"`
					LiveNetwork  bool   `json:"live_network"`
				} `json:"replay"`
			} `json:"projected_result_envelope"`
			CapabilityMap []struct {
				Source string `json:"source"`
				Target string `json:"target"`
			} `json:"capability_map"`
			FieldMap []struct {
				Source string `json:"source"`
				Target string `json:"target"`
			} `json:"field_map"`
			ApprovalMap struct {
				SourcePolicy       string `json:"source_policy"`
				SourceRequired     bool   `json:"source_required"`
				SourceDecision     string `json:"source_decision"`
				ProjectedRequired  bool   `json:"projected_required"`
				ProjectedState     string `json:"projected_state"`
				ProjectedErrorKind string `json:"projected_error_kind"`
			} `json:"approval_map"`
		} `json:"descriptor_projections"`
		ProjectionAssertions map[string]bool `json:"projection_assertions"`
	}
	decodeGenericToolContractsJSONFile(t, filepath.Join(contractsBase, "fixtures", "registry_descriptor_to_tool_contract_projection_fixture.json"), &projection)

	if projection.SchemaVersion != 1 ||
		projection.FixtureKey != "generic_tool:projection:registry_descriptor:tool_contract:v1" ||
		projection.ProjectionKind != "registry_descriptor_to_tool_contract" ||
		!projection.ProviderFree || projection.LiveNetwork || projection.RealDependencyImport ||
		projection.SourceFixtureRefs.RegistryReadOnly == "" ||
		projection.SourceFixtureRefs.RegistryEffectfulDenied == "" ||
		projection.SourceFixtureRefs.ToolContract == "" ||
		len(projection.DescriptorProjections) != 2 {
		t.Fatalf("projection header incomplete: %#v", projection)
	}
	for assertion, ok := range projection.ProjectionAssertions {
		if !ok {
			t.Fatalf("projection assertion %q is false", assertion)
		}
	}

	projectionsByTool := map[string]int{}
	for i, descriptorProjection := range projection.DescriptorProjections {
		toolName := descriptorProjection.SourceDescriptor.ToolName
		projectionsByTool[toolName] = i
		if toolName == "" ||
			descriptorProjection.ProjectedToolContract.Name != toolName ||
			descriptorProjection.ProjectedInvokeRequest.ToolName != toolName ||
			descriptorProjection.ProjectedResultEnvelope.ToolName != toolName {
			t.Fatalf("tool name not preserved through projection: %#v", descriptorProjection)
		}
		if descriptorProjection.ProjectedToolContract.DeclarativeSchema.Type != "object" ||
			descriptorProjection.ProjectedToolContract.DeclarativeSchema.AdditionalProperties ||
			!reflect.DeepEqual(descriptorProjection.ProjectedToolContract.DeclarativeSchema.Required, descriptorProjection.SourceDescriptor.InputRequired) {
			t.Fatalf("declarative schema not projected from input schema: %#v", descriptorProjection.ProjectedToolContract.DeclarativeSchema)
		}
		for _, ref := range []string{
			descriptorProjection.ProjectedToolContract.ResultSchemaRef,
			descriptorProjection.ProjectedToolContract.ErrorSchemaRef,
			descriptorProjection.ProjectedToolContract.ArtifactSchemaRef,
		} {
			assertGenericToolContractsSchemaRef(t, ref)
		}
		assertGenericToolContractsCapabilityTags(t, descriptorProjection.ProjectedToolContract.CapabilityTags)
		assertGenericToolContractsCapabilityTags(t, descriptorProjection.ProjectedInvokeRequest.CapabilityTags)
		if !genericToolContractsMappingContains(descriptorProjection.FieldMap, "descriptor.tool_name", "projected_tool_contract.name") ||
			!genericToolContractsMappingContains(descriptorProjection.FieldMap, "descriptor.input_schema", "projected_tool_contract.declarative_schema") {
			t.Fatalf("projection field map incomplete: %#v", descriptorProjection.FieldMap)
		}
	}

	readOnlyProjection := projection.DescriptorProjections[projectionsByTool[readOnly.Descriptor.ToolName]]
	readOnlyInputRequired := genericToolContractsSchemaRequired(t, readOnly.Descriptor.InputSchema)
	if readOnlyProjection.SourceDescriptor.ToolName != readOnly.Descriptor.ToolName ||
		readOnlyProjection.SourceDescriptor.Effect != readOnly.Descriptor.Effect ||
		readOnlyProjection.SourceDescriptor.ApprovalPolicy != readOnly.Descriptor.ApprovalPolicy ||
		!reflect.DeepEqual(readOnlyProjection.SourceDescriptor.CapabilityIDs, readOnly.Descriptor.CapabilityIDs) ||
		!reflect.DeepEqual(readOnlyProjection.SourceDescriptor.InputRequired, readOnlyInputRequired) ||
		readOnlyProjection.SourceDescriptor.SourceFixtureKey != readOnly.Trace.Provenance.FixtureKey ||
		readOnlyProjection.ProjectedInvokeRequest.ReplayKey != readOnly.Trace.Provenance.FixtureKey ||
		readOnlyProjection.ProjectedResultEnvelope.Replay.ReplayKey != readOnly.Trace.Provenance.FixtureKey {
		t.Fatalf("read-only registry fixture not projected faithfully: %#v", readOnlyProjection.SourceDescriptor)
	}
	if readOnly.Trace.Approval.Required ||
		readOnlyProjection.ProjectedToolContract.Approval.Required ||
		readOnlyProjection.ProjectedResultEnvelope.Approval.Required ||
		readOnlyProjection.ProjectedResultEnvelope.Error != nil ||
		!readOnlyProjection.ProjectedResultEnvelope.OK ||
		!readOnlyProjection.ProjectedResultEnvelope.Replay.ProviderFree ||
		readOnlyProjection.ProjectedResultEnvelope.Replay.LiveNetwork ||
		!genericToolContractsMappingContains(readOnlyProjection.CapabilityMap, "generic.tool.registry.validate_schema", "ai.tool.schema.declarative") {
		t.Fatalf("read-only projection approval/replay semantics invalid: %#v", readOnlyProjection)
	}

	deniedProjection := projection.DescriptorProjections[projectionsByTool[denied.Descriptor.ToolName]]
	deniedInputRequired := genericToolContractsSchemaRequired(t, denied.Descriptor.InputSchema)
	if deniedProjection.SourceDescriptor.ToolName != denied.Descriptor.ToolName ||
		deniedProjection.SourceDescriptor.Effect != denied.Descriptor.Effect ||
		deniedProjection.SourceDescriptor.ApprovalPolicy != denied.Descriptor.ApprovalPolicy ||
		!reflect.DeepEqual(deniedProjection.SourceDescriptor.CapabilityIDs, denied.Descriptor.CapabilityIDs) ||
		!reflect.DeepEqual(deniedProjection.SourceDescriptor.InputRequired, deniedInputRequired) ||
		deniedProjection.SourceDescriptor.SourceFixtureKey != denied.Trace.Provenance.FixtureKey ||
		deniedProjection.ProjectedInvokeRequest.ReplayKey != denied.Trace.Provenance.FixtureKey ||
		deniedProjection.ProjectedResultEnvelope.Replay.ReplayKey != denied.Trace.Provenance.FixtureKey {
		t.Fatalf("effectful denied registry fixture not projected faithfully: %#v", deniedProjection.SourceDescriptor)
	}
	if !denied.Trace.Approval.Required ||
		denied.Trace.Approval.Decision != "deny" ||
		!deniedProjection.ProjectedToolContract.Approval.Required ||
		deniedProjection.ProjectedToolContract.Approval.State != "denied" ||
		!deniedProjection.ProjectedInvokeRequest.Approval.Required ||
		deniedProjection.ProjectedInvokeRequest.Approval.State != "required" ||
		deniedProjection.ProjectedResultEnvelope.OK ||
		deniedProjection.ProjectedResultEnvelope.Error == nil ||
		deniedProjection.ProjectedResultEnvelope.Error.Kind != "approval_denied" ||
		deniedProjection.ProjectedResultEnvelope.Error.Retryable ||
		!deniedProjection.ProjectedResultEnvelope.Error.ProviderFree ||
		!deniedProjection.ProjectedResultEnvelope.Replay.ProviderFree ||
		deniedProjection.ProjectedResultEnvelope.Replay.LiveNetwork ||
		deniedProjection.ApprovalMap.SourceDecision != denied.Trace.Approval.Decision ||
		deniedProjection.ApprovalMap.ProjectedErrorKind != "approval_denied" ||
		denied.Trace.Result.Metadata["clean_skip"] != true ||
		denied.Trace.Result.Metadata["executed"] != false ||
		!genericToolContractsMappingContains(deniedProjection.CapabilityMap, "generic.tool.approval.edge", "ai.tool.approval.state") {
		t.Fatalf("effectful denied projection approval/error semantics invalid: %#v", deniedProjection)
	}
}

func assertGenericToolContractsInvokeFixture(t *testing.T, path string, wantOK bool, wantApproval string, wantErrorKind string, wantArtifacts int) {
	t.Helper()
	var fixture struct {
		Request struct {
			ToolName       string         `json:"tool_name"`
			Arguments      map[string]any `json:"arguments"`
			CapabilityTags []string       `json:"capability_tags"`
			Approval       struct {
				State    string `json:"state"`
				Required bool   `json:"required"`
			} `json:"approval"`
			ReplayKey    string `json:"replay_key"`
			ProviderFree bool   `json:"provider_free"`
			LiveNetwork  bool   `json:"live_network"`
		} `json:"request"`
		Result struct {
			OK             bool     `json:"ok"`
			ToolName       string   `json:"tool_name"`
			CapabilityTags []string `json:"capability_tags"`
			Approval       struct {
				State    string `json:"state"`
				Required bool   `json:"required"`
			} `json:"approval"`
			Value any `json:"value"`
			Error *struct {
				Kind         string `json:"kind"`
				Message      string `json:"message"`
				Field        string `json:"field"`
				Retryable    bool   `json:"retryable"`
				ProviderFree bool   `json:"provider_free"`
			} `json:"error"`
			ArtifactRefs []struct {
				ArtifactID string `json:"artifact_id"`
				URI        string `json:"uri"`
				MediaType  string `json:"media_type"`
				SHA256     string `json:"sha256"`
				Bytes      int    `json:"bytes"`
				Provenance struct {
					Source       string `json:"source"`
					ToolName     string `json:"tool_name"`
					ProviderFree bool   `json:"provider_free"`
					LiveNetwork  bool   `json:"live_network"`
				} `json:"provenance"`
				ReplayKey string `json:"replay_key"`
			} `json:"artifact_refs"`
			Replay struct {
				Mode          string `json:"mode"`
				ReplayKey     string `json:"replay_key"`
				Deterministic bool   `json:"deterministic"`
				ProviderFree  bool   `json:"provider_free"`
				LiveNetwork   bool   `json:"live_network"`
			} `json:"replay"`
		} `json:"result"`
	}
	decodeGenericToolContractsJSONFile(t, path, &fixture)
	if fixture.Request.ToolName != "fixture.lookup" ||
		fixture.Request.Approval.State != wantApproval ||
		fixture.Request.Approval.Required ||
		!fixture.Request.ProviderFree ||
		fixture.Request.LiveNetwork ||
		fixture.Request.ReplayKey == "" ||
		len(fixture.Request.Arguments) == 0 {
		t.Fatalf("request envelope incomplete: %#v", fixture.Request)
	}
	assertGenericToolContractsCapabilityTags(t, fixture.Request.CapabilityTags)
	assertGenericToolContractsRefString(t, "request replay key", fixture.Request.ReplayKey, "generic_tool:")
	if fixture.Result.OK != wantOK ||
		fixture.Result.ToolName != fixture.Request.ToolName ||
		fixture.Result.Approval.State != wantApproval ||
		fixture.Result.Approval.Required ||
		len(fixture.Result.ArtifactRefs) != wantArtifacts ||
		fixture.Result.Replay.Mode != "fixture_replay" ||
		fixture.Result.Replay.ReplayKey != fixture.Request.ReplayKey ||
		!fixture.Result.Replay.Deterministic ||
		!fixture.Result.Replay.ProviderFree ||
		fixture.Result.Replay.LiveNetwork {
		t.Fatalf("result envelope incomplete: %#v", fixture.Result)
	}
	assertGenericToolContractsCapabilityTags(t, fixture.Result.CapabilityTags)
	assertGenericToolContractsRefString(t, "result replay key", fixture.Result.Replay.ReplayKey, "generic_tool:")
	if wantOK {
		if fixture.Result.Error != nil || fixture.Result.Value == nil {
			t.Fatalf("success envelope value/error = value:%#v error:%#v", fixture.Result.Value, fixture.Result.Error)
		}
	} else {
		if fixture.Result.Value != nil || fixture.Result.Error == nil || fixture.Result.Error.Kind != wantErrorKind ||
			fixture.Result.Error.Field == "" || fixture.Result.Error.Retryable || !fixture.Result.Error.ProviderFree {
			t.Fatalf("error envelope incomplete: value:%#v error:%#v", fixture.Result.Value, fixture.Result.Error)
		}
		assertGenericToolContractsRefString(t, "error field", fixture.Result.Error.Field, "arguments.")
	}
	for _, artifact := range fixture.Result.ArtifactRefs {
		if artifact.ArtifactID == "" || !strings.HasPrefix(artifact.URI, "artifact://") ||
			artifact.MediaType == "" || len(artifact.SHA256) != 64 || artifact.Bytes <= 0 ||
			artifact.Provenance.Source != "fixture_replay" ||
			!artifact.Provenance.ProviderFree || artifact.Provenance.LiveNetwork ||
			artifact.ReplayKey == "" {
			t.Fatalf("artifact ref incomplete: %#v", artifact)
		}
		assertGenericToolContractsRefString(t, "artifact id", artifact.ArtifactID, "artifact_")
		assertGenericToolContractsArtifactURI(t, artifact.URI)
		assertGenericToolContractsRefString(t, "artifact replay key", artifact.ReplayKey, "generic_tool:")
	}
}

func genericToolContractsLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_tool_contracts")
}

func loadGenericToolContractsManifest(t *testing.T, base string) genericToolContractsManifest {
	t.Helper()
	var manifest genericToolContractsManifest
	decodeGenericToolContractsJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func genericToolContractsCapabilitySet(capabilities []string) map[string]bool {
	set := map[string]bool{}
	for _, capability := range capabilities {
		set[capability] = true
	}
	return set
}

func genericToolContractsValueSet(values map[string]string) map[string]bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return set
}

func assertGenericToolContractsJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeGenericToolContractsJSONFile(t, path, &value)
}

func assertGenericToolContractsEntrypointPath(t *testing.T, path string) {
	t.Helper()
	if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path {
		t.Fatalf("entrypoint must be a clean relative file path: %q", path)
	}
	switch filepath.Ext(path) {
	case ".json", ".leia":
	default:
		t.Fatalf("entrypoint must reference a JSON or Leia file path: %q", path)
	}
}

func assertGenericToolContractsSchemaRef(t *testing.T, ref string) {
	t.Helper()
	if ref == "" {
		t.Fatal("schema ref must be non-empty")
	}
	path, _, _ := strings.Cut(ref, "#")
	if filepath.IsAbs(path) || filepath.Clean(path) != path || !strings.HasPrefix(path, "schemas/") || !strings.HasSuffix(path, ".schema.json") {
		t.Fatalf("schema ref must be an explainable relative schemas/*.schema.json ref: %q", ref)
	}
	assertGenericToolContractsProviderFreeString(t, "schema ref", ref)
}

func assertGenericToolContractsCapabilityTags(t *testing.T, tags []string) {
	t.Helper()
	if len(tags) == 0 {
		t.Fatal("capability tags must be explicit")
	}
	seen := map[string]bool{}
	for _, tag := range tags {
		if tag == "" || seen[tag] {
			t.Fatalf("capability tags must be non-empty and unique: %#v", tags)
		}
		seen[tag] = true
		if !(strings.HasPrefix(tag, "generic.ai.") || strings.HasPrefix(tag, "ai.tool.")) {
			t.Fatalf("capability tag must use generic AI/tool namespace: %q", tag)
		}
		for _, r := range tag {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '.' && r != '-' && r != '_' {
				t.Fatalf("capability tag must be lowercase provider-neutral token: %q", tag)
			}
		}
		assertGenericToolContractsProviderFreeString(t, "capability tag", tag)
	}
}

func assertGenericToolContractsArtifactURI(t *testing.T, uri string) {
	t.Helper()
	if !strings.HasPrefix(uri, "artifact://") || strings.Contains(uri, "://http") {
		t.Fatalf("artifact refs must be artifact:// metadata refs only: %q", uri)
	}
	assertGenericToolContractsProviderFreeString(t, "artifact uri", uri)
}

func assertGenericToolContractsRefString(t *testing.T, label, value, prefix string) {
	t.Helper()
	if value == "" || !strings.HasPrefix(value, prefix) {
		t.Fatalf("%s must be explainable and start with %q: %q", label, prefix, value)
	}
	assertGenericToolContractsProviderFreeString(t, label, value)
}

func genericToolContractsMappingContains(mappings []struct {
	Source string `json:"source"`
	Target string `json:"target"`
}, source, target string) bool {
	for _, mapping := range mappings {
		if mapping.Source == source && mapping.Target == target {
			return true
		}
	}
	return false
}

func genericToolContractsSchemaRequired(t *testing.T, schema map[string]any) []string {
	t.Helper()
	raw, ok := schema["required"].([]any)
	if !ok {
		t.Fatalf("schema required must be a JSON array: %#v", schema["required"])
	}
	required := make([]string, 0, len(raw))
	for _, value := range raw {
		item, ok := value.(string)
		if !ok {
			t.Fatalf("schema required item must be a string: %#v", value)
		}
		required = append(required, item)
	}
	return required
}

func assertGenericToolContractsProviderFreeString(t *testing.T, label, value string) {
	t.Helper()
	lower := strings.ToLower(value)
	for _, forbidden := range []string{
		"openai",
		"anthropic",
		"gemini",
		"vertex",
		"bedrock",
		"azure",
		"api_key",
		"apikey",
		"secret",
		"password",
		"bearer",
		"credential",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("%s must stay provider-free/offline/no-secret; found %q in %q", label, forbidden, value)
		}
	}
}

func decodeGenericToolContractsJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func containsGenericToolContractsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
