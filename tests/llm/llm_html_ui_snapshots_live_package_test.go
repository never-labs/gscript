package leia_test

import (
	"crypto/sha256"
	"encoding/hex"
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

type htmlUISnapshotsManifest struct {
	SchemaVersion               int      `json:"schema_version"`
	ID                          string   `json:"id"`
	PackageName                 string   `json:"package_name"`
	ProviderFree                bool     `json:"provider_free"`
	LiveNetworkDefault          bool     `json:"live_network_default"`
	RealDependencyImportDefault bool     `json:"real_dependency_import_default"`
	BrowserRequiredDefault      bool     `json:"browser_required_default"`
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
		BrowserRequired             bool   `json:"browser_required"`
		CleanSkipWithoutBrowser     bool   `json:"clean_skip_without_browser"`
		FixtureHook                 string `json:"fixture_hook"`
	} `json:"default_policy"`
	Entrypoints        map[string]string        `json:"entrypoints"`
	Schemas            map[string]string        `json:"schemas"`
	Fixtures           map[string]string        `json:"fixtures"`
	Capabilities       []string                 `json:"capabilities"`
	TemplateBoundaries []htmlUITemplateBoundary `json:"template_boundaries"`
	TestGates          []string                 `json:"test_gates"`
}

type htmlUITemplateBoundary struct {
	ID                 string `json:"id"`
	SourceModule       string `json:"source_module"`
	Capability         string `json:"capability"`
	LiveNetwork        bool   `json:"live_network"`
	BrowserRequired    bool   `json:"browser_required"`
	CredentialRequired bool   `json:"credential_required"`
	CleanSkip          bool   `json:"clean_skip"`
}

