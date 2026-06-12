package leia_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type genericAgentRunnerManifest struct {
	SchemaVersion               int    `json:"schema_version"`
	ID                          string `json:"id"`
	PackageName                 string `json:"package_name"`
	ProviderFree                bool   `json:"provider_free"`
	LiveNetworkDefault          bool   `json:"live_network_default"`
	RealDependencyImportDefault bool   `json:"real_dependency_import_default"`
	DefaultPolicy               struct {
		Mode                        string `json:"mode"`
		LiveNetwork                 bool   `json:"live_network"`
		ProviderCredentialsRequired bool   `json:"provider_credentials_required"`
		RealDependencyImports       bool   `json:"real_dependency_imports"`
		CleanSkipWithoutDependency  bool   `json:"clean_skip_without_dependency"`
		FixtureHook                 string `json:"fixture_hook"`
	} `json:"default_policy"`
	SourceExamples      []string          `json:"source_examples"`
	Entrypoints         map[string]string `json:"entrypoints"`
	Schemas             map[string]string `json:"schemas"`
	Fixtures            map[string]string `json:"fixtures"`
	Capabilities        []string          `json:"capabilities"`
	DialectBackendShape string            `json:"dialect_backend_shape"`
	NoBuiltInGuarantee  struct {
		Required  bool   `json:"required"`
		Statement string `json:"statement"`
	} `json:"no_built_in_guarantee"`
}

