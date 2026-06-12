package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericOptionalAdapterBoundaryLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericOptionalAdapterBoundaryPackageDir(t)
	var manifest struct {
		SchemaVersion      int               `json:"schema_version"`
		ID                 string            `json:"id"`
		PackageName        string            `json:"package_name"`
		PackageBoundaryID  string            `json:"package_boundary_id"`
		CapabilityID       string            `json:"capability_id"`
		ProviderFree       bool              `json:"provider_free"`
		DomainSpecific     bool              `json:"domain_specific"`
		LiveNetworkDefault bool              `json:"live_network_default"`
		LiveModelDefault   bool              `json:"live_model_default"`
		DependsOnQRuntime  bool              `json:"depends_on_q_runtime"`
		CredentialRequired bool              `json:"credential_required_default"`
		Capabilities       []string          `json:"capabilities"`
		Contracts          map[string]string `json:"contracts"`
		Schemas            map[string]string `json:"schemas"`
		Fixtures           map[string]string `json:"fixtures"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	if manifest.SchemaVersion != 1 || manifest.ID != "generic-optional-adapter-boundary" ||
		manifest.PackageName != "leia-generic-ai-optional-adapter-boundary" ||
		manifest.PackageBoundaryID != "generic-ai-optional-adapter-boundary" ||
		manifest.CapabilityID != "generic.ai.optional_adapter.boundary" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetworkDefault ||
		manifest.LiveModelDefault || manifest.DependsOnQRuntime || manifest.CredentialRequired {
		t.Fatalf("manifest must stay provider-free/generic/offline/credential-free: %#v", manifest)
	}
	for _, want := range []string{"generic.ai.optional_adapter.boundary", "generic.ai.optional_adapter.registry", "generic.ai.optional_dependency.gate", "generic.ai.optional_adapter.result_envelope", "generic.ai.optional_adapter.version_metadata", "generic.ai.optional_adapter.terms_metadata", "generic.ai.optional_adapter.credential_redaction", "generic.ai.optional_adapter.clean_skip", "generic.ai.optional_adapter.no_live_import_default"} {
		if !genericLivePackageContains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}

	var contract struct {
		SchemaVersion         int               `json:"schema_version"`
		PackageBoundaryID     string            `json:"package_boundary_id"`
		PackageName           string            `json:"package_name"`
		Entrypoint            string            `json:"entrypoint"`
		ProviderFree          bool              `json:"provider_free"`
		DomainSpecific        bool              `json:"domain_specific"`
		LiveNetwork           bool              `json:"live_network"`
		LiveModelCalls        bool              `json:"live_model_calls"`
		RealDependencyImports bool              `json:"real_dependency_imports"`
		RequiresCredentials   bool              `json:"requires_credentials"`
		FieldContracts        map[string]string `json:"field_contracts"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Contracts["contract"]), &contract)
	if contract.SchemaVersion != 1 || contract.PackageBoundaryID != manifest.PackageBoundaryID ||
		contract.PackageName != "generic.ai.optional_adapter.boundary" || contract.Entrypoint != "ai.optional_adapter.boundary" ||
		!contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork ||
		contract.LiveModelCalls || contract.RealDependencyImports || contract.RequiresCredentials {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	for _, want := range []string{"adapter_registry", "dependency_gate", "result_envelope", "version_metadata", "terms_metadata", "credential_redaction", "no_live_import_default", "clean_skip"} {
		if contract.FieldContracts[want] == "" {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
		}
	}
}

