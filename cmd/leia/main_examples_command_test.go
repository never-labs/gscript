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
		"repo-embedding-go-doc-examples",
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

func TestExamplesCommandDiscoversPackageManagedProjectEntrypoints(t *testing.T) {
	root := filepath.Dir(playgroundExamplesRoot())
	examples, err := cliRepositoryExamples()
	if err != nil {
		t.Fatal(err)
	}
	discovered := make(map[string]bool, len(examples))
	for _, example := range examples {
		discovered[example.Path] = true
	}

	var projectRoots []string
	if err := filepath.WalkDir(filepath.Join(root, "examples"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if _, err := os.Stat(filepath.Join(path, "leia.mod")); err != nil {
			return nil
		}
		if _, err := os.Stat(filepath.Join(path, "main.leia")); err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		projectRoots = append(projectRoots, rel)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(projectRoots) == 0 {
		t.Fatal("no package-managed example projects found")
	}
	sort.Strings(projectRoots)
	for _, projectRoot := range projectRoots {
		t.Run(filepath.ToSlash(projectRoot), func(t *testing.T) {
			if _, err := os.Stat(filepath.Join(root, projectRoot, "leia.mod")); err != nil {
				t.Fatalf("project manifest missing: %v", err)
			}
			path := filepath.ToSlash(filepath.Join(projectRoot, "main.leia"))
			if !discovered[path] {
				t.Fatalf("examples CLI must discover package-managed project entrypoint %s", path)
			}
		})
	}
}

func TestExamplesCommandDirectorySelectorsCoverExampleProjects(t *testing.T) {
	root := filepath.Dir(playgroundExamplesRoot())
	entries, err := os.ReadDir(filepath.Join(root, "examples"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.ToSlash(filepath.Join("examples", entry.Name()))
		t.Run(dir, func(t *testing.T) {
			matches, err := selectedCLIExamples([]string{dir})
			if err != nil {
				t.Fatalf("select examples in %s: %v", dir, err)
			}
			if len(matches) == 0 {
				t.Fatalf("directory selector %s matched no examples", dir)
			}
			for _, example := range matches {
				if !strings.HasPrefix(example.Path, dir+"/") {
					t.Fatalf("directory selector %s returned %s", dir, example.Path)
				}
			}
		})
	}
}

func TestExamplesCommandManifestMatchesPlaygroundRepositoryExamples(t *testing.T) {
	playgroundExamples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load playground repository examples: %v", err)
	}
	cliExamples, err := cliRepositoryExamples()
	if err != nil {
		t.Fatalf("load CLI repository examples: %v", err)
	}

	byID := make(map[string]cliExample, len(cliExamples))
	for _, example := range cliExamples {
		if _, exists := byID[example.ID]; exists {
			t.Fatalf("duplicate CLI example id %s", example.ID)
		}
		byID[example.ID] = example
	}

	for _, playground := range playgroundExamples {
		cli, ok := byID[playground.ID]
		if !ok {
			t.Fatalf("CLI examples list is missing playground repository example %s", playground.ID)
		}
		if cli.Path != playground.Summary {
			t.Fatalf("%s CLI path = %q, playground summary = %q", playground.ID, cli.Path, playground.Summary)
		}
		if playground.Runnable && !cli.Runnable {
			t.Fatalf("%s is runnable in the playground manifest but not in the CLI manifest", playground.ID)
		}
		if !playground.Runnable && !cli.Runnable && strings.TrimSpace(cli.Requires) == "" {
			t.Fatalf("%s is manual/check-only in the CLI manifest but has no requires reason", playground.ID)
		}
	}
}

func TestExamplesCommandChecksSubdirectoryProjectSelector(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"check", "--timeout=20s", "database/package_managed"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ok      repo-database-package_managed-main",
		"examples: 1 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples check missing %q\n%s", want, out)
		}
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

func TestExamplesCommandChecksEmbeddingDocExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"check", "repo-embedding-go-doc-examples"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ok      repo-embedding-go-doc-examples",
		"examples: 1 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples check missing %q\n%s", want, out)
		}
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
	code := runExamplesCommand([]string{"run", "repo-llm-glm_smoke"}, &stdout, &stderr)
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
		"ok      repo-llm-agent",
		"examples: 2 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples check missing %q\n%s", want, out)
		}
	}
}

