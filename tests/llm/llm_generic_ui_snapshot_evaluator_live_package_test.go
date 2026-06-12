package leia_test

import (
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
	for _, want := range []string{"generic.ai.ui.snapshot.evaluator", "generic.ai.ui.snapshot.evidence_report_projection", "generic.ai.ui.route_dom_schema", "generic.ai.ui.viewport_matrix", "generic.ai.ui.visual_diff_budget", "generic.ai.ui.accessibility_summary", "generic.ai.ui.artifact_uri_manifest", "generic.ai.ui.redaction_policy", "generic.ai.ui.static_asset_policy", "generic.ai.ui.browser_clean_skip"} {
		if !genericLivePackageContains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}
	for _, key := range []string{"ui_snapshot_evaluator", "evidence_report_ui_snapshot_projection"} {
		if manifest.Schemas[key] == "" || manifest.Fixtures[key] == "" {
			t.Fatalf("manifest missing schema/fixture key %q: schemas=%#v fixtures=%#v", key, manifest.Schemas, manifest.Fixtures)
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
		Capabilities          []string          `json:"capabilities"`
		FieldContracts        map[string]string `json:"field_contracts"`
		EvidenceReport        struct {
			SourcePackage        string   `json:"source_package"`
			SourceFixture        string   `json:"source_fixture"`
			TargetFixture        string   `json:"target_fixture"`
			TargetCapability     string   `json:"target_capability"`
			RequiredSourceFields []string `json:"required_source_fields"`
			RequiredTargetFields []string `json:"required_target_fields"`
			RawRenderPayloads    bool     `json:"raw_render_payloads_allowed"`
			RemoteFetchAllowed   bool     `json:"remote_fetch_allowed"`
			BrowserRequired      bool     `json:"browser_required"`
			ProviderFree         bool     `json:"provider_free"`
		} `json:"evidence_report_projection_contract"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Contracts["contract"]), &contract)
	if contract.SchemaVersion != 1 || contract.PackageBoundaryID != manifest.PackageBoundaryID ||
		contract.PackageName != "generic.ai.ui.snapshot.evaluator" || contract.Entrypoint != "ai.ui.snapshot_evaluator" ||
		!contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork ||
		contract.LiveModelCalls || contract.RealDependencyImports || contract.RequiresBrowser {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	if !genericLivePackageContains(contract.Capabilities, "generic.ai.ui.snapshot.evidence_report_projection") {
		t.Fatalf("contract capabilities missing evidence projection: %#v", contract.Capabilities)
	}
	if contract.EvidenceReport.SourcePackage != "generic_evidence_report_artifacts" ||
		contract.EvidenceReport.SourceFixture == "" || contract.EvidenceReport.TargetFixture == "" ||
		contract.EvidenceReport.TargetCapability != "generic.ai.ui.snapshot.evidence_report_projection" ||
		!contract.EvidenceReport.ProviderFree || contract.EvidenceReport.RawRenderPayloads ||
		contract.EvidenceReport.RemoteFetchAllowed || contract.EvidenceReport.BrowserRequired {
		t.Fatalf("evidence projection contract drifted: %#v", contract.EvidenceReport)
	}
	for _, want := range []string{"artifact_manifest.artifacts.artifact_id", "artifact_manifest.artifacts.uri", "snapshot_metadata.snapshot_id", "snapshot_metadata.content_hash", "warnings.id", "accessibility_checklist.checks.id"} {
		if !genericLivePackageContains(contract.EvidenceReport.RequiredSourceFields, want) {
			t.Fatalf("evidence projection source fields missing %q: %#v", want, contract.EvidenceReport.RequiredSourceFields)
		}
	}
	for _, want := range []string{"route_id", "snapshot_ref", "artifact_ref", "viewport_id", "accessibility_check"} {
		if !genericLivePackageContains(contract.EvidenceReport.RequiredTargetFields, want) {
			t.Fatalf("evidence projection target fields missing %q: %#v", want, contract.EvidenceReport.RequiredTargetFields)
		}
	}
	for _, want := range []string{"route_dom_snapshots", "evidence_report_projection", "viewport_matrix", "visual_diff_budgets", "accessibility_summaries", "artifact_uri_manifest", "redaction_policy", "static_asset_policy", "browser_clean_skip"} {
		if contract.FieldContracts[want] == "" {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
		}
	}

	var fixtureIndex struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Fixtures              []struct {
			FixtureKey            string         `json:"fixture_key"`
			Capability            string         `json:"capability"`
			Path                  string         `json:"path"`
			Schema                string         `json:"schema"`
			ProviderFree          bool           `json:"provider_free"`
			LiveNetwork           bool           `json:"live_network"`
			RealDependencyImports bool           `json:"real_dependency_imports"`
			Metadata              map[string]any `json:"metadata"`
		} `json:"fixtures"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Fixtures["index"]), &fixtureIndex)
	if !fixtureIndex.ProviderFree || fixtureIndex.LiveNetwork || fixtureIndex.RealDependencyImports || len(fixtureIndex.Fixtures) != 2 {
		t.Fatalf("fixture index drifted: %#v", fixtureIndex)
	}
	foundProjection := false
	for _, fixture := range fixtureIndex.Fixtures {
		if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports {
			t.Fatalf("fixture index entry must be provider-free/offline: %#v", fixture)
		}
		if fixture.FixtureKey == "generic:ui_snapshot_evaluator:evidence_report_projection" {
			foundProjection = true
			if fixture.Capability != "generic.ai.ui.snapshot.evidence_report_projection" ||
				fixture.Path != "fixtures/evidence_report_ui_snapshot_projection_fixture.json" ||
				fixture.Schema != "schemas/evidence_report_ui_snapshot_projection_v1.schema.json" ||
				fixture.Metadata["source_package"] != "generic_evidence_report_artifacts" ||
				fixture.Metadata["remote_fetch"] != false || fixture.Metadata["browser_required"] != false ||
				fixture.Metadata["snapshot_mappings"] != float64(2) ||
				fixture.Metadata["artifact_uri_mappings"] != float64(3) ||
				fixture.Metadata["warning_mappings"] != float64(2) ||
				fixture.Metadata["accessibility_mappings"] != float64(3) {
				t.Fatalf("projection fixture index entry drifted: %#v", fixture)
			}
		}
	}
	if !foundProjection {
		t.Fatalf("fixture index missing evidence report projection: %#v", fixtureIndex.Fixtures)
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

	projectionSchema := filepath.Join(base, "schemas", "evidence_report_ui_snapshot_projection_v1.schema.json")
	assertDocumentPipelineSchemaRequired(t, projectionSchema, []string{"schema_version", "id", "projection_kind", "package_boundary_id", "provider_free", "domain_specific", "live_network", "live_model_calls", "real_dependency_imports", "source_fixture_refs", "snapshot_mappings", "artifact_uri_mappings", "warning_mappings", "accessibility_mappings", "viewport_budget_mappings", "projection_assertions"})
	assertDocumentPipelineNestedSchemaRequired(t, projectionSchema, []string{"properties", "snapshot_mappings", "items"}, []string{"source_snapshot_id", "source_content_hash", "source_dimensions", "target_route_id", "target_snapshot_ref", "target_viewport_id", "mapping_policy"})
	assertDocumentPipelineNestedSchemaRequired(t, projectionSchema, []string{"properties", "artifact_uri_mappings", "items"}, []string{"source_artifact_id", "source_uri", "source_content_hash", "target_artifact_id", "target_uri", "remote_fetch"})
	assertDocumentPipelineNestedSchemaRequired(t, projectionSchema, []string{"properties", "warning_mappings", "items"}, []string{"source_warning_id", "source_severity", "target_route_id", "target_review_marker", "manual_review_required"})
	assertDocumentPipelineNestedSchemaRequired(t, projectionSchema, []string{"properties", "accessibility_mappings", "items"}, []string{"source_check_id", "source_status", "target_route_id", "target_required_check"})
	assertDocumentPipelineNestedSchemaRequired(t, projectionSchema, []string{"properties", "viewport_budget_mappings", "items"}, []string{"source_snapshot_id", "target_route_id", "target_viewport_id", "target_budget_ratio", "text_overflow_allowed", "missing_region_allowed"})
}

func TestGenericUISnapshotEvaluatorEvidenceReportProjection(t *testing.T) {
	base := genericUISnapshotEvaluatorPackageDir(t)
	sourceBase := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_evidence_report_artifacts")
	source := loadGenericUIEvidenceReportArtifactsFixture(t, filepath.Join(sourceBase, "fixtures", "generic_evidence_report_artifacts_fixture.json"))
	target := loadGenericUISnapshotEvaluatorFixture(t, filepath.Join(base, "fixtures", "generic_ui_snapshot_evaluator_fixture.json"))
	projection := loadGenericUIEvidenceReportProjectionFixture(t, filepath.Join(base, "fixtures", "evidence_report_ui_snapshot_projection_fixture.json"))

	if projection.SchemaVersion != 1 ||
		projection.ID != "generic-ui-evidence-report-projection-fixture" ||
		projection.ProjectionKind != "evidence_report_artifacts_to_ui_snapshot_evaluator_projection" ||
		projection.PackageBoundaryID != "generic-ai-ui-snapshot-evaluator" ||
		!projection.ProviderFree || projection.DomainSpecific || projection.LiveNetwork ||
		projection.LiveModelCalls || projection.RealDependencyImports ||
		projection.SourceFixtureRefs["evidence_report_artifacts"] == "" ||
		projection.SourceFixtureRefs["ui_snapshot_evaluator"] == "" {
		t.Fatalf("projection header drifted: %#v", projection)
	}

	sourceSnapshots := map[string]struct {
		ContentHash string
		Width       int
		Height      int
	}{}
	for _, snapshot := range source.SnapshotMetadata {
		sourceSnapshots[snapshot.SnapshotID] = struct {
			ContentHash string
			Width       int
			Height      int
		}{ContentHash: snapshot.ContentHash, Width: snapshot.Dimensions.Width, Height: snapshot.Dimensions.Height}
	}
	sourceArtifacts := map[string]struct {
		URI  string
		Hash string
	}{}
	for _, artifact := range source.ArtifactManifest.Artifacts {
		sourceArtifacts[artifact.ArtifactID] = struct {
			URI  string
			Hash string
		}{URI: artifact.URI, Hash: artifact.ContentHash}
	}
	sourceWarnings := map[string]string{}
	for _, warning := range source.Warnings {
		sourceWarnings[warning.ID] = warning.Severity
	}
	sourceChecks := map[string]string{}
	for _, check := range source.AccessibilityChecklist.Checks {
		sourceChecks[check.ID] = check.Status
	}

	targetRoutes := map[string]genericUISnapshotRoute{}
	targetSnapshotRefs := map[string]bool{}
	targetArtifactRefs := map[string]bool{}
	for _, route := range target.RouteDOMSnapshots {
		targetRoutes[route.RouteID] = route
		for _, ref := range route.SnapshotRefs {
			targetSnapshotRefs[ref] = true
		}
		for _, ref := range route.ArtifactRefs {
			targetArtifactRefs[ref] = true
		}
	}
	targetViewports := map[string]bool{}
	for _, viewport := range target.ViewportMatrix {
		targetViewports[viewport.ID] = true
	}
	targetBudgets := map[string]genericUISnapshotBudget{}
	for _, budget := range target.VisualDiffBudgets {
		targetBudgets[budget.RouteID+"|"+budget.ViewportID] = budget
	}
	targetArtifacts := map[string]string{}
	for _, artifact := range target.ArtifactURIManifest.Artifacts {
		targetArtifacts[artifact.ID] = artifact.URI
	}
	targetChecks := map[string]map[string]bool{}
	for _, summary := range target.AccessibilitySummaries {
		if targetChecks[summary.RouteID] == nil {
			targetChecks[summary.RouteID] = map[string]bool{}
		}
		for _, check := range summary.RequiredChecks {
			targetChecks[summary.RouteID][check] = true
		}
	}

	for _, mapping := range projection.SnapshotMappings {
		sourceSnapshot, ok := sourceSnapshots[mapping.SourceSnapshotID]
		if !ok || sourceSnapshot.ContentHash != mapping.SourceContentHash ||
			sourceSnapshot.Width != mapping.SourceDimensions.Width ||
			sourceSnapshot.Height != mapping.SourceDimensions.Height ||
			targetRoutes[mapping.TargetRouteID].RouteID == "" ||
			!targetSnapshotRefs[mapping.TargetSnapshotRef] ||
			!targetViewports[mapping.TargetViewportID] ||
			mapping.MappingPolicy != "explicit_snapshot_ref" {
			t.Fatalf("snapshot mapping unresolved: %#v source=%#v", mapping, sourceSnapshot)
		}
	}
	for _, mapping := range projection.ArtifactURIMappings {
		sourceArtifact, ok := sourceArtifacts[mapping.SourceArtifactID]
		if !ok || sourceArtifact.URI != mapping.SourceURI || sourceArtifact.Hash != mapping.SourceContentHash ||
			!targetArtifactRefs[mapping.TargetArtifactID] ||
			targetArtifacts[mapping.TargetArtifactID] != mapping.TargetURI ||
			mapping.RemoteFetch ||
			!genericUIFixtureOrArtifactURI(mapping.SourceURI) ||
			!genericUIFixtureOrArtifactURI(mapping.TargetURI) {
			t.Fatalf("artifact URI mapping unresolved: %#v source=%#v targetURI=%q", mapping, sourceArtifact, targetArtifacts[mapping.TargetArtifactID])
		}
	}
	for _, mapping := range projection.WarningMappings {
		if sourceWarnings[mapping.SourceWarningID] != mapping.SourceSeverity ||
			targetRoutes[mapping.TargetRouteID].RouteID == "" ||
			mapping.TargetReviewMarker == "" ||
			mapping.ManualReviewRequired {
			t.Fatalf("warning mapping unresolved: %#v", mapping)
		}
	}
	for _, mapping := range projection.AccessibilityMappings {
		if sourceChecks[mapping.SourceCheckID] != mapping.SourceStatus ||
			!targetChecks[mapping.TargetRouteID][mapping.TargetRequiredCheck] {
			t.Fatalf("accessibility mapping unresolved: %#v", mapping)
		}
	}
	for _, mapping := range projection.ViewportBudgetMappings {
		budget := targetBudgets[mapping.TargetRouteID+"|"+mapping.TargetViewportID]
		if sourceSnapshots[mapping.SourceSnapshotID].ContentHash == "" ||
			!targetViewports[mapping.TargetViewportID] ||
			budget.RouteID == "" ||
			budget.MaxChangedPixelsRatio != mapping.TargetBudgetRatio ||
			mapping.TextOverflowAllowed || mapping.MissingRegionAllowed ||
			budget.TextOverflowAllowed || budget.MissingRegionAllowed {
			t.Fatalf("viewport budget mapping unresolved: %#v targetBudget=%#v", mapping, budget)
		}
	}
	for _, want := range []string{"projection_is_provider_free", "source_and_target_ids_are_explicitly_mapped", "artifact_uris_use_fixture_or_artifact_schemes", "remote_fetch_disabled", "browser_required_false", "snapshot_refs_resolve_in_target", "artifact_refs_resolve_in_target", "accessibility_checks_resolve_in_target"} {
		if !projection.ProjectionAssertions[want] {
			t.Fatalf("projection assertion %q missing/false: %#v", want, projection.ProjectionAssertions)
		}
	}
}

func TestGenericUISnapshotEvaluatorLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericUISnapshotEvaluatorPackageDir(t), "main.leia")
	want := "generic_ui_snapshot_evaluator_live_package capability=generic.ai.ui.snapshot.evaluator entrypoint=ai.ui.snapshot_evaluator routes=2 viewports=3 budgets=3 accessibility=2 artifacts=3 evidence_report_projections=1 clean_skip=2 provider_free=true live_network=false imports=false model_calls=false browser=false"
	for _, result := range runFinRobotLivePackageSummarySmoke(t, path, "generic_ui_snapshot_evaluator_live_package_summary", "generic_ui_snapshot_evaluator_live_package", leia.LibString) {
		if result.Summary != want {
			t.Fatalf("summary = %#v, want %#v", result.Summary, want)
		}
	}
}

