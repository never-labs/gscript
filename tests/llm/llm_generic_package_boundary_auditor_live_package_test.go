package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type genericPackageBoundaryAuditorManifest struct {
	SchemaVersion               int                                         `json:"schema_version"`
	ID                          string                                      `json:"id"`
	PackageName                 string                                      `json:"package_name"`
	DialectAliases              []genericPackageBoundaryAuditorDialectAlias `json:"dialect_aliases"`
	DialectSurface              []string                                    `json:"dialect_surface"`
	ProviderFree                bool                                        `json:"provider_free"`
	LiveNetworkDefault          bool                                        `json:"live_network_default"`
	RealDependencyImportDefault bool                                        `json:"real_dependency_import_default"`
	Credentials                 struct {
		Required          []string `json:"required"`
		Optional          []string `json:"optional"`
		SecretEnvPatterns []string `json:"secret_env_patterns"`
		Policy            string   `json:"policy"`
	} `json:"credentials"`
	DefaultPolicy struct {
		Mode                        string `json:"mode"`
		LiveNetwork                 bool   `json:"live_network"`
		ProviderCredentialsRequired bool   `json:"provider_credentials_required"`
		RealDependencyImports       bool   `json:"real_dependency_imports"`
		CleanSkipWithoutDependency  bool   `json:"clean_skip_without_dependency"`
		FixtureHook                 string `json:"fixture_hook"`
	} `json:"default_policy"`
	Entrypoints  map[string]string                     `json:"entrypoints"`
	Schemas      map[string]string                     `json:"schemas"`
	Fixtures     map[string]string                     `json:"fixtures"`
	Targets      []genericPackageBoundaryAuditorTarget `json:"audit_targets"`
	Capabilities []string                              `json:"capabilities"`
	TestGates    []string                              `json:"test_gates"`
	NoBuiltIn    struct {
		Required  bool   `json:"required"`
		Statement string `json:"statement"`
	} `json:"no_built_in_guarantee"`
}

type genericPackageBoundaryAuditorTarget struct {
	ID                    string `json:"id"`
	Capability            string `json:"capability"`
	FixtureKey            string `json:"fixture_key"`
	Schema                string `json:"schema"`
	ProviderFree          bool   `json:"provider_free"`
	LiveNetwork           bool   `json:"live_network"`
	RealDependencyImports bool   `json:"real_dependency_imports"`
}

type genericPackageBoundaryAuditorDialectAlias struct {
	ID     string `json:"id"`
	Target string `json:"target"`
	Scope  string `json:"scope"`
	Source string `json:"source"`
	Status string `json:"status"`
}

