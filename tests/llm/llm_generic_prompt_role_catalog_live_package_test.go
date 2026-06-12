package leia_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericPromptRoleCatalogLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericPromptRoleCatalogPackageDir(t)

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
		Capabilities       []string          `json:"capabilities"`
		Contracts          map[string]string `json:"contracts"`
		Schemas            map[string]string `json:"schemas"`
		Fixtures           map[string]string `json:"fixtures"`
		NoBuiltInGuarantee struct {
			Required  bool   `json:"required"`
			Statement string `json:"statement"`
		} `json:"no_built_in_guarantee"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	if manifest.SchemaVersion != 1 ||
		manifest.ID != "generic-prompt-role-catalog" ||
		manifest.PackageName != "leia-generic-ai-prompt-role-catalog" ||
		manifest.PackageBoundaryID != "generic-ai-prompt-role-catalog" ||
		manifest.CapabilityID != "generic.ai.prompt.role.catalog" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetworkDefault || manifest.LiveModelDefault || manifest.DependsOnQRuntime {
		t.Fatalf("manifest must stay provider-free/generic/offline: %#v", manifest)
	}
	statement := strings.ToLower(manifest.NoBuiltInGuarantee.Statement)
	if !manifest.NoBuiltInGuarantee.Required ||
		!strings.Contains(statement, "leia core") ||
		!strings.Contains(statement, "does not provide") ||
		!strings.Contains(statement, "built-in") ||
		!strings.Contains(statement, manifest.PackageName) ||
		!strings.Contains(statement, "package boundary") {
		t.Fatalf("manifest missing no-built-in boundary: %#v", manifest.NoBuiltInGuarantee)
	}
	for _, want := range []string{
		"generic.ai.prompt.role.catalog",
		"generic.ai.prompt.template.snapshot",
		"generic.ai.role.profile.version",
		"generic.ai.role.delegation.trigger",
		"generic.ai.role.output.schema",
		"generic.ai.role.evidence.validation",
		"generic.ai.role.termination.convention",
		"generic.ai.prompt.drift.fixture",
	} {
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
		LiveModel             bool              `json:"live_model"`
		LiveModelCalls        bool              `json:"live_model_calls"`
		RealDependencyImports bool              `json:"real_dependency_imports"`
		RequiresCredentials   bool              `json:"requires_credentials"`
		ProviderSDKsRequired  bool              `json:"provider_sdks_required"`
		Capabilities          []string          `json:"capabilities"`
		FieldContracts        map[string]string `json:"field_contracts"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Contracts["contract"]), &contract)
	if contract.SchemaVersion != 1 || contract.PackageBoundaryID != manifest.PackageBoundaryID ||
		contract.PackageName != "generic.ai.prompt.role.catalog" || contract.Entrypoint != "ai.prompt.role_catalog" ||
		!contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork || contract.LiveModel ||
		contract.LiveModelCalls || contract.RealDependencyImports || contract.RequiresCredentials || contract.ProviderSDKsRequired {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	for _, want := range []string{"role_profile_versions", "prompt_template_snapshots", "delegation_triggers", "evidence_validation", "termination_convention"} {
		if contract.FieldContracts[want] == "" {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
		}
	}

	var index struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Fixtures              []struct {
			FixtureKey string         `json:"fixture_key"`
			Capability string         `json:"capability"`
			Path       string         `json:"path"`
			Schema     string         `json:"schema"`
			Metadata   map[string]any `json:"metadata"`
		} `json:"fixtures"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Fixtures["index"]), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 1 {
		t.Fatalf("fixture index header/count mismatch: %#v", index)
	}
	fixture := index.Fixtures[0]
	if fixture.FixtureKey != "generic:prompt_role_catalog:offline" ||
		fixture.Capability != "generic.ai.prompt.role.catalog" ||
		fixture.Path != manifest.Fixtures["prompt_role_catalog"] ||
		fixture.Schema != manifest.Schemas["prompt_role_catalog"] ||
		fixture.Metadata["replay_ready"] != true {
		t.Fatalf("fixture index entry mismatch: %#v", fixture)
	}
}

func TestGenericPromptRoleCatalogLivePackageFixtureShape(t *testing.T) {
	base := genericPromptRoleCatalogPackageDir(t)
	fixture := loadGenericPromptRoleCatalogFixture(t, filepath.Join(base, "fixtures", "generic_prompt_role_catalog_fixture.json"))
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.LiveModelCalls {
		t.Fatalf("fixture must stay provider-free and offline: %#v", fixture)
	}
	if fixture.Catalog.CatalogID == "" || fixture.Catalog.DefaultTermination != "COMPLETE" {
		t.Fatalf("catalog header mismatch: %#v", fixture.Catalog)
	}
	if len(fixture.RoleProfiles) != 3 || len(fixture.PromptTemplates) != 3 || len(fixture.DelegationTriggers) != 4 || len(fixture.EvidenceValidationCases) != 3 {
		t.Fatalf("fixture counts drifted: roles=%d templates=%d triggers=%d evidence=%d",
			len(fixture.RoleProfiles), len(fixture.PromptTemplates), len(fixture.DelegationTriggers), len(fixture.EvidenceValidationCases))
	}

	roles := map[string]bool{}
	for _, role := range fixture.RoleProfiles {
		if role.RoleID == "" || role.Version == "" || role.OutputSchema == "" || role.Termination != fixture.Catalog.DefaultTermination {
			t.Fatalf("role profile incomplete: %#v", role)
		}
		roles[role.RoleID] = true
	}
	for _, template := range fixture.PromptTemplates {
		if !roles[template.RoleID] || template.EntryID == "" || template.Template == "" || len(template.RequiredPlaceholders) == 0 {
			t.Fatalf("prompt template does not resolve to a role or is incomplete: %#v", template)
		}
	}
	for _, trigger := range fixture.DelegationTriggers {
		if trigger.Matched && !roles[trigger.DelegateRoleID] {
			t.Fatalf("delegation trigger target role does not resolve: %#v", trigger)
		}
		if !trigger.Matched && trigger.DelegateRoleID != "" {
			t.Fatalf("non-match trigger must not name delegate role: %#v", trigger)
		}
	}
	for _, evidence := range fixture.EvidenceValidationCases {
		if evidence.MinimumSourceRefs < 1 || len(evidence.Rules) == 0 {
			t.Fatalf("evidence validation case incomplete: %#v", evidence)
		}
	}
	for _, boundary := range fixture.AdapterBoundaries {
		if boundary.DependencyImported || boundary.CredentialRequired || boundary.LiveNetwork || !boundary.CleanSkip {
			t.Fatalf("adapter boundary must be provider-free clean-skip: %#v", boundary)
		}
	}
}

func TestGenericPromptRoleCatalogLivePackageIsDomainNeutral(t *testing.T) {
	base := genericPromptRoleCatalogPackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"finrobot", "acme", "ticker", "equity", "sec.gov", "10-k", "market_analyst", "expert_investor", "report_section_writer"} {
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

func TestGenericPromptRoleCatalogLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericPromptRoleCatalogPackageDir(t)
	schema := filepath.Join(base, "schemas", "generic_prompt_role_catalog_v1.schema.json")
	assertDocumentPipelineSchemaRequired(t, schema, []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "live_model_calls", "catalog", "role_profiles", "prompt_templates", "delegation_triggers", "output_schema", "evidence_validation_cases", "termination_convention", "adapter_boundaries"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "catalog"}, []string{"catalog_id", "fixture_key", "default_termination", "domain_identifiers"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "role_profiles", "items"}, []string{"role_id", "version", "status", "instructions", "tool_policy", "output_schema", "termination"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "prompt_templates", "items"}, []string{"entry_id", "role_id", "kind", "template", "required_placeholders", "constraints", "expected_sections"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "delegation_triggers", "items"}, []string{"entry_id", "pattern", "input", "matched", "delegate_role_id", "reason"})
}

func TestGenericPromptRoleCatalogLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericPromptRoleCatalogPackageDir(t), "main.leia")
	want := "generic_prompt_role_catalog_live_package capability=generic.ai.prompt.role.catalog entrypoint=ai.prompt.role_catalog roles=3 templates=3 delegation=4 evidence=3 termination=COMPLETE provider_free=true live_network=false imports=false model_calls=false"
	for _, result := range runFinRobotLivePackageSummarySmoke(t, path, "generic_prompt_role_catalog_live_package_summary", "generic_prompt_role_catalog_live_package", leia.LibString) {
		if result.Summary != want {
			t.Fatalf("summary = %#v, want %#v", result.Summary, want)
		}
		fields := result.Fields
		requireFinRobotSummaryFields(t, fields, "capability", "entrypoint", "roles", "templates", "delegation", "evidence", "termination", "provider_free", "live_network", "imports", "model_calls")
		if fields["capability"] != "generic.ai.prompt.role.catalog" ||
			fields["entrypoint"] != "ai.prompt.role_catalog" ||
			fields["roles"] != "3" ||
			fields["templates"] != "3" ||
			fields["delegation"] != "4" ||
			fields["evidence"] != "3" ||
			fields["termination"] != "COMPLETE" ||
			fields["provider_free"] != "true" ||
			fields["live_network"] != "false" ||
			fields["imports"] != "false" ||
			fields["model_calls"] != "false" {
			t.Fatalf("summary fields = %#v", fields)
		}
	}
}

type genericPromptRoleCatalogFixture struct {
	ProviderFree          bool `json:"provider_free"`
	LiveNetwork           bool `json:"live_network"`
	RealDependencyImports bool `json:"real_dependency_imports"`
	LiveModelCalls        bool `json:"live_model_calls"`
	Catalog               struct {
		CatalogID          string `json:"catalog_id"`
		DefaultTermination string `json:"default_termination"`
	} `json:"catalog"`
	RoleProfiles []struct {
		RoleID       string `json:"role_id"`
		Version      string `json:"version"`
		OutputSchema string `json:"output_schema"`
		Termination  string `json:"termination"`
	} `json:"role_profiles"`
	PromptTemplates []struct {
		EntryID              string   `json:"entry_id"`
		RoleID               string   `json:"role_id"`
		Template             string   `json:"template"`
		RequiredPlaceholders []string `json:"required_placeholders"`
	} `json:"prompt_templates"`
	DelegationTriggers []struct {
		Matched        bool   `json:"matched"`
		DelegateRoleID string `json:"delegate_role_id"`
	} `json:"delegation_triggers"`
	EvidenceValidationCases []struct {
		MinimumSourceRefs int `json:"minimum_source_refs"`
		Rules             []struct {
			RuleID string `json:"rule_id"`
			Passes bool   `json:"passes"`
		} `json:"rules"`
	} `json:"evidence_validation_cases"`
	AdapterBoundaries []struct {
		ID                 string `json:"id"`
		DependencyImported bool   `json:"dependency_imported"`
		CredentialRequired bool   `json:"credential_required"`
		LiveNetwork        bool   `json:"live_network"`
		CleanSkip          bool   `json:"clean_skip"`
	} `json:"adapter_boundaries"`
}

func loadGenericPromptRoleCatalogFixture(t *testing.T, path string) genericPromptRoleCatalogFixture {
	t.Helper()
	var fixture genericPromptRoleCatalogFixture
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func genericPromptRoleCatalogPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_prompt_role_catalog")
}
