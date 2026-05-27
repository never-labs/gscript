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
