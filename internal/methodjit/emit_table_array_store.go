//go:build darwin && arm64

// emit_table_array_store.go: array/index store code paths (raw typed-array
// store helper, OpTableArrayStore, swap/swap-pairs mutations, OpSetTable) and
// their store-side helpers and bounded-key recording. Pure code movement from
// emit_table_array.go; no behavior change.

package methodjit

import (
	"github.com/Never-Labs/gscript/internal/jit"
	"github.com/Never-Labs/gscript/internal/vm"
)

func (ec *emitContext) emitTableArrayRawStore(cfg tableArrayRawStoreConfig) bool {
	if cfg.labelPrefix == "" {
		cfg.labelPrefix = "tarr_raw_store"
	}
	dataOff, lenOff, capOff, ok := tableArrayStoreOffsets(cfg.kind)
	if !ok {
		return false
	}

	asm := ec.asm
	storeLabel := ec.uniqueLabel(cfg.labelPrefix + "_store")
	appendLabel := ec.uniqueLabel(cfg.labelPrefix + "_append")
	sparseLabel := ec.uniqueLabel(cfg.labelPrefix + "_sparse")
	allowGrow := cfg.allowGrowWithinCapacity && !cfg.priorLoadBounds && !cfg.upperBoundSafe
	allowSparse := allowGrow && cfg.kind != int64(vm.FBKindMixed)

	emitBounds := func(boundsOnly bool) {
		switch {
		case cfg.upperBoundSafe:
			return
		case cfg.priorLoadBounds:
			asm.CBZ(jit.X17, cfg.missLabel)
		case boundsOnly:
			emitTypedArraySetBoundsOnlyCheck(asm, cfg.tableReg, cfg.keyReg, cfg.lenReg, lenOff, cfg.missLabel)
		case allowGrow:
			if allowSparse {
				emitTypedArraySetBoundsAppendOrSparseCheck(asm, cfg.tableReg, cfg.keyReg, cfg.lenReg, lenOff, appendLabel, sparseLabel, cfg.missLabel)
			} else {
				emitTypedArraySetBoundsOrAppendCheck(asm, cfg.tableReg, cfg.keyReg, cfg.lenReg, lenOff, appendLabel, cfg.missLabel)
			}
		default:
			asm.CMPreg(cfg.keyReg, cfg.lenReg)
			asm.BCond(jit.CondGE, cfg.missLabel)
		}
	}

	emitLoadData := func() {
		if cfg.loadDataFromTable {
			asm.LDR(cfg.dataReg, cfg.tableReg, dataOff)
		}
	}
	emitKeysDirty := func() {
		if cfg.keysDirtyAlreadyWritten {
			return
		}
		asm.MOVimm16(jit.X5, 1)
		asm.STRB(jit.X5, cfg.tableReg, jit.TableOffKeysDirty)
	}
	emitZeroValidForTypedStore := func() {
		if cfg.kind != int64(vm.FBKindInt) && cfg.kind != int64(vm.FBKindFloat) {
			return
		}
		scratch := jit.X17
		if scratch == cfg.tableReg || scratch == cfg.keyReg || scratch == cfg.dataReg {
			scratch = jit.X16
		}
		if scratch == cfg.tableReg || scratch == cfg.keyReg || scratch == cfg.dataReg {
			scratch = jit.X5
		}
		if scratch == cfg.tableReg || scratch == cfg.keyReg || scratch == cfg.dataReg {
			scratch = jit.X6
		}
		nonZeroLabel := ec.uniqueLabel(cfg.labelPrefix + "_nonzero_key")
		asm.CMPimm(cfg.keyReg, 0)
		asm.BCond(jit.CondNE, nonZeroLabel)
		asm.MOVimm16(scratch, 1)
		asm.STRB(scratch, cfg.tableReg, jit.TableOffArrayZeroValid)
		asm.Label(nonZeroLabel)
	}
	emitSuccess := func() {
		if !cfg.fallthroughOnSuccess {
			asm.B(cfg.successLabel)
		}
	}
	emitGrowPaths := func(markDirty bool) {
		if !allowGrow {
			return
		}
		if markDirty {
			if cfg.carryLenOnGrow {
				emitTypedArraySetAppendPathCarryLenDirty(asm, cfg.tableReg, cfg.keyReg, jit.X6, cfg.lenReg, lenOff, capOff, appendLabel, cfg.missLabel, storeLabel, true)
			} else {
				emitTypedArraySetAppendPathDirty(asm, cfg.tableReg, cfg.keyReg, jit.X6, lenOff, capOff, appendLabel, cfg.missLabel, storeLabel)
			}
			if allowSparse {
				if cfg.carryLenOnGrow {
					emitTypedArraySetSparsePathCarryLenDirty(asm, cfg.tableReg, cfg.keyReg, jit.X6, cfg.lenReg, lenOff, capOff, sparseLabel, cfg.missLabel, storeLabel, true)
				} else {
					emitTypedArraySetSparsePathDirty(asm, cfg.tableReg, cfg.keyReg, jit.X6, lenOff, capOff, sparseLabel, cfg.missLabel, storeLabel)
				}
			}
			return
		}
		if cfg.carryLenOnGrow {
			emitTypedArraySetAppendPathCarryLenDirty(asm, cfg.tableReg, cfg.keyReg, jit.X6, cfg.lenReg, lenOff, capOff, appendLabel, cfg.missLabel, storeLabel, false)
		} else {
			emitTypedArraySetAppendPath(asm, cfg.tableReg, cfg.keyReg, jit.X6, lenOff, capOff, appendLabel, cfg.missLabel, storeLabel)
		}
		if allowSparse {
			if cfg.carryLenOnGrow {
				emitTypedArraySetSparsePathCarryLenDirty(asm, cfg.tableReg, cfg.keyReg, jit.X6, cfg.lenReg, lenOff, capOff, sparseLabel, cfg.missLabel, storeLabel, false)
			} else {
				emitTypedArraySetSparsePath(asm, cfg.tableReg, cfg.keyReg, jit.X6, lenOff, capOff, sparseLabel, cfg.missLabel, storeLabel)
			}
		}
	}

	switch cfg.kind {
	case int64(vm.FBKindMixed):
		ec.resolveValueToReg(cfg.valueID, jit.X4)
		emitBounds(false)
		asm.Label(storeLabel)
		emitLoadData()
		asm.STRreg(jit.X4, cfg.dataReg, cfg.keyReg)
		emitKeysDirty()
		emitSuccess()
		emitGrowPaths(false)

	case int64(vm.FBKindInt):
		if val, ok := ec.constInts[cfg.valueID]; ok {
			asm.LoadImm64(jit.X4, val)
		} else if ec.hasReg(cfg.valueID) && ec.valueReprOf(cfg.valueID) == valueReprRawInt {
			reg := ec.physReg(cfg.valueID)
			if reg != jit.X4 {
				asm.MOVreg(jit.X4, reg)
			}
		} else if ec.irTypes[cfg.valueID] == TypeInt {
			ec.resolveValueToReg(cfg.valueID, jit.X4)
			ec.emitUnboxInt48(jit.X4)
		} else {
			ec.resolveValueToReg(cfg.valueID, jit.X4)
			ec.emitIntTagCheckBranch(jit.X4, jit.X5, jit.X6, jit.CondNE, cfg.missLabel)
			ec.emitUnboxInt48(jit.X4)
		}
		emitBounds(false)
		asm.Label(storeLabel)
		emitLoadData()
		emitZeroValidForTypedStore()
		asm.STRreg(jit.X4, cfg.dataReg, cfg.keyReg)
		emitSuccess()
		emitGrowPaths(true)

	case int64(vm.FBKindFloat):
		valueIsTypedFloat := ec.irTypes[cfg.valueID] == TypeFloat
		valueHasRawFPR := valueIsTypedFloat && ec.hasFPReg(cfg.valueID)
		if !valueHasRawFPR {
			ec.resolveValueToReg(cfg.valueID, jit.X4)
			if !valueIsTypedFloat {
				jit.EmitIsTaggedPinned(asm, jit.X4, jit.X5, mRegTagInt)
				asm.BCond(jit.CondEQ, cfg.missLabel)
			}
		}
		emitBounds(false)
		asm.Label(storeLabel)
		emitLoadData()
		emitZeroValidForTypedStore()
		if valueHasRawFPR {
			valFPR := ec.resolveRawFloat(cfg.valueID, jit.D0)
			asm.FSTRdReg(valFPR, cfg.dataReg, cfg.keyReg)
		} else {
			asm.STRreg(jit.X4, cfg.dataReg, cfg.keyReg)
		}
		emitSuccess()
		emitGrowPaths(true)

	case int64(vm.FBKindBool):
		if boolVal, ok := ec.constBools[cfg.valueID]; ok {
			asm.MOVimm16(jit.X4, uint16(boolVal+1))
			emitBounds(false)
			asm.Label(storeLabel)
			emitLoadData()
			asm.STRBreg(jit.X4, cfg.dataReg, cfg.keyReg)
			emitKeysDirty()
			emitSuccess()
			emitGrowPaths(false)
			return true
		}

		ec.resolveValueToReg(cfg.valueID, jit.X4)
		asm.LSRimm(jit.X5, jit.X4, 48)
		asm.MOVimm16(jit.X6, uint16(jit.NB_TagBoolShr48))
		asm.CMPreg(jit.X5, jit.X6)
		boolOKLabel := ec.uniqueLabel(cfg.labelPrefix + "_bool_isbool")
		asm.BCond(jit.CondEQ, boolOKLabel)
		asm.MOVimm16(jit.X6, uint16(jit.NB_TagNilShr48))
		asm.CMPreg(jit.X5, jit.X6)
		asm.BCond(jit.CondNE, cfg.missLabel)
		asm.MOVimm16(jit.X4, 0)
		emitBounds(true)
		asm.B(storeLabel)
		asm.Label(boolOKLabel)
		asm.LoadImm64(jit.X5, 1)
		asm.ANDreg(jit.X4, jit.X4, jit.X5)
		asm.ADDimm(jit.X4, jit.X4, 1)
		emitBounds(false)
		asm.Label(storeLabel)
		emitLoadData()
		asm.STRBreg(jit.X4, cfg.dataReg, cfg.keyReg)
		emitKeysDirty()
		emitSuccess()
		emitGrowPaths(false)

	default:
		return false
	}
	return true
}

