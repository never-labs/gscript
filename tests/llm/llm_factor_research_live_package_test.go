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

type factorResearchLiveManifest struct {
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
	Entrypoints            map[string]string               `json:"entrypoints"`
	Schemas                map[string]string               `json:"schemas"`
	Fixtures               map[string]string               `json:"fixtures"`
	Modules                []factorResearchModule          `json:"modules"`
	FactorTransforms       []factorResearchTransform       `json:"factor_transforms"`
	AgentHandoffBoundaries []factorResearchHandoffBoundary `json:"agent_handoff_boundaries"`
	TestGates              []string                        `json:"test_gates"`
}

type factorResearchModule struct {
	ID            string   `json:"id"`
	SourceModule  string   `json:"source_module"`
	Capabilities  []string `json:"capabilities"`
	OutputSchemas []string `json:"output_schemas"`
}

type factorResearchTransform struct {
	ID           string         `json:"id"`
	Capability   string         `json:"capability"`
	InputSchema  string         `json:"input_schema"`
	OutputSchema string         `json:"output_schema"`
	Parameters   map[string]any `json:"parameters"`
	ProviderFree bool           `json:"provider_free"`
}

type factorResearchHandoffBoundary struct {
	ID                 string `json:"id"`
	DisplayName        string `json:"display_name"`
	Capability         string `json:"capability"`
	FixtureKey         string `json:"fixture_key"`
	Schema             string `json:"schema"`
	LiveNetwork        bool   `json:"live_network"`
	DependencyImported bool   `json:"dependency_imported"`
	CredentialRequired bool   `json:"credential_required"`
	CleanSkip          bool   `json:"clean_skip"`
}

func TestFinRobotFactorResearchLivePackageManifest(t *testing.T) {
	base := factorResearchLivePackageDir(t)
	manifest := loadFactorResearchLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-factor-research-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-factor-research" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	if !strings.Contains(manifest.Credentials.Policy, "market data") || !strings.Contains(manifest.Credentials.Policy, "optimizer") {
		t.Fatalf("credential policy should name future external boundaries: %q", manifest.Credentials.Policy)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_factor_research_live_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}

	wantSources := []string{
		"experiments/multi_factor_agents.py",
		"finrobot_quantitative/factor_transforms.py",
		"finrobot_quantitative/portfolio_factor_exposure.py",
	}
	if !reflect.DeepEqual(manifest.SourceModules, wantSources) {
		t.Fatalf("source modules = %#v, want %#v", manifest.SourceModules, wantSources)
	}

	for _, key := range []string{"smoke", "factor_research_contract", "agent_handoff_boundary_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"market_data", "factor_data", "portfolio_factor_exposure", "agent_handoff_boundary"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertFactorResearchJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "market_data", "factor_data", "portfolio_exposure", "agent_handoff_boundary"} {
		path := manifest.Fixtures[key]
		if path == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertFactorResearchJSONFile(t, filepath.Join(base, path))
	}

	var ids []string
	for _, module := range manifest.Modules {
		ids = append(ids, module.ID)
		if module.ID == "" || module.SourceModule == "" || len(module.Capabilities) < 4 || len(module.OutputSchemas) == 0 {
			t.Fatalf("module metadata incomplete: %#v", module)
		}
	}
	sort.Strings(ids)
	wantIDs := []string{"factor_transforms", "multi_factor_agents", "portfolio_factor_exposure"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("module ids = %#v, want %#v", ids, wantIDs)
	}

	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"multi_factor_agents.py", "factor transform", "market data", "factor data", "portfolio factor exposure", "handoff"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotFactorResearchTransformsContractsAndFixtures(t *testing.T) {
	base := factorResearchLivePackageDir(t)
	manifest := loadFactorResearchLiveManifest(t, base)

	var transformIDs []string
	for _, transform := range manifest.FactorTransforms {
		transformIDs = append(transformIDs, transform.ID)
		if transform.Capability == "" || transform.InputSchema != "factor_data" || transform.OutputSchema != "factor_data" || len(transform.Parameters) == 0 || !transform.ProviderFree {
			t.Fatalf("transform metadata incomplete: %#v", transform)
		}
		if !strings.HasPrefix(transform.Capability, "finance.factor_research.transform.") {
			t.Fatalf("transform capability = %q", transform.Capability)
		}
	}
	sort.Strings(transformIDs)
	wantTransforms := []string{"composite_score", "sector_neutralize", "winsorize", "zscore"}
	if !reflect.DeepEqual(transformIDs, wantTransforms) {
		t.Fatalf("transform ids = %#v, want %#v", transformIDs, wantTransforms)
	}

	var contract struct {
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		SourceModule          string `json:"source_module"`
		Modules               []struct {
			ID             string   `json:"id"`
			SourceModule   string   `json:"source_module"`
			RequiredFields []string `json:"required_fields"`
		} `json:"modules"`
		FieldContracts  map[string]any `json:"field_contracts"`
		AcceptanceGates []string       `json:"acceptance_gates"`
	}
	decodeFactorResearchJSONFile(t, filepath.Join(base, "contracts", "factor_research_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || contract.SourceModule != "experiments/multi_factor_agents.py" || len(contract.Modules) != 3 {
		t.Fatalf("contract header/modules = %#v", contract)
	}
	for _, field := range []string{"market_data", "factor_data", "portfolio_factor_exposure", "agent_handoff_boundary"} {
		if contract.FieldContracts[field] == nil {
			t.Fatalf("missing field contract %q", field)
		}
	}
	acceptance := strings.ToLower(strings.Join(contract.AcceptanceGates, " "))
	for _, want := range []string{"multi-factor", "factor transforms", "market data", "portfolio exposure", "clean-skip"} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance gates missing %q: %s", want, acceptance)
		}
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
	decodeFactorResearchJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 4 {
		t.Fatalf("fixture index header/count = %#v", index)
	}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Capability == "" || fixture.Path == "" || fixture.Schema == "" {
			t.Fatalf("fixture metadata incomplete: %#v", fixture)
		}
		if fixture.Metadata["replay_ready"] != true {
			t.Fatalf("%s replay_ready = %#v", fixture.FixtureKey, fixture.Metadata["replay_ready"])
		}
		assertFactorResearchJSONFile(t, filepath.Join(base, fixture.Path))
		assertFactorResearchJSONFile(t, filepath.Join(base, fixture.Schema))
	}
}

