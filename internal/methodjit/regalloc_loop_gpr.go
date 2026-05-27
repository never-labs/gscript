// regalloc_loop_gpr.go holds the Method JIT register allocator's loop-invariant
// GPR carry machinery: pinning selected integer/pointer facts in GPRs across a
// loop body, plus the loop-bound GPR collection used to keep header comparison
// operands resident.
// Pure code movement from regalloc.go; no behavior change.

package methodjit

// collectLoopBoundGPRs scans a loop header block for loop-bound comparison ops
// and returns value IDs of non-phi, GPR-allocated
// arguments (e.g., loop bounds from LoadSlot). These are candidates for
// carrying across the loop body to avoid eviction and per-iteration reloads.
func collectLoopBoundGPRs(hdr *Block, alloc *RegAllocation) []int {
	if hdr == nil {
		return nil
	}
	phiIDs := make(map[int]bool)
	for _, instr := range hdr.Instrs {
		if instr.Op == OpPhi {
			phiIDs[instr.ID] = true
		}
	}
	var bounds []int
	for _, instr := range hdr.Instrs {
		spec, ok := instr.Op.Spec()
		if !ok || !spec.LoopBoundComparison {
			continue
		}
		for _, arg := range instr.Args {
			if arg == nil || phiIDs[arg.ID] {
				continue
			}
			if pr, ok := alloc.ValueRegs[arg.ID]; ok && !pr.IsFloat {
				bounds = append(bounds, arg.ID)
			}
		}
	}
	return bounds
}

func addLoopInvariantGPRCarry(block *Block, li *loopInfo, alloc *RegAllocation, carried map[int]PhysReg) map[int]PhysReg {
	if block == nil || li == nil || alloc == nil || len(alloc.LoopInvariantGPRs) == 0 {
		return carried
	}
	usedRegs := make(map[int]bool)
	for _, pr := range carried {
		if !pr.IsFloat {
			usedRegs[pr.Reg] = true
		}
	}
	for _, headerID := range sortedLoopHeaders(li) {
		body := li.headerBlocks[headerID]
		if body == nil || !body[block.ID] {
			continue
		}
		ids := sortedInvariantIDs(alloc.LoopInvariantGPRs[headerID])
		for _, id := range ids {
			pr := alloc.LoopInvariantGPRs[headerID][id]
			if pr.IsFloat || usedRegs[pr.Reg] {
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

func isLoopInvariantGPRValue(alloc *RegAllocation, valueID int) bool {
	if alloc == nil {
		return false
	}
	for _, values := range alloc.LoopInvariantGPRs {
		if _, ok := values[valueID]; ok {
			return true
		}
	}
	return false
}

func updateLoopInvariantGPRReg(alloc *RegAllocation, valueID int, pr PhysReg) {
	if alloc == nil || pr.IsFloat {
		return
	}
	for _, values := range alloc.LoopInvariantGPRs {
		if _, ok := values[valueID]; ok {
			values[valueID] = pr
		}
	}
}

func sortedInvariantIDs(m map[int]PhysReg) []int {
	ids := make([]int, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j-1] > ids[j]; j-- {
			ids[j-1], ids[j] = ids[j], ids[j-1]
		}
	}
	return ids
}
