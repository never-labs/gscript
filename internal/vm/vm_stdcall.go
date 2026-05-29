package vm

// Standard-library fast-call dispatch helpers, split verbatim from vm.go.

import (
	"fmt"
	"github.com/gscript/gscript/internal/runtime"
	"math"
)

func (vm *VM) executeStdSelectCall(absSlot, nArgs, rawC int, gf *runtime.GoFunction) error {
	if nArgs == 0 || absSlot+1 >= len(vm.regs) {
		return fmt.Errorf("bad argument #1 to 'select'")
	}
	start, countOnly, err := runtime.SelectReturnRange(vm.regs[absSlot+1], nArgs)
	if err != nil {
		return err
	}
	if err := vm.recordFastNativeCall(gf); err != nil {
		return err
	}
	if countOnly {
		vm.storeStdSelectOne(absSlot, rawC, runtime.IntValue(int64(start)))
		return nil
	}
	valueStart := absSlot + 1 + start
	valueEnd := absSlot + 1 + nArgs
	if start >= nArgs || valueStart >= len(vm.regs) {
		vm.storeStdSelectResults(absSlot, rawC, nil)
		return nil
	}
	if valueEnd > len(vm.regs) {
		valueEnd = len(vm.regs)
	}
	vm.storeStdSelectRange(absSlot, rawC, valueStart, valueEnd)
	return nil
}

func (vm *VM) ExecuteStdSelectVarargCall(absSlot, rawC int, selector runtime.Value, varargs []runtime.Value, gf *runtime.GoFunction) error {
	if rawC == 2 {
		if selector.IsString() && selector.Str() == "#" {
			if err := vm.recordFastNativeCall(gf); err != nil {
				return err
			}
			vm.regs[absSlot] = runtime.IntValue(int64(len(varargs)))
			return nil
		}
		if selector.RawType() == runtime.TypeInt {
			idx := int(selector.RawInt())
			argCount := len(varargs) + 1
			if idx < 0 {
				idx = argCount + idx
			}
			if idx < 1 {
				return fmt.Errorf("bad argument #1 to 'select' (index out of range)")
			}
			if err := vm.recordFastNativeCall(gf); err != nil {
				return err
			}
			if idx > len(varargs) {
				vm.regs[absSlot] = runtime.NilValue()
			} else {
				vm.regs[absSlot] = varargs[idx-1]
			}
			return nil
		}
	}
	start, countOnly, err := runtime.SelectReturnRange(selector, len(varargs)+1)
	if err != nil {
		return err
	}
	if err := vm.recordFastNativeCall(gf); err != nil {
		return err
	}
	if countOnly {
		vm.storeStdSelectOne(absSlot, rawC, runtime.IntValue(int64(start)))
		return nil
	}
	if start > len(varargs) {
		vm.storeStdSelectResults(absSlot, rawC, nil)
		return nil
	}
	vm.storeStdSelectResults(absSlot, rawC, varargs[start-1:])
	return nil
}

func (vm *VM) tryExecuteStdSelectVarargPeephole(frame *CallFrame, base, varargA int, varargs []runtime.Value) (bool, error) {
	if frame.pc >= len(frame.closure.Proto.Code) || varargA < 2 {
		return false, nil
	}
	callInst := frame.closure.Proto.Code[frame.pc]
	if DecodeOp(callInst) != OP_CALL || DecodeB(callInst) != 0 {
		return false, nil
	}
	callA := DecodeA(callInst)
	if callA+2 != varargA {
		return false, nil
	}
	absSlot := base + callA
	if absSlot+1 >= len(vm.regs) {
		return false, nil
	}
	gf := vm.regs[absSlot].GoFunction()
	if gf == nil || gf.NativeKind != runtime.NativeKindStdSelect || gf.NativeData != runtime.StdSelectIdentityPtr() {
		return false, nil
	}
	if err := vm.ExecuteStdSelectVarargCall(absSlot, DecodeC(callInst), vm.regs[absSlot+1], varargs, gf); err != nil {
		return true, err
	}
	frame.pc++
	return true, nil
}

