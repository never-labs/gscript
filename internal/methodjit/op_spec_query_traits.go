package methodjit

func opIsConstantPhiBranchThreadPure(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.ConstantPhiBranchThreadPure
}

func opIsBoolTableFillBodyBenign(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.BoolTableFillBodyBenign
}

func opIsBoolTableFillStore(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.BoolTableFillStore
}

func opIsBoolTableCountLoadBodyBenign(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.BoolTableCountLoadBodyBenign
}

func opIsBoolTableCountLoad(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.BoolTableCountLoad
}

func opIsBoolTableCountIncrementBenign(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.BoolTableCountIncrementBenign
}

func opIsBoolTableCountIncrement(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.BoolTableCountIncrement
}

func opIsFieldNumFusionGapSafe(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FieldNumFusionGapSafe
}

func fieldShapeSplitInlineOpSafe(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FieldShapeSplitInlineSafe
}

func opIsTableArraySwapPureBetween(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.TableArraySwapPureBetween
}

func opIsNestedCallLike(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NestedCallLike
}

func opIsTier2ResidualCallBlocker(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.Tier2ResidualCallBlocker
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

func opIsRuntimeOverflowBoxable(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RuntimeOverflowBoxable
}

func opIsRuntimeGuardRefreshable(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RuntimeGuardRefreshable
}

func opIsNativeCalleeResumeUnsafe(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NativeCalleeResumeUnsafe
}

func opIsNativeReplayVisibleTableMutation(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NativeReplayVisibleTableMutation
}

func opIsNativeReplayVisibleSideEffect(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NativeReplayVisibleSideEffect
}

func opIsNativeReplayMayExit(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NativeReplayMayExit
}

func opIsRestartVisibleSideEffect(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RestartVisibleSideEffect
}

func nonNegativeDerivationKind(op Op) OpNonNegativeDerivationKind {
	spec, ok := op.Spec()
	if !ok {
		return OpNonNegativeNone
	}
	return spec.NonNegativeDerivationKind
}

func opBackendPolicy(op Op) OpBackendPolicy {
	spec, ok := op.Spec()
	if !ok {
		return 0
	}
	return spec.BackendPolicy
}

func opIsFusableComparison(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FusableComparison
}

func opIsTier2LoopCall(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.Tier2LoopCall
}

func opIsTableArrayRegionGlobalBarrier(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.TableArrayRegionGlobalBarrier
}

func opIsTableArrayRegionAliasingCall(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.TableArrayRegionAliasingCall
}

func opIsTableArrayRegionAliasingAlways(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.TableArrayRegionAliasingAlways
}

func opIsTableArrayRegionTableMutation(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.TableArrayRegionTableMutation
}

func opIsNativeNumericValueProducer(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NativeNumericValueProducer
}

func opIsTier2LoopNativeCandidate(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.Tier2LoopNativeCandidate
}

func opIsTier2LoopFeedbackVMProtoCall(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.Tier2LoopFeedbackVMProtoCall
}

func rawIntSpecializedOp(op Op) (Op, bool) {
	spec, ok := op.Spec()
	return spec.RawIntSpecializedOp, ok && spec.RawIntSpecializedOp != Op(0)
}

func opIsRawIntSpecializationBlocker(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawIntSpecializationBlocker
}

func opIsTableArrayGPRInvariant(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.TableArrayGPRInvariant
}

func tableArrayGPRInvariantUseMask(op Op) uint8 {
	spec, ok := op.Spec()
	if !ok {
		return 0
	}
	return spec.TableArrayGPRInvariantUseMask
}

func tableArrayGPRInvariantRank(op Op) int {
	spec, ok := op.Spec()
	if !ok {
		return 1
	}
	return spec.TableArrayGPRInvariantRank
}

func opIsStaticTableLenBenignUse(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.StaticTableLenBenignUse
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

func opIsFieldShapePreEffectInlineSafe(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FieldShapePreEffectInlineSafe
}

func opIsFieldShapeInlineSideEffect(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FieldShapeInlineSideEffect
}

func opIsFieldShapePostEffectInlineUnsafe(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FieldShapePostEffectInlineUnsafe
}

func opNeedsTier2FieldCache(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NeedsTier2FieldCache
}

func opMayDirectDeoptWithoutFullFlush(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.DirectDeoptWithoutFullFlush
}

func tableArrayKeyArgIndex(op Op) (int, bool) {
	spec, ok := op.Spec()
	return spec.TableArrayKeyArgIndex, ok && spec.TableArrayKeyArgIndex >= 0
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

func opIsTableMetatableMutationBarrier(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.TableMetatableMutationBarrier
}

func instrUsesConstPool(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.ConstPoolUser
}

func opIsInt48SafeRangeCandidate(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.Int48SafeRangeCandidate
}

func opHasRawStringResult(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawStringResult
}

func opIsNestedFloatPhiOverrideSafe(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NestedFloatPhiOverrideSafe
}

func opIsLoopBoundComparison(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.LoopBoundComparison
}

func opIsDynamicStringQueryCacheKey(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.DynamicStringQueryCacheKey
}

func opIsRawFloatValueProducer(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawFloatValueProducer
}
