//go:build darwin && arm64

// tier1_handlers_table.go contains the Tier 1 baseline JIT exit handlers for
// global, table, field, and object-construction opcodes (GETGLOBAL/SETGLOBAL,
// NEWTABLE/NEWOBJECT2/NEWOBJECTN, GET/SETTABLE, GET/SETFIELD, SETLIST, APPEND),
// plus the inline-cache lazy-allocation helpers they populate.
// Pure code movement from tier1_handlers.go; no behavior change.

package methodjit

import (
	"fmt"
	"unsafe"

	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

func (e *BaselineJITEngine) handleNewObject2(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto, bf *BaselineFunc) error {
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB)
	c := int(ctx.BaselineC)
	absA := base + a
	if absA >= len(regs) {
		return nil
	}
	if b < 0 || b >= len(proto.TableCtors2) || base+c+1 >= len(regs) {
		regs[absA] = runtime.FreshTableValue(runtime.NewTableSized(0, 2))
		return nil
	}
	ctor := &proto.TableCtors2[b].Runtime
	val1 := regs[base+c]
	val2 := regs[base+c+1]
	tbl := runtime.NewTableFromCtor2(ctor, val1, val2)
	fillBaselineNewObject2Cache(bf, int(ctx.BaselinePC)-1, ctor, val1, val2)
	regs[absA] = runtime.FreshTableValue(tbl)
	return nil
}

func (e *BaselineJITEngine) handleNewObjectN(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto, bf *BaselineFunc) error {
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB)
	c := int(ctx.BaselineC)
	absA := base + a
	if absA >= len(regs) {
		return nil
	}
	if b < 0 || b >= len(proto.TableCtorsN) {
		regs[absA] = runtime.FreshTableValue(runtime.NewEmptyTable())
		return nil
	}
	ctor := &proto.TableCtorsN[b].Runtime
	n := len(ctor.Keys)
	start := base + c
	if start < 0 || start+n > len(regs) {
		regs[absA] = runtime.FreshTableValue(runtime.NewTableSized(0, n))
		return nil
	}
	pc := int(ctx.BaselinePC) - 1
	if bf != nil && !bf.HasNativeCoroutineSwitch && pc >= 0 && pc < len(proto.Code) && baselineNewObjectNCacheable(proto, proto.Code[pc]) {
		regs[absA] = runtime.FreshTableValue(allocateFixedTableNWithCache(bf.NewTableCaches, pc, ctor, regs[start:start+n]))
		return nil
	}
	if e.callVM != nil {
		return e.callVM.NewObjectNFromSlots(proto, b, absA, start)
	}
	regs[absA] = runtime.FreshTableValue(runtime.NewTableFromCtorN(ctor, regs[start:start+n]))
	return nil
}

// handleGetGlobal handles OP_GETGLOBAL exit.
// Populates bf.GlobalValCache so the native inline cache hits on next access.
func (e *BaselineJITEngine) handleGetGlobal(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto, bf *BaselineFunc) error {
	if e.callVM == nil {
		return fmt.Errorf("no callVM for global-exit")
	}
	a := int(ctx.BaselineA)
	bx := int(ctx.BaselineB)
	if bx >= len(proto.Constants) {
		return fmt.Errorf("global const index %d out of range", bx)
	}
	name := proto.Constants[bx].Str()
	val := e.callVM.GetGlobal(name)
	absSlot := base + a
	if absSlot < len(regs) {
		regs[absSlot] = val
	}
	// Populate the per-PC global value cache for the native fast path.
	// BaselinePC is the resume (next) PC, so current instruction PC = BaselinePC - 1.
	if bf.GlobalValCache != nil {
		pc := int(ctx.BaselinePC) - 1
		if pc >= 0 && pc < len(bf.GlobalValCache) && uint64(val) != 0 {
			// If the generation has changed since we last cached, ALL entries
			// are potentially stale. Clear the entire cache before repopulating
			// this entry. Without this, updating CachedGlobalGen would make
			// other PCs' stale cached values appear valid.
			if bf.CachedGlobalGen != e.globalCacheGen {
				for i := range bf.GlobalValCache {
					bf.GlobalValCache[i] = 0
				}
			}
			bf.GlobalValCache[pc] = uint64(val)
			bf.CachedGlobalGen = e.globalCacheGen
			ctx.BaselineGlobalCachedGen = e.globalCacheGen
		}
	}
	return nil
}

