package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPlaygroundRunAPI(t *testing.T) {
	handler := newPlaygroundHandler(playgroundOptions{
		Timeout:        time.Second,
		MaxSourceBytes: 1024,
		MaxSteps:       1000,
		Runner: func(ctx context.Context, req playgroundRunRequest, opts playgroundOptions) playgroundRunResponse {
			if req.Mode != "bytecode" {
				t.Fatalf("mode = %q, want bytecode", req.Mode)
			}
			if req.Source != `print("hi")` {
				t.Fatalf("source = %q", req.Source)
			}
			return playgroundRunResponse{OK: true, Stdout: "hi\n", DurationMS: 2}
		},
	})
	body := strings.NewReader(`{"source":"print(\"hi\")","mode":"vm"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/run", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp playgroundRunResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK || resp.Stdout != "hi\n" || resp.DurationMS != 2 {
		t.Fatalf("response = %#v", resp)
	}
}

func TestPlaygroundRunAPIRejectsOversizedSource(t *testing.T) {
	handler := newPlaygroundHandler(playgroundOptions{
		Timeout:        time.Second,
		MaxSourceBytes: 8,
		MaxSteps:       1000,
		Runner: func(context.Context, playgroundRunRequest, playgroundOptions) playgroundRunResponse {
			t.Fatal("runner should not be called")
			return playgroundRunResponse{}
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/run", strings.NewReader(`{"source":"123456789","mode":"interpreter"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestPlaygroundExamplesAPI(t *testing.T) {
	handler := newPlaygroundHandler(playgroundOptions{
		Runner: func(context.Context, playgroundRunRequest, playgroundOptions) playgroundRunResponse {
			return playgroundRunResponse{}
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/examples", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var examples []playgroundExample
	if err := json.Unmarshal(rec.Body.Bytes(), &examples); err != nil {
		t.Fatalf("decode examples: %v", err)
	}
	if len(examples) < 3 {
		t.Fatalf("examples = %d, want at least 3", len(examples))
	}
	for _, example := range examples {
		if example.ID == "" || example.Title == "" || example.Section == "" || example.Summary == "" {
			t.Fatalf("incomplete tour example: %#v", example)
		}
	}
}

func TestPlaygroundPageSyntaxSurfaceMatchesLeia(t *testing.T) {
	handler := newPlaygroundHandler(playgroundOptions{
		Runner: func(context.Context, playgroundRunRequest, playgroundOptions) playgroundRunResponse {
			return playgroundRunResponse{}
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"import"`,
		`"select"`,
		`"model"`,
		`"select"`,
		`ch === "/" && next === "/"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("playground page missing %q", want)
		}
	}
	if strings.Contains(body, `"while"`) {
		t.Fatalf("playground page still advertises unsupported while keyword")
	}
}

func TestPlaygroundTourAndAIAPI(t *testing.T) {
	handler := newPlaygroundHandler(playgroundOptions{
		Runner: func(context.Context, playgroundRunRequest, playgroundOptions) playgroundRunResponse {
			return playgroundRunResponse{}
		},
	})
	for _, path := range []string{"/api/tour", "/api/ai"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			var examples []playgroundExample
			if err := json.Unmarshal(rec.Body.Bytes(), &examples); err != nil {
				t.Fatalf("decode examples: %v", err)
			}
			if len(examples) < 3 {
				t.Fatalf("examples = %d, want at least 3", len(examples))
			}
			for _, example := range examples {
				if example.ID == "" || example.Title == "" || example.Section == "" || example.Summary == "" {
					t.Fatalf("incomplete example: %#v", example)
				}
			}
		})
	}
}

func TestPlaygroundExamplesExecute(t *testing.T) {
	t.Setenv("LEIA_PLAYGROUND_MOCK_LLM", "1")
	var examples []playgroundExample
	examples = append(examples, playgroundTourLessons()...)
	examples = append(examples, playgroundAIExamples()...)
	for _, example := range examples {
		if !example.Runnable {
			continue
		}
		if strings.Contains(strings.ToLower(example.Requires), "credential") {
			continue
		}
		t.Run(example.ID, func(t *testing.T) {
			dir := t.TempDir()
			path := dir + "/main.leia"
			if err := os.WriteFile(path, []byte(example.Source), 0600); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			args := []string{"--max-steps=2000000", path}
			if strings.HasPrefix(example.ID, "ai-") {
				args = []string{"--profile=ai", "--max-steps=2000000", path}
			}
			code := runPlaygroundExecCommand(args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("example failed with code %d\nstdout:\n%s\nstderr:\n%s\nsource:\n%s", code, stdout.String(), stderr.String(), example.Source)
			}
			if strings.TrimSpace(stdout.String()) == "" {
				t.Fatalf("example produced no stdout\nsource:\n%s", example.Source)
			}
		})
	}
}

func TestPlaygroundRepositoryExamplesExecuteOrExplain(t *testing.T) {
	examples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load repository examples: %v", err)
	}
	if len(examples) == 0 {
		t.Fatal("repository examples are empty")
	}
	for _, example := range examples {
		t.Run(example.ID, func(t *testing.T) {
			if strings.TrimSpace(example.Source) == "" {
				t.Fatalf("source is empty")
			}
			if !example.Runnable {
				if strings.TrimSpace(example.Requires) == "" {
					t.Fatalf("non-runnable example must explain requires: %#v", example)
				}
				return
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "main.leia")
			if err := os.WriteFile(path, []byte(example.Source), 0600); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			code := runPlaygroundExecCommand([]string{"--max-steps=2000000", path}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("repository example failed with code %d\nstdout:\n%s\nstderr:\n%s\nsource:\n%s", code, stdout.String(), stderr.String(), example.Source)
			}
			if strings.TrimSpace(stdout.String()) == "" {
				t.Fatalf("repository example produced no stdout\nsource:\n%s", example.Source)
			}
		})
	}
}

func TestPlaygroundRepositoryCoreExampleCoverage(t *testing.T) {
	root := playgroundExamplesRoot()
	examples, err := playgroundRepositoryExamples(root)
	if err != nil {
		t.Fatalf("load repository examples: %v", err)
	}
	byID := make(map[string]playgroundExample, len(examples))
	for _, example := range examples {
		byID[example.ID] = example
	}

	manualRequires := map[string]string{
		"repo-concurrency-context_process":                        "process host access",
		"repo-concurrency-goroutine_errors":                       "debug event sink host access",
		"repo-data_processing-data_oriented-particle_integration": "higher playground step budget",
		"repo-dialects-shell_filesystem":                          "process shell and filesystem host access",
	}
	for _, dir := range []string{"hello", "concurrency", "data_processing", "dialects"} {
		dir := dir
		t.Run(dir, func(t *testing.T) {
			err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || filepath.Ext(path) != ".leia" {
					return nil
				}
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				id := "repo-" + strings.TrimSuffix(filepath.ToSlash(rel), ".leia")
				id = strings.ReplaceAll(id, "/", "-")
				example, ok := byID[id]
				if !ok {
					t.Fatalf("%s is not exposed through playground repository examples", filepath.ToSlash(rel))
				}
				if requires, manual := manualRequires[id]; manual {
					if example.Runnable {
						t.Fatalf("%s should be manual-run only", id)
					}
					if example.Requires != requires {
						t.Fatalf("%s requires = %q, want %q", id, example.Requires, requires)
					}
					return nil
				}
				if !example.Runnable {
					t.Fatalf("%s should be runnable, requires = %q", id, example.Requires)
				}
				if strings.TrimSpace(example.Requires) != "" {
					t.Fatalf("%s runnable example should not require manual setup: %q", id, example.Requires)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPlaygroundRepositoryHostCapabilityExampleIsManualRunOnly(t *testing.T) {
	examples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load repository examples: %v", err)
	}
	for _, example := range examples {
		if example.ID != "repo-dialects-shell_filesystem" {
			continue
		}
		if example.Runnable {
			t.Fatalf("%s should be manual-run only in the playground", example.ID)
		}
		if example.Requires != "process shell and filesystem host access" {
			t.Fatalf("requires = %q", example.Requires)
		}
		for _, want := range []string{"$`printf", "$!`printf", "cmd`printf", `dialect.eval("glob"`, "path.join(entry.dir", "path`examples/dialects/../dialects/./shell_filesystem.leia`"} {
			if !strings.Contains(example.Source, want) {
				t.Fatalf("source missing %q\nsource:\n%s", want, example.Source)
			}
		}
		return
	}
	t.Fatal("repo-dialects-shell_filesystem example not found")
}

func TestPlaygroundRepositoryPackageManagedUIExampleIsManualRunOnly(t *testing.T) {
	examples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load repository examples: %v", err)
	}
	for _, example := range examples {
		if example.ID != "repo-ui-package_managed-main" {
			continue
		}
		if example.Runnable {
			t.Fatalf("%s should be manual-run only in the playground", example.ID)
		}
		if example.Requires != "package-managed UI runtime and native window host access" {
			t.Fatalf("requires = %q", example.Requires)
		}
		if !strings.Contains(example.Source, `import "github.com/never-labs/leia-ui/raylib" as ui`) {
			t.Fatalf("source missing package-managed UI runtime import\nsource:\n%s", example.Source)
		}
		return
	}
	t.Fatal("repo-ui-package_managed-main example not found")
}

func TestPlaygroundRepositoryPackageManagedDatabaseExampleIsManualRunOnly(t *testing.T) {
	examples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load repository examples: %v", err)
	}
	for _, example := range examples {
		if example.ID != "repo-database-package_managed-main" {
			continue
		}
		if example.Runnable {
			t.Fatalf("%s should be manual-run only in the playground", example.ID)
		}
		if example.Requires != "package-managed database runtime and native SQL driver" {
			t.Fatalf("requires = %q", example.Requires)
		}
		if !strings.Contains(example.Source, `import "github.com/never-labs/leia-db/sqlite" as sqlite`) {
			t.Fatalf("source missing package-managed database runtime import\nsource:\n%s", example.Source)
		}
		return
	}
	t.Fatal("repo-database-package_managed-main example not found")
}

func TestPlaygroundRepositoryWorkflowReplayExampleIsManualRunOnly(t *testing.T) {
	examples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load repository examples: %v", err)
	}
	for _, example := range examples {
		if example.ID != "repo-workflow-support_triage_replay" {
			continue
		}
		if example.Runnable {
			t.Fatalf("%s should be manual-run only in the playground", example.ID)
		}
		if example.Requires != "LLM replay fixture or provider" {
			t.Fatalf("requires = %q", example.Requires)
		}
		if !strings.Contains(example.Source, `model: "mock-fast"`) {
			t.Fatalf("source missing replay-backed mock model\nsource:\n%s", example.Source)
		}
		return
	}
	t.Fatal("repo-workflow-support_triage_replay example not found")
}

func TestPlaygroundAIExamplesCoverRunnableWorkflowShapes(t *testing.T) {
	t.Setenv("LEIA_PLAYGROUND_MOCK_LLM", "1")
	want := map[string][]string{
		"ai-one-line":          {"LEIA_GLM_OK"},
		"ai-agent-shape":       {"MOCK_AI_OK"},
		"ai-structured-output": {"\"product\":\"playground\""},
		"ai-streaming":         {"streamed\tLEIA_GLM_OK", "final\tLEIA_GLM_OK"},
		"ai-tool":              {"MOCK_TOOL_RESULT"},
		"ai-memory":            {"project=ORCHID"},
		"ai-agent-tool":        {"MOCK_TOOL_RESULT"},
		"ai-support-triage":    {"MOCK_TOOL_RESULT"},
		"ai-draft-review":      {"draft:", "review:", "PASS"},
		"ai-coding-agent":      {"attempts", "2 slugify cases passed", "tools\tsearch_repo,read_file,read_docs,run_tests,propose_file", "func slugify"},
		"ai-self-check-loop":   {"local classifier checks passed"},
	}
	for _, example := range playgroundAIExamples() {
		t.Run(example.ID, func(t *testing.T) {
			if !example.Runnable {
				t.Fatalf("%s should be runnable under the local mock AI profile", example.ID)
			}
			dir := t.TempDir()
			path := dir + "/main.leia"
			if err := os.WriteFile(path, []byte(example.Source), 0600); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			code := runPlaygroundExecCommand([]string{"--profile=ai", "--max-steps=2000000", path}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("example failed with code %d\nstdout:\n%s\nstderr:\n%s\nsource:\n%s", code, stdout.String(), stderr.String(), example.Source)
			}
			checks, ok := want[example.ID]
			if !ok {
				t.Fatalf("missing workflow-shape assertion for %s", example.ID)
			}
			for _, needle := range checks {
				if !strings.Contains(stdout.String(), needle) {
					t.Fatalf("stdout for %s missing %q\nstdout:\n%s", example.ID, needle, stdout.String())
				}
			}
		})
	}
}

func TestPlaygroundTourExamplesProduceTeachingOutputs(t *testing.T) {
	want := map[string][]string{
		"welcome": {
			`"name":"Ada"`,
			`"email":"ada@example.com"`,
			`"name":"Grace"`,
		},
		"control-flow": {
			"click\t3",
			"view\t5",
		},
		"functions": {
			"1\tAda\t98",
			"skip\tbad row: bad",
			"2\tGrace\t95",
		},
		"tables": {
			`"apac"`,
			`"total":42`,
			`"count":2`,
		},
		"strings-stdlib": {
			"ALPHA,BETA,GAMMA",
			"distance\t5",
		},
		"errors": {
			"name\tAda",
			"recover\tmissing field: name",
		},
		"concurrency": {
			"sum of squares\t267",
		},
		"data-oriented": {
			"positions",
			"fast count\t2",
			"fast ids",
		},
		"dialects": {
			"leia\tdialect",
			"true",
			"Explain dialect",
		},
	}
	for _, example := range playgroundTourLessons() {
		t.Run(example.ID, func(t *testing.T) {
			checks, ok := want[example.ID]
			if !ok {
				t.Fatalf("missing teaching-output assertion for %s", example.ID)
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "main.leia")
			if err := os.WriteFile(path, []byte(example.Source), 0600); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			code := runPlaygroundExecCommand([]string{"--max-steps=2000000", path}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("example failed with code %d\nstdout:\n%s\nstderr:\n%s\nsource:\n%s", code, stdout.String(), stderr.String(), example.Source)
			}
			for _, needle := range checks {
				if !strings.Contains(stdout.String(), needle) {
					t.Fatalf("stdout for %s missing %q\nstdout:\n%s", example.ID, needle, stdout.String())
				}
			}
		})
	}
}

func TestPlaygroundExecAIProfileUsesGLMEnv(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"LEIA_GLM_OK"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer server.Close()
	t.Setenv("LEIA_GLM_BASE_URL", server.URL)
	t.Setenv("LEIA_GLM_API_KEY", "test-key")
	t.Setenv("LEIA_GLM_MODEL", "mock-glm")

	dir := t.TempDir()
	path := dir + "/main.leia"
	src := `result, err := llm.turn({ user: "Reply exactly: LEIA_GLM_OK", max_tokens: 16, temperature: 0 })
if err != nil { print(err.message); return }
print(result.text)`
	if err := os.WriteFile(path, []byte(src), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runPlaygroundExecCommand([]string{"--profile=ai", "--max-steps=100000", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "LEIA_GLM_OK" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestPlaygroundExecGLMProfileAliasUsesAIProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"LEIA_GLM_OK"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer server.Close()
	t.Setenv("LEIA_GLM_BASE_URL", server.URL)
	t.Setenv("LEIA_GLM_API_KEY", "test-key")
	t.Setenv("LEIA_GLM_MODEL", "mock-glm")

	dir := t.TempDir()
	path := dir + "/main.leia"
	src := `result, err := llm.turn({ user: "Reply exactly: LEIA_GLM_OK", max_tokens: 16, temperature: 0 })
if err != nil { print(err.message); return }
print(result.text)`
	if err := os.WriteFile(path, []byte(src), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runPlaygroundExecCommand([]string{"--profile=glm", "--max-steps=100000", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "LEIA_GLM_OK" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestPlaygroundCommandRejectsBadFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runPlaygroundCommand([]string{"--timeout=0s"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--timeout must be positive") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestPlaygroundExecRunsSandboxedSource(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/main.leia"
	if err := os.WriteFile(path, []byte(`print("ok")`), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runPlaygroundExecCommand([]string{"--max-steps=1000", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if stdout.String() != "ok\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
