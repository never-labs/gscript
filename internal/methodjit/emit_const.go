//go:build darwin && arm64

package methodjit

func (ec *emitContext) emitConstInstr(instr *Instr, _ *Block) bool {
	switch instr.Op {
	case OpConstInt:
		ec.emitConstInt(instr)
	case OpConstNil:
		ec.emitConstNil(instr)
	case OpConstBool:
		ec.emitConstBool(instr)
	case OpConstFloat:
		ec.emitConstFloat(instr)
	case OpConstString:
		ec.emitConstString(instr)
	default:
		return false
	}
	return true
}
