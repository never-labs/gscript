package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMTurnRequestProviderOptions(t *testing.T) {
	provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: "done"}}
	vm := leia.New(leia.WithLibs(leia.LibString|leia.LibLLM), leia.WithLLMProvider(provider))
	if err := vm.Exec(`
result, err := llm.turn({
    model: "mock-fast",
    messages: {llm.user("hello")},
    force_tool: "lookup",
    max_tokens: 16,
    temperature: 0.25,
    top_p: 0.9,
    response_format: {type: "json_object"},
    stream: true,
    stop: {"END", "\n\n"},
    metadata: {trace_id: "abc", route: "test"},
})
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if provider.last.Model != "mock-fast" || provider.last.MaxTokens != 16 || !provider.last.Stream {
		t.Fatalf("request = %#v", provider.last)
	}
	if provider.last.ForceTool != "lookup" {
		t.Fatalf("force_tool = %#v", provider.last.ForceTool)
	}
	if provider.last.Temperature == nil || *provider.last.Temperature != 0.25 {
		t.Fatalf("temperature = %#v", provider.last.Temperature)
	}
	if provider.last.TopP == nil || *provider.last.TopP != 0.9 {
		t.Fatalf("top_p = %#v", provider.last.TopP)
	}
	format, _ := provider.last.ResponseFormat.(map[string]any)
	if format["type"] != "json_object" {
		t.Fatalf("response_format = %#v", provider.last.ResponseFormat)
	}
	if len(provider.last.Stop) != 2 || provider.last.Stop[0] != "END" || provider.last.Stop[1] != "\n\n" {
		t.Fatalf("stop = %#v", provider.last.Stop)
	}
	if provider.last.Metadata["trace_id"] != "abc" || provider.last.Metadata["route"] != "test" {
		t.Fatalf("metadata = %#v", provider.last.Metadata)
	}
}

func TestLLMToolMetadata(t *testing.T) {
	provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: "done"}}
	vm := leia.New(leia.WithLibs(leia.LibString|leia.LibLLM), leia.WithLLMProvider(provider))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {
    description: "lookup docs",
    params: {"name"},
    requires: {"docs.read", "net.client"},
    schema: {
        type: "object",
        properties: {name: {type: "string"}},
        required: {"name"},
    },
})
result, err := llm.turn({
    messages: {llm.user("hello")},
    tools: {lookup},
})
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(provider.last.Tools) != 1 {
		t.Fatalf("tools = %#v", provider.last.Tools)
	}
	tool := provider.last.Tools[0]
	if tool.Name != "lookup" || tool.Description != "lookup docs" {
		t.Fatalf("tool = %#v", tool)
	}
	if len(tool.Requires) != 2 || tool.Requires[0] != "docs.read" || tool.Requires[1] != "net.client" {
		t.Fatalf("requires = %#v", tool.Requires)
	}
	schema, ok := tool.Schema.(map[string]any)
	if !ok || schema["type"] != "object" {
		t.Fatalf("schema = %#v", tool.Schema)
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v", schema["properties"])
	}
	name, ok := props["name"].(map[string]any)
	if !ok || name["type"] != "string" {
		t.Fatalf("name schema = %#v", props["name"])
	}
}

func TestLLMToolParamsGenerateProviderSchema(t *testing.T) {
	provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: "done"}}
	vm := leia.New(leia.WithLibs(leia.LibString|leia.LibLLM), leia.WithLLMProvider(provider))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(query, source) {
    return query .. ":" .. source, nil
}, {
    description: "lookup docs",
    params: {"query", "source"},
    requires: {"docs.read"},
    output: {answer: "string"},
})
info := llm.tool_schema(lookup)
info_schema_type := info.schema.type
info_required_1 := info.schema.required[1]
info_required_2 := info.schema.required[2]
info_prop_query_type := info.schema.properties.query.type
info_output_answer := info.output.answer
result, err := llm.turn({
    messages: {llm.user("hello")},
    tools: {lookup},
})
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(provider.last.Tools) != 1 {
		t.Fatalf("tools = %#v", provider.last.Tools)
	}
	schema, ok := provider.last.Tools[0].Schema.(map[string]any)
	if !ok || schema["type"] != "object" {
		t.Fatalf("schema = %#v", provider.last.Tools[0].Schema)
	}
	required, ok := schema["required"].([]any)
	if !ok || len(required) != 2 || required[0] != "query" || required[1] != "source" {
		t.Fatalf("required = %#v", schema["required"])
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v", schema["properties"])
	}
	query, ok := props["query"].(map[string]any)
	if !ok || query["type"] != "string" {
		t.Fatalf("query schema = %#v", props["query"])
	}
	for name, want := range map[string]interface{}{
		"info_schema_type":     "object",
		"info_required_1":      "query",
		"info_required_2":      "source",
		"info_prop_query_type": "string",
		"info_output_answer":   "string",
	} {
		got, _ := vm.Get(name)
		if got != want {
			t.Fatalf("%s = %#v, want %#v", name, got, want)
		}
	}
}

func TestLLMToolExplicitSchemaWinsOverGeneratedParamsSchema(t *testing.T) {
	provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: "done"}}
	vm := leia.New(leia.WithLibs(leia.LibString|leia.LibLLM), leia.WithLLMProvider(provider))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(query) {
    return query, nil
}, {
    params: {"query"},
    schema: {
        type: "object",
        properties: {query: {type: "string", description: "search terms"}},
        required: {"query"},
        additionalProperties: false,
    },
})
info := llm.toolSchema({lookup})
info_additional := info[1].schema.additionalProperties
result, err := llm.turn({
    messages: {llm.user("hello")},
    tools: {lookup},
})
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	schema, ok := provider.last.Tools[0].Schema.(map[string]any)
	if !ok || schema["additionalProperties"] != false {
		t.Fatalf("schema = %#v", provider.last.Tools[0].Schema)
	}
	got, _ := vm.Get("info_additional")
	if got != false {
		t.Fatalf("info_additional = %#v, want false", got)
	}
}

func TestDialectToolUsesLLMToolSchemaHelper(t *testing.T) {
	vm := leia.New(leia.WithLibs(leia.LibString | leia.LibLLM | leia.LibDialect))
	if err := vm.Exec(`
search_runbook := tool {
    name: "search_runbook"
    params: {"service"}
    description: "Search local runbooks."
    requires: {"docs.read", "runbooks.read"}
    fn: func(service) {
        return "runbook:" .. service, nil
    }
}
info := llm.tool_schema({search_runbook})
tool_name := info[1].name
schema_type := info[1].schema.type
required_1 := info[1].schema.required[1]
cap_1 := info[1].requires[1]
cap_2 := info[1].requires[2]
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	for name, want := range map[string]interface{}{
		"tool_name":   "search_runbook",
		"schema_type": "object",
		"required_1":  "service",
		"cap_1":       "docs.read",
		"cap_2":       "runbooks.read",
	} {
		got, _ := vm.Get(name)
		if got != want {
			t.Fatalf("%s = %#v, want %#v", name, got, want)
		}
	}
}

