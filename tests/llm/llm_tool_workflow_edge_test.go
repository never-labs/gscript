package leia_test

import (
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestLLMWorkflowEdgeRejectsMissingStepFunction(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, mode.opts...)...)
			err := vm.Exec(`
flow := llm.workflow({
    {name: "draft"}
})
`)
			if err == nil || !strings.Contains(err.Error(), "bad workflow step #1 (fn function expected)") {
				t.Fatalf("Exec error = %v, want missing fn validation", err)
			}
		})
	}
}

func TestLLMWorkflowEdgePropagatesStepErrorAndStops(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, mode.opts...)...)
			if err := vm.Exec(`
flow := llm.workflow({
    llm.step("gate", func(ctx) {
        return nil, {kind: "policy", message: "step blocked"}
    }),
    llm.step("should_not_run", func(ctx) {
        return {text: "ran"}, nil
    }),
})
result, err := flow.run("topic")
status := result.status
err_kind := err.kind
err_message := err.message
step_count := #result.steps
first_step_err_kind := result.steps[1].err.kind
second_step_is_nil := result.steps[2] == nil
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"status":              "error",
				"err_kind":            "policy",
				"err_message":         "step blocked",
				"step_count":          int64(1),
				"first_step_err_kind": "policy",
				"second_step_is_nil":  true,
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

func TestLLMWorkflowGraphEdgeRejectsDuplicateAndInvalidOrder(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name+"/duplicate", func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, mode.opts...)...)
			err := vm.Exec(`
graph := llm.workflow_graph({
    stages: {
        llm.stage("plan", func(ctx) { return ctx.input, nil }),
        llm.stage("plan", func(ctx) { return ctx.input, nil }),
    }
})
`)
			if err == nil || !strings.Contains(err.Error(), `duplicate stage "plan"`) {
				t.Fatalf("Exec error = %v, want duplicate stage validation", err)
			}
		})

		t.Run(mode.name+"/depends_on_later_stage", func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, mode.opts...)...)
			err := vm.Exec(`
graph := llm.workflow_graph({
    stages: {
        llm.stage("finalize", func(ctx) { return ctx.input, nil }, {depends_on: {"plan"}}),
        llm.stage("plan", func(ctx) { return ctx.input, nil }),
    }
})
`)
			if err == nil || !strings.Contains(err.Error(), `depends on unknown or later stage "plan"`) {
				t.Fatalf("Exec error = %v, want dependency order validation", err)
			}
		})

		t.Run(mode.name+"/edge_unknown_stage", func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, mode.opts...)...)
			err := vm.Exec(`
graph := llm.workflow_graph({
    stages: {
        llm.stage("plan", func(ctx) { return ctx.input, nil }),
    }
    edges: {
        {from: "plan", to: "missing"}
    }
})
`)
			if err == nil || !strings.Contains(err.Error(), `edge references unknown stage "plan" -> "missing"`) {
				t.Fatalf("Exec error = %v, want unknown edge validation", err)
			}
		})
	}
}

func TestLLMAgentAsToolEdgePropagatesPendingAndExplicitErrors(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, mode.opts...)...)
			if err := vm.Exec(`
func pending_config(topic) {
    return {model: "mock", user: topic}, nil
}

func pending_flow(topic) {
    return {
        status: "pending"
        token: "approval-token"
        payload: {topic: topic}
    }, nil
}

pending_agent := llm.agent("pending_agent", pending_config, pending_flow, {params: ["topic"]})
pending_tool := llm.agent_as_tool(pending_agent, {name: "pending_delegate"})
pending_value, pending_err := llm.dispatch({
    id: "call_pending"
    tool: "pending_delegate"
    args: {topic: "needs approval"}
}, {pending_tool})

func error_config(topic) {
    return {model: "mock", user: topic}, nil
}

func error_flow(topic) {
    return nil, {
        kind: "policy"
        message: "delegated policy denied"
        topic: topic
    }
}

error_agent := llm.agent("error_agent", error_config, error_flow, {params: ["topic"]})
error_tool := llm.toolof(error_agent, {name: "error_delegate"})
error_value, error_err := llm.dispatch({
    id: "call_error"
    tool: "error_delegate"
    args: {topic: "restricted"}
}, {error_tool})

pending_value_is_nil := pending_value == nil
pending_kind := pending_err.kind
pending_message := pending_err.message
pending_status := pending_err.pending.status
pending_token := pending_err.pending.token
pending_topic := pending_err.pending.payload.topic
error_value_is_nil := error_value == nil
error_kind := error_err.kind
error_message := error_err.message
error_topic := error_err.topic
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"pending_value_is_nil": true,
				"pending_kind":         "pending",
				"pending_message":      "delegated agent paused for approval",
				"pending_status":       "pending",
				"pending_token":        "approval-token",
				"pending_topic":        "needs approval",
				"error_value_is_nil":   true,
				"error_kind":           "policy",
				"error_message":        "delegated policy denied",
				"error_topic":          "restricted",
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
