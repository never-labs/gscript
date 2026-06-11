package bind

import "fmt"

const (
	llmSectionsKey        = "sections"
	llmSectionNameKey     = "name"
	llmSectionPromptKey   = "prompt"
	llmSectionInstrKey    = "instructions"
	llmSectionEvidenceKey = "evidence"
)

type llmRunAgentConfigFunc func(*Table) ([]Value, error)

func registerLLMSectionHelpers(t *Table, runAgentConfig llmRunAgentConfigFunc) {
	runSections := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.sections' (table expected)")
		}
		if runAgentConfig == nil {
			return nil, fmt.Errorf("llm.sections requires an agent runner")
		}
		return llmRunSections(args[0].Table(), runAgentConfig)
	}
	setLLMFunction(t, "llm", "sections", runSections)
	setLLMFunction(t, "llm", "generate_sections", runSections)
}

func llmRunSections(src *Table, runAgentConfig llmRunAgentConfigFunc) ([]Value, error) {
	sectionsValue := src.RawGetString(llmSectionsKey)
	if !sectionsValue.IsTable() {
		return []Value{NilValue(), llmErrorValue("validation", "llm.sections requires a sections table")}, nil
	}
	sectionTables := llmSectionTables(sectionsValue.Table())
	if len(sectionTables) == 0 {
		return []Value{NilValue(), llmErrorValue("validation", "llm.sections requires at least one section")}, nil
	}

	out := NewTable()
	ordered := NewSequentialArrayTable(len(sectionTables))
	results := NewTable()
	values := NewTable()

	for i, section := range sectionTables {
		name := section.RawGetString(llmSectionNameKey).Str()
		if name == "" {
			return []Value{NilValue(), llmErrorValue("validation", fmt.Sprintf("llm.sections section %d requires name", i+1))}, nil
		}
		config := llmSectionAgentConfig(src, section)
		resultVals, err := runAgentConfig(config)
		if err != nil {
			return nil, err
		}
		if len(resultVals) >= 2 && !resultVals[1].IsNil() {
			return []Value{NilValue(), resultVals[1]}, nil
		}
		if len(resultVals) == 0 || !resultVals[0].IsTable() {
			return []Value{NilValue(), llmErrorValue("validation", "llm.sections agent result must be a table")}, nil
		}
		result := resultVals[0]
		resultTable := result.Table()
		value := resultTable.RawGetString("value")
		if value.IsNil() {
			value = result
		}

		entry := NewTable()
		entry.RawSetString("name", StringValue(name))
		entry.RawSetString("result", result)
		entry.RawSetString("value", value)
		entry.RawSetString("text", resultTable.RawGetString("text"))
		entry.RawSetString("status", resultTable.RawGetString("status"))
		ordered.RawSet(IntValue(int64(i+1)), TableValue(entry))
		results.RawSetString(name, result)
		values.RawSetString(name, value)
	}

	out.RawSetString("sections", TableValue(ordered))
	out.RawSetString("results", TableValue(results))
	out.RawSetString("values", TableValue(values))
	return []Value{TableValue(out), NilValue()}, nil
}

func llmSectionTables(sections *Table) []*Table {
	if sections == nil {
		return nil
	}
	out := make([]*Table, 0, sections.Length())
	for i := 1; i <= sections.Length(); i++ {
		if section := sections.RawGet(IntValue(int64(i))).Table(); section != nil {
			out = append(out, section)
		}
	}
	return out
}

func llmSectionAgentConfig(src, section *Table) *Table {
	config := NewTable()
	llmCopyTableExcept(config, src, true, llmSectionsKey, llmSectionEvidenceKey)
	llmCopyTableExcept(config, section, true, llmSectionNameKey, llmSectionPromptKey, llmSectionInstrKey)
	if !section.RawGetString("messages").IsTable() && llmSectionShouldBuildMessages(src, section) {
		config.RawSetString("messages", llmSectionMessages(src, config, section))
	}
	return config
}

func llmSectionShouldBuildMessages(src, section *Table) bool {
	if src.RawGetString("messages").IsTable() {
		return true
	}
	if !src.RawGetString(llmSectionEvidenceKey).IsNil() || !section.RawGetString(llmSectionEvidenceKey).IsNil() {
		return true
	}
	return llmSectionPrompt(section) != ""
}

func llmSectionMessages(src, config, section *Table) Value {
	values := make([]Value, 0, 4)
	if baseMessages := src.RawGetString("messages"); baseMessages.IsTable() {
		values = append(values, llmMessageValuesFromTable(baseMessages.Table())...)
	} else {
		if system := config.RawGetString("system"); !system.IsNil() {
			values = append(values, TableValue(llmMessageTable("system", system.Str())))
		}
		if user := config.RawGetString("user"); !user.IsNil() {
			values = append(values, TableValue(llmMessageTable("user", user.Str())))
		}
	}
	for _, evidence := range []Value{src.RawGetString(llmSectionEvidenceKey), section.RawGetString(llmSectionEvidenceKey)} {
		if msg := llmSectionEvidenceMessage(evidence); !msg.IsNil() {
			values = append(values, msg)
		}
	}
	if prompt := llmSectionPrompt(section); prompt != "" {
		values = append(values, TableValue(llmMessageTable("user", prompt)))
	}
	return llmTableFromValues(values)
}

func llmSectionPrompt(section *Table) string {
	if section == nil {
		return ""
	}
	if prompt := section.RawGetString(llmSectionPromptKey); !prompt.IsNil() {
		return prompt.Str()
	}
	return section.RawGetString(llmSectionInstrKey).Str()
}

func llmSectionEvidenceMessage(evidence Value) Value {
	if evidence.IsNil() {
		return NilValue()
	}
	if evidence.IsTable() {
		messages := llmMessageValuesFromTable(evidence.Table())
		if len(messages) == 1 {
			return messages[0]
		}
	}
	return TableValue(llmMessageTable("user", evidence.Str()))
}

func llmCopyTableExcept(dst, src *Table, overwrite bool, except ...string) {
	if dst == nil || src == nil {
		return
	}
	skip := make(map[string]bool, len(except))
	for _, key := range except {
		skip[key] = true
	}
	for _, key := range src.PairsKeysSnapshot() {
		if key.IsString() && skip[key.Str()] {
			continue
		}
		val := src.RawGet(key)
		if val.IsNil() {
			continue
		}
		if !overwrite && !dst.RawGet(key).IsNil() {
			continue
		}
		dst.RawSet(key, val)
	}
}
