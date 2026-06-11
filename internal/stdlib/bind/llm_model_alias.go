package bind

import "strings"

func llmModelAliasRoute(aliases, opts *Table) (*Table, Value) {
	if aliases == nil {
		return nil, llmErrorValue("config", "model alias table expected")
	}
	if err := llmValidateModelAliases(aliases); err != nil {
		return nil, llmErrorValue("config", err.Error())
	}
	if replay := llmModelAliasReplayDecision(opts); replay != nil {
		return llmModelAliasReplayRoute(replay), NilValue()
	}
	requested := llmModelAliasRequestedModel(aliases, opts)
	if requested == "" {
		return nil, llmErrorValue("config", "model alias route requires a model or default alias")
	}
	resolved, path, spec := llmModelAliasResolveSpec(aliases, requested)
	if resolved == "" {
		resolved = requested
	}

	provider := ""
	providerModel := resolved
	reason := "model"
	if spec != nil {
		provider = llmTableString(spec, "provider")
		providerModel = llmModelAliasProviderModel(spec, resolved)
		reason = "alias"
	}
	if provider == "" && opts != nil {
		provider = llmTableString(opts, "provider")
	}
	if provider == "" {
		provider = "default"
		reason = "default_provider"
	}

	out := NewTable()
	out.RawSetString("alias", StringValue(requested))
	out.RawSetString("model", StringValue(resolved))
	out.RawSetString("provider", StringValue(provider))
	out.RawSetString("provider_model", StringValue(providerModel))
	out.RawSetString("decision", llmModelAliasDecisionID(requested, provider, resolved, providerModel))
	out.RawSetString("trace", TableValue(llmModelAliasTrace(requested, provider, resolved, providerModel, path, reason, false)))
	out.RawSetString("replay", TableValue(llmModelAliasReplay(requested, provider, resolved, providerModel, path)))
	return out, NilValue()
}

func llmModelAliasRequestedModel(aliases, opts *Table) string {
	if opts != nil {
		model := opts.RawGetString("model")
		if model.IsString() && model.Str() != "" {
			return model.Str()
		}
	}
	def := aliases.RawGetString("default")
	if def.IsString() {
		return def.Str()
	}
	if def.IsTable() {
		return "default"
	}
	return ""
}

func llmModelAliasResolveSpec(aliases *Table, requested string) (string, []string, *Table) {
	path := []string{requested}
	name := requested
	for {
		alias := aliases.RawGetString(name)
		switch {
		case alias.IsString() && alias.Str() != "":
			name = alias.Str()
			path = append(path, name)
		case alias.IsTable():
			spec := alias.Table()
			model := llmModelAliasProviderModel(spec, name)
			if model != name {
				path = append(path, model)
			}
			return model, path, spec
		default:
			return name, path, nil
		}
	}
}

func llmModelAliasProviderModel(spec *Table, fallback string) string {
	if spec == nil {
		return fallback
	}
	for _, key := range []string{"provider_model", "model"} {
		value := spec.RawGetString(key)
		if value.IsString() && value.Str() != "" {
			return value.Str()
		}
	}
	return fallback
}

func llmModelAliasTrace(alias, provider, model, providerModel string, path []string, reason string, replayed bool) *Table {
	trace := NewTable()
	trace.RawSetString("type", StringValue("model_route"))
	trace.RawSetString("alias", StringValue(alias))
	trace.RawSetString("provider", StringValue(provider))
	trace.RawSetString("model", StringValue(model))
	trace.RawSetString("provider_model", StringValue(providerModel))
	trace.RawSetString("reason", StringValue(reason))
	trace.RawSetString("replayed", BoolValue(replayed))
	trace.RawSetString("path", llmModelAliasStringArray(path))
	return trace
}

func llmModelAliasReplay(alias, provider, model, providerModel string, path []string) *Table {
	replay := NewTable()
	replay.RawSetString("decision", llmModelAliasDecisionID(alias, provider, model, providerModel))
	replay.RawSetString("alias", StringValue(alias))
	replay.RawSetString("provider", StringValue(provider))
	replay.RawSetString("model", StringValue(model))
	replay.RawSetString("provider_model", StringValue(providerModel))
	replay.RawSetString("path", llmModelAliasStringArray(path))
	return replay
}

func llmModelAliasReplayDecision(opts *Table) *Table {
	if opts == nil {
		return nil
	}
	replay := opts.RawGetString("replay")
	if replay.IsTable() {
		if decision := replay.Table().RawGetString("decision"); decision.IsString() && decision.Str() != "" {
			return replay.Table()
		}
	}
	decision := opts.RawGetString("decision")
	if decision.IsTable() {
		return decision.Table()
	}
	return nil
}

func llmModelAliasReplayRoute(replay *Table) *Table {
	alias := llmTableString(replay, "alias")
	provider := llmTableString(replay, "provider")
	model := llmTableString(replay, "model")
	providerModel := llmTableString(replay, "provider_model")
	path := llmModelAliasPathFromValue(replay.RawGetString("path"), alias, model)

	out := NewTable()
	out.RawSetString("alias", StringValue(alias))
	out.RawSetString("model", StringValue(model))
	out.RawSetString("provider", StringValue(provider))
	out.RawSetString("provider_model", StringValue(providerModel))
	out.RawSetString("decision", replay.RawGetString("decision"))
	out.RawSetString("trace", TableValue(llmModelAliasTrace(alias, provider, model, providerModel, path, "replay", true)))
	out.RawSetString("replay", TableValue(llmConfigCopy(replay)))
	return out
}

func llmModelAliasDecisionID(alias, provider, model, providerModel string) Value {
	parts := []string{
		"alias=" + alias,
		"provider=" + provider,
		"model=" + model,
		"provider_model=" + providerModel,
	}
	return StringValue(strings.Join(parts, "|"))
}

func llmModelAliasStringArray(values []string) Value {
	out := NewAppendArrayTable(len(values))
	for i, value := range values {
		out.RawSet(IntValue(int64(i+1)), StringValue(value))
	}
	return TableValue(out)
}

func llmModelAliasPathFromValue(value Value, alias, model string) []string {
	if value.IsTable() {
		t := value.Table()
		out := make([]string, 0, t.Length())
		for i := 1; i <= t.Length(); i++ {
			item := t.RawGet(IntValue(int64(i)))
			if item.IsString() {
				out = append(out, item.Str())
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	if alias == "" {
		return nil
	}
	if model == "" || model == alias {
		return []string{alias}
	}
	return []string{alias, model}
}
