//go:build darwin && arm64

// tier1_table_key_feedback.go emits ARM64 templates that mirror the
// TableKeyFeedback observe machinery for Tier 1 native dynamic string-key
// cache hits: shape/field/key polymorphism tracking and the monotonic
// key/value-type observers. Pure code movement out of tier1_table.go.

package methodjit

import (
	"github.com/never-labs/gscript/internal/jit"
	"github.com/never-labs/gscript/internal/vm"
)

// emitBaselineTableStringKeyCacheHitFeedback mirrors the stable-fact portion of
// TableKeyFeedback.ObserveTableAccess for Tier 1 native dynamic string cache
// hits. Inputs are preserved for the caller:
//
//	X5 = key data pointer, X6 = key length, X7 = table shape id, X11 = field idx.
func emitBaselineTableStringKeyCacheHitFeedback(asm *jit.Assembler, pc int, accessKind uint8, valueReg jit.Reg, suffix string) {
	skipLabel := nextLabel("tkf_hit_skip_" + suffix)
	shapeSetLabel := nextLabel("tkf_hit_shape_set_" + suffix)
	fieldSetLabel := nextLabel("tkf_hit_field_set_" + suffix)
	fieldSeenLabel := nextLabel("tkf_hit_field_seen_" + suffix)
	keySeenLabel := nextLabel("tkf_hit_key_seen_" + suffix)
	keyCompareLoopLabel := nextLabel("tkf_hit_key_cmp_loop_" + suffix)
	keyPolyLabel := nextLabel("tkf_hit_key_poly_" + suffix)

	asm.LDR(jit.X12, mRegCtx, execCtxOffBaselineTableKeyFeedbackPtr)
	asm.CBZ(jit.X12, skipLabel)
	entryOff := pc * tableKeyFeedbackSize
	if entryOff > 0 {
		if entryOff <= 4095 {
			asm.ADDimm(jit.X12, jit.X12, uint16(entryOff))
		} else {
			asm.LoadImm64(jit.X13, int64(entryOff))
			asm.ADDreg(jit.X12, jit.X12, jit.X13)
		}
	}

	asm.LDRB(jit.X13, jit.X12, tableKeyFeedbackStringKeySeenOff)
	asm.CBZ(jit.X13, skipLabel)

	asm.LDR(jit.X13, jit.X12, tableKeyFeedbackStringKeyOff)
	asm.LDR(jit.X14, jit.X12, tableKeyFeedbackStringKeyOff+8)
	asm.CMPreg(jit.X14, jit.X6)
	asm.BCond(jit.CondNE, keyPolyLabel)
	asm.CMPreg(jit.X13, jit.X5)
	asm.BCond(jit.CondEQ, keySeenLabel)
	asm.CBZ(jit.X14, keySeenLabel)
	asm.MOVimm16(jit.X15, 0)
	asm.Label(keyCompareLoopLabel)
	asm.LDRBreg(jit.X16, jit.X13, jit.X15)
	asm.LDRBreg(jit.X17, jit.X5, jit.X15)
	asm.CMPreg(jit.X16, jit.X17)
	asm.BCond(jit.CondNE, keyPolyLabel)
	asm.ADDimm(jit.X15, jit.X15, 1)
	asm.CMPreg(jit.X15, jit.X14)
	asm.BCond(jit.CondLT, keyCompareLoopLabel)
	asm.B(keySeenLabel)
	asm.Label(keyPolyLabel)
	emitBaselineTableKeyFeedbackOrFlag(asm, jit.X12, vm.TableAccessKeyPolymorphic)
	asm.B(skipLabel)
	asm.Label(keySeenLabel)

	asm.LDRW(jit.X13, jit.X12, tableKeyFeedbackCountOff)
	asm.ADDimm(jit.X13, jit.X13, 1)
	asm.STRW(jit.X13, jit.X12, tableKeyFeedbackCountOff)

	emitBaselineTableKeyFeedbackObserveByte(asm, jit.X12, tableKeyFeedbackKeyTypeOff, uint8(vm.FBString), suffix+"_key")
	emitBaselineTableKeyFeedbackObserveValueType(asm, jit.X12, tableKeyFeedbackValueTypeOff, valueReg, suffix+"_value")
	emitBaselineTableKeyFeedbackMergeAccessKind(asm, jit.X12, accessKind)

	asm.LDRW(jit.X13, jit.X12, tableKeyFeedbackShapeIDOff)
	asm.CMPreg(jit.X13, jit.X7)
	asm.BCond(jit.CondEQ, fieldSeenLabel)
	asm.CBZ(jit.X13, shapeSetLabel)
	emitBaselineTableKeyFeedbackOrFlag(asm, jit.X12, vm.TableAccessShapePolymorphic)
	asm.B(fieldSeenLabel)
	asm.Label(shapeSetLabel)
	asm.STRW(jit.X7, jit.X12, tableKeyFeedbackShapeIDOff)

	asm.Label(fieldSeenLabel)
	asm.LDRB(jit.X13, jit.X12, tableKeyFeedbackFieldIdxSeenOff)
	asm.CBZ(jit.X13, fieldSetLabel)
	asm.LDR(jit.X13, jit.X12, tableKeyFeedbackFieldIdxOff)
	asm.CMPreg(jit.X13, jit.X11)
	asm.BCond(jit.CondEQ, skipLabel)
	emitBaselineTableKeyFeedbackOrFlag(asm, jit.X12, vm.TableAccessFieldPolymorphic)
	asm.B(skipLabel)
	asm.Label(fieldSetLabel)
	asm.STR(jit.X11, jit.X12, tableKeyFeedbackFieldIdxOff)
	asm.MOVimm16(jit.X13, 1)
	asm.STRB(jit.X13, jit.X12, tableKeyFeedbackFieldIdxSeenOff)

	asm.Label(skipLabel)
}

