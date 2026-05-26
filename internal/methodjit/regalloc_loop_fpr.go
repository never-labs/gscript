// regalloc_loop_fpr.go holds the Method JIT register allocator's loop-invariant
// FPR carry machinery: assigning float-invariant candidates to FPRs that stay
// resident across a loop, carrying them through body/preheader/header blocks,
// and the candidate collection/sorting helpers those passes depend on.
// Pure code movement from regalloc.go; no behavior change.

package methodjit

import "sort"

func assignLoopFloatInvariantFPRs(fn *Function, li *loopInfo, alloc *RegAllocation) map[int]map[int]PhysReg {
	if fn == nil || li == nil || !li.hasLoops() || alloc == nil {
		return nil
	}
	preheaders := computeLoopPreheaders(fn, li)
	if len(preheaders) == 0 {
		return nil
	}
	allInvariants := collectLoopFloatInvariantCandidates(fn, li, preheaders)
	if len(allInvariants) == 0 {
		return nil
	}

	defs := make(map[int]*Instr)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op.IsTerminator() {
				continue
			}
			defs[instr.ID] = instr
		}
	}

	out := make(map[int]map[int]PhysReg)
	for _, headerID := range sortedLoopHeaders(li) {
		invIDs := allInvariants[headerID]
		if len(invIDs) == 0 {
			continue
		}
		bodyBlocks := li.headerBlocks[headerID]
		preheaderID := preheaders[headerID]
		if bodyBlocks == nil {
			continue
		}

		useCounts := make(map[int]int)
		candidateSet := make(map[int]bool)
		for _, vid := range invIDs {
			instr := defs[vid]
			if instr == nil || !needsFloatReg(instr) {
				continue
			}
			if preheaderInvariantUsedOutsideLoop(fn, vid, bodyBlocks, preheaderID) && instr.Op != OpConstFloat {
				continue
			}
			candidateSet[vid] = true
		}
		if len(candidateSet) == 0 {
			continue
		}

		for _, block := range fn.Blocks {
			if !bodyBlocks[block.ID] {
				continue
			}
			for _, instr := range block.Instrs {
				if instr.Op == OpPhi {
					continue
				}
				for _, arg := range instr.Args {
					if arg != nil && candidateSet[arg.ID] {
						useCounts[arg.ID]++
					}
				}
			}
		}

		candidates := make([]int, 0, len(candidateSet))
		for id := range candidateSet {
			if useCounts[id] > 0 {
				candidates = append(candidates, id)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		sortFloatInvariantCandidates(candidates, useCounts)

		usedRegs := loopFloatPhiRegsInBody(li, alloc, headerID, bodyBlocks)

		const reservedTemps = 3
		budget := len(allocatableFPRs) - reservedTemps - len(usedRegs)
		if budget <= 0 {
			continue
		}
		for _, id := range candidates {
			if len(out[headerID]) >= budget {
				break
			}
			var pr PhysReg
			if existing, ok := alloc.ValueRegs[id]; ok && existing.IsFloat && !usedRegs[existing.Reg] {
				pr = existing
			} else {
				reg, ok := firstFreeFPR(usedRegs)
				if !ok {
					break
				}
				pr = PhysReg{Reg: reg, IsFloat: true}
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

func collectLoopFloatInvariantCandidates(fn *Function, li *loopInfo, preheaders map[int]int) map[int][]int {
	result := make(map[int][]int, len(preheaders))
	if fn == nil || li == nil || len(preheaders) == 0 {
		return result
	}
	dom := computeDominators(fn)
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
	for _, headerID := range sortedLoopHeaders(li) {
		body := li.headerBlocks[headerID]
		preheaderID, hasPreheader := preheaders[headerID]
		if body == nil || !hasPreheader {
			continue
		}
		used := make(map[int]bool)
		for _, block := range fn.Blocks {
			if !body[block.ID] {
				continue
			}
			for _, instr := range block.Instrs {
				if instr.Op == OpPhi {
					continue
				}
				for _, arg := range instr.Args {
					if arg == nil {
						continue
					}
					def := defs[arg.ID]
					defBlock, ok := defBlocks[arg.ID]
					if def == nil || !ok || body[defBlock] || !needsFloatReg(def) {
						continue
					}
					if !dom.dominates(defBlock, headerID) {
						continue
					}
					if defBlock != preheaderID && !safeExternalFPRInvariantDef(fn, defBlock, preheaderID, dom) {
						continue
					}
					if preheaderInvariantUsedOutsideLoop(fn, arg.ID, body, preheaderID) && def.Op != OpConstFloat {
						continue
					}
					used[arg.ID] = true
				}
			}
		}
		if len(used) == 0 {
			continue
		}
		ids := make([]int, 0, len(used))
		for id := range used {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		result[headerID] = ids
	}
	return result
}

func safeExternalFPRInvariantDef(fn *Function, defBlockID, preheaderID int, dom *domInfo) bool {
	if fn == nil || dom == nil || defBlockID == preheaderID {
		return true
	}
	preheader := findBlockByID(fn, preheaderID)
	if preheader == nil || len(preheader.Preds) != 1 || preheader.Preds[0].ID != defBlockID {
		return false
	}
	return dom.dominates(defBlockID, preheaderID)
}

func preheaderInvariantUsedOutsideLoop(fn *Function, valueID int, bodyBlocks map[int]bool, preheaderID int) bool {
	for _, block := range fn.Blocks {
		if bodyBlocks[block.ID] || block.ID == preheaderID {
			continue
		}
		for _, instr := range block.Instrs {
			for _, arg := range instr.Args {
				if arg != nil && arg.ID == valueID {
					return true
				}
			}
		}
	}
	return false
}

func sortFloatInvariantCandidates(ids []int, useCounts map[int]int) {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0; j-- {
			a, b := ids[j-1], ids[j]
			if useCounts[a] < useCounts[b] || (useCounts[a] == useCounts[b] && a > b) {
				ids[j-1], ids[j] = ids[j], ids[j-1]
			} else {
				break
			}
		}
	}
}

func loopFloatPhiRegsInBody(li *loopInfo, alloc *RegAllocation, headerID int, bodyBlocks map[int]bool) map[int]bool {
	usedRegs := make(map[int]bool)
	if li == nil || alloc == nil {
		return usedRegs
	}
	for phiHeaderID, phiIDs := range li.loopPhis {
		if phiHeaderID != headerID && !bodyBlocks[phiHeaderID] {
			continue
		}
		for _, phiID := range phiIDs {
			if pr, ok := alloc.ValueRegs[phiID]; ok && pr.IsFloat {
				usedRegs[pr.Reg] = true
			}
		}
	}
	return usedRegs
}

func isLoopInvariantFPRValue(alloc *RegAllocation, valueID int) bool {
	if alloc == nil {
		return false
	}
	for _, values := range alloc.LoopInvariantFPRs {
		if _, ok := values[valueID]; ok {
			return true
		}
	}
	return false
}

func updateLoopInvariantFPRReg(alloc *RegAllocation, valueID int, pr PhysReg) {
	if alloc == nil || !pr.IsFloat {
		return
	}
	for _, values := range alloc.LoopInvariantFPRs {
		if _, ok := values[valueID]; ok {
			values[valueID] = pr
		}
	}
}

func addLoopInvariantFPRCarry(block *Block, li *loopInfo, alloc *RegAllocation, carried map[int]PhysReg) map[int]PhysReg {
	if block == nil || li == nil || alloc == nil || len(alloc.LoopInvariantFPRs) == 0 {
		return carried
	}
	usedRegs := make(map[int]bool)
	for _, pr := range carried {
		if pr.IsFloat {
			usedRegs[pr.Reg] = true
		}
	}
	for _, headerID := range sortedLoopHeaders(li) {
		body := li.headerBlocks[headerID]
		if body == nil || !body[block.ID] {
			continue
		}
		ids := sortedInvariantIDs(alloc.LoopInvariantFPRs[headerID])
		for _, id := range ids {
			pr := alloc.LoopInvariantFPRs[headerID][id]
			if !pr.IsFloat || usedRegs[pr.Reg] {
				continue
			}
			if carried == nil {
				carried = make(map[int]PhysReg)
			}
			carried[id] = pr
			usedRegs[pr.Reg] = true
		}
	}
	return carried
}

func addLoopPreheaderExternalFPRCarry(block *Block, li *loopInfo, preheaders map[int]int, alloc *RegAllocation, defs map[int]*Instr, carried map[int]PhysReg) map[int]PhysReg {
	if block == nil || li == nil || alloc == nil || len(preheaders) == 0 || len(alloc.LoopInvariantFPRs) == 0 {
		return carried
	}
	usedRegs := make(map[int]bool)
	for _, pr := range carried {
		if pr.IsFloat {
			usedRegs[pr.Reg] = true
		}
	}
	for _, headerID := range sortedLoopHeaders(li) {
		if preheaders[headerID] != block.ID {
			continue
		}
		for _, id := range sortedInvariantIDs(alloc.LoopInvariantFPRs[headerID]) {
			def := defs[id]
			if def == nil || def.Block == nil || def.Block.ID == block.ID {
				continue
			}
			pr := alloc.LoopInvariantFPRs[headerID][id]
			if !pr.IsFloat || usedRegs[pr.Reg] {
				continue
			}
			if carried == nil {
				carried = make(map[int]PhysReg)
			}
			carried[id] = pr
			usedRegs[pr.Reg] = true
		}
	}
	return carried
}

func addLoopHeaderFPRCarry(block *Block, li *loopInfo, alloc *RegAllocation, carried map[int]PhysReg) map[int]PhysReg {
	if block == nil || li == nil || alloc == nil {
		return carried
	}
	usedRegs := make(map[int]bool)
	for _, pr := range carried {
		if pr.IsFloat {
			usedRegs[pr.Reg] = true
		}
	}
	for _, headerID := range sortedLoopHeadersByDepth(li) {
		if headerID == block.ID {
			continue
		}
		body := li.headerBlocks[headerID]
		if body == nil || !body[block.ID] {
			continue
		}
		for _, phiID := range li.loopPhis[headerID] {
			pr, ok := alloc.ValueRegs[phiID]
			if !ok || !pr.IsFloat || usedRegs[pr.Reg] {
				continue
			}
			if carried == nil {
				carried = make(map[int]PhysReg)
			}
			carried[phiID] = pr
			usedRegs[pr.Reg] = true
		}
	}
	return carried
}
