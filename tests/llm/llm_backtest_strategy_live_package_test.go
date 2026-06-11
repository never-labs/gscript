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

type backtestStrategyLiveManifest struct {
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
	Entrypoints          map[string]string            `json:"entrypoints"`
	Schemas              map[string]string            `json:"schemas"`
	Fixtures             map[string]string            `json:"fixtures"`
	Modules              []backtestStrategyModule     `json:"modules"`
	DeterministicReplay  backtestDeterministicReplay  `json:"deterministic_replay"`
	StrategyOutputs      []backtestStrategyOutput     `json:"strategy_outputs"`
	AnalyticsOutputs     []backtestStrategyOutput     `json:"analytics_outputs"`
	RiskLimits           []backtestRiskLimit          `json:"risk_limits"`
	OptionalDependencies []backtestOptionalDependency `json:"optional_dependencies"`
	TestGates            []string                     `json:"test_gates"`
}

type backtestStrategyModule struct {
	ID            string   `json:"id"`
	SourceModule  string   `json:"source_module"`
	Capabilities  []string `json:"capabilities"`
	OutputSchemas []string `json:"output_schemas"`
}

type backtestDeterministicReplay struct {
	Seed                         string  `json:"seed"`
	StartCash                    float64 `json:"start_cash"`
	CommissionBPS                float64 `json:"commission_bps"`
	SlippageBPS                  float64 `json:"slippage_bps"`
	DataFeedFixture              string  `json:"data_feed_fixture"`
	TradeLedgerFixture           string  `json:"trade_ledger_fixture"`
	PortfolioWeightsFixture      string  `json:"portfolio_weights_fixture"`
	ReturnsDrawdownSharpeFixture string  `json:"returns_drawdown_sharpe_fixture"`
	MetricsFixture               string  `json:"metrics_fixture"`
	DeterministicOrder           bool    `json:"deterministic_order"`
	ProviderFree                 bool    `json:"provider_free"`
	LiveNetwork                  bool    `json:"live_network"`
}

type backtestStrategyOutput struct {
	ID             string   `json:"id"`
	Capability     string   `json:"capability"`
	FixtureKey     string   `json:"fixture_key"`
	Schema         string   `json:"schema"`
	RequiredFields []string `json:"required_fields"`
	ProviderFree   bool     `json:"provider_free"`
	LiveNetwork    bool     `json:"live_network"`
}

type backtestRiskLimit struct {
	ID          string  `json:"id"`
	Metric      string  `json:"metric"`
	Limit       float64 `json:"limit"`
	Observed    float64 `json:"observed"`
	Status      string  `json:"status"`
	LiveNetwork bool    `json:"live_network"`
}

type backtestOptionalDependency struct {
	ID                         string `json:"id"`
	ImportName                 string `json:"import_name"`
	RequiredByDefault          bool   `json:"required_by_default"`
	RealDependencyImported     bool   `json:"real_dependency_imported"`
	CleanSkipWithoutDependency bool   `json:"clean_skip_without_dependency"`
}

