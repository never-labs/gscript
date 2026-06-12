package leia_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericAgentStateStoreLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericAgentStateStorePackageDir(t)

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
		manifest.ID != "generic-agent-state-store" ||
		manifest.PackageName != "leia-generic-ai-agent-state-store" ||
		manifest.PackageBoundaryID != "generic-ai-agent-state-store" ||
		manifest.CapabilityID != "generic.ai.agent_state.store" {
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
		"generic.ai.agent_state.store",
		"generic.ai.agent_state.snapshot",
		"generic.ai.agent_state.session",
		"generic.ai.agent_state.checkpoint",
		"generic.ai.agent_state.input_ref",
		"generic.ai.agent_state.output_ref",
		"generic.ai.agent_state.trace_correlation",
		"generic.ai.agent_state.trace_replay_projection",
		"generic.ai.agent_state.memory_context_projection",
		"generic.ai.agent_state.cache_key",
		"generic.ai.agent_state.redaction",
		"generic.ai.agent_state.clean_skip",
	} {
		if !genericLivePackageContains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}

	var contract struct {
		SchemaVersion         int      `json:"schema_version"`
		PackageBoundaryID     string   `json:"package_boundary_id"`
		ProviderFree          bool     `json:"provider_free"`
		DomainSpecific        bool     `json:"domain_specific"`
		LiveNetwork           bool     `json:"live_network"`
		LiveModel             bool     `json:"live_model"`
		Credentials           bool     `json:"credentials_required"`
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
		AdapterContract struct {
			DurableProviderNamesAllowed  bool `json:"durable_provider_names_allowed"`
			LiveDependencyImportsAllowed bool `json:"live_dependency_imports_allowed"`
			CredentialValuesAllowed      bool `json:"credential_values_allowed"`
			CleanSkipRequired            bool `json:"clean_skip_required"`
			ProviderFreeReplayRequired   bool `json:"provider_free_replay_required"`
		} `json:"adapter_contract"`
		TraceReplayProjectionContract struct {
			Source                     string   `json:"source"`
			TargetEventTypes           []string `json:"target_event_types"`
			RequiredCorrelationFields  []string `json:"required_correlation_fields"`
			ReplayMatchFields          []string `json:"replay_match_fields"`
			RawPayloadsAllowed         bool     `json:"raw_payloads_allowed"`
			SecretValuesAllowed        bool     `json:"secret_values_allowed"`
			DeterministicOrderRequired bool     `json:"deterministic_order_required"`
		} `json:"trace_replay_projection_contract"`
		MemoryContextProjectionContract struct {
			SourcePackage              string   `json:"source_package"`
			SourceFixture              string   `json:"source_fixture"`
			Target                     string   `json:"target"`
			TargetCapability           string   `json:"target_capability"`
			RequiredSourceFields       []string `json:"required_source_fields"`
			RequiredTargetFields       []string `json:"required_target_fields"`
			RawMemoryTextAllowed       bool     `json:"raw_memory_text_allowed"`
			SecretValuesAllowed        bool     `json:"secret_values_allowed"`
			DeterministicOrderRequired bool     `json:"deterministic_order_required"`
		} `json:"memory_context_projection_contract"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Contracts["contract"]), &contract)
	if contract.SchemaVersion != 1 || contract.PackageBoundaryID != manifest.PackageBoundaryID ||
		!contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork || contract.LiveModel ||
		contract.Credentials || contract.RealDependencyImports || contract.DependsOnQRuntime {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	if contract.StateContract.ResumeTokenSource != "checkpoint_key" ||
		!contract.StateContract.StateVersionMonotonic ||
		contract.StateContract.RawInputsAllowed ||
		contract.StateContract.RawOutputsAllowed {
		t.Fatalf("state contract drifted: %#v", contract.StateContract)
	}
	for _, want := range []string{"agent_run_id", "session_id", "state_version", "resume_token", "input_refs", "output_refs", "trace_correlation", "checkpoint", "redaction"} {
		if !genericLivePackageContains(contract.StateContract.RequiredFields, want) {
			t.Fatalf("state contract required_fields missing %q: %#v", want, contract.StateContract.RequiredFields)
		}
	}
	if contract.CheckpointContract.KeyAlgorithm != "sha256" ||
		contract.CheckpointContract.CacheKeyAlgorithm != "sha256" ||
		!contract.CheckpointContract.StableAcrossReplay {
		t.Fatalf("checkpoint contract drifted: %#v", contract.CheckpointContract)
	}
	if contract.AdapterContract.DurableProviderNamesAllowed ||
		contract.AdapterContract.LiveDependencyImportsAllowed ||
		contract.AdapterContract.CredentialValuesAllowed ||
		!contract.AdapterContract.CleanSkipRequired ||
		!contract.AdapterContract.ProviderFreeReplayRequired {
		t.Fatalf("adapter contract is not provider-free: %#v", contract.AdapterContract)
	}
	if contract.TraceReplayProjectionContract.Source != "state_snapshots" ||
		contract.TraceReplayProjectionContract.RawPayloadsAllowed ||
		contract.TraceReplayProjectionContract.SecretValuesAllowed ||
		!contract.TraceReplayProjectionContract.DeterministicOrderRequired {
		t.Fatalf("trace replay projection contract drifted: %#v", contract.TraceReplayProjectionContract)
	}
	for _, want := range []string{"agent_state_checkpoint", "agent_state_resume"} {
		if !genericLivePackageContains(contract.TraceReplayProjectionContract.TargetEventTypes, want) {
			t.Fatalf("trace replay target_event_types missing %q: %#v", want, contract.TraceReplayProjectionContract.TargetEventTypes)
		}
	}
	for _, want := range []string{"agent_run_id", "session_id", "state_version", "checkpoint_key", "cache_key"} {
		if !genericLivePackageContains(contract.TraceReplayProjectionContract.ReplayMatchFields, want) {
			t.Fatalf("trace replay match fields missing %q: %#v", want, contract.TraceReplayProjectionContract.ReplayMatchFields)
		}
	}
	if contract.MemoryContextProjectionContract.SourcePackage != "generic_memory_store" ||
		contract.MemoryContextProjectionContract.Target != "state_snapshots" ||
		contract.MemoryContextProjectionContract.TargetCapability != "generic.ai.agent_state.memory_context_projection" ||
		contract.MemoryContextProjectionContract.RawMemoryTextAllowed ||
		contract.MemoryContextProjectionContract.SecretValuesAllowed ||
		!contract.MemoryContextProjectionContract.DeterministicOrderRequired {
		t.Fatalf("memory context projection contract drifted: %#v", contract.MemoryContextProjectionContract)
	}
	for _, want := range []string{"retrieval_results.memory_id", "retrieval_results.rank", "retrieval_results.context_ref", "context_window.included_memory_ids", "provenance.fixture_key"} {
		if !genericLivePackageContains(contract.MemoryContextProjectionContract.RequiredSourceFields, want) {
			t.Fatalf("memory context source field missing %q: %#v", want, contract.MemoryContextProjectionContract.RequiredSourceFields)
		}
	}
	for _, want := range []string{"agent_run_id", "session_id", "state_version", "checkpoint_key", "memory_id", "context_ref", "input_ref_id"} {
		if !genericLivePackageContains(contract.MemoryContextProjectionContract.RequiredTargetFields, want) {
			t.Fatalf("memory context target field missing %q: %#v", want, contract.MemoryContextProjectionContract.RequiredTargetFields)
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
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 4 {
		t.Fatalf("fixture index header/count mismatch: %#v", index)
	}
	if index.Fixtures[0].FixtureKey != "generic:agent_state_store:offline" ||
		index.Fixtures[0].Capability != "generic.ai.agent_state.store" ||
		index.Fixtures[0].Path != manifest.Fixtures["agent_state_snapshot"] ||
		index.Fixtures[0].Schema != manifest.Schemas["agent_state_snapshot"] ||
		index.Fixtures[0].Metadata["replay_ready"] != true {
		t.Fatalf("fixture index entry mismatch: %#v", index.Fixtures[0])
	}
	if index.Fixtures[1].FixtureKey != "generic:agent_state_store:clean_skip" ||
		index.Fixtures[1].Capability != "generic.ai.agent_state.clean_skip" ||
		index.Fixtures[1].Path != manifest.Fixtures["checkpoint_clean_skip"] {
		t.Fatalf("clean-skip fixture index entry mismatch: %#v", index.Fixtures[1])
	}
	if index.Fixtures[2].FixtureKey != "generic:agent_state_store:trace_replay_projection" ||
		index.Fixtures[2].Capability != "generic.ai.agent_state.trace_replay_projection" ||
		index.Fixtures[2].Path != manifest.Fixtures["trace_replay_projection"] ||
		index.Fixtures[2].Schema != manifest.Schemas["trace_replay_projection"] ||
		index.Fixtures[2].Metadata["raw_payloads_allowed"] != false {
		t.Fatalf("trace replay projection fixture index entry mismatch: %#v", index.Fixtures[2])
	}
	if index.Fixtures[3].FixtureKey != "generic:agent_state_store:memory_context_projection" ||
		index.Fixtures[3].Capability != "generic.ai.agent_state.memory_context_projection" ||
		index.Fixtures[3].Path != manifest.Fixtures["memory_context_projection"] ||
		index.Fixtures[3].Schema != manifest.Schemas["memory_context_projection"] ||
		index.Fixtures[3].Metadata["raw_memory_text_allowed"] != false {
		t.Fatalf("memory context projection fixture index entry mismatch: %#v", index.Fixtures[3])
	}
}

func TestGenericAgentStateStoreLivePackageFixtureShape(t *testing.T) {
	base := genericAgentStateStorePackageDir(t)
	doc := loadGenericAgentStateCheckpointFixtureFromPath(t, filepath.Join(base, "fixtures", "agent_state_snapshot_fixture.json"))
	if doc.PackageBoundaryID != "generic-ai-agent-state-store" ||
		doc.Contract != "examples/ai/finrobot_translation/live_packages/generic_agent_state_store/contracts/generic_agent_state_store_contract.json" {
		t.Fatalf("unexpected package-local fixture header: %#v", doc)
	}
	assertGenericAgentStateSnapshots(t, doc)
}

func TestGenericAgentStateStoreLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericAgentStateStorePackageDir(t)
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "agent_state_snapshot_v1.schema.json"), []string{"schema_version", "id", "package_boundary_id", "provider_free", "domain_specific", "live_network", "live_model", "credentials_required", "real_dependency_imports", "depends_on_q_runtime", "contract", "state_snapshots", "replay_assertions"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "state_ref_v1.schema.json"), []string{"ref_id", "kind", "media_type", "digest", "summary", "raw_value_stored"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "checkpoint_v1.schema.json"), []string{"checkpoint_key", "cache_key", "key_algorithm", "key_material_fields", "excluded_fields", "stable_across_replay"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "trace_correlation_v1.schema.json"), []string{"trace_id", "event_id", "parent_event_id", "agent_run_id", "session_id", "turn_id", "checkpoint_key"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "redaction_audit_v1.schema.json"), []string{"enabled", "secret_values_present", "raw_prompt_stored", "raw_completion_stored", "redacted_fields", "placeholder", "credential_refs"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "trace_replay_projection_v1.schema.json"), []string{"schema_version", "id", "package_boundary_id", "provider_free", "domain_specific", "live_network", "live_model", "credentials_required", "real_dependency_imports", "depends_on_q_runtime", "source_fixture", "projection_kind", "raw_payloads_allowed", "secret_values_allowed", "projected_events", "replay_assertions"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "memory_context_projection_v1.schema.json"), []string{"schema_version", "id", "package_boundary_id", "provider_free", "domain_specific", "live_network", "live_model", "credentials_required", "real_dependency_imports", "depends_on_q_runtime", "source_fixture_refs", "projection_kind", "raw_memory_text_allowed", "secret_values_allowed", "context_mappings", "snapshot_context_windows", "projection_assertions"})
}

func TestGenericAgentStateStoreTraceReplayProjection(t *testing.T) {
	base := genericAgentStateStorePackageDir(t)
	source := loadGenericAgentStateCheckpointFixtureFromPath(t, filepath.Join(base, "fixtures", "agent_state_snapshot_fixture.json"))
	var projection struct {
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
		SourceFixture         string `json:"source_fixture"`
		ProjectionKind        string `json:"projection_kind"`
		RawPayloadsAllowed    bool   `json:"raw_payloads_allowed"`
		SecretValuesAllowed   bool   `json:"secret_values_allowed"`
		ProjectedEvents       []struct {
			EventType           string   `json:"event_type"`
			Sequence            int      `json:"sequence"`
			Status              string   `json:"status"`
			TraceID             string   `json:"trace_id"`
			EventID             string   `json:"event_id"`
			ParentEventID       string   `json:"parent_event_id"`
			AgentRunID          string   `json:"agent_run_id"`
			SessionID           string   `json:"session_id"`
			TurnID              string   `json:"turn_id"`
			StateVersion        int      `json:"state_version"`
			CheckpointKey       string   `json:"checkpoint_key"`
			CacheKey            string   `json:"cache_key"`
			ResumeToken         string   `json:"resume_token"`
			InputRefIDs         []string `json:"input_ref_ids"`
			OutputRefIDs        []string `json:"output_ref_ids"`
			ReplayMatchKey      string   `json:"replay_match_key"`
			RawPayloadsAllowed  bool     `json:"raw_payloads_allowed"`
			SecretValuesPresent bool     `json:"secret_values_present"`
		} `json:"projected_events"`
		ReplayAssertions map[string]bool `json:"replay_assertions"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "trace_replay_projection_fixture.json"), &projection)
	if projection.SchemaVersion != 1 ||
		projection.ID != "generic-agent-state-trace-replay-projection-fixture" ||
		projection.PackageBoundaryID != "generic-ai-agent-state-store" ||
		projection.SourceFixture != "fixtures/agent_state_snapshot_fixture.json" ||
		projection.ProjectionKind != "agent_state_checkpoint_trace_replay_projection" {
		t.Fatalf("unexpected projection header: %#v", projection)
	}
	if !projection.ProviderFree || projection.DomainSpecific || projection.LiveNetwork || projection.LiveModel ||
		projection.CredentialsRequired || projection.RealDependencyImports || projection.DependsOnQRuntime ||
		projection.RawPayloadsAllowed || projection.SecretValuesAllowed {
		t.Fatalf("projection must stay provider-free and no-secret: %#v", projection)
	}
	if len(projection.ProjectedEvents) != len(source.StateSnapshots) {
		t.Fatalf("projected event count = %d, want %d", len(projection.ProjectedEvents), len(source.StateSnapshots))
	}
	lastSequence := 0
	for i, event := range projection.ProjectedEvents {
		snapshot := source.StateSnapshots[i]
		if event.Sequence <= lastSequence {
			t.Fatalf("projection sequence is not increasing: %#v", projection.ProjectedEvents)
		}
		lastSequence = event.Sequence
		if event.AgentRunID != snapshot.AgentRunID ||
			event.SessionID != snapshot.SessionID ||
			event.StateVersion != snapshot.StateVersion ||
			event.Status != snapshot.Status ||
			event.TraceID != snapshot.TraceCorrelation.TraceID ||
			event.EventID != snapshot.TraceCorrelation.EventID ||
			event.ParentEventID != snapshot.TraceCorrelation.ParentEventID ||
			event.TurnID != snapshot.TraceCorrelation.TurnID ||
			event.CheckpointKey != snapshot.Checkpoint.CheckpointKey ||
			event.CacheKey != snapshot.Checkpoint.CacheKey ||
			event.ResumeToken != snapshot.ResumeToken {
			t.Fatalf("projection event does not match source snapshot: event=%#v snapshot=%#v", event, snapshot)
		}
		if event.RawPayloadsAllowed || event.SecretValuesPresent ||
			!strings.Contains(event.ReplayMatchKey, event.AgentRunID) ||
			!strings.Contains(event.ReplayMatchKey, event.SessionID) ||
			!strings.Contains(event.ReplayMatchKey, event.CheckpointKey) {
			t.Fatalf("projection replay/no-secret fields are invalid: %#v", event)
		}
	}
	for _, want := range []string{"sequence_strictly_increasing", "checkpoint_keys_match_source_snapshots", "resume_tokens_derive_from_checkpoint_keys", "replay_match_keys_include_state_version", "raw_payloads_absent", "secret_values_absent"} {
		if !projection.ReplayAssertions[want] {
			t.Fatalf("projection replay assertion missing %q: %#v", want, projection.ReplayAssertions)
		}
	}
}

