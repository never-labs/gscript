package runtime

import (
	"testing"

	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/parser"
)

func execTestProgram(t *testing.T, interp *Interpreter, src string) {
	t.Helper()
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		t.Fatalf("lexer error: %v", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := interp.Exec(prog); err != nil {
		t.Fatalf("exec error: %v", err)
	}
}

func execBinaryIOTest(t *testing.T, interp *Interpreter, src string) {
	t.Helper()
	execTestProgram(t, interp, src)
}
