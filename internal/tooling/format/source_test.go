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

func TestSourceFormatsTaggedBlockIndentation(t *testing.T) {
	src := []byte("prompt {\nsystem: \"Be brief.\"\nuser: \"hello\"\noptions: {\nmax_tokens: 16\n}\n}\n")
	got, err := Source("eval.leia", src)
	if err != nil {
		t.Fatalf("Source returned error: %v", err)
	}
	want := "prompt {\n    system: \"Be brief.\"\n    user: \"hello\"\n    options: {\n        max_tokens: 16\n    }\n}\n"
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

func TestSourcePreservesRawStringIndentationInsideTaggedRawBlock(t *testing.T) {
	src := []byte("quote {\nbody := `{\n  \"ok\": true\n}`\nif true {\nreturn body\n}\n}\n")
	got, err := Source("eval.leia", src)
	if err != nil {
		t.Fatalf("Source returned error: %v", err)
	}
	want := "quote {\n    body := `{\n  \"ok\": true\n}`\n    if true {\n        return body\n    }\n}\n"
	if string(got) != want {
		t.Fatalf("Source() = %q, want %q", string(got), want)
	}
}

func TestSourcePreservesCommentLikeLinesInsideRawString(t *testing.T) {
	src := []byte("quote {\nbody := `first\n// keep at column zero\n  // keep two spaces\nlast`\n}\n")
	got, err := Source("eval.leia", src)
	if err != nil {
		t.Fatalf("Source returned error: %v", err)
	}
	want := "quote {\n    body := `first\n// keep at column zero\n  // keep two spaces\nlast`\n}\n"
	if string(got) != want {
		t.Fatalf("Source() = %q, want %q", string(got), want)
	}
}

func TestSourcePreservesTrailingSpacesInsideRawString(t *testing.T) {
	src := []byte("quote {\nbody := `first  \nmid\t \nlast`\n}\n")
	got, err := Source("eval.leia", src)
	if err != nil {
		t.Fatalf("Source returned error: %v", err)
	}
	want := "quote {\n    body := `first  \nmid\t \nlast`\n}\n"
	if string(got) != want {
		t.Fatalf("Source() = %q, want %q", string(got), want)
	}
}