func (ec *emitContext) emitTableArrayStore(instr *Instr) {
	if len(instr.Args) < 5 {
		return
	}
	asm := ec.asm
	missLabel := ec.uniqueLabel("tarr_store_miss")
	successLabel := ec.uniqueLabel("tarr_store_success")
	doneLabel := ec.uniqueLabel("tarr_store_done")

	tblID := instr.Args[0].ID
	allowGrow := instr.Aux2&tableArrayStoreFlagAllowGrow != 0
	needsTablePtr := tableArrayStoreNeedsTablePtr(instr.Aux, instr.Aux2)
	if needsTablePtr {
		if len(instr.Args) >= 6 && instr.Args[5] != nil {
			tblReg := ec.resolveRawTablePtr(instr.Args[5].ID, jit.X0)
			if tblReg != jit.X0 {
				asm.MOVreg(jit.X0, tblReg)
			}
		} else {
			ec.resolveValueToReg(tblID, jit.X0)
			jit.EmitExtractPtr(asm, jit.X0, jit.X0)
		}
	}

	dataReg := ec.resolveRawDataPtr(instr.Args[1].ID, jit.X2)
	upperBoundSafe := !allowGrow && ec.tableArrayUpperBoundSafe(instr.ID)
	// The key helper materializes the key in X1. If the store value currently
	// lives in X1, preserve it before clobbering the register; the raw-store
	// helper may need to resolve the value after key materialization.
	ec.spillActiveGPRIfPhysReg(instr.Args[4].ID, jit.X1)
	if !ec.emitTableArrayKeyToReg(instr.Args[3], missLabel) {
		ec.emitDeopt(instr)
		return
	}
	keyID := instr.Args[3].ID
	if kv, isConst := ec.constInts[keyID]; (!isConst || kv < 0) && !ec.intNonNegative(keyID) && !ec.tableArrayLowerBoundSafe(instr.ID) {
		asm.CMPimm(jit.X1, 0)
		asm.BCond(jit.CondLT, missLabel)
	}
	priorLoadBounds := ec.tableArrayKeyBounded(instr.Args[0].ID, keyID)
	lenReg := jit.X3
	if !upperBoundSafe && !priorLoadBounds {
		lenReg = ec.resolveRawInt(instr.Args[2].ID, jit.X3)
	}

	if !ec.emitTableArrayRawStore(tableArrayRawStoreConfig{
		labelPrefix:             "tarr_store",
		kind:                    instr.Aux,
		valueID:                 instr.Args[4].ID,
		tableReg:                jit.X0,
		keyReg:                  jit.X1,
		dataReg:                 dataReg,
		lenReg:                  lenReg,
		missLabel:               missLabel,
		successLabel:            successLabel,
		priorLoadBounds:         priorLoadBounds,
		upperBoundSafe:          upperBoundSafe,
		allowGrowWithinCapacity: allowGrow,
		carryLenOnGrow:          allowGrow,
		fallthroughOnSuccess:    !allowGrow,
	}) {
		ec.emitDeopt(instr)
		return
	}

	asm.Label(successLabel)
	ec.recordTableArrayStoreBoundedKey(instr)
	asm.B(doneLabel)

	asm.Label(missLabel)
	if instr.Aux2&tableArrayStoreFlagExitResumeOnMiss != 0 {
		savedReprs := ec.snapshotValueReprs()
		ec.emitTableArrayStoreExit(instr)
		ec.emitUnboxRawIntRegs(savedReprs)
		ec.restoreValueReprSnapshot(savedReprs)
		ec.refreshTableArrayStoreFactsAfterExit(instr)
		asm.MOVimm16(jit.X17, 0)
		asm.B(doneLabel)
	} else {
		ec.emitPreciseDeopt(instr)
	}
	asm.Label(doneLabel)
}

