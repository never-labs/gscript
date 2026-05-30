//go:build darwin && arm64

// tier1_table.go emits ARM64 templates for baseline length, self, and
// upvalue operations, plus the shared type-feedback observe helpers used by
// the Tier 1 table/field native paths. These are the highest-value native
// ops after arithmetic and control flow.
//
// Strategy:
//   - LEN: for tables, load array length directly. Falls back for strings
//     and metatables.
//   - SELF: R(A+1) = R(B); R(A) = R(B)[RK(C)] using GETTABLE logic.
//   - GETUPVAL/SETUPVAL: load/store through the closure's upvalue refs.

package methodjit

import (
	"github.com/Never-Labs/gscript/internal/jit"
	"github.com/Never-Labs/gscript/internal/vm"
)

// emitBaselineLen emits ARM64 for OP_LEN: R(A) = #R(B).
// String length is a fixed header load and can stay native. Table length still
// falls back because mixed/bool arrays require the runtime's trailing-nil scan
// and tables may define __len.
func emitBaselineLen(asm *jit.Assembler, inst uint32, pc int) {
	a := vm.DecodeA(inst)
	b := vm.DecodeB(inst)

	slowLabel := nextLabel("len_slow")
	doneLabel := nextLabel("len_done")

	loadSlot(asm, jit.X0, b)
	jit.EmitCheckIsString(asm, jit.X0, jit.X1, jit.X2, slowLabel)
	jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	asm.LDR(jit.X1, jit.X0, 8) // Go string header length.
	jit.EmitBoxIntFast(asm, jit.X0, jit.X1, mRegTagInt)
	storeSlot(asm, a, jit.X0)
	emitBaselineFeedbackResult(asm, pc, 1, "len_string")
	asm.B(doneLabel)

	asm.Label(slowLabel)
	emitBaselineOpExitCommon(asm, vm.OP_LEN, pc, a, b, 0)
	asm.Label(doneLabel)
}

// emitBaselineSelf emits native ARM64 for OP_SELF: R(A+1) = R(B); R(A) = R(B)[RK(C)]
// This is R(A+1) = obj, R(A) = obj.method -- used for method calls.
func emitBaselineSelf(asm *jit.Assembler, inst uint32, pc int) {
	a := vm.DecodeA(inst)
	b := vm.DecodeB(inst)
	cidx := vm.DecodeC(inst)

	slowLabel := nextLabel("self_slow")
	doneLabel := nextLabel("self_done")
	polyMissLabel := nextLabel("self_poly_miss")

	// R(A+1) = R(B) (copy the object reference).
	loadSlot(asm, jit.X0, b)
	asm.STR(jit.X0, mRegRegs, slotOff(a+1))

	// Now do table lookup: R(A) = R(B)[RK(C)]
	// X0 already has the table value.

	// Check table pointer.
	jit.EmitCheckIsTableFull(asm, jit.X0, jit.X1, jit.X2, slowLabel)
	jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	asm.CBZ(jit.X0, slowLabel)

	// Load key RK(C).
	loadRK(asm, jit.X1, cidx) // X1 = key

	// Check if key is a string (most common case for method names).
	// For SELF, the key is typically a constant string.
	// We use the generic RawGet path: check if key is string, do skeys scan.
	// For simplicity, check if it's a string and do linear scan of skeys.
	jit.EmitCheckIsString(asm, jit.X1, jit.X2, jit.X3, slowLabel)

	// It's a string. Extract string pointer from NaN-box.
	// Actually, RawGet dispatches on type. For string keys, it calls RawGetString.
	// The string comparison is complex in JIT. Let's use the FieldCache instead
	// if available, or fall back to slow path.
	//
	// For method calls, the key is always a constant. We can use FieldCache[pc].
	asm.LDR(jit.X2, mRegCtx, execCtxOffBaselineFieldCache)
	asm.CBZ(jit.X2, slowLabel) // no field cache

	// Compute &FieldCache[pc].
	if pc > 0 {
		entryOff := pc * jit.FieldCacheEntrySize
		if entryOff < 4096 {
			asm.ADDimm(jit.X2, jit.X2, uint16(entryOff))
		} else {
			asm.LoadImm64(jit.X3, int64(entryOff))
			asm.ADDreg(jit.X2, jit.X2, jit.X3)
		}
	}

	// Load entry.ShapeID.
	asm.LDRW(jit.X3, jit.X2, jit.FieldCacheEntryOffShapeID)
	asm.CBZ(jit.X3, slowLabel)

	// Shape guard.
	asm.LDRW(jit.X4, jit.X0, jit.TableOffShapeID)
	asm.CMPreg(jit.X4, jit.X3)
	asm.BCond(jit.CondNE, polyMissLabel)

	// Load FieldIdx.
	asm.LDR(jit.X3, jit.X2, jit.FieldCacheEntryOffFieldIdx)

	// Bounds check.
	asm.LDR(jit.X4, jit.X0, jit.TableOffSvalsLen)
	asm.CMPreg(jit.X3, jit.X4)
	asm.BCond(jit.CondGE, slowLabel)

	// Direct access: svals[fieldIdx].
	// LDRreg uses [Xn + Xm, LSL #3] which already scales by 8 (= ValueSize),
	// so X3 must hold the raw fieldIdx (not pre-multiplied).
	asm.LDR(jit.X4, jit.X0, jit.TableOffSvals)
	asm.LDRreg(jit.X0, jit.X4, jit.X3)

	// Store result to R(A).
	storeSlot(asm, a, jit.X0)
	asm.B(doneLabel)

	asm.Label(polyMissLabel)
	asm.CBZ(jit.X4, slowLabel)
	emitBaselineFieldPolyLookup(asm, pc, a, jit.X0, jit.X4, false, "self_poly", slowLabel, doneLabel)

	// Slow path: exit-resume.
	asm.Label(slowLabel)
	emitBaselineOpExitCommon(asm, vm.OP_SELF, pc, a, b, cidx)

	asm.Label(doneLabel)
}

