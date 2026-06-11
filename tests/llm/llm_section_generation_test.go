package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMSectionsGenerateStructuredResults(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: `{"headline":"Launch is ready","confidence":0.91}`},
				{Status: "final_answer", Text: `{"risk":"Low","owner":"ops"}`},
			}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
lookup := llm.tool("lookup", func(query) {
    return "found:" .. query, nil
}, {
    params: {"query"}
    description: "Lookup shared evidence."
})

generated, err := llm.sections({
    model: "mock-json"
    messages: {
        llm.system("Use the provided evidence and return JSON.")
        llm.user("Project: reusable generation helpers.")
    }
    evidence: "Evidence: launch checklist is complete."
    tools: {lookup}
    sections: {
        {
            name: "summary"
            instructions: "Create the summary section."
            output: {
                headline: "Short headline"
                confidence: 0.5
            }
        }
        {
            name: "risk"
            prompt: "Create the risk section."
            output: {
                risk: "Low"
                owner: "team"
            }
        }
    }
})

summary_headline := generated.values.summary.headline
summary_confidence := generated.sections[1].value.confidence
risk_owner := generated.values.risk.owner
risk_text := generated.sections[2].text
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 2 {
				t.Fatalf("requests = %d, want 2", len(provider.requests))
			}
			for i, req := range provider.requests {
				if req.Model != "mock-json" {
					t.Fatalf("request %d model = %q", i, req.Model)
				}
				if len(req.Tools) != 1 || req.Tools[0].Name != "lookup" {
					t.Fatalf("request %d tools = %#v", i, req.Tools)
				}
				if len(req.Messages) != 4 ||
					req.Messages[0].Role != "system" || req.Messages[0].Text != "Use the provided evidence and return JSON." ||
					req.Messages[1].Role != "user" || req.Messages[1].Text != "Project: reusable generation helpers." ||
					req.Messages[2].Role != "user" || req.Messages[2].Text != "Evidence: launch checklist is complete." {
					t.Fatalf("request %d messages = %#v", i, req.Messages)
				}
				format, ok := req.ResponseFormat.(map[string]any)
				if !ok || format["type"] != "json_object" {
					t.Fatalf("request %d response_format = %#v", i, req.ResponseFormat)
				}
			}
			if provider.requests[0].Messages[3].Text != "Create the summary section." {
				t.Fatalf("summary prompt = %#v", provider.requests[0].Messages)
			}
			if provider.requests[1].Messages[3].Text != "Create the risk section." {
				t.Fatalf("risk prompt = %#v", provider.requests[1].Messages)
			}
			summaryHeadline, err := vm.Get("summary_headline")
			if err != nil {
				t.Fatalf("Get summary_headline: %v", err)
			}
			if summaryHeadline != "Launch is ready" {
				t.Fatalf("summary_headline = %#v", summaryHeadline)
			}
			summaryConfidence, err := vm.Get("summary_confidence")
			if err != nil {
				t.Fatalf("Get summary_confidence: %v", err)
			}
			if summaryConfidence != 0.91 {
				t.Fatalf("summary_confidence = %#v", summaryConfidence)
			}
			riskOwner, err := vm.Get("risk_owner")
			if err != nil {
				t.Fatalf("Get risk_owner: %v", err)
			}
			if riskOwner != "ops" {
				t.Fatalf("risk_owner = %#v", riskOwner)
			}
			riskText, err := vm.Get("risk_text")
			if err != nil {
				t.Fatalf("Get risk_text: %v", err)
			}
			if riskText != "{\"risk\":\"Low\",\"owner\":\"ops\"}" {
				t.Fatalf("risk_text = %#v", riskText)
			}
		})
	}
}

func TestLLMSectionsPropagatesSectionErrors(t *testing.T) {
	provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: `not json`}}
	vm := leia.New(llmScenarioOptions(provider)...)

	if err := vm.Exec(`
generated, err := llm.generate_sections({
    model: "mock-json"
    user: "Generate one section."
    sections: {
        {
            name: "bad"
            instructions: "Return structured data."
            output: {ok: true}
        }
    }
})

err_kind := err.kind
err_message := err.message
generated_is_nil := generated == nil
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	kind, err := vm.Get("err_kind")
	if err != nil {
		t.Fatalf("Get err_kind: %v", err)
	}
	if kind != "validation" {
		t.Fatalf("err_kind = %#v", kind)
	}
	generatedIsNil, err := vm.Get("generated_is_nil")
	if err != nil {
		t.Fatalf("Get generated_is_nil: %v", err)
	}
	if generatedIsNil != true {
		t.Fatalf("generated_is_nil = %#v", generatedIsNil)
	}
	message, err := vm.Get("err_message")
	if err != nil {
		t.Fatalf("Get err_message: %v", err)
	}
	if message == "" {
		t.Fatalf("err_message is empty")
	}
}
