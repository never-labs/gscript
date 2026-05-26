// pass_range_nonneg.go holds non-negativity and strict-positivity fact
// derivation over the range lattice, plus the block-entry zero-exclusion
// helper. Pure code movement from pass_range.go.

package methodjit

func collectIntNonNegativeFacts(intInstrs []*Instr, ranges map[int]intRange) map[int]bool {
	facts := make(map[int]bool, len(intInstrs))
	for _, instr := range intInstrs {
		if r, ok := ranges[instr.ID]; ok && r.nonNegative() {
			facts[instr.ID] = true
		}
	}
	candidates := make(map[int]*Instr, len(intInstrs))
	for _, instr := range intInstrs {
		if instr == nil || facts[instr.ID] {
			continue
		}
		if opCanDeriveNonNegative(instr) {
			candidates[instr.ID] = instr
			facts[instr.ID] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for id, instr := range candidates {
			if !facts[id] {
				continue
			}
			if !instrDerivesNonNegative(instr, facts, ranges) {
				delete(facts, id)
				changed = true
			}
		}
	}
	return facts
}

func opCanDeriveNonNegative(instr *Instr) bool {
	if instr == nil {
		return false
	}
	switch instr.Op {
	case OpConstInt, OpLen, OpTableArrayLen, OpGuardIntRange,
		OpAddInt, OpMulInt, OpModInt, OpDivIntExact, OpPhi, OpBoxInt, OpUnboxInt:
		return true
	default:
		return false
	}
}

func instrDerivesNonNegative(instr *Instr, facts map[int]bool, ranges map[int]intRange) bool {
	if instr == nil {
		return false
	}
	switch instr.Op {
	case OpConstInt:
		return instr.Aux >= 0
	case OpLen, OpTableArrayLen:
		return true
	case OpGuardIntRange:
		return instr.Aux >= 0
	case OpAddInt, OpMulInt:
		return len(instr.Args) >= 2 &&
			valueNonNegative(instr.Args[0], facts, ranges) &&
			valueNonNegative(instr.Args[1], facts, ranges)
	case OpModInt:
		return len(instr.Args) >= 2 && valueNonNegative(instr.Args[1], facts, ranges)
	case OpDivIntExact:
		return len(instr.Args) >= 2 &&
			valueNonNegative(instr.Args[0], facts, ranges) &&
			valueStrictlyPositive(instr.Args[1], ranges)
	case OpPhi:
		if len(instr.Args) == 0 {
			return false
		}
		for _, arg := range instr.Args {
			if !valueNonNegative(arg, facts, ranges) {
				return false
			}
		}
		return true
	case OpBoxInt, OpUnboxInt:
		return len(instr.Args) >= 1 && valueNonNegative(instr.Args[0], facts, ranges)
	default:
		return false
	}
}

func valueNonNegative(v *Value, facts map[int]bool, ranges map[int]intRange) bool {
	if v == nil {
		return false
	}
	if c, ok := constIntFromValue(v); ok {
		return c >= 0
	}
	if facts[v.ID] {
		return true
	}
	if r, ok := ranges[v.ID]; ok && r.nonNegative() {
		return true
	}
	return false
}

func valueStrictlyPositive(v *Value, ranges map[int]intRange) bool {
	if v == nil {
		return false
	}
	if c, ok := constIntFromValue(v); ok {
		return c > 0
	}
	if r, ok := ranges[v.ID]; ok && r.known && r.min > 0 {
		return true
	}
	return false
}

func valueNonNegativeInEnv(v *Value, facts map[int]bool, env, baseRanges map[int]intRange) bool {
	if v == nil {
		return false
	}
	if c, ok := constIntFromValue(v); ok {
		return c >= 0
	}
	if facts[v.ID] {
		return true
	}
	r := argRangeInEnv(v, env, baseRanges)
	return r.known && r.min >= 0
}

func valueStrictlyPositiveInEnv(v *Value, facts map[int]bool, env, baseRanges map[int]intRange) bool {
	if v == nil {
		return false
	}
	if c, ok := constIntFromValue(v); ok {
		return c > 0
	}
	r := argRangeInEnv(v, env, baseRanges)
	if r.known && r.min > 0 {
		return true
	}
	return facts[v.ID] && rangeExcludesZero(r)
}

func blockEntryExcludesZero(block *Block, valueID int) bool {
	if block == nil || len(block.Preds) != 1 {
		return false
	}
	pred := block.Preds[0]
	if pred == nil || len(pred.Succs) < 2 || pred.Succs[1] != block || len(pred.Instrs) == 0 {
		return false
	}
	term := pred.Instrs[len(pred.Instrs)-1]
	if term == nil || term.Op != OpBranch || len(term.Args) == 0 || term.Args[0] == nil || term.Args[0].Def == nil {
		return false
	}
	cond := term.Args[0].Def
	if cond.Op != OpEqInt || len(cond.Args) < 2 {
		return false
	}
	if cond.Args[0] != nil && cond.Args[0].ID == valueID {
		if c, ok := constIntFromValue(cond.Args[1]); ok && c == 0 {
			return true
		}
	}
	if cond.Args[1] != nil && cond.Args[1].ID == valueID {
		if c, ok := constIntFromValue(cond.Args[0]); ok && c == 0 {
			return true
		}
	}
	return false
}
