package leia_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
	"github.com/never-labs/leia/llm/openai"
)

func float64Ptr(v float64) *float64 {
	return &v
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
        "function": {"name": "lookup", "arguments": "{\"name\":\"leia\",\"limit\":3}"}
      }]
    }
  }],
  "usage": {"prompt_tokens": 7, "completion_tokens": 5}
}`)
	}))
	defer server.Close()
	provider := openai.Provider{
		Endpoint: server.URL,
		APIKey:   "test-key",
		Model:    "fallback-model",
		Client:   server.Client(),
		Headers:  map[string]string{"X-Test": "ok"},
	}
	res, err := provider.Turn(context.Background(), llm.TurnRequest{
		Model: "mock-fast",
		Messages: []llm.Message{
			{Role: "system", Text: "short"},
			{Role: "user", Text: "find docs"},
		},
		Tools: []llm.Tool{{
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
	provider := openai.Provider{
		Endpoint:    server.URL,
		Model:       "mock-fast",
		Client:      server.Client(),
		MaxAttempts: 2,
	}
	res, err := provider.Turn(context.Background(), llm.TurnRequest{
		Messages: []llm.Message{{Role: "user", Text: "hello"}},
	})
	if err != nil {
		t.Fatalf("Turn: %v", err)
	}
	if attempts != 2 || res.Text != "ok" {
		t.Fatalf("attempts=%d result=%#v", attempts, res)
	}
}

func TestOpenAICompatibleLLMProviderStreamsContent(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("Decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"role":"assistant"}}]}`)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"hello"}}]}`)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":" world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`)
		fmt.Fprintln(w, `data: [DONE]`)
	}))
	defer server.Close()
	provider := openai.Provider{
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
	if res.Status != "final_answer" || res.Text != "hello world" || res.Usage.InputTokens != 3 || res.Usage.OutputTokens != 2 {
		t.Fatalf("result = %#v", res)
	}
	if strings.Join(tokens, "|") != "hello| world" {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestOpenAICompatibleLLMProviderTypedStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()
	provider := openai.Provider{
		Endpoint: server.URL,
		Model:    "mock-fast",
		Client:   server.Client(),
	}
	_, err := provider.Turn(context.Background(), llm.TurnRequest{
		Messages: []llm.Message{{Role: "user", Text: "hello"}},
	})
	var statusErr *openai.Error
	if !errors.As(err, &statusErr) {
		t.Fatalf("err = %T %v, want OpenAICompatibleLLMError", err, err)
	}
	if statusErr.StatusCode != http.StatusBadRequest || statusErr.Retryable || !strings.Contains(statusErr.Body, "bad request") {
		t.Fatalf("statusErr = %#v", statusErr)
	}
}

func TestOpenAICompatibleLLMProviderTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()
	provider := openai.Provider{
		Endpoint: server.URL,
		Model:    "mock-fast",
		Client:   server.Client(),
		Timeout:  1 * time.Millisecond,
	}
	_, err := provider.Turn(context.Background(), llm.TurnRequest{
		Messages: []llm.Message{{Role: "user", Text: "hello"}},
	})
	if err == nil {
		t.Fatal("Turn succeeded, want timeout")
	}
}

func TestOpenAIProviderThroughEmbeddingOption(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`)
	}))
	defer server.Close()
	vm := leia.New(
		leia.WithLibs(leia.LibString|leia.LibLLM),
		leia.WithLLMProvider(openai.Provider{Endpoint: server.URL, Model: "mock-fast", Client: server.Client()}),
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

func TestLLMModelsProviderConfigOpenAICompatible(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
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

			opts := append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, mode.opts...)
			vm := leia.New(opts...)
			err := vm.Exec(fmt.Sprintf(`
llm.register_models({
    default: "chat"
    chat: {
        protocol: "openai_compatible"
        base_url: %q
        api_key: ("test" .. "-key")
        provider_model: "provider-fast"
    }
})

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
