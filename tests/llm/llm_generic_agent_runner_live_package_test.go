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
	SchemaVersion               int               `json:"schema_version"`
	ID                          string            `json:"id"`
	PackageName                 string            `json:"package_name"`
	ProviderFree                bool              `json:"provider_free"`
	LiveNetworkDefault          bool              `json:"live_network_default"`
	RealDependencyImportDefault bool              `json:"real_dependency_import_default"`
	SourceExamples              []string          `json:"source_examples"`
	Entrypoints                 map[string]string `json:"entrypoints"`
	Schemas                     map[string]string `json:"schemas"`
	Fixtures                    map[string]string `json:"fixtures"`
	Capabilities                []string          `json:"capabilities"`
	BackendShape                string            `json:"backend_shape"`
}

func TestGenericAgentRunnerLivePackageManifest(t *testing.T) {
	root := repoRoot(t)
	base := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "generic_agent_runner")
	var manifest genericAgentRunnerManifest
	decodeGenericAgentRunnerJSON(t, filepath.Join(base, "package.manifest.json"), &manifest)
	if manifest.SchemaVersion != 1 || manifest.ID != "generic-ai-agent-runner-live-package" {
		t.Fatalf("manifest header = %#v", manifest)
	}
	if manifest.PackageName != "leia-generic-ai-agent-runner" || manifest.BackendShape != "ai.agent.run" {
		t.Fatalf("manifest package/backend = %q/%q", manifest.PackageName, manifest.BackendShape)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("manifest must be provider-free: %#v", manifest)
	}
	for _, source := range manifest.SourceExamples {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(source))); err != nil {
			t.Fatalf("source example %q: %v", source, err)
		}
	}
	for _, key := range []string{"smoke", "contract", "fixture_index"} {
		assertGenericAgentRunnerPath(t, base, manifest.Entrypoints[key])
	}
	for _, key := range []string{"agent_config", "loop_trace", "structured_output"} {
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
	} {
		if !contains(manifest.Capabilities, want) {
			t.Fatalf("capabilities missing %q: %#v", want, manifest.Capabilities)
		}
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
	if !fixtureIndex.ProviderFree || fixtureIndex.LiveNetwork || len(fixtureIndex.Fixtures) != 3 {
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
	want := "generic_agent_runner_live_package backend=ai.agent.run tools=2 max_steps=4 trace_events=6 output_fields=3 capabilities=6 provider_free=true live_network=false imports=false"
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
