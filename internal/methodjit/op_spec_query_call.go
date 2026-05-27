package methodjit

func callUserArgs(instr *Instr) ([]*Value, bool) {
	if instr == nil {
		return nil, false
	}
	spec, ok := instr.Op.Spec()
	if !ok || spec.CallUserArgStart < 0 || len(instr.Args) < spec.CallUserArgStart {
		return nil, false
	}
	return instr.Args[spec.CallUserArgStart:], true
}

func callUserArgStart(op Op) (int, bool) {
	spec, ok := op.Spec()
	return spec.CallUserArgStart, ok && spec.CallUserArgStart >= 0
}

func sourceFeedbackPolicy(op Op) OpSourceFeedbackPolicy {
	spec, ok := op.Spec()
	if !ok {
		return OpSourceFeedbackNone
	}
	return spec.SourceFeedbackPolicy
}

func isPureNumericUnknownValue(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.PureNumericUnknownValue
}

func pureNumericInlineOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.PureNumericInline
}

func nativeEffectLoopInlineOp(op Op) bool {
	if pureNumericInlineOp(op) {
		return true
	}
	spec, ok := op.Spec()
	return ok && spec.NativeEffectLoopInline
}

func opCanDeriveNonNegative(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.NonNegativeDerivationCandidate
}

func isInt48RuntimeValue(instr *Instr) bool {
	if instr == nil || instr.Type != TypeInt {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.Int48RuntimeValue
}

func instrIsDirectIntValue(instr *Instr) bool {
	if instr == nil {
		return false
	}
	switch instr.Op {
	case OpConstInt, OpUnboxInt:
		return true
	case OpGuardType:
		return instr.Type == TypeInt || Type(instr.Aux) == TypeInt
	case OpGuardIntRange:
		return true
	default:
		return false
	}
}

func instrSatisfiesIntNarrowTypeConstraint(instr *Instr) bool {
	if instr == nil {
		return false
	}
	switch instr.Op {
	case OpConstInt, OpUnboxInt:
		return true
	case OpGuardType, OpGuardIntRange:
		return instr.Type == TypeInt || Type(instr.Aux) == TypeInt
	default:
		return false
	}
}
