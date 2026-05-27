package methodjit

func applyOpSpecBackendPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opBackendPolicies) {
		spec.BackendPolicy = opBackendPolicies[op]
	}
	if int(op) < len(opKeepUnusedPolicies) {
		spec.KeepUnused = opKeepUnusedPolicies[op]
	}
	if int(op) < len(opNativeReplayMayExitPolicies) {
		spec.NativeReplayMayExit = opNativeReplayMayExitPolicies[op]
	}
	if int(op) < len(opNativeReplayVisibleSideEffectPolicies) {
		spec.NativeReplayVisibleSideEffect = opNativeReplayVisibleSideEffectPolicies[op]
	}
	if int(op) < len(opNativeReplayVisibleTableMutationPolicies) {
		spec.NativeReplayVisibleTableMutation = opNativeReplayVisibleTableMutationPolicies[op]
	}
	if int(op) < len(opNativeCalleeResumeUnsafePolicies) {
		spec.NativeCalleeResumeUnsafe = opNativeCalleeResumeUnsafePolicies[op]
	}
	if int(op) < len(opRestartVisibleSideEffectPolicies) {
		spec.RestartVisibleSideEffect = opRestartVisibleSideEffectPolicies[op]
	}
}
