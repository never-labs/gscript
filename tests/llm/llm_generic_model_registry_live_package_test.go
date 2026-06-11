package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type genericModelRegistryManifest struct {
	SchemaVersion         int    `json:"schema_version"`
	ID                    string `json:"id"`
	Package               string `json:"package"`
	Version               string `json:"version"`
	CapabilityID          string `json:"capability_id"`
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
		CapabilityID             string `json:"capability_id"`
		MissingBoundaryPackageID string `json:"missing_boundary_package_id"`
		ResolvedBoundary         string `json:"resolved_boundary"`
	} `json:"source_index_entry"`
	Entrypoints   map[string]string `json:"entrypoints"`
	Schemas       map[string]string `json:"schemas"`
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
		ProviderCredentialsRequired bool     `json:"provider_credentials_required"`
		LiveNetwork                 bool     `json:"live_network"`
		LiveModelCalls              bool     `json:"live_model_calls"`
		DescriptorProvider          string   `json:"descriptor_provider"`
		FallbackProvider            string   `json:"fallback_provider"`
		DenyReasons                 []string `json:"deny_reasons"`
	} `json:"provider_policy"`
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
	TestGates []string `json:"test_gates"`
}

func TestGenericModelRegistryLivePackageManifest(t *testing.T) {
	base := genericModelRegistryPackageDir(t)
	manifest := loadGenericModelRegistryManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "generic-ai-model-registry" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.CapabilityID != "generic.ai.model.registry" || manifest.DomainSpecific {
		t.Fatalf("capability boundary is not generic model registry: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.LiveNetwork || manifest.RealDependencyImports || manifest.LiveModelCalls {
		t.Fatalf("provider-free defaults broken: provider_free=%v live_network=%v imports=%v live_model_calls=%v", manifest.ProviderFree, manifest.LiveNetwork, manifest.RealDependencyImports, manifest.LiveModelCalls)
	}
	if manifest.SourceIndexEntry.Index == "" ||
		manifest.SourceIndexEntry.CapabilityID != "generic.ai.model.registry" ||
		manifest.SourceIndexEntry.MissingBoundaryPackageID != "generic-ai-model-registry" ||
		!strings.Contains(manifest.SourceIndexEntry.ResolvedBoundary, "provider-free") {
		t.Fatalf("source index mapping incomplete: %#v", manifest.SourceIndexEntry)
	}
	for _, rel := range []string{
		manifest.Entrypoints["main"],
		manifest.Entrypoints["contract"],
		manifest.Entrypoints["fixture_index"],
		manifest.Entrypoints["alias_registry"],
		manifest.Schemas["alias_registry"],
		manifest.Schemas["execution_descriptor"],
		manifest.Schemas["redaction_policy"],
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
	manifest := loadGenericModelRegistryManifest(t, genericModelRegistryPackageDir(t))

	if manifest.ProviderPolicy.ID != "provider-free-model-policy-v1" ||
		manifest.ProviderPolicy.AllowNamedProviders ||
		manifest.ProviderPolicy.ProviderCredentialsRequired ||
		manifest.ProviderPolicy.LiveNetwork ||
		manifest.ProviderPolicy.LiveModelCalls ||
		manifest.ProviderPolicy.DescriptorProvider != "fixture-replay" ||
		manifest.ProviderPolicy.FallbackProvider != "fixture-replay" {
		t.Fatalf("provider policy must deny live providers: %#v", manifest.ProviderPolicy)
	}
	for _, want := range []string{"live_provider_disabled", "credential_material_forbidden", "network_disabled"} {
		if !contains(manifest.ProviderPolicy.DenyReasons, want) {
			t.Fatalf("provider policy missing deny reason %q: %#v", want, manifest.ProviderPolicy.DenyReasons)
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
			if !contains(descriptor.CapabilityFlags, want) {
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
			AllowNamedProviders bool   `json:"allow_named_providers"`
			DescriptorProvider  string `json:"descriptor_provider"`
			CredentialMaterial  string `json:"credential_material"`
			Network             string `json:"network"`
		} `json:"provider_policy"`
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
		if !contains(contract.Inputs, want) {
			t.Fatalf("contract input missing %q: %#v", want, contract.Inputs)
		}
	}
	for _, want := range []string{"model_descriptor", "capability_flags", "redaction_policy"} {
		if !contains(contract.Outputs, want) {
			t.Fatalf("contract output missing %q: %#v", want, contract.Outputs)
		}
	}
	if contract.ProviderPolicy.AllowNamedProviders ||
		contract.ProviderPolicy.DescriptorProvider != "fixture-replay" ||
		contract.ProviderPolicy.CredentialMaterial != "forbidden" ||
		contract.ProviderPolicy.Network != "disabled" {
		t.Fatalf("contract provider policy must deny live provider edges: %#v", contract.ProviderPolicy)
	}

	for _, rel := range []string{
		"fixtures/provider_free_fixture_index.json",
		"fixtures/model_alias_registry_fixture.json",
		"fixtures/replay_execution_descriptor_fixture.json",
		"fixtures/redaction_policy_fixture.json",
		"schemas/model_alias_registry_v1.schema.json",
		"schemas/execution_descriptor_v1.schema.json",
		"schemas/redaction_policy_v1.schema.json",
	} {
		assertJSONFile(t, filepath.Join(base, rel))
	}
}

func genericModelRegistryPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_model_registry")
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
