//go:build darwin && arm64

// tiering_manager_exit_batch.go holds the call-site no-result runtime
// specialization batch loop driven from the OpCall op-exit, plus the global
// value resolution / cache invalidation helpers it relies on.
//
// Pure code movement from tiering_manager_exit.go; no behavior change.

package methodjit

import (
	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

func (tm *TieringManager) executeCallSiteNoResultRuntimeSpecializationBatch(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	if tm == nil || tm.callVM == nil || ctx == nil || proto == nil {
		return nil
	}
	cf, ok := tm.tier2CompiledFor(proto)
	if !ok || cf == nil || len(cf.CallSiteNoResultRuntimeSpecializationBatches) == 0 {
		return nil
	}
	fact, ok := cf.CallSiteNoResultRuntimeSpecializationBatches[int(ctx.OpExitID)]
	if !ok || len(fact.Calls) == 0 {
		return nil
	}
	loopBase := base + fact.LoopBase
	if loopBase < 0 || loopBase+3 >= len(regs) {
		return nil
	}
	cur, limit, step, ok := callSiteBatchIntLoopState(regs[loopBase], regs[loopBase+1], regs[loopBase+2])
	if !ok || step == 0 {
		return nil
	}
	lastComplete := cur
	for next := cur + step; callSiteBatchLoopContinues(next, limit, step); next += step {
		iterComplete := true
		for _, call := range fact.Calls {
			handled, err := tm.executeCallSiteNoResultRuntimeSpecializationBatchCall(proto, call)
			if err != nil {
				return err
			}
			if !handled {
				iterComplete = false
				break
			}
		}
		if !iterComplete {
			break
		}
		lastComplete = next
	}
	if lastComplete != limit {
		return nil
	}
	ctx.OpExitAux = 1
	return nil
}

func callSiteBatchIntLoopState(cur, limit, step runtime.Value) (int64, int64, int64, bool) {
	if !cur.IsInt() || !limit.IsInt() || !step.IsInt() {
		return 0, 0, 0, false
	}
	return cur.Int(), limit.Int(), step.Int(), true
}

func callSiteBatchLoopContinues(next, limit, step int64) bool {
	if step > 0 {
		return next <= limit
	}
	return next >= limit
}

func (tm *TieringManager) executeCallSiteNoResultRuntimeSpecializationBatchCall(proto *vm.FuncProto, call CallSiteNoResultRuntimeSpecializationBatchCall) (bool, error) {
	if tm == nil || tm.callVM == nil || call.FuncConst < 0 {
		return false, nil
	}
	fnVal, ok := tm.globalValueByConst(proto, call.FuncConst)
	if !ok {
		return false, nil
	}
	var local [3]runtime.Value
	args := local[:0]
	if len(call.ArgConsts) > len(local) {
		args = make([]runtime.Value, 0, len(call.ArgConsts))
	}
	for _, constIdx := range call.ArgConsts {
		val, ok := tm.globalValueByConst(proto, constIdx)
		if !ok {
			return false, nil
		}
		args = append(args, val)
	}
	handled, err := tm.callVM.TryRunNoResultCallSiteRuntimeSpecializationForJIT(fnVal, args)
	if handled || err != nil {
		return handled, err
	}
	if _, err := tm.callVM.CallValue(fnVal, args); err != nil {
		return true, err
	}
	return true, nil
}

func (tm *TieringManager) globalValueByConst(proto *vm.FuncProto, constIdx int) (runtime.Value, bool) {
	if tm == nil || tm.callVM == nil || constIdx < 0 {
		return runtime.NilValue(), false
	}
	// Batch metadata only records GETGLOBAL-backed call recipes; the constant
	// index is resolved through the VM's current globals so normal global
	// rebinding still guards/falls back through the VM closure/specialization checks.
	if proto == nil || constIdx >= len(proto.Constants) {
		return runtime.NilValue(), false
	}
	nameVal := proto.Constants[constIdx]
	if !nameVal.IsString() {
		return runtime.NilValue(), false
	}
	return tm.callVM.GetGlobal(nameVal.Str()), true
}

func (tm *TieringManager) invalidateGlobalValueCaches(name string) {
	if name == "" {
		return
	}
	tm.tier1.invalidateGlobalValueCaches(name)
	tm.forEachTier2Compiled(func(_ *vm.FuncProto, cf *CompiledFunction) {
		if cf == nil || cf.Proto == nil || len(cf.GlobalCache) == 0 {
			return
		}
		for cacheIdx, constIdx := range cf.GlobalCacheConsts {
			if cacheIdx >= len(cf.GlobalCache) || constIdx < 0 || constIdx >= len(cf.Proto.Constants) {
				continue
			}
			kv := cf.Proto.Constants[constIdx]
			if kv.IsString() && kv.Str() == name {
				cf.GlobalCache[cacheIdx] = 0
			}
		}
	})
}
