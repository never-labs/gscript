// regalloc_header_phi.go holds the Method JIT register allocator's loop-header
// phi pre-allocation logic: committing header phi GPR/FPR assignments before the
// main block walk, plus the loop-nesting/header-ordering helpers those passes use.
// Pure code movement from regalloc.go; no behavior change.

package methodjit

import "sort"

// preAllocateHeaderPhis walks the leading phi instructions of a loop header
// block and commits their FPR/GPR assignments into alloc.ValueRegs. This is
// called before the main block-by-block allocation loop so that non-header
// loop-body blocks (which may be processed before their header in RPO) can
// reserve the header's phi registers and avoid clobbering them. If a phi
// cannot fit (pool exhausted), it is spilled here, matching Phase 1 of
// allocateBlock's logic.
func preAllocateHeaderPhis(block *Block, alloc *RegAllocation) {
	if block == nil {
		return
	}
	gprs := newRegState(allocatableGPRs[:], false)
	fprs := newRegState(allocatableFPRs[:], true)
	for _, instr := range block.Instrs {
		if instr.Op != OpPhi {
			break
		}
		wantFloat := needsFloatReg(instr)
		var rs *regState
		if wantFloat {
			rs = fprs
		} else {
			rs = gprs
		}
		if pr, ok := alloc.ValueRegs[instr.ID]; ok && pr.IsFloat == wantFloat && rs.regToID[pr.Reg] == -1 {
			rs.assign(instr.ID, pr.Reg)
			continue
		}
		if _, ok := alloc.SpillSlots[instr.ID]; ok {
			continue
		}
		r := rs.findFree()
		if r >= 0 {
			rs.assign(instr.ID, r)
			alloc.ValueRegs[instr.ID] = PhysReg{Reg: r, IsFloat: wantFloat}
		} else {
			// Pool exhausted: spill. The later full allocateBlock call on
			// this header will see the spill and skip re-allocation.
			alloc.SpillSlots[instr.ID] = alloc.NumSpillSlots
			alloc.NumSpillSlots++
		}
	}
}

func preAllocateLoopHeaderFPPhis(fn *Function, li *loopInfo, alloc *RegAllocation) {
	if fn == nil || li == nil || alloc == nil || !li.hasLoops() {
		return
	}
	headers := sortedLoopHeadersByDepth(li)
	for _, headerID := range headers {
		block := findBlockByID(fn, headerID)
		if block == nil {
			continue
		}
		used := enclosingLoopFPRegs(headerID, li, alloc)
		for _, instr := range block.Instrs {
			if instr.Op != OpPhi {
				break
			}
			if !needsFloatReg(instr) {
				continue
			}
			if pr, ok := alloc.ValueRegs[instr.ID]; ok && pr.IsFloat && !used[pr.Reg] {
				used[pr.Reg] = true
				continue
			}
			reg, ok := firstFreeFPR(used)
			if !ok {
				if _, spilled := alloc.SpillSlots[instr.ID]; !spilled {
					alloc.SpillSlots[instr.ID] = alloc.NumSpillSlots
					alloc.NumSpillSlots++
				}
				delete(alloc.ValueRegs, instr.ID)
				continue
			}
			alloc.ValueRegs[instr.ID] = PhysReg{Reg: reg, IsFloat: true}
			used[reg] = true
		}
	}
}

func enclosingLoopFPRegs(headerID int, li *loopInfo, alloc *RegAllocation) map[int]bool {
	used := make(map[int]bool)
	for _, ancestorID := range enclosingLoopHeaders(headerID, li) {
		for _, phiID := range li.loopPhis[ancestorID] {
			if pr, ok := alloc.ValueRegs[phiID]; ok && pr.IsFloat {
				used[pr.Reg] = true
			}
		}
	}
	return used
}

func enclosingLoopHeaders(headerID int, li *loopInfo) []int {
	if li == nil {
		return nil
	}
	nest := loopNest(li)
	var headers []int
	for cur := nest[headerID]; cur >= 0; cur = nest[cur] {
		headers = append(headers, cur)
	}
	for i, j := 0, len(headers)-1; i < j; i, j = i+1, j-1 {
		headers[i], headers[j] = headers[j], headers[i]
	}
	return headers
}

func sortedLoopHeadersByDepth(li *loopInfo) []int {
	headers := sortedLoopHeaders(li)
	depths := loopHeaderDepths(li)
	sort.Slice(headers, func(i, j int) bool {
		if depths[headers[i]] != depths[headers[j]] {
			return depths[headers[i]] < depths[headers[j]]
		}
		return headers[i] < headers[j]
	})
	return headers
}

func loopHeaderDepths(li *loopInfo) map[int]int {
	depths := make(map[int]int, len(li.loopHeaders))
	nest := loopNest(li)
	for headerID := range li.loopHeaders {
		depth := 0
		for cur := nest[headerID]; cur >= 0; cur = nest[cur] {
			depth++
		}
		depths[headerID] = depth
	}
	return depths
}

func sortedLoopHeaders(li *loopInfo) []int {
	headers := make([]int, 0, len(li.loopHeaders))
	for id := range li.loopHeaders {
		headers = append(headers, id)
	}
	for i := 1; i < len(headers); i++ {
		for j := i; j > 0 && headers[j-1] > headers[j]; j-- {
			headers[j-1], headers[j] = headers[j], headers[j-1]
		}
	}
	return headers
}
