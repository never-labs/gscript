//go:build darwin && arm64

package methodjit

import (
	"fmt"

	"github.com/Never-Labs/gscript/internal/runtime"
	"github.com/Never-Labs/gscript/internal/vm"
)

type runtimeRecursiveTableFoldSpecialization struct {
	nilField    string
	nilCache    runtime.FieldCacheEntry
	baseValue   int64
	combineBias int64
	children    []runtimeRecursiveTableFoldChild
}

type runtimeRecursiveTableFoldChild struct {
	field string
	cache runtime.FieldCacheEntry
}

type runtimeRecursiveTableFoldExpr struct {
	constant int64
	calls    map[string]int
	valid    bool
}

type runtimeRecursiveTableFoldSlot struct {
	selfFunc bool
	field    string
	expr     runtimeRecursiveTableFoldExpr
}

const (
	runtimeFoldMaxInt64 = int64(^uint64(0) >> 1)
	runtimeFoldMinInt64 = -runtimeFoldMaxInt64 - 1
	runtimeFoldMaxInt48 = (1 << 47) - 1
	runtimeFoldMinInt48 = -(1 << 47)
)

func qualifiesForRuntimeRecursiveTableFold(proto *vm.FuncProto) bool {
	_, ok := analyzeRuntimeRecursiveTableFold(proto)
	return ok
}

func analyzeRuntimeRecursiveTableFold(proto *vm.FuncProto) (*runtimeRecursiveTableFoldSpecialization, bool) {
	if proto == nil || proto.IsVarArg || proto.NumParams != 1 || proto.Name == "" {
		return nil, false
	}
	if len(proto.Upvalues) != 0 || len(proto.Protos) != 0 || len(proto.Code) < 8 {
		return nil, false
	}

	nilField, basePC, recursePC, ok := runtimeFoldParseNilBaseHeader(proto)
	if !ok {
		return nil, false
	}
	baseValue, ok := runtimeFoldParseIntReturn(proto, basePC)
	if !ok {
		return nil, false
	}
	expr, children, ok := runtimeFoldParseRecursiveExpr(proto, recursePC)
	if !ok || !expr.valid || len(expr.calls) == 0 {
		return nil, false
	}
	for _, child := range children {
		if expr.calls[child.field] != 1 {
			return nil, false
		}
	}
	if len(expr.calls) != len(children) {
		return nil, false
	}
	if !runtimeFoldHasChild(children, nilField) {
		return nil, false
	}

	planChildren := make([]runtimeRecursiveTableFoldChild, len(children))
	for i, child := range children {
		planChildren[i] = runtimeRecursiveTableFoldChild{field: child.field}
	}
	return &runtimeRecursiveTableFoldSpecialization{
		nilField:    nilField,
		baseValue:   baseValue,
		combineBias: expr.constant,
		children:    planChildren,
	}, true
}

func runtimeFoldHasChild(children []runtimeRecursiveTableFoldChild, field string) bool {
	for _, child := range children {
		if child.field == field {
			return true
		}
	}
	return false
}

func runtimeFoldParseNilBaseHeader(proto *vm.FuncProto) (nilField string, basePC, recursePC int, ok bool) {
	code := proto.Code
	if len(code) < 6 || vm.DecodeOp(code[0]) != vm.OP_GETFIELD || vm.DecodeB(code[0]) != 0 {
		return "", 0, 0, false
	}
	nilField = protoConstString(proto, vm.DecodeC(code[0]))
	if nilField == "" {
		return "", 0, 0, false
	}
	if vm.DecodeOp(code[1]) != vm.OP_LOADNIL {
		return "", 0, 0, false
	}
	fieldSlot := vm.DecodeA(code[0])
	nilStart := vm.DecodeA(code[1])
	nilEnd := nilStart + vm.DecodeB(code[1])
	if vm.DecodeOp(code[2]) != vm.OP_EQ {
		return "", 0, 0, false
	}
	eqB, eqC := vm.DecodeB(code[2]), vm.DecodeC(code[2])
	if !((eqB == fieldSlot && eqC >= nilStart && eqC <= nilEnd) ||
		(eqC == fieldSlot && eqB >= nilStart && eqB <= nilEnd)) {
		return "", 0, 0, false
	}
	if vm.DecodeOp(code[3]) != vm.OP_JMP {
		return "", 0, 0, false
	}
	basePC = 4
	recursePC = 4 + vm.DecodesBx(code[3])
	if recursePC <= basePC || recursePC >= len(code) {
		return "", 0, 0, false
	}
	return nilField, basePC, recursePC, true
}