func TestFinRobotHTMLUISnapshotsManifestAndContract(t *testing.T) {
	base := htmlUISnapshotsLivePackageDir(t)
	manifest := loadHTMLUISnapshotsManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-html-ui-snapshots-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-html-ui-snapshots" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault || manifest.BrowserRequiredDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v browser:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault, manifest.BrowserRequiredDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("HTML/UI snapshots must not declare credentials: %#v", manifest.Credentials)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		manifest.DefaultPolicy.BrowserRequired ||
		!manifest.DefaultPolicy.CleanSkipWithoutBrowser ||
		manifest.DefaultPolicy.FixtureHook != "recorded_html_ui_snapshot_fixture" {
		t.Fatalf("default policy must stay fixture-only and browser-free: %#v", manifest.DefaultPolicy)
	}

	wantSources := []string{
		"finrobot_equity/web_app/templates/*",
		"finrobot_equity/web_app/static/*",
		"finrobot_equity/core/src/modules/html_template_professional.py",
	}
	if !reflect.DeepEqual(manifest.SourceModules, wantSources) {
		t.Fatalf("source modules = %#v, want %#v", manifest.SourceModules, wantSources)
	}
	for _, key := range []string{"smoke", "html_ui_snapshot_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"template_inventory", "static_asset_manifest", "snapshot_metadata", "accessibility_checklist", "ui_snapshot_evaluation"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertHTMLUIJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "template_inventory", "snapshot_contract", "static_asset_manifest", "ui_snapshot_evaluation"} {
		path := manifest.Fixtures[key]
		if path == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertHTMLUIJSONFile(t, filepath.Join(base, path))
	}
	for _, want := range []string{
		"ui.snapshot.route_dom_schema",
		"ui.snapshot.viewport_matrix",
		"ui.snapshot.visual_diff_budget",
		"ui.snapshot.accessibility_summary",
		"ui.snapshot.artifact_uri_manifest",
		"ui.snapshot.redaction_policy",
		"finance.html_ui.template_inventory",
		"finance.html_ui.required_sections",
		"finance.html_ui.table_placeholder_contract",
		"finance.html_ui.chart_placeholder_contract",
		"finance.html_ui.disclosure_markup",
		"finance.html_ui.source_provenance_markup",
		"finance.html_ui.accessibility_static_checklist",
		"finance.html_ui.static_asset_manifest",
		"finance.html_ui.deterministic_snapshot_hash",
		"finance.html_ui.provider_free_clean_skip",
	} {
		if !contains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}
	if len(manifest.TemplateBoundaries) != 3 {
		t.Fatalf("template boundaries = %#v", manifest.TemplateBoundaries)
	}
	for _, boundary := range manifest.TemplateBoundaries {
		if boundary.ID == "" || boundary.SourceModule == "" || boundary.Capability == "" {
			t.Fatalf("template boundary incomplete: %#v", boundary)
		}
		if boundary.LiveNetwork || boundary.BrowserRequired || boundary.CredentialRequired || !boundary.CleanSkip {
			t.Fatalf("template boundary must be provider-free/browser-free clean skip: %#v", boundary)
		}
	}
	gates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"provider-free", "template inventory", "required sections", "table", "chart", "disclosure", "source provenance", "accessibility", "static asset", "route dom", "viewport", "visual diff", "artifact uri", "redaction", "deterministic snapshot hash", "no browser"} {
		if !strings.Contains(gates, want) {
			t.Fatalf("test gates missing %q: %s", want, gates)
		}
	}

	var contract struct {
		ProviderFree        bool `json:"provider_free"`
		LiveNetwork         bool `json:"live_network"`
		RequiresBrowser     bool `json:"requires_browser"`
		RequiresCredentials bool `json:"requires_credentials"`
		TypedFixtures       []struct {
			ID             string   `json:"id"`
			Schema         string   `json:"schema"`
			Fixture        string   `json:"fixture"`
			RequiredFields []string `json:"required_fields"`
		} `json:"typed_fixtures"`
		RequiredSectionContract struct {
			Sections []string `json:"sections"`
		} `json:"required_section_contract"`
		PlaceholderContract struct {
			TablePlaceholdersRequired bool     `json:"table_placeholders_required"`
			ChartPlaceholdersRequired bool     `json:"chart_placeholders_required"`
			PlaceholderAccessibility  []string `json:"placeholder_accessibility"`
		} `json:"placeholder_contract"`
		ProvenanceContract struct {
			SourceRefsMustResolve      bool     `json:"source_refs_must_resolve"`
			SourceMarkupAttributes     []string `json:"source_markup_attributes"`
			DisclosureMarkupAttributes []string `json:"disclosure_markup_attributes"`
			UnresolvedRefsAllowed      bool     `json:"unresolved_refs_allowed"`
		} `json:"provenance_contract"`
		StaticAssetContract struct {
			ExternalFetch     bool   `json:"external_fetch"`
			RemoteFonts       bool   `json:"remote_fonts"`
			InlineOrLocalOnly bool   `json:"inline_or_local_only"`
			HashAlgorithm     string `json:"hash_algorithm"`
		} `json:"static_asset_contract"`
		UISnapshotEvaluationContract struct {
			DialectScope                 string   `json:"dialect_scope"`
			RouteDOMSchemaRequired       bool     `json:"route_dom_schema_required"`
			ViewportMatrixRequired       bool     `json:"viewport_matrix_required"`
			VisualDiffBudgetRequired     bool     `json:"visual_diff_budget_required"`
			AccessibilitySummaryRequired bool     `json:"accessibility_summary_required"`
			ArtifactURIManifestRequired  bool     `json:"artifact_uri_manifest_required"`
			RedactionPolicyRequired      bool     `json:"redaction_policy_required"`
			ExternalArtifactFetch        bool     `json:"external_artifact_fetch"`
			AllowedArtifactURISchemes    []string `json:"allowed_artifact_uri_schemes"`
		} `json:"ui_snapshot_evaluation_contract"`
		AcceptanceGates []string `json:"acceptance_gates"`
	}
	decodeHTMLUIJSONFile(t, filepath.Join(base, "contracts", "html_ui_snapshot_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RequiresBrowser || contract.RequiresCredentials || len(contract.TypedFixtures) != 4 {
		t.Fatalf("contract header/fixtures = %#v", contract)
	}
	if !contains(contract.RequiredSectionContract.Sections, "sources") || !contains(contract.RequiredSectionContract.Sections, "disclosures") || len(contract.RequiredSectionContract.Sections) < 8 {
		t.Fatalf("required sections incomplete: %#v", contract.RequiredSectionContract.Sections)
	}
	if !contract.PlaceholderContract.TablePlaceholdersRequired || !contract.PlaceholderContract.ChartPlaceholdersRequired || !contains(contract.PlaceholderContract.PlaceholderAccessibility, "text_equivalent") {
		t.Fatalf("placeholder contract incomplete: %#v", contract.PlaceholderContract)
	}
	if !contract.ProvenanceContract.SourceRefsMustResolve || contract.ProvenanceContract.UnresolvedRefsAllowed ||
		!contains(contract.ProvenanceContract.SourceMarkupAttributes, "data-source-id") ||
		!contains(contract.ProvenanceContract.SourceMarkupAttributes, "data-evidence-hash") ||
		!contains(contract.ProvenanceContract.DisclosureMarkupAttributes, "data-disclosure-id") {
		t.Fatalf("provenance contract incomplete: %#v", contract.ProvenanceContract)
	}
	if contract.StaticAssetContract.ExternalFetch || contract.StaticAssetContract.RemoteFonts || !contract.StaticAssetContract.InlineOrLocalOnly || contract.StaticAssetContract.HashAlgorithm != "sha256" {
		t.Fatalf("static asset contract must forbid remote dependencies: %#v", contract.StaticAssetContract)
	}
	evalContract := contract.UISnapshotEvaluationContract
	if evalContract.DialectScope != "generic_ui_snapshot_evaluation" ||
		!evalContract.RouteDOMSchemaRequired ||
		!evalContract.ViewportMatrixRequired ||
		!evalContract.VisualDiffBudgetRequired ||
		!evalContract.AccessibilitySummaryRequired ||
		!evalContract.ArtifactURIManifestRequired ||
		!evalContract.RedactionPolicyRequired ||
		evalContract.ExternalArtifactFetch ||
		!contains(evalContract.AllowedArtifactURISchemes, "fixture") ||
		!contains(evalContract.AllowedArtifactURISchemes, "artifact") {
		t.Fatalf("ui snapshot evaluation contract incomplete: %#v", evalContract)
	}
}

