//go:build darwin && arm64

// emit_table_array_load.go: array/index load code paths (OpTableArrayHeader
// data/len accessors, OpTableArrayLoad, nested loads, OpGetTable) and their
// load-side helpers and bounded-key recording. Pure code movement from
// emit_table_array.go; no behavior change.

package methodjit

import (
	"github.com/gscript/gscript/internal/jit"
	"github.com/gscript/gscript/internal/vm"
)

func (ec *emitContext) emitTableArrayLen(instr *Instr) {
	if len(instr.Args) < 1 {
		return
	}
	_, lenOff, ok := tableArrayOffsets(instr.Aux)
	if !ok {
		ec.emitDeopt(instr)
		return
	}
	hdr := ec.resolveRawTablePtr(instr.Args[0].ID, jit.X0)
	if hdr != jit.X0 {
		ec.asm.MOVreg(jit.X0, hdr)
	}
	ec.asm.LDR(jit.X0, jit.X0, lenOff)
	ec.storeRawInt(jit.X0, instr.ID)
}

func (ec *emitContext) emitTableArrayData(instr *Instr) {
	if len(instr.Args) < 1 {
		return
	}
	dataOff, _, ok := tableArrayOffsets(instr.Aux)
	if !ok {
		ec.emitDeopt(instr)
		return
	}
	hdr := ec.resolveRawTablePtr(instr.Args[0].ID, jit.X0)
	if hdr != jit.X0 {
		ec.asm.MOVreg(jit.X0, hdr)
	}
	ec.asm.LDR(jit.X0, jit.X0, dataOff)
	ec.storeRawDataPtr(jit.X0, instr.ID)
}

