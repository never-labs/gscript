package leia_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type analyticsReportLivePackageManifest struct {
	SchemaVersion               int      `json:"schema_version"`
	ID                          string   `json:"id"`
	PackageName                 string   `json:"package_name"`
	ProviderFree                bool     `json:"provider_free"`
	LiveNetworkDefault          bool     `json:"live_network_default"`
	RealDependencyImportDefault bool     `json:"real_dependency_import_default"`
	Capabilities                []string `json:"capabilities"`
	Entrypoints                 struct {
		Main    string `json:"main"`
		Example string `json:"example"`
		Schema  string `json:"schema"`
	} `json:"entrypoints"`
	DefaultPolicy struct {
		Mode                        string `json:"mode"`
		LiveNetwork                 bool   `json:"live_network"`
		ProviderCredentialsRequired bool   `json:"provider_credentials_required"`
		RealDependencyImports       bool   `json:"real_dependency_imports"`
		CleanSkipWithoutDependency  bool   `json:"clean_skip_without_dependency"`
		FixtureHook                 string `json:"fixture_hook"`
	} `json:"default_policy"`
	Contracts    map[string]string `json:"contracts"`
	ProviderGate struct {
		AllowNetwork        bool     `json:"allow_network"`
		RequiredCredentials []string `json:"required_credentials"`
		OptionalCredentials []string `json:"optional_credentials"`
		BlockedImports      []string `json:"blocked_imports"`
		TestRule            string   `json:"test_rule"`
	} `json:"provider_gate"`
	ArtifactContracts struct {
		ReportFormats          []string `json:"report_formats"`
		SnapshotFormats        []string `json:"snapshot_formats"`
		SnapshotStatuses       []string `json:"snapshot_statuses"`
		AccessibilityStandards []string `json:"accessibility_standards"`
		StaleDataWarningPolicy struct {
			WarningRequiredWhenSourceStale bool   `json:"warning_required_when_source_stale"`
			WarningMarker                  string `json:"warning_marker"`
			StaleSourceRefsMustBeNamed     bool   `json:"stale_source_refs_must_be_named"`
			SnapshotsMustCarryWarningRefs  bool   `json:"snapshots_must_carry_warning_refs"`
		} `json:"stale_data_warning_policy"`
		SourceAnnotationRequirements       []string `json:"source_annotation_requirements"`
		ReportOutlineRequirements          []string `json:"report_outline_requirements"`
		SectionDependencyDAGRequirements   []string `json:"section_dependency_dag_requirements"`
		EvidenceEnvelopeRequirements       []string `json:"evidence_envelope_requirements"`
		RenderArtifactManifestRequirements []string `json:"render_artifact_manifest_requirements"`
		StyleProfilePolicyRequirements     []string `json:"style_profile_policy_requirements"`
		PartialReportFixtureRequirements   []string `json:"partial_report_fixture_requirements"`
		RequiredMarkers                    []string `json:"required_markers"`
	} `json:"artifact_contracts"`
}

type analyticsReportLivePackageSchema struct {
	SchemaVersion int `json:"schema_version"`
	Schemas       map[string]struct {
		Required                         []string `json:"required"`
		ArtifactRequired                 []string `json:"artifact_required"`
		Formats                          []string `json:"formats"`
		Statuses                         []string `json:"statuses"`
		Methods                          []string `json:"methods"`
		ItemRequired                     []string `json:"item_required"`
		SectionRequired                  []string `json:"section_required"`
		EdgeRequired                     []string `json:"edge_required"`
		CitationRequired                 []string `json:"citation_required"`
		OutputRequired                   []string `json:"output_required"`
		MustBeAcyclic                    bool     `json:"must_be_acyclic"`
		AllReportSourceRefsMustResolve   bool     `json:"all_report_source_refs_must_resolve"`
		AllSnapshotSourceRefsMustResolve bool     `json:"all_snapshot_source_refs_must_resolve"`
		AllCitationSourceRefsMustResolve bool     `json:"all_citation_source_refs_must_resolve"`
	} `json:"schemas"`
	ProviderFreeGate struct {
		NetworkRequired      bool `json:"network_required"`
		CredentialsRequired  bool `json:"credentials_required"`
		ProviderSDKsRequired bool `json:"provider_sdks_required"`
	} `json:"provider_free_gate"`
}

