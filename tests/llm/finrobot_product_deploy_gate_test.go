package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type finrobotProductDeployParityLedger struct {
	SchemaVersion          int                 `json:"schema_version"`
	ID                     string              `json:"id"`
	ProviderFree           bool                `json:"provider_free"`
	LiveNetworkDefault     bool                `json:"live_network_default"`
	CleanSkipDefault       bool                `json:"clean_skip_default"`
	PackageDeployFiles     map[string]string   `json:"package_deploy_files"`
	ProductWorkflowSources map[string]string   `json:"product_workflow_sources"`
	ParityEntries          []deployParityEntry `json:"parity_entries"`
	EnvGates               []struct {
		ID                  string `json:"id"`
		Env                 string `json:"env"`
		Path                string `json:"path"`
		Required            bool   `json:"required"`
		Redact              bool   `json:"redact"`
		CleanSkipWhenAbsent bool   `json:"clean_skip_when_absent"`
		ProviderFree        bool   `json:"provider_free"`
	} `json:"env_gates"`
	ServerMetadata struct {
		RecommendedServer     string   `json:"recommended_server"`
		RecommendedDependency string   `json:"recommended_dependency"`
		FallbackServer        string   `json:"fallback_server"`
		Entrypoint            string   `json:"entrypoint"`
		BindEnv               string   `json:"bind_env"`
		DefaultPort           string   `json:"default_port"`
		HealthRoutes          []string `json:"health_routes"`
		ProviderFree          bool     `json:"provider_free"`
		LiveNetworkDefault    bool     `json:"live_network_default"`
	} `json:"server_metadata"`
	StaticAssets struct {
		Source         string `json:"source"`
		Capability     string `json:"capability"`
		RequiredPolicy struct {
			ExternalFetch bool   `json:"external_fetch"`
			RemoteFonts   bool   `json:"remote_fonts"`
			InlineOnly    bool   `json:"inline_only"`
			HashAlgorithm string `json:"hash_algorithm"`
		} `json:"required_policy"`
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
	} `json:"static_assets"`
	AuthSessionGates struct {
		Source              string   `json:"source"`
		Capability          string   `json:"capability"`
		Mode                string   `json:"mode"`
		RequiredStates      []string `json:"required_states"`
		RequiredDeniedCases []string `json:"required_denied_cases"`
		ProviderFree        bool     `json:"provider_free"`
		LiveNetwork         bool     `json:"live_network"`
	} `json:"auth_session_gates"`
	DBMigrationGates struct {
		Source           string   `json:"source"`
		Capability       string   `json:"capability"`
		MigrationEngine  string   `json:"migration_engine"`
		RequiredTables   []string `json:"required_tables"`
		RollbackRequired bool     `json:"rollback_required"`
		ProviderFree     bool     `json:"provider_free"`
		LiveNetwork      bool     `json:"live_network"`
	} `json:"db_migration_gates"`
	CleanSkips []struct {
		ID             string `json:"id"`
		Status         string `json:"status"`
		NetworkAllowed bool   `json:"network_allowed"`
	} `json:"clean_skips"`
	NoNetworkDefault map[string]bool `json:"no_network_default"`
}

type deployParityEntry struct {
	ID                  string   `json:"id"`
	DeploySurface       string   `json:"deploy_surface"`
	PackageFile         string   `json:"package_file"`
	ProductCapabilities []string `json:"product_capabilities"`
	RequiredEvidence    []string `json:"required_evidence"`
	ProviderFree        bool     `json:"provider_free"`
	LiveNetwork         bool     `json:"live_network"`
	CleanSkip           bool     `json:"clean_skip"`
	SkipReason          string   `json:"skip_reason"`
}

