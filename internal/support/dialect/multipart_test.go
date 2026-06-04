package dialect

import (
	"strings"
	"testing"
)

func TestParseMultipart(t *testing.T) {
	src := strings.Join([]string{
		"--fixture",
		`Content-Disposition: form-data; name="meta"`,
		"Content-Type: application/json",
		"",
		`{"ok":true}`,
		"--fixture",
		`Content-Disposition: form-data; name="file"; filename="a.txt"`,
		"Content-Type: text/plain",
		"",
		"hello",
		"--fixture--",
		"",
	}, "\r\n")

	parts, err := ParseMultipart(src, "fixture")
	if err != nil {
		t.Fatalf("ParseMultipart error: %v", err)
	}
	if len(parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(parts))
	}
	if parts[0].Name != "meta" || parts[0].ContentType != "application/json" || parts[0].Body != `{"ok":true}` {
		t.Fatalf("first part = %+v", parts[0])
	}
	if parts[1].Name != "file" || parts[1].Filename != "a.txt" || parts[1].Body != "hello" {
		t.Fatalf("second part = %+v", parts[1])
	}
}

func TestEncodeMultipartDeterministic(t *testing.T) {
	got, err := EncodeMultipart([]MultipartPart{
		{Name: "meta", ContentType: "application/json", Body: `{"ok":true}`},
		{Name: "file", Filename: "a.txt", ContentType: "text/plain", Body: "hello"},
	}, "fixture")
	if err != nil {
		t.Fatalf("EncodeMultipart error: %v", err)
	}
	want := strings.Join([]string{
		"--fixture",
		`Content-Disposition: form-data; name=meta`,
		"Content-Type: application/json",
		"",
		`{"ok":true}`,
		"--fixture",
		`Content-Disposition: form-data; filename=a.txt; name=file`,
		"Content-Type: text/plain",
		"",
		"hello",
		"--fixture--",
		"",
	}, "\r\n")
	if got != want {
		t.Fatalf("encoded multipart = %q, want %q", got, want)
	}
}

func TestMultipartRejectsInvalidBoundary(t *testing.T) {
	if _, err := ParseMultipart("", "bad\r\nboundary"); err == nil {
		t.Fatalf("ParseMultipart accepted invalid boundary")
	}
	if _, err := EncodeMultipart(nil, "bad\r\nboundary"); err == nil {
		t.Fatalf("EncodeMultipart accepted invalid boundary")
	}
}
