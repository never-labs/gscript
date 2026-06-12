package bind

import "fmt"

func (b *llmLibBuilder) registerToolRegistryHelpers() {
	toolRegistry := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.tool_registry' (tools table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.tool_registry' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmToolRegistryValue(args[0].Table(), opts))}, nil
	}
	b.set("tool_registry", toolRegistry)
	b.set("toolRegistry", toolRegistry)

	validateRegistry := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.validate_tool_registry' (tool registry table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.validate_tool_registry' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmValidateToolRegistryValue(args[0].Table(), opts))}, nil
	}
	b.set("validate_tool_registry", validateRegistry)
	b.set("validateToolRegistry", validateRegistry)

	invocationTrace := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.tool_invocation_trace' (tool call table expected)")
		}
		result := NilValue()
		if len(args) >= 2 {
			result = args[1]
		}
		opts := NewTable()
		if len(args) >= 3 {
			if !args[2].IsTable() {
				return nil, fmt.Errorf("bad argument #3 to 'llm.tool_invocation_trace' (options table expected)")
			}
			opts = args[2].Table()
		}
		return []Value{TableValue(llmToolInvocationTraceValue(args[0].Table(), result, opts))}, nil
	}
	b.set("tool_invocation_trace", invocationTrace)
	b.set("toolInvocationTrace", invocationTrace)
}

func llmToolRegistryValue(tools, opts *Table) *Table {
	out := NewTable()
	out.RawSetString("__llm_tool_registry", BoolValue(true))
	out.RawSetString("kind", StringValue("tool_registry"))
	out.RawSetString("schema_version", IntValue(int64(llmReplayOptionInt(opts, "schema_version", 1))))
	out.RawSetString("version", StringValue(llmReplayOptionString(opts, "version", "tool_registry.v1")))
	out.RawSetString("mode", StringValue(llmReplayOptionString(opts, "mode", llmReplayModeFixture)))
	out.RawSetString("capability", StringValue(llmReplayOptionString(opts, "capability", "generic.tool.registry.declare")))
	out.RawSetString("provider_free", BoolValue(llmReplayOptionBool(opts, "provider_free", true)))
	out.RawSetString("live_network", BoolValue(llmReplayOptionBool(opts, "live_network", false)))
	out.RawSetString("real_dependency_imports", BoolValue(llmReplayOptionBool(opts, "real_dependency_imports", false)))
	out.RawSetString("local_execution", BoolValue(llmReplayOptionBool(opts, "local_execution", false)))
	out.RawSetString("provider_credentials_required", BoolValue(llmReplayOptionBool(opts, "provider_credentials_required", false)))
	out.RawSetString("secret_values_present", BoolValue(false))
	for _, field := range []string{"registry_id", "package_id", "package_name", "workflow_run_id", "workflow_step_id", "trace_id", "correlation_id", "replay_session_id"} {
		if value := opts.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		}
	}

	descriptors := NewSequentialArrayTable(0)
	for i := 1; i <= tools.Length(); i++ {
		item := tools.RawGet(IntValue(int64(i)))
		if item.IsTable() && llmLooksLikeToolTable(item.Table()) {
			descriptors.RawSet(IntValue(int64(descriptors.Length()+1)), TableValue(llmSingleToolContractTable(item.Table())))
		}
	}
	out.RawSetString("tools", TableValue(descriptors))
	out.RawSetString("descriptors", TableValue(descriptors))
	out.RawSetString("tool_count", IntValue(int64(descriptors.Length())))
	out.RawSetString("capabilities", TableValue(llmToolRegistryCapabilities(descriptors)))
	out.RawSetString("effectful_count", IntValue(int64(llmToolRegistryCountDescriptors(descriptors, "effect", "effectful"))))
	out.RawSetString("approval_required_count", IntValue(int64(llmToolRegistryApprovalRequiredCount(descriptors))))
	out.RawSetString("redaction", TableValue(llmToolRegistryRedaction(opts)))
	out.RawSetString("summary", TableValue(llmToolRegistrySummary(out)))
	return out
}

