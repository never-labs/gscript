package vm

import (
	"github.com/never-labs/gscript/internal/testutil/vmtest"
	"testing"

	"github.com/never-labs/gscript/internal/lexer"
	"github.com/never-labs/gscript/internal/parser"
)

func compileSpectralSpecializationTestProgram(t *testing.T, src string) (*FuncProto, *VM) {
	t.Helper()
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		t.Fatalf("lexer error: %v", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	proto, err := Compile(prog)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	return proto, New(vmtest.NewInterpreterGlobals())
}

func findTestProtoByName(proto *FuncProto, name string) *FuncProto {
	if proto == nil {
		return nil
	}
	if proto.Name == name {
		return proto
	}
	for _, child := range proto.Protos {
		if got := findTestProtoByName(child, name); got != nil {
			return got
		}
	}
	return nil
}
