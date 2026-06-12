package bind

import "fmt"

func (b *llmLibBuilder) registerModelIOEnvelopeHelpers() {
	envelope := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.model_io_envelope' (model I/O table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.model_io_envelope' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmModelIOEnvelopeValue(args[0].Table(), opts))}, nil
	}
	b.set("model_io_envelope", envelope)
	b.set("modelIOEnvelope", envelope)
	b.set("model_call_envelope", envelope)
	b.set("modelCallEnvelope", envelope)
	b.set("turn_envelope", envelope)
	b.set("turnEnvelope", envelope)
}

func llmModelIOEnvelopeValue(src, opts *Table) *Table {
	out := NewTable()
	out.RawSetString("__llm_model_io_envelope", BoolValue(true))
	out.RawSetString("kind", StringValue("model_io_envelope"))
	out.RawSetString("schema_version", IntValue(int64(llmReplayOptionInt(src, "schema_version", llmReplayOptionInt(opts, "schema_version", 1)))))
	out.RawSetString("version", StringValue(llmReplayOptionString(src, "version", llmReplayOptionString(opts, "version", "model_io_envelope.v1"))))
	out.RawSetString("operation", StringValue(llmReplayOptionString(src, "operation", llmReplayOptionString(opts, "operation", "model.turn"))))
	out.RawSetString("capability", StringValue(llmReplayOptionString(src, "capability", llmReplayOptionString(opts, "capability", "generic.ai.model_io"))))
	out.RawSetString("provider_free", BoolValue(llmReplayOptionBool(src, "provider_free", llmReplayOptionBool(opts, "provider_free", true))))
	out.RawSetString("live_network", BoolValue(llmReplayOptionBool(src, "live_network", llmReplayOptionBool(opts, "live_network", false))))
	out.RawSetString("live_model", BoolValue(llmReplayOptionBool(src, "live_model", llmReplayOptionBool(opts, "live_model", false))))
	out.RawSetString("provider_credentials_required", BoolValue(llmReplayOptionBool(src, "provider_credentials_required", llmReplayOptionBool(opts, "provider_credentials_required", false))))
	out.RawSetString("secret_values_present", BoolValue(false))

	for _, field := range []string{"model", "model_alias", "provider", "route", "replay_key", "fixture_key", "trace_id", "turn_id", "agent_run_id", "workflow_run_id", "workflow_step_id", "correlation_id", "replay_session_id", "component"} {
		if value := opts.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		} else if value := src.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		}
	}
	if out.RawGetString("replay_key").IsNil() && !out.RawGetString("fixture_key").IsNil() {
		out.RawSetString("replay_key", llmCloneValue(out.RawGetString("fixture_key")))
	}

	out.RawSetString("request", TableValue(llmModelIORequestEnvelope(src.RawGetString("request"), src)))
	out.RawSetString("response", TableValue(llmModelIOResponseEnvelope(src.RawGetString("response"), src)))
	out.RawSetString("usage", TableValue(llmModelIOUsageEnvelope(src)))
	out.RawSetString("schema", TableValue(llmModelIOSchemaEnvelope(src, opts)))
	out.RawSetString("refs", TableValue(llmModelIORefsEnvelope(src, opts)))
	out.RawSetString("redaction", TableValue(llmModelIORedaction(src, opts)))
	out.RawSetString("summary", TableValue(llmModelIOSummary(out)))
	return out
}

func llmModelIORequestEnvelope(requestValue Value, src *Table) *Table {
	request := NewTable()
	if requestValue.IsTable() {
		req := requestValue.Table()
		for _, field := range []string{"request_id", "mode", "temperature", "top_p", "max_tokens", "seed", "response_format", "tool_choice"} {
			if value := req.RawGetString(field); !value.IsNil() {
				request.RawSetString(field, llmCloneValue(value))
			}
		}
		if headers := req.RawGetString("headers"); headers.IsTable() {
			request.RawSetString("headers", TableValue(llmReplayRedactedHeaders(headers.Table())))
		}
		if auth := req.RawGetString("auth"); auth.IsTable() {
			authCopy := llmCloneTable(auth.Table())
			authCopy.RawSetString("redacted", BoolValue(true))
			if !authCopy.RawGetString("secret_ref").IsNil() {
				authCopy.RawSetString("secret_ref", StringValue("<redacted>"))
			}
			request.RawSetString("auth", TableValue(authCopy))
		}
		request.RawSetString("messages", TableValue(llmModelIOMessagesSummary(req.RawGetString("messages"))))
		request.RawSetString("tools", TableValue(llmModelIOToolsSummary(req.RawGetString("tools"))))
	} else {
		request.RawSetString("messages", TableValue(llmModelIOMessagesSummary(src.RawGetString("messages"))))
		request.RawSetString("tools", TableValue(llmModelIOToolsSummary(src.RawGetString("tools"))))
	}
	request.RawSetString("raw_prompt_stored", BoolValue(false))
	request.RawSetString("raw_messages_stored", BoolValue(false))
	return request
}

