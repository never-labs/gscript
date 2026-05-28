//go:build darwin && arm64

package methodjit

import (
	"fmt"
	"unsafe"

	"github.com/gscript/gscript/internal/jit"
	"github.com/gscript/gscript/internal/runtime"
)

func (ec *emitContext) emitNewFixedTable(instr *Instr) {
	asm := ec.asm

	resultSlot, hasResultSlot := ec.slotMap[instr.ID]
	if !hasResultSlot {
		ec.emitDeopt(instr)
		return
	}
	if instr.Aux2 != 2 || len(instr.Args) != 2 {
		ec.emitNewFixedTableN(instr, resultSlot)
		return
	}

	doneLabel := ec.uniqueLabel("newfixed_done")
	missLabel := ec.uniqueLabel("newfixed_miss")
	if ec.emitNewFixedTable2CacheFastPath(instr, doneLabel, missLabel) {
		asm.Label(missLabel)
	}
	ec.emitNewFixedTable2Exit(instr, resultSlot)
	asm.Label(doneLabel)
}

func (ec *emitContext) emitNewFixedTableN(instr *Instr, resultSlot int) {
	if instr == nil || instr.Aux2 <= 2 || len(instr.Args) != int(instr.Aux2) {
		ec.emitDeopt(instr)
		return
	}
	doneLabel := ec.uniqueLabel("newfixedn_done")
	missLabel := ec.uniqueLabel("newfixedn_miss")
	ec.emitNewFixedTableNCacheFastPath(instr, doneLabel, missLabel)
	ec.asm.Label(missLabel)
	ec.emitNewFixedTableNExit(instr, resultSlot)
	ec.asm.Label(doneLabel)
}