func TestLLMToolCapabilities(t *testing.T) {
	vm := leia.New(leia.WithLibs(leia.LibString | leia.LibLLM))
	if err := vm.Exec(`
read_docs := llm.tool("read_docs", func(name) {
    return "docs:" .. name, nil
}, {requires: {"docs.read", "net.client"}})
refund := llm.tool("refund", func(id) {
    return id, nil
}, {requires: {"payments.refund"}})
tools := {read_docs, refund}
caps := llm.tool_caps(tools)
ok, ok_err := llm.check_tools(tools, {"docs.read", "net.client", "payments.refund"})
missing, missing_err := llm.check_tools(tools, {"docs.read", "net.client"})
all_ok, all_err := llm.check_tools(tools, {"cap.all"})
cap1 := caps[1]
cap2 := caps[2]
cap3 := caps[3]
missing_kind := missing_err.kind
missing_cap := missing_err.capability
missing_tool := missing_err.tool
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	for name, want := range map[string]interface{}{
		"cap1":         "docs.read",
		"cap2":         "net.client",
		"cap3":         "payments.refund",
		"ok":           true,
		"ok_err":       nil,
		"missing":      nil,
		"missing_kind": "capability",
		"missing_cap":  "payments.refund",
		"missing_tool": "refund",
		"all_ok":       true,
		"all_err":      nil,
	} {
		got, _ := vm.Get(name)
		if got != want {
			t.Fatalf("%s = %#v, want %#v", name, got, want)
		}
	}
}

func TestLoopRequestProviderOptions(t *testing.T) {
	provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: "done"}}
	vm := leia.New(leia.WithLibs(leia.LibString|leia.LibLLM), leia.WithLLMProvider(provider))
	if err := vm.Exec(`
lookup := llm.tool("lookup", func(name) {
    return "docs:" .. name, nil
}, {params: {"name"}})
result, err := loop.react({
    user: "hello",
    model: "mock-fast",
    tools: {lookup},
    force_tool: lookup,
    max_tokens: 32,
    stream: true,
    stop: {"DONE"},
    metadata: {trace_id: "loop-1"},
})
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("requests = %d", len(provider.requests))
	}
	req := provider.requests[0]
	if req.Model != "mock-fast" || req.MaxTokens != 32 || !req.Stream {
		t.Fatalf("request = %#v", req)
	}
	if req.ForceTool != "lookup" {
		t.Fatalf("force_tool = %#v", req.ForceTool)
	}
	if len(req.Stop) != 1 || req.Stop[0] != "DONE" {
		t.Fatalf("stop = %#v", req.Stop)
	}
	if req.Metadata["trace_id"] != "loop-1" {
		t.Fatalf("metadata = %#v", req.Metadata)
	}
}
