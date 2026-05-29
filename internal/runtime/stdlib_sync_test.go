package runtime

import "testing"

func TestSyncWaitGroup(t *testing.T) {
	interp := New()
	code := `
wg := sync.waitgroup()
ch := make(chan, 4)
for i := 1; i <= 4; i++ {
    wg.add(1)
    go func(v) {
        ch <- v
        wg.done()
    }(i)
}
wg.wait()
total := 0
for i := 1; i <= 4; i++ {
    total = total + <-ch
}
`
	tokens, err := lexerNew(code)
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
	if got := interp.GetGlobal("total"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("total = %v, want 10", got)
	}
}
