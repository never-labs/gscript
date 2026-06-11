package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const genericModelIOEnvelopeRoot = "tests/llm/testdata/generic_model_io_envelope"

type genericModelIOEnvelopeManifest struct {
	SchemaVersion       int      `json:"schema_version"`
	ID                  string   `json:"id"`
	PackageBoundaryID   string   `json:"package_boundary_id"`
	CapabilityID        string   `json:"capability_id"`
	ProviderFree        bool     `json:"provider_free"`
	DomainSpecific      bool     `json:"domain_specific"`
	LiveNetwork         bool     `json:"live_network"`
	LiveModel           bool     `json:"live_model"`
	CredentialsRequired bool     `json:"credentials_required"`
	DependsOnQRuntime   bool     `json:"depends_on_q_runtime"`
	RealImports         bool     `json:"real_dependency_imports"`
	CapabilitySurfaces  []string `json:"capability_surfaces"`
	NoBuiltInGuarantee  struct {
		Required  bool   `json:"required"`
		Statement string `json:"statement"`
	} `json:"no_built_in_guarantee"`
	Contracts map[string]string `json:"contracts"`
	Fixtures  map[string]string `json:"fixtures"`
}

type genericModelIOEnvelopeContract struct {
	SchemaVersion       int    `json:"schema_version"`
	ID                  string `json:"id"`
	PackageBoundaryID   string `json:"package_boundary_id"`
	ProviderFree        bool   `json:"provider_free"`
	DomainSpecific      bool   `json:"domain_specific"`
	LiveNetwork         bool   `json:"live_network"`
	LiveModel           bool   `json:"live_model"`
	CredentialsRequired bool   `json:"credentials_required"`
	AdapterContract     struct {
		AdapterIdentity              string `json:"adapter_identity"`
		ConcreteProviderNamesAllowed bool   `json:"concrete_provider_names_allowed"`
		RequestResponseRequired      bool   `json:"request_response_envelope_required"`
		ProviderFreeReplayRequired   bool   `json:"provider_free_replay_required"`
	} `json:"adapter_contract"`
	RequestEnvelope struct {
		Name           string   `json:"name"`
		RequiredFields []string `json:"required_fields"`
		MetadataFields []string `json:"metadata_fields"`
	} `json:"request_envelope"`
	StreamChunkEnvelope struct {
		Name                          string   `json:"name"`
		RequiredFields                []string `json:"required_fields"`
		EventTypes                    []string `json:"event_types"`
		SequenceStable                bool     `json:"sequence_stable"`
		ChunkTextReconstructsResponse bool     `json:"chunk_text_reconstructs_response"`
	} `json:"stream_chunk_envelope"`
	ResponseEnvelope struct {
		Name                 string   `json:"name"`
		RequiredFields       []string `json:"required_fields"`
		UsageFields          []string `json:"usage_fields"`
		CostMetadataRequired bool     `json:"cost_metadata_required"`
	} `json:"response_envelope"`
	Redaction struct {
		Required            bool     `json:"required"`
		SecretValuesPresent bool     `json:"secret_values_present"`
		RedactedFields      []string `json:"redacted_fields"`
		Replacement         string   `json:"replacement"`
	} `json:"redaction"`
}

type genericModelIOReplayFixture struct {
	SchemaVersion       int    `json:"schema_version"`
	ID                  string `json:"id"`
	ProviderFree        bool   `json:"provider_free"`
	DomainSpecific      bool   `json:"domain_specific"`
	LiveNetwork         bool   `json:"live_network"`
	LiveModel           bool   `json:"live_model"`
	CredentialsRequired bool   `json:"credentials_required"`
	Replay              struct {
		ReplaySessionID    string `json:"replay_session_id"`
		Mode               string `json:"mode"`
		StrictOrderedMatch bool   `json:"strict_ordered_match"`
		AdapterIdentity    string `json:"adapter_identity"`
	} `json:"replay"`
	Records []struct {
		Request      map[string]any `json:"request"`
		StreamChunks []struct {
			ChunkID    string         `json:"chunk_id"`
			TurnID     string         `json:"turn_id"`
			Sequence   int            `json:"sequence"`
			EventType  string         `json:"event_type"`
			Delta      string         `json:"delta"`
			Status     string         `json:"status"`
			UsageDelta map[string]any `json:"usage_delta"`
		} `json:"stream_chunks"`
		Response map[string]any `json:"response"`
	} `json:"records"`
}

