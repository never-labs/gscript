package leia_test

import (
	"os"
	"path/filepath"
	"reflect"
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
		"generic.ai.evidence.report.render_projection",
		"generic.ai.evidence.report.chart_artifact_projection",
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
	for _, want := range []string{"source_annotations", "citation_envelopes", "section_dependency_dag", "artifact_manifest", "render_manifest", "verification_render_projection", "snapshot_metadata", "stale_data_policy", "accessibility_checklist"} {
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
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 3 {
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
	projection := index.Fixtures[1]
	if projection.FixtureKey != "generic:evidence_report_artifacts:verification_render_projection" ||
		projection.Capability != "generic.ai.evidence.report.render_projection" ||
		projection.Path != manifest.Fixtures["verification_render_projection"] ||
		projection.Schema != manifest.Schemas["verification_render_projection"] ||
		projection.Metadata["replay_ready"] != true ||
		projection.Metadata["source_mappings"] != float64(4) ||
		projection.Metadata["render_request_mappings"] != float64(2) {
		t.Fatalf("projection fixture index entry mismatch: %#v", projection)
	}
	chartProjection := index.Fixtures[2]
	if chartProjection.FixtureKey != "generic:evidence_report_artifacts:chart_render_artifact_projection" ||
		chartProjection.Capability != "generic.ai.evidence.report.chart_artifact_projection" ||
		chartProjection.Path != manifest.Fixtures["chart_render_artifact_projection"] ||
		chartProjection.Schema != manifest.Schemas["chart_render_artifact_projection"] ||
		chartProjection.Metadata["replay_ready"] != true ||
		chartProjection.Metadata["chart_artifact_mappings"] != float64(1) ||
		chartProjection.Metadata["chart_warning_mappings"] != float64(2) ||
		chartProjection.Metadata["source_metadata_mappings"] != float64(2) ||
		chartProjection.Metadata["snapshot_mappings"] != float64(2) {
		t.Fatalf("chart projection fixture index entry mismatch: %#v", chartProjection)
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
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "verification_render_projection_v1.schema.json"), []string{"schema_version", "id", "projection_kind", "package_boundary_id", "provider_free", "domain_specific", "live_network", "live_model_calls", "real_dependency_imports", "source_fixture_refs", "canonical_report_id", "identity_policy", "source_mappings", "citation_mappings", "warning_mappings", "artifact_mappings", "render_request_mappings", "projection_assertions"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "chart_render_artifact_projection_v1.schema.json"), []string{"schema_version", "id", "projection_kind", "package_boundary_id", "provider_free", "domain_specific", "live_network", "live_model_calls", "real_dependency_imports", "source_fixture_refs", "chart_artifact_mappings", "chart_warning_mappings", "source_metadata_mappings", "snapshot_mappings", "projection_assertions"})
}

func TestGenericEvidenceReportArtifactsVerificationRenderProjection(t *testing.T) {
	base := genericEvidenceReportArtifactsPackageDir(t)
	report := loadGenericEvidenceReportArtifactsFixture(t, filepath.Join(base, "fixtures", "generic_evidence_report_artifacts_fixture.json"))
	var projection struct {
		SchemaVersion         int    `json:"schema_version"`
		ID                    string `json:"id"`
		ProjectionKind        string `json:"projection_kind"`
		PackageBoundaryID     string `json:"package_boundary_id"`
		ProviderFree          bool   `json:"provider_free"`
		DomainSpecific        bool   `json:"domain_specific"`
		LiveNetwork           bool   `json:"live_network"`
		LiveModelCalls        bool   `json:"live_model_calls"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		SourceFixtureRefs     struct {
			EvidenceVerification   string `json:"evidence_verification"`
			EvidenceReportArtifact string `json:"evidence_report_artifacts"`
			ReportRenderContracts  string `json:"report_render_contracts"`
		} `json:"source_fixture_refs"`
		CanonicalReportID string `json:"canonical_report_id"`
		IdentityPolicy    struct {
			SourceIDsAreNotAssumedEqual        bool   `json:"source_ids_are_not_assumed_equal"`
			CitationIDsAreNotAssumedEqual      bool   `json:"citation_ids_are_not_assumed_equal"`
			RenderRequestIDsAreNotAssumedEqual bool   `json:"render_request_ids_are_not_assumed_equal"`
			NormalizationPolicy                string `json:"normalization_policy"`
		} `json:"identity_policy"`
		SourceMappings []struct {
			VerificationSourceID string   `json:"verification_source_id"`
			ReportSourceID       string   `json:"report_source_id"`
			SnapshotRefs         []string `json:"snapshot_refs"`
			EvidenceHashPreserve bool     `json:"evidence_hash_preserved"`
		} `json:"source_mappings"`
		CitationMappings []struct {
			ClaimID                  string   `json:"claim_id"`
			VerificationStatus       string   `json:"verification_status"`
			ReportCitationEnvelopeID string   `json:"report_citation_envelope_id"`
			RenderAnnotationRefs     []string `json:"render_annotation_refs"`
			ProjectedAction          string   `json:"projected_action"`
		} `json:"citation_mappings"`
		WarningMappings []struct {
			VerificationWarningRef string `json:"verification_warning_ref"`
			ReportWarningRef       string `json:"report_warning_ref"`
			RenderWarningRef       string `json:"render_warning_ref"`
			Severity               string `json:"severity"`
		} `json:"warning_mappings"`
		ArtifactMappings []struct {
			ReportArtifactID  string `json:"report_artifact_id"`
			RenderArtifactID  string `json:"render_artifact_id"`
			OutputManifestRef string `json:"output_manifest_ref"`
			URIPolicy         string `json:"uri_policy"`
			RemoteFetch       bool   `json:"remote_fetch"`
		} `json:"artifact_mappings"`
		RenderRequestMappings []struct {
			ReportRenderRequestID   string `json:"report_render_request_id"`
			RenderContractRequestID string `json:"render_contract_request_id"`
			Format                  string `json:"format"`
			CleanSkip               bool   `json:"clean_skip"`
			ProviderFree            bool   `json:"provider_free"`
		} `json:"render_request_mappings"`
		ProjectionAssertions map[string]bool `json:"projection_assertions"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "verification_render_projection_fixture.json"), &projection)
	if projection.SchemaVersion != 1 ||
		projection.ID != "generic-evidence-verification-render-projection-fixture" ||
		projection.ProjectionKind != "evidence_verification_to_report_render_projection" ||
		projection.PackageBoundaryID != "generic-ai-evidence-report-artifacts" {
		t.Fatalf("unexpected projection identity: %#v", projection)
	}
	if !projection.ProviderFree || projection.DomainSpecific || projection.LiveNetwork ||
		projection.LiveModelCalls || projection.RealDependencyImports {
		t.Fatalf("projection must stay provider-free/offline: %#v", projection)
	}
	if projection.SourceFixtureRefs.EvidenceVerification == "" ||
		projection.SourceFixtureRefs.EvidenceReportArtifact == "" ||
		projection.SourceFixtureRefs.ReportRenderContracts == "" ||
		projection.CanonicalReportID != "generic-report" {
		t.Fatalf("projection source refs incomplete: %#v", projection.SourceFixtureRefs)
	}
	if !projection.IdentityPolicy.SourceIDsAreNotAssumedEqual ||
		!projection.IdentityPolicy.CitationIDsAreNotAssumedEqual ||
		!projection.IdentityPolicy.RenderRequestIDsAreNotAssumedEqual {
		t.Fatalf("projection must not assume cross-package ID equality: %#v", projection.IdentityPolicy)
	}
	reportSourceIDs := map[string]bool{}
	for _, source := range report.SourceAnnotations {
		reportSourceIDs[source.ID] = true
	}
	reportSnapshotIDs := map[string]bool{}
	for _, snapshot := range report.SnapshotMetadata {
		reportSnapshotIDs[snapshot.SnapshotID] = true
	}
	for _, mapping := range projection.SourceMappings {
		if mapping.VerificationSourceID == "" || !reportSourceIDs[mapping.ReportSourceID] || !mapping.EvidenceHashPreserve {
			t.Fatalf("source mapping does not resolve: %#v", mapping)
		}
		for _, ref := range mapping.SnapshotRefs {
			if !reportSnapshotIDs[ref] {
				t.Fatalf("source mapping snapshot ref %q does not resolve: %#v", ref, mapping)
			}
		}
	}
	reportCitationIDs := map[string]bool{}
	for _, citation := range report.CitationEnvelopes {
		reportCitationIDs[citation.ID] = true
	}
	for _, mapping := range projection.CitationMappings {
		if mapping.ClaimID == "" || mapping.VerificationStatus == "" || mapping.ProjectedAction == "" {
			t.Fatalf("citation mapping incomplete: %#v", mapping)
		}
		if mapping.ReportCitationEnvelopeID != "partial:generic-report" && !reportCitationIDs[mapping.ReportCitationEnvelopeID] {
			t.Fatalf("citation mapping envelope does not resolve: %#v", mapping)
		}
	}
	reportWarningIDs := map[string]bool{}
	for _, warning := range report.Warnings {
		reportWarningIDs[warning.ID] = true
	}
	for _, mapping := range projection.WarningMappings {
		if mapping.VerificationWarningRef == "" || !reportWarningIDs[mapping.ReportWarningRef] || mapping.RenderWarningRef == "" {
			t.Fatalf("warning mapping does not resolve: %#v", mapping)
		}
	}
	reportArtifactIDs := map[string]bool{}
	for _, artifact := range report.ArtifactManifest.Artifacts {
		reportArtifactIDs[artifact.ArtifactID] = true
	}
	for _, mapping := range projection.ArtifactMappings {
		if !reportArtifactIDs[mapping.ReportArtifactID] || mapping.RenderArtifactID == "" ||
			mapping.OutputManifestRef == "" || mapping.URIPolicy != "artifact_uri_only" || mapping.RemoteFetch {
			t.Fatalf("artifact mapping invalid: %#v", mapping)
		}
	}
	if len(projection.RenderRequestMappings) != 2 {
		t.Fatalf("render request mappings = %d, want 2", len(projection.RenderRequestMappings))
	}
	for _, mapping := range projection.RenderRequestMappings {
		if mapping.ReportRenderRequestID != report.RenderRequest.RequestID ||
			mapping.RenderContractRequestID == "" || mapping.Format == "" || !mapping.ProviderFree {
			t.Fatalf("render request mapping invalid: %#v", mapping)
		}
	}
	for _, want := range []string{"all_verified_claims_project_to_citation_or_warning", "all_report_artifacts_project_to_render_artifacts", "all_render_outputs_use_artifact_uri_only", "warning_refs_preserved_across_packages", "raw_provider_payloads_absent", "live_network_absent", "real_dependency_imports_absent"} {
		if !projection.ProjectionAssertions[want] {
			t.Fatalf("projection assertion missing %q: %#v", want, projection.ProjectionAssertions)
		}
	}
}

