//go:build darwin && arm64

package methodjit

func (ec *emitContext) emitArithmeticInstr(instr *Instr, _ *Block) bool {
	switch instr.Op {
	case OpAdd:
		ec.emitFloatBinOp(instr, intBinAdd)
	case OpSub:
		ec.emitFloatBinOp(instr, intBinSub)
	case OpMul:
		ec.emitFloatBinOp(instr, intBinMul)
	case OpMod:
		ec.emitFloatBinOp(instr, intBinMod)
	case OpAddInt:
		ec.emitRawIntBinOp(instr, intBinAdd)
	case OpSubInt:
		ec.emitRawIntBinOp(instr, intBinSub)
	case OpMulInt:
		ec.emitRawIntBinOp(instr, intBinMul)
	case OpModInt:
		ec.emitRawIntBinOp(instr, intBinMod)
	case OpDivIntExact:
		ec.emitRawIntExactDiv(instr)
	case OpAddFloat:
		ec.emitTypedFloatBinOp(instr, intBinAdd)
	case OpSubFloat:
		ec.emitTypedFloatBinOp(instr, intBinSub)
	case OpMulFloat:
		ec.emitTypedFloatBinOp(instr, intBinMul)
	case OpDiv, OpDivFloat:
		ec.emitDiv(instr)
	case OpUnm:
		ec.emitUnm(instr)
	case OpNegInt:
		ec.emitNegInt(instr)
	case OpNegFloat:
		ec.emitNegFloat(instr)
	case OpSqrt:
		ec.emitSqrtFloat(instr)
	case OpFloor:
		ec.emitFloor(instr)
	case OpFMA:
		ec.emitFMA(instr)
	case OpFMSUB:
		ec.emitFMSUB(instr)
	case OpNot:
		ec.emitNot(instr)
	case OpLen:
		ec.emitLenNative(instr)
		ec.clearTableArrayBoundedKeys()
	default:
		return false
	}
	return true
}