func (vm *VM) storeStdSelectOne(absSlot, rawC int, value runtime.Value) {
	if rawC == 0 {
		if absSlot < len(vm.regs) {
			vm.regs[absSlot] = value
		}
		vm.top = absSlot + 1
		return
	}
	nr := rawC - 1
	if nr <= 0 {
		return
	}
	if absSlot < len(vm.regs) {
		vm.regs[absSlot] = value
	}
	for i := 1; i < nr; i++ {
		if idx := absSlot + i; idx < len(vm.regs) {
			vm.regs[idx] = runtime.NilValue()
		}
	}
}

func (vm *VM) storeStdSelectRange(absSlot, rawC, valueStart, valueEnd int) {
	if valueEnd < valueStart {
		valueEnd = valueStart
	}
	if rawC == 0 {
		n := valueEnd - valueStart
		for i := 0; i < n; i++ {
			dst := absSlot + i
			src := valueStart + i
			if dst < len(vm.regs) && src < len(vm.regs) {
				vm.regs[dst] = vm.regs[src]
			}
		}
		vm.top = absSlot + n
		return
	}
	nr := rawC - 1
	for i := 0; i < nr; i++ {
		dst := absSlot + i
		if dst >= len(vm.regs) {
			continue
		}
		src := valueStart + i
		if src < valueEnd && src < len(vm.regs) {
			vm.regs[dst] = vm.regs[src]
		} else {
			vm.regs[dst] = runtime.NilValue()
		}
	}
}

func (vm *VM) storeStdSelectResults(absSlot, rawC int, results []runtime.Value) {
	if rawC == 0 {
		for i, r := range results {
			if idx := absSlot + i; idx < len(vm.regs) {
				vm.regs[idx] = r
			}
		}
		vm.top = absSlot + len(results)
		return
	}
	nr := rawC - 1
	for i := 0; i < nr; i++ {
		idx := absSlot + i
		if idx >= len(vm.regs) {
			continue
		}
		if i < len(results) {
			vm.regs[idx] = results[i]
		} else {
			vm.regs[idx] = runtime.NilValue()
		}
	}
}

func (vm *VM) storeFixedFastCallResult(absSlot, rawC int, r0, r1 runtime.Value, n int) {
	if rawC == 0 {
		switch {
		case n <= 0:
			vm.top = absSlot
		case n == 1:
			vm.regs[absSlot] = r0
			vm.top = absSlot + 1
		default:
			vm.regs[absSlot] = r0
			vm.regs[absSlot+1] = r1
			vm.top = absSlot + 2
		}
		return
	}
	nr := rawC - 1
	if nr <= 0 {
		return
	}
	if n >= 1 {
		vm.regs[absSlot] = r0
	} else {
		vm.regs[absSlot] = runtime.NilValue()
	}
	if nr <= 1 {
		return
	}
	if n >= 2 {
		vm.regs[absSlot+1] = r1
	} else {
		vm.regs[absSlot+1] = runtime.NilValue()
	}
	for i := 2; i < nr; i++ {
		vm.regs[absSlot+i] = runtime.NilValue()
	}
}

