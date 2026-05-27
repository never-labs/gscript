// regalloc_table_invariant.go holds the Method JIT register allocator's
// table-array / matrix loop-invariant GPR machinery: selecting len/data/header
// facts that stay resident across a loop, recording candidates dominated by the
// loop, and the ranking/sorting helpers that prioritize them.
// Pure code movement from regalloc.go; no behavior change.

package methodjit

func assignLoopTableArrayInvariantGPRs(fn *Function, li *loopInfo, alloc *RegAllocation) map[int]map[int]PhysReg {
	if fn == nil || li == nil || !li.hasLoops() || alloc == nil {
		return nil
	}
	defs := make(map[int]*Instr)
	defBlocks := make(map[int]int)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op.IsTerminator() {
				continue
			}
			defs[instr.ID] = instr
			defBlocks[instr.ID] = block.ID
		}
	}
	dom := computeDominators(fn)
	headers := sortedLoopHeaders(li)
	out := make(map[int]map[int]PhysReg)
	for _, headerID := range headers {
		body := li.headerBlocks[headerID]
		if body == nil {
			continue
		}
		useCounts := make(map[int]int)
		for _, block := range fn.Blocks {
			if !body[block.ID] {
				continue
			}
			for _, instr := range block.Instrs {
				mask := tableArrayInvariantUseMask(instr)
				for argIdx := 0; mask != 0 && argIdx < len(instr.Args); argIdx++ {
					if mask&(1<<uint(argIdx)) == 0 {
						continue
					}
					recordTableArrayInvariantCandidate(instr.Args[argIdx], body, headerID, defs, defBlocks, dom, useCounts)
				}
			}
		}
		if len(useCounts) == 0 {
			continue
		}

		candidates := make([]int, 0, len(useCounts))
		for id := range useCounts {
			candidates = append(candidates, id)
		}
		sortTableArrayInvariantCandidates(candidates, useCounts, defs)

		usedRegs := make(map[int]bool)
		for _, phiID := range li.loopPhis[headerID] {
			if pr, ok := alloc.ValueRegs[phiID]; ok && !pr.IsFloat {
				usedRegs[pr.Reg] = true
			}
		}

		maxTableArrayGPRInvariants := 2
		if tableArrayInvariantSetHasHeader(candidates, defs) {
			maxTableArrayGPRInvariants = 3
		}
		for _, id := range candidates {
			if len(out[headerID]) >= maxTableArrayGPRInvariants {
				break
			}
			var pr PhysReg
			if existing, ok := alloc.ValueRegs[id]; ok && !existing.IsFloat && !usedRegs[existing.Reg] {
				pr = existing
			} else {
				reg, ok := firstFreeGPR(usedRegs)
				if !ok {
					break
				}
				pr = PhysReg{Reg: reg, IsFloat: false}
				alloc.ValueRegs[id] = pr
			}
			usedRegs[pr.Reg] = true
			if out[headerID] == nil {
				out[headerID] = make(map[int]PhysReg)
			}
			out[headerID][id] = pr
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func recordTableArrayInvariantCandidate(v *Value, body map[int]bool, headerID int, defs map[int]*Instr, defBlocks map[int]int, dom *domInfo, useCounts map[int]int) {
	if v == nil || dom == nil {
		return
	}
	def := defs[v.ID]
	if def == nil || !isTableArrayGPRInvariant(def) {
		return
	}
	defBlock, ok := defBlocks[v.ID]
	if !ok || body[defBlock] || !dom.dominates(defBlock, headerID) {
		return
	}
	useCounts[v.ID]++
}

func isTableArrayGPRInvariant(instr *Instr) bool {
	if instr == nil || instr.Type != TypeInt {
		return false
	}
	return opIsTableArrayGPRInvariant(instr.Op)
}

func sortTableArrayInvariantCandidates(ids []int, useCounts map[int]int, defs map[int]*Instr) {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0; j-- {
			a, b := ids[j-1], ids[j]
			if tableArrayInvariantLess(b, a, useCounts, defs) {
				ids[j-1], ids[j] = ids[j], ids[j-1]
			} else {
				break
			}
		}
	}
}

func tableArrayInvariantUseMask(instr *Instr) uint8 {
	if instr == nil {
		return 0
	}
	mask := tableArrayGPRInvariantUseMask(instr.Op)
	if instr.Op == OpTableArrayStore && !tableArrayStoreNeedsTablePtr(instr.Aux, instr.Aux2) {
		mask &^= 1 << 5
	}
	return mask
}

func tableArrayInvariantLess(a, b int, useCounts map[int]int, defs map[int]*Instr) bool {
	if useCounts[a] != useCounts[b] {
		return useCounts[a] > useCounts[b]
	}
	ra := tableArrayInvariantRank(defs[a])
	rb := tableArrayInvariantRank(defs[b])
	if ra != rb {
		return ra < rb
	}
	return a < b
}

func tableArrayInvariantRank(instr *Instr) int {
	if instr == nil {
		return 1
	}
	return tableArrayGPRInvariantRank(instr.Op)
}

func tableArrayInvariantSetHasHeader(ids []int, defs map[int]*Instr) bool {
	for _, id := range ids {
		if instr := defs[id]; instr != nil && instr.Op == OpTableArrayHeader {
			return true
		}
	}
	return false
}
