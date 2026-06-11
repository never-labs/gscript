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
		History     map[string]any `json:"history"`
		TaskLog     map[string]any `json:"task_log"`
		Download    map[string]any `json:"download"`
		CRUD        map[string]any `json:"crud"`
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
		"task_log":     workflow.TaskLog,
		"download":     workflow.Download,
		"crud":         workflow.CRUD,
	} {
		if block["capability"] == "" {
			t.Fatalf("%s missing capability: %#v", name, block)
		}
	}

	var migration struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		Capability   string `json:"capability"`
		Migrations   []struct {
			Tables []struct {
				Name    string   `json:"name"`
				Columns []string `json:"columns"`
			} `json:"tables"`
			RollbackRequired bool `json:"rollback_required"`
		} `json:"migrations"`
	}
	decodeJSONFile(t, filepath.Join(base, "contracts", "db_migration_contract.json"), &migration)
	if !migration.ProviderFree || migration.LiveNetwork || migration.Capability != "product.workflow.db.migration.plan" {
		t.Fatalf("migration contract header = %#v", migration)
	}
	if len(migration.Migrations) != 1 || !migration.Migrations[0].RollbackRequired {
		t.Fatalf("migration contract migrations = %#v", migration.Migrations)
	}
	wantTables := []string{"users", "sessions", "report_requests", "reports", "artifacts", "task_events", "audit_events", "schema_versions"}
	for _, want := range wantTables {
		found := false
		for _, table := range migration.Migrations[0].Tables {
			if table.Name != want {
				continue
			}
			if contains(table.Columns, "id") || want == "schema_versions" && contains(table.Columns, "version") {
				found = true
			}
		}
		if !found {
			t.Fatalf("migration missing table %q in %#v", want, migration.Migrations[0].Tables)
		}
	}

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
