package methodjit

import "github.com/gscript/gscript/internal/vm"

const callResultRangeGuardMinCount uint32 = 4
const observedParamRangeGuardMinCount uint32 = 2
const callFloorSpecRangeMin int64 = -1 << 31
const callFloorSpecRangeMax int64 = 1<<31 - 1

// CallResultRangeGuardPass turns mature call-result range feedback into an
// explicit GuardIntRange. For floor-projected calls with stable callee facts
// but no mature result range yet, it may add a conservative int32 guard so
// first-entry Tier 2 can still specialize while guard/deopt preserves semantics.
// RangeAnalysis can then consume the guarded value without trusting profile
// data unconditionally.
func CallResultRangeGuardPass(fn *Function) (*Function, error) {
	if fn == nil || fn.Analysis == nil {
		return callResultRangeGuardPass(fn, nil, nil, nil, nil)
	}
	return callResultRangeGuardPass(
		fn,
		fn.Analysis.SpeculationFacts(),
		fn.Analysis.CallFacts(),
		fn.Analysis.TableShapeFacts(),
		fn.Analysis.GlobalFacts(),
	)
}

func CallResultRangeGuardPassCtx(ctx *PassContext) (*Function, error) {
	return callResultRangeGuardPass(ctx.Func(), ctx.Speculation(), ctx.Call(), ctx.TableShape(), ctx.Global())
}

func callResultRangeGuardPass(fn *Function, spec *SpeculationFacts, callFacts *CallFacts, tableShapes *TableShapeFacts, globals *GlobalFacts) (*Function, error) {
	if fn == nil || fn.Proto == nil || fn.Proto.CallSiteFeedback == nil {
		return fn, nil
	}
	fn.ensureAnalysis()
	uses := computeUseCounts(fn)
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for i := 0; i < len(block.Instrs); i++ {
			instr := block.Instrs[i]
			if instr == nil || !callResultRangeGuardCandidate(instr) {
				continue
			}
			if !instr.HasSource || instr.SourcePC < 0 || instr.SourcePC >= len(fn.Proto.CallSiteFeedback) {
				continue
			}
			fb := fn.Proto.CallSiteFeedback[instr.SourcePC]
			if instr.Type != TypeInt {
				if _, _, ok := stableCallResultRange(fb); !ok && !callSpeculativeIntUseRangeCandidate(fn, instr, fb, uses, globals) {
					continue
				}
			}
			if specGuardKindSuppressedFacts(spec, instr.SourcePC, "GuardIntRange") {
				functionRemarks(fn).Add("CallResultRangeGuard", "missed", block.ID, instr.ID, instr.Op,
					"skipped suppressed int-range guard")
				continue
			}
			if callFloorResultModuloReduced(fn, instr, uses) {
				functionRemarks(fn).Add("CallResultRangeGuard", "missed", block.ID, instr.ID, instr.Op,
					"skipped int-range guard for modulo-reduced floor-call result")
				continue
			}
			min, max, reason, ok := callResultGuardRange(fn, instr, fb, uses, callFacts, tableShapes, globals)
			if !ok || nextInstrIsSameIntRangeGuard(block, i, instr.ID, min, max) {
				continue
			}
			guard := newCallResultRangeGuard(fn, block, instr, min, max)
			block.Instrs = append(block.Instrs[:i+1], append([]*Instr{guard}, block.Instrs[i+1:]...)...)
			replaceValueUses(fn, instr.ID, guard.Value(), guard.ID)
			i++
			functionRemarks(fn).Add("CallResultRangeGuard", "changed", block.ID, guard.ID, guard.Op,
				reason)
		}
	}
	return fn, nil
}

func callFloorResultModuloReduced(fn *Function, instr *Instr, uses map[int]int) bool {
	if fn == nil || instr == nil || uses[instr.ID] == 0 {
		return false
	}
	switch instr.Op {
	case OpCallFloor, OpFieldCallFloor:
	default:
		return false
	}
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, user := range block.Instrs {
			if user == nil || user.Op != OpModInt || len(user.Args) < 2 || !positiveIntModDivisorValue(user.Args[1]) {
				continue
			}
			if user.Args[0] != nil && user.Args[0].ID == instr.ID {
				return uses[instr.ID] == 1
			}
		}
	}
	return false
}

func callResultRangeGuardCandidate(instr *Instr) bool {
	switch instr.Op {
	case OpCall, OpCallFloor, OpFieldCallFloor:
		return true
	default:
		return false
	}
}

func stableCallResultRange(fb vm.CallSiteFeedback) (int64, int64, bool) {
	if fb.Count < callResultRangeGuardMinCount || fb.ResultRange.Count < callResultRangeGuardMinCount ||
		fb.Flags&vm.CallSiteArityPolymorphic != 0 {
		return 0, 0, false
	}
	return fb.ResultRange.StableRange()
}

