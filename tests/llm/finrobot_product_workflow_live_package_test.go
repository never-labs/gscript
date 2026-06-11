package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type finrobotProductWorkflowLiveManifest struct {
	SchemaVersion             int               `json:"schema_version"`
	ID                        string            `json:"id"`
	PackageName               string            `json:"package_name"`
	SourceExamples            []string          `json:"source_examples"`
	ProviderFree              bool              `json:"provider_free"`
	LiveNetworkDefault        bool              `json:"live_network_default"`
	Credentials               credentialsBlock  `json:"credentials"`
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

	for _, source := range manifest.SourceExamples {
		if _, err := os.Stat(filepath.Join(repoRoot(t), source)); err != nil {
			t.Fatalf("source example %q: %v", source, err)
		}
	}
	for _, key := range []string{"equity_cli_workflow", "web_product", "db_migrations", "deployment_capability_gates"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"workflow_run", "web_product", "db_migration", "deployment_gate"} {
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
		"product.workflow.auth.session",
		"product.workflow.history.read",
		"product.workflow.task_log.read",
		"product.workflow.download.artifact",
		"product.workflow.crud.report_request",
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
	for _, want := range []string{"provider_free", "credentials", "equity cli", "web product", "capability gates"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
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
		CRUD map[string]any `json:"crud"`
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
		"crud":         workflow.CRUD,
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
	assertProductWorkflowLifecycleContract(t, workflow.AuthSession)
	assertReportRequestStateMachine(t, workflow.RequestSM)
	assertTaskEventOrderingContract(t, workflow.TaskLog.Ordering.Key, workflow.TaskLog.Ordering.SequenceColumn, workflow.TaskLog.Ordering.MonotonicPerTask)
	assertDownloadAuthorizationContract(t, workflow.Download.Authorization.Mode, workflow.Download.Authorization.RequiresSessionState, workflow.Download.Authorization.RequiresArtifactState, workflow.Download.Authorization.DeniedCases)

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
				TaskEvents []struct {
					ID       string `json:"id"`
					TaskID   string `json:"task_id"`
					Sequence int    `json:"sequence"`
					Event    string `json:"event"`
				} `json:"task_events"`
				DownloadAuthorization []struct {
					ArtifactID string `json:"artifact_id"`
					SessionID  string `json:"session_id"`
					Decision   string `json:"decision"`
					Reason     string `json:"reason"`
				} `json:"download_authorization"`
			} `json:"web_product"`
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
	assertMonotonicSequences(t, "task events", taskEventSequences(fixtures.Fixtures.WebProduct.TaskEvents))
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
