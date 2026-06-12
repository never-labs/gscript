package leia_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericDocumentRAGPipelineLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericDocumentRAGPipelinePackageDir(t)

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
		manifest.ID != "generic-document-rag-pipeline" ||
		manifest.PackageName != "leia-generic-ai-document-rag-pipeline" ||
		manifest.PackageBoundaryID != "generic-ai-document-rag-pipeline" ||
		manifest.CapabilityID != "generic.ai.document.rag" {
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
		"generic.ai.document.rag",
		"generic.ai.document.convert",
		"generic.ai.document.section.extract",
		"generic.ai.rag.chunk",
		"generic.ai.rag.corpus_manifest",
		"generic.ai.rag.retrieve",
		"generic.ai.rag.citation_consistency",
		"generic.ai.rag.adapter.clean_skip",
	} {
		if !genericLivePackageContains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}

	var contract struct {
		SchemaVersion     int      `json:"schema_version"`
		PackageBoundaryID string   `json:"package_boundary_id"`
		PackageName       string   `json:"package_name"`
		Entrypoint        string   `json:"entrypoint"`
		ProviderFree      bool     `json:"provider_free"`
		DomainSpecific    bool     `json:"domain_specific"`
		LiveNetwork       bool     `json:"live_network"`
		LiveModel         bool     `json:"live_model"`
		Capabilities      []string `json:"capabilities"`
		AdapterContract   struct {
			ConverterDependencyImported bool `json:"converter_dependency_imported"`
			ChunkerDependencyImported   bool `json:"chunker_dependency_imported"`
			EmbeddingDependencyImported bool `json:"embedding_dependency_imported"`
			VectorDependencyImported    bool `json:"vector_dependency_imported"`
			RetrieverDependencyImported bool `json:"retriever_dependency_imported"`
			CredentialValuesAllowed     bool `json:"credential_values_allowed"`
			LiveNetworkAllowed          bool `json:"live_network_allowed"`
			CleanSkipRequired           bool `json:"clean_skip_required"`
			ProviderFreeReplayRequired  bool `json:"provider_free_replay_required"`
		} `json:"adapter_contract"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Contracts["contract"]), &contract)
	if contract.SchemaVersion != 1 || contract.PackageBoundaryID != manifest.PackageBoundaryID ||
		contract.PackageName != "generic.ai.document.rag" || contract.Entrypoint != "ai.document.rag_pipeline" ||
		!contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork || contract.LiveModel {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	if contract.AdapterContract.ConverterDependencyImported ||
		contract.AdapterContract.ChunkerDependencyImported ||
		contract.AdapterContract.EmbeddingDependencyImported ||
		contract.AdapterContract.VectorDependencyImported ||
		contract.AdapterContract.RetrieverDependencyImported ||
		contract.AdapterContract.CredentialValuesAllowed ||
		contract.AdapterContract.LiveNetworkAllowed ||
		!contract.AdapterContract.CleanSkipRequired ||
		!contract.AdapterContract.ProviderFreeReplayRequired {
		t.Fatalf("adapter contract is not provider-free: %#v", contract.AdapterContract)
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
	if fixture.FixtureKey != "generic:document_rag:pipeline:offline" ||
		fixture.Capability != "generic.ai.document.rag" ||
		fixture.Path != manifest.Fixtures["document_rag_pipeline"] ||
		fixture.Schema != manifest.Schemas["document_rag_pipeline"] ||
		fixture.Metadata["replay_ready"] != true {
		t.Fatalf("fixture index entry mismatch: %#v", fixture)
	}
}

func TestGenericDocumentRAGPipelineLivePackageFixtureShape(t *testing.T) {
	base := genericDocumentRAGPipelinePackageDir(t)
	fixture := loadGenericDocumentRAGPipelineFixture(t, filepath.Join(base, "fixtures", "generic_document_rag_pipeline_fixture.json"))
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports {
		t.Fatalf("fixture must stay provider-free and offline: %#v", fixture)
	}
	if fixture.DocumentSource.DocumentID == "" || fixture.DocumentSource.SourceRef == "" || fixture.DocumentSource.SourceType == "" {
		t.Fatalf("document source incomplete: %#v", fixture.DocumentSource)
	}
	if len(fixture.Sections) != 3 || len(fixture.Chunks) != 3 || fixture.CorpusManifest.ChunkCount != len(fixture.Chunks) {
		t.Fatalf("section/chunk/corpus counts drifted: sections=%d chunks=%d manifest=%#v", len(fixture.Sections), len(fixture.Chunks), fixture.CorpusManifest)
	}

	chunkIDs := map[string]bool{}
	for _, chunk := range fixture.Chunks {
		if chunk.ChunkID == "" || chunk.SectionID == "" || chunk.Text == "" || chunk.Citation.SourceRef == "" {
			t.Fatalf("chunk is incomplete: %#v", chunk)
		}
		if chunk.Provenance.SourceDocumentRef != fixture.DocumentSource.SourceRef {
			t.Fatalf("chunk provenance source ref = %q, want %q", chunk.Provenance.SourceDocumentRef, fixture.DocumentSource.SourceRef)
		}
		if chunk.EmbeddingRef.DependencyImported || chunk.EmbeddingRef.CredentialRequired || !chunk.EmbeddingRef.CleanSkip {
			t.Fatalf("embedding ref must be clean-skip: %#v", chunk.EmbeddingRef)
		}
		chunkIDs[chunk.ChunkID] = true
	}
	for _, retrieved := range fixture.RetrievedChunks {
		if !chunkIDs[retrieved.ChunkID] {
			t.Fatalf("retrieved chunk %q does not resolve in chunk set", retrieved.ChunkID)
		}
		if retrieved.Rank <= 0 || retrieved.Score <= 0 || retrieved.Citation.SourceRef == "" {
			t.Fatalf("retrieved chunk is incomplete: %#v", retrieved)
		}
	}
	for _, citation := range fixture.AnswerCitations {
		if !chunkIDs[citation.ChunkID] {
			t.Fatalf("answer citation %q does not resolve in chunk set", citation.ChunkID)
		}
	}
	for _, boundary := range fixture.AdapterBoundaries {
		if boundary.DependencyImported || boundary.CredentialRequired || boundary.LiveNetwork || !boundary.CleanSkip {
			t.Fatalf("adapter boundary must be provider-free clean-skip: %#v", boundary)
		}
	}
}

func TestGenericDocumentRAGPipelineLivePackageIsDomainNeutral(t *testing.T) {
	base := genericDocumentRAGPipelinePackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"acme", "sec.gov", "accession_number", "ticker", "cik", "form_type", "10-k", "10q", "finrobot.", "finrobot_"} {
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

func TestGenericDocumentRAGPipelineLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericDocumentRAGPipelinePackageDir(t)
	schema := filepath.Join(base, "schemas", "generic_document_rag_pipeline_v1.schema.json")
	assertDocumentPipelineSchemaRequired(t, schema, []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "document_source", "document_markdown", "sections", "chunk_policy", "corpus_manifest", "chunks", "retriever_query", "retrieved_chunks", "answer_citations", "adapter_boundaries", "provenance"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "document_source"}, []string{"document_id", "source_ref", "source_type", "media_type"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "sections", "items"}, []string{"section_id", "section_title", "start_offset", "end_offset"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "chunks", "items"}, []string{"chunk_id", "section_id", "text", "token_count", "citation", "provenance", "embedding_ref"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "retrieved_chunks", "items"}, []string{"chunk_id", "rank", "score", "section_id", "citation"})
}

func TestGenericDocumentRAGPipelineLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericDocumentRAGPipelinePackageDir(t), "main.leia")
	want := "generic_document_rag_pipeline_live_package capability=generic.ai.document.rag entrypoint=ai.document.rag_pipeline chunks=3 retrieved=2 adapters=5 provider_free=true live_network=false imports=false"
	for _, result := range runFinRobotLivePackageSummarySmoke(t, path, "generic_document_rag_pipeline_live_package_summary", "generic_document_rag_pipeline_live_package", leia.LibString) {
		if result.Summary != want {
			t.Fatalf("summary = %#v, want %#v", result.Summary, want)
		}
	}
}

type genericDocumentRAGPipelineFixture struct {
	ProviderFree          bool `json:"provider_free"`
	LiveNetwork           bool `json:"live_network"`
	RealDependencyImports bool `json:"real_dependency_imports"`
	DocumentSource        struct {
		DocumentID string `json:"document_id"`
		SourceRef  string `json:"source_ref"`
		SourceType string `json:"source_type"`
	} `json:"document_source"`
	Sections []struct {
		SectionID    string `json:"section_id"`
		SectionTitle string `json:"section_title"`
		StartOffset  int    `json:"start_offset"`
		EndOffset    int    `json:"end_offset"`
	} `json:"sections"`
	CorpusManifest struct {
		CorpusID   string `json:"corpus_id"`
		ChunkCount int    `json:"chunk_count"`
	} `json:"corpus_manifest"`
	Chunks []struct {
		ChunkID    string `json:"chunk_id"`
		SectionID  string `json:"section_id"`
		Text       string `json:"text"`
		TokenCount int    `json:"token_count"`
		Citation   struct {
			SourceRef    string `json:"source_ref"`
			SectionTitle string `json:"section_title"`
		} `json:"citation"`
		Provenance struct {
			CorpusID          string `json:"corpus_id"`
			SourceDocumentRef string `json:"source_document_ref"`
		} `json:"provenance"`
		EmbeddingRef struct {
			DependencyImported bool `json:"dependency_imported"`
			CredentialRequired bool `json:"credential_required"`
			CleanSkip          bool `json:"clean_skip"`
		} `json:"embedding_ref"`
	} `json:"chunks"`
	RetrievedChunks []struct {
		ChunkID  string  `json:"chunk_id"`
		Rank     int     `json:"rank"`
		Score    float64 `json:"score"`
		Citation struct {
			SourceRef string `json:"source_ref"`
		} `json:"citation"`
	} `json:"retrieved_chunks"`
	AnswerCitations []struct {
		ChunkID string `json:"chunk_id"`
		Claim   string `json:"claim"`
	} `json:"answer_citations"`
	AdapterBoundaries []struct {
		ID                 string `json:"id"`
		DependencyImported bool   `json:"dependency_imported"`
		CredentialRequired bool   `json:"credential_required"`
		LiveNetwork        bool   `json:"live_network"`
		CleanSkip          bool   `json:"clean_skip"`
	} `json:"adapter_boundaries"`
}

func loadGenericDocumentRAGPipelineFixture(t *testing.T, path string) genericDocumentRAGPipelineFixture {
	t.Helper()
	var fixture genericDocumentRAGPipelineFixture
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func genericDocumentRAGPipelinePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_document_rag_pipeline")
}
