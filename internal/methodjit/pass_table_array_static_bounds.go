package methodjit

// TableArrayStaticBoundsPass marks typed array loads as bounds-safe when the
// table comes from a dominating SetList construction and RangeAnalysis proves
// the key stays inside the constructed array length.
func TableArrayStaticBoundsPass(fn *Function) (*Function, error) {
	if fn == nil {
		return fn, nil
	}
	fn.ensureAnalysis()
	return tableArrayStaticBoundsPass(fn, functionNumericFacts(fn), functionLoopSpecializationFacts(fn))
}

func TableArrayStaticBoundsPassCtx(ctx *PassContext) (*Function, error) {
	return tableArrayStaticBoundsPass(ctx.Func(), ctx.Numeric(), ctx.LoopSpecialization())
}

func tableArrayStaticBoundsPass(fn *Function, numeric *NumericFacts, loopFacts *LoopSpecializationFacts) (*Function, error) {
	if fn == nil || len(fn.Blocks) == 0 {
		return fn, nil
	}
	fn.ensureAnalysis()
	facts := collectStaticTableLenFacts(fn)
	dom := computeDominators(fn)
	li := computeLoopInfo(fn)
	order := blockInstructionOrder(fn)
	lenGuards := collectDominatingTableArrayLenLowerGuards(fn)
	keyUpperGuards := collectDominatingKeyUpperGuards(fn)
	headers := make(map[int]int)
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpTableArrayHeader || len(instr.Args) < 1 || instr.Args[0] == nil {
				continue
			}
			headers[instr.ID] = instr.Args[0].ID
		}
	}
	if len(headers) == 0 && len(lenGuards) == 0 {
		return fn, nil
	}

	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			lenValue, keyValue, ok := tableArrayStaticBoundsAccessOperands(instr)
			if !ok || lenValue == nil || keyValue == nil || lenValue.Def == nil {
				continue
			}
			keyNonNegative, keyMax, keyMaxKnown := tableArrayStaticKeyBounds(numeric, li, keyValue, keyUpperGuards, dom, block.ID)
			lenInstr := lenValue.Def
			if lenInstr.Op != OpTableArrayLen || len(lenInstr.Args) < 1 || lenInstr.Args[0] == nil {
				continue
			}

			if tableID, ok := headers[lenInstr.Args[0].ID]; ok {
				if fact, ok := staticTableLenFactForLen(facts[tableID], dom, order, block.ID, instr.ID); ok && fact.length >= 0 {
					if keyNonNegative {
						markTableArrayLowerBoundSafe(loopFacts, instr)
					}
					if keyNonNegative && keyMaxKnown && keyMax <= fact.length {
						markTableArrayUpperBoundSafe(loopFacts, instr)
						functionRemarks(fn).Add("TableArrayStaticBounds", "changed", block.ID, instr.ID, instr.Op,
							"static SetList length and key range prove table-array bounds")
						continue
					}
				}
			}
			if lenMax, ok := profiledTableArrayLenMax(numeric, lenInstr); ok && keyNonNegative {
				markTableArrayLowerBoundSafe(loopFacts, instr)
				if keyMaxKnown && keyMax <= lenMax {
					markTableArrayUpperBoundSafe(loopFacts, instr)
					functionRemarks(fn).Add("TableArrayStaticBounds", "changed", block.ID, instr.ID, instr.Op,
						"profiled table-array length and key range prove table-array bounds")
					continue
				}
			}
			if lenMax, ok := profiledTableAccessLenMax(fn, instr); ok && keyNonNegative {
				markTableArrayLowerBoundSafe(loopFacts, instr)
				if keyMaxKnown && keyMax <= lenMax {
					markTableArrayUpperBoundSafe(loopFacts, instr)
					functionRemarks(fn).Add("TableArrayStaticBounds", "changed", block.ID, instr.ID, instr.Op,
						"profiled table access length and key range prove table-array bounds")
					continue
				}
			}

			if maxSafe, ok := dominatingTableArrayLenGuardMaxSafe(lenGuards[lenInstr.ID], dom, order, block.ID, instr.ID); ok {
				if keyNonNegative {
					markTableArrayLowerBoundSafe(loopFacts, instr)
				}
				if keyMaxKnown && keyMax <= maxSafe {
					markTableArrayUpperBoundSafe(loopFacts, instr)
					functionRemarks(fn).Add("TableArrayStaticBounds", "changed", block.ID, instr.ID, instr.Op,
						"dominating array-len guard and key range prove table-array bounds")
				}
			}
		}
	}
	return fn, nil
}