// handleSetGlobal handles OP_SETGLOBAL exit.
// Invalidates only GlobalValCache entries that read the written name.
func (e *BaselineJITEngine) handleSetGlobal(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	if e.callVM == nil {
		return fmt.Errorf("no callVM for setglobal-exit")
	}
	a := int(ctx.BaselineA)
	bx := int(ctx.BaselineB)
	if bx >= len(proto.Constants) {
		return fmt.Errorf("setglobal const index %d out of range", bx)
	}
	name := proto.Constants[bx].Str()
	absSlot := base + a
	if absSlot < len(regs) {
		if err := e.callVM.SetGlobalForJIT(name, regs[absSlot]); err != nil {
			return err
		}
	}
	if verPtr, ver, ok := e.callVM.GlobalValueVersionPtr(); ok {
		ctx.Tier2GlobalVerPtr = uintptr(unsafe.Pointer(verPtr))
		ctx.Tier2GlobalVer = ver
	}
	e.invalidateGlobalValueCaches(name)
	return nil
}

// handleNewTable handles OP_NEWTABLE exit.
func (e *BaselineJITEngine) handleNewTable(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto, bf *BaselineFunc) error {
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB) // array hint
	c := int(ctx.BaselineC) // hash hint
	absSlot := base + a
	pc := int(ctx.BaselinePC) - 1
	var tbl *runtime.Table
	if bf != nil {
		tbl = allocateBaselineNewTableWithCache(bf.NewTableCaches, pc, b, c, runtime.ArrayMixed)
	} else {
		tbl = runtime.NewTableSized(b, c)
	}
	if absSlot < len(regs) {
		regs[absSlot] = runtime.FreshTableValue(tbl)
	}
	return nil
}

// handleGetTable handles OP_GETTABLE exit.
func (e *BaselineJITEngine) handleGetTable(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB)
	c := int(ctx.BaselineC)

	absB := base + b
	if absB >= len(regs) {
		return nil
	}
	tblVal := regs[absB]

	// Resolve RK(C)
	var key runtime.Value
	if c >= vm.RKBit {
		key = proto.Constants[c-vm.RKBit]
	} else {
		absC := base + c
		if absC < len(regs) {
			key = regs[absC]
		}
	}

	absA := base + a
	if key.IsString() {
		if v, ok := tblVal.FixedRecordRawGetString(key.Str()); ok {
			if absA < len(regs) {
				regs[absA] = v
				pc := int(ctx.BaselinePC) - 1
				if proto.Feedback != nil && pc >= 0 && pc < len(proto.Feedback) {
					proto.Feedback[pc].Result.Observe(v.Type())
				}
			}
			return nil
		}
	}
	if tblVal.IsTable() {
		if absA < len(regs) {
			tbl := tblVal.Table()
			// Record type feedback so Tier 2 can specialize.
			pc := int(ctx.BaselinePC) - 1
			if key.IsString() {
				ensureTableStringKeyCache(proto)
				syncTableStringKeyCacheContext(ctx, proto)
				regs[absA] = tbl.RawGetStringDynamicCached(
					key.Str(),
					runtime.TableStringKeyCacheSlot(proto.TableStringKeyCache, pc),
				)
			} else {
				regs[absA] = tbl.RawGet(key)
			}
			if proto.Feedback != nil && pc >= 0 && pc < len(proto.Feedback) {
				proto.Feedback[pc].Result.Observe(regs[absA].Type())
				// Record array kind for table-access specialization.
				proto.Feedback[pc].ObserveKind(uint8(tbl.GetArrayKind()))
				if proto.TableKeyFeedback != nil && pc < len(proto.TableKeyFeedback) {
					tkf := &proto.TableKeyFeedback[pc]
					tkf.ObserveTableAccess(tbl, key, regs[absA], vm.TableAccessKindGet, -1, -1)
				}
			}
		}
	} else if absA < len(regs) {
		regs[absA] = runtime.NilValue()
	}
	return nil
}

