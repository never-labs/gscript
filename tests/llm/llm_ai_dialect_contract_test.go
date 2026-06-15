package leia_test

import (
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func aiDialectContractSource() string {
	return `
model {
    default: "mock-supervisor"
    json: {provider_model: "mock-json"}
}

preflight, preflight_err := turn {
    model: "json"
    messages: [
        prompt { role: "system", text: "Return a compact JSON contract verdict." },
        prompt { role: "user", text: "Check AI dialect prompt messages." },
    ]
    response_format: {
        type: "json_schema"
        json_schema: {
            name: "contract_verdict"
            schema: {
                type: "object"
                properties: {
                    ok: {type: "boolean"}
                    note: {type: "string"}
                }
                required: ["ok", "note"]
                additionalProperties: false
            }
        }
    }
}

extractor := agent {
    name: "extractor"
    model: "json"
    instructions: prompt { role: "system", text: "Extract a structured delegation result." }
    params: ["topic"]
    output: {
        summary: "short finding"
        confidence: 1
    }
    max_steps: 1
}

delegate_extract := llm.agent_as_tool(extractor, {
    name: "delegate_extract"
    description: "Delegate extraction to the structured agent."
    requires: ["none"]
})

supervisor := agent {
    name: "supervisor"
    model: "mock-supervisor"
    instructions: prompt { role: "system", text: "Use delegate_extract before answering." }
    tools: [delegate_extract]
    params: ["question"]
    max_steps: 2
}

answer, answer_err := supervisor("Need AI dialect contract coverage.")

preflight_text := preflight.text
answer_text := answer.text
tool_summary := answer.history[4].value.summary
tool_confidence := answer.history[4].value.confidence
`
}

func TestAIDialectContractPromptAgentToolOutputRecordReplay(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: `{"ok":true,"note":"prompt messages accepted"}`},
				{
					Status: "tool_calls",
					Calls: []llm.ToolCall{{
						ID:   "call_delegate_extract_1",
						Tool: "delegate_extract",
						Args: map[string]any{"topic": "prompt messages"},
					}},
				},
				{Status: "final_answer", Text: `{"summary":"prompt blocks became messages","confidence":0.94}`},
				{Status: "final_answer", Text: "Contract covered."},
			}}
			recorder := llm.NewRecorder()
			recordOpts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM | leia.LibDialect),
				leia.WithLLMProvider(provider),
				leia.WithLLMRecorder(recorder.Record),
			}, tc.opts...)
			vm := leia.New(recordOpts...)
			if err := vm.Exec(aiDialectContractSource()); err != nil {
				t.Fatalf("record Exec: %v", err)
			}
			if len(provider.requests) != 4 {
				t.Fatalf("requests = %#v, want four provider turns", provider.requests)
			}

			preflight := provider.requests[0]
			if preflight.Model != "mock-json" {
				t.Fatalf("preflight model = %q, want provider model mock-json", preflight.Model)
			}
			if len(preflight.Messages) != 2 ||
				preflight.Messages[0].Role != "system" || preflight.Messages[0].Text != "Return a compact JSON contract verdict." ||
				preflight.Messages[1].Role != "user" || preflight.Messages[1].Text != "Check AI dialect prompt messages." {
				t.Fatalf("preflight messages = %#v", preflight.Messages)
			}
			format, ok := preflight.ResponseFormat.(map[string]any)
			if !ok || format["type"] != "json_schema" {
				t.Fatalf("preflight response_format = %#v", preflight.ResponseFormat)
			}
			jsonSchema, ok := format["json_schema"].(map[string]any)
			if !ok || jsonSchema["name"] != "contract_verdict" {
				t.Fatalf("preflight json_schema = %#v", format["json_schema"])
			}

			supervisorFirst := provider.requests[1]
			if supervisorFirst.Model != "mock-supervisor" || len(supervisorFirst.Tools) != 1 {
				t.Fatalf("supervisor first request = %#v", supervisorFirst)
			}
			if len(supervisorFirst.Messages) != 2 ||
				supervisorFirst.Messages[0].Role != "system" || supervisorFirst.Messages[0].Text != "Use delegate_extract before answering." ||
				supervisorFirst.Messages[1].Role != "user" || supervisorFirst.Messages[1].Text != "Need AI dialect contract coverage." {
				t.Fatalf("supervisor first messages = %#v", supervisorFirst.Messages)
			}
			tool := supervisorFirst.Tools[0]
			if tool.Name != "delegate_extract" ||
				tool.Description != "Delegate extraction to the structured agent." ||
				len(tool.Params) != 1 || tool.Params[0] != "topic" ||
				len(tool.Requires) != 1 || tool.Requires[0] != "none" ||
				tool.Schema != nil {
				t.Fatalf("delegate tool metadata = %#v", tool)
			}

			extractorReq := provider.requests[2]
			if extractorReq.Model != "mock-json" || len(extractorReq.Tools) != 0 {
				t.Fatalf("extractor request = %#v", extractorReq)
			}
			if len(extractorReq.Messages) != 2 ||
				extractorReq.Messages[0].Role != "system" || extractorReq.Messages[0].Text != "Extract a structured delegation result." ||
				extractorReq.Messages[1].Role != "user" || extractorReq.Messages[1].Text != "prompt messages" {
				t.Fatalf("extractor messages = %#v", extractorReq.Messages)
			}
			extractorFormat, ok := extractorReq.ResponseFormat.(map[string]any)
			if !ok || extractorFormat["type"] != "json_object" {
				t.Fatalf("extractor response_format = %#v", extractorReq.ResponseFormat)
			}

			supervisorFinal := provider.requests[3]
			if len(supervisorFinal.Messages) != 4 ||
				supervisorFinal.Messages[2].Role != "assistant" ||
				supervisorFinal.Messages[2].ToolCall == nil ||
				supervisorFinal.Messages[2].ToolCall.ID != "call_delegate_extract_1" ||
				supervisorFinal.Messages[3].Role != "tool" ||
				supervisorFinal.Messages[3].ToolUseID != "call_delegate_extract_1" {
				t.Fatalf("supervisor final messages = %#v", supervisorFinal.Messages)
			}
			toolValue, ok := supervisorFinal.Messages[3].Value.(map[string]any)
			if !ok || toolValue["summary"] != "prompt blocks became messages" || toolValue["confidence"] != 0.94 {
				t.Fatalf("tool result value = %#v", supervisorFinal.Messages[3].Value)
			}

			for name, want := range map[string]any{
				"preflight_text":  `{"ok":true,"note":"prompt messages accepted"}`,
				"answer_text":     "Contract covered.",
				"tool_summary":    "prompt blocks became messages",
				"tool_confidence": 0.94,
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}

			records := recorder.Records()
			if len(records) != 4 {
				t.Fatalf("records = %#v, want four", records)
			}
			replayOpts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM | leia.LibDialect),
				leia.WithLLMReplay(records),
			}, tc.opts...)
			replayVM := leia.New(replayOpts...)
			if err := replayVM.Exec(aiDialectContractSource()); err != nil {
				t.Fatalf("replay Exec: %v", err)
			}
			for name, want := range map[string]any{
				"preflight_text":  `{"ok":true,"note":"prompt messages accepted"}`,
				"answer_text":     "Contract covered.",
				"tool_summary":    "prompt blocks became messages",
				"tool_confidence": 0.94,
			} {
				got, err := replayVM.Get(name)
				if err != nil {
					t.Fatalf("replay Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("replay %s = %#v, want %#v", name, got, want)
				}
			}
		})
	}
}

