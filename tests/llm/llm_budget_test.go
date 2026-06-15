package leia_test

import (
	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
	"testing"
)

func TestLLMStandaloneBudgetLimitsDirectTurns(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: "one"},
				{Status: "final_answer", Text: "two"},
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)
			if err := vm.Exec(`
err_kind := nil
err_dimension := nil
err_message := nil
err_limit := nil
err_used := nil
outcome_kind := nil
outcome_source_kind := nil
outcome_status := nil
outcome_result_status := nil
outcome_blocked := nil
event_type := nil
event_status := nil
event_payload_dimension := nil
event_correlation_workflow := nil
event_correlation_id := nil
gate_ok := nil
gate_status := nil

llm.with_budget({turns: 1}, func() {
    first, first_err := llm.turn({messages: {llm.user("first")}})
    second, second_err := llm.turn({messages: {llm.user("second")}})
    err_kind = second_err.kind
    err_dimension = second_err.dimension
    err_message = second_err.message
    err_limit = second_err.limit
    err_used = second_err.used
    outcome := llm.budget_outcome(second_err, {
        workflow_run_id: "wf-budget"
        workflow_step_id: "step-turns"
        component: "budget-test"
    })
    event := llm.budget_outcome_event(outcome, {
        trace_id: "trace-budget"
        sequence: 1
    })
    envelope := llm.trace_envelope([event], {trace_id: "trace-budget"})
    gate := llm.trace_assert(envelope, {
        required_event_types: ["budget_outcome"]
        require_event_payload_fields: {budget_outcome: ["status", "result_status", "dimension", "limit", "used"]}
        require_correlation_fields: ["workflow_run_id", "workflow_step_id", "correlation_id"]
        max_status_counts: {exceeded: 1}
    })
    outcome_kind = outcome.kind
    outcome_source_kind = outcome.source_kind
    outcome_status = outcome.status
    outcome_result_status = outcome.result_status
    outcome_blocked = outcome.blocked
    event_type = event.event_type
    event_status = event.status
    event_payload_dimension = event.payload.dimension
    event_correlation_workflow = event.correlation.workflow_run_id
    event_correlation_id = event.correlation.correlation_id
    gate_ok = gate.ok
    gate_status = gate.status
})
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			kind, _ := vm.Get("err_kind")
			dimension, _ := vm.Get("err_dimension")
			if kind != "budget" || dimension != "turns" {
				t.Fatalf("err kind=%#v dimension=%#v", kind, dimension)
			}
			message, _ := vm.Get("err_message")
			limit, _ := vm.Get("err_limit")
			used, _ := vm.Get("err_used")
			if message != "llm budget exceeded: turns" || limit != int64(1) || used != int64(1) {
				t.Fatalf("err message=%#v limit=%#v used=%#v", message, limit, used)
			}
			outcomeKind, _ := vm.Get("outcome_kind")
			outcomeSourceKind, _ := vm.Get("outcome_source_kind")
			outcomeStatus, _ := vm.Get("outcome_status")
			outcomeResultStatus, _ := vm.Get("outcome_result_status")
			outcomeBlocked, _ := vm.Get("outcome_blocked")
			if outcomeKind != "budget_outcome" || outcomeSourceKind != "budget" || outcomeStatus != "exceeded" || outcomeResultStatus != "blocked" || outcomeBlocked != true {
				t.Fatalf("outcome kind=%#v source=%#v status=%#v result=%#v blocked=%#v", outcomeKind, outcomeSourceKind, outcomeStatus, outcomeResultStatus, outcomeBlocked)
			}
			eventType, _ := vm.Get("event_type")
			eventStatus, _ := vm.Get("event_status")
			eventPayloadDimension, _ := vm.Get("event_payload_dimension")
			eventCorrelationWorkflow, _ := vm.Get("event_correlation_workflow")
			eventCorrelationID, _ := vm.Get("event_correlation_id")
			gateOK, _ := vm.Get("gate_ok")
			gateStatus, _ := vm.Get("gate_status")
			if eventType != "budget_outcome" || eventStatus != "exceeded" || eventPayloadDimension != "turns" || eventCorrelationWorkflow != "wf-budget" || eventCorrelationID != "turns" || gateOK != true || gateStatus != "ok" {
				t.Fatalf("event type=%#v status=%#v dim=%#v workflow=%#v corr=%#v gate=%#v/%#v", eventType, eventStatus, eventPayloadDimension, eventCorrelationWorkflow, eventCorrelationID, gateOK, gateStatus)
			}
		})
	}
}

func TestLLMStandaloneBudgetNestedIntersectionUsesOuterTokens(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: "one", Usage: llm.TurnUsage{InputTokens: 2, OutputTokens: 3}},
				{Status: "final_answer", Text: "two"},
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)
			if err := vm.Exec(`
err_kind := nil
err_dimension := nil

llm.with_budget({tokens: 5}, func() {
    llm.with_budget({tokens: 100}, func() {
        first, first_err := llm.turn({messages: {llm.user("first")}})
        second, second_err := llm.turn({messages: {llm.user("second")}})
        err_kind = second_err.kind
        err_dimension = second_err.dimension
    })
})
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			kind, _ := vm.Get("err_kind")
			dimension, _ := vm.Get("err_dimension")
			if kind != "budget" || dimension != "tokens" {
				t.Fatalf("err kind=%#v dimension=%#v", kind, dimension)
			}
		})
	}
}

