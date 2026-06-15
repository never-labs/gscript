package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMSchemaLightweightFieldDescriptions(t *testing.T) {
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
schema := llm.schema({
    name: {type: "string", description: "Display name"}
    score: "number"
    nickname: "string?"
    tags: ["string"]
})
kind := llm.schema_info(schema).kind
ok, msg := llm.validate_output({name: "Ada", score: 0.98, tags: ["math"]}, schema)
schema_type := schema.type
schema_additional := schema.additionalProperties
name_desc := schema.properties.name.description
score_type := schema.properties.score.type
tags_type := schema.properties.tags.type
tag_item_type := schema.properties.tags.items.type
required_1 := schema.required[1]
required_2 := schema.required[2]
required_3 := schema.required[3]
validation := tostring(ok) .. "|" .. msg
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]interface{}{
				"kind":              "object",
				"schema_type":       "object",
				"schema_additional": false,
				"name_desc":         "Display name",
				"score_type":        "number",
				"tags_type":         "array",
				"tag_item_type":     "string",
				"required_1":        "name",
				"required_2":        "score",
				"required_3":        "tags",
				"validation":        "true|",
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

func TestLLMToolSchemaNormalizesLightweightSchema(t *testing.T) {
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
lookup := llm.tool("lookup", func(query, limit) {
    return query, nil
	}, {
	    params: ["query", "limit"]
	    schema: llm.schema({
	        query: {type: "string", description: "Search query"}
	        limit: "integer?"
	    })
	})
info := llm.tool_schema(lookup)
schema_type := info.schema.type
query_type := info.schema.properties.query.type
query_desc := info.schema.properties.query.description
limit_type := info.schema.properties.limit.type
required_1 := info.schema.required[1]
required_2_is_nil := info.schema.required[2] == nil
result, err := llm.turn({
    model: "mock"
    messages: [llm.user("hello")]
    tools: [lookup]
})
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]interface{}{
				"schema_type":       "object",
				"query_type":        "string",
				"query_desc":        "Search query",
				"limit_type":        "integer",
				"required_1":        "query",
				"required_2_is_nil": true,
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
			schema, ok := provider.last.Tools[0].Schema.(map[string]any)
			if !ok || schema["type"] != "object" || schema["additionalProperties"] != false {
				t.Fatalf("provider schema = %#v", provider.last.Tools[0].Schema)
			}
			props, ok := schema["properties"].(map[string]any)
			if !ok {
				t.Fatalf("provider properties = %#v", schema["properties"])
			}
			query, ok := props["query"].(map[string]any)
			if !ok || query["type"] != "string" || query["description"] != "Search query" {
				t.Fatalf("provider query schema = %#v", props["query"])
			}
			required, ok := schema["required"].([]any)
			if !ok || len(required) != 1 || required[0] != "query" {
				t.Fatalf("provider required = %#v", schema["required"])
			}
		})
	}
}

func TestLLMOutputSchemaUsesJSONSchemaResponseFormat(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: `{"name":"Ada","score":0.99}`}}
			vm := leia.New(llmScenarioOptions(provider, tc.opts...)...)
			if err := vm.Exec(`
schema := llm.schema({
    name: "string"
    score: "number"
})
format := llm.output_schema("contact", schema)
format_name := format.json_schema.name
format_strict := format.json_schema.strict
result, err := llm.turn({
    model: "mock-json"
    messages: [llm.user("Extract Ada.")]
    response_format: format
})
value_ok, value_msg := llm.validate_output(result.text, schema)
validation := tostring(value_ok) .. "|" .. value_msg
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			format, ok := provider.last.ResponseFormat.(map[string]any)
			if !ok || format["type"] != "json_schema" {
				t.Fatalf("response_format = %#v", provider.last.ResponseFormat)
			}
			jsonSchema, ok := format["json_schema"].(map[string]any)
			if !ok || jsonSchema["name"] != "contact" || jsonSchema["strict"] != true {
				t.Fatalf("json_schema = %#v", format["json_schema"])
			}
			schema, ok := jsonSchema["schema"].(map[string]any)
			if !ok || schema["type"] != "object" {
				t.Fatalf("schema = %#v", jsonSchema["schema"])
			}
			required, ok := schema["required"].([]any)
			if !ok || len(required) != 2 || required[0] != "name" || required[1] != "score" {
				t.Fatalf("required = %#v", schema["required"])
			}
			for name, want := range map[string]interface{}{
				"format_name":   "contact",
				"format_strict": true,
				"validation":    "true|",
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
