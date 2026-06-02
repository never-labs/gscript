//go:build darwin && arm64

// tier1_handlers_misc.go contains the remaining Tier 1 baseline JIT exit handlers
// for less common operations: string concatenation, length, closures, upvalues,
// self/method calls, varargs, generic for loops, and power.

package methodjit

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

// handleConcat handles OP_CONCAT exit: R(A) = R(B)..R(B+1)..R(C)
func (e *BaselineJITEngine) handleConcat(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB) // start register
	c := int(ctx.BaselineC) // end register

	absA := base + a
	start := base + b
	end := base + c + 1
	if absA >= len(regs) || start < 0 || end < start || end > len(regs) {
		return nil
	}
	if e.callVM != nil {
		v, err := e.callVM.ConcatValues(regs[start:end])
		if err != nil {
			return err
		}
		regs[absA] = v
		return nil
	}
	regs[absA] = runtime.ConcatValues(regs[start:end])
	return nil
}

// handleLen handles OP_LEN exit: R(A) = #R(B)
func (e *BaselineJITEngine) handleLen(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB)
	absA := base + a
	absB := base + b
	if absA >= len(regs) || absB >= len(regs) {
		return nil
	}
	v := regs[absB]
	if e.callVM != nil {
		var cache *runtime.FieldCacheEntry
		pc := int(ctx.BaselinePC) - 1
		if proto != nil && pc >= 0 && pc < len(proto.Code) {
			ensureFieldCache(proto)
			cache = &proto.FieldCache[pc]
		}
		result, err := e.callVM.LengthForJIT(v, cache)
		if err != nil {
			return err
		}
		regs[absA] = result
	} else {
		if v.IsTable() {
			regs[absA] = runtime.IntValue(int64(v.Table().Len()))
		} else if v.IsString() {
			regs[absA] = runtime.IntValue(int64(runtime.StringLen(v)))
		} else {
			regs[absA] = runtime.IntValue(0)
		}
	}
	if proto != nil && proto.Feedback != nil {
		pc := int(ctx.BaselinePC) - 1
		if pc >= 0 && pc < len(proto.Feedback) {
			proto.Feedback[pc].Result.Observe(regs[absA].Type())
		}
	}
	return nil
}

// handleClosure handles OP_CLOSURE exit.
// This is complex: needs the parent closure's upvalues. For Tier 1 baseline,
// we exit to Go which creates the closure using the VM.
func (e *BaselineJITEngine) handleClosure(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	a := int(ctx.BaselineA)
	bx := int(ctx.BaselineB)
	if bx >= len(proto.Protos) {
		return fmt.Errorf("closure proto index %d out of range", bx)
	}
	subProto := proto.Protos[bx]
	cl := vm.NewClosure(subProto)
	closeAtCreation := closureReturnedImmediately(proto, int(ctx.BaselinePC)-1, a)
	switch len(subProto.Upvalues) {
	case 0:
	case 1:
		desc := subProto.Upvalues[0]
		if desc.InStack {
			absIdx := base + desc.Index
			if absIdx < len(regs) {
				if closeAtCreation {
					cl.SetInlineClosedUpvalue0(regs[absIdx])
				} else if e.callVM != nil {
					cl.Upvalues[0] = e.callVM.FindOrCreateUpvalue(absIdx)
				} else {
					cl.Upvalues[0] = vm.NewOpenUpvalue(&regs[absIdx], absIdx)
				}
			}
		} else {
			var parentCl *vm.Closure
			if e.callVM != nil {
				parentCl = e.callVM.CurrentClosure()
			}
			if parentCl != nil && desc.Index < len(parentCl.Upvalues) && parentCl.Upvalues[desc.Index] != nil {
				cl.Upvalues[0] = parentCl.Upvalues[desc.Index]
			} else {
				cl.Upvalues[0] = vm.NewOpenUpvalue(new(runtime.Value), 0)
			}
		}
	default:
		// Capture upvalues.
		var parentCl *vm.Closure
		if e.callVM != nil {
			parentCl = e.callVM.CurrentClosure()
		}
		for i, desc := range subProto.Upvalues {
			if desc.InStack {
				absIdx := base + desc.Index
				if absIdx < len(regs) {
					if closeAtCreation {
						cl.Upvalues[i] = vm.NewClosedUpvalue(regs[absIdx])
					} else if e.callVM != nil {
						cl.Upvalues[i] = e.callVM.FindOrCreateUpvalue(absIdx)
					} else {
						cl.Upvalues[i] = vm.NewOpenUpvalue(&regs[absIdx], absIdx)
					}
				}
			} else {
				// Parent upvalue: copy from the parent closure's upvalue list.
				if parentCl != nil && desc.Index < len(parentCl.Upvalues) && parentCl.Upvalues[desc.Index] != nil {
					cl.Upvalues[i] = parentCl.Upvalues[desc.Index]
				} else {
					cl.Upvalues[i] = vm.NewOpenUpvalue(new(runtime.Value), 0)
				}
			}
		}
	}
	absA := base + a
	if absA < len(regs) {
		regs[absA] = runtime.VMClosureFastValue(unsafe.Pointer(cl))
	}
	return nil
}