func TestFinRobotFactorResearchPortfolioExposureAndHandoffBoundaries(t *testing.T) {
	base := factorResearchLivePackageDir(t)
	manifest := loadFactorResearchLiveManifest(t, base)

	var ids []string
	fixtures := map[string]bool{}
	for _, boundary := range manifest.AgentHandoffBoundaries {
		ids = append(ids, boundary.ID)
		if boundary.ID == "" || boundary.DisplayName == "" || boundary.Capability == "" || boundary.FixtureKey == "" || boundary.Schema == "" {
			t.Fatalf("handoff boundary metadata incomplete: %#v", boundary)
		}
		if boundary.LiveNetwork || boundary.DependencyImported || boundary.CredentialRequired || !boundary.CleanSkip {
			t.Fatalf("handoff boundary must be fixture-only and clean-skip safe: %#v", boundary)
		}
		if fixtures[boundary.FixtureKey] {
			t.Fatalf("duplicate fixture key %q", boundary.FixtureKey)
		}
		fixtures[boundary.FixtureKey] = true
	}
	sort.Strings(ids)
	wantIDs := []string{"factor_data_provider", "market_data_provider", "portfolio_optimizer"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("handoff ids = %#v, want %#v", ids, wantIDs)
	}

	var boundaryContract struct {
		ProviderFree               bool                            `json:"provider_free"`
		LiveNetwork                bool                            `json:"live_network"`
		RealDependencyImports      bool                            `json:"real_dependency_imports"`
		CleanSkipWithoutDependency bool                            `json:"clean_skip_without_dependency"`
		Boundaries                 []factorResearchHandoffBoundary `json:"boundaries"`
	}
	decodeFactorResearchJSONFile(t, filepath.Join(base, "contracts", "agent_handoff_boundary_contract.json"), &boundaryContract)
	if !boundaryContract.ProviderFree || boundaryContract.LiveNetwork || boundaryContract.RealDependencyImports || !boundaryContract.CleanSkipWithoutDependency || len(boundaryContract.Boundaries) != 3 {
		t.Fatalf("boundary contract = %#v", boundaryContract)
	}

	var exposureFixture struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		PortfolioID  string `json:"portfolio_id"`
		Holdings     []struct {
			Symbol string  `json:"symbol"`
			Weight float64 `json:"weight"`
		} `json:"holdings"`
		FactorExposures map[string]float64 `json:"factor_exposures"`
		SectorExposures []struct {
			Sector string  `json:"sector"`
			Weight float64 `json:"weight"`
		} `json:"sector_exposures"`
		RiskLimits []struct {
			ID       string  `json:"id"`
			Limit    float64 `json:"limit"`
			Observed float64 `json:"observed"`
			Status   string  `json:"status"`
		} `json:"risk_limits"`
		SourceRefs []string `json:"source_refs"`
	}
	decodeFactorResearchJSONFile(t, filepath.Join(base, "fixtures", "portfolio_factor_exposure_fixture.json"), &exposureFixture)
	if !exposureFixture.ProviderFree || exposureFixture.LiveNetwork || exposureFixture.PortfolioID == "" || len(exposureFixture.Holdings) != 3 || len(exposureFixture.RiskLimits) < 3 || len(exposureFixture.SourceRefs) < 2 {
		t.Fatalf("portfolio exposure fixture incomplete: %#v", exposureFixture)
	}
	for _, want := range []string{"value", "growth", "momentum", "quality", "volatility", "liquidity", "sentiment", "macro", "composite_score"} {
		if _, ok := exposureFixture.FactorExposures[want]; !ok {
			t.Fatalf("portfolio exposure missing factor %q: %#v", want, exposureFixture.FactorExposures)
		}
	}
	for _, limit := range exposureFixture.RiskLimits {
		if limit.ID == "" || limit.Status != "pass" {
			t.Fatalf("risk limit must be explicit and passing in skeleton fixture: %#v", limit)
		}
	}
}

