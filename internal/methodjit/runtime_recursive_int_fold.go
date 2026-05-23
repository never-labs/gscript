//go:build darwin && arm64

package methodjit

import (
	"fmt"
	"sort"

	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

const maxRuntimeRecursiveIntFoldIterations = 1_000_000

type runtimeRecursiveIntFoldProtocol struct {
	threshold int64
	bias      int64
	terms     []runtimeRecursiveIntFoldTerm
}

type runtimeRecursiveIntFoldTerm struct {
	decrement int64
	count     int
}

type runtimeRecursiveIntFoldExpr struct {
	constant int64
	calls    map[int64]int
	valid    bool
}

type runtimeRecursiveIntFoldSlotKind uint8

const (
	runtimeIntSlotUnknown runtimeRecursiveIntFoldSlotKind = iota
	runtimeIntSlotParam
	runtimeIntSlotConst
	runtimeIntSlotSelfFunc
	runtimeIntSlotArg
	runtimeIntSlotExpr
)

type runtimeRecursiveIntFoldSlot struct {
	kind      runtimeRecursiveIntFoldSlotKind
	constant  int64
	decrement int64
	expr      runtimeRecursiveIntFoldExpr
}

func qualifiesForRuntimeRecursiveIntFold(proto *vm.FuncProto) bool {
	_, ok := analyzeRuntimeRecursiveIntFold(proto)
	return ok
}

func analyzeRuntimeRecursiveIntFold(proto *vm.FuncProto) (*runtimeRecursiveIntFoldProtocol, bool) {
	if proto == nil || proto.IsVarArg || proto.NumParams != 1 || proto.Name == "" {
		return nil, false
	}
	if len(proto.Upvalues) != 0 || len(proto.Protos) != 0 || len(proto.Code) < 8 {
		return nil, false
	}

	threshold, recursePC, ok := runtimeIntParseIdentityBaseHeader(proto)
	if !ok {
		return nil, false
	}
	expr, ok := runtimeIntParseRecursiveExpr(proto, recursePC)
	if !ok || !expr.valid || len(expr.calls) == 0 {
		return nil, false
	}

	decrements := make([]int64, 0, len(expr.calls))
	for decrement := range expr.calls {
		if decrement <= 0 {
			return nil, false
		}
		decrements = append(decrements, decrement)
	}
	sort.Slice(decrements, func(i, j int) bool { return decrements[i] < decrements[j] })

	terms := make([]runtimeRecursiveIntFoldTerm, 0, len(decrements))
	for _, decrement := range decrements {
		terms = append(terms, runtimeRecursiveIntFoldTerm{
			decrement: decrement,
			count:     expr.calls[decrement],
		})
	}
	return &runtimeRecursiveIntFoldProtocol{
		threshold: threshold,
		bias:      expr.constant,
		terms:     terms,
	}, true
}

func runtimeIntParseIdentityBaseHeader(proto *vm.FuncProto) (threshold int64, recursePC int, ok bool) {
	code := proto.Code
	if len(code) < 5 || vm.DecodeOp(code[0]) != vm.OP_LOADINT {
		return 0, 0, false
	}
	thresholdSlot := vm.DecodeA(code[0])
	threshold = int64(vm.DecodesBx(code[0]))

	if vm.DecodeOp(code[1]) != vm.OP_LT || vm.DecodeA(code[1]) != 0 ||
		vm.DecodeB(code[1]) != 0 || vm.DecodeC(code[1]) != thresholdSlot {
		return 0, 0, false
	}
	if vm.DecodeOp(code[2]) != vm.OP_JMP {
		return 0, 0, false
	}
	recursePC = 3 + vm.DecodesBx(code[2])
	if recursePC <= 4 || recursePC >= len(code) {
		return 0, 0, false
	}

	switch vm.DecodeOp(code[3]) {
	case vm.OP_MOVE:
		baseSlot := vm.DecodeA(code[3])
		if vm.DecodeB(code[3]) != 0 || vm.DecodeOp(code[4]) != vm.OP_RETURN ||
			vm.DecodeA(code[4]) != baseSlot || vm.DecodeB(code[4]) != 2 {
			return 0, 0, false
		}
	case vm.OP_RETURN:
		if vm.DecodeA(code[3]) != 0 || vm.DecodeB(code[3]) != 2 {
			return 0, 0, false
		}
	default:
		return 0, 0, false
	}
	return threshold, recursePC, true
}

func runtimeIntParseRecursiveExpr(proto *vm.FuncProto, startPC int) (runtimeRecursiveIntFoldExpr, bool) {
	slots := make([]runtimeRecursiveIntFoldSlot, maxTrackedSlots)
	slots[0] = runtimeRecursiveIntFoldSlot{kind: runtimeIntSlotParam}
	for pc := startPC; pc < len(proto.Code); pc++ {
		inst := proto.Code[pc]
		a, b, c := vm.DecodeA(inst), vm.DecodeB(inst), vm.DecodeC(inst)
		switch vm.DecodeOp(inst) {
		case vm.OP_LOADINT:
			if !runtimeFoldSlotOK(a) {
				return runtimeRecursiveIntFoldExpr{}, false
			}
			v := int64(vm.DecodesBx(inst))
			slots[a] = runtimeRecursiveIntFoldSlot{
				kind:     runtimeIntSlotConst,
				constant: v,
				expr:     runtimeIntExprConst(v),
			}
		case vm.OP_MOVE:
			if !runtimeFoldSlotOK(a) || !runtimeFoldSlotOK(b) {
				return runtimeRecursiveIntFoldExpr{}, false
			}
			slots[a] = slots[b]
		case vm.OP_GETGLOBAL:
			if !runtimeFoldSlotOK(a) || protoConstString(proto, vm.DecodeBx(inst)) != proto.Name {
				return runtimeRecursiveIntFoldExpr{}, false
			}
			slots[a] = runtimeRecursiveIntFoldSlot{kind: runtimeIntSlotSelfFunc}
		case vm.OP_SUB:
			if !runtimeFoldSlotOK(a) || !runtimeFoldSlotOK(b) || !runtimeFoldSlotOK(c) {
				return runtimeRecursiveIntFoldExpr{}, false
			}
			if slots[b].kind != runtimeIntSlotParam || slots[c].kind != runtimeIntSlotConst {
				return runtimeRecursiveIntFoldExpr{}, false
			}
			slots[a] = runtimeRecursiveIntFoldSlot{
				kind:      runtimeIntSlotArg,
				decrement: slots[c].constant,
			}
		case vm.OP_CALL:
			if !runtimeFoldSlotOK(a) || !slots[a].isFixedIntSelfFunc() || b != 2 || c != 2 || !runtimeFoldSlotOK(a+1) {
				return runtimeRecursiveIntFoldExpr{}, false
			}
			arg := slots[a+1]
			if arg.kind != runtimeIntSlotArg || arg.decrement <= 0 {
				return runtimeRecursiveIntFoldExpr{}, false
			}
			slots[a] = runtimeRecursiveIntFoldSlot{
				kind: runtimeIntSlotExpr,
				expr: runtimeIntExprCall(arg.decrement),
			}
		case vm.OP_ADD:
			if !runtimeFoldSlotOK(a) || !runtimeFoldSlotOK(b) || !runtimeFoldSlotOK(c) {
				return runtimeRecursiveIntFoldExpr{}, false
			}
			expr, ok := runtimeIntExprAdd(slots[b].expr, slots[c].expr)
			if !ok {
				return runtimeRecursiveIntFoldExpr{}, false
			}
			slots[a] = runtimeRecursiveIntFoldSlot{kind: runtimeIntSlotExpr, expr: expr}
		case vm.OP_RETURN:
			if !runtimeFoldSlotOK(a) || b != 2 {
				return runtimeRecursiveIntFoldExpr{}, false
			}
			return slots[a].expr, true
		default:
			return runtimeRecursiveIntFoldExpr{}, false
		}
	}
	return runtimeRecursiveIntFoldExpr{}, false
}

func (s runtimeRecursiveIntFoldSlot) isFixedIntSelfFunc() bool {
	return s.kind == runtimeIntSlotSelfFunc
}

func runtimeIntExprConst(v int64) runtimeRecursiveIntFoldExpr {
	return runtimeRecursiveIntFoldExpr{constant: v, calls: make(map[int64]int), valid: true}
}

func runtimeIntExprCall(decrement int64) runtimeRecursiveIntFoldExpr {
	return runtimeRecursiveIntFoldExpr{calls: map[int64]int{decrement: 1}, valid: true}
}

func runtimeIntExprAdd(a, b runtimeRecursiveIntFoldExpr) (runtimeRecursiveIntFoldExpr, bool) {
	if !a.valid || !b.valid {
		return runtimeRecursiveIntFoldExpr{}, false
	}
	constant, ok := runtimeFoldCheckedAdd(a.constant, b.constant)
	if !ok {
		return runtimeRecursiveIntFoldExpr{}, false
	}
	out := runtimeRecursiveIntFoldExpr{
		constant: constant,
		calls:    make(map[int64]int, len(a.calls)+len(b.calls)),
		valid:    true,
	}
	for decrement, count := range a.calls {
		out.calls[decrement] += count
	}
	for decrement, count := range b.calls {
		out.calls[decrement] += count
	}
	return out, true
}

func newRuntimeRecursiveIntFoldCompiled(proto *vm.FuncProto) (*CompiledFunction, bool) {
	protocol, ok := analyzeRuntimeRecursiveIntFold(proto)
	if !ok {
		return nil, false
	}
	return &CompiledFunction{
		Proto:                   proto,
		numRegs:                 proto.MaxStack,
		RuntimeRecursiveIntFold: protocol,
	}, true
}

func (tm *TieringManager) compileRuntimeRecursiveIntFoldTier2(proto *vm.FuncProto) (*CompiledFunction, bool) {
	cf, ok := newRuntimeRecursiveIntFoldCompiled(proto)
	if !ok {
		return nil, false
	}
	tm.tier2Attempts++
	attempt := tm.tier2Attempts
	tm.traceEvent("tier2_attempt", "tier2", proto, map[string]any{
		"attempt":    attempt,
		"call_count": proto.CallCount,
		"protocol":   "recursive_int_fold",
	})
	tm.traceTier2Success(proto, cf, attempt)
	return cf, true
}

func (tm *TieringManager) executeRuntimeRecursiveIntFold(cf *CompiledFunction, regs []runtime.Value, base int, proto *vm.FuncProto, retBuf []runtime.Value) ([]runtime.Value, error) {
	if cf == nil || cf.RuntimeRecursiveIntFold == nil || proto == nil {
		return nil, fmt.Errorf("tier2: missing runtime recursive int fold protocol")
	}
	if base < 0 || base >= len(regs) {
		return nil, fmt.Errorf("tier2: runtime recursive int fold base %d outside regs len %d", base, len(regs))
	}
	if !tm.runtimeRecursiveSelfGlobalMatches(proto) {
		tm.disableTier2AfterRuntimeDeopt(proto, "tier2: runtime recursive int fold self global changed")
		return nil, fmt.Errorf("tier2: runtime recursive int fold self global changed")
	}
	proto.EnteredTier2 = 1
	n, ok := cf.RuntimeRecursiveIntFold.fold(regs[base])
	if !ok {
		tm.disableTier2AfterRuntimeDeopt(proto, "tier2: runtime recursive int fold fallback")
		return nil, fmt.Errorf("tier2: runtime recursive int fold fallback")
	}
	result := runtime.IntValue(n)
	regs[base] = result
	return runtime.ReuseValueSlice1(retBuf, result), nil
}

func (p *runtimeRecursiveIntFoldProtocol) fold(v runtime.Value) (int64, bool) {
	if p == nil || !v.IsInt() {
		return 0, false
	}
	n := v.Int()
	if n < p.threshold {
		return n, true
	}
	iterations := n - p.threshold + 1
	if iterations < 0 || iterations > maxRuntimeRecursiveIntFoldIterations {
		return 0, false
	}
	values := make([]int64, int(iterations))
	for k := p.threshold; k <= n; k++ {
		total := p.bias
		for _, term := range p.terms {
			child := k - term.decrement
			childValue := child
			if child >= p.threshold {
				childValue = values[child-p.threshold]
			}
			for i := 0; i < term.count; i++ {
				var ok bool
				total, ok = runtimeFoldCheckedAdd(total, childValue)
				if !ok {
					return 0, false
				}
			}
		}
		values[k-p.threshold] = total
	}
	return values[len(values)-1], true
}
