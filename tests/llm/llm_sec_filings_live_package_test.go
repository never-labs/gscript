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

type secFilingsLiveManifest struct {
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
		CleanSkipWithoutDependency  bool   `json:"clean_skip_without_dependency"`
		FixtureHook                 string `json:"fixture_hook"`
	} `json:"default_policy"`
	LiveGate struct {
		DefaultEnabled             bool   `json:"default_enabled"`
		Capability                 string `json:"capability"`
		Env                        string `json:"env"`
		RequiresUserAgentEnv       string `json:"requires_user_agent_env"`
		TermsAckEnv                string `json:"terms_ack_env"`
		CleanSkipWithoutCapability bool   `json:"clean_skip_without_capability"`
	} `json:"live_gate"`
	Entrypoints       map[string]string          `json:"entrypoints"`
	Schemas           map[string]string          `json:"schemas"`
	Fixtures          map[string]string          `json:"fixtures"`
	Modules           []secFilingsModule         `json:"modules"`
	AdapterBoundaries []secFilingsBoundary       `json:"adapter_boundaries"`
	TestGates         []string                   `json:"test_gates"`
	NoBuiltIn         map[string]json.RawMessage `json:"no_built_in_guarantee"`
}

type secFilingsModule struct {
	ID            string   `json:"id"`
	SourceModule  string   `json:"source_module"`
	Capabilities  []string `json:"capabilities"`
	OutputSchemas []string `json:"output_schemas"`
}

type secFilingsBoundary struct {
	ID                 string `json:"id"`
	DisplayName        string `json:"display_name"`
	Capability         string `json:"capability"`
	FixtureKey         string `json:"fixture_key"`
	Schema             string `json:"schema"`
	LiveNetwork        bool   `json:"live_network"`
	DependencyImported bool   `json:"dependency_imported"`
	CredentialRequired bool   `json:"credential_required"`
	CleanSkip          bool   `json:"clean_skip"`
}

func TestFinRobotSECFilingsLivePackageManifest(t *testing.T) {
	base := secFilingsLivePackageDir(t)
	manifest := loadSECFilingsLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-sec-filings-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-sec-filings" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	for _, want := range []string{"SEC", "user-agent", "rate-limit", "terms"} {
		if !strings.Contains(manifest.Credentials.Policy, want) {
			t.Fatalf("credential policy should name %q boundary: %q", want, manifest.Credentials.Policy)
		}
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_sec_filings_live_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}
	if manifest.LiveGate.DefaultEnabled ||
		manifest.LiveGate.Capability != "finance.document.sec.live_network" ||
		manifest.LiveGate.Env == "" ||
		manifest.LiveGate.RequiresUserAgentEnv == "" ||
		manifest.LiveGate.TermsAckEnv == "" ||
		!manifest.LiveGate.CleanSkipWithoutCapability {
		t.Fatalf("live gate must be explicit capability/env metadata only: %#v", manifest.LiveGate)
	}

	wantSources := []string{
		"FinRobot/filings_src/sec_api.py",
		"FinRobot/marker_sec_src/sec_filings_to_markdown.py",
		"FinRobot/functional/rag.py",
	}
	if !reflect.DeepEqual(manifest.SourceModules, wantSources) {
		t.Fatalf("source modules = %#v, want %#v", manifest.SourceModules, wantSources)
	}

	for _, key := range []string{"smoke", "sec_filings_contract", "adapter_boundary_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"sec_search_result", "sec_filing_document", "section_extraction", "artifact_provenance", "adapter_boundary"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertSECFilingsJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "search", "fetch_10k", "fetch_10q", "section_extraction", "adapter_boundary"} {
		path := manifest.Fixtures[key]
		if path == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertSECFilingsJSONFile(t, filepath.Join(base, path))
	}

	var ids []string
	for _, module := range manifest.Modules {
		ids = append(ids, module.ID)
		if module.ID == "" || module.SourceModule == "" || len(module.Capabilities) < 5 || len(module.OutputSchemas) == 0 {
			t.Fatalf("module metadata incomplete: %#v", module)
		}
		for _, capability := range module.Capabilities {
			if !strings.HasPrefix(capability, "finance.document.sec.") {
				t.Fatalf("module %s capability %q does not use finance/document/sec dialect", module.ID, capability)
			}
		}
	}
	sort.Strings(ids)
	wantIDs := []string{"sec_fetch", "sec_search", "section_extraction"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("module ids = %#v, want %#v", ids, wantIDs)
	}

	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"10-k", "10-q", "search", "fetch", "section", "html/pdf", "redirect/cache", "user-agent", "terms", "live gate"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
	if len(manifest.NoBuiltIn) == 0 {
		t.Fatal("missing no_built_in_guarantee")
	}
}

