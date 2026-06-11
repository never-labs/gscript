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

type chartRendererLiveManifest struct {
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
	Entrypoints        map[string]string               `json:"entrypoints"`
	Schemas            map[string]string               `json:"schemas"`
	Fixtures           map[string]string               `json:"fixtures"`
	Modules            []chartRendererModule           `json:"modules"`
	RendererBoundaries []chartRendererRendererBoundary `json:"renderer_boundaries"`
	TestGates          []string                        `json:"test_gates"`
}

type chartRendererModule struct {
	ID            string   `json:"id"`
	SourceModule  string   `json:"source_module"`
	Capabilities  []string `json:"capabilities"`
	OutputSchemas []string `json:"output_schemas"`
}

type chartRendererRendererBoundary struct {
	ID                 string `json:"id"`
	Capability         string `json:"capability"`
	FixtureKey         string `json:"fixture_key"`
	Schema             string `json:"schema"`
	LiveNetwork        bool   `json:"live_network"`
	DependencyImported bool   `json:"dependency_imported"`
	CredentialRequired bool   `json:"credential_required"`
	CleanSkip          bool   `json:"clean_skip"`
}

type chartRendererSourceMetadata struct {
	Provider          string `json:"provider"`
	FixtureKey        string `json:"fixture_key"`
	CapturedAt        string `json:"captured_at"`
	AsOf              string `json:"as_of"`
	SourceSchema      string `json:"source_schema"`
	SourceURLRedacted bool   `json:"source_url_redacted"`
	StaleAfterDays    int    `json:"stale_after_days"`
	IsStale           bool   `json:"is_stale"`
	ReplayReady       bool   `json:"replay_ready"`
}

