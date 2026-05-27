package methodjit

func opIsFieldNumFusionGapSafe(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FieldNumFusionGapSafe
}

func fieldShapeSplitInlineOpSafe(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FieldShapeSplitInlineSafe
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
