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
}

func applyOpSpecFieldLenBarrierPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opFieldLenFoldBarrierPolicies) {
		spec.FieldLenFoldBarrier = opFieldLenFoldBarrierPolicies[op]
	}
	if int(op) < len(opFieldCallPolyLenFusionBarrierPolicies) {
		spec.FieldCallPolyLenFusionBarrier = opFieldCallPolyLenFusionBarrierPolicies[op]
	}
}
