package leia_test

import (
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMAgentScenarioIncidentResponseExampleSmoke(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
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
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.ExecFile(filepath.Join(repoRoot(t), "examples", "llm", "incident_response.leia")); err != nil {
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

func TestLLMAgentScenarioManualToolHistoryExampleSmoke(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{
					Status: "tool_calls",
					Calls: []llm.ToolCall{{
						ID:   "call_doc_1",
						Tool: "lookup_doc",
						Args: map[string]any{"topic": "agent history"},
					}},
				},
				{Status: "final_answer", Text: "Manual flows append tool evidence before the next turn."},
			}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.ExecFile(filepath.Join(repoRoot(t), "examples", "llm", "manual_tool_history.leia")); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}

			if len(provider.requests) != 2 {
				t.Fatalf("requests = %d, want 2", len(provider.requests))
			}
			first := provider.requests[0]
			if first.Model != "mock-manual" || len(first.Tools) != 1 || first.Tools[0].Name != "lookup_doc" {
				t.Fatalf("first request = %#v", first)
			}
			if len(first.Messages) != 2 ||
				first.Messages[0].Role != "system" ||
				first.Messages[1].Text != "Find docs for agent history" {
				t.Fatalf("first messages = %#v", first.Messages)
			}

			second := provider.requests[1]
			if second.Model != "mock-manual" || second.MaxTokens != 128 || len(second.Tools) != 1 {
				t.Fatalf("second request = %#v", second)
			}
			if len(second.Messages) != 4 ||
				second.Messages[2].Role != "assistant" ||
				second.Messages[2].ToolCall == nil ||
				second.Messages[2].ToolCall.ID != "call_doc_1" ||
				second.Messages[3].Role != "tool" ||
				second.Messages[3].ToolUseID != "call_doc_1" ||
				second.Messages[3].Value != "doc:agent history" {
				t.Fatalf("second messages = %#v", second.Messages)
			}

			for name, want := range map[string]any{
				"manual_text":        "Manual flows append tool evidence before the next turn.",
				"manual_evidence":    "doc:agent history",
				"manual_history_len": int64(4),
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

func TestLLMAgentAsToolExampleSmoke(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{
					Status: "tool_calls",
					Calls: []llm.ToolCall{{
						ID:   "call_delegate_1",
						Tool: "extract_research",
						Args: map[string]any{
							"domain":  "refunds",
							"control": "PCI",
						},
					}},
				},
				{
					Status: "tool_calls",
					Calls: []llm.ToolCall{{
						ID:   "call_policy_1",
						Tool: "lookup_policy",
						Args: map[string]any{
							"domain":  "refunds",
							"control": "PCI",
						},
					}},
				},
				{
					Status: "final_answer",
					Text:   `{"summary":"PCI review is required for refunds","confidence":0.88,"risk":"high"}`,
				},
				{Status: "final_answer", Text: "Do not bypass PCI review; use delegated policy evidence."},
				{Status: "final_answer", Text: "Audit: delegated answer cites policy-backed evidence."},
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibAll),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)

			if err := vm.ExecFile(filepath.Join(repoRoot(t), "examples", "llm", "agent_as_tool.leia")); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}

			if len(provider.requests) != 5 {
				t.Fatalf("requests = %d, want 5", len(provider.requests))
			}

			supervisorFirst := provider.requests[0]
			if supervisorFirst.Model != "mock-supervisor" || len(supervisorFirst.Tools) != 1 || supervisorFirst.Tools[0].Name != "extract_research" {
				t.Fatalf("supervisor first request = %#v", supervisorFirst)
			}
			if len(supervisorFirst.Messages) != 3 ||
				supervisorFirst.Messages[0].Role != "system" ||
				supervisorFirst.Messages[0].Text != "Delegate policy research before making a decision." ||
				supervisorFirst.Messages[1].Role != "assistant" ||
				supervisorFirst.Messages[1].Text != "Previous review: refunds need explicit evidence." ||
				supervisorFirst.Messages[2].Role != "user" ||
				supervisorFirst.Messages[2].Text != "Answer with a decision for: Can refunds bypass PCI review?" {
				t.Fatalf("supervisor first messages = %#v", supervisorFirst.Messages)
			}
			if supervisorFirst.Tools[0].Schema != nil {
				t.Fatalf("agent output shape must not be sent as provider tool input schema: %#v", supervisorFirst.Tools[0].Schema)
			}

			extractorFirst := provider.requests[1]
			if extractorFirst.Model != "mock-extractor" || len(extractorFirst.Tools) != 1 || extractorFirst.Tools[0].Name != "lookup_policy" {
				t.Fatalf("extractor first request = %#v", extractorFirst)
			}
			if extractorFirst.Tools[0].Description != "Read deterministic local policy evidence." ||
				len(extractorFirst.Tools[0].Requires) != 1 ||
				extractorFirst.Tools[0].Requires[0] != "policies.read" {
				t.Fatalf("extractor tool metadata = %#v", extractorFirst.Tools[0])
			}
			if len(extractorFirst.Messages) != 2 ||
				extractorFirst.Messages[0].Role != "system" ||
				extractorFirst.Messages[0].Text != "Extract only claims supported by local policy tool evidence." ||
				extractorFirst.Messages[1].Role != "user" ||
				extractorFirst.Messages[1].Text != "Research refunds control PCI." {
				t.Fatalf("extractor first messages = %#v", extractorFirst.Messages)
			}

			extractorFinal := provider.requests[2]
			if extractorFinal.Model != "mock-extractor" || len(extractorFinal.Tools) != 1 {
				t.Fatalf("extractor final request = %#v", extractorFinal)
			}
			if len(extractorFinal.Messages) != 4 ||
				extractorFinal.Messages[2].Role != "assistant" ||
				extractorFinal.Messages[2].ToolCall == nil ||
				extractorFinal.Messages[2].ToolCall.ID != "call_policy_1" ||
				extractorFinal.Messages[3].Role != "tool" ||
				extractorFinal.Messages[3].ToolUseID != "call_policy_1" ||
				extractorFinal.Messages[3].Value != "policy:refunds:PCI=review-required" {
				t.Fatalf("extractor final messages = %#v", extractorFinal.Messages)
			}
			format, ok := extractorFinal.ResponseFormat.(map[string]any)
			if !ok || format["type"] != "json_object" {
				t.Fatalf("extractor response_format = %#v", extractorFinal.ResponseFormat)
			}

			supervisorFinal := provider.requests[3]
			if supervisorFinal.Model != "mock-supervisor" || len(supervisorFinal.Tools) != 1 {
				t.Fatalf("supervisor final request = %#v", supervisorFinal)
			}
			if len(supervisorFinal.Messages) != 5 ||
				supervisorFinal.Messages[3].Role != "assistant" ||
				supervisorFinal.Messages[3].ToolCall == nil ||
				supervisorFinal.Messages[3].ToolCall.ID != "call_delegate_1" ||
				supervisorFinal.Messages[4].Role != "tool" ||
				supervisorFinal.Messages[4].ToolUseID != "call_delegate_1" {
				t.Fatalf("supervisor final messages = %#v", supervisorFinal.Messages)
			}
			toolValue, ok := supervisorFinal.Messages[4].Value.(map[string]any)
			if !ok ||
				toolValue["summary"] != "PCI review is required for refunds" ||
				toolValue["confidence"] != 0.88 ||
				toolValue["risk"] != "high" {
				t.Fatalf("supervisor tool value = %#v", supervisorFinal.Messages[4].Value)
			}

			audit := provider.requests[4]
			if audit.Model != "mock-audit" || audit.MaxTokens != 96 || len(audit.Tools) != 0 {
				t.Fatalf("audit request = %#v", audit)
			}
			if len(audit.Messages) != 7 ||
				audit.Messages[4].Role != "tool" ||
				audit.Messages[5].Role != "assistant" ||
				audit.Messages[5].Text != "Do not bypass PCI review; use delegated policy evidence." ||
				audit.Messages[6].Role != "user" ||
				audit.Messages[6].Text != "Audit delegated answer: Do not bypass PCI review; use delegated policy evidence." {
				t.Fatalf("audit messages = %#v", audit.Messages)
			}

			for name, want := range map[string]any{
				"final_text":           "Do not bypass PCI review; use delegated policy evidence.",
				"delegated_summary":    "PCI review is required for refunds",
				"delegated_confidence": 0.88,
				"delegated_risk":       "high",
				"tool_history_idx":     int64(5),
				"task_history_idx":     int64(3),
				"task_text":            "Answer with a decision for: Can refunds bypass PCI review?",
				"outer_history_len":    int64(5),
				"audit_text":           "Audit: delegated answer cites policy-backed evidence.",
				"audit_history_len":    int64(7),
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

func TestLLMDirectTurnExampleSmoke(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{
				Status: "final_answer",
				Text:   "Direct llm.turn keeps examples simple.",
			}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)

			if err := vm.ExecFile(filepath.Join(repoRoot(t), "examples", "llm", "direct_turn.leia")); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}

			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			req := provider.requests[0]
			if req.Model != "mock-fast" || req.MaxTokens != 64 || req.Temperature == nil || *req.Temperature != 0 {
				t.Fatalf("request = %#v", req)
			}
			if len(req.Messages) != 2 ||
				req.Messages[0].Role != "system" ||
				req.Messages[1].Role != "user" ||
				req.Messages[1].Text != "Summarize direct LLM calls." {
				t.Fatalf("messages = %#v", req.Messages)
			}

			for name, want := range map[string]any{
				"direct_text":   "Direct llm.turn keeps examples simple.",
				"direct_status": "final_answer",
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

func TestLLMStreamingTurnExampleSmoke(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &streamingTraceProvider{}
			var events []llm.TraceEvent
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
				leia.WithLLMTrace(func(event llm.TraceEvent) {
					events = append(events, event)
				}),
			}, tc.opts...)
			vm := leia.New(opts...)

			if err := vm.ExecFile(filepath.Join(repoRoot(t), "examples", "llm", "streaming_turn.leia")); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}

			for name, want := range map[string]any{
				"stream_text":        "hello stream",
				"stream_status":      "final_answer",
				"streamed_text":      "hello stream",
				"stream_event_count": int64(3),
				"last_stream_event":  "token",
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
			if !provider.usedStream {
				t.Fatalf("provider did not receive streaming request")
			}

			gotTypes := make([]string, 0, len(events))
			tokens := make([]string, 0, 3)
			for _, event := range events {
				gotTypes = append(gotTypes, event.Type)
				if event.Type == "turn_stream" {
					tokens = append(tokens, event.Token)
				}
			}
			wantTypes := []string{"turn_start", "turn_stream", "turn_stream", "turn_stream", "turn_end"}
			if strings.Join(gotTypes, ",") != strings.Join(wantTypes, ",") {
				t.Fatalf("trace event types = %#v, want %#v", gotTypes, wantTypes)
			}
			if strings.Join(tokens, "") != "hello stream" {
				t.Fatalf("trace tokens = %#v", tokens)
			}
		})
	}
}

