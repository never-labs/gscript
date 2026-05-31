//go:build darwin && arm64

package methodjit

import (
	"fmt"

	"github.com/never-labs/gscript/internal/runtime"
	"github.com/never-labs/gscript/internal/vm"
)

// Keep the native call-site builder bounded by a practical allocation limit.
// depth=20 is already roughly two million nodes; deeper inputs fall back to the
// interpreter so unusual programs keep normal VM semantics instead of letting a
// specialized runtime path monopolize the process.
const runtimeRecursiveTableBuilderMaxDepth = 20

type runtimeRecursiveTableBuilderSpecialization struct {
	ctor runtime.SmallTableCtor2
}

func qualifiesForRuntimeRecursiveTableBuilder(proto *vm.FuncProto) bool {
	_, ok := analyzeRuntimeRecursiveTableBuilder(proto)
	return ok
}

func analyzeRuntimeRecursiveTableBuilder(proto *vm.FuncProto) (*runtimeRecursiveTableBuilderSpecialization, bool) {
	if proto == nil || proto.IsVarArg || proto.NumParams != 1 || proto.Name == "" {
		return nil, false
	}
	if len(proto.Upvalues) != 0 || len(proto.Protos) != 0 || len(proto.Code) != 15 {
		return nil, false
	}
	code := proto.Code
	if vm.DecodeOp(code[0]) != vm.OP_LOADINT || vm.DecodeA(code[0]) != 1 || vm.DecodesBx(code[0]) != 0 {
		return nil, false
	}
	if vm.DecodeOp(code[1]) != vm.OP_EQ || vm.DecodeA(code[1]) != 0 ||
		!((vm.DecodeB(code[1]) == 0 && vm.DecodeC(code[1]) == 1) ||
			(vm.DecodeB(code[1]) == 1 && vm.DecodeC(code[1]) == 0)) {
		return nil, false
	}
	if vm.DecodeOp(code[2]) != vm.OP_JMP || 3+vm.DecodesBx(code[2]) != 5 {
		return nil, false
	}
	if vm.DecodeOp(code[3]) != vm.OP_NEWTABLE ||
		vm.DecodeA(code[3]) != 1 || vm.DecodeB(code[3]) != 0 || vm.DecodeC(code[3]) != 0 {
		return nil, false
	}
	if vm.DecodeOp(code[4]) != vm.OP_RETURN || vm.DecodeA(code[4]) != 1 || vm.DecodeB(code[4]) != 2 {
		return nil, false
	}
	if !runtimeBuilderSelfCall(proto, code[5], code[6], code[7], code[8], 2, 3) {
		return nil, false
	}
	if !runtimeBuilderSelfCall(proto, code[9], code[10], code[11], code[12], 3, 4) {
		return nil, false
	}
	if vm.DecodeOp(code[13]) != vm.OP_NEWOBJECT2 || vm.DecodeA(code[13]) != 1 ||
		vm.DecodeC(code[13]) != 2 {
		return nil, false
	}
	ctorIdx := vm.DecodeB(code[13])
	if ctorIdx < 0 || ctorIdx >= len(proto.TableCtors2) {
		return nil, false
	}
	if vm.DecodeOp(code[14]) != vm.OP_RETURN || vm.DecodeA(code[14]) != 1 || vm.DecodeB(code[14]) != 2 {
		return nil, false
	}
	ctor := proto.TableCtors2[ctorIdx].Runtime
	if !cacheableSmallCtor2(&ctor) {
		return nil, false
	}
	return &runtimeRecursiveTableBuilderSpecialization{ctor: ctor}, true
}

func runtimeBuilderSelfCall(proto *vm.FuncProto, get, one, sub, call uint32, fnSlot, argSlot int) bool {
	if vm.DecodeOp(get) != vm.OP_GETGLOBAL || vm.DecodeA(get) != fnSlot {
		return false
	}
	if protoConstString(proto, vm.DecodeBx(get)) != proto.Name {
		return false
	}
	if vm.DecodeOp(one) != vm.OP_LOADINT || vm.DecodeA(one) != argSlot+1 || vm.DecodesBx(one) != 1 {
		return false
	}
	if vm.DecodeOp(sub) != vm.OP_SUB || vm.DecodeA(sub) != argSlot ||
		vm.DecodeB(sub) != 0 || vm.DecodeC(sub) != argSlot+1 {
		return false
	}
	return vm.DecodeOp(call) == vm.OP_CALL &&
		vm.DecodeA(call) == fnSlot &&
		vm.DecodeB(call) == 2 &&
		vm.DecodeC(call) == 2
}

func newRuntimeRecursiveTableBuilderCompiled(proto *vm.FuncProto) (*CompiledFunction, bool) {
	specialization, ok := analyzeRuntimeRecursiveTableBuilder(proto)
	if !ok {
		return nil, false
	}
	return &CompiledFunction{
		Proto:                        proto,
		numRegs:                      proto.MaxStack,
		RuntimeRecursiveTableBuilder: specialization,
	}, true
}

func (tm *TieringManager) compileRuntimeRecursiveTableBuilderTier2(proto *vm.FuncProto) (*CompiledFunction, bool) {
	cf, ok := newRuntimeRecursiveTableBuilderCompiled(proto)
	if !ok {
		return nil, false
	}
	tm.tier2Attempts++
	attempt := tm.tier2Attempts
	tm.traceEvent("tier2_attempt", "tier2", proto, map[string]any{
		"attempt":        attempt,
		"call_count":     proto.CallCount,
		"specialization": "lazy_recursive_table_builder",
	})
	tm.traceTier2Success(proto, cf, attempt)
	return cf, true
}

func (tm *TieringManager) executeRuntimeRecursiveTableBuilder(cf *CompiledFunction, regs []runtime.Value, base int, proto *vm.FuncProto, retBuf []runtime.Value) ([]runtime.Value, error) {
	if cf == nil || cf.RuntimeRecursiveTableBuilder == nil || proto == nil {
		return nil, fmt.Errorf("tier2: missing runtime recursive table builder specialization")
	}
	if base < 0 || base >= len(regs) {
		return nil, fmt.Errorf("tier2: runtime recursive table builder base %d outside regs len %d", base, len(regs))
	}
	if !tm.runtimeRecursiveSelfGlobalMatches(proto) {
		tm.disableTier2AfterRuntimeDeopt(proto, "tier2: runtime recursive table builder self global changed")
		return nil, fmt.Errorf("tier2: runtime recursive table builder self global changed")
	}
	depthValue := regs[base]
	if !depthValue.IsInt() {
		tm.disableTier2AfterRuntimeDeopt(proto, "tier2: runtime recursive table builder non-int depth")
		return nil, fmt.Errorf("tier2: runtime recursive table builder non-int depth")
	}
	depth := depthValue.Int()
	if depth < 0 || depth > runtimeRecursiveTableBuilderMaxDepth {
		tm.disableTier2AfterRuntimeDeopt(proto, "tier2: runtime recursive table builder depth outside fast range")
		return nil, fmt.Errorf("tier2: runtime recursive table builder depth outside fast range")
	}
	result := cf.RuntimeRecursiveTableBuilder.build(depth)
	regs[base] = result
	proto.EnteredTier2 = 1
	return runtime.ReuseValueSlice1(retBuf, result), nil
}

func (p *runtimeRecursiveTableBuilderSpecialization) build(depth int64) runtime.Value {
	return runtime.FreshTableValue(runtime.NewLazyRecursiveTable(&p.ctor, depth))
}