func TestGenericAgentRunnerLivePackageManifest(t *testing.T) {
	root := repoRoot(t)
	base := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "generic_agent_runner")
	var manifest genericAgentRunnerManifest
	decodeGenericAgentRunnerJSON(t, filepath.Join(base, "package.manifest.json"), &manifest)
	if manifest.SchemaVersion != 1 || manifest.ID != "generic-ai-agent-runner-live-package" {
		t.Fatalf("manifest header = %#v", manifest)
	}
	if manifest.PackageName != "leia-generic-ai-agent-runner" || manifest.DialectBackendShape != "ai.agent.run" {
		t.Fatalf("manifest package/backend = %q/%q", manifest.PackageName, manifest.DialectBackendShape)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("manifest must be provider-free: %#v", manifest)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_generic_agent_runner_fixture" {
		t.Fatalf("default policy must stay provider-free fixture replay: %#v", manifest.DefaultPolicy)
	}
	for _, source := range manifest.SourceExamples {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(source))); err != nil {
			t.Fatalf("source example %q: %v", source, err)
		}
	}
	for _, key := range []string{"smoke", "contract", "fixture_index", "tool_contract_agent_projection"} {
		assertGenericAgentRunnerPath(t, base, manifest.Entrypoints[key])
	}
	for _, key := range []string{"agent_config", "loop_trace", "structured_output", "tool_contract_agent_projection"} {
		assertGenericAgentRunnerPath(t, base, manifest.Schemas[key])
		assertGenericAgentRunnerPath(t, base, manifest.Fixtures[key])
	}
	for _, want := range []string{
		"ai.agent.config.declarative",
		"ai.agent.tool_history.replay",
		"ai.agent.loop_trace.emit",
		"ai.agent.structured_output.validate",
		"ai.agent.replay_policy.enforce",
		"ai.agent.max_steps.guard",
		"ai.agent.tool_contract_projection",
	} {
		if !contains(manifest.Capabilities, want) {
			t.Fatalf("capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}
	if !manifest.NoBuiltInGuarantee.Required ||
		!strings.Contains(manifest.NoBuiltInGuarantee.Statement, manifest.PackageName) {
		t.Fatalf("no_built_in_guarantee inconsistent: %#v", manifest.NoBuiltInGuarantee)
	}
}

func TestGenericAgentRunnerLivePackageContractsAndFixtures(t *testing.T) {
	base := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_agent_runner")
	var contract struct {
		ProviderFree bool     `json:"provider_free"`
		LiveNetwork  bool     `json:"live_network"`
		BackendShape string   `json:"backend_shape"`
		Inputs       []string `json:"inputs"`
		Outputs      []string `json:"outputs"`
		LoopTrace    struct {
			Events                 []string `json:"events"`
			CorrelationIDsRequired bool     `json:"correlation_ids_required"`
			RedactionPolicy        string   `json:"redaction_policy"`
		} `json:"loop_trace"`
		Guards struct {
			MaxStepsRequired            bool `json:"max_steps_required"`
			LiveToolInvocation          bool `json:"live_tool_invocation"`
			ProviderCredentialsRequired bool `json:"provider_credentials_required"`
			CleanSkipWithoutReplay      bool `json:"clean_skip_without_replay"`
		} `json:"guards"`
		ToolContractProjection struct {
			Schema                    string   `json:"schema"`
			SourcePackage             string   `json:"source_package"`
			TargetSurfaces            []string `json:"target_surfaces"`
			CorrelationOwner          string   `json:"correlation_owner"`
			TraceEventSchemaRedefined bool     `json:"trace_event_schema_redefined"`
			ProviderFree              bool     `json:"provider_free"`
		} `json:"tool_contract_projection"`
	}
	decodeGenericAgentRunnerJSON(t, filepath.Join(base, "contracts", "agent_runner_contract.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.BackendShape != "ai.agent.run" {
		t.Fatalf("contract provider/backend = %#v", contract)
	}
	if len(contract.Inputs) != 4 || len(contract.Outputs) != 4 || len(contract.LoopTrace.Events) != 6 {
		t.Fatalf("contract shape incomplete: %#v", contract)
	}
	if !contract.LoopTrace.CorrelationIDsRequired || contract.LoopTrace.RedactionPolicy != "no_secret_values" {
		t.Fatalf("loop trace policy incomplete: %#v", contract.LoopTrace)
	}
	if !contract.Guards.MaxStepsRequired || contract.Guards.LiveToolInvocation || contract.Guards.ProviderCredentialsRequired || !contract.Guards.CleanSkipWithoutReplay {
		t.Fatalf("agent guards must fail closed and stay provider-free: %#v", contract.Guards)
	}
	if contract.ToolContractProjection.Schema != "tool_contract_agent_projection_v1" ||
		contract.ToolContractProjection.SourcePackage != "generic_tool_contracts" ||
		contract.ToolContractProjection.CorrelationOwner != "agent_runner" ||
		contract.ToolContractProjection.TraceEventSchemaRedefined ||
		!contract.ToolContractProjection.ProviderFree ||
		!contains(contract.ToolContractProjection.TargetSurfaces, "agent_config.tools") ||
		!contains(contract.ToolContractProjection.TargetSurfaces, "structured_output.tool_history") ||
		!contains(contract.ToolContractProjection.TargetSurfaces, "loop_trace.events") {
		t.Fatalf("tool contract projection contract incomplete: %#v", contract.ToolContractProjection)
	}

	var fixtureIndex struct {
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
		Fixtures     []struct {
			Key          string `json:"key"`
			Path         string `json:"path"`
			ProviderFree bool   `json:"provider_free"`
		} `json:"fixtures"`
	}
	decodeGenericAgentRunnerJSON(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &fixtureIndex)
	if !fixtureIndex.ProviderFree || fixtureIndex.LiveNetwork || len(fixtureIndex.Fixtures) != 4 {
		t.Fatalf("fixture index incomplete: %#v", fixtureIndex)
	}
	for _, fixture := range fixtureIndex.Fixtures {
		if !fixture.ProviderFree {
			t.Fatalf("fixture is not provider-free: %#v", fixture)
		}
		assertGenericAgentRunnerPath(t, base, fixture.Path)
	}
}

func TestGenericAgentRunnerLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_agent_runner", "main.leia")
	want := "generic_agent_runner_live_package backend=ai.agent.run tools=2 max_steps=4 trace_events=6 output_fields=3 capabilities=7 tool_contract_projections=1 provider_free=true live_network=false imports=false"
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
			got, err := vm.Get("generic_agent_runner_live_package_summary")
			if err != nil {
				t.Fatalf("Get summary: %v", err)
			}
			if got != want {
				t.Fatalf("summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func TestGenericAgentRunnerToolContractProjection(t *testing.T) {
	root := repoRoot(t)
	base := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "generic_agent_runner")
	toolBase := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "generic_tool_contracts")

	var source struct {
		SchemaVersion         int    `json:"schema_version"`
		FixtureKey            string `json:"fixture_key"`
		ProjectionKind        string `json:"projection_kind"`
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		DescriptorProjections []struct {
			SourceDescriptor struct {
				ToolName         string `json:"tool_name"`
				SourceFixtureKey string `json:"source_fixture_key"`
			} `json:"source_descriptor"`
			ProjectedToolContract struct {
				Name     string `json:"name"`
				Approval struct {
					Required bool   `json:"required"`
					State    string `json:"state"`
				} `json:"approval"`
			} `json:"projected_tool_contract"`
			ProjectedResultEnvelope struct {
				OK    bool `json:"ok"`
				Error *struct {
					Kind string `json:"kind"`
				} `json:"error"`
				Replay struct {
					ReplayKey string `json:"replay_key"`
				} `json:"replay"`
			} `json:"projected_result_envelope"`
		} `json:"descriptor_projections"`
	}
	decodeGenericAgentRunnerJSON(t, filepath.Join(toolBase, "fixtures", "registry_descriptor_to_tool_contract_projection_fixture.json"), &source)

	var agentConfig struct {
		Tools []struct {
			Name         string `json:"name"`
			Capability   string `json:"capability"`
			ProviderFree bool   `json:"provider_free"`
		} `json:"tools"`
		ReplayPolicy struct {
			Mode               string `json:"mode"`
			StrictOrderedMatch bool   `json:"strict_ordered_match"`
			UnconsumedRecords  string `json:"unconsumed_records"`
		} `json:"replay_policy"`
	}
	decodeGenericAgentRunnerJSON(t, filepath.Join(base, "fixtures", "agent_config_fixture.json"), &agentConfig)
	var loopTrace struct {
		Events []struct {
			Type string `json:"type"`
		} `json:"events"`
	}
	decodeGenericAgentRunnerJSON(t, filepath.Join(base, "fixtures", "loop_trace_fixture.json"), &loopTrace)
	var structuredOutput struct {
		ToolHistory []struct {
			Tool      string `json:"tool"`
			Status    string `json:"status"`
			ResultRef string `json:"result_ref"`
		} `json:"tool_history"`
	}
	decodeGenericAgentRunnerJSON(t, filepath.Join(base, "fixtures", "structured_output_fixture.json"), &structuredOutput)

	var projection struct {
		SchemaVersion        int    `json:"schema_version"`
		FixtureKey           string `json:"fixture_key"`
		ProjectionKind       string `json:"projection_kind"`
		ProviderFree         bool   `json:"provider_free"`
		LiveNetwork          bool   `json:"live_network"`
		RealDependencyImport bool   `json:"real_dependency_imports"`
		SourceFixtureRefs    struct {
			ToolContractProjection string `json:"tool_contract_projection"`
			AgentConfig            string `json:"agent_config"`
			LoopTrace              string `json:"loop_trace"`
			StructuredOutput       string `json:"structured_output"`
		} `json:"source_fixture_refs"`
		ProjectedAgentTools []struct {
			SourceToolName    string `json:"source_tool_name"`
			AgentToolName     string `json:"agent_tool_name"`
			Capability        string `json:"capability"`
			ApprovalRequired  bool   `json:"approval_required"`
			ProviderFree      bool   `json:"provider_free"`
			ReplayKey         string `json:"replay_key"`
			AgentConfigAction string `json:"agent_config_action"`
		} `json:"projected_agent_tools"`
		ToolHistoryMappings []struct {
			Seq             int    `json:"seq"`
			SourceToolName  string `json:"source_tool_name"`
			AgentToolName   string `json:"agent_tool_name"`
			SourceResultOK  bool   `json:"source_result_ok"`
			SourceErrorKind string `json:"source_error_kind"`
			AgentStatus     string `json:"agent_status"`
			ResultRef       string `json:"result_ref"`
			ReplayReady     bool   `json:"replay_ready"`
			ProviderFree    bool   `json:"provider_free"`
		} `json:"tool_history_mappings"`
		LoopTraceMappings []struct {
			SourceToolName         string `json:"source_tool_name"`
			SourceReplayKey        string `json:"source_replay_key"`
			AgentEventType         string `json:"agent_event_type"`
			AgentCorrelationPolicy string `json:"agent_correlation_policy"`
			AgentStatus            string `json:"agent_status"`
		} `json:"loop_trace_mappings"`
		ReplayPolicyMapping struct {
			SourceReplayMode       string `json:"source_replay_mode"`
			AgentReplayMode        string `json:"agent_replay_mode"`
			StrictOrderedMatch     bool   `json:"strict_ordered_match"`
			UnconsumedRecords      string `json:"unconsumed_records"`
			CleanSkipWithoutReplay bool   `json:"clean_skip_without_replay"`
		} `json:"replay_policy_mapping"`
		ProjectionAssertions map[string]bool `json:"projection_assertions"`
	}
	decodeGenericAgentRunnerJSON(t, filepath.Join(base, "fixtures", "tool_contract_agent_projection_fixture.json"), &projection)
	if projection.SchemaVersion != 1 ||
		projection.FixtureKey != "generic:agent_runner:tool_contract_projection" ||
		projection.ProjectionKind != "tool_contract_to_agent_runner_projection" ||
		!projection.ProviderFree || projection.LiveNetwork || projection.RealDependencyImport ||
		projection.SourceFixtureRefs.ToolContractProjection == "" ||
		projection.SourceFixtureRefs.AgentConfig == "" ||
		projection.SourceFixtureRefs.LoopTrace == "" ||
		projection.SourceFixtureRefs.StructuredOutput == "" {
		t.Fatalf("projection header incomplete: %#v", projection)
	}
	if !source.ProviderFree || source.LiveNetwork || source.RealDependencyImports || len(source.DescriptorProjections) != 2 {
		t.Fatalf("source tool projection boundary invalid: %#v", source)
	}

	sourceTools := map[string]struct {
		replayKey string
		required  bool
		ok        bool
		errorKind string
	}{}
	for _, item := range source.DescriptorProjections {
		sourceTools[item.ProjectedToolContract.Name] = struct {
			replayKey string
			required  bool
			ok        bool
			errorKind string
		}{
			replayKey: item.ProjectedResultEnvelope.Replay.ReplayKey,
			required:  item.ProjectedToolContract.Approval.Required,
			ok:        item.ProjectedResultEnvelope.OK,
			errorKind: genericAgentRunnerProjectionErrorKind(item.ProjectedResultEnvelope.Error),
		}
	}
	if len(projection.ProjectedAgentTools) != 2 || len(projection.ToolHistoryMappings) != 2 || len(projection.LoopTraceMappings) != 2 {
		t.Fatalf("projection mapping counts incomplete: %#v", projection)
	}
	for _, tool := range projection.ProjectedAgentTools {
		sourceTool, ok := sourceTools[tool.SourceToolName]
		if !ok || tool.AgentToolName != tool.SourceToolName ||
			tool.Capability != "generic.ai.tool.contract" ||
			tool.ApprovalRequired != sourceTool.required ||
			tool.ReplayKey != sourceTool.replayKey ||
			!tool.ProviderFree ||
			!strings.HasPrefix(tool.AgentConfigAction, "append_tool_config") {
			t.Fatalf("projected agent tool does not resolve to source contract: %#v", tool)
		}
	}
	for _, mapping := range projection.ToolHistoryMappings {
		sourceTool, ok := sourceTools[mapping.SourceToolName]
		if !ok || mapping.AgentToolName != mapping.SourceToolName ||
			mapping.SourceResultOK != sourceTool.ok ||
			mapping.SourceErrorKind != sourceTool.errorKind ||
			mapping.ResultRef != sourceTool.replayKey ||
			!mapping.ReplayReady || !mapping.ProviderFree {
			t.Fatalf("tool history mapping does not resolve to result envelope: %#v", mapping)
		}
		if sourceTool.ok && mapping.AgentStatus != "ok" {
			t.Fatalf("successful source result must project to ok agent history: %#v", mapping)
		}
		if !sourceTool.ok && mapping.AgentStatus != "denied" {
			t.Fatalf("failed approval source result must project to denied agent history: %#v", mapping)
		}
	}
	loopEventTypes := map[string]bool{}
	for _, event := range loopTrace.Events {
		loopEventTypes[event.Type] = true
	}
	for _, mapping := range projection.LoopTraceMappings {
		if !loopEventTypes[mapping.AgentEventType] ||
			mapping.AgentEventType != "tool_result" ||
			sourceTools[mapping.SourceToolName].replayKey != mapping.SourceReplayKey ||
			mapping.AgentCorrelationPolicy != "new_tool_correlation_id_per_replay_key" {
			t.Fatalf("loop trace mapping crosses trace ownership boundary: %#v", mapping)
		}
	}
	if projection.ReplayPolicyMapping.SourceReplayMode != "fixture_replay" ||
		projection.ReplayPolicyMapping.AgentReplayMode != agentConfig.ReplayPolicy.Mode ||
		projection.ReplayPolicyMapping.StrictOrderedMatch != agentConfig.ReplayPolicy.StrictOrderedMatch ||
		projection.ReplayPolicyMapping.UnconsumedRecords != agentConfig.ReplayPolicy.UnconsumedRecords ||
		!projection.ReplayPolicyMapping.CleanSkipWithoutReplay {
		t.Fatalf("replay policy mapping invalid: %#v", projection.ReplayPolicyMapping)
	}
	if len(agentConfig.Tools) != 2 || len(structuredOutput.ToolHistory) != 1 {
		t.Fatalf("baseline agent fixtures drifted: tools=%d history=%d", len(agentConfig.Tools), len(structuredOutput.ToolHistory))
	}
	for _, want := range []string{"tool_contract_names_project_to_agent_tools", "result_envelopes_project_to_tool_history", "approval_denied_projects_to_denied_history_not_live_execution", "replay_keys_are_preserved_as_result_refs", "agent_loop_trace_owns_correlation_ids", "trace_event_schema_is_not_redefined", "provider_free_boundary_preserved", "live_network_absent", "real_dependency_imports_absent"} {
		if !projection.ProjectionAssertions[want] {
			t.Fatalf("projection assertion missing %q: %#v", want, projection.ProjectionAssertions)
		}
	}
}

func decodeGenericAgentRunnerJSON(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	lower := strings.ToLower(string(data))
	if strings.Contains(lower, `"provider_free": false`) || strings.Contains(lower, `"live_network": true`) {
		t.Fatalf("%s must stay provider-free and network-off", path)
	}
}

func assertGenericAgentRunnerPath(t *testing.T, base, rel string) {
	t.Helper()
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(filepath.ToSlash(rel), "../") {
		t.Fatalf("invalid package path %q", rel)
	}
	if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(rel))); err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
}

func genericAgentRunnerProjectionErrorKind(value *struct {
	Kind string `json:"kind"`
}) string {
	if value == nil {
		return ""
	}
	return value.Kind
}