func TestFinRobotHTMLUISnapshotsFixtures(t *testing.T) {
	base := htmlUISnapshotsLivePackageDir(t)

	var inventory htmlUITemplateInventoryFixture
	decodeHTMLUIJSONFile(t, filepath.Join(base, "fixtures", "template_inventory_fixture.json"), &inventory)
	if !inventory.ProviderFree || inventory.LiveNetwork || inventory.RequiresBrowser || len(inventory.Templates) < 8 {
		t.Fatalf("template inventory header/templates = %#v", inventory)
	}
	if inventory.ProfessionalTemplate.SourcePath != "finrobot_equity/core/src/modules/html_template_professional.py" {
		t.Fatalf("professional template path = %q", inventory.ProfessionalTemplate.SourcePath)
	}
	for _, want := range []string{"executive_summary", "financial_analysis", "valuation", "sources", "disclosures"} {
		if !contains(inventory.ProfessionalTemplate.RequiredSections, want) {
			t.Fatalf("professional template required sections missing %q: %#v", want, inventory.ProfessionalTemplate.RequiredSections)
		}
	}
	if len(inventory.ProfessionalTemplate.TablePlaceholders) < 2 || len(inventory.ProfessionalTemplate.ChartPlaceholders) < 2 {
		t.Fatalf("expected table/chart placeholders: %#v", inventory.ProfessionalTemplate)
	}
	if !inventory.ProfessionalTemplate.ProvenanceMarkup.Required || !contains(inventory.ProfessionalTemplate.ProvenanceMarkup.Attributes, "data-source-id") || !contains(inventory.ProfessionalTemplate.ProvenanceMarkup.Attributes, "data-evidence-hash") {
		t.Fatalf("source provenance markup incomplete: %#v", inventory.ProfessionalTemplate.ProvenanceMarkup)
	}
	if !inventory.ProfessionalTemplate.DisclosureMarkup.Required || !contains(inventory.ProfessionalTemplate.DisclosureMarkup.Attributes, "data-disclosure-id") {
		t.Fatalf("disclosure markup incomplete: %#v", inventory.ProfessionalTemplate.DisclosureMarkup)
	}

	var assets htmlUIStaticAssetManifest
	decodeHTMLUIJSONFile(t, filepath.Join(base, "fixtures", "static_asset_manifest_fixture.json"), &assets)
	if !assets.ProviderFree || assets.LiveNetwork || assets.RequiresBrowser || assets.AssetPolicy.ExternalFetch || assets.AssetPolicy.RemoteFonts || assets.AssetPolicy.NetworkRequired {
		t.Fatalf("static asset manifest must be provider-free/local-only: %#v", assets)
	}
	if len(assets.Assets) < 4 || len(assets.TemplateAssetRefs) < 5 {
		t.Fatalf("static asset manifest incomplete: %#v", assets)
	}
	for _, asset := range assets.Assets {
		if asset.Remote || !strings.HasPrefix(asset.SourcePath, "finrobot_equity/web_app/static/") || asset.FixtureSHA256 == "" {
			t.Fatalf("asset must be local static fixture: %#v", asset)
		}
	}

	var fixture htmlUISnapshotFixture
	decodeHTMLUIJSONFile(t, filepath.Join(base, "fixtures", "html_ui_snapshot_ACME_fixture.json"), &fixture)
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.RequiresBrowser || fixture.RequiresCredentials || !fixture.CleanSkipWithoutBrowser {
		t.Fatalf("snapshot fixture must be provider-free/browser-free: %#v", fixture)
	}
	gotHash := sha256.Sum256([]byte(fixture.DeterministicHashSeed))
	if hex.EncodeToString(gotHash[:]) != fixture.DeterministicSnapshotHash {
		t.Fatalf("deterministic hash mismatch: seed=%q got=%s fixture=%s", fixture.DeterministicHashSeed, hex.EncodeToString(gotHash[:]), fixture.DeterministicSnapshotHash)
	}
	if len(fixture.Snapshots) != 2 {
		t.Fatalf("snapshots = %#v", fixture.Snapshots)
	}
	sourceIDs := map[string]bool{}
	for _, source := range fixture.SourceProvenance {
		sourceIDs[source.ID] = true
		if source.Locator == "" || source.AsOf == "" || source.EvidenceHash == "" {
			t.Fatalf("source provenance incomplete: %#v", source)
		}
	}
	disclosureIDs := map[string]bool{}
	for _, disclosure := range fixture.Disclosures {
		disclosureIDs[disclosure.ID] = true
		if !disclosure.MustRender || !strings.Contains(disclosure.Markup, "data-disclosure-id") {
			t.Fatalf("disclosure incomplete: %#v", disclosure)
		}
	}
	for _, snapshot := range fixture.Snapshots {
		if snapshot.ID == "" || snapshot.MediaType != "text/html" || snapshot.Status != "declared_not_rendered" || snapshot.HashAlgorithm != "sha256" || snapshot.ContentHash == "" {
			t.Fatalf("snapshot metadata incomplete: %#v", snapshot)
		}
		if len(snapshot.RequiredSectionRefs) == 0 || len(snapshot.SourceRefs) == 0 || len(snapshot.DisclosureRefs) == 0 {
			t.Fatalf("snapshot refs incomplete: %#v", snapshot)
		}
		for _, ref := range snapshot.SourceRefs {
			if !sourceIDs[ref] {
				t.Fatalf("snapshot %s unresolved source ref %q", snapshot.ID, ref)
			}
		}
		for _, ref := range snapshot.DisclosureRefs {
			if !disclosureIDs[ref] {
				t.Fatalf("snapshot %s unresolved disclosure ref %q", snapshot.ID, ref)
			}
		}
	}
	if fixture.AccessibilityChecklist.Mode != "static_dom_contract" || fixture.AccessibilityChecklist.RequiresBrowser || len(fixture.AccessibilityChecklist.Checks) < 8 {
		t.Fatalf("accessibility checklist incomplete: %#v", fixture.AccessibilityChecklist)
	}

	var evaluation htmlUISnapshotEvaluationFixture
	decodeHTMLUIJSONFile(t, filepath.Join(base, "fixtures", "ui_snapshot_evaluation_fixture.json"), &evaluation)
	if !evaluation.ProviderFree || evaluation.LiveNetwork || evaluation.RealDependencyImports || evaluation.RequiresBrowser || evaluation.RequiresCredentials || evaluation.DialectScope != "generic_ui_snapshot_evaluation" {
		t.Fatalf("ui snapshot evaluation fixture must be generic/provider-free: %#v", evaluation)
	}
	if len(evaluation.RouteDOMSnapshots) < 2 || len(evaluation.ViewportMatrix) < 3 || len(evaluation.VisualDiffBudgets) < 3 || len(evaluation.AccessibilitySummaries) < 2 {
		t.Fatalf("ui snapshot evaluation coverage incomplete: %#v", evaluation)
	}
	routeIDs := map[string]bool{}
	for _, route := range evaluation.RouteDOMSnapshots {
		routeIDs[route.RouteID] = true
		if route.Route == "" || route.Method == "" || !strings.HasPrefix(route.FixtureURL, "fixture://") || route.DOMSchema.RootSelector == "" {
			t.Fatalf("route DOM snapshot incomplete: %#v", route)
		}
		if len(route.DOMSchema.RequiredLandmarks) == 0 || len(route.DOMSchema.RequiredRegions) == 0 || !contains(route.DOMSchema.StableAttributes, "data-route") {
			t.Fatalf("route DOM schema must declare landmarks, regions, and stable attrs: %#v", route.DOMSchema)
		}
		if len(route.SnapshotRefs) == 0 || len(route.ArtifactRefs) == 0 {
			t.Fatalf("route DOM snapshot refs incomplete: %#v", route)
		}
	}
	viewportIDs := map[string]bool{}
	for _, viewport := range evaluation.ViewportMatrix {
		viewportIDs[viewport.ID] = true
		if viewport.Width <= 0 || viewport.Height <= 0 || viewport.DeviceScaleFactor <= 0 {
			t.Fatalf("viewport dimensions invalid: %#v", viewport)
		}
	}
	for _, budget := range evaluation.VisualDiffBudgets {
		if !routeIDs[budget.RouteID] || !viewportIDs[budget.ViewportID] {
			t.Fatalf("visual diff budget has unresolved route/viewport: %#v", budget)
		}
		if budget.MaxChangedPixelsRatio <= 0 || budget.MaxChangedPixelsRatio > 0.01 || budget.MaxLayoutShiftPX > 2 || budget.TextOverflowAllowed || budget.MissingRegionAllowed {
			t.Fatalf("visual diff budget too loose: %#v", budget)
		}
	}
	for _, summary := range evaluation.AccessibilitySummaries {
		if !routeIDs[summary.RouteID] || summary.Mode != "static_dom_summary" || summary.ViolationsAllowed != 0 || summary.ManualReviewRequired || len(summary.RequiredChecks) == 0 {
			t.Fatalf("accessibility summary incomplete: %#v", summary)
		}
	}
	if evaluation.ArtifactURIManifest.URIPolicy != "fixture_uri_only" || evaluation.ArtifactURIManifest.ExternalFetch || !contains(evaluation.ArtifactURIManifest.AllowedSchemes, "fixture") || !contains(evaluation.ArtifactURIManifest.AllowedSchemes, "artifact") {
		t.Fatalf("artifact URI manifest must be fixture/artifact only: %#v", evaluation.ArtifactURIManifest)
	}
	artifactIDs := map[string]bool{}
	for _, artifact := range evaluation.ArtifactURIManifest.Artifacts {
		artifactIDs[artifact.ID] = true
		if !(strings.HasPrefix(artifact.URI, "artifact://") || strings.HasPrefix(artifact.URI, "fixture://")) || artifact.MediaType == "" || artifact.HashAlgorithm != "sha256" || artifact.Hash == "" {
			t.Fatalf("artifact URI manifest entry incomplete: %#v", artifact)
		}
	}
	for _, route := range evaluation.RouteDOMSnapshots {
		for _, ref := range route.ArtifactRefs {
			if !artifactIDs[ref] {
				t.Fatalf("route %s unresolved artifact ref %q", route.RouteID, ref)
			}
		}
	}
	if evaluation.RedactionPolicy.Mode != "deterministic_static_redaction" || !evaluation.RedactionPolicy.AppliesBeforeHashing || evaluation.RedactionPolicy.SecretEnvPatternsAllowed || len(evaluation.RedactionPolicy.Rules) < 3 || len(evaluation.RedactionPolicy.ForbiddenOutputPatterns) == 0 {
		t.Fatalf("redaction policy incomplete: %#v", evaluation.RedactionPolicy)
	}
}

