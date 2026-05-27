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