func (ec *emitContext) spillActiveGPRIfPhysReg(valueID int, reg jit.Reg) {
	if !ec.hasReg(valueID) || ec.physReg(valueID) != reg {
		return
	}
	slot, ok := ec.slotMap[valueID]
	if !ok {
		return
	}
	ec.emitStoreGPRValueAsBoxed(valueID, reg, slot)
	delete(ec.activeRegs, valueID)
	ec.clearValueRepr(valueID)
}

func (ec *emitContext) refreshTableArrayStoreFactsAfterExit(instr *Instr) {
	if instr == nil || len(instr.Args) < 3 || instr.Args[0] == nil || instr.Args[1] == nil || instr.Args[2] == nil {
		return
	}
	dataOff, lenOff, ok := tableArrayOffsets(instr.Aux)
	if !ok {
		return
	}
	asm := ec.asm
	tblReg := ec.resolveRawTablePtr(instr.Args[0].ID, jit.X0)
	if tblReg != jit.X0 {
		asm.MOVreg(jit.X0, tblReg)
	}
	asm.LDR(jit.X16, jit.X0, dataOff)
	asm.LDR(jit.X17, jit.X0, lenOff)
	ec.storeRawDataPtr(jit.X16, instr.Args[1].ID)
	ec.storeRawInt(jit.X17, instr.Args[2].ID)
}

