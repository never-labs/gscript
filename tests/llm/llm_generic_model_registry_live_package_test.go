package leia_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type genericModelRegistryManifest struct {
	SchemaVersion         int    `json:"schema_version"`
	ID                    string `json:"id"`
	PackageName           string `json:"package_name"`
	Package               string `json:"package"`
	Version               string `json:"version"`
	DialectCapabilityID   string `json:"dialect_capability_id"`
	ProviderFree          bool   `json:"provider_free"`
	DomainSpecific        bool   `json:"domain_specific"`
	LiveNetwork           bool   `json:"live_network"`
	RealDependencyImports bool   `json:"real_dependency_imports"`
	LiveModelCalls        bool   `json:"live_model_calls"`
	FixtureRoot           string `json:"fixture_root"`
	SchemaRoot            string `json:"schema_root"`
	ContractRoot          string `json:"contract_root"`
	SourceIndexEntry      struct {
		Index                    string `json:"index"`
		DialectCapabilityID      string `json:"dialect_capability_id"`
		MissingBoundaryPackageID string `json:"missing_boundary_package_id"`
		ResolvedBoundary         string `json:"resolved_boundary"`
	} `json:"source_index_entry"`
	Entrypoints   map[string]string `json:"entrypoints"`
	Schemas       map[string]string `json:"schemas"`
	Capabilities  []string          `json:"capabilities"`
	DefaultPolicy struct {
		Mode                        string `json:"mode"`
		ProviderPolicy              string `json:"provider_policy"`
		LiveNetwork                 bool   `json:"live_network"`
		ProviderCredentialsRequired bool   `json:"provider_credentials_required"`
		RealDependencyImports       bool   `json:"real_dependency_imports"`
		LiveModelCalls              bool   `json:"live_model_calls"`
		CleanSkipWithoutDescriptor  bool   `json:"clean_skip_without_descriptor"`
		RedactSecretValues          bool   `json:"redact_secret_values"`
		ReplayMissBehavior          string `json:"replay_miss_behavior"`
	} `json:"default_policy"`
	ProviderPolicy struct {
		ID                          string   `json:"id"`
		AllowNamedProviders         bool     `json:"allow_named_providers"`
		AllowLiveProviderGate       bool     `json:"allow_live_provider_gate"`
		ProviderCredentialsRequired bool     `json:"provider_credentials_required"`
		LiveNetwork                 bool     `json:"live_network"`
		LiveModelCalls              bool     `json:"live_model_calls"`
		DescriptorProvider          string   `json:"descriptor_provider"`
		FallbackProvider            string   `json:"fallback_provider"`
		DenyReasons                 []string `json:"deny_reasons"`
	} `json:"provider_policy"`
	RoutingGuard struct {
		ID                      string   `json:"id"`
		ProviderFree            bool     `json:"provider_free"`
		LiveNetwork             bool     `json:"live_network"`
		SecretValuesPresent     bool     `json:"secret_values_present"`
		RequestProviderOverride string   `json:"request_provider_override"`
		ReplayDescriptorSource  string   `json:"replay_descriptor_source"`
		LiveProviderBehavior    string   `json:"live_provider_behavior"`
		FallbackProvider        string   `json:"fallback_provider"`
		DecisionReplayStable    bool     `json:"decision_replay_stable"`
		DeniedProviderKinds     []string `json:"denied_provider_kinds"`
		RedirectEvidence        []struct {
			RequestedAlias string `json:"requested_alias"`
			DeniedProvider string `json:"denied_provider"`
			RedirectAlias  string `json:"redirect_alias"`
			DescriptorRef  string `json:"descriptor_ref"`
			Provider       string `json:"provider"`
			Reason         string `json:"reason"`
		} `json:"redirect_evidence"`
	} `json:"routing_guard"`
	LiveProviderGate struct {
		ID                             string   `json:"id"`
		Capability                     string   `json:"capability"`
		EnabledByDefault               bool     `json:"enabled_by_default"`
		ProviderFreeDefault            bool     `json:"provider_free_default"`
		LiveNetworkDefault             bool     `json:"live_network_default"`
		LiveModelCallsDefault          bool     `json:"live_model_calls_default"`
		RequiresExplicitIntegrationEnv bool     `json:"requires_explicit_integration_env"`
		IntegrationEnv                 string   `json:"integration_env"`
		AllowedProviderEnvRefs         []string `json:"allowed_provider_env_refs"`
		SecretValuesPresent            bool     `json:"secret_values_present"`
		RedactionPolicyRef             string   `json:"redaction_policy_ref"`
		CleanSkipWithoutCredentials    bool     `json:"clean_skip_without_credentials"`
		DefaultSkipReason              string   `json:"default_skip_reason"`
		ProviderProtocols              []string `json:"provider_protocols"`
	} `json:"live_provider_gate"`
	RedactionPolicy struct {
		ID                  string   `json:"id"`
		Enabled             bool     `json:"enabled"`
		Replacement         string   `json:"replacement"`
		SecretValuePolicy   string   `json:"secret_value_policy"`
		SecretValuesPresent bool     `json:"secret_values_present"`
		RedactFields        []string `json:"redact_fields"`
		EnvRefPatterns      []string `json:"env_ref_patterns"`
	} `json:"redaction_policy"`
	CapabilityFlags map[string]bool `json:"capability_flags"`
	AliasRegistry   []struct {
		Alias         string `json:"alias"`
		Target        string `json:"target"`
		Kind          string `json:"kind"`
		DescriptorRef string `json:"descriptor_ref"`
	} `json:"alias_registry"`
	ExecutionDescriptors []struct {
		DescriptorRef               string   `json:"descriptor_ref"`
		ModelAlias                  string   `json:"model_alias"`
		Provider                    string   `json:"provider"`
		ProviderModel               string   `json:"provider_model"`
		Mode                        string   `json:"mode"`
		FixtureKey                  string   `json:"fixture_key"`
		ReplaySafe                  bool     `json:"replay_safe"`
		LiveNetwork                 bool     `json:"live_network"`
		ProviderCredentialsRequired bool     `json:"provider_credentials_required"`
		SecretValuesPresent         bool     `json:"secret_values_present"`
		Temperature                 int      `json:"temperature"`
		CapabilityFlags             []string `json:"capability_flags"`
		RedactionPolicyRef          string   `json:"redaction_policy_ref"`
	} `json:"execution_descriptors"`
	TestGates          []string `json:"test_gates"`
	NoBuiltInGuarantee struct {
		Required  bool   `json:"required"`
		Statement string `json:"statement"`
	} `json:"no_built_in_guarantee"`
}

