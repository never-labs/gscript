package gscript_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	gs "github.com/gscript/gscript/gscript"
)

// TestAnthropicCompatibleLLMIntegration is a gated real-provider smoke test.
// It intentionally uses generic Anthropic-compatible configuration names so
// local wrappers or vendor-specific credentials do not become GScript API.
func TestAnthropicCompatibleLLMIntegration(t *testing.T) {
	if os.Getenv("GSCRIPT_LLM_INTEGRATION") == "" {
		t.Skip("set GSCRIPT_LLM_INTEGRATION=1 to run real Anthropic-compatible provider smoke")
	}
	endpoint := os.Getenv("GSCRIPT_ANTHROPIC_COMPAT_BASE_URL")
	apiKey := os.Getenv("GSCRIPT_ANTHROPIC_COMPAT_API_KEY")
	model := os.Getenv("GSCRIPT_ANTHROPIC_COMPAT_MODEL")
	if endpoint == "" || apiKey == "" || model == "" {
		t.Skip("set GSCRIPT_ANTHROPIC_COMPAT_BASE_URL, GSCRIPT_ANTHROPIC_COMPAT_API_KEY, and GSCRIPT_ANTHROPIC_COMPAT_MODEL")
	}
	temperature := 0.0
	provider := gs.AnthropicCompatibleLLMProvider{
		Endpoint:     endpoint,
		APIKey:       apiKey,
		Model:        model,
		Timeout:      45 * time.Second,
		MaxAttempts:  2,
		RetryBackoff: 500 * time.Millisecond,
	}
	res, err := provider.Turn(context.Background(), gs.LLMTurnRequest{
		Messages: []gs.LLMMessage{
			{Role: "system", Text: "You are a concise test assistant. Return plain text only."},
			{Role: "user", Text: "Reply with exactly: gscript llm native ok"},
		},
		MaxTokens:   32,
		Temperature: &temperature,
	})
	if err != nil {
		t.Fatalf("Turn failed: %v", err)
	}
	fmt.Printf("endpoint=%s\n", endpoint)
	fmt.Printf("model=%s\n", model)
	fmt.Printf("status=%s reason=%s input_tokens=%d output_tokens=%d\n",
		res.Status, res.Reason, res.Usage.InputTokens, res.Usage.OutputTokens)
	fmt.Printf("text=%q\n", res.Text)
	if res.Text == "" {
		t.Fatalf("empty response: %#v", res)
	}
}
