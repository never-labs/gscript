package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	leia "github.com/never-labs/leia"
)

type analyzerReportLiveManifest struct {
	SchemaVersion               int      `json:"schema_version"`
	ID                          string   `json:"id"`
	PackageName                 string   `json:"package_name"`
	ProviderFree                bool     `json:"provider_free"`
	LiveNetworkDefault          bool     `json:"live_network_default"`
	RealDependencyImportDefault bool     `json:"real_dependency_import_default"`
	SourceModules               []string `json:"source_modules"`
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
	Entrypoints               map[string]string                 `json:"entrypoints"`
	Schemas                   map[string]string                 `json:"schemas"`
	Fixtures                  map[string]string                 `json:"fixtures"`
	Capabilities              []string                          `json:"capabilities"`
	PromptCatalogs            []analyzerPromptCatalog           `json:"prompt_catalogs"`
	SectionSchemas            []analyzerSectionSchema           `json:"section_schemas"`
	SectionSchemaRegistry     analyzerSectionSchemaRegistry     `json:"section_schema_registry"`
	EvidenceRules             analyzerEvidenceRules             `json:"evidence_rules"`
	EvidenceRequirementMatrix analyzerEvidenceRequirementMatrix `json:"evidence_requirement_matrix"`
	CitationRequirements      struct {
		Capability                 string                        `json:"capability"`
		RequiredFields             []string                      `json:"required_fields"`
		InlineCitationFormat       string                        `json:"inline_citation_format"`
		NormalizedCitationFormat   string                        `json:"normalized_citation_format"`
		Normalization              analyzerCitationNormalization `json:"normalization"`
		AllClaimsRequireSourceRefs bool                          `json:"all_claims_require_source_refs"`
		UnresolvedRefsAllowed      bool                          `json:"unresolved_refs_allowed"`
	} `json:"citation_requirements"`
	SourceFreshnessPolicy      analyzerSourceFreshnessPolicy      `json:"source_freshness_policy"`
	AnalystRolePromptEnvelope  analyzerAnalystRolePromptEnvelope  `json:"analyst_role_prompt_envelope"`
	MissingEvidenceDegradation analyzerMissingEvidenceDegradation `json:"missing_evidence_degradation"`
	OutputEnvelopes            []analyzerOutputEnvelope           `json:"output_envelopes"`
	Replay                     struct {
		ExpectedSectionsFixture string   `json:"expected_sections_fixture"`
		ExpectedSectionIDs      []string `json:"expected_section_ids"`
		DeterministicOrder      bool     `json:"deterministic_order"`
		ProviderFree            bool     `json:"provider_free"`
		LiveNetwork             bool     `json:"live_network"`
	} `json:"replay"`
	TestGates []string `json:"test_gates"`
}

type analyzerPromptCatalog struct {
	ID         string `json:"id"`
	SourcePath string `json:"source_path"`
	Capability string `json:"capability"`
	FixtureKey string `json:"fixture_key"`
	Schema     string `json:"schema"`
}

type analyzerSectionSchema struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Capability    string `json:"capability"`
	Schema        string `json:"schema"`
	MinSourceRefs int    `json:"min_source_refs"`
	Envelope      string `json:"envelope"`
}

type analyzerEvidenceRules struct {
	Capability                  string `json:"capability"`
	Schema                      string `json:"schema"`
	MinimumSourceRefsPerSection int    `json:"minimum_source_refs_per_section"`
	RejectsUnresolvedRefs       bool   `json:"rejects_unresolved_refs"`
	RequiresSourceQuoteOrMetric bool   `json:"requires_source_quote_or_metric"`
	RequiresFixtureKey          bool   `json:"requires_fixture_key"`
	RequiresProviderFree        bool   `json:"requires_provider_free"`
	LiveNetwork                 bool   `json:"live_network"`
}

type analyzerOutputEnvelope struct {
	ID             string   `json:"id"`
	SectionID      string   `json:"section_id"`
	RequiredFields []string `json:"required_fields"`
}

type analyzerSectionSchemaRegistry struct {
	Capability         string   `json:"capability"`
	Schema             string   `json:"schema"`
	RegistryVersion    string   `json:"registry_version"`
	DeterministicOrder []string `json:"deterministic_order"`
	Sections           []struct {
		SectionID             string   `json:"section_id"`
		FixtureKey            string   `json:"fixture_key"`
		PromptID              string   `json:"prompt_id"`
		OutputEnvelope        string   `json:"output_envelope"`
		RequiredEvidenceKinds []string `json:"required_evidence_kinds"`
	} `json:"sections"`
}

type analyzerEvidenceRequirementMatrix struct {
	Capability                  string `json:"capability"`
	Schema                      string `json:"schema"`
	MinimumSourceRefsPerSection int    `json:"minimum_source_refs_per_section"`
	Rows                        []struct {
		SectionID             string   `json:"section_id"`
		RequiredEvidenceKinds []string `json:"required_evidence_kinds"`
		RequiredFields        []string `json:"required_fields"`
		AllowMissingEvidence  bool     `json:"allow_missing_evidence"`
		CleanDegradationMode  string   `json:"clean_degradation_mode"`
	} `json:"rows"`
}

