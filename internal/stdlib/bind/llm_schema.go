package bind

import (
	"fmt"
	"strings"
)

func llmSchemaValue(spec Value) Value {
	return llmNormalizeSchemaValue(spec, true)
}

func llmSchemaInfoValue(spec Value) Value {
	out := NewTable()
	schema := llmSchemaValue(spec)
	out.RawSetString("schema", schema)
	out.RawSetString("json_schema", schema)
	out.RawSetString("kind", StringValue(llmSchemaKind(schema)))
	return TableValue(out)
}

func llmOutputSchemaValue(args []Value) (Value, error) {
	name := "output"
	var spec Value
	var opts *Table
	switch {
	case len(args) == 1:
		spec = args[0]
	case len(args) >= 2 && args[0].IsString():
		name = args[0].Str()
		spec = args[1]
		if len(args) >= 3 && args[2].IsTable() {
			opts = args[2].Table()
		}
	case len(args) >= 2:
		spec = args[0]
		if args[1].IsTable() {
			opts = args[1].Table()
			if n := opts.RawGetString("name"); n.IsString() && n.Str() != "" {
				name = n.Str()
			}
		}
	default:
		return NilValue(), fmt.Errorf("bad argument to 'llm.output_schema' (schema spec expected)")
	}
	if spec.IsNil() {
		return NilValue(), fmt.Errorf("bad argument to 'llm.output_schema' (schema spec expected)")
	}
	schema := llmSchemaValue(spec)
	if opts != nil {
		if n := opts.RawGetString("name"); n.IsString() && n.Str() != "" {
			name = n.Str()
		}
	}
	return TableValue(llmJSONSchemaResponseFormatTable(name, schema, opts)), nil
}

func llmJSONSchemaResponseFormatTable(name string, schema Value, opts *Table) *Table {
	format := NewTable()
	format.RawSetString("type", StringValue("json_schema"))
	jsonSchema := NewTable()
	if name == "" {
		name = "output"
	}
	jsonSchema.RawSetString("name", StringValue(name))
	jsonSchema.RawSetString("schema", llmCloneValue(schema))
	strict := BoolValue(true)
	if opts != nil && !opts.RawGetString("strict").IsNil() {
		strict = BoolValue(opts.RawGetString("strict").Truthy())
	}
	jsonSchema.RawSetString("strict", strict)
	format.RawSetString("json_schema", TableValue(jsonSchema))
	return format
}

func llmOutputJSONSchemaForValue(v Value) (Value, bool) {
	if !llmLooksLikeJSONSchema(v) {
		return NilValue(), false
	}
	return llmNormalizeSchemaValue(v, true), true
}

func llmNormalizeSchemaValue(v Value, root bool) Value {
	if v.IsString() {
		return TableValue(llmPrimitiveSchemaTable(v.Str()))
	}
	if v.IsNumber() {
		return TableValue(llmTypedSchemaTable("number"))
	}
	if v.IsBool() {
		return TableValue(llmTypedSchemaTable("boolean"))
	}
	if !v.IsTable() {
		return TableValue(llmTypedSchemaTable("string"))
	}
	t := v.Table()
	if llmLooksLikeJSONSchema(v) {
		return TableValue(llmNormalizeExplicitSchemaTable(t))
	}
	if fields := t.RawGetString("fields"); fields.IsTable() {
		return TableValue(llmObjectSchemaFromFields(fields.Table()))
	}
	if t.Length() > 0 {
		item := t.RawGet(IntValue(1))
		arraySchema := NewTable()
		arraySchema.RawSetString("type", StringValue("array"))
		arraySchema.RawSetString("items", llmNormalizeSchemaValue(item, false))
		return TableValue(arraySchema)
	}
	if root {
		return TableValue(llmObjectSchemaFromFields(t))
	}
	return TableValue(llmNormalizeFieldDescriptorTable(t))
}

func llmLooksLikeJSONSchema(v Value) bool {
	if !v.IsTable() {
		return false
	}
	t := v.Table()
	if typ := t.RawGetString("type"); typ.IsString() && llmJSONSchemaTypeName(typ.Str()) != "" {
		return true
	}
	for _, key := range []string{"properties", "anyOf", "oneOf", "allOf", "$schema"} {
		if !t.RawGetString(key).IsNil() {
			return true
		}
	}
	return false
}

