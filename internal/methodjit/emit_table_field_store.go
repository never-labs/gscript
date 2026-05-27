//go:build darwin && arm64

// emit_table_field_store.go: table field store/write code paths (OpSetField,
// OpFieldStore, store type guards) and the table-exit fallback for stores.
// Pure code movement from emit_table_field.go; no behavior change.

package methodjit

import (
	"fmt"

	"github.com/gscript/gscript/internal/jit"
	"github.com/gscript/gscript/internal/runtime"
)

func (ec *emitContext) emitFieldStore(instr *Instr) {
	if instr == nil || len(instr.Args) < 2 || instr.Args[0] == nil || instr.Args[1] == nil {
		return
	}
	fieldIdx := int(instr.Aux)
	if fieldIdx < 0 {
		ec.emitDeopt(instr)
		return
	}
	if !ec.emitStableShapeFieldStoreTypeGuard(instr, fieldIdx) {
		return
	}
	valueID := instr.Args[1].ID
	valStore := ec.prepareFieldStoreValue(valueID)
	if !valStore.isFPR {
		ec.resolveValueToReg(valueID, jit.X3)
		valStore.gpr = jit.X3
	}
	svals := ec.resolveRawFieldSvalsPtr(instr.Args[0].ID, jit.X1)
	ec.emitPreparedFieldStoreAt(valStore, svals, fieldIdx)
}

func (ec *emitContext) emitStableShapeFieldStoreTypeGuard(instr *Instr, fieldIdx int) bool {
	if instr == nil || len(instr.Args) < 2 || instr.Args[0] == nil || instr.Args[1] == nil {
		return true
	}
	svals := instr.Args[0].Def
	if svals == nil || svals.Op != OpFieldSvals || svals.Aux <= 0 {
		return true
	}
	shapeID := uint32(svals.Aux)
	stable, ok := runtime.ShapeFieldStableType(shapeID, fieldIdx)
	if !ok {
		return true
	}
	value := instr.Args[1]
	if value.Def != nil {
		if rt, ok := irTypeToRuntimeValueType(value.Def.Type); ok {
			if rt == stable {
				return true
			}
			ec.emitPreciseDeopt(instr)
			return false
		}
	}
	ec.resolveValueToReg(value.ID, jit.X0)
	deoptLabel := ec.uniqueLabel("field_store_type_deopt")
	doneLabel := ec.uniqueLabel("field_store_type_done")
	switch stable {
	case runtime.TypeFloat:
		ec.asm.LSRimm(jit.X2, jit.X0, 48)
		ec.asm.MOVimm16(jit.X3, jit.NB_TagNilShr48)
		ec.asm.CMPreg(jit.X2, jit.X3)
		ec.asm.BCond(jit.CondGE, deoptLabel)
	case runtime.TypeInt:
		emitCheckIsIntPinned(ec.asm, jit.X0, jit.X2)
		ec.asm.BCond(jit.CondNE, deoptLabel)
	default:
		return true
	}
	ec.emitGuardDeoptExit(instr, deoptLabel, doneLabel, true)
	return true
}

