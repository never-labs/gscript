//go:build darwin && arm64

// tier1_handlers_call_helpers.go contains the supporting helpers for the Tier 1
// baseline JIT OP_CALL exit handler: the const-leaf mod-add fusion, Tier 2 call
// dispatch, std.select handling, leaf-coroutine create/resume fusion, and the
// call-result store helpers.
// Pure code movement from tier1_handlers.go; no behavior change.

package methodjit

import (
	"fmt"

	"github.com/never-labs/gscript/internal/runtime"
	"github.com/never-labs/gscript/internal/vm"
)

func (e *BaselineJITEngine) tryModAddGlobalConstLeafCall(fnVal runtime.Value, regs []runtime.Value, absSlot, nArgs, rawC int) bool {
	if e == nil || e.callVM == nil || nArgs != 2 || !fnVal.IsFunction() || absSlot+2 >= len(regs) {
		return false
	}
	cl, ok := vmClosureFromValue(fnVal)
	if !ok || cl == nil || cl.Proto == nil || !isModAddGlobalConstLeaf(cl.Proto) {
		return false
	}
	a := regs[absSlot+1]
	b := regs[absSlot+2]
	if !a.IsInt() || !b.IsInt() {
		return false
	}
	modName := cl.Proto.Constants[0]
	if !modName.IsString() {
		return false
	}
	mod, ok := e.modAddGlobal(cl.Proto, modName.Str())
	if !ok {
		return false
	}
	if !mod.IsInt() || mod.Int() == 0 {
		return false
	}
	result := runtime.IntValue((a.Int() + b.Int()) % mod.Int())
	e.storeSingleCallResult(absSlot, rawC, result)
	return true
}

func (e *BaselineJITEngine) modAddGlobal(proto *vm.FuncProto, name string) (runtime.Value, bool) {
	if idx, ok := e.modAddGlobalIdx[proto]; ok {
		if v, ok := e.callVM.GetGlobalByIndex(idx); ok {
			return v, true
		}
	}
	if idx, ok := e.callVM.GlobalIndex(name); ok {
		e.modAddGlobalIdx[proto] = idx
		if v, ok := e.callVM.GetGlobalByIndex(idx); ok {
			return v, true
		}
	}
	return e.callVM.GetGlobal(name), true
}

func isModAddGlobalConstLeaf(proto *vm.FuncProto) bool {
	if proto == nil || proto.NumParams != 2 || len(proto.Code) != 4 || len(proto.Constants) != 1 {
		return false
	}
	return vm.DecodeOp(proto.Code[0]) == vm.OP_ADD &&
		vm.DecodeA(proto.Code[0]) == 3 &&
		vm.DecodeB(proto.Code[0]) == 0 &&
		vm.DecodeC(proto.Code[0]) == 1 &&
		vm.DecodeOp(proto.Code[1]) == vm.OP_GETGLOBAL &&
		vm.DecodeA(proto.Code[1]) == 4 &&
		vm.DecodeBx(proto.Code[1]) == 0 &&
		vm.DecodeOp(proto.Code[2]) == vm.OP_MOD &&
		vm.DecodeA(proto.Code[2]) == 2 &&
		vm.DecodeB(proto.Code[2]) == 3 &&
		vm.DecodeC(proto.Code[2]) == 4 &&
		vm.DecodeOp(proto.Code[3]) == vm.OP_RETURN &&
		vm.DecodeA(proto.Code[3]) == 2 &&
		vm.DecodeB(proto.Code[3]) == 2
}

