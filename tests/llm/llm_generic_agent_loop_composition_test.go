package leia_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

const genericAgentLoopCompositionExample = "examples/ai/finrobot_translation/generic_agent_loop_composition.leia"

func TestGenericAgentLoopCompositionExampleIsProviderFreeAndGeneric(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(genericAgentLoopCompositionExample)))
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ToLower(string(data))
	for _, want := range []string{
		"generic_agent_runner",
		"generic_turn_runner",
		"generic_tool_contracts",
		"generic_trace_events",
		"llm.dispatch",
		"provider_free: true",
		"live_network: false",
		"live_model: false",
		"live_provider_calls: 0",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("generic agent loop example missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"finrobot",
		"finance",
		"openai",
		"anthropic",
		"vendor",
		"provider_model",
		"provider_client",
		"mock-",
		"trading.",
		"openbb",
		"fingpt",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("generic agent loop example contains provider/domain-specific marker %q", forbidden)
		}
	}
}

func TestGenericAgentLoopCompositionExampleIsDiscoverable(t *testing.T) {
	root := repoRoot(t)
	cmd := exec.Command("go", "run", "./cmd/leia", "examples", "list", "--json")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("examples list failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	var payload struct {
		Examples []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Section  string `json:"section"`
			Path     string `json:"path"`
			Runnable bool   `json:"runnable"`
		} `json:"examples"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode examples list: %v\n%s", err, stdout.String())
	}
	for _, example := range payload.Examples {
		if example.Path != genericAgentLoopCompositionExample {
			continue
		}
		if example.ID != "repo-ai-finrobot_translation-generic_agent_loop_composition" ||
			example.Title != "generic agent loop composition" ||
			example.Section != "Ai" ||
			!example.Runnable {
			t.Fatalf("discoverable example metadata = %#v", example)
		}
		return
	}
	t.Fatalf("examples list missing %s", genericAgentLoopCompositionExample)
}

func TestGenericAgentLoopCompositionExampleExecutesWithoutProvider(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
			}, tc.opts...)...)
			if err := vm.ExecFile(filepath.Join(repoRoot(t), filepath.FromSlash(genericAgentLoopCompositionExample))); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}
			if err := vm.Exec(`
probe_agent_runner_id := generic_agent_runner.id
probe_turn_runner_id := generic_turn_runner.id
probe_tool_contracts_id := generic_tool_contracts.id
probe_trace_events_id := generic_trace_events.id
probe_runner_turn_contract := generic_agent_runner.turn_runner
probe_runner_tool_contract := generic_agent_runner.tool_contracts
probe_runner_trace_contract := generic_agent_runner.trace_events
probe_tool_count := #generic_tool_contracts.tools
probe_schema_1_name := generic_tool_contracts.schemas[1].name
probe_schema_2_name := generic_tool_contracts.schemas[2].name
probe_turn_count := #agent_loop_result.turns
probe_tool_result_count := #agent_loop_result.tool_results
probe_trace_count := #generic_trace_events.events
probe_first_trace_kind := generic_trace_events.events[1].kind
probe_last_trace_kind := generic_trace_events.events[8].kind
probe_result_status := agent_loop_result.status
probe_provider_free := agent_loop_result.provider_free
probe_live_network := agent_loop_result.live_network
probe_live_model := agent_loop_result.live_model
probe_live_provider_calls := agent_loop_result.live_provider_calls
probe_summary := agent_loop_summary
`); err != nil {
				t.Fatalf("Exec probes: %v", err)
			}
			for name, want := range map[string]any{
				"probe_agent_runner_id":       "generic_agent_runner_v1",
				"probe_turn_runner_id":        "generic_turn_runner_v1",
				"probe_tool_contracts_id":     "generic_tool_contracts_v1",
				"probe_trace_events_id":       "generic_trace_events_v1",
				"probe_runner_turn_contract":  "generic_turn_runner_v1",
				"probe_runner_tool_contract":  "generic_tool_contracts_v1",
				"probe_runner_trace_contract": "generic_trace_events_v1",
				"probe_tool_count":            int64(2),
				"probe_schema_1_name":         "lookup_context",
				"probe_schema_2_name":         "compose_response",
				"probe_turn_count":            int64(2),
				"probe_tool_result_count":     int64(2),
				"probe_trace_count":           int64(8),
				"probe_first_trace_kind":      "agent.start",
				"probe_last_trace_kind":       "agent.done",
				"probe_result_status":         "ok",
				"probe_provider_free":         true,
				"probe_live_network":          false,
				"probe_live_model":            false,
				"probe_live_provider_calls":   int64(0),
				"probe_summary":               "response:compose a launch checklist:local-context:compose a launch checklist",
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
		})
	}
}

func TestGenericAgentLoopCompositionEvaluateGate(t *testing.T) {
	root := repoRoot(t)
	reportPath := filepath.Join(t.TempDir(), "generic_agent_loop_composition.report.json")
	cmd := exec.Command(
		"go", "run", "./cmd/leia",
		"evaluate",
		"--gate",
		"--report", reportPath,
		genericAgentLoopCompositionExample,
	)
	cmd.Dir = root
	cmd.Env = withoutLLMProviderEnv(os.Environ())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("evaluate failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Status  string `json:"status"`
		Summary struct {
			EvaluateBlocks int     `json:"evaluate_blocks"`
			CasesPassed    int     `json:"cases_passed"`
			CasesFailed    int     `json:"cases_failed"`
			Assertions     int     `json:"assertions"`
			PassRate       float64 `json:"pass_rate"`
			LLMTurns       int     `json:"llm_turns"`
		} `json:"summary"`
		LLM      any   `json:"llm"`
		Findings []any `json:"findings"`
		Cases    []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, string(data))
	}
	if report.Status != "ok" ||
		report.Summary.EvaluateBlocks != 1 ||
		report.Summary.CasesPassed != 1 ||
		report.Summary.CasesFailed != 0 ||
		report.Summary.Assertions != 20 ||
		report.Summary.PassRate != 1 {
		t.Fatalf("evaluate summary = %#v status=%q", report.Summary, report.Status)
	}
	if report.LLM != nil || report.Summary.LLMTurns != 0 {
		t.Fatalf("evaluate report used LLM provider: llm=%#v summary=%#v", report.LLM, report.Summary)
	}
	if len(report.Findings) != 0 || len(report.Cases) != 1 ||
		report.Cases[0].Name != "generic agent loop provider-free composition" ||
		report.Cases[0].Status != "passed" {
		t.Fatalf("evaluate cases/findings = %#v / %#v", report.Cases, report.Findings)
	}
}
