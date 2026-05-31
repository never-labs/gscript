package gscript_test

import (
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

func TestLLMToolMetadata(t *testing.T) {
	provider := &mockLLMProvider{res: gs.LLMTurnResult{Status: "final_answer", Text: "done"}}
	vm := gs.New(gs.WithLibs(gs.LibString|gs.LibLLM), gs.WithLLMProvider(provider))
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

func TestLLMToolCapabilities(t *testing.T) {
	vm := gs.New(gs.WithLibs(gs.LibString | gs.LibLLM))
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