func (ec *emitContext) emitTableArrayLoad(instr *Instr) {
	if len(instr.Args) < 3 {
		return
	}
	asm := ec.asm
	deoptLabel := ec.uniqueLabel("tarr_load_deopt")
	successLabel := ec.uniqueLabel("tarr_load_success")
	doneLabel := ec.uniqueLabel("tarr_load_done")

	dataReg := ec.resolveRawDataPtr(instr.Args[0].ID, jit.X2)
	if tableArrayLoadScratchClobbers(dataReg) {
		asm.MOVreg(jit.X16, dataReg)
		dataReg = jit.X16
	}
	keyID := instr.Args[2].ID
	if kv, isConst := ec.constInts[keyID]; isConst {
		asm.LoadImm64(jit.X1, kv)
	} else if ec.hasReg(keyID) && ec.valueReprOf(keyID) == valueReprRawInt {
		reg := ec.physReg(keyID)
		if reg != jit.X1 {
			asm.MOVreg(jit.X1, reg)
		}
	} else if ec.irTypes[keyID] == TypeInt {
		keyReg := ec.resolveValueNB(keyID, jit.X1)
		if keyReg != jit.X1 {
			asm.MOVreg(jit.X1, keyReg)
		}
		ec.emitUnboxInt48(jit.X1)
	} else {
		keyReg := ec.resolveValueNB(keyID, jit.X1)
		if keyReg != jit.X1 {
			asm.MOVreg(jit.X1, keyReg)
		}
		ec.emitIntTagCheckBranch(jit.X1, jit.X4, jit.X5, jit.CondNE, deoptLabel)
		ec.emitUnboxInt48(jit.X1)
	}
	if kv, isConst := ec.constInts[keyID]; (!isConst || kv < 0) && !ec.intNonNegative(keyID) && !ec.tableArrayLowerBoundSafe(instr.ID) {
		asm.CMPimm(jit.X1, 0)
		asm.BCond(jit.CondLT, deoptLabel)
	}
	if !ec.tableArrayUpperBoundSafe(instr.ID) {
		lenReg := ec.resolveRawInt(instr.Args[1].ID, jit.X3)
		if tableArrayLoadScratchClobbers(lenReg) || lenReg == dataReg {
			asm.MOVreg(jit.X17, lenReg)
			lenReg = jit.X17
		}
		asm.CMPreg(jit.X1, lenReg)
		asm.BCond(jit.CondGE, deoptLabel)
	}
	if tableArrayLoadNeedsZeroValidGuard(instr) && !ec.tableArrayKeyKnownNonZero(keyID) {
		zeroKeyOKLabel := ec.uniqueLabel("tarr_load_zero_key_ok")
		zeroKeyDoneLabel := ec.uniqueLabel("tarr_load_zero_key_done")
		asm.CMPimm(jit.X1, 0)
		asm.BCond(jit.CondNE, zeroKeyDoneLabel)
		if headerValue, ok := tableArrayLoadHeaderValue(instr); ok {
			tblReg := ec.resolveRawTablePtr(headerValue.ID, jit.X4)
			if tblReg != jit.X4 {
				asm.MOVreg(jit.X4, tblReg)
			}
			asm.LDRB(jit.X5, jit.X4, jit.TableOffArrayZeroValid)
			asm.CBNZ(jit.X5, zeroKeyOKLabel)
		}
		asm.B(deoptLabel)
		asm.Label(zeroKeyOKLabel)
		asm.Label(zeroKeyDoneLabel)
	}

	switch instr.Aux {
	case int64(vm.FBKindMixed):
		asm.LDRreg(jit.X0, dataReg, jit.X1)
		switch instr.Type {
		case TypeInt:
			ec.emitIntTagCheckBranch(jit.X0, jit.X2, jit.X3, jit.CondNE, deoptLabel)
			ec.emitUnboxInt48(jit.X0)
			ec.storeRawInt(jit.X0, instr.ID)
		case TypeFloat:
			jit.EmitIsTaggedPinned(asm, jit.X0, jit.X2, mRegTagInt)
			asm.BCond(jit.CondEQ, deoptLabel)
			asm.FMOVtoFP(jit.D0, jit.X0)
			ec.storeRawFloat(jit.D0, instr.ID)
		case TypeTable:
			jit.EmitCheckIsTableFull(asm, jit.X0, jit.X2, jit.X3, deoptLabel)
			jit.EmitExtractPtr(asm, jit.X0, jit.X0)
			ec.storeRawTablePtr(jit.X0, instr.ID)
		case TypeString:
			if instr.Aux2&tableArrayLoadFlagProvenString == 0 {
				jit.EmitCheckIsString(asm, jit.X0, jit.X2, jit.X3, deoptLabel)
			}
			ec.storeResultNB(jit.X0, instr.ID)
		default:
			ec.storeResultNB(jit.X0, instr.ID)
		}
	case int64(vm.FBKindInt):
		if instr.Type == TypeInt {
			dst := jit.X0
			if pr, ok := ec.alloc.ValueRegs[instr.ID]; ok && !pr.IsFloat {
				dst = jit.Reg(pr.Reg)
			}
			asm.LDRreg(dst, dataReg, jit.X1)
			ec.storeRawInt(dst, instr.ID)
		} else {
			asm.LDRreg(jit.X0, dataReg, jit.X1)
			jit.EmitBoxIntFast(asm, jit.X0, jit.X0, mRegTagInt)
			ec.storeResultNB(jit.X0, instr.ID)
		}
	case int64(vm.FBKindFloat):
		if instr.Type == TypeFloat {
			dstF := jit.D0
			if pr, ok := ec.alloc.ValueRegs[instr.ID]; ok && pr.IsFloat {
				dstF = jit.FReg(pr.Reg)
			}
			asm.FLDRdReg(dstF, dataReg, jit.X1)
			ec.storeRawFloat(dstF, instr.ID)
		} else {
			asm.LDRreg(jit.X0, dataReg, jit.X1)
			ec.storeResultNB(jit.X0, instr.ID)
		}
	case int64(vm.FBKindBool):
		asm.LDRBreg(jit.X3, dataReg, jit.X1)
		nilLabel := ec.uniqueLabel("tarr_bool_nil")
		falseLabel := ec.uniqueLabel("tarr_bool_false")
		asm.CBZ(jit.X3, nilLabel)
		asm.CMPimm(jit.X3, 1)
		asm.BCond(jit.CondEQ, falseLabel)
		asm.LoadImm64(jit.X0, nb64(jit.NB_TagBool|1))
		ec.storeResultNB(jit.X0, instr.ID)
		asm.B(successLabel)
		asm.Label(falseLabel)
		asm.LoadImm64(jit.X0, nb64(jit.NB_TagBool))
		ec.storeResultNB(jit.X0, instr.ID)
		asm.B(successLabel)
		asm.Label(nilLabel)
		asm.LoadImm64(jit.X0, nb64(jit.NB_ValNil))
		ec.storeResultNB(jit.X0, instr.ID)
	default:
		ec.emitDeopt(instr)
		return
	}

	asm.Label(successLabel)
	ec.recordTableArrayBoundedKey(instr)
	asm.B(doneLabel)

	asm.Label(deoptLabel)
	savedReprs := ec.snapshotValueReprs()
	if ec.emitTableArrayLoadExit(instr) {
		typeDeoptLabel := ec.uniqueLabel("tarr_load_exit_type_deopt")
		ec.emitCheckTableArrayLoadExitResult(instr, typeDeoptLabel)
		ec.emitUnboxRawIntRegs(savedReprs)
		ec.restoreValueReprSnapshot(savedReprs)
		asm.MOVimm16(jit.X17, 0)
		asm.B(doneLabel)
		asm.Label(typeDeoptLabel)
		ec.emitPreciseDeopt(instr)
	}
	asm.Label(doneLabel)
}