func (ec *emitContext) emitNewFixedTable2CacheFastPath(instr *Instr, doneLabel, missLabel string) bool {
	if ec == nil || instr == nil || instr.ID < 0 || instr.ID >= len(ec.newTableCaches) {
		return false
	}
	if ec.fn == nil || !fixedTableCtor2Cacheable(ec.fn.Proto, instr) {
		return false
	}
	asm := ec.asm

	ec.resolveValueToReg(instr.Args[0].ID, jit.X5)
	ec.resolveValueToReg(instr.Args[1].ID, jit.X6)
	emptyLabel := ec.uniqueLabel("newfixed_empty")
	val1NeedsNilCheck := !fixedTableArgProvenNonNil(instr.Args[0])
	val2NeedsNilCheck := !fixedTableArgProvenNonNil(instr.Args[1])
	val1NilLabel := ""
	if val1NeedsNilCheck || val2NeedsNilCheck {
		asm.LoadImm64(jit.X7, nb64(jit.NB_ValNil))
	}
	if val1NeedsNilCheck {
		asm.CMPreg(jit.X5, jit.X7)
		if val2NeedsNilCheck {
			val1NilLabel = ec.uniqueLabel("newfixed_val1_nil")
			asm.BCond(jit.CondEQ, val1NilLabel)
			asm.CMPreg(jit.X6, jit.X7)
			asm.BCond(jit.CondEQ, missLabel)
		} else {
			asm.BCond(jit.CondEQ, missLabel)
		}
	} else if val2NeedsNilCheck {
		asm.CMPreg(jit.X6, jit.X7)
		asm.BCond(jit.CondEQ, missLabel)
	}

	cacheBase := uintptr(unsafe.Pointer(&ec.newTableCaches[0]))
	entryOff := instr.ID * newTableCacheEntrySize
	asm.LoadImm64(jit.X2, int64(cacheBase))
	if entryOff > 0 {
		if entryOff <= 4095 {
			asm.ADDimm(jit.X2, jit.X2, uint16(entryOff))
		} else {
			asm.LoadImm64(jit.X3, int64(entryOff))
			asm.ADDreg(jit.X2, jit.X2, jit.X3)
		}
	}

	asm.LDR(jit.X0, jit.X2, newTableCacheEntryValuesOff)
	asm.CBZ(jit.X0, missLabel)
	asm.LDR(jit.X3, jit.X2, newTableCacheEntryPosOff)
	asm.LDR(jit.X4, jit.X2, newTableCacheEntryLenOff)
	asm.CMPreg(jit.X3, jit.X4)
	asm.BCond(jit.CondGE, missLabel)
	asm.LDRreg(jit.X0, jit.X0, jit.X3)
	asm.ADDimm(jit.X3, jit.X3, 1)
	asm.STR(jit.X3, jit.X2, newTableCacheEntryPosOff)

	jit.EmitExtractPtr(asm, jit.X1, jit.X0)
	asm.LDR(jit.X2, jit.X1, jit.TableOffSvals)
	asm.STR(jit.X5, jit.X2, 0)
	asm.STR(jit.X6, jit.X2, jit.ValueSize)
	ec.storeResultNB(jit.X0, instr.ID)
	asm.B(doneLabel)

	if !val1NeedsNilCheck || !val2NeedsNilCheck {
		return true
	}
	asm.Label(val1NilLabel)
	asm.CMPreg(jit.X6, jit.X7)
	asm.BCond(jit.CondEQ, emptyLabel)
	asm.B(missLabel)

	asm.Label(emptyLabel)
	asm.LoadImm64(jit.X2, int64(cacheBase))
	if entryOff > 0 {
		if entryOff <= 4095 {
			asm.ADDimm(jit.X2, jit.X2, uint16(entryOff))
		} else {
			asm.LoadImm64(jit.X3, int64(entryOff))
			asm.ADDreg(jit.X2, jit.X2, jit.X3)
		}
	}
	asm.LDR(jit.X0, jit.X2, newTableCacheEntryEmptyValuesOff)
	asm.CBZ(jit.X0, missLabel)
	asm.LDR(jit.X3, jit.X2, newTableCacheEntryEmptyPosOff)
	asm.LDR(jit.X4, jit.X2, newTableCacheEntryEmptyLenOff)
	asm.CMPreg(jit.X3, jit.X4)
	asm.BCond(jit.CondGE, missLabel)
	asm.LDRreg(jit.X0, jit.X0, jit.X3)
	asm.ADDimm(jit.X3, jit.X3, 1)
	asm.STR(jit.X3, jit.X2, newTableCacheEntryEmptyPosOff)
	ec.storeResultNB(jit.X0, instr.ID)
	asm.B(doneLabel)
	return true
}

func (ec *emitContext) emitNewFixedTable2Exit(instr *Instr, resultSlot int) {
	asm := ec.asm

	val1Slot, ok1 := fixedTableArgSlot(ec, instr, 0)
	val2Slot, ok2 := fixedTableArgSlot(ec, instr, 1)
	if !ok1 || !ok2 {
		ec.emitDeopt(instr)
		return
	}

	ec.recordExitResumeCheckSite(instr, ExitTableExit, []int{resultSlot}, exitResumeCheckOptions{RequireTableInputs: true})
	ec.emitStoreAllActiveRegs()

	asm.LoadImm64(jit.X0, int64(TableOpNewFixedTable2))
	asm.STR(jit.X0, mRegCtx, execCtxOffTableOp)
	asm.LoadImm64(jit.X0, int64(resultSlot))
	asm.STR(jit.X0, mRegCtx, execCtxOffTableSlot)
	asm.LoadImm64(jit.X0, int64(val1Slot))
	asm.STR(jit.X0, mRegCtx, execCtxOffTableKeySlot)
	asm.LoadImm64(jit.X0, int64(val2Slot))
	asm.STR(jit.X0, mRegCtx, execCtxOffTableValSlot)
	asm.LoadImm64(jit.X0, instr.Aux)
	asm.STR(jit.X0, mRegCtx, execCtxOffTableAux)
	asm.LoadImm64(jit.X0, int64(instr.ID))
	asm.STR(jit.X0, mRegCtx, execCtxOffTableExitID)

	ec.emitSetResumeNumericPass()
	asm.LoadImm64(jit.X0, int64(ExitTableExit))
	asm.STR(jit.X0, mRegCtx, execCtxOffExitCode)
	if ec.numericMode {
		asm.B("num_deopt_epilogue")
	} else {
		asm.B("deopt_epilogue")
	}

	continueLabel := ec.passLabel(fmt.Sprintf("table_continue_%d", instr.ID))
	asm.Label(continueLabel)
	ec.emitReloadAllActiveRegs()
	asm.LDR(jit.X0, mRegRegs, slotOffset(resultSlot))
	ec.storeResultNB(jit.X0, instr.ID)

	ec.callExitIDs = append(ec.callExitIDs, instr.ID)
	ec.deferredResumes = append(ec.deferredResumes, deferredResume{
		instrID:       instr.ID,
		continueLabel: continueLabel,
		numericPass:   ec.numericMode,
	})
}

