//go:build darwin && arm64

package methodjit

import (
	"fmt"

	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

type compiledProtocolKind uint8

const (
	compiledProtocolNone compiledProtocolKind = iota
	compiledProtocolRuntimeRecursiveIntFold
	compiledProtocolRuntimeRecursiveNestedIntFold
	compiledProtocolRuntimeRecursiveTableBuilder
	compiledProtocolRuntimeRecursiveTableFold
	compiledProtocolMutualRecursiveIntSCC
)

func (k compiledProtocolKind) String() string {
	switch k {
	case compiledProtocolRuntimeRecursiveIntFold:
		return "recursive_int_fold"
	case compiledProtocolRuntimeRecursiveNestedIntFold:
		return "nested_recursive_int_fold"
	case compiledProtocolRuntimeRecursiveTableBuilder:
		return "lazy_recursive_table_builder"
	case compiledProtocolRuntimeRecursiveTableFold:
		return "lazy_recursive_table_fold"
	case compiledProtocolMutualRecursiveIntSCC:
		return "mutual_recursive_int_scc"
	default:
		return "none"
	}
}

func (cf *CompiledFunction) ProtocolKind() compiledProtocolKind {
	if cf == nil {
		return compiledProtocolNone
	}
	switch {
	case cf.RuntimeRecursiveIntFold != nil:
		return compiledProtocolRuntimeRecursiveIntFold
	case cf.RuntimeRecursiveNestedIntFold != nil:
		return compiledProtocolRuntimeRecursiveNestedIntFold
	case cf.RuntimeRecursiveTableBuilder != nil:
		return compiledProtocolRuntimeRecursiveTableBuilder
	case cf.RuntimeRecursiveTableFold != nil:
		return compiledProtocolRuntimeRecursiveTableFold
	case cf.MutualRecursiveIntSCC != nil:
		return compiledProtocolMutualRecursiveIntSCC
	default:
		return compiledProtocolNone
	}
}

func (tm *TieringManager) executeCompiledProtocol(cf *CompiledFunction, regs []runtime.Value, base int, proto *vm.FuncProto, retBuf []runtime.Value) ([]runtime.Value, bool, error) {
	kind := cf.ProtocolKind()
	if kind == compiledProtocolNone {
		return nil, false, nil
	}
	mark := tm.tier2PerfStart()

	switch kind {
	case compiledProtocolRuntimeRecursiveIntFold:
		out, err := tm.executeRuntimeRecursiveIntFold(cf, regs, base, proto, retBuf)
		tm.tier2PerfStop(perfTier2CompiledProtocol, mark)
		return out, true, err
	case compiledProtocolRuntimeRecursiveNestedIntFold:
		out, err := tm.executeRuntimeRecursiveNestedIntFold(cf, regs, base, proto, retBuf)
		tm.tier2PerfStop(perfTier2CompiledProtocol, mark)
		return out, true, err
	case compiledProtocolRuntimeRecursiveTableBuilder:
		out, err := tm.executeRuntimeRecursiveTableBuilder(cf, regs, base, proto, retBuf)
		tm.tier2PerfStop(perfTier2CompiledProtocol, mark)
		return out, true, err
	case compiledProtocolRuntimeRecursiveTableFold:
		out, err := tm.executeRuntimeRecursiveTableFold(cf, regs, base, proto, retBuf)
		tm.tier2PerfStop(perfTier2CompiledProtocol, mark)
		return out, true, err
	case compiledProtocolMutualRecursiveIntSCC:
		out, err := tm.executeMutualRecursiveIntSCC(cf, regs, base, proto, retBuf)
		tm.tier2PerfStop(perfTier2CompiledProtocol, mark)
		return out, true, err
	default:
		tm.tier2PerfStop(perfTier2CompiledProtocol, mark)
		return nil, true, fmt.Errorf("tier2: unknown compiled protocol kind %d", kind)
	}
}

func (tm *TieringManager) tryCompiledProtocolCallExit(fnVal runtime.Value, regs []runtime.Value, absSlot, nArgs, nRets int) (bool, error) {
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
	kind := cf.ProtocolKind()
	if kind == compiledProtocolNone || !compiledProtocolCallExitFastPathSupports(kind) {
		return false, nil
	}

	mark := tm.tier2PerfStart()
	result, ok, err := tm.executeCompiledProtocolCallExitResult(cf, calleeProto, regs, absSlot, nArgs)
	tm.tier2PerfStop(perfTier2CompiledProtocolCallExit, mark)
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

func (tm *TieringManager) TryExecuteCompiledProtocolCall(fnVal runtime.Value, regs []runtime.Value, absSlot, nArgs, nRets int) (bool, error) {
	return tm.tryCompiledProtocolCallExit(fnVal, regs, absSlot, nArgs, nRets)
}

func (tm *TieringManager) executeCompiledProtocolCallExitResult(cf *CompiledFunction, proto *vm.FuncProto, regs []runtime.Value, absSlot, nArgs int) (runtime.Value, bool, error) {
	switch cf.ProtocolKind() {
	case compiledProtocolRuntimeRecursiveIntFold:
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
	case compiledProtocolRuntimeRecursiveNestedIntFold:
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
	case compiledProtocolMutualRecursiveIntSCC:
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

func compiledProtocolCallExitFastPathSupports(kind compiledProtocolKind) bool {
	switch kind {
	case compiledProtocolRuntimeRecursiveIntFold,
		compiledProtocolRuntimeRecursiveNestedIntFold,
		compiledProtocolMutualRecursiveIntSCC:
		return true
	default:
		return false
	}
}