func (ec *emitContext) emitTableArrayNestedLoad(instr *Instr) {
	if len(instr.Args) < 5 {
		return
	}
	asm := ec.asm
	deoptLabel := ec.uniqueLabel("tarr_nested_deopt")
	doneLabel := ec.uniqueLabel("tarr_nested_done")
	normalLabel := ec.uniqueLabel("tarr_nested_normal")

	rowDataOff, rowLenOff, ok := tableArrayOffsets(instr.Aux)
	if !ok || !tableArrayNestedLoadSupported(instr.Aux, instr.Type) {
		ec.emitDeopt(instr)
		return
	}
	expectedRowKind, ok := fbKindToAK(instr.Aux)
	if !ok {
		ec.emitDeopt(instr)
		return
	}

	if instr.Aux == int64(vm.FBKindFloat) {
		outerTblReg := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
		if outerTblReg != jit.X0 {
			asm.MOVreg(jit.X0, outerTblReg)
		}
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
		asm.LDRW(jit.X6, jit.X0, jit.TableOffDMStride)
		asm.CBZ(jit.X6, normalLabel)
		if !ec.emitTableArrayKeyToReg(instr.Args[3], deoptLabel) {
			ec.emitDeopt(instr)
			return
		}
		outerKeyID := instr.Args[3].ID
		if kv, isConst := ec.constInts[outerKeyID]; (!isConst || kv < 0) && !ec.intNonNegative(outerKeyID) {
			asm.CMPimm(jit.X1, 0)
			asm.BCond(jit.CondLT, deoptLabel)
		}
		asm.MOVreg(jit.X2, jit.X1)
		outerLenReg := ec.resolveRawInt(instr.Args[2].ID, jit.X3)
		if outerLenReg != jit.X3 {
			asm.MOVreg(jit.X3, outerLenReg)
		}
		asm.CMPreg(jit.X2, jit.X3)
		asm.BCond(jit.CondGE, deoptLabel)
		if !ec.emitTableArrayKeyToReg(instr.Args[4], deoptLabel) {
			ec.emitDeopt(instr)
			return
		}
		innerKeyID := instr.Args[4].ID
		if kv, isConst := ec.constInts[innerKeyID]; (!isConst || kv < 0) && !ec.intNonNegative(innerKeyID) {
			asm.CMPimm(jit.X1, 0)
			asm.BCond(jit.CondLT, deoptLabel)
		}
		asm.CMPreg(jit.X1, jit.X6)
		asm.BCond(jit.CondGE, deoptLabel)
		asm.MUL(jit.X4, jit.X2, jit.X6)
		asm.ADDreg(jit.X4, jit.X4, jit.X1)
		asm.LDR(jit.X5, jit.X0, jit.TableOffDMFlat)
		dstF := jit.D0
		if pr, ok := ec.alloc.ValueRegs[instr.ID]; ok && pr.IsFloat {
			dstF = jit.FReg(pr.Reg)
		}
		asm.FLDRdReg(dstF, jit.X5, jit.X4)
		ec.storeRawFloat(dstF, instr.ID)
		asm.B(doneLabel)
	}

	asm.Label(normalLabel)
	outerDataReg := ec.resolveRawDataPtr(instr.Args[1].ID, jit.X2)
	if outerDataReg != jit.X2 {
		asm.MOVreg(jit.X2, outerDataReg)
	}
	outerLenReg := ec.resolveRawInt(instr.Args[2].ID, jit.X3)
	if outerLenReg != jit.X3 {
		asm.MOVreg(jit.X3, outerLenReg)
	}
	if !ec.emitTableArrayKeyToReg(instr.Args[3], deoptLabel) {
		ec.emitDeopt(instr)
		return
	}
	outerKeyID := instr.Args[3].ID
	if kv, isConst := ec.constInts[outerKeyID]; (!isConst || kv < 0) && !ec.intNonNegative(outerKeyID) {
		asm.CMPimm(jit.X1, 0)
		asm.BCond(jit.CondLT, deoptLabel)
	}
	asm.CMPreg(jit.X1, jit.X3)
	asm.BCond(jit.CondGE, deoptLabel)

	asm.LDRreg(jit.X0, jit.X2, jit.X1)
	jit.EmitCheckIsTableFull(asm, jit.X0, jit.X2, jit.X3, deoptLabel)
	jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	asm.CBZ(jit.X0, deoptLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffMetatable)
	asm.CBNZ(jit.X2, deoptLabel)
	asm.LDRB(jit.X2, jit.X0, jit.TableOffArrayKind)
	asm.CMPimm(jit.X2, expectedRowKind)
	asm.BCond(jit.CondNE, deoptLabel)
	asm.LDR(jit.X3, jit.X0, rowLenOff)
	asm.LDR(jit.X2, jit.X0, rowDataOff)

	if !ec.emitTableArrayKeyToReg(instr.Args[4], deoptLabel) {
		ec.emitDeopt(instr)
		return
	}
	innerKeyID := instr.Args[4].ID
	if kv, isConst := ec.constInts[innerKeyID]; (!isConst || kv < 0) && !ec.intNonNegative(innerKeyID) {
		asm.CMPimm(jit.X1, 0)
		asm.BCond(jit.CondLT, deoptLabel)
	}
	asm.CMPreg(jit.X1, jit.X3)
	asm.BCond(jit.CondGE, deoptLabel)
	switch instr.Aux {
	case int64(vm.FBKindInt):
		asm.LDRreg(jit.X0, jit.X2, jit.X1)
		ec.storeRawInt(jit.X0, instr.ID)
	case int64(vm.FBKindFloat):
		dstF := jit.D0
		if pr, ok := ec.alloc.ValueRegs[instr.ID]; ok && pr.IsFloat {
			dstF = jit.FReg(pr.Reg)
		}
		asm.FLDRdReg(dstF, jit.X2, jit.X1)
		ec.storeRawFloat(dstF, instr.ID)
	default:
		ec.emitDeopt(instr)
		return
	}
	ec.emitGuardDeoptExit(instr, deoptLabel, doneLabel, true)
}

