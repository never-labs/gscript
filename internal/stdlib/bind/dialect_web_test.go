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