func closureReturnedImmediately(proto *vm.FuncProto, pc, slot int) bool {
	if proto == nil || pc < 0 || pc+1 >= len(proto.Code) {
		return false
	}
	ret := proto.Code[pc+1]
	return vm.DecodeOp(ret) == vm.OP_RETURN && vm.DecodeA(ret) == slot && vm.DecodeB(ret) == 2
}

// handleClose handles OP_CLOSE exit: close upvalues >= R(A).
func (e *BaselineJITEngine) handleClose(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	if e.callVM == nil {
		return nil
	}
	a := int(ctx.BaselineA)
	e.callVM.CloseUpvalues(base + a)
	return nil
}

// handleGetUpval handles OP_GETUPVAL exit: R(A) = Upvalues[B].Get()
func (e *BaselineJITEngine) handleGetUpval(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	if e.callVM == nil {
		return fmt.Errorf("no callVM for GETUPVAL")
	}
	cl := e.callVM.CurrentClosure()
	if cl == nil {
		return fmt.Errorf("GETUPVAL: no current closure")
	}
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB)
	if b >= len(cl.Upvalues) || cl.Upvalues[b] == nil {
		return fmt.Errorf("GETUPVAL: upvalue %d out of range", b)
	}
	absA := base + a
	if absA < len(regs) {
		regs[absA] = cl.Upvalues[b].Get()
	}
	return nil
}

// handleSetUpval handles OP_SETUPVAL exit: Upvalues[B].Set(R(A))
func (e *BaselineJITEngine) handleSetUpval(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	if e.callVM == nil {
		return fmt.Errorf("no callVM for SETUPVAL")
	}
	cl := e.callVM.CurrentClosure()
	if cl == nil {
		return fmt.Errorf("SETUPVAL: no current closure")
	}
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB)
	if b >= len(cl.Upvalues) || cl.Upvalues[b] == nil {
		return fmt.Errorf("SETUPVAL: upvalue %d out of range", b)
	}
	absA := base + a
	if absA < len(regs) {
		cl.Upvalues[b].Set(regs[absA])
	}
	return nil
}

// handleSelf handles OP_SELF exit: R(A+1) = R(B); R(A) = R(B)[RK(C)]
// When the key is a constant string (the common case for method calls),
// uses the cached path to populate FieldCache for the native inline cache.
func (e *BaselineJITEngine) handleSelf(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB)
	c := int(ctx.BaselineC)

	absA := base + a
	absB := base + b
	if absA+1 >= len(regs) || absB >= len(regs) {
		return nil
	}

	obj := regs[absB]
	regs[absA+1] = obj

	if obj.IsTable() {
		tbl := obj.Table()
		// For constant string keys (the common case: method names), use cached lookup.
		if c >= vm.RKBit {
			key := proto.Constants[c-vm.RKBit]
			if key.IsString() {
				pc := int(ctx.BaselinePC) - 1
				ensureFieldCache(proto)
				ensureFieldPolyCache(proto)
				regs[absA] = tbl.RawGetStringCachedPoly(
					key.Str(),
					&proto.FieldCache[pc],
					runtime.FieldPolyCacheSlot(proto.FieldPolyCache, pc),
				)
				return nil
			}
			regs[absA] = tbl.RawGet(key)
		} else {
			absC := base + c
			if absC < len(regs) {
				regs[absA] = tbl.RawGet(regs[absC])
			}
		}
	} else {
		regs[absA] = runtime.NilValue()
	}
	return nil
}

