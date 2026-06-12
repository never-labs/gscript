package leia_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type financeFacadeLiveManifest struct {
	SchemaVersion               int      `json:"schema_version"`
	ID                          string   `json:"id"`
	PackageName                 string   `json:"package_name"`
	ProviderFree                bool     `json:"provider_free"`
	LiveNetworkDefault          bool     `json:"live_network_default"`
	RealDependencyImportDefault bool     `json:"real_dependency_import_default"`
	SourceModules               []string `json:"source_modules"`
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
		CleanSkipWithoutDependency  bool   `json:"clean_skip_without_dependency"`
		FixtureHook                 string `json:"fixture_hook"`
	} `json:"default_policy"`
	Entrypoints           map[string]string               `json:"entrypoints"`
	Schemas               map[string]string               `json:"schemas"`
	Fixtures              map[string]string               `json:"fixtures"`
	Modules               []financeFacadeModule           `json:"modules"`
	FunctionMatrix        []financeFacadeFunctionMatrix   `json:"function_matrix"`
	ProviderFallbackOrder []financeFacadeFallbackProvider `json:"provider_fallback_order"`
	CachePolicy           financeFacadePolicy             `json:"cache_policy"`
	RetryPolicy           financeFacadePolicy             `json:"retry_policy"`
	RateLimitPolicy       financeFacadePolicy             `json:"rate_limit_policy"`
	TestGates             []string                        `json:"test_gates"`
}

type financeFacadeModule struct {
	ID            string   `json:"id"`
	SourceModule  string   `json:"source_module"`
	Capabilities  []string `json:"capabilities"`
	OutputSchemas []string `json:"output_schemas"`
}

type financeFacadeFallbackProvider struct {
	ID                 string `json:"id"`
	Capability         string `json:"capability"`
	FixtureKey         string `json:"fixture_key"`
	LiveNetwork        bool   `json:"live_network"`
	DependencyImported bool   `json:"dependency_imported"`
	CredentialRequired bool   `json:"credential_required"`
	CleanSkip          bool   `json:"clean_skip"`
}

type financeFacadeFunctionMatrix struct {
	ID                    string         `json:"id"`
	SourceModule          string         `json:"source_module"`
	Provider              string         `json:"provider"`
	Capability            string         `json:"capability"`
	Fixture               string         `json:"fixture"`
	Schema                string         `json:"schema"`
	FallbackOrder         []string       `json:"fallback_order"`
	ProviderFree          bool           `json:"provider_free"`
	LiveNetwork           bool           `json:"live_network"`
	RealDependencyImports bool           `json:"real_dependency_imports"`
	CleanSkip             bool           `json:"clean_skip"`
	Metadata              map[string]any `json:"metadata"`
}

type financeFacadePolicy struct {
	Enabled     bool   `json:"enabled"`
	Mode        string `json:"mode"`
	LiveNetwork bool   `json:"live_network"`
}

