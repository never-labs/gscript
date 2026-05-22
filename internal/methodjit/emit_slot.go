//go:build darwin && arm64

package methodjit

func (ec *emitContext) emitSlotInstr(instr *Instr, _ *Block) bool {
	switch instr.Op {
	case OpLoadSlot:
		ec.emitLoadSlot(instr)
	case OpStoreSlot:
		ec.emitStoreSlot(instr)
	default:
		return false
	}
	return true
}