func TestFinRobotHTMLUISnapshotsFixtureIndexAndNoRuntimeImports(t *testing.T) {
	base := htmlUISnapshotsLivePackageDir(t)
	var index struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		RequiresBrowser       bool `json:"requires_browser"`
		RequiresCredentials   bool `json:"requires_credentials"`
		Fixtures              []struct {
			FixtureKey string         `json:"fixture_key"`
			Capability string         `json:"capability"`
			Path       string         `json:"path"`
			Schemas    []string       `json:"schemas"`
			Metadata   map[string]any `json:"metadata"`
		} `json:"fixtures"`
	}
	decodeHTMLUIJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || index.RequiresBrowser || index.RequiresCredentials || len(index.Fixtures) != 4 {
		t.Fatalf("fixture index header/count = %#v", index)
	}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Capability == "" || fixture.Path == "" || len(fixture.Schemas) == 0 {
			t.Fatalf("fixture index entry incomplete: %#v", fixture)
		}
		if fixture.Metadata["replay_ready"] != true || fixture.Metadata["browser_required"] != false || fixture.Metadata["network_required"] != false {
			t.Fatalf("fixture index metadata must be replay/browser/network free: %#v", fixture.Metadata)
		}
		assertHTMLUIJSONFile(t, filepath.Join(base, fixture.Path))
		for _, schema := range fixture.Schemas {
			assertHTMLUIJSONFile(t, filepath.Join(base, schema))
		}
	}
	seed, _ := index.Fixtures[2].Metadata["deterministic_hash_seed"].(string)
	wantHash := sha256.Sum256([]byte(seed))
	if hex.EncodeToString(wantHash[:]) != index.Fixtures[2].Metadata["deterministic_snapshot_hash"] {
		t.Fatalf("fixture index deterministic hash mismatch: %#v", index.Fixtures[2].Metadata)
	}

	for _, rel := range []string{"main.leia", "package.manifest.json", filepath.Join("contracts", "html_ui_snapshot_contract.json")} {
		data, err := os.ReadFile(filepath.Join(base, rel))
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		for _, pattern := range []string{
			`q/runtime`,
			`(?m)^\s*import\s+`,
			`(?m)^\s*use\s+`,
			`(?m)^\s*load\s*\(`,
			`(?m)^\s*require\s*\(`,
			`(?m)^\s*(playwright|puppeteer|chromium|selenium|requests|http)\s*[.(]`,
		} {
			if regexp.MustCompile(pattern).FindString(source) != "" {
				t.Fatalf("%s contains forbidden runtime/browser/network dependency matching %q", rel, pattern)
			}
		}
	}
}