func tableArrayLoadScratchClobbers(reg jit.Reg) bool {
	switch reg {
	case jit.X1, jit.X4, jit.X5:
		return true
	default:
		return false
	}
}

func tableArrayLoadNeedsZeroValidGuard(instr *Instr) bool {
	if instr == nil {
		return false
	}
	// Typed-array loads are only lowered after table feedback has observed a
	// present numeric element at the access site. Stores must maintain
	// arrayZeroValid for key 0 so later generic reads preserve nil-vs-zero
	// semantics; the load fast path can then use the typed backing directly.
	return false
}

func tableArrayLoadHeaderValue(instr *Instr) (*Value, bool) {
	if instr == nil || len(instr.Args) < 1 || instr.Args[0] == nil {
		return nil, false
	}
	if len(instr.Args) >= 4 && instr.Args[3] != nil {
		return instr.Args[3], true
	}
	data := instr.Args[0].Def
	if data == nil || data.Op != OpTableArrayData || len(data.Args) < 1 || data.Args[0] == nil {
		return nil, false
	}
	return data.Args[0], true
}

func tableArrayLoadTableValue(instr *Instr) (*Value, bool) {
	if instr == nil || len(instr.Args) < 1 || instr.Args[0] == nil {
		return nil, false
	}
	headerValue, ok := tableArrayLoadHeaderValue(instr)
	if !ok {
		return nil, false
	}
	header := headerValue.Def
	if header == nil || header.Op != OpTableArrayHeader || len(header.Args) < 1 || header.Args[0] == nil {
		return nil, false
	}
	return header.Args[0], true
}

