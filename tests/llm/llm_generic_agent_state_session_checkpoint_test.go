package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const genericAgentStateCheckpointFixture = "examples/ai/finrobot_translation/ai_dialect_index/fixtures/generic_agent_state_session_checkpoint_fixture.json"
const genericAgentStateCheckpointContract = "examples/ai/finrobot_translation/ai_dialect_index/contracts/generic_agent_state_session_checkpoint_contract.json"

type genericAgentStateCheckpointDoc struct {
	SchemaVersion         int    `json:"schema_version"`
	ID                    string `json:"id"`
	PackageBoundaryID     string `json:"package_boundary_id"`
	ProviderFree          bool   `json:"provider_free"`
	DomainSpecific        bool   `json:"domain_specific"`
	LiveNetwork           bool   `json:"live_network"`
	LiveModel             bool   `json:"live_model"`
	CredentialsRequired   bool   `json:"credentials_required"`
	RealDependencyImports bool   `json:"real_dependency_imports"`
	DependsOnQRuntime     bool   `json:"depends_on_q_runtime"`
	Contract              string `json:"contract"`
	StateSnapshots        []struct {
		AgentRunID       string                 `json:"agent_run_id"`
		SessionID        string                 `json:"session_id"`
		StateVersion     int                    `json:"state_version"`
		ResumeToken      string                 `json:"resume_token"`
		Status           string                 `json:"status"`
		InputRefs        []genericAgentStateRef `json:"input_refs"`
		OutputRefs       []genericAgentStateRef `json:"output_refs"`
		TraceCorrelation struct {
			TraceID       string `json:"trace_id"`
			EventID       string `json:"event_id"`
			ParentEventID string `json:"parent_event_id"`
			AgentRunID    string `json:"agent_run_id"`
			SessionID     string `json:"session_id"`
			TurnID        string `json:"turn_id"`
			CheckpointKey string `json:"checkpoint_key"`
		} `json:"trace_correlation"`
		Checkpoint struct {
			CheckpointKey      string   `json:"checkpoint_key"`
			CacheKey           string   `json:"cache_key"`
			KeyAlgorithm       string   `json:"key_algorithm"`
			KeyMaterialFields  []string `json:"key_material_fields"`
			ExcludedFields     []string `json:"excluded_fields"`
			StableAcrossReplay bool     `json:"stable_across_replay"`
		} `json:"checkpoint"`
		Redaction struct {
			Enabled              bool     `json:"enabled"`
			SecretValuesPresent  bool     `json:"secret_values_present"`
			RawPromptStored      bool     `json:"raw_prompt_stored"`
			RawCompletionStored  bool     `json:"raw_completion_stored"`
			RedactedFields       []string `json:"redacted_fields"`
			Placeholder          string   `json:"placeholder"`
			CredentialReferences []struct {
				Field          string `json:"field"`
				Value          string `json:"value"`
				RawValueStored bool   `json:"raw_value_stored"`
			} `json:"credential_refs"`
		} `json:"redaction"`
	} `json:"state_snapshots"`
	ReplayAssertions map[string]any `json:"replay_assertions"`
}

type genericAgentStateRef struct {
	RefID          string `json:"ref_id"`
	Kind           string `json:"kind"`
	MediaType      string `json:"media_type"`
	Digest         string `json:"digest"`
	Summary        string `json:"summary"`
	RawValueStored bool   `json:"raw_value_stored"`
}

