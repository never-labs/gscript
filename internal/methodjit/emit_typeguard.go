//go:build darwin && arm64

// emit_typeguard.go: NaN-box type-check guards and guard-op lowering for the
// Method JIT (OpGuardType, OpGuardCalleeProto, OpGuardConstString,
// OpGuardIntRange, OpGuardTruthy and their NaN-box int tag-check helpers).
// Pure code movement from emit_call.go; no behavior change.

package methodjit

import (
	"unsafe"

	"github.com/never-labs/gscript/internal/jit"
)

// emitCheckIsInt emits ARM64 code that checks if a NaN-boxed value in valReg
// is an integer (top 16 bits == 0xFFFE). After this: CondEQ = int, CondNE = not int.
// Uses scratch as temporary register. Also clobbers X3.
func emitCheckIsInt(asm *jit.Assembler, valReg, scratch jit.Reg) {
	asm.LSRimm(scratch, valReg, 48)          // scratch = top 16 bits
	asm.MOVimm16(jit.X3, jit.NB_TagIntShr48) // X3 = 0xFFFE
	asm.CMPreg(scratch, jit.X3)              // EQ = int, NE = not int
}

// emitCheckIsIntPinned checks if a NaN-boxed value is an integer using the
// pinned tag register mRegTagInt (X24 = 0xFFFE000000000000). This avoids the
// MOVimm16 constant load by using a shifted-register comparison instead.
// After this: CondEQ = int, CondNE = not int. Uses scratch as temporary.
func emitCheckIsIntPinned(asm *jit.Assembler, valReg, scratch jit.Reg) {
	asm.LSRimm(scratch, valReg, 48)        // scratch = top 16 bits
	asm.CMPregLSR(scratch, mRegTagInt, 48) // scratch vs (mRegTagInt >> 48) = 0xFFFE
}

func emitCheckIsIntWithTag(asm *jit.Assembler, valReg, scratch, tagReg jit.Reg) {
	asm.LSRimm(scratch, valReg, 48)
	asm.CMPreg(scratch, tagReg)
}

// emitGuardType emits a native type check for OpGuardType.
// On success, passes the value through. On failure, deopts.
func (ec *emitContext) emitGuardType(instr *Instr) {
	if len(instr.Args) == 0 {
		return
	}
	asm := ec.asm

	// R130 layer 3: in numeric pass 2, if the arg is already a raw int
	// (e.g., loaded from a param slot that holds raw int), the
	// GuardType(TypeInt) check is redundant. Pass through: copy raw
	// int to the guard's destination register, mark it raw.
	if ec.numericMode && Type(instr.Aux) == TypeInt {
		argID := instr.Args[0].ID
		if ec.valueReprOf(argID) == valueReprRawInt {
			src := ec.resolveRawInt(argID, jit.X0)
			ec.storeRawInt(src, instr.ID)
			return
		}
	}

	guardType := Type(instr.Aux)
	if guardType == TypeString && instr.Args[0].Def != nil && instr.Args[0].Def.Type == TypeString {
		srcReg := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
		ec.storeResultNB(srcReg, instr.ID)
		return
	}

	// Load the value to check.
	srcReg := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	if srcReg != jit.X0 {
		asm.MOVreg(jit.X0, srcReg)
	}

	switch guardType {
	case TypeInt:
		// Check NaN-box int tag: top 16 bits must be 0xFFFE.
		emitCheckIsIntPinned(asm, jit.X0, jit.X2)
		deoptLabel := ec.uniqueLabel("guard_deopt")
		asm.BCond(jit.CondNE, deoptLabel)
		// Success: store the value as the guard's result.
		ec.storeResultNB(jit.X0, instr.ID)
		doneLabel := ec.uniqueLabel("guard_done")
		asm.B(doneLabel)
		// Deopt path.
		asm.Label(deoptLabel)
		ec.emitDeopt(instr)
		asm.Label(doneLabel)

	case TypeFloat:
		// Float: tag < 0xFFFC (raw IEEE754 bits have no NaN-box tag).
		// Extract top 16 bits and compare against NB_TagNilShr48.
		asm.LSRimm(jit.X2, jit.X0, 48)
		asm.MOVimm16(jit.X3, jit.NB_TagNilShr48) // 0xFFFC
		asm.CMPreg(jit.X2, jit.X3)
		deoptLabel := ec.uniqueLabel("guard_deopt")
		asm.BCond(jit.CondGE, deoptLabel) // tag >= 0xFFFC means non-float → deopt
		asm.FMOVtoFP(jit.D0, jit.X0)
		ec.storeRawFloat(jit.D0, instr.ID)
		doneLabel := ec.uniqueLabel("guard_done")
		asm.B(doneLabel)
		asm.Label(deoptLabel)
		ec.emitDeopt(instr)
		asm.Label(doneLabel)

	case TypeTable:
		deoptLabel := ec.uniqueLabel("guard_deopt")
		jit.EmitCheckIsTableFull(asm, jit.X0, jit.X2, jit.X3, deoptLabel)
		ec.storeResultNB(jit.X0, instr.ID)
		doneLabel := ec.uniqueLabel("guard_done")
		asm.B(doneLabel)
		asm.Label(deoptLabel)
		ec.emitDeopt(instr)
		asm.Label(doneLabel)

	default:
		// Unsupported guard type: just pass through.
		ec.storeResultNB(jit.X0, instr.ID)
	}
}

