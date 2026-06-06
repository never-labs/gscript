package leia_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
	"github.com/never-labs/leia/llm/anthropic"
)

const defaultGLMAnthropicCompatibleBaseURL = "https://open.bigmodel.cn/api/anthropic"

type glmSmokeConfig struct {
	Endpoint string
	APIKey   string
	Model    string
}

func glmAnthropicCompatibleSmokeConfig(t *testing.T) glmSmokeConfig {
	t.Helper()
	if os.Getenv("LEIA_LLM_INTEGRATION") == "" {
		t.Skip("set LEIA_LLM_INTEGRATION=1 to run real GLM provider smoke")
	}
	endpoint := firstNonEmptyEnv("LEIA_GLM_BASE_URL", "ANTHROPIC_BASE_URL")
	if endpoint == "" {
		endpoint = defaultGLMAnthropicCompatibleBaseURL
	}
	apiKey := firstNonEmptyEnv("LEIA_GLM_API_KEY", "SENTINEL_GLM_API_KEY", "GLM_API_KEY", "ANTHROPIC_AUTH_TOKEN")
	model := firstNonEmptyEnv("LEIA_GLM_MODEL", "GLM_MODEL", "ANTHROPIC_MODEL")
	if model == "" {
		model = "glm-5.1"
	}
	if apiKey == "" {
		t.Skip("set LEIA_GLM_API_KEY, SENTINEL_GLM_API_KEY, or GLM_API_KEY; glm_cc uses SENTINEL_GLM_API_KEY/GLM_API_KEY")
	}
	return glmSmokeConfig{Endpoint: endpoint, APIKey: apiKey, Model: model}
}

func firstNonEmptyEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func assertLLMSmokeText(t *testing.T, got, want string) {
	t.Helper()
	normalized := strings.ToLower(strings.TrimSpace(got))
	normalized = strings.Trim(normalized, `"'`+"`")
	if !strings.Contains(normalized, want) {
		t.Fatalf("llm text = %q, want it to contain %q", got, want)
	}
}

// TestGLMAnthropicCompatibleLLMIntegration is a gated smoke for the GLM setup
// used by the local glm_cc wrapper, without invoking the wrapper itself.
func TestGLMAnthropicCompatibleLLMIntegration(t *testing.T) {
	cfg := glmAnthropicCompatibleSmokeConfig(t)
	provider := anthropic.Provider{
		Endpoint:     cfg.Endpoint,
		APIKey:       cfg.APIKey,
		Model:        cfg.Model,
		Timeout:      45 * time.Second,
		MaxAttempts:  2,
		RetryBackoff: 500 * time.Millisecond,
	}

	temperature := 0.0
	prompt := "Reply with exactly: leia glm smoke ok"
	fmt.Printf("endpoint=%s\n", cfg.Endpoint)
	fmt.Printf("model=%s\n", cfg.Model)
	fmt.Printf("user=%q\n", prompt)
	res, err := provider.Turn(context.Background(), llm.TurnRequest{
		Messages: []llm.Message{
			{Role: "system", Text: "You are a concise smoke-test assistant. Return plain text only."},
			{Role: "user", Text: prompt},
		},
		MaxTokens:   32,
		Temperature: &temperature,
	})
	if err != nil {
		t.Fatalf("Turn failed: %v", err)
	}
	fmt.Printf("status=%s reason=%s input_tokens=%d output_tokens=%d\n",
		res.Status, res.Reason, res.Usage.InputTokens, res.Usage.OutputTokens)
	fmt.Printf("text=%q\n", res.Text)
	assertLLMSmokeText(t, res.Text, "leia glm smoke ok")
}