func (vm *VM) executeDirectGoFunctionFastCall(gf *runtime.GoFunction, absSlot, nArgs, rawC int) (bool, error) {
	if gf == nil || nArgs < 1 || nArgs > 8 || absSlot+nArgs >= len(vm.regs) {
		return false, nil
	}
	if nArgs == 7 {
		return false, nil
	}
	hasFastPath := (nArgs == 1 && (gf.FastArg1Ret2 != nil || gf.FastArg1 != nil)) ||
		(nArgs == 2 && (gf.FastArg2Ret2 != nil || gf.FastArg2 != nil)) ||
		(nArgs == 3 && gf.FastArg3 != nil) ||
		(nArgs == 4 && gf.FastArg4 != nil) ||
		(nArgs == 5 && gf.FastArg5 != nil) ||
		(nArgs == 6 && gf.FastArg6 != nil) ||
		(nArgs == 8 && gf.FastArg8 != nil)
	if !hasFastPath {
		return false, nil
	}
	if err := vm.checkNativeCallBudget(); err != nil {
		return true, err
	}
	var err error
	if err = vm.emitDebugHook("call", "native", gf.Name, runtime.NilValue()); err != nil {
		return true, err
	}
	a0 := vm.regs[absSlot+1]
	r0, r1 := runtime.NilValue(), runtime.NilValue()
	n := 1
	switch nArgs {
	case 1:
		if gf.FastArg1Ret2 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			r0, r1, n, err = gf.FastArg1Ret2(a0)
		} else if gf.FastArg1 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			r0, err = gf.FastArg1(a0)
		}
	case 2:
		a1 := vm.regs[absSlot+2]
		if gf.FastArg2Ret2 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			r0, r1, n, err = gf.FastArg2Ret2(a0, a1)
		} else if gf.FastArg2 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			r0, err = gf.FastArg2(a0, a1)
		}
	case 3:
		if gf.FastArg3 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			r0, err = gf.FastArg3(a0, vm.regs[absSlot+2], vm.regs[absSlot+3])
		}
	case 4:
		if gf.FastArg4 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			r0, err = gf.FastArg4(a0, vm.regs[absSlot+2], vm.regs[absSlot+3], vm.regs[absSlot+4])
		}
	case 5:
		if gf.FastArg5 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			r0, err = gf.FastArg5(a0, vm.regs[absSlot+2], vm.regs[absSlot+3], vm.regs[absSlot+4], vm.regs[absSlot+5])
		}
	case 6:
		if gf.FastArg6 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			r0, err = gf.FastArg6(a0, vm.regs[absSlot+2], vm.regs[absSlot+3], vm.regs[absSlot+4], vm.regs[absSlot+5], vm.regs[absSlot+6])
		}
	case 8:
		if gf.FastArg8 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			r0, err = gf.FastArg8(a0, vm.regs[absSlot+2], vm.regs[absSlot+3], vm.regs[absSlot+4], vm.regs[absSlot+5], vm.regs[absSlot+6], vm.regs[absSlot+7], vm.regs[absSlot+8])
		}
	}
	if err != nil {
		_ = vm.emitDebugHook("error", "native", gf.Name, runtime.StringValue(err.Error()))
		return true, err
	}
	if err = vm.emitDebugHook("return", "native", gf.Name, runtime.NilValue()); err != nil {
		return true, err
	}
	vm.storeFixedFastCallResult(absSlot, rawC, r0, r1, n)
	return true, nil
}

// ExecuteStdIPairsCall handles the standard multi-return ipairs setup without
// routing through the generic GoFunction adapter.

func (vm *VM) ExecuteStdIPairsCall(absSlot, nArgs, rawC int) (bool, error) {
	if nArgs == 0 || absSlot+1 >= len(vm.regs) || !vm.regs[absSlot+1].IsTable() {
		return true, fmt.Errorf("bad argument #1 to 'ipairs' (table expected)")
	}
	if err := vm.recordFastNativeCall(vm.regs[absSlot].GoFunction()); err != nil {
		return true, err
	}
	vm.storeStdSelectResults(absSlot, rawC, []runtime.Value{
		runtime.FunctionValue(vm.ipairsIteratorFn),
		vm.regs[absSlot+1],
		runtime.IntValue(0),
	})
	return true, nil
}

// ExecuteStdPairsCall handles the ordinary-table standard pairs setup. Tables
// with a __pairs metamethod deliberately fall back to the GoFunction body so
// the existing callback/yield diagnostics stay centralized.

func (vm *VM) ExecuteStdPairsCall(absSlot, nArgs, rawC int) (bool, error) {
	if nArgs == 0 || absSlot+1 >= len(vm.regs) || !vm.regs[absSlot+1].IsTable() {
		return true, fmt.Errorf("bad argument #1 to 'pairs' (table expected)")
	}
	tbl := vm.regs[absSlot+1].Table()
	if mt := tbl.GetMetatable(); mt != nil && !mt.RawGetString("__pairs").IsNil() {
		return false, nil
	}
	if err := vm.recordFastNativeCall(vm.regs[absSlot].GoFunction()); err != nil {
		return true, err
	}
	vm.storeStdSelectResults(absSlot, rawC, []runtime.Value{
		runtime.FunctionValue(vm.newPairsIteratorFunction(tbl)),
		vm.regs[absSlot+1],
		runtime.NilValue(),
	})
	return true, nil
}