func (e *BaselineJITEngine) executeCompiledTier2Call(compiled interface{}, cl *vm.Closure, regs []runtime.Value, base int, callerProto *vm.FuncProto, absSlot, nArgs, rawC int) error {
	if e == nil || e.callVM == nil || e.compiledTier2Executor == nil || cl == nil || cl.Proto == nil {
		return nil
	}
	calleeProto := cl.Proto
	calleeBase := base
	if callerProto != nil {
		calleeBase += callerProto.MaxStack
	}
	top := e.callVM.Top()
	if top > calleeBase {
		calleeBase = top
	}
	needed := calleeBase + calleeProto.MaxStack + 1
	currentRegs := e.callVM.EnsureRegs(needed)
	srcStart := absSlot + 1
	for i := 0; i < calleeProto.NumParams && i < nArgs; i++ {
		currentRegs[calleeBase+i] = currentRegs[srcStart+i]
	}
	for i := nArgs; i < calleeProto.NumParams; i++ {
		currentRegs[calleeBase+i] = runtime.NilValue()
	}
	if !e.callVM.PushFrame(cl, calleeBase) {
		return nil
	}
	results, err := e.compiledTier2Executor(compiled, currentRegs, calleeBase, calleeProto)
	if vm.IsCoroutineYield(err) {
		return err
	}
	e.callVM.CloseUpvalues(calleeBase)
	e.callVM.PopFrame()
	if err != nil {
		return err
	}
	currentRegs = e.callVM.Regs()
	if rawC == 0 {
		for i, r := range results {
			idx := absSlot + i
			if idx < len(currentRegs) {
				currentRegs[idx] = r
			}
		}
		e.callVM.SetTop(absSlot + len(results))
		return nil
	}
	nr := rawC - 1
	for i := 0; i < nr; i++ {
		idx := absSlot + i
		if idx < len(currentRegs) {
			if i < len(results) {
				currentRegs[idx] = results[i]
			} else {
				currentRegs[idx] = runtime.NilValue()
			}
		}
	}
	return nil
}

func (e *BaselineJITEngine) executeStdSelectCall(regs []runtime.Value, absSlot, nArgs, rawC int, gf *runtime.GoFunction) error {
	if nArgs == 0 || absSlot+1 >= len(regs) {
		return fmt.Errorf("bad argument #1 to 'select'")
	}
	start, countOnly, err := runtime.SelectReturnRange(regs[absSlot+1], nArgs)
	if err != nil {
		return err
	}
	runtime.RecordRuntimePathNativeCallFastFor(gf)
	currentRegs := regs
	if e != nil && e.callVM != nil {
		currentRegs = e.callVM.Regs()
	}
	if countOnly {
		storeStdSelectOne(e, currentRegs, absSlot, rawC, runtime.IntValue(int64(start)))
		return nil
	}
	valueStart := absSlot + 1 + start
	valueEnd := absSlot + 1 + nArgs
	if start >= nArgs || valueStart >= len(currentRegs) {
		storeStdSelectResults(e, currentRegs, absSlot, rawC, nil)
		return nil
	}
	if valueEnd > len(currentRegs) {
		valueEnd = len(currentRegs)
	}
	storeStdSelectRange(e, currentRegs, absSlot, rawC, valueStart, valueEnd)
	return nil
}

func storeStdSelectOne(e *BaselineJITEngine, regs []runtime.Value, absSlot, rawC int, value runtime.Value) {
	if rawC == 0 {
		if absSlot < len(regs) {
			regs[absSlot] = value
		}
		if e != nil && e.callVM != nil {
			e.callVM.SetTop(absSlot + 1)
		}
		return
	}
	nr := rawC - 1
	if nr <= 0 {
		return
	}
	if absSlot < len(regs) {
		regs[absSlot] = value
	}
	for i := 1; i < nr; i++ {
		if idx := absSlot + i; idx < len(regs) {
			regs[idx] = runtime.NilValue()
		}
	}
}

func storeStdSelectRange(e *BaselineJITEngine, regs []runtime.Value, absSlot, rawC, valueStart, valueEnd int) {
	if valueEnd < valueStart {
		valueEnd = valueStart
	}
	if rawC == 0 {
		n := valueEnd - valueStart
		for i := 0; i < n; i++ {
			dst := absSlot + i
			src := valueStart + i
			if dst < len(regs) && src < len(regs) {
				regs[dst] = regs[src]
			}
		}
		if e != nil && e.callVM != nil {
			e.callVM.SetTop(absSlot + n)
		}
		return
	}
	nr := rawC - 1
	for i := 0; i < nr; i++ {
		dst := absSlot + i
		if dst >= len(regs) {
			continue
		}
		src := valueStart + i
		if src < valueEnd && src < len(regs) {
			regs[dst] = regs[src]
		} else {
			regs[dst] = runtime.NilValue()
		}
	}
}