func TestFinRobotBacktestStrategyLivePackageManifest(t *testing.T) {
	base := backtestStrategyLivePackageDir(t)
	manifest := loadBacktestStrategyLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-backtest-strategy-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-backtest-strategy" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	if !strings.Contains(manifest.Credentials.Policy, "Backtrader") || !strings.Contains(manifest.Credentials.Policy, "broker") {
		t.Fatalf("credential policy should name future external boundaries: %q", manifest.Credentials.Policy)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_backtest_strategy_live_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}

	for _, want := range []string{
		"FinRobot.BackTraderUtils.back_test",
		"backtrader.Strategy",
		"backtrader.Sizer",
		"backtrader.Indicator",
		"backtrader.Analyzer",
	} {
		if !contains(manifest.SourceModules, want) {
			t.Fatalf("source modules missing %q: %#v", want, manifest.SourceModules)
		}
	}

	for _, key := range []string{"smoke", "backtest_strategy_contract", "optional_dependency_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"strategy_manifest", "data_feed", "trade_ledger", "portfolio_weights", "returns_drawdown_sharpe", "metrics", "risk_limits"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertBacktestStrategyJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "strategy_manifest", "data_feed", "trade_ledger", "portfolio_weights", "returns_drawdown_sharpe", "metrics"} {
		path := manifest.Fixtures[key]
		if path == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertBacktestStrategyJSONFile(t, filepath.Join(base, path))
	}

	var moduleIDs []string
	for _, module := range manifest.Modules {
		moduleIDs = append(moduleIDs, module.ID)
		if module.ID == "" || module.SourceModule == "" || len(module.Capabilities) < 4 || len(module.OutputSchemas) == 0 {
			t.Fatalf("module metadata incomplete: %#v", module)
		}
	}
	sort.Strings(moduleIDs)
	wantModuleIDs := []string{"analyzer", "back_test", "indicator", "sizer", "strategy"}
	if !reflect.DeepEqual(moduleIDs, wantModuleIDs) {
		t.Fatalf("module ids = %#v, want %#v", moduleIDs, wantModuleIDs)
	}

	replay := manifest.DeterministicReplay
	if replay.Seed != "finrobot-backtest-strategy-offline-v1" ||
		replay.StartCash != 100000 ||
		replay.CommissionBPS != 5 ||
		replay.SlippageBPS != 2 ||
		replay.DataFeedFixture == "" ||
		replay.TradeLedgerFixture == "" ||
		replay.PortfolioWeightsFixture == "" ||
		replay.ReturnsDrawdownSharpeFixture == "" ||
		replay.MetricsFixture == "" ||
		!replay.DeterministicOrder ||
		!replay.ProviderFree ||
		replay.LiveNetwork {
		t.Fatalf("deterministic replay contract incomplete: %#v", replay)
	}

	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"backtraderutils.back_test", "strategy", "sizer", "indicator", "analyzer", "trade ledger", "portfolio weights", "returns/drawdown/sharpe", "metrics", "risk-limit", "clean-skip", "mplfinance", "openbb"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotBacktestStrategyContractsFixturesAndSchemas(t *testing.T) {
	base := backtestStrategyLivePackageDir(t)
	manifest := loadBacktestStrategyLiveManifest(t, base)

	if len(manifest.StrategyOutputs) != 1 {
		t.Fatalf("strategy outputs = %d, want 1", len(manifest.StrategyOutputs))
	}
	output := manifest.StrategyOutputs[0]
	if output.ID != "sma_cross_strategy" ||
		output.Capability != "finance.backtest.strategy.signal" ||
		output.FixtureKey == "" ||
		output.Schema != "strategy_manifest" ||
		!output.ProviderFree ||
		output.LiveNetwork {
		t.Fatalf("strategy output metadata incomplete: %#v", output)
	}
	for _, want := range []string{"strategy_id", "parameters", "indicator_bindings", "sizer_binding", "analyzer_bindings", "deterministic_seed"} {
		if !contains(output.RequiredFields, want) {
			t.Fatalf("strategy output required fields missing %q: %#v", want, output.RequiredFields)
		}
	}
	if len(manifest.AnalyticsOutputs) != 2 {
		t.Fatalf("analytics outputs = %d, want 2", len(manifest.AnalyticsOutputs))
	}
	analyticsByID := map[string]backtestStrategyOutput{}
	for _, output := range manifest.AnalyticsOutputs {
		analyticsByID[output.ID] = output
		if output.FixtureKey == "" || output.Schema == "" || !output.ProviderFree || output.LiveNetwork {
			t.Fatalf("analytics output metadata incomplete: %#v", output)
		}
	}
	for _, id := range []string{"portfolio_weights", "returns_drawdown_sharpe"} {
		if analyticsByID[id].ID == "" {
			t.Fatalf("missing analytics output %q: %#v", id, manifest.AnalyticsOutputs)
		}
	}

	var contract struct {
		ProviderFree               bool   `json:"provider_free"`
		LiveNetwork                bool   `json:"live_network"`
		RealDependencyImports      bool   `json:"real_dependency_imports"`
		CleanSkipWithoutDependency bool   `json:"clean_skip_without_dependency"`
		SourceModule               string `json:"source_module"`
		DeterministicSeed          string `json:"deterministic_seed"`
		Modules                    []struct {
			ID             string   `json:"id"`
			SourceModule   string   `json:"source_module"`
			RequiredFields []string `json:"required_fields"`
		} `json:"modules"`
		FieldContracts  map[string]any `json:"field_contracts"`
		AcceptanceGates []string       `json:"acceptance_gates"`
	}
	decodeBacktestStrategyJSONFile(t, filepath.Join(base, "contracts", "backtest_strategy_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || !contract.CleanSkipWithoutDependency ||
		contract.SourceModule != "FinRobot.BackTraderUtils.back_test" ||
		contract.DeterministicSeed != "finrobot-backtest-strategy-offline-v1" ||
		len(contract.Modules) != 5 {
		t.Fatalf("contract header/modules = %#v", contract)
	}
	for _, field := range []string{"strategy_manifest", "data_feed", "trade_ledger", "portfolio_weights", "returns_drawdown_sharpe", "metrics", "risk_limits"} {
		if contract.FieldContracts[field] == nil {
			t.Fatalf("missing field contract %q", field)
		}
	}
	acceptance := strings.ToLower(strings.Join(contract.AcceptanceGates, " "))
	for _, want := range []string{"backtraderutils.back_test", "strategy", "sizer", "indicator", "analyzer", "data feed", "trade ledger", "portfolio weights", "returns", "drawdown", "sharpe", "metrics", "risk limits", "clean-skip", "mplfinance", "openbb"} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance gates missing %q: %s", want, acceptance)
		}
	}

	var index struct {
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		DeterministicSeed     string `json:"deterministic_seed"`
		Fixtures              []struct {
			FixtureKey string         `json:"fixture_key"`
			Capability string         `json:"capability"`
			Path       string         `json:"path"`
			Schema     string         `json:"schema"`
			Metadata   map[string]any `json:"metadata"`
		} `json:"fixtures"`
	}
	decodeBacktestStrategyJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports ||
		index.DeterministicSeed != "finrobot-backtest-strategy-offline-v1" ||
		len(index.Fixtures) != len(manifest.Fixtures)-1 {
		t.Fatalf("fixture index header/count = %#v", index)
	}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Capability == "" || fixture.Path == "" || fixture.Schema == "" {
			t.Fatalf("fixture metadata incomplete: %#v", fixture)
		}
		if fixture.Metadata["replay_ready"] != true || fixture.Metadata["provider_free"] != true || fixture.Metadata["live_network"] != false {
			t.Fatalf("%s replay metadata = %#v", fixture.FixtureKey, fixture.Metadata)
		}
		assertBacktestStrategyJSONFile(t, filepath.Join(base, fixture.Path))
		assertBacktestStrategyJSONFile(t, filepath.Join(base, fixture.Schema))
	}
	fixtureKeys := map[string]bool{}
	for _, fixture := range index.Fixtures {
		fixtureKeys[fixture.FixtureKey] = true
	}
	for _, key := range []string{
		"strategy_manifest:sma_cross:offline:v1",
		"data_feed:ACME:daily:offline:v1",
		"trade_ledger:ACME:sma_cross:offline:v1",
		"portfolio_weights:ACME:sma_cross:offline:v1",
		"returns_drawdown_sharpe:ACME:sma_cross:offline:v1",
		"metrics:ACME:sma_cross:offline:v1",
	} {
		if !fixtureKeys[key] {
			t.Fatalf("fixture index missing %q: %#v", key, index.Fixtures)
		}
	}
}

