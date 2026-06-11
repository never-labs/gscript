package leia_test

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type valuationEngineLiveManifest struct {
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
	Entrypoints      map[string]string           `json:"entrypoints"`
	Schemas          map[string]string           `json:"schemas"`
	Fixtures         map[string]string           `json:"fixtures"`
	Modules          []valuationEngineModule     `json:"modules"`
	ValuationMethods []valuationEngineMethodMeta `json:"valuation_methods"`
	ToleranceGates   []valuationEngineGate       `json:"tolerance_gates"`
	TestGates        []string                    `json:"test_gates"`
}

type valuationEngineModule struct {
	ID            string   `json:"id"`
	SourceModule  string   `json:"source_module"`
	Capabilities  []string `json:"capabilities"`
	OutputSchemas []string `json:"output_schemas"`
}

type valuationEngineMethodMeta struct {
	ID           string `json:"id"`
	Capability   string `json:"capability"`
	Schema       string `json:"schema"`
	FixtureKey   string `json:"fixture_key"`
	Currency     string `json:"currency"`
	Period       string `json:"period"`
	ProviderFree bool   `json:"provider_free"`
}

type valuationEngineGate struct {
	ID                string  `json:"id"`
	Metric            string  `json:"metric"`
	Expected          float64 `json:"expected"`
	Actual            float64 `json:"actual"`
	AbsoluteTolerance float64 `json:"absolute_tolerance"`
	Status            string  `json:"status"`
	Period            string  `json:"period"`
}

type valuationEngineProvenance struct {
	Provider          string `json:"provider"`
	FixtureKey        string `json:"fixture_key"`
	CapturedAt        string `json:"captured_at"`
	SourceSchema      string `json:"source_schema"`
	SourceURLRedacted bool   `json:"source_url_redacted"`
	StaleAfterDays    int    `json:"stale_after_days"`
	ReplayReady       bool   `json:"replay_ready"`
}