func TestGenericModelIOEnvelopeManifestContractFixtureClosedLoop(t *testing.T) {
	root := repoRoot(t)
	base := filepath.Join(root, filepath.FromSlash(genericModelIOEnvelopeRoot))

	var manifest genericModelIOEnvelopeManifest
	decodeGenericModelIOEnvelopeJSON(t, filepath.Join(base, "manifest.json"), &manifest)
	assertGenericModelIOEnvelopeBoundary(t, manifest)

	contractPath := filepath.Join(base, filepath.FromSlash(manifest.Contracts["model_io_envelope"]))
	fixturePath := filepath.Join(base, filepath.FromSlash(manifest.Fixtures["provider_free_replay"]))
	var contract genericModelIOEnvelopeContract
	var fixture genericModelIOReplayFixture
	decodeGenericModelIOEnvelopeJSON(t, contractPath, &contract)
	decodeGenericModelIOEnvelopeJSON(t, fixturePath, &fixture)

	assertGenericModelIOEnvelopeContract(t, manifest, contract)
	assertGenericModelIOEnvelopeFixture(t, contract, fixture)
	assertGenericModelIOEnvelopeNoSecrets(t, base)
}

func assertGenericModelIOEnvelopeBoundary(t *testing.T, manifest genericModelIOEnvelopeManifest) {
	t.Helper()
	if manifest.SchemaVersion != 1 ||
		manifest.ID != "generic-model-io-envelope" ||
		manifest.PackageBoundaryID != "generic-ai-turn-runner" ||
		manifest.CapabilityID != "generic.ai.turn.request" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetwork || manifest.LiveModel ||
		manifest.CredentialsRequired || manifest.DependsOnQRuntime || manifest.RealImports {
		t.Fatalf("manifest must stay provider-free/local/generic: %#v", manifest)
	}
	if !manifest.NoBuiltInGuarantee.Required || !strings.Contains(strings.ToLower(manifest.NoBuiltInGuarantee.Statement), "does not provide built-in") {
		t.Fatalf("manifest missing no built-in boundary: %#v", manifest.NoBuiltInGuarantee)
	}
	assertGenericModelIOStringSet(t, "capability surfaces", manifest.CapabilitySurfaces, []string{
		"messages",
		"response_format",
		"turn",
		"usage",
	})
	if manifest.Contracts["model_io_envelope"] == "" || manifest.Fixtures["provider_free_replay"] == "" {
		t.Fatalf("manifest must link contract and fixture: %#v", manifest)
	}
}

func assertGenericModelIOEnvelopeContract(t *testing.T, manifest genericModelIOEnvelopeManifest, contract genericModelIOEnvelopeContract) {
	t.Helper()
	if contract.SchemaVersion != 1 || contract.PackageBoundaryID != manifest.PackageBoundaryID {
		t.Fatalf("contract identity mismatch: %#v", contract)
	}
	if !contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork || contract.LiveModel || contract.CredentialsRequired {
		t.Fatalf("contract must stay provider-free/local/generic: %#v", contract)
	}
	if contract.AdapterContract.AdapterIdentity == "" ||
		contract.AdapterContract.ConcreteProviderNamesAllowed ||
		!contract.AdapterContract.RequestResponseRequired ||
		!contract.AdapterContract.ProviderFreeReplayRequired {
		t.Fatalf("adapter contract is not provider-neutral: %#v", contract.AdapterContract)
	}
	assertGenericModelIOStringSet(t, "request envelope required fields", contract.RequestEnvelope.RequiredFields, []string{
		"envelope_version",
		"messages",
		"metadata",
		"model",
		"redaction",
		"response_format",
		"tools",
		"turn_id",
	})
	assertGenericModelIOStringSet(t, "stream chunk required fields", contract.StreamChunkEnvelope.RequiredFields, []string{
		"chunk_id",
		"delta",
		"event_type",
		"sequence",
		"status",
		"turn_id",
		"usage_delta",
	})
	if !contract.StreamChunkEnvelope.SequenceStable || !contract.StreamChunkEnvelope.ChunkTextReconstructsResponse {
		t.Fatalf("stream chunk semantics incomplete: %#v", contract.StreamChunkEnvelope)
	}
	assertGenericModelIOStringSet(t, "response usage fields", contract.ResponseEnvelope.UsageFields, []string{
		"cost",
		"currency",
		"input_tokens",
		"latency_ms",
		"output_tokens",
		"total_tokens",
	})
	if !contract.ResponseEnvelope.CostMetadataRequired {
		t.Fatalf("response envelope must require cost metadata: %#v", contract.ResponseEnvelope)
	}
	if !contract.Redaction.Required || contract.Redaction.SecretValuesPresent || contract.Redaction.Replacement != "<redacted>" ||
		len(contract.Redaction.RedactedFields) < 3 {
		t.Fatalf("redaction contract incomplete: %#v", contract.Redaction)
	}
}

