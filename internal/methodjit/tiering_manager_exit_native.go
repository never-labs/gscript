//go:build darwin && arm64

// tiering_manager_exit_native.go holds the Tier 2 native-call-exit protocol:
// suspended-callee frame snapshot/restore, the resume-context rebind, the
// nested native callee resume loop, and the native-callee deopt path.
//
// Pure code movement from tiering_manager_exit.go; no behavior change.

package methodjit

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"github.com/Never-Labs/gscript/internal/jit"
	"github.com/Never-Labs/gscript/internal/runtime"
	"github.com/Never-Labs/gscript/internal/vm"
)

// errNestedNativeCallExit is a known bridge limitation: the current
// ExitNativeCallExit descriptor represents one suspended native callee. Nested
// typed-self exits need a descriptor stack to resume fully in Tier 2, so the VM
// falls through to the interpreter. Keep this sentinel allocation-free; the
// fallback path can hit it once per recursive leaf.
var errNestedNativeCallExit = errors.New("tier2: nested native-call-exit")

func (tm *TieringManager) executeNativeCallExit(ctx *ExecContext, callerCF *CompiledFunction, regs []runtime.Value, callerBase int, callerProto *vm.FuncProto) ([]runtime.Value, error) {
	if tm.callVM == nil {
		return regs, fmt.Errorf("no callVM set for native-call-exit")
	}
	callerFrame := snapshotNativeCallExitFrame(ctx)
	callerClosurePtr := callerFrame.NativeCallerClosurePtr
	calleeProto, calleeCF, calleeBase, err := tm.nativeExitCallee(ctx, regs, callerBase, callerProto, callerCF)
	if err != nil {
		return regs, err
	}
	observeTier2NativeCalleeArgShapes(calleeProto, regs, calleeBase)
	if tm.envR154Trace {
		fmt.Fprintf(os.Stderr, "[R154] tier2 native-call execute caller=%q instr=%d source_pc=%d path=call-exit callee=%q callee_ops=%s native_exit_code=%d native_resume_pc=%d tier2_only=%d abi=resume\n",
			traceProtoName(callerProto), ctx.CallID, tier2CallExitSourcePC(callerCF, ctx),
			traceProtoName(calleeProto), protoNativeCallRiskSummary(calleeProto),
			ctx.NativeCalleeExitCode, ctx.NativeCalleeResumePC, ctx.NativeCalleeTier2Only)
	}

	if !calleeCF.DirectEntrySafe {
		tier2Entry := uintptr(0)
		if calleeCF.Tier2DirectEntrySafe && calleeCF.DirectEntryOffset > 0 {
			tier2Entry = uintptr(calleeCF.Code.Ptr()) + uintptr(calleeCF.DirectEntryOffset)
		}
		setFuncProtoTier2DirectEntries(calleeProto, 0, tier2Entry)
	}

	result, err := tm.resumeNativeTier2CalleeExit(ctx, calleeCF, regs, calleeBase, calleeProto)
	if err != nil {
		return regs, err
	}
	restoreNativeCallExitFrame(ctx, callerFrame)
	tm.setTier2ResumeContext(ctx, callerCF, callerProto, callerBase)
	if callerClosurePtr != 0 {
		ctx.BaselineClosurePtr = callerClosurePtr
	}
	regs = tm.callVM.Regs()
	absSlot := callerBase + int(ctx.CallSlot)
	nRets := int(ctx.CallNRets)
	for i := 0; i < nRets; i++ {
		idx := absSlot + i
		if idx >= 0 && idx < len(regs) {
			if i == 0 {
				regs[idx] = result
			} else {
				regs[idx] = runtime.NilValue()
			}
		}
	}
	return regs, nil
}

