//go:build darwin && arm64

package methodjit

import (
	"testing"

	"github.com/Never-Labs/gscript/internal/vm"
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

func TestJITSemanticGateRejectsComparisonBranches(t *testing.T) {
	top := compileTop(t, `func f(a) { if a < 10 { return 1 } }`)
	fn := findFirstProtoWithName(t, top, "f")
	if !jitShouldStayInInterpreter(fn) {
		t.Fatal("comparison branch function should stay interpreted until JIT comparison semantics are complete")
	}
}

func TestJITSemanticGateRejectsCallBoundaries(t *testing.T) {
	top := compileTop(t, `func f(g) { return g(1) }`)
	fn := findFirstProtoWithName(t, top, "f")
	if !jitShouldStayInInterpreter(fn) {
		t.Fatal("call boundary function should stay interpreted until JIT call/error boundaries are complete")
	}
}

func TestJITSemanticGateRequiresInterpreterForConcurrencyOps(t *testing.T) {
	top := compileTop(t, `
func f(ch) {
	go func() { ch <- 1 }()
	return <-ch
}
`)
	fn := findFirstProtoWithName(t, top, "f")
	if !jitRequiresInterpreter(fn) {
		t.Fatal("concurrency op function should stay interpreted to preserve blocking/scheduling semantics")
	}
	if !jitShouldStayInInterpreter(fn) {
		t.Fatal("concurrency op function should be covered by semantic gate")
	}
}

func TestJITSemanticGateRejectsDynamicOperators(t *testing.T) {
	top := compileTop(t, `func f(a, b) { return a + b }`)
	fn := findFirstProtoWithName(t, top, "f")
	if !jitShouldStayInInterpreter(fn) {
		t.Fatal("dynamic operator function should stay interpreted until JIT type/error fallbacks are complete")
	}
}

func TestTier2CallableGateAllowsDeclaredVarargWithoutOPVararg(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vararg_unused",
		IsVarArg: true,
		Code:     []uint32{vm.EncodeABC(vm.OP_RETURN, 0, 1, 0)},
	}
	if !proto.MethodJITTier1Callable() {
		t.Fatal("declared but unread vararg function should remain Tier 1 callable")
	}
	if !proto.MethodJITTier2Callable() {
		t.Fatal("declared but unread vararg function should be Tier 2 callable")
	}
	if !canPromoteToTier2(proto) {
		t.Fatal("declared but unread vararg function should pass Tier 2 bytecode gate")
	}
	gate := jitTier2CallableGate(proto)
	if !gate.Allowed {
		t.Fatal("Tier2Callable gate should allow declared but unread vararg function")
	}
	if gate.Reason != vm.MethodJITCallableReasonDeclaredVarargTier2 {
		t.Fatalf("Tier2Callable reason = %q, want %q", gate.Reason, vm.MethodJITCallableReasonDeclaredVarargTier2)
	}
}

func TestTier2CallableGateRejectsOPVarargWithoutDeclaration(t *testing.T) {
	proto := &vm.FuncProto{
		Name:               "op_vararg",
		UsesVarargBytecode: true,
		Code:               []uint32{vm.EncodeABC(vm.OP_VARARG, 0, 2, 0), vm.EncodeABC(vm.OP_RETURN, 0, 2, 0)},
	}
	if !proto.MethodJITTier1Callable() {
		t.Fatal("legacy Tier 1 callable boundary should continue allowing this bytecode shape")
	}
	if proto.MethodJITTier2Callable() {
		t.Fatal("OP_VARARG function should not be Tier 2 callable")
	}
	if canPromoteToTier2(proto) {
		t.Fatal("OP_VARARG function should not pass Tier 2 bytecode gate")
	}
	gate := jitTier2CallableGate(proto)
	if gate.Allowed {
		t.Fatal("Tier2Callable gate should reject OP_VARARG function")
	}
	if gate.Reason != vm.MethodJITCallableReasonOPVarargNeedsVMFrame {
		t.Fatalf("Tier2Callable reason = %q, want %q", gate.Reason, vm.MethodJITCallableReasonOPVarargNeedsVMFrame)
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
