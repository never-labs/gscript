//go:build darwin && arm64

// emit_compare_ops.go: comparison-op lowering for the Method JIT (float
// compares, generic numeric compares with int/float/string fast paths, eq-nil
// fast path, and the native string compare/eq byte loops). Pure code movement
// from emit_call.go; no behavior change.

package methodjit

import (
	"github.com/never-labs/gscript/internal/jit"
)

// emitFloatCmp emits ARM64 code for float comparison (OpLtFloat, OpLeFloat).
// Uses FCMP on FP registers instead of integer CMP, since NaN-boxed floats
// are raw IEEE 754 bits and integer comparison doesn't handle sign/exponent
// ordering correctly for floats.
//
// With raw float mode, resolves operands from FPRs directly when available,
// avoiding the FMOVtoFP conversion from GPR.
func (ec *emitContext) emitFloatCmp(instr *Instr, cond jit.Cond) {
	if len(instr.Args) < 2 {
		return
	}
	asm := ec.asm

	// Resolve both operands as raw floats in FPRs.
	lhsF := ec.resolveRawFloat(instr.Args[0].ID, jit.D0)
	rhsF := ec.resolveRawFloat(instr.Args[1].ID, jit.D1)

	// Float compare sets NZCV flags.
	asm.FCMPd(lhsF, rhsF)

	// Fused path: preceding FCMP already set flags; the following Branch
	// will emit B.cc directly. Skip bool materialization (saves 3 insns).
	if ec.fusedCmps[instr.ID] {
		ec.fusedCond = cond
		ec.fusedActive = true
		return
	}

	// Normal path: materialize NaN-boxed bool.
	// Set result: 1 if condition true, 0 if false.
	asm.CSET(jit.X0, cond)

	// Box as bool: NB_TagBool | (0 or 1).
	asm.ORRreg(jit.X0, jit.X0, mRegTagBool)

	// Store NaN-boxed bool result (comparison result is always bool, not float).
	ec.storeResultNB(jit.X0, instr.ID)
}

