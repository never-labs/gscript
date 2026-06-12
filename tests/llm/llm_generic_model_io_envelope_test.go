package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const genericModelIOEnvelopeRoot = "examples/ai/finrobot_translation/live_packages/generic_model_io_envelope"

type genericModelIOEnvelopeManifest struct {
	SchemaVersion       int      `json:"schema_version"`
	ID                  string   `json:"id"`
	PackageBoundaryID   string   `json:"package_boundary_id"`
	CapabilityID        string   `json:"capability_id"`
	ProviderFree        bool     `json:"provider_free"`
	DomainSpecific      bool     `json:"domain_specific"`
	LiveNetwork         bool     `json:"live_network"`
	LiveNetworkDefault  bool     `json:"live_network_default"`
	LiveModel           bool     `json:"live_model"`
	LiveModelDefault    bool     `json:"live_model_default"`
	CredentialsRequired bool     `json:"credentials_required"`
	DependsOnQRuntime   bool     `json:"depends_on_q_runtime"`
	RealImports         bool     `json:"real_dependency_imports"`
	RealImportsDefault  bool     `json:"real_dependency_import_default"`
	Capabilities        []string `json:"capabilities"`
	CapabilitySurfaces  []string `json:"capability_surfaces"`
	NoBuiltInGuarantee  struct {
		Required  bool   `json:"required"`
		Statement string `json:"statement"`
	} `json:"no_built_in_guarantee"`
	Contracts map[string]string `json:"contracts"`
	Schemas   map[string]string `json:"schemas"`
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
	RegistryTurnProjection struct {
		SourcePackages              []string `json:"source_packages"`
		Inputs                      []string `json:"inputs"`
		Outputs                     []string `json:"outputs"`
		RequiredFields              []string `json:"required_fields"`
		SourceIDsAreNotAssumedEqual bool     `json:"source_ids_are_not_assumed_equal"`
		RawPromptStored             bool     `json:"raw_prompt_stored"`
		SecretValuesAllowed         bool     `json:"secret_values_allowed"`
		DeterministicReplayRequired bool     `json:"deterministic_replay_required"`
	} `json:"registry_turn_projection"`
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
	decodeGenericModelIOEnvelopeJSON(t, filepath.Join(base, "package.manifest.json"), &manifest)
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

func TestGenericModelIOEnvelopeSchemaRequiredFields(t *testing.T) {
	root := repoRoot(t)
	base := filepath.Join(root, filepath.FromSlash(genericModelIOEnvelopeRoot))

	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "model_request_envelope.schema.json"), []string{"envelope_version", "turn_id", "model", "messages", "tools", "response_format", "metadata", "redaction"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "model_request_envelope.schema.json"), []string{"properties", "metadata"}, []string{"trace_id", "replay_session_id", "adapter_identity", "provider_free"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "model_request_envelope.schema.json"), []string{"properties", "redaction"}, []string{"applied", "secret_values_present", "redacted_fields"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "model_stream_chunk.schema.json"), []string{"chunk_id", "turn_id", "sequence", "event_type", "delta", "status", "usage_delta"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "model_stream_chunk.schema.json"), []string{"properties", "usage_delta"}, []string{"output_tokens"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "model_response_envelope.schema.json"), []string{"envelope_version", "turn_id", "status", "text", "tool_calls", "finish_reason", "usage", "metadata", "redaction"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "model_response_envelope.schema.json"), []string{"properties", "usage"}, []string{"input_tokens", "output_tokens", "total_tokens", "cost", "currency", "latency_ms"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "model_response_envelope.schema.json"), []string{"properties", "metadata"}, []string{"trace_id", "replay_session_id", "adapter_identity", "provider_free"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "model_response_envelope.schema.json"), []string{"properties", "redaction"}, []string{"applied", "secret_values_present", "redacted_fields"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "provider_free_replay_fixture.schema.json"), []string{"schema_version", "id", "provider_free", "domain_specific", "live_network", "live_model", "credentials_required", "replay", "records"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "provider_free_replay_fixture.schema.json"), []string{"properties", "replay"}, []string{"replay_session_id", "mode", "strict_ordered_match", "adapter_identity"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "provider_free_replay_fixture.schema.json"), []string{"properties", "records", "items"}, []string{"request", "stream_chunks", "response"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "registry_turn_projection_v1.schema.json"), []string{"schema_version", "id", "projection_kind", "package_boundary_id", "provider_free", "domain_specific", "live_network", "live_model", "credentials_required", "real_dependency_imports", "source_fixture_refs", "identity_policy", "alias_resolution_mappings", "request_projection", "redaction_projection", "replay_projection", "projection_assertions"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "registry_turn_projection_v1.schema.json"), []string{"properties", "source_fixture_refs"}, []string{"model_alias_registry", "replay_execution_descriptor", "redaction_policy", "model_io_replay", "turn_request"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "registry_turn_projection_v1.schema.json"), []string{"properties", "identity_policy"}, []string{"model_aliases_are_resolved_before_projection", "model_io_turn_id_and_turn_request_id_are_not_assumed_equal", "model_names_are_canonicalized_per_target_envelope", "request_hash_source", "normalization_policy"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "registry_turn_projection_v1.schema.json"), []string{"properties", "alias_resolution_mappings", "items"}, []string{"requested_alias", "resolution_path", "resolved_descriptor_ref", "model_io_model", "turn_request_model"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "registry_turn_projection_v1.schema.json"), []string{"properties", "request_projection"}, []string{"requested_alias", "resolved_descriptor_ref", "descriptor_fixture_key", "descriptor_mode", "model_io_turn_id", "turn_request_id", "turn_correlation_id", "trace_id", "replay_session_id", "adapter_identity", "response_format_projection", "tool_projection", "provider_free", "live_network", "live_model"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "registry_turn_projection_v1.schema.json"), []string{"properties", "request_projection", "properties", "response_format_projection"}, []string{"model_io_type", "turn_runner_type", "schema_projection_required"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "registry_turn_projection_v1.schema.json"), []string{"properties", "request_projection", "properties", "tool_projection"}, []string{"descriptor_tool_calling", "model_io_tool_count", "turn_runner_tool_count", "tool_declarations_projected_by_turn_runner"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "registry_turn_projection_v1.schema.json"), []string{"properties", "redaction_projection"}, []string{"redaction_policy_ref", "model_registry_redact_fields", "model_io_redacted_fields", "turn_runner_secret_env_patterns", "replacement", "secret_values_present", "raw_prompt_stored", "raw_completion_stored"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "registry_turn_projection_v1.schema.json"), []string{"properties", "replay_projection"}, []string{"model_registry_replay_safe", "model_io_mode", "model_io_strict_ordered_match", "turn_runner_match_key", "turn_runner_request_hash", "request_hash_preserved_to_execute_response", "provider_free", "live_network", "credentials_required"})
}

