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
	PackageName           string `json:"package_name"`
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
	Entrypoints map[string]string `json:"entrypoints"`
	Redaction   struct {
		Enabled        bool     `json:"enabled"`
		SecretPatterns []string `json:"secret_patterns"`
		Replacement    string   `json:"replacement"`
	} `json:"redaction"`
	CapabilityRegistry struct {
		Version            string   `json:"version"`
		Mode               string   `json:"mode"`
		ProviderFree       bool     `json:"provider_free"`
		LiveNetwork        bool     `json:"live_network"`
		RegistryScope      string   `json:"registry_scope"`
		CommonCapabilities []string `json:"common_capabilities"`
		Providers          []struct {
			Provider       string   `json:"provider"`
			AdapterCount   int      `json:"adapter_count"`
			CredentialRefs []string `json:"credential_refs"`
			FixtureReplay  bool     `json:"fixture_replay"`
		} `json:"providers"`
	} `json:"capability_registry"`
	AuthRedaction struct {
		Enabled                            bool     `json:"enabled"`
		Fixture                            string   `json:"fixture"`
		FixtureKey                         string   `json:"fixture_key"`
		SecretValuePolicy                  string   `json:"secret_value_policy"`
		CredentialRequiredForFixtureReplay bool     `json:"credential_required_for_fixture_replay"`
		SecretValuesPresent                bool     `json:"secret_values_present"`
		Replacement                        string   `json:"replacement"`
		Headers                            []string `json:"headers"`
		QueryParams                        []string `json:"query_params"`
		EnvRefs                            []string `json:"env_refs"`
	} `json:"auth_redaction"`
	RetryCacheEnvelope struct {
		Mode                string   `json:"mode"`
		ProviderFree        bool     `json:"provider_free"`
		LiveNetwork         bool     `json:"live_network"`
		MaxAttempts         int      `json:"max_attempts"`
		Backoff             string   `json:"backoff"`
		RetryableErrorKinds []string `json:"retryable_error_kinds"`
		Cache               struct {
			Mode         string `json:"mode"`
			Namespace    string `json:"namespace"`
			KeySource    string `json:"key_source"`
			TTLSeconds   int    `json:"ttl_seconds"`
			HitState     string `json:"hit_state"`
			MissBehavior string `json:"miss_behavior"`
		} `json:"cache"`
	} `json:"retry_cache_envelope"`
	RateLimitFixture struct {
		Fixture     string `json:"fixture"`
		FixtureKey  string `json:"fixture_key"`
		Capability  string `json:"capability"`
		LiveNetwork bool   `json:"live_network"`
	} `json:"rate_limit_fixture"`
	ErrorTaxonomy struct {
		Version          string `json:"version"`
		NormalizedErrors []struct {
			Kind       string `json:"kind"`
			HTTPStatus int    `json:"http_status"`
			Retryable  bool   `json:"retryable"`
			CacheState string `json:"cache_state"`
		} `json:"normalized_errors"`
	} `json:"error_taxonomy"`
	OfflineReplay struct {
		Enabled                     bool   `json:"enabled"`
		FixtureIndex                string `json:"fixture_index"`
		RequireFixtureKey           bool   `json:"require_fixture_key"`
		LiveNetwork                 bool   `json:"live_network"`
		ProviderCredentialsRequired bool   `json:"provider_credentials_required"`
		MissBehavior                string `json:"miss_behavior"`
		SecretValuesPresent         bool   `json:"secret_values_present"`
	} `json:"offline_replay"`
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
	NoBuiltInGuarantee struct {
		Required  bool   `json:"required"`
		Statement string `json:"statement"`
	} `json:"no_built_in_guarantee"`
}

