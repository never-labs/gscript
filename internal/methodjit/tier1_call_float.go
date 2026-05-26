//go:build darwin && arm64

// tier1_call_float.go holds the Tier 1 baseline float-coercion helpers used by
// the closure fast paths. Split out of tier1_call.go by pure code movement.

package methodjit

import "github.com/gscript/gscript/internal/jit"

func emitFloatValueOrMiss(asm *jit.Assembler, fpReg jit.FReg, gpReg, scratch jit.Reg, missLabel string) {
	jit.EmitIsTaggedPinned(asm, gpReg, scratch, mRegTagInt)
	asm.BCond(jit.CondEQ, missLabel)
	asm.FMOVtoFP(fpReg, gpReg)
}

func emitToFloatNumberOrMiss(asm *jit.Assembler, fpReg jit.FReg, gpReg, scratch jit.Reg, missLabel string) {
	isIntLabel := nextLabel("number_to_float_int")
	doneLabel := nextLabel("number_to_float_done")

	emitCheckIsIntPinned(asm, gpReg, scratch)
	asm.BCond(jit.CondEQ, isIntLabel)
	jit.EmitIsTaggedPinned(asm, gpReg, scratch, mRegTagInt)
	asm.BCond(jit.CondEQ, missLabel)
	asm.FMOVtoFP(fpReg, gpReg)
	asm.B(doneLabel)

	asm.Label(isIntLabel)
	asm.SBFX(scratch, gpReg, 0, 48)
	asm.SCVTF(fpReg, scratch)

	asm.Label(doneLabel)
}
