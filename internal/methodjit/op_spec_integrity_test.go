package methodjit

import (
	"reflect"
	"testing"
)

func TestOpSpecPolicyTablesDoNotExceedOpSpace(t *testing.T) {
	tables := []struct {
		name  string
		table any
	}{
		{"opBackendPolicies", opBackendPolicies},
		{"opKeepUnusedPolicies", opKeepUnusedPolicies},
		{"opNativeReplayMayExitPolicies", opNativeReplayMayExitPolicies},
		{"opNativeReplayVisibleSideEffectPolicies", opNativeReplayVisibleSideEffectPolicies},
		{"opNativeReplayVisibleTableMutationPolicies", opNativeReplayVisibleTableMutationPolicies},
		{"opNativeCalleeResumeUnsafePolicies", opNativeCalleeResumeUnsafePolicies},
		{"opRestartVisibleSideEffectPolicies", opRestartVisibleSideEffectPolicies},
		{"opFieldShapeSplitInlineSafePolicies", opFieldShapeSplitInlineSafePolicies},
		{"opFieldShapePreEffectInlineSafePolicies", opFieldShapePreEffectInlineSafePolicies},
		{"opFieldShapeInlineSideEffectPolicies", opFieldShapeInlineSideEffectPolicies},
		{"opFieldShapePostEffectInlineUnsafePolicies", opFieldShapePostEffectInlineUnsafePolicies},
		{"opGlobalConstUnsafePolicies", opGlobalConstUnsafePolicies},
		{"opNestedCallLikePolicies", opNestedCallLikePolicies},
		{"opLoadElimConstCSEPolicies", opLoadElimConstCSEPolicies},
		{"opLiteralConstPolicies", opLiteralConstPolicies},
		{"opLoadElimPureCSEPolicies", opLoadElimPureCSEPolicies},
		{"opLoadElimShapeFactKillerPolicies", opLoadElimShapeFactKillerPolicies},
		{"opNoSSAResultPolicies", opNoSSAResultPolicies},
		{"opRawIntResultPolicies", opRawIntResultPolicies},
		{"opRawTablePtrResultPolicies", opRawTablePtrResultPolicies},
		{"opRawDataPtrResultPolicies", opRawDataPtrResultPolicies},
		{"opRawFloatResultPolicies", opRawFloatResultPolicies},
		{"opMatrixNativePolicies", opMatrixNativePolicies},
		{"opTableArrayGPRInvariantPolicies", opTableArrayGPRInvariantPolicies},
		{"opTableArrayGPRInvariantRankPolicies", opTableArrayGPRInvariantRankPolicies},
		{"opTableArrayGPRInvariantUseMaskPolicies", opTableArrayGPRInvariantUseMaskPolicies},
		{"opTableArrayKeyArgIndexPolicies", opTableArrayKeyArgIndexPolicies},
		{"opLICMHoistablePolicies", opLICMHoistablePolicies},
		{"opLICMInterestingMissPolicies", opLICMInterestingMissPolicies},
		{"opLICMIntArithPolicies", opLICMIntArithPolicies},
		{"opPureNumericInlinePolicies", opPureNumericInlinePolicies},
		{"opNativeEffectLoopInlinePolicies", opNativeEffectLoopInlinePolicies},
		{"opDirectDeoptWithoutFullFlushPolicies", opDirectDeoptWithoutFullFlushPolicies},
		{"opGenericSpecializablePolicies", opGenericSpecializablePolicies},
		{"opTypeSpecializationPolicies", opTypeSpecializationPolicies},
		{"opNumToFloatInsertCandidatePolicies", opNumToFloatInsertCandidatePolicies},
		{"opIntRecurrencePolicies", opIntRecurrencePolicies},
		{"opNumericOperandPolicies", opNumericOperandPolicies},
		{"opFieldSvalsCrossBlockBarrierPolicies", opFieldSvalsCrossBlockBarrierPolicies},
		{"opFieldSvalsGlobalBarrierPolicies", opFieldSvalsGlobalBarrierPolicies},
		{"opFieldLenFoldBarrierPolicies", opFieldLenFoldBarrierPolicies},
		{"opFieldCallPolyLenFusionBarrierPolicies", opFieldCallPolyLenFusionBarrierPolicies},
		{"opBoxableIntArithmeticPolicies", opBoxableIntArithmeticPolicies},
		{"opUnsafeIntArithmeticCandidatePolicies", opUnsafeIntArithmeticCandidatePolicies},
		{"opInt48SafeRangeCandidatePolicies", opInt48SafeRangeCandidatePolicies},
		{"opExactDivAllowedExternalUsePolicies", opExactDivAllowedExternalUsePolicies},
		{"opNonNegativeDerivationCandidatePolicies", opNonNegativeDerivationCandidatePolicies},
		{"opNonNegativeDerivationKindPolicies", opNonNegativeDerivationKindPolicies},
		{"opInt48RuntimeValuePolicies", opInt48RuntimeValuePolicies},
		{"opFusableComparisonPolicies", opFusableComparisonPolicies},
		{"opLoopBoundComparisonPolicies", opLoopBoundComparisonPolicies},
		{"opConstPoolUserPolicies", opConstPoolUserPolicies},
		{"opRawStringResultPolicies", opRawStringResultPolicies},
		{"opDynamicStringQueryCacheKeyPolicies", opDynamicStringQueryCacheKeyPolicies},
		{"opUnrollCloneablePolicies", opUnrollCloneablePolicies},
		{"opNestedFloatPhiOverrideSafePolicies", opNestedFloatPhiOverrideSafePolicies},
		{"opFloatReductionWideUnrollBarrierPolicies", opFloatReductionWideUnrollBarrierPolicies},
		{"opFloatReductionLatencyUnrollSeedPolicies", opFloatReductionLatencyUnrollSeedPolicies},
		{"opFloatReductionLatencyUnrollBlockPolicies", opFloatReductionLatencyUnrollBlockPolicies},
		{"opFloatReductionDivOpPolicies", opFloatReductionDivOpPolicies},
		{"opConstantPhiBranchThreadPurePolicies", opConstantPhiBranchThreadPurePolicies},
		{"opNeedsTier2FieldCachePolicies", opNeedsTier2FieldCachePolicies},
		{"opFieldReadPolicies", opFieldReadPolicies},
		{"opFieldSlotLoadPolicies", opFieldSlotLoadPolicies},
		{"opFieldWritePolicies", opFieldWritePolicies},
		{"opBoolTableFillBodyBenignPolicies", opBoolTableFillBodyBenignPolicies},
		{"opBoolTableFillStorePolicies", opBoolTableFillStorePolicies},
		{"opBoolTableCountLoadBodyBenignPolicies", opBoolTableCountLoadBodyBenignPolicies},
		{"opBoolTableCountLoadPolicies", opBoolTableCountLoadPolicies},
		{"opBoolTableCountIncrementBenignPolicies", opBoolTableCountIncrementBenignPolicies},
		{"opBoolTableCountIncrementPolicies", opBoolTableCountIncrementPolicies},
		{"opCallResultRangeGuardCandidatePolicies", opCallResultRangeGuardCandidatePolicies},
		{"opModuloReducibleCallFloorPolicies", opModuloReducibleCallFloorPolicies},
		{"opCallFloorSpecStableCalleePolicies", opCallFloorSpecStableCalleePolicies},
		{"opCallFloorSpecFieldShapePolicies", opCallFloorSpecFieldShapePolicies},
		{"opTier2LoopCallPolicies", opTier2LoopCallPolicies},
		{"opTier2LoopFeedbackVMProtoCallPolicies", opTier2LoopFeedbackVMProtoCallPolicies},
		{"opTier2ResidualCallBlockerPolicies", opTier2ResidualCallBlockerPolicies},
		{"opTier2LoopNativeCandidatePolicies", opTier2LoopNativeCandidatePolicies},
		{"opCallUserArgStartPolicies", opCallUserArgStartPolicies},
		{"opSpeculativeIntUseCandidatePolicies", opSpeculativeIntUseCandidatePolicies},
		{"opFloatRegResultPolicies", opFloatRegResultPolicies},
		{"opFloatRegResultBlockedPolicies", opFloatRegResultBlockedPolicies},
		{"opRawIntCarryValuePolicies", opRawIntCarryValuePolicies},
		{"opTableResultRawTablePtrPolicies", opTableResultRawTablePtrPolicies},
		{"opTableArrayRegionGlobalBarrierPolicies", opTableArrayRegionGlobalBarrierPolicies},
		{"opTableArrayRegionAliasingCallPolicies", opTableArrayRegionAliasingCallPolicies},
		{"opTableArrayRegionAliasingAlwaysPolicies", opTableArrayRegionAliasingAlwaysPolicies},
		{"opTableArrayRegionTableMutationPolicies", opTableArrayRegionTableMutationPolicies},
		{"opTableMetatableMutationBarrierPolicies", opTableMetatableMutationBarrierPolicies},
		{"opRuntimeOverflowBoxablePolicies", opRuntimeOverflowBoxablePolicies},
		{"opRuntimeGuardRefreshablePolicies", opRuntimeGuardRefreshablePolicies},
		{"opNativeNumericValueProducerPolicies", opNativeNumericValueProducerPolicies},
		{"opPureNumericUnknownValuePolicies", opPureNumericUnknownValuePolicies},
		{"opTableArraySwapPureBetweenPolicies", opTableArraySwapPureBetweenPolicies},
		{"opStaticTableLenBenignUsePolicies", opStaticTableLenBenignUsePolicies},
		{"opFixedResultTypePolicies", opFixedResultTypePolicies},
		{"opProvesNonNilResultPolicies", opProvesNonNilResultPolicies},
		{"opGuardProvenResultTypePolicies", opGuardProvenResultTypePolicies},
		{"opRawFloatValueProducerPolicies", opRawFloatValueProducerPolicies},
		{"opFieldFactWideKillerPolicies", opFieldFactWideKillerPolicies},
		{"opTableMutationFirstArgPolicies", opTableMutationFirstArgPolicies},
		{"opCallLikeFactBarrierPolicies", opCallLikeFactBarrierPolicies},
		{"opRawCarryClobberPolicies", opRawCarryClobberPolicies},
		{"opExactDivComponentPolicies", opExactDivComponentPolicies},
		{"opIntNarrowCandidatePolicies", opIntNarrowCandidatePolicies},
		{"opIntNarrowAllArgsConstraintPolicies", opIntNarrowAllArgsConstraintPolicies},
		{"opFieldNumFusionGapSafePolicies", opFieldNumFusionGapSafePolicies},
		{"opRawIntSpecializationBlockerPolicies", opRawIntSpecializationBlockerPolicies},
		{"opRawIntSpecializedOpPolicies", opRawIntSpecializedOpPolicies},
		{"opExactIntNarrowOpPolicies", opExactIntNarrowOpPolicies},
		{"opBoxedFallbackOpPolicies", opBoxedFallbackOpPolicies},
		{"opBoxedFallbackResultUnknownPolicies", opBoxedFallbackResultUnknownPolicies},
		{"opSourceFeedbackPolicies", opSourceFeedbackPolicies},
		{"opRangeRefineKindPolicies", opRangeRefineKindPolicies},
	}
	for _, table := range tables {
		if got := reflect.ValueOf(table.table).Len(); got > int(OpMax) {
			t.Fatalf("%s has length %d beyond OpMax %d", table.name, got, OpMax)
		}
	}
}

