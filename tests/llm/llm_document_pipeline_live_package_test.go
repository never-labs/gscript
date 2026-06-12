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

type documentPipelineLiveManifest struct {
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
	Entrypoints       map[string]string          `json:"entrypoints"`
	Schemas           map[string]string          `json:"schemas"`
	Fixtures          map[string]string          `json:"fixtures"`
	Modules           []documentPipelineModule   `json:"modules"`
	AdapterBoundaries []documentPipelineBoundary `json:"adapter_boundaries"`
	TestGates         []string                   `json:"test_gates"`
	NoBuiltIn         map[string]json.RawMessage `json:"no_built_in_guarantee"`
}

type documentPipelineModule struct {
	ID            string   `json:"id"`
	SourceModule  string   `json:"source_module"`
	Capabilities  []string `json:"capabilities"`
	OutputSchemas []string `json:"output_schemas"`
}

type documentPipelineBoundary struct {
	ID                 string `json:"id"`
	DisplayName        string `json:"display_name"`
	Capability         string `json:"capability"`
	FixtureKey         string `json:"fixture_key"`
	Schema             string `json:"schema"`
	LiveNetwork        bool   `json:"live_network"`
	DependencyImported bool   `json:"dependency_imported"`
	CredentialRequired bool   `json:"credential_required"`
	CleanSkip          bool   `json:"clean_skip"`
	ExternalPackage    string `json:"external_package"`
}

func TestFinRobotDocumentPipelineLivePackageManifest(t *testing.T) {
	base := documentPipelineLivePackageDir(t)
	manifest := loadDocumentPipelineLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-document-pipeline-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-document-pipeline" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	for _, want := range []string{"SEC", "Marker", "embedding", "LangChain", "vector", "retriever"} {
		if !strings.Contains(manifest.Credentials.Policy, want) {
			t.Fatalf("credential policy should name %q boundary: %q", want, manifest.Credentials.Policy)
		}
	}
	for _, want := range []string{"user-agent", "rate-limit"} {
		if !strings.Contains(manifest.Credentials.Policy, want) {
			t.Fatalf("credential policy should name SEC access policy %q: %q", want, manifest.Credentials.Policy)
		}
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_document_pipeline_live_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}

	wantSources := []string{
		"FinRobot/filings_src/sec_api.py",
		"FinRobot/marker_sec_src/sec_filings_to_markdown.py",
		"FinRobot/functional/rag.py",
		"FinRobot/functional/ragquery.py",
	}
	if !reflect.DeepEqual(manifest.SourceModules, wantSources) {
		t.Fatalf("source modules = %#v, want %#v", manifest.SourceModules, wantSources)
	}

	for _, key := range []string{"smoke", "document_pipeline_contract", "adapter_boundary_contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"filing_search_result", "document_markdown", "section_extraction", "rag_chunk", "retriever_query_result", "adapter_boundary", "rag_corpus_manifest"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertDocumentPipelineJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "sec_search", "markdown_conversion", "rag_index", "retriever_query", "adapter_boundary", "rag_corpus_manifest"} {
		path := manifest.Fixtures[key]
		if path == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertDocumentPipelineJSONFile(t, filepath.Join(base, path))
	}

	var ids []string
	for _, module := range manifest.Modules {
		ids = append(ids, module.ID)
		if module.ID == "" || module.SourceModule == "" || len(module.Capabilities) < 4 || len(module.OutputSchemas) == 0 {
			t.Fatalf("module metadata incomplete: %#v", module)
		}
	}
	sort.Strings(ids)
	wantIDs := []string{"filings_src", "marker_sec_src", "rag", "ragquery"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("module ids = %#v, want %#v", ids, wantIDs)
	}

	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"sec", "user-agent", "rate-limit", "redirect/cache", "html/pdf", "section", "generic memory", "chunk", "corpus", "citation", "provenance", "vector capability", "retriever", "langchain", "fixture"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
	if len(manifest.NoBuiltIn) == 0 {
		t.Fatal("missing no_built_in_guarantee")
	}
}