func TestFinRobotValuationEngineLivePackageManifest(t *testing.T) {
	base := valuationEngineLivePackageDir(t)
	manifest := loadValuationEngineLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-valuation-engine-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-valuation-engine" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	if !strings.Contains(manifest.Credentials.Policy, "market data") || !strings.Contains(manifest.Credentials.Policy, "peer multiple") {
		t.Fatalf("credential policy should name future external boundaries: %q", manifest.Credentials.Policy)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_valuation_engine_live_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}

	wantSources := []string{
		"finrobot/valuation/dcf.py",
		"finrobot/valuation/multiples.py",
		"finrobot/valuation/target_price.py",
	}
	if !reflect.DeepEqual(manifest.SourceModules, wantSources) {
		t.Fatalf("source modules = %#v, want %#v", manifest.SourceModules, wantSources)
	}
	for _, key := range []string{"smoke", "valuation_engine_contract", "assumption_audit_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"valuation_output", "football_field", "assumption_audit", "tolerance_gate"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertValuationEngineJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "valuation_output", "football_field", "assumption_audit", "tolerance_gates"} {
		path := manifest.Fixtures[key]
		if path == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertValuationEngineJSONFile(t, filepath.Join(base, path))
	}

	var moduleIDs []string
	for _, module := range manifest.Modules {
		moduleIDs = append(moduleIDs, module.ID)
		if module.ID == "" || module.SourceModule == "" || len(module.Capabilities) < 5 || len(module.OutputSchemas) == 0 {
			t.Fatalf("module metadata incomplete: %#v", module)
		}
	}
	sort.Strings(moduleIDs)
	wantModuleIDs := []string{"dcf", "multiples", "target_price_synthesis"}
	if !reflect.DeepEqual(moduleIDs, wantModuleIDs) {
		t.Fatalf("module ids = %#v, want %#v", moduleIDs, wantModuleIDs)
	}

	var methodIDs []string
	for _, method := range manifest.ValuationMethods {
		methodIDs = append(methodIDs, method.ID)
		if method.Capability == "" || method.Schema != "valuation_output" || method.FixtureKey == "" || method.Currency != "USD" || method.Period == "" || !method.ProviderFree {
			t.Fatalf("method metadata incomplete: %#v", method)
		}
	}
	sort.Strings(methodIDs)
	if !reflect.DeepEqual(methodIDs, []string{"dcf", "ev_ebitda", "pe"}) {
		t.Fatalf("method ids = %#v", methodIDs)
	}

	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"dcf", "ev/ebitda", "p/e", "target price", "football-field", "assumption audit", "tolerance", "hidden data fetch"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotValuationEngineContractsAndFixtureIndex(t *testing.T) {
	base := valuationEngineLivePackageDir(t)

	var contract struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Modules               []struct {
			ID             string   `json:"id"`
			SourceModule   string   `json:"source_module"`
			RequiredFields []string `json:"required_fields"`
		} `json:"modules"`
		MethodOutputs []struct {
			ID              string   `json:"id"`
			Schema          string   `json:"schema"`
			Fixture         string   `json:"fixture"`
			PrimaryKey      []string `json:"primary_key"`
			RequiredMetrics []string `json:"required_metrics"`
		} `json:"method_outputs"`
		ProvenanceContract struct {
			RequiredFields  []string `json:"required_fields"`
			RedactSourceURL bool     `json:"redact_source_url"`
			LiveNetwork     bool     `json:"live_network"`
		} `json:"provenance_contract"`
		AcceptanceGates []string `json:"acceptance_gates"`
	}
	decodeValuationEngineJSONFile(t, filepath.Join(base, "contracts", "valuation_engine_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || len(contract.Modules) != 3 || len(contract.MethodOutputs) != 2 {
		t.Fatalf("contract header/modules/outputs = %#v", contract)
	}
	for _, output := range contract.MethodOutputs {
		if output.ID == "" || output.Schema == "" || output.Fixture == "" || len(output.PrimaryKey) == 0 || len(output.RequiredMetrics) < 3 {
			t.Fatalf("method output contract incomplete: %#v", output)
		}
		assertValuationEngineJSONFile(t, filepath.Join(base, output.Schema))
		assertValuationEngineJSONFile(t, filepath.Join(base, output.Fixture))
	}
	if !contract.ProvenanceContract.RedactSourceURL || contract.ProvenanceContract.LiveNetwork || len(contract.ProvenanceContract.RequiredFields) < 7 {
		t.Fatalf("provenance contract incomplete: %#v", contract.ProvenanceContract)
	}
	acceptance := strings.ToLower(strings.Join(contract.AcceptanceGates, " "))
	for _, want := range []string{"typed envelopes", "dcf math", "ev/ebitda", "p/e", "target price", "football-field", "hidden live data"} {
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
	decodeValuationEngineJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 5 {
		t.Fatalf("fixture index header/count = %#v", index)
	}
	fixtureKeys := map[string]bool{}
	for _, fixture := range index.Fixtures {
		fixtureKeys[fixture.FixtureKey] = true
		if fixture.FixtureKey == "" || fixture.Capability == "" || fixture.Path == "" || fixture.Schema == "" {
			t.Fatalf("fixture metadata incomplete: %#v", fixture)
		}
		if fixture.Metadata["replay_ready"] != true || fixture.Metadata["deterministic_math"] != true || fixture.Metadata["hidden_live_data_fetch"] != false {
			t.Fatalf("%s metadata = %#v", fixture.FixtureKey, fixture.Metadata)
		}
		assertValuationEngineJSONFile(t, filepath.Join(base, fixture.Path))
		assertValuationEngineJSONFile(t, filepath.Join(base, fixture.Schema))
	}
	if !fixtureKeys["valuation:ACME:scenario_book:offline"] {
		t.Fatalf("fixture index missing scenario book: %#v", fixtureKeys)
	}
}

func TestFinRobotValuationEngineDeterministicFixtureMath(t *testing.T) {
	base := valuationEngineLivePackageDir(t)

	var fixture struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		Symbol       string `json:"symbol"`
		Currency     string `json:"currency"`
		Methods      []struct {
			Method       string             `json:"method"`
			Period       string             `json:"period"`
			PeerSet      []string           `json:"peer_set"`
			Inputs       map[string]any     `json:"inputs"`
			Outputs      map[string]float64 `json:"outputs"`
			MathMetadata struct {
				Formula         string `json:"formula"`
				InputHash       string `json:"input_hash"`
				Deterministic   bool   `json:"deterministic"`
				RoundedDecimals int    `json:"rounded_decimals"`
			} `json:"math_metadata"`
			ProvenanceRef string `json:"provenance_ref"`
		} `json:"methods"`
		TargetPriceSynthesis struct {
			Period      string             `json:"period"`
			Weights     map[string]float64 `json:"weights"`
			Formula     string             `json:"formula"`
			TargetPrice float64            `json:"target_price"`
		} `json:"target_price_synthesis"`
		Provenance valuationEngineProvenance `json:"provenance"`
	}
	decodeValuationEngineJSONFile(t, filepath.Join(base, "fixtures", "valuation_output_ACME_fixture.json"), &fixture)
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.Symbol != "ACME" || fixture.Currency != "USD" || len(fixture.Methods) != 3 {
		t.Fatalf("valuation fixture header/count = %#v", fixture)
	}
	assertValuationEngineProvenance(t, fixture.Provenance)

	methods := map[string]struct {
		Period  string
		PeerSet []string
		Inputs  map[string]any
		Outputs map[string]float64
	}{}
	for _, method := range fixture.Methods {
		if method.Method == "" || method.Period == "" || method.ProvenanceRef == "" || !method.MathMetadata.Deterministic || method.MathMetadata.InputHash == "" || method.MathMetadata.Formula == "" {
			t.Fatalf("method metadata incomplete: %#v", method)
		}
		methods[method.Method] = struct {
			Period  string
			PeerSet []string
			Inputs  map[string]any
			Outputs map[string]float64
		}{method.Period, method.PeerSet, method.Inputs, method.Outputs}
	}

	dcf := methods["dcf"]
	fcfs := floatsFromAnySlice(t, dcf.Inputs["projected_fcf"])
	discountRate := floatFromAny(t, dcf.Inputs["discount_rate"])
	terminalGrowth := floatFromAny(t, dcf.Inputs["terminal_growth_rate"])
	cash := floatFromAny(t, dcf.Inputs["cash"])
	debt := floatFromAny(t, dcf.Inputs["debt"])
	shares := floatFromAny(t, dcf.Inputs["diluted_shares"])
	terminalValue := fcfs[len(fcfs)-1] * (1 + terminalGrowth) / (discountRate - terminalGrowth)
	enterpriseValue := terminalValue / math.Pow(1+discountRate, float64(len(fcfs)))
	for i, fcf := range fcfs {
		enterpriseValue += fcf / math.Pow(1+discountRate, float64(i+1))
	}
	equityValue := enterpriseValue + cash - debt
	assertFloatWithin(t, "dcf terminal value", terminalValue, dcf.Outputs["terminal_value"], 0.000001)
	assertFloatWithin(t, "dcf enterprise value", enterpriseValue, dcf.Outputs["enterprise_value"], 0.000001)
	assertFloatWithin(t, "dcf price per share", equityValue/shares, dcf.Outputs["price_per_share"], 0.000001)

	ev := methods["ev_ebitda"]
	if ev.Period != "FY2026E" || len(ev.PeerSet) != 3 {
		t.Fatalf("EV/EBITDA method period/peer_set incomplete: %#v", ev)
	}
	evPrice := (floatFromAny(t, ev.Inputs["ebitda"])*floatFromAny(t, ev.Inputs["selected_multiple"]) + floatFromAny(t, ev.Inputs["cash"]) - floatFromAny(t, ev.Inputs["debt"])) / floatFromAny(t, ev.Inputs["diluted_shares"])
	assertFloatWithin(t, "ev/ebitda price", evPrice, ev.Outputs["price_per_share"], 0.000001)

	pe := methods["pe"]
	if pe.Period != "FY2026E" || len(pe.PeerSet) != 3 {
		t.Fatalf("P/E method period/peer_set incomplete: %#v", pe)
	}
	pePrice := floatFromAny(t, pe.Inputs["eps"]) * floatFromAny(t, pe.Inputs["selected_multiple"])
	assertFloatWithin(t, "pe price", pePrice, pe.Outputs["price_per_share"], 0.000001)

	weights := fixture.TargetPriceSynthesis.Weights
	weightSum := weights["dcf"] + weights["ev_ebitda"] + weights["pe"]
	assertFloatWithin(t, "weight sum", weightSum, 1.0, 0.000001)
	targetPrice := dcf.Outputs["price_per_share"]*weights["dcf"] + ev.Outputs["price_per_share"]*weights["ev_ebitda"] + pe.Outputs["price_per_share"]*weights["pe"]
	assertFloatWithin(t, "target price synthesis", targetPrice, fixture.TargetPriceSynthesis.TargetPrice, 0.000001)
}

