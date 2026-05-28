package methodjit

func opIsConstantPhiBranchThreadPure(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.ConstantPhiBranchThreadPure
}

func opIsBoxableIntArithmetic(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.BoxableIntArithmetic
}

func boxedFallbackOp(op Op) (Op, bool) {
	spec, ok := op.Spec()
	return spec.BoxedFallbackOp, ok && spec.BoxedFallbackOp < OpMax
}

func opHasBoxedFallbackUnknownResult(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.BoxedFallbackResultUnknown
}

func opIsUnsafeIntArithmeticCandidate(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.UnsafeIntArithmeticCandidate
}

func nonNegativeDerivationKind(op Op) OpNonNegativeDerivationKind {
	spec, ok := op.Spec()
	if !ok {
		return OpNonNegativeNone
	}
	return spec.NonNegativeDerivationKind
}

func opIsFusableComparison(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FusableComparison
}

func opIsNativeNumericValueProducer(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NativeNumericValueProducer
}

func rawIntSpecializedOp(op Op) (Op, bool) {
	spec, ok := op.Spec()
	return spec.RawIntSpecializedOp, ok && spec.RawIntSpecializedOp != OpMax
}

func opIsRawIntSpecializationBlocker(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawIntSpecializationBlocker
}

func opIsExactDivComponent(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.ExactDivComponent
}

func opIsIntNarrowCandidate(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.IntNarrowCandidate
}

func opHasIntNarrowAllArgsConstraint(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.IntNarrowAllArgsConstraint
}

func opIsFloatReductionWideUnrollBarrier(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FloatReductionWideUnrollBarrier
}

func opIsFloatReductionLatencyUnrollSeed(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FloatReductionLatencyUnrollSeed
}

func opIsFloatReductionLatencyUnrollBlock(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FloatReductionLatencyUnrollBlock
}

func opIsFloatReductionDivOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FloatReductionDivOp
}

func opIsInt48SafeRangeCandidate(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.Int48SafeRangeCandidate
}

func opIsLoopBoundComparison(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.LoopBoundComparison
}

func opIsRawFloatValueProducer(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawFloatValueProducer
}
