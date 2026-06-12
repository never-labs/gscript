package bind

import "fmt"

func (b *llmLibBuilder) registerApprovalHelpers() {
	traceFn := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.approval_trace' (table expected)")
		}
		return []Value{TableValue(llmApprovalReplayTraceValue(args[0].Table()))}, nil
	}
	b.set("approval_trace", traceFn)
	b.set("approvalTrace", traceFn)
}

func llmApprovalReplayTraceValue(src *Table) *Table {
	out := NewTable()
	out.RawSetString("kind", StringValue("approval_replay_trace"))
	out.RawSetString("version", StringValue("approval_replay.v1"))
	for _, key := range []string{"token", "pending", "approval", "result"} {
		if v := src.RawGetString(key); !v.IsNil() {
			out.RawSetString(key, llmCloneValue(v))
		}
	}
	if policy := src.RawGetString("policy"); policy.IsTable() {
		out.RawSetString("policy", llmCloneValue(policy))
	} else {
		out.RawSetString("policy", TableValue(llmCapabilityPolicyValue(nil)))
	}
	if approval := out.RawGetString("approval"); approval.IsTable() {
		out.RawSetString("decision", llmApprovalDecisionValue(approval.Table()))
	}
	return out
}

func llmApprovalDecisionValue(approval *Table) Value {
	decision := NewTable()
	if approval.RawGetString("ok").Truthy() {
		decision.RawSetString("status", StringValue("approved"))
	} else {
		decision.RawSetString("status", StringValue("denied"))
	}
	if reason := approval.RawGetString("reason"); !reason.IsNil() {
		decision.RawSetString("reason", reason)
	}
	return TableValue(decision)
}
