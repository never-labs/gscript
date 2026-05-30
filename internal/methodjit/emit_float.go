//go:build darwin && arm64

// emit_float.go: float-aware and numeric arithmetic lowering for the Method JIT
// (OpDiv, OpUnm, OpNot, generic int/float binary ops, modulo, FMA/FMSUB, sqrt,
// negate, and the numeric widening/floor helpers). Pure code movement from
// emit_call.go; no behavior change.

package methodjit

import (
	"github.com/Never-Labs/gscript/internal/jit"
)

// emitNumToFloat emits a numeric widening check/conversion. It accepts either
// NaN-boxed int or NaN-boxed float and stores a raw float result for the
// downstream FPR pipeline. Non-numeric values deopt.
func (ec *emitContext) emitNumToFloat(instr *Instr) {
	if len(instr.Args) == 0 {
		return
	}
	asm := ec.asm
	argID := instr.Args[0].ID

	if ec.hasFPReg(argID) {
		ec.storeRawFloat(ec.physFPReg(argID), instr.ID)
		return
	}
	if ec.hasReg(argID) && ec.valueReprOf(argID) == valueReprRawInt {
		asm.SCVTF(jit.D0, ec.physReg(argID))
		ec.storeRawFloat(jit.D0, instr.ID)
		return
	}

	srcReg := ec.resolveValueNB(argID, jit.X0)
	if srcReg != jit.X0 {
		asm.MOVreg(jit.X0, srcReg)
	}

	floatLabel := ec.uniqueLabel("num_to_float_float")
	storeLabel := ec.uniqueLabel("num_to_float_store")
	deoptLabel := ec.uniqueLabel("num_to_float_deopt")
	doneLabel := ec.uniqueLabel("num_to_float_done")

	emitCheckIsIntPinned(asm, jit.X0, jit.X2)
	asm.BCond(jit.CondNE, floatLabel)
	jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	asm.SCVTF(jit.D0, jit.X0)
	asm.B(storeLabel)

	asm.Label(floatLabel)
	asm.LSRimm(jit.X2, jit.X0, 48)
	asm.MOVimm16(jit.X3, jit.NB_TagNilShr48)
	asm.CMPreg(jit.X2, jit.X3)
	asm.BCond(jit.CondGE, deoptLabel)
	asm.FMOVtoFP(jit.D0, jit.X0)

	asm.Label(storeLabel)
	ec.storeRawFloat(jit.D0, instr.ID)
	asm.B(doneLabel)

	asm.Label(deoptLabel)
	ec.emitDeopt(instr)
	asm.Label(doneLabel)
}

// emitFloor emits math.floor(x) as a numeric guard plus native rounding.
// Int inputs pass through; float inputs round toward -inf and convert to int64.
func (ec *emitContext) emitFloor(instr *Instr) {
	if len(instr.Args) == 0 {
		return
	}
	asm := ec.asm
	argID := instr.Args[0].ID

	if ec.hasReg(argID) && ec.valueReprOf(argID) == valueReprRawInt {
		src := ec.physReg(argID)
		if src != jit.X0 {
			asm.MOVreg(jit.X0, src)
		}
		ec.storeRawInt(jit.X0, instr.ID)
		return
	}
	if ec.hasFPReg(argID) {
		asm.FCVTMS(jit.X0, ec.physFPReg(argID))
		ec.storeRawInt(jit.X0, instr.ID)
		return
	}

	srcReg := ec.resolveValueNB(argID, jit.X0)
	if srcReg != jit.X0 {
		asm.MOVreg(jit.X0, srcReg)
	}

	floatLabel := ec.uniqueLabel("floor_float")
	deoptLabel := ec.uniqueLabel("floor_deopt")
	doneLabel := ec.uniqueLabel("floor_done")

	emitCheckIsIntPinned(asm, jit.X0, jit.X2)
	asm.BCond(jit.CondNE, floatLabel)
	jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	ec.storeRawInt(jit.X0, instr.ID)
	asm.B(doneLabel)

	asm.Label(floatLabel)
	asm.LSRimm(jit.X2, jit.X0, 48)
	asm.MOVimm16(jit.X3, jit.NB_TagNilShr48)
	asm.CMPreg(jit.X2, jit.X3)
	asm.BCond(jit.CondGE, deoptLabel)
	asm.FMOVtoFP(jit.D0, jit.X0)
	asm.FCVTMS(jit.X0, jit.D0)
	ec.storeRawInt(jit.X0, instr.ID)
	asm.B(doneLabel)

	asm.Label(deoptLabel)
	ec.emitDeopt(instr)
	asm.Label(doneLabel)
}

