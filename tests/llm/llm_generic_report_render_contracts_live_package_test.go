package leia_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericReportRenderContractsLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericReportRenderContractsPackageDir(t)
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
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	if manifest.SchemaVersion != 1 || manifest.ID != "generic-report-render-contracts" ||
		manifest.PackageName != "leia-generic-ai-report-render-contracts" ||
		manifest.PackageBoundaryID != "generic-ai-report-render-contracts" ||
		manifest.CapabilityID != "generic.ai.report.render.contracts" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetworkDefault ||
		manifest.LiveModelDefault || manifest.DependsOnQRuntime || manifest.CredentialRequired {
		t.Fatalf("manifest must stay provider-free/generic/offline/credential-free: %#v", manifest)
	}
	for _, want := range []string{"generic.ai.report.render.contracts", "generic.ai.report.render.request", "generic.ai.report.render.output_manifest", "generic.ai.report.render.artifact_manifest", "generic.ai.report.render.snapshot_metadata", "generic.ai.report.render.warning", "generic.ai.report.render.annotation", "generic.ai.report.render.fixture_hash", "generic.ai.report.render.clean_skip"} {
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
		contract.PackageName != "generic.ai.report.render.contracts" || contract.Entrypoint != "ai.report.render_contracts" ||
		!contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork ||
		contract.LiveModelCalls || contract.RealDependencyImports || contract.RequiresCredentials {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	for _, want := range []string{"render_request", "request_envelope", "render_result", "output_manifest", "artifact_manifest", "snapshot_metadata", "warnings", "annotations", "fixture_hash", "clean_skip"} {
		if contract.FieldContracts[want] == "" {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
		}
	}
}

func TestGenericReportRenderContractsLivePackageFixtureShape(t *testing.T) {
	base := genericReportRenderContractsPackageDir(t)
	fixture := loadGenericReportRenderContractsFixture(t, filepath.Join(base, "fixtures", "report_render_contracts_fixture.json"))
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.LiveModelCalls {
		t.Fatalf("fixture must stay provider-free and offline: %#v", fixture)
	}
	if len(fixture.RenderRequests) != 2 || len(fixture.RequestEnvelopes) != 2 ||
		len(fixture.RenderResults) != 2 || len(fixture.OutputManifests) != 2 ||
		len(fixture.ArtifactManifest.Artifacts) != 3 || len(fixture.SnapshotMetadata) != 3 ||
		len(fixture.Warnings) != 3 || len(fixture.Annotations) != 2 ||
		len(fixture.FixtureHashes) != 2 || len(fixture.RendererSkips) != 1 ||
		len(fixture.AdapterBoundaries) != 2 {
		t.Fatalf("fixture counts drifted: requests=%d envelopes=%d results=%d outputs=%d artifacts=%d snapshots=%d warnings=%d annotations=%d hashes=%d skips=%d adapters=%d",
			len(fixture.RenderRequests), len(fixture.RequestEnvelopes), len(fixture.RenderResults), len(fixture.OutputManifests), len(fixture.ArtifactManifest.Artifacts), len(fixture.SnapshotMetadata), len(fixture.Warnings), len(fixture.Annotations), len(fixture.FixtureHashes), len(fixture.RendererSkips), len(fixture.AdapterBoundaries))
	}
	requests := map[string]bool{}
	for _, request := range fixture.RenderRequests {
		if request.RequestID == "" || request.RenderMode == "" || len(request.Formats) == 0 ||
			request.FixtureKey == "" || request.LiveNetwork || request.RealDependencyImports ||
			request.RendererDependencyGate.DependencyImported {
			t.Fatalf("render request invalid: %#v", request)
		}
		requests[request.RequestID] = true
	}
	outputs := map[string]bool{}
	for _, output := range fixture.OutputManifests {
		if output.OutputID == "" || output.MediaType == "" || len(output.ArtifactRefs) == 0 ||
			output.URIPolicy != "artifact_uri_only" || output.RemoteFetch {
			t.Fatalf("output manifest invalid: %#v", output)
		}
		outputs[output.OutputID] = true
	}
	snapshots := map[string]bool{}
	for _, snapshot := range fixture.SnapshotMetadata {
		if snapshot.SnapshotID == "" || snapshot.HashAlgorithm != "sha256" ||
			!strings.HasPrefix(snapshot.Hash, "sha256:") || len(snapshot.DeterministicInputs) == 0 {
			t.Fatalf("snapshot metadata invalid: %#v", snapshot)
		}
		snapshots[snapshot.SnapshotID] = true
	}
	warnings := map[string]bool{}
	for _, warning := range fixture.Warnings {
		if warning.WarningID == "" || warning.Kind == "" || warning.Severity == "" || warning.Message == "" {
			t.Fatalf("warning invalid: %#v", warning)
		}
		warnings[warning.WarningID] = true
	}
	annotations := map[string]bool{}
	for _, annotation := range fixture.Annotations {
		if annotation.AnnotationID == "" || annotation.Kind == "" || !annotation.Required || !annotation.Resolved {
			t.Fatalf("annotation invalid: %#v", annotation)
		}
		annotations[annotation.AnnotationID] = true
	}
	for _, result := range fixture.RenderResults {
		if !requests[result.RequestID] || !outputs[result.OutputManifestRef] ||
			result.ArtifactManifestRef != fixture.ArtifactManifest.ManifestID ||
			len(result.SnapshotRefs) == 0 || len(result.WarningRefs) == 0 || len(result.AnnotationRefs) == 0 {
			t.Fatalf("render result invalid or unresolved: %#v", result)
		}
		for _, ref := range result.SnapshotRefs {
			if !snapshots[ref] {
				t.Fatalf("render result snapshot ref %q does not resolve", ref)
			}
		}
		for _, ref := range result.WarningRefs {
			if !warnings[ref] {
				t.Fatalf("render result warning ref %q does not resolve", ref)
			}
		}
		for _, ref := range result.AnnotationRefs {
			if !annotations[ref] {
				t.Fatalf("render result annotation ref %q does not resolve", ref)
			}
		}
	}
	for _, skip := range fixture.RendererSkips {
		if skip.DependencyImported || skip.CredentialRequired || skip.LiveNetwork || !skip.CleanSkip {
			t.Fatalf("renderer skip must clean-skip without live dependencies: %#v", skip)
		}
	}
	for _, boundary := range fixture.AdapterBoundaries {
		if boundary.DependencyImported || boundary.CredentialRequired || boundary.LiveNetwork || !boundary.CleanSkip {
			t.Fatalf("adapter boundary must clean-skip: %#v", boundary)
		}
	}
}

func TestGenericReportRenderContractsLivePackageIsDomainNeutral(t *testing.T) {
	base := genericReportRenderContractsPackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"finrobot", "acme", "aapl", "ticker", "equity", "investment", "valuation_engine", "target_price", "dcf", "sec.gov", "10-k", "finance.", "reportlab", "chromium", "playwright"} {
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

func TestGenericReportRenderContractsLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericReportRenderContractsPackageDir(t)
	schema := filepath.Join(base, "schemas", "generic_report_render_contracts_v1.schema.json")
	assertDocumentPipelineSchemaRequired(t, schema, []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "live_model_calls", "render_requests", "request_envelopes", "render_results", "output_manifests", "artifact_manifest", "snapshot_metadata", "warnings", "annotations", "fixture_hashes", "renderer_skips", "adapter_boundaries"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "render_requests", "items"}, []string{"request_id", "render_mode", "formats", "artifact_policy", "fixture_key", "live_network", "real_dependency_imports", "renderer_dependency_gate"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "render_results", "items"}, []string{"request_id", "status", "output_manifest_ref", "artifact_manifest_ref", "snapshot_refs", "warning_refs", "annotation_refs", "clean_skip"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "output_manifests", "items"}, []string{"output_id", "media_type", "artifact_refs", "uri_policy", "remote_fetch"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "snapshot_metadata", "items"}, []string{"snapshot_id", "page_ref", "hash_algorithm", "hash", "deterministic_inputs"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "report_render_request_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "render_requests"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "report_render_output_manifest_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "output_manifests", "artifact_manifest", "snapshot_metadata"})
}

func TestGenericReportRenderContractsLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericReportRenderContractsPackageDir(t), "main.leia")
	want := "generic_report_render_contracts_live_package capability=generic.ai.report.render.contracts entrypoint=ai.report.render_contracts requests=2 results=2 outputs=2 artifacts=3 snapshots=3 warnings=3 annotations=2 fixture_hashes=2 clean_skip=2 provider_free=true live_network=false imports=false model_calls=false"
	for _, result := range runFinRobotLivePackageSummarySmoke(t, path, "generic_report_render_contracts_live_package_summary", "generic_report_render_contracts_live_package", leia.LibString) {
		if result.Summary != want {
			t.Fatalf("summary = %#v, want %#v", result.Summary, want)
		}
		fields := result.Fields
		requireFinRobotSummaryFields(t, fields, "capability", "entrypoint", "requests", "results", "outputs", "artifacts", "snapshots", "warnings", "annotations", "fixture_hashes", "clean_skip", "provider_free", "live_network", "imports", "model_calls")
		if fields["capability"] != "generic.ai.report.render.contracts" ||
			fields["entrypoint"] != "ai.report.render_contracts" ||
			fields["requests"] != "2" ||
			fields["results"] != "2" ||
			fields["outputs"] != "2" ||
			fields["artifacts"] != "3" ||
			fields["snapshots"] != "3" ||
			fields["warnings"] != "3" ||
			fields["annotations"] != "2" ||
			fields["fixture_hashes"] != "2" ||
			fields["clean_skip"] != "2" ||
			fields["provider_free"] != "true" ||
			fields["live_network"] != "false" ||
			fields["imports"] != "false" ||
			fields["model_calls"] != "false" {
			t.Fatalf("summary fields = %#v", fields)
		}
	}
}

