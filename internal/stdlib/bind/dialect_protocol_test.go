package bind

import (
	"strings"
	"testing"
)

func TestDialectURLParseBoundaries(t *testing.T) {
	interp := runWithLib(t, `
		parsed := dialect.eval("url", "http://user:p%40ss@[2001:db8::1]:8080/a%20b?tag=a&tag=b&empty=#frag")
		invalid, invalid_err := dialect.eval("url", "http://[::1")
		bad_path_percent, bad_path_percent_err := dialect.eval("url", "https://example.test/a%zz")
		bad_user_percent, bad_user_percent_err := dialect.eval("url", "https://u%zz@example.test/")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	parsed := interp.GetGlobal("parsed").Table()
	if got := parsed.RawGetString("scheme").Str(); got != "http" {
		t.Fatalf("scheme = %q, want http", got)
	}
	if got := parsed.RawGetString("host").Str(); got != "2001:db8::1" {
		t.Fatalf("host = %q, want 2001:db8::1", got)
	}
	if got := parsed.RawGetString("port").Str(); got != "8080" {
		t.Fatalf("port = %q, want 8080", got)
	}
	if got := parsed.RawGetString("path").Str(); got != "/a b" {
		t.Fatalf("path = %q, want /a b", got)
	}
	if got := parsed.RawGetString("fragment").Str(); got != "frag" {
		t.Fatalf("fragment = %q, want frag", got)
	}
	if got := parsed.RawGetString("user").Str(); got != "user" {
		t.Fatalf("user = %q, want user", got)
	}
	if got := parsed.RawGetString("password").Str(); got != "p@ss" {
		t.Fatalf("password = %q, want p@ss", got)
	}
	query := parsed.RawGetString("query").Table()
	if got := query.RawGetString("tag").Str(); got != "a,b" {
		t.Fatalf("query tag = %q, want a,b", got)
	}
	if got := query.RawGetString("empty").Str(); got != "" {
		t.Fatalf("query empty = %q, want empty string", got)
	}
	if !interp.GetGlobal("invalid").IsNil() {
		t.Fatalf("invalid URL returned non-nil result")
	}
	if got := interp.GetGlobal("invalid_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid URL error = %v, want non-empty string", got)
	}
	for _, name := range []string{"bad_path_percent", "bad_user_percent"} {
		if !interp.GetGlobal(name).IsNil() {
			t.Fatalf("%s returned non-nil result", name)
		}
	}
	for _, name := range []string{"bad_path_percent_err", "bad_user_percent_err"} {
		if got := interp.GetGlobal(name); !got.IsString() || got.Str() == "" {
			t.Fatalf("%s = %v, want non-empty string", name, got)
		}
	}
}

func TestDialectURLQueryBoundaryModes(t *testing.T) {
	interp := runWithLib(t, `
		to_encode := {}
		to_encode.tag = {"b", "a"}
		to_encode.empty = ""
		to_encode.page = 2
		encoded := dialect.eval("urlquery", to_encode)
		parsed := dialect.eval("urlquery", "tag=b&tag=a&empty=&space=a+b")
		form_encoded := dialect.eval("form", to_encode)
		form_parsed := dialect.eval("urlform", "tag=b&tag=a&empty=&space=a+b")
		bad_component, bad_component_err := dialect.eval("urlquery", "%zz", {mode: "unescape"})
		bad_query, bad_query_err := dialect.eval("urlquery", "ok=1&bad=%zz")
		bad_form, bad_form_err := dialect.eval("form", "a=1", {mode: "bogus"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got, want := interp.GetGlobal("encoded").Str(), "empty=&page=2&tag=b&tag=a"; got != want {
		t.Fatalf("encoded query = %q, want %q", got, want)
	}
	parsed := interp.GetGlobal("parsed").Table()
	if got := parsed.RawGetString("empty").Str(); got != "" {
		t.Fatalf("empty = %q, want empty string", got)
	}
	if got := parsed.RawGetString("space").Str(); got != "a b" {
		t.Fatalf("space = %q, want a b", got)
	}
	tags := parsed.RawGetString("tag").Table()
	if got := tags.RawGetInt(1).Str(); got != "b" {
		t.Fatalf("first tag = %q, want b", got)
	}
	if got := tags.RawGetInt(2).Str(); got != "a" {
		t.Fatalf("second tag = %q, want a", got)
	}
	if !interp.GetGlobal("bad_component").IsNil() {
		t.Fatalf("invalid component returned non-nil result")
	}
	if got := interp.GetGlobal("bad_component_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid component error = %v, want non-empty string", got)
	}
	if !interp.GetGlobal("bad_query").IsNil() {
		t.Fatalf("invalid query returned non-nil result")
	}
	if got := interp.GetGlobal("bad_query_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid query error = %v, want non-empty string", got)
	}
	if got, want := interp.GetGlobal("form_encoded").Str(), "empty=&page=2&tag=b&tag=a"; got != want {
		t.Fatalf("encoded form = %q, want %q", got, want)
	}
	if got := interp.GetGlobal("form_parsed").Table().RawGetString("tag").Table().RawGetInt(2).Str(); got != "a" {
		t.Fatalf("form parsed second tag = %q, want a", got)
	}
	if !interp.GetGlobal("bad_form").IsNil() {
		t.Fatalf("invalid form mode returned non-nil result")
	}
	if got := interp.GetGlobal("bad_form_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid form error = %v, want non-empty string", got)
	}
}