func TestFinRobotValuationEngineScenarioBookFixture(t *testing.T) {
	base := valuationEngineLivePackageDir(t)

	var book struct {
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		Symbol                string `json:"symbol"`
		Currency              string `json:"currency"`
		ScenarioDimensions    []struct {
			ID       string `json:"id"`
			Unit     string `json:"unit"`
			Base     any    `json:"base"`
			LowCase  any    `json:"low_case"`
			HighCase any    `json:"high_case"`
		} `json:"scenario_dimensions"`
		Scenarios []struct {
			ID            string             `json:"id"`
			Weight        float64            `json:"weight"`
			Assumptions   map[string]any     `json:"assumptions"`
			MethodTargets map[string]float64 `json:"method_targets"`
			TargetPrice   float64            `json:"target_price"`
			ProvenanceRef string             `json:"provenance_ref"`
		} `json:"scenarios"`
		ScenarioWeightedTarget struct {
			Formula        string  `json:"formula"`
			TargetPrice    float64 `json:"target_price"`
			WeightSum      float64 `json:"weight_sum"`
			RatingBand     string  `json:"rating_band"`
			ConflictPolicy string  `json:"conflict_policy"`
		} `json:"scenario_weighted_target"`
		ToleranceMetadata struct {
			AbsoluteTolerance float64           `json:"absolute_tolerance"`
			RoundedDecimals   int               `json:"rounded_decimals"`
			DeterministicMath bool              `json:"deterministic_math"`
			SensitivityUnits  map[string]string `json:"sensitivity_units"`
		} `json:"tolerance_metadata"`
		Provenance valuationEngineProvenance `json:"provenance"`
	}
	decodeValuationEngineJSONFile(t, filepath.Join(base, "fixtures", "scenario_book_ACME_fixture.json"), &book)
	if !book.ProviderFree || book.LiveNetwork || book.RealDependencyImports || book.Symbol != "ACME" || book.Currency != "USD" {
		t.Fatalf("scenario book header = %#v", book)
	}
	assertValuationEngineProvenance(t, book.Provenance)

	dimensions := map[string]string{}
	for _, dimension := range book.ScenarioDimensions {
		if dimension.ID == "" || dimension.Unit == "" || dimension.Base == nil || dimension.LowCase == nil || dimension.HighCase == nil {
			t.Fatalf("scenario dimension incomplete: %#v", dimension)
		}
		dimensions[dimension.ID] = dimension.Unit
	}
	for _, want := range []string{"wacc", "terminal_growth", "revenue_cagr", "ebitda_margin", "peer_outlier_handling", "analyst_target_conflict_reconciliation"} {
		if dimensions[want] == "" {
			t.Fatalf("scenario dimensions missing %q: %#v", want, dimensions)
		}
	}
	if dimensions["wacc"] != "ratio" || dimensions["terminal_growth"] != "ratio" || dimensions["revenue_cagr"] != "ratio" || dimensions["ebitda_margin"] != "ratio" {
		t.Fatalf("numeric sensitivity dimensions must be ratios: %#v", dimensions)
	}

	requiredAssumptions := []string{"wacc", "terminal_growth", "revenue_cagr", "ebitda_margin", "peer_outlier_handling", "analyst_target_conflict_reconciliation"}
	requiredMethods := []string{"dcf", "ev_ebitda", "pe", "analyst_reconciled"}
	var weightSum, weightedTarget float64
	for _, scenario := range book.Scenarios {
		if scenario.ID == "" || scenario.Weight <= 0 || scenario.ProvenanceRef == "" || scenario.TargetPrice <= 0 {
			t.Fatalf("scenario incomplete: %#v", scenario)
		}
		for _, assumption := range requiredAssumptions {
			if _, ok := scenario.Assumptions[assumption]; !ok {
				t.Fatalf("scenario %q missing assumption %q: %#v", scenario.ID, assumption, scenario.Assumptions)
			}
		}
		if _, ok := scenario.Assumptions["peer_outlier_handling"].(string); !ok {
			t.Fatalf("scenario %q peer outlier policy must be explicit: %#v", scenario.ID, scenario.Assumptions)
		}
		if _, ok := scenario.Assumptions["analyst_target_conflict_reconciliation"].(string); !ok {
			t.Fatalf("scenario %q analyst conflict policy must be explicit: %#v", scenario.ID, scenario.Assumptions)
		}
		for _, method := range requiredMethods {
			if scenario.MethodTargets[method] <= 0 {
				t.Fatalf("scenario %q missing positive method target %q: %#v", scenario.ID, method, scenario.MethodTargets)
			}
		}
		weightSum += scenario.Weight
		weightedTarget += scenario.TargetPrice * scenario.Weight
	}
	assertFloatWithin(t, "scenario weight sum", weightSum, book.ScenarioWeightedTarget.WeightSum, book.ToleranceMetadata.AbsoluteTolerance)
	assertFloatWithin(t, "scenario weight sum one", weightSum, 1.0, book.ToleranceMetadata.AbsoluteTolerance)
	assertFloatWithin(t, "scenario weighted target", weightedTarget, book.ScenarioWeightedTarget.TargetPrice, book.ToleranceMetadata.AbsoluteTolerance)
	if !strings.Contains(book.ScenarioWeightedTarget.Formula, "scenario.target_price") || !strings.Contains(book.ScenarioWeightedTarget.ConflictPolicy, "analyst") {
		t.Fatalf("scenario weighted target metadata incomplete: %#v", book.ScenarioWeightedTarget)
	}
	if book.ToleranceMetadata.AbsoluteTolerance <= 0 || book.ToleranceMetadata.RoundedDecimals != 6 || !book.ToleranceMetadata.DeterministicMath {
		t.Fatalf("tolerance metadata incomplete: %#v", book.ToleranceMetadata)
	}
	for _, sensitivity := range []string{"wacc", "terminal_growth", "revenue_cagr", "ebitda_margin"} {
		if book.ToleranceMetadata.SensitivityUnits[sensitivity] != "ratio" {
			t.Fatalf("tolerance metadata missing ratio unit for %q: %#v", sensitivity, book.ToleranceMetadata.SensitivityUnits)
		}
	}
}

