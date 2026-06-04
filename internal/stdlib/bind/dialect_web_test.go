package bind

import "testing"

func TestDialectURLParseBoundaries(t *testing.T) {
	interp := runWithLib(t, `
		parsed := dialect.eval("url", "http://user:p%40ss@[2001:db8::1]:8080/a%20b?tag=a&tag=b&empty=#frag")
		invalid, invalid_err := dialect.eval("url", "http://[::1")
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
}

func TestDialectURLQueryBoundaryModes(t *testing.T) {
	interp := runWithLib(t, `
		to_encode := {}
		to_encode.tag = {"b", "a"}
		to_encode.empty = ""
		to_encode.page = 2
		encoded := dialect.eval("urlquery", to_encode)
		parsed := dialect.eval("urlquery", "tag=b&tag=a&empty=&space=a+b")
		bad_component, bad_component_err := dialect.eval("urlquery", "%zz", {mode: "unescape"})
		bad_query, bad_query_err := dialect.eval("urlquery", "ok=1&bad=%zz")
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
	if !interp.GetGlobal("bad").IsNil() || !interp.GetGlobal("bad_err").IsString() {
		t.Fatalf("bad SSE = %v err %v, want nil error string", interp.GetGlobal("bad"), interp.GetGlobal("bad_err"))
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
		bad_start, bad_start_err := dialect.eval("httpmsg", "not http\r\nx: y\r\n\r\n")
		bad_header, bad_header_err := dialect.eval("httpmsg", "GET / HTTP/1.1\r\nbad header\r\n\r\n")
		bad_encode, bad_encode_err := dialect.eval("httpmsg", {method: "BAD METHOD", target: "/"}, {mode: "encode"})
		bad_header_encode, bad_header_encode_err := dialect.eval("httpmsg", {method: "GET", headers: {["bad name"]: "x"}}, {mode: "encode"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	for _, name := range []string{"bad_start", "bad_header", "bad_encode", "bad_header_encode"} {
		if !interp.GetGlobal(name).IsNil() {
			t.Fatalf("%s returned non-nil result", name)
		}
	}
	for _, name := range []string{"bad_start_err", "bad_header_err", "bad_encode_err", "bad_header_encode_err"} {
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