// handleSetTable handles OP_SETTABLE exit.
func (e *BaselineJITEngine) handleSetTable(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB)
	c := int(ctx.BaselineC)

	absA := base + a
	if absA >= len(regs) {
		return nil
	}
	tblVal := regs[absA]

	// Resolve RK(B) = key
	var key runtime.Value
	if b >= vm.RKBit {
		key = proto.Constants[b-vm.RKBit]
	} else {
		absB := base + b
		if absB < len(regs) {
			key = regs[absB]
		}
	}

	// Resolve RK(C) = value
	var val runtime.Value
	if c >= vm.RKBit {
		val = proto.Constants[c-vm.RKBit]
	} else {
		absC := base + c
		if absC < len(regs) {
			val = regs[absC]
		}
	}

	if tblVal.IsTable() {
		tbl := tblVal.Table()
		if tbl.HasMetatable() {
			if e.callVM == nil {
				return fmt.Errorf("no callVM for table-set exit")
			}
			return e.callVM.TableSetForJIT(tblVal, key, val)
		}
		// Record array kind feedback for table-access specialization.
		pc := int(ctx.BaselinePC) - 1
		beforeLen, beforeFieldIdx := -1, -1
		if proto.TableKeyFeedback != nil && pc >= 0 && pc < len(proto.TableKeyFeedback) {
			if key.IsInt() {
				beforeLen = tbl.Len()
			} else if key.IsString() {
				beforeFieldIdx = tbl.FieldIndex(key.Str())
			}
		}
		if key.IsString() {
			ensureTableStringKeyCache(proto)
			syncTableStringKeyCacheContext(ctx, proto)
			tbl.RawSetStringDynamicCached(
				key.Str(),
				val,
				runtime.TableStringKeyCacheSlot(proto.TableStringKeyCache, pc),
			)
		} else {
			tbl.RawSet(key, val)
		}
		if proto.Feedback != nil && pc >= 0 && pc < len(proto.Feedback) {
			proto.Feedback[pc].ObserveKind(uint8(tbl.GetArrayKind()))
			if proto.TableKeyFeedback != nil && pc < len(proto.TableKeyFeedback) {
				proto.TableKeyFeedback[pc].ObserveTableAccess(tbl, key, val, vm.TableAccessKindSet, beforeLen, beforeFieldIdx)
			}
		}
	}
	return nil
}

// ensureFieldCache lazily allocates the FieldCache on the FuncProto if nil.
func ensureFieldCache(proto *vm.FuncProto) {
	if proto.FieldCache == nil {
		proto.FieldCache = make([]runtime.FieldCacheEntry, len(proto.Code))
	}
}

func ensureFieldPolyCache(proto *vm.FuncProto) {
	if proto.FieldPolyCache == nil {
		proto.FieldPolyCache = make([]runtime.FieldPolyCacheEntry, len(proto.Code)*runtime.FieldPolyCacheWays)
	}
}

func ensureTableStringKeyCache(proto *vm.FuncProto) {
	if proto.TableStringKeyCache == nil {
		proto.TableStringKeyCache = make([]runtime.TableStringKeyCacheEntry, len(proto.Code)*runtime.TableStringKeyCacheWays)
	}
}