func TestAIDialectContractReplayMismatchSurfacesProviderError(t *testing.T) {
	records := []llm.Record{{
		Request: llm.TurnRequest{
			Model: "mock-json",
			Messages: []llm.Message{
				{Role: "system", Text: "Return a compact JSON contract verdict."},
				{Role: "user", Text: "Check AI dialect prompt messages."},
			},
			ResponseFormat: map[string]any{
				"type": "json_schema",
				"json_schema": map[string]any{
					"name": "contract_verdict",
					"schema": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"ok":   map[string]any{"type": "boolean"},
							"note": map[string]any{"type": "string"},
						},
						"required": []any{"ok", "note"},
					},
				},
			},
		},
		Result: llm.TurnResult{Status: "final_answer", Text: `{"ok":true,"note":"recorded"}`},
	}}
	source := strings.Replace(`
model {
    json: {provider_model: "mock-json"}
}

preflight, preflight_err := turn {
    model: "json"
    messages: [
        prompt { role: "system", text: "Return a compact JSON contract verdict." },
        prompt { role: "user", text: "Check AI dialect prompt messages." },
    ]
    response_format: {
        type: "json_schema"
        json_schema: {
            name: "contract_verdict"
            schema: {
                type: "object"
                properties: {
                    ok: {type: "boolean"}
                    note: {type: "string"}
                }
                required: ["ok", "note"]
                additionalProperties: false
            }
        }
    }
}
preflight_err_kind := preflight_err.kind
preflight_err_message := preflight_err.message
`, "Check AI dialect prompt messages.", "Check AI dialect prompt messages changed.", 1)
	vm := leia.New(
		leia.WithLibs(leia.LibString|leia.LibLLM|leia.LibDialect),
		leia.WithLLMReplay(records),
	)
	if err := vm.Exec(source); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	kind, err := vm.Get("preflight_err_kind")
	if err != nil {
		t.Fatalf("Get preflight_err_kind: %v", err)
	}
	if kind != "provider" {
		t.Fatalf("preflight_err_kind = %#v, want provider", kind)
	}
	message, err := vm.Get("preflight_err_message")
	if err != nil {
		t.Fatalf("Get preflight_err_message: %v", err)
	}
	if !strings.Contains(message.(string), "llm replay") || !strings.Contains(message.(string), "mismatch") {
		t.Fatalf("preflight_err_message = %#v, want replay mismatch", message)
	}
}
