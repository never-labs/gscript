//go:build darwin && arm64

// tier1_field_global.go emits ARM64 templates for baseline static field and
// global-variable access: shape-guarded inline caches for GETFIELD/SETFIELD,
// the polymorphic field-cache lookup, and the generation-versioned GETGLOBAL
// value cache. Pure code movement out of tier1_table.go.

package methodjit

import (
	"github.com/never-labs/gscript/internal/jit"
	"github.com/never-labs/gscript/internal/runtime"
	"github.com/never-labs/gscript/internal/vm"
)

// emitBaselineGetGlobal emits native ARM64 for OP_GETGLOBAL: R(A) = globals[K(Bx)]
// Uses a per-PC value cache stored in BaselineFunc.GlobalValCache with a
// generation-based invalidation scheme. The cache is populated by the Go slow
// path (handleGetGlobal) on first miss. SetGlobal increments the generation
// counter, causing all caches to miss on next access.
//
// Fast path (~8 instructions):
//  1. Version check: engine.globalCacheGen == bf.CachedGlobalGen
//  2. Load GlobalCache pointer from ExecContext
//  3. Load cached value at GlobalValCache[pc]
//  4. If non-zero (cached), store to R(A) and continue
//
// Slow path: standard exit-resume to handleGetGlobal in Go.
func emitBaselineGetGlobal(asm *jit.Assembler, inst uint32, pc int) {
	a := vm.DecodeA(inst)
	bx := vm.DecodeBx(inst)

	slowLabel := nextLabel("getglobal_slow")
	doneLabel := nextLabel("getglobal_done")

	// Version check: engine.globalCacheGen == ctx.BaselineGlobalCachedGen
	asm.LDR(jit.X0, mRegCtx, execCtxOffBaselineGlobalGenPtr)
	asm.CBZ(jit.X0, slowLabel)                                  // no gen pointer = no cache
	asm.LDR(jit.X1, jit.X0, 0)                                  // X1 = current gen
	asm.LDR(jit.X2, mRegCtx, execCtxOffBaselineGlobalCachedGen) // X2 = cached gen
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondNE, slowLabel) // generation mismatch -> cache invalid

	// Load GlobalCache pointer from ExecContext.
	asm.LDR(jit.X0, mRegCtx, execCtxOffBaselineGlobalCache)
	asm.CBZ(jit.X0, slowLabel) // no cache allocated

	// Load cached value at GlobalValCache[pc].
	cacheOff := pc * 8 // each entry is 8 bytes (uint64)
	if cacheOff < 4096 {
		asm.LDR(jit.X1, jit.X0, cacheOff)
	} else {
		asm.LoadImm64(jit.X1, int64(cacheOff))
		asm.ADDreg(jit.X0, jit.X0, jit.X1)
		asm.LDR(jit.X1, jit.X0, 0)
	}

	// If zero (cache miss), go to slow path.
	asm.CBZ(jit.X1, slowLabel)

	// Cache hit: store to R(A).
	storeSlot(asm, a, jit.X1)
	asm.B(doneLabel)

	// Slow path: exit-resume.
	asm.Label(slowLabel)
	emitBaselineOpExitCommon(asm, vm.OP_GETGLOBAL, pc, a, bx, 0)

	asm.Label(doneLabel)
}