func TestFinRobotFinanceFacadeLivePackageManifest(t *testing.T) {
	base := financeFacadeLivePackageDir(t)
	manifest := loadFinanceFacadeLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-finance-facade-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-finance-facade" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	if !strings.Contains(manifest.Credentials.Policy, "yfinance") || !strings.Contains(manifest.Credentials.Policy, "OpenBB") {
		t.Fatalf("credential policy should name future external providers: %q", manifest.Credentials.Policy)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_finance_facade_live_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}

	wantSources := []string{
		"finrobot/data_source/finance_data.py",
		"finrobot/data_source/market_data.py",
		"finrobot/data_source/market_data_api.py",
	}
	if !reflect.DeepEqual(manifest.SourceModules, wantSources) {
		t.Fatalf("source modules = %#v, want %#v", manifest.SourceModules, wantSources)
	}
	for _, key := range []string{"smoke", "finance_facade_contract", "provider_fallback_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"market_data_table", "fundamental_table", "function_fixture", "provider_fallback", "provider_edge_matrix", "error_envelope"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertFinanceFacadeJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "market_data_table", "fundamental_table", "error_envelope"} {
		path := manifest.Fixtures[key]
		if path == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertFinanceFacadeJSONFile(t, filepath.Join(base, path))
	}

	var moduleIDs []string
	for _, module := range manifest.Modules {
		moduleIDs = append(moduleIDs, module.ID)
		if module.ID == "" || module.SourceModule == "" || len(module.Capabilities) < 5 || len(module.OutputSchemas) == 0 {
			t.Fatalf("module metadata incomplete: %#v", module)
		}
	}
	sort.Strings(moduleIDs)
	wantModuleIDs := []string{"finance_data", "market_data", "market_data_api"}
	if !reflect.DeepEqual(moduleIDs, wantModuleIDs) {
		t.Fatalf("module ids = %#v, want %#v", moduleIDs, wantModuleIDs)
	}
	assertFinanceFacadeFunctionMatrix(t, base, manifest.FunctionMatrix)

	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"fallback", "typed table", "function matrix", "market_data_api.py", "cache", "retry", "rate-limit", "provenance", "error envelope", "edge matrix", "partial outage", "stale rows", "empty result", "malformed response", "capability denial", "terms metadata"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotFinanceFacadeFallbackContract(t *testing.T) {
	base := financeFacadeLivePackageDir(t)
	manifest := loadFinanceFacadeLiveManifest(t, base)

	var ids []string
	for _, provider := range manifest.ProviderFallbackOrder {
		ids = append(ids, provider.ID)
		if provider.ID == "" || provider.Capability == "" || provider.FixtureKey == "" {
			t.Fatalf("fallback provider metadata incomplete: %#v", provider)
		}
		if provider.LiveNetwork || provider.DependencyImported || provider.CredentialRequired || !provider.CleanSkip {
			t.Fatalf("fallback provider must be fixture-only and clean-skip safe: %#v", provider)
		}
		if !strings.HasPrefix(provider.Capability, "finance.facade.provider.") {
			t.Fatalf("%s capability = %q", provider.ID, provider.Capability)
		}
	}
	wantOrder := []string{"yfinance", "fmp", "finnhub", "sec", "openbb"}
	if !reflect.DeepEqual(ids, wantOrder) {
		t.Fatalf("provider fallback order = %#v, want %#v", ids, wantOrder)
	}

	if !manifest.CachePolicy.Enabled || manifest.CachePolicy.LiveNetwork ||
		!manifest.RetryPolicy.Enabled || manifest.RetryPolicy.LiveNetwork ||
		!manifest.RateLimitPolicy.Enabled || manifest.RateLimitPolicy.LiveNetwork {
		t.Fatalf("cache/retry/rate-limit policies must be enabled but offline: cache=%#v retry=%#v rate=%#v", manifest.CachePolicy, manifest.RetryPolicy, manifest.RateLimitPolicy)
	}

	var contract struct {
		ProviderFree               bool                            `json:"provider_free"`
		LiveNetwork                bool                            `json:"live_network"`
		RealDependencyImports      bool                            `json:"real_dependency_imports"`
		CleanSkipWithoutDependency bool                            `json:"clean_skip_without_dependency"`
		FallbackOrder              []financeFacadeFallbackProvider `json:"fallback_order"`
		FallbackRules              []string                        `json:"fallback_rules"`
		AcceptanceGates            []string                        `json:"acceptance_gates"`
	}
	decodeFinanceFacadeJSONFile(t, filepath.Join(base, "contracts", "provider_fallback_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || !contract.CleanSkipWithoutDependency || len(contract.FallbackOrder) != 5 {
		t.Fatalf("fallback contract header/count = %#v", contract)
	}
	for _, provider := range contract.FallbackOrder {
		if provider.LiveNetwork || provider.DependencyImported || provider.CredentialRequired || !provider.CleanSkip {
			t.Fatalf("fallback contract must not enable live providers: %#v", provider)
		}
	}
	joined := strings.ToLower(strings.Join(append(contract.FallbackRules, contract.AcceptanceGates...), " "))
	for _, want := range []string{"listed order", "typed table", "error_envelope", "rate limit", "clean-skip"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("fallback contract missing %q: %s", want, joined)
		}
	}
}