func TestGenericAgentStateStoreMemoryContextProjection(t *testing.T) {
	base := genericAgentStateStorePackageDir(t)
	root := repoRoot(t)
	source := loadGenericMemoryStoreFixture(t, filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "generic_memory_store", "fixtures", "generic_memory_store_fixture.json"))
	target := loadGenericAgentStateCheckpointFixtureFromPath(t, filepath.Join(base, "fixtures", "agent_state_snapshot_fixture.json"))

	var projection struct {
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
		SourceFixtureRefs     struct {
			MemoryStore        string `json:"memory_store"`
			AgentStateSnapshot string `json:"agent_state_snapshot"`
		} `json:"source_fixture_refs"`
		ProjectionKind       string `json:"projection_kind"`
		RawMemoryTextAllowed bool   `json:"raw_memory_text_allowed"`
		SecretValuesAllowed  bool   `json:"secret_values_allowed"`
		ContextMappings      []struct {
			MemoryID            string  `json:"memory_id"`
			Rank                int     `json:"rank"`
			Score               float64 `json:"score"`
			ContextRef          string  `json:"context_ref"`
			SourceFixtureKey    string  `json:"source_fixture_key"`
			TargetAgentRunID    string  `json:"target_agent_run_id"`
			TargetSessionID     string  `json:"target_session_id"`
			TargetStateVersion  int     `json:"target_state_version"`
			TargetCheckpointKey string  `json:"target_checkpoint_key"`
			TargetInputRefID    string  `json:"target_input_ref_id"`
			ProjectionPolicy    string  `json:"projection_policy"`
		} `json:"context_mappings"`
		SnapshotContextWindows []struct {
			AgentRunID            string   `json:"agent_run_id"`
			SessionID             string   `json:"session_id"`
			StateVersion          int      `json:"state_version"`
			CheckpointKey         string   `json:"checkpoint_key"`
			ContextWindowStrategy string   `json:"context_window_strategy"`
			IncludedMemoryIDs     []string `json:"included_memory_ids"`
			ContextRefs           []string `json:"context_refs"`
			DeterministicOrder    bool     `json:"deterministic_order"`
			RawMemoryTextStored   bool     `json:"raw_memory_text_stored"`
		} `json:"snapshot_context_windows"`
		ProjectionAssertions map[string]bool `json:"projection_assertions"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "fixtures", "memory_context_projection_fixture.json"), &projection)
	if projection.SchemaVersion != 1 ||
		projection.ID != "generic-agent-state-memory-context-projection-fixture" ||
		projection.PackageBoundaryID != "generic-ai-agent-state-store" ||
		projection.ProjectionKind != "memory_context_to_agent_state_projection" ||
		projection.SourceFixtureRefs.MemoryStore == "" ||
		projection.SourceFixtureRefs.AgentStateSnapshot == "" {
		t.Fatalf("unexpected memory context projection header: %#v", projection)
	}
	if !projection.ProviderFree || projection.DomainSpecific || projection.LiveNetwork || projection.LiveModel ||
		projection.CredentialsRequired || projection.RealDependencyImports || projection.DependsOnQRuntime ||
		projection.RawMemoryTextAllowed || projection.SecretValuesAllowed {
		t.Fatalf("memory context projection must stay provider-free and ref-only: %#v", projection)
	}
	if len(projection.ContextMappings) != len(source.RetrievalResults) {
		t.Fatalf("context mapping count = %d, want %d", len(projection.ContextMappings), len(source.RetrievalResults))
	}

	memoryItems := map[string]bool{}
	for _, item := range source.MemoryItems {
		memoryItems[item.MemoryID] = true
	}
	retrievalByMemory := map[string]struct {
		Rank       int
		Score      float64
		ContextRef string
	}{}
	for _, result := range source.RetrievalResults {
		retrievalByMemory[result.MemoryID] = struct {
			Rank       int
			Score      float64
			ContextRef string
		}{Rank: result.Rank, Score: result.Score, ContextRef: result.ContextRef}
	}
	snapshots := map[string]bool{}
	inputRefs := map[string]bool{}
	for _, snapshot := range target.StateSnapshots {
		key := fmt.Sprintf("%s/%s/%d/%s", snapshot.AgentRunID, snapshot.SessionID, snapshot.StateVersion, snapshot.Checkpoint.CheckpointKey)
		snapshots[key] = true
		for _, ref := range snapshot.InputRefs {
			inputRefs[ref.RefID] = true
		}
	}

	lastRank := 0
	for _, mapping := range projection.ContextMappings {
		retrieval, ok := retrievalByMemory[mapping.MemoryID]
		if !ok || !memoryItems[mapping.MemoryID] {
			t.Fatalf("mapping references unknown memory result: %#v", mapping)
		}
		if mapping.Rank <= lastRank || mapping.Rank != retrieval.Rank ||
			mapping.Score != retrieval.Score ||
			mapping.ContextRef != retrieval.ContextRef ||
			mapping.SourceFixtureKey != source.Provenance.FixtureKey ||
			mapping.ProjectionPolicy != "memory_ref_only" {
			t.Fatalf("mapping does not preserve deterministic retrieval refs: mapping=%#v retrieval=%#v", mapping, retrieval)
		}
		lastRank = mapping.Rank
		snapshotKey := fmt.Sprintf("%s/%s/%d/%s", mapping.TargetAgentRunID, mapping.TargetSessionID, mapping.TargetStateVersion, mapping.TargetCheckpointKey)
		if !snapshots[snapshotKey] {
			t.Fatalf("mapping target snapshot does not resolve: %#v", mapping)
		}
		if !inputRefs[mapping.TargetInputRefID] {
			t.Fatalf("mapping target input ref does not resolve: %#v", mapping)
		}
	}
	if len(projection.SnapshotContextWindows) != 1 {
		t.Fatalf("snapshot context windows = %d, want 1", len(projection.SnapshotContextWindows))
	}
	window := projection.SnapshotContextWindows[0]
	if !window.DeterministicOrder || window.RawMemoryTextStored ||
		window.ContextWindowStrategy != source.ContextWindow.Strategy ||
		!sameStringSet(window.IncludedMemoryIDs, source.ContextWindow.IncludedMemoryIDs) {
		t.Fatalf("snapshot context window does not match source context window: window=%#v source=%#v", window, source.ContextWindow)
	}
	for _, want := range []string{"projection_is_provider_free", "source_memory_store_is_ref_only", "raw_memory_text_absent", "secret_values_absent", "context_refs_match_retrieval_results", "included_memory_ids_match_context_window", "target_snapshot_identity_resolves", "target_input_ref_resolves", "deterministic_order_preserved"} {
		if !projection.ProjectionAssertions[want] {
			t.Fatalf("memory context projection assertion missing %q: %#v", want, projection.ProjectionAssertions)
		}
	}
}

func TestGenericAgentStateStoreLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericAgentStateStorePackageDir(t), "main.leia")
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
			got, err := vm.Get("generic_agent_state_store_live_package_summary")
			if err != nil {
				t.Fatalf("Get generic_agent_state_store_live_package_summary: %v", err)
			}
			want := "generic_agent_state_store_live_package capability=generic.ai.agent_state.store fixture=generic:agent_state_store:offline snapshots=2 checkpoint=sha256 memory_context_projections=1 provider_free=true live_network=false imports=false"
			if got != want {
				t.Fatalf("summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func genericAgentStateStorePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_agent_state_store")
}

func loadGenericAgentStateCheckpointFixtureFromPath(t *testing.T, path string) genericAgentStateCheckpointDoc {
	t.Helper()
	var doc genericAgentStateCheckpointDoc
	decodeGenericAgentStateJSONFile(t, path, &doc)
	return doc
}

func sameStringSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]int{}
	for _, value := range got {
		seen[value]++
	}
	for _, value := range want {
		if seen[value] == 0 {
			return false
		}
		seen[value]--
	}
	return true
}

func assertGenericAgentStateSnapshots(t *testing.T, doc genericAgentStateCheckpointDoc) {
	t.Helper()
	if !doc.ProviderFree || doc.DomainSpecific || doc.LiveNetwork || doc.LiveModel ||
		doc.CredentialsRequired || doc.RealDependencyImports || doc.DependsOnQRuntime {
		t.Fatalf("fixture must stay provider-free, generic, and local-only: %#v", doc)
	}
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
		if snapshot.TraceCorrelation.AgentRunID != snapshot.AgentRunID ||
			snapshot.TraceCorrelation.SessionID != snapshot.SessionID ||
			snapshot.TraceCorrelation.CheckpointKey != snapshot.Checkpoint.CheckpointKey {
			t.Fatalf("trace correlation does not match state/checkpoint identity: %#v", snapshot.TraceCorrelation)
		}
		if seenEvents[snapshot.TraceCorrelation.EventID] {
			t.Fatalf("duplicate trace event id %q", snapshot.TraceCorrelation.EventID)
		}
		seenEvents[snapshot.TraceCorrelation.EventID] = true
		assertGenericAgentStateCheckpointKeys(t, snapshot.Checkpoint.CheckpointKey, snapshot.Checkpoint.CacheKey, snapshot.Checkpoint.KeyAlgorithm)
		if !snapshot.Redaction.Enabled || snapshot.Redaction.SecretValuesPresent ||
			snapshot.Redaction.RawPromptStored || snapshot.Redaction.RawCompletionStored ||
			snapshot.Redaction.Placeholder != "<redacted>" {
			t.Fatalf("redaction contract is not no-secret safe: %#v", snapshot.Redaction)
		}
	}
}