// TestLLMSyntaxGLMIntegration verifies a real multi-turn GLM flow through
// the LLM stdlib API. It mirrors glm_cc's endpoint/key/model env convention
// but never shells out to it.
func TestLLMSyntaxGLMIntegration(t *testing.T) {
	cfg := glmAnthropicCompatibleSmokeConfig(t)
	t.Setenv("LEIA_GLM_BASE_URL", cfg.Endpoint)
	t.Setenv("LEIA_GLM_API_KEY", cfg.APIKey)
	t.Setenv("LEIA_GLM_MODEL", cfg.Model)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	vm := leia.New(leia.WithLibs(leia.LibString | leia.LibOS | leia.LibLLM))
	if err := vm.ExecContext(ctx, `
llm.register_models({
    default: "glm-smoke"
    "glm-smoke": {
        protocol: "anthropic_compatible"
        base_url: os.getenv("LEIA_GLM_BASE_URL")
        api_key: os.getenv("LEIA_GLM_API_KEY")
        provider_model: os.getenv("LEIA_GLM_MODEL")
    }
})

glm_memory_error := nil
glm_stored_text := ""
glm_recalled_text := ""
glm_history_len := 0
glm_project := ""
glm_owner := ""
glm_remembered := false
glm_source := ""
history := {
    llm.system("You are a deterministic memory smoke-test assistant. Follow exact reply instructions. Keep answers short."),
    llm.user("Store this memory: project codename is ORCHID and owner is ADA. Reply exactly: MEMORY_STORED"),
}

stored, stored_err := llm.turn({
    messages: history
    max_tokens: 32
    temperature: 0
})
if stored_err != nil {
    glm_memory_error = stored_err.message
} else {
    history[#history + 1] = msg.assistant(stored.text)
    history[#history + 1] = msg.user("Using only the stored memory, reply exactly: project=ORCHID;owner=ADA")

    recalled, recalled_err := llm.turn({
        messages: history
        max_tokens: 48
        temperature: 0
    })
    if recalled_err != nil {
        glm_memory_error = recalled_err.message
    } else {
        history[#history + 1] = msg.assistant(recalled.text)

        extractor := llm.agent("extractor", func(summary) {
            return {
                model: "glm-smoke"
                system: "Return only compact JSON with exactly these keys: project, owner, remembered, meta. meta must be an object with source. Do not include Markdown."
                user: "Convert this memory recall into JSON. Use project=\"ORCHID\", owner=\"ADA\", remembered=true, meta.source=\"history\" when the recall says project=ORCHID;owner=ADA. Recall: " .. summary
                output: {
                    project: "ORCHID"
                    owner: "ADA"
                    remembered: true
                    meta: {
                        source: "history"
                    }
                }
                max_tokens: 96
                temperature: 0
            }, nil
        }, nil, {params: {"summary"}})

        extracted, extract_err := extractor(recalled.text)
        if extract_err != nil {
            glm_memory_error = extract_err.message
        } else {
            glm_stored_text = stored.text
            glm_recalled_text = recalled.text
            glm_history_len = #history
            glm_project = extracted.value.project
            glm_owner = extracted.value.owner
            glm_remembered = extracted.value.remembered
            glm_source = extracted.value.meta.source
        }
    }
}
`); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}
	if got, err := vm.Get("glm_memory_error"); err == nil && got != nil {
		t.Fatalf("glm_memory_error = %#v", got)
	}
	stored, err := vm.Get("glm_stored_text")
	if err != nil {
		t.Fatalf("Get glm_stored_text: %v", err)
	}
	recalled, err := vm.Get("glm_recalled_text")
	if err != nil {
		t.Fatalf("Get glm_recalled_text: %v", err)
	}
	historyLen, err := vm.Get("glm_history_len")
	if err != nil {
		t.Fatalf("Get glm_history_len: %v", err)
	}
	project, err := vm.Get("glm_project")
	if err != nil {
		t.Fatalf("Get glm_project: %v", err)
	}
	owner, err := vm.Get("glm_owner")
	if err != nil {
		t.Fatalf("Get glm_owner: %v", err)
	}
	remembered, err := vm.Get("glm_remembered")
	if err != nil {
		t.Fatalf("Get glm_remembered: %v", err)
	}
	source, err := vm.Get("glm_source")
	if err != nil {
		t.Fatalf("Get glm_source: %v", err)
	}
	fmt.Printf("endpoint=%s\n", cfg.Endpoint)
	fmt.Printf("model=%s\n", cfg.Model)
	fmt.Printf("stored=%q\n", stored)
	fmt.Printf("recalled=%q\n", recalled)
	fmt.Printf("history_len=%#v project=%#v owner=%#v remembered=%#v source=%#v\n", historyLen, project, owner, remembered, source)
	assertLLMSmokeText(t, fmt.Sprint(stored), "memory_stored")
	assertLLMSmokeText(t, fmt.Sprint(recalled), "project=orchid;owner=ada")
	if historyLen != int64(5) || project != "ORCHID" || owner != "ADA" || remembered != true || source != "history" {
		t.Fatalf("structured memory result mismatch: history=%#v project=%#v owner=%#v remembered=%#v source=%#v", historyLen, project, owner, remembered, source)
	}
}