func TestFinRobotVendorLivePackageManifestProviderFree(t *testing.T) {
	root := repoRoot(t)
	pkgDir := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "vendor_adapters")
	manifest := loadVendorLivePackageManifest(t, pkgDir)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-live-vendor-adapters" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-vendor-adapters" {
		t.Fatalf("package_name = %q", manifest.PackageName)
	}
	if manifest.Entrypoints["main"] != "main.leia" {
		t.Fatalf("entrypoints.main = %q", manifest.Entrypoints["main"])
	}
	if !manifest.NoBuiltInGuarantee.Required ||
		!strings.Contains(strings.ToLower(manifest.NoBuiltInGuarantee.Statement), "does not provide") ||
		!strings.Contains(strings.ToLower(manifest.NoBuiltInGuarantee.Statement), "built-in") {
		t.Fatalf("no_built_in_guarantee incomplete: %#v", manifest.NoBuiltInGuarantee)
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
	if manifest.CapabilityRegistry.Version != "vendor-adapter-capabilities-v1" ||
		manifest.CapabilityRegistry.Mode != "provider-free-boundary" ||
		!manifest.CapabilityRegistry.ProviderFree ||
		manifest.CapabilityRegistry.LiveNetwork ||
		len(manifest.CapabilityRegistry.CommonCapabilities) < 10 {
		t.Fatalf("capability registry incomplete: %#v", manifest.CapabilityRegistry)
	}
	if !manifest.AuthRedaction.Enabled ||
		manifest.AuthRedaction.Fixture == "" ||
		manifest.AuthRedaction.FixtureKey != "vendor_adapters:auth_redaction:offline" ||
		manifest.AuthRedaction.SecretValuePolicy != "never-store-secret-values" ||
		manifest.AuthRedaction.CredentialRequiredForFixtureReplay ||
		manifest.AuthRedaction.SecretValuesPresent ||
		manifest.AuthRedaction.Replacement != manifest.Redaction.Replacement {
		t.Fatalf("auth redaction contract incomplete: %#v", manifest.AuthRedaction)
	}
	if manifest.RetryCacheEnvelope.Mode != "deterministic_fixture_replay" ||
		!manifest.RetryCacheEnvelope.ProviderFree ||
		manifest.RetryCacheEnvelope.LiveNetwork ||
		manifest.RetryCacheEnvelope.MaxAttempts != 1 ||
		manifest.RetryCacheEnvelope.Backoff != "none" ||
		manifest.RetryCacheEnvelope.Cache.Mode != "fixture_replay" ||
		manifest.RetryCacheEnvelope.Cache.KeySource != "fixture_key" ||
		manifest.RetryCacheEnvelope.Cache.MissBehavior != "clean-skip" {
		t.Fatalf("retry/cache envelope must stay deterministic and offline: %#v", manifest.RetryCacheEnvelope)
	}
	if manifest.RateLimitFixture.Fixture == "" ||
		manifest.RateLimitFixture.FixtureKey != "vendor_adapters:rate_limits:offline" ||
		manifest.RateLimitFixture.Capability != "finance.vendor.adapter.rate_limits" ||
		manifest.RateLimitFixture.LiveNetwork {
		t.Fatalf("rate limit fixture metadata incomplete: %#v", manifest.RateLimitFixture)
	}
	if manifest.ErrorTaxonomy.Version != "vendor-error-taxonomy-v1" || len(manifest.ErrorTaxonomy.NormalizedErrors) < 5 {
		t.Fatalf("normalized error taxonomy incomplete: %#v", manifest.ErrorTaxonomy)
	}
	if !manifest.OfflineReplay.Enabled ||
		manifest.OfflineReplay.FixtureIndex == "" ||
		!manifest.OfflineReplay.RequireFixtureKey ||
		manifest.OfflineReplay.LiveNetwork ||
		manifest.OfflineReplay.ProviderCredentialsRequired ||
		manifest.OfflineReplay.MissBehavior != "clean-skip" ||
		manifest.OfflineReplay.SecretValuesPresent {
		t.Fatalf("offline replay contract incomplete: %#v", manifest.OfflineReplay)
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
		assertFixtureProviderFree(t, filepath.Join(pkgDir, adapter.Fixture), adapter)
	}

	sort.Strings(ids)
	wantIDs := []string{
		"earnings_transcript",
		"finnhub_metrics",
		"finnhub_news_error",
		"finnhub_profile",
		"fmp_company_metrics",
		"fmp_key_metrics",
		"fmp_news",
		"fmp_ratings",
		"fmp_targets",
		"fmp_technical_indicators",
		"reddit_search",
		"sec_companyfacts",
		"sec_filing_10k",
		"sec_filing_10q",
		"sec_filing_8k",
		"yfinance_cashflow",
		"yfinance_chart",
		"yfinance_dividends",
		"yfinance_info",
		"yfinance_recommendations",
		"yfinance_statements",
	}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("adapter ids = %#v, want %#v", ids, wantIDs)
	}

	wantProviders := map[string]int{
		"earnings": 1,
		"finnhub":  3,
		"fmp":      6,
		"reddit":   1,
		"sec":      4,
		"yfinance": 6,
	}
	gotProviders := map[string]int{}
	for _, adapter := range manifest.Adapters {
		gotProviders[adapter.Provider]++
	}
	if !reflect.DeepEqual(gotProviders, wantProviders) {
		t.Fatalf("provider matrix = %#v, want %#v", gotProviders, wantProviders)
	}
	gotRegistryProviders := map[string]int{}
	for _, provider := range manifest.CapabilityRegistry.Providers {
		if !provider.FixtureReplay {
			t.Fatalf("%s registry entry must use fixture replay: %#v", provider.Provider, provider)
		}
		gotRegistryProviders[provider.Provider] = provider.AdapterCount
	}
	if !reflect.DeepEqual(gotRegistryProviders, wantProviders) {
		t.Fatalf("capability registry provider matrix = %#v, want %#v", gotRegistryProviders, wantProviders)
	}
	for _, envRef := range manifest.AuthRedaction.EnvRefs {
		if !vendorLiveContainsString(manifest.Redaction.SecretPatterns, envRef) {
			t.Fatalf("auth redaction env ref %q missing from redaction patterns %#v", envRef, manifest.Redaction.SecretPatterns)
		}
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
			want := "vendor_live_package adapters=21 provider_free=true live_network=false imports=false redaction=true capabilities=11 errors=5 replay=clean-skip cache=fixture-hit"
			if got != want {
				t.Fatalf("vendor_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func TestFinRobotVendorLivePackageProviderFreeBoundaryFixtures(t *testing.T) {
	root := repoRoot(t)
	pkgDir := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "vendor_adapters")
	manifest := loadVendorLivePackageManifest(t, pkgDir)

	var rateFixture struct {
		FixtureKey   string `json:"fixture_key"`
		Provider     string `json:"provider"`
		Capability   string `json:"capability"`
		LiveNetwork  bool   `json:"live_network"`
		ProviderFree bool   `json:"provider_free"`
		Metadata     struct {
			Mode                string `json:"mode"`
			Redacted            bool   `json:"redacted"`
			SecretValuesPresent bool   `json:"secret_values_present"`
			RetryCacheEnvelope  struct {
				MaxAttempts int    `json:"max_attempts"`
				Backoff     string `json:"backoff"`
				CacheMode   string `json:"cache_mode"`
				CacheState  string `json:"cache_state"`
			} `json:"retry_cache_envelope"`
		} `json:"metadata"`
		Providers []struct {
			Provider          string `json:"provider"`
			WindowSeconds     int    `json:"window_seconds"`
			Limit             int    `json:"limit"`
			RetryAfterSeconds int    `json:"retry_after_seconds"`
			Policy            string `json:"policy"`
		} `json:"providers"`
	}
	readVendorLiveJSON(t, filepath.Join(pkgDir, manifest.RateLimitFixture.Fixture), &rateFixture)
	if rateFixture.FixtureKey != manifest.RateLimitFixture.FixtureKey ||
		rateFixture.Capability != manifest.RateLimitFixture.Capability ||
		rateFixture.LiveNetwork ||
		!rateFixture.ProviderFree ||
		rateFixture.Metadata.Mode != "fixture_replay" ||
		!rateFixture.Metadata.Redacted ||
		rateFixture.Metadata.SecretValuesPresent ||
		rateFixture.Metadata.RetryCacheEnvelope.MaxAttempts != manifest.RetryCacheEnvelope.MaxAttempts ||
		rateFixture.Metadata.RetryCacheEnvelope.CacheMode != manifest.RetryCacheEnvelope.Cache.Mode {
		t.Fatalf("rate limit fixture does not match provider-free envelope: %#v", rateFixture)
	}

	providerRateLimits := map[string]int{}
	for _, provider := range rateFixture.Providers {
		if provider.Policy != "metadata-only" || provider.WindowSeconds <= 0 || provider.Limit <= 0 {
			t.Fatalf("rate fixture provider metadata incomplete: %#v", provider)
		}
		providerRateLimits[provider.Provider] = provider.Limit
	}
	for _, adapter := range manifest.Adapters {
		if providerRateLimits[adapter.Provider] != adapter.RateLimit.Limit {
			t.Fatalf("%s rate fixture limit = %d, want %d", adapter.Provider, providerRateLimits[adapter.Provider], adapter.RateLimit.Limit)
		}
	}

	var replayIndex struct {
		FixtureKey                  string   `json:"fixture_key"`
		Provider                    string   `json:"provider"`
		Capability                  string   `json:"capability"`
		LiveNetwork                 bool     `json:"live_network"`
		ProviderFree                bool     `json:"provider_free"`
		Mode                        string   `json:"mode"`
		RequireFixtureKey           bool     `json:"require_fixture_key"`
		ProviderCredentialsRequired bool     `json:"provider_credentials_required"`
		MissBehavior                string   `json:"miss_behavior"`
		SecretValuesPresent         bool     `json:"secret_values_present"`
		ReplayKeys                  []string `json:"replay_keys"`
		NormalizedErrorExamples     []struct {
			FixtureKey        string `json:"fixture_key"`
			Provider          string `json:"provider"`
			Kind              string `json:"kind"`
			HTTPStatus        int    `json:"http_status"`
			Retryable         bool   `json:"retryable"`
			RetryAfterSeconds int    `json:"retry_after_seconds"`
			CacheState        string `json:"cache_state"`
		} `json:"normalized_error_examples"`
	}
	readVendorLiveJSON(t, filepath.Join(pkgDir, manifest.OfflineReplay.FixtureIndex), &replayIndex)
	if replayIndex.LiveNetwork ||
		!replayIndex.ProviderFree ||
		replayIndex.Mode != "fixture_replay" ||
		!replayIndex.RequireFixtureKey ||
		replayIndex.ProviderCredentialsRequired ||
		replayIndex.MissBehavior != manifest.OfflineReplay.MissBehavior ||
		replayIndex.SecretValuesPresent ||
		len(replayIndex.ReplayKeys) != len(manifest.Adapters) {
		t.Fatalf("offline replay index incomplete: %#v", replayIndex)
	}
	replayKeys := map[string]bool{}
	for _, key := range replayIndex.ReplayKeys {
		replayKeys[key] = true
	}
	for _, adapter := range manifest.Adapters {
		if !replayKeys[adapter.FixtureKey] {
			t.Fatalf("offline replay index missing %q", adapter.FixtureKey)
		}
	}
	if len(replayIndex.NormalizedErrorExamples) == 0 ||
		!manifestErrorKind(manifest, replayIndex.NormalizedErrorExamples[0].Kind, replayIndex.NormalizedErrorExamples[0].HTTPStatus) {
		t.Fatalf("offline replay normalized error example missing taxonomy match: %#v", replayIndex.NormalizedErrorExamples)
	}

	var authFixture struct {
		FixtureKey                  string   `json:"fixture_key"`
		Capability                  string   `json:"capability"`
		LiveNetwork                 bool     `json:"live_network"`
		ProviderFree                bool     `json:"provider_free"`
		SecretValuePolicy           string   `json:"secret_value_policy"`
		SecretValuesPresent         bool     `json:"secret_values_present"`
		Replacement                 string   `json:"replacement"`
		HeadersRedacted             []string `json:"headers_redacted"`
		QueryParamsRedacted         []string `json:"query_params_redacted"`
		EnvRefs                     []string `json:"env_refs"`
		CredentialRequiredForReplay bool     `json:"credential_required_for_replay"`
	}
	readVendorLiveJSON(t, filepath.Join(pkgDir, manifest.AuthRedaction.Fixture), &authFixture)
	if authFixture.FixtureKey != manifest.AuthRedaction.FixtureKey ||
		authFixture.LiveNetwork ||
		!authFixture.ProviderFree ||
		authFixture.SecretValuePolicy != manifest.AuthRedaction.SecretValuePolicy ||
		authFixture.SecretValuesPresent ||
		authFixture.Replacement != manifest.AuthRedaction.Replacement ||
		authFixture.CredentialRequiredForReplay ||
		!reflect.DeepEqual(authFixture.EnvRefs, manifest.AuthRedaction.EnvRefs) {
		t.Fatalf("auth redaction fixture incomplete: %#v", authFixture)
	}
}

func loadVendorLivePackageManifest(t *testing.T, pkgDir string) vendorLivePackageManifest {
	t.Helper()
	var data []byte
	var manifestPath string
	for _, name := range []string{"package.manifest.json", "manifest.json"} {
		path := filepath.Join(pkgDir, name)
		var err error
		data, err = os.ReadFile(path)
		if err == nil {
			manifestPath = path
			break
		}
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	if data == nil {
		t.Fatalf("%s has no package.manifest.json or legacy manifest.json", pkgDir)
	}
	var manifest vendorLivePackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode vendor adapter live manifest %s: %v", manifestPath, err)
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

func readVendorLiveJSON(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
}

func vendorLiveContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func manifestErrorKind(manifest vendorLivePackageManifest, kind string, status int) bool {
	for _, normalized := range manifest.ErrorTaxonomy.NormalizedErrors {
		if normalized.Kind == kind && normalized.HTTPStatus == status {
			return true
		}
	}
	return false
}

func assertFixtureProviderFree(t *testing.T, path string, adapter struct {
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
}) {
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
		Metadata struct {
			Schema     string `json:"schema"`
			Capability string `json:"capability"`
			Provenance struct {
				Source      string `json:"source"`
				CapturedAt  string `json:"captured_at"`
				LiveNetwork bool   `json:"live_network"`
				Redacted    bool   `json:"redacted"`
			} `json:"provenance"`
			RateLimit struct {
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
		} `json:"metadata"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
	if fixture.FixtureKey != adapter.FixtureKey || fixture.Provider != adapter.Provider || fixture.Request.LiveNetwork {
		t.Fatalf("fixture must match manifest and disable live network: %#v", fixture)
	}
	if fixture.Metadata.Schema != adapter.Schema || fixture.Metadata.Capability != adapter.Capability {
		t.Fatalf("%s fixture schema/capability metadata mismatch: %#v", adapter.ID, fixture.Metadata)
	}
	if fixture.Metadata.Provenance.Source == "" || fixture.Metadata.Provenance.CapturedAt == "" ||
		fixture.Metadata.Provenance.LiveNetwork || !fixture.Metadata.Provenance.Redacted {
		t.Fatalf("%s fixture provenance metadata incomplete: %#v", adapter.ID, fixture.Metadata.Provenance)
	}
	if !reflect.DeepEqual(fixture.Metadata.RateLimit, adapter.RateLimit) {
		t.Fatalf("%s fixture rate limit metadata = %#v, want %#v", adapter.ID, fixture.Metadata.RateLimit, adapter.RateLimit)
	}
	if !reflect.DeepEqual(fixture.Metadata.Terms, adapter.Terms) {
		t.Fatalf("%s fixture terms metadata = %#v, want %#v", adapter.ID, fixture.Metadata.Terms, adapter.Terms)
	}
}
