//go:build darwin && arm64

// tier1_handlers_call.go contains the Tier 1 baseline JIT exit handler for
// OP_CALL and its supporting fast paths: GScript-closure direct dispatch,
// native-function fast-arg paths, std-library call fusions, leaf-coroutine
// fusion, and the call-result store helpers.
// Pure code movement from tier1_handlers.go; no behavior change.

package methodjit

import (
	"fmt"

	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

// handleCall handles OP_CALL exit: execute the function call via the VM.
// BaselineB and BaselineC are the raw B and C fields from the instruction:
//
//	B=0: variable args (use vm.top), else nArgs=B-1
//	C=0: return all values, C=1: no results, else nRets=C-1
func (e *BaselineJITEngine) handleCall(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	if e.callVM == nil {
		return fmt.Errorf("no callVM for call-exit")
	}
	callSlot := int(ctx.BaselineA)
	rawB := int(ctx.BaselineB)
	rawC := int(ctx.BaselineC)

	absSlot := base + callSlot
	if absSlot >= len(regs) {
		return fmt.Errorf("call slot %d out of range", absSlot)
	}
	fnVal := regs[absSlot]

	// Determine number of arguments.
	var nArgs int
	if rawB == 0 {
		top := e.callVM.Top()
		nArgs = top - (absSlot + 1)
		if nArgs < 0 {
			nArgs = 0
		}
	} else {
		nArgs = rawB - 1
	}

	if handled, err := e.tryFuseCreateResumeLeafCoroutine(ctx, regs, base, proto, fnVal, absSlot, nArgs, rawC); handled {
		return err
	}

	if proto != nil && proto.CallSiteFeedback != nil {
		// BaselinePC is the resume PC (the instruction after OP_CALL).
		// Callsite feedback is indexed by the current bytecode PC.
		pc := int(ctx.BaselinePC) - 1
		if pc >= 0 && pc < len(proto.CallSiteFeedback) {
			argStart := absSlot + 1
			argEnd := argStart + nArgs
			if argStart >= 0 && argEnd >= argStart && argEnd <= len(regs) {
				proto.CallSiteFeedback[pc].ObserveCall(fnVal, regs[argStart:argEnd], nArgs, rawC)
			} else {
				proto.CallSiteFeedback[pc].ObserveCall(fnVal, nil, nArgs, rawC)
			}
		}
	}

	if handled, err := e.callVM.TryFastCoroutineCallValue(fnVal, absSlot, nArgs, rawC); handled {
		return err
	}

	if handled := e.tryModAddGlobalConstLeafCall(fnVal, regs, absSlot, nArgs, rawC); handled {
		return nil
	}

	if vmCallSiteRuntimeSpecializationArity(nArgs) && fnVal.IsFunction() {
		argsStart := absSlot + 1
		argsEnd := argsStart + nArgs
		if argsStart >= 0 && argsEnd >= argsStart && argsEnd <= len(regs) {
			if handled, err := e.callVM.TryRunNoResultCallSiteRuntimeSpecializationForJIT(fnVal, regs[argsStart:argsEnd]); handled {
				if err != nil {
					return err
				}
				if rawC == 0 {
					e.callVM.SetTop(absSlot)
				} else {
					currentRegs := e.callVM.Regs()
					for i := 0; i < rawC-1; i++ {
						idx := absSlot + i
						if idx < len(currentRegs) {
							currentRegs[idx] = runtime.NilValue()
						}
					}
				}
				return nil
			}
		}
	}
	if rawB != 0 && rawC > 1 && e.protocolCallExecutor != nil {
		if handled, err := e.protocolCallExecutor(fnVal, regs, absSlot, nArgs, rawC-1); handled {
			return err
		}
	}

	// Fast path: GScript closure with compiled proto. Avoids heap-allocating
	// callArgs and bypasses CallValue → callValue → call dispatch.
	if fnVal.IsFunction() {
		if cl, ok := vmClosureFromValue(fnVal); ok && cl.Proto.MethodJITCallable() {
			calleeProto := cl.Proto
			if calleeProto.JITDisabled {
				goto slowPath
			}
			if calleeProto.Tier2Promoted {
				goto slowPath
			}
			// If the TieringManager has set a tier-up threshold and the callee
			// has EXACTLY reached it, fall to slow path ONCE so the VM's
			// TryCompile can trigger Tier 2 compilation. Using == instead of
			// >= ensures this detour happens only once per function.
			if e.tierUpThreshold > 0 && calleeProto.CallCount == e.tierUpThreshold {
				goto slowPath
			}
			// If Tier 2 applied an intrinsic (e.g., math.sqrt→FSQRT), Tier 1
			// code would execute a different (slower, allocating) sequence.
			// Dispatch via slowPath so Tier 2's compiled code runs.
			// For functions without intrinsics, Tier 1 execution is equivalent
			// and faster than going through VM.CallValue.
			if calleeProto.NeedsTier2 {
				goto slowPath
			}
			calleeBF, compiled := e.compiled[calleeProto]
			if !compiled {
				// Try to compile the callee on the fly.
				calleeProto.CallCount++
				if calleeProto.CallCount <= 64 {
					argsStart := absSlot + 1
					argsEnd := argsStart + nArgs
					if argsStart >= 0 && argsEnd >= argsStart && argsEnd <= len(regs) {
						calleeProto.ObserveArgShapes(regs[argsStart:argsEnd])
						calleeProto.ObserveArgArrayElementShapes(regs[argsStart:argsEnd])
					}
				}
				var compileResult interface{}
				if e.outerCompiler != nil {
					compileResult = e.outerCompiler(calleeProto)
				} else {
					compileResult = e.TryCompile(calleeProto)
				}
				// If the result is a *BaselineFunc, use it for fast-path execution.
				// If it's a *CompiledFunction (Tier 2), fall to slow path
				// so Execute dispatches to executeTier2.
				if bf, ok := compileResult.(*BaselineFunc); ok {
					calleeBF = bf
					compiled = true
				} else if compileResult != nil {
					// Tier 2 compiled — fall to slow path for proper dispatch.
					goto slowPath
				}
			}
			if compiled {
				// Compute callee base: after caller's register window.
				calleeBase := base + proto.MaxStack
				top := e.callVM.Top()
				if top > calleeBase {
					calleeBase = top
				}

				// Ensure register space (may grow the register file).
				needed := calleeBase + calleeProto.MaxStack + 1
				currentRegs := e.callVM.EnsureRegs(needed)

				// Copy args directly — no heap allocation.
				nParams := calleeProto.NumParams
				srcStart := absSlot + 1
				for i := 0; i < nParams && i < nArgs; i++ {
					currentRegs[calleeBase+i] = currentRegs[srcStart+i]
				}
				for i := nArgs; i < nParams; i++ {
					currentRegs[calleeBase+i] = runtime.NilValue()
				}

				var varargs []runtime.Value
				if calleeProto.IsVarArg && nArgs > nParams {
					varargs = currentRegs[srcStart+nParams : srcStart+nArgs]
				}

				// Push a VM frame so CurrentClosure()/CurrentVarargs() return
				// the callee state and CloseUpvalues works correctly on return.
				if !e.callVM.PushFrameWithBorrowedVarargs(cl, calleeBase, varargs) {
					// Stack overflow — fall through to generic path.
					goto slowPath
				}

				// Execute the callee directly via JIT.
				results, err := e.Execute(calleeBF, currentRegs, calleeBase, calleeProto)
				if err == errOSRRequested && e.osrHandler != nil {
					currentRegs = e.callVM.Regs()
					results, err = e.osrHandler(currentRegs, calleeBase, calleeProto)
				}
				if vm.IsCoroutineYield(err) {
					return err
				}

				// Close upvalues and pop frame regardless of error.
				e.callVM.CloseUpvalues(calleeBase)
				e.callVM.PopFrame()

				if err != nil {
					return err
				}

				// Re-read regs (Execute may have grown the register file).
				currentRegs = e.callVM.Regs()

				// Place results starting at the function slot.
				if rawC == 0 {
					for i, r := range results {
						idx := absSlot + i
						if idx < len(currentRegs) {
							currentRegs[idx] = r
						}
					}
					e.callVM.SetTop(absSlot + len(results))
				} else {
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
				}
				return nil
			}
		}
	}
slowPath:
	if gf := fnVal.GoFunction(); gf != nil {
		switch gf.NativeKind {
		case runtime.NativeKindStdSelect:
			if gf.NativeData == runtime.StdSelectIdentityPtr() {
				return e.executeStdSelectCall(regs, absSlot, nArgs, rawC, gf)
			}
		case runtime.NativeKindStdIPairs:
			if e != nil && e.callVM != nil && gf.NativeData == runtime.StdIPairsIdentityPtr() {
				handled, err := e.callVM.ExecuteStdIPairsCall(absSlot, nArgs, rawC)
				if err != nil || handled {
					return err
				}
			}
		case runtime.NativeKindStdPairs:
			if e != nil && e.callVM != nil && gf.NativeData == runtime.StdPairsIdentityPtr() {
				handled, err := e.callVM.ExecuteStdPairsCall(absSlot, nArgs, rawC)
				if err != nil || handled {
					return err
				}
			}
		case runtime.NativeKindStdStringFind:
			if e != nil && e.callVM != nil && gf.NativeData == runtime.StdStringFindIdentityPtr() {
				handled, err := e.callVM.ExecuteStdStringFindCall(absSlot, nArgs, rawC)
				if err != nil || handled {
					return err
				}
			}
		case runtime.NativeKindStdStringMatch:
			if e != nil && e.callVM != nil && gf.NativeData == runtime.StdStringMatchIdentityPtr() {
				handled, err := e.callVM.ExecuteStdStringMatchCall(absSlot, nArgs, rawC)
				if err != nil || handled {
					return err
				}
			}
		case runtime.NativeKindStdRawGet:
			if e != nil && e.callVM != nil && gf.NativeData == runtime.StdRawGetIdentityPtr() {
				handled, err := e.callVM.ExecuteStdRawGetCall(absSlot, nArgs, rawC)
				if err != nil || handled {
					return err
				}
			}
		case runtime.NativeKindStdNext:
			if e != nil && e.callVM != nil && gf.NativeData == runtime.StdNextIdentityPtr() {
				handled, err := e.callVM.ExecuteStdNextCall(absSlot, nArgs, rawC)
				if err != nil || handled {
					return err
				}
			}
		case runtime.NativeKindStdRawSet:
			if e != nil && e.callVM != nil && gf.NativeData == runtime.StdRawSetIdentityPtr() {
				handled, err := e.callVM.ExecuteStdRawSetCall(absSlot, nArgs, rawC)
				if err != nil || handled {
					return err
				}
			}
		case runtime.NativeKindStdRawLen:
			if e != nil && e.callVM != nil && gf.NativeData == runtime.StdRawLenIdentityPtr() {
				handled, err := e.callVM.ExecuteStdRawLenCall(absSlot, nArgs, rawC)
				if err != nil || handled {
					return err
				}
			}
		case runtime.NativeKindStdType:
			if e != nil && e.callVM != nil && gf.NativeData == runtime.StdTypeIdentityPtr() {
				handled, err := e.callVM.ExecuteStdTypeCall(absSlot, nArgs, rawC)
				if err != nil || handled {
					return err
				}
			}
		case runtime.NativeKindStdGetMetatable:
			if e != nil && e.callVM != nil && gf.NativeData == runtime.StdGetMetatableIdentityPtr() {
				handled, err := e.callVM.ExecuteStdGetMetatableCall(absSlot, nArgs, rawC)
				if err != nil || handled {
					return err
				}
			}
		}
		if nArgs == 1 && gf.FastArg1Ret2 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			idx := absSlot + 1
			arg := runtime.NilValue()
			if idx < len(regs) {
				arg = regs[idx]
			}
			r0, r1, n, err := gf.FastArg1Ret2(arg)
			if err != nil {
				return err
			}
			e.storeCallResult2(absSlot, rawC, r0, r1, n)
			return nil
		}
		if nArgs == 2 && gf.FastArg2Ret2 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			idx0 := absSlot + 1
			idx1 := absSlot + 2
			arg0 := runtime.NilValue()
			arg1 := runtime.NilValue()
			if idx0 < len(regs) {
				arg0 = regs[idx0]
			}
			if idx1 < len(regs) {
				arg1 = regs[idx1]
			}
			r0, r1, n, err := gf.FastArg2Ret2(arg0, arg1)
			if err != nil {
				return err
			}
			e.storeCallResult2(absSlot, rawC, r0, r1, n)
			return nil
		}
		if nArgs == 1 && gf.FastArg1 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			idx := absSlot + 1
			arg := runtime.NilValue()
			if idx < len(regs) {
				arg = regs[idx]
			}
			result, err := gf.FastArg1(arg)
			if err != nil {
				return err
			}
			e.storeSingleCallResult(absSlot, rawC, result)
			return nil
		}
		if nArgs == 2 && gf.FastArg2 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			idx0 := absSlot + 1
			idx1 := absSlot + 2
			arg0 := runtime.NilValue()
			arg1 := runtime.NilValue()
			if idx0 < len(regs) {
				arg0 = regs[idx0]
			}
			if idx1 < len(regs) {
				arg1 = regs[idx1]
			}
			result, err := gf.FastArg2(arg0, arg1)
			if err != nil {
				return err
			}
			e.storeSingleCallResult(absSlot, rawC, result)
			return nil
		}
		if nArgs == 3 && gf.FastArg3 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			idx0 := absSlot + 1
			idx1 := absSlot + 2
			idx2 := absSlot + 3
			arg0 := runtime.NilValue()
			arg1 := runtime.NilValue()
			arg2 := runtime.NilValue()
			if idx0 < len(regs) {
				arg0 = regs[idx0]
			}
			if idx1 < len(regs) {
				arg1 = regs[idx1]
			}
			if idx2 < len(regs) {
				arg2 = regs[idx2]
			}
			result, err := gf.FastArg3(arg0, arg1, arg2)
			if err != nil {
				return err
			}
			e.storeSingleCallResult(absSlot, rawC, result)
			return nil
		}
		if nArgs == 4 && gf.FastArg4 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			idx0 := absSlot + 1
			idx1 := absSlot + 2
			idx2 := absSlot + 3
			idx3 := absSlot + 4
			arg0 := runtime.NilValue()
			arg1 := runtime.NilValue()
			arg2 := runtime.NilValue()
			arg3 := runtime.NilValue()
			if idx0 < len(regs) {
				arg0 = regs[idx0]
			}
			if idx1 < len(regs) {
				arg1 = regs[idx1]
			}
			if idx2 < len(regs) {
				arg2 = regs[idx2]
			}
			if idx3 < len(regs) {
				arg3 = regs[idx3]
			}
			result, err := gf.FastArg4(arg0, arg1, arg2, arg3)
			if err != nil {
				return err
			}
			e.storeSingleCallResult(absSlot, rawC, result)
			return nil
		}
		if nArgs == 5 && gf.FastArg5 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			idx0 := absSlot + 1
			idx1 := absSlot + 2
			idx2 := absSlot + 3
			idx3 := absSlot + 4
			idx4 := absSlot + 5
			arg0 := runtime.NilValue()
			arg1 := runtime.NilValue()
			arg2 := runtime.NilValue()
			arg3 := runtime.NilValue()
			arg4 := runtime.NilValue()
			if idx0 < len(regs) {
				arg0 = regs[idx0]
			}
			if idx1 < len(regs) {
				arg1 = regs[idx1]
			}
			if idx2 < len(regs) {
				arg2 = regs[idx2]
			}
			if idx3 < len(regs) {
				arg3 = regs[idx3]
			}
			if idx4 < len(regs) {
				arg4 = regs[idx4]
			}
			result, err := gf.FastArg5(arg0, arg1, arg2, arg3, arg4)
			if err != nil {
				return err
			}
			e.storeSingleCallResult(absSlot, rawC, result)
			return nil
		}
		if nArgs == 6 && gf.FastArg6 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			var args [6]runtime.Value
			for i := range args {
				idx := absSlot + 1 + i
				if idx < len(regs) {
					args[i] = regs[idx]
				} else {
					args[i] = runtime.NilValue()
				}
			}
			result, err := gf.FastArg6(args[0], args[1], args[2], args[3], args[4], args[5])
			if err != nil {
				return err
			}
			e.storeSingleCallResult(absSlot, rawC, result)
			return nil
		}
		if nArgs == 8 && gf.FastArg8 != nil {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			var args [8]runtime.Value
			for i := range args {
				idx := absSlot + 1 + i
				if idx < len(regs) {
					args[i] = regs[idx]
				} else {
					args[i] = runtime.NilValue()
				}
			}
			result, err := gf.FastArg8(args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7])
			if err != nil {
				return err
			}
			e.storeSingleCallResult(absSlot, rawC, result)
			return nil
		}
		if gf.Fast1 == nil {
			goto genericNativePath
		}
		runtime.RecordRuntimePathNativeCallFastFor(gf)
		var local [16]runtime.Value
		var callArgs []runtime.Value
		if nArgs <= len(local) {
			callArgs = local[:nArgs]
		} else {
			callArgs = make([]runtime.Value, nArgs)
		}
		for i := 0; i < nArgs; i++ {
			idx := absSlot + 1 + i
			if idx < len(regs) {
				callArgs[i] = regs[idx]
			}
		}
		result, err := gf.Fast1(callArgs)
		if err != nil {
			return err
		}
		e.storeSingleCallResult(absSlot, rawC, result)
		return nil
	}

genericNativePath:
	if gf := fnVal.GoFunction(); gf != nil {
		runtime.RecordRuntimePathNativeCallFallbackFor(gf)
	}
	// Generic path: heap-allocate args and go through CallValue.
	callArgs := make([]runtime.Value, nArgs)
	for i := 0; i < nArgs; i++ {
		idx := absSlot + 1 + i
		if idx < len(regs) {
			callArgs[i] = regs[idx]
		}
	}

	results, err := e.callVM.CallValue(fnVal, callArgs)
	if err != nil {
		return err
	}

	// Re-read regs in case the callee grew the register file.
	currentRegs := e.callVM.Regs()

	// Place results: overwrite starting from the function slot.
	if rawC == 0 {
		for i, r := range results {
			idx := absSlot + i
			if idx < len(currentRegs) {
				currentRegs[idx] = r
			}
		}
		e.callVM.SetTop(absSlot + len(results))
	} else {
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
	}
	return nil
}