func TestGenericModelRegistryLivePackageManifest(t *testing.T) {
	base := genericModelRegistryPackageDir(t)
	manifest := loadGenericModelRegistryManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "generic-ai-model-registry" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-generic-ai-model-registry" {
		t.Fatalf("package_name = %q", manifest.PackageName)
	}
	if manifest.DialectCapabilityID != "generic.ai.model.registry" || manifest.DomainSpecific {
		t.Fatalf("capability boundary is not generic model registry: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.LiveNetwork || manifest.RealDependencyImports || manifest.LiveModelCalls {
		t.Fatalf("provider-free defaults broken: provider_free=%v live_network=%v imports=%v live_model_calls=%v", manifest.ProviderFree, manifest.LiveNetwork, manifest.RealDependencyImports, manifest.LiveModelCalls)
	}
	if manifest.SourceIndexEntry.Index == "" ||
		manifest.SourceIndexEntry.DialectCapabilityID != "generic.ai.model.registry" ||
		manifest.SourceIndexEntry.MissingBoundaryPackageID != "generic-ai-model-registry" ||
		!strings.Contains(manifest.SourceIndexEntry.ResolvedBoundary, "provider-free") {
		t.Fatalf("source index mapping incomplete: %#v", manifest.SourceIndexEntry)
	}
	for _, want := range []string{
		"generic.ai.model.registry",
		"generic.ai.model.alias.resolve",
		"generic.ai.model.descriptor.replay",
		"generic.ai.model.redaction.policy",
		"generic.ai.model.routing.guard",
		"generic.ai.model.live_provider_gate",
	} {
		if !genericModelRegistryContains(manifest.Capabilities, want) {
			t.Fatalf("capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}
	if !manifest.NoBuiltInGuarantee.Required ||
		!strings.Contains(manifest.NoBuiltInGuarantee.Statement, manifest.PackageName) {
		t.Fatalf("no_built_in_guarantee inconsistent: %#v", manifest.NoBuiltInGuarantee)
	}
	for _, rel := range []string{
		manifest.Entrypoints["main"],
		manifest.Entrypoints["contract"],
		manifest.Entrypoints["fixture_index"],
		manifest.Entrypoints["alias_registry"],
		manifest.Entrypoints["live_provider_gate"],
		manifest.Schemas["alias_registry"],
		manifest.Schemas["execution_descriptor"],
		manifest.Schemas["redaction_policy"],
		manifest.Schemas["live_provider_gate"],
	} {
		if rel == "" {
			t.Fatalf("manifest contains empty referenced artifact")
		}
		if _, err := os.Stat(filepath.Join(base, rel)); err != nil {
			t.Fatalf("referenced artifact %q: %v", rel, err)
		}
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.ProviderPolicy != "provider-free" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		manifest.DefaultPolicy.LiveModelCalls ||
		!manifest.DefaultPolicy.CleanSkipWithoutDescriptor ||
		!manifest.DefaultPolicy.RedactSecretValues ||
		manifest.DefaultPolicy.ReplayMissBehavior != "clean-skip" {
		t.Fatalf("default policy must stay replay-only and credential-free: %#v", manifest.DefaultPolicy)
	}
}

func TestGenericModelRegistryProviderPolicyRedactionAndCapabilities(t *testing.T) {
	base := genericModelRegistryPackageDir(t)
	manifest := loadGenericModelRegistryManifest(t, base)

	if manifest.ProviderPolicy.ID != "provider-free-model-policy-v1" ||
		manifest.ProviderPolicy.AllowNamedProviders ||
		!manifest.ProviderPolicy.AllowLiveProviderGate ||
		manifest.ProviderPolicy.ProviderCredentialsRequired ||
		manifest.ProviderPolicy.LiveNetwork ||
		manifest.ProviderPolicy.LiveModelCalls ||
		manifest.ProviderPolicy.DescriptorProvider != "fixture-replay" ||
		manifest.ProviderPolicy.FallbackProvider != "fixture-replay" {
		t.Fatalf("provider policy must deny live providers: %#v", manifest.ProviderPolicy)
	}
	for _, want := range []string{"live_provider_disabled", "credential_material_forbidden", "network_disabled", "live_provider_requires_explicit_integration_env"} {
		if !genericModelRegistryContains(manifest.ProviderPolicy.DenyReasons, want) {
			t.Fatalf("provider policy missing deny reason %q: %#v", want, manifest.ProviderPolicy.DenyReasons)
		}
	}
	if manifest.RoutingGuard.ID != "provider-free-routing-guard-v1" ||
		!manifest.RoutingGuard.ProviderFree ||
		manifest.RoutingGuard.LiveNetwork ||
		manifest.RoutingGuard.SecretValuesPresent ||
		manifest.RoutingGuard.RequestProviderOverride != "ignored-when-replay-descriptor-present" ||
		manifest.RoutingGuard.ReplayDescriptorSource != "model_alias_registry" ||
		manifest.RoutingGuard.LiveProviderBehavior != "deny-or-redirect-to-fixture-replay" ||
		manifest.RoutingGuard.FallbackProvider != manifest.ProviderPolicy.FallbackProvider ||
		!manifest.RoutingGuard.DecisionReplayStable {
		t.Fatalf("routing guard must be provider-free replay redirect policy: %#v", manifest.RoutingGuard)
	}
	for _, want := range []string{"future-live-provider", "request-scoped-live-provider", "provider-sdk-import", "implicit-live-provider"} {
		if !genericModelRegistryContains(manifest.RoutingGuard.DeniedProviderKinds, want) {
			t.Fatalf("routing guard missing denied provider kind %q: %#v", want, manifest.RoutingGuard.DeniedProviderKinds)
		}
	}
	if len(manifest.RoutingGuard.RedirectEvidence) == 0 {
		t.Fatalf("routing guard missing redirect evidence")
	}
	for _, evidence := range manifest.RoutingGuard.RedirectEvidence {
		if evidence.RequestedAlias == "" ||
			evidence.DeniedProvider == "" ||
			evidence.RedirectAlias == "" ||
			evidence.DescriptorRef == "" ||
			evidence.Provider != "fixture-replay" ||
			evidence.Reason != "replay" {
			t.Fatalf("routing guard redirect evidence incomplete: %#v", evidence)
		}
	}
	if !manifest.RedactionPolicy.Enabled ||
		manifest.RedactionPolicy.Replacement != "<redacted>" ||
		manifest.RedactionPolicy.SecretValuePolicy != "never-store-secret-values" ||
		manifest.RedactionPolicy.SecretValuesPresent ||
		len(manifest.RedactionPolicy.RedactFields) < 5 ||
		len(manifest.RedactionPolicy.EnvRefPatterns) < 4 {
		t.Fatalf("redaction policy incomplete: %#v", manifest.RedactionPolicy)
	}
	var fixtureRedaction struct {
		SchemaVersion       int      `json:"schema_version"`
		ID                  string   `json:"id"`
		ProviderFree        bool     `json:"provider_free"`
		Enabled             bool     `json:"enabled"`
		Replacement         string   `json:"replacement"`
		SecretValuePolicy   string   `json:"secret_value_policy"`
		SecretValuesPresent bool     `json:"secret_values_present"`
		RedactFields        []string `json:"redact_fields"`
		EnvRefPatterns      []string `json:"env_ref_patterns"`
	}
	readJSONFile(t, filepath.Join(base, "fixtures", "redaction_policy_fixture.json"), &fixtureRedaction)
	if fixtureRedaction.SchemaVersion != 1 ||
		fixtureRedaction.ID != manifest.RedactionPolicy.ID ||
		!fixtureRedaction.ProviderFree ||
		!fixtureRedaction.Enabled ||
		fixtureRedaction.Replacement != manifest.RedactionPolicy.Replacement ||
		fixtureRedaction.SecretValuePolicy != manifest.RedactionPolicy.SecretValuePolicy ||
		fixtureRedaction.SecretValuesPresent {
		t.Fatalf("redaction fixture must mirror provider-free no-secret policy: %#v", fixtureRedaction)
	}
	for _, field := range manifest.RedactionPolicy.RedactFields {
		if !genericModelRegistryContains(fixtureRedaction.RedactFields, field) {
			t.Fatalf("redaction fixture missing manifest redact field %q: %#v", field, fixtureRedaction.RedactFields)
		}
	}
	for _, pattern := range fixtureRedaction.EnvRefPatterns {
		if !strings.HasPrefix(pattern, "env:") {
			t.Fatalf("redaction fixture env pattern must be an env ref, got %q", pattern)
		}
	}
	for _, want := range []string{
		"model_alias_registry",
		"provider_policy",
		"replay_safe_execution_descriptor",
		"redaction_policy",
		"structured_output",
		"tool_calling",
	} {
		if !manifest.CapabilityFlags[want] {
			t.Fatalf("capability flag %q missing or false: %#v", want, manifest.CapabilityFlags)
		}
	}
	for _, wantFalse := range []string{"live_network", "provider_credentials", "streaming", "vision"} {
		if manifest.CapabilityFlags[wantFalse] {
			t.Fatalf("capability flag %q must remain false: %#v", wantFalse, manifest.CapabilityFlags)
		}
	}
	if !manifest.CapabilityFlags["live_provider_gate"] {
		t.Fatalf("capability flag live_provider_gate missing or false: %#v", manifest.CapabilityFlags)
	}
	assertGenericModelRegistryLiveProviderGate(t, base, manifest)
}

func TestGenericModelRegistryAliasResolutionToReplayDescriptors(t *testing.T) {
	manifest := loadGenericModelRegistryManifest(t, genericModelRegistryPackageDir(t))
	aliases := map[string]struct {
		target        string
		kind          string
		descriptorRef string
	}{}
	for _, entry := range manifest.AliasRegistry {
		if entry.Alias == "" || entry.Kind == "" {
			t.Fatalf("alias entry incomplete: %#v", entry)
		}
		aliases[entry.Alias] = struct {
			target        string
			kind          string
			descriptorRef string
		}{target: entry.Target, kind: entry.Kind, descriptorRef: entry.DescriptorRef}
	}
	if got := resolveGenericModelAlias(t, aliases, "default"); got != "model_registry:descriptor:fixture_analyst:v1" {
		t.Fatalf("default resolves to %q", got)
	}
	if got := resolveGenericModelAlias(t, aliases, "reviewer"); got != "model_registry:descriptor:fixture_reviewer:v1" {
		t.Fatalf("reviewer resolves to %q", got)
	}

	descriptors := map[string]bool{}
	for _, descriptor := range manifest.ExecutionDescriptors {
		descriptors[descriptor.DescriptorRef] = true
		if descriptor.Provider != "fixture-replay" ||
			descriptor.Mode != "deterministic_fixture_replay" ||
			descriptor.FixtureKey == "" ||
			!descriptor.ReplaySafe ||
			descriptor.LiveNetwork ||
			descriptor.ProviderCredentialsRequired ||
			descriptor.SecretValuesPresent ||
			descriptor.Temperature != 0 ||
			descriptor.RedactionPolicyRef != manifest.RedactionPolicy.ID {
			t.Fatalf("descriptor must be replay-safe and provider-free: %#v", descriptor)
		}
		for _, want := range []string{"model_alias_registry", "provider_policy", "replay_safe_execution_descriptor", "redaction_policy"} {
			if !genericModelRegistryContains(descriptor.CapabilityFlags, want) {
				t.Fatalf("%s missing descriptor capability %q: %#v", descriptor.DescriptorRef, want, descriptor.CapabilityFlags)
			}
		}
	}
	for alias := range aliases {
		ref := resolveGenericModelAlias(t, aliases, alias)
		if !descriptors[ref] {
			t.Fatalf("alias %q resolves to unknown descriptor %q", alias, ref)
		}
	}
}

func TestGenericModelRegistryContractAndFixtures(t *testing.T) {
	base := genericModelRegistryPackageDir(t)
	var contract struct {
		SchemaVersion         int      `json:"schema_version"`
		ID                    string   `json:"id"`
		CapabilityID          string   `json:"capability_id"`
		ProviderFree          bool     `json:"provider_free"`
		DomainSpecific        bool     `json:"domain_specific"`
		LiveNetwork           bool     `json:"live_network"`
		RealDependencyImports bool     `json:"real_dependency_imports"`
		LiveModelCalls        bool     `json:"live_model_calls"`
		Boundary              string   `json:"boundary"`
		Inputs                []string `json:"inputs"`
		Outputs               []string `json:"outputs"`
		ProviderPolicy        struct {
			AllowNamedProviders   bool   `json:"allow_named_providers"`
			AllowLiveProviderGate bool   `json:"allow_live_provider_gate"`
			DescriptorProvider    string `json:"descriptor_provider"`
			CredentialMaterial    string `json:"credential_material"`
			Network               string `json:"network"`
			LiveProviderDefault   string `json:"live_provider_default"`
		} `json:"provider_policy"`
		LiveProviderGate struct {
			Required                       bool   `json:"required"`
			EnabledByDefault               bool   `json:"enabled_by_default"`
			RequiresExplicitIntegrationEnv bool   `json:"requires_explicit_integration_env"`
			IntegrationEnv                 string `json:"integration_env"`
			CredentialValuesAllowed        bool   `json:"credential_values_allowed"`
			CredentialEnvRefsAllowed       bool   `json:"credential_env_refs_allowed"`
			CleanSkipWithoutCredentials    bool   `json:"clean_skip_without_credentials"`
			RedactionPolicyRequired        bool   `json:"redaction_policy_required"`
		} `json:"live_provider_gate"`
		RoutingGuard struct {
			RequestProviderOverride string `json:"request_provider_override"`
			ReplayDescriptorSource  string `json:"replay_descriptor_source"`
			LiveProviderBehavior    string `json:"live_provider_behavior"`
			DecisionReplayStable    bool   `json:"decision_replay_stable"`
		} `json:"routing_guard"`
		ResolutionRules []struct {
			Rule string `json:"rule"`
		} `json:"resolution_rules"`
	}
	readJSONFile(t, filepath.Join(base, "contracts", "model_registry_contract.json"), &contract)
	if contract.CapabilityID != "generic.ai.model.registry" ||
		!contract.ProviderFree ||
		contract.DomainSpecific ||
		contract.LiveNetwork ||
		contract.RealDependencyImports ||
		contract.LiveModelCalls ||
		!strings.Contains(contract.Boundary, "model alias registry") {
		t.Fatalf("contract boundary is not provider-free generic model registry: %#v", contract)
	}
	for _, want := range []string{"model_alias", "provider_policy", "replay_mode"} {
		if !genericModelRegistryContains(contract.Inputs, want) {
			t.Fatalf("contract input missing %q: %#v", want, contract.Inputs)
		}
	}
	for _, want := range []string{"model_descriptor", "capability_flags", "redaction_policy"} {
		if !genericModelRegistryContains(contract.Outputs, want) {
			t.Fatalf("contract output missing %q: %#v", want, contract.Outputs)
		}
	}
	if contract.ProviderPolicy.AllowNamedProviders ||
		!contract.ProviderPolicy.AllowLiveProviderGate ||
		contract.ProviderPolicy.DescriptorProvider != "fixture-replay" ||
		contract.ProviderPolicy.CredentialMaterial != "forbidden" ||
		contract.ProviderPolicy.Network != "disabled" ||
		contract.ProviderPolicy.LiveProviderDefault != "clean-skip" {
		t.Fatalf("contract provider policy must deny live provider edges: %#v", contract.ProviderPolicy)
	}
	if !contract.LiveProviderGate.Required ||
		contract.LiveProviderGate.EnabledByDefault ||
		!contract.LiveProviderGate.RequiresExplicitIntegrationEnv ||
		contract.LiveProviderGate.IntegrationEnv != "LEIA_LLM_INTEGRATION" ||
		contract.LiveProviderGate.CredentialValuesAllowed ||
		!contract.LiveProviderGate.CredentialEnvRefsAllowed ||
		!contract.LiveProviderGate.CleanSkipWithoutCredentials ||
		!contract.LiveProviderGate.RedactionPolicyRequired {
		t.Fatalf("contract live provider gate must be explicit and clean-skip safe: %#v", contract.LiveProviderGate)
	}
	if contract.RoutingGuard.RequestProviderOverride != "ignored-when-replay-descriptor-present" ||
		contract.RoutingGuard.ReplayDescriptorSource != "model_alias_registry" ||
		contract.RoutingGuard.LiveProviderBehavior != "deny-or-redirect-to-fixture-replay" ||
		!contract.RoutingGuard.DecisionReplayStable {
		t.Fatalf("contract routing guard must cleanly redirect live provider candidates: %#v", contract.RoutingGuard)
	}

	for _, rel := range []string{
		"fixtures/provider_free_fixture_index.json",
		"fixtures/model_alias_registry_fixture.json",
		"fixtures/replay_execution_descriptor_fixture.json",
		"fixtures/redaction_policy_fixture.json",
		"fixtures/live_provider_gate_fixture.json",
		"schemas/model_alias_registry_v1.schema.json",
		"schemas/execution_descriptor_v1.schema.json",
		"schemas/redaction_policy_v1.schema.json",
		"schemas/live_provider_gate_v1.schema.json",
	} {
		assertGenericModelRegistryJSONFile(t, filepath.Join(base, rel))
	}
	assertGenericModelRegistryFixtureIndexNoSecretGuard(t, base)
}

func TestGenericModelRegistryNestedSchemaRequiredFields(t *testing.T) {
	base := genericModelRegistryPackageDir(t)

	assertGenericModelRegistrySchemaRequired(t, filepath.Join(base, "schemas", "model_alias_registry_v1.schema.json"), []string{
		"schema_version",
		"id",
		"capability_id",
		"provider_free",
		"live_network",
		"aliases",
	})
	assertGenericModelRegistryNestedSchemaRequired(t, filepath.Join(base, "schemas", "model_alias_registry_v1.schema.json"), []string{"properties", "aliases", "items"}, []string{
		"alias",
		"kind",
	})
	assertGenericModelRegistryNestedSchemaRequired(t, filepath.Join(base, "schemas", "model_alias_registry_v1.schema.json"), []string{"properties", "resolution_examples", "items"}, []string{
		"input_alias",
		"path",
		"descriptor_ref",
	})

	assertGenericModelRegistrySchemaRequired(t, filepath.Join(base, "schemas", "execution_descriptor_v1.schema.json"), []string{
		"schema_version",
		"id",
		"provider_free",
		"live_network",
		"descriptors",
	})
	assertGenericModelRegistryNestedSchemaRequired(t, filepath.Join(base, "schemas", "execution_descriptor_v1.schema.json"), []string{"properties", "descriptors", "items"}, []string{
		"descriptor_ref",
		"model_alias",
		"provider",
		"provider_model",
		"mode",
		"fixture_key",
		"replay_safe",
		"live_network",
		"provider_credentials_required",
		"secret_values_present",
		"temperature",
		"capability_flags",
		"redaction_policy_ref",
	})

	assertGenericModelRegistrySchemaRequired(t, filepath.Join(base, "schemas", "live_provider_gate_v1.schema.json"), []string{
		"schema_version",
		"id",
		"capability",
		"provider_free",
		"live_network",
		"live_model_calls",
		"enabled_by_default",
		"requires_explicit_integration_env",
		"integration_env",
		"allowed_provider_env_refs",
		"secret_values_present",
		"credential_values_allowed",
		"redaction_policy_ref",
		"clean_skip_without_credentials",
		"default_skip_reason",
		"provider_protocols",
		"live_smoke_contract",
	})
	assertGenericModelRegistryNestedSchemaRequired(t, filepath.Join(base, "schemas", "live_provider_gate_v1.schema.json"), []string{"properties", "live_smoke_contract"}, []string{
		"prompt",
		"expected_text",
		"max_tokens",
		"temperature",
		"timeout_seconds",
		"record_replay_required_for_ci",
	})
}

func TestGenericModelRegistryFixtureIndexManifestAlignment(t *testing.T) {
	base := genericModelRegistryPackageDir(t)
	manifest := loadGenericModelRegistryManifest(t, base)
	type fixtureIndexEntry struct {
		Key                 string         `json:"key"`
		Capability          string         `json:"capability"`
		Path                string         `json:"path"`
		Schema              string         `json:"schema"`
		ProviderFree        bool           `json:"provider_free"`
		LiveNetwork         bool           `json:"live_network"`
		SecretValuesPresent bool           `json:"secret_values_present"`
		Metadata            map[string]any `json:"metadata"`
	}
	var index struct {
		ProviderFree        bool                `json:"provider_free"`
		LiveNetwork         bool                `json:"live_network"`
		SecretValuesPresent bool                `json:"secret_values_present"`
		Fixtures            []fixtureIndexEntry `json:"fixtures"`
	}
	readJSONFile(t, filepath.Join(base, manifest.Entrypoints["fixture_index"]), &index)
	if !index.ProviderFree || index.LiveNetwork || index.SecretValuesPresent || len(index.Fixtures) != 4 {
		t.Fatalf("fixture index header/count mismatch: %#v", index)
	}
	want := map[string]struct {
		capability string
		path       string
		schema     string
	}{
		"model_registry:aliases:v1": {
			capability: "generic.ai.model.alias.resolve",
			path:       manifest.Entrypoints["alias_registry"],
			schema:     manifest.Schemas["alias_registry"],
		},
		"model_registry:descriptor:fixture_analyst:v1": {
			capability: "generic.ai.model.descriptor.replay",
			path:       "fixtures/replay_execution_descriptor_fixture.json",
			schema:     manifest.Schemas["execution_descriptor"],
		},
		"model_registry:redaction:v1": {
			capability: "generic.ai.model.redaction.policy",
			path:       "fixtures/redaction_policy_fixture.json",
			schema:     manifest.Schemas["redaction_policy"],
		},
		"model_registry:live_provider_gate:v1": {
			capability: "generic.ai.model.live_provider_gate",
			path:       manifest.Entrypoints["live_provider_gate"],
			schema:     manifest.Schemas["live_provider_gate"],
		},
	}
	seen := map[string]bool{}
	for _, fixture := range index.Fixtures {
		wantEntry, ok := want[fixture.Key]
		if !ok {
			t.Fatalf("unexpected fixture index key %q", fixture.Key)
		}
		if seen[fixture.Key] {
			t.Fatalf("duplicate fixture index key %q", fixture.Key)
		}
		seen[fixture.Key] = true
		if fixture.Capability != wantEntry.capability ||
			!genericModelRegistryContains(manifest.Capabilities, fixture.Capability) ||
			fixture.Path != wantEntry.path ||
			fixture.Schema != wantEntry.schema ||
			!fixture.ProviderFree ||
			fixture.LiveNetwork ||
			fixture.SecretValuesPresent {
			t.Fatalf("fixture index entry is not aligned with manifest: entry=%#v want=%#v capabilities=%#v", fixture, wantEntry, manifest.Capabilities)
		}
		assertGenericModelRegistryJSONFile(t, filepath.Join(base, fixture.Path))
		assertGenericModelRegistryJSONFile(t, filepath.Join(base, fixture.Schema))
	}
	for key := range want {
		if !seen[key] {
			t.Fatalf("fixture index missing key %q", key)
		}
	}
}

func TestGenericModelRegistryMainSmokeOutput(t *testing.T) {
	base := genericModelRegistryPackageDir(t)
	manifest := loadGenericModelRegistryManifest(t, base)
	want := "generic_model_registry_live_package capability=generic.ai.model.registry capabilities=6 schemas=4 fixtures=4 provider_free=true live_network=false imports=false live_provider_gate=true routing_guard=true"

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
			if err := vm.ExecFile(filepath.Join(base, manifest.Entrypoints["main"])); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}
			got, err := vm.Get("generic_model_registry_live_package_summary")
			if err != nil {
				t.Fatalf("Get generic_model_registry_live_package_summary: %v", err)
			}
			if got != want {
				t.Fatalf("summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
			summary := genericModelRegistrySummaryFields(t, got.(string))
			if summary["capability"] != manifest.DialectCapabilityID ||
				summary["capabilities"] != fmt.Sprint(len(manifest.Capabilities)) ||
				summary["schemas"] != fmt.Sprint(len(manifest.Schemas)) ||
				summary["fixtures"] != "4" ||
				(summary["provider_free"] == "true") != manifest.ProviderFree ||
				(summary["live_network"] == "true") != manifest.LiveNetwork ||
				(summary["imports"] == "true") != manifest.RealDependencyImports ||
				(summary["live_provider_gate"] == "true") != manifest.CapabilityFlags["live_provider_gate"] ||
				(summary["routing_guard"] == "true") != genericModelRegistryContains(manifest.Capabilities, "generic.ai.model.routing.guard") {
				t.Fatalf("summary does not align with manifest: summary=%#v manifest=%#v", summary, manifest)
			}
		})
	}
}

func assertGenericModelRegistryFixtureIndexNoSecretGuard(t *testing.T, base string) {
	t.Helper()
	var index struct {
		ProviderFree        bool `json:"provider_free"`
		LiveNetwork         bool `json:"live_network"`
		SecretValuesPresent bool `json:"secret_values_present"`
		Fixtures            []struct {
			Key                 string         `json:"key"`
			Capability          string         `json:"capability"`
			ProviderFree        bool           `json:"provider_free"`
			LiveNetwork         bool           `json:"live_network"`
			SecretValuesPresent bool           `json:"secret_values_present"`
			Metadata            map[string]any `json:"metadata"`
		} `json:"fixtures"`
	}
	readJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if !index.ProviderFree || index.LiveNetwork || index.SecretValuesPresent {
		t.Fatalf("fixture index must stay provider-free, offline, and no-secret: %#v", index)
	}
	for _, fixture := range index.Fixtures {
		if fixture.Key == "" || fixture.Capability == "" || !fixture.ProviderFree || fixture.LiveNetwork || fixture.SecretValuesPresent {
			t.Fatalf("fixture index entry must stay provider-free, offline, and no-secret: %#v", fixture)
		}
		if fixture.Metadata["secret_values_present"] != false {
			t.Fatalf("%s secret_values_present metadata = %#v, want false", fixture.Key, fixture.Metadata["secret_values_present"])
		}
	}
}

func assertGenericModelRegistryLiveProviderGate(t *testing.T, base string, manifest genericModelRegistryManifest) {
	t.Helper()
	gate := manifest.LiveProviderGate
	if gate.ID != "explicit-live-provider-gate-v1" ||
		gate.Capability != "generic.ai.model.live_provider_gate" ||
		gate.EnabledByDefault ||
		!gate.ProviderFreeDefault ||
		gate.LiveNetworkDefault ||
		gate.LiveModelCallsDefault ||
		!gate.RequiresExplicitIntegrationEnv ||
		gate.IntegrationEnv != "LEIA_LLM_INTEGRATION" ||
		gate.SecretValuesPresent ||
		gate.RedactionPolicyRef != manifest.RedactionPolicy.ID ||
		!gate.CleanSkipWithoutCredentials ||
		!strings.Contains(strings.ToLower(gate.DefaultSkipReason), "disabled") {
		t.Fatalf("live provider gate must be explicit, provider-free by default, and clean-skip safe: %#v", gate)
	}
	for _, want := range []string{"env:LEIA_GLM_API_KEY", "env:LEIA_GLM_MODEL", "env:ANTHROPIC_AUTH_TOKEN"} {
		if !genericModelRegistryContains(gate.AllowedProviderEnvRefs, want) {
			t.Fatalf("live provider gate missing env ref %q: %#v", want, gate.AllowedProviderEnvRefs)
		}
	}
	for _, envRef := range gate.AllowedProviderEnvRefs {
		if !strings.HasPrefix(envRef, "env:") || strings.Contains(envRef, "=") {
			t.Fatalf("live provider gate must store env refs only, not values: %q", envRef)
		}
	}
	if !genericModelRegistryContains(gate.ProviderProtocols, "anthropic_compatible") ||
		!genericModelRegistryContains(gate.ProviderProtocols, "openai_compatible") {
		t.Fatalf("live provider gate protocols incomplete: %#v", gate.ProviderProtocols)
	}

	var fixture struct {
		SchemaVersion                  int      `json:"schema_version"`
		ID                             string   `json:"id"`
		Capability                     string   `json:"capability"`
		ProviderFree                   bool     `json:"provider_free"`
		LiveNetwork                    bool     `json:"live_network"`
		LiveModelCalls                 bool     `json:"live_model_calls"`
		EnabledByDefault               bool     `json:"enabled_by_default"`
		RequiresExplicitIntegrationEnv bool     `json:"requires_explicit_integration_env"`
		IntegrationEnv                 string   `json:"integration_env"`
		AllowedProviderEnvRefs         []string `json:"allowed_provider_env_refs"`
		SecretValuesPresent            bool     `json:"secret_values_present"`
		CredentialValuesAllowed        bool     `json:"credential_values_allowed"`
		RedactionPolicyRef             string   `json:"redaction_policy_ref"`
		CleanSkipWithoutCredentials    bool     `json:"clean_skip_without_credentials"`
		LiveSmokeContract              struct {
			Prompt                    string `json:"prompt"`
			ExpectedText              string `json:"expected_text"`
			RecordReplayRequiredForCI bool   `json:"record_replay_required_for_ci"`
		} `json:"live_smoke_contract"`
	}
	readJSONFile(t, filepath.Join(base, "fixtures", "live_provider_gate_fixture.json"), &fixture)
	if fixture.SchemaVersion != 1 ||
		fixture.ID != "generic-model-registry-live-provider-gate-v1" ||
		fixture.Capability != gate.Capability ||
		!fixture.ProviderFree ||
		fixture.LiveNetwork ||
		fixture.LiveModelCalls ||
		fixture.EnabledByDefault ||
		!fixture.RequiresExplicitIntegrationEnv ||
		fixture.IntegrationEnv != gate.IntegrationEnv ||
		fixture.SecretValuesPresent ||
		fixture.CredentialValuesAllowed ||
		fixture.RedactionPolicyRef != gate.RedactionPolicyRef ||
		!fixture.CleanSkipWithoutCredentials ||
		fixture.LiveSmokeContract.Prompt == "" ||
		fixture.LiveSmokeContract.ExpectedText == "" ||
		!fixture.LiveSmokeContract.RecordReplayRequiredForCI {
		t.Fatalf("live provider gate fixture mismatch: %#v", fixture)
	}
}

func genericModelRegistryPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(genericModelRegistryRepoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_model_registry")
}

func loadGenericModelRegistryManifest(t *testing.T, base string) genericModelRegistryManifest {
	t.Helper()
	var manifest genericModelRegistryManifest
	readJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func readJSONFile(t *testing.T, path string, into any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func assertGenericModelRegistryJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	readJSONFile(t, path, &value)
}

func assertGenericModelRegistrySchemaRequired(t *testing.T, path string, want []string) {
	t.Helper()
	assertGenericModelRegistryNestedSchemaRequired(t, path, nil, want)
}

func assertGenericModelRegistryNestedSchemaRequired(t *testing.T, path string, objectPath []string, want []string) {
	t.Helper()
	var schema map[string]any
	readJSONFile(t, path, &schema)
	current := any(schema)
	for _, segment := range objectPath {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("%s path %v reached non-object at %q: %#v", path, objectPath, segment, current)
		}
		current, ok = object[segment]
		if !ok {
			t.Fatalf("%s missing schema path segment %q in %v", path, segment, objectPath)
		}
	}
	object, ok := current.(map[string]any)
	if !ok {
		t.Fatalf("%s path %v is not an object: %#v", path, objectPath, current)
	}
	required, ok := object["required"].([]any)
	if !ok {
		t.Fatalf("%s path %v missing required array: %#v", path, objectPath, object["required"])
	}
	requiredSet := map[string]bool{}
	for _, value := range required {
		name, ok := value.(string)
		if !ok {
			t.Fatalf("%s path %v has non-string required value %#v", path, objectPath, value)
		}
		requiredSet[name] = true
	}
	for _, field := range want {
		if !requiredSet[field] {
			t.Fatalf("%s path %v required missing %q: %#v", path, objectPath, field, required)
		}
	}
}

func genericModelRegistrySummaryFields(t *testing.T, value string) map[string]string {
	t.Helper()
	fields := strings.Fields(value)
	if len(fields) == 0 || fields[0] != "generic_model_registry_live_package" {
		t.Fatalf("unexpected summary prefix: %q", value)
	}
	result := map[string]string{}
	for _, field := range fields[1:] {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			t.Fatalf("malformed summary field %q in %q", field, value)
		}
		result[parts[0]] = parts[1]
	}
	return result
}

func genericModelRegistryRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func genericModelRegistryContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func resolveGenericModelAlias(t *testing.T, aliases map[string]struct {
	target        string
	kind          string
	descriptorRef string
}, alias string) string {
	t.Helper()
	seen := map[string]bool{}
	current := alias
	for {
		if seen[current] {
			t.Fatalf("alias cycle at %q", current)
		}
		seen[current] = true
		entry, ok := aliases[current]
		if !ok {
			t.Fatalf("unknown model alias %q", current)
		}
		switch entry.kind {
		case "alias":
			if entry.target == "" {
				t.Fatalf("alias %q has empty target", current)
			}
			current = entry.target
		case "descriptor":
			if entry.descriptorRef == "" {
				t.Fatalf("descriptor alias %q has empty descriptor_ref", current)
			}
			return entry.descriptorRef
		default:
			t.Fatalf("alias %q has unknown kind %q", current, entry.kind)
		}
	}
}