func TestDialectURLPathBoundaryModes(t *testing.T) {
	interp := runWithLib(t, `
		escaped := dialect.eval("urlpath", "a b/米")
		unescaped := dialect.eval("urlpath", "a%20b%2F%E7%B1%B3", {mode: "unescape"})
		bad, bad_err := dialect.eval("urlpath", "%zz", {mode: "decode"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got, want := interp.GetGlobal("escaped").Str(), "a%20b%2F%E7%B1%B3"; got != want {
		t.Fatalf("escaped path = %q, want %q", got, want)
	}
	if got, want := interp.GetGlobal("unescaped").Str(), "a b/米"; got != want {
		t.Fatalf("unescaped path = %q, want %q", got, want)
	}
	if !interp.GetGlobal("bad").IsNil() {
		t.Fatalf("invalid path escape returned non-nil result")
	}
	if got := interp.GetGlobal("bad_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid path escape error = %v, want non-empty string", got)
	}
}

func TestDialectURLPathTemplateMatchAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		matched := dialect.eval("urlpath", "/v1/users/alice%40example/files/docs/report%201.pdf", {template: "/v1/users/{id}/files/{*rest}", mode: "match_template"})
		no_match := dialect.eval("urlpath", "/v1/orgs/123", {template: "/v1/users/{id}", mode: "match_template"})
		encoded := dialect.eval("urlpath", {id: "alice@example", rest: "docs/report 1.pdf"}, {template: "/v1/users/{id}/files/{*rest}", mode: "encode_template"})
		bad, bad_err := dialect.eval("urlpath", "/v1/users/123", {template: "/v1/{bad-name}", mode: "match_template"})
		bad_encoded, bad_encode_err := dialect.eval("urlpath", {id: "a/b"}, {template: "/v1/users/{id}", mode: "encode_template"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	matched := interp.GetGlobal("matched").Table()
	if got := matched.RawGetString("matched").Bool(); !got {
		t.Fatalf("matched = false, want true")
	}
	params := matched.RawGetString("params").Table()
	if got := params.RawGetString("id").Str(); got != "alice@example" {
		t.Fatalf("id = %q, want alice@example", got)
	}
	if got := params.RawGetString("rest").Str(); got != "docs/report 1.pdf" {
		t.Fatalf("rest = %q, want docs/report 1.pdf", got)
	}
	if got := interp.GetGlobal("no_match").Table().RawGetString("matched").Bool(); got {
		t.Fatalf("no_match matched = true, want false")
	}
	if got, want := interp.GetGlobal("encoded").Str(), "/v1/users/alice@example/files/docs/report%201.pdf"; got != want {
		t.Fatalf("encoded path = %q, want %q", got, want)
	}
	if !interp.GetGlobal("bad").IsNil() {
		t.Fatalf("invalid template returned non-nil result")
	}
	if got := interp.GetGlobal("bad_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid template error = %v, want non-empty string", got)
	}
	if !interp.GetGlobal("bad_encoded").IsNil() {
		t.Fatalf("invalid encode returned non-nil result")
	}
	if got := interp.GetGlobal("bad_encode_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid encode error = %v, want non-empty string", got)
	}
}

func TestDialectHTMLValueEncoder(t *testing.T) {
	interp := runWithLib(t, `
		escaped := html`+"`"+`<b>Ada & Bob</b>`+"`"+`
		bad_attrs := {}
		bad_attrs["bad attr"] = "x"
		page := html {
			tag: "main",
			attrs: {class: "card", hidden: false, data_id: "x&1"},
			children: {
				{tag: "h1", text: "Release <ok>"},
				{tag: "p", children: {"Status: ", {tag: "strong", text: "green"}}},
				{tag: "input", attrs: {disabled: true, value: "ship"}},
				{raw: "<!-- generated -->"},
			},
		}
		bad_tag, bad_tag_err := dialect.eval("html", {tag: "script src"})
		bad_attr, bad_attr_err := dialect.eval("html", {tag: "div", attrs: bad_attrs})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got, want := interp.GetGlobal("escaped").Str(), "&lt;b&gt;Ada &amp; Bob&lt;/b&gt;"; got != want {
		t.Fatalf("html escaped = %q, want %q", got, want)
	}
	wantPage := `<main class="card" data_id="x&amp;1"><h1>Release &lt;ok&gt;</h1><p>Status: <strong>green</strong></p><input disabled value="ship"><!-- generated --></main>`
	if got := interp.GetGlobal("page").Str(); got != wantPage {
		t.Fatalf("html page = %q, want %q", got, wantPage)
	}
	if !interp.GetGlobal("bad_tag").IsNil() || !strings.Contains(interp.GetGlobal("bad_tag_err").Str(), "invalid tag") {
		t.Fatalf("bad tag = %v err %v", interp.GetGlobal("bad_tag"), interp.GetGlobal("bad_tag_err"))
	}
	if !interp.GetGlobal("bad_attr").IsNil() || !strings.Contains(interp.GetGlobal("bad_attr_err").Str(), "invalid attribute") {
		t.Fatalf("bad attr = %v err %v", interp.GetGlobal("bad_attr"), interp.GetGlobal("bad_attr_err"))
	}
}

func TestDialectSSEParseAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		events := sse`+"`"+`: keepalive
id: 1
event: token
data: hello
data: world
retry: 2500

`+"`"+`
		encoded := dialect.eval("sse", {{event: "done", id: "2", data: "ok"}}, {mode: "encode"})
		roundtrip := dialect.eval("sse", encoded)
		boundary_events := dialect.eval("sse", ": heartbeat\nevent: zero\nretry: 0\ndata:\ndata\n\n")
		boundary_encoded := dialect.eval("sse", {{event: "zero", retry: 0, data: ""}}, {mode: "encode"})
		bad, bad_err := dialect.eval("sse", "retry: soon\n\n")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	events := interp.GetGlobal("events").Table()
	first := events.RawGetInt(1).Table()
	if got := first.RawGetString("id").Str(); got != "1" {
		t.Fatalf("event id = %q, want 1", got)
	}
	if got := first.RawGetString("event").Str(); got != "token" {
		t.Fatalf("event type = %q, want token", got)
	}
	if got := first.RawGetString("data").Str(); got != "hello\nworld" {
		t.Fatalf("event data = %q, want multiline data", got)
	}
	if got := first.RawGetString("retry").Int(); got != 2500 {
		t.Fatalf("event retry = %d, want 2500", got)
	}
	roundtrip := interp.GetGlobal("roundtrip").Table().RawGetInt(1).Table()
	if got := roundtrip.RawGetString("event").Str(); got != "done" {
		t.Fatalf("roundtrip event = %q, want done", got)
	}
	boundary := interp.GetGlobal("boundary_events").Table().RawGetInt(1).Table()
	if got := boundary.RawGetString("event").Str(); got != "zero" {
		t.Fatalf("boundary event = %q, want zero", got)
	}
	if got := boundary.RawGetString("data").Str(); got != "\n" {
		t.Fatalf("boundary data = %q, want two empty data lines joined", got)
	}
	if !boundary.RawGetString("retry").IsNil() {
		t.Fatalf("retry:0 materialized as %v, want nil field", boundary.RawGetString("retry"))
	}
	if got := interp.GetGlobal("boundary_encoded").Str(); got != "event: zero\ndata: \n\n" {
		t.Fatalf("boundary encoded = %q, want zero event with empty data", got)
	}
	if !interp.GetGlobal("bad").IsNil() || !interp.GetGlobal("bad_err").IsString() {
		t.Fatalf("bad SSE = %v err %v, want nil error string", interp.GetGlobal("bad"), interp.GetGlobal("bad_err"))
	}
}

func TestDialectMultipartParseAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		body := "--fixture\r\nContent-Disposition: form-data; name=\"meta\"\r\nContent-Type: application/json\r\n\r\n{\"ok\":true}\r\n--fixture\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.txt\"\r\nContent-Type: text/plain\r\n\r\nhello\r\n--fixture--\r\n"
		parts := dialect.eval("multipart", body, {boundary: "fixture"})
		parts_by_content_type := dialect.eval("multipart", body, {content_type: "multipart/form-data; boundary=fixture"})
		encoded := dialect.eval("multipart", {
			{name: "meta", content_type: "application/json", body: "{\"ok\":true}"},
			{name: "file", filename: "a.txt", content_type: "text/plain", body: "hello"},
		}, {mode: "encode", boundary: "fixture"})
		roundtrip := dialect.eval("multipart", encoded, {boundary: "fixture"})
		single_headers := {}
		single_headers["X-Tag"] = {"a", "b"}
		single_encoded, single_encode_err := dialect.eval("multipart", {name: "note", value: "hello", contentType: "text/plain", headers: single_headers}, {mode: "encode", boundary: "fixture"})
		single_roundtrip := dialect.eval("multipart", single_encoded, {boundary: "fixture"})
		missing_boundary, missing_boundary_err := dialect.eval("multipart", body)
	`, "dialect", BuildDialect(HostOptions{}, nil))

	parts := interp.GetGlobal("parts").Table()
	if got := parts.Length(); got != 2 {
		t.Fatalf("parts length = %d, want 2", got)
	}
	first := parts.RawGetInt(1).Table()
	if got := first.RawGetString("name").Str(); got != "meta" {
		t.Fatalf("first name = %q, want meta", got)
	}
	if got := first.RawGetString("content_type").Str(); got != "application/json" {
		t.Fatalf("first content_type = %q, want application/json", got)
	}
	if got := first.RawGetString("body").Str(); got != `{"ok":true}` {
		t.Fatalf("first body = %q, want json", got)
	}
	second := parts.RawGetInt(2).Table()
	if got := second.RawGetString("filename").Str(); got != "a.txt" {
		t.Fatalf("second filename = %q, want a.txt", got)
	}
	if got := second.RawGetString("headers").Table().RawGetString("Content-Type").Str(); got != "text/plain" {
		t.Fatalf("second Content-Type header = %q, want text/plain", got)
	}
	if got := interp.GetGlobal("parts_by_content_type").Table().RawGetInt(1).Table().RawGetString("name").Str(); got != "meta" {
		t.Fatalf("content_type boundary parse first name = %q, want meta", got)
	}
	roundtrip := interp.GetGlobal("roundtrip").Table()
	if got := roundtrip.RawGetInt(2).Table().RawGetString("body").Str(); got != "hello" {
		t.Fatalf("roundtrip body = %q, want hello", got)
	}
	if !interp.GetGlobal("single_encode_err").IsNil() {
		t.Fatalf("single multipart encode err = %v, want nil", interp.GetGlobal("single_encode_err"))
	}
	single := interp.GetGlobal("single_roundtrip").Table().RawGetInt(1).Table()
	if got := single.RawGetString("name").Str(); got != "note" {
		t.Fatalf("single name = %q, want note", got)
	}
	if got := single.RawGetString("content_type").Str(); got != "text/plain" {
		t.Fatalf("single content_type = %q, want text/plain", got)
	}
	if got := single.RawGetString("contentType").Str(); got != "text/plain" {
		t.Fatalf("single contentType = %q, want text/plain", got)
	}
	headers := single.RawGetString("headers").Table()
	if got := headers.RawGetString("X-Tag").Table().RawGetInt(2).Str(); got != "b" {
		t.Fatalf("single X-Tag second = %q, want b", got)
	}
	if !interp.GetGlobal("missing_boundary").IsNil() || !interp.GetGlobal("missing_boundary_err").IsString() {
		t.Fatalf("missing boundary = %v err %v, want nil error string", interp.GetGlobal("missing_boundary"), interp.GetGlobal("missing_boundary_err"))
	}
}

func TestDialectJWTUnverifiedDecode(t *testing.T) {
	interp := runWithLib(t, `
		token := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJ1c2VyLTQyIiwic2NvcGUiOiJyZWFkIHdyaXRlIiwiZXhwIjoxODkzNDU2MDAwfQ.signature"
		structured_token := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJ1c2VyLTQyIiwiYWRtaW4iOnRydWUsInJvbGVzIjpbInJlYWQiLCJ3cml0ZSJdLCJwcm9maWxlIjp7InRlYW0iOiJpbmZyYSIsImxldmVsIjozfSwic2NvcmUiOjEyLjV9.signature"
		decoded := dialect.eval("jwt", token)
		structured := dialect.eval("jwt", structured_token)
		decoded_explicit := dialect.eval("jwt", token, {mode: "unverified"})
		bad, bad_err := dialect.eval("jwt", "not-a-token")
		bad_mode, bad_mode_err := dialect.eval("jwt", token, {mode: "verify"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	decoded := interp.GetGlobal("decoded").Table()
	header := decoded.RawGetString("header").Table()
	payload := decoded.RawGetString("payload").Table()
	if got := header.RawGetString("alg").Str(); got != "none" {
		t.Fatalf("header alg = %q, want none", got)
	}
	if got := payload.RawGetString("sub").Str(); got != "user-42" {
		t.Fatalf("payload sub = %q, want user-42", got)
	}
	if got := payload.RawGetString("exp").Int(); got != 1893456000 {
		t.Fatalf("payload exp = %d, want 1893456000", got)
	}
	if got := decoded.RawGetString("verified").Bool(); got {
		t.Fatalf("verified = true, want false for unverified decode")
	}
	if got := decoded.RawGetString("segments").Table().RawGetString("signature").Str(); got != "signature" {
		t.Fatalf("signature segment = %q, want signature", got)
	}
	structured := interp.GetGlobal("structured").Table()
	if got := structured.RawGetString("header_json").Str(); got != `{"alg":"none","typ":"JWT"}` {
		t.Fatalf("structured header_json = %q", got)
	}
	if got := structured.RawGetString("payload_json").Str(); got != `{"sub":"user-42","admin":true,"roles":["read","write"],"profile":{"team":"infra","level":3},"score":12.5}` {
		t.Fatalf("structured payload_json = %q", got)
	}
	structuredPayload := structured.RawGetString("payload").Table()
	if !structuredPayload.RawGetString("admin").Bool() {
		t.Fatalf("structured admin = false, want true")
	}
	if got := structuredPayload.RawGetString("roles").Table().RawGetInt(2).Str(); got != "write" {
		t.Fatalf("structured role[2] = %q, want write", got)
	}
	if got := structuredPayload.RawGetString("profile").Table().RawGetString("team").Str(); got != "infra" {
		t.Fatalf("structured profile.team = %q, want infra", got)
	}
	if got := structured.RawGetString("segments").Table().RawGetString("header").Str(); got == "" {
		t.Fatalf("structured header segment is empty")
	}
	if got := interp.GetGlobal("decoded_explicit").Table().RawGetString("payload").Table().RawGetString("scope").Str(); got != "read write" {
		t.Fatalf("explicit payload scope = %q, want read write", got)
	}
	if !interp.GetGlobal("bad").IsNil() || !interp.GetGlobal("bad_err").IsString() {
		t.Fatalf("bad jwt = %v err %v, want nil error string", interp.GetGlobal("bad"), interp.GetGlobal("bad_err"))
	}
	if !interp.GetGlobal("bad_mode").IsNil() || !interp.GetGlobal("bad_mode_err").IsString() {
		t.Fatalf("bad mode = %v err %v, want nil error string", interp.GetGlobal("bad_mode"), interp.GetGlobal("bad_mode_err"))
	}
}

func TestDialectProtocolUnknownModesAreReported(t *testing.T) {
	interp := runWithLib(t, `
		html_bad, html_bad_err := dialect.eval("html_escape", "<x>", {mode: "bogus"})
		urlquery_bad, urlquery_bad_err := dialect.eval("urlquery", "a=1", {mode: "bogus"})
		form_bad, form_bad_err := dialect.eval("form", "a=1", {mode: "bogus"})
		mime_bad, mime_bad_err := dialect.eval("mime", "text/plain", {mode: "bogus"})
		headers_bad, headers_bad_err := dialect.eval("headers", "X-Test: ok\r\n", {mode: "bogus"})
		cookie_bad, cookie_bad_err := dialect.eval("cookie", "a=1", {mode: "bogus"})
		httpmsg_bad, httpmsg_bad_err := dialect.eval("httpmsg", "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n", {mode: "bogus"})
		sse_bad, sse_bad_err := dialect.eval("sse", "data: hello\n\n", {mode: "bogus"})
		multipart_bad, multipart_bad_err := dialect.eval("multipart", "--fixture--\r\n", {mode: "bogus", boundary: "fixture"})
		jwt_bad, jwt_bad_err := dialect.eval("jwt", "a.b.c", {mode: "verify"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	assertDialectModeError(t, interp.GetGlobal("html_bad"), interp.GetGlobal("html_bad_err"), "html_escape dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("urlquery_bad"), interp.GetGlobal("urlquery_bad_err"), "urlquery dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("form_bad"), interp.GetGlobal("form_bad_err"), "form dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("mime_bad"), interp.GetGlobal("mime_bad_err"), "mime dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("headers_bad"), interp.GetGlobal("headers_bad_err"), "headers dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("cookie_bad"), interp.GetGlobal("cookie_bad_err"), "cookie dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("httpmsg_bad"), interp.GetGlobal("httpmsg_bad_err"), "httpmsg dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("sse_bad"), interp.GetGlobal("sse_bad_err"), "sse dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("multipart_bad"), interp.GetGlobal("multipart_bad_err"), "multipart dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("jwt_bad"), interp.GetGlobal("jwt_bad_err"), "jwt dialect: unknown mode")
}

func TestDialectProtocolModeAliasesKeepDirectionInference(t *testing.T) {
	interp := runWithLib(t, `
		html_escaped := dialect.eval("html_escape", "<x>", {mode: "encode"})
		html_unescaped := dialect.eval("html_escape", "&lt;x&gt;", {mode: "decode"})
		query_encoded := dialect.eval("urlquery", {q: "a b"}, {mode: "format"})
		query_decoded := dialect.eval("urlquery", "q=a+b", {mode: "decode"})
		query_component := dialect.eval("urlquery", "a b", {mode: "escape"})
		form_encoded := dialect.eval("form", {q: "a b"}, {mode: "format"})
		form_decoded := dialect.eval("urlform", "q=a+b", {mode: "decode"})
		mime_encoded := dialect.eval("mime", {type: "text/plain", params: {charset: "utf-8"}}, {mode: "format"})
		mime_parsed := dialect.eval("mime", "text/plain; charset=utf-8", {mode: "parse"})
		headers_encoded := dialect.eval("headers", {x_test: "ok"}, {mode: "format"})
		headers_parsed := dialect.eval("headers", "X-Test: ok\r\n", {mode: "decode"})
		cookie_encoded := dialect.eval("cookie", {sid: "abc"}, {mode: "format"})
		cookie_parsed := dialect.eval("cookie", "sid=abc", {mode: "parse"})
		http_encoded := dialect.eval("httpmsg", {method: "GET", target: "/", version: "HTTP/1.1", headers: {Host: "example.com"}}, {mode: "format"})
		http_parsed := dialect.eval("httpmsg", "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n", {mode: "decode"})
		sse_encoded := dialect.eval("sse", {{event: "token", data: "hi"}}, {mode: "format"})
		sse_parsed := dialect.eval("sse", "event: token\ndata: hi\n\n", {mode: "parse"})
		mp_encoded := dialect.eval("multipart", {{name: "field", body: "value"}}, {mode: "format", boundary: "fixture"})
		mp_parsed := dialect.eval("multipart", mp_encoded, {mode: "decode", boundary: "fixture"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got := interp.GetGlobal("html_escaped").Str(); got != "&lt;x&gt;" {
		t.Fatalf("html escaped = %q, want &lt;x&gt;", got)
	}
	if got := interp.GetGlobal("html_unescaped").Str(); got != "<x>" {
		t.Fatalf("html unescaped = %q, want <x>", got)
	}
	if got := interp.GetGlobal("query_encoded").Str(); got != "q=a+b" {
		t.Fatalf("query encoded = %q, want q=a+b", got)
	}
	if got := interp.GetGlobal("query_decoded").Table().RawGetString("q").Str(); got != "a b" {
		t.Fatalf("query decoded q = %q, want a b", got)
	}
	if got := interp.GetGlobal("query_component").Str(); got != "a+b" {
		t.Fatalf("query component = %q, want a+b", got)
	}
	if got := interp.GetGlobal("form_encoded").Str(); got != "q=a+b" {
		t.Fatalf("form encoded = %q, want q=a+b", got)
	}
	if got := interp.GetGlobal("form_decoded").Table().RawGetString("q").Str(); got != "a b" {
		t.Fatalf("form decoded q = %q, want a b", got)
	}
	if got := interp.GetGlobal("mime_encoded").Str(); got != "text/plain; charset=utf-8" {
		t.Fatalf("mime encoded = %q, want formatted media type", got)
	}
	if got := interp.GetGlobal("mime_parsed").Table().RawGetString("params").Table().RawGetString("charset").Str(); got != "utf-8" {
		t.Fatalf("mime charset = %q, want utf-8", got)
	}
	if got := interp.GetGlobal("headers_encoded").Str(); got != "X_test: ok\r\n" {
		t.Fatalf("headers encoded = %q, want header line", got)
	}
	if got := interp.GetGlobal("headers_parsed").Table().RawGetString("X-Test").Str(); got != "ok" {
		t.Fatalf("headers parsed = %q, want ok", got)
	}
	if got := interp.GetGlobal("cookie_encoded").Str(); got != "sid=abc" {
		t.Fatalf("cookie encoded = %q, want sid=abc", got)
	}
	if got := interp.GetGlobal("cookie_parsed").Table().RawGetString("sid").Str(); got != "abc" {
		t.Fatalf("cookie parsed = %q, want abc", got)
	}
	if got := interp.GetGlobal("http_encoded").Str(); !strings.Contains(got, "GET / HTTP/1.1") {
		t.Fatalf("http encoded = %q, want request line", got)
	}
	if got := interp.GetGlobal("http_parsed").Table().RawGetString("method").Str(); got != "GET" {
		t.Fatalf("http parsed method = %q, want GET", got)
	}
	if got := interp.GetGlobal("sse_encoded").Str(); !strings.Contains(got, "event: token") {
		t.Fatalf("sse encoded = %q, want event", got)
	}
	if got := interp.GetGlobal("sse_parsed").Table().RawGetInt(1).Table().RawGetString("data").Str(); got != "hi" {
		t.Fatalf("sse parsed data = %q, want hi", got)
	}
	if got := interp.GetGlobal("mp_parsed").Table().RawGetInt(1).Table().RawGetString("body").Str(); got != "value" {
		t.Fatalf("multipart parsed body = %q, want value", got)
	}
}

func TestDialectMIMEBoundaryParseAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		parsed := dialect.eval("mime", "Text/HTML; Charset=UTF-8; boundary=\"abc def\"")
		encoded := dialect.eval("mime", {type: "application/example", params: {title: "a b", version: 2}}, {mode: "encode"})
		invalid, invalid_err := dialect.eval("mime", "text/plain; bad")
		bad_encoded, bad_encode_err := dialect.eval("mime", {type: "bad type", params: {ok: "1"}}, {mode: "encode"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	parsed := interp.GetGlobal("parsed").Table()
	if got := parsed.RawGetString("type").Str(); got != "text/html" {
		t.Fatalf("media type = %q, want text/html", got)
	}
	params := parsed.RawGetString("params").Table()
	if got := params.RawGetString("charset").Str(); got != "UTF-8" {
		t.Fatalf("charset = %q, want UTF-8", got)
	}
	if got := params.RawGetString("boundary").Str(); got != "abc def" {
		t.Fatalf("boundary = %q, want abc def", got)
	}
	if got, want := interp.GetGlobal("encoded").Str(), `application/example; title="a b"; version=2`; got != want {
		t.Fatalf("encoded mime = %q, want %q", got, want)
	}
	if !interp.GetGlobal("invalid").IsNil() {
		t.Fatalf("invalid MIME returned non-nil result")
	}
	if got := interp.GetGlobal("invalid_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid MIME error = %v, want non-empty string", got)
	}
	if !interp.GetGlobal("bad_encoded").IsNil() {
		t.Fatalf("invalid MIME encode returned non-nil result")
	}
	if got := interp.GetGlobal("bad_encode_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid MIME encode error = %v, want non-empty string", got)
	}
}

func TestDialectMailAddressParseListAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		single := mailaddr`+"`"+`Ada Lovelace <ada@example.org>`+"`"+`
		alias := dialect.eval("emailaddr", "ops@example.net")
		explicit_list := dialect.eval("mailaddr", "Ada <ada@example.org>, Bob <bob@example.net>", {list: true})
		auto_list := dialect.eval("mailaddr", "Ada <ada@example.org>, Bob <bob@example.net>")
		encoded := dialect.eval("mailaddr", {name: "Ada Lovelace", address: "ada@example.org"}, {mode: "format"})
		encoded_list := dialect.eval("emailaddr", {
			{name: "Ada Lovelace", address: "ada@example.org"},
			{address: "bob@example.net"},
		}, {mode: "encode"})
		roundtrip := dialect.eval("mailaddr", encoded_list, {mode: "parse"})
		bad_parse, bad_parse_err := dialect.eval("mailaddr", "not an address")
		bad_encode, bad_encode_err := dialect.eval("mailaddr", {name: "Missing"}, {mode: "encode"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	single := interp.GetGlobal("single").Table()
	if got := single.RawGetString("name").Str(); got != "Ada Lovelace" {
		t.Fatalf("single name = %q, want Ada Lovelace", got)
	}
	if got := single.RawGetString("address").Str(); got != "ada@example.org" {
		t.Fatalf("single address = %q, want ada@example.org", got)
	}
	if got := single.RawGetString("local").Str(); got != "ada" {
		t.Fatalf("single local = %q, want ada", got)
	}
	if got := single.RawGetString("domain").Str(); got != "example.org" {
		t.Fatalf("single domain = %q, want example.org", got)
	}
	if got := single.RawGetString("raw").Str(); got != "Ada Lovelace <ada@example.org>" {
		t.Fatalf("single raw = %q, want original input", got)
	}
	if got := interp.GetGlobal("alias").Table().RawGetString("address").Str(); got != "ops@example.net" {
		t.Fatalf("emailaddr alias address = %q, want ops@example.net", got)
	}
	if got := interp.GetGlobal("explicit_list").Table().Length(); got != 2 {
		t.Fatalf("explicit list length = %d, want 2", got)
	}
	autoList := interp.GetGlobal("auto_list").Table()
	if got := autoList.RawGetInt(2).Table().RawGetString("local").Str(); got != "bob" {
		t.Fatalf("auto list second local = %q, want bob", got)
	}
	if got := interp.GetGlobal("encoded").Str(); got != `"Ada Lovelace" <ada@example.org>` {
		t.Fatalf("encoded = %q, want quoted display name", got)
	}
	if got := interp.GetGlobal("encoded_list").Str(); got != `"Ada Lovelace" <ada@example.org>, <bob@example.net>` {
		t.Fatalf("encoded list = %q, want formatted list", got)
	}
	roundtrip := interp.GetGlobal("roundtrip").Table()
	if got := roundtrip.RawGetInt(1).Table().RawGetString("name").Str(); got != "Ada Lovelace" {
		t.Fatalf("roundtrip first name = %q, want Ada Lovelace", got)
	}
	if got := roundtrip.RawGetInt(2).Table().RawGetString("address").Str(); got != "bob@example.net" {
		t.Fatalf("roundtrip second address = %q, want bob@example.net", got)
	}
	if !interp.GetGlobal("bad_parse").IsNil() || !strings.Contains(interp.GetGlobal("bad_parse_err").Str(), "mailaddr dialect:") {
		t.Fatalf("bad parse = %v err %v", interp.GetGlobal("bad_parse"), interp.GetGlobal("bad_parse_err"))
	}
	if !interp.GetGlobal("bad_encode").IsNil() || !strings.Contains(interp.GetGlobal("bad_encode_err").Str(), "address field required") {
		t.Fatalf("bad encode = %v err %v", interp.GetGlobal("bad_encode"), interp.GetGlobal("bad_encode_err"))
	}
}

func TestDialectHeadersParseAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		parsed := dialect.eval("headers", "content-type: text/plain\r\nset-cookie: a=1\r\nset-cookie: b=2\r\nx-trace:  abc  \r\n")
		no_trailing := dialect.eval("headers", "x-one: 1")
		content_type := parsed["Content-Type"]
		one := no_trailing["X-One"]
		cookies := parsed["Set-Cookie"]
		first_cookie := cookies[1]
		second_cookie := cookies[2]
		trace := parsed["X-Trace"]
		to_encode := {}
		to_encode["x-trace"] = "abc"
		to_encode["set-cookie"] = {"a=1", "b=2"}
		to_encode["content-type"] = "text/plain"
		encoded := dialect.eval("headers", to_encode, {mode: "encode"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got := interp.GetGlobal("content_type").Str(); got != "text/plain" {
		t.Fatalf("content type = %q, want text/plain", got)
	}
	if got := interp.GetGlobal("one").Str(); got != "1" {
		t.Fatalf("one = %q, want 1", got)
	}
	if got := interp.GetGlobal("first_cookie").Str(); got != "a=1" {
		t.Fatalf("first cookie = %q, want a=1", got)
	}
	if got := interp.GetGlobal("second_cookie").Str(); got != "b=2" {
		t.Fatalf("second cookie = %q, want b=2", got)
	}
	if got := interp.GetGlobal("trace").Str(); got != "abc" {
		t.Fatalf("trace = %q, want abc", got)
	}
	want := "Content-Type: text/plain\r\nSet-Cookie: a=1\r\nSet-Cookie: b=2\r\nX-Trace: abc\r\n"
	if got := interp.GetGlobal("encoded").Str(); got != want {
		t.Fatalf("encoded headers = %q, want %q", got, want)
	}
}

func TestDialectHeadersBoundaryParsing(t *testing.T) {
	interp := runWithLib(t, `
		folded := dialect.eval("headers", "x-note: first\r\n second\r\nx-empty:\r\n")
		empty := dialect.eval("headers", "")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	folded := interp.GetGlobal("folded").Table()
	if got := folded.RawGetString("X-Note").Str(); got != "first second" {
		t.Fatalf("folded header = %q, want first second", got)
	}
	if got := folded.RawGetString("X-Empty").Str(); got != "" {
		t.Fatalf("empty header = %q, want empty string", got)
	}
	if got := interp.GetGlobal("empty"); !got.IsTable() || got.Table().Length() != 0 {
		t.Fatalf("empty headers = %v, want empty table", got)
	}
}

func TestDialectHeadersInvalidInputReturnsError(t *testing.T) {
	interp := runWithLib(t, `
		parsed, parse_err := dialect.eval("http_headers", "ok: yes\r\nnot a header\r\n")
		encoded_scalar, encode_scalar_err := dialect.eval("headers", "x-ok: yes", {mode: "encode"})
		bad := {}
		bad["bad name"] = "x"
		encoded, encode_err := dialect.eval("headers", bad, {mode: "encode"})
		bad_value := {}
		bad_value["x-ok"] = "first\r\nsecond: no"
		encoded_value, encode_value_err := dialect.eval("headers", bad_value, {mode: "encode"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if !interp.GetGlobal("parsed").IsNil() {
		t.Fatalf("invalid parse returned non-nil result")
	}
	if got := interp.GetGlobal("parse_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid parse error = %v, want non-empty string", got)
	}
	if !interp.GetGlobal("encoded_scalar").IsNil() {
		t.Fatalf("invalid scalar encode returned non-nil result")
	}
	if got := interp.GetGlobal("encode_scalar_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid scalar encode error = %v, want non-empty string", got)
	}
	if !interp.GetGlobal("encoded").IsNil() {
		t.Fatalf("invalid encode returned non-nil result")
	}
	if got := interp.GetGlobal("encode_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid encode error = %v, want non-empty string", got)
	}
	if !interp.GetGlobal("encoded_value").IsNil() {
		t.Fatalf("invalid encode value returned non-nil result")
	}
	if got := interp.GetGlobal("encode_value_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid encode value error = %v, want non-empty string", got)
	}
}

func TestDialectCookieParseAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		parsed := dialect.eval("cookie", "session=abc123; theme=light; tag=a; tag=b")
		session := parsed.session
		theme := parsed.theme
		first_tag := parsed.tag[1]
		second_tag := parsed.tag[2]

		to_encode := {}
		to_encode.theme = "light"
		to_encode.session = "abc123"
		to_encode.tag = {"a", "b"}
		encoded := dialect.eval("cookies", to_encode, {mode: "encode"})
		empty := dialect.eval("cookie", "  ")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got := interp.GetGlobal("session").Str(); got != "abc123" {
		t.Fatalf("session = %q, want abc123", got)
	}
	if got := interp.GetGlobal("theme").Str(); got != "light" {
		t.Fatalf("theme = %q, want light", got)
	}
	if got := interp.GetGlobal("first_tag").Str(); got != "a" {
		t.Fatalf("first tag = %q, want a", got)
	}
	if got := interp.GetGlobal("second_tag").Str(); got != "b" {
		t.Fatalf("second tag = %q, want b", got)
	}
	want := "session=abc123; tag=a; tag=b; theme=light"
	if got := interp.GetGlobal("encoded").Str(); got != want {
		t.Fatalf("encoded cookies = %q, want %q", got, want)
	}
	if got := interp.GetGlobal("empty"); !got.IsTable() || got.Table().Length() != 0 {
		t.Fatalf("empty cookies = %v, want empty table", got)
	}
}

func TestDialectHTTPMessageParseRequestResponseAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		request := httpmsg`+"`"+`POST /v1/events?debug=true HTTP/1.1
host: api.example.test
content-type: application/json
x-trace-id: req-42
set-cookie: a=1
set-cookie: b=2

{"ok":true}`+"`"+`
		response := dialect.eval("httpmsg", "HTTP/1.1 202 Accepted\r\ncontent-type: application/json\r\nx-trace-id: req-42\r\n\r\n{\"queued\":true}")

		encoded_request := dialect.eval("httpmsg", {
			method: "PUT",
			target: "/v1/events/42",
			headers: {
				host: "api.example.test",
				["content-type"]: "application/json",
				["x-trace-id"]: "req-43",
			},
			body: "{\"done\":true}",
		}, {mode: "encode"})
		encoded_response := dialect.eval("httpmsg", {
			type: "response",
			status: 404,
			reason: "Not Found",
			headers: {["content-length"]: 0},
		}, {mode: "encode"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	request := interp.GetGlobal("request").Table()
	if got := request.RawGetString("type").Str(); got != "request" {
		t.Fatalf("request type = %q, want request", got)
	}
	if got := request.RawGetString("method").Str(); got != "POST" {
		t.Fatalf("request method = %q, want POST", got)
	}
	if got := request.RawGetString("target").Str(); got != "/v1/events?debug=true" {
		t.Fatalf("request target = %q, want /v1/events?debug=true", got)
	}
	headers := request.RawGetString("headers").Table()
	if got := headers.RawGetString("Host").Str(); got != "api.example.test" {
		t.Fatalf("host = %q, want api.example.test", got)
	}
	cookies := headers.RawGetString("Set-Cookie").Table()
	if got := cookies.RawGetInt(2).Str(); got != "b=2" {
		t.Fatalf("second set-cookie = %q, want b=2", got)
	}
	if got := request.RawGetString("body").Str(); got != `{"ok":true}` {
		t.Fatalf("request body = %q, want JSON body", got)
	}

	response := interp.GetGlobal("response").Table()
	if got := response.RawGetString("type").Str(); got != "response" {
		t.Fatalf("response type = %q, want response", got)
	}
	if got := response.RawGetString("status").Int(); got != 202 {
		t.Fatalf("response status = %d, want 202", got)
	}
	if got := response.RawGetString("reason").Str(); got != "Accepted" {
		t.Fatalf("response reason = %q, want Accepted", got)
	}

	wantRequest := "PUT /v1/events/42 HTTP/1.1\r\nContent-Type: application/json\r\nHost: api.example.test\r\nX-Trace-Id: req-43\r\n\r\n{\"done\":true}"
	if got := interp.GetGlobal("encoded_request").Str(); got != wantRequest {
		t.Fatalf("encoded request = %q, want %q", got, wantRequest)
	}
	wantResponse := "HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\n\r\n"
	if got := interp.GetGlobal("encoded_response").Str(); got != wantResponse {
		t.Fatalf("encoded response = %q, want %q", got, wantResponse)
	}
}

func TestDialectHTTPMessageInvalidInputReturnsError(t *testing.T) {
	interp := runWithLib(t, `
		bad_empty, bad_empty_err := dialect.eval("httpmsg", "")
		bad_start, bad_start_err := dialect.eval("httpmsg", "not http\r\nx: y\r\n\r\n")
		bad_header, bad_header_err := dialect.eval("httpmsg", "GET / HTTP/1.1\r\nbad header\r\n\r\n")
		bad_status, bad_status_err := dialect.eval("httpmsg", {type: "response", status: 99}, {mode: "encode"})
		bad_encode, bad_encode_err := dialect.eval("httpmsg", {method: "BAD METHOD", target: "/"}, {mode: "encode"})
		bad_header_encode, bad_header_encode_err := dialect.eval("httpmsg", {method: "GET", headers: {["bad name"]: "x"}}, {mode: "encode"})
		bad_header_value, bad_header_value_err := dialect.eval("httpmsg", {method: "GET", headers: {["x-ok"]: "first\r\nsecond: no"}}, {mode: "format"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	for _, name := range []string{"bad_empty", "bad_start", "bad_header", "bad_status", "bad_encode", "bad_header_encode", "bad_header_value"} {
		if !interp.GetGlobal(name).IsNil() {
			t.Fatalf("%s returned non-nil result", name)
		}
	}
	for _, name := range []string{"bad_empty_err", "bad_start_err", "bad_header_err", "bad_status_err", "bad_encode_err", "bad_header_encode_err", "bad_header_value_err"} {
		if got := interp.GetGlobal(name); !got.IsString() || got.Str() == "" {
			t.Fatalf("%s = %v, want non-empty string", name, got)
		}
	}
}

func TestDialectCookieBoundaryValues(t *testing.T) {
	interp := runWithLib(t, `
		parsed := dialect.eval("cookie", "empty=; token=a=b; spaced = value")
		encoded := dialect.eval("cookie", {empty: "", token: "a=b"}, {mode: "encode"})
		bad_quoted, bad_quoted_err := dialect.eval("cookie", "bad=\"quoted\"")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	parsed := interp.GetGlobal("parsed").Table()
	if got := parsed.RawGetString("empty").Str(); got != "" {
		t.Fatalf("empty cookie = %q, want empty string", got)
	}
	if got := parsed.RawGetString("token").Str(); got != "a=b" {
		t.Fatalf("token cookie = %q, want a=b", got)
	}
	if got := parsed.RawGetString("spaced").Str(); got != "value" {
		t.Fatalf("spaced cookie = %q, want value", got)
	}
	if got, want := interp.GetGlobal("encoded").Str(), "empty=; token=a=b"; got != want {
		t.Fatalf("encoded cookies = %q, want %q", got, want)
	}
	if !interp.GetGlobal("bad_quoted").IsNil() {
		t.Fatalf("quoted cookie returned non-nil result")
	}
	if got := interp.GetGlobal("bad_quoted_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("quoted cookie error = %v, want non-empty string", got)
	}
}

func TestDialectCookieInvalidInputReturnsError(t *testing.T) {
	interp := runWithLib(t, `
		parsed, parse_err := dialect.eval("cookie", "ok=1; missing")
		encoded_scalar, encode_scalar_err := dialect.eval("cookie", "ok=1", {mode: "encode"})
		bad := {}
		bad["bad name"] = "x"
		encoded, encode_err := dialect.eval("cookie", bad, {mode: "encode"})
		bad_value := {}
		bad_value.ok = "first;second"
		encoded_value, encode_value_err := dialect.eval("cookie", bad_value, {mode: "encode"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if !interp.GetGlobal("parsed").IsNil() {
		t.Fatalf("invalid parse returned non-nil result")
	}
	if got := interp.GetGlobal("parse_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid parse error = %v, want non-empty string", got)
	}
	if !interp.GetGlobal("encoded_scalar").IsNil() {
		t.Fatalf("invalid scalar encode returned non-nil result")
	}
	if got := interp.GetGlobal("encode_scalar_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid scalar encode error = %v, want non-empty string", got)
	}
	if !interp.GetGlobal("encoded").IsNil() {
		t.Fatalf("invalid encode returned non-nil result")
	}
	if got := interp.GetGlobal("encode_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid encode error = %v, want non-empty string", got)
	}
	if !interp.GetGlobal("encoded_value").IsNil() {
		t.Fatalf("invalid encode value returned non-nil result")
	}
	if got := interp.GetGlobal("encode_value_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid encode value error = %v, want non-empty string", got)
	}
}

func TestDialectSSEInvalidInputReturnsError(t *testing.T) {
	interp := runWithLib(t, `
		bad_retry, bad_retry_err := dialect.eval("sse", "retry: soon\n\n")
		ok_scalar_encode, scalar_encode_err := pcall(dialect.eval, "sse", "data: hi\n\n", {mode: "encode"})
		ok_bad_retry_encode, bad_retry_encode_err := pcall(dialect.eval, "sse", {{data: "hi", retry: "soon"}}, {mode: "format"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if !interp.GetGlobal("bad_retry").IsNil() {
		t.Fatalf("invalid retry parse returned non-nil result")
	}
	if got := interp.GetGlobal("bad_retry_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("invalid retry parse error = %v, want non-empty string", got)
	}
	if got := interp.GetGlobal("ok_scalar_encode").Bool(); got {
		t.Fatalf("scalar encode pcall succeeded, want failure")
	}
	if got := interp.GetGlobal("scalar_encode_err"); got.IsNil() || got.String() == "" {
		t.Fatalf("scalar encode error = %v, want non-empty error", got)
	}
	if got := interp.GetGlobal("ok_bad_retry_encode").Bool(); got {
		t.Fatalf("bad retry encode pcall succeeded, want failure")
	}
	if got := interp.GetGlobal("bad_retry_encode_err"); got.IsNil() || got.String() == "" {
		t.Fatalf("bad retry encode error = %v, want non-empty error", got)
	}
}

func TestDialectMultipartInvalidInputReturnsError(t *testing.T) {
	interp := runWithLib(t, `
		bad_content_type, bad_content_type_err := dialect.eval("multipart", "--x--\r\n", {content_type: "multipart/form-data; boundary"})
		bad_boundary, bad_boundary_err := dialect.eval("multipart", "--bad--\r\n", {boundary: "bad\r\nboundary"})
		bad_parse, bad_parse_err := dialect.eval("multipart", "not multipart", {boundary: "fixture"})
		bad_scalar_encode, bad_scalar_encode_err := dialect.eval("multipart", "not parts", {mode: "encode", boundary: "fixture"})
		bad_item_encode, bad_item_encode_err := dialect.eval("multipart", {"not table"}, {mode: "format", boundary: "fixture"})
		bad_header := {{name: "field", body: "value", headers: {["bad name"]: "x"}}}
		bad_header_encode, bad_header_encode_err := dialect.eval("multipart", bad_header, {mode: "encode", boundary: "fixture"})
		bad_header_value := {{name: "field", body: "value", headers: {["x-ok"]: "first\r\nsecond: no"}}}
		bad_header_value_encode, bad_header_value_encode_err := dialect.eval("multipart", bad_header_value, {mode: "encode", boundary: "fixture"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	for _, name := range []string{"bad_content_type", "bad_boundary", "bad_parse", "bad_scalar_encode", "bad_item_encode", "bad_header_encode", "bad_header_value_encode"} {
		if !interp.GetGlobal(name).IsNil() {
			t.Fatalf("%s returned non-nil result", name)
		}
	}
	for _, name := range []string{"bad_content_type_err", "bad_boundary_err", "bad_parse_err", "bad_scalar_encode_err", "bad_item_encode_err", "bad_header_encode_err", "bad_header_value_encode_err"} {
		if got := interp.GetGlobal(name); !got.IsString() || got.Str() == "" {
			t.Fatalf("%s = %v, want non-empty string", name, got)
		}
	}
}

func TestDialectJWTInvalidInputReturnsError(t *testing.T) {
	interp := runWithLib(t, `
		bad_segments, bad_segments_err := dialect.eval("jwt", "not-a-token")
		bad_header_b64, bad_header_b64_err := dialect.eval("jwt", "%.e30.sig")
		bad_payload_b64, bad_payload_b64_err := dialect.eval("jwt", "e30.%.sig")
		bad_header_json, bad_header_json_err := dialect.eval("jwt", "bm90LWpzb24.e30.sig")
		bad_payload_json, bad_payload_json_err := dialect.eval("jwt", "e30.bm90LWpzb24.sig", {mode: "decode"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	for _, name := range []string{"bad_segments", "bad_header_b64", "bad_payload_b64", "bad_header_json", "bad_payload_json"} {
		if !interp.GetGlobal(name).IsNil() {
			t.Fatalf("%s returned non-nil result", name)
		}
	}
	for _, name := range []string{"bad_segments_err", "bad_header_b64_err", "bad_payload_b64_err", "bad_header_json_err", "bad_payload_json_err"} {
		if got := interp.GetGlobal(name); !got.IsString() || got.Str() == "" {
			t.Fatalf("%s = %v, want non-empty string", name, got)
		}
	}
}
