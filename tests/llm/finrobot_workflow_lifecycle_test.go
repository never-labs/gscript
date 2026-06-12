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

const finrobotWorkflowLifecyclePath = "examples/ai/finrobot_translation/core_agents/workflow_lifecycle.leia"

func TestFinRobotWorkflowLifecycleExampleRegisteredAndCheckable(t *testing.T) {
	root := repoRoot(t)
	example := findFinRobotWorkflowLifecycleExample(t, root)
	if example.ID != "repo-ai-finrobot_translation-core_agents-workflow_lifecycle" {
		t.Fatalf("workflow lifecycle id = %q", example.ID)
	}
	if example.Path != finrobotWorkflowLifecyclePath {
		t.Fatalf("workflow lifecycle path = %q", example.Path)
	}
	if !example.Runnable || !example.Checkable {
		t.Fatalf("workflow lifecycle must be provider-free runnable/checkable: %#v", example)
	}
	if example.Runner != "evaluate" {
		t.Fatalf("workflow lifecycle runner = %q, want evaluate", example.Runner)
	}
	if example.Requires != "" {
		t.Fatalf("workflow lifecycle unexpectedly requires %q", example.Requires)
	}
}

func TestFinRobotWorkflowLifecycleProviderFreeParity(t *testing.T) {
	root := repoRoot(t)
	sourcePath := filepath.Join(root, filepath.FromSlash(finrobotWorkflowLifecyclePath))
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)

	for _, want := range []string{
		"SingleAssistant",
		"SingleAssistantShadow",
		"MultiAssistantWithLeader",
		"reset_lifecycle",
		"cache_lookup",
		"trigger",
		"TERMINATE",
		"nested_chat_summary",
		"handoff_trace_type",
		"max_round",
		"provider_free_replay",
		"live_network: false",
		"credentials_required: false",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("workflow lifecycle source missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"OPENAI_API_KEY",
		"ANTHROPIC_API_KEY",
		"GOOGLE_API_KEY",
		"http://",
		"https://",
		"live_network: true",
		"credentials_required: true",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("workflow lifecycle source contains forbidden live/provider marker %q", forbidden)
		}
	}

	report := finrobotWorkflowLifecycleExamplesCheck(t, root)
	if report.SchemaVersion != 1 || !report.OK || report.Runnable != 1 || report.Skipped != 0 || report.Failed != 0 {
		t.Fatalf("workflow lifecycle check summary = schema:%d ok:%t runnable:%d skipped:%d failed:%d",
			report.SchemaVersion, report.OK, report.Runnable, report.Skipped, report.Failed)
	}
	if len(report.Results) != 1 {
		t.Fatalf("workflow lifecycle check results = %d, want 1", len(report.Results))
	}
	result := report.Results[0]
	if result.ID != "repo-ai-finrobot_translation-core_agents-workflow_lifecycle" ||
		result.Path != finrobotWorkflowLifecyclePath ||
		result.Status != "ok" ||
		result.Requires != "" ||
		result.Error != "" {
		t.Fatalf("workflow lifecycle check result = %#v", result)
	}
}

func findFinRobotWorkflowLifecycleExample(t *testing.T, root string) finrobotAggregateExample {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", "run", "./cmd/leia", "examples", "list", "--json")
	cmd.Dir = root
	cmd.Env = withoutLLMProviderEnv(os.Environ())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("examples list failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	var payload struct {
		SchemaVersion int                        `json:"schema_version"`
		Examples      []finrobotAggregateExample `json:"examples"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode examples list: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != 1 {
		t.Fatalf("examples list schema_version = %d, want 1", payload.SchemaVersion)
	}
	for _, example := range payload.Examples {
		if example.Path == finrobotWorkflowLifecyclePath {
			return example
		}
	}
	t.Fatalf("examples list missing %s", finrobotWorkflowLifecyclePath)
	return finrobotAggregateExample{}
}

func finrobotWorkflowLifecycleExamplesCheck(t *testing.T, root string) struct {
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