// handleVararg handles OP_VARARG exit.
func (e *BaselineJITEngine) handleVararg(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	if e.callVM == nil {
		return fmt.Errorf("no callVM for VARARG")
	}
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB)
	varargs := e.callVM.CurrentVarargs()

	if b == 0 && proto != nil {
		pc := int(ctx.BaselinePC) - 1
		if pc+1 < len(proto.Code) {
			next := proto.Code[pc+1]
			if vm.DecodeOp(next) == vm.OP_CALL && vm.DecodeB(next) == 0 {
				callA := vm.DecodeA(next)
				if callA+2 == a {
					absSlot := base + callA
					if absSlot+1 < len(regs) {
						if gf := regs[absSlot].GoFunction(); gf != nil &&
							gf.NativeKind == runtime.NativeKindStdSelect &&
							gf.NativeData == runtime.StdSelectIdentityPtr() {
							if ok, err := tier1ExecuteStdSelectVarargFast(regs, absSlot, vm.DecodeC(next), regs[absSlot+1], varargs, gf); err != nil {
								return err
							} else if !ok {
								if err := e.callVM.ExecuteStdSelectVarargCall(absSlot, vm.DecodeC(next), regs[absSlot+1], varargs, gf); err != nil {
									return err
								}
							}
							resumePC, err := e.foldStdSelectVarargCluster(regs, base, proto, pc+2, varargs, pc+2, regs[absSlot], gf)
							if err != nil {
								return err
							}
							ctx.BaselinePC = int64(resumePC)
							return nil
						}
					}
				}
			}
		}
	}

	absA := base + a
	if b == 0 {
		for i, v := range varargs {
			idx := absA + i
			if idx < len(regs) {
				regs[idx] = v
			}
		}
		e.callVM.SetTop(absA + len(varargs))
		return nil
	}

	n := b - 1
	for i := 0; i < n; i++ {
		idx := absA + i
		if idx >= len(regs) {
			continue
		}
		if i < len(varargs) {
			regs[idx] = varargs[i]
		} else {
			regs[idx] = runtime.NilValue()
		}
	}
	return nil
}

func (e *BaselineJITEngine) foldStdSelectVarargCluster(regs []runtime.Value, base int, proto *vm.FuncProto, startPC int, varargs []runtime.Value, fallbackPC int, selectFn runtime.Value, gf *runtime.GoFunction) (int, error) {
	if e.callVM == nil || proto == nil || startPC >= len(proto.Code) || gf == nil {
		return fallbackPC, nil
	}
	pc := startPC
	for pc+3 < len(proto.Code) {
		getInst := proto.Code[pc]
		selInst := proto.Code[pc+1]
		varInst := proto.Code[pc+2]
		callInst := proto.Code[pc+3]
		if vm.DecodeOp(getInst) != vm.OP_GETGLOBAL ||
			vm.DecodeOp(varInst) != vm.OP_VARARG ||
			vm.DecodeOp(callInst) != vm.OP_CALL ||
			vm.DecodeB(varInst) != 0 ||
			vm.DecodeB(callInst) != 0 {
			break
		}
		callA := vm.DecodeA(callInst)
		if vm.DecodeA(getInst) != callA || vm.DecodeA(selInst) != callA+1 || vm.DecodeA(varInst) != callA+2 {
			break
		}
		nameK := vm.DecodeBx(getInst)
		if nameK >= len(proto.Constants) || !proto.Constants[nameK].IsString() || proto.Constants[nameK].Str() != "select" {
			break
		}
		selector, ok := tier1ConstSelector(proto, selInst)
		if !ok {
			break
		}
		absSlot := base + callA
		if absSlot+1 >= len(regs) {
			break
		}
		regs[absSlot] = selectFn
		regs[absSlot+1] = selector
		if ok, err := tier1ExecuteStdSelectVarargFast(regs, absSlot, vm.DecodeC(callInst), selector, varargs, gf); err != nil {
			return pc, err
		} else if !ok {
			if err := e.callVM.ExecuteStdSelectVarargCall(absSlot, vm.DecodeC(callInst), selector, varargs, gf); err != nil {
				return pc, err
			}
		}
		pc += 4
	}
	return pc, nil
}

func tier1ConstSelector(proto *vm.FuncProto, inst uint32) (runtime.Value, bool) {
	switch vm.DecodeOp(inst) {
	case vm.OP_LOADINT:
		return runtime.IntValue(int64(vm.DecodesBx(inst))), true
	case vm.OP_LOADK:
		bx := vm.DecodeBx(inst)
		if bx >= len(proto.Constants) {
			return runtime.NilValue(), false
		}
		return proto.Constants[bx], true
	default:
		return runtime.NilValue(), false
	}
}

func tier1ExecuteStdSelectVarargFast(regs []runtime.Value, absSlot, rawC int, selector runtime.Value, varargs []runtime.Value, gf *runtime.GoFunction) (bool, error) {
	if rawC != 2 || gf == nil {
		return false, nil
	}
	if selector.IsString() && selector.Str() == "#" {
		runtime.RecordRuntimePathNativeCallFastFor(gf)
		if absSlot < len(regs) {
			regs[absSlot] = runtime.IntValue(int64(len(varargs)))
		}
		return true, nil
	}
	if selector.RawType() != runtime.TypeInt {
		return false, nil
	}
	idx := int(selector.RawInt())
	argCount := len(varargs) + 1
	if idx < 0 {
		idx = argCount + idx
	}
	if idx < 1 {
		return true, fmt.Errorf("bad argument #1 to 'select' (index out of range)")
	}
	runtime.RecordRuntimePathNativeCallFastFor(gf)
	if absSlot >= len(regs) {
		return true, nil
	}
	if idx > len(varargs) {
		regs[absSlot] = runtime.NilValue()
	} else {
		regs[absSlot] = varargs[idx-1]
	}
	return true, nil
}

