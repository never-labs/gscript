//go:build darwin && arm64

// tiering_manager_exit.go implements exit handlers for the TieringManager's
// Tier 2 execute loop. These handlers are invoked when Tier 2 JIT code
// encounters operations it cannot handle natively (calls, globals, tables,
// generic ops).
//
// Slot indices in ExecContext are relative to the callee's frame (base=0 in
// JIT), so we add `base` for absolute positions.

package methodjit

import (
	"fmt"

	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

// executeGlobalExit handles a global-exit in the TieringManager's Tier 2 path.
// After resolving the global value, populates the per-instruction value cache
// in CompiledFunction.GlobalCache so subsequent accesses hit the fast path.
func (tm *TieringManager) executeGlobalExit(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto, cf *CompiledFunction) error {
	if tm.callVM == nil {
		return fmt.Errorf("no callVM set for global-exit")
	}

	globalSlot := int(ctx.GlobalSlot)
	constIdx := int(ctx.GlobalConst)

	if constIdx >= len(proto.Constants) {
		return fmt.Errorf("global constant index %d out of range (len %d)", constIdx, len(proto.Constants))
	}
	globalName := proto.Constants[constIdx].Str()
	val := tm.callVM.GetGlobal(globalName)

	absSlot := base + globalSlot
	if absSlot < len(regs) {
		regs[absSlot] = val
	}

	// Populate the per-instruction global value cache.
	cacheIdx := int(ctx.GlobalCacheIdx)
	if cacheIdx >= 0 && cf != nil && cf.GlobalCache != nil && cacheIdx < len(cf.GlobalCache) {
		valBits := uint64(val)
		if valBits != 0 { // don't cache zero (used as "empty" sentinel)
			// If the generation has changed since we last cached, clear all
			// entries before repopulating. Without this, updating GlobalCacheGen
			// would make other entries' stale values appear valid.
			if cf.GlobalCacheGen != tm.tier1.globalCacheGen {
				for i := range cf.GlobalCache {
					cf.GlobalCache[i] = 0
				}
			}
			cf.GlobalCache[cacheIdx] = valBits
			cf.GlobalCacheGen = tm.tier1.globalCacheGen
		}
	}

	return nil
}

// executeTableExit handles table operations in the TieringManager's Tier 2 path.
func (tm *TieringManager) executeTableExit(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto, cf *CompiledFunction) error {
	switch ctx.TableOp {
	case TableOpNewTable:
		arrayHint := int(ctx.TableAux)
		hashHint, arrayKind := unpackNewTableAux2(ctx.TableAux2)
		var tbl *runtime.Table
		if unpackNewTableDenseMixed(ctx.TableAux2) {
			tbl = cf.allocateDenseMixedNewTableForExit(int(ctx.TableExitID), arrayHint, hashHint)
		} else {
			tbl = cf.allocateNewTableForExit(int(ctx.TableExitID), arrayHint, hashHint, arrayKind)
		}
		absSlot := base + int(ctx.TableSlot)
		if absSlot < len(regs) {
			regs[absSlot] = runtime.FreshTableValue(tbl)
		}

	case TableOpNewFixedTable2:
		ctorIdx := int(ctx.TableAux)
		absSlot := base + int(ctx.TableSlot)
		absVal1 := base + int(ctx.TableKeySlot)
		absVal2 := base + int(ctx.TableValSlot)
		if proto != nil && ctorIdx >= 0 && ctorIdx < len(proto.TableCtors2) &&
			absVal1 >= 0 && absVal1 < len(regs) &&
			absVal2 >= 0 && absVal2 < len(regs) &&
			absSlot >= 0 && absSlot < len(regs) {
			ctor := &proto.TableCtors2[ctorIdx].Runtime
			tbl := cf.allocateFixedTable2ForExit(int(ctx.TableExitID), ctor, regs[absVal1], regs[absVal2])
			regs[absSlot] = runtime.FreshTableValue(tbl)
		}

	case TableOpNewFixedTableN:
		ctorIdx := int(ctx.TableAux)
		absSlot := base + int(ctx.TableSlot)
		instrID := int(ctx.TableExitID)
		argSlots := cf.FixedTableArgSlots[instrID]
		if proto != nil && ctorIdx >= 0 && ctorIdx < len(proto.TableCtorsN) &&
			absSlot >= 0 && absSlot < len(regs) &&
			len(argSlots) == int(ctx.TableAux2) {
			vals := make([]runtime.Value, len(argSlots))
			ok := true
			for i, slot := range argSlots {
				abs := base + slot
				if abs < 0 || abs >= len(regs) {
					ok = false
					break
				}
				vals[i] = regs[abs]
			}
			if ok {
				ctor := &proto.TableCtorsN[ctorIdx].Runtime
				regs[absSlot] = cf.allocateFixedTableNValueForExit(instrID, ctor, vals)
			}
		}

	case TableOpGetTable:
		absTable := base + int(ctx.TableSlot)
		absKey := base + int(ctx.TableKeySlot)
		absResult := base + int(ctx.TableAux)
		if absTable < len(regs) && absKey < len(regs) {
			tblVal := regs[absTable]
			keyVal := regs[absKey]
			var result runtime.Value
			if tblVal.IsTable() && !tblVal.Table().HasMetatable() {
				pc := int(ctx.TableAux2)
				if keyVal.IsString() && proto != nil && pc >= 0 {
					ensureTableStringKeyCache(proto)
					syncTableStringKeyCacheContext(ctx, proto)
					result = tblVal.Table().RawGetStringDynamicCached(
						keyVal.Str(),
						runtime.TableStringKeyCacheSlot(proto.TableStringKeyCache, pc),
					)
				} else {
					result = tblVal.Table().RawGet(keyVal)
				}
			} else {
				if tm.callVM == nil {
					return fmt.Errorf("no callVM set for table-get exit")
				}
				var err error
				result, err = tm.callVM.TableGetForJIT(tblVal, keyVal)
				if err != nil {
					return err
				}
			}
			if absResult < len(regs) {
				regs[absResult] = result
			}
			pc := int(ctx.TableAux2)
			if proto != nil && proto.TableKeyFeedback != nil && pc >= 0 && pc < len(proto.TableKeyFeedback) && tblVal.IsTable() {
				proto.TableKeyFeedback[pc].ObserveTableAccess(tblVal.Table(), keyVal, result, vm.TableAccessKindGet, -1, -1)
			}
		}

	case TableOpSetTable:
		absTable := base + int(ctx.TableSlot)
		absKey := base + int(ctx.TableKeySlot)
		absVal := base + int(ctx.TableValSlot)
		if absTable < len(regs) && absKey < len(regs) && absVal < len(regs) {
			tblVal := regs[absTable]
			keyVal := regs[absKey]
			valVal := regs[absVal]
			if tblVal.IsTable() {
				pc := int(ctx.TableAux2)
				tbl := tblVal.Table()
				if tbl.HasMetatable() {
					if tm.callVM == nil {
						return fmt.Errorf("no callVM set for table-set exit")
					}
					return tm.callVM.TableSetForJIT(tblVal, keyVal, valVal)
				}
				beforeLen, beforeFieldIdx := -1, -1
				if keyVal.IsInt() {
					beforeLen = tbl.Len()
				} else if keyVal.IsString() {
					beforeFieldIdx = tbl.FieldIndex(keyVal.Str())
				}
				if keyVal.IsString() && proto != nil && pc >= 0 {
					ensureTableStringKeyCache(proto)
					syncTableStringKeyCacheContext(ctx, proto)
					tbl.RawSetStringDynamicCached(
						keyVal.Str(),
						valVal,
						runtime.TableStringKeyCacheSlot(proto.TableStringKeyCache, pc),
					)
				} else {
					tbl.RawSet(keyVal, valVal)
				}
				if proto != nil && proto.TableKeyFeedback != nil && pc >= 0 && pc < len(proto.TableKeyFeedback) {
					proto.TableKeyFeedback[pc].ObserveTableAccess(tbl, keyVal, valVal, vm.TableAccessKindSet, beforeLen, beforeFieldIdx)
				}
			}
		}

	case TableOpBoolArrayFill:
		absTable := base + int(ctx.TableSlot)
		absStart := base + int(ctx.TableKeySlot)
		absEnd := base + int(ctx.TableValSlot)
		absStep := base + int(ctx.TableAux2)
		if absTable < len(regs) && absStart < len(regs) && absEnd < len(regs) {
			tblVal := regs[absTable]
			startVal := regs[absStart]
			endVal := regs[absEnd]
			if tblVal.IsTable() && startVal.IsInt() && endVal.IsInt() {
				val := runtime.BoolValue(ctx.TableAux != 0)
				step := int64(1)
				if absStep > 0 && absStep < len(regs) && regs[absStep].IsInt() {
					step = regs[absStep].Int()
				}
				if step <= 0 {
					break
				}
				tbl := tblVal.Table()
				for i, end := startVal.Int(), endVal.Int(); i <= end; i += step {
					tbl.RawSetInt(i, val)
					if i == end || i > end-step {
						break
					}
				}
			}
		}

	case TableOpBoolArrayCount:
		absTable := base + int(ctx.TableSlot)
		absStart := base + int(ctx.TableKeySlot)
		absEnd := base + int(ctx.TableValSlot)
		absResult := base + int(ctx.TableAux)
		if absTable >= 0 && absTable < len(regs) &&
			absStart >= 0 && absStart < len(regs) &&
			absEnd >= 0 && absEnd < len(regs) &&
			absResult >= 0 && absResult < len(regs) {
			tblVal := regs[absTable]
			startVal := regs[absStart]
			endVal := regs[absEnd]
			if !startVal.IsInt() || !endVal.IsInt() {
				return fmt.Errorf("boolcount table exit: bounds are not ints")
			}
			start, end := startVal.Int(), endVal.Int()
			count := int64(0)
			if start <= end && tblVal.IsTable() && !tblVal.Table().HasMetatable() {
				tbl := tblVal.Table()
				for i := start; i <= end; i++ {
					if tbl.RawGetInt(i).Truthy() {
						count++
					}
					if i == end {
						break
					}
				}
			} else if start <= end {
				if tm.callVM == nil {
					return fmt.Errorf("no callVM set for boolcount table-get exit")
				}
				for i := start; i <= end; i++ {
					v, err := tm.callVM.TableGetForJIT(tblVal, runtime.IntValue(i))
					if err != nil {
						return err
					}
					if v.Truthy() {
						count++
					}
					if i == end {
						break
					}
				}
			}
			regs[absResult] = runtime.IntValue(count)
		}

	case TableOpGetField:
		absTable := base + int(ctx.TableSlot)
		constIdx := int(ctx.TableAux)
		absResult := base + int(ctx.TableAux2)
		if absTable < len(regs) && constIdx < len(proto.Constants) {
			tblVal := regs[absTable]
			fieldName := proto.Constants[constIdx].Str()
			var result runtime.Value
			if tblVal.IsTable() && !tblVal.Table().HasMetatable() {
				pc := int(ctx.TableKeySlot)
				if tier2TableExitFieldCachePC(proto, pc, vm.OP_GETFIELD, constIdx) {
					ensureFieldCache(proto)
					ensureFieldPolyCache(proto)
					result = tblVal.Table().RawGetStringCachedPoly(
						fieldName,
						&proto.FieldCache[pc],
						runtime.FieldPolyCacheSlot(proto.FieldPolyCache, pc),
					)
					if proto.FieldAccessFeedback != nil && pc < len(proto.FieldAccessFeedback) {
						proto.FieldAccessFeedback[pc].ObserveFieldCache(proto.FieldCache[pc], result, 1)
					}
					ensureTableStringKeyCache(proto)
					syncTableStringKeyCacheContext(ctx, proto)
					_ = tblVal.Table().RawGetStringDynamicCached(fieldName, runtime.TableStringKeyCacheSlot(proto.TableStringKeyCache, pc))
				} else {
					result = tblVal.Table().RawGetString(fieldName)
				}
			} else {
				if tm.callVM == nil {
					return fmt.Errorf("no callVM set for table-get exit")
				}
				var err error
				result, err = tm.callVM.TableGetForJIT(tblVal, runtime.StringValue(fieldName))
				if err != nil {
					return err
				}
			}
			if absResult < len(regs) {
				regs[absResult] = result
			}
		}

	case TableOpSetField:
		absTable := base + int(ctx.TableSlot)
		constIdx := int(ctx.TableAux)
		absVal := base + int(ctx.TableValSlot)
		if absTable < len(regs) && constIdx < len(proto.Constants) && absVal < len(regs) {
			tblVal := regs[absTable]
			fieldName := proto.Constants[constIdx].Str()
			valVal := regs[absVal]
			if tblVal.IsTable() {
				pc := int(ctx.TableKeySlot)
				if tblVal.Table().HasMetatable() {
					if tm.callVM == nil {
						return fmt.Errorf("no callVM set for table-field-set exit")
					}
					return tm.callVM.TableSetForJIT(tblVal, runtime.StringValue(fieldName), valVal)
				}
				if tier2TableExitFieldCachePC(proto, pc, vm.OP_SETFIELD, constIdx) {
					ensureFieldCache(proto)
					tblVal.Table().RawSetStringCached(fieldName, valVal, &proto.FieldCache[pc])
					if proto.FieldAccessFeedback != nil && pc < len(proto.FieldAccessFeedback) {
						proto.FieldAccessFeedback[pc].ObserveFieldCache(proto.FieldCache[pc], valVal, 2)
					}
				} else {
					tblVal.Table().RawSetString(fieldName, valVal)
				}
			}
		}

	default:
		return fmt.Errorf("unknown table op %d", ctx.TableOp)
	}
	return nil
}
