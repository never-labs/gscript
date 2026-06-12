package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericEvidenceReportArtifactsLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericEvidenceReportArtifactsPackageDir(t)

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
		Capabilities       []string          `json:"capabilities"`
		Contracts          map[string]string `json:"contracts"`
		Schemas            map[string]string `json:"schemas"`
		Fixtures           map[string]string `json:"fixtures"`
		NoBuiltInGuarantee struct {
			Required  bool   `json:"required"`
			Statement string `json:"statement"`
		} `json:"no_built_in_guarantee"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	if manifest.SchemaVersion != 1 ||
		manifest.ID != "generic-evidence-report-artifacts" ||
		manifest.PackageName != "leia-generic-ai-evidence-report-artifacts" ||
		manifest.PackageBoundaryID != "generic-ai-evidence-report-artifacts" ||
		manifest.CapabilityID != "generic.ai.evidence.report.artifacts" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetworkDefault || manifest.LiveModelDefault || manifest.DependsOnQRuntime {
		t.Fatalf("manifest must stay provider-free/generic/offline: %#v", manifest)
	}
	statement := strings.ToLower(manifest.NoBuiltInGuarantee.Statement)
	if !manifest.NoBuiltInGuarantee.Required ||
		!strings.Contains(statement, "leia core") ||
		!strings.Contains(statement, "does not provide") ||
		!strings.Contains(statement, "built-in") ||
		!strings.Contains(statement, manifest.PackageName) ||
		!strings.Contains(statement, "package boundary") {
		t.Fatalf("manifest missing no-built-in boundary: %#v", manifest.NoBuiltInGuarantee)
	}
	for _, want := range []string{
		"generic.ai.evidence.report.artifacts",
		"generic.ai.evidence.source_annotation",
		"generic.ai.evidence.citation_envelope",
		"generic.ai.report.outline",
		"generic.ai.report.section_dependency_dag",
		"generic.ai.artifact.manifest",
		"generic.ai.artifact.render_manifest",
		"generic.ai.artifact.snapshot_metadata",
		"generic.ai.artifact.accessibility_checklist",
		"generic.ai.render.request",
		"generic.ai.render.output_manifest",
		"generic.ai.render.clean_skip",
	} {
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
		LiveModel             bool              `json:"live_model"`
		LiveModelCalls        bool              `json:"live_model_calls"`
		RealDependencyImports bool              `json:"real_dependency_imports"`
		RequiresCredentials   bool              `json:"requires_credentials"`
		ProviderSDKsRequired  bool              `json:"provider_sdks_required"`
		FieldContracts        map[string]string `json:"field_contracts"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Contracts["contract"]), &contract)
	if contract.SchemaVersion != 1 || contract.PackageBoundaryID != manifest.PackageBoundaryID ||
		contract.PackageName != "generic.ai.evidence.report.artifacts" || contract.Entrypoint != "ai.evidence.report_artifacts" ||
		!contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork || contract.LiveModel ||
		contract.LiveModelCalls || contract.RealDependencyImports || contract.RequiresCredentials || contract.ProviderSDKsRequired {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	for _, want := range []string{"source_annotations", "citation_envelopes", "section_dependency_dag", "artifact_manifest", "render_manifest", "snapshot_metadata", "stale_data_policy", "accessibility_checklist"} {
		if contract.FieldContracts[want] == "" {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
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
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Fixtures["index"]), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 1 {
		t.Fatalf("fixture index header/count mismatch: %#v", index)
	}
	fixture := index.Fixtures[0]
	if fixture.FixtureKey != "generic:evidence_report_artifacts:offline" ||
		fixture.Capability != "generic.ai.evidence.report.artifacts" ||
		fixture.Path != manifest.Fixtures["evidence_report_artifacts"] ||
		fixture.Schema != manifest.Schemas["evidence_report_artifacts"] ||
		fixture.Metadata["replay_ready"] != true {
		t.Fatalf("fixture index entry mismatch: %#v", fixture)
	}
}

func TestGenericEvidenceReportArtifactsLivePackageFixtureShape(t *testing.T) {
	base := genericEvidenceReportArtifactsPackageDir(t)
	fixture := loadGenericEvidenceReportArtifactsFixture(t, filepath.Join(base, "fixtures", "generic_evidence_report_artifacts_fixture.json"))
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.LiveModelCalls {
		t.Fatalf("fixture must stay provider-free and offline: %#v", fixture)
	}
	if len(fixture.SourceAnnotations) != 2 || len(fixture.CitationEnvelopes) != 2 ||
		len(fixture.ArtifactManifest.Artifacts) != 3 || len(fixture.SnapshotMetadata) != 2 || len(fixture.Warnings) != 2 {
		t.Fatalf("fixture counts drifted: sources=%d citations=%d artifacts=%d snapshots=%d warnings=%d",
			len(fixture.SourceAnnotations), len(fixture.CitationEnvelopes), len(fixture.ArtifactManifest.Artifacts), len(fixture.SnapshotMetadata), len(fixture.Warnings))
	}

	sourceIDs := map[string]bool{}
	for _, source := range fixture.SourceAnnotations {
		if source.ID == "" || source.Title == "" || source.Kind == "" || source.Locator == "" || source.EvidenceHash == "" {
			t.Fatalf("source annotation incomplete: %#v", source)
		}
		sourceIDs[source.ID] = true
	}
	for _, envelope := range fixture.CitationEnvelopes {
		if envelope.ID == "" || envelope.ClaimID == "" || !envelope.ProviderFree || len(envelope.SourceRefs) == 0 || len(envelope.CitationRefs) == 0 || len(envelope.UnresolvedRefs) != 0 {
			t.Fatalf("citation envelope incomplete: %#v", envelope)
		}
		for _, ref := range envelope.SourceRefs {
			if !sourceIDs[ref] {
				t.Fatalf("citation envelope source_ref %q does not resolve", ref)
			}
		}
		for _, ref := range envelope.CitationRefs {
			if !sourceIDs[ref.SourceID] {
				t.Fatalf("citation envelope citation_ref %q does not resolve", ref.SourceID)
			}
		}
	}
	if !fixture.SectionDependencyDAG.Acyclic || fixture.ReportOutline.RenderManifestID != fixture.RenderManifest.ID {
		t.Fatalf("report outline/DAG mismatch: outline=%#v dag=%#v", fixture.ReportOutline, fixture.SectionDependencyDAG)
	}
	warningIDs := map[string]bool{}
	for _, warning := range fixture.Warnings {
		if warning.ID == "" || warning.Kind == "" || warning.Severity == "" {
			t.Fatalf("warning incomplete: %#v", warning)
		}
		warningIDs[warning.ID] = true
		for _, ref := range warning.SourceRefs {
			if !sourceIDs[ref] {
				t.Fatalf("warning source_ref %q does not resolve", ref)
			}
		}
	}
	for _, artifact := range fixture.ArtifactManifest.Artifacts {
		if artifact.ArtifactID == "" || artifact.Kind == "" || artifact.Format == "" || artifact.Status == "" || artifact.ContentHash == "" {
			t.Fatalf("artifact incomplete: %#v", artifact)
		}
		for _, ref := range artifact.WarningRefs {
			if !warningIDs[ref] {
				t.Fatalf("artifact warning_ref %q does not resolve", ref)
			}
		}
	}
	gate := fixture.RenderRequest.RendererDependencyGate
	if gate.DependencyImported || gate.CredentialRequired || gate.LiveNetwork || !gate.CleanSkip {
		t.Fatalf("renderer dependency gate must clean-skip: %#v", gate)
	}
	for _, boundary := range fixture.AdapterBoundaries {
		if boundary.DependencyImported || boundary.CredentialRequired || boundary.LiveNetwork || !boundary.CleanSkip {
			t.Fatalf("adapter boundary must be provider-free clean-skip: %#v", boundary)
		}
	}
}

func TestGenericEvidenceReportArtifactsLivePackageIsDomainNeutral(t *testing.T) {
	base := genericEvidenceReportArtifactsPackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"finrobot", "acme", "aapl", "ticker", "equity", "investment", "valuation", "dcf", "target_price", "sec.gov", "10-k", "10-q", "product.workflow", "finance."} {
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

func TestGenericEvidenceReportArtifactsLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericEvidenceReportArtifactsPackageDir(t)
	schema := filepath.Join(base, "schemas", "generic_evidence_report_artifacts_v1.schema.json")
	assertDocumentPipelineSchemaRequired(t, schema, []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "live_model_calls", "source_annotations", "citation_envelopes", "report_outline", "section_dependency_dag", "style_profile_policy", "partial_report_fixture", "artifact_manifest", "render_manifest", "render_request", "output_manifest", "snapshot_metadata", "warnings", "accessibility_checklist", "adapter_boundaries"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "source_annotations", "items"}, []string{"id", "title", "kind", "locator", "as_of", "stale_after", "stale", "license", "retrieved_at", "evidence_hash"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "citation_envelopes", "items"}, []string{"id", "claim_id", "source_refs", "citation_refs", "evidence_quality", "provider_free", "unresolved_refs"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "snapshot_metadata", "items"}, []string{"snapshot_id", "format", "viewport", "dimensions", "status", "content_hash", "hash_algorithm", "source_refs", "warning_refs", "disclosure_refs"})
}

func TestGenericEvidenceReportArtifactsLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericEvidenceReportArtifactsPackageDir(t), "main.leia")
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
			got, err := vm.Get("generic_evidence_report_artifacts_live_package_summary")
			if err != nil {
				t.Fatalf("Get generic_evidence_report_artifacts_live_package_summary: %v", err)
			}
			want := "generic_evidence_report_artifacts_live_package capability=generic.ai.evidence.report.artifacts entrypoint=ai.evidence.report_artifacts sources=2 evidence=2 artifacts=3 snapshots=2 warnings=2 clean_skip=2 provider_free=true live_network=false imports=false model_calls=false"
			if got != want {
				t.Fatalf("summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

type genericEvidenceReportArtifactsFixture struct {
	ProviderFree          bool `json:"provider_free"`
	LiveNetwork           bool `json:"live_network"`
	RealDependencyImports bool `json:"real_dependency_imports"`
	LiveModelCalls        bool `json:"live_model_calls"`
	SourceAnnotations     []struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		Kind         string `json:"kind"`
		Locator      string `json:"locator"`
		EvidenceHash string `json:"evidence_hash"`
	} `json:"source_annotations"`
	CitationEnvelopes []struct {
		ID           string   `json:"id"`
		ClaimID      string   `json:"claim_id"`
		SourceRefs   []string `json:"source_refs"`
		CitationRefs []struct {
			SourceID string `json:"source_id"`
		} `json:"citation_refs"`
		ProviderFree   bool     `json:"provider_free"`
		UnresolvedRefs []string `json:"unresolved_refs"`
	} `json:"citation_envelopes"`
	ReportOutline struct {
		RenderManifestID string `json:"render_manifest_id"`
	} `json:"report_outline"`
	SectionDependencyDAG struct {
		Acyclic bool `json:"acyclic"`
	} `json:"section_dependency_dag"`
	ArtifactManifest struct {
		Artifacts []struct {
			ArtifactID  string   `json:"artifact_id"`
			Kind        string   `json:"kind"`
			Format      string   `json:"format"`
			Status      string   `json:"status"`
			ContentHash string   `json:"content_hash"`
			WarningRefs []string `json:"warning_refs"`
		} `json:"artifacts"`
	} `json:"artifact_manifest"`
	RenderManifest struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"render_manifest"`
	RenderRequest struct {
		RendererDependencyGate struct {
			DependencyImported bool `json:"dependency_imported"`
			CredentialRequired bool `json:"credential_required"`
			LiveNetwork        bool `json:"live_network"`
			CleanSkip          bool `json:"clean_skip"`
		} `json:"renderer_dependency_gate"`
	} `json:"render_request"`
	SnapshotMetadata []struct {
		SnapshotID  string   `json:"snapshot_id"`
		SourceRefs  []string `json:"source_refs"`
		WarningRefs []string `json:"warning_refs"`
	} `json:"snapshot_metadata"`
	Warnings []struct {
		ID         string   `json:"id"`
		Kind       string   `json:"kind"`
		Severity   string   `json:"severity"`
		SourceRefs []string `json:"source_refs"`
	} `json:"warnings"`
	AdapterBoundaries []struct {
		ID                 string `json:"id"`
		DependencyImported bool   `json:"dependency_imported"`
		CredentialRequired bool   `json:"credential_required"`
		LiveNetwork        bool   `json:"live_network"`
		CleanSkip          bool   `json:"clean_skip"`
	} `json:"adapter_boundaries"`
}

func loadGenericEvidenceReportArtifactsFixture(t *testing.T, path string) genericEvidenceReportArtifactsFixture {
	t.Helper()
	var fixture genericEvidenceReportArtifactsFixture
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func genericEvidenceReportArtifactsPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_evidence_report_artifacts")
}