func (ec *emitContext) emitCheckTableArrayLoadExitResult(instr *Instr, deoptLabel string) {
	if instr == nil {
		return
	}
	asm := ec.asm
	switch instr.Aux {
	case int64(vm.FBKindMixed):
		switch instr.Type {
		case TypeInt:
			ec.emitIntTagCheckBranch(jit.X0, jit.X2, jit.X3, jit.CondNE, deoptLabel)
		case TypeFloat:
			jit.EmitIsTaggedPinned(asm, jit.X0, jit.X2, mRegTagInt)
			asm.BCond(jit.CondEQ, deoptLabel)
		case TypeTable:
			jit.EmitCheckIsTableFull(asm, jit.X0, jit.X2, jit.X3, deoptLabel)
		case TypeString:
			jit.EmitCheckIsString(asm, jit.X0, jit.X2, jit.X3, deoptLabel)
		}
	case int64(vm.FBKindInt):
		if instr.Type == TypeInt {
			ec.emitIntTagCheckBranch(jit.X0, jit.X2, jit.X3, jit.CondNE, deoptLabel)
		}
	case int64(vm.FBKindFloat):
		if instr.Type == TypeFloat {
			jit.EmitIsTaggedPinned(asm, jit.X0, jit.X2, mRegTagInt)
			asm.BCond(jit.CondEQ, deoptLabel)
		}
	}
}

func (ec *emitContext) recordTableArrayBoundedKey(instr *Instr) {
	if ec == nil || instr == nil || len(instr.Args) < 3 || instr.Args[2] == nil {
		return
	}
	tableValue, ok := tableArrayLoadTableValue(instr)
	if !ok || tableValue == nil {
		return
	}
	if !tableArrayLoadFeedsStore(instr, tableValue.ID, instr.Args[2].ID) {
		return
	}
	if ec.tableArrayBoundedKeys == nil {
		ec.tableArrayBoundedKeys = make(map[tableArrayBoundKey]bool, 1)
	}
	ec.asm.MOVimm16(jit.X17, 1)
	ec.tableArrayBoundedKeys[tableArrayBoundKey{tableID: tableValue.ID, keyID: instr.Args[2].ID}] = true
}

func tableArrayLoadFeedsStore(load *Instr, tableID, keyID int) bool {
	if load == nil || load.Block == nil {
		return false
	}
	seenLoad := false
	for _, instr := range load.Block.Instrs {
		if instr == nil {
			continue
		}
		if instr == load {
			seenLoad = true
			continue
		}
		if !seenLoad {
			continue
		}
		if instr.Op != OpTableArrayStore || len(instr.Args) < 4 || instr.Args[0] == nil || instr.Args[3] == nil {
			continue
		}
		if instr.Args[0].ID == tableID && instr.Args[3].ID == keyID {
			return true
		}
	}
	return false
}

