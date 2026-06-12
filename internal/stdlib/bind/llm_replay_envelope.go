package bind

import "strings"

func llmReplayHTTPRecordValue(src, opts *Table) *Table {
	out := NewTable()
	for _, key := range src.PairsKeysSnapshot() {
		switch key.Str() {
		case "request", "response":
			continue
		}
		out.RawSet(key, llmCloneValue(src.RawGet(key)))
	}
	out.RawSetString("__llm_replay_http_record", BoolValue(true))
	out.RawSetString("kind", StringValue("replay_http_record"))
	out.RawSetString("schema_version", IntValue(int64(llmReplayOptionInt(src, "schema_version", llmReplayOptionInt(opts, "schema_version", 1)))))
	out.RawSetString("version", StringValue(llmReplayOptionString(src, "version", llmReplayOptionString(opts, "version", "replay_http_record.v1"))))
	out.RawSetString("mode", StringValue(llmReplayOptionString(src, "mode", llmReplayModeFixture)))
	out.RawSetString("operation", StringValue(llmReplayOptionString(src, "operation", "http.request")))
	replayKey := llmReplayOptionString(src, "replay_key", llmReplayOptionString(src, "fixture_key", llmReplayOptionString(opts, "replay_key", "http:1")))
	out.RawSetString("replay_key", StringValue(replayKey))
	capability := llmReplayOptionString(src, "capability", llmReplayOptionString(opts, "capability", "generic.ai.http.replay"))
	if request := src.RawGetString("request"); request.IsTable() {
		if requestCapability := llmReplayOptionString(request.Table(), "capability", ""); requestCapability != "" {
			capability = requestCapability
		}
	}
	out.RawSetString("capability", StringValue(capability))
	llmReplayEnvelopeOfflineDefaults(out, src, opts)
	out.RawSetString("request", TableValue(llmReplayHTTPRequestValue(src.RawGetString("request").Table())))
	out.RawSetString("response", TableValue(llmReplayHTTPResponseValue(src.RawGetString("response").Table())))
	for _, field := range []string{"retry", "pagination", "rate_limit", "error", "terms"} {
		if value := src.RawGetString(field); value.IsTable() {
			out.RawSetString(field, llmCloneValue(value))
		}
	}
	out.RawSetString("redaction", TableValue(llmReplayEnvelopeRedaction(src, opts)))
	out.RawSetString("summary", TableValue(llmReplayHTTPRecordSummary(out)))
	return out
}

func llmReplayArtifactRecordValue(src, opts *Table) *Table {
	out := NewTable()
	for _, key := range src.PairsKeysSnapshot() {
		switch key.Str() {
		case "request", "response", "artifact":
			continue
		}
		out.RawSet(key, llmCloneValue(src.RawGet(key)))
	}
	out.RawSetString("__llm_replay_artifact_record", BoolValue(true))
	out.RawSetString("kind", StringValue("replay_artifact_record"))
	out.RawSetString("schema_version", IntValue(int64(llmReplayOptionInt(src, "schema_version", llmReplayOptionInt(opts, "schema_version", 1)))))
	out.RawSetString("version", StringValue(llmReplayOptionString(src, "version", llmReplayOptionString(opts, "version", "replay_artifact_record.v1"))))
	out.RawSetString("mode", StringValue(llmReplayOptionString(src, "mode", llmReplayModeFixture)))
	out.RawSetString("operation", StringValue(llmReplayOptionString(src, "operation", "artifact.replay")))
	out.RawSetString("replay_key", StringValue(llmReplayOptionString(src, "replay_key", llmReplayOptionString(src, "fixture_key", llmReplayOptionString(opts, "replay_key", "artifact:1")))))
	out.RawSetString("capability", StringValue(llmReplayOptionString(src, "capability", llmReplayOptionString(opts, "capability", "generic.ai.artifact.replay"))))
	llmReplayEnvelopeOfflineDefaults(out, src, opts)
	out.RawSetString("request", TableValue(llmReplayHTTPRequestValue(src.RawGetString("request").Table())))
	out.RawSetString("response", TableValue(llmReplayHTTPResponseValue(src.RawGetString("response").Table())))
	out.RawSetString("artifact", TableValue(llmReplayArtifactMetadataValue(src.RawGetString("artifact").Table())))
	for _, field := range []string{"redirects", "parse", "parsed", "terms"} {
		if value := src.RawGetString(field); !value.IsNil() {
			out.RawSetString(field, llmCloneValue(value))
		}
	}
	out.RawSetString("redaction", TableValue(llmReplayEnvelopeRedaction(src, opts)))
	out.RawSetString("summary", TableValue(llmReplayArtifactRecordSummary(out)))
	return out
}

func llmReplayEnvelopeOfflineDefaults(out, src, opts *Table) {
	out.RawSetString("provider_free", BoolValue(llmReplayOptionBool(src, "provider_free", llmReplayOptionBool(opts, "provider_free", true))))
	out.RawSetString("live_network", BoolValue(llmReplayOptionBool(src, "live_network", llmReplayOptionBool(opts, "live_network", false))))
	out.RawSetString("live_model", BoolValue(llmReplayOptionBool(src, "live_model", llmReplayOptionBool(opts, "live_model", false))))
	out.RawSetString("real_dependency_imports", BoolValue(llmReplayOptionBool(src, "real_dependency_imports", llmReplayOptionBool(opts, "real_dependency_imports", false))))
	out.RawSetString("credentials_required", BoolValue(llmReplayOptionBool(src, "credentials_required", llmReplayOptionBool(opts, "credentials_required", false))))
	out.RawSetString("provider_credentials_required", BoolValue(llmReplayOptionBool(src, "provider_credentials_required", llmReplayOptionBool(opts, "provider_credentials_required", false))))
	out.RawSetString("secret_values_present", BoolValue(llmReplayOptionBool(src, "secret_values_present", llmReplayOptionBool(opts, "secret_values_present", false))))
}