func assertGenericModelIOEnvelopeBoundary(t *testing.T, manifest genericModelIOEnvelopeManifest) {
	t.Helper()
	if manifest.SchemaVersion != 1 ||
		manifest.ID != "generic-model-io-envelope" ||
		manifest.PackageBoundaryID != "generic-ai-model-io-envelope" ||
		manifest.CapabilityID != "generic.ai.model.io.envelope" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetwork || manifest.LiveNetworkDefault || manifest.LiveModel || manifest.LiveModelDefault ||
		manifest.CredentialsRequired || manifest.DependsOnQRuntime || manifest.RealImports || manifest.RealImportsDefault {
		t.Fatalf("manifest must stay provider-free/local/generic: %#v", manifest)
	}
	statement := strings.ToLower(manifest.NoBuiltInGuarantee.Statement)
	if !manifest.NoBuiltInGuarantee.Required || !strings.Contains(statement, "does not provide") || !strings.Contains(statement, "built-in") {
		t.Fatalf("manifest missing no built-in boundary: %#v", manifest.NoBuiltInGuarantee)
	}
	assertGenericModelIOStringSet(t, "capability surfaces", manifest.CapabilitySurfaces, []string{
		"model_io",
		"provider_free_replay",
		"redaction",
		"registry_turn_projection",
		"request_envelope",
		"response_envelope",
		"stream_chunk",
		"usage",
	})
	if !genericLivePackageContains(manifest.Capabilities, "generic.ai.model.io.registry_turn_projection") {
		t.Fatalf("manifest capabilities missing registry turn projection: %#v", manifest.Capabilities)
	}
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
	if len(contract.RegistryTurnProjection.SourcePackages) != 3 ||
		!contract.RegistryTurnProjection.SourceIDsAreNotAssumedEqual ||
		contract.RegistryTurnProjection.RawPromptStored ||
		contract.RegistryTurnProjection.SecretValuesAllowed ||
		!contract.RegistryTurnProjection.DeterministicReplayRequired {
		t.Fatalf("registry turn projection contract incomplete: %#v", contract.RegistryTurnProjection)
	}
	for _, want := range []string{"projection_id", "requested_alias", "resolved_descriptor_ref", "model_request_turn_id", "turn_request_id", "request_hash", "redaction_policy_ref", "provider_free"} {
		if !genericLivePackageContains(contract.RegistryTurnProjection.RequiredFields, want) {
			t.Fatalf("registry turn projection required_fields missing %q: %#v", want, contract.RegistryTurnProjection.RequiredFields)
		}
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
		filepath.Join(base, "package.manifest.json"),
		filepath.Join(base, "contracts", "model_io_envelope_contract.json"),
		filepath.Join(base, "fixtures", "provider_free_replay_fixture.json"),
		filepath.Join(base, "fixtures", "registry_turn_projection_fixture.json"),
		filepath.Join(base, "schemas", "registry_turn_projection_v1.schema.json"),
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

func TestGenericModelIOEnvelopeRegistryTurnProjection(t *testing.T) {
	root := repoRoot(t)
	base := filepath.Join(root, filepath.FromSlash(genericModelIOEnvelopeRoot))
	var manifest genericModelIOEnvelopeManifest
	decodeGenericModelIOEnvelopeJSON(t, filepath.Join(base, "package.manifest.json"), &manifest)

	var index struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Fixtures              []struct {
			FixtureKey string         `json:"fixture_key"`
			Capability string         `json:"capability"`
			Path       string         `json:"path"`
			Schema     string         `json:"schema"`
			SchemaRef  string         `json:"schema_ref"`
			Metadata   map[string]any `json:"metadata"`
		} `json:"fixtures"`
	}
	decodeGenericModelIOEnvelopeJSON(t, filepath.Join(base, filepath.FromSlash(manifest.Fixtures["index"])), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 2 {
		t.Fatalf("fixture index header/count mismatch: %#v", index)
	}
	replayEntry := index.Fixtures[0]
	if replayEntry.FixtureKey != "generic_model_io:provider_free_replay:v1" ||
		replayEntry.Capability != "generic.ai.model.io.provider_free_replay" ||
		replayEntry.Path != manifest.Fixtures["provider_free_replay"] ||
		replayEntry.Schema != manifest.Schemas["provider_free_replay_fixture"] ||
		replayEntry.SchemaRef != manifest.Schemas["provider_free_replay_fixture"] ||
		replayEntry.Metadata["replay_ready"] != true ||
		replayEntry.Metadata["adapter_identity"] != "generic-model-io-envelope-adapter" {
		t.Fatalf("provider-free replay fixture index entry mismatch: %#v", replayEntry)
	}
	assertGenericModelIOEnvelopeJSONPath(t, base, replayEntry.Path)
	assertGenericModelIOEnvelopeJSONPath(t, base, replayEntry.Schema)
	assertGenericModelIOEnvelopeJSONPath(t, base, replayEntry.SchemaRef)
	projectionEntry := index.Fixtures[1]
	if projectionEntry.FixtureKey != "generic_model_io:registry_turn_projection:v1" ||
		projectionEntry.Capability != "generic.ai.model.io.registry_turn_projection" ||
		projectionEntry.Path != manifest.Fixtures["registry_turn_projection"] ||
		projectionEntry.Schema != manifest.Schemas["registry_turn_projection"] ||
		projectionEntry.SchemaRef != manifest.Schemas["registry_turn_projection"] ||
		projectionEntry.Metadata["alias_mappings"] != float64(2) ||
		projectionEntry.Metadata["request_mappings"] != float64(1) ||
		projectionEntry.Metadata["redaction_mappings"] != float64(2) ||
		projectionEntry.Metadata["replay_ready"] != true {
		t.Fatalf("projection fixture index entry mismatch: %#v", projectionEntry)
	}
	assertGenericModelIOEnvelopeJSONPath(t, base, projectionEntry.Path)
	assertGenericModelIOEnvelopeJSONPath(t, base, projectionEntry.Schema)
	assertGenericModelIOEnvelopeJSONPath(t, base, projectionEntry.SchemaRef)

	var projection struct {
		SchemaVersion         int               `json:"schema_version"`
		ID                    string            `json:"id"`
		ProjectionKind        string            `json:"projection_kind"`
		PackageBoundaryID     string            `json:"package_boundary_id"`
		ProviderFree          bool              `json:"provider_free"`
		DomainSpecific        bool              `json:"domain_specific"`
		LiveNetwork           bool              `json:"live_network"`
		LiveModel             bool              `json:"live_model"`
		CredentialsRequired   bool              `json:"credentials_required"`
		RealDependencyImports bool              `json:"real_dependency_imports"`
		SourceFixtureRefs     map[string]string `json:"source_fixture_refs"`
		IdentityPolicy        struct {
			ModelAliasesAreResolvedBeforeProjection         bool   `json:"model_aliases_are_resolved_before_projection"`
			ModelIOTurnIDAndTurnRequestIDAreNotAssumedEqual bool   `json:"model_io_turn_id_and_turn_request_id_are_not_assumed_equal"`
			ModelNamesAreCanonicalizedPerTargetEnvelope     bool   `json:"model_names_are_canonicalized_per_target_envelope"`
			RequestHashSource                               string `json:"request_hash_source"`
		} `json:"identity_policy"`
		AliasResolutionMappings []struct {
			RequestedAlias        string   `json:"requested_alias"`
			ResolutionPath        []string `json:"resolution_path"`
			ResolvedDescriptorRef string   `json:"resolved_descriptor_ref"`
			ModelIOModel          string   `json:"model_io_model"`
			TurnRequestModel      string   `json:"turn_request_model"`
		} `json:"alias_resolution_mappings"`
		RequestProjection struct {
			RequestedAlias        string `json:"requested_alias"`
			ResolvedDescriptorRef string `json:"resolved_descriptor_ref"`
			DescriptorFixtureKey  string `json:"descriptor_fixture_key"`
			DescriptorMode        string `json:"descriptor_mode"`
			ModelIOTurnID         string `json:"model_io_turn_id"`
			TurnRequestID         string `json:"turn_request_id"`
			TurnCorrelationID     string `json:"turn_correlation_id"`
			TraceID               string `json:"trace_id"`
			ReplaySessionID       string `json:"replay_session_id"`
			AdapterIdentity       string `json:"adapter_identity"`
			ProviderFree          bool   `json:"provider_free"`
			LiveNetwork           bool   `json:"live_network"`
			LiveModel             bool   `json:"live_model"`
		} `json:"request_projection"`
		RedactionProjection struct {
			RedactionPolicyRef          string   `json:"redaction_policy_ref"`
			ModelRegistryRedactFields   []string `json:"model_registry_redact_fields"`
			ModelIORedactedFields       []string `json:"model_io_redacted_fields"`
			TurnRunnerSecretEnvPatterns []string `json:"turn_runner_secret_env_patterns"`
			Replacement                 string   `json:"replacement"`
			SecretValuesPresent         bool     `json:"secret_values_present"`
			RawPromptStored             bool     `json:"raw_prompt_stored"`
			RawCompletionStored         bool     `json:"raw_completion_stored"`
		} `json:"redaction_projection"`
		ReplayProjection struct {
			ModelRegistryReplaySafe               bool   `json:"model_registry_replay_safe"`
			ModelIOMode                           string `json:"model_io_mode"`
			ModelIOStrictOrderedMatch             bool   `json:"model_io_strict_ordered_match"`
			TurnRunnerMatchKey                    string `json:"turn_runner_match_key"`
			TurnRunnerRequestHash                 string `json:"turn_runner_request_hash"`
			RequestHashPreservedToExecuteResponse bool   `json:"request_hash_preserved_to_execute_response"`
			ProviderFree                          bool   `json:"provider_free"`
			LiveNetwork                           bool   `json:"live_network"`
			CredentialsRequired                   bool   `json:"credentials_required"`
		} `json:"replay_projection"`
		ProjectionAssertions map[string]bool `json:"projection_assertions"`
	}
	decodeGenericModelIOEnvelopeJSON(t, filepath.Join(base, "fixtures", "registry_turn_projection_fixture.json"), &projection)
	if projection.SchemaVersion != 1 ||
		projection.ID != "generic-model-io-registry-turn-projection-fixture" ||
		projection.ProjectionKind != "model_registry_to_model_io_turn_projection" ||
		projection.PackageBoundaryID != manifest.PackageBoundaryID {
		t.Fatalf("unexpected projection identity: %#v", projection)
	}
	if !projection.ProviderFree || projection.DomainSpecific || projection.LiveNetwork ||
		projection.LiveModel || projection.CredentialsRequired || projection.RealDependencyImports {
		t.Fatalf("projection must stay provider-free/local: %#v", projection)
	}
	for _, key := range []string{"model_alias_registry", "replay_execution_descriptor", "redaction_policy", "model_io_replay", "turn_request"} {
		if projection.SourceFixtureRefs[key] == "" {
			t.Fatalf("projection source fixture ref %q missing: %#v", key, projection.SourceFixtureRefs)
		}
	}
	if !projection.IdentityPolicy.ModelAliasesAreResolvedBeforeProjection ||
		!projection.IdentityPolicy.ModelIOTurnIDAndTurnRequestIDAreNotAssumedEqual ||
		!projection.IdentityPolicy.ModelNamesAreCanonicalizedPerTargetEnvelope ||
		projection.IdentityPolicy.RequestHashSource != "generic_turn_runner.replay.request_hash" {
		t.Fatalf("identity policy incomplete: %#v", projection.IdentityPolicy)
	}
	if len(projection.AliasResolutionMappings) != 2 {
		t.Fatalf("alias mappings = %d, want 2", len(projection.AliasResolutionMappings))
	}
	for _, mapping := range projection.AliasResolutionMappings {
		if mapping.RequestedAlias == "" || len(mapping.ResolutionPath) == 0 ||
			!strings.HasPrefix(mapping.ResolvedDescriptorRef, "model_registry:descriptor:") ||
			mapping.ModelIOModel == "" || mapping.TurnRequestModel == "" {
			t.Fatalf("alias projection mapping incomplete: %#v", mapping)
		}
	}
	if projection.RequestProjection.RequestedAlias != "default" ||
		projection.RequestProjection.ResolvedDescriptorRef != "model_registry:descriptor:fixture_analyst:v1" ||
		projection.RequestProjection.DescriptorMode != "deterministic_fixture_replay" ||
		projection.RequestProjection.ModelIOTurnID != "turn-generic-model-io-001" ||
		projection.RequestProjection.TurnRequestID != "turn-request-acme-summary-001" ||
		projection.RequestProjection.TurnCorrelationID != "corr-generic-turn-acme-summary-001" ||
		projection.RequestProjection.AdapterIdentity != "generic-model-io-envelope-adapter" ||
		!projection.RequestProjection.ProviderFree ||
		projection.RequestProjection.LiveNetwork ||
		projection.RequestProjection.LiveModel {
		t.Fatalf("request projection mismatch: %#v", projection.RequestProjection)
	}
	if projection.RedactionProjection.RedactionPolicyRef != "generic-model-registry-redaction-v1" ||
		projection.RedactionProjection.Replacement != "<redacted>" ||
		projection.RedactionProjection.SecretValuesPresent ||
		projection.RedactionProjection.RawPromptStored ||
		projection.RedactionProjection.RawCompletionStored ||
		len(projection.RedactionProjection.ModelRegistryRedactFields) < len(projection.RedactionProjection.ModelIORedactedFields) {
		t.Fatalf("redaction projection mismatch: %#v", projection.RedactionProjection)
	}
	if !projection.ReplayProjection.ModelRegistryReplaySafe ||
		projection.ReplayProjection.ModelIOMode != "provider-free" ||
		!projection.ReplayProjection.ModelIOStrictOrderedMatch ||
		projection.ReplayProjection.TurnRunnerMatchKey != "deterministic_request_hash" ||
		projection.ReplayProjection.TurnRunnerRequestHash != "sha256:40c43cc2c05232c0dc659cf6e266d046905b01b383cccfd952d9d9abcabd05ba" ||
		!projection.ReplayProjection.RequestHashPreservedToExecuteResponse ||
		!projection.ReplayProjection.ProviderFree ||
		projection.ReplayProjection.LiveNetwork ||
		projection.ReplayProjection.CredentialsRequired {
		t.Fatalf("replay projection mismatch: %#v", projection.ReplayProjection)
	}
	for _, want := range []string{"alias_resolution_is_explicit", "descriptor_ref_projects_to_model_io_request", "model_io_request_projects_to_turn_request", "redaction_policy_is_preserved", "request_hash_is_turn_runner_owned", "provider_free_chain", "live_network_absent", "real_dependency_imports_absent"} {
		if !projection.ProjectionAssertions[want] {
			t.Fatalf("projection assertion missing %q: %#v", want, projection.ProjectionAssertions)
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

func assertGenericModelIOEnvelopeJSONPath(t *testing.T, base, rel string) {
	t.Helper()
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(filepath.ToSlash(rel), "../") {
		t.Fatalf("invalid model IO package-relative path %q", rel)
	}
	var decoded any
	decodeGenericModelIOEnvelopeJSON(t, filepath.Join(base, filepath.FromSlash(rel)), &decoded)
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
