package leia_test

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type promptTemplateDriftCorpus struct {
	SchemaVersion         int      `json:"schema_version"`
	ID                    string   `json:"id"`
	ProviderFree          bool     `json:"provider_free"`
	LiveNetwork           bool     `json:"live_network"`
	RealDependencyImports bool     `json:"real_dependency_imports"`
	LiveModelCalls        bool     `json:"live_model_calls"`
	Capabilities          []string `json:"capabilities"`
	TriggerParsing        []struct {
		ID                     string `json:"id"`
		SourceModule           string `json:"source_module"`
		SourceFileHint         string `json:"source_file_hint"`
		EntryID                string `json:"entry_id"`
		Pattern                string `json:"pattern"`
		Input                  string `json:"input"`
		ExpectedMatch          bool   `json:"expected_match"`
		ExpectedDelegateRoleID string `json:"expected_delegate_role_id"`
	} `json:"trigger_parsing"`
	TemplateSnapshots []struct {
		ID                   string   `json:"id"`
		Kind                 string   `json:"kind"`
		RoleID               string   `json:"role_id"`
		SourceCatalog        string   `json:"source_catalog"`
		RequiredPlaceholders []string `json:"required_placeholders"`
		Template             string   `json:"template"`
		ExpectedSections     []string `json:"expected_sections"`
	} `json:"template_snapshots"`
	EquityAgentsResponseDrift struct {
		ContractName   string            `json:"contract_name"`
		SourceModule   string            `json:"source_module"`
		SourceFileHint string            `json:"source_file_hint"`
		RequiredFields map[string]string `json:"required_fields"`
		Cases          []struct {
			ID             string         `json:"id"`
			ExpectedValid  bool           `json:"expected_valid"`
			ExpectedReason string         `json:"expected_reason"`
			Response       map[string]any `json:"response"`
		} `json:"cases"`
	} `json:"equity_agents_response_drift"`
	SourceEvidenceValidatorCorpus []struct {
		ID                   string   `json:"id"`
		ExpectedValid        bool     `json:"expected_valid"`
		MinimumSourceRefs    int      `json:"minimum_source_refs"`
		SourceRefs           []string `json:"source_refs"`
		UnresolvedSourceRefs []string `json:"unresolved_source_refs"`
		FixtureKeys          []string `json:"fixture_keys"`
		ProviderFree         bool     `json:"provider_free"`
		LiveNetwork          bool     `json:"live_network"`
	} `json:"source_evidence_validator_corpus"`
	TerminationConventionDrift []struct {
		ID             string `json:"id"`
		Output         string `json:"output"`
		ExpectedAccept bool   `json:"expected_accept"`
	} `json:"termination_convention_drift"`
}