func TestFinRobotHTMLUISnapshotsExecutableSkeleton(t *testing.T) {
	path := filepath.Join(htmlUISnapshotsLivePackageDir(t), "main.leia")

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
			got, err := vm.Get("html_ui_snapshots_live_package_summary")
			if err != nil {
				t.Fatalf("Get html_ui_snapshots_live_package_summary: %v", err)
			}
			want := "html_ui_snapshots_live_package templates=3 sections=8 tables=2 charts=2 a11y=8 routes=2 viewports=3 diff_budgets=3 artifacts=3 redactions=3 assets=4 provider_free=true browser=false"
			if got != want {
				t.Fatalf("html_ui_snapshots_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func TestFinRobotHTMLUISnapshotsDeterministicOrdering(t *testing.T) {
	manifest := loadHTMLUISnapshotsManifest(t, htmlUISnapshotsLivePackageDir(t))
	var got []string
	for key := range manifest.Schemas {
		got = append(got, key)
	}
	sort.Strings(got)
	want := []string{"accessibility_checklist", "snapshot_metadata", "static_asset_manifest", "template_inventory", "ui_snapshot_evaluation"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("schema keys = %#v, want %#v", got, want)
	}
}

func htmlUISnapshotsLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "html_ui_snapshots")
}

func loadHTMLUISnapshotsManifest(t *testing.T, base string) htmlUISnapshotsManifest {
	t.Helper()
	var manifest htmlUISnapshotsManifest
	decodeHTMLUIJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertHTMLUIJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeHTMLUIJSONFile(t, path, &value)
}

func decodeHTMLUIJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

type htmlUITemplateInventoryFixture struct {
	ProviderFree    bool `json:"provider_free"`
	LiveNetwork     bool `json:"live_network"`
	RequiresBrowser bool `json:"requires_browser"`
	Templates       []struct {
		ID            string   `json:"id"`
		SourcePath    string   `json:"source_path"`
		RequiredSlots []string `json:"required_slots"`
	} `json:"templates"`
	ProfessionalTemplate struct {
		SourcePath        string   `json:"source_path"`
		RequiredSections  []string `json:"required_sections"`
		TablePlaceholders []struct {
			ID            string   `json:"id"`
			Accessibility []string `json:"accessibility"`
		} `json:"table_placeholders"`
		ChartPlaceholders []struct {
			ID            string   `json:"id"`
			Accessibility []string `json:"accessibility"`
		} `json:"chart_placeholders"`
		ProvenanceMarkup struct {
			Required   bool     `json:"required"`
			Attributes []string `json:"attributes"`
		} `json:"provenance_markup"`
		DisclosureMarkup struct {
			Required   bool     `json:"required"`
			Attributes []string `json:"attributes"`
		} `json:"disclosure_markup"`
	} `json:"professional_template"`
}

type htmlUIStaticAssetManifest struct {
	ProviderFree    bool `json:"provider_free"`
	LiveNetwork     bool `json:"live_network"`
	RequiresBrowser bool `json:"requires_browser"`
	AssetPolicy     struct {
		ExternalFetch   bool `json:"external_fetch"`
		RemoteFonts     bool `json:"remote_fonts"`
		NetworkRequired bool `json:"network_required"`
	} `json:"asset_policy"`
	Assets []struct {
		ID            string `json:"id"`
		SourcePath    string `json:"source_path"`
		FixtureSHA256 string `json:"fixture_sha256"`
		Remote        bool   `json:"remote"`
	} `json:"assets"`
	TemplateAssetRefs []struct {
		TemplateID string   `json:"template_id"`
		AssetIDs   []string `json:"asset_ids"`
	} `json:"template_asset_refs"`
}