// emitBaselineGetField emits native ARM64 for OP_GETFIELD: R(A) = R(B).field[C]
// Uses runtime inline cache from proto.FieldCache[pc].
// Falls back to exit-resume if cache miss or non-table.
func emitBaselineGetField(asm *jit.Assembler, inst uint32, pc int, feedbackEnabled bool) {
	a := vm.DecodeA(inst)
	b := vm.DecodeB(inst)
	c := vm.DecodeC(inst) // constant index for field name

	slowLabel := nextLabel("getfield_slow")
	doneLabel := nextLabel("getfield_done")
	emptyMissLabel := nextLabel("getfield_empty_miss")
	polyMissLabel := nextLabel("getfield_poly_miss")

	// Load FieldCache pointer from ExecContext.
	asm.LDR(jit.X0, mRegCtx, execCtxOffBaselineFieldCache)
	asm.CBZ(jit.X0, slowLabel) // no field cache allocated yet

	// Compute &FieldCache[pc]: X0 + pc * FieldCacheEntrySize
	if pc > 0 {
		entryOff := pc * jit.FieldCacheEntrySize
		if entryOff < 4096 {
			asm.ADDimm(jit.X0, jit.X0, uint16(entryOff))
		} else {
			asm.LoadImm64(jit.X1, int64(entryOff))
			asm.ADDreg(jit.X0, jit.X0, jit.X1)
		}
	}
	// X0 = &FieldCache[pc]

	// Load entry.ShapeID (uint32 at offset 8). Use LDRW for 32-bit.
	asm.LDRW(jit.X2, jit.X0, jit.FieldCacheEntryOffShapeID) // X2 = cached shapeID
	asm.CBZ(jit.X2, slowLabel)                              // shapeID==0 means not cached

	// Load entry.FieldIdx (int at offset 0).
	asm.LDR(jit.X3, jit.X0, jit.FieldCacheEntryOffFieldIdx) // X3 = fieldIdx

	// Load table value from R(B).
	loadSlot(asm, jit.X0, b)

	tableCheckLabel := nextLabel("getfield_table_check")

	// FixedRecord values share the field cache contract with shaped tables:
	// guard by shapeID and field index, then load the inline value directly.
	asm.LSRimm(jit.X1, jit.X0, 48)
	asm.MOVimm16(jit.X4, jit.NB_TagPtrShr48)
	asm.CMPreg(jit.X1, jit.X4)
	asm.BCond(jit.CondNE, tableCheckLabel)
	asm.LSRimm(jit.X1, jit.X0, uint8(jit.NB_PtrSubShift))
	asm.LoadImm64(jit.X4, 0xF)
	asm.ANDreg(jit.X1, jit.X1, jit.X4)
	asm.CMPimm(jit.X1, jit.NB_PtrSubFixedRecord)
	asm.BCond(jit.CondNE, tableCheckLabel)
	jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	asm.CBZ(jit.X0, slowLabel)
	asm.LDRW(jit.X1, jit.X0, jit.FixedRecordOffShapeID)
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondNE, slowLabel)
	asm.LDRB(jit.X1, jit.X0, jit.FixedRecordOffN)
	asm.CMPreg(jit.X3, jit.X1)
	asm.BCond(jit.CondGE, slowLabel)
	asm.ADDimm(jit.X1, jit.X0, jit.FixedRecordOffValues)
	asm.LDRreg(jit.X0, jit.X1, jit.X3)
	if feedbackEnabled {
		emitBaselineFeedbackResultFromValue(asm, pc, jit.X0, "getfield_fixed_record")
	}
	storeSlot(asm, a, jit.X0)
	asm.B(doneLabel)

	asm.Label(tableCheckLabel)

	// Check it's a table pointer (tag = 0xFFFF, sub = 0).
	jit.EmitCheckIsTableFull(asm, jit.X0, jit.X1, jit.X4, slowLabel)

	// Extract raw *Table pointer (44-bit payload).
	jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	asm.CBZ(jit.X0, slowLabel)

	// Shape guard: table.shapeID must match cached shapeID.
	asm.LDRW(jit.X1, jit.X0, jit.TableOffShapeID) // X1 = table.shapeID
	asm.CMPreg(jit.X1, jit.X2)                    // compare with cached shapeID
	asm.BCond(jit.CondNE, polyMissLabel)

	// Bounds check: fieldIdx < len(svals)
	asm.LDR(jit.X1, jit.X0, jit.TableOffSvalsLen) // X1 = svals.len
	asm.CMPreg(jit.X3, jit.X1)                    // fieldIdx < svals.len?
	asm.BCond(jit.CondGE, slowLabel)              // unsigned >= means out of bounds

	// Direct field access: svals[fieldIdx]
	// LDRreg uses [Xn + Xm, LSL #3] which already scales by 8 (= ValueSize),
	// so X3 must hold the raw fieldIdx (not pre-multiplied).
	asm.LDR(jit.X1, jit.X0, jit.TableOffSvals) // X1 = svals data pointer
	asm.LDRreg(jit.X0, jit.X1, jit.X3)         // X0 = svals[fieldIdx]

	// Keep type feedback current on the field-cache fast path. The slow path
	// updates feedback in Go; without this, a site can stay stuck on the first
	// cached value's type even after later cache hits observe a different type.
	if feedbackEnabled {
		emitBaselineFeedbackResultFromValue(asm, pc, jit.X0, "getfield")
	}

	// Store result to R(A).
	storeSlot(asm, a, jit.X0)
	asm.B(doneLabel)

	// Polymorphic static-field cache for object dispatch sites that rotate
	// among a small number of stable table shapes.
	asm.Label(polyMissLabel)
	asm.CBZ(jit.X1, emptyMissLabel)
	emitBaselineFieldPolyLookup(asm, pc, a, jit.X0, jit.X1, feedbackEnabled, "getfield_poly", slowLabel, doneLabel)

	// Empty shape-less tables cannot contain any string field. This catches
	// leaf objects built from nil fields without bouncing through Go.
	asm.Label(emptyMissLabel)
	asm.CBNZ(jit.X1, slowLabel) // non-empty shape mismatch
	asm.LDR(jit.X4, jit.X0, jit.TableOffMetatable)
	asm.CBNZ(jit.X4, slowLabel) // __index may synthesize a missing field
	asm.LDR(jit.X4, jit.X0, jit.TableOffSvalsLen)
	asm.CBNZ(jit.X4, slowLabel) // shape-less but not empty
	asm.LDR(jit.X4, jit.X0, jit.TableOffSmap)
	asm.CBNZ(jit.X4, slowLabel) // large string-key table
	asm.LDR(jit.X4, jit.X0, jit.TableOffLazyTree)
	asm.CBNZ(jit.X4, slowLabel) // lazy fields must be resolved by runtime
	jit.EmitBoxNil(asm, jit.X0)
	if feedbackEnabled {
		emitBaselineFeedbackResult(asm, pc, 7, "getfield_empty")
	}
	storeSlot(asm, a, jit.X0)
	asm.B(doneLabel)

	asm.Label(slowLabel)
	emitBaselineOpExitCommon(asm, vm.OP_GETFIELD, pc, a, b, c)

	asm.Label(doneLabel)
}

