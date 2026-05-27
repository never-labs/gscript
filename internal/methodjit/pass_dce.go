// pass_dce.go removes SSA instructions whose results are never used.
// A value is "dead" if no other instruction references it and it has no
// side effects (not a store, call, branch, return, guard, or table mutation).
//
// The pass runs to a fixed point: removing a dead instruction may make its
// operands dead, which can be removed in the next iteration.

package methodjit

// DCEPass removes dead (unused, side-effect-free) instructions from the IR.
func DCEPass(fn *Function) (*Function, error) {
	// Fixed-point iteration: keep removing dead code until stable.
	for {
		useCounts := computeUseCounts(fn)
		removed := false

		for _, block := range fn.Blocks {
			alive := make([]*Instr, 0, len(block.Instrs))
			for _, instr := range block.Instrs {
				if useCounts[instr.ID] == 0 && !hasSideEffect(instr) {
					removed = true
					continue // drop this instruction
				}
				alive = append(alive, instr)
			}
			block.Instrs = alive
		}

		if !removed {
			break
		}
	}

	return fn, nil
}

// computeUseCounts counts how many times each value ID is referenced as an
// argument in any instruction across all blocks.
func computeUseCounts(fn *Function) map[int]int {
	counts := make(map[int]int)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			for _, arg := range instr.Args {
				if arg != nil {
					counts[arg.ID]++
				}
			}
		}
	}
	return counts
}

// hasSideEffect returns true if the instruction has observable side effects
// and must not be removed even if its result is unused.
func hasSideEffect(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.KeepUnused
}