func TestFinRobotValuationEngineFootballFieldAuditAndToleranceGates(t *testing.T) {
	base := valuationEngineLivePackageDir(t)

	var football struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		Symbol       string `json:"symbol"`
		Currency     string `json:"currency"`
		Ranges       []struct {
			Method        string  `json:"method"`
			Period        string  `json:"period"`
			Low           float64 `json:"low"`
			Base          float64 `json:"base"`
			High          float64 `json:"high"`
			ProvenanceRef string  `json:"provenance_ref"`
		} `json:"ranges"`
		Provenance valuationEngineProvenance `json:"provenance"`
	}
	decodeValuationEngineJSONFile(t, filepath.Join(base, "fixtures", "football_field_ACME_fixture.json"), &football)
	if !football.ProviderFree || football.LiveNetwork || football.Symbol != "ACME" || football.Currency != "USD" || len(football.Ranges) != 4 {
		t.Fatalf("football field fixture incomplete: %#v", football)
	}
	assertValuationEngineProvenance(t, football.Provenance)
	for _, r := range football.Ranges {
		if r.Method == "" || r.Period == "" || r.ProvenanceRef == "" || !(r.Low < r.Base && r.Base < r.High) {
			t.Fatalf("football-field range invalid: %#v", r)
		}
	}

	var audit struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		Symbol       string `json:"symbol"`
		Currency     string `json:"currency"`
		Period       string `json:"period"`
		Assumptions  []struct {
			ID        string  `json:"id"`
			Value     float64 `json:"value"`
			Unit      string  `json:"unit"`
			SourceRef string  `json:"source_ref"`
		} `json:"assumptions"`
		AuditResults []struct {
			Rule     string  `json:"rule"`
			Status   string  `json:"status"`
			Observed float64 `json:"observed"`
			Message  string  `json:"message"`
		} `json:"audit_results"`
		Provenance valuationEngineProvenance `json:"provenance"`
	}
	decodeValuationEngineJSONFile(t, filepath.Join(base, "fixtures", "assumption_audit_ACME_fixture.json"), &audit)
	if !audit.ProviderFree || audit.LiveNetwork || audit.Currency != "USD" || audit.Period != "FY2026E" || len(audit.Assumptions) < 5 || len(audit.AuditResults) < 4 {
		t.Fatalf("assumption audit fixture incomplete: %#v", audit)
	}
	assertValuationEngineProvenance(t, audit.Provenance)
	for _, result := range audit.AuditResults {
		if result.Rule == "" || result.Status != "pass" || result.Message == "" {
			t.Fatalf("audit result must be explicit and passing in skeleton fixture: %#v", result)
		}
	}

	var gates struct {
		ProviderFree bool                      `json:"provider_free"`
		LiveNetwork  bool                      `json:"live_network"`
		Symbol       string                    `json:"symbol"`
		Currency     string                    `json:"currency"`
		Gates        []valuationEngineGate     `json:"gates"`
		Provenance   valuationEngineProvenance `json:"provenance"`
	}
	decodeValuationEngineJSONFile(t, filepath.Join(base, "fixtures", "tolerance_gates_ACME_fixture.json"), &gates)
	if !gates.ProviderFree || gates.LiveNetwork || gates.Symbol != "ACME" || gates.Currency != "USD" || len(gates.Gates) != 5 {
		t.Fatalf("tolerance gates fixture incomplete: %#v", gates)
	}
	assertValuationEngineProvenance(t, gates.Provenance)
	for _, gate := range gates.Gates {
		if gate.ID == "" || gate.Metric == "" || gate.Period == "" || gate.Status != "pass" || gate.AbsoluteTolerance <= 0 {
			t.Fatalf("tolerance gate incomplete: %#v", gate)
		}
		assertFloatWithin(t, gate.ID, gate.Actual, gate.Expected, gate.AbsoluteTolerance)
	}
}

