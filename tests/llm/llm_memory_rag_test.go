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
        tags: ["checkout", "payments"]
    })
    llm.document({
        id: "notes"
        title: "Release notes"
        text: "Search indexing work is unrelated to checkout incidents."
        source: "local/notes"
    })
})

ctx := llm.retrieve(docs, "checkout payment sev2", {limit: 1 label: "Retrieved context"})
memory_outcome := llm.memory_outcome(ctx, {
    workflow_run_id: "wf-memory"
    workflow_step_id: "retrieve-runbook"
    component: "memory-rag-test"
})
memory_event := llm.memory_outcome_event(memory_outcome, {
    trace_id: "trace-memory"
    sequence: 1
})
memory_trace := llm.trace_envelope([memory_event], {trace_id: "trace-memory"})
memory_gate := llm.trace_assert(memory_trace, {
    required_event_types: ["memory_outcome"]
    require_event_payload_fields: {memory_outcome: ["status", "result_status", "match_count", "match_refs", "top_match"]}
    require_correlation_fields: ["workflow_run_id", "workflow_step_id", "correlation_id"]
    max_status_counts: {matched: 1}
})

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
memory_outcome_kind := memory_outcome.kind
memory_outcome_source_kind := memory_outcome.source_kind
memory_outcome_status := memory_outcome.status
memory_outcome_result_status := memory_outcome.result_status
memory_outcome_match_count := memory_outcome.match_count
memory_outcome_top_id := memory_outcome.top_id
memory_outcome_top_score := memory_outcome.top_score
memory_outcome_top_text_missing := memory_outcome.top_match.text == nil
memory_outcome_ref_text_missing := memory_outcome.match_refs[1].text == nil
memory_outcome_ref_snippet_missing := memory_outcome.match_refs[1].citation.snippet == nil
memory_event_type := memory_event.event_type
memory_event_status := memory_event.status
memory_event_payload_count := memory_event.payload.match_count
memory_event_payload_top_id := memory_event.payload.top_id
memory_event_correlation_workflow := memory_event.correlation.workflow_run_id
memory_event_correlation_id := memory_event.correlation.correlation_id
memory_gate_ok := memory_gate.ok
memory_gate_status := memory_gate.status
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
			for name, want := range map[string]any{
				"memory_outcome_kind":                "memory_outcome",
				"memory_outcome_source_kind":         "retrieval",
				"memory_outcome_status":              "matched",
				"memory_outcome_result_status":       "ok",
				"memory_outcome_match_count":         int64(1),
				"memory_outcome_top_id":              "runbook",
				"memory_event_type":                  "memory_outcome",
				"memory_event_status":                "matched",
				"memory_event_payload_count":         int64(1),
				"memory_event_payload_top_id":        "runbook",
				"memory_event_correlation_workflow":  "wf-memory",
				"memory_event_correlation_id":        "checkout payment sev2",
				"memory_outcome_top_text_missing":    true,
				"memory_outcome_ref_text_missing":    true,
				"memory_outcome_ref_snippet_missing": true,
				"memory_gate_ok":                     true,
				"memory_gate_status":                 "ok",
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
			topScore, err := vm.Get("memory_outcome_top_score")
			if err != nil {
				t.Fatalf("Get memory_outcome_top_score: %v", err)
			}
			if topScore.(int64) <= 0 {
				t.Fatalf("memory_outcome_top_score = %#v, want positive", topScore)
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
