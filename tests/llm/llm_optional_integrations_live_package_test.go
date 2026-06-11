package leia_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type optionalIntegrationsLiveManifest struct {
	SchemaVersion               int      `json:"schema_version"`
	ID                          string   `json:"id"`
	PackageName                 string   `json:"package_name"`
	ProviderFree                bool     `json:"provider_free"`
	LiveNetworkDefault          bool     `json:"live_network_default"`
	RealDependencyImportDefault bool     `json:"real_dependency_import_default"`
	SourceExamples              []string `json:"source_examples"`
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
	Entrypoints  map[string]string `json:"entrypoints"`
	Schemas      map[string]string `json:"schemas"`
	Fixtures     map[string]string `json:"fixtures"`
	Integrations []struct {
		ID           string `json:"id"`
		DisplayName  string `json:"display_name"`
		PackageName  string `json:"package_name"`
		ImportName   string `json:"import_name"`
		Capability   string `json:"capability"`
		FixtureKey   string `json:"fixture_key"`
		OutputSchema string `json:"output_schema"`
		CleanSkip    bool   `json:"clean_skip"`
		SkipReason   string `json:"skip_reason"`
	} `json:"integrations"`
	TestGates []string `json:"test_gates"`
}

func TestFinRobotOptionalIntegrationsLivePackageManifest(t *testing.T) {
	base := optionalIntegrationsLivePackageDir(t)
	manifest := loadOptionalIntegrationsLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-optional-integrations-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-optional-integrations" {
		t.Fatalf("package name = %q", manifest.PackageName)
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
		manifest.DefaultPolicy.FixtureHook != "recorded_optional_live_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}

	for _, source := range manifest.SourceExamples {
		if _, err := os.Stat(filepath.Join(repoRoot(t), source)); err != nil {
			t.Fatalf("source example %q: %v", source, err)
		}
	}
	for _, key := range []string{"capability_gates", "fixture_index", "smoke"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"capability_gate", "fixture_index"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertOptionalLiveJSONFile(t, filepath.Join(base, path))
	}
	assertOptionalLiveJSONFile(t, filepath.Join(base, manifest.Fixtures["index"]))

	var ids []string
	capabilities := map[string]bool{}
	fixtures := map[string]bool{}
	for _, integration := range manifest.Integrations {
		ids = append(ids, integration.ID)
		if integration.ID == "" || integration.DisplayName == "" || integration.PackageName == "" || integration.ImportName == "" {
			t.Fatalf("integration identity incomplete: %#v", integration)
		}
		if !strings.HasPrefix(integration.Capability, "optional.") {
			t.Fatalf("%s capability = %q", integration.ID, integration.Capability)
		}
		if integration.FixtureKey == "" || integration.OutputSchema == "" {
			t.Fatalf("%s fixture/schema metadata incomplete: %#v", integration.ID, integration)
		}
		if !integration.CleanSkip || !strings.Contains(integration.SkipReason, "not installed") {
			t.Fatalf("%s clean skip metadata incomplete: %#v", integration.ID, integration)
		}
		if capabilities[integration.Capability] {
			t.Fatalf("duplicate capability %q", integration.Capability)
		}
		if fixtures[integration.FixtureKey] {
			t.Fatalf("duplicate fixture key %q", integration.FixtureKey)
		}
		capabilities[integration.Capability] = true
		fixtures[integration.FixtureKey] = true
	}
	sort.Strings(ids)
	wantIDs := []string{"backtrader", "fingpt", "finml", "finrl", "mplfinance", "ollama", "openbb"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("integration ids = %#v, want %#v", ids, wantIDs)
	}

	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"provider_free", "credentials", "capability", "clean skip", "import"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotOptionalIntegrationsLivePackageContractsAndFixtures(t *testing.T) {
	base := optionalIntegrationsLivePackageDir(t)
	var contract struct {
		ProviderFree               bool   `json:"provider_free"`
		LiveNetwork                bool   `json:"live_network"`
		RealDependencyImports      bool   `json:"real_dependency_imports"`
		CleanSkipWithoutDependency bool   `json:"clean_skip_without_dependency"`
		FixtureHook                string `json:"fixture_hook"`
		Gates                      []struct {
			ID                      string `json:"id"`
			Capability              string `json:"capability"`
			PackageName             string `json:"package_name"`
			ImportName              string `json:"import_name"`
			FixtureKey              string `json:"fixture_key"`
			CleanSkip               bool   `json:"clean_skip"`
			RequiresCredentials     bool   `json:"requires_credentials"`
			LiveNetwork             bool   `json:"live_network"`
			DependencyImported      bool   `json:"dependency_imported"`
			NoLiveImportDefault     bool   `json:"no_live_import_default"`
			ProviderCredentials     bool   `json:"provider_credentials_required"`
			CredentialAbsentSafe    bool   `json:"credential_absent_safe"`
			StatusWithoutDependency string `json:"status_without_dependency"`
			VersionMetadata         struct {
				Package               string  `json:"package"`
				VersionSource         string  `json:"version_source"`
				MinimumSupported      string  `json:"minimum_supported"`
				DetectedVersion       *string `json:"detected_version"`
				DependencyAbsentState string  `json:"dependency_absent_status"`
			} `json:"version_metadata"`
			ToolAdapterResultEnvelope map[string]any            `json:"tool_adapter_result_envelope"`
			AbsenceGates              map[string]map[string]any `json:"absence_gates"`
			TermsMetadata             struct {
				Integration                       string `json:"integration"`
				Mode                              string `json:"mode"`
				MetadataOnly                      bool   `json:"metadata_only"`
				LiveTermsRequiredBeforeNetworkUse bool   `json:"live_terms_required_before_network_use"`
				LiveNetworkEnabled                bool   `json:"live_network_enabled"`
			} `json:"terms_metadata"`
			CredentialRedaction struct {
				Status              string   `json:"status"`
				SecretValuesPresent bool     `json:"secret_values_present"`
				RedactedFields      []string `json:"redacted_fields"`
			} `json:"credential_redaction"`
		} `json:"gates"`
	}
	decodeOptionalLiveJSONFile(t, filepath.Join(base, "contracts", "optional_integration_capability_gates.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || !contract.CleanSkipWithoutDependency || contract.FixtureHook == "" {
		t.Fatalf("contract header must stay provider-free and clean-skip safe: %#v", contract)
	}
	if len(contract.Gates) != 7 {
		t.Fatalf("contract gates = %d, want 7", len(contract.Gates))
	}
	for _, gate := range contract.Gates {
		if gate.ID == "" || gate.PackageName == "" || gate.ImportName == "" || gate.FixtureKey == "" {
			t.Fatalf("gate metadata incomplete: %#v", gate)
		}
		if !strings.HasPrefix(gate.Capability, "optional.") {
			t.Fatalf("gate capability = %q", gate.Capability)
		}
		if !gate.CleanSkip || gate.RequiresCredentials || gate.LiveNetwork || gate.DependencyImported || gate.StatusWithoutDependency != "skipped" {
			t.Fatalf("gate must cleanly skip without live imports: %#v", gate)
		}
		if !gate.NoLiveImportDefault || gate.ProviderCredentials || !gate.CredentialAbsentSafe {
			t.Fatalf("%s optional gate must be provider-free by default: %#v", gate.ID, gate)
		}
		assertOptionalLiveVersionMetadata(t, gate.ID, gate.PackageName, gate.VersionMetadata.Package, gate.VersionMetadata.VersionSource, gate.VersionMetadata.DependencyAbsentState, gate.VersionMetadata.DetectedVersion)
		assertOptionalLiveResultEnvelope(t, gate.ID, gate.Capability, gate.FixtureKey, gate.ToolAdapterResultEnvelope)
		assertOptionalLiveAbsenceGates(t, gate.ID, gate.AbsenceGates)
		if gate.TermsMetadata.Integration != gate.ID ||
			gate.TermsMetadata.Mode != "fixture_replay" ||
			!gate.TermsMetadata.MetadataOnly ||
			!gate.TermsMetadata.LiveTermsRequiredBeforeNetworkUse ||
			gate.TermsMetadata.LiveNetworkEnabled {
			t.Fatalf("%s terms metadata must stay fixture-only: %#v", gate.ID, gate.TermsMetadata)
		}
		if gate.CredentialRedaction.Status != "redacted" || gate.CredentialRedaction.SecretValuesPresent || len(gate.CredentialRedaction.RedactedFields) == 0 {
			t.Fatalf("%s credential redaction metadata incomplete: %#v", gate.ID, gate.CredentialRedaction)
		}
	}

	var fixtures struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Fixtures              []struct {
			FixtureKey    string         `json:"fixture_key"`
			IntegrationID string         `json:"integration_id"`
			Capability    string         `json:"capability"`
			Metadata      map[string]any `json:"metadata"`
		} `json:"fixtures"`
	}
	decodeOptionalLiveJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &fixtures)
	if !fixtures.ProviderFree || fixtures.LiveNetwork || fixtures.RealDependencyImports || len(fixtures.Fixtures) != 7 {
		t.Fatalf("fixture index header/count = %#v", fixtures)
	}
	for _, fixture := range fixtures.Fixtures {
		if fixture.FixtureKey == "" || fixture.IntegrationID == "" || !strings.HasPrefix(fixture.Capability, "optional.") {
			t.Fatalf("fixture metadata incomplete: %#v", fixture)
		}
		if fixture.Metadata["replay_ready"] != true {
			t.Fatalf("%s replay_ready = %#v", fixture.FixtureKey, fixture.Metadata["replay_ready"])
		}
		version, _ := fixture.Metadata["version_metadata"].(map[string]any)
		assertOptionalLiveVersionMetadata(t, fixture.IntegrationID, fixture.IntegrationID, stringValue(version["package"]), stringValue(version["version_source"]), stringValue(version["dependency_absent_status"]), nil)
		envelope, _ := fixture.Metadata["result_envelope"].(map[string]any)
		assertOptionalLiveResultEnvelope(t, fixture.IntegrationID, fixture.Capability, fixture.FixtureKey, envelope)
		absence, _ := fixture.Metadata["absence_gates"].(map[string]any)
		if len(absence) == 0 {
			t.Fatalf("%s fixture absence gates missing", fixture.FixtureKey)
		}
		terms, _ := fixture.Metadata["terms"].(map[string]any)
		if terms["mode"] != "fixture_replay" || terms["metadata_only"] != true || terms["live_terms_required_before_network_use"] != true {
			t.Fatalf("%s terms metadata incomplete: %#v", fixture.FixtureKey, terms)
		}
		redaction, _ := fixture.Metadata["credential_redaction"].(map[string]any)
		if redaction["status"] != "redacted" || redaction["secret_values_present"] != false {
			t.Fatalf("%s credential redaction metadata incomplete: %#v", fixture.FixtureKey, redaction)
		}
	}
	assertOptionalLiveNoCredentialLeakage(t, filepath.Join(base, "contracts", "optional_integration_capability_gates.json"))
	assertOptionalLiveNoCredentialLeakage(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"))
}

func TestFinRobotOptionalIntegrationsLivePackageNoLiveImports(t *testing.T) {
	base := optionalIntegrationsLivePackageDir(t)
	data, err := os.ReadFile(filepath.Join(base, "main.leia"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pattern := range []string{
		`(?m)^\s*import\s+`,
		`(?m)^\s*use\s+`,
		`(?m)^\s*load\s*\(`,
		`(?m)^\s*require\s*\(`,
	} {
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("main.leia contains live dependency loader matching %q", pattern)
		}
	}
	for _, provider := range []string{"fingpt", "finrl", "finml", "backtrader", "mplfinance", "openbb", "ollama"} {
		pattern := `(?m)^\s*` + regexp.QuoteMeta(provider) + `\s*[.(]`
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("main.leia must not call provider SDK symbol matching %q", pattern)
		}
	}
}

func TestFinRobotOptionalIntegrationsLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(optionalIntegrationsLivePackageDir(t), "main.leia")

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
			got, err := vm.Get("optional_live_package_summary")
			if err != nil {
				t.Fatalf("Get optional_live_package_summary: %v", err)
			}
			want := "optional_integrations_live_package integrations=7 provider_free=true live_network=false imports=false clean_skip=7"
			if got != want {
				t.Fatalf("optional_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func optionalIntegrationsLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "optional_integrations")
}

func loadOptionalIntegrationsLiveManifest(t *testing.T, base string) optionalIntegrationsLiveManifest {
	t.Helper()
	var manifest optionalIntegrationsLiveManifest
	decodeOptionalLiveJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertOptionalLiveJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeOptionalLiveJSONFile(t, path, &value)
}

func assertOptionalLiveVersionMetadata(t *testing.T, id, wantPackage, gotPackage, source, absentStatus string, detected *string) {
	t.Helper()
	if gotPackage != wantPackage || source != "fixture_metadata" || absentStatus != "skipped" {
		t.Fatalf("%s version metadata = package:%q source:%q absent:%q, want package:%q fixture_metadata skipped", id, gotPackage, source, absentStatus, wantPackage)
	}
	if detected != nil {
		t.Fatalf("%s detected version must stay nil without importing dependency: %q", id, *detected)
	}
}

func assertOptionalLiveResultEnvelope(t *testing.T, id, capability, fixtureKey string, envelope map[string]any) {
	t.Helper()
	if len(envelope) == 0 {
		t.Fatalf("%s result envelope missing", id)
	}
	providerFree := envelope["provider_free"]
	liveNetwork := envelope["live_network"]
	realImports := envelope["real_dependency_imports"]
	if metadata, _ := envelope["metadata"].(map[string]any); len(metadata) != 0 {
		if providerFree == nil {
			providerFree = metadata["provider_free"]
		}
		if liveNetwork == nil {
			liveNetwork = metadata["live_network"]
		}
		if realImports == nil {
			realImports = metadata["real_dependency_imports"]
		}
	}
	if providerFree != true || liveNetwork != false || realImports != false {
		t.Fatalf("%s result envelope must stay provider-free: %#v", id, envelope)
	}
	if envelope["skip_status"] != nil && envelope["skip_status"] != "skipped" {
		t.Fatalf("%s result envelope skip status = %#v", id, envelope["skip_status"])
	}
	if envelope["data_source"] != nil && envelope["data_source"] != "fixture" {
		t.Fatalf("%s result envelope data source = %#v", id, envelope["data_source"])
	}
	if envelope["status"] != nil && envelope["status"] != "ok" {
		t.Fatalf("%s fixture result status = %#v", id, envelope["status"])
	}
	if envelope["capability"] != nil && envelope["capability"] != capability {
		t.Fatalf("%s fixture result capability = %#v, want %q", id, envelope["capability"], capability)
	}
	if envelope["fixture_key"] != nil && envelope["fixture_key"] != fixtureKey {
		t.Fatalf("%s fixture result fixture_key = %#v, want %q", id, envelope["fixture_key"], fixtureKey)
	}
}

func assertOptionalLiveAbsenceGates(t *testing.T, id string, gates map[string]map[string]any) {
	t.Helper()
	if len(gates) == 0 {
		t.Fatalf("%s absence gates missing", id)
	}
	for name, gate := range gates {
		if gate["clean_skip"] != true || gate["absent_status"] != "skipped" {
			t.Fatalf("%s absence gate %q must cleanly skip: %#v", id, name, gate)
		}
	}
}

func assertOptionalLiveNoCredentialLeakage(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pattern := range []string{
		`sk-[A-Za-z0-9_-]{12,}`,
		`Bearer\s+[A-Za-z0-9._-]{12,}`,
		`gh[pousr]_[A-Za-z0-9_]{20,}`,
		`xox[baprs]-[A-Za-z0-9-]{12,}`,
		`(?i)"(?:api_key|access_token|secret|password)"\s*:\s*"[^"<{\[][^"]{7,}"`,
	} {
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("%s appears to contain an unredacted credential matching %q", path, pattern)
		}
	}
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	s, _ := value.(string)
	return s
}

func decodeOptionalLiveJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