type analyzerCitationNormalization struct {
	Capability                     string `json:"capability"`
	SourceIDCase                   string `json:"source_id_case"`
	StripQueryFragmentsFromLocator bool   `json:"strip_query_fragments_from_locator"`
	DedupeRepeatedRefs             bool   `json:"dedupe_repeated_refs"`
	PreserveFirstSeenOrder         bool   `json:"preserve_first_seen_order"`
}

type analyzerSourceFreshnessPolicy struct {
	Capability                  string         `json:"capability"`
	AsOf                        string         `json:"as_of"`
	StaleAfterDaysByKind        map[string]int `json:"stale_after_days_by_kind"`
	WarningLevelForStaleSources string         `json:"warning_level_for_stale_sources"`
	BlockOnStaleSource          bool           `json:"block_on_stale_source"`
}

type analyzerAnalystRolePromptEnvelope struct {
	Capability          string   `json:"capability"`
	Role                string   `json:"role"`
	Audience            string   `json:"audience"`
	Tone                string   `json:"tone"`
	MustInclude         []string `json:"must_include"`
	MustNotInclude      []string `json:"must_not_include"`
	FallbackInstruction string   `json:"fallback_instruction"`
}

type analyzerMissingEvidenceDegradation struct {
	Capability           string   `json:"capability"`
	FixtureKey           string   `json:"fixture_key"`
	Mode                 string   `json:"mode"`
	ProviderFree         bool     `json:"provider_free"`
	LiveNetwork          bool     `json:"live_network"`
	ExpectedValidated    bool     `json:"expected_validated"`
	RequiredWarningCodes []string `json:"required_warning_codes"`
}

