package methodjit

func inlineAllocationRole(op Op) OpInlineAllocationRole {
	spec, ok := op.Spec()
	if !ok {
		return OpInlineAllocationNone
	}
	return spec.InlineAllocationRole
}