func TestFinRobotChartRendererLivePackageManifest(t *testing.T) {
	base := chartRendererLivePackageDir(t)
	manifest := loadChartRendererLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-chart-renderer-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-chart-renderer" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	if !strings.Contains(manifest.Credentials.Policy, "mplfinance") || !strings.Contains(manifest.Credentials.Policy, "matplotlib") {
		t.Fatalf("credential policy should name future renderer dependencies: %q", manifest.Credentials.Policy)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_chart_renderer_live_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}

	wantSources := []string{
		"finrobot/charting.py",
		"finrobot/chart_generator.py",
		"finrobot/enhanced_chart_generator.py",
		"mplfinance",
	}
	if !reflect.DeepEqual(manifest.SourceModules, wantSources) {
		t.Fatalf("source modules = %#v, want %#v", manifest.SourceModules, wantSources)
	}

	for _, key := range []string{"smoke", "chart_renderer_contract", "unsupported_renderer_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"chart_spec", "render_request", "render_result", "source_metadata", "renderer_skip"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertChartRendererJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "chart_spec", "render_request", "render_result", "unsupported_renderer"} {
		path := manifest.Fixtures[key]
		if path == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertChartRendererJSONFile(t, filepath.Join(base, path))
	}

	var moduleIDs []string
	for _, module := range manifest.Modules {
		moduleIDs = append(moduleIDs, module.ID)
		if module.ID == "" || module.SourceModule == "" || len(module.Capabilities) < 4 || len(module.OutputSchemas) == 0 {
			t.Fatalf("module metadata incomplete: %#v", module)
		}
	}
	sort.Strings(moduleIDs)
	wantModuleIDs := []string{"chart_generator", "charting", "enhanced_chart_generator", "mplfinance_adapter"}
	if !reflect.DeepEqual(moduleIDs, wantModuleIDs) {
		t.Fatalf("module ids = %#v, want %#v", moduleIDs, wantModuleIDs)
	}

	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"chart spec", "render request", "result", "snapshot", "stale-data", "unsupported renderer", "q/runtime"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotChartRendererContractsAndFixtureIndex(t *testing.T) {
	base := chartRendererLivePackageDir(t)
	manifest := loadChartRendererLiveManifest(t, base)

	var boundaryIDs []string
	for _, boundary := range manifest.RendererBoundaries {
		boundaryIDs = append(boundaryIDs, boundary.ID)
		if boundary.ID == "" || boundary.Capability == "" || boundary.FixtureKey == "" || boundary.Schema == "" {
			t.Fatalf("renderer boundary metadata incomplete: %#v", boundary)
		}
		if boundary.LiveNetwork || boundary.DependencyImported || boundary.CredentialRequired || !boundary.CleanSkip {
			t.Fatalf("renderer boundary must be fixture-only and clean-skip safe: %#v", boundary)
		}
		if !strings.HasPrefix(boundary.Capability, "finance.chart_renderer.renderer.") {
			t.Fatalf("%s capability = %q", boundary.ID, boundary.Capability)
		}
	}
	sort.Strings(boundaryIDs)
	wantBoundaryIDs := []string{"matplotlib", "mplfinance", "unsupported"}
	if !reflect.DeepEqual(boundaryIDs, wantBoundaryIDs) {
		t.Fatalf("renderer boundaries = %#v, want %#v", boundaryIDs, wantBoundaryIDs)
	}

	var contract struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Modules               []struct {
			ID             string   `json:"id"`
			SourceModule   string   `json:"source_module"`
			RequiredFields []string `json:"required_fields"`
		} `json:"modules"`
		TypedEnvelopes []struct {
			ID             string   `json:"id"`
			Schema         string   `json:"schema"`
			Fixture        string   `json:"fixture"`
			RequiredFields []string `json:"required_fields"`
		} `json:"typed_envelopes"`
		SourceMetadataContract struct {
			Schema          string   `json:"schema"`
			RequiredFields  []string `json:"required_fields"`
			RedactSourceURL bool     `json:"redact_source_url"`
			LiveNetwork     bool     `json:"live_network"`
		} `json:"source_metadata_contract"`
		StaleDataWarningContract struct {
			Required    bool   `json:"required"`
			WarningCode string `json:"warning_code"`
			Fixture     string `json:"fixture"`
		} `json:"stale_data_warning_contract"`
		SnapshotContract struct {
			Required      bool   `json:"required"`
			HashAlgorithm string `json:"hash_algorithm"`
			HashField     string `json:"hash_field"`
			Fixture       string `json:"fixture"`
		} `json:"snapshot_contract"`
		AcceptanceGates []string `json:"acceptance_gates"`
	}
	decodeChartRendererJSONFile(t, filepath.Join(base, "contracts", "chart_renderer_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || len(contract.Modules) != 4 || len(contract.TypedEnvelopes) != 3 {
		t.Fatalf("contract header/modules/envelopes = %#v", contract)
	}
	for _, envelope := range contract.TypedEnvelopes {
		if envelope.ID == "" || envelope.Schema == "" || envelope.Fixture == "" || len(envelope.RequiredFields) < 7 {
			t.Fatalf("typed envelope contract incomplete: %#v", envelope)
		}
		assertChartRendererJSONFile(t, filepath.Join(base, envelope.Schema))
		assertChartRendererJSONFile(t, filepath.Join(base, envelope.Fixture))
	}
	if contract.SourceMetadataContract.Schema == "" || !contract.SourceMetadataContract.RedactSourceURL || contract.SourceMetadataContract.LiveNetwork || len(contract.SourceMetadataContract.RequiredFields) < 9 {
		t.Fatalf("source metadata contract incomplete: %#v", contract.SourceMetadataContract)
	}
	if !contract.StaleDataWarningContract.Required || contract.StaleDataWarningContract.WarningCode != "STALE_SOURCE_DATA" || contract.StaleDataWarningContract.Fixture == "" {
		t.Fatalf("stale warning contract incomplete: %#v", contract.StaleDataWarningContract)
	}
	if !contract.SnapshotContract.Required || contract.SnapshotContract.HashAlgorithm != "sha256" || contract.SnapshotContract.HashField != "snapshot.hash" || contract.SnapshotContract.Fixture == "" {
		t.Fatalf("snapshot contract incomplete: %#v", contract.SnapshotContract)
	}
	acceptance := strings.ToLower(strings.Join(contract.AcceptanceGates, " "))
	for _, want := range []string{"typed envelopes", "live_network=false", "stale-data", "dimensions", "theme", "snapshot", "clean-skip"} {
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
	decodeChartRendererJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
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
		assertChartRendererJSONFile(t, filepath.Join(base, fixture.Path))
		assertChartRendererJSONFile(t, filepath.Join(base, fixture.Schema))
	}
}