func emitBaselineFieldPolyLookup(asm *jit.Assembler, pc, dstSlot int, tableReg, shapeReg jit.Reg, feedbackEnabled bool, feedbackName, slowLabel, doneLabel string) {
	asm.LDR(jit.X5, mRegCtx, execCtxOffBaselineFieldPolyCache)
	asm.CBZ(jit.X5, slowLabel)
	entryOff := pc * runtime.FieldPolyCacheWays * jit.FieldPolyCacheEntrySize
	if entryOff > 0 {
		if entryOff <= 4095 {
			asm.ADDimm(jit.X5, jit.X5, uint16(entryOff))
		} else {
			asm.LoadImm64(jit.X6, int64(entryOff))
			asm.ADDreg(jit.X5, jit.X5, jit.X6)
		}
	}

	for i := 0; i < runtime.FieldPolyCacheWays; i++ {
		nextWayLabel := nextLabel("field_poly_next")
		asm.LDRW(jit.X6, jit.X5, jit.FieldPolyCacheEntryOffShapeID)
		asm.CMPreg(jit.X6, shapeReg)
		asm.BCond(jit.CondNE, nextWayLabel)

		asm.LDR(jit.X3, jit.X5, jit.FieldPolyCacheEntryOffFieldIdx)
		asm.LDR(jit.X6, tableReg, jit.TableOffSvalsLen)
		asm.CMPreg(jit.X3, jit.X6)
		asm.BCond(jit.CondGE, slowLabel)

		asm.LDR(jit.X6, tableReg, jit.TableOffSvals)
		asm.LDRreg(jit.X0, jit.X6, jit.X3)
		if feedbackEnabled {
			emitBaselineFeedbackResultFromValue(asm, pc, jit.X0, feedbackName)
		}
		storeSlot(asm, dstSlot, jit.X0)
		asm.B(doneLabel)

		asm.Label(nextWayLabel)
		if i+1 < runtime.FieldPolyCacheWays {
			asm.ADDimm(jit.X5, jit.X5, uint16(jit.FieldPolyCacheEntrySize))
		}
	}
	asm.B(slowLabel)
}

