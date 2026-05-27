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
					op, ok := rawIntSpecializedOp(instr.Op)
					if !ok {
						continue
					}
					if allInstrArgsType(instr, TypeInt) {
						instr.Op = op
						instr.Type = TypeInt
						changed = true
					}
					if typ, ok := fixedResultType(instr.Op); ok {
						if instr.Type != typ {
							instr.Type = typ
							changed = true
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
			if opIsRawIntSpecializationBlocker(instr.Op) {
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