func (ec *emitContext) emitNewFixedTableNCacheFastPath(instr *Instr, doneLabel, missLabel string) bool {
	if ec == nil || instr == nil || instr.ID < 0 || instr.ID >= len(ec.newTableCaches) {
		return false
	}
	if ec.fn == nil || !fixedTableCtorNCacheable(ec.fn.Proto, instr) {
		return false
	}
	ctor, _ := fixedTableCtorNForInstr(ec.fn.Proto, instr)
	shapeID := uint32(0)
	if ctor != nil && ctor.Shape != nil {
		shapeID = ctor.Shape.ID
	}
	useFixedRecord := fixedRecordCtorNCacheableForFunction(ec.fn, instr, ctor)
	slots := fixedTableArgSlots(ec, instr)
	if len(slots) != len(instr.Args) {
		return false
	}
	asm := ec.asm

	if ec.emitNewFixedTableNConstNilMaskCacheFastPath(instr, ctor, doneLabel, missLabel) {
		return true
	}

	nilBits := nb64(jit.NB_ValNil)
	nilReg := ec.reusableFixedTableNNilReg(instr)
	if nilReg != jit.XZR {
		asm.LoadImm64(nilReg, nilBits)
	}
	sparseNilLabel := ""
	sparseNilArg := singleRuntimeNullableFixedTableArg(instr)
	if sparseNilArg >= 0 {
		mask := fixedTableMaskExcept(len(instr.Args), sparseNilArg)
		if sparse := runtime.SmallTableCtorNForMask(ctor, mask); sparse.Shape != nil {
			sparseNilLabel = ec.uniqueLabel("newfixedn_sparse_nil")
			ec.resolveValueToReg(instr.Args[sparseNilArg].ID, jit.X5)
			if nilReg != jit.XZR {
				asm.CMPreg(jit.X5, nilReg)
			} else {
				asm.LoadImm64(jit.X6, nilBits)
				asm.CMPreg(jit.X5, jit.X6)
			}
			asm.BCond(jit.CondEQ, sparseNilLabel)
		}
	}
	for _, arg := range instr.Args {
		if fixedTableArgProvenNonNil(arg) {
			continue
		}
		ec.resolveValueToReg(arg.ID, jit.X5)
		if nilReg != jit.XZR {
			asm.CMPreg(jit.X5, nilReg)
		} else {
			asm.LoadImm64(jit.X6, nilBits)
			asm.CMPreg(jit.X5, jit.X6)
		}
		asm.BCond(jit.CondEQ, missLabel)
	}

	cacheBase := uintptr(unsafe.Pointer(&ec.newTableCaches[0]))
	entryOff := instr.ID * newTableCacheEntrySize
	asm.LoadImm64(jit.X2, int64(cacheBase))
	if entryOff > 0 {
		if entryOff <= 4095 {
			asm.ADDimm(jit.X2, jit.X2, uint16(entryOff))
		} else {
			asm.LoadImm64(jit.X3, int64(entryOff))
			asm.ADDreg(jit.X2, jit.X2, jit.X3)
		}
	}

	asm.LDR(jit.X0, jit.X2, newTableCacheEntryValuesOff)
	asm.CBZ(jit.X0, missLabel)
	asm.LDR(jit.X3, jit.X2, newTableCacheEntryPosOff)
	asm.LDR(jit.X4, jit.X2, newTableCacheEntryLenOff)
	asm.CMPreg(jit.X3, jit.X4)
	asm.BCond(jit.CondGE, missLabel)
	asm.LDRreg(jit.X0, jit.X0, jit.X3)
	asm.ADDimm(jit.X3, jit.X3, 1)
	asm.STR(jit.X3, jit.X2, newTableCacheEntryPosOff)

	jit.EmitExtractPtr(asm, jit.X1, jit.X0)
	if useFixedRecord {
		notRecordLabel := ec.uniqueLabel("newfixedn_not_record")
		valuesReadyLabel := ec.uniqueLabel("newfixedn_values_ready")
		asm.LSRimm(jit.X2, jit.X0, uint8(jit.NB_PtrSubShift))
		asm.LoadImm64(jit.X3, 0xF)
		asm.ANDreg(jit.X2, jit.X2, jit.X3)
		asm.CMPimm(jit.X2, jit.NB_PtrSubFixedRecord)
		asm.BCond(jit.CondNE, notRecordLabel)
		asm.LoadImm64(jit.X2, 0)
		asm.STR(jit.X2, jit.X1, jit.FixedRecordOffMaterialized)
		asm.ADDimm(jit.X2, jit.X1, jit.FixedRecordOffValues)
		asm.B(valuesReadyLabel)

		asm.Label(notRecordLabel)
		asm.LDR(jit.X2, jit.X1, jit.TableOffSvals)
		asm.Label(valuesReadyLabel)
	} else {
		asm.LDR(jit.X2, jit.X1, jit.TableOffSvals)
	}
	for i, arg := range instr.Args {
		ec.resolveValueToReg(arg.ID, jit.X5)
		if !ec.emitNewFixedTableValueTypeGuard(shapeID, i, arg, jit.X5, missLabel) {
			return false
		}
		asm.STR(jit.X5, jit.X2, i*jit.ValueSize)
	}
	ec.storeResultNB(jit.X0, instr.ID)
	asm.B(doneLabel)
	if sparseNilLabel != "" {
		asm.Label(sparseNilLabel)
		ec.emitNewFixedTableNSparseMaskCacheResult(instr, ctor, sparseNilArg, doneLabel, missLabel)
	}
	return true
}

