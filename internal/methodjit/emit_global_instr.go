//go:build darwin && arm64

package methodjit

func (ec *emitContext) emitGlobalInstr(instr *Instr) bool {
	switch instr.Op {
	case OpGetGlobal:
		ec.emitGetGlobalNative(instr)
		ec.clearTableArrayBoundedKeys()
	case OpSetGlobal:
		ec.emitSetGlobalNative(instr)
		ec.clearTableArrayBoundedKeys()
	default:
		return false
	}
	return true
}
