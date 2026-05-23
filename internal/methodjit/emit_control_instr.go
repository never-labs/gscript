//go:build darwin && arm64

package methodjit

func (ec *emitContext) emitControlInstr(instr *Instr, block *Block) bool {
	switch instr.Op {
	case OpJump:
		ec.emitJump(instr, block)
	case OpBranch:
		ec.emitBranch(instr, block)
	case OpReturn:
		ec.emitReturn(instr, block)
	case OpTestSet:
		ec.emitOpExit(instr)
		ec.clearTableArrayBoundedKeys()
	default:
		return false
	}
	return true
}