// emitDiv emits ARM64 code for OpDiv (a / b, always returns float).
// Both operands may be int or float. Result is always NaN-boxed float.
//
// When the instruction is OpDivFloat with TypeFloat, both operands are known
// to be float, so we use the raw float fast path with no type checks.
func (ec *emitContext) emitDiv(instr *Instr) {
	if len(instr.Args) < 2 {
		return
	}
	asm := ec.asm

	// Fast path: OpDivFloat with TypeFloat — both operands are float, use raw FPR path.
	if instr.Op == OpDivFloat && instr.Type == TypeFloat {
		lhsF := ec.resolveRawFloat(instr.Args[0].ID, jit.D0)
		rhsF := ec.resolveRawFloat(instr.Args[1].ID, jit.D1)
		dstF := ec.rawFloatDst(instr)
		asm.FDIVd(dstF, lhsF, rhsF)
		ec.storeRawFloat(dstF, instr.ID)
		return
	}

	// Generic path: operands may be int or float, with type checks.
	// Load both operands as NaN-boxed values.
	lhsReg := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	if lhsReg != jit.X0 {
		ec.asm.MOVreg(jit.X0, lhsReg)
	}
	rhsReg := ec.resolveValueNB(instr.Args[1].ID, jit.X1)
	if rhsReg != jit.X1 {
		ec.asm.MOVreg(jit.X1, rhsReg)
	}

	// Check if lhs is int.
	emitCheckIsIntPinned(asm, jit.X0, jit.X2)
	lhsNotInt := ec.uniqueLabel("div_lhs_not_int")
	lhsBoth := ec.uniqueLabel("div_both_ready")
	asm.BCond(jit.CondNE, lhsNotInt)

	// LHS is int: unbox, convert to float.
	jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	asm.SCVTF(jit.D0, jit.X0)
	asm.B(lhsBoth)

	// LHS is float: move bits to FP register.
	asm.Label(lhsNotInt)
	asm.FMOVtoFP(jit.D0, jit.X0)

	asm.Label(lhsBoth)

	// Check if rhs is int.
	emitCheckIsIntPinned(asm, jit.X1, jit.X2)
	rhsNotInt := ec.uniqueLabel("div_rhs_not_int")
	rhsBoth := ec.uniqueLabel("div_do_div")
	asm.BCond(jit.CondNE, rhsNotInt)

	// RHS is int: unbox, convert to float.
	jit.EmitUnboxInt(asm, jit.X1, jit.X1)
	asm.SCVTF(jit.D1, jit.X1)
	asm.B(rhsBoth)

	asm.Label(rhsNotInt)
	asm.FMOVtoFP(jit.D1, jit.X1)

	asm.Label(rhsBoth)

	// D0 = lhs, D1 = rhs (both float64). Divide.
	asm.FDIVd(jit.D0, jit.D0, jit.D1)

	// Move result bits back to GP register (float stored as raw IEEE bits).
	asm.FMOVtoGP(jit.X0, jit.D0)

	// Store NaN-boxed float result.
	ec.storeResultNB(jit.X0, instr.ID)
}

// emitUnm emits ARM64 code for OpUnm (-a).
// If the operand is int, uses NEG. If float, uses FNEGd.
func (ec *emitContext) emitUnm(instr *Instr) {
	if len(instr.Args) < 1 {
		return
	}
	asm := ec.asm

	// Load operand as NaN-boxed for type dispatch.
	unmSrc := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	if unmSrc != jit.X0 {
		ec.asm.MOVreg(jit.X0, unmSrc)
	}

	// Check if int.
	emitCheckIsIntPinned(asm, jit.X0, jit.X2)
	notInt := ec.uniqueLabel("unm_not_int")
	done := ec.uniqueLabel("unm_done")
	asm.BCond(jit.CondNE, notInt)

	// Int path: unbox, negate, rebox.
	jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	asm.NEG(jit.X0, jit.X0)
	jit.EmitBoxIntFast(asm, jit.X0, jit.X0, mRegTagInt)
	asm.B(done)

	// Float path: move to FP, negate, move back.
	asm.Label(notInt)
	asm.FMOVtoFP(jit.D0, jit.X0)
	asm.FNEGd(jit.D0, jit.D0)
	asm.FMOVtoGP(jit.X0, jit.D0)

	asm.Label(done)
	// Store NaN-boxed result (int or float).
	ec.storeResultNB(jit.X0, instr.ID)
}

