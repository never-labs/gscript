package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	leia "github.com/never-labs/leia"
)

type optionalIntegrationsManifest struct {
	SchemaVersion        int    `json:"schema_version"`
	ID                   string `json:"id"`
	Package              string `json:"package"`
	FixtureVersion       string `json:"fixture_version"`
	OfflineOnly          bool   `json:"offline_only"`
	LiveNetwork          bool   `json:"live_network"`
	RealDependencyImport bool   `json:"real_dependency_imports"`
	FixtureHook          string `json:"fixture_hook"`
	Integrations         []struct {
		ID           string `json:"id"`
		PackageName  string `json:"package_name"`
		ImportName   string `json:"import_name"`
		Capability   string `json:"capability"`
		FixtureKey   string `json:"fixture_key"`
		OutputSchema string `json:"output_schema"`
		CleanSkip    bool   `json:"clean_skip"`
	} `json:"integrations"`
}

func TestFinRobotOptionalIntegrationsManifest(t *testing.T) {
	root := repoRoot(t)
	manifest := loadOptionalIntegrationsManifest(t, root)
	if manifest.SchemaVersion != 1 || manifest.ID != "FR-GAP-018-optional-integrations" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.Package != "finrobot_translation.optional_integrations" || manifest.FixtureVersion != "finrobot-optional-integrations-v1" {
		t.Fatalf("manifest package/version = %q %q", manifest.Package, manifest.FixtureVersion)
	}
	if !manifest.OfflineOnly || manifest.LiveNetwork || manifest.RealDependencyImport || manifest.FixtureHook != "recorded_optional_fixture" {
		t.Fatalf("offline guard fields = %#v", manifest)
	}
	if len(manifest.Integrations) != 7 {
		t.Fatalf("integrations = %d, want 7", len(manifest.Integrations))
	}

	var ids []string
	caps := map[string]bool{}
	fixtures := map[string]bool{}
	for _, integration := range manifest.Integrations {
		if integration.ID == "" || integration.PackageName == "" || integration.ImportName == "" ||
			integration.Capability == "" || integration.FixtureKey == "" || integration.OutputSchema == "" {
			t.Fatalf("incomplete integration manifest: %#v", integration)
		}
		if !integration.CleanSkip {
			t.Fatalf("%s clean_skip = false", integration.ID)
		}
		if caps[integration.Capability] {
			t.Fatalf("duplicate capability %q", integration.Capability)
		}
		if fixtures[integration.FixtureKey] {
			t.Fatalf("duplicate fixture_key %q", integration.FixtureKey)
		}
		ids = append(ids, integration.ID)
		caps[integration.Capability] = true
		fixtures[integration.FixtureKey] = true
	}
	sort.Strings(ids)
	wantIDs := []string{"backtrader", "fingpt", "finml", "finrl", "mplfinance", "ollama", "openbb"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("ids = %#v, want %#v", ids, wantIDs)
	}
}

func TestFinRobotOptionalIntegrationsGateExample(t *testing.T) {
	root := repoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, "examples", "ai", "finrobot_translation", "optional_integrations.leia"))
	if err != nil {
		t.Fatalf("ReadFile optional_integrations.leia: %v", err)
	}

	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, mode.opts...)...)
			if err := vm.Exec(string(src)); err != nil {
				t.Fatalf("Exec optional_integrations.leia: %v", err)
			}
			for name, want := range map[string]any{
				"skipped_count":      int64(7),
				"imported_count":     int64(0),
				"fixture_hook_count": int64(7),
				"tools_ok":           true,
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}

			for name, want := range map[string]string{
				"ready_status":  "ready",
				"denied_status": "denied",
			} {
				summary, err := vm.Get("optional_integrations_summary")
				if err != nil {
					t.Fatalf("Get optional_integrations_summary: %v", err)
				}
				table, ok := summary.(map[string]any)
				if !ok {
					t.Fatalf("optional_integrations_summary = %#v", summary)
				}
				if got := table[name]; got != want {
					t.Fatalf("summary[%s] = %#v, want %#v", name, got, want)
				}
			}
		})
	}
}

func loadOptionalIntegrationsManifest(t *testing.T, root string) optionalIntegrationsManifest {
	t.Helper()
	path := filepath.Join(root, "examples", "ai", "finrobot_translation", "optional_integrations_manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest optionalIntegrationsManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	return manifest
}