// ExecuteStdStringFindCall handles string.find when the first two results can
// be produced without allocating the generic native result slice.

func (vm *VM) ExecuteStdStringFindCall(absSlot, nArgs, rawC int) (bool, error) {
	if (nArgs != 2 && nArgs != 3 && nArgs != 4) || absSlot+nArgs >= len(vm.regs) {
		return false, nil
	}
	initv := runtime.NilValue()
	plainv := runtime.NilValue()
	if nArgs >= 3 {
		initv = vm.regs[absSlot+3]
	}
	if nArgs >= 4 {
		plainv = vm.regs[absSlot+4]
	}
	r0, r1, n, handled, err := runtime.FastStringFindRet2(vm.regs[absSlot+1], vm.regs[absSlot+2], initv, plainv, nArgs, rawC)
	if err != nil || !handled {
		return handled, err
	}
	if err := vm.recordFastNativeCall(vm.regs[absSlot].GoFunction()); err != nil {
		return true, err
	}
	vm.storeFixedFastCallResult(absSlot, rawC, r0, r1, n)
	return true, nil
}

// ExecuteStdStringMatchCall handles string.match when the visible result range
// fits the fixed two-value native fast protocol.

func (vm *VM) ExecuteStdStringMatchCall(absSlot, nArgs, rawC int) (bool, error) {
	if (nArgs != 2 && nArgs != 3) || absSlot+nArgs >= len(vm.regs) {
		return false, nil
	}
	initv := runtime.NilValue()
	if nArgs >= 3 {
		initv = vm.regs[absSlot+3]
	}
	r0, r1, n, handled, err := runtime.FastStringMatchRet2(vm.regs[absSlot+1], vm.regs[absSlot+2], initv, nArgs, rawC)
	if err != nil || !handled {
		return handled, err
	}
	if err := vm.recordFastNativeCall(vm.regs[absSlot].GoFunction()); err != nil {
		return true, err
	}
	vm.storeFixedFastCallResult(absSlot, rawC, r0, r1, n)
	return true, nil
}

// ExecuteStdRawGetCall handles rawget(table, key) without allocating the
// generic native result slice.

func (vm *VM) ExecuteStdRawGetCall(absSlot, nArgs, rawC int) (bool, error) {
	if nArgs != 2 || absSlot+2 >= len(vm.regs) {
		return false, nil
	}
	table := vm.regs[absSlot+1]
	if !table.IsTable() {
		return true, fmt.Errorf("bad argument #1 to 'rawget' (table expected, got %s)", table.TypeName())
	}
	if err := vm.recordFastNativeCall(vm.regs[absSlot].GoFunction()); err != nil {
		return true, err
	}
	vm.writeSingleCallResult(absSlot, rawC, table.Table().RawGet(vm.regs[absSlot+2]))
	return true, nil
}

// ExecuteStdNextCall handles next(table, key) without allocating the generic
// native argument/result slice.

func (vm *VM) ExecuteStdNextCall(absSlot, nArgs, rawC int) (bool, error) {
	if (nArgs != 1 && nArgs != 2) || absSlot+nArgs >= len(vm.regs) {
		return false, nil
	}
	table := vm.regs[absSlot+1]
	if !table.IsTable() {
		return true, fmt.Errorf("bad argument #1 to 'next' (table expected)")
	}
	key := runtime.NilValue()
	if nArgs == 2 {
		key = vm.regs[absSlot+2]
	}
	tbl := table.Table()
	nk, nv, ok := tbl.Next(key)
	if err := vm.recordFastNativeCall(vm.regs[absSlot].GoFunction()); err != nil {
		return true, err
	}
	if !ok {
		if !key.IsNil() && tbl.RawGet(key).IsNil() {
			return true, fmt.Errorf("invalid key to 'next'")
		}
		vm.storeFixedFastCallResult(absSlot, rawC, runtime.NilValue(), runtime.NilValue(), 1)
		return true, nil
	}
	vm.storeFixedFastCallResult(absSlot, rawC, nk, nv, 2)
	return true, nil
}