func TestGenericAgentStateSessionCheckpointFixtureContract(t *testing.T) {
	root := repoRoot(t)
	doc := loadGenericAgentStateCheckpointFixture(t, root)

	if doc.SchemaVersion != 1 ||
		doc.ID != "generic-agent-state-session-checkpoint-fixture" ||
		doc.PackageBoundaryID != "generic-ai-agent-state-store" ||
		doc.Contract != genericAgentStateCheckpointContract {
		t.Fatalf("unexpected fixture header: %#v", doc)
	}
	if !doc.ProviderFree || doc.DomainSpecific || doc.LiveNetwork || doc.LiveModel ||
		doc.CredentialsRequired || doc.RealDependencyImports || doc.DependsOnQRuntime {
		t.Fatalf("fixture must stay provider-free, generic, and local-only: %#v", doc)
	}
	assertJSONFile(t, filepath.Join(root, filepath.FromSlash(doc.Contract)))

	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(genericAgentStateCheckpointFixture)))
	if err != nil {
		t.Fatal(err)
	}
	assertGenericAgentStateNoSecretLeakage(t, string(data))

	if len(doc.StateSnapshots) != 2 {
		t.Fatalf("state snapshot count = %d, want 2", len(doc.StateSnapshots))
	}
	seenEvents := map[string]bool{}
	lastVersion := 0
	for _, snapshot := range doc.StateSnapshots {
		if snapshot.AgentRunID == "" || snapshot.SessionID == "" || snapshot.StateVersion <= lastVersion ||
			snapshot.ResumeToken == "" || snapshot.Status == "" {
			t.Fatalf("snapshot has incomplete resumable identity: %#v", snapshot)
		}
		lastVersion = snapshot.StateVersion
		if !strings.HasPrefix(snapshot.ResumeToken, "checkpoint:"+snapshot.Checkpoint.CheckpointKey) {
			t.Fatalf("resume token %q does not derive from checkpoint key %q", snapshot.ResumeToken, snapshot.Checkpoint.CheckpointKey)
		}
		assertGenericAgentStateRefs(t, "input", snapshot.InputRefs)
		assertGenericAgentStateRefs(t, "output", snapshot.OutputRefs)
		if snapshot.TraceCorrelation.TraceID == "" ||
			snapshot.TraceCorrelation.EventID == "" ||
			snapshot.TraceCorrelation.ParentEventID == "" ||
			snapshot.TraceCorrelation.AgentRunID != snapshot.AgentRunID ||
			snapshot.TraceCorrelation.SessionID != snapshot.SessionID ||
			snapshot.TraceCorrelation.TurnID == "" ||
			snapshot.TraceCorrelation.CheckpointKey != snapshot.Checkpoint.CheckpointKey {
			t.Fatalf("trace correlation does not match state/checkpoint identity: %#v", snapshot.TraceCorrelation)
		}
		if seenEvents[snapshot.TraceCorrelation.EventID] {
			t.Fatalf("duplicate trace event id %q", snapshot.TraceCorrelation.EventID)
		}
		seenEvents[snapshot.TraceCorrelation.EventID] = true
		assertGenericAgentStateCheckpointKeys(t, snapshot.Checkpoint.CheckpointKey, snapshot.Checkpoint.CacheKey, snapshot.Checkpoint.KeyAlgorithm)
		for _, field := range []string{"raw_input", "raw_output", "secret", "token", "authorization", "cookie"} {
			if !contains(snapshot.Checkpoint.ExcludedFields, field) {
				t.Fatalf("checkpoint excluded_fields missing %q: %#v", field, snapshot.Checkpoint.ExcludedFields)
			}
		}
		if !snapshot.Checkpoint.StableAcrossReplay {
			t.Fatalf("checkpoint must be stable across replay: %#v", snapshot.Checkpoint)
		}
		if !snapshot.Redaction.Enabled || snapshot.Redaction.SecretValuesPresent ||
			snapshot.Redaction.RawPromptStored || snapshot.Redaction.RawCompletionStored ||
			snapshot.Redaction.Placeholder != "<redacted>" {
			t.Fatalf("redaction contract is not no-secret safe: %#v", snapshot.Redaction)
		}
		for _, cred := range snapshot.Redaction.CredentialReferences {
			if cred.Value != "<redacted>" || cred.RawValueStored {
				t.Fatalf("credential reference leaked raw value: %#v", cred)
			}
		}
	}
}

func loadGenericAgentStateCheckpointFixture(t *testing.T, root string) genericAgentStateCheckpointDoc {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(genericAgentStateCheckpointFixture))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc genericAgentStateCheckpointDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return doc
}

