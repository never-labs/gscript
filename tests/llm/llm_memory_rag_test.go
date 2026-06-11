package leia_test

import (
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMMemoryRAGContextFeedsTurnAgentAndSections(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: "turn used memory"},
				{Status: "final_answer", Text: "agent used memory"},
				{Status: "final_answer", Text: `{"section":"memory"}`},
			}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.Exec(`
docs := llm.collection({
    llm.doc("Checkout runbook says payment queue owns sev2 incidents.", {
        id: "runbook"
        title: "Checkout runbook"
        source: "local/runbook"
        tags: {"checkout", "payments"}
    })
    llm.document({
        id: "notes"
        title: "Release notes"
        text: "Search indexing work is unrelated to checkout incidents."
        source: "local/notes"
    })
})

ctx := llm.retrieve(docs, "checkout payment sev2", {limit: 1 label: "Retrieved context"})

turn_result, turn_err := llm.turn({
    model: "mock-memory"
    user: "Who owns the incident?"
    context: ctx
})

memory_agent := llm.agent("memory_agent", func(question) {
    return {
        model: "mock-memory"
        user: question
        evidence: llm.evidence(ctx.matches, {label: "Agent evidence"})
    }
})

agent_result, agent_err := memory_agent("Answer with local evidence.")

generated, section_err := llm.sections({
    model: "mock-memory"
    user: "Draft sections."
    context: llm.context(ctx.matches, {label: "Shared context"})
    sections: {
        {
            name: "summary"
            instructions: "Summarize owner."
            output: {section: "memory"}
        }
    }
})

first_match_id := ctx.matches[1].id
turn_text := turn_result.text
agent_text := agent_result.text
section_value := generated.values.summary.section
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 3 {
				t.Fatalf("requests = %d, want 3", len(provider.requests))
			}
			for i, req := range provider.requests {
				if req.Model != "mock-memory" {
					t.Fatalf("request %d model = %q", i, req.Model)
				}
				if len(req.Messages) < 2 {
					t.Fatalf("request %d messages = %#v", i, req.Messages)
				}
			}
			if got := provider.requests[0].Messages[1].Text; !strings.Contains(got, "Retrieved context:") || !strings.Contains(got, "[runbook] Checkout runbook") || strings.Contains(got, "[notes]") {
				t.Fatalf("turn context message = %q", got)
			}
			if got := provider.requests[1].Messages[1].Text; !strings.Contains(got, "Agent evidence:") || !strings.Contains(got, "payment queue owns sev2") {
				t.Fatalf("agent evidence message = %q", got)
			}
			if len(provider.requests[2].Messages) != 3 ||
				provider.requests[2].Messages[1].Text != "Summarize owner." ||
				!strings.Contains(provider.requests[2].Messages[2].Text, "Shared context:") {
				t.Fatalf("section messages = %#v", provider.requests[2].Messages)
			}
			firstMatchID, err := vm.Get("first_match_id")
			if err != nil {
				t.Fatalf("Get first_match_id: %v", err)
			}
			if firstMatchID != "runbook" {
				t.Fatalf("first_match_id = %#v", firstMatchID)
			}
			sectionValue, err := vm.Get("section_value")
			if err != nil {
				t.Fatalf("Get section_value: %v", err)
			}
			if sectionValue != "memory" {
				t.Fatalf("section_value = %#v", sectionValue)
			}
		})
	}
}