// emitSetField emits ARM64 code for OpSetField (table field write).
//
// If field cache info is available (Aux2 != 0), emits inline shape-guarded
// store: extract table pointer, check shapeID, store to svals[fieldIndex].
// On shape guard failure, falls back to table-exit.
//
// Instr layout:
//   - Args[0] = table value (NaN-boxed)
//   - Args[1] = value to store (NaN-boxed)
//   - Aux = constant pool index for field name
//   - Aux2 = (shapeID << 32) | fieldIndex  (0 if no cache)
func (ec *emitContext) emitSetField(instr *Instr) {
	shapeID := uint32(instr.Aux2 >> 32)
	fieldIdx := int(int32(instr.Aux2 & 0xFFFFFFFF))

	// No field cache or invalid: use table-exit fallback.
	if shapeID == 0 || instr.Aux2 == 0 {
		if ec.emitSetFieldDynamicCache(instr) {
			return
		}
		ec.invalidateFieldSvalsCache()
		ec.emitSetFieldExit(instr)
		return
	}

	asm := ec.asm
	tblValueID := instr.Args[0].ID
	valueID := instr.Args[1].ID

	deoptLabel := ec.uniqueLabel("setfield_deopt")
	valStore := ec.prepareFieldStoreValue(valueID)
	if !valStore.isFPR {
		// Load boxed values into X3 first, before table preparation uses
		// X0-X2. X3 is scratch but not touched by emitPrepareFieldTablePtr.
		ec.resolveValueToReg(valueID, jit.X3)
		valStore.gpr = jit.X3
	}

	if ec.hasFieldSvalsCache(tblValueID, shapeID) {
		needsGuard := !ec.stringLookupCleanGuarded[tblValueID]
		if needsGuard {
			ec.ensureNoStringLookupCacheGuard(tblValueID, jit.X0, jit.X2, deoptLabel)
		}
		ec.emitPreparedFieldStore(valStore, fieldIdx)
		if needsGuard {
			doneLabel := ec.uniqueLabel("setfield_clean_done")
			asm.B(doneLabel)
			asm.Label(deoptLabel)
			savedReprs := ec.snapshotValueReprs()
			ec.emitSetFieldExit(instr)
			ec.emitUnboxRawIntRegs(savedReprs)
			ec.restoreValueReprSnapshot(savedReprs)
			asm.Label(doneLabel)
		}
		return
	}

	doneLabel := ec.uniqueLabel("setfield_done")
	notRecordLabel := ec.uniqueLabel("setfield_not_fixed_record")
	ec.resolveValueToReg(tblValueID, jit.X0)
	asm.LSRimm(jit.X4, jit.X0, 48)
	asm.MOVimm16(jit.X5, jit.NB_TagPtrShr48)
	asm.CMPreg(jit.X4, jit.X5)
	asm.BCond(jit.CondNE, notRecordLabel)
	asm.LSRimm(jit.X4, jit.X0, uint8(jit.NB_PtrSubShift))
	asm.LoadImm64(jit.X5, 0xF)
	asm.ANDreg(jit.X4, jit.X4, jit.X5)
	asm.CMPimm(jit.X4, jit.NB_PtrSubFixedRecord)
	asm.BCond(jit.CondNE, notRecordLabel)
	jit.EmitExtractPtr(asm, jit.X4, jit.X0)
	asm.CBZ(jit.X4, deoptLabel)
	asm.LDR(jit.X5, jit.X4, jit.FixedRecordOffMaterialized)
	asm.CBNZ(jit.X5, deoptLabel)
	asm.LDRW(jit.X5, jit.X4, jit.FixedRecordOffShapeID)
	asm.LoadImm64(jit.X6, int64(shapeID))
	asm.CMPreg(jit.X5, jit.X6)
	asm.BCond(jit.CondNE, deoptLabel)
	asm.LDRB(jit.X5, jit.X4, jit.FixedRecordOffN)
	asm.LoadImm64(jit.X6, int64(fieldIdx))
	asm.CMPreg(jit.X6, jit.X5)
	asm.BCond(jit.CondGE, deoptLabel)
	asm.ADDimm(jit.X1, jit.X4, jit.FixedRecordOffValues)
	ec.emitPreparedFieldStore(valStore, fieldIdx)
	asm.B(doneLabel)

	asm.Label(notRecordLabel)
	ec.emitPrepareFieldTablePtr(tblValueID, shapeID, deoptLabel)
	needsCleanGuard := !ec.stringLookupCleanGuarded[tblValueID]
	if needsCleanGuard {
		ec.ensureNoStringLookupCacheGuardWithTablePtr(tblValueID, jit.X0, jit.X2, deoptLabel)
	}

	// Direct field store: svals[fieldIndex] = value.
	asm.LDR(jit.X2, jit.X0, jit.TableOffSvalsLen)
	asm.LoadImm64(jit.X4, int64(fieldIdx))
	asm.CMPreg(jit.X4, jit.X2)
	asm.BCond(jit.CondGE, deoptLabel)
	asm.LDR(jit.X1, jit.X0, jit.TableOffSvals) // X1 = svals data pointer
	ec.emitPreparedFieldStore(valStore, fieldIdx)
	ec.rememberFieldSvalsCache(tblValueID, shapeID)

	// Skip the deopt fallback.
	asm.B(doneLabel)

	// Deopt fallback: use table-exit.
	asm.Label(deoptLabel)
	savedReprs := ec.snapshotValueReprs()
	ec.emitSetFieldExit(instr)
	ec.emitUnboxRawIntRegs(savedReprs)
	ec.restoreValueReprSnapshot(savedReprs)

	asm.Label(doneLabel)
}