func TestFinRobotDocumentPipelineLivePackageContractsAndFixtures(t *testing.T) {
	base := documentPipelineLivePackageDir(t)

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
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "contracts", "document_pipeline_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || len(contract.Modules) != 4 {
		t.Fatalf("contract header/modules = %#v", contract)
	}
	for _, field := range []string{"filing_search", "sec_access_policy", "redirect_cache_metadata", "html_pdf_to_markdown", "html_parser_boundary", "pdf_to_markdown_converter_boundary", "section_extraction", "chunk_citation_provenance", "rag_corpus_manifest", "chunk_citation_vector_adapter_matrix", "vector_store_capability_gates", "citation_consistency", "vector_retriever_adapter", "embedding_retriever_clean_skip"} {
		if contract.FieldContracts[field] == nil {
			t.Fatalf("missing field contract %q", field)
		}
	}
	acceptance := strings.ToLower(strings.Join(contract.AcceptanceGates, " "))
	for _, want := range []string{"sec filing", "user-agent", "rate-limit", "redirect", "cache", "html parser", "pdf-to-markdown", "section", "corpus manifest", "adapter matrix", "capability gates", "generic memory store package handoff", "clean-skip", "retriever", "langchain", "fixture replay"} {
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
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 6 {
		t.Fatalf("fixture index header/count = %#v", index)
	}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Capability == "" || fixture.Path == "" || fixture.Schema == "" {
			t.Fatalf("fixture metadata incomplete: %#v", fixture)
		}
		if fixture.Metadata["replay_ready"] != true {
			t.Fatalf("%s replay_ready = %#v", fixture.FixtureKey, fixture.Metadata["replay_ready"])
		}
		assertDocumentPipelineJSONFile(t, filepath.Join(base, fixture.Path))
		assertDocumentPipelineJSONFile(t, filepath.Join(base, fixture.Schema))
	}

	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "filing_search_result_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "ticker", "query", "results"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "filing_search_result_v1.schema.json"), []string{"properties", "query"}, []string{"form_type", "user_agent_policy", "rate_limit_policy"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "filing_search_result_v1.schema.json"), []string{"properties", "results", "items"}, []string{"accession_number", "cik", "company_name", "ticker", "form_type", "filing_date", "source_url", "document_url", "redirect_chain", "cache", "provenance"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "document_markdown_v1.schema.json"), []string{"properties", "conversion"}, []string{"conversion_source", "converter", "html_supported", "pdf_supported", "html_parser_boundary", "pdf_to_markdown_boundary", "redirect_cache_metadata", "warnings"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "rag_chunk_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "document_id", "corpus_manifest", "chunk_policy", "adapter_matrix", "chunks", "vector_store_capabilities"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "rag_chunk_v1.schema.json"), []string{"properties", "chunks", "items", "properties", "provenance"}, []string{"corpus_id", "accession_number", "source_fixture_key", "fixture_key", "converter_fixture_key", "source_offsets"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "rag_corpus_manifest_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "corpus_id", "document_id", "fixture_key", "source_fixture_keys", "chunk_fixture_key", "retriever_fixture_key", "chunk_ids", "citation_consistency", "vector_store_capabilities", "retriever_boundaries", "optional_dependencies"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "retriever_query_result_v1.schema.json"), []string{"properties", "retriever"}, []string{"adapter", "top_k", "live_network", "dependency_imported", "clean_skip_without_dependency", "skip_reason", "embedding_adapter", "vector_adapter"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "retriever_query_result_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "query", "answer", "retriever", "retriever_boundaries", "retrieved_chunks", "answer_citations", "provenance"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "adapter_boundary_v1.schema.json"), []string{"properties", "boundaries", "items"}, []string{"id", "display_name", "capability", "fixture_key", "live_network", "dependency_imported", "credential_required", "clean_skip", "clean_skip_reason", "policy", "gap"})
}

func TestFinRobotDocumentPipelineLivePackageAdapterBoundaries(t *testing.T) {
	base := documentPipelineLivePackageDir(t)
	manifest := loadDocumentPipelineLiveManifest(t, base)

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
	wantIDs := []string{"chunk_citation_adapter", "embedding_adapter", "generic_memory_store", "html_parser", "pdf_to_markdown_converter", "sec_filing_client", "vector_retriever", "vector_store_adapter"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("adapter ids = %#v, want %#v", ids, wantIDs)
	}

	var boundaryContract struct {
		ProviderFree               bool                       `json:"provider_free"`
		LiveNetwork                bool                       `json:"live_network"`
		RealDependencyImports      bool                       `json:"real_dependency_imports"`
		CleanSkipWithoutDependency bool                       `json:"clean_skip_without_dependency"`
		Boundaries                 []documentPipelineBoundary `json:"boundaries"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "contracts", "adapter_boundary_contract.json"), &boundaryContract)
	if !boundaryContract.ProviderFree || boundaryContract.LiveNetwork || boundaryContract.RealDependencyImports || !boundaryContract.CleanSkipWithoutDependency {
		t.Fatalf("boundary contract header = %#v", boundaryContract)
	}
	if len(boundaryContract.Boundaries) != 8 {
		t.Fatalf("boundary contract count = %d, want 8", len(boundaryContract.Boundaries))
	}
	for _, boundary := range boundaryContract.Boundaries {
		if boundary.LiveNetwork || boundary.DependencyImported || boundary.CredentialRequired || !boundary.CleanSkip {
			t.Fatalf("boundary contract must not enable live adapters: %#v", boundary)
		}
	}
}

func assertDocumentPipelineExternalGenericMemoryBoundary(t *testing.T, base, rel string) {
	t.Helper()
	var doc struct {
		AdapterBoundaries []documentPipelineBoundary `json:"adapter_boundaries"`
		Boundaries        []documentPipelineBoundary `json:"boundaries"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, rel), &doc)
	boundaries := doc.AdapterBoundaries
	if len(boundaries) == 0 {
		boundaries = doc.Boundaries
	}
	for _, boundary := range boundaries {
		if boundary.ID != "generic_memory_store" {
			continue
		}
		if boundary.Capability != "generic.ai.memory.store" ||
			boundary.FixtureKey != "generic:memory_store:offline" ||
			boundary.ExternalPackage != "generic_memory_store" ||
			boundary.LiveNetwork ||
			boundary.DependencyImported ||
			boundary.CredentialRequired ||
			!boundary.CleanSkip {
			t.Fatalf("%s generic memory boundary mismatch: %#v", rel, boundary)
		}
		return
	}
	t.Fatalf("%s missing external generic memory store boundary", rel)
}

