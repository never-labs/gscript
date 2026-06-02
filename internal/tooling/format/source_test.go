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

func TestSourceFormatsEvaluateBlockIndentation(t *testing.T) {
	src := []byte("evaluate \"slug baseline\" {\nassert(true)\nif true {\nassert(true)\n}\n}\n")
	got, err := Source("eval.leia", src)
	if err != nil {
		t.Fatalf("Source returned error: %v", err)
	}
	want := "evaluate \"slug baseline\" {\n    assert(true)\n    if true {\n        assert(true)\n    }\n}\n"
	if string(got) != want {
		t.Fatalf("Source() = %q, want %q", string(got), want)
	}
	again, err := Source("eval.leia", got)
	if err != nil {
		t.Fatalf("Source second pass returned error: %v", err)
	}
	if string(again) != want {
		t.Fatalf("Source second pass = %q, want %q", string(again), want)
	}
}
