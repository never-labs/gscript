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
	Entrypoints          map[string]string       `json:"entrypoints"`
	Schemas              map[string]string       `json:"schemas"`
	Fixtures             map[string]string       `json:"fixtures"`
	Capabilities         []string                `json:"capabilities"`
	PromptCatalogs       []analyzerPromptCatalog `json:"prompt_catalogs"`
	SectionSchemas       []analyzerSectionSchema `json:"section_schemas"`
	EvidenceRules        analyzerEvidenceRules   `json:"evidence_rules"`
	CitationRequirements struct {
		Capability                 string   `json:"capability"`
		RequiredFields             []string `json:"required_fields"`
		InlineCitationFormat       string   `json:"inline_citation_format"`
		AllClaimsRequireSourceRefs bool     `json:"all_claims_require_source_refs"`
		UnresolvedRefsAllowed      bool     `json:"unresolved_refs_allowed"`
	} `json:"citation_requirements"`
	OutputEnvelopes []analyzerOutputEnvelope `json:"output_envelopes"`
	Replay          struct {
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
		"finance.analyzer.prompt.catalog",
		"finance.analyzer.evidence.enforce",
		"finance.analyzer.citation.require",
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
		!manifest.CitationRequirements.AllClaimsRequireSourceRefs ||
		manifest.CitationRequirements.UnresolvedRefsAllowed {
		t.Fatalf("citation requirements incomplete: %#v", manifest.CitationRequirements)
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
		SourceEvidenceEnforcement analyzerEvidenceRules `json:"source_evidence_enforcement"`
		ReplayContract            struct {
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
	if contract.SourceEvidenceEnforcement.MinimumSourceRefsPerSection != 2 ||
		!contract.SourceEvidenceEnforcement.RejectsUnresolvedRefs ||
		!contract.SourceEvidenceEnforcement.RequiresSourceQuoteOrMetric ||
		contract.SourceEvidenceEnforcement.LiveNetwork {
		t.Fatalf("source evidence enforcement incomplete: %#v", contract.SourceEvidenceEnforcement)
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
			SourceID     string `json:"source_id"`
			Title        string `json:"title"`
			Publisher    string `json:"publisher"`
			PublishedAt  string `json:"published_at"`
			RetrievedAt  string `json:"retrieved_at"`
			Locator      string `json:"locator"`
			EvidenceHash string `json:"evidence_hash"`
		} `json:"source_index"`
		ExpectedSections []struct {
			SectionID          string   `json:"section_id"`
			OutputEnvelope     string   `json:"output_envelope"`
			SourceRefs         []string `json:"source_refs"`
			Citations          []string `json:"citations"`
			EvidenceValidation struct {
				ProviderFree         bool     `json:"provider_free"`
				LiveNetwork          bool     `json:"live_network"`
				Validated            bool     `json:"validated"`
				UnresolvedSourceRefs []string `json:"unresolved_source_refs"`
				FixtureKey           string   `json:"fixture_key"`
			} `json:"evidence_validation"`
			Payload map[string]any `json:"payload"`
		} `json:"expected_sections"`
	}
	decodeJSONFile(t, filepath.Join(base, "fixtures", "expected_sections_ACME_fixture.json"), &fixture)
	if !fixture.ProviderFree || fixture.LiveNetwork || len(fixture.SourceIndex) < 4 || len(fixture.ExpectedSections) != 4 {
		t.Fatalf("fixture header/count incomplete: %#v", fixture)
	}
	sourceIDs := map[string]bool{}
	for _, source := range fixture.SourceIndex {
		if source.SourceID == "" || source.Title == "" || source.Publisher == "" || source.PublishedAt == "" || source.RetrievedAt == "" || source.Locator == "" || source.EvidenceHash == "" {
			t.Fatalf("source citation metadata incomplete: %#v", source)
		}
		sourceIDs[source.SourceID] = true
	}
	var gotSections []string
	for _, section := range fixture.ExpectedSections {
		gotSections = append(gotSections, section.SectionID)
		if len(section.SourceRefs) < 2 || len(section.Citations) < 2 || len(section.Payload) == 0 {
			t.Fatalf("section replay payload incomplete: %#v", section)
		}
		if !section.EvidenceValidation.ProviderFree ||
			section.EvidenceValidation.LiveNetwork ||
			!section.EvidenceValidation.Validated ||
			len(section.EvidenceValidation.UnresolvedSourceRefs) != 0 ||
			section.EvidenceValidation.FixtureKey == "" {
			t.Fatalf("section evidence validation incomplete: %#v", section.EvidenceValidation)
		}
		for _, ref := range section.SourceRefs {
			if !sourceIDs[ref] {
				t.Fatalf("%s has unresolved source ref %q", section.SectionID, ref)
			}
			if !contains(section.Citations, "[source:"+ref+"]") {
				t.Fatalf("%s missing citation for source ref %q: %#v", section.SectionID, ref, section.Citations)
			}
		}
	}
	if !reflect.DeepEqual(gotSections, []string{"risk", "thesis", "valuation", "news"}) {
		t.Fatalf("expected section order = %#v", gotSections)
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
