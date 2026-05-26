// pass_range_compute.go holds forward range computation: per-instruction range
// inference, static-length collection, block-entry range propagation, and the
// environment-aware range helpers. Pure code movement from pass_range.go.

package methodjit

// computeRange returns the inferred range of `instr`'s result value using the
// current `ranges` map. Unknown/unsupported ops produce top.
func computeRange(instr *Instr, ranges map[int]intRange, staticLens, profiledRanges, profiledLenRanges map[int]intRange) intRange {
	if r, ok := profiledRanges[instr.ID]; ok && r.known {
		return r
	}
	switch instr.Op {
	case OpConstInt:
		return pointRange(instr.Aux)
	case OpLen:
		if r, ok := staticLens[instr.ID]; ok {
			return r
		}
		if len(instr.Args) >= 1 && instr.Args[0] != nil {
			if r, ok := profiledLenRanges[instr.Args[0].ID]; ok && r.known {
				return r
			}
		}
		return topRange()
	case OpTableArrayLen:
		if len(instr.Args) >= 1 && instr.Args[0] != nil {
			if r, ok := profiledLenRanges[instr.Args[0].ID]; ok && r.known {
				return r
			}
			if table := tableArrayHeaderSourceTableValue(instr.Args[0]); table != nil {
				if r, ok := profiledLenRanges[table.ID]; ok && r.known {
					return r
				}
			}
		}
		return topRange()

	case OpAddInt:
		if len(instr.Args) < 2 {
			return topRange()
		}
		return addRange(argRange(instr.Args[0], ranges), argRange(instr.Args[1], ranges))

	case OpSubInt:
		if len(instr.Args) < 2 {
			return topRange()
		}
		return subRange(argRange(instr.Args[0], ranges), argRange(instr.Args[1], ranges))

	case OpMulInt:
		if len(instr.Args) < 2 {
			return topRange()
		}
		return mulRange(argRange(instr.Args[0], ranges), argRange(instr.Args[1], ranges))

	case OpModInt:
		if len(instr.Args) < 2 {
			return topRange()
		}
		return modRange(argRange(instr.Args[0], ranges), argRange(instr.Args[1], ranges))

	case OpDivIntExact:
		if len(instr.Args) < 2 {
			return topRange()
		}
		return divExactRange(argRange(instr.Args[0], ranges), argRange(instr.Args[1], ranges))

	case OpNegInt:
		if len(instr.Args) < 1 {
			return topRange()
		}
		return negRange(argRange(instr.Args[0], ranges))

	case OpPhi:
		if len(instr.Args) == 0 {
			return topRange()
		}
		// If this phi already has a seeded range (from Phase A), start from it
		// so the loop induction range isn't widened beyond the seeded interval.
		// Phase A seeds are derived from the loop's bounds; joining with the
		// back-edge value (which is in that same range by construction) keeps
		// the range stable.
		if seeded, ok := ranges[instr.ID]; ok && seeded.known {
			return seeded
		}
		acc := argRange(instr.Args[0], ranges)
		for i := 1; i < len(instr.Args); i++ {
			acc = joinRange(acc, argRange(instr.Args[i], ranges))
			if !acc.known {
				break
			}
		}
		return acc

	case OpBoxInt:
		if len(instr.Args) < 1 {
			return topRange()
		}
		return argRange(instr.Args[0], ranges)

	case OpUnboxInt:
		if len(instr.Args) < 1 {
			return topRange()
		}
		return argRange(instr.Args[0], ranges)

	case OpGuardIntRange:
		if len(instr.Args) < 1 || instr.Aux > instr.Aux2 {
			return topRange()
		}
		arg := argRange(instr.Args[0], ranges)
		guard := intRange{min: instr.Aux, max: instr.Aux2, known: true}
		if !arg.known {
			return guard
		}
		return intersectRange(arg, guard)
	}
	return topRange()
}

func collectStaticLenRanges(fn *Function) map[int]intRange {
	out := make(map[int]intRange)
	if fn == nil {
		return out
	}
	tableLens := make(map[int]int64)
	invalid := make(map[int]bool)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			switch instr.Op {
			case OpSetList:
				if len(instr.Args) < 1 || instr.Args[0] == nil {
					continue
				}
				tableID := instr.Args[0].ID
				if invalid[tableID] {
					continue
				}
				end := instr.Aux + int64(len(instr.Args)-1) - 1
				if end > tableLens[tableID] {
					tableLens[tableID] = end
				}
			case OpSetTable, OpAppend:
				if len(instr.Args) < 1 || instr.Args[0] == nil {
					continue
				}
				tableID := instr.Args[0].ID
				invalid[tableID] = true
				delete(tableLens, tableID)
			}
		}
	}
	if len(tableLens) == 0 {
		return out
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpLen || len(instr.Args) < 1 || instr.Args[0] == nil {
				continue
			}
			if n, ok := tableLens[instr.Args[0].ID]; ok && n >= 0 {
				out[instr.ID] = pointRange(n)
				continue
			}
			if instr.Args[0].Def != nil && instr.Args[0].Def.Op == OpConstString {
				if s, ok := constString(fn, instr.Args[0].Def.Aux); ok {
					out[instr.ID] = pointRange(int64(len(s)))
				}
			}
		}
	}
	return out
}

