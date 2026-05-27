package methodjit

func buildExpandedOpSpecs() [OpMax]OpSpec {
	var out [OpMax]OpSpec
	for op := Op(0); op < OpMax; op++ {
		if spec, ok := buildOpSpec(op); ok {
			out[op] = spec
		}
	}
	return out
}

func (op Op) Spec() (OpSpec, bool) {
	if int(op) < len(expandedOpSpecs) && expandedOpSpecs[op].Name != "" {
		return expandedOpSpecs[op], true
	}
	return OpSpec{}, false
}

func buildOpSpec(op Op) (OpSpec, bool) {
	if int(op) < len(opSpecs) && opSpecs[op].Name != "" {
		spec := opSpecs[op]
		if int(op) < len(opBackendPolicies) {
			spec.BackendPolicy = opBackendPolicies[op]
		}
		if int(op) < len(opKeepUnusedPolicies) {
			spec.KeepUnused = opKeepUnusedPolicies[op]
		}
		if int(op) < len(opNativeReplayMayExitPolicies) {
			spec.NativeReplayMayExit = opNativeReplayMayExitPolicies[op]
		}
		if int(op) < len(opNativeReplayVisibleSideEffectPolicies) {
			spec.NativeReplayVisibleSideEffect = opNativeReplayVisibleSideEffectPolicies[op]
		}
		if int(op) < len(opNativeReplayVisibleTableMutationPolicies) {
			spec.NativeReplayVisibleTableMutation = opNativeReplayVisibleTableMutationPolicies[op]
		}
		if int(op) < len(opNativeCalleeResumeUnsafePolicies) {
			spec.NativeCalleeResumeUnsafe = opNativeCalleeResumeUnsafePolicies[op]
		}
		if int(op) < len(opRestartVisibleSideEffectPolicies) {
			spec.RestartVisibleSideEffect = opRestartVisibleSideEffectPolicies[op]
		}
		if int(op) < len(opFieldShapeSplitInlineSafePolicies) {
			spec.FieldShapeSplitInlineSafe = opFieldShapeSplitInlineSafePolicies[op]
		}
		if int(op) < len(opFieldShapePreEffectInlineSafePolicies) {
			spec.FieldShapePreEffectInlineSafe = opFieldShapePreEffectInlineSafePolicies[op]
		}
		if int(op) < len(opFieldShapeInlineSideEffectPolicies) {
			spec.FieldShapeInlineSideEffect = opFieldShapeInlineSideEffectPolicies[op]
		}
		if int(op) < len(opFieldShapePostEffectInlineUnsafePolicies) {
			spec.FieldShapePostEffectInlineUnsafe = opFieldShapePostEffectInlineUnsafePolicies[op]
		}
		if int(op) < len(opGlobalConstUnsafePolicies) {
			spec.GlobalConstUnsafe = opGlobalConstUnsafePolicies[op]
		}
		if int(op) < len(opNestedCallLikePolicies) {
			spec.NestedCallLike = opNestedCallLikePolicies[op]
		}
		if int(op) < len(opLoadElimConstCSEPolicies) {
			spec.LoadElimConstCSE = opLoadElimConstCSEPolicies[op]
		}
		if int(op) < len(opLiteralConstPolicies) {
			spec.LiteralConst = opLiteralConstPolicies[op]
		}
		if int(op) < len(opLoadElimPureCSEPolicies) {
			spec.LoadElimPureCSE = opLoadElimPureCSEPolicies[op]
		}
		if int(op) < len(opLoadElimShapeFactKillerPolicies) {
			spec.LoadElimShapeFactKiller = opLoadElimShapeFactKillerPolicies[op]
		}
		if int(op) < len(opNoSSAResultPolicies) {
			spec.NoSSAResult = opNoSSAResultPolicies[op]
		}
		if int(op) < len(opRawIntResultPolicies) {
			spec.RawIntResult = opRawIntResultPolicies[op]
		}
		if int(op) < len(opRawTablePtrResultPolicies) {
			spec.RawTablePtrResult = opRawTablePtrResultPolicies[op]
		}
		if int(op) < len(opRawDataPtrResultPolicies) {
			spec.RawDataPtrResult = opRawDataPtrResultPolicies[op]
		}
		if int(op) < len(opRawFloatResultPolicies) {
			spec.RawFloatResult = opRawFloatResultPolicies[op]
		}
		if int(op) < len(opMatrixNativePolicies) {
			spec.MatrixNative = opMatrixNativePolicies[op]
		}
		if int(op) < len(opTableArrayGPRInvariantPolicies) {
			spec.TableArrayGPRInvariant = opTableArrayGPRInvariantPolicies[op]
		}
		if int(op) < len(opTableArrayGPRInvariantRankPolicies) && opTableArrayGPRInvariantRankPolicies[op] != 0 {
			spec.TableArrayGPRInvariantRank = int(opTableArrayGPRInvariantRankPolicies[op]) - 1
		}
		if int(op) < len(opTableArrayGPRInvariantUseMaskPolicies) {
			spec.TableArrayGPRInvariantUseMask = opTableArrayGPRInvariantUseMaskPolicies[op]
		}
		if int(op) < len(opTableArrayKeyArgIndexPolicies) && opTableArrayKeyArgIndexPolicies[op] != 0 {
			spec.TableArrayKeyArgIndex = int(opTableArrayKeyArgIndexPolicies[op]) - 1
		}
		if int(op) < len(opLICMHoistablePolicies) {
			spec.LICMHoistable = opLICMHoistablePolicies[op]
		}
		if int(op) < len(opLICMInterestingMissPolicies) {
			spec.LICMInterestingMiss = opLICMInterestingMissPolicies[op]
		}
		if int(op) < len(opLICMIntArithPolicies) {
			spec.LICMIntArith = opLICMIntArithPolicies[op]
		}
		if int(op) < len(opPureNumericInlinePolicies) {
			spec.PureNumericInline = opPureNumericInlinePolicies[op]
		}
		if int(op) < len(opNativeEffectLoopInlinePolicies) {
			spec.NativeEffectLoopInline = opNativeEffectLoopInlinePolicies[op]
		}
		if int(op) < len(opDirectDeoptWithoutFullFlushPolicies) {
			spec.DirectDeoptWithoutFullFlush = opDirectDeoptWithoutFullFlushPolicies[op]
		}
		if int(op) < len(opGenericSpecializablePolicies) {
			spec.GenericSpecializable = opGenericSpecializablePolicies[op]
		}
		if int(op) < len(opTypeSpecializationPolicies) && opTypeSpecializationPolicies[op].Set {
			policy := opTypeSpecializationPolicies[op]
			spec.TypeSpecializeIntOp = policy.IntOp
			spec.TypeSpecializeFloatOp = policy.FloatOp
			spec.TypeSpecializeStringOp = policy.StringOp
		}
		if int(op) < len(opNumToFloatInsertCandidatePolicies) {
			spec.NumToFloatInsertCandidate = opNumToFloatInsertCandidatePolicies[op]
		}
		if int(op) < len(opIntRecurrencePolicies) {
			spec.IntRecurrence = opIntRecurrencePolicies[op]
		}
		if int(op) < len(opNumericOperandPolicies) {
			spec.NumericOperand = opNumericOperandPolicies[op]
		}
		if int(op) < len(opFieldSvalsCrossBlockBarrierPolicies) {
			spec.FieldSvalsCrossBlockBarrier = opFieldSvalsCrossBlockBarrierPolicies[op]
		}
		if int(op) < len(opFieldSvalsGlobalBarrierPolicies) {
			spec.FieldSvalsGlobalBarrier = opFieldSvalsGlobalBarrierPolicies[op]
		}
		if int(op) < len(opFieldLenFoldBarrierPolicies) {
			spec.FieldLenFoldBarrier = opFieldLenFoldBarrierPolicies[op]
		}
		if int(op) < len(opFieldCallPolyLenFusionBarrierPolicies) {
			spec.FieldCallPolyLenFusionBarrier = opFieldCallPolyLenFusionBarrierPolicies[op]
		}
		if int(op) < len(opBoxableIntArithmeticPolicies) {
			spec.BoxableIntArithmetic = opBoxableIntArithmeticPolicies[op]
		}
		if int(op) < len(opUnsafeIntArithmeticCandidatePolicies) {
			spec.UnsafeIntArithmeticCandidate = opUnsafeIntArithmeticCandidatePolicies[op]
		}
		if int(op) < len(opInt48SafeRangeCandidatePolicies) {
			spec.Int48SafeRangeCandidate = opInt48SafeRangeCandidatePolicies[op]
		}
		if int(op) < len(opExactDivAllowedExternalUsePolicies) {
			spec.ExactDivAllowedExternalUse = opExactDivAllowedExternalUsePolicies[op]
		}
		if int(op) < len(opNonNegativeDerivationCandidatePolicies) {
			spec.NonNegativeDerivationCandidate = opNonNegativeDerivationCandidatePolicies[op]
		}
		if int(op) < len(opNonNegativeDerivationKindPolicies) {
			spec.NonNegativeDerivationKind = opNonNegativeDerivationKindPolicies[op]
			spec.NonNegativeDerivationCandidate = spec.NonNegativeDerivationCandidate || spec.NonNegativeDerivationKind != OpNonNegativeNone
		}
		if int(op) < len(opInt48RuntimeValuePolicies) {
			spec.Int48RuntimeValue = opInt48RuntimeValuePolicies[op]
		}
		if int(op) < len(opFusableComparisonPolicies) {
			spec.FusableComparison = opFusableComparisonPolicies[op]
		}
		if int(op) < len(opLoopBoundComparisonPolicies) {
			spec.LoopBoundComparison = opLoopBoundComparisonPolicies[op]
		}
		if int(op) < len(opConstPoolUserPolicies) {
			spec.ConstPoolUser = opConstPoolUserPolicies[op]
		}
		if int(op) < len(opRawStringResultPolicies) {
			spec.RawStringResult = opRawStringResultPolicies[op]
		}
		if int(op) < len(opDynamicStringQueryCacheKeyPolicies) {
			spec.DynamicStringQueryCacheKey = opDynamicStringQueryCacheKeyPolicies[op]
		}
		if int(op) < len(opUnrollCloneablePolicies) {
			spec.UnrollCloneable = opUnrollCloneablePolicies[op]
		}
		if int(op) < len(opNestedFloatPhiOverrideSafePolicies) {
			spec.NestedFloatPhiOverrideSafe = opNestedFloatPhiOverrideSafePolicies[op]
		}
		if int(op) < len(opFloatReductionWideUnrollBarrierPolicies) {
			spec.FloatReductionWideUnrollBarrier = opFloatReductionWideUnrollBarrierPolicies[op]
		}
		if int(op) < len(opFloatReductionLatencyUnrollSeedPolicies) {
			spec.FloatReductionLatencyUnrollSeed = opFloatReductionLatencyUnrollSeedPolicies[op]
		}
		if int(op) < len(opFloatReductionLatencyUnrollBlockPolicies) {
			spec.FloatReductionLatencyUnrollBlock = opFloatReductionLatencyUnrollBlockPolicies[op]
		}
		if int(op) < len(opFloatReductionDivOpPolicies) {
			spec.FloatReductionDivOp = opFloatReductionDivOpPolicies[op]
		}
		if int(op) < len(opConstantPhiBranchThreadPurePolicies) {
			spec.ConstantPhiBranchThreadPure = opConstantPhiBranchThreadPurePolicies[op]
		}
		if int(op) < len(opNeedsTier2FieldCachePolicies) {
			spec.NeedsTier2FieldCache = opNeedsTier2FieldCachePolicies[op]
		}
		if int(op) < len(opFieldReadPolicies) {
			spec.FieldRead = opFieldReadPolicies[op]
		}
		if int(op) < len(opFieldSlotLoadPolicies) {
			spec.FieldSlotLoad = opFieldSlotLoadPolicies[op]
		}
		if int(op) < len(opFieldWritePolicies) {
			spec.FieldWrite = opFieldWritePolicies[op]
		}
		if int(op) < len(opBoolTableFillBodyBenignPolicies) {
			spec.BoolTableFillBodyBenign = opBoolTableFillBodyBenignPolicies[op]
		}
		if int(op) < len(opBoolTableFillStorePolicies) {
			spec.BoolTableFillStore = opBoolTableFillStorePolicies[op]
		}
		if int(op) < len(opBoolTableCountLoadBodyBenignPolicies) {
			spec.BoolTableCountLoadBodyBenign = opBoolTableCountLoadBodyBenignPolicies[op]
		}
		if int(op) < len(opBoolTableCountLoadPolicies) {
			spec.BoolTableCountLoad = opBoolTableCountLoadPolicies[op]
		}
		if int(op) < len(opBoolTableCountIncrementBenignPolicies) {
			spec.BoolTableCountIncrementBenign = opBoolTableCountIncrementBenignPolicies[op]
		}
		if int(op) < len(opBoolTableCountIncrementPolicies) {
			spec.BoolTableCountIncrement = opBoolTableCountIncrementPolicies[op]
		}
		if int(op) < len(opCallResultRangeGuardCandidatePolicies) {
			spec.CallResultRangeGuardCandidate = opCallResultRangeGuardCandidatePolicies[op]
		}
		if int(op) < len(opModuloReducibleCallFloorPolicies) {
			spec.ModuloReducibleCallFloor = opModuloReducibleCallFloorPolicies[op]
		}
		if int(op) < len(opCallFloorSpecStableCalleePolicies) {
			spec.CallFloorSpecStableCallee = opCallFloorSpecStableCalleePolicies[op]
		}
		if int(op) < len(opCallFloorSpecFieldShapePolicies) {
			spec.CallFloorSpecFieldShape = opCallFloorSpecFieldShapePolicies[op]
		}
		if int(op) < len(opTier2LoopCallPolicies) {
			spec.Tier2LoopCall = opTier2LoopCallPolicies[op]
		}
		if int(op) < len(opTier2LoopFeedbackVMProtoCallPolicies) {
			spec.Tier2LoopFeedbackVMProtoCall = opTier2LoopFeedbackVMProtoCallPolicies[op]
		}
		if int(op) < len(opTier2ResidualCallBlockerPolicies) {
			spec.Tier2ResidualCallBlocker = opTier2ResidualCallBlockerPolicies[op]
		}
		if int(op) < len(opTier2LoopNativeCandidatePolicies) {
			spec.Tier2LoopNativeCandidate = opTier2LoopNativeCandidatePolicies[op]
		}
		if int(op) < len(opCallUserArgStartPolicies) && opCallUserArgStartPolicies[op].Set {
			spec.CallUserArgStart = opCallUserArgStartPolicies[op].Start
		}
		if int(op) < len(opSpeculativeIntUseCandidatePolicies) {
			spec.SpeculativeIntUseCandidate = opSpeculativeIntUseCandidatePolicies[op]
		}
		if int(op) < len(opFloatRegResultPolicies) {
			spec.FloatRegResult = opFloatRegResultPolicies[op]
		}
		if int(op) < len(opFloatRegResultBlockedPolicies) {
			spec.FloatRegResultBlocked = opFloatRegResultBlockedPolicies[op]
		}
		if int(op) < len(opRawIntCarryValuePolicies) {
			spec.RawIntCarryValue = opRawIntCarryValuePolicies[op]
		}
		if int(op) < len(opTableResultRawTablePtrPolicies) {
			spec.TableResultRawTablePtr = opTableResultRawTablePtrPolicies[op]
		}
		if int(op) < len(opTableArrayRegionGlobalBarrierPolicies) {
			spec.TableArrayRegionGlobalBarrier = opTableArrayRegionGlobalBarrierPolicies[op]
		}
		if int(op) < len(opTableArrayRegionAliasingCallPolicies) {
			spec.TableArrayRegionAliasingCall = opTableArrayRegionAliasingCallPolicies[op]
		}
		if int(op) < len(opTableArrayRegionAliasingAlwaysPolicies) {
			spec.TableArrayRegionAliasingAlways = opTableArrayRegionAliasingAlwaysPolicies[op]
		}
		if int(op) < len(opTableArrayRegionTableMutationPolicies) {
			spec.TableArrayRegionTableMutation = opTableArrayRegionTableMutationPolicies[op]
		}
		if int(op) < len(opTableMetatableMutationBarrierPolicies) {
			spec.TableMetatableMutationBarrier = opTableMetatableMutationBarrierPolicies[op]
		}
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
		return spec, true
	}
	return OpSpec{}, false
}

func OpByName(name string) (Op, bool) {
	op, ok := opNameLookup[name]
	return op, ok
}

func (spec OpSpec) MayCallOrRunConcurrently() bool {
	return spec.SideEffect == OpSideEffectCall || spec.SideEffect == OpSideEffectConcurrency
}

func OpsByEmitterFamily(family OpEmitterFamily) []Op {
	var ops []Op
	for op := Op(0); op < OpMax; op++ {
		spec := expandedOpSpecs[op]
		if spec.Name == "" || spec.EmitterFamily != family {
			continue
		}
		ops = append(ops, op)
	}
	return ops
}
