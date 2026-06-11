package leia_test

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestFinanceProviderFreeFixtureIndexesDeclareOfflineFlags(t *testing.T) {
	root := repoRoot(t)
	packagesRoot := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages")
	for _, packageName := range []string{
		"finance_facade",
		"finance_normalizers",
		"valuation_engine",
		"analytics_report",
		"chart_renderer",
		"report_renderer",
	} {
		packageName := packageName
		t.Run(packageName, func(t *testing.T) {
			indexPath := filepath.Join(packagesRoot, packageName, "fixtures", "provider_free_fixture_index.json")
			index := readJSONMap(t, indexPath)
			assertFinanceProviderFreeFlags(t, indexPath, "index", index)

			fixtures, ok := index["fixtures"].([]any)
			if !ok {
				t.Fatalf("%s fixtures = %#v, want array", indexPath, index["fixtures"])
			}
			if packageName != "analytics_report" && len(fixtures) == 0 {
				t.Fatalf("%s missing fixture entries", indexPath)
			}
			for i, value := range fixtures {
				fixture, ok := value.(map[string]any)
				if !ok {
					t.Fatalf("%s fixtures[%d] = %#v, want object", indexPath, i, value)
				}
				assertFinanceProviderFreeFlags(t, indexPath, fixtureKeyForFinanceProviderFreeIndex(fixture, i), fixture)
				metadata, ok := fixture["metadata"].(map[string]any)
				if !ok {
					t.Fatalf("%s fixtures[%d] missing metadata map", indexPath, i)
				}
				assertFinanceProviderFreeFlags(t, indexPath, fixtureKeyForFinanceProviderFreeIndex(fixture, i)+".metadata", metadata)
			}
		})
	}
}

func assertFinanceProviderFreeFlags(t *testing.T, path, label string, value map[string]any) {
	t.Helper()
	if !finrobotLivePackageBoolOrConst(value["provider_free"], true) ||
		!finrobotLivePackageBoolOrConst(value["live_network"], false) ||
		!finrobotLivePackageBoolOrConst(value["real_dependency_imports"], false) {
		t.Fatalf("%s %s must declare provider_free=true live_network=false real_dependency_imports=false: %#v", path, label, value)
	}
}

func fixtureKeyForFinanceProviderFreeIndex(fixture map[string]any, fallback int) string {
	for _, key := range []string{"fixture_key", "key"} {
		if value, ok := fixture[key].(string); ok && value != "" {
			return value
		}
	}
	return fmt.Sprintf("fixtures[%d]", fallback)
}
