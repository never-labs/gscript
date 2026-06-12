package leia_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type finrobotProductWorkflowLiveManifest struct {
	SchemaVersion      int              `json:"schema_version"`
	ID                 string           `json:"id"`
	PackageName        string           `json:"package_name"`
	SourceExamples     []string         `json:"source_examples"`
	ProviderFree       bool             `json:"provider_free"`
	LiveNetworkDefault bool             `json:"live_network_default"`
	Credentials        credentialsBlock `json:"credentials"`
	DefaultPolicy      struct {
		Mode                        string `json:"mode"`
		LiveNetwork                 bool   `json:"live_network"`
		ProviderCredentialsRequired bool   `json:"provider_credentials_required"`
		RealDependencyImports       bool   `json:"real_dependency_imports"`
		CleanSkipWithoutDependency  bool   `json:"clean_skip_without_dependency"`
		FixtureHook                 string `json:"fixture_hook"`
	} `json:"default_policy"`
	Entrypoints               map[string]string `json:"entrypoints"`
	Schemas                   map[string]string `json:"schemas"`
	Fixtures                  map[string]string `json:"fixtures"`
	Capabilities              []string          `json:"capabilities"`
	DeploymentCapabilityGates []struct {
		ID                  string   `json:"id"`
		Capability          string   `json:"capability"`
		LiveNetwork         bool     `json:"live_network"`
		RequiresCredentials bool     `json:"requires_credentials"`
		Checks              []string `json:"checks"`
	} `json:"deployment_capability_gates"`
	TestGates []string `json:"test_gates"`
}

type credentialsBlock struct {
	Required          []string `json:"required"`
	Optional          []string `json:"optional"`
	SecretEnvPatterns []string `json:"secret_env_patterns"`
	Policy            string   `json:"policy"`
}