func llmValidateToolRegistryValue(registry, opts *Table) *Table {
	out := NewTable()
	out.RawSetString("kind", StringValue("tool_registry_validation"))
	out.RawSetString("schema_version", IntValue(1))
	out.RawSetString("version", StringValue("tool_registry_validation.v1"))
	out.RawSetString("provider_free", BoolValue(true))
	findings := NewSequentialArrayTable(0)
	addFinding := func(kind, message, field string) {
		f := NewTable()
		f.RawSetString("kind", StringValue(kind))
		f.RawSetString("message", StringValue(message))
		if field != "" {
			f.RawSetString("field", StringValue(field))
		}
		findings.RawSet(IntValue(int64(findings.Length()+1)), TableValue(f))
	}
	if !llmReplayOptionBool(registry, "provider_free", false) {
		addFinding("provider_free", "tool registry must be provider-free", "provider_free")
	}
	if llmReplayOptionBool(registry, "live_network", false) {
		addFinding("live_network", "tool registry must not require live network", "live_network")
	}
	if llmReplayOptionBool(registry, "real_dependency_imports", false) {
		addFinding("real_dependency_imports", "tool registry must not require real dependency imports", "real_dependency_imports")
	}
	if llmReplayOptionBool(registry, "provider_credentials_required", false) {
		addFinding("provider_credentials_required", "tool registry must not require provider credentials", "provider_credentials_required")
	}
	descriptors := registry.RawGetString("descriptors")
	if !descriptors.IsTable() {
		descriptors = registry.RawGetString("tools")
	}
	if !descriptors.IsTable() {
		addFinding("descriptors", "tool registry must contain descriptors", "descriptors")
	} else {
		llmValidateToolRegistryDescriptors(descriptors.Table(), opts, addFinding)
	}
	out.RawSetString("findings", TableValue(findings))
	out.RawSetString("finding_count", IntValue(int64(findings.Length())))
	ok := findings.Length() == 0
	out.RawSetString("ok", BoolValue(ok))
	if ok {
		out.RawSetString("status", StringValue("ok"))
	} else {
		out.RawSetString("status", StringValue("failed"))
	}
	return out
}

func llmValidateToolRegistryDescriptors(descriptors, opts *Table, addFinding func(kind, message, field string)) {
	seen := map[string]bool{}
	for i := 1; i <= descriptors.Length(); i++ {
		item := descriptors.RawGet(IntValue(int64(i)))
		if !item.IsTable() {
			addFinding("descriptor", "tool descriptor must be a table", "descriptors")
			continue
		}
		desc := item.Table()
		name := desc.RawGetString("tool_name").Str()
		if name == "" {
			name = desc.RawGetString("name").Str()
		}
		if name == "" {
			addFinding("tool_name", "tool descriptor must have a tool name", "tool_name")
		} else if seen[name] {
			addFinding("duplicate_tool", "tool descriptor names must be unique", "tool_name")
		}
		seen[name] = true
		if caps := desc.RawGetString("capability_ids"); !caps.IsTable() || caps.Table().Length() == 0 {
			addFinding("capability_ids", "tool descriptor must declare capability ids", "capability_ids")
		}
		if desc.RawGetString("input_schema").IsNil() {
			addFinding("input_schema", "tool descriptor must expose input schema", "input_schema")
		}
		if desc.RawGetString("output_schema").IsNil() {
			addFinding("output_schema", "tool descriptor must expose output schema", "output_schema")
		}
		if desc.RawGetString("provider_wire_format").Str() != "none" {
			addFinding("provider_wire_format", "tool descriptor must not require provider wire format", "provider_wire_format")
		}
		if desc.RawGetString("live_network").Bool() {
			addFinding("live_network", "tool descriptor must not require live network", "live_network")
		}
		if desc.RawGetString("secret_parameters_allowed").Bool() {
			addFinding("secret_parameters_allowed", "tool descriptor must not allow secret parameters", "secret_parameters_allowed")
		}
		if desc.RawGetString("effect").Str() == "effectful" {
			policy := desc.RawGetString("approval_policy").Str()
			if policy == "" || policy == "not_required_for_fixture" {
				addFinding("approval_policy", "effectful tools must declare an approval policy", "approval_policy")
			}
		}
	}
}