func TestExamplesCommandChecksMockFriendlyLLMExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{
		"check",
		"--jobs=4",
		"--timeout=10s",
		"repo-llm-agent",
		"repo-llm-agent_as_tool",
		"repo-llm-direct_turn",
		"repo-llm-incident_response",
		"repo-llm-manual_tool_history",
		"repo-llm-prompt_tagged_messages",
		"repo-llm-rich_agent_demo",
		"repo-llm-streaming_turn",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ok      repo-llm-agent",
		"ok      repo-llm-agent_as_tool",
		"ok      repo-llm-direct_turn",
		"ok      repo-llm-incident_response",
		"ok      repo-llm-manual_tool_history",
		"ok      repo-llm-prompt_tagged_messages",
		"ok      repo-llm-rich_agent_demo",
		"ok      repo-llm-streaming_turn",
		"examples: 8 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples check missing %q\n%s", want, out)
		}
	}
}

func TestExamplesCommandDefaultCheckSkipsOnlyOptInExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"check", "--json", "--jobs=6", "--timeout=30s"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stderr = %q", code, stderr.String())
	}
	var payload struct {
		SchemaVersion int                     `json:"schema_version"`
		OK            bool                    `json:"ok"`
		Skipped       int                     `json:"skipped"`
		Failed        int                     `json:"failed"`
		Results       []cliExampleCheckResult `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid examples check JSON: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != 1 || !payload.OK || payload.Failed != 0 {
		t.Fatalf("unexpected examples check payload: %#v", payload)
	}
	allowed := map[string]string{
		"repo-game_engine-chess":                "game/window host access",
		"repo-game_engine-chess_ai":             "game/window host access",
		"repo-game_engine-chess_bench":          "higher playground step budget",
		"repo-game_engine-chess_bench_parallel": "higher playground step budget",
		"repo-game_engine-game":                 "game/window host access",
		"repo-game_engine-tetris":               "game/window host access",
		"repo-llm-glm_direct_agent_tools":       "LLM provider",
		"repo-llm-glm_smoke":                    "LLM provider",
	}
	seen := map[string]bool{}
	for _, result := range payload.Results {
		if result.Status != "skipped" {
			continue
		}
		wantReason, ok := allowed[result.ID]
		if !ok {
			t.Fatalf("example %s is unexpectedly skipped: %s", result.ID, result.Requires)
		}
		if result.Requires != wantReason {
			t.Fatalf("example %s skip reason = %q, want %q", result.ID, result.Requires, wantReason)
		}
		seen[result.ID] = true
	}
	if len(seen) != len(allowed) || payload.Skipped != len(allowed) {
		t.Fatalf("skipped examples = %v payload skipped=%d, want exactly %d opt-in examples", seen, payload.Skipped, len(allowed))
	}
}

func TestExamplesCommandChecksDeterministicSpecialRunners(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{
		"check",
		"--jobs=4",
		"repo-evaluate-basic_assert",
		"repo-evaluate-llm_replay",
		"repo-evaluate-agent_replay",
		"repo-evaluate-multiturn_replay",
		"repo-evaluate-project_agent_regression",
		"repo-ai-coding_agent_replay",
		"repo-ai-tagged_agent_workflow",
		"repo-ai-record_replay_trace_project",
		"repo-workflow-support_triage_replay",
		"repo-testing-jsonl_workflow_test",
		"repo-tooling-release_evidence_pipeline",
		"repo-performance-execution_modes_matrix",
		"repo-ui-package_managed-main",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ok      repo-evaluate-basic_assert",
		"ok      repo-evaluate-llm_replay",
		"ok      repo-evaluate-agent_replay",
		"ok      repo-evaluate-multiturn_replay",
		"ok      repo-evaluate-project_agent_regression",
		"ok      repo-ai-coding_agent_replay",
		"ok      repo-ai-tagged_agent_workflow",
		"ok      repo-ai-record_replay_trace_project",
		"ok      repo-workflow-support_triage_replay",
		"ok      repo-testing-jsonl_workflow_test",
		"ok      repo-tooling-release_evidence_pipeline",
		"ok      repo-performance-execution_modes_matrix",
		"ok      repo-ui-package_managed-main",
		"examples: 13 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples check missing %q\n%s", want, out)
		}
	}
}

func TestExamplesCommandRunsEvaluateReplayExample(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"run", "repo-evaluate-agent_replay"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status": "ok"`) {
		t.Fatalf("evaluate replay run output = %q, want JSON ok report", stdout.String())
	}
}