func singleRuntimeNullableFixedTableArg(instr *Instr) int {
	if instr == nil {
		return -1
	}
	idx := -1
	for i, arg := range instr.Args {
		if fixedTableArgProvenNonNil(arg) {
			continue
		}
		if idx >= 0 {
			return -1
		}
		idx = i
	}
	return idx
}

func (ec *emitContext) emitNewFixedTableNSparseMaskCacheResult(instr *Instr, ctor *runtime.SmallTableCtorN, omitted int, doneLabel, missLabel string) {
	if ec == nil || instr == nil || ctor == nil || omitted < 0 || omitted >= len(instr.Args) {
		ec.emitDeopt(instr)
		return
	}
	mask := fixedTableMaskExcept(len(instr.Args), omitted)
	sparse := runtime.SmallTableCtorNForMask(ctor, mask)
	if sparse.Shape == nil {
		ec.emitDeopt(instr)
		return
	}
	asm := ec.asm
	cacheBase := uintptr(unsafe.Pointer(&ec.newTableCaches[0]))
	entryOff := instr.ID * newTableCacheEntrySize
	asm.LoadImm64(jit.X2, int64(cacheBase))
	if entryOff > 0 {
		if entryOff <= 4095 {
			asm.ADDimm(jit.X2, jit.X2, uint16(entryOff))
		} else {
			asm.LoadImm64(jit.X3, int64(entryOff))
			asm.ADDreg(jit.X2, jit.X2, jit.X3)
		}
	}

	asm.LDR(jit.X0, jit.X2, newTableCacheEntrySparseOff)
	asm.CBZ(jit.X0, missLabel)
	asm.LDR(jit.X3, jit.X2, newTableCacheEntrySparseOff+int(unsafe.Sizeof(uintptr(0))))
	asm.CMPimm(jit.X3, uint16(omitted+1))
	asm.BCond(jit.CondLT, missLabel)
	if omitted > 0 {
		asm.LoadImm64(jit.X3, int64(omitted*newTableSparseCacheEntrySize))
		asm.ADDreg(jit.X0, jit.X0, jit.X3)
	}
	asm.LoadImm64(jit.X3, int64(mask))
	asm.LDR(jit.X4, jit.X0, newTableSparseCacheEntryMaskOff)
	asm.CMPreg(jit.X4, jit.X3)
	asm.BCond(jit.CondNE, missLabel)
	asm.LDR(jit.X3, jit.X0, newTableSparseCacheEntryValuesOff)
	asm.CBZ(jit.X3, missLabel)
	asm.LDR(jit.X4, jit.X0, newTableSparseCacheEntryPosOff)
	asm.LDR(jit.X5, jit.X0, newTableSparseCacheEntryLenOff)
	asm.CMPreg(jit.X4, jit.X5)
	asm.BCond(jit.CondGE, missLabel)
	asm.LDRreg(jit.X6, jit.X3, jit.X4)
	asm.ADDimm(jit.X4, jit.X4, 1)
	asm.STR(jit.X4, jit.X0, newTableSparseCacheEntryPosOff)

	jit.EmitExtractPtr(asm, jit.X1, jit.X6)
	asm.LDR(jit.X2, jit.X1, jit.TableOffSvals)
	dst := 0
	for i, arg := range instr.Args {
		if i == omitted {
			continue
		}
		ec.resolveValueToReg(arg.ID, jit.X5)
		if !ec.emitNewFixedTableValueTypeGuard(sparse.Shape.ID, dst, arg, jit.X5, missLabel) {
			ec.emitDeopt(instr)
			return
		}
		asm.STR(jit.X5, jit.X2, dst*jit.ValueSize)
		dst++
	}
	ec.storeResultNB(jit.X6, instr.ID)
	asm.B(doneLabel)
}