func llmToolInvocationTraceValue(call *Table, result Value, opts *Table) *Table {
	out := NewTable()
	out.RawSetString("kind", StringValue("tool_invocation_trace"))
	out.RawSetString("schema_version", IntValue(1))
	out.RawSetString("version", StringValue("tool_invocation_trace.v1"))
	out.RawSetString("provider_free", BoolValue(true))
	out.RawSetString("live_network", BoolValue(false))
	out.RawSetString("real_dependency_imports", BoolValue(false))
	out.RawSetString("local_execution", BoolValue(false))
	out.RawSetString("trace_id", StringValue(llmReplayOptionString(opts, "trace_id", llmReplayOptionString(call, "trace_id", "tool-trace:1"))))
	toolName := llmToolOutcomeString(opts, call, call, "tool_name", "tool", "name")
	out.RawSetString("tool_name", StringValue(toolName))
	out.RawSetString("caller_id", StringValue(llmReplayOptionString(opts, "caller_id", llmReplayOptionString(call, "caller_id", "caller"))))
	out.RawSetString("executor_id", StringValue(llmReplayOptionString(opts, "executor_id", llmReplayOptionString(call, "executor_id", "tool"))))
	out.RawSetString("capability_ids", TableValue(llmToolInvocationTraceCapabilities(call, opts)))
	out.RawSetString("events", TableValue(llmToolInvocationTraceEvents()))
	out.RawSetString("schema_validation", TableValue(llmToolInvocationTraceSchemaValidation(opts)))
	out.RawSetString("approval", TableValue(llmToolInvocationTraceApproval(call, opts)))
	out.RawSetString("result", TableValue(llmToolInvocationTraceResult(result, opts)))
	out.RawSetString("provenance", TableValue(llmToolInvocationTraceProvenance(call, opts)))
	out.RawSetString("redaction", TableValue(llmToolInvocationTraceRedaction(opts)))
	return out
}

func llmToolRegistryCapabilities(descriptors *Table) *Table {
	out := NewSequentialArrayTable(0)
	seen := map[string]bool{}
	for i := 1; i <= descriptors.Length(); i++ {
		item := descriptors.RawGet(IntValue(int64(i)))
		if !item.IsTable() {
			continue
		}
		caps := item.Table().RawGetString("capability_ids")
		if !caps.IsTable() {
			caps = item.Table().RawGetString("capabilities")
		}
		if !caps.IsTable() {
			continue
		}
		for j := 1; j <= caps.Table().Length(); j++ {
			cap := caps.Table().RawGet(IntValue(int64(j))).Str()
			if cap == "" || seen[cap] {
				continue
			}
			seen[cap] = true
			out.RawSet(IntValue(int64(out.Length()+1)), StringValue(cap))
		}
	}
	return out
}

func llmToolRegistryCountDescriptors(descriptors *Table, field, want string) int {
	count := 0
	for i := 1; i <= descriptors.Length(); i++ {
		item := descriptors.RawGet(IntValue(int64(i)))
		if item.IsTable() && item.Table().RawGetString(field).Str() == want {
			count++
		}
	}
	return count
}

func llmToolRegistryApprovalRequiredCount(descriptors *Table) int {
	count := 0
	for i := 1; i <= descriptors.Length(); i++ {
		item := descriptors.RawGet(IntValue(int64(i)))
		if !item.IsTable() {
			continue
		}
		policy := item.Table().RawGetString("approval_policy").Str()
		if policy != "" && policy != "not_required_for_fixture" {
			count++
		}
	}
	return count
}

func llmToolRegistryRedaction(opts *Table) *Table {
	redaction := NewTable()
	redaction.RawSetString("enabled", BoolValue(true))
	redaction.RawSetString("policy", StringValue(llmReplayOptionString(opts, "redaction_policy", "tool_registry_metadata_only")))
	redaction.RawSetString("raw_args_stored", BoolValue(false))
	redaction.RawSetString("raw_results_stored", BoolValue(false))
	redaction.RawSetString("secret_values_present", BoolValue(false))
	return redaction
}

func llmToolRegistrySummary(registry *Table) *Table {
	summary := NewTable()
	for _, field := range []string{"kind", "registry_id", "package_id", "tool_count", "effectful_count", "approval_required_count", "provider_free", "live_network", "real_dependency_imports"} {
		if value := registry.RawGetString(field); !value.IsNil() {
			summary.RawSetString(field, llmCloneValue(value))
		}
	}
	if caps := registry.RawGetString("capabilities"); caps.IsTable() {
		summary.RawSetString("capability_count", IntValue(int64(caps.Table().Length())))
	}
	return summary
}

func llmToolInvocationTraceCapabilities(call, opts *Table) *Table {
	if caps := opts.RawGetString("capability_ids"); caps.IsTable() {
		return llmCloneTable(caps.Table())
	}
	if caps := call.RawGetString("capability_ids"); caps.IsTable() {
		return llmCloneTable(caps.Table())
	}
	if caps := call.RawGetString("capabilities"); caps.IsTable() {
		return llmCloneTable(caps.Table())
	}
	if cap := llmReplayOptionString(opts, "capability", llmReplayOptionString(call, "capability", "")); cap != "" {
		out := NewSequentialArrayTable(1)
		out.RawSet(IntValue(1), StringValue(cap))
		return out
	}
	return NewSequentialArrayTable(0)
}