func llmModelIOResponseEnvelope(responseValue Value, src *Table) *Table {
	response := NewTable()
	if responseValue.IsTable() {
		resp := responseValue.Table()
		for _, field := range []string{"response_id", "finish_reason", "stop_reason", "status", "result_status", "error_kind", "retryable"} {
			if value := resp.RawGetString(field); !value.IsNil() {
				response.RawSetString(field, llmCloneValue(value))
			}
		}
		for _, field := range []string{"text", "content", "raw_text", "raw_completion"} {
			if value := resp.RawGetString(field); !value.IsNil() {
				response.RawSetString("text_present", BoolValue(true))
				if value.IsString() {
					response.RawSetString("text_bytes", IntValue(int64(len(value.Str()))))
				}
				break
			}
		}
		response.RawSetString("tool_calls", TableValue(llmModelIOToolCallsSummary(resp.RawGetString("tool_calls"))))
	} else {
		for _, field := range []string{"text", "content", "raw_text", "raw_completion"} {
			if value := src.RawGetString(field); !value.IsNil() {
				response.RawSetString("text_present", BoolValue(true))
				if value.IsString() {
					response.RawSetString("text_bytes", IntValue(int64(len(value.Str()))))
				}
				break
			}
		}
		response.RawSetString("tool_calls", TableValue(llmModelIOToolCallsSummary(src.RawGetString("tool_calls"))))
	}
	if response.RawGetString("text_present").IsNil() {
		response.RawSetString("text_present", BoolValue(false))
	}
	response.RawSetString("raw_completion_stored", BoolValue(false))
	return response
}

func llmModelIOUsageEnvelope(src *Table) *Table {
	usage := NewTable()
	llmModelIOCopyUsageFields(usage, src.RawGetString("usage"))
	if response := src.RawGetString("response"); response.IsTable() {
		llmModelIOCopyUsageFields(usage, response.Table().RawGetString("usage"))
	}
	if usage.RawGetString("total_tokens").IsNil() {
		prompt := usage.RawGetString("prompt_tokens")
		completion := usage.RawGetString("completion_tokens")
		if prompt.IsInt() && completion.IsInt() {
			usage.RawSetString("total_tokens", IntValue(prompt.Int()+completion.Int()))
		}
	}
	return usage
}

func llmModelIOCopyUsageFields(dst *Table, value Value) {
	if !value.IsTable() {
		return
	}
	for _, field := range []string{"prompt_tokens", "completion_tokens", "total_tokens", "input_tokens", "output_tokens", "cached_tokens", "latency_ms", "cost", "cost_usd"} {
		if item := value.Table().RawGetString(field); !item.IsNil() {
			dst.RawSetString(field, llmCloneValue(item))
		}
	}
}

func llmModelIOSchemaEnvelope(src, opts *Table) *Table {
	schema := NewTable()
	for _, field := range []string{"schema", "output_schema", "response_schema", "typed_as"} {
		if value := opts.RawGetString(field); !value.IsNil() {
			schema.RawSetString(field, llmCloneValue(value))
		} else if value := src.RawGetString(field); !value.IsNil() {
			schema.RawSetString(field, llmCloneValue(value))
		}
	}
	if response := src.RawGetString("response"); response.IsTable() {
		for _, field := range []string{"schema", "output_schema", "response_schema", "typed_as"} {
			if value := response.Table().RawGetString(field); !value.IsNil() && schema.RawGetString(field).IsNil() {
				schema.RawSetString(field, llmCloneValue(value))
			}
		}
	}
	return schema
}

func llmModelIORefsEnvelope(src, opts *Table) *Table {
	refs := NewTable()
	for _, field := range []string{"input_refs", "output_refs", "evidence_refs", "artifact_refs", "memory_refs", "tool_refs"} {
		if value := opts.RawGetString(field); !value.IsNil() {
			refs.RawSetString(field, llmCloneValue(value))
		} else if value := src.RawGetString(field); !value.IsNil() {
			refs.RawSetString(field, llmCloneValue(value))
		}
	}
	return refs
}