func (ec *emitContext) emitNewFixedTableNConstNilMaskCacheFastPath(instr *Instr, ctor *runtime.SmallTableCtorN, doneLabel, missLabel string) bool {
	if ec == nil || instr == nil || ctor == nil || ctor.Shape == nil || len(instr.Args) != len(ctor.Keys) {
		return false
	}
	if len(instr.Args) <= 2 || len(instr.Args) > runtime.SmallFieldCap {
		return false
	}
	omitted := -1
	for i, arg := range instr.Args {
		if isConstNilValue(arg) {
			if omitted >= 0 {
				return false
			}
			omitted = i
		}
	}
	if omitted < 0 {
		return false
	}
	mask := fixedTableMaskExcept(len(instr.Args), omitted)
	sparse := runtime.SmallTableCtorNForMask(ctor, mask)
	if sparse.Shape == nil {
		return false
	}

	asm := ec.asm
	nilBits := nb64(jit.NB_ValNil)
	nilReg := ec.reusableFixedTableNNilReg(instr)
	if nilReg != jit.XZR {
		asm.LoadImm64(nilReg, nilBits)
	}
	for i, arg := range instr.Args {
		if i == omitted || fixedTableArgProvenNonNil(arg) {
			continue
		}
		ec.resolveValueToReg(arg.ID, jit.X5)
		if nilReg != jit.XZR {
			asm.CMPreg(jit.X5, nilReg)
		} else {
			asm.LoadImm64(jit.X6, nilBits)
			asm.CMPreg(jit.X5, jit.X6)
		}
		asm.BCond(jit.CondEQ, missLabel)
	}

	cacheBase := uintptr(unsafe.Pointer(&ec.newTableCaches[0]))
	entryOff := instr.ID * newTableCacheEntrySize
	asm.LoadImm64(jit.X2, int64(cacheBase))
	if entryOff > 0 {
		if entryOff <= 4095 {
			asm.ADDimm(jit.X2, jit.X2, uint16(entryOff))
		} else {
			asm.LoadImm64(jit.X3, int64(entryOff))
			asm.ADDreg(jit.X2, jit.X2, jit.X3)
		}
	}

	asm.LDR(jit.X0, jit.X2, newTableCacheEntrySparseOff)
	asm.CBZ(jit.X0, missLabel)
	asm.LDR(jit.X3, jit.X2, newTableCacheEntrySparseOff+int(unsafe.Sizeof(uintptr(0))))
	asm.CMPimm(jit.X3, uint16(omitted+1))
	asm.BCond(jit.CondLT, missLabel)
	if omitted > 0 {
		asm.LoadImm64(jit.X3, int64(omitted*newTableSparseCacheEntrySize))
		asm.ADDreg(jit.X0, jit.X0, jit.X3)
	}
	asm.LoadImm64(jit.X3, int64(mask))
	asm.LDR(jit.X4, jit.X0, newTableSparseCacheEntryMaskOff)
	asm.CMPreg(jit.X4, jit.X3)
	asm.BCond(jit.CondNE, missLabel)
	asm.LDR(jit.X3, jit.X0, newTableSparseCacheEntryValuesOff)
	asm.CBZ(jit.X3, missLabel)
	asm.LDR(jit.X4, jit.X0, newTableSparseCacheEntryPosOff)
	asm.LDR(jit.X5, jit.X0, newTableSparseCacheEntryLenOff)
	asm.CMPreg(jit.X4, jit.X5)
	asm.BCond(jit.CondGE, missLabel)
	asm.LDRreg(jit.X6, jit.X3, jit.X4)
	asm.ADDimm(jit.X4, jit.X4, 1)
	asm.STR(jit.X4, jit.X0, newTableSparseCacheEntryPosOff)

	jit.EmitExtractPtr(asm, jit.X1, jit.X6)
	asm.LDR(jit.X2, jit.X1, jit.TableOffSvals)
	dst := 0
	for i, arg := range instr.Args {
		if i == omitted {
			continue
		}
		ec.resolveValueToReg(arg.ID, jit.X5)
		if !ec.emitNewFixedTableValueTypeGuard(sparse.Shape.ID, dst, arg, jit.X5, missLabel) {
			return false
		}
		asm.STR(jit.X5, jit.X2, dst*jit.ValueSize)
		dst++
	}
	ec.storeResultNB(jit.X6, instr.ID)
	asm.B(doneLabel)
	return true
}

