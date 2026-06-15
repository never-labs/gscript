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
    params: ["query"]
    capabilities: ["docs.read", "replay.local"]
    result: {
        answer: "string"
    }
    error: {
        kind: "validation"
        message: "string"
    }
    replay_key: "lookup:{query}"
    caller_role: "caller"
    executor_role: "executor"
    effect: "read_only"
    approval_policy: "not_required_for_fixture"
    provider_wire_format: "none"
    live_network: false
    secret_parameters_allowed: false
})

inventory := llm.tool_info([lookup])
single := llm.tool_info(lookup)
descriptor := llm.tool_descriptor(lookup)
schema_info := llm.tool_schema(lookup)
ok, validate_err := llm.validate_tools([lookup])

inventory_kind := inventory.kind
inventory_cap_1 := inventory.capabilities[1]
tool_name := inventory[1].name
tool_descriptor_name := inventory[1].tool_name
tool_kind := inventory[1].kind
tool_type := inventory[1].type
tool_capability_id := inventory[1].capability_ids[1]
tool_cap_1 := inventory[1].capabilities[1]
tool_cap_2 := inventory[1].requires[2]
tool_schema_type := inventory[1].schema.type
tool_input_schema_type := inventory[1].input_schema.type
tool_schema_required := inventory[1].schema.required[1]
tool_result_answer := inventory[1].result.answer
tool_output_answer := inventory[1].output_schema.answer
tool_error_kind := inventory[1].error.kind
tool_replay_key := inventory[1].replay_key
tool_effect := inventory[1].effect
tool_approval_policy := inventory[1].approval_policy
tool_provider_wire_format := inventory[1].provider_wire_format
tool_live_network := inventory[1].live_network
tool_secret_params := inventory[1].secret_parameters_allowed
single_replay_key := single.replay_key
descriptor_tool_name := descriptor.tool_name
descriptor_capability_id := descriptor.capability_ids[2]
descriptor_input_type := descriptor.input_schema.type
descriptor_output_answer := descriptor.output_schema.answer
schema_result_answer := schema_info.result.answer
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"inventory_kind":            "tool_inventory",
				"inventory_cap_1":           "docs.read",
				"tool_name":                 "lookup",
				"tool_descriptor_name":      "lookup",
				"tool_kind":                 "tool_contract",
				"tool_type":                 "function",
				"tool_capability_id":        "docs.read",
				"tool_cap_1":                "docs.read",
				"tool_cap_2":                "replay.local",
				"tool_schema_type":          "object",
				"tool_input_schema_type":    "object",
				"tool_schema_required":      "query",
				"tool_result_answer":        "string",
				"tool_output_answer":        "string",
				"tool_error_kind":           "validation",
				"tool_replay_key":           "lookup:{query}",
				"tool_effect":               "read_only",
				"tool_approval_policy":      "not_required_for_fixture",
				"tool_provider_wire_format": "none",
				"tool_live_network":         false,
				"tool_secret_params":        false,
				"single_replay_key":         "lookup:{query}",
				"descriptor_tool_name":      "lookup",
				"descriptor_capability_id":  "replay.local",
				"descriptor_input_type":     "object",
				"descriptor_output_answer":  "string",
				"schema_result_answer":      "string",
				"ok":                        true,
				"validate_err":              nil,
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
    params: ["query"]
    result: "string"
    error: "tool_error"
    replay_key: "incomplete:{query}"
})
ok, err := llm.validate_tools([incomplete])
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

func TestLLMValidateToolsRejectsUnsafeProviderFreeDescriptorFields(t *testing.T) {
	vm := leia.New(leia.WithLibs(leia.LibString | leia.LibLLM))
	if err := vm.Exec(`
unsafe_tool := llm.tool("unsafe_tool", func(query) {
    return {answer: query}, nil
}, {
    description: "Unsafe external tool."
    params: ["query"]
    capabilities: ["generic.tool.registry.declare"]
    result: {answer: "string"}
    error: {kind: "validation" message: "string"}
    replay_key: "unsafe:{query}"
    provider_wire_format: "http"
    live_network: true
    secret_parameters_allowed: true
})
ok, err := llm.validate_tools([unsafe_tool])
err_kind := err.kind
err_field := err.field
err_tool := err.tool
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	for name, want := range map[string]any{
		"ok":        nil,
		"err_kind":  "validation",
		"err_field": "provider_wire_format",
		"err_tool":  "unsafe_tool",
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
    params: ["topic"]
    output: {
        summary: "short finding"
    }
})

delegate := llm.agent_as_tool(extract, {
    name: "delegate"
    description: "Delegate to extractor."
    capabilities: ["agent.delegate"]
    error: {
        kind: "tool"
        message: "string"
    }
    replay_key: "delegate:{topic}"
})

info := llm.tool_info(delegate)
ok, validate_err := llm.validate_tools([delegate])
result, turn_err := llm.turn({
    model: "mock-supervisor"
    messages: [llm.user("hello")]
    tools: [delegate]
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