// TestLLMSyntaxGLMStreamingIntegration verifies that the Anthropic-compatible
// GLM path supports true streaming callbacks, not only a final non-stream turn.
func TestLLMSyntaxGLMStreamingIntegration(t *testing.T) {
	cfg := glmAnthropicCompatibleSmokeConfig(t)
	t.Setenv("LEIA_GLM_BASE_URL", cfg.Endpoint)
	t.Setenv("LEIA_GLM_API_KEY", cfg.APIKey)
	t.Setenv("LEIA_GLM_MODEL", cfg.Model)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	vm := leia.New(leia.WithLibs(leia.LibString | leia.LibOS | leia.LibLLM))
	if err := vm.ExecContext(ctx, `
llm.register_models({
    default: "glm-smoke"
    "glm-smoke": {
        protocol: "anthropic_compatible"
        base_url: os.getenv("LEIA_GLM_BASE_URL")
        api_key: os.getenv("LEIA_GLM_API_KEY")
        provider_model: os.getenv("LEIA_GLM_MODEL")
    }
})

glm_stream_error := nil
glm_streamed_text := ""
glm_stream_final_text := ""
glm_stream_event_count := 0
result, err := llm.turn({
    model: "glm-smoke",
    messages: {llm.system("Return plain text only."), llm.user("Reply with exactly: leia glm stream ok")},
    max_tokens: 32,
    temperature: 0,
    on_stream: func(event) {
        if event.type == "token" {
            glm_stream_event_count = glm_stream_event_count + 1
            glm_streamed_text = glm_streamed_text .. event.token
        }
    },
})
if err != nil {
    glm_stream_error = err.message
} else {
    glm_stream_final_text = result.text
}
`); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}
	if got, err := vm.Get("glm_stream_error"); err == nil && got != nil {
		t.Fatalf("glm_stream_error = %#v", got)
	}
	streamed, err := vm.Get("glm_streamed_text")
	if err != nil {
		t.Fatalf("Get glm_streamed_text: %v", err)
	}
	finalText, err := vm.Get("glm_stream_final_text")
	if err != nil {
		t.Fatalf("Get glm_stream_final_text: %v", err)
	}
	eventCount, err := vm.Get("glm_stream_event_count")
	if err != nil {
		t.Fatalf("Get glm_stream_event_count: %v", err)
	}
	fmt.Printf("endpoint=%s\n", cfg.Endpoint)
	fmt.Printf("model=%s\n", cfg.Model)
	fmt.Printf("stream_events=%#v streamed=%q final=%q\n", eventCount, streamed, finalText)
	if eventCount == int64(0) {
		t.Fatalf("stream callback did not receive token events")
	}
	assertLLMSmokeText(t, fmt.Sprint(streamed), "leia glm stream ok")
	assertLLMSmokeText(t, fmt.Sprint(finalText), "leia glm stream ok")
}

