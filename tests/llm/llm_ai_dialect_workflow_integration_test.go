package leia_test

import (
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func aiDialectResearchAssistantWorkflowSource() string {
	return `
model {
    json: {provider_model: "mock-json"}
    supervisor: {provider_model: "mock-supervisor"}
    specialist: {provider_model: "mock-specialist"}
}

docs := llm.collection({
    llm.doc("Checkout runbook says payment queue owns sev2 checkout incidents and rollback requires on-call approval.", {
        id: "runbook"
        title: "Checkout runbook"
        source: "local/runbook"
        tags: {"checkout", "payments"}
    })
    llm.document({
        id: "notes"
        title: "Search notes"
        text: "Search indexing ownership is unrelated to checkout incidents."
        source: "local/notes"
    })
})

retrieved := llm.retrieve(docs, "checkout payment sev2 owner", {limit: 1 label: "Research memory"})

brief_schema := llm.schema({
    finding: {type: "string", description: "Evidence-backed finding"}
    owner: "string"
    confidence: "number"
})

decision_schema := llm.schema({
    decision: "string"
    risk: "string"
    owner: "string"
})

research_flow := llm.workflow({
    llm.step("collect", func(ctx) {
        return {
            value: retrieved
            text: "retrieved:" .. retrieved.matches[1].id
        }, nil
    })
    llm.step("brief", func(ctx) {
        return turn {
            model: "json"
            messages: [
                prompt { role: "system", text: "Write a structured research brief from retrieved memory." },
                prompt { role: "user", text: ctx.input },
                llm.context(retrieved.matches, {label: "Research memory"}),
            ]
            response_format: llm.output_schema("research_brief", brief_schema)
        }
    })
})

flow_result, flow_err := research_flow.run("checkout payment sev2")
brief_ok, brief_msg := llm.validate_output(flow_result.text, brief_schema)

sections_result, sections_err := llm.sections({
    model: "json"
    messages: [
        prompt { role: "system", text: "Draft operational decision sections as JSON." },
        prompt { role: "user", text: flow_result.text },
    ]
    context: llm.context(retrieved.matches, {label: "Section memory"})
    sections: {
        {
            name: "recommendation"
            instructions: "Return the decision, risk, and owner."
            output: {
                decision: "short decision"
                risk: "medium"
                owner: "team"
            }
        }
    }
})
decision_ok, decision_msg := llm.validate_output(sections_result.sections[1].text, decision_schema)

specialist := agent {
    name: "specialist"
    model: "specialist"
    instructions: prompt { role: "system", text: "Return structured delegated review." }
    params: ["topic"]
    output: {
        summary: "short finding"
        owner: "team"
        confidence: 1
    }
    max_steps: 1
}

handoff_review := llm.handoff(specialist, {
    name: "handoff_review"
    description: "Delegate the research brief to a specialist."
    requires: ["none"]
})

supervisor := agent {
    name: "supervisor"
    model: "supervisor"
    instructions: prompt { role: "system", text: "Use handoff_review before producing the final answer." }
    tools: [handoff_review]
    params: ["question"]
    max_steps: 2
}

supervisor_result, supervisor_err := supervisor("Review checkout incident ownership.")

first_match_id := retrieved.matches[1].id
flow_text := flow_result.text
flow_second_input := flow_result.steps[2].input
section_decision := sections_result.values.recommendation.decision
section_owner := sections_result.values.recommendation.owner
brief_validation := tostring(brief_ok) .. "|" .. brief_msg
decision_validation := tostring(decision_ok) .. "|" .. decision_msg
final_text := supervisor_result.text
delegated_summary := supervisor_result.history[4].value.summary
delegated_owner := supervisor_result.history[4].value.owner
delegated_confidence := supervisor_result.history[4].value.confidence
handoff_param := handoff_review.params[1]
handoff_output_summary := handoff_review.output.summary
`
}

func TestAIDialectResearchAssistantWorkflowIntegrationRecordReplay(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: `{"finding":"Payment queue owns sev2 checkout incidents","owner":"payment queue","confidence":0.93}`},
				{Status: "final_answer", Text: `{"decision":"route to payment queue","risk":"medium","owner":"payment queue"}`},
				{
					Status: "tool_calls",
					Calls: []llm.ToolCall{{
						ID:   "call_handoff_review_1",
						Tool: "handoff_review",
						Args: map[string]any{"topic": "checkout ownership"},
					}},
				},
				{Status: "final_answer", Text: `{"summary":"payment queue should own the sev2 handoff","owner":"payment queue","confidence":0.89}`},
				{Status: "final_answer", Text: "Final: route checkout sev2 to payment queue with on-call approval."},
			}}
			recorder := llm.NewRecorder()
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM | leia.LibDialect),
				leia.WithLLMProvider(provider),
				leia.WithLLMRecorder(recorder.Record),
			}, tc.opts...)
			vm := leia.New(opts...)
			source := aiDialectResearchAssistantWorkflowSource()
			if err := vm.Exec(source); err != nil {
				t.Fatalf("record Exec: %v", err)
			}
			assertResearchAssistantWorkflowContract(t, vm, provider.requests)

			records := recorder.Records()
			if len(records) != 5 {
				t.Fatalf("records = %#v, want five", records)
			}
			replayOpts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM | leia.LibDialect),
				leia.WithLLMReplay(records),
			}, tc.opts...)
			replayVM := leia.New(replayOpts...)
			if err := replayVM.Exec(source); err != nil {
				t.Fatalf("replay Exec: %v", err)
			}
			assertResearchAssistantWorkflowValues(t, replayVM)
		})
	}
}

