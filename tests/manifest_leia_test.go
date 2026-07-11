package tests_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLeiaManifestScriptCheckPasses(t *testing.T) {
	root := findRepoRoot(t)
	runCommand(t, root, 60*time.Second, "scripts/run.sh", "manifest-check", "tests", "benchmarks")
}

func TestLeiaManifestScriptListEmitsJSONLines(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "run", "scripts/manifest.leia", "list", "tests")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatal("manifest list tests emitted no JSON lines")
	}
	var first struct {
		ID        string          `json:"id"`
		Path      string          `json:"path"`
		Domain    string          `json:"domain"`
		Kind      string          `json:"kind"`
		Reference json.RawMessage `json:"reference"`
		Status    string          `json:"status"`
		Tags      []string        `json:"tags"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("first manifest list line is not JSON: %v\n%s", err, lines[0])
	}
	if first.ID == "" || first.Path == "" || first.Domain == "" || first.Kind != "test" || first.Status != "passing" || len(first.Tags) == 0 {
		t.Fatalf("first manifest list row = %+v, want complete test metadata", first)
	}
	if len(first.Reference) == 0 {
		t.Fatalf("first manifest list row missing explicit reference field: %s", lines[0])
	}
}
