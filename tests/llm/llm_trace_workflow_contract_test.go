package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
)

func TestLLMWorkflowTraceContractIncludesStepErrorMetadata(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
			}, tc.opts...)...)
			if err := vm.Exec(`
flow := llm.workflow({
    llm.step("collect", func(ctx) {
        return {value: "draft", text: "draft"}, nil
    }),
    llm.step("spend", func(ctx) {
        return nil, {
            kind: "budget"
            dimension: "tokens"
            limit: 5
            used: 6
            message: "budget exceeded"
        }
    }),
    llm.step("unused", func(ctx) {
        return "unused", nil
    }),
})
result, err := flow.run("topic")
status := result.status
err_kind := err.kind
workflow_trace_type := result.trace.type
workflow_trace_status := result.trace.status
child_count := #result.trace.children
first_step_name := result.trace.children[1].name
first_step_status := result.trace.children[1].status
first_step_parent := result.trace.children[1].parent.type
second_step_name := result.steps[2].trace.name
second_step_status := result.steps[2].trace.status
second_step_error_kind := result.steps[2].trace.error.kind
second_step_budget_dimension := result.steps[2].trace.budget.dimension
second_step_metadata_index := result.steps[2].trace.metadata.index
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"status":                       "error",
				"err_kind":                     "budget",
				"workflow_trace_type":          "workflow",
				"workflow_trace_status":        "error",
				"child_count":                  int64(2),
				"first_step_name":              "collect",
				"first_step_status":            "ok",
				"first_step_parent":            "workflow",
				"second_step_name":             "spend",
				"second_step_status":           "error",
				"second_step_error_kind":       "budget",
				"second_step_budget_dimension": "tokens",
				"second_step_metadata_index":   int64(2),
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

func TestLLMAgentAsToolTraceContractExposesHandoffParentChild(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
			}, tc.opts...)...)
			if err := vm.Exec(`
func specialist_config(topic) {
    return {model: "mock", user: topic}, nil
}

func specialist_flow(topic) {
    return {
        status: "done"
        value: {
            summary: "checked " .. topic
        }
        text: "checked " .. topic
    }, nil
}

specialist := llm.agent("specialist", specialist_config, specialist_flow, {params: {"topic"}})
handoff := llm.handoff(specialist, {name: "handoff_specialist"})
value, err := llm.dispatch({
    id: "call_handoff"
    tool: "handoff_specialist"
    args: {topic: "trace"}
}, {handoff})
summary := value.summary
tool_contract := handoff.trace_contract
trace_type := value.trace.type
trace_name := value.trace.name
trace_status := value.trace.status
child_type := value.trace.children[1].type
child_name := value.trace.children[1].name
child_parent_name := value.trace.children[1].parent.name
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"summary":           "checked trace",
				"tool_contract":     "agent_tool.v1",
				"trace_type":        "agent_tool",
				"trace_name":        "handoff_specialist",
				"trace_status":      "done",
				"child_type":        "agent",
				"child_name":        "specialist",
				"child_parent_name": "handoff_specialist",
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
