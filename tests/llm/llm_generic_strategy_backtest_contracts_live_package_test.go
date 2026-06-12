package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericStrategyBacktestContractsLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericStrategyBacktestContractsPackageDir(t)
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
	if manifest.SchemaVersion != 1 || manifest.ID != "generic-strategy-backtest-contracts" ||
		manifest.PackageName != "leia-generic-ai-strategy-backtest-contracts" ||
		manifest.PackageBoundaryID != "generic-ai-strategy-backtest-contracts" ||
		manifest.CapabilityID != "generic.ai.strategy.backtest.contracts" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetworkDefault ||
		manifest.LiveModelDefault || manifest.DependsOnQRuntime || manifest.CredentialRequired {
		t.Fatalf("manifest must stay provider-free/generic/offline/credential-free: %#v", manifest)
	}
	for _, want := range []string{"generic.ai.strategy.backtest.contracts", "generic.ai.strategy.backtest.strategy.bind", "generic.ai.strategy.backtest.observation_feed.bind", "generic.ai.strategy.backtest.execution_ledger", "generic.ai.strategy.backtest.allocation_series", "generic.ai.strategy.backtest.performance.series", "generic.ai.strategy.backtest.metric_summary.collect", "generic.ai.strategy.backtest.constraint_limits.enforce", "generic.ai.strategy.backtest.engine.clean_skip"} {
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
	if contract.ID != "generic-strategy-backtest-contracts-contract" ||
		!contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	for _, want := range []string{"strategy_manifest", "observation_feed", "execution_ledger", "allocation_series", "performance_series", "metric_summary", "constraint_limits"} {
		field := contract.FieldContracts[want]
		if field.Schema == "" || field.Fixture == "" || len(field.Required) == 0 {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
		}
	}
}

func TestGenericStrategyBacktestContractsLivePackageFixtureShape(t *testing.T) {
	base := genericStrategyBacktestContractsPackageDir(t)
	index := loadGenericStrategyBacktestFixtureIndex(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"))
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 6 {
		t.Fatalf("fixture index invalid: %#v", index)
	}
	seen := map[string]bool{}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Path == "" || fixture.Schema == "" || !fixture.Metadata.ReplayReady ||
			!fixture.Metadata.ProviderFree || fixture.Metadata.LiveNetwork {
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
	for _, want := range []string{"strategy_manifest:baseline_rule:offline:v1", "observation_feed:subject_alpha:step:offline:v1", "execution_ledger:subject_alpha:baseline_rule:offline:v1", "allocation_series:subject_alpha:baseline_rule:offline:v1", "performance_series:subject_alpha:baseline_rule:offline:v1", "metric_summary:subject_alpha:baseline_rule:offline:v1"} {
		if !seen[want] {
			t.Fatalf("fixture key %q missing from %#v", want, seen)
		}
	}
}

func TestGenericStrategyBacktestContractsLivePackageIsDomainNeutral(t *testing.T) {
	base := genericStrategyBacktestContractsPackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"finrobot", "backtrader", "acme", "aapl", "ticker", "stock", "equity", "finance.", "investment", "valuation", "sec.gov", "10-k", "openbb", "mplfinance", "yfinance", "broker", "portfolio", "trade", "commission", "slippage", "sharpe", "drawdown", "sma", "crossover"} {
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

func TestGenericStrategyBacktestContractsLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericStrategyBacktestContractsPackageDir(t)
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "strategy_manifest_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "strategy_id", "strategy_class", "parameters", "feature_bindings", "allocation_policy_binding", "evaluator_bindings", "deterministic_seed"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "observation_feed_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "subject_id", "granularity", "timezone", "schedule", "unit", "source_ref", "deterministic_seed", "rows"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "execution_ledger_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "strategy_id", "subject_id", "deterministic_seed", "schema_fields", "executions"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "allocation_series_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "strategy_id", "subject_id", "deterministic_seed", "source_ref", "rows"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "performance_series_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "strategy_id", "subject_id", "deterministic_seed", "source_ref", "summary", "rows"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "metric_summary_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "strategy_id", "subject_id", "deterministic_seed", "metric_summary", "constraint_limits"})
}

func TestGenericStrategyBacktestContractsLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericStrategyBacktestContractsPackageDir(t), "main.leia")
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
			got, err := vm.Get("generic_strategy_backtest_contracts_live_package_summary")
			if err != nil {
				t.Fatalf("Get summary: %v", err)
			}
			want := "generic_strategy_backtest_contracts_live_package capability=generic.ai.strategy.backtest.contracts entrypoint=ai.strategy.backtest_contracts strategies=1 observation_feeds=1 seeds=1 ledgers=1 allocation_series=1 performance=1 metric_summary=1 constraint_limits=4 clean_skip=3 provider_free=true live_network=false imports=false model_calls=false"
			if got != want {
				t.Fatalf("summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

type genericStrategyBacktestFixtureIndex struct {
	ProviderFree          bool `json:"provider_free"`
	LiveNetwork           bool `json:"live_network"`
	RealDependencyImports bool `json:"real_dependency_imports"`
	Fixtures              []struct {
		FixtureKey string `json:"fixture_key"`
		Path       string `json:"path"`
		Schema     string `json:"schema"`
		Metadata   struct {
			ReplayReady  bool `json:"replay_ready"`
			ProviderFree bool `json:"provider_free"`
			LiveNetwork  bool `json:"live_network"`
		} `json:"metadata"`
	} `json:"fixtures"`
}

func loadGenericStrategyBacktestFixtureIndex(t *testing.T, path string) genericStrategyBacktestFixtureIndex {
	t.Helper()
	var fixture genericStrategyBacktestFixtureIndex
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func genericStrategyBacktestContractsPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_strategy_backtest_contracts")
}
