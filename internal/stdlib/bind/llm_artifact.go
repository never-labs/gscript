package bind

func llmReportArtifactContractValue(opts *Table) Value {
	name := "report_artifact_contract"
	version := "FR-GAP-012"
	if opts != nil {
		if v := opts.RawGetString("name"); v.IsString() && v.Str() != "" {
			name = v.Str()
		}
		if v := opts.RawGetString("version"); v.IsString() && v.Str() != "" {
			version = v.Str()
		}
	}

	out := NewTable()
	out.RawSetString("name", StringValue(name))
	out.RawSetString("version", StringValue(version))
	out.RawSetString("offline_verifiable", BoolValue(true))
	out.RawSetString("renderer_required", BoolValue(false))
	out.RawSetString("schemas", TableValue(llmReportArtifactSchemas()))
	out.RawSetString("manifest_template", TableValue(llmReportArtifactManifestTemplate(version)))
	out.RawSetString("required_markers", stringArrayValue([]string{
		"report_sections",
		"chart_specs",
		"artifact_manifest",
		"source_annotations",
		"stale_data_warning",
		"ai_disclosure",
	}))
	return TableValue(out)
}

func llmReportArtifactSchemas() *Table {
	schemas := NewTable()
	schemas.RawSetString("report_section", llmReportSectionSchemaValue())
	schemas.RawSetString("chart_spec", llmChartSpecSchemaValue())
	schemas.RawSetString("artifact_manifest", llmArtifactManifestSchemaValue())
	schemas.RawSetString("source_annotation", llmSourceAnnotationSchemaValue())
	return schemas
}

func llmReportSectionSchemaValue() Value {
	return llmSchemaValue(llmArtifactFields(
		llmArtifactField{"id", StringValue("string")},
		llmArtifactField{"title", StringValue("string")},
		llmArtifactField{"order", StringValue("integer")},
		llmArtifactField{"required", StringValue("boolean")},
		llmArtifactField{"content", StringValue("string")},
		llmArtifactField{"chart_refs", llmArtifactArrayOf(StringValue("string"))},
		llmArtifactField{"source_refs", llmArtifactArrayOf(StringValue("string"))},
		llmArtifactField{"ai_disclosure", StringValue("boolean")},
		llmArtifactField{"disclosure_ref", StringValue("string?")},
	))
}

func llmChartSpecSchemaValue() Value {
	return llmSchemaValue(llmArtifactFields(
		llmArtifactField{"id", StringValue("string")},
		llmArtifactField{"title", StringValue("string")},
		llmArtifactField{"section_id", StringValue("string")},
		llmArtifactField{"kind", StringValue("string")},
		llmArtifactField{"renderer", StringValue("string")},
		llmArtifactField{"source_refs", llmArtifactArrayOf(StringValue("string"))},
		llmArtifactField{"x", StringValue("string")},
		llmArtifactField{"series", llmArtifactArrayOf(StringValue("string"))},
		llmArtifactField{"artifact", llmArtifactFields(
			llmArtifactField{"id", StringValue("string")},
			llmArtifactField{"kind", StringValue("string")},
			llmArtifactField{"path", StringValue("string")},
			llmArtifactField{"status", StringValue("string")},
		)},
	))
}

func llmArtifactManifestSchemaValue() Value {
	return llmSchemaValue(llmArtifactFields(
		llmArtifactField{"contract", StringValue("string")},
		llmArtifactField{"report_id", StringValue("string")},
		llmArtifactField{"generated_at", StringValue("string")},
		llmArtifactField{"report_sections", llmArtifactArrayOf(StringValue("string"))},
		llmArtifactField{"chart_specs", llmArtifactArrayOf(StringValue("string"))},
		llmArtifactField{"artifacts", llmArtifactArrayOf(llmArtifactFields(
			llmArtifactField{"id", StringValue("string")},
			llmArtifactField{"kind", StringValue("string")},
			llmArtifactField{"path", StringValue("string")},
			llmArtifactField{"status", StringValue("string")},
		))},
		llmArtifactField{"source_annotations", llmArtifactArrayOf(llmSourceAnnotationSchemaValue())},
		llmArtifactField{"warnings", llmArtifactArrayOf(StringValue("string"))},
		llmArtifactField{"ai_disclosure", StringValue("string")},
	))
}

func llmSourceAnnotationSchemaValue() Value {
	return llmSchemaValue(llmArtifactFields(
		llmArtifactField{"id", StringValue("string")},
		llmArtifactField{"title", StringValue("string")},
		llmArtifactField{"kind", StringValue("string")},
		llmArtifactField{"locator", StringValue("string")},
		llmArtifactField{"as_of", StringValue("string")},
		llmArtifactField{"stale_after", StringValue("string")},
		llmArtifactField{"stale", StringValue("boolean")},
		llmArtifactField{"license", StringValue("string?")},
		llmArtifactField{"retrieved_at", StringValue("string?")},
		llmArtifactField{"evidence_hash", StringValue("string?")},
	))
}

func llmReportArtifactManifestTemplate(version string) *Table {
	manifest := NewTable()
	manifest.RawSetString("contract", StringValue(version))
	manifest.RawSetString("renderer", StringValue("planned_not_rendered"))
	manifest.RawSetString("offline_verifiable", BoolValue(true))
	manifest.RawSetString("warnings", TableValue(NewAppendArrayTable(0)))
	return manifest
}

type llmArtifactField struct {
	name  string
	value Value
}

func llmArtifactFields(fields ...llmArtifactField) Value {
	t := NewTable()
	for _, field := range fields {
		t.RawSetString(field.name, field.value)
	}
	return TableValue(t)
}

func llmArtifactArrayOf(item Value) Value {
	t := NewTable()
	t.RawSetInt(1, item)
	return TableValue(t)
}