func tableArrayStaticBoundsAccessOperands(instr *Instr) (*Value, *Value, bool) {
	if instr == nil {
		return nil, nil, false
	}
	layout, ok := tableArrayAccessLayoutForOp(instr.Op)
	if !ok || layout.LenArg < 0 || layout.KeyArg < 0 {
		return nil, nil, false
	}
	if len(instr.Args) <= layout.LenArg || len(instr.Args) <= layout.KeyArg {
		return nil, nil, false
	}
	return instr.Args[layout.LenArg], instr.Args[layout.KeyArg], true
}

type tableArrayLenGuardFact struct {
	blockID int
	instrID int
	maxSafe int64
}

type keyUpperGuardFact struct {
	trueBlockID int
	max         int64
}

func tableArrayStaticKeyBounds(numeric *NumericFacts, li *loopInfo, key *Value, guards map[int][]keyUpperGuardFact, dom *domInfo, blockID int) (bool, int64, bool) {
	if key == nil {
		return false, 0, false
	}
	nonNegative := false
	var max int64
	maxKnown := false
	if numeric != nil {
		if r, ok := numeric.IntRange(key.ID); ok && r.known {
			nonNegative = r.min >= 0
			max = r.max
			maxKnown = true
		}
	}
	if c, ok := constIntFromValue(key); ok {
		nonNegative = c >= 0
		if !maxKnown || c < max {
			max = c
			maxKnown = true
		}
	}
	if numeric != nil && numeric.IsIntNonNegative(key.ID) {
		nonNegative = true
	}
	if tableArrayKeyNonNegativeFromInduction(li, key) {
		nonNegative = true
	}
	if guardMax, ok := dominatingKeyUpperGuardMax(guards[key.ID], dom, blockID); ok {
		if !maxKnown || guardMax < max {
			max = guardMax
			maxKnown = true
		}
	}
	return nonNegative, max, maxKnown
}

func profiledTableArrayLenMax(numeric *NumericFacts, lenInstr *Instr) (int64, bool) {
	if numeric == nil || lenInstr == nil || len(lenInstr.Args) < 1 || lenInstr.Args[0] == nil {
		return 0, false
	}
	if r, ok := numeric.IntRange(lenInstr.ID); ok && r.known && r.max >= 0 {
		return r.max, true
	}
	if r, ok := numeric.ProfiledLenRange(lenInstr.Args[0].ID); ok && r.known && r.max >= 0 {
		return r.max, true
	}
	if table := tableArrayHeaderSourceTableValue(lenInstr.Args[0]); table != nil {
		if r, ok := numeric.ProfiledLenRange(table.ID); ok && r.known && r.max >= 0 {
			return r.max, true
		}
	}
	return 0, false
}

func profiledTableAccessLenMax(fn *Function, instr *Instr) (int64, bool) {
	proto := instrSourceProto(fn, instr)
	if proto == nil || instr == nil || !instr.HasSource || instr.SourcePC < 0 || instr.SourcePC >= len(proto.TableKeyFeedback) {
		return 0, false
	}
	rf := proto.TableKeyFeedback[instr.SourcePC].TableLenRange
	min, max, ok := rf.StableRange()
	if !ok || min < 0 || max < 0 {
		return 0, false
	}
	return max, true
}

