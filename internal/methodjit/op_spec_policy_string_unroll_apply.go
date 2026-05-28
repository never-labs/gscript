package methodjit

func applyOpSpecStringUnrollPolicies(op Op, spec *OpSpec) {
	applyOpSpecStringPolicies(op, spec)
	applyOpSpecUnrollPolicies(op, spec)
	applyOpSpecReductionPolicies(op, spec)
	applyOpSpecBranchThreadPolicies(op, spec)
}

func applyOpSpecStringPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opConstPoolUserPolicies) {
		spec.ConstPoolUser = opConstPoolUserPolicies[op]
	}
	if int(op) < len(opRawStringResultPolicies) {
		spec.RawStringResult = opRawStringResultPolicies[op]
	}
	if int(op) < len(opDynamicStringQueryCacheKeyPolicies) {
		spec.DynamicStringQueryCacheKey = opDynamicStringQueryCacheKeyPolicies[op]
	}
	if int(op) < len(opStringEnumCompareLoweredOpPolicies) && opStringEnumCompareLoweredOpPolicies[op].Set {
		spec.StringEnumCompareLoweredOp = opStringEnumCompareLoweredOpPolicies[op].Op
	}
}

func applyOpSpecUnrollPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opUnrollCloneablePolicies) {
		spec.UnrollCloneable = opUnrollCloneablePolicies[op]
	}
	if int(op) < len(opNestedFloatPhiOverrideSafePolicies) {
		spec.NestedFloatPhiOverrideSafe = opNestedFloatPhiOverrideSafePolicies[op]
	}
}

func applyOpSpecReductionPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opFloatReductionWideUnrollBarrierPolicies) {
		spec.FloatReductionWideUnrollBarrier = opFloatReductionWideUnrollBarrierPolicies[op]
	}
	if int(op) < len(opFloatReductionLatencyUnrollSeedPolicies) {
		spec.FloatReductionLatencyUnrollSeed = opFloatReductionLatencyUnrollSeedPolicies[op]
	}
	if int(op) < len(opFloatReductionLatencyUnrollBlockPolicies) {
		spec.FloatReductionLatencyUnrollBlock = opFloatReductionLatencyUnrollBlockPolicies[op]
	}
	if int(op) < len(opFloatReductionDivOpPolicies) {
		spec.FloatReductionDivOp = opFloatReductionDivOpPolicies[op]
	}
}

func applyOpSpecBranchThreadPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opConstantPhiBranchThreadPurePolicies) {
		spec.ConstantPhiBranchThreadPure = opConstantPhiBranchThreadPurePolicies[op]
	}
}
