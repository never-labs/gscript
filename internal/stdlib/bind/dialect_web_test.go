package bind

import "testing"

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