func TestFinRobotPromptTemplateDriftCorpus(t *testing.T) {
	corpus := loadPromptTemplateDriftCorpus(t)
	if corpus.SchemaVersion != 1 || corpus.ID == "" {
		t.Fatalf("corpus header incomplete: %#v", corpus)
	}
	if !corpus.ProviderFree || corpus.LiveNetwork || corpus.RealDependencyImports || corpus.LiveModelCalls {
		t.Fatalf("corpus must stay provider-free/offline: %#v", corpus)
	}
	for _, want := range []string{
		"prompt.role.catalog.prompts",
		"prompt.role.catalog.utils",
		"prompt.role.catalog.equity_agents",
		"prompt.role.section.output_schema",
		"prompt.role.termination.convention",
		"prompt.role.evidence.validation",
	} {
		if !contains(corpus.Capabilities, want) {
			t.Fatalf("corpus capabilities missing %q: %#v", want, corpus.Capabilities)
		}
	}

	coveredTriggerSources := map[string]bool{}
	for _, tc := range corpus.TriggerParsing {
		if tc.ID == "" || tc.SourceModule == "" || tc.SourceFileHint == "" || tc.EntryID == "" || tc.Pattern == "" || tc.Input == "" {
			t.Fatalf("trigger case incomplete: %#v", tc)
		}
		re, err := regexp.Compile(tc.Pattern)
		if err != nil {
			t.Fatalf("trigger case %q pattern %q: %v", tc.ID, tc.Pattern, err)
		}
		gotMatch := re.MatchString(tc.Input)
		if gotMatch != tc.ExpectedMatch {
			t.Fatalf("trigger case %q match=%v, want %v", tc.ID, gotMatch, tc.ExpectedMatch)
		}
		if tc.ExpectedMatch && tc.ExpectedDelegateRoleID == "" {
			t.Fatalf("trigger case %q matched without delegate role", tc.ID)
		}
		if !tc.ExpectedMatch && tc.ExpectedDelegateRoleID != "" {
			t.Fatalf("trigger case %q did not match but declares delegate %q", tc.ID, tc.ExpectedDelegateRoleID)
		}
		coveredTriggerSources[tc.SourceFileHint] = true
	}
	for _, want := range []string{"prompts.py", "utils.py"} {
		if !coveredTriggerSources[want] {
			t.Fatalf("trigger parsing corpus missing %s coverage: %#v", want, coveredTriggerSources)
		}
	}

	for _, snapshot := range corpus.TemplateSnapshots {
		if snapshot.ID == "" || snapshot.Kind == "" || snapshot.RoleID == "" || snapshot.SourceCatalog == "" || snapshot.Template == "" {
			t.Fatalf("template snapshot incomplete: %#v", snapshot)
		}
		for _, placeholder := range snapshot.RequiredPlaceholders {
			if !strings.Contains(snapshot.Template, placeholder) {
				t.Fatalf("template snapshot %q missing placeholder %q in %q", snapshot.ID, placeholder, snapshot.Template)
			}
		}
		for _, section := range snapshot.ExpectedSections {
			if !strings.Contains(strings.ToLower(snapshot.Template), strings.ToLower(section)) {
				t.Fatalf("template snapshot %q missing section marker %q in %q", snapshot.ID, section, snapshot.Template)
			}
		}
		if !strings.Contains(snapshot.Template, "TERMINATE") {
			t.Fatalf("template snapshot %q missing TERMINATE convention", snapshot.ID)
		}
	}

	if corpus.EquityAgentsResponseDrift.SourceFileHint != "equity_agents.py" ||
		corpus.EquityAgentsResponseDrift.SourceModule != "FinRobot.equity_agents" {
		t.Fatalf("equity response drift source incomplete: %#v", corpus.EquityAgentsResponseDrift)
	}
	for _, tc := range corpus.EquityAgentsResponseDrift.Cases {
		gotValid, gotReason := validateFixtureResponseShape(tc.Response, corpus.EquityAgentsResponseDrift.RequiredFields)
		if gotValid != tc.ExpectedValid || gotReason != tc.ExpectedReason {
			t.Fatalf("response drift case %q = valid:%v reason:%s, want valid:%v reason:%s", tc.ID, gotValid, gotReason, tc.ExpectedValid, tc.ExpectedReason)
		}
	}

	for _, tc := range corpus.SourceEvidenceValidatorCorpus {
		gotValid := tc.ProviderFree &&
			!tc.LiveNetwork &&
			len(tc.SourceRefs) >= tc.MinimumSourceRefs &&
			len(tc.UnresolvedSourceRefs) == 0 &&
			len(tc.FixtureKeys) > 0
		if gotValid != tc.ExpectedValid {
			t.Fatalf("source evidence case %q valid=%v, want %v", tc.ID, gotValid, tc.ExpectedValid)
		}
	}

	for _, tc := range corpus.TerminationConventionDrift {
		gotAccept := tc.Output == "TERMINATE"
		if gotAccept != tc.ExpectedAccept {
			t.Fatalf("termination drift case %q accepted=%v, want %v", tc.ID, gotAccept, tc.ExpectedAccept)
		}
	}
}

func loadPromptTemplateDriftCorpus(t *testing.T) promptTemplateDriftCorpus {
	t.Helper()
	var corpus promptTemplateDriftCorpus
	decodeJSONFile(t, filepath.Join(promptRolesLivePackageDir(t), "fixtures", "prompt_template_drift_corpus.json"), &corpus)
	return corpus
}

func validateFixtureResponseShape(response map[string]any, requiredFields map[string]string) (bool, string) {
	for field, wantType := range requiredFields {
		value, ok := response[field]
		if !ok {
			return false, "missing_required_field"
		}
		if !fixtureValueMatchesType(value, wantType) {
			return false, "field_type_drift"
		}
	}
	if response["termination"] != "TERMINATE" {
		return false, "termination_drift"
	}
	evidence, ok := response["evidence_validation"].(map[string]any)
	if !ok {
		return false, "field_type_drift"
	}
	if valid, reason := validateFixtureEvidencePayload(evidence); !valid {
		return false, reason
	}
	return true, "matches_required_shape"
}

func fixtureValueMatchesType(value any, wantType string) bool {
	switch wantType {
	case "string":
		_, ok := value.(string)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	default:
		panic(fmt.Sprintf("unsupported fixture type %q", wantType))
	}
}

func validateFixtureEvidencePayload(evidence map[string]any) (bool, string) {
	if evidence["provider_free"] != true || evidence["live_network"] != false {
		return false, "provider_policy_drift"
	}
	sourceRefs, ok := evidence["source_refs"].([]any)
	if !ok {
		return false, "field_type_drift"
	}
	minimumSourceRefs, ok := evidence["minimum_source_refs"].(float64)
	if !ok {
		return false, "field_type_drift"
	}
	unresolvedRefs, ok := evidence["unresolved_source_refs"].([]any)
	if !ok {
		return false, "field_type_drift"
	}
	fixtureKeys, ok := evidence["fixture_keys"].([]any)
	if !ok {
		return false, "field_type_drift"
	}
	if len(sourceRefs) < int(minimumSourceRefs) {
		return false, "source_evidence_drift"
	}
	if len(unresolvedRefs) > 0 {
		return false, "source_evidence_drift"
	}
	if len(fixtureKeys) == 0 {
		return false, "source_evidence_drift"
	}
	return true, "matches_required_shape"
}
