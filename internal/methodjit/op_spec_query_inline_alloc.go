package methodjit

func inlineAllocationRole(op Op) OpInlineAllocationRole {
	spec, ok := op.Spec()
	if !ok {
		return OpInlineAllocationNone
	}
	return spec.InlineAllocationRole
}

func inlineAllocationLoweredOp(op Op) (Op, bool) {
	spec, ok := op.Spec()
	return spec.InlineAllocationLoweredOp, ok && spec.InlineAllocationLoweredOp != OpMax
}
