//go:build darwin && arm64

// tier1_call_closure_expr.go holds the Tier 1 baseline simple/immediate
// closure-expression fast-path emitters.
// Split out of tier1_call.go by pure code movement.

package methodjit

import (
	"unsafe"

	"github.com/gscript/gscript/internal/jit"
	"github.com/gscript/gscript/internal/vm"
)

func emitBaselineSimpleClosureExprFastPath(asm *jit.Assembler, callerProto *vm.FuncProto, doneLabel string, callSlot int) {
	fastPaths := simpleClosureExprFastPathsForProto(callerProto)
	if len(fastPaths) == 0 {
		return
	}
	missLabel := nextLabel("simple_closure_expr_miss")
	for _, fast := range fastPaths {
		nextFastLabel := nextLabel("simple_closure_expr_next")
		asm.LoadImm64(jit.X3, int64(uintptr(unsafe.Pointer(fast.proto))))
		asm.CMPreg(jit.X1, jit.X3)
		asm.BCond(jit.CondNE, nextFastLabel)

		emitSimpleClosureExprValue(asm, fast.expr, callSlot+1, len(fast.proto.Upvalues), jit.X4, jit.X7, jit.X5, jit.X6, missLabel)
		jit.EmitBoxIntFast(asm, jit.X4, jit.X4, mRegTagInt)
		storeSlot(asm, callSlot, jit.X4)
		asm.B(doneLabel)

		asm.Label(nextFastLabel)
	}
	asm.Label(missLabel)
}

func emitBaselineImmediateClosureFactoryFastPath(asm *jit.Assembler, callerProto *vm.FuncProto, pc, factoryCallSlot int) {
	fastPaths := immediateClosureFactoryFastPathsForProto(callerProto)
	if len(fastPaths) == 0 || pc+4 >= len(callerProto.Code) {
		return
	}
	moveCallee := callerProto.Code[pc+1]
	moveArg := callerProto.Code[pc+2]
	callClosure := callerProto.Code[pc+3]
	if vm.DecodeOp(moveCallee) != vm.OP_MOVE ||
		vm.DecodeB(moveCallee) != factoryCallSlot ||
		vm.DecodeOp(moveArg) != vm.OP_MOVE ||
		vm.DecodeOp(callClosure) != vm.OP_CALL ||
		vm.DecodeB(callClosure) != 2 ||
		vm.DecodeC(callClosure) != 2 ||
		vm.DecodeA(callClosure) != vm.DecodeA(moveCallee) ||
		vm.DecodeA(moveArg) != vm.DecodeA(callClosure)+1 {
		return
	}

	resultSlot := vm.DecodeA(callClosure)
	argSrcSlot := vm.DecodeB(moveArg)
	missLabel := nextLabel("immediate_closure_factory_miss")
	for _, fast := range fastPaths {
		nextFastLabel := nextLabel("immediate_closure_factory_next")
		asm.LoadImm64(jit.X3, int64(uintptr(unsafe.Pointer(fast.proto))))
		asm.CMPreg(jit.X1, jit.X3)
		asm.BCond(jit.CondNE, nextFastLabel)

		emitImmediateClosureFactoryExprValue(asm, fast.expr, argSrcSlot, factoryCallSlot+1, fast.upvalSlots, jit.X4, jit.X7, jit.X5, missLabel)
		jit.EmitBoxIntFast(asm, jit.X4, jit.X4, mRegTagInt)
		storeSlot(asm, resultSlot, jit.X4)
		asm.B(pcLabel(pc + 4))

		asm.Label(nextFastLabel)
	}
	asm.Label(missLabel)
}

func emitSimpleClosureExprValue(asm *jit.Assembler, expr simpleClosureExpr, argSlot, upvalCount int, dst, rhs, tagScratch, refScratch jit.Reg, missLabel string) {
	switch expr.kind {
	case simpleClosureExprParam:
		loadSlot(asm, dst, argSlot)
		emitCheckIsIntPinned(asm, dst, tagScratch)
		asm.BCond(jit.CondNE, missLabel)
		jit.EmitUnboxInt(asm, dst, dst)
	case simpleClosureExprIntConst:
		asm.LoadImm64(dst, expr.value)
	case simpleClosureExprUpval:
		emitLoadClosureUpvalueRef(asm, jit.X0, expr.upval, upvalCount, refScratch, rhs, tagScratch, missLabel)
		asm.LDR(dst, refScratch, 0)
		emitCheckIsIntPinned(asm, dst, tagScratch)
		asm.BCond(jit.CondNE, missLabel)
		jit.EmitUnboxInt(asm, dst, dst)
	case simpleClosureExprAdd, simpleClosureExprMul:
		if expr.left == nil || expr.right == nil {
			asm.B(missLabel)
			return
		}
		emitSimpleClosureExprValue(asm, *expr.left, argSlot, upvalCount, dst, rhs, tagScratch, refScratch, missLabel)
		emitSimpleClosureExprValue(asm, *expr.right, argSlot, upvalCount, rhs, dst, tagScratch, refScratch, missLabel)
		switch expr.kind {
		case simpleClosureExprAdd:
			asm.ADDreg(dst, dst, rhs)
		case simpleClosureExprMul:
			asm.MUL(dst, dst, rhs)
		}
		asm.SBFX(tagScratch, dst, 0, 48)
		asm.CMPreg(tagScratch, dst)
		asm.BCond(jit.CondNE, missLabel)
	default:
		asm.B(missLabel)
	}
}
