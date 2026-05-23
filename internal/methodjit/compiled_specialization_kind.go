//go:build darwin && arm64

package methodjit

import (
	"fmt"

	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

type compiledSpecializationKind uint8

const (
	compiledSpecializationNone compiledSpecializationKind = iota
	compiledSpecializationRuntimeRecursiveIntFold
	compiledSpecializationRuntimeRecursiveNestedIntFold
	compiledSpecializationRuntimeRecursiveTableBuilder
	compiledSpecializationRuntimeRecursiveTableFold
	compiledSpecializationMutualRecursiveIntSCC
)

func (k compiledSpecializationKind) String() string {
	switch k {
	case compiledSpecializationRuntimeRecursiveIntFold:
		return "recursive_int_fold"
	case compiledSpecializationRuntimeRecursiveNestedIntFold:
		return "nested_recursive_int_fold"
	case compiledSpecializationRuntimeRecursiveTableBuilder:
		return "lazy_recursive_table_builder"
	case compiledSpecializationRuntimeRecursiveTableFold:
		return "lazy_recursive_table_fold"
	case compiledSpecializationMutualRecursiveIntSCC:
		return "mutual_recursive_int_scc"
	default:
		return "none"
	}
}

func (cf *CompiledFunction) SpecializationKind() compiledSpecializationKind {
	if cf == nil {
		return compiledSpecializationNone
	}
	switch {
	case cf.RuntimeRecursiveIntFold != nil:
		return compiledSpecializationRuntimeRecursiveIntFold
	case cf.RuntimeRecursiveNestedIntFold != nil:
		return compiledSpecializationRuntimeRecursiveNestedIntFold
	case cf.RuntimeRecursiveTableBuilder != nil:
		return compiledSpecializationRuntimeRecursiveTableBuilder
	case cf.RuntimeRecursiveTableFold != nil:
		return compiledSpecializationRuntimeRecursiveTableFold
	case cf.MutualRecursiveIntSCC != nil:
		return compiledSpecializationMutualRecursiveIntSCC
	default:
		return compiledSpecializationNone
	}
}

func (tm *TieringManager) executeCompiledSpecialization(cf *CompiledFunction, regs []runtime.Value, base int, proto *vm.FuncProto, retBuf []runtime.Value) ([]runtime.Value, bool, error) {
	kind := cf.SpecializationKind()
	if kind == compiledSpecializationNone {
		return nil, false, nil
	}
	mark := tm.tier2PerfStart()

	switch kind {
	case compiledSpecializationRuntimeRecursiveIntFold:
		out, err := tm.executeRuntimeRecursiveIntFold(cf, regs, base, proto, retBuf)
		tm.tier2PerfStop(perfTier2CompiledSpecialization, mark)
		return out, true, err
	case compiledSpecializationRuntimeRecursiveNestedIntFold:
		out, err := tm.executeRuntimeRecursiveNestedIntFold(cf, regs, base, proto, retBuf)
		tm.tier2PerfStop(perfTier2CompiledSpecialization, mark)
		return out, true, err
	case compiledSpecializationRuntimeRecursiveTableBuilder:
		out, err := tm.executeRuntimeRecursiveTableBuilder(cf, regs, base, proto, retBuf)
		tm.tier2PerfStop(perfTier2CompiledSpecialization, mark)
		return out, true, err
	case compiledSpecializationRuntimeRecursiveTableFold:
		out, err := tm.executeRuntimeRecursiveTableFold(cf, regs, base, proto, retBuf)
		tm.tier2PerfStop(perfTier2CompiledSpecialization, mark)
		return out, true, err
	case compiledSpecializationMutualRecursiveIntSCC:
		out, err := tm.executeMutualRecursiveIntSCC(cf, regs, base, proto, retBuf)
		tm.tier2PerfStop(perfTier2CompiledSpecialization, mark)
		return out, true, err
	default:
		tm.tier2PerfStop(perfTier2CompiledSpecialization, mark)
		return nil, true, fmt.Errorf("tier2: unknown compiled specialization kind %d", kind)
	}
}

func (tm *TieringManager) tryCompiledSpecializationCallExit(fnVal runtime.Value, regs []runtime.Value, absSlot, nArgs, nRets int) (bool, error) {
	cl, ok := vmClosureFromValue(fnVal)
	if !ok || cl == nil || cl.Proto == nil || tm == nil || tm.callVM == nil {
		return false, nil
	}
	calleeProto := cl.Proto
	if nArgs != calleeProto.NumParams || nArgs < 0 || absSlot+nArgs >= len(regs) {
		return false, nil
	}
	cf, ok := tm.tier2CompiledFor(calleeProto)
	if !ok || cf == nil {
		return false, nil
	}
	kind := cf.SpecializationKind()
	if kind == compiledSpecializationNone || !compiledSpecializationCallExitFastPathSupports(kind) {
		return false, nil
	}

	mark := tm.tier2PerfStart()
	result, ok, err := tm.executeCompiledSpecializationCallExitResult(cf, calleeProto, regs, absSlot, nArgs)
	tm.tier2PerfStop(perfTier2CompiledSpecializationCallExit, mark)
	if err != nil {
		return false, nil
	}
	if !ok {
		return false, nil
	}
	storeCallExitSingleResult(regs, absSlot, nRets, result)
	if currentRegs := tm.callVM.Regs(); len(currentRegs) > 0 {
		storeCallExitSingleResult(currentRegs, absSlot, nRets, result)
	}
	return true, nil
}

func (tm *TieringManager) TryExecuteCompiledSpecializationCall(fnVal runtime.Value, regs []runtime.Value, absSlot, nArgs, nRets int) (bool, error) {
	return tm.tryCompiledSpecializationCallExit(fnVal, regs, absSlot, nArgs, nRets)
}

func (tm *TieringManager) executeCompiledSpecializationCallExitResult(cf *CompiledFunction, proto *vm.FuncProto, regs []runtime.Value, absSlot, nArgs int) (runtime.Value, bool, error) {
	switch cf.SpecializationKind() {
	case compiledSpecializationRuntimeRecursiveIntFold:
		if nArgs != 1 || absSlot+1 >= len(regs) {
			return runtime.NilValue(), false, nil
		}
		if !tm.runtimeRecursiveSelfGlobalMatches(proto) {
			tm.disableTier2AfterRuntimeDeopt(proto, "tier2: runtime recursive int fold self global changed")
			return runtime.NilValue(), false, nil
		}
		n, ok := cf.RuntimeRecursiveIntFold.fold(regs[absSlot+1])
		if !ok {
			return runtime.NilValue(), false, nil
		}
		proto.EnteredTier2 = 1
		return runtime.IntValue(n), true, nil
	case compiledSpecializationRuntimeRecursiveNestedIntFold:
		if nArgs != 2 || absSlot+2 >= len(regs) {
			return runtime.NilValue(), false, nil
		}
		if !tm.runtimeRecursiveSelfGlobalMatches(proto) {
			tm.disableTier2AfterRuntimeDeopt(proto, "tier2: runtime recursive nested int fold self global changed")
			return runtime.NilValue(), false, nil
		}
		n, ok := cf.RuntimeRecursiveNestedIntFold.fold(regs[absSlot+1], regs[absSlot+2])
		if !ok {
			return runtime.NilValue(), false, nil
		}
		proto.EnteredTier2 = 1
		return runtime.IntValue(n), true, nil
	case compiledSpecializationMutualRecursiveIntSCC:
		if nArgs < 0 || nArgs > 4 || absSlot+nArgs >= len(regs) {
			return runtime.NilValue(), false, nil
		}
		var args [4]int64
		for i := 0; i < nArgs; i++ {
			arg := regs[absSlot+1+i]
			if !arg.IsInt() || arg.Int() < 0 {
				return runtime.NilValue(), false, nil
			}
			args[i] = arg.Int()
		}
		n, ok, err := tm.executeMutualRecursiveIntSCCArgs(cf, proto, args)
		if err != nil || !ok {
			return runtime.NilValue(), ok, err
		}
		return runtime.IntValue(n), true, nil
	default:
		return runtime.NilValue(), false, nil
	}
}

func compiledSpecializationCallExitFastPathSupports(kind compiledSpecializationKind) bool {
	switch kind {
	case compiledSpecializationRuntimeRecursiveIntFold,
		compiledSpecializationRuntimeRecursiveNestedIntFold,
		compiledSpecializationMutualRecursiveIntSCC:
		return true
	default:
		return false
	}
}
