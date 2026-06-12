package leia_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericMemoryStoreLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericMemoryStorePackageDir(t)

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
		CapabilitySurfaces []string          `json:"capability_surfaces"`
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
		manifest.ID != "generic-memory-store" ||
		manifest.PackageName != "leia-generic-ai-memory-store" ||
		manifest.PackageBoundaryID != "generic-ai-memory-store" ||
		manifest.CapabilityID != "generic.ai.memory.store" {
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
		"generic.ai.memory.store",
		"generic.ai.memory.namespace",
		"generic.ai.memory.retrieve",
		"generic.ai.memory.context_window",
		"generic.ai.memory.retrieval_policy",
		"generic.ai.memory.provenance",
		"generic.ai.memory.clean_skip",
	} {
		if !genericLivePackageContains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}

	var contract struct {
		SchemaVersion     int      `json:"schema_version"`
		PackageBoundaryID string   `json:"package_boundary_id"`
		ProviderFree      bool     `json:"provider_free"`
		DomainSpecific    bool     `json:"domain_specific"`
		LiveNetwork       bool     `json:"live_network"`
		LiveModel         bool     `json:"live_model"`
		Credentials       bool     `json:"credentials_required"`
		Capabilities      []string `json:"capabilities"`
		RequiredFields    []string `json:"required_fields"`
		AdapterContract   struct {
			ConcreteProviderNamesAllowed bool `json:"concrete_provider_names_allowed"`
			LiveDependencyImportsAllowed bool `json:"live_dependency_imports_allowed"`
			CredentialValuesAllowed      bool `json:"credential_values_allowed"`
			CleanSkipRequired            bool `json:"clean_skip_required"`
			ProviderFreeReplayRequired   bool `json:"provider_free_replay_required"`
		} `json:"adapter_contract"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Contracts["contract"]), &contract)
	if contract.SchemaVersion != 1 || contract.PackageBoundaryID != manifest.PackageBoundaryID ||
		!contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork || contract.LiveModel || contract.Credentials {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	if contract.AdapterContract.ConcreteProviderNamesAllowed ||
		contract.AdapterContract.LiveDependencyImportsAllowed ||
		contract.AdapterContract.CredentialValuesAllowed ||
		!contract.AdapterContract.CleanSkipRequired ||
		!contract.AdapterContract.ProviderFreeReplayRequired {
		t.Fatalf("adapter contract is not provider-free: %#v", contract.AdapterContract)
	}
	for _, want := range []string{"memory_store.store_id", "namespace_policy.namespace", "retrieval_results.rank", "context_window.included_memory_ids", "provenance.fixture_key"} {
		if !genericLivePackageContains(contract.RequiredFields, want) {
			t.Fatalf("contract required_fields missing %q: %#v", want, contract.RequiredFields)
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
	if fixture.FixtureKey != "generic:memory_store:offline" ||
		fixture.Capability != "generic.ai.memory.store" ||
		fixture.Path != manifest.Fixtures["generic_memory_store"] ||
		fixture.Schema != manifest.Schemas["generic_memory_store"] ||
		fixture.Metadata["replay_ready"] != true {
		t.Fatalf("fixture index entry mismatch: %#v", fixture)
	}
}

func TestGenericMemoryStoreLivePackageFixtureShape(t *testing.T) {
	base := genericMemoryStorePackageDir(t)
	memory := loadGenericMemoryStoreFixture(t, filepath.Join(base, "fixtures", "generic_memory_store_fixture.json"))
	assertGenericMemoryStoreFixture(t, memory, nil)
}

func TestGenericMemoryStoreLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericMemoryStorePackageDir(t)
	schema := filepath.Join(base, "schemas", "generic_memory_store_v1.schema.json")
	assertDocumentPipelineSchemaRequired(t, schema, []string{"schema_version", "provider_free", "live_network", "memory_store", "namespace_policy", "memory_items", "retrieval_policy", "retrieval_results", "context_window", "adapter_boundaries", "provenance"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "memory_store"}, []string{"store_id", "store_type", "persistent", "dependency_imported", "credential_required", "live_network", "clean_skip"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "memory_items", "items"}, []string{"memory_id", "text", "tags", "source_ref", "chunk_id", "provenance"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "retrieval_results", "items"}, []string{"memory_id", "rank", "score", "reason", "context_ref"})
}

func TestGenericMemoryStoreLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericMemoryStorePackageDir(t), "main.leia")
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
			got, err := vm.Get("generic_memory_store_live_package_summary")
			if err != nil {
				t.Fatalf("Get generic_memory_store_live_package_summary: %v", err)
			}
			want := "generic_memory_store_live_package capability=generic.ai.memory.store fixture=generic:memory_store:offline ranked_items=2 provider_free=true live_network=false imports=false"
			if got != want {
				t.Fatalf("summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

type genericMemoryStoreFixture struct {
	ProviderFree bool `json:"provider_free"`
	LiveNetwork  bool `json:"live_network"`
	MemoryStore  struct {
		StoreID            string `json:"store_id"`
		StoreType          string `json:"store_type"`
		Persistent         bool   `json:"persistent"`
		DependencyImported bool   `json:"dependency_imported"`
		CredentialRequired bool   `json:"credential_required"`
		LiveNetwork        bool   `json:"live_network"`
		CleanSkip          bool   `json:"clean_skip"`
		SkipReason         string `json:"skip_reason"`
	} `json:"memory_store"`
	NamespacePolicy struct {
		Namespace    string   `json:"namespace"`
		TenantScope  string   `json:"tenant_scope"`
		AllowedTags  []string `json:"allowed_tags"`
		PIIPolicy    string   `json:"pii_policy"`
		SecretPolicy string   `json:"secret_policy"`
	} `json:"namespace_policy"`
	MemoryItems []struct {
		MemoryID   string   `json:"memory_id"`
		Text       string   `json:"text"`
		Tags       []string `json:"tags"`
		SourceRef  string   `json:"source_ref"`
		ChunkID    string   `json:"chunk_id"`
		Provenance struct {
			FixtureKey          string `json:"fixture_key"`
			SourceFixtureKey    string `json:"source_fixture_key"`
			RetrieverFixtureKey string `json:"retriever_fixture_key"`
		} `json:"provenance"`
	} `json:"memory_items"`
	RetrievalPolicy struct {
		Query                       string         `json:"query"`
		TopK                        int            `json:"top_k"`
		Ranking                     string         `json:"ranking"`
		MetadataFilters             map[string]any `json:"metadata_filters"`
		EmbeddingDependencyImported bool           `json:"embedding_dependency_imported"`
		VectorDependencyImported    bool           `json:"vector_dependency_imported"`
		ProviderCredentialsRequired bool           `json:"provider_credentials_required"`
	} `json:"retrieval_policy"`
	RetrievalResults []struct {
		MemoryID   string  `json:"memory_id"`
		Rank       int     `json:"rank"`
		Score      float64 `json:"score"`
		Reason     string  `json:"reason"`
		ContextRef string  `json:"context_ref"`
	} `json:"retrieval_results"`
	ContextWindow struct {
		Strategy           string   `json:"strategy"`
		MaxContextItems    int      `json:"max_context_items"`
		TokenBudget        int      `json:"token_budget"`
		IncludedMemoryIDs  []string `json:"included_memory_ids"`
		ExcludedMemoryIDs  []string `json:"excluded_memory_ids"`
		DeterministicOrder bool     `json:"deterministic_order"`
	} `json:"context_window"`
	AdapterBoundaries []struct {
		ID                 string `json:"id"`
		Adapter            string `json:"adapter"`
		Capability         string `json:"capability"`
		FixtureKey         string `json:"fixture_key"`
		DependencyImported bool   `json:"dependency_imported"`
		CredentialRequired bool   `json:"credential_required"`
		LiveNetwork        bool   `json:"live_network"`
		CleanSkip          bool   `json:"clean_skip"`
	} `json:"adapter_boundaries"`
	Provenance struct {
		FixtureKey                string `json:"fixture_key"`
		LinkedChunkFixtureKey     string `json:"linked_chunk_fixture_key"`
		LinkedRetrieverFixtureKey string `json:"linked_retriever_fixture_key"`
	} `json:"provenance"`
}

func loadGenericMemoryStoreFixture(t *testing.T, path string) genericMemoryStoreFixture {
	t.Helper()
	var memory genericMemoryStoreFixture
	decodeDocumentPipelineJSONFile(t, path, &memory)
	return memory
}

func assertGenericMemoryStoreFixture(t *testing.T, memory genericMemoryStoreFixture, chunkIDs map[string]bool) {
	t.Helper()
	if !memory.ProviderFree || memory.LiveNetwork || len(memory.MemoryItems) < 2 || len(memory.RetrievalResults) < 2 {
		t.Fatalf("memory fixture header/counts = %#v", memory)
	}
	if memory.MemoryStore.StoreType != "fixture_in_memory" ||
		memory.MemoryStore.Persistent ||
		memory.MemoryStore.DependencyImported ||
		memory.MemoryStore.CredentialRequired ||
		memory.MemoryStore.LiveNetwork ||
		!memory.MemoryStore.CleanSkip ||
		!strings.Contains(strings.ToLower(memory.MemoryStore.SkipReason), "static json") {
		t.Fatalf("memory store must remain provider-free fixture storage: %#v", memory.MemoryStore)
	}
	if memory.NamespacePolicy.Namespace == "" ||
		memory.NamespacePolicy.TenantScope != "fixture_only" ||
		!strings.Contains(strings.ToLower(memory.NamespacePolicy.SecretPolicy), "no_secrets") {
		t.Fatalf("namespace policy incomplete: %#v", memory.NamespacePolicy)
	}
	if memory.RetrievalPolicy.Query == "" ||
		memory.RetrievalPolicy.TopK != len(memory.RetrievalResults) ||
		memory.RetrievalPolicy.Ranking != "deterministic_fixture_score" ||
		memory.RetrievalPolicy.EmbeddingDependencyImported ||
		memory.RetrievalPolicy.VectorDependencyImported ||
		memory.RetrievalPolicy.ProviderCredentialsRequired {
		t.Fatalf("retrieval policy must be deterministic and provider-free: %#v", memory.RetrievalPolicy)
	}
	if memory.ContextWindow.Strategy == "" ||
		memory.ContextWindow.MaxContextItems != len(memory.ContextWindow.IncludedMemoryIDs) ||
		memory.ContextWindow.TokenBudget <= 0 ||
		!memory.ContextWindow.DeterministicOrder ||
		len(memory.ContextWindow.ExcludedMemoryIDs) != 0 {
		t.Fatalf("context window incomplete: %#v", memory.ContextWindow)
	}
	for _, boundary := range memory.AdapterBoundaries {
		if boundary.ID == "" || boundary.Adapter == "" || boundary.Capability == "" || boundary.FixtureKey != "generic:memory_store:offline" || boundary.DependencyImported || boundary.CredentialRequired || boundary.LiveNetwork || !boundary.CleanSkip {
			t.Fatalf("memory adapter boundary must be fixture-only: %#v", boundary)
		}
	}
	memoryByID := map[string]string{}
	for _, item := range memory.MemoryItems {
		if item.MemoryID == "" || item.Text == "" || item.SourceRef == "" || item.ChunkID == "" || item.Provenance.FixtureKey != memory.Provenance.FixtureKey || item.Provenance.SourceFixtureKey != memory.Provenance.LinkedChunkFixtureKey || item.Provenance.RetrieverFixtureKey != memory.Provenance.LinkedRetrieverFixtureKey {
			t.Fatalf("memory item provenance incomplete: %#v", item)
		}
		if chunkIDs != nil && !chunkIDs[item.ChunkID] {
			t.Fatalf("memory item %q references missing chunk %q", item.MemoryID, item.ChunkID)
		}
		memoryByID[item.MemoryID] = item.ChunkID
	}
	for i, result := range memory.RetrievalResults {
		if result.MemoryID == "" || result.Rank != i+1 || result.Score <= 0 || result.Reason == "" || result.ContextRef == "" {
			t.Fatalf("retrieval result incomplete: %#v", result)
		}
		if memoryByID[result.MemoryID] == "" {
			t.Fatalf("retrieval result references missing memory id %q", result.MemoryID)
		}
		if memory.ContextWindow.IncludedMemoryIDs[i] != result.MemoryID {
			t.Fatalf("context window order %d = %q, want retrieval result %q", i, memory.ContextWindow.IncludedMemoryIDs[i], result.MemoryID)
		}
	}
}

func genericMemoryStorePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_memory_store")
}