func TestExamplesCommandKeepsPackageManifestExamplesNonRunnable(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"run", "repo-ui-package_managed-main"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("package manifest example unexpectedly ran, stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "manual example") || !strings.Contains(stderr.String(), "package manifest check") {
		t.Fatalf("manual package example error missing explanation: %q", stderr.String())
	}
}

func TestExamplesCommandChecksDeterministicHostExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{
		"check",
		"--jobs=4",
		"--timeout=10s",
		"repo-web-hello_server",
		"repo-web-route_workbench",
		"repo-web-serve_dialect_app",
		"repo-web-tiny_app",
		"repo-web-webserver",
		"repo-concurrency-context_process",
		"repo-concurrency-goroutine_errors",
		"repo-dialects-shell_filesystem",
		"repo-data_processing-data_oriented-particle_integration",
		"repo-game_engine-game_of_life",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ok      repo-web-hello_server",
		"ok      repo-web-route_workbench",
		"ok      repo-web-serve_dialect_app",
		"ok      repo-web-tiny_app",
		"ok      repo-web-webserver",
		"ok      repo-concurrency-context_process",
		"ok      repo-concurrency-goroutine_errors",
		"ok      repo-dialects-shell_filesystem",
		"ok      repo-data_processing-data_oriented-particle_integration",
		"ok      repo-game_engine-game_of_life",
		"examples: 10 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples check missing %q\n%s", want, out)
		}
	}
}

func TestExamplesCommandChecksConcurrencyContractExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{
		"check",
		"--jobs=1",
		"--timeout=10s",
		"repo-concurrency-goroutines_channels",
		"repo-concurrency-select_timeout",
		"repo-concurrency-select_default",
		"repo-concurrency-sync_group",
		"repo-concurrency-context_sleep",
		"repo-concurrency-context_cancel",
		"repo-concurrency-sync_group_cancel",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ok      repo-concurrency-goroutines_channels",
		"ok      repo-concurrency-select_timeout",
		"ok      repo-concurrency-select_default",
		"ok      repo-concurrency-sync_group",
		"ok      repo-concurrency-context_sleep",
		"ok      repo-concurrency-context_cancel",
		"ok      repo-concurrency-sync_group_cancel",
		"examples: 7 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples check missing %q\n%s", want, out)
		}
	}
}

func TestExamplesCommandChecksReadmeCapabilityEvidenceExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{
		"check",
		"--jobs=2",
		"--timeout=20s",
		"repo-embedding-go-doc-examples",
		"repo-llm-agent",
		"repo-ai-coding_agent_replay",
		"repo-evaluate-agent_replay",
		"repo-concurrency-goroutines_channels",
		"repo-concurrency-select_timeout",
		"repo-concurrency-sync_group",
		"repo-data-q_vector_basics",
		"repo-data_processing-data_oriented-soa_kernels",
		"repo-data_processing-data_oriented-dense_matrix_vec_kernels",
		"repo-performance-execution_modes_matrix",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ok      repo-embedding-go-doc-examples",
		"ok      repo-llm-agent",
		"ok      repo-ai-coding_agent_replay",
		"ok      repo-evaluate-agent_replay",
		"ok      repo-concurrency-goroutines_channels",
		"ok      repo-concurrency-select_timeout",
		"ok      repo-concurrency-sync_group",
		"ok      repo-data-q_vector_basics",
		"ok      repo-data_processing-data_oriented-soa_kernels",
		"ok      repo-data_processing-data_oriented-dense_matrix_vec_kernels",
		"ok      repo-performance-execution_modes_matrix",
		"examples: 11 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("README capability examples check missing %q\n%s", want, out)
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
	if payload.SchemaVersion != 1 || !payload.OK || payload.Runnable != 2 || payload.Skipped != 0 || payload.Failed != 0 {
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
