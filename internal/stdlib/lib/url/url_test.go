package url

import "testing"

func TestParse(t *testing.T) {
	parsed, err := Parse("https://user:pass@example.com:8080/path?q=1&r=2#frag")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if parsed.Scheme != "https" {
		t.Fatalf("scheme = %q, want %q", parsed.Scheme, "https")
	}
	if parsed.Host != "example.com" {
		t.Fatalf("host = %q, want %q", parsed.Host, "example.com")
	}
	if parsed.Port != "8080" {
		t.Fatalf("port = %q, want %q", parsed.Port, "8080")
	}
	if parsed.Path != "/path" {
		t.Fatalf("path = %q, want %q", parsed.Path, "/path")
	}
	if parsed.Fragment != "frag" {
		t.Fatalf("fragment = %q, want %q", parsed.Fragment, "frag")
	}
	if parsed.User != "user" {
		t.Fatalf("user = %q, want %q", parsed.User, "user")
	}
	if parsed.Password == nil || *parsed.Password != "pass" {
		t.Fatalf("password = %v, want %q", parsed.Password, "pass")
	}
	if parsed.Query["q"] != "1" {
		t.Fatalf("query[q] = %q, want %q", parsed.Query["q"], "1")
	}
	if parsed.Raw != "https://user:pass@example.com:8080/path?q=1&r=2#frag" {
		t.Fatalf("raw = %q", parsed.Raw)
	}
}

func TestBuild(t *testing.T) {
	password := "pass"
	got := Build(Parts{
		Scheme:   "https",
		Host:     "example.com",
		Port:     "8080",
		Path:     "/api/v1",
		Fragment: "frag",
		User:     "user",
		HasUser:  true,
		Password: &password,
		Query: map[string]string{
			"key": "value",
		},
	})

	want := "https://user:pass@example.com:8080/api/v1?key=value#frag"
	if got != want {
		t.Fatalf("Build() = %q, want %q", got, want)
	}
}

func TestEncodeDecode(t *testing.T) {
	encoded := Encode("hello world & more")
	if encoded != "hello+world+%26+more" {
		t.Fatalf("Encode() = %q", encoded)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if decoded != "hello world & more" {
		t.Fatalf("Decode() = %q", decoded)
	}
}

func TestQueryEncodeDecode(t *testing.T) {
	encoded := QueryEncode(map[string]string{
		"a": "1",
		"b": "hello world",
	})
	if encoded != "a=1&b=hello+world" {
		t.Fatalf("QueryEncode() = %q", encoded)
	}

	decoded, err := QueryDecode(encoded)
	if err != nil {
		t.Fatalf("QueryDecode returned error: %v", err)
	}
	if decoded["a"] != "1" {
		t.Fatalf("decoded[a] = %q", decoded["a"])
	}
	if decoded["b"] != "hello world" {
		t.Fatalf("decoded[b] = %q", decoded["b"])
	}
}

func TestJoin(t *testing.T) {
	got, err := Join("https://example.com/base/", "../other")
	if err != nil {
		t.Fatalf("Join returned error: %v", err)
	}
	if got != "https://example.com/other" {
		t.Fatalf("Join() = %q", got)
	}
}

func TestIsValidHostAndPath(t *testing.T) {
	if !IsValid("https://example.com/foo") {
		t.Fatalf("IsValid() = false")
	}
	if IsValid("not a url") {
		t.Fatalf("IsValid() = true")
	}

	host, err := Host("https://example.com:8080/foo")
	if err != nil {
		t.Fatalf("Host returned error: %v", err)
	}
	if host != "example.com" {
		t.Fatalf("Host() = %q", host)
	}

	path, err := Path("https://example.com:8080/foo")
	if err != nil {
		t.Fatalf("Path returned error: %v", err)
	}
	if path != "/foo" {
		t.Fatalf("Path() = %q", path)
	}
}
