//go:build darwin && arm64

package methodjit

import (
	"testing"

	"github.com/gscript/gscript/internal/vm"
)

func TestJITSemanticGateKeepsTopLevelInInterpreter(t *testing.T) {
	top := compileTop(t, `print("x")`)
	if !jitShouldStayInInterpreter(top) {
		t.Fatal("top-level script should stay interpreted")
	}
}

func TestJITSemanticGateRejectsMultiReturnABI(t *testing.T) {
	top := compileTop(t, `func f() { return 1, 2 }`)
	fn := findFirstProtoWithName(t, top, "f")
	if !jitShouldStayInInterpreter(fn) {
		t.Fatal("multi-return function should stay interpreted")
	}
}

func TestJITSemanticGateRejectsClosureArithmetic(t *testing.T) {
	top := compileTop(t, `
func make(a) {
	return func(b) { return a + b }
}
`)
	inner := findFirstAnonymousProto(t, top)
	if !jitShouldStayInInterpreter(inner) {
		t.Fatal("closure arithmetic should stay interpreted until JIT numeric guards cover upvalues")
	}
}

func findFirstProtoWithName(t *testing.T, root *vm.FuncProto, name string) *vm.FuncProto {
	t.Helper()
	if root.Name == name {
		return root
	}
	for _, child := range root.Protos {
		if got := findFirstProtoWithName(t, child, name); got != nil {
			return got
		}
	}
	return nil
}

func findFirstAnonymousProto(t *testing.T, root *vm.FuncProto) *vm.FuncProto {
	t.Helper()
	for _, child := range root.Protos {
		if child.Name == "<anonymous>" {
			return child
		}
		if got := findFirstAnonymousProto(t, child); got != nil {
			return got
		}
	}
	return nil
}
