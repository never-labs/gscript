//go:build darwin && arm64

// tier1_call_accumulator.go holds the Tier 1 baseline accumulator-closure
// fast-path emitters and the accumulator int-result store helper.
// Split out of tier1_call.go by pure code movement.

package methodjit

import (
	"unsafe"

	"github.com/gscript/gscript/internal/jit"
	"github.com/gscript/gscript/internal/vm"
)

func emitBaselineAccumulatorClosureFastPath(asm *jit.Assembler, callerProto *vm.FuncProto, slowLabel, doneLabel string, dstSlot int, fillCallIC bool, callICOff int) {
	emitBaselineAccumulatorClosureFastPathMatchingProto(asm, callerProto, slowLabel, doneLabel, dstSlot, fillCallIC, callICOff, false)
}

func emitBaselineAccumulatorClosureFastPathFromBoxedHit(asm *jit.Assembler, callerProto *vm.FuncProto, slowLabel, doneLabel string, dstSlot int) {
	emitBaselineAccumulatorClosureFastPathMatchingProto(asm, callerProto, slowLabel, doneLabel, dstSlot, false, 0, true)
}

func emitBaselineAccumulatorClosureFastPathMatchingProto(asm *jit.Assembler, callerProto *vm.FuncProto, slowLabel, doneLabel string, dstSlot int, fillCallIC bool, callICOff int, extractBoxedClosure bool) {
	fastPaths := accumulatorClosureFastPathsForProto(callerProto)
	if len(fastPaths) == 0 {
		return
	}
	if extractBoxedClosure && len(fastPaths) == 1 {
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
		emitBaselineAccumulatorClosureFastPathBody(asm, fastPaths[0], slowLabel, doneLabel, dstSlot)
		return
	}
	missLabel := nextLabel("accum_closure_miss")
	for _, fast := range fastPaths {
		nextFastLabel := nextLabel("accum_closure_next")
		asm.LoadImm64(jit.X3, int64(uintptr(unsafe.Pointer(fast.proto))))
		asm.CMPreg(jit.X1, jit.X3)
		asm.BCond(jit.CondNE, nextFastLabel)
		if extractBoxedClosure {
			jit.EmitExtractPtr(asm, jit.X0, jit.X0)
		}
		if fillCallIC {
			asm.LDR(jit.X5, mRegCtx, execCtxOffBaselineCallCache)
			asm.MOVimm16(jit.X7, 0)
			asm.STP(jit.X4, jit.X7, jit.X5, callICOff+baselineCallCacheBoxedOff)
			asm.STP(jit.X1, jit.X7, jit.X5, callICOff+baselineCallCacheProtoOff)
		}

		emitBaselineAccumulatorClosureFastPathBody(asm, fast, missLabel, doneLabel, dstSlot)

		asm.Label(nextFastLabel)
	}
	asm.Label(missLabel)
}

