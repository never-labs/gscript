package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericProductAppBoundaryLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericProductAppBoundaryPackageDir(t)
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
	if manifest.SchemaVersion != 1 || manifest.ID != "generic-product-app-boundary" ||
		manifest.PackageName != "leia-generic-ai-product-app-boundary" ||
		manifest.PackageBoundaryID != "generic-ai-product-app-boundary" ||
		manifest.CapabilityID != "generic.ai.product_app.boundary" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetworkDefault ||
		manifest.LiveModelDefault || manifest.DependsOnQRuntime || manifest.CredentialRequired {
		t.Fatalf("manifest must stay provider-free/generic/offline/credential-free: %#v", manifest)
	}
	for _, want := range []string{"generic.ai.product_app.boundary", "generic.ai.product_app.route_contract", "generic.ai.product_app.session_contract", "generic.ai.product_app.workflow_projection", "generic.ai.product_app.task_log", "generic.ai.product_app.artifact_download", "generic.ai.product_app.crud_fixture_state", "generic.ai.product_app.db_migration_plan", "generic.ai.product_app.deployment_target", "generic.ai.product_app.approval_boundary", "generic.ai.product_app.clean_skip"} {
		if !genericLivePackageContains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}

	var contract struct {
		SchemaVersion              int               `json:"schema_version"`
		PackageBoundaryID          string            `json:"package_boundary_id"`
		PackageName                string            `json:"package_name"`
		Entrypoint                 string            `json:"entrypoint"`
		ProviderFree               bool              `json:"provider_free"`
		DomainSpecific             bool              `json:"domain_specific"`
		LiveNetwork                bool              `json:"live_network"`
		LiveModelCalls             bool              `json:"live_model_calls"`
		RealDependencyImports      bool              `json:"real_dependency_imports"`
		RequiresCredentials        bool              `json:"requires_credentials"`
		FieldContracts             map[string]string `json:"field_contracts"`
		WorkflowProjectionContract struct {
			SourcePackage              string   `json:"source_package"`
			SourceFixtures             []string `json:"source_fixtures"`
			TargetFixture              string   `json:"target_fixture"`
			TargetCapability           string   `json:"target_capability"`
			RequiredSourceFields       []string `json:"required_source_fields"`
			RequiredTargetFields       []string `json:"required_target_fields"`
			RawStagePayloadsAllowed    bool     `json:"raw_stage_payloads_allowed"`
			LiveSideEffectsAllowed     bool     `json:"live_side_effects_allowed"`
			DeterministicOrderRequired bool     `json:"deterministic_order_required"`
			ProviderFree               bool     `json:"provider_free"`
		} `json:"workflow_projection_contract"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Contracts["contract"]), &contract)
	if contract.SchemaVersion != 1 || contract.PackageBoundaryID != manifest.PackageBoundaryID ||
		contract.PackageName != "generic.ai.product_app.boundary" || contract.Entrypoint != "ai.product_app.boundary" ||
		!contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork ||
		contract.LiveModelCalls || contract.RealDependencyImports || contract.RequiresCredentials {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	for _, want := range []string{"route_contracts", "session_contracts", "workflow_projection", "task_logs", "artifact_downloads", "crud_fixture_state", "db_migration_plan", "deployment_targets", "approval_boundaries", "clean_skips"} {
		if contract.FieldContracts[want] == "" {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
		}
	}
	if contract.WorkflowProjectionContract.SourcePackage != "generic_workflow_orchestrator" ||
		contract.WorkflowProjectionContract.TargetFixture != "fixtures/product_app_boundary_fixture.json" ||
		contract.WorkflowProjectionContract.TargetCapability != "generic.ai.product_app.workflow_projection" ||
		contract.WorkflowProjectionContract.RawStagePayloadsAllowed ||
		contract.WorkflowProjectionContract.LiveSideEffectsAllowed ||
		!contract.WorkflowProjectionContract.DeterministicOrderRequired ||
		!contract.WorkflowProjectionContract.ProviderFree {
		t.Fatalf("workflow projection contract drifted: %#v", contract.WorkflowProjectionContract)
	}
	for _, want := range []string{"workflow_id", "run_id", "stage_results.stage_id", "stage_results.status", "artifacts.artifact_id", "trace_refs"} {
		if !genericLivePackageContains(contract.WorkflowProjectionContract.RequiredSourceFields, want) {
			t.Fatalf("workflow projection source field missing %q: %#v", want, contract.WorkflowProjectionContract.RequiredSourceFields)
		}
	}
	for _, want := range []string{"route_id", "task_id", "artifact_id", "correlation_id", "fixture_uri"} {
		if !genericLivePackageContains(contract.WorkflowProjectionContract.RequiredTargetFields, want) {
			t.Fatalf("workflow projection target field missing %q: %#v", want, contract.WorkflowProjectionContract.RequiredTargetFields)
		}
	}
}

func TestGenericProductAppBoundaryLivePackageFixtureShape(t *testing.T) {
	base := genericProductAppBoundaryPackageDir(t)
	fixture := loadGenericProductAppBoundaryFixture(t, filepath.Join(base, "fixtures", "product_app_boundary_fixture.json"))
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.LiveModelCalls || fixture.RequiresCredentials {
		t.Fatalf("fixture must stay provider-free and offline: %#v", fixture)
	}
	if len(fixture.RouteContracts) != 3 || len(fixture.SessionContracts) != 2 ||
		len(fixture.TaskLogs) != 2 || len(fixture.ArtifactDownloads) != 2 ||
		len(fixture.CRUDFixtureState) != 3 || len(fixture.DBMigrationPlans) != 1 ||
		len(fixture.DeploymentTargets) != 2 || len(fixture.ApprovalBoundaries) != 2 ||
		len(fixture.AdapterBoundaries) != 3 {
		t.Fatalf("fixture counts drifted: routes=%d sessions=%d tasks=%d downloads=%d crud=%d migrations=%d deployments=%d approvals=%d adapters=%d",
			len(fixture.RouteContracts), len(fixture.SessionContracts), len(fixture.TaskLogs), len(fixture.ArtifactDownloads), len(fixture.CRUDFixtureState), len(fixture.DBMigrationPlans), len(fixture.DeploymentTargets), len(fixture.ApprovalBoundaries), len(fixture.AdapterBoundaries))
	}
	for _, route := range fixture.RouteContracts {
		if route.RouteID == "" || route.Method == "" || route.Path == "" || route.HandlerRef == "" || route.Capability == "" {
			t.Fatalf("route contract incomplete: %#v", route)
		}
	}
	for _, session := range fixture.SessionContracts {
		if session.SessionID == "" || session.ActorRef == "" || len(session.Roles) == 0 ||
			session.CredentialRequired || session.CredentialPresent || !strings.HasPrefix(session.FixtureURI, "fixture://") {
			t.Fatalf("session contract invalid: %#v", session)
		}
	}
	for _, task := range fixture.TaskLogs {
		if task.TaskID == "" || task.Status == "" || task.CorrelationID == "" || !task.ReplayReady || !strings.HasPrefix(task.FixtureURI, "fixture://") {
			t.Fatalf("task log invalid: %#v", task)
		}
	}
	for _, download := range fixture.ArtifactDownloads {
		if download.ArtifactID == "" || !strings.HasPrefix(download.URI, "artifact://") || download.Hash == "" || download.RemoteFetch {
			t.Fatalf("artifact download invalid: %#v", download)
		}
	}
	for _, op := range fixture.CRUDFixtureState {
		if op.OperationID == "" || op.Op == "" || op.StateKey == "" || !op.Deterministic || !strings.HasPrefix(op.FixtureURI, "fixture://") {
			t.Fatalf("CRUD fixture state invalid: %#v", op)
		}
	}
	for _, migration := range fixture.DBMigrationPlans {
		if migration.MigrationID == "" || migration.LiveDatabase || !migration.CleanSkip || len(migration.OrderedSteps) == 0 {
			t.Fatalf("migration plan invalid: %#v", migration)
		}
	}
	for _, target := range fixture.DeploymentTargets {
		if target.TargetID == "" || target.LiveNetwork || target.RealDependencyImports || !target.CleanSkip {
			t.Fatalf("deployment target invalid: %#v", target)
		}
	}
	for _, approval := range fixture.ApprovalBoundaries {
		if approval.BoundaryID == "" || approval.Decision != "skipped" || approval.SideEffectAllowed || !approval.CleanSkip {
			t.Fatalf("approval boundary invalid: %#v", approval)
		}
	}
	for _, boundary := range fixture.AdapterBoundaries {
		if boundary.DependencyImported || boundary.CredentialRequired || boundary.LiveNetwork || !boundary.CleanSkip {
			t.Fatalf("adapter boundary must clean-skip: %#v", boundary)
		}
	}
}

func TestGenericProductAppBoundaryLivePackageIsDomainNeutral(t *testing.T) {
	base := genericProductAppBoundaryPackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"finrobot", "acme", "aapl", "ticker", "equity", "investment", "valuation_engine", "target_price", "dcf", "sec.gov", "10-k", "finance.", "fastapi", "flask", "django", "postgres", "s3"} {
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

func TestGenericProductAppBoundaryLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericProductAppBoundaryPackageDir(t)
	schema := filepath.Join(base, "schemas", "generic_product_app_boundary_v1.schema.json")
	assertDocumentPipelineSchemaRequired(t, schema, []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "live_model_calls", "requires_credentials", "route_contracts", "session_contracts", "task_logs", "artifact_downloads", "crud_fixture_state", "db_migration_plans", "deployment_targets", "approval_boundaries", "adapter_boundaries"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "route_contracts", "items"}, []string{"route_id", "method", "path", "handler_ref", "auth_required", "capability"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "adapter_boundaries", "items"}, []string{"id", "capability", "dependency_imported", "credential_required", "live_network", "clean_skip"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "product_app_route_contract_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "route_contracts"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "product_app_lifecycle_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "session_contracts", "task_logs", "crud_fixture_state", "db_migration_plans", "deployment_targets", "approval_boundaries"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "workflow_product_app_projection_v1.schema.json"), []string{"schema_version", "fixture_key", "projection_kind", "provider_free", "live_network", "real_dependency_imports", "source_fixture_refs", "target_fixture_ref", "workflow_identity", "route_mappings", "task_mappings", "artifact_mappings", "projection_assertions"})
}

func TestGenericProductAppBoundaryWorkflowProjection(t *testing.T) {
	base := genericProductAppBoundaryPackageDir(t)
	root := repoRoot(t)
	product := loadGenericProductAppBoundaryFixture(t, filepath.Join(base, "fixtures", "product_app_boundary_fixture.json"))
	var workflowResult struct {
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		WorkflowID            string `json:"workflow_id"`
		RunID                 string `json:"run_id"`
		Entrypoint            string `json:"entrypoint"`
		Status                string `json:"status"`
		StageResults          []struct {
			StageID      string `json:"stage_id"`
			Status       string `json:"status"`
			OutputRef    string `json:"output_ref"`
			TraceEventID string `json:"trace_event_id"`
		} `json:"stage_results"`
		Artifacts []struct {
			ArtifactID string   `json:"artifact_id"`
			Kind       string   `json:"kind"`
			Status     string   `json:"status"`
			SourceRefs []string `json:"source_refs"`
		} `json:"artifacts"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "generic_workflow_orchestrator", "fixtures", "workflow_result_fixture.json"), &workflowResult)

	var projection struct {
		SchemaVersion         int    `json:"schema_version"`
		FixtureKey            string `json:"fixture_key"`
		ProjectionKind        string `json:"projection_kind"`
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		SourceFixtureRefs     struct {
			WorkflowGraph      string `json:"workflow_graph"`
			WorkflowResult     string `json:"workflow_result"`
			ProductAppBoundary string `json:"product_app_boundary"`
		} `json:"source_fixture_refs"`
		TargetFixtureRef string `json:"target_fixture_ref"`
		WorkflowIdentity struct {
			WorkflowID string `json:"workflow_id"`
			RunID      string `json:"run_id"`
			Entrypoint string `json:"entrypoint"`
			Status     string `json:"status"`
		} `json:"workflow_identity"`
		RouteMappings []struct {
			StageID          string `json:"stage_id"`
			RouteID          string `json:"route_id"`
			HandlerRef       string `json:"handler_ref"`
			ProjectionPolicy string `json:"projection_policy"`
		} `json:"route_mappings"`
		TaskMappings []struct {
			StageID        string `json:"stage_id"`
			StageStatus    string `json:"stage_status"`
			StageOutputRef string `json:"stage_output_ref"`
			TraceEventID   string `json:"trace_event_id"`
			TaskID         string `json:"task_id"`
			TaskStatus     string `json:"task_status"`
			CorrelationID  string `json:"correlation_id"`
			FixtureURI     string `json:"fixture_uri"`
		} `json:"task_mappings"`
		ArtifactMappings []struct {
			WorkflowArtifactID     string   `json:"workflow_artifact_id"`
			WorkflowArtifactKind   string   `json:"workflow_artifact_kind"`
			WorkflowArtifactStatus string   `json:"workflow_artifact_status"`
			SourceRefs             []string `json:"source_refs"`
			ProductArtifactID      string   `json:"product_artifact_id"`
			DownloadURI            string   `json:"download_uri"`
			RemoteFetch            bool     `json:"remote_fetch"`
		} `json:"artifact_mappings"`
		ProjectionAssertions map[string]bool `json:"projection_assertions"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "workflow_product_app_projection_fixture.json"), &projection)
	if projection.SchemaVersion != 1 ||
		projection.FixtureKey != "generic:product_app_boundary:workflow_projection" ||
		projection.ProjectionKind != "workflow_result_to_product_app_boundary_projection" ||
		projection.SourceFixtureRefs.WorkflowGraph == "" ||
		projection.SourceFixtureRefs.WorkflowResult == "" ||
		projection.TargetFixtureRef != "fixtures/product_app_boundary_fixture.json" {
		t.Fatalf("projection header/source refs invalid: %#v", projection)
	}
	if !projection.ProviderFree || projection.LiveNetwork || projection.RealDependencyImports {
		t.Fatalf("projection must stay provider-free/offline: %#v", projection)
	}
	if projection.WorkflowIdentity.WorkflowID != workflowResult.WorkflowID ||
		projection.WorkflowIdentity.RunID != workflowResult.RunID ||
		projection.WorkflowIdentity.Entrypoint != workflowResult.Entrypoint ||
		projection.WorkflowIdentity.Status != workflowResult.Status {
		t.Fatalf("workflow identity does not resolve: projection=%#v source=%#v", projection.WorkflowIdentity, workflowResult)
	}

	routes := map[string]string{}
	for _, route := range product.RouteContracts {
		routes[route.RouteID] = route.HandlerRef
	}
	for _, mapping := range projection.RouteMappings {
		if routes[mapping.RouteID] != mapping.HandlerRef || mapping.ProjectionPolicy != "route_ref_only" {
			t.Fatalf("route mapping does not resolve: %#v routes=%#v", mapping, routes)
		}
	}

	stageResults := map[string]struct {
		Status       string
		OutputRef    string
		TraceEventID string
	}{}
	for _, stage := range workflowResult.StageResults {
		stageResults[stage.StageID] = struct {
			Status       string
			OutputRef    string
			TraceEventID string
		}{Status: stage.Status, OutputRef: stage.OutputRef, TraceEventID: stage.TraceEventID}
	}
	taskLogs := map[string]struct {
		Status        string
		CorrelationID string
		FixtureURI    string
	}{}
	for _, task := range product.TaskLogs {
		taskLogs[task.TaskID] = struct {
			Status        string
			CorrelationID string
			FixtureURI    string
		}{Status: task.Status, CorrelationID: task.CorrelationID, FixtureURI: task.FixtureURI}
	}
	if len(projection.TaskMappings) != len(workflowResult.StageResults) {
		t.Fatalf("task mapping count = %d, want %d", len(projection.TaskMappings), len(workflowResult.StageResults))
	}
	for _, mapping := range projection.TaskMappings {
		stage, ok := stageResults[mapping.StageID]
		task := taskLogs[mapping.TaskID]
		if !ok ||
			mapping.StageStatus != stage.Status ||
			mapping.StageOutputRef != stage.OutputRef ||
			mapping.TraceEventID != stage.TraceEventID ||
			mapping.TaskStatus != task.Status ||
			mapping.CorrelationID != task.CorrelationID ||
			mapping.FixtureURI != task.FixtureURI ||
			!strings.HasPrefix(mapping.FixtureURI, "fixture://") {
			t.Fatalf("task mapping does not resolve: mapping=%#v stage=%#v task=%#v", mapping, stage, task)
		}
	}

	workflowArtifacts := map[string]struct {
		Kind       string
		Status     string
		SourceRefs []string
	}{}
	for _, artifact := range workflowResult.Artifacts {
		workflowArtifacts[artifact.ArtifactID] = struct {
			Kind       string
			Status     string
			SourceRefs []string
		}{Kind: artifact.Kind, Status: artifact.Status, SourceRefs: artifact.SourceRefs}
	}
	downloads := map[string]struct {
		URI         string
		RemoteFetch bool
	}{}
	for _, download := range product.ArtifactDownloads {
		downloads[download.ArtifactID] = struct {
			URI         string
			RemoteFetch bool
		}{URI: download.URI, RemoteFetch: download.RemoteFetch}
	}
	for _, mapping := range projection.ArtifactMappings {
		artifact, ok := workflowArtifacts[mapping.WorkflowArtifactID]
		download := downloads[mapping.ProductArtifactID]
		if !ok ||
			mapping.WorkflowArtifactKind != artifact.Kind ||
			mapping.WorkflowArtifactStatus != artifact.Status ||
			!sameStringSet(mapping.SourceRefs, artifact.SourceRefs) ||
			mapping.DownloadURI != download.URI ||
			mapping.RemoteFetch || download.RemoteFetch ||
			!strings.HasPrefix(mapping.DownloadURI, "artifact://") {
			t.Fatalf("artifact mapping does not resolve: mapping=%#v artifact=%#v download=%#v", mapping, artifact, download)
		}
	}
	for _, want := range []string{"projection_is_provider_free", "workflow_identity_resolves", "all_stage_results_mapped_to_task_refs", "route_refs_resolve_in_product_fixture", "artifact_refs_resolve_in_product_fixture", "fixture_uris_only", "remote_fetch_disabled", "live_side_effects_absent"} {
		if !projection.ProjectionAssertions[want] {
			t.Fatalf("projection assertion missing %q: %#v", want, projection.ProjectionAssertions)
		}
	}
}

func TestGenericProductAppBoundaryLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericProductAppBoundaryPackageDir(t), "main.leia")
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
			got, err := vm.Get("generic_product_app_boundary_live_package_summary")
			if err != nil {
				t.Fatalf("Get summary: %v", err)
			}
			want := "generic_product_app_boundary_live_package capability=generic.ai.product_app.boundary entrypoint=ai.product_app.boundary routes=3 sessions=2 tasks=2 downloads=2 crud=3 migrations=1 deployments=2 approvals=2 clean_skip=3 workflow_projections=1 provider_free=true live_network=false imports=false model_calls=false"
			if got != want {
				t.Fatalf("summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

type genericProductAppBoundaryFixture struct {
	ProviderFree          bool `json:"provider_free"`
	LiveNetwork           bool `json:"live_network"`
	RealDependencyImports bool `json:"real_dependency_imports"`
	LiveModelCalls        bool `json:"live_model_calls"`
	RequiresCredentials   bool `json:"requires_credentials"`
	RouteContracts        []struct {
		RouteID    string `json:"route_id"`
		Method     string `json:"method"`
		Path       string `json:"path"`
		HandlerRef string `json:"handler_ref"`
		Capability string `json:"capability"`
	} `json:"route_contracts"`
	SessionContracts []struct {
		SessionID          string   `json:"session_id"`
		ActorRef           string   `json:"actor_ref"`
		Roles              []string `json:"roles"`
		CredentialRequired bool     `json:"credential_required"`
		CredentialPresent  bool     `json:"credential_present"`
		FixtureURI         string   `json:"fixture_uri"`
	} `json:"session_contracts"`
	TaskLogs []struct {
		TaskID        string `json:"task_id"`
		Status        string `json:"status"`
		CorrelationID string `json:"correlation_id"`
		ReplayReady   bool   `json:"replay_ready"`
		FixtureURI    string `json:"fixture_uri"`
	} `json:"task_logs"`
	ArtifactDownloads []struct {
		ArtifactID  string `json:"artifact_id"`
		URI         string `json:"uri"`
		Hash        string `json:"hash"`
		RemoteFetch bool   `json:"remote_fetch"`
	} `json:"artifact_downloads"`
	CRUDFixtureState []struct {
		OperationID   string `json:"operation_id"`
		Op            string `json:"op"`
		StateKey      string `json:"state_key"`
		FixtureURI    string `json:"fixture_uri"`
		Deterministic bool   `json:"deterministic"`
	} `json:"crud_fixture_state"`
	DBMigrationPlans []struct {
		MigrationID  string   `json:"migration_id"`
		LiveDatabase bool     `json:"live_database"`
		CleanSkip    bool     `json:"clean_skip"`
		OrderedSteps []string `json:"ordered_steps"`
	} `json:"db_migration_plans"`
	DeploymentTargets []struct {
		TargetID              string `json:"target_id"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		CleanSkip             bool   `json:"clean_skip"`
	} `json:"deployment_targets"`
	ApprovalBoundaries []struct {
		BoundaryID        string `json:"boundary_id"`
		Decision          string `json:"decision"`
		SideEffectAllowed bool   `json:"side_effect_allowed"`
		CleanSkip         bool   `json:"clean_skip"`
	} `json:"approval_boundaries"`
	AdapterBoundaries []struct {
		ID                 string `json:"id"`
		Capability         string `json:"capability"`
		DependencyImported bool   `json:"dependency_imported"`
		CredentialRequired bool   `json:"credential_required"`
		LiveNetwork        bool   `json:"live_network"`
		CleanSkip          bool   `json:"clean_skip"`
	} `json:"adapter_boundaries"`
}

func loadGenericProductAppBoundaryFixture(t *testing.T, path string) genericProductAppBoundaryFixture {
	t.Helper()
	var fixture genericProductAppBoundaryFixture
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func genericProductAppBoundaryPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_product_app_boundary")
}
