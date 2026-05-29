package runtime

import "testing"

func TestTimeAfterChannel(t *testing.T) {
	interp := New()
	tokens, err := lexerNew(`
deadline := time.after(0.001)
result := "none"
select {
case <-deadline:
    result = "timeout"
}
`)
	if err != nil {
		t.Fatalf("lexer: %v", err)
	}
	prog, err := parserNew(tokens)
	if err != nil {
		t.Fatalf("parser: %v", err)
	}
	if err := interp.Exec(prog); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if got := interp.GetGlobal("result"); !got.IsString() || got.Str() != "timeout" {
		t.Fatalf("result = %v, want timeout", got)
	}
}