func (ec *emitContext) emitNewFixedTableValueTypeGuard(shapeID uint32, fieldIdx int, arg *Value, valReg jit.Reg, missLabel string) bool {
	stable, ok := runtime.ShapeFieldStableType(shapeID, fieldIdx)
	if !ok {
		return true
	}
	if arg != nil && arg.Def != nil {
		if rt, ok := irTypeToRuntimeValueType(arg.Def.Type); ok {
			if rt == stable {
				return true
			}
			return false
		}
	}
	switch stable {
	case runtime.TypeFloat:
		ec.asm.LSRimm(jit.X6, valReg, 48)
		ec.asm.MOVimm16(jit.X7, jit.NB_TagNilShr48)
		ec.asm.CMPreg(jit.X6, jit.X7)
		ec.asm.BCond(jit.CondGE, missLabel)
	case runtime.TypeInt:
		if valReg != jit.X6 {
			ec.asm.MOVreg(jit.X6, valReg)
		}
		emitCheckIsIntPinned(ec.asm, jit.X6, jit.X7)
		ec.asm.BCond(jit.CondNE, missLabel)
	default:
		return true
	}
	return true
}

func (ec *emitContext) reusableFixedTableNNilReg(instr *Instr) jit.Reg {
	if ec == nil || instr == nil {
		return jit.XZR
	}
	candidates := []jit.Reg{jit.X14, jit.X15, jit.X13, jit.X12, jit.X11, jit.X10, jit.X9, jit.X8}
	for _, cand := range candidates {
		used := false
		for _, arg := range instr.Args {
			if arg == nil {
				continue
			}
			if ec.hasReg(arg.ID) && ec.physReg(arg.ID) == cand {
				used = true
				break
			}
		}
		if !used {
			return cand
		}
	}
	return jit.XZR
}