// emitGenericNumericCmp emits comparison for generic numeric values that may be
// int or float after overflow boxing. Raw int-int comparisons stay integer;
// mixed int/float comparisons convert the int side to float. For EQ, identical
// NaN-boxed bit patterns are accepted first so nil/bool/pointer identity keeps
// the old fast behavior for generic Eq sites.
func (ec *emitContext) emitGenericNumericCmp(instr *Instr, cond jit.Cond) {
	if len(instr.Args) < 2 {
		return
	}
	if cond == jit.CondEQ && ec.emitEqNilFastPath(instr) {
		return
	}
	asm := ec.asm

	lhsReg := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	if lhsReg != jit.X0 {
		asm.MOVreg(jit.X0, lhsReg)
	}
	rhsReg := ec.resolveValueNB(instr.Args[1].ID, jit.X1)
	if rhsReg != jit.X1 {
		asm.MOVreg(jit.X1, rhsReg)
	}

	trueLabel := ec.uniqueLabel("cmp_true")
	falseLabel := ec.uniqueLabel("cmp_false")
	doneLabel := ec.uniqueLabel("cmp_done")
	fallbackLabel := ec.uniqueLabel("cmp_fallback")
	fastDoneLabel := ec.uniqueLabel("cmp_fast_done")

	if cond == jit.CondEQ {
		asm.CMPreg(jit.X0, jit.X1)
		asm.BCond(jit.CondEQ, trueLabel)
		asm.LoadImm64(jit.X2, nb64(jit.NB_ValNil))
		asm.CMPreg(jit.X0, jit.X2)
		asm.BCond(jit.CondEQ, falseLabel)
		asm.CMPreg(jit.X1, jit.X2)
		asm.BCond(jit.CondEQ, falseLabel)
	}

	emitCheckIsIntPinned(asm, jit.X0, jit.X2)
	lhsNotInt := ec.uniqueLabel("cmp_lhs_not_int")
	asm.BCond(jit.CondNE, lhsNotInt)

	emitCheckIsIntPinned(asm, jit.X1, jit.X2)
	lhsIntRhsNotInt := ec.uniqueLabel("cmp_lhs_int_rhs_not_int")
	asm.BCond(jit.CondNE, lhsIntRhsNotInt)

	if cond == jit.CondEQ {
		asm.B(falseLabel)
	} else {
		jit.EmitUnboxInt(asm, jit.X0, jit.X0)
		jit.EmitUnboxInt(asm, jit.X1, jit.X1)
		asm.CMPreg(jit.X0, jit.X1)
		asm.BCond(cond, trueLabel)
		asm.B(falseLabel)
	}

	asm.Label(lhsIntRhsNotInt)
	jit.EmitIsTaggedPinned(asm, jit.X1, jit.X2, mRegTagInt)
	asm.BCond(jit.CondEQ, fallbackLabel)
	jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	asm.SCVTF(jit.D0, jit.X0)
	asm.FMOVtoFP(jit.D1, jit.X1)
	asm.FCMPd(jit.D0, jit.D1)
	asm.BCond(cond, trueLabel)
	asm.B(falseLabel)

	asm.Label(lhsNotInt)
	jit.EmitIsTaggedPinned(asm, jit.X0, jit.X2, mRegTagInt)
	lhsTaggedLabel := ec.uniqueLabel("cmp_lhs_tagged")
	asm.BCond(jit.CondEQ, lhsTaggedLabel)
	asm.FMOVtoFP(jit.D0, jit.X0)
	emitCheckIsIntPinned(asm, jit.X1, jit.X2)
	bothNotInt := ec.uniqueLabel("cmp_both_not_int")
	asm.BCond(jit.CondNE, bothNotInt)

	jit.EmitUnboxInt(asm, jit.X1, jit.X1)
	asm.SCVTF(jit.D1, jit.X1)
	asm.FCMPd(jit.D0, jit.D1)
	asm.BCond(cond, trueLabel)
	asm.B(falseLabel)

	asm.Label(bothNotInt)
	jit.EmitIsTaggedPinned(asm, jit.X1, jit.X2, mRegTagInt)
	asm.BCond(jit.CondEQ, fallbackLabel)
	asm.FMOVtoFP(jit.D1, jit.X1)
	asm.FCMPd(jit.D0, jit.D1)
	asm.BCond(cond, trueLabel)
	asm.B(falseLabel)

	asm.Label(lhsTaggedLabel)
	if cond == jit.CondEQ {
		jit.EmitCheckIsString(asm, jit.X0, jit.X2, jit.X3, fallbackLabel)
		jit.EmitCheckIsString(asm, jit.X1, jit.X2, jit.X3, fallbackLabel)
		ec.emitStringEqFast(trueLabel, falseLabel)
	} else {
		jit.EmitCheckIsString(asm, jit.X0, jit.X2, jit.X3, fallbackLabel)
		jit.EmitCheckIsString(asm, jit.X1, jit.X2, jit.X3, fallbackLabel)
		ec.emitStringCmpFast(cond, trueLabel, falseLabel)
	}

	asm.Label(trueLabel)
	asm.ADDimm(jit.X0, mRegTagBool, 1)
	asm.B(doneLabel)

	asm.Label(falseLabel)
	asm.MOVreg(jit.X0, mRegTagBool)

	asm.Label(doneLabel)
	ec.storeResultNB(jit.X0, instr.ID)
	asm.B(fastDoneLabel)

	asm.Label(fallbackLabel)
	ec.emitOpExit(instr)

	asm.Label(fastDoneLabel)
}