func (ec *emitContext) emitTableArraySwap(instr *Instr) {
	if len(instr.Args) < 5 {
		return
	}
	asm := ec.asm
	deoptLabel := ec.uniqueLabel("tarr_swap_deopt")
	doneLabel := ec.uniqueLabel("tarr_swap_done")

	ec.emitTableIntArraySpecializationKeyToReg(instr.Args[3], jit.X1, deoptLabel)
	keyAID := instr.Args[3].ID
	if kv, isConst := ec.constInts[keyAID]; (!isConst || kv < 0) && !ec.intNonNegative(keyAID) {
		asm.CMPimm(jit.X1, 0)
		asm.BCond(jit.CondLT, deoptLabel)
	}
	ec.emitTableIntArraySpecializationKeyToReg(instr.Args[4], jit.X4, deoptLabel)
	keyBID := instr.Args[4].ID
	if kv, isConst := ec.constInts[keyBID]; (!isConst || kv < 0) && !ec.intNonNegative(keyBID) {
		asm.CMPimm(jit.X4, 0)
		asm.BCond(jit.CondLT, deoptLabel)
	}
	dataReg := ec.resolveRawDataPtr(instr.Args[1].ID, jit.X2)
	lenReg := ec.resolveRawInt(instr.Args[2].ID, jit.X3)
	asm.CMPreg(jit.X1, lenReg)
	asm.BCond(jit.CondGE, deoptLabel)
	asm.CMPreg(jit.X4, lenReg)
	asm.BCond(jit.CondGE, deoptLabel)

	switch instr.Aux {
	case int64(vm.FBKindInt), int64(vm.FBKindFloat):
		asm.LDRreg(jit.X5, dataReg, jit.X1)
		asm.LDRreg(jit.X6, dataReg, jit.X4)
		asm.STRreg(jit.X6, dataReg, jit.X1)
		asm.STRreg(jit.X5, dataReg, jit.X4)
	default:
		ec.emitDeopt(instr)
		return
	}
	ec.emitGuardDeoptExit(instr, deoptLabel, doneLabel, true)
}

