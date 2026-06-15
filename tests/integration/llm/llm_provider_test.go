package leia_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
	"github.com/never-labs/leia/llm/anthropic"
	"github.com/never-labs/leia/llm/command"
	"github.com/never-labs/leia/llm/openai"
)

func TestLLMCommandProvider(t *testing.T) {
	vm := leia.New(
		leia.WithLibs(leia.LibString|leia.LibLLM),
		leia.WithLLMProvider(command.Provider{Command: "sh", Args: []string{"-c", `printf 'mock:%s' "$0"`}}),
	)
	if err := vm.Exec(`
result, err := llm.turn({messages: [llm.user("hello")]})
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
    "input": {"name": "leia", "limit": 3}
  }],
  "usage": {"input_tokens": 9, "output_tokens": 6}
}`)
	}))
	defer server.Close()
	provider := anthropic.Provider{
		Endpoint: server.URL,
		APIKey:   "test-key",
		Model:    "fallback-model",
		Client:   server.Client(),
	}
	res, err := provider.Turn(context.Background(), llm.TurnRequest{
		Model: "claude-test",
		Messages: []llm.Message{
			{Role: "system", Text: "short"},
			{Role: "user", Text: "find docs"},
		},
		Tools: []llm.Tool{{
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

func TestAnthropicCompatibleLLMProviderStreamsContent(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("Decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `event: message_start`)
		fmt.Fprintln(w, `data: {"type":"message_start","message":{"usage":{"input_tokens":3,"output_tokens":0}}}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `event: content_block_start`)
		fmt.Fprintln(w, `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `event: content_block_delta`)
		fmt.Fprintln(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `event: content_block_delta`)
		fmt.Fprintln(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" stream"}}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `event: message_delta`)
		fmt.Fprintln(w, `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `event: message_stop`)
		fmt.Fprintln(w, `data: {"type":"message_stop"}`)
	}))
	defer server.Close()
	provider := anthropic.Provider{
		Endpoint: server.URL,
		Model:    "mock-fast",
		Client:   server.Client(),
	}
	var tokens []string
	res, err := provider.StreamTurn(context.Background(), llm.TurnRequest{
		Messages: []llm.Message{{Role: "user", Text: "hello"}},
	}, func(event llm.StreamEvent) error {
		if event.Type == "token" {
			tokens = append(tokens, event.Token)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("StreamTurn: %v", err)
	}
	if got["stream"] != true {
		t.Fatalf("request stream = %#v", got["stream"])
	}
	if res.Status != "final_answer" || res.Text != "hello stream" || res.Reason != "end_turn" || res.Usage.InputTokens != 3 || res.Usage.OutputTokens != 2 {
		t.Fatalf("result = %#v", res)
	}
	if strings.Join(tokens, "|") != "hello| stream" {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestAnthropicCompatibleLLMProviderStreamsToolUse(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("Decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"lookup","input":{}}}`)
		fmt.Fprintln(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"query\":"}}`)
		fmt.Fprintln(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"leia\"}"}}`)
		fmt.Fprintln(w, `data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":4}}`)
		fmt.Fprintln(w, `data: {"type":"message_stop"}`)
	}))
	defer server.Close()
	provider := anthropic.Provider{
		Endpoint: server.URL,
		Model:    "mock-fast",
		Client:   server.Client(),
	}
	res, err := provider.StreamTurn(context.Background(), llm.TurnRequest{
		Messages: []llm.Message{{Role: "user", Text: "lookup"}},
		Tools:    []llm.Tool{{Name: "lookup", Params: []string{"query"}}},
	}, nil)
	if err != nil {
		t.Fatalf("StreamTurn: %v", err)
	}
	if got["stream"] != true {
		t.Fatalf("request stream = %#v", got["stream"])
	}
	if res.Status != "tool_calls" || len(res.Calls) != 1 {
		t.Fatalf("result = %#v", res)
	}
	if res.Calls[0].ID != "toolu_1" || res.Calls[0].Tool != "lookup" || res.Calls[0].Args["query"] != "leia" {
		t.Fatalf("calls = %#v", res.Calls)
	}
}

func TestClassifyProviderError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", err: nil, want: ""},
		{name: "openai auth", err: &openai.Error{StatusCode: http.StatusUnauthorized}, want: llm.ProviderErrorAuth},
		{name: "anthropic rate limit", err: &anthropic.Error{StatusCode: http.StatusTooManyRequests, Retryable: true}, want: llm.ProviderErrorRateLimit},
		{name: "retryable status", err: &openai.Error{StatusCode: http.StatusBadGateway, Retryable: true}, want: llm.ProviderErrorNetwork},
		{name: "request status", err: &anthropic.Error{StatusCode: http.StatusUnprocessableEntity}, want: llm.ProviderErrorRequest},
		{name: "context", err: context.DeadlineExceeded, want: llm.ProviderErrorNetwork},
		{name: "net", err: &net.DNSError{Err: "no such host", Name: "provider.test"}, want: llm.ProviderErrorNetwork},
		{name: "generic", err: errors.New("provider rejected request"), want: llm.ProviderErrorProvider},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := llm.ClassifyProviderError(tc.err); got != tc.want {
				t.Fatalf("ClassifyProviderError(%T) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestLLMModelsProviderConfigPreservesHostProvider(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: "host"}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, mode.opts...)
			vm := leia.New(opts...)
			err := vm.Exec(`
llm.register_models({
    default: "chat"
    chat: {
        protocol: "openai_compatible"
        base_url: "http://127.0.0.1:1"
        api_key: ("test" .. "-key")
        provider_model: "host-model"
    }
})

result, err := llm.turn({model: "chat", messages: [llm.user("hello")]})
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
