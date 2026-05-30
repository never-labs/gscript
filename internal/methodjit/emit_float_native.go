//go:build darwin && arm64

// emit_float_native.go: type-specialized float operations for the Method JIT
// where the operands are known float (OpAddFloat/OpSubFloat/OpMulFloat,
// OpNegFloat, OpFMA, OpFMSUB, OpSqrt). These skip type checks and operate
// directly on FPRs in raw float mode. Pure code movement from emit_call.go;
// no behavior change.

package methodjit

import (
	"github.com/Never-Labs/gscript/internal/jit"
)

// emitTypedFloatBinOp emits ARM64 code for type-specialized float binary ops
// (OpAddFloat, OpSubFloat, OpMulFloat). Both operands are known to be float,
// so we skip the type check and go straight to FP arithmetic.
//
// Raw float mode: when the result type is TypeFloat and has an FPR allocation,
// operands are resolved as raw floats in FPRs and the result stays in an FPR.
// This avoids FMOVtoFP/FMOVtoGP conversions between every float op.
func (ec *emitContext) emitTypedFloatBinOp(instr *Instr, op intBinOp) {
	if len(instr.Args) < 2 {
		return
	}
	asm := ec.asm

	// Raw float mode: resolve operands into FPRs, compute in FPR, store as raw float.
	if instr.Type == TypeFloat {
		lhsF := ec.resolveRawFloat(instr.Args[0].ID, jit.D0)
		rhsF := ec.resolveRawFloat(instr.Args[1].ID, jit.D1)
		dstF := ec.rawFloatDst(instr)
		switch op {
		case intBinAdd:
			asm.FADDd(dstF, lhsF, rhsF)
		case intBinSub:
			asm.FSUBd(dstF, lhsF, rhsF)
		case intBinMul:
			asm.FMULd(dstF, lhsF, rhsF)
		}
		ec.storeRawFloat(dstF, instr.ID)
		return
	}

	// Fallback: NaN-boxed float ops (original code path for non-TypeFloat).
	lhs := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	asm.FMOVtoFP(jit.D0, lhs)
	rhs := ec.resolveValueNB(instr.Args[1].ID, jit.X1)
	asm.FMOVtoFP(jit.D1, rhs)

	switch op {
	case intBinAdd:
		asm.FADDd(jit.D0, jit.D0, jit.D1)
	case intBinSub:
		asm.FSUBd(jit.D0, jit.D0, jit.D1)
	case intBinMul:
		asm.FMULd(jit.D0, jit.D0, jit.D1)
	}

	// Move float result back to GP (raw IEEE 754 bits = NaN-boxed float).
	asm.FMOVtoGP(jit.X0, jit.D0)
	ec.storeResultNB(jit.X0, instr.ID)
}

// emitNegFloat emits ARM64 code for OpNegFloat (-float).
// The operand is known to be float, so we skip the type check.
// With raw float mode, operates directly on FPRs.
func (ec *emitContext) emitNegFloat(instr *Instr) {
	if len(instr.Args) < 1 {
		return
	}
	asm := ec.asm

	if instr.Type == TypeFloat {
		srcF := ec.resolveRawFloat(instr.Args[0].ID, jit.D0)
		dstF := ec.rawFloatDst(instr)
		asm.FNEGd(dstF, srcF)
		ec.storeRawFloat(dstF, instr.ID)
		return
	}

	// Fallback: NaN-boxed path.
	src := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	asm.FMOVtoFP(jit.D0, src)
	asm.FNEGd(jit.D0, jit.D0)
	asm.FMOVtoGP(jit.X0, jit.D0)
	ec.storeResultNB(jit.X0, instr.ID)
}