func TestFinRobotFinanceFacadeContractsAndFixtures(t *testing.T) {
	base := financeFacadeLivePackageDir(t)

	var contract struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Modules               []struct {
			ID             string   `json:"id"`
			SourceModule   string   `json:"source_module"`
			RequiredFields []string `json:"required_fields"`
		} `json:"modules"`
		FunctionMatrix []struct {
			ID                   string   `json:"id"`
			Provider             string   `json:"provider"`
			Capability           string   `json:"capability"`
			Schema               string   `json:"schema"`
			Fixture              string   `json:"fixture"`
			RequiredOutputFields []string `json:"required_output_fields"`
		} `json:"function_matrix"`
		TypedTables []struct {
			ID              string   `json:"id"`
			Schema          string   `json:"schema"`
			Fixture         string   `json:"fixture"`
			PrimaryKey      []string `json:"primary_key"`
			RequiredColumns []string `json:"required_columns"`
		} `json:"typed_tables"`
		ProvenanceContract struct {
			RequiredFields  []string `json:"required_fields"`
			RedactSourceURL bool     `json:"redact_source_url"`
			LiveNetwork     bool     `json:"live_network"`
		} `json:"provenance_contract"`
		ErrorEnvelope struct {
			Schema         string   `json:"schema"`
			Fixture        string   `json:"fixture"`
			RequiredFields []string `json:"required_fields"`
		} `json:"error_envelope"`
		AcceptanceGates []string `json:"acceptance_gates"`
	}
	decodeFinanceFacadeJSONFile(t, filepath.Join(base, "contracts", "finance_facade_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || len(contract.Modules) != 3 || len(contract.TypedTables) != 2 || len(contract.FunctionMatrix) != 9 {
		t.Fatalf("contract header/modules/tables = %#v", contract)
	}
	for _, fn := range contract.FunctionMatrix {
		if fn.ID == "" || fn.Provider == "" || fn.Capability == "" || fn.Schema == "" || fn.Fixture == "" || len(fn.RequiredOutputFields) == 0 {
			t.Fatalf("function matrix contract incomplete: %#v", fn)
		}
		if !strings.HasPrefix(fn.Capability, "finance.facade.") {
			t.Fatalf("%s capability = %q", fn.ID, fn.Capability)
		}
		assertFinanceFacadeJSONFile(t, filepath.Join(base, fn.Schema))
		assertFinanceFacadeJSONFile(t, filepath.Join(base, fn.Fixture))
	}
	for _, table := range contract.TypedTables {
		if table.ID == "" || table.Schema == "" || table.Fixture == "" || len(table.PrimaryKey) == 0 || len(table.RequiredColumns) < 7 {
			t.Fatalf("typed table contract incomplete: %#v", table)
		}
		assertFinanceFacadeJSONFile(t, filepath.Join(base, table.Schema))
		assertFinanceFacadeJSONFile(t, filepath.Join(base, table.Fixture))
	}
	if !contract.ProvenanceContract.RedactSourceURL || contract.ProvenanceContract.LiveNetwork || len(contract.ProvenanceContract.RequiredFields) < 7 {
		t.Fatalf("provenance contract incomplete: %#v", contract.ProvenanceContract)
	}
	if contract.ErrorEnvelope.Schema == "" || contract.ErrorEnvelope.Fixture == "" || len(contract.ErrorEnvelope.RequiredFields) < 7 {
		t.Fatalf("error envelope contract incomplete: %#v", contract.ErrorEnvelope)
	}

	var index struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Fixtures              []struct {
			FixtureKey string         `json:"fixture_key"`
			Capability string         `json:"capability"`
			Path       string         `json:"path"`
			Schema     string         `json:"schema"`
			Metadata   map[string]any `json:"metadata"`
		} `json:"fixtures"`
	}
	decodeFinanceFacadeJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 13 {
		t.Fatalf("fixture index header/count = %#v", index)
	}
	indexCapabilities := map[string]bool{}
	fixtureKeys := map[string]bool{}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Capability == "" || fixture.Path == "" || fixture.Schema == "" {
			t.Fatalf("fixture metadata incomplete: %#v", fixture)
		}
		if fixture.Metadata["replay_ready"] != true {
			t.Fatalf("%s replay_ready = %#v", fixture.FixtureKey, fixture.Metadata["replay_ready"])
		}
		fixtureKeys[fixture.FixtureKey] = true
		indexCapabilities[fixture.Capability] = true
		assertFinanceFacadeJSONFile(t, filepath.Join(base, fixture.Path))
		assertFinanceFacadeJSONFile(t, filepath.Join(base, fixture.Schema))
	}
	for _, capability := range []string{
		"finance.facade.fmp.enterprise_value.fetch",
		"finance.facade.fmp.ratios_key_metrics.fetch",
		"finance.facade.fmp.ratings.fetch",
		"finance.facade.fmp.targets.fetch",
		"finance.facade.fmp.technical_indicators.fetch",
		"finance.facade.fmp.company_profile.fetch",
		"finance.facade.yfinance.company_profile.fetch",
		"finance.facade.yfinance.market_cap.fetch",
		"finance.facade.provider.fallback.resolve",
		"finance.facade.provider.edge_matrix",
		"finance.facade.error.envelope",
	} {
		if !indexCapabilities[capability] {
			t.Fatalf("fixture index missing capability %q", capability)
		}
	}
	if !fixtureKeys["edge:provider_matrix:offline"] {
		t.Fatalf("fixture index missing provider edge matrix fixture: %#v", fixtureKeys)
	}
}