func TestFinRobotAnalyzerReportLivePackageManifestCatalogSchemasAndRules(t *testing.T) {
	base := analyzerReportLivePackageDir(t)
	manifest := loadAnalyzerReportLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-analyzer-report-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-analyzer-report" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		manifest.DefaultPolicy.LiveModelCalls ||
		manifest.DefaultPolicy.FixtureHook != "recorded_analyzer_report_live_fixture" {
		t.Fatalf("default policy must stay fixture-only: %#v", manifest.DefaultPolicy)
	}
	for _, want := range []string{
		"finrobot/functional/analyzer.py",
		"finrobot/functional/analyzer/*.py",
		"finrobot/functional/prompts/finance_analysis.py",
	} {
		if !contains(manifest.SourceModules, want) {
			t.Fatalf("source modules missing %q: %#v", want, manifest.SourceModules)
		}
	}
	for _, key := range []string{"smoke", "analyzer_report_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"prompt_catalog", "section_schema", "source_evidence_rule", "report_envelope", "expected_section"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "expected_sections"} {
		path := manifest.Fixtures[key]
		if path == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertJSONFile(t, filepath.Join(base, path))
	}

	wantCapabilities := []string{
		"finance.analyzer.report.orchestrate",
		"finance.analyzer.section.schema",
		"finance.analyzer.section.schema.registry",
		"finance.analyzer.prompt.catalog",
		"finance.analyzer.evidence.enforce",
		"finance.analyzer.evidence.matrix",
		"finance.analyzer.citation.require",
		"finance.analyzer.citation.normalize",
		"finance.analyzer.source.freshness.warn",
		"finance.analyzer.prompt.analyst_role.envelope",
		"finance.analyzer.evidence.degrade_clean",
		"finance.analyzer.output.risk",
		"finance.analyzer.output.thesis",
		"finance.analyzer.output.valuation",
		"finance.analyzer.output.news",
		"finance.analyzer.expected_sections.replay",
	}
	for _, want := range wantCapabilities {
		if !contains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}

	if len(manifest.PromptCatalogs) != 2 {
		t.Fatalf("prompt catalogs = %d, want 2", len(manifest.PromptCatalogs))
	}
	for _, catalog := range manifest.PromptCatalogs {
		if !strings.HasPrefix(catalog.SourcePath, "finrobot.functional.") ||
			!strings.HasPrefix(catalog.FixtureKey, "prompt_catalog:") ||
			catalog.Schema != "prompt_catalog_v1" ||
			catalog.Capability == "" {
			t.Fatalf("catalog metadata incomplete: %#v", catalog)
		}
	}

	assertAnalyzerSectionsAndEnvelopes(t, manifest.SectionSchemas, manifest.OutputEnvelopes)
	assertAnalyzerSchemaRegistryAndEvidenceMatrix(t, manifest.SectionSchemaRegistry, manifest.EvidenceRequirementMatrix)
	if manifest.EvidenceRules.MinimumSourceRefsPerSection < 2 ||
		!manifest.EvidenceRules.RejectsUnresolvedRefs ||
		!manifest.EvidenceRules.RequiresSourceQuoteOrMetric ||
		!manifest.EvidenceRules.RequiresFixtureKey ||
		!manifest.EvidenceRules.RequiresProviderFree ||
		manifest.EvidenceRules.LiveNetwork {
		t.Fatalf("evidence rules are not strict enough: %#v", manifest.EvidenceRules)
	}
	for _, want := range []string{"source_id", "title", "publisher", "published_at", "retrieved_at", "locator", "evidence_hash"} {
		if !contains(manifest.CitationRequirements.RequiredFields, want) {
			t.Fatalf("citation requirements missing %q: %#v", want, manifest.CitationRequirements.RequiredFields)
		}
	}
	if manifest.CitationRequirements.InlineCitationFormat != "[source:{source_id}]" ||
		manifest.CitationRequirements.NormalizedCitationFormat != "[source:{normalized_source_id}]" ||
		manifest.CitationRequirements.Normalization.Capability != "finance.analyzer.citation.normalize" ||
		manifest.CitationRequirements.Normalization.SourceIDCase != "lower_snake" ||
		!manifest.CitationRequirements.Normalization.DedupeRepeatedRefs ||
		!manifest.CitationRequirements.Normalization.PreserveFirstSeenOrder ||
		!manifest.CitationRequirements.AllClaimsRequireSourceRefs ||
		manifest.CitationRequirements.UnresolvedRefsAllowed {
		t.Fatalf("citation requirements incomplete: %#v", manifest.CitationRequirements)
	}
	assertAnalyzerSourceFreshnessPolicy(t, manifest.SourceFreshnessPolicy)
	assertAnalyzerAnalystRolePromptEnvelope(t, manifest.AnalystRolePromptEnvelope)
	if manifest.MissingEvidenceDegradation.Capability != "finance.analyzer.evidence.degrade_clean" ||
		manifest.MissingEvidenceDegradation.FixtureKey != "analyzer_report:ACME:missing-evidence:offline:v1" ||
		manifest.MissingEvidenceDegradation.Mode != "clean_degradation" ||
		!manifest.MissingEvidenceDegradation.ProviderFree ||
		manifest.MissingEvidenceDegradation.LiveNetwork ||
		manifest.MissingEvidenceDegradation.ExpectedValidated ||
		!contains(manifest.MissingEvidenceDegradation.RequiredWarningCodes, "missing_required_source_ref") ||
		!contains(manifest.MissingEvidenceDegradation.RequiredWarningCodes, "claim_omitted_due_to_missing_evidence") {
		t.Fatalf("missing evidence degradation incomplete: %#v", manifest.MissingEvidenceDegradation)
	}
	if !reflect.DeepEqual(manifest.Replay.ExpectedSectionIDs, []string{"risk", "thesis", "valuation", "news"}) ||
		!manifest.Replay.DeterministicOrder ||
		!manifest.Replay.ProviderFree ||
		manifest.Replay.LiveNetwork {
		t.Fatalf("replay contract incomplete: %#v", manifest.Replay)
	}
}