func llmReplayHTTPRequestValue(src *Table) *Table {
	request := NewTable()
	if src != nil {
		for _, key := range src.PairsKeysSnapshot() {
			switch key.Str() {
			case "headers":
				if headers := src.RawGet(key); headers.IsTable() {
					request.RawSet(key, TableValue(llmReplayRedactedHeaders(headers.Table())))
				}
			default:
				request.RawSet(key, llmCloneValue(src.RawGet(key)))
			}
		}
	}
	request.RawSetString("method", StringValue(llmReplayOptionString(request, "method", "GET")))
	if auth := request.RawGetString("auth"); auth.IsTable() {
		authCopy := llmCloneTable(auth.Table())
		authCopy.RawSetString("redacted", BoolValue(true))
		if !authCopy.RawGetString("secret_ref").IsNil() {
			authCopy.RawSetString("secret_ref", StringValue("<redacted>"))
		}
		request.RawSetString("auth", TableValue(authCopy))
	}
	return request
}

func llmReplayHTTPResponseValue(src *Table) *Table {
	response := NewTable()
	if src != nil {
		for _, key := range src.PairsKeysSnapshot() {
			switch key.Str() {
			case "headers":
				if headers := src.RawGet(key); headers.IsTable() {
					response.RawSet(key, TableValue(llmReplayRedactedHeaders(headers.Table())))
				}
			case "body", "content", "raw_body":
				value := src.RawGet(key)
				response.RawSetString("body_present", BoolValue(!value.IsNil()))
				if value.IsString() {
					response.RawSetString("body_bytes", IntValue(int64(len(value.Str()))))
				}
			default:
				response.RawSet(key, llmCloneValue(src.RawGet(key)))
			}
		}
	}
	return response
}

func llmReplayArtifactMetadataValue(src *Table) *Table {
	artifact := NewTable()
	if src != nil {
		for _, field := range []string{"id", "kind", "media_type", "filename", "bytes", "sha256", "digest", "replay_uri", "uri", "path", "source", "status"} {
			if value := src.RawGetString(field); !value.IsNil() {
				artifact.RawSetString(field, llmCloneValue(value))
			}
		}
	}
	return artifact
}

func llmReplayRedactedHeaders(src *Table) *Table {
	headers := NewTable()
	for _, key := range src.PairsKeysSnapshot() {
		value := src.RawGet(key)
		if key.IsString() && llmReplaySensitiveHeader(key.Str()) {
			headers.RawSet(key, StringValue("<redacted>"))
			continue
		}
		headers.RawSet(key, llmCloneValue(value))
	}
	return headers
}

func llmReplaySensitiveHeader(name string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	switch normalized {
	case "authorization", "cookie", "set-cookie", "x-api-key", "api-key", "x-auth-token", "x-access-token":
		return true
	default:
		return strings.Contains(normalized, "token") || strings.Contains(normalized, "secret")
	}
}

func llmReplayEnvelopeRedaction(src, opts *Table) *Table {
	redaction := NewTable()
	redaction.RawSetString("enabled", BoolValue(true))
	redaction.RawSetString("policy", StringValue(llmReplayOptionString(opts, "redaction_policy", "replay_envelope_metadata_only")))
	redaction.RawSetString("headers_redacted", BoolValue(true))
	redaction.RawSetString("auth_redacted", BoolValue(true))
	redaction.RawSetString("raw_body_stored", BoolValue(false))
	redaction.RawSetString("secret_values_present", BoolValue(false))
	if existing := src.RawGetString("redaction"); existing.IsTable() {
		llmCopyTable(redaction, existing.Table(), true)
	}
	return redaction
}

func llmReplayHTTPRecordSummary(record *Table) *Table {
	summary := NewTable()
	for _, field := range []string{"kind", "replay_key", "operation", "capability", "provider_free", "live_network", "real_dependency_imports"} {
		if value := record.RawGetString(field); !value.IsNil() {
			summary.RawSetString(field, llmCloneValue(value))
		}
	}
	if request := record.RawGetString("request"); request.IsTable() {
		for _, field := range []string{"method", "url"} {
			if value := request.Table().RawGetString(field); !value.IsNil() {
				summary.RawSetString(field, llmCloneValue(value))
			}
		}
	}
	if response := record.RawGetString("response"); response.IsTable() {
		if value := response.Table().RawGetString("status"); !value.IsNil() {
			summary.RawSetString("status", llmCloneValue(value))
		}
	}
	return summary
}

func llmReplayArtifactRecordSummary(record *Table) *Table {
	summary := llmReplayHTTPRecordSummary(record)
	if artifact := record.RawGetString("artifact"); artifact.IsTable() {
		for _, field := range []string{"id", "media_type", "filename", "bytes", "sha256", "replay_uri"} {
			if value := artifact.Table().RawGetString(field); !value.IsNil() {
				summary.RawSetString("artifact_"+field, llmCloneValue(value))
			}
		}
	}
	return summary
}
