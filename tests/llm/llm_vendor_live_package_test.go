package leia_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type vendorLivePackageManifest struct {
	SchemaVersion         int    `json:"schema_version"`
	ID                    string `json:"id"`
	Package               string `json:"package"`
	Version               string `json:"version"`
	ProviderFree          bool   `json:"provider_free"`
	LiveNetwork           bool   `json:"live_network"`
	RealDependencyImports bool   `json:"real_dependency_imports"`
	FixtureRoot           string `json:"fixture_root"`
	SchemaRoot            string `json:"schema_root"`
	DefaultPolicy         struct {
		Mode                        string `json:"mode"`
		LiveNetwork                 bool   `json:"live_network"`
		ProviderCredentialsRequired bool   `json:"provider_credentials_required"`
		CleanSkipWithoutCredentials bool   `json:"clean_skip_without_credentials"`
		RedactSecretValues          bool   `json:"redact_secret_values"`
	} `json:"default_policy"`
	Redaction struct {
		Enabled        bool     `json:"enabled"`
		SecretPatterns []string `json:"secret_patterns"`
		Replacement    string   `json:"replacement"`
	} `json:"redaction"`
	Adapters []struct {
		ID                 string   `json:"id"`
		DisplayName        string   `json:"display_name"`
		Provider           string   `json:"provider"`
		Capability         string   `json:"capability"`
		CommonCapabilities []string `json:"common_capabilities"`
		Credential         struct {
			Required  bool    `json:"required"`
			SecretRef *string `json:"secret_ref"`
			Redacted  bool    `json:"redacted"`
		} `json:"credential"`
		Schema     string `json:"schema"`
		Fixture    string `json:"fixture"`
		FixtureKey string `json:"fixture_key"`
		RateLimit  struct {
			Policy            string `json:"policy"`
			WindowSeconds     int    `json:"window_seconds"`
			Limit             int    `json:"limit"`
			RetryAfterSeconds int    `json:"retry_after_seconds"`
		} `json:"rate_limit"`
		Terms struct {
			Usage          string `json:"usage"`
			Attribution    string `json:"attribution"`
			Redistribution string `json:"redistribution"`
			LiveNetwork    bool   `json:"live_network"`
		} `json:"terms"`
	} `json:"adapters"`
}

func TestFinRobotVendorLivePackageManifestProviderFree(t *testing.T) {
	root := repoRoot(t)
	pkgDir := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "vendor_adapters")
	manifest := loadVendorLivePackageManifest(t, pkgDir)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-live-vendor-adapters" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if !manifest.ProviderFree || manifest.LiveNetwork || manifest.RealDependencyImports {
		t.Fatalf("provider-free defaults = provider_free=%v live_network=%v imports=%v", manifest.ProviderFree, manifest.LiveNetwork, manifest.RealDependencyImports)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		!manifest.DefaultPolicy.CleanSkipWithoutCredentials ||
		!manifest.DefaultPolicy.RedactSecretValues {
		t.Fatalf("default policy must stay fixture-only and credential-absent safe: %#v", manifest.DefaultPolicy)
	}
	if !manifest.Redaction.Enabled || manifest.Redaction.Replacement != "<redacted>" || len(manifest.Redaction.SecretPatterns) < 5 {
		t.Fatalf("redaction metadata incomplete: %#v", manifest.Redaction)
	}

	var ids []string
	capabilities := map[string]bool{}
	fixtures := map[string]bool{}
	for _, adapter := range manifest.Adapters {
		ids = append(ids, adapter.ID)
		if adapter.ID == "" || adapter.Provider == "" || adapter.DisplayName == "" {
			t.Fatalf("adapter identity incomplete: %#v", adapter)
		}
		if !strings.HasPrefix(adapter.Capability, "finance.vendor.") {
			t.Fatalf("%s capability = %q", adapter.ID, adapter.Capability)
		}
		if len(adapter.CommonCapabilities) == 0 {
			t.Fatalf("%s common capabilities missing", adapter.ID)
		}
		if adapter.Credential.Required || !adapter.Credential.Redacted {
			t.Fatalf("%s credential policy must be optional and redacted: %#v", adapter.ID, adapter.Credential)
		}
		if adapter.RateLimit.Policy == "" || adapter.RateLimit.Limit <= 0 || adapter.RateLimit.WindowSeconds <= 0 {
			t.Fatalf("%s rate limit metadata incomplete: %#v", adapter.ID, adapter.RateLimit)
		}
		if adapter.Terms.Usage == "" || adapter.Terms.Attribution == "" || adapter.Terms.Redistribution == "" || adapter.Terms.LiveNetwork {
			t.Fatalf("%s terms metadata must deny live network: %#v", adapter.ID, adapter.Terms)
		}
		if capabilities[adapter.Capability] {
			t.Fatalf("duplicate capability %q", adapter.Capability)
		}
		if fixtures[adapter.FixtureKey] {
			t.Fatalf("duplicate fixture key %q", adapter.FixtureKey)
		}
		capabilities[adapter.Capability] = true
		fixtures[adapter.FixtureKey] = true
		assertVendorLiveJSONFile(t, filepath.Join(pkgDir, adapter.Schema))
		assertFixtureProviderFree(t, filepath.Join(pkgDir, adapter.Fixture), adapter.ID, adapter.FixtureKey)
	}

	sort.Strings(ids)
	wantIDs := []string{"earnings", "finnhub", "fmp", "reddit", "sec", "yfinance"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("adapter ids = %#v, want %#v", ids, wantIDs)
	}
}

func TestFinRobotVendorLivePackageExecutableSkeleton(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "vendor_adapters", "main.leia")

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
			got, err := vm.Get("vendor_live_package_summary")
			if err != nil {
				t.Fatalf("Get vendor_live_package_summary: %v", err)
			}
			want := "vendor_live_package adapters=6 provider_free=true live_network=false imports=false redaction=true"
			if got != want {
				t.Fatalf("vendor_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func loadVendorLivePackageManifest(t *testing.T, pkgDir string) vendorLivePackageManifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(pkgDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest vendorLivePackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode vendor adapter live manifest: %v", err)
	}
	return manifest
}

func assertVendorLiveJSONFile(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
}

func assertFixtureProviderFree(t *testing.T, path, provider, fixtureKey string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		FixtureKey string `json:"fixture_key"`
		Provider   string `json:"provider"`
		Request    struct {
			LiveNetwork bool `json:"live_network"`
		} `json:"request"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
	if fixture.FixtureKey != fixtureKey || fixture.Provider != provider || fixture.Request.LiveNetwork {
		t.Fatalf("fixture must match manifest and disable live network: %#v", fixture)
	}
}
