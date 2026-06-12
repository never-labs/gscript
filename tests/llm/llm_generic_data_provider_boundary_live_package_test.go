package leia_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericDataProviderBoundaryLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericDataProviderBoundaryPackageDir(t)
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
	if manifest.SchemaVersion != 1 || manifest.ID != "generic-data-provider-boundary" ||
		manifest.PackageName != "leia-generic-ai-data-provider-boundary" ||
		manifest.PackageBoundaryID != "generic-ai-data-provider-boundary" ||
		manifest.CapabilityID != "generic.ai.data_provider.boundary" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetworkDefault ||
		manifest.LiveModelDefault || manifest.DependsOnQRuntime || manifest.CredentialRequired {
		t.Fatalf("manifest must stay provider-free/generic/offline/credential-free: %#v", manifest)
	}
	for _, want := range []string{"generic.ai.data_provider.boundary", "generic.ai.data_provider.registry", "generic.ai.data_provider.request_envelope", "generic.ai.data_provider.response_envelope", "generic.ai.data_provider.pagination", "generic.ai.data_provider.rate_limit", "generic.ai.data_provider.auth_redaction", "generic.ai.data_provider.cache_retry_policy", "generic.ai.data_provider.provenance", "generic.ai.data_provider.error_envelope", "generic.ai.data_provider.clean_skip"} {
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
		contract.PackageName != "generic.ai.data_provider.boundary" || contract.Entrypoint != "ai.data_provider.boundary" ||
		!contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork ||
		contract.LiveModelCalls || contract.RealDependencyImports || contract.RequiresCredentials {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	for _, want := range []string{"provider_registry", "request_envelope", "response_envelope", "pagination", "rate_limit", "auth_redaction", "cache_retry_policy", "provenance", "error_envelope", "clean_skips"} {
		if contract.FieldContracts[want] == "" {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
		}
	}
}

func TestGenericDataProviderBoundaryLivePackageFixtureShape(t *testing.T) {
	base := genericDataProviderBoundaryPackageDir(t)
	fixture := loadGenericDataProviderBoundaryFixture(t, filepath.Join(base, "fixtures", "data_provider_boundary_fixture.json"))
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.LiveModelCalls || fixture.RequiresCredentials {
		t.Fatalf("fixture must stay provider-free and offline: %#v", fixture)
	}
	if len(fixture.ProviderRegistry) != 3 || len(fixture.RequestEnvelopes) != 3 ||
		len(fixture.ResponseEnvelopes) != 3 || len(fixture.Pagination) != 2 ||
		len(fixture.RateLimits) != 2 || len(fixture.AuthRedaction) != 2 ||
		len(fixture.CacheRetryPolicy) != 2 || len(fixture.Provenance) != 3 ||
		len(fixture.ErrorEnvelopes) != 1 || len(fixture.AdapterBoundaries) != 3 {
		t.Fatalf("fixture counts drifted: providers=%d requests=%d responses=%d pagination=%d rates=%d redactions=%d policies=%d provenance=%d errors=%d adapters=%d",
			len(fixture.ProviderRegistry), len(fixture.RequestEnvelopes), len(fixture.ResponseEnvelopes), len(fixture.Pagination), len(fixture.RateLimits), len(fixture.AuthRedaction), len(fixture.CacheRetryPolicy), len(fixture.Provenance), len(fixture.ErrorEnvelopes), len(fixture.AdapterBoundaries))
	}
	providers := map[string]bool{}
	for _, provider := range fixture.ProviderRegistry {
		if provider.ProviderID == "" || provider.Capability == "" || provider.FixtureKey == "" ||
			provider.SchemaRef == "" || provider.LiveNetwork {
			t.Fatalf("provider registry entry invalid: %#v", provider)
		}
		providers[provider.ProviderID] = true
	}
	requests := map[string]string{}
	for _, request := range fixture.RequestEnvelopes {
		if request.RequestID == "" || !providers[request.ProviderID] || request.Operation == "" ||
			request.FixtureKey == "" || request.ReplayKey == "" || request.LiveNetwork ||
			request.RealDependencyImports {
			t.Fatalf("request envelope invalid or unresolved: %#v", request)
		}
		requests[request.RequestID] = request.ProviderID
	}
	provenance := map[string]bool{}
	for _, source := range fixture.Provenance {
		if source.ProvenanceID == "" || source.FixtureKey == "" || source.SourceSchema == "" || !source.ReplayReady {
			t.Fatalf("provenance invalid: %#v", source)
		}
		provenance[source.ProvenanceID] = true
	}
	for _, response := range fixture.ResponseEnvelopes {
		if requests[response.RequestID] == "" || response.Status == "" || !provenance[response.ProvenanceRef] ||
			response.Metadata.ProviderID == "" || response.Metadata.FixtureKey == "" || !response.Metadata.ReplayReady {
			t.Fatalf("response envelope invalid or unresolved: %#v", response)
		}
		if response.Status == "skipped" && (response.Error == nil || !response.Error.CleanSkip) {
			t.Fatalf("skipped response must carry clean-skip error: %#v", response)
		}
	}
	for _, redaction := range fixture.AuthRedaction {
		if !providers[redaction.ProviderID] || redaction.Status != "redacted" ||
			redaction.SecretValuesPresent || len(redaction.RedactedFields) == 0 {
			t.Fatalf("auth redaction invalid: %#v", redaction)
		}
	}
	for _, err := range fixture.ErrorEnvelopes {
		if requests[err.RequestID] == "" || err.Kind == "" || err.Retryable || !err.CleanSkip {
			t.Fatalf("error envelope invalid: %#v", err)
		}
	}
	for _, boundary := range fixture.AdapterBoundaries {
		if boundary.DependencyImported || boundary.LiveNetwork || !boundary.CleanSkip {
			t.Fatalf("adapter boundary must clean-skip: %#v", boundary)
		}
	}
}

func TestGenericDataProviderBoundaryLivePackageIsDomainNeutral(t *testing.T) {
	base := genericDataProviderBoundaryPackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"finrobot", "acme", "aapl", "ticker", "equity", "investment", "valuation_engine", "target_price", "dcf", "sec.gov", "10-k", "finance.", "yfinance", "finnhub", "fmp", "reddit"} {
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

func TestGenericDataProviderBoundaryLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericDataProviderBoundaryPackageDir(t)
	schema := filepath.Join(base, "schemas", "generic_data_provider_boundary_v1.schema.json")
	assertDocumentPipelineSchemaRequired(t, schema, []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "live_model_calls", "requires_credentials", "provider_registry", "request_envelopes", "response_envelopes", "pagination", "rate_limits", "auth_redaction", "cache_retry_policy", "provenance", "error_envelopes", "adapter_boundaries"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "provider_registry", "items"}, []string{"provider_id", "capability", "fixture_key", "schema_ref", "auth_policy", "live_network"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "request_envelopes", "items"}, []string{"request_id", "provider_id", "operation", "query", "fixture_key", "replay_key", "live_network", "real_dependency_imports"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "response_envelopes", "items"}, []string{"request_id", "status", "data", "metadata", "provenance_ref", "error"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "data_provider_request_envelope_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "request_envelopes"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "data_provider_response_envelope_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "response_envelopes", "provenance", "error_envelopes"})
}

func TestGenericDataProviderBoundaryLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericDataProviderBoundaryPackageDir(t), "main.leia")
	want := "generic_data_provider_boundary_live_package capability=generic.ai.data_provider.boundary entrypoint=ai.data_provider.boundary providers=3 requests=3 responses=3 pagination=2 rate_limits=2 redactions=2 policies=2 provenance=3 errors=1 clean_skip=3 provider_free=true live_network=false imports=false model_calls=false"
	for _, result := range runFinRobotLivePackageSummarySmoke(t, path, "generic_data_provider_boundary_live_package_summary", "generic_data_provider_boundary_live_package", leia.LibString) {
		if result.Summary != want {
			t.Fatalf("summary = %#v, want %#v", result.Summary, want)
		}
	}
}

type genericDataProviderBoundaryFixture struct {
	ProviderFree          bool `json:"provider_free"`
	LiveNetwork           bool `json:"live_network"`
	RealDependencyImports bool `json:"real_dependency_imports"`
	LiveModelCalls        bool `json:"live_model_calls"`
	RequiresCredentials   bool `json:"requires_credentials"`
	ProviderRegistry      []struct {
		ProviderID  string `json:"provider_id"`
		Capability  string `json:"capability"`
		FixtureKey  string `json:"fixture_key"`
		SchemaRef   string `json:"schema_ref"`
		LiveNetwork bool   `json:"live_network"`
	} `json:"provider_registry"`
	RequestEnvelopes []struct {
		RequestID             string `json:"request_id"`
		ProviderID            string `json:"provider_id"`
		Operation             string `json:"operation"`
		FixtureKey            string `json:"fixture_key"`
		ReplayKey             string `json:"replay_key"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
	} `json:"request_envelopes"`
	ResponseEnvelopes []struct {
		RequestID     string `json:"request_id"`
		Status        string `json:"status"`
		ProvenanceRef string `json:"provenance_ref"`
		Metadata      struct {
			ProviderID  string `json:"provider_id"`
			FixtureKey  string `json:"fixture_key"`
			ReplayReady bool   `json:"replay_ready"`
		} `json:"metadata"`
		Error *struct {
			CleanSkip bool `json:"clean_skip"`
		} `json:"error"`
	} `json:"response_envelopes"`
	Pagination       []any `json:"pagination"`
	RateLimits       []any `json:"rate_limits"`
	CacheRetryPolicy []any `json:"cache_retry_policy"`
	AuthRedaction    []struct {
		ProviderID          string   `json:"provider_id"`
		Status              string   `json:"status"`
		SecretValuesPresent bool     `json:"secret_values_present"`
		RedactedFields      []string `json:"redacted_fields"`
	} `json:"auth_redaction"`
	Provenance []struct {
		ProvenanceID string `json:"provenance_id"`
		FixtureKey   string `json:"fixture_key"`
		SourceSchema string `json:"source_schema"`
		ReplayReady  bool   `json:"replay_ready"`
	} `json:"provenance"`
	ErrorEnvelopes []struct {
		RequestID string `json:"request_id"`
		Kind      string `json:"kind"`
		Retryable bool   `json:"retryable"`
		CleanSkip bool   `json:"clean_skip"`
	} `json:"error_envelopes"`
	AdapterBoundaries []struct {
		ID                 string `json:"id"`
		Capability         string `json:"capability"`
		DependencyImported bool   `json:"dependency_imported"`
		LiveNetwork        bool   `json:"live_network"`
		CleanSkip          bool   `json:"clean_skip"`
	} `json:"adapter_boundaries"`
}

func loadGenericDataProviderBoundaryFixture(t *testing.T, path string) genericDataProviderBoundaryFixture {
	t.Helper()
	var fixture genericDataProviderBoundaryFixture
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func genericDataProviderBoundaryPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_data_provider_boundary")
}
