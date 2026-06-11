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

type reportRendererLiveManifest struct {
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
		RendererDependencyRequired  bool   `json:"renderer_dependency_required"`
		CleanSkipWithoutRenderer    bool   `json:"clean_skip_without_renderer"`
		FixtureHook                 string `json:"fixture_hook"`
	} `json:"default_policy"`
	Entrypoints     map[string]string              `json:"entrypoints"`
	Schemas         map[string]string              `json:"schemas"`
	Fixtures        map[string]string              `json:"fixtures"`
	Capabilities    []string                       `json:"capabilities"`
	RenderContracts []reportRendererRenderContract `json:"render_contracts"`
	VisualContract  reportRendererVisualContract   `json:"visual_contract"`
	NoBuiltIn       map[string]json.RawMessage     `json:"no_built_in_guarantee"`
	TestGates       []string                       `json:"test_gates"`
}

type reportRendererRenderContract struct {
	ID                         string   `json:"id"`
	Format                     string   `json:"format"`
	Capability                 string   `json:"capability"`
	RequiredRequestFields      []string `json:"required_request_fields"`
	RequiredOutputFields       []string `json:"required_output_fields"`
	RendererDependencyRequired bool     `json:"renderer_dependency_required"`
	CleanSkip                  bool     `json:"clean_skip"`
}

type reportRendererVisualContract struct {
	OutputFormats          []string `json:"output_formats"`
	SnapshotFormats        []string `json:"snapshot_formats"`
	SnapshotStatuses       []string `json:"snapshot_statuses"`
	RequiredSnapshotFields []string `json:"required_snapshot_fields"`
	WarningKinds           []string `json:"warning_kinds"`
	AnnotationChecks       struct {
		AllSourceRefsMustResolve bool `json:"all_source_refs_must_resolve"`
		AllDisclosuresMustRender bool `json:"all_disclosures_must_render"`
		UnresolvedRefsAllowed    bool `json:"unresolved_refs_allowed"`
	} `json:"annotation_checks"`
}

func TestFinRobotReportRendererLivePackageManifestAndContracts(t *testing.T) {
	base := reportRendererLivePackageDir(t)
	manifest := loadReportRendererLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-report-renderer-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-report-renderer" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		manifest.DefaultPolicy.RendererDependencyRequired ||
		!manifest.DefaultPolicy.CleanSkipWithoutRenderer ||
		manifest.DefaultPolicy.FixtureHook != "recorded_report_renderer_live_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}

	for _, key := range []string{"smoke", "report_renderer_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"render_request", "output_manifest", "page_snapshot_metadata", "render_warning", "source_annotation"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "render_request"} {
		path := manifest.Fixtures[key]
		if path == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertJSONFile(t, filepath.Join(base, path))
	}

	for _, want := range []string{
		"finance.report.render.request",
		"finance.report.render.html.contract",
		"finance.report.render.pdf.contract",
		"finance.report.render.output_manifest",
		"finance.report.render.page_snapshot_metadata",
		"finance.report.render.table_warning",
		"finance.report.render.markdown_warning",
		"finance.report.render.disclosure_annotation",
		"finance.report.render.source_annotation",
		"finance.report.render.missing_chart_handling",
		"finance.report.render.fixture_hash",
		"finance.report.render.clean_skip",
	} {
		if !contains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}

	contractsByFormat := map[string]reportRendererRenderContract{}
	for _, contract := range manifest.RenderContracts {
		contractsByFormat[contract.Format] = contract
		if contract.RendererDependencyRequired || !contract.CleanSkip {
			t.Fatalf("renderer contracts must be clean-skip safe: %#v", contract)
		}
		for _, field := range []string{"report_id", "sections", "tables", "charts", "source_annotations", "disclosures", "output_formats"} {
			if !contains(contract.RequiredRequestFields, field) {
				t.Fatalf("%s request contract missing %q", contract.Format, field)
			}
		}
		for _, field := range []string{"artifact_id", "format", "uri", "content_hash", "page_snapshots", "warnings", "source_annotation_checks", "disclosure_checks"} {
			if !contains(contract.RequiredOutputFields, field) {
				t.Fatalf("%s output contract missing %q", contract.Format, field)
			}
		}
	}
	if contractsByFormat["text/html"].Capability != "finance.report.render.html.contract" ||
		contractsByFormat["application/pdf"].Capability != "finance.report.render.pdf.contract" {
		t.Fatalf("HTML/PDF render contracts missing or mismatched: %#v", contractsByFormat)
	}

	for _, want := range []string{"text/html", "application/pdf"} {
		if !contains(manifest.VisualContract.OutputFormats, want) {
			t.Fatalf("visual output formats missing %q: %#v", want, manifest.VisualContract.OutputFormats)
		}
	}
	for _, want := range []string{"text/html", "application/pdf", "image/png"} {
		if !contains(manifest.VisualContract.SnapshotFormats, want) {
			t.Fatalf("snapshot formats missing %q: %#v", want, manifest.VisualContract.SnapshotFormats)
		}
	}
	for _, want := range []string{"table_render_warning", "markdown_render_warning", "missing_chart_warning"} {
		if !contains(manifest.VisualContract.WarningKinds, want) {
			t.Fatalf("warning kinds missing %q: %#v", want, manifest.VisualContract.WarningKinds)
		}
	}
	if !manifest.VisualContract.AnnotationChecks.AllSourceRefsMustResolve ||
		!manifest.VisualContract.AnnotationChecks.AllDisclosuresMustRender ||
		manifest.VisualContract.AnnotationChecks.UnresolvedRefsAllowed {
		t.Fatalf("annotation checks must be strict: %#v", manifest.VisualContract.AnnotationChecks)
	}
	if len(manifest.NoBuiltIn) == 0 {
		t.Fatal("missing no_built_in_guarantee")
	}
}