func tableArrayHeaderSourceTableValue(header *Value) *Value {
	if header == nil || header.Def == nil || header.Def.Op != OpTableArrayHeader || len(header.Def.Args) < 1 {
		return nil
	}
	return header.Def.Args[0]
}

func tableArrayKeyNonNegativeFromInduction(li *loopInfo, key *Value) bool {
	if li == nil || key == nil || key.Def == nil || !key.Def.Type.isIntegerLike() {
		return false
	}
	if step, phi := tableArrayForwardStepWithPhi(key.Def); step >= 0 && phi != nil && phi.Block != nil {
		if ind, ok := analyzeForwardInduction(phi, li); ok && ind.step >= 0 && ind.init.min >= 0 {
			return true
		}
	}
	return false
}

func tableArrayForwardStepWithPhi(instr *Instr) (int64, *Instr) {
	if instr == nil || len(instr.Args) < 2 {
		return 0, nil
	}
	for _, arg := range instr.Args {
		if arg == nil || arg.Def == nil || arg.Def.Op != OpPhi {
			continue
		}
		step, ok := forwardStepFromPhi(instr, arg.ID)
		if !ok {
			continue
		}
		return step, arg.Def
	}
	return 0, nil
}

func collectDominatingTableArrayLenLowerGuards(fn *Function) map[int][]tableArrayLenGuardFact {
	out := make(map[int][]tableArrayLenGuardFact)
	if fn == nil {
		return out
	}
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpGuardTruthy || len(instr.Args) < 1 || instr.Args[0] == nil || instr.Args[0].Def == nil {
				continue
			}
			cond := instr.Args[0].Def
			lenID, maxSafe, ok := tableArrayLenLowerGuard(cond)
			if !ok {
				continue
			}
			out[lenID] = append(out[lenID], tableArrayLenGuardFact{
				blockID: block.ID,
				instrID: instr.ID,
				maxSafe: maxSafe,
			})
		}
	}
	return out
}

func tableArrayLenLowerGuard(cond *Instr) (int, int64, bool) {
	if cond == nil || len(cond.Args) < 2 || cond.Args[0] == nil || cond.Args[1] == nil {
		return 0, 0, false
	}
	lhsConst, lhsIsConst := constIntFromValue(cond.Args[0])
	rhsConst, rhsIsConst := constIntFromValue(cond.Args[1])
	lhsLen := tableArrayLenValueID(cond.Args[0])
	rhsLen := tableArrayLenValueID(cond.Args[1])
	strict, ok := orderedRangeRefineKind(cond.Op)
	if !ok {
		return 0, 0, false
	}
	if strict {
		if lhsIsConst && rhsLen != 0 {
			return rhsLen, lhsConst, true
		}
		if rhsIsConst && lhsLen != 0 {
			return lhsLen, satSub(rhsConst, 1), true
		}
		return 0, 0, false
	}
	if lhsIsConst && rhsLen != 0 {
		return rhsLen, satSub(lhsConst, 1), true
	}
	if rhsIsConst && lhsLen != 0 {
		return lhsLen, rhsConst, true
	}
	return 0, 0, false
}

func tableArrayLenValueID(v *Value) int {
	if v == nil || v.Def == nil || v.Def.Op != OpTableArrayLen {
		return 0
	}
	return v.ID
}

func collectDominatingKeyUpperGuards(fn *Function) map[int][]keyUpperGuardFact {
	out := make(map[int][]keyUpperGuardFact)
	if fn == nil {
		return out
	}
	for _, block := range fn.Blocks {
		if block == nil || len(block.Instrs) == 0 {
			continue
		}
		term := block.Instrs[len(block.Instrs)-1]
		if term == nil || term.Op != OpBranch || len(term.Args) < 1 || term.Args[0] == nil || term.Args[0].Def == nil {
			continue
		}
		trueBlockID, ok := branchTrueBlockID(term)
		if !ok {
			continue
		}
		keyID, max, ok := keyUpperGuard(term.Args[0].Def)
		if !ok {
			continue
		}
		out[keyID] = append(out[keyID], keyUpperGuardFact{trueBlockID: trueBlockID, max: max})
	}
	return out
}