func assertResearchAssistantWorkflowContract(t *testing.T, vm *leia.VM, requests []llm.TurnRequest) {
	t.Helper()
	if len(requests) != 5 {
		t.Fatalf("requests = %d, want 5", len(requests))
	}
	brief := requests[0]
	if brief.Model != "mock-json" {
		t.Fatalf("brief model = %q, want mock-json", brief.Model)
	}
	if len(brief.Messages) != 3 ||
		brief.Messages[0].Role != "system" || brief.Messages[0].Text != "Write a structured research brief from retrieved memory." ||
		brief.Messages[1].Role != "user" || brief.Messages[1].Text != "retrieved:runbook" ||
		!strings.Contains(brief.Messages[2].Text, "Research memory:") ||
		!strings.Contains(brief.Messages[2].Text, "[runbook] Checkout runbook") ||
		strings.Contains(brief.Messages[2].Text, "[notes]") {
		t.Fatalf("brief messages = %#v", brief.Messages)
	}
	assertJSONSchemaResponseFormat(t, brief.ResponseFormat, "research_brief", []string{"finding", "owner", "confidence"})

	section := requests[1]
	if section.Model != "mock-json" {
		t.Fatalf("section model = %q, want mock-json", section.Model)
	}
	if len(section.Messages) != 4 ||
		section.Messages[0].Role != "system" || section.Messages[0].Text != "Draft operational decision sections as JSON." ||
		section.Messages[1].Role != "user" || !strings.Contains(section.Messages[1].Text, `"owner":"payment queue"`) ||
		section.Messages[2].Text != "Return the decision, risk, and owner." ||
		!strings.Contains(section.Messages[3].Text, "Section memory:") {
		t.Fatalf("section messages = %#v", section.Messages)
	}
	if format, ok := section.ResponseFormat.(map[string]any); !ok || format["type"] != "json_object" {
		t.Fatalf("section response_format = %#v", section.ResponseFormat)
	}

	supervisorFirst := requests[2]
	if supervisorFirst.Model != "mock-supervisor" || len(supervisorFirst.Tools) != 1 {
		t.Fatalf("supervisor first request = %#v", supervisorFirst)
	}
	tool := supervisorFirst.Tools[0]
	if tool.Name != "handoff_review" ||
		tool.Description != "Delegate the research brief to a specialist." ||
		len(tool.Params) != 1 || tool.Params[0] != "topic" ||
		len(tool.Requires) != 1 || tool.Requires[0] != "none" ||
		tool.Schema != nil {
		t.Fatalf("handoff tool metadata = %#v", tool)
	}

	specialist := requests[3]
	if specialist.Model != "mock-specialist" || len(specialist.Tools) != 0 {
		t.Fatalf("specialist request = %#v", specialist)
	}
	if len(specialist.Messages) != 2 ||
		specialist.Messages[0].Role != "system" || specialist.Messages[0].Text != "Return structured delegated review." ||
		specialist.Messages[1].Role != "user" || specialist.Messages[1].Text != "checkout ownership" {
		t.Fatalf("specialist messages = %#v", specialist.Messages)
	}
	if format, ok := specialist.ResponseFormat.(map[string]any); !ok || format["type"] != "json_object" {
		t.Fatalf("specialist response_format = %#v", specialist.ResponseFormat)
	}

	supervisorFinal := requests[4]
	if supervisorFinal.Model != "mock-supervisor" || len(supervisorFinal.Tools) != 1 {
		t.Fatalf("supervisor final request = %#v", supervisorFinal)
	}
	if len(supervisorFinal.Messages) != 4 ||
		supervisorFinal.Messages[2].Role != "assistant" ||
		supervisorFinal.Messages[2].ToolCall == nil ||
		supervisorFinal.Messages[2].ToolCall.ID != "call_handoff_review_1" ||
		supervisorFinal.Messages[3].Role != "tool" ||
		supervisorFinal.Messages[3].ToolUseID != "call_handoff_review_1" {
		t.Fatalf("supervisor final messages = %#v", supervisorFinal.Messages)
	}
	toolValue, ok := supervisorFinal.Messages[3].Value.(map[string]any)
	if !ok ||
		toolValue["summary"] != "payment queue should own the sev2 handoff" ||
		toolValue["owner"] != "payment queue" ||
		toolValue["confidence"] != 0.89 {
		t.Fatalf("tool result value = %#v", supervisorFinal.Messages[3].Value)
	}

	assertResearchAssistantWorkflowValues(t, vm)
}

