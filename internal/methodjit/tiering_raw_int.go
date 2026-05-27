//go:build darwin && arm64

package methodjit

func forceRawIntSpecializationIR(fn *Function) {
	if fn == nil || fn.Proto == nil {
		return
	}
	for {
		changed := false
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				switch instr.Op {
				case OpLoadSlot:
					if int(instr.Aux) < fn.Proto.NumParams && instr.Type != TypeInt {
						instr.Type = TypeInt
						changed = true
					}
				case OpConstInt:
					if instr.Type != TypeInt {
						instr.Type = TypeInt
						changed = true
					}
				case OpPhi:
					if instr.Type != TypeInt {
						instr.Type = TypeInt
						changed = true
					}
				default:
					spec, ok := instr.Op.Spec()
					if !ok || spec.RawIntSpecializedOp == Op(0) {
						continue
					}
					if allInstrArgsType(instr, TypeInt) {
						instr.Op = spec.RawIntSpecializedOp
						instr.Type = TypeInt
						changed = true
					}
					if instr.Op == OpEqInt || instr.Op == OpLtInt || instr.Op == OpLeInt {
						if instr.Type != TypeBool {
							instr.Type = TypeBool
						}
					}
				}
			}
		}
		if !changed {
			return
		}
	}
}

func firstResidualRawIntSpecializationGenericNumeric(fn *Function) (Op, bool) {
	gate := firstResidualRawIntSpecializationGenericNumericGate(fn)
	return gate.Op, !gate.Allowed
}

func firstResidualRawIntSpecializationGenericNumericGate(fn *Function) GateResult {
	if fn == nil {
		return allowGate("RawIntSpecializationIR", "no function")
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			spec, ok := instr.Op.Spec()
			if ok && spec.RawIntSpecializationBlocker {
				return blockGateOp("RawIntSpecializationIR", "raw-int specialization has residual generic numeric op", instr.Op)
			}
		}
	}
	return allowGate("RawIntSpecializationIR", "no residual generic numeric op")
}

func allInstrArgsType(instr *Instr, typ Type) bool {
	if instr == nil || len(instr.Args) == 0 {
		return false
	}
	for _, arg := range instr.Args {
		if arg == nil || arg.Def == nil || arg.Def.Type != typ {
			return false
		}
	}
	return true
}
