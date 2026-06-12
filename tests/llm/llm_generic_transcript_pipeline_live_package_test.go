package leia_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericTranscriptPipelineLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericTranscriptPipelinePackageDir(t)
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
	if manifest.SchemaVersion != 1 || manifest.ID != "generic-transcript-pipeline" ||
		manifest.PackageName != "leia-generic-ai-transcript-pipeline" ||
		manifest.PackageBoundaryID != "generic-ai-transcript-pipeline" ||
		manifest.CapabilityID != "generic.ai.transcript.pipeline" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetworkDefault ||
		manifest.LiveModelDefault || manifest.DependsOnQRuntime || manifest.CredentialRequired {
		t.Fatalf("manifest must stay provider-free/generic/offline/credential-free: %#v", manifest)
	}
	for _, want := range []string{"generic.ai.transcript.pipeline", "generic.ai.transcript.source_envelope", "generic.ai.transcript.speaker.normalize", "generic.ai.transcript.segment.normalize", "generic.ai.transcript.event_time.correct", "generic.ai.transcript.chunk", "generic.ai.transcript.adapter.clean_skip"} {
		if !genericLivePackageContains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}

	var contract struct {
		ID                    string `json:"id"`
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		FieldContracts        map[string]struct {
			Schema   string   `json:"schema"`
			Fixture  string   `json:"fixture"`
			Required []string `json:"required_fields"`
		} `json:"field_contracts"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Contracts["contract"]), &contract)
	if contract.ID != "generic-transcript-pipeline-contract" ||
		!contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	for _, want := range []string{"source_envelope", "segment", "chunk", "clean_skip"} {
		field := contract.FieldContracts[want]
		if field.Schema == "" || field.Fixture == "" || len(field.Required) == 0 {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
		}
	}
}

func TestGenericTranscriptPipelineLivePackageFixtureShape(t *testing.T) {
	base := genericTranscriptPipelinePackageDir(t)
	index := loadGenericTranscriptPipelineFixtureIndex(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"))
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 4 {
		t.Fatalf("fixture index invalid: %#v", index)
	}
	seen := map[string]bool{}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || fixture.Path == "" || fixture.Schema == "" || !fixture.Metadata.ReplayReady ||
			!fixture.Metadata.ProviderFree || fixture.Metadata.LiveNetwork {
			t.Fatalf("fixture entry invalid: %#v", fixture)
		}
		if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(fixture.Path))); err != nil {
			t.Fatalf("fixture path %q: %v", fixture.Path, err)
		}
		if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(fixture.Schema))); err != nil {
			t.Fatalf("fixture schema %q: %v", fixture.Schema, err)
		}
		seen[fixture.FixtureKey] = true
	}
	for _, want := range []string{"transcript_source:session_alpha:offline:v1", "transcript_segments:session_alpha:offline:v1", "transcript_chunks:session_alpha:offline:v1", "transcript_clean_skip:offline:v1"} {
		if !seen[want] {
			t.Fatalf("fixture key %q missing from %#v", want, seen)
		}
	}
}

func TestGenericTranscriptPipelineLivePackageIsDomainNeutral(t *testing.T) {
	base := genericTranscriptPipelinePackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"finrobot", "earnings", "finance", "financial", "fiscal", "quarter", "ticker", "stock", "equity", "company", "sec.gov", "10-k", "10-q", "filing", "cik", "accession", "analyst", "investor", "cfo", "ceo", "guidance", "yfinance", "finnhub", "openbb"} {
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

func TestGenericTranscriptPipelineLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericTranscriptPipelinePackageDir(t)
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "transcript_source_envelope_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "source_id", "source_ref", "source_type", "event_id", "event_time", "language", "timezone", "raw_turns"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "transcript_segment_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "source_id", "event_id", "segment_id", "turn_index", "speaker_raw", "speaker_label", "speaker_role", "text", "provenance"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "transcript_chunk_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "source_id", "event_id", "chunk_id", "chunk_index", "segment_ids", "token_estimate", "text", "provenance"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "transcript_clean_skip_v1.schema.json"), []string{"provider_free", "live_network", "fixture_key", "skip_code", "dependency", "adapter", "reason", "recoverable"})
}

func TestGenericTranscriptPipelineLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericTranscriptPipelinePackageDir(t), "main.leia")
	want := "generic_transcript_pipeline_live_package capability=generic.ai.transcript.pipeline entrypoint=ai.transcript.pipeline sources=1 speakers=3 segments=4 event_time_policies=1 chunks=2 provenance=4 clean_skip=3 provider_free=true live_network=false imports=false model_calls=false"
	for _, result := range runFinRobotLivePackageSummarySmoke(t, path, "generic_transcript_pipeline_live_package_summary", "generic_transcript_pipeline_live_package", leia.LibString) {
		if result.Summary != want {
			t.Fatalf("summary = %#v, want %#v", result.Summary, want)
		}
	}
}

type genericTranscriptPipelineFixtureIndex struct {
	ProviderFree          bool `json:"provider_free"`
	LiveNetwork           bool `json:"live_network"`
	RealDependencyImports bool `json:"real_dependency_imports"`
	Fixtures              []struct {
		FixtureKey string `json:"fixture_key"`
		Path       string `json:"path"`
		Schema     string `json:"schema"`
		Metadata   struct {
			ReplayReady  bool `json:"replay_ready"`
			ProviderFree bool `json:"provider_free"`
			LiveNetwork  bool `json:"live_network"`
		} `json:"metadata"`
	} `json:"fixtures"`
}

func loadGenericTranscriptPipelineFixtureIndex(t *testing.T, path string) genericTranscriptPipelineFixtureIndex {
	t.Helper()
	var fixture genericTranscriptPipelineFixtureIndex
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func genericTranscriptPipelinePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_transcript_pipeline")
}