// handleTForCall handles OP_TFORCALL exit.
func (e *BaselineJITEngine) handleTForCall(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	if e.callVM == nil {
		return fmt.Errorf("no callVM for TFORCALL")
	}
	a := int(ctx.BaselineA)
	c := int(ctx.BaselineC)

	absA := base + a
	if absA+2 >= len(regs) {
		return nil
	}
	fnVal := regs[absA]
	if fnVal.IsTable() {
		pairsFn := e.callVM.GetGlobal("pairs")
		if pairsFn.IsNil() {
			return fmt.Errorf("no pairs function for table TFORCALL")
		}
		results, err := e.callVM.CallValue(pairsFn, []runtime.Value{fnVal})
		if err != nil {
			return err
		}
		for i := 0; i < 3; i++ {
			idx := absA + i
			if idx >= len(regs) {
				continue
			}
			if i < len(results) {
				regs[idx] = results[i]
			} else {
				regs[idx] = runtime.NilValue()
			}
		}
		fnVal = regs[absA]
	}
	if fnVal.IsChannel() {
		ch := fnVal.Channel()
		val, ok := ch.Recv()
		for i := 0; i < c; i++ {
			idx := absA + 3 + i
			if idx >= len(regs) {
				continue
			}
			if i == 0 && ok {
				regs[idx] = val
			} else {
				regs[idx] = runtime.NilValue()
			}
		}
		return nil
	}
	if gf := fnVal.GoFunction(); gf != nil && gf.FastArg1Ret2 != nil {
		runtime.RecordRuntimePathNativeCallFastFor(gf)
		r0, r1, n, err := gf.FastArg1Ret2(regs[absA+1])
		if err != nil {
			return err
		}
		for i := 0; i < c; i++ {
			idx := absA + 3 + i
			if idx >= len(regs) {
				continue
			}
			switch {
			case i == 0 && n > 0:
				regs[idx] = r0
			case i == 1 && n > 1:
				regs[idx] = r1
			default:
				regs[idx] = runtime.NilValue()
			}
		}
		return nil
	}
	if gf := fnVal.GoFunction(); gf != nil && gf.FastArg2Ret2 != nil {
		runtime.RecordRuntimePathNativeCallFastFor(gf)
		r0, r1, n, err := gf.FastArg2Ret2(regs[absA+1], regs[absA+2])
		if err != nil {
			return err
		}
		for i := 0; i < c; i++ {
			idx := absA + 3 + i
			if idx >= len(regs) {
				continue
			}
			switch {
			case i == 0 && n > 0:
				regs[idx] = r0
			case i == 1 && n > 1:
				regs[idx] = r1
			default:
				regs[idx] = runtime.NilValue()
			}
		}
		return nil
	}
	args := []runtime.Value{regs[absA+1], regs[absA+2]}
	results, err := e.callVM.CallValue(fnVal, args)
	if err != nil {
		return err
	}
	for i := 0; i < c; i++ {
		idx := absA + 3 + i
		if idx < len(regs) {
			if i < len(results) {
				regs[idx] = results[i]
			} else {
				regs[idx] = runtime.NilValue()
			}
		}
	}
	return nil
}

// handleTForLoop handles OP_TFORLOOP exit.
func (e *BaselineJITEngine) handleTForLoop(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	// TFORLOOP is handled natively (compare + branch). Should not reach here.
	return fmt.Errorf("TFORLOOP should not op-exit")
}

// handlePow handles OP_POW exit.
func (e *BaselineJITEngine) handlePow(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB)
	c := int(ctx.BaselineC)
	absA := base + a

	var bv, cv runtime.Value
	if b >= vm.RKBit {
		bv = proto.Constants[b-vm.RKBit]
	} else {
		bv = regs[base+b]
	}
	if c >= vm.RKBit {
		cv = proto.Constants[c-vm.RKBit]
	} else {
		cv = regs[base+c]
	}

	var baseF, expF float64
	if bv.IsInt() {
		baseF = float64(bv.Int())
	} else {
		baseF = bv.Float()
	}
	if cv.IsInt() {
		expF = float64(cv.Int())
	} else {
		expF = cv.Float()
	}

	if absA < len(regs) {
		regs[absA] = runtime.FloatValue(math.Pow(baseF, expF))
	}
	return nil
}