func TestAnalyticsReportLivePackageManifestSchemaAndContracts(t *testing.T) {
	root := repoRoot(t)
	base := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "analytics_report")
	manifest := loadAnalyticsReportLivePackageManifest(t, base)
	schema := loadAnalyticsReportLivePackageSchema(t, base, manifest.Entrypoints.Schema)

	if manifest.SchemaVersion != 1 || manifest.ID != "leia-analytics-report-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-analytics-report" {
		t.Fatalf("package_name = %q", manifest.PackageName)
	}
	if manifest.Entrypoints.Main != "analytics_report.leia" || manifest.Entrypoints.Example != manifest.Entrypoints.Main {
		t.Fatalf("entrypoints = %#v, want main/example analytics_report.leia", manifest.Entrypoints)
	}
	if _, err := os.Stat(filepath.Join(base, manifest.Entrypoints.Main)); err != nil {
		t.Fatalf("main entrypoint %q: %v", manifest.Entrypoints.Main, err)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault || manifest.ProviderGate.AllowNetwork {
		t.Fatalf("provider-free gate is not closed: %#v", manifest)
	}
	if len(manifest.ProviderGate.RequiredCredentials) != 0 || len(manifest.ProviderGate.OptionalCredentials) != 0 {
		t.Fatalf("provider credentials must not be required by this skeleton: %#v", manifest.ProviderGate)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_analytics_report_live_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}
	for _, want := range []string{
		"finance.analytics_report.normalized_inputs",
		"finance.analytics_report.valuation_summary",
		"finance.analytics_report.sensitivity_matrix",
		"finance.analytics_report.chart_specs",
		"finance.analytics_report.report_outline",
		"finance.analytics_report.section_dependency_dag",
		"finance.analytics_report.evidence_citation_envelope",
		"finance.analytics_report.render_manifest",
		"finance.analytics_report.style_profile_policy",
		"finance.analytics_report.partial_failure_fixture",
		"finance.analytics_report.snapshot_metadata",
		"finance.analytics_report.accessibility_checklist",
		"finance.analytics_report.stale_data_policy",
		"finance.analytics_report.source_annotations",
		"finance.analytics_report.output_manifest",
		"finance.analytics_report.renderer_contracts",
		"finance.normalize.price_series",
		"analytics.valuation.dcf",
		"analytics.sensitivity.matrix",
		"artifact.chart.spec",
		"artifact.snapshot.metadata",
		"artifact.accessibility.checklist",
		"artifact.stale_data.policy",
		"artifact.source_annotation.requirements",
		"artifact.evidence_citation.envelope",
		"artifact.report.manifest",
		"artifact.render.manifest",
		"report.outline.schema",
		"report.section_dependency.dag",
		"report.style_profile.policy",
		"report.partial_failure.fixture",
		"renderer.html.contract",
		"renderer.pdf.contract",
	} {
		if !analyticsReportContains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q", want)
		}
	}
	for _, planCapability := range analyticsReportPlanCapabilities(t) {
		if !strings.HasPrefix(planCapability, "finance.analytics_report.") {
			t.Fatalf("analytics_report plan capability %q does not use finance.analytics_report.*", planCapability)
		}
		if !analyticsReportContains(manifest.Capabilities, planCapability) {
			t.Fatalf("analytics_report plan capability %q missing from manifest capabilities", planCapability)
		}
	}
	if !analyticsReportContains(manifest.ArtifactContracts.ReportFormats, "text/html") || !analyticsReportContains(manifest.ArtifactContracts.ReportFormats, "application/pdf") {
		t.Fatalf("report formats = %#v, want HTML and PDF", manifest.ArtifactContracts.ReportFormats)
	}
	for _, want := range []string{"text/html", "application/pdf", "image/png"} {
		if !analyticsReportContains(manifest.ArtifactContracts.SnapshotFormats, want) {
			t.Fatalf("snapshot formats missing %q: %#v", want, manifest.ArtifactContracts.SnapshotFormats)
		}
	}
	for _, want := range []string{"specified_not_rendered", "planned_not_rendered"} {
		if !analyticsReportContains(manifest.ArtifactContracts.SnapshotStatuses, want) {
			t.Fatalf("snapshot statuses missing %q: %#v", want, manifest.ArtifactContracts.SnapshotStatuses)
		}
	}
	if !analyticsReportContains(manifest.ArtifactContracts.AccessibilityStandards, "WCAG 2.2 AA contract checklist") {
		t.Fatalf("accessibility standards = %#v", manifest.ArtifactContracts.AccessibilityStandards)
	}
	if !manifest.ArtifactContracts.StaleDataWarningPolicy.WarningRequiredWhenSourceStale ||
		manifest.ArtifactContracts.StaleDataWarningPolicy.WarningMarker != "stale_data_warning" ||
		!manifest.ArtifactContracts.StaleDataWarningPolicy.StaleSourceRefsMustBeNamed ||
		!manifest.ArtifactContracts.StaleDataWarningPolicy.SnapshotsMustCarryWarningRefs {
		t.Fatalf("stale data warning policy not strict enough: %#v", manifest.ArtifactContracts.StaleDataWarningPolicy)
	}
	for _, want := range []string{"id", "title", "kind", "locator", "as_of", "stale_after", "stale", "license", "retrieved_at", "evidence_hash"} {
		if !analyticsReportContains(manifest.ArtifactContracts.SourceAnnotationRequirements, want) {
			t.Fatalf("source annotation requirements missing %q: %#v", want, manifest.ArtifactContracts.SourceAnnotationRequirements)
		}
	}
	for _, want := range []string{"id", "title", "audience", "time_horizon", "sections", "style_profile_id", "evidence_policy_id", "render_manifest_id"} {
		if !analyticsReportContains(manifest.ArtifactContracts.ReportOutlineRequirements, want) {
			t.Fatalf("report outline requirements missing %q: %#v", want, manifest.ArtifactContracts.ReportOutlineRequirements)
		}
	}
	for _, want := range []string{"id", "nodes", "edges", "acyclic", "missing_dependency_policy"} {
		if !analyticsReportContains(manifest.ArtifactContracts.SectionDependencyDAGRequirements, want) {
			t.Fatalf("section DAG requirements missing %q: %#v", want, manifest.ArtifactContracts.SectionDependencyDAGRequirements)
		}
	}
	for _, want := range []string{"id", "claim_id", "source_refs", "citation_refs", "evidence_quality", "provider_free", "unresolved_refs"} {
		if !analyticsReportContains(manifest.ArtifactContracts.EvidenceEnvelopeRequirements, want) {
			t.Fatalf("evidence envelope requirements missing %q: %#v", want, manifest.ArtifactContracts.EvidenceEnvelopeRequirements)
		}
	}
	for _, want := range []string{"id", "renderer_contract_refs", "outputs", "snapshots", "dependency_required", "status"} {
		if !analyticsReportContains(manifest.ArtifactContracts.RenderArtifactManifestRequirements, want) {
			t.Fatalf("render artifact manifest requirements missing %q: %#v", want, manifest.ArtifactContracts.RenderArtifactManifestRequirements)
		}
	}
	for _, want := range []string{"id", "tone", "locale", "number_format", "currency", "disclosure_policy", "forbidden_content", "provider_free"} {
		if !analyticsReportContains(manifest.ArtifactContracts.StyleProfilePolicyRequirements, want) {
			t.Fatalf("style profile policy requirements missing %q: %#v", want, manifest.ArtifactContracts.StyleProfilePolicyRequirements)
		}
	}
	for _, want := range []string{"id", "status", "completed_sections", "failed_sections", "retryable", "renderable", "warning_refs", "failure_reason"} {
		if !analyticsReportContains(manifest.ArtifactContracts.PartialReportFixtureRequirements, want) {
			t.Fatalf("partial report fixture requirements missing %q: %#v", want, manifest.ArtifactContracts.PartialReportFixtureRequirements)
		}
	}
	for _, marker := range []string{"report_outline", "section_dependency_dag", "evidence_citation_envelopes", "render_artifact_manifest", "style_profile_policy", "partial_report_fixture", "source_annotations", "stale_data_warnings", "ai_disclosure", "renderer_contract", "snapshot_metadata", "accessibility_checklist", "source_annotation_requirements", "provider_free"} {
		if !analyticsReportContains(manifest.ArtifactContracts.RequiredMarkers, marker) {
			t.Fatalf("artifact contract markers missing %q", marker)
		}
	}
	if schema.SchemaVersion != 1 {
		t.Fatalf("schema version = %d, want 1", schema.SchemaVersion)
	}
	if schema.ProviderFreeGate.NetworkRequired || schema.ProviderFreeGate.CredentialsRequired || schema.ProviderFreeGate.ProviderSDKsRequired {
		t.Fatalf("schema provider-free gate must require no network, credentials, or provider SDKs: %#v", schema.ProviderFreeGate)
	}
	for _, key := range []string{"finance_normalizer", "valuation_result", "sensitivity_matrix", "chart_artifact", "snapshot_metadata", "accessibility_checklist", "stale_data_policy", "source_annotation_requirements", "report_outline", "section_dependency_dag", "evidence_citation_envelope", "render_artifact_manifest", "style_profile_policy", "partial_report_fixture", "report_artifact", "renderer_contract"} {
		if len(schema.Schemas[key].Required) == 0 {
			t.Fatalf("schema %q missing required fields", key)
		}
	}
	if !analyticsReportContains(schema.Schemas["renderer_contract"].Formats, "text/html") || !analyticsReportContains(schema.Schemas["renderer_contract"].Formats, "application/pdf") {
		t.Fatalf("renderer formats = %#v, want HTML and PDF", schema.Schemas["renderer_contract"].Formats)
	}
	for _, want := range []string{"text/html", "application/pdf", "image/png"} {
		if !analyticsReportContains(schema.Schemas["snapshot_metadata"].Formats, want) {
			t.Fatalf("snapshot metadata formats missing %q: %#v", want, schema.Schemas["snapshot_metadata"].Formats)
		}
	}
	for _, want := range []string{"renderer_dependency_required", "warning_refs", "accessibility_refs"} {
		if !analyticsReportContains(schema.Schemas["snapshot_metadata"].Required, want) {
			t.Fatalf("snapshot metadata required fields missing %q: %#v", want, schema.Schemas["snapshot_metadata"].Required)
		}
	}
	for _, want := range []string{"report_sections", "chart_specs", "snapshot_metadata", "accessibility_checklist", "stale_data_policy", "source_annotation_requirements"} {
		if !analyticsReportContains(schema.Schemas["report_artifact"].Required, want) {
			t.Fatalf("report artifact required fields missing %q: %#v", want, schema.Schemas["report_artifact"].Required)
		}
	}
	for _, want := range []string{"snapshot_metadata_schema", "accessibility_checklist_schema", "stale_data_policy"} {
		if !analyticsReportContains(schema.Schemas["renderer_contract"].Required, want) {
			t.Fatalf("renderer contract required fields missing %q: %#v", want, schema.Schemas["renderer_contract"].Required)
		}
	}
	if !schema.Schemas["source_annotation_requirements"].AllReportSourceRefsMustResolve ||
		!schema.Schemas["source_annotation_requirements"].AllSnapshotSourceRefsMustResolve {
		t.Fatalf("source annotation refs must resolve: %#v", schema.Schemas["source_annotation_requirements"])
	}
	if !schema.Schemas["section_dependency_dag"].MustBeAcyclic || !analyticsReportContains(schema.Schemas["section_dependency_dag"].EdgeRequired, "reason") {
		t.Fatalf("section DAG schema must be acyclic and require edge reasons: %#v", schema.Schemas["section_dependency_dag"])
	}
	if !schema.Schemas["evidence_citation_envelope"].AllCitationSourceRefsMustResolve || !analyticsReportContains(schema.Schemas["evidence_citation_envelope"].CitationRequired, "quote_policy") {
		t.Fatalf("evidence envelope citations must resolve and carry quote policy: %#v", schema.Schemas["evidence_citation_envelope"])
	}
	for _, want := range []string{"report_outline", "section_dependency_dag", "evidence_citation_envelopes", "render_artifact_manifest", "style_profile_policy", "partial_report_fixture"} {
		if !analyticsReportContains(schema.Schemas["report_artifact"].Required, want) {
			t.Fatalf("report artifact required fields missing %q: %#v", want, schema.Schemas["report_artifact"].Required)
		}
	}
	if !analyticsReportContains(schema.Schemas["render_artifact_manifest"].Statuses, "partial_not_rendered") ||
		!analyticsReportContains(schema.Schemas["render_artifact_manifest"].OutputRequired, "snapshot_ref") {
		t.Fatalf("render artifact manifest schema missing partial status or snapshot refs: %#v", schema.Schemas["render_artifact_manifest"])
	}
	if !analyticsReportContains(schema.Schemas["partial_report_fixture"].Statuses, "partial") {
		t.Fatalf("partial report fixture schema statuses = %#v", schema.Schemas["partial_report_fixture"].Statuses)
	}

	exampleData, err := os.ReadFile(filepath.Join(base, manifest.Entrypoints.Example))
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	example := string(exampleData)
	for _, blocked := range manifest.ProviderGate.BlockedImports {
		if strings.Contains(example, "import "+blocked) || strings.Contains(example, `require("`+blocked+`"`) {
			t.Fatalf("example appears to import blocked provider dependency %q", blocked)
		}
	}
}