func assertGenericAgentStateRefs(t *testing.T, label string, refs []genericAgentStateRef) {
	t.Helper()
	if len(refs) == 0 {
		t.Fatalf("%s refs are empty", label)
	}
	seen := map[string]bool{}
	for _, ref := range refs {
		if ref.RefID == "" || ref.Kind == "" || ref.MediaType == "" || ref.Summary == "" {
			t.Fatalf("%s ref is incomplete: %#v", label, ref)
		}
		if seen[ref.RefID] {
			t.Fatalf("duplicate %s ref id %q", label, ref.RefID)
		}
		seen[ref.RefID] = true
		assertGenericAgentStateSHA256(t, ref.Digest)
		if ref.RawValueStored {
			t.Fatalf("%s ref stores raw value: %#v", label, ref)
		}
	}
}

func assertGenericAgentStateCheckpointKeys(t *testing.T, checkpointKey, cacheKey, algorithm string) {
	t.Helper()
	if algorithm != "sha256" {
		t.Fatalf("key algorithm = %q, want sha256", algorithm)
	}
	assertGenericAgentStateSHA256(t, checkpointKey)
	assertGenericAgentStateSHA256(t, cacheKey)
	if checkpointKey == cacheKey {
		t.Fatalf("checkpoint_key and cache_key should be independently derived: %q", checkpointKey)
	}
}

func assertGenericAgentStateSHA256(t *testing.T, value string) {
	t.Helper()
	matched, err := regexp.MatchString(`^sha256:[0-9a-f]{64}$`, value)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Fatalf("value %q is not a sha256 digest", value)
	}
}

func assertGenericAgentStateNoSecretLeakage(t *testing.T, data string) {
	t.Helper()
	lower := strings.ToLower(data)
	for _, forbidden := range []string{
		`"provider":`,
		"openai",
		"anthropic",
		"autogen",
		"openbb",
		"fingpt",
		"finance",
		"trading",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("generic state fixture contains specialized marker %q", forbidden)
		}
	}
	secretPattern := regexp.MustCompile(`(?i)"(?:api_key|access_token|refresh_token|password|authorization|cookie|secret|token)"\s*:\s*"[^"<{\[][^"]{7,}"`)
	if secretPattern.MatchString(data) {
		t.Fatalf("generic state fixture appears to contain an unredacted secret-shaped value")
	}
}

