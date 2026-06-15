package bind

import "testing"

func TestLLMPlanningGraphNormalizesValidatesAndTraces(t *testing.T) {
	interp := runLLMTestProgram(t, `
input := llm.plan_node("input", {
    node_type: "input"
    trace_evidence: ["evt-input-completed"]
})
expand := llm.planNode("expand", {
    node_type: "transform"
    depends_on: ["input"]
    retry_policy: {max_attempts: 2 retryable: true backoff: "fixed_fixture_ms"}
    trace_evidence: ["evt-expand-attempt-1", "evt-expand-attempt-2"]
})
branch := llm.plan_node("branch", {
    node_type: "branch"
    depends_on: ["expand"]
    branches: ["collect", "validate"]
    trace_evidence: ["evt-branch-selected"]
})
merge := llm.plan_node("merge", {
    node_type: "merge"
    depends_on: ["branch"]
    merge_policy: {mode: "all" required_inputs: ["collect", "validate"]}
    trace_evidence: ["evt-merge-completed"]
})

graph := llm.planning_graph([input, expand, branch, merge], [
    {from: "input" to: "expand" edge_type: "control"},
    {from: "expand" to: "branch" edge_type: "branch"},
    {from: "branch" to: "merge" edge_type: "merge"},
], {
    id: "generic-planning"
    capability_refs: ["generic.planning.graph.define", "generic.planning.trace.evidence"]
})
gate := llm.validate_planning_graph(graph)
trace := llm.planning_trace(graph, [
    {event_id: "evt-input-completed" evidence_kind: "plan.node.completed" plan_node_id: "input" status: "completed"},
    {event_id: "evt-expand-attempt-1" evidence_kind: "plan.retry.attempt" plan_node_id: "expand" status: "retrying" attempt: 1},
    {event_id: "evt-branch-selected" evidence_kind: "plan.branch.selected" plan_node_id: "branch" status: "completed" branches: ["collect", "validate"]},
    {event_id: "evt-merge-completed" evidence_kind: "plan.merge.completed" plan_node_id: "merge" status: "completed" merged_inputs: ["collect", "validate"]},
], {run_id: "run-1" fixture_key: "planning:fixture"})

bad_graph := llm.planningGraph([input, input], [
    {from: "missing" to: "input" edge_type: "control"}
], {provider_free: false live_model: true})
bad_gate := llm.validatePlanningGraph(bad_graph)

missing_node_ok, missing_node_err := pcall(llm.plan_node)
bad_node_opts_ok, bad_node_opts_err := pcall(llm.plan_node, "x", "opts")
missing_graph_ok, missing_graph_err := pcall(llm.planning_graph)
bad_graph_edges_ok, bad_graph_edges_err := pcall(llm.planning_graph, [], "edges")
missing_validate_ok, missing_validate_err := pcall(llm.validate_planning_graph)
missing_trace_ok, missing_trace_err := pcall(llm.planning_trace)

graph_kind := graph.kind
graph_marker := graph.__llm_planning_graph
graph_nodes := graph.node_count
graph_edges := graph.edge_count
graph_provider_free := graph.provider_free
graph_live_model := graph.live_model
graph_branch_count := graph.summary.branch_count
graph_merge_count := graph.summary.merge_count
graph_retryable_count := graph.summary.retryable_count
graph_redaction := graph.redaction.policy
gate_ok := gate.ok
gate_status := gate.status
gate_findings := gate.finding_count
bad_ok := bad_gate.ok
bad_status := bad_gate.status
bad_findings := bad_gate.finding_count

trace_kind := trace.kind
trace_run := trace.run_id
trace_events := #trace.events
trace_first_sequence := trace.events[1].sequence
trace_retry_count := trace.summary.retry_attempt_count
trace_branch_count := trace.summary.branch_count
trace_merge_count := trace.summary.merge_count
trace_final := trace.summary.final_status
trace_provider_free := trace.events[1].provider_free
trace_live_model := trace.events[1].live_model
`, nil)

	for name, want := range map[string]Value{
		"graph_kind":            StringValue("planning_graph"),
		"graph_marker":          BoolValue(true),
		"graph_nodes":           IntValue(4),
		"graph_edges":           IntValue(3),
		"graph_provider_free":   BoolValue(true),
		"graph_live_model":      BoolValue(false),
		"graph_branch_count":    IntValue(1),
		"graph_merge_count":     IntValue(1),
		"graph_retryable_count": IntValue(1),
		"graph_redaction":       StringValue("planning_graph_metadata_only"),
		"gate_ok":               BoolValue(true),
		"gate_status":           StringValue("ok"),
		"gate_findings":         IntValue(0),
		"bad_ok":                BoolValue(false),
		"bad_status":            StringValue("failed"),
		"trace_kind":            StringValue("planning_trace"),
		"trace_run":             StringValue("run-1"),
		"trace_events":          IntValue(4),
		"trace_first_sequence":  IntValue(1),
		"trace_retry_count":     IntValue(1),
		"trace_branch_count":    IntValue(1),
		"trace_merge_count":     IntValue(1),
		"trace_final":           StringValue("completed"),
		"trace_provider_free":   BoolValue(true),
		"trace_live_model":      BoolValue(false),
		"missing_node_ok":       BoolValue(false),
		"bad_node_opts_ok":      BoolValue(false),
		"missing_graph_ok":      BoolValue(false),
		"bad_graph_edges_ok":    BoolValue(false),
		"missing_validate_ok":   BoolValue(false),
		"missing_trace_ok":      BoolValue(false),
	} {
		got := interp.GetGlobal(name)
		if !got.Equal(want) {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
	}
	if got := interp.GetGlobal("bad_findings"); !got.IsInt() || got.Int() < 2 {
		t.Fatalf("bad_findings = %v, want at least two findings", got)
	}
	for _, name := range []string{"missing_node_err", "bad_node_opts_err", "missing_graph_err", "bad_graph_edges_err", "missing_validate_err", "missing_trace_err"} {
		if got := interp.GetGlobal(name); !got.IsString() || got.Str() == "" {
			t.Fatalf("%s = %v, want non-empty error string", name, got)
		}
	}
}
