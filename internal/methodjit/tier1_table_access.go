//go:build darwin && arm64

// tier1_table_access.go emits ARM64 templates for baseline indexed table
// access: the integer-key array fast paths (Mixed/Int/Float/Bool) for
// GETTABLE and SETTABLE, with string keys delegated to the dynamic
// string-key cache. Pure code movement out of tier1_table.go.

package methodjit

import (
	"github.com/Never-Labs/gscript/internal/jit"
	"github.com/Never-Labs/gscript/internal/vm"
)

func emitBaselineGetTable(asm *jit.Assembler, inst uint32, pc int, feedbackEnabled bool) {
	a := vm.DecodeA(inst)
	b := vm.DecodeB(inst)
	cidx := vm.DecodeC(inst)

	slowLabel := nextLabel("gettable_slow")
	doneLabel := nextLabel("gettable_done")
	intArrayLabel := nextLabel("gettable_intarr")
	floatArrayLabel := nextLabel("gettable_floatarr")
	boolArrayLabel := nextLabel("gettable_boolarr")
	stringKeyLabel := nextLabel("gettable_string_key")

	// Load table value from R(B).
	loadSlot(asm, jit.X0, b)

	// Check table pointer.
	jit.EmitCheckIsTableFull(asm, jit.X0, jit.X1, jit.X2, slowLabel)
	jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	asm.CBZ(jit.X0, slowLabel)

	// Check metatable is nil (offset TableOffMetatable, must be 0).
	asm.LDR(jit.X1, jit.X0, jit.TableOffMetatable)
	asm.CBNZ(jit.X1, slowLabel) // has metatable -> slow path

	// Load key RK(C).
	loadRK(asm, jit.X1, cidx) // X1 = key (NaN-boxed)

	// Check if key is integer.
	asm.LSRimm(jit.X2, jit.X1, 48)
	asm.MOVimm16(jit.X3, uint16(jit.NB_TagIntShr48))
	asm.CMPreg(jit.X2, jit.X3)
	asm.BCond(jit.CondNE, stringKeyLabel) // not int -> try dynamic string-key cache

	// Extract integer key.
	asm.SBFX(jit.X1, jit.X1, 0, 48) // X1 = signed int key

	// Check key >= 0 (shared by all array kinds).
	asm.CMPimm(jit.X1, 0)
	asm.BCond(jit.CondLT, slowLabel)

	// Dispatch on arrayKind: 0=Mixed, 1=Int, 2=Float, 3=Bool, else=slow.
	asm.LDRB(jit.X2, jit.X0, jit.TableOffArrayKind)
	asm.CMPimm(jit.X2, jit.AKBool)
	asm.BCond(jit.CondEQ, boolArrayLabel)
	asm.CMPimm(jit.X2, jit.AKFloat)
	asm.BCond(jit.CondEQ, floatArrayLabel)
	asm.CMPimm(jit.X2, jit.AKInt)
	asm.BCond(jit.CondEQ, intArrayLabel)
	asm.CBNZ(jit.X2, slowLabel) // not Mixed (0) -> slow

	// --- ArrayMixed fast path ---
	asm.LDR(jit.X2, jit.X0, jit.TableOffArrayLen) // X2 = array.len
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondGE, slowLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffArray) // X2 = array data pointer
	if feedbackEnabled {
		emitBaselineFeedbackDenseMatrix(asm, pc, jit.X0, "mixed")
	}
	asm.LDRreg(jit.X0, jit.X2, jit.X1) // X0 = array[key] (NaN-boxed Value)
	storeSlot(asm, a, jit.X0)
	if feedbackEnabled {
		emitBaselineFeedbackResultFromValue(asm, pc, jit.X0, "mixed") // includes FBTable for table-of-tables rows
		emitBaselineFeedbackKind(asm, pc, 1, "mixed")                 // FBKindMixed=1
	}
	asm.B(doneLabel)

	// --- ArrayInt fast path ---
	asm.Label(intArrayLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffIntArrayLen) // X2 = intArray.len
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondGE, slowLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffIntArray) // X2 = intArray data pointer
	asm.LDRreg(jit.X0, jit.X2, jit.X1)            // X0 = intArray[key] (raw int64)
	// NaN-box the int64: UBFX + ORR with pinned tag register.
	jit.EmitBoxIntFast(asm, jit.X0, jit.X0, mRegTagInt)
	storeSlot(asm, a, jit.X0)
	if feedbackEnabled {
		emitBaselineFeedbackResult(asm, pc, 1, "int") // FBInt=1
		emitBaselineFeedbackKind(asm, pc, 2, "int")   // FBKindInt=2
	}
	asm.B(doneLabel)

	// --- ArrayFloat fast path ---
	asm.Label(floatArrayLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffFloatArrayLen) // X2 = floatArray.len
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondGE, slowLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffFloatArray) // X2 = floatArray data pointer
	asm.LDRreg(jit.X0, jit.X2, jit.X1)              // X0 = raw float64 bits = floatArray[key]
	// Float64 bits ARE the NaN-boxed value — no conversion needed!
	storeSlot(asm, a, jit.X0)
	if feedbackEnabled {
		emitBaselineFeedbackResult(asm, pc, 2, "float") // FBFloat=2
		emitBaselineFeedbackKind(asm, pc, 3, "float")   // FBKindFloat=3
	}
	asm.B(doneLabel)

	// --- ArrayBool fast path ---
	asm.Label(boolArrayLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffBoolArrayLen) // X2 = boolArray.len
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondGE, slowLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffBoolArray) // X2 = boolArray data pointer
	asm.LDRBreg(jit.X3, jit.X2, jit.X1)            // X3 = byte = boolArray[key]
	// Convert byte to NaN-boxed value: 0=nil, 1=false, 2=true
	boolNilLabel := nextLabel("gettable_bool_nil")
	boolFalseLabel := nextLabel("gettable_bool_false")
	asm.CBZ(jit.X3, boolNilLabel) // byte == 0 → nil
	asm.CMPimm(jit.X3, 1)
	asm.BCond(jit.CondEQ, boolFalseLabel) // byte == 1 → false
	// byte == 2 → true: NaN-boxed true = 0xFFFD000000000001
	asm.LoadImm64(jit.X0, nb64(jit.NB_TagBool|1))
	storeSlot(asm, a, jit.X0)
	if feedbackEnabled {
		emitBaselineFeedbackResult(asm, pc, 4, "bool_true") // FBBool=4
		emitBaselineFeedbackKind(asm, pc, 4, "bool_true")   // FBKindBool=4
	}
	asm.B(doneLabel)
	asm.Label(boolFalseLabel)
	// NaN-boxed false = 0xFFFD000000000000
	asm.LoadImm64(jit.X0, nb64(jit.NB_TagBool))
	storeSlot(asm, a, jit.X0)
	if feedbackEnabled {
		emitBaselineFeedbackResult(asm, pc, 4, "bool_false") // FBBool=4
		emitBaselineFeedbackKind(asm, pc, 4, "bool_false")   // FBKindBool=4
	}
	asm.B(doneLabel)
	asm.Label(boolNilLabel)
	// NaN-boxed nil = 0xFFFC000000000000
	asm.LoadImm64(jit.X0, nb64(jit.NB_ValNil))
	storeSlot(asm, a, jit.X0)
	if feedbackEnabled {
		emitBaselineFeedbackResult(asm, pc, 7, "bool_nil") // FBAny=7 for nil
		emitBaselineFeedbackKind(asm, pc, 4, "bool_nil")   // FBKindBool=4 (still a bool array)
	}
	asm.B(doneLabel)

	asm.Label(stringKeyLabel)
	emitBaselineDynamicStringGetTable(asm, a, pc, feedbackEnabled, slowLabel, doneLabel)

	// Slow path: exit-resume.
	asm.Label(slowLabel)
	emitBaselineOpExitCommon(asm, vm.OP_GETTABLE, pc, a, b, cidx)

	asm.Label(doneLabel)
}

