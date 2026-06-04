package dialect

import (
	"reflect"
	"strings"
	"testing"
)

func TestShellwordsParsesQuotesAndEscapes(t *testing.T) {
	got, err := Shellwords(`printf "hello world" 'it'\''s' a\ b ""`)
	if err != nil {
		t.Fatalf("Shellwords: %v", err)
	}
	want := []string{"printf", "hello world", "it's", "a b", ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Shellwords = %#v, want %#v", got, want)
	}
}

func TestShellwordsRejectsMalformedInput(t *testing.T) {
	for _, src := range []string{`"unterminated`, `'unterminated`, `trailing\`, "bad\x00arg"} {
		if _, err := Shellwords(src); err == nil {
			t.Fatalf("Shellwords(%q) returned nil error", src)
		}
	}
}

func TestShellwordsEncodeRoundTrip(t *testing.T) {
	args := []string{"printf", "%s\n", "hello world", "it's", "", "safe/path-1"}
	encoded, err := ShellwordsEncode(args)
	if err != nil {
		t.Fatalf("ShellwordsEncode: %v", err)
	}
	if !strings.Contains(encoded, `'hello world'`) || !strings.Contains(encoded, `'it'\''s'`) || !strings.Contains(encoded, `''`) {
		t.Fatalf("encoded = %q, want shell-safe quoting", encoded)
	}
	decoded, err := Shellwords(encoded)
	if err != nil {
		t.Fatalf("Shellwords(encoded): %v", err)
	}
	if !reflect.DeepEqual(decoded, args) {
		t.Fatalf("round trip = %#v, want %#v", decoded, args)
	}
}