func TestAnalyticsReportLivePackageLeiaFixture(t *testing.T) {
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
				leia.WithLibs(leia.LibMath | leia.LibMatrix | leia.LibString | leia.LibLLM),
				leia.WithPrint(func(args ...any) {
					var parts []string
					for _, arg := range args {
						parts = append(parts, fmt.Sprint(arg))
					}
					prints = append(prints, strings.Join(parts, " "))
				}),
			}, tc.opts...)...)
			path := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "analytics_report", "analytics_report.leia")
			if err := vm.ExecFile(path); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}
			assertVMValue(t, vm, "analytics_report_live_package_summary", "analytics_report_live_package provider_free=true normalizers=2 valuations=3 sensitivity=3x3 outline=2 dag_edges=1 evidence=2 charts=2 artifacts=4 render_outputs=4 snapshots=4 a11y=4 warnings=1 partial_failures=1 html=text/html pdf=application/pdf")
			for name, want := range map[string]any{
				"normalizers_ok":         true,
				"section_ok":             true,
				"chart_ok":               true,
				"source_ok":              true,
				"manifest_ok":            true,
				"snapshot_ok":            true,
				"accessibility_ok":       true,
				"stale_policy_ok":        true,
				"source_requirements_ok": true,
				"outline_ok":             true,
				"section_dag_ok":         true,
				"evidence_ok":            true,
				"render_manifest_ok":     true,
				"style_policy_ok":        true,
				"partial_fixture_ok":     true,
				"dcf_price":              float64(50.0826446281),
				"peer_price":             float64(66),
				"target_price":           float64(56.4495867769),
				"upside":                 float64(0.1289917355),
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				switch want := want.(type) {
				case float64:
					gotFloat, ok := got.(float64)
					if !ok || absFloat(gotFloat-want) > 0.000001 {
						t.Fatalf("%s = %#v, want %.10f", name, got, want)
					}
				default:
					if got != want {
						t.Fatalf("%s = %#v, want %#v", name, got, want)
					}
				}
			}
			if len(prints) != 1 || prints[0] != "analytics_report_live_package provider_free=true normalizers=2 valuations=3 sensitivity=3x3 outline=2 dag_edges=1 evidence=2 charts=2 artifacts=4 render_outputs=4 snapshots=4 a11y=4 warnings=1 partial_failures=1 html=text/html pdf=application/pdf" {
				t.Fatalf("prints = %#v", prints)
			}
		})
	}
}

func loadAnalyticsReportLivePackageManifest(t *testing.T, base string) analyticsReportLivePackageManifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(base, "package.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest analyticsReportLivePackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode package.manifest.json: %v", err)
	}
	return manifest
}

func loadAnalyticsReportLivePackageSchema(t *testing.T, base string, name string) analyticsReportLivePackageSchema {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(base, name))
	if err != nil {
		t.Fatal(err)
	}
	var schema analyticsReportLivePackageSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return schema
}

func analyticsReportPlanCapabilities(t *testing.T) []string {
	t.Helper()
	plan := loadLivePackagePlanManifest(t, repoRoot(t))
	for _, pkg := range plan.Packages {
		if pkg.ID == "analytics_report" {
			return pkg.Capabilities
		}
	}
	t.Fatal("analytics_report package missing from live package plan")
	return nil
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func analyticsReportContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
