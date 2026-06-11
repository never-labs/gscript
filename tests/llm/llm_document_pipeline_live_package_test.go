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
	for _, want := range []string{"SEC", "Marker", "embedding", "vector", "retriever"} {
		if !strings.Contains(manifest.Credentials.Policy, want) {
			t.Fatalf("credential policy should name %q boundary: %q", want, manifest.Credentials.Policy)
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
	for _, key := range []string{"filing_search_result", "document_markdown", "section_extraction", "rag_chunk", "retriever_query_result", "adapter_boundary"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertDocumentPipelineJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "sec_search", "markdown_conversion", "rag_index", "retriever_query", "adapter_boundary"} {
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
	for _, want := range []string{"sec", "html/pdf", "section", "chunk", "citation", "provenance", "retriever", "fixture"} {
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
	for _, field := range []string{"filing_search", "html_pdf_to_markdown", "section_extraction", "chunk_citation_provenance", "vector_retriever_adapter"} {
		if contract.FieldContracts[field] == nil {
			t.Fatalf("missing field contract %q", field)
		}
	}
	acceptance := strings.ToLower(strings.Join(contract.AcceptanceGates, " "))
	for _, want := range []string{"sec filing", "html", "pdf", "section", "rag chunks", "retriever", "fixture replay"} {
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
		assertDocumentPipelineJSONFile(t, filepath.Join(base, fixture.Path))
		assertDocumentPipelineJSONFile(t, filepath.Join(base, fixture.Schema))
	}
}

func TestFinRobotDocumentPipelineLivePackageAdapterBoundaries(t *testing.T) {
	base := documentPipelineLivePackageDir(t)
	manifest := loadDocumentPipelineLiveManifest(t, base)

	var ids []string
	fixtures := map[string]bool{}
	for _, boundary := range manifest.AdapterBoundaries {
		ids = append(ids, boundary.ID)
		if boundary.ID == "" || boundary.DisplayName == "" || boundary.Capability == "" || boundary.FixtureKey == "" || boundary.Schema != "adapter_boundary" {
			t.Fatalf("adapter boundary metadata incomplete: %#v", boundary)
		}
		if boundary.LiveNetwork || boundary.DependencyImported || boundary.CredentialRequired || !boundary.CleanSkip {
			t.Fatalf("adapter boundary must be fixture-only and clean-skip safe: %#v", boundary)
		}
		if fixtures[boundary.FixtureKey] {
			t.Fatalf("duplicate fixture key %q", boundary.FixtureKey)
		}
		fixtures[boundary.FixtureKey] = true
	}
	sort.Strings(ids)
	wantIDs := []string{"document_converter", "sec_filing_client", "vector_retriever"}
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
	if len(boundaryContract.Boundaries) != 3 {
		t.Fatalf("boundary contract count = %d, want 3", len(boundaryContract.Boundaries))
	}
	for _, boundary := range boundaryContract.Boundaries {
		if boundary.LiveNetwork || boundary.DependencyImported || boundary.CredentialRequired || !boundary.CleanSkip {
			t.Fatalf("boundary contract must not enable live adapters: %#v", boundary)
		}
	}
}

func TestFinRobotDocumentPipelineLivePackageFixtureShape(t *testing.T) {
	base := documentPipelineLivePackageDir(t)

	var searchFixture struct {
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
		Results      []struct {
			AccessionNumber string `json:"accession_number"`
			CIK             string `json:"cik"`
			FormType        string `json:"form_type"`
			FilingDate      string `json:"filing_date"`
			SourceURL       string `json:"source_url"`
			Provenance      struct {
				FixtureKey  string `json:"fixture_key"`
				LiveNetwork bool   `json:"live_network"`
			} `json:"provenance"`
		} `json:"results"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "sec_filing_search_ACME_fixture.json"), &searchFixture)
	if !searchFixture.ProviderFree || searchFixture.LiveNetwork || len(searchFixture.Results) != 1 {
		t.Fatalf("search fixture header/counts = %#v", searchFixture)
	}
	result := searchFixture.Results[0]
	if result.AccessionNumber == "" || result.CIK == "" || result.FormType != "10-K" || result.FilingDate == "" || result.SourceURL == "" || result.Provenance.FixtureKey == "" || result.Provenance.LiveNetwork {
		t.Fatalf("search result missing SEC provenance fields: %#v", result)
	}

	var markdownFixture struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		Markdown     string `json:"markdown"`
		Conversion   struct {
			ConversionSource string   `json:"conversion_source"`
			HTMLSupported    bool     `json:"html_supported"`
			PDFSupported     bool     `json:"pdf_supported"`
			Warnings         []string `json:"warnings"`
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
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
		Chunks       []struct {
			ChunkID      string         `json:"chunk_id"`
			SectionID    string         `json:"section_id"`
			Text         string         `json:"text"`
			Citation     map[string]any `json:"citation"`
			Provenance   map[string]any `json:"provenance"`
			EmbeddingRef map[string]any `json:"embedding_ref"`
		} `json:"chunks"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "rag_chunks_ACME_10k_fixture.json"), &chunksFixture)
	if !chunksFixture.ProviderFree || chunksFixture.LiveNetwork || len(chunksFixture.Chunks) < 3 {
		t.Fatalf("chunk fixture header/counts = %#v", chunksFixture)
	}
	for _, chunk := range chunksFixture.Chunks {
		if chunk.ChunkID == "" || chunk.SectionID == "" || chunk.Text == "" || chunk.Citation["source_url"] == "" || chunk.Provenance["accession_number"] == "" || chunk.EmbeddingRef["adapter"] == "" {
			t.Fatalf("chunk missing citation/provenance/embedding ref: %#v", chunk)
		}
	}

	var queryFixture struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		Query        string `json:"query"`
		Answer       string `json:"answer"`
		Retriever    struct {
			Adapter            string `json:"adapter"`
			LiveNetwork        bool   `json:"live_network"`
			DependencyImported bool   `json:"dependency_imported"`
		} `json:"retriever"`
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
	for _, chunk := range queryFixture.RetrievedChunks {
		if chunk.ChunkID == "" || chunk.Rank <= 0 || chunk.Score <= 0 {
			t.Fatalf("retrieved chunk ranking incomplete: %#v", chunk)
		}
	}
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
		`(?m)^\s*(requests|http|sec_api|marker|pymupdf|fitz|pdfplumber|chromadb|faiss|pinecone|weaviate|qdrant|langchain|openai)\s*[.(]`,
	} {
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("main.leia contains live dependency loader matching %q", pattern)
		}
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
			want := "document_pipeline_live_package modules=4 adapters=3 provider_free=true live_network=false imports=false fixtures=5"
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