func fixedTableArgProvenNonNil(arg *Value) bool {
	if arg == nil || arg.Def == nil {
		return false
	}
	switch arg.Def.Type {
	case TypeInt, TypeFloat, TypeBool, TypeString, TypeTable, TypeFunction:
		return true
	default:
		return false
	}
}

func (ec *emitContext) emitNewFixedTableNExit(instr *Instr, resultSlot int) {
	asm := ec.asm

	slots := fixedTableArgSlots(ec, instr)
	if len(slots) != len(instr.Args) {
		ec.emitDeopt(instr)
		return
	}
	if ec.fixedTableArgSlots != nil {
		ec.fixedTableArgSlots[instr.ID] = append([]int(nil), slots...)
	}
	for i, arg := range instr.Args {
		ec.resolveValueToReg(arg.ID, jit.X0)
		asm.STR(jit.X0, mRegRegs, slotOffset(slots[i]))
	}

	ec.recordExitResumeCheckSite(instr, ExitTableExit, []int{resultSlot}, exitResumeCheckOptions{RequireTableInputs: true})
	ec.emitStoreAllActiveRegs()

	asm.LoadImm64(jit.X0, int64(TableOpNewFixedTableN))
	asm.STR(jit.X0, mRegCtx, execCtxOffTableOp)
	asm.LoadImm64(jit.X0, int64(resultSlot))
	asm.STR(jit.X0, mRegCtx, execCtxOffTableSlot)
	asm.LoadImm64(jit.X0, instr.Aux)
	asm.STR(jit.X0, mRegCtx, execCtxOffTableAux)
	asm.LoadImm64(jit.X0, instr.Aux2)
	asm.STR(jit.X0, mRegCtx, execCtxOffTableAux2)
	asm.LoadImm64(jit.X0, int64(instr.ID))
	asm.STR(jit.X0, mRegCtx, execCtxOffTableExitID)

	ec.emitSetResumeNumericPass()
	asm.LoadImm64(jit.X0, int64(ExitTableExit))
	asm.STR(jit.X0, mRegCtx, execCtxOffExitCode)
	if ec.numericMode {
		asm.B("num_deopt_epilogue")
	} else {
		asm.B("deopt_epilogue")
	}

	continueLabel := ec.passLabel(fmt.Sprintf("table_continue_%d", instr.ID))
	asm.Label(continueLabel)
	ec.emitReloadAllActiveRegs()
	asm.LDR(jit.X0, mRegRegs, slotOffset(resultSlot))
	ec.storeResultNB(jit.X0, instr.ID)

	ec.callExitIDs = append(ec.callExitIDs, instr.ID)
	ec.deferredResumes = append(ec.deferredResumes, deferredResume{
		instrID:       instr.ID,
		continueLabel: continueLabel,
		numericPass:   ec.numericMode,
	})
}

func fixedTableArgSlots(ec *emitContext, instr *Instr) []int {
	if ec == nil || instr == nil || len(instr.Args) == 0 || len(instr.Args) > runtime.SmallFieldCap {
		return nil
	}
	slots := make([]int, len(instr.Args))
	for i := range instr.Args {
		slot, ok := fixedTableArgSlot(ec, instr, i)
		if !ok {
			return nil
		}
		slots[i] = slot
	}
	return slots
}

func fixedTableArgSlot(ec *emitContext, instr *Instr, argIdx int) (int, bool) {
	if instr == nil || argIdx < 0 || argIdx >= len(instr.Args) || instr.Args[argIdx] == nil {
		return 0, false
	}
	slot, ok := ec.slotMap[instr.Args[argIdx].ID]
	return slot, ok
}
