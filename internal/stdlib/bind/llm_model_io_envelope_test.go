package bind

import "testing"

func TestLLMModelIOEnvelopeNormalizesMetadataOnly(t *testing.T) {
	interp := runLLMTestProgram(t, `
lookup := llm.tool("lookup", func(symbol) {
    return {symbol: symbol price: 42}, nil
}, {requires: ["market.read"]})

envelope := llm.model_io_envelope({
    model: "fast"
    provider: "local"
    replay_key: "turn:ACME:1"
    request: {
        headers: {Authorization: "Bearer secret" Accept: "application/json"}
        auth: {scheme: "bearer" secret_ref: "env:LOCAL_LLM_KEY"}
        messages: [llm.system("do not store me"), llm.user("ACME?")]
        tools: [lookup]
        temperature: 0
        max_tokens: 128
    }
    response: {
        text: "raw answer should be summarized only"
        finish_reason: "stop"
        usage: {prompt_tokens: 11 completion_tokens: 7 latency_ms: 13}
        tool_calls: [{name: "lookup" arguments: {symbol: "ACME"}}]
    }
    output_schema: {type: "object"}
    evidence_refs: [{id: "src-1" artifact_id: "filing-1"}]
}, {capability: "generic.ai.turn"})

alias := llm.modelCallEnvelope({
    model_alias: "analyst"
    messages: [llm.user("hello")]
    text: "ok"
})

missing_ok, missing_err := pcall(llm.model_io_envelope)
bad_opts_ok, bad_opts_err := pcall(llm.model_io_envelope, {}, "opts")

kind := envelope.kind
version := envelope.version
marker := envelope.__llm_model_io_envelope
capability := envelope.capability
provider_free := envelope.provider_free
live_network := envelope.live_network
live_model := envelope.live_model
secret_values_present := envelope.secret_values_present
auth_header := envelope.request.headers.Authorization
auth_secret := envelope.request.auth.secret_ref
auth_redacted := envelope.request.auth.redacted
message_count := envelope.request.messages.count
first_role := envelope.request.messages.roles[1]
raw_content_stored := envelope.request.messages.raw_content_stored
tool_count := envelope.request.tools.count
tool_name := envelope.request.tools.names[1]
text_missing := envelope.response.text == nil
text_present := envelope.response.text_present
raw_completion_stored := envelope.response.raw_completion_stored
tool_call_count := envelope.response.tool_calls.count
usage_total := envelope.usage.total_tokens
summary_messages := envelope.summary.message_count
summary_tokens := envelope.summary.total_tokens
schema_type := envelope.schema.output_schema.type
ref_id := envelope.refs.evidence_refs[1].id
redaction_policy := envelope.redaction.policy

alias_kind := alias.kind
alias_message_count := alias.request.messages.count
alias_text_present := alias.response.text_present
`, nil)

	for name, want := range map[string]Value{
		"kind":                  StringValue("model_io_envelope"),
		"version":               StringValue("model_io_envelope.v1"),
		"marker":                BoolValue(true),
		"capability":            StringValue("generic.ai.turn"),
		"provider_free":         BoolValue(true),
		"live_network":          BoolValue(false),
		"live_model":            BoolValue(false),
		"secret_values_present": BoolValue(false),
		"auth_header":           StringValue("<redacted>"),
		"auth_secret":           StringValue("<redacted>"),
		"auth_redacted":         BoolValue(true),
		"message_count":         IntValue(2),
		"first_role":            StringValue("system"),
		"raw_content_stored":    BoolValue(false),
		"tool_count":            IntValue(1),
		"tool_name":             StringValue("lookup"),
		"text_missing":          BoolValue(true),
		"text_present":          BoolValue(true),
		"raw_completion_stored": BoolValue(false),
		"tool_call_count":       IntValue(1),
		"usage_total":           IntValue(18),
		"summary_messages":      IntValue(2),
		"summary_tokens":        IntValue(18),
		"schema_type":           StringValue("object"),
		"ref_id":                StringValue("src-1"),
		"redaction_policy":      StringValue("model_io_metadata_only"),
		"alias_kind":            StringValue("model_io_envelope"),
		"alias_message_count":   IntValue(1),
		"alias_text_present":    BoolValue(true),
		"missing_ok":            BoolValue(false),
		"bad_opts_ok":           BoolValue(false),
	} {
		got := interp.GetGlobal(name)
		if !got.Equal(want) {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
	}
	if got := interp.GetGlobal("missing_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("missing_err = %v, want non-empty string", got)
	}
	if got := interp.GetGlobal("bad_opts_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("bad_opts_err = %v, want non-empty string", got)
	}
}