func TestLLMPromptTaggedMessagesExampleSmoke(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{
				Status: "final_answer",
				Text:   "Prompt tags can organize agent history.",
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibAll),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)

			if err := vm.ExecFile(filepath.Join(repoRoot(t), "examples", "llm", "prompt_tagged_messages.leia")); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}

			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			req := provider.requests[0]
			if req.Model != "mock-prompt" || req.MaxTokens != 96 {
				t.Fatalf("request = %#v", req)
			}
			if len(req.Messages) != 2 ||
				req.Messages[0].Role != "system" ||
				req.Messages[0].Text != "Answer using the supplied context." ||
				req.Messages[1].Role != "user" ||
				req.Messages[1].Text != "Summarize Leia agent history." {
				t.Fatalf("messages = %#v", req.Messages)
			}

			for name, want := range map[string]any{
				"prompt_tagged_text":        "Prompt tags can organize agent history.",
				"prompt_tagged_task":        "Summarize Leia agent history.",
				"prompt_tagged_task_idx":    int64(2),
				"prompt_tagged_history_len": int64(2),
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

func TestLLMRichAgentDemoExampleSmoke(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{
					Status: "tool_calls",
					Calls: []llm.ToolCall{{
						ID:   "call_signal_1",
						Tool: "lookup_signal",
						Args: map[string]any{
							"service":  "checkout",
							"severity": "sev2",
						},
					}},
				},
				{Status: "final_answer", Text: "Checkout sev2 is payment-owned; continue with local signal evidence."},
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibAll),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)

			if err := vm.ExecFile(filepath.Join(repoRoot(t), "examples", "llm", "rich_agent_demo.leia")); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}

			if len(provider.requests) != 2 {
				t.Fatalf("requests = %d, want 2", len(provider.requests))
			}
			first := provider.requests[0]
			if first.Model != "mock-rich-agent" || len(first.Tools) != 1 || first.Tools[0].Name != "lookup_signal" {
				t.Fatalf("first request = %#v", first)
			}
			if first.Tools[0].Description != "Lookup deterministic incident signal evidence." ||
				len(first.Tools[0].Requires) != 1 ||
				first.Tools[0].Requires[0] != "signals.read" {
				t.Fatalf("first tool metadata = %#v", first.Tools[0])
			}
			if len(first.Messages) != 3 ||
				first.Messages[0].Role != "system" ||
				first.Messages[0].Text != "Triage incidents using only supplied history and local tool evidence." ||
				first.Messages[1].Role != "assistant" ||
				first.Messages[1].Text != "Previous handoff: checkout has pending payment alerts." ||
				first.Messages[2].Role != "user" ||
				first.Messages[2].Text != "Assess checkout at sev2 severity." {
				t.Fatalf("first messages = %#v", first.Messages)
			}

			final := provider.requests[1]
			if final.Model != "mock-rich-agent" || final.MaxTokens != 160 || len(final.Tools) != 1 {
				t.Fatalf("final request = %#v", final)
			}
			if len(final.Messages) != 5 ||
				final.Messages[3].Role != "assistant" ||
				final.Messages[3].ToolCall == nil ||
				final.Messages[3].ToolCall.ID != "call_signal_1" ||
				final.Messages[3].ToolCall.Tool != "lookup_signal" ||
				final.Messages[4].Role != "tool" ||
				final.Messages[4].ToolUseID != "call_signal_1" ||
				final.Messages[4].Value != "signal:checkout:severity=sev2:queue=payments" {
				t.Fatalf("final messages = %#v", final.Messages)
			}

			for name, want := range map[string]any{
				"rich_text":        "Checkout sev2 is payment-owned; continue with local signal evidence.",
				"rich_task":        "Assess checkout at sev2 severity.",
				"rich_task_idx":    int64(3),
				"rich_evidence":    "signal:checkout:severity=sev2:queue=payments",
				"rich_history_len": int64(5),
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
