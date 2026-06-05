package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestExamplesCommandListsRepositoryExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"repo-hello-counter",
		"repo-site-static_docs_generator",
		"repo-security-supply_chain_audit",
		"repo-security-vendor_onboarding_audit",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples list missing %q\n%s", want, out)
		}
	}
}

func TestExamplesCommandListsJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stderr = %q", code, stderr.String())
	}
	var payload struct {
		SchemaVersion int          `json:"schema_version"`
		Examples      []cliExample `json:"examples"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid examples JSON: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", payload.SchemaVersion)
	}
	if len(payload.Examples) == 0 {
		t.Fatal("examples JSON is empty")
	}
}

func TestExamplesCommandDiscoversDialectExamples(t *testing.T) {
	root := filepath.Dir(playgroundExamplesRoot())
	dialectDir := filepath.Join(root, "examples", "dialects")
	entries, err := os.ReadDir(dialectDir)
	if err != nil {
		t.Fatal(err)
	}

	examples, err := cliRepositoryExamples()
	if err != nil {
		t.Fatal(err)
	}
	discovered := make(map[string]bool, len(examples))
	for _, example := range examples {
		discovered[example.Path] = true
	}

	var missing []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".leia" {
			continue
		}
		path := filepath.ToSlash(filepath.Join("examples", "dialects", entry.Name()))
		if !discovered[path] {
			missing = append(missing, path)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("examples CLI must discover runnable dialect gate inputs; missing %s", strings.Join(missing, ", "))
	}
}

func TestExamplesCommandShowAcceptsIDAndPath(t *testing.T) {
	for _, selector := range []string{"repo-hello-counter", "examples/hello/counter.leia", "hello/counter.leia"} {
		t.Run(selector, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runExamplesCommand([]string{"show", selector}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("runExamplesCommand code = %d, stderr = %q", code, stderr.String())
			}
			out := stdout.String()
			if !strings.Contains(out, "id: repo-hello-counter") || !strings.Contains(out, "makeCounter") {
				t.Fatalf("unexpected show output for %s:\n%s", selector, out)
			}
		})
	}
}

func TestExamplesCommandRunsRunnableExample(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"run", "repo-hello-counter"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
}

func TestExamplesCommandRefusesManualExample(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"run", "repo-llm-agent"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("manual example unexpectedly ran, stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "manual example") || !strings.Contains(stderr.String(), "LLM provider") {
		t.Fatalf("manual example error missing explanation: %q", stderr.String())
	}
}

func TestExamplesCommandChecksSelectedExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"check", "--jobs=2", "repo-hello-counter", "repo-llm-agent"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ok      repo-hello-counter",
		"skip    repo-llm-agent",
		"examples: 1 ok, 1 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples check missing %q\n%s", want, out)
		}
	}
}

func TestExamplesCommandChecksSelectedExamplesJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"check", "--json", "repo-hello-counter", "repo-llm-agent"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stderr = %q", code, stderr.String())
	}
	var payload struct {
		SchemaVersion int                     `json:"schema_version"`
		OK            bool                    `json:"ok"`
		Runnable      int                     `json:"runnable"`
		Skipped       int                     `json:"skipped"`
		Failed        int                     `json:"failed"`
		Results       []cliExampleCheckResult `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid examples check JSON: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != 1 || !payload.OK || payload.Runnable != 1 || payload.Skipped != 1 || payload.Failed != 0 {
		t.Fatalf("unexpected examples check payload: %#v", payload)
	}
}

func TestExamplesCommandCheckRejectsInvalidTimeout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"check", "--timeout=0s", "repo-hello-counter"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runExamplesCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--timeout must be positive") {
		t.Fatalf("stderr = %q, want timeout validation", stderr.String())
	}
}
