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
	}
	if !reflect.DeepEqual(manifest.SourceModules, wantSources) {
		t.Fatalf("source modules = %#v, want %#v", manifest.SourceModules, wantSources)
	}
	for _, key := range []string{"smoke", "finance_facade_contract", "provider_fallback_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"market_data_table", "fundamental_table", "provider_fallback", "error_envelope"} {
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
	wantModuleIDs := []string{"finance_data", "market_data"}
	if !reflect.DeepEqual(moduleIDs, wantModuleIDs) {
		t.Fatalf("module ids = %#v, want %#v", moduleIDs, wantModuleIDs)
	}

	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"fallback", "typed table", "cache", "retry", "rate-limit", "provenance", "error envelope"} {
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
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || len(contract.Modules) != 2 || len(contract.TypedTables) != 2 {
		t.Fatalf("contract header/modules/tables = %#v", contract)
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
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 3 {
		t.Fatalf("fixture index header/count = %#v", index)
	}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Capability == "" || fixture.Path == "" || fixture.Schema == "" {
			t.Fatalf("fixture metadata incomplete: %#v", fixture)
		}
		if fixture.Metadata["replay_ready"] != true {
			t.Fatalf("%s replay_ready = %#v", fixture.FixtureKey, fixture.Metadata["replay_ready"])
		}
		assertFinanceFacadeJSONFile(t, filepath.Join(base, fixture.Path))
		assertFinanceFacadeJSONFile(t, filepath.Join(base, fixture.Schema))
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
			want := "finance_facade_live_package modules=2 providers=5 tables=2 policies=3 errors=1 provider_free=true live_network=false imports=false fixtures=3"
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
