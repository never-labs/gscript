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

func opNarrowsExactlyTo(op, narrowed Op) bool {
	out, ok := exactIntNarrowOp(op)
	return ok && out == narrowed
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

func loadElimConstCSE(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.LoadElimConstCSE
}

func loadElimPureCSE(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.LoadElimPureCSE
}

func loadElimShapeFactKiller(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.LoadElimShapeFactKiller
}

func loadElimFieldFactWideKiller(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.FieldFactWideKiller
}

func opIsCallLikeFactBarrier(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.CallLikeFactBarrier
}

func opIsTableMutationFirstArg(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.TableMutationFirstArg
}

func fieldLenFoldBarrier(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.FieldLenFoldBarrier
}

func fieldCallPolyLenFusionBarrier(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.FieldCallPolyLenFusionBarrier
}

func guardProvenByProducer(v *Value, guardType Type) bool {
	if v == nil || v.Def == nil || guardType == TypeUnknown {
		return false
	}
	spec, ok := v.Def.Op.Spec()
	return ok && spec.GuardProvenResultType == guardType
}

func fixedResultType(op Op) (Type, bool) {
	spec, ok := op.Spec()
	return spec.FixedResultType, ok && spec.FixedResultType != TypeUnknown
}

func typeSpecializedOp(op Op, lt, rt Type) (Op, Type, bool) {
	spec, ok := op.Spec()
	if !ok {
		return OpMax, TypeUnknown, false
	}
	if lt == TypeInt && rt == TypeInt && spec.TypeSpecializeIntOp < OpMax {
		return opSpecializedTarget(spec.TypeSpecializeIntOp)
	}
	if isNumericType(lt) && isNumericType(rt) && (lt == TypeFloat || rt == TypeFloat) && spec.TypeSpecializeFloatOp < OpMax {
		return opSpecializedTarget(spec.TypeSpecializeFloatOp)
	}
	if lt == TypeString && rt == TypeString && spec.TypeSpecializeStringOp < OpMax {
		return opSpecializedTarget(spec.TypeSpecializeStringOp)
	}
	return OpMax, TypeUnknown, false
}

func unaryTypeSpecializedOp(op Op, arg Type) (Op, Type, bool) {
	spec, ok := op.Spec()
	if !ok {
		return OpMax, TypeUnknown, false
	}
	if arg == TypeInt && spec.TypeSpecializeIntOp < OpMax {
		return opSpecializedTarget(spec.TypeSpecializeIntOp)
	}
	if arg == TypeFloat && spec.TypeSpecializeFloatOp < OpMax {
		return opSpecializedTarget(spec.TypeSpecializeFloatOp)
	}
	return OpMax, TypeUnknown, false
}

func opSpecializedTarget(op Op) (Op, Type, bool) {
	typ, ok := fixedResultType(op)
	if !ok {
		return OpMax, TypeUnknown, false
	}
	return op, typ, true
}

func isNumericType(t Type) bool {
	return t == TypeInt || t == TypeFloat
}

func instructionHasNoSSAResult(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.NoSSAResult
}

func hasSideEffect(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.KeepUnused
}

func needsFloatReg(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	if ok && spec.FloatRegResultBlocked {
		return false
	}
	if instr.Type == TypeFloat {
		return true
	}
	return ok && spec.FloatRegResult
}

func isRawIntOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawIntResult
}

func isRawTablePtrOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawTablePtrResult
}

func isRawTablePtrValue(instr *Instr) bool {
	if instr == nil {
		return false
	}
	if isRawTablePtrOp(instr.Op) {
		return true
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.TableResultRawTablePtr && instr.Type == TypeTable
}

func isRawDataPtrOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawDataPtrResult
}

func isRawFloatOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawFloatResult
}

func isMatrixNativeOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.MatrixNative
}

func opIsRawCarryClobber(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawCarryClobber
}

func isRawIntCarryValue(instr *Instr) bool {
	if instr == nil {
		return false
	}
	if instr.Type != TypeInt {
		return false
	}
	if isRawIntOp(instr.Op) {
		return true
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.RawIntCarryValue
}

func opIsGlobalConstUnsafe(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.GlobalConstUnsafe
}

func opMayCallOrRunConcurrently(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.MayCallOrRunConcurrently()
}

func crossBlockFieldSvalsGlobalBarrier(instr *Instr) bool {
	if instr == nil {
		return true
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.FieldSvalsCrossBlockBarrier
}

func opIsFieldSvalsGlobalBarrier(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.FieldSvalsGlobalBarrier
}

func valueProvenNonNil(v *Value) bool {
	if v == nil || v.Def == nil {
		return false
	}
	spec, ok := v.Def.Op.Spec()
	if ok && spec.ProvesNonNilResult {
		return true
	}
	return v.Def.Type == TypeInt || v.Def.Type == TypeFloat || v.Def.Type == TypeBool || v.Def.Type == TypeString || v.Def.Type == TypeTable
}

func isModuloReducibleCallFloor(instr *Instr) bool {
	if instr == nil || instr.Type != TypeInt {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.ModuloReducibleCallFloor
}

func isCallResultRangeGuardCandidate(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.CallResultRangeGuardCandidate
}

func opIsCallFloorSpecStableCallee(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.CallFloorSpecStableCallee
}

func opIsCallFloorSpecFieldShape(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.CallFloorSpecFieldShape
}

func opIsSpeculativeIntUseCandidate(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.SpeculativeIntUseCandidate
}

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