func TestFinRobotProductDeployParityLedgerCoversPackageDeployFiles(t *testing.T) {
	base := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation")
	parityBase := filepath.Join(base, "product_deploy_parity")
	ledger := loadFinRobotProductDeployParityLedger(t, base)

	if ledger.SchemaVersion != 1 || ledger.ID != "finrobot-package-product-deploy-parity-ledger" {
		t.Fatalf("ledger header = schema %d id %q", ledger.SchemaVersion, ledger.ID)
	}
	if !ledger.ProviderFree || ledger.LiveNetworkDefault || !ledger.CleanSkipDefault {
		t.Fatalf("ledger defaults = provider_free:%v live_network:%v clean_skip:%v", ledger.ProviderFree, ledger.LiveNetworkDefault, ledger.CleanSkipDefault)
	}

	for _, key := range []string{"requirements", "setup", "dockerfile", "run_web_app", "gcloud", "manifest"} {
		name := ledger.PackageDeployFiles[key]
		if name == "" {
			t.Fatalf("ledger missing package_deploy_files.%s", key)
		}
		assertDeployGateReadable(t, parityBase, name)
	}
	for _, key := range []string{"manifest", "deployment_gates", "product_contract", "db_migration_contract", "static_asset_manifest"} {
		name := ledger.ProductWorkflowSources[key]
		if name == "" {
			t.Fatalf("ledger missing product_workflow_sources.%s", key)
		}
		assertDeployGateReadable(t, parityBase, name)
	}

	packageManifest := loadFinRobotPackageDeployManifest(t, base)
	for _, key := range []string{"requirements", "setup", "dockerfile", "run_web_app", "gcloud"} {
		if ledger.PackageDeployFiles[key] != "../"+packageManifest.Files[key] {
			t.Fatalf("ledger package file %s = %q, manifest has %q", key, ledger.PackageDeployFiles[key], packageManifest.Files[key])
		}
	}

	entries := map[string]deployParityEntry{}
	for _, entry := range ledger.ParityEntries {
		entries[entry.ID] = entry
		if !entry.ProviderFree || entry.LiveNetwork {
			t.Fatalf("parity entry %s must be provider-free/no-network: %#v", entry.ID, entry)
		}
		if entry.CleanSkip && entry.SkipReason == "" {
			t.Fatalf("clean-skip entry %s must explain skip reason", entry.ID)
		}
		data := readDeployGateText(t, filepath.Join(base, "product_deploy_parity"), entry.PackageFile)
		for _, want := range entry.RequiredEvidence {
			if !strings.Contains(data, want) {
				t.Fatalf("parity entry %s file %s missing evidence %q", entry.ID, entry.PackageFile, want)
			}
		}
		for _, capability := range entry.ProductCapabilities {
			if !strings.HasPrefix(capability, "product.workflow.") {
				t.Fatalf("parity entry %s capability = %q", entry.ID, capability)
			}
		}
	}
	for _, id := range []string{"dockerfile_fixture_service", "setup_requirements_extras", "requirements_extra_index", "run_web_app_env_health", "gcloud_deploy_gate"} {
		if entries[id].ID == "" {
			t.Fatalf("missing parity entry %q", id)
		}
	}
}

