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
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 2 {
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
			want := "generic_agent_state_store_live_package capability=generic.ai.agent_state.store fixture=generic:agent_state_store:offline snapshots=2 checkpoint=sha256 provider_free=true live_network=false imports=false"
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
