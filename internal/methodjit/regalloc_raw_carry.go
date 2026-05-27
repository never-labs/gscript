// regalloc_raw_carry.go holds the Method JIT register allocator's single-predecessor
// raw-value carry support: classifying which raw int/float/pointer values are
// carry-eligible, deciding when their defining store can be elided because the
// value stays resident in its register, and the path/clobber analyses that
// validate carrying a register across single-predecessor edges.
// Pure code movement from regalloc.go; no behavior change.

package methodjit

func enableSinglePredRawIntCarry(fn *Function) bool {
	if fn == nil {
		return false
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if isSinglePredRawCarryValue(instr) && instr.Type == TypeTable {
				return true
			}
			if opIsRawCarryClobber(instr.Op) && instr.Type == TypeInt {
				return true
			}
			if isMatrixNativeOp(instr.Op) {
				return true
			}
		}
	}
	return false
}

func isSinglePredRawCarryValue(instr *Instr) bool {
	if instr == nil {
		return false
	}
	if isRawIntCarryValue(instr) {
		return true
	}
	return isRawTablePtrValue(instr) || isRawDataPtrOp(instr.Op) || isRawFieldSvalsPtrValue(instr)
}

func isRawFieldSvalsPtrValue(instr *Instr) bool {
	return instr != nil && instr.Op == OpFieldSvals
}

func computeSinglePredRawIntStoreElision(fn *Function, alloc *RegAllocation, blockLiveIn map[int]map[int]bool) map[int]bool {
	return computeSinglePredRawStoreElision(fn, alloc, blockLiveIn, false)
}

func computeSinglePredRawFloatStoreElision(fn *Function, alloc *RegAllocation, blockLiveIn map[int]map[int]bool) map[int]bool {
	return computeSinglePredRawStoreElision(fn, alloc, blockLiveIn, true)
}

func computeSinglePredRawStoreElision(fn *Function, alloc *RegAllocation, blockLiveIn map[int]map[int]bool, wantFloat bool) map[int]bool {
	defs := computeValueDefs(fn)
	defBlock := make(map[int]int, len(defs))
	defIndex := make(map[int]int, len(defs))
	blockByID := make(map[int]*Block, len(fn.Blocks))
	for _, block := range fn.Blocks {
		blockByID[block.ID] = block
		for i, instr := range block.Instrs {
			if !instr.Op.IsTerminator() {
				defBlock[instr.ID] = block.ID
				defIndex[instr.ID] = i
			}
		}
	}

	result := make(map[int]bool)
	for valueID, def := range defs {
		if wantFloat {
			if !isSinglePredRawFloatCarryValue(def) {
				continue
			}
		} else if !isSinglePredRawCarryValue(def) {
			continue
		}
		pr, ok := alloc.ValueRegs[valueID]
		if !ok || pr.IsFloat != wantFloat {
			continue
		}
		db, ok := defBlock[valueID]
		if !ok {
			continue
		}
		hasCrossUse := false
		eligible := true
		for _, block := range fn.Blocks {
			for i, instr := range block.Instrs {
				if instr.Op == OpPhi {
					for _, arg := range instr.Args {
						if arg != nil && arg.ID == valueID {
							hasCrossUse = true
							eligible = false
							break
						}
					}
					if !eligible {
						break
					}
					continue
				}
				for _, arg := range instr.Args {
					if arg == nil || arg.ID != valueID || block.ID == db {
						continue
					}
					hasCrossUse = true
					if !singlePredRawCarryPathEligible(blockByID, db, block.ID, i, valueID, pr.Reg, wantFloat, defIndex[valueID], alloc, blockLiveIn) {
						eligible = false
						break
					}
				}
				if !eligible {
					break
				}
			}
			if !eligible {
				break
			}
		}
		if hasCrossUse && eligible {
			result[valueID] = true
		}
	}
	return result
}

func isSinglePredRawFloatCarryValue(instr *Instr) bool {
	return instr != nil && instr.Type == TypeFloat && isRawFloatOp(instr.Op)
}

func singlePredRawCarryPathEligible(blockByID map[int]*Block, defBlockID, useBlockID, useIndex, valueID, reg int, isFloat bool, defIndex int, alloc *RegAllocation, blockLiveIn map[int]map[int]bool) bool {
	if defBlockID == useBlockID {
		return true
	}
	useBlock := blockByID[useBlockID]
	if useBlock == nil || !blockLiveIn[useBlockID][valueID] {
		return false
	}
	if rawCarryRegClobberedInInstrRange(useBlock, 0, useIndex, reg, isFloat, alloc) {
		return false
	}
	if len(useBlock.Preds) == 0 {
		return false
	}
	for _, pred := range useBlock.Preds {
		if !rawCarryPathToBlockEndEligible(blockByID, defBlockID, pred, valueID, reg, isFloat, defIndex, alloc, blockLiveIn, make(map[int]bool)) {
			return false
		}
	}
	return true
}

func rawCarryPathToBlockEndEligible(blockByID map[int]*Block, defBlockID int, block *Block, valueID, reg int, isFloat bool, defIndex int, alloc *RegAllocation, blockLiveIn map[int]map[int]bool, seen map[int]bool) bool {
	if block == nil || seen[block.ID] {
		return false
	}
	seen[block.ID] = true
	if block.ID == defBlockID {
		return !rawCarryRegClobberedInInstrRange(block, defIndex+1, len(block.Instrs), reg, isFloat, alloc)
	}
	if !blockLiveIn[block.ID][valueID] || rawCarryRegClobberedInInstrRange(block, 0, len(block.Instrs), reg, isFloat, alloc) {
		return false
	}
	if len(block.Preds) != 1 {
		return false
	}
	return rawCarryPathToBlockEndEligible(blockByID, defBlockID, block.Preds[0], valueID, reg, isFloat, defIndex, alloc, blockLiveIn, seen)
}

func rawCarryRegClobberedInInstrRange(block *Block, start, end, reg int, isFloat bool, alloc *RegAllocation) bool {
	if isFloat {
		return fprClobberedInInstrRange(block, start, end, reg, alloc)
	}
	return gprClobberedInInstrRange(block, start, end, reg, alloc)
}

func gprClobberedInInstrRange(block *Block, start, end, reg int, alloc *RegAllocation) bool {
	if block == nil || alloc == nil {
		return true
	}
	if start < 0 {
		start = 0
	}
	if end > len(block.Instrs) {
		end = len(block.Instrs)
	}
	for i := start; i < end; i++ {
		instr := block.Instrs[i]
		if instr == nil || instr.Op.IsTerminator() {
			continue
		}
		if opIsRawCarryClobber(instr.Op) {
			return true
		}
		if pr, ok := alloc.ValueRegs[instr.ID]; ok && !pr.IsFloat && pr.Reg == reg {
			return true
		}
	}
	return false
}

func fprClobberedInInstrRange(block *Block, start, end, reg int, alloc *RegAllocation) bool {
	if block == nil || alloc == nil {
		return true
	}
	if start < 0 {
		start = 0
	}
	if end > len(block.Instrs) {
		end = len(block.Instrs)
	}
	for i := start; i < end; i++ {
		instr := block.Instrs[i]
		if instr == nil || instr.Op.IsTerminator() {
			continue
		}
		if opIsRawCarryClobber(instr.Op) {
			return true
		}
		if pr, ok := alloc.ValueRegs[instr.ID]; ok && pr.IsFloat && pr.Reg == reg {
			return true
		}
	}
	return false
}
