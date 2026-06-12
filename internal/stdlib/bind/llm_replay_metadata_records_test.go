package bind

import "testing"

func TestLLMReplayHTTPAndArtifactRecordsNormalizeMetadataOnly(t *testing.T) {
	interp := runLLMTestProgram(t, `
http_record, http_err := llm.replay_http_record({
    replay_key: "fmp:metrics:ACME:page-1"
    request: {
        method: "GET"
        url: "https://example.invalid/metrics?page=1"
        headers: {
            Authorization: "Bearer secret-token"
            Accept: "application/json"
            x_api_key: "secret-key"
        }
        auth: {scheme: "bearer" secret_ref: "env:FMP_API_KEY"}
        capability: "network.api.fixture"
    }
    response: {
        status: 200
        headers: {content_type: "application/json" set_cookie: "sid=secret"}
        body: "[{\"x\":1}]"
        typed_as: "MetricRow[]"
    }
    retry: {attempt: 1 max_attempts: 3}
    pagination: {cursor: "page-1" next: "page-2" has_more: true}
    rate_limit: {limit: 300 remaining: 1 retry_after_seconds: 2}
    terms: {usage: "offline-fixture" live_network: false}
})

alias_http, alias_http_err := llm.replayApiRecord({
    fixture_key: "alias:http"
    request: {url: "https://example.invalid/alias"}
    response: {status: 204}
})

artifact_record, artifact_err := llm.replay_artifact_record({
    replay_key: "sec:download:ACME:10-K"
    request: {
        method: "GET"
        url: "https://example.invalid/10-k.html"
        headers: {Cookie: "sid=secret" Accept: "text/html"}
    }
    response: {
        status: 200
        body: "<html>secret fixture body</html>"
    }
    artifact: {
        id: "artifact-sec-acme"
        media_type: "text/html"
        filename: "ACME_10K.html"
        bytes: 91
        sha256: "fixture-sha"
        replay_uri: "mock://artifacts/acme/10-k.html"
        content: "raw artifact bytes"
    }
    parse: {selector: "section#item-1a" text: "Risk factors."}
    terms: {usage: "offline-fixture" live_network: false}
})

alias_artifact, alias_artifact_err := llm.replayArtifactRecord({
    fixture_key: "alias:artifact"
    artifact: {id: "artifact-alias" path: "/definitely/not/read" sha256: "provided"}
})

http_err_nil := http_err == nil
http_kind := http_record.kind
http_version := http_record.version
http_operation := http_record.operation
http_capability := http_record.capability
http_provider_free := http_record.provider_free
http_live_network := http_record.live_network
http_auth_header := http_record.request.headers.Authorization
http_api_header := http_record.request.headers.x_api_key
http_auth_secret := http_record.request.auth.secret_ref
http_auth_redacted := http_record.request.auth.redacted
http_response_cookie := http_record.response.headers.set_cookie
http_body_missing := http_record.response.body == nil
http_body_present := http_record.response.body_present
http_body_bytes := http_record.response.body_bytes
http_redaction_policy := http_record.redaction.policy
http_summary_url := http_record.summary.url
http_summary_status := http_record.summary.status
http_pagination_next := http_record.pagination.next
http_rate_remaining := http_record.rate_limit.remaining
alias_http_ok := alias_http_err == nil && alias_http.replay_key == "alias:http"

artifact_err_nil := artifact_err == nil
artifact_kind := artifact_record.kind
artifact_operation := artifact_record.operation
artifact_replay_key := artifact_record.replay_key
artifact_cookie := artifact_record.request.headers.Cookie
artifact_body_missing := artifact_record.response.body == nil
artifact_body_present := artifact_record.response.body_present
artifact_id := artifact_record.artifact.id
artifact_media_type := artifact_record.artifact.media_type
artifact_content_missing := artifact_record.artifact.content == nil
artifact_summary_id := artifact_record.summary.artifact_id
artifact_summary_uri := artifact_record.summary.artifact_replay_uri
alias_artifact_ok := alias_artifact_err == nil && alias_artifact.artifact.path == "/definitely/not/read"

missing_http_ok, missing_http_err := pcall(llm.replay_http_record)
bad_http_opts_ok, bad_http_opts_err := pcall(llm.replay_http_record, {}, "opts")
missing_artifact_ok, missing_artifact_err := pcall(llm.replay_artifact_record)
bad_artifact_opts_ok, bad_artifact_opts_err := pcall(llm.replay_artifact_record, {}, "opts")
`, nil)

	for name, want := range map[string]Value{
		"http_err_nil":          BoolValue(true),
		"http_kind":             StringValue("replay_http_record"),
		"http_version":          StringValue("replay_http_record.v1"),
		"http_operation":        StringValue("http.request"),
		"http_capability":       StringValue("network.api.fixture"),
		"http_provider_free":    BoolValue(true),
		"http_live_network":     BoolValue(false),
		"http_auth_header":      StringValue("<redacted>"),
		"http_api_header":       StringValue("<redacted>"),
		"http_auth_secret":      StringValue("<redacted>"),
		"http_auth_redacted":    BoolValue(true),
		"http_response_cookie":  StringValue("<redacted>"),
		"http_body_missing":     BoolValue(true),
		"http_body_present":     BoolValue(true),
		"http_body_bytes":       IntValue(9),
		"http_redaction_policy": StringValue("replay_envelope_metadata_only"),
		"http_summary_url":      StringValue("https://example.invalid/metrics?page=1"),
		"http_summary_status":   IntValue(200),
		"http_pagination_next":  StringValue("page-2"),
		"http_rate_remaining":   IntValue(1),
		"alias_http_ok":         BoolValue(true),

		"artifact_err_nil":         BoolValue(true),
		"artifact_kind":            StringValue("replay_artifact_record"),
		"artifact_operation":       StringValue("artifact.replay"),
		"artifact_replay_key":      StringValue("sec:download:ACME:10-K"),
		"artifact_cookie":          StringValue("<redacted>"),
		"artifact_body_missing":    BoolValue(true),
		"artifact_body_present":    BoolValue(true),
		"artifact_id":              StringValue("artifact-sec-acme"),
		"artifact_media_type":      StringValue("text/html"),
		"artifact_content_missing": BoolValue(true),
		"artifact_summary_id":      StringValue("artifact-sec-acme"),
		"artifact_summary_uri":     StringValue("mock://artifacts/acme/10-k.html"),
		"alias_artifact_ok":        BoolValue(true),

		"missing_http_ok":      BoolValue(false),
		"bad_http_opts_ok":     BoolValue(false),
		"missing_artifact_ok":  BoolValue(false),
		"bad_artifact_opts_ok": BoolValue(false),
	} {
		got := interp.GetGlobal(name)
		if !got.Equal(want) {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
	}
	for _, name := range []string{"missing_http_err", "bad_http_opts_err", "missing_artifact_err", "bad_artifact_opts_err"} {
		if got := interp.GetGlobal(name); !got.IsString() || got.Str() == "" {
			t.Fatalf("%s = %v, want error string", name, got)
		}
	}
}
