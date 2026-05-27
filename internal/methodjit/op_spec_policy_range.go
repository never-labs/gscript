package methodjit

var opBoxableIntArithmeticPolicies = [...]bool{
	OpAddInt:      true,
	OpSubInt:      true,
	OpMulInt:      true,
	OpModInt:      true,
	OpDivIntExact: true,
	OpNegInt:      true,
}

var opUnsafeIntArithmeticCandidatePolicies = [...]bool{
	OpAddInt:      true,
	OpSubInt:      true,
	OpMulInt:      true,
	OpNegInt:      true,
	OpDivIntExact: true,
}

var opInt48SafeRangeCandidatePolicies = [...]bool{
	OpAddInt:      true,
	OpSubInt:      true,
	OpMulInt:      true,
	OpNegInt:      true,
	OpDivIntExact: true,
}

var opExactDivAllowedExternalUsePolicies = [...]bool{
	OpEq:            true,
	OpLt:            true,
	OpLe:            true,
	OpEqInt:         true,
	OpLtInt:         true,
	OpLeInt:         true,
	OpGuardType:     true,
	OpGuardIntRange: true,
	OpBranch:        true,
}

var opNonNegativeDerivationCandidatePolicies = [...]bool{
	OpConstInt:      true,
	OpLen:           true,
	OpTableArrayLen: true,
	OpGuardIntRange: true,
	OpAddInt:        true,
	OpMulInt:        true,
	OpModInt:        true,
	OpDivIntExact:   true,
	OpPhi:           true,
	OpBoxInt:        true,
	OpUnboxInt:      true,
}

var opNonNegativeDerivationKindPolicies = [...]OpNonNegativeDerivationKind{
	OpConstInt:      OpNonNegativeConstIntAux,
	OpLen:           OpNonNegativeAlways,
	OpTableArrayLen: OpNonNegativeAlways,
	OpGuardIntRange: OpNonNegativeGuardRangeMin,
	OpAddInt:        OpNonNegativeBinaryAllArgs,
	OpMulInt:        OpNonNegativeBinaryAllArgs,
	OpModInt:        OpNonNegativeModuloDivisor,
	OpDivIntExact:   OpNonNegativeExactDivPositiveDivisor,
	OpPhi:           OpNonNegativeAllArgs,
	OpBoxInt:        OpNonNegativeForwardArg,
	OpUnboxInt:      OpNonNegativeForwardArg,
}

var opInt48RuntimeValuePolicies = [...]bool{
	OpConstInt:      true,
	OpGuardType:     true,
	OpGuardIntRange: true,
	OpLoadSlot:      true,
	OpUnboxInt:      true,
}

var opFusableComparisonPolicies = [...]bool{
	OpEq:         true,
	OpLtInt:      true,
	OpLeInt:      true,
	OpEqInt:      true,
	OpModZeroInt: true,
	OpLtFloat:    true,
	OpLeFloat:    true,
}

var opLoopBoundComparisonPolicies = [...]bool{
	OpLtInt: true,
	OpLeInt: true,
	OpEqInt: true,
}

func applyOpSpecRangePolicies(op Op, spec *OpSpec) {
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
	if int(op) < len(opNonNegativeDerivationCandidatePolicies) {
		spec.NonNegativeDerivationCandidate = opNonNegativeDerivationCandidatePolicies[op]
	}
	if int(op) < len(opNonNegativeDerivationKindPolicies) {
		spec.NonNegativeDerivationKind = opNonNegativeDerivationKindPolicies[op]
		spec.NonNegativeDerivationCandidate = spec.NonNegativeDerivationCandidate || spec.NonNegativeDerivationKind != OpNonNegativeNone
	}
	if int(op) < len(opInt48RuntimeValuePolicies) {
		spec.Int48RuntimeValue = opInt48RuntimeValuePolicies[op]
	}
	if int(op) < len(opFusableComparisonPolicies) {
		spec.FusableComparison = opFusableComparisonPolicies[op]
	}
	if int(op) < len(opLoopBoundComparisonPolicies) {
		spec.LoopBoundComparison = opLoopBoundComparisonPolicies[op]
	}
}