func TestFinRobotDocumentPipelineLivePackageFixtureShape(t *testing.T) {
	base := documentPipelineLivePackageDir(t)

	var searchFixture struct {
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
		Query        struct {
			UserAgentPolicy struct {
				RequiredForLive           bool `json:"required_for_live"`
				DeclaredInFixture         bool `json:"declared_in_fixture"`
				CleanSkipWithoutUserAgent bool `json:"clean_skip_without_user_agent"`
			} `json:"user_agent_policy"`
			RateLimitPolicy struct {
				MaxRequestsPerSecond   int  `json:"max_requests_per_second"`
				EnforcedInFixture      bool `json:"enforced_in_fixture"`
				CleanSkipWithoutPolicy bool `json:"clean_skip_without_policy"`
			} `json:"rate_limit_policy"`
		} `json:"query"`
		Results []struct {
			AccessionNumber string `json:"accession_number"`
			CIK             string `json:"cik"`
			FormType        string `json:"form_type"`
			FilingDate      string `json:"filing_date"`
			SourceURL       string `json:"source_url"`
			RedirectChain   []struct {
				From            string `json:"from"`
				To              string `json:"to"`
				Status          int    `json:"status"`
				FixtureRedirect bool   `json:"fixture_redirect"`
			} `json:"redirect_chain"`
			Cache struct {
				Key        string `json:"key"`
				Mode       string `json:"mode"`
				Hit        bool   `json:"hit"`
				TTLSeconds int    `json:"ttl_seconds"`
			} `json:"cache"`
			Provenance struct {
				FixtureKey  string `json:"fixture_key"`
				LiveNetwork bool   `json:"live_network"`
			} `json:"provenance"`
		} `json:"results"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "sec_filing_search_ACME_fixture.json"), &searchFixture)
	if !searchFixture.ProviderFree || searchFixture.LiveNetwork || len(searchFixture.Results) != 1 {
		t.Fatalf("search fixture header/counts = %#v", searchFixture)
	}
	if !searchFixture.Query.UserAgentPolicy.RequiredForLive ||
		searchFixture.Query.UserAgentPolicy.DeclaredInFixture ||
		!searchFixture.Query.UserAgentPolicy.CleanSkipWithoutUserAgent ||
		searchFixture.Query.RateLimitPolicy.MaxRequestsPerSecond != 10 ||
		searchFixture.Query.RateLimitPolicy.EnforcedInFixture ||
		!searchFixture.Query.RateLimitPolicy.CleanSkipWithoutPolicy {
		t.Fatalf("SEC user-agent/rate-limit policy incomplete: %#v", searchFixture.Query)
	}
	result := searchFixture.Results[0]
	if result.AccessionNumber == "" || result.CIK == "" || result.FormType != "10-K" || result.FilingDate == "" || result.SourceURL == "" || result.Provenance.FixtureKey == "" || result.Provenance.LiveNetwork {
		t.Fatalf("search result missing SEC provenance fields: %#v", result)
	}
	if len(result.RedirectChain) == 0 || result.RedirectChain[0].Status != 200 || !result.RedirectChain[0].FixtureRedirect || result.Cache.Key == "" || result.Cache.Mode != "fixture_replay" || !result.Cache.Hit || result.Cache.TTLSeconds != 0 {
		t.Fatalf("search result missing redirect/cache metadata: %#v", result)
	}

	var markdownFixture struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		Markdown     string `json:"markdown"`
		Conversion   struct {
			ConversionSource   string `json:"conversion_source"`
			HTMLSupported      bool   `json:"html_supported"`
			PDFSupported       bool   `json:"pdf_supported"`
			HTMLParserBoundary struct {
				Adapter            string            `json:"adapter"`
				AcceptedInput      string            `json:"accepted_input"`
				DependencyImported bool              `json:"dependency_imported"`
				LiveNetwork        bool              `json:"live_network"`
				CleanSkip          bool              `json:"clean_skip"`
				NormalizedDOMRef   string            `json:"normalized_dom_ref"`
				SectionAnchorMap   map[string]string `json:"section_anchor_map"`
				TablePolicy        string            `json:"table_policy"`
			} `json:"html_parser_boundary"`
			PDFToMarkdownBoundary struct {
				Adapter            string   `json:"adapter"`
				AcceptedInput      string   `json:"accepted_input"`
				DependencyImported bool     `json:"dependency_imported"`
				LiveNetwork        bool     `json:"live_network"`
				CleanSkip          bool     `json:"clean_skip"`
				LayoutWarnings     []string `json:"layout_warnings"`
			} `json:"pdf_to_markdown_boundary"`
			RedirectCacheMetadata struct {
				RedirectChain []struct {
					Status int `json:"status"`
				} `json:"redirect_chain"`
				Cache struct {
					Key  string `json:"key"`
					Mode string `json:"mode"`
					Hit  bool   `json:"hit"`
				} `json:"cache"`
			} `json:"redirect_cache_metadata"`
			Warnings []string `json:"warnings"`
		} `json:"conversion"`
		PageSpans []struct {
			Page        int `json:"page"`
			StartOffset int `json:"start_offset"`
			EndOffset   int `json:"end_offset"`
		} `json:"page_spans"`
		Sections []struct {
			SectionID    string `json:"section_id"`
			SectionTitle string `json:"section_title"`
			StartOffset  int    `json:"start_offset"`
			EndOffset    int    `json:"end_offset"`
		} `json:"sections"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "sec_markdown_ACME_10k_fixture.json"), &markdownFixture)
	if !markdownFixture.ProviderFree || markdownFixture.LiveNetwork || markdownFixture.Markdown == "" || len(markdownFixture.PageSpans) < 3 || len(markdownFixture.Sections) < 3 {
		t.Fatalf("markdown fixture header/counts = %#v", markdownFixture)
	}
	if markdownFixture.Conversion.ConversionSource != "sec_html" || !markdownFixture.Conversion.HTMLSupported || !markdownFixture.Conversion.PDFSupported || len(markdownFixture.Conversion.Warnings) == 0 {
		t.Fatalf("conversion boundary missing HTML/PDF support or warnings: %#v", markdownFixture.Conversion)
	}
	htmlBoundary := markdownFixture.Conversion.HTMLParserBoundary
	if htmlBoundary.Adapter != "fixture_html_parser" || htmlBoundary.AcceptedInput != "sec_html" || htmlBoundary.DependencyImported || htmlBoundary.LiveNetwork || !htmlBoundary.CleanSkip || htmlBoundary.NormalizedDOMRef == "" || len(htmlBoundary.SectionAnchorMap) < 3 || htmlBoundary.TablePolicy == "" {
		t.Fatalf("HTML parser boundary incomplete: %#v", htmlBoundary)
	}
	pdfBoundary := markdownFixture.Conversion.PDFToMarkdownBoundary
	if pdfBoundary.Adapter != "fixture_pdf_to_markdown" || pdfBoundary.AcceptedInput != "sec_pdf" || pdfBoundary.DependencyImported || pdfBoundary.LiveNetwork || !pdfBoundary.CleanSkip || len(pdfBoundary.LayoutWarnings) == 0 {
		t.Fatalf("PDF-to-markdown boundary incomplete: %#v", pdfBoundary)
	}
	redirectCache := markdownFixture.Conversion.RedirectCacheMetadata
	if len(redirectCache.RedirectChain) == 0 || redirectCache.RedirectChain[0].Status != 200 || redirectCache.Cache.Key == "" || redirectCache.Cache.Mode != "fixture_replay" || !redirectCache.Cache.Hit {
		t.Fatalf("markdown conversion missing redirect/cache metadata: %#v", redirectCache)
	}
	sections := map[string]bool{}
	for _, section := range markdownFixture.Sections {
		sections[section.SectionID] = true
		if section.SectionTitle == "" || section.StartOffset >= section.EndOffset {
			t.Fatalf("section extraction incomplete: %#v", section)
		}
	}
	for _, want := range []string{"business", "risk_factors", "mda"} {
		if !sections[want] {
			t.Fatalf("missing extracted section %q in %#v", want, sections)
		}
	}

	var chunksFixture struct {
		ProviderFree   bool `json:"provider_free"`
		LiveNetwork    bool `json:"live_network"`
		CorpusManifest struct {
			CorpusID                    string `json:"corpus_id"`
			FixtureKey                  string `json:"fixture_key"`
			ChunkCount                  int    `json:"chunk_count"`
			CitationConsistencyRequired bool   `json:"citation_consistency_required"`
		} `json:"corpus_manifest"`
		AdapterMatrix []struct {
			Stage              string `json:"stage"`
			Input              string `json:"input"`
			Output             string `json:"output"`
			Adapter            string `json:"adapter"`
			FixtureKey         string `json:"fixture_key"`
			LiveNetwork        bool   `json:"live_network"`
			DependencyImported bool   `json:"dependency_imported"`
			CleanSkip          bool   `json:"clean_skip"`
		} `json:"adapter_matrix"`
		Chunks []struct {
			ChunkID      string         `json:"chunk_id"`
			SectionID    string         `json:"section_id"`
			Text         string         `json:"text"`
			Citation     map[string]any `json:"citation"`
			Provenance   map[string]any `json:"provenance"`
			EmbeddingRef map[string]any `json:"embedding_ref"`
		} `json:"chunks"`
		VectorStoreCapabilities struct {
			Adapter            string   `json:"adapter"`
			CapabilityGates    []string `json:"capability_gates"`
			DependencyImported bool     `json:"dependency_imported"`
			CredentialRequired bool     `json:"credential_required"`
			LiveNetwork        bool     `json:"live_network"`
			CleanSkip          bool     `json:"clean_skip"`
		} `json:"vector_store_capabilities"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "rag_chunks_ACME_10k_fixture.json"), &chunksFixture)
	if !chunksFixture.ProviderFree || chunksFixture.LiveNetwork || len(chunksFixture.Chunks) < 3 {
		t.Fatalf("chunk fixture header/counts = %#v", chunksFixture)
	}
	if chunksFixture.CorpusManifest.CorpusID == "" || chunksFixture.CorpusManifest.FixtureKey != "rag:corpus_manifest:ACME:10-K:offline" || chunksFixture.CorpusManifest.ChunkCount != len(chunksFixture.Chunks) || !chunksFixture.CorpusManifest.CitationConsistencyRequired {
		t.Fatalf("chunk fixture missing corpus manifest linkage: %#v", chunksFixture.CorpusManifest)
	}
	matrixStages := map[string]bool{}
	for _, row := range chunksFixture.AdapterMatrix {
		matrixStages[row.Stage] = true
		if row.Input == "" || row.Output == "" || row.Adapter == "" || row.FixtureKey == "" || row.LiveNetwork || row.DependencyImported || !row.CleanSkip {
			t.Fatalf("chunk/citation/vector adapter matrix row incomplete: %#v", row)
		}
	}
	for _, want := range []string{"chunk", "citation", "vector_payload"} {
		if !matrixStages[want] {
			t.Fatalf("adapter matrix missing stage %q in %#v", want, matrixStages)
		}
	}
	for _, chunk := range chunksFixture.Chunks {
		if chunk.ChunkID == "" || chunk.SectionID == "" || chunk.Text == "" || chunk.Citation["source_url"] == "" || chunk.Provenance["accession_number"] == "" || chunk.Provenance["source_fixture_key"] == "" || chunk.Provenance["corpus_id"] == "" || chunk.Provenance["source_offsets"] == nil || chunk.EmbeddingRef["adapter"] == "" || chunk.EmbeddingRef["dependency_imported"] != false || chunk.EmbeddingRef["clean_skip"] != true {
			t.Fatalf("chunk missing citation/provenance/embedding ref: %#v", chunk)
		}
	}
	if chunksFixture.VectorStoreCapabilities.Adapter != "fixture_vector_index" ||
		chunksFixture.VectorStoreCapabilities.DependencyImported ||
		chunksFixture.VectorStoreCapabilities.CredentialRequired ||
		chunksFixture.VectorStoreCapabilities.LiveNetwork ||
		!chunksFixture.VectorStoreCapabilities.CleanSkip ||
		len(chunksFixture.VectorStoreCapabilities.CapabilityGates) < 3 {
		t.Fatalf("chunk fixture missing vector store capability gates: %#v", chunksFixture.VectorStoreCapabilities)
	}

	var queryFixture struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		Query        string `json:"query"`
		Answer       string `json:"answer"`
		Retriever    struct {
			Adapter                    string `json:"adapter"`
			LiveNetwork                bool   `json:"live_network"`
			DependencyImported         bool   `json:"dependency_imported"`
			CleanSkipWithoutDependency bool   `json:"clean_skip_without_dependency"`
			SkipReason                 string `json:"skip_reason"`
			EmbeddingAdapter           struct {
				Adapter            string `json:"adapter"`
				DependencyImported bool   `json:"dependency_imported"`
				CredentialRequired bool   `json:"credential_required"`
				CleanSkip          bool   `json:"clean_skip"`
			} `json:"embedding_adapter"`
			VectorAdapter struct {
				Adapter            string `json:"adapter"`
				DependencyImported bool   `json:"dependency_imported"`
				CredentialRequired bool   `json:"credential_required"`
				CleanSkip          bool   `json:"clean_skip"`
			} `json:"vector_adapter"`
		} `json:"retriever"`
		RetrieverBoundaries []struct {
			ID                 string `json:"id"`
			Adapter            string `json:"adapter"`
			FixtureKey         string `json:"fixture_key"`
			DependencyImported bool   `json:"dependency_imported"`
			CredentialRequired bool   `json:"credential_required"`
			LiveNetwork        bool   `json:"live_network"`
			CleanSkip          bool   `json:"clean_skip"`
		} `json:"retriever_boundaries"`
		RetrievedChunks []struct {
			ChunkID string  `json:"chunk_id"`
			Rank    int     `json:"rank"`
			Score   float64 `json:"score"`
		} `json:"retrieved_chunks"`
		AnswerCitations []struct {
			ChunkID string `json:"chunk_id"`
			Claim   string `json:"claim"`
		} `json:"answer_citations"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "retriever_query_ACME_fixture.json"), &queryFixture)
	if !queryFixture.ProviderFree || queryFixture.LiveNetwork || queryFixture.Query == "" || queryFixture.Answer == "" || len(queryFixture.RetrievedChunks) < 2 || len(queryFixture.AnswerCitations) == 0 {
		t.Fatalf("query fixture header/counts = %#v", queryFixture)
	}
	if queryFixture.Retriever.Adapter != "fixture_vector_index" || queryFixture.Retriever.LiveNetwork || queryFixture.Retriever.DependencyImported {
		t.Fatalf("retriever adapter must stay fixture-only: %#v", queryFixture.Retriever)
	}
	if !queryFixture.Retriever.CleanSkipWithoutDependency || queryFixture.Retriever.SkipReason == "" ||
		queryFixture.Retriever.EmbeddingAdapter.Adapter == "" ||
		queryFixture.Retriever.EmbeddingAdapter.DependencyImported ||
		queryFixture.Retriever.EmbeddingAdapter.CredentialRequired ||
		!queryFixture.Retriever.EmbeddingAdapter.CleanSkip ||
		queryFixture.Retriever.VectorAdapter.Adapter != "fixture_vector_index" ||
		queryFixture.Retriever.VectorAdapter.DependencyImported ||
		queryFixture.Retriever.VectorAdapter.CredentialRequired ||
		!queryFixture.Retriever.VectorAdapter.CleanSkip {
		t.Fatalf("retriever clean-skip metadata incomplete: %#v", queryFixture.Retriever)
	}
	if len(queryFixture.RetrieverBoundaries) < 2 {
		t.Fatalf("retriever boundaries missing: %#v", queryFixture.RetrieverBoundaries)
	}
	for _, boundary := range queryFixture.RetrieverBoundaries {
		if boundary.ID == "" || boundary.Adapter == "" || boundary.FixtureKey == "" || boundary.DependencyImported || boundary.CredentialRequired || boundary.LiveNetwork || !boundary.CleanSkip {
			t.Fatalf("retriever boundary must be fixture-only and clean-skip safe: %#v", boundary)
		}
	}
	for _, chunk := range queryFixture.RetrievedChunks {
		if chunk.ChunkID == "" || chunk.Rank <= 0 || chunk.Score <= 0 {
			t.Fatalf("retrieved chunk ranking incomplete: %#v", chunk)
		}
	}
}