type genericUISnapshotEvaluatorFixture struct {
	ProviderFree          bool                     `json:"provider_free"`
	LiveNetwork           bool                     `json:"live_network"`
	RealDependencyImports bool                     `json:"real_dependency_imports"`
	LiveModelCalls        bool                     `json:"live_model_calls"`
	RequiresBrowser       bool                     `json:"requires_browser"`
	RequiresCredentials   bool                     `json:"requires_credentials"`
	RouteDOMSnapshots     []genericUISnapshotRoute `json:"route_dom_snapshots"`
	ViewportMatrix        []struct {
		ID                string  `json:"id"`
		Width             int     `json:"width"`
		Height            int     `json:"height"`
		DeviceScaleFactor float64 `json:"device_scale_factor"`
	} `json:"viewport_matrix"`
	VisualDiffBudgets      []genericUISnapshotBudget `json:"visual_diff_budgets"`
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
			ID  string `json:"id"`
			URI string `json:"uri"`
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

type genericUISnapshotRoute struct {
	RouteID    string `json:"route_id"`
	Route      string `json:"route"`
	Method     string `json:"method"`
	FixtureURL string `json:"fixture_url"`
	DOMSchema  struct {
		RootSelector string `json:"root_selector"`
	} `json:"dom_schema"`
	SnapshotRefs []string `json:"snapshot_refs"`
	ArtifactRefs []string `json:"artifact_refs"`
}

type genericUISnapshotBudget struct {
	RouteID               string  `json:"route_id"`
	ViewportID            string  `json:"viewport_id"`
	MaxChangedPixelsRatio float64 `json:"max_changed_pixels_ratio"`
	MaxLayoutShiftPx      int     `json:"max_layout_shift_px"`
	TextOverflowAllowed   bool    `json:"text_overflow_allowed"`
	MissingRegionAllowed  bool    `json:"missing_region_allowed"`
}

type genericUIEvidenceReportArtifactsFixture struct {
	ArtifactManifest struct {
		Artifacts []struct {
			ArtifactID  string `json:"artifact_id"`
			URI         string `json:"uri"`
			ContentHash string `json:"content_hash"`
		} `json:"artifacts"`
	} `json:"artifact_manifest"`
	SnapshotMetadata []struct {
		SnapshotID  string `json:"snapshot_id"`
		ContentHash string `json:"content_hash"`
		Dimensions  struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"dimensions"`
	} `json:"snapshot_metadata"`
	Warnings []struct {
		ID       string `json:"id"`
		Severity string `json:"severity"`
	} `json:"warnings"`
	AccessibilityChecklist struct {
		Checks []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"checks"`
	} `json:"accessibility_checklist"`
}

type genericUIEvidenceReportProjectionFixture struct {
	SchemaVersion         int               `json:"schema_version"`
	ID                    string            `json:"id"`
	ProjectionKind        string            `json:"projection_kind"`
	PackageBoundaryID     string            `json:"package_boundary_id"`
	ProviderFree          bool              `json:"provider_free"`
	DomainSpecific        bool              `json:"domain_specific"`
	LiveNetwork           bool              `json:"live_network"`
	LiveModelCalls        bool              `json:"live_model_calls"`
	RealDependencyImports bool              `json:"real_dependency_imports"`
	SourceFixtureRefs     map[string]string `json:"source_fixture_refs"`
	SnapshotMappings      []struct {
		SourceSnapshotID  string `json:"source_snapshot_id"`
		SourceContentHash string `json:"source_content_hash"`
		SourceDimensions  struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"source_dimensions"`
		TargetRouteID     string `json:"target_route_id"`
		TargetSnapshotRef string `json:"target_snapshot_ref"`
		TargetViewportID  string `json:"target_viewport_id"`
		MappingPolicy     string `json:"mapping_policy"`
	} `json:"snapshot_mappings"`
	ArtifactURIMappings []struct {
		SourceArtifactID  string `json:"source_artifact_id"`
		SourceURI         string `json:"source_uri"`
		SourceContentHash string `json:"source_content_hash"`
		TargetArtifactID  string `json:"target_artifact_id"`
		TargetURI         string `json:"target_uri"`
		RemoteFetch       bool   `json:"remote_fetch"`
	} `json:"artifact_uri_mappings"`
	WarningMappings []struct {
		SourceWarningID      string `json:"source_warning_id"`
		SourceSeverity       string `json:"source_severity"`
		TargetRouteID        string `json:"target_route_id"`
		TargetReviewMarker   string `json:"target_review_marker"`
		ManualReviewRequired bool   `json:"manual_review_required"`
	} `json:"warning_mappings"`
	AccessibilityMappings []struct {
		SourceCheckID       string `json:"source_check_id"`
		SourceStatus        string `json:"source_status"`
		TargetRouteID       string `json:"target_route_id"`
		TargetRequiredCheck string `json:"target_required_check"`
	} `json:"accessibility_mappings"`
	ViewportBudgetMappings []struct {
		SourceSnapshotID     string  `json:"source_snapshot_id"`
		TargetRouteID        string  `json:"target_route_id"`
		TargetViewportID     string  `json:"target_viewport_id"`
		TargetBudgetRatio    float64 `json:"target_budget_ratio"`
		TextOverflowAllowed  bool    `json:"text_overflow_allowed"`
		MissingRegionAllowed bool    `json:"missing_region_allowed"`
	} `json:"viewport_budget_mappings"`
	ProjectionAssertions map[string]bool `json:"projection_assertions"`
}

func loadGenericUISnapshotEvaluatorFixture(t *testing.T, path string) genericUISnapshotEvaluatorFixture {
	t.Helper()
	var fixture genericUISnapshotEvaluatorFixture
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func loadGenericUIEvidenceReportArtifactsFixture(t *testing.T, path string) genericUIEvidenceReportArtifactsFixture {
	t.Helper()
	var fixture genericUIEvidenceReportArtifactsFixture
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func loadGenericUIEvidenceReportProjectionFixture(t *testing.T, path string) genericUIEvidenceReportProjectionFixture {
	t.Helper()
	var fixture genericUIEvidenceReportProjectionFixture
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func genericUIFixtureOrArtifactURI(uri string) bool {
	return strings.HasPrefix(uri, "fixture://") || strings.HasPrefix(uri, "artifact://")
}

func genericUISnapshotEvaluatorPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_ui_snapshot_evaluator")
}
