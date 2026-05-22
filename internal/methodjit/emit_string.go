//go:build darwin && arm64

package methodjit

func (ec *emitContext) emitStringInstr(instr *Instr) bool {
	switch instr.Op {
	case OpConcat:
		ec.emitConcatExit(instr)
		ec.clearTableArrayBoundedKeys()
	case OpStringConstLookup:
		ec.emitStringConstLookup(instr)
		ec.clearTableArrayBoundedKeys()
	case OpStringFormatInt:
		ec.emitStringFormatIntNative(instr)
		ec.clearTableArrayBoundedKeys()
	case OpStringFormatConst:
		ec.emitStringFormatConstNative(instr)
		ec.clearTableArrayBoundedKeys()
	case OpStringFormatConstLen:
		ec.emitStringFormatConstLenNative(instr)
		ec.clearTableArrayBoundedKeys()
	case OpGetTableStringFormatInt:
		ec.emitGetTableStringFormatIntNative(instr)
	case OpStringSplitPart:
		ec.emitStringSplitPartNative(instr)
		ec.clearTableArrayBoundedKeys()
	case OpStringSplitSubstr:
		ec.emitStringSplitSubstrNative(instr)
		ec.clearTableArrayBoundedKeys()
	case OpStringSplitSubstrNumber:
		ec.emitStringSplitSubstrNumberNative(instr)
		ec.clearTableArrayBoundedKeys()
	default:
		return false
	}
	return true
}
