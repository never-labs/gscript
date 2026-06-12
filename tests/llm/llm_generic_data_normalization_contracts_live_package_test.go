package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericDataNormalizationContractsLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericDataNormalizationContractsPackageDir(t)
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
	if manifest.SchemaVersion != 1 || manifest.ID != "generic-data-normalization-contracts" ||
		manifest.PackageName != "leia-generic-ai-data-normalization-contracts" ||
		manifest.PackageBoundaryID != "generic-ai-data-normalization-contracts" ||
		manifest.CapabilityID != "generic.ai.data_normalization.contracts" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetworkDefault ||
		manifest.LiveModelDefault || manifest.DependsOnQRuntime || manifest.CredentialRequired {
		t.Fatalf("manifest must stay provider-free/generic/offline/credential-free: %#v", manifest)
	}
	for _, want := range []string{"generic.ai.data_normalization.contracts", "generic.ai.data_normalization.schema_mapping", "generic.ai.data_normalization.field_policy", "generic.ai.data_normalization.missing_policy", "generic.ai.data_normalization.stale_policy", "generic.ai.data_normalization.type_coercion", "generic.ai.data_normalization.unit_transform", "generic.ai.data_normalization.provenance", "generic.ai.data_normalization.validation_error", "generic.ai.data_normalization.adapter_clean_skip"} {
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
	if contract.ID != "generic-data-normalization-contracts-contract" ||
		!contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	for _, want := range []string{"mapping", "normalized_rows", "validation", "clean_skip"} {
		field := contract.FieldContracts[want]
		if field.Schema == "" || field.Fixture == "" || len(field.Required) == 0 {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
		}
	}
}

func TestGenericDataNormalizationContractsLivePackageFixtureShape(t *testing.T) {
	base := genericDataNormalizationContractsPackageDir(t)
	index := loadGenericDataNormalizationContractsFixtureIndex(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"))
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
	for _, want := range []string{"normalization_mapping:observations:offline:v1", "normalized_rows:observations:offline:v1", "normalization_validation:observations:offline:v1", "normalization_clean_skip:offline:v1"} {
		if !seen[want] {
			t.Fatalf("fixture key %q missing from %#v", want, seen)
		}
	}
}

func TestGenericDataNormalizationContractsLivePackageIsDomainNeutral(t *testing.T) {
	base := genericDataNormalizationContractsPackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"finrobot", "finance", "financial", "equity", "valuation", "ebitda", "sec.gov", "10-k", "10-q", "filing", "ticker", "stock", "market", "price", "portfolio", "factor", "backtest", "dividend", "yfinance", "finnhub", "openbb"} {
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

func TestGenericDataNormalizationContractsLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericDataNormalizationContractsPackageDir(t)
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "normalization_mapping_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "mapping_id", "dataset_id", "canonical_fields", "field_policies", "deterministic_order"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "normalized_rows_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "dataset_id", "mapping_id", "rows", "provenance"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "normalization_validation_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "dataset_id", "mapping_id", "validation_errors", "summary"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "normalization_clean_skip_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "skip_code", "dependency", "adapter", "reason", "recoverable"})
}

func TestGenericDataNormalizationContractsLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericDataNormalizationContractsPackageDir(t), "main.leia")
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
			got, err := vm.Get("generic_data_normalization_contracts_live_package_summary")
			if err != nil {
				t.Fatalf("Get summary: %v", err)
			}
			want := "generic_data_normalization_contracts_live_package capability=generic.ai.data_normalization.contracts entrypoint=ai.data_normalization.contracts mappings=2 field_policies=3 missing=2 stale=2 coercions=3 unit_transforms=2 rows=4 provenance=4 validation_errors=2 clean_skip=2 provider_free=true live_network=false imports=false model_calls=false"
			if got != want {
				t.Fatalf("summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

type genericDataNormalizationContractsFixtureIndex struct {
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

func loadGenericDataNormalizationContractsFixtureIndex(t *testing.T, path string) genericDataNormalizationContractsFixtureIndex {
	t.Helper()
	var fixture genericDataNormalizationContractsFixtureIndex
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func genericDataNormalizationContractsPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_data_normalization_contracts")
}
