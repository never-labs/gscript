package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GSCRIPT_TEST_HELPER") == "" {
		return
	}
	switch os.Getenv("GSCRIPT_TEST_HELPER") {
	case "bench":
		_, _ = os.Stdout.WriteString("bench helper ok\n")
		os.Exit(0)
	case "ci":
		_, _ = os.Stdout.WriteString("ci helper ok\n")
		os.Exit(0)
	case "diag":
		_, _ = os.Stdout.WriteString("diag helper ok\n")
		os.Exit(0)
	case "doc":
		_, _ = os.Stdout.WriteString("doc helper ok\n")
		os.Exit(0)
	case "docs":
		_, _ = os.Stdout.WriteString("docs helper ok\n")
		os.Exit(0)
	case "manifest":
		_, _ = os.Stdout.WriteString("manifest helper ok\n")
		os.Exit(0)
	default:
		os.Exit(2)
	}
}

func testHelperCommand(t *testing.T, helper string) (string, []string) {
	t.Helper()
	args := []string{"-test.run=TestHelperProcess", "--"}
	t.Setenv("GSCRIPT_TEST_HELPER", helper)
	return os.Args[0], args
}

func TestCLIAINativeAnthropicCompatibleRequestsKeepPrompts(t *testing.T) {
	type anthropicRequest struct {
		Model    string `json:"model"`
		System   string `json:"system"`
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
	}

	var (
		mu       sync.Mutex
		requests []anthropicRequest
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var req anthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		mu.Lock()
		requests = append(requests, req)
		requestCount := len(requests)
		mu.Unlock()
		text := "MEMORY_STORED"
		switch requestCount {
		case 2:
			text = "project=ORCHID;owner=ADA"
		case 3:
			text = `{"project":"ORCHID","owner":"ADA","remembered":true,"meta":{"source":"history"}}`
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"content":[{"type":"text","text":%q}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`, text)
	}))
	defer server.Close()

	source := `
models {
    default: "glm-smoke"
    "glm-smoke": {
        protocol: "anthropic_compatible"
        base_url: os.getenv("GSCRIPT_GLM_BASE_URL")
        api_key: os.getenv("GSCRIPT_GLM_API_KEY")
        provider_model: os.getenv("GSCRIPT_GLM_MODEL")
    }
}

history := messages {
    system: "You are a deterministic memory smoke-test assistant."
    user: "Store this memory: project codename is ORCHID and owner is ADA. Reply exactly: MEMORY_STORED"
}

stored, err := turn {
    messages: history
    max_tokens: 32
    temperature: 0
}
if err != nil {
    return
}

history[#history + 1] = msg.assistant(stored.text)
history[#history + 1] = msg.user("Using only the stored memory, reply exactly: project=ORCHID;owner=ADA")

recalled, err := turn {
    messages: history
    max_tokens: 48
    temperature: 0
}
if err != nil {
    return
}

extractor := agent(summary) {
    model: "glm-smoke"
    system: "Return only compact JSON."
    user: "Convert this memory recall into JSON. Recall: " .. summary
    output: {
        project: "ORCHID"
        owner: "ADA"
        remembered: true
        meta: {source: "history"}
    }
    max_tokens: 96
    temperature: 0
}

extracted, err := extractor(recalled.text)
project := extracted.value.project
`

	cases := []struct {
		name string
		run  func(*runtime.Interpreter, string) error
	}{
		{name: "interpreter", run: func(interp *runtime.Interpreter, src string) error {
			return runString(interp, src)
		}},
		{name: "bytecode", run: func(interp *runtime.Interpreter, src string) error {
			return runStringVM(interp, src, false, false, jitCLIOptions{})
		}},
	}
	if goruntime.GOOS == "darwin" && goruntime.GOARCH == "arm64" {
		cases = append(cases, struct {
			name string
			run  func(*runtime.Interpreter, string) error
		}{name: "jit", run: func(interp *runtime.Interpreter, src string) error {
			return runStringVM(interp, src, true, false, jitCLIOptions{})
		}})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mu.Lock()
			requests = nil
			mu.Unlock()
			t.Setenv("GSCRIPT_GLM_BASE_URL", server.URL)
			t.Setenv("GSCRIPT_GLM_API_KEY", "test-key")
			t.Setenv("GSCRIPT_GLM_MODEL", "mock-glm")
			interp := runtime.New()
			installCLILLMProviderFactory(interp)
			if err := tc.run(interp, source); err != nil {
				t.Fatalf("run: %v", err)
			}
			mu.Lock()
			gotRequests := append([]anthropicRequest(nil), requests...)
			mu.Unlock()
			if len(gotRequests) != 3 {
				t.Fatalf("requests = %d, want 3: %#v", len(gotRequests), gotRequests)
			}
			for i, req := range gotRequests {
				if req.Model != "mock-glm" {
					t.Fatalf("request %d model = %q, want mock-glm", i+1, req.Model)
				}
				if strings.TrimSpace(req.System) == "" {
					t.Fatalf("request %d system prompt is empty: %#v", i+1, req)
				}
				if len(req.Messages) == 0 {
					t.Fatalf("request %d messages empty: %#v", i+1, req)
				}
				if req.Messages[0].Role != "user" || strings.TrimSpace(fmt.Sprint(req.Messages[0].Content)) == "" {
					t.Fatalf("request %d first user message empty: %#v", i+1, req.Messages)
				}
			}
		})
	}
}

func TestFmtStdinAINativeIndentation(t *testing.T) {
	src := `tool lookup(query) {
return "found:" .. query, nil
}
models {
default: "fast"
fast: {provider_model: "mock-fast"}
}
agent defaults {
model: "fast"
tools: [lookup]
budget: {turns: 2, calls: 4, tokens: 1000, time: 30s}
}
agent researcher(topic) {
system: "Use the tool."
user: topic
tools: [lookup]
} flow {
history := messages {
system: system
user: topic
}
result, err := turn {
messages: history
tools: tools
model: model
}
return result, err
}
answer := agent(q) {
user: q
}
budget { turns: 1 } {
direct, direct_err := turn {
messages: messages { user: "one-shot" }
}
_ = direct
_ = direct_err
}
`
	want := `tool lookup(query) {
    return "found:" .. query, nil
}
models {
    default: "fast"
    fast: {provider_model: "mock-fast"}
}
agent defaults {
    model: "fast"
    tools: [lookup]
    budget: {turns: 2, calls: 4, tokens: 1000, time: 30s}
}
agent researcher(topic) {
    system: "Use the tool."
    user: topic
    tools: [lookup]
} flow {
    history := messages {
        system: system
        user: topic
    }
    result, err := turn {
        messages: history
        tools: tools
        model: model
    }
    return result, err
}
answer := agent(q) {
    user: q
}
budget { turns: 1 } {
    direct, direct_err := turn {
        messages: messages { user: "one-shot" }
    }
    _ = direct
    _ = direct_err
}
`

	oldStdin := cliStdin
	cliStdin = strings.NewReader(src)
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--stdin-file-name", "ai_native.gs"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runFmtCommand code = %d, stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestFmtStdinPreservesCommentOnlyLines(t *testing.T) {
	src := `agent sample() {
// keep this note

user: "hello"
} flow {
if true {
// nested
print("ok")
}
}
`
	want := `agent sample() {
    // keep this note

    user: "hello"
} flow {
    if true {
        // nested
        print("ok")
    }
}
`

	oldStdin := cliStdin
	cliStdin = strings.NewReader(src)
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--stdin-file-name", "comments.gs"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runFmtCommand code = %d, stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestFmtPreservesIntraLineFormattingBoundary(t *testing.T) {
	src := `// lookup searches project docs.
//gscript:requires docs.read
tool lookup(query) {
return "found:"..query,nil
}
models {
short: "x"
longer_key : {provider_model:"mock-fast"}
}
cfg := {short:1, longer_key : 2}
total:=1+  2
`
	want := `// lookup searches project docs.
//gscript:requires docs.read
tool lookup(query) {
    return "found:"..query,nil
}
models {
    short: "x"
    longer_key : {provider_model:"mock-fast"}
}
cfg := {short:1, longer_key : 2}
total:=1+  2
`

	formatted, err := formatSource("boundary.gs", []byte(src))
	if err != nil {
		t.Fatalf("formatSource: %v", err)
	}
	if got := string(formatted); got != want {
		t.Fatalf("formatted = %q, want %q", got, want)
	}
}

func aiNativeToolchainCoverageSource() []byte {
	return []byte(`// lookup searches project docs.
//gscript:requires docs.read
//gscript:param query search query
tool lookup(query) {
    return "found:" .. query, nil
}

models {
    default: "fast"
    fast: {provider_model: "mock-fast"}
}

agent extractor(topic) {
    model: "fast"
    system: "Return JSON."
    user: topic
    output: {summary: "example"}
}

delegate := toolof(extractor, {
    name: "delegate"
    description: "Delegate extraction."
})

agent supervisor(topic) {
    model: "fast"
    tools: [extractor, delegate, lookup]
    user: topic
} flow {
    call := {id: "call_1", tool: "lookup", args: {query: topic}}
    msgs := messages {
        system: system
        user: topic
        msg.assistant_call(call)
        msg.tool_result("call_1", {summary: "docs"})
    }
    tool_msg, tool_idx := history.find(msgs, {role: "tool"})
    assistant_msg, assistant_idx := history.last(msgs, {role: "assistant"})
    all_users := history.find_all(msgs, {role: "user"})
    history.append(msgs, msg.user("Summarize."))
    ok, ok_msg := llm.validate_output({summary: "docs"}, {summary: "example"})
    _ = tool_msg
    _ = tool_idx
    _ = assistant_msg
    _ = assistant_idx
    _ = all_users
    _ = ok
    _ = ok_msg
    return turn {
        messages: msgs
        tools: tools
        model: model
    }
}

answer, answer_err := supervisor("gscript")
_ = answer
_ = answer_err
`)
}

func TestFmtAINativeSyntaxCoverage(t *testing.T) {
	formatted, err := formatSource("ai_native.gs", aiNativeToolchainCoverageSource())
	if err != nil {
		t.Fatalf("formatSource: %v", err)
	}
	for _, want := range []string{
		"tools: [extractor, delegate, lookup]",
		"msg.assistant_call(call)",
		"msg.tool_result(\"call_1\", {summary: \"docs\"})",
		"history.find(msgs, {role: \"tool\"})",
		"history.find_all(msgs, {role: \"user\"})",
		"llm.validate_output({summary: \"docs\"}, {summary: \"example\"})",
	} {
		if !strings.Contains(string(formatted), want) {
			t.Fatalf("formatted AI-native source missing %q:\n%s", want, formatted)
		}
	}
	if strings.Contains(string(formatted), "}  \n") {
		t.Fatalf("formatted source still contains trailing spaces: %q", string(formatted))
	}
	if !strings.HasSuffix(string(formatted), "\n") {
		t.Fatalf("formatted source does not end with newline: %q", string(formatted))
	}
	formattedAgain, err := formatSource("ai_native.gs", formatted)
	if err != nil {
		t.Fatalf("format formatted source: %v", err)
	}
	if !bytes.Equal(formattedAgain, formatted) {
		t.Fatalf("AI-native formatting is not idempotent:\nonce:\n%s\ntwice:\n%s", formatted, formattedAgain)
	}
}

func TestSoADirectAccessRunsInInterpreterAndVM(t *testing.T) {
	source := `
points := soa.zip({
    x: []f64{1, 2, 3},
    y: []f64{10, 20, 30},
    id: []i64{101, 102, 103},
})
xcol := points.x
row := points[2]
row.x = 42
points[2] = row
points.y = []f64{100, 200, 300}
points.z = []i64{7, 8, 9}
assert(xcol[2] == 42)
assert(points.x[2] == 42)
assert(points["x"][3] == 3)
assert(points[2].x == 42)
assert(points.y[3] == 300)
assert(points.z[1] == 7)
assert(points.missing == nil)
`
	for _, tc := range []struct {
		name string
		run  func(*runtime.Interpreter, string) error
	}{
		{name: "interpreter", run: runString},
		{name: "bytecode", run: func(interp *runtime.Interpreter, src string) error {
			return runStringVM(interp, src, false, false, jitCLIOptions{})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			interp := runtime.New()
			if err := tc.run(interp, source); err != nil {
				t.Fatalf("run: %v", err)
			}
		})
	}
}

func TestFmtStdinRejectsPathArguments(t *testing.T) {
	oldStdin := cliStdin
	cliStdin = strings.NewReader("x := 1\n")
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--stdin-file-name", "scratch.gs", "file.gs"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runFmtCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--stdin-file-name cannot be used with path arguments") {
		t.Fatalf("stderr = %q, want stdin/path diagnostic", stderr.String())
	}
}

func TestFmtRefusesSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.gs")
	original := []byte("func {\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runFmtCommand code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "parse error") {
		t.Fatalf("stderr = %q, want parse error", stderr.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("file changed after parse failure: %q", string(got))
	}
}
