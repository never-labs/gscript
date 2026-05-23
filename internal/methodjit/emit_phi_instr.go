//go:build darwin && arm64

package methodjit

func (ec *emitContext) emitPhiInstr(instr *Instr, _ *Block) bool {
	switch instr.Op {
	case OpPhi:
		// Phi resolution happens at block transitions (emitPhiMoves).
	default:
		return false
	}
	return true
}