func TestFinRobotAnalyzerReportLivePackageContractAndReplayFixtures(t *testing.T) {
	base := analyzerReportLivePackageDir(t)

	var contract struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		LiveModelCalls        bool `json:"live_model_calls"`
		PromptCatalog         struct {
			ID         string `json:"id"`
			Version    string `json:"version"`
			Capability string `json:"capability"`
			Entries    []struct {
				ID                string `json:"id"`
				SectionID         string `json:"section_id"`
				RequiresEvidence  bool   `json:"requires_evidence"`
				RequiresCitations bool   `json:"requires_citations"`
			} `json:"entries"`
		} `json:"prompt_catalog"`
		SectionSchemas []struct {
			ID             string   `json:"id"`
			OutputEnvelope string   `json:"output_envelope"`
			RequiredFields []string `json:"required_fields"`
		} `json:"section_schemas"`
		SectionSchemaRegistry     analyzerSectionSchemaRegistry     `json:"section_schema_registry"`
		SourceEvidenceEnforcement analyzerEvidenceRules             `json:"source_evidence_enforcement"`
		EvidenceRequirementMatrix analyzerEvidenceRequirementMatrix `json:"evidence_requirement_matrix"`
		CitationRequirements      struct {
			Capability                 string                        `json:"capability"`
			RequiredFields             []string                      `json:"required_fields"`
			InlineCitationFormat       string                        `json:"inline_citation_format"`
			NormalizedCitationFormat   string                        `json:"normalized_citation_format"`
			Normalization              analyzerCitationNormalization `json:"normalization"`
			AllClaimsRequireSourceRefs bool                          `json:"all_claims_require_source_refs"`
			UnresolvedRefsAllowed      bool                          `json:"unresolved_refs_allowed"`
		} `json:"citation_requirements"`
		SourceFreshnessPolicy      analyzerSourceFreshnessPolicy      `json:"source_freshness_policy"`
		AnalystRolePromptEnvelope  analyzerAnalystRolePromptEnvelope  `json:"analyst_role_prompt_envelope"`
		MissingEvidenceDegradation analyzerMissingEvidenceDegradation `json:"missing_evidence_degradation"`
		ReplayContract             struct {
			FixtureKey                 string   `json:"fixture_key"`
			ExpectedSections           []string `json:"expected_sections"`
			DeterministicOrder         bool     `json:"deterministic_order"`
			RequiresResolvedSourceRefs bool     `json:"requires_resolved_source_refs"`
			ProviderFree               bool     `json:"provider_free"`
			LiveNetwork                bool     `json:"live_network"`
		} `json:"replay_contract"`
	}
	decodeJSONFile(t, filepath.Join(base, "contracts", "analyzer_report_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || contract.LiveModelCalls {
		t.Fatalf("contract must be provider-free and offline: %#v", contract)
	}
	if contract.PromptCatalog.ID != "finance_analysis_prompt_library" || contract.PromptCatalog.Version != "1.0.0" || len(contract.PromptCatalog.Entries) != 4 {
		t.Fatalf("prompt catalog incomplete: %#v", contract.PromptCatalog)
	}
	for _, entry := range contract.PromptCatalog.Entries {
		if entry.ID == "" || entry.SectionID == "" || !entry.RequiresEvidence || !entry.RequiresCitations {
			t.Fatalf("prompt catalog entry missing evidence/citation requirements: %#v", entry)
		}
	}
	if len(contract.SectionSchemas) != 4 {
		t.Fatalf("contract section schemas = %d, want 4", len(contract.SectionSchemas))
	}
	for _, section := range contract.SectionSchemas {
		for _, want := range []string{"section_id", "summary", "source_refs", "citations", "evidence_validation"} {
			if !contains(section.RequiredFields, want) {
				t.Fatalf("%s required fields missing %q: %#v", section.ID, want, section.RequiredFields)
			}
		}
	}
	assertAnalyzerSchemaRegistryAndEvidenceMatrix(t, contract.SectionSchemaRegistry, contract.EvidenceRequirementMatrix)
	if contract.SourceEvidenceEnforcement.MinimumSourceRefsPerSection != 2 ||
		!contract.SourceEvidenceEnforcement.RejectsUnresolvedRefs ||
		!contract.SourceEvidenceEnforcement.RequiresSourceQuoteOrMetric ||
		contract.SourceEvidenceEnforcement.LiveNetwork {
		t.Fatalf("source evidence enforcement incomplete: %#v", contract.SourceEvidenceEnforcement)
	}
	if contract.CitationRequirements.InlineCitationFormat != "[source:{source_id}]" ||
		contract.CitationRequirements.NormalizedCitationFormat != "[source:{normalized_source_id}]" ||
		contract.CitationRequirements.Normalization.Capability != "finance.analyzer.citation.normalize" ||
		contract.CitationRequirements.Normalization.SourceIDCase != "lower_snake" ||
		!contract.CitationRequirements.Normalization.DedupeRepeatedRefs ||
		!contract.CitationRequirements.Normalization.PreserveFirstSeenOrder ||
		!contract.CitationRequirements.AllClaimsRequireSourceRefs ||
		contract.CitationRequirements.UnresolvedRefsAllowed {
		t.Fatalf("contract citation normalization incomplete: %#v", contract.CitationRequirements)
	}
	assertAnalyzerSourceFreshnessPolicy(t, contract.SourceFreshnessPolicy)
	assertAnalyzerAnalystRolePromptEnvelope(t, contract.AnalystRolePromptEnvelope)
	if contract.MissingEvidenceDegradation.FixtureKey != "analyzer_report:ACME:missing-evidence:offline:v1" ||
		contract.MissingEvidenceDegradation.ExpectedValidated ||
		!contract.MissingEvidenceDegradation.ProviderFree ||
		contract.MissingEvidenceDegradation.LiveNetwork {
		t.Fatalf("contract missing evidence degradation incomplete: %#v", contract.MissingEvidenceDegradation)
	}
	if contract.ReplayContract.FixtureKey != "analyzer_report:ACME:offline:v1" ||
		!reflect.DeepEqual(contract.ReplayContract.ExpectedSections, []string{"risk", "thesis", "valuation", "news"}) ||
		!contract.ReplayContract.DeterministicOrder ||
		!contract.ReplayContract.RequiresResolvedSourceRefs ||
		!contract.ReplayContract.ProviderFree ||
		contract.ReplayContract.LiveNetwork {
		t.Fatalf("replay contract incomplete: %#v", contract.ReplayContract)
	}

	var fixture struct {
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
		SourceIndex  []struct {
			SourceID           string `json:"source_id"`
			NormalizedSourceID string `json:"normalized_source_id"`
			Kind               string `json:"kind"`
			Title              string `json:"title"`
			Publisher          string `json:"publisher"`
			PublishedAt        string `json:"published_at"`
			RetrievedAt        string `json:"retrieved_at"`
			Locator            string `json:"locator"`
			EvidenceHash       string `json:"evidence_hash"`
		} `json:"source_index"`
		ExpectedSections []struct {
			SectionID               string   `json:"section_id"`
			OutputEnvelope          string   `json:"output_envelope"`
			SourceRefs              []string `json:"source_refs"`
			Citations               []string `json:"citations"`
			NormalizedCitations     []string `json:"normalized_citations"`
			SourceFreshnessWarnings []struct {
				SourceID    string `json:"source_id"`
				WarningCode string `json:"warning_code"`
				Message     string `json:"message"`
			} `json:"source_freshness_warnings"`
			EvidenceValidation struct {
				ProviderFree         bool     `json:"provider_free"`
				LiveNetwork          bool     `json:"live_network"`
				Validated            bool     `json:"validated"`
				UnresolvedSourceRefs []string `json:"unresolved_source_refs"`
				MissingSourceRefs    []string `json:"missing_source_refs"`
				OmittedClaims        []string `json:"omitted_claims"`
				FixtureKey           string   `json:"fixture_key"`
			} `json:"evidence_validation"`
			Payload map[string]any `json:"payload"`
		} `json:"expected_sections"`
		MissingEvidenceDegradation []struct {
			SectionID               string   `json:"section_id"`
			OutputEnvelope          string   `json:"output_envelope"`
			SourceRefs              []string `json:"source_refs"`
			Citations               []string `json:"citations"`
			NormalizedCitations     []string `json:"normalized_citations"`
			SourceFreshnessWarnings []struct {
				SourceID    string `json:"source_id"`
				WarningCode string `json:"warning_code"`
				Message     string `json:"message"`
			} `json:"source_freshness_warnings"`
			EvidenceValidation struct {
				ProviderFree         bool     `json:"provider_free"`
				LiveNetwork          bool     `json:"live_network"`
				Validated            bool     `json:"validated"`
				UnresolvedSourceRefs []string `json:"unresolved_source_refs"`
				MissingSourceRefs    []string `json:"missing_source_refs"`
				OmittedClaims        []string `json:"omitted_claims"`
				WarningCodes         []string `json:"warning_codes"`
				FixtureKey           string   `json:"fixture_key"`
			} `json:"evidence_validation"`
			Payload map[string]any `json:"payload"`
		} `json:"missing_evidence_degradation"`
	}
	decodeJSONFile(t, filepath.Join(base, "fixtures", "expected_sections_ACME_fixture.json"), &fixture)
	if !fixture.ProviderFree || fixture.LiveNetwork || len(fixture.SourceIndex) < 4 || len(fixture.ExpectedSections) != 4 {
		t.Fatalf("fixture header/count incomplete: %#v", fixture)
	}
	sourceIDs := map[string]bool{}
	for _, source := range fixture.SourceIndex {
		if source.SourceID == "" || source.NormalizedSourceID == "" || source.Kind == "" || source.Title == "" || source.Publisher == "" || source.PublishedAt == "" || source.RetrievedAt == "" || source.Locator == "" || source.EvidenceHash == "" {
			t.Fatalf("source citation metadata incomplete: %#v", source)
		}
		if source.NormalizedSourceID != normalizeAnalyzerSourceID(source.SourceID) {
			t.Fatalf("source normalized id mismatch: %#v", source)
		}
		sourceIDs[source.SourceID] = true
	}
	var gotSections []string
	for _, section := range fixture.ExpectedSections {
		gotSections = append(gotSections, section.SectionID)
		if len(section.SourceRefs) < 2 || len(section.Citations) < 2 || len(section.NormalizedCitations) < 2 || len(section.Payload) == 0 {
			t.Fatalf("section replay payload incomplete: %#v", section)
		}
		if !section.EvidenceValidation.ProviderFree ||
			section.EvidenceValidation.LiveNetwork ||
			!section.EvidenceValidation.Validated ||
			len(section.EvidenceValidation.UnresolvedSourceRefs) != 0 ||
			section.EvidenceValidation.FixtureKey == "" {
			t.Fatalf("section evidence validation incomplete: %#v", section.EvidenceValidation)
		}
		if len(section.SourceFreshnessWarnings) != 0 {
			t.Fatalf("%s should not warn for fresh replay sources: %#v", section.SectionID, section.SourceFreshnessWarnings)
		}
		for _, ref := range section.SourceRefs {
			if !sourceIDs[ref] {
				t.Fatalf("%s has unresolved source ref %q", section.SectionID, ref)
			}
			if !contains(section.Citations, "[source:"+ref+"]") {
				t.Fatalf("%s missing citation for source ref %q: %#v", section.SectionID, ref, section.Citations)
			}
			if !contains(section.NormalizedCitations, "[source:"+normalizeAnalyzerSourceID(ref)+"]") {
				t.Fatalf("%s missing normalized citation for source ref %q: %#v", section.SectionID, ref, section.NormalizedCitations)
			}
		}
	}
	if !reflect.DeepEqual(gotSections, []string{"risk", "thesis", "valuation", "news"}) {
		t.Fatalf("expected section order = %#v", gotSections)
	}
	if len(fixture.MissingEvidenceDegradation) != 1 {
		t.Fatalf("missing evidence degradation examples = %d, want 1", len(fixture.MissingEvidenceDegradation))
	}
	degraded := fixture.MissingEvidenceDegradation[0]
	if degraded.SectionID != "valuation" ||
		degraded.OutputEnvelope != "valuation_output_v1" ||
		len(degraded.SourceRefs) != 1 ||
		!sourceIDs[degraded.SourceRefs[0]] ||
		!degraded.EvidenceValidation.ProviderFree ||
		degraded.EvidenceValidation.LiveNetwork ||
		degraded.EvidenceValidation.Validated ||
		len(degraded.EvidenceValidation.UnresolvedSourceRefs) != 0 ||
		!contains(degraded.EvidenceValidation.MissingSourceRefs, "earnings_transcript") ||
		!contains(degraded.EvidenceValidation.OmittedClaims, "target_price") ||
		!contains(degraded.EvidenceValidation.WarningCodes, "missing_required_source_ref") ||
		!contains(degraded.EvidenceValidation.WarningCodes, "claim_omitted_due_to_missing_evidence") ||
		degraded.EvidenceValidation.FixtureKey != "analyzer_report:ACME:missing-evidence:offline:v1" {
		t.Fatalf("missing evidence degradation is not clean: %#v", degraded)
	}
	if degraded.Payload["target_price"] != nil {
		t.Fatalf("degraded valuation target_price must be omitted/null: %#v", degraded.Payload)
	}
}