func (ec *emitContext) emitGuardCalleeProto(instr *Instr) {
	if len(instr.Args) == 0 {
		return
	}
	asm := ec.asm
	srcReg := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	if srcReg != jit.X0 {
		asm.MOVreg(jit.X0, srcReg)
	}
	deoptLabel := ec.uniqueLabel("guard_callee_deopt")
	doneLabel := ec.uniqueLabel("guard_callee_done")

	asm.LSRimm(jit.X1, jit.X0, 48)
	asm.MOVimm16(jit.X2, jit.NB_TagPtrShr48)
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondNE, deoptLabel)
	asm.LSRimm(jit.X1, jit.X0, uint8(nbPtrSubShift))
	asm.LoadImm64(jit.X2, 0xF)
	asm.ANDreg(jit.X1, jit.X1, jit.X2)
	asm.CMPimm(jit.X1, nbPtrSubVMClosure)
	asm.BCond(jit.CondNE, deoptLabel)
	jit.EmitExtractPtr(asm, jit.X1, jit.X0)
	asm.LDR(jit.X1, jit.X1, vmClosureOffProto)
	asm.LoadImm64(jit.X2, instr.Aux)
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondNE, deoptLabel)
	ec.storeResultNBIfUsed(jit.X0, instr.ID)
	asm.B(doneLabel)
	asm.Label(deoptLabel)
	ec.emitDeopt(instr)
	asm.Label(doneLabel)
}

func (ec *emitContext) emitGuardFieldCalleeProto(instr *Instr) {
	if len(instr.Args) == 0 {
		return
	}
	shapeID := uint32(instr.Aux2 >> 32)
	fieldIdx := int(int32(instr.Aux2 & 0xFFFFFFFF))
	if shapeID == 0 || fieldIdx < 0 {
		ec.emitDeopt(instr)
		return
	}
	asm := ec.asm
	deoptLabel := ec.uniqueLabel("guard_field_callee_deopt")
	doneLabel := ec.uniqueLabel("guard_field_callee_done")

	tblValueID := instr.Args[0].ID
	ec.emitPrepareFieldTablePtr(tblValueID, shapeID, deoptLabel)
	asm.LDR(jit.X1, jit.X0, jit.TableOffSvals)
	asm.LDR(jit.X0, jit.X1, fieldIdx*jit.ValueSize)
	ec.rememberFieldSvalsCache(tblValueID, shapeID)

	if exactClosure := ec.guardFieldCalleeExactClosure(instr, shapeID, fieldIdx); exactClosure != 0 {
		asm.LoadImm64(jit.X2, nbClosureTagBits|int64(exactClosure))
		asm.CMPreg(jit.X0, jit.X2)
		asm.BCond(jit.CondNE, deoptLabel)
		ec.storeResultNBIfUsed(jit.X0, instr.ID)
		asm.B(doneLabel)
		asm.Label(deoptLabel)
		ec.emitDeopt(instr)
		asm.Label(doneLabel)
		return
	}

	asm.LSRimm(jit.X1, jit.X0, 48)
	asm.MOVimm16(jit.X2, jit.NB_TagPtrShr48)
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondNE, deoptLabel)
	asm.LSRimm(jit.X1, jit.X0, uint8(nbPtrSubShift))
	asm.LoadImm64(jit.X2, 0xF)
	asm.ANDreg(jit.X1, jit.X1, jit.X2)
	asm.CMPimm(jit.X1, nbPtrSubVMClosure)
	asm.BCond(jit.CondNE, deoptLabel)
	jit.EmitExtractPtr(asm, jit.X1, jit.X0)
	asm.LDR(jit.X1, jit.X1, vmClosureOffProto)
	asm.LoadImm64(jit.X2, instr.Aux)
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondNE, deoptLabel)
	ec.storeResultNBIfUsed(jit.X0, instr.ID)
	asm.B(doneLabel)
	asm.Label(deoptLabel)
	ec.emitDeopt(instr)
	asm.Label(doneLabel)
}

func (ec *emitContext) guardFieldCalleeExactClosure(instr *Instr, shapeID uint32, fieldIdx int) uintptr {
	if ec == nil || ec.fn == nil || instr == nil || shapeID == 0 || fieldIdx < 0 || instr.Aux == 0 {
		return 0
	}
	cases, _ := functionTableShapeFacts(ec.fn).FieldPolyShapeCases(instr.ID)
	if len(cases) != 1 {
		return 0
	}
	c := cases[0]
	if c.ShapeID != shapeID || c.FieldIdx != fieldIdx || c.VMClosure == 0 || c.VMProto == nil {
		return 0
	}
	if uintptr(instr.Aux) != uintptr(unsafe.Pointer(c.VMProto)) {
		return 0
	}
	return c.VMClosure
}

