//go:build darwin && arm64

package methodjit

import (
	"os"

	"github.com/gscript/gscript/internal/vm"
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
		jitUnsupportedMultiReturn(proto) ||
		jitUnsupportedClosureArithmetic(proto)
}

func jitSemanticGateEnabled() bool {
	return os.Getenv("GSCRIPT_OFFICIAL_CHECK_JIT") == "1" ||
		os.Getenv("GSCRIPT_JIT_SEMANTIC_GATE") == "1"
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