type htmlUISnapshotFixture struct {
	ProviderFree              bool   `json:"provider_free"`
	LiveNetwork               bool   `json:"live_network"`
	RealDependencyImports     bool   `json:"real_dependency_imports"`
	RequiresBrowser           bool   `json:"requires_browser"`
	RequiresCredentials       bool   `json:"requires_credentials"`
	CleanSkipWithoutBrowser   bool   `json:"clean_skip_without_browser"`
	DeterministicHashSeed     string `json:"deterministic_hash_seed"`
	DeterministicSnapshotHash string `json:"deterministic_snapshot_hash"`
	Snapshots                 []struct {
		ID                  string   `json:"id"`
		MediaType           string   `json:"media_type"`
		Status              string   `json:"status"`
		HashAlgorithm       string   `json:"hash_algorithm"`
		ContentHash         string   `json:"content_hash"`
		RequiredSectionRefs []string `json:"required_section_refs"`
		SourceRefs          []string `json:"source_refs"`
		DisclosureRefs      []string `json:"disclosure_refs"`
	} `json:"snapshots"`
	SourceProvenance []struct {
		ID           string `json:"id"`
		Locator      string `json:"locator"`
		AsOf         string `json:"as_of"`
		EvidenceHash string `json:"evidence_hash"`
	} `json:"source_provenance"`
	Disclosures []struct {
		ID         string `json:"id"`
		MustRender bool   `json:"must_render"`
		Markup     string `json:"markup"`
	} `json:"disclosures"`
	AccessibilityChecklist struct {
		Mode            string `json:"mode"`
		RequiresBrowser bool   `json:"requires_browser"`
		Checks          []struct {
			ID       string `json:"id"`
			Required bool   `json:"required"`
			Status   string `json:"status"`
		} `json:"checks"`
	} `json:"accessibility_checklist"`
}