func (ec *emitContext) emitGuardConstString(instr *Instr) {
	if len(instr.Args) == 0 || ec.fn == nil || ec.fn.Proto == nil {
		return
	}
	constIdx := int(instr.Aux)
	if constIdx < 0 || constIdx >= len(ec.fn.Proto.Constants) || !ec.fn.Proto.Constants[constIdx].IsString() {
		ec.emitDeopt(instr)
		return
	}
	asm := ec.asm
	srcReg := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	if srcReg != jit.X0 {
		asm.MOVreg(jit.X0, srcReg)
	}
	deoptLabel := ec.uniqueLabel("guard_const_string_deopt")
	doneLabel := ec.uniqueLabel("guard_const_string_done")
	ec.emitStringValueEqualsConstGuard(jit.X0, ec.fn.Proto.Constants[constIdx].Str(), deoptLabel)
	ec.storeResultNB(jit.X0, instr.ID)
	asm.B(doneLabel)
	asm.Label(deoptLabel)
	ec.emitDeopt(instr)
	asm.Label(doneLabel)
}

// emitGuardIntRange emits a signed raw-int range check for OpGuardIntRange.
// On success it passes the raw int through; on failure it deopts before the
// optimized body observes the speculative range fact.
func (ec *emitContext) emitGuardIntRange(instr *Instr) {
	if len(instr.Args) == 0 {
		return
	}
	asm := ec.asm
	deoptLabel := ec.uniqueLabel("guard_int_range_deopt")
	doneLabel := ec.uniqueLabel("guard_int_range_done")

	argID := instr.Args[0].ID
	if ec.hasReg(argID) && ec.valueReprOf(argID) == valueReprRawInt {
		src := ec.physReg(argID)
		if src != jit.X0 {
			asm.MOVreg(jit.X0, src)
		}
	} else {
		src := ec.resolveValueNB(argID, jit.X0)
		if src != jit.X0 {
			asm.MOVreg(jit.X0, src)
		}
		emitCheckIsIntPinned(asm, jit.X0, jit.X2)
		asm.BCond(jit.CondNE, deoptLabel)
		jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	}
	emitCmpInt64(asm, jit.X0, instr.Aux, jit.X2)
	asm.BCond(jit.CondLT, deoptLabel)
	emitCmpInt64(asm, jit.X0, instr.Aux2, jit.X2)
	asm.BCond(jit.CondGT, deoptLabel)

	ec.storeRawInt(jit.X0, instr.ID)
	asm.B(doneLabel)
	asm.Label(deoptLabel)
	ec.emitDeopt(instr)
	asm.Label(doneLabel)
}

func emitCmpInt64(asm *jit.Assembler, rn jit.Reg, imm int64, scratch jit.Reg) {
	if imm >= 0 && imm <= 0xfff {
		asm.CMPimm(rn, uint16(imm))
		return
	}
	asm.LoadImm64(scratch, imm)
	asm.CMPreg(rn, scratch)
}

// emitGuardTruthy emits ARM64 code for OpGuardTruthy.
// Converts any value to a NaN-boxed bool based on truthiness:
// nil and false are falsy (returns NB_TagBool|0), everything else is truthy
// (returns NB_TagBool|1). This is the non-inverted version of emitNot.
func (ec *emitContext) emitGuardTruthy(instr *Instr) {
	if len(instr.Args) < 1 {
		return
	}
	asm := ec.asm

	// Load operand as NaN-boxed for truthiness check.
	src := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	if src != jit.X0 {
		asm.MOVreg(jit.X0, src)
	}

	// Check for nil: val == NB_ValNil.
	asm.LoadImm64(jit.X1, nb64(jit.NB_ValNil))
	asm.CMPreg(jit.X0, jit.X1)
	isFalsy := ec.uniqueLabel("truthy_falsy")
	asm.BCond(jit.CondEQ, isFalsy)

	// Check for false: val == NB_TagBool|0. Use pinned X25.
	asm.CMPreg(jit.X0, mRegTagBool)
	asm.BCond(jit.CondEQ, isFalsy)

	// Truthy value: return true (NB_TagBool|1).
	asm.ADDimm(jit.X0, mRegTagBool, 1)
	done := ec.uniqueLabel("truthy_done")
	asm.B(done)

	// Nil or false: return false (NB_TagBool|0).
	asm.Label(isFalsy)
	asm.MOVreg(jit.X0, mRegTagBool)

	asm.Label(done)
	// Store NaN-boxed bool result.
	ec.storeResultNB(jit.X0, instr.ID)
}
