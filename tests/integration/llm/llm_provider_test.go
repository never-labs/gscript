package gscript_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	gs "github.com/never-labs/gscript"
)

func TestLLMCommandProvider(t *testing.T) {
	vm := gs.New(
		gs.WithLibs(gs.LibString|gs.LibLLM),
		gs.WithLLMCommand("sh", "-c", `printf 'mock:%s' "$0"`),
	)
	if err := vm.Exec(`
result, err := llm.turn({messages: {llm.user("hello")}})
text := result.text
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	got, err := vm.Get("text")
	if err != nil {
		t.Fatalf("Get text: %v", err)
	}
	if got != "mock:User: hello" {
		t.Fatalf("text = %#v", got)
	}
}

func TestAnthropicCompatibleLLMProvider(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if key := r.Header.Get("x-api-key"); key != "test-key" {
			t.Fatalf("x-api-key = %q", key)
		}
		if version := r.Header.Get("anthropic-version"); version == "" {
			t.Fatal("missing anthropic-version")
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("Decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
  "type": "message",
  "role": "assistant",
  "stop_reason": "tool_use",
  "content": [{
    "type": "tool_use",
    "id": "toolu_1",
    "name": "lookup",
    "input": {"name": "gscript", "limit": 3}
  }],
  "usage": {"input_tokens": 9, "output_tokens": 6}
}`)
	}))
	defer server.Close()
	provider := gs.AnthropicCompatibleLLMProvider{
		Endpoint: server.URL,
		APIKey:   "test-key",
		Model:    "fallback-model",
		Client:   server.Client(),
	}
	res, err := provider.Turn(context.Background(), gs.LLMTurnRequest{
		Model: "claude-test",
		Messages: []gs.LLMMessage{
			{Role: "system", Text: "short"},
			{Role: "user", Text: "find docs"},
		},
		Tools: []gs.LLMTool{{
			Name:        "lookup",
			Description: "lookup docs",
			Params:      []string{"name"},
		}},
		ForceTool: "lookup",
		MaxTokens: 32,
	})
	if err != nil {
		t.Fatalf("Turn: %v", err)
	}
	if got["model"] != "claude-test" || got["system"] != "short" || got["max_tokens"] != float64(32) {
		t.Fatalf("request scalar fields = %#v", got)
	}
	messages := got["messages"].([]any)
	if messages[0].(map[string]any)["role"] != "user" || messages[0].(map[string]any)["content"] != "find docs" {
		t.Fatalf("messages = %#v", messages)
	}
	tools := got["tools"].([]any)
	if tools[0].(map[string]any)["name"] != "lookup" {
		t.Fatalf("tools = %#v", tools)
	}
	choice := got["tool_choice"].(map[string]any)
	if choice["type"] != "tool" || choice["name"] != "lookup" {
		t.Fatalf("tool_choice = %#v", got["tool_choice"])
	}
	if res.Status != "tool_calls" || res.Usage.InputTokens != 9 || res.Usage.OutputTokens != 6 {
		t.Fatalf("result = %#v", res)
	}
	if len(res.Calls) != 1 || res.Calls[0].Tool != "lookup" || res.Calls[0].Args["limit"] != int64(3) {
		t.Fatalf("calls = %#v", res.Calls)
	}
}

func TestClassifyLLMProviderError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", err: nil, want: ""},
		{name: "openai auth", err: &gs.OpenAICompatibleLLMError{StatusCode: http.StatusUnauthorized}, want: gs.LLMProviderErrorAuth},
		{name: "anthropic rate limit", err: &gs.AnthropicCompatibleLLMError{StatusCode: http.StatusTooManyRequests, Retryable: true}, want: gs.LLMProviderErrorRateLimit},
		{name: "retryable status", err: &gs.OpenAICompatibleLLMError{StatusCode: http.StatusBadGateway, Retryable: true}, want: gs.LLMProviderErrorNetwork},
		{name: "request status", err: &gs.AnthropicCompatibleLLMError{StatusCode: http.StatusUnprocessableEntity}, want: gs.LLMProviderErrorRequest},
		{name: "context", err: context.DeadlineExceeded, want: gs.LLMProviderErrorNetwork},
		{name: "net", err: &net.DNSError{Err: "no such host", Name: "provider.test"}, want: gs.LLMProviderErrorNetwork},
		{name: "generic", err: errors.New("provider rejected request"), want: gs.LLMProviderErrorProvider},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := gs.ClassifyLLMProviderError(tc.err); got != tc.want {
				t.Fatalf("ClassifyLLMProviderError(%T) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestLLMModelsProviderConfigPreservesHostProvider(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: "host"}}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, mode.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
models {
    default: "chat"
    chat: {
        protocol: "openai_compatible"
        base_url: "http://127.0.0.1:1"
        api_key: ("test" .. "-key")
        provider_model: "host-model"
    }
}

result, err := llm.turn({model: "chat", messages: {llm.user("hello")}})
text := result.text
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			if provider.requests[0].Model != "host-model" {
				t.Fatalf("model = %q", provider.requests[0].Model)
			}
			text, err := vm.Get("text")
			if err != nil {
				t.Fatalf("Get text: %v", err)
			}
			if text != "host" {
				t.Fatalf("text = %#v", text)
			}
		})
	}
}