func snapshotNativeCallExitFrame(ctx *ExecContext) NativeCallExitFrame {
	if ctx == nil {
		return NativeCallExitFrame{}
	}
	return NativeCallExitFrame{
		CallSlot:               ctx.CallSlot,
		CallNArgs:              ctx.CallNArgs,
		CallNRets:              ctx.CallNRets,
		CallID:                 ctx.CallID,
		NativeCallA:            ctx.NativeCallA,
		NativeCallB:            ctx.NativeCallB,
		NativeCallC:            ctx.NativeCallC,
		NativeCalleeExitCode:   ctx.NativeCalleeExitCode,
		NativeCalleeResumePass: ctx.NativeCalleeResumePass,
		NativeCalleeBaseOff:    ctx.NativeCalleeBaseOff,
		NativeCalleeResumePC:   ctx.NativeCalleeResumePC,
		NativeCalleeClosurePtr: ctx.NativeCalleeClosurePtr,
		NativeCalleeTier2Only:  ctx.NativeCalleeTier2Only,
		NativeCallerClosurePtr: ctx.NativeCallerClosurePtr,
		ResumeNumericPass:      ctx.ResumeNumericPass,
	}
}

func restoreNativeCallExitFrame(ctx *ExecContext, frame NativeCallExitFrame) {
	if ctx == nil {
		return
	}
	ctx.CallSlot = frame.CallSlot
	ctx.CallNArgs = frame.CallNArgs
	ctx.CallNRets = frame.CallNRets
	ctx.CallID = frame.CallID
	ctx.NativeCallA = frame.NativeCallA
	ctx.NativeCallB = frame.NativeCallB
	ctx.NativeCallC = frame.NativeCallC
	ctx.NativeCalleeExitCode = frame.NativeCalleeExitCode
	ctx.NativeCalleeResumePass = frame.NativeCalleeResumePass
	ctx.NativeCalleeBaseOff = frame.NativeCalleeBaseOff
	ctx.NativeCalleeResumePC = frame.NativeCalleeResumePC
	ctx.NativeCalleeClosurePtr = frame.NativeCalleeClosurePtr
	ctx.NativeCalleeTier2Only = frame.NativeCalleeTier2Only
	ctx.NativeCallerClosurePtr = frame.NativeCallerClosurePtr
	ctx.ResumeNumericPass = frame.ResumeNumericPass
}

func popNativeCallExitFrame(ctx *ExecContext) bool {
	if ctx == nil || ctx.NativeCallExitStackDepth <= 0 {
		return false
	}
	ctx.NativeCallExitStackDepth--
	frame := ctx.NativeCallExitStack[ctx.NativeCallExitStackDepth]
	restoreNativeCallExitFrame(ctx, frame)
	ctx.NativeCallExitStack[ctx.NativeCallExitStackDepth] = NativeCallExitFrame{}
	return true
}

func (tm *TieringManager) setTier2ResumeContext(ctx *ExecContext, cf *CompiledFunction, proto *vm.FuncProto, base int) {
	if ctx == nil || tm.callVM == nil {
		return
	}
	regs := tm.callVM.Regs()
	if base >= 0 && base < len(regs) {
		ctx.Regs = uintptr(unsafe.Pointer(&regs[base]))
		ctx.RegsBase = uintptr(unsafe.Pointer(&regs[0]))
		ctx.RegsEnd = ctx.RegsBase + uintptr(len(regs)*jit.ValueSize)
	}
	if proto != nil && len(proto.Constants) > 0 {
		ctx.Constants = uintptr(unsafe.Pointer(&proto.Constants[0]))
	} else {
		ctx.Constants = 0
	}
	tm.setTier2FieldCacheContext(ctx, proto)
	if cf != nil && len(cf.GlobalCache) > 0 {
		ctx.Tier2GlobalCache = uintptr(unsafe.Pointer(&cf.GlobalCache[0]))
		ctx.Tier2GlobalCacheGen = uintptr(unsafe.Pointer(&cf.GlobalCacheGen))
	} else {
		ctx.Tier2GlobalCache = 0
		ctx.Tier2GlobalCacheGen = 0
	}
	ctx.Tier2GlobalGenPtr = uintptr(unsafe.Pointer(&tm.tier1.globalCacheGen))
	if proto != nil && cf != nil {
		if arrayPtr, verPtr, ver, ok := tm.prepareTier2GlobalIndexes(proto, cf); ok {
			ctx.Tier2GlobalIndex = proto.Tier2GlobalIndexPtr
			ctx.Tier2GlobalArray = arrayPtr
			ctx.Tier2GlobalVerPtr = uintptr(unsafe.Pointer(verPtr))
			ctx.Tier2GlobalVer = uint64(ver)
		} else {
			ctx.Tier2GlobalIndex = 0
			ctx.Tier2GlobalArray = 0
			ctx.Tier2GlobalVerPtr = 0
			ctx.Tier2GlobalVer = 0
		}
	}
	if cf != nil && len(cf.CallCache) > 0 {
		ctx.Tier2CallCache = uintptr(unsafe.Pointer(&cf.CallCache[0]))
	} else {
		ctx.Tier2CallCache = 0
	}
	if cl := tm.callVM.CurrentClosure(); cl != nil {
		ctx.BaselineClosurePtr = uintptr(unsafe.Pointer(cl))
	}
}