func emitBaselineAccumulatorClosureFastPathBody(asm *jit.Assembler, fast accumulatorClosureFastPath, missLabel, doneLabel string, dstSlot int) {
	switch fast.deltaKind {
	case accumulatorDeltaConst:
		emitLoadClosureUpvalueRef(asm, jit.X0, fast.valueUpval, len(fast.proto.Upvalues), jit.X6, jit.X2, jit.X3, missLabel)
		asm.LDR(jit.X4, jit.X6, 0) // boxed current value
		floatLabel := nextLabel("accum_closure_float")
		emitCheckIsIntPinned(asm, jit.X4, jit.X5)
		asm.BCond(jit.CondNE, floatLabel)
		jit.EmitUnboxInt(asm, jit.X4, jit.X4)
		if fast.delta >= 0 && fast.delta <= 4095 {
			asm.ADDimm(jit.X4, jit.X4, uint16(fast.delta))
		} else if fast.delta < 0 && fast.delta >= -4095 {
			asm.SUBimm(jit.X4, jit.X4, uint16(-fast.delta))
		} else {
			asm.LoadImm64(jit.X5, fast.delta)
			asm.ADDreg(jit.X4, jit.X4, jit.X5)
		}
		emitStoreAccumulatorIntResult(asm, dstSlot, doneLabel, jit.X6)
		asm.Label(floatLabel)
		emitFloatValueOrMiss(asm, jit.D0, jit.X4, jit.X5, missLabel)
		asm.LoadImm64(jit.X5, fast.delta)
		asm.SCVTF(jit.D1, jit.X5)
		asm.FADDd(jit.D0, jit.D0, jit.D1)
		asm.FMOVtoGP(jit.X4, jit.D0)
		asm.STR(jit.X4, jit.X6, 0)
		storeSlot(asm, dstSlot, jit.X4)
		asm.B(doneLabel)
	case accumulatorDeltaUpval:
		emitLoadClosureTwoUpvalueRefs(asm, jit.X0,
			fast.valueUpval, fast.deltaUpval, len(fast.proto.Upvalues),
			jit.X6, jit.X2, jit.X5, jit.X3, jit.X7, missLabel)
		asm.LDR(jit.X4, jit.X6, 0) // boxed current value
		asm.LDR(jit.X7, jit.X2, 0) // boxed delta value
		floatLabel := nextLabel("accum_closure_float")
		mixedFloatLabel := nextLabel("accum_closure_mixed_float")
		asm.MOVimm16(jit.X3, jit.NB_TagIntShr48)
		asm.LSRimm(jit.X5, jit.X4, 48)
		asm.CMPreg(jit.X5, jit.X3)
		asm.BCond(jit.CondNE, floatLabel)
		asm.LSRimm(jit.X5, jit.X7, 48)
		asm.CMPreg(jit.X5, jit.X3)
		asm.BCond(jit.CondNE, mixedFloatLabel)
		jit.EmitUnboxInt(asm, jit.X4, jit.X4)
		jit.EmitUnboxInt(asm, jit.X7, jit.X7)
		asm.ADDreg(jit.X4, jit.X4, jit.X7)
		emitStoreAccumulatorIntResult(asm, dstSlot, doneLabel, jit.X6)
		asm.Label(mixedFloatLabel)
		asm.SBFX(jit.X5, jit.X4, 0, 48)
		asm.SCVTF(jit.D0, jit.X5)
		emitFloatValueOrMiss(asm, jit.D1, jit.X7, jit.X5, missLabel)
		asm.FADDd(jit.D0, jit.D0, jit.D1)
		asm.FMOVtoGP(jit.X4, jit.D0)
		asm.STR(jit.X4, jit.X6, 0)
		storeSlot(asm, dstSlot, jit.X4)
		asm.B(doneLabel)
		asm.Label(floatLabel)
		emitFloatValueOrMiss(asm, jit.D0, jit.X4, jit.X5, missLabel)
		emitToFloatNumberOrMiss(asm, jit.D1, jit.X7, jit.X5, missLabel)
		asm.FADDd(jit.D0, jit.D0, jit.D1)
		asm.FMOVtoGP(jit.X4, jit.D0)
		asm.STR(jit.X4, jit.X6, 0)
		storeSlot(asm, dstSlot, jit.X4)
		asm.B(doneLabel)
	default:
		asm.B(missLabel)
	}
}

func emitStoreAccumulatorIntResult(asm *jit.Assembler, dstSlot int, doneLabel string, valueRefReg jit.Reg) {
	overflowLabel := nextLabel("accum_closure_int_overflow")
	asm.SBFX(jit.X5, jit.X4, 0, 48)
	asm.CMPreg(jit.X5, jit.X4)
	asm.BCond(jit.CondNE, overflowLabel)
	jit.EmitBoxIntFast(asm, jit.X4, jit.X4, mRegTagInt)
	asm.STR(jit.X4, valueRefReg, 0)
	storeSlot(asm, dstSlot, jit.X4)
	asm.B(doneLabel)

	asm.Label(overflowLabel)
	asm.SCVTF(jit.D0, jit.X4)
	asm.FMOVtoGP(jit.X4, jit.D0)
	asm.STR(jit.X4, valueRefReg, 0)
	storeSlot(asm, dstSlot, jit.X4)
	asm.B(doneLabel)
}