func TestFinRobotBacktestStrategyFixtureShapes(t *testing.T) {
	base := backtestStrategyLivePackageDir(t)

	var strategyFixture struct {
		ProviderFree          bool           `json:"provider_free"`
		LiveNetwork           bool           `json:"live_network"`
		RealDependencyImports bool           `json:"real_dependency_imports"`
		StrategyID            string         `json:"strategy_id"`
		StrategyClass         string         `json:"strategy_class"`
		DeterministicSeed     string         `json:"deterministic_seed"`
		Parameters            map[string]any `json:"parameters"`
		IndicatorBindings     []struct {
			IndicatorID string `json:"indicator_id"`
			Lookback    int    `json:"lookback"`
			WarmupBars  int    `json:"warmup_bars"`
			OutputField string `json:"output_field"`
		} `json:"indicator_bindings"`
		SizerBinding struct {
			SizerID        string  `json:"sizer_id"`
			CashBufferPct  float64 `json:"cash_buffer_pct"`
			MaxPositionPct float64 `json:"max_position_pct"`
			LotSize        int     `json:"lot_size"`
		} `json:"sizer_binding"`
		AnalyzerBindings []struct {
			AnalyzerID   string   `json:"analyzer_id"`
			MetricKeys   []string `json:"metric_keys"`
			OutputSchema string   `json:"output_schema"`
		} `json:"analyzer_bindings"`
	}
	decodeBacktestStrategyJSONFile(t, filepath.Join(base, "fixtures", "strategy_manifest_fixture.json"), &strategyFixture)
	if !strategyFixture.ProviderFree || strategyFixture.LiveNetwork || strategyFixture.RealDependencyImports ||
		strategyFixture.StrategyID != "sma_cross_strategy" ||
		strategyFixture.StrategyClass == "" ||
		strategyFixture.DeterministicSeed != "finrobot-backtest-strategy-offline-v1" ||
		len(strategyFixture.Parameters) == 0 ||
		len(strategyFixture.IndicatorBindings) != 2 ||
		len(strategyFixture.AnalyzerBindings) != 2 {
		t.Fatalf("strategy fixture incomplete: %#v", strategyFixture)
	}
	if strategyFixture.SizerBinding.SizerID == "" || strategyFixture.SizerBinding.CashBufferPct <= 0 || strategyFixture.SizerBinding.MaxPositionPct <= 0 || strategyFixture.SizerBinding.LotSize <= 0 {
		t.Fatalf("sizer binding incomplete: %#v", strategyFixture.SizerBinding)
	}
	for _, indicator := range strategyFixture.IndicatorBindings {
		if indicator.IndicatorID == "" || indicator.Lookback <= 0 || indicator.WarmupBars < indicator.Lookback || indicator.OutputField == "" {
			t.Fatalf("indicator binding incomplete: %#v", indicator)
		}
	}
	for _, analyzer := range strategyFixture.AnalyzerBindings {
		if analyzer.AnalyzerID == "" || len(analyzer.MetricKeys) == 0 || analyzer.OutputSchema == "" {
			t.Fatalf("analyzer binding incomplete: %#v", analyzer)
		}
	}

	var feedFixture struct {
		ProviderFree      bool   `json:"provider_free"`
		LiveNetwork       bool   `json:"live_network"`
		FixtureKey        string `json:"fixture_key"`
		Symbol            string `json:"symbol"`
		Timeframe         string `json:"timeframe"`
		Timezone          string `json:"timezone"`
		Calendar          string `json:"calendar"`
		Currency          string `json:"currency"`
		SourceRef         string `json:"source_ref"`
		DeterministicSeed string `json:"deterministic_seed"`
		Rows              []struct {
			Timestamp string  `json:"timestamp"`
			Open      float64 `json:"open"`
			High      float64 `json:"high"`
			Low       float64 `json:"low"`
			Close     float64 `json:"close"`
			Volume    int     `json:"volume"`
		} `json:"rows"`
	}
	decodeBacktestStrategyJSONFile(t, filepath.Join(base, "fixtures", "data_feed_ACME_daily_fixture.json"), &feedFixture)
	if !feedFixture.ProviderFree || feedFixture.LiveNetwork ||
		feedFixture.FixtureKey != "data_feed:ACME:daily:offline:v1" ||
		feedFixture.Symbol != "ACME" ||
		feedFixture.Timeframe != "1d" ||
		feedFixture.Timezone != "UTC" ||
		feedFixture.Calendar != "XNYS" ||
		feedFixture.Currency != "USD" ||
		feedFixture.SourceRef == "" ||
		feedFixture.DeterministicSeed != "finrobot-backtest-strategy-offline-v1" ||
		len(feedFixture.Rows) < 8 {
		t.Fatalf("data feed fixture incomplete: %#v", feedFixture)
	}
	for _, row := range feedFixture.Rows {
		if row.Timestamp == "" || row.Open <= 0 || row.High < row.Low || row.Close <= 0 || row.Volume <= 0 {
			t.Fatalf("data feed row incomplete: %#v", row)
		}
	}

	var weightsFixture struct {
		ProviderFree      bool   `json:"provider_free"`
		LiveNetwork       bool   `json:"live_network"`
		FixtureKey        string `json:"fixture_key"`
		StrategyID        string `json:"strategy_id"`
		Symbol            string `json:"symbol"`
		DeterministicSeed string `json:"deterministic_seed"`
		SourceRef         string `json:"source_ref"`
		Rows              []struct {
			Timestamp      string  `json:"timestamp"`
			Cash           float64 `json:"cash"`
			CashWeight     float64 `json:"cash_weight"`
			Shares         float64 `json:"shares"`
			MarketValue    float64 `json:"market_value"`
			PositionWeight float64 `json:"position_weight"`
			GrossExposure  float64 `json:"gross_exposure"`
			NetExposure    float64 `json:"net_exposure"`
			PortfolioValue float64 `json:"portfolio_value"`
		} `json:"rows"`
	}
	decodeBacktestStrategyJSONFile(t, filepath.Join(base, "fixtures", "portfolio_weights_ACME_fixture.json"), &weightsFixture)
	if !weightsFixture.ProviderFree || weightsFixture.LiveNetwork ||
		weightsFixture.FixtureKey != "portfolio_weights:ACME:sma_cross:offline:v1" ||
		weightsFixture.StrategyID != strategyFixture.StrategyID ||
		weightsFixture.Symbol != feedFixture.Symbol ||
		weightsFixture.DeterministicSeed != strategyFixture.DeterministicSeed ||
		weightsFixture.SourceRef == "" ||
		len(weightsFixture.Rows) != len(feedFixture.Rows) {
		t.Fatalf("portfolio weights fixture incomplete: %#v", weightsFixture)
	}
	for _, row := range weightsFixture.Rows {
		if row.Timestamp == "" || row.Cash < 0 || row.Shares < 0 || row.MarketValue < 0 || row.PortfolioValue <= 0 {
			t.Fatalf("portfolio weight row incomplete: %#v", row)
		}
		if math.Abs(row.CashWeight+row.PositionWeight-1) > 0.00001 ||
			math.Abs(row.PositionWeight-row.GrossExposure) > 0.00001 ||
			math.Abs(row.PositionWeight-row.NetExposure) > 0.00001 {
			t.Fatalf("portfolio weight row not normalized: %#v", row)
		}
	}

	var returnsFixture struct {
		ProviderFree      bool   `json:"provider_free"`
		LiveNetwork       bool   `json:"live_network"`
		FixtureKey        string `json:"fixture_key"`
		StrategyID        string `json:"strategy_id"`
		Symbol            string `json:"symbol"`
		DeterministicSeed string `json:"deterministic_seed"`
		SourceRef         string `json:"source_ref"`
		Summary           struct {
			StartValue       float64 `json:"start_value"`
			EndValue         float64 `json:"end_value"`
			TotalReturn      float64 `json:"total_return"`
			MaxDrawdown      float64 `json:"max_drawdown"`
			Annualization    float64 `json:"annualization_factor"`
			FullPeriodSharpe float64 `json:"full_period_sharpe"`
		} `json:"summary"`
		Rows []struct {
			Timestamp        string  `json:"timestamp"`
			PortfolioValue   float64 `json:"portfolio_value"`
			DailyReturn      float64 `json:"daily_return"`
			CumulativeReturn float64 `json:"cumulative_return"`
			Drawdown         float64 `json:"drawdown"`
			RollingSharpe    float64 `json:"rolling_sharpe"`
		} `json:"rows"`
	}
	decodeBacktestStrategyJSONFile(t, filepath.Join(base, "fixtures", "returns_drawdown_sharpe_ACME_fixture.json"), &returnsFixture)
	if !returnsFixture.ProviderFree || returnsFixture.LiveNetwork ||
		returnsFixture.FixtureKey != "returns_drawdown_sharpe:ACME:sma_cross:offline:v1" ||
		returnsFixture.StrategyID != strategyFixture.StrategyID ||
		returnsFixture.Symbol != feedFixture.Symbol ||
		returnsFixture.DeterministicSeed != strategyFixture.DeterministicSeed ||
		returnsFixture.SourceRef != weightsFixture.FixtureKey ||
		len(returnsFixture.Rows) != len(weightsFixture.Rows) {
		t.Fatalf("returns/drawdown/sharpe fixture incomplete: %#v", returnsFixture)
	}
	if returnsFixture.Summary.StartValue != 100000 ||
		returnsFixture.Summary.EndValue <= returnsFixture.Summary.StartValue ||
		returnsFixture.Summary.TotalReturn <= 0 ||
		returnsFixture.Summary.MaxDrawdown >= 0 ||
		returnsFixture.Summary.Annualization != 252 ||
		returnsFixture.Summary.FullPeriodSharpe <= 0 {
		t.Fatalf("returns/drawdown/sharpe summary incomplete: %#v", returnsFixture.Summary)
	}
	var sawDrawdown bool
	for i, row := range returnsFixture.Rows {
		if row.Timestamp == "" || row.PortfolioValue <= 0 {
			t.Fatalf("returns row incomplete: %#v", row)
		}
		if math.Abs(row.PortfolioValue-weightsFixture.Rows[i].PortfolioValue) > 0.01 {
			t.Fatalf("returns row value does not match portfolio weights row %d: %#v vs %#v", i, row, weightsFixture.Rows[i])
		}
		if row.Drawdown < 0 {
			sawDrawdown = true
		}
	}
	if !sawDrawdown {
		t.Fatalf("returns/drawdown/sharpe fixture did not record any drawdown: %#v", returnsFixture.Rows)
	}
}