// emitNot emits ARM64 code for OpNot (!a).
// Returns true if the operand is falsy (nil or false), false otherwise.
func (ec *emitContext) emitNot(instr *Instr) {
	if len(instr.Args) < 1 {
		return
	}
	asm := ec.asm

	// Load operand as NaN-boxed for truthiness check.
	notSrc := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	if notSrc != jit.X0 {
		ec.asm.MOVreg(jit.X0, notSrc)
	}

	// Check for nil: val == NB_ValNil (1 instruction: MOVZ with top chunk)
	asm.LoadImm64(jit.X1, nb64(jit.NB_ValNil))
	asm.CMPreg(jit.X0, jit.X1)
	isFalsy := ec.uniqueLabel("not_falsy")
	asm.BCond(jit.CondEQ, isFalsy)

	// Check for false: val == NB_TagBool|0. Use pinned X25 directly.
	asm.CMPreg(jit.X0, mRegTagBool)
	asm.BCond(jit.CondEQ, isFalsy)

	// Truthy value: return false (NB_TagBool|0). Use pinned X25.
	asm.MOVreg(jit.X0, mRegTagBool)
	done := ec.uniqueLabel("not_done")
	asm.B(done)

	// Nil or false: return true (NB_TagBool|1). Compute from pinned X25.
	asm.Label(isFalsy)
	asm.ADDimm(jit.X0, mRegTagBool, 1)

	asm.Label(done)
	// Store NaN-boxed bool result.
	ec.storeResultNB(jit.X0, instr.ID)
}

// emitFloatBinOp emits ARM64 code for type-generic binary arithmetic
// that handles both int and float operands. For int+int, it keeps an int
// result while the value fits the int48 NaN-box payload; otherwise it promotes
// the result to float, matching runtime.Value.SetInt. For any float operand,
// it promotes to float and produces a float result.
func (ec *emitContext) emitFloatBinOp(instr *Instr, op intBinOp) {
	if len(instr.Args) < 2 {
		return
	}
	asm := ec.asm

	if op == intBinMod {
		ec.emitGenericMod(instr)
		return
	}

	if op != intBinMod {
		if imm, constOnLeft, ok := ec.genericIntConstOperand(instr); ok {
			ec.emitFloatBinOpIntConst(instr, op, imm, constOnLeft)
			return
		}
	}

	// Load both operands as NaN-boxed for type dispatch.
	lhsReg := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	if lhsReg != jit.X0 {
		ec.asm.MOVreg(jit.X0, lhsReg)
	}
	rhsReg := ec.resolveValueNB(instr.Args[1].ID, jit.X1)
	if rhsReg != jit.X1 {
		ec.asm.MOVreg(jit.X1, rhsReg)
	}

	done := ec.uniqueLabel("arith_done")
	asm.MOVimm16(jit.X3, jit.NB_TagIntShr48)

	// Check if LHS is int.
	emitCheckIsIntWithTag(asm, jit.X0, jit.X2, jit.X3)
	lhsNotInt := ec.uniqueLabel("arith_lhs_not_int")
	asm.BCond(jit.CondNE, lhsNotInt)

	// LHS is int. Check RHS.
	emitCheckIsIntWithTag(asm, jit.X1, jit.X2, jit.X3)
	rhsNotInt := ec.uniqueLabel("arith_rhs_not_int")
	asm.BCond(jit.CondNE, rhsNotInt)

	// Both are int: fast integer path.
	jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	jit.EmitUnboxInt(asm, jit.X1, jit.X1)
	switch op {
	case intBinAdd:
		asm.ADDreg(jit.X0, jit.X0, jit.X1)
	case intBinSub:
		asm.SUBreg(jit.X0, jit.X0, jit.X1)
	case intBinMul:
		asm.MUL(jit.X0, jit.X0, jit.X1)
	case intBinMod:
		ec.emitIntModX0X1(instr)
	}
	// Int48 overflow in the generic boxed path promotes to float instead of
	// deopting. Raw-int specialized ops still deopt because their loop phis
	// cannot carry a boxed float, but OpAdd/OpSub/OpMul can.
	if op != intBinMod && instr.Aux2 == 0 && !ec.int48Safe(instr.ID) {
		overflow := ec.uniqueLabel("arith_int_overflow")
		asm.SBFX(jit.X2, jit.X0, 0, 48)
		asm.CMPreg(jit.X2, jit.X0)
		asm.BCond(jit.CondNE, overflow)
		jit.EmitBoxIntFast(asm, jit.X0, jit.X0, mRegTagInt)
		asm.B(done)

		asm.Label(overflow)
		asm.SCVTF(jit.D0, jit.X0)
		asm.FMOVtoGP(jit.X0, jit.D0)
		asm.B(done)
	} else {
		jit.EmitBoxIntFast(asm, jit.X0, jit.X0, mRegTagInt)
		asm.B(done)
	}

	// LHS is float (not int).
	asm.Label(lhsNotInt)
	asm.FMOVtoFP(jit.D0, jit.X0) // D0 = lhs as float

	// Check if RHS is int.
	emitCheckIsIntWithTag(asm, jit.X1, jit.X2, jit.X3)
	bothFloat := ec.uniqueLabel("arith_both_float")
	asm.BCond(jit.CondNE, bothFloat)

	// RHS is int, LHS is float: convert RHS to float.
	jit.EmitUnboxInt(asm, jit.X1, jit.X1)
	asm.SCVTF(jit.D1, jit.X1)
	doFloat := ec.uniqueLabel("arith_do_float")
	asm.B(doFloat)

	// RHS is not int while LHS was int: convert LHS to float.
	asm.Label(rhsNotInt)
	jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	asm.SCVTF(jit.D0, jit.X0)
	asm.FMOVtoFP(jit.D1, jit.X1) // D1 = rhs as float
	asm.B(doFloat)

	// Both float.
	asm.Label(bothFloat)
	asm.FMOVtoFP(jit.D1, jit.X1)

	// Float arithmetic.
	asm.Label(doFloat)
	switch op {
	case intBinAdd:
		asm.FADDd(jit.D0, jit.D0, jit.D1)
	case intBinSub:
		asm.FSUBd(jit.D0, jit.D0, jit.D1)
	case intBinMul:
		asm.FMULd(jit.D0, jit.D0, jit.D1)
	case intBinMod:
		emitFloatMod(asm)
	}

	// Move float result back to GP and store.
	asm.FMOVtoGP(jit.X0, jit.D0)

	asm.Label(done)
	// Store NaN-boxed result (int or float).
	ec.storeResultNB(jit.X0, instr.ID)
}