// emitFMA emits ARM64 code for OpFMA(a, b, acc) → acc + a*b, using a
// single FMADDd instruction. Args: [a, b, acc], all TypeFloat in raw-
// FPR mode (ensured by FMAFusionPass running after TypeSpecialize).
func (ec *emitContext) emitFMA(instr *Instr) {
	if len(instr.Args) < 3 {
		return
	}
	asm := ec.asm
	if instr.Type == TypeFloat {
		aF := ec.resolveRawFloat(instr.Args[0].ID, jit.D0)
		bF := ec.resolveRawFloat(instr.Args[1].ID, jit.D1)
		cF := ec.resolveRawFloat(instr.Args[2].ID, jit.D2)
		dstF := ec.rawFloatDst(instr)
		// FMADDd: Dd = Da + Dn * Dm  (a + n*m in assembler naming;
		// our helper is FMADDd(rd, rn, rm, ra).)
		asm.FMADDd(dstF, aF, bF, cF)
		ec.storeRawFloat(dstF, instr.ID)
		return
	}
	// NaN-boxed fallback: unlikely but safe.
	aNB := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	asm.FMOVtoFP(jit.D0, aNB)
	bNB := ec.resolveValueNB(instr.Args[1].ID, jit.X1)
	asm.FMOVtoFP(jit.D1, bNB)
	cNB := ec.resolveValueNB(instr.Args[2].ID, jit.X2)
	asm.FMOVtoFP(jit.D2, cNB)
	asm.FMADDd(jit.D0, jit.D0, jit.D1, jit.D2)
	asm.FMOVtoGP(jit.X0, jit.D0)
	ec.storeResultNB(jit.X0, instr.ID)
}

// emitFMSUB emits ARM64 code for OpFMSUB(a, b, acc) → acc - a*b, using a
// single FMSUBd instruction. Args: [a, b, acc], all TypeFloat in raw-FPR mode.
func (ec *emitContext) emitFMSUB(instr *Instr) {
	if len(instr.Args) < 3 {
		return
	}
	asm := ec.asm
	if instr.Type == TypeFloat {
		aF := ec.resolveRawFloat(instr.Args[0].ID, jit.D0)
		bF := ec.resolveRawFloat(instr.Args[1].ID, jit.D1)
		cF := ec.resolveRawFloat(instr.Args[2].ID, jit.D2)
		dstF := ec.rawFloatDst(instr)
		asm.FMSUBd(dstF, aF, bF, cF)
		ec.storeRawFloat(dstF, instr.ID)
		return
	}
	aNB := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	asm.FMOVtoFP(jit.D0, aNB)
	bNB := ec.resolveValueNB(instr.Args[1].ID, jit.X1)
	asm.FMOVtoFP(jit.D1, bNB)
	cNB := ec.resolveValueNB(instr.Args[2].ID, jit.X2)
	asm.FMOVtoFP(jit.D2, cNB)
	asm.FMSUBd(jit.D0, jit.D0, jit.D1, jit.D2)
	asm.FMOVtoGP(jit.X0, jit.D0)
	ec.storeResultNB(jit.X0, instr.ID)
}

// emitSqrtFloat emits ARM64 code for OpSqrt (sqrt(float)).
// The operand is known to be float, so we skip the type check and use FSQRT
// directly on an FPR. With raw float mode, operates entirely in FPRs.
func (ec *emitContext) emitSqrtFloat(instr *Instr) {
	if len(instr.Args) < 1 {
		return
	}
	asm := ec.asm

	if instr.Type == TypeFloat {
		srcF := ec.resolveRawFloat(instr.Args[0].ID, jit.D0)
		dstF := ec.rawFloatDst(instr)
		asm.FSQRTd(dstF, srcF)
		ec.storeRawFloat(dstF, instr.ID)
		return
	}

	// Fallback: NaN-boxed path (operand float bits interpreted as double).
	src := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	asm.FMOVtoFP(jit.D0, src)
	asm.FSQRTd(jit.D0, jit.D0)
	asm.FMOVtoGP(jit.X0, jit.D0)
	ec.storeResultNB(jit.X0, instr.ID)
}