func llmNormalizeExplicitSchemaTable(src *Table) *Table {
	out := NewTable()
	for _, key := range src.PairsKeysSnapshot() {
		val := src.RawGet(key)
		if key.IsString() {
			switch key.Str() {
			case "type":
				if val.IsString() {
					out.RawSet(key, StringValue(llmJSONSchemaTypeName(val.Str())))
					continue
				}
			case "properties":
				if val.IsTable() {
					props := NewTable()
					for _, propKey := range val.Table().PairsKeysSnapshot() {
						props.RawSet(propKey, llmNormalizeSchemaValue(val.Table().RawGet(propKey), false))
					}
					out.RawSet(key, TableValue(props))
					continue
				}
			case "items":
				out.RawSet(key, llmNormalizeSchemaValue(val, false))
				continue
			case "fields":
				if val.IsTable() && out.RawGetString("properties").IsNil() {
					obj := llmObjectSchemaFromFields(val.Table())
					for _, objKey := range obj.PairsKeysSnapshot() {
						out.RawSet(objKey, obj.RawGet(objKey))
					}
				}
				continue
			}
		}
		out.RawSet(key, llmCloneValue(val))
	}
	if out.RawGetString("type").IsNil() && !out.RawGetString("properties").IsNil() {
		out.RawSetString("type", StringValue("object"))
	}
	if out.RawGetString("type").Str() == "object" && out.RawGetString("additionalProperties").IsNil() {
		out.RawSetString("additionalProperties", BoolValue(false))
	}
	return out
}

func llmObjectSchemaFromFields(fields *Table) *Table {
	schema := NewTable()
	schema.RawSetString("type", StringValue("object"))
	props := NewTable()
	required := NewTable()
	for _, key := range fields.PairsKeysSnapshot() {
		if !key.IsString() {
			continue
		}
		name := key.Str()
		field, isRequired := llmNormalizeFieldSchema(fields.RawGet(key))
		props.RawSetString(name, field)
		if isRequired {
			required.RawSet(IntValue(int64(required.Length()+1)), StringValue(name))
		}
	}
	schema.RawSetString("properties", TableValue(props))
	schema.RawSetString("required", TableValue(required))
	schema.RawSetString("additionalProperties", BoolValue(false))
	return schema
}

func llmNormalizeFieldSchema(v Value) (Value, bool) {
	required := true
	if v.IsString() {
		text := v.Str()
		if strings.HasSuffix(text, "?") {
			required = false
			text = strings.TrimSuffix(text, "?")
		}
		return TableValue(llmPrimitiveSchemaTable(text)), required
	}
	if v.IsTable() {
		t := v.Table()
		if opt := t.RawGetString("optional"); !opt.IsNil() && opt.Truthy() {
			required = false
		}
		if req := t.RawGetString("required"); !req.IsNil() {
			required = req.Truthy()
		}
		return TableValue(llmNormalizeFieldDescriptorTable(t)), required
	}
	return llmNormalizeSchemaValue(v, false), required
}

func llmNormalizeFieldDescriptorTable(src *Table) *Table {
	if src.Length() > 0 {
		if schema := llmNormalizeSchemaValue(TableValue(src), false); schema.IsTable() {
			return schema.Table()
		}
	}
	if llmLooksLikeJSONSchema(TableValue(src)) {
		normalized := llmNormalizeExplicitSchemaTable(src)
		out := NewTable()
		for _, key := range normalized.PairsKeysSnapshot() {
			if key.IsString() && (key.Str() == "required" || key.Str() == "optional") {
				continue
			}
			out.RawSet(key, normalized.RawGet(key))
		}
		return out
	}
	out := NewTable()
	typ := "object"
	if t := src.RawGetString("type"); t.IsString() {
		typ = llmJSONSchemaTypeName(t.Str())
	}
	out.RawSetString("type", StringValue(typ))
	if desc := src.RawGetString("description"); desc.IsString() {
		out.RawSetString("description", desc)
	}
	if items := src.RawGetString("items"); !items.IsNil() {
		out.RawSetString("items", llmNormalizeSchemaValue(items, false))
	}
	if fields := src.RawGetString("fields"); fields.IsTable() {
		obj := llmObjectSchemaFromFields(fields.Table())
		out.RawSetString("type", StringValue("object"))
		out.RawSetString("properties", obj.RawGetString("properties"))
		out.RawSetString("required", obj.RawGetString("required"))
		out.RawSetString("additionalProperties", BoolValue(false))
	}
	if enum := src.RawGetString("enum"); enum.IsTable() {
		out.RawSetString("enum", llmCloneValue(enum))
	}
	if format := src.RawGetString("format"); format.IsString() {
		out.RawSetString("format", format)
	}
	return out
}

