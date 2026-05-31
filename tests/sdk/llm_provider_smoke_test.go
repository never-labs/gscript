package gscript_test

import (
	"context"
	"testing"

	gs "github.com/never-labs/gscript"
)

type sdkSmokeLLMProvider struct {
	last gs.LLMTurnRequest
}

func (p *sdkSmokeLLMProvider) Turn(_ context.Context, req gs.LLMTurnRequest) (gs.LLMTurnResult, error) {
	p.last = req
	return gs.LLMTurnResult{Status: "final_answer", Text: "sdk smoke"}, nil
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