func TestOpSpecLookupAndTargetIntegrity(t *testing.T) {
	seenNames := make(map[string]Op, int(OpMax))
	for op := Op(0); op < OpMax; op++ {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		if prior, exists := seenNames[spec.Name]; exists {
			t.Fatalf("duplicate OpSpec name %q for %s and %s", spec.Name, prior, op)
		}
		seenNames[spec.Name] = op
		if got, ok := OpByName(spec.Name); !ok || got != op {
			t.Fatalf("OpByName(%q)=(%s,%v), want (%s,true)", spec.Name, got, ok, op)
		}
		assertOpSpecTarget(t, op, "TypeSpecializeIntOp", spec.TypeSpecializeIntOp)
		assertOpSpecTarget(t, op, "TypeSpecializeFloatOp", spec.TypeSpecializeFloatOp)
		assertOpSpecTarget(t, op, "TypeSpecializeStringOp", spec.TypeSpecializeStringOp)
		assertOpSpecTarget(t, op, "RawIntSpecializedOp", spec.RawIntSpecializedOp)
		assertOpSpecTarget(t, op, "ExactIntNarrowOp", spec.ExactIntNarrowOp)
		assertOpSpecTarget(t, op, "BoxedFallbackOp", spec.BoxedFallbackOp)
	}
	if len(seenNames) != int(OpMax) {
		t.Fatalf("OpSpec name lookup saw %d names, want %d", len(seenNames), OpMax)
	}
}

func assertOpSpecTarget(t *testing.T, owner Op, field string, target Op) {
	t.Helper()
	if target == 0 || target == OpMax {
		return
	}
	if target < 0 || target >= OpMax {
		t.Fatalf("%s.%s targets invalid op %d", owner, field, target)
	}
	if _, ok := target.Spec(); !ok {
		t.Fatalf("%s.%s targets op %d without OpSpec", owner, field, target)
	}
}