func TestFinRobotBacktestStrategyLedgerMetricsRiskAndOptionalDependencies(t *testing.T) {
	base := backtestStrategyLivePackageDir(t)
	manifest := loadBacktestStrategyLiveManifest(t, base)

	if len(manifest.RiskLimits) != 4 {
		t.Fatalf("risk limits = %d, want 4", len(manifest.RiskLimits))
	}
	for _, limit := range manifest.RiskLimits {
		if limit.ID == "" || limit.Metric == "" || limit.Limit <= 0 || limit.Observed < 0 || limit.Status != "pass" || limit.LiveNetwork {
			t.Fatalf("risk limit incomplete: %#v", limit)
		}
	}

	var ledgerFixture struct {
		ProviderFree      bool     `json:"provider_free"`
		LiveNetwork       bool     `json:"live_network"`
		FixtureKey        string   `json:"fixture_key"`
		StrategyID        string   `json:"strategy_id"`
		Symbol            string   `json:"symbol"`
		DeterministicSeed string   `json:"deterministic_seed"`
		SchemaFields      []string `json:"schema_fields"`
		Trades            []struct {
			TradeID       string  `json:"trade_id"`
			OrderID       string  `json:"order_id"`
			Timestamp     string  `json:"timestamp"`
			Side          string  `json:"side"`
			Quantity      float64 `json:"quantity"`
			Price         float64 `json:"price"`
			GrossNotional float64 `json:"gross_notional"`
			Commission    float64 `json:"commission"`
			Slippage      float64 `json:"slippage"`
			CashAfter     float64 `json:"cash_after"`
			PositionAfter float64 `json:"position_after"`
			RealizedPnL   float64 `json:"realized_pnl"`
			SourceRef     string  `json:"source_ref"`
		} `json:"trades"`
	}
	decodeBacktestStrategyJSONFile(t, filepath.Join(base, "fixtures", "trade_ledger_ACME_fixture.json"), &ledgerFixture)
	if !ledgerFixture.ProviderFree || ledgerFixture.LiveNetwork ||
		ledgerFixture.FixtureKey != "trade_ledger:ACME:sma_cross:offline:v1" ||
		ledgerFixture.StrategyID != "sma_cross_strategy" ||
		ledgerFixture.Symbol != "ACME" ||
		ledgerFixture.DeterministicSeed != "finrobot-backtest-strategy-offline-v1" ||
		len(ledgerFixture.Trades) != 3 {
		t.Fatalf("ledger fixture incomplete: %#v", ledgerFixture)
	}
	for _, field := range []string{"trade_id", "order_id", "timestamp", "side", "quantity", "price", "commission", "slippage", "cash_after", "position_after", "realized_pnl", "source_ref"} {
		if !contains(ledgerFixture.SchemaFields, field) {
			t.Fatalf("ledger schema fields missing %q: %#v", field, ledgerFixture.SchemaFields)
		}
	}
	for _, trade := range ledgerFixture.Trades {
		if trade.TradeID == "" || trade.OrderID == "" || trade.Timestamp == "" || (trade.Side != "buy" && trade.Side != "sell") ||
			trade.Quantity <= 0 || trade.Price <= 0 || trade.GrossNotional <= 0 || trade.Commission < 0 || trade.Slippage < 0 ||
			trade.CashAfter <= 0 || trade.SourceRef == "" {
			t.Fatalf("ledger trade incomplete: %#v", trade)
		}
	}

	var metricsFixture struct {
		ProviderFree      bool                `json:"provider_free"`
		LiveNetwork       bool                `json:"live_network"`
		FixtureKey        string              `json:"fixture_key"`
		StrategyID        string              `json:"strategy_id"`
		DeterministicSeed string              `json:"deterministic_seed"`
		Metrics           map[string]float64  `json:"metrics"`
		RiskLimits        []backtestRiskLimit `json:"risk_limits"`
	}
	decodeBacktestStrategyJSONFile(t, filepath.Join(base, "fixtures", "metrics_ACME_fixture.json"), &metricsFixture)
	if !metricsFixture.ProviderFree || metricsFixture.LiveNetwork ||
		metricsFixture.FixtureKey != "metrics:ACME:sma_cross:offline:v1" ||
		metricsFixture.StrategyID != "sma_cross_strategy" ||
		metricsFixture.DeterministicSeed != "finrobot-backtest-strategy-offline-v1" ||
		len(metricsFixture.RiskLimits) != 4 {
		t.Fatalf("metrics fixture incomplete: %#v", metricsFixture)
	}
	for _, want := range []string{"start_cash", "end_value", "total_return", "annualized_return", "volatility", "sharpe", "max_drawdown", "win_rate", "trade_count", "turnover", "average_exposure", "max_exposure"} {
		if _, ok := metricsFixture.Metrics[want]; !ok {
			t.Fatalf("metrics missing %q: %#v", want, metricsFixture.Metrics)
		}
	}
	for _, limit := range metricsFixture.RiskLimits {
		if limit.ID == "" || limit.Metric == "" || limit.Status != "pass" || limit.Limit <= 0 || limit.Observed < 0 {
			t.Fatalf("metrics risk limit incomplete: %#v", limit)
		}
	}
	if metricsFixture.Metrics["max_drawdown"] <= 0 ||
		math.Abs(metricsFixture.Metrics["max_drawdown"]-0.006957) > 0.000001 ||
		math.Abs(metricsFixture.Metrics["sharpe"]-0.1753) > 0.0001 {
		t.Fatalf("metrics should align with deterministic returns/drawdown/sharpe fixture: %#v", metricsFixture.Metrics)
	}

	if len(manifest.OptionalDependencies) != 3 {
		t.Fatalf("optional dependencies = %d, want 3", len(manifest.OptionalDependencies))
	}
	optionalIDs := map[string]bool{}
	for _, dep := range manifest.OptionalDependencies {
		if dep.ID == "" || dep.ImportName == "" || dep.RequiredByDefault || dep.RealDependencyImported || !dep.CleanSkipWithoutDependency {
			t.Fatalf("optional dependency must clean-skip and avoid default imports: %#v", dep)
		}
		optionalIDs[dep.ID] = true
	}
	for _, id := range []string{"backtrader", "mplfinance", "openbb"} {
		if !optionalIDs[id] {
			t.Fatalf("optional dependencies missing %q: %#v", id, manifest.OptionalDependencies)
		}
	}

	var optionalContract struct {
		ProviderFree               bool                         `json:"provider_free"`
		LiveNetwork                bool                         `json:"live_network"`
		RealDependencyImports      bool                         `json:"real_dependency_imports"`
		CleanSkipWithoutDependency bool                         `json:"clean_skip_without_dependency"`
		Dependencies               []backtestOptionalDependency `json:"dependencies"`
		DefaultBehavior            struct {
			Mode                      string `json:"mode"`
			RaisesOnMissingDependency bool   `json:"raises_on_missing_optional_dependency"`
			ImportsLiveDependency     bool   `json:"imports_live_dependency"`
			NetworkAccess             bool   `json:"network_access"`
			DefaultCIEnabled          bool   `json:"default_ci_enabled"`
		} `json:"default_behavior"`
	}
	decodeBacktestStrategyJSONFile(t, filepath.Join(base, "contracts", "optional_dependency_contract.json"), &optionalContract)
	if !optionalContract.ProviderFree || optionalContract.LiveNetwork || optionalContract.RealDependencyImports || !optionalContract.CleanSkipWithoutDependency ||
		optionalContract.DefaultBehavior.Mode != "fixture_replay" ||
		optionalContract.DefaultBehavior.RaisesOnMissingDependency ||
		optionalContract.DefaultBehavior.ImportsLiveDependency ||
		optionalContract.DefaultBehavior.NetworkAccess ||
		optionalContract.DefaultBehavior.DefaultCIEnabled {
		t.Fatalf("optional dependency contract should stay clean-skip and outside default CI: %#v", optionalContract)
	}
	if len(optionalContract.Dependencies) != 3 {
		t.Fatalf("optional dependency contract count = %d, want 3", len(optionalContract.Dependencies))
	}
}

