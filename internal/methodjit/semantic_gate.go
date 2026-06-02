//go:build darwin && arm64

package methodjit

import (
	"os"

	"github.com/never-labs/leia/internal/vm"
)

// jitUnsupportedMultiReturn reports whether proto uses return forms that the
// current method JIT ABI cannot faithfully represent. Tier 1 and Tier 2 both
// return one boxed value to the VM, so fixed multi-return and "return all"
// functions must stay in the interpreter until the JIT grows a real result
// slice ABI.
func jitUnsupportedMultiReturn(proto *vm.FuncProto) bool {
	if proto == nil {
		return false
	}
	for _, inst := range proto.Code {
		if vm.DecodeOp(inst) != vm.OP_RETURN {
			continue
		}
		b := vm.DecodeB(inst)
		if b == 0 || b > 2 {
			return true
		}
	}
	return false
}

// jitShouldStayInInterpreter reports proto shapes where executing the method
// body natively is not currently a semantic win. The top-level script is an
// observable harness: if an exit handler surfaces a script error after partial
// native execution, VM fallback would replay prints/mutations from pc=0. Keep
// it interpreted and let ordinary function bodies called by the script exercise
// the JIT.
func jitShouldStayInInterpreter(proto *vm.FuncProto) bool {
	if proto == nil {
		return false
	}
	return proto.Name == "<main>" ||
		jitUnsupportedConcurrencyControl(proto) ||
		jitUnsupportedVMOnlyControl(proto) ||
		jitUnsupportedMultiReturn(proto) ||
		jitUnsupportedClosureArithmetic(proto) ||
		jitUnsupportedDynamicOperators(proto) ||
		jitUnsupportedComparisonBranch(proto) ||
		jitUnsupportedCallBoundary(proto)
}

func jitRequiresInterpreter(proto *vm.FuncProto) bool {
	return jitUnsupportedVMOnlyControl(proto) ||
		jitUnsupportedConcurrencyControl(proto) ||
		jitUnsupportedMultiReturn(proto)
}

func jitTier2CallableGate(proto *vm.FuncProto) GateResult {
	decision := proto.MethodJITTier2CallableDecision()
	if decision.Allowed {
		return allowGate("Tier2Callable", decision.Reason)
	}
	return blockGate("Tier2Callable", decision.Reason)
}

func jitSemanticGateEnabled() bool {
	return os.Getenv("LEIA_OFFICIAL_CHECK_JIT") == "1" ||
		os.Getenv("LEIA_JIT_SEMANTIC_GATE") == "1"
}

func jitUnsupportedClosureArithmetic(proto *vm.FuncProto) bool {
	if proto == nil || len(proto.Upvalues) == 0 {
		return false
	}
	for _, inst := range proto.Code {
		switch vm.DecodeOp(inst) {
		case vm.OP_ADD, vm.OP_SUB, vm.OP_MUL, vm.OP_DIV, vm.OP_MOD:
			return true
		}
	}
	return false
}

func jitUnsupportedComparisonBranch(proto *vm.FuncProto) bool {
	if proto == nil {
		return false
	}
	for _, inst := range proto.Code {
		switch vm.DecodeOp(inst) {
		case vm.OP_EQ, vm.OP_LT, vm.OP_LE:
			return true
		}
	}
	return false
}

func jitUnsupportedCallBoundary(proto *vm.FuncProto) bool {
	if proto == nil {
		return false
	}
	for _, inst := range proto.Code {
		switch vm.DecodeOp(inst) {
		case vm.OP_CALL, vm.OP_CALLTABLE, vm.OP_SELF, vm.OP_TFORCALL, vm.OP_TFORLOOP, vm.OP_RESUME, vm.OP_YIELD:
			return true
		}
	}
	return false
}

func jitUnsupportedConcurrencyControl(proto *vm.FuncProto) bool {
	if proto == nil {
		return false
	}
	for _, inst := range proto.Code {
		switch vm.DecodeOp(inst) {
		case vm.OP_GO, vm.OP_MAKECHAN, vm.OP_SEND, vm.OP_RECV, vm.OP_RECVOK,
			vm.OP_TRYSEND, vm.OP_TRYRECV, vm.OP_TRYRECVOK, vm.OP_SELECT:
			return true
		}
	}
	return false
}

func jitUnsupportedVMOnlyControl(proto *vm.FuncProto) bool {
	if proto == nil {
		return false
	}
	if len(proto.ReadOnlyLocals) > 0 {
		return true
	}
	for _, uv := range proto.Upvalues {
		if uv.ReadOnly {
			return true
		}
	}
	for _, inst := range proto.Code {
		switch vm.DecodeOp(inst) {
		case vm.OP_DEFER, vm.OP_SETGLOBALRO, vm.OP_CHECKCONST:
			return true
		}
	}
	return false
}

func jitUnsupportedDynamicOperators(proto *vm.FuncProto) bool {
	if proto == nil {
		return false
	}
	for _, inst := range proto.Code {
		switch vm.DecodeOp(inst) {
		case vm.OP_ADD, vm.OP_SUB, vm.OP_MUL, vm.OP_DIV, vm.OP_MOD, vm.OP_POW, vm.OP_UNM, vm.OP_LEN, vm.OP_CONCAT:
			return true
		}
	}
	return false
}