// emitBaselineSetField emits native ARM64 for OP_SETFIELD: R(A).field[B] = RK(C)
// Uses runtime inline cache from proto.FieldCache[pc].
func emitBaselineSetField(asm *jit.Assembler, inst uint32, pc int, feedbackEnabled bool) {
	a := vm.DecodeA(inst)
	b := vm.DecodeB(inst) // constant index for field name
	c := vm.DecodeC(inst) // RK(C) = value

	slowLabel := nextLabel("setfield_slow")
	doneLabel := nextLabel("setfield_done")

	// Load FieldCache pointer from ExecContext.
	asm.LDR(jit.X0, mRegCtx, execCtxOffBaselineFieldCache)
	asm.CBZ(jit.X0, slowLabel)

	// Compute &FieldCache[pc].
	if pc > 0 {
		entryOff := pc * jit.FieldCacheEntrySize
		if entryOff < 4096 {
			asm.ADDimm(jit.X0, jit.X0, uint16(entryOff))
		} else {
			asm.LoadImm64(jit.X1, int64(entryOff))
			asm.ADDreg(jit.X0, jit.X0, jit.X1)
		}
	}
	asm.MOVreg(jit.X7, jit.X0) // X7 = &FieldCache[pc]

	// Load entry.ShapeID.
	asm.LDRW(jit.X2, jit.X0, jit.FieldCacheEntryOffShapeID)
	asm.CBZ(jit.X2, slowLabel)

	// Load entry.FieldIdx.
	asm.LDR(jit.X3, jit.X0, jit.FieldCacheEntryOffFieldIdx) // X3 = fieldIdx

	// Load table value from R(A).
	loadSlot(asm, jit.X0, a)

	// Check table pointer.
	jit.EmitCheckIsTableFull(asm, jit.X0, jit.X1, jit.X4, slowLabel)
	jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	asm.CBZ(jit.X0, slowLabel)

	// Shape guard.
	asm.LDRW(jit.X1, jit.X0, jit.TableOffShapeID)
	asm.CMPreg(jit.X1, jit.X2)
	appendLabel := nextLabel("setfield_append")
	asm.BCond(jit.CondNE, appendLabel)

	// Bounds check: fieldIdx < len(svals)
	asm.LDR(jit.X1, jit.X0, jit.TableOffSvalsLen)
	asm.CMPreg(jit.X3, jit.X1)
	asm.BCond(jit.CondGE, slowLabel)

	// Load value to store: RK(C).
	loadRK(asm, jit.X4, c) // X4 = value

	// Mirror the interpreter/slow-path SETFIELD feedback for cache hits too.
	if feedbackEnabled {
		emitBaselineFeedbackResultFromValue(asm, pc, jit.X4, "setfield")
	}

	// Direct field store: svals[fieldIdx] = value.
	// STRreg uses [Xn + Xm, LSL #3] which already scales by 8 (= ValueSize),
	// so X3 must hold the raw fieldIdx (not pre-multiplied).
	asm.LDR(jit.X1, jit.X0, jit.TableOffSvals) // X1 = svals data pointer
	asm.STRreg(jit.X4, jit.X1, jit.X3)         // svals[fieldIdx] = value
	versionSkipLabel := nextLabel("setfield_string_lookup_version_skip")
	asm.LDR(jit.X8, jit.X0, jit.TableOffStringLookupVer)
	asm.CBZ(jit.X8, versionSkipLabel)
	asm.ADDimm(jit.X8, jit.X8, 1)
	asm.STR(jit.X8, jit.X0, jit.TableOffStringLookupVer)
	asm.Label(versionSkipLabel)

	asm.B(doneLabel)

	// Constructor-style append: the Go slow path records AppendShapeID and
	// AppendShape when this SETFIELD appends a new key to a small shaped table.
	asm.Label(appendLabel)
	asm.LDRW(jit.X5, jit.X7, fieldCacheEntryOffAppendShapeID)
	asm.CMPreg(jit.X1, jit.X5)
	asm.BCond(jit.CondNE, slowLabel)
	asm.LDR(jit.X5, jit.X7, fieldCacheEntryOffAppendShape)
	asm.CBZ(jit.X5, slowLabel)
	asm.LDR(jit.X6, jit.X0, jit.TableOffSmap)
	asm.CBNZ(jit.X6, slowLabel)
	asm.LDR(jit.X6, jit.X0, jit.TableOffLazyTree)
	asm.CBNZ(jit.X6, slowLabel)
	asm.LDR(jit.X6, jit.X0, jit.TableOffSvalsLen)
	asm.CMPreg(jit.X3, jit.X6)
	asm.BCond(jit.CondNE, slowLabel)
	asm.CMPimm(jit.X3, runtime.SmallFieldCap)
	asm.BCond(jit.CondGE, slowLabel)
	asm.LDR(jit.X8, jit.X0, jit.TableOffSvals+16)
	asm.CMPreg(jit.X3, jit.X8)
	asm.BCond(jit.CondGE, slowLabel)

	loadRK(asm, jit.X4, c)
	asm.LoadImm64(jit.X8, nb64(jit.NB_ValNil))
	asm.CMPreg(jit.X4, jit.X8)
	asm.BCond(jit.CondEQ, slowLabel)
	asm.LDR(jit.X8, jit.X0, jit.TableOffSvals)
	asm.STRreg(jit.X4, jit.X8, jit.X3)
	appendVersionSkipLabel := nextLabel("setfield_append_string_lookup_version_skip")
	asm.LDR(jit.X8, jit.X0, jit.TableOffStringLookupVer)
	asm.CBZ(jit.X8, appendVersionSkipLabel)
	asm.ADDimm(jit.X8, jit.X8, 1)
	asm.STR(jit.X8, jit.X0, jit.TableOffStringLookupVer)
	asm.Label(appendVersionSkipLabel)
	asm.ADDimm(jit.X6, jit.X6, 1)
	asm.STR(jit.X6, jit.X0, jit.TableOffSvalsLen)
	asm.STRW(jit.X2, jit.X0, jit.TableOffShapeID)
	asm.STR(jit.X5, jit.X0, jit.TableOffShape)
	asm.LDR(jit.X8, jit.X5, shapeOffFieldKeys)
	asm.STR(jit.X8, jit.X0, jit.TableOffSkeys)
	asm.LDR(jit.X8, jit.X5, shapeOffFieldKeysLen)
	asm.STR(jit.X8, jit.X0, jit.TableOffSkeysLen)
	asm.LDR(jit.X8, jit.X5, shapeOffFieldKeysCap)
	asm.STR(jit.X8, jit.X0, jit.TableOffSkeys+16)
	asm.MOVimm16(jit.X8, 1)
	asm.STRB(jit.X8, jit.X0, jit.TableOffKeysDirty)
	asm.B(doneLabel)

	// Slow path: exit-resume.
	asm.Label(slowLabel)
	emitBaselineOpExitCommon(asm, vm.OP_SETFIELD, pc, a, b, c)

	asm.Label(doneLabel)
}
