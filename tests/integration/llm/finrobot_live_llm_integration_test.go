package leia_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
	"github.com/never-labs/leia/llm/anthropic"
)

func TestFinRobotLiveLLMGateUsesExplicitProviderConfigWithMockServer(t *testing.T) {
	type request struct {
		Model    string `json:"model"`
		System   string `json:"system"`
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
	}
	var got request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
  "type": "message",
  "role": "assistant",
  "stop_reason": "end_turn",
  "content": [{"type":"text","text":"{\"domain\":\"finrobot\",\"handoff\":\"equity_report\",\"provider_gate\":\"ok\"}"}],
  "usage": {"input_tokens": 11, "output_tokens": 9}
}`)
	}))
	defer server.Close()

	t.Setenv("LEIA_LLM_INTEGRATION", "1")
	t.Setenv("LEIA_GLM_BASE_URL", server.URL)
	t.Setenv("LEIA_GLM_API_KEY", "test-key")
	t.Setenv("LEIA_GLM_MODEL", "mock-finrobot-live")

	vm := leia.New(
		leia.WithLibs(leia.LibString|leia.LibOS|leia.LibLLM),
		leia.WithLLMProviderFactory(func(cfg llm.ProviderConfig) (llm.Provider, error) {
			return anthropic.Provider{
				Endpoint: cfg.BaseURL,
				APIKey:   cfg.APIKey,
				Model:    cfg.ProviderModel,
				Client:   server.Client(),
			}, nil
		}),
	)
	if err := vm.Exec(`
llm.register_models({
    default: "finrobot_live_gate"
    finrobot_live_gate: {
        protocol: "anthropic_compatible"
        base_url: os.getenv("LEIA_GLM_BASE_URL")
        api_key: os.getenv("LEIA_GLM_API_KEY")
        provider_model: os.getenv("LEIA_GLM_MODEL")
    }
})

result, err := llm.turn({
    model: "finrobot_live_gate"
    messages: {
        llm.system("Return only compact JSON. Do not include Markdown."),
        llm.user("Produce a FinRobot equity-report handoff smoke response with domain, handoff, and provider_gate fields.")
    }
    max_tokens: 96
    temperature: 0
})
if err != nil {
    return
}
finrobot_live_gate_text := result.text
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	text, err := vm.Get("finrobot_live_gate_text")
	if err != nil {
		t.Fatalf("Get finrobot_live_gate_text: %v", err)
	}
	gotText := fmt.Sprint(text)
	for _, want := range []string{`"domain":"finrobot"`, `"handoff":"equity_report"`, `"provider_gate":"ok"`} {
		if !strings.Contains(gotText, want) {
			t.Fatalf("response %q missing %q", gotText, want)
		}
	}
	if got.Model != "mock-finrobot-live" ||
		!strings.Contains(got.System, "compact JSON") ||
		len(got.Messages) != 1 ||
		got.Messages[0].Role != "user" ||
		!strings.Contains(fmt.Sprint(got.Messages[0].Content), "FinRobot equity-report handoff") {
		t.Fatalf("provider request did not preserve FinRobot live gate prompt/config: %#v", got)
	}
}

func TestFinRobotLiveLLMGateGLMIntegration(t *testing.T) {
	cfg := glmAnthropicCompatibleSmokeConfig(t)
	t.Setenv("LEIA_GLM_BASE_URL", cfg.Endpoint)
	t.Setenv("LEIA_GLM_API_KEY", cfg.APIKey)
	t.Setenv("LEIA_GLM_MODEL", cfg.Model)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	vm := leia.New(leia.WithLibs(leia.LibString | leia.LibOS | leia.LibLLM))
	if err := vm.ExecContext(ctx, `
llm.register_models({
    default: "finrobot_live_gate"
    finrobot_live_gate: {
        protocol: "anthropic_compatible"
        base_url: os.getenv("LEIA_GLM_BASE_URL")
        api_key: os.getenv("LEIA_GLM_API_KEY")
        provider_model: os.getenv("LEIA_GLM_MODEL")
    }
})

finrobot_live_gate_error := nil
finrobot_live_gate_text := ""
result, err := llm.turn({
    model: "finrobot_live_gate"
    messages: {
        llm.system("Return only plain text. Keep the answer short."),
        llm.user("Reply with exactly: finrobot live llm gate ok")
    }
    max_tokens: 32
    temperature: 0
})
if err != nil {
    finrobot_live_gate_error = err.message
} else {
    finrobot_live_gate_text = result.text
}
`); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}
	if got, err := vm.Get("finrobot_live_gate_error"); err == nil && got != nil {
		t.Fatalf("finrobot_live_gate_error = %#v", got)
	}
	text, err := vm.Get("finrobot_live_gate_text")
	if err != nil {
		t.Fatalf("Get finrobot_live_gate_text: %v", err)
	}
	fmt.Printf("endpoint=%s\n", cfg.Endpoint)
	fmt.Printf("model=%s\n", cfg.Model)
	fmt.Printf("text=%q\n", text)
	assertLLMSmokeText(t, fmt.Sprint(text), "finrobot live llm gate ok")
}
