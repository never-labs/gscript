package methodjit

func opIsRuntimeOverflowBoxable(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RuntimeOverflowBoxable
}

func opIsRuntimeGuardRefreshable(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RuntimeGuardRefreshable
}

func opIsNestedCallLike(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NestedCallLike
}

func opIsTier2ResidualCallBlocker(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.Tier2ResidualCallBlocker
}

func opIsTier2LoopCall(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.Tier2LoopCall
}

func opIsTier2LoopNativeCandidate(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.Tier2LoopNativeCandidate
}

func opIsTier2CallBoundaryLoopBlocker(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.Tier2CallBoundaryLoopBlocker
}

func opIsTier2LoopAllocationBlocker(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.Tier2LoopAllocationBlocker
}

func opIsTier2LoopFeedbackVMProtoCall(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.Tier2LoopFeedbackVMProtoCall
}