func (tm *TieringManager) nativeExitCallee(ctx *ExecContext, regs []runtime.Value, callerBase int, callerProto *vm.FuncProto, callerCF *CompiledFunction) (*vm.FuncProto, *CompiledFunction, int, error) {
	calleeBase := callerBase + int(ctx.NativeCalleeBaseOff)/jit.ValueSize
	callSlot := callerBase + int(ctx.CallSlot)
	if callSlot < 0 || callSlot >= len(regs) {
		return nil, nil, 0, fmt.Errorf("native-call-exit: call slot %d out of range", callSlot)
	}
	fnVal := regs[callSlot]
	cl, ok := vmClosureFromValue(fnVal)
	if !ok || cl == nil || cl.Proto == nil {
		return nil, nil, 0, fmt.Errorf("native-call-exit: call slot %d is not a VM closure", callSlot)
	}
	if ctx.NativeCalleeClosurePtr != 0 && uintptr(unsafe.Pointer(cl)) != ctx.NativeCalleeClosurePtr {
		return nil, nil, 0, fmt.Errorf("native-call-exit: callee closure changed")
	}
	calleeCF, ok := tm.tier2CompiledFor(cl.Proto)
	if !ok || calleeCF == nil {
		if callerProto != nil && callerCF != nil && (cl.Proto == callerProto || cl.Proto.Name == callerProto.Name) {
			return callerProto, callerCF, calleeBase, nil
		}
		return nil, nil, 0, fmt.Errorf("native-call-exit: callee %q is not compiled at Tier 2", cl.Proto.Name)
	}
	return cl.Proto, calleeCF, calleeBase, nil
}

func observeTier2NativeCalleeArgShapes(proto *vm.FuncProto, regs []runtime.Value, base int) {
	if proto == nil || proto.NumParams <= 0 || base < 0 || base >= len(regs) {
		return
	}
	end := base + proto.NumParams
	if end > len(regs) {
		end = len(regs)
	}
	if end <= base {
		return
	}
	args := regs[base:end]
	proto.ObserveArgShapes(args)
	proto.ObserveArgArrayElementShapes(args)
}