func TestFinRobotChartRendererFixtures(t *testing.T) {
	base := chartRendererLivePackageDir(t)

	var spec struct {
		ProviderFree     bool                        `json:"provider_free"`
		LiveNetwork      bool                        `json:"live_network"`
		ChartID          string                      `json:"chart_id"`
		Symbol           string                      `json:"symbol"`
		ChartType        string                      `json:"chart_type"`
		Series           []map[string]any            `json:"series"`
		Dimensions       chartRendererDimensions     `json:"dimensions"`
		Theme            chartRendererTheme          `json:"theme"`
		SourceMetadata   chartRendererSourceMetadata `json:"source_metadata"`
		StaleDataWarning struct {
			Required    bool   `json:"required"`
			WarningCode string `json:"warning_code"`
			Message     string `json:"message"`
		} `json:"stale_data_warning"`
	}
	decodeChartRendererJSONFile(t, filepath.Join(base, "fixtures", "chart_spec_ACME_candlestick_fixture.json"), &spec)
	if !spec.ProviderFree || spec.LiveNetwork || spec.ChartID == "" || spec.Symbol != "ACME" || spec.ChartType != "candlestick" || len(spec.Series) != 3 {
		t.Fatalf("chart spec fixture header/count = %#v", spec)
	}
	assertChartRendererDimensions(t, spec.Dimensions)
	assertChartRendererTheme(t, spec.Theme)
	assertChartRendererSourceMetadata(t, spec.SourceMetadata, true)
	if !spec.StaleDataWarning.Required || spec.StaleDataWarning.WarningCode != "STALE_SOURCE_DATA" || spec.StaleDataWarning.Message == "" {
		t.Fatalf("stale warning incomplete: %#v", spec.StaleDataWarning)
	}

	var request struct {
		ProviderFree          bool                    `json:"provider_free"`
		LiveNetwork           bool                    `json:"live_network"`
		RealDependencyImports bool                    `json:"real_dependency_imports"`
		RequestID             string                  `json:"request_id"`
		Renderer              string                  `json:"renderer"`
		OutputFormat          string                  `json:"output_format"`
		Dimensions            chartRendererDimensions `json:"dimensions"`
		Theme                 chartRendererTheme      `json:"theme"`
		ChartSpecRef          string                  `json:"chart_spec_ref"`
		ChartSpec             map[string]any          `json:"chart_spec"`
	}
	decodeChartRendererJSONFile(t, filepath.Join(base, "fixtures", "render_request_ACME_candlestick_fixture.json"), &request)
	if !request.ProviderFree || request.LiveNetwork || request.RealDependencyImports || request.RequestID == "" || request.Renderer != "mplfinance" || request.OutputFormat != "svg" || request.ChartSpecRef == "" || len(request.ChartSpec) == 0 {
		t.Fatalf("render request fixture incomplete: %#v", request)
	}
	assertChartRendererDimensions(t, request.Dimensions)
	assertChartRendererTheme(t, request.Theme)

	var result struct {
		ProviderFree          bool                    `json:"provider_free"`
		LiveNetwork           bool                    `json:"live_network"`
		RealDependencyImports bool                    `json:"real_dependency_imports"`
		RequestID             string                  `json:"request_id"`
		OK                    bool                    `json:"ok"`
		Renderer              string                  `json:"renderer"`
		Artifact              map[string]any          `json:"artifact"`
		Dimensions            chartRendererDimensions `json:"dimensions"`
		Theme                 chartRendererTheme      `json:"theme"`
		Snapshot              struct {
			HashAlgorithm string   `json:"hash_algorithm"`
			Hash          string   `json:"hash"`
			FixtureKey    string   `json:"fixture_key"`
			Inputs        []string `json:"deterministic_inputs"`
		} `json:"snapshot"`
		Warnings []struct {
			WarningCode string `json:"warning_code"`
			Severity    string `json:"severity"`
			Message     string `json:"message"`
		} `json:"warnings"`
		SourceMetadata chartRendererSourceMetadata `json:"source_metadata"`
	}
	decodeChartRendererJSONFile(t, filepath.Join(base, "fixtures", "render_result_ACME_candlestick_snapshot_fixture.json"), &result)
	if !result.ProviderFree || result.LiveNetwork || result.RealDependencyImports || !result.OK || result.Renderer != "mplfinance" || result.RequestID != request.RequestID {
		t.Fatalf("render result fixture header = %#v", result)
	}
	if result.Artifact["format"] != "svg" || result.Artifact["mime_type"] != "image/svg+xml" || result.Artifact["storage_uri"] == "" {
		t.Fatalf("artifact incomplete: %#v", result.Artifact)
	}
	assertChartRendererDimensions(t, result.Dimensions)
	assertChartRendererTheme(t, result.Theme)
	if result.Snapshot.HashAlgorithm != "sha256" || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(result.Snapshot.Hash) || result.Snapshot.FixtureKey == "" || len(result.Snapshot.Inputs) < 5 {
		t.Fatalf("snapshot incomplete: %#v", result.Snapshot)
	}
	if len(result.Warnings) == 0 || result.Warnings[0].WarningCode != "STALE_SOURCE_DATA" {
		t.Fatalf("missing stale source warning: %#v", result.Warnings)
	}
	assertChartRendererSourceMetadata(t, result.SourceMetadata, true)
}