// emitGenericMod lowers a generic % to native int/float modulo with an
// op-exit fallback. The common int/int path is SDIV+MSUB; zero divisors and
// non-numeric operands leave through the normal Tier 2 exit-resume protocol so
// the VM reports the exact runtime behavior.
func (ec *emitContext) emitGenericMod(instr *Instr) {
	if len(instr.Args) < 2 {
		return
	}
	asm := ec.asm

	if divisor, ok := constIntFromValue(instr.Args[1]); ok {
		ec.emitGenericModConstRHS(instr, divisor)
		return
	}

	if instr.Type == TypeFloat {
		lhsF := ec.resolveRawFloat(instr.Args[0].ID, jit.D0)
		rhsF := ec.resolveRawFloat(instr.Args[1].ID, jit.D1)
		done := ec.uniqueLabel("mod_float_done")
		fallback := ec.uniqueLabel("mod_float_fallback")
		if lhsF != jit.D0 {
			asm.FMOVd(jit.D0, lhsF)
		}
		if rhsF != jit.D1 {
			asm.FMOVd(jit.D1, rhsF)
		}
		asm.LoadImm64(jit.X2, 0)
		asm.FMOVtoFP(jit.D2, jit.X2)
		asm.FCMPd(jit.D1, jit.D2)
		asm.BCond(jit.CondEQ, fallback)
		emitFloatMod(asm)
		ec.storeRawFloat(jit.D0, instr.ID)
		asm.B(done)
		asm.Label(fallback)
		ec.emitOpExit(instr)
		asm.Label(done)
		return
	}

	lhsReg := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	if lhsReg != jit.X0 {
		asm.MOVreg(jit.X0, lhsReg)
	}
	rhsReg := ec.resolveValueNB(instr.Args[1].ID, jit.X1)
	if rhsReg != jit.X1 {
		asm.MOVreg(jit.X1, rhsReg)
	}

	done := ec.uniqueLabel("mod_done")
	fallback := ec.uniqueLabel("mod_fallback")
	lhsNotInt := ec.uniqueLabel("mod_lhs_not_int")
	rhsNotInt := ec.uniqueLabel("mod_rhs_not_int")
	doFloat := ec.uniqueLabel("mod_do_float")
	bothFloat := ec.uniqueLabel("mod_both_float")

	asm.MOVimm16(jit.X3, jit.NB_TagIntShr48)
	emitCheckIsIntWithTag(asm, jit.X0, jit.X2, jit.X3)
	asm.BCond(jit.CondNE, lhsNotInt)

	emitCheckIsIntWithTag(asm, jit.X1, jit.X2, jit.X3)
	asm.BCond(jit.CondNE, rhsNotInt)

	jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	jit.EmitUnboxInt(asm, jit.X1, jit.X1)
	asm.CBZ(jit.X1, fallback)
	ec.emitIntModX0X1(instr)
	jit.EmitBoxIntFast(asm, jit.X0, jit.X0, mRegTagInt)
	ec.storeResultNB(jit.X0, instr.ID)
	asm.B(done)

	asm.Label(rhsNotInt)
	jit.EmitIsTaggedPinned(asm, jit.X1, jit.X2, mRegTagInt)
	asm.BCond(jit.CondEQ, fallback)
	jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	asm.SCVTF(jit.D0, jit.X0)
	asm.FMOVtoFP(jit.D1, jit.X1)
	asm.B(doFloat)

	asm.Label(lhsNotInt)
	jit.EmitIsTaggedPinned(asm, jit.X0, jit.X2, mRegTagInt)
	asm.BCond(jit.CondEQ, fallback)
	asm.FMOVtoFP(jit.D0, jit.X0)
	emitCheckIsIntWithTag(asm, jit.X1, jit.X2, jit.X3)
	asm.BCond(jit.CondNE, bothFloat)
	jit.EmitUnboxInt(asm, jit.X1, jit.X1)
	asm.SCVTF(jit.D1, jit.X1)
	asm.B(doFloat)

	asm.Label(bothFloat)
	jit.EmitIsTaggedPinned(asm, jit.X1, jit.X2, mRegTagInt)
	asm.BCond(jit.CondEQ, fallback)
	asm.FMOVtoFP(jit.D1, jit.X1)

	asm.Label(doFloat)
	asm.LoadImm64(jit.X2, 0)
	asm.FMOVtoFP(jit.D2, jit.X2)
	asm.FCMPd(jit.D1, jit.D2)
	asm.BCond(jit.CondEQ, fallback)
	emitFloatMod(asm)
	asm.FMOVtoGP(jit.X0, jit.D0)
	ec.storeResultNB(jit.X0, instr.ID)
	asm.B(done)

	asm.Label(fallback)
	ec.emitOpExit(instr)

	asm.Label(done)
}

