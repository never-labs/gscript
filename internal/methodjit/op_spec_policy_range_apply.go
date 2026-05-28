package methodjit

func applyOpSpecRangePolicies(op Op, spec *OpSpec) {
	applyOpSpecRangeIntPolicies(op, spec)
	applyOpSpecRangeNonNegativePolicies(op, spec)
	applyOpSpecRangeComparePolicies(op, spec)
}

func applyOpSpecRangeIntPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opBoxableIntArithmeticPolicies) {
		spec.BoxableIntArithmetic = opBoxableIntArithmeticPolicies[op]
	}
	if int(op) < len(opUnsafeIntArithmeticCandidatePolicies) {
		spec.UnsafeIntArithmeticCandidate = opUnsafeIntArithmeticCandidatePolicies[op]
	}
	if int(op) < len(opInt48SafeRangeCandidatePolicies) {
		spec.Int48SafeRangeCandidate = opInt48SafeRangeCandidatePolicies[op]
	}
	if int(op) < len(opExactDivAllowedExternalUsePolicies) {
		spec.ExactDivAllowedExternalUse = opExactDivAllowedExternalUsePolicies[op]
	}
	if int(op) < len(opInt48RuntimeValuePolicies) {
		spec.Int48RuntimeValue = opInt48RuntimeValuePolicies[op]
	}
}

func applyOpSpecRangeNonNegativePolicies(op Op, spec *OpSpec) {
	if int(op) < len(opNonNegativeDerivationCandidatePolicies) {
		spec.NonNegativeDerivationCandidate = opNonNegativeDerivationCandidatePolicies[op]
	}
	if int(op) < len(opNonNegativeDerivationKindPolicies) {
		spec.NonNegativeDerivationKind = opNonNegativeDerivationKindPolicies[op]
		spec.NonNegativeDerivationCandidate = spec.NonNegativeDerivationCandidate || spec.NonNegativeDerivationKind != OpNonNegativeNone
	}
}

func applyOpSpecRangeComparePolicies(op Op, spec *OpSpec) {
	if int(op) < len(opFusableComparisonPolicies) {
		spec.FusableComparison = opFusableComparisonPolicies[op]
	}
	if int(op) < len(opModZeroCompareLoweredOpPolicies) && opModZeroCompareLoweredOpPolicies[op].Set {
		spec.ModZeroCompareLoweredOp = opModZeroCompareLoweredOpPolicies[op].Op
	}
	if int(op) < len(opLoopBoundComparisonPolicies) {
		spec.LoopBoundComparison = opLoopBoundComparisonPolicies[op]
	}
}
