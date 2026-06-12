package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
)

func TestLLMApprovalPolicyDefaultDenyAndReplayTrace(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, mode.opts...)
			vm := leia.New(opts...)
			if err := vm.Exec(`
local_read := llm.tool("local_read", func(name) {
    return "doc:" .. name, nil
}, {params: {"name"}, requires: {"local.read"}})
send_order := llm.tool("send_order", func(id) {
    return "sent:" .. id, nil
}, {params: {"id"}, requires: {"trading.order.submit"}})
publish_artifact := llm.tool("publish_artifact", func(id) {
    return "published:" .. id, nil
}, {params: {"id"}, requires: {"publish.web"}})
call_network := llm.tool("call_network", func(url) {
    return url, nil
}, {params: {"url"}, requires: {"network.http"}})
load_secret := llm.tool("load_secret", func(name) {
    return name, nil
}, {params: {"name"}, requires: {"credential.read"}})
run_generated := llm.tool("run_generated", func(name) {
    return name, nil
}, {params: {"name"}, requires: {"generated-code.execute"}})
rebalance := llm.tool("rebalance", func(name) {
    return name, nil
}, {params: {"name"}, requires: {"portfolio.rebalance"}})

policy := llm.policy()
ok, ok_err := llm.check_policy({local_read}, policy)
_, trading_err := llm.check_policy({send_order}, policy)
_, publish_err := llm.check_policy({publish_artifact}, policy)
_, network_err := llm.check_policy({call_network}, policy)
_, credential_err := llm.check_policy({load_secret}, policy)
_, generated_err := llm.check_policy({run_generated}, policy)
_, portfolio_err := llm.check_policy({rebalance}, policy)
allow_network := llm.policy({allow: {"network.http"}})
network_ok, network_allow_err := llm.check_policy({call_network}, allow_network)
local_outcome := llm.policy_outcome({local_read}, policy)
network_outcome := llm.policy_outcome({call_network}, policy, {approval_required: true})
allowed_network_outcome := llm.policy_outcome({call_network}, allow_network)
clean_skip_outcome := llm.policy_outcome({call_network}, policy, {
    clean_skip: true
    reason: "network disabled by host"
    dependency: "live_network"
})

pending := {id: "call_approval_1", tool: "send_order", args: {id: "order-1"}}
token := loop.snapshot({msg.user("submit order")}, pending)
approval := {
    ok: false
    reason: "default high-risk action denied"
    approval_id: "approval-1"
}
denied, denied_err := loop.resume(token, approval)
replay_trace := llm.approval_trace({
    token: token
    pending: pending
    approval: approval
    result: denied
    policy: policy
})
approval_event := llm.approval_trace_event(replay_trace, {
    trace_id: "trace-approval"
    sequence: 1
    workflow_run_id: "wf-approval"
    workflow_step_id: "step-approval"
})
approval_envelope := llm.trace_envelope({approval_event}, {
    trace_id: "trace-approval"
})
approval_gate := llm.trace_assert(approval_envelope, {
    require_provider_free: true
    deny_live_network: true
    required_event_types: {"approval_replay_trace"}
    require_correlation_fields: {"approval_id", "tool_call_id", "workflow_run_id", "workflow_step_id"}
})
approved_trace := llm.approval_trace({
    pending: {id: "call_approval_2", tool: "send_order"}
    approval: {ok: true, approval_id: "approval-2"}
    result: {status: "approved"}
    policy: policy
})
approved_event := llm.approval_trace_event(approved_trace, {
    trace_id: "trace-approved"
    sequence: 1
})

ok_status := ok
ok_err_is_nil := ok_err == nil
trading_class := trading_err.class
publish_class := publish_err.class
network_class := network_err.class
credential_class := credential_err.class
generated_class := generated_err.class
portfolio_class := portfolio_err.class
network_allow_status := network_ok
network_allow_err_is_nil := network_allow_err == nil
local_outcome_status := local_outcome.status
local_outcome_result_status := local_outcome.result_status
local_outcome_ok := local_outcome.ok
local_outcome_allowed := local_outcome.allowed
local_outcome_capability := local_outcome.capabilities[1]
network_outcome_status := network_outcome.status
network_outcome_result_status := network_outcome.result_status
network_outcome_ok := network_outcome.ok
network_outcome_denied := network_outcome.denied
network_outcome_capability := network_outcome.capability
network_outcome_class := network_outcome.class
network_outcome_policy := network_outcome.policy
network_outcome_approval_required := network_outcome.approval_required
network_outcome_side_effect_allowed := network_outcome.side_effect_allowed
network_outcome_error_kind := network_outcome.error.kind
allowed_network_outcome_status := allowed_network_outcome.status
allowed_network_outcome_result_status := allowed_network_outcome.result_status
allowed_network_outcome_ok := allowed_network_outcome.ok
clean_skip_outcome_status := clean_skip_outcome.status
clean_skip_outcome_result_status := clean_skip_outcome.result_status
clean_skip_outcome_ok := clean_skip_outcome.ok
clean_skip_outcome_clean_skip := clean_skip_outcome.clean_skip
clean_skip_outcome_allowed := clean_skip_outcome.allowed
clean_skip_outcome_reason := clean_skip_outcome.reason
clean_skip_outcome_dependency := clean_skip_outcome.dependency
clean_skip_outcome_side_effect_allowed := clean_skip_outcome.side_effect_allowed
trace_kind := replay_trace.kind
trace_policy_default := replay_trace.policy.default
trace_decision := replay_trace.decision.status
trace_reason := replay_trace.decision.reason
trace_result_status := replay_trace.result.status
trace_pending_tool := replay_trace.pending.tool
event_type := approval_event.event_type
event_status := approval_event.status
event_decision := approval_event.payload.decision
event_reason := approval_event.payload.reason
event_operation := approval_event.payload.operation
event_policy_default := approval_event.payload.policy_default
event_result_status := approval_event.payload.result_status
event_approval_id := approval_event.correlation.approval_id
event_tool_call_id := approval_event.correlation.tool_call_id
event_workflow_run_id := approval_event.correlation.workflow_run_id
event_workflow_step_id := approval_event.correlation.workflow_step_id
event_provider_free := approval_event.provider_free
event_live_network := approval_event.live_network
event_secret_values := approval_event.redaction.secret_values_present
event_raw_args_nil := approval_event.payload.args == nil
event_raw_token_nil := approval_event.payload.token == nil
event_raw_result_nil := approval_event.payload.result == nil
approval_gate_ok := approval_gate.ok
approval_gate_status := approval_gate.status
approved_event_status := approved_event.status
approved_event_decision := approved_event.payload.decision
approved_event_result_status := approved_event.payload.result_status
approved_event_approval_id := approved_event.correlation.approval_id
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]interface{}{
				"ok_status":                              true,
				"ok_err_is_nil":                          true,
				"trading_class":                          "trading",
				"publish_class":                          "publish",
				"network_class":                          "network",
				"credential_class":                       "credential",
				"generated_class":                        "generated-code",
				"portfolio_class":                        "portfolio",
				"network_allow_status":                   true,
				"network_allow_err_is_nil":               true,
				"local_outcome_status":                   "allowed",
				"local_outcome_result_status":            "ok",
				"local_outcome_ok":                       true,
				"local_outcome_allowed":                  true,
				"local_outcome_capability":               "local.read",
				"network_outcome_status":                 "denied",
				"network_outcome_result_status":          "denied",
				"network_outcome_ok":                     false,
				"network_outcome_denied":                 true,
				"network_outcome_capability":             "network.http",
				"network_outcome_class":                  "network",
				"network_outcome_policy":                 "capability_policy.v1",
				"network_outcome_approval_required":      true,
				"network_outcome_side_effect_allowed":    false,
				"network_outcome_error_kind":             "policy",
				"allowed_network_outcome_status":         "allowed",
				"allowed_network_outcome_result_status":  "ok",
				"allowed_network_outcome_ok":             true,
				"clean_skip_outcome_status":              "clean_skip",
				"clean_skip_outcome_result_status":       "skipped",
				"clean_skip_outcome_ok":                  true,
				"clean_skip_outcome_clean_skip":          true,
				"clean_skip_outcome_allowed":             false,
				"clean_skip_outcome_reason":              "network disabled by host",
				"clean_skip_outcome_dependency":          "live_network",
				"clean_skip_outcome_side_effect_allowed": false,
				"trace_kind":                             "approval_replay_trace",
				"trace_policy_default":                   "deny_high_risk",
				"trace_decision":                         "denied",
				"trace_reason":                           "default high-risk action denied",
				"trace_result_status":                    "denied",
				"trace_pending_tool":                     "send_order",
				"event_type":                             "approval_replay_trace",
				"event_status":                           "denied",
				"event_decision":                         "denied",
				"event_reason":                           "default high-risk action denied",
				"event_operation":                        "send_order",
				"event_policy_default":                   "deny_high_risk",
				"event_result_status":                    "denied",
				"event_approval_id":                      "approval-1",
				"event_tool_call_id":                     "call_approval_1",
				"event_workflow_run_id":                  "wf-approval",
				"event_workflow_step_id":                 "step-approval",
				"event_provider_free":                    true,
				"event_live_network":                     false,
				"event_secret_values":                    false,
				"event_raw_args_nil":                     true,
				"event_raw_token_nil":                    true,
				"event_raw_result_nil":                   true,
				"approval_gate_ok":                       true,
				"approval_gate_status":                   "ok",
				"approved_event_status":                  "approved",
				"approved_event_decision":                "approved",
				"approved_event_result_status":           "approved",
				"approved_event_approval_id":             "approval-2",
			} {
				got, _ := vm.Get(name)
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
		})
	}
}
