//go:build darwin && arm64

package methodjit

func (ec *emitContext) emitGuardInstr(instr *Instr) bool {
	switch instr.Op {
	case OpGuardType:
		ec.emitGuardType(instr)
	case OpGuardIntRange:
		ec.emitGuardIntRange(instr)
	case OpGuardGlobalConst:
		ec.emitGuardGlobalConst(instr)
	case OpGuardConstString:
		ec.emitGuardConstString(instr)
	case OpGuardTableKind:
		ec.emitGuardTableKind(instr)
	case OpGuardCalleeProto:
		ec.emitGuardCalleeProto(instr)
	case OpGuardFieldCalleeProto:
		ec.emitGuardFieldCalleeProto(instr)
	case OpGuardShapeFieldType:
		ec.emitGuardShapeFieldType(instr)
	case OpGuardShapeFieldTypeMask:
		ec.emitGuardShapeFieldTypeMask(instr)
	case OpGuardShapeFieldVMClosure:
		ec.emitGuardShapeFieldVMClosure(instr)
	case OpGuardTruthy:
		ec.emitGuardTruthy(instr)
	case OpGuardNonNil:
		ec.emitOpExit(instr)
		ec.clearTableArrayBoundedKeys()
	default:
		return false
	}
	return true
}