func emitBaselineTableKeyFeedbackMergeAccessKind(asm *jit.Assembler, base jit.Reg, accessKind uint8) {
	doneLabel := nextLabel("tkf_access_done")
	setLabel := nextLabel("tkf_access_set")
	asm.LDRB(jit.X13, base, tableKeyFeedbackAccessKindOff)
	asm.CMPimm(jit.X13, uint16(accessKind))
	asm.BCond(jit.CondEQ, doneLabel)
	asm.CBZ(jit.X13, setLabel)
	asm.MOVimm16(jit.X14, uint16(accessKind))
	asm.ORRreg(jit.X13, jit.X13, jit.X14)
	asm.STRB(jit.X13, base, tableKeyFeedbackAccessKindOff)
	asm.B(doneLabel)
	asm.Label(setLabel)
	asm.MOVimm16(jit.X13, uint16(accessKind))
	asm.STRB(jit.X13, base, tableKeyFeedbackAccessKindOff)
	asm.Label(doneLabel)
}

func emitBaselineTableKeyFeedbackOrFlag(asm *jit.Assembler, base jit.Reg, flag uint16) {
	asm.LDRB(jit.X15, base, tableKeyFeedbackFlagsOff)
	asm.MOVimm16(jit.X16, flag)
	asm.ORRreg(jit.X15, jit.X15, jit.X16)
	asm.STRB(jit.X15, base, tableKeyFeedbackFlagsOff)
}

func emitBaselineTableKeyFeedbackObserveByte(asm *jit.Assembler, base jit.Reg, off int, observed uint8, suffix string) {
	doneLabel := nextLabel("tkf_byte_done_" + suffix)
	setLabel := nextLabel("tkf_byte_set_" + suffix)
	asm.LDRB(jit.X13, base, off)
	asm.CMPimm(jit.X13, uint16(observed))
	asm.BCond(jit.CondEQ, doneLabel)
	asm.CMPimm(jit.X13, uint16(vm.FBAny))
	asm.BCond(jit.CondEQ, doneLabel)
	asm.CBZ(jit.X13, setLabel)
	asm.MOVimm16(jit.X13, uint16(vm.FBAny))
	asm.STRB(jit.X13, base, off)
	asm.B(doneLabel)
	asm.Label(setLabel)
	asm.MOVimm16(jit.X13, uint16(observed))
	asm.STRB(jit.X13, base, off)
	asm.Label(doneLabel)
}