func (tm *TieringManager) resumeNativeTier2CalleeExit(ctx *ExecContext, cf *CompiledFunction, regs []runtime.Value, base int, proto *vm.FuncProto) (runtime.Value, error) {
	tm.recordTier2NativeCalleeExit(proto, cf, ctx)
	observeTier2NativeCalleeArgShapes(proto, regs, base)
	codePtr := uintptr(0)
	resumeClosurePtr := ctx.NativeCalleeClosurePtr
	midRunRefreshDeferred := false
	refreshCallee := func(resumeOff int) int {
		if midRunRefreshDeferred {
			return resumeOff
		}
		current := tm.currentTier2SpeculationProfile(proto)
		if !tm.recompile.ShouldRefreshProfileForProto(proto, cf, current) {
			return resumeOff
		}
		nextCF, nextResumeOff, switched := tm.tryMidRunTier2Refresh(proto, cf, ctx)
		if !switched {
			midRunRefreshDeferred = true
			return resumeOff
		}
		cf = nextCF
		tm.setTier2ResumeContext(ctx, cf, proto, base)
		return nextResumeOff
	}
	switch ctx.NativeCalleeExitCode {
	case ExitTableExit:
		handlerMark := tm.tier2PerfStart()
		err := tm.executeTableExit(ctx, regs, base, proto, cf)
		tm.tier2PerfStop(perfTier2TableExit, handlerMark)
		if err != nil {
			return runtime.NilValue(), fmt.Errorf("callee table-exit: %w", err)
		}
		resumeOff, ok := cf.resumeOffset(int(ctx.TableExitID), ctx.NativeCalleeResumePass != 0)
		if !ok {
			return runtime.NilValue(), fmt.Errorf("callee table-exit: no resume for %d", ctx.TableExitID)
		}
		resumeOff = refreshCallee(resumeOff)
		codePtr = uintptr(cf.Code.Ptr()) + uintptr(resumeOff)
	case ExitGlobalExit:
		if err := tm.executeGlobalExit(ctx, regs, base, proto, cf); err != nil {
			return runtime.NilValue(), fmt.Errorf("callee global-exit: %w", err)
		}
		resumeOff, ok := cf.resumeOffset(int(ctx.GlobalExitID), ctx.NativeCalleeResumePass != 0)
		if !ok {
			return runtime.NilValue(), fmt.Errorf("callee global-exit: no resume for %d", ctx.GlobalExitID)
		}
		codePtr = uintptr(cf.Code.Ptr()) + uintptr(resumeOff)
	case ExitOpExit:
		handlerMark := tm.tier2PerfStart()
		err := tm.executeOpExit(ctx, regs, base, proto)
		tm.tier2PerfStop(perfTier2OpExit, handlerMark)
		if err != nil {
			return runtime.NilValue(), fmt.Errorf("callee op-exit: %w", err)
		}
		resumeOff, ok := cf.resumeOffset(int(ctx.OpExitID), ctx.NativeCalleeResumePass != 0)
		if !ok {
			return runtime.NilValue(), fmt.Errorf("callee op-exit: no resume for %d", ctx.OpExitID)
		}
		codePtr = uintptr(cf.Code.Ptr()) + uintptr(resumeOff)
	case ExitCallExit:
		if err := tm.executeCallExit(ctx, regs, base, proto, cf); err != nil {
			if vm.IsCoroutineYield(err) {
				return runtime.NilValue(), err
			}
			return runtime.NilValue(), fmt.Errorf("callee call-exit: %w", err)
		}
		resumeOff, ok := cf.resumeOffset(int(ctx.CallID), ctx.NativeCalleeResumePass != 0)
		if !ok {
			return runtime.NilValue(), fmt.Errorf("callee call-exit: no resume for %d", ctx.CallID)
		}
		codePtr = uintptr(cf.Code.Ptr()) + uintptr(resumeOff)
	case ExitDeopt:
		action := tm.nativeCalleeDeoptAction(proto, cf, ctx, ctx.NativeCalleeResumePC)
		tm.applyTier2DeoptAction(proto, action)
		return tm.resumeNativeCalleePreciseDeopt(ctx, base, proto, ctx.NativeCalleeResumePC)
	case ExitNativeCallExit:
		if ctx.NativeCallExitStackOverflow != 0 || !popNativeCallExitFrame(ctx) {
			return runtime.NilValue(), errNestedNativeCallExit
		}
		var err error
		protocolMark := tm.tier2PerfStart()
		regs, err = tm.executeNativeCallExit(ctx, cf, regs, base, proto)
		tm.tier2PerfStop(perfTier2NativeCallExitProtocol, protocolMark)
		if err != nil {
			return runtime.NilValue(), err
		}
		resumeOff, ok := cf.resumeOffset(int(ctx.CallID), ctx.ResumeNumericPass != 0)
		if !ok {
			return runtime.NilValue(), fmt.Errorf("callee native-call-exit: no resume for %d", ctx.CallID)
		}
		codePtr = uintptr(cf.Code.Ptr()) + uintptr(resumeOff)
		resumeClosurePtr = ctx.NativeCallerClosurePtr
	default:
		return runtime.NilValue(), fmt.Errorf("unknown callee exit code %d", ctx.NativeCalleeExitCode)
	}

	currentRegs := tm.callVM.Regs()
	tm.setTier2ResumeContext(ctx, cf, proto, base)
	if resumeClosurePtr != 0 {
		ctx.BaselineClosurePtr = resumeClosurePtr
	}
	ctx.CallMode = 1
	ctx.ExitCode = 0
	ctx.ResumeNumericPass = 0

	for {
		nativeMark := tm.tier2PerfStart()
		jit.CallJIT(codePtr, uintptr(unsafe.Pointer(ctx)))
		tm.tier2PerfStop(perfTier2NativeExecution, nativeMark)
		switch ctx.ExitCode {
		case ExitNormal:
			if midRunRefreshDeferred {
				tm.retireStaleTier2AfterFeedback(proto, cf)
			}
			return runtime.Value(ctx.BaselineReturnValue), nil
		case ExitTableExit:
			handlerMark := tm.tier2PerfStart()
			err := tm.executeTableExit(ctx, currentRegs, base, proto, cf)
			tm.tier2PerfStop(perfTier2TableExit, handlerMark)
			if err != nil {
				return runtime.NilValue(), fmt.Errorf("callee table-exit: %w", err)
			}
			currentRegs = tm.callVM.Regs()
			resumeMark := tm.tier2PerfStart()
			resumeOff, ok := cf.resumeOffset(int(ctx.TableExitID), ctx.ResumeNumericPass != 0)
			if !ok {
				return runtime.NilValue(), fmt.Errorf("callee table-exit: no resume for %d", ctx.TableExitID)
			}
			resumeOff = refreshCallee(resumeOff)
			codePtr = uintptr(cf.Code.Ptr()) + uintptr(resumeOff)
			ctx.ExitCode = 0
			ctx.ResumeNumericPass = 0
			tm.tier2PerfStop(perfTier2ExitResume, resumeMark)
		case ExitGlobalExit:
			if err := tm.executeGlobalExit(ctx, currentRegs, base, proto, cf); err != nil {
				return runtime.NilValue(), fmt.Errorf("callee global-exit: %w", err)
			}
			currentRegs = tm.callVM.Regs()
			resumeMark := tm.tier2PerfStart()
			resumeOff, ok := cf.resumeOffset(int(ctx.GlobalExitID), ctx.ResumeNumericPass != 0)
			if !ok {
				return runtime.NilValue(), fmt.Errorf("callee global-exit: no resume for %d", ctx.GlobalExitID)
			}
			codePtr = uintptr(cf.Code.Ptr()) + uintptr(resumeOff)
			ctx.ExitCode = 0
			ctx.ResumeNumericPass = 0
			tm.tier2PerfStop(perfTier2ExitResume, resumeMark)
		case ExitOpExit:
			handlerMark := tm.tier2PerfStart()
			err := tm.executeOpExit(ctx, currentRegs, base, proto)
			tm.tier2PerfStop(perfTier2OpExit, handlerMark)
			if err != nil {
				return runtime.NilValue(), fmt.Errorf("callee op-exit: %w", err)
			}
			currentRegs = tm.callVM.Regs()
			resumeMark := tm.tier2PerfStart()
			resumeOff, ok := cf.resumeOffset(int(ctx.OpExitID), ctx.ResumeNumericPass != 0)
			if !ok {
				return runtime.NilValue(), fmt.Errorf("callee op-exit: no resume for %d", ctx.OpExitID)
			}
			codePtr = uintptr(cf.Code.Ptr()) + uintptr(resumeOff)
			ctx.ExitCode = 0
			ctx.ResumeNumericPass = 0
			tm.tier2PerfStop(perfTier2ExitResume, resumeMark)
		case ExitCallExit:
			if err := tm.executeCallExit(ctx, currentRegs, base, proto, cf); err != nil {
				if vm.IsCoroutineYield(err) {
					return runtime.NilValue(), err
				}
				return runtime.NilValue(), fmt.Errorf("callee call-exit: %w", err)
			}
			currentRegs = tm.callVM.Regs()
			resumeMark := tm.tier2PerfStart()
			resumeOff, ok := cf.resumeOffset(int(ctx.CallID), ctx.ResumeNumericPass != 0)
			if !ok {
				return runtime.NilValue(), fmt.Errorf("callee call-exit: no resume for %d", ctx.CallID)
			}
			codePtr = uintptr(cf.Code.Ptr()) + uintptr(resumeOff)
			ctx.ExitCode = 0
			ctx.ResumeNumericPass = 0
			tm.tier2PerfStop(perfTier2ExitResume, resumeMark)
		case ExitNativeCallExit:
			var err error
			protocolMark := tm.tier2PerfStart()
			currentRegs, err = tm.executeNativeCallExit(ctx, cf, currentRegs, base, proto)
			tm.tier2PerfStop(perfTier2NativeCallExitProtocol, protocolMark)
			if err != nil {
				return runtime.NilValue(), fmt.Errorf("callee native-call-exit: %w", err)
			}
			resumeMark := tm.tier2PerfStart()
			resumeOff, ok := cf.resumeOffset(int(ctx.CallID), ctx.ResumeNumericPass != 0)
			if !ok {
				return runtime.NilValue(), fmt.Errorf("callee native-call-exit: no resume for %d", ctx.CallID)
			}
			codePtr = uintptr(cf.Code.Ptr()) + uintptr(resumeOff)
			ctx.ExitCode = 0
			ctx.ResumeNumericPass = 0
			tm.tier2PerfStop(perfTier2ExitResume, resumeMark)
		case ExitDeopt:
			tm.traceEvent("native_callee_deopt", "tier2", proto, map[string]any{
				"deopt_instr_id": ctx.DeoptInstrID,
				"resume_pc":      ctx.ExitResumePC,
				"call_id":        ctx.CallID,
				"table_exit_id":  ctx.TableExitID,
				"op_exit_id":     ctx.OpExitID,
			})
			action := tm.nativeCalleeDeoptAction(proto, cf, ctx, ctx.ExitResumePC)
			tm.applyTier2DeoptAction(proto, action)
			return tm.resumeNativeCalleePreciseDeopt(ctx, base, proto, ctx.ExitResumePC)
		default:
			return runtime.NilValue(), fmt.Errorf("unknown callee exit code %d", ctx.ExitCode)
		}
		ctx.Regs = uintptr(unsafe.Pointer(&currentRegs[base]))
		ctx.RegsBase = uintptr(unsafe.Pointer(&currentRegs[0]))
		ctx.RegsEnd = ctx.RegsBase + uintptr(len(currentRegs)*jit.ValueSize)
		tm.setTier2FieldCacheContext(ctx, proto)
	}
}