func TestGenericPackageBoundaryAuditorLivePackageManifest(t *testing.T) {
	base := genericPackageBoundaryAuditorDir(t)
	manifest := loadGenericPackageBoundaryAuditorManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "generic-ai-package-boundary-audit-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-generic-ai-package-boundary-auditor" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !genericPackageBoundaryAuditorDialectAliasContains(manifest.DialectAliases, "ai.package.audit", "generic.ai.package.boundary.audit.manifest") {
		t.Fatalf("dialect aliases = %#v", manifest.DialectAliases)
	}
	for _, want := range []string{"generic.ai.package.boundary.audit", "ai.package.audit"} {
		if !genericPackageBoundaryAuditorContains(manifest.DialectSurface, want) {
			t.Fatalf("dialect surface missing %q: %#v", want, manifest.DialectSurface)
		}
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
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
		manifest.DefaultPolicy.FixtureHook != "recorded_generic_package_boundary_audit_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}
	for _, key := range []string{"smoke", "capability_policy", "audit_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
		assertGenericPackageBoundaryAuditorJSONOrLeiaFile(t, filepath.Join(base, manifest.Entrypoints[key]))
	}
	for _, key := range []string{"audit_finding", "audit_scope", "capability_policy", "fixture_index", "missing_boundary_record"} {
		if manifest.Schemas[key] == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertGenericPackageBoundaryAuditorJSONFile(t, filepath.Join(base, manifest.Schemas[key]))
	}
	for _, key := range []string{"index", "package_manifest_audit", "fixture_index_audit", "example_registry_audit", "audit_findings", "missing_boundary_records"} {
		if manifest.Fixtures[key] == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertGenericPackageBoundaryAuditorJSONFile(t, filepath.Join(base, manifest.Fixtures[key]))
	}

	var targetIDs []string
	for _, target := range manifest.Targets {
		targetIDs = append(targetIDs, target.ID)
		if target.ID == "" || target.Capability == "" || target.FixtureKey == "" || target.Schema == "" {
			t.Fatalf("target metadata incomplete: %#v", target)
		}
		if !strings.HasPrefix(target.Capability, "generic.ai.package.boundary.audit.") {
			t.Fatalf("%s capability = %q", target.ID, target.Capability)
		}
		if !target.ProviderFree || target.LiveNetwork || target.RealDependencyImports {
			t.Fatalf("%s target must stay provider-free: %#v", target.ID, target)
		}
	}
	sort.Strings(targetIDs)
	wantTargetIDs := []string{"audit_findings", "capability_policy", "example_registry_audit", "fixture_index_audit", "missing_boundary_records", "package_manifest_audit"}
	if !reflect.DeepEqual(targetIDs, wantTargetIDs) {
		t.Fatalf("target ids = %#v, want %#v", targetIDs, wantTargetIDs)
	}
	for _, want := range []string{
		"generic.ai.package.boundary.audit.manifest",
		"generic.ai.package.boundary.audit.fixture_index",
		"generic.ai.package.boundary.audit.example_registry",
		"generic.ai.package.boundary.audit.capability_policy",
		"generic.ai.package.boundary.audit.findings",
		"generic.ai.package.boundary.audit.missing_boundary_records",
	} {
		if !genericPackageBoundaryAuditorContains(manifest.Capabilities, want) {
			t.Fatalf("capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}
	if !manifest.NoBuiltIn.Required || !strings.Contains(strings.ToLower(manifest.NoBuiltIn.Statement), "does not provide") {
		t.Fatalf("no-built-in guarantee missing or weak: %#v", manifest.NoBuiltIn)
	}
	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"package manifest", "fixture index", "example registry", "capability policy", "audit findings", "missing boundary records"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestGenericPackageBoundaryAuditorCapabilityPolicyAndContract(t *testing.T) {
	base := genericPackageBoundaryAuditorDir(t)
	var policy struct {
		ProviderFree              bool     `json:"provider_free"`
		LiveNetwork               bool     `json:"live_network"`
		RealDependencyImports     bool     `json:"real_dependency_imports"`
		DefaultDecision           string   `json:"default_decision"`
		AllowedCapabilityPrefixes []string `json:"allowed_capability_prefixes"`
		DeniedByDefault           []string `json:"denied_by_default"`
		Capabilities              []struct {
			ID                          string `json:"id"`
			Capability                  string `json:"capability"`
			Mode                        string `json:"mode"`
			LiveNetwork                 bool   `json:"live_network"`
			ProviderCredentialsRequired bool   `json:"provider_credentials_required"`
			RealDependencyImports       bool   `json:"real_dependency_imports"`
		} `json:"capabilities"`
		AcceptanceGates []string `json:"acceptance_gates"`
	}
	decodeGenericPackageBoundaryAuditorJSONFile(t, filepath.Join(base, "contracts", "capability_policy.json"), &policy)
	if !policy.ProviderFree || policy.LiveNetwork || policy.RealDependencyImports || policy.DefaultDecision != "deny" {
		t.Fatalf("capability policy header = %#v", policy)
	}
	for _, prefix := range []string{"generic.ai.package.boundary.audit.", "ai.package.audit."} {
		if !genericPackageBoundaryAuditorContains(policy.AllowedCapabilityPrefixes, prefix) {
			t.Fatalf("capability policy missing prefix %q: %#v", prefix, policy.AllowedCapabilityPrefixes)
		}
	}
	denied := strings.ToLower(strings.Join(policy.DeniedByDefault, " "))
	for _, want := range []string{"live_network", "provider_credentials", "provider_sdk_imports", "runtime_mutation"} {
		if !strings.Contains(denied, want) {
			t.Fatalf("denied_by_default missing %q: %s", want, denied)
		}
	}
	if len(policy.Capabilities) < 5 {
		t.Fatalf("capability policy should cover all audit surfaces: %#v", policy.Capabilities)
	}
	for _, capability := range policy.Capabilities {
		if capability.ID == "" || capability.Capability == "" || capability.Mode != "fixture_replay" {
			t.Fatalf("capability metadata incomplete: %#v", capability)
		}
		if capability.LiveNetwork || capability.ProviderCredentialsRequired || capability.RealDependencyImports {
			t.Fatalf("%s capability must deny live/provider behavior: %#v", capability.ID, capability)
		}
	}

	var contract struct {
		ProviderFree                  bool     `json:"provider_free"`
		LiveNetwork                   bool     `json:"live_network"`
		RealDependencyImports         bool     `json:"real_dependency_imports"`
		AuditSurfaces                 []string `json:"audit_surfaces"`
		RequiredManifestFields        []string `json:"required_manifest_fields"`
		RequiredFixtureIndexFields    []string `json:"required_fixture_index_fields"`
		RequiredRegistryFields        []string `json:"required_registry_fields"`
		RequiredMissingBoundaryFields []string `json:"required_missing_boundary_fields"`
		AcceptanceGates               []string `json:"acceptance_gates"`
	}
	decodeGenericPackageBoundaryAuditorJSONFile(t, filepath.Join(base, "contracts", "package_boundary_audit_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports {
		t.Fatalf("audit contract must stay provider-free: %#v", contract)
	}
	for _, surface := range []string{"package_manifest", "fixture_index", "example_registry", "capability_policy", "audit_findings", "missing_boundary_records"} {
		if !genericPackageBoundaryAuditorContains(contract.AuditSurfaces, surface) {
			t.Fatalf("audit contract missing surface %q: %#v", surface, contract.AuditSurfaces)
		}
	}
	for _, field := range []string{"provider_free", "live_network_default", "real_dependency_import_default", "no_built_in_guarantee"} {
		if !genericPackageBoundaryAuditorContains(contract.RequiredManifestFields, field) {
			t.Fatalf("audit contract missing manifest field %q: %#v", field, contract.RequiredManifestFields)
		}
	}
	for _, field := range []string{"fixture_key", "capability", "path", "schema", "metadata.replay_ready"} {
		if !genericPackageBoundaryAuditorContains(contract.RequiredFixtureIndexFields, field) {
			t.Fatalf("audit contract missing fixture index field %q: %#v", field, contract.RequiredFixtureIndexFields)
		}
	}
}

func TestGenericPackageBoundaryAuditorFixtures(t *testing.T) {
	base := genericPackageBoundaryAuditorDir(t)
	manifest := loadGenericPackageBoundaryAuditorManifest(t, base)
	var index struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Fixtures              []struct {
			FixtureKey            string         `json:"fixture_key"`
			Capability            string         `json:"capability"`
			Path                  string         `json:"path"`
			Schema                string         `json:"schema"`
			Metadata              map[string]any `json:"metadata"`
			ProviderFree          bool           `json:"provider_free"`
			LiveNetwork           bool           `json:"live_network"`
			RealDependencyImports bool           `json:"real_dependency_imports"`
		} `json:"fixtures"`
	}
	decodeGenericPackageBoundaryAuditorJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != len(manifest.Targets) {
		t.Fatalf("fixture index header/count = %#v", index)
	}
	targetsByKey := map[string]genericPackageBoundaryAuditorTarget{}
	for _, target := range manifest.Targets {
		targetsByKey[target.FixtureKey] = target
	}
	seen := map[string]bool{}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Capability == "" || fixture.Path == "" || fixture.Schema == "" {
			t.Fatalf("fixture metadata incomplete: %#v", fixture)
		}
		target, ok := targetsByKey[fixture.FixtureKey]
		if !ok {
			t.Fatalf("fixture %q is not declared by manifest audit_targets", fixture.FixtureKey)
		}
		if fixture.Capability != target.Capability {
			t.Fatalf("%s capability = %q, want manifest capability %q", fixture.FixtureKey, fixture.Capability, target.Capability)
		}
		if !strings.HasPrefix(fixture.Capability, "generic.ai.package.boundary.audit.") {
			t.Fatalf("fixture capability = %q", fixture.Capability)
		}
		if fixture.Metadata["replay_ready"] != true || fixture.Metadata["surface"] == "" {
			t.Fatalf("%s fixture metadata = %#v", fixture.FixtureKey, fixture.Metadata)
		}
		if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports {
			t.Fatalf("%s fixture must stay provider-free: %#v", fixture.FixtureKey, fixture)
		}
		if seen[fixture.FixtureKey] {
			t.Fatalf("duplicate fixture key %q", fixture.FixtureKey)
		}
		seen[fixture.FixtureKey] = true
		assertGenericPackageBoundaryAuditorJSONFile(t, filepath.Join(base, fixture.Path))
		assertGenericPackageBoundaryAuditorJSONFile(t, filepath.Join(base, fixture.Schema))
	}
	for _, target := range manifest.Targets {
		if !seen[target.FixtureKey] {
			t.Fatalf("manifest audit target %q is missing from fixture index", target.FixtureKey)
		}
	}

	assertGenericPackageBoundaryAuditorFindings(t, filepath.Join(base, "fixtures", "package_manifest_audit_fixture.json"), "package_manifest")
	assertGenericPackageBoundaryAuditorFindings(t, filepath.Join(base, "fixtures", "fixture_index_audit_fixture.json"), "fixture_index")
	assertGenericPackageBoundaryAuditorFindings(t, filepath.Join(base, "fixtures", "example_registry_audit_fixture.json"), "example_registry")
	assertGenericPackageBoundaryAuditorFindings(t, filepath.Join(base, "fixtures", "audit_findings_fixture.json"), "")
}

func TestGenericPackageBoundaryAuditorMissingBoundaryRecords(t *testing.T) {
	base := genericPackageBoundaryAuditorDir(t)
	var fixture struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Records               []struct {
			RecordID            string `json:"record_id"`
			PackageID           string `json:"package_id"`
			MissingBoundaryType string `json:"missing_boundary_type"`
			RequiredArtifact    string `json:"required_artifact"`
			OwnerHint           string `json:"owner_hint"`
			Blocking            bool   `json:"blocking"`
			Remediation         string `json:"remediation"`
		} `json:"records"`
	}
	decodeGenericPackageBoundaryAuditorJSONFile(t, filepath.Join(base, "fixtures", "missing_boundary_records_fixture.json"), &fixture)
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || len(fixture.Records) < 3 {
		t.Fatalf("missing boundary fixture header/count = %#v", fixture)
	}
	types := map[string]bool{}
	for _, record := range fixture.Records {
		if record.RecordID == "" || record.PackageID == "" || record.MissingBoundaryType == "" || record.RequiredArtifact == "" || record.OwnerHint == "" || record.Remediation == "" {
			t.Fatalf("missing boundary record incomplete: %#v", record)
		}
		if !record.Blocking {
			t.Fatalf("sample missing boundary records should be blocking: %#v", record)
		}
		types[record.MissingBoundaryType] = true
	}
	for _, want := range []string{"fixture_index", "example_registry", "capability_policy"} {
		if !types[want] {
			t.Fatalf("missing boundary fixture lacks %q record: %#v", want, types)
		}
	}
}

func TestGenericPackageBoundaryAuditorNoLiveImports(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(genericPackageBoundaryAuditorDir(t), "main.leia"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pattern := range []string{
		`(?m)^\s*import\s+`,
		`(?m)^\s*use\s+`,
		`(?m)^\s*load\s*\(`,
		`(?m)^\s*require\s*\(`,
		`(?m)^\s*(openai|anthropic|requests|http|urllib|yfinance|openbb)\s*[.(]`,
	} {
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("main.leia contains live dependency loader matching %q", pattern)
		}
	}
}

func TestGenericPackageBoundaryAuditorExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericPackageBoundaryAuditorDir(t), "main.leia")
	want := "generic_package_boundary_audit targets=6 aliases=1 provider_free=true live_network=false imports=false"
	for _, result := range runFinRobotLivePackageSummarySmoke(t, path, "generic_package_boundary_audit_summary", "generic_package_boundary_audit", leia.LibString) {
		if result.Summary != want {
			t.Fatalf("generic_package_boundary_audit_summary = %#v, want %#v", result.Summary, want)
		}
	}
}

func genericPackageBoundaryAuditorDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_package_boundary_auditor")
}