func TestLLMStandaloneBudgetLimitsToolCallsAndTime(t *testing.T) {
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
						ID:   "call_1",
						Tool: "lookup",
						Args: map[string]any{"query": "leia"},
					}},
				},
				{Status: "final_answer", Text: "unused"},
			}}
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
			}, tc.opts...)
			vm := leia.New(opts...)
			if err := vm.Exec(`
lookup := llm.tool("lookup", func(query) {
    return "found:" .. query, nil
}, {params: {"query"}, requires: {"none"}})

call_kind := nil
call_dimension := nil
time_kind := nil
time_message := nil
time_outcome_source_kind := nil
time_outcome_status := nil
time_outcome_result_status := nil
time_event_type := nil
call_limit_missing := nil
call_used_missing := nil
call_outcome_status := nil

llm.with_budget({calls: 0}, func() {
    result, err := llm.run_agent({
        model: "mock"
        tools: {lookup}
        user: "find leia"
    })
    call_kind = err.kind
    call_dimension = err.dimension
    call_limit_missing = err.limit == nil
    call_used_missing = err.used == nil
    call_outcome_status = llm.budget_outcome(err).status
})

llm.with_budget({time: 0}, func() {
    result, err := llm.turn({messages: {llm.user("deadline")}})
    time_kind = err.kind
    time_message = err.message
    outcome := llm.budget_outcome(err)
    event := llm.budget_outcome_trace_event(outcome, {trace_id: "trace-deadline"})
    time_outcome_source_kind = outcome.source_kind
    time_outcome_status = outcome.status
    time_outcome_result_status = outcome.result_status
    time_event_type = event.event_type
})
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(provider.requests))
			}
			callKind, _ := vm.Get("call_kind")
			callDimension, _ := vm.Get("call_dimension")
			if callKind != "budget" || callDimension != "calls" {
				t.Fatalf("call err kind=%#v dimension=%#v", callKind, callDimension)
			}
			callLimitMissing, _ := vm.Get("call_limit_missing")
			callUsedMissing, _ := vm.Get("call_used_missing")
			callOutcomeStatus, _ := vm.Get("call_outcome_status")
			if callLimitMissing != true || callUsedMissing != true || callOutcomeStatus != "exceeded" {
				t.Fatalf("call limit_missing=%#v used_missing=%#v outcome_status=%#v", callLimitMissing, callUsedMissing, callOutcomeStatus)
			}
			timeKind, _ := vm.Get("time_kind")
			timeMessage, _ := vm.Get("time_message")
			if timeKind != "deadline" || timeMessage != "deadline exceeded" {
				t.Fatalf("time err kind=%#v message=%#v", timeKind, timeMessage)
			}
			timeOutcomeSourceKind, _ := vm.Get("time_outcome_source_kind")
			timeOutcomeStatus, _ := vm.Get("time_outcome_status")
			timeOutcomeResultStatus, _ := vm.Get("time_outcome_result_status")
			timeEventType, _ := vm.Get("time_event_type")
			if timeOutcomeSourceKind != "deadline" || timeOutcomeStatus != "deadline" || timeOutcomeResultStatus != "blocked" || timeEventType != "budget_outcome" {
				t.Fatalf("time outcome source=%#v status=%#v result=%#v event=%#v", timeOutcomeSourceKind, timeOutcomeStatus, timeOutcomeResultStatus, timeEventType)
			}
		})
	}
}
