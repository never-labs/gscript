package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericChartRenderContractsLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericChartRenderContractsPackageDir(t)
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
		Schemas            map[string]string `json:"schemas"`
		Fixtures           map[string]string `json:"fixtures"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	if manifest.SchemaVersion != 1 || manifest.ID != "generic-chart-render-contracts" ||
		manifest.PackageName != "leia-generic-ai-chart-render-contracts" ||
		manifest.PackageBoundaryID != "generic-ai-chart-render-contracts" ||
		manifest.CapabilityID != "generic.ai.chart.render.contracts" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetworkDefault ||
		manifest.LiveModelDefault || manifest.DependsOnQRuntime || manifest.CredentialRequired {
		t.Fatalf("manifest must stay provider-free/generic/offline/credential-free: %#v", manifest)
	}
	for _, want := range []string{"generic.ai.chart.render.contracts", "generic.ai.chart.spec", "generic.ai.chart.recipe.matrix", "generic.ai.chart.render.request", "generic.ai.chart.render.result", "generic.ai.chart.source.metadata", "generic.ai.chart.snapshot.hash", "generic.ai.chart.renderer.clean_skip", "generic.ai.chart.unsupported_renderer"} {
		if !genericLivePackageContains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}

	var contract struct {
		SchemaVersion         int               `json:"schema_version"`
		PackageBoundaryID     string            `json:"package_boundary_id"`
		PackageName           string            `json:"package_name"`
		Entrypoint            string            `json:"entrypoint"`
		ProviderFree          bool              `json:"provider_free"`
		DomainSpecific        bool              `json:"domain_specific"`
		LiveNetwork           bool              `json:"live_network"`
		LiveModelCalls        bool              `json:"live_model_calls"`
		RealDependencyImports bool              `json:"real_dependency_imports"`
		RequiresCredentials   bool              `json:"requires_credentials"`
		FieldContracts        map[string]string `json:"field_contracts"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Contracts["contract"]), &contract)
	if contract.SchemaVersion != 1 || contract.PackageBoundaryID != manifest.PackageBoundaryID ||
		contract.PackageName != "generic.ai.chart.render.contracts" || contract.Entrypoint != "ai.chart.render_contracts" ||
		!contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork ||
		contract.LiveModelCalls || contract.RealDependencyImports || contract.RequiresCredentials {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	for _, want := range []string{"chart_spec", "recipe_matrix", "render_request", "render_result", "source_metadata", "unsupported_renderer"} {
		if contract.FieldContracts[want] == "" {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
		}
	}
}

func TestGenericChartRenderContractsLivePackageFixtureShape(t *testing.T) {
	base := genericChartRenderContractsPackageDir(t)
	fixture := loadGenericChartRenderContractsFixture(t, filepath.Join(base, "fixtures", "generic_chart_render_contracts_fixture.json"))
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.LiveModelCalls {
		t.Fatalf("fixture must stay provider-free and offline: %#v", fixture)
	}
	if len(fixture.ChartSpecs) != 2 || len(fixture.RecipeMatrix.Recipes) != 3 ||
		len(fixture.RenderRequests) != 2 || len(fixture.RenderResults) != 2 ||
		len(fixture.RendererSkips) != 1 || len(fixture.AdapterBoundaries) != 2 {
		t.Fatalf("fixture counts drifted: specs=%d recipes=%d requests=%d results=%d skips=%d adapters=%d",
			len(fixture.ChartSpecs), len(fixture.RecipeMatrix.Recipes), len(fixture.RenderRequests), len(fixture.RenderResults), len(fixture.RendererSkips), len(fixture.AdapterBoundaries))
	}
	specs := map[string]string{}
	sources := map[string]bool{}
	for _, spec := range fixture.ChartSpecs {
		if spec.ChartID == "" || spec.ChartType == "" || len(spec.Series) == 0 ||
			spec.Dimensions.Width <= 0 || spec.Dimensions.Height <= 0 ||
			spec.SourceMetadata.Provider != "fixture" || spec.SourceMetadata.FixtureKey == "" ||
			!spec.SourceMetadata.ReplayReady {
			t.Fatalf("chart spec incomplete or not replay-ready: %#v", spec)
		}
		specs[spec.ChartID] = spec.SourceMetadata.FixtureKey
		sources[spec.SourceMetadata.FixtureKey] = true
	}
	requests := map[string]string{}
	for _, req := range fixture.RenderRequests {
		if req.RequestID == "" || req.Renderer == "" || req.OutputFormat == "" ||
			req.Dimensions.Width <= 0 || req.Dimensions.Height <= 0 ||
			req.LiveNetwork || req.RealDependencyImports || specs[req.ChartSpecRef] == "" {
			t.Fatalf("render request invalid or unresolved: %#v", req)
		}
		requests[req.RequestID] = req.ChartSpecRef
	}
	skips := map[string]genericChartRenderSkip{}
	for _, skip := range fixture.RendererSkips {
		if skip.ID == "" || skip.Renderer == "" || skip.DependencyImported || skip.CredentialRequired || skip.LiveNetwork || !skip.CleanSkip {
			t.Fatalf("renderer skip must clean-skip without live dependencies: %#v", skip)
		}
		skips[skip.ID] = skip
	}
	for _, result := range fixture.RenderResults {
		if requests[result.RequestID] == "" || result.Renderer == "" ||
			result.Snapshot.HashAlgorithm != "sha256" || result.Snapshot.Hash == "" ||
			len(result.Snapshot.DeterministicInputs) == 0 || !sources[result.SourceMetadataRef] {
			t.Fatalf("render result invalid or unresolved: %#v", result)
		}
		if !result.OK {
			if result.Artifact != nil {
				t.Fatalf("clean-skip result must not create artifact: %#v", result)
			}
			for _, warning := range result.Warnings {
				if !skips[warning].CleanSkip {
					t.Fatalf("result warning %q does not resolve to clean skip", warning)
				}
			}
		}
	}
	for _, boundary := range fixture.AdapterBoundaries {
		if boundary.DependencyImported || boundary.CredentialRequired || boundary.LiveNetwork || !boundary.CleanSkip {
			t.Fatalf("adapter boundary must clean-skip: %#v", boundary)
		}
	}
}