func runtimeFoldParseIntReturn(proto *vm.FuncProto, pc int) (int64, bool) {
	if pc < 0 || pc+1 >= len(proto.Code) {
		return 0, false
	}
	load := proto.Code[pc]
	ret := proto.Code[pc+1]
	if vm.DecodeOp(load) != vm.OP_LOADINT || vm.DecodeOp(ret) != vm.OP_RETURN {
		return 0, false
	}
	if vm.DecodeA(ret) != vm.DecodeA(load) || vm.DecodeB(ret) != 2 {
		return 0, false
	}
	return int64(vm.DecodesBx(load)), true
}

func runtimeFoldParseRecursiveExpr(proto *vm.FuncProto, startPC int) (runtimeRecursiveTableFoldExpr, []runtimeRecursiveTableFoldChild, bool) {
	slots := make([]runtimeRecursiveTableFoldSlot, maxTrackedSlots)
	children := make([]runtimeRecursiveTableFoldChild, 0, 2)
	for pc := startPC; pc < len(proto.Code); pc++ {
		inst := proto.Code[pc]
		a, b, c := vm.DecodeA(inst), vm.DecodeB(inst), vm.DecodeC(inst)
		switch vm.DecodeOp(inst) {
		case vm.OP_LOADINT:
			if !runtimeFoldSlotOK(a) {
				return runtimeRecursiveTableFoldExpr{}, nil, false
			}
			slots[a] = runtimeRecursiveTableFoldSlot{expr: runtimeFoldConst(int64(vm.DecodesBx(inst)))}
		case vm.OP_MOVE:
			if !runtimeFoldSlotOK(a) || !runtimeFoldSlotOK(b) {
				return runtimeRecursiveTableFoldExpr{}, nil, false
			}
			slots[a] = slots[b]
		case vm.OP_GETGLOBAL:
			if !runtimeFoldSlotOK(a) || protoConstString(proto, vm.DecodeBx(inst)) != proto.Name {
				return runtimeRecursiveTableFoldExpr{}, nil, false
			}
			slots[a] = runtimeRecursiveTableFoldSlot{selfFunc: true}
		case vm.OP_GETFIELD:
			if !runtimeFoldSlotOK(a) || b != 0 {
				return runtimeRecursiveTableFoldExpr{}, nil, false
			}
			field := protoConstString(proto, c)
			if field == "" {
				return runtimeRecursiveTableFoldExpr{}, nil, false
			}
			slots[a] = runtimeRecursiveTableFoldSlot{field: field}
		case vm.OP_CALL:
			if !runtimeFoldSlotOK(a) || !slots[a].selfFunc || b != 2 || c != 2 || !runtimeFoldSlotOK(a+1) {
				return runtimeRecursiveTableFoldExpr{}, nil, false
			}
			field := slots[a+1].field
			if field == "" {
				return runtimeRecursiveTableFoldExpr{}, nil, false
			}
			children = append(children, runtimeRecursiveTableFoldChild{field: field})
			slots[a] = runtimeRecursiveTableFoldSlot{expr: runtimeFoldCall(field)}
		case vm.OP_ADD:
			if !runtimeFoldSlotOK(a) || !runtimeFoldSlotOK(b) || !runtimeFoldSlotOK(c) {
				return runtimeRecursiveTableFoldExpr{}, nil, false
			}
			expr, ok := runtimeFoldAdd(slots[b].expr, slots[c].expr)
			if !ok {
				return runtimeRecursiveTableFoldExpr{}, nil, false
			}
			slots[a] = runtimeRecursiveTableFoldSlot{expr: expr}
		case vm.OP_RETURN:
			if !runtimeFoldSlotOK(a) || b != 2 {
				return runtimeRecursiveTableFoldExpr{}, nil, false
			}
			return slots[a].expr, children, true
		default:
			return runtimeRecursiveTableFoldExpr{}, nil, false
		}
	}
	return runtimeRecursiveTableFoldExpr{}, nil, false
}

func runtimeFoldSlotOK(slot int) bool {
	return slot >= 0 && slot < maxTrackedSlots
}

func runtimeFoldConst(v int64) runtimeRecursiveTableFoldExpr {
	return runtimeRecursiveTableFoldExpr{constant: v, calls: make(map[string]int), valid: true}
}

func runtimeFoldCall(field string) runtimeRecursiveTableFoldExpr {
	return runtimeRecursiveTableFoldExpr{calls: map[string]int{field: 1}, valid: true}
}

func runtimeFoldAdd(a, b runtimeRecursiveTableFoldExpr) (runtimeRecursiveTableFoldExpr, bool) {
	if !a.valid || !b.valid {
		return runtimeRecursiveTableFoldExpr{}, false
	}
	constant, ok := runtimeFoldCheckedAdd(a.constant, b.constant)
	if !ok {
		return runtimeRecursiveTableFoldExpr{}, false
	}
	out := runtimeRecursiveTableFoldExpr{
		constant: constant,
		calls:    make(map[string]int, len(a.calls)+len(b.calls)),
		valid:    true,
	}
	for field, count := range a.calls {
		out.calls[field] += count
	}
	for field, count := range b.calls {
		out.calls[field] += count
	}
	return out, true
}