func TestFinRobotProductDeployGateCoverage(t *testing.T) {
	base := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation")
	ledger := loadFinRobotProductDeployParityLedger(t, base)

	envGates := map[string]bool{}
	for _, gate := range ledger.EnvGates {
		envGates[gate.ID] = true
		if !gate.ProviderFree {
			t.Fatalf("env gate %s must be provider-free: %#v", gate.ID, gate)
		}
		if strings.Contains(gate.Env, "API_KEY") && (!gate.Redact || !gate.CleanSkipWhenAbsent || gate.Required) {
			t.Fatalf("API key env gate must redact and clean-skip when absent: %#v", gate)
		}
	}
	for _, id := range []string{"data_dir_required", "replay_manifest_required", "live_provider_keys_optional"} {
		if !envGates[id] {
			t.Fatalf("missing env gate %q", id)
		}
	}

	if ledger.ServerMetadata.RecommendedServer != "gunicorn" ||
		ledger.ServerMetadata.RecommendedDependency != "gunicorn>=22.0" ||
		ledger.ServerMetadata.FallbackServer != "ThreadingHTTPServer" ||
		ledger.ServerMetadata.Entrypoint != "package_deploy_run_web_app:main" ||
		ledger.ServerMetadata.BindEnv != "PORT" ||
		ledger.ServerMetadata.DefaultPort != "8080" ||
		!ledger.ServerMetadata.ProviderFree ||
		ledger.ServerMetadata.LiveNetworkDefault {
		t.Fatalf("server metadata = %#v", ledger.ServerMetadata)
	}
	for _, route := range []string{"/", "/healthz"} {
		if !contains(ledger.ServerMetadata.HealthRoutes, route) {
			t.Fatalf("server metadata missing health route %q: %#v", route, ledger.ServerMetadata.HealthRoutes)
		}
	}

	setupData := readDeployGateText(t, base, "package_deploy_setup.py")
	runWebAppData := readDeployGateText(t, base, "package_deploy_run_web_app.py")
	if !strings.Contains(setupData, `"gunicorn>=22.0"`) {
		t.Fatal("setup metadata must keep gunicorn in the web extra")
	}
	for _, want := range []string{"ThreadingHTTPServer", "PORT", `("/", "/healthz")`, "check_environment"} {
		if !strings.Contains(runWebAppData, want) {
			t.Fatalf("run_web_app missing server/env evidence %q", want)
		}
	}

	var staticAssets struct {
		ProviderFree        bool `json:"provider_free"`
		LiveNetwork         bool `json:"live_network"`
		RequiresCredentials bool `json:"requires_credentials"`
		AssetPolicy         struct {
			ExternalFetch bool   `json:"external_fetch"`
			RemoteFonts   bool   `json:"remote_fonts"`
			InlineOnly    bool   `json:"inline_only"`
			HashAlgorithm string `json:"hash_algorithm"`
		} `json:"asset_policy"`
		Assets []struct {
			ID            string   `json:"id"`
			FixtureSHA256 string   `json:"fixture_sha256"`
			UsedBy        []string `json:"used_by_templates"`
		} `json:"assets"`
	}
	decodeDeployGateJSON(t, filepath.Join(base, "live_packages", "product_workflow", "templates", "static_asset_manifest.json"), &staticAssets)
	if !staticAssets.ProviderFree || staticAssets.LiveNetwork || staticAssets.RequiresCredentials {
		t.Fatalf("static asset manifest must be provider-free/no-network/no-credentials: %#v", staticAssets)
	}
	if staticAssets.AssetPolicy.ExternalFetch != ledger.StaticAssets.RequiredPolicy.ExternalFetch ||
		staticAssets.AssetPolicy.RemoteFonts != ledger.StaticAssets.RequiredPolicy.RemoteFonts ||
		staticAssets.AssetPolicy.InlineOnly != ledger.StaticAssets.RequiredPolicy.InlineOnly ||
		staticAssets.AssetPolicy.HashAlgorithm != ledger.StaticAssets.RequiredPolicy.HashAlgorithm {
		t.Fatalf("static asset policy mismatch: manifest=%#v ledger=%#v", staticAssets.AssetPolicy, ledger.StaticAssets.RequiredPolicy)
	}
	if len(staticAssets.Assets) < 3 {
		t.Fatalf("static assets = %#v", staticAssets.Assets)
	}
	for _, asset := range staticAssets.Assets {
		if asset.FixtureSHA256 == "" || len(asset.UsedBy) == 0 {
			t.Fatalf("static asset missing hash/template usage: %#v", asset)
		}
	}

	var cleanSkipIDs []string
	for _, skip := range ledger.CleanSkips {
		cleanSkipIDs = append(cleanSkipIDs, skip.ID)
		if skip.Status != "clean_skip" || skip.NetworkAllowed {
			t.Fatalf("clean skip must be explicit and network-denied: %#v", skip)
		}
	}
	for _, id := range []string{"missing_live_provider_keys", "cloud_deploy_without_external_capability", "database_without_live_migration_engine"} {
		if !contains(cleanSkipIDs, id) {
			t.Fatalf("missing clean skip %q: %#v", id, cleanSkipIDs)
		}
	}

	for _, key := range []string{"package_manifest", "product_workflow_manifest", "deployment_capability_gates", "static_asset_manifest", "db_migration_contract"} {
		if !ledger.NoNetworkDefault[key] {
			t.Fatalf("no_network_default missing %q: %#v", key, ledger.NoNetworkDefault)
		}
	}
}