func llmToolInvocationTraceEvents() *Table {
	events := NewSequentialArrayTable(0)
	for i, name := range []string{"registered", "schema_validated", "approval_checked", "invoked", "result_recorded"} {
		event := NewTable()
		event.RawSetString("event", StringValue(name))
		event.RawSetString("sequence", IntValue(int64(i+1)))
		events.RawSet(IntValue(int64(i+1)), TableValue(event))
	}
	return events
}

func llmToolInvocationTraceSchemaValidation(opts *Table) *Table {
	out := NewTable()
	out.RawSetString("input_valid", BoolValue(llmReplayOptionBool(opts, "input_valid", true)))
	out.RawSetString("output_valid", BoolValue(llmReplayOptionBool(opts, "output_valid", true)))
	out.RawSetString("additional_properties_rejected", BoolValue(llmReplayOptionBool(opts, "additional_properties_rejected", true)))
	return out
}

func llmToolInvocationTraceApproval(call, opts *Table) *Table {
	out := NewTable()
	required := llmReplayOptionBool(opts, "approval_required", llmReplayOptionBool(call, "approval_required", false))
	out.RawSetString("required", BoolValue(required))
	if required {
		out.RawSetString("decision", StringValue(llmReplayOptionString(opts, "approval_decision", llmReplayOptionString(call, "approval_decision", "deny"))))
	} else {
		out.RawSetString("decision", StringValue(llmReplayOptionString(opts, "approval_decision", llmReplayOptionString(call, "approval_decision", "allow_fixture"))))
	}
	if id := llmReplayOptionString(opts, "approval_id", llmReplayOptionString(call, "approval_id", "")); id != "" {
		out.RawSetString("approval_id", StringValue(id))
	} else {
		out.RawSetString("approval_id", NilValue())
	}
	out.RawSetString("reason", StringValue(llmReplayOptionString(opts, "approval_reason", llmReplayOptionString(call, "approval_reason", ""))))
	return out
}

func llmToolInvocationTraceResult(result Value, opts *Table) *Table {
	out := NewTable()
	isError := llmToolOutcomeIsError(result)
	ok := llmReplayOptionBool(opts, "ok", !isError)
	status := llmReplayOptionString(opts, "status", "ok")
	if isError {
		status = llmReplayOptionString(opts, "status", "error")
	}
	if decision := llmReplayOptionString(opts, "approval_decision", ""); decision == "deny" {
		ok = false
		status = llmReplayOptionString(opts, "status", "denied")
	}
	out.RawSetString("ok", BoolValue(ok))
	out.RawSetString("status", StringValue(status))
	out.RawSetString("content", StringValue(llmReplayOptionString(opts, "content", "")))
	metadata := NewTable()
	metadata.RawSetString("result_present", BoolValue(!result.IsNil()))
	metadata.RawSetString("result_type", StringValue(llmToolOutcomeValueType(result)))
	metadata.RawSetString("raw_result_stored", BoolValue(false))
	if extra := opts.RawGetString("metadata"); extra.IsTable() {
		llmCopyTable(metadata, extra.Table(), true)
	}
	out.RawSetString("metadata", TableValue(metadata))
	return out
}

func llmToolInvocationTraceProvenance(call, opts *Table) *Table {
	out := NewTable()
	out.RawSetString("provider_free", BoolValue(true))
	out.RawSetString("live_network", BoolValue(false))
	out.RawSetString("real_dependency_imports", BoolValue(false))
	out.RawSetString("local_execution", BoolValue(false))
	out.RawSetString("fixture_key", StringValue(llmReplayOptionString(opts, "fixture_key", llmReplayOptionString(call, "fixture_key", llmReplayOptionString(call, "replay_key", "")))))
	return out
}

func llmToolInvocationTraceRedaction(opts *Table) *Table {
	redaction := NewTable()
	redaction.RawSetString("enabled", BoolValue(true))
	redaction.RawSetString("policy", StringValue(llmReplayOptionString(opts, "redaction_policy", "tool_invocation_metadata_only")))
	redaction.RawSetString("raw_args_stored", BoolValue(false))
	redaction.RawSetString("raw_result_stored", BoolValue(false))
	redaction.RawSetString("secret_values_present", BoolValue(false))
	return redaction
}