func assertGenericModelIOEnvelopeFixture(t *testing.T, contract genericModelIOEnvelopeContract, fixture genericModelIOReplayFixture) {
	t.Helper()
	if fixture.SchemaVersion != 1 || fixture.ID == "" {
		t.Fatalf("fixture identity incomplete: %#v", fixture)
	}
	if !fixture.ProviderFree || fixture.DomainSpecific || fixture.LiveNetwork || fixture.LiveModel || fixture.CredentialsRequired {
		t.Fatalf("fixture must stay provider-free/local/generic: %#v", fixture)
	}
	if fixture.Replay.Mode != "provider-free" ||
		!fixture.Replay.StrictOrderedMatch ||
		fixture.Replay.ReplaySessionID == "" ||
		fixture.Replay.AdapterIdentity != contract.AdapterContract.AdapterIdentity {
		t.Fatalf("replay metadata incomplete: %#v", fixture.Replay)
	}
	if len(fixture.Records) != 1 {
		t.Fatalf("records = %d, want one deterministic fixture record", len(fixture.Records))
	}

	record := fixture.Records[0]
	wantTurnID, _ := record.Request["turn_id"].(string)
	if wantTurnID == "" || record.Response["turn_id"] != wantTurnID {
		t.Fatalf("request/response turn correlation mismatch: request=%#v response=%#v", record.Request, record.Response)
	}
	if record.Request["envelope_version"] != contract.RequestEnvelope.Name ||
		record.Response["envelope_version"] != contract.ResponseEnvelope.Name {
		t.Fatalf("fixture envelope versions do not match contract: request=%#v response=%#v", record.Request, record.Response)
	}
	requestMetadata, _ := record.Request["metadata"].(map[string]any)
	responseMetadata, _ := record.Response["metadata"].(map[string]any)
	if requestMetadata["replay_session_id"] != fixture.Replay.ReplaySessionID ||
		responseMetadata["replay_session_id"] != fixture.Replay.ReplaySessionID ||
		requestMetadata["adapter_identity"] != fixture.Replay.AdapterIdentity ||
		responseMetadata["adapter_identity"] != fixture.Replay.AdapterIdentity {
		t.Fatalf("fixture metadata does not carry replay/adapter identity: request=%#v response=%#v", requestMetadata, responseMetadata)
	}

	var reconstructed string
	lastSequence := 0
	seenCompleted := false
	for _, chunk := range record.StreamChunks {
		if chunk.TurnID != wantTurnID || chunk.ChunkID == "" || chunk.Sequence <= lastSequence {
			t.Fatalf("chunk sequence/correlation invalid: %#v after %d", chunk, lastSequence)
		}
		lastSequence = chunk.Sequence
		switch chunk.EventType {
		case "response.delta":
			if chunk.Status != "streaming" || chunk.Delta == "" {
				t.Fatalf("delta chunk malformed: %#v", chunk)
			}
			reconstructed += chunk.Delta
		case "response.completed":
			if chunk.Status != "completed" || chunk.Delta != "" {
				t.Fatalf("completion chunk malformed: %#v", chunk)
			}
			seenCompleted = true
		default:
			t.Fatalf("unexpected chunk event type %q", chunk.EventType)
		}
	}
	if !seenCompleted || reconstructed != record.Response["text"] {
		t.Fatalf("stream chunks reconstruct %q completed=%v, want response text %q", reconstructed, seenCompleted, record.Response["text"])
	}

	usage, _ := record.Response["usage"].(map[string]any)
	if usage["currency"] != "USD" || usage["cost"] == nil || usage["input_tokens"] == nil ||
		usage["output_tokens"] == nil || usage["total_tokens"] == nil || usage["latency_ms"] == nil {
		t.Fatalf("usage/cost metadata incomplete: %#v", usage)
	}
	requestRedaction, _ := record.Request["redaction"].(map[string]any)
	responseRedaction, _ := record.Response["redaction"].(map[string]any)
	if requestRedaction["secret_values_present"] != false || responseRedaction["secret_values_present"] != false {
		t.Fatalf("fixture redaction must assert no secret values: request=%#v response=%#v", requestRedaction, responseRedaction)
	}
}

func assertGenericModelIOEnvelopeNoSecrets(t *testing.T, base string) {
	t.Helper()
	entries := []string{
		filepath.Join(base, "manifest.json"),
		filepath.Join(base, "contracts", "model_io_envelope_contract.json"),
		filepath.Join(base, "fixtures", "provider_free_replay_fixture.json"),
	}
	for _, path := range entries {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{
			"anthropic",
			"openai",
			"ollama",
			"gemini",
			"bedrock",
			"finrobot",
			"openbb",
			"fingpt",
			"sk-",
			"bearer ",
			"api_secret",
			"private_key",
		} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("%s contains provider/domain/secret marker %q", path, forbidden)
			}
		}
	}
}

func decodeGenericModelIOEnvelopeJSON(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func assertGenericModelIOStringSet(t *testing.T, label string, got, want []string) {
	t.Helper()
	got = append([]string(nil), got...)
	want = append([]string(nil), want...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("%s = %#v, want %#v", label, got, want)
	}
}
