package methodjit

import "github.com/never-labs/leia/internal/vm"

func protoLoopCallsAreAllLowerableBy(proto *vm.FuncProto, lowerable func(*Function, *Instr) bool) bool {
	if lowerable == nil {
		return false
	}
	fn := BuildGraph(proto)
	if fn == nil || fn.Entry == nil || fn.Unpromotable {
		return false
	}
	li := computeLoopInfo(fn)
	found := false
	for _, block := range fn.Blocks {
		if block == nil || !li.loopBlocks[block.ID] {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || !tier2LoopCallOp(instr.Op) {
				continue
			}
			if lowerable(fn, instr) {
				found = true
				continue
			}
			return false
		}
	}
	return found
}
