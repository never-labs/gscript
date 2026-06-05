package main

import (
	"bytes"
	"encoding/json"
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
