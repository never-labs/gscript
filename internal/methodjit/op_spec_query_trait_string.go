package methodjit

func opHasRawStringResult(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawStringResult
}

func opIsDynamicStringQueryCacheKey(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.DynamicStringQueryCacheKey
}
