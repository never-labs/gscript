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

type newsCatalystLiveManifest struct {
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
	Entrypoints       map[string]string `json:"entrypoints"`
	Schemas           map[string]string `json:"schemas"`
	Fixtures          map[string]string `json:"fixtures"`
	Modules           []newsCatalystModule
	AdapterBoundaries []newsCatalystBoundary `json:"adapter_boundaries"`
	TestGates         []string               `json:"test_gates"`
}

type newsCatalystModule struct {
	ID            string   `json:"id"`
	SourceModule  string   `json:"source_module"`
	Capabilities  []string `json:"capabilities"`
	OutputSchemas []string `json:"output_schemas"`
}

type newsCatalystBoundary struct {
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

func TestFinRobotNewsCatalystLivePackageManifest(t *testing.T) {
	base := newsCatalystLivePackageDir(t)
	manifest := loadNewsCatalystLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-news-catalyst-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-news-catalyst" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	if !strings.Contains(manifest.Credentials.Policy, "Polymarket") || !strings.Contains(manifest.Credentials.Policy, "Reddit") {
		t.Fatalf("credential policy should name external adapter credentials: %q", manifest.Credentials.Policy)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_news_catalyst_live_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}

	wantSources := []string{
		"finrobot_equity/core/src/modules/catalyst_analyzer.py",
		"finrobot_equity/core/src/modules/news_integrator.py",
		"finrobot_equity/core/src/modules/retail_sentiment_client.py",
	}
	if !reflect.DeepEqual(manifest.SourceModules, wantSources) {
		t.Fatalf("source modules = %#v, want %#v", manifest.SourceModules, wantSources)
	}

	for _, key := range []string{"smoke", "news_catalyst_contract", "adapter_boundary_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"news_event", "source_ranking", "retail_sentiment_snapshot", "catalyst_analysis", "adapter_boundary"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertNewsCatalystJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "news_catalyst", "retail_sentiment", "adapter_boundary"} {
		path := manifest.Fixtures[key]
		if path == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertNewsCatalystJSONFile(t, filepath.Join(base, path))
	}

	var ids []string
	for _, module := range manifest.Modules {
		ids = append(ids, module.ID)
		if module.ID == "" || module.SourceModule == "" || len(module.Capabilities) < 4 || len(module.OutputSchemas) == 0 {
			t.Fatalf("module metadata incomplete: %#v", module)
		}
	}
	sort.Strings(ids)
	wantIDs := []string{"catalyst_analyzer", "news_integrator", "retail_sentiment_client"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("module ids = %#v, want %#v", ids, wantIDs)
	}

	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"relevance", "category", "sentiment", "impact", "source ranking", "polymarket", "reddit"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotNewsCatalystLivePackageContractsAndFixtures(t *testing.T) {
	base := newsCatalystLivePackageDir(t)

	var contract struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Modules               []struct {
			ID             string   `json:"id"`
			SourceModule   string   `json:"source_module"`
			RequiredFields []string `json:"required_fields"`
		} `json:"modules"`
		FieldContracts  map[string]any `json:"field_contracts"`
		AcceptanceGates []string       `json:"acceptance_gates"`
	}
	decodeNewsCatalystJSONFile(t, filepath.Join(base, "contracts", "news_catalyst_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || len(contract.Modules) != 3 {
		t.Fatalf("contract header/modules = %#v", contract)
	}
	for _, field := range []string{"relevance_score", "category", "sentiment", "impact_score", "source_ranking", "retail_sentiment_snapshot"} {
		if contract.FieldContracts[field] == nil {
			t.Fatalf("missing field contract %q", field)
		}
	}
	acceptance := strings.ToLower(strings.Join(contract.AcceptanceGates, " "))
	for _, want := range []string{"news events", "source rankings", "catalyst", "retail sentiment", "fixture replay"} {
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
	decodeNewsCatalystJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 3 {
		t.Fatalf("fixture index header/count = %#v", index)
	}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Capability == "" || fixture.Path == "" || fixture.Schema == "" {
			t.Fatalf("fixture metadata incomplete: %#v", fixture)
		}
		if fixture.Metadata["replay_ready"] != true {
			t.Fatalf("%s replay_ready = %#v", fixture.FixtureKey, fixture.Metadata["replay_ready"])
		}
		assertNewsCatalystJSONFile(t, filepath.Join(base, fixture.Path))
		assertNewsCatalystJSONFile(t, filepath.Join(base, fixture.Schema))
	}
}

func TestFinRobotNewsCatalystLivePackageAdapterBoundaries(t *testing.T) {
	base := newsCatalystLivePackageDir(t)
	manifest := loadNewsCatalystLiveManifest(t, base)

	var ids []string
	fixtures := map[string]bool{}
	for _, boundary := range manifest.AdapterBoundaries {
		ids = append(ids, boundary.ID)
		if boundary.ID == "" || boundary.DisplayName == "" || boundary.Capability == "" || boundary.FixtureKey == "" || boundary.Schema != "adapter_boundary" {
			t.Fatalf("adapter boundary metadata incomplete: %#v", boundary)
		}
		if boundary.LiveNetwork || boundary.DependencyImported || boundary.CredentialRequired || !boundary.CleanSkip {
			t.Fatalf("adapter boundary must be fixture-only and clean-skip safe: %#v", boundary)
		}
		if fixtures[boundary.FixtureKey] {
			t.Fatalf("duplicate fixture key %q", boundary.FixtureKey)
		}
		fixtures[boundary.FixtureKey] = true
	}
	sort.Strings(ids)
	wantIDs := []string{"polymarket", "reddit", "x_social"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("adapter ids = %#v, want %#v", ids, wantIDs)
	}

	var boundaryContract struct {
		ProviderFree               bool                   `json:"provider_free"`
		LiveNetwork                bool                   `json:"live_network"`
		RealDependencyImports      bool                   `json:"real_dependency_imports"`
		CleanSkipWithoutDependency bool                   `json:"clean_skip_without_dependency"`
		Boundaries                 []newsCatalystBoundary `json:"boundaries"`
	}
	decodeNewsCatalystJSONFile(t, filepath.Join(base, "contracts", "adapter_boundary_contract.json"), &boundaryContract)
	if !boundaryContract.ProviderFree || boundaryContract.LiveNetwork || boundaryContract.RealDependencyImports || !boundaryContract.CleanSkipWithoutDependency {
		t.Fatalf("boundary contract header = %#v", boundaryContract)
	}
	if len(boundaryContract.Boundaries) != 3 {
		t.Fatalf("boundary contract count = %d, want 3", len(boundaryContract.Boundaries))
	}
	for _, boundary := range boundaryContract.Boundaries {
		if boundary.LiveNetwork || boundary.DependencyImported || boundary.CredentialRequired || !boundary.CleanSkip {
			t.Fatalf("boundary contract must not enable live adapters: %#v", boundary)
		}
	}
}

func TestFinRobotNewsCatalystLivePackageFixtureShape(t *testing.T) {
	base := newsCatalystLivePackageDir(t)

	var newsFixture struct {
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
		NewsEvents   []struct {
			EventID        string  `json:"event_id"`
			Category       string  `json:"category"`
			Sentiment      string  `json:"sentiment"`
			RelevanceScore float64 `json:"relevance_score"`
			ImpactScore    float64 `json:"impact_score"`
			SourceRank     int     `json:"source_rank"`
		} `json:"news_events"`
		SourceRankings []struct {
			SourceName      string  `json:"source_name"`
			Rank            int     `json:"rank"`
			TrustScore      float64 `json:"trust_score"`
			RecencyScore    float64 `json:"recency_score"`
			DirectnessScore float64 `json:"directness_score"`
		} `json:"source_rankings"`
		Catalysts []struct {
			CatalystID       string   `json:"catalyst_id"`
			ImpactScore      float64  `json:"impact_score"`
			EvidenceEventIDs []string `json:"evidence_event_ids"`
		} `json:"catalysts"`
	}
	decodeNewsCatalystJSONFile(t, filepath.Join(base, "fixtures", "news_catalyst_ACME_fixture.json"), &newsFixture)
	if !newsFixture.ProviderFree || newsFixture.LiveNetwork || len(newsFixture.NewsEvents) < 2 || len(newsFixture.SourceRankings) < 2 || len(newsFixture.Catalysts) < 2 {
		t.Fatalf("news fixture header/counts = %#v", newsFixture)
	}
	for _, event := range newsFixture.NewsEvents {
		if event.EventID == "" || event.Category == "" || event.Sentiment == "" || event.RelevanceScore <= 0 || event.SourceRank <= 0 {
			t.Fatalf("news event missing relevance/category/sentiment/source rank: %#v", event)
		}
		if event.ImpactScore < -1 || event.ImpactScore > 1 {
			t.Fatalf("news impact out of range: %#v", event)
		}
	}
	for _, catalyst := range newsFixture.Catalysts {
		if catalyst.CatalystID == "" || len(catalyst.EvidenceEventIDs) == 0 || catalyst.ImpactScore < -1 || catalyst.ImpactScore > 1 {
			t.Fatalf("catalyst evidence/impact incomplete: %#v", catalyst)
		}
	}

	var sentimentFixture struct {
		ProviderFree      bool    `json:"provider_free"`
		LiveNetwork       bool    `json:"live_network"`
		AggregateScore    float64 `json:"aggregate_score"`
		MentionVolume     int     `json:"mention_volume"`
		BullishRatio      float64 `json:"bullish_ratio"`
		BearishRatio      float64 `json:"bearish_ratio"`
		PlatformSnapshots []struct {
			Platform       string  `json:"platform"`
			FixtureKey     string  `json:"fixture_key"`
			SentimentScore float64 `json:"sentiment_score"`
			MentionVolume  int     `json:"mention_volume"`
		} `json:"platform_snapshots"`
	}
	decodeNewsCatalystJSONFile(t, filepath.Join(base, "fixtures", "retail_sentiment_ACME_snapshot.json"), &sentimentFixture)
	if !sentimentFixture.ProviderFree || sentimentFixture.LiveNetwork || sentimentFixture.MentionVolume == 0 || len(sentimentFixture.PlatformSnapshots) != 3 {
		t.Fatalf("retail sentiment fixture header/counts = %#v", sentimentFixture)
	}
	platforms := map[string]bool{}
	for _, snapshot := range sentimentFixture.PlatformSnapshots {
		platforms[snapshot.Platform] = true
		if snapshot.FixtureKey == "" || snapshot.MentionVolume < 0 || snapshot.SentimentScore < -1 || snapshot.SentimentScore > 1 {
			t.Fatalf("platform snapshot incomplete: %#v", snapshot)
		}
	}
	for _, want := range []string{"polymarket", "x_social", "reddit"} {
		if !platforms[want] {
			t.Fatalf("missing platform snapshot %q in %#v", want, platforms)
		}
	}
}

func TestFinRobotNewsCatalystLivePackageNoLiveImports(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(newsCatalystLivePackageDir(t), "main.leia"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pattern := range []string{
		`(?m)^\s*import\s+`,
		`(?m)^\s*use\s+`,
		`(?m)^\s*load\s*\(`,
		`(?m)^\s*require\s*\(`,
		`(?m)^\s*(polymarket|praw|reddit|tweepy|xai|requests|http)\s*[.(]`,
	} {
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("main.leia contains live dependency loader matching %q", pattern)
		}
	}
}

func TestFinRobotNewsCatalystLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(newsCatalystLivePackageDir(t), "main.leia")

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
			got, err := vm.Get("news_catalyst_live_package_summary")
			if err != nil {
				t.Fatalf("Get news_catalyst_live_package_summary: %v", err)
			}
			want := "news_catalyst_live_package modules=3 adapters=3 provider_free=true live_network=false imports=false fixtures=3"
			if got != want {
				t.Fatalf("news_catalyst_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func newsCatalystLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "news_catalyst")
}

func loadNewsCatalystLiveManifest(t *testing.T, base string) newsCatalystLiveManifest {
	t.Helper()
	var manifest newsCatalystLiveManifest
	decodeNewsCatalystJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertNewsCatalystJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeNewsCatalystJSONFile(t, path, &value)
}

func decodeNewsCatalystJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