func TestFinRobotProductWorkflowLivePackageManifestSchemaAndGates(t *testing.T) {
	base := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "product_workflow")
	manifest := loadProductWorkflowLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-product-workflow-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-product-workflow" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault {
		t.Fatalf("provider-free/live network defaults = provider_free:%v live_network:%v", manifest.ProviderFree, manifest.LiveNetworkDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	if !strings.Contains(strings.ToLower(manifest.Credentials.Policy), "no credentials") {
		t.Fatalf("credential policy should document no credentials: %q", manifest.Credentials.Policy)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_product_workflow_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}

	for _, source := range manifest.SourceExamples {
		if _, err := os.Stat(filepath.Join(repoRoot(t), source)); err != nil {
			t.Fatalf("source example %q: %v", source, err)
		}
	}
	for _, key := range []string{"smoke", "equity_cli_workflow", "web_product", "ui_template_snapshots", "static_asset_manifest", "db_migrations", "deployment_capability_gates"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	if manifest.Entrypoints["smoke"] != "main.leia" {
		t.Fatalf("smoke entrypoint = %q, want main.leia", manifest.Entrypoints["smoke"])
	}
	for _, key := range []string{"workflow_run", "web_product", "ui_template_snapshot", "db_migration", "deployment_gate"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertJSONFile(t, filepath.Join(base, path))
	}
	if manifest.Fixtures["index"] == "" {
		t.Fatal("missing provider-free fixture index")
	}
	assertJSONFile(t, filepath.Join(base, manifest.Fixtures["index"]))

	wantCapabilities := []string{
		"product.workflow.equity_cli.run",
		"product.workflow.web.route",
		"product.workflow.web.template_snapshot",
		"product.workflow.web.accessibility_snapshot_gate",
		"product.workflow.auth.session",
		"product.workflow.history.read",
		"product.workflow.task_log.read",
		"product.workflow.download.artifact",
		"product.workflow.download.artifact_manifest",
		"product.workflow.crud.report_request",
		"product.workflow.approval.boundary",
		"product.workflow.replay.fixture",
		"product.workflow.db.migration.plan",
		"product.workflow.deploy.local",
		"product.workflow.deploy.container",
		"product.workflow.deploy.cloud",
	}
	for _, want := range wantCapabilities {
		if !contains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}
	if len(manifest.DeploymentCapabilityGates) < 3 {
		t.Fatalf("deployment gates = %d, want at least 3", len(manifest.DeploymentCapabilityGates))
	}
	for _, gate := range manifest.DeploymentCapabilityGates {
		if !strings.HasPrefix(gate.Capability, "product.workflow.deploy.") {
			t.Fatalf("deployment gate %s capability = %q", gate.ID, gate.Capability)
		}
		if gate.LiveNetwork || gate.RequiresCredentials {
			t.Fatalf("deployment gate %s must be provider-free: %#v", gate.ID, gate)
		}
		if len(gate.Checks) == 0 {
			t.Fatalf("deployment gate %s missing checks", gate.ID)
		}
	}
	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"provider_free", "credentials", "equity cli", "web product", "capability gates", "crud command", "artifact manifests", "approval boundaries", "workflow replay"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
	for _, want := range []string{"template", "asset", "accessibility", "visual", "browser"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing UI snapshot term %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotProductWorkflowLivePackageContracts(t *testing.T) {
	base := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "product_workflow")
	var workflow struct {
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
		Workflows    map[string]struct {
			Stages    []string `json:"stages"`
			Routes    []string `json:"routes"`
			Contracts []string `json:"contracts"`
		} `json:"workflows"`
		RouteSessionStateContract struct {
			Capability          string `json:"capability"`
			Mode                string `json:"mode"`
			RequiresBrowser     bool   `json:"requires_browser"`
			RequiresLiveNetwork bool   `json:"requires_live_network"`
			RequiresCredentials bool   `json:"requires_credentials"`
			StateSource         string `json:"state_source"`
			Routes              []struct {
				Route                string   `json:"route"`
				Method               string   `json:"method"`
				AllowedSessionStates []string `json:"allowed_session_states"`
				RequiredRole         string   `json:"required_role"`
				SideEffects          []string `json:"side_effects"`
			} `json:"routes"`
			DeniedCases []string `json:"denied_cases"`
			Invariants  struct {
				ProtectedRoutesRequireAuthenticatedState bool `json:"protected_routes_require_authenticated_state"`
				PostRoutesRequireCSRF                    bool `json:"post_routes_require_csrf"`
				AdminRoutesRequireAdminRole              bool `json:"admin_routes_require_admin_role"`
				DeniedCasesFixtureCovered                bool `json:"denied_cases_fixture_covered"`
				SessionStateChangesEmitAuditEvent        bool `json:"session_state_changes_emit_audit_event"`
			} `json:"invariants"`
		} `json:"route_session_state_contract"`
		AuthSession map[string]any `json:"auth_session"`
		RequestSM   struct {
			Capability     string   `json:"capability"`
			States         []string `json:"states"`
			InitialState   string   `json:"initial_state"`
			TerminalStates []string `json:"terminal_states"`
			Transitions    []struct {
				From         string   `json:"from"`
				Event        string   `json:"event"`
				To           string   `json:"to"`
				RequiresRole string   `json:"requires_role"`
				Writes       []string `json:"writes"`
			} `json:"transitions"`
			FixturePath string `json:"fixture_path"`
		} `json:"report_request_state_machine"`
		History map[string]any `json:"history"`
		TaskLog struct {
			Capability string `json:"capability"`
			Format     string `json:"format"`
			Ordering   struct {
				Key              []string `json:"key"`
				SequenceColumn   string   `json:"sequence_column"`
				MonotonicPerTask bool     `json:"monotonic_per_task"`
				TieBreaker       string   `json:"tie_breaker"`
				Clock            string   `json:"clock"`
				FixturePath      string   `json:"fixture_path"`
			} `json:"ordering"`
			Events    []string `json:"events"`
			Redaction string   `json:"redaction"`
		} `json:"task_log"`
		Download struct {
			Capability    string   `json:"capability"`
			Artifacts     []string `json:"artifacts"`
			Authorization struct {
				Mode                  string   `json:"mode"`
				RequiresSessionState  string   `json:"requires_session_state"`
				RequiresArtifactState string   `json:"requires_artifact_state"`
				DeniedCases           []string `json:"denied_cases"`
				AuditEvent            string   `json:"audit_event"`
				FixturePath           string   `json:"fixture_path"`
			} `json:"authorization"`
			RemoteObjectFetch bool `json:"remote_object_fetch"`
		} `json:"download"`
		ArtifactManifestContract struct {
			Capability          string   `json:"capability"`
			ProviderFree        bool     `json:"provider_free"`
			LiveNetwork         bool     `json:"live_network"`
			RequiresCredentials bool     `json:"requires_credentials"`
			ManifestPath        string   `json:"manifest_path"`
			RequiredFields      []string `json:"required_fields"`
			ArtifactFields      []string `json:"artifact_fields"`
			PathPolicy          struct {
				LocalFixturePrefix string `json:"local_fixture_prefix"`
				RemoteURIAllowed   bool   `json:"remote_uri_allowed"`
				SignedURLAllowed   bool   `json:"signed_url_allowed"`
				ChecksumAlgorithm  string `json:"checksum_algorithm"`
			} `json:"path_policy"`
		} `json:"artifact_manifest_contract"`
		CRUD struct {
			Capability      string   `json:"capability"`
			Operations      []string `json:"operations"`
			Tables          []string `json:"tables"`
			CommandEnvelope struct {
				Mode                string   `json:"mode"`
				RequiresLiveNetwork bool     `json:"requires_live_network"`
				RequiresCredentials bool     `json:"requires_credentials"`
				RequiredFields      []string `json:"required_fields"`
				Operations          []struct {
					Operation    string   `json:"operation"`
					AllowedFrom  []string `json:"allowed_from"`
					AllowedTo    string   `json:"allowed_to"`
					RequiresRole string   `json:"requires_role"`
				} `json:"operations"`
				FixturePath string `json:"fixture_path"`
			} `json:"command_envelope"`
		} `json:"crud"`
		ApprovalBoundary struct {
			Capability                      string   `json:"capability"`
			ProviderFree                    bool     `json:"provider_free"`
			DefaultDecision                 string   `json:"default_decision"`
			FixturePath                     string   `json:"fixture_path"`
			RequiresExplicitCapabilityGrant bool     `json:"requires_explicit_capability_grant"`
			DeniedByDefault                 []string `json:"denied_by_default"`
			AllowedWithoutApproval          []string `json:"allowed_without_approval"`
		} `json:"approval_boundary"`
		WorkflowReplay struct {
			Capability                string   `json:"capability"`
			ProviderFree              bool     `json:"provider_free"`
			LiveNetwork               bool     `json:"live_network"`
			RequiresCredentials       bool     `json:"requires_credentials"`
			RequiresProviderCallbacks bool     `json:"requires_provider_callbacks"`
			FixturePath               string   `json:"fixture_path"`
			Inputs                    []string `json:"inputs"`
			OrderedEventSources       []string `json:"ordered_event_sources"`
			Outputs                   []string `json:"outputs"`
			Determinism               struct {
				Clock             string `json:"clock"`
				RandomSeed        string `json:"random_seed"`
				ExternalCallbacks bool   `json:"external_callbacks"`
			} `json:"determinism"`
		} `json:"workflow_replay"`
	}
	decodeJSONFile(t, filepath.Join(base, "contracts", "product_workflow_contract.json"), &workflow)
	if !workflow.ProviderFree || workflow.LiveNetwork {
		t.Fatalf("workflow contract provider-free/live_network = %v/%v", workflow.ProviderFree, workflow.LiveNetwork)
	}
	if len(workflow.Workflows["equity_cli"].Stages) != 7 {
		t.Fatalf("equity CLI stages = %#v", workflow.Workflows["equity_cli"].Stages)
	}
	if len(workflow.Workflows["web_product"].Routes) < 12 {
		t.Fatalf("web routes = %#v", workflow.Workflows["web_product"].Routes)
	}
	for name, block := range map[string]map[string]any{
		"auth_session": workflow.AuthSession,
		"history":      workflow.History,
	} {
		if block["capability"] == "" {
			t.Fatalf("%s missing capability: %#v", name, block)
		}
	}
	if workflow.TaskLog.Capability != "product.workflow.task_log.read" || workflow.TaskLog.Format != "jsonl" {
		t.Fatalf("task log contract = %#v", workflow.TaskLog)
	}
	if workflow.Download.Capability != "product.workflow.download.artifact" || workflow.Download.RemoteObjectFetch {
		t.Fatalf("download contract = %#v", workflow.Download)
	}
	assertProductWorkflowUISnapshotContract(t, base, workflow.Workflows["web_product"].Routes)
	assertRouteSessionStateContract(t, workflow.RouteSessionStateContract, workflow.Workflows["web_product"].Routes)
	assertProductWorkflowLifecycleContract(t, workflow.AuthSession)
	assertReportRequestStateMachine(t, workflow.RequestSM)
	assertTaskEventOrderingContract(t, workflow.TaskLog.Ordering.Key, workflow.TaskLog.Ordering.SequenceColumn, workflow.TaskLog.Ordering.MonotonicPerTask)
	assertDownloadAuthorizationContract(t, workflow.Download.Authorization.Mode, workflow.Download.Authorization.RequiresSessionState, workflow.Download.Authorization.RequiresArtifactState, workflow.Download.Authorization.DeniedCases)
	assertArtifactManifestContract(t, workflow.ArtifactManifestContract)
	assertCRUDCommandEnvelopeContract(t, workflow.CRUD)
	assertApprovalBoundaryContract(t, workflow.ApprovalBoundary)
	assertWorkflowReplayContract(t, workflow.WorkflowReplay)

	var migration struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		Capability   string `json:"capability"`
		Versioning   struct {
			CurrentVersion int    `json:"current_version"`
			SchemaTable    string `json:"schema_table"`
			Ordering       string `json:"ordering"`
			Fixtures       struct {
				AppliedVersions string `json:"applied_versions"`
				RollbackRuns    string `json:"rollback_runs"`
			} `json:"fixtures"`
		} `json:"versioning"`
		Migrations []struct {
			ID              string `json:"id"`
			Version         int    `json:"version"`
			PreviousVersion int    `json:"previous_version"`
			Tables          []struct {
				Name    string   `json:"name"`
				Columns []string `json:"columns"`
			} `json:"tables"`
			RollbackRequired bool `json:"rollback_required"`
			Rollback         struct {
				ID             string   `json:"id"`
				Direction      string   `json:"direction"`
				FromVersion    int      `json:"from_version"`
				ToVersion      int      `json:"to_version"`
				FixturePath    string   `json:"fixture_path"`
				DropsTables    []string `json:"drops_tables"`
				RemovesColumns []string `json:"removes_columns"`
			} `json:"rollback"`
		} `json:"migrations"`
	}
	decodeJSONFile(t, filepath.Join(base, "contracts", "db_migration_contract.json"), &migration)
	if !migration.ProviderFree || migration.LiveNetwork || migration.Capability != "product.workflow.db.migration.plan" {
		t.Fatalf("migration contract header = %#v", migration)
	}
	if migration.Versioning.CurrentVersion != 2 || migration.Versioning.Ordering != "strictly_increasing_integer" {
		t.Fatalf("migration versioning = %#v", migration.Versioning)
	}
	if migration.Versioning.Fixtures.AppliedVersions == "" || migration.Versioning.Fixtures.RollbackRuns == "" {
		t.Fatalf("migration versioning missing fixtures: %#v", migration.Versioning.Fixtures)
	}
	if len(migration.Migrations) != 2 {
		t.Fatalf("migration contract migrations = %#v", migration.Migrations)
	}
	for i, m := range migration.Migrations {
		wantVersion := i + 1
		if m.Version != wantVersion || m.PreviousVersion != i {
			t.Fatalf("migration %s version chain = %d <- %d, want %d <- %d", m.ID, m.Version, m.PreviousVersion, wantVersion, i)
		}
		if !m.RollbackRequired || m.Rollback.Direction != "down" || m.Rollback.FromVersion != m.Version || m.Rollback.ToVersion != m.PreviousVersion || m.Rollback.FixturePath == "" {
			t.Fatalf("migration %s rollback contract = %#v", m.ID, m.Rollback)
		}
	}
	wantTables := []string{"users", "sessions", "report_requests", "reports", "artifacts", "task_events", "audit_events", "schema_versions"}
	for _, want := range wantTables {
		found := false
		for _, migrationStep := range migration.Migrations {
			for _, table := range migrationStep.Tables {
				if table.Name != want {
					continue
				}
				if contains(table.Columns, "id") || want == "schema_versions" && contains(table.Columns, "version") {
					found = true
				}
				if table.Name == "task_events" && !contains(table.Columns, "sequence") {
					t.Fatalf("task_events missing sequence column in migration %s: %#v", migrationStep.ID, table.Columns)
				}
				if table.Name == "artifacts" && contains(table.Columns, "authorized_role") && !contains(table.Columns, "owner_user_id") {
					t.Fatalf("artifact authorization columns incomplete in migration %s: %#v", migrationStep.ID, table.Columns)
				}
			}
			if want == "schema_rollbacks" {
				found = true
			}
		}
		if !found {
			t.Fatalf("migration missing table %q in %#v", want, migration.Migrations)
		}
	}
	assertProductWorkflowFixtureContracts(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"))

	var gates struct {
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
		Gates        []struct {
			Capability          string `json:"capability"`
			RequiresCredentials bool   `json:"requires_credentials"`
			LiveNetwork         bool   `json:"live_network"`
		} `json:"gates"`
	}
	decodeJSONFile(t, filepath.Join(base, "contracts", "deployment_capability_gates.json"), &gates)
	if !gates.ProviderFree || gates.LiveNetwork || len(gates.Gates) < 6 {
		t.Fatalf("deployment gate contract = %#v", gates)
	}
	for _, gate := range gates.Gates {
		if !strings.HasPrefix(gate.Capability, "product.workflow.") {
			t.Fatalf("gate capability = %q", gate.Capability)
		}
		if gate.RequiresCredentials || gate.LiveNetwork {
			t.Fatalf("gate must not require credentials or live network: %#v", gate)
		}
	}
}

func TestFinRobotProductWorkflowLivePackageNoLiveImports(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(productWorkflowLivePackageDir(t), "main.leia"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pattern := range []string{
		`(?m)^\s*import\s+`,
		`(?m)^\s*use\s+`,
		`(?m)^\s*load\s*\(`,
		`(?m)^\s*require\s*\(`,
		`(?m)^\s*(requests|http|fetch|sql|sqlite|postgres|mysql|redis|s3|boto|yfinance|finnhub|openbb)\s*[.(]`,
	} {
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("main.leia contains live dependency loader matching %q", pattern)
		}
	}
}

func TestFinRobotProductWorkflowLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(productWorkflowLivePackageDir(t), "main.leia")

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
			got, err := vm.Get("product_workflow_live_package_summary")
			if err != nil {
				t.Fatalf("Get product_workflow_live_package_summary: %v", err)
			}
			want := "product_workflow_live_package routes=12 auth=5 request_states=8 task_events=5 downloads=4 db_migrations=2 db_tables=9 ui_routes=12 accessibility=9 a11y_gate_results=3 route_sessions=3 crud_commands=3 artifact_items=4 approvals=4 replay_events=8 gates=6 provider_free=true live_network=false imports=false"
			if got != want {
				t.Fatalf("product_workflow_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func TestFinRobotProductWorkflowLivePackageRegisteredExample(t *testing.T) {
	const path = "examples/ai/finrobot_translation/live_packages/product_workflow/main.leia"
	root := repoRoot(t)
	examples := finrobotProductWorkflowRegistryExamples(t, root)
	var found bool
	for _, example := range examples {
		if filepath.ToSlash(example.Path) != path {
			continue
		}
		found = true
		if !example.Runnable || !example.Checkable || example.Runner != "host-vm" || example.Requires != "" {
			t.Fatalf("product workflow main registry metadata = %#v", example)
		}
	}
	if !found {
		t.Fatalf("examples list missing %s", path)
	}

	report := finrobotProductWorkflowExamplesCheck(t, root, path)
	if report.SchemaVersion != 1 || !report.OK || report.Runnable != 1 || report.Skipped != 0 || report.Failed != 0 || len(report.Results) != 1 {
		t.Fatalf("examples check summary = %#v", report)
	}
	result := report.Results[0]
	if filepath.ToSlash(result.Path) != path || result.Status != "ok" || result.Requires != "" || result.Error != "" {
		t.Fatalf("examples check result = %#v", result)
	}
}

func assertProductWorkflowUISnapshotContract(t *testing.T, base string, webRoutes []string) {
	t.Helper()
	var ui struct {
		ProviderFree        bool   `json:"provider_free"`
		LiveNetwork         bool   `json:"live_network"`
		RequiresCredentials bool   `json:"requires_credentials"`
		RequiresBrowser     bool   `json:"requires_browser"`
		Capability          string `json:"capability"`
		StaticAssetManifest string `json:"static_asset_manifest"`
		RouteMapping        []struct {
			Route         string   `json:"route"`
			Path          string   `json:"path"`
			TemplateID    string   `json:"template_id"`
			TemplatePath  string   `json:"template_path"`
			RequiredSlots []string `json:"required_slots"`
			SnapshotID    string   `json:"snapshot_id"`
		} `json:"route_to_template_mapping"`
		Accessibility struct {
			Mode                string   `json:"mode"`
			RequiresBrowser     bool     `json:"requires_browser"`
			RequiresLiveNetwork bool     `json:"requires_live_network"`
			RequiresCredentials bool     `json:"requires_credentials"`
			Standards           []string `json:"standards"`
			Checks              []string `json:"checks"`
			SnapshotFixturePath string   `json:"snapshot_fixture_path"`
		} `json:"accessibility_snapshot_requirements"`
		AccessibilityGate struct {
			Capability          string `json:"capability"`
			Mode                string `json:"mode"`
			ProviderFree        bool   `json:"provider_free"`
			RequiresBrowser     bool   `json:"requires_browser"`
			RequiresLiveNetwork bool   `json:"requires_live_network"`
			RequiresCredentials bool   `json:"requires_credentials"`
			Coverage            struct {
				RequiredRouteSource   string `json:"required_route_source"`
				RequiredRouteCoverage string `json:"required_route_coverage"`
				FixturePath           string `json:"fixture_path"`
			} `json:"coverage"`
			BlockingViolationLevels []string `json:"blocking_violation_levels"`
			WaiversAllowed          bool     `json:"waivers_allowed"`
			FailOn                  []string `json:"fail_on"`
		} `json:"accessibility_snapshot_gate"`
		Visual struct {
			Mode                string `json:"mode"`
			RequiresBrowser     bool   `json:"requires_browser"`
			RequiresLiveNetwork bool   `json:"requires_live_network"`
			RequiresCredentials bool   `json:"requires_credentials"`
			SnapshotSource      string `json:"snapshot_source"`
			Viewports           []struct {
				ID                string `json:"id"`
				Width             int    `json:"width"`
				Height            int    `json:"height"`
				DeviceScaleFactor int    `json:"device_scale_factor"`
			} `json:"viewports"`
			MetadataFields      []string `json:"metadata_fields"`
			SnapshotFixturePath string   `json:"snapshot_fixture_path"`
			DiffPolicy          struct {
				Baseline             string `json:"baseline"`
				PixelThreshold       int    `json:"pixel_threshold"`
				LayoutShiftThreshold int    `json:"layout_shift_threshold"`
				FontSource           string `json:"font_source"`
			} `json:"diff_policy"`
		} `json:"visual_regression_metadata"`
	}
	decodeJSONFile(t, filepath.Join(base, "contracts", "ui_template_snapshot_contract.json"), &ui)
	if !ui.ProviderFree || ui.LiveNetwork || ui.RequiresCredentials || ui.RequiresBrowser {
		t.Fatalf("UI snapshot contract must be provider-free and browserless: %#v", ui)
	}
	if ui.Capability != "product.workflow.web.template_snapshot" || ui.StaticAssetManifest == "" {
		t.Fatalf("UI snapshot contract header = %#v", ui)
	}
	if len(ui.RouteMapping) != len(webRoutes) {
		t.Fatalf("UI route mapping count = %d, want %d", len(ui.RouteMapping), len(webRoutes))
	}
	templates := map[string]bool{}
	for _, route := range webRoutes {
		found := false
		for _, mapping := range ui.RouteMapping {
			if mapping.Route != route {
				continue
			}
			found = true
			templates[mapping.TemplateID] = true
			if mapping.Path == "" || mapping.TemplatePath == "" || len(mapping.RequiredSlots) < 3 || mapping.SnapshotID == "" {
				t.Fatalf("incomplete UI route mapping for %q: %#v", route, mapping)
			}
			if !strings.HasPrefix(mapping.TemplatePath, "templates/") || strings.Contains(mapping.TemplatePath, "://") {
				t.Fatalf("template path must be local templates path: %#v", mapping)
			}
		}
		if !found {
			t.Fatalf("web route %q missing UI template mapping: %#v", route, ui.RouteMapping)
		}
	}
	assertAccessibilitySnapshotRequirements(t, ui.Accessibility)
	assertAccessibilitySnapshotGate(t, ui.AccessibilityGate)
	assertVisualRegressionMetadata(t, ui.Visual)
	assertStaticAssetManifest(t, filepath.Join(base, ui.StaticAssetManifest), templates)
}

func assertAccessibilitySnapshotRequirements(t *testing.T, accessibility struct {
	Mode                string   `json:"mode"`
	RequiresBrowser     bool     `json:"requires_browser"`
	RequiresLiveNetwork bool     `json:"requires_live_network"`
	RequiresCredentials bool     `json:"requires_credentials"`
	Standards           []string `json:"standards"`
	Checks              []string `json:"checks"`
	SnapshotFixturePath string   `json:"snapshot_fixture_path"`
}) {
	t.Helper()
	if accessibility.Mode != "static_dom_contract" || accessibility.RequiresBrowser || accessibility.RequiresLiveNetwork || accessibility.RequiresCredentials {
		t.Fatalf("accessibility snapshot requirements must be static/provider-free: %#v", accessibility)
	}
	for _, want := range []string{"WCAG_2_2_AA_static_subset", "ARIA_landmark_names"} {
		if !contains(accessibility.Standards, want) {
			t.Fatalf("accessibility standards missing %q: %#v", want, accessibility.Standards)
		}
	}
	for _, want := range []string{"one_main_landmark", "one_h1_per_route", "form_controls_have_labels", "buttons_have_accessible_names", "tables_have_captions_or_aria_labels"} {
		if !contains(accessibility.Checks, want) {
			t.Fatalf("accessibility checks missing %q: %#v", want, accessibility.Checks)
		}
	}
	if !strings.Contains(accessibility.SnapshotFixturePath, "provider_free_fixture_index.json#") {
		t.Fatalf("accessibility fixture path must point at provider-free fixture index: %q", accessibility.SnapshotFixturePath)
	}
}

func assertAccessibilitySnapshotGate(t *testing.T, gate struct {
	Capability          string `json:"capability"`
	Mode                string `json:"mode"`
	ProviderFree        bool   `json:"provider_free"`
	RequiresBrowser     bool   `json:"requires_browser"`
	RequiresLiveNetwork bool   `json:"requires_live_network"`
	RequiresCredentials bool   `json:"requires_credentials"`
	Coverage            struct {
		RequiredRouteSource   string `json:"required_route_source"`
		RequiredRouteCoverage string `json:"required_route_coverage"`
		FixturePath           string `json:"fixture_path"`
	} `json:"coverage"`
	BlockingViolationLevels []string `json:"blocking_violation_levels"`
	WaiversAllowed          bool     `json:"waivers_allowed"`
	FailOn                  []string `json:"fail_on"`
}) {
	t.Helper()
	if gate.Capability != "product.workflow.web.accessibility_snapshot_gate" || gate.Mode != "static_fixture_gate" || !gate.ProviderFree {
		t.Fatalf("accessibility snapshot gate header = %#v", gate)
	}
	if gate.RequiresBrowser || gate.RequiresLiveNetwork || gate.RequiresCredentials || gate.WaiversAllowed {
		t.Fatalf("accessibility snapshot gate must be provider-free/browserless with no waivers: %#v", gate)
	}
	if gate.Coverage.RequiredRouteSource != "route_to_template_mapping" || gate.Coverage.RequiredRouteCoverage != "all_routes" {
		t.Fatalf("accessibility gate coverage = %#v", gate.Coverage)
	}
	if !strings.Contains(gate.Coverage.FixturePath, "provider_free_fixture_index.json#") {
		t.Fatalf("accessibility gate fixture path must point at provider-free fixture index: %q", gate.Coverage.FixturePath)
	}
	for _, want := range []string{"critical", "serious"} {
		if !contains(gate.BlockingViolationLevels, want) {
			t.Fatalf("accessibility gate blocking levels missing %q: %#v", want, gate.BlockingViolationLevels)
		}
	}
	for _, want := range []string{"missing_route_snapshot", "missing_main_landmark", "unlabelled_form_control", "browser_required"} {
		if !contains(gate.FailOn, want) {
			t.Fatalf("accessibility gate fail_on missing %q: %#v", want, gate.FailOn)
		}
	}
}

func assertVisualRegressionMetadata(t *testing.T, visual struct {
	Mode                string `json:"mode"`
	RequiresBrowser     bool   `json:"requires_browser"`
	RequiresLiveNetwork bool   `json:"requires_live_network"`
	RequiresCredentials bool   `json:"requires_credentials"`
	SnapshotSource      string `json:"snapshot_source"`
	Viewports           []struct {
		ID                string `json:"id"`
		Width             int    `json:"width"`
		Height            int    `json:"height"`
		DeviceScaleFactor int    `json:"device_scale_factor"`
	} `json:"viewports"`
	MetadataFields      []string `json:"metadata_fields"`
	SnapshotFixturePath string   `json:"snapshot_fixture_path"`
	DiffPolicy          struct {
		Baseline             string `json:"baseline"`
		PixelThreshold       int    `json:"pixel_threshold"`
		LayoutShiftThreshold int    `json:"layout_shift_threshold"`
		FontSource           string `json:"font_source"`
	} `json:"diff_policy"`
}) {
	t.Helper()
	if visual.Mode != "static_template_asset_contract" || visual.RequiresBrowser || visual.RequiresLiveNetwork || visual.RequiresCredentials {
		t.Fatalf("visual regression metadata must be static/provider-free: %#v", visual)
	}
	if visual.SnapshotSource != "template_hash_plus_static_asset_manifest" || len(visual.Viewports) < 2 {
		t.Fatalf("visual snapshot source/viewports = %#v", visual)
	}
	for _, want := range []string{"mobile", "desktop"} {
		found := false
		for _, viewport := range visual.Viewports {
			found = found || viewport.ID == want && viewport.Width > 0 && viewport.Height > 0 && viewport.DeviceScaleFactor > 0
		}
		if !found {
			t.Fatalf("visual metadata missing viewport %q: %#v", want, visual.Viewports)
		}
	}
	for _, want := range []string{"route", "template_id", "snapshot_id", "viewport_id", "fixture_dataset_id", "template_sha256", "asset_manifest_sha256"} {
		if !contains(visual.MetadataFields, want) {
			t.Fatalf("visual metadata fields missing %q: %#v", want, visual.MetadataFields)
		}
	}
	if visual.DiffPolicy.Baseline != "provider_free_fixture" || visual.DiffPolicy.PixelThreshold != 0 || visual.DiffPolicy.LayoutShiftThreshold != 0 || visual.DiffPolicy.FontSource != "system_ui_only" {
		t.Fatalf("visual diff policy = %#v", visual.DiffPolicy)
	}
	if !strings.Contains(visual.SnapshotFixturePath, "provider_free_fixture_index.json#") {
		t.Fatalf("visual fixture path must point at provider-free fixture index: %q", visual.SnapshotFixturePath)
	}
}

func assertStaticAssetManifest(t *testing.T, path string, templates map[string]bool) {
	t.Helper()
	var manifest struct {
		ProviderFree        bool `json:"provider_free"`
		LiveNetwork         bool `json:"live_network"`
		RequiresCredentials bool `json:"requires_credentials"`
		RequiresBrowser     bool `json:"requires_browser"`
		AssetPolicy         struct {
			ExternalFetch bool   `json:"external_fetch"`
			RemoteFonts   bool   `json:"remote_fonts"`
			InlineOnly    bool   `json:"inline_only"`
			HashAlgorithm string `json:"hash_algorithm"`
		} `json:"asset_policy"`
		Assets []struct {
			ID              string   `json:"id"`
			Type            string   `json:"type"`
			Path            string   `json:"path"`
			FixtureSHA256   string   `json:"fixture_sha256"`
			UsedByTemplates []string `json:"used_by_templates"`
		} `json:"assets"`
		TemplateAssets []struct {
			TemplateID string   `json:"template_id"`
			AssetIDs   []string `json:"asset_ids"`
		} `json:"template_assets"`
	}
	decodeJSONFile(t, path, &manifest)
	if !manifest.ProviderFree || manifest.LiveNetwork || manifest.RequiresCredentials || manifest.RequiresBrowser {
		t.Fatalf("static asset manifest must be provider-free/browserless: %#v", manifest)
	}
	if manifest.AssetPolicy.ExternalFetch || manifest.AssetPolicy.RemoteFonts || !manifest.AssetPolicy.InlineOnly || manifest.AssetPolicy.HashAlgorithm != "sha256" {
		t.Fatalf("static asset policy = %#v", manifest.AssetPolicy)
	}
	assets := map[string]bool{}
	for _, asset := range manifest.Assets {
		if asset.ID == "" || asset.Type == "" || asset.FixtureSHA256 == "" || len(asset.UsedByTemplates) == 0 {
			t.Fatalf("incomplete static asset entry: %#v", asset)
		}
		if strings.Contains(asset.Path, "://") || !strings.HasPrefix(asset.Path, "templates/assets/") {
			t.Fatalf("static asset path must be local templates/assets path: %#v", asset)
		}
		assets[asset.ID] = true
	}
	mappedTemplates := map[string]bool{}
	for _, templateAssets := range manifest.TemplateAssets {
		if !templates[templateAssets.TemplateID] {
			t.Fatalf("static asset manifest references unknown template %q", templateAssets.TemplateID)
		}
		if len(templateAssets.AssetIDs) == 0 {
			t.Fatalf("template %q missing asset refs", templateAssets.TemplateID)
		}
		for _, assetID := range templateAssets.AssetIDs {
			if !assets[assetID] {
				t.Fatalf("template %q references unknown asset %q", templateAssets.TemplateID, assetID)
			}
		}
		mappedTemplates[templateAssets.TemplateID] = true
	}
	for templateID := range templates {
		if !mappedTemplates[templateID] {
			t.Fatalf("template %q missing static asset mapping", templateID)
		}
	}
}

func assertRouteSessionStateContract(t *testing.T, contract struct {
	Capability          string `json:"capability"`
	Mode                string `json:"mode"`
	RequiresBrowser     bool   `json:"requires_browser"`
	RequiresLiveNetwork bool   `json:"requires_live_network"`
	RequiresCredentials bool   `json:"requires_credentials"`
	StateSource         string `json:"state_source"`
	Routes              []struct {
		Route                string   `json:"route"`
		Method               string   `json:"method"`
		AllowedSessionStates []string `json:"allowed_session_states"`
		RequiredRole         string   `json:"required_role"`
		SideEffects          []string `json:"side_effects"`
	} `json:"routes"`
	DeniedCases []string `json:"denied_cases"`
	Invariants  struct {
		ProtectedRoutesRequireAuthenticatedState bool `json:"protected_routes_require_authenticated_state"`
		PostRoutesRequireCSRF                    bool `json:"post_routes_require_csrf"`
		AdminRoutesRequireAdminRole              bool `json:"admin_routes_require_admin_role"`
		DeniedCasesFixtureCovered                bool `json:"denied_cases_fixture_covered"`
		SessionStateChangesEmitAuditEvent        bool `json:"session_state_changes_emit_audit_event"`
	} `json:"invariants"`
}, webRoutes []string) {
	t.Helper()
	if contract.Capability != "product.workflow.web.route" || contract.Mode != "fixture_session_state" {
		t.Fatalf("route/session contract header = %#v", contract)
	}
	if contract.RequiresBrowser || contract.RequiresLiveNetwork || contract.RequiresCredentials {
		t.Fatalf("route/session contract must be static/provider-free: %#v", contract)
	}
	if !strings.Contains(contract.StateSource, "provider_free_fixture_index.json#") {
		t.Fatalf("route/session state source must point at fixture index: %q", contract.StateSource)
	}
	if len(contract.Routes) != len(webRoutes) {
		t.Fatalf("route/session contract route count = %d, want %d", len(contract.Routes), len(webRoutes))
	}
	for _, route := range webRoutes {
		found := false
		for _, entry := range contract.Routes {
			if entry.Route != route {
				continue
			}
			found = true
			if entry.Method == "" || len(entry.AllowedSessionStates) == 0 || entry.RequiredRole == "" {
				t.Fatalf("route/session entry incomplete for %q: %#v", route, entry)
			}
			if strings.HasPrefix(route, "admin_") && entry.RequiredRole != "admin" {
				t.Fatalf("admin route %q must require admin role: %#v", route, entry)
			}
			if route != "home" && route != "login" && route != "oauth_callback" && !contains(entry.AllowedSessionStates, "authenticated") {
				t.Fatalf("protected route %q must allow authenticated fixture state: %#v", route, entry)
			}
		}
		if !found {
			t.Fatalf("web route %q missing route/session contract: %#v", route, contract.Routes)
		}
	}
	for _, want := range []string{"anonymous_on_protected_route", "expired_session", "csrf_missing_for_post", "role_mismatch"} {
		if !contains(contract.DeniedCases, want) {
			t.Fatalf("route/session denied cases missing %q: %#v", want, contract.DeniedCases)
		}
	}
	if !contract.Invariants.ProtectedRoutesRequireAuthenticatedState ||
		!contract.Invariants.PostRoutesRequireCSRF ||
		!contract.Invariants.AdminRoutesRequireAdminRole ||
		!contract.Invariants.DeniedCasesFixtureCovered ||
		!contract.Invariants.SessionStateChangesEmitAuditEvent {
		t.Fatalf("route/session invariants must all be explicit and enabled: %#v", contract.Invariants)
	}
}

func assertProductWorkflowLifecycleContract(t *testing.T, authSession map[string]any) {
	t.Helper()
	lifecycle, ok := authSession["lifecycle"].(map[string]any)
	if !ok {
		t.Fatalf("auth_session missing lifecycle: %#v", authSession)
	}
	if lifecycle["initial_state"] != "anonymous" {
		t.Fatalf("auth lifecycle initial state = %#v", lifecycle["initial_state"])
	}
	if !stringSliceFromJSON(t, lifecycle["terminal_states"], "auth terminal states").has("expired") ||
		!stringSliceFromJSON(t, lifecycle["terminal_states"], "auth terminal states").has("revoked") {
		t.Fatalf("auth lifecycle terminal states = %#v", lifecycle["terminal_states"])
	}
	transitions, ok := lifecycle["transitions"].([]any)
	if !ok || len(transitions) < 5 {
		t.Fatalf("auth lifecycle transitions = %#v", lifecycle["transitions"])
	}
	required := map[string]bool{
		"anonymous/login_started/oauth_state_issued":                false,
		"oauth_state_issued/oauth_callback_validated/authenticated": false,
		"authenticated/logout_requested/revoked":                    false,
	}
	for _, raw := range transitions {
		transition, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("auth lifecycle transition = %#v", raw)
		}
		key := transition["from"].(string) + "/" + transition["event"].(string) + "/" + transition["to"].(string)
		if _, exists := required[key]; exists {
			required[key] = true
		}
	}
	for key, found := range required {
		if !found {
			t.Fatalf("auth lifecycle missing transition %s", key)
		}
	}
}

func assertReportRequestStateMachine(t *testing.T, sm struct {
	Capability     string   `json:"capability"`
	States         []string `json:"states"`
	InitialState   string   `json:"initial_state"`
	TerminalStates []string `json:"terminal_states"`
	Transitions    []struct {
		From         string   `json:"from"`
		Event        string   `json:"event"`
		To           string   `json:"to"`
		RequiresRole string   `json:"requires_role"`
		Writes       []string `json:"writes"`
	} `json:"transitions"`
	FixturePath string `json:"fixture_path"`
}) {
	t.Helper()
	if sm.Capability != "product.workflow.crud.report_request" || sm.InitialState != "draft" || sm.FixturePath == "" {
		t.Fatalf("report request state machine header = %#v", sm)
	}
	for _, want := range []string{"draft", "queued", "running", "artifact_ready", "completed", "failed", "cancelled", "deleted"} {
		if !contains(sm.States, want) {
			t.Fatalf("report request state machine missing state %q: %#v", want, sm.States)
		}
	}
	for _, want := range []string{"completed", "failed", "cancelled", "deleted"} {
		if !contains(sm.TerminalStates, want) {
			t.Fatalf("report request state machine missing terminal state %q: %#v", want, sm.TerminalStates)
		}
	}
	required := map[string]bool{
		"draft/submit/queued":                              false,
		"queued/worker_started/running":                    false,
		"running/artifact_manifest_written/artifact_ready": false,
		"artifact_ready/finalize/completed":                false,
	}
	for _, transition := range sm.Transitions {
		if transition.RequiresRole == "" || len(transition.Writes) == 0 {
			t.Fatalf("report request transition missing role/writes: %#v", transition)
		}
		key := transition.From + "/" + transition.Event + "/" + transition.To
		if _, exists := required[key]; exists {
			required[key] = true
		}
	}
	for key, found := range required {
		if !found {
			t.Fatalf("report request state machine missing transition %s", key)
		}
	}
}

func assertTaskEventOrderingContract(t *testing.T, key []string, sequenceColumn string, monotonic bool) {
	t.Helper()
	if !contains(key, "task_id") || !contains(key, "sequence") || sequenceColumn != "sequence" || !monotonic {
		t.Fatalf("task event ordering contract = key:%#v sequence:%q monotonic:%v", key, sequenceColumn, monotonic)
	}
}

func assertDownloadAuthorizationContract(t *testing.T, mode, sessionState, artifactState string, deniedCases []string) {
	t.Helper()
	if mode != "owner_or_admin" || sessionState != "authenticated" || artifactState != "completed" {
		t.Fatalf("download authorization contract = mode:%q session:%q artifact:%q", mode, sessionState, artifactState)
	}
	for _, want := range []string{"missing_session", "expired_session", "wrong_owner", "artifact_not_ready", "artifact_sha_mismatch"} {
		if !contains(deniedCases, want) {
			t.Fatalf("download authorization missing denied case %q: %#v", want, deniedCases)
		}
	}
}

func assertArtifactManifestContract(t *testing.T, contract struct {
	Capability          string   `json:"capability"`
	ProviderFree        bool     `json:"provider_free"`
	LiveNetwork         bool     `json:"live_network"`
	RequiresCredentials bool     `json:"requires_credentials"`
	ManifestPath        string   `json:"manifest_path"`
	RequiredFields      []string `json:"required_fields"`
	ArtifactFields      []string `json:"artifact_fields"`
	PathPolicy          struct {
		LocalFixturePrefix string `json:"local_fixture_prefix"`
		RemoteURIAllowed   bool   `json:"remote_uri_allowed"`
		SignedURLAllowed   bool   `json:"signed_url_allowed"`
		ChecksumAlgorithm  string `json:"checksum_algorithm"`
	} `json:"path_policy"`
}) {
	t.Helper()
	if contract.Capability != "product.workflow.download.artifact_manifest" || !contract.ProviderFree || contract.LiveNetwork || contract.RequiresCredentials {
		t.Fatalf("artifact manifest contract header = %#v", contract)
	}
	if !strings.Contains(contract.ManifestPath, "provider_free_fixture_index.json#") {
		t.Fatalf("artifact manifest fixture path = %q", contract.ManifestPath)
	}
	for _, want := range []string{"manifest_version", "request_id", "owner_user_id", "artifacts", "created_at"} {
		if !contains(contract.RequiredFields, want) {
			t.Fatalf("artifact manifest required fields missing %q: %#v", want, contract.RequiredFields)
		}
	}
	for _, want := range []string{"artifact_id", "kind", "media_type", "local_path", "sha256", "byte_size", "download_capability"} {
		if !contains(contract.ArtifactFields, want) {
			t.Fatalf("artifact fields missing %q: %#v", want, contract.ArtifactFields)
		}
	}
	if contract.PathPolicy.LocalFixturePrefix != "output/fixture/" || contract.PathPolicy.RemoteURIAllowed || contract.PathPolicy.SignedURLAllowed || contract.PathPolicy.ChecksumAlgorithm != "sha256" {
		t.Fatalf("artifact path policy = %#v", contract.PathPolicy)
	}
}

func assertCRUDCommandEnvelopeContract(t *testing.T, crud struct {
	Capability      string   `json:"capability"`
	Operations      []string `json:"operations"`
	Tables          []string `json:"tables"`
	CommandEnvelope struct {
		Mode                string   `json:"mode"`
		RequiresLiveNetwork bool     `json:"requires_live_network"`
		RequiresCredentials bool     `json:"requires_credentials"`
		RequiredFields      []string `json:"required_fields"`
		Operations          []struct {
			Operation    string   `json:"operation"`
			AllowedFrom  []string `json:"allowed_from"`
			AllowedTo    string   `json:"allowed_to"`
			RequiresRole string   `json:"requires_role"`
		} `json:"operations"`
		FixturePath string `json:"fixture_path"`
	} `json:"command_envelope"`
}) {
	t.Helper()
	if crud.Capability != "product.workflow.crud.report_request" {
		t.Fatalf("CRUD capability = %q", crud.Capability)
	}
	for _, want := range []string{"create", "read", "update", "delete"} {
		if !contains(crud.Operations, want) {
			t.Fatalf("CRUD operations missing %q: %#v", want, crud.Operations)
		}
	}
	envelope := crud.CommandEnvelope
	if envelope.Mode != "deterministic_fixture_command" || envelope.RequiresLiveNetwork || envelope.RequiresCredentials {
		t.Fatalf("CRUD command envelope must be deterministic/provider-free: %#v", envelope)
	}
	for _, want := range []string{"command_id", "operation", "actor_user_id", "actor_role", "idempotency_key", "expected_state", "payload", "fixture_result_id"} {
		if !contains(envelope.RequiredFields, want) {
			t.Fatalf("CRUD command envelope required fields missing %q: %#v", want, envelope.RequiredFields)
		}
	}
	if !strings.Contains(envelope.FixturePath, "provider_free_fixture_index.json#") {
		t.Fatalf("CRUD command envelope fixture path = %q", envelope.FixturePath)
	}
	if len(envelope.Operations) != 4 {
		t.Fatalf("CRUD command envelope operations = %#v", envelope.Operations)
	}
	for _, op := range envelope.Operations {
		if op.Operation == "" || len(op.AllowedFrom) == 0 || op.AllowedTo == "" || op.RequiresRole != "analyst" {
			t.Fatalf("CRUD command operation incomplete: %#v", op)
		}
	}
}

func assertApprovalBoundaryContract(t *testing.T, boundary struct {
	Capability                      string   `json:"capability"`
	ProviderFree                    bool     `json:"provider_free"`
	DefaultDecision                 string   `json:"default_decision"`
	FixturePath                     string   `json:"fixture_path"`
	RequiresExplicitCapabilityGrant bool     `json:"requires_explicit_capability_grant"`
	DeniedByDefault                 []string `json:"denied_by_default"`
	AllowedWithoutApproval          []string `json:"allowed_without_approval"`
}) {
	t.Helper()
	if boundary.Capability != "product.workflow.approval.boundary" || !boundary.ProviderFree || boundary.DefaultDecision != "deny" || !boundary.RequiresExplicitCapabilityGrant {
		t.Fatalf("approval boundary header = %#v", boundary)
	}
	if !strings.Contains(boundary.FixturePath, "provider_free_fixture_index.json#") {
		t.Fatalf("approval fixture path = %q", boundary.FixturePath)
	}
	for _, want := range []string{"live_provider_call", "live_network_fetch", "credential_read", "provider_sdk_import", "external_deploy"} {
		if !contains(boundary.DeniedByDefault, want) {
			t.Fatalf("approval denied-by-default missing %q: %#v", want, boundary.DeniedByDefault)
		}
	}
	for _, want := range []string{"read_checked_in_fixture", "render_static_template_snapshot", "replay_recorded_workflow", "validate_json_contract"} {
		if !contains(boundary.AllowedWithoutApproval, want) {
			t.Fatalf("approval allowed list missing %q: %#v", want, boundary.AllowedWithoutApproval)
		}
	}
}

func assertWorkflowReplayContract(t *testing.T, replay struct {
	Capability                string   `json:"capability"`
	ProviderFree              bool     `json:"provider_free"`
	LiveNetwork               bool     `json:"live_network"`
	RequiresCredentials       bool     `json:"requires_credentials"`
	RequiresProviderCallbacks bool     `json:"requires_provider_callbacks"`
	FixturePath               string   `json:"fixture_path"`
	Inputs                    []string `json:"inputs"`
	OrderedEventSources       []string `json:"ordered_event_sources"`
	Outputs                   []string `json:"outputs"`
	Determinism               struct {
		Clock             string `json:"clock"`
		RandomSeed        string `json:"random_seed"`
		ExternalCallbacks bool   `json:"external_callbacks"`
	} `json:"determinism"`
}) {
	t.Helper()
	if replay.Capability != "product.workflow.replay.fixture" || !replay.ProviderFree || replay.LiveNetwork || replay.RequiresCredentials || replay.RequiresProviderCallbacks {
		t.Fatalf("workflow replay header = %#v", replay)
	}
	if !strings.Contains(replay.FixturePath, "provider_free_fixture_index.json#") {
		t.Fatalf("workflow replay fixture path = %q", replay.FixturePath)
	}
	for _, want := range []string{"ticker", "sections", "session_fixture_id", "crud_command_ids"} {
		if !contains(replay.Inputs, want) {
			t.Fatalf("workflow replay inputs missing %q: %#v", want, replay.Inputs)
		}
	}
	for _, want := range []string{"report_request_lifecycle", "task_events", "download_authorization"} {
		if !contains(replay.OrderedEventSources, want) {
			t.Fatalf("workflow replay event sources missing %q: %#v", want, replay.OrderedEventSources)
		}
	}
	for _, want := range []string{"artifact_manifest", "task_history", "ui_snapshot_metadata"} {
		if !contains(replay.Outputs, want) {
			t.Fatalf("workflow replay outputs missing %q: %#v", want, replay.Outputs)
		}
	}
	if replay.Determinism.Clock != "fixture_logical_clock" || replay.Determinism.RandomSeed != "not_used" || replay.Determinism.ExternalCallbacks {
		t.Fatalf("workflow replay determinism = %#v", replay.Determinism)
	}
}

func assertProductWorkflowFixtureContracts(t *testing.T, path string) {
	t.Helper()
	var fixtures struct {
		ProviderFree bool `json:"provider_free"`
		Fixtures     struct {
			WebProduct struct {
				AuthSessions []struct {
					ID        string `json:"id"`
					State     string `json:"state"`
					CSRFValid bool   `json:"csrf_valid"`
				} `json:"auth_sessions"`
				ReportRequestLifecycle []struct {
					Sequence int    `json:"sequence"`
					From     string `json:"from"`
					Event    string `json:"event"`
					To       string `json:"to"`
				} `json:"report_request_lifecycle"`
				RouteSessionSnapshots []struct {
					Route        string `json:"route"`
					Method       string `json:"method"`
					SessionID    string `json:"session_id"`
					SessionState string `json:"session_state"`
					Role         string `json:"role"`
					Decision     string `json:"decision"`
					Reason       string `json:"reason"`
				} `json:"route_session_snapshots"`
				CRUDCommandEnvelopes []struct {
					CommandID       string         `json:"command_id"`
					Operation       string         `json:"operation"`
					ActorUserID     string         `json:"actor_user_id"`
					ActorRole       string         `json:"actor_role"`
					IdempotencyKey  string         `json:"idempotency_key"`
					ExpectedState   string         `json:"expected_state"`
					Payload         map[string]any `json:"payload"`
					FixtureResultID string         `json:"fixture_result_id"`
				} `json:"crud_command_envelopes"`
				TaskEvents []struct {
					ID       string `json:"id"`
					TaskID   string `json:"task_id"`
					Sequence int    `json:"sequence"`
					Event    string `json:"event"`
				} `json:"task_events"`
				ArtifactManifest struct {
					ManifestVersion int    `json:"manifest_version"`
					RequestID       string `json:"request_id"`
					OwnerUserID     string `json:"owner_user_id"`
					Artifacts       []struct {
						ArtifactID         string `json:"artifact_id"`
						Kind               string `json:"kind"`
						MediaType          string `json:"media_type"`
						LocalPath          string `json:"local_path"`
						SHA256             string `json:"sha256"`
						ByteSize           int    `json:"byte_size"`
						DownloadCapability string `json:"download_capability"`
					} `json:"artifacts"`
				} `json:"artifact_manifest"`
				DownloadAuthorization []struct {
					ArtifactID string `json:"artifact_id"`
					SessionID  string `json:"session_id"`
					Decision   string `json:"decision"`
					Reason     string `json:"reason"`
				} `json:"download_authorization"`
				ApprovalDecisions []struct {
					Request  string `json:"request"`
					Decision string `json:"decision"`
					Reason   string `json:"reason"`
				} `json:"approval_decisions"`
				UISnapshots struct {
					Accessibility []struct {
						Route           string   `json:"route"`
						TemplateID      string   `json:"template_id"`
						Checks          []string `json:"checks"`
						RequiresBrowser bool     `json:"requires_browser"`
					} `json:"accessibility"`
					AccessibilityGateResults []struct {
						Route              string   `json:"route"`
						TemplateID         string   `json:"template_id"`
						Decision           string   `json:"decision"`
						BlockingViolations []string `json:"blocking_violations"`
						RequiresBrowser    bool     `json:"requires_browser"`
					} `json:"accessibility_gate_results"`
					Visual []struct {
						Route               string `json:"route"`
						TemplateID          string `json:"template_id"`
						SnapshotID          string `json:"snapshot_id"`
						ViewportID          string `json:"viewport_id"`
						FixtureDatasetID    string `json:"fixture_dataset_id"`
						TemplateSHA256      string `json:"template_sha256"`
						AssetManifestSHA256 string `json:"asset_manifest_sha256"`
						RequiresBrowser     bool   `json:"requires_browser"`
					} `json:"visual"`
				} `json:"ui_snapshots"`
			} `json:"web_product"`
			WorkflowReplay struct {
				ID                  string `json:"id"`
				ProviderFree        bool   `json:"provider_free"`
				LiveNetwork         bool   `json:"live_network"`
				RequiresCredentials bool   `json:"requires_credentials"`
				Inputs              struct {
					Ticker           string   `json:"ticker"`
					Sections         []string `json:"sections"`
					SessionFixtureID string   `json:"session_fixture_id"`
					CRUDCommandIDs   []string `json:"crud_command_ids"`
				} `json:"inputs"`
				OrderedEvents []string          `json:"ordered_events"`
				Outputs       map[string]string `json:"outputs"`
			} `json:"workflow_replay"`
		} `json:"fixtures"`
		DBSeed struct {
			SchemaVersions []struct {
				Version     int    `json:"version"`
				MigrationID string `json:"migration_id"`
				Checksum    string `json:"checksum"`
			} `json:"schema_versions"`
			SchemaRollbacks []struct {
				FromVersion int    `json:"from_version"`
				ToVersion   int    `json:"to_version"`
				MigrationID string `json:"migration_id"`
				Checksum    string `json:"checksum"`
			} `json:"schema_rollbacks"`
			RollbackFixtures map[string]struct {
				FromVersion int `json:"from_version"`
				ToVersion   int `json:"to_version"`
			} `json:"rollback_fixtures"`
		} `json:"db_seed"`
	}
	decodeJSONFile(t, path, &fixtures)
	if !fixtures.ProviderFree {
		t.Fatal("fixtures must remain provider-free")
	}
	if len(fixtures.Fixtures.WebProduct.AuthSessions) < 3 {
		t.Fatalf("auth session fixtures = %#v", fixtures.Fixtures.WebProduct.AuthSessions)
	}
	hasExpired := false
	for _, session := range fixtures.Fixtures.WebProduct.AuthSessions {
		if session.State == "expired" && !session.CSRFValid {
			hasExpired = true
		}
	}
	if !hasExpired {
		t.Fatalf("auth session fixtures missing expired denial case: %#v", fixtures.Fixtures.WebProduct.AuthSessions)
	}
	assertMonotonicSequences(t, "report request lifecycle", lifecycleSequences(fixtures.Fixtures.WebProduct.ReportRequestLifecycle))
	hasRouteAllow, hasRouteDeny := false, false
	for _, snapshot := range fixtures.Fixtures.WebProduct.RouteSessionSnapshots {
		if snapshot.Route == "" || snapshot.Method == "" || snapshot.SessionID == "" || snapshot.SessionState == "" || snapshot.Role == "" || snapshot.Decision == "" {
			t.Fatalf("route/session snapshot incomplete: %#v", snapshot)
		}
		hasRouteAllow = hasRouteAllow || snapshot.Decision == "allow"
		hasRouteDeny = hasRouteDeny || snapshot.Decision == "deny" && snapshot.Reason != ""
	}
	if !hasRouteAllow || !hasRouteDeny {
		t.Fatalf("route/session snapshots need allow and deny cases: %#v", fixtures.Fixtures.WebProduct.RouteSessionSnapshots)
	}
	for _, command := range fixtures.Fixtures.WebProduct.CRUDCommandEnvelopes {
		if command.CommandID == "" || command.Operation == "" || command.ActorUserID == "" || command.ActorRole == "" || command.IdempotencyKey == "" || command.ExpectedState == "" || command.FixtureResultID == "" || len(command.Payload) == 0 {
			t.Fatalf("CRUD command envelope fixture incomplete: %#v", command)
		}
	}
	if len(fixtures.Fixtures.WebProduct.CRUDCommandEnvelopes) < 3 {
		t.Fatalf("CRUD command envelope fixtures too shallow: %#v", fixtures.Fixtures.WebProduct.CRUDCommandEnvelopes)
	}
	assertMonotonicSequences(t, "task events", taskEventSequences(fixtures.Fixtures.WebProduct.TaskEvents))
	if fixtures.Fixtures.WebProduct.ArtifactManifest.ManifestVersion != 1 || fixtures.Fixtures.WebProduct.ArtifactManifest.RequestID == "" || fixtures.Fixtures.WebProduct.ArtifactManifest.OwnerUserID == "" {
		t.Fatalf("artifact manifest fixture header = %#v", fixtures.Fixtures.WebProduct.ArtifactManifest)
	}
	if len(fixtures.Fixtures.WebProduct.ArtifactManifest.Artifacts) != 4 {
		t.Fatalf("artifact manifest items = %#v", fixtures.Fixtures.WebProduct.ArtifactManifest.Artifacts)
	}
	for _, artifact := range fixtures.Fixtures.WebProduct.ArtifactManifest.Artifacts {
		if artifact.ArtifactID == "" || artifact.Kind == "" || artifact.MediaType == "" || artifact.SHA256 == "" || artifact.ByteSize <= 0 || artifact.DownloadCapability == "" {
			t.Fatalf("artifact manifest item incomplete: %#v", artifact)
		}
		if !strings.HasPrefix(artifact.LocalPath, "output/fixture/") || strings.Contains(artifact.LocalPath, "://") {
			t.Fatalf("artifact manifest path must be local fixture path: %#v", artifact)
		}
	}
	hasAllow, hasDeny := false, false
	for _, authz := range fixtures.Fixtures.WebProduct.DownloadAuthorization {
		hasAllow = hasAllow || authz.Decision == "allow"
		hasDeny = hasDeny || authz.Decision == "deny"
		if authz.ArtifactID == "" || authz.SessionID == "" || authz.Reason == "" {
			t.Fatalf("download authorization fixture incomplete: %#v", authz)
		}
	}
	if !hasAllow || !hasDeny {
		t.Fatalf("download authorization fixtures need allow and deny cases: %#v", fixtures.Fixtures.WebProduct.DownloadAuthorization)
	}
	hasApprovalAllow, hasApprovalDeny := false, false
	for _, decision := range fixtures.Fixtures.WebProduct.ApprovalDecisions {
		if decision.Request == "" || decision.Decision == "" || decision.Reason == "" {
			t.Fatalf("approval decision fixture incomplete: %#v", decision)
		}
		hasApprovalAllow = hasApprovalAllow || decision.Decision == "allow"
		hasApprovalDeny = hasApprovalDeny || decision.Decision == "deny"
	}
	if !hasApprovalAllow || !hasApprovalDeny {
		t.Fatalf("approval fixtures need allow and deny cases: %#v", fixtures.Fixtures.WebProduct.ApprovalDecisions)
	}
	if len(fixtures.Fixtures.WebProduct.UISnapshots.Accessibility) < 3 || len(fixtures.Fixtures.WebProduct.UISnapshots.Visual) < 3 {
		t.Fatalf("UI snapshot fixtures too shallow: %#v", fixtures.Fixtures.WebProduct.UISnapshots)
	}
	for _, snapshot := range fixtures.Fixtures.WebProduct.UISnapshots.Accessibility {
		if snapshot.Route == "" || snapshot.TemplateID == "" || len(snapshot.Checks) == 0 || snapshot.RequiresBrowser {
			t.Fatalf("accessibility snapshot fixture incomplete or browser-bound: %#v", snapshot)
		}
	}
	if len(fixtures.Fixtures.WebProduct.UISnapshots.AccessibilityGateResults) < 3 {
		t.Fatalf("accessibility gate result fixtures too shallow: %#v", fixtures.Fixtures.WebProduct.UISnapshots.AccessibilityGateResults)
	}
	for _, result := range fixtures.Fixtures.WebProduct.UISnapshots.AccessibilityGateResults {
		if result.Route == "" || result.TemplateID == "" || result.Decision != "pass" || len(result.BlockingViolations) != 0 || result.RequiresBrowser {
			t.Fatalf("accessibility gate result fixture must be a browserless pass with no blocking violations: %#v", result)
		}
	}
	for _, snapshot := range fixtures.Fixtures.WebProduct.UISnapshots.Visual {
		if snapshot.Route == "" || snapshot.TemplateID == "" || snapshot.SnapshotID == "" || snapshot.ViewportID == "" || snapshot.FixtureDatasetID == "" || snapshot.TemplateSHA256 == "" || snapshot.AssetManifestSHA256 == "" || snapshot.RequiresBrowser {
			t.Fatalf("visual snapshot fixture incomplete or browser-bound: %#v", snapshot)
		}
	}
	lastVersion := 0
	for _, version := range fixtures.DBSeed.SchemaVersions {
		if version.Version <= lastVersion || version.MigrationID == "" || version.Checksum == "" {
			t.Fatalf("schema version fixture not strictly increasing or incomplete: %#v", fixtures.DBSeed.SchemaVersions)
		}
		lastVersion = version.Version
	}
	for _, want := range []string{"001_product_workflow_base_down", "002_request_artifact_authorization_down"} {
		if _, ok := fixtures.DBSeed.RollbackFixtures[want]; !ok {
			t.Fatalf("missing rollback fixture %q: %#v", want, fixtures.DBSeed.RollbackFixtures)
		}
	}
	if len(fixtures.DBSeed.SchemaRollbacks) < 2 {
		t.Fatalf("schema rollback fixtures = %#v", fixtures.DBSeed.SchemaRollbacks)
	}
	replay := fixtures.Fixtures.WorkflowReplay
	if replay.ID == "" || !replay.ProviderFree || replay.LiveNetwork || replay.RequiresCredentials {
		t.Fatalf("workflow replay fixture header = %#v", replay)
	}
	if replay.Inputs.Ticker == "" || len(replay.Inputs.Sections) == 0 || replay.Inputs.SessionFixtureID == "" || len(replay.Inputs.CRUDCommandIDs) == 0 {
		t.Fatalf("workflow replay inputs incomplete: %#v", replay.Inputs)
	}
	if len(replay.OrderedEvents) < 8 {
		t.Fatalf("workflow replay ordered events too shallow: %#v", replay.OrderedEvents)
	}
	for _, want := range []string{"artifact_manifest", "task_history", "ui_snapshot_metadata"} {
		if !strings.Contains(replay.Outputs[want], "provider_free_fixture_index.json#") {
			t.Fatalf("workflow replay output %q must point at fixture index: %#v", want, replay.Outputs)
		}
	}
}

type jsonStringSlice []string

func (values jsonStringSlice) has(want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func stringSliceFromJSON(t *testing.T, value any, label string) jsonStringSlice {
	t.Helper()
	rawValues, ok := value.([]any)
	if !ok {
		t.Fatalf("%s = %#v, want string array", label, value)
	}
	values := make([]string, 0, len(rawValues))
	for _, raw := range rawValues {
		value, ok := raw.(string)
		if !ok {
			t.Fatalf("%s contains non-string value %#v", label, raw)
		}
		values = append(values, value)
	}
	return values
}

func lifecycleSequences(values []struct {
	Sequence int    `json:"sequence"`
	From     string `json:"from"`
	Event    string `json:"event"`
	To       string `json:"to"`
}) []int {
	sequences := make([]int, 0, len(values))
	for _, value := range values {
		sequences = append(sequences, value.Sequence)
	}
	return sequences
}

func taskEventSequences(values []struct {
	ID       string `json:"id"`
	TaskID   string `json:"task_id"`
	Sequence int    `json:"sequence"`
	Event    string `json:"event"`
}) []int {
	sequences := make([]int, 0, len(values))
	for _, value := range values {
		sequences = append(sequences, value.Sequence)
	}
	return sequences
}

func assertMonotonicSequences(t *testing.T, label string, sequences []int) {
	t.Helper()
	previous := 0
	for _, sequence := range sequences {
		if sequence <= previous {
			t.Fatalf("%s sequences are not strictly increasing: %#v", label, sequences)
		}
		previous = sequence
	}
}

func loadProductWorkflowLiveManifest(t *testing.T, base string) finrobotProductWorkflowLiveManifest {
	t.Helper()
	var manifest finrobotProductWorkflowLiveManifest
	decodeJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func productWorkflowLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "product_workflow")
}

func finrobotProductWorkflowRegistryExamples(t *testing.T, root string) []finrobotAggregateExample {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", "run", "./cmd/leia", "examples", "list", "--json")
	cmd.Dir = root
	cmd.Env = withoutLLMProviderEnv(os.Environ())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("examples list failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	var payload struct {
		SchemaVersion int                        `json:"schema_version"`
		Examples      []finrobotAggregateExample `json:"examples"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode examples list: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != 1 {
		t.Fatalf("examples list schema_version = %d, want 1", payload.SchemaVersion)
	}
	return payload.Examples
}

type productWorkflowExamplesCheckReport struct {
	SchemaVersion int                                  `json:"schema_version"`
	OK            bool                                 `json:"ok"`
	Runnable      int                                  `json:"runnable"`
	Skipped       int                                  `json:"skipped"`
	Failed        int                                  `json:"failed"`
	Results       []productWorkflowExamplesCheckResult `json:"results"`
}

type productWorkflowExamplesCheckResult struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Status   string `json:"status"`
	Requires string `json:"requires"`
	Error    string `json:"error"`
}

func finrobotProductWorkflowExamplesCheck(t *testing.T, root, selector string) productWorkflowExamplesCheckReport {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(
		"go", "run", "./cmd/leia",
		"examples", "check",
		"--json",
		"--timeout=30s",
		selector,
	)
	cmd.Dir = root
	cmd.Env = withoutLLMProviderEnv(os.Environ())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("examples check failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	var payload productWorkflowExamplesCheckReport
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode examples check: %v\n%s", err, stdout.String())
	}
	return payload
}

func assertJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeJSONFile(t, path, &value)
}

func decodeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
