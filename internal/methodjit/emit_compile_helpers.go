//go:build darwin && arm64

package methodjit

import (
	"fmt"
	"sort"

	"github.com/gscript/gscript/internal/jit"
)

// computeTailCalls (R107) scans the IR for the tail-call pattern:
// an OpCall immediately followed (within the same block, skipping OpNop)
// by an OpReturn whose single arg is the Call's result. Returns a set
// of matching OpCall IDs. The caller's emit path uses emitCallNativeTail
// for these and skips the following Return's emission.
func computeTailCalls(fn *Function) map[int]bool {
	out := make(map[int]bool)
	if fn == nil {
		return out
	}
	for _, block := range fn.Blocks {
		for i, instr := range block.Instrs {
			if instr.Op != OpCall {
				continue
			}
			// Find the next non-nop instruction.
			j := i + 1
			for j < len(block.Instrs) && block.Instrs[j].Op == OpNop {
				j++
			}
			if j >= len(block.Instrs) {
				continue
			}
			next := block.Instrs[j]
			if next.Op != OpReturn {
				continue
			}
			if len(next.Args) != 1 || next.Args[0].ID != instr.ID {
				continue
			}
			out[instr.ID] = true
		}
	}
	return out
}

// isFusableComparison returns true for comparison ops that can be fused
// with an immediately-following Branch (emit CMP/FCMP + B.cc).
func isFusableComparison(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FusableComparison
}

func computeFusedComparisons(fn *Function) map[int]bool {
	useCounts := computeUseCounts(fn)
	fusedCmps := make(map[int]bool)
	for _, block := range fn.Blocks {
		for i, instr := range block.Instrs {
			if !isFusableComparison(instr.Op) || useCounts[instr.ID] != 1 {
				continue
			}
			if i+1 < len(block.Instrs) {
				next := block.Instrs[i+1]
				if next.Op == OpBranch && len(next.Args) > 0 && next.Args[0].ID == instr.ID {
					fusedCmps[instr.ID] = true
				}
			}
		}
	}
	return fusedCmps
}

// assignSlots assigns a home slot for every SSA value.
// LoadSlot values keep their original VM slot; all others get temp slots.
func (ec *emitContext) assignSlots() {
	for _, block := range ec.fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op.IsTerminator() {
				continue
			}
			if instr.Op == OpLoadSlot || instr.Op == OpResume {
				ec.slotMap[instr.ID] = int(instr.Aux)
			} else {
				ec.slotMap[instr.ID] = ec.nextSlot
				ec.nextSlot++
			}
		}
	}
}

// slotOffset returns the byte offset for a slot in the VM register file.
func slotOffset(slot int) int {
	return slot * jit.ValueSize
}

// loadValue loads a NaN-boxed value from its home slot into the given scratch register.
func (ec *emitContext) loadValue(dst jit.Reg, valueID int) {
	slot, ok := ec.slotMap[valueID]
	if !ok {
		return
	}
	ec.asm.LDR(dst, mRegRegs, slotOffset(slot))
}

// storeValue stores a NaN-boxed value from a scratch register to its home slot.
func (ec *emitContext) storeValue(src jit.Reg, valueID int) {
	slot, ok := ec.slotMap[valueID]
	if !ok {
		return
	}
	ec.asm.STR(src, mRegRegs, slotOffset(slot))
}

// blockLabel returns the assembler label name for a basic block.
// Numeric variant (pass 2) prefixes with "num_" to avoid label
// collision with the normal pass-1 body.
func blockLabel(b *Block) string {
	return fmt.Sprintf("B%d", b.ID)
}

func (ec *emitContext) entryBlockLabel() string {
	label, ok := ec.entryBlockLabelOK()
	if !ok {
		panic("methodjit: entry label requested without function entry")
	}
	return label
}

func (ec *emitContext) entryBlockLabelOK() (string, bool) {
	if ec == nil || ec.fn == nil || ec.fn.Entry == nil {
		return "", false
	}
	return ec.blockLabelFor(ec.fn.Entry), true
}

func (ec *emitContext) hasEntryShapeGuards() bool {
	return ec != nil && len(ec.entryShapeGuards) > 0
}

func (ec *emitContext) emitBoxedEntryShapeGuards() {
	if !ec.hasEntryShapeGuards() {
		return
	}
	params := make([]int, 0, len(ec.entryShapeGuards))
	for paramIdx, fact := range ec.entryShapeGuards {
		if fact.ShapeID != 0 {
			params = append(params, paramIdx)
		}
	}
	if len(params) == 0 {
		return
	}
	sort.Ints(params)
	failLabel := ec.uniqueLabel("entry_shape_deopt")
	doneLabel := ec.uniqueLabel("entry_shape_done")
	for _, paramIdx := range params {
		fact := ec.entryShapeGuards[paramIdx]
		ec.asm.LDR(jit.X0, mRegRegs, slotOffset(paramIdx))
		jit.EmitCheckIsTableFull(ec.asm, jit.X0, jit.X16, jit.X17, failLabel)
		jit.EmitExtractPtr(ec.asm, jit.X0, jit.X0)
		ec.asm.CBZ(jit.X0, failLabel)
		ec.asm.LDRW(jit.X16, jit.X0, jit.TableOffShapeID)
		emitCMPWConst(ec.asm, jit.X16, jit.X17, int64(fact.ShapeID))
		ec.asm.BCond(jit.CondNE, failLabel)
	}
	ec.asm.B(doneLabel)
	ec.asm.Label(failLabel)
	ec.emitDeopt(nil)
	ec.asm.Label(doneLabel)
}

func branchTableShapeEqConst(instr *Instr) (int, uint32, bool) {
	if instr == nil || instr.Op != OpEqInt || len(instr.Args) < 2 || instr.Args[0] == nil || instr.Args[1] == nil {
		return 0, 0, false
	}
	if tableID, shapeID, ok := tableShapeConstPair(instr.Args[0].Def, instr.Args[1].Def); ok {
		return tableID, shapeID, true
	}
	return tableShapeConstPair(instr.Args[1].Def, instr.Args[0].Def)
}

func tableShapeConstPair(shapeDef, constDef *Instr) (int, uint32, bool) {
	if shapeDef == nil || constDef == nil || shapeDef.Op != OpTableShapeID || constDef.Op != OpConstInt || len(shapeDef.Args) == 0 || shapeDef.Args[0] == nil {
		return 0, 0, false
	}
	if constDef.Aux <= 0 || constDef.Aux > int64(^uint32(0)) {
		return 0, 0, false
	}
	return shapeDef.Args[0].ID, uint32(constDef.Aux), true
}

func emitBoxTablePtr(asm *jit.Assembler, dst, ptr, scratch jit.Reg) {
	asm.UBFX(dst, ptr, 0, 44)
	asm.LoadImm64(scratch, nb64(jit.NB_TagPtr))
	asm.ORRreg(dst, dst, scratch)
}
