//go:build darwin && arm64

// tiering_manager_exit_call.go holds the Tier 2 call-exit handler and its
// callee promotion / arg-shape observation helpers. A call-exit fires when
// Tier 2 native code reaches a call it cannot dispatch natively.
//
// Pure code movement from tiering_manager_exit.go; no behavior change.

package methodjit

import (
	"fmt"

	"github.com/Never-Labs/gscript/internal/runtime"
	"github.com/Never-Labs/gscript/internal/vm"
)

// executeCallExit handles a call-exit in the TieringManager's Tier 2 path.
func (tm *TieringManager) executeCallExit(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto, cf *CompiledFunction) error {
	if tm.callVM == nil {
		return fmt.Errorf("no callVM set for call-exit")
	}

	callSlot := int(ctx.CallSlot)
	nArgs := int(ctx.CallNArgs)
	nRets := int(ctx.CallNRets)

	absSlot := base + callSlot
	if absSlot >= len(regs) {
		return fmt.Errorf("call slot %d (abs %d) out of range (regs len %d)", callSlot, absSlot, len(regs))
	}
	fnVal := regs[absSlot]
	observeTier2CallExitFeedback(proto, cf, ctx, regs, base)

	if handled, err := tm.callVM.TryFastCoroutineCallValue(fnVal, absSlot, nArgs, nRets+1); handled {
		return err
	}

	if handled, err := tm.tryCompiledSpecializationCallExit(fnVal, regs, absSlot, nArgs, nRets); handled || err != nil {
		if handled && err == nil {
			currentRegs := tm.callVM.Regs()
			if absSlot >= 0 && absSlot < len(currentRegs) {
				observeTier2CallExitResultFeedback(proto, cf, ctx, currentRegs[absSlot], true)
			}
		}
		return err
	}

	if gf := fnVal.GoFunction(); gf != nil {
		result, ok, err := callGoFunctionFast(gf, regs, absSlot, nArgs)
		if err != nil || ok {
			if err != nil {
				return err
			}
			currentRegs := tm.callVM.Regs()
			storeCallExitSingleResult(currentRegs, absSlot, nRets, result)
			observeTier2CallExitResultFeedback(proto, cf, ctx, result, true)
			return nil
		}
	}

	callArgs := collectCallExitArgs(regs, absSlot, nArgs)
	observeTier2CallExitCalleeArgShapes(fnVal, callArgs)
	if gf := fnVal.GoFunction(); gf != nil && gf.Fast1 != nil {
		result, err := gf.Fast1(callArgs)
		if err != nil {
			return err
		}
		currentRegs := tm.callVM.Regs()
		storeCallExitSingleResult(currentRegs, absSlot, nRets, result)
		observeTier2CallExitResultFeedback(proto, cf, ctx, result, true)
		return nil
	}

	results, err := tm.callValueForTier2Exit(fnVal, callArgs, proto)
	if err != nil {
		return err
	}

	// Re-read regs — CallValue may have grown the register file.
	currentRegs := tm.callVM.Regs()

	nr := nRets
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
	if len(results) > 0 {
		observeTier2CallExitResultFeedback(proto, cf, ctx, results[0], true)
	} else {
		observeTier2CallExitResultFeedback(proto, cf, ctx, runtime.NilValue(), false)
	}

	// After the callee has executed once (in Tier 1 or interpreter),
	// try to promote it to Tier 2. When the caller is Tier 2 and calls
	// through the IC, the IC caches the callee's entry on the second
	// call and never calls TryCompile again. This hook ensures the
	// callee gets Tier 2 compiled after its first execution, so the
	// IC picks up the Tier 2 entry via the version-refresh path.
	tm.tryPromoteCallExitCallee(fnVal)

	return nil
}

func (tm *TieringManager) tryPromoteCallExitCallee(fnVal runtime.Value) {
	if tm == nil || fnVal.IsNil() {
		return
	}
	cl, ok := vmClosureFromValue(fnVal)
	if !ok || cl == nil || cl.Proto == nil {
		return
	}
	calleeProto := cl.Proto
	if _, ok := tm.tier2CompiledFor(calleeProto); ok {
		return
	}
	if tm.tier2HasFailed(calleeProto) {
		return
	}
	profile := tm.getProfile(calleeProto)
	if !shouldPromoteTier2(calleeProto, profile, calleeProto.CallCount) {
		return
	}
	if !canPromoteToTier2(calleeProto) {
		return
	}
	if cf, err := tm.compileTier2(calleeProto); err == nil {
		tm.markTier2Compiled(calleeProto, cf)
	}
}

func observeTier2CallExitCalleeArgShapes(fnVal runtime.Value, args []runtime.Value) {
	cl, ok := vmClosureFromValue(fnVal)
	if !ok || cl == nil || cl.Proto == nil || cl.Proto.IsVarArg || cl.Proto.NumParams == 0 {
		return
	}
	cl.Proto.ObserveArgShapes(args)
	cl.Proto.ObserveArgArrayElementShapes(args)
}

func (tm *TieringManager) callValueForTier2Exit(fnVal runtime.Value, args []runtime.Value, callerProto *vm.FuncProto) ([]runtime.Value, error) {
	if !tm.shouldSuppressUnsafeSelfTier2Reentry(fnVal, callerProto) {
		return tm.callVM.CallValue(fnVal, args)
	}

	// DirectEntrySafe=false means native callers may not safely recurse into
	// this Tier 2 body. A self call-exit that goes through VM.CallValue would
	// otherwise re-enter the same Tier 2 function through the normal VM JIT
	// dispatch path, recreating the native stack nesting the direct-entry gate
	// was meant to avoid.
	var (
		values []runtime.Value
		err    error
	)
	tm.withJITTemporarilyDisabled(callerProto, func() {
		values, err = tm.callVM.CallValue(fnVal, args)
	})
	return values, err
}

func (tm *TieringManager) shouldSuppressUnsafeSelfTier2Reentry(fnVal runtime.Value, callerProto *vm.FuncProto) bool {
	if tm == nil || callerProto == nil {
		return false
	}
	cl, ok := vmClosureFromValue(fnVal)
	if !ok || cl == nil || cl.Proto != callerProto {
		return false
	}
	cf, _ := tm.tier2CompiledFor(callerProto)
	return cf != nil && !cf.DirectEntrySafe
}