func emitBaselineTableKeyFeedbackObserveValueType(asm *jit.Assembler, base jit.Reg, off int, valReg jit.Reg, suffix string) {
	floatLabel := nextLabel("tkf_val_float_" + suffix)
	intLabel := nextLabel("tkf_val_int_" + suffix)
	boolLabel := nextLabel("tkf_val_bool_" + suffix)
	ptrLabel := nextLabel("tkf_val_ptr_" + suffix)
	stringLabel := nextLabel("tkf_val_string_" + suffix)
	tableLabel := nextLabel("tkf_val_table_" + suffix)
	functionLabel := nextLabel("tkf_val_function_" + suffix)
	updateLabel := nextLabel("tkf_val_update_" + suffix)

	asm.LSRimm(jit.X14, valReg, 48)
	asm.MOVimm16(jit.X15, jit.NB_TagNilShr48)
	asm.CMPreg(jit.X14, jit.X15)
	asm.BCond(jit.CondLT, floatLabel)
	asm.MOVimm16(jit.X15, jit.NB_TagIntShr48)
	asm.CMPreg(jit.X14, jit.X15)
	asm.BCond(jit.CondEQ, intLabel)
	asm.MOVimm16(jit.X15, jit.NB_TagBoolShr48)
	asm.CMPreg(jit.X14, jit.X15)
	asm.BCond(jit.CondEQ, boolLabel)
	asm.MOVimm16(jit.X15, jit.NB_TagPtrShr48)
	asm.CMPreg(jit.X14, jit.X15)
	asm.BCond(jit.CondEQ, ptrLabel)
	asm.MOVimm16(jit.X13, uint16(vm.FBAny))
	asm.B(updateLabel)

	asm.Label(ptrLabel)
	asm.LSRimm(jit.X14, valReg, uint8(jit.NB_PtrSubShift))
	asm.LoadImm64(jit.X15, 0xF)
	asm.ANDreg(jit.X14, jit.X14, jit.X15)
	asm.CMPimm(jit.X14, 0)
	asm.BCond(jit.CondEQ, tableLabel)
	asm.CMPimm(jit.X14, 1)
	asm.BCond(jit.CondEQ, stringLabel)
	asm.CMPimm(jit.X14, 9)
	asm.BCond(jit.CondEQ, stringLabel)
	asm.CMPimm(jit.X14, 2)
	asm.BCond(jit.CondEQ, functionLabel)
	asm.CMPimm(jit.X14, 3)
	asm.BCond(jit.CondEQ, functionLabel)
	asm.CMPimm(jit.X14, 6)
	asm.BCond(jit.CondEQ, functionLabel)
	asm.CMPimm(jit.X14, 8)
	asm.BCond(jit.CondEQ, functionLabel)
	asm.MOVimm16(jit.X13, uint16(vm.FBAny))
	asm.B(updateLabel)

	asm.Label(floatLabel)
	asm.MOVimm16(jit.X13, uint16(vm.FBFloat))
	asm.B(updateLabel)
	asm.Label(intLabel)
	asm.MOVimm16(jit.X13, uint16(vm.FBInt))
	asm.B(updateLabel)
	asm.Label(boolLabel)
	asm.MOVimm16(jit.X13, uint16(vm.FBBool))
	asm.B(updateLabel)
	asm.Label(stringLabel)
	asm.MOVimm16(jit.X13, uint16(vm.FBString))
	asm.B(updateLabel)
	asm.Label(tableLabel)
	asm.MOVimm16(jit.X13, uint16(vm.FBTable))
	asm.B(updateLabel)
	asm.Label(functionLabel)
	asm.MOVimm16(jit.X13, uint16(vm.FBFunction))

	asm.Label(updateLabel)
	emitBaselineTableKeyFeedbackObserveByteReg(asm, base, off, jit.X13, suffix)
}

func emitBaselineTableKeyFeedbackObserveByteReg(asm *jit.Assembler, base jit.Reg, off int, observed jit.Reg, suffix string) {
	doneLabel := nextLabel("tkf_byter_done_" + suffix)
	setLabel := nextLabel("tkf_byter_set_" + suffix)
	asm.LDRB(jit.X14, base, off)
	asm.CMPreg(jit.X14, observed)
	asm.BCond(jit.CondEQ, doneLabel)
	asm.CMPimm(jit.X14, uint16(vm.FBAny))
	asm.BCond(jit.CondEQ, doneLabel)
	asm.CBZ(jit.X14, setLabel)
	asm.MOVimm16(jit.X14, uint16(vm.FBAny))
	asm.STRB(jit.X14, base, off)
	asm.B(doneLabel)
	asm.Label(setLabel)
	asm.STRB(observed, base, off)
	asm.Label(doneLabel)
}
