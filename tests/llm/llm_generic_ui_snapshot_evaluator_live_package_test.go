package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericUISnapshotEvaluatorLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericUISnapshotEvaluatorPackageDir(t)
	var manifest struct {
		SchemaVersion          int               `json:"schema_version"`
		ID                     string            `json:"id"`
		PackageName            string            `json:"package_name"`
		PackageBoundaryID      string            `json:"package_boundary_id"`
		CapabilityID           string            `json:"capability_id"`
		ProviderFree           bool              `json:"provider_free"`
		DomainSpecific         bool              `json:"domain_specific"`
		LiveNetworkDefault     bool              `json:"live_network_default"`
		LiveModelDefault       bool              `json:"live_model_default"`
		DependsOnQRuntime      bool              `json:"depends_on_q_runtime"`
		BrowserRequiredDefault bool              `json:"browser_required_default"`
		Capabilities           []string          `json:"capabilities"`
		Contracts              map[string]string `json:"contracts"`
		Schemas                map[string]string `json:"schemas"`
		Fixtures               map[string]string `json:"fixtures"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	if manifest.SchemaVersion != 1 || manifest.ID != "generic-ui-snapshot-evaluator" ||
		manifest.PackageName != "leia-generic-ai-ui-snapshot-evaluator" ||
		manifest.PackageBoundaryID != "generic-ai-ui-snapshot-evaluator" ||
		manifest.CapabilityID != "generic.ai.ui.snapshot.evaluator" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetworkDefault ||
		manifest.LiveModelDefault || manifest.DependsOnQRuntime || manifest.BrowserRequiredDefault {
		t.Fatalf("manifest must stay provider-free/generic/offline/browser-free: %#v", manifest)
	}
	for _, want := range []string{"generic.ai.ui.snapshot.evaluator", "generic.ai.ui.route_dom_schema", "generic.ai.ui.viewport_matrix", "generic.ai.ui.visual_diff_budget", "generic.ai.ui.accessibility_summary", "generic.ai.ui.artifact_uri_manifest", "generic.ai.ui.redaction_policy", "generic.ai.ui.static_asset_policy", "generic.ai.ui.browser_clean_skip"} {
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
		RequiresBrowser       bool              `json:"requires_browser"`
		FieldContracts        map[string]string `json:"field_contracts"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Contracts["contract"]), &contract)
	if contract.SchemaVersion != 1 || contract.PackageBoundaryID != manifest.PackageBoundaryID ||
		contract.PackageName != "generic.ai.ui.snapshot.evaluator" || contract.Entrypoint != "ai.ui.snapshot_evaluator" ||
		!contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork ||
		contract.LiveModelCalls || contract.RealDependencyImports || contract.RequiresBrowser {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	for _, want := range []string{"route_dom_snapshots", "viewport_matrix", "visual_diff_budgets", "accessibility_summaries", "artifact_uri_manifest", "redaction_policy", "static_asset_policy", "browser_clean_skip"} {
		if contract.FieldContracts[want] == "" {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
		}
	}
}