func TestFinRobotBacktestStrategyLivePackageNoLiveImportsOrRuntimeCoupling(t *testing.T) {
	base := backtestStrategyLivePackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(path) != ".json" && filepath.Ext(path) != ".leia" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		source := string(data)
		for _, pattern := range []string{
			`(?m)^\s*import\s+`,
			`(?m)^\s*use\s+`,
			`(?m)^\s*load\s*\(`,
			`(?m)^\s*require\s*\(`,
			`(?m)^\s*(requests|fetch|urllib|aiohttp|http|socket|openbb|yfinance|pandas|backtrader|alpaca|ibapi)\s*[.(]`,
			`(?i)\bq/runtime\b|\bruntime/main\b`,
		} {
			if regexp.MustCompile(pattern).FindString(source) != "" {
				return fmt.Errorf("%s contains forbidden live dependency/runtime pattern %q", path, pattern)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFinRobotBacktestStrategyLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(backtestStrategyLivePackageDir(t), "main.leia")
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
			got, err := vm.Get("backtest_strategy_live_package_summary")
			if err != nil {
				t.Fatalf("Get backtest_strategy_live_package_summary: %v", err)
			}
			want := "backtest_strategy_live_package strategies=1 data_feeds=1 seed=finrobot-backtest-strategy-offline-v1 ledgers=1 weights=1 returns=1 metrics=1 risk_limits=4 provider_free=true live_network=false imports=false fixtures=6"
			if got != want {
				t.Fatalf("backtest_strategy_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func backtestStrategyLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "backtest_strategy")
}

func loadBacktestStrategyLiveManifest(t *testing.T, base string) backtestStrategyLiveManifest {
	t.Helper()
	var manifest backtestStrategyLiveManifest
	decodeBacktestStrategyJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertBacktestStrategyJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeBacktestStrategyJSONFile(t, path, &value)
}

func decodeBacktestStrategyJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