// argRange resolves the range of an SSA value argument. Returns top if the
// value isn't in the map (e.g. function parameter, LoadSlot result).
func argRange(v *Value, ranges map[int]intRange) intRange {
	if v == nil || v.Def == nil {
		return topRange()
	}
	if r, ok := ranges[v.ID]; ok {
		return r
	}
	return topRange()
}

func computeBlockEntryRanges(fn *Function, baseRanges map[int]intRange) map[int]map[int]intRange {
	entries := make(map[int]map[int]intRange, len(fn.Blocks))
	if fn.Entry != nil {
		entries[fn.Entry.ID] = make(map[int]intRange)
	}

	const maxIter = 8
	for iter := 0; iter < maxIter; iter++ {
		changed := false
		for _, block := range fn.Blocks {
			env := cloneRangeMap(entries[block.ID])
			for _, instr := range block.Instrs {
				if instr.Type.isIntegerLike() {
					env[instr.ID] = computeRangeInEnv(instr, env, baseRanges)
				}
			}
			if len(block.Instrs) == 0 {
				continue
			}
			term := block.Instrs[len(block.Instrs)-1]
			if term.Op == OpBranch && len(term.Args) > 0 && len(block.Succs) >= 2 {
				trueEnv := cloneRangeMap(env)
				falseEnv := cloneRangeMap(env)
				refineBranchEnvs(term.Args[0], trueEnv, falseEnv)
				if mergeBlockEntry(entries, block.Succs[0], trueEnv) {
					changed = true
				}
				if mergeBlockEntry(entries, block.Succs[1], falseEnv) {
					changed = true
				}
				continue
			}
			for _, succ := range block.Succs {
				if mergeBlockEntry(entries, succ, env) {
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	return entries
}

func computeRangeInEnv(instr *Instr, env, baseRanges map[int]intRange) intRange {
	switch instr.Op {
	case OpConstInt:
		return pointRange(instr.Aux)
	case OpAddInt:
		if len(instr.Args) < 2 {
			return topRange()
		}
		return addRange(argRangeInEnv(instr.Args[0], env, baseRanges), argRangeInEnv(instr.Args[1], env, baseRanges))
	case OpSubInt:
		if len(instr.Args) < 2 {
			return topRange()
		}
		return subRange(argRangeInEnv(instr.Args[0], env, baseRanges), argRangeInEnv(instr.Args[1], env, baseRanges))
	case OpMulInt:
		if len(instr.Args) < 2 {
			return topRange()
		}
		return mulRange(argRangeInEnv(instr.Args[0], env, baseRanges), argRangeInEnv(instr.Args[1], env, baseRanges))
	case OpModInt:
		if len(instr.Args) < 2 {
			return topRange()
		}
		return modRange(argRangeInEnv(instr.Args[0], env, baseRanges), argRangeInEnv(instr.Args[1], env, baseRanges))
	case OpDivIntExact:
		if len(instr.Args) < 2 {
			return topRange()
		}
		return divExactRange(argRangeInEnv(instr.Args[0], env, baseRanges), argRangeInEnv(instr.Args[1], env, baseRanges))
	case OpNegInt:
		if len(instr.Args) < 1 {
			return topRange()
		}
		return negRange(argRangeInEnv(instr.Args[0], env, baseRanges))
	case OpGuardIntRange:
		if len(instr.Args) < 1 || instr.Aux > instr.Aux2 {
			return topRange()
		}
		arg := argRangeInEnv(instr.Args[0], env, baseRanges)
		guard := intRange{min: instr.Aux, max: instr.Aux2, known: true}
		if !arg.known {
			return guard
		}
		return intersectRange(arg, guard)
	case OpPhi:
		if r, ok := baseRanges[instr.ID]; ok && r.known {
			return r
		}
		if len(instr.Args) == 0 {
			return topRange()
		}
		acc := argRangeInEnv(instr.Args[0], env, baseRanges)
		for i := 1; i < len(instr.Args); i++ {
			acc = joinRange(acc, argRangeInEnv(instr.Args[i], env, baseRanges))
			if !acc.known {
				break
			}
		}
		return acc
	case OpBoxInt, OpUnboxInt:
		if len(instr.Args) < 1 {
			return topRange()
		}
		return argRangeInEnv(instr.Args[0], env, baseRanges)
	}
	if r, ok := baseRanges[instr.ID]; ok {
		return r
	}
	return topRange()
}

func argRangeInEnv(v *Value, env, baseRanges map[int]intRange) intRange {
	if v == nil || v.Def == nil {
		return topRange()
	}
	if r, ok := env[v.ID]; ok && r.known {
		return r
	}
	if r, ok := baseRanges[v.ID]; ok {
		return r
	}
	return topRange()
}

func mergeBlockEntry(entries map[int]map[int]intRange, block *Block, incoming map[int]intRange) bool {
	if block == nil {
		return false
	}
	current, ok := entries[block.ID]
	if !ok {
		entries[block.ID] = cloneRangeMap(incoming)
		return len(incoming) > 0
	}
	changed := false
	for id, in := range incoming {
		if !in.known {
			continue
		}
		if old, ok := current[id]; ok && old.known {
			joined := joinRange(old, in)
			if !rangeEqual(old, joined) {
				current[id] = joined
				changed = true
			}
			continue
		}
		current[id] = in
		changed = true
	}
	return changed
}

func cloneRangeMap(in map[int]intRange) map[int]intRange {
	out := make(map[int]intRange, len(in))
	for id, r := range in {
		if r.known {
			out[id] = r
		}
	}
	return out
}