func (ec *emitContext) emitSetFieldDynamicCache(instr *Instr) bool {
	if instr == nil || instr.SourcePC < 0 || len(instr.Args) < 2 {
		return false
	}
	asm := ec.asm
	tblValueID := instr.Args[0].ID
	valueID := instr.Args[1].ID
	if def := instr.Args[0].Def; def != nil && (def.Op == OpNewTable || def.Op == OpNewFixedTable) {
		return false
	}
	deoptLabel := ec.uniqueLabel("setfield_dyn_deopt")
	doneLabel := ec.uniqueLabel("setfield_dyn_done")

	asm.LDR(jit.X3, mRegCtx, execCtxOffBaselineFieldCache)
	asm.CBZ(jit.X3, deoptLabel)
	entryOff := instr.SourcePC * jit.FieldCacheEntrySize
	if entryOff <= 4095 {
		asm.ADDimm(jit.X3, jit.X3, uint16(entryOff))
	} else {
		asm.LoadImm64(jit.X4, int64(entryOff))
		asm.ADDreg(jit.X3, jit.X3, jit.X4)
	}
	asm.LDRW(jit.X5, jit.X3, jit.FieldCacheEntryOffShapeID)
	asm.CBZ(jit.X5, deoptLabel)
	asm.LDR(jit.X4, jit.X3, jit.FieldCacheEntryOffFieldIdx)
	asm.CMPimm(jit.X4, 0)
	asm.BCond(jit.CondLT, deoptLabel)

	ec.resolveValueToReg(tblValueID, jit.X0)
	if ec.irTypes[tblValueID] != TypeTable {
		jit.EmitCheckIsTableFull(asm, jit.X0, jit.X1, jit.X2, deoptLabel)
	}
	jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	asm.CBZ(jit.X0, deoptLabel)
	asm.LDRW(jit.X1, jit.X0, jit.TableOffShapeID)
	asm.CMPreg(jit.X1, jit.X5)
	asm.BCond(jit.CondNE, deoptLabel)
	asm.LDR(jit.X1, jit.X0, jit.TableOffSvalsLen)
	asm.CMPreg(jit.X4, jit.X1)
	asm.BCond(jit.CondGE, deoptLabel)
	asm.LDR(jit.X1, jit.X0, jit.TableOffSvals)
	if ec.setFieldValueMayBeRawFloat(instr.Args[1]) || ec.hasFPReg(valueID) {
		valStore := ec.prepareFieldStoreValue(valueID)
		if valStore.isFPR {
			asm.FSTRdReg(valStore.fpr, jit.X1, jit.X4)
		} else {
			ec.resolveValueToReg(valueID, jit.X6)
			asm.STRreg(jit.X6, jit.X1, jit.X4)
		}
	} else {
		ec.resolveValueToReg(valueID, jit.X6)
		asm.LoadImm64(jit.X7, nb64(jit.NB_ValNil))
		asm.CMPreg(jit.X6, jit.X7)
		asm.BCond(jit.CondEQ, deoptLabel)
		asm.STRreg(jit.X6, jit.X1, jit.X4)
	}
	ec.emitBumpTableStringLookupVersion(jit.X0, jit.X7)
	asm.B(doneLabel)

	asm.Label(deoptLabel)
	savedReprs := ec.snapshotValueReprs()
	ec.emitSetFieldExit(instr)
	ec.emitUnboxRawIntRegs(savedReprs)
	ec.restoreValueReprSnapshot(savedReprs)

	asm.Label(doneLabel)
	return true
}

func (ec *emitContext) setFieldValueMayBeRawFloat(v *Value) bool {
	if v == nil {
		return false
	}
	if ec.irTypes[v.ID] == TypeFloat {
		return true
	}
	if v.Def == nil {
		return false
	}
	if v.Def.Type == TypeFloat {
		return true
	}
	switch v.Def.Op {
	case OpAddFloat, OpSubFloat, OpMulFloat, OpDivFloat, OpNegFloat, OpSqrt, OpFMA, OpFMSUB, OpGetFieldNumToFloat, OpFieldLoadNumToFloat, OpNumToFloat:
		return true
	default:
		return false
	}
}

type fieldStoreValue struct {
	isFPR bool
	fpr   jit.FReg
	gpr   jit.Reg
}

func (ec *emitContext) prepareFieldStoreValue(valueID int) fieldStoreValue {
	if ec.hasFPReg(valueID) {
		return fieldStoreValue{isFPR: true, fpr: ec.physFPReg(valueID)}
	}
	return fieldStoreValue{gpr: jit.X3}
}

func (ec *emitContext) emitPreparedFieldStore(val fieldStoreValue, fieldIdx int) {
	ec.emitPreparedFieldStoreAt(val, jit.X1, fieldIdx)
}

func (ec *emitContext) emitPreparedFieldStoreAt(val fieldStoreValue, base jit.Reg, fieldIdx int) {
	if val.isFPR {
		ec.asm.FSTRd(val.fpr, base, fieldIdx*jit.ValueSize)
		return
	}
	ec.asm.STR(val.gpr, base, fieldIdx*jit.ValueSize)
}

