package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
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
		`"chan"`,
		`"goto"`,
		`"var"`,
		`"model"`,
		`const leiaDialectTags = new Set([`,
		`"sh", "cmd", "shellwords", "glob", "path", "re", "regexp", "json", "jsonptr",`,
		`"httpmsg", "sse", "multipart", "jwt", "ipaddr", "cidr", "hostport", "serve",`,
		`"binary", "q", "pem", "xlsx", "excel", "sql", "prompt", "quote", "model",`,
		`"turn", "tool", "agent"`,
		`const bt = String.fromCharCode(96);`,
		`ch === "$" && (next === bt || (next === "!" && text[i + 2] === bt))`,
		`leiaDialectTags.has(word) && (text[payloadStart] === bt || text[payloadStart] === "{")`,
		`ch === "/" && next === "/"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("playground page missing %q", want)
		}
	}
	for _, stale := range []string{`"and"`, `"do"`, `"end"`, `"local"`, `"not"`, `"or"`, `"then"`, `"while"`} {
		if strings.Contains(body, stale) {
			t.Fatalf("playground page still advertises unsupported keyword %s", stale)
		}
	}
}

func TestReadmePlaygroundTabsMatchAPISurface(t *testing.T) {
	root := filepath.Dir(playgroundExamplesRoot())
	cliRef, err := os.ReadFile(filepath.Join(root, "docs", "reference", "cli", "index.md"))
	if err != nil {
		t.Fatalf("read CLI reference: %v", err)
	}
	cliText := string(cliRef)
	for _, want := range []string{
		"| `playground` |",
	} {
		if !strings.Contains(cliText, want) {
			t.Fatalf("CLI reference missing playground entry %q", want)
		}
	}

	handler := newPlaygroundHandler(playgroundOptions{
		Runner: func(context.Context, playgroundRunRequest, playgroundOptions) playgroundRunResponse {
			return playgroundRunResponse{}
		},
		EvaluateRunner: func(context.Context, playgroundEvaluateRunRequest, playgroundOptions) playgroundRunResponse {
			return playgroundRunResponse{}
		},
	})
	pageReq := httptest.NewRequest(http.MethodGet, "/", nil)
	pageRec := httptest.NewRecorder()
	handler.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("page status = %d, body = %s", pageRec.Code, pageRec.Body.String())
	}
	page := pageRec.Body.String()
	for _, want := range []string{
		`data-tab="playground"`,
		`data-tab="tour"`,
		`data-tab="examples"`,
		`data-tab="evaluate"`,
		`data-tab="ai"`,
		`url: "/api/tour"`,
		`url: "/api/examples"`,
		`url: "/api/evaluate"`,
		`url: "/api/ai"`,
		`activeTab === "evaluate" ? "/api/evaluate/run" : "/api/run"`,
		`profile: activeTab === "ai" ? "ai" : "sandbox"`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("playground page missing %q", want)
		}
	}

	wantAPIs := map[string]string{
		"/api/tour":     "welcome",
		"/api/examples": "repo-web-route_workbench",
		"/api/evaluate": "evaluate-agent-replay",
		"/api/ai":       "ai-coding-agent",
	}
	for path, wantID := range wantAPIs {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			var examples []playgroundExample
			if err := json.Unmarshal(rec.Body.Bytes(), &examples); err != nil {
				t.Fatalf("decode %s: %v", path, err)
			}
			if len(examples) == 0 {
				t.Fatalf("%s returned no examples", path)
			}
			for _, example := range examples {
				if example.ID == wantID {
					if strings.TrimSpace(example.Source) == "" {
						t.Fatalf("%s source is empty", wantID)
					}
					return
				}
			}
			t.Fatalf("%s did not expose %s", path, wantID)
		})
	}
}

func TestPlaygroundHTTPSmokeCoversBrowserSurface(t *testing.T) {
	var gotRun *playgroundRunRequest
	var gotEvaluate *playgroundEvaluateRunRequest
	handler := newPlaygroundHandler(playgroundOptions{
		Timeout:        time.Second,
		MaxSourceBytes: defaultPlaygroundMaxSourceBytes,
		Runner: func(ctx context.Context, req playgroundRunRequest, opts playgroundOptions) playgroundRunResponse {
			copyReq := req
			gotRun = &copyReq
			return playgroundRunResponse{OK: true, Stdout: "run ok\n", DurationMS: 1}
		},
		EvaluateRunner: func(ctx context.Context, req playgroundEvaluateRunRequest, opts playgroundOptions) playgroundRunResponse {
			copyReq := req
			gotEvaluate = &copyReq
			return playgroundRunResponse{OK: true, Stdout: `{"cases_passed":1}` + "\n", DurationMS: 1}
		},
	})

	pageReq := httptest.NewRequest(http.MethodGet, "/", nil)
	pageRec := httptest.NewRecorder()
	handler.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("page status = %d, body = %s", pageRec.Code, pageRec.Body.String())
	}
	page := pageRec.Body.String()
	for _, want := range []string{
		`<nav class="tabs" aria-label="Playground sections">`,
		`<button class="tab-button active" data-tab="playground" type="button">Playground</button>`,
		`<button class="tab-button" data-tab="tour" type="button">Tour</button>`,
		`<button class="tab-button" data-tab="examples" type="button">Examples</button>`,
		`<button class="tab-button" data-tab="evaluate" type="button">Evaluate</button>`,
		`<button class="tab-button" data-tab="ai" type="button">AI</button>`,
		`<select id="examples" aria-label="Examples"></select>`,
		`<select id="mode" aria-label="Execution mode">`,
		`<button class="primary" id="run">Run</button>`,
		`const source = document.getElementById("source");`,
		`const tabButtons = document.querySelectorAll(".tab-button");`,
		`const tabConfig = {`,
		`playground: {`,
		`tour: {`,
		`url: "/api/tour"`,
		`examples: {`,
		`url: "/api/examples"`,
		`evaluate: {`,
		`url: "/api/evaluate"`,
		`ai: {`,
		`url: "/api/ai"`,
		`button.addEventListener("click", () => setActiveItem(item.id));`,
		`examples.addEventListener("change", () => {`,
		`runButton.disabled = !item.runnable;`,
		`const runURL = activeTab === "evaluate" ? "/api/evaluate/run" : "/api/run";`,
		`? {source: source.value, example_id: activeItemID}`,
		`: {source: source.value, mode: mode.value, profile: activeTab === "ai" ? "ai" : "sandbox"};`,
		`method: "POST"`,
		`headers: {"Content-Type": "application/json"}`,
		`runButton.addEventListener("click", run);`,
		`button.addEventListener("click", () => loadTab(button.dataset.tab));`,
		`loadTab("playground");`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("playground HTML/JS missing browser smoke marker %q", want)
		}
	}

	apiIDs := map[string]string{
		"/api/tour":     "welcome",
		"/api/examples": "repo-hello-counter",
		"/api/evaluate": "evaluate-basic-assert",
		"/api/ai":       "ai-one-line",
	}
	for path, wantID := range apiIDs {
		t.Run("GET "+path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
				t.Fatalf("content-type = %q, want JSON", ct)
			}
			var examples []playgroundExample
			if err := json.Unmarshal(rec.Body.Bytes(), &examples); err != nil {
				t.Fatalf("decode %s: %v", path, err)
			}
			found := false
			for _, example := range examples {
				if example.ID == "" || example.Title == "" || example.Section == "" || strings.TrimSpace(example.Source) == "" {
					t.Fatalf("%s returned incomplete browser option: %#v", path, example)
				}
				if example.ID == wantID {
					found = true
				}
			}
			if !found {
				t.Fatalf("%s missing smoke example %s", path, wantID)
			}
		})
	}

	runPayload := []byte(`{"source":"print(\"browser smoke\")","mode":"bytecode","profile":"ai"}`)
	runReq := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(runPayload))
	runReq.Header.Set("Content-Type", "application/json")
	runRec := httptest.NewRecorder()
	handler.ServeHTTP(runRec, runReq)
	if runRec.Code != http.StatusOK {
		t.Fatalf("run status = %d, body = %s", runRec.Code, runRec.Body.String())
	}
	var runResp playgroundRunResponse
	if err := json.Unmarshal(runRec.Body.Bytes(), &runResp); err != nil {
		t.Fatalf("decode run response: %v", err)
	}
	if !runResp.OK || runResp.Stdout != "run ok\n" {
		t.Fatalf("run response = %#v", runResp)
	}
	if gotRun == nil || gotRun.Source != `print("browser smoke")` || gotRun.Mode != "bytecode" || gotRun.Profile != "ai" {
		t.Fatalf("run request = %#v", gotRun)
	}

	evaluatePayload := []byte(`{"source":"evaluate \"smoke\" { assert(true) }","example_id":"evaluate-basic-assert"}`)
	evaluateReq := httptest.NewRequest(http.MethodPost, "/api/evaluate/run", bytes.NewReader(evaluatePayload))
	evaluateReq.Header.Set("Content-Type", "application/json")
	evaluateRec := httptest.NewRecorder()
	handler.ServeHTTP(evaluateRec, evaluateReq)
	if evaluateRec.Code != http.StatusOK {
		t.Fatalf("evaluate run status = %d, body = %s", evaluateRec.Code, evaluateRec.Body.String())
	}
	var evaluateResp playgroundRunResponse
	if err := json.Unmarshal(evaluateRec.Body.Bytes(), &evaluateResp); err != nil {
		t.Fatalf("decode evaluate response: %v", err)
	}
	if !evaluateResp.OK || !strings.Contains(evaluateResp.Stdout, `"cases_passed":1`) {
		t.Fatalf("evaluate response = %#v", evaluateResp)
	}
	if gotEvaluate == nil || gotEvaluate.Source != `evaluate "smoke" { assert(true) }` || gotEvaluate.ExampleID != "evaluate-basic-assert" {
		t.Fatalf("evaluate request = %#v", gotEvaluate)
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

func TestPlaygroundAIExamplesCoverReadmeAIDialectSurface(t *testing.T) {
	root := filepath.Dir(playgroundExamplesRoot())
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	readmeText := string(readme)
	for _, claim := range []string{
		"answer, err := turn {",
		"[AI dialect](docs/reference/ai/index.md)",
	} {
		if !strings.Contains(readmeText, claim) {
			t.Fatalf("README missing AI playground claim %q", claim)
		}
	}

	byID := make(map[string]playgroundExample)
	for _, example := range playgroundAIExamples() {
		byID[example.ID] = example
	}
	want := map[string][]string{
		"ai-model-alias": {
			"llm.register_models",
			"provider_model",
			"anthropic_compatible",
		},
		"ai-tagged-dialect": {
			"model {",
			"tool {",
			"agent {",
		},
		"ai-memory": {
			"llm.system",
			"llm.user",
			"msg.assistant",
			"msg.user",
		},
		"ai-agent-tool": {
			"tools: {extract_memory}",
		},
		"ai-replay-trace": {
			"turn {",
			"stream: true",
			"on_stream",
			"replay_ready: true",
		},
	}
	for id, snippets := range want {
		example, ok := byID[id]
		if !ok {
			t.Fatalf("AI playground examples missing %s", id)
		}
		if !example.Runnable {
			t.Fatalf("%s should be runnable through the AI playground profile", id)
		}
		for _, snippet := range snippets {
			if !strings.Contains(example.Source, snippet) {
				t.Fatalf("%s source missing %q\nsource:\n%s", id, snippet, example.Source)
			}
		}
	}
}

func TestPlaygroundEvaluateAPI(t *testing.T) {
	handler := newPlaygroundHandler(playgroundOptions{
		Runner: func(context.Context, playgroundRunRequest, playgroundOptions) playgroundRunResponse {
			return playgroundRunResponse{}
		},
		EvaluateRunner: func(context.Context, playgroundEvaluateRunRequest, playgroundOptions) playgroundRunResponse {
			return playgroundRunResponse{}
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/evaluate", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var examples []playgroundExample
	if err := json.Unmarshal(rec.Body.Bytes(), &examples); err != nil {
		t.Fatalf("decode evaluate examples: %v", err)
	}
	byID := make(map[string]playgroundExample, len(examples))
	for _, example := range examples {
		byID[example.ID] = example
		if example.Section == "" || example.Title == "" || example.Summary == "" || strings.TrimSpace(example.Source) == "" {
			t.Fatalf("incomplete evaluate example: %#v", example)
		}
		if !example.Runnable {
			t.Fatalf("%s should be runnable through the evaluate endpoint", example.ID)
		}
	}
	want := map[string][]string{
		"evaluate-basic-assert":             {"evaluate \"basic assert\"", "assert(slug == \"leia-checks\")"},
		"evaluate-corpus-metrics":           {"eval.load_jsonl", "eval.metric"},
		"evaluate-llm-replay":               {"llm.turn({", "Deterministic agent checks with replay"},
		"evaluate-agent-replay":             {"llm.agent(\"classify_support\"", "result.text == \"refund\""},
		"evaluate-multiturn-replay":         {"run_multiturn_replay", "Leia checks now support deterministic replay."},
		"evaluate-project-agent-regression": {"eval.usage().turns", "eval.usage().stream_events"},
	}
	for id, snippets := range want {
		example, ok := byID[id]
		if !ok {
			t.Fatalf("%s not returned by /api/evaluate", id)
		}
		for _, snippet := range snippets {
			if !strings.Contains(example.Source, snippet) {
				t.Fatalf("%s source missing %q\nsource:\n%s", id, snippet, example.Source)
			}
		}
	}
}

func TestPlaygroundEvaluateRunAPIExecutesReplayExample(t *testing.T) {
	handler := newPlaygroundHandler(playgroundOptions{
		Timeout:        10 * time.Second,
		MaxSourceBytes: defaultPlaygroundMaxSourceBytes,
		EvaluateRunner: directPlaygroundEvaluateRunner,
		Runner: func(context.Context, playgroundRunRequest, playgroundOptions) playgroundRunResponse {
			t.Fatal("script runner should not be called for evaluate run")
			return playgroundRunResponse{}
		},
	})
	example, ok := playgroundEvaluateExampleByID("evaluate-agent-replay")
	if !ok {
		t.Fatal("evaluate-agent-replay example not found")
	}
	payload, err := json.Marshal(playgroundEvaluateRunRequest{
		Source:    example.Source,
		ExampleID: example.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/evaluate/run", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp playgroundRunResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode run response: %v", err)
	}
	if !resp.OK {
		t.Fatalf("evaluate run failed\nstdout:\n%s\nstderr:\n%s\nerror:%s", resp.Stdout, resp.Stderr, resp.Error)
	}
	if !strings.Contains(resp.Stdout, `"cases_passed": 1`) || !strings.Contains(resp.Stdout, `"mode": "replay"`) {
		t.Fatalf("stdout is not a passed replay evaluate report:\n%s", resp.Stdout)
	}
}

func TestPlaygroundEvaluateRepositoryMetadataDoesNotConflictWithCLIRunner(t *testing.T) {
	playgroundExamples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load playground repository examples: %v", err)
	}
	cliExamples, err := cliRepositoryExamples()
	if err != nil {
		t.Fatalf("load CLI repository examples: %v", err)
	}
	var playgroundExample playgroundExample
	foundPlayground := false
	for _, example := range playgroundExamples {
		if example.ID == "repo-evaluate-agent_replay" {
			playgroundExample = example
			foundPlayground = true
			break
		}
	}
	if !foundPlayground {
		t.Fatal("repo-evaluate-agent_replay not found in playground repository examples")
	}
	if playgroundExample.Runnable || playgroundExample.Requires != "leia evaluate CLI" {
		t.Fatalf("playground repository metadata = %#v, want manual evaluate CLI gate", playgroundExample)
	}
	for _, example := range cliExamples {
		if example.ID != "repo-evaluate-agent_replay" {
			continue
		}
		if !example.Runnable || !example.Checkable || example.Runner != "evaluate-replay" || example.Requires != "" {
			t.Fatalf("CLI metadata = %#v, want runnable evaluate-replay runner", example)
		}
		return
	}
	t.Fatal("repo-evaluate-agent_replay not found in CLI examples")
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
		"repo-concurrency-pipeline_project-main":                  "host VM concurrency runner",
		"repo-concurrency-context_process":                        "process host access",
		"repo-concurrency-goroutine_errors":                       "debug event sink host access",
		"repo-data-db_q_frame_project-main":                       "SQLite and host VM data runner",
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

func TestPlaygroundWebExamplesKeepRouteServerCoverage(t *testing.T) {
	playgroundExamples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load repository examples: %v", err)
	}
	cliExamples, err := cliRepositoryExamples()
	if err != nil {
		t.Fatalf("load CLI examples: %v", err)
	}
	byPlaygroundID := make(map[string]playgroundExample, len(playgroundExamples))
	for _, example := range playgroundExamples {
		byPlaygroundID[example.ID] = example
	}
	byCLIPath := make(map[string]cliExample, len(cliExamples))
	for _, example := range cliExamples {
		byCLIPath[example.Path] = example
	}

	required := map[string][]string{
		"repo-web-route_workbench": {
			`serve.app({`,
			`path: "/api/items/:id"`,
			`net.put(`,
			`net.delete(`,
		},
		"repo-web-serve_dialect_app": {
			`app := serve {`,
			`path: "/orders/:id"`,
			`wrong_method.status == 405`,
		},
		"repo-web-tiny_fullstack_app": {
			`conn := db.memory()`,
			`path: "/api/posts/:id"`,
			`Content-Type`,
			`app.close()`,
		},
	}
	for id, snippets := range required {
		example, ok := byPlaygroundID[id]
		if !ok {
			t.Fatalf("playground repository examples missing %s", id)
		}
		if example.Section != "Web" {
			t.Fatalf("%s section = %q, want Web", id, example.Section)
		}
		if example.Runnable || !strings.Contains(example.Requires, "host access") {
			t.Fatalf("%s playground metadata = %#v, want manual host-access explanation", id, example)
		}
		cli, ok := byCLIPath[example.Summary]
		if !ok {
			t.Fatalf("%s path %s missing from examples CLI metadata", id, example.Summary)
		}
		if !cli.Runnable || !cli.Checkable {
			t.Fatalf("%s CLI metadata = %#v, want runnable and checkable", id, cli)
		}
		for _, snippet := range snippets {
			if !strings.Contains(example.Source, snippet) {
				t.Fatalf("%s source missing %q\nsource:\n%s", id, snippet, example.Source)
			}
		}
	}
}

func TestPlaygroundRepositoryCoversDocumentedExampleDirectories(t *testing.T) {
	root := filepath.Dir(playgroundExamplesRoot())
	playgroundExamples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load repository examples: %v", err)
	}
	cliExamples, err := cliRepositoryExamples()
	if err != nil {
		t.Fatalf("load CLI examples: %v", err)
	}

	playgroundDirs := make(map[string]int)
	for _, example := range playgroundExamples {
		parts := strings.Split(example.Summary, "/")
		if len(parts) >= 3 && parts[0] == "examples" {
			playgroundDirs[parts[1]]++
		}
	}
	cliIDs := make(map[string]bool, len(cliExamples))
	cliDirs := make(map[string]int)
	for _, example := range cliExamples {
		cliIDs[example.ID] = true
		parts := strings.Split(example.Path, "/")
		if len(parts) >= 3 && parts[0] == "examples" {
			cliDirs[parts[1]]++
		}
	}

	// Keep this list in sync with the directory matrices in docs/examples/index.md and examples/README.md.
	wantExampleDirs := []string{
		"ai",
		"api",
		"automation",
		"concurrency",
		"data",
		"data_processing",
		"database",
		"dialects",
		"evaluate",
		"game_engine",
		"hello",
		"llm",
		"macos",
		"operations",
		"performance",
		"security",
		"site",
		"testing",
		"tooling",
		"ui",
		"web",
		"workflow",
	}
	var missing []string
	for _, dir := range wantExampleDirs {
		if cliDirs[dir] == 0 {
			missing = append(missing, "examples/"+dir+"/ (CLI)")
		}
		if dir != "embedding" && playgroundDirs[dir] == 0 {
			missing = append(missing, "examples/"+dir+"/")
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("repository examples missing documented directories: %s", strings.Join(missing, ", "))
	}
	if !cliIDs["repo-embedding-go-doc-examples"] {
		t.Fatal("examples/embedding/ is documented but missing from examples CLI curated metadata")
	}
	docsIndex, err := os.ReadFile(filepath.Join(root, "docs", "examples", "index.md"))
	if err != nil {
		t.Fatalf("read docs examples index: %v", err)
	}
	examplesReadme, err := os.ReadFile(filepath.Join(root, "examples", "README.md"))
	if err != nil {
		t.Fatalf("read examples README: %v", err)
	}
	docsText := string(docsIndex)
	readmeText := string(examplesReadme)
	for _, dir := range wantExampleDirs {
		if !strings.Contains(docsText, "| `examples/"+dir+"/` |") {
			t.Fatalf("docs/examples/index.md missing directory matrix row for examples/%s/", dir)
		}
		if !strings.Contains(readmeText, "| `"+dir+"/` |") {
			t.Fatalf("examples/README.md missing directory matrix row for %s/", dir)
		}
	}
}

func TestPlaygroundAndExamplesCoverFeatureMatrixExampleRefs(t *testing.T) {
	root := filepath.Dir(playgroundExamplesRoot())
	matrixPath := filepath.Join(root, "tests", "feature_matrix.json")
	data, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatalf("read feature matrix: %v", err)
	}
	var matrix struct {
		RequiredFields []string                     `json:"required_fields"`
		Features       []map[string]json.RawMessage `json:"features"`
	}
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatalf("decode feature matrix: %v", err)
	}

	playgroundExamples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load repository examples: %v", err)
	}
	playgroundPaths := make(map[string]bool, len(playgroundExamples))
	for _, example := range playgroundExamples {
		playgroundPaths[example.Summary] = true
	}
	cliExamples, err := cliRepositoryExamples()
	if err != nil {
		t.Fatalf("load CLI examples: %v", err)
	}
	cliPaths := make(map[string]cliExample, len(cliExamples))
	for _, example := range cliExamples {
		cliPaths[example.Path] = example
	}

	var missing []string
	var uncheckable []string
	seen := map[string]bool{}
	for i, feature := range matrix.Features {
		id := ""
		if raw, ok := feature["id"]; ok {
			_ = json.Unmarshal(raw, &id)
		}
		for _, field := range matrix.RequiredFields {
			raw, ok := feature[field]
			if !ok {
				continue
			}
			var cell struct {
				Refs []string `json:"refs"`
			}
			if err := json.Unmarshal(raw, &cell); err != nil {
				t.Fatalf("features[%d] %s.%s: %v", i, id, field, err)
			}
			for _, ref := range cell.Refs {
				if seen[ref] || !strings.HasPrefix(ref, "examples/") {
					continue
				}
				seen[ref] = true
				switch filepath.Ext(ref) {
				case ".leia":
					if !playgroundPaths[ref] {
						missing = append(missing, ref)
					}
					if cli, ok := cliPaths[ref]; !ok {
						missing = append(missing, ref+" (CLI)")
					} else if !cli.Runnable && !cli.Checkable && strings.TrimSpace(cli.Requires) == "" {
						uncheckable = append(uncheckable, ref)
					}
				case ".go":
					if cli, ok := cliPaths[ref]; !ok {
						missing = append(missing, ref+" (CLI)")
					} else if !cli.Runnable && !cli.Checkable && strings.TrimSpace(cli.Requires) == "" {
						uncheckable = append(uncheckable, ref)
					}
				}
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("feature matrix example refs missing from playground or examples metadata: %s", strings.Join(missing, ", "))
	}
	if len(uncheckable) > 0 {
		sort.Strings(uncheckable)
		t.Fatalf("feature matrix example refs must be runnable or checkable in examples metadata: %s", strings.Join(uncheckable, ", "))
	}
}

func TestReadmeFacingFeatureMatrixClaimsKeepRunnableExamples(t *testing.T) {
	root := filepath.Dir(playgroundExamplesRoot())
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	readmeText := string(readme)

	featureExamples := loadFeatureMatrixSemanticExampleRefs(t, root)
	cliExamples, err := cliRepositoryExamples()
	if err != nil {
		t.Fatalf("load CLI examples: %v", err)
	}
	cliPaths := make(map[string]cliExample, len(cliExamples))
	for _, example := range cliExamples {
		cliPaths[example.Path] = example
	}

	claims := []struct {
		readmeSnippet string
		featureID     string
		exampleRef    string
	}{
		{"leia.New(leia.WithLibs(leia.LibSafe))", "embedding_host_bindings", "examples/embedding/embedding_test.go"},
		{"answer, err := turn {", "llm_native_integration", "examples/llm/agent.leia"},
		{"cmd := $`git status --short`", "tagged_dialect_syntax", "examples/hello/dialects.leia"},
		{"q-style columnar analytics", "matrix_dense_arrays", "examples/data_processing/data_oriented/dense_matrix_vec_kernels.leia"},
		{"ARM64 JIT", "arm64_jit_runtime_fallback", "examples/performance/execution_modes_matrix.leia"},
	}

	for _, claim := range claims {
		if !strings.Contains(readmeText, claim.readmeSnippet) {
			t.Fatalf("README claim snippet %q missing", claim.readmeSnippet)
		}
		if !featureExamples[claim.featureID][claim.exampleRef] {
			t.Fatalf("README-facing feature %s must keep semantic_gate example ref %s", claim.featureID, claim.exampleRef)
		}
		example, ok := cliPaths[claim.exampleRef]
		if !ok {
			t.Fatalf("README-facing feature %s example %s missing from examples CLI metadata", claim.featureID, claim.exampleRef)
		}
		if !example.Runnable && !example.Checkable {
			t.Fatalf("README-facing feature %s example %s must be runnable or checkable, metadata = %#v", claim.featureID, claim.exampleRef, example)
		}
	}
}

func loadFeatureMatrixSemanticExampleRefs(t *testing.T, root string) map[string]map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "tests", "feature_matrix.json"))
	if err != nil {
		t.Fatalf("read feature matrix: %v", err)
	}
	var matrix struct {
		Features []struct {
			ID           string `json:"id"`
			SemanticGate struct {
				Refs []string `json:"refs"`
			} `json:"semantic_gate"`
		} `json:"features"`
	}
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatalf("decode feature matrix: %v", err)
	}
	out := make(map[string]map[string]bool, len(matrix.Features))
	for _, feature := range matrix.Features {
		refs := make(map[string]bool)
		for _, ref := range feature.SemanticGate.Refs {
			if strings.HasPrefix(ref, "examples/") {
				refs[ref] = true
			}
		}
		out[feature.ID] = refs
	}
	return out
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
		if !strings.Contains(example.Source, `import ui "github.com/never-labs/leia-ui/raylib"`) {
			t.Fatalf("source missing package-managed UI runtime import\nsource:\n%s", example.Source)
		}
		return
	}
	t.Fatal("repo-ui-package_managed-main example not found")
}

func TestPlaygroundRepositoryBuiltinDatabaseExampleIsRunnable(t *testing.T) {
	examples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load repository examples: %v", err)
	}
	for _, example := range examples {
		if example.ID != "repo-database-package_managed-main" {
			continue
		}
		if !example.Runnable {
			t.Fatalf("%s should be runnable in the playground, requires = %q", example.ID, example.Requires)
		}
		if example.Requires != "" {
			t.Fatalf("requires = %q", example.Requires)
		}
		if !strings.Contains(example.Source, "conn := db.memory()") || !strings.Contains(example.Source, "conn.query(") {
			t.Fatalf("source missing built-in database runtime usage\nsource:\n%s", example.Source)
		}
		return
	}
	t.Fatal("repo-database-package_managed-main example not found")
}

func TestPlaygroundRepositoryPackageManagedMacOSExampleIsManualRunOnly(t *testing.T) {
	examples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load repository examples: %v", err)
	}
	for _, example := range examples {
		if example.ID != "repo-macos-package_managed-main" {
			continue
		}
		if example.Runnable {
			t.Fatalf("%s should be manual-run only in the playground", example.ID)
		}
		if example.Requires != "package-managed macOS automation runtime and process host access" {
			t.Fatalf("requires = %q", example.Requires)
		}
		if !strings.Contains(example.Source, `import macos "github.com/never-labs/leia-macos/automation"`) {
			t.Fatalf("source missing package-managed macOS runtime import\nsource:\n%s", example.Source)
		}
		return
	}
	t.Fatal("repo-macos-package_managed-main example not found")
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

func TestPlaygroundRepositoryAIDialectExamplesHaveExplicitGates(t *testing.T) {
	examples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load repository examples: %v", err)
	}
	byID := make(map[string]playgroundExample, len(examples))
	for _, example := range examples {
		byID[example.ID] = example
	}

	want := map[string]struct {
		requires string
		snippets []string
	}{
		"repo-ai-coding_agent_replay": {
			requires: "local AI playground profile or LLM replay fixture",
			snippets: []string{"model {", "tool {", "turn {", "tools: {read_file, search_text, apply_patch, run_shell}"},
		},
		"repo-ai-coding_agent_project-main": {
			requires: "local AI playground profile or LLM replay fixture",
			snippets: []string{"llm.run_agent({", "read_file := tool {", "search_text := tool {", "apply_patch := tool {", "run_shell := tool {", "test_runs == 2"},
		},
		"repo-ai-tagged_agent_workflow": {
			requires: "local AI playground profile or LLM replay fixture",
			snippets: []string{"model {", "tool {", "agent {", "responder("},
		},
		"repo-ai-general_agent_workflow": {
			requires: "local AI playground profile or LLM replay fixture",
			snippets: []string{"model {", "tool {", "agent {", "output: {", "turn {", "replay_ready"},
		},
		"repo-ai-record_replay_trace_project": {
			requires: "local AI playground profile or LLM replay fixture",
			snippets: []string{"model {", "turn {", "stream: true", "replay_ready"},
		},
		"repo-llm-agent": {
			requires: "LLM provider",
			snippets: []string{"llm.register_models({", "llm.agent(\"answer\"", "messages: {"},
		},
		"repo-llm-agent_as_tool": {
			requires: "LLM provider",
			snippets: []string{"llm.agent(\"extract_research\"", "tools: {extract_research}", "llm.turn({"},
		},
		"repo-llm-direct_turn": {
			requires: "LLM provider",
			snippets: []string{"llm.turn({", "llm.user(question)", "direct_text"},
		},
		"repo-llm-glm_smoke": {
			requires: "LLM provider",
			snippets: []string{"protocol: \"anthropic_compatible\"", "LEIA_GLM_API_KEY", "SENTINEL_GLM_API_KEY", "provider_model: env_first(\"LEIA_GLM_MODEL\""},
		},
		"repo-evaluate-llm_replay": {
			requires: "leia evaluate CLI",
			snippets: []string{"evaluate", "llm.turn({"},
		},
		"repo-evaluate-agent_replay": {
			requires: "leia evaluate CLI",
			snippets: []string{"evaluate", "llm.agent("},
		},
		"repo-evaluate-multiturn_replay": {
			requires: "leia evaluate CLI",
			snippets: []string{"evaluate", "llm.turn({"},
		},
		"repo-workflow-support_triage_replay": {
			requires: "LLM replay fixture or provider",
			snippets: []string{"model: \"mock-fast\"", "llm.turn({"},
		},
	}
	for id, want := range want {
		t.Run(id, func(t *testing.T) {
			example, ok := byID[id]
			if !ok {
				t.Fatalf("%s example not found", id)
			}
			if example.Runnable {
				t.Fatalf("%s should be manual-run only in the playground", id)
			}
			if example.Requires != want.requires {
				t.Fatalf("%s requires = %q, want %q", id, example.Requires, want.requires)
			}
			for _, snippet := range want.snippets {
				if !strings.Contains(example.Source, snippet) {
					t.Fatalf("%s source missing %q\nsource:\n%s", id, snippet, example.Source)
				}
			}
		})
	}
}

func TestPlaygroundRepositoryHighLevelReleaseExamplesAreExposed(t *testing.T) {
	examples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load repository examples: %v", err)
	}
	byID := make(map[string]playgroundExample, len(examples))
	for _, example := range examples {
		byID[example.ID] = example
	}

	want := map[string]struct {
		runnable bool
		requires string
		snippets []string
	}{
		"repo-data-db_q_frame_project-main": {
			requires: "SQLite and host VM data runner",
			snippets: []string{"conn.frame(", "q.query(", "dialect.eval(\"xlsx\"", "dialect.eval(\"excel\""},
		},
		"repo-evaluate-project_agent_regression": {
			requires: "leia evaluate CLI",
			snippets: []string{"stream: true", "on_stream:", "eval.usage().turns", "eval.usage().stream_events"},
		},
		"repo-web-serve_dialect_app": {
			requires: "network/server host access",
			snippets: []string{"serve {", "routes:", "http.get(", "net.post(", "method_status"},
		},
		"repo-web-tiny_fullstack_app": {
			requires: "network/server host access",
			snippets: []string{"serve {", "db.memory()", "http.get(", "net.post(", "/static/:name"},
		},
		"repo-tooling-release_gate_project-main": {
			requires: "examples CLI release-gate-project runner",
			snippets: []string{"release_gate_project", "sh`printf release-gate`", "q.query(", "dialect.eval(\"xlsx\"", "agent {", "serve {"},
		},
	}
	for id, want := range want {
		t.Run(id, func(t *testing.T) {
			example, ok := byID[id]
			if !ok {
				t.Fatalf("%s example not exposed through playground repository examples", id)
			}
			if example.Runnable != want.runnable {
				t.Fatalf("%s runnable = %t, want %t", id, example.Runnable, want.runnable)
			}
			if example.Requires != want.requires {
				t.Fatalf("%s requires = %q, want %q", id, example.Requires, want.requires)
			}
			for _, snippet := range want.snippets {
				if !strings.Contains(example.Source, snippet) {
					t.Fatalf("%s source missing %q\nsource:\n%s", id, snippet, example.Source)
				}
			}
		})
	}
}

func TestPlaygroundRepositoryGameEngineExampleClassification(t *testing.T) {
	examples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load repository examples: %v", err)
	}
	byID := make(map[string]playgroundExample, len(examples))
	for _, example := range examples {
		byID[example.ID] = example
	}
	want := map[string]struct {
		runnable bool
		requires string
	}{
		"repo-game_engine-event_system":         {runnable: true},
		"repo-game_engine-game_of_life":         {requires: "higher playground step budget"},
		"repo-game_engine-chess_bench":          {requires: "higher playground step budget"},
		"repo-game_engine-chess_bench_parallel": {requires: "higher playground step budget"},
		"repo-game_engine-chess":                {requires: "game/window host access"},
		"repo-game_engine-chess_ai":             {requires: "game/window host access"},
		"repo-game_engine-game":                 {requires: "game/window host access"},
		"repo-game_engine-tetris":               {requires: "game/window host access"},
	}
	for id, want := range want {
		t.Run(id, func(t *testing.T) {
			example, ok := byID[id]
			if !ok {
				t.Fatalf("%s example not found", id)
			}
			if example.Runnable != want.runnable {
				t.Fatalf("runnable = %t, want %t", example.Runnable, want.runnable)
			}
			if example.Requires != want.requires {
				t.Fatalf("requires = %q, want %q", example.Requires, want.requires)
			}
		})
	}
}

func TestPlaygroundAIExamplesCoverRunnableWorkflowShapes(t *testing.T) {
	t.Setenv("LEIA_PLAYGROUND_MOCK_LLM", "1")
	want := map[string][]string{
		"ai-model-alias":       {"model alias\tLEIA_GLM_OK"},
		"ai-one-line":          {"LEIA_GLM_OK"},
		"ai-tagged-dialect":    {"done", "MOCK_TOOL_RESULT service=checkout p95 latency is elevated; severity=sev2"},
		"ai-agent-shape":       {"MOCK_AI_OK"},
		"ai-structured-output": {"\"product\":\"playground\""},
		"ai-streaming":         {"streamed\tLEIA_GLM_OK", "final\tLEIA_GLM_OK"},
		"ai-replay-trace":      {"turns\t1", "stream_events\t3", "replay_ready\ttrue"},
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
			"leia serves application/json",
			"GET /health HTTP/1.1",
			"Explain dialect",
		},
		"data-dialects": {
			"spread",
			"bid\t99.5",
			"rows\t2",
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

func TestPlaygroundRepositoryAPIContractDialectExample(t *testing.T) {
	examples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load repository examples: %v", err)
	}
	for _, example := range examples {
		if example.ID != "repo-dialects-api_contract_fixture" {
			continue
		}
		if !example.Runnable {
			t.Fatalf("%s should be runnable, requires = %q", example.ID, example.Requires)
		}
		if strings.TrimSpace(example.Requires) != "" {
			t.Fatalf("%s runnable example should not require manual setup: %q", example.ID, example.Requires)
		}
		for _, want := range []string{`httpmsg {`, `dialect.eval("cookies"`, `template {`, `assert(fingerprint ==`} {
			if !strings.Contains(example.Source, want) {
				t.Fatalf("source missing %q\nsource:\n%s", want, example.Source)
			}
		}
		return
	}
	t.Fatal("repo-dialects-api_contract_fixture example not found")
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
	src := `result, err := llm.turn({ messages: {llm.user("Reply exactly: LEIA_GLM_OK")}, max_tokens: 16, temperature: 0 })
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
	src := `result, err := llm.turn({ messages: {llm.user("Reply exactly: LEIA_GLM_OK")}, max_tokens: 16, temperature: 0 })
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
