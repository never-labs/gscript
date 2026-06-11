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
	ProviderFree                bool     `json:"provider_free"`
	LiveNetworkDefault          bool     `json:"live_network_default"`
	RealDependencyImportDefault bool     `json:"real_dependency_import_default"`
	Capabilities                []string `json:"capabilities"`
	Entrypoints                 struct {
		Example string `json:"example"`
		Schema  string `json:"schema"`
	} `json:"entrypoints"`
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
		SourceAnnotationRequirements []string `json:"source_annotation_requirements"`
		RequiredMarkers              []string `json:"required_markers"`
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
		AllReportSourceRefsMustResolve   bool     `json:"all_report_source_refs_must_resolve"`
		AllSnapshotSourceRefsMustResolve bool     `json:"all_snapshot_source_refs_must_resolve"`
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
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault || manifest.ProviderGate.AllowNetwork {
		t.Fatalf("provider-free gate is not closed: %#v", manifest)
	}
	if len(manifest.ProviderGate.RequiredCredentials) != 0 || len(manifest.ProviderGate.OptionalCredentials) != 0 {
		t.Fatalf("provider credentials must not be required by this skeleton: %#v", manifest.ProviderGate)
	}
	for _, want := range []string{
		"finance.normalize.price_series",
		"analytics.valuation.dcf",
		"analytics.sensitivity.matrix",
		"artifact.chart.spec",
		"artifact.snapshot.metadata",
		"artifact.accessibility.checklist",
		"artifact.stale_data.policy",
		"artifact.source_annotation.requirements",
		"artifact.report.manifest",
		"renderer.html.contract",
		"renderer.pdf.contract",
	} {
		if !contains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q", want)
		}
	}
	if !contains(manifest.ArtifactContracts.ReportFormats, "text/html") || !contains(manifest.ArtifactContracts.ReportFormats, "application/pdf") {
		t.Fatalf("report formats = %#v, want HTML and PDF", manifest.ArtifactContracts.ReportFormats)
	}
	for _, want := range []string{"text/html", "application/pdf", "image/png"} {
		if !contains(manifest.ArtifactContracts.SnapshotFormats, want) {
			t.Fatalf("snapshot formats missing %q: %#v", want, manifest.ArtifactContracts.SnapshotFormats)
		}
	}
	for _, want := range []string{"specified_not_rendered", "planned_not_rendered"} {
		if !contains(manifest.ArtifactContracts.SnapshotStatuses, want) {
			t.Fatalf("snapshot statuses missing %q: %#v", want, manifest.ArtifactContracts.SnapshotStatuses)
		}
	}
	if !contains(manifest.ArtifactContracts.AccessibilityStandards, "WCAG 2.2 AA contract checklist") {
		t.Fatalf("accessibility standards = %#v", manifest.ArtifactContracts.AccessibilityStandards)
	}
	if !manifest.ArtifactContracts.StaleDataWarningPolicy.WarningRequiredWhenSourceStale ||
		manifest.ArtifactContracts.StaleDataWarningPolicy.WarningMarker != "stale_data_warning" ||
		!manifest.ArtifactContracts.StaleDataWarningPolicy.StaleSourceRefsMustBeNamed ||
		!manifest.ArtifactContracts.StaleDataWarningPolicy.SnapshotsMustCarryWarningRefs {
		t.Fatalf("stale data warning policy not strict enough: %#v", manifest.ArtifactContracts.StaleDataWarningPolicy)
	}
	for _, want := range []string{"id", "title", "kind", "locator", "as_of", "stale_after", "stale", "license", "retrieved_at", "evidence_hash"} {
		if !contains(manifest.ArtifactContracts.SourceAnnotationRequirements, want) {
			t.Fatalf("source annotation requirements missing %q: %#v", want, manifest.ArtifactContracts.SourceAnnotationRequirements)
		}
	}
	for _, marker := range []string{"source_annotations", "stale_data_warnings", "ai_disclosure", "renderer_contract", "snapshot_metadata", "accessibility_checklist", "source_annotation_requirements", "provider_free"} {
		if !contains(manifest.ArtifactContracts.RequiredMarkers, marker) {
			t.Fatalf("artifact contract markers missing %q", marker)
		}
	}
	if schema.SchemaVersion != 1 {
		t.Fatalf("schema version = %d, want 1", schema.SchemaVersion)
	}
	if schema.ProviderFreeGate.NetworkRequired || schema.ProviderFreeGate.CredentialsRequired || schema.ProviderFreeGate.ProviderSDKsRequired {
		t.Fatalf("schema provider-free gate must require no network, credentials, or provider SDKs: %#v", schema.ProviderFreeGate)
	}
	for _, key := range []string{"finance_normalizer", "valuation_result", "sensitivity_matrix", "chart_artifact", "snapshot_metadata", "accessibility_checklist", "stale_data_policy", "source_annotation_requirements", "report_artifact", "renderer_contract"} {
		if len(schema.Schemas[key].Required) == 0 {
			t.Fatalf("schema %q missing required fields", key)
		}
	}
	if !contains(schema.Schemas["renderer_contract"].Formats, "text/html") || !contains(schema.Schemas["renderer_contract"].Formats, "application/pdf") {
		t.Fatalf("renderer formats = %#v, want HTML and PDF", schema.Schemas["renderer_contract"].Formats)
	}
	for _, want := range []string{"text/html", "application/pdf", "image/png"} {
		if !contains(schema.Schemas["snapshot_metadata"].Formats, want) {
			t.Fatalf("snapshot metadata formats missing %q: %#v", want, schema.Schemas["snapshot_metadata"].Formats)
		}
	}
	for _, want := range []string{"renderer_dependency_required", "warning_refs", "accessibility_refs"} {
		if !contains(schema.Schemas["snapshot_metadata"].Required, want) {
			t.Fatalf("snapshot metadata required fields missing %q: %#v", want, schema.Schemas["snapshot_metadata"].Required)
		}
	}
	for _, want := range []string{"report_sections", "chart_specs", "snapshot_metadata", "accessibility_checklist", "stale_data_policy", "source_annotation_requirements"} {
		if !contains(schema.Schemas["report_artifact"].Required, want) {
			t.Fatalf("report artifact required fields missing %q: %#v", want, schema.Schemas["report_artifact"].Required)
		}
	}
	for _, want := range []string{"snapshot_metadata_schema", "accessibility_checklist_schema", "stale_data_policy"} {
		if !contains(schema.Schemas["renderer_contract"].Required, want) {
			t.Fatalf("renderer contract required fields missing %q: %#v", want, schema.Schemas["renderer_contract"].Required)
		}
	}
	if !schema.Schemas["source_annotation_requirements"].AllReportSourceRefsMustResolve ||
		!schema.Schemas["source_annotation_requirements"].AllSnapshotSourceRefsMustResolve {
		t.Fatalf("source annotation refs must resolve: %#v", schema.Schemas["source_annotation_requirements"])
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
			assertVMValue(t, vm, "analytics_report_live_package_summary", "analytics_report_live_package provider_free=true normalizers=2 valuations=3 sensitivity=3x3 charts=2 artifacts=4 snapshots=4 a11y=4 warnings=1 html=text/html pdf=application/pdf")
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
			if len(prints) != 1 || prints[0] != "analytics_report_live_package provider_free=true normalizers=2 valuations=3 sensitivity=3x3 charts=2 artifacts=4 snapshots=4 a11y=4 warnings=1 html=text/html pdf=application/pdf" {
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

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
