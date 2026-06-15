package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
)

func TestLLMEvidenceOutcomeProjectsRefOnlyTraceEvent(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(tc.opts...)
			if err := vm.Exec(`
evidence := llm.evidence({
    llm.doc("Revenue increased because enterprise renewal rates improved.", {
        id: "rev-note"
        title: "Revenue note"
        source: "filings/10q"
        artifact_id: "artifact-10q"
        section: "mdna"
    })
    llm.doc("Margin pressure came from higher infrastructure spend.", {
        id: "margin-note"
        title: "Margin note"
        source: "earnings/transcript"
        artifact_id: "artifact-call"
        section: "qa"
    })
}, {label: "Analyst evidence"})

outcome := llm.evidence_outcome(evidence, {
    workflow_run_id: "wf-evidence"
    workflow_step_id: "draft-section"
    report_id: "report-aapl"
    section: "investment_view"
})
event := llm.evidence_outcome_event(outcome, {
    trace_id: "trace-evidence"
    sequence: 1
})
trace := llm.trace_envelope([event], {trace_id: "trace-evidence"})
gate := llm.trace_assert(trace, {
    required_event_types: ["evidence_outcome"]
    require_event_payload_fields: {evidence_outcome: ["status", "result_status", "evidence_count", "evidence_refs", "citation_count"]}
    require_correlation_fields: ["workflow_run_id", "workflow_step_id", "correlation_id"]
    max_status_counts: {cited: 1}
    deny_payload_fields: ["text", "snippet", "raw_prompt", "raw_completion", "secret"]
})

outcome_kind := outcome.kind
outcome_source_kind := outcome.source_kind
outcome_status := outcome.status
outcome_result_status := outcome.result_status
outcome_evidence_count := outcome.evidence_count
outcome_citation_count := outcome.citation_count
outcome_source_count := outcome.source_count
outcome_artifact_count := outcome.artifact_count
outcome_top_id := outcome.top_id
outcome_top_source := outcome.top_source
outcome_top_artifact_id := outcome.top_artifact_id
outcome_redaction_policy := outcome.redaction.policy
outcome_redaction_raw_text := outcome.redaction.raw_text_stored
outcome_ref_text_missing := outcome.evidence_refs[1].text == nil
outcome_ref_snippet_missing := outcome.evidence_refs[1].citation.snippet == nil
event_type := event.event_type
event_status := event.status
event_payload_count := event.payload.evidence_count
event_payload_top_id := event.payload.top_id
event_payload_ref_text_missing := event.payload.evidence_refs[1].text == nil
event_payload_ref_snippet_missing := event.payload.evidence_refs[1].citation.snippet == nil
event_correlation_workflow := event.correlation.workflow_run_id
event_correlation_step := event.correlation.workflow_step_id
event_correlation_id := event.correlation.correlation_id
event_redaction_policy := event.redaction.policy
gate_ok := gate.ok
gate_status := gate.status
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"outcome_kind":                      "evidence_outcome",
				"outcome_source_kind":               "evidence",
				"outcome_status":                    "cited",
				"outcome_result_status":             "ok",
				"outcome_evidence_count":            int64(2),
				"outcome_citation_count":            int64(2),
				"outcome_source_count":              int64(2),
				"outcome_artifact_count":            int64(2),
				"outcome_top_id":                    "rev-note",
				"outcome_top_source":                "filings/10q",
				"outcome_top_artifact_id":           "artifact-10q",
				"outcome_redaction_policy":          "evidence_outcome_ref_only",
				"outcome_redaction_raw_text":        false,
				"outcome_ref_text_missing":          true,
				"outcome_ref_snippet_missing":       true,
				"event_type":                        "evidence_outcome",
				"event_status":                      "cited",
				"event_payload_count":               int64(2),
				"event_payload_top_id":              "rev-note",
				"event_payload_ref_text_missing":    true,
				"event_payload_ref_snippet_missing": true,
				"event_correlation_workflow":        "wf-evidence",
				"event_correlation_step":            "draft-section",
				"event_correlation_id":              "report-aapl",
				"event_redaction_policy":            "evidence_outcome_ref_only",
				"gate_ok":                           true,
				"gate_status":                       "ok",
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