func TestGenericEvidenceReportArtifactsChartRenderArtifactProjection(t *testing.T) {
	base := genericEvidenceReportArtifactsPackageDir(t)
	root := repoRoot(t)
	report := loadGenericEvidenceReportArtifactsFixture(t, filepath.Join(base, "fixtures", "generic_evidence_report_artifacts_fixture.json"))
	chart := loadGenericChartRenderContractsFixture(t, filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "generic_chart_render_contracts", "fixtures", "generic_chart_render_contracts_fixture.json"))

	var projection struct {
		SchemaVersion         int    `json:"schema_version"`
		ID                    string `json:"id"`
		ProjectionKind        string `json:"projection_kind"`
		PackageBoundaryID     string `json:"package_boundary_id"`
		ProviderFree          bool   `json:"provider_free"`
		DomainSpecific        bool   `json:"domain_specific"`
		LiveNetwork           bool   `json:"live_network"`
		LiveModelCalls        bool   `json:"live_model_calls"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		SourceFixtureRefs     struct {
			ChartRenderContracts   string `json:"chart_render_contracts"`
			EvidenceReportArtifact string `json:"evidence_report_artifacts"`
		} `json:"source_fixture_refs"`
		ChartArtifactMappings []struct {
			ChartResultRequestID string `json:"chart_result_request_id"`
			ChartArtifactID      string `json:"chart_artifact_id"`
			ReportArtifactID     string `json:"report_artifact_id"`
			ReportArtifactKind   string `json:"report_artifact_kind"`
			MediaType            string `json:"media_type"`
			URIPolicy            string `json:"uri_policy"`
			RemoteFetch          bool   `json:"remote_fetch"`
			ProviderFree         bool   `json:"provider_free"`
			ProjectedStatus      string `json:"projected_status"`
			ContentHashSource    string `json:"content_hash_source"`
		} `json:"chart_artifact_mappings"`
		ChartWarningMappings []struct {
			ChartResultRequestID string `json:"chart_result_request_id"`
			ChartSpecID          string `json:"chart_spec_id"`
			ChartWarningRef      string `json:"chart_warning_ref"`
			ReportWarningRef     string `json:"report_warning_ref"`
			Severity             string `json:"severity"`
			CleanSkip            bool   `json:"clean_skip"`
			ProviderFree         bool   `json:"provider_free"`
		} `json:"chart_warning_mappings"`
		SourceMetadataMappings []struct {
			ChartSpecID           string `json:"chart_spec_id"`
			ChartSourceFixtureKey string `json:"chart_source_fixture_key"`
			ReportSourceID        string `json:"report_source_id"`
			SourceLocatorPolicy   string `json:"source_locator_policy"`
			Stale                 bool   `json:"stale"`
			ReplayReady           bool   `json:"replay_ready"`
		} `json:"source_metadata_mappings"`
		SnapshotMappings []struct {
			ChartResultRequestID         string   `json:"chart_result_request_id"`
			ChartSnapshotHash            string   `json:"chart_snapshot_hash"`
			ReportSnapshotID             string   `json:"report_snapshot_id"`
			DeterministicInputsPreserved []string `json:"deterministic_inputs_preserved"`
			ProviderFree                 bool     `json:"provider_free"`
		} `json:"snapshot_mappings"`
		ProjectionAssertions map[string]bool `json:"projection_assertions"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "chart_render_artifact_projection_fixture.json"), &projection)
	if projection.SchemaVersion != 1 ||
		projection.ID != "generic-chart-render-artifact-projection-fixture" ||
		projection.ProjectionKind != "chart_render_to_evidence_report_artifact_projection" ||
		projection.PackageBoundaryID != "generic-ai-evidence-report-artifacts" ||
		!projection.ProviderFree || projection.DomainSpecific || projection.LiveNetwork ||
		projection.LiveModelCalls || projection.RealDependencyImports ||
		projection.SourceFixtureRefs.ChartRenderContracts == "" ||
		projection.SourceFixtureRefs.EvidenceReportArtifact == "" {
		t.Fatalf("chart projection header/provider boundary mismatch: %#v", projection)
	}

	reportArtifacts := map[string]struct {
		kind   string
		status string
	}{}
	for _, artifact := range report.ArtifactManifest.Artifacts {
		reportArtifacts[artifact.ArtifactID] = struct {
			kind   string
			status string
		}{kind: artifact.Kind, status: artifact.Status}
	}
	reportWarnings := map[string]string{}
	for _, warning := range report.Warnings {
		reportWarnings[warning.ID] = warning.Severity
	}
	reportSources := map[string]bool{}
	for _, source := range report.SourceAnnotations {
		reportSources[source.ID] = true
	}
	reportSnapshots := map[string]bool{}
	for _, snapshot := range report.SnapshotMetadata {
		reportSnapshots[snapshot.SnapshotID] = true
	}

	chartResults := map[string]genericChartRenderResult{}
	for _, result := range chart.RenderResults {
		chartResults[result.RequestID] = result
	}
	chartSpecs := map[string]genericChartRenderSpec{}
	for _, spec := range chart.ChartSpecs {
		chartSpecs[spec.ChartID] = spec
	}
	chartSkipWarnings := map[string]bool{}
	for _, skip := range chart.RendererSkips {
		chartSkipWarnings[skip.ID] = skip.CleanSkip
	}

	if len(projection.ChartArtifactMappings) != 1 {
		t.Fatalf("chart artifact mappings = %d, want 1", len(projection.ChartArtifactMappings))
	}
	for _, mapping := range projection.ChartArtifactMappings {
		result := chartResults[mapping.ChartResultRequestID]
		reportArtifact, ok := reportArtifacts[mapping.ReportArtifactID]
		if !ok || !result.OK || result.Artifact == nil ||
			mapping.ChartArtifactID != result.Artifact.ArtifactID ||
			mapping.MediaType != result.Artifact.MediaType ||
			mapping.ContentHashSource != result.Artifact.Hash ||
			mapping.ReportArtifactKind != reportArtifact.kind ||
			mapping.ProjectedStatus != reportArtifact.status ||
			mapping.URIPolicy != "artifact_uri_only" ||
			mapping.RemoteFetch || !mapping.ProviderFree {
			t.Fatalf("chart artifact mapping does not resolve: %#v", mapping)
		}
	}
	if len(projection.ChartWarningMappings) != 2 {
		t.Fatalf("chart warning mappings = %d, want 2", len(projection.ChartWarningMappings))
	}
	for _, mapping := range projection.ChartWarningMappings {
		if reportWarnings[mapping.ReportWarningRef] != mapping.Severity || !mapping.ProviderFree {
			t.Fatalf("chart warning mapping report warning unresolved: %#v", mapping)
		}
		if mapping.ChartResultRequestID != "" {
			result := chartResults[mapping.ChartResultRequestID]
			if result.OK || !genericEvidenceReportStringSliceContains(result.Warnings, mapping.ChartWarningRef) || !chartSkipWarnings[mapping.ChartWarningRef] || !mapping.CleanSkip {
				t.Fatalf("renderer clean-skip warning mapping invalid: %#v", mapping)
			}
		}
		if mapping.ChartSpecID != "" {
			spec := chartSpecs[mapping.ChartSpecID]
			if spec.StaleDataWarning == nil ||
				spec.StaleDataWarning.Code != mapping.ChartWarningRef ||
				!spec.SourceMetadata.IsStale ||
				mapping.CleanSkip {
				t.Fatalf("stale chart source warning mapping invalid: %#v", mapping)
			}
		}
	}
	for _, mapping := range projection.SourceMetadataMappings {
		spec := chartSpecs[mapping.ChartSpecID]
		if spec.SourceMetadata.FixtureKey == "" ||
			mapping.ChartSourceFixtureKey != spec.SourceMetadata.FixtureKey ||
			mapping.Stale != spec.SourceMetadata.IsStale ||
			mapping.ReplayReady != spec.SourceMetadata.ReplayReady ||
			!reportSources[mapping.ReportSourceID] ||
			mapping.SourceLocatorPolicy != "redacted_fixture_locator_only" {
			t.Fatalf("chart source metadata mapping invalid: %#v", mapping)
		}
	}
	for _, mapping := range projection.SnapshotMappings {
		result := chartResults[mapping.ChartResultRequestID]
		if result.Snapshot.Hash == "" ||
			mapping.ChartSnapshotHash != result.Snapshot.Hash ||
			!reportSnapshots[mapping.ReportSnapshotID] ||
			!reflect.DeepEqual(mapping.DeterministicInputsPreserved, result.Snapshot.DeterministicInputs) ||
			!mapping.ProviderFree {
			t.Fatalf("chart snapshot mapping invalid: %#v", mapping)
		}
	}
	for _, want := range []string{"successful_chart_artifacts_project_to_report_artifacts", "unsupported_renderer_projects_to_clean_skip_warning", "stale_chart_source_projects_to_report_warning", "chart_source_metadata_resolves_to_report_sources", "snapshot_hashes_are_preserved_as_metadata", "artifact_uris_remain_metadata_refs_only", "raw_chart_payloads_absent", "live_network_absent", "real_dependency_imports_absent"} {
		if !projection.ProjectionAssertions[want] {
			t.Fatalf("chart projection assertion missing %q: %#v", want, projection.ProjectionAssertions)
		}
	}
}

func TestGenericEvidenceReportArtifactsLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericEvidenceReportArtifactsPackageDir(t), "main.leia")
	want := "generic_evidence_report_artifacts_live_package capability=generic.ai.evidence.report.artifacts entrypoint=ai.evidence.report_artifacts sources=2 evidence=2 artifacts=3 snapshots=2 warnings=2 clean_skip=2 chart_artifact_projections=1 chart_warning_projections=2 provider_free=true live_network=false imports=false model_calls=false"
	for _, result := range runFinRobotLivePackageSummarySmoke(t, path, "generic_evidence_report_artifacts_live_package_summary", "generic_evidence_report_artifacts_live_package", leia.LibString) {
		if result.Summary != want {
			t.Fatalf("summary = %#v, want %#v", result.Summary, want)
		}
		fields := result.Fields
		requireFinRobotSummaryFields(t, fields, "capability", "entrypoint", "sources", "evidence", "artifacts", "snapshots", "warnings", "clean_skip", "chart_artifact_projections", "chart_warning_projections", "provider_free", "live_network", "imports", "model_calls")
		if fields["capability"] != "generic.ai.evidence.report.artifacts" ||
			fields["entrypoint"] != "ai.evidence.report_artifacts" ||
			fields["sources"] != "2" ||
			fields["evidence"] != "2" ||
			fields["artifacts"] != "3" ||
			fields["snapshots"] != "2" ||
			fields["warnings"] != "2" ||
			fields["clean_skip"] != "2" ||
			fields["chart_artifact_projections"] != "1" ||
			fields["chart_warning_projections"] != "2" ||
			fields["provider_free"] != "true" ||
			fields["live_network"] != "false" ||
			fields["imports"] != "false" ||
			fields["model_calls"] != "false" {
			t.Fatalf("summary fields = %#v", fields)
		}
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
		RequestID              string `json:"request_id"`
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

func genericEvidenceReportStringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func genericEvidenceReportArtifactsPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_evidence_report_artifacts")
}