func (ec *emitContext) emitTableArraySwapPairs(instr *Instr) {
	if len(instr.Args) < 3 {
		return
	}
	asm := ec.asm
	failLabel := ec.uniqueLabel("tarr_swappairs_fail")
	successNoMutLabel := ec.uniqueLabel("tarr_swappairs_success_nomut")
	successMutLabel := ec.uniqueLabel("tarr_swappairs_success_mut")
	loopLabel := ec.uniqueLabel("tarr_swappairs_loop")
	doneLabel := ec.uniqueLabel("tarr_swappairs_done")

	ec.resolveValueToReg(instr.Args[0].ID, jit.X0)
	ec.emitTableIntArraySpecializationKeyToReg(instr.Args[1], jit.X1, failLabel)
	ec.emitTableIntArraySpecializationKeyToReg(instr.Args[2], jit.X4, failLabel)
	jit.EmitCheckIsTableFull(asm, jit.X0, jit.X2, jit.X3, failLabel)
	jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	asm.CBZ(jit.X0, failLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffMetatable)
	asm.CBNZ(jit.X2, failLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffLazyTree)
	asm.CBNZ(jit.X2, failLabel)
	asm.LDRB(jit.X2, jit.X0, jit.TableOffArrayKind)
	var lenOff, dataOff int
	switch instr.Aux {
	case int64(vm.FBKindInt):
		asm.CMPimm(jit.X2, jit.AKInt)
		lenOff, dataOff = jit.TableOffIntArrayLen, jit.TableOffIntArray
	case int64(vm.FBKindFloat):
		asm.CMPimm(jit.X2, jit.AKFloat)
		lenOff, dataOff = jit.TableOffFloatArrayLen, jit.TableOffFloatArray
	default:
		ec.emitDeopt(instr)
		return
	}
	asm.BCond(jit.CondNE, failLabel)

	asm.CMPreg(jit.X1, jit.X4)
	asm.BCond(jit.CondGT, successNoMutLabel)
	asm.CMPimm(jit.X1, 0)
	asm.BCond(jit.CondLT, failLabel)
	asm.LDR(jit.X3, jit.X0, lenOff)
	asm.CMPreg(jit.X4, jit.X3)
	asm.BCond(jit.CondGE, failLabel)
	asm.ADDimm(jit.X5, jit.X4, 1)
	asm.CMPreg(jit.X5, jit.X3)
	asm.BCond(jit.CondGE, failLabel)
	asm.LDR(jit.X6, jit.X0, dataOff)
	asm.CBZ(jit.X6, failLabel)

	asm.Label(loopLabel)
	asm.CMPreg(jit.X1, jit.X4)
	asm.BCond(jit.CondGT, successMutLabel)
	asm.ADDregLSL(jit.X5, jit.X6, jit.X1, 3)
	asm.LDP(jit.X7, jit.X8, jit.X5, 0)
	asm.STP(jit.X8, jit.X7, jit.X5, 0)
	asm.ADDimm(jit.X1, jit.X1, 2)
	asm.B(loopLabel)

	asm.Label(successMutLabel)
	asm.MOVimm16(jit.X7, 1)
	asm.STRB(jit.X7, jit.X0, jit.TableOffKeysDirty)
	asm.Label(successNoMutLabel)
	asm.ADDimm(jit.X0, mRegTagBool, 1)
	ec.storeResultNB(jit.X0, instr.ID)
	asm.B(doneLabel)

	asm.Label(failLabel)
	asm.MOVreg(jit.X0, mRegTagBool)
	ec.storeResultNB(jit.X0, instr.ID)
	asm.Label(doneLabel)
	ec.fuseBoolResultBitTest(instr.ID, jit.X0)
}

func (ec *emitContext) fuseBoolResultBitTest(valueID int, fallback jit.Reg) {
	if ec == nil {
		return
	}
	reg := fallback
	if pr, ok := ec.alloc.ValueRegs[valueID]; ok && !pr.IsFloat {
		reg = jit.Reg(pr.Reg)
	}
	ec.fusedBitTestReg = reg
	ec.fusedBitTestBit = 0
	ec.fusedBitTestZero = false
	ec.fusedBitTestActive = true
}

func tableArrayStoreNeedsTablePtr(kind, flags int64) bool {
	return flags&tableArrayStoreFlagAllowGrow != 0 ||
		kind == int64(vm.FBKindMixed) ||
		kind == int64(vm.FBKindBool) ||
		kind == int64(vm.FBKindInt) ||
		kind == int64(vm.FBKindFloat)
}