func TestFinRobotDocumentPipelineLivePackageRAGCorpusVectorParity(t *testing.T) {
	base := documentPipelineLivePackageDir(t)

	var corpus struct {
		ProviderFree        bool     `json:"provider_free"`
		LiveNetwork         bool     `json:"live_network"`
		CorpusID            string   `json:"corpus_id"`
		DocumentID          string   `json:"document_id"`
		FixtureKey          string   `json:"fixture_key"`
		SourceFixtureKeys   []string `json:"source_fixture_keys"`
		ChunkFixtureKey     string   `json:"chunk_fixture_key"`
		RetrieverFixtureKey string   `json:"retriever_fixture_key"`
		ChunkIDs            []string `json:"chunk_ids"`
		CitationConsistency struct {
			RequiredForAllChunks       bool     `json:"required_for_all_chunks"`
			RequiredForAnswerCitations bool     `json:"required_for_answer_citations"`
			SourceURL                  string   `json:"source_url"`
			AccessionNumber            string   `json:"accession_number"`
			SectionIDs                 []string `json:"section_ids"`
			OffsetsRequired            bool     `json:"offsets_required"`
		} `json:"citation_consistency"`
		VectorStoreCapabilities struct {
			Adapter         string `json:"adapter"`
			CapabilityGates []struct {
				Capability               string `json:"capability"`
				Required                 bool   `json:"required"`
				AvailableInFixture       bool   `json:"available_in_fixture"`
				RequiresVectorDependency bool   `json:"requires_vector_dependency"`
				CleanSkip                bool   `json:"clean_skip"`
			} `json:"capability_gates"`
			DependencyImported bool   `json:"dependency_imported"`
			CredentialRequired bool   `json:"credential_required"`
			LiveNetwork        bool   `json:"live_network"`
			CleanSkip          bool   `json:"clean_skip"`
			SkipReason         string `json:"skip_reason"`
		} `json:"vector_store_capabilities"`
		RetrieverBoundaries []struct {
			ID                 string `json:"id"`
			Adapter            string `json:"adapter"`
			Input              string `json:"input"`
			Output             string `json:"output"`
			FixtureKey         string `json:"fixture_key"`
			DependencyImported bool   `json:"dependency_imported"`
			CredentialRequired bool   `json:"credential_required"`
			LiveNetwork        bool   `json:"live_network"`
			CleanSkip          bool   `json:"clean_skip"`
		} `json:"retriever_boundaries"`
		OptionalDependencies []struct {
			Name               string `json:"name"`
			Purpose            string `json:"purpose"`
			DependencyImported bool   `json:"dependency_imported"`
			CredentialRequired bool   `json:"credential_required"`
			CleanSkip          bool   `json:"clean_skip"`
			SkipReason         string `json:"skip_reason"`
		} `json:"optional_dependencies"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "rag_corpus_manifest_ACME_fixture.json"), &corpus)
	if !corpus.ProviderFree || corpus.LiveNetwork || corpus.CorpusID == "" || corpus.DocumentID == "" || corpus.FixtureKey != "rag:corpus_manifest:ACME:10-K:offline" {
		t.Fatalf("corpus manifest header incomplete: %#v", corpus)
	}
	if corpus.ChunkFixtureKey != "rag:chunks:ACME:10-K:offline" || corpus.RetrieverFixtureKey != "retriever:ragquery:ACME:offline" || len(corpus.SourceFixtureKeys) < 2 || len(corpus.ChunkIDs) < 3 {
		t.Fatalf("corpus manifest fixture references incomplete: %#v", corpus)
	}
	if !corpus.CitationConsistency.RequiredForAllChunks || !corpus.CitationConsistency.RequiredForAnswerCitations || corpus.CitationConsistency.SourceURL == "" || corpus.CitationConsistency.AccessionNumber == "" || !corpus.CitationConsistency.OffsetsRequired {
		t.Fatalf("corpus citation consistency incomplete: %#v", corpus.CitationConsistency)
	}
	if corpus.VectorStoreCapabilities.Adapter != "fixture_vector_index" ||
		corpus.VectorStoreCapabilities.DependencyImported ||
		corpus.VectorStoreCapabilities.CredentialRequired ||
		corpus.VectorStoreCapabilities.LiveNetwork ||
		!corpus.VectorStoreCapabilities.CleanSkip ||
		!strings.Contains(strings.ToLower(corpus.VectorStoreCapabilities.SkipReason), "langchain") ||
		len(corpus.VectorStoreCapabilities.CapabilityGates) < 3 {
		t.Fatalf("vector store capability gates incomplete: %#v", corpus.VectorStoreCapabilities)
	}
	capabilityGates := map[string]bool{}
	for _, gate := range corpus.VectorStoreCapabilities.CapabilityGates {
		capabilityGates[gate.Capability] = true
		if gate.Capability == "" || (gate.Required && !gate.AvailableInFixture) {
			t.Fatalf("required vector capability gate unavailable: %#v", gate)
		}
		if gate.RequiresVectorDependency && !gate.CleanSkip {
			t.Fatalf("optional vector dependency gate must clean skip: %#v", gate)
		}
	}
	for _, want := range []string{"similarity_search", "metadata_filter", "persistent_index"} {
		if !capabilityGates[want] {
			t.Fatalf("missing vector capability gate %q in %#v", want, capabilityGates)
		}
	}
	for _, boundary := range corpus.RetrieverBoundaries {
		if boundary.ID == "" || boundary.Adapter == "" || boundary.Input == "" || boundary.Output == "" || boundary.FixtureKey == "" || boundary.DependencyImported || boundary.CredentialRequired || boundary.LiveNetwork || !boundary.CleanSkip {
			t.Fatalf("corpus retriever boundary incomplete: %#v", boundary)
		}
	}
	dependencyNames := map[string]bool{}
	for _, dependency := range corpus.OptionalDependencies {
		dependencyNames[dependency.Name] = true
		if dependency.Name == "" || dependency.Purpose == "" || dependency.DependencyImported || dependency.CredentialRequired || !dependency.CleanSkip || dependency.SkipReason == "" {
			t.Fatalf("optional dependency must be clean-skip only: %#v", dependency)
		}
	}
	for _, want := range []string{"langchain", "vector_store"} {
		if !dependencyNames[want] {
			t.Fatalf("missing optional dependency clean skip %q in %#v", want, dependencyNames)
		}
	}

	var chunks struct {
		CorpusManifest struct {
			CorpusID   string `json:"corpus_id"`
			FixtureKey string `json:"fixture_key"`
		} `json:"corpus_manifest"`
		Chunks []struct {
			ChunkID   string `json:"chunk_id"`
			SectionID string `json:"section_id"`
			Citation  struct {
				SourceURL   string `json:"source_url"`
				Page        int    `json:"page"`
				StartOffset int    `json:"start_offset"`
				EndOffset   int    `json:"end_offset"`
			} `json:"citation"`
			Provenance struct {
				CorpusID        string `json:"corpus_id"`
				AccessionNumber string `json:"accession_number"`
			} `json:"provenance"`
		} `json:"chunks"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "rag_chunks_ACME_10k_fixture.json"), &chunks)
	if chunks.CorpusManifest.CorpusID != corpus.CorpusID || chunks.CorpusManifest.FixtureKey != corpus.FixtureKey {
		t.Fatalf("chunk fixture corpus linkage = %#v, want corpus %q fixture %q", chunks.CorpusManifest, corpus.CorpusID, corpus.FixtureKey)
	}
	chunkByID := map[string]struct {
		sectionID   string
		sourceURL   string
		page        int
		startOffset int
		endOffset   int
	}{}
	for _, chunk := range chunks.Chunks {
		if chunk.Provenance.CorpusID != corpus.CorpusID || chunk.Provenance.AccessionNumber != corpus.CitationConsistency.AccessionNumber {
			t.Fatalf("chunk provenance does not match corpus: %#v", chunk)
		}
		if chunk.Citation.SourceURL != corpus.CitationConsistency.SourceURL || chunk.Citation.StartOffset >= chunk.Citation.EndOffset {
			t.Fatalf("chunk citation does not match corpus consistency: %#v", chunk)
		}
		chunkByID[chunk.ChunkID] = struct {
			sectionID   string
			sourceURL   string
			page        int
			startOffset int
			endOffset   int
		}{sectionID: chunk.SectionID, sourceURL: chunk.Citation.SourceURL, page: chunk.Citation.Page, startOffset: chunk.Citation.StartOffset, endOffset: chunk.Citation.EndOffset}
	}
	if len(chunkByID) != len(corpus.ChunkIDs) {
		t.Fatalf("chunk count = %d, corpus chunk ids = %d", len(chunkByID), len(corpus.ChunkIDs))
	}
	for _, chunkID := range corpus.ChunkIDs {
		if _, ok := chunkByID[chunkID]; !ok {
			t.Fatalf("corpus chunk id %q missing from chunk fixture", chunkID)
		}
	}

	var query struct {
		RetrievedChunks []struct {
			ChunkID   string `json:"chunk_id"`
			SectionID string `json:"section_id"`
			Citation  struct {
				SourceURL   string `json:"source_url"`
				Page        int    `json:"page"`
				StartOffset int    `json:"start_offset"`
				EndOffset   int    `json:"end_offset"`
			} `json:"citation"`
		} `json:"retrieved_chunks"`
		AnswerCitations []struct {
			ChunkID   string `json:"chunk_id"`
			Page      int    `json:"page"`
			SourceURL string `json:"source_url"`
		} `json:"answer_citations"`
		Provenance struct {
			ChunkFixtureKey string `json:"chunk_fixture_key"`
		} `json:"provenance"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "retriever_query_ACME_fixture.json"), &query)
	if query.Provenance.ChunkFixtureKey != corpus.ChunkFixtureKey {
		t.Fatalf("retriever provenance chunk fixture = %q, want %q", query.Provenance.ChunkFixtureKey, corpus.ChunkFixtureKey)
	}
	retrieved := map[string]bool{}
	for _, chunk := range query.RetrievedChunks {
		retrieved[chunk.ChunkID] = true
		source, ok := chunkByID[chunk.ChunkID]
		if !ok {
			t.Fatalf("retrieved chunk %q missing from chunk fixture", chunk.ChunkID)
		}
		if chunk.SectionID != source.sectionID || chunk.Citation.SourceURL != source.sourceURL || chunk.Citation.Page != source.page || chunk.Citation.StartOffset != source.startOffset || chunk.Citation.EndOffset != source.endOffset {
			t.Fatalf("retrieved citation mismatch for %q: %#v vs %#v", chunk.ChunkID, chunk.Citation, source)
		}
	}
	for _, citation := range query.AnswerCitations {
		source, ok := chunkByID[citation.ChunkID]
		if !ok || !retrieved[citation.ChunkID] {
			t.Fatalf("answer citation %q must reference a retrieved chunk", citation.ChunkID)
		}
		if citation.SourceURL != source.sourceURL || citation.Page != source.page {
			t.Fatalf("answer citation mismatch for %q: %#v vs %#v", citation.ChunkID, citation, source)
		}
	}
}

func TestFinRobotDocumentPipelineLivePackageGenericMemoryStoreIntegration(t *testing.T) {
	base := documentPipelineLivePackageDir(t)
	assertDocumentPipelineExternalGenericMemoryBoundary(t, base, "package.manifest.json")
	assertDocumentPipelineExternalGenericMemoryBoundary(t, base, filepath.Join("contracts", "adapter_boundary_contract.json"))
	assertDocumentPipelineExternalGenericMemoryBoundary(t, base, filepath.Join("fixtures", "adapter_boundary_fixture.json"))
}

func TestFinRobotDocumentPipelineLivePackageNoLiveImports(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(documentPipelineLivePackageDir(t), "main.leia"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pattern := range []string{
		`(?m)^\s*import\s+`,
		`(?m)^\s*use\s+`,
		`(?m)^\s*load\s*\(`,
		`(?m)^\s*require\s*\(`,
		`(?m)^\s*(requests|http|sec_api|marker|pymupdf|fitz|pdfplumber|redis|postgres|chromadb|faiss|pinecone|weaviate|qdrant|langchain|openai)\s*[.(]`,
	} {
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("main.leia contains live dependency loader matching %q", pattern)
		}
	}
}

func TestFinRobotDocumentPipelineLivePackageNoLiveNetworkOrDependencyFlags(t *testing.T) {
	base := documentPipelineLivePackageDir(t)
	for _, rel := range []string{
		"package.manifest.json",
		"contracts/document_pipeline_contract.json",
		"contracts/adapter_boundary_contract.json",
		"fixtures/provider_free_fixture_index.json",
		"fixtures/sec_filing_search_ACME_fixture.json",
		"fixtures/sec_markdown_ACME_10k_fixture.json",
		"fixtures/rag_chunks_ACME_10k_fixture.json",
		"fixtures/rag_corpus_manifest_ACME_fixture.json",
		"fixtures/retriever_query_ACME_fixture.json",
		"fixtures/adapter_boundary_fixture.json",
	} {
		var value any
		decodeDocumentPipelineJSONFile(t, filepath.Join(base, rel), &value)
		assertNoEnabledLiveOrDependencyFlags(t, rel, value)
	}
}

func TestFinRobotDocumentPipelineLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(documentPipelineLivePackageDir(t), "main.leia")

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
			got, err := vm.Get("document_pipeline_live_package_summary")
			if err != nil {
				t.Fatalf("Get document_pipeline_live_package_summary: %v", err)
			}
			want := "document_pipeline_live_package modules=4 adapters=8 provider_free=true live_network=false imports=false fixtures=6"
			if got != want {
				t.Fatalf("document_pipeline_live_package_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func documentPipelineLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "document_pipeline")
}

func loadDocumentPipelineLiveManifest(t *testing.T, base string) documentPipelineLiveManifest {
	t.Helper()
	var manifest documentPipelineLiveManifest
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertDocumentPipelineJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeDocumentPipelineJSONFile(t, path, &value)
}

func assertDocumentPipelineSchemaRequired(t *testing.T, path string, fields []string) {
	t.Helper()
	assertDocumentPipelineNestedSchemaRequired(t, path, nil, fields)
}

func assertDocumentPipelineNestedSchemaRequired(t *testing.T, path string, objectPath []string, fields []string) {
	t.Helper()
	var schema map[string]any
	decodeDocumentPipelineJSONFile(t, path, &schema)
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

func decodeDocumentPipelineJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func assertNoEnabledLiveOrDependencyFlags(t *testing.T, path string, value any) {
	t.Helper()
	var walk func(string, any)
	walk = func(jsonPath string, node any) {
		switch typed := node.(type) {
		case map[string]any:
			for key, child := range typed {
				childPath := jsonPath + "." + key
				switch key {
				case "live_network", "live_network_default", "real_dependency_imports", "real_dependency_import_default", "dependency_imported", "credential_required", "provider_credentials_required", "declared_in_fixture", "enforced_in_fixture":
					if enabled, ok := child.(bool); ok && enabled {
						t.Fatalf("%s enables live network, dependency import, credential, or live policy flag", childPath)
					}
				}
				walk(childPath, child)
			}
		case []any:
			for i, child := range typed {
				walk(fmt.Sprintf("%s[%d]", jsonPath, i), child)
			}
		}
	}
	walk(path, value)
}
