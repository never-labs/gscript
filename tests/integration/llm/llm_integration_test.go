package leia_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
	"github.com/never-labs/leia/llm/anthropic"
)

// TestAnthropicCompatibleLLMIntegration is a gated real-provider smoke test.
// It intentionally uses generic Anthropic-compatible configuration names so
// local wrappers or vendor-specific credentials do not become Leia API.
func TestAnthropicCompatibleLLMIntegration(t *testing.T) {
	if os.Getenv("LEIA_LLM_INTEGRATION") == "" {
		t.Skip("set LEIA_LLM_INTEGRATION=1 to run real Anthropic-compatible provider smoke")
	}
	endpoint := os.Getenv("LEIA_ANTHROPIC_COMPAT_BASE_URL")
	apiKey := os.Getenv("LEIA_ANTHROPIC_COMPAT_API_KEY")
	model := os.Getenv("LEIA_ANTHROPIC_COMPAT_MODEL")
	if endpoint == "" || apiKey == "" || model == "" {
		t.Skip("set LEIA_ANTHROPIC_COMPAT_BASE_URL, LEIA_ANTHROPIC_COMPAT_API_KEY, and LEIA_ANTHROPIC_COMPAT_MODEL")
	}
	provider := anthropic.Provider{
		Endpoint:     endpoint,
		APIKey:       apiKey,
		Model:        model,
		Timeout:      45 * time.Second,
		MaxAttempts:  2,
		RetryBackoff: 500 * time.Millisecond,
	}
	fmt.Printf("endpoint=%s\n", endpoint)
	fmt.Printf("model=%s\n", model)
	for _, tc := range []struct {
		name      string
		system    string
		user      string
		maxTokens int64
	}{
		{
			name:      "exact_text",
			system:    "You are a concise test assistant. Return plain text only.",
			user:      "Reply with exactly: leia ai dialect ok",
			maxTokens: 32,
		},
		{
			name:      "model_identity",
			system:    "You are a concise test assistant. Return plain text only.",
			user:      "What model are you? Answer in one short sentence.",
			maxTokens: 64,
		},
		{
			name:      "json_answer",
			system:    "Return only valid compact JSON. Do not wrap it in Markdown.",
			user:      `Return {"language":"leia","ai_dialect":true,"provider":"anthropic-compatible"}.`,
			maxTokens: 96,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			temperature := 0.0
			fmt.Printf("case=%s\n", tc.name)
			fmt.Printf("system=%q\n", tc.system)
			fmt.Printf("user=%q\n", tc.user)
			res, err := provider.Turn(context.Background(), llm.TurnRequest{
				Messages: []llm.Message{
					{Role: "system", Text: tc.system},
					{Role: "user", Text: tc.user},
				},
				MaxTokens:   tc.maxTokens,
				Temperature: &temperature,
			})
			if err != nil {
				t.Fatalf("Turn failed: %v", err)
			}
			fmt.Printf("status=%s reason=%s input_tokens=%d output_tokens=%d\n",
				res.Status, res.Reason, res.Usage.InputTokens, res.Usage.OutputTokens)
			fmt.Printf("text=%q\n", res.Text)
			if res.Text == "" {
				t.Fatalf("empty response: %#v", res)
			}
		})
	}
}

// TestLLMSyntaxAnthropicCompatibleLLMIntegration verifies that the LLM stdlib
// path can construct the same gated real-provider adapter. It is intentionally
// skipped unless all real-provider environment variables are set.
func TestLLMSyntaxAnthropicCompatibleLLMIntegration(t *testing.T) {
	if os.Getenv("LEIA_LLM_INTEGRATION") == "" {
		t.Skip("set LEIA_LLM_INTEGRATION=1 to run real provider smoke")
	}
	endpoint := os.Getenv("LEIA_ANTHROPIC_COMPAT_BASE_URL")
	apiKey := os.Getenv("LEIA_ANTHROPIC_COMPAT_API_KEY")
	model := os.Getenv("LEIA_ANTHROPIC_COMPAT_MODEL")
	if endpoint == "" || apiKey == "" || model == "" {
		t.Skip("set LEIA_ANTHROPIC_COMPAT_BASE_URL, LEIA_ANTHROPIC_COMPAT_API_KEY, and LEIA_ANTHROPIC_COMPAT_MODEL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	vm := leia.New(leia.WithLibs(leia.LibString | leia.LibOS | leia.LibLLM))
	if err := vm.ExecContext(ctx, `
llm.register_models({
    default: "smoke"
    smoke: {
        protocol: "anthropic_compatible"
        base_url: os.getenv("LEIA_ANTHROPIC_COMPAT_BASE_URL")
        api_key: os.getenv("LEIA_ANTHROPIC_COMPAT_API_KEY")
        provider_model: os.getenv("LEIA_ANTHROPIC_COMPAT_MODEL")
    }
})

result, err := llm.turn({
    messages: [
        llm.system("You are a concise test assistant. Return plain text only."),
        llm.user("Reply with exactly: leia llm provider ok"),
    ]
    max_tokens: 32
    temperature: 0
})
smoke_text := result.text
`); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}
	got, err := vm.Get("smoke_text")
	if err != nil {
		t.Fatalf("Get smoke_text: %v", err)
	}
	text, ok := got.(string)
	if !ok || text == "" {
		t.Fatalf("smoke_text = %#v, want non-empty string", got)
	}
	fmt.Printf("endpoint=%s\n", endpoint)
	fmt.Printf("model=%s\n", model)
	fmt.Printf("text=%q\n", text)
}