func (ec *emitContext) recordTableArrayStoreBoundedKey(instr *Instr) {
	if ec == nil || instr == nil || len(instr.Args) < 4 || instr.Args[0] == nil || instr.Args[3] == nil {
		return
	}
	if ec.tableArrayBoundedKeys == nil {
		ec.tableArrayBoundedKeys = make(map[tableArrayBoundKey]bool, 1)
	}
	ec.asm.MOVimm16(jit.X17, 1)
	ec.tableArrayBoundedKeys[tableArrayBoundKey{tableID: instr.Args[0].ID, keyID: instr.Args[3].ID}] = true
}

// emitSetTableNative emits a native ARM64 fast path for OpSetTable with
// deopt fallback to exit-resume. The fast path handles integer keys with
// bounds-checked store to the table's array part (both Mixed and Int kinds).
// Non-integer keys, tables with metatables, and out-of-bounds access fall
// through to the exit-resume slow path.
//
// Instr layout:
//   - Args[0] = table value (NaN-boxed)
//   - Args[1] = key value (NaN-boxed)
//   - Args[2] = value to store (NaN-boxed)
func (ec *emitContext) emitSetTableNative(instr *Instr) {
	asm := ec.asm
	deoptLabel := ec.uniqueLabel("settable_deopt")
	doneLabel := ec.uniqueLabel("settable_done")
	intArrayLabel := ec.uniqueLabel("settable_intarr")
	boolArrayLabel := ec.uniqueLabel("settable_boolarr")
	floatArrayLabel := ec.uniqueLabel("settable_floatarr")

	// Load table value (NaN-boxed) into X0.
	tblValueID := instr.Args[0].ID
	ec.resolveValueToReg(tblValueID, jit.X0)

	if ec.tableVerified[tblValueID] {
		// Table already validated in this block — skip type/nil/metatable checks.
		// Just extract the raw pointer.
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	} else if ec.isLocalNewTableWithoutMetatable(instr.Args[0]) {
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
		ec.tableVerified[tblValueID] = true
	} else if ec.irTypes[tblValueID] == TypeTable {
		// The producer already guards/proves table-ness. Re-check the dynamic
		// metatable because table identity can still carry metamethods.
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
		asm.CBZ(jit.X0, deoptLabel)
		asm.LDR(jit.X1, jit.X0, jit.TableOffMetatable)
		asm.CBNZ(jit.X1, deoptLabel)
		ec.tableVerified[tblValueID] = true
	} else {
		// Full validation.
		jit.EmitCheckIsTableFull(asm, jit.X0, jit.X1, jit.X2, deoptLabel)
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
		asm.CBZ(jit.X0, deoptLabel)
		asm.LDR(jit.X1, jit.X0, jit.TableOffMetatable)
		asm.CBNZ(jit.X1, deoptLabel)
		ec.tableVerified[tblValueID] = true
	}

	// Load key into X1 with type-specialized fast paths.
	keyID := instr.Args[1].ID
	ec.emitDynamicStringSetTableCache(instr, doneLabel)

	if kv, isConst := ec.constInts[keyID]; isConst {
		// R98: const int key — direct immediate load.
		asm.LoadImm64(jit.X1, kv)
	} else if ec.hasReg(keyID) && ec.valueReprOf(keyID) == valueReprRawInt {
		// Fast path 1: key is raw int in a register.
		reg := ec.physReg(keyID)
		if reg != jit.X1 {
			asm.MOVreg(jit.X1, reg)
		}
		// Key is already a raw int64 — skip boxing, tag check, and unbox.
	} else if ec.irTypes[keyID] == TypeInt {
		// Fast path 2: key is known TypeInt but NaN-boxed — skip tag check, just unbox.
		ec.resolveValueToReg(keyID, jit.X1)
		ec.emitUnboxInt48(jit.X1)
	} else {
		// Slow path: full NaN-boxed key with tag check.
		ec.resolveValueToReg(keyID, jit.X1)
		ec.emitIntTagCheckBranch(jit.X1, jit.X2, jit.X3, jit.CondNE, deoptLabel)
		ec.emitUnboxInt48(jit.X1)
	}

	// Check key >= 0 (shared by all paths). R97: skip when key is a
	// ConstInt with a non-negative compile-time value.
	if kv, isConst := ec.constInts[keyID]; (!isConst || kv < 0) && !ec.intNonNegative(keyID) {
		asm.CMPimm(jit.X1, 0)
		asm.BCond(jit.CondLT, deoptLabel)
	}
	keyBoundsAlreadyChecked := ec.tableArrayKeyBounded(tblValueID, keyID)

	// Mixed array stores of table rows are the construction side of ordinary
	// table-of-row arrays. Prefer the dense-matrix append path when its full
	// contract is already present, but let non-dense row arrays fall through to
	// the generic mixed-array append/store fast path instead of forcing one
	// exit per row.
	if instr.Aux2 == int64(vm.FBKindMixed) && ec.irTypes[instr.Args[2].ID] == TypeTable {
		denseMissLabel := ec.uniqueLabel("settable_dense_row_miss")
		ec.emitDenseMatrixRowAppendFastPath(instr, denseMissLabel, doneLabel)
		asm.Label(denseMissLabel)
	}

	// Kind-specialized dispatch: when Aux2 carries feedback, emit a kind
	// guard instead of the 4-way cascade. When the same (table, kind) pair
	// has already been verified earlier in this block, skip the guard
	// entirely and omit the mixed fallback that the guard cannot reach.
	mixedArrayLabel := ec.uniqueLabel("settable_mixedarr")
	knownSetKind := int(instr.Aux2) // 0=unknown, 1..4=known FBKind
	expectedKind, hasKnownSetKind := fbKindToAK(instr.Aux2)
	kindAlreadyVerified := hasKnownSetKind && ec.kindVerified[tblValueID] == uint16(knownSetKind)
	emitMixedArrayPath := !hasKnownSetKind || expectedKind == jit.AKMixed || !kindAlreadyVerified
	emitIntArrayPath := !hasKnownSetKind || expectedKind == jit.AKInt
	emitFloatArrayPath := !hasKnownSetKind || expectedKind == jit.AKFloat
	emitBoolArrayPath := !hasKnownSetKind || expectedKind == jit.AKBool
	fastPathAlwaysWritesKeysDirty := !emitIntArrayPath && !emitFloatArrayPath
	if hasKnownSetKind {
		if fbKind, ok := ec.localNewTableFBKind(instr.Args[0]); ok && fbKind == uint16(knownSetKind) {
			ec.kindVerified[tblValueID] = uint16(knownSetKind)
		}
		if ec.kindVerified[tblValueID] != uint16(knownSetKind) {
			asm.LDRB(jit.X2, jit.X0, jit.TableOffArrayKind)
			asm.CMPimm(jit.X2, expectedKind)
			cacheKind := expectedKind == jit.AKMixed
			if expectedKind == jit.AKMixed {
				asm.BCond(jit.CondNE, deoptLabel) // kind mismatch → deopt
			} else {
				setKindOKLabel := ec.uniqueLabel("settable_kind_ok")
				asm.BCond(jit.CondEQ, setKindOKLabel)
				asm.CMPimm(jit.X2, jit.AKMixed)
				asm.BCond(jit.CondEQ, mixedArrayLabel)
				asm.B(deoptLabel)
				asm.Label(setKindOKLabel)
			}
			if cacheKind {
				ec.kindVerified[tblValueID] = uint16(knownSetKind)
			}
		}
		// Jump directly to the matching kind path.
		switch expectedKind {
		case jit.AKMixed:
			asm.B(mixedArrayLabel)
		case jit.AKInt:
			asm.B(intArrayLabel)
		case jit.AKFloat:
			asm.B(floatArrayLabel)
		case jit.AKBool:
			asm.B(boolArrayLabel)
		}
	} else {
		// Unknown kind: use existing 4-way dispatch cascade.
		asm.LDRB(jit.X2, jit.X0, jit.TableOffArrayKind)
		asm.CMPimm(jit.X2, jit.AKBool)
		asm.BCond(jit.CondEQ, boolArrayLabel)
		asm.CMPimm(jit.X2, jit.AKFloat)
		asm.BCond(jit.CondEQ, floatArrayLabel)
		asm.CMPimm(jit.X2, jit.AKInt)
		asm.BCond(jit.CondEQ, intArrayLabel)
		asm.CBNZ(jit.X2, deoptLabel) // not Mixed(0) -> deopt
	}

	if emitMixedArrayPath {
		// --- ArrayMixed fast path ---
		asm.Label(mixedArrayLabel)
		if !ec.emitTableArrayRawStore(tableArrayRawStoreConfig{
			labelPrefix:             "settable_mixed",
			kind:                    int64(vm.FBKindMixed),
			valueID:                 instr.Args[2].ID,
			tableReg:                jit.X0,
			keyReg:                  jit.X1,
			dataReg:                 jit.X2,
			lenReg:                  jit.X2,
			missLabel:               deoptLabel,
			successLabel:            doneLabel,
			loadDataFromTable:       true,
			priorLoadBounds:         keyBoundsAlreadyChecked,
			keysDirtyAlreadyWritten: ec.keysDirtyWritten[tblValueID],
			allowGrowWithinCapacity: true,
		}) {
			ec.emitDeopt(instr)
			return
		}
	}

	// --- ArrayInt fast path ---
	if emitIntArrayPath {
		asm.Label(intArrayLabel)
		if !ec.emitTableArrayRawStore(tableArrayRawStoreConfig{
			labelPrefix:             "settable_int",
			kind:                    int64(vm.FBKindInt),
			valueID:                 instr.Args[2].ID,
			tableReg:                jit.X0,
			keyReg:                  jit.X1,
			dataReg:                 jit.X2,
			lenReg:                  jit.X2,
			missLabel:               deoptLabel,
			successLabel:            doneLabel,
			loadDataFromTable:       true,
			priorLoadBounds:         keyBoundsAlreadyChecked,
			keysDirtyAlreadyWritten: ec.keysDirtyWritten[tblValueID],
			allowGrowWithinCapacity: true,
		}) {
			ec.emitDeopt(instr)
			return
		}
	}

	if emitFloatArrayPath {
		// --- ArrayFloat fast path ---
		asm.Label(floatArrayLabel)
		if !ec.emitTableArrayRawStore(tableArrayRawStoreConfig{
			labelPrefix:             "settable_float",
			kind:                    int64(vm.FBKindFloat),
			valueID:                 instr.Args[2].ID,
			tableReg:                jit.X0,
			keyReg:                  jit.X1,
			dataReg:                 jit.X2,
			lenReg:                  jit.X2,
			missLabel:               deoptLabel,
			successLabel:            doneLabel,
			loadDataFromTable:       true,
			priorLoadBounds:         keyBoundsAlreadyChecked,
			keysDirtyAlreadyWritten: ec.keysDirtyWritten[tblValueID],
			allowGrowWithinCapacity: true,
		}) {
			ec.emitDeopt(instr)
			return
		}
	}

	if emitBoolArrayPath {
		// --- ArrayBool fast path ---
		asm.Label(boolArrayLabel)
		if !ec.emitTableArrayRawStore(tableArrayRawStoreConfig{
			labelPrefix:             "settable_bool",
			kind:                    int64(vm.FBKindBool),
			valueID:                 instr.Args[2].ID,
			tableReg:                jit.X0,
			keyReg:                  jit.X1,
			dataReg:                 jit.X2,
			lenReg:                  jit.X2,
			missLabel:               deoptLabel,
			successLabel:            doneLabel,
			loadDataFromTable:       true,
			priorLoadBounds:         keyBoundsAlreadyChecked,
			keysDirtyAlreadyWritten: ec.keysDirtyWritten[tblValueID],
			allowGrowWithinCapacity: true,
		}) {
			ec.emitDeopt(instr)
			return
		}
	}

	// Deopt: fall back to exit-resume.
	asm.Label(deoptLabel)
	savedReprs := ec.snapshotValueReprs()
	ec.emitSetTableExit(instr)
	ec.emitUnboxRawIntRegs(savedReprs)
	ec.restoreValueReprSnapshot(savedReprs)

	asm.Label(doneLabel)
	// Runtime exit-resume can invoke metamethods or demote unknown tables, so
	// most writes invalidate table/kind facts. A local NewTable in a function
	// with no metatable mutation surface keeps its facts only when the store
	// value is proven compatible with the typed backing, so even a slow-path
	// sparse/append store cannot demote the array kind.
	if ec.setTablePreservesLocalArrayFacts(instr) {
		ec.tableVerified[tblValueID] = true
		if hasKnownSetKind {
			ec.kindVerified[tblValueID] = uint16(knownSetKind)
		}
	} else {
		delete(ec.tableVerified, tblValueID)
		delete(ec.kindVerified, tblValueID)
	}
	// keysDirty is idempotent. Record only when every native path writes it;
	// typed int/float in-bounds overwrites intentionally skip it because they
	// do not change the table's key set.
	if fastPathAlwaysWritesKeysDirty {
		ec.keysDirtyWritten[tblValueID] = true
	} else {
		delete(ec.keysDirtyWritten, tblValueID)
	}
}
