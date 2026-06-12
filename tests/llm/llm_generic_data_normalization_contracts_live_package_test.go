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
		Schemas            map[string]string `json:"schemas"`
		Fixtures           map[string]string `json:"fixtures"`
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
	for _, want := range []string{"generic.ai.data_normalization.contracts", "generic.ai.data_normalization.schema_mapping", "generic.ai.data_normalization.provider_response_projection", "generic.ai.data_normalization.field_policy", "generic.ai.data_normalization.missing_policy", "generic.ai.data_normalization.stale_policy", "generic.ai.data_normalization.type_coercion", "generic.ai.data_normalization.unit_transform", "generic.ai.data_normalization.provenance", "generic.ai.data_normalization.validation_error", "generic.ai.data_normalization.adapter_clean_skip"} {
		if !genericLivePackageContains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}
	if manifest.Schemas["provider_response_projection"] == "" || manifest.Fixtures["provider_response_projection"] == "" {
		t.Fatalf("manifest missing provider response projection schema/fixture: schemas=%#v fixtures=%#v", manifest.Schemas, manifest.Fixtures)
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
	for _, want := range []string{"mapping", "provider_response_projection", "normalized_rows", "validation", "clean_skip"} {
		field := contract.FieldContracts[want]
		if field.Schema == "" || field.Fixture == "" || len(field.Required) == 0 {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
		}
	}
}

