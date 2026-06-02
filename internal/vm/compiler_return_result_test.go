package vm

import (
	"testing"

	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/parser"
)

func TestCompilerReturnSingleResultCallsStayJITEligible(t *testing.T) {
	proto := compileCompilerReturnResultProto(t, `
func fact(n, acc) {
	if n <= 1 { return acc }
	return fact(n - 1, acc * n)
}

func label(n) {
	return string.format("key%d", n)
}
`)

	for _, name := range []string{"fact", "label"} {
		fn := findCompilerReturnResultProto(proto, name)
		if fn == nil {
			t.Fatalf("function %q not found", name)
		}
		if fn.JITDisabled {
			t.Fatalf("%s was marked JITDisabled by fixed single-result return lowering", name)
		}
		if hasReturnAll(fn) {
			t.Fatalf("%s used dynamic return-all ABI for a statically single-result call", name)
		}
	}
}

func TestCompilerReturnUnknownCallKeepsDynamicMultiReturn(t *testing.T) {
	proto := compileCompilerReturnResultProto(t, `
func triple() {
	return 1, 2, 3
}

func forward() {
	return triple()
}
`)

	forward := findCompilerReturnResultProto(proto, "forward")
	if forward == nil {
		t.Fatal("function forward not found")
	}
	if !hasReturnAll(forward) {
		t.Fatal("forward should use return-all ABI for return triple()")
	}
}

func TestCompilerReturnNoResultCallKeepsDynamicReturn(t *testing.T) {
	proto := compileCompilerReturnResultProto(t, `
func no_result() {
	x := 1
}

func forward_none() {
	return no_result()
}
`)

	forward := findCompilerReturnResultProto(proto, "forward_none")
	if forward == nil {
		t.Fatal("function forward_none not found")
	}
	if !hasReturnAll(forward) {
		t.Fatal("forward_none should use return-all ABI so zero-result calls remain zero-result")
	}
}

func compileCompilerReturnResultProto(t *testing.T, src string) *FuncProto {
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
	return proto
}

func findCompilerReturnResultProto(proto *FuncProto, name string) *FuncProto {
	if proto == nil {
		return nil
	}
	if proto.Name == name {
		return proto
	}
	for _, child := range proto.Protos {
		if found := findCompilerReturnResultProto(child, name); found != nil {
			return found
		}
	}
	return nil
}

func hasReturnAll(proto *FuncProto) bool {
	for _, inst := range proto.Code {
		if DecodeOp(inst) == OP_RETURN && DecodeB(inst) == 0 {
			return true
		}
	}
	return false
}
