package methodjit

func applyOpSpecLICMPolicies(op Op, spec *OpSpec) {
	applyOpSpecLICMHoistPolicies(op, spec)
	applyOpSpecLICMMissPolicies(op, spec)
	applyOpSpecLICMIntPolicies(op, spec)
	applyOpSpecLICMLoopEffectPolicies(op, spec)
}

func applyOpSpecLICMHoistPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opLICMHoistablePolicies) {
		spec.LICMHoistable = opLICMHoistablePolicies[op]
	}
}

func applyOpSpecLICMMissPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opLICMInterestingMissPolicies) {
		spec.LICMInterestingMiss = opLICMInterestingMissPolicies[op]
	}
}

func applyOpSpecLICMIntPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opLICMIntArithPolicies) {
		spec.LICMIntArith = opLICMIntArithPolicies[op]
	}
}

func applyOpSpecLICMLoopEffectPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opLICMLoopEffectRolePolicies) {
		spec.LICMLoopEffectRole = opLICMLoopEffectRolePolicies[op]
	}
}