func assertResearchAssistantWorkflowValues(t *testing.T, vm *leia.VM) {
	t.Helper()
	for name, want := range map[string]any{
		"first_match_id":         "runbook",
		"flow_text":              `{"finding":"Payment queue owns sev2 checkout incidents","owner":"payment queue","confidence":0.93}`,
		"flow_second_input":      "retrieved:runbook",
		"section_decision":       "route to payment queue",
		"section_owner":          "payment queue",
		"brief_validation":       "true|",
		"decision_validation":    "true|",
		"final_text":             "Final: route checkout sev2 to payment queue with on-call approval.",
		"delegated_summary":      "payment queue should own the sev2 handoff",
		"delegated_owner":        "payment queue",
		"delegated_confidence":   0.89,
		"handoff_param":          "topic",
		"handoff_output_summary": "short finding",
	} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatalf("Get %s: %v", name, err)
		}
		if got != want {
			t.Fatalf("%s = %#v, want %#v", name, got, want)
		}
	}
}

func assertJSONSchemaResponseFormat(t *testing.T, raw any, name string, required []string) {
	t.Helper()
	format, ok := raw.(map[string]any)
	if !ok || format["type"] != "json_schema" {
		t.Fatalf("response_format = %#v", raw)
	}
	jsonSchema, ok := format["json_schema"].(map[string]any)
	if !ok || jsonSchema["name"] != name || jsonSchema["strict"] != true {
		t.Fatalf("json_schema = %#v", format["json_schema"])
	}
	schema, ok := jsonSchema["schema"].(map[string]any)
	if !ok || schema["type"] != "object" || schema["additionalProperties"] != false {
		t.Fatalf("schema = %#v", jsonSchema["schema"])
	}
	gotRequired, ok := schema["required"].([]any)
	if !ok || len(gotRequired) != len(required) {
		t.Fatalf("required = %#v", schema["required"])
	}
	for i, want := range required {
		if gotRequired[i] != want {
			t.Fatalf("required[%d] = %#v, want %#v", i, gotRequired[i], want)
		}
	}
}