func (ec *emitContext) emitEqNilFastPath(instr *Instr) bool {
	lhsNil := valueIsConstNil(instr.Args[0])
	rhsNil := valueIsConstNil(instr.Args[1])
	if !lhsNil && !rhsNil {
		return false
	}

	asm := ec.asm
	if lhsNil && rhsNil {
		asm.MOVreg(jit.X0, mRegTagBool)
		asm.ADDimm(jit.X0, jit.X0, 1)
		ec.storeResultNB(jit.X0, instr.ID)
		return true
	}

	arg := instr.Args[0]
	if lhsNil {
		arg = instr.Args[1]
	}
	valReg := ec.resolveValueNB(arg.ID, jit.X0)
	if valReg != jit.X0 {
		asm.MOVreg(jit.X0, valReg)
	}
	asm.LoadImm64(jit.X1, nb64(jit.NB_ValNil))
	asm.CMPreg(jit.X0, jit.X1)

	if ec.fusedCmps[instr.ID] {
		ec.fusedCond = jit.CondEQ
		ec.fusedActive = true
		return true
	}

	asm.CSET(jit.X0, jit.CondEQ)
	asm.ORRreg(jit.X0, jit.X0, mRegTagBool)
	ec.storeResultNB(jit.X0, instr.ID)
	return true
}

func valueIsConstNil(v *Value) bool {
	return v != nil && v.Def != nil && v.Def.Op == OpConstNil
}

// emitStringCmpFast compares two NaN-boxed string values in X0 and X1.
// Both operands must already be checked as strings. The runtime represents
// strings as tagged pointers to Go string headers, so the native path can load
// data/len and do the same byte-wise lexicographic comparison as Go strings.
func (ec *emitContext) emitStringCmpFast(cond jit.Cond, trueLabel, falseLabel string) {
	asm := ec.asm

	loopLabel := ec.uniqueLabel("str_cmp_loop")
	prefixLabel := ec.uniqueLabel("str_cmp_prefix")

	// Strip NaN-boxing tag/subtype bits and recover *string pointers.
	asm.LSLimm(jit.X2, jit.X0, 20)
	asm.LSRimm(jit.X2, jit.X2, 20)
	asm.LSLimm(jit.X3, jit.X1, 20)
	asm.LSRimm(jit.X3, jit.X3, 20)

	// Go string header: data pointer at +0, length at +8.
	asm.LDR(jit.X4, jit.X2, 0) // lhs data
	asm.LDR(jit.X5, jit.X2, 8) // lhs len
	asm.LDR(jit.X6, jit.X3, 0) // rhs data
	asm.LDR(jit.X7, jit.X3, 8) // rhs len

	asm.MOVimm16(jit.X8, 0) // byte index

	asm.Label(loopLabel)
	asm.CMPreg(jit.X8, jit.X5)
	asm.BCond(jit.CondHS, prefixLabel)
	asm.CMPreg(jit.X8, jit.X7)
	asm.BCond(jit.CondHS, prefixLabel)

	asm.LDRBreg(jit.X9, jit.X4, jit.X8)
	asm.LDRBreg(jit.X10, jit.X6, jit.X8)
	asm.CMPreg(jit.X9, jit.X10)
	asm.BCond(jit.CondLO, trueLabel)
	asm.BCond(jit.CondHI, falseLabel)
	asm.ADDimm(jit.X8, jit.X8, 1)
	asm.B(loopLabel)

	asm.Label(prefixLabel)
	asm.CMPreg(jit.X5, jit.X7)
	if cond == jit.CondLE {
		asm.BCond(jit.CondLS, trueLabel)
	} else {
		asm.BCond(jit.CondLO, trueLabel)
	}
	asm.B(falseLabel)
}

