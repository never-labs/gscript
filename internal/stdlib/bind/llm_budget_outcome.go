package bind

import "fmt"

func (b *llmLibBuilder) registerBudgetOutcomeHelpers() {
	budgetOutcome := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.budget_outcome' (table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.budget_outcome' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmBudgetOutcomeValue(args[0].Table(), opts))}, nil
	}
	b.set("budget_outcome", budgetOutcome)
	b.set("budgetOutcome", budgetOutcome)
}

func llmBudgetOutcomeValue(src, opts *Table) *Table {
	out := NewTable()
	out.RawSetString("kind", StringValue("budget_outcome"))
	out.RawSetString("version", StringValue("budget_outcome.v1"))
	sourceKind := llmTraceString(src, "kind", llmTraceString(opts, "kind", ""))
	out.RawSetString("source_kind", StringValue(sourceKind))
	out.RawSetString("ok", BoolValue(true))
	out.RawSetString("blocked", BoolValue(false))
	out.RawSetString("status", StringValue("ok"))
	out.RawSetString("result_status", StringValue("ok"))
	switch sourceKind {
	case "budget":
		out.RawSetString("ok", BoolValue(false))
		out.RawSetString("blocked", BoolValue(true))
		out.RawSetString("status", StringValue("exceeded"))
		out.RawSetString("result_status", StringValue("blocked"))
	case "deadline":
		out.RawSetString("ok", BoolValue(false))
		out.RawSetString("blocked", BoolValue(true))
		out.RawSetString("status", StringValue("deadline"))
		out.RawSetString("result_status", StringValue("blocked"))
	}
	for _, field := range []string{"dimension", "limit", "used", "message"} {
		if value := src.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		}
	}
	for _, field := range []string{"operation", "component", "workflow_run_id", "workflow_step_id", "agent_run_id", "turn_id", "tool_call_id", "correlation_id"} {
		if value := opts.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		} else if value := src.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		}
	}
	return out
}
