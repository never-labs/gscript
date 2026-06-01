package format

import "testing"

func TestSourceNormalizesWhitespaceAndIndentation(t *testing.T) {
	got, err := Source("scratch.leia", []byte("if true {\r\nreturn 1  \r\n}\r\n\r\n"))
	if err != nil {
		t.Fatalf("Source returned error: %v", err)
	}
	if want := "if true {\n    return 1\n}\n"; string(got) != want {
		t.Fatalf("Source() = %q, want %q", string(got), want)
	}
}

func TestSourceRejectsInvalidSyntax(t *testing.T) {
	if _, err := Source("scratch.leia", []byte("x := \n")); err == nil {
		t.Fatalf("Source returned nil error for invalid syntax")
	}
}
