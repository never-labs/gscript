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

type retailSentimentLiveManifest struct {
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
	Entrypoints      map[string]string               `json:"entrypoints"`
	Schemas          map[string]string               `json:"schemas"`
	Fixtures         map[string]string               `json:"fixtures"`
	Modules          []retailSentimentModule         `json:"modules"`
	SourceBoundaries []retailSentimentSourceBoundary `json:"source_boundaries"`
	PromptContracts  struct {
		SourceSnapshotFormat  string   `json:"source_snapshot_format"`
		AggregateFormat       string   `json:"aggregate_format"`
		ForbiddenPromptFields []string `json:"forbidden_prompt_fields"`
	} `json:"prompt_contracts"`
	TestGates []string `json:"test_gates"`
}

type retailSentimentModule struct {
	ID            string   `json:"id"`
	SourceModule  string   `json:"source_module"`
	Capabilities  []string `json:"capabilities"`
	OutputSchemas []string `json:"output_schemas"`
}

type retailSentimentSourceBoundary struct {
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

func TestFinRobotRetailSentimentLivePackageManifest(t *testing.T) {
	base := retailSentimentLivePackageDir(t)
	manifest := loadRetailSentimentLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-retail-sentiment-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-retail-sentiment" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	for _, want := range []string{"Adanos", "Reddit", "X", "Polymarket"} {
		if !strings.Contains(manifest.Credentials.Policy, want) {
			t.Fatalf("credential policy missing %q: %q", want, manifest.Credentials.Policy)
		}
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_retail_sentiment_live_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}

	for _, key := range []string{"smoke", "retail_sentiment_contract", "prompt_format_contract", "adapter_clean_skip_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
		assertRetailSentimentJSONFile(t, filepath.Join(base, manifest.Entrypoints[key]))
	}
	for _, key := range []string{"source_snapshot", "sentiment_aggregate", "redaction_policy", "terms_metadata", "adapter_clean_skip"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertRetailSentimentJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "source_snapshot", "sentiment_aggregate", "redaction_policy", "adapter_clean_skip"} {
		path := manifest.Fixtures[key]
		if path == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertRetailSentimentJSONFile(t, filepath.Join(base, path))
	}

	var ids []string
	for _, module := range manifest.Modules {
		ids = append(ids, module.ID)
		if module.ID == "" || module.SourceModule == "" || len(module.Capabilities) < 4 || len(module.OutputSchemas) == 0 {
			t.Fatalf("module metadata incomplete: %#v", module)
		}
	}
	sort.Strings(ids)
	wantIDs := []string{"optional_adapter_boundary", "retail_sentiment_client"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("module ids = %#v, want %#v", ids, wantIDs)
	}

	for _, want := range []string{"raw_user_handle", "raw_url", "access_token", "cookie", "authorization_header"} {
		if !retailSentimentStringSliceContains(manifest.PromptContracts.ForbiddenPromptFields, want) {
			t.Fatalf("prompt contract missing forbidden field %q", want)
		}
	}
	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"adanos", "reddit", "x", "polymarket", "redaction", "terms", "stale", "q/runtime"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotRetailSentimentLivePackageContractsAndFixtures(t *testing.T) {
	base := retailSentimentLivePackageDir(t)

	var contract struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Modules               []struct {
			ID             string   `json:"id"`
			RequiredFields []string `json:"required_fields"`
		} `json:"modules"`
		FieldContracts  map[string]any `json:"field_contracts"`
		AcceptanceGates []string       `json:"acceptance_gates"`
	}
	decodeRetailSentimentJSONFile(t, filepath.Join(base, "contracts", "retail_sentiment_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || len(contract.Modules) != 1 {
		t.Fatalf("contract header/modules = %#v", contract)
	}
	for _, field := range []string{"source_snapshot", "sentiment_aggregate", "redaction_policy", "source_terms", "stale_snapshot_warning"} {
		if contract.FieldContracts[field] == nil {
			t.Fatalf("missing field contract %q", field)
		}
	}
	acceptance := strings.ToLower(strings.Join(contract.AcceptanceGates, " "))
	for _, want := range []string{"adanos", "reddit", "x", "polymarket", "source terms", "redaction", "stale"} {
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
	decodeRetailSentimentJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
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
		assertRetailSentimentJSONFile(t, filepath.Join(base, fixture.Path))
		assertRetailSentimentJSONFile(t, filepath.Join(base, fixture.Schema))
	}
}

func TestFinRobotRetailSentimentLivePackageSourceSnapshots(t *testing.T) {
	base := retailSentimentLivePackageDir(t)
	manifest := loadRetailSentimentLiveManifest(t, base)

	var boundaryIDs []string
	for _, boundary := range manifest.SourceBoundaries {
		boundaryIDs = append(boundaryIDs, boundary.ID)
		if boundary.ID == "" || boundary.DisplayName == "" || boundary.Capability == "" || boundary.FixtureKey == "" || boundary.Schema != "source_snapshot" {
			t.Fatalf("source boundary metadata incomplete: %#v", boundary)
		}
		if boundary.LiveNetwork || boundary.DependencyImported || boundary.CredentialRequired || !boundary.CleanSkip {
			t.Fatalf("source boundary must be fixture-only and clean-skip safe: %#v", boundary)
		}
	}
	sort.Strings(boundaryIDs)
	wantBoundaryIDs := []string{"adanos", "polymarket", "reddit", "x_social"}
	if !reflect.DeepEqual(boundaryIDs, wantBoundaryIDs) {
		t.Fatalf("boundary ids = %#v, want %#v", boundaryIDs, wantBoundaryIDs)
	}

	var fixture struct {
		ProviderFree    bool `json:"provider_free"`
		LiveNetwork     bool `json:"live_network"`
		SourceSnapshots []struct {
			SourceID string `json:"source_id"`
			Platform string `json:"platform"`
			Metrics  struct {
				SentimentScore float64 `json:"sentiment_score"`
				MentionVolume  int     `json:"mention_volume"`
				Confidence     float64 `json:"confidence"`
			} `json:"metrics"`
			Redaction struct {
				RawIdentifiersRemoved bool   `json:"raw_identifiers_removed"`
				RawURLsRemoved        bool   `json:"raw_urls_removed"`
				SecretsRemoved        bool   `json:"secrets_removed"`
				DisplayHandle         string `json:"display_handle"`
				RedactedURI           string `json:"redacted_uri"`
			} `json:"redaction"`
			SourceTerms struct {
				Platform       string `json:"platform"`
				Usage          string `json:"usage"`
				Attribution    string `json:"attribution"`
				Redistribution string `json:"redistribution"`
				RetentionDays  int    `json:"retention_days"`
			} `json:"source_terms"`
			StaleSnapshotWarning struct {
				IsStale        bool   `json:"is_stale"`
				StaleAfterDays int    `json:"stale_after_days"`
				Message        string `json:"message"`
			} `json:"stale_snapshot_warning"`
		} `json:"source_snapshots"`
	}
	decodeRetailSentimentJSONFile(t, filepath.Join(base, "fixtures", "source_snapshot_ACME_fixture.json"), &fixture)
	if !fixture.ProviderFree || fixture.LiveNetwork || len(fixture.SourceSnapshots) != 4 {
		t.Fatalf("source snapshot fixture header/count = %#v", fixture)
	}
	platforms := map[string]bool{}
	for _, snapshot := range fixture.SourceSnapshots {
		platforms[snapshot.Platform] = true
		if snapshot.SourceID == "" || snapshot.Metrics.MentionVolume < 0 || snapshot.Metrics.SentimentScore < -1 || snapshot.Metrics.SentimentScore > 1 || snapshot.Metrics.Confidence < 0 {
			t.Fatalf("source snapshot metrics incomplete: %#v", snapshot)
		}
		if !snapshot.Redaction.RawIdentifiersRemoved || !snapshot.Redaction.RawURLsRemoved || !snapshot.Redaction.SecretsRemoved || snapshot.Redaction.DisplayHandle == "" || snapshot.Redaction.RedactedURI == "" {
			t.Fatalf("source snapshot redaction incomplete: %#v", snapshot)
		}
		if snapshot.SourceTerms.Platform != snapshot.Platform || snapshot.SourceTerms.Usage == "" || snapshot.SourceTerms.Attribution == "" || snapshot.SourceTerms.Redistribution == "" {
			t.Fatalf("source terms incomplete: %#v", snapshot)
		}
		if !snapshot.StaleSnapshotWarning.IsStale || snapshot.StaleSnapshotWarning.StaleAfterDays <= 0 || snapshot.StaleSnapshotWarning.Message == "" {
			t.Fatalf("stale snapshot warning incomplete: %#v", snapshot)
		}
	}
	for _, want := range wantBoundaryIDs {
		if !platforms[want] {
			t.Fatalf("missing platform snapshot %q in %#v", want, platforms)
		}
	}
}

func TestFinRobotRetailSentimentLivePackageAggregateAndCleanSkip(t *testing.T) {
	base := retailSentimentLivePackageDir(t)

	var aggregate struct {
		ProviderFree       bool               `json:"provider_free"`
		LiveNetwork        bool               `json:"live_network"`
		AggregateScore     float64            `json:"aggregate_score"`
		MentionVolume      int                `json:"mention_volume"`
		PlatformWeights    map[string]float64 `json:"platform_weights"`
		SourceSnapshotRefs []string           `json:"source_snapshot_refs"`
		RedactionSummary   struct {
			RawIdentifiersRemoved bool `json:"raw_identifiers_removed"`
			RawURLsRemoved        bool `json:"raw_urls_removed"`
			SecretsRemoved        bool `json:"secrets_removed"`
			PromptSafe            bool `json:"prompt_safe"`
		} `json:"redaction_summary"`
		StaleSnapshotWarning struct {
			IsStale        bool   `json:"is_stale"`
			StaleAfterDays int    `json:"stale_after_days"`
			Message        string `json:"message"`
		} `json:"stale_snapshot_warning"`
	}
	decodeRetailSentimentJSONFile(t, filepath.Join(base, "fixtures", "sentiment_aggregate_ACME_fixture.json"), &aggregate)
	if !aggregate.ProviderFree || aggregate.LiveNetwork || aggregate.MentionVolume == 0 || aggregate.AggregateScore < -1 || aggregate.AggregateScore > 1 {
		t.Fatalf("aggregate fixture header/ranges = %#v", aggregate)
	}
	if len(aggregate.PlatformWeights) != 4 || len(aggregate.SourceSnapshotRefs) != 4 {
		t.Fatalf("aggregate platform weights/refs incomplete: %#v", aggregate)
	}
	for _, want := range []string{"adanos", "reddit", "x_social", "polymarket"} {
		if aggregate.PlatformWeights[want] <= 0 {
			t.Fatalf("aggregate missing positive platform weight %q: %#v", want, aggregate.PlatformWeights)
		}
	}
	if !aggregate.RedactionSummary.RawIdentifiersRemoved || !aggregate.RedactionSummary.RawURLsRemoved || !aggregate.RedactionSummary.SecretsRemoved || !aggregate.RedactionSummary.PromptSafe {
		t.Fatalf("aggregate redaction summary incomplete: %#v", aggregate.RedactionSummary)
	}
	if !aggregate.StaleSnapshotWarning.IsStale || aggregate.StaleSnapshotWarning.StaleAfterDays <= 0 || aggregate.StaleSnapshotWarning.Message == "" {
		t.Fatalf("aggregate stale warning incomplete: %#v", aggregate.StaleSnapshotWarning)
	}

	var skip struct {
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		AdapterID             string `json:"adapter_id"`
		OK                    bool   `json:"ok"`
		Skip                  bool   `json:"skip"`
		DependencyImported    bool   `json:"dependency_imported"`
		CredentialRequired    bool   `json:"credential_required"`
		CleanSkip             bool   `json:"clean_skip"`
		FixtureKey            string `json:"fixture_key"`
	}
	decodeRetailSentimentJSONFile(t, filepath.Join(base, "fixtures", "adapter_clean_skip_fixture.json"), &skip)
	if !skip.ProviderFree || skip.LiveNetwork || skip.RealDependencyImports || skip.AdapterID == "" || skip.OK || !skip.Skip || skip.DependencyImported || skip.CredentialRequired || !skip.CleanSkip || skip.FixtureKey == "" {
		t.Fatalf("clean skip fixture must stay provider-free and dependency-free: %#v", skip)
	}
}

func TestFinRobotRetailSentimentLivePackageNoLiveImports(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(retailSentimentLivePackageDir(t), "main.leia"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pattern := range []string{
		`(?m)^\s*import\s+`,
		`(?m)^\s*use\s+`,
		`(?m)^\s*load\s*\(`,
		`(?m)^\s*require\s*\(`,
		`(?m)^\s*(q/runtime|adanos|polymarket|praw|reddit|tweepy|xai|requests|http)\s*[.(]`,
	} {
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("main.leia contains live dependency loader matching %q", pattern)
		}
	}
}

func TestFinRobotRetailSentimentLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(retailSentimentLivePackageDir(t), "main.leia")

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
			got, err := vm.Get("retail_sentiment_live_package_summary")
			if err != nil {
				t.Fatalf("Get retail_sentiment_live_package_summary: %v", err)
			}
			want := "retail_sentiment_live_package sources=4 schemas=5 fixtures=4 provider_free=true live_network=false imports=false clean_skip=true redaction=true terms=true stale_warning=true"
			if got != want {
				t.Fatalf("retail_sentiment_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func retailSentimentLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "retail_sentiment")
}

func loadRetailSentimentLiveManifest(t *testing.T, base string) retailSentimentLiveManifest {
	t.Helper()
	var manifest retailSentimentLiveManifest
	decodeRetailSentimentJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertRetailSentimentJSONFile(t *testing.T, path string) {
	t.Helper()
	if strings.HasSuffix(path, ".leia") {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
		return
	}
	var value any
	decodeRetailSentimentJSONFile(t, path, &value)
}

func decodeRetailSentimentJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func retailSentimentStringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
