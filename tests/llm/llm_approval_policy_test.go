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

pending := {id: "call_approval_1", tool: "send_order", args: {id: "order-1"}}
token := loop.snapshot({msg.user("submit order")}, pending)
approval := {ok: false, reason: "default high-risk action denied"}
denied, denied_err := loop.resume(token, approval)
replay_trace := llm.approval_trace({
    token: token
    pending: pending
    approval: approval
    result: denied
    policy: policy
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
trace_kind := replay_trace.kind
trace_policy_default := replay_trace.policy.default
trace_decision := replay_trace.decision.status
trace_reason := replay_trace.decision.reason
trace_result_status := replay_trace.result.status
trace_pending_tool := replay_trace.pending.tool
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]interface{}{
				"ok_status":                true,
				"ok_err_is_nil":            true,
				"trading_class":            "trading",
				"publish_class":            "publish",
				"network_class":            "network",
				"credential_class":         "credential",
				"generated_class":          "generated-code",
				"portfolio_class":          "portfolio",
				"network_allow_status":     true,
				"network_allow_err_is_nil": true,
				"trace_kind":               "approval_replay_trace",
				"trace_policy_default":     "deny_high_risk",
				"trace_decision":           "denied",
				"trace_reason":             "default high-risk action denied",
				"trace_result_status":      "denied",
				"trace_pending_tool":       "send_order",
			} {
				got, _ := vm.Get(name)
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
		})
	}
}
