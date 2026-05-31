package gscript_test

import (
	"path/filepath"
	"testing"

	gs "github.com/never-labs/gscript"
	"github.com/never-labs/gscript/llm"
)

func TestLLMAgentScenarioIncidentResponseExampleSmoke(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: "Checkout incidents are owned by payments on-call."},
				{
					Status: "tool_calls",
					Calls: []llm.ToolCall{{
						ID:   "call_metrics_1",
						Tool: "get_metrics",
						Args: map[string]any{"service": "checkout"},
					}},
				},
				{Status: "final_answer", Text: "Checkout latency is elevated; page payments on-call."},
				{
					Status: "tool_calls",
					Calls: []llm.ToolCall{{
						ID:   "call_runbook_1",
						Tool: "search_runbook",
						Args: map[string]any{
							"service": "checkout",
							"symptom": "p95 latency spike",
						},
					}},
				},
				{Status: "final_answer", Text: "Incident brief: checkout latency spike, follow runbook."},
			}}
			vm := gs.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.ExecFile(filepath.Join(repoRoot(t), "examples", "llm", "incident_response.gs")); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}

			if len(provider.requests) != 5 {
				t.Fatalf("requests = %d, want 5", len(provider.requests))
			}
			faq := provider.requests[0]
			if faq.Model != "mock-ops" || len(faq.Tools) != 0 {
				t.Fatalf("faq request = %#v", faq)
			}
			if len(faq.Messages) != 2 || faq.Messages[0].Role != "system" || faq.Messages[1].Text != "Who owns checkout incidents?" {
				t.Fatalf("faq messages = %#v", faq.Messages)
			}

			researchFirst := provider.requests[1]
			if researchFirst.Model != "mock-ops" || len(researchFirst.Tools) != 2 {
				t.Fatalf("research first request = %#v", researchFirst)
			}
			if researchFirst.Tools[0].Name != "search_runbook" || researchFirst.Tools[1].Name != "get_metrics" {
				t.Fatalf("research tools = %#v", researchFirst.Tools)
			}
			researchFinal := provider.requests[2]
			if len(researchFinal.Messages) != 4 ||
				researchFinal.Messages[2].ToolCall == nil ||
				researchFinal.Messages[2].ToolCall.Tool != "get_metrics" ||
				researchFinal.Messages[3].Value != "metrics:checkout:latency=high,error_rate=2%" {
				t.Fatalf("research final messages = %#v", researchFinal.Messages)
			}

			briefFirst := provider.requests[3]
			if briefFirst.Model != "mock-planner" || len(briefFirst.Tools) != 2 {
				t.Fatalf("brief first request = %#v", briefFirst)
			}
			briefFinal := provider.requests[4]
			if briefFinal.Model != "mock-planner" || briefFinal.MaxTokens != 320 {
				t.Fatalf("brief final request = %#v", briefFinal)
			}
			if len(briefFinal.Messages) != 4 ||
				briefFinal.Messages[2].ToolCall == nil ||
				briefFinal.Messages[2].ToolCall.ID != "call_runbook_1" ||
				briefFinal.Messages[3].ToolUseID != "call_runbook_1" ||
				briefFinal.Messages[3].Value != "runbook:checkout:p95 latency spike" {
				t.Fatalf("brief final messages = %#v", briefFinal.Messages)
			}

			for name, want := range map[string]any{
				"faq_text":          "Checkout incidents are owned by payments on-call.",
				"research_text":     "Checkout latency is elevated; page payments on-call.",
				"brief_text":        "Incident brief: checkout latency spike, follow runbook.",
				"brief_evidence":    "runbook:checkout:p95 latency spike",
				"brief_history_len": int64(4),
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
		})
	}
}
