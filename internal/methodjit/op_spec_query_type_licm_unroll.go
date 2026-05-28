package methodjit

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

func opLICMLoopEffectRole(op Op) OpLICMLoopEffectRole {
	spec, ok := op.Spec()
	if !ok {
		return OpLICMLoopEffectNone
	}
	return spec.LICMLoopEffectRole
}

func isUnrollCloneableOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.UnrollCloneable
}
