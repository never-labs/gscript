package methodjit

func opHasRawStringResult(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawStringResult
}

func opIsDynamicStringQueryCacheKey(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.DynamicStringQueryCacheKey
}

func stringEnumCompareLoweredOp(op Op) (Op, bool) {
	spec, ok := op.Spec()
	return spec.StringEnumCompareLoweredOp, ok && spec.StringEnumCompareLoweredOp != OpMax
}
