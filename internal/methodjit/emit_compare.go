//go:build darwin && arm64

package methodjit

import "github.com/Never-Labs/gscript/internal/jit"

func (ec *emitContext) emitCompareInstr(instr *Instr, _ *Block) bool {
	switch instr.Op {
	case OpLt:
		if !ec.emitTypedStringCmp(instr, jit.CondLT) {
			ec.emitGenericNumericCmp(instr, jit.CondLT)
		}
	case OpLe:
		if !ec.emitTypedStringCmp(instr, jit.CondLE) {
			ec.emitGenericNumericCmp(instr, jit.CondLE)
		}
	case OpEq:
		ec.emitGenericNumericCmp(instr, jit.CondEQ)
	case OpLtInt:
		ec.emitIntCmp(instr, jit.CondLT)
	case OpLeInt:
		ec.emitIntCmp(instr, jit.CondLE)
	case OpEqInt:
		ec.emitIntCmp(instr, jit.CondEQ)
	case OpEqString:
		ec.emitStringEqCmp(instr)
	case OpModZeroInt:
		ec.emitModZeroInt(instr)
	case OpLtFloat:
		ec.emitFloatCmp(instr, jit.CondLT)
	case OpLeFloat:
		ec.emitFloatCmp(instr, jit.CondLE)
	default:
		return false
	}
	return true
}