func loadGenericPackageBoundaryAuditorManifest(t *testing.T, base string) genericPackageBoundaryAuditorManifest {
	t.Helper()
	var manifest genericPackageBoundaryAuditorManifest
	decodeGenericPackageBoundaryAuditorJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertGenericPackageBoundaryAuditorFindings(t *testing.T, path, surface string) {
	t.Helper()
	var fixture struct {
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		AuditSurface          string `json:"audit_surface"`
		Findings              []struct {
			FindingID    string `json:"finding_id"`
			Severity     string `json:"severity"`
			RuleID       string `json:"rule_id"`
			EvidencePath string `json:"evidence_path"`
			Message      string `json:"message"`
			Remediation  string `json:"remediation"`
			ProviderFree bool   `json:"provider_free"`
		} `json:"findings"`
	}
	decodeGenericPackageBoundaryAuditorJSONFile(t, path, &fixture)
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || len(fixture.Findings) == 0 {
		t.Fatalf("%s finding fixture header/count = %#v", path, fixture)
	}
	if surface != "" && fixture.AuditSurface != surface {
		t.Fatalf("%s audit surface = %q, want %q", path, fixture.AuditSurface, surface)
	}
	for _, finding := range fixture.Findings {
		if finding.FindingID == "" || finding.Severity == "" || finding.RuleID == "" || finding.EvidencePath == "" || finding.Message == "" || finding.Remediation == "" {
			t.Fatalf("%s finding incomplete: %#v", path, finding)
		}
		if !finding.ProviderFree {
			t.Fatalf("%s finding must be provider-free: %#v", path, finding)
		}
	}
}

func assertGenericPackageBoundaryAuditorJSONOrLeiaFile(t *testing.T, path string) {
	t.Helper()
	if strings.HasSuffix(path, ".leia") {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
		return
	}
	assertGenericPackageBoundaryAuditorJSONFile(t, path)
}

func assertGenericPackageBoundaryAuditorJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeGenericPackageBoundaryAuditorJSONFile(t, path, &value)
}

func decodeGenericPackageBoundaryAuditorJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func genericPackageBoundaryAuditorContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func genericPackageBoundaryAuditorDialectAliasContains(aliases []genericPackageBoundaryAuditorDialectAlias, id, target string) bool {
	for _, alias := range aliases {
		if alias.ID == id && alias.Target == target && alias.Scope == "package" && alias.Status == "active" {
			return true
		}
	}
	return false
}