// handleGetField handles OP_GETFIELD exit: R(A) = R(B).Constants[Bx]
// Populates proto.FieldCache so the native inline cache hits on next access.
func (e *BaselineJITEngine) handleGetField(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB)
	c := int(ctx.BaselineC) // constant index for field name

	absB := base + b
	absA := base + a
	if absB >= len(regs) || absA >= len(regs) {
		return nil
	}
	tblVal := regs[absB]
	if c >= len(proto.Constants) {
		return nil
	}
	fieldName := proto.Constants[c].Str()

	if v, ok := tblVal.FixedRecordRawGetString(fieldName); ok {
		regs[absA] = v
		pc := int(ctx.BaselinePC) - 1
		if fr := tblVal.FixedRecord(); fr != nil && pc >= 0 {
			idx := fr.FieldIndex(fieldName)
			if idx >= 0 {
				ensureFieldCache(proto)
				proto.FieldCache[pc].FieldIdx = idx
				proto.FieldCache[pc].ShapeID = fr.ShapeID()
			}
		}
		if proto.Feedback != nil && pc >= 0 && pc < len(proto.Feedback) {
			proto.Feedback[pc].Result.Observe(v.Type())
		}
		return nil
	}

	if tblVal.IsTable() {
		tbl := tblVal.Table()
		if tbl.HasMetatable() {
			if e.callVM == nil {
				return fmt.Errorf("no callVM for table-field-get exit")
			}
			v, err := e.callVM.TableGetForJIT(tblVal, runtime.StringValue(fieldName))
			if err != nil {
				return err
			}
			regs[absA] = v
			return nil
		}
		// Use the cached path to populate the FieldCache for the native inline cache.
		// BaselinePC is the resume (next) PC, so current instruction PC = BaselinePC - 1.
		pc := int(ctx.BaselinePC) - 1
		ensureFieldCache(proto)
		ensureFieldPolyCache(proto)
		regs[absA] = tbl.RawGetStringCachedPoly(
			fieldName,
			&proto.FieldCache[pc],
			runtime.FieldPolyCacheSlot(proto.FieldPolyCache, pc),
		)
		ensureTableStringKeyCache(proto)
		syncTableStringKeyCacheContext(ctx, proto)
		_ = tbl.RawGetStringDynamicCached(fieldName, runtime.TableStringKeyCacheSlot(proto.TableStringKeyCache, pc))
		// Record type feedback so Tier 2 can specialize.
		if proto.Feedback != nil && pc < len(proto.Feedback) {
			proto.Feedback[pc].Result.Observe(regs[absA].Type())
		}
		if proto.FieldAccessFeedback != nil && pc >= 0 && pc < len(proto.FieldAccessFeedback) {
			proto.FieldAccessFeedback[pc].ObserveFieldCache(proto.FieldCache[pc], regs[absA], 1)
		}
	} else {
		regs[absA] = runtime.NilValue()
	}
	return nil
}

// handleSetField handles OP_SETFIELD exit: R(A).Constants[Bx] = RK(C)
// Populates proto.FieldCache so the native inline cache hits on next access.
func (e *BaselineJITEngine) handleSetField(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB) // constant index for field name
	c := int(ctx.BaselineC) // RK(C) = value

	absA := base + a
	if absA >= len(regs) || b >= len(proto.Constants) {
		return nil
	}
	tblVal := regs[absA]
	fieldName := proto.Constants[b].Str()

	// Resolve RK(C) = value
	var val runtime.Value
	if c >= vm.RKBit {
		val = proto.Constants[c-vm.RKBit]
	} else {
		absC := base + c
		if absC < len(regs) {
			val = regs[absC]
		}
	}

	if tblVal.IsTable() {
		tbl := tblVal.Table()
		if tbl.HasMetatable() {
			if e.callVM == nil {
				return fmt.Errorf("no callVM for table-field-set exit")
			}
			return e.callVM.TableSetForJIT(tblVal, runtime.StringValue(fieldName), val)
		}
		// Use the cached path to populate the FieldCache for the native inline cache.
		// BaselinePC is the resume (next) PC, so current instruction PC = BaselinePC - 1.
		pc := int(ctx.BaselinePC) - 1
		ensureFieldCache(proto)
		tbl.RawSetStringCached(fieldName, val, &proto.FieldCache[pc])
		if proto.FieldAccessFeedback != nil && pc >= 0 && pc < len(proto.FieldAccessFeedback) {
			proto.FieldAccessFeedback[pc].ObserveFieldCache(proto.FieldCache[pc], val, 2)
		}
	}
	return nil
}

// handleSetList handles OP_SETLIST exit.
func (e *BaselineJITEngine) handleSetList(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB) // count
	c := int(ctx.BaselineC) // block

	absA := base + a
	if absA >= len(regs) {
		return nil
	}
	tblVal := regs[absA]
	if !tblVal.IsTable() {
		return fmt.Errorf("SETLIST on non-table")
	}
	tbl := tblVal.Table()
	offset := (c - 1) * 50
	for i := 1; i <= b; i++ {
		idx := absA + i
		if idx < len(regs) {
			tbl.RawSetInt(int64(offset+i), regs[idx])
		}
	}
	return nil
}

// handleAppend handles OP_APPEND exit.
func (e *BaselineJITEngine) handleAppend(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB)
	absA := base + a
	absB := base + b
	if absA >= len(regs) || absB >= len(regs) {
		return nil
	}
	tblVal := regs[absA]
	if tblVal.IsTable() {
		tblVal.Table().Append(regs[absB])
	}
	return nil
}