func llmModelIOMessagesSummary(value Value) *Table {
	summary := NewTable()
	roles := NewSequentialArrayTable(0)
	count := int64(0)
	if value.IsTable() {
		messages := value.Table()
		for i := 1; i <= messages.Length(); i++ {
			msg := messages.RawGet(IntValue(int64(i)))
			if !msg.IsTable() {
				continue
			}
			count++
			role := msg.Table().RawGetString("role").Str()
			if role == "" {
				role = "unknown"
			}
			roles.RawSet(IntValue(int64(roles.Length()+1)), StringValue(role))
		}
	}
	summary.RawSetString("count", IntValue(count))
	summary.RawSetString("roles", TableValue(roles))
	summary.RawSetString("raw_content_stored", BoolValue(false))
	return summary
}

func llmModelIOToolsSummary(value Value) *Table {
	summary := NewTable()
	names := NewSequentialArrayTable(0)
	count := int64(0)
	if value.IsTable() {
		tools := value.Table()
		for i := 1; i <= tools.Length(); i++ {
			tool := tools.RawGet(IntValue(int64(i)))
			if !tool.IsTable() {
				continue
			}
			count++
			name := tool.Table().RawGetString("name").Str()
			if name == "" {
				name = tool.Table().RawGetString("tool").Str()
			}
			if name != "" {
				names.RawSet(IntValue(int64(names.Length()+1)), StringValue(name))
			}
		}
	}
	summary.RawSetString("count", IntValue(count))
	summary.RawSetString("names", TableValue(names))
	return summary
}

func llmModelIOToolCallsSummary(value Value) *Table {
	summary := NewTable()
	names := NewSequentialArrayTable(0)
	count := int64(0)
	if value.IsTable() {
		calls := value.Table()
		for i := 1; i <= calls.Length(); i++ {
			call := calls.RawGet(IntValue(int64(i)))
			if !call.IsTable() {
				continue
			}
			count++
			name := call.Table().RawGetString("name").Str()
			if name == "" {
				name = call.Table().RawGetString("function").Str()
			}
			if name != "" {
				names.RawSet(IntValue(int64(names.Length()+1)), StringValue(name))
			}
		}
	}
	summary.RawSetString("count", IntValue(count))
	summary.RawSetString("names", TableValue(names))
	return summary
}

func llmModelIORedaction(src, opts *Table) *Table {
	redaction := NewTable()
	redaction.RawSetString("enabled", BoolValue(true))
	redaction.RawSetString("policy", StringValue(llmReplayOptionString(opts, "redaction_policy", "model_io_metadata_only")))
	redaction.RawSetString("raw_prompt_stored", BoolValue(false))
	redaction.RawSetString("raw_messages_stored", BoolValue(false))
	redaction.RawSetString("raw_completion_stored", BoolValue(false))
	redaction.RawSetString("secret_values_present", BoolValue(false))
	redaction.RawSetString("headers_redacted", BoolValue(true))
	redaction.RawSetString("auth_redacted", BoolValue(true))
	if existing := src.RawGetString("redaction"); existing.IsTable() {
		llmCopyTable(redaction, existing.Table(), true)
	}
	return redaction
}

func llmModelIOSummary(envelope *Table) *Table {
	summary := NewTable()
	for _, field := range []string{"kind", "operation", "capability", "model", "model_alias", "provider", "replay_key", "provider_free", "live_network", "live_model"} {
		if value := envelope.RawGetString(field); !value.IsNil() {
			summary.RawSetString(field, llmCloneValue(value))
		}
	}
	if request := envelope.RawGetString("request"); request.IsTable() {
		if messages := request.Table().RawGetString("messages"); messages.IsTable() {
			summary.RawSetString("message_count", llmCloneValue(messages.Table().RawGetString("count")))
		}
		if tools := request.Table().RawGetString("tools"); tools.IsTable() {
			summary.RawSetString("tool_count", llmCloneValue(tools.Table().RawGetString("count")))
		}
	}
	if response := envelope.RawGetString("response"); response.IsTable() {
		for _, field := range []string{"finish_reason", "status", "result_status", "text_present"} {
			if value := response.Table().RawGetString(field); !value.IsNil() {
				summary.RawSetString(field, llmCloneValue(value))
			}
		}
	}
	if usage := envelope.RawGetString("usage"); usage.IsTable() {
		for _, field := range []string{"prompt_tokens", "completion_tokens", "total_tokens", "latency_ms", "cost_usd"} {
			if value := usage.Table().RawGetString(field); !value.IsNil() {
				summary.RawSetString(field, llmCloneValue(value))
			}
		}
	}
	return summary
}