func TestFinRobotReportRendererLivePackageFixtureContract(t *testing.T) {
	base := reportRendererLivePackageDir(t)
	var fixture reportRendererFixture
	decodeReportRendererJSONFile(t, filepath.Join(base, "fixtures", "render_request_ACME_report_fixture.json"), &fixture)

	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.RendererDependencyRequired || !fixture.CleanSkipWithoutRenderer {
		t.Fatalf("fixture must stay provider-free and clean-skip safe: %#v", fixture)
	}
	hash := sha256.Sum256([]byte(fixture.DeterministicHashSeed))
	gotHash := hex.EncodeToString(hash[:])
	if gotHash != fixture.DeterministicFixtureHash || gotHash != fixture.OutputManifest.DeterministicFixtureHash {
		t.Fatalf("deterministic fixture hash = seed:%q got:%s fixture:%s manifest:%s", fixture.DeterministicHashSeed, gotHash, fixture.DeterministicFixtureHash, fixture.OutputManifest.DeterministicFixtureHash)
	}

	if fixture.RenderRequest.ReportID == "" || fixture.RenderRequest.Title == "" || len(fixture.RenderRequest.Sections) < 2 || len(fixture.RenderRequest.Tables) == 0 || len(fixture.RenderRequest.Charts) == 0 {
		t.Fatalf("render request shape incomplete: %#v", fixture.RenderRequest)
	}
	if !contains(fixture.RenderRequest.OutputFormats, "text/html") || !contains(fixture.RenderRequest.OutputFormats, "application/pdf") {
		t.Fatalf("render request output formats = %#v", fixture.RenderRequest.OutputFormats)
	}
	annotations := map[string]bool{}
	for _, source := range fixture.RenderRequest.SourceAnnotations {
		if source.ID == "" || source.Title == "" || source.Kind == "" || source.Locator == "" || source.AsOf == "" || source.RetrievedAt == "" || source.EvidenceHash == "" {
			t.Fatalf("source annotation incomplete: %#v", source)
		}
		annotations[source.ID] = true
	}
	disclosures := map[string]bool{}
	for _, disclosure := range fixture.RenderRequest.Disclosures {
		if disclosure.ID == "" || disclosure.Kind == "" || disclosure.Text == "" || !disclosure.MustRender {
			t.Fatalf("disclosure incomplete: %#v", disclosure)
		}
		disclosures[disclosure.ID] = true
	}

	formats := map[string]bool{}
	for _, output := range fixture.OutputManifest.Outputs {
		formats[output.Format] = true
		if output.ArtifactID == "" || output.URI == "" || output.ContentHash == "" || len(output.PageSnapshotIDs) == 0 {
			t.Fatalf("output manifest item incomplete: %#v", output)
		}
	}
	if !formats["text/html"] || !formats["application/pdf"] {
		t.Fatalf("output manifest missing HTML/PDF outputs: %#v", fixture.OutputManifest.Outputs)
	}
	if fixture.OutputManifest.Renderer.DependencyAvailable || !fixture.OutputManifest.Renderer.CleanSkip || fixture.OutputManifest.Renderer.Status != "clean_skipped_without_renderer" {
		t.Fatalf("renderer clean skip not represented: %#v", fixture.OutputManifest.Renderer)
	}

	for _, snapshot := range fixture.OutputManifest.PageSnapshots {
		if snapshot.ID == "" || snapshot.PageNumber <= 0 || snapshot.Format == "" || snapshot.Status == "" || snapshot.ContentHash == "" {
			t.Fatalf("snapshot metadata incomplete: %#v", snapshot)
		}
		if snapshot.Viewport.Width <= 0 || snapshot.Viewport.Height <= 0 || snapshot.Dimensions.Width <= 0 || snapshot.Dimensions.Height <= 0 || snapshot.Dimensions.Unit == "" {
			t.Fatalf("snapshot dimensions incomplete: %#v", snapshot)
		}
		for _, sourceRef := range snapshot.SourceRefs {
			if !annotations[sourceRef] {
				t.Fatalf("snapshot %s unresolved source ref %q", snapshot.ID, sourceRef)
			}
		}
		for _, disclosureRef := range snapshot.DisclosureRefs {
			if !disclosures[disclosureRef] {
				t.Fatalf("snapshot %s unresolved disclosure ref %q", snapshot.ID, disclosureRef)
			}
		}
	}

	warningKinds := map[string]bool{}
	for _, warning := range fixture.OutputManifest.Warnings {
		warningKinds[warning.Kind] = true
		if warning.ID == "" || warning.Severity == "" || warning.Message == "" || warning.TargetID == "" || len(warning.SourceRefs) == 0 {
			t.Fatalf("render warning incomplete: %#v", warning)
		}
		for _, sourceRef := range warning.SourceRefs {
			if !annotations[sourceRef] {
				t.Fatalf("warning %s unresolved source ref %q", warning.ID, sourceRef)
			}
		}
	}
	wantKinds := []string{"table_render_warning", "markdown_render_warning", "missing_chart_warning"}
	for _, want := range wantKinds {
		if !warningKinds[want] {
			t.Fatalf("missing warning kind %q: %#v", want, warningKinds)
		}
	}
	for _, chart := range fixture.RenderRequest.Charts {
		if chart.Status == "missing_input" && !chart.PlaceholderRequired {
			t.Fatalf("missing chart must require placeholder: %#v", chart)
		}
	}

	for _, check := range fixture.OutputManifest.SourceAnnotationChecks {
		if !annotations[check.SourceID] || !check.Resolved || len(check.RenderedInOutputs) < 2 {
			t.Fatalf("source annotation check incomplete: %#v", check)
		}
	}
	for _, check := range fixture.OutputManifest.DisclosureChecks {
		if !disclosures[check.DisclosureID] || !check.Rendered || len(check.RenderedInOutputs) < 2 {
			t.Fatalf("disclosure check incomplete: %#v", check)
		}
	}
}