// ExecuteStdRawSetCall handles rawset(table, key, value) without allocating a
// generic native argument/result slice.

func (vm *VM) ExecuteStdRawSetCall(absSlot, nArgs, rawC int) (bool, error) {
	if nArgs < 3 || absSlot+3 >= len(vm.regs) {
		return false, nil
	}
	table := vm.regs[absSlot+1]
	key := vm.regs[absSlot+2]
	if !table.IsTable() {
		return true, fmt.Errorf("bad argument #1 to 'rawset' (table expected, got %s)", table.TypeName())
	}
	if key.IsNil() {
		return true, fmt.Errorf("table index is nil")
	}
	if key.IsFloat() && math.IsNaN(key.Float()) {
		return true, fmt.Errorf("table index is NaN")
	}
	if err := vm.recordFastNativeCall(vm.regs[absSlot].GoFunction()); err != nil {
		return true, err
	}
	table.Table().RawSet(key, vm.regs[absSlot+3])
	vm.writeSingleCallResult(absSlot, rawC, table)
	return true, nil
}

// ExecuteStdRawLenCall handles rawlen(value) without allocating a generic
// native argument/result slice.

func (vm *VM) ExecuteStdRawLenCall(absSlot, nArgs, rawC int) (bool, error) {
	if nArgs < 1 || absSlot+1 >= len(vm.regs) {
		return false, nil
	}
	arg := vm.regs[absSlot+1]
	var result runtime.Value
	switch arg.Type() {
	case runtime.TypeString:
		result = runtime.IntValue(int64(runtime.StringLen(arg)))
	case runtime.TypeTable:
		result = runtime.IntValue(int64(arg.Table().Length()))
	case runtime.TypeChannel:
		result = runtime.IntValue(int64(arg.Channel().Len()))
	default:
		return true, fmt.Errorf("bad argument to 'rawlen' (table, string, or channel expected, got %s)", arg.TypeName())
	}
	if err := vm.recordFastNativeCall(vm.regs[absSlot].GoFunction()); err != nil {
		return true, err
	}
	vm.writeSingleCallResult(absSlot, rawC, result)
	return true, nil
}

// ExecuteStdTypeCall handles type(value) without allocating a fresh string box.

func (vm *VM) ExecuteStdTypeCall(absSlot, nArgs, rawC int) (bool, error) {
	if nArgs != 1 || absSlot+1 >= len(vm.regs) {
		return false, nil
	}
	if err := vm.recordFastNativeCall(vm.regs[absSlot].GoFunction()); err != nil {
		return true, err
	}
	vm.writeSingleCallResult(absSlot, rawC, vm.typeNameValue(vm.regs[absSlot+1]))
	return true, nil
}

// ExecuteStdGetMetatableCall handles getmetatable(value) without allocating a
// generic native argument/result slice.

func (vm *VM) ExecuteStdGetMetatableCall(absSlot, nArgs, rawC int) (bool, error) {
	if nArgs != 1 || absSlot+1 >= len(vm.regs) {
		return false, nil
	}
	arg := vm.regs[absSlot+1]
	result := runtime.NilValue()
	if arg.IsTable() {
		if mt := arg.Table().GetMetatable(); mt != nil {
			if protected := mt.RawGetString("__metatable"); !protected.IsNil() {
				result = protected
			} else {
				result = runtime.TableValue(mt)
			}
		}
	}
	if err := vm.recordFastNativeCall(vm.regs[absSlot].GoFunction()); err != nil {
		return true, err
	}
	vm.writeSingleCallResult(absSlot, rawC, result)
	return true, nil
}