func TestFinRobotChartRendererUnsupportedRendererCleanSkip(t *testing.T) {
	base := chartRendererLivePackageDir(t)

	var contract struct {
		ProviderFree               bool     `json:"provider_free"`
		LiveNetwork                bool     `json:"live_network"`
		RealDependencyImports      bool     `json:"real_dependency_imports"`
		CleanSkipWithoutDependency bool     `json:"clean_skip_without_dependency"`
		Renderer                   string   `json:"renderer"`
		SkipSchema                 string   `json:"skip_schema"`
		SkipFixture                string   `json:"skip_fixture"`
		FallbackRules              []string `json:"fallback_rules"`
		AcceptanceGates            []string `json:"acceptance_gates"`
	}
	decodeChartRendererJSONFile(t, filepath.Join(base, "contracts", "unsupported_renderer_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || !contract.CleanSkipWithoutDependency || contract.Renderer != "plotly" || contract.SkipSchema == "" || contract.SkipFixture == "" {
		t.Fatalf("unsupported renderer contract incomplete: %#v", contract)
	}
	joined := strings.ToLower(strings.Join(append(contract.FallbackRules, contract.AcceptanceGates...), " "))
	for _, want := range []string{"clean-skip", "do not import", "q/runtime", "network"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("unsupported renderer contract missing %q: %s", want, joined)
		}
	}

	var skip struct {
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		RequestID             string `json:"request_id"`
		OK                    bool   `json:"ok"`
		Skip                  bool   `json:"skip"`
		Renderer              string `json:"renderer"`
		UnsupportedReason     string `json:"unsupported_reason"`
		DependencyImported    bool   `json:"dependency_imported"`
		CleanSkip             bool   `json:"clean_skip"`
		FixtureKey            string `json:"fixture_key"`
	}
	decodeChartRendererJSONFile(t, filepath.Join(base, "fixtures", "unsupported_renderer_clean_skip_fixture.json"), &skip)
	if !skip.ProviderFree || skip.LiveNetwork || skip.RealDependencyImports || skip.OK || !skip.Skip || skip.Renderer != "plotly" || skip.UnsupportedReason == "" || skip.DependencyImported || !skip.CleanSkip || skip.FixtureKey == "" || skip.RequestID == "" {
		t.Fatalf("unsupported renderer clean skip fixture incomplete: %#v", skip)
	}
}

func TestFinRobotChartRendererLivePackageNoLiveImports(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(chartRendererLivePackageDir(t), "main.leia"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pattern := range []string{
		`(?m)^\s*import\s+`,
		`(?m)^\s*use\s+`,
		`(?m)^\s*load\s*\(`,
		`(?m)^\s*require\s*\(`,
		`(?m)^\s*(mplfinance|matplotlib|plotly|selenium|requests|http|pandas|numpy|q|runtime)\s*[.(]`,
	} {
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("main.leia contains live dependency loader matching %q", pattern)
		}
	}
}

func TestFinRobotChartRendererLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(chartRendererLivePackageDir(t), "main.leia")

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
			got, err := vm.Get("chart_renderer_live_package_summary")
			if err != nil {
				t.Fatalf("Get chart_renderer_live_package_summary: %v", err)
			}
			want := "chart_renderer_live_package modules=4 renderers=3 schemas=5 fixtures=4 provider_free=true live_network=false imports=false clean_skip=true"
			if got != want {
				t.Fatalf("chart_renderer_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

type chartRendererDimensions struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	DPI    int `json:"dpi"`
}

type chartRendererTheme struct {
	Name       string `json:"name"`
	Background string `json:"background"`
	UpColor    string `json:"up_color"`
	DownColor  string `json:"down_color"`
}

func assertChartRendererDimensions(t *testing.T, dimensions chartRendererDimensions) {
	t.Helper()
	if dimensions.Width <= 0 || dimensions.Height <= 0 || dimensions.DPI <= 0 {
		t.Fatalf("dimensions incomplete: %#v", dimensions)
	}
}

func assertChartRendererTheme(t *testing.T, theme chartRendererTheme) {
	t.Helper()
	if theme.Name == "" || theme.Background == "" || theme.UpColor == "" || theme.DownColor == "" {
		t.Fatalf("theme incomplete: %#v", theme)
	}
}

func assertChartRendererSourceMetadata(t *testing.T, metadata chartRendererSourceMetadata, wantStale bool) {
	t.Helper()
	if metadata.Provider == "" || metadata.FixtureKey == "" || metadata.CapturedAt == "" || metadata.AsOf == "" || metadata.SourceSchema == "" || !metadata.SourceURLRedacted || metadata.StaleAfterDays <= 0 || metadata.IsStale != wantStale || !metadata.ReplayReady {
		t.Fatalf("source metadata incomplete: %#v", metadata)
	}
}

func chartRendererLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "chart_renderer")
}

func loadChartRendererLiveManifest(t *testing.T, base string) chartRendererLiveManifest {
	t.Helper()
	var manifest chartRendererLiveManifest
	decodeChartRendererJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertChartRendererJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeChartRendererJSONFile(t, path, &value)
}

func decodeChartRendererJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