func TestGenericAgentStateSessionCheckpointContractDeclaresRequiredFields(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, filepath.FromSlash(genericAgentStateCheckpointContract))
	var contract struct {
		ProviderFree          bool     `json:"provider_free"`
		DomainSpecific        bool     `json:"domain_specific"`
		LiveNetwork           bool     `json:"live_network"`
		LiveModel             bool     `json:"live_model"`
		CredentialsRequired   bool     `json:"credentials_required"`
		RealDependencyImports bool     `json:"real_dependency_imports"`
		DependsOnQRuntime     bool     `json:"depends_on_q_runtime"`
		Capabilities          []string `json:"capabilities"`
		StateContract         struct {
			RequiredFields        []string `json:"required_fields"`
			ResumeTokenSource     string   `json:"resume_token_source"`
			StateVersionMonotonic bool     `json:"state_version_monotonic"`
			RawInputsAllowed      bool     `json:"raw_inputs_allowed"`
			RawOutputsAllowed     bool     `json:"raw_outputs_allowed"`
		} `json:"state_contract"`
		CheckpointContract struct {
			KeyAlgorithm       string   `json:"key_algorithm"`
			CacheKeyAlgorithm  string   `json:"cache_key_algorithm"`
			KeyFields          []string `json:"key_fields"`
			ExcludeFields      []string `json:"exclude_fields"`
			StableAcrossReplay bool     `json:"stable_across_replay"`
		} `json:"checkpoint_contract"`
		TraceCorrelationContract struct {
			RequiredFields            []string `json:"required_fields"`
			CheckpointKeyMatchesState bool     `json:"checkpoint_key_matches_state"`
			EventIDUnique             bool     `json:"event_id_unique"`
		} `json:"trace_correlation_contract"`
		RedactionContract struct {
			Enabled             bool     `json:"enabled"`
			SecretValuesPresent bool     `json:"secret_values_present"`
			RedactedFields      []string `json:"redacted_fields"`
			Placeholder         string   `json:"placeholder"`
			RawPromptStored     bool     `json:"raw_prompt_stored"`
			RawCompletionStored bool     `json:"raw_completion_stored"`
		} `json:"redaction_contract"`
	}
	decodeGenericAgentStateJSONFile(t, path, &contract)

	if !contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork || contract.LiveModel ||
		contract.CredentialsRequired || contract.RealDependencyImports || contract.DependsOnQRuntime {
		t.Fatalf("contract must stay provider-free/generic/local-only: %#v", contract)
	}
	for _, capability := range []string{
		"generic.ai.agent_state.snapshot",
		"generic.ai.agent_state.session",
		"generic.ai.agent_state.checkpoint",
		"generic.ai.agent_state.input_ref",
		"generic.ai.agent_state.output_ref",
		"generic.ai.agent_state.trace_correlation",
		"generic.ai.agent_state.cache_key",
		"generic.ai.agent_state.redaction",
	} {
		if !contains(contract.Capabilities, capability) {
			t.Fatalf("contract capabilities missing %q: %#v", capability, contract.Capabilities)
		}
	}
	for _, field := range []string{"agent_run_id", "session_id", "state_version", "resume_token", "input_refs", "output_refs", "trace_correlation", "checkpoint", "redaction"} {
		if !contains(contract.StateContract.RequiredFields, field) {
			t.Fatalf("state contract required_fields missing %q: %#v", field, contract.StateContract.RequiredFields)
		}
	}
	if contract.StateContract.ResumeTokenSource != "checkpoint_key" ||
		!contract.StateContract.StateVersionMonotonic ||
		contract.StateContract.RawInputsAllowed ||
		contract.StateContract.RawOutputsAllowed {
		t.Fatalf("state contract resume/raw-payload settings drifted: %#v", contract.StateContract)
	}
	for _, field := range []string{"agent_run_id", "session_id", "state_version", "input_ref_ids", "output_ref_ids", "trace_id", "turn_id"} {
		if !contains(contract.CheckpointContract.KeyFields, field) {
			t.Fatalf("checkpoint key fields missing %q: %#v", field, contract.CheckpointContract.KeyFields)
		}
	}
	for _, field := range []string{"raw_input", "raw_output", "secret", "token", "authorization", "cookie"} {
		if !contains(contract.CheckpointContract.ExcludeFields, field) {
			t.Fatalf("checkpoint exclude fields missing %q: %#v", field, contract.CheckpointContract.ExcludeFields)
		}
	}
	if contract.CheckpointContract.KeyAlgorithm != "sha256" ||
		contract.CheckpointContract.CacheKeyAlgorithm != "sha256" ||
		!contract.CheckpointContract.StableAcrossReplay {
		t.Fatalf("checkpoint key contract drifted: %#v", contract.CheckpointContract)
	}
	if !contract.TraceCorrelationContract.CheckpointKeyMatchesState ||
		!contract.TraceCorrelationContract.EventIDUnique {
		t.Fatalf("trace correlation contract drifted: %#v", contract.TraceCorrelationContract)
	}
	for _, field := range []string{"trace_id", "event_id", "agent_run_id", "session_id", "turn_id", "parent_event_id", "checkpoint_key"} {
		if !contains(contract.TraceCorrelationContract.RequiredFields, field) {
			t.Fatalf("trace correlation required fields missing %q: %#v", field, contract.TraceCorrelationContract.RequiredFields)
		}
	}
	if !contract.RedactionContract.Enabled ||
		contract.RedactionContract.SecretValuesPresent ||
		contract.RedactionContract.RawPromptStored ||
		contract.RedactionContract.RawCompletionStored ||
		contract.RedactionContract.Placeholder != "<redacted>" {
		t.Fatalf("redaction contract drifted: %#v", contract.RedactionContract)
	}
}

func decodeGenericAgentStateJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