func (ec *emitContext) emitGenericModConstRHS(instr *Instr, divisor int64) {
	if len(instr.Args) < 2 || instr.Args[0] == nil {
		return
	}
	if divisor == 0 {
		ec.emitOpExit(instr)
		return
	}
	asm := ec.asm
	lhsReg := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	if lhsReg != jit.X0 {
		asm.MOVreg(jit.X0, lhsReg)
	}

	done := ec.uniqueLabel("mod_const_done")
	floatPath := ec.uniqueLabel("mod_const_float")
	fallback := ec.uniqueLabel("mod_const_fallback")

	asm.MOVimm16(jit.X3, jit.NB_TagIntShr48)
	emitCheckIsIntWithTag(asm, jit.X0, jit.X2, jit.X3)
	asm.BCond(jit.CondNE, floatPath)

	jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	asm.LoadImm64(jit.X1, divisor)
	ec.emitIntModX0X1(instr)
	jit.EmitBoxIntFast(asm, jit.X0, jit.X0, mRegTagInt)
	ec.storeResultNB(jit.X0, instr.ID)
	asm.B(done)

	asm.Label(floatPath)
	jit.EmitIsTaggedPinned(asm, jit.X0, jit.X2, mRegTagInt)
	asm.BCond(jit.CondEQ, fallback)
	asm.FMOVtoFP(jit.D0, jit.X0)
	asm.LoadImm64(jit.X1, divisor)
	asm.SCVTF(jit.D1, jit.X1)
	emitFloatMod(asm)
	asm.FMOVtoGP(jit.X0, jit.D0)
	ec.storeResultNB(jit.X0, instr.ID)
	asm.B(done)

	asm.Label(fallback)
	ec.emitOpExit(instr)

	asm.Label(done)
}

