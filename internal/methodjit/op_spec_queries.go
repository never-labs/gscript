package methodjit

func opIsFieldRead(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FieldRead
}

func opIsFieldSlotLoad(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FieldSlotLoad
}

func opIsFieldWrite(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FieldWrite
}

func opIsLiteralConst(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.LiteralConst
}

func opIsBoxedOrFallback(op, boxed Op) bool {
	if op == boxed {
		return true
	}
	spec, ok := op.Spec()
	return ok && spec.BoxedFallbackOp == boxed
}

func orderedRangeRefineKind(op Op) (strict bool, ok bool) {
	spec, specOK := op.Spec()
	if !specOK {
		return false, false
	}
	switch spec.RangeRefineKind {
	case OpRangeRefineLessThan:
		return true, true
	case OpRangeRefineLessEqual:
		return false, true
	default:
		return false, false
	}
}

func exactIntNarrowOp(op Op) (Op, bool) {
	spec, ok := op.Spec()
	return spec.ExactIntNarrowOp, ok && spec.ExactIntNarrowOp < OpMax
}

func isGenericSpecializableOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.GenericSpecializable
}

func isIntRecurrenceOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.IntRecurrence
}

func isNumericOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NumericOperand
}

func canHoistOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.LICMHoistable
}

func isInterestingLICMMiss(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.LICMInterestingMiss
}

func isIntArithOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.LICMIntArith
}

func isUnrollCloneableOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.UnrollCloneable
}

func shouldInsertNumToFloat(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NumToFloatInsertCandidate
}

func isExactDivAllowedExternalUse(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.ExactDivAllowedExternalUse
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
