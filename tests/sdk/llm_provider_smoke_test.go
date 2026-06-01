package gscript_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	gs "github.com/never-labs/gscript"
	"github.com/never-labs/gscript/llm"
	llmcommand "github.com/never-labs/gscript/llm/command"
	llmopenai "github.com/never-labs/gscript/llm/openai"
)

type sdkSmokeLLMProvider struct {
	last llm.TurnRequest
}

func (p *sdkSmokeLLMProvider) Turn(_ context.Context, req llm.TurnRequest) (llm.TurnResult, error) {
	p.last = req
	return llm.TurnResult{Status: "final_answer", Text: "sdk smoke"}, nil
}

func TestWithLLMProviderSDKSmoke(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &sdkSmokeLLMProvider{}
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
result, err := llm.turn({
    model: "sdk-smoke",
    messages: {llm.user("hello")},
})
text := result.text
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			text, err := vm.Get("text")
			if err != nil {
				t.Fatalf("Get text: %v", err)
			}
			if text != "sdk smoke" {
				t.Fatalf("text = %#v", text)
			}
			if provider.last.Model != "sdk-smoke" || len(provider.last.Messages) != 1 || provider.last.Messages[0].Text != "hello" {
				t.Fatalf("provider request = %#v", provider.last)
			}
		})
	}
}

func TestLLMProviderSubpackageSDKSmoke(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/chat/completions" {
					t.Fatalf("path = %q", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"sdk openai"}}]}`)
			}))
			defer server.Close()

			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(llmopenai.Provider{
					Endpoint: server.URL,
					Model:    "sdk-openai",
					Client:   server.Client(),
				}),
			}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
result, err := llm.turn({messages: {llm.user("hello")}})
text := result.text
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			text, err := vm.Get("text")
			if err != nil {
				t.Fatalf("Get text: %v", err)
			}
			if text != "sdk openai" {
				t.Fatalf("text = %#v", text)
			}
		})
	}
}

func TestLLMCommandSubpackageSDKSmoke(t *testing.T) {
	vm := gs.New(
		gs.WithLibs(gs.LibString|gs.LibLLM),
		gs.WithLLMProvider(llmcommand.Provider{
			Command: "sh",
			Args:    []string{"-c", `printf 'sdk command:%s' "$0"`},
		}),
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
	if got != "sdk command:User: hello" {
		t.Fatalf("text = %#v", got)
	}
}

func TestLLMWithoutProviderSDKSmoke(t *testing.T) {
	vm := gs.New(gs.WithLibs(gs.LibString | gs.LibLLM))
	if err := vm.Exec(`
result, err := llm.turn({messages: {llm.user("hi")}})
kind := err.kind
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	got, err := vm.Get("kind")
	if err != nil {
		t.Fatalf("Get kind: %v", err)
	}
	if got != "provider" {
		t.Fatalf("kind = %#v", got)
	}
}