func TestFinRobotFinanceFacadeProviderEdgeMatrix(t *testing.T) {
	base := financeFacadeLivePackageDir(t)

	var contract financeFacadeProviderEdgeContract
	decodeFinanceFacadeJSONFile(t, filepath.Join(base, "contracts", "provider_edge_matrix_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || !contract.CleanSkipWithoutDependency {
		t.Fatalf("edge matrix contract must stay provider-free and clean-skip safe: %#v", contract)
	}
	if contract.Fixture == "" || contract.Schema == "" || len(contract.DialectCapabilities) < 9 || len(contract.EdgeCases) < 8 {
		t.Fatalf("edge matrix contract incomplete: %#v", contract)
	}
	assertFinanceFacadeJSONFile(t, filepath.Join(base, contract.Fixture))
	assertFinanceFacadeJSONFile(t, filepath.Join(base, contract.Schema))

	capabilityText := strings.Join(contract.DialectCapabilities, " ")
	for _, want := range []string{"retry_observable", "cache_state", "partial_outage", "stale_rows", "empty_result", "malformed_response", "capability_denied", "fallback_explanation", "terms_metadata"} {
		if !strings.Contains(capabilityText, want) {
			t.Fatalf("edge dialect capabilities missing %q: %#v", want, contract.DialectCapabilities)
		}
	}

	var fixture financeFacadeProviderEdgeFixture
	decodeFinanceFacadeJSONFile(t, filepath.Join(base, "fixtures", "provider_edge_matrix_fixture.json"), &fixture)
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.Dialect != "finance_facade.provider_edge_matrix.v1" {
		t.Fatalf("edge matrix fixture header = %#v", fixture)
	}

	providerTerms := map[string]financeFacadeProviderTerms{}
	for _, provider := range fixture.Providers {
		if provider.ProviderID == "" || len(provider.Capabilities) == 0 {
			t.Fatalf("provider edge metadata incomplete: %#v", provider)
		}
		if provider.Terms.TermsRef == "" || len(provider.Terms.AllowedUses) == 0 ||
			!provider.Terms.AttributionRequired || provider.Terms.Redistribution == "" ||
			provider.Terms.LiveNetwork || provider.Terms.CredentialRequired {
			t.Fatalf("%s terms metadata incomplete or live-enabled: %#v", provider.ProviderID, provider.Terms)
		}
		providerTerms[provider.ProviderID] = provider.Terms
	}
	if len(providerTerms) < 5 {
		t.Fatalf("edge fixture should provide terms metadata for the provider dialect: %#v", providerTerms)
	}

	rowsByID := map[string]financeFacadeProviderEdgeRow{}
	categories := map[string]bool{}
	for _, row := range fixture.EdgeRows {
		if row.CaseID == "" || row.Category == "" || row.ProviderID == "" || row.FixtureKey == "" || row.Operation == "" {
			t.Fatalf("edge row metadata incomplete: %#v", row)
		}
		if row.ErrorEnvelope.OK || row.ErrorEnvelope.ErrorCode == "" || row.Retry.Attempts < 0 || row.CacheState == "" {
			t.Fatalf("edge row retry/cache/error envelope incomplete: %#v", row)
		}
		if row.Fallback.Required && (len(row.Fallback.Attempted) < 2 || row.Fallback.SelectedProvider == "" || row.Fallback.Explanation == "") {
			t.Fatalf("required fallback row missing explanation/attempts: %#v", row)
		}
		if row.Category != "provider_terms_metadata" {
			if _, ok := providerTerms[row.ProviderID]; !ok {
				t.Fatalf("edge row %s references provider without terms metadata: %q", row.CaseID, row.ProviderID)
			}
			if row.Provenance.FixtureKey == "" || !row.Provenance.SourceURLRedacted || !row.Provenance.ReplayReady {
				t.Fatalf("edge row provenance incomplete: %#v", row.Provenance)
			}
		}
		rowsByID[row.CaseID] = row
		categories[row.Category] = true
	}

	requiredCategories := []string{
		"retry_cache_error",
		"partial_provider_outage",
		"stale_rows",
		"empty_provider_result",
		"malformed_response",
		"capability_denied",
		"provider_terms_metadata",
	}
	for _, category := range requiredCategories {
		if !categories[category] {
			t.Fatalf("edge matrix missing category %q: %#v", category, categories)
		}
	}

	for _, edgeCase := range contract.EdgeCases {
		row, ok := rowsByID[edgeCase.ID]
		if !ok {
			t.Fatalf("contract edge case %q missing fixture row", edgeCase.ID)
		}
		if row.Category != edgeCase.Category ||
			row.FixtureKey != edgeCase.TriggerFixtureKey ||
			row.ErrorEnvelope.ErrorCode != edgeCase.ExpectedErrorCode ||
			row.Retry.Retryable != edgeCase.Retryable ||
			row.CacheState != edgeCase.CacheState ||
			row.Fallback.Required != edgeCase.FallbackRequired {
			t.Fatalf("fixture row does not satisfy contract edge case %#v: %#v", edgeCase, row)
		}
		if edgeCase.FallbackExplanationRequired && row.Fallback.Explanation == "" {
			t.Fatalf("edge case %s requires fallback explanation", edgeCase.ID)
		}
	}
}

func TestFinRobotFinanceFacadeFunctionFixtures(t *testing.T) {
	base := financeFacadeLivePackageDir(t)
	manifest := loadFinanceFacadeLiveManifest(t, base)

	wantFunctions := map[string]struct {
		provider string
		dataset  string
		fields   []string
	}{
		"fmp_get_enterprise_value":        {provider: "fmp", dataset: "enterprise_value", fields: []string{"enterprise_value", "market_cap", "total_debt", "cash_and_equivalents"}},
		"fmp_get_ratios_key_metrics":      {provider: "fmp", dataset: "ratios_key_metrics", fields: []string{"pe_ratio", "ev_to_ebitda", "gross_margin", "operating_margin"}},
		"fmp_get_ratings":                 {provider: "fmp", dataset: "ratings", fields: []string{"rating", "rating_score", "recommendation"}},
		"fmp_get_price_targets":           {provider: "fmp", dataset: "price_targets", fields: []string{"target_low", "target_median", "target_high", "analyst_count"}},
		"fmp_get_technical_indicators":    {provider: "fmp", dataset: "technical_indicators", fields: []string{"sma_20", "sma_50", "ema_20", "rsi_14", "macd"}},
		"fmp_get_company_profile":         {provider: "fmp", dataset: "company_profile", fields: []string{"company_name", "exchange", "sector", "industry"}},
		"yfinance_get_company_profile":    {provider: "yfinance", dataset: "company_profile", fields: []string{"company_name", "exchange", "sector", "industry"}},
		"yfinance_get_market_cap":         {provider: "yfinance", dataset: "market_cap", fields: []string{"market_cap", "shares_outstanding", "last_price"}},
		"resolve_provider_fallback_order": {provider: "facade", dataset: "fallback_order", fields: []string{"ordered_providers", "selected_provider", "reason"}},
	}
	if len(manifest.FunctionMatrix) != len(wantFunctions) {
		t.Fatalf("function matrix length = %d, want %d", len(manifest.FunctionMatrix), len(wantFunctions))
	}

	for _, entry := range manifest.FunctionMatrix {
		want, ok := wantFunctions[entry.ID]
		if !ok {
			t.Fatalf("unexpected function matrix entry %q", entry.ID)
		}
		if entry.Provider != want.provider || !entry.ProviderFree || entry.LiveNetwork || entry.RealDependencyImports || !entry.CleanSkip {
			t.Fatalf("function matrix provider-free metadata invalid for %s: %#v", entry.ID, entry)
		}
		if entry.SourceModule != "finrobot/data_source/market_data_api.py" || !strings.HasPrefix(entry.Capability, "finance.facade.") || entry.Schema == "" || entry.Fixture == "" {
			t.Fatalf("function matrix entry incomplete: %#v", entry)
		}
		if entry.Metadata["replay_ready"] != true || entry.Metadata["domain"] != want.dataset {
			t.Fatalf("%s metadata = %#v, want domain %q replay_ready=true", entry.ID, entry.Metadata, want.dataset)
		}
		if len(entry.FallbackOrder) == 0 {
			t.Fatalf("%s fallback order missing", entry.ID)
		}
		assertFinanceFacadeJSONFile(t, filepath.Join(base, entry.Schema))

		var fixture financeFacadeFunctionFixture
		decodeFinanceFacadeJSONFile(t, filepath.Join(base, entry.Fixture), &fixture)
		if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.FunctionID != entry.ID || fixture.Provider != entry.Provider || fixture.Capability != entry.Capability || fixture.Symbol != "ACME" || fixture.Dataset != want.dataset {
			t.Fatalf("function fixture header mismatch for %s: %#v", entry.ID, fixture)
		}
		if fixture.SourceModule != "finrobot/data_source/market_data_api.py" || fixture.Input["symbol"] != "ACME" || fixture.Output.SourceRef == "" {
			t.Fatalf("function fixture shape incomplete for %s: %#v", entry.ID, fixture)
		}
		for _, field := range want.fields {
			if _, ok := fixture.Output.Fields[field]; !ok {
				t.Fatalf("%s output missing field %q: %#v", entry.ID, field, fixture.Output.Fields)
			}
		}
		if !reflect.DeepEqual(fixture.Fallback.Order, entry.FallbackOrder) || fixture.Fallback.SelectedProvider == "" || len(fixture.Fallback.Attempted) == 0 || !fixture.Fallback.CleanSkipWithoutDependency {
			t.Fatalf("%s fallback metadata mismatch: %#v vs %#v", entry.ID, fixture.Fallback, entry.FallbackOrder)
		}
		if entry.ID == "resolve_provider_fallback_order" && fixture.Fallback.ErrorEnvelopeFixture != "fixtures/error_envelope_yfinance_rate_limit_fixture.json" {
			t.Fatalf("fallback fixture should reference provider error envelope: %#v", fixture.Fallback)
		}
		assertFinanceFacadeProvenance(t, fixture.Provenance)
		if fixture.Provenance.Provider != fixture.Provider {
			t.Fatalf("%s provenance provider = %q, want %q", entry.ID, fixture.Provenance.Provider, fixture.Provider)
		}
	}
}

func TestFinRobotFinanceFacadeTypedTableAndErrorFixtures(t *testing.T) {
	base := financeFacadeLivePackageDir(t)

	var marketFixture struct {
		ProviderFree bool              `json:"provider_free"`
		LiveNetwork  bool              `json:"live_network"`
		TableID      string            `json:"table_id"`
		Dataset      string            `json:"dataset"`
		TypedColumns map[string]string `json:"typed_columns"`
		Rows         []struct {
			Symbol        string  `json:"symbol"`
			AsOf          string  `json:"as_of"`
			Open          float64 `json:"open"`
			High          float64 `json:"high"`
			Low           float64 `json:"low"`
			Close         float64 `json:"close"`
			AdjustedClose float64 `json:"adjusted_close"`
			Volume        int     `json:"volume"`
			Currency      string  `json:"currency"`
			SourceRef     string  `json:"source_ref"`
		} `json:"rows"`
		Provenance financeFacadeProvenance `json:"provenance"`
	}
	decodeFinanceFacadeJSONFile(t, filepath.Join(base, "fixtures", "market_data_ACME_daily_fixture.json"), &marketFixture)
	if !marketFixture.ProviderFree || marketFixture.LiveNetwork || marketFixture.TableID != "market_data_table" || marketFixture.Dataset != "daily_prices" || len(marketFixture.Rows) != 3 {
		t.Fatalf("market fixture header/count = %#v", marketFixture)
	}
	for _, column := range []string{"symbol", "as_of", "close", "volume", "source_ref"} {
		if marketFixture.TypedColumns[column] == "" {
			t.Fatalf("market fixture missing typed column %q: %#v", column, marketFixture.TypedColumns)
		}
	}
	assertFinanceFacadeProvenance(t, marketFixture.Provenance)
	for _, row := range marketFixture.Rows {
		if row.Symbol != "ACME" || row.AsOf == "" || row.High < row.Low || row.Close <= 0 || row.Volume <= 0 || row.Currency != "USD" || row.SourceRef == "" {
			t.Fatalf("market row incomplete: %#v", row)
		}
	}

	var fundamentalFixture struct {
		ProviderFree bool              `json:"provider_free"`
		LiveNetwork  bool              `json:"live_network"`
		TableID      string            `json:"table_id"`
		Dataset      string            `json:"dataset"`
		TypedColumns map[string]string `json:"typed_columns"`
		Rows         []struct {
			Symbol    string  `json:"symbol"`
			Metric    string  `json:"metric"`
			Period    string  `json:"period"`
			Value     float64 `json:"value"`
			Unit      string  `json:"unit"`
			Currency  string  `json:"currency"`
			SourceRef string  `json:"source_ref"`
		} `json:"rows"`
		Provenance financeFacadeProvenance `json:"provenance"`
	}
	decodeFinanceFacadeJSONFile(t, filepath.Join(base, "fixtures", "fundamentals_ACME_annual_fixture.json"), &fundamentalFixture)
	if !fundamentalFixture.ProviderFree || fundamentalFixture.LiveNetwork || fundamentalFixture.TableID != "fundamental_table" || fundamentalFixture.Dataset != "annual_fundamentals" || len(fundamentalFixture.Rows) != 3 {
		t.Fatalf("fundamental fixture header/count = %#v", fundamentalFixture)
	}
	for _, row := range fundamentalFixture.Rows {
		if row.Symbol != "ACME" || row.Metric == "" || row.Period != "FY2025" || row.Value == 0 || row.Unit == "" || row.Currency != "USD" || row.SourceRef == "" {
			t.Fatalf("fundamental row incomplete: %#v", row)
		}
	}
	assertFinanceFacadeProvenance(t, fundamentalFixture.Provenance)

	var errFixture struct {
		ProviderFree      bool                    `json:"provider_free"`
		LiveNetwork       bool                    `json:"live_network"`
		OK                bool                    `json:"ok"`
		ErrorCode         string                  `json:"error_code"`
		Provider          string                  `json:"provider"`
		Retryable         bool                    `json:"retryable"`
		RetryAfterSeconds int                     `json:"retry_after_seconds"`
		FallbackAttempted []string                `json:"fallback_attempted"`
		Provenance        financeFacadeProvenance `json:"provenance"`
	}
	decodeFinanceFacadeJSONFile(t, filepath.Join(base, "fixtures", "error_envelope_yfinance_rate_limit_fixture.json"), &errFixture)
	if !errFixture.ProviderFree || errFixture.LiveNetwork || errFixture.OK || errFixture.ErrorCode != "RATE_LIMITED" || errFixture.Provider != "yfinance" || !errFixture.Retryable || errFixture.RetryAfterSeconds <= 0 || len(errFixture.FallbackAttempted) < 2 {
		t.Fatalf("error envelope fixture incomplete: %#v", errFixture)
	}
	assertFinanceFacadeProvenance(t, errFixture.Provenance)
}

func TestFinRobotFinanceFacadePlanManifestEntry(t *testing.T) {
	plan := loadLivePackagePlanManifest(t, repoRoot(t))
	var found bool
	for _, pkg := range plan.Packages {
		if pkg.ID != "finance_facade" {
			continue
		}
		found = true
		if pkg.PackageName != "leia-finrobot-finance-facade" ||
			pkg.SkeletonDirectory != "examples/ai/finrobot_translation/live_packages/finance_facade" ||
			pkg.MigrationSource == "" ||
			!pkg.NoBuiltInGuarantee {
			t.Fatalf("finance_facade plan entry incomplete: %#v", pkg)
		}
		if len(pkg.Capabilities) < 8 || len(pkg.TestGates) < 4 {
			t.Fatalf("finance_facade plan capabilities/gates incomplete: %#v", pkg)
		}
		joined := strings.ToLower(strings.Join(append(pkg.Capabilities, pkg.TestGates...), " "))
		for _, want := range []string{"market_data", "fundamentals", "fallback", "cache", "retry", "rate", "provenance", "error"} {
			if !strings.Contains(joined, want) {
				t.Fatalf("finance_facade plan entry missing %q: %s", want, joined)
			}
		}
	}
	if !found {
		t.Fatal("live_package_plan_manifest.json missing finance_facade package entry")
	}
}

func TestFinRobotFinanceFacadeLivePackageNoLiveImports(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(financeFacadeLivePackageDir(t), "main.leia"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pattern := range []string{
		`(?m)^\s*import\s+`,
		`(?m)^\s*use\s+`,
		`(?m)^\s*load\s*\(`,
		`(?m)^\s*require\s*\(`,
		`(?m)^\s*(yfinance|finnhub|openbb|requests|http|pandas|numpy)\s*[.(]`,
	} {
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("main.leia contains live dependency loader matching %q", pattern)
		}
	}
}

func TestFinRobotFinanceFacadeLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(financeFacadeLivePackageDir(t), "main.leia")

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
			got, err := vm.Get("finance_facade_live_package_summary")
			if err != nil {
				t.Fatalf("Get finance_facade_live_package_summary: %v", err)
			}
			want := "finance_facade_live_package modules=3 providers=5 tables=2 functions=9 policies=3 errors=8 provider_free=true live_network=false imports=false fixtures=13 edge_matrix=8"
			if got != want {
				t.Fatalf("finance_facade_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

type financeFacadeProvenance struct {
	Provider          string `json:"provider"`
	FixtureKey        string `json:"fixture_key"`
	CapturedAt        string `json:"captured_at"`
	SourceSchema      string `json:"source_schema"`
	SourceURLRedacted bool   `json:"source_url_redacted"`
	StaleAfterDays    int    `json:"stale_after_days"`
	ReplayReady       bool   `json:"replay_ready"`
}

type financeFacadeFunctionFixture struct {
	SchemaVersion         int                           `json:"schema_version"`
	ProviderFree          bool                          `json:"provider_free"`
	LiveNetwork           bool                          `json:"live_network"`
	RealDependencyImports bool                          `json:"real_dependency_imports"`
	FunctionID            string                        `json:"function_id"`
	SourceModule          string                        `json:"source_module"`
	Provider              string                        `json:"provider"`
	Capability            string                        `json:"capability"`
	Symbol                string                        `json:"symbol"`
	Dataset               string                        `json:"dataset"`
	Input                 map[string]any                `json:"input"`
	Output                financeFacadeFunctionOutput   `json:"output"`
	Fallback              financeFacadeFunctionFallback `json:"fallback"`
	Provenance            financeFacadeProvenance       `json:"provenance"`
}

type financeFacadeFunctionOutput struct {
	Fields    map[string]any `json:"fields"`
	SourceRef string         `json:"source_ref"`
}

type financeFacadeFunctionFallback struct {
	Order                      []string `json:"order"`
	SelectedProvider           string   `json:"selected_provider"`
	Attempted                  []string `json:"attempted"`
	CleanSkipWithoutDependency bool     `json:"clean_skip_without_dependency"`
	ErrorEnvelopeFixture       string   `json:"error_envelope_fixture"`
}

type financeFacadeProviderEdgeContract struct {
	ProviderFree               bool                            `json:"provider_free"`
	LiveNetwork                bool                            `json:"live_network"`
	RealDependencyImports      bool                            `json:"real_dependency_imports"`
	CleanSkipWithoutDependency bool                            `json:"clean_skip_without_dependency"`
	Fixture                    string                          `json:"fixture"`
	Schema                     string                          `json:"schema"`
	DialectCapabilities        []string                        `json:"dialect_capabilities"`
	EdgeCases                  []financeFacadeProviderEdgeCase `json:"edge_cases"`
}

type financeFacadeProviderEdgeCase struct {
	ID                          string `json:"id"`
	Category                    string `json:"category"`
	TriggerFixtureKey           string `json:"trigger_fixture_key"`
	ExpectedErrorCode           string `json:"expected_error_code"`
	Retryable                   bool   `json:"retryable"`
	CacheState                  string `json:"cache_state"`
	FallbackRequired            bool   `json:"fallback_required"`
	FallbackExplanationRequired bool   `json:"fallback_explanation_required"`
	Terminal                    bool   `json:"terminal"`
}

type financeFacadeProviderEdgeFixture struct {
	ProviderFree          bool                           `json:"provider_free"`
	LiveNetwork           bool                           `json:"live_network"`
	RealDependencyImports bool                           `json:"real_dependency_imports"`
	Dialect               string                         `json:"dialect"`
	Providers             []financeFacadeEdgeProvider    `json:"providers"`
	EdgeRows              []financeFacadeProviderEdgeRow `json:"edge_rows"`
}

type financeFacadeEdgeProvider struct {
	ProviderID   string                     `json:"provider_id"`
	Capabilities []string                   `json:"capabilities"`
	Terms        financeFacadeProviderTerms `json:"terms"`
}

type financeFacadeProviderTerms struct {
	TermsRef            string   `json:"terms_ref"`
	AllowedUses         []string `json:"allowed_uses"`
	AttributionRequired bool     `json:"attribution_required"`
	Redistribution      string   `json:"redistribution"`
	LiveNetwork         bool     `json:"live_network"`
	CredentialRequired  bool     `json:"credential_required"`
}

type financeFacadeProviderEdgeRow struct {
	CaseID     string `json:"case_id"`
	Category   string `json:"category"`
	ProviderID string `json:"provider_id"`
	FixtureKey string `json:"fixture_key"`
	Operation  string `json:"operation"`
	CacheState string `json:"cache_state"`
	Retry      struct {
		Attempts  int    `json:"attempts"`
		Retryable bool   `json:"retryable"`
		Backoff   string `json:"backoff"`
	} `json:"retry"`
	ErrorEnvelope struct {
		OK                bool   `json:"ok"`
		ErrorCode         string `json:"error_code"`
		RetryAfterSeconds int    `json:"retry_after_seconds"`
	} `json:"error_envelope"`
	Fallback struct {
		Required         bool     `json:"required"`
		Attempted        []string `json:"attempted"`
		SelectedProvider string   `json:"selected_provider"`
		Explanation      string   `json:"explanation"`
	} `json:"fallback"`
	Provenance struct {
		FixtureKey        string `json:"fixture_key"`
		SourceURLRedacted bool   `json:"source_url_redacted"`
		ReplayReady       bool   `json:"replay_ready"`
	} `json:"provenance"`
}

func assertFinanceFacadeFunctionMatrix(t *testing.T, base string, matrix []financeFacadeFunctionMatrix) {
	t.Helper()
	if len(matrix) != 9 {
		t.Fatalf("function matrix length = %d, want 9", len(matrix))
	}
	seen := map[string]bool{}
	for _, entry := range matrix {
		if entry.ID == "" || entry.SourceModule != "finrobot/data_source/market_data_api.py" || entry.Provider == "" || entry.Fixture == "" || entry.Schema == "" || len(entry.FallbackOrder) == 0 {
			t.Fatalf("function matrix entry incomplete: %#v", entry)
		}
		if seen[entry.ID] {
			t.Fatalf("duplicate function matrix id %q", entry.ID)
		}
		seen[entry.ID] = true
		if !strings.HasPrefix(entry.Capability, "finance.facade.") || !entry.ProviderFree || entry.LiveNetwork || entry.RealDependencyImports || !entry.CleanSkip {
			t.Fatalf("function matrix metadata invalid: %#v", entry)
		}
		if entry.Metadata["replay_ready"] != true || entry.Metadata["domain"] == "" {
			t.Fatalf("%s function matrix metadata incomplete: %#v", entry.ID, entry.Metadata)
		}
		assertFinanceFacadeJSONFile(t, filepath.Join(base, entry.Schema))
		assertFinanceFacadeJSONFile(t, filepath.Join(base, entry.Fixture))
	}
}

func assertFinanceFacadeProvenance(t *testing.T, provenance financeFacadeProvenance) {
	t.Helper()
	if provenance.Provider == "" || provenance.FixtureKey == "" || provenance.CapturedAt == "" || provenance.SourceSchema == "" || !provenance.SourceURLRedacted || provenance.StaleAfterDays < 0 || !provenance.ReplayReady {
		t.Fatalf("provenance incomplete: %#v", provenance)
	}
}

func financeFacadeLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "finance_facade")
}

func loadFinanceFacadeLiveManifest(t *testing.T, base string) financeFacadeLiveManifest {
	t.Helper()
	var manifest financeFacadeLiveManifest
	decodeFinanceFacadeJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertFinanceFacadeJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeFinanceFacadeJSONFile(t, path, &value)
}

func decodeFinanceFacadeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