func TestGenericDataNormalizationContractsLivePackageFixtureShape(t *testing.T) {
	base := genericDataNormalizationContractsPackageDir(t)
	index := loadGenericDataNormalizationContractsFixtureIndex(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"))
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 5 {
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
	for _, want := range []string{"normalization_mapping:observations:offline:v1", "provider_response_projection:observations:offline:v1", "normalized_rows:observations:offline:v1", "normalization_validation:observations:offline:v1", "normalization_clean_skip:offline:v1"} {
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
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "provider_response_projection_v1.schema.json"), []string{"schema_version", "fixture_key", "projection_kind", "provider_free", "live_network", "real_dependency_imports", "source_refs", "dataset_id", "mapping_id", "response_mappings", "provenance_mappings", "clean_skip_mappings", "projection_assertions"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "normalized_rows_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "dataset_id", "mapping_id", "rows", "provenance"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "normalization_validation_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "dataset_id", "mapping_id", "validation_errors", "summary"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "normalization_clean_skip_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "skip_code", "dependency", "adapter", "reason", "recoverable"})
}

func TestGenericDataNormalizationContractsProviderResponseProjection(t *testing.T) {
	root := repoRoot(t)
	base := genericDataNormalizationContractsPackageDir(t)
	provider := loadGenericDataProviderBoundaryFixture(t, filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "generic_data_provider_boundary", "fixtures", "data_provider_boundary_fixture.json"))

	var mapping struct {
		DatasetID       string `json:"dataset_id"`
		MappingID       string `json:"mapping_id"`
		CanonicalFields []struct {
			CanonicalField string `json:"canonical_field"`
		} `json:"canonical_fields"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "normalization_mapping_fixture.json"), &mapping)
	var rows struct {
		DatasetID string `json:"dataset_id"`
		MappingID string `json:"mapping_id"`
		Rows      []struct {
			SourceRow int `json:"source_row"`
		} `json:"rows"`
		Provenance []struct {
			SourceRow  int    `json:"source_row"`
			SourceHash string `json:"source_hash"`
		} `json:"provenance"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "normalized_rows_fixture.json"), &rows)
	var validation struct {
		ValidationErrors []struct {
			ErrorCode string `json:"error_code"`
		} `json:"validation_errors"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "normalization_validation_fixture.json"), &validation)

	var projection struct {
		SchemaVersion         int    `json:"schema_version"`
		FixtureKey            string `json:"fixture_key"`
		ProjectionKind        string `json:"projection_kind"`
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		SourceRefs            struct {
			DataProviderBoundary string `json:"data_provider_boundary"`
			NormalizationMapping string `json:"normalization_mapping"`
			NormalizedRows       string `json:"normalized_rows"`
			Validation           string `json:"validation"`
		} `json:"source_refs"`
		DatasetID        string `json:"dataset_id"`
		MappingID        string `json:"mapping_id"`
		ResponseMappings []struct {
			RequestID          string   `json:"request_id"`
			ProviderID         string   `json:"provider_id"`
			Status             string   `json:"status"`
			ProviderFixtureKey string   `json:"provider_fixture_key"`
			ReplayKey          string   `json:"replay_key"`
			ProvenanceRef      string   `json:"provenance_ref"`
			NormalizedRowRefs  []int    `json:"normalized_row_refs"`
			CanonicalFields    []string `json:"canonical_fields"`
			ValidationRefs     []string `json:"validation_refs"`
		} `json:"response_mappings"`
		ProvenanceMappings []struct {
			ProviderProvenanceRef string   `json:"provider_provenance_ref"`
			NormalizedSourceRefs  []string `json:"normalized_source_refs"`
			SourceRows            []int    `json:"source_rows"`
			SourceHashes          []string `json:"source_hashes"`
		} `json:"provenance_mappings"`
		CleanSkipMappings []struct {
			RequestID          string   `json:"request_id"`
			ProviderID         string   `json:"provider_id"`
			Status             string   `json:"status"`
			ProviderFixtureKey string   `json:"provider_fixture_key"`
			ProvenanceRef      string   `json:"provenance_ref"`
			ErrorID            string   `json:"error_id"`
			NormalizedRowRefs  []int    `json:"normalized_row_refs"`
			ValidationRefs     []string `json:"validation_refs"`
			CleanSkip          bool     `json:"clean_skip"`
		} `json:"clean_skip_mappings"`
		ProjectionAssertions map[string]bool `json:"projection_assertions"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "provider_response_projection_fixture.json"), &projection)
	if projection.SchemaVersion != 1 || projection.FixtureKey != "provider_response_projection:observations:offline:v1" ||
		projection.ProjectionKind != "data_provider_response_to_normalized_rows" ||
		!projection.ProviderFree || projection.LiveNetwork || projection.RealDependencyImports {
		t.Fatalf("projection header/provider boundary mismatch: %#v", projection)
	}
	if projection.SourceRefs.DataProviderBoundary == "" || projection.SourceRefs.NormalizationMapping == "" ||
		projection.SourceRefs.NormalizedRows == "" || projection.SourceRefs.Validation == "" {
		t.Fatalf("projection source refs incomplete: %#v", projection.SourceRefs)
	}
	if projection.DatasetID != mapping.DatasetID || projection.MappingID != mapping.MappingID ||
		projection.DatasetID != rows.DatasetID || projection.MappingID != rows.MappingID {
		t.Fatalf("projection identity does not match mapping/rows: projection=%#v mapping=%#v rows=%#v", projection, mapping, rows)
	}

	providerRequests := map[string]struct {
		providerID string
		fixtureKey string
		replayKey  string
	}{}
	for _, request := range provider.RequestEnvelopes {
		providerRequests[request.RequestID] = struct {
			providerID string
			fixtureKey string
			replayKey  string
		}{providerID: request.ProviderID, fixtureKey: request.FixtureKey, replayKey: request.ReplayKey}
	}
	providerResponses := map[string]struct {
		status        string
		provenanceRef string
		fixtureKey    string
	}{}
	for _, response := range provider.ResponseEnvelopes {
		providerResponses[response.RequestID] = struct {
			status        string
			provenanceRef string
			fixtureKey    string
		}{status: response.Status, provenanceRef: response.ProvenanceRef, fixtureKey: response.Metadata.FixtureKey}
	}
	providerProvenance := map[string]bool{}
	for _, provenance := range provider.Provenance {
		providerProvenance[provenance.ProvenanceID] = true
	}
	rowRefs := map[int]bool{}
	for _, row := range rows.Rows {
		rowRefs[row.SourceRow] = true
	}
	sourceHashes := map[string]bool{}
	for _, provenance := range rows.Provenance {
		sourceHashes[provenance.SourceHash] = true
	}
	canonicalFields := map[string]bool{}
	for _, field := range mapping.CanonicalFields {
		canonicalFields[field.CanonicalField] = true
	}
	validationCodes := map[string]bool{}
	for _, validationError := range validation.ValidationErrors {
		validationCodes[validationError.ErrorCode] = true
	}

	for _, responseMapping := range projection.ResponseMappings {
		request := providerRequests[responseMapping.RequestID]
		response := providerResponses[responseMapping.RequestID]
		if request.providerID == "" || response.status == "" {
			t.Fatalf("response mapping does not resolve provider request/response: %#v", responseMapping)
		}
		if responseMapping.ProviderID != request.providerID ||
			responseMapping.ProviderFixtureKey != request.fixtureKey ||
			responseMapping.ReplayKey != request.replayKey ||
			responseMapping.Status != response.status ||
			responseMapping.ProvenanceRef != response.provenanceRef ||
			!providerProvenance[responseMapping.ProvenanceRef] {
			t.Fatalf("response mapping drifted from provider fixture: mapping=%#v request=%#v response=%#v", responseMapping, request, response)
		}
		for _, rowRef := range responseMapping.NormalizedRowRefs {
			if !rowRefs[rowRef] {
				t.Fatalf("response mapping references unknown normalized row %d: %#v", rowRef, responseMapping)
			}
		}
		for _, field := range responseMapping.CanonicalFields {
			if !canonicalFields[field] {
				t.Fatalf("response mapping references unknown canonical field %q: %#v", field, responseMapping)
			}
		}
		for _, validationRef := range responseMapping.ValidationRefs {
			if !validationCodes[validationRef] {
				t.Fatalf("response mapping references unknown validation code %q", validationRef)
			}
		}
	}
	for _, provenanceMapping := range projection.ProvenanceMappings {
		if !providerProvenance[provenanceMapping.ProviderProvenanceRef] || len(provenanceMapping.SourceRows) == 0 {
			t.Fatalf("provenance mapping does not resolve provider provenance: %#v", provenanceMapping)
		}
		for _, rowRef := range provenanceMapping.SourceRows {
			if !rowRefs[rowRef] {
				t.Fatalf("provenance mapping references unknown row %d: %#v", rowRef, provenanceMapping)
			}
		}
		for _, hash := range provenanceMapping.SourceHashes {
			if !sourceHashes[hash] {
				t.Fatalf("provenance mapping references unknown source hash %q", hash)
			}
		}
	}
	if len(projection.CleanSkipMappings) != 1 {
		t.Fatalf("clean skip mapping count = %d, want 1", len(projection.CleanSkipMappings))
	}
	for _, cleanSkip := range projection.CleanSkipMappings {
		request := providerRequests[cleanSkip.RequestID]
		response := providerResponses[cleanSkip.RequestID]
		if request.providerID == "" || response.status != "skipped" || cleanSkip.Status != "skipped" ||
			cleanSkip.ProviderID != request.providerID || cleanSkip.ProviderFixtureKey != request.fixtureKey ||
			cleanSkip.ProvenanceRef != response.provenanceRef || !cleanSkip.CleanSkip ||
			len(cleanSkip.NormalizedRowRefs) != 0 || len(cleanSkip.ValidationRefs) == 0 {
			t.Fatalf("clean skip projection is not tied to skipped provider response: %#v", cleanSkip)
		}
	}
	for _, want := range []string{
		"all_successful_provider_responses_mapped",
		"clean_skip_responses_do_not_emit_rows",
		"provider_request_ids_are_not_assumed_equal_to_row_ids",
		"provenance_refs_are_preserved",
		"replay_keys_are_preserved",
		"canonical_fields_resolve_in_mapping",
		"projection_is_provider_free",
	} {
		if !projection.ProjectionAssertions[want] {
			t.Fatalf("projection assertion missing %q: %#v", want, projection.ProjectionAssertions)
		}
	}
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
			want := "generic_data_normalization_contracts_live_package capability=generic.ai.data_normalization.contracts entrypoint=ai.data_normalization.contracts mappings=2 provider_response_projections=1 field_policies=3 missing=2 stale=2 coercions=3 unit_transforms=2 rows=4 provenance=4 validation_errors=2 clean_skip=2 provider_free=true live_network=false imports=false model_calls=false"
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