func TestFinRobotAnalyzerReportLivePackageDeepAnalyzerCoverage(t *testing.T) {
	base := analyzerReportLivePackageDir(t)
	manifest := loadAnalyzerReportLiveManifest(t, base)

	var fixture struct {
		AsOf        string `json:"as_of"`
		SourceIndex []struct {
			SourceID    string `json:"source_id"`
			Kind        string `json:"kind"`
			PublishedAt string `json:"published_at"`
		} `json:"source_index"`
		ExpectedSections []struct {
			SectionID               string   `json:"section_id"`
			OutputEnvelope          string   `json:"output_envelope"`
			SourceRefs              []string `json:"source_refs"`
			NormalizedCitations     []string `json:"normalized_citations"`
			SourceFreshnessWarnings []struct {
				SourceID    string `json:"source_id"`
				WarningCode string `json:"warning_code"`
			} `json:"source_freshness_warnings"`
			Payload map[string]any `json:"payload"`
		} `json:"expected_sections"`
	}
	decodeJSONFile(t, filepath.Join(base, "fixtures", "expected_sections_ACME_fixture.json"), &fixture)
	if fixture.AsOf != manifest.SourceFreshnessPolicy.AsOf {
		t.Fatalf("fixture as_of %q must match freshness policy %q", fixture.AsOf, manifest.SourceFreshnessPolicy.AsOf)
	}

	sourceByID := map[string]struct {
		Kind        string
		PublishedAt string
	}{}
	for _, source := range fixture.SourceIndex {
		sourceByID[source.SourceID] = struct {
			Kind        string
			PublishedAt string
		}{Kind: source.Kind, PublishedAt: source.PublishedAt}
	}
	matrixBySection := map[string][]string{}
	requiredFieldsBySection := map[string][]string{}
	for _, row := range manifest.EvidenceRequirementMatrix.Rows {
		matrixBySection[row.SectionID] = row.RequiredEvidenceKinds
		requiredFieldsBySection[row.SectionID] = row.RequiredFields
	}
	registryBySection := map[string]struct {
		PromptID       string
		OutputEnvelope string
		Kinds          []string
	}{}
	for _, section := range manifest.SectionSchemaRegistry.Sections {
		registryBySection[section.SectionID] = struct {
			PromptID       string
			OutputEnvelope string
			Kinds          []string
		}{PromptID: section.PromptID, OutputEnvelope: section.OutputEnvelope, Kinds: section.RequiredEvidenceKinds}
	}

	for _, section := range fixture.ExpectedSections {
		registry, ok := registryBySection[section.SectionID]
		if !ok || registry.OutputEnvelope != section.OutputEnvelope || registry.PromptID == "" {
			t.Fatalf("%s missing registry binding: %#v", section.SectionID, registry)
		}
		if !reflect.DeepEqual(registry.Kinds, matrixBySection[section.SectionID]) {
			t.Fatalf("%s registry/matrix evidence kinds drift: %#v vs %#v", section.SectionID, registry.Kinds, matrixBySection[section.SectionID])
		}
		observedKinds := map[string]bool{}
		for _, ref := range section.SourceRefs {
			source, ok := sourceByID[ref]
			if !ok {
				t.Fatalf("%s unknown source ref %q", section.SectionID, ref)
			}
			observedKinds[source.Kind] = true
			if stale, err := analyzerSourceIsStale(manifest.SourceFreshnessPolicy, source.Kind, source.PublishedAt); err != nil {
				t.Fatalf("freshness parse failed for %s/%s: %v", section.SectionID, ref, err)
			} else if stale {
				if !analyzerSectionHasFreshnessWarning(section.SourceFreshnessWarnings, ref) {
					t.Fatalf("%s stale source %q missing freshness warning", section.SectionID, ref)
				}
			}
		}
		for _, wantKind := range matrixBySection[section.SectionID] {
			if !observedKinds[wantKind] {
				t.Fatalf("%s missing required evidence kind %q from refs %#v", section.SectionID, wantKind, section.SourceRefs)
			}
		}
		for _, field := range requiredFieldsBySection[section.SectionID] {
			if _, ok := section.Payload[field]; !ok {
				t.Fatalf("%s payload missing evidence matrix field %q: %#v", section.SectionID, field, section.Payload)
			}
		}
		for _, ref := range section.SourceRefs {
			want := "[source:" + normalizeAnalyzerSourceID(ref) + "]"
			if !contains(section.NormalizedCitations, want) {
				t.Fatalf("%s normalized citations missing %q: %#v", section.SectionID, want, section.NormalizedCitations)
			}
		}
	}
}