func (ec *emitContext) emitBumpTableStringLookupVersion(tableReg, tmp jit.Reg) {
	asm := ec.asm
	skipLabel := ec.uniqueLabel("string_lookup_version_skip")
	asm.LDR(tmp, tableReg, jit.TableOffStringLookupVer)
	asm.CBZ(tmp, skipLabel)
	asm.ADDimm(tmp, tmp, 1)
	asm.STR(tmp, tableReg, jit.TableOffStringLookupVer)
	asm.Label(skipLabel)
}

func (ec *emitContext) ensureNoStringLookupCacheGuard(tblValueID int, tableReg, tmp jit.Reg, deoptLabel string) bool {
	if ec.stringLookupCleanGuarded[tblValueID] {
		return true
	}
	ec.resolveValueToReg(tblValueID, tableReg)
	jit.EmitExtractPtr(ec.asm, tableReg, tableReg)
	return ec.ensureNoStringLookupCacheGuardWithTablePtr(tblValueID, tableReg, tmp, deoptLabel)
}

func (ec *emitContext) ensureNoStringLookupCacheGuardWithTablePtr(tblValueID int, tableReg, tmp jit.Reg, deoptLabel string) bool {
	if ec.stringLookupCleanGuarded[tblValueID] {
		return true
	}
	ec.asm.LDR(tmp, tableReg, jit.TableOffStringLookupVer)
	ec.asm.CBNZ(tmp, deoptLabel)
	ec.stringLookupCleanGuarded[tblValueID] = true
	return true
}

// emitSetFieldExit emits a table-exit for OpSetField when no inline cache
// is available or when the shape guard fails.
func (ec *emitContext) emitSetFieldExit(instr *Instr) {
	asm := ec.asm

	// Store the table arg to its home slot.
	if len(instr.Args) > 0 {
		ec.resolveValueToReg(instr.Args[0].ID, jit.X0)
		tblSlot, hasTblSlot := ec.slotMap[instr.Args[0].ID]
		if hasTblSlot {
			asm.STR(jit.X0, mRegRegs, slotOffset(tblSlot))
		}
	}

	valSlot := 0
	if len(instr.Args) > 1 {
		var ok bool
		valSlot, ok = ec.slotMap[instr.Args[1].ID]
		if !ok {
			ec.emitDeopt(instr)
			return
		}
		ec.resolveValueToReg(instr.Args[1].ID, jit.X0)
		asm.STR(jit.X0, mRegRegs, slotOffset(valSlot))
	}

	tblSlot := 0
	if len(instr.Args) > 0 {
		if s, ok := ec.slotMap[instr.Args[0].ID]; ok {
			tblSlot = s
		}
	}

	// Store all active register-resident values to memory.
	ec.recordExitResumeCheckSite(instr, ExitTableExit, nil, exitResumeCheckOptions{RequireTableInputs: true})
	ec.emitStoreAllActiveRegs()

	// Write table-exit descriptor.
	asm.LoadImm64(jit.X0, int64(TableOpSetField))
	asm.STR(jit.X0, mRegCtx, execCtxOffTableOp)
	asm.LoadImm64(jit.X0, int64(tblSlot))
	asm.STR(jit.X0, mRegCtx, execCtxOffTableSlot)
	asm.LoadImm64(jit.X0, int64(instr.Aux)) // constant pool index
	asm.STR(jit.X0, mRegCtx, execCtxOffTableAux)
	asm.LoadImm64(jit.X0, int64(valSlot))
	asm.STR(jit.X0, mRegCtx, execCtxOffTableValSlot)
	asm.LoadImm64(jit.X0, int64(instr.SourcePC))
	asm.STR(jit.X0, mRegCtx, execCtxOffTableKeySlot)
	asm.LoadImm64(jit.X0, int64(instr.ID))
	asm.STR(jit.X0, mRegCtx, execCtxOffTableExitID)

	// Set ExitCode = ExitTableExit and return to Go.
	ec.emitSetResumeNumericPass()
	asm.LoadImm64(jit.X0, int64(ExitTableExit))
	asm.STR(jit.X0, mRegCtx, execCtxOffExitCode)
	if ec.numericMode {
		asm.B("num_deopt_epilogue")
	} else {
		asm.B("deopt_epilogue")
	}

	// Continue label: resume entry jumps here.
	continueLabel := ec.passLabel(fmt.Sprintf("table_continue_%d", instr.ID))
	asm.Label(continueLabel)

	// Reload all active registers from memory.
	ec.emitReloadAllActiveRegs()

	// Record for deferred resume.
	ec.callExitIDs = append(ec.callExitIDs, instr.ID)
	ec.deferredResumes = append(ec.deferredResumes, deferredResume{
		instrID:       instr.ID,
		continueLabel: continueLabel,
		numericPass:   ec.numericMode,
	})
}
