package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMToolContractInventoryExportsTypedMetadata(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, tc.opts...)...)
			if err := vm.Exec(`
lookup := llm.tool("lookup", func(query) {
    return {answer: "docs:" .. query}, nil
}, {
    description: "Lookup local documents."
    params: {"query"}
    capabilities: {"docs.read", "replay.local"}
    result: {
        answer: "string"
    }
    error: {
        kind: "validation"
        message: "string"
    }
    replay_key: "lookup:{query}"
})

inventory := llm.tool_info({lookup})
single := llm.tool_info(lookup)
schema_info := llm.tool_schema(lookup)
ok, validate_err := llm.validate_tools({lookup})

inventory_kind := inventory.kind
inventory_cap_1 := inventory.capabilities[1]
tool_name := inventory[1].name
tool_kind := inventory[1].kind
tool_type := inventory[1].type
tool_cap_1 := inventory[1].capabilities[1]
tool_cap_2 := inventory[1].requires[2]
tool_schema_type := inventory[1].schema.type
tool_schema_required := inventory[1].schema.required[1]
tool_result_answer := inventory[1].result.answer
tool_error_kind := inventory[1].error.kind
tool_replay_key := inventory[1].replay_key
single_replay_key := single.replay_key
schema_result_answer := schema_info.result.answer
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"inventory_kind":       "tool_inventory",
				"inventory_cap_1":      "docs.read",
				"tool_name":            "lookup",
				"tool_kind":            "tool_contract",
				"tool_type":            "function",
				"tool_cap_1":           "docs.read",
				"tool_cap_2":           "replay.local",
				"tool_schema_type":     "object",
				"tool_schema_required": "query",
				"tool_result_answer":   "string",
				"tool_error_kind":      "validation",
				"tool_replay_key":      "lookup:{query}",
				"single_replay_key":    "lookup:{query}",
				"schema_result_answer": "string",
				"ok":                   true,
				"validate_err":         nil,
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

func TestLLMValidateToolsReportsMissingContractFields(t *testing.T) {
	vm := leia.New(leia.WithLibs(leia.LibString | leia.LibLLM))
	if err := vm.Exec(`
incomplete := llm.tool("incomplete", func(query) {
    return query, nil
}, {
    params: {"query"}
    result: "string"
    error: "tool_error"
    replay_key: "incomplete:{query}"
})
ok, err := llm.validate_tools({incomplete})
err_kind := err.kind
err_field := err.field
err_tool := err.tool
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	for name, want := range map[string]any{
		"ok":        nil,
		"err_kind":  "validation",
		"err_field": "capabilities",
		"err_tool":  "incomplete",
	} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatalf("Get %s: %v", name, err)
		}
		if got != want {
			t.Fatalf("%s = %#v, want %#v", name, got, want)
		}
	}
}

func TestLLMAgentAsToolContractInventoryKeepsProviderSchemaCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: "done"}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)
			if err := vm.Exec(`
func extract_config(topic) {
    return {
        model: "mock-extractor"
        user: topic
        output: {
            summary: "short finding"
        }
    }, nil
}

extract := llm.agent("extract", extract_config, nil, {
    params: {"topic"}
    output: {
        summary: "short finding"
    }
})

delegate := llm.agent_as_tool(extract, {
    name: "delegate"
    description: "Delegate to extractor."
    capabilities: {"agent.delegate"}
    error: {
        kind: "tool"
        message: "string"
    }
    replay_key: "delegate:{topic}"
})

info := llm.tool_info(delegate)
ok, validate_err := llm.validate_tools({delegate})
result, turn_err := llm.turn({
    model: "mock-supervisor"
    messages: {llm.user("hello")}
    tools: {delegate}
})

info_type := info.type
info_schema_type := info.schema.type
info_required := info.schema.required[1]
info_result_summary := info.result.summary
info_cap := info.capabilities[1]
info_replay_key := info.replay_key
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.last.Tools) != 1 {
				t.Fatalf("tools = %#v", provider.last.Tools)
			}
			if provider.last.Tools[0].Schema != nil {
				t.Fatalf("agent-as-tool provider schema = %#v, want nil", provider.last.Tools[0].Schema)
			}
			for name, want := range map[string]any{
				"info_type":           "agent",
				"info_schema_type":    "object",
				"info_required":       "topic",
				"info_result_summary": "short finding",
				"info_cap":            "agent.delegate",
				"info_replay_key":     "delegate:{topic}",
				"ok":                  true,
				"validate_err":        nil,
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
