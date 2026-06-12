package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowPackageFixtureIndexesHaveOfflineMetadata(t *testing.T) {
	root := repoRoot(t)
	packagesRoot := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages")
	for _, packageName := range []string{
		"equity_analysis_pipeline",
		"document_pipeline",
		"backtest_strategy",
		"factor_research",
		"analyzer_report",
		"coding_notebook",
	} {
		packageName := packageName
		t.Run(packageName, func(t *testing.T) {
			base := filepath.Join(packagesRoot, packageName)
			indexPath := filepath.Join(base, "fixtures", "provider_free_fixture_index.json")
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
			decodeWorkflowPackageFixtureIndexJSON(t, indexPath, &index)
			if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports {
				t.Fatalf("index must be provider-free and offline: %#v", index)
			}
			if len(index.Fixtures) == 0 {
				t.Fatal("fixture index must include at least one fixture")
			}
			for _, fixture := range index.Fixtures {
				if fixture.FixtureKey == "" || fixture.Capability == "" || fixture.Path == "" || fixture.Schema == "" {
					t.Fatalf("fixture entry is missing key/capability/path/schema: %#v", fixture)
				}
				if fixture.Metadata["replay_ready"] != true ||
					fixture.Metadata["provider_free"] != true ||
					fixture.Metadata["live_network"] != false {
					t.Fatalf("%s offline metadata = %#v", fixture.FixtureKey, fixture.Metadata)
				}
				assertWorkflowPackageFixtureIndexJSONFile(t, base, fixture.Path)
				assertWorkflowPackageFixtureIndexJSONFile(t, base, fixture.Schema)
			}
		})
	}
}

func assertWorkflowPackageFixtureIndexJSONFile(t *testing.T, base, relPath string) {
	t.Helper()
	filePath, _, _ := strings.Cut(relPath, "#")
	decodeWorkflowPackageFixtureIndexJSON(t, filepath.Join(base, filePath), new(any))
}

func decodeWorkflowPackageFixtureIndexJSON(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