func runtimeFoldCheckedAdd(a, b int64) (int64, bool) {
	if (b > 0 && a > runtimeFoldMaxInt64-b) || (b < 0 && a < runtimeFoldMinInt64-b) {
		return 0, false
	}
	out := a + b
	if out < runtimeFoldMinInt48 || out > runtimeFoldMaxInt48 {
		return 0, false
	}
	return out, true
}

func newRuntimeRecursiveTableFoldCompiled(proto *vm.FuncProto) (*CompiledFunction, bool) {
	specialization, ok := analyzeRuntimeRecursiveTableFold(proto)
	if !ok {
		return nil, false
	}
	return &CompiledFunction{
		Proto:                     proto,
		numRegs:                   proto.MaxStack,
		RuntimeRecursiveTableFold: specialization,
	}, true
}

func (tm *TieringManager) compileRuntimeRecursiveTableFoldTier2(proto *vm.FuncProto) (*CompiledFunction, bool) {
	cf, ok := newRuntimeRecursiveTableFoldCompiled(proto)
	if !ok {
		return nil, false
	}
	tm.tier2Attempts++
	attempt := tm.tier2Attempts
	tm.traceEvent("tier2_attempt", "tier2", proto, map[string]any{
		"attempt":        attempt,
		"call_count":     proto.CallCount,
		"specialization": "lazy_recursive_table_fold",
	})
	tm.traceTier2Success(proto, cf, attempt)
	return cf, true
}

func (tm *TieringManager) executeRuntimeRecursiveTableFold(cf *CompiledFunction, regs []runtime.Value, base int, proto *vm.FuncProto, retBuf []runtime.Value) ([]runtime.Value, error) {
	if cf == nil || cf.RuntimeRecursiveTableFold == nil || proto == nil {
		return nil, fmt.Errorf("tier2: missing runtime recursive table fold specialization")
	}
	if base < 0 || base >= len(regs) {
		return nil, fmt.Errorf("tier2: runtime recursive table fold base %d outside regs len %d", base, len(regs))
	}
	if !tm.runtimeRecursiveSelfGlobalMatches(proto) {
		tm.disableTier2AfterRuntimeDeopt(proto, "tier2: runtime recursive table fold self global changed")
		return nil, fmt.Errorf("tier2: runtime recursive table fold self global changed")
	}
	proto.EnteredTier2 = 1
	n, ok := cf.RuntimeRecursiveTableFold.fold(regs[base])
	if !ok {
		tm.disableTier2AfterRuntimeDeopt(proto, "tier2: runtime recursive table fold fallback")
		return nil, fmt.Errorf("tier2: runtime recursive table fold fallback")
	}
	result := runtime.IntValue(n)
	regs[base] = result
	return runtime.ReuseValueSlice1(retBuf, result), nil
}

func (tm *TieringManager) runtimeRecursiveSelfGlobalMatches(proto *vm.FuncProto) bool {
	if tm == nil || tm.callVM == nil || proto == nil || proto.Name == "" {
		return false
	}
	cl, ok := vmClosureFromValue(tm.callVM.GetGlobal(proto.Name))
	return ok && cl != nil && cl.Proto == proto
}

func (p *runtimeRecursiveTableFoldSpecialization) fold(v runtime.Value) (int64, bool) {
	t := v.Table()
	if t == nil {
		return 0, false
	}
	if n, ok := p.foldLazy(t); ok {
		return n, true
	}
	nilValue := t.RawGetStringCached(p.nilField, &p.nilCache)
	if nilValue.IsNil() {
		return p.baseValue, true
	}
	total := p.combineBias
	for i := range p.children {
		childValue := t.RawGetStringCached(p.children[i].field, &p.children[i].cache)
		childTotal, ok := p.fold(childValue)
		if !ok {
			return 0, false
		}
		total, ok = runtimeFoldCheckedAdd(total, childTotal)
		if !ok {
			return 0, false
		}
	}
	return total, true
}

func (p *runtimeRecursiveTableFoldSpecialization) foldLazy(t *runtime.Table) (int64, bool) {
	depth, key1, key2, ok := t.LazyRecursiveTablePureInfo()
	if !ok || depth < 0 || p.nilField != key1 || len(p.children) != 2 {
		return 0, false
	}
	if p.children[0].field != key1 || p.children[1].field != key2 {
		return 0, false
	}
	total := p.baseValue
	for i := int64(0); i < depth; i++ {
		next := p.combineBias
		var ok bool
		next, ok = runtimeFoldCheckedAdd(next, total)
		if !ok {
			return 0, false
		}
		next, ok = runtimeFoldCheckedAdd(next, total)
		if !ok {
			return 0, false
		}
		total = next
	}
	return total, true
}