// TestLLMSyntaxGLMDirectAgentToolsIntegration verifies a real GLM
// agent-as-tool loop using a direct agent value in tools: [agent]. It is gated
// the same way as the other GLM smokes and never invokes glm_cc.
func TestLLMSyntaxGLMDirectAgentToolsIntegration(t *testing.T) {
	cfg := glmAnthropicCompatibleSmokeConfig(t)
	t.Setenv("LEIA_GLM_BASE_URL", cfg.Endpoint)
	t.Setenv("LEIA_GLM_API_KEY", cfg.APIKey)
	t.Setenv("LEIA_GLM_MODEL", cfg.Model)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	vm := leia.New(leia.WithLibs(leia.LibString | leia.LibOS | leia.LibLLM))
	if err := vm.ExecContext(ctx, `
llm.register_models({
    default: "glm-smoke"
    "glm-smoke": {
        protocol: "anthropic_compatible"
        base_url: os.getenv("LEIA_GLM_BASE_URL")
        api_key: os.getenv("LEIA_GLM_API_KEY")
        provider_model: os.getenv("LEIA_GLM_MODEL")
    }
})

glm_direct_error := nil
glm_direct_text := ""
glm_direct_history_len := 0
glm_direct_system_role := ""
glm_direct_user_role := ""
glm_direct_assistant_role := ""
glm_direct_tool_role := ""
glm_direct_tool_name := ""
glm_direct_tool_project := ""
glm_direct_tool_owner := ""
glm_direct_tool_source := ""

func extract_memory_config(note) {
    return {
        model: "glm-smoke"
        output: {
            project: "ORCHID"
            owner: "ADA"
            remembered: true
            source: "direct-agent-tool"
        }
    }
}

func extract_memory_flow(note) {
    lower := string.lower(note)
    if string.find(lower, "orchid") == nil || string.find(lower, "ada") == nil {
        return nil, {kind: "validation", message: "memory note missing ORCHID or ADA"}
    }
    return {
        project: "ORCHID"
        owner: "ADA"
        remembered: true
        source: "direct-agent-tool"
    }, nil
}

extract_memory := llm.agent("extract_memory", extract_memory_config, extract_memory_flow, {params: {"note"}})

supervisor := llm.agent("supervisor", func(question) {
    return {
        model: "glm-smoke"
        system: "You are testing tool use. You must call extract_memory exactly once before answering. After the tool result, answer in one short sentence that includes DIRECT_AGENT_TOOL_OK."
        user: question
        tools: {extract_memory}
        max_steps: 4
        max_tokens: 128
        temperature: 0
    }, nil
}, nil, {params: {"question"}})

result, err := supervisor("Use the extract_memory tool with this note: project codename is ORCHID and owner is ADA. Do not answer from memory; call the tool first.")
if err != nil {
    glm_direct_error = err.message
} else {
    glm_direct_text = result.text
    glm_direct_history_len = #result.history
    glm_direct_system_role = result.history[1].role
    glm_direct_user_role = result.history[2].role
    glm_direct_assistant_role = result.history[3].role
    glm_direct_tool_role = result.history[4].role
    glm_direct_tool_name = result.history[3].tool_call.tool
    glm_direct_tool_project = result.history[4].value.project
    glm_direct_tool_owner = result.history[4].value.owner
    glm_direct_tool_source = result.history[4].value.source
}
`); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}
	if got, err := vm.Get("glm_direct_error"); err == nil && got != nil {
		t.Fatalf("glm_direct_error = %#v", got)
	}
	text, err := vm.Get("glm_direct_text")
	if err != nil {
		t.Fatalf("Get glm_direct_text: %v", err)
	}
	historyLen, err := vm.Get("glm_direct_history_len")
	if err != nil {
		t.Fatalf("Get glm_direct_history_len: %v", err)
	}
	systemRole, err := vm.Get("glm_direct_system_role")
	if err != nil {
		t.Fatalf("Get glm_direct_system_role: %v", err)
	}
	userRole, err := vm.Get("glm_direct_user_role")
	if err != nil {
		t.Fatalf("Get glm_direct_user_role: %v", err)
	}
	assistantRole, err := vm.Get("glm_direct_assistant_role")
	if err != nil {
		t.Fatalf("Get glm_direct_assistant_role: %v", err)
	}
	toolRole, err := vm.Get("glm_direct_tool_role")
	if err != nil {
		t.Fatalf("Get glm_direct_tool_role: %v", err)
	}
	toolName, err := vm.Get("glm_direct_tool_name")
	if err != nil {
		t.Fatalf("Get glm_direct_tool_name: %v", err)
	}
	project, err := vm.Get("glm_direct_tool_project")
	if err != nil {
		t.Fatalf("Get glm_direct_tool_project: %v", err)
	}
	owner, err := vm.Get("glm_direct_tool_owner")
	if err != nil {
		t.Fatalf("Get glm_direct_tool_owner: %v", err)
	}
	source, err := vm.Get("glm_direct_tool_source")
	if err != nil {
		t.Fatalf("Get glm_direct_tool_source: %v", err)
	}
	fmt.Printf("endpoint=%s\n", cfg.Endpoint)
	fmt.Printf("model=%s\n", cfg.Model)
	fmt.Printf("direct_text=%q\n", text)
	fmt.Printf("history_len=%#v roles=%#v/%#v/%#v/%#v tool=%#v project=%#v owner=%#v source=%#v\n",
		historyLen, systemRole, userRole, assistantRole, toolRole, toolName, project, owner, source)
	assertLLMSmokeText(t, fmt.Sprint(text), "direct_agent_tool_ok")
	if historyLen != int64(4) ||
		systemRole != "system" || userRole != "user" || assistantRole != "assistant" || toolRole != "tool" ||
		toolName != "extract_memory" || project != "ORCHID" || owner != "ADA" || source != "direct-agent-tool" {
		t.Fatalf("direct agent tool result mismatch: history=%#v roles=%#v/%#v/%#v/%#v tool=%#v project=%#v owner=%#v source=%#v",
			historyLen, systemRole, userRole, assistantRole, toolRole, toolName, project, owner, source)
	}
}

