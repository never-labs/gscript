package gscript_test

import (
	gs "github.com/never-labs/gscript"
	"github.com/never-labs/gscript/llm"
	"testing"
)

func TestLLMDirectTurnResponseFormatProviderRequest(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: `{"name":"Ada","email":"ada@example.com"}`}}
			vm := gs.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
result, err := turn {
    model: "mock-json"
    messages: messages {
        system: "Return only valid JSON."
        user: "Extract the contact."
    }
    response_format: {
        type: "json_schema"
        json_schema: {
            name: "contact"
            schema: {
                type: "object"
                properties: {
                    name: {type: "string"}
                    email: {type: "string"}
                }
                required: ["name", "email"]
                additionalProperties: false
            }
        }
    }
}
text := result.text
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			req := provider.requests[0]
			if req.Model != "mock-json" {
				t.Fatalf("model = %q, want mock-json", req.Model)
			}
			if len(req.Messages) != 2 ||
				req.Messages[0].Role != "system" || req.Messages[0].Text != "Return only valid JSON." ||
				req.Messages[1].Role != "user" || req.Messages[1].Text != "Extract the contact." {
				t.Fatalf("messages = %#v", req.Messages)
			}
			format, ok := req.ResponseFormat.(map[string]any)
			if !ok || format["type"] != "json_schema" {
				t.Fatalf("response_format = %#v", req.ResponseFormat)
			}
			jsonSchema, ok := format["json_schema"].(map[string]any)
			if !ok || jsonSchema["name"] != "contact" {
				t.Fatalf("json_schema = %#v", format["json_schema"])
			}
			schema, ok := jsonSchema["schema"].(map[string]any)
			if !ok || schema["type"] != "object" || schema["additionalProperties"] != false {
				t.Fatalf("schema = %#v", jsonSchema["schema"])
			}
			properties, ok := schema["properties"].(map[string]any)
			if !ok {
				t.Fatalf("properties = %#v", schema["properties"])
			}
			name, ok := properties["name"].(map[string]any)
			if !ok || name["type"] != "string" {
				t.Fatalf("name property = %#v", properties["name"])
			}
			required, ok := schema["required"].([]any)
			if !ok || len(required) != 2 || required[0] != "name" || required[1] != "email" {
				t.Fatalf("required = %#v", schema["required"])
			}
			text, err := vm.Get("text")
			if err != nil {
				t.Fatalf("Get text: %v", err)
			}
			if text != `{"name":"Ada","email":"ada@example.com"}` {
				t.Fatalf("text = %#v", text)
			}
		})
	}
}

func TestLLMAgentOutputStructuredValue(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: `{"name":"Ada","email":"ada@example.com"}`}}
			vm := gs.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
agent extract_contact(text) {
    model: "mock-json"
    system: "Extract contact information."
    user: text
    output: {
        name: "Ada Lovelace"
        email: "ada@example.com"
    }
}

result, err := extract_contact("Ada <ada@example.com>")
name := result.value.name
email := result.value.email
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
			name, err := vm.Get("name")
			if err != nil {
				t.Fatalf("Get name: %v", err)
			}
			if name != "Ada" {
				t.Fatalf("name = %#v, want Ada", name)
			}
			email, err := vm.Get("email")
			if err != nil {
				t.Fatalf("Get email: %v", err)
			}
			if email != "ada@example.com" {
				t.Fatalf("email = %#v, want ada@example.com", email)
			}
		})
	}
}

func TestLLMAgentOutputKeepsExplicitResponseFormat(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: `{"ok":true}`}}
			vm := gs.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
agent extract(text) {
    model: "mock-json"
    user: text
    output: {ok: true}
    response_format: {type: "json_schema", name: "explicit"}
}

result, err := extract("ok")
ok := result.value.ok
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}

			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			format, ok := provider.requests[0].ResponseFormat.(map[string]any)
			if !ok || format["type"] != "json_schema" || format["name"] != "explicit" {
				t.Fatalf("response_format = %#v", provider.requests[0].ResponseFormat)
			}
			value, err := vm.Get("ok")
			if err != nil {
				t.Fatalf("Get ok: %v", err)
			}
			if value != true {
				t.Fatalf("ok = %#v, want true", value)
			}
		})
	}
}

func TestLLMCustomFlowDoesNotAutoValidateOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: `not json`}}
			vm := gs.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
agent extract(text) {
    model: "mock-json"
    user: text
    output: {name: "Ada"}
} flow {
    result, err := turn {}
    return result, err
}

result, err := extract("Ada")
text := result.text
err_is_nil := err == nil
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
			text, err := vm.Get("text")
			if err != nil {
				t.Fatalf("Get text: %v", err)
			}
			errIsNil, err := vm.Get("err_is_nil")
			if err != nil {
				t.Fatalf("Get err_is_nil: %v", err)
			}
			if text != "not json" || errIsNil != true {
				t.Fatalf("text=%#v err_is_nil=%#v, want unvalidated flow result", text, errIsNil)
			}
		})
	}
}