func TestFinRobotFactorResearchFixtureShape(t *testing.T) {
	base := factorResearchLivePackageDir(t)

	var marketFixture struct {
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
		Rows         []struct {
			Symbol    string  `json:"symbol"`
			AsOf      string  `json:"as_of"`
			Close     float64 `json:"close"`
			Volume    int     `json:"volume"`
			Currency  string  `json:"currency"`
			SourceRef string  `json:"source_ref"`
		} `json:"rows"`
	}
	decodeFactorResearchJSONFile(t, filepath.Join(base, "fixtures", "market_data_ACME_universe_fixture.json"), &marketFixture)
	if !marketFixture.ProviderFree || marketFixture.LiveNetwork || len(marketFixture.Rows) != 5 {
		t.Fatalf("market fixture header/count = %#v", marketFixture)
	}
	for _, row := range marketFixture.Rows {
		if row.Symbol == "" || row.AsOf == "" || row.Close <= 0 || row.Volume <= 0 || row.Currency != "USD" || row.SourceRef == "" {
			t.Fatalf("market row incomplete: %#v", row)
		}
	}

	var factorFixture struct {
		ProviderFree      bool     `json:"provider_free"`
		LiveNetwork       bool     `json:"live_network"`
		TransformsApplied []string `json:"transforms_applied"`
		Rows              []struct {
			Symbol         string             `json:"symbol"`
			Sector         string             `json:"sector"`
			Factors        map[string]float64 `json:"factors"`
			RiskPenalty    float64            `json:"risk_penalty"`
			CompositeScore float64            `json:"composite_score"`
			SourceRef      string             `json:"source_ref"`
		} `json:"rows"`
	}
	decodeFactorResearchJSONFile(t, filepath.Join(base, "fixtures", "factor_data_ACME_universe_fixture.json"), &factorFixture)
	if !factorFixture.ProviderFree || factorFixture.LiveNetwork || len(factorFixture.Rows) != 5 || len(factorFixture.TransformsApplied) != 4 {
		t.Fatalf("factor fixture header/count = %#v", factorFixture)
	}
	for _, row := range factorFixture.Rows {
		if row.Symbol == "" || row.Sector == "" || row.SourceRef == "" || row.CompositeScore <= 0 || row.RiskPenalty < 0 {
			t.Fatalf("factor row incomplete: %#v", row)
		}
		for _, want := range []string{"value", "growth", "momentum", "quality", "volatility", "liquidity", "sentiment", "macro"} {
			if _, ok := row.Factors[want]; !ok {
				t.Fatalf("%s missing factor %q: %#v", row.Symbol, want, row.Factors)
			}
		}
	}
}

func TestFinRobotFactorResearchLivePackageNoLiveImports(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(factorResearchLivePackageDir(t), "main.leia"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pattern := range []string{
		`(?m)^\s*import\s+`,
		`(?m)^\s*use\s+`,
		`(?m)^\s*load\s*\(`,
		`(?m)^\s*require\s*\(`,
		`(?m)^\s*(yfinance|pandas|numpy|requests|http|openbb|cvxpy|sklearn)\s*[.(]`,
	} {
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("main.leia contains live dependency loader matching %q", pattern)
		}
	}
}

func TestFinRobotFactorResearchLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(factorResearchLivePackageDir(t), "main.leia")

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
			got, err := vm.Get("factor_research_live_package_summary")
			if err != nil {
				t.Fatalf("Get factor_research_live_package_summary: %v", err)
			}
			want := "factor_research_live_package experiments=1 transforms=4 exposures=3 handoffs=3 provider_free=true live_network=false imports=false fixtures=4"
			if got != want {
				t.Fatalf("factor_research_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func factorResearchLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "factor_research")
}

func loadFactorResearchLiveManifest(t *testing.T, base string) factorResearchLiveManifest {
	t.Helper()
	var manifest factorResearchLiveManifest
	decodeFactorResearchJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertFactorResearchJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeFactorResearchJSONFile(t, path, &value)
}

func decodeFactorResearchJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
