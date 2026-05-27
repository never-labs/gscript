package methodjit

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