func TestGLMExamplesRunWithRealProviderIntegration(t *testing.T) {
	cfg := glmAnthropicCompatibleSmokeConfig(t)
	t.Setenv("LEIA_GLM_BASE_URL", cfg.Endpoint)
	t.Setenv("LEIA_GLM_API_KEY", cfg.APIKey)
	t.Setenv("LEIA_GLM_MODEL", cfg.Model)
	t.Setenv("SENTINEL_GLM_API_KEY", cfg.APIKey)
	t.Setenv("GLM_API_KEY", cfg.APIKey)
	t.Setenv("GLM_MODEL", cfg.Model)

	root := integrationRepoRoot(t)
	for _, tc := range []struct {
		path string
		want []string
	}{
		{
			path: "examples/llm/glm_smoke.leia",
			want: []string{"stored=", "recalled=", "project=ORCHID", "owner=ADA", "remembered=true", "source=history"},
		},
		{
			path: "examples/llm/glm_direct_agent_tools.leia",
			want: []string{"text=", "history_len=4", "roles=system/user/assistant/tool", "tool=extract_memory", "project=ORCHID", "owner=ADA", "source=direct-agent-tool"},
		},
	} {
		tc := tc
		t.Run(filepath.Base(tc.path), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			var printed []string
			vm := leia.New(
				leia.WithLibs(leia.LibString|leia.LibOS|leia.LibLLM),
				leia.WithPrint(func(args ...interface{}) {
					parts := make([]string, len(args))
					for i, arg := range args {
						parts[i] = fmt.Sprint(arg)
					}
					printed = append(printed, strings.Join(parts, "\t"))
				}),
			)
			if err := vm.ExecFileContext(ctx, filepath.Join(root, filepath.FromSlash(tc.path))); err != nil {
				t.Fatalf("run %s: %v\nprinted:\n%s", tc.path, err, strings.Join(printed, "\n"))
			}
			out := strings.Join(printed, "\n")
			fmt.Printf("%s output:\n%s\n", tc.path, out)
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Fatalf("%s output missing %q:\n%s", tc.path, want, out)
				}
			}
		})
	}
}

