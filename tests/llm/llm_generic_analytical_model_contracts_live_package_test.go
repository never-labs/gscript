package leia_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericAnalyticalModelContractsLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericAnalyticalModelContractsPackageDir(t)
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
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	if manifest.SchemaVersion != 1 || manifest.ID != "generic-analytical-model-contracts" ||
		manifest.PackageName != "leia-generic-ai-analytical-model-contracts" ||
		manifest.PackageBoundaryID != "generic-ai-analytical-model-contracts" ||
		manifest.CapabilityID != "generic.ai.analytical_model.contracts" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetworkDefault ||
		manifest.LiveModelDefault || manifest.DependsOnQRuntime || manifest.CredentialRequired {
		t.Fatalf("manifest must stay provider-free/generic/offline/credential-free: %#v", manifest)
	}
	for _, want := range []string{"generic.ai.analytical_model.contracts", "generic.ai.analytical_model.scenario.book", "generic.ai.analytical_model.assumption.audit", "generic.ai.analytical_model.sensitivity.grid", "generic.ai.analytical_model.method.output", "generic.ai.analytical_model.provenance.attach", "generic.ai.analytical_model.tolerance_gate.evaluate", "generic.ai.analytical_model.clean_skip"} {
		if !genericLivePackageContains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}

	var contract struct {
		ID                    string `json:"id"`
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		FieldContracts        map[string]struct {
			Schema   string   `json:"schema"`
			Fixture  string   `json:"fixture"`
			Required []string `json:"required_fields"`
		} `json:"field_contracts"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Contracts["contract"]), &contract)
	if contract.ID != "generic-analytical-model-contracts-contract" ||
		!contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	for _, want := range []string{"scenario_book", "forecast_result", "tolerance_gate", "clean_skip"} {
		field := contract.FieldContracts[want]
		if field.Schema == "" || field.Fixture == "" || len(field.Required) == 0 {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
		}
	}
}

func TestGenericAnalyticalModelContractsLivePackageFixtureShape(t *testing.T) {
	base := genericAnalyticalModelContractsPackageDir(t)
	index := loadGenericAnalyticalModelContractsFixtureIndex(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"))
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 4 {
		t.Fatalf("fixture index invalid: %#v", index)
	}
	seen := map[string]bool{}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Path == "" || fixture.Schema == "" || !fixture.Metadata.ReplayReady ||
			!fixture.Metadata.ProviderFree || fixture.Metadata.LiveNetwork || fixture.Metadata.RealDependencyImports {
			t.Fatalf("fixture entry invalid: %#v", fixture)
		}
		if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(fixture.Path))); err != nil {
			t.Fatalf("fixture path %q: %v", fixture.Path, err)
		}
		if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(fixture.Schema))); err != nil {
			t.Fatalf("fixture schema %q: %v", fixture.Schema, err)
		}
		seen[fixture.FixtureKey] = true
	}
	for _, want := range []string{"scenario_book:subject_alpha:offline:v1", "forecast_result:subject_alpha:offline:v1", "tolerance_gates:subject_alpha:offline:v1", "forecast_clean_skip:offline:v1"} {
		if !seen[want] {
			t.Fatalf("fixture key %q missing from %#v", want, seen)
		}
	}
}

func TestGenericAnalyticalModelContractsLivePackageIsDomainNeutral(t *testing.T) {
	base := genericAnalyticalModelContractsPackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"finrobot", "finance", "financial", "valuation", "dcf", "ebitda", "target_price", "football_field", "ticker", "stock", "equity", "shares", "wacc", "terminal_growth", "analyst", "peer_multiple", "sec.gov", "10-k", "10-q", "filing", "acme", "portfolio", "backtest"} {
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

func TestGenericAnalyticalModelContractsLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericAnalyticalModelContractsPackageDir(t)
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "scenario_book_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "scenario_book_id", "assumption_sets", "scenarios", "sensitivity_axes", "deterministic_order"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "forecast_result_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "scenario_book_id", "forecast_rows", "provenance", "audit_events"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "tolerance_gate_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "scenario_book_id", "tolerance_gates", "summary"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "forecast_clean_skip_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "skip_code", "dependency", "adapter", "reason", "recoverable"})
}

func TestGenericAnalyticalModelContractsLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericAnalyticalModelContractsPackageDir(t), "main.leia")
	want := "generic_analytical_model_contracts_live_package capability=generic.ai.analytical_model.contracts entrypoint=ai.analytical_model.contracts assumption_sets=1 scenarios=3 forecast_rows=6 sensitivity_axes=2 tolerance_gates=3 audit_events=4 clean_skip=2 provider_free=true live_network=false imports=false model_calls=false"
	for _, result := range runFinRobotLivePackageSummarySmoke(t, path, "generic_analytical_model_contracts_live_package_summary", "generic_analytical_model_contracts_live_package", leia.LibString) {
		if result.Summary != want {
			t.Fatalf("summary = %#v, want %#v", result.Summary, want)
		}
	}
}

type genericAnalyticalModelContractsFixtureIndex struct {
	ProviderFree          bool `json:"provider_free"`
	LiveNetwork           bool `json:"live_network"`
	RealDependencyImports bool `json:"real_dependency_imports"`
	Fixtures              []struct {
		FixtureKey string `json:"fixture_key"`
		Path       string `json:"path"`
		Schema     string `json:"schema"`
		Metadata   struct {
			ReplayReady           bool `json:"replay_ready"`
			ProviderFree          bool `json:"provider_free"`
			LiveNetwork           bool `json:"live_network"`
			RealDependencyImports bool `json:"real_dependency_imports"`
		} `json:"metadata"`
	} `json:"fixtures"`
}

func loadGenericAnalyticalModelContractsFixtureIndex(t *testing.T, path string) genericAnalyticalModelContractsFixtureIndex {
	t.Helper()
	var fixture genericAnalyticalModelContractsFixtureIndex
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func genericAnalyticalModelContractsPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_analytical_model_contracts")
}