func TestFinRobotAnalyzerReportLivePackageNoLiveImportsOrRuntimeCoupling(t *testing.T) {
	base := analyzerReportLivePackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(path) != ".json" && filepath.Ext(path) != ".leia" {
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
			`(?i)openai|anthropic|gemini|finnhub|fmp_api|yfinance|requests\.|fetch\(`,
			`(?i)\bq/runtime\b|\bruntime/main\b`,
		} {
			if regexp.MustCompile(pattern).FindString(source) != "" {
				return fmt.Errorf("%s contains forbidden live dependency/runtime pattern %q", path, pattern)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFinRobotAnalyzerReportLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(analyzerReportLivePackageDir(t), "main.leia")
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
			got, err := vm.Get("analyzer_report_live_package_summary")
			if err != nil {
				t.Fatalf("Get analyzer_report_live_package_summary: %v", err)
			}
			want := "analyzer_report_live_package catalogs=2 sections=4 provider_free=true live_network=false model_calls=false replay=analyzer_report:ACME:offline:v1"
			if got != want {
				t.Fatalf("analyzer_report_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func assertAnalyzerSectionsAndEnvelopes(t *testing.T, sections []analyzerSectionSchema, envelopes []analyzerOutputEnvelope) {
	t.Helper()
	if len(sections) != 4 || len(envelopes) != 4 {
		t.Fatalf("sections/envelopes = %d/%d, want 4/4", len(sections), len(envelopes))
	}
	var sectionIDs []string
	envelopeBySection := map[string]analyzerOutputEnvelope{}
	for _, envelope := range envelopes {
		envelopeBySection[envelope.SectionID] = envelope
		if !contains(envelope.RequiredFields, "source_refs") || !contains(envelope.RequiredFields, "citations") {
			t.Fatalf("output envelope missing evidence fields: %#v", envelope)
		}
	}
	for _, section := range sections {
		sectionIDs = append(sectionIDs, section.ID)
		if section.Schema != "section_schema_v1" || section.MinSourceRefs < 2 || section.Envelope == "" {
			t.Fatalf("section schema incomplete: %#v", section)
		}
		envelope, ok := envelopeBySection[section.ID]
		if !ok || envelope.ID != section.Envelope {
			t.Fatalf("section %s envelope mismatch: section=%#v envelope=%#v", section.ID, section, envelope)
		}
	}
	sort.Strings(sectionIDs)
	if !reflect.DeepEqual(sectionIDs, []string{"news", "risk", "thesis", "valuation"}) {
		t.Fatalf("section ids = %#v", sectionIDs)
	}
}

func assertAnalyzerSchemaRegistryAndEvidenceMatrix(t *testing.T, registry analyzerSectionSchemaRegistry, matrix analyzerEvidenceRequirementMatrix) {
	t.Helper()
	if registry.Capability != "finance.analyzer.section.schema.registry" ||
		registry.Schema != "section_schema_v1" ||
		registry.RegistryVersion == "" ||
		!reflect.DeepEqual(registry.DeterministicOrder, []string{"risk", "thesis", "valuation", "news"}) ||
		len(registry.Sections) != 4 {
		t.Fatalf("section schema registry incomplete: %#v", registry)
	}
	if matrix.Capability != "finance.analyzer.evidence.matrix" ||
		matrix.Schema != "source_evidence_rule_v1" ||
		matrix.MinimumSourceRefsPerSection != 2 ||
		len(matrix.Rows) != 4 {
		t.Fatalf("evidence requirement matrix incomplete: %#v", matrix)
	}
	registryBySection := map[string]struct {
		FixtureKey     string
		PromptID       string
		OutputEnvelope string
		Kinds          []string
	}{}
	for _, section := range registry.Sections {
		if section.SectionID == "" ||
			section.FixtureKey != "section_schema:"+section.SectionID+":v1" ||
			section.PromptID == "" ||
			section.OutputEnvelope == "" ||
			len(section.RequiredEvidenceKinds) < 2 {
			t.Fatalf("registry section incomplete: %#v", section)
		}
		registryBySection[section.SectionID] = struct {
			FixtureKey     string
			PromptID       string
			OutputEnvelope string
			Kinds          []string
		}{
			FixtureKey:     section.FixtureKey,
			PromptID:       section.PromptID,
			OutputEnvelope: section.OutputEnvelope,
			Kinds:          section.RequiredEvidenceKinds,
		}
	}
	for _, row := range matrix.Rows {
		registrySection, ok := registryBySection[row.SectionID]
		if !ok {
			t.Fatalf("matrix row missing registry section: %#v", row)
		}
		if row.AllowMissingEvidence || row.CleanDegradationMode == "" || len(row.RequiredFields) == 0 {
			t.Fatalf("matrix row degradation contract incomplete: %#v", row)
		}
		if !reflect.DeepEqual(row.RequiredEvidenceKinds, registrySection.Kinds) {
			t.Fatalf("%s registry/matrix kinds mismatch: %#v vs %#v", row.SectionID, registrySection.Kinds, row.RequiredEvidenceKinds)
		}
	}
}

func assertAnalyzerSourceFreshnessPolicy(t *testing.T, policy analyzerSourceFreshnessPolicy) {
	t.Helper()
	if policy.Capability != "finance.analyzer.source.freshness.warn" ||
		policy.AsOf != "2026-06-11" ||
		policy.WarningLevelForStaleSources != "section_warning" ||
		policy.BlockOnStaleSource ||
		len(policy.StaleAfterDaysByKind) < 6 {
		t.Fatalf("source freshness policy incomplete: %#v", policy)
	}
	for _, kind := range []string{"news_event", "price_series", "fundamental_metric", "analyst_estimate", "earnings_transcript", "filing"} {
		if policy.StaleAfterDaysByKind[kind] <= 0 {
			t.Fatalf("freshness policy missing positive threshold for %q: %#v", kind, policy.StaleAfterDaysByKind)
		}
	}
}

func assertAnalyzerAnalystRolePromptEnvelope(t *testing.T, envelope analyzerAnalystRolePromptEnvelope) {
	t.Helper()
	if envelope.Capability != "finance.analyzer.prompt.analyst_role.envelope" ||
		envelope.Role != "equity_research_analyst" ||
		envelope.Audience != "investment_committee" ||
		envelope.Tone != "evidence_first_neutral" ||
		!contains(envelope.MustInclude, "source_refs") ||
		!contains(envelope.MustInclude, "source_freshness_warnings") ||
		!contains(envelope.MustNotInclude, "provider_api_keys") ||
		!contains(envelope.MustNotInclude, "uncited_material_claims") ||
		!strings.Contains(envelope.FallbackInstruction, "omit unsupported claims") {
		t.Fatalf("analyst role prompt envelope incomplete: %#v", envelope)
	}
}

func normalizeAnalyzerSourceID(sourceID string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '_' || r == ':' || r == '-':
			return r
		default:
			return '_'
		}
	}, sourceID)
}

func analyzerSourceIsStale(policy analyzerSourceFreshnessPolicy, kind, publishedAt string) (bool, error) {
	threshold, ok := policy.StaleAfterDaysByKind[kind]
	if !ok {
		return false, fmt.Errorf("missing freshness threshold for %q", kind)
	}
	asOf, err := time.Parse("2006-01-02", policy.AsOf)
	if err != nil {
		return false, err
	}
	published, err := time.Parse("2006-01-02", publishedAt)
	if err != nil {
		return false, err
	}
	return int(asOf.Sub(published).Hours()/24) > threshold, nil
}

func analyzerSectionHasFreshnessWarning(warnings []struct {
	SourceID    string `json:"source_id"`
	WarningCode string `json:"warning_code"`
}, sourceID string) bool {
	for _, warning := range warnings {
		if warning.SourceID == sourceID && warning.WarningCode != "" {
			return true
		}
	}
	return false
}

func analyzerReportLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "analyzer_report")
}

func loadAnalyzerReportLiveManifest(t *testing.T, base string) analyzerReportLiveManifest {
	t.Helper()
	var manifest analyzerReportLiveManifest
	decodeJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}