type htmlUISnapshotEvaluationFixture struct {
	ProviderFree          bool   `json:"provider_free"`
	LiveNetwork           bool   `json:"live_network"`
	RealDependencyImports bool   `json:"real_dependency_imports"`
	RequiresBrowser       bool   `json:"requires_browser"`
	RequiresCredentials   bool   `json:"requires_credentials"`
	DialectScope          string `json:"dialect_scope"`
	RouteDOMSnapshots     []struct {
		RouteID    string `json:"route_id"`
		Route      string `json:"route"`
		Method     string `json:"method"`
		FixtureURL string `json:"fixture_url"`
		DOMSchema  struct {
			RootSelector       string   `json:"root_selector"`
			RequiredLandmarks  []string `json:"required_landmarks"`
			RequiredRegions    []string `json:"required_regions"`
			ForbiddenSelectors []string `json:"forbidden_selectors"`
			StableAttributes   []string `json:"stable_attributes"`
		} `json:"dom_schema"`
		SnapshotRefs []string `json:"snapshot_refs"`
		ArtifactRefs []string `json:"artifact_refs"`
	} `json:"route_dom_snapshots"`
	ViewportMatrix []struct {
		ID                string `json:"id"`
		Width             int    `json:"width"`
		Height            int    `json:"height"`
		DeviceScaleFactor int    `json:"device_scale_factor"`
		ColorScheme       string `json:"color_scheme"`
		ReducedMotion     bool   `json:"reduced_motion"`
	} `json:"viewport_matrix"`
	VisualDiffBudgets []struct {
		RouteID               string  `json:"route_id"`
		ViewportID            string  `json:"viewport_id"`
		MaxChangedPixelsRatio float64 `json:"max_changed_pixels_ratio"`
		MaxLayoutShiftPX      int     `json:"max_layout_shift_px"`
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
			ID            string `json:"id"`
			URI           string `json:"uri"`
			MediaType     string `json:"media_type"`
			HashAlgorithm string `json:"hash_algorithm"`
			Hash          string `json:"hash"`
		} `json:"artifacts"`
	} `json:"artifact_uri_manifest"`
	RedactionPolicy struct {
		Mode                     string `json:"mode"`
		AppliesBeforeHashing     bool   `json:"applies_before_hashing"`
		SecretEnvPatternsAllowed bool   `json:"secret_env_patterns_allowed"`
		Rules                    []struct {
			ID           string `json:"id"`
			PatternClass string `json:"pattern_class"`
			Replacement  string `json:"replacement"`
		} `json:"rules"`
		ForbiddenOutputPatterns []string `json:"forbidden_output_patterns"`
	} `json:"redaction_policy"`
}