func TestFinRobotReportRendererLivePackageFixtureIndexAndSchemas(t *testing.T) {
	base := reportRendererLivePackageDir(t)
	var index struct {
		ProviderFree               bool `json:"provider_free"`
		LiveNetwork                bool `json:"live_network"`
		RealDependencyImports      bool `json:"real_dependency_imports"`
		RendererDependencyRequired bool `json:"renderer_dependency_required"`
		CleanSkipWithoutRenderer   bool `json:"clean_skip_without_renderer"`
		Fixtures                   []struct {
			FixtureKey string         `json:"fixture_key"`
			Capability string         `json:"capability"`
			Path       string         `json:"path"`
			Schemas    []string       `json:"schemas"`
			Metadata   map[string]any `json:"metadata"`
		} `json:"fixtures"`
	}
	decodeReportRendererJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || index.RendererDependencyRequired || !index.CleanSkipWithoutRenderer || len(index.Fixtures) != 1 {
		t.Fatalf("fixture index header/count = %#v", index)
	}
	fixture := index.Fixtures[0]
	if fixture.FixtureKey != "report_renderer:render_request:ACME:offline" || fixture.Capability != "finance.report.render.request" || fixture.Path == "" || len(fixture.Schemas) != 5 {
		t.Fatalf("fixture metadata incomplete: %#v", fixture)
	}
	if fixture.Metadata["replay_ready"] != true || fixture.Metadata["renderer_clean_skip"] != true {
		t.Fatalf("fixture metadata should be replay-ready clean-skip: %#v", fixture.Metadata)
	}
	seed, _ := fixture.Metadata["deterministic_hash_seed"].(string)
	wantHash := sha256.Sum256([]byte(seed))
	if hex.EncodeToString(wantHash[:]) != fixture.Metadata["deterministic_fixture_hash"] {
		t.Fatalf("fixture index hash mismatch: %#v", fixture.Metadata)
	}
	assertJSONFile(t, filepath.Join(base, fixture.Path))
	for _, schema := range fixture.Schemas {
		assertJSONFile(t, filepath.Join(base, schema))
	}
}