func TestFinRobotProductDeployParityReflectsProductWorkflowDefaultPolicy(t *testing.T) {
	base := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation")
	productWorkflowBase := filepath.Join(base, "live_packages", "product_workflow")
	ledger := loadFinRobotProductDeployParityLedger(t, base)
	workflowManifest := loadProductWorkflowLiveManifest(t, productWorkflowBase)
	var packageManifest struct {
		ProductDeployParity struct {
			ProviderFree       bool `json:"provider_free"`
			LiveNetworkDefault bool `json:"live_network_default"`
			CleanSkipDefault   bool `json:"clean_skip_default"`
		} `json:"product_deploy_parity"`
	}
	decodeDeployGateJSON(t, filepath.Join(base, "package_deploy_manifest.json"), &packageManifest)

	if workflowManifest.DefaultPolicy.Mode != "fixture_replay" {
		t.Fatalf("product workflow default_policy.mode = %q, want fixture_replay", workflowManifest.DefaultPolicy.Mode)
	}
	if workflowManifest.DefaultPolicy.LiveNetwork != ledger.LiveNetworkDefault ||
		workflowManifest.DefaultPolicy.LiveNetwork != packageManifest.ProductDeployParity.LiveNetworkDefault ||
		workflowManifest.LiveNetworkDefault != ledger.LiveNetworkDefault ||
		workflowManifest.LiveNetworkDefault != packageManifest.ProductDeployParity.LiveNetworkDefault {
		t.Fatalf("live-network defaults drifted: workflow policy=%v workflow default=%v ledger=%v package manifest=%v",
			workflowManifest.DefaultPolicy.LiveNetwork,
			workflowManifest.LiveNetworkDefault,
			ledger.LiveNetworkDefault,
			packageManifest.ProductDeployParity.LiveNetworkDefault)
	}
	if workflowManifest.ProviderFree != ledger.ProviderFree ||
		workflowManifest.ProviderFree != packageManifest.ProductDeployParity.ProviderFree {
		t.Fatalf("provider-free defaults drifted: workflow=%v ledger=%v package manifest=%v",
			workflowManifest.ProviderFree,
			ledger.ProviderFree,
			packageManifest.ProductDeployParity.ProviderFree)
	}
	if workflowManifest.DefaultPolicy.ProviderCredentialsRequired ||
		workflowManifest.DefaultPolicy.RealDependencyImports ||
		!workflowManifest.DefaultPolicy.CleanSkipWithoutDependency ||
		!ledger.CleanSkipDefault ||
		!packageManifest.ProductDeployParity.CleanSkipDefault {
		t.Fatalf("default policy is not reflected in deploy clean-skip gates: workflow=%#v ledger_clean_skip=%v package_clean_skip=%v",
			workflowManifest.DefaultPolicy,
			ledger.CleanSkipDefault,
			packageManifest.ProductDeployParity.CleanSkipDefault)
	}

	var gates struct {
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
		Gates        []struct {
			ID                  string `json:"id"`
			RequiresCredentials bool   `json:"requires_credentials"`
			LiveNetwork         bool   `json:"live_network"`
		} `json:"gates"`
	}
	decodeDeployGateJSON(t, filepath.Join(productWorkflowBase, "contracts", "deployment_capability_gates.json"), &gates)
	if gates.ProviderFree != workflowManifest.ProviderFree || gates.LiveNetwork != workflowManifest.DefaultPolicy.LiveNetwork {
		t.Fatalf("deployment gate defaults drifted from workflow policy: gates=%#v workflow=%#v", gates, workflowManifest.DefaultPolicy)
	}
	for _, gate := range gates.Gates {
		if gate.LiveNetwork != workflowManifest.DefaultPolicy.LiveNetwork || gate.RequiresCredentials != workflowManifest.DefaultPolicy.ProviderCredentialsRequired {
			t.Fatalf("deployment gate %s does not reflect workflow default_policy: gate=%#v workflow=%#v", gate.ID, gate, workflowManifest.DefaultPolicy)
		}
	}
}