// emitStringEqFast compares two NaN-boxed string values in X0 and X1.
// Both operands must already be checked as strings. It checks length first,
// then compares backing bytes only when distinct equal-length strings are
// observed.
func (ec *emitContext) emitStringEqFast(trueLabel, falseLabel string) {
	asm := ec.asm

	loopLabel := ec.uniqueLabel("str_eq_loop")

	asm.LSLimm(jit.X2, jit.X0, 20)
	asm.LSRimm(jit.X2, jit.X2, 20)
	asm.LSLimm(jit.X3, jit.X1, 20)
	asm.LSRimm(jit.X3, jit.X3, 20)
	asm.CMPreg(jit.X2, jit.X3)
	asm.BCond(jit.CondEQ, trueLabel)

	asm.LDR(jit.X4, jit.X2, 0) // lhs data
	asm.LDR(jit.X5, jit.X2, 8) // lhs len
	asm.LDR(jit.X6, jit.X3, 0) // rhs data
	asm.LDR(jit.X7, jit.X3, 8) // rhs len
	asm.CMPreg(jit.X5, jit.X7)
	asm.BCond(jit.CondNE, falseLabel)
	asm.CBZ(jit.X5, trueLabel)
	asm.CMPreg(jit.X4, jit.X6)
	asm.BCond(jit.CondEQ, trueLabel)

	asm.MOVimm16(jit.X8, 0)
	asm.Label(loopLabel)
	asm.LDRBreg(jit.X9, jit.X4, jit.X8)
	asm.LDRBreg(jit.X10, jit.X6, jit.X8)
	asm.CMPreg(jit.X9, jit.X10)
	asm.BCond(jit.CondNE, falseLabel)
	asm.ADDimm(jit.X8, jit.X8, 1)
	asm.CMPreg(jit.X8, jit.X5)
	asm.BCond(jit.CondLT, loopLabel)
	asm.B(trueLabel)
}

func (ec *emitContext) emitStringEqCmp(instr *Instr) {
	if len(instr.Args) < 2 {
		return
	}
	asm := ec.asm
	lhsReg := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	if lhsReg != jit.X0 {
		asm.MOVreg(jit.X0, lhsReg)
	}
	rhsReg := ec.resolveValueNB(instr.Args[1].ID, jit.X1)
	if rhsReg != jit.X1 {
		asm.MOVreg(jit.X1, rhsReg)
	}

	trueLabel := ec.uniqueLabel("str_eq_true")
	falseLabel := ec.uniqueLabel("str_eq_false")
	doneLabel := ec.uniqueLabel("str_eq_done")
	ec.emitStringEqFast(trueLabel, falseLabel)

	asm.Label(trueLabel)
	asm.ADDimm(jit.X0, mRegTagBool, 1)
	asm.B(doneLabel)

	asm.Label(falseLabel)
	asm.MOVreg(jit.X0, mRegTagBool)

	asm.Label(doneLabel)
	ec.storeResultNB(jit.X0, instr.ID)
}

func (ec *emitContext) emitTypedStringCmp(instr *Instr, cond jit.Cond) bool {
	if instr == nil || len(instr.Args) < 2 {
		return false
	}
	if valueStaticType(instr.Args[0]) != TypeString || valueStaticType(instr.Args[1]) != TypeString {
		return false
	}
	asm := ec.asm
	lhsReg := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	if lhsReg != jit.X0 {
		asm.MOVreg(jit.X0, lhsReg)
	}
	rhsReg := ec.resolveValueNB(instr.Args[1].ID, jit.X1)
	if rhsReg != jit.X1 {
		asm.MOVreg(jit.X1, rhsReg)
	}

	trueLabel := ec.uniqueLabel("str_cmp_true")
	falseLabel := ec.uniqueLabel("str_cmp_false")
	doneLabel := ec.uniqueLabel("str_cmp_done")
	ec.emitStringCmpFast(cond, trueLabel, falseLabel)

	asm.Label(trueLabel)
	asm.ADDimm(jit.X0, mRegTagBool, 1)
	asm.B(doneLabel)

	asm.Label(falseLabel)
	asm.MOVreg(jit.X0, mRegTagBool)

	asm.Label(doneLabel)
	ec.storeResultNB(jit.X0, instr.ID)
	return true
}

func valueStaticType(v *Value) Type {
	if v == nil || v.Def == nil {
		return TypeUnknown
	}
	return v.Def.Type
}