// emitBaselineGetUpval emits native ARM64 for OP_GETUPVAL: R(A) = Upvalues[B].ref
// Uses the Closure pointer stored in ExecContext.
func emitBaselineGetUpval(asm *jit.Assembler, inst uint32, pc int, proto *vm.FuncProto) {
	a := vm.DecodeA(inst)
	b := vm.DecodeB(inst)

	slowLabel := nextLabel("getupval_slow")
	doneLabel := nextLabel("getupval_done")

	// Load Closure pointer from ExecContext.
	asm.LDR(jit.X0, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.CBZ(jit.X0, slowLabel) // no closure pointer

	if proto != nil && len(proto.Upvalues) == 1 && b == 0 {
		// One-upvalue closures keep Upvalues[0] inline in the closure object.
		asm.LDR(jit.X2, jit.X0, vmClosureOffInlineUpvalue0)
	} else {
		// Closure.Upvalues is a []*Upvalue slice at offset 8.
		// Load slice data pointer.
		asm.LDR(jit.X1, jit.X0, 8) // X1 = Upvalues data ptr ([]* element ptr)

		// Load Upvalue pointer: Upvalues[B] (each element is 8 bytes = *Upvalue).
		asm.LDR(jit.X2, jit.X1, b*8) // X2 = *Upvalue
	}

	asm.CBZ(jit.X2, slowLabel)

	// Upvalue.ref is at offset 0 (*runtime.Value pointer).
	asm.LDR(jit.X3, jit.X2, 0) // X3 = ref ptr
	asm.CBZ(jit.X3, slowLabel)

	// Load the value: *ref.
	asm.LDR(jit.X0, jit.X3, 0) // X0 = *ref (the actual value)

	// Store to R(A).
	storeSlot(asm, a, jit.X0)
	asm.B(doneLabel)

	// Slow path: exit-resume.
	asm.Label(slowLabel)
	emitBaselineOpExitCommon(asm, vm.OP_GETUPVAL, pc, a, b, 0)

	asm.Label(doneLabel)
}

// emitBaselineSetUpval emits native ARM64 for OP_SETUPVAL: Upvalues[B].ref = R(A)
func emitBaselineSetUpval(asm *jit.Assembler, inst uint32, pc int, proto *vm.FuncProto) {
	a := vm.DecodeA(inst)
	b := vm.DecodeB(inst)

	slowLabel := nextLabel("setupval_slow")
	doneLabel := nextLabel("setupval_done")

	// Load Closure pointer from ExecContext.
	asm.LDR(jit.X0, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.CBZ(jit.X0, slowLabel)

	if proto != nil && len(proto.Upvalues) == 1 && b == 0 {
		// One-upvalue closures keep Upvalues[0] inline in the closure object.
		asm.LDR(jit.X2, jit.X0, vmClosureOffInlineUpvalue0)
	} else {
		// Load Upvalues slice data pointer.
		asm.LDR(jit.X1, jit.X0, 8) // Closure.Upvalues data ptr

		// Load Upvalue[B] pointer.
		asm.LDR(jit.X2, jit.X1, b*8) // *Upvalue
	}
	asm.CBZ(jit.X2, slowLabel)

	// Upvalue.ref at offset 0.
	asm.LDR(jit.X3, jit.X2, 0) // ref ptr
	asm.CBZ(jit.X3, slowLabel)

	// Load value from R(A).
	loadSlot(asm, jit.X4, a)

	// Store: *ref = value.
	asm.STR(jit.X4, jit.X3, 0)

	asm.B(doneLabel)

	// Slow path: exit-resume.
	asm.Label(slowLabel)
	emitBaselineOpExitCommon(asm, vm.OP_SETUPVAL, pc, a, b, 0)

	asm.Label(doneLabel)
}

// emitBaselineFeedbackResult emits ARM64 code to record the Result type
// in the TypeFeedback[pc] entry. expectedFB is the FeedbackType constant
// (e.g., FBFloat=2, FBInt=1, FBBool=4). This implements the monotonic
// Observe logic: Unobserved→concrete, concrete→concrete (same=nop, diff→Any), Any→Any.
func emitBaselineFeedbackResult(asm *jit.Assembler, pc int, expectedFB uint16, suffix string) {
	emitBaselineFeedbackFixedAt(asm, pc, 2, expectedFB, suffix)
}

// emitBaselineFeedbackFixedAt is the generalized form — R85 Option 2.
// Writes expectedFB to the (Left=0|Right=1|Result=2) field of TypeFeedback[pc]
// with monotonic-observe semantics. Used when the operand type is statically
// known (e.g., IntSpec variants of OP_EQ/LT/LE where both operands are
// known-int at compile time).
func emitBaselineFeedbackFixedAt(asm *jit.Assembler, pc int, fieldOff int, expectedFB uint16, suffix string) {
	fbSkipLabel := nextLabel("fb_skip_" + suffix)
	fbSetLabel := nextLabel("fb_set_" + suffix)

	asm.LDR(jit.X5, mRegCtx, execCtxOffBaselineFeedbackPtr)
	asm.CBZ(jit.X5, fbSkipLabel)

	fbOff := pc*4 + fieldOff
	if fbOff < 4096 {
		asm.LDRB(jit.X6, jit.X5, fbOff)
		asm.CMPimm(jit.X6, expectedFB)
		asm.BCond(jit.CondEQ, fbSkipLabel)
		asm.CMPimm(jit.X6, 7) // FBAny
		asm.BCond(jit.CondEQ, fbSkipLabel)
		asm.CBZ(jit.X6, fbSetLabel)
		// Different type → FBAny
		asm.MOVimm16(jit.X6, 7)
		asm.STRB(jit.X6, jit.X5, fbOff)
		asm.B(fbSkipLabel)
		asm.Label(fbSetLabel)
		asm.MOVimm16(jit.X6, expectedFB)
		asm.STRB(jit.X6, jit.X5, fbOff)
	} else {
		asm.LoadImm64(jit.X6, int64(fbOff))
		asm.ADDreg(jit.X5, jit.X5, jit.X6)
		asm.LDRB(jit.X6, jit.X5, 0)
		asm.CMPimm(jit.X6, expectedFB)
		asm.BCond(jit.CondEQ, fbSkipLabel)
		asm.CMPimm(jit.X6, 7) // FBAny
		asm.BCond(jit.CondEQ, fbSkipLabel)
		asm.CBZ(jit.X6, fbSetLabel)
		asm.MOVimm16(jit.X6, 7)
		asm.STRB(jit.X6, jit.X5, 0)
		asm.B(fbSkipLabel)
		asm.Label(fbSetLabel)
		asm.MOVimm16(jit.X6, expectedFB)
		asm.STRB(jit.X6, jit.X5, 0)
	}
	asm.Label(fbSkipLabel)
}

// emitBaselineFeedbackDenseMatrix records whether a GETTABLE receiver had a
// DenseMatrix descriptor. It mirrors TableKeyFeedback.ObserveDenseMatrix using
// only scratch registers X5-X7 and leaves tblPtrReg untouched.
func emitBaselineFeedbackDenseMatrix(asm *jit.Assembler, pc int, tblPtrReg jit.Reg, suffix string) {
	fbSkipLabel := nextLabel("fb_dm_skip_" + suffix)
	fbSetLabel := nextLabel("fb_dm_set_" + suffix)
	fbObservedLabel := nextLabel("fb_dm_observed_" + suffix)

	asm.LDR(jit.X5, mRegCtx, execCtxOffBaselineTableKeyFeedbackPtr)
	asm.CBZ(jit.X5, fbSkipLabel)

	asm.MOVimm16(jit.X7, uint16(vm.FBDenseMatrixNo))
	asm.LDRW(jit.X6, tblPtrReg, jit.TableOffDMStride)
	asm.CBZ(jit.X6, fbObservedLabel)
	asm.MOVimm16(jit.X7, uint16(vm.FBDenseMatrixYes))
	asm.Label(fbObservedLabel)

	fbOff := pc*tableKeyFeedbackSize + tableKeyFeedbackDenseMatrixOff
	if fbOff < 4096 {
		asm.LDRB(jit.X6, jit.X5, fbOff)
		asm.CMPreg(jit.X6, jit.X7)
		asm.BCond(jit.CondEQ, fbSkipLabel)
		asm.CMPimm(jit.X6, uint16(vm.FBDenseMatrixPolymorphic))
		asm.BCond(jit.CondEQ, fbSkipLabel)
		asm.CBZ(jit.X6, fbSetLabel)
		asm.MOVimm16(jit.X6, uint16(vm.FBDenseMatrixPolymorphic))
		asm.STRB(jit.X6, jit.X5, fbOff)
		asm.B(fbSkipLabel)
		asm.Label(fbSetLabel)
		asm.STRB(jit.X7, jit.X5, fbOff)
	} else {
		asm.LoadImm64(jit.X6, int64(fbOff))
		asm.ADDreg(jit.X5, jit.X5, jit.X6)
		asm.LDRB(jit.X6, jit.X5, 0)
		asm.CMPreg(jit.X6, jit.X7)
		asm.BCond(jit.CondEQ, fbSkipLabel)
		asm.CMPimm(jit.X6, uint16(vm.FBDenseMatrixPolymorphic))
		asm.BCond(jit.CondEQ, fbSkipLabel)
		asm.CBZ(jit.X6, fbSetLabel)
		asm.MOVimm16(jit.X6, uint16(vm.FBDenseMatrixPolymorphic))
		asm.STRB(jit.X6, jit.X5, 0)
		asm.B(fbSkipLabel)
		asm.Label(fbSetLabel)
		asm.STRB(jit.X7, jit.X5, 0)
	}
	asm.Label(fbSkipLabel)
}

// emitBaselineFeedbackResultFromValue emits ARM64 code to extract the type from
// a NaN-boxed value and record it as Result feedback for TypeFeedback[pc]. The
// value must be in valReg. It distinguishes float, int, and table; all other
// types map to FBAny. Table feedback is important for mixed table-of-tables
// array access where the outer array stores row tables.
//
// Uses scratch registers X5, X6, X7. Does not clobber valReg.
func emitBaselineFeedbackResultFromValue(asm *jit.Assembler, pc int, valReg jit.Reg, suffix string) {
	emitBaselineFeedbackFromValueAt(asm, pc, valReg, 2, suffix)
}

// emitBaselineFeedbackFromValueAt is the generalized form of the above — R85
// Option 2. fieldOff selects the byte within TypeFeedback[pc] to update:
//
//	0 = Left, 1 = Right, 2 = Result, 3 = Kind.
//
// Left/Right are consumed by OP_EQ/LT/LE; Result by GETFIELD-style ops. The
// monotonic-observe semantics are identical across fields — Unobserved→concrete,
// same→nop, different→FBAny, FBAny→nop.
func emitBaselineFeedbackFromValueAt(asm *jit.Assembler, pc int, valReg jit.Reg, fieldOff int, suffix string) {
	fbSkipLabel := nextLabel("fb_val_skip_" + suffix)
	fbFloatLabel := nextLabel("fb_val_float_" + suffix)
	fbIntLabel := nextLabel("fb_val_int_" + suffix)
	fbPtrLabel := nextLabel("fb_val_ptr_" + suffix)
	fbTableLabel := nextLabel("fb_val_table_" + suffix)
	fbSetLabel := nextLabel("fb_val_set_" + suffix)
	fbUpdateLabel := nextLabel("fb_val_update_" + suffix)

	// Load feedback pointer.
	asm.LDR(jit.X5, mRegCtx, execCtxOffBaselineFeedbackPtr)
	asm.CBZ(jit.X5, fbSkipLabel)

	// Extract type from NaN-boxed value.
	// Tag = top 16 bits. Float: tag < 0xFFFC. Int: tag == 0xFFFE.
	// Pointers need the subtype check to distinguish table from string/function.
	asm.LSRimm(jit.X7, valReg, 48) // X7 = tag
	asm.MOVimm16(jit.X6, 0xFFFC)   // NB_TagNilShr48
	asm.CMPreg(jit.X7, jit.X6)
	asm.BCond(jit.CondLT, fbFloatLabel) // tag < 0xFFFC → float
	asm.MOVimm16(jit.X6, 0xFFFE)        // NB_TagIntShr48
	asm.CMPreg(jit.X7, jit.X6)
	asm.BCond(jit.CondEQ, fbIntLabel) // tag == 0xFFFE → int
	asm.MOVimm16(jit.X6, 0xFFFF)      // NB_TagPtrShr48
	asm.CMPreg(jit.X7, jit.X6)
	asm.BCond(jit.CondEQ, fbPtrLabel) // ptr → maybe table
	// Everything else (bool, nil) → FBAny.
	asm.MOVimm16(jit.X7, 7) // FBAny
	asm.B(fbUpdateLabel)
	asm.Label(fbPtrLabel)
	asm.LSRimm(jit.X6, valReg, uint8(jit.NB_PtrSubShift))
	asm.LoadImm64(jit.X7, 0xF)
	asm.ANDreg(jit.X6, jit.X6, jit.X7)
	asm.CMPimm(jit.X6, 0) // ptrSubTable
	asm.BCond(jit.CondEQ, fbTableLabel)
	asm.MOVimm16(jit.X7, 7) // non-table pointer → FBAny
	asm.B(fbUpdateLabel)
	asm.Label(fbFloatLabel)
	asm.MOVimm16(jit.X7, 2) // FBFloat
	asm.B(fbUpdateLabel)
	asm.Label(fbIntLabel)
	asm.MOVimm16(jit.X7, 1) // FBInt
	asm.B(fbUpdateLabel)
	asm.Label(fbTableLabel)
	asm.MOVimm16(jit.X7, 5) // FBTable

	// Monotonic update: X7 = observed type.
	asm.Label(fbUpdateLabel)
	fbOff := pc*4 + fieldOff // TypeFeedback[pc] byte offset
	if fbOff < 4096 {
		asm.LDRB(jit.X6, jit.X5, fbOff) // X6 = current value
		asm.CMPreg(jit.X6, jit.X7)
		asm.BCond(jit.CondEQ, fbSkipLabel) // same type → skip
		asm.CMPimm(jit.X6, 7)              // FBAny?
		asm.BCond(jit.CondEQ, fbSkipLabel) // already megamorphic → skip
		asm.CBZ(jit.X6, fbSetLabel)        // Unobserved → set
		// Different type → FBAny
		asm.MOVimm16(jit.X6, 7)
		asm.STRB(jit.X6, jit.X5, fbOff)
		asm.B(fbSkipLabel)
		asm.Label(fbSetLabel)
		asm.STRB(jit.X7, jit.X5, fbOff) // store observed type
	} else {
		asm.LoadImm64(jit.X6, int64(fbOff))
		asm.ADDreg(jit.X5, jit.X5, jit.X6)
		asm.LDRB(jit.X6, jit.X5, 0)
		asm.CMPreg(jit.X6, jit.X7)
		asm.BCond(jit.CondEQ, fbSkipLabel)
		asm.CMPimm(jit.X6, 7)
		asm.BCond(jit.CondEQ, fbSkipLabel)
		asm.CBZ(jit.X6, fbSetLabel)
		asm.MOVimm16(jit.X6, 7)
		asm.STRB(jit.X6, jit.X5, 0)
		asm.B(fbSkipLabel)
		asm.Label(fbSetLabel)
		asm.STRB(jit.X7, jit.X5, 0)
	}
	asm.Label(fbSkipLabel)
}

// emitBaselineFeedbackKind emits ARM64 code to record the array kind
// in the TypeFeedback[pc].Kind field. Uses monotonic observe logic:
// Unobserved->concrete kind, same->nop, different->Polymorphic, Polymorphic->nop.
// expectedKind is the FBKind* constant (1=Mixed, 2=Int, 3=Float, 4=Bool).
func emitBaselineFeedbackKind(asm *jit.Assembler, pc int, expectedKind uint16, suffix string) {
	fbSkipLabel := nextLabel("fbk_skip_" + suffix)
	fbSetLabel := nextLabel("fbk_set_" + suffix)

	asm.LDR(jit.X5, mRegCtx, execCtxOffBaselineFeedbackPtr)
	asm.CBZ(jit.X5, fbSkipLabel)

	fbKindOff := pc*4 + 3 // TypeFeedback[pc].Kind offset
	if fbKindOff < 4096 {
		asm.LDRB(jit.X6, jit.X5, fbKindOff)
		asm.CMPimm(jit.X6, expectedKind)
		asm.BCond(jit.CondEQ, fbSkipLabel) // same kind -> skip
		asm.CMPimm(jit.X6, 0xFF)           // FBKindPolymorphic
		asm.BCond(jit.CondEQ, fbSkipLabel) // already poly -> skip
		asm.CBZ(jit.X6, fbSetLabel)        // unobserved -> set
		// different kind -> polymorphic
		asm.MOVimm16(jit.X6, 0xFF)
		asm.STRB(jit.X6, jit.X5, fbKindOff)
		asm.B(fbSkipLabel)
		asm.Label(fbSetLabel)
		asm.MOVimm16(jit.X6, expectedKind)
		asm.STRB(jit.X6, jit.X5, fbKindOff)
	} else {
		asm.LoadImm64(jit.X6, int64(fbKindOff))
		asm.ADDreg(jit.X5, jit.X5, jit.X6)
		asm.LDRB(jit.X6, jit.X5, 0)
		asm.CMPimm(jit.X6, expectedKind)
		asm.BCond(jit.CondEQ, fbSkipLabel)
		asm.CMPimm(jit.X6, 0xFF)
		asm.BCond(jit.CondEQ, fbSkipLabel)
		asm.CBZ(jit.X6, fbSetLabel)
		asm.MOVimm16(jit.X6, 0xFF)
		asm.STRB(jit.X6, jit.X5, 0)
		asm.B(fbSkipLabel)
		asm.Label(fbSetLabel)
		asm.MOVimm16(jit.X6, expectedKind)
		asm.STRB(jit.X6, jit.X5, 0)
	}
	asm.Label(fbSkipLabel)
}
