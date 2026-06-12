package bind

import "testing"

func TestLLMToolRegistryNormalizesValidatesAndTraces(t *testing.T) {
	interp := runLLMTestProgram(t, `
lookup := llm.tool("lookup", func(symbol) {
    return {symbol: symbol price: 42}, nil
}, {
    description: "Lookup fixture-backed market data."
    params: {"symbol"}
    requires: {"generic.tool.registry.declare" "market.read"}
    result: {type: "object"}
    replay_key: "tool:lookup:fixture"
})
write := llm.tool("write_report", func(path) {
    return {path: path}, nil
}, {
    params: {"path"}
    requires: {"generic.tool.approval.edge"}
    result: {type: "object"}
    effect: "effectful"
    approval_policy: "deny_without_approval"
})

registry := llm.tool_registry({lookup, write}, {registry_id: "generic-tool-registry"})
gate := llm.validate_tool_registry(registry)
trace := llm.tool_invocation_trace({
    tool_name: "lookup"
    capability_ids: {"generic.tool.invocation.trace"}
    fixture_key: "generic_tool:invocation:trace:v1"
}, {price: 42}, {trace_id: "trace-tool-1"})

bad_registry := llm.tool_registry({lookup}, {provider_free: false live_network: true})
bad_gate := llm.validateToolRegistry(bad_registry)

missing_registry_ok, missing_registry_err := pcall(llm.tool_registry)
bad_registry_opts_ok, bad_registry_opts_err := pcall(llm.tool_registry, {}, "opts")
missing_validate_ok, missing_validate_err := pcall(llm.validate_tool_registry)
missing_trace_ok, missing_trace_err := pcall(llm.tool_invocation_trace)

kind := registry.kind
marker := registry.__llm_tool_registry
tool_count := registry.tool_count
capability_count := #registry.capabilities
effectful_count := registry.effectful_count
approval_count := registry.approval_required_count
first_tool := registry.descriptors[1].tool_name
first_wire := registry.descriptors[1].provider_wire_format
first_live_network := registry.descriptors[1].live_network
first_secret_allowed := registry.descriptors[1].secret_parameters_allowed
summary_tools := registry.summary.tool_count
redaction_policy := registry.redaction.policy
gate_ok := gate.ok
gate_status := gate.status
gate_findings := gate.finding_count
bad_ok := bad_gate.ok
bad_status := bad_gate.status
bad_findings := bad_gate.finding_count

trace_kind := trace.kind
trace_tool := trace.tool_name
trace_event_count := #trace.events
trace_event_1 := trace.events[1].event
trace_event_5 := trace.events[5].event
trace_input_valid := trace.schema_validation.input_valid
trace_approval_decision := trace.approval.decision
trace_result_ok := trace.result.ok
trace_result_type := trace.result.metadata.result_type
trace_raw_result_stored := trace.result.metadata.raw_result_stored
trace_fixture_key := trace.provenance.fixture_key
trace_redaction := trace.redaction.policy
`, nil)

	for name, want := range map[string]Value{
		"kind":                    StringValue("tool_registry"),
		"marker":                  BoolValue(true),
		"tool_count":              IntValue(2),
		"capability_count":        IntValue(3),
		"effectful_count":         IntValue(1),
		"approval_count":          IntValue(1),
		"first_tool":              StringValue("lookup"),
		"first_wire":              StringValue("none"),
		"first_live_network":      BoolValue(false),
		"first_secret_allowed":    BoolValue(false),
		"summary_tools":           IntValue(2),
		"redaction_policy":        StringValue("tool_registry_metadata_only"),
		"gate_ok":                 BoolValue(true),
		"gate_status":             StringValue("ok"),
		"gate_findings":           IntValue(0),
		"bad_ok":                  BoolValue(false),
		"bad_status":              StringValue("failed"),
		"trace_kind":              StringValue("tool_invocation_trace"),
		"trace_tool":              StringValue("lookup"),
		"trace_event_count":       IntValue(5),
		"trace_event_1":           StringValue("registered"),
		"trace_event_5":           StringValue("result_recorded"),
		"trace_input_valid":       BoolValue(true),
		"trace_approval_decision": StringValue("allow_fixture"),
		"trace_result_ok":         BoolValue(true),
		"trace_result_type":       StringValue("table"),
		"trace_raw_result_stored": BoolValue(false),
		"trace_fixture_key":       StringValue("generic_tool:invocation:trace:v1"),
		"trace_redaction":         StringValue("tool_invocation_metadata_only"),
		"missing_registry_ok":     BoolValue(false),
		"bad_registry_opts_ok":    BoolValue(false),
		"missing_validate_ok":     BoolValue(false),
		"missing_trace_ok":        BoolValue(false),
	} {
		got := interp.GetGlobal(name)
		if !got.Equal(want) {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
	}
	if got := interp.GetGlobal("bad_findings"); !got.IsInt() || got.Int() < 2 {
		t.Fatalf("bad_findings = %v, want at least two findings", got)
	}
	for _, name := range []string{"missing_registry_err", "bad_registry_opts_err", "missing_validate_err", "missing_trace_err"} {
		if got := interp.GetGlobal(name); !got.IsString() || got.Str() == "" {
			t.Fatalf("%s = %v, want non-empty error string", name, got)
		}
	}
}
