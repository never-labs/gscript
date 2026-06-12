package leia_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinRobotAgentLifecycleParitySourceContract(t *testing.T) {
	root := repoRoot(t)
	sourcePath := filepath.Join(root, filepath.FromSlash(finrobotWorkflowLifecyclePath))
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)

	for _, want := range []string{
		"reset_lifecycle",
		"cache_seed: \"finrobot-lifecycle-seed-001\"",
		"seed_cache",
		"stopped_by_max_round",
		"handoff_dag",
		"nested_summary_ledger",
		"reflection_summary_method",
		"reflection-disabled-placeholder",
		"tool_call_retry_envelope",
		"human_input_mode_metadata",
		"human_input_mode: \"NEVER\"",
		"provider_free: true",
		"live_network: false",
		"credentials_required: false",
		"replay_mode: \"local-fixture\"",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("workflow lifecycle source missing parity marker %q", want)
		}
	}

	for _, forbidden := range []string{
		"OPENAI_API_KEY",
		"ANTHROPIC_API_KEY",
		"GOOGLE_API_KEY",
		"live_network: true",
		"credentials_required: true",
		"q/runtime",
		"live_packages",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("workflow lifecycle source contains forbidden marker %q", forbidden)
		}
	}
}

func TestFinRobotAgentLifecycleParityExampleCheck(t *testing.T) {
	root := repoRoot(t)
	report := finrobotAgentLifecycleExamplesCheck(t, root)
	if report.SchemaVersion != 1 || !report.OK || report.Runnable != 1 || report.Skipped != 0 || report.Failed != 0 {
		t.Fatalf("agent lifecycle check summary = schema:%d ok:%t runnable:%d skipped:%d failed:%d",
			report.SchemaVersion, report.OK, report.Runnable, report.Skipped, report.Failed)
	}
	if len(report.Results) != 1 {
		t.Fatalf("agent lifecycle check results = %d, want 1", len(report.Results))
	}
	result := report.Results[0]
	if result.ID != "repo-ai-finrobot_translation-core_agents-workflow_lifecycle" ||
		result.Path != finrobotWorkflowLifecyclePath ||
		result.Status != "ok" ||
		result.Requires != "" ||
		result.Error != "" {
		t.Fatalf("agent lifecycle check result = %#v", result)
	}
}

func TestFinRobotAgentLifecycleParityRegistryIsReplayDriven(t *testing.T) {
	root := repoRoot(t)
	example := findFinRobotWorkflowLifecycleExample(t, root)
	if !example.Runnable || !example.Checkable {
		t.Fatalf("agent lifecycle example must be runnable/checkable: %#v", example)
	}
	if example.Runner != "evaluate" || example.Requires != "" {
		t.Fatalf("agent lifecycle example runner/requires = %q/%q, want evaluate with no provider requirement", example.Runner, example.Requires)
	}
}

func finrobotAgentLifecycleExamplesCheck(t *testing.T, root string) struct {
	SchemaVersion int  `json:"schema_version"`
	OK            bool `json:"ok"`
	Runnable      int  `json:"runnable"`
	Skipped       int  `json:"skipped"`
	Failed        int  `json:"failed"`
	Results       []struct {
		ID       string `json:"id"`
		Path     string `json:"path"`
		Status   string `json:"status"`
		Requires string `json:"requires"`
		Error    string `json:"error"`
	} `json:"results"`
} {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(
		"go", "run", "./cmd/leia",
		"examples", "check",
		"--json",
		"--timeout=30s",
		finrobotWorkflowLifecyclePath,
	)
	cmd.Dir = root
	cmd.Env = withoutLLMProviderEnv(os.Environ())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("examples check failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	var payload struct {
		SchemaVersion int  `json:"schema_version"`
		OK            bool `json:"ok"`
		Runnable      int  `json:"runnable"`
		Skipped       int  `json:"skipped"`
		Failed        int  `json:"failed"`
		Results       []struct {
			ID       string `json:"id"`
			Path     string `json:"path"`
			Status   string `json:"status"`
			Requires string `json:"requires"`
			Error    string `json:"error"`
		} `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode examples check: %v\n%s", err, stdout.String())
	}
	return payload
}
