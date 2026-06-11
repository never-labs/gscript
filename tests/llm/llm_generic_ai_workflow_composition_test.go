package leia_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericAIWorkflowCompositionExampleExecutes(t *testing.T) {
	vm := leia.New(leia.WithLibs(leia.LibString))
	path := genericAIWorkflowCompositionExamplePath(t)
	if err := vm.ExecFile(path); err != nil {
		t.Fatalf("ExecFile: %v", err)
	}

	summary, err := vm.Get("composition_summary")
	if err != nil {
		t.Fatalf("composition_summary: %v", err)
	}
	if summary != "generic-ai-workflow-composition boundaries=10 edges=9 provider_free=true" {
		t.Fatalf("composition_summary = %#v", summary)
	}
	count, err := vm.Get("package_boundary_count")
	if err != nil {
		t.Fatalf("package_boundary_count: %v", err)
	}
	if count != int64(10) {
		t.Fatalf("package_boundary_count = %#v, want 10", count)
	}
}

func TestGenericAIWorkflowCompositionCoversGenericPackageBoundaries(t *testing.T) {
	data := readGenericAIWorkflowCompositionExample(t)
	for _, want := range []string{
		`role: "model"`,
		`role: "turn"`,
		`role: "tool"`,
		`role: "agent"`,
		`role: "workflow"`,
		`role: "eval"`,
		`role: "replay"`,
		`role: "trace"`,
		`role: "approval"`,
		`role: "package-audit"`,
		`package_id: "generic-model-registry"`,
		`package_id: "generic-turn-runner"`,
		`package_id: "generic-tool-contracts"`,
		`package_id: "generic-agent-runner"`,
		`package_id: "generic-workflow-orchestrator"`,
		`package_id: "generic-evaluation-harness"`,
		`package_id: "generic-record-replay"`,
		`package_id: "generic-trace-events"`,
		`package_id: "generic-approval-policy"`,
		`package_id: "generic-package-boundary-auditor"`,
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("composition example missing %q", want)
		}
	}
	for _, forbidden := range []string{"openai", "anthropic", "gemini", "finrobot", "yfinance", "finnhub"} {
		if strings.Contains(strings.ToLower(data), forbidden) {
			t.Fatalf("composition example must remain provider-free and domain-neutral; found %q", forbidden)
		}
	}
}

func TestGenericAIWorkflowCompositionExampleIsDiscoverableAndCheckable(t *testing.T) {
	root := repoRoot(t)
	examplePath := "examples/ai/finrobot_translation/generic_ai_workflow_composition.leia"
	cmd := exec.Command("go", "run", "./cmd/leia", "examples", "check", "--json", examplePath)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("examples check failed: %v\n%s", err, string(output))
	}

	var report struct {
		SchemaVersion int `json:"schema_version"`
		OK            bool
		Runnable      int
		Skipped       int
		Failed        int
		Results       []struct {
			ID     string `json:"id"`
			Path   string `json:"path"`
			Status string `json:"status"`
		} `json:"results"`
	}
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("decode examples check: %v\n%s", err, string(output))
	}
	if report.SchemaVersion != 1 || !report.OK || report.Runnable != 1 || report.Skipped != 0 || report.Failed != 0 || len(report.Results) != 1 {
		t.Fatalf("unexpected examples check report: %#v", report)
	}
	result := report.Results[0]
	if result.ID != "repo-ai-finrobot_translation-generic_ai_workflow_composition" || result.Path != examplePath || result.Status != "ok" {
		t.Fatalf("unexpected examples check result: %#v", result)
	}
}

func genericAIWorkflowCompositionExamplePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "generic_ai_workflow_composition.leia")
}

func readGenericAIWorkflowCompositionExample(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(genericAIWorkflowCompositionExamplePath(t))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
