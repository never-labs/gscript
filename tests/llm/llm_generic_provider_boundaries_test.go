package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericModelRegistryIsProviderFreeAndPluggable(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibLLM)}, tc.opts...)...)
			if err := vm.Exec(`
config := llm.config
models, models_err := config.aliases({
    default: "primary",
    primary: "offline_fixture_gateway",
    offline_fixture_gateway: {
        provider: "fixture-router-42",
        provider_model: "mock-generic-analyst",
    },
    review_gateway: {
        provider: "bring-your-own-gateway",
        model: "mock-generic-reviewer",
    },
})

primary, primary_err := config.route(models, {model: "primary"})
review, review_err := config.route(models, {model: "review_gateway"})
replayed, replayed_err := config.route(models, {
    model: "review_gateway",
    replay: primary.replay,
})

registry_provider_free := models_err == nil && primary_err == nil && review_err == nil && replayed_err == nil
primary_provider := primary.provider
primary_model := primary.model
primary_provider_model := primary.provider_model
review_provider := review.provider
review_model := review.model
replayed_provider := replayed.provider
replayed_model := replayed.model
replayed_reason := replayed.trace.reason
replayed_flag := replayed.trace.replayed
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			for name, want := range map[string]any{
				"registry_provider_free": true,
				"primary_provider":       "fixture-router-42",
				"primary_model":          "mock-generic-analyst",
				"primary_provider_model": "mock-generic-analyst",
				"review_provider":        "bring-your-own-gateway",
				"review_model":           "mock-generic-reviewer",
				"replayed_provider":      "fixture-router-42",
				"replayed_model":         "mock-generic-analyst",
				"replayed_reason":        "replay",
				"replayed_flag":          true,
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
		})
	}
}

func TestVendorAdapterMetadataKeepsProviderSpecificsBehindGenericCapabilities(t *testing.T) {
	root := repoRoot(t)
	pkgDir := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "vendor_adapters")
	manifest := loadVendorLivePackageManifest(t, pkgDir)

	if !manifest.ProviderFree || manifest.LiveNetwork || manifest.RealDependencyImports {
		t.Fatalf("vendor adapter manifest must stay provider-free: %#v", manifest)
	}

	providersByCommonCapability := map[string]map[string]bool{}
	credentialAbsentSafe := 0
	for _, adapter := range manifest.Adapters {
		if adapter.Credential.Required || !adapter.Credential.Redacted {
			t.Fatalf("%s credential gate is not absence-safe: %#v", adapter.ID, adapter.Credential)
		}
		if adapter.Credential.SecretRef != nil {
			if !strings.HasPrefix(*adapter.Credential.SecretRef, "env:") {
				t.Fatalf("%s secret ref must be an env gate, got %q", adapter.ID, *adapter.Credential.SecretRef)
			}
			credentialAbsentSafe++
		}
		if !strings.HasPrefix(adapter.Capability, "finance.vendor."+adapter.Provider+".") {
			t.Fatalf("%s provider-specific capability must remain adapter metadata: %q", adapter.ID, adapter.Capability)
		}
		for _, capability := range adapter.CommonCapabilities {
			if capability == "" {
				t.Fatalf("%s has empty common capability", adapter.ID)
			}
			if strings.HasPrefix(capability, "finance.vendor.") || capability == adapter.Capability {
				t.Fatalf("%s common capability %q should not be the provider-specific adapter capability", adapter.ID, capability)
			}
			if providersByCommonCapability[capability] == nil {
				providersByCommonCapability[capability] = map[string]bool{}
			}
			providersByCommonCapability[capability][adapter.Provider] = true
		}
	}
	if credentialAbsentSafe == 0 {
		t.Fatalf("vendor adapters should exercise optional env credential gates")
	}
	for _, capability := range []string{"company_profile", "exchange_metadata", "company_news"} {
		if got := len(providersByCommonCapability[capability]); got < 2 {
			t.Fatalf("common capability %q has %d providers, want pluggable coverage across providers", capability, got)
		}
	}
	assertNoDirectProviderSDKCallsOrLiveDefaults(t, filepath.Join(pkgDir, "main.leia"), []string{"yfinance", "finnhub", "fmp", "reddit", "openbb", "ollama"})
}

func TestOptionalIntegrationBoundariesCleanSkipWithoutDefaultProviderExecution(t *testing.T) {
	base := optionalIntegrationsLivePackageDir(t)
	var contract struct {
		ProviderFree               bool `json:"provider_free"`
		LiveNetwork                bool `json:"live_network"`
		RealDependencyImports      bool `json:"real_dependency_imports"`
		CleanSkipWithoutDependency bool `json:"clean_skip_without_dependency"`
		Gates                      []struct {
			ID                      string                    `json:"id"`
			Capability              string                    `json:"capability"`
			CleanSkip               bool                      `json:"clean_skip"`
			RequiresCredentials     bool                      `json:"requires_credentials"`
			ProviderCredentials     bool                      `json:"provider_credentials_required"`
			CredentialAbsentSafe    bool                      `json:"credential_absent_safe"`
			LiveNetwork             bool                      `json:"live_network"`
			DependencyImported      bool                      `json:"dependency_imported"`
			NoLiveImportDefault     bool                      `json:"no_live_import_default"`
			StatusWithoutDependency string                    `json:"status_without_dependency"`
			AbsenceGates            map[string]map[string]any `json:"absence_gates"`
		} `json:"gates"`
	}
	decodeOptionalLiveJSONFile(t, filepath.Join(base, "contracts", "optional_integration_capability_gates.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || !contract.CleanSkipWithoutDependency {
		t.Fatalf("optional integration contract must stay provider-free: %#v", contract)
	}

	for _, gate := range contract.Gates {
		if !strings.HasPrefix(gate.Capability, "optional.") {
			t.Fatalf("%s capability should be package metadata, got %q", gate.ID, gate.Capability)
		}
		if !gate.CleanSkip || !gate.CredentialAbsentSafe ||
			gate.LiveNetwork || gate.DependencyImported || !gate.NoLiveImportDefault || gate.StatusWithoutDependency != "skipped" {
			t.Fatalf("%s absence gate should cleanly skip without live execution/imports: %#v", gate.ID, gate)
		}
		if len(gate.AbsenceGates) == 0 {
			t.Fatalf("%s has no absence gates", gate.ID)
		}
		for name, absence := range gate.AbsenceGates {
			if absence["clean_skip"] != true || absence["absent_status"] != "skipped" {
				t.Fatalf("%s absence gate %q = %#v, want clean skipped", gate.ID, name, absence)
			}
		}
	}
	assertNoDirectProviderSDKCallsOrLiveDefaults(t, filepath.Join(base, "main.leia"), []string{"fingpt", "finrl", "finml", "backtrader", "mplfinance", "openbb", "ollama"})
}

func TestProviderBoundaryFixturesDoNotEnableLiveProviderExecutionByDefault(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"examples/ai/finrobot_translation/live_packages/vendor_adapters/package.manifest.json",
		"examples/ai/finrobot_translation/live_packages/optional_integrations/package.manifest.json",
		"examples/ai/finrobot_translation/live_packages/optional_integrations/contracts/optional_integration_capability_gates.json",
		"examples/ai/finrobot_translation/live_packages/optional_integrations/fixtures/provider_free_fixture_index.json",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		var decoded any
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		assertNoLiveProviderExecutionDefaults(t, rel, decoded)
	}
}

func assertNoDirectProviderSDKCallsOrLiveDefaults(t *testing.T, path string, providers []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, provider := range providers {
		pattern := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(provider) + `\s*[.(]`)
		if pattern.FindString(source) != "" {
			t.Fatalf("%s contains provider SDK call matching %q", path, pattern.String())
		}
	}
	for _, forbidden := range []string{"built_in_provider: true", "built_in_api: true", "live_network: true", "real_dependency_imports: true"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("%s contains forbidden provider boundary marker %q", path, forbidden)
		}
	}
}

func assertNoLiveProviderExecutionDefaults(t *testing.T, path string, value any) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			switch key {
			case "live_network", "real_dependency_imports", "real_dependency_import_default", "dependency_imported", "built_in_provider", "built_in_api":
				if child == true {
					t.Fatalf("%s declares %s=true in provider-free boundary", path, key)
				}
			}
			assertNoLiveProviderExecutionDefaults(t, path, child)
		}
	case []any:
		for _, child := range value {
			assertNoLiveProviderExecutionDefaults(t, path, child)
		}
	}
}