func TestGenericUISnapshotEvaluatorLivePackageFixtureShape(t *testing.T) {
	base := genericUISnapshotEvaluatorPackageDir(t)
	fixture := loadGenericUISnapshotEvaluatorFixture(t, filepath.Join(base, "fixtures", "generic_ui_snapshot_evaluator_fixture.json"))
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.LiveModelCalls || fixture.RequiresBrowser || fixture.RequiresCredentials {
		t.Fatalf("fixture must stay provider-free and offline: %#v", fixture)
	}
	if len(fixture.RouteDOMSnapshots) != 2 || len(fixture.ViewportMatrix) != 3 || len(fixture.VisualDiffBudgets) != 3 ||
		len(fixture.AccessibilitySummaries) != 2 || len(fixture.ArtifactURIManifest.Artifacts) != 3 || len(fixture.AdapterBoundaries) != 2 {
		t.Fatalf("fixture counts drifted: routes=%d viewports=%d budgets=%d accessibility=%d artifacts=%d adapters=%d",
			len(fixture.RouteDOMSnapshots), len(fixture.ViewportMatrix), len(fixture.VisualDiffBudgets), len(fixture.AccessibilitySummaries), len(fixture.ArtifactURIManifest.Artifacts), len(fixture.AdapterBoundaries))
	}
	routes := map[string]bool{}
	for _, route := range fixture.RouteDOMSnapshots {
		if route.RouteID == "" || route.Route == "" || route.Method == "" || route.FixtureURL == "" || route.DOMSchema.RootSelector == "" || len(route.SnapshotRefs) == 0 || len(route.ArtifactRefs) == 0 {
			t.Fatalf("route DOM snapshot incomplete: %#v", route)
		}
		routes[route.RouteID] = true
	}
	viewports := map[string]bool{}
	for _, viewport := range fixture.ViewportMatrix {
		if viewport.ID == "" || viewport.Width <= 0 || viewport.Height <= 0 || viewport.DeviceScaleFactor <= 0 {
			t.Fatalf("viewport incomplete: %#v", viewport)
		}
		viewports[viewport.ID] = true
	}
	for _, budget := range fixture.VisualDiffBudgets {
		if !routes[budget.RouteID] || !viewports[budget.ViewportID] || budget.MaxChangedPixelsRatio <= 0 || budget.MaxLayoutShiftPx < 0 || budget.TextOverflowAllowed || budget.MissingRegionAllowed {
			t.Fatalf("visual diff budget invalid or unresolved: %#v", budget)
		}
	}
	for _, summary := range fixture.AccessibilitySummaries {
		if !routes[summary.RouteID] || summary.Mode == "" || len(summary.Standards) == 0 || len(summary.RequiredChecks) == 0 || summary.ViolationsAllowed != 0 || summary.ManualReviewRequired {
			t.Fatalf("accessibility summary invalid or unresolved: %#v", summary)
		}
	}
	if fixture.ArtifactURIManifest.ExternalFetch || fixture.ArtifactURIManifest.URIPolicy != "fixture_uri_only" ||
		!genericLivePackageContains(fixture.ArtifactURIManifest.AllowedSchemes, "fixture") ||
		!genericLivePackageContains(fixture.ArtifactURIManifest.AllowedSchemes, "artifact") {
		t.Fatalf("artifact URI policy must stay fixture/artifact only: %#v", fixture.ArtifactURIManifest)
	}
	if fixture.StaticAssetPolicy.ExternalFetch || fixture.StaticAssetPolicy.RemoteFonts || !fixture.StaticAssetPolicy.InlineOrLocalOnly || fixture.StaticAssetPolicy.NetworkRequired {
		t.Fatalf("static asset policy must forbid remote dependencies: %#v", fixture.StaticAssetPolicy)
	}
	if !fixture.RedactionPolicy.AppliesBeforeHashing || fixture.RedactionPolicy.SecretEnvPatternsAllowed || len(fixture.RedactionPolicy.ForbiddenOutputPatterns) == 0 {
		t.Fatalf("redaction policy incomplete: %#v", fixture.RedactionPolicy)
	}
	for _, boundary := range fixture.AdapterBoundaries {
		if boundary.DependencyImported || boundary.CredentialRequired || boundary.LiveNetwork || !boundary.CleanSkip {
			t.Fatalf("adapter boundary must clean-skip: %#v", boundary)
		}
	}
}

func TestGenericUISnapshotEvaluatorLivePackageIsDomainNeutral(t *testing.T) {
	base := genericUISnapshotEvaluatorPackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"finrobot", "acme", "aapl", "ticker", "equity", "investment", "valuation_engine", "target_price", "dcf", "sec.gov", "10-k", "finance.", "product.workflow"} {
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

func TestGenericUISnapshotEvaluatorLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericUISnapshotEvaluatorPackageDir(t)
	schema := filepath.Join(base, "schemas", "generic_ui_snapshot_evaluator_v1.schema.json")
	assertDocumentPipelineSchemaRequired(t, schema, []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "live_model_calls", "requires_browser", "requires_credentials", "route_dom_snapshots", "viewport_matrix", "visual_diff_budgets", "accessibility_summaries", "artifact_uri_manifest", "static_asset_policy", "redaction_policy", "adapter_boundaries"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "route_dom_snapshots", "items"}, []string{"route_id", "route", "method", "fixture_url", "dom_schema", "snapshot_refs", "artifact_refs"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "viewport_matrix", "items"}, []string{"id", "width", "height", "device_scale_factor", "color_scheme", "reduced_motion"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "visual_diff_budgets", "items"}, []string{"route_id", "viewport_id", "max_changed_pixels_ratio", "max_layout_shift_px", "text_overflow_allowed", "missing_region_allowed"})
}

func TestGenericUISnapshotEvaluatorLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericUISnapshotEvaluatorPackageDir(t), "main.leia")
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
			got, err := vm.Get("generic_ui_snapshot_evaluator_live_package_summary")
			if err != nil {
				t.Fatalf("Get summary: %v", err)
			}
			want := "generic_ui_snapshot_evaluator_live_package capability=generic.ai.ui.snapshot.evaluator entrypoint=ai.ui.snapshot_evaluator routes=2 viewports=3 budgets=3 accessibility=2 artifacts=3 clean_skip=2 provider_free=true live_network=false imports=false model_calls=false browser=false"
			if got != want {
				t.Fatalf("summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

type genericUISnapshotEvaluatorFixture struct {
	ProviderFree          bool `json:"provider_free"`
	LiveNetwork           bool `json:"live_network"`
	RealDependencyImports bool `json:"real_dependency_imports"`
	LiveModelCalls        bool `json:"live_model_calls"`
	RequiresBrowser       bool `json:"requires_browser"`
	RequiresCredentials   bool `json:"requires_credentials"`
	RouteDOMSnapshots     []struct {
		RouteID    string `json:"route_id"`
		Route      string `json:"route"`
		Method     string `json:"method"`
		FixtureURL string `json:"fixture_url"`
		DOMSchema  struct {
			RootSelector string `json:"root_selector"`
		} `json:"dom_schema"`
		SnapshotRefs []string `json:"snapshot_refs"`
		ArtifactRefs []string `json:"artifact_refs"`
	} `json:"route_dom_snapshots"`
	ViewportMatrix []struct {
		ID                string  `json:"id"`
		Width             int     `json:"width"`
		Height            int     `json:"height"`
		DeviceScaleFactor float64 `json:"device_scale_factor"`
	} `json:"viewport_matrix"`
	VisualDiffBudgets []struct {
		RouteID               string  `json:"route_id"`
		ViewportID            string  `json:"viewport_id"`
		MaxChangedPixelsRatio float64 `json:"max_changed_pixels_ratio"`
		MaxLayoutShiftPx      int     `json:"max_layout_shift_px"`
		TextOverflowAllowed   bool    `json:"text_overflow_allowed"`
		MissingRegionAllowed  bool    `json:"missing_region_allowed"`
	} `json:"visual_diff_budgets"`
	AccessibilitySummaries []struct {
		RouteID              string   `json:"route_id"`
		Mode                 string   `json:"mode"`
		Standards            []string `json:"standards"`
		RequiredChecks       []string `json:"required_checks"`
		ViolationsAllowed    int      `json:"violations_allowed"`
		ManualReviewRequired bool     `json:"manual_review_required"`
	} `json:"accessibility_summaries"`
	ArtifactURIManifest struct {
		URIPolicy      string   `json:"uri_policy"`
		ExternalFetch  bool     `json:"external_fetch"`
		AllowedSchemes []string `json:"allowed_schemes"`
		Artifacts      []struct {
			ID string `json:"id"`
		} `json:"artifacts"`
	} `json:"artifact_uri_manifest"`
	StaticAssetPolicy struct {
		ExternalFetch     bool `json:"external_fetch"`
		RemoteFonts       bool `json:"remote_fonts"`
		InlineOrLocalOnly bool `json:"inline_or_local_only"`
		NetworkRequired   bool `json:"network_required"`
	} `json:"static_asset_policy"`
	RedactionPolicy struct {
		AppliesBeforeHashing     bool     `json:"applies_before_hashing"`
		SecretEnvPatternsAllowed bool     `json:"secret_env_patterns_allowed"`
		ForbiddenOutputPatterns  []string `json:"forbidden_output_patterns"`
	} `json:"redaction_policy"`
	AdapterBoundaries []struct {
		DependencyImported bool `json:"dependency_imported"`
		CredentialRequired bool `json:"credential_required"`
		LiveNetwork        bool `json:"live_network"`
		CleanSkip          bool `json:"clean_skip"`
	} `json:"adapter_boundaries"`
}

func loadGenericUISnapshotEvaluatorFixture(t *testing.T, path string) genericUISnapshotEvaluatorFixture {
	t.Helper()
	var fixture genericUISnapshotEvaluatorFixture
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func genericUISnapshotEvaluatorPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_ui_snapshot_evaluator")
}
