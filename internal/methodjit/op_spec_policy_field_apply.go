package methodjit

func applyOpSpecFieldPolicies(op Op, spec *OpSpec) {
	applyOpSpecFieldInlinePolicies(op, spec)
	applyOpSpecFieldGlobalPolicies(op, spec)
	applyOpSpecFieldLoadPolicies(op, spec)
}

func applyOpSpecFieldInlinePolicies(op Op, spec *OpSpec) {
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
}

func applyOpSpecFieldGlobalPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opGlobalConstUnsafePolicies) {
		spec.GlobalConstUnsafe = opGlobalConstUnsafePolicies[op]
	}
	if int(op) < len(opNestedCallLikePolicies) {
		spec.NestedCallLike = opNestedCallLikePolicies[op]
	}
}

func applyOpSpecFieldLoadPolicies(op Op, spec *OpSpec) {
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
	if int(op) < len(opLoadElimDynamicTableCacheMutationPolicies) {
		spec.LoadElimDynamicTableCacheMutation = opLoadElimDynamicTableCacheMutationPolicies[op]
	}
	if int(op) < len(opLoadElimTypedArrayFactMutationPolicies) {
		spec.LoadElimTypedArrayFactMutation = opLoadElimTypedArrayFactMutationPolicies[op]
	}
	if int(op) < len(opLoadElimTableCacheKeyArgIndexPolicies) && opLoadElimTableCacheKeyArgIndexPolicies[op] != 0 {
		spec.LoadElimTableCacheKeyArgIndex = int(opLoadElimTableCacheKeyArgIndexPolicies[op]) - 1
	}
	if int(op) < len(opLoadElimTableCacheValueArgIndexPolicies) && opLoadElimTableCacheValueArgIndexPolicies[op] != 0 {
		spec.LoadElimTableCacheValueArgIndex = int(opLoadElimTableCacheValueArgIndexPolicies[op]) - 1
	}
	if int(op) < len(opLoadElimFactBarrierPolicies) {
		spec.LoadElimFactBarrier = opLoadElimFactBarrierPolicies[op]
	}
}

func applyOpSpecFieldBarrierPolicies(op Op, spec *OpSpec) {
	applyOpSpecFieldSvalsBarrierPolicies(op, spec)
	applyOpSpecFieldLenBarrierPolicies(op, spec)
}

func applyOpSpecFieldSvalsBarrierPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opFieldSvalsCrossBlockBarrierPolicies) {
		spec.FieldSvalsCrossBlockBarrier = opFieldSvalsCrossBlockBarrierPolicies[op]
	}
	if int(op) < len(opFieldSvalsGlobalBarrierPolicies) {
		spec.FieldSvalsGlobalBarrier = opFieldSvalsGlobalBarrierPolicies[op]
	}
	if int(op) < len(opFieldSvalsFirstArgMutationBarrierPolicies) {
		spec.FieldSvalsFirstArgMutationBarrier = opFieldSvalsFirstArgMutationBarrierPolicies[op]
	}
	if int(op) < len(opFieldSvalsLoweredOpPolicies) && opFieldSvalsLoweredOpPolicies[op].Set {
		spec.FieldSvalsLoweredOp = opFieldSvalsLoweredOpPolicies[op].Op
	}
}

func applyOpSpecFieldLenBarrierPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opFieldLenFoldBarrierPolicies) {
		spec.FieldLenFoldBarrier = opFieldLenFoldBarrierPolicies[op]
	}
	if int(op) < len(opFieldLenLoweredOpPolicies) && opFieldLenLoweredOpPolicies[op].Set {
		spec.FieldLenLoweredOp = opFieldLenLoweredOpPolicies[op].Op
	}
	if int(op) < len(opFieldCallPolyLenFusionBarrierPolicies) {
		spec.FieldCallPolyLenFusionBarrier = opFieldCallPolyLenFusionBarrierPolicies[op]
	}
	if int(op) < len(opFieldNumFusionLoweredOpPolicies) && opFieldNumFusionLoweredOpPolicies[op].Set {
		spec.FieldNumFusionLoweredOp = opFieldNumFusionLoweredOpPolicies[op].Op
	}
}