func TestGenericChartRenderContractsLivePackageIsDomainNeutral(t *testing.T) {
	base := genericChartRenderContractsPackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"finrobot", "acme", "aapl", "ticker", "equity", "investment", "valuation_engine", "target_price", "dcf", "sec.gov", "10-k", "finance.", "mplfinance", "matplotlib", "plotly"} {
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

func TestGenericChartRenderContractsLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericChartRenderContractsPackageDir(t)
	schema := filepath.Join(base, "schemas", "generic_chart_render_contracts_v1.schema.json")
	assertDocumentPipelineSchemaRequired(t, schema, []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "live_model_calls", "chart_specs", "recipe_matrix", "render_requests", "render_results", "renderer_skips", "adapter_boundaries"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "chart_specs", "items"}, []string{"chart_id", "chart_type", "series", "dimensions", "theme", "source_metadata", "stale_data_warning"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "render_requests", "items"}, []string{"request_id", "renderer", "output_format", "dimensions", "theme", "chart_spec_ref", "live_network", "real_dependency_imports"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "render_results", "items"}, []string{"request_id", "ok", "renderer", "artifact", "snapshot", "warnings", "source_metadata_ref"})
}

func TestGenericChartRenderContractsLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericChartRenderContractsPackageDir(t), "main.leia")
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
			got, err := vm.Get("generic_chart_render_contracts_live_package_summary")
			if err != nil {
				t.Fatalf("Get summary: %v", err)
			}
			want := "generic_chart_render_contracts_live_package capability=generic.ai.chart.render.contracts entrypoint=ai.chart.render_contracts specs=2 recipes=3 requests=2 results=2 skips=1 clean_skip=2 provider_free=true live_network=false imports=false model_calls=false"
			if got != want {
				t.Fatalf("summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

type genericChartRenderContractsFixture struct {
	ProviderFree          bool                     `json:"provider_free"`
	LiveNetwork           bool                     `json:"live_network"`
	RealDependencyImports bool                     `json:"real_dependency_imports"`
	LiveModelCalls        bool                     `json:"live_model_calls"`
	ChartSpecs            []genericChartRenderSpec `json:"chart_specs"`
	RecipeMatrix          struct {
		Recipes []struct {
			Family    string `json:"family"`
			ChartType string `json:"chart_type"`
			SpecHash  string `json:"spec_hash"`
		} `json:"recipes"`
	} `json:"recipe_matrix"`
	RenderRequests []struct {
		RequestID             string `json:"request_id"`
		Renderer              string `json:"renderer"`
		OutputFormat          string `json:"output_format"`
		ChartSpecRef          string `json:"chart_spec_ref"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		Dimensions            struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"dimensions"`
	} `json:"render_requests"`
	RenderResults     []genericChartRenderResult `json:"render_results"`
	RendererSkips     []genericChartRenderSkip   `json:"renderer_skips"`
	AdapterBoundaries []struct {
		ID                 string `json:"id"`
		Capability         string `json:"capability"`
		DependencyImported bool   `json:"dependency_imported"`
		CredentialRequired bool   `json:"credential_required"`
		LiveNetwork        bool   `json:"live_network"`
		CleanSkip          bool   `json:"clean_skip"`
	} `json:"adapter_boundaries"`
}

type genericChartRenderSpec struct {
	ChartID    string `json:"chart_id"`
	ChartType  string `json:"chart_type"`
	Series     []any  `json:"series"`
	Dimensions struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"dimensions"`
	SourceMetadata struct {
		Provider    string `json:"provider"`
		FixtureKey  string `json:"fixture_key"`
		IsStale     bool   `json:"is_stale"`
		ReplayReady bool   `json:"replay_ready"`
	} `json:"source_metadata"`
	StaleDataWarning *struct {
		Code     string `json:"code"`
		Severity string `json:"severity"`
	} `json:"stale_data_warning"`
}

type genericChartRenderResult struct {
	RequestID string `json:"request_id"`
	OK        bool   `json:"ok"`
	Renderer  string `json:"renderer"`
	Artifact  *struct {
		ArtifactID string `json:"artifact_id"`
		URI        string `json:"uri"`
		MediaType  string `json:"media_type"`
		Hash       string `json:"hash"`
	} `json:"artifact"`
	Snapshot struct {
		HashAlgorithm       string   `json:"hash_algorithm"`
		Hash                string   `json:"hash"`
		DeterministicInputs []string `json:"deterministic_inputs"`
	} `json:"snapshot"`
	Warnings          []string `json:"warnings"`
	SourceMetadataRef string   `json:"source_metadata_ref"`
}

type genericChartRenderSkip struct {
	ID                 string `json:"id"`
	Renderer           string `json:"renderer"`
	DependencyImported bool   `json:"dependency_imported"`
	CredentialRequired bool   `json:"credential_required"`
	LiveNetwork        bool   `json:"live_network"`
	CleanSkip          bool   `json:"clean_skip"`
}

func loadGenericChartRenderContractsFixture(t *testing.T, path string) genericChartRenderContractsFixture {
	t.Helper()
	var fixture genericChartRenderContractsFixture
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func genericChartRenderContractsPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_chart_render_contracts")
}