func TestGenericOptionalAdapterBoundaryLivePackageFixtureShape(t *testing.T) {
	base := genericOptionalAdapterBoundaryPackageDir(t)
	fixture := loadGenericOptionalAdapterBoundaryFixture(t, filepath.Join(base, "fixtures", "optional_adapter_boundary_fixture.json"))
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.LiveModelCalls || !fixture.NoLiveImportDefault {
		t.Fatalf("fixture must stay provider-free, offline, and no-live-import by default: %#v", fixture)
	}
	if len(fixture.AdapterRegistry) != 3 || len(fixture.DependencyGates) != 3 ||
		len(fixture.ResultEnvelopes) != 3 || len(fixture.VersionMetadata) != 3 ||
		len(fixture.TermsMetadata) != 3 || len(fixture.CredentialRedaction) != 2 ||
		len(fixture.AdapterBoundaries) != 3 {
		t.Fatalf("fixture counts drifted: registry=%d gates=%d results=%d versions=%d terms=%d redactions=%d adapters=%d",
			len(fixture.AdapterRegistry), len(fixture.DependencyGates), len(fixture.ResultEnvelopes), len(fixture.VersionMetadata), len(fixture.TermsMetadata), len(fixture.CredentialRedaction), len(fixture.AdapterBoundaries))
	}
	registry := map[string]genericOptionalAdapterRegistryEntry{}
	for _, entry := range fixture.AdapterRegistry {
		if entry.AdapterID == "" || entry.Capability == "" || entry.PackageName == "" ||
			entry.ImportName == "" || entry.FixtureKey == "" || entry.OutputSchema == "" ||
			!entry.NoLiveImportDefault {
			t.Fatalf("registry entry incomplete: %#v", entry)
		}
		registry[entry.AdapterID] = entry
	}
	for _, gate := range fixture.DependencyGates {
		if registry[gate.AdapterID].AdapterID == "" || gate.DependencyImported ||
			gate.CredentialPresent || gate.LiveNetwork ||
			gate.StatusWithoutDependency != "skipped" || !gate.CleanSkip {
			t.Fatalf("dependency gate invalid or unresolved: %#v", gate)
		}
	}
	for _, result := range fixture.ResultEnvelopes {
		if result.Status != "skipped" || result.FixtureKey == "" || result.Data != nil ||
			result.Metadata.AdapterID == "" || !result.Metadata.ReplayReady ||
			registry[result.Metadata.AdapterID].FixtureKey != result.FixtureKey {
			t.Fatalf("result envelope invalid or unresolved: %#v", result)
		}
	}
	for _, version := range fixture.VersionMetadata {
		if registry[version.AdapterID].AdapterID == "" || version.VersionSource != "fixture_metadata" ||
			version.DependencyAbsentStatus != "skipped" || version.DetectedVersion != nil {
			t.Fatalf("version metadata must stay fixture-sourced and absent-safe: %#v", version)
		}
	}
	for _, terms := range fixture.TermsMetadata {
		if registry[terms.AdapterID].AdapterID == "" || terms.Mode != "fixture_replay" ||
			!terms.MetadataOnly || !terms.LiveTermsRequiredBeforeNetworkUse || terms.LiveNetworkEnabled {
			t.Fatalf("terms metadata invalid: %#v", terms)
		}
	}
	for _, redaction := range fixture.CredentialRedaction {
		if registry[redaction.AdapterID].AdapterID == "" || redaction.Status != "redacted" ||
			redaction.SecretValuesPresent || len(redaction.RedactedFields) == 0 {
			t.Fatalf("credential redaction invalid: %#v", redaction)
		}
	}
	for _, boundary := range fixture.AdapterBoundaries {
		if boundary.DependencyImported || boundary.CredentialRequired || boundary.LiveNetwork || !boundary.CleanSkip {
			t.Fatalf("adapter boundary must clean-skip: %#v", boundary)
		}
	}
}

func TestGenericOptionalAdapterBoundaryLivePackageIsDomainNeutral(t *testing.T) {
	base := genericOptionalAdapterBoundaryPackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"finrobot", "acme", "aapl", "ticker", "equity", "investment", "valuation_engine", "target_price", "dcf", "sec.gov", "10-k", "finance.", "fingpt", "finrl", "openbb", "mplfinance", "backtrader", "ollama"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("%s leaks domain-specific marker %q", path, forbidden)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGenericOptionalAdapterBoundaryLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericOptionalAdapterBoundaryPackageDir(t)
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "optional_adapter_registry_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "adapter_registry"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "optional_adapter_registry_v1.schema.json"), []string{"properties", "adapter_registry", "items"}, []string{"adapter_id", "capability", "package_name", "import_name", "fixture_key", "output_schema", "no_live_import_default"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "optional_adapter_gate_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "dependency_gates"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "optional_adapter_gate_v1.schema.json"), []string{"properties", "dependency_gates", "items"}, []string{"adapter_id", "dependency_imported", "credential_required", "credential_present", "live_network", "status_without_dependency", "clean_skip"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "optional_adapter_result_envelope_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "live_model_calls", "no_live_import_default", "adapter_registry", "dependency_gates", "result_envelopes", "version_metadata", "terms_metadata", "credential_redaction", "adapter_boundaries"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "optional_adapter_result_envelope_v1.schema.json"), []string{"properties", "result_envelopes", "items"}, []string{"status", "capability", "fixture_key", "data", "metadata"})
}

func TestGenericOptionalAdapterBoundaryLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericOptionalAdapterBoundaryPackageDir(t), "main.leia")
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
			got, err := vm.Get("generic_optional_adapter_boundary_live_package_summary")
			if err != nil {
				t.Fatalf("Get summary: %v", err)
			}
			want := "generic_optional_adapter_boundary_live_package capability=generic.ai.optional_adapter.boundary entrypoint=ai.optional_adapter.boundary adapters=3 gates=3 results=3 versions=3 terms=3 redactions=2 clean_skip=3 provider_free=true live_network=false imports=false model_calls=false"
			if got != want {
				t.Fatalf("summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

type genericOptionalAdapterBoundaryFixture struct {
	ProviderFree          bool                                  `json:"provider_free"`
	LiveNetwork           bool                                  `json:"live_network"`
	RealDependencyImports bool                                  `json:"real_dependency_imports"`
	LiveModelCalls        bool                                  `json:"live_model_calls"`
	NoLiveImportDefault   bool                                  `json:"no_live_import_default"`
	AdapterRegistry       []genericOptionalAdapterRegistryEntry `json:"adapter_registry"`
	DependencyGates       []struct {
		AdapterID               string `json:"adapter_id"`
		DependencyImported      bool   `json:"dependency_imported"`
		CredentialRequired      bool   `json:"credential_required"`
		CredentialPresent       bool   `json:"credential_present"`
		LiveNetwork             bool   `json:"live_network"`
		StatusWithoutDependency string `json:"status_without_dependency"`
		CleanSkip               bool   `json:"clean_skip"`
	} `json:"dependency_gates"`
	ResultEnvelopes []struct {
		Status     string `json:"status"`
		Capability string `json:"capability"`
		FixtureKey string `json:"fixture_key"`
		Data       any    `json:"data"`
		Metadata   struct {
			AdapterID   string `json:"adapter_id"`
			SkipReason  string `json:"skip_reason"`
			ReplayReady bool   `json:"replay_ready"`
		} `json:"metadata"`
	} `json:"result_envelopes"`
	VersionMetadata []struct {
		AdapterID              string `json:"adapter_id"`
		VersionSource          string `json:"version_source"`
		DetectedVersion        any    `json:"detected_version"`
		DependencyAbsentStatus string `json:"dependency_absent_status"`
	} `json:"version_metadata"`
	TermsMetadata []struct {
		AdapterID                         string `json:"adapter_id"`
		Mode                              string `json:"mode"`
		MetadataOnly                      bool   `json:"metadata_only"`
		LiveTermsRequiredBeforeNetworkUse bool   `json:"live_terms_required_before_network_use"`
		LiveNetworkEnabled                bool   `json:"live_network_enabled"`
	} `json:"terms_metadata"`
	CredentialRedaction []struct {
		AdapterID           string   `json:"adapter_id"`
		Status              string   `json:"status"`
		SecretValuesPresent bool     `json:"secret_values_present"`
		RedactedFields      []string `json:"redacted_fields"`
	} `json:"credential_redaction"`
	AdapterBoundaries []struct {
		ID                 string `json:"id"`
		Capability         string `json:"capability"`
		DependencyImported bool   `json:"dependency_imported"`
		CredentialRequired bool   `json:"credential_required"`
		LiveNetwork        bool   `json:"live_network"`
		CleanSkip          bool   `json:"clean_skip"`
	} `json:"adapter_boundaries"`
}

type genericOptionalAdapterRegistryEntry struct {
	AdapterID           string `json:"adapter_id"`
	Capability          string `json:"capability"`
	PackageName         string `json:"package_name"`
	ImportName          string `json:"import_name"`
	FixtureKey          string `json:"fixture_key"`
	OutputSchema        string `json:"output_schema"`
	NoLiveImportDefault bool   `json:"no_live_import_default"`
}

func loadGenericOptionalAdapterBoundaryFixture(t *testing.T, path string) genericOptionalAdapterBoundaryFixture {
	t.Helper()
	var fixture genericOptionalAdapterBoundaryFixture
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func genericOptionalAdapterBoundaryPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_optional_adapter_boundary")
}
