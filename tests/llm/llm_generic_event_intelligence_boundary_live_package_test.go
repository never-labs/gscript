package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericEventIntelligenceBoundaryLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericEventIntelligenceBoundaryPackageDir(t)
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
		CredentialRequired bool              `json:"credential_required_default"`
		Capabilities       []string          `json:"capabilities"`
		Contracts          map[string]string `json:"contracts"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	if manifest.SchemaVersion != 1 || manifest.ID != "generic-event-intelligence-boundary" ||
		manifest.PackageName != "leia-generic-ai-event-intelligence-boundary" ||
		manifest.PackageBoundaryID != "generic-ai-event-intelligence-boundary" ||
		manifest.CapabilityID != "generic.ai.event_intelligence.boundary" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetworkDefault ||
		manifest.LiveModelDefault || manifest.DependsOnQRuntime || manifest.CredentialRequired {
		t.Fatalf("manifest must stay provider-free/generic/offline/credential-free: %#v", manifest)
	}
	for _, want := range []string{"generic.ai.event_intelligence.boundary", "generic.ai.event.source_snapshot", "generic.ai.event.extraction", "generic.ai.event.taxonomy", "generic.ai.event.freshness_policy", "generic.ai.event.dedupe_confidence", "generic.ai.event.relevance_score", "generic.ai.event.sentiment_impact", "generic.ai.event.prompt_contract", "generic.ai.event.adapter_clean_skip"} {
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
		LiveModelCalls        bool              `json:"live_model_calls"`
		RealDependencyImports bool              `json:"real_dependency_imports"`
		RequiresCredentials   bool              `json:"requires_credentials"`
		FieldContracts        map[string]string `json:"field_contracts"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Contracts["contract"]), &contract)
	if contract.SchemaVersion != 1 || contract.PackageBoundaryID != manifest.PackageBoundaryID ||
		contract.PackageName != "generic.ai.event_intelligence.boundary" || contract.Entrypoint != "ai.event_intelligence.boundary" ||
		!contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork ||
		contract.LiveModelCalls || contract.RealDependencyImports || contract.RequiresCredentials {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	for _, want := range []string{"source_snapshots", "event_extractions", "taxonomy", "freshness_policy", "dedupe_confidence", "relevance_scores", "sentiment_impact", "prompt_contracts", "adapter_clean_skip"} {
		if contract.FieldContracts[want] == "" {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
		}
	}
}

func TestGenericEventIntelligenceBoundaryLivePackageFixtureShape(t *testing.T) {
	base := genericEventIntelligenceBoundaryPackageDir(t)
	fixture := loadGenericEventIntelligenceBoundaryFixture(t, filepath.Join(base, "fixtures", "event_intelligence_boundary_fixture.json"))
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.LiveModelCalls {
		t.Fatalf("fixture must stay provider-free and offline: %#v", fixture)
	}
	if len(fixture.SourceSnapshots) != 2 || len(fixture.EventRecords) != 2 ||
		len(fixture.FreshnessPolicy) != 1 || len(fixture.DedupeConfidence) != 2 ||
		len(fixture.RelevanceScores) != 2 || len(fixture.SentimentImpact) != 2 ||
		len(fixture.PromptContracts) != 1 || len(fixture.AdapterBoundaries) != 2 {
		t.Fatalf("fixture counts drifted: sources=%d events=%d freshness=%d dedupe=%d relevance=%d sentiment=%d prompts=%d adapters=%d",
			len(fixture.SourceSnapshots), len(fixture.EventRecords), len(fixture.FreshnessPolicy), len(fixture.DedupeConfidence), len(fixture.RelevanceScores), len(fixture.SentimentImpact), len(fixture.PromptContracts), len(fixture.AdapterBoundaries))
	}
	sources := map[string]bool{}
	for _, source := range fixture.SourceSnapshots {
		if source.SourceID == "" || source.SourceType == "" || source.FixtureKey == "" ||
			source.RedactionPolicy == "" || source.LiveNetwork {
			t.Fatalf("source snapshot invalid: %#v", source)
		}
		sources[source.SourceID] = true
	}
	events := map[string]bool{}
	for _, event := range fixture.EventRecords {
		if event.EventID == "" || len(event.SourceRefs) == 0 || event.Category == "" ||
			event.Summary == "" || event.Confidence <= 0 || event.Freshness == "" ||
			event.DedupeGroup == "" {
			t.Fatalf("event record invalid: %#v", event)
		}
		for _, ref := range event.SourceRefs {
			if !sources[ref] {
				t.Fatalf("event source ref %q does not resolve", ref)
			}
		}
		events[event.EventID] = true
	}
	for _, score := range fixture.RelevanceScores {
		if !events[score.EventID] || score.Relevance <= 0 || score.Explanation == "" {
			t.Fatalf("relevance score invalid: %#v", score)
		}
	}
	for _, label := range fixture.SentimentImpact {
		if !events[label.EventID] || label.Sentiment == "" || label.Impact == "" || label.Explanation == "" {
			t.Fatalf("sentiment/impact invalid: %#v", label)
		}
	}
	for _, prompt := range fixture.PromptContracts {
		if prompt.PromptID == "" || len(prompt.AllowedFields) == 0 || len(prompt.ForbiddenFields) == 0 || !prompt.RedactionRequired {
			t.Fatalf("prompt contract invalid: %#v", prompt)
		}
	}
	for _, boundary := range fixture.AdapterBoundaries {
		if boundary.DependencyImported || boundary.LiveNetwork || !boundary.CleanSkip {
			t.Fatalf("adapter boundary must clean-skip: %#v", boundary)
		}
	}
}