func (ec *emitContext) genericIntConstOperand(instr *Instr) (imm int64, constOnLeft bool, ok bool) {
	if instr == nil || len(instr.Args) < 2 {
		return 0, false, false
	}
	if instr.Args[0] == nil || instr.Args[1] == nil {
		return 0, false, false
	}
	if v, isConst := ec.constInts[instr.Args[0].ID]; isConst {
		return v, true, true
	}
	if v, isConst := ec.constInts[instr.Args[1].ID]; isConst {
		return v, false, true
	}
	return 0, false, false
}

func (ec *emitContext) emitFloatBinOpIntConst(instr *Instr, op intBinOp, imm int64, constOnLeft bool) {
	asm := ec.asm
	var valueArg *Value
	if constOnLeft {
		valueArg = instr.Args[1]
	} else {
		valueArg = instr.Args[0]
	}
	if valueArg == nil {
		return
	}

	valueReg := ec.resolveValueNB(valueArg.ID, jit.X0)
	if valueReg != jit.X0 {
		asm.MOVreg(jit.X0, valueReg)
	}

	done := ec.uniqueLabel("arith_const_done")
	floatPath := ec.uniqueLabel("arith_const_float")
	overflow := ec.uniqueLabel("arith_const_overflow")

	asm.MOVimm16(jit.X3, jit.NB_TagIntShr48)
	emitCheckIsIntWithTag(asm, jit.X0, jit.X2, jit.X3)
	asm.BCond(jit.CondNE, floatPath)

	jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	ec.emitIntConstArithmetic(op, imm, constOnLeft)
	if instr.Aux2 == 0 && !ec.int48Safe(instr.ID) {
		asm.SBFX(jit.X2, jit.X0, 0, 48)
		asm.CMPreg(jit.X2, jit.X0)
		asm.BCond(jit.CondNE, overflow)
	}
	jit.EmitBoxIntFast(asm, jit.X0, jit.X0, mRegTagInt)
	asm.B(done)

	asm.Label(overflow)
	asm.SCVTF(jit.D0, jit.X0)
	asm.FMOVtoGP(jit.X0, jit.D0)
	asm.B(done)

	asm.Label(floatPath)
	asm.FMOVtoFP(jit.D0, jit.X0)
	asm.LoadImm64(jit.X1, imm)
	asm.SCVTF(jit.D1, jit.X1)
	if constOnLeft {
		switch op {
		case intBinAdd:
			asm.FADDd(jit.D0, jit.D1, jit.D0)
		case intBinSub:
			asm.FSUBd(jit.D0, jit.D1, jit.D0)
		case intBinMul:
			asm.FMULd(jit.D0, jit.D1, jit.D0)
		}
	} else {
		switch op {
		case intBinAdd:
			asm.FADDd(jit.D0, jit.D0, jit.D1)
		case intBinSub:
			asm.FSUBd(jit.D0, jit.D0, jit.D1)
		case intBinMul:
			asm.FMULd(jit.D0, jit.D0, jit.D1)
		}
	}
	asm.FMOVtoGP(jit.X0, jit.D0)

	asm.Label(done)
	ec.storeResultNB(jit.X0, instr.ID)
}

func (ec *emitContext) emitIntConstArithmetic(op intBinOp, imm int64, constOnLeft bool) {
	asm := ec.asm
	switch op {
	case intBinAdd:
		if imm >= 0 && imm <= 4095 {
			asm.ADDimm(jit.X0, jit.X0, uint16(imm))
			return
		}
		asm.LoadImm64(jit.X1, imm)
		asm.ADDreg(jit.X0, jit.X0, jit.X1)
	case intBinSub:
		if constOnLeft {
			asm.LoadImm64(jit.X1, imm)
			asm.SUBreg(jit.X0, jit.X1, jit.X0)
			return
		}
		if imm >= 0 && imm <= 4095 {
			asm.SUBimm(jit.X0, jit.X0, uint16(imm))
			return
		}
		asm.LoadImm64(jit.X1, imm)
		asm.SUBreg(jit.X0, jit.X0, jit.X1)
	case intBinMul:
		asm.LoadImm64(jit.X1, imm)
		asm.MUL(jit.X0, jit.X0, jit.X1)
	}
}

// emitFloatMod computes D0 = D0 % D1 using Lua-style modulo semantics:
// a - floor(a / b) * b. Callers must have numeric operands in D0 and D1.
func emitFloatMod(asm *jit.Assembler) {
	asm.FDIVd(jit.D2, jit.D0, jit.D1)
	asm.FRINTMd(jit.D2, jit.D2)
	asm.FMULd(jit.D2, jit.D2, jit.D1)
	asm.FSUBd(jit.D0, jit.D0, jit.D2)
}