func llmPrimitiveSchemaTable(text string) *Table {
	typ := llmJSONSchemaTypeName(text)
	if typ == "" {
		typ = "string"
	}
	return llmTypedSchemaTable(typ)
}

func llmTypedSchemaTable(typ string) *Table {
	t := NewTable()
	t.RawSetString("type", StringValue(typ))
	if typ == "object" {
		t.RawSetString("properties", TableValue(NewTable()))
		t.RawSetString("required", TableValue(NewTable()))
		t.RawSetString("additionalProperties", BoolValue(false))
	}
	return t
}

func llmJSONSchemaTypeName(text string) string {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "str", "string", "":
		return "string"
	case "num", "number", "float", "double":
		return "number"
	case "int", "integer":
		return "integer"
	case "bool", "boolean":
		return "boolean"
	case "object", "table":
		return "object"
	case "array", "list":
		return "array"
	case "null":
		return "null"
	default:
		return ""
	}
}

func llmSchemaKind(schema Value) string {
	if schema.IsTable() {
		if typ := schema.Table().RawGetString("type"); typ.IsString() {
			return typ.Str()
		}
	}
	return ""
}

func llmValidateStructuredOutputSchema(schema, actual Value, path string) string {
	if !schema.IsTable() {
		return ""
	}
	t := schema.Table()
	typ := t.RawGetString("type").Str()
	if typ != "" && !llmJSONSchemaTypeMatches(typ, actual) {
		return fmt.Sprintf("structured output field %q has type %s, want %s", path, llmStructuredOutputActualType(actual), typ)
	}
	if enum := t.RawGetString("enum"); enum.IsTable() && !llmSchemaEnumContains(enum.Table(), actual) {
		return fmt.Sprintf("structured output field %q is not one of the allowed values", path)
	}
	switch typ {
	case "object", "":
		if props := t.RawGetString("properties"); props.IsTable() {
			if !actual.IsTable() {
				return fmt.Sprintf("structured output field %q has type %s, want object", path, llmStructuredOutputActualType(actual))
			}
			required := llmStringSliceFromValue(t.RawGetString("required"))
			for _, name := range required {
				if actual.Table().RawGetString(name).IsNil() {
					return fmt.Sprintf("structured output missing field %q", llmStructuredOutputPath(path, name))
				}
			}
			for _, key := range props.Table().PairsKeysSnapshot() {
				if !key.IsString() {
					continue
				}
				value := actual.Table().RawGetString(key.Str())
				if value.IsNil() {
					continue
				}
				if msg := llmValidateStructuredOutputSchema(props.Table().RawGet(key), value, llmStructuredOutputPath(path, key.Str())); msg != "" {
					return msg
				}
			}
		}
	case "array":
		if !actual.IsTable() {
			return fmt.Sprintf("structured output field %q has type %s, want array", path, llmStructuredOutputActualType(actual))
		}
		items := t.RawGetString("items")
		for i := 1; i <= actual.Table().Length(); i++ {
			item := actual.Table().RawGet(IntValue(int64(i)))
			if item.IsNil() {
				return fmt.Sprintf("structured output missing field %q", llmStructuredOutputIndexPath(path, i))
			}
			if msg := llmValidateStructuredOutputSchema(items, item, llmStructuredOutputIndexPath(path, i)); msg != "" {
				return msg
			}
		}
	}
	return ""
}

func llmJSONSchemaTypeMatches(typ string, v Value) bool {
	switch typ {
	case "string":
		return v.IsString()
	case "number":
		return v.IsNumber()
	case "integer":
		return v.IsNumber()
	case "boolean":
		return v.IsBool()
	case "object", "array":
		return v.IsTable()
	case "null":
		return v.IsNil()
	default:
		return true
	}
}

func llmSchemaEnumContains(enum *Table, actual Value) bool {
	for i := 1; i <= enum.Length(); i++ {
		if llmSchemaScalarEqual(enum.RawGet(IntValue(int64(i))), actual) {
			return true
		}
	}
	return false
}

func llmSchemaScalarEqual(a, b Value) bool {
	switch {
	case a.IsString() && b.IsString():
		return a.Str() == b.Str()
	case a.IsBool() && b.IsBool():
		return a.Bool() == b.Bool()
	case a.IsNumber() && b.IsNumber():
		return a.Number() == b.Number()
	default:
		return false
	}
}