// emitGetTableNative emits a native ARM64 fast path for OpGetTable with
// deopt fallback to exit-resume. The fast path handles integer keys with
// bounds-checked access to the table's array part (both Mixed and Int kinds).
// Non-integer keys, tables with metatables, and out-of-bounds access fall
// through to the exit-resume slow path.
//
// Instr layout:
//   - Args[0] = table value (NaN-boxed)
//   - Args[1] = key value (NaN-boxed)
func (ec *emitContext) emitGetTableNative(instr *Instr) {
	asm := ec.asm
	deoptLabel := ec.uniqueLabel("gettable_deopt")
	doneLabel := ec.uniqueLabel("gettable_done")
	intArrayLabel := ec.uniqueLabel("gettable_intarr")
	boolArrayLabel := ec.uniqueLabel("gettable_boolarr")
	floatArrayLabel := ec.uniqueLabel("gettable_floatarr")

	// Load table value (NaN-boxed) into X0.
	tblValueID := instr.Args[0].ID
	tblReg := ec.resolveValueNB(tblValueID, jit.X0)
	if tblReg != jit.X0 {
		asm.MOVreg(jit.X0, tblReg)
	}

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
	ec.emitDynamicStringGetTableCache(instr, doneLabel)

	if kv, isConst := ec.constInts[keyID]; isConst {
		// R98: const int key — load the immediate directly, bypass reg
		// resolution, tag check, and unbox.
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
		keyReg := ec.resolveValueNB(keyID, jit.X1)
		if keyReg != jit.X1 {
			asm.MOVreg(jit.X1, keyReg)
		}
		ec.emitUnboxInt48(jit.X1)
	} else {
		// Slow path: full NaN-boxed key with tag check.
		keyReg := ec.resolveValueNB(keyID, jit.X1)
		if keyReg != jit.X1 {
			asm.MOVreg(jit.X1, keyReg)
		}
		ec.emitIntTagCheckBranch(jit.X1, jit.X2, jit.X3, jit.CondNE, deoptLabel)
		ec.emitUnboxInt48(jit.X1)
	}

	// Check key >= 0 (shared by all paths). R97: skip when key is a
	// ConstInt with a non-negative compile-time value.
	if kv, isConst := ec.constInts[keyID]; (!isConst || kv < 0) && !ec.intNonNegative(keyID) {
		asm.CMPimm(jit.X1, 0)
		asm.BCond(jit.CondLT, deoptLabel)
	}

	// Kind-specialized dispatch: when Aux2 carries feedback, emit a kind
	// guard (3 insns) instead of the 4-way cascade (8 insns). When the
	// same (table, kind) pair has already been verified earlier in this
	// block, skip the guard entirely — emit only the direct jump.
	mixedArrayLabel := ec.uniqueLabel("gettable_mixedarr")
	knownGetKind := int(instr.Aux2) // 0=unknown, 1..4=known FBKind
	if knownGetKind >= 1 && knownGetKind <= 4 {
		expectedKind := uint16(knownGetKind - 1) // convert FBKind to AK constant
		if fbKind, ok := ec.localNewTableFBKind(instr.Args[0]); ok && fbKind == uint16(knownGetKind) {
			ec.kindVerified[tblValueID] = uint16(knownGetKind)
		}
		if ec.kindVerified[tblValueID] != uint16(knownGetKind) {
			asm.LDRB(jit.X2, jit.X0, jit.TableOffArrayKind)
			asm.CMPimm(jit.X2, expectedKind)
			cacheKind := expectedKind == jit.AKMixed
			if expectedKind == jit.AKMixed {
				asm.BCond(jit.CondNE, deoptLabel) // kind mismatch → deopt
			} else {
				getKindOKLabel := ec.uniqueLabel("gettable_kind_ok")
				asm.BCond(jit.CondEQ, getKindOKLabel)
				asm.CMPimm(jit.X2, jit.AKMixed)
				asm.BCond(jit.CondEQ, mixedArrayLabel)
				asm.B(deoptLabel)
				asm.Label(getKindOKLabel)
			}
			if cacheKind {
				ec.kindVerified[tblValueID] = uint16(knownGetKind)
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

	// --- ArrayMixed fast path ---
	asm.Label(mixedArrayLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffArrayLen) // array.len
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondGE, deoptLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffArray) // array data pointer
	asm.LDRreg(jit.X0, jit.X2, jit.X1)         // value = array[key]
	switch instr.Type {
	case TypeInt:
		ec.emitIntTagCheckBranch(jit.X0, jit.X2, jit.X3, jit.CondNE, deoptLabel)
		ec.emitUnboxInt48(jit.X0)
		ec.storeRawInt(jit.X0, instr.ID)
	case TypeFloat:
		jit.EmitIsTaggedPinned(asm, jit.X0, jit.X2, mRegTagInt)
		asm.BCond(jit.CondEQ, deoptLabel)
		asm.FMOVtoFP(jit.D0, jit.X0)
		ec.storeRawFloat(jit.D0, instr.ID)
	case TypeTable:
		jit.EmitCheckIsTableFull(asm, jit.X0, jit.X2, jit.X3, deoptLabel)
		ec.storeResultNB(jit.X0, instr.ID)
	default:
		ec.storeResultNB(jit.X0, instr.ID)
	}
	asm.B(doneLabel)

	// --- ArrayInt fast path ---
	asm.Label(intArrayLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffIntArrayLen) // intArray.len
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondGE, deoptLabel)
	asm.CMPimm(jit.X1, 0)
	nonZeroIntKeyLabel := ec.uniqueLabel("gettable_int_nonzero")
	asm.BCond(jit.CondNE, nonZeroIntKeyLabel)
	asm.LDRB(jit.X3, jit.X0, jit.TableOffArrayZeroValid)
	asm.CBZ(jit.X3, deoptLabel)
	asm.Label(nonZeroIntKeyLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffIntArray) // intArray data pointer
	asm.LDRreg(jit.X0, jit.X2, jit.X1)            // raw int64 = intArray[key]
	if instr.Type == TypeInt {
		ec.storeRawInt(jit.X0, instr.ID)
	} else {
		// NaN-box the int64: UBFX + ORR with pinned tag register.
		jit.EmitBoxIntFast(asm, jit.X0, jit.X0, mRegTagInt)
		ec.storeResultNB(jit.X0, instr.ID)
	}
	asm.B(doneLabel)

	// --- ArrayFloat fast path ---
	asm.Label(floatArrayLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffFloatArrayLen) // floatArray.len
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondGE, deoptLabel)
	asm.CMPimm(jit.X1, 0)
	nonZeroFloatKeyLabel := ec.uniqueLabel("gettable_float_nonzero")
	asm.BCond(jit.CondNE, nonZeroFloatKeyLabel)
	asm.LDRB(jit.X3, jit.X0, jit.TableOffArrayZeroValid)
	asm.CBZ(jit.X3, deoptLabel)
	asm.Label(nonZeroFloatKeyLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffFloatArray) // floatArray data pointer
	if instr.Type == TypeFloat {
		asm.FLDRdReg(jit.D0, jit.X2, jit.X1) // raw float64 = floatArray[key]
		ec.storeRawFloat(jit.D0, instr.ID)
	} else {
		asm.LDRreg(jit.X0, jit.X2, jit.X1) // raw float64 bits = floatArray[key]
		// Float64 bits ARE the NaN-boxed value — no conversion needed!
		ec.storeResultNB(jit.X0, instr.ID)
	}
	asm.B(doneLabel)

	// --- ArrayBool fast path ---
	asm.Label(boolArrayLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffBoolArrayLen) // boolArray.len
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondGE, deoptLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffBoolArray) // boolArray data pointer
	asm.LDRBreg(jit.X3, jit.X2, jit.X1)            // byte = boolArray[key]
	// Convert byte to NaN-boxed value: 0=nil, 1=false, 2=true
	nilLabel := ec.uniqueLabel("gettable_bool_nil")
	falseLabel := ec.uniqueLabel("gettable_bool_false")
	asm.CBZ(jit.X3, nilLabel) // byte == 0 → nil
	asm.CMPimm(jit.X3, 1)
	asm.BCond(jit.CondEQ, falseLabel) // byte == 1 → false
	// byte == 2 → true: NaN-boxed true = 0xFFFD000000000001
	asm.LoadImm64(jit.X0, nb64(jit.NB_TagBool|1))
	ec.storeResultNB(jit.X0, instr.ID)
	asm.B(doneLabel)
	asm.Label(falseLabel)
	// NaN-boxed false = 0xFFFD000000000000
	asm.LoadImm64(jit.X0, nb64(jit.NB_TagBool))
	ec.storeResultNB(jit.X0, instr.ID)
	asm.B(doneLabel)
	asm.Label(nilLabel)
	// NaN-boxed nil = 0xFFFC000000000000
	asm.LoadImm64(jit.X0, nb64(jit.NB_ValNil))
	ec.storeResultNB(jit.X0, instr.ID)
	asm.B(doneLabel)

	// Deopt: fall back to exit-resume.
	asm.Label(deoptLabel)
	savedReprs := ec.snapshotValueReprs()
	ec.emitGetTableExit(instr)
	ec.emitUnboxRawIntRegs(savedReprs)
	ec.restoreValueReprSnapshot(savedReprs)

	asm.Label(doneLabel)
}
