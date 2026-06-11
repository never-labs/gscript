package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type promptRolesLiveManifest struct {
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
		LiveModelCalls              bool   `json:"live_model_calls"`
		FixtureHook                 string `json:"fixture_hook"`
	} `json:"default_policy"`
	Entrypoints    map[string]string `json:"entrypoints"`
	Schemas        map[string]string `json:"schemas"`
	Fixtures       map[string]string `json:"fixtures"`
	Capabilities   []string          `json:"capabilities"`
	PromptCatalogs []struct {
		ID         string `json:"id"`
		SourcePath string `json:"source_path"`
		Capability string `json:"capability"`
		FixtureKey string `json:"fixture_key"`
		Schema     string `json:"schema"`
	} `json:"prompt_catalogs"`
	RoleProfiles []struct {
		ID           string `json:"id"`
		Version      string `json:"version"`
		Status       string `json:"status"`
		Capability   string `json:"capability"`
		Termination  string `json:"termination"`
		OutputSchema string `json:"output_schema"`
	} `json:"role_profiles"`
	TestGates []string `json:"test_gates"`
}

func TestFinRobotPromptRolesLivePackageManifestSchemaFixturesAndGates(t *testing.T) {
	base := promptRolesLivePackageDir(t)
	manifest := loadPromptRolesLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-prompt-roles-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-prompt-roles" {
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
		manifest.DefaultPolicy.LiveModelCalls ||
		manifest.DefaultPolicy.FixtureHook != "recorded_prompt_role_fixture" {
		t.Fatalf("default policy must stay fixture-only: %#v", manifest.DefaultPolicy)
	}

	for _, source := range manifest.SourceExamples {
		if _, err := os.Stat(filepath.Join(repoRoot(t), source)); err != nil {
			t.Fatalf("source example %q: %v", source, err)
		}
	}
	for _, key := range []string{"prompt_role_contract", "fixture_index", "smoke"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"prompt_catalog", "role_profile", "section_agent_output", "source_evidence_validation"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertJSONFile(t, filepath.Join(base, path))
	}
	schemaExpectations := map[string][]string{
		manifest.Schemas["prompt_catalog"]:             {"prompt_snapshot", "delegation_triggers", "parse_examples"},
		manifest.Schemas["role_profile"]:               {"profile_snapshot", "version_history", "handoff_fields"},
		manifest.Schemas["source_evidence_validation"]: {"minimum_source_refs", "source_refs", "rules"},
	}
	for rel, wants := range schemaExpectations {
		data, err := os.ReadFile(filepath.Join(base, rel))
		if err != nil {
			t.Fatal(err)
		}
		schemaText := string(data)
		for _, want := range wants {
			if !strings.Contains(schemaText, want) {
				t.Fatalf("schema %s missing %q", rel, want)
			}
		}
	}
	assertJSONFile(t, filepath.Join(base, manifest.Fixtures["index"]))

	wantCapabilities := []string{
		"prompt.role.catalog.agent_library",
		"prompt.role.catalog.prompts",
		"prompt.role.catalog.utils",
		"prompt.role.catalog.text_generator_agents",
		"prompt.role.catalog.equity_agents",
		"prompt.role.profile.version",
		"prompt.role.section.output_schema",
		"prompt.role.termination.convention",
		"prompt.role.evidence.validation",
	}
	for _, want := range wantCapabilities {
		if !contains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}

	var catalogs []string
	for _, catalog := range manifest.PromptCatalogs {
		catalogs = append(catalogs, catalog.ID)
		if !strings.HasPrefix(catalog.SourcePath, "FinRobot.") ||
			!strings.HasPrefix(catalog.Capability, "prompt.role.catalog.") ||
			!strings.HasPrefix(catalog.FixtureKey, "prompt_catalog:") ||
			catalog.Schema != "prompt_catalog_v1" {
			t.Fatalf("catalog metadata incomplete: %#v", catalog)
		}
	}
	sort.Strings(catalogs)
	wantCatalogs := []string{"agent_library", "equity_agents", "prompts", "text_generator_agents", "utils"}
	if strings.Join(catalogs, ",") != strings.Join(wantCatalogs, ",") {
		t.Fatalf("catalog ids = %#v, want %#v", catalogs, wantCatalogs)
	}

	if len(manifest.RoleProfiles) != 4 {
		t.Fatalf("role profiles = %d, want 4", len(manifest.RoleProfiles))
	}
	for _, role := range manifest.RoleProfiles {
		if role.Version != "1.0.0" || role.Status != "fixture_locked" || role.Capability != "prompt.role.profile.version" || role.Termination != "TERMINATE" {
			t.Fatalf("role profile versioning/termination incomplete: %#v", role)
		}
	}

	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"prompt catalog", "provider_free", "credentials", "terminate", "source evidence"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotPromptRolesLivePackageContractAndEvidenceFixture(t *testing.T) {
	base := promptRolesLivePackageDir(t)
	var contract struct {
		ProviderFree          bool     `json:"provider_free"`
		LiveNetwork           bool     `json:"live_network"`
		RealDependencyImports bool     `json:"real_dependency_imports"`
		LiveModelCalls        bool     `json:"live_model_calls"`
		Capabilities          []string `json:"capabilities"`
		PromptCatalogs        []struct {
			ID         string `json:"id"`
			SourcePath string `json:"source_path"`
			Version    string `json:"version"`
			FixtureKey string `json:"fixture_key"`
			Capability string `json:"capability"`
			Entries    []struct {
				ID             string `json:"id"`
				RoleID         string `json:"role_id"`
				Kind           string `json:"kind"`
				PromptSnapshot struct {
					System         string   `json:"system"`
					Task           string   `json:"task"`
					Constraints    []string `json:"constraints"`
					ExpectedOutput string   `json:"expected_output"`
				} `json:"prompt_snapshot"`
				DelegationTriggers []struct {
					Pattern        string `json:"pattern"`
					DelegateRoleID string `json:"delegate_role_id"`
					Reason         string `json:"reason"`
					ParseExamples  []struct {
						Input          string `json:"input"`
						Matched        bool   `json:"matched"`
						DelegateRoleID string `json:"delegate_role_id"`
					} `json:"parse_examples"`
				} `json:"delegation_triggers"`
				Termination string `json:"termination"`
			} `json:"entries"`
		} `json:"prompt_catalogs"`
		RoleProfileVersions []struct {
			ID              string `json:"id"`
			Version         string `json:"version"`
			Status          string `json:"status"`
			SourceCatalog   string `json:"source_catalog"`
			OutputSchema    string `json:"output_schema"`
			ProfileSnapshot struct {
				Mission       string   `json:"mission"`
				Inputs        []string `json:"inputs"`
				HandoffFields []string `json:"handoff_fields"`
			} `json:"profile_snapshot"`
			VersionHistory []struct {
				Version string `json:"version"`
				Status  string `json:"status"`
				Change  string `json:"change"`
			} `json:"version_history"`
			Termination string `json:"termination"`
		} `json:"role_profile_versions"`
		SectionAgentOutput struct {
			Schema         string   `json:"schema"`
			Capability     string   `json:"capability"`
			RequiredFields []string `json:"required_fields"`
			Termination    string   `json:"termination"`
		} `json:"section_agent_output"`
		SourceEvidenceValidation struct {
			Schema                string `json:"schema"`
			Capability            string `json:"capability"`
			MinimumSourceRefs     int    `json:"minimum_source_refs"`
			RejectsUnresolvedRefs bool   `json:"rejects_unresolved_refs"`
			RequiresFixtureKey    bool   `json:"requires_fixture_key"`
			RequiresProviderFree  bool   `json:"requires_provider_free"`
			LiveNetwork           bool   `json:"live_network"`
			Rules                 []struct {
				ID          string   `json:"id"`
				Description string   `json:"description"`
				PassExample []string `json:"pass_example"`
				FailExample []string `json:"fail_example"`
			} `json:"rules"`
		} `json:"source_evidence_validation"`
		TerminationConvention struct {
			Capability         string   `json:"capability"`
			Literal            string   `json:"literal"`
			RequiredOnComplete bool     `json:"required_on_complete"`
			AllowedFinalStatus []string `json:"allowed_final_statuses"`
			DriftChecks        []struct {
				Output   string `json:"output"`
				Accepted bool   `json:"accepted"`
				Reason   string `json:"reason"`
			} `json:"drift_checks"`
		} `json:"termination_convention"`
	}
	decodeJSONFile(t, filepath.Join(base, "contracts", "prompt_role_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || contract.LiveModelCalls {
		t.Fatalf("contract must be provider-free and offline: %#v", contract)
	}
	if len(contract.PromptCatalogs) != 5 || len(contract.RoleProfileVersions) != 4 {
		t.Fatalf("contract catalog/profile counts = %d/%d", len(contract.PromptCatalogs), len(contract.RoleProfileVersions))
	}
	for _, catalog := range contract.PromptCatalogs {
		if catalog.Version != "1.0.0" || len(catalog.Entries) == 0 {
			t.Fatalf("catalog version/entries incomplete: %#v", catalog)
		}
		for _, entry := range catalog.Entries {
			if entry.ID == "" || entry.RoleID == "" || entry.Kind == "" || entry.Termination != "TERMINATE" {
				t.Fatalf("catalog entry incomplete: %#v", entry)
			}
			if entry.PromptSnapshot.System == "" ||
				entry.PromptSnapshot.Task == "" ||
				len(entry.PromptSnapshot.Constraints) == 0 ||
				!strings.Contains(entry.PromptSnapshot.ExpectedOutput, "TERMINATE") {
				t.Fatalf("catalog prompt snapshot incomplete: %#v", entry)
			}
			for _, trigger := range entry.DelegationTriggers {
				if trigger.Pattern == "" || trigger.DelegateRoleID == "" || trigger.Reason == "" || len(trigger.ParseExamples) == 0 {
					t.Fatalf("delegation trigger incomplete: %#v", trigger)
				}
				re, err := regexp.Compile(trigger.Pattern)
				if err != nil {
					t.Fatalf("compile delegation trigger %q: %v", trigger.Pattern, err)
				}
				for _, example := range trigger.ParseExamples {
					gotMatch := re.MatchString(example.Input)
					if gotMatch != example.Matched {
						t.Fatalf("trigger %q match for %q = %v, want %v", trigger.Pattern, example.Input, gotMatch, example.Matched)
					}
					if example.Matched && example.DelegateRoleID != trigger.DelegateRoleID {
						t.Fatalf("trigger delegate mismatch: trigger=%#v example=%#v", trigger, example)
					}
				}
			}
		}
	}
	for _, role := range contract.RoleProfileVersions {
		if role.Version != "1.0.0" || role.Status != "fixture_locked" || role.Termination != "TERMINATE" {
			t.Fatalf("role profile version incomplete: %#v", role)
		}
		if role.ProfileSnapshot.Mission == "" || len(role.ProfileSnapshot.Inputs) == 0 || len(role.ProfileSnapshot.HandoffFields) == 0 {
			t.Fatalf("role profile snapshot incomplete: %#v", role)
		}
		if len(role.VersionHistory) != 1 ||
			role.VersionHistory[0].Version != role.Version ||
			role.VersionHistory[0].Status != role.Status ||
			role.VersionHistory[0].Change == "" {
			t.Fatalf("role profile version history incomplete: %#v", role)
		}
	}
	for _, want := range []string{"section_id", "source_refs", "evidence_validation", "termination"} {
		if !contains(contract.SectionAgentOutput.RequiredFields, want) {
			t.Fatalf("section output required fields missing %q: %#v", want, contract.SectionAgentOutput.RequiredFields)
		}
	}
	if contract.SectionAgentOutput.Termination != "TERMINATE" ||
		contract.TerminationConvention.Literal != "TERMINATE" ||
		!contract.TerminationConvention.RequiredOnComplete {
		t.Fatalf("termination convention incomplete: %#v %#v", contract.SectionAgentOutput, contract.TerminationConvention)
	}
	if contract.SourceEvidenceValidation.MinimumSourceRefs < 1 ||
		!contract.SourceEvidenceValidation.RejectsUnresolvedRefs ||
		!contract.SourceEvidenceValidation.RequiresFixtureKey ||
		!contract.SourceEvidenceValidation.RequiresProviderFree ||
		contract.SourceEvidenceValidation.LiveNetwork {
		t.Fatalf("source evidence validation incomplete: %#v", contract.SourceEvidenceValidation)
	}
	for _, want := range []string{"minimum_source_refs", "no_unresolved_source_refs", "fixture_key_required"} {
		found := false
		for _, rule := range contract.SourceEvidenceValidation.Rules {
			if rule.ID == want && rule.Description != "" {
				found = true
			}
		}
		if !found {
			t.Fatalf("source evidence validation rules missing %q: %#v", want, contract.SourceEvidenceValidation.Rules)
		}
	}
	if len(contract.TerminationConvention.DriftChecks) < 5 {
		t.Fatalf("termination drift checks incomplete: %#v", contract.TerminationConvention)
	}
	for _, check := range contract.TerminationConvention.DriftChecks {
		gotAccepted := check.Output == "TERMINATE"
		if gotAccepted != check.Accepted {
			t.Fatalf("termination drift check %q accepted=%v, want %v", check.Output, gotAccepted, check.Accepted)
		}
	}

	var fixtures struct {
		ProviderFree   bool `json:"provider_free"`
		LiveNetwork    bool `json:"live_network"`
		LiveModelCalls bool `json:"live_model_calls"`
		Fixtures       []struct {
			FixtureKey string         `json:"fixture_key"`
			Capability string         `json:"capability"`
			Schema     string         `json:"schema"`
			Metadata   map[string]any `json:"metadata"`
		} `json:"fixtures"`
		DelegationTriggerParseExamples []struct {
			Catalog        string `json:"catalog"`
			EntryID        string `json:"entry_id"`
			Input          string `json:"input"`
			Matched        bool   `json:"matched"`
			DelegateRoleID string `json:"delegate_role_id"`
		} `json:"delegation_trigger_parse_examples"`
		EvidenceValidationCases []struct {
			ID                   string   `json:"id"`
			ProviderFree         bool     `json:"provider_free"`
			LiveNetwork          bool     `json:"live_network"`
			Validated            bool     `json:"validated"`
			MinimumSourceRefs    int      `json:"minimum_source_refs"`
			SourceRefs           []string `json:"source_refs"`
			UnresolvedSourceRefs []string `json:"unresolved_source_refs"`
			FixtureKeys          []string `json:"fixture_keys"`
			Rules                []struct {
				ID     string `json:"id"`
				Passes bool   `json:"passes"`
			} `json:"rules"`
		} `json:"evidence_validation_cases"`
		SampleSectionOutput struct {
			SectionID          string   `json:"section_id"`
			SourceRefs         []string `json:"source_refs"`
			Termination        string   `json:"termination"`
			EvidenceValidation struct {
				ProviderFree         bool     `json:"provider_free"`
				LiveNetwork          bool     `json:"live_network"`
				Validated            bool     `json:"validated"`
				MinimumSourceRefs    int      `json:"minimum_source_refs"`
				SourceRefs           []string `json:"source_refs"`
				UnresolvedSourceRefs []string `json:"unresolved_source_refs"`
				FixtureKeys          []string `json:"fixture_keys"`
				Rules                []struct {
					ID     string `json:"id"`
					Passes bool   `json:"passes"`
				} `json:"rules"`
			} `json:"evidence_validation"`
		} `json:"sample_section_output"`
	}
	decodeJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &fixtures)
	if !fixtures.ProviderFree || fixtures.LiveNetwork || fixtures.LiveModelCalls || len(fixtures.Fixtures) != 10 {
		t.Fatalf("fixture index header/count = %#v", fixtures)
	}
	for _, fixture := range fixtures.Fixtures {
		if !strings.HasPrefix(fixture.Capability, "prompt.role.") || fixture.Metadata["replay_ready"] != true {
			t.Fatalf("fixture metadata incomplete: %#v", fixture)
		}
	}
	if len(fixtures.DelegationTriggerParseExamples) < 4 {
		t.Fatalf("delegation trigger parse fixtures incomplete: %#v", fixtures.DelegationTriggerParseExamples)
	}
	if len(fixtures.EvidenceValidationCases) != 3 {
		t.Fatalf("evidence validation case count = %d, want 3", len(fixtures.EvidenceValidationCases))
	}
	for _, tc := range fixtures.EvidenceValidationCases {
		gotValidated := tc.ProviderFree &&
			!tc.LiveNetwork &&
			len(tc.SourceRefs) >= tc.MinimumSourceRefs &&
			len(tc.UnresolvedSourceRefs) == 0 &&
			len(tc.FixtureKeys) > 0
		if gotValidated != tc.Validated {
			t.Fatalf("evidence validation case %q validated=%v, want %v", tc.ID, gotValidated, tc.Validated)
		}
		if len(tc.Rules) != 3 {
			t.Fatalf("evidence validation case %q rules = %#v", tc.ID, tc.Rules)
		}
	}
	if fixtures.SampleSectionOutput.Termination != "TERMINATE" ||
		len(fixtures.SampleSectionOutput.SourceRefs) < 1 ||
		!fixtures.SampleSectionOutput.EvidenceValidation.ProviderFree ||
		fixtures.SampleSectionOutput.EvidenceValidation.LiveNetwork ||
		!fixtures.SampleSectionOutput.EvidenceValidation.Validated ||
		len(fixtures.SampleSectionOutput.EvidenceValidation.SourceRefs) < fixtures.SampleSectionOutput.EvidenceValidation.MinimumSourceRefs ||
		len(fixtures.SampleSectionOutput.EvidenceValidation.UnresolvedSourceRefs) != 0 ||
		len(fixtures.SampleSectionOutput.EvidenceValidation.FixtureKeys) == 0 ||
		len(fixtures.SampleSectionOutput.EvidenceValidation.Rules) != 3 {
		t.Fatalf("sample section evidence validation incomplete: %#v", fixtures.SampleSectionOutput)
	}
}

func TestFinRobotPromptRolesLivePackageNoLiveImportsCredentialsOrModelCalls(t *testing.T) {
	base := promptRolesLivePackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(path) != ".json" && filepath.Ext(path) != ".leia" && filepath.Base(path) != "README.md" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		source := string(data)
		for _, pattern := range []string{
			`(?m)^\s*import\s+`,
			`(?m)^\s*use\s+`,
			`(?m)^\s*load\s*\(`,
			`(?m)^\s*require\s*\(`,
			`(?i)api[_-]?key|password|credential_required`,
			`(?i)openai|anthropic|gemini|finnhub|fmp_api|yfinance|requests\.|fetch\(`,
		} {
			if regexp.MustCompile(pattern).FindString(source) != "" {
				return fmt.Errorf("%s contains forbidden live dependency/credential/model pattern %q", path, pattern)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFinRobotPromptRolesLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(promptRolesLivePackageDir(t), "main.leia")

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
			got, err := vm.Get("prompt_role_live_package_summary")
			if err != nil {
				t.Fatalf("Get prompt_role_live_package_summary: %v", err)
			}
			want := "prompt_roles_live_package catalogs=5 roles=4 provider_free=true live_network=false model_calls=false termination=TERMINATE"
			if got != want {
				t.Fatalf("prompt_role_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func promptRolesLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "prompt_roles")
}

func loadPromptRolesLiveManifest(t *testing.T, base string) promptRolesLiveManifest {
	t.Helper()
	var manifest promptRolesLiveManifest
	decodeJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}