func callResultGuardRange(fn *Function, instr *Instr, fb vm.CallSiteFeedback, uses map[int]int, callFacts *CallFacts, tableShapes *TableShapeFacts, globals *GlobalFacts) (int64, int64, string, bool) {
	if min, max, ok := stableCallResultRange(fb); ok {
		return min, max, "guarded profiled call result range", true
	}
	if callFloorSpeculativeNarrowRangeCandidate(instr, fb, callFacts, tableShapes) {
		return callFloorSpecRangeMin, callFloorSpecRangeMax, "guarded speculative floor-call int32 result range", true
	}
	if callSpeculativeIntUseRangeCandidate(fn, instr, fb, uses, globals) {
		return callFloorSpecRangeMin, callFloorSpecRangeMax, "guarded speculative call int32 result for integer use", true
	}
	return 0, 0, "", false
}

func callFloorSpeculativeNarrowRangeCandidate(instr *Instr, fb vm.CallSiteFeedback, callFacts *CallFacts, tableShapes *TableShapeFacts) bool {
	if instr == nil || instr.Type != TypeInt || fb.Flags&vm.CallSiteArityPolymorphic != 0 {
		return false
	}
	switch instr.Op {
	case OpCallFloor:
		if callFacts != nil {
			if desc, ok := callFacts.CallABI(instr.ID); ok && desc.ReturnRep != SpecializedABIReturnNone {
				return true
			}
		}
		_, _, nativeOK := fb.StableCalleeNativeIdentity()
		_, vmOK := fb.StableCalleeVMProto()
		return nativeOK || vmOK
	case OpFieldCallFloor:
		return tableShapes != nil && tableShapes.HasFieldPolyShapeCases(instr.ID)
	default:
		return false
	}
}

func callSpeculativeIntUseRangeCandidate(fn *Function, instr *Instr, fb vm.CallSiteFeedback, uses map[int]int, globals *GlobalFacts) bool {
	if fn == nil || instr == nil || instr.Op != OpCall || instr.Type == TypeInt ||
		fb.Flags&vm.CallSiteArityPolymorphic != 0 || uses[instr.ID] == 0 {
		return false
	}
	if !callResultHasStableCallee(fn, instr, fb, globals) {
		return false
	}
	return callResultHasIntegerUse(fn, instr.ID)
}

func callResultHasStableCallee(fn *Function, instr *Instr, fb vm.CallSiteFeedback, globals *GlobalFacts) bool {
	if _, _, ok := fb.StableCalleeNativeIdentity(); ok {
		return true
	}
	if _, ok := fb.StableCalleeVMProto(); ok {
		return true
	}
	var globalProtos map[string]*vm.FuncProto
	if globals != nil {
		globalProtos = globals.GlobalsMap()
	}
	if _, callee := resolveCallee(instr, fn, InlineConfig{Globals: globalProtos}); callee != nil {
		return true
	}
	return false
}

func callResultHasIntegerUse(fn *Function, valueID int) bool {
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, user := range block.Instrs {
			if user == nil {
				continue
			}
			switch user.Op {
			case OpAdd, OpSub, OpMul, OpMod, OpLt, OpLe:
			default:
				continue
			}
			for _, arg := range user.Args {
				if arg == nil || arg.ID != valueID {
					continue
				}
				if genericNumericUseHasIntPeer(user, valueID) {
					return true
				}
			}
		}
	}
	return false
}

func genericNumericUseHasIntPeer(user *Instr, valueID int) bool {
	for _, arg := range user.Args {
		if arg == nil || arg.ID == valueID || arg.Def == nil {
			continue
		}
		if arg.Def.Type == TypeInt || arg.Def.Op == OpConstInt || arg.Def.Op == OpGuardIntRange {
			return true
		}
	}
	return false
}

func newCallResultRangeGuard(fn *Function, block *Block, instr *Instr, min, max int64) *Instr {
	return &Instr{
		ID:          fn.newValueID(),
		Op:          OpGuardIntRange,
		Type:        TypeInt,
		Args:        []*Value{instr.Value()},
		Aux:         min,
		Aux2:        max,
		Block:       block,
		HasSource:   instr.HasSource,
		SourceProto: instr.SourceProto,
		SourcePC:    instr.SourcePC,
		SourceLine:  instr.SourceLine,
	}
}

func nextInstrIsSameIntRangeGuard(block *Block, idx int, valueID int, min, max int64) bool {
	if block == nil || idx+1 >= len(block.Instrs) {
		return false
	}
	next := block.Instrs[idx+1]
	return next != nil && next.Op == OpGuardIntRange && len(next.Args) == 1 &&
		next.Args[0] != nil && next.Args[0].ID == valueID &&
		next.Aux == min && next.Aux2 == max
}
