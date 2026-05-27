package methodjit

func applyOpSpecTypePolicies(op Op, spec *OpSpec) {
	applyOpSpecTypeRuntimePolicies(op, spec)
	applyOpSpecTypeResultPolicies(op, spec)
	applyOpSpecTypeBarrierPolicies(op, spec)
	applyOpSpecTypeNarrowPolicies(op, spec)
}

func applyOpSpecTypeRuntimePolicies(op Op, spec *OpSpec) {
	if int(op) < len(opRuntimeOverflowBoxablePolicies) {
		spec.RuntimeOverflowBoxable = opRuntimeOverflowBoxablePolicies[op]
	}
	if int(op) < len(opRuntimeGuardRefreshablePolicies) {
		spec.RuntimeGuardRefreshable = opRuntimeGuardRefreshablePolicies[op]
	}
	if int(op) < len(opNativeNumericValueProducerPolicies) {
		spec.NativeNumericValueProducer = opNativeNumericValueProducerPolicies[op]
	}
	if int(op) < len(opPureNumericUnknownValuePolicies) {
		spec.PureNumericUnknownValue = opPureNumericUnknownValuePolicies[op]
	}
	if int(op) < len(opTableArraySwapPureBetweenPolicies) {
		spec.TableArraySwapPureBetween = opTableArraySwapPureBetweenPolicies[op]
	}
	if int(op) < len(opStaticTableLenBenignUsePolicies) {
		spec.StaticTableLenBenignUse = opStaticTableLenBenignUsePolicies[op]
	}
}

func applyOpSpecTypeResultPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opFixedResultTypePolicies) {
		spec.FixedResultType = opFixedResultTypePolicies[op]
	}
	if int(op) < len(opProvesNonNilResultPolicies) {
		spec.ProvesNonNilResult = opProvesNonNilResultPolicies[op]
	}
	if int(op) < len(opGuardProvenResultTypePolicies) {
		spec.GuardProvenResultType = opGuardProvenResultTypePolicies[op]
	}
	if int(op) < len(opRawFloatValueProducerPolicies) {
		spec.RawFloatValueProducer = opRawFloatValueProducerPolicies[op]
	}
}

func applyOpSpecTypeBarrierPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opFieldFactWideKillerPolicies) {
		spec.FieldFactWideKiller = opFieldFactWideKillerPolicies[op]
	}
	if int(op) < len(opTableMutationFirstArgPolicies) {
		spec.TableMutationFirstArg = opTableMutationFirstArgPolicies[op]
	}
	if int(op) < len(opCallLikeFactBarrierPolicies) {
		spec.CallLikeFactBarrier = opCallLikeFactBarrierPolicies[op]
	}
	if int(op) < len(opRawCarryClobberPolicies) {
		spec.RawCarryClobber = opRawCarryClobberPolicies[op]
	}
}

func applyOpSpecTypeNarrowPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opExactDivComponentPolicies) {
		spec.ExactDivComponent = opExactDivComponentPolicies[op]
	}
	if int(op) < len(opIntNarrowCandidatePolicies) {
		spec.IntNarrowCandidate = opIntNarrowCandidatePolicies[op]
	}
	if int(op) < len(opIntNarrowAllArgsConstraintPolicies) {
		spec.IntNarrowAllArgsConstraint = opIntNarrowAllArgsConstraintPolicies[op]
	}
	if int(op) < len(opFieldNumFusionGapSafePolicies) {
		spec.FieldNumFusionGapSafe = opFieldNumFusionGapSafePolicies[op]
	}
	if int(op) < len(opRawIntSpecializationBlockerPolicies) {
		spec.RawIntSpecializationBlocker = opRawIntSpecializationBlockerPolicies[op]
	}
	if int(op) < len(opRawIntSpecializedOpPolicies) {
		spec.RawIntSpecializedOp = opRawIntSpecializedOpPolicies[op]
	}
	if int(op) < len(opExactIntNarrowOpPolicies) && opExactIntNarrowOpPolicies[op].Set {
		spec.ExactIntNarrowOp = opExactIntNarrowOpPolicies[op].Op
	}
	if int(op) < len(opBoxedFallbackOpPolicies) && opBoxedFallbackOpPolicies[op].Set {
		spec.BoxedFallbackOp = opBoxedFallbackOpPolicies[op].Op
	}
	if int(op) < len(opBoxedFallbackResultUnknownPolicies) {
		spec.BoxedFallbackResultUnknown = opBoxedFallbackResultUnknownPolicies[op]
	}
	if int(op) < len(opSourceFeedbackPolicies) {
		spec.SourceFeedbackPolicy = opSourceFeedbackPolicies[op]
	}
	if int(op) < len(opRangeRefineKindPolicies) {
		spec.RangeRefineKind = opRangeRefineKindPolicies[op]
	}
}