func TestFinRobotSECFilingsLivePackageContractsAndFixtures(t *testing.T) {
	base := secFilingsLivePackageDir(t)

	var contract struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Modules               []struct {
			ID             string   `json:"id"`
			SourceModule   string   `json:"source_module"`
			RequiredFields []string `json:"required_fields"`
		} `json:"modules"`
		FieldContracts  map[string]any `json:"field_contracts"`
		AcceptanceGates []string       `json:"acceptance_gates"`
	}
	decodeSECFilingsJSONFile(t, filepath.Join(base, "contracts", "sec_filings_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || len(contract.Modules) != 3 {
		t.Fatalf("contract header/modules = %#v", contract)
	}
	for _, field := range []string{"filing_search", "filing_fetch", "artifact_provenance", "sec_access_policy", "redirect_cache_metadata", "section_extraction", "live_gate"} {
		if contract.FieldContracts[field] == nil {
			t.Fatalf("missing field contract %q", field)
		}
	}
	acceptance := strings.ToLower(strings.Join(contract.AcceptanceGates, " "))
	for _, want := range []string{"10-k", "10-q", "html", "pdf", "section", "redirect/cache", "user-agent", "rate-limit", "terms", "live network"} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance gates missing %q: %s", want, acceptance)
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
	decodeSECFilingsJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 5 {
		t.Fatalf("fixture index header/count = %#v", index)
	}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Capability == "" || fixture.Path == "" || fixture.Schema == "" {
			t.Fatalf("fixture metadata incomplete: %#v", fixture)
		}
		if fixture.Metadata["replay_ready"] != true {
			t.Fatalf("%s replay_ready = %#v", fixture.FixtureKey, fixture.Metadata["replay_ready"])
		}
		assertSECFilingsJSONFile(t, filepath.Join(base, fixture.Path))
		assertSECFilingsJSONFile(t, filepath.Join(base, fixture.Schema))
	}

	assertSECFilingsSchemaRequired(t, filepath.Join(base, "schemas", "sec_search_result_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "query", "results"})
	assertSECFilingsNestedSchemaRequired(t, filepath.Join(base, "schemas", "sec_search_result_v1.schema.json"), []string{"properties", "query"}, []string{"ticker", "cik", "form_types", "date_range", "user_agent_policy", "rate_limit_policy", "terms"})
	assertSECFilingsNestedSchemaRequired(t, filepath.Join(base, "schemas", "sec_search_result_v1.schema.json"), []string{"properties", "results", "items"}, []string{"document_id", "ticker", "cik", "accession_number", "form_type", "filing_date", "source_url", "document_url", "artifact_hint", "redirect_chain", "cache", "terms", "provenance"})
	assertSECFilingsSchemaRequired(t, filepath.Join(base, "schemas", "sec_filing_document_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "document_id", "ticker", "cik", "accession_number", "form_type", "filing_date", "primary_artifact", "user_agent_policy", "redirect_chain", "cache", "terms", "artifacts"})
	assertSECFilingsNestedSchemaRequired(t, filepath.Join(base, "schemas", "sec_filing_document_v1.schema.json"), []string{"properties", "artifacts", "items"}, []string{"artifact_id", "artifact_type", "source_url", "mime_type", "content_sha256", "byte_length", "parser_boundary", "provenance", "terms"})
	assertSECFilingsNestedSchemaRequired(t, filepath.Join(base, "schemas", "section_extraction_v1.schema.json"), []string{"properties", "documents", "items"}, []string{"document_id", "form_type", "artifact_ref", "artifact_type", "sections"})
	assertSECFilingsNestedSchemaRequired(t, filepath.Join(base, "schemas", "section_extraction_v1.schema.json"), []string{"properties", "documents", "items", "properties", "sections", "items"}, []string{"section_id", "section_title", "text", "source_offsets", "citation", "provenance"})
}

func TestFinRobotSECFilingsLivePackageFixtureShape(t *testing.T) {
	base := secFilingsLivePackageDir(t)

	var searchFixture struct {
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
		Query        struct {
			UserAgentPolicy secUserAgentPolicy `json:"user_agent_policy"`
			RateLimitPolicy struct {
				MaxRequestsPerSecond   int  `json:"max_requests_per_second"`
				EnforcedInFixture      bool `json:"enforced_in_fixture"`
				CleanSkipWithoutPolicy bool `json:"clean_skip_without_policy"`
			} `json:"rate_limit_policy"`
			Terms secTerms `json:"terms"`
		} `json:"query"`
		Results []secSearchResult `json:"results"`
	}
	decodeSECFilingsJSONFile(t, filepath.Join(base, "fixtures", "sec_search_ACME_fixture.json"), &searchFixture)
	if !searchFixture.ProviderFree || searchFixture.LiveNetwork || len(searchFixture.Results) != 2 {
		t.Fatalf("search fixture header/counts = %#v", searchFixture)
	}
	if !searchFixture.Query.UserAgentPolicy.RequiredForLive ||
		searchFixture.Query.UserAgentPolicy.DeclaredInFixture ||
		!searchFixture.Query.UserAgentPolicy.CleanSkipWithoutUserAgent ||
		searchFixture.Query.RateLimitPolicy.MaxRequestsPerSecond != 10 ||
		searchFixture.Query.RateLimitPolicy.EnforcedInFixture ||
		!searchFixture.Query.RateLimitPolicy.CleanSkipWithoutPolicy ||
		searchFixture.Query.Terms.LiveNetwork {
		t.Fatalf("SEC user-agent/rate-limit/terms policy incomplete: %#v", searchFixture.Query)
	}
	forms := map[string]bool{}
	artifactHints := map[string]bool{}
	for _, result := range searchFixture.Results {
		forms[result.FormType] = true
		artifactHints[result.ArtifactHint] = true
		if result.DocumentID == "" || result.AccessionNumber == "" || result.CIK == "" || result.FilingDate == "" || result.SourceURL == "" || result.DocumentURL == "" || result.Provenance.FixtureKey == "" || result.Provenance.LiveNetwork || result.Terms.LiveNetwork {
			t.Fatalf("search result missing SEC provenance fields: %#v", result)
		}
		if len(result.RedirectChain) == 0 || result.RedirectChain[0].Status != 200 || !result.RedirectChain[0].FixtureRedirect || result.Cache.Key == "" || result.Cache.Mode != "fixture_replay" || !result.Cache.Hit || result.Cache.TTLSeconds != 0 {
			t.Fatalf("search result missing redirect/cache metadata: %#v", result)
		}
	}
	for _, want := range []string{"10-K", "10-Q"} {
		if !forms[want] {
			t.Fatalf("search fixture missing form %q in %#v", want, forms)
		}
	}
	for _, want := range []string{"sec_html", "sec_pdf"} {
		if !artifactHints[want] {
			t.Fatalf("search fixture missing artifact hint %q in %#v", want, artifactHints)
		}
	}

	fetch10K := loadSECFetchFixture(t, filepath.Join(base, "fixtures", "sec_fetch_ACME_10k_fixture.json"))
	fetch10Q := loadSECFetchFixture(t, filepath.Join(base, "fixtures", "sec_fetch_ACME_10q_fixture.json"))
	assertSECFetchFixture(t, fetch10K, "10-K", "sec_html")
	assertSECFetchFixture(t, fetch10Q, "10-Q", "sec_pdf")
}

func TestFinRobotSECFilingsLivePackageSectionExtraction(t *testing.T) {
	base := secFilingsLivePackageDir(t)

	var fixture struct {
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
		Documents    []struct {
			DocumentID   string `json:"document_id"`
			FormType     string `json:"form_type"`
			ArtifactRef  string `json:"artifact_ref"`
			ArtifactType string `json:"artifact_type"`
			Sections     []struct {
				SectionID     string `json:"section_id"`
				SectionTitle  string `json:"section_title"`
				Text          string `json:"text"`
				SourceOffsets struct {
					Start int `json:"start"`
					End   int `json:"end"`
				} `json:"source_offsets"`
				Citation struct {
					DocumentURL string `json:"document_url"`
				} `json:"citation"`
				Provenance struct {
					AccessionNumber string `json:"accession_number"`
					FixtureKey      string `json:"fixture_key"`
					LiveNetwork     bool   `json:"live_network"`
				} `json:"provenance"`
			} `json:"sections"`
		} `json:"documents"`
	}
	decodeSECFilingsJSONFile(t, filepath.Join(base, "fixtures", "section_extraction_ACME_fixture.json"), &fixture)
	if !fixture.ProviderFree || fixture.LiveNetwork || len(fixture.Documents) != 2 {
		t.Fatalf("section fixture header/counts = %#v", fixture)
	}

	byForm := map[string]map[string]bool{}
	artifactTypes := map[string]bool{}
	for _, doc := range fixture.Documents {
		if doc.DocumentID == "" || doc.FormType == "" || doc.ArtifactRef == "" || doc.ArtifactType == "" || len(doc.Sections) == 0 {
			t.Fatalf("section document incomplete: %#v", doc)
		}
		artifactTypes[doc.ArtifactType] = true
		if byForm[doc.FormType] == nil {
			byForm[doc.FormType] = map[string]bool{}
		}
		for _, section := range doc.Sections {
			byForm[doc.FormType][section.SectionID] = true
			if section.SectionTitle == "" || section.Text == "" || section.SourceOffsets.Start >= section.SourceOffsets.End || section.Citation.DocumentURL == "" || section.Provenance.AccessionNumber == "" || section.Provenance.FixtureKey == "" || section.Provenance.LiveNetwork {
				t.Fatalf("section extraction missing citation/provenance: %#v", section)
			}
		}
	}
	if !byForm["10-K"]["business"] || !byForm["10-K"]["risk_factors"] || !byForm["10-K"]["mda"] {
		t.Fatalf("10-K sections incomplete: %#v", byForm["10-K"])
	}
	if !byForm["10-Q"]["mda"] || !byForm["10-Q"]["risk_factors"] {
		t.Fatalf("10-Q sections incomplete: %#v", byForm["10-Q"])
	}
	if !artifactTypes["sec_html"] || !artifactTypes["sec_pdf"] {
		t.Fatalf("section fixture must retain HTML/PDF artifact refs: %#v", artifactTypes)
	}
}

func TestFinRobotSECFilingsLivePackageAdapterBoundaries(t *testing.T) {
	base := secFilingsLivePackageDir(t)
	manifest := loadSECFilingsLiveManifest(t, base)

	var ids []string
	for _, boundary := range manifest.AdapterBoundaries {
		ids = append(ids, boundary.ID)
		if boundary.ID == "" || boundary.DisplayName == "" || boundary.Capability == "" || boundary.FixtureKey == "" || boundary.Schema != "adapter_boundary" {
			t.Fatalf("adapter boundary metadata incomplete: %#v", boundary)
		}
		if boundary.LiveNetwork || boundary.DependencyImported || boundary.CredentialRequired || !boundary.CleanSkip {
			t.Fatalf("adapter boundary must be fixture-only and clean-skip safe: %#v", boundary)
		}
	}
	sort.Strings(ids)
	wantIDs := []string{"html_document_parser", "pdf_artifact_reader", "sec_archive_fetch", "sec_company_submissions"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("adapter ids = %#v, want %#v", ids, wantIDs)
	}

	var boundaryContract struct {
		ProviderFree               bool                 `json:"provider_free"`
		LiveNetwork                bool                 `json:"live_network"`
		RealDependencyImports      bool                 `json:"real_dependency_imports"`
		CleanSkipWithoutDependency bool                 `json:"clean_skip_without_dependency"`
		Boundaries                 []secFilingsBoundary `json:"boundaries"`
	}
	decodeSECFilingsJSONFile(t, filepath.Join(base, "contracts", "adapter_boundary_contract.json"), &boundaryContract)
	if !boundaryContract.ProviderFree || boundaryContract.LiveNetwork || boundaryContract.RealDependencyImports || !boundaryContract.CleanSkipWithoutDependency {
		t.Fatalf("boundary contract header = %#v", boundaryContract)
	}
	if len(boundaryContract.Boundaries) != 4 {
		t.Fatalf("boundary contract count = %d, want 4", len(boundaryContract.Boundaries))
	}
	for _, boundary := range boundaryContract.Boundaries {
		if boundary.LiveNetwork || boundary.DependencyImported || boundary.CredentialRequired || !boundary.CleanSkip {
			t.Fatalf("boundary contract must not enable live adapters: %#v", boundary)
		}
	}
}

func TestFinRobotSECFilingsLivePackageNoLiveImports(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(secFilingsLivePackageDir(t), "main.leia"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pattern := range []string{
		`(?m)^\s*import\s+`,
		`(?m)^\s*use\s+`,
		`(?m)^\s*load\s*\(`,
		`(?m)^\s*require\s*\(`,
		`(?m)^\s*(requests|http|sec_api|edgar|pymupdf|fitz|pdfplumber|beautifulsoup|bs4|openai)\s*[.(]`,
	} {
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("main.leia contains live dependency loader matching %q", pattern)
		}
	}
}

func TestFinRobotSECFilingsLivePackageNoLiveNetworkOrDependencyFlags(t *testing.T) {
	base := secFilingsLivePackageDir(t)
	for _, rel := range []string{
		"package.manifest.json",
		"contracts/sec_filings_contract.json",
		"contracts/adapter_boundary_contract.json",
		"fixtures/provider_free_fixture_index.json",
		"fixtures/sec_search_ACME_fixture.json",
		"fixtures/sec_fetch_ACME_10k_fixture.json",
		"fixtures/sec_fetch_ACME_10q_fixture.json",
		"fixtures/section_extraction_ACME_fixture.json",
		"fixtures/adapter_boundary_fixture.json",
	} {
		var value any
		decodeSECFilingsJSONFile(t, filepath.Join(base, rel), &value)
		assertNoEnabledLiveOrDependencyFlags(t, rel, value)
	}
}

func TestFinRobotSECFilingsLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(secFilingsLivePackageDir(t), "main.leia")

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
			got, err := vm.Get("sec_filings_live_package_summary")
			if err != nil {
				t.Fatalf("Get sec_filings_live_package_summary: %v", err)
			}
			want := "sec_filings_live_package modules=3 adapters=4 provider_free=true live_network=false imports=false fixtures=5 live_gate=false"
			if got != want {
				t.Fatalf("sec_filings_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

type secUserAgentPolicy struct {
	RequiredForLive           bool `json:"required_for_live"`
	DeclaredInFixture         bool `json:"declared_in_fixture"`
	CleanSkipWithoutUserAgent bool `json:"clean_skip_without_user_agent"`
}

type secTerms struct {
	Usage                    string `json:"usage"`
	Attribution              string `json:"attribution"`
	Redistribution           string `json:"redistribution"`
	LiveNetwork              bool   `json:"live_network"`
	UserAgentRequiredForLive bool   `json:"user_agent_required_for_live"`
}

type secSearchResult struct {
	DocumentID      string `json:"document_id"`
	CIK             string `json:"cik"`
	AccessionNumber string `json:"accession_number"`
	FormType        string `json:"form_type"`
	FilingDate      string `json:"filing_date"`
	SourceURL       string `json:"source_url"`
	DocumentURL     string `json:"document_url"`
	ArtifactHint    string `json:"artifact_hint"`
	RedirectChain   []struct {
		Status          int  `json:"status"`
		FixtureRedirect bool `json:"fixture_redirect"`
	} `json:"redirect_chain"`
	Cache struct {
		Key        string `json:"key"`
		Mode       string `json:"mode"`
		Hit        bool   `json:"hit"`
		TTLSeconds int    `json:"ttl_seconds"`
	} `json:"cache"`
	Terms      secTerms `json:"terms"`
	Provenance struct {
		FixtureKey  string `json:"fixture_key"`
		LiveNetwork bool   `json:"live_network"`
	} `json:"provenance"`
}

type secFetchFixture struct {
	ProviderFree    bool               `json:"provider_free"`
	LiveNetwork     bool               `json:"live_network"`
	DocumentID      string             `json:"document_id"`
	AccessionNumber string             `json:"accession_number"`
	FormType        string             `json:"form_type"`
	PrimaryArtifact string             `json:"primary_artifact"`
	UserAgentPolicy secUserAgentPolicy `json:"user_agent_policy"`
	RedirectChain   []struct {
		Status          int  `json:"status"`
		FixtureRedirect bool `json:"fixture_redirect"`
	} `json:"redirect_chain"`
	Cache struct {
		Key        string `json:"key"`
		Mode       string `json:"mode"`
		Hit        bool   `json:"hit"`
		TTLSeconds int    `json:"ttl_seconds"`
	} `json:"cache"`
	Terms     secTerms `json:"terms"`
	Artifacts []struct {
		ArtifactID     string `json:"artifact_id"`
		ArtifactType   string `json:"artifact_type"`
		SourceURL      string `json:"source_url"`
		MimeType       string `json:"mime_type"`
		ContentSHA256  string `json:"content_sha256"`
		ParserBoundary struct {
			Adapter            string `json:"adapter"`
			DependencyImported bool   `json:"dependency_imported"`
			LiveNetwork        bool   `json:"live_network"`
			CleanSkip          bool   `json:"clean_skip"`
		} `json:"parser_boundary"`
		Provenance struct {
			FixtureKey  string `json:"fixture_key"`
			LiveNetwork bool   `json:"live_network"`
			Redacted    bool   `json:"redacted"`
		} `json:"provenance"`
		Terms secTerms `json:"terms"`
	} `json:"artifacts"`
}

func loadSECFetchFixture(t *testing.T, path string) secFetchFixture {
	t.Helper()
	var fixture secFetchFixture
	decodeSECFilingsJSONFile(t, path, &fixture)
	return fixture
}

func assertSECFetchFixture(t *testing.T, fixture secFetchFixture, formType string, artifactType string) {
	t.Helper()
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.DocumentID == "" || fixture.AccessionNumber == "" || fixture.FormType != formType || fixture.PrimaryArtifact == "" || len(fixture.Artifacts) != 1 {
		t.Fatalf("fetch fixture header/counts = %#v", fixture)
	}
	if !fixture.UserAgentPolicy.RequiredForLive || fixture.UserAgentPolicy.DeclaredInFixture || !fixture.UserAgentPolicy.CleanSkipWithoutUserAgent {
		t.Fatalf("fetch user-agent policy incomplete: %#v", fixture.UserAgentPolicy)
	}
	if len(fixture.RedirectChain) == 0 || fixture.RedirectChain[0].Status != 200 || !fixture.RedirectChain[0].FixtureRedirect || fixture.Cache.Key == "" || fixture.Cache.Mode != "fixture_replay" || !fixture.Cache.Hit || fixture.Cache.TTLSeconds != 0 {
		t.Fatalf("fetch redirect/cache metadata incomplete: %#v", fixture)
	}
	if fixture.Terms.LiveNetwork || fixture.Terms.UserAgentRequiredForLive != true || fixture.Terms.Usage == "" || fixture.Terms.Attribution == "" {
		t.Fatalf("fetch terms metadata incomplete: %#v", fixture.Terms)
	}
	artifact := fixture.Artifacts[0]
	if artifact.ArtifactID == "" || artifact.ArtifactType != artifactType || artifact.SourceURL == "" || artifact.MimeType == "" || artifact.ContentSHA256 == "" || artifact.Provenance.FixtureKey == "" || artifact.Provenance.LiveNetwork || !artifact.Provenance.Redacted || artifact.Terms.LiveNetwork {
		t.Fatalf("artifact provenance incomplete: %#v", artifact)
	}
	if artifact.ParserBoundary.Adapter == "" || artifact.ParserBoundary.DependencyImported || artifact.ParserBoundary.LiveNetwork || !artifact.ParserBoundary.CleanSkip {
		t.Fatalf("artifact parser boundary must be fixture-only: %#v", artifact.ParserBoundary)
	}
}

func secFilingsLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "sec_filings")
}

func loadSECFilingsLiveManifest(t *testing.T, base string) secFilingsLiveManifest {
	t.Helper()
	var manifest secFilingsLiveManifest
	decodeSECFilingsJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertSECFilingsJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeSECFilingsJSONFile(t, path, &value)
}

func assertSECFilingsSchemaRequired(t *testing.T, path string, fields []string) {
	t.Helper()
	assertSECFilingsNestedSchemaRequired(t, path, nil, fields)
}

func assertSECFilingsNestedSchemaRequired(t *testing.T, path string, objectPath []string, fields []string) {
	t.Helper()
	var schema map[string]any
	decodeSECFilingsJSONFile(t, path, &schema)
	var node any = schema
	for _, part := range objectPath {
		asMap, ok := node.(map[string]any)
		if !ok {
			t.Fatalf("%s path %v is not an object", path, objectPath)
		}
		node = asMap[part]
	}
	asMap, ok := node.(map[string]any)
	if !ok {
		t.Fatalf("%s path %v is not an object", path, objectPath)
	}
	requiredValues, ok := asMap["required"].([]any)
	if !ok {
		t.Fatalf("%s path %v has no required array", path, objectPath)
	}
	required := map[string]bool{}
	for _, value := range requiredValues {
		if field, ok := value.(string); ok {
			required[field] = true
		}
	}
	for _, field := range fields {
		if !required[field] {
			t.Fatalf("%s path %v required fields missing %q in %#v", path, objectPath, field, required)
		}
	}
}

func decodeSECFilingsJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