type genericReportRenderContractsFixture struct {
	ProviderFree          bool `json:"provider_free"`
	LiveNetwork           bool `json:"live_network"`
	RealDependencyImports bool `json:"real_dependency_imports"`
	LiveModelCalls        bool `json:"live_model_calls"`
	RenderRequests        []struct {
		RequestID              string   `json:"request_id"`
		RenderMode             string   `json:"render_mode"`
		Formats                []string `json:"formats"`
		FixtureKey             string   `json:"fixture_key"`
		LiveNetwork            bool     `json:"live_network"`
		RealDependencyImports  bool     `json:"real_dependency_imports"`
		RendererDependencyGate struct {
			DependencyImported bool `json:"dependency_imported"`
		} `json:"renderer_dependency_gate"`
	} `json:"render_requests"`
	RequestEnvelopes []any `json:"request_envelopes"`
	RenderResults    []struct {
		RequestID           string   `json:"request_id"`
		Status              string   `json:"status"`
		OutputManifestRef   string   `json:"output_manifest_ref"`
		ArtifactManifestRef string   `json:"artifact_manifest_ref"`
		SnapshotRefs        []string `json:"snapshot_refs"`
		WarningRefs         []string `json:"warning_refs"`
		AnnotationRefs      []string `json:"annotation_refs"`
		CleanSkip           bool     `json:"clean_skip"`
	} `json:"render_results"`
	OutputManifests []struct {
		OutputID     string   `json:"output_id"`
		MediaType    string   `json:"media_type"`
		ArtifactRefs []string `json:"artifact_refs"`
		URIPolicy    string   `json:"uri_policy"`
		RemoteFetch  bool     `json:"remote_fetch"`
	} `json:"output_manifests"`
	ArtifactManifest struct {
		ManifestID string `json:"manifest_id"`
		Artifacts  []any  `json:"artifacts"`
	} `json:"artifact_manifest"`
	SnapshotMetadata []struct {
		SnapshotID          string   `json:"snapshot_id"`
		PageRef             string   `json:"page_ref"`
		HashAlgorithm       string   `json:"hash_algorithm"`
		Hash                string   `json:"hash"`
		DeterministicInputs []string `json:"deterministic_inputs"`
	} `json:"snapshot_metadata"`
	Warnings []struct {
		WarningID string `json:"warning_id"`
		Kind      string `json:"kind"`
		Severity  string `json:"severity"`
		Message   string `json:"message"`
	} `json:"warnings"`
	Annotations []struct {
		AnnotationID string `json:"annotation_id"`
		Kind         string `json:"kind"`
		Required     bool   `json:"required"`
		Resolved     bool   `json:"resolved"`
	} `json:"annotations"`
	FixtureHashes []any `json:"fixture_hashes"`
	RendererSkips []struct {
		DependencyImported bool `json:"dependency_imported"`
		CredentialRequired bool `json:"credential_required"`
		LiveNetwork        bool `json:"live_network"`
		CleanSkip          bool `json:"clean_skip"`
	} `json:"renderer_skips"`
	AdapterBoundaries []struct {
		ID                 string `json:"id"`
		Capability         string `json:"capability"`
		DependencyImported bool   `json:"dependency_imported"`
		CredentialRequired bool   `json:"credential_required"`
		LiveNetwork        bool   `json:"live_network"`
		CleanSkip          bool   `json:"clean_skip"`
	} `json:"adapter_boundaries"`
}

func loadGenericReportRenderContractsFixture(t *testing.T, path string) genericReportRenderContractsFixture {
	t.Helper()
	var fixture genericReportRenderContractsFixture
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func genericReportRenderContractsPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_report_render_contracts")
}