func TestFinRobotValuationEngineLivePackageNoLiveImportsOrFetches(t *testing.T) {
	base := valuationEngineLivePackageDir(t)
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s*import\s+`),
		regexp.MustCompile(`(?m)^\s*use\s+`),
		regexp.MustCompile(`(?m)^\s*load\s*\(`),
		regexp.MustCompile(`(?m)^\s*require\s*\(`),
		regexp.MustCompile(`(?m)^\s*(yfinance|finnhub|openbb|requests|http|pandas|numpy|q|runtime)\s*[.(]`),
		regexp.MustCompile(`(?i)"hidden_live_data_fetch"\s*:\s*true`),
		regexp.MustCompile(`(?i)"live_network"\s*:\s*true`),
		regexp.MustCompile(`(?i)(fetch_live|network_call|provider_secret|api_key)`),
	}
	if err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		source := string(data)
		for _, pattern := range patterns {
			if pattern.FindString(source) != "" {
				t.Fatalf("%s contains live dependency or hidden fetch marker matching %q", path, pattern.String())
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestFinRobotValuationEngineLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(valuationEngineLivePackageDir(t), "main.leia")

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
			got, err := vm.Get("valuation_engine_live_package_summary")
			if err != nil {
				t.Fatalf("Get valuation_engine_live_package_summary: %v", err)
			}
			want := "valuation_engine_live_package methods=3 synthesis=1 football_field=1 audits=1 gates=5 provider_free=true live_network=false imports=false fixtures=5"
			if got != want {
				t.Fatalf("valuation_engine_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func valuationEngineLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "valuation_engine")
}

func loadValuationEngineLiveManifest(t *testing.T, base string) valuationEngineLiveManifest {
	t.Helper()
	var manifest valuationEngineLiveManifest
	decodeValuationEngineJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertValuationEngineProvenance(t *testing.T, provenance valuationEngineProvenance) {
	t.Helper()
	if provenance.Provider == "" || provenance.FixtureKey == "" || provenance.CapturedAt == "" || provenance.SourceSchema == "" || !provenance.SourceURLRedacted || provenance.StaleAfterDays < 0 || !provenance.ReplayReady {
		t.Fatalf("provenance incomplete: %#v", provenance)
	}
}

func assertValuationEngineJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeValuationEngineJSONFile(t, path, &value)
}

func decodeValuationEngineJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func assertFloatWithin(t *testing.T, name string, actual, expected, tolerance float64) {
	t.Helper()
	if math.Abs(actual-expected) > tolerance {
		t.Fatalf("%s = %.9f, want %.9f within %.9f", name, actual, expected, tolerance)
	}
}

func floatFromAny(t *testing.T, value any) float64 {
	t.Helper()
	f, ok := value.(float64)
	if !ok {
		t.Fatalf("value %#v has type %T, want float64", value, value)
	}
	return f
}

func floatsFromAnySlice(t *testing.T, value any) []float64 {
	t.Helper()
	raw, ok := value.([]any)
	if !ok {
		t.Fatalf("value %#v has type %T, want []any", value, value)
	}
	out := make([]float64, len(raw))
	for i, value := range raw {
		out[i] = floatFromAny(t, value)
	}
	return out
}
