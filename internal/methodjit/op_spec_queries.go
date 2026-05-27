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
