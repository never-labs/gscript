// pass_range_modulo.go holds modulo-recurrence range seeding and the IntMod
// fact population that decides whether the native ARM64 remainder sequence
// matches Lua modulo semantics. Pure code movement from pass_range.go.

package methodjit

func seedModuloRecurrenceRanges(fn *Function, ranges map[int]intRange) {
	if fn == nil || ranges == nil {
		return
	}
	candidates := make(map[int]intRange)
	// Seed candidates from direct modulo backedges first. Validation happens
	// after phi-to-phi propagation so mutually recursive phi groups like
	// x=phi(y, mod(...)); y=phi(0, x) can be proven together.
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpPhi {
				continue
			}
			if candidate, ok := directModuloPhiCandidateRange(instr, ranges); ok {
				candidates[instr.ID] = candidate
			}
		}
	}
	for iter := 0; iter < 8; iter++ {
		changed := false
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if instr == nil || instr.Op != OpPhi {
					continue
				}
				candidate, ok := moduloPhiCandidateRange(instr, ranges, candidates)
				if !ok {
					continue
				}
				if old, ok := candidates[instr.ID]; ok {
					candidate = joinRange(old, candidate)
				}
				if !candidate.known {
					continue
				}
				if old, ok := candidates[instr.ID]; !ok || !rangeEqual(old, candidate) {
					candidates[instr.ID] = candidate
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	for {
		changed := false
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if instr == nil || instr.Op != OpPhi {
					continue
				}
				candidate, ok := candidates[instr.ID]
				if !ok || allPhiInputsWithin(instr, candidate, ranges, candidates) {
					continue
				}
				delete(candidates, instr.ID)
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	for id, r := range candidates {
		if old, ok := ranges[id]; ok && old.known {
			r = intersectRange(old, r)
		}
		if r.known {
			ranges[id] = r
		}
	}
}

func directModuloPhiCandidateRange(instr *Instr, ranges map[int]intRange) (intRange, bool) {
	if instr == nil || instr.Op != OpPhi {
		return intRange{}, false
	}
	var candidate intRange
	for _, arg := range instr.Args {
		r, ok := moduloResultRange(arg, ranges)
		if !ok || !r.known {
			continue
		}
		if !candidate.known {
			candidate = r
		} else {
			candidate = joinRange(candidate, r)
		}
	}
	return candidate, candidate.known
}

func moduloPhiCandidateRange(instr *Instr, ranges, phiCandidates map[int]intRange) (intRange, bool) {
	if instr == nil || instr.Op != OpPhi {
		return intRange{}, false
	}
	var candidate intRange
	for _, arg := range instr.Args {
		r, ok := moduloResultRange(arg, ranges)
		if !ok {
			if arg != nil {
				r, ok = phiCandidates[arg.ID]
			}
		}
		if !ok || !r.known {
			continue
		}
		if !candidate.known {
			candidate = r
		} else {
			candidate = joinRange(candidate, r)
		}
	}
	return candidate, candidate.known
}

func allPhiInputsWithin(instr *Instr, bounds intRange, ranges, phiCandidates map[int]intRange) bool {
	if instr == nil || instr.Op != OpPhi || !bounds.known {
		return false
	}
	for _, arg := range instr.Args {
		if !valueRangeWithin(arg, bounds, ranges, phiCandidates) {
			return false
		}
	}
	return true
}

func moduloResultRange(v *Value, ranges map[int]intRange) (intRange, bool) {
	if v == nil || v.Def == nil || v.Def.Op != OpModInt || len(v.Def.Args) < 2 {
		return intRange{}, false
	}
	divisor := v.Def.Args[1]
	if divisor == nil || divisor.Def == nil {
		return intRange{}, false
	}
	if divisor.Def.Op == OpConstInt && divisor.Def.Aux > 0 {
		return intRange{min: 0, max: divisor.Def.Aux - 1, known: true}, true
	}
	if dr, ok := ranges[divisor.ID]; ok && dr.known && dr.min > 0 {
		return intRange{min: 0, max: satSub(dr.max, 1), known: true}, true
	}
	return intRange{}, false
}

func valueRangeWithin(v *Value, bounds intRange, ranges, phiCandidates map[int]intRange) bool {
	if v == nil || !bounds.known {
		return false
	}
	if r, ok := moduloResultRange(v, ranges); ok {
		return rangeWithin(r, bounds)
	}
	if v.Def != nil && v.Def.Op == OpConstInt {
		return v.Def.Aux >= bounds.min && v.Def.Aux <= bounds.max
	}
	if r, ok := ranges[v.ID]; ok && r.known {
		return rangeWithin(r, bounds)
	}
	if r, ok := phiCandidates[v.ID]; ok && r.known {
		return rangeWithin(r, bounds)
	}
	return false
}

func populateIntModFacts(fn *Function, baseRanges map[int]intRange) {
	nonZeroDivisor := make(map[int]bool)
	noSignAdjust := make(map[int]bool)
	nonNegative := functionNumericFacts(fn).IntNonNegativeMap()
	blockEntries := computeBlockEntryRanges(fn, baseRanges)

	for _, block := range fn.Blocks {
		env := cloneRangeMap(blockEntries[block.ID])
		for _, instr := range block.Instrs {
			if instr.Op == OpModInt && len(instr.Args) >= 2 {
				lhs := argRangeInEnv(instr.Args[0], env, baseRanges)
				rhs := argRangeInEnv(instr.Args[1], env, baseRanges)
				divisorNonZero := rangeExcludesZero(rhs)
				if !divisorNonZero && (valueStrictlyPositiveInEnv(instr.Args[1], nonNegative, env, baseRanges) ||
					blockEntryExcludesZero(block, instr.Args[1].ID)) {
					divisorNonZero = true
				}
				if divisorNonZero {
					nonZeroDivisor[instr.ID] = true
				}
				divisorPositive := valueStrictlyPositiveInEnv(instr.Args[1], nonNegative, env, baseRanges) ||
					(divisorNonZero && valueNonNegativeInEnv(instr.Args[1], nonNegative, env, baseRanges))
				if rangesHaveSameKnownModuloSign(lhs, rhs) ||
					(valueNonNegativeInEnv(instr.Args[0], nonNegative, env, baseRanges) &&
						divisorPositive) {
					noSignAdjust[instr.ID] = true
				}
			}
			if instr.Type.isIntegerLike() {
				env[instr.ID] = computeRangeInEnv(instr, env, baseRanges)
			}
		}
	}

	functionNumericFacts(fn).SetModuloFacts(nonZeroDivisor, noSignAdjust)
}
