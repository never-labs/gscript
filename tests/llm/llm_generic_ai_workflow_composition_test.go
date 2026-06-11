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

	boundaries := parseGenericAIWorkflowCompositionBoundaries(t, data)
	if len(boundaries) != 10 {
		t.Fatalf("composition package boundaries = %d, want 10", len(boundaries))
	}

	roles := map[string]genericAIWorkflowCompositionBoundary{}
	seen := map[string]bool{}
	for _, boundary := range boundaries {
		if boundary.Role == "" || boundary.PackageID == "" || boundary.Capability == "" || boundary.Entrypoint == "" {
			t.Fatalf("composition boundary has missing resolvable fields: %#v", boundary)
		}
		if _, ok := roles[boundary.Role]; ok {
			t.Fatalf("duplicate composition role %q", boundary.Role)
		}
		roles[boundary.Role] = boundary

		row, ok := matrixByPackageID[boundary.PackageID]
		if !ok {
			t.Fatalf("composition package_id %q missing from generic package matrix", boundary.PackageID)
		}
		if !genericLivePackageContains(row.Capabilities, boundary.Capability) {
			t.Fatalf("%s composition capability %q does not resolve in matrix capabilities %#v", boundary.Role, boundary.Capability, row.Capabilities)
		}
		if boundary.Entrypoint != row.BackendShape {
			t.Fatalf("%s composition entrypoint = %q, want matrix backend_shape %q", boundary.Role, boundary.Entrypoint, row.BackendShape)
		}
		manifest := readJSONMap(t, filepath.Join(root, filepath.FromSlash(row.Manifest)))
		if packageName, _ := manifest["package_name"].(string); packageName != row.PackageName {
			t.Fatalf("%s manifest package_name = %q, want matrix package_name %q", row.ID, packageName, row.PackageName)
		}
		if !genericAIWorkflowCompositionValueContainsCapability(manifest["capabilities"], boundary.Capability) {
			t.Fatalf("%s manifest capabilities do not expose composition capability %q", row.ID, boundary.Capability)
		}
		if !genericAIWorkflowCompositionManifestEntrypointsResolveFixtureIndex(manifest, row.FixtureIndex) {
			t.Fatalf("%s manifest entrypoints do not resolve matrix fixture index %q", row.ID, row.FixtureIndex)
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
		assertGenericAIWorkflowCompositionFixtureIndexPaths(t, root, row, fixtureIndex)
	}
	for _, row := range matrix.Packages {
		if !seen[row.ID] {
			t.Fatalf("generic package matrix row %q is not represented in workflow composition", row.ID)
		}
	}

	edges := parseGenericAIWorkflowCompositionEdges(t, data)
	if len(edges) != 9 {
		t.Fatalf("composition edges = %d, want 9", len(edges))
	}
	seenEdges := map[string]bool{}
	for _, edge := range edges {
		if _, ok := roles[edge.From]; !ok {
			t.Fatalf("composition edge from %q does not resolve to a boundary", edge.From)
		}
		if _, ok := roles[edge.To]; !ok {
			t.Fatalf("composition edge to %q does not resolve to a boundary", edge.To)
		}
		seenEdges[edge.From+"->"+edge.To] = true
	}
	for _, want := range []string{
		"approval->tool",
		"tool->turn",
		"replay->turn",
		"trace->workflow",
		"agent->workflow",
		"workflow->eval",
	} {
		if !seenEdges[want] {
			t.Fatalf("composition missing required edge %s", want)
		}
	}
	assertGenericAIWorkflowCompositionBoundaryOrder(t, boundaries, "approval", "tool", "turn", "agent", "workflow", "eval")
	assertGenericAIWorkflowCompositionBoundaryOrder(t, boundaries, "replay", "trace", "approval", "package-audit")
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

type genericAIWorkflowCompositionBoundary struct {
	Role       string
	PackageID  string
	Capability string
	Entrypoint string
}

type genericAIWorkflowCompositionEdge struct {
	From string
	To   string
}

func parseGenericAIWorkflowCompositionBoundaries(t *testing.T, data string) []genericAIWorkflowCompositionBoundary {
	t.Helper()
	matches := regexp.MustCompile(`\{role:\s*"([^"]+)"\s+package_id:\s*"([^"]+)"\s+capability:\s*"([^"]+)"\s+entrypoint:\s*"([^"]+)"`).FindAllStringSubmatch(data, -1)
	boundaries := make([]genericAIWorkflowCompositionBoundary, 0, len(matches))
	for _, match := range matches {
		boundaries = append(boundaries, genericAIWorkflowCompositionBoundary{
			Role:       match[1],
			PackageID:  match[2],
			Capability: match[3],
			Entrypoint: match[4],
		})
	}
	return boundaries
}

func parseGenericAIWorkflowCompositionEdges(t *testing.T, data string) []genericAIWorkflowCompositionEdge {
	t.Helper()
	matches := regexp.MustCompile(`\{from:\s*"([^"]+)"\s+to:\s*"([^"]+)"`).FindAllStringSubmatch(data, -1)
	edges := make([]genericAIWorkflowCompositionEdge, 0, len(matches))
	for _, match := range matches {
		edges = append(edges, genericAIWorkflowCompositionEdge{From: match[1], To: match[2]})
	}
	return edges
}

func genericAIWorkflowCompositionValueContainsCapability(value any, capability string) bool {
	switch value := value.(type) {
	case string:
		return value == capability
	case []any:
		for _, item := range value {
			if genericAIWorkflowCompositionValueContainsCapability(item, capability) {
				return true
			}
		}
	case map[string]any:
		for key, item := range value {
			if (key == "capability" || key == "capability_id" || key == "dialect_export") && genericAIWorkflowCompositionValueContainsCapability(item, capability) {
				return true
			}
			if key == "capabilities" || key == "capability_tags" {
				if genericAIWorkflowCompositionValueContainsCapability(item, capability) {
					return true
				}
			}
		}
	}
	return false
}

func genericAIWorkflowCompositionManifestEntrypointsResolveFixtureIndex(manifest map[string]any, fixtureIndex string) bool {
	entrypoints, ok := manifest["entrypoints"].(map[string]any)
	if !ok {
		return false
	}
	for name, value := range entrypoints {
		rel, ok := value.(string)
		if !ok {
			continue
		}
		if name == "fixture_index" && filepath.ToSlash(filepath.Join(filepath.Dir(fixtureIndex), "..", rel)) == fixtureIndex {
			return true
		}
		if rel == "fixtures/provider_free_fixture_index.json" {
			return true
		}
	}
	return false
}

func assertGenericAIWorkflowCompositionFixtureIndexPaths(t *testing.T, root string, row genericAIPackageRow, index map[string]any) {
	t.Helper()
	fixtures, ok := index["fixtures"].([]any)
	if !ok || len(fixtures) == 0 {
		t.Fatalf("%s fixture index must declare fixtures", row.ID)
	}
	for _, item := range fixtures {
		fixture, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("%s fixture index item = %#v, want object", row.ID, item)
		}
		if !finrobotLivePackageBoolOrConst(fixture["provider_free"], true) ||
			!finrobotLivePackageBoolOrConst(fixture["live_network"], false) ||
			!finrobotLivePackageBoolOrConst(fixture["real_dependency_imports"], false) {
			t.Fatalf("%s fixture index item must stay provider-free and offline: %#v", row.ID, fixture)
		}
		for _, key := range []string{"path", "schema"} {
			rel, ok := fixture[key].(string)
			if !ok || rel == "" {
				continue
			}
			rel = strings.Split(rel, "#")[0]
			if rel == "" {
				continue
			}
			if key == "schema" && !strings.Contains(rel, "/") && !strings.HasSuffix(rel, ".json") {
				continue
			}
			path := filepath.Join(root, filepath.FromSlash(row.PackageDir), filepath.FromSlash(rel))
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("%s fixture index %s %q does not resolve: %v", row.ID, key, rel, err)
			}
		}
	}
}

func assertGenericAIWorkflowCompositionBoundaryOrder(t *testing.T, boundaries []genericAIWorkflowCompositionBoundary, roles ...string) {
	t.Helper()
	positions := map[string]int{}
	for i, boundary := range boundaries {
		positions[boundary.Role] = i
	}
	for i := 1; i < len(roles); i++ {
		before, beforeOK := positions[roles[i-1]]
		after, afterOK := positions[roles[i]]
		if !beforeOK || !afterOK {
			t.Fatalf("cannot compare boundary order for %q before %q: positions=%#v", roles[i-1], roles[i], positions)
		}
		if before >= after {
			t.Fatalf("boundary order must keep %s before %s; positions=%#v", roles[i-1], roles[i], positions)
		}
	}
}