// emitBaselineSetTable emits native ARM64 for OP_SETTABLE: R(A)[RK(B)] = RK(C)
// Fast path for integer keys with array bounds check.
// Supports ArrayMixed ([]Value), ArrayInt ([]int64), ArrayFloat ([]float64),
// and ArrayBool ([]byte) array kinds.
func emitBaselineSetTable(asm *jit.Assembler, inst uint32, pc int, feedbackEnabled bool) {
	a := vm.DecodeA(inst)
	bidx := vm.DecodeB(inst) // RK(B) = key
	cidx := vm.DecodeC(inst) // RK(C) = value

	slowLabel := nextLabel("settable_slow")
	doneLabel := nextLabel("settable_done")
	intArrayLabel := nextLabel("settable_intarr")
	floatArrayLabel := nextLabel("settable_floatarr")
	boolArrayLabel := nextLabel("settable_boolarr")
	stringKeyLabel := nextLabel("settable_string_key")

	// Load table value from R(A).
	loadSlot(asm, jit.X0, a)

	// Check table pointer.
	jit.EmitCheckIsTableFull(asm, jit.X0, jit.X1, jit.X2, slowLabel)
	jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	asm.CBZ(jit.X0, slowLabel)

	// Check metatable is nil.
	asm.LDR(jit.X1, jit.X0, jit.TableOffMetatable)
	asm.CBNZ(jit.X1, slowLabel)

	// Load key RK(B).
	loadRK(asm, jit.X1, bidx) // X1 = key (NaN-boxed)

	// Check if key is integer.
	asm.LSRimm(jit.X2, jit.X1, 48)
	asm.MOVimm16(jit.X3, uint16(jit.NB_TagIntShr48))
	asm.CMPreg(jit.X2, jit.X3)
	asm.BCond(jit.CondNE, stringKeyLabel) // not int -> try dynamic string-key cache

	// Extract integer key.
	asm.SBFX(jit.X1, jit.X1, 0, 48) // X1 = signed int key

	// Check key >= 0 (shared by both array kinds).
	asm.CMPimm(jit.X1, 0)
	asm.BCond(jit.CondLT, slowLabel)

	// Dispatch on arrayKind: 0=Mixed, 1=Int, 2=Float, 3=Bool, else=slow.
	asm.LDRB(jit.X2, jit.X0, jit.TableOffArrayKind)
	asm.CMPimm(jit.X2, jit.AKBool)
	asm.BCond(jit.CondEQ, boolArrayLabel)
	asm.CMPimm(jit.X2, jit.AKFloat)
	asm.BCond(jit.CondEQ, floatArrayLabel)
	asm.CMPimm(jit.X2, jit.AKInt)
	asm.BCond(jit.CondEQ, intArrayLabel)
	asm.CBNZ(jit.X2, slowLabel) // not Mixed (0) -> slow

	// --- ArrayMixed fast path ---
	mixedStoreLabel := nextLabel("settable_mixed_store")
	mixedAppendLabel := nextLabel("settable_mixed_append")
	emitTypedArraySetBoundsOrAppendCheck(asm, jit.X0, jit.X1, jit.X2, jit.TableOffArrayLen, mixedAppendLabel, slowLabel)
	asm.Label(mixedStoreLabel)
	loadRK(asm, jit.X4, cidx) // X4 = value (NaN-boxed)
	asm.LDR(jit.X2, jit.X0, jit.TableOffArray)
	asm.STRreg(jit.X4, jit.X2, jit.X1) // array[key] = value
	asm.MOVimm16(jit.X5, 1)
	asm.STRB(jit.X5, jit.X0, jit.TableOffKeysDirty)
	if feedbackEnabled {
		emitBaselineFeedbackKind(asm, pc, 1, "set_mixed") // FBKindMixed=1
	}
	asm.B(doneLabel)
	emitTypedArraySetAppendPath(asm, jit.X0, jit.X1, jit.X6, jit.TableOffArrayLen, jit.TableOffArrayCap, mixedAppendLabel, slowLabel, mixedStoreLabel)

	// --- ArrayInt fast path ---
	asm.Label(intArrayLabel)
	intStoreLabel := nextLabel("settable_int_store")
	intAppendLabel := nextLabel("settable_int_append")
	emitTypedArraySetBoundsOrAppendCheck(asm, jit.X0, jit.X1, jit.X2, jit.TableOffIntArrayLen, intAppendLabel, slowLabel)
	asm.Label(intStoreLabel)
	// Load value RK(C) and check it's an integer.
	loadRK(asm, jit.X4, cidx) // X4 = value (NaN-boxed)
	asm.LSRimm(jit.X5, jit.X4, 48)
	asm.MOVimm16(jit.X6, uint16(jit.NB_TagIntShr48))
	asm.CMPreg(jit.X5, jit.X6)
	asm.BCond(jit.CondNE, slowLabel) // value not int -> slow (type mismatch)
	// Unbox int64 from NaN-boxed value.
	asm.SBFX(jit.X4, jit.X4, 0, 48) // X4 = raw int64
	asm.LDR(jit.X2, jit.X0, jit.TableOffIntArray)
	asm.STRreg(jit.X4, jit.X2, jit.X1) // intArray[key] = int64
	if feedbackEnabled {
		emitBaselineFeedbackKind(asm, pc, 2, "set_int") // FBKindInt=2
	}
	asm.B(doneLabel)
	emitTypedArraySetAppendPathDirty(asm, jit.X0, jit.X1, jit.X6, jit.TableOffIntArrayLen, jit.TableOffIntArrayCap, intAppendLabel, slowLabel, intStoreLabel)

	// --- ArrayFloat fast path ---
	asm.Label(floatArrayLabel)
	floatStoreLabel := nextLabel("settable_float_store")
	floatAppendLabel := nextLabel("settable_float_append")
	emitTypedArraySetBoundsOrAppendCheck(asm, jit.X0, jit.X1, jit.X2, jit.TableOffFloatArrayLen, floatAppendLabel, slowLabel)
	asm.Label(floatStoreLabel)
	// Load value RK(C) and check it's a float.
	loadRK(asm, jit.X4, cidx) // X4 = value (NaN-boxed)
	// Float check: if top bits indicate tagged (int/bool/nil/ptr), not a float → slow.
	jit.EmitIsTaggedPinned(asm, jit.X4, jit.X5, mRegTagInt) // sets flags: EQ = tagged, NE = float
	asm.BCond(jit.CondEQ, slowLabel)                        // tagged → slow (not a float)
	// Float64 bits ARE the NaN-boxed representation — store directly.
	asm.LDR(jit.X2, jit.X0, jit.TableOffFloatArray) // floatArray data pointer
	asm.STRreg(jit.X4, jit.X2, jit.X1)              // floatArray[key] = float64
	if feedbackEnabled {
		emitBaselineFeedbackKind(asm, pc, 3, "set_float") // FBKindFloat=3
	}
	asm.B(doneLabel)
	emitTypedArraySetAppendPathDirty(asm, jit.X0, jit.X1, jit.X6, jit.TableOffFloatArrayLen, jit.TableOffFloatArrayCap, floatAppendLabel, slowLabel, floatStoreLabel)

	// --- ArrayBool fast path ---
	asm.Label(boolArrayLabel)
	boolStoreLabel := nextLabel("settable_bool_store")
	boolAppendLabel := nextLabel("settable_bool_append")
	emitTypedArraySetBoundsOrAppendCheck(asm, jit.X0, jit.X1, jit.X2, jit.TableOffBoolArrayLen, boolAppendLabel, slowLabel)
	asm.Label(boolStoreLabel)
	// Load value RK(C).
	loadRK(asm, jit.X4, cidx) // X4 = value (NaN-boxed)
	// Check value type: must be bool (tag=0xFFFD) or nil (0xFFFC).
	asm.LSRimm(jit.X5, jit.X4, 48)
	asm.MOVimm16(jit.X6, uint16(jit.NB_TagBoolShr48))
	asm.CMPreg(jit.X5, jit.X6)
	boolOkLabel := nextLabel("settable_bool_isbool")
	asm.BCond(jit.CondEQ, boolOkLabel)
	// Check if nil.
	asm.MOVimm16(jit.X6, uint16(jit.NB_TagNilShr48))
	asm.CMPreg(jit.X5, jit.X6)
	asm.BCond(jit.CondNE, slowLabel) // not bool, not nil → slow
	// Nil → byte 0.
	asm.MOVimm16(jit.X4, 0)
	setByteLabel := nextLabel("settable_bool_store")
	asm.B(setByteLabel)
	asm.Label(boolOkLabel)
	// Bool: extract payload bit 0. false=0xFFFD000000000000 (payload=0) → byte 1
	//                                true=0xFFFD000000000001 (payload=1) → byte 2
	// Conversion: byte = payload + 1
	asm.LoadImm64(jit.X5, 1)
	asm.ANDreg(jit.X4, jit.X4, jit.X5) // extract bit 0 (payload)
	asm.ADDimm(jit.X4, jit.X4, 1)      // 0→1 (false), 1→2 (true)
	asm.Label(setByteLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffBoolArray) // boolArray data pointer
	asm.STRBreg(jit.X4, jit.X2, jit.X1)            // boolArray[key] = byte
	// Set keysDirty flag.
	asm.MOVimm16(jit.X5, 1)
	asm.STRB(jit.X5, jit.X0, jit.TableOffKeysDirty)
	if feedbackEnabled {
		emitBaselineFeedbackKind(asm, pc, 4, "set_bool") // FBKindBool=4
	}
	asm.B(doneLabel)
	emitTypedArraySetAppendPath(asm, jit.X0, jit.X1, jit.X6, jit.TableOffBoolArrayLen, jit.TableOffBoolArrayCap, boolAppendLabel, slowLabel, boolStoreLabel)

	asm.Label(stringKeyLabel)
	emitBaselineDynamicStringSetTable(asm, cidx, pc, feedbackEnabled, slowLabel, doneLabel)

	// Slow path: exit-resume.
	asm.Label(slowLabel)
	emitBaselineOpExitCommon(asm, vm.OP_SETTABLE, pc, a, bidx, cidx)

	asm.Label(doneLabel)
}
