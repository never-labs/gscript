//go:build darwin && arm64

package methodjit

import (
	"github.com/Never-Labs/gscript/internal/jit"
)

func (ec *emitContext) emitSetRawSelfRegsEndFromMRegRegs() {
	if ec.numericParamCount < 2 {
		return
	}
	ec.emitSetRawSelfRegsEnd(mRegRegs, ec.nextSlot, jit.X16, jit.X17)
}

func (ec *emitContext) emitSetRawSelfRegsEnd(baseReg jit.Reg, numRegs int, scratchActual, scratchBudget jit.Reg) {
	if numRegs <= 0 {
		return
	}
	asm := ec.asm
	useActualLabel := ec.uniqueLabel("rawself_regsend_actual")
	doneLabel := ec.uniqueLabel("rawself_regsend_done")
	budgetBytes := numRegs * (maxRawSelfCallDepth + 1) * jit.ValueSize

	asm.LDR(scratchActual, mRegCtx, execCtxOffRegsEnd)
	if budgetBytes <= 4095 {
		asm.ADDimm(scratchBudget, baseReg, uint16(budgetBytes))
	} else {
		asm.LoadImm64(scratchBudget, int64(budgetBytes))
		asm.ADDreg(scratchBudget, baseReg, scratchBudget)
	}
	asm.CMPreg(scratchBudget, scratchActual)
	asm.BCond(jit.CondHI, useActualLabel)
	asm.STR(scratchBudget, mRegCtx, execCtxOffRawSelfRegsEnd)
	asm.B(doneLabel)
	asm.Label(useActualLabel)
	asm.STR(scratchActual, mRegCtx, execCtxOffRawSelfRegsEnd)
	asm.Label(doneLabel)
}

func (ec *emitContext) typedSelfEntryParamLoads(block *Block) map[int]bool {
	if ec == nil || ec.numericMode || !ec.typedSelfABI.Eligible || ec.fn == nil || block == nil || block != ec.fn.Entry {
		return nil
	}
	remaining := make(map[int]bool, ec.typedSelfABI.NumParams)
	for i := 0; i < ec.typedSelfABI.NumParams; i++ {
		remaining[i] = true
	}
	if len(remaining) == 0 {
		return nil
	}
	pending := make(map[int]bool, len(remaining))
	for _, instr := range block.Instrs {
		if instr.Op != OpLoadSlot {
			break
		}
		slot := int(instr.Aux)
		if !remaining[slot] {
			return nil
		}
		pending[slot] = true
		delete(remaining, slot)
		if len(remaining) == 0 {
			return pending
		}
	}
	return nil
}

func (ec *emitContext) entryParamLoad(slot int) (*Instr, bool) {
	if ec == nil || ec.fn == nil || ec.fn.Entry == nil {
		return nil, false
	}
	for _, instr := range ec.fn.Entry.Instrs {
		if instr.Op == OpLoadSlot && int(instr.Aux) == slot {
			return instr, true
		}
	}
	return nil, false
}

func (ec *emitContext) typedSelfSavedRegs() ([]int, []int) {
	if ec == nil || ec.alloc == nil {
		return []int{19, 20, 21, 22, 23, 24, 25, 26, 27, 28}, nil
	}
	gprs := typedPeerAllocatedCalleeSavedGPRs(ec.alloc)
	fprs := typedPeerAllocatedCalleeSavedFPRs(ec.alloc)
	return gprs, fprs
}

func (ec *emitContext) typedSelfFrameSize() int {
	gprs, fprs := ec.typedSelfSavedRegs()
	return typedPeerActualFrameBytes(gprs, fprs)
}