func branchTrueBlockID(branch *Instr) (int, bool) {
	if branch == nil || branch.Op != OpBranch {
		return 0, false
	}
	if branch.Aux != 0 {
		return int(branch.Aux), true
	}
	if branch.Block != nil && len(branch.Block.Succs) >= 1 && branch.Block.Succs[0] != nil {
		return branch.Block.Succs[0].ID, true
	}
	return 0, false
}

func keyUpperGuard(cond *Instr) (int, int64, bool) {
	if cond == nil || len(cond.Args) < 2 || cond.Args[0] == nil || cond.Args[1] == nil {
		return 0, 0, false
	}
	lhsConst, lhsIsConst := constIntFromValue(cond.Args[0])
	rhsConst, rhsIsConst := constIntFromValue(cond.Args[1])
	strict, ok := orderedRangeRefineKind(cond.Op)
	if !ok {
		return 0, 0, false
	}
	if strict {
		if rhsIsConst && cond.Args[0].Def != nil && cond.Args[0].Def.Type.isIntegerLike() {
			return cond.Args[0].ID, satSub(rhsConst, 1), true
		}
		if lhsIsConst && cond.Args[1].Def != nil && cond.Args[1].Def.Type.isIntegerLike() {
			return cond.Args[1].ID, lhsConst, true
		}
		return 0, 0, false
	}
	if rhsIsConst && cond.Args[0].Def != nil && cond.Args[0].Def.Type.isIntegerLike() {
		return cond.Args[0].ID, rhsConst, true
	}
	if lhsIsConst && cond.Args[1].Def != nil && cond.Args[1].Def.Type.isIntegerLike() {
		return cond.Args[1].ID, satSub(lhsConst, 1), true
	}
	return 0, 0, false
}

func dominatingKeyUpperGuardMax(facts []keyUpperGuardFact, dom *domInfo, blockID int) (int64, bool) {
	var best int64
	ok := false
	for _, fact := range facts {
		if dom == nil || !dom.dominates(fact.trueBlockID, blockID) {
			continue
		}
		if !ok || fact.max < best {
			best = fact.max
			ok = true
		}
	}
	return best, ok
}

func dominatingTableArrayLenGuardMaxSafe(facts []tableArrayLenGuardFact, dom *domInfo, order map[int]map[int]int, blockID, instrID int) (int64, bool) {
	var best int64
	ok := false
	for _, fact := range facts {
		if !tableArrayLenGuardDominates(fact, dom, order, blockID, instrID) {
			continue
		}
		if !ok || fact.maxSafe > best {
			best = fact.maxSafe
			ok = true
		}
	}
	return best, ok
}

func tableArrayLenGuardDominates(fact tableArrayLenGuardFact, dom *domInfo, order map[int]map[int]int, blockID, instrID int) bool {
	if fact.blockID == blockID {
		blockOrder, ok := order[blockID]
		if !ok {
			return false
		}
		return blockOrder[fact.instrID] < blockOrder[instrID]
	}
	return dom != nil && dom.dominates(fact.blockID, blockID)
}

func markTableArrayLowerBoundSafe(loopFacts *LoopSpecializationFacts, instr *Instr) {
	if loopFacts == nil || instr == nil {
		return
	}
	loopFacts.RecordTableArrayLowerBoundSafe(instr.ID)
}

func markTableArrayUpperBoundSafe(loopFacts *LoopSpecializationFacts, instr *Instr) {
	if loopFacts == nil || instr == nil {
		return
	}
	loopFacts.RecordTableArrayUpperBoundSafe(instr.ID)
}