func (tm *TieringManager) nativeCalleeDeoptAction(proto *vm.FuncProto, cf *CompiledFunction, ctx *ExecContext, resumePC int64) Tier2DeoptAction {
	if ctx == nil {
		return Tier2DeoptAction{
			Kind:   Tier2DeoptDisableAndFallback,
			Reason: "tier2 native callee deopt",
		}
	}
	savedResumePC := ctx.ExitResumePC
	ctx.ExitResumePC = resumePC
	defer func() {
		ctx.ExitResumePC = savedResumePC
	}()
	action := Tier2DeoptPolicy{}.DecideRuntimeDeoptWithProfile(cf, int(resumePC), tm.currentTier2SpeculationProfile(proto))
	action.Reason = "tier2 native callee deopt"
	if guardAction, ok := tm.guardDeoptRefreshAction(proto, cf, ctx); ok {
		action = guardAction
	}
	return action
}

func (tm *TieringManager) resumeNativeCalleePreciseDeopt(ctx *ExecContext, base int, proto *vm.FuncProto, resumePC int64) (runtime.Value, error) {
	if tm.callVM == nil {
		return runtime.NilValue(), fmt.Errorf("callee deopt")
	}
	if resumePC <= 0 {
		return runtime.NilValue(), fmt.Errorf("callee deopt")
	}
	cl := ptrToVMClosure(ctx.NativeCalleeClosurePtr)
	if cl == nil || cl.Proto != proto {
		return runtime.NilValue(), fmt.Errorf("native-call-exit: callee closure unavailable for precise deopt")
	}
	if !tm.callVM.PushFrame(cl, base) {
		return runtime.NilValue(), fmt.Errorf("native-call-exit: stack overflow")
	}
	results, err := tm.callVM.ResumeFromPC(int(resumePC))
	tm.callVM.PopFrame()
	ctx.ExitResumePC = 0
	ctx.NativeCalleeResumePC = 0
	if err != nil {
		return runtime.NilValue(), err
	}
	if len(results) > 0 {
		return results[0], nil
	}
	return runtime.NilValue(), nil
}