func TestGenericEventIntelligenceBoundaryLivePackageIsDomainNeutral(t *testing.T) {
	base := genericEventIntelligenceBoundaryPackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"finrobot", "acme", "aapl", "ticker", "equity", "investment", "valuation_engine", "target_price", "dcf", "sec.gov", "10-k", "finance.", "polymarket", "reddit", "twitter", "x.com"} {
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

func TestGenericEventIntelligenceBoundaryLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericEventIntelligenceBoundaryPackageDir(t)
	schema := filepath.Join(base, "schemas", "generic_event_intelligence_boundary_v1.schema.json")
	assertDocumentPipelineSchemaRequired(t, schema, []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "live_model_calls", "source_snapshots", "event_taxonomy", "event_records", "freshness_policy", "dedupe_confidence", "relevance_scores", "sentiment_impact", "prompt_contracts", "adapter_boundaries"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "source_snapshots", "items"}, []string{"source_id", "source_type", "fixture_key", "captured_at", "redaction_policy", "live_network", "terms_metadata"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "event_records", "items"}, []string{"event_id", "source_refs", "category", "summary", "confidence", "freshness", "dedupe_group"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "event_source_snapshot_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "source_snapshots"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "event_extraction_v1.schema.json"), []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "event_records", "event_taxonomy", "dedupe_confidence"})
}

func TestGenericEventIntelligenceBoundaryLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericEventIntelligenceBoundaryPackageDir(t), "main.leia")
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
			got, err := vm.Get("generic_event_intelligence_boundary_live_package_summary")
			if err != nil {
				t.Fatalf("Get summary: %v", err)
			}
			want := "generic_event_intelligence_boundary_live_package capability=generic.ai.event_intelligence.boundary entrypoint=ai.event_intelligence.boundary sources=2 events=2 taxonomies=1 freshness=1 dedupe=2 relevance=2 sentiment_impact=2 prompts=1 clean_skip=2 provider_free=true live_network=false imports=false model_calls=false"
			if got != want {
				t.Fatalf("summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

type genericEventIntelligenceBoundaryFixture struct {
	ProviderFree          bool `json:"provider_free"`
	LiveNetwork           bool `json:"live_network"`
	RealDependencyImports bool `json:"real_dependency_imports"`
	LiveModelCalls        bool `json:"live_model_calls"`
	SourceSnapshots       []struct {
		SourceID        string `json:"source_id"`
		SourceType      string `json:"source_type"`
		FixtureKey      string `json:"fixture_key"`
		RedactionPolicy string `json:"redaction_policy"`
		LiveNetwork     bool   `json:"live_network"`
	} `json:"source_snapshots"`
	EventRecords []struct {
		EventID     string   `json:"event_id"`
		SourceRefs  []string `json:"source_refs"`
		Category    string   `json:"category"`
		Summary     string   `json:"summary"`
		Confidence  float64  `json:"confidence"`
		Freshness   string   `json:"freshness"`
		DedupeGroup string   `json:"dedupe_group"`
	} `json:"event_records"`
	FreshnessPolicy  []any `json:"freshness_policy"`
	DedupeConfidence []any `json:"dedupe_confidence"`
	RelevanceScores  []struct {
		EventID     string  `json:"event_id"`
		Relevance   float64 `json:"relevance"`
		Explanation string  `json:"explanation"`
	} `json:"relevance_scores"`
	SentimentImpact []struct {
		EventID     string `json:"event_id"`
		Sentiment   string `json:"sentiment"`
		Impact      string `json:"impact"`
		Explanation string `json:"explanation"`
	} `json:"sentiment_impact"`
	PromptContracts []struct {
		PromptID          string   `json:"prompt_id"`
		AllowedFields     []string `json:"allowed_fields"`
		ForbiddenFields   []string `json:"forbidden_fields"`
		RedactionRequired bool     `json:"redaction_required"`
	} `json:"prompt_contracts"`
	AdapterBoundaries []struct {
		ID                 string `json:"id"`
		Capability         string `json:"capability"`
		DependencyImported bool   `json:"dependency_imported"`
		LiveNetwork        bool   `json:"live_network"`
		CleanSkip          bool   `json:"clean_skip"`
	} `json:"adapter_boundaries"`
}

func loadGenericEventIntelligenceBoundaryFixture(t *testing.T, path string) genericEventIntelligenceBoundaryFixture {
	t.Helper()
	var fixture genericEventIntelligenceBoundaryFixture
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func genericEventIntelligenceBoundaryPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_event_intelligence_boundary")
}
