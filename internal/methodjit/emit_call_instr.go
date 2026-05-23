//go:build darwin && arm64

package methodjit

func (ec *emitContext) emitCallInstr(instr *Instr) bool {
	switch instr.Op {
	case OpCall:
		ec.emitOpCall(instr)
		ec.clearTableArrayBoundedKeys()
	case OpCallFloor:
		ec.emitOpCallFloor(instr)
		ec.clearTableArrayBoundedKeys()
	case OpFieldCallFloor:
		ec.emitOpFieldCallFloor(instr)
		ec.clearTableArrayBoundedKeys()
	case OpResume:
		ec.emitResumeExit(instr)
		ec.clearTableArrayBoundedKeys()
	case OpSelf:
		ec.emitOpExit(instr)
		ec.shapeVerified = make(map[int]uint32)
		ec.tableVerified = make(map[int]bool)
		ec.kindVerified = make(map[int]uint16)
		ec.keysDirtyWritten = make(map[int]bool)
		ec.clearTableArrayBoundedKeys()
		ec.dmVerified = make(map[int]bool)
	default:
		return false
	}
	return true
}