func storeStdSelectResults(e *BaselineJITEngine, regs []runtime.Value, absSlot, rawC int, results []runtime.Value) {
	if rawC == 0 {
		for i, r := range results {
			if idx := absSlot + i; idx < len(regs) {
				regs[idx] = r
			}
		}
		if e != nil && e.callVM != nil {
			e.callVM.SetTop(absSlot + len(results))
		}
		return
	}
	nr := rawC - 1
	for i := 0; i < nr; i++ {
		idx := absSlot + i
		if idx >= len(regs) {
			continue
		}
		if i < len(results) {
			regs[idx] = results[i]
		} else {
			regs[idx] = runtime.NilValue()
		}
	}
}

func (e *BaselineJITEngine) tryFuseCreateResumeLeafCoroutine(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto, fnVal runtime.Value, absSlot, nArgs, rawC int) (bool, error) {
	if e == nil || e.callVM == nil || ctx == nil || proto == nil || rawC != 2 || nArgs != 1 || !fnVal.IsFunction() {
		return false, nil
	}
	gf := fnVal.GoFunction()
	if gf == nil || gf.Name != "coroutine.create" {
		return false, nil
	}
	argSlot := absSlot + 1
	if argSlot < 0 || argSlot >= len(regs) {
		return false, nil
	}
	cl, ok := vmClosureFromValue(regs[argSlot])
	if !ok || cl == nil || cl.Proto == nil || !cl.Proto.LeafNoCall {
		return false, nil
	}
	callPC := int(ctx.BaselinePC) - 1
	if callPC < 0 || callPC+1 >= len(proto.Code) {
		return false, nil
	}
	resumePC := callPC + 1
	resumeOperandReg := absSlot - base
	next := proto.Code[resumePC]
	if vm.DecodeOp(next) == vm.OP_MOVE {
		if vm.DecodeB(next) != resumeOperandReg || resumePC+1 >= len(proto.Code) {
			return false, nil
		}
		resumeOperandReg = vm.DecodeA(next)
		resumePC++
		next = proto.Code[resumePC]
	}
	if vm.DecodeOp(next) != vm.OP_RESUME || vm.DecodeB(next) != 2 {
		return false, nil
	}
	resumeA := vm.DecodeA(next)
	if resumeA+1 != resumeOperandReg {
		return false, nil
	}
	handled, err := e.callVM.TryResumeLeafClosureToSlots(cl, nil, base+resumeA, vm.DecodeC(next))
	if !handled || err != nil {
		return handled, err
	}
	ctx.BaselinePC = int64(resumePC + 1)
	return true, nil
}

func (e *BaselineJITEngine) storeSingleCallResult(absSlot, rawC int, result runtime.Value) {
	currentRegs := e.callVM.Regs()
	if rawC == 0 {
		if absSlot < len(currentRegs) {
			currentRegs[absSlot] = result
		}
		e.callVM.SetTop(absSlot + 1)
		return
	}
	nr := rawC - 1
	for i := 0; i < nr; i++ {
		idx := absSlot + i
		if idx >= len(currentRegs) {
			continue
		}
		if i == 0 {
			currentRegs[idx] = result
		} else {
			currentRegs[idx] = runtime.NilValue()
		}
	}
}

func (e *BaselineJITEngine) storeCallResult2(absSlot, rawC int, r0, r1 runtime.Value, n int) {
	currentRegs := e.callVM.Regs()
	if n < 0 {
		n = 0
	} else if n > 2 {
		n = 2
	}
	if rawC == 0 {
		if n > 0 && absSlot < len(currentRegs) {
			currentRegs[absSlot] = r0
		}
		if n > 1 && absSlot+1 < len(currentRegs) {
			currentRegs[absSlot+1] = r1
		}
		e.callVM.SetTop(absSlot + n)
		return
	}
	nr := rawC - 1
	for i := 0; i < nr; i++ {
		idx := absSlot + i
		if idx >= len(currentRegs) {
			continue
		}
		switch {
		case i == 0 && n > 0:
			currentRegs[idx] = r0
		case i == 1 && n > 1:
			currentRegs[idx] = r1
		default:
			currentRegs[idx] = runtime.NilValue()
		}
	}
}