func TestGLMExamplesRunAgainstLocalAnthropicCompatibleProvider(t *testing.T) {
	root := integrationRepoRoot(t)
	for _, tc := range []struct {
		name string
		path string
		want []string
	}{
		{
			name: "memory-smoke",
			path: "examples/llm/glm_smoke.leia",
			want: []string{"stored=MEMORY_STORED", "recalled=project=ORCHID;owner=ADA", "project=ORCHID", "owner=ADA", "remembered=true", "source=history"},
		},
		{
			name: "direct-agent-tools",
			path: "examples/llm/glm_direct_agent_tools.leia",
			want: []string{"text=DIRECT_AGENT_TOOL_OK via extract_memory.", "history_len=4", "roles=system/user/assistant/tool", "tool=extract_memory", "project=ORCHID", "owner=ADA", "source=direct-agent-tool"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			server := newLocalGLMAnthropicCompatibleServer(t)
			defer server.Close()
			t.Setenv("LEIA_GLM_BASE_URL", server.URL)
			t.Setenv("LEIA_GLM_API_KEY", "test-key")
			t.Setenv("LEIA_GLM_MODEL", "mock-glm")
			t.Setenv("SENTINEL_GLM_API_KEY", "test-key")
			t.Setenv("GLM_API_KEY", "test-key")
			t.Setenv("GLM_MODEL", "mock-glm")
			t.Setenv("LEIA_LLM_INTEGRATION", "1")

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			var printed []string
			vm := leia.New(
				leia.WithLibs(leia.LibString|leia.LibOS|leia.LibLLM),
				leia.WithPrint(func(args ...interface{}) {
					parts := make([]string, len(args))
					for i, arg := range args {
						parts[i] = fmt.Sprint(arg)
					}
					printed = append(printed, strings.Join(parts, "\t"))
				}),
			)
			if err := vm.ExecFileContext(ctx, filepath.Join(root, filepath.FromSlash(tc.path))); err != nil {
				t.Fatalf("run %s: %v\nprinted:\n%s", tc.path, err, strings.Join(printed, "\n"))
			}
			out := strings.Join(printed, "\n")
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Fatalf("%s output missing %q:\n%s", tc.path, want, out)
				}
			}
		})
	}
}

func newLocalGLMAnthropicCompatibleServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want Bearer test-key", auth)
		}
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "mock-glm" {
			t.Fatalf("model = %q, want mock-glm", req.Model)
		}
		prompt := string(req.Messages[len(req.Messages)-1].Content)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(prompt, "Store this memory"):
			writeAnthropicTextResponse(w, "MEMORY_STORED")
		case strings.Contains(prompt, "Using only the stored memory"):
			writeAnthropicTextResponse(w, "project=ORCHID;owner=ADA")
		case strings.Contains(prompt, "Convert this memory recall into JSON"):
			writeAnthropicTextResponse(w, `{"project":"ORCHID","owner":"ADA","remembered":true,"meta":{"source":"history"}}`)
		case len(req.Tools) > 0 && !strings.Contains(string(mustMarshalGLMRequestMessages(t, req.Messages)), "tool_result"):
			writeAnthropicToolUseResponse(w)
		default:
			writeAnthropicTextResponse(w, "DIRECT_AGENT_TOOL_OK via extract_memory.")
		}
	}))
}

func mustMarshalGLMRequestMessages(t *testing.T, messages []struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}) []byte {
	t.Helper()
	data, err := json.Marshal(messages)
	if err != nil {
		t.Fatalf("marshal request messages: %v", err)
	}
	return data
}

func writeAnthropicTextResponse(w http.ResponseWriter, text string) {
	fmt.Fprintf(w, `{"id":"msg_test","type":"message","role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":%q}],"usage":{"input_tokens":1,"output_tokens":1}}`, text)
}

func writeAnthropicToolUseResponse(w http.ResponseWriter) {
	fmt.Fprint(w, `{"id":"msg_test","type":"message","role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","id":"call_extract_1","name":"extract_memory","input":{"note":"project codename is ORCHID and owner is ADA"}}],"usage":{"input_tokens":1,"output_tokens":1}}`)
}

func integrationRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("could not find repo root containing go.mod")
		}
		wd = parent
	}
}
