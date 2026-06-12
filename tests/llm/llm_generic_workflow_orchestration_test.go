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

const genericWorkflowOrchestrationExample = "examples/ai/finrobot_translation/generic_workflow_orchestration.leia"

func TestGenericWorkflowOrchestrationExampleIsProviderFreeAndGeneric(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(genericWorkflowOrchestrationExample)))
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ToLower(string(data))
	for _, want := range []string{"workflow_orchestrator", "approval_policy", "evaluation_harness", "provider_free: true", "domain_specific: false"} {
		if !strings.Contains(source, want) {
			t.Fatalf("generic orchestration example missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"finrobot",
		"finance",
		"openai",
		"anthropic",
		"provider_model",
		"mock-",
		"trading.",
		"openbb",
		"fingpt",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("generic orchestration example contains provider/domain-specific marker %q", forbidden)
		}
	}
}

func TestGenericWorkflowOrchestrationExampleIsDiscoverable(t *testing.T) {
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
		if example.Path != genericWorkflowOrchestrationExample {
			continue
		}
		if example.ID != "repo-ai-finrobot_translation-generic_workflow_orchestration" ||
			example.Title != "generic workflow orchestration" ||
			example.Section != "Ai" ||
			!example.Runnable {
			t.Fatalf("discoverable example metadata = %#v", example)
		}
		return
	}
	t.Fatalf("examples list missing %s", genericWorkflowOrchestrationExample)
}

func TestGenericWorkflowOrchestrationExampleExecutesWithoutProvider(t *testing.T) {
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
			if err := vm.ExecFile(filepath.Join(repoRoot(t), filepath.FromSlash(genericWorkflowOrchestrationExample))); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}
			if err := vm.Exec(`
probe_contract_id := orchestration_contract.id
probe_provider_free := orchestration_contract.provider_free
probe_live_network := orchestration_contract.live_network
probe_live_model := orchestration_contract.live_model
probe_domain_specific := orchestration_contract.domain_specific
probe_capability_count := #orchestration_contract.capabilities
probe_step_count := #workflow_result.steps
probe_step_1_component := workflow_orchestrator.steps[1].opts.component
probe_step_2_component := workflow_orchestrator.steps[2].opts.component
probe_step_3_component := workflow_orchestrator.steps[3].opts.component
probe_policy_default := approval_policy.default
probe_denied_class := approval_summary.denied_class
probe_trace_kind := approval_summary.trace.kind
probe_trace_status := approval_summary.trace.decision.status
probe_harness_id := evaluation_summary.harness_id
probe_harness_gate_count := evaluation_summary.gate_count
probe_live_model_calls := evaluation_summary.live_model_calls
probe_provider_nil := evaluation_summary.provider == nil
`); err != nil {
				t.Fatalf("Exec probes: %v", err)
			}
			for name, want := range map[string]any{
				"probe_contract_id":        "generic_workflow_orchestration_contract_v1",
				"probe_provider_free":      true,
				"probe_live_network":       false,
				"probe_live_model":         false,
				"probe_domain_specific":    false,
				"probe_capability_count":   int64(4),
				"probe_step_count":         int64(3),
				"probe_step_1_component":   "workflow_orchestrator",
				"probe_step_2_component":   "approval_policy",
				"probe_step_3_component":   "evaluation_harness",
				"probe_policy_default":     "deny_high_risk",
				"probe_denied_class":       "publish",
				"probe_trace_kind":         "approval_replay_trace",
				"probe_trace_status":       "denied",
				"probe_harness_id":         "generic_workflow_evaluation_harness_v1",
				"probe_harness_gate_count": int64(3),
				"probe_live_model_calls":   int64(0),
				"probe_provider_nil":       true,
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

func TestGenericWorkflowOrchestrationEvaluateGate(t *testing.T) {
	root := repoRoot(t)
	reportPath := filepath.Join(t.TempDir(), "generic_workflow_orchestration.report.json")
	cmd := exec.Command(
		"go", "run", "./cmd/leia",
		"evaluate",
		"--gate",
		"--report", reportPath,
		genericWorkflowOrchestrationExample,
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
		report.Summary.Assertions != 15 ||
		report.Summary.PassRate != 1 {
		t.Fatalf("evaluate summary = %#v status=%q", report.Summary, report.Status)
	}
	if report.LLM != nil || report.Summary.LLMTurns != 0 {
		t.Fatalf("evaluate report used LLM provider: llm=%#v summary=%#v", report.LLM, report.Summary)
	}
	if len(report.Findings) != 0 || len(report.Cases) != 1 ||
		report.Cases[0].Name != "generic workflow orchestration composition" ||
		report.Cases[0].Status != "passed" {
		t.Fatalf("evaluate cases/findings = %#v / %#v", report.Cases, report.Findings)
	}
}
