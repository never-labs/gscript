// regalloc_table_carry.go holds the Method JIT register allocator's post-pass
// table-array register coalescing: reusing a now-dead length register for a
// table-array load result and folding a single-use load into the following
// OpFieldSvals so both share a register.
// Pure code movement from regalloc.go; no behavior change.

package methodjit

func avoidTableArrayLoadDataRegClobber(fn *Function, alloc *RegAllocation) {
	if fn == nil || alloc == nil {
		return
	}
	specializations := functionLoopSpecializationFacts(fn)
	for _, block := range fn.Blocks {
		for i, instr := range block.Instrs {
			if instr == nil || instr.Op != OpTableArrayLoad || instr.Type != TypeTable || len(instr.Args) < 2 {
				continue
			}
			if !specializations.TableArrayUpperBoundIsSafe(instr.ID) {
				continue
			}
			result, ok := alloc.ValueRegs[instr.ID]
			if !ok || result.IsFloat {
				continue
			}
			dataReg, ok := alloc.ValueRegs[instr.Args[0].ID]
			if !ok || dataReg.IsFloat || dataReg.Reg != result.Reg {
				continue
			}
			lenReg, ok := alloc.ValueRegs[instr.Args[1].ID]
			if !ok || lenReg.IsFloat || lenReg.Reg == dataReg.Reg {
				continue
			}
			if valueUsedLaterInBlock(block, i, instr.Args[1].ID) {
				continue
			}
			alloc.ValueRegs[instr.ID] = lenReg
			delete(alloc.SpillSlots, instr.ID)
		}
	}
}

func valueUsedLaterInBlock(block *Block, instrIdx, valueID int) bool {
	if block == nil {
		return false
	}
	for i := instrIdx + 1; i < len(block.Instrs); i++ {
		for _, arg := range block.Instrs[i].Args {
			if arg != nil && arg.ID == valueID {
				return true
			}
		}
	}
	return false
}

func coalesceTableArrayLoadFieldSvalsRegs(fn *Function, alloc *RegAllocation) {
	if fn == nil || alloc == nil {
		return
	}
	uses := computeUseCounts(fn)
	for _, block := range fn.Blocks {
		for i, instr := range block.Instrs {
			if instr == nil || instr.Op != OpTableArrayLoad || instr.Type != TypeTable || uses[instr.ID] != 1 {
				continue
			}
			if i+1 >= len(block.Instrs) {
				continue
			}
			next := block.Instrs[i+1]
			if next == nil || next.Op != OpFieldSvals || len(next.Args) == 0 || next.Args[0] == nil || next.Args[0].ID != instr.ID {
				continue
			}
			pr, ok := alloc.ValueRegs[next.ID]
			if !ok || pr.IsFloat {
				continue
			}
			alloc.ValueRegs[instr.ID] = pr
			delete(alloc.SpillSlots, instr.ID)
		}
	}
}
