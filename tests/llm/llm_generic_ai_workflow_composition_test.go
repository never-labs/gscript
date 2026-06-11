package leia_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

func TestGenericAIWorkflowCompositionUsesMatrixFixtureIndexes(t *testing.T) {
	root := repoRoot(t)
	data := readGenericAIWorkflowCompositionExample(t)
	matrix := loadGenericAIPackageMatrix(t, root)
	matrixByPackageID := map[string]genericAIPackageRow{}
	for _, row := range matrix.Packages {
		matrixByPackageID[strings.ReplaceAll(row.ID, "_", "-")] = row
	}

	matches := regexp.MustCompile(`package_id:\s*"([^"]+)"`).FindAllStringSubmatch(data, -1)
	if len(matches) != 10 {
		t.Fatalf("composition package boundaries = %d, want 10", len(matches))
	}

	seen := map[string]bool{}
	for _, match := range matches {
		packageID := match[1]
		row, ok := matrixByPackageID[packageID]
		if !ok {
			t.Fatalf("composition package_id %q missing from generic package matrix", packageID)
		}
		seen[row.ID] = true
		wantFixtureIndex := filepath.ToSlash(filepath.Join(row.PackageDir, "fixtures", "provider_free_fixture_index.json"))
		if row.FixtureIndex != wantFixtureIndex {
			t.Fatalf("%s fixture_index = %q, want %q", row.ID, row.FixtureIndex, wantFixtureIndex)
		}

		fixtureIndex := readJSONMap(t, filepath.Join(root, filepath.FromSlash(row.FixtureIndex)))
		if !finrobotLivePackageBoolOrConst(fixtureIndex["provider_free"], true) ||
			!finrobotLivePackageBoolOrConst(fixtureIndex["live_network"], false) ||
			!finrobotLivePackageBoolOrConst(fixtureIndex["real_dependency_imports"], false) {
			t.Fatalf("%s composed fixture index must stay provider-free and offline: %#v", row.ID, fixtureIndex)
		}
	}
	for _, row := range matrix.Packages {
		if !seen[row.ID] {
			t.Fatalf("generic package matrix row %q is not represented in workflow composition", row.ID)
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