func TestFinRobotProductDeployAuthSessionAndDBMigrationGates(t *testing.T) {
	base := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation")
	ledger := loadFinRobotProductDeployParityLedger(t, base)

	var workflow struct {
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
		AuthSession  struct {
			Capability string `json:"capability"`
			Mode       string `json:"mode"`
			Lifecycle  struct {
				States []string `json:"states"`
			} `json:"lifecycle"`
			DeniedCases []string `json:"denied_cases"`
		} `json:"auth_session"`
	}
	decodeDeployGateJSON(t, filepath.Join(base, "live_packages", "product_workflow", "contracts", "product_workflow_contract.json"), &workflow)
	if !workflow.ProviderFree || workflow.LiveNetwork {
		t.Fatalf("product workflow contract must be provider-free/no-network: %#v", workflow)
	}
	if workflow.AuthSession.Capability != ledger.AuthSessionGates.Capability || workflow.AuthSession.Mode != ledger.AuthSessionGates.Mode {
		t.Fatalf("auth/session gate mismatch: workflow=%#v ledger=%#v", workflow.AuthSession, ledger.AuthSessionGates)
	}
	for _, state := range ledger.AuthSessionGates.RequiredStates {
		if !contains(workflow.AuthSession.Lifecycle.States, state) {
			t.Fatalf("auth/session lifecycle missing state %q: %#v", state, workflow.AuthSession.Lifecycle.States)
		}
	}
	for _, denied := range ledger.AuthSessionGates.RequiredDeniedCases {
		if !contains(workflow.AuthSession.DeniedCases, denied) {
			t.Fatalf("auth/session denied cases missing %q: %#v", denied, workflow.AuthSession.DeniedCases)
		}
	}

	var migration struct {
		ProviderFree    bool   `json:"provider_free"`
		LiveNetwork     bool   `json:"live_network"`
		Capability      string `json:"capability"`
		MigrationEngine string `json:"migration_engine"`
		Migrations      []struct {
			ID               string `json:"id"`
			RollbackRequired bool   `json:"rollback_required"`
			Tables           []struct {
				Name string `json:"name"`
			} `json:"tables"`
		} `json:"migrations"`
	}
	decodeDeployGateJSON(t, filepath.Join(base, "live_packages", "product_workflow", "contracts", "db_migration_contract.json"), &migration)
	if !migration.ProviderFree || migration.LiveNetwork ||
		migration.Capability != ledger.DBMigrationGates.Capability ||
		migration.MigrationEngine != ledger.DBMigrationGates.MigrationEngine {
		t.Fatalf("db migration gate mismatch: migration=%#v ledger=%#v", migration, ledger.DBMigrationGates)
	}

	tables := map[string]bool{}
	for _, step := range migration.Migrations {
		if ledger.DBMigrationGates.RollbackRequired && !step.RollbackRequired {
			t.Fatalf("migration step %s must declare rollback", step.ID)
		}
		for _, table := range step.Tables {
			tables[table.Name] = true
		}
	}
	for _, table := range ledger.DBMigrationGates.RequiredTables {
		if !tables[table] {
			t.Fatalf("db migration gate missing table %q: %#v", table, tables)
		}
	}
}

func loadFinRobotProductDeployParityLedger(t *testing.T, base string) finrobotProductDeployParityLedger {
	t.Helper()
	var ledger finrobotProductDeployParityLedger
	decodeDeployGateJSON(t, filepath.Join(base, "product_deploy_parity", "package_deploy_parity_ledger.json"), &ledger)
	return ledger
}

func decodeDeployGateJSON(t *testing.T, path string, dst any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func assertDeployGateReadable(t *testing.T, base, name string) {
	t.Helper()
	if _, err := os.Stat(filepath.Clean(filepath.Join(base, name))); err != nil {
		t.Fatalf("missing deploy gate file %s from %s: %v", name, base, err)
	}
}

func readDeployGateText(t *testing.T, base, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(filepath.Join(base, name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
