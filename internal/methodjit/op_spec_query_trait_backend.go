package methodjit

func opIsNativeCalleeResumeUnsafe(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NativeCalleeResumeUnsafe
}

func opIsNativeReplayVisibleTableMutation(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NativeReplayVisibleTableMutation
}

func opIsNativeReplayVisibleSideEffect(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NativeReplayVisibleSideEffect
}

func opIsNativeReplayMayExit(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.NativeReplayMayExit
}

func opIsRestartVisibleSideEffect(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RestartVisibleSideEffect
}

func opBackendPolicy(op Op) OpBackendPolicy {
	spec, ok := op.Spec()
	if !ok {
		return 0
	}
	return spec.BackendPolicy
}

func opMayDirectDeoptWithoutFullFlush(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.DirectDeoptWithoutFullFlush
}

func instrUsesConstPool(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.ConstPoolUser
}
