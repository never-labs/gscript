package gscript_test

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
	"time"

	gs "github.com/never-labs/gscript/gscript"
)

func float64Ptr(v float64) *float64 {
	return &v
}

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

func TestOpenAICompatibleLLMProvider(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Fatalf("Authorization = %q", auth)
		}
		if header := r.Header.Get("X-Test"); header != "ok" {
			t.Fatalf("X-Test = %q", header)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("Decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
  "choices": [{
    "finish_reason": "tool_calls",
    "message": {
      "role": "assistant",
      "tool_calls": [{
        "id": "call_1",
        "type": "function",
        "function": {"name": "lookup", "arguments": "{\"name\":\"gscript\",\"limit\":3}"}
      }]
    }
  }],
  "usage": {"prompt_tokens": 7, "completion_tokens": 5}
}`)
	}))
	defer server.Close()
	provider := gs.OpenAICompatibleLLMProvider{
		Endpoint: server.URL,
		APIKey:   "test-key",
		Model:    "fallback-model",
		Client:   server.Client(),
		Headers:  map[string]string{"X-Test": "ok"},
	}
	res, err := provider.Turn(context.Background(), gs.LLMTurnRequest{
		Model: "mock-fast",
		Messages: []gs.LLMMessage{
			{Role: "system", Text: "short"},
			{Role: "user", Text: "find docs"},
		},
		Tools: []gs.LLMTool{{
			Name:        "lookup",
			Description: "lookup docs",
			Params:      []string{"name"},
		}},
		ForceTool:   "lookup",
		MaxTokens:   32,
		Temperature: float64Ptr(0.25),
		TopP:        float64Ptr(0.9),
		ResponseFormat: map[string]any{
			"type": "json_object",
		},
		Stop:     []string{"END"},
		Metadata: map[string]string{"trace_id": "abc"},
	})
	if err != nil {
		t.Fatalf("Turn: %v", err)
	}
	if got["model"] != "mock-fast" || got["max_tokens"] != float64(32) {
		t.Fatalf("request scalar fields = %#v", got)
	}
	if got["temperature"] != 0.25 || got["top_p"] != 0.9 {
		t.Fatalf("request sampling fields = %#v", got)
	}
	format := got["response_format"].(map[string]any)
	if format["type"] != "json_object" {
		t.Fatalf("response_format = %#v", got["response_format"])
	}
	messages := got["messages"].([]any)
	if messages[0].(map[string]any)["role"] != "system" || messages[1].(map[string]any)["content"] != "find docs" {
		t.Fatalf("messages = %#v", messages)
	}
	tools := got["tools"].([]any)
	fn := tools[0].(map[string]any)["function"].(map[string]any)
	if fn["name"] != "lookup" || fn["description"] != "lookup docs" {
		t.Fatalf("tools = %#v", tools)
	}
	choice := got["tool_choice"].(map[string]any)
	if choice["type"] != "function" || choice["function"].(map[string]any)["name"] != "lookup" {
		t.Fatalf("tool_choice = %#v", got["tool_choice"])
	}
	if res.Status != "tool_calls" || res.Usage.InputTokens != 7 || res.Usage.OutputTokens != 5 {
		t.Fatalf("result = %#v", res)
	}
	if len(res.Calls) != 1 || res.Calls[0].Tool != "lookup" || res.Calls[0].Args["limit"] != int64(3) {
		t.Fatalf("calls = %#v", res.Calls)
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

func TestOpenAICompatibleLLMProviderRetriesTransientStatus(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`)
	}))
	defer server.Close()
	provider := gs.OpenAICompatibleLLMProvider{
		Endpoint:    server.URL,
		Model:       "mock-fast",
		Client:      server.Client(),
		MaxAttempts: 2,
	}
	res, err := provider.Turn(context.Background(), gs.LLMTurnRequest{
		Messages: []gs.LLMMessage{{Role: "user", Text: "hello"}},
	})
	if err != nil {
		t.Fatalf("Turn: %v", err)
	}
	if attempts != 2 || res.Text != "ok" {
		t.Fatalf("attempts=%d result=%#v", attempts, res)
	}
}

func TestOpenAICompatibleLLMProviderTypedStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()
	provider := gs.OpenAICompatibleLLMProvider{
		Endpoint: server.URL,
		Model:    "mock-fast",
		Client:   server.Client(),
	}
	_, err := provider.Turn(context.Background(), gs.LLMTurnRequest{
		Messages: []gs.LLMMessage{{Role: "user", Text: "hello"}},
	})
	var statusErr *gs.OpenAICompatibleLLMError
	if !errors.As(err, &statusErr) {
		t.Fatalf("err = %T %v, want OpenAICompatibleLLMError", err, err)
	}
	if statusErr.StatusCode != http.StatusBadRequest || statusErr.Retryable || !strings.Contains(statusErr.Body, "bad request") {
		t.Fatalf("statusErr = %#v", statusErr)
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

func TestOpenAICompatibleLLMProviderTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()
	provider := gs.OpenAICompatibleLLMProvider{
		Endpoint: server.URL,
		Model:    "mock-fast",
		Client:   server.Client(),
		Timeout:  1 * time.Millisecond,
	}
	_, err := provider.Turn(context.Background(), gs.LLMTurnRequest{
		Messages: []gs.LLMMessage{{Role: "user", Text: "hello"}},
	})
	if err == nil {
		t.Fatal("Turn succeeded, want timeout")
	}
}

func TestWithOpenAICompatibleLLM(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`)
	}))
	defer server.Close()
	vm := gs.New(
		gs.WithLibs(gs.LibString|gs.LibLLM),
		gs.WithOpenAICompatibleLLM(server.URL, "", "mock-fast"),
	)
	if err := vm.Exec(`
result, err := llm.turn({messages: {llm.user("hello")}})
text := result.text
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	text, _ := vm.Get("text")
	if text != "ok" {
		t.Fatalf("text = %#v", text)
	}
}

func TestAINativeModelsProviderConfigOpenAICompatible(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			var got map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/chat/completions" {
					t.Fatalf("path = %q", r.URL.Path)
				}
				if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
					t.Fatalf("Authorization = %q", auth)
				}
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Fatalf("Decode request: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`)
			}))
			defer server.Close()

			opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(fmt.Sprintf(`
models {
    default: "chat"
    chat: {
        protocol: "openai_compatible"
        base_url: %q
        api_key: ("test" .. "-key")
        provider_model: "provider-fast"
    }
}

result, err := llm.turn({model: "chat", messages: {llm.user("hello")}})
text := result.text
`, server.URL))
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if got["model"] != "provider-fast" {
				t.Fatalf("model = %#v", got["model"])
			}
			text, err := vm.Get("text")
			if err != nil {
				t.Fatalf("Get text: %v", err)
			}
			if text != "ok" {
				t.Fatalf("text = %#v", text)
			}
		})
	}
}

func TestAINativeModelsProviderConfigPreservesHostProvider(t *testing.T) {
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
