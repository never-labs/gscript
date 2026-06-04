package leia_test

import (
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMAgentOutputValidationError(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: `not json`}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
extract_contact := llm.agent("extract_contact", func(text) {
    return {
        model: "mock-json"
        user: text
        output: {name: "Ada"}
    }, nil
})

result, err := extract_contact("Ada")
err_kind := err.kind
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			format, ok := provider.requests[0].ResponseFormat.(map[string]any)
			if !ok || format["type"] != "json_object" {
				t.Fatalf("response_format = %#v", provider.requests[0].ResponseFormat)
			}
			kind, err := vm.Get("err_kind")
			if err != nil {
				t.Fatalf("Get err_kind: %v", err)
			}
			if kind != "validation" {
				t.Fatalf("err_kind = %#v, want validation", kind)
			}
		})
	}
}

func TestLLMAgentOutputValidationMissingField(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: `{"name":"Ada"}`}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
extract_contact := llm.agent("extract_contact", func(text) {
    return {
        model: "mock-json"
        user: text
        output: {
            name: "Ada"
            email: "ada@example.com"
        }
    }, nil
})

result, err := extract_contact("Ada")
err_kind := err.kind
err_message := err.message
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			kind, err := vm.Get("err_kind")
			if err != nil {
				t.Fatalf("Get err_kind: %v", err)
			}
			if kind != "validation" {
				t.Fatalf("err_kind = %#v, want validation", kind)
			}
			message, err := vm.Get("err_message")
			if err != nil {
				t.Fatalf("Get err_message: %v", err)
			}
			if !strings.Contains(message.(string), `missing field "email"`) {
				t.Fatalf("err_message = %#v, want missing email field", message)
			}
		})
	}
}

func TestLLMAgentOutputValidationTypeMismatch(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: `{"name":"Ada","score":"high","ok":true,"meta":{}}`}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
classify := llm.agent("classify", func(text) {
    return {
        model: "mock-json"
        user: text
        output: {
            name: "Ada"
            score: 1
            ok: true
            meta: {source: "email"}
        }
    }, nil
})

result, err := classify("Ada")
err_kind := err.kind
err_message := err.message
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			kind, err := vm.Get("err_kind")
			if err != nil {
				t.Fatalf("Get err_kind: %v", err)
			}
			if kind != "validation" {
				t.Fatalf("err_kind = %#v, want validation", kind)
			}
			message, err := vm.Get("err_message")
			if err != nil {
				t.Fatalf("Get err_message: %v", err)
			}
			if !strings.Contains(message.(string), `field "score" has type string, want number`) {
				t.Fatalf("err_message = %#v, want score type mismatch", message)
			}
		})
	}
}
