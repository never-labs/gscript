package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	goruntime "runtime"
	"strings"
	"sync"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestCLILLMAnthropicCompatibleRequestsKeepPrompts(t *testing.T) {
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
llm.register_models({
    default: "glm_smoke"
    glm_smoke: {
        protocol: "anthropic_compatible"
        base_url: os.getenv("LEIA_GLM_BASE_URL")
        api_key: os.getenv("LEIA_GLM_API_KEY")
        provider_model: os.getenv("LEIA_GLM_MODEL")
    }
})

history := {
    llm.system("You are a deterministic memory smoke-test assistant."),
    llm.user("Store this memory: project codename is ORCHID and owner is ADA. Reply exactly: MEMORY_STORED"),
}

stored, err := llm.turn({
    messages: history
    max_tokens: 32
    temperature: 0
})
if err != nil {
    return
}

history[#history + 1] = msg.assistant(stored.text)
history[#history + 1] = msg.user("Using only the stored memory, reply exactly: project=ORCHID;owner=ADA")

recalled, err := llm.turn({
    messages: history
    max_tokens: 48
    temperature: 0
})
if err != nil {
    return
}

extractor := llm.agent("extractor", func(summary) {
    return {
        model: "glm_smoke"
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
    }, nil
})

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
			t.Setenv("LEIA_GLM_BASE_URL", server.URL)
			t.Setenv("LEIA_GLM_API_KEY", "test-key")
			t.Setenv("LEIA_GLM_MODEL", "mock-glm")
			interp := newCLIInterpreter()
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
