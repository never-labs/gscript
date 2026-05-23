package vm

import (
	"testing"

	"github.com/gscript/gscript/internal/lexer"
	"github.com/gscript/gscript/internal/parser"
	"github.com/gscript/gscript/internal/runtime"
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
	return proto, New(runtime.NewInterpreterGlobals())
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
