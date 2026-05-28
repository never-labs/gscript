package methodjit

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

func shouldInsertNumToFloat(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NumToFloatInsertCandidate
}

func isNumericType(t Type) bool {
	return t == TypeInt || t == TypeFloat
}