func TestFinRobotReportRendererLivePackageNoLiveImports(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(reportRendererLivePackageDir(t), "main.leia"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pattern := range []string{
		`(?m)^\s*import\s+`,
		`(?m)^\s*use\s+`,
		`(?m)^\s*load\s*\(`,
		`(?m)^\s*require\s*\(`,
		`(?m)^\s*(playwright|puppeteer|chromium|wkhtmltopdf|weasyprint|pdfkit|plotly|kaleido|selenium|requests|http)\s*[.(]`,
	} {
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("main.leia contains live dependency loader matching %q", pattern)
		}
	}
}

func TestFinRobotReportRendererLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(reportRendererLivePackageDir(t), "main.leia")

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
			got, err := vm.Get("report_renderer_live_package_summary")
			if err != nil {
				t.Fatalf("Get report_renderer_live_package_summary: %v", err)
			}
			want := "report_renderer_live_package formats=2 snapshots=3 warnings=3 sources=2 disclosures=2 provider_free=true clean_skip=true"
			if got != want {
				t.Fatalf("report_renderer_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func reportRendererLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "report_renderer")
}

func loadReportRendererLiveManifest(t *testing.T, base string) reportRendererLiveManifest {
	t.Helper()
	var manifest reportRendererLiveManifest
	decodeReportRendererJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func decodeReportRendererJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

type reportRendererFixture struct {
	ProviderFree               bool   `json:"provider_free"`
	LiveNetwork                bool   `json:"live_network"`
	RealDependencyImports      bool   `json:"real_dependency_imports"`
	RendererDependencyRequired bool   `json:"renderer_dependency_required"`
	CleanSkipWithoutRenderer   bool   `json:"clean_skip_without_renderer"`
	DeterministicHashSeed      string `json:"deterministic_hash_seed"`
	DeterministicFixtureHash   string `json:"deterministic_fixture_hash"`
	RenderRequest              struct {
		ReportID      string   `json:"report_id"`
		Title         string   `json:"title"`
		OutputFormats []string `json:"output_formats"`
		Sections      []struct {
			ID         string   `json:"id"`
			SourceRefs []string `json:"source_refs"`
		} `json:"sections"`
		Tables []struct {
			ID         string   `json:"id"`
			SourceRefs []string `json:"source_refs"`
		} `json:"tables"`
		Charts []struct {
			ID                  string   `json:"id"`
			Status              string   `json:"status"`
			PlaceholderRequired bool     `json:"placeholder_required"`
			SourceRefs          []string `json:"source_refs"`
		} `json:"charts"`
		SourceAnnotations []struct {
			ID           string `json:"id"`
			Title        string `json:"title"`
			Kind         string `json:"kind"`
			Locator      string `json:"locator"`
			AsOf         string `json:"as_of"`
			RetrievedAt  string `json:"retrieved_at"`
			EvidenceHash string `json:"evidence_hash"`
		} `json:"source_annotations"`
		Disclosures []struct {
			ID         string `json:"id"`
			Kind       string `json:"kind"`
			Text       string `json:"text"`
			MustRender bool   `json:"must_render"`
		} `json:"disclosures"`
	} `json:"render_request"`
	OutputManifest struct {
		Renderer struct {
			DependencyAvailable bool   `json:"dependency_available"`
			CleanSkip           bool   `json:"clean_skip"`
			Status              string `json:"status"`
		} `json:"renderer"`
		Outputs []struct {
			ArtifactID      string   `json:"artifact_id"`
			Format          string   `json:"format"`
			URI             string   `json:"uri"`
			ContentHash     string   `json:"content_hash"`
			PageSnapshotIDs []string `json:"page_snapshot_ids"`
		} `json:"outputs"`
		PageSnapshots []struct {
			ID         string `json:"id"`
			PageNumber int    `json:"page_number"`
			Format     string `json:"format"`
			Viewport   struct {
				Width  float64 `json:"width"`
				Height float64 `json:"height"`
			} `json:"viewport"`
			Dimensions struct {
				Width  float64 `json:"width"`
				Height float64 `json:"height"`
				Unit   string  `json:"unit"`
			} `json:"dimensions"`
			Status         string   `json:"status"`
			ContentHash    string   `json:"content_hash"`
			SourceRefs     []string `json:"source_refs"`
			WarningRefs    []string `json:"warning_refs"`
			DisclosureRefs []string `json:"disclosure_refs"`
		} `json:"page_snapshots"`
		Warnings []struct {
			ID         string   `json:"id"`
			Kind       string   `json:"kind"`
			Severity   string   `json:"severity"`
			Message    string   `json:"message"`
			TargetID   string   `json:"target_id"`
			SourceRefs []string `json:"source_refs"`
		} `json:"warnings"`
		SourceAnnotationChecks []struct {
			SourceID          string   `json:"source_id"`
			Resolved          bool     `json:"resolved"`
			RenderedInOutputs []string `json:"rendered_in_outputs"`
		} `json:"source_annotation_checks"`
		DisclosureChecks []struct {
			DisclosureID      string   `json:"disclosure_id"`
			Rendered          bool     `json:"rendered"`
			RenderedInOutputs []string `json:"rendered_in_outputs"`
		} `json:"disclosure_checks"`
		DeterministicFixtureHash string `json:"deterministic_fixture_hash"`
	} `json:"output_manifest"`
}

func TestFinRobotReportRendererLivePackageDeterministicOrdering(t *testing.T) {
	base := reportRendererLivePackageDir(t)
	manifest := loadReportRendererLiveManifest(t, base)
	var got []string
	for key := range manifest.Schemas {
		got = append(got, key)
	}
	sort.Strings(got)
	want := []string{"output_manifest", "page_snapshot_metadata", "render_request", "render_warning", "source_annotation"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("schema keys = %#v, want %#v", got, want)
	}
}
